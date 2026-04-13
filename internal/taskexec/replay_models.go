package taskexec

import (
	"time"
)

// =============================================================================
// 离线重放模式数据模型
// 支持从历史Raw文件重新解析构建拓扑，无需连接设备
// =============================================================================

// TopologyReplayRecord 重放记录
// 记录每次离线重放操作的元数据和结果
type TopologyReplayRecord struct {
	ID             uint       `gorm:"primaryKey;autoIncrement" json:"id"`
	OriginalRunID  string     `gorm:"index;not null" json:"originalRunId"`     // 原始运行ID
	ReplayRunID    string     `gorm:"uniqueIndex;not null" json:"replayRunId"` // 重放运行ID（虚拟运行ID）
	Status         string     `json:"status"`                                  // pending, running, completed, failed, cancelled
	TriggerSource  string     `json:"triggerSource"`                           // manual, auto
	ParserVersion  string     `json:"parserVersion"`                           // 解析器版本标识
	BuilderVersion string     `json:"builderVersion"`                          // 构建器版本标识
	StartedAt      *time.Time `json:"startedAt"`
	FinishedAt     *time.Time `json:"finishedAt"`
	ErrorMessage   string     `json:"errorMessage"`
	// 统计信息（JSON序列化）
	Statistics string    `gorm:"type:text" json:"statistics"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

func (TopologyReplayRecord) TableName() string {
	return "topology_replay_records"
}

// ReplayStatus 重放状态常量
type ReplayStatus string

const (
	ReplayStatusPending   ReplayStatus = "pending"
	ReplayStatusRunning   ReplayStatus = "running"
	ReplayStatusCompleted ReplayStatus = "completed"
	ReplayStatusFailed    ReplayStatus = "failed"
	ReplayStatusCancelled ReplayStatus = "cancelled"
)

// ReplayOptions 重放选项
type ReplayOptions struct {
	// ClearExisting 是否清除现有解析结果后重新构建
	ClearExisting bool `json:"clearExisting"`
	// ParserVersion 指定解析器版本（空则使用当前版本）
	ParserVersion string `json:"parserVersion"`
	// DeviceIPs 指定要重放的设备列表（空则重放所有设备）
	DeviceIPs []string `json:"deviceIps"`
	// SkipBuild 是否跳过构建阶段（仅执行解析）
	SkipBuild bool `json:"skipBuild"`
}

// RawFileInfo Raw文件信息
type RawFileInfo struct {
	DeviceIP   string `json:"deviceIp"`
	CommandKey string `json:"commandKey"`
	FilePath   string `json:"filePath"`
	FileSize   int64  `json:"fileSize"`
}

// ReplayProgress 重放进度
type ReplayProgress struct {
	Phase         string  `json:"phase"` // scan, parse, build, finalize
	CurrentDevice string  `json:"currentDevice"`
	TotalDevices  int     `json:"totalDevices"`
	Processed     int     `json:"processed"`
	Percent       float64 `json:"percent"`
	Message       string  `json:"message"`
}

// ReplayStatistics 重放统计信息
type ReplayStatistics struct {
	// 扫描阶段
	TotalRawFiles    int `json:"totalRawFiles"`
	TotalDevices     int `json:"totalDevices"`
	TotalCommandKeys int `json:"totalCommandKeys"`

	// 解析阶段
	ParsedDevices  int `json:"parsedDevices"`
	ParsedCommands int `json:"parsedCommands"`
	FailedCommands int `json:"failedCommands"`
	LLDPCount      int `json:"lldpCount"`
	FDBCount       int `json:"fdbCount"`
	ARPCount       int `json:"arpCount"`
	InterfaceCount int `json:"interfaceCount"`

	// 构建阶段
	TotalCandidates int `json:"totalCandidates"`
	RetainedEdges   int `json:"retainedEdges"`
	RejectedEdges   int `json:"rejectedEdges"`
	ConflictEdges   int `json:"conflictEdges"`

	// 耗时统计
	ScanDurationMs  int64 `json:"scanDurationMs"`
	ParseDurationMs int64 `json:"parseDurationMs"`
	BuildDurationMs int64 `json:"buildDurationMs"`
	TotalDurationMs int64 `json:"totalDurationMs"`
}

// ReplayResult 重放结果
type ReplayResult struct {
	ReplayRunID string           `json:"replayRunId"`
	Status      string           `json:"status"`
	Statistics  ReplayStatistics `json:"statistics"`
	Errors      []string         `json:"errors"`
	StartedAt   time.Time        `json:"startedAt"`
	FinishedAt  time.Time        `json:"finishedAt"`
}

// ReplayableRunInfo 可重放的运行信息
type ReplayableRunInfo struct {
	RunID        string    `json:"runId"`
	TaskName     string    `json:"taskName"`
	Status       string    `json:"status"`
	RunKind      string    `json:"runKind"`
	DeviceCount  int       `json:"deviceCount"`
	RawFileCount int       `json:"rawFileCount"`
	CreatedAt    time.Time `json:"createdAt"`
	HasRawFiles  bool      `json:"hasRawFiles"`
}

// ParseResultDiff 解析结果差异
type ParseResultDiff struct {
	RunID1        string               `json:"runId1"`
	RunID2        string               `json:"runId2"`
	LLDPDiff      LLDPDiffSummary      `json:"lldpDiff"`
	FDBDiff       FDBDiffSummary       `json:"fdbDiff"`
	ARPDiff       ARPDiffSummary       `json:"arpDiff"`
	InterfaceDiff InterfaceDiffSummary `json:"interfaceDiff"`
}

// LLDPDiffSummary LLDP差异摘要
type LLDPDiffSummary struct {
	OnlyIn1   int `json:"onlyIn1"`
	OnlyIn2   int `json:"onlyIn2"`
	Modified  int `json:"modified"`
	Unchanged int `json:"unchanged"`
}

// FDBDiffSummary FDB差异摘要
type FDBDiffSummary struct {
	OnlyIn1   int `json:"onlyIn1"`
	OnlyIn2   int `json:"onlyIn2"`
	Modified  int `json:"modified"`
	Unchanged int `json:"unchanged"`
}

// ARPDiffSummary ARP差异摘要
type ARPDiffSummary struct {
	OnlyIn1   int `json:"onlyIn1"`
	OnlyIn2   int `json:"onlyIn2"`
	Modified  int `json:"modified"`
	Unchanged int `json:"unchanged"`
}

// InterfaceDiffSummary 接口差异摘要
type InterfaceDiffSummary struct {
	OnlyIn1   int `json:"onlyIn1"`
	OnlyIn2   int `json:"onlyIn2"`
	Modified  int `json:"modified"`
	Unchanged int `json:"unchanged"`
}

// TopologyEdgeDiff 拓扑边差异
type TopologyEdgeDiff struct {
	RunID1         string         `json:"runId1"`
	RunID2         string         `json:"runId2"`
	AddedEdges     []EdgeDiffItem `json:"addedEdges"`
	RemovedEdges   []EdgeDiffItem `json:"removedEdges"`
	ModifiedEdges  []EdgeDiffItem `json:"modifiedEdges"`
	UnchangedCount int            `json:"unchangedCount"`
}

// EdgeDiffItem 边差异项
type EdgeDiffItem struct {
	ADeviceID  string  `json:"aDeviceId"`
	AIf        string  `json:"aIf"`
	BDeviceID  string  `json:"bDeviceId"`
	BIf        string  `json:"bIf"`
	Status     string  `json:"status"`
	Confidence float64 `json:"confidence"`
	EdgeType   string  `json:"edgeType"`
}
