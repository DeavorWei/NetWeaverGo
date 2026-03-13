package ui

import (
	"context"

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
	return config.LoadDeviceAssets()
}

// AddDevice 新增设备
func (s *DeviceService) AddDevice(device config.DeviceAsset) error {
	return config.CreateDevice(device)
}

// AddDevices 批量新增设备
func (s *DeviceService) AddDevices(devices []config.DeviceAsset) error {
	return config.CreateDevices(devices)
}

// UpdateDevice 更新设备
func (s *DeviceService) UpdateDevice(id uint, device config.DeviceAsset) error {
	return config.UpdateDevice(id, device)
}

// UpdateDevices 批量更新设备
func (s *DeviceService) UpdateDevices(devices []config.DeviceAsset) error {
	return config.UpdateDevices(devices)
}

// DeleteDevice 删除设备
func (s *DeviceService) DeleteDevice(id uint) error {
	return config.DeleteDevice(id)
}

// DeleteDevices 批量删除设备
func (s *DeviceService) DeleteDevices(ids []uint) error {
	return config.DeleteDevices(ids)
}

// GetProtocolDefaultPorts 获取协议默认端口映射
func (s *DeviceService) GetProtocolDefaultPorts() map[string]int {
	return config.ProtocolDefaultPorts
}

// GetValidProtocols 获取有效协议列表
func (s *DeviceService) GetValidProtocols() []string {
	return config.ValidProtocols
}
