package executor

import (
	"testing"

	"github.com/NetWeaverGo/core/internal/matcher"
)

func TestInvariant_PendingLinesBlocksCommand(t *testing.T) {
	m := matcher.NewStreamMatcher()
	reducer := NewSessionReducer([]string{"display version"}, m)
	ctx := reducer.Context()
	ctx.PendingLines = []string{"some unconsumed output"}
	reducer.state = NewStateReady

	actions := reduceEffects(reducer, EvActivePromptSeen{Prompt: "<SW1>"})
	for _, action := range actions {
		if _, ok := action.(ActSendCommand); ok {
			t.Fatal("pendingLines 非空时不应发送命令")
		}
	}
	if reducer.State() != NewStateReady {
		t.Fatalf("状态不应离开 Ready，实际是 %s", reducer.State())
	}
}

func TestInvariant_TerminalStateNoRegression(t *testing.T) {
	m := matcher.NewStreamMatcher()
	tests := []NewSessionState{NewStateCompleted, NewStateFailed}

	for _, terminalState := range tests {
		t.Run(terminalState.String(), func(t *testing.T) {
			reducer := NewSessionReducer([]string{"cmd"}, m)
			reducer.state = terminalState
			actions := reduceEffects(reducer, EvActivePromptSeen{Prompt: "<SW1>"})
			if len(actions) != 0 {
				t.Fatalf("终态不应产生动作，得到 %d 个", len(actions))
			}
			if reducer.State() != terminalState {
				t.Fatalf("终态不应回退: got=%s want=%s", reducer.State(), terminalState)
			}
		})
	}
}

func TestInvariant_InitRequiredBeforeCommand(t *testing.T) {
	m := matcher.NewStreamMatcher()
	adapter := NewSessionAdapter(80, []string{"display version"}, m)

	actions := feedEffects(adapter, "some random output\n")
	for _, action := range actions {
		if _, ok := action.(ActSendCommand); ok {
			t.Fatal("初始化未完成时不应发送命令")
		}
	}

	state := adapter.NewState()
	if state == NewStateReady || state == NewStateRunning {
		t.Fatalf("状态不应跳过初始化阶段: %s", state)
	}
}

func TestInvariant_PaginationBeforePrompt(t *testing.T) {
	m := matcher.NewStreamMatcher()
	reducer := NewSessionReducer([]string{"display version"}, m)
	reducer.state = NewStateRunning
	reducer.ctx.Current = NewCommandContext(0, "display version")
	reducer.ctx.Current.SetCommand("display version")

	actions := reduceEffects(reducer, EvPagerSeen{Line: "--More--"})

	found := false
	for _, action := range actions {
		if _, ok := action.(ActSendPagerContinue); ok {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("检测到分页事件后应优先发送续页动作")
	}
	if reducer.State() != NewStateAwaitPagerContinueAck {
		t.Fatalf("状态应进入 AwaitPagerContinueAck，实际是 %s", reducer.State())
	}
}

func TestInvariant_CommandContextIntegrity(t *testing.T) {
	ctx := NewCommandContext(0, "display version")

	if ctx.Index != 0 || ctx.RawCommand != "display version" {
		t.Fatal("命令上下文初始字段异常")
	}
	if ctx.Command != "" || ctx.IsCompleted() || ctx.PaginationCount != 0 {
		t.Fatal("命令上下文初始状态异常")
	}

	ctx.SetCommand("display version")
	ctx.AddNormalizedLine("line1")
	ctx.AddNormalizedLine("line2")
	if text := ctx.NormalizedText(); text != "line1\nline2" {
		t.Fatalf("NormalizedText 异常: %q", text)
	}
}

func TestInvariant_SessionAdapterInitialState(t *testing.T) {
	m := matcher.NewStreamMatcher()
	adapter := NewSessionAdapter(80, []string{"cmd1", "cmd2", "cmd3"}, m)

	if adapter.NewState() != NewStateInitAwaitPrompt {
		t.Fatalf("初始状态异常: %s", adapter.NewState())
	}
	if adapter.TotalCommands() != 3 {
		t.Fatalf("命令总数异常: %d", adapter.TotalCommands())
	}
	if adapter.CurrentCommand() != nil {
		t.Fatal("初始 current 应为 nil")
	}
	if len(adapter.Results()) != 0 {
		t.Fatalf("初始结果应为空: %d", len(adapter.Results()))
	}
}
