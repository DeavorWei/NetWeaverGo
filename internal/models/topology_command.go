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
