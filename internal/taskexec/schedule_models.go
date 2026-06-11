package taskexec

import "time"

// TaskScheduleLog 调度执行日志
type TaskScheduleLog struct {
	ID             uint       `json:"id" gorm:"primaryKey;autoIncrement"`
	TaskGroupID    uint       `json:"taskGroupId" gorm:"index;not null"`
	TaskGroupName  string     `json:"taskGroupName"`
	CronExpression string     `json:"cronExpression"`
	TriggeredAt    time.Time  `json:"triggeredAt"`
	ActualRunAt    *time.Time `json:"actualRunAt,omitempty"`
	RunID          string     `json:"runId,omitempty"`
	Status         string     `json:"status"`
	SkipReason     string     `json:"skipReason,omitempty"`
	Error          string     `json:"error,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
}

func (TaskScheduleLog) TableName() string {
	return "task_schedule_logs"
}
