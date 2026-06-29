# server-report — Linux 服务器日报程序

每天自动把系统安全状况发送到你的邮箱。支持 SSH 登录审计、fail2ban 封禁统计、IP 归属地中文显示、子网自动封段等功能。

详细说明: <a href="https://rongyan.cc/code/server-report.html" target="_blank">https://rongyan.cc/code/server-report.html</a>

---

## 目录

```
go/            ← Go 源代码（可自行编译修改）
gosrc/         ← Linux amd64 二进制 + 示例配置
```

## 快速部署

### 1. 下载

从 `gosrc/` 下载 `server-report` 二进制和 `config.sample.yml`。

### 2. 配置

把 `config.sample.yml` 改名为 `config.yml`，编辑 SMTP 信息、服务器名称、SSH 端口。

### 3. 运行

```bash
# server-report 和 config.yml 必须放在同一目录
./server-report --run-once
```

如果能收到邮件，说明部署成功。

### 4. 设置定时任务

每天 00:01 自动发送：

```bash
crontab -e
# 添加一行：
1 0 * * * /path/to/server-report
```

## 配置文件路径说明

**v1.0.3+（2026-06-29 更新）：** 二进制优先读取同目录下的 `config.yml`，不再需要放到 `/etc/` 下。两个文件放一起即可：

```
/data/server-report/
├── server-report
└── config.yml
```

## 命令

| 命令 | 作用 |
|------|------|
| `server-report --run-once` | 立即发送一次日报 |
| `server-report --subnet-scan` | 手动扫描并封禁恶意子网 |
| `server-report --version` | 查看版本 |
| `server-report --help` | 查看帮助 |

## 自行编译

```bash
cd go
GOOS=linux GOARCH=amd64 go build -o ../gosrc/server-report .
```

## 效果截图

日报包含以下内容：

- 系统健康（CPU / 内存 / 磁盘）
- SSH 登录审计（失败统计 + 成功 IP 聚合）
- fail2ban 封禁列表（IP + 归属地 + 尝试用户 + 失败次数）
- 子网自动封段
- 系统服务变更追踪
- 安全检测（SUID / 异常进程 / 系统错误日志）

---

MIT License
