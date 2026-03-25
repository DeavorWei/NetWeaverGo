package ui

import (
	"encoding/json"
	"time"

	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/taskexec"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// TaskExecutionEventBridge 统一任务执行事件桥接器
// 负责将 taskexec 内部事件转换为 Wails 事件发送到前端
type TaskExecutionEventBridge struct {
	eventBus *taskexec.EventBus
	wailsApp *application.App
	runIDs   map[string]bool // 追踪已订阅的run
}

// NewTaskExecutionEventBridge 创建事件桥接器
func NewTaskExecutionEventBridge(eventBus *taskexec.EventBus) *TaskExecutionEventBridge {
	return &TaskExecutionEventBridge{
		eventBus: eventBus,
		runIDs:   make(map[string]bool),
	}
}

// SetWailsApp 设置Wails应用实例
func (b *TaskExecutionEventBridge) SetWailsApp(app *application.App) {
	b.wailsApp = app
}

// Start 启动事件桥接
func (b *TaskExecutionEventBridge) Start() {
	if b.eventBus == nil {
		logger.Error("TaskExecEventBridge", "-", "事件总线未初始化")
		return
	}

	// 订阅所有任务事件
	b.eventBus.Subscribe(b.handleEvent)
	logger.Info("TaskExecEventBridge", "-", "事件桥接已启动")
}

// Stop 停止事件桥接
func (b *TaskExecutionEventBridge) Stop() {
	// 取消订阅（当前实现是清空所有处理器）
	if b.eventBus != nil {
		b.eventBus.Unsubscribe()
	}
	logger.Info("TaskExecEventBridge", "-", "事件桥接已停止")
}

// handleEvent 处理任务事件
func (b *TaskExecutionEventBridge) handleEvent(event *taskexec.TaskEvent) {
	if b.wailsApp == nil {
		return
	}

	// 转换为前端事件格式
	frontendEvent := b.convertToFrontendEvent(event)

	// 发送全局任务事件
	b.emitToFrontend("task:event", frontendEvent)

	// 根据事件类型发送特定事件
	switch event.Type {
	case taskexec.EventTypeRunStarted:
		b.emitToFrontend("task:started", frontendEvent)
	case taskexec.EventTypeRunFinished:
		b.emitToFrontend("task:finished", frontendEvent)
		// 清理追踪
		delete(b.runIDs, event.RunID)
	case taskexec.EventTypeStageStarted, taskexec.EventTypeStageFinished, taskexec.EventTypeStageProgress:
		b.emitToFrontend("task:stage_updated", frontendEvent)
	case taskexec.EventTypeUnitStarted, taskexec.EventTypeUnitFinished, taskexec.EventTypeUnitProgress:
		b.emitToFrontend("task:unit_updated", frontendEvent)
	}

	// 发送run特定的快照更新事件
	b.emitToFrontend("task:snapshot", map[string]interface{}{
		"runId":     event.RunID,
		"timestamp": time.Now().UnixMilli(),
		"eventType": string(event.Type),
	})
}

// convertToFrontendEvent 转换为前端事件格式
func (b *TaskExecutionEventBridge) convertToFrontendEvent(event *taskexec.TaskEvent) map[string]interface{} {
	return map[string]interface{}{
		"id":        event.ID,
		"runId":     event.RunID,
		"stageId":   event.StageID,
		"unitId":    event.UnitID,
		"type":      string(event.Type),
		"level":     string(event.Level),
		"message":   event.Message,
		"payload":   event.Payload,
		"timestamp": event.Timestamp.UnixMilli(),
	}
}

// emitToFrontend 发送事件到前端
func (b *TaskExecutionEventBridge) emitToFrontend(eventName string, data interface{}) {
	if b.wailsApp == nil || b.wailsApp.Event == nil {
		return
	}

	// 序列化数据
	jsonData, err := json.Marshal(data)
	if err != nil {
		logger.Error("TaskExecEventBridge", "-", "序列化事件数据失败: %v", err)
		return
	}

	// 使用 Wails 事件系统发送到前端
	b.wailsApp.Event.Emit(eventName, string(jsonData))
}

// SubscribeRun 订阅特定run的事件（用于前端页面打开时）
func (b *TaskExecutionEventBridge) SubscribeRun(runID string) {
	b.runIDs[runID] = true
}

// UnsubscribeRun 取消订阅特定run的事件
func (b *TaskExecutionEventBridge) UnsubscribeRun(runID string) {
	delete(b.runIDs, runID)
}

// FrontendEvent 前端事件结构
type FrontendEvent struct {
	ID        string                 `json:"id"`
	RunID     string                 `json:"runId"`
	StageID   string                 `json:"stageId,omitempty"`
	UnitID    string                 `json:"unitId,omitempty"`
	Type      string                 `json:"type"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Payload   map[string]interface{} `json:"payload,omitempty"`
	Timestamp int64                  `json:"timestamp"`
}
