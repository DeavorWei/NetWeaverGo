// Package models 包含所有数据库模型定义
package models

import "time"

// ============================================================================
// 发现任务相关模型
// ============================================================================

// DiscoveryTaskPhase 发现任务阶段
type DiscoveryTaskPhase string

const (
	PhasePending    DiscoveryTaskPhase = "pending"    // 等待启动
	PhaseCollecting DiscoveryTaskPhase = "collecting" // SSH 采集中
	PhaseParsing    DiscoveryTaskPhase = "parsing"    // 结构化解析中
	PhaseBuilding   DiscoveryTaskPhase = "building"   // 拓扑构建中
	PhaseCompleted  DiscoveryTaskPhase = "completed"  // 完成
	PhaseFailed     DiscoveryTaskPhase = "failed"     // 失败
	PhaseCancelled  DiscoveryTaskPhase = "cancelled"  // 已取消
)

// DiscoveryTask 发现任务主表
type DiscoveryTask struct {
	ID             string             `json:"id" gorm:"primaryKey"`
	Name           string             `json:"name"`
	Status         string             `json:"status"`         // 终态：completed / failed / cancelled
	Phase          DiscoveryTaskPhase `json:"phase"`          // 当前阶段
	PhaseStartedAt *time.Time         `json:"phaseStartedAt"` // 当前阶段开始时间
	PhaseProgress  int                `json:"phaseProgress"`  // 当前阶段进度 (0-100)
	TotalCount     int                `json:"totalCount"`
	SuccessCount   int                `json:"successCount"`
	FailedCount    int                `json:"failedCount"`
	StartedAt      *time.Time         `json:"startedAt"`
	FinishedAt     *time.Time         `json:"finishedAt"`
	CreatedAt      time.Time          `json:"createdAt"`
	UpdatedAt      time.Time          `json:"updatedAt"`
	MaxWorkers     int                `json:"maxWorkers"`
	TimeoutSec     int                `json:"timeoutSec"`
	Vendor         string             `json:"vendor"`
	ParseErrors    string             `json:"parseErrors" gorm:"type:text"` // JSON 序列化的解析错误
}

// TableName 指定表名
func (DiscoveryTask) TableName() string {
	return "discovery_tasks"
}

// DiscoveryDevice 发现设备结果表
type DiscoveryDevice struct {
	ID             uint       `json:"id" gorm:"primaryKey;autoIncrement"`
	TaskID         string     `json:"taskId" gorm:"index;not null"`
	DeviceIP       string     `json:"deviceIp" gorm:"index;not null"`
	DeviceID       uint       `json:"deviceId"`
	Status         string     `json:"status"` // pending / running / success / partial / failed
	ErrorMessage   string     `json:"errorMessage"`
	StartedAt      *time.Time `json:"startedAt"`
	FinishedAt     *time.Time `json:"finishedAt"`
	DisplayName    string     `json:"displayName"`
	Role           string     `json:"role"`
	Site           string     `json:"site"`
	Vendor         string     `json:"vendor"`
	Model          string     `json:"model"`
	SerialNumber   string     `json:"serialNumber"`
	Version        string     `json:"version"`
	Hostname       string     `json:"hostname"`
	NormalizedName string     `json:"normalizedName"`
	MgmtIP         string     `json:"mgmtIp"`
	ChassisID      string     `json:"chassisId"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

// TableName 指定表名
func (DiscoveryDevice) TableName() string {
	return "discovery_devices"
}

// RawCommandOutput 命令输出索引表
// FilePath 存储 normalized output（规范化输出，供 parser 读取）
// RawFilePath 存储原始审计输出（供审计和排障使用）
type RawCommandOutput struct {
	ID         uint   `json:"id" gorm:"primaryKey;autoIncrement"`
	TaskID     string `json:"taskId" gorm:"index;not null"`
	DeviceIP   string `json:"deviceIp" gorm:"index;not null"`
	CommandKey string `json:"commandKey" gorm:"index;not null"` // version, lldp_neighbor, interface 等
	Command    string `json:"command"`

	FilePath    string `json:"filePath"`    // normalized output 路径（供 parser 读取）
	RawFilePath string `json:"rawFilePath"` // 原始审计输出路径（供审计排障）

	Status       string `json:"status"` // pending / success / failed
	ErrorMessage string `json:"errorMessage"`

	ParseStatus string `json:"parseStatus"` // pending / success / parse_failed / skipped
	ParseError  string `json:"parseError"`

	RawSize        int64 `json:"rawSize"`        // 原始输出大小
	NormalizedSize int64 `json:"normalizedSize"` // 规范化输出大小
	LineCount      int   `json:"lineCount"`      // 规范化输出行数
	PagerCount     int   `json:"pagerCount"`     // 分页符处理次数
	EchoConsumed   bool  `json:"echoConsumed"`   // 回显是否被消费
	PromptMatched  bool  `json:"promptMatched"`  // 是否匹配到提示符

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// TableName 指定表名
func (RawCommandOutput) TableName() string {
	return "raw_command_outputs"
}

// GetParseFilePath 返回解析器应读取的文件路径（normalized output）
func (r *RawCommandOutput) GetParseFilePath() string {
	return r.FilePath
}

// GetAuditFilePath 返回审计用的原始输出路径
func (r *RawCommandOutput) GetAuditFilePath() string {
	return r.RawFilePath
}

// ============================================================================
// 发现任务视图模型
// ============================================================================

// StartDiscoveryRequest 启动发现任务请求
type StartDiscoveryRequest struct {
	DeviceIDs  []string `json:"deviceIds"`
	GroupNames []string `json:"groupNames"`
	Vendor     string   `json:"vendor"`
	MaxWorkers int      `json:"maxWorkers"`
	TimeoutSec int      `json:"timeoutSec"`
}

// TaskStartResponse 任务启动响应
type TaskStartResponse struct {
	TaskID string `json:"taskId"`
}

// DiscoveryTaskView 发现任务视图
type DiscoveryTaskView struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	Status       string     `json:"status"`
	TotalCount   int        `json:"totalCount"`
	SuccessCount int        `json:"successCount"`
	FailedCount  int        `json:"failedCount"`
	StartedAt    *time.Time `json:"startedAt"`
	FinishedAt   *time.Time `json:"finishedAt"`
	CreatedAt    time.Time  `json:"createdAt"`
	MaxWorkers   int        `json:"maxWorkers"`
	Vendor       string     `json:"vendor"`
}

// DiscoveryDeviceView 发现设备视图
type DiscoveryDeviceView struct {
	ID           uint       `json:"id"`
	TaskID       string     `json:"taskId"`
	DeviceIP     string     `json:"deviceIp"`
	Status       string     `json:"status"`
	ErrorMessage string     `json:"errorMessage"`
	StartedAt    *time.Time `json:"startedAt"`
	FinishedAt   *time.Time `json:"finishedAt"`
	DisplayName  string     `json:"displayName"`
	Role         string     `json:"role"`
	Site         string     `json:"site"`
	Vendor       string     `json:"vendor"`
	Model        string     `json:"model"`
	SerialNumber string     `json:"serialNumber"`
	Version      string     `json:"version"`
	Hostname     string     `json:"hostname"`
	MgmtIP       string     `json:"mgmtIp"`
	ChassisID    string     `json:"chassisId"`
}

// DeviceInfo 设备信息（用于任务执行）
type DeviceInfo struct {
	ID          uint
	IP          string
	Port        int
	Username    string
	Password    string
	Vendor      string
	DisplayName string
	Role        string
	Site        string
}
