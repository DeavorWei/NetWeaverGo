package ui

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/models"
	"github.com/NetWeaverGo/core/internal/repository"
	"github.com/NetWeaverGo/core/internal/taskexec"
	"github.com/wailsapp/wails/v3/pkg/application"
)

const defaultTopologyCommandTimeoutSec = 30

// TopologyCommandService 拓扑命令预览与配置服务。
type TopologyCommandService struct {
	wailsApp *application.App
	repo     repository.DeviceRepository
}

// NewTopologyCommandService 创建拓扑命令预览服务实例。
func NewTopologyCommandService() *TopologyCommandService {
	return &TopologyCommandService{
		repo: repository.NewDeviceRepository(),
	}
}

// ServiceStartup Wails 服务启动生命周期钩子。
func (s *TopologyCommandService) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	s.wailsApp = application.Get()
	return nil
}

// GetSupportedTopologyVendors 返回系统支持的拓扑厂商列表。
func (s *TopologyCommandService) GetSupportedTopologyVendors() []string {
	resolver := taskexec.NewTopologyCommandResolver()
	vendors := resolver.SupportedVendors()
	if len(vendors) == 0 {
		return []string{"huawei", "h3c", "cisco"}
	}
	return vendors
}

// GetTopologyFieldCatalog 返回拓扑字段目录。
func (s *TopologyCommandService) GetTopologyFieldCatalog() []models.TopologyFieldSpec {
	return taskexec.GetTopologyFieldCatalog()
}

// GetVendorCommandConfig 查询厂商默认命令配置。
func (s *TopologyCommandService) GetVendorCommandConfig(vendor string) (*TopologyVendorCommandSetView, error) {
	if err := taskexec.EnsureTopologyCommandSeeds(); err != nil {
		return nil, err
	}

	normalizedVendor, profile, err := s.resolveVendorProfile(vendor)
	if err != nil {
		return nil, err
	}

	catalog := taskexec.GetTopologyFieldCatalog()
	records, err := config.GetTopologyVendorFieldCommands(normalizedVendor)
	if err != nil {
		return nil, err
	}
	recordMap := make(map[string]models.TopologyVendorFieldCommand, len(records))
	for _, record := range records {
		recordMap[strings.TrimSpace(record.FieldKey)] = record
	}
	profileMap := profileCommandsByKey(profile)

	result := &TopologyVendorCommandSetView{
		Vendor:   normalizedVendor,
		Commands: make([]TopologyVendorCommandItemView, 0, len(catalog)),
	}
	for _, spec := range catalog {
		item := TopologyVendorCommandItemView{
			FieldKey:      spec.FieldKey,
			DisplayName:   spec.Name,
			ParserBinding: spec.ParserBinding,
			Description:   spec.Description,
			Required:      spec.Required,
			TimeoutSec:    defaultTopologyCommandTimeoutSec,
			Enabled:       false,
			Source:        "field_default",
		}
		if record, ok := recordMap[spec.FieldKey]; ok {
			item.Command = strings.TrimSpace(record.Command)
			item.TimeoutSec = clampTimeout(record.TimeoutSec)
			item.Enabled = record.Enabled && item.Command != ""
			item.Notes = strings.TrimSpace(record.Notes)
			item.UpdatedAt = record.UpdatedAt
			item.Source = "vendor_config"
		} else if profileCommand, ok := profileMap[spec.FieldKey]; ok {
			item.Command = strings.TrimSpace(profileCommand.Command)
			item.TimeoutSec = clampTimeout(profileCommand.TimeoutSec)
			item.Enabled = spec.DefaultEnabled && item.Command != ""
			item.Notes = "seeded_from_device_profile"
			item.Source = "profile_seed"
		}
		result.Commands = append(result.Commands, item)
	}

	return result, nil
}

// SaveVendorCommandConfig 保存厂商默认命令配置。
func (s *TopologyCommandService) SaveVendorCommandConfig(request TopologyVendorCommandSaveRequest) (*TopologyVendorCommandSetView, error) {
	if err := taskexec.EnsureTopologyCommandSeeds(); err != nil {
		return nil, err
	}

	normalizedVendor, profile, err := s.resolveVendorProfile(request.Vendor)
	if err != nil {
		return nil, err
	}
	catalog := taskexec.GetTopologyFieldCatalog()
	if len(catalog) == 0 {
		return nil, fmt.Errorf("拓扑字段目录为空")
	}

	catalogMap := make(map[string]models.TopologyFieldSpec, len(catalog))
	for _, spec := range catalog {
		catalogMap[spec.FieldKey] = spec
	}

	base, err := s.GetVendorCommandConfig(normalizedVendor)
	if err != nil {
		return nil, err
	}
	merged := make(map[string]TopologyVendorCommandItemView, len(base.Commands))
	for _, item := range base.Commands {
		merged[item.FieldKey] = item
	}

	for _, item := range request.Commands {
		fieldKey := strings.TrimSpace(item.FieldKey)
		if fieldKey == "" {
			continue
		}
		if _, ok := catalogMap[fieldKey]; !ok {
			return nil, fmt.Errorf("无效字段键: %s", fieldKey)
		}
		current := merged[fieldKey]
		current.Command = strings.TrimSpace(item.Command)
		current.TimeoutSec = item.TimeoutSec
		current.Enabled = item.Enabled
		current.Notes = strings.TrimSpace(item.Notes)
		merged[fieldKey] = current
	}

	profileMap := profileCommandsByKey(profile)
	persist := make([]models.TopologyVendorFieldCommand, 0, len(catalog))
	for _, spec := range catalog {
		item, ok := merged[spec.FieldKey]
		if !ok {
			item = TopologyVendorCommandItemView{FieldKey: spec.FieldKey}
		}

		command := strings.TrimSpace(item.Command)
		timeoutSec := item.TimeoutSec
		if timeoutSec <= 0 {
			if profileCommand, ok := profileMap[spec.FieldKey]; ok {
				timeoutSec = clampTimeout(profileCommand.TimeoutSec)
			} else {
				timeoutSec = defaultTopologyCommandTimeoutSec
			}
		}
		enabled := item.Enabled
		if enabled && command == "" {
			return nil, fmt.Errorf("字段 %s 已启用但命令为空", spec.FieldKey)
		}
		if timeoutSec <= 0 {
			return nil, fmt.Errorf("字段 %s 超时时间必须大于 0", spec.FieldKey)
		}

		persist = append(persist, models.TopologyVendorFieldCommand{
			Vendor:     normalizedVendor,
			FieldKey:   spec.FieldKey,
			Command:    command,
			TimeoutSec: timeoutSec,
			Enabled:    enabled && command != "",
			Notes:      strings.TrimSpace(item.Notes),
		})
	}

	if err := config.SaveTopologyVendorFieldCommands(normalizedVendor, persist); err != nil {
		return nil, err
	}
	return s.GetVendorCommandConfig(normalizedVendor)
}

// ResetVendorCommandConfig 将厂商默认命令重置为系统种子。
func (s *TopologyCommandService) ResetVendorCommandConfig(vendor string) (*TopologyVendorCommandSetView, error) {
	normalizedVendor, profile, err := s.resolveVendorProfile(vendor)
	if err != nil {
		return nil, err
	}

	catalog := taskexec.GetTopologyFieldCatalog()
	profileMap := profileCommandsByKey(profile)
	seed := make([]models.TopologyVendorFieldCommand, 0, len(catalog))
	for _, spec := range catalog {
		item := models.TopologyVendorFieldCommand{
			Vendor:     normalizedVendor,
			FieldKey:   spec.FieldKey,
			TimeoutSec: defaultTopologyCommandTimeoutSec,
			Enabled:    false,
			Notes:      "reset_from_device_profile",
		}
		if cmd, ok := profileMap[spec.FieldKey]; ok {
			item.Command = strings.TrimSpace(cmd.Command)
			item.TimeoutSec = clampTimeout(cmd.TimeoutSec)
			item.Enabled = spec.DefaultEnabled && item.Command != ""
		}
		seed = append(seed, item)
	}
	if err := config.SaveTopologyVendorFieldCommands(normalizedVendor, seed); err != nil {
		return nil, err
	}
	return s.GetVendorCommandConfig(normalizedVendor)
}

// PreviewTopologyCommands 返回当前任务配置下的拓扑命令预览。
func (s *TopologyCommandService) PreviewTopologyCommands(taskVendor string, deviceIDs []uint, overrides []models.TopologyTaskFieldOverride) (*TopologyCommandPreviewView, error) {
	resolver := taskexec.NewTopologyCommandResolver()
	preview := &TopologyCommandPreviewView{
		SupportedVendors: s.GetSupportedTopologyVendors(),
		FieldCatalog:     taskexec.GetTopologyFieldCatalog(),
		Devices:          make([]TopologyPreviewDeviceView, 0, len(deviceIDs)),
	}

	defaultResolution, err := resolver.Resolve(taskVendor, nil, overrides)
	if err != nil {
		return nil, err
	}
	preview.DefaultResolution = convertTopologyResolution(defaultResolution)

	devices, err := s.findDevicesByIDs(deviceIDs)
	if err != nil {
		return nil, err
	}
	for _, device := range devices {
		resolution, err := resolver.Resolve(taskVendor, &device, overrides)
		if err != nil {
			return nil, err
		}
		preview.Devices = append(preview.Devices, TopologyPreviewDeviceView{
			DeviceID:        device.ID,
			DeviceIP:        strings.TrimSpace(device.IP),
			InventoryVendor: strings.ToLower(strings.TrimSpace(device.Vendor)),
			Resolution:      convertTopologyResolution(resolution),
		})
	}

	sort.Slice(preview.Devices, func(i, j int) bool {
		return preview.Devices[i].DeviceIP < preview.Devices[j].DeviceIP
	})
	return preview, nil
}

func (s *TopologyCommandService) findDevicesByIDs(deviceIDs []uint) ([]models.DeviceAsset, error) {
	if len(deviceIDs) == 0 {
		return []models.DeviceAsset{}, nil
	}
	if s.repo == nil {
		s.repo = repository.NewDeviceRepository()
	}

	allDevices, err := s.repo.FindAll()
	if err != nil {
		return nil, err
	}
	selected := make(map[uint]struct{}, len(deviceIDs))
	for _, id := range deviceIDs {
		if id == 0 {
			continue
		}
		selected[id] = struct{}{}
	}

	result := make([]models.DeviceAsset, 0, len(selected))
	for _, device := range allDevices {
		if _, ok := selected[device.ID]; !ok {
			continue
		}
		result = append(result, device)
	}
	return result, nil
}

func convertTopologyResolution(resolution *taskexec.TopologyCommandResolution) TopologyCommandResolutionView {
	if resolution == nil {
		return TopologyCommandResolutionView{Commands: []TopologyResolvedCommandView{}}
	}
	commands := make([]TopologyResolvedCommandView, 0, len(resolution.Commands))
	for _, item := range resolution.Commands {
		commands = append(commands, TopologyResolvedCommandView{
			FieldKey:       item.FieldKey,
			DisplayName:    item.DisplayName,
			Command:        item.Command,
			TimeoutSec:     item.TimeoutSec,
			Enabled:        item.Enabled,
			CommandSource:  item.CommandSource,
			ParserBinding:  item.ParserBinding,
			ResolvedVendor: item.ResolvedVendor,
			VendorSource:   item.VendorSource,
			Required:       item.Required,
			Description:    item.Description,
		})
	}
	return TopologyCommandResolutionView{
		ResolvedVendor: resolution.ResolvedVendor,
		VendorSource:   resolution.VendorSource,
		ProfileVendor:  resolution.ProfileVendor,
		Commands:       commands,
	}
}

func (s *TopologyCommandService) resolveVendorProfile(vendor string) (string, *config.DeviceProfile, error) {
	normalizedVendor := strings.ToLower(strings.TrimSpace(vendor))
	if normalizedVendor == "" {
		return "", nil, fmt.Errorf("厂商不能为空")
	}
	profile, ok := config.GetDeviceProfileByVendor(normalizedVendor)
	if !ok || profile == nil {
		return "", nil, fmt.Errorf("不支持的厂商: %s", normalizedVendor)
	}
	return normalizedVendor, profile, nil
}

func profileCommandsByKey(profile *config.DeviceProfile) map[string]config.CommandSpec {
	result := make(map[string]config.CommandSpec)
	if profile == nil {
		return result
	}
	for _, item := range profile.Commands {
		fieldKey := strings.TrimSpace(item.CommandKey)
		if fieldKey == "" {
			continue
		}
		result[fieldKey] = item
	}
	return result
}

func clampTimeout(timeoutSec int) int {
	if timeoutSec <= 0 {
		return defaultTopologyCommandTimeoutSec
	}
	return timeoutSec
}
