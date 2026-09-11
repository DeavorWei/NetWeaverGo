package ceas

// Node 硬件树节点（自包含模型）
type Node struct {
	ID           string            `json:"id"`           // 层级 ID: 0_1_2
	ParentID     string            `json:"parentId"`     // 父层级 ID: 0_1
	Level        int               `json:"level"`        // 1: 框, 2: 槽位/框架, 3: 单板/端口
	Type         string            `json:"type"`         // frame / slot / card / port / power / fanframe / subcard ...
	Name         string            `json:"name"`         // Slot_1 / BackPlane_1 / Fan_1 ...
	Path         string            `json:"path"`         // 3段式 ID: 1/0, 1/2, 1/2/3
	Slot         string            `json:"slot"`         // 归属或继承的主槽位号
	Item         string            `json:"item"`         // BOM 编码
	BarCode      string            `json:"barCode"`      // 条形码 / 电子标签序列号
	Description  string            `json:"description"`  // 物料描述
	Manufactured string            `json:"manufactured"` // 生产日期
	VendorName   string            `json:"vendorName"`   // 厂商名称
	BoardType    string            `json:"boardType"`    // 单板类型
	Attrs        map[string]string `json:"attrs"`        // 扩展属性键值对
	Children     []*Node           `json:"children"`     // 子节点树形引用
}

// HardwareTree 完整硬件树拓扑结构
type HardwareTree struct {
	DeviceIP   string  `json:"deviceIp"`
	ChassisESN string  `json:"chassisEsn"` // 主机 ESN
	Roots      []*Node `json:"roots"`      // 根机框列表
	AllNodes   []*Node `json:"allNodes"`   // 扁平节点全量列表
}

// BOMAlertItem BOM 批次预警命中明细
type BOMAlertItem struct {
	DeviceIP    string `json:"deviceIp"`
	Slot        string `json:"slot"`
	Path        string `json:"path"`
	NodeType    string `json:"nodeType"`
	NodeName    string `json:"nodeName"`
	Item        string `json:"item"`
	BarCode     string `json:"barCode"`
	Description string `json:"description"`
	Severity    string `json:"severity"` // warning, danger, critical
	BatchNo     string `json:"batchNo"`  // 批次号/预警通知单号
}

// CEASTaskConfig 硬件清单采集任务执行配置
type CEASTaskConfig struct {
	DeviceIPs   []string `json:"deviceIps"`
	Concurrency int      `json:"concurrency"`
	TimeoutSec  int      `json:"timeoutSec"`
}

// HardwareTreeVO 前端视图对象
type HardwareTreeVO struct {
	DeviceIP   string    `json:"deviceIp"`
	ChassisESN string    `json:"chassisEsn"`
	TotalNodes int       `json:"totalNodes"`
	Roots      []*NodeVO `json:"roots"`
}

// NodeVO 前端层级节点视图
type NodeVO struct {
	ID           string            `json:"id"`
	ParentID     string            `json:"parentId"`
	Level        int               `json:"level"`
	Type         string            `json:"type"`
	Name         string            `json:"name"`
	Path         string            `json:"path"`
	Slot         string            `json:"slot"`
	Item         string            `json:"item"`
	BarCode      string            `json:"barCode"`
	Description  string            `json:"description"`
	Manufactured string            `json:"manufactured"`
	VendorName   string            `json:"vendorName"`
	BoardType    string            `json:"boardType"`
	Attrs        map[string]string `json:"attrs,omitempty"`
	Children     []*NodeVO         `json:"children,omitempty"`
}

// BOMAlertVO 前端预警视图对象
type BOMAlertVO struct {
	DeviceIP    string `json:"deviceIp"`
	Slot        string `json:"slot"`
	Path        string `json:"path"`
	NodeType    string `json:"nodeType"`
	NodeName    string `json:"nodeName"`
	Item        string `json:"item"`
	BarCode     string `json:"barCode"`
	Description string `json:"description"`
	Severity    string `json:"severity"`
	BatchNo     string `json:"batchNo"`
}
