// Package snmp 提供 SNMP 核心业务功能
// cleanup.go 实现数据自动清理任务
package snmp

import (
	"context"
	"sync"
	"time"

	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/repository"
)

// ============================================================================
// 清理配置
// ============================================================================

// CleanupConfig 数据清理配置
type CleanupConfig struct {
	TrapRetentionDays         int  `json:"trapRetentionDays"`         // Trap 保留天数（默认 30）
	PollResultRetentionDays   int  `json:"pollResultRetentionDays"`   // 轮询结果保留天数（默认 7）
	CleanupIntervalHours      int  `json:"cleanupIntervalHours"`      // 清理间隔（小时，默认 24）
	Enabled                   bool `json:"enabled"`                   // 是否启用自动清理（默认 true）
}

// DefaultCleanupConfig 默认清理配置
func DefaultCleanupConfig() CleanupConfig {
	return CleanupConfig{
		TrapRetentionDays:       30,
		PollResultRetentionDays: 7,
		CleanupIntervalHours:    24,
		Enabled:                 true,
	}
}

// ============================================================================
// 清理结果
// ============================================================================

// CleanupResult 清理执行结果
type CleanupResult struct {
	TrapDeleted      int64 `json:"trapDeleted"`      // 删除的 Trap 记录数
	PollResultDeleted int64 `json:"pollResultDeleted"` // 删除的轮询结果数
	ExecutedAt       string `json:"executedAt"`      // 执行时间
	DurationMs       int64  `json:"durationMs"`      // 执行耗时（毫秒）
}

// ============================================================================
// DataCleaner 数据清理器
// ============================================================================

// DataCleaner 数据清理器
// 定期清理过期的 Trap 记录和轮询结果
type DataCleaner struct {
	trapRepo repository.TrapRepository
	pollRepo repository.PollingRepository
	config   CleanupConfig
	ticker   *time.Ticker
	stopCh   chan struct{}
	running  bool
	mu       sync.Mutex
}

// NewDataCleaner 创建数据清理器实例
func NewDataCleaner(
	trapRepo repository.TrapRepository,
	pollRepo repository.PollingRepository,
	config CleanupConfig,
) *DataCleaner {
	// 应用默认值
	if config.TrapRetentionDays <= 0 {
		config.TrapRetentionDays = 30
	}
	if config.PollResultRetentionDays <= 0 {
		config.PollResultRetentionDays = 7
	}
	if config.CleanupIntervalHours <= 0 {
		config.CleanupIntervalHours = 24
	}

	return &DataCleaner{
		trapRepo: trapRepo,
		pollRepo: pollRepo,
		config:   config,
		stopCh:   make(chan struct{}),
	}
}

// Start 启动清理任务
func (c *DataCleaner) Start() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.running {
		logger.Warn("SNMP-Cleanup", "-", "清理任务已在运行")
		return nil
	}

	if !c.config.Enabled {
		logger.Info("SNMP-Cleanup", "-", "自动清理已禁用，跳过启动")
		return nil
	}

	// 创建定时器
	interval := time.Duration(c.config.CleanupIntervalHours) * time.Hour
	c.ticker = time.NewTicker(interval)
	c.running = true

	// 启动后台清理循环
	go c.cleanupLoop()

	logger.Info("SNMP-Cleanup", "-", "数据清理任务已启动: 间隔=%d小时, Trap保留=%d天, 轮询保留=%d天",
		c.config.CleanupIntervalHours, c.config.TrapRetentionDays, c.config.PollResultRetentionDays)

	return nil
}

// Stop 停止清理任务
func (c *DataCleaner) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.running {
		return nil
	}

	// 发送停止信号
	close(c.stopCh)

	// 停止定时器
	if c.ticker != nil {
		c.ticker.Stop()
		c.ticker = nil
	}

	c.running = false
	logger.Info("SNMP-Cleanup", "-", "数据清理任务已停止")

	return nil
}

// IsRunning 检查清理任务是否在运行
func (c *DataCleaner) IsRunning() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.running
}

// GetConfig 获取当前清理配置
func (c *DataCleaner) GetConfig() CleanupConfig {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.config
}

// UpdateConfig 更新清理配置
// 更新后需要重启清理任务才能生效
func (c *DataCleaner) UpdateConfig(config CleanupConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 应用默认值
	if config.TrapRetentionDays <= 0 {
		config.TrapRetentionDays = 30
	}
	if config.PollResultRetentionDays <= 0 {
		config.PollResultRetentionDays = 7
	}
	if config.CleanupIntervalHours <= 0 {
		config.CleanupIntervalHours = 24
	}

	c.config = config
	logger.Info("SNMP-Cleanup", "-", "清理配置已更新: 间隔=%d小时, Trap保留=%d天, 轮询保留=%d天",
		config.CleanupIntervalHours, config.TrapRetentionDays, config.PollResultRetentionDays)
}

// cleanupLoop 清理循环
func (c *DataCleaner) cleanupLoop() {
	for {
		select {
		case <-c.ticker.C:
			// 定时触发清理
			c.executeCleanup()
		case <-c.stopCh:
			// 收到停止信号
			return
		}
	}
}

// executeCleanup 执行清理操作
func (c *DataCleaner) executeCleanup() {
	startTime := time.Now()
	logger.Info("SNMP-Cleanup", "-", "开始执行数据清理...")

	trapDeleted, pollDeleted, err := c.RunCleanup()
	if err != nil {
		logger.Error("SNMP-Cleanup", "-", "数据清理失败: %v", err)
		return
	}

	duration := time.Since(startTime).Milliseconds()
	logger.Info("SNMP-Cleanup", "-", "数据清理完成: Trap删除=%d, 轮询删除=%d, 耗时=%dms",
		trapDeleted, pollDeleted, duration)
}

// RunCleanup 执行一次清理操作（可手动调用）
func (c *DataCleaner) RunCleanup() (trapDeleted, pollDeleted int64, err error) {
	c.mu.Lock()
	config := c.config
	c.mu.Unlock()

	ctx := context.Background()

	// 清理过期 Trap 记录
	if config.TrapRetentionDays > 0 {
		trapBefore := time.Now().AddDate(0, 0, -config.TrapRetentionDays)
		trapDeleted, err = c.trapRepo.DeleteTrapsBefore(ctx, trapBefore)
		if err != nil {
			logger.Error("SNMP-Cleanup", "-", "清理 Trap 记录失败: %v", err)
			return 0, 0, err
		}
		logger.Debug("SNMP-Cleanup", "-", "清理 Trap: 截止时间=%s, 删除数=%d", trapBefore.Format("2006-01-02"), trapDeleted)
	}

	// 清理过期轮询结果
	if config.PollResultRetentionDays > 0 {
		pollBefore := time.Now().AddDate(0, 0, -config.PollResultRetentionDays)
		pollDeleted, err = c.pollRepo.DeletePollingResultsBefore(ctx, pollBefore)
		if err != nil {
			logger.Error("SNMP-Cleanup", "-", "清理轮询结果失败: %v", err)
			return trapDeleted, 0, err
		}
		logger.Debug("SNMP-Cleanup", "-", "清理轮询结果: 截止时间=%s, 删除数=%d", pollBefore.Format("2006-01-02"), pollDeleted)
	}

	return trapDeleted, pollDeleted, nil
}

// RunCleanupWithResult 执行清理并返回详细结果
func (c *DataCleaner) RunCleanupWithResult() *CleanupResult {
	startTime := time.Now()

	trapDeleted, pollDeleted, err := c.RunCleanup()
	if err != nil {
		logger.Error("SNMP-Cleanup", "-", "清理执行失败: %v", err)
	}

	duration := time.Since(startTime).Milliseconds()

	return &CleanupResult{
		TrapDeleted:       trapDeleted,
		PollResultDeleted: pollDeleted,
		ExecutedAt:        startTime.Format("2006-01-02 15:04:05"),
		DurationMs:        duration,
	}
}

// Restart 重启清理任务（配置变更后调用）
func (c *DataCleaner) Restart() error {
	c.Stop()
	return c.Start()
}