// Server Report - Linux 服务器日报程序
// 博客: https://rongyan.cc
// 说明: https://rongyan.cc/code/server-report.html
//
package main

import (
	"fmt"
	"strings"
)

func collectServices() string {
	var b strings.Builder
	writeLine(&b, separator)
	writeLine(&b, " 运行中的系统服务")
	writeLine(&b, separator, "")

	serviceList := runCmd("systemctl", "list-units", "--type=service", "--state=running", "--no-legend")
	lines := splitLines(serviceList)
	baselinePath := "/var/lib/server-report/services-baseline.txt"

	writeLine(&b, "■ 当前运行服务列表\n")
	count := 0
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 4 {
			name := fields[0]
			desc := strings.Join(fields[3:], " ")
			writeLine(&b, fmt.Sprintf("  %-30s %s", name, desc))
			count++
		}
	}
	writeLine(&b, fmt.Sprintf("  —— 共 %d 个运行中的服务 ——", count))

	current := runCmd("systemctl", "list-unit-files", "--type=service", "--no-legend")

	writeLine(&b, "■ 服务新增/移除检测（与基线对比）\n")
	if baselineExists(baselinePath) {
		baseline := readFile(baselinePath)
		currentLines := make(map[string]bool)
		for _, line := range splitLines(current) {
			fields := strings.Fields(line)
			if len(fields) > 0 {
				currentLines[fields[0]] = true
			}
		}
		baselineLines := make(map[string]bool)
		for _, line := range splitLines(baseline) {
			fields := strings.Fields(line)
			if len(fields) > 0 {
				baselineLines[fields[0]] = true
			}
		}
		hasChange := false
		for name := range currentLines {
			if !baselineLines[name] {
				writeLine(&b, fmt.Sprintf("  ➕ 新增: %s", name))
				hasChange = true
			}
		}
		for name := range baselineLines {
			if !currentLines[name] {
				writeLine(&b, fmt.Sprintf("  ➖ 移除: %s", name))
				hasChange = true
			}
		}
		if !hasChange {
			writeLine(&b, "  与基线一致，无新增或移除的服务")
		}
	} else {
		writeLine(&b, "  首次运行，基线已建立")
	}
	writeFile(baselinePath, current)

	return b.String()
}
