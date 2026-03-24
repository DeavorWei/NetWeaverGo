package executor

import (
	"fmt"

	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/matcher"
)

// ============================================================================
// Session Reducer (Phase 1 重构)
// ============================================================================
// 纯函数式状态机：输入当前状态 + 事件，输出新状态 + 动作列表
// 不执行任何 I/O 操作，只负责状态决策

// SessionReducer 会话状态 Reducer
type SessionReducer struct {
	state   NewSessionState
	ctx     *SessionContext
	matcher MatcherInterface
}

// MatcherInterface 匹配器接口（用于依赖注入）
type MatcherInterface interface {
	IsPrompt(line string) bool
	IsPromptStrict(line string) bool
	IsPaginationPrompt(line string) bool
	MatchErrorRule(line string) (bool, *matcher.ErrorRule)
}

// NewSessionReducer 创建新的 Reducer
func NewSessionReducer(commands []string, matcher MatcherInterface) *SessionReducer {
	return &SessionReducer{
		state:   NewStateInitAwaitPrompt,
		ctx:     NewSessionContext(commands),
		matcher: matcher,
	}
}

// State 返回当前状态
func (r *SessionReducer) State() NewSessionState {
	return r.state
}

// Context 返回上下文
func (r *SessionReducer) Context() *SessionContext {
	return r.ctx
}

// Reduce 状态迁移核心函数
// 输入：事件
// 输出：动作列表
func (r *SessionReducer) Reduce(event SessionEvent) []SessionAction {
	// 终态不处理任何事件
	if r.state.IsTerminal() {
		return nil
	}

	switch e := event.(type) {
	case EvInitPromptStable:
		return r.handleInitPromptStable(e)

	case EvWarmupPromptSeen:
		return r.handleWarmupPromptSeen(e)

	case EvCommittedLine:
		return r.handleCommittedLine(e)

	case EvPagerSeen:
		return r.handlePagerSeen(e)

	case EvActivePromptSeen:
		return r.handleActivePromptSeen(e)

	case EvErrorMatched:
		return r.handleErrorMatched(e)

	case EvTimeout:
		return r.handleTimeout(e)

	case EvUserContinue:
		return r.handleUserContinue(e)

	case EvUserAbort:
		return r.handleUserAbort(e)

	case EvSuspendTimeout:
		return r.handleSuspendTimeout(e)

	case EvStreamClosed:
		return r.handleStreamClosed(e)

	case EvCommandPromptSeen:
		return r.handleCommandPromptSeen(e)

	default:
		logger.Debug("SessionReducer", "-", "未知事件类型: %s", event.EventType())
		return nil
	}
}

// ============================================================================
// 事件处理器
// ============================================================================

// handleInitPromptStable 处理初始提示符稳定事件
func (r *SessionReducer) handleInitPromptStable(e EvInitPromptStable) []SessionAction {
	if r.state != NewStateInitAwaitPrompt {
		return nil
	}

	r.ctx.PromptFingerprint = e.Prompt
	r.state = NewStateInitAwaitWarmupPrompt

	logger.Debug("SessionReducer", "-", "初始提示符稳定，进入预热等待: %s", e.Prompt)
	return []SessionAction{ActSendWarmup{}}
}

// handleWarmupPromptSeen 处理预热后提示符检测事件
func (r *SessionReducer) handleWarmupPromptSeen(e EvWarmupPromptSeen) []SessionAction {
	if r.state != NewStateInitAwaitWarmupPrompt {
		return nil
	}

	r.ctx.PromptFingerprint = e.Prompt
	r.ctx.InitResidualCleared = true
	r.state = NewStateReady

	logger.Debug("SessionReducer", "-", "预热完成，进入就绪状态: %s", e.Prompt)

	// 尝试发送第一条命令
	return r.trySendCommand()
}

// handleCommittedLine 处理行提交事件
func (r *SessionReducer) handleCommittedLine(e EvCommittedLine) []SessionAction {
	// 添加到待处理行
	r.ctx.AddPendingLine(e.Line)

	// 在运行相关状态下处理待处理行，避免分页后的输出滞留在 pendingLines 中
	switch r.state {
	case NewStateRunning, NewStateAwaitPagerContinueAck:
		return r.processPendingLines()
	}

	return nil
}

// handlePagerSeen 处理分页符检测事件
func (r *SessionReducer) handlePagerSeen(e EvPagerSeen) []SessionAction {
	switch r.state {
	case NewStateRunning, NewStateReady:
		if r.ctx.Current != nil {
			r.ctx.Current.IncrementPagination()
			if actions := r.checkPaginationLimit(); actions != nil {
				return actions
			}
		}
		r.state = NewStateAwaitPagerContinueAck
		logger.Debug("SessionReducer", "-", "检测到分页符，进入等待续页确认")
		return []SessionAction{ActSendPagerContinue{}}

	case NewStateAwaitPagerContinueAck:
		// 已经在等待续页确认，记录新的分页符
		if r.ctx.Current != nil {
			r.ctx.Current.IncrementPagination()
			if actions := r.checkPaginationLimit(); actions != nil {
				return actions
			}
		}
		return []SessionAction{ActSendPagerContinue{}}
	}

	return nil
}

// handleActivePromptSeen 处理活动行提示符检测事件
func (r *SessionReducer) handleActivePromptSeen(e EvActivePromptSeen) []SessionAction {
	switch r.state {
	case NewStateRunning:
		// 命令完成
		return r.completeCurrentCommand()

	case NewStateAwaitPagerContinueAck:
		// 真实设备通常只在分页结束后返回一次提示符，直接视为命令完成
		logger.Debug("SessionReducer", "-", "分页续页后检测到提示符，命令完成")
		return r.completeCurrentCommand()
	}

	return nil
}

// handleErrorMatched 处理错误匹配事件
func (r *SessionReducer) handleErrorMatched(e EvErrorMatched) []SessionAction {
	if r.state.IsTerminal() {
		return nil
	}

	// 警告级别直接放行
	if e.Rule.Severity == matcher.SeverityWarning {
		logger.Warn("SessionReducer", "-", "[告警放行] %s: %s", e.Rule.Name, e.Rule.Message)
		return nil
	}

	// 如果 ContinueOnCmdError=true，记录错误但继续执行下一条命令
	if r.ctx.ContinueOnCmdError {
		logger.Warn("SessionReducer", "-", "[错误放行] %s: %s (ContinueOnCmdError=true)",
			e.Rule.Name, e.Rule.Message)
		// 标记当前命令失败
		r.ctx.FailCurrentCommand(fmt.Sprintf("%s: %s", e.Rule.Name, e.Line))
		// 完成当前命令，继续下一条
		return r.completeCurrentCommand()
	}

	// 严重错误，进入挂起状态
	r.ctx.PendingError = &ErrorContext{
		Line:     e.Line,
		Rule:     e.Rule,
		CmdIndex: r.ctx.NextIndex - 1,
		Cmd:      r.ctx.CurrentCommand(),
	}
	r.state = NewStateSuspended

	logger.Debug("SessionReducer", "-", "检测到严重错误，进入挂起状态: %s", e.Line)
	return []SessionAction{ActRequestSuspendDecision{ErrorContext: r.ctx.PendingError}}
}

// handleTimeout 处理超时事件
func (r *SessionReducer) handleTimeout(e EvTimeout) []SessionAction {
	if r.state.IsTerminal() {
		return nil
	}

	r.ctx.FailCurrentCommand("命令执行超时")
	r.state = NewStateFailed

	logger.Debug("SessionReducer", "-", "命令超时，进入失败状态")
	return []SessionAction{ActAbortSession{Reason: "timeout"}}
}

// handleUserContinue 处理用户继续事件
func (r *SessionReducer) handleUserContinue(e EvUserContinue) []SessionAction {
	if r.state != NewStateSuspended {
		return nil
	}

	r.ctx.PendingError = nil
	r.state = NewStateRunning

	logger.Debug("SessionReducer", "-", "用户选择继续，恢复执行")
	return []SessionAction{ActResetReadTimeout{}}
}

// handleUserAbort 处理用户中止事件
func (r *SessionReducer) handleUserAbort(e EvUserAbort) []SessionAction {
	if r.state != NewStateSuspended {
		return nil
	}

	r.ctx.FailCurrentCommand("用户中止")
	r.state = NewStateFailed

	logger.Debug("SessionReducer", "-", "用户选择中止，进入失败状态")
	return []SessionAction{ActAbortSession{Reason: "user_abort"}}
}

// handleSuspendTimeout 处理挂起超时事件
func (r *SessionReducer) handleSuspendTimeout(e EvSuspendTimeout) []SessionAction {
	if r.state != NewStateSuspended {
		return nil
	}

	reason := e.Reason
	if reason == "" {
		reason = "suspend_timeout"
	}

	r.ctx.FailCurrentCommand("挂起超时: " + reason)
	r.state = NewStateFailed

	logger.Warn("SessionReducer", "-", "挂起超时，进入失败状态: %s", reason)
	return []SessionAction{ActAbortSession{Reason: "suspend_timeout"}}
}

// handleStreamClosed 处理流关闭事件
func (r *SessionReducer) handleStreamClosed(e EvStreamClosed) []SessionAction {
	if r.state.IsTerminal() {
		return nil
	}

	if r.state == NewStateCompleted {
		return nil
	}

	r.ctx.FailCurrentCommand("流意外关闭")
	r.state = NewStateFailed

	logger.Debug("SessionReducer", "-", "流关闭，进入失败状态")
	return []SessionAction{ActAbortSession{Reason: "stream_closed"}}
}

// handleCommandPromptSeen 处理命令完成后提示符检测事件
func (r *SessionReducer) handleCommandPromptSeen(e EvCommandPromptSeen) []SessionAction {
	if r.state == NewStateRunning {
		return r.completeCurrentCommand()
	}
	return nil
}

// ============================================================================
// 辅助方法
// ============================================================================

// trySendCommand 尝试发送下一条命令
func (r *SessionReducer) trySendCommand() []SessionAction {
	// 不变量检查：pendingLines 非空时不发送命令
	if r.ctx.HasPendingLines() {
		logger.Debug("SessionReducer", "-", "防串台门禁：存在 %d 行未消费输出，禁止发送新命令", len(r.ctx.PendingLines))
		return nil
	}

	// 检查是否还有命令
	if !r.ctx.HasMoreCommands() {
		r.state = NewStateCompleted
		logger.Debug("SessionReducer", "-", "所有命令执行完成")
		return nil
	}

	// 推进到下一条命令
	ctx := r.ctx.AdvanceCommand()
	if ctx == nil {
		r.state = NewStateCompleted
		return nil
	}

	// 解析命令（设置 Command 字段）
	cmdToSend, _ := parseInlineCommand(ctx.RawCommand)
	ctx.SetCommand(cmdToSend)

	r.state = NewStateRunning
	logger.Debug("SessionReducer", "-", "准备发送命令 [%d]: %s", ctx.Index, ctx.Command)

	return []SessionAction{
		ActSendCommand{Index: ctx.Index, Command: ctx.Command},
	}
}

// completeCurrentCommand 完成当前命令
func (r *SessionReducer) completeCurrentCommand() []SessionAction {
	r.ctx.CompleteCurrentCommand()
	r.state = NewStateReady

	logger.Debug("SessionReducer", "-", "命令完成，回到就绪状态")

	// 尝试发送下一条命令
	return r.trySendCommand()
}

// processPendingLines 处理待处理行
func (r *SessionReducer) processPendingLines() []SessionAction {
	var actions []SessionAction

	for r.ctx.HasPendingLines() {
		line := r.ctx.PendingLines[0]
		r.ctx.PendingLines = r.ctx.PendingLines[1:]

		// 添加到当前命令的规范化行
		if r.ctx.Current != nil {
			r.ctx.Current.AddNormalizedLine(line)
		}

		// 检查错误规则
		if matched, rule := r.matcher.MatchErrorRule(line); matched {
			if rule.Severity == matcher.SeverityWarning {
				logger.Warn("SessionReducer", "-", "[告警放行] %s: %s", rule.Name, rule.Message)
				continue
			}

			r.ctx.PendingError = &ErrorContext{
				Line:     line,
				Rule:     rule,
				CmdIndex: r.ctx.NextIndex - 1,
				Cmd:      r.ctx.CurrentCommand(),
			}
			r.state = NewStateSuspended
			actions = append(actions, ActRequestSuspendDecision{ErrorContext: r.ctx.PendingError})
			return actions
		}

		// 检查分页符
		if r.matcher.IsPaginationPrompt(line) {
			if r.ctx.Current != nil {
				r.ctx.Current.IncrementPagination()
				if limitActions := r.checkPaginationLimit(); limitActions != nil {
					return limitActions
				}
			}
			r.state = NewStateAwaitPagerContinueAck
			actions = append(actions, ActSendPagerContinue{})
			return actions
		}

		// 检查提示符
		if r.matcher.IsPromptStrict(line) {
			return r.completeCurrentCommand()
		}
	}

	return actions
}

func (r *SessionReducer) checkPaginationLimit() []SessionAction {
	if r.ctx == nil || r.ctx.Current == nil {
		return nil
	}

	limit := r.ctx.MaxPaginationCount
	if limit <= 0 {
		return nil
	}

	if r.ctx.Current.PaginationCount <= limit {
		return nil
	}

	reason := "pagination_limit_exceeded"
	r.ctx.FailCurrentCommand("分页次数超限")
	r.state = NewStateFailed

	logger.Warn("SessionReducer", "-", "分页次数超限: current=%d limit=%d", r.ctx.Current.PaginationCount, limit)
	return []SessionAction{ActAbortSession{Reason: reason}}
}
