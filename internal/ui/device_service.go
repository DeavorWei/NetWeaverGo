package ui

import (
	"context"
	"fmt"
	"strings"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/models"
	"github.com/NetWeaverGo/core/internal/repository"
	"github.com/NetWeaverGo/core/internal/sshutil"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// DeviceService 设备管理服务 - 负责设备的增删改查
type DeviceService struct {
	wailsApp *application.App
	repo     repository.DeviceRepository
}

// NewDeviceService 创建设备服务实例
func NewDeviceService() *DeviceService {
	return &DeviceService{
		repo: repository.NewDeviceRepository(),
	}
}

// NewDeviceServiceWithRepo 使用指定 Repository 创建设备服务实例（用于测试）
func NewDeviceServiceWithRepo(repo repository.DeviceRepository) *DeviceService {
	return &DeviceService{
		repo: repo,
	}
}

// ServiceStartup Wails 服务启动生命周期钩子
func (s *DeviceService) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	s.wailsApp = application.Get()
	return nil
}

// ListDevices 获取设备列表（不含密码）
// 列表场景不返回密码，密码仅在单设备详情接口中返回
func (s *DeviceService) ListDevices() ([]models.DeviceAssetListItem, error) {
	devices, err := s.repo.FindAll()
	if err != nil {
		return nil, err
	}

	items := make([]models.DeviceAssetListItem, len(devices))
	for i, d := range devices {
		items[i] = d.ToListItem()
	}
	return items, nil
}

// GetDeviceByID 根据 ID 获取单个设备详情（包含解密后的密码，用于编辑）
func (s *DeviceService) GetDeviceByID(id uint) (*models.DeviceAssetResponse, error) {
	device, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}
	resp := device.ToResponse()
	return resp, nil
}

// ResetDeviceSSHHostKey 清理指定设备在 known_hosts 中的主机密钥记录。
func (s *DeviceService) ResetDeviceSSHHostKey(id uint) error {
	if id == 0 {
		return fmt.Errorf("无效的设备 ID")
	}

	device, err := s.repo.FindByID(id)
	if err != nil {
		return fmt.Errorf("未找到设备: %d", id)
	}

	knownHostsPath := config.GetPathManager().GetSSHKnownHostsPath()
	removed, err := sshutil.RemoveKnownHost(knownHostsPath, device.IP)
	if err != nil {
		return fmt.Errorf("重置设备 %s 的 SSH 主机密钥失败: %w", device.IP, err)
	}
	if !removed {
		return fmt.Errorf("设备 %s 未找到可清理的 SSH 主机密钥记录", device.IP)
	}
	return nil
}

// AddDevice 新增设备
func (s *DeviceService) AddDevice(device models.DeviceAsset) error {
	// 标准化设备信息
	config.NormalizeDevice(&device)

	// 校验设备信息
	if err := config.ValidateDevice(&device); err != nil {
		return err
	}

	// 检查 IP 是否已存在
	exists, err := s.repo.ExistsByIP(device.IP)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("IP 地址 %s 已存在", device.IP)
	}

	return s.repo.Create(&device)
}

// AddDevices 批量新增设备
func (s *DeviceService) AddDevices(devices []models.DeviceAsset) error {
	if len(devices) == 0 {
		return nil
	}

	expandedDevices := make([]models.DeviceAsset, 0, len(devices))

	for i := range devices {
		config.NormalizeDevice(&devices[i])

		rangeResult, rangeErr := parseIPv4LastOctetRange(devices[i].IP)
		if rangeErr != nil {
			return fmt.Errorf("第 %d 台设备: %v", i+1, rangeErr)
		}

		if rangeResult != nil {
			for _, ip := range rangeResult.List {
				newDevice := devices[i]
				newDevice.IP = ip
				if err := config.ValidateDevice(&newDevice); err != nil {
					return fmt.Errorf("第 %d 台设备: 展开后 IP %s 校验失败: %v", i+1, ip, err)
				}
				expandedDevices = append(expandedDevices, newDevice)
			}
			continue
		}

		if strings.Contains(devices[i].IP, "-") || strings.Contains(devices[i].IP, "~") {
			return fmt.Errorf("第 %d 台设备: 无法识别IP范围格式，期望格式如: 192.168.1.10-20", i+1)
		}

		if err := config.ValidateDevice(&devices[i]); err != nil {
			return fmt.Errorf("第 %d 台设备: %v", i+1, err)
		}
		expandedDevices = append(expandedDevices, devices[i])
	}

	// 检查展开后的重复 IP
	ipSet := make(map[string]struct{}, len(expandedDevices))
	for _, d := range expandedDevices {
		if _, exists := ipSet[d.IP]; exists {
			return fmt.Errorf("存在重复的 IP 地址: %s", d.IP)
		}
		ipSet[d.IP] = struct{}{}
	}

	// 检查 IP 是否已存在数据库中
	ips := make([]string, 0, len(expandedDevices))
	for _, d := range expandedDevices {
		ips = append(ips, d.IP)
	}
	existing, err := s.repo.FindByIPs(ips)
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		return fmt.Errorf("IP 地址 %s 已存在", existing[0].IP)
	}

	return s.repo.CreateBatch(expandedDevices)
}

// UpdateDevice 更新设备
func (s *DeviceService) UpdateDevice(id uint, device models.DeviceAsset) error {
	if id == 0 {
		return fmt.Errorf("无效的设备 ID")
	}

	// 获取现有设备
	existing, err := s.repo.FindByID(id)
	if err != nil {
		return fmt.Errorf("未找到设备: %d", id)
	}

	// 字段合并：非零值覆盖，零值保留原值
	if device.IP != "" {
		existing.IP = device.IP
	}
	if device.Protocol != "" {
		existing.Protocol = device.Protocol
	}
	if device.Port != 0 {
		existing.Port = device.Port
	}
	if device.Username != "" {
		existing.Username = device.Username
	}
	// Password: 使用统一密码合并规则
	pwdResult := config.MergePassword(existing.Password, device.Password)
	existing.Password = pwdResult.Password
	if device.Group != "" {
		existing.Group = device.Group
	}
	if device.DisplayName != "" {
		existing.DisplayName = device.DisplayName
	}
	if device.Vendor != "" {
		existing.Vendor = device.Vendor
	}
	if device.Role != "" {
		existing.Role = device.Role
	}
	if device.Site != "" {
		existing.Site = device.Site
	}
	if device.Description != "" {
		existing.Description = device.Description
	}
	if len(device.Tags) > 0 {
		existing.Tags = device.Tags
	}

	// 标准化
	config.NormalizeDevice(existing)

	// 校验
	if err := config.ValidateDevice(existing); err != nil {
		return err
	}

	// IP 冲突检查
	if device.IP != "" {
		conflict, err := s.repo.FindByIP(device.IP)
		if err == nil && conflict.ID != id {
			return fmt.Errorf("IP 地址 %s 已被其他设备使用", device.IP)
		}
	}

	return s.repo.Update(existing)
}

// UpdateDevices 批量更新设备
func (s *DeviceService) UpdateDevices(devices []models.DeviceAsset) error {
	if len(devices) == 0 {
		return nil
	}

	// 校验
	for i := range devices {
		config.NormalizeDevice(&devices[i])
		if err := config.ValidateDevice(&devices[i]); err != nil {
			return fmt.Errorf("第 %d 台设备: %v", i+1, err)
		}
		if devices[i].ID == 0 {
			return fmt.Errorf("批量更新时存在无效设备 ID")
		}
	}

	// 获取现有设备
	ids := make([]uint, 0, len(devices))
	for _, d := range devices {
		ids = append(ids, d.ID)
	}

	// 处理密码合并
	for i := range devices {
		existing, err := s.repo.FindByID(devices[i].ID)
		if err != nil {
			return fmt.Errorf("未找到设备: %d", devices[i].ID)
		}
		pwdResult := config.MergePassword(existing.Password, devices[i].Password)
		devices[i].Password = pwdResult.Password
	}

	return s.repo.UpdateBatch(devices)
}

// DeleteDevice 删除设备
func (s *DeviceService) DeleteDevice(id uint) error {
	return s.repo.Delete(id)
}

// DeleteDevices 批量删除设备
func (s *DeviceService) DeleteDevices(ids []uint) error {
	return s.repo.DeleteBatch(ids)
}

// GetProtocolDefaultPorts 获取协议默认端口映射
func (s *DeviceService) GetProtocolDefaultPorts() map[string]int {
	return config.ProtocolDefaultPorts
}

// GetValidProtocols 获取有效协议列表
func (s *DeviceService) GetValidProtocols() []string {
	return config.ValidProtocols
}
