package ui

import "github.com/NetWeaverGo/core/internal/models"

// TopologyResolvedCommandView 拓扑命令解析后的前端视图模型。
type TopologyResolvedCommandView struct {
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

// TopologyCommandResolutionView 拓扑命令解析结果视图模型。
type TopologyCommandResolutionView struct {
	ResolvedVendor string                        `json:"resolvedVendor"`
	VendorSource   string                        `json:"vendorSource"`
	ProfileVendor  string                        `json:"profileVendor"`
	Commands       []TopologyResolvedCommandView `json:"commands"`
}

// TopologyPreviewDeviceView 单台设备的拓扑命令预览。
type TopologyPreviewDeviceView struct {
	DeviceID        uint                          `json:"deviceId"`
	DeviceIP        string                        `json:"deviceIP"`
	InventoryVendor string                        `json:"inventoryVendor"`
	Resolution      TopologyCommandResolutionView `json:"resolution"`
}

// TopologyCommandPreviewView 拓扑命令预览结果。
type TopologyCommandPreviewView struct {
	SupportedVendors  []string                      `json:"supportedVendors"`
	FieldCatalog      []models.TopologyFieldSpec    `json:"fieldCatalog"`
	DefaultResolution TopologyCommandResolutionView `json:"defaultResolution"`
	Devices           []TopologyPreviewDeviceView   `json:"devices"`
}
