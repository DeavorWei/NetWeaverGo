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
}

var defaultTopologyFieldCatalog = []TopologyFieldSpec{
	{FieldKey: "version", Name: "系统版本", Phase: "collect", Required: true, ParserBinding: "version", DefaultEnabled: true, Description: "采集设备版本与系统镜像信息。"},
	{FieldKey: "sysname", Name: "设备名称", Phase: "collect", Required: true, ParserBinding: "sysname", DefaultEnabled: true, Description: "采集设备 sysname 或 hostname。"},
	{FieldKey: "interface_brief", Name: "接口概要", Phase: "collect", Required: true, ParserBinding: "interface_brief", DefaultEnabled: true, Description: "采集接口 up/down 与基础摘要。"},
	{FieldKey: "lldp_neighbor", Name: "LLDP 邻居", Phase: "collect", Required: true, ParserBinding: "lldp_neighbor", DefaultEnabled: true, Description: "采集 LLDP 邻居发现结果。"},
	{FieldKey: "arp_all", Name: "ARP 表", Phase: "collect", Required: true, ParserBinding: "arp_all", DefaultEnabled: true, Description: "采集 ARP 地址表。"},
	{FieldKey: "eth_trunk", Name: "聚合链路", Phase: "collect", Required: false, ParserBinding: "eth_trunk", DefaultEnabled: true, Description: "采集 Eth-Trunk/Port-Channel 聚合信息。"},
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
}

// TopologyVendorFieldCommand 厂商默认字段命令映射。
type TopologyVendorFieldCommand struct {
	ID         uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	Vendor     string    `json:"vendor" gorm:"not null;index:idx_topology_vendor_field,unique"`
	FieldKey   string    `json:"fieldKey" gorm:"not null;index:idx_topology_vendor_field,unique"`
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
