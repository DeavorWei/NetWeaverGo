package taskexec

import (
	"context"
	"testing"
	"time"

	"github.com/NetWeaverGo/core/internal/executor"
)

func TestSuspendManager_RegisterAndResolve(t *testing.T) {
	mgr := &SuspendManager{
		requests: make(map[string]*SuspendRequest),
	}

	req := &SuspendRequest{
		ID:        "req-001",
		RunID:     "run-123",
		DeviceIP:  "192.168.1.1",
		Prompt:    "Continue? [Y/N]:",
		Command:   "reset saved-configuration",
		CreatedAt: time.Now(),
	}

	mgr.Register(req)

	pending := mgr.ListPending()
	if len(pending) != 1 {
		t.Fatalf("期望待处理请求为 1，实际为 %d", len(pending))
	}
	if pending[0].ID != "req-001" {
		t.Errorf("期望请求 ID 为 req-001，实际为 %s", pending[0].ID)
	}

	// 异步等待决议
	done := make(chan executor.ErrorAction, 1)
	go func() {
		action := mgr.WaitDecision(context.Background(), req, 2*time.Second)
		done <- action
	}()

	// 决议为 Continue
	time.Sleep(50 * time.Millisecond)
	resolved := mgr.Resolve("req-001", executor.ActionContinue)
	if !resolved {
		t.Fatalf("期望决议成功，但返回 false")
	}

	select {
	case act := <-done:
		if act != executor.ActionContinue {
			t.Errorf("期望决议动作为 Continue，实际得到 %v", act)
		}
	case <-time.After(1 * time.Second):
		t.Fatalf("等待决议超时")
	}

	// 决议后从待处理列表中移除
	if len(mgr.ListPending()) != 0 {
		t.Errorf("决议后待处理列表应为空，实际为 %d", len(mgr.ListPending()))
	}
}

func TestSuspendManager_WaitDecision_ContextCancel(t *testing.T) {
	mgr := &SuspendManager{
		requests: make(map[string]*SuspendRequest),
	}

	req := &SuspendRequest{
		ID:        "req-cancel",
		RunID:     "run-cancel",
		DeviceIP:  "192.168.1.2",
		Prompt:    "Are you sure?",
		Command:   "reboot",
		CreatedAt: time.Now(),
	}

	mgr.Register(req)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	action := mgr.WaitDecision(ctx, req, 1*time.Second)
	if action != executor.ActionAbort {
		t.Errorf("上下文取消后期望动作为 ActionAbort，实际得到 %v", action)
	}
}

func TestBuildDefaultSuspendHandler_Policies(t *testing.T) {
	ctx := context.Background()

	// 1. errorMode == "skip" 自动放行
	skipHandler := BuildDefaultSuspendHandler("run-test", "skip")
	action := skipHandler(ctx, "10.0.0.1", "Warning prompt", "disp cur")
	if action != executor.ActionContinue {
		t.Errorf("skip 模式期望 ActionContinue，实际得到 %v", action)
	}

	// 2. errorMode == "abort" 直接终止
	abortHandler := BuildDefaultSuspendHandler("run-test", "abort")
	action = abortHandler(ctx, "10.0.0.1", "Warning prompt", "disp cur")
	if action != executor.ActionAbort {
		t.Errorf("abort 模式期望 ActionAbort，实际得到 %v", action)
	}
}
