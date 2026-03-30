package taskexec

import (
	"time"
)

// ExecutionSnapshot 执行快照 - 前端消费的唯一视图
type ExecutionSnapshot struct {
	RunID                string            `json:"runId"`
	TaskName             string            `json:"taskName"`
	RunKind              string            `json:"runKind"`  // normal / topology
	Status               string            `json:"status"`   // pending / running / completed / partial / failed / cancelled
	Progress             int               `json:"progress"` // 0-100
	Revision             uint64            `json:"revision"`
	LastRunSeq           uint64            `json:"lastRunSeq"`
	UpdatedAt            time.Time         `json:"updatedAt"`
	CurrentStage         string            `json:"currentStage"`
	Stages               []StageSnapshot   `json:"stages"`
	Units                []UnitSnapshot    `json:"units"`
	StartedAt            *time.Time        `json:"startedAt"`
	FinishedAt           *time.Time        `json:"finishedAt"`
	Events               []EventSnapshot   `json:"events"` // 最近事件
	LastSessionSeqByUnit map[string]uint64 `json:"lastSessionSeqByUnit,omitempty"`
}

// StageSnapshot Stage快照
type StageSnapshot struct {
	ID             string     `json:"id"`
	Kind           string     `json:"kind"`
	Name           string     `json:"name"`
	Order          int        `json:"order"`
	Status         string     `json:"status"`
	Progress       int        `json:"progress"` // 0-100
	TotalUnits     int        `json:"totalUnits"`
	CompletedUnits int        `json:"completedUnits"`
	SuccessUnits   int        `json:"successUnits"`
	FailedUnits    int        `json:"failedUnits"`
	CancelledUnits int        `json:"cancelledUnits"`
	StartedAt      *time.Time `json:"startedAt"`
	FinishedAt     *time.Time `json:"finishedAt"`
}

// UnitSnapshot Unit快照
type UnitSnapshot struct {
	ID             string     `json:"id"`
	StageID        string     `json:"stageId"`
	Kind           string     `json:"kind"`
	TargetType     string     `json:"targetType"`
	TargetKey      string     `json:"targetKey"`
	Status         string     `json:"status"`
	Progress       int        `json:"progress"` // 基于 steps 计算
	TotalSteps     int        `json:"totalSteps"`
	DoneSteps      int        `json:"doneSteps"`
	ErrorMessage   string     `json:"errorMessage"`
	Logs           []string   `json:"logs,omitempty"`
	LogCount       int        `json:"logCount"`
	Truncated      bool       `json:"truncated"`
	SummaryLogPath string     `json:"summaryLogPath,omitempty"`
	DetailLogPath  string     `json:"detailLogPath,omitempty"`
	RawLogPath     string     `json:"rawLogPath,omitempty"`
	JournalLogPath string     `json:"journalLogPath,omitempty"`
	StartedAt      *time.Time `json:"startedAt"`
	FinishedAt     *time.Time `json:"finishedAt"`
}

// EventSnapshot 事件快照
type EventSnapshot struct {
	ID        string    `json:"id"`
	Seq       uint64    `json:"seq"`
	Type      string    `json:"type"`
	Level     string    `json:"level"`
	StageID   string    `json:"stageId,omitempty"`
	UnitID    string    `json:"unitId,omitempty"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// SnapshotDeltaOpType 增量操作类型。
type SnapshotDeltaOpType string

const (
	SnapshotDeltaOpRunPatch    SnapshotDeltaOpType = "run_patch"
	SnapshotDeltaOpStageUpsert SnapshotDeltaOpType = "stage_upsert"
	SnapshotDeltaOpUnitUpsert  SnapshotDeltaOpType = "unit_upsert"
	SnapshotDeltaOpEventAppend SnapshotDeltaOpType = "event_append"
)

// SnapshotDeltaOp 快照增量操作。
type SnapshotDeltaOp struct {
	Type SnapshotDeltaOpType `json:"type"`

	// run_patch
	Status       *string    `json:"status,omitempty"`
	CurrentStage *string    `json:"currentStage,omitempty"`
	Progress     *int       `json:"progress,omitempty"`
	StartedAt    *time.Time `json:"startedAt,omitempty"`
	FinishedAt   *time.Time `json:"finishedAt,omitempty"`

	// stage/unit/event
	Stage *StageSnapshot `json:"stage,omitempty"`
	Unit  *UnitSnapshot  `json:"unit,omitempty"`
	Event *EventSnapshot `json:"event,omitempty"`
}

// SnapshotDelta 快照增量事件。
// BaseSeq/Seq 描述该批增量作用区间，客户端可据此做 gap 检测。
type SnapshotDelta struct {
	RunID     string             `json:"runId"`
	BaseSeq   uint64             `json:"baseSeq"`
	Seq       uint64             `json:"seq"`
	Revision  uint64             `json:"revision"`
	UpdatedAt time.Time          `json:"updatedAt"`
	Ops       []SnapshotDeltaOp  `json:"ops,omitempty"`
	Snapshot  *ExecutionSnapshot `json:"snapshot,omitempty"`
}

// ArtifactSnapshot 产物快照
type ArtifactSnapshot struct {
	ID        string                 `json:"id"`
	StageID   string                 `json:"stageId,omitempty"`
	UnitID    string                 `json:"unitId,omitempty"`
	Type      string                 `json:"type"`
	Key       string                 `json:"key"`
	FilePath  string                 `json:"filePath,omitempty"`
	Meta      map[string]interface{} `json:"meta,omitempty"`
	CreatedAt time.Time              `json:"createdAt"`
}

// RunSummary Run摘要 - 用于历史记录
type RunSummary struct {
	RunID            string    `json:"runId"`
	TaskGroupID      uint      `json:"taskGroupId"`
	TaskName         string    `json:"taskName"`
	TaskNameSnapshot string    `json:"taskNameSnapshot"`
	RunKind          string    `json:"runKind"`
	Status           string    `json:"status"`
	Progress         int       `json:"progress"`
	TotalStages      int       `json:"totalStages"`
	CompletedStages  int       `json:"completedStages"`
	TotalUnits       int       `json:"totalUnits"`
	SuccessUnits     int       `json:"successUnits"`
	FailedUnits      int       `json:"failedUnits"`
	StartedAt        time.Time `json:"startedAt"`
	FinishedAt       time.Time `json:"finishedAt"`
	DurationMs       int64     `json:"durationMs"`
}

// SnapshotBuilder 快照构建器
type SnapshotBuilder struct{}

// NewSnapshotBuilder 创建快照构建器
func NewSnapshotBuilder() *SnapshotBuilder {
	return &SnapshotBuilder{}
}

func NewExecutionSnapshotFromRun(run *TaskRun) *ExecutionSnapshot {
	if run == nil {
		return nil
	}
	return &ExecutionSnapshot{
		RunID:                run.ID,
		TaskName:             run.Name,
		RunKind:              run.RunKind,
		Status:               run.Status,
		Progress:             run.Progress,
		Revision:             run.LastRunSeq,
		LastRunSeq:           run.LastRunSeq,
		UpdatedAt:            time.Now(),
		CurrentStage:         run.CurrentStage,
		StartedAt:            run.StartedAt,
		FinishedAt:           run.FinishedAt,
		Stages:               []StageSnapshot{},
		Units:                []UnitSnapshot{},
		Events:               []EventSnapshot{},
		LastSessionSeqByUnit: map[string]uint64{},
	}
}

func NewStageSnapshotFromModel(stage *TaskRunStage) StageSnapshot {
	if stage == nil {
		return StageSnapshot{}
	}
	return StageSnapshot{
		ID:             stage.ID,
		Kind:           stage.StageKind,
		Name:           stage.StageName,
		Order:          stage.StageOrder,
		Status:         stage.Status,
		Progress:       stage.Progress,
		TotalUnits:     stage.TotalUnits,
		CompletedUnits: stage.CompletedUnits,
		SuccessUnits:   stage.SuccessUnits,
		FailedUnits:    stage.FailedUnits,
		CancelledUnits: stage.CancelledUnits,
		StartedAt:      stage.StartedAt,
		FinishedAt:     stage.FinishedAt,
	}
}

func NewUnitSnapshotFromModel(unit *TaskRunUnit) UnitSnapshot {
	if unit == nil {
		return UnitSnapshot{}
	}
	progress := 0
	if unit.TotalSteps > 0 {
		progress = unit.DoneSteps * 100 / unit.TotalSteps
	}
	return UnitSnapshot{
		ID:           unit.ID,
		StageID:      unit.TaskRunStageID,
		Kind:         unit.UnitKind,
		TargetType:   unit.TargetType,
		TargetKey:    unit.TargetKey,
		Status:       unit.Status,
		Progress:     progress,
		TotalSteps:   unit.TotalSteps,
		DoneSteps:    unit.DoneSteps,
		ErrorMessage: unit.ErrorMessage,
		StartedAt:    unit.StartedAt,
		FinishedAt:   unit.FinishedAt,
	}
}

func NewEventSnapshotFromTaskEvent(event *TaskEvent) EventSnapshot {
	if event == nil {
		return EventSnapshot{}
	}
	seq := uint64(0)
	if runSeq, ok := payloadUint64(event.Payload, "runSeq"); ok {
		seq = runSeq
	}
	return EventSnapshot{
		ID:        event.ID,
		Seq:       seq,
		Type:      string(event.Type),
		Level:     string(event.Level),
		StageID:   event.StageID,
		UnitID:    event.UnitID,
		Message:   event.Message,
		Timestamp: event.Timestamp,
	}
}

func cloneTimePtr(src *time.Time) *time.Time {
	if src == nil {
		return nil
	}
	value := *src
	return &value
}

func cloneStringSlice(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	cloned := make([]string, len(values))
	copy(cloned, values)
	return cloned
}

func cloneUint64Map(values map[string]uint64) map[string]uint64 {
	if len(values) == 0 {
		return nil
	}
	cloned := make(map[string]uint64, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func cloneStageSnapshots(stages []StageSnapshot) []StageSnapshot {
	if len(stages) == 0 {
		return nil
	}
	cloned := make([]StageSnapshot, len(stages))
	for i := range stages {
		cloned[i] = stages[i]
		cloned[i].StartedAt = cloneTimePtr(stages[i].StartedAt)
		cloned[i].FinishedAt = cloneTimePtr(stages[i].FinishedAt)
	}
	return cloned
}

func cloneUnitSnapshots(units []UnitSnapshot) []UnitSnapshot {
	if len(units) == 0 {
		return nil
	}
	cloned := make([]UnitSnapshot, len(units))
	for i := range units {
		cloned[i] = units[i]
		cloned[i].Logs = cloneStringSlice(units[i].Logs)
		cloned[i].StartedAt = cloneTimePtr(units[i].StartedAt)
		cloned[i].FinishedAt = cloneTimePtr(units[i].FinishedAt)
	}
	return cloned
}

func cloneEventSnapshots(events []EventSnapshot) []EventSnapshot {
	if len(events) == 0 {
		return nil
	}
	cloned := make([]EventSnapshot, len(events))
	copy(cloned, events)
	return cloned
}

func (s StageSnapshot) Clone() StageSnapshot {
	s.StartedAt = cloneTimePtr(s.StartedAt)
	s.FinishedAt = cloneTimePtr(s.FinishedAt)
	return s
}

func (u UnitSnapshot) Clone() UnitSnapshot {
	u.Logs = cloneStringSlice(u.Logs)
	u.StartedAt = cloneTimePtr(u.StartedAt)
	u.FinishedAt = cloneTimePtr(u.FinishedAt)
	return u
}

func (e EventSnapshot) Clone() EventSnapshot {
	return e
}

func (s *ExecutionSnapshot) Clone() *ExecutionSnapshot {
	if s == nil {
		return nil
	}
	cloned := *s
	cloned.StartedAt = cloneTimePtr(s.StartedAt)
	cloned.FinishedAt = cloneTimePtr(s.FinishedAt)
	cloned.Stages = cloneStageSnapshots(s.Stages)
	cloned.Units = cloneUnitSnapshots(s.Units)
	cloned.Events = cloneEventSnapshots(s.Events)
	cloned.LastSessionSeqByUnit = cloneUint64Map(s.LastSessionSeqByUnit)
	return &cloned
}

// Build 从运行状态构建快照
func (b *SnapshotBuilder) Build(run *TaskRun, stages []TaskRunStage, units []TaskRunUnit, events []TaskRunEvent) *ExecutionSnapshot {
	snapshot := &ExecutionSnapshot{
		RunID:                run.ID,
		TaskName:             run.Name,
		RunKind:              run.RunKind,
		Status:               run.Status,
		Progress:             run.Progress,
		Revision:             run.LastRunSeq,
		LastRunSeq:           run.LastRunSeq,
		UpdatedAt:            time.Now(),
		CurrentStage:         run.CurrentStage,
		StartedAt:            run.StartedAt,
		FinishedAt:           run.FinishedAt,
		Stages:               make([]StageSnapshot, 0, len(stages)),
		Units:                make([]UnitSnapshot, 0, len(units)),
		Events:               make([]EventSnapshot, 0, len(events)),
		LastSessionSeqByUnit: map[string]uint64{},
	}

	// 构建 Stage 快照
	for _, stage := range stages {
		stageCopy := stage
		snapshot.Stages = append(snapshot.Stages, NewStageSnapshotFromModel(&stageCopy))
	}

	// 构建 Unit 快照
	for _, unit := range units {
		unitCopy := unit
		snapshot.Units = append(snapshot.Units, NewUnitSnapshotFromModel(&unitCopy))
	}

	// 构建 Event 快照
	for _, event := range events {
		snapshot.Events = append(snapshot.Events, EventSnapshot{
			ID:        event.ID,
			Seq:       event.RunSeq,
			Type:      event.EventType,
			Level:     event.EventLevel,
			StageID:   event.StageID,
			UnitID:    event.UnitID,
			Message:   event.Message,
			Timestamp: event.CreatedAt,
		})
		if event.RunSeq > snapshot.LastRunSeq {
			snapshot.LastRunSeq = event.RunSeq
		}
		if event.UnitID != "" && event.SessionSeq > snapshot.LastSessionSeqByUnit[event.UnitID] {
			snapshot.LastSessionSeqByUnit[event.UnitID] = event.SessionSeq
		}
	}

	if snapshot.Revision < snapshot.LastRunSeq {
		snapshot.Revision = snapshot.LastRunSeq
	}
	return snapshot
}

// BuildSummary 构建运行摘要
func (b *SnapshotBuilder) BuildSummary(run *TaskRun, stages []TaskRunStage) *RunSummary {
	completedStages := 0
	for _, s := range stages {
		if StageStatus(s.Status).IsTerminal() {
			completedStages++
		}
	}

	durationMs := int64(0)
	if run.StartedAt != nil && run.FinishedAt != nil {
		durationMs = run.FinishedAt.Sub(*run.StartedAt).Milliseconds()
	}

	startedAt := time.Time{}
	if run.StartedAt != nil {
		startedAt = *run.StartedAt
	}
	finishedAt := time.Time{}
	if run.FinishedAt != nil {
		finishedAt = *run.FinishedAt
	}

	return &RunSummary{
		RunID:            run.ID,
		TaskGroupID:      run.TaskGroupID,
		TaskName:         run.Name,
		TaskNameSnapshot: run.TaskNameSnapshot,
		RunKind:          run.RunKind,
		Status:           run.Status,
		Progress:         run.Progress,
		TotalStages:      len(stages),
		CompletedStages:  completedStages,
		StartedAt:        startedAt,
		FinishedAt:       finishedAt,
		DurationMs:       durationMs,
	}
}
