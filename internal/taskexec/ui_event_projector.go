package taskexec

import (
	"fmt"

	"github.com/NetWeaverGo/core/internal/logger"
)

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
