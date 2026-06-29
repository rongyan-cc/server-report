// Server Report - Linux 服务器日报程序
// 博客: https://rongyan.cc
// 说明: https://rongyan.cc/code/server-report.html
//
package main

import (
	"fmt"
	"io/ioutil"
	"strings"
)

func collectSecurity() string {
	var b strings.Builder

	writeLine(&b, separator)
	writeLine(&b, " 安全检测")
	writeLine(&b, separator, "\n")

	writeLine(&b, "■ SUID 文件变更\n")
	suidOutput := runCmd("sh", "-c", "find /usr/bin /usr/sbin /bin /sbin -type f -perm -4000 2>/dev/null | sort")
	suidBase := "/var/lib/server-report/suid-baseline.txt"
	if baselineExists(suidBase) {
		oldSuid := readFile(suidBase)
		if oldSuid != suidOutput {
			writeLine(&b, "  ⚠️ SUID 文件有变更！\n")
		} else {
			writeLine(&b, "  ✅ SUID 文件无变化\n")
		}
	} else {
		writeLine(&b, "  首次运行，已建立 SUID 基线\n")
	}
	writeFile(suidBase, suidOutput)

	writeLine(&b, "\n■ 新增用户\n")
	passwd := runCmd("awk", "-F:", "$3>=1000 && $3<65534 {print $1\":\"$3\":\"$5}", "/etc/passwd")
	userBase := "/var/lib/server-report/newusers-baseline.txt"
	if baselineExists(userBase) {
		oldUsers := readFile(userBase)
		if oldUsers != passwd {
			oldLines := strings.Split(oldUsers, "\n")
			oldMap := make(map[string]bool)
			for _, l := range oldLines {
				oldMap[l] = true
			}
			for _, line := range splitLines(passwd) {
				if !oldMap[line] {
					parts := strings.Split(line, ":")
					if len(parts) >= 2 {
						writeLine(&b, fmt.Sprintf("  ✨ 新用户: %s (UID: %s)\n", parts[0], parts[1]))
					}
				}
			}
		} else {
			writeLine(&b, "  昨日无新增用户\n")
		}
	} else {
		writeLine(&b, "  首次运行，已建立基线\n")
	}
	writeFile(userBase, passwd)

	writeLine(&b, "\n■ 异常进程/痕迹\n")
	suspicious := []string{"ksysrqd", "xmrig", "miner", "hashcat", "stratum"}
	found := false
	psOut := runCmd("ps", "aux")
	for _, line := range splitLines(psOut) {
		for _, proc := range suspicious {
			if strings.Contains(line, proc) && !strings.Contains(line, "grep") {
				fields := strings.Fields(line)
				if len(fields) >= 11 {
					writeLine(&b, fmt.Sprintf("  ⚠️ 可疑进程: %s\n", fields[10]))
					found = true
				}
				break
			}
		}
	}
	if !found {
		writeLine(&b, "  ✅ 无异常进程\n")
	}

	writeLine(&b, "\n■ 当前系统在线用户\n")
	who := runCmd("who")
	if who != "" {
		for _, line := range splitLines(who) {
			fields := strings.Fields(line)
			if len(fields) >= 3 {
				writeLine(&b, fmt.Sprintf("  %-10s %-15s %s\n", fields[0], fields[len(fields)-1], fields[2]+" "+fields[3]))
			}
		}
	} else {
		writeLine(&b, "  无在线用户\n")
	}

	writeLine(&b, "\n■ 系统错误日志（昨日至今）\n")
	errLogs := runCmd("journalctl", "-p", "err", "--since", "-24h", "--no-pager")
	errLines := splitLines(errLogs)

	// 过滤无意义噪音
	noisePatterns := []string{
		"no more sessions",
		"networking.service",
		"Raise network",
		"Failed unmounting",
		"AF_VSOCK CID",
		"systemd-ssh-generator",
		"net_driver",
		"Unknown builtin command",
		"sd-exec-",
		"hp-lite-server",
		"kex_exchange_identification",
		"send_error: write",
		"soft lockup",
		"watchdog: BUG",
		"(CRON) EXEC FAILED",
		"sendmail): No such file",
		"Crash recovery kernel",
		"kdump",
		"rpcbind",
		"Apply Kernel Variables",
		"chrony",
		"NTP client/server",
		"rcu_sched",
		"rcu:",
		"systemd-networkd-wait-online",
		"Watchdog timeout",
		"kauditd hold queue",
		"anacron.*sendmail",
		"Can't find sendmail",
	}

	var filtered []string
	for _, line := range errLines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		isNoise := false
		for _, p := range noisePatterns {
			if strings.Contains(line, p) {
				isNoise = true
				break
			}
		}
		if isNoise {
			continue
		}
		// 过滤时间戳不正常的行
		if strings.HasPrefix(line, "C20250425817519") {
			continue
		}
		filtered = append(filtered, line)
	}

	if len(filtered) > 10 {
		filtered = filtered[:10]
	}

	if len(filtered) > 0 {
		for _, line := range filtered {
			writeLine(&b, fmt.Sprintf("    %s", line))
		}
	} else {
		writeLine(&b, "  无系统错误日志\n")
	}

	return b.String()
}

func checkSubnetBan() {
	if !hasCmd("fail2ban-client") {
		return
	}

	banLog := "/var/log/subnet-ban.log"
	lockFile := "/var/lib/server-report/banned-subnets.txt"
	chainName := "AUTO_BAN_SUBNET"

	runCmd("iptables", "-N", chainName)
	_ = runCmd("iptables", "-C", "INPUT", "-j", chainName)
	_ = runCmd("iptables", "-I", "INPUT", "-j", chainName)

	status := runCmd("fail2ban-client", "status", "sshd")
	bannedIPs := ""
	for _, line := range splitLines(status) {
		if strings.Contains(line, "Banned IP list") {
			idx := strings.Index(line, ":")
			if idx != -1 {
				bannedIPs = strings.TrimSpace(line[idx+1:])
			}
		}
	}
	if bannedIPs == "" {
		return
	}

	type subnetInfo struct {
		count int
		ips   []string
	}
	subnets := make(map[string]*subnetInfo)

	for _, ip := range strings.Fields(bannedIPs) {
		if strings.Contains(ip, ":") {
			continue
		}
		parts := strings.Split(ip, ".")
		if len(parts) < 4 {
			continue
		}
		subnet := parts[0] + "." + parts[1] + "." + parts[2] + ".0/24"
		if _, ok := subnets[subnet]; !ok {
			subnets[subnet] = &subnetInfo{}
		}
		subnets[subnet].count++
		subnets[subnet].ips = append(subnets[subnet].ips, ip)
	}

	alreadyBanned := make(map[string]bool)
	if data, err := ioutil.ReadFile(lockFile); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if strings.Contains(line, "/") {
				alreadyBanned[line] = true
			}
		}
	}

	for subnet, info := range subnets {
		if info.count >= 2 && !alreadyBanned[subnet] {
			_ = runCmd("iptables", "-A", chainName, "-s", subnet, "-j", "DROP")
			logMsg := fmt.Sprintf("[%s] 已封禁子网: %s (触发IP: %s)",
				runCmd("date", "+%Y-%m-%d %H:%M:%S"),
				subnet, strings.Join(info.ips, " "))
			existing := readFile(lockFile)
			existing += "\n---\n" + subnet + "\n" + subnet +
				fmt.Sprintf(" (%d个IP: %s)", info.count, strings.Join(info.ips, " "))
			writeFile(lockFile, existing)
			appendFile(banLog, logMsg)
			_ = runCmd("ufw", "deny", "from", subnet, "comment", "auto-ban subnet")
		}
	}

	_ = runCmd("sh", "-c", "iptables-save 2>/dev/null | grep -v '^#' > /etc/iptables/rules.v4 2>/dev/null")
}
