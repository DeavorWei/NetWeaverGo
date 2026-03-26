package taskexec

import (
	"context"
	"encoding/json"

	"github.com/NetWeaverGo/core/internal/logger"
)

func NewTaskEventRepositoryProjector(repo Repository) EventHandler {
	return func(event *TaskEvent) {
		if repo == nil || event == nil {
			return
		}

		payloadJSON := ""
		if len(event.Payload) > 0 {
			payloadBytes, err := json.Marshal(event.Payload)
			if err != nil {
				logger.Warn("TaskExec", event.RunID, "序列化任务事件负载失败: event=%s, err=%v", event.Type, err)
			} else {
				payloadJSON = string(payloadBytes)
			}
		}

		record := &TaskRunEvent{
			ID:          event.ID,
			TaskRunID:   event.RunID,
			StageID:     event.StageID,
			UnitID:      event.UnitID,
			EventType:   string(event.Type),
			EventLevel:  string(event.Level),
			Message:     event.Message,
			PayloadJSON: payloadJSON,
			CreatedAt:   event.Timestamp,
		}

		if err := repo.CreateEvent(context.Background(), record); err != nil {
			logger.Warn("TaskExec", event.RunID, "持久化任务事件失败: type=%s, stage=%s, unit=%s, err=%v",
				event.Type, event.StageID, event.UnitID, err)
			return
		}

		logger.Verbose("TaskExec", event.RunID, "任务事件已持久化: type=%s, stage=%s, unit=%s", event.Type, event.StageID, event.UnitID)
	}
}
