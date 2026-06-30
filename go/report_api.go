// Server Report - Linux 服务器日报程序
// 博客: https://rongyan.cc
// 说明: https://rongyan.cc/code/server-report.html
//
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"strconv"
)

func buildJSONReport(cfg *Config) APIReport {
	yesterday, today := cfg.ReportDate()

	return APIReport{
		Status:    "ok",
		Server:    cfg.Server.Name,
		Timestamp: runCmd("date", "+%Y-%m-%d %H:%M:%S"),
		Date:      yesterday + " → " + today,
		Sections: []APISection{
			buildSystemSection(),
			buildSSHAuthSection(),
			buildFail2banSection(),
			buildNetworkSection(),
			buildFirewallSection(),
			buildServicesSection(),
			buildChangesSection(),
			buildSecuritySection(),
		},
	}
}

// ── 系统 ──
func buildSystemSection() APISection {
	hostname := runCmd("hostname")
	data := SystemData{Hostname: hostname}

	data.IP = runCmd("curl", "-s", "-4", "https://one.one.one.one/cdn-cgi/trace")
	for _, line := range splitLines(data.IP) {
		if strings.HasPrefix(line, "ip=") {
			data.IP = strings.TrimPrefix(line, "ip=")
			break
		}
	}

	uptime := runCmd("uptime", "-p")
	data.Uptime = strings.TrimPrefix(uptime, "up ")
	data.Load = runCmd("uptime")
	if idx := strings.Index(data.Load, "load average:"); idx != -1 {
		data.Load = strings.TrimSpace(data.Load[idx+13:])
	}

	cpu := runCmd("top", "-bn1")
	for _, line := range splitLines(cpu) {
		if strings.Contains(line, "Cpu(s)") || strings.Contains(line, "%Cpu") {
			fields := strings.Fields(line)
			for i, f := range fields {
				if f == "us," && i > 0 {
					data.CPU = fields[i-1] + " us"
					break
				}
			}
			break
		}
	}

	mem := runCmd("free", "-b")
	for _, line := range splitLines(mem) {
		if strings.HasPrefix(line, "Mem:") {
			fields := strings.Fields(line)
			if len(fields) >= 3 {
				total, _ := strconv.ParseInt(fields[1], 10, 64)
				used, _ := strconv.ParseInt(fields[2], 10, 64)
				data.Memory.Total = formatBytes(total)
				data.Memory.Used = formatBytes(used)
				if total > 0 {
					data.Memory.Percent = int(used * 100 / total)
				}
			}
			break
		}
	}

	disk := runCmd("df", "-B1", "-x", "tmpfs", "-x", "devtmpfs")
	for _, line := range splitLines(disk) {
		fields := strings.Fields(line)
		if len(fields) >= 6 && strings.HasPrefix(line, "/") {
			total, _ := strconv.ParseInt(fields[1], 10, 64)
			used, _ := strconv.ParseInt(fields[2], 10, 64)
			info := DiskInfo{
				Mount:   fields[5],
				Used:    formatBytes(used),
				Total:   formatBytes(total),
			}
			if total > 0 {
				info.Percent = int(used * 100 / total)
			}
			data.Disks = append(data.Disks, info)
		}
	}

	return APISection{ID: "system", Title: "系统健康", Type: "system", Data: data}
}

// ── SSH ──
func buildSSHAuthSection() APISection {
	sd := SSHAuthData{}

	since := runCmd("date", "-d", "yesterday 00:00:00", "+%Y-%m-%d %H:%M:%S")
	until := runCmd("date", "-d", "today 00:00:00", "+%Y-%m-%d %H:%M:%S")
	log := runCmd("journalctl", "-u", "ssh.service", "--since", since, "--until", until, "-o", "short-iso", "--no-pager")

	failLines := filterLines(log, "Failed password", "Invalid user")
	okLines := filterLines(log, "Accepted")

	// 失败统计
	type fe struct { count int; users string; first, last string }
	fm := make(map[string]*fe)
	for _, line := range failLines {
		ip := extractIP(line)
		u := extractUser(line)
		ts := extractTimestamp(line)
		if ip == "" { continue }
		if _, ok := fm[ip]; !ok { fm[ip] = &fe{} }
		fm[ip].count++
		if u != "" && !strings.Contains(fm[ip].users, u) {
			if fm[ip].users != "" { fm[ip].users += "," }
			fm[ip].users += u
		}
		if fm[ip].first == "" || ts < fm[ip].first { fm[ip].first = ts }
		if ts > fm[ip].last { fm[ip].last = ts }
	}
	sd.FailedTotal = len(failLines)
	sd.FailedIPs = len(fm)
	for ip, e := range fm {
		fi := FailedIPInfo{
			IP: ip, Count: e.count,
			Users: splitLines(e.users),
			First: e.first, Last: e.last,
		}
		loc := geoLookup(ip)
		if parts := strings.SplitN(loc, "/", 2); len(parts) == 2 {
			fi.LocationCN, fi.LocationEN = parts[0], parts[1]
		} else {
			fi.LocationEN = loc
		}
		if fi.LocationCN == "中国" {
			fi.LocationDetail = geoLookupDetail(ip)
		}
		sd.FailedDetail = append(sd.FailedDetail, fi)
	}

	// 成功聚合
	type se struct{ count int; method string }
	sm := make(map[string]*se)
	for _, line := range okLines {
		ip := extractIP(line)
		method := extractMethod(line)
		if ip == "" { continue }
		if _, ok := sm[ip]; !ok { sm[ip] = &se{} }
		sm[ip].count++
		if method != "" { sm[ip].method = method }
	}
	sd.SuccessTotal = len(okLines)
	for ip, e := range sm {
		sd.SuccessList = append(sd.SuccessList, SuccessIPInfo{IP: ip, Count: e.count, Method: e.method})
	}

	return APISection{ID: "ssh_auth", Title: "SSH 登录审计", Type: "ssh_auth", Data: sd}
}

// ── fail2ban ──
func buildFail2banSection() APISection {
	fd := Fail2banData{}
	status := runCmd("fail2ban-client", "status", "sshd")

	for _, line := range splitLines(status) {
		if strings.Contains(line, "Total banned") {
			fields := strings.Fields(line)
			if len(fields) > 0 { fd.TotalBanned = atoi(fields[len(fields)-1]) }
		}
		if strings.Contains(line, "Currently banned") {
			fields := strings.Fields(line)
			if len(fields) > 0 { fd.CurrentBanned = atoi(fields[len(fields)-1]) }
		}
	}

	// 获取被封 IP 列表
	bannedIPs := ""
	for _, line := range splitLines(status) {
		if strings.Contains(line, "Banned IP list") {
			if idx := strings.Index(line, ":"); idx != -1 {
				bannedIPs = strings.TrimSpace(line[idx+1:])
			}
		}
	}

	journalAll := runCmd("journalctl", "-u", "ssh.service", "--since", "-7 days", "-o", "short-iso", "--no-pager")
	subnetCount := make(map[string]int)

	for _, ip := range strings.Fields(bannedIPs) {
		if strings.Contains(ip, ":") { continue }

		bi := BannedIP{IP: ip}
		// 归属地
		loc := geoLookup(ip)
		if parts := strings.SplitN(loc, "/", 2); len(parts) == 2 {
			bi.LocationCN, bi.LocationEN = parts[0], parts[1]
		} else {
			bi.LocationEN = loc
		}

		// 中国 IP 查询详细位置
		if bi.LocationCN == "中国" {
			bi.LocationDetail = geoLookupDetail(ip)
		}

		bi.BanTime = getBanTime(ip)

		sshLines := filterLines(journalAll, ip)
		failLines := filterLinesFromSlice(sshLines, "Failed password", "Invalid user")
		bi.Attempts = len(failLines)
		for _, l := range failLines {
			if u := extractUser(l); u != "" {
				bi.Users = append(bi.Users, u)
			}
		}

		// 子网
		parts := strings.Split(ip, ".")
		if len(parts) >= 3 {
			subnet := parts[0] + "." + parts[1] + "." + parts[2] + ".0/24"
			bi.Subnet = subnet
			subnetCount[subnet]++
		}

		// 去重用户名
		seen := make(map[string]bool)
		var uniq []string
		for _, u := range bi.Users {
			if !seen[u] { seen[u] = true; uniq = append(uniq, u) }
		}
		bi.Users = uniq

		fd.BannedIPs = append(fd.BannedIPs, bi)
	}

	for subnet, count := range subnetCount {
		if count >= 2 {
			fd.BannedSubnets = append(fd.BannedSubnets, SubnetInfo{Subnet: subnet, IPCount: count})
		}
	}

	return APISection{ID: "fail2ban", Title: "fail2ban 封禁统计", Type: "fail2ban", Data: fd}
}

// ── 网络 ──
func buildNetworkSection() APISection {
	nd := NetworkData{}

	ss := runCmd("ss", "-tun")
	for _, line := range splitLines(ss) {
		if strings.Contains(line, "ESTAB") { nd.Established++; nd.Total++ }
		if strings.Contains(line, "TIME-WAIT") { nd.TimeWait++; nd.Total++ }
	}

	ssEstab := runCmd("ss", "-tun", "state", "established")
	connCount := make(map[string]int)
	for _, line := range splitLines(ssEstab) {
		fields := strings.Fields(line)
		if len(fields) >= 5 {
			dest := fields[4]
			dest = strings.TrimLeft(dest, "[")
			dest = strings.TrimRight(dest, "]")
			connCount[dest]++
		}
	}
	for addr, c := range sortMapStrInt(connCount, 10) {
		nd.TopConns = append(nd.TopConns, ConnInfo{Dest: addr, Count: c})
	}

	ssListen := runCmd("ss", "-tlnp")
	seen := make(map[string]bool)
	for _, line := range splitLines(ssListen) {
		fields := strings.Fields(line)
		if len(fields) >= 4 {
			parts := strings.Split(fields[3], ":")
			port := parts[len(parts)-1]
			name := ""
			if len(fields) > 4 {
				pidField := fields[len(fields)-1]
				if strings.Contains(pidField, "\"") {
					pts := strings.Split(pidField, "\"")
					if len(pts) >= 2 { name = pts[1] }
				}
			}
			key := port + "|" + name
			if !seen[key] && name != "" {
				seen[key] = true
				nd.ListeningP = append(nd.ListeningP, PortInfo{Port: port, Name: name})
			}
		}
	}

	for _, iface := range splitLines(runCmd("ls", "/sys/class/net/")) {
		if iface == "lo" { continue }
		rx := runCmd("cat", "/sys/class/net/"+iface+"/statistics/rx_bytes")
		tx := runCmd("cat", "/sys/class/net/"+iface+"/statistics/tx_bytes")
		if rx == "" || tx == "" { continue }

		rxFile := "/var/lib/server-report/net_" + iface + "_rx"
		txFile := "/var/lib/server-report/net_" + iface + "_tx"
		lastRx := readInt64(rxFile)
		lastTx := readInt64(txFile)
		diffRx, _ := strconv.ParseInt(rx, 10, 64)
		diffTx, _ := strconv.ParseInt(tx, 10, 64)

		ti := TrafficInfo{Iface: iface}
		if lastRx > 0 {
			ti.Rx = formatBytes(diffRx - lastRx)
			ti.Tx = formatBytes(diffTx - lastTx)
		} else {
			ti.Rx = "（首次）"
		}
		nd.Traffic = append(nd.Traffic, ti)
		writeInt64(rxFile, diffRx)
		writeInt64(txFile, diffTx)
	}

	return APISection{ID: "network", Title: "网络连接与流量", Type: "network", Data: nd}
}

// ── 防火墙 ──
func buildFirewallSection() APISection {
	fd := FirewallData{}
	log := runCmd("cat", "/var/log/ufw.log")
	yesterday := runCmd("date", "-d", "yesterday", "+%b %e")

	for _, line := range splitLines(log) {
		if !strings.Contains(line, yesterday) && !strings.Contains(line, strings.TrimSpace(yesterday)) { continue }
		if strings.Contains(line, "UFW BLOCK") {
			fd.TotalBlocks++
			src := extractFieldValue(line, "SRC=")
			dst := extractFieldValue(line, "DPT=")
			proto := extractFieldValue(line, "PROTO=")
			if src != "" { fd.TopSrc = append(fd.TopSrc, KVEntry{Key: src, Val: 1}) }
			if dst != "" { fd.TopDst = append(fd.TopDst, KVEntry{Key: dst + "/" + proto, Val: 1}) }
		}
	}

	// 聚合统计
	fd.TopSrc = aggregateKV(fd.TopSrc, 10)
	fd.TopDst = aggregateKV(fd.TopDst, 10)

	return APISection{ID: "firewall", Title: "防火墙拦截统计", Type: "firewall", Data: fd}
}

// ── 服务 ──
func buildServicesSection() APISection {
	sd := ServicesData{}
	svc := runCmd("systemctl", "list-units", "--type=service", "--state=running", "--no-legend")
	for _, line := range splitLines(svc) {
		fields := strings.Fields(line)
		if len(fields) >= 4 {
			sd.Running = append(sd.Running, ServiceInfo{
				Name: fields[0],
				Desc: strings.Join(fields[3:], " "),
			})
		}
	}
	sd.Total = len(sd.Running)
	return APISection{ID: "services", Title: "运行中的系统服务", Type: "services", Data: sd}
}

// ── 变更 ──
func buildChangesSection() APISection {
	cd := ChangesData{}
	aptLog := runCmd("cat", "/var/log/apt/history.log")
	yesterday := runCmd("date", "-d", "yesterday", "+%Y-%m-%d")
	for _, line := range splitLines(aptLog) {
		if strings.Contains(line, yesterday) && (strings.Contains(line, "Install:") || strings.Contains(line, "Upgrade:") || strings.Contains(line, "Remove:")) {
			cd.Packages = append(cd.Packages, strings.TrimSpace(line))
		}
	}
	return APISection{ID: "changes", Title: "系统变更记录", Type: "changes", Data: cd}
}

// ── 安全 ──
func buildSecuritySection() APISection {
	sd := SecurityData{SUIDOK: true}

	suid := runCmd("sh", "-c", "find /usr/bin /usr/sbin /bin /sbin -type f -perm -4000 2>/dev/null | sort")
	if baselineExists("/var/lib/server-report/suid-baseline.txt") {
		old := readFile("/var/lib/server-report/suid-baseline.txt")
		if old != suid { sd.SUIDOK = false }
	} else {
		writeFile("/var/lib/server-report/suid-baseline.txt", suid)
	}

	passwd := runCmd("awk", "-F:", "$3>=1000 && $3<65534 {print $1\":\"$3\":\"$5}", "/etc/passwd")
	ubase := "/var/lib/server-report/newusers-baseline.txt"
	if baselineExists(ubase) {
		old := readFile(ubase)
		if old != passwd {
			oldLines := strings.Split(old, "\n")
			om := make(map[string]bool)
			for _, l := range oldLines { om[l] = true }
			for _, line := range splitLines(passwd) {
				if !om[line] {
					parts := strings.Split(line, ":")
					if len(parts) >= 2 { sd.NewUsers = append(sd.NewUsers, parts[0]) }
				}
			}
		}
	}
	writeFile(ubase, passwd)

	ps := runCmd("ps", "aux")
	for _, proc := range []string{"ksysrqd", "xmrig", "miner", "hashcat", "stratum"} {
		for _, line := range splitLines(ps) {
			if strings.Contains(line, proc) && !strings.Contains(line, "grep") {
				fields := strings.Fields(line)
				if len(fields) >= 11 { sd.Suspicious = append(sd.Suspicious, fields[10]) }
			}
		}
	}

	who := runCmd("who")
	for _, line := range splitLines(who) {
		fields := strings.Fields(line)
		if len(fields) >= 3 { sd.OnlineUsers = append(sd.OnlineUsers, fmt.Sprintf("%s@%s", fields[0], fields[len(fields)-1])) }
	}

	errLogs := runCmd("journalctl", "-p", "err", "--since", "-24h", "--no-pager")
	noisePatterns := []string{
		"no more sessions", "networking.service", "Raise network",
		"Failed unmounting", "AF_VSOCK CID", "systemd-ssh-generator",
		"net_driver", "Unknown builtin command", "sd-exec-", "hp-lite-server",
		"kex_exchange_identification", "send_error: write", "soft lockup",
		"watchdog: BUG", "(CRON) EXEC FAILED", "sendmail): No such file",
		"Crash recovery kernel", "kdump", "rpcbind", "Apply Kernel Variables",
		"chrony", "NTP client/server", "rcu_sched", "rcu:",
		"systemd-networkd-wait-online", "Watchdog timeout",
		"kauditd hold queue", "anacron.*sendmail", "Can't find sendmail",
	}
	for _, line := range splitLines(errLogs) {
		line = strings.TrimSpace(line)
		if line == "" { continue }
		isNoise := false
		for _, p := range noisePatterns {
			if strings.Contains(line, p) { isNoise = true; break }
		}
		if isNoise { continue }
		if len(sd.Errors) >= 10 { break }
		sd.Errors = append(sd.Errors, line)
	}

	return APISection{ID: "security", Title: "安全检测", Type: "security", Data: sd}
}

// ── 辅助 ──
func sortMapStrInt(m map[string]int, limit int) map[string]int {
	type kv struct{ k string; v int }
	var sorted []kv
	for k, v := range m { sorted = append(sorted, kv{k, v}) }
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].v > sorted[i].v { sorted[i], sorted[j] = sorted[j], sorted[i] }
		}
	}
	r := make(map[string]int)
	for i, e := range sorted {
		if i >= limit { break }
		r[e.k] = e.v
	}
	return r
}

func aggregateKV(entries []KVEntry, limit int) []KVEntry {
	m := make(map[string]int)
	for _, e := range entries { m[e.Key] += e.Val }
	var sorted []KVEntry
	for k, v := range m { sorted = append(sorted, KVEntry{k, v}) }
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].Val > sorted[i].Val { sorted[i], sorted[j] = sorted[j], sorted[i] }
		}
	}
	if len(sorted) > limit { sorted = sorted[:limit] }
	return sorted
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func readInt64(path string) int64 {
	data := strings.TrimSpace(readFile(path))
	if data == "" { return 0 }
	n, _ := strconv.ParseInt(data, 10, 64)
	return n
}

func writeInt64(path string, val int64) {
	writeFile(path, strconv.FormatInt(val, 10))
}

// ── 保存报告 ──

func saveReport(cfg *Config, archive bool) {
	dir := reportDir(cfg)
	os.MkdirAll(dir, 0755)

	result := buildJSONReport(cfg)
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fmt.Printf("序列化报告失败: %v\n", err)
		return
	}

	if archive {
		// 归档：保存为 YYYY-MM-DD.json
		yesterday, _ := cfg.ReportDate()
		path := dateReportPath(cfg, yesterday)
		if err := os.WriteFile(path, data, 0644); err != nil {
			fmt.Printf("保存归档报告失败: %v\n", err)
			return
		}
		fmt.Printf("归档报告已保存: %s (%d bytes)\n", path, len(data))

		// 归档后清理 today.json（它已被覆盖）
		os.Remove(todayReportPath(cfg))
	} else {
		// 每小时快照：保存为 today.json
		path := todayReportPath(cfg)
		if err := os.WriteFile(path, data, 0644); err != nil {
			fmt.Printf("保存今日报告失败: %v\n", err)
			return
		}
		fmt.Printf("今日报告已更新: %s (%d bytes)\n", path, len(data))
	}
}
