package taskexec

import (
	"fmt"
	"time"
)

type projectedStageCompletion struct {
	Status         string
	Progress       int
	CompletedUnits int
	SuccessUnits   int
	FailedUnits    int
	CancelledUnits int
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
	return projection
}
