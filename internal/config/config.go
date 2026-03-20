package config

import (
	"fmt"
	"net"
	"strings"

	"github.com/NetWeaverGo/core/internal/forge"
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

// LoadExecutionResources 获取执行所需的设备资产和默认命令组
func LoadExecutionResources() ([]models.DeviceAsset, []string, error) {
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
func LoadDeviceAssets() ([]models.DeviceAsset, error) {
	logger.Debug("Config", "-", "开始从数据库加载设备资产")
	if DB == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}

	var devices []models.DeviceAsset
	if err := DB.Order("ip ASC").Find(&devices).Error; err != nil {
		return nil, err
	}
	return devices, nil
}

// LoadDeviceByID 根据 ID 加载单个设备资产（包含解密后的密码）
func LoadDeviceByID(id uint) (*models.DeviceAsset, error) {
	logger.Debug("Config", "-", "开始从数据库加载设备详情, ID: %d", id)
	if DB == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}

	var device models.DeviceAsset
	if err := DB.First(&device, id).Error; err != nil {
		return nil, err
	}
	return &device, nil
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

func normalizeDevice(device *models.DeviceAsset) {
	device.IP = strings.TrimSpace(device.IP)
	device.Protocol = strings.ToUpper(strings.TrimSpace(device.Protocol))
	device.Username = strings.TrimSpace(device.Username)
	device.Password = strings.TrimSpace(device.Password)
	device.Group = strings.TrimSpace(device.Group)
}

func validateDevicesForWrite(devices []models.DeviceAsset) error {
	ipSet := make(map[string]struct{}, len(devices))
	idSet := make(map[uint]struct{}, len(devices))

	for i := range devices {
		normalizeDevice(&devices[i])
		if err := ValidateDevice(&devices[i]); err != nil {
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
func CreateDevice(device models.DeviceAsset) error {
	if DB == nil {
		return fmt.Errorf("数据库未初始化")
	}

	normalizeDevice(&device)
	if err := ValidateDevice(&device); err != nil {
		return err
	}

	var count int64
	if err := DB.Model(&models.DeviceAsset{}).Where("ip = ?", device.IP).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("IP 地址 %s 已存在", device.IP)
	}

	device.ID = 0
	return DB.Create(&device).Error
}

// CreateDevices 批量创建设备
// 支持IP范围语法糖展开，如 "192.168.1.1-3" 展开为 3 台设备
func CreateDevices(devices []models.DeviceAsset) error {
	if DB == nil {
		return fmt.Errorf("数据库未初始化")
	}
	if len(devices) == 0 {
		return nil
	}

	// 展开IP范围语法糖
	expandedDevices := make([]models.DeviceAsset, 0, len(devices))
	for _, device := range devices {
		// 使用 forge.ExpandSyntaxSugar 展开IP范围
		ips, err := forge.ExpandSyntaxSugar(device.IP)
		if err != nil {
			return fmt.Errorf("IP 地址展开失败: %s, 错误: %v", device.IP, err)
		}

		// 为每个展开后的IP创建设备
		for _, ip := range ips {
			newDevice := device
			newDevice.IP = ip
			newDevice.ID = 0
			expandedDevices = append(expandedDevices, newDevice)
		}
	}

	if err := validateDevicesForWrite(expandedDevices); err != nil {
		return err
	}

	ips := make([]string, 0, len(expandedDevices))
	for _, device := range expandedDevices {
		ips = append(ips, device.IP)
	}

	var existing []models.DeviceAsset
	if err := DB.Where("ip IN ?", ips).Find(&existing).Error; err != nil {
		return err
	}
	if len(existing) > 0 {
		return fmt.Errorf("IP 地址 %s 已存在", existing[0].IP)
	}

	return DB.Create(&expandedDevices).Error
}

// UpdateDevice 更新单台设备
func UpdateDevice(id uint, device models.DeviceAsset) error {
	if DB == nil {
		return fmt.Errorf("数据库未初始化")
	}
	if id == 0 {
		return fmt.Errorf("无效的设备 ID")
	}

	device.ID = id
	if err := validateDevicesForWrite([]models.DeviceAsset{device}); err != nil {
		return err
	}

	return DB.Transaction(func(tx *gorm.DB) error {
		var existing models.DeviceAsset
		if err := tx.First(&existing, id).Error; err != nil {
			return fmt.Errorf("未找到设备: %d", id)
		}
		logger.Verbose(
			"Config",
			existing.IP,
			"收到单设备更新请求: id=%d, protocol=%s->%s, port=%d->%d, group=%q->%q, username=%q->%q",
			id,
			existing.Protocol, device.Protocol,
			existing.Port, device.Port,
			existing.Group, device.Group,
			existing.Username, device.Username,
		)

		var conflict models.DeviceAsset
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
		logger.Verbose("Config", device.IP, "单设备更新完成: id=%d", id)
		return nil
	})
}

// UpdateDevices 批量更新设备
func UpdateDevices(devices []models.DeviceAsset) error {
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

	logger.Verbose("Config", "-", "收到批量设备更新请求: count=%d", len(devices))
	return DB.Transaction(func(tx *gorm.DB) error {
		var existing []models.DeviceAsset
		if err := tx.Where("id IN ?", ids).Find(&existing).Error; err != nil {
			return err
		}
		if len(existing) != len(ids) {
			return fmt.Errorf("部分设备不存在，无法完成批量更新")
		}
		existingByID := make(map[uint]models.DeviceAsset, len(existing))
		for _, item := range existing {
			existingByID[item.ID] = item
		}

		var conflicts []models.DeviceAsset
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
				logger.Verbose(
					"Config",
					old.IP,
					"批量更新设备: id=%d, protocol=%s->%s, port=%d->%d, group=%q->%q, username=%q->%q",
					device.ID,
					old.Protocol, device.Protocol,
					old.Port, device.Port,
					old.Group, device.Group,
					old.Username, device.Username,
				)
			}
			if err := tx.Save(&device).Error; err != nil {
				return err
			}
		}
		logger.Verbose("Config", "-", "批量设备更新完成: count=%d", len(devices))
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

	result := DB.Delete(&models.DeviceAsset{}, id)
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

	result := DB.Where("id IN ?", ids).Delete(&models.DeviceAsset{})
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
