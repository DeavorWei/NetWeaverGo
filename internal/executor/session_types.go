package executor

import (
	"time"

	"github.com/NetWeaverGo/core/internal/matcher"
)

const DefaultMaxPaginationCount = 100

// ============================================================================
// 新状态模型 (Phase 1 重构)
// ============================================================================
// 这是重构后的新状态类型，与旧状态并存，逐步迁移
// 状态枚举只保留核心状态，删除瞬时状态和 flag 驱动的状态

// NewSessionState 会话状态枚举（重构后）
type NewSessionState int

const (
	// NewStateInitAwaitPrompt 等待初始提示符
	NewStateInitAwaitPrompt NewSessionState = iota

	// NewStateInitAwaitWarmupPrompt 等待预热后提示符
	NewStateInitAwaitWarmupPrompt

	// NewStateReady 就绪状态
	NewStateReady

	// NewStateRunning 命令执行中
	NewStateRunning

	// NewStateAwaitPagerContinueAck 等待分页续页确认
	NewStateAwaitPagerContinueAck

	// NewStateSuspended 挂起状态
	NewStateSuspended

	// NewStateCompleted 完成状态
	NewStateCompleted

	// NewStateFailed 失败状态
	NewStateFailed
)

// String 返回状态的字符串表示
func (s NewSessionState) String() string {
	switch s {
	case NewStateInitAwaitPrompt:
		return "InitAwaitPrompt"
	case NewStateInitAwaitWarmupPrompt:
		return "InitAwaitWarmupPrompt"
	case NewStateReady:
		return "Ready"
	case NewStateRunning:
		return "Running"
	case NewStateAwaitPagerContinueAck:
		return "AwaitPagerContinueAck"
	case NewStateSuspended:
		return "Suspended"
	case NewStateCompleted:
		return "Completed"
	case NewStateFailed:
		return "Failed"
	default:
		return "Unknown"
	}
}

// IsTerminal 判断状态是否为终态
func (s NewSessionState) IsTerminal() bool {
	return s == NewStateCompleted || s == NewStateFailed
}

// ============================================================================
// 协议事件 (Protocol Events)
// ============================================================================

// SessionEvent 协议事件接口
type SessionEvent interface {
	EventType() string
}

// EvChunkProcessed chunk 处理完成事件
type EvChunkProcessed struct {
	LinesProcessed int
}

func (e EvChunkProcessed) EventType() string { return "ChunkProcessed" }

// EvCommittedLine 行提交事件
type EvCommittedLine struct {
	Line string
}

func (e EvCommittedLine) EventType() string { return "CommittedLine" }

// EvActivePromptSeen 活动行检测到提示符事件
type EvActivePromptSeen struct {
	Prompt string
}

func (e EvActivePromptSeen) EventType() string { return "ActivePromptSeen" }

// EvPagerSeen 检测到分页符事件
type EvPagerSeen struct {
	Line string
}

func (e EvPagerSeen) EventType() string { return "PagerSeen" }

// EvErrorMatched 检测到错误规则命中事件
type EvErrorMatched struct {
	Line string
	Rule *matcher.ErrorRule
}

func (e EvErrorMatched) EventType() string { return "ErrorMatched" }

// EvTimeout 超时事件
type EvTimeout struct {
	CommandIndex int
}

func (e EvTimeout) EventType() string { return "Timeout" }

// EvUserContinue 用户选择继续事件
type EvUserContinue struct {
	CommandIndex int
}

func (e EvUserContinue) EventType() string { return "UserContinue" }

// EvUserAbort 用户选择中止事件
type EvUserAbort struct {
	CommandIndex int
}

func (e EvUserAbort) EventType() string { return "UserAbort" }

// EvSuspendTimeout 挂起超时事件
type EvSuspendTimeout struct {
	CommandIndex int
	Reason       string
}

func (e EvSuspendTimeout) EventType() string { return "SuspendTimeout" }

// EvStreamClosed 流关闭事件
type EvStreamClosed struct{}

func (e EvStreamClosed) EventType() string { return "StreamClosed" }

// EvInitPromptStable 初始提示符稳定事件
type EvInitPromptStable struct {
	Prompt string
}

func (e EvInitPromptStable) EventType() string { return "InitPromptStable" }

// EvWarmupPromptSeen 预热后提示符检测事件
type EvWarmupPromptSeen struct {
	Prompt string
}

func (e EvWarmupPromptSeen) EventType() string { return "WarmupPromptSeen" }

// EvCommandPromptSeen 命令完成后提示符检测事件
type EvCommandPromptSeen struct {
	Prompt string
}

func (e EvCommandPromptSeen) EventType() string { return "CommandPromptSeen" }

// ============================================================================
// 动作类型 (Action Types)
// ============================================================================

// SessionAction 动作接口
type SessionAction interface {
	ActionType() string
}

// ActSendWarmup 发送预热空行动作
type ActSendWarmup struct{}

func (a ActSendWarmup) ActionType() string { return "SendWarmup" }

// ActSendCommand 发送命令动作
type ActSendCommand struct {
	Index   int
	Command string
}

func (a ActSendCommand) ActionType() string { return "SendCommand" }

// ActSendPagerContinue 发送分页续页动作
type ActSendPagerContinue struct{}

func (a ActSendPagerContinue) ActionType() string { return "SendPagerContinue" }

// ActEmitCommandStart 发送命令开始事件动作
type ActEmitCommandStart struct {
	Index   int
	Command string
}

func (a ActEmitCommandStart) ActionType() string { return "EmitCommandStart" }

// ActEmitCommandDone 发送命令完成事件动作
type ActEmitCommandDone struct {
	Index    int
	Success  bool
	Duration time.Duration
}

func (a ActEmitCommandDone) ActionType() string { return "EmitCommandDone" }

// ActEmitDeviceError 发送设备错误事件动作
type ActEmitDeviceError struct {
	Index   int
	Message string
}

func (a ActEmitDeviceError) ActionType() string { return "EmitDeviceError" }

// ActRequestSuspendDecision 请求挂起决策动作
type ActRequestSuspendDecision struct {
	ErrorContext *ErrorContext
}

func (a ActRequestSuspendDecision) ActionType() string { return "RequestSuspendDecision" }

// ActAbortSession 中止会话动作
type ActAbortSession struct {
	Reason string
}

func (a ActAbortSession) ActionType() string { return "AbortSession" }

// ActResetReadTimeout 重置读取超时动作
type ActResetReadTimeout struct {
	Timeout time.Duration
}

func (a ActResetReadTimeout) ActionType() string { return "ResetReadTimeout" }

// ActFlushDetailLog 刷新详细日志动作
type ActFlushDetailLog struct {
	Lines []string
}

func (a ActFlushDetailLog) ActionType() string { return "FlushDetailLog" }

// ActClearInitResiduals 清理初始化残留动作
type ActClearInitResiduals struct{}

func (a ActClearInitResiduals) ActionType() string { return "ClearInitResiduals" }

// ============================================================================
// 会话上下文 (Session Context)
// ============================================================================

// SessionContext 会话上下文
type SessionContext struct {
	// 命令队列
	Queue []string

	// 命令标识队列（与 Queue 一一对应）
	CommandKeys []string

	// 下一条命令索引
	NextIndex int

	// 当前命令上下文
	Current *CommandContext

	// 待处理的逻辑行
	PendingLines []string

	// 提示符指纹
	PromptFingerprint string

	// 最后一次 chunk 处理时间
	LastChunkAt time.Time

	// 初始化残留是否已清理
	InitResidualCleared bool

	// 已完成的命令结果
	Results []*CommandResult

	// 待处理的错误上下文
	PendingError *ErrorContext

	// 分页次数上限
	MaxPaginationCount int
}

// NewSessionContext 创建新的会话上下文
func NewSessionContext(commands []string) *SessionContext {
	return &SessionContext{
		Queue:              commands,
		CommandKeys:        make([]string, len(commands)),
		NextIndex:          0,
		PendingLines:       make([]string, 0),
		Results:            make([]*CommandResult, 0),
		MaxPaginationCount: DefaultMaxPaginationCount,
	}
}

// NewSessionContextWithKeys 创建带命令标识的会话上下文
func NewSessionContextWithKeys(commands []string, keys []string) *SessionContext {
	ctx := NewSessionContext(commands)
	if len(keys) == len(commands) {
		ctx.CommandKeys = keys
	}
	return ctx
}

// HasMoreCommands 是否还有更多命令
func (c *SessionContext) HasMoreCommands() bool {
	return c.NextIndex < len(c.Queue)
}

// CurrentCommand 获取当前命令
func (c *SessionContext) CurrentCommand() string {
	if c.Current == nil {
		return ""
	}
	return c.Current.Command
}

// AdvanceCommand 推进到下一条命令
func (c *SessionContext) AdvanceCommand() *CommandContext {
	if !c.HasMoreCommands() {
		return nil
	}

	rawCmd := c.Queue[c.NextIndex]
	ctx := NewCommandContext(c.NextIndex, rawCmd)

	// 解析命令文本（去除内联注释等）
	cmdToSend, _ := parseInlineCommand(rawCmd)
	ctx.SetCommand(cmdToSend)

	// 注意：CommandKey 不在此处设置，由 executor 在结果映射阶段回填
	// 保持 ctx.Command 为实际命令文本，确保日志和输出正确

	c.NextIndex++
	c.Current = ctx
	return ctx
}

// SetCommandKeys 设置命令标识列表
func (c *SessionContext) SetCommandKeys(keys []string) {
	if len(keys) == len(c.Queue) {
		c.CommandKeys = keys
	}
}

// GetCommandKey 获取指定索引的命令标识
func (c *SessionContext) GetCommandKey(index int) string {
	if index >= 0 && index < len(c.CommandKeys) {
		return c.CommandKeys[index]
	}
	return ""
}

// AddPendingLine 添加待处理行
func (c *SessionContext) AddPendingLine(line string) {
	c.PendingLines = append(c.PendingLines, line)
}

// ClearPendingLines 清空待处理行
func (c *SessionContext) ClearPendingLines() {
	c.PendingLines = c.PendingLines[:0]
}

// HasPendingLines 是否有待处理行
func (c *SessionContext) HasPendingLines() bool {
	return len(c.PendingLines) > 0
}

// CompleteCurrentCommand 完成当前命令
func (c *SessionContext) CompleteCurrentCommand() {
	if c.Current != nil {
		c.Current.MarkCompleted()
		c.Results = append(c.Results, c.Current.ToResult())
	}
}

// FailCurrentCommand 标记当前命令失败
func (c *SessionContext) FailCurrentCommand(errMsg string) {
	if c.Current != nil {
		c.Current.MarkFailed(errMsg)
		c.Results = append(c.Results, c.Current.ToResult())
	}
}
