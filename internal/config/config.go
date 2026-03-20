package config

import (
	"fmt"
	"net"
	"strings"

	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
	"gorm.io/gorm"
)

// 协议默认端口映射
var ProtocolDefaultPorts = map[string]int{
	"SSH":    22,
	"SNMP":   161,
	"TELNET": 23,
}

// ValidProtocols 有效协议列表
var ValidProtocols = []string{"SSH", "SNMP", "TELNET"}

const (
	defaultInventoryFile = "inventory.csv"
	defaultConfigFile    = "config.txt"
)

// PasswordMergeResult 密码合并结果
// 用于统一单设备更新和批量更新的密码处理逻辑
type PasswordMergeResult struct {
	Password    string // 合并后的密码
	Changed     bool   // 密码是否发生变化
	OldPassword string // 原密码（用于审计日志脱敏）
}

// MergePassword 统一密码合并规则
// 语义：空值不修改，非空值更新
// 参数：
//   - oldPassword: 原密码
//   - newPassword: 新密码
//
// 返回：
//   - PasswordMergeResult: 包含合并后的密码、是否变化、原密码
func MergePassword(oldPassword, newPassword string) PasswordMergeResult {
	// 空值不修改
	if newPassword == "" {
		return PasswordMergeResult{
			Password:    oldPassword,
			Changed:     false,
			OldPassword: oldPassword,
		}
	}

	// 非空值更新
	changed := newPassword != oldPassword
	return PasswordMergeResult{
		Password:    newPassword,
		Changed:     changed,
		OldPassword: oldPassword,
	}
}

// LoadDefaultCommands 从默认命令组加载命令列表
func LoadDefaultCommands() ([]string, error) {
	logger.Debug("Config", "-", "开始从数据库加载默认命令组")
	if DB == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}

	var defaultGroup models.CommandGroup
	if err := DB.Where("name = ?", "默认命令组").First(&defaultGroup).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return []string{}, nil
		}
		return nil, err
	}

	return defaultGroup.Commands, nil
}

// isValidProtocol 检查协议是否有效
func isValidProtocol(protocol string) bool {
	for _, p := range ValidProtocols {
		if p == protocol {
			return true
		}
	}
	return false
}

// GetDefaultPort 根据协议获取默认端口
func GetDefaultPort(protocol string) int {
	if port, ok := ProtocolDefaultPorts[strings.ToUpper(protocol)]; ok {
		return port
	}
	return 22 // 默认 SSH 端口
}

// ValidateDevice 校验设备信息合法性
func ValidateDevice(device *models.DeviceAsset) error {
	if device.IP == "" {
		return fmt.Errorf("IP 地址不能为空")
	}

	// 使用 net.ParseIP 支持 IPv4 和 IPv6
	ip := net.ParseIP(device.IP)
	if ip == nil {
		return fmt.Errorf("IP 地址格式不正确: %s", device.IP)
	}

	if device.Port <= 0 || device.Port > 65535 {
		return fmt.Errorf("端口号必须在 1-65535 之间")
	}

	if !isValidProtocol(device.Protocol) {
		return fmt.Errorf("无效的协议类型: %s", device.Protocol)
	}

	return nil
}

// NormalizeDevice 标准化设备信息
// 导出供 Service 层使用
func NormalizeDevice(device *models.DeviceAsset) {
	device.IP = strings.TrimSpace(device.IP)
	device.Protocol = strings.ToUpper(strings.TrimSpace(device.Protocol))
	device.Username = strings.TrimSpace(device.Username)
	device.Password = strings.TrimSpace(device.Password)
	device.Group = strings.TrimSpace(device.Group)
}

// SaveCommands 保存命令列表到默认命令组
func SaveCommands(commands []string) error {
	if DB == nil {
		return fmt.Errorf("数据库未初始化")
	}

	var group models.CommandGroup
	err := DB.Where("name = ?", "默认命令组").First(&group).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			group = models.CommandGroup{
				Name:        "默认命令组",
				Description: "自动生成的默认命令组",
				Commands:    commands,
			}
			return DB.Create(&group).Error
		}
		return err
	}

	group.Commands = commands
	return DB.Save(&group).Error
}
