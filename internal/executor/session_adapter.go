package executor

import (
	"time"

	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/matcher"
	"github.com/NetWeaverGo/core/internal/terminal"
)

// ============================================================================
// Session Adapter (Phase 7 重构)
// ============================================================================
// 适配器模式：桥接新旧架构，支持渐进式迁移
// - useNewArchitecture = false: 使用 SessionMachine（旧架构）
// - useNewArchitecture = true: 使用 SessionDetector + SessionReducer + SessionDriver（新架构）

// SessionAdapter 会话适配器
// 统一新旧架构的调用接口，支持灰度切换
type SessionAdapter struct {
	// 旧架构组件
	machine *SessionMachine

	// 新架构组件
	detector *SessionDetector
	reducer  *SessionReducer

	// 共享组件
	replayer *terminal.Replayer
	matcher  *matcher.StreamMatcher

	// 迁移模式控制
	useNewArchitecture bool

	// 新架构状态
	newState          NewSessionState
	newContext        *SessionContext
	newCommittedLines []string
}

// NewSessionAdapter 创建新的会话适配器
func NewSessionAdapter(width int, commands []string, m *matcher.StreamMatcher) *SessionAdapter {
	adapter := &SessionAdapter{
		machine:            NewSessionMachine(width, commands, m),
		replayer:           terminal.NewReplayer(width),
		matcher:            m,
		useNewArchitecture: true, // 默认使用新架构（Phase 8 清理后）
		newState:           NewStateInitAwaitPrompt,
		newContext:         NewSessionContext(commands),
		newCommittedLines:  make([]string, 0),
	}

	// 初始化新架构组件（因为默认使用新架构）
	adapter.detector = NewSessionDetector(m)
	adapter.reducer = NewSessionReducer(commands, m)

	return adapter
}

// SetUseNewArchitecture 设置是否使用新架构
func (a *SessionAdapter) SetUseNewArchitecture(use bool) {
	if use && a.detector == nil {
		// 初始化新架构组件
		a.InitNewArchitecture(a.newContext.Queue)
	}
	a.useNewArchitecture = use
}

// UseNewArchitecture 返回是否使用新架构
func (a *SessionAdapter) UseNewArchitecture() bool {
	return a.useNewArchitecture
}

// Feed 消费原始 chunk，返回需要执行的动作
// 统一入口，根据配置选择新旧架构
func (a *SessionAdapter) Feed(chunk string) []Action {
	if a.useNewArchitecture {
		return a.feedNew(chunk)
	}
	return a.machine.Feed(chunk)
}

// feedNew 新架构处理流程
// 流程: Replayer -> Detector -> Reducer -> Action 转换
func (a *SessionAdapter) feedNew(chunk string) []Action {
	// 1. 使用 Replayer 处理 chunk
	events := a.replayer.Process(chunk)

	// 收集新提交的行
	for _, event := range events {
		if event.Type == terminal.EventLineCommitted {
			a.newCommittedLines = append(a.newCommittedLines, event.Line)
		}
	}

	// 2. 使用 Detector 检测协议事件
	protocolEvents := a.detector.DetectFromChunk(chunk)

	// 3. 使用 Reducer 进行状态归约
	var sessionActions []SessionAction
	for _, event := range protocolEvents {
		actions := a.reducer.Reduce(event)
		sessionActions = append(sessionActions, actions...)
	}

	// 4. 转换为旧格式 Action（兼容层）
	return a.convertActions(sessionActions)
}

// convertActions 将新架构的 SessionAction 转换为旧架构的 Action
func (adapter *SessionAdapter) convertActions(actions []SessionAction) []Action {
	result := make([]Action, 0, len(actions))

	for _, action := range actions {
		switch act := action.(type) {
		case ActSendWarmup:
			result = append(result, ActionSendWarmup)
			logger.Debug("SessionAdapter", "-", "[新架构] 转换动作: SendWarmup")

		case ActSendCommand:
			result = append(result, ActionSendCommand)
			// 同步命令上下文到旧架构（用于兼容）
			if act.Index >= 0 && act.Index < len(adapter.machine.queue) {
				adapter.machine.current = NewCommandContext(act.Index, act.Command)
				adapter.machine.current.SetCommand(act.Command)
			}
			logger.Debug("SessionAdapter", "-", "[新架构] 转换动作: SendCommand(%s)", act.Command)

		case ActSendPagerContinue:
			result = append(result, ActionSendSpace)
			logger.Debug("SessionAdapter", "-", "[新架构] 转换动作: SendPagerContinue")

		case ActRequestSuspendDecision:
			// 同步错误上下文
			if act.ErrorContext != nil {
				adapter.machine.pendingError = act.ErrorContext
			}
			result = append(result, ActionHandleError)
			logger.Debug("SessionAdapter", "-", "[新架构] 转换动作: RequestSuspendDecision")

		case ActAbortSession:
			result = append(result, ActionAbortTask)
			logger.Debug("SessionAdapter", "-", "[新架构] 转换动作: AbortSession(%s)", act.Reason)

		case ActResetReadTimeout:
			// 超时重置不需要转换为 Action，由 StreamEngine 处理
			logger.Debug("SessionAdapter", "-", "[新架构] 转换动作: ResetReadTimeout")

		case ActFlushDetailLog:
			// 日志刷新不需要转换为 Action，由 StreamEngine 处理
			logger.Debug("SessionAdapter", "-", "[新架构] 转换动作: FlushDetailLog")

		case ActEmitCommandStart:
			logger.Debug("SessionAdapter", "-", "[新架构] 转换动作: EmitCommandStart(%d)", act.Index)

		case ActEmitCommandDone:
			logger.Debug("SessionAdapter", "-", "[新架构] 转换动作: EmitCommandDone(%d)", act.Index)

		case ActEmitDeviceError:
			logger.Debug("SessionAdapter", "-", "[新架构] 转换动作: EmitDeviceError(%s)", act.Message)

		default:
			logger.Debug("SessionAdapter", "-", "[新架构] 未知动作类型: %T", action)
		}
	}

	return result
}

// ============================================================================
// 委托方法 - 保持与 SessionMachine 兼容的接口
// ============================================================================

// State 返回当前状态（旧架构）
func (a *SessionAdapter) State() SessionState {
	if a.useNewArchitecture {
		return a.convertNewStateToOld(a.newState)
	}
	return a.machine.State()
}

// NewState 返回当前状态（新架构）
func (a *SessionAdapter) NewState() NewSessionState {
	if a.useNewArchitecture {
		return a.newState
	}
	return a.reducer.State()
}

// CurrentCommand 返回当前命令上下文
func (a *SessionAdapter) CurrentCommand() *CommandContext {
	if a.useNewArchitecture {
		return a.newContext.Current
	}
	return a.machine.CurrentCommand()
}

// Results 返回所有已完成的命令结果
func (a *SessionAdapter) Results() []*CommandResult {
	if a.useNewArchitecture {
		return a.newContext.Results
	}
	return a.machine.Results()
}

// ActiveLine 返回当前活动行
func (a *SessionAdapter) ActiveLine() string {
	if a.useNewArchitecture {
		return a.replayer.ActiveLine()
	}
	return a.machine.ActiveLine()
}

// Lines 返回已提交的逻辑行
func (a *SessionAdapter) Lines() []string {
	if a.useNewArchitecture {
		return a.replayer.Lines()
	}
	return a.machine.Lines()
}

// GetNewCommittedLines 获取自上次调用以来新提交的规范化行
func (a *SessionAdapter) GetNewCommittedLines() []string {
	if a.useNewArchitecture {
		if len(a.newCommittedLines) == 0 {
			return nil
		}
		lines := make([]string, len(a.newCommittedLines))
		copy(lines, a.newCommittedLines)
		a.newCommittedLines = a.newCommittedLines[:0]
		return lines
	}
	return a.machine.GetNewCommittedLines()
}

// ClearInitResiduals 清空初始化阶段的残留数据
func (a *SessionAdapter) ClearInitResiduals() {
	if a.useNewArchitecture {
		a.replayer.Reset()
		a.newCommittedLines = a.newCommittedLines[:0]
		a.newContext.PendingLines = a.newContext.PendingLines[:0]
		a.newContext.Current = nil
		logger.Debug("SessionAdapter", "-", "[新架构] 已清空初始化残留数据")
		return
	}
	a.machine.ClearInitResiduals()
}

// MarkFailed 标记失败
func (a *SessionAdapter) MarkFailed(reason string) {
	if a.useNewArchitecture {
		a.newState = NewStateFailed
		if a.newContext.Current != nil {
			a.newContext.FailCurrentCommand(reason)
		}
		logger.Debug("SessionAdapter", "-", "[新架构] 标记失败: %s", reason)
		return
	}
	a.machine.MarkFailed(reason)
}

// GetActionData 获取动作数据
func (a *SessionAdapter) GetActionData(action Action) ActionData {
	if a.useNewArchitecture {
		return a.getActionDataNew(action)
	}
	return a.machine.GetActionData(action)
}

// getActionDataNew 新架构获取动作数据
func (a *SessionAdapter) getActionDataNew(action Action) ActionData {
	switch action {
	case ActionSendCommand:
		if a.newContext.Current != nil {
			return ActionData{
				Type:    ActionSendCommand,
				Command: a.newContext.Current.Command,
				Timeout: a.newContext.Current.CustomTimeout,
			}
		}
	case ActionSendWarmup:
		return ActionData{
			Type:    ActionSendWarmup,
			Command: "",
		}
	case ActionSendSpace:
		return ActionData{
			Type:    ActionSendSpace,
			Command: " ",
		}
	case ActionHandleError:
		return ActionData{
			Type:     ActionHandleError,
			ErrorCtx: a.newContext.PendingError,
		}
	case ActionAbortTask:
		return ActionData{
			Type: ActionAbortTask,
		}
	case ActionSkipError:
		return ActionData{
			Type: ActionSkipError,
		}
	}
	return ActionData{Type: ActionNone}
}

// GetPendingError 获取待处理的错误上下文
func (a *SessionAdapter) GetPendingError() *ErrorContext {
	if a.useNewArchitecture {
		return a.newContext.PendingError
	}
	return a.machine.GetPendingError()
}

// ResolveError 解决错误（外部决策后调用）
func (a *SessionAdapter) ResolveError(continueExec bool) {
	if a.useNewArchitecture {
		if continueExec {
			a.newState = NewStateRunning
			a.newContext.PendingError = nil
		} else {
			a.newState = NewStateFailed
		}
		logger.Debug("SessionAdapter", "-", "[新架构] 解决错误: continue=%v", continueExec)
		return
	}
	a.machine.ResolveError(continueExec)
}

// ClearError 清除错误状态
func (a *SessionAdapter) ClearError() {
	if a.useNewArchitecture {
		a.newContext.PendingError = nil
		return
	}
	a.machine.ClearError()
}

// ============================================================================
// 状态转换辅助方法
// ============================================================================

// convertNewStateToOld 将新状态转换为旧状态（用于兼容）
func (a *SessionAdapter) convertNewStateToOld(newState NewSessionState) SessionState {
	switch newState {
	case NewStateInitAwaitPrompt:
		return StateWaitInitialPrompt
	case NewStateInitAwaitWarmupPrompt:
		return StateWarmup
	case NewStateReady:
		return StateReady
	case NewStateRunning:
		return StateCollecting
	case NewStateAwaitPagerContinueAck:
		return StateHandlingPager
	case NewStateAwaitFinalPromptConfirm:
		return StateWaitingFinalPrompt
	case NewStateSuspended:
		return StateHandlingError
	case NewStateCompleted:
		return StateCompleted
	case NewStateFailed:
		return StateFailed
	default:
		return StateFailed
	}
}

// ============================================================================
// 初始化方法
// ============================================================================

// InitNewArchitecture 初始化新架构组件
// 在切换到新架构前调用
func (a *SessionAdapter) InitNewArchitecture(commands []string) {
	// 创建 Detector
	if a.detector == nil {
		a.detector = NewSessionDetector(a.matcher)
	}

	// 创建 Reducer
	if a.reducer == nil {
		a.reducer = NewSessionReducer(commands, a.matcher)
	}

	// 初始化上下文
	a.newContext = NewSessionContext(commands)
	a.newState = NewStateInitAwaitPrompt
	a.newCommittedLines = make([]string, 0)

	logger.Info("SessionAdapter", "-", "新架构组件初始化完成")
}

// ============================================================================
// 调试和监控方法
// ============================================================================

// GetArchitectureMode 返回当前架构模式字符串
func (a *SessionAdapter) GetArchitectureMode() string {
	if a.useNewArchitecture {
		return "new (Detector+Reducer+Driver)"
	}
	return "legacy (SessionMachine)"
}

// GetStats 返回适配器统计信息
func (a *SessionAdapter) GetStats() map[string]interface{} {
	stats := map[string]interface{}{
		"mode":               a.GetArchitectureMode(),
		"useNewArchitecture": a.useNewArchitecture,
		"committedLines":     len(a.newCommittedLines),
		"pendingLines":       len(a.newContext.PendingLines),
		"completedCommands":  len(a.newContext.Results),
	}

	if a.useNewArchitecture {
		stats["state"] = a.newState.String()
		stats["nextIndex"] = a.newContext.NextIndex
	} else {
		stats["state"] = a.machine.State().String()
		stats["nextIndex"] = a.machine.nextIndex
	}

	return stats
}

// ============================================================================
// 时间相关方法
// ============================================================================

// SetCurrentTimeout 设置当前命令的超时时间
func (a *SessionAdapter) SetCurrentTimeout(timeout time.Duration) {
	if a.useNewArchitecture && a.newContext.Current != nil {
		a.newContext.Current.SetCustomTimeout(timeout)
	} else if a.machine.current != nil {
		a.machine.current.SetCustomTimeout(timeout)
	}
}

// GetCurrentTimeout 获取当前命令的超时时间
func (a *SessionAdapter) GetCurrentTimeout() time.Duration {
	if a.useNewArchitecture {
		if a.newContext.Current != nil {
			return a.newContext.Current.CustomTimeout
		}
		return 0
	}
	if a.machine.current != nil {
		return a.machine.current.CustomTimeout
	}
	return 0
}
