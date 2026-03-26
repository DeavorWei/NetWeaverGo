package taskexec

import (
	"time"
)

// ExecutionSnapshot 执行快照 - 前端消费的唯一视图
type ExecutionSnapshot struct {
	RunID        string          `json:"runId"`
	TaskName     string          `json:"taskName"`
	RunKind      string          `json:"runKind"`  // normal / topology
	Status       string          `json:"status"`   // pending / running / completed / partial / failed / cancelled
	Progress     int             `json:"progress"` // 0-100
	CurrentStage string          `json:"currentStage"`
	Stages       []StageSnapshot `json:"stages"`
	Units        []UnitSnapshot  `json:"units"`
	StartedAt    *time.Time      `json:"startedAt"`
	FinishedAt   *time.Time      `json:"finishedAt"`
	Events       []EventSnapshot `json:"events"` // 最近事件
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
	ID           string     `json:"id"`
	StageID      string     `json:"stageId"`
	Kind         string     `json:"kind"`
	TargetType   string     `json:"targetType"`
	TargetKey    string     `json:"targetKey"`
	Status       string     `json:"status"`
	Progress     int        `json:"progress"` // 基于 steps 计算
	TotalSteps   int        `json:"totalSteps"`
	DoneSteps    int        `json:"doneSteps"`
	ErrorMessage string     `json:"errorMessage"`
	StartedAt    *time.Time `json:"startedAt"`
	FinishedAt   *time.Time `json:"finishedAt"`
}

// EventSnapshot 事件快照
type EventSnapshot struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Level     string    `json:"level"`
	StageID   string    `json:"stageId,omitempty"`
	UnitID    string    `json:"unitId,omitempty"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
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

// Build 从运行状态构建快照
func (b *SnapshotBuilder) Build(run *TaskRun, stages []TaskRunStage, units []TaskRunUnit, events []TaskRunEvent) *ExecutionSnapshot {
	snapshot := &ExecutionSnapshot{
		RunID:        run.ID,
		TaskName:     run.Name,
		RunKind:      run.RunKind,
		Status:       run.Status,
		Progress:     run.Progress,
		CurrentStage: run.CurrentStage,
		StartedAt:    run.StartedAt,
		FinishedAt:   run.FinishedAt,
		Stages:       make([]StageSnapshot, 0, len(stages)),
		Units:        make([]UnitSnapshot, 0, len(units)),
		Events:       make([]EventSnapshot, 0, len(events)),
	}

	// 构建 Stage 快照
	for _, stage := range stages {
		snapshot.Stages = append(snapshot.Stages, StageSnapshot{
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
		})
	}

	// 构建 Unit 快照
	for _, unit := range units {
		progress := 0
		if unit.TotalSteps > 0 {
			progress = unit.DoneSteps * 100 / unit.TotalSteps
		}
		snapshot.Units = append(snapshot.Units, UnitSnapshot{
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
		})
	}

	// 构建 Event 快照
	for _, event := range events {
		snapshot.Events = append(snapshot.Events, EventSnapshot{
			ID:        event.ID,
			Type:      event.EventType,
			Level:     event.EventLevel,
			StageID:   event.StageID,
			UnitID:    event.UnitID,
			Message:   event.Message,
			Timestamp: event.CreatedAt,
		})
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
