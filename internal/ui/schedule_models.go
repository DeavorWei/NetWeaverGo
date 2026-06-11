package ui

import "time"

// ScheduleConfigRequest 调度配置请求
type ScheduleConfigRequest struct {
	ScheduleEnabled bool       `json:"scheduleEnabled"`
	ScheduleType    string     `json:"scheduleType"`
	CronExpression  string     `json:"cronExpression,omitempty"`
	OnceScheduledAt *time.Time `json:"onceScheduledAt,omitempty"`
}

// ScheduleConfigResponse 调度配置响应
type ScheduleConfigResponse struct {
	TaskGroupID        uint       `json:"taskGroupId"`
	ScheduleEnabled    bool       `json:"scheduleEnabled"`
	ScheduleType       string     `json:"scheduleType"`
	CronExpression     string     `json:"cronExpression,omitempty"`
	OnceScheduledAt    *time.Time `json:"onceScheduledAt,omitempty"`
	NextRunAt          *time.Time `json:"nextRunAt,omitempty"`
	LastScheduledAt    *time.Time `json:"lastScheduledAt,omitempty"`
	LastScheduledRunID string     `json:"lastScheduledRunId,omitempty"`
	ScheduleError      string     `json:"scheduleError,omitempty"`
	Description        string     `json:"description"`
}

// CronValidationResult Cron 表达式校验结果
type CronValidationResult struct {
	Valid       bool   `json:"valid"`
	Error       string `json:"error,omitempty"`
	Description string `json:"description"`
}

// SchedulePreset 调度预设
type SchedulePreset struct {
	Label          string `json:"label"`
	CronExpression string `json:"cronExpression"`
	Description    string `json:"description"`
}
