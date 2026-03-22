package executor

import (
	"testing"

	"github.com/NetWeaverGo/core/internal/matcher"
)

// ============================================================================
// Reducer 单元测试 (Phase 1 重构)
// ============================================================================
// 这些测试完全不依赖 SSH，只测试状态迁移逻辑

// MockMatcher 实现 MatcherInterface 用于测试
type MockMatcher struct {
	prompts    []string
	pagers     []string
	errorLines map[string]*matcher.ErrorRule
}

func NewMockMatcher() *MockMatcher {
	return &MockMatcher{
		prompts:    []string{"<SW1>", "<S1>", ">", "#"},
		pagers:     []string{"--More--", "---- More ----"},
		errorLines: make(map[string]*matcher.ErrorRule),
	}
}

func (m *MockMatcher) IsPrompt(line string) bool {
	for _, p := range m.prompts {
		if line == p || len(line) > 0 && line[len(line)-1:] == ">" {
			return true
		}
	}
	return false
}

func (m *MockMatcher) IsPromptStrict(line string) bool {
	for _, p := range m.prompts {
		if line == p {
			return true
		}
	}
	// 简单匹配 <xxx> 格式
	if len(line) >= 3 && line[0] == '<' && line[len(line)-1] == '>' {
		return true
	}
	return false
}

func (m *MockMatcher) IsPaginationPrompt(line string) bool {
	for _, p := range m.pagers {
		if line == p {
			return true
		}
	}
	return false
}

func (m *MockMatcher) MatchErrorRule(line string) (bool, *matcher.ErrorRule) {
	rule, ok := m.errorLines[line]
	return ok, rule
}

func (m *MockMatcher) AddErrorLine(line string, rule *matcher.ErrorRule) {
	m.errorLines[line] = rule
}

// ============================================================================
// 基础测试
// ============================================================================

// TestReducerInitialState 测试 Reducer 初始状态
func TestReducerInitialState(t *testing.T) {
	m := NewMockMatcher()
	reducer := NewSessionReducer([]string{"cmd1", "cmd2"}, m)

	if reducer.State() != NewStateInitAwaitPrompt {
		t.Errorf("初始状态应该是 NewStateInitAwaitPrompt，实际是 %s", reducer.State())
	}

	if reducer.Context() == nil {
		t.Error("上下文不应该为 nil")
	}

	if len(reducer.Context().Queue) != 2 {
		t.Errorf("命令队列应该有 2 条命令，实际有 %d 条", len(reducer.Context().Queue))
	}
}

// TestReducerTerminalState 测试终态不处理事件
func TestReducerTerminalState(t *testing.T) {
	m := NewMockMatcher()
	reducer := NewSessionReducer([]string{"cmd1"}, m)

	// 设置为终态
	reducer.state = NewStateCompleted

	// 尝试处理事件
	actions := reducer.Reduce(EvInitPromptStable{Prompt: "<SW1>"})

	if len(actions) != 0 {
		t.Errorf("终态不应该产生动作，但产生了 %d 个动作", len(actions))
	}

	if reducer.State() != NewStateCompleted {
		t.Errorf("终态不应该改变，实际是 %s", reducer.State())
	}
}

// ============================================================================
// 初始化流程测试
// ============================================================================

// TestReducerInitPromptToWarmup 测试初始提示符到预热
func TestReducerInitPromptToWarmup(t *testing.T) {
	m := NewMockMatcher()
	reducer := NewSessionReducer([]string{"display version"}, m)

	// 发送初始提示符稳定事件
	actions := reducer.Reduce(EvInitPromptStable{Prompt: "<SW1>"})

	// 验证状态转换
	if reducer.State() != NewStateInitAwaitWarmupPrompt {
		t.Errorf("状态应该是 NewStateInitAwaitWarmupPrompt，实际是 %s", reducer.State())
	}

	// 验证产生发送预热动作
	if len(actions) != 1 {
		t.Fatalf("应该产生 1 个动作，实际产生了 %d 个", len(actions))
	}

	if _, ok := actions[0].(ActSendWarmup); !ok {
		t.Errorf("动作应该是 ActSendWarmup，实际是 %T", actions[0])
	}
}

// TestReducerWarmupToReady 测试预热后进入就绪
func TestReducerWarmupToReady(t *testing.T) {
	m := NewMockMatcher()
	reducer := NewSessionReducer([]string{"display version"}, m)

	// 先进入预热等待状态
	reducer.Reduce(EvInitPromptStable{Prompt: "<SW1>"})

	// 发送预热后提示符事件
	actions := reducer.Reduce(EvWarmupPromptSeen{Prompt: "<SW1>"})

	// 验证状态转换
	if reducer.State() != NewStateRunning {
		t.Errorf("状态应该是 NewStateRunning（自动发送命令），实际是 %s", reducer.State())
	}

	// 验证产生发送命令动作
	found := false
	for _, a := range actions {
		if _, ok := a.(ActSendCommand); ok {
			found = true
			break
		}
	}

	if !found {
		t.Error("应该产生 ActSendCommand 动作")
	}
}

// ============================================================================
// 命令执行测试
// ============================================================================

// TestReducerSendCommand 测试发送命令
func TestReducerSendCommand(t *testing.T) {
	m := NewMockMatcher()
	reducer := NewSessionReducer([]string{"display version"}, m)

	// 设置为就绪状态
	reducer.state = NewStateReady
	reducer.ctx.InitResidualCleared = true

	// 触发命令发送（通过空事件或直接调用 trySendCommand）
	actions := reducer.trySendCommand()

	// 验证状态转换
	if reducer.State() != NewStateRunning {
		t.Errorf("状态应该是 NewStateRunning，实际是 %s", reducer.State())
	}

	// 验证命令上下文
	if reducer.Context().Current == nil {
		t.Fatal("当前命令上下文不应该为 nil")
	}

	// 注意：Command 字段需要通过 SetCommand 设置，这里检查 RawCommand
	if reducer.Context().Current.RawCommand != "display version" {
		t.Errorf("RawCommand 应该是 'display version'，实际是 '%s'", reducer.Context().Current.RawCommand)
	}

	// 验证产生发送命令动作
	if len(actions) != 1 {
		t.Fatalf("应该产生 1 个动作，实际产生了 %d 个", len(actions))
	}

	act, ok := actions[0].(ActSendCommand)
	if !ok {
		t.Errorf("动作应该是 ActSendCommand，实际是 %T", actions[0])
	}

	// 动作中的命令从 RawCommand 获取
	if act.Command != "display version" {
		t.Errorf("动作命令应该是 'display version'，实际是 '%s'", act.Command)
	}
}

// TestReducerPendingLinesBlocksCommand 测试 pendingLines 阻止命令发送
func TestReducerPendingLinesBlocksCommand(t *testing.T) {
	m := NewMockMatcher()
	reducer := NewSessionReducer([]string{"display version"}, m)

	// 设置为就绪状态
	reducer.state = NewStateReady
	reducer.ctx.InitResidualCleared = true

	// 添加待处理行
	reducer.ctx.AddPendingLine("some output")

	// 尝试发送命令
	actions := reducer.trySendCommand()

	// 验证状态没有改变
	if reducer.State() != NewStateReady {
		t.Errorf("状态应该保持 NewStateReady，实际是 %s", reducer.State())
	}

	// 验证没有产生动作
	if len(actions) != 0 {
		t.Errorf("不应该产生动作，但产生了 %d 个", len(actions))
	}
}

// ============================================================================
// 分页处理测试
// ============================================================================

// TestReducerPagerSeen 测试分页符检测
func TestReducerPagerSeen(t *testing.T) {
	m := NewMockMatcher()
	reducer := NewSessionReducer([]string{"display version"}, m)

	// 设置为运行状态
	reducer.state = NewStateRunning
	reducer.ctx.Current = NewCommandContext(0, "display version")

	// 发送分页符事件
	actions := reducer.Reduce(EvPagerSeen{Line: "--More--"})

	// 验证状态转换
	if reducer.State() != NewStateAwaitPagerContinueAck {
		t.Errorf("状态应该是 NewStateAwaitPagerContinueAck，实际是 %s", reducer.State())
	}

	// 验证分页计数
	if reducer.ctx.Current.PaginationCount != 1 {
		t.Errorf("分页计数应该是 1，实际是 %d", reducer.ctx.Current.PaginationCount)
	}

	// 验证产生发送空格动作
	if len(actions) != 1 {
		t.Fatalf("应该产生 1 个动作，实际产生了 %d 个", len(actions))
	}

	if _, ok := actions[0].(ActSendPagerContinue); !ok {
		t.Errorf("动作应该是 ActSendPagerContinue，实际是 %T", actions[0])
	}
}

// ============================================================================
// 错误处理测试
// ============================================================================

// TestReducerErrorMatched 测试错误匹配
func TestReducerErrorMatched(t *testing.T) {
	m := NewMockMatcher()
	m.AddErrorLine("Error: command failed", &matcher.ErrorRule{
		Name:     "命令失败",
		Severity: matcher.SeverityCritical,
		Message:  "命令执行失败",
	})

	reducer := NewSessionReducer([]string{"bad command"}, m)

	// 设置为运行状态
	reducer.state = NewStateRunning
	reducer.ctx.Current = NewCommandContext(0, "bad command")

	// 发送错误匹配事件
	actions := reducer.Reduce(EvErrorMatched{
		Line: "Error: command failed",
		Rule: &matcher.ErrorRule{
			Name:     "命令失败",
			Severity: matcher.SeverityCritical,
			Message:  "命令执行失败",
		},
	})

	// 验证状态转换
	if reducer.State() != NewStateSuspended {
		t.Errorf("状态应该是 NewStateSuspended，实际是 %s", reducer.State())
	}

	// 验证产生请求挂起决策动作
	if len(actions) != 1 {
		t.Fatalf("应该产生 1 个动作，实际产生了 %d 个", len(actions))
	}

	if _, ok := actions[0].(ActRequestSuspendDecision); !ok {
		t.Errorf("动作应该是 ActRequestSuspendDecision，实际是 %T", actions[0])
	}
}

// TestReducerWarningPass 测试警告级别错误放行
func TestReducerWarningPass(t *testing.T) {
	m := NewMockMatcher()
	reducer := NewSessionReducer([]string{"cmd"}, m)

	// 设置为运行状态
	reducer.state = NewStateRunning
	reducer.ctx.Current = NewCommandContext(0, "cmd")

	// 发送警告级别错误事件
	actions := reducer.Reduce(EvErrorMatched{
		Line: "Warning: minor issue",
		Rule: &matcher.ErrorRule{
			Name:     "警告",
			Severity: matcher.SeverityWarning,
			Message:  "轻微警告",
		},
	})

	// 验证状态没有改变
	if reducer.State() != NewStateRunning {
		t.Errorf("状态应该保持 NewStateRunning，实际是 %s", reducer.State())
	}

	// 验证没有产生动作
	if len(actions) != 0 {
		t.Errorf("警告级别不应该产生动作，但产生了 %d 个", len(actions))
	}
}

// TestReducerUserContinue 测试用户继续
func TestReducerUserContinue(t *testing.T) {
	m := NewMockMatcher()
	reducer := NewSessionReducer([]string{"cmd"}, m)

	// 设置为挂起状态
	reducer.state = NewStateSuspended
	reducer.ctx.Current = NewCommandContext(0, "cmd")

	// 发送用户继续事件
	actions := reducer.Reduce(EvUserContinue{CommandIndex: 0})

	// 验证状态转换
	if reducer.State() != NewStateRunning {
		t.Errorf("状态应该是 NewStateRunning，实际是 %s", reducer.State())
	}

	// 验证产生重置超时动作
	if len(actions) != 1 {
		t.Fatalf("应该产生 1 个动作，实际产生了 %d 个", len(actions))
	}

	if _, ok := actions[0].(ActResetReadTimeout); !ok {
		t.Errorf("动作应该是 ActResetReadTimeout，实际是 %T", actions[0])
	}
}

// TestReducerUserAbort 测试用户中止
func TestReducerUserAbort(t *testing.T) {
	m := NewMockMatcher()
	reducer := NewSessionReducer([]string{"cmd"}, m)

	// 设置为挂起状态
	reducer.state = NewStateSuspended
	reducer.ctx.Current = NewCommandContext(0, "cmd")

	// 发送用户中止事件
	actions := reducer.Reduce(EvUserAbort{CommandIndex: 0})

	// 验证状态转换
	if reducer.State() != NewStateFailed {
		t.Errorf("状态应该是 NewStateFailed，实际是 %s", reducer.State())
	}

	// 验证产生中止会话动作
	if len(actions) != 1 {
		t.Fatalf("应该产生 1 个动作，实际产生了 %d 个", len(actions))
	}

	if _, ok := actions[0].(ActAbortSession); !ok {
		t.Errorf("动作应该是 ActAbortSession，实际是 %T", actions[0])
	}
}

// ============================================================================
// 超时和流关闭测试
// ============================================================================

// TestReducerTimeout 测试超时
func TestReducerTimeout(t *testing.T) {
	m := NewMockMatcher()
	reducer := NewSessionReducer([]string{"cmd"}, m)

	// 设置为运行状态
	reducer.state = NewStateRunning
	reducer.ctx.Current = NewCommandContext(0, "cmd")

	// 发送超时事件
	actions := reducer.Reduce(EvTimeout{CommandIndex: 0})

	// 验证状态转换
	if reducer.State() != NewStateFailed {
		t.Errorf("状态应该是 NewStateFailed，实际是 %s", reducer.State())
	}

	// 验证产生中止会话动作
	if len(actions) != 1 {
		t.Fatalf("应该产生 1 个动作，实际产生了 %d 个", len(actions))
	}

	act, ok := actions[0].(ActAbortSession)
	if !ok {
		t.Errorf("动作应该是 ActAbortSession，实际是 %T", actions[0])
	}

	if act.Reason != "timeout" {
		t.Errorf("中止原因应该是 'timeout'，实际是 '%s'", act.Reason)
	}
}

// TestReducerStreamClosed 测试流关闭
func TestReducerStreamClosed(t *testing.T) {
	m := NewMockMatcher()
	reducer := NewSessionReducer([]string{"cmd"}, m)

	// 设置为运行状态
	reducer.state = NewStateRunning
	reducer.ctx.Current = NewCommandContext(0, "cmd")

	// 发送流关闭事件
	actions := reducer.Reduce(EvStreamClosed{})

	// 验证状态转换
	if reducer.State() != NewStateFailed {
		t.Errorf("状态应该是 NewStateFailed，实际是 %s", reducer.State())
	}

	// 验证产生中止会话动作
	if len(actions) != 1 {
		t.Fatalf("应该产生 1 个动作，实际产生了 %d 个", len(actions))
	}

	act, ok := actions[0].(ActAbortSession)
	if !ok {
		t.Errorf("动作应该是 ActAbortSession，实际是 %T", actions[0])
	}

	if act.Reason != "stream_closed" {
		t.Errorf("中止原因应该是 'stream_closed'，实际是 '%s'", act.Reason)
	}
}

// ============================================================================
// 命令完成测试
// ============================================================================

// TestReducerCommandComplete 测试命令完成
func TestReducerCommandComplete(t *testing.T) {
	m := NewMockMatcher()
	reducer := NewSessionReducer([]string{"cmd1", "cmd2"}, m)

	// 设置为运行状态
	reducer.state = NewStateRunning
	reducer.ctx.Current = NewCommandContext(0, "cmd1")
	reducer.ctx.Current.SetCommand("cmd1")
	reducer.ctx.NextIndex = 1 // 第一条命令已发送

	// 发送提示符事件
	actions := reducer.Reduce(EvActivePromptSeen{Prompt: "<SW1>"})

	// 验证状态转换（应该自动发送下一条命令）
	if reducer.State() != NewStateRunning {
		t.Errorf("状态应该是 NewStateRunning（自动发送下一条命令），实际是 %s", reducer.State())
	}

	// 验证命令完成
	if len(reducer.ctx.Results) != 1 {
		t.Errorf("应该有 1 个完成的命令结果，实际有 %d 个", len(reducer.ctx.Results))
	}

	// 验证产生发送命令动作
	found := false
	for _, a := range actions {
		if act, ok := a.(ActSendCommand); ok {
			if act.Index == 1 && act.Command == "cmd2" {
				found = true
			}
		}
	}

	if !found {
		t.Error("应该产生发送第二条命令的动作")
	}
}

// TestReducerAllCommandsComplete 测试所有命令完成
func TestReducerAllCommandsComplete(t *testing.T) {
	m := NewMockMatcher()
	reducer := NewSessionReducer([]string{"cmd1"}, m)

	// 设置为运行状态
	reducer.state = NewStateRunning
	reducer.ctx.Current = NewCommandContext(0, "cmd1")
	reducer.ctx.Current.SetCommand("cmd1")
	reducer.ctx.NextIndex = 1 // 已经发送了第一条命令

	// 发送提示符事件
	actions := reducer.Reduce(EvActivePromptSeen{Prompt: "<SW1>"})

	// 验证状态转换
	if reducer.State() != NewStateCompleted {
		t.Errorf("状态应该是 NewStateCompleted，实际是 %s", reducer.State())
	}

	// 验证没有产生动作
	if len(actions) != 0 {
		t.Errorf("所有命令完成后不应该产生动作，但产生了 %d 个", len(actions))
	}
}

// ============================================================================
// 状态字符串测试
// ============================================================================

// TestNewSessionStateString 测试状态字符串表示
func TestNewSessionStateString(t *testing.T) {
	tests := []struct {
		state    NewSessionState
		expected string
	}{
		{NewStateInitAwaitPrompt, "InitAwaitPrompt"},
		{NewStateInitAwaitWarmupPrompt, "InitAwaitWarmupPrompt"},
		{NewStateReady, "Ready"},
		{NewStateRunning, "Running"},
		{NewStateAwaitPagerContinueAck, "AwaitPagerContinueAck"},
		{NewStateAwaitFinalPromptConfirm, "AwaitFinalPromptConfirm"},
		{NewStateSuspended, "Suspended"},
		{NewStateCompleted, "Completed"},
		{NewStateFailed, "Failed"},
	}

	for _, tt := range tests {
		if got := tt.state.String(); got != tt.expected {
			t.Errorf("State(%d).String() = %q, want %q", tt.state, got, tt.expected)
		}
	}
}

// TestNewSessionStateIsTerminal 测试终态判断
func TestNewSessionStateIsTerminal(t *testing.T) {
	terminalStates := []NewSessionState{NewStateCompleted, NewStateFailed}
	nonTerminalStates := []NewSessionState{
		NewStateInitAwaitPrompt, NewStateInitAwaitWarmupPrompt, NewStateReady,
		NewStateRunning, NewStateAwaitPagerContinueAck, NewStateAwaitFinalPromptConfirm,
		NewStateSuspended,
	}

	for _, s := range terminalStates {
		if !s.IsTerminal() {
			t.Errorf("State(%s).IsTerminal() = false, want true", s)
		}
	}

	for _, s := range nonTerminalStates {
		if s.IsTerminal() {
			t.Errorf("State(%s).IsTerminal() = true, want false", s)
		}
	}
}
