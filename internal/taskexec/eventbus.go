package taskexec

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

// TaskEvent 统一任务事件
type TaskEvent struct {
	ID        string                 `json:"id"`
	RunID     string                 `json:"runId"`
	StageID   string                 `json:"stageId,omitempty"`
	UnitID    string                 `json:"unitId,omitempty"`
	Type      EventType              `json:"type"`
	Level     EventLevel             `json:"level"`
	Message   string                 `json:"message"`
	Payload   map[string]interface{} `json:"payload,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// NewTaskEvent 创建任务事件
func NewTaskEvent(runID string, eventType EventType, message string) *TaskEvent {
	return &TaskEvent{
		ID:        uuid.New().String(),
		RunID:     runID,
		Type:      eventType,
		Level:     EventLevelInfo,
		Message:   message,
		Timestamp: time.Now(),
	}
}

// WithStage 设置StageID
func (e *TaskEvent) WithStage(stageID string) *TaskEvent {
	e.StageID = stageID
	return e
}

// WithUnit 设置UnitID
func (e *TaskEvent) WithUnit(unitID string) *TaskEvent {
	e.UnitID = unitID
	return e
}

// WithLevel 设置事件级别
func (e *TaskEvent) WithLevel(level EventLevel) *TaskEvent {
	e.Level = level
	return e
}

// WithPayload 设置负载
func (e *TaskEvent) WithPayload(key string, value interface{}) *TaskEvent {
	if e.Payload == nil {
		e.Payload = make(map[string]interface{})
	}
	e.Payload[key] = value
	return e
}

// EventHandler 事件处理器
type EventHandler func(event *TaskEvent)

// EventBus 事件总线
type EventBus struct {
	handlers []EventHandler
	mu       sync.RWMutex
	buffer   chan *TaskEvent
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewEventBus 创建事件总线
func NewEventBus(bufferSize int) *EventBus {
	ctx, cancel := context.WithCancel(context.Background())
	return &EventBus{
		handlers: make([]EventHandler, 0),
		buffer:   make(chan *TaskEvent, bufferSize),
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Subscribe 订阅事件
func (b *EventBus) Subscribe(handler EventHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers = append(b.handlers, handler)
}

// Unsubscribe 取消订阅（简单实现：清空所有）
func (b *EventBus) Unsubscribe() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers = make([]EventHandler, 0)
}

// Emit 发送事件
func (b *EventBus) Emit(event *TaskEvent) {
	select {
	case b.buffer <- event:
	default:
		// 缓冲区满，丢弃最旧的事件
		select {
		case <-b.buffer:
			b.buffer <- event
		default:
		}
	}
}

// EmitSync 同步发送事件（直接调用处理器）
func (b *EventBus) EmitSync(event *TaskEvent) {
	b.mu.RLock()
	handlers := make([]EventHandler, len(b.handlers))
	copy(handlers, b.handlers)
	b.mu.RUnlock()

	for _, handler := range handlers {
		handler(event)
	}
}

// Start 启动事件分发
func (b *EventBus) Start() {
	go b.dispatchLoop()
}

// Stop 停止事件总线
func (b *EventBus) Stop() {
	b.cancel()
}

// dispatchLoop 事件分发循环
func (b *EventBus) dispatchLoop() {
	for {
		select {
		case <-b.ctx.Done():
			return
		case event := <-b.buffer:
			b.mu.RLock()
			handlers := make([]EventHandler, len(b.handlers))
			copy(handlers, b.handlers)
			b.mu.RUnlock()

			for _, handler := range handlers {
				go handler(event) // 异步处理避免阻塞
			}
		}
	}
}

// SnapshotHub 快照中心 - 维护最新快照供前端查询
type SnapshotHub struct {
	snapshots map[string]*ExecutionSnapshot
	mu        sync.RWMutex
	eventBus  *EventBus
}

// NewSnapshotHub 创建快照中心
func NewSnapshotHub(eventBus *EventBus) *SnapshotHub {
	hub := &SnapshotHub{
		snapshots: make(map[string]*ExecutionSnapshot),
		eventBus:  eventBus,
	}

	// 订阅事件更新快照
	eventBus.Subscribe(func(event *TaskEvent) {
		hub.invalidate(event.RunID)
	})

	return hub
}

// Update 更新快照
func (h *SnapshotHub) Update(runID string, snapshot *ExecutionSnapshot) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.snapshots[runID] = snapshot
}

// Get 获取快照
func (h *SnapshotHub) Get(runID string) (*ExecutionSnapshot, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	snapshot, ok := h.snapshots[runID]
	return snapshot, ok
}

// invalidate 标记快照失效（触发重新查询）
func (h *SnapshotHub) invalidate(runID string) {
	// 实际实现可以选择删除缓存，让下次查询从数据库重建
	// 或者这里可以触发异步重建
}

// ListRunning 获取所有运行中的快照
func (h *SnapshotHub) ListRunning() []*ExecutionSnapshot {
	h.mu.RLock()
	defer h.mu.RUnlock()

	result := make([]*ExecutionSnapshot, 0)
	for _, snapshot := range h.snapshots {
		if snapshot.Status == string(RunStatusRunning) {
			result = append(result, snapshot)
		}
	}
	return result
}

// Cleanup 清理已完成的快照
func (h *SnapshotHub) Cleanup() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for id, snapshot := range h.snapshots {
		if RunStatus(snapshot.Status).IsTerminal() {
			delete(h.snapshots, id)
		}
	}
}
