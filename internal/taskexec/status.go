package taskexec

// RunStatus Run状态
type RunStatus string

const (
	// RunStatusPending 等待执行
	RunStatusPending RunStatus = "pending"
	// RunStatusRunning 执行中
	RunStatusRunning RunStatus = "running"
	// RunStatusCompleted 全部成功
	RunStatusCompleted RunStatus = "completed"
	// RunStatusPartial 部分成功
	RunStatusPartial RunStatus = "partial"
	// RunStatusFailed 失败
	RunStatusFailed RunStatus = "failed"
	// RunStatusCancelled 已取消
	RunStatusCancelled RunStatus = "cancelled"
)

// IsTerminal 是否终态
func (s RunStatus) IsTerminal() bool {
	switch s {
	case RunStatusCompleted, RunStatusPartial, RunStatusFailed, RunStatusCancelled:
		return true
	default:
		return false
	}
}

// StageStatus Stage状态
type StageStatus string

const (
	// StageStatusPending 等待执行
	StageStatusPending StageStatus = "pending"
	// StageStatusRunning 执行中
	StageStatusRunning StageStatus = "running"
	// StageStatusCompleted 全部成功
	StageStatusCompleted StageStatus = "completed"
	// StageStatusPartial 部分成功
	StageStatusPartial StageStatus = "partial"
	// StageStatusFailed 失败
	StageStatusFailed StageStatus = "failed"
	// StageStatusCancelled 已取消
	StageStatusCancelled StageStatus = "cancelled"
)

// IsTerminal 是否终态
func (s StageStatus) IsTerminal() bool {
	switch s {
	case StageStatusCompleted, StageStatusPartial, StageStatusFailed, StageStatusCancelled:
		return true
	default:
		return false
	}
}

// UnitStatus Unit状态
type UnitStatus string

const (
	// UnitStatusPending 等待执行
	UnitStatusPending UnitStatus = "pending"
	// UnitStatusRunning 执行中
	UnitStatusRunning UnitStatus = "running"
	// UnitStatusCompleted 全部成功
	UnitStatusCompleted UnitStatus = "completed"
	// UnitStatusPartial 部分成功
	UnitStatusPartial UnitStatus = "partial"
	// UnitStatusFailed 失败
	UnitStatusFailed UnitStatus = "failed"
	// UnitStatusCancelled 已取消
	UnitStatusCancelled UnitStatus = "cancelled"
)

// IsTerminal 是否终态
func (s UnitStatus) IsTerminal() bool {
	switch s {
	case UnitStatusCompleted, UnitStatusPartial, UnitStatusFailed, UnitStatusCancelled:
		return true
	default:
		return false
	}
}

// StageKind Stage类型
type StageKind string

const (
	// StageKindDeviceCommand 设备命令执行 (普通任务)
	StageKindDeviceCommand StageKind = "device_command"
	// StageKindDeviceCollect 设备采集 (拓扑任务)
	StageKindDeviceCollect StageKind = "device_collect"
	// StageKindParse 解析阶段
	StageKindParse StageKind = "parse"
	// StageKindTopologyBuild 拓扑构建阶段
	StageKindTopologyBuild StageKind = "topology_build"
)

// UnitKind Unit类型
type UnitKind string

const (
	// UnitKindDevice 设备类型
	UnitKindDevice UnitKind = "device"
	// UnitKindRun 运行类型
	UnitKindRun UnitKind = "run"
	// UnitKindDataset 数据集类型
	UnitKindDataset UnitKind = "dataset"
)

// RunKind 运行类型
type RunKind string

const (
	// RunKindNormal 普通任务
	RunKindNormal RunKind = "normal"
	// RunKindTopology 拓扑任务
	RunKindTopology RunKind = "topology"
)

// EventType 事件类型
type EventType string

const (
	// EventTypeRunStarted Run开始
	EventTypeRunStarted EventType = "run_started"
	// EventTypeRunProgress Run进度
	EventTypeRunProgress EventType = "run_progress"
	// EventTypeRunFinished Run结束
	EventTypeRunFinished EventType = "run_finished"
	// EventTypeStageStarted Stage开始
	EventTypeStageStarted EventType = "stage_started"
	// EventTypeStageProgress Stage进度
	EventTypeStageProgress EventType = "stage_progress"
	// EventTypeStageFinished Stage结束
	EventTypeStageFinished EventType = "stage_finished"
	// EventTypeUnitStarted Unit开始
	EventTypeUnitStarted EventType = "unit_started"
	// EventTypeUnitProgress Unit进度
	EventTypeUnitProgress EventType = "unit_progress"
	// EventTypeUnitFinished Unit结束
	EventTypeUnitFinished EventType = "unit_finished"
	// EventTypeStepStarted Step开始
	EventTypeStepStarted EventType = "step_started"
	// EventTypeStepFinished Step结束
	EventTypeStepFinished EventType = "step_finished"
	// EventTypeLog 日志
	EventTypeLog EventType = "log"
	// EventTypeWarning 警告
	EventTypeWarning EventType = "warning"
	// EventTypeError 错误
	EventTypeError EventType = "error"
)

// EventLevel 事件级别
type EventLevel string

const (
	// EventLevelInfo 信息
	EventLevelInfo EventLevel = "info"
	// EventLevelWarn 警告
	EventLevelWarn EventLevel = "warn"
	// EventLevelError 错误
	EventLevelError EventLevel = "error"
)

// ArtifactType 产物类型
type ArtifactType string

const (
	// ArtifactTypeRawOutput 原始输出
	ArtifactTypeRawOutput ArtifactType = "raw_output"
	// ArtifactTypeNormalizedOutput 规范化输出
	ArtifactTypeNormalizedOutput ArtifactType = "normalized_output"
	// ArtifactTypeParseResult 解析结果
	ArtifactTypeParseResult ArtifactType = "parse_result"
	// ArtifactTypeTopologyGraph 拓扑图
	ArtifactTypeTopologyGraph ArtifactType = "topology_graph"
	// ArtifactTypeReport 报告
	ArtifactTypeReport ArtifactType = "report"
)
