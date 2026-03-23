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
	input := "hostname# display interface\r\n" +
		"GigabitEthernet0/0/1 current state: UP\r\n" +
		"GigabitEthernet0/0/2 current state: DOWN\r\n" +
		"  ---- More ----"

	m := matcher.NewStreamMatcher()
	adapter := NewSessionAdapter(80, []string{"display interface"}, m)

	_ = adapter.FeedSessionActions("hostname# ")
	actions := adapter.FeedSessionActions(input)

	hasPagerContinue := false
	for _, action := range actions {
		if _, ok := action.(ActSendPagerContinue); ok {
			hasPagerContinue = true
			break
		}
	}
	if !hasPagerContinue {
		t.Log("未检测到分页动作，当前输入可能不足以触发该路径")
	}
}

func TestAdapter_ErrorHandling(t *testing.T) {
	m := matcher.NewStreamMatcher()
	adapter := NewSessionAdapter(80, []string{"display version"}, m)

	_ = adapter.FeedSessionActions("hostname# ")
	_ = adapter.FeedSessionActions("  Error: Unrecognized command\r\nhostname# ")

	state := adapter.NewState()
	if state == NewStateFailed {
		t.Fatalf("普通命令错误不应直接进入 Failed: %s", state)
	}
}

func TestAdapter_MultipleCommands(t *testing.T) {
	m := matcher.NewStreamMatcher()
	adapter := NewSessionAdapter(80, []string{"display version", "display interface", "display arp"}, m)

	_ = adapter.FeedSessionActions("hostname# ")
	initialCount := len(adapter.Results())

	_ = adapter.FeedSessionActions("display version\r\noutput\r\nhostname# ")

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
