package executor

import (
	"fmt"
	"time"
)

// PlanExecutionMode 执行计划模式
// 注意：避免与 RunMode 的 ModePlaybook 冲突
type PlanExecutionMode int

const (
	// PlanModeDiscovery 拓扑发现模式
	PlanModeDiscovery PlanExecutionMode = iota
	// PlanModePlaybook 剧本执行模式
	PlanModePlaybook
)

// PlannedCommand 计划化命令
type PlannedCommand struct {
	Key             string        // 命令标识: version, lldp_neighbor等
	Command         string        // 实际命令文本
	Timeout         time.Duration // 命令级超时
	ContinueOnError bool          // 错误时是否继续
}

// ExecutionPlan 执行计划
type ExecutionPlan struct {
	Name                  string            // 计划名称
	Commands              []PlannedCommand  // 命令列表
	AbortOnTransportErr   bool              // 传输错误时中止
	AbortOnCommandTimeout bool              // 命令超时时中止
	ContinueOnCmdError    bool              // 命令错误时继续(全局默认)
	Mode                  PlanExecutionMode // 执行模式
}

// ExecutionReport 执行报告
type ExecutionReport struct {
	PlanName       string           // 计划名称
	Results        []*CommandResult // 命令结果列表
	FatalError     error            // 会话级致命错误
	SessionHealthy bool             // 会话是否健康
	StartedAt      time.Time        // 开始时间
	FinishedAt     time.Time        // 结束时间

	// === 新增字段 ===
	// 执行统计
	TotalCommands   int // 总命令数
	SuccessCountVal int // 成功数（缓存值）
	FailureCountVal int // 失败数（缓存值）

	// 时间统计
	InitDuration     time.Duration // 初始化耗时
	CommandsDuration time.Duration // 命令执行耗时
	AverageCmdTime   time.Duration // 平均命令耗时
	MaxCmdTime       time.Duration // 最大命令耗时
	MinCmdTime       time.Duration // 最小命令耗时

	// 资源使用
	TotalBytesRead   int64 // 读取的总字节数
	TotalLinesParsed int   // 解析的总行数
}

// IsSuccess 检查执行是否成功
func (r *ExecutionReport) IsSuccess() bool {
	if r.FatalError != nil || !r.SessionHealthy {
		return false
	}
	for _, result := range r.Results {
		if !result.Success {
			return false
		}
	}
	return true
}

// PartialSuccess 检查是否有部分成功
func (r *ExecutionReport) PartialSuccess() bool {
	if len(r.Results) == 0 {
		return false
	}
	successCount := 0
	for _, result := range r.Results {
		if result.Success {
			successCount++
		}
	}
	return successCount > 0 && successCount < len(r.Results)
}

// SuccessCount 返回成功命令数
func (r *ExecutionReport) SuccessCount() int {
	if r.SuccessCountVal > 0 {
		return r.SuccessCountVal
	}
	count := 0
	for _, result := range r.Results {
		if result.Success {
			count++
		}
	}
	return count
}

// FailureCount 返回失败命令数
func (r *ExecutionReport) FailureCount() int {
	if r.FailureCountVal > 0 {
		return r.FailureCountVal
	}
	count := 0
	for _, result := range r.Results {
		if !result.Success {
			count++
		}
	}
	return count
}

// ComputeStats 计算统计信息
func (r *ExecutionReport) ComputeStats() {
	r.TotalCommands = len(r.Results)
	r.SuccessCountVal = 0
	r.FailureCountVal = 0

	for _, result := range r.Results {
		if result.Success {
			r.SuccessCountVal++
		} else {
			r.FailureCountVal++
		}
	}

	// 计算命令执行时间统计
	if len(r.Results) > 0 {
		var totalCmdTime time.Duration
		r.MaxCmdTime = 0
		r.MinCmdTime = time.Hour * 24

		for _, result := range r.Results {
			if result.Duration > r.MaxCmdTime {
				r.MaxCmdTime = result.Duration
			}
			if result.Duration < r.MinCmdTime {
				r.MinCmdTime = result.Duration
			}
			totalCmdTime += result.Duration
		}

		r.AverageCmdTime = totalCmdTime / time.Duration(len(r.Results))
	}

	// 计算命令执行总耗时
	if r.FinishedAt.After(r.StartedAt) {
		totalDuration := r.FinishedAt.Sub(r.StartedAt)
		r.CommandsDuration = totalDuration - r.InitDuration
	}
}

// String 返回执行报告的字符串表示
func (r *ExecutionReport) String() string {
	return fmt.Sprintf("ExecutionReport{PlanName: %s, Success: %d, Failed: %d, SessionHealthy: %v, FatalError: %v}",
		r.PlanName, r.SuccessCount(), r.FailureCount(), r.SessionHealthy, r.FatalError)
}

// ============================================================================
// 执行事件 (用于实时监控)
// ============================================================================

// ExecutionEventType 兼容旧链路的事件类型。
type ExecutionEventType string

const (
	// EventCmdStart 命令开始
	EventCmdStart ExecutionEventType = "cmd_start"
	// EventCmdComplete 命令完成
	EventCmdComplete ExecutionEventType = "cmd_complete"
	// EventError 错误事件
	EventError ExecutionEventType = "error"
	// EventComplete 执行完成
	EventComplete ExecutionEventType = "complete"
)

// ExecutionRecordKind 结构化执行记录类型。
type ExecutionRecordKind string

const (
	// RecordCommandDispatched 命令已发送到设备。
	RecordCommandDispatched ExecutionRecordKind = "command_dispatched"
	// RecordCommandCompleted 命令成功完成。
	RecordCommandCompleted ExecutionRecordKind = "command_completed"
	// RecordCommandFailed 命令执行失败，但结果已经封存。
	RecordCommandFailed ExecutionRecordKind = "command_failed"
	// RecordExecutionCompleted 本次执行计划已结束。
	RecordExecutionCompleted ExecutionRecordKind = "execution_completed"
)

// ExecutionEvent 执行过程中的结构化事件。
type ExecutionEvent struct {
	Type          ExecutionEventType  `json:"type"`
	Kind          ExecutionRecordKind `json:"kind"`
	SessionSeq    uint64              `json:"sessionSeq"`
	Command       string              `json:"command,omitempty"`
	Key           string              `json:"key,omitempty"`
	Index         int                 `json:"index"`
	TotalCommands int                 `json:"totalCommands,omitempty"`
	Duration      time.Duration       `json:"duration,omitempty"`
	Error         error               `json:"-"`
	ErrorMessage  string              `json:"errorMessage,omitempty"`
	Timestamp     time.Time           `json:"timestamp"`
}
