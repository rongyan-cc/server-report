// Server Report - Linux 服务器日报程序
// 博客: https://rongyan.cc
// 说明: https://rongyan.cc/code/server-report.html
//
package main

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os/exec"
	"strings"
)

func sendMail(cfg *Config, subject, body string) error {
	encodedSubject := "=?UTF-8?B?" + base64.StdEncoding.EncodeToString([]byte(subject)) + "?="

	tmpFile, err := ioutil.TempFile("", "report-*.txt")
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.WriteString(body)
	tmpFile.Close()

	script := fmt.Sprintf(`
import smtplib, ssl
from email.mime.text import MIMEText
from email.header import Header
with open("%s") as f:
    content = f.read()
msg = MIMEText(content, "plain", "utf-8")
msg["Subject"] = "%s"
msg["From"] = Header("%s", "utf-8").encode() + " <%s>"
msg["To"] = "%s"
ctx = ssl.create_default_context()
ctx.check_hostname = False
ctx.verify_mode = ssl.CERT_NONE
s = smtplib.SMTP_SSL("%s", %d, context=ctx, timeout=30)
try:
    s.login("%s", "%s")
    s.sendmail("%s", ["%s"], msg.as_string())
finally:
    s.quit()
`, tmpPath, encodedSubject, cfg.Server.Name, cfg.Mail.From, cfg.Mail.To,
		cfg.SMTP.Server, cfg.SMTP.Port,
		cfg.SMTP.User, cfg.SMTP.Password,
		cfg.Mail.From, cfg.Mail.To)

	cmd := exec.Command("python3", "-c", script)
	out, err := cmd.CombinedOutput()
	exec.Command("rm", "-f", tmpPath).Run()
	if err != nil {
		return fmt.Errorf("发送邮件失败: %w\n%s", err, string(out))
	}
	return nil
}

func writeSectionTitle(b *strings.Builder, title string) {
	writeLine(b, separator)
	writeLine(b, fmt.Sprintf(" %s", title))
	writeLine(b, separator)
	writeLine(b, "")
}

func mailSystem(b *strings.Builder, d SystemData) {
	writeLine(b, fmt.Sprintf("  运行时间: %s", d.Uptime))
	writeLine(b, fmt.Sprintf("  系统负载: %s", d.Load))
	writeLine(b, fmt.Sprintf("  CPU: %s", d.CPU))
	writeLine(b, fmt.Sprintf("  内存: %s / %s (%d%%)", d.Memory.Used, d.Memory.Total, d.Memory.Percent))
	for _, disk := range d.Disks {
		writeLine(b, fmt.Sprintf("  磁盘 %s: %s / %s (%d%%)", disk.Mount, disk.Used, disk.Total, disk.Percent))
	}
	writeLine(b, "")
}

func mailSSHAuth(b *strings.Builder, d SSHAuthData) {
	writeLine(b, fmt.Sprintf("  失败次数: %d | 来源 IP: %d | 成功登录: %d", d.FailedTotal, d.FailedIPs, d.SuccessTotal))
	if len(d.SuccessList) > 0 {
		writeLine(b, "")
		writeLine(b, "  ■ 成功登录")
		for _, s := range d.SuccessList {
			line := fmt.Sprintf("    %-15s  %d 次", s.IP, s.Count)
			if s.Method != "" {
				line += fmt.Sprintf("  方式: %s", s.Method)
			}
			writeLine(b, line)
		}
	}
	if len(d.FailedDetail) > 0 {
		writeLine(b, "")
		writeLine(b, fmt.Sprintf("  ■ 失败详情 (%d 个 IP)", len(d.FailedDetail)))
		for _, f := range d.FailedDetail {
			loc := f.LocationCN
			if f.LocationDetail != "" && loc == "中国" {
				loc = f.LocationDetail
			}
			users := strings.Join(f.Users, ",")
			bt := f.Last
			if len(bt) > 16 {
				bt = bt[:16]
			}
			writeLine(b, fmt.Sprintf("    %-15s  %-12s  %s  %s  %d次", f.IP, loc, bt, users, f.Count))
		}
	}
	writeLine(b, "")
}

func mailFail2ban(b *strings.Builder, d Fail2banData) {
	writeLine(b, fmt.Sprintf("  累计封禁: %d | 本日封禁: %d", d.TotalBanned, d.CurrentBanned))
	if len(d.BannedIPs) > 0 {
		writeLine(b, "")
		writeLine(b, "  ■ 被封禁 IP")
		for _, ip := range d.BannedIPs {
			loc := ip.LocationCN
			if ip.LocationDetail != "" && loc == "中国" {
				loc = ip.LocationDetail
			}
			bt := ip.BanTime
			if len(bt) > 16 {
				bt = bt[:16]
			}
			users := strings.Join(ip.Users, ",")
			writeLine(b, fmt.Sprintf("    %-15s  %-12s  封禁: %s  用户: %s  失败: %d次", ip.IP, loc, bt, users, ip.Attempts))
		}
	}
	if len(d.BannedSubnets) > 0 {
		writeLine(b, "")
		writeLine(b, "  ■ 已封 C 段子网")
		for _, sn := range d.BannedSubnets {
			writeLine(b, fmt.Sprintf("    %s  %d 个 IP", sn.Subnet, sn.IPCount))
		}
	}
	writeLine(b, "")
}

func mailNetwork(b *strings.Builder, d NetworkData) {
	writeLine(b, fmt.Sprintf("  已建立: %d | 等待: %d | 总计: %d", d.Established, d.TimeWait, d.Total))
	if len(d.TopConns) > 0 {
		writeLine(b, "")
		writeLine(b, "  ■ 对外连接 TOP 5")
		for _, c := range d.TopConns {
			writeLine(b, fmt.Sprintf("    %s  %d", c.Dest, c.Count))
		}
	}
	if len(d.ListeningP) > 0 {
		writeLine(b, "")
		writeLine(b, "  ■ 监听端口")
		for _, p := range d.ListeningP {
			writeLine(b, fmt.Sprintf("    %s  %s", p.Port, p.Name))
		}
	}
	if len(d.Traffic) > 0 {
		writeLine(b, "")
		writeLine(b, "  ■ 网卡流量")
		for _, t := range d.Traffic {
			writeLine(b, fmt.Sprintf("    %s  入: %s  出: %s", t.Iface, t.Rx, t.Tx))
		}
	}
	writeLine(b, "")
}

func mailFirewall(b *strings.Builder, d FirewallData) {
	writeLine(b, fmt.Sprintf("  拦截次数: %d", d.TotalBlocks))
	if len(d.TopSrc) > 0 {
		writeLine(b, "")
		writeLine(b, "  ■ 来源 IP Top 5")
		for _, s := range d.TopSrc {
			writeLine(b, fmt.Sprintf("    %s  %d 次", s.Key, s.Val))
		}
	}
	writeLine(b, "")
}

func mailServices(b *strings.Builder, d ServicesData) {
	writeLine(b, fmt.Sprintf("  共 %d 个服务", d.Total))
	for i, s := range d.Running {
		if i >= 25 {
			break
		}
		writeLine(b, fmt.Sprintf("    %-30s %s", s.Name, s.Desc))
	}
	if len(d.Running) > 25 {
		writeLine(b, fmt.Sprintf("    ...及其他 %d 个", len(d.Running)-25))
	}
	writeLine(b, "")
}

func mailChanges(b *strings.Builder, d ChangesData) {
	if len(d.Packages) > 0 {
		for i, p := range d.Packages {
			if i >= 10 {
				break
			}
			writeLine(b, fmt.Sprintf("    %s", p))
		}
		if len(d.Packages) > 10 {
			writeLine(b, fmt.Sprintf("    ...及其他 %d 条", len(d.Packages)-10))
		}
	} else {
		writeLine(b, "  昨日无软件包变更")
	}
	writeLine(b, "")
}

func mailSecurity(b *strings.Builder, d SecurityData) {
	if !d.SUIDOK {
		writeLine(b, "  ⚠ SUID 文件有变更，可能存在提权风险")
	}
	if len(d.NewUsers) > 0 {
		writeLine(b, fmt.Sprintf("  ⚠ 新用户: %s", strings.Join(d.NewUsers, ", ")))
	}
	if len(d.Suspicious) > 0 {
		writeLine(b, fmt.Sprintf("  ⚠ 可疑进程: %s", strings.Join(d.Suspicious, ", ")))
	}
	if len(d.OnlineUsers) > 0 {
		writeLine(b, fmt.Sprintf("  在线用户: %s", strings.Join(d.OnlineUsers, ", ")))
	}
	if len(d.Errors) > 0 {
		writeLine(b, fmt.Sprintf("  系统错误日志 (%d 条):", len(d.Errors)))
		for i, e := range d.Errors {
			if i >= 5 {
				break
			}
			writeLine(b, fmt.Sprintf("    %s", e))
		}
	}
	writeLine(b, "")
}

// buildMailText 从 API 报告生成邮件正文文本
func buildMailText(report APIReport, cfg *Config) string {
	var b strings.Builder
	hostname := runCmd("hostname")

	writeLine(&b, "========================================")
	writeLine(&b, fmt.Sprintf(" 服务器日报"))
	writeLine(&b, fmt.Sprintf(" %s", report.Date))
	writeLine(&b, fmt.Sprintf(" 主机: %s | IP: %s", hostname, cfg.Server.IP))
	writeLine(&b, "========================================")
	writeLine(&b, "")

	enabled := map[string]bool{
		"system": cfg.Modules.System, "ssh_auth": cfg.Modules.SSHAuth,
		"fail2ban": cfg.Modules.Fail2ban, "network": cfg.Modules.Network,
		"firewall": cfg.Modules.Firewall, "services": cfg.Modules.Services,
		"changes": cfg.Modules.Changes, "security": cfg.Modules.Security,
	}

	for _, sec := range report.Sections {
		if !enabled[sec.ID] {
			continue
		}
		writeSectionTitle(&b, sec.Title)
		switch d := sec.Data.(type) {
		case SystemData:
			mailSystem(&b, d)
		case SSHAuthData:
			mailSSHAuth(&b, d)
		case Fail2banData:
			mailFail2ban(&b, d)
		case NetworkData:
			mailNetwork(&b, d)
		case FirewallData:
			mailFirewall(&b, d)
		case ServicesData:
			mailServices(&b, d)
		case ChangesData:
			mailChanges(&b, d)
		case SecurityData:
			mailSecurity(&b, d)
		}
	}

	writeLine(&b, "========================================")
	writeLine(&b, fmt.Sprintf(" 报告生成时间: %s", runCmd("date", "+%Y-%m-%d %H:%M:%S")))
	writeLine(&b, "========================================")

	return b.String()
}
