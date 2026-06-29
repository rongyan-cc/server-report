// Server Report - Linux 服务器日报程序
// 博客: https://rongyan.cc
// 说明: https://rongyan.cc/code/server-report.html
//
package main

import (
	"fmt"
	"strings"
)

func collectSystem() string {
	var b strings.Builder

	writeLine(&b, separator)
	writeLine(&b, " 系统健康")
	writeLine(&b, separator)
	writeLine(&b, "")

	hostname := runCmd("hostname")
	uptime := runCmd("uptime", "-p")
	load := runCmd("uptime")
	ip := runCmd("curl", "-s", "-4", "https://one.one.one.one/cdn-cgi/trace")
	if ip != "" {
		for _, line := range splitLines(ip) {
			if strings.HasPrefix(line, "ip=") {
				ip = strings.TrimPrefix(line, "ip=")
				break
			}
		}
	} else {
		ip = runCmd("ip", "route", "get", "1")
		fields := strings.Fields(ip)
		for i, f := range fields {
			if f == "src" && i+1 < len(fields) {
				ip = fields[i+1]
				break
			}
		}
	}
	uptime = strings.TrimPrefix(uptime, "up ")

	writeLine(&b, fmt.Sprintf("  主机名: %s", hostname))
	writeLine(&b, fmt.Sprintf("  公网IP: %s", ip))
	writeLine(&b, fmt.Sprintf("  运行时间: %s", uptime))
	if idx := strings.Index(load, "load average:"); idx != -1 {
		writeLine(&b, fmt.Sprintf("  系统负载: %s", load[idx+13:]))
	}

	// CPU
	cpu := runCmd("top", "-bn1")
	for _, line := range splitLines(cpu) {
		if strings.Contains(line, "Cpu(s)") || strings.Contains(line, "%Cpu") {
			fields := strings.Fields(line)
			for i, f := range fields {
				if f == "us," && i > 0 {
					writeLine(&b, fmt.Sprintf("  CPU 使用率: %s", fields[i-1]))
					break
				}
			}
			break
		}
	}

	// Memory
	mem := runCmd("free", "-h")
	for _, line := range splitLines(mem) {
		if strings.HasPrefix(line, "Mem:") {
			fields := strings.Fields(line)
			if len(fields) >= 3 {
				writeLine(&b, fmt.Sprintf("  内存: %s / %s", fields[2], fields[1]))
			}
			break
		}
	}

	// Disk
	disk := runCmd("df", "-h", "/")
	for _, line := range splitLines(disk) {
		if strings.HasPrefix(line, "/") {
			fields := strings.Fields(line)
			if len(fields) >= 5 {
				writeLine(&b, fmt.Sprintf("  磁盘 /: %s / %s (%s)", fields[2], fields[1], fields[4]))
			}
			break
		}
	}

	writeLine(&b, "")
	writeLine(&b, "  挂载点:")
	df := runCmd("df", "-h", "-x", "tmpfs", "-x", "devtmpfs")
	for _, line := range splitLines(df) {
		if len(line) > 0 && line[0] == '/' {
			fields := strings.Fields(line)
			if len(fields) >= 6 {
				writeLine(&b, fmt.Sprintf("    %-12s %s/%s (%s)", fields[5], fields[2], fields[1], fields[4]))
			}
		}
	}
	writeLine(&b, "")

	return b.String()
}
