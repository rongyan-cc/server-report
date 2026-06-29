// Server Report - Linux 服务器日报程序
// 博客: https://rongyan.cc
// 说明: https://rongyan.cc/code/server-report.html
//
package main

import (
	"fmt"
	"os"
	"io/ioutil"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	SMTP     SMTPConfig     `yaml:"smtp"`
	Mail     MailConfig     `yaml:"mail"`
	Schedule ScheduleConfig `yaml:"schedule"`
	Modules  ModuleConfig   `yaml:"modules"`
	Security SecurityConfig `yaml:"security"`
}

type ServerConfig struct {
	Name    string `yaml:"name"`
	IP      string `yaml:"ip"`
	SSHPort int    `yaml:"ssh_port"`
	APIKey  string `yaml:"api_key"`
	APIPort int    `yaml:"api_port"`
}

type SMTPConfig struct {
	Server   string `yaml:"server"`
	Port     int    `yaml:"port"`
	SSL      bool   `yaml:"ssl"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
}

type MailConfig struct {
	To      string `yaml:"to"`
	From    string `yaml:"from"`
	Subject string `yaml:"subject"`
}

type ScheduleConfig struct {
	ReportAt string `yaml:"report_at"`
}

type ModuleConfig struct {
	System    bool `yaml:"system"`
	SSHAuth   bool `yaml:"ssh_auth"`
	Fail2ban  bool `yaml:"fail2ban"`
	Network   bool `yaml:"network"`
	Firewall  bool `yaml:"firewall"`
	Services  bool `yaml:"services"`
	Changes   bool `yaml:"changes"`
	Security  bool `yaml:"security"`
}

type Fail2banConfig struct {
	Enabled        bool   `yaml:"enabled"`
	Maxretry       int    `yaml:"maxretry"`
	Bantime        int    `yaml:"bantime"`
	Findtime       int    `yaml:"findtime"`
	Journalmatch   string `yaml:"journalmatch"`
}

type SubnetBanConfig struct {
	Enabled       bool `yaml:"enabled"`
	Threshold     int  `yaml:"threshold"`
	CheckInterval int  `yaml:"check_interval"`
}

type SecurityConfig struct {
	Fail2ban  Fail2banConfig  `yaml:"fail2ban"`
	SubnetBan SubnetBanConfig `yaml:"subnet_ban"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}
	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}
	if cfg.Server.IP == "" {
		ip := runCmd("curl", "-s", "-4", "https://one.one.one.one/cdn-cgi/trace")
		if ip != "" {
			for _, line := range splitLines(ip) {
				if hasPrefix(line, "ip=") {
					cfg.Server.IP = trimPrefix(line, "ip=")
					break
				}
			}
		}
	}
	return cfg, nil
}

func (cfg *Config) BuildSubject(date string) string {
	s := cfg.Mail.Subject
	s = replaceAll(s, "{name}", cfg.Server.Name)
	s = replaceAll(s, "{ip}", cfg.Server.IP)
	s = replaceAll(s, "{date}", date)
	return s
}

func (cfg *Config) ReportDate() (string, string) {
	yesterday := runCmd("date", "-d", "yesterday", "+%Y-%m-%d")
	today := runCmd("date", "+%Y-%m-%d")
	if yesterday == "" {
		yesterday = "1970-01-01"
	}
	if today == "" {
		today = "1970-01-02"
	}
	return yesterday, today
}

func (cfg *Config) WriteDefaultConfig(path string) error {
	defaultCfg := `server:
  name: "myserver"
  ip: ""
  ssh_port: 22

smtp:
  server: "mail.example.com"
  port: 465
  ssl: true
  user: "user@example.com"
  password: "your-password"

mail:
  to: "admin@example.com"
  from: "user@example.com"
  subject: "服务器日报 - {name} / {ip} - {date}"

schedule:
  report_at: "00:01"

modules:
  system: true
  ssh_auth: true
  fail2ban: true
  network: true
  firewall: true
  services: true
  changes: true
  security: true

security:
  fail2ban:
    enabled: true
    maxretry: 1
    bantime: -1
    findtime: 600
    journalmatch: "_SYSTEMD_UNIT=ssh.service + _COMM=sshd"
  subnet_ban:
    enabled: true
    threshold: 2
    check_interval: 30
`
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return ioutil.WriteFile(path, []byte(defaultCfg), 0600)
	}
	return nil
}
