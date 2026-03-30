package taskexec

import (
	"context"
	"log"
	"sort"
	"sync"
	"time"
)

// SubscriptionID 订阅标识
type SubscriptionID int64

// DefaultEmitSyncTimeout EmitSync 默认超时时间
const DefaultEmitSyncTimeout = 5 * time.Second

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
		ID:        newEventID(),
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
	handlers map[SubscriptionID]EventHandler
	nextID   SubscriptionID
	mu       sync.RWMutex
	buffer   chan *TaskEvent
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup // 等待 goroutine 退出
	started  bool
	closed   bool
}

// NewEventBus 创建事件总线
func NewEventBus(bufferSize int) *EventBus {
	ctx, cancel := context.WithCancel(context.Background())
	return &EventBus{
		handlers: make(map[SubscriptionID]EventHandler),
		nextID:   0,
		buffer:   make(chan *TaskEvent, bufferSize),
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Subscribe 订阅事件，返回取消订阅函数
func (b *EventBus) Subscribe(handler EventHandler) func() {
	b.mu.Lock()
	defer b.mu.Unlock()

	id := b.nextID
	b.nextID++
	b.handlers[id] = handler

	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		delete(b.handlers, id)
	}
}

// UnsubscribeAll 取消所有订阅
func (b *EventBus) UnsubscribeAll() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers = make(map[SubscriptionID]EventHandler)
}

// Emit 发送事件
func (b *EventBus) Emit(event *TaskEvent) {
	if event == nil || b.isClosed() {
		return
	}

	select {
	case <-b.ctx.Done():
		return
	case b.buffer <- event:
	default:
		// 缓冲区满，丢弃最旧的事件
		select {
		case <-b.buffer:
			select {
			case <-b.ctx.Done():
				return
			case b.buffer <- event:
			default:
			}
		default:
		}
	}
}

// EmitSync 同步发送事件（串行调用处理器，带超时保护）
func (b *EventBus) EmitSync(event *TaskEvent) {
	if event == nil || b.isClosed() {
		return
	}

	handlers := b.snapshotHandlers()
	if len(handlers) == 0 {
		return
	}

	done := make(chan struct{})
	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		defer close(done)
		for _, handler := range handlers {
			if b.isClosed() {
				return
			}
			b.safeInvokeHandler(handler, event)
		}
	}()

	select {
	case <-done:
		// 正常完成
	case <-time.After(DefaultEmitSyncTimeout):
		log.Printf("[EventBus] EmitSync timeout after %v, runID=%s", DefaultEmitSyncTimeout, event.RunID)
	case <-b.ctx.Done():
		// EventBus 已关闭
	}
}

// Start 启动事件分发
func (b *EventBus) Start() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.started || b.closed {
		return
	}
	b.started = true
	b.wg.Add(1)
	go b.dispatchLoop()
}

// Stop 停止事件总线
func (b *EventBus) Stop() {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return
	}
	b.closed = true
	b.mu.Unlock()

	b.cancel()

	// 等待所有 dispatch / handler / EmitSync 受控 goroutine 退出（带超时保护）
	done := make(chan struct{})
	go func() {
		b.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// 正常退出
	case <-time.After(5 * time.Second):
		log.Printf("[EventBus] Stop timeout, some handlers may still be running")
	}
}

// dispatchLoop 事件分发循环
// 说明：这里必须按订阅顺序串行执行处理器，避免 SnapshotHub/Bridge 出现跨处理器乱序。
func (b *EventBus) dispatchLoop() {
	defer b.wg.Done()
	for {
		select {
		case <-b.ctx.Done():
			return
		case event := <-b.buffer:
			if event == nil || b.isClosed() {
				continue
			}
			handlers := b.snapshotHandlers()
			for _, handler := range handlers {
				if b.isClosed() {
					return
				}
				b.safeInvokeHandler(handler, event)
			}
		}
	}
}

func (b *EventBus) snapshotHandlers() []EventHandler {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if len(b.handlers) == 0 {
		return nil
	}

	ids := make([]SubscriptionID, 0, len(b.handlers))
	for id := range b.handlers {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		return ids[i] < ids[j]
	})

	handlers := make([]EventHandler, 0, len(ids))
	for _, id := range ids {
		h := b.handlers[id]
		if h != nil {
			handlers = append(handlers, h)
		}
	}
	return handlers
}

func (b *EventBus) safeInvokeHandler(handler EventHandler, event *TaskEvent) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[EventBus] handler panic recovered: %v", r)
		}
	}()
	handler(event)
}

func (b *EventBus) isClosed() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.closed
}

// SnapshotHub 快照中心 - 维护最新快照供前端查询
type SnapshotHub struct {
	snapshots      map[string]*ExecutionSnapshot
	revisions      map[string]uint64
	pendingOps     map[string][]SnapshotDeltaOp
	pendingBaseSeq map[string]uint64
	unsubscribe    func()
	mu             sync.RWMutex
}

// NewSnapshotHub 创建快照中心
func NewSnapshotHub(eventBus *EventBus) *SnapshotHub {
	hub := &SnapshotHub{
		snapshots:      make(map[string]*ExecutionSnapshot),
		revisions:      make(map[string]uint64),
		pendingOps:     make(map[string][]SnapshotDeltaOp),
		pendingBaseSeq: make(map[string]uint64),
	}
	if eventBus != nil {
		hub.unsubscribe = eventBus.Subscribe(func(event *TaskEvent) {
			hub.AppendEvent(event)
		})
	}
	return hub
}

// Update 更新快照
func (h *SnapshotHub) Update(runID string, snapshot *ExecutionSnapshot) {
	if snapshot == nil {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	cloned := snapshot.Clone()
	h.bumpRevisionLocked(runID, cloned)
	h.resetDeltaQueueLocked(runID)
	snapshot.Revision = cloned.Revision
	snapshot.UpdatedAt = cloned.UpdatedAt
	snapshot.LastRunSeq = cloned.LastRunSeq
	h.snapshots[runID] = cloned
}

// Get 获取快照
func (h *SnapshotHub) Get(runID string) (*ExecutionSnapshot, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	snapshot, ok := h.snapshots[runID]
	if !ok || snapshot == nil {
		return nil, false
	}
	return snapshot.Clone(), true
}

// BuildDelta 基于当前快照构建增量消息。
// 当存在 pending ops 时只返回 patch；否则返回全量快照用于初始化。
func (h *SnapshotHub) BuildDelta(runID string) (*SnapshotDelta, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	snapshot, ok := h.snapshots[runID]
	if !ok || snapshot == nil {
		return nil, false
	}
	cloned := snapshot.Clone()

	ops := h.pendingOps[runID]
	if len(ops) == 0 {
		return &SnapshotDelta{
			RunID:     runID,
			BaseSeq:   0,
			Seq:       cloned.LastRunSeq,
			Revision:  cloned.Revision,
			UpdatedAt: cloned.UpdatedAt,
			Snapshot:  cloned,
		}, true
	}

	baseSeq := h.pendingBaseSeq[runID]
	opsCopy := make([]SnapshotDeltaOp, len(ops))
	copy(opsCopy, ops)
	h.pendingOps[runID] = nil
	delete(h.pendingBaseSeq, runID)

	return &SnapshotDelta{
		RunID:     runID,
		BaseSeq:   baseSeq,
		Seq:       cloned.LastRunSeq,
		Revision:  cloned.Revision,
		UpdatedAt: cloned.UpdatedAt,
		Ops:       opsCopy,
	}, true
}

// Close 关闭快照中心。
func (h *SnapshotHub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.unsubscribe != nil {
		h.unsubscribe()
		h.unsubscribe = nil
	}
}

// ListRunning 获取所有运行中的快照
func (h *SnapshotHub) ListRunning() []*ExecutionSnapshot {
	h.mu.RLock()
	defer h.mu.RUnlock()

	result := make([]*ExecutionSnapshot, 0)
	for _, snapshot := range h.snapshots {
		if snapshot.Status == string(RunStatusRunning) {
			result = append(result, snapshot.Clone())
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
			delete(h.revisions, id)
			delete(h.pendingOps, id)
			delete(h.pendingBaseSeq, id)
		}
	}
}

func (h *SnapshotHub) EnsureRun(run *TaskRun) bool {
	if run == nil {
		return false
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	snapshot, ok := h.snapshots[run.ID]
	if !ok || snapshot == nil {
		snapshot = NewExecutionSnapshotFromRun(run)
		h.touchSnapshotLocked(run.ID, snapshot)
		h.snapshots[run.ID] = snapshot
		return true
	}

	prevStatus := snapshot.Status
	prevCurrentStage := snapshot.CurrentStage
	prevProgress := snapshot.Progress
	prevStartedAt := cloneTimePtr(snapshot.StartedAt)
	prevFinishedAt := cloneTimePtr(snapshot.FinishedAt)

	changed := applyRunModelToSnapshot(snapshot, run)
	if changed {
		baseSeq := snapshot.LastRunSeq
		h.touchSnapshotLocked(run.ID, snapshot)
		op := SnapshotDeltaOp{Type: SnapshotDeltaOpRunPatch}
		if prevStatus != snapshot.Status {
			op.Status = cloneStringPtr(snapshot.Status)
		}
		if prevCurrentStage != snapshot.CurrentStage {
			op.CurrentStage = cloneStringPtr(snapshot.CurrentStage)
		}
		if prevProgress != snapshot.Progress {
			op.Progress = cloneIntPtr(snapshot.Progress)
		}
		if !timesEqual(prevStartedAt, snapshot.StartedAt) {
			op.StartedAt = cloneTimePtr(snapshot.StartedAt)
		}
		if !timesEqual(prevFinishedAt, snapshot.FinishedAt) {
			op.FinishedAt = cloneTimePtr(snapshot.FinishedAt)
		}
		h.enqueueDeltaOpLocked(run.ID, baseSeq, op)
	}
	return changed
}

func (h *SnapshotHub) UpsertStage(runID string, stage *TaskRunStage) bool {
	if runID == "" || stage == nil {
		return false
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	snapshot := h.ensureSnapshotLocked(runID)
	stageSnapshot := NewStageSnapshotFromModel(stage)
	changed := false
	for idx := range snapshot.Stages {
		if snapshot.Stages[idx].ID != stage.ID {
			continue
		}
		if !stageSnapshotsEqual(snapshot.Stages[idx], stageSnapshot) {
			baseSeq := snapshot.LastRunSeq
			snapshot.Stages[idx] = stageSnapshot
			sortStageSnapshots(snapshot.Stages)
			h.touchSnapshotLocked(runID, snapshot)
			opStage := stageSnapshot.Clone()
			h.enqueueDeltaOpLocked(runID, baseSeq, SnapshotDeltaOp{Type: SnapshotDeltaOpStageUpsert, Stage: &opStage})
			changed = true
		}
		return changed
	}

	baseSeq := snapshot.LastRunSeq
	snapshot.Stages = append(snapshot.Stages, stageSnapshot)
	sortStageSnapshots(snapshot.Stages)
	h.touchSnapshotLocked(runID, snapshot)
	opStage := stageSnapshot.Clone()
	h.enqueueDeltaOpLocked(runID, baseSeq, SnapshotDeltaOp{Type: SnapshotDeltaOpStageUpsert, Stage: &opStage})
	return true
}

func (h *SnapshotHub) UpsertUnit(runID string, unit *TaskRunUnit) bool {
	if runID == "" || unit == nil {
		return false
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	snapshot := h.ensureSnapshotLocked(runID)
	unitSnapshot := NewUnitSnapshotFromModel(unit)
	for idx := range snapshot.Units {
		if snapshot.Units[idx].ID != unit.ID {
			continue
		}
		preserveProjectedUnitFields(&unitSnapshot, snapshot.Units[idx])
		if unitSnapshotsEqual(snapshot.Units[idx], unitSnapshot) {
			return false
		}
		baseSeq := snapshot.LastRunSeq
		snapshot.Units[idx] = unitSnapshot
		h.touchSnapshotLocked(runID, snapshot)
		opUnit := unitSnapshot.Clone()
		h.enqueueDeltaOpLocked(runID, baseSeq, SnapshotDeltaOp{Type: SnapshotDeltaOpUnitUpsert, Unit: &opUnit})
		return true
	}

	baseSeq := snapshot.LastRunSeq
	snapshot.Units = append(snapshot.Units, unitSnapshot)
	h.touchSnapshotLocked(runID, snapshot)
	opUnit := unitSnapshot.Clone()
	h.enqueueDeltaOpLocked(runID, baseSeq, SnapshotDeltaOp{Type: SnapshotDeltaOpUnitUpsert, Unit: &opUnit})
	return true
}

func (h *SnapshotHub) ApplyRunPatch(runID string, patch *RunPatch) bool {
	if runID == "" || patch == nil {
		return false
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	snapshot, ok := h.snapshots[runID]
	if !ok || snapshot == nil {
		return false
	}

	changed := false
	op := SnapshotDeltaOp{Type: SnapshotDeltaOpRunPatch}
	if patch.Status != nil && snapshot.Status != *patch.Status {
		snapshot.Status = *patch.Status
		op.Status = cloneStringPtr(snapshot.Status)
		changed = true
	}
	if patch.CurrentStage != nil && snapshot.CurrentStage != *patch.CurrentStage {
		snapshot.CurrentStage = *patch.CurrentStage
		op.CurrentStage = cloneStringPtr(snapshot.CurrentStage)
		changed = true
	}
	if patch.Progress != nil && snapshot.Progress != *patch.Progress {
		snapshot.Progress = *patch.Progress
		op.Progress = cloneIntPtr(snapshot.Progress)
		changed = true
	}
	if patch.StartedAt != nil && !timesEqual(snapshot.StartedAt, patch.StartedAt) {
		snapshot.StartedAt = cloneTimePtr(patch.StartedAt)
		op.StartedAt = cloneTimePtr(snapshot.StartedAt)
		changed = true
	}
	if patch.FinishedAt != nil && !timesEqual(snapshot.FinishedAt, patch.FinishedAt) {
		snapshot.FinishedAt = cloneTimePtr(patch.FinishedAt)
		op.FinishedAt = cloneTimePtr(snapshot.FinishedAt)
		changed = true
	}
	if changed {
		baseSeq := snapshot.LastRunSeq
		h.touchSnapshotLocked(runID, snapshot)
		h.enqueueDeltaOpLocked(runID, baseSeq, op)
	}
	return changed
}

func (h *SnapshotHub) ApplyStagePatch(runID, stageID string, patch *StagePatch) bool {
	if runID == "" || stageID == "" || patch == nil {
		return false
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	snapshot, ok := h.snapshots[runID]
	if !ok || snapshot == nil {
		return false
	}

	for idx := range snapshot.Stages {
		stage := &snapshot.Stages[idx]
		if stage.ID != stageID {
			continue
		}

		changed := false
		if patch.Status != nil && stage.Status != *patch.Status {
			stage.Status = *patch.Status
			changed = true
		}
		if patch.Progress != nil && stage.Progress != *patch.Progress {
			stage.Progress = *patch.Progress
			changed = true
		}
		if patch.CompletedUnits != nil && stage.CompletedUnits != *patch.CompletedUnits {
			stage.CompletedUnits = *patch.CompletedUnits
			changed = true
		}
		if patch.SuccessUnits != nil && stage.SuccessUnits != *patch.SuccessUnits {
			stage.SuccessUnits = *patch.SuccessUnits
			changed = true
		}
		if patch.FailedUnits != nil && stage.FailedUnits != *patch.FailedUnits {
			stage.FailedUnits = *patch.FailedUnits
			changed = true
		}
		if patch.CancelledUnits != nil && stage.CancelledUnits != *patch.CancelledUnits {
			stage.CancelledUnits = *patch.CancelledUnits
			changed = true
		}
		if patch.StartedAt != nil && !timesEqual(stage.StartedAt, patch.StartedAt) {
			stage.StartedAt = cloneTimePtr(patch.StartedAt)
			changed = true
		}
		if patch.FinishedAt != nil && !timesEqual(stage.FinishedAt, patch.FinishedAt) {
			stage.FinishedAt = cloneTimePtr(patch.FinishedAt)
			changed = true
		}
		if changed {
			baseSeq := snapshot.LastRunSeq
			h.touchSnapshotLocked(runID, snapshot)
			opStage := stage.Clone()
			h.enqueueDeltaOpLocked(runID, baseSeq, SnapshotDeltaOp{Type: SnapshotDeltaOpStageUpsert, Stage: &opStage})
		}
		return changed
	}

	return false
}

func (h *SnapshotHub) ApplyUnitPatch(runID, unitID string, patch *UnitPatch) bool {
	if runID == "" || unitID == "" || patch == nil {
		return false
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	snapshot, ok := h.snapshots[runID]
	if !ok || snapshot == nil {
		return false
	}

	for idx := range snapshot.Units {
		unit := &snapshot.Units[idx]
		if unit.ID != unitID {
			continue
		}

		changed := false
		if patch.Status != nil && unit.Status != *patch.Status {
			unit.Status = *patch.Status
			changed = true
		}
		if patch.DoneSteps != nil && unit.DoneSteps != *patch.DoneSteps {
			unit.DoneSteps = *patch.DoneSteps
			progress := 0
			if unit.TotalSteps > 0 {
				progress = unit.DoneSteps * 100 / unit.TotalSteps
			}
			unit.Progress = progress
			changed = true
		}
		if patch.ErrorMessage != nil && unit.ErrorMessage != *patch.ErrorMessage {
			unit.ErrorMessage = *patch.ErrorMessage
			changed = true
		}
		if patch.StartedAt != nil && !timesEqual(unit.StartedAt, patch.StartedAt) {
			unit.StartedAt = cloneTimePtr(patch.StartedAt)
			changed = true
		}
		if patch.FinishedAt != nil && !timesEqual(unit.FinishedAt, patch.FinishedAt) {
			unit.FinishedAt = cloneTimePtr(patch.FinishedAt)
			changed = true
		}
		if changed {
			baseSeq := snapshot.LastRunSeq
			h.touchSnapshotLocked(runID, snapshot)
			opUnit := unit.Clone()
			h.enqueueDeltaOpLocked(runID, baseSeq, SnapshotDeltaOp{Type: SnapshotDeltaOpUnitUpsert, Unit: &opUnit})
		}
		return changed
	}

	return false
}

func (h *SnapshotHub) AppendEvent(event *TaskEvent) bool {
	if event == nil || event.RunID == "" {
		return false
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	snapshot := h.ensureSnapshotLocked(event.RunID)
	if containsEventSnapshot(snapshot.Events, event.ID) {
		return false
	}

	baseSeq := snapshot.LastRunSeq
	eventSnapshot := NewEventSnapshotFromTaskEvent(event)
	snapshot.Events = append([]EventSnapshot{eventSnapshot}, snapshot.Events...)
	if len(snapshot.Events) > 50 {
		snapshot.Events = snapshot.Events[:50]
	}
	updateSnapshotSessionSeq(snapshot, event)
	h.touchSnapshotLocked(event.RunID, snapshot)
	if event.Payload == nil {
		event.Payload = make(map[string]interface{})
	}
	event.Payload["runSeq"] = snapshot.LastRunSeq
	if len(snapshot.Events) > 0 {
		snapshot.Events[0].Seq = snapshot.LastRunSeq
		opEvent := snapshot.Events[0].Clone()
		h.enqueueDeltaOpLocked(event.RunID, baseSeq, SnapshotDeltaOp{Type: SnapshotDeltaOpEventAppend, Event: &opEvent})
	}
	return true
}

func (h *SnapshotHub) ensureSnapshotLocked(runID string) *ExecutionSnapshot {
	if snapshot, ok := h.snapshots[runID]; ok && snapshot != nil {
		return snapshot
	}
	snapshot := &ExecutionSnapshot{
		RunID:                runID,
		Stages:               []StageSnapshot{},
		Units:                []UnitSnapshot{},
		Events:               []EventSnapshot{},
		LastSessionSeqByUnit: map[string]uint64{},
	}
	h.snapshots[runID] = snapshot
	return snapshot
}

func (h *SnapshotHub) bumpRevisionLocked(runID string, snapshot *ExecutionSnapshot) {
	h.revisions[runID]++
	snapshot.Revision = h.revisions[runID]
	snapshot.UpdatedAt = time.Now()
	if snapshot.LastSessionSeqByUnit == nil {
		snapshot.LastSessionSeqByUnit = map[string]uint64{}
	}
}

func (h *SnapshotHub) touchSnapshotLocked(runID string, snapshot *ExecutionSnapshot) {
	h.bumpRevisionLocked(runID, snapshot)
	if snapshot.LastRunSeq == 0 {
		snapshot.LastRunSeq = 1
		return
	}
	snapshot.LastRunSeq++
}

func (h *SnapshotHub) resetDeltaQueueLocked(runID string) {
	h.pendingOps[runID] = nil
	delete(h.pendingBaseSeq, runID)
}

func (h *SnapshotHub) enqueueDeltaOpLocked(runID string, baseSeq uint64, op SnapshotDeltaOp) {
	if runID == "" {
		return
	}
	if len(h.pendingOps[runID]) == 0 {
		h.pendingBaseSeq[runID] = baseSeq
	}
	h.pendingOps[runID] = append(h.pendingOps[runID], cloneSnapshotDeltaOp(op))
}

func cloneSnapshotDeltaOp(op SnapshotDeltaOp) SnapshotDeltaOp {
	cloned := op
	cloned.StartedAt = cloneTimePtr(op.StartedAt)
	cloned.FinishedAt = cloneTimePtr(op.FinishedAt)
	if op.Stage != nil {
		stage := op.Stage.Clone()
		cloned.Stage = &stage
	}
	if op.Unit != nil {
		unit := op.Unit.Clone()
		cloned.Unit = &unit
	}
	if op.Event != nil {
		event := op.Event.Clone()
		cloned.Event = &event
	}
	return cloned
}

func cloneStringPtr(value string) *string {
	v := value
	return &v
}

func cloneIntPtr(value int) *int {
	v := value
	return &v
}

func updateSnapshotSessionSeq(snapshot *ExecutionSnapshot, event *TaskEvent) {
	if snapshot == nil || event == nil || event.UnitID == "" {
		return
	}
	if snapshot.LastSessionSeqByUnit == nil {
		snapshot.LastSessionSeqByUnit = map[string]uint64{}
	}
	seq, ok := payloadUint64(event.Payload, "sessionSeq")
	if !ok {
		seq, ok = payloadUint64(event.Payload, "seq")
	}
	if !ok {
		return
	}
	if current := snapshot.LastSessionSeqByUnit[event.UnitID]; seq > current {
		snapshot.LastSessionSeqByUnit[event.UnitID] = seq
	}
}

func payloadUint64(payload map[string]interface{}, key string) (uint64, bool) {
	if len(payload) == 0 {
		return 0, false
	}
	value, ok := payload[key]
	if !ok {
		return 0, false
	}
	switch v := value.(type) {
	case uint64:
		return v, true
	case uint32:
		return uint64(v), true
	case uint:
		return uint64(v), true
	case int:
		if v < 0 {
			return 0, false
		}
		return uint64(v), true
	case int64:
		if v < 0 {
			return 0, false
		}
		return uint64(v), true
	case float64:
		if v < 0 {
			return 0, false
		}
		return uint64(v), true
	default:
		return 0, false
	}
}

func applyRunModelToSnapshot(snapshot *ExecutionSnapshot, run *TaskRun) bool {
	if snapshot == nil || run == nil {
		return false
	}

	changed := false
	if snapshot.RunID != run.ID {
		snapshot.RunID = run.ID
		changed = true
	}
	if snapshot.TaskName != run.Name {
		snapshot.TaskName = run.Name
		changed = true
	}
	if snapshot.RunKind != run.RunKind {
		snapshot.RunKind = run.RunKind
		changed = true
	}
	if snapshot.Status != run.Status {
		snapshot.Status = run.Status
		changed = true
	}
	if snapshot.Progress != run.Progress {
		snapshot.Progress = run.Progress
		changed = true
	}
	if snapshot.CurrentStage != run.CurrentStage {
		snapshot.CurrentStage = run.CurrentStage
		changed = true
	}
	if !timesEqual(snapshot.StartedAt, run.StartedAt) {
		snapshot.StartedAt = cloneTimePtr(run.StartedAt)
		changed = true
	}
	if !timesEqual(snapshot.FinishedAt, run.FinishedAt) {
		snapshot.FinishedAt = cloneTimePtr(run.FinishedAt)
		changed = true
	}
	if run.LastRunSeq > snapshot.LastRunSeq {
		snapshot.LastRunSeq = run.LastRunSeq
	}
	return changed
}

func stageSnapshotsEqual(left, right StageSnapshot) bool {
	return left.ID == right.ID &&
		left.Kind == right.Kind &&
		left.Name == right.Name &&
		left.Order == right.Order &&
		left.Status == right.Status &&
		left.Progress == right.Progress &&
		left.TotalUnits == right.TotalUnits &&
		left.CompletedUnits == right.CompletedUnits &&
		left.SuccessUnits == right.SuccessUnits &&
		left.FailedUnits == right.FailedUnits &&
		left.CancelledUnits == right.CancelledUnits &&
		timesEqual(left.StartedAt, right.StartedAt) &&
		timesEqual(left.FinishedAt, right.FinishedAt)
}

func unitSnapshotsEqual(left, right UnitSnapshot) bool {
	if left.ID != right.ID ||
		left.StageID != right.StageID ||
		left.Kind != right.Kind ||
		left.TargetType != right.TargetType ||
		left.TargetKey != right.TargetKey ||
		left.Status != right.Status ||
		left.Progress != right.Progress ||
		left.TotalSteps != right.TotalSteps ||
		left.DoneSteps != right.DoneSteps ||
		left.ErrorMessage != right.ErrorMessage ||
		left.LogCount != right.LogCount ||
		left.Truncated != right.Truncated ||
		left.SummaryLogPath != right.SummaryLogPath ||
		left.DetailLogPath != right.DetailLogPath ||
		left.RawLogPath != right.RawLogPath ||
		left.JournalLogPath != right.JournalLogPath ||
		!timesEqual(left.StartedAt, right.StartedAt) ||
		!timesEqual(left.FinishedAt, right.FinishedAt) {
		return false
	}
	if len(left.Logs) != len(right.Logs) {
		return false
	}
	for i := range left.Logs {
		if left.Logs[i] != right.Logs[i] {
			return false
		}
	}
	return true
}

func preserveProjectedUnitFields(target *UnitSnapshot, existing UnitSnapshot) {
	if target == nil {
		return
	}
	target.Logs = cloneStringSlice(existing.Logs)
	target.LogCount = existing.LogCount
	target.Truncated = existing.Truncated
	target.SummaryLogPath = existing.SummaryLogPath
	target.DetailLogPath = existing.DetailLogPath
	target.RawLogPath = existing.RawLogPath
	target.JournalLogPath = existing.JournalLogPath
}

func containsEventSnapshot(events []EventSnapshot, eventID string) bool {
	if eventID == "" {
		return false
	}
	for _, event := range events {
		if event.ID == eventID {
			return true
		}
	}
	return false
}

func sortStageSnapshots(stages []StageSnapshot) {
	sort.SliceStable(stages, func(i, j int) bool {
		if stages[i].Order == stages[j].Order {
			return stages[i].ID < stages[j].ID
		}
		return stages[i].Order < stages[j].Order
	})
}

func timesEqual(left, right *time.Time) bool {
	switch {
	case left == nil && right == nil:
		return true
	case left == nil || right == nil:
		return false
	default:
		return left.Equal(*right)
	}
}
