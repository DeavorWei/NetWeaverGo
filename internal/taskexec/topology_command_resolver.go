package taskexec

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
)

const defaultTopologyVendor = "huawei"

const (
	TopologyVendorSourceTask      = "task"
	TopologyVendorSourceInventory = "inventory"
	TopologyVendorSourceFallback  = "fallback_default"
)

const (
	TopologyCommandSourceTaskOverride = "task_override"
	TopologyCommandSourceVendorConfig = "vendor_config"
	TopologyCommandSourceProfileSeed  = "profile_seed"
	TopologyCommandSourceBuiltinSeed  = "builtin_seed"
	TopologyCommandSourceDisabled     = "disabled"
)

// ResolvedTopologyCommand 拓扑命令最终解析结果。
type ResolvedTopologyCommand struct {
	FieldKey       string `json:"fieldKey"`
	DisplayName    string `json:"displayName"`
	Command        string `json:"command"`
	TimeoutSec     int    `json:"timeoutSec"`
	Enabled        bool   `json:"enabled"`
	CommandSource  string `json:"commandSource"`
	ParserBinding  string `json:"parserBinding"`
	ResolvedVendor string `json:"resolvedVendor"`
	VendorSource   string `json:"vendorSource"`
	Required       bool   `json:"required"`
	Description    string `json:"description"`
}

// TopologyCommandResolution 拓扑命令统一解析结果。
type TopologyCommandResolution struct {
	ResolvedVendor string                    `json:"resolvedVendor"`
	VendorSource   string                    `json:"vendorSource"`
	ProfileVendor  string                    `json:"profileVendor"`
	Commands       []ResolvedTopologyCommand `json:"commands"`
}

// TopologyCommandResolver 负责统一解析拓扑采集命令计划。
type TopologyCommandResolver struct{}

var (
	topologyCommandSeedOnce sync.Once
	topologyCommandSeedErr  error
)

var topologyFieldCatalog = []models.TopologyFieldSpec{
	{FieldKey: "version", Name: "系统版本", Phase: "collect", Required: true, ParserBinding: "version", DefaultEnabled: true, Description: "采集设备版本与系统镜像信息。"},
	{FieldKey: "sysname", Name: "设备名称", Phase: "collect", Required: true, ParserBinding: "sysname", DefaultEnabled: true, Description: "采集设备 sysname 或 hostname。"},
	{FieldKey: "esn", Name: "设备序列号", Phase: "collect", Required: false, ParserBinding: "esn", DefaultEnabled: true, Description: "采集设备电子序列号。"},
	{FieldKey: "device_info", Name: "设备信息", Phase: "collect", Required: true, ParserBinding: "device_info", DefaultEnabled: true, Description: "采集机型、板卡与设备摘要信息。"},
	{FieldKey: "interface_brief", Name: "接口概要", Phase: "collect", Required: true, ParserBinding: "interface_brief", DefaultEnabled: true, Description: "采集接口 up/down 与基础摘要。"},
	{FieldKey: "interface_detail", Name: "接口详情", Phase: "collect", Required: false, ParserBinding: "interface_detail", DefaultEnabled: true, Description: "采集接口详细属性。"},
	{FieldKey: "lldp_neighbor", Name: "LLDP 邻居", Phase: "collect", Required: true, ParserBinding: "lldp_neighbor", DefaultEnabled: true, Description: "采集 LLDP 邻居发现结果。"},
	{FieldKey: "mac_address", Name: "MAC 地址表", Phase: "collect", Required: true, ParserBinding: "mac_address", DefaultEnabled: true, Description: "采集 MAC/FDB 地址学习表。"},
	{FieldKey: "arp_all", Name: "ARP 表", Phase: "collect", Required: true, ParserBinding: "arp_all", DefaultEnabled: true, Description: "采集 ARP 地址表。"},
	{FieldKey: "eth_trunk", Name: "聚合链路", Phase: "collect", Required: false, ParserBinding: "eth_trunk", DefaultEnabled: true, Description: "采集 Eth-Trunk/Port-Channel 聚合信息。"},
}

func NewTopologyCommandResolver() *TopologyCommandResolver {
	return &TopologyCommandResolver{}
}

// GetTopologyFieldCatalog 返回固定字段目录。
func GetTopologyFieldCatalog() []models.TopologyFieldSpec {
	result := make([]models.TopologyFieldSpec, len(topologyFieldCatalog))
	copy(result, topologyFieldCatalog)
	return result
}

// EnsureTopologyCommandSeeds 确保内置画像命令已写入配置域表。
func EnsureTopologyCommandSeeds() error {
	topologyCommandSeedOnce.Do(func() {
		topologyCommandSeedErr = config.EnsureTopologyVendorCommandSeeds(buildTopologyCommandSeeds())
	})
	return topologyCommandSeedErr
}

// SupportedVendors 返回系统支持的拓扑厂商目录。
func (r *TopologyCommandResolver) SupportedVendors() []string {
	profiles := config.GetAllDeviceProfiles()
	vendors := make([]string, 0, len(profiles))
	seen := make(map[string]struct{})
	for _, profile := range profiles {
		if profile == nil {
			continue
		}
		vendor := strings.ToLower(strings.TrimSpace(profile.Vendor))
		if vendor == "" {
			continue
		}
		if _, ok := seen[vendor]; ok {
			continue
		}
		seen[vendor] = struct{}{}
		vendors = append(vendors, vendor)
	}
	sort.Strings(vendors)
	return vendors
}

// Resolve 为指定设备生成统一拓扑命令计划。
func (r *TopologyCommandResolver) Resolve(taskVendor string, device *models.DeviceAsset, overrides []models.TopologyTaskFieldOverride) (*TopologyCommandResolution, error) {
	if err := EnsureTopologyCommandSeeds(); err != nil {
		return nil, err
	}

	resolvedVendor, vendorSource := r.resolveVendor(taskVendor, device)
	profile, ok := config.GetDeviceProfileByVendor(resolvedVendor)
	if !ok || profile == nil {
		profile = config.GetDeviceProfile(defaultTopologyVendor)
	}
	if profile == nil {
		return nil, fmt.Errorf("拓扑命令解析失败: 无法加载厂商画像 %s", resolvedVendor)
	}

	vendorCommands, err := config.GetTopologyVendorFieldCommands(profile.Vendor)
	useBuiltinSeed := false
	if err != nil {
		logger.Warn("TaskExec", "-", "读取拓扑厂商命令配置失败，回退内置种子: vendor=%s, err=%v", profile.Vendor, err)
		vendorCommands = nil
		useBuiltinSeed = true
	}
	vendorCommandMap := make(map[string]models.TopologyVendorFieldCommand, len(vendorCommands))
	for _, item := range vendorCommands {
		vendorCommandMap[strings.TrimSpace(item.FieldKey)] = item
	}

	profileCommandMap := make(map[string]config.CommandSpec, len(profile.Commands))
	for _, item := range profile.Commands {
		profileCommandMap[strings.TrimSpace(item.CommandKey)] = item
	}

	overrideMap := normalizeTopologyOverrides(overrides)
	resolved := make([]ResolvedTopologyCommand, 0, len(topologyFieldCatalog))
	for _, spec := range topologyFieldCatalog {
		item := ResolvedTopologyCommand{
			FieldKey:       spec.FieldKey,
			DisplayName:    spec.Name,
			TimeoutSec:     0,
			Enabled:        spec.DefaultEnabled,
			CommandSource:  TopologyCommandSourceDisabled,
			ParserBinding:  spec.ParserBinding,
			ResolvedVendor: profile.Vendor,
			VendorSource:   vendorSource,
			Required:       spec.Required,
			Description:    spec.Description,
		}

		if vendorItem, ok := vendorCommandMap[spec.FieldKey]; ok {
			item.Command = strings.TrimSpace(vendorItem.Command)
			item.TimeoutSec = vendorItem.TimeoutSec
			item.Enabled = vendorItem.Enabled
			item.CommandSource = TopologyCommandSourceVendorConfig
		} else if profileItem, ok := profileCommandMap[spec.FieldKey]; ok {
			item.Command = strings.TrimSpace(profileItem.Command)
			item.TimeoutSec = profileItem.TimeoutSec
			item.Enabled = spec.DefaultEnabled && item.Command != ""
			if useBuiltinSeed {
				item.CommandSource = TopologyCommandSourceBuiltinSeed
			} else {
				item.CommandSource = TopologyCommandSourceProfileSeed
			}
		}

		if override, ok := overrideMap[spec.FieldKey]; ok {
			if override.Command != "" {
				item.Command = override.Command
			}
			if override.TimeoutSec > 0 {
				item.TimeoutSec = override.TimeoutSec
			}
			if override.Enabled != nil {
				item.Enabled = *override.Enabled
			}
			item.CommandSource = TopologyCommandSourceTaskOverride
		}

		if strings.TrimSpace(item.Command) == "" {
			item.Enabled = false
			if item.CommandSource == "" {
				item.CommandSource = TopologyCommandSourceDisabled
			}
		}

		resolved = append(resolved, item)
	}

	return &TopologyCommandResolution{
		ResolvedVendor: profile.Vendor,
		VendorSource:   vendorSource,
		ProfileVendor:  profile.Vendor,
		Commands:       resolved,
	}, nil
}

func (r *TopologyCommandResolver) resolveVendor(taskVendor string, device *models.DeviceAsset) (string, string) {
	if vendor := normalizeSupportedVendor(taskVendor); vendor != "" {
		return vendor, TopologyVendorSourceTask
	}
	if device != nil {
		if vendor := normalizeSupportedVendor(device.Vendor); vendor != "" {
			return vendor, TopologyVendorSourceInventory
		}
	}
	return defaultTopologyVendor, TopologyVendorSourceFallback
}

func buildTopologyCommandSeeds() map[string][]models.TopologyVendorFieldCommand {
	seeds := make(map[string][]models.TopologyVendorFieldCommand)
	for _, profile := range config.GetAllDeviceProfiles() {
		if profile == nil {
			continue
		}
		vendor := strings.ToLower(strings.TrimSpace(profile.Vendor))
		if vendor == "" {
			continue
		}
		items := make([]models.TopologyVendorFieldCommand, 0, len(profile.Commands))
		for _, spec := range topologyFieldCatalog {
			for _, cmd := range profile.Commands {
				if strings.TrimSpace(cmd.CommandKey) != spec.FieldKey {
					continue
				}
				items = append(items, models.TopologyVendorFieldCommand{
					Vendor:     vendor,
					FieldKey:   spec.FieldKey,
					Command:    strings.TrimSpace(cmd.Command),
					TimeoutSec: cmd.TimeoutSec,
					Enabled:    spec.DefaultEnabled && strings.TrimSpace(cmd.Command) != "",
					Notes:      "seeded_from_device_profile",
				})
				break
			}
		}
		seeds[vendor] = items
	}
	return seeds
}

func normalizeTopologyOverrides(overrides []models.TopologyTaskFieldOverride) map[string]models.TopologyTaskFieldOverride {
	result := make(map[string]models.TopologyTaskFieldOverride, len(overrides))
	for _, item := range overrides {
		fieldKey := strings.TrimSpace(item.FieldKey)
		if fieldKey == "" {
			continue
		}
		result[fieldKey] = models.TopologyTaskFieldOverride{
			FieldKey:   fieldKey,
			Command:    strings.TrimSpace(item.Command),
			TimeoutSec: item.TimeoutSec,
			Enabled:    item.Enabled,
		}
	}
	return result
}

func normalizeSupportedVendor(value string) string {
	vendor := strings.ToLower(strings.TrimSpace(value))
	if vendor == "" {
		return ""
	}
	if _, ok := config.GetDeviceProfileByVendor(vendor); ok {
		return vendor
	}
	return ""
}
