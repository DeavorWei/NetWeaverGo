package ui

import (
	"context"
	"fmt"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// DeviceService 设备管理服务 - 负责设备的增删改查
type DeviceService struct {
	wailsApp *application.App
}

// NewDeviceService 创建设备服务实例
func NewDeviceService() *DeviceService {
	return &DeviceService{}
}

// ServiceStartup Wails 服务启动生命周期钩子
func (s *DeviceService) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	s.wailsApp = application.Get()
	return nil
}

// ListDevices 获取设备列表
func (s *DeviceService) ListDevices() ([]config.DeviceAsset, error) {
	assets, _, _, _, err := config.ParseOrGenerate(false)
	return assets, err
}

// AddDevice 新增设备
func (s *DeviceService) AddDevice(device config.DeviceAsset) error {
	// 校验设备信息
	if err := config.ValidateDevice(device); err != nil {
		return err
	}

	// 读取现有设备列表
	devices, _, _, _, err := config.ParseOrGenerate(false)
	if err != nil {
		return err
	}

	// 检查 IP 是否已存在
	for _, d := range devices {
		if d.IP == device.IP {
			return fmt.Errorf("IP 地址 %s 已存在", device.IP)
		}
	}

	// 添加新设备
	devices = append(devices, device)

	// 保存到文件
	return config.SaveInventory(devices)
}

// UpdateDevice 更新设备
func (s *DeviceService) UpdateDevice(index int, device config.DeviceAsset) error {
	// 校验设备信息
	if err := config.ValidateDevice(device); err != nil {
		return err
	}

	// 读取现有设备列表
	devices, _, _, _, err := config.ParseOrGenerate(false)
	if err != nil {
		return err
	}

	// 检查索引有效性
	if index < 0 || index >= len(devices) {
		return fmt.Errorf("无效的设备索引: %d", index)
	}

	// 检查 IP 是否与其他设备冲突
	for i, d := range devices {
		if i != index && d.IP == device.IP {
			return fmt.Errorf("IP 地址 %s 已被其他设备使用", device.IP)
		}
	}

	// 更新设备
	devices[index] = device

	// 保存到文件
	return config.SaveInventory(devices)
}

// DeleteDevice 删除设备
func (s *DeviceService) DeleteDevice(index int) error {
	// 读取现有设备列表
	devices, _, _, _, err := config.ParseOrGenerate(false)
	if err != nil {
		return err
	}

	// 检查索引有效性
	if index < 0 || index >= len(devices) {
		return fmt.Errorf("无效的设备索引: %d", index)
	}

	// 删除设备
	devices = append(devices[:index], devices[index+1:]...)

	// 保存到文件
	return config.SaveInventory(devices)
}

// SaveDevices 批量保存设备列表
func (s *DeviceService) SaveDevices(devices []config.DeviceAsset) error {
	// 校验所有设备
	for i, device := range devices {
		if err := config.ValidateDevice(device); err != nil {
			return fmt.Errorf("第 %d 台设备: %v", i+1, err)
		}
	}

	// 检查 IP 重复
	ipSet := make(map[string]bool)
	for _, device := range devices {
		if ipSet[device.IP] {
			return fmt.Errorf("存在重复的 IP 地址: %s", device.IP)
		}
		ipSet[device.IP] = true
	}

	return config.SaveInventory(devices)
}

// GetProtocolDefaultPorts 获取协议默认端口映射
func (s *DeviceService) GetProtocolDefaultPorts() map[string]int {
	return config.ProtocolDefaultPorts
}

// GetValidProtocols 获取有效协议列表
func (s *DeviceService) GetValidProtocols() []string {
	return config.ValidProtocols
}
