// Package taskexec 提供统一任务执行运行时
package taskexec

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

// ==================== Execution Plan Models ====================

// ExecutionPlan 执行计划 - 运行时唯一的输入结构
type ExecutionPlan struct {
	RunKind string      `json:"runKind"` // normal / topology
	Name    string      `json:"name"`
	Stages  []StagePlan `json:"stages"`
}

// StagePlan 阶段计划
type StagePlan struct {
	ID          string     `json:"id"`
	Kind        string     `json:"kind"` // device_command / device_collect / parse / topology_build
	Name        string     `json:"name"`
	Order       int        `json:"order"`
	Concurrency int        `json:"concurrency"` // 并发数，0表示不限制
	Units       []UnitPlan `json:"units"`
}

// UnitPlan 调度单元计划
type UnitPlan struct {
	ID      string        `json:"id"`
	Kind    string        `json:"kind"` // device / run / dataset
	Target  TargetRef     `json:"target"`
	Timeout time.Duration `json:"timeout"`
	Steps   []StepPlan    `json:"steps"`
}

// TargetRef 目标引用
type TargetRef struct {
	Type string `json:"type"` // device_ip / task_run / dataset_key
	Key  string `json:"key"`  // 目标标识
}

// StepPlan 步骤计划
type StepPlan struct {
	ID         string            `json:"id"`
	Kind       string            `json:"kind"` // command / parse / build
	Name       string            `json:"name"`
	Command    string            `json:"command"`    // 实际命令内容
	CommandKey string            `json:"commandKey"` // 命令标识
	Params     map[string]string `json:"params"`
}

// ==================== Task Definition Models ====================

// TaskDefinition 任务定义 - 表示用户配置的任务
type TaskDefinition struct {
	ID          string          `gorm:"primaryKey" json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Kind        string          `json:"kind"` // normal / topology
	Config      json.RawMessage `gorm:"type:text" json:"config"`
	CreatedAt   time.Time       `json:"createdAt"`
	UpdatedAt   time.Time       `json:"updatedAt"`
}

// TableName 指定表名
func (TaskDefinition) TableName() string {
	return "task_definitions"
}

// ==================== Runtime State Models ====================

// TaskRun 任务运行实例 - 统一运行时的真相源
type TaskRun struct {
	ID               string         `gorm:"primaryKey" json:"id"`
	TaskDefinitionID string         `json:"taskDefinitionId"`
	TaskGroupID      uint           `gorm:"index" json:"taskGroupId"`
	TaskNameSnapshot string         `json:"taskNameSnapshot"`
	LaunchSpecJSON   string         `gorm:"type:text" json:"launchSpecJson"`
	Name             string         `json:"name"`
	RunKind          string         `json:"runKind"` // normal / topology
	Status           string         `json:"status"`  // pending / running / completed / partial / failed / cancelled / aborted
	CurrentStage     string         `json:"currentStage"`
	Progress         int            `json:"progress"` // 0-100
	LastRunSeq       uint64         `gorm:"default:0" json:"lastRunSeq"`
	StartedAt        *time.Time     `json:"startedAt"`
	FinishedAt       *time.Time     `json:"finishedAt"`
	CreatedAt        time.Time      `json:"createdAt"`
	UpdatedAt        time.Time      `json:"updatedAt"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName 指定表名
func (TaskRun) TableName() string {
	return "task_runs"
}

// TaskRunStage 阶段运行状态
type TaskRunStage struct {
	ID             string     `gorm:"primaryKey" json:"id"`
	TaskRunID      string     `gorm:"index" json:"taskRunId"`
	StageKind      string     `json:"stageKind"` // device_command / device_collect / parse / topology_build
	StageName      string     `json:"stageName"`
	StageOrder     int        `json:"stageOrder"`
	Status         string     `json:"status"`   // pending / running / completed / partial / failed / cancelled
	Progress       int        `json:"progress"` // 0-100
	TotalUnits     int        `json:"totalUnits"`
	CompletedUnits int        `json:"completedUnits"`
	SuccessUnits   int        `json:"successUnits"`
	FailedUnits    int        `json:"failedUnits"`
	CancelledUnits int        `json:"cancelledUnits"`
	PartialUnits   int        `json:"partialUnits"`
	StartedAt      *time.Time `json:"startedAt"`
	FinishedAt     *time.Time `json:"finishedAt"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

// TableName 指定表名
func (TaskRunStage) TableName() string {
	return "task_run_stages"
}

// TaskRunUnit 调度单元状态
type TaskRunUnit struct {
	ID             string     `gorm:"primaryKey" json:"id"`
	TaskRunID      string     `gorm:"index" json:"taskRunId"`
	TaskRunStageID string     `gorm:"index" json:"taskRunStageId"`
	UnitKind       string     `json:"unitKind"`   // device / run / dataset
	TargetType     string     `json:"targetType"` // device_ip / task_run / dataset_key
	TargetKey      string     `json:"targetKey"`  // 目标标识
	Status         string     `json:"status"`     // pending / running / completed / partial / failed / cancelled
	TotalSteps     int        `json:"totalSteps"`
	DoneSteps      int        `json:"doneSteps"`
	ErrorMessage   string     `json:"errorMessage"`
	StartedAt      *time.Time `json:"startedAt"`
	FinishedAt     *time.Time `json:"finishedAt"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

// TableName 指定表名
func (TaskRunUnit) TableName() string {
	return "task_run_units"
}

// TaskRunEvent 事件流水
type TaskRunEvent struct {
	ID          string    `gorm:"primaryKey" json:"id"`
	TaskRunID   string    `gorm:"index" json:"taskRunId"`
	StageID     string    `gorm:"index" json:"stageId"`
	UnitID      string    `gorm:"index" json:"unitId"`
	EventType   string    `json:"eventType"`  // run_started, stage_started, unit_started, step_finished, etc.
	EventLevel  string    `json:"eventLevel"` // info / warn / error
	RunSeq      uint64    `gorm:"index" json:"runSeq"`
	SessionSeq  uint64    `gorm:"index" json:"sessionSeq"`
	Message     string    `json:"message"`
	PayloadJSON string    `gorm:"type:text" json:"payloadJson"` // 扩展数据
	CreatedAt   time.Time `json:"createdAt"`
}

// TableName 指定表名
func (TaskRunEvent) TableName() string {
	return "task_run_events"
}

// TaskArtifact 产物索引
type TaskArtifact struct {
	ID           string    `gorm:"primaryKey" json:"id"`
	TaskRunID    string    `gorm:"index" json:"taskRunId"`
	StageID      string    `gorm:"index" json:"stageId"`
	UnitID       string    `gorm:"index" json:"unitId"`
	ArtifactType string    `json:"artifactType"` // raw_output / normalized_output / parse_result / topology_graph / report
	ArtifactKey  string    `json:"artifactKey"`
	FilePath     string    `json:"filePath"`
	MetaJSON     string    `gorm:"type:text" json:"metaJson"` // 扩展元数据
	CreatedAt    time.Time `json:"createdAt"`
}

// TableName 指定表名
func (TaskArtifact) TableName() string {
	return "task_artifacts"
}

// ==================== Patch Models for Updates ====================

// RunPatch Run更新补丁
type RunPatch struct {
	Status       *string    `json:"status,omitempty"`
	CurrentStage *string    `json:"currentStage,omitempty"`
	Progress     *int       `json:"progress,omitempty"`
	LastRunSeq   *uint64    `json:"lastRunSeq,omitempty"`
	StartedAt    *time.Time `json:"startedAt,omitempty"`
	FinishedAt   *time.Time `json:"finishedAt,omitempty"`
}

// StagePatch Stage更新补丁
type StagePatch struct {
	Status         *string    `json:"status,omitempty"`
	Progress       *int       `json:"progress,omitempty"`
	CompletedUnits *int       `json:"completedUnits,omitempty"`
	SuccessUnits   *int       `json:"successUnits,omitempty"`
	FailedUnits    *int       `json:"failedUnits,omitempty"`
	CancelledUnits *int       `json:"cancelledUnits,omitempty"`
	PartialUnits   *int       `json:"partialUnits,omitempty"`
	StartedAt      *time.Time `json:"startedAt,omitempty"`
	FinishedAt     *time.Time `json:"finishedAt,omitempty"`
}

// UnitPatch Unit更新补丁
type UnitPatch struct {
	Status       *string    `json:"status,omitempty"`
	DoneSteps    *int       `json:"doneSteps,omitempty"`
	ErrorMessage *string    `json:"errorMessage,omitempty"`
	StartedAt    *time.Time `json:"startedAt,omitempty"`
	FinishedAt   *time.Time `json:"finishedAt,omitempty"`
}
