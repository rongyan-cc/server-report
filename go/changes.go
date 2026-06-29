// Server Report - Linux 服务器日报程序
// 博客: https://rongyan.cc
// 说明: https://rongyan.cc/code/server-report.html
//
package main

import (
	"fmt"
	"strings"
)

func collectChanges() string {
	var b strings.Builder

	writeLine(&b, separator)
	writeLine(&b, " 系统变更记录")
	writeLine(&b, separator, "\n")

	writeLine(&b, "■ 软件包变更\n")
	aptLog := runCmd("cat", "/var/log/apt/history.log")
	yesterday := runCmd("date", "-d", "yesterday", "+%Y-%m-%d")
	hasChanges := false
	inBlock := false
	for _, line := range splitLines(aptLog) {
		if strings.Contains(line, yesterday) || inBlock {
			if strings.HasPrefix(line, "Start-Date") {
				inBlock = true
			}
			if inBlock && (strings.Contains(line, "Install:") || strings.Contains(line, "Remove:") || strings.Contains(line, "Upgrade:")) {
				writeLine(&b, fmt.Sprintf("    %s\n", strings.TrimSpace(line)))
				hasChanges = true
			}
			if strings.HasPrefix(line, "End-Date") {
				inBlock = false
			}
		}
	}
	if !hasChanges {
		writeLine(&b, "    昨日无软件包变更\n")
	}

	writeLine(&b, "\n■ 用户/组变更\n")
	passwd := runCmd("getent", "passwd")
	var currentUsers []string
	for _, line := range splitLines(passwd) {
		if !strings.Contains(line, "nologin") && !strings.Contains(line, "/sync") && !strings.Contains(line, "/false") {
			currentUsers = append(currentUsers, line)
		}
	}
	userBase := "/var/lib/server-report/users-baseline.txt"
	if baselineExists(userBase) {
		oldData := readFile(userBase)
		oldUsers := strings.Split(oldData, "\n")
		oldMap := make(map[string]bool)
		for _, u := range oldUsers {
			oldMap[u] = true
		}
		hasUserChange := false
		for _, u := range currentUsers {
			fields := strings.Split(u, ":")
			if len(fields) >= 5 && !oldMap[u] {
				writeLine(&b, fmt.Sprintf("  ✨ 新用户: %s (%s)\n", fields[0], fields[4]))
				hasUserChange = true
			}
		}
		if !hasUserChange {
			writeLine(&b, "    昨日无用户/组变更\n")
		}
	} else {
		writeLine(&b, "    首次运行，已建立用户基线\n")
	}
	writeFile(userBase, strings.Join(currentUsers, "\n"))

	writeLine(&b, "\n■ 防火墙规则变更\n")
	ufwRules := runCmd("ufw", "status", "numbered")
	ufwBase := "/var/lib/server-report/ufw-baseline.txt"
	if baselineExists(ufwBase) {
		oldRules := readFile(ufwBase)
		if oldRules != ufwRules {
			writeLine(&b, "  ⚠️ 防火墙规则有变更\n")
			writeLine(&b, "  当前规则:\n")
			for _, line := range splitLines(ufwRules) {
				if strings.Contains(line, "ALLOW") || strings.Contains(line, "DENY") {
					writeLine(&b, fmt.Sprintf("    %s\n", strings.TrimSpace(line)))
				}
			}
		} else {
			writeLine(&b, "  防火墙规则无变更\n")
		}
	} else {
		writeLine(&b, "  首次运行，基线已建立\n")
	}
	writeFile(ufwBase, ufwRules)

	return b.String()
}
