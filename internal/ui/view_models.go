package ui

import "github.com/NetWeaverGo/core/internal/config"

// ================== 视图模型定义 ==================
// 本文件定义与前端直接对接的视图模型结构体
// 前端无需任何计算，直接绑定渲染即可

// ExecutionSnapshot 任务执行快照 - 前端直接绑定渲染
// 替代原有的零散事件推送，提供完整的状态树
type ExecutionSnapshot struct {
	TaskName      string            `json:"taskName"`
	TotalDevices  int               `json:"totalDevices"`
	FinishedCount int               `json:"finishedCount"`
	Progress      int               `json:"progress"` // 0-100 百分比
	IsRunning     bool              `json:"isRunning"`
	StartTime     string            `json:"startTime"` // ISO8601 格式
	Devices       []DeviceViewState `json:"devices"`
	Summary       *ExecutionSummary `json:"summary,omitempty"`
}

// DeviceViewState 单设备视图状态
// 包含设备执行的所有状态信息，已处理完毕可直接渲染
type DeviceViewState struct {
	IP        string   `json:"ip"`
	Status    string   `json:"status"`    // running/success/error/aborted/waiting
	Logs      []string `json:"logs"`      // 已截断的日志数组
	LogCount  int      `json:"logCount"`  // 原始日志总条数
	Truncated bool     `json:"truncated"` // 是否已截断标记
	CmdIndex  int      `json:"cmdIndex"`  // 当前执行命令索引 (0-based)
	TotalCmd  int      `json:"totalCmd"`  // 总命令数
	Message   string   `json:"message"`   // 当前状态消息/错误信息
}

// ExecutionSummary 执行结果汇总
// 任务结束后提供的统计信息
type ExecutionSummary struct {
	TotalDevices  int            `json:"totalDevices"`
	SuccessCount  int            `json:"successCount"`
	ErrorCount    int            `json:"errorCount"`
	AbortedCount  int            `json:"abortedCount"`
	WarningCount  int            `json:"warningCount"`
	Duration      string         `json:"duration"`      // 执行时长 (可读格式)
	DurationMs    int64          `json:"durationMs"`    // 执行时长 (毫秒)
	ReportPath    string         `json:"reportPath"`    // 生成的报告文件路径
	FailedDevices []FailedDevice `json:"failedDevices"` // 失败设备列表
}

// FailedDevice 失败设备信息
type FailedDevice struct {
	IP       string `json:"ip"`
	Reason   string `json:"reason"`   // 失败原因
	Status   string `json:"status"`   // 最终状态
	CmdIndex int    `json:"cmdIndex"` // 失败时的命令索引
}

// ================== 查询相关视图模型 ==================

// QueryOptions 通用查询条件
// 用于设备列表、任务列表等分页查询
type QueryOptions struct {
	SearchQuery string `json:"searchQuery"` // 搜索关键词
	FilterField string `json:"filterField"` // 搜索字段 (如 group, ip, tag)
	FilterValue string `json:"filterValue"` // 过滤值 (如 status=running)
	Page        int    `json:"page"`        // 页码 (1-based)
	PageSize    int    `json:"pageSize"`    // 每页条数
	SortBy      string `json:"sortBy"`      // 排序字段
	SortOrder   string `json:"sortOrder"`   // 排序方向: asc/desc
}

// QueryResult 分页查询结果
type QueryResult struct {
	Data       interface{} `json:"data"`       // 数据列表
	Total      int         `json:"total"`      // 总记录数
	Page       int         `json:"page"`       // 当前页码
	PageSize   int         `json:"pageSize"`   // 每页条数
	TotalPages int         `json:"totalPages"` // 总页数
}

// ================== ConfigForge 视图模型 ==================

// ForgeBuildRequest 配置构建请求
type ForgeBuildRequest struct {
	Template  string          `json:"template"`  // 配置模板文本
	Variables []ForgeVariable `json:"variables"` // 变量列表
}

// ForgeVariable 变量定义
type ForgeVariable struct {
	Name        string `json:"name"`        // 变量名 (如 [A], [B])
	ValueString string `json:"valueString"` // 变量值 (逗号或换行分隔)
}

// ForgeBuildResult 配置构建结果
type ForgeBuildResult struct {
	Blocks         []string       `json:"blocks"`         // 生成的配置块
	Total          int            `json:"total"`          // 总块数
	Warnings       []string       `json:"warnings"`       // 警告信息
	VariableCounts map[string]int `json:"variableCounts"` // 各变量的值数量
	IsBindingMode  bool           `json:"isBindingMode"`  // 是否为 IP 绑定模式
	InvalidIPs     []string       `json:"invalidIPs"`     // 无效 IP 列表
}

// IPValidationResult IP 验证结果
type IPValidationResult struct {
	Valid      bool   `json:"valid"`      // 是否有效
	IP         string `json:"ip"`         // 原始输入
	Error      string `json:"error"`      // 错误信息
	IsRange    bool   `json:"isRange"`    // 是否为范围格式
	RangeStart string `json:"rangeStart"` // 范围起始 (当 IsRange=true)
	RangeEnd   string `json:"rangeEnd"`   // 范围结束 (当 IsRange=true)
	RangeCount int    `json:"rangeCount"` // 范围数量 (当 IsRange=true)
}

// ================== 日志配置常量 ==================

// 日志相关配置，后端统一管理
const (
	MaxLogsPerDevice    = 500  // 每设备最大日志条数
	MaxLogLength        = 2000 // 单条日志最大字符数
	LogWarningThreshold = 400  // 日志条数警告阈值
)

// ================== 辅助方法 ==================

// GetDefaultQueryOptions 获取默认查询选项
func GetDefaultQueryOptions() QueryOptions {
	return QueryOptions{
		Page:      1,
		PageSize:  10,
		SortBy:    "updatedAt",
		SortOrder: "desc",
	}
}

// NewExecutionSnapshot 创建新的执行快照
func NewExecutionSnapshot(taskName string, totalDevices int) *ExecutionSnapshot {
	return &ExecutionSnapshot{
		TaskName:      taskName,
		TotalDevices:  totalDevices,
		FinishedCount: 0,
		Progress:      0,
		IsRunning:     true,
		Devices:       make([]DeviceViewState, 0, totalDevices),
	}
}

// NewDeviceViewState 创建新的设备视图状态
func NewDeviceViewState(ip string, totalCmd int) *DeviceViewState {
	return &DeviceViewState{
		IP:        ip,
		Status:    "waiting",
		Logs:      make([]string, 0, 100),
		LogCount:  0,
		Truncated: false,
		CmdIndex:  0,
		TotalCmd:  totalCmd,
		Message:   "",
	}
}

// ================== 任务详情视图模型 ==================

// TaskGroupDetailViewModel 任务详情聚合模型
// 为执行大屏详情/编辑弹窗提供后端解析后的结构化数据
type TaskGroupDetailViewModel struct {
	Task               config.TaskGroup               `json:"task"`
	ItemCount          int                            `json:"itemCount"`
	CanEdit            bool                           `json:"canEdit"`
	EditDisabledReason string                         `json:"editDisabledReason"`
	Items              []TaskGroupItemDetailViewModel `json:"items"`
	MissingDevices     []uint                         `json:"missingDevices"`
	MissingCommandIDs  []string                       `json:"missingCommandIds"`
}

// TaskGroupItemDetailViewModel 单个任务项详情
type TaskGroupItemDetailViewModel struct {
	Index       int                  `json:"index"`
	Mode        string               `json:"mode"`
	DeviceCount int                  `json:"deviceCount"`
	Devices     []TaskDeviceOverview `json:"devices"`
	CommandInfo *TaskCommandOverview `json:"commandInfo,omitempty"`
	Commands    []string             `json:"commands"`
}

// TaskDeviceOverview 任务关联设备概览
type TaskDeviceOverview struct {
	ID      uint     `json:"id"`
	IP      string   `json:"ip"`
	Group   string   `json:"group"`
	Tags    []string `json:"tags"`
	Missing bool     `json:"missing"`
}

// TaskCommandOverview 任务命令信息概览
type TaskCommandOverview struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	Commands    []string `json:"commands"`
	Missing     bool     `json:"missing"`
}
