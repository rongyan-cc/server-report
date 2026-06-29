// Server Report - Linux 服务器日报程序
// 博客: https://rongyan.cc
// 说明: https://rongyan.cc/code/server-report.html
//
package main

import (
	"fmt"
	"strings"
	"strconv"
)

func collectNetwork() string {
	var b strings.Builder

	writeLine(&b, separator)
	writeLine(&b, " 网络连接与流量")
	writeLine(&b, separator, "\n")

	ss := runCmd("ss", "-tun")
	lines := splitLines(ss)

	estab := 0
	timewait := 0
	total := 0
	for _, line := range lines {
		if strings.Contains(line, "ESTAB") {
			estab++
			total++
		} else if strings.Contains(line, "TIME-WAIT") {
			timewait++
			total++
		} else if strings.Contains(line, "SYN-SENT") {
			total++
		}
	}

	writeLine(&b, fmt.Sprintf("  ESTABLISHED: %d  |  TIME-WAIT: %d  |  总计: %d\n", estab, timewait, total))

	// TOP 10 connections
	writeLine(&b, "■ 对外连接 TOP 10 (ESTABLISHED)\n")
	ssEstab := runCmd("ss", "-tun", "state", "established")
	connCount := make(map[string]int)
	for _, line := range splitLines(ssEstab) {
		fields := strings.Fields(line)
		if len(fields) >= 5 {
			dest := fields[4]
			// Remove brackets for IPv6
			dest = strings.TrimLeft(dest, "[")
			dest = strings.TrimRight(dest, "]")
			connCount[dest]++
		}
	}

	type connEntry struct {
		addr  string
		count int
	}
	var sorted []connEntry
	for addr, c := range connCount {
		sorted = append(sorted, connEntry{addr, c})
	}
	// Sort by count desc (simple bubble sort)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].count > sorted[i].count {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	for i, e := range sorted {
		if i >= 10 {
			break
		}
		proto := "TCP"
		writeLine(&b, fmt.Sprintf("  %-3d  %-4s  %s", e.count, proto, e.addr))
	}

	writeLine(&b, "")

	// Listening ports
	writeLine(&b, "■ 监听端口\n")
	ssListen := runCmd("ss", "-tlnp")
	type portEntry struct {
		port string
		name string
	}
	var ports []portEntry
	seen := make(map[string]bool)
	for _, line := range splitLines(ssListen) {
		fields := strings.Fields(line)
		if len(fields) >= 4 {
			addr := fields[3]
			parts := strings.Split(addr, ":")
			port := parts[len(parts)-1]
			name := ""
			pidField := fields[len(fields)-1]
			if strings.Contains(pidField, "\"") {
				parts2 := strings.Split(pidField, "\"")
				if len(parts2) >= 2 {
					name = parts2[1]
				}
			}
			key := port + "|" + name
			if !seen[key] && name != "" {
				seen[key] = true
				ports = append(ports, portEntry{port, name})
			}
		}
	}
	for _, p := range ports {
		writeLine(&b, fmt.Sprintf("  %-6s %s", p.port, p.name))
	}

	writeLine(&b, "")

	// Network traffic
	writeLine(&b, "■ 网卡流量（昨日至今）\n")
	ifaces := runCmd("ls", "/sys/class/net/")
	for _, iface := range splitLines(ifaces) {
		if iface == "lo" {
			continue
		}
		rx := runCmd("cat", "/sys/class/net/"+iface+"/statistics/rx_bytes")
		tx := runCmd("cat", "/sys/class/net/"+iface+"/statistics/tx_bytes")
		if rx == "" || tx == "" {
			continue
		}
		rxBytes, _ := strconv.ParseInt(rx, 10, 64)
		txBytes, _ := strconv.ParseInt(tx, 10, 64)

		baseDir := "/var/lib/server-report"
		rxFile := baseDir + "/net_" + iface + "_rx"
		txFile := baseDir + "/net_" + iface + "_tx"

		lastRx := readIntFromFile(rxFile)
		lastTx := readIntFromFile(txFile)

		if lastRx > 0 {
			diffRx := rxBytes - lastRx
			diffTx := txBytes - lastTx
			writeLine(&b, fmt.Sprintf("  %s:  入站 %s  出站 %s",
				iface, formatBytes(diffRx), formatBytes(diffTx)))
		} else {
			writeLine(&b, fmt.Sprintf("  %s:  首次运行，明日开始统计", iface))
		}
		writeIntToFile(rxFile, rxBytes)
		writeIntToFile(txFile, txBytes)
	}

	writeLine(&b, "")
	return b.String()
}

func formatBytes(b int64) string {
	if b < 1024 { return fmt.Sprintf("%d B", b) }
	if b < 1024*1024 { return fmt.Sprintf("%.1f KB", float64(b)/1024) }
	if b < 1024*1024*1024 { return fmt.Sprintf("%.1f MB", float64(b)/(1024*1024)) }
	return fmt.Sprintf("%.1f GB", float64(b)/(1024*1024*1024))
}

func readIntFromFile(path string) int64 {
	data := runCmd("cat", path)
	data = strings.TrimSpace(data)
	if data == "" {
		return 0
	}
	n, err := strconv.ParseInt(data, 10, 64)
	if err != nil {
		return 0
	}
	return n
}

func writeIntToFile(path string, val int64) {
	runCmd("sh", "-c", fmt.Sprintf("echo %d > %s", val, path))
}
