package taskexec

import (
	"context"
	"testing"
	"time"

	"github.com/NetWeaverGo/core/internal/executor"
)

func TestProjectStageCompletion(t *testing.T) {
	units := []TaskRunUnit{
		{Status: string(UnitStatusCompleted)},
		{Status: string(UnitStatusFailed)},
		{Status: string(UnitStatusCancelled)},
		{Status: string(UnitStatusPartial)},
	}

	projection := projectStageCompletion(units, false)
	if projection.Status != string(StageStatusPartial) {
		t.Fatalf("stage 状态错误: got=%s want=%s", projection.Status, StageStatusPartial)
	}
	if projection.Progress != 100 {
		t.Fatalf("stage 进度错误: got=%d want=100", projection.Progress)
	}
	if projection.CompletedUnits != 4 || projection.SuccessUnits != 1 || projection.FailedUnits != 1 || projection.CancelledUnits != 1 {
		t.Fatalf("stage 统计错误: %+v", projection)
	}
}

func TestProjectRunFinalStatus(t *testing.T) {
	stages := []TaskRunStage{
		{Status: string(StageStatusCompleted)},
		{Status: string(StageStatusPartial)},
	}
	if status := projectRunFinalStatus(stages); status != string(RunStatusPartial) {
		t.Fatalf("run 状态错误: got=%s want=%s", status, RunStatusPartial)
	}

	allCancelled := []TaskRunStage{
		{Status: string(StageStatusCancelled)},
		{Status: string(StageStatusCancelled)},
	}
	if status := projectRunFinalStatus(allCancelled); status != string(RunStatusCancelled) {
		t.Fatalf("run 取消状态错误: got=%s want=%s", status, RunStatusCancelled)
	}
}

func TestProjectRunProgressFromStages(t *testing.T) {
	stages := []TaskRunStage{
		{Progress: 25},
		{Progress: 75},
	}
	if progress := projectRunProgressFromStages(stages); progress != 50 {
		t.Fatalf("run 进度错误: got=%d want=50", progress)
	}
}

type projectorTestRuntimeContext struct {
	runID        string
	updatedUnits map[string]UnitPatch
	emitted      []*TaskEvent
}

func newProjectorTestRuntimeContext(runID string) *projectorTestRuntimeContext {
	return &projectorTestRuntimeContext{
		runID:        runID,
		updatedUnits: make(map[string]UnitPatch),
		emitted:      make([]*TaskEvent, 0),
	}
}

func (c *projectorTestRuntimeContext) RunID() string {
	return c.runID
}

func (c *projectorTestRuntimeContext) Context() context.Context {
	return context.Background()
}

func (c *projectorTestRuntimeContext) UpdateRun(_ *RunPatch) error {
	return nil
}

func (c *projectorTestRuntimeContext) UpdateStage(_ string, _ *StagePatch) error {
	return nil
}

func (c *projectorTestRuntimeContext) UpdateUnit(unitID string, patch *UnitPatch) error {
	if patch == nil {
		return nil
	}
	copied := UnitPatch{}
	if patch.Status != nil {
		status := *patch.Status
		copied.Status = &status
	}
	if patch.DoneSteps != nil {
		done := *patch.DoneSteps
		copied.DoneSteps = &done
	}
	if patch.ErrorMessage != nil {
		errMsg := *patch.ErrorMessage
		copied.ErrorMessage = &errMsg
	}
	if patch.StartedAt != nil {
		started := *patch.StartedAt
		copied.StartedAt = &started
	}
	if patch.FinishedAt != nil {
		finished := *patch.FinishedAt
		copied.FinishedAt = &finished
	}
	c.updatedUnits[unitID] = copied
	return nil
}

func (c *projectorTestRuntimeContext) Emit(event *TaskEvent) {
	c.emitted = append(c.emitted, event)
}

func (c *projectorTestRuntimeContext) Logger(_ LogScope) RuntimeLogger {
	return newNoopRuntimeLogger()
}

func (c *projectorTestRuntimeContext) IsCancelled() bool {
	return false
}

func TestProjectExecutorRecord_EmitsCommandLifecycleEvents(t *testing.T) {
	ctx := newProjectorTestRuntimeContext("run-1")
	handler := NewErrorHandler("run-1")
	scope := LogScope{RunID: "run-1", StageID: "stage-1", UnitID: "unit-1"}

	projectExecutorRecord(
		ctx,
		handler,
		newNoopRuntimeLogger(),
		scope,
		"stage-1",
		"unit-1",
		2,
		executor.ExecutionEvent{
			Kind:          executor.RecordCommandDispatched,
			Type:          executor.EventCmdStart,
			SessionSeq:    11,
			Command:       "display version",
			Index:         0,
			TotalCommands: 2,
			Timestamp:     time.Now(),
		},
		nil,
		executorRecordProjectionOptions{CommandNoun: "命令"},
	)

	if len(ctx.emitted) != 1 {
		t.Fatalf("dispatched 事件数量错误: got=%d want=1", len(ctx.emitted))
	}
	if ctx.emitted[0].Type != EventTypeCommandDispatched {
		t.Fatalf("dispatched 事件类型错误: got=%s want=%s", ctx.emitted[0].Type, EventTypeCommandDispatched)
	}
	if seq, ok := payloadUint64(ctx.emitted[0].Payload, "sessionSeq"); !ok || seq != 11 {
		t.Fatalf("dispatched sessionSeq 错误: payload=%v", ctx.emitted[0].Payload)
	}

	if len(ctx.updatedUnits) != 0 {
		t.Fatalf("dispatched 不应更新 unit 进度: %+v", ctx.updatedUnits)
	}

	projectExecutorRecord(
		ctx,
		handler,
		newNoopRuntimeLogger(),
		scope,
		"stage-1",
		"unit-1",
		2,
		executor.ExecutionEvent{
			Kind:          executor.RecordCommandFailed,
			Type:          executor.EventError,
			SessionSeq:    12,
			Command:       "display clock",
			Index:         1,
			TotalCommands: 2,
			ErrorMessage:  "mock failed",
			Timestamp:     time.Now(),
		},
		nil,
		executorRecordProjectionOptions{CommandNoun: "命令"},
	)

	if len(ctx.emitted) != 3 {
		t.Fatalf("failed 事件数量错误: got=%d want=3", len(ctx.emitted))
	}
	if ctx.emitted[1].Type != EventTypeCommandFailed {
		t.Fatalf("failed 生命周期事件类型错误: got=%s want=%s", ctx.emitted[1].Type, EventTypeCommandFailed)
	}
	if ctx.emitted[2].Type != EventTypeUnitProgress {
		t.Fatalf("failed 进度事件类型错误: got=%s want=%s", ctx.emitted[2].Type, EventTypeUnitProgress)
	}

	unitPatch, ok := ctx.updatedUnits["unit-1"]
	if !ok || unitPatch.DoneSteps == nil || *unitPatch.DoneSteps != 2 {
		t.Fatalf("failed 后 unit 进度更新错误: %+v", unitPatch)
	}
}
