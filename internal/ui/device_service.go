package ui

import (
	"context"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
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
func (s *DeviceService) ListDevices() ([]models.DeviceAsset, error) {
	return config.LoadDeviceAssets()
}

// GetDeviceByID 根据 ID 获取单个设备详情（包含解密后的密码，用于编辑）
func (s *DeviceService) GetDeviceByID(id uint) (*models.DeviceAssetResponse, error) {
	device, err := config.LoadDeviceByID(id)
	if err != nil {
		return nil, err
	}
	logger.Debug("DeviceService", "-", "GetDeviceByID: ID=%d, Password='%s'", id, device.Password)
	resp := device.ToResponse()
	logger.Debug("DeviceService", "-", "GetDeviceByID Response: Password='%s'", resp.Password)
	return resp, nil
}

// AddDevice 新增设备
func (s *DeviceService) AddDevice(device models.DeviceAsset) error {
	return config.CreateDevice(device)
}

// AddDevices 批量新增设备
func (s *DeviceService) AddDevices(devices []models.DeviceAsset) error {
	return config.CreateDevices(devices)
}

// UpdateDevice 更新设备
func (s *DeviceService) UpdateDevice(id uint, device models.DeviceAsset) error {
	return config.UpdateDevice(id, device)
}

// UpdateDevices 批量更新设备
func (s *DeviceService) UpdateDevices(devices []models.DeviceAsset) error {
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
