package normalize

import (
	"regexp"
	"strings"
)

// 接口名缩写映射
var interfaceAbbreviations = map[string]string{
	"GigabitEthernet":     "GE",
	"XGigabitEthernet":    "XGE",
	"Ten-GigabitEthernet": "XGE",
	"FortyGigE":           "FGE",
	"HundredGigE":         "HGE",
	"Ethernet":            "Eth",
	"FastEthernet":        "FE",
	"Eth-Trunk":           "Trunk",
	"Bridge-Aggregation":  "BAgg",
	"Route-Aggregation":   "RAGG",
	"Port-channel":        "Po",
	"Vlan-interface":      "Vlanif",
	"Vlanif":              "Vlanif",
	"LoopBack":            "Loop",
	"NULL":                "NULL",
	"GigabitEthernet0/":   "GE0/",
	"GigabitEthernet1/":   "GE1/",
	"XGigabitEthernet0/":  "XGE0/",
	"XGigabitEthernet1/":  "XGE1/",
}

// 接口名正则表达式
var (
	// 匹配接口名格式：类型+槽位/子槽位/端口
	interfacePattern = regexp.MustCompile(`^([A-Za-z\-]+)(\d+/\d+/\d+)$`)
	// 匹配简化接口名格式：类型+端口号
	simpleInterfacePattern = regexp.MustCompile(`^([A-Za-z\-]+)(\d+)$`)
)

// NormalizeInterfaceName 归一化接口名
// 示例：GigabitEthernet1/0/1 → GE1/0/1
//
//	XGigabitEthernet1/0/1 → XGE1/0/1
//	Eth-Trunk10 → Trunk10
func NormalizeInterfaceName(name string) string {
	if name == "" {
		return ""
	}

	// 去除空格
	name = strings.TrimSpace(name)

	// 尝试匹配完整接口名模式
	for full, abbrev := range interfaceAbbreviations {
		if strings.HasPrefix(name, full) {
			return abbrev + strings.TrimPrefix(name, full)
		}
	}

	// 如果没有匹配到，返回原始名称（统一小写）
	return name
}

// NormalizeLLDPRemotePort 归一化 LLDP 远端端口字段。
func NormalizeLLDPRemotePort(port string) string {
	port = strings.TrimSpace(port)
	if port == "" {
		return ""
	}
	switch strings.ToLower(port) {
	case "-", "n/a", "unknown":
		return ""
	default:
		return NormalizeInterfaceName(port)
	}
}

// NormalizeAggregateName 归一化聚合口名称。
func NormalizeAggregateName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}

	lower := strings.ToLower(name)
	digits := trailingDigits(name)
	switch {
	case strings.HasPrefix(lower, "eth-trunk"), strings.HasPrefix(lower, "trunk"),
		strings.HasPrefix(lower, "bridge-aggregation"), strings.HasPrefix(lower, "route-aggregation"):
		if digits != "" {
			return "Trunk" + digits
		}
	case strings.HasPrefix(lower, "port-channel"), strings.HasPrefix(lower, "po"):
		if digits != "" {
			return "Po" + digits
		}
	}

	return NormalizeInterfaceName(name)
}

func trailingDigits(s string) string {
	end := len(s)
	start := end
	for start > 0 {
		ch := s[start-1]
		if ch < '0' || ch > '9' {
			break
		}
		start--
	}
	if start == end {
		return ""
	}
	return s[start:end]
}

// NormalizeDeviceName 归一化设备名
// 统一转为大写，去除特殊字符
func NormalizeDeviceName(name string) string {
	if name == "" {
		return ""
	}

	// 去除空格并转大写
	name = strings.TrimSpace(name)
	name = strings.ToUpper(name)

	// 去除常见前缀
	prefixes := []string{"HW-", "H3C-", "CISCO-"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(name, prefix) {
			name = strings.TrimPrefix(name, prefix)
			break
		}
	}

	return name
}

// MAC地址正则表达式
var macPatterns = []*regexp.Regexp{
	// 格式：xx:xx:xx:xx:xx:xx
	regexp.MustCompile(`^([0-9a-fA-F]{2}):([0-9a-fA-F]{2}):([0-9a-fA-F]{2}):([0-9a-fA-F]{2}):([0-9a-fA-F]{2}):([0-9a-fA-F]{2})$`),
	// 格式：xx-xx-xx-xx-xx-xx
	regexp.MustCompile(`^([0-9a-fA-F]{2})-([0-9a-fA-F]{2})-([0-9a-fA-F]{2})-([0-9a-fA-F]{2})-([0-9a-fA-F]{2})-([0-9a-fA-F]{2})$`),
	// 格式：xxxx.xxxx.xxxx
	regexp.MustCompile(`^([0-9a-fA-F]{4})\.([0-9a-fA-F]{4})\.([0-9a-fA-F]{4})$`),
}

// NormalizeMAC 归一化MAC地址
// 统一转为小写冒号分隔格式：xx:xx:xx:xx:xx:xx
func NormalizeMAC(mac string) string {
	if mac == "" {
		return ""
	}

	mac = strings.TrimSpace(mac)
	mac = strings.ToLower(mac)

	// 尝试匹配不同格式
	for _, pattern := range macPatterns {
		if matches := pattern.FindStringSubmatch(mac); matches != nil {
			// 根据匹配结果构建统一格式
			if len(matches) == 7 {
				// xx:xx:xx:xx:xx:xx 或 xx-xx-xx-xx-xx-xx 格式
				return matches[1] + ":" + matches[2] + ":" + matches[3] + ":" + matches[4] + ":" + matches[5] + ":" + matches[6]
			} else if len(matches) == 4 {
				// xxxx.xxxx.xxxx 格式
				// 将每4位拆分为2个2位
				parts := matches[1] + matches[2] + matches[3]
				return parts[0:2] + ":" + parts[2:4] + ":" + parts[4:6] + ":" + parts[6:8] + ":" + parts[8:10] + ":" + parts[10:12]
			}
		}
	}

	// 如果没有匹配到，返回原始值
	return mac
}

// NormalizeVendor 归一化厂商名称
func NormalizeVendor(vendor string) string {
	if vendor == "" {
		return "unknown"
	}

	vendor = strings.TrimSpace(vendor)
	vendor = strings.ToLower(vendor)

	// 厂商名称映射
	vendorMap := map[string]string{
		"huawei":  "huawei",
		"h3c":     "h3c",
		"hp":      "h3c",
		"cisco":   "cisco",
		"arista":  "arista",
		"juniper": "juniper",
		"server":  "server",
		"unknown": "unknown",
	}

	if normalized, ok := vendorMap[vendor]; ok {
		return normalized
	}

	// 尝试模糊匹配
	if strings.Contains(vendor, "huawei") {
		return "huawei"
	}
	if strings.Contains(vendor, "h3c") || strings.Contains(vendor, "hp ") {
		return "h3c"
	}
	if strings.Contains(vendor, "cisco") {
		return "cisco"
	}

	return "unknown"
}

// NormalizeRole 归一化设备角色
func NormalizeRole(role string) string {
	if role == "" {
		return "unknown"
	}

	role = strings.TrimSpace(role)
	role = strings.ToLower(role)

	// 角色映射
	roleMap := map[string]string{
		"core":        "core",
		"aggregation": "aggregation",
		"aggregate":   "aggregation",
		"access":      "access",
		"firewall":    "firewall",
		"fw":          "firewall",
		"server":      "server",
		"router":      "router",
		"switch":      "switch",
		"unknown":     "unknown",
	}

	if normalized, ok := roleMap[role]; ok {
		return normalized
	}

	return "unknown"
}

// IsAggregateInterface 判断是否为聚合接口
func IsAggregateInterface(name string) bool {
	name = strings.ToLower(name)
	return strings.Contains(name, "trunk") ||
		strings.Contains(name, "port-channel") ||
		strings.Contains(name, "aggregation") ||
		strings.Contains(name, "bundle")
}

// ParseInterfaceSpeed 解析接口速率
func ParseInterfaceSpeed(speed string) int {
	speed = strings.ToLower(speed)

	// 速率映射（单位：Mbps）
	speedMap := map[string]int{
		"10m":   10,
		"100m":  100,
		"1g":    1000,
		"1000m": 1000,
		"10g":   10000,
		"25g":   25000,
		"40g":   40000,
		"100g":  100000,
		"400g":  400000,
	}

	// 直接匹配
	if s, ok := speedMap[speed]; ok {
		return s
	}

	// 尝试解析数字
	for k, v := range speedMap {
		if strings.Contains(speed, k) {
			return v
		}
	}

	return 0
}
