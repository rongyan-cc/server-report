// Server Report - Linux 服务器日报程序
// 博客: https://rongyan.cc
// 说明: https://rongyan.cc/code/server-report.html
//
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
	"io/ioutil"
)

func runCmd(name string, args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func runCmdWithStderr(name string, args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func hasCmd(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func writeLine(buf *strings.Builder, args ...string) {
	for _, s := range args {
		buf.WriteString(s)
	}
	buf.WriteString("\n")
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(strings.TrimRight(s, "\n"), "\n")
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func trimPrefix(s, prefix string) string {
	if hasPrefix(s, prefix) {
		return s[len(prefix):]
	}
	return s
}

func replaceAll(s, old, new string) string {
	return strings.ReplaceAll(s, old, new)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func baselineExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func readFile(path string) string {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func writeFile(path, content string) {
	ioutil.WriteFile(path, []byte(content), 0644)
}

func appendFile(path, content string) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintln(f, content)
}

func extractUser(line string) string {
	lineLower := strings.ToLower(line)
	if strings.Contains(lineLower, "invalid user") {
		for i := 0; i < len(line)-10; i++ {
			if strings.HasPrefix(line[i:], "Invalid user") || strings.HasPrefix(line[i:], "invalid user") {
				start := i + 12
				for start < len(line) && line[start] == ' ' {
					start++
				}
				end := start
				for end < len(line) && line[end] != ' ' {
					end++
				}
				user := line[start:end]
				if user != "from" && !strings.Contains(user, ".") {
					return user
				}
			}
		}
	}
	if strings.Contains(lineLower, "failed password") {
		for i := 0; i < len(lineLower)-15; i++ {
			if strings.Contains(lineLower[i:], "failed password for") {
				prefix := "failed password for"
				if strings.Contains(lineLower[i:], "invalid user") {
					prefix = "failed password for invalid user"
				}
				idx := strings.Index(strings.ToLower(line[i:]), prefix)
				if idx != -1 {
					start := i + idx + len(prefix)
					for start < len(line) && line[start] == ' ' {
						start++
					}
					end := start
					for end < len(line) && line[end] != ' ' {
						end++
					}
					return line[start:end]
				}
			}
		}
	}
	return ""
}

const separator = "========================================"
