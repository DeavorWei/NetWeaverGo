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
	Tags     []string `json:"tags" gorm:"serializer:json"`    // 设备标签列表
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
	defaultInventoryFile = "inventory.csv"
	defaultConfigFile    = "config.txt"
)

// LoadExecutionResources 获取执行所需的设备资产和默认命令组
func LoadExecutionResources() ([]DeviceAsset, []string, error) {
	devices, err := LoadDeviceAssets()
	if err != nil {
		return nil, nil, err
	}

	commands, err := LoadDefaultCommands()
	if err != nil {
		return nil, nil, err
	}

	return devices, commands, nil
}

// LoadDeviceAssets 从数据库加载全部设备资产
func LoadDeviceAssets() ([]DeviceAsset, error) {
	logger.Debug("Config", "-", "开始从数据库加载设备资产")
	if DB == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}

	var devices []DeviceAsset
	if err := DB.Order("ip ASC").Find(&devices).Error; err != nil {
		return nil, err
	}
	return devices, nil
}

// LoadDefaultCommands 从默认命令组加载命令列表
func LoadDefaultCommands() ([]string, error) {
	logger.Debug("Config", "-", "开始从数据库加载默认命令组")
	if DB == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}

	var defaultGroup CommandGroup
	if err := DB.Where("name = ?", "默认命令组").First(&defaultGroup).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return []string{}, nil
		}
		return nil, err
	}

	return append([]string(nil), defaultGroup.Commands...), nil
}

// readInventoryLegacy 读取并解析旧版资产清单文件
func readInventoryLegacy(filePath string) ([]DeviceAsset, error) {
	if strings.TrimSpace(filePath) == "" {
		filePath = filepath.Join(GetPathManager().WorkDir, defaultInventoryFile)
	}
	logger.DebugAll("Config", "-", "尝试打开文件: %s", filePath)
	file, err := os.Open(filePath)
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
	configPath := GetPathManager().GetLegacyConfigFile()
	if strings.TrimSpace(configPath) == "" {
		configPath = filepath.Join(GetPathManager().WorkDir, defaultConfigFile)
	}
	logger.DebugAll("Config", "-", "尝试打开文件: %s", configPath)
	content, err := os.ReadFile(configPath)
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

func normalizeDevice(device *DeviceAsset) {
	device.IP = strings.TrimSpace(device.IP)
	device.Protocol = strings.ToUpper(strings.TrimSpace(device.Protocol))
	device.Username = strings.TrimSpace(device.Username)
	device.Password = strings.TrimSpace(device.Password)
	device.Group = strings.TrimSpace(device.Group)

	if len(device.Tags) == 0 {
		device.Tags = []string{}
		return
	}

	tags := make([]string, 0, len(device.Tags))
	seen := make(map[string]struct{}, len(device.Tags))
	for _, tag := range device.Tags {
		trimmed := strings.TrimSpace(tag)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		tags = append(tags, trimmed)
	}
	device.Tags = tags
}

func validateDevicesForWrite(devices []DeviceAsset) error {
	ipSet := make(map[string]struct{}, len(devices))
	idSet := make(map[uint]struct{}, len(devices))

	for i := range devices {
		normalizeDevice(&devices[i])
		if err := ValidateDevice(devices[i]); err != nil {
			return fmt.Errorf("第 %d 台设备: %v", i+1, err)
		}

		if _, exists := ipSet[devices[i].IP]; exists {
			return fmt.Errorf("存在重复的 IP 地址: %s", devices[i].IP)
		}
		ipSet[devices[i].IP] = struct{}{}

		if devices[i].ID != 0 {
			if _, exists := idSet[devices[i].ID]; exists {
				return fmt.Errorf("存在重复的设备 ID: %d", devices[i].ID)
			}
			idSet[devices[i].ID] = struct{}{}
		}
	}

	return nil
}

// CreateDevice 创建单台设备
func CreateDevice(device DeviceAsset) error {
	if DB == nil {
		return fmt.Errorf("数据库未初始化")
	}

	normalizeDevice(&device)
	if err := ValidateDevice(device); err != nil {
		return err
	}

	var count int64
	if err := DB.Model(&DeviceAsset{}).Where("ip = ?", device.IP).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("IP 地址 %s 已存在", device.IP)
	}

	device.ID = 0
	return DB.Create(&device).Error
}

// CreateDevices 批量创建设备
func CreateDevices(devices []DeviceAsset) error {
	if DB == nil {
		return fmt.Errorf("数据库未初始化")
	}
	if len(devices) == 0 {
		return nil
	}

	for i := range devices {
		devices[i].ID = 0
	}
	if err := validateDevicesForWrite(devices); err != nil {
		return err
	}

	ips := make([]string, 0, len(devices))
	for _, device := range devices {
		ips = append(ips, device.IP)
	}

	var existing []DeviceAsset
	if err := DB.Where("ip IN ?", ips).Find(&existing).Error; err != nil {
		return err
	}
	if len(existing) > 0 {
		return fmt.Errorf("IP 地址 %s 已存在", existing[0].IP)
	}

	return DB.Create(&devices).Error
}

// UpdateDevice 更新单台设备
func UpdateDevice(id uint, device DeviceAsset) error {
	if DB == nil {
		return fmt.Errorf("数据库未初始化")
	}
	if id == 0 {
		return fmt.Errorf("无效的设备 ID")
	}

	device.ID = id
	if err := validateDevicesForWrite([]DeviceAsset{device}); err != nil {
		return err
	}

	return DB.Transaction(func(tx *gorm.DB) error {
		var existing DeviceAsset
		if err := tx.First(&existing, id).Error; err != nil {
			return fmt.Errorf("未找到设备: %d", id)
		}
		logger.DebugAll(
			"Config",
			existing.IP,
			"收到单设备更新请求: id=%d, protocol=%s->%s, port=%d->%d, group=%q->%q, username=%q->%q, tags=%q->%q",
			id,
			existing.Protocol, device.Protocol,
			existing.Port, device.Port,
			existing.Group, device.Group,
			existing.Username, device.Username,
			strings.Join(existing.Tags, ","), strings.Join(device.Tags, ","),
		)

		var conflict DeviceAsset
		err := tx.Where("ip = ? AND id <> ?", device.IP, id).First(&conflict).Error
		if err == nil {
			return fmt.Errorf("IP 地址 %s 已被其他设备使用", device.IP)
		}
		if err != nil && err != gorm.ErrRecordNotFound {
			return err
		}

		if err := tx.Save(&device).Error; err != nil {
			return err
		}
		logger.DebugAll("Config", device.IP, "单设备更新完成: id=%d", id)
		return nil
	})
}

// UpdateDevices 批量更新设备
func UpdateDevices(devices []DeviceAsset) error {
	if DB == nil {
		return fmt.Errorf("数据库未初始化")
	}
	if len(devices) == 0 {
		return nil
	}

	if err := validateDevicesForWrite(devices); err != nil {
		return err
	}

	ids := make([]uint, 0, len(devices))
	ips := make([]string, 0, len(devices))
	idSet := make(map[uint]struct{}, len(devices))
	for _, device := range devices {
		if device.ID == 0 {
			return fmt.Errorf("批量更新时存在无效设备 ID")
		}
		ids = append(ids, device.ID)
		ips = append(ips, device.IP)
		idSet[device.ID] = struct{}{}
	}

	logger.DebugAll("Config", "-", "收到批量设备更新请求: count=%d", len(devices))
	return DB.Transaction(func(tx *gorm.DB) error {
		var existing []DeviceAsset
		if err := tx.Where("id IN ?", ids).Find(&existing).Error; err != nil {
			return err
		}
		if len(existing) != len(ids) {
			return fmt.Errorf("部分设备不存在，无法完成批量更新")
		}
		existingByID := make(map[uint]DeviceAsset, len(existing))
		for _, item := range existing {
			existingByID[item.ID] = item
		}

		var conflicts []DeviceAsset
		if err := tx.Where("ip IN ?", ips).Find(&conflicts).Error; err != nil {
			return err
		}
		for _, conflict := range conflicts {
			if _, ok := idSet[conflict.ID]; !ok {
				return fmt.Errorf("IP 地址 %s 已被其他设备使用", conflict.IP)
			}
		}

		for _, device := range devices {
			old, ok := existingByID[device.ID]
			if ok {
				logger.DebugAll(
					"Config",
					old.IP,
					"批量更新设备: id=%d, protocol=%s->%s, port=%d->%d, group=%q->%q, username=%q->%q, tags=%q->%q",
					device.ID,
					old.Protocol, device.Protocol,
					old.Port, device.Port,
					old.Group, device.Group,
					old.Username, device.Username,
					strings.Join(old.Tags, ","), strings.Join(device.Tags, ","),
				)
			}
			if err := tx.Save(&device).Error; err != nil {
				return err
			}
		}
		logger.DebugAll("Config", "-", "批量设备更新完成: count=%d", len(devices))
		return nil
	})
}

// DeleteDevice 删除单台设备
func DeleteDevice(id uint) error {
	if DB == nil {
		return fmt.Errorf("数据库未初始化")
	}
	if id == 0 {
		return fmt.Errorf("无效的设备 ID")
	}

	result := DB.Delete(&DeviceAsset{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("未找到设备: %d", id)
	}
	return nil
}

// DeleteDevices 批量删除设备
func DeleteDevices(ids []uint) error {
	if DB == nil {
		return fmt.Errorf("数据库未初始化")
	}
	if len(ids) == 0 {
		return nil
	}

	result := DB.Where("id IN ?", ids).Delete(&DeviceAsset{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("未找到可删除的设备")
	}
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
