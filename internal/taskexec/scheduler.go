package taskexec

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/robfig/cron/v3"
	"gorm.io/gorm"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
)

// TaskSchedulerStatus 调度器状态
type TaskSchedulerStatus struct {
	IsRunning       bool       `json:"isRunning"`                  // 是否运行中
	ScheduledCount  int        `json:"scheduledCount"`             // 已调度任务组数
	TotalTriggers   int64      `json:"totalTriggers"`              // 总触发次数
	LastTriggerTime *time.Time `json:"lastTriggerTime,omitempty"`  // 最后触发时间（指针类型，未触发时为 nil）
	StartTime       time.Time  `json:"startTime"`                  // 调度器启动时间
}

// scheduledTaskGroup 调度任务信息
type scheduledTaskGroup struct {
	TaskGroupID   uint         // 任务组 ID
	CronID        cron.EntryID // Cron 任务 ID
	TaskGroupName string       // 任务组名称快照
	CronExpr      string       // Cron 表达式快照
	ScheduleType  string       // 调度类型
}

// TaskScheduler 任务调度引擎
// 基于 robfig/cron/v3 实现，管理 TaskGroup 的定时执行
type TaskScheduler struct {
	launchService *TaskLaunchService
	eventBus      *EventBus
	scheduler     *cron.Cron
	db            *gorm.DB // 数据库引用，用于调度日志等独立领域操作

	// 已调度任务映射: taskGroupID -> scheduledTaskGroup
	jobs map[uint]*scheduledTaskGroup

	// 状态
	running         bool
	startTime       time.Time
	totalTriggers   int64
	lastTriggerTime time.Time

	// 并发控制
	mu sync.RWMutex
}

// NewTaskScheduler 创建任务调度器实例
func NewTaskScheduler(launchService *TaskLaunchService, eventBus *EventBus, db *gorm.DB) *TaskScheduler {
	// 使用标准5字段Cron格式（分 时 日 月 周），使用本地时区
	scheduler := cron.New(cron.WithLocation(time.Local))

	s := &TaskScheduler{
		launchService: launchService,
		eventBus:      eventBus,
		scheduler:     scheduler,
		db:            db,
		jobs:          make(map[uint]*scheduledTaskGroup),
	}

	logger.Info("TaskScheduler", "-", "任务调度器已创建")
	return s
}

// Start 启动调度器
// 从数据库加载所有已启用调度的 TaskGroup 并注册 Cron 任务
func (s *TaskScheduler) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		logger.Warn("TaskScheduler", "-", "调度器已在运行中")
		return nil
	}

	logger.Info("TaskScheduler", "-", "正在启动任务调度器...")

	// 从数据库加载已启用调度的任务组
	groups, err := config.ListTaskGroups()
	if err != nil {
		logger.Error("TaskScheduler", "-", "加载任务组失败: %v", err)
		return fmt.Errorf("加载任务组失败: %w", err)
	}

	successCount := 0
	failCount := 0
	for _, group := range groups {
		if !group.ScheduleEnabled {
			continue
		}
		if err := s.addJobUnsafe(&group); err != nil {
			logger.Error("TaskScheduler", "-", "注册调度任务失败: ID=%d, Name=%s, %v",
				group.ID, group.Name, err)
			failCount++
			continue
		}
		successCount++
	}

	// 启动 cron 调度器
	s.scheduler.Start()
	s.running = true
	s.startTime = time.Now()

	logger.Info("TaskScheduler", "-", "任务调度器已启动: 注册成功=%d, 注册失败=%d",
		successCount, failCount)
	return nil
}

// Stop 停止调度器
func (s *TaskScheduler) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		logger.Debug("TaskScheduler", "-", "调度器未运行，无需停止")
		return nil
	}

	logger.Info("TaskScheduler", "-", "正在停止任务调度器...")

	targetCount := len(s.jobs)
	totalTriggers := atomic.LoadInt64(&s.totalTriggers)
	uptime := time.Since(s.startTime)

	// 停止 cron 调度器，等待正在执行的任务完成
	ctx := s.scheduler.Stop()
	<-ctx.Done()

	// 清空任务映射
	s.jobs = make(map[uint]*scheduledTaskGroup)
	s.running = false

	logger.Info("TaskScheduler", "-", "任务调度器已停止: 原调度任务=%d, 总触发次数=%d, 运行时长=%v",
		targetCount, totalTriggers, uptime)
	return nil
}

// ==================== 动态增删调度任务 ====================

// AddSchedule 为指定 TaskGroup 添加调度
func (s *TaskScheduler) AddSchedule(taskGroupID uint) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	group, err := config.GetTaskGroup(taskGroupID)
	if err != nil {
		return fmt.Errorf("获取任务组失败: %w", err)
	}

	return s.addJobUnsafe(group)
}

// RemoveSchedule 移除指定 TaskGroup 的调度
func (s *TaskScheduler) RemoveSchedule(taskGroupID uint) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.removeJobUnsafe(taskGroupID)
}

// UpdateSchedule 更新指定 TaskGroup 的调度（先移除再添加）
func (s *TaskScheduler) UpdateSchedule(taskGroupID uint) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 移除旧任务
	if _, exists := s.jobs[taskGroupID]; exists {
		if err := s.removeJobUnsafe(taskGroupID); err != nil {
			logger.Warn("TaskScheduler", "-", "移除旧调度失败: ID=%d, %v", taskGroupID, err)
		}
	}

	group, err := config.GetTaskGroup(taskGroupID)
	if err != nil {
		return fmt.Errorf("获取任务组失败: %w", err)
	}

	return s.addJobUnsafe(group)
}

// addJobUnsafe 添加调度任务（需在锁内调用）
func (s *TaskScheduler) addJobUnsafe(group *models.TaskGroup) error {
	if !group.ScheduleEnabled {
		logger.Debug("TaskScheduler", "-", "任务组未启用调度，跳过: ID=%d", group.ID)
		return nil
	}

	var cronSpec string
	var scheduleType string

	switch group.ScheduleType {
	case "once":
		// 一次性调度：使用绝对时间的 cron 表达式
		if group.OnceScheduledAt == nil {
			return fmt.Errorf("一次性调度缺少计划时间: ID=%d", group.ID)
		}
		if time.Now().After(*group.OnceScheduledAt) {
			logger.Info("TaskScheduler", "-", "一次性调度时间已过，跳过: ID=%d, ScheduledAt=%v",
				group.ID, group.OnceScheduledAt)
			return nil
		}
		t := *group.OnceScheduledAt
		cronSpec = fmt.Sprintf("%d %d %d %d *", t.Minute(), t.Hour(), t.Day(), t.Month())
		scheduleType = "once"

	case "cron":
		cronSpec = group.CronExpression
		scheduleType = "cron"
		if cronSpec == "" {
			return fmt.Errorf("Cron 表达式为空: ID=%d", group.ID)
		}

	default:
		return fmt.Errorf("不支持的调度类型: %s", group.ScheduleType)
	}

	job := &scheduledTaskGroup{
		TaskGroupID:   group.ID,
		TaskGroupName: group.Name,
		CronExpr:      cronSpec,
		ScheduleType:  scheduleType,
	}

	cronID, err := s.scheduler.AddFunc(cronSpec, s.createExecuteFunc(job))
	if err != nil {
		return fmt.Errorf("注册 cron 任务失败: %w", err)
	}

	job.CronID = cronID
	s.jobs[group.ID] = job

	// 更新 NextRunAt
	nextRun := s.scheduler.Entry(cronID).Next
	s.updateNextRunAt(group.ID, &nextRun)

	logger.Info("TaskScheduler", "-", "已注册调度任务: ID=%d, Name=%s, Type=%s, Cron=%s, NextRun=%v",
		group.ID, group.Name, scheduleType, cronSpec, nextRun)
	return nil
}

// removeJobUnsafe 移除调度任务（需在锁内调用）
func (s *TaskScheduler) removeJobUnsafe(taskGroupID uint) error {
	job, exists := s.jobs[taskGroupID]
	if !exists {
		return nil
	}

	s.scheduler.Remove(job.CronID)
	delete(s.jobs, taskGroupID)

	// 清除 NextRunAt
	s.updateNextRunAt(taskGroupID, nil)

	logger.Info("TaskScheduler", "-", "已移除调度任务: ID=%d", taskGroupID)
	return nil
}

// ==================== 触发执行 ====================

// createExecuteFunc 创建调度执行函数
func (s *TaskScheduler) createExecuteFunc(job *scheduledTaskGroup) func() {
	return func() {
		triggerTime := time.Now()
		logger.Info("TaskScheduler", "-", "调度触发: TaskGroupID=%d, Name=%s, TriggerTime=%v",
			job.TaskGroupID, job.TaskGroupName, triggerTime)

		// 一次性调度：立即从 cron 调度器移除（防止重复触发），但暂不更新数据库
		if job.ScheduleType == "once" {
			s.mu.Lock()
			s.scheduler.Remove(job.CronID)
			delete(s.jobs, job.TaskGroupID)
			s.mu.Unlock()
		}

		// 检查是否存在活跃运行
		if s.hasActiveRun(job.TaskGroupID) {
			reason := "存在活跃运行，跳过本次调度"
			logger.Warn("TaskScheduler", "-", "调度跳过: ID=%d, 原因=%s", job.TaskGroupID, reason)
			s.recordScheduleLog(job, triggerTime, "skipped", "", reason)
			s.emitScheduleEvent(job, EventTypeScheduleSkipped, reason)
			// 一次性调度被跳过：需要禁用，避免用户困惑
			// cron job 已从内存移除，不会再自动触发
			if job.ScheduleType == "once" {
				s.disableOnceSchedule(job.TaskGroupID)
			}
			return
		}

		// 调用 TaskLaunchService 启动任务
		runID, err := s.launchService.StartTaskGroupWithSource(context.Background(), job.TaskGroupID, "schedule")
		if err != nil {
			logger.Error("TaskScheduler", "-", "调度执行失败: ID=%d, %v", job.TaskGroupID, err)
			s.recordScheduleLog(job, triggerTime, "failed", "", err.Error())
			s.updateScheduleError(job.TaskGroupID, err.Error())
			s.emitScheduleEvent(job, EventTypeScheduleFailed, err.Error())
			// 执行失败也禁用一次性调度，避免反复失败
			if job.ScheduleType == "once" {
				s.disableOnceSchedule(job.TaskGroupID)
			}
			return
		}

		logger.Info("TaskScheduler", "-", "调度执行成功: ID=%d, RunID=%s", job.TaskGroupID, runID)
		s.recordScheduleLog(job, triggerTime, "triggered", runID, "")
		s.updateLastScheduledRun(job.TaskGroupID, runID, triggerTime)
		s.emitScheduleEvent(job, EventTypeScheduleTriggered, "")

		// 一次性调度成功执行后禁用
		if job.ScheduleType == "once" {
			s.disableOnceSchedule(job.TaskGroupID)
		}

		// 更新最后触发时间
		s.mu.Lock()
		s.lastTriggerTime = triggerTime
		s.mu.Unlock()

		atomic.AddInt64(&s.totalTriggers, 1)
	}
}

// ==================== 辅助方法 ====================

// hasActiveRun 检查指定 TaskGroup 是否有活跃运行
// 使用 ListRunningRuns 查询所有处于 pending/running 状态的运行记录
func (s *TaskScheduler) hasActiveRun(taskGroupID uint) bool {
	if s.launchService == nil || s.launchService.taskexec == nil {
		return false
	}
	runs, err := s.launchService.taskexec.GetRepository().ListRunningRuns(context.Background())
	if err != nil {
		logger.Warn("TaskScheduler", "-", "查询活跃运行失败: %v", err)
		return false
	}
	for _, run := range runs {
		if run.TaskGroupID == taskGroupID {
			return true
		}
	}
	return false
}

// disableOnceSchedule 一次性调度触发后自动禁用
func (s *TaskScheduler) disableOnceSchedule(taskGroupID uint) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 从调度器移除
	if _, exists := s.jobs[taskGroupID]; exists {
		s.removeJobUnsafe(taskGroupID)
	}

	// 更新数据库
	group, err := config.GetTaskGroup(taskGroupID)
	if err != nil {
		logger.Error("TaskScheduler", "-", "获取任务组失败: ID=%d, %v", taskGroupID, err)
		return
	}
	group.ScheduleEnabled = false
	group.ScheduleType = ""
	group.OnceScheduledAt = nil
	if _, err := config.UpdateTaskGroup(taskGroupID, *group); err != nil {
		logger.Error("TaskScheduler", "-", "禁用一次性调度失败: ID=%d, %v", taskGroupID, err)
	}

	logger.Info("TaskScheduler", "-", "一次性调度已自动禁用: ID=%d", taskGroupID)
}

// updateNextRunAt 更新下次执行时间
func (s *TaskScheduler) updateNextRunAt(taskGroupID uint, nextRun *time.Time) {
	group, err := config.GetTaskGroup(taskGroupID)
	if err != nil {
		return
	}
	group.NextRunAt = nextRun
	config.UpdateTaskGroup(taskGroupID, *group)
}

// updateLastScheduledRun 更新最近调度执行信息
func (s *TaskScheduler) updateLastScheduledRun(taskGroupID uint, runID string, triggerTime time.Time) {
	group, err := config.GetTaskGroup(taskGroupID)
	if err != nil {
		return
	}
	group.LastScheduledRunID = runID
	group.LastScheduledAt = &triggerTime
	group.ScheduleError = ""
	config.UpdateTaskGroup(taskGroupID, *group)
}

// updateScheduleError 更新调度错误信息
func (s *TaskScheduler) updateScheduleError(taskGroupID uint, errMsg string) {
	group, err := config.GetTaskGroup(taskGroupID)
	if err != nil {
		return
	}
	group.ScheduleError = errMsg
	config.UpdateTaskGroup(taskGroupID, *group)
}

// ==================== 查询方法 ====================

// GetStatus 获取调度器状态
func (s *TaskScheduler) GetStatus() *TaskSchedulerStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	status := &TaskSchedulerStatus{
		IsRunning:      s.running,
		ScheduledCount: len(s.jobs),
		TotalTriggers:  atomic.LoadInt64(&s.totalTriggers),
	}

	if s.running {
		status.StartTime = s.startTime
		if !s.lastTriggerTime.IsZero() {
			lastTime := s.lastTriggerTime
			status.LastTriggerTime = &lastTime
		}
	}

	return status
}

// GetScheduledTaskGroups 获取已调度的任务组 ID 列表
func (s *TaskScheduler) GetScheduledTaskGroups() []uint {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids := make([]uint, 0, len(s.jobs))
	for id := range s.jobs {
		ids = append(ids, id)
	}
	return ids
}

// GetScheduleInfo 获取指定任务组的调度信息
func (s *TaskScheduler) GetScheduleInfo(taskGroupID uint) (*scheduledTaskGroup, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	job, exists := s.jobs[taskGroupID]
	return job, exists
}

// GetCronEntry 获取 Cron 条目信息
func (s *TaskScheduler) GetCronEntry(id cron.EntryID) cron.Entry {
	return s.scheduler.Entry(id)
}

// ==================== 调度日志记录 ====================

// recordScheduleLog 记录调度执行日志
func (s *TaskScheduler) recordScheduleLog(
	job *scheduledTaskGroup,
	triggerTime time.Time,
	status string,
	runID string,
	reason string,
) {
	log := &TaskScheduleLog{
		TaskGroupID:    job.TaskGroupID,
		TaskGroupName:  job.TaskGroupName,
		CronExpression: job.CronExpr,
		TriggeredAt:    triggerTime,
		RunID:          runID,
		Status:         status,
		SkipReason:     reason,
		CreatedAt:      time.Now(),
	}

	if status == "triggered" && runID != "" {
		now := time.Now()
		log.ActualRunAt = &now
	}

	if status == "failed" {
		log.Error = reason
	}

	if err := s.saveScheduleLog(log); err != nil {
		logger.Error("TaskScheduler", "-", "保存调度日志失败: %v", err)
	}
}

// 设计说明：调度日志的 CRUD 操作通过注入的 db 引用直接访问，
// 而非通过 taskexec.Repository 接口，因为调度日志属于调度器自身的领域，
// 不属于任务执行的核心 Repository 职责范围。

// saveScheduleLog 保存调度日志到数据库
func (s *TaskScheduler) saveScheduleLog(log *TaskScheduleLog) error {
	if s.db == nil {
		return fmt.Errorf("数据库未初始化")
	}
	return s.db.Create(log).Error
}

// ListScheduleLogs 查询指定任务组的调度日志
func (s *TaskScheduler) ListScheduleLogs(taskGroupID uint, limit int) ([]TaskScheduleLog, error) {
	if s.db == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}

	var logs []TaskScheduleLog
	query := s.db.Where("task_group_id = ?", taskGroupID).Order("triggered_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if err := query.Find(&logs).Error; err != nil {
		return nil, err
	}
	return logs, nil
}

// ==================== 事件发布 ====================

// emitScheduleEvent 发布调度事件到 EventBus
// 使用 Emit（异步）而非 EmitSync（同步），避免阻塞 cron 调度线程
func (s *TaskScheduler) emitScheduleEvent(job *scheduledTaskGroup, eventType EventType, message string) {
	if s.eventBus == nil {
		return
	}

	event := &TaskEvent{
		ID:        newEventID(),
		RunID:     "", // 调度事件不关联具体 RunID
		Type:      eventType,
		Level:     EventLevelInfo,
		Message:   message,
		Timestamp: time.Now(),
		Payload: map[string]interface{}{
			"taskGroupId":    job.TaskGroupID,
			"taskGroupName":  job.TaskGroupName,
			"cronExpression": job.CronExpr,
			"scheduleType":   job.ScheduleType,
		},
	}

	if eventType == EventTypeScheduleFailed {
		event.Level = EventLevelError
	} else if eventType == EventTypeScheduleSkipped {
		event.Level = EventLevelWarn
	}

	s.eventBus.Emit(event)
}
