package ui

import (
	"context"
	"sort"
	"strings"

	"github.com/NetWeaverGo/core/internal/models"
	"github.com/NetWeaverGo/core/internal/repository"
	"github.com/NetWeaverGo/core/internal/taskexec"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// TopologyCommandService 拓扑命令预览服务。
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
