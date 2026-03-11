package config

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/NetWeaverGo/core/internal/logger"
	"gorm.io/gorm"
)

// DeviceAsset 表示单台交换机的连接凭证信息
type DeviceAsset struct {
	ID       uint     `json:"id" gorm:"primaryKey"`
	IP       string   `json:"ip" gorm:"uniqueIndex;not null"`
	Port     int      `json:"port"`
	Protocol string   `json:"protocol"` // 连接协议：SSH/SNMP/TELNET
	Username string   `json:"username"`
	Password string   `json:"password"`
	Group    string   `json:"group" gorm:"column:group_name"` // 设备分组
	Tags     []string `json:"tags" gorm:"serializer:json"`  // 设备标签列表
}

// 协议默认端口映射
var ProtocolDefaultPorts = map[string]int{
	"SSH":    22,
	"SNMP":   161,
	"TELNET": 23,
}

// ValidProtocols 有效协议列表
var ValidProtocols = []string{"SSH", "SNMP", "TELNET"}

const (
	inventoryFile = "inventory.csv"
	configFile    = "config.txt"
)

// ParseOrGenerate 尝试获取数据库内的设备和默认命令
func ParseOrGenerate(isBackup bool) ([]DeviceAsset, []string, []string, []string, error) {
	logger.Debug("Config", "-", "开始从数据库获取设备和默认命令")
	var devices []DeviceAsset
	if DB != nil {
		DB.Find(&devices)
	}

	var commands []string
	if DB != nil {
		var defaultGroup CommandGroup
		if err := DB.Where("name = ?", "默认命令组").First(&defaultGroup).Error; err == nil {
			commands = defaultGroup.Commands
		}
	}

	return devices, commands, nil, nil, nil
}

// readInventoryLegacy 读取并解析旧版资产清单文件
func readInventoryLegacy() ([]DeviceAsset, error) {
	logger.DebugAll("Config", "-", "尝试打开文件: %s", inventoryFile)
	file, err := os.Open(inventoryFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	if len(records) <= 1 {
		return nil, fmt.Errorf("资产文件内容为空或只有表头")
	}

	var devices []DeviceAsset

	for i, row := range records[1:] {
		// 兼容新旧格式：
		// 新格式7列(IP,Port,Protocol,Username,Password,Group,Tag)
		// 旧格式5列(IP,Port,Protocol,Username,Password)
		// 最旧格式4列(IP,Port,Username,Password)
		if len(row) < 4 {
			return nil, fmt.Errorf("资产文件第 %d 行格式不匹配", i+2)
		}

		ip := strings.TrimSpace(row[0])
		portStr := strings.TrimSpace(row[1])
		var protocol, username, password, group, tag string

		// 判断是哪种格式
		if len(row) >= 7 {
			// 最新格式：IP,Port,Protocol,Username,Password,Group,Tag
			protocol = strings.ToUpper(strings.TrimSpace(row[2]))
			username = strings.TrimSpace(row[3])
			password = strings.TrimSpace(row[4])
			group = strings.TrimSpace(row[5])
			tag = strings.TrimSpace(row[6])
		} else if len(row) >= 5 {
			// 旧格式：IP,Port,Protocol,Username,Password
			protocol = strings.ToUpper(strings.TrimSpace(row[2]))
			username = strings.TrimSpace(row[3])
			password = strings.TrimSpace(row[4])
			group = ""
			tag = ""
		} else {
			// 最旧格式：IP,Port,Username,Password (兼容)
			protocol = "SSH"
			username = strings.TrimSpace(row[2])
			password = strings.TrimSpace(row[3])
			group = ""
			tag = ""
		}

		if ip == "" {
			continue
		}

		// 校验协议有效性
		if !isValidProtocol(protocol) {
			protocol = "SSH"
		}

		port, err := strconv.Atoi(portStr)
		if err != nil || port <= 0 {
			port = GetDefaultPort(protocol)
		}

		devices = append(devices, DeviceAsset{
			IP:       ip,
			Port:     port,
			Protocol: protocol,
			Username: username,
			Password: password,
			Group:    group,
			Tags:     parseTags(tag),
		})
	}
	return devices, nil
}

// readCommandsLegacy 读取并解析旧版命令列表文件
func readCommandsLegacy() ([]string, error) {
	logger.DebugAll("Config", "-", "尝试打开文件: %s", configFile)
	content, err := os.ReadFile(configFile)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(content), "\n")
	var commands []string
	for _, line := range lines {
		cmd := strings.TrimRight(line, "\r\n")
		// 忽略空行和注释
		if strings.TrimSpace(cmd) == "" || strings.HasPrefix(strings.TrimSpace(cmd), "#") {
			continue
		}
		commands = append(commands, cmd)
	}

	if len(commands) == 0 {
		return nil, fmt.Errorf("命令文件为空")
	}
	return commands, nil
}

// generateInventoryTemplate 生成默认的资产清单模板
func generateInventoryTemplate() {
	cwd, _ := os.Getwd()
	path := filepath.Join(cwd, inventoryFile)
	file, err := os.Create(path)
	if err != nil {
		fmt.Printf("无法创建资产模板文件: %v\n", err)
		return
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	writer.Write([]string{"IP", "Port", "Protocol", "Username", "Password", "Group", "Tag"})
	writer.Write([]string{"192.168.1.10", "22", "SSH", "admin", "Admin@123", "核心交换机", "生产环境"})
	writer.Write([]string{"192.168.1.11", "161", "SNMP", "public", "public", "接入交换机", "测试环境"})
}

// generateConfigTemplate 生成默认的命令列表模板
func generateConfigTemplate() {
	cwd, _ := os.Getwd()
	path := filepath.Join(cwd, configFile)
	content := []byte(`# 在此输入需要批量下发的交换机命令，每行一条
# 空行和以 # 开头的行将被忽略

system-view
display interface brief
quit
`)
	err := os.WriteFile(path, content, 0666)
	if err != nil {
		fmt.Printf("无法创建命令模板文件: %v\n", err)
	}
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

// parseTags 从字符串解析标签数组（支持逗号或分号分隔）
func parseTags(tagStr string) []string {
	if tagStr == "" {
		return []string{}
	}
	// 支持逗号或分号分隔
	tags := strings.Split(strings.ReplaceAll(tagStr, ";", ","), ",")
	result := make([]string, 0, len(tags))
	for _, tag := range tags {
		if t := strings.TrimSpace(tag); t != "" {
			result = append(result, t)
		}
	}
	return result
}

// joinTags 将标签数组合并为逗号分隔的字符串
func joinTags(tags []string) string {
	return strings.Join(tags, ",")
}

// GetDefaultPort 根据协议获取默认端口
func GetDefaultPort(protocol string) int {
	if port, ok := ProtocolDefaultPorts[strings.ToUpper(protocol)]; ok {
		return port
	}
	return 22 // 默认 SSH 端口
}

// ValidateDevice 校验设备信息合法性
func ValidateDevice(device DeviceAsset) error {
	if device.IP == "" {
		return fmt.Errorf("IP 地址不能为空")
	}

	// 简单的 IP 格式校验
	parts := strings.Split(device.IP, ".")
	if len(parts) != 4 {
		return fmt.Errorf("IP 地址格式不正确")
	}
	for _, part := range parts {
		num, err := strconv.Atoi(part)
		if err != nil || num < 0 || num > 255 {
			return fmt.Errorf("IP 地址格式不正确")
		}
	}

	if device.Port <= 0 || device.Port > 65535 {
		return fmt.Errorf("端口号必须在 1-65535 之间")
	}

	if !isValidProtocol(device.Protocol) {
		return fmt.Errorf("无效的协议类型: %s", device.Protocol)
	}

	// 用户名和密码可选，不再强制要求

	return nil
}

// SaveCommandsLegacy 保存命令列表到文件 (已遗弃，仅为兼容保留)
func SaveCommandsLegacy(commands []string) error {
	cwd, _ := os.Getwd()
	path := filepath.Join(cwd, configFile)

	var content strings.Builder
	for _, cmd := range commands {
		content.WriteString(cmd + "\n")
	}

	err := os.WriteFile(path, []byte(content.String()), 0666)
	if err != nil {
		return fmt.Errorf("无法保存命令文件: %v", err)
	}

	logger.Info("Config", "-", "成功保存 %d 条命令到 %s", len(commands), configFile)
	return nil
}

// SaveCommands 保存命令列表到默认命令组
func SaveCommands(commands []string) error {
	if DB == nil {
		return fmt.Errorf("数据库未初始化")
	}

	var group CommandGroup
	err := DB.Where("name = ?", "默认命令组").First(&group).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			group = CommandGroup{
				ID:          generateID(),
				Name:        "默认命令组",
				Description: "自动生成的默认命令组",
				Commands:    commands,
				CreatedAt:   nowFormatted(),
				UpdatedAt:   nowFormatted(),
				Tags:        []string{"系统默认"},
			}
			return DB.Create(&group).Error
		}
		return err
	}

	group.Commands = commands
	group.UpdatedAt = nowFormatted()
	return DB.Save(&group).Error
}

// SaveInventory 保存设备列表到数据库（全量覆盖）
func SaveInventory(devices []DeviceAsset) error {
	if DB == nil {
		return fmt.Errorf("数据库未初始化")
	}

	return DB.Transaction(func(tx *gorm.DB) error {
		tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&DeviceAsset{})
		if len(devices) > 0 {
			for i := range devices {
				devices[i].ID = 0 // 重置ID以重新排列
			}
			return tx.Create(&devices).Error
		}
		return nil
	})
}

