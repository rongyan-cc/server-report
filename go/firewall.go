// Server Report - Linux 服务器日报程序
// 博客: https://rongyan.cc
// 说明: https://rongyan.cc/code/server-report.html
//
package main

import (
	"fmt"
	"strings"
	"strconv"
	"sort"
)

func collectFirewall() string {
	var b strings.Builder

	writeLine(&b, separator)
	writeLine(&b, " 防火墙 (UFW) 拦截统计")
	writeLine(&b, separator, "\n")

	ufwLog := runCmd("cat", "/var/log/ufw.log")
	if ufwLog == "" {
		writeLine(&b, "  UFW 日志未找到\n")
		return b.String()
	}

	yesterday := runCmd("date", "-d", "yesterday", "+%b %e")

	var totalBlocks int
	srcCount := make(map[string]int)
	dstCount := make(map[string]int)

	for _, line := range splitLines(ufwLog) {
		if !strings.Contains(line, yesterday) && !strings.Contains(line, strings.TrimSpace(yesterday)) {
			continue
		}
		if strings.Contains(line, "UFW BLOCK") {
			totalBlocks++

			src := extractFieldValue(line, "SRC=")
			dpt := extractFieldValue(line, "DPT=")
			proto := extractFieldValue(line, "PROTO=")

			if src != "" {
				srcCount[src]++
			}
			if dpt != "" {
				key := dpt + "/" + proto
				dstCount[key]++
			}
		}
	}

	writeLine(&b, fmt.Sprintf("  今日拦截总数: %d 次\n", totalBlocks))

	if totalBlocks > 0 {
		writeLine(&b, "■ 被拦截最多的来源 IP\n")
		for _, entry := range sortMapDesc(srcCount, 10) {
			writeLine(&b, fmt.Sprintf("  %s  (%d 次)\n", entry.key, entry.val))
		}

		writeLine(&b, "\n■ 被拦截最多的目标端口\n")
		for _, entry := range sortMapDesc(dstCount, 10) {
			writeLine(&b, fmt.Sprintf("  %s  (%d 次)\n", entry.key, entry.val))
		}
	}

	writeLine(&b, "")
	return b.String()
}

type kvEntry struct {
	key string
	val int
}

func sortMapDesc(m map[string]int, limit int) []kvEntry {
	var entries []kvEntry
	for k, v := range m {
		entries = append(entries, kvEntry{k, v})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].val > entries[j].val
	})
	if len(entries) > limit {
		entries = entries[:limit]
	}
	return entries
}

func extractFieldValue(line, prefix string) string {
	idx := strings.Index(line, prefix)
	if idx == -1 {
		return ""
	}
	start := idx + len(prefix)
	end := start
	for end < len(line) && line[end] != ' ' {
		end++
	}
	return line[start:end]
}

func strconvItoa(n int) string {
	return strconv.Itoa(n)
}
