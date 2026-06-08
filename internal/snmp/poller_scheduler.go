// Package snmp 提供 SNMP 核心业务功能
// poller_scheduler.go 实现轮询调度器，管理定时轮询任务
package snmp

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
	"github.com/NetWeaverGo/core/internal/repository"
)

// ============================================================================
// 调度器状态
// ============================================================================

// SchedulerStatus 调度器状态
type SchedulerStatus struct {
	IsRunning    bool      `json:"isRunning"`    // 是否运行中
	TargetCount  int       `json:"targetCount"`  // 已调度目标数
	TotalPolls   int64     `json:"totalPolls"`   // 总轮询次数
	LastPollTime time.Time `json:"lastPollTime"` // 最后轮询时间
	StartTime    time.Time `json:"startTime"`    // 调度器启动时间
}

// ============================================================================
// 调度任务信息
// ============================================================================

// scheduledJob 调度任务信息
type scheduledJob struct {
	TargetID uint          // 目标 ID
	CronID   cron.EntryID  // Cron 任务 ID
	Target   *models.SNMPPollingTarget
	Template *models.SNMPPollingTemplate
	Cred     *models.SNMPCredential
}

// ============================================================================
// 轮询调度器
// ============================================================================

// PollerScheduler 轮询调度器
// 管理定时轮询任务，支持动态添加/移除目标
type PollerScheduler struct {
	poller    *Poller
	repo      repository.PollingRepository
	scheduler *cron.Cron
	notifier  EventNotifier

	// P2-18: 可配置的轮询超时时间
	pollTimeout time.Duration

	// 已调度任务映射
	jobs map[uint]*scheduledJob // targetID -> job

	// 状态
	running    bool
	startTime  time.Time
	totalPolls int64

	// 并发控制
	mu sync.RWMutex
}

// DefaultPollTimeout 默认轮询超时时间
const DefaultPollTimeout = 30 * time.Second

// NewPollerScheduler 创建轮询调度器实例
// P2-18: 添加可选的超时配置参数
func NewPollerScheduler(poller *Poller, repo repository.PollingRepository, notifier EventNotifier, timeout ...time.Duration) *PollerScheduler {
	// 创建 cron 调度器，支持秒级精度
	scheduler := cron.New(cron.WithSeconds(), cron.WithLocation(time.Local))

	pollTimeout := DefaultPollTimeout
	if len(timeout) > 0 && timeout[0] > 0 {
		pollTimeout = timeout[0]
	}

	s := &PollerScheduler{
		poller:      poller,
		repo:        repo,
		scheduler:   scheduler,
		notifier:    notifier,
		pollTimeout: pollTimeout,
		jobs:        make(map[uint]*scheduledJob),
	}

	logger.Info("SNMP-Scheduler", "-", "轮询调度器已创建 (超时: %v)", pollTimeout)
	return s
}

// ============================================================================
// 调度器生命周期
// ============================================================================

// Start 启动调度器
// 加载所有已启用的轮询目标并注册定时任务
func (s *PollerScheduler) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	startTime := time.Now()

	if s.running {
		logger.Warn("SNMP-Scheduler", "-", "调度器已在运行中")
		return nil
	}

	logger.Info("SNMP-Scheduler", "-", "正在启动轮询调度器...")

	// 加载已启用的轮询目标
	enabled := true
	targets, _, err := s.repo.ListPollingTargets(context.Background(), repository.PollingTargetFilter{
		Enabled: &enabled,
	})
	if err != nil {
		logger.Error("SNMP-Scheduler", "-", "加载轮询目标失败: %v", err)
		return fmt.Errorf("加载轮询目标失败: %w", err)
	}

	logger.Debug("SNMP-Scheduler", "-", "已加载 %d 个启用的轮询目标", len(targets))

	// 注册每个目标的定时任务
	successCount := 0
	failCount := 0
	for _, target := range targets {
		if err := s.addJob(target); err != nil {
			logger.Error("SNMP-Scheduler", "-", "注册轮询任务失败: ID=%d, IP=%s, %v",
				target.ID, target.TargetIP, err)
			failCount++
			continue
		}
		successCount++
	}

	// 启动 cron 调度器
	s.scheduler.Start()
	s.running = true
	s.startTime = time.Now()

	// 通知状态变更
	if s.notifier != nil {
		s.notifier.NotifySchedulerStatus(true)
	}

	latency := time.Since(startTime)
	logger.Info("SNMP-Scheduler", "-", "轮询调度器已启动: 注册成功=%d, 注册失败=%d, 耗时=%v",
		successCount, failCount, latency)
	return nil
}

// Stop 停止调度器
func (s *PollerScheduler) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		logger.Debug("SNMP-Scheduler", "-", "调度器未运行，无需停止")
		return nil
	}

	logger.Info("SNMP-Scheduler", "-", "正在停止轮询调度器...")

	targetCount := len(s.jobs)
	totalPolls := atomic.LoadInt64(&s.totalPolls)
	uptime := time.Since(s.startTime)

	// 停止 cron 调度器
	ctx := s.scheduler.Stop()
	logger.Debug("SNMP-Scheduler", "-", "等待正在执行的任务完成...")
	<-ctx.Done() // 等待正在执行的任务完成

	// 清空任务映射
	s.jobs = make(map[uint]*scheduledJob)
	s.running = false

	// 通知状态变更
	if s.notifier != nil {
		s.notifier.NotifySchedulerStatus(false)
	}

	logger.Info("SNMP-Scheduler", "-", "轮询调度器已停止: 原调度目标=%d, 总轮询次数=%d, 运行时长=%v",
		targetCount, totalPolls, uptime)
	return nil
}

// IsRunning 检查调度器是否运行中
func (s *PollerScheduler) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// ============================================================================
// 目标管理
// ============================================================================

// AddTarget 添加轮询目标到调度
func (s *PollerScheduler) AddTarget(target *models.SNMPPollingTarget, template *models.SNMPPollingTemplate, cred *models.SNMPCredential) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 如果目标已存在，先移除
	if _, exists := s.jobs[target.ID]; exists {
		if err := s.removeJob(target.ID); err != nil {
			logger.Warn("SNMP-Scheduler", "-", "移除已有任务失败: ID=%d, %v", target.ID, err)
		}
	}

	// 如果目标未启用，不添加
	if !target.Enabled {
		logger.Debug("SNMP-Scheduler", "-", "目标未启用，跳过调度: ID=%d", target.ID)
		return nil
	}

	// 保存关联数据
	job := &scheduledJob{
		TargetID: target.ID,
		Target:   target,
		Template: template,
		Cred:     cred,
	}

	// 生成 cron 表达式
	cronSpec := s.intervalToCronSpec(target.PollInterval)
	cronID, err := s.scheduler.AddFunc(cronSpec, s.createPollFunc(job))
	if err != nil {
		logger.Error("SNMP-Scheduler", "-", "注册 cron 任务失败: ID=%d, %v", target.ID, err)
		return fmt.Errorf("注册 cron 任务失败: %w", err)
	}

	job.CronID = cronID
	s.jobs[target.ID] = job

	logger.Info("SNMP-Scheduler", "-", "已添加轮询目标: ID=%d, IP=%s, 间隔=%ds, Cron=%s",
		target.ID, target.TargetIP, target.PollInterval, cronSpec)
	return nil
}

// RemoveTarget 从调度移除目标
func (s *PollerScheduler) RemoveTarget(targetID uint) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.removeJob(targetID)
}

// UpdateTarget 更新轮询目标
// 先移除旧任务，再添加新任务
func (s *PollerScheduler) UpdateTarget(target *models.SNMPPollingTarget, template *models.SNMPPollingTemplate, cred *models.SNMPCredential) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 移除旧任务
	if _, exists := s.jobs[target.ID]; exists {
		if err := s.removeJob(target.ID); err != nil {
			logger.Warn("SNMP-Scheduler", "-", "移除旧任务失败: ID=%d, %v", target.ID, err)
		}
	}

	// 如果目标未启用，不重新添加
	if !target.Enabled {
		logger.Debug("SNMP-Scheduler", "-", "目标未启用，不重新调度: ID=%d", target.ID)
		return nil
	}

	// 添加新任务
	job := &scheduledJob{
		TargetID: target.ID,
		Target:   target,
		Template: template,
		Cred:     cred,
	}

	cronSpec := s.intervalToCronSpec(target.PollInterval)
	cronID, err := s.scheduler.AddFunc(cronSpec, s.createPollFunc(job))
	if err != nil {
		logger.Error("SNMP-Scheduler", "-", "更新 cron 任务失败: ID=%d, %v", target.ID, err)
		return fmt.Errorf("更新 cron 任务失败: %w", err)
	}

	job.CronID = cronID
	s.jobs[target.ID] = job

	logger.Info("SNMP-Scheduler", "-", "已更新轮询目标: ID=%d, IP=%s, 间隔=%ds",
		target.ID, target.TargetIP, target.PollInterval)
	return nil
}

// ============================================================================
// 立即执行
// ============================================================================

// RunNow 立即执行一次轮询
func (s *PollerScheduler) RunNow(ctx context.Context, targetID uint) ([]*models.SNMPPollingResult, error) {
	pollStartTime := time.Now()
	logger.Info("SNMP-Scheduler", "-", "立即轮询请求: TargetID=%d", targetID)

	s.mu.RLock()
	job, exists := s.jobs[targetID]
	s.mu.RUnlock()

	if !exists {
		logger.Debug("SNMP-Scheduler", "-", "目标不在调度中，尝试从数据库加载: TargetID=%d", targetID)
		// 任务不在调度中，尝试从数据库加载
		target, err := s.repo.GetPollingTarget(ctx, targetID)
		if err != nil {
			logger.Error("SNMP-Scheduler", "-", "获取轮询目标失败: TargetID=%d, %v", targetID, err)
			return nil, fmt.Errorf("获取轮询目标失败: %w", err)
		}
		if target == nil {
			logger.Error("SNMP-Scheduler", "-", "轮询目标不存在: TargetID=%d", targetID)
			return nil, fmt.Errorf("轮询目标不存在: ID=%d", targetID)
		}

		// 加载关联的模板和凭据
		var template *models.SNMPPollingTemplate
		var cred *models.SNMPCredential

		if target.TemplateID != nil {
			template, _ = s.repo.GetPollingTemplate(ctx, *target.TemplateID)
			logger.Debug("SNMP-Scheduler", "-", "已加载模板: TargetID=%d, TemplateID=%d", targetID, *target.TemplateID)
		}
		if target.CredentialID != nil {
			cred, _ = s.repo.GetCredential(ctx, *target.CredentialID)
			logger.Debug("SNMP-Scheduler", "-", "已加载凭据: TargetID=%d, CredentialID=%d", targetID, *target.CredentialID)
		}

		job = &scheduledJob{
			TargetID: targetID,
			Target:   target,
			Template: template,
			Cred:     cred,
		}
	}

	// 执行轮询
	pollTarget := &PollTarget{
		Target:   job.Target,
		Template: job.Template,
		Cred:     job.Cred,
	}

	results, err := s.poller.PollSingle(ctx, pollTarget)
	pollLatency := time.Since(pollStartTime)

	if err != nil {
		logger.Error("SNMP-Scheduler", job.Target.TargetIP, "立即轮询失败: TargetID=%d, 耗时=%v, 错误=%v",
			targetID, pollLatency, err)
		s.updateTargetPollError(ctx, job.Target, err)
		return nil, err
	}

	logger.Info("SNMP-Scheduler", job.Target.TargetIP, "立即轮询成功: TargetID=%d, 结果数=%d, 耗时=%v",
		targetID, len(results), pollLatency)

	// 保存结果到数据库
	if len(results) > 0 {
		saveStartTime := time.Now()
		resultPtrs := make([]*models.SNMPPollingResult, len(results))
		for i := range results {
			resultPtrs[i] = results[i]
		}
		if err := s.repo.CreatePollingResults(ctx, resultPtrs); err != nil {
			logger.Error("SNMP-Scheduler", job.Target.TargetIP, "保存轮询结果失败: TargetID=%d, 耗时=%v, %v",
				targetID, time.Since(saveStartTime), err)
		} else {
			logger.Debug("SNMP-Scheduler", job.Target.TargetIP, "轮询结果已保存: TargetID=%d, 结果数=%d, 耗时=%v",
				targetID, len(results), time.Since(saveStartTime))
		}
	}

	// 更新目标最后轮询状态
	s.updateTargetPollStatus(ctx, job.Target, results)

	atomic.AddInt64(&s.totalPolls, 1)

	return results, nil
}

// RunAllNow 立即执行所有目标轮询
func (s *PollerScheduler) RunAllNow(ctx context.Context) [][]*models.SNMPPollingResult {
	s.mu.RLock()
	jobs := make([]*scheduledJob, 0, len(s.jobs))
	for _, job := range s.jobs {
		jobs = append(jobs, job)
	}
	s.mu.RUnlock()

	if len(jobs) == 0 {
		logger.Info("SNMP-Scheduler", "-", "没有已调度的轮询目标")
		return nil
	}

	logger.Info("SNMP-Scheduler", "-", "开始执行所有目标轮询: 数量=%d", len(jobs))

	// 构建轮询目标列表
	pollTargets := make([]*PollTarget, len(jobs))
	for i, job := range jobs {
		pollTargets[i] = &PollTarget{
			Target:   job.Target,
			Template: job.Template,
			Cred:     job.Cred,
		}
	}

	// 批量轮询
	allResults := s.poller.PollBatch(ctx, pollTargets)

	// 保存结果并更新状态
	for i, results := range allResults {
		if results == nil {
			// 记录失败结果
			s.updateTargetPollError(ctx, jobs[i].Target, fmt.Errorf("批量轮询发生错误"))
			continue
		}

		if len(results) == 0 {
			// 成功执行但没有数据
			s.updateTargetPollStatus(ctx, jobs[i].Target, results)
			atomic.AddInt64(&s.totalPolls, 1)
			continue
		}

		// 保存结果
		if err := s.repo.CreatePollingResults(ctx, results); err != nil {
			logger.Error("SNMP-Scheduler", "-", "保存轮询结果失败: ID=%d, %v", jobs[i].TargetID, err)
		}

		// 更新目标状态
		s.updateTargetPollStatus(ctx, jobs[i].Target, results)

		atomic.AddInt64(&s.totalPolls, 1)
	}

	return allResults
}

// ============================================================================
// 查询方法
// ============================================================================

// GetScheduledTargets 获取已调度目标 ID 列表
func (s *PollerScheduler) GetScheduledTargets() []uint {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids := make([]uint, 0, len(s.jobs))
	for id := range s.jobs {
		ids = append(ids, id)
	}
	return ids
}

// GetSchedulerStatus 获取调度器状态
func (s *PollerScheduler) GetSchedulerStatus() *SchedulerStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	status := &SchedulerStatus{
		IsRunning:   s.running,
		TargetCount: len(s.jobs),
		TotalPolls:  atomic.LoadInt64(&s.totalPolls),
	}

	if s.running {
		status.StartTime = s.startTime
	}

	return status
}

// ============================================================================
// 内部方法
// ============================================================================

// addJob 添加调度任务（需在锁内调用）
func (s *PollerScheduler) addJob(target *models.SNMPPollingTarget) error {
	// 加载关联的模板和凭据
	var template *models.SNMPPollingTemplate
	var cred *models.SNMPCredential

	if target.TemplateID != nil {
		var err error
		template, err = s.repo.GetPollingTemplate(context.Background(), *target.TemplateID)
		if err != nil {
			logger.Warn("SNMP-Scheduler", "-", "加载模板失败: TargetID=%d, TemplateID=%d, %v",
				target.ID, *target.TemplateID, err)
		}
	}

	if target.CredentialID != nil {
		var err error
		cred, err = s.repo.GetCredential(context.Background(), *target.CredentialID)
		if err != nil {
			logger.Warn("SNMP-Scheduler", "-", "加载凭据失败: TargetID=%d, CredentialID=%d, %v",
				target.ID, *target.CredentialID, err)
		}
	}

	job := &scheduledJob{
		TargetID: target.ID,
		Target:   target,
		Template: template,
		Cred:     cred,
	}

	cronSpec := s.intervalToCronSpec(target.PollInterval)
	cronID, err := s.scheduler.AddFunc(cronSpec, s.createPollFunc(job))
	if err != nil {
		return fmt.Errorf("注册 cron 任务失败: %w", err)
	}

	job.CronID = cronID
	s.jobs[target.ID] = job

	logger.Info("SNMP-Scheduler", "-", "已注册轮询任务: ID=%d, IP=%s, Cron=%s",
		target.ID, target.TargetIP, cronSpec)
	return nil
}

// removeJob 移除调度任务（需在锁内调用）
func (s *PollerScheduler) removeJob(targetID uint) error {
	job, exists := s.jobs[targetID]
	if !exists {
		return nil
	}

	// 从 cron 调度器移除
	s.scheduler.Remove(job.CronID)
	delete(s.jobs, targetID)

	logger.Info("SNMP-Scheduler", "-", "已移除轮询任务: ID=%d", targetID)
	return nil
}

// createPollFunc 创建轮询执行函数
// P2-18: 使用可配置的超时时间
func (s *PollerScheduler) createPollFunc(job *scheduledJob) func() {
	return func() {
		pollStartTime := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), s.pollTimeout)
		defer cancel()

		logger.Debug("SNMP-Scheduler", job.Target.TargetIP, "定时轮询触发: ID=%d, 超时=%v",
			job.TargetID, s.pollTimeout)

		pollTarget := &PollTarget{
			Target:   job.Target,
			Template: job.Template,
			Cred:     job.Cred,
		}

		results, err := s.poller.PollSingle(ctx, pollTarget)
		pollLatency := time.Since(pollStartTime)

		if err != nil {
			logger.Error("SNMP-Scheduler", job.Target.TargetIP, "定时轮询失败: ID=%d, 耗时=%v, 错误=%v",
				job.TargetID, pollLatency, err)

			// 更新目标状态为错误
			s.updateTargetPollError(ctx, job.Target, err)
			return
		}

		logger.Info("SNMP-Scheduler", job.Target.TargetIP, "定时轮询完成: ID=%d, 结果数=%d, 耗时=%v",
			job.TargetID, len(results), pollLatency)

		// 保存结果到数据库
		if len(results) > 0 {
			saveStartTime := time.Now()
			if err := s.repo.CreatePollingResults(ctx, results); err != nil {
				logger.Error("SNMP-Scheduler", job.Target.TargetIP, "保存轮询结果失败: ID=%d, 耗时=%v, 错误=%v",
					job.TargetID, time.Since(saveStartTime), err)
			} else {
				logger.Debug("SNMP-Scheduler", job.Target.TargetIP, "轮询结果已保存: ID=%d, 结果数=%d, 耗时=%v",
					job.TargetID, len(results), time.Since(saveStartTime))
			}
		}

		// 更新目标最后轮询状态
		s.updateTargetPollStatus(ctx, job.Target, results)

		atomic.AddInt64(&s.totalPolls, 1)
	}
}

// updateTargetPollStatus 更新目标轮询成功状态
func (s *PollerScheduler) updateTargetPollStatus(ctx context.Context, target *models.SNMPPollingTarget, results []*models.SNMPPollingResult) {
	now := time.Now()
	target.LastPollAt = &now
	target.LastPollStatus = "success"
	target.LastPollError = ""

	if err := s.repo.UpdatePollingTarget(ctx, target); err != nil {
		logger.Error("SNMP-Scheduler", "-", "更新目标轮询状态失败: ID=%d, %v", target.ID, err)
	}
}

// updateTargetPollError 更新目标轮询错误状态
func (s *PollerScheduler) updateTargetPollError(ctx context.Context, target *models.SNMPPollingTarget, pollErr error) {
	now := time.Now()
	target.LastPollAt = &now
	target.LastPollStatus = "error"
	target.LastPollError = pollErr.Error()

	if err := s.repo.UpdatePollingTarget(ctx, target); err != nil {
		logger.Error("SNMP-Scheduler", "-", "更新目标轮询错误状态失败: ID=%d, %v", target.ID, err)
	}

	// 额外保存一条失败的轮询结果，使得前端列表可以展示失败记录
	failResult := &models.SNMPPollingResult{
		TargetID:  target.ID,
		TargetIP:  target.TargetIP,
		BatchID:   fmt.Sprintf("%d-%d", target.ID, now.Unix()),
		OID:       "-",
		OIDName:   "轮询失败",
		Value:     pollErr.Error(),
		ValueType: "error",
		PollTime:  now,
		CreatedAt: now,
	}
	if err := s.repo.CreatePollingResult(ctx, failResult); err != nil {
		logger.Error("SNMP-Scheduler", "-", "保存失败的轮询结果记录失败: ID=%d, %v", target.ID, err)
	}
}

// intervalToCronSpec 将轮询间隔（秒）转换为 cron 表达式
// 支持常见的轮询间隔：30s, 1m, 2m, 5m, 10m, 15m, 30m, 1h 等
func (s *PollerScheduler) intervalToCronSpec(intervalSeconds int) string {
	switch {
	case intervalSeconds <= 0:
		// 默认 5 分钟
		return "0 */5 * * * *"
	case intervalSeconds < 60:
		// 不足 1 分钟，按秒间隔
		return fmt.Sprintf("*/%d * * * * *", intervalSeconds)
	case intervalSeconds < 3600:
		// 不足 1 小时，按分钟间隔
		minutes := intervalSeconds / 60
		if minutes < 1 {
			minutes = 1
		}
		return fmt.Sprintf("0 */%d * * * *", minutes)
	default:
		// 1 小时及以上，按小时间隔
		hours := intervalSeconds / 3600
		if hours < 1 {
			hours = 1
		}
		return fmt.Sprintf("0 0 */%d * * *", hours)
	}
}
