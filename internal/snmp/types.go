// Package snmp 提供 SNMP 核心业务功能
// types.go 定义核心类型和事件通知接口
package snmp

import (
	"context"
	"sync"
	"time"

	"github.com/NetWeaverGo/core/internal/models"
)

// ============================================================================
// 事件通知接口（解耦 Wails 依赖）
// ============================================================================

// EventNotifier 事件通知接口
// SNMP 核心模块通过此接口推送事件，不直接依赖 Wails 框架
// 实现：internal/ui/snmp_event_notifier.go（Wails Events 推送）
type EventNotifier interface {
	// Trap 事件
	NotifyNewTrap(trap *models.SNMPTrapRecord)
	NotifyTrapStats(stats *TrapStats)
	NotifyListenerStatus(stats *ListenerStats) // P2-13: 发送完整状态对象
	NotifyTrapReceived(trap TrapEvent)

	// 轮询事件
	NotifyPollResult(targetID uint, results []models.SNMPPollingResult)
	NotifyPollError(targetID uint, err error)
	NotifySchedulerStatus(running bool)
	NotifyPollingResult(result PollingResultEvent)

	// MIB 事件
	NotifyMIBImported(module *models.MIBModule)
	NotifyMIBDeleted(moduleID uint)
	NotifyMIBImportProgress(progress MIBImportProgress)

	// 通用错误通知（A-18: rebuildTaskSem 超时等非特定上下文的错误）
	NotifyError(err error)
}

// ============================================================================
// 事件类型定义（用于 Wails 实时推送）
// ============================================================================

// MIBImportProgress MIB 导入进度事件
type MIBImportProgress struct {
	FileName   string  `json:"fileName"`            // 正在导入的文件名
	ModuleName string  `json:"moduleName"`          // 模块名
	Phase      string  `json:"phase"`               // 当前阶段：parsing/saving/caching/completed/error
	Progress   float64 `json:"progress"`            // 进度百分比 0-100
	NodesDone  int     `json:"nodesDone"`           // 已处理节点数
	NodesTotal int     `json:"nodesTotal"`          // 总节点数（预估）
	Error      string  `json:"error"`               // 错误信息（仅 error 阶段）
	Message    string  `json:"message,omitempty"`   // 进度描述信息

	// 批量导入扩展字段
	BatchID        string `json:"batchId,omitempty"`        // 批量导入批次 ID
	TotalFiles     int    `json:"totalFiles,omitempty"`     // 批量导入总文件数
	ProcessedFiles int    `json:"processedFiles,omitempty"` // 已处理文件数
	CurrentPhase   string `json:"currentPhase,omitempty"`   // 批量导入当前阶段：copy/parse/save/cache/done
}

// TrapEvent Trap 接收事件（轻量级，用于实时推送）
type TrapEvent struct {
	SourceIP   string `json:"sourceIP"`
	SourcePort int    `json:"sourcePort"`
	TrapOID    string `json:"trapOID"`
	TrapName   string `json:"trapName"`
	Severity   string `json:"severity"`
	Community  string `json:"community"`
	Version    string `json:"version"`
	ReceivedAt int64  `json:"receivedAt"` // Unix 毫秒时间戳
}

// PollingResultEvent 轮询结果事件（轻量级，用于实时推送）
type PollingResultEvent struct {
	TargetID  uint   `json:"targetId"`
	TargetIP  string `json:"targetIP"`
	Status    string `json:"status"`    // success/error/timeout
	Error     string `json:"error"`     // 错误信息
	PollTime  int64  `json:"pollTime"`  // Unix 毫秒时间戳
	OIDCount  int    `json:"oidCount"`  // 采集的 OID 数量
	BatchID   string `json:"batchId"`   // 批次 ID
}

// ============================================================================
// Trap 相关类型
// ============================================================================

// TrapStats Trap 告警统计信息
type TrapStats struct {
	TotalCount        int64 `json:"totalCount"`
	Unacknowledged    int64 `json:"unacknowledged"`
	CriticalCount     int64 `json:"criticalCount"`
	MajorCount        int64 `json:"majorCount"`
	MinorCount        int64 `json:"minorCount"`
	InfoCount         int64 `json:"infoCount"`
	TodayCount        int64 `json:"todayCount"`
	LastHourCount     int64 `json:"lastHourCount"`
}

// TrapQueryOptions Trap 查询选项
type TrapQueryOptions struct {
	SearchQuery   string   `json:"searchQuery"`   // 搜索关键词（OID/IP/名称）
	Severity      string   `json:"severity"`      // 严重级别过滤
	SourceIP      string   `json:"sourceIP"`      // 来源 IP 过滤
	StartTime     string   `json:"startTime"`     // 开始时间
	EndTime       string   `json:"endTime"`       // 结束时间
	Acknowledged  *bool    `json:"acknowledged"`  // 确认状态过滤
	TrapOIDPrefix string   `json:"trapOIDPrefix"` // OID 前缀过滤
	Page          int      `json:"page"`          // 页码（1-based）
	PageSize      int      `json:"pageSize"`      // 每页条数
	SortBy        string   `json:"sortBy"`        // 排序字段
	SortOrder     string   `json:"sortOrder"`     // 排序方向（asc/desc）
}

// TrapQueryResult Trap 查询结果
type TrapQueryResult struct {
	Data       []models.SNMPTrapRecord `json:"data"`
	Total      int64                   `json:"total"`
	Page       int                     `json:"page"`
	PageSize   int                     `json:"pageSize"`
	TotalPages int                     `json:"totalPages"`
}

// ============================================================================
// 轮询相关类型
// ============================================================================

// PollingStatus 轮询目标状态
type PollingStatus struct {
	TargetID      uint   `json:"targetId"`
	TargetIP      string `json:"targetIP"`
	DisplayName   string `json:"displayName"`
	Enabled       bool   `json:"enabled"`
	LastPollAt    string `json:"lastPollAt"`
	LastPollStatus string `json:"lastPollStatus"`
	NextPollAt    string `json:"nextPollAt"`
	PollInterval  int    `json:"pollInterval"`
}

// PollingResultQueryOpts 轮询结果查询选项
type PollingResultQueryOpts struct {
	TargetID  uint   `json:"targetId"`
	TargetIP  string `json:"targetIP"`
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
	OID       string `json:"oid"`
	Page      int    `json:"page"`
	PageSize  int    `json:"pageSize"`
}

// PollingResultQueryResult 轮询结果查询结果
type PollingResultQueryResult struct {
	Data       []models.SNMPPollingResult `json:"data"`
	Total      int64                      `json:"total"`
	Page       int                        `json:"page"`
	PageSize   int                        `json:"pageSize"`
	TotalPages int                        `json:"totalPages"`
}

// SNMPResult SNMP 操作结果
type SNMPResult struct {
	OID       string `json:"oid"`
	OIDName   string `json:"oidName"`
	Value     string `json:"value"`
	ValueType string `json:"valueType"`
	Error     string `json:"error,omitempty"`
}

// ============================================================================
// MIB 相关类型
// ============================================================================

// MIBTreeNode MIB 树节点（用于前端树形展示）
type MIBTreeNode struct {
	ID          uint         `json:"id"`
	OID         string       `json:"oid"`
	Name        string       `json:"name"`
	NodeType    string       `json:"nodeType"`
	Syntax      string       `json:"syntax"`
	Access      string       `json:"access"`
	Status      string       `json:"status"`
	Description string       `json:"description"`
	Children    []MIBTreeNode `json:"children,omitempty"`
	HasChildren bool         `json:"hasChildren"`
}

// MIBImportResult MIB 导入结果
type MIBImportResult struct {
	Module      *models.MIBModule `json:"module"`
	Nodes       []models.MIBNode  `json:"-"` // 内部使用，不序列化到 JSON
	NodeCount   int               `json:"nodeCount"`
	ErrorCount  int               `json:"errorCount"`
	Errors      []MIBParseError   `json:"errors,omitempty"`
}

// MIBParseError MIB 解析错误详情
type MIBParseError struct {
	LineNumber int    `json:"lineNumber,omitempty"`
	NodeName   string `json:"nodeName,omitempty"`
	Message    string `json:"message"`
}

// NodeQueryOptions MIB 节点查询选项
type NodeQueryOptions struct {
	ModuleID  *uint  `json:"moduleId"`
	ParentOID string `json:"parentOid"`
	NodeType  string `json:"nodeType"`
	Search    string `json:"search"`
	Page      int    `json:"page"`
	PageSize  int    `json:"pageSize"`
}

// ============================================================================
// OID 翻译相关类型
// ============================================================================

// OIDTranslation OID 翻译结果
type OIDTranslation struct {
	OID         string `json:"oid"`
	Name        string `json:"name"`
	Module      string `json:"module"`
	Description string `json:"description"`
	Found       bool   `json:"found"`
}

// ============================================================================
// 凭据加密相关类型
// ============================================================================

// CredentialCrypto SNMP 凭据加密管理器
// 使用 AES-256-GCM 对敏感字段加密
type CredentialCrypto struct {
	key []byte      // 32 bytes AES-256 key
	mu  sync.RWMutex // 并发保护
}

// ============================================================================
// 上下文支持
// ============================================================================

// CancellableOperation 可取消的操作接口
type CancellableOperation interface {
	Start(ctx context.Context) error
	Stop() error
	IsRunning() bool
}

// ============================================================================
// 批量导入相关类型
// ============================================================================

// MIBBatchImportResult 批量导入结果
type MIBBatchImportResult struct {
	TotalFiles    int               `json:"totalFiles"`
	SuccessCount  int               `json:"successCount"`
	FailedCount   int               `json:"failedCount"`
	SkippedCount  int               `json:"skippedCount"`
	Results       []FileImportResult `json:"results"`
	Errors        []FileImportError  `json:"errors"`
	TotalDuration int64             `json:"totalDuration"` // 毫秒数
}

// FileImportResult 单文件导入结果
type FileImportResult struct {
	FileName   string `json:"fileName"`
	ModuleName string `json:"moduleName"`
	NodeCount  int    `json:"nodeCount"`
	Duration   int64  `json:"duration"` // 毫秒数
	Status     string `json:"status"`   // "success", "failed", "skipped"
}

// FileImportError 文件导入错误
type FileImportError struct {
	FileName  string `json:"fileName"`
	Error     string `json:"error"`
	ErrorType string `json:"errorType"` // "parse", "dependency", "database", "unknown"
}

// MIBBatchImportOptions 批量导入选项
type MIBBatchImportOptions struct {
	Concurrency       int      `json:"concurrency"`
	SkipErrors        bool     `json:"skipErrors"`
	OverwriteExisting bool     `json:"overwriteExisting"`
	DependencyDirs    []string `json:"dependencyDirs"`
	FolderID          *uint    `json:"folderId,omitempty"` // 目标文件夹 ID
}

// ============================================================================
// PollDispatcher 并发控制相关类型
// ============================================================================

// DispatcherConfig 分发器配置
type DispatcherConfig struct {
	// MaxConcurrentDevices 最大并发设备数
	// 同一时刻最多有多少台设备在进行 SNMP 轮询
	// 默认: 10
	MaxConcurrentDevices int `json:"maxConcurrentDevices"`

	// MaxOpsPerDevice 单设备最大并发操作数
	// 同一台设备同时允许多少个 SNMP 操作（GET/WALK/BULK）
	// 对于不支持并发的设备应设为 1
	// 默认: 1
	MaxOpsPerDevice int `json:"maxOpsPerDevice"`

	// SkipIfBusy 如果设备繁忙是否跳过
	// true: 设备正在轮询中时跳过本次触发（适用于 Cron 防重叠）
	// false: 排队等待（适用于手动触发和批量轮询）
	// 默认: true（Cron 场景）
	SkipIfBusy bool `json:"skipIfBusy"`

	// QueueTimeout 排队超时
	// 等待信号量的最大时间，超时后放弃
	// 默认: 30s
	QueueTimeout time.Duration `json:"queueTimeout"`
}

// DefaultDispatcherConfig 默认分发器配置
var DefaultDispatcherConfig = DispatcherConfig{
	MaxConcurrentDevices: 10,
	MaxOpsPerDevice:      1,
	SkipIfBusy:           true,
	QueueTimeout:         30 * time.Second,
}

// PollResult 轮询结果封装
type PollResult struct {
	Target    *PollTarget                 // 轮询目标
	Results   []*models.SNMPPollingResult // 轮询数据结果
	Error     error                       // 错误信息
	Latency   time.Duration               // 执行耗时
	Skipped   bool                        // 因设备繁忙而跳过
	Cancelled bool                        // 因 Context 取消或排队超时
	Queued    bool                        // 是否在队列中等待过
}

// DispatcherStatus 分发器运行状态
type DispatcherStatus struct {
	ActiveDevices   int                         `json:"activeDevices"`   // 当前活跃设备数
	MaxDevices      int                         `json:"maxDevices"`      // 最大并发设备数
	MaxOpsPerDevice int                         `json:"maxOpsPerDevice"` // 单设备最大并发操作数
	WaitingTasks    int                         `json:"waitingTasks"`    // 排队等待信号量的任务数
	SkippedTasks    int64                       `json:"skippedTasks"`    // 累计因繁忙跳过的任务数
	DeviceStatus    map[string]*DeviceSemStatus `json:"deviceStatus"`    // 各设备信号量状态
}

// DeviceSemStatus 设备信号量状态
type DeviceSemStatus struct {
	ActiveOps  int `json:"activeOps"`  // 当前活跃操作数
	MaxOps     int `json:"maxOps"`     // 最大并发操作数
	WaitingOps int `json:"waitingOps"` // 等待中的操作数（暂为 0，后续可扩展）
}

// DispatchOption 函数选项模式
type DispatchOption func(*dispatchOptions)

type dispatchOptions struct {
	skipIfBusy bool
}

// WithSkipIfBusy 设置是否在繁忙时跳过
func WithSkipIfBusy(skip bool) DispatchOption {
	return func(o *dispatchOptions) {
		o.skipIfBusy = skip
	}
}