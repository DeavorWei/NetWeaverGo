package taskexec

import (
	"fmt"
	"time"

	"github.com/NetWeaverGo/core/internal/executor"
	"github.com/NetWeaverGo/core/internal/logger"
)

type taskexecRecordKind string

const (
	recordDeviceMissing        taskexecRecordKind = "device_missing"
	recordNoCommands           taskexecRecordKind = "no_commands"
	recordSessionConnecting    taskexecRecordKind = "session_connecting"
	recordSessionConnected     taskexecRecordKind = "session_connected"
	recordSessionConnectFailed taskexecRecordKind = "session_connect_failed"
	recordExecutionStarting    taskexecRecordKind = "execution_starting"
	recordExecutionCancelled   taskexecRecordKind = "execution_cancelled"
	recordExecutionFailed      taskexecRecordKind = "execution_failed"
	recordExecutionSucceeded   taskexecRecordKind = "execution_succeeded"
)

type taskexecLifecycleRecord struct {
	Source        string             `json:"source"`
	Kind          taskexecRecordKind `json:"kind"`
	RunID         string             `json:"runId"`
	StageID       string             `json:"stageId,omitempty"`
	UnitID        string             `json:"unitId,omitempty"`
	UnitKey       string             `json:"unitKey,omitempty"`
	TotalCommands int                `json:"totalCommands,omitempty"`
	SuccessCount  int                `json:"successCount,omitempty"`
	Message       string             `json:"message,omitempty"`
	Timestamp     time.Time          `json:"timestamp"`
}

type executorRecordProjectionOptions struct {
	CommandNoun string
}

type projectedStageCompletion struct {
	Status         string
	Progress       int
	CompletedUnits int
	SuccessUnits   int
	FailedUnits    int
	CancelledUnits int
}

func emitProjectedRunEvent(
	ctx RuntimeContext,
	eventType EventType,
	level EventLevel,
	message string,
) {
	logger.Verbose("TaskExec", ctx.RunID(), "发出 Run 事件: type=%s, level=%s, message=%s", eventType, level, message)
	ctx.Emit(NewTaskEvent(ctx.RunID(), eventType, message).
		WithLevel(level))
}

func emitProjectedUnitEvent(
	ctx RuntimeContext,
	stageID string,
	unitID string,
	eventType EventType,
	level EventLevel,
	message string,
) {
	logger.Verbose("TaskExec", ctx.RunID(), "发出 Unit 事件: stage=%s, unit=%s, type=%s, level=%s, message=%s",
		stageID, unitID, eventType, level, message)
	ctx.Emit(NewTaskEvent(ctx.RunID(), eventType, message).
		WithStage(stageID).
		WithUnit(unitID).
		WithLevel(level))
}

func emitProjectedStageEvent(
	ctx RuntimeContext,
	stageID string,
	eventType EventType,
	level EventLevel,
	message string,
) {
	logger.Verbose("TaskExec", ctx.RunID(), "发出 Stage 事件: stage=%s, type=%s, level=%s, message=%s",
		stageID, eventType, level, message)
	ctx.Emit(NewTaskEvent(ctx.RunID(), eventType, message).
		WithStage(stageID).
		WithLevel(level))
}

func emitProjectedStageProgressEvent(
	ctx RuntimeContext,
	stageID string,
	progress int,
	totalUnits int,
	completedCount int,
	failedCount int,
	cancelledCount int,
) {
	message := fmt.Sprintf("Stage progress %d%% (%d/%d)", progress, completedCount+failedCount+cancelledCount, totalUnits)
	event := NewTaskEvent(ctx.RunID(), EventTypeStageProgress, message).
		WithStage(stageID).
		WithLevel(EventLevelInfo).
		WithPayload("progress", progress).
		WithPayload("totalUnits", totalUnits).
		WithPayload("completedUnits", completedCount+failedCount+cancelledCount).
		WithPayload("successUnits", completedCount).
		WithPayload("failedUnits", failedCount).
		WithPayload("cancelledUnits", cancelledCount)
	logger.Verbose("TaskExec", ctx.RunID(), "发出 StageProgress 事件: stage=%s, progress=%d, completed=%d, failed=%d, cancelled=%d",
		stageID, progress, completedCount, failedCount, cancelledCount)
	ctx.Emit(event)
}

func applyProjectedStageProgress(
	handler *ErrorHandler,
	ctx RuntimeContext,
	stageID string,
	totalUnits int,
	completedCount int,
	failedCount int,
	cancelledCount int,
	progress int,
	operation string,
) {
	finished := completedCount + failedCount + cancelledCount
	handler.UpdateStageBestEffort(ctx, stageID, &StagePatch{
		CompletedUnits: &finished,
		SuccessUnits:   &completedCount,
		FailedUnits:    &failedCount,
		CancelledUnits: &cancelledCount,
		Progress:       &progress,
	}, operation)
	emitProjectedStageProgressEvent(ctx, stageID, progress, totalUnits, completedCount, failedCount, cancelledCount)
}

func applyProjectedRunProgress(handler *ErrorHandler, ctx RuntimeContext, progress int, operation string) {
	handler.UpdateRunBestEffort(ctx, &RunPatch{Progress: &progress}, operation)
	event := NewTaskEvent(ctx.RunID(), EventTypeRunProgress, fmt.Sprintf("Run progress %d%%", progress)).
		WithLevel(EventLevelInfo).
		WithPayload("progress", progress)
	logger.Verbose("TaskExec", ctx.RunID(), "发出 RunProgress 事件: progress=%d", progress)
	ctx.Emit(event)
}

func projectRunProgressFromStages(stages []TaskRunStage) int {
	if len(stages) == 0 {
		return 0
	}
	progress := 0
	for _, stage := range stages {
		progress += stage.Progress
	}
	return progress / len(stages)
}

func projectRunFinalStatus(stages []TaskRunStage) string {
	var failedCount, partialCount, cancelledCount int
	for _, stage := range stages {
		switch stage.Status {
		case string(StageStatusFailed):
			failedCount++
		case string(StageStatusPartial):
			partialCount++
		case string(StageStatusCancelled):
			cancelledCount++
		}
	}

	if cancelledCount == len(stages) && len(stages) > 0 {
		return string(RunStatusCancelled)
	}
	if failedCount == len(stages) && len(stages) > 0 {
		return string(RunStatusFailed)
	}
	if failedCount > 0 || partialCount > 0 || cancelledCount > 0 {
		return string(RunStatusPartial)
	}
	return string(RunStatusCompleted)
}

func projectStageCompletion(units []TaskRunUnit, runtimeCancelled bool) projectedStageCompletion {
	total := len(units)
	result := projectedStageCompletion{}

	for _, unit := range units {
		switch UnitStatus(unit.Status) {
		case UnitStatusCompleted:
			result.CompletedUnits++
			result.SuccessUnits++
		case UnitStatusFailed:
			result.CompletedUnits++
			result.FailedUnits++
		case UnitStatusCancelled:
			result.CompletedUnits++
			result.CancelledUnits++
		case UnitStatusPartial:
			result.CompletedUnits++
		}
	}

	if total > 0 {
		result.Progress = result.CompletedUnits * 100 / total
	}

	result.Status = string(StageStatusCompleted)
	if total == 0 {
		result.Status = string(StageStatusCompleted)
	} else if result.CancelledUnits == total {
		result.Status = string(StageStatusCancelled)
	} else if result.FailedUnits > 0 && result.SuccessUnits == 0 && result.CancelledUnits == 0 {
		result.Status = string(StageStatusFailed)
	} else if result.FailedUnits > 0 || result.CancelledUnits > 0 {
		result.Status = string(StageStatusPartial)
	}
	if runtimeCancelled && result.CompletedUnits < total {
		if result.CancelledUnits+result.FailedUnits == total && result.CancelledUnits > 0 {
			result.Status = string(StageStatusCancelled)
		} else {
			result.Status = string(StageStatusPartial)
		}
	}
	if total == 0 && runtimeCancelled {
		result.Status = string(StageStatusCancelled)
	}

	return result
}

func applyProjectedStageCompletion(
	handler *ErrorHandler,
	ctx RuntimeContext,
	stageID string,
	units []TaskRunUnit,
	runtimeCancelled bool,
	finishedAt time.Time,
	operation string,
) projectedStageCompletion {
	projection := projectStageCompletion(units, runtimeCancelled)
	handler.UpdateStageBestEffort(ctx, stageID, &StagePatch{
		Status:         &projection.Status,
		Progress:       &projection.Progress,
		CompletedUnits: &projection.CompletedUnits,
		SuccessUnits:   &projection.SuccessUnits,
		FailedUnits:    &projection.FailedUnits,
		CancelledUnits: &projection.CancelledUnits,
		FinishedAt:     &finishedAt,
	}, operation)
	logger.Verbose("TaskExec", ctx.RunID(), "应用 Stage 完成投影: stage=%s, status=%s, progress=%d, completed=%d, success=%d, failed=%d, cancelled=%d",
		stageID, projection.Status, projection.Progress, projection.CompletedUnits, projection.SuccessUnits, projection.FailedUnits, projection.CancelledUnits)
	return projection
}

func projectTaskexecLifecycleRecord(
	ctx RuntimeContext,
	runtimeLogger RuntimeLogger,
	scope LogScope,
	kind taskexecRecordKind,
	message string,
	totalCommands int,
	successCount int,
) {
	record := taskexecLifecycleRecord{
		Source:        "taskexec",
		Kind:          kind,
		RunID:         scope.RunID,
		StageID:       scope.StageID,
		UnitID:        scope.UnitID,
		UnitKey:       scope.UnitKey,
		TotalCommands: totalCommands,
		SuccessCount:  successCount,
		Message:       message,
		Timestamp:     time.Now(),
	}
	runtimeLogger.WriteJournal(scope, record)
	runtimeLogger.WriteSummary(scope, message)
	logger.Verbose("TaskExec", ctx.RunID(), "应用生命周期记录: unit=%s, kind=%s, commandTotal=%d, success=%d, message=%s",
		scope.UnitID, kind, totalCommands, successCount, message)
}

func projectExecutorRecord(
	ctx RuntimeContext,
	handler *ErrorHandler,
	runtimeLogger RuntimeLogger,
	scope LogScope,
	stageID string,
	unitID string,
	totalCommands int,
	event executor.ExecutionEvent,
	reportProgress func(doneSteps, totalSteps int),
	options executorRecordProjectionOptions,
) {
	runtimeLogger.WriteJournal(scope, event)

	logger.Verbose("TaskExec", ctx.RunID(), "应用执行记录: unit=%s, seq=%d, kind=%s, type=%s, cmdIndex=%d/%d, command=%s, err=%s",
		unitID, event.SessionSeq, event.Kind, event.Type, event.Index+1, totalCommands, event.Command, event.ErrorMessage)

	commandNoun := options.CommandNoun
	if commandNoun == "" {
		commandNoun = "命令"
	}

	switch event.Kind {
	case executor.RecordCommandDispatched:
		runtimeLogger.WriteSummary(scope, fmt.Sprintf("%s[%d/%d]开始: %s", commandNoun, event.Index+1, totalCommands, event.Command))

	case executor.RecordCommandCompleted, executor.RecordCommandFailed:
		doneSteps := event.Index + 1
		handler.UpdateUnitBestEffort(ctx, unitID, &UnitPatch{
			DoneSteps: &doneSteps,
		}, "根据执行记录更新 Unit 进度")
		if reportProgress != nil {
			reportProgress(doneSteps, totalCommands)
		}

		message := fmt.Sprintf("%s[%d/%d]完成: %s", commandNoun, doneSteps, totalCommands, event.Command)
		level := EventLevelInfo
		if event.Kind == executor.RecordCommandFailed {
			message = fmt.Sprintf("%s[%d/%d]失败: %s (%s)", commandNoun, doneSteps, totalCommands, event.Command, event.ErrorMessage)
			level = EventLevelWarn
		}
		runtimeLogger.WriteSummary(scope, message)
		ctx.Emit(NewTaskEvent(ctx.RunID(), EventTypeUnitProgress, message).
			WithStage(stageID).
			WithUnit(unitID).
			WithLevel(level).
			WithPayload("doneSteps", doneSteps).
			WithPayload("totalSteps", totalCommands).
			WithPayload("command", event.Command).
			WithPayload("sessionSeq", event.SessionSeq).
			WithPayload("kind", string(event.Kind)))

	case executor.RecordExecutionCompleted:
		// 执行计划结束由 Unit 最终状态流转统一处理，这里只保留 journal 记录。
	}
}
