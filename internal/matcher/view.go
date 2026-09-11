package matcher

import (
	"regexp"
	"strings"
)

// View 设备视图标识，用于命令缓存等需要区分上下文的场景。
// 视图一律从设备**真实返回的提示符行**反解，不依赖命令跳转推断
// （避免因权限不足导致 system-view 失败、或 return 跳回多级造成状态崩塌）。
type View string

const (
	ViewUser      View = "user"      // 用户视图 (<host> / host>)
	ViewSystem    View = "system"    // 系统/特权视图 ([host] / host#)
	ViewInterface View = "interface" // 接口视图 ([host-GE0/0/1] / host(config-if)#)
	ViewRouting   View = "routing"   // 路由协议视图 ([host-ospf-1] / host(config-router)#)
	ViewConfig    View = "config"    // 全局配置视图 (host(config)#)
	ViewDiagnose  View = "diagnose"  // 诊断视图 ([host-diagnose])
	ViewSubview   View = "subview"   // 未知通用子视图 ([host-xxx])
	ViewUnknown   View = "unknown"   // 无法识别
)

// 接口与路由协议视图识别特征
var (
	reInterfaceToken = regexp.MustCompile(`(?i)(?:^|-)(?:ge|xge|10ge|25ge|40ge|100ge|eth|ethernet|gigabitethernet|eth-trunk|vlanif|meth|tunnel|loopback)\d`)
	reRoutingToken   = regexp.MustCompile(`(?i)(?:^|-)(?:ospf|ospfv3|bgp|isis|rip|ripng)\d*`)
	// 已知子视图关键字：命中说明确实处于某个子视图，
	// 未命中则视为"主机名本身含连字符"（如 [FW-1] / [Core-Switch-A]），归为系统视图。
	reSubviewKeyword = regexp.MustCompile(`(?i)(?:^|-)(?:aaa|acl|vlan|mpls|stp|mstp|dhcp|ntp|snmp|qos|lldp|lacp|vrrp|bfd|nat|ssh|telnet|user|security|nqa|twamp|stack|irf|css|cluster|smart-link|keychain|local-user)\d*`)
)

// ResolveView 从命中的提示符整行反解当前视图（纯函数、无副作用、可单测）。
// vendor 用于区分提示符风格（VRP 系 / Cisco 系）；未知厂商时按顺序尝试两种风格。
func ResolveView(vendor, promptLine string) View {
	line := strings.TrimSpace(promptLine)
	if line == "" {
		return ViewUnknown
	}

	switch strings.ToLower(strings.TrimSpace(vendor)) {
	case "huawei", "h3c", "hp", "comware", "vrp":
		return resolveVRPView(line)
	case "cisco", "ruijie", "ios":
		return resolveCiscoView(line)
	default:
		// 未知厂商：先按 VRP 括号风格判定，再回退 Cisco 风格
		if v := resolveVRPView(line); v != ViewUnknown {
			return v
		}
		return resolveCiscoView(line)
	}
}

// resolveVRPView 解析华为/H3C(Comware) 风格提示符：<host> / [host] / [host-xxx]
func resolveVRPView(line string) View {
	// 用户视图：<主机名>
	if strings.HasPrefix(line, "<") && strings.HasSuffix(line, ">") {
		inner := strings.TrimSuffix(strings.TrimPrefix(line, "<"), ">")
		if inner != "" && !strings.Contains(inner, " ") {
			return ViewUser
		}
	}

	// 方括号视图：[主机名] / [主机名-子视图]
	if idx := strings.Index(line, "["); idx >= 0 && strings.HasSuffix(line, "]") {
		inner := strings.TrimSuffix(line[idx+1:], "]")
		if inner == "" {
			return ViewSystem
		}
		// 兼容子视图中的主机名部分：[SW1-ospf-1] -> 去掉首段主机名后判定
		lower := strings.ToLower(inner)
		if strings.HasSuffix(lower, "-diagnose") {
			return ViewDiagnose
		}
		if reRoutingToken.MatchString(lower) {
			return ViewRouting
		}
		if reInterfaceToken.MatchString(lower) {
			return ViewInterface
		}
		if strings.Contains(lower, "diagnose") {
			return ViewDiagnose
		}
		// 命中已知子视图关键字才判为 subview；
		// 否则视为主机名自身含连字符（如 [FW-1]），归为系统视图，避免误判。
		if reSubviewKeyword.MatchString(lower) {
			return ViewSubview
		}
		return ViewSystem
	}

	// 无括号的裸提示符兜底
	switch {
	case strings.HasSuffix(line, ">"):
		return ViewUser
	case strings.HasSuffix(line, "#"):
		return ViewSystem
	}
	return ViewUnknown
}

// resolveCiscoView 解析 Cisco/Ruijie 风格提示符：host> / host# / host(config)# ...
func resolveCiscoView(line string) View {
	lower := strings.ToLower(line)

	// 配置子模式优先判定
	switch {
	case strings.HasSuffix(lower, "(config-router)#"):
		return ViewRouting
	case strings.HasSuffix(lower, "(config-if)#"), strings.HasSuffix(lower, "(config-subif)#"):
		return ViewInterface
	case strings.HasSuffix(lower, "(config)#"), strings.HasSuffix(lower, "(config)"):
		return ViewConfig
	case strings.HasSuffix(lower, "#"):
		return ViewSystem
	case strings.HasSuffix(lower, ">"):
		return ViewUser
	}
	return ViewUnknown
}
