package security

import (
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
)

var (
	// commandSafeRegex 校验命令是否含有非法 shell 注入字符（例如管道重定向、后台执行等）
	// CLI 采集场景下通常只允许标准命令行字符
	commandSafeRegex = regexp.MustCompile(`^[a-zA-Z0-9_\-\.\s/:@\?\[\]\(\)\{\}\*=,]+$`)

	// interfaceNameRegex 标准网络接口名称模式
	interfaceNameRegex = regexp.MustCompile(`^(?i)[a-z0-9/_\-\.:]+$`)
)

// ValidateIPv4 校验是否合法的 IPv4 地址
func ValidateIPv4(ip string) bool {
	parsed := net.ParseIP(strings.TrimSpace(ip))
	return parsed != nil && parsed.To4() != nil
}

// ValidateIPOrCIDR 校验是否合法的 IP 地址或 CIDR 子网
func ValidateIPOrCIDR(cidr string) bool {
	cidr = strings.TrimSpace(cidr)
	if ValidateIPv4(cidr) {
		return true
	}
	_, _, err := net.ParseCIDR(cidr)
	return err == nil
}

// ValidatePort 校验是否合法的 TCP/UDP 端口 (1 ~ 65535)
func ValidatePort(port int) bool {
	return port >= 1 && port <= 65535
}

// ValidateVlanID 校验是否合法的 VLAN ID (1 ~ 4094)
func ValidateVlanID(vlan int) bool {
	return vlan >= 1 && vlan <= 4094
}

// ValidateInterfaceName 校验接口名称格式是否安全合规
func ValidateInterfaceName(name string) bool {
	trimmed := strings.TrimSpace(name)
	if len(trimmed) == 0 || len(trimmed) > 64 {
		return false
	}
	return interfaceNameRegex.MatchString(trimmed)
}

// ValidateSafeCommand 校验命令字符串是否包含潜在代码注入或非法控制字符
func ValidateSafeCommand(cmd string) error {
	trimmed := strings.TrimSpace(cmd)
	if trimmed == "" {
		return fmt.Errorf("命令不能为空")
	}
	if len(trimmed) > 1024 {
		return fmt.Errorf("命令长度超限 (最大 1024 字节)")
	}
	// 拦截包含控制字符、换行或非法 Shell 管道符号
	if strings.ContainsAny(trimmed, "\r\n;&|`$><") {
		return fmt.Errorf("命令包含非法或危险字符: %q", trimmed)
	}
	if !commandSafeRegex.MatchString(trimmed) {
		return fmt.Errorf("命令格式不合规")
	}
	return nil
}

// SanitizeFilePath 防御路径穿越漏洞 (Directory Traversal)
func SanitizeFilePath(path string) bool {
	if strings.Contains(path, "..") || strings.Contains(path, "\\..") || strings.Contains(path, "/..") {
		return false
	}
	return true
}

// ValidateSlotID 校验槽位/板卡号格式（如 "0", "1/0", "0/1/2"）
func ValidateSlotID(slot string) bool {
	slot = strings.TrimSpace(slot)
	if slot == "" {
		return false
	}
	parts := strings.Split(slot, "/")
	for _, p := range parts {
		val, err := strconv.Atoi(p)
		if err != nil || val < 0 || val > 128 {
			return false
		}
	}
	return true
}
