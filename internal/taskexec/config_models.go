package taskexec

// ==================== 普通任务配置模型 ====================

// NormalTaskConfig 普通任务配置
type NormalTaskConfig struct {
	// Mode A / Mode B
	Mode string `json:"mode"` // "group" = Mode A, "binding" = Mode B

	// 设备选择
	DeviceIDs []uint `json:"deviceIDs"` // Mode A: 选择的设备ID列表
	DeviceIPs []string `json:"deviceIPs"` // Mode A: 解析后的设备IP列表（运行时使用）

	// 命令来源 (Mode A)
	CommandGroupID string   `json:"commandGroupID"` // 命令组ID
	Commands       []string `json:"commands"`       // 直接指定的命令列表

	// 任务项 (Mode B)
	Items []NormalTaskItem `json:"items"` // 每个任务项绑定设备和命令

	// 执行选项
	Concurrency  int  `json:"concurrency"`  // 并发数
	TimeoutSec   int  `json:"timeoutSec"`   // 超时(秒)
	EnableRawLog bool `json:"enableRawLog"` // 是否启用raw日志
}

// NormalTaskItem 普通任务项 (Mode B)
type NormalTaskItem struct {
	DeviceIDs      []uint   `json:"deviceIDs"`      // 设备ID列表
	DeviceIPs      []string `json:"deviceIPs"`      // 解析后的设备IP列表（运行时使用）
	CommandGroupID string   `json:"commandGroupID"` // 命令组ID
	Commands       []string `json:"commands"`       // 直接命令列表
}

// ==================== 拓扑任务配置模型 ====================

// TopologyTaskConfig 拓扑任务配置
type TopologyTaskConfig struct {
	// 设备选择
	DeviceIDs  []uint   `json:"deviceIDs"`  // 指定设备ID列表
	DeviceIPs  []string `json:"deviceIPs"`  // 解析后的设备IP列表（运行时使用）
	GroupNames []string `json:"groupNames"` // 设备组名称列表

	// 厂商配置
	Vendor string `json:"vendor"` // 厂商: huawei / h3c / cisco / auto

	// 采集选项
	MaxWorkers int `json:"maxWorkers"` // 并发数
	TimeoutSec int `json:"timeoutSec"` // 单设备超时(秒)

	// 自动构建选项
	AutoBuildTopology bool `json:"autoBuildTopology"` // 采集完成后自动构建拓扑

	// 日志选项
	EnableRawLog bool `json:"enableRawLog"` // 是否启用raw日志
}

// CompileOptions 编译选项
type CompileOptions struct {
	DefaultConcurrency      int
	DefaultTimeoutSec       int
	DefaultDiscoveryWorkers int
}

// DefaultCompileOptions 默认编译选项
func DefaultCompileOptions() *CompileOptions {
	return &CompileOptions{
		DefaultConcurrency:      10,
		DefaultTimeoutSec:       300,
		DefaultDiscoveryWorkers: 5,
	}
}
