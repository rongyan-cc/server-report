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

func buildReport(cfg *Config, modules map[string]func() string) string {
	var b strings.Builder
	hostname := runCmd("hostname")
	yesterday, today := cfg.ReportDate()

	writeLine(&b, "========================================")
	writeLine(&b, fmt.Sprintf(" 服务器日报"))
	writeLine(&b, fmt.Sprintf(" %s → %s", yesterday, today))
	writeLine(&b, fmt.Sprintf(" 主机: %s | IP: %s", hostname, cfg.Server.IP))
	writeLine(&b, "========================================")
	writeLine(&b, "")

	order := []string{"system", "ssh_auth", "fail2ban", "network", "firewall", "services", "changes", "security"}
	enabled := map[string]bool{
		"system":   cfg.Modules.System,
		"ssh_auth": cfg.Modules.SSHAuth,
		"fail2ban": cfg.Modules.Fail2ban,
		"network":  cfg.Modules.Network,
		"firewall": cfg.Modules.Firewall,
		"services": cfg.Modules.Services,
		"changes":  cfg.Modules.Changes,
		"security": cfg.Modules.Security,
	}
	for _, name := range order {
		if enabled[name] && modules[name] != nil {
			b.WriteString(modules[name]())
		}
	}

	writeLine(&b, "========================================")
	writeLine(&b, fmt.Sprintf(" 报告生成时间: %s", runCmd("date", "+%Y-%m-%d %H:%M:%S")))
	writeLine(&b, "========================================")

	return b.String()
}
