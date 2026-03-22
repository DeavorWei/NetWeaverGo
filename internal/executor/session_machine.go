package executor

import (
	"strings"
	"time"

	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/matcher"
	"github.com/NetWeaverGo/core/internal/terminal"
)

// ErrorContext 错误上下文
// 用于传递错误检测信息给外部决策处理器
type ErrorContext struct {
	// Line 命中错误规则的行
	Line string
	// Rule 命中的错误规则
	Rule *matcher.ErrorRule
	// CmdIndex 当前命令索引
	CmdIndex int
	// Cmd 当前命令内容
	Cmd string
}

// SessionMachine 会话状态机
// 负责消费原始 chunk，产出规范化事件，驱动命令执行流程
type SessionMachine struct {
	// state 当前状态
	state SessionState

	// replayer 终端重放器
	replayer *terminal.Replayer

	// current 当前命令上下文
	current *CommandContext

	// queue 命令队列
	queue []string

	// nextIndex 下一条命令索引
	nextIndex int

	// promptHint 提示符提示（用于判断）
	promptHint string

	// matcher 流匹配器
	matcher *matcher.StreamMatcher

	// results 已完成的命令结果
	results []*CommandResult

	// warmupSent 是否已发送预热空行
	warmupSent bool

	// pendingLines 待处理的逻辑行
	pendingLines []string

	// newCommittedLines 新提交的规范化行（用于 Detail 日志同步）
	newCommittedLines []string

	// pendingError 待处理的错误上下文
	pendingError *ErrorContext

	// errorDecided 错误决策是否已做出
	errorDecided bool

	// errorContinue 错误决策结果：继续执行
	errorContinue bool

	// afterPager 分页后标志，用于强制使用严格提示符判定
	afterPager bool
}

// NewSessionMachine 创建新的会话状态机
func NewSessionMachine(width int, commands []string, m *matcher.StreamMatcher) *SessionMachine {
	return &SessionMachine{
		state:             StateWaitInitialPrompt,
		replayer:          terminal.NewReplayer(width),
		queue:             commands,
		nextIndex:         0,
		matcher:           m,
		results:           make([]*CommandResult, 0),
		pendingLines:      make([]string, 0),
		newCommittedLines: make([]string, 0),
	}
}

// State 返回当前状态
func (m *SessionMachine) State() SessionState {
	return m.state
}

// CurrentCommand 返回当前命令上下文
func (m *SessionMachine) CurrentCommand() *CommandContext {
	return m.current
}

// Results 返回所有已完成的命令结果
func (m *SessionMachine) Results() []*CommandResult {
	return m.results
}

// ActiveLine 返回当前活动行
func (m *SessionMachine) ActiveLine() string {
	return m.replayer.ActiveLine()
}

// Lines 返回已提交的逻辑行
func (m *SessionMachine) Lines() []string {
	return m.replayer.Lines()
}

// GetNewCommittedLines 获取自上次调用以来新提交的规范化行
// 用于同步写入 Detail 日志
func (m *SessionMachine) GetNewCommittedLines() []string {
	if len(m.newCommittedLines) == 0 {
		return nil
	}
	lines := make([]string, len(m.newCommittedLines))
	copy(lines, m.newCommittedLines)
	m.newCommittedLines = m.newCommittedLines[:0]
	return lines
}

// ClearInitResiduals 清空初始化阶段的残留数据
// 在进入 Ready 或发送第一条命令前调用，避免登录噪声污染第一条命令
func (m *SessionMachine) ClearInitResiduals() {
	// 关键修复：先刷新 Detail 日志，避免丢失初始化阶段的输出
	// 这一步很重要，因为 newCommittedLines 可能包含登录信息
	if len(m.newCommittedLines) > 0 {
		logger.Debug("SessionMachine", "-", "ClearInitResiduals: 保留 %d 行已提交内容到 Detail 日志", len(m.newCommittedLines))
		// 注意：newCommittedLines 会在下一次 GetNewCommittedLines 调用时被消费
		// 这里不清空，让 stream_engine 正常写入
	}

	// 清空待处理行
	m.pendingLines = m.pendingLines[:0]

	// 清空 replayer 中的已提交行和活动行
	m.replayer.Reset()

	// 清空当前命令上下文
	m.current = nil

	// 清空错误上下文
	m.pendingError = nil
	m.errorDecided = false

	logger.Debug("SessionMachine", "-", "已清空初始化残留数据")
}

// Feed 消费原始 chunk，更新状态机
// 返回需要执行的动作
func (m *SessionMachine) Feed(chunk string) []Action {
	actions := make([]Action, 0)

	// 使用 replayer 处理 chunk
	events := m.replayer.Process(chunk)

	// 收集新提交的行
	for _, event := range events {
		if event.Type == terminal.EventLineCommitted {
			m.pendingLines = append(m.pendingLines, event.Line)
			// 同时收集到 newCommittedLines 用于 Detail 日志同步
			m.newCommittedLines = append(m.newCommittedLines, event.Line)
		}
	}

	// 根据当前状态处理
	switch m.state {
	case StateWaitInitialPrompt:
		actions = m.handleWaitInitialPrompt()

	case StateWarmup:
		actions = m.handleWarmup()

	case StateReady:
		actions = m.handleReady()

	case StateCollecting:
		actions = m.handleCollecting()

	case StateHandlingPager:
		actions = m.handlePager()

	case StateWaitingFinalPrompt:
		actions = m.handleWaitingFinalPrompt()

	case StateHandlingError:
		actions = m.handleHandlingError()

	case StateCompleted, StateFailed:
		// 终态，不再处理
	}

	return actions
}

// handleWaitInitialPrompt 处理等待初始提示符状态
func (m *SessionMachine) handleWaitInitialPrompt() []Action {
	// 检查活动行或最新提交行是否为提示符
	activeLine := m.replayer.ActiveLine()
	lines := m.replayer.Lines()

	// 关键修复：初始化阶段使用严格提示符检测
	// 防止混合行（如 "The current login time...<S1>"）被误判为提示符
	// 先检查活动行
	if m.matcher.IsPromptStrict(activeLine) {
		m.promptHint = extractPromptHint(activeLine)
		m.state = StateWarmup
		logger.Debug("SessionMachine", "-", "严格模式检测到初始提示符（活动行），进入预热状态: %s", activeLine)
		return []Action{ActionSendWarmup}
	}

	// 再检查最新提交行
	if len(lines) > 0 {
		lastLine := lines[len(lines)-1]
		if m.matcher.IsPromptStrict(lastLine) {
			m.promptHint = extractPromptHint(lastLine)
			m.state = StateWarmup
			logger.Debug("SessionMachine", "-", "严格模式检测到初始提示符（最后一行），进入预热状态: %s", lastLine)
			return []Action{ActionSendWarmup}
		}
	}

	return nil
}

// handleWarmup 处理预热状态
func (m *SessionMachine) handleWarmup() []Action {
	// 检查是否收到预热后的提示符
	activeLine := m.replayer.ActiveLine()
	lines := m.replayer.Lines()

	// 关键修复：预热阶段使用严格提示符检测
	// 防止混合行被误判为提示符
	// 先检查活动行
	if m.matcher.IsPromptStrict(activeLine) {
		m.state = StateReady
		logger.Debug("SessionMachine", "-", "预热完成（严格模式），进入就绪状态: %s", activeLine)
		// 预热完成后立即触发发送第一条命令
		return m.handleReady()
	}

	// 再检查最新提交行
	if len(lines) > 0 {
		lastLine := lines[len(lines)-1]
		if m.matcher.IsPromptStrict(lastLine) {
			m.state = StateReady
			logger.Debug("SessionMachine", "-", "预热完成（严格模式），进入就绪状态: %s", lastLine)
			// 预热完成后立即触发发送第一条命令
			return m.handleReady()
		}
	}

	return nil
}

// handleReady 处理就绪状态
func (m *SessionMachine) handleReady() []Action {
	// 首先检查活动行是否为分页符
	// 这处理了分页符在提示符之后到达的情况（跨 chunk 时序竞争）
	activeLine := m.replayer.ActiveLine()
	if m.matcher.IsPaginationPrompt(activeLine) {
		m.state = StateHandlingPager
		logger.Debug("SessionMachine", "-", "就绪状态检测到未处理的分页符，进入分页处理")
		return []Action{ActionSendSpace}
	}

	// 【方案四+修复】防串台门禁检查
	// 满足以下任一条件时，不允许发送下一条命令：
	// 1. 当前命令分页待续
	// 2. 待处理行非空（无论是否有分页符）
	// 3. 活动行非空且不是提示符（可能还在输出）

	// 检查当前命令是否分页待续
	if m.current != nil && m.current.IsPaginationPending() {
		logger.Debug("SessionMachine", "-", "防串台门禁：当前命令分页待续，不允许发送新命令")
		return nil
	}

	// 【核心修复】硬门禁：pendingLines 非空时禁止发送新命令
	// 这防止了"未消费输出"导致的命令队列抢跑问题
	if len(m.pendingLines) > 0 {
		// 先检查是否有分页符，如果有则进入分页处理
		for _, line := range m.pendingLines {
			if m.matcher.IsPaginationPrompt(line) {
				m.state = StateHandlingPager
				logger.Debug("SessionMachine", "-", "防串台门禁：待处理行中有分页符，进入分页处理")
				return []Action{ActionSendSpace}
			}
		}
		// 没有分页符，但仍有未消费输出，等待处理
		logger.Debug("SessionMachine", "-", "防串台门禁：存在 %d 行未消费输出，禁止发送新命令", len(m.pendingLines))
		return nil
	}

	// 检查活动行是否非提示符（等待更多数据）
	if activeLine != "" && !m.matcher.IsPromptStrict(activeLine) {
		logger.Debug("SessionMachine", "-", "防串台门禁：活动行非提示符，等待更多数据: %s", truncateStringDebug(activeLine, 50))
		return nil
	}

	// 检查是否还有命令要执行
	if m.nextIndex >= len(m.queue) {
		m.state = StateCompleted
		logger.Debug("SessionMachine", "-", "所有命令执行完成")
		return nil
	}

	// 关键修复：在发送命令前清理 Replayer 中残留的提示符
	// 避免提示符污染命令输出（如 <S1>PHY: Physical 的情况）
	if activeLine != "" && m.matcher.IsPrompt(activeLine) {
		logger.Debug("SessionMachine", "-", "发送命令前清理残留提示符: %s", activeLine)
		m.replayer.Reset()
	}

	// 准备发送下一条命令
	rawCmd := m.queue[m.nextIndex]
	m.current = NewCommandContext(m.nextIndex, rawCmd)

	// 解析内联注释
	cmdToSend, customTimeout := parseInlineCommand(rawCmd)
	m.current.SetCommand(cmdToSend)
	if customTimeout > 0 {
		m.current.SetCustomTimeout(customTimeout)
	}

	m.nextIndex++
	// StateSendCommand 是瞬时状态，直接转换到 StateCollecting
	// 外部调用者会在收到 ActionSendCommand 后发送命令
	m.state = StateCollecting

	logger.Debug("SessionMachine", "-", "准备发送命令 [%d]: %s", m.current.Index, cmdToSend)
	return []Action{ActionSendCommand}
}

// handleSendCommand 处理发送命令状态（瞬时状态，应该立即转换）
func (m *SessionMachine) handleSendCommand() []Action {
	// 这是一个瞬时状态，应该立即转换到 StateCollecting
	m.state = StateCollecting
	return nil
}

// handleCollecting 处理收集输出状态
func (m *SessionMachine) handleCollecting() []Action {
	actions := make([]Action, 0)

	// 处理待处理的行
	for len(m.pendingLines) > 0 {
		line := m.pendingLines[0]
		m.pendingLines = m.pendingLines[1:]

		// 添加到当前命令的规范化行
		if m.current != nil {
			m.current.AddNormalizedLine(line)
		}

		// 检查是否命中错误规则
		if matched, rule := m.matcher.MatchErrorRule(line); matched {
			// 构建错误上下文
			cmdIndex := -1
			cmd := ""
			if m.current != nil {
				cmdIndex = m.current.Index
				cmd = m.current.RawCommand
			}
			m.pendingError = &ErrorContext{
				Line:     line,
				Rule:     rule,
				CmdIndex: cmdIndex,
				Cmd:      cmd,
			}

			// 如果是警告级别，直接放行
			if rule.Severity == matcher.SeverityWarning {
				logger.Warn("SessionMachine", "-", "[告警放行] %s: %s", rule.Name, rule.Message)
				m.pendingError = nil // 清除，不需要外部决策
				continue
			}

			// 严重错误，进入错误处理状态
			m.state = StateHandlingError
			logger.Debug("SessionMachine", "-", "检测到严重错误，进入错误处理状态: %s", line)
			actions = append(actions, ActionHandleError)
			return actions
		}

		// 检查是否为分页符（优先于提示符检测）
		// 分页符表示输出未完成，必须优先处理
		if m.matcher.IsPaginationPrompt(line) {
			m.state = StateHandlingPager
			if m.current != nil {
				m.current.IncrementPagination()
			}
			logger.Debug("SessionMachine", "-", "检测到分页符，进入分页处理状态")
			actions = append(actions, ActionSendSpace)
			return actions
		}

		// 检查是否为提示符
		// 关键修复：检测顺序很重要！
		// 1. 首先检查是否在分页中（跨 chunk 场景）
		// 2. 然后检查当前 chunk 是否有分页符
		// 3. 最后才完成命令
		// 【方案四】分页后使用严格提示符判定
		// 【修复】始终使用严格提示符判定，避免旧 prompt 误判
		isPrompt := m.matcher.IsPromptStrict(line)
		if !isPrompt && m.afterPager {
			// 分页后也尝试非严格匹配
			isPrompt = m.matcher.IsPrompt(line)
		}
		if isPrompt {
			// 关键修复1：首先检查是否正在分页中
			// 这处理了分页符在前一个 chunk，提示符在当前 chunk 的情况
			if m.current != nil && m.current.IsPaginationPending() {
				// 检查剩余的 pendingLines 是否包含新的分页符
				hasNewPager := false
				for _, remainingLine := range m.pendingLines {
					if m.matcher.IsPaginationPrompt(remainingLine) {
						hasNewPager = true
						break
					}
				}

				// 如果还有新的分页符，继续处理
				if hasNewPager {
					logger.Debug("SessionMachine", "-", "分页中检测到提示符，但剩余行有新分页符，继续处理")
					continue
				}

				// 检查活动行是否为新的分页符
				activeLine := m.replayer.ActiveLine()
				if m.matcher.IsPaginationPrompt(activeLine) {
					m.state = StateHandlingPager
					m.current.IncrementPagination()
					logger.Debug("SessionMachine", "-", "分页中检测到提示符，但活动行是新分页符，进入分页处理")
					actions = append(actions, ActionSendSpace)
					return actions
				}

				// 没有新的分页符，进入等待确认状态
				m.state = StateWaitingFinalPrompt
				logger.Debug("SessionMachine", "-", "分页后检测到提示符，进入等待确认状态")
				return nil
			}

			// 检查剩余的 pendingLines 是否包含分页符
			hasPager := false
			for _, remainingLine := range m.pendingLines {
				if m.matcher.IsPaginationPrompt(remainingLine) {
					hasPager = true
					break
				}
			}

			// 如果剩余行中有分页符，优先处理分页符
			if hasPager {
				// 不完成命令，继续处理剩余行
				logger.Debug("SessionMachine", "-", "检测到提示符，但剩余行中有分页符，继续处理")
				continue
			}

			// 检查活动行是否为分页符
			activeLine := m.replayer.ActiveLine()
			if m.matcher.IsPaginationPrompt(activeLine) {
				m.state = StateHandlingPager
				if m.current != nil {
					m.current.IncrementPagination()
				}
				logger.Debug("SessionMachine", "-", "检测到提示符，但活动行是分页符，进入分页处理状态")
				actions = append(actions, ActionSendSpace)
				return actions
			}

			// 确认没有分页符，命令完成
			m.completeCurrentCommand()
			m.state = StateReady
			logger.Debug("SessionMachine", "-", "检测到提示符，命令完成，回到就绪状态")
			return nil
		}
	}

	// 检查活动行是否为分页符（优先于提示符检测）
	// 分页符表示输出未完成，必须优先处理
	activeLine := m.replayer.ActiveLine()
	if m.matcher.IsPaginationPrompt(activeLine) {
		m.state = StateHandlingPager
		if m.current != nil {
			m.current.IncrementPagination()
		}
		logger.Debug("SessionMachine", "-", "活动行检测到分页符，进入分页处理状态")
		actions = append(actions, ActionSendSpace)
		return actions
	}

	// 检查活动行是否为提示符
	// 注意：活动行提示符检测时直接完成命令，因为如果没有新数据，
	// 状态机不会被再次驱动，WaitingFinalPrompt 状态会卡住
	// 【方案四】分页后使用严格提示符判定
	isPromptActive := false
	if m.afterPager {
		isPromptActive = m.matcher.IsPromptStrict(activeLine)
	} else {
		isPromptActive = m.matcher.IsPrompt(activeLine)
	}
	if isPromptActive {
		// 清除分页后标志
		m.afterPager = false
		// 方案1+3：如果正在分页中，需要进入等待确认状态
		if m.current != nil && m.current.IsPaginationPending() {
			m.state = StateWaitingFinalPrompt
			logger.Debug("SessionMachine", "-", "分页后活动行检测到提示符，进入等待确认状态")
			return nil
		}

		m.completeCurrentCommand()
		m.state = StateReady
		logger.Debug("SessionMachine", "-", "活动行检测到提示符，命令完成，回到就绪状态")
		return nil
	}

	return actions
}

// handleHandlingError 处理错误处理状态
func (m *SessionMachine) handleHandlingError() []Action {
	// 等待外部决策
	if !m.errorDecided {
		return nil // 继续等待
	}

	// 决策已做出
	if m.errorContinue {
		// 继续执行
		logger.Debug("SessionMachine", "-", "错误已放行，继续执行")
		m.ClearError()
		m.state = StateCollecting
		return nil
	}

	// 中止执行
	logger.Debug("SessionMachine", "-", "错误导致中止执行")
	m.state = StateFailed
	return []Action{ActionAbortTask}
}

// handlePager 处理分页状态
func (m *SessionMachine) handlePager() []Action {
	// 设置分页后标志，后续必须使用严格提示符判定
	m.afterPager = true
	// 发送空格后，回到收集状态
	m.state = StateCollecting
	logger.Debug("SessionMachine", "-", "分页处理完成，回到收集状态（启用严格提示符判定）")
	return nil
}

// handleWaitingFinalPrompt 处理等待最终提示符状态
// 方案3：分页后进入此状态，等待确认命令真正结束
func (m *SessionMachine) handleWaitingFinalPrompt() []Action {
	// 处理待处理的行
	for len(m.pendingLines) > 0 {
		line := m.pendingLines[0]
		m.pendingLines = m.pendingLines[1:]

		// 如果还有更多行，说明之前的提示符是误判
		// 回到收集状态继续处理
		if m.current != nil {
			m.current.AddNormalizedLine(line)
		}

		// 如果又遇到分页符，需要继续翻页
		if m.matcher.IsPaginationPrompt(line) {
			m.state = StateHandlingPager
			if m.current != nil {
				m.current.IncrementPagination()
			}
			logger.Debug("SessionMachine", "-", "等待确认时检测到分页符，进入分页处理状态")
			return []Action{ActionSendSpace}
		}

		// 如果遇到提示符，确认命令真正完成
		if m.matcher.IsPrompt(line) {
			if m.current != nil {
				m.current.ClearPaginationPending()
			}
			m.completeCurrentCommand()
			m.state = StateReady
			logger.Debug("SessionMachine", "-", "等待确认后检测到提示符，命令完成")
			return nil
		}
	}

	// 检查活动行
	activeLine := m.replayer.ActiveLine()

	// 优先检查分页符
	if m.matcher.IsPaginationPrompt(activeLine) {
		m.state = StateHandlingPager
		if m.current != nil {
			m.current.IncrementPagination()
		}
		logger.Debug("SessionMachine", "-", "等待确认时活动行检测到分页符，进入分页处理状态")
		return []Action{ActionSendSpace}
	}

	if m.matcher.IsPrompt(activeLine) {
		// 确认命令完成
		if m.current != nil {
			m.current.ClearPaginationPending()
		}
		m.completeCurrentCommand()
		m.state = StateReady
		logger.Debug("SessionMachine", "-", "等待确认后活动行检测到提示符，命令完成")
		return nil
	}

	// 如果活动行不为空，可能还在输出
	if activeLine != "" {
		m.state = StateCollecting
		logger.Debug("SessionMachine", "-", "误判提示符，回到收集状态")
	}

	return nil
}

// completeCurrentCommand 完成当前命令
func (m *SessionMachine) completeCurrentCommand() {
	if m.current != nil {
		// 【方案五】收紧活动行刷出策略
		// 只在以下条件下刷出活动行：
		// 1. 活动行非空
		// 2. 不是提示符
		// 3. 不是分页符
		// 4. 不是疑似半截行（长度过短且无完整语义）
		activeLine := m.replayer.ActiveLine()
		if activeLine != "" && !m.matcher.IsPrompt(activeLine) && !m.matcher.IsPaginationPrompt(activeLine) {
			// 半截行检测：如果活动行过短（< 3 字符）且不以常见结尾字符结束，可能是半截行
			isSuspectedHalfLine := len(activeLine) < 3 && !strings.HasSuffix(activeLine, ".") && !strings.HasSuffix(activeLine, ":") && !strings.HasSuffix(activeLine, "]")

			if !isSuspectedHalfLine {
				m.current.AddNormalizedLine(activeLine)
				m.newCommittedLines = append(m.newCommittedLines, activeLine)
				logger.Debug("SessionMachine", "-", "命令完成时处理活动行: %s", truncateStringDebug(activeLine, 100))
			} else {
				logger.Debug("SessionMachine", "-", "跳过疑似半截活动行: %s", truncateStringDebug(activeLine, 100))
			}
		}

		m.current.MarkCompleted()
		m.results = append(m.results, m.current.ToResult())
		logger.Debug("SessionMachine", "-", "命令 [%d] 完成，耗时 %v", m.current.Index, m.current.Duration())
	}
	m.current = nil
}

// MarkFailed 标记失败
func (m *SessionMachine) MarkFailed(errMsg string) {
	if m.current != nil {
		m.current.MarkFailed(errMsg)
		m.results = append(m.results, m.current.ToResult())
	} else {
		// 如果没有当前命令，创建一个失败的结果
		result := &CommandResult{
			Index:        m.nextIndex,
			Success:      false,
			ErrorMessage: errMsg,
		}
		m.results = append(m.results, result)
	}
	m.state = StateFailed
}

// Reset 重置状态机
func (m *SessionMachine) Reset() {
	m.state = StateWaitInitialPrompt
	m.replayer.Reset()
	m.current = nil
	m.nextIndex = 0
	m.results = m.results[:0]
	m.warmupSent = false
	m.pendingLines = m.pendingLines[:0]
	m.ClearError()
}

// Action 动作类型
type Action int

const (
	// ActionNone 无动作
	ActionNone Action = iota
	// ActionSendCommand 发送命令
	ActionSendCommand
	// ActionSendSpace 发送空格（分页）
	ActionSendSpace
	// ActionSendWarmup 发送预热空行
	ActionSendWarmup
	// ActionHandleError 处理错误（需要外部决策）
	ActionHandleError
	// ActionAbortTask 中止执行
	ActionAbortTask
	// ActionSkipError 跳过错误继续执行
	ActionSkipError
)

// ActionData 动作数据
type ActionData struct {
	Type    Action
	Command string
	Timeout time.Duration
	// ErrorCtx 错误上下文（仅 ActionHandleError 时有效）
	ErrorCtx *ErrorContext
}

// GetActionData 获取动作数据
func (m *SessionMachine) GetActionData(action Action) ActionData {
	switch action {
	case ActionSendCommand:
		if m.current != nil {
			return ActionData{
				Type:    ActionSendCommand,
				Command: m.current.Command,
				Timeout: m.current.CustomTimeout,
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
			ErrorCtx: m.pendingError,
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
func (m *SessionMachine) GetPendingError() *ErrorContext {
	return m.pendingError
}

// ResolveError 解决错误（外部决策后调用）
// continueExec=true 表示继续执行，false 表示中止
func (m *SessionMachine) ResolveError(continueExec bool) {
	m.errorDecided = true
	m.errorContinue = continueExec
}

// ClearError 清除错误状态
func (m *SessionMachine) ClearError() {
	m.pendingError = nil
	m.errorDecided = false
	m.errorContinue = false
}

// parseInlineCommand 解析内联命令注释
// 返回实际命令和自定义超时
func parseInlineCommand(rawCmd string) (string, time.Duration) {
	cmdToSend := rawCmd
	var customTimeout time.Duration

	if idx := strings.Index(rawCmd, "// nw-timeout="); idx != -1 {
		cmdToSend = strings.TrimSpace(rawCmd[:idx])
		timeoutStr := strings.TrimSpace(rawCmd[idx+len("// nw-timeout="):])
		if pd, err := time.ParseDuration(timeoutStr); err == nil {
			customTimeout = pd
		}
	}

	return cmdToSend, customTimeout
}

// extractPromptHint 从行中提取提示符提示
func extractPromptHint(line string) string {
	// 简单实现：返回行的最后几个字符作为提示符提示
	line = strings.TrimSpace(line)
	if len(line) > 20 {
		return line[len(line)-20:]
	}
	return line
}

// truncateStringDebug 截断字符串用于调试日志
func truncateStringDebug(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
