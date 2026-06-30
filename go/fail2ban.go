// Server Report - Linux 服务器日报程序
// 博客: https://rongyan.cc
// 说明: https://rongyan.cc/code/server-report.html
//
package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

func collectFail2ban() string {
	var b strings.Builder

	writeLine(&b, separator)
	writeLine(&b, " fail2ban 封禁统计")
	writeLine(&b, separator, "\n")

	if !hasCmd("fail2ban-client") {
		writeLine(&b, "  fail2ban 未安装\n")
		return b.String()
	}

	status := runCmd("fail2ban-client", "status", "sshd")

	totalBanned := ""
	currentBanned := ""
	bannedIPs := ""

	for _, line := range splitLines(status) {
		if strings.Contains(line, "Total banned") {
			fields := strings.Fields(line)
			if len(fields) > 0 {
				totalBanned = fields[len(fields)-1]
			}
		}
		if strings.Contains(line, "Currently banned") {
			fields := strings.Fields(line)
			if len(fields) > 0 {
				currentBanned = fields[len(fields)-1]
			}
		}
		if strings.Contains(line, "Banned IP list") {
			idx := strings.Index(line, ":")
			if idx != -1 {
				bannedIPs = strings.TrimSpace(line[idx+1:])
			}
		}
	}

	writeLine(&b, fmt.Sprintf("  累计封禁: %s 次", totalBanned))
	writeLine(&b, fmt.Sprintf("  当前封禁: %s 个 IP\n", currentBanned))

	if bannedIPs == "" {
		writeLine(&b, "  当前无 IP 被封禁\n")
		return b.String()
	}

	writeLine(&b, "■ 当前被封禁 IP\n")

	// Single journalctl call for all IPs
	journalAll := runCmd("journalctl", "-u", "ssh.service", "--since", "-7 days", "-o", "short-iso", "--no-pager")

	ips := strings.Fields(bannedIPs)
	for _, ip := range ips {
		if strings.Contains(ip, ":") {
			continue // skip IPv6
		}
		location := geoLookup(ip)
		banTime := getBanTime(ip)
		sshLines := filterLines(journalAll, ip)
		failLines := filterLinesFromSlice(sshLines, "Failed password", "Invalid user")

		if len(failLines) > 0 {
			userMap := make(map[string]bool)
			for _, l := range failLines {
				u := extractUser(l)
				if u != "" {
					userMap[u] = true
				}
			}
			var users []string
			for u := range userMap {
				users = append(users, u)
			}
			userStr := strings.Join(users, ",")
			writeLine(&b, fmt.Sprintf("  %s  %s  封禁: %s  用户: %s  失败: %d 次",
				ip, location, truncate(banTime, 16), userStr, len(failLines)))
		} else {
			writeLine(&b, fmt.Sprintf("  %s  %s  封禁: %s", ip, location, truncate(banTime, 16)))
		}
	}

	writeLine(&b, "")

	// Subnet bans - 从当前封禁列表实时计算
	subnetData := runCmd("cat", "/var/lib/server-report/banned-subnets.txt")
	writeLine(&b, "■ 已封禁的 C 段子网\n")

	// 获取当前被封禁的 IP 列表
	status = runCmd("fail2ban-client", "status", "sshd")
	bannedIPs = ""
	for _, line := range splitLines(status) {
		if strings.Contains(line, "Banned IP list") {
			idx := strings.Index(line, ":")
			if idx != -1 {
				bannedIPs = strings.TrimSpace(line[idx+1:])
			}
		}
	}

	subnetCount := make(map[string]int)
	if bannedIPs != "" {
		for _, ip := range strings.Fields(bannedIPs) {
			if strings.Contains(ip, ":") {
				continue
			}
			parts := strings.Split(ip, ".")
			if len(parts) >= 3 {
				subnet := parts[0] + "." + parts[1] + "." + parts[2] + ".0/24"
				subnetCount[subnet]++
			}
		}
	}

	hasSubnet := false
	for _, line := range splitLines(subnetData) {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "-") {
			continue
		}
		if strings.Contains(line, "/") {
			// 从实时封禁列表获取精确计数
			subnet := line
			if !strings.Contains(subnet, "(") {
				subnet = strings.Fields(line)[0]
			}
			if strings.Contains(subnet, "/") {
				currentCount := subnetCount[subnet]
				if currentCount > 0 {
					if !hasSubnet {
						hasSubnet = true
					}
					writeLine(&b, fmt.Sprintf("  %s  当前 %d 个 IP 被封", subnet, currentCount))
				}
			}
		}
	}

	if !hasSubnet {
		writeLine(&b, "  当前无子网封禁\n")
	}

	return b.String()
}

func filterLinesFromSlice(lines []string, keywords ...string) []string {
	var result []string
	for _, line := range lines {
		for _, kw := range keywords {
			if strings.Contains(line, kw) {
				result = append(result, line)
				break
			}
		}
	}
	return result
}

func filterLinesByIP(lines []string) map[string][]string {
	result := make(map[string][]string)
	for _, line := range lines {
		ip := extractIP(line)
		if ip != "" {
			result[ip] = append(result[ip], line)
		}
	}
	return result
}

func geoLookup(ip string) string {
	out := runCmd("geoiplookup", ip)
	for _, line := range splitLines(out) {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "GeoIP Country Edition:") {
			raw := strings.TrimSpace(strings.Replace(line, "GeoIP Country Edition:", "", 1))
			return translateCountry(raw)
		}
	}
	return ip
}

func getBanTime(ip string) string {
	log := runCmd("grep", "Ban "+ip, "/var/log/fail2ban.log")
	for _, line := range splitLines(log) {
		if strings.Contains(line, "Ban "+ip) {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				return fields[0] + " " + fields[1]
			}
		}
	}
	return ""
}

func translateCountry(raw string) string {
	rawLower := strings.ToLower(raw)
	if strings.Contains(rawLower, "guatemala") { return "危地马拉/Guatemala" }
	if strings.Contains(rawLower, "china") {
		if strings.Contains(rawLower, "hong") {
			return "香港/Hong Kong"
		}
		if strings.Contains(rawLower, "macau") {
			return "澳门/Macau"
		}
		return "中国/China"
	}
	if strings.Contains(rawLower, "hong kong") { return "香港/Hong Kong" }
	if strings.Contains(rawLower, "macau") { return "澳门/Macau" }
	if strings.Contains(rawLower, "taiwan") { return "台湾/Taiwan" }
	if strings.Contains(rawLower, "japan") { return "日本/Japan" }
	if strings.Contains(rawLower, "korea") { return "韩国/Korea" }
	if strings.Contains(rawLower, "singapore") { return "新加坡/Singapore" }
	if strings.Contains(rawLower, "indonesia") { return "印尼/Indonesia" }
	if strings.Contains(rawLower, "india") { return "印度/India" }
	if strings.Contains(rawLower, "vietnam") { return "越南/Vietnam" }
	if strings.Contains(rawLower, "thailand") { return "泰国/Thailand" }
	if strings.Contains(rawLower, "malaysia") { return "马来西亚/Malaysia" }
	if strings.Contains(rawLower, "philippines") { return "菲律宾/Philippines" }
	if strings.Contains(rawLower, "united states") { return "美国/USA" }
	if strings.Contains(rawLower, "canada") { return "加拿大/Canada" }
	if strings.Contains(rawLower, "brazil") { return "巴西/Brazil" }
	if strings.Contains(rawLower, "russia") { return "俄罗斯/Russia" }
	if strings.Contains(rawLower, "united kingdom") { return "英国/UK" }
	if strings.Contains(rawLower, "germany") { return "德国/Germany" }
	if strings.Contains(rawLower, "france") { return "法国/France" }
	if strings.Contains(rawLower, "netherlands") { return "荷兰/Netherlands" }
	if strings.Contains(rawLower, "australia") { return "澳大利亚/Australia" }
	if strings.Contains(rawLower, "poland") { return "波兰/Poland" }
	if strings.Contains(rawLower, "ukraine") { return "乌克兰/Ukraine" }
	if strings.Contains(rawLower, "sweden") { return "瑞典/Sweden" }
	if strings.Contains(rawLower, "norway") { return "挪威/Norway" }
	if strings.Contains(rawLower, "italy") { return "意大利/Italy" }
	if strings.Contains(rawLower, "spain") { return "西班牙/Spain" }
	if strings.Contains(rawLower, "turkey") { return "土耳其/Turkey" }
	if strings.Contains(rawLower, "iran") { return "伊朗/Iran" }
	if strings.Contains(rawLower, "israel") { return "以色列/Israel" }
	if strings.Contains(rawLower, "egypt") { return "埃及/Egypt" }
	if strings.Contains(rawLower, "south africa") { return "南非/South Africa" }
	if strings.Contains(rawLower, "argentina") { return "阿根廷/Argentina" }
	if strings.Contains(rawLower, "mexico") { return "墨西哥/Mexico" }
	if strings.Contains(rawLower, "nigeria") { return "尼日利亚/Nigeria" }
	if strings.Contains(rawLower, "bangladesh") { return "孟加拉/Bangladesh" }
	if strings.Contains(rawLower, "pakistan") { return "巴基斯坦/Pakistan" }
	if strings.Contains(rawLower, "romania") { return "罗马尼亚/Romania" }
	if strings.Contains(rawLower, "bulgaria") { return "保加利亚/Bulgaria" }
	if strings.Contains(rawLower, "belgium") { return "比利时/Belgium" }
	if strings.Contains(rawLower, "switzerland") { return "瑞士/Switzerland" }
	if strings.Contains(rawLower, "austria") { return "奥地利/Austria" }
	if strings.Contains(rawLower, "czech") { return "捷克/Czech" }
	if strings.Contains(rawLower, "finland") { return "芬兰/Finland" }
	if strings.Contains(rawLower, "denmark") { return "丹麦/Denmark" }
	if strings.Contains(rawLower, "portugal") { return "葡萄牙/Portugal" }
	if strings.Contains(rawLower, "greece") { return "希腊/Greece" }
	if strings.Contains(rawLower, "hungary") { return "匈牙利/Hungary" }
	if strings.Contains(rawLower, "ireland") { return "爱尔兰/Ireland" }
	if strings.Contains(rawLower, "new zealand") { return "新西兰/New Zealand" }
	if strings.Contains(rawLower, "kenya") { return "肯尼亚/Kenya" }
	if strings.Contains(rawLower, "morocco") { return "摩洛哥/Morocco" }
	if strings.Contains(rawLower, "colombia") { return "哥伦比亚/Colombia" }
	if strings.Contains(rawLower, "venezuela") { return "委内瑞拉/Venezuela" }
	if strings.Contains(rawLower, "peru") { return "秘鲁/Peru" }
	if strings.Contains(rawLower, "chile") { return "智利/Chile" }
	return raw
}

func geoLookupDetail(ip string) string {
	out := runCmd("curl", "-s", "--connect-timeout", "3", "http://ip-api.com/json/"+ip+"?lang=zh-CN")
	if out == "" {
		return ""
	}
	var result struct {
		Region string `json:"regionName"`
		City   string `json:"city"`
		ISP    string `json:"isp"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		return ""
	}
	if result.Status != "success" {
		return ""
	}

	// ISP 中文映射
	isp := result.ISP
	ispLower := strings.ToLower(isp)
	switch {
	case strings.Contains(ispLower, "chinanet"), strings.Contains(ispLower, "china telecom"):
		isp = "中国电信"
	case strings.Contains(ispLower, "china unicom"), strings.Contains(ispLower, "cnc group"):
		isp = "中国联通"
	case strings.Contains(ispLower, "china mobile"):
		isp = "中国移动"
	case strings.Contains(ispLower, "baidu"):
		isp = "百度"
	case strings.Contains(ispLower, "alibaba"), strings.Contains(ispLower, "taobao"), strings.Contains(ispLower, "hangzhou alibaba"):
		isp = "阿里云"
	case strings.Contains(ispLower, "tencent"), strings.Contains(ispLower, "weixin"):
		isp = "腾讯云"
	case strings.Contains(ispLower, "volcano engine"), strings.Contains(ispLower, "bytedance"), strings.Contains(ispLower, "volcano"):
		isp = "火山引擎"
	case strings.Contains(ispLower, "cloud computing"):
		isp = "腾讯云"
	case strings.Contains(ispLower, "bitnet"):
		isp = "中国广电"
	case strings.Contains(ispLower, "china internet network"):
		isp = "CNNIC"
	case strings.Contains(ispLower, "gds changan"):
		isp = "中国广电"
	case strings.Contains(ispLower, "cn2"):
		isp = "中国电信"
	case strings.Contains(ispLower, "qingdao"):
		isp = "中国联通"
	default:
		// 保留原始ISP缩短显示
		isp = strings.SplitN(isp, " ", 2)[0]
	}

	// 直辖市处理
	var detail string
	region := strings.TrimSuffix(strings.TrimSpace(result.Region), "市")
	region = strings.TrimSuffix(region, "省")
	city := strings.TrimSpace(result.City)
	city = strings.TrimSuffix(city, "市")
	regionIsDirectCity := strings.Contains(region, "北京") || strings.Contains(region, "上海") ||
		strings.Contains(region, "天津") || strings.Contains(region, "重庆")
	if regionIsDirectCity {
		if city == "" || city == region {
			detail = region + "市"
		} else {
			detail = region + "市" + city
		}
	} else {
		if city == "" || city == region {
			detail = region
		} else {
			detail = region + "省" + city + "市"
		}
	}

	if detail == "省市 " || detail == "市 " || detail == "" {
		return ""
	}
	return detail
}

