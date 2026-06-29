// Server Report - Linux 服务器日报程序
// 博客: https://rongyan.cc
// 说明: https://rongyan.cc/code/server-report.html
//
package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	configPath := findConfigPath()
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--init":
			cmdInit(configPath)
			return
		case "--run-once":
			cmdRunOnce(configPath)
			return
		case "--subnet-scan":
			checkSubnetBan()
			fmt.Println("子网封禁扫描完成")
			return
		case "--help", "-h":
			printHelp()
			return
		case "--version", "-v":
			fmt.Println("server-report v1.0")
			return
		default:
			if strings.HasSuffix(os.Args[1], ".yml") || strings.HasSuffix(os.Args[1], ".yaml") {
				configPath = os.Args[1]
			}
		}
	}
	cmdRunOnce(configPath)
	cfg, _ := LoadConfig(configPath)
	if cfg != nil && cfg.Security.SubnetBan.Enabled {
		checkSubnetBan()
	}
}

func findConfigPath() string {
	// 优先读取同目录下的 config.yml
	exe, _ := os.Executable()
	if exe != "" {
		dir := exe
		for i := len(exe) - 1; i >= 0; i-- {
			if exe[i] == '/' {
				dir = exe[:i+1]
				break
			}
		}
		candidate := dir + "config.yml"
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	// 同目录没有，尝试当前工作目录
	if _, err := os.Stat("config.yml"); err == nil {
		return "config.yml"
	}
	// 最后回退到 /etc/server-report.yml
	return "/etc/server-report.yml"
}

func printHelp() {
	fmt.Println(`server-report - 服务器日报工具

用法:
  server-report                   使用 /etc/server-report.yml 发送日报
  server-report --init           初始化（安装 cron、建立基线）
  server-report --run-once       立即发送一次日报
  server-report --subnet-scan    手动扫描并封禁恶意子网
  server-report --help           显示帮助
  server-report --version        显示版本`)
}

func cmdInit(configPath string) {
	cfg := &Config{}
	if err := cfg.WriteDefaultConfig(configPath); err != nil {
		fmt.Printf("写入默认配置失败: %v\n", err)
		os.Exit(1)
	}
	os.MkdirAll("/var/lib/server-report", 0755)
	runCmd("/usr/local/lib/server-report/report-modules/06-services.sh")
	runCmd("/usr/local/lib/server-report/report-modules/07-changes.sh")
	runCmd("/usr/local/lib/server-report/report-modules/08-security.sh")
	runCmd("touch", "/var/lib/server-report/banned-subnets.txt")

	cronContent := "# 每天 00:01 发送前一天的服务器日报\n1 0 * * * root /data/server-report/server-report\n"
	writeFile("/etc/cron.d/server-report", cronContent)
	subnetCron := "# 每30分钟检查一次子网封禁\n*/30 * * * * root /data/server-report/server-report --subnet-scan\n"
	writeFile("/etc/cron.d/ban-subnet", subnetCron)

	fmt.Println("初始化完成:")
	fmt.Printf("  配置文件: %s\n", configPath)
	fmt.Println("  基线: 已建立")
	fmt.Println("  cron: 已安装")
	fmt.Println("  请编辑配置文件后执行: server-report --run-once")
}

func cmdRunOnce(configPath string) {
	cfg, err := LoadConfig(configPath)
	if err != nil {
		fmt.Printf("加载配置失败: %v\n", err)
		return
	}

	modules := map[string]func() string{
		"system":   collectSystem,
		"ssh_auth": collectSSHAuth,
		"fail2ban": collectFail2ban,
		"network":  collectNetwork,
		"firewall": collectFirewall,
		"services": collectServices,
		"changes":  collectChanges,
		"security": collectSecurity,
	}

	report := buildReport(cfg, modules)
	yesterday, _ := cfg.ReportDate()
	subject := cfg.BuildSubject(yesterday)

	if err := sendMail(cfg, subject, report); err != nil {
		fmt.Printf("发送邮件失败: %v\n", err)
		return
	}
	fmt.Printf("日报已发送: %s → %s\n", yesterday, runCmd("date", "+%Y-%m-%d"))
}
