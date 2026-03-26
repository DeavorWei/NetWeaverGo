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

// ReduceBatch 状态迁移核心函数
// 输入：事件
// 输出：批次（新接口）
func (r *SessionReducer) ReduceBatch(event SessionEvent) *TransitionBatch {
	batch := NewTransitionBatch()

	// 终态不处理任何事件
	if r.state.IsTerminal() {
		return batch
	}

	var effects []SessionEffect
	switch e := event.(type) {
	case EvInitPromptStable:
		effects = r.handleInitPromptStable(e)

	case EvWarmupPromptSeen:
		effects = r.handleWarmupPromptSeen(e)

	case EvCommittedLine:
		effects = r.handleCommittedLine(e)

	case EvPagerSeen:
		effects = r.handlePagerSeen(e)

	case EvActivePromptSeen:
		effects = r.handleActivePromptSeen(e)

	case EvErrorMatched:
		effects = r.handleErrorMatched(e)

	case EvTimeout:
		effects = r.handleTimeout(e)

	case EvUserContinue:
		effects = r.handleUserContinue(e)

	case EvUserAbort:
		effects = r.handleUserAbort(e)

	case EvSuspendTimeout:
		effects = r.handleSuspendTimeout(e)

	case EvStreamClosed:
		effects = r.handleStreamClosed(e)

	case EvCommandPromptSeen:
		effects = r.handleCommandPromptSeen(e)

	default:
		logger.Debug("SessionReducer", "-", "未知事件类型: %s", event.EventType())
	}

	batch.AppendEffects(effects...)
	return batch
}

// ============================================================================
// 事件处理器
// ============================================================================

// handleInitPromptStable 处理初始提示符稳定事件
func (r *SessionReducer) handleInitPromptStable(e EvInitPromptStable) []SessionEffect {
	if r.state != NewStateInitAwaitPrompt {
		return nil
	}

	r.ctx.PromptFingerprint = e.Prompt
	r.state = NewStateInitAwaitWarmupPrompt

	logger.Debug("SessionReducer", "-", "初始提示符稳定，进入预热等待: %s", e.Prompt)
	return []SessionEffect{ActSendWarmup{}}
}

// handleWarmupPromptSeen 处理预热后提示符检测事件
func (r *SessionReducer) handleWarmupPromptSeen(e EvWarmupPromptSeen) []SessionEffect {
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
func (r *SessionReducer) handleCommittedLine(e EvCommittedLine) []SessionEffect {
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
func (r *SessionReducer) handlePagerSeen(e EvPagerSeen) []SessionEffect {
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
		return []SessionEffect{ActSendPagerContinue{}}

	case NewStateAwaitPagerContinueAck:
		// 已经在等待续页确认，记录新的分页符
		if r.ctx.Current != nil {
			r.ctx.Current.IncrementPagination()
			if actions := r.checkPaginationLimit(); actions != nil {
				return actions
			}
		}
		return []SessionEffect{ActSendPagerContinue{}}
	}

	return nil
}

// handleActivePromptSeen 处理活动行提示符检测事件
func (r *SessionReducer) handleActivePromptSeen(e EvActivePromptSeen) []SessionEffect {
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
func (r *SessionReducer) handleErrorMatched(e EvErrorMatched) []SessionEffect {
	if r.state.IsTerminal() {
		return nil
	}

	if r.ctx.Current != nil {
		r.ctx.Current.AddNormalizedLine(e.Line)
	}

	// 警告级别直接放行
	if e.Rule.Severity == matcher.SeverityWarning {
		logger.Warn("SessionReducer", "-", "[告警放行] %s: %s", e.Rule.Name, e.Rule.Message)
		return nil
	}

	// 如果 ContinueOnCmdError=true，记录错误但继续执行下一条命令
	if r.ctx.ContinueOnCmdError {
		if r.ctx.Current != nil && r.ctx.Current.HasError() {
			logger.Debug("SessionReducer", "-", "当前命令已标记失败，忽略同一错误块的后续错误行: %s", e.Line)
			return nil
		}
		logger.Warn("SessionReducer", "-", "[错误放行] %s: %s (ContinueOnCmdError=true)",
			e.Rule.Name, e.Rule.Message)
		// 标记当前命令失败，并等待设备返回提示符后再推进下一条命令。
		r.ctx.MarkCurrentCommandFailed(fmt.Sprintf("%s: %s", e.Rule.Name, e.Line))
		return nil
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
	return []SessionEffect{ActRequestSuspendDecision{ErrorContext: r.ctx.PendingError}}
}

// handleTimeout 处理超时事件
func (r *SessionReducer) handleTimeout(e EvTimeout) []SessionEffect {
	if r.state.IsTerminal() {
		return nil
	}

	effects := r.failCurrentCommand("命令执行超时")
	r.state = NewStateFailed

	logger.Debug("SessionReducer", "-", "命令超时，进入失败状态")
	effects = append(effects, ActAbortSession{Reason: "timeout"})
	return effects
}

// handleUserContinue 处理用户继续事件
func (r *SessionReducer) handleUserContinue(e EvUserContinue) []SessionEffect {
	if r.state != NewStateSuspended {
		return nil
	}

	r.ctx.PendingError = nil
	r.state = NewStateRunning

	logger.Debug("SessionReducer", "-", "用户选择继续，恢复执行")
	return []SessionEffect{ActResetReadTimeout{}}
}

// handleUserAbort 处理用户中止事件
func (r *SessionReducer) handleUserAbort(e EvUserAbort) []SessionEffect {
	if r.state != NewStateSuspended {
		return nil
	}

	effects := r.failCurrentCommand("用户中止")
	r.state = NewStateFailed

	logger.Debug("SessionReducer", "-", "用户选择中止，进入失败状态")
	effects = append(effects, ActAbortSession{Reason: "user_abort"})
	return effects
}

// handleSuspendTimeout 处理挂起超时事件
func (r *SessionReducer) handleSuspendTimeout(e EvSuspendTimeout) []SessionEffect {
	if r.state != NewStateSuspended {
		return nil
	}

	reason := e.Reason
	if reason == "" {
		reason = "suspend_timeout"
	}

	effects := r.failCurrentCommand("挂起超时: " + reason)
	r.state = NewStateFailed

	logger.Warn("SessionReducer", "-", "挂起超时，进入失败状态: %s", reason)
	effects = append(effects, ActAbortSession{Reason: "suspend_timeout"})
	return effects
}

// handleStreamClosed 处理流关闭事件
func (r *SessionReducer) handleStreamClosed(e EvStreamClosed) []SessionEffect {
	if r.state.IsTerminal() {
		return nil
	}

	if r.state == NewStateCompleted {
		return nil
	}

	effects := r.failCurrentCommand("流意外关闭")
	r.state = NewStateFailed

	logger.Debug("SessionReducer", "-", "流关闭，进入失败状态")
	effects = append(effects, ActAbortSession{Reason: "stream_closed"})
	return effects
}

// handleCommandPromptSeen 处理命令完成后提示符检测事件
func (r *SessionReducer) handleCommandPromptSeen(e EvCommandPromptSeen) []SessionEffect {
	if r.state == NewStateRunning {
		return r.completeCurrentCommand()
	}
	return nil
}

// ============================================================================
// 辅助方法
// ============================================================================

// trySendCommand 尝试发送下一条命令
func (r *SessionReducer) trySendCommand() []SessionEffect {
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

	return []SessionEffect{
		ActSendCommand{Index: ctx.Index, Command: ctx.Command},
	}
}

// completeCurrentCommand 完成当前命令
func (r *SessionReducer) completeCurrentCommand() []SessionEffect {
	var effects []SessionEffect

	if doneEffect, ok := r.buildCommandDoneEffectFromCurrent(); ok {
		effects = append(effects, doneEffect)
	}
	r.ctx.CompleteCurrentCommand()
	r.state = NewStateReady

	logger.Debug("SessionReducer", "-", "命令完成，回到就绪状态")

	// 先发命令完成事件，再尝试发送下一条命令，确保顺序为 completed -> next dispatched。
	effects = append(effects, r.trySendCommand()...)
	return effects
}

func (r *SessionReducer) failCurrentCommand(reason string) []SessionEffect {
	doneEffect, ok := r.buildCommandDoneEffectFromCurrent()
	r.ctx.FailCurrentCommand(reason)
	if !ok || r.ctx == nil || r.ctx.Current == nil {
		return nil
	}
	doneEffect.Success = false
	doneEffect.Duration = r.ctx.Current.Duration()
	doneEffect.ErrorMessage = r.ctx.Current.ErrorMessage
	return []SessionEffect{doneEffect}
}

func (r *SessionReducer) buildCommandDoneEffectFromCurrent() (ActEmitCommandDone, bool) {
	if r.ctx == nil || r.ctx.Current == nil {
		return ActEmitCommandDone{}, false
	}
	current := r.ctx.Current
	command := current.Command
	if command == "" {
		parsed, _ := parseInlineCommand(current.RawCommand)
		command = parsed
	}
	return ActEmitCommandDone{
		Index:        current.Index,
		Command:      command,
		Success:      !current.HasError(),
		Duration:     current.Duration(),
		ErrorMessage: current.ErrorMessage,
	}, true
}

// processPendingLines 处理待处理行
func (r *SessionReducer) processPendingLines() []SessionEffect {
	var actions []SessionEffect

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

func (r *SessionReducer) checkPaginationLimit() []SessionEffect {
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
	effects := r.failCurrentCommand("分页次数超限")
	r.state = NewStateFailed

	logger.Warn("SessionReducer", "-", "分页次数超限: current=%d limit=%d", r.ctx.Current.PaginationCount, limit)
	effects = append(effects, ActAbortSession{Reason: reason})
	return effects
}
