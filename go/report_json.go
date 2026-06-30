// Server Report - Linux 服务器日报程序
// 博客: https://rongyan.cc
// 说明: https://rongyan.cc/code/server-report.html
//
package main

type APIReport struct {
	Status    string           `json:"status"`
	Server    string           `json:"server"`
	Timestamp string           `json:"timestamp"`
	Date      string           `json:"date"`
	Sections  []APISection     `json:"sections"`
}

type APISection struct {
	ID    string      `json:"id"`
	Title string      `json:"title"`
	Type  string      `json:"type"`
	Data  interface{} `json:"data"`
}

// ── 系统健康 ──
type SystemData struct {
	Hostname string      `json:"hostname"`
	IP       string      `json:"ip"`
	Uptime   string      `json:"uptime"`
	Load     string      `json:"load"`
	CPU      string      `json:"cpu"`
	Memory   MemoryInfo  `json:"memory"`
	Disks    []DiskInfo  `json:"disks"`
}

type MemoryInfo struct {
	Used     string `json:"used"`
	Total    string `json:"total"`
	Percent  int    `json:"percent"`
}

type DiskInfo struct {
	Mount   string `json:"mount"`
	Used    string `json:"used"`
	Total   string `json:"total"`
	Percent int    `json:"percent"`
}

// ── SSH 登录审计 ──
type SSHAuthData struct {
	FailedTotal  int              `json:"failed_total"`
	FailedIPs    int              `json:"failed_ips"`
	FailedDetail []FailedIPInfo   `json:"failed_detail"`
	SuccessTotal int              `json:"success_total"`
	SuccessList  []SuccessIPInfo  `json:"success_list"`
}

type FailedIPInfo struct {
	IP              string   `json:"ip"`
	Count           int      `json:"count"`
	Users           []string `json:"users"`
	First           string   `json:"first"`
	Last            string   `json:"last"`
	LocationCN      string   `json:"location_cn"`
	LocationEN      string   `json:"location_en"`
	LocationDetail  string   `json:"location_detail,omitempty"`
}

type SuccessIPInfo struct {
	IP     string `json:"ip"`
	Count  int    `json:"count"`
	Method string `json:"method"`
}

// ── fail2ban ──
type Fail2banData struct {
	TotalBanned    int           `json:"total_banned"`
	CurrentBanned  int           `json:"current_banned"`
	BannedIPs      []BannedIP    `json:"banned_ips"`
	BannedSubnets  []SubnetInfo  `json:"banned_subnets"`
}

type BannedIP struct {
	IP              string   `json:"ip"`
	LocationCN      string   `json:"location_cn"`
	LocationEN      string   `json:"location_en"`
	LocationDetail  string   `json:"location_detail,omitempty"`
	BanTime         string   `json:"ban_time"`
	Users           []string `json:"users"`
	Attempts        int      `json:"attempts"`
	Subnet          string   `json:"subnet"`
}

type SubnetInfo struct {
	Subnet     string `json:"subnet"`
	IPCount    int    `json:"ip_count"`
}

// ── 网络 ──
type NetworkData struct {
	Established   int            `json:"established"`
	TimeWait      int            `json:"time_wait"`
	Total         int            `json:"total"`
	TopConns      []ConnInfo     `json:"top_conns"`
	ListeningP    []PortInfo     `json:"listening_ports"`
	Traffic       []TrafficInfo  `json:"traffic"`
}

type ConnInfo struct {
	Count int    `json:"count"`
	Dest  string `json:"dest"`
}

type PortInfo struct {
	Port string `json:"port"`
	Name string `json:"name"`
}

type TrafficInfo struct {
	Iface string `json:"iface"`
	Rx    string `json:"rx"`
	Tx    string `json:"tx"`
}

// ── 防火墙 ──
type FirewallData struct {
	TotalBlocks int        `json:"total_blocks"`
	TopSrc      []KVEntry  `json:"top_src"`
	TopDst      []KVEntry  `json:"top_dst"`
}

type KVEntry struct {
	Key string `json:"key"`
	Val int    `json:"val"`
}

// ── 服务 ──
type ServicesData struct {
	Running  []ServiceInfo `json:"running"`
	Total    int           `json:"total"`
}

type ServiceInfo struct {
	Name string `json:"name"`
	Desc string `json:"desc"`
}

// ── 变更 ──
type ChangesData struct {
	Packages []string `json:"packages"`
}

// ── 安全 ──
type SecurityData struct {
	SUIDOK      bool         `json:"suid_ok"`
	NewUsers    []string     `json:"new_users"`
	Suspicious  []string     `json:"suspicious"`
	OnlineUsers []string     `json:"online_users"`
	Errors      []string     `json:"errors"`
}
