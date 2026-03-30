package ui

import (
	"testing"
	"time"

	"github.com/NetWeaverGo/core/internal/taskexec"
	"github.com/wailsapp/wails/v3/pkg/application"
)

func TestTaskExecutionEventBridgeShouldEmitDefaultAll(t *testing.T) {
	bridge := NewTaskExecutionEventBridge(nil, nil)
	if !bridge.shouldEmit("run-1") {
		t.Fatal("未设置订阅过滤时应默认放行所有 run")
	}
}

func TestTaskExecutionEventBridgeSubscribeAndUnsubscribe(t *testing.T) {
	bridge := NewTaskExecutionEventBridge(nil, nil)
	bridge.SubscribeRun("run-1")

	if !bridge.shouldEmit("run-1") {
		t.Fatal("已订阅 run-1 后应允许发送")
	}
	if bridge.shouldEmit("run-2") {
		t.Fatal("存在订阅过滤时，未订阅 run-2 不应允许发送")
	}

	bridge.UnsubscribeRun("run-1")
	if !bridge.shouldEmit("run-2") {
		t.Fatal("清空订阅过滤后应恢复默认放行")
	}
}

func TestTaskExecutionEventBridgeConvertToFrontendEvent(t *testing.T) {
	bridge := NewTaskExecutionEventBridge(nil, nil)
	event := taskexec.NewTaskEvent("run-1", taskexec.EventTypeCommandDispatched, "命令开始").
		WithStage("stage-1").
		WithUnit("unit-1").
		WithLevel(taskexec.EventLevelWarn).
		WithPayload("sessionSeq", 5)

	data := bridge.convertToFrontendEvent(event)
	if data["runId"] != "run-1" {
		t.Fatalf("runId 映射错误: %v", data["runId"])
	}
	if data["type"] != string(taskexec.EventTypeCommandDispatched) {
		t.Fatalf("type 映射错误: %v", data["type"])
	}
	if data["level"] != string(taskexec.EventLevelWarn) {
		t.Fatalf("level 映射错误: %v", data["level"])
	}
	payload, ok := data["payload"].(map[string]interface{})
	if !ok {
		t.Fatalf("payload 映射错误: %#v", data["payload"])
	}
	if payload["sessionSeq"] != 5 {
		t.Fatalf("payload sessionSeq 映射错误: %#v", payload["sessionSeq"])
	}
}

func TestTaskExecutionEventBridgeHandleEventWithFilterAndFinishUnsubscribe(t *testing.T) {
	deltaCalls := 0
	bridge := NewTaskExecutionEventBridge(nil, func(runID string) (*taskexec.SnapshotDelta, error) {
		deltaCalls++
		return nil, nil
	})
	bridge.SetWailsApp(&application.App{})
	bridge.SubscribeRun("run-1")
	bridge.SubscribeRun("run-2")

	bridge.handleEvent(taskexec.NewTaskEvent("run-1", taskexec.EventTypeRunStarted, "started").WithPayload("ts", time.Now().UnixNano()))
	if deltaCalls != 1 {
		t.Fatalf("run-1 started 后应拉取一次 delta，实际=%d", deltaCalls)
	}

	bridge.handleEvent(taskexec.NewTaskEvent("run-1", taskexec.EventTypeRunFinished, "finished"))
	if deltaCalls != 2 {
		t.Fatalf("run-1 finished 后应拉取一次 delta，实际=%d", deltaCalls)
	}
	if bridge.shouldEmit("run-1") {
		t.Fatal("run-1 完成后应被取消订阅")
	}
	if !bridge.shouldEmit("run-2") {
		t.Fatal("run-2 应仍保留订阅")
	}

	bridge.handleEvent(taskexec.NewTaskEvent("run-1", taskexec.EventTypeRunStarted, "restart"))
	if deltaCalls != 2 {
		t.Fatalf("未订阅 run-1 时不应再拉取 delta，实际=%d", deltaCalls)
	}
}
