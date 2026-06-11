package ui

import (
	"context"
	"fmt"
	"strings"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
	"github.com/NetWeaverGo/core/internal/taskexec"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// ScheduleUIService 调度配置 UI 服务
// 对接前端调度配置界面，提供调度的增删改查、校验、状态查询等能力
type ScheduleUIService struct {
	scheduler *taskexec.TaskScheduler
	validator *taskexec.ScheduleValidator
	wailsApp  *application.App
}

// NewScheduleUIService 创建调度 UI 服务
func NewScheduleUIService(scheduler *taskexec.TaskScheduler) *ScheduleUIService {
	return &ScheduleUIService{
		scheduler: scheduler,
		validator: taskexec.NewScheduleValidator(),
	}
}

// ServiceStartup Wails 服务启动回调，获取全局 App 实例
func (s *ScheduleUIService) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	s.wailsApp = application.Get()
	return nil
}

// ==================== 调度配置 CRUD ====================

// UpdateScheduleConfig 更新调度配置
// 包含校验、数据库更新、调度器更新、回滚逻辑
func (s *ScheduleUIService) UpdateScheduleConfig(taskGroupID uint, req ScheduleConfigRequest) (*ScheduleConfigResponse, error) {
	// 1. 获取当前任务组
	group, err := config.GetTaskGroup(taskGroupID)
	if err != nil {
		return nil, fmt.Errorf("获取任务组失败: %w", err)
	}

	// 2. 保存原始值用于回滚
	origEnabled := group.ScheduleEnabled
	origType := group.ScheduleType
	origCron := group.CronExpression
	origOnce := group.OnceScheduledAt

	// 3. 更新字段
	group.ScheduleEnabled = req.ScheduleEnabled
	group.ScheduleType = req.ScheduleType
	group.CronExpression = req.CronExpression
	group.OnceScheduledAt = req.OnceScheduledAt

	// 4. 校验配置
	if err := s.validator.ValidateScheduleConfig(group); err != nil {
		return nil, fmt.Errorf("调度配置校验失败: %w", err)
	}

	// 5. 更新数据库
	updatedGroup, err := config.UpdateTaskGroup(taskGroupID, *group)
	if err != nil {
		return nil, fmt.Errorf("更新任务组失败: %w", err)
	}

	// 6. 尝试更新调度器
	scheduleErr := s.syncScheduler(taskGroupID, updatedGroup)
	if scheduleErr != nil {
		// 回滚数据库
		logger.Warn("ScheduleUIService", "-", "调度器更新失败，回滚数据库: ID=%d, %v", taskGroupID, scheduleErr)
		rollbackGroup := *updatedGroup
		rollbackGroup.ScheduleEnabled = origEnabled
		rollbackGroup.ScheduleType = origType
		rollbackGroup.CronExpression = origCron
		rollbackGroup.OnceScheduledAt = origOnce
		if _, rbErr := config.UpdateTaskGroup(taskGroupID, rollbackGroup); rbErr != nil {
			logger.Error("ScheduleUIService", "-", "回滚数据库失败: ID=%d, %v", taskGroupID, rbErr)
		}
		return nil, fmt.Errorf("调度器更新失败: %w", scheduleErr)
	}

	// 7. 重新读取最新数据构建响应
	group, err = config.GetTaskGroup(taskGroupID)
	if err != nil {
		return nil, fmt.Errorf("读取更新后的任务组失败: %w", err)
	}

	return s.buildScheduleResponse(group), nil
}

// GetScheduleConfig 获取调度配置
func (s *ScheduleUIService) GetScheduleConfig(taskGroupID uint) (*ScheduleConfigResponse, error) {
	group, err := config.GetTaskGroup(taskGroupID)
	if err != nil {
		return nil, fmt.Errorf("获取任务组失败: %w", err)
	}

	return s.buildScheduleResponse(group), nil
}

// ValidateCronExpression 校验 Cron 表达式
func (s *ScheduleUIService) ValidateCronExpression(expr string) (*CronValidationResult, error) {
	result := &CronValidationResult{}

	// 构造临时 TaskGroup 用于校验
	tempGroup := &models.TaskGroup{
		ScheduleEnabled: true,
		ScheduleType:    "cron",
		CronExpression:  expr,
	}

	if err := s.validator.ValidateScheduleConfig(tempGroup); err != nil {
		result.Valid = false
		result.Error = err.Error()
		result.Description = ""
		return result, nil
	}

	result.Valid = true
	result.Description = s.validator.DescribeCronExpression(expr)
	return result, nil
}

// EnableSchedule 启用调度
func (s *ScheduleUIService) EnableSchedule(taskGroupID uint) error {
	group, err := config.GetTaskGroup(taskGroupID)
	if err != nil {
		return fmt.Errorf("获取任务组失败: %w", err)
	}

	if group.ScheduleEnabled {
		return nil // 已启用，无需操作
	}

	group.ScheduleEnabled = true
	if _, err := config.UpdateTaskGroup(taskGroupID, *group); err != nil {
		return fmt.Errorf("更新任务组失败: %w", err)
	}

	// 重新读取最新数据
	group, err = config.GetTaskGroup(taskGroupID)
	if err != nil {
		return fmt.Errorf("读取任务组失败: %w", err)
	}

	if err := s.syncScheduler(taskGroupID, group); err != nil {
		// 回滚
		group.ScheduleEnabled = false
		config.UpdateTaskGroup(taskGroupID, *group)
		return fmt.Errorf("调度器注册失败: %w", err)
	}

	logger.Info("ScheduleUIService", "-", "已启用调度: TaskGroupID=%d", taskGroupID)
	return nil
}

// DisableSchedule 禁用调度
func (s *ScheduleUIService) DisableSchedule(taskGroupID uint) error {
	group, err := config.GetTaskGroup(taskGroupID)
	if err != nil {
		return fmt.Errorf("获取任务组失败: %w", err)
	}

	if !group.ScheduleEnabled {
		return nil // 已禁用，无需操作
	}

	origEnabled := group.ScheduleEnabled
	group.ScheduleEnabled = false
	if _, err := config.UpdateTaskGroup(taskGroupID, *group); err != nil {
		return fmt.Errorf("更新任务组失败: %w", err)
	}

	// 从调度器移除
	if err := s.scheduler.RemoveSchedule(taskGroupID); err != nil {
		// 回滚
		logger.Warn("ScheduleUIService", "-", "从调度器移除失败，回滚: ID=%d, %v", taskGroupID, err)
		group.ScheduleEnabled = origEnabled
		config.UpdateTaskGroup(taskGroupID, *group)
		return fmt.Errorf("调度器移除失败: %w", err)
	}

	logger.Info("ScheduleUIService", "-", "已禁用调度: TaskGroupID=%d", taskGroupID)
	return nil
}

// ==================== 状态查询 ====================

// GetSchedulerStatus 获取调度器状态
func (s *ScheduleUIService) GetSchedulerStatus() *taskexec.TaskSchedulerStatus {
	return s.scheduler.GetStatus()
}

// GetSchedulePresets 获取常用调度预设列表
func (s *ScheduleUIService) GetSchedulePresets() []SchedulePreset {
	return []SchedulePreset{
		{Label: "每5分钟", CronExpression: "*/5 * * * *", Description: "每隔5分钟执行一次"},
		{Label: "每15分钟", CronExpression: "*/15 * * * *", Description: "每隔15分钟执行一次"},
		{Label: "每30分钟", CronExpression: "*/30 * * * *", Description: "每隔30分钟执行一次"},
		{Label: "每小时", CronExpression: "0 * * * *", Description: "每小时整点执行"},
		{Label: "每天凌晨2点", CronExpression: "0 2 * * *", Description: "每天凌晨02:00执行"},
		{Label: "每天早上8点", CronExpression: "0 8 * * *", Description: "每天早上08:00执行"},
		{Label: "工作日早上9点", CronExpression: "0 9 * * 1-5", Description: "周一至周五早上09:00执行"},
		{Label: "每周一凌晨3点", CronExpression: "0 3 * * 1", Description: "每周一凌晨03:00执行"},
		{Label: "每月1号凌晨0点", CronExpression: "0 0 1 * *", Description: "每月1号凌晨00:00执行"},
	}
}

// ListScheduleLogs 获取调度日志
func (s *ScheduleUIService) ListScheduleLogs(taskGroupID uint, limit int) ([]taskexec.TaskScheduleLog, error) {
	if limit <= 0 {
		limit = 50
	}
	return s.scheduler.ListScheduleLogs(taskGroupID, limit)
}

// ==================== 内部辅助方法 ====================

// syncScheduler 同步调度器状态
// 根据任务组当前的调度配置，决定添加、更新或移除调度任务
func (s *ScheduleUIService) syncScheduler(taskGroupID uint, group *models.TaskGroup) error {
	if !group.ScheduleEnabled {
		// 已禁用，移除调度
		return s.scheduler.RemoveSchedule(taskGroupID)
	}

	// 已启用，更新调度（先移除再添加）
	return s.scheduler.UpdateSchedule(taskGroupID)
}

// buildScheduleResponse 从 TaskGroup 模型构建调度配置响应
func (s *ScheduleUIService) buildScheduleResponse(group *models.TaskGroup) *ScheduleConfigResponse {
	resp := &ScheduleConfigResponse{
		TaskGroupID:        group.ID,
		ScheduleEnabled:    group.ScheduleEnabled,
		ScheduleType:       group.ScheduleType,
		CronExpression:     group.CronExpression,
		OnceScheduledAt:    group.OnceScheduledAt,
		NextRunAt:          group.NextRunAt,
		LastScheduledAt:    group.LastScheduledAt,
		LastScheduledRunID: group.LastScheduledRunID,
		ScheduleError:      group.ScheduleError,
	}

	// 生成描述
	switch group.ScheduleType {
	case "cron":
		if strings.TrimSpace(group.CronExpression) != "" {
			resp.Description = s.validator.DescribeCronExpression(group.CronExpression)
		} else {
			resp.Description = "未配置"
		}
	case "once":
		if group.OnceScheduledAt != nil {
			resp.Description = fmt.Sprintf("一次性执行: %s", group.OnceScheduledAt.Format("2006-01-02 15:04"))
		} else {
			resp.Description = "一次性执行（未指定时间）"
		}
	default:
		if group.ScheduleEnabled {
			resp.Description = "已启用（未配置调度规则）"
		} else {
			resp.Description = "未启用调度"
		}
	}

	return resp
}

