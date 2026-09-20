package models

import "time"

// TopologyFieldSpec 拓扑固定字段目录定义。
type TopologyFieldSpec struct {
	FieldKey       string `json:"fieldKey"`
	Name           string `json:"name"`
	Phase          string `json:"phase"`
	Required       bool   `json:"required"`
	ParserBinding  string `json:"parserBinding"`
	DefaultEnabled bool   `json:"defaultEnabled"`
	Description    string `json:"description"`
	Scene          string `json:"scene,omitempty"` // 适用场景，如 default, datacenter, campus, wan
}

var defaultTopologyFieldCatalog = []TopologyFieldSpec{
	{FieldKey: "version", Name: "系统版本", Phase: "collect", Required: true, ParserBinding: "version", DefaultEnabled: true, Description: "采集设备版本与系统镜像信息。", Scene: "default"},
	{FieldKey: "patch_info", Name: "补丁信息", Phase: "collect", Required: false, ParserBinding: "patch_info", DefaultEnabled: true, Description: "采集设备补丁版本信息（如 display patch-information）。", Scene: "default"},
	{FieldKey: "sysname", Name: "设备名称", Phase: "collect", Required: true, ParserBinding: "sysname", DefaultEnabled: true, Description: "采集设备 sysname 或 hostname。", Scene: "default"},
	{FieldKey: "interface_brief", Name: "接口概要", Phase: "collect", Required: true, ParserBinding: "interface_brief", DefaultEnabled: true, Description: "采集接口 up/down 与基础摘要。", Scene: "default"},
	{FieldKey: "lldp_neighbor", Name: "LLDP 邻居", Phase: "collect", Required: true, ParserBinding: "lldp_neighbor", DefaultEnabled: true, Description: "采集 LLDP 邻居发现结果。", Scene: "default"},
	{FieldKey: "cdp_neighbor", Name: "CDP 邻居", Phase: "collect", Required: false, ParserBinding: "cdp_neighbor", DefaultEnabled: true, Description: "采集 Cisco CDP 邻居发现结果。", Scene: "default"},
	{FieldKey: "arp_all", Name: "ARP 表", Phase: "collect", Required: true, ParserBinding: "arp_all", DefaultEnabled: true, Description: "采集 ARP 地址表。", Scene: "default"},
	{FieldKey: "mac_address", Name: "MAC 地址表", Phase: "collect", Required: false, ParserBinding: "mac_address", DefaultEnabled: true, Description: "采集 FDB/MAC 地址转发表，用于推断终端设备连接。", Scene: "default"},
	{FieldKey: "eth_trunk", Name: "聚合链路", Phase: "collect", Required: false, ParserBinding: "eth_trunk", DefaultEnabled: true, Description: "采集 Eth-Trunk/Port-Channel 聚合信息。", Scene: "default"},
	{FieldKey: "optical_power", Name: "光功率监控", Phase: "collect", Required: false, ParserBinding: "optical_power", DefaultEnabled: true, Description: "采集收发光功率与光衰指标。", Scene: "optical"},
}

// DefaultTopologyFieldCatalog 返回固定拓扑字段目录。
func DefaultTopologyFieldCatalog() []TopologyFieldSpec {
	result := make([]TopologyFieldSpec, len(defaultTopologyFieldCatalog))
	copy(result, defaultTopologyFieldCatalog)
	return result
}

// TopologyTaskFieldOverride 拓扑任务级字段覆盖配置。
type TopologyTaskFieldOverride struct {
	FieldKey   string `json:"fieldKey"`
	Command    string `json:"command"`
	TimeoutSec int    `json:"timeoutSec"`
	Enabled    *bool  `json:"enabled,omitempty"`
	Scene      string `json:"scene,omitempty"` // 任务指定场景
}

// TopologyVendorFieldCommand 厂商默认字段命令映射。
type TopologyVendorFieldCommand struct {
	ID         uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	Vendor     string    `json:"vendor" gorm:"not null;index:idx_topology_vendor_field_scene,unique"`
	FieldKey   string    `json:"fieldKey" gorm:"not null;index:idx_topology_vendor_field_scene,unique"`
	Scene      string    `json:"scene" gorm:"size:64;default:'default';not null;index:idx_topology_vendor_field_scene,unique"`
	Command    string    `json:"command"`
	TimeoutSec int       `json:"timeoutSec"`
	Enabled    bool      `json:"enabled"`
	Notes      string    `json:"notes"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// TableName 指定表名。
func (TopologyVendorFieldCommand) TableName() string {
	return "topology_vendor_field_commands"
}
