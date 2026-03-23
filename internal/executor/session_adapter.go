package executor

import (
	"time"

	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/matcher"
	"github.com/NetWeaverGo/core/internal/terminal"
)

// SessionAdapter 会话适配器
// 统一封装 Replayer + Detector + Reducer 的会话处理流程。
type SessionAdapter struct {
	detector *SessionDetector
	reducer  *SessionReducer

	replayer *terminal.Replayer
	matcher  *matcher.StreamMatcher

	newState          NewSessionState
	newContext        *SessionContext
	newCommittedLines []string
}

// NewSessionAdapter 创建新的会话适配器
func NewSessionAdapter(width int, commands []string, m *matcher.StreamMatcher) *SessionAdapter {
	adapter := &SessionAdapter{
		detector:          NewSessionDetector(m),
		reducer:           NewSessionReducer(commands, m),
		replayer:          terminal.NewReplayer(width),
		matcher:           m,
		newState:          NewStateInitAwaitPrompt,
		newContext:        NewSessionContext(commands),
		newCommittedLines: make([]string, 0),
	}

	return adapter
}

// FeedSessionActions 消费原始 chunk，返回统一的新动作模型
func (a *SessionAdapter) FeedSessionActions(chunk string) []SessionAction {
	// 1. 使用 Replayer 处理 chunk
	events := a.replayer.Process(chunk)
	newLines := make([]string, 0)
	activeLineUpdated := false

	// 收集新提交的行
	for _, event := range events {
		switch event.Type {
		case terminal.EventLineCommitted:
			newLines = append(newLines, event.Line)
			a.newCommittedLines = append(a.newCommittedLines, event.Line)
		case terminal.EventActiveLineUpdated:
			activeLineUpdated = true
		}
	}

	// 2. 使用 Detector 检测协议事件
	protocolEvents := a.detectSessionEvents(newLines, activeLineUpdated)

	// 3. 使用 Reducer 进行状态归约
	var sessionActions []SessionAction
	for _, event := range protocolEvents {
		actions := a.reducer.Reduce(event)
		sessionActions = append(sessionActions, actions...)
	}

	// 同步状态和上下文快照
	a.newState = a.reducer.State()
	a.newContext = a.reducer.Context()

	return sessionActions
}

// NewState 返回当前状态（新架构）
func (a *SessionAdapter) NewState() NewSessionState {
	return a.newState
}

// CurrentCommand 返回当前命令上下文
func (a *SessionAdapter) CurrentCommand() *CommandContext {
	return a.newContext.Current
}

// Results 返回所有已完成的命令结果
func (a *SessionAdapter) Results() []*CommandResult {
	return a.newContext.Results
}

// ActiveLine 返回当前活动行
func (a *SessionAdapter) ActiveLine() string {
	return a.replayer.ActiveLine()
}

// Lines 返回已提交的逻辑行
func (a *SessionAdapter) Lines() []string {
	return a.replayer.Lines()
}

// GetNewCommittedLines 获取自上次调用以来新提交的规范化行
func (a *SessionAdapter) GetNewCommittedLines() []string {
	if len(a.newCommittedLines) == 0 {
		return nil
	}
	lines := make([]string, len(a.newCommittedLines))
	copy(lines, a.newCommittedLines)
	a.newCommittedLines = a.newCommittedLines[:0]
	return lines
}

// ClearInitResiduals 清空初始化阶段的残留数据
func (a *SessionAdapter) ClearInitResiduals() {
	a.replayer.Reset()
	a.newCommittedLines = a.newCommittedLines[:0]
	a.newContext.PendingLines = a.newContext.PendingLines[:0]
	a.newContext.Current = nil
	logger.Debug("SessionAdapter", "-", "已清空初始化残留数据")
}

// MarkFailed 标记失败
func (a *SessionAdapter) MarkFailed(reason string) {
	a.newState = NewStateFailed
	if a.newContext.Current != nil {
		a.newContext.FailCurrentCommand(reason)
	}
	logger.Debug("SessionAdapter", "-", "标记失败: %s", reason)
}

// GetPendingError 获取待处理的错误上下文
func (a *SessionAdapter) GetPendingError() *ErrorContext {
	return a.newContext.PendingError
}

// ResolveErrorActions 解决错误（外部决策后调用），返回后续动作
func (a *SessionAdapter) ResolveErrorActions(continueExec bool) []SessionAction {
	var actions []SessionAction
	if continueExec {
		actions = a.reducer.Reduce(EvUserContinue{})
	} else {
		actions = a.reducer.Reduce(EvUserAbort{})
	}
	a.newState = a.reducer.State()
	a.newContext = a.reducer.Context()
	logger.Debug("SessionAdapter", "-", "解决错误: continue=%v", continueExec)
	return actions
}

// ReduceEvent 注入一个外部事件并同步状态快照。
func (a *SessionAdapter) ReduceEvent(event SessionEvent) []SessionAction {
	actions := a.reducer.Reduce(event)
	a.newState = a.reducer.State()
	a.newContext = a.reducer.Context()
	return actions
}

// ResolveError 兼容旧调用
func (a *SessionAdapter) ResolveError(continueExec bool) {
	_ = a.ResolveErrorActions(continueExec)
}

// ClearError 清除错误状态
func (a *SessionAdapter) ClearError() {
	a.newContext.PendingError = nil
}

// ============================================================================
// 状态转换辅助方法
// ============================================================================

// InitNewArchitecture 初始化新架构组件
// 供测试或重置场景重新初始化内部组件。
func (a *SessionAdapter) InitNewArchitecture(commands []string) {
	a.detector = NewSessionDetector(a.matcher)
	a.reducer = NewSessionReducer(commands, a.matcher)
	a.newContext = NewSessionContext(commands)
	a.newState = NewStateInitAwaitPrompt
	a.newCommittedLines = make([]string, 0)

	logger.Info("SessionAdapter", "-", "会话组件初始化完成")
}

// GetArchitectureMode 返回当前架构模式字符串
func (a *SessionAdapter) GetArchitectureMode() string {
	return "single (Replayer+Detector+Reducer)"
}

// GetStats 返回适配器统计信息
func (a *SessionAdapter) GetStats() map[string]interface{} {
	stats := map[string]interface{}{
		"mode":              a.GetArchitectureMode(),
		"committedLines":    len(a.newCommittedLines),
		"pendingLines":      len(a.newContext.PendingLines),
		"completedCommands": len(a.newContext.Results),
		"state":             a.newState.String(),
		"nextIndex":         a.newContext.NextIndex,
	}

	return stats
}

// TotalCommands 返回总命令数
func (a *SessionAdapter) TotalCommands() int {
	return len(a.newContext.Queue)
}

// SetCurrentTimeout 设置当前命令的超时时间
func (a *SessionAdapter) SetCurrentTimeout(timeout time.Duration) {
	if a.newContext.Current != nil {
		a.newContext.Current.SetCustomTimeout(timeout)
	}
}

// GetCurrentTimeout 获取当前命令的超时时间
func (a *SessionAdapter) GetCurrentTimeout() time.Duration {
	if a.newContext.Current != nil {
		return a.newContext.Current.CustomTimeout
	}
	return 0
}

func (a *SessionAdapter) detectSessionEvents(newLines []string, activeLineUpdated bool) []SessionEvent {
	activeLine := ""
	if activeLineUpdated {
		activeLine = a.replayer.ActiveLine()
	}

	switch a.reducer.State() {
	case NewStateInitAwaitPrompt:
		return a.detector.DetectInitPrompt(newLines, activeLine)
	case NewStateInitAwaitWarmupPrompt:
		return a.detector.DetectWarmupPrompt(newLines, activeLine)
	default:
		return a.detector.Detect(newLines, activeLine)
	}
}
