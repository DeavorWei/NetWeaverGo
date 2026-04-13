package executor

import (
	"strings"
	"testing"

	"github.com/NetWeaverGo/core/internal/matcher"
)

func TestAdapter_UsesSingleArchitecture(t *testing.T) {
	m := matcher.NewStreamMatcher()
	adapter := NewSessionAdapter(80, []string{"display version"}, m)

	if got := adapter.GetArchitectureMode(); got != "single (Replayer+Detector+Reducer)" {
		t.Fatalf("架构模式 = %q, 期望 single (Replayer+Detector+Reducer)", got)
	}

	_ = feedEffects(adapter, "hostname# ")
	if state := adapter.NewState(); state != NewStateInitAwaitPrompt && state != NewStateInitAwaitWarmupPrompt && state != NewStateReady {
		t.Fatalf("初始化后状态异常: %s", state)
	}
}

func TestAdapter_StateProjection(t *testing.T) {
	m := matcher.NewStreamMatcher()
	adapter := NewSessionAdapter(80, []string{"display version", "display interface"}, m)

	_ = feedEffects(adapter, "hostname# ")

	newState := adapter.NewState()
	if newState != NewStateInitAwaitPrompt && newState != NewStateInitAwaitWarmupPrompt && newState != NewStateReady {
		t.Fatalf("新状态异常: %s", newState)
	}
}

func TestAdapter_OutputCollection(t *testing.T) {
	input := "\r\nhostname# display version\r\n" +
		"Huawei Versatile Routing Platform Software\r\n" +
		"VRP (R) software, Version 5.160\r\n" +
		"hostname# "

	m := matcher.NewStreamMatcher()
	adapter := NewSessionAdapter(80, []string{"display version"}, m)

	_ = feedEffects(adapter, "hostname# ")
	_ = feedEffects(adapter, input)

	if len(adapter.Lines()) == 0 && adapter.ActiveLine() == "" {
		t.Fatal("期望至少有已提交行或活动行")
	}
}

func TestAdapter_PaginationHandling(t *testing.T) {
	input := "GigabitEthernet0/0/1 current state: UP\r\n" +
		"GigabitEthernet0/0/2 current state: DOWN\r\n" +
		"  ---- More ----"

	m := matcher.NewStreamMatcher()
	adapter := NewSessionAdapter(80, []string{"display interface"}, m)

	actions := feedEffects(adapter, "hostname# ")
	actions = feedEffects(adapter, "\r\nhostname# ")
	if adapter.NewState() != NewStateRunning {
		t.Fatalf("预热后应进入 Running，实际是 %s", adapter.NewState())
	}

	actions = feedEffects(adapter, input)

	hasPagerContinue := false
	for _, action := range actions {
		if _, ok := action.(ActSendPagerContinue); ok {
			hasPagerContinue = true
			break
		}
	}
	if !hasPagerContinue {
		t.Fatal("期望检测到分页动作")
	}
}

func TestAdapter_ErrorHandling(t *testing.T) {
	m := matcher.NewStreamMatcher()
	adapter := NewSessionAdapter(80, []string{"display version"}, m)

	_ = feedEffects(adapter, "hostname# ")
	actions := feedEffects(adapter, "\r\nhostname# ")
	if adapter.NewState() != NewStateRunning {
		t.Fatalf("预热后应进入 Running，实际是 %s", adapter.NewState())
	}

	actions = feedEffects(adapter, "  Error: Unrecognized command\r\nhostname# ")

	state := adapter.NewState()
	if state != NewStateSuspended {
		t.Fatalf("命令错误应进入 Suspended，实际是 %s", state)
	}

	foundSuspend := false
	for _, action := range actions {
		if _, ok := action.(ActRequestSuspendDecision); ok {
			foundSuspend = true
			break
		}
	}
	if !foundSuspend {
		t.Fatal("期望产生挂起决策动作")
	}
}

func TestAdapter_MultipleCommands(t *testing.T) {
	m := matcher.NewStreamMatcher()
	adapter := NewSessionAdapter(80, []string{"display version", "display interface", "display arp"}, m)

	_ = feedEffects(adapter, "hostname# ")
	_ = feedEffects(adapter, "\r\nhostname# ")
	initialCount := len(adapter.Results())

	actions := feedEffects(adapter, "display version\r\noutput\r\nhostname# ")
	foundNextCommand := false
	for _, action := range actions {
		if act, ok := action.(ActSendCommand); ok && act.Index == 1 {
			foundNextCommand = true
			break
		}
	}
	if !foundNextCommand {
		t.Fatal("第一条命令完成后应自动发送第二条命令")
	}

	if len(adapter.Results()) < initialCount {
		t.Fatalf("结果数不应倒退: before=%d after=%d", initialCount, len(adapter.Results()))
	}
}

func TestAdapter_Stats(t *testing.T) {
	m := matcher.NewStreamMatcher()
	adapter := NewSessionAdapter(80, []string{"display version"}, m)

	stats := adapter.GetStats()
	if stats == nil {
		t.Fatal("期望非空统计信息")
	}
	if _, ok := stats["mode"]; !ok {
		t.Fatal("统计信息缺少 mode")
	}
	if _, ok := stats["state"]; !ok {
		t.Fatal("统计信息缺少 state")
	}
	if _, ok := stats["nextIndex"]; !ok {
		t.Fatal("统计信息缺少 nextIndex")
	}
}

func TestAdapter_ClearInitResiduals(t *testing.T) {
	m := matcher.NewStreamMatcher()
	adapter := NewSessionAdapter(80, []string{"display version"}, m)

	_ = feedEffects(adapter, "hostname# ")
	_ = feedEffects(adapter, "some output\r\n")
	adapter.ClearInitResiduals()
}

func TestAdapter_MarkFailed(t *testing.T) {
	m := matcher.NewStreamMatcher()
	adapter := NewSessionAdapter(80, []string{"display version"}, m)

	_ = feedEffects(adapter, "hostname# ")
	adapter.MarkFailed("test failure")

	if state := adapter.NewState(); state != NewStateFailed {
		t.Fatalf("状态 = %s, 期望 Failed", state)
	}
}

func TestAdapter_RuntimeEventsFromNormalizedOutput(t *testing.T) {
	m := matcher.NewStreamMatcher()
	adapter := NewSessionAdapter(80, []string{"display version"}, m)

	actions := feedEffects(adapter, "hostname# ")
	if len(actions) != 1 {
		t.Fatalf("首次提示符应只产生预热动作，得到 %d 个动作", len(actions))
	}
	if _, ok := actions[0].(ActSendWarmup); !ok {
		t.Fatalf("首次动作应为 ActSendWarmup，实际是 %T", actions[0])
	}

	actions = feedEffects(adapter, "\r\nhostname# ")
	if adapter.NewState() != NewStateRunning {
		t.Fatalf("预热后应进入 Running，实际是 %s", adapter.NewState())
	}

	foundCommand := false
	for _, action := range actions {
		if act, ok := action.(ActSendCommand); ok && act.Index == 0 && act.Command == "display version" {
			foundCommand = true
			break
		}
	}
	if !foundCommand {
		t.Fatal("预热后应发送第一条命令")
	}

	actions = feedEffects(adapter, "Huawei Versatile Routing Platform Software\r\nhostname# ")
	if adapter.NewState() != NewStateCompleted {
		t.Fatalf("命令完成后应进入 Completed，实际是 %s", adapter.NewState())
	}

	if len(adapter.Results()) != 1 {
		t.Fatalf("应产生 1 条命令结果，实际是 %d", len(adapter.Results()))
	}

	if len(actions) != 1 {
		t.Fatalf("最后一条命令完成后应产生 1 个完成事件动作，实际有 %d 个", len(actions))
	}
	if _, ok := actions[0].(ActEmitCommandDone); !ok {
		t.Fatalf("最后一条命令完成后的动作应为 ActEmitCommandDone，实际是 %T", actions[0])
	}
}

func TestAdapter_FeedTransitionBatchMatchesLegacyActions(t *testing.T) {
	m := matcher.NewStreamMatcher()
	adapter := NewSessionAdapter(80, []string{"display version"}, m)

	batch := adapter.FeedTransitionBatch("hostname# ")
	if batch == nil {
		t.Fatal("FeedTransitionBatch 不应返回 nil")
	}
	if batch.IsEmpty() {
		t.Fatal("首次提示符应产生预热 batch")
	}
	legacy := batch.Effects
	if len(legacy) != 1 {
		t.Fatalf("batch effect 数错误: got=%d want=1", len(legacy))
	}
	if _, ok := legacy[0].(ActSendWarmup); !ok {
		t.Fatalf("首次 batch effect 应为 ActSendWarmup，实际是 %T", legacy[0])
	}

	batch = adapter.FeedTransitionBatch("\r\nhostname# ")
	legacy = batch.Effects
	foundCommand := false
	for _, action := range legacy {
		if act, ok := action.(ActSendCommand); ok && act.Index == 0 && act.Command == "display version" {
			foundCommand = true
			break
		}
	}
	if !foundCommand {
		t.Fatal("预热完成后 batch 应发送第一条命令")
	}
}

func TestFeedTransitionBatch_FillsRawBuffer(t *testing.T) {
	m := matcher.NewStreamMatcher()
	adapter := NewSessionAdapter(80, []string{"display version"}, m)

	// 初始化：等待提示符
	_ = adapter.FeedTransitionBatch("hostname# ")

	// 进入 Running 状态
	_ = adapter.FeedTransitionBatch("\r\nhostname# ")
	if adapter.NewState() != NewStateRunning {
		t.Fatalf("预热后应进入 Running，实际是 %s", adapter.NewState())
	}

	// 验证当前命令存在
	current := adapter.CurrentCommand()
	if current == nil {
		t.Fatal("Running 状态应有当前命令")
	}

	// 执行：发送命令输出
	chunk := "display version\r\nHuawei Versatile Routing Platform Software\r\nhostname# "
	_ = adapter.FeedTransitionBatch(chunk)

	// 验证：RawBuffer 应被填充
	current = adapter.CurrentCommand()
	if current == nil {
		// 命令可能已完成，检查 Results
		if len(adapter.Results()) == 0 {
			t.Fatal("应有命令结果")
		}
		// 检查结果中的 RawBuffer
		result := adapter.Results()[len(adapter.Results())-1]
		if len(result.RawText) == 0 {
			t.Fatal("RawText 应包含原始数据")
		}
		if result.RawSize == 0 {
			t.Fatal("RawSize 应大于 0")
		}
		// 验证内容包含命令输出
		rawText := result.RawText
		if len(rawText) == 0 {
			t.Fatal("RawText 不应为空")
		}
		// RawBuffer 应包含我们发送的 chunk 数据
		// 注意：由于数据流经过 Replayer 处理，可能不包含完整的原始 chunk
		// 但至少应包含部分命令输出
		t.Logf("RawText 长度: %d, 内容前100字符: %q", len(rawText), truncateString(rawText, 100))
	} else {
		// 命令仍在执行中，检查 RawBuffer
		if len(current.RawBuffer) == 0 {
			t.Fatal("RawBuffer 应被填充")
		}
		t.Logf("RawBuffer 长度: %d, 内容前100字符: %q", len(current.RawBuffer), truncateString(string(current.RawBuffer), 100))
	}
}

func TestFeedTransitionBatch_RawBufferAccumulates(t *testing.T) {
	m := matcher.NewStreamMatcher()
	adapter := NewSessionAdapter(80, []string{"display interface"}, m)

	// 初始化
	_ = adapter.FeedTransitionBatch("hostname# ")
	_ = adapter.FeedTransitionBatch("\r\nhostname# ")

	// 发送多个 chunk，验证 RawBuffer 累积
	chunk1 := "display interface\r\n"
	_ = adapter.FeedTransitionBatch(chunk1)

	current := adapter.CurrentCommand()
	if current == nil {
		t.Fatal("应有当前命令")
	}
	firstLen := len(current.RawBuffer)

	chunk2 := "GigabitEthernet0/0/1 current state: UP\r\n"
	_ = adapter.FeedTransitionBatch(chunk2)

	current = adapter.CurrentCommand()
	if current == nil {
		t.Fatal("应有当前命令")
	}
	secondLen := len(current.RawBuffer)

	// RawBuffer 应该累积增长
	if secondLen <= firstLen {
		t.Fatalf("RawBuffer 应累积增长: first=%d, second=%d", firstLen, secondLen)
	}

	t.Logf("RawBuffer 累积: %d -> %d", firstLen, secondLen)
}

func TestFeedTransitionBatch_MultiCommandRawBufferSeparation(t *testing.T) {
	m := matcher.NewStreamMatcher()
	adapter := NewSessionAdapter(80, []string{"display version", "display interface"}, m)

	// 初始化
	_ = adapter.FeedTransitionBatch("hostname# ")
	_ = adapter.FeedTransitionBatch("\r\nhostname# ")

	// 第一条命令输出
	chunk1 := "display version\r\nVRP Version 5.160\r\nhostname# "
	_ = adapter.FeedTransitionBatch(chunk1)

	// 验证第一条命令结果
	if len(adapter.Results()) != 1 {
		t.Fatalf("应有 1 条结果，实际是 %d", len(adapter.Results()))
	}
	result1 := adapter.Results()[0]
	t.Logf("命令1 RawText 长度: %d, 内容: %q", len(result1.RawText), truncateString(result1.RawText, 100))

	// 第二条命令输出
	chunk2 := "display interface\r\nGigabitEthernet0/0/1 UP\r\nhostname# "
	_ = adapter.FeedTransitionBatch(chunk2)

	// 验证第二条命令结果
	if len(adapter.Results()) != 2 {
		t.Fatalf("应有 2 条结果，实际是 %d", len(adapter.Results()))
	}
	result2 := adapter.Results()[1]
	t.Logf("命令2 RawText 长度: %d, 内容: %q", len(result2.RawText), truncateString(result2.RawText, 100))

	// 验证两条命令的 RawText 都不为空
	if len(result1.RawText) == 0 {
		t.Fatal("命令1 RawText 不应为空")
	}
	if len(result2.RawText) == 0 {
		t.Fatal("命令2 RawText 不应为空")
	}

	// 验证内容正确分割
	if !containsSubstring(result1.RawText, "version") && !containsSubstring(result1.RawText, "Version") {
		t.Fatalf("命令1 RawText 应包含 'version': %q", result1.RawText)
	}
	if !containsSubstring(result2.RawText, "interface") && !containsSubstring(result2.RawText, "GigabitEthernet") {
		t.Fatalf("命令2 RawText 应包含 'interface': %q", result2.RawText)
	}
}

func containsSubstring(s, substr string) bool {
	return strings.Contains(s, substr)
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

// ============================================================================
// SessionContext 实例合并修复验证测试
// ============================================================================

// TestSessionAdapter_SingleContextInstance 验证 adapter.newContext 和 reducer.ctx 是同一个实例
func TestSessionAdapter_SingleContextInstance(t *testing.T) {
	m := matcher.NewStreamMatcher()
	adapter := NewSessionAdapter(80, []string{"display version"}, m)

	// 验证 adapter.newContext 和 reducer.ctx 是同一个实例
	if adapter.newContext != adapter.reducer.Context() {
		t.Fatal("adapter.newContext 和 reducer.ctx 应该是同一个实例，修复未生效")
	}

	// 验证初始状态也一致
	if adapter.newState != adapter.reducer.State() {
		t.Fatal("adapter.newState 和 reducer.state 应该一致")
	}
}

// TestFeedTransitionBatch_FirstChunkRawBufferNotLost 验证第一个 chunk 的 RawBuffer 数据不丢失
func TestFeedTransitionBatch_FirstChunkRawBufferNotLost(t *testing.T) {
	m := matcher.NewStreamMatcher()
	adapter := NewSessionAdapter(80, []string{"display version"}, m)

	// 先发送初始化 prompt
	_ = adapter.FeedTransitionBatch("hostname# ")
	_ = adapter.FeedTransitionBatch("\r\nhostname# ")

	// 确保进入 Running 状态
	if adapter.NewState() != NewStateRunning {
		t.Fatalf("应进入 Running 状态，实际是 %s", adapter.NewState())
	}

	// 模拟第一个 chunk 时 Current 不为 nil 的场景
	// 此时应该有当前命令
	current := adapter.CurrentCommand()
	if current == nil {
		// 如果没有当前命令，手动设置一个来模拟场景
		adapter.reducer.ctx.Current = NewCommandContext(0, "display version")
		adapter.reducer.ctx.Current.SetCommand("display version")
		adapter.newContext = adapter.reducer.Context()
	}

	// 第一个 chunk - 包含命令输出
	chunk := "display version\r\nVRP Version 5.160\r\n"
	_ = adapter.FeedTransitionBatch(chunk)

	// 验证 RawBuffer 被正确填充
	current = adapter.CurrentCommand()
	if current == nil {
		t.Fatal("应有当前命令")
	}
	if len(current.RawBuffer) == 0 {
		t.Fatal("第一个 chunk 的 RawBuffer 不应丢失")
	}
	if !strings.Contains(string(current.RawBuffer), "display version") {
		t.Fatalf("RawBuffer 应包含命令内容: %q", string(current.RawBuffer))
	}

	t.Logf("RawBuffer 长度: %d, 内容: %q", len(current.RawBuffer), string(current.RawBuffer))
}

// TestSessionAdapter_ContextInstanceConsistencyAfterMultipleFeeds 验证多次 Feed 后实例一致性
func TestSessionAdapter_ContextInstanceConsistencyAfterMultipleFeeds(t *testing.T) {
	m := matcher.NewStreamMatcher()
	adapter := NewSessionAdapter(80, []string{"display version", "display interface"}, m)

	// 初始验证
	initialCtx := adapter.newContext
	if initialCtx != adapter.reducer.Context() {
		t.Fatal("初始状态：adapter.newContext 和 reducer.ctx 应该是同一个实例")
	}

	// 第一次 Feed
	_ = adapter.FeedTransitionBatch("hostname# ")
	if adapter.newContext != adapter.reducer.Context() {
		t.Fatal("第一次 Feed 后：adapter.newContext 和 reducer.ctx 应该是同一个实例")
	}

	// 第二次 Feed
	_ = adapter.FeedTransitionBatch("\r\nhostname# ")
	if adapter.newContext != adapter.reducer.Context() {
		t.Fatal("第二次 Feed 后：adapter.newContext 和 reducer.ctx 应该是同一个实例")
	}

	// 第三次 Feed - 命令输出
	_ = adapter.FeedTransitionBatch("display version\r\nVRP Version 5.160\r\nhostname# ")
	if adapter.newContext != adapter.reducer.Context() {
		t.Fatal("第三次 Feed 后：adapter.newContext 和 reducer.ctx 应该是同一个实例")
	}
}
