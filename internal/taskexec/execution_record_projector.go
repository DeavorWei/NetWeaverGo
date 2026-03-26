package taskexec

import (
	"fmt"
	"time"

	"github.com/NetWeaverGo/core/internal/executor"
	"github.com/NetWeaverGo/core/internal/logger"
)

type taskexecRecordKind string

type journalRecordType string

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

	journalRecordTypeLifecycle journalRecordType = "lifecycle"
	journalRecordTypeExecutor  journalRecordType = "executor"
)

type executionJournalRecord struct {
	Source        string            `json:"source"`
	RecordType    journalRecordType `json:"recordType"`
	Kind          string            `json:"kind"`
	RunID         string            `json:"runId,omitempty"`
	StageID       string            `json:"stageId,omitempty"`
	UnitID        string            `json:"unitId,omitempty"`
	UnitKey       string            `json:"unitKey,omitempty"`
	SessionSeq    uint64            `json:"sessionSeq,omitempty"`
	EventType     string            `json:"eventType,omitempty"`
	Command       string            `json:"command,omitempty"`
	CommandIndex  int               `json:"commandIndex"`
	TotalCommands int               `json:"totalCommands,omitempty"`
	SuccessCount  int               `json:"successCount,omitempty"`
	ErrorMessage  string            `json:"errorMessage,omitempty"`
	DurationMs    int64             `json:"durationMs,omitempty"`
	Message       string            `json:"message,omitempty"`
	Timestamp     time.Time         `json:"timestamp"`
}

type executorRecordProjectionOptions struct {
	CommandNoun string
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
	record := executionJournalRecord{
		Source:        "taskexec",
		RecordType:    journalRecordTypeLifecycle,
		Kind:          string(kind),
		RunID:         scope.RunID,
		StageID:       scope.StageID,
		UnitID:        scope.UnitID,
		UnitKey:       scope.UnitKey,
		CommandIndex:  -1,
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
	journalRecord := executionJournalRecord{
		Source:        "taskexec",
		RecordType:    journalRecordTypeExecutor,
		Kind:          string(event.Kind),
		RunID:         scope.RunID,
		StageID:       scope.StageID,
		UnitID:        scope.UnitID,
		UnitKey:       scope.UnitKey,
		SessionSeq:    event.SessionSeq,
		EventType:     string(event.Type),
		Command:       event.Command,
		CommandIndex:  event.Index,
		TotalCommands: event.TotalCommands,
		ErrorMessage:  event.ErrorMessage,
		DurationMs:    event.Duration.Milliseconds(),
		Timestamp:     event.Timestamp,
	}
	if journalRecord.TotalCommands == 0 {
		journalRecord.TotalCommands = totalCommands
	}
	runtimeLogger.WriteJournal(scope, journalRecord)

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
