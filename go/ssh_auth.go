// Server Report - Linux 服务器日报程序
// 博客: https://rongyan.cc
// 说明: https://rongyan.cc/code/server-report.html
//
package main

import (
	"fmt"
	"strings"
)

func collectSSHAuth() string {
	var b strings.Builder

	writeLine(&b, separator)
	writeLine(&b, " SSH 登录审计")
	writeLine(&b, separator, "\n")

	since := runCmd("date", "-d", "yesterday 00:00:00", "+%Y-%m-%d %H:%M:%S")
	until := runCmd("date", "-d", "today 00:00:00", "+%Y-%m-%d %H:%M:%S")

	journalLog := runCmd("journalctl", "-u", "ssh.service", "--since", since, "--until", until, "-o", "short-iso", "--no-pager")

	// Failed logins - 详见 fail2ban
	writeLine(&b, "■ 失败登录\n")
	failLines := filterLines(journalLog, "Failed password", "Invalid user")
	if len(failLines) == 0 {
		writeLine(&b, "  今日无失败登录\n")
	} else {
		failIPs := make(map[string]int)
		for _, line := range failLines {
			ip := extractIP(line)
			if ip != "" {
				failIPs[ip]++
			}
		}
		total := len(failLines)
		writeLine(&b, fmt.Sprintf("  总失败: %d 次 | 涉及 IP: %d 个  (详见 fail2ban 封禁统计)\n", total, len(failIPs)))
	}

	// Successful logins - 按 IP 聚合
	writeLine(&b, "\n■ 成功登录\n")
	okLines := filterLines(journalLog, "Accepted")
	if len(okLines) == 0 {
		writeLine(&b, "  今日无成功登录\n")
	} else {
		type okEntry struct {
			count  int
			method string
		}
		okMap := make(map[string]*okEntry)
		okOrder := []string{}
		for _, line := range okLines {
			ip := extractIP(line)
			method := extractMethod(line)
			if ip == "" {
				continue
			}
			if _, ok := okMap[ip]; !ok {
				okMap[ip] = &okEntry{}
				okOrder = append(okOrder, ip)
			}
			okMap[ip].count++
			if method != "" {
				okMap[ip].method = method
			}
		}
		for _, ip := range okOrder {
			entry := okMap[ip]
			writeLine(&b, fmt.Sprintf("  %-15s  %d 次  方式: %s", ip, entry.count, entry.method))
		}
		total := len(okLines)
		writeLine(&b, fmt.Sprintf("  总成功: %d 次\n", total))
	}

	return b.String()
}

func filterLines(output string, keywords ...string) []string {
	var result []string
	for _, line := range splitLines(output) {
		for _, kw := range keywords {
			if strings.Contains(line, kw) {
				result = append(result, line)
				break
			}
		}
	}
	return result
}

func extractIP(line string) string {
	fields := strings.Fields(line)
	for i, f := range fields {
		if f == "from" && i+1 < len(fields) {
			ip := fields[i+1]
			ip = strings.TrimRight(ip, ".")
			return ip
		}
	}
	return ""
}

func extractMethod(line string) string {
	fields := strings.Fields(line)
	for i, f := range fields {
		if f == "Accepted" && i+1 < len(fields) {
			return fields[i+1]
		}
	}
	return ""
}

func extractTimestamp(line string) string {
	fields := strings.Fields(line)
	if len(fields) >= 2 {
		return fields[0] + " " + fields[1]
	}
	return ""
}
