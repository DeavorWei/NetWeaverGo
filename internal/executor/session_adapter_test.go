package executor

import (
	"testing"

	"github.com/NetWeaverGo/core/internal/matcher"
)

func TestAdapter_UsesSingleArchitecture(t *testing.T) {
	m := matcher.NewStreamMatcher()
	adapter := NewSessionAdapter(80, []string{"display version"}, m)

	if got := adapter.GetArchitectureMode(); got != "single (Replayer+Detector+Reducer)" {
		t.Fatalf("架构模式 = %q, 期望 single (Replayer+Detector+Reducer)", got)
	}

	_ = adapter.FeedSessionActions("hostname# ")
	if state := adapter.NewState(); state != NewStateInitAwaitPrompt && state != NewStateInitAwaitWarmupPrompt && state != NewStateReady {
		t.Fatalf("初始化后状态异常: %s", state)
	}
}

func TestAdapter_StateProjection(t *testing.T) {
	m := matcher.NewStreamMatcher()
	adapter := NewSessionAdapter(80, []string{"display version", "display interface"}, m)

	_ = adapter.FeedSessionActions("hostname# ")

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

	_ = adapter.FeedSessionActions("hostname# ")
	_ = adapter.FeedSessionActions(input)

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

	actions := adapter.FeedSessionActions("hostname# ")
	actions = adapter.FeedSessionActions("\r\nhostname# ")
	if adapter.NewState() != NewStateRunning {
		t.Fatalf("预热后应进入 Running，实际是 %s", adapter.NewState())
	}

	actions = adapter.FeedSessionActions(input)

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

	_ = adapter.FeedSessionActions("hostname# ")
	actions := adapter.FeedSessionActions("\r\nhostname# ")
	if adapter.NewState() != NewStateRunning {
		t.Fatalf("预热后应进入 Running，实际是 %s", adapter.NewState())
	}

	actions = adapter.FeedSessionActions("  Error: Unrecognized command\r\nhostname# ")

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

	_ = adapter.FeedSessionActions("hostname# ")
	_ = adapter.FeedSessionActions("\r\nhostname# ")
	initialCount := len(adapter.Results())

	actions := adapter.FeedSessionActions("display version\r\noutput\r\nhostname# ")
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

	_ = adapter.FeedSessionActions("hostname# ")
	_ = adapter.FeedSessionActions("some output\r\n")
	adapter.ClearInitResiduals()
}

func TestAdapter_MarkFailed(t *testing.T) {
	m := matcher.NewStreamMatcher()
	adapter := NewSessionAdapter(80, []string{"display version"}, m)

	_ = adapter.FeedSessionActions("hostname# ")
	adapter.MarkFailed("test failure")

	if state := adapter.NewState(); state != NewStateFailed {
		t.Fatalf("状态 = %s, 期望 Failed", state)
	}
}

func TestAdapter_RuntimeEventsFromNormalizedOutput(t *testing.T) {
	m := matcher.NewStreamMatcher()
	adapter := NewSessionAdapter(80, []string{"display version"}, m)

	actions := adapter.FeedSessionActions("hostname# ")
	if len(actions) != 1 {
		t.Fatalf("首次提示符应只产生预热动作，得到 %d 个动作", len(actions))
	}
	if _, ok := actions[0].(ActSendWarmup); !ok {
		t.Fatalf("首次动作应为 ActSendWarmup，实际是 %T", actions[0])
	}

	actions = adapter.FeedSessionActions("\r\nhostname# ")
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

	actions = adapter.FeedSessionActions("Huawei Versatile Routing Platform Software\r\nhostname# ")
	if adapter.NewState() != NewStateCompleted {
		t.Fatalf("命令完成后应进入 Completed，实际是 %s", adapter.NewState())
	}

	if len(adapter.Results()) != 1 {
		t.Fatalf("应产生 1 条命令结果，实际是 %d", len(adapter.Results()))
	}

	if len(actions) != 0 {
		t.Fatalf("最后一条命令完成后不应再有后续动作，实际有 %d 个", len(actions))
	}
}
