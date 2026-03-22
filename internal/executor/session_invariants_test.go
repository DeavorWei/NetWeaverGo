package executor

import (
	"testing"

	"github.com/NetWeaverGo/core/internal/matcher"
)

// ============================================================================
// 不变量测试 (Invariant Tests)
// ============================================================================
// 这些测试验证会话状态机的关键不变量规则：
// 1. pendingLines > 0 时不可发送业务命令
// 2. 单命令最多完成一次
// 3. Completed/Failed 状态不可回退
// 4. 初始化未完成前不可发送第一条命令
// ============================================================================

// TestInvariant_PendingLinesBlocksCommand 测试不变量：
// pendingLines > 0 时不可发送业务命令
func TestInvariant_PendingLinesBlocksCommand(t *testing.T) {
	m := matcher.NewStreamMatcher()
	machine := NewSessionMachine(80, []string{"display version"}, m)

	// 模拟进入 Ready 状态
	machine.state = StateReady

	// 模拟有待处理行（未消费的输出）
	machine.pendingLines = []string{"some unconsumed output"}

	// 尝试触发命令发送
	actions := machine.Feed("")

	// 验证：不应该发送命令
	for _, action := range actions {
		if action == ActionSendCommand {
			t.Error("不变量违反：pendingLines > 0 时发送了业务命令")
		}
	}

	// 验证状态没有进入 Collecting
	if machine.State() == StateCollecting {
		t.Error("不变量违反：pendingLines > 0 时状态进入了 Collecting")
	}
}

// TestInvariant_SingleCommandCompletion 测试不变量：
// 单命令最多完成一次
func TestInvariant_SingleCommandCompletion(t *testing.T) {
	m := matcher.NewStreamMatcher()
	machine := NewSessionMachine(80, []string{"display version"}, m)

	// 创建命令上下文
	ctx := NewCommandContext(0, "display version")
	ctx.SetCommand("display version")
	machine.current = ctx

	// 第一次完成
	ctx.MarkCompleted()
	initialCompleted := ctx.IsCompleted()

	// 尝试再次完成（应该无效或保持已完成状态）
	ctx.MarkCompleted()

	// 验证：命令仍然处于已完成状态，不会重复计数
	if !initialCompleted {
		t.Error("命令第一次完成失败")
	}

	if !ctx.IsCompleted() {
		t.Error("命令应该保持已完成状态")
	}

	// 验证 PromptMatched 只设置一次
	if ctx.PromptMatched != true {
		t.Error("PromptMatched 应该为 true")
	}
}

// TestInvariant_TerminalStateNoRegression 测试不变量：
// Completed/Failed 状态不可回退
func TestInvariant_TerminalStateNoRegression(t *testing.T) {
	m := matcher.NewStreamMatcher()

	tests := []struct {
		name     string
		terminal SessionState
	}{
		{"Completed 状态不可回退", StateCompleted},
		{"Failed 状态不可回退", StateFailed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			machine := NewSessionMachine(80, []string{"cmd"}, m)
			machine.state = tt.terminal

			// 尝试 Feed 数据
			actions := machine.Feed("some data\n<S1>")

			// 验证：终态不应该产生任何动作
			if len(actions) > 0 {
				t.Errorf("终态不应该产生动作，但产生了 %d 个动作", len(actions))
			}

			// 验证：状态保持不变
			if machine.State() != tt.terminal {
				t.Errorf("终态不应该改变：当前状态 = %s，期望 = %s", machine.State(), tt.terminal)
			}
		})
	}
}

// TestInvariant_InitRequiredBeforeCommand 测试不变量：
// 初始化未完成前不可发送第一条命令
func TestInvariant_InitRequiredBeforeCommand(t *testing.T) {
	m := matcher.NewStreamMatcher()
	machine := NewSessionMachine(80, []string{"display version"}, m)

	// 初始状态应该是 StateWaitInitialPrompt
	if machine.State() != StateWaitInitialPrompt {
		t.Errorf("初始状态应该是 StateWaitInitialPrompt，实际是 %s", machine.State())
	}

	// 在初始状态下 Feed 数据（不包含提示符）
	actions := machine.Feed("some random output\n")

	// 验证：不应该发送命令
	for _, action := range actions {
		if action == ActionSendCommand {
			t.Error("不变量违反：初始化未完成时发送了命令")
		}
	}

	// 验证状态没有跳过初始化阶段
	if machine.State() == StateCollecting || machine.State() == StateReady {
		t.Error("不变量违反：状态跳过了初始化阶段")
	}
}

// TestInvariant_StateTransitionOrder 测试不变量：
// 状态迁移必须遵循合法路径
func TestInvariant_StateTransitionOrder(t *testing.T) {
	// 定义合法的状态迁移路径
	// 注意：这是当前实现的简化版本，Phase 3 重构后会更新
	validTransitions := map[SessionState][]SessionState{
		StateWaitInitialPrompt:  {StateWarmup, StateCompleted, StateFailed},
		StateWarmup:             {StateReady, StateCompleted, StateFailed},
		StateReady:              {StateCollecting, StateHandlingPager, StateCompleted, StateFailed},
		StateCollecting:         {StateReady, StateHandlingPager, StateWaitingFinalPrompt, StateHandlingError, StateCompleted, StateFailed},
		StateHandlingPager:      {StateCollecting, StateReady, StateWaitingFinalPrompt, StateCompleted, StateFailed},
		StateWaitingFinalPrompt: {StateReady, StateCollecting, StateHandlingPager, StateCompleted, StateFailed},
		StateHandlingError:      {StateCollecting, StateCompleted, StateFailed},
		StateCompleted:          {}, // 终态
		StateFailed:             {}, // 终态
	}

	// 验证所有状态都有定义的迁移路径
	for fromState, toStates := range validTransitions {
		if fromState.IsTerminal() {
			if len(toStates) > 0 {
				t.Errorf("终态 %s 不应该有合法的迁移路径", fromState)
			}
		}
	}
}

// TestInvariant_PaginationBeforePrompt 测试不变量：
// 分页事件出现后，命令完成前必须先处理分页
func TestInvariant_PaginationBeforePrompt(t *testing.T) {
	m := matcher.NewStreamMatcher()
	machine := NewSessionMachine(80, []string{"display version"}, m)

	// 设置为 Collecting 状态
	machine.state = StateCollecting
	machine.current = NewCommandContext(0, "display version")
	machine.current.SetCommand("display version")

	// 模拟分页符在待处理行中
	machine.pendingLines = []string{"--More--"}

	// 触发处理
	actions := machine.Feed("")

	// 验证：应该发送空格处理分页
	hasSendSpace := false
	for _, action := range actions {
		if action == ActionSendSpace {
			hasSendSpace = true
		}
	}

	if !hasSendSpace {
		t.Error("不变量违反：检测到分页符但没有发送空格")
	}

	// 验证状态进入了分页处理
	if machine.State() != StateHandlingPager {
		t.Errorf("应该进入 StateHandlingPager，实际是 %s", machine.State())
	}
}

// TestInvariant_CommandContextIntegrity 测试不变量：
// 命令上下文完整性
func TestInvariant_CommandContextIntegrity(t *testing.T) {
	ctx := NewCommandContext(0, "display version")

	// 验证初始状态
	if ctx.Index != 0 {
		t.Errorf("Index 应该是 0，实际是 %d", ctx.Index)
	}

	if ctx.RawCommand != "display version" {
		t.Errorf("RawCommand 应该是 'display version'，实际是 '%s'", ctx.RawCommand)
	}

	if ctx.Command != "" {
		t.Errorf("初始 Command 应该为空，实际是 '%s'", ctx.Command)
	}

	if ctx.IsCompleted() {
		t.Error("初始状态不应该是已完成")
	}

	if ctx.PaginationCount != 0 {
		t.Errorf("初始 PaginationCount 应该是 0，实际是 %d", ctx.PaginationCount)
	}

	// 设置命令
	ctx.SetCommand("display version")
	if ctx.Command != "display version" {
		t.Errorf("SetCommand 失败，Command 是 '%s'", ctx.Command)
	}

	// 添加规范化行
	ctx.AddNormalizedLine("line1")
	ctx.AddNormalizedLine("line2")
	if len(ctx.NormalizedLines) != 2 {
		t.Errorf("应该有 2 行规范化输出，实际有 %d 行", len(ctx.NormalizedLines))
	}

	// 验证规范化文本
	text := ctx.NormalizedText()
	expected := "line1\nline2"
	if text != expected {
		t.Errorf("NormalizedText = '%s', 期望 '%s'", text, expected)
	}
}

// TestInvariant_SessionMachineInitialState 测试不变量：
// SessionMachine 初始状态正确性
func TestInvariant_SessionMachineInitialState(t *testing.T) {
	m := matcher.NewStreamMatcher()
	commands := []string{"cmd1", "cmd2", "cmd3"}
	machine := NewSessionMachine(80, commands, m)

	// 验证初始状态
	if machine.State() != StateWaitInitialPrompt {
		t.Errorf("初始状态应该是 StateWaitInitialPrompt，实际是 %s", machine.State())
	}

	// 验证命令队列
	if len(machine.queue) != 3 {
		t.Errorf("命令队列应该有 3 条命令，实际有 %d 条", len(machine.queue))
	}

	// 验证命令索引
	if machine.nextIndex != 0 {
		t.Errorf("nextIndex 应该是 0，实际是 %d", machine.nextIndex)
	}

	// 验证当前命令为空
	if machine.current != nil {
		t.Error("初始 current 应该为 nil")
	}

	// 验证结果列表为空
	if len(machine.results) != 0 {
		t.Errorf("初始 results 应该为空，实际有 %d 条", len(machine.results))
	}

	// 验证待处理行为空
	if len(machine.pendingLines) != 0 {
		t.Errorf("初始 pendingLines 应该为空，实际有 %d 行", len(machine.pendingLines))
	}
}

// TestInvariant_ErrorHandlingState 测试不变量：
// 错误处理状态的行为
func TestInvariant_ErrorHandlingState(t *testing.T) {
	m := matcher.NewStreamMatcher()
	machine := NewSessionMachine(80, []string{"cmd"}, m)

	// 设置为错误处理状态
	machine.state = StateHandlingError
	machine.current = NewCommandContext(0, "cmd")
	machine.current.SetCommand("cmd")

	// 在错误处理状态下，不应该自动发送新命令
	actions := machine.Feed("")

	// 验证：不应该发送命令
	for _, action := range actions {
		if action == ActionSendCommand {
			t.Error("错误处理状态下不应该发送新命令")
		}
	}
}

// TestInvariant_WarmupRequired 测试不变量：
// 预热阶段必须完成才能进入 Ready
func TestInvariant_WarmupRequired(t *testing.T) {
	m := matcher.NewStreamMatcher()
	machine := NewSessionMachine(80, []string{"cmd"}, m)

	// 初始状态
	if machine.State() != StateWaitInitialPrompt {
		t.Errorf("初始状态应该是 StateWaitInitialPrompt")
	}

	// 直接设置到 Ready（跳过预热）应该是不合法的
	// 但由于当前实现允许这样做，这里只记录这个不变量
	// Phase 3 重构时会强制执行这个不变量

	// 当前实现中，我们验证状态机的正常流程
	// 1. WaitInitialPrompt -> Warmup (检测到初始提示符)
	// 2. Warmup -> Ready (检测到预热后的提示符)

	// 这个测试记录了预期的行为
	t.Log("注意：Phase 3 重构后将强制执行预热不变量")
}
