// Package snmp 提供 SNMP 核心业务功能
// poll_dispatcher.go 实现两级信号量并发控制的轮询分发器
// 参考设计文档: docs/SNMP轮询并发控制设计方案.md (v1.3)
package snmp

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/NetWeaverGo/core/internal/logger"
)

// ============================================================================
// PollDispatcher 轮询分发器
// ============================================================================

// PollDispatcher 轮询分发器
// 实现两级信号量并发控制：
//   - Level 1 (taskSem): 控制全局并发设备数
//   - Level 2 (deviceSems): 控制单设备并发操作数
type PollDispatcher struct {
	poller   *Poller        // 轮询执行器
	notifier EventNotifier  // 事件通知器（A-18: 超时等异常通知前端）

	// Level 1: 任务级信号量（控制全局并发设备数）
	taskSem chan struct{}

	// Level 2: 设备级信号量 map（控制单设备并发操作数）
	deviceSems   map[string]chan struct{}
	deviceSemsMu sync.RWMutex

	// 设备最后使用时间（用于过期清理）
	deviceLastUsed map[string]time.Time

	// maxOpsPerDevice 设备级并发数快照
	// 在 deviceSemsMu 保护下更新，避免直接读 config 产生竞态
	maxOpsPerDevice int

	// 配置（支持动态更新）
	config   DispatcherConfig
	configMu sync.RWMutex

	// 原子计数器（运行时统计）
	activeCount  int64 // 当前活跃任务数
	waitingCount int64 // 正在排队等待的任务数
	skippedCount int64 // 因繁忙被跳过的任务总数

	// done 用于优雅关闭后台 goroutine
	done chan struct{}
}

// NewPollDispatcher 创建轮询分发器
// notifier 可以为 nil（此时超时等异常仅记录日志，不通知前端）
// config 可选，不传时使用 DefaultDispatcherConfig
func NewPollDispatcher(poller *Poller, notifier EventNotifier, config ...DispatcherConfig) *PollDispatcher {
	cfg := DefaultDispatcherConfig
	if len(config) > 0 {
		cfg = config[0]
	}

	// 配置底线校验，防止容量为 0 引发永久阻塞
	if cfg.MaxConcurrentDevices <= 0 {
		cfg.MaxConcurrentDevices = 1
	}
	if cfg.MaxOpsPerDevice <= 0 {
		cfg.MaxOpsPerDevice = 1
	}
	if cfg.QueueTimeout <= 0 {
		cfg.QueueTimeout = DefaultDispatcherConfig.QueueTimeout
	}

	d := &PollDispatcher{
		poller:          poller,
		notifier:        notifier,
		taskSem:         make(chan struct{}, cfg.MaxConcurrentDevices),
		deviceSems:      make(map[string]chan struct{}),
		deviceLastUsed:  make(map[string]time.Time),
		maxOpsPerDevice: cfg.MaxOpsPerDevice,
		config:          cfg,
		done:            make(chan struct{}),
	}

	// 启动后台清理 goroutine
	go d.cleanupLoop()

	logger.Info("Dispatcher", "-", "轮询分发器已初始化 (最大设备数: %d, 单设备并发: %d, 跳过繁忙: %v, 队列超时: %v)",
		cfg.MaxConcurrentDevices, cfg.MaxOpsPerDevice, cfg.SkipIfBusy, cfg.QueueTimeout)

	return d
}

// ============================================================================
// 核心分发方法
// ============================================================================

// DispatchSync 同步提交轮询任务（阻塞直到完成）
// 实现两级信号量并发控制，根据 SkipIfBusy 配置决定等待或跳过策略
func (d *PollDispatcher) DispatchSync(ctx context.Context, target *PollTarget, opts ...DispatchOption) *PollResult {
	// 1. 读取配置快照（持有读锁，避免与 UpdateConfig 竞态）
	d.configMu.RLock()
	cfg := d.config
	currentTaskSem := d.taskSem
	d.configMu.RUnlock()

	// 2. 解析选项
	options := &dispatchOptions{skipIfBusy: cfg.SkipIfBusy}
	for _, opt := range opts {
		opt(options)
	}

	result := &PollResult{Target: target}
	start := time.Now()
	defer func() {
		result.Latency = time.Since(start)
	}()

	targetIP := target.Target.TargetIP

	// ━━━ Level 1: 获取任务级信号量 ━━━
	// 【v1.2 关键修订】先捕获 taskSem 到局部变量，再用局部变量获取令牌
	// 保证获取和释放操作在同一个 channel 上，彻底消除与 UpdateConfig 的竞态窗口

	atomic.AddInt64(&d.waitingCount, 1)

	if options.skipIfBusy {
		// Cron 路径：Level 1 非阻塞尝试，系统过载时快速跳过
		select {
		case currentTaskSem <- struct{}{}:
			// 获取成功
		default:
			atomic.AddInt64(&d.waitingCount, -1)
			atomic.AddInt64(&d.skippedCount, 1)
			logger.Info("Dispatcher", targetIP, "系统繁忙（任务槽已满），跳过本次轮询")
			result.Skipped = true
			return result
		}
	} else {
		// 手动触发路径：阻塞等待，带超时
		timer := time.NewTimer(cfg.QueueTimeout)
		select {
		case currentTaskSem <- struct{}{}:
			timer.Stop()
		case <-ctx.Done():
			timer.Stop()
			atomic.AddInt64(&d.waitingCount, -1)
			result.Cancelled = true
			result.Error = ctx.Err()
			return result
		case <-timer.C:
			atomic.AddInt64(&d.waitingCount, -1)
			result.Cancelled = true
			result.Error = fmt.Errorf("等待任务槽位超时 (%v)", cfg.QueueTimeout)
			return result
		}
	}
	atomic.AddInt64(&d.waitingCount, -1)
	atomic.AddInt64(&d.activeCount, 1)

	// defer 释放任务级信号量（使用捕获的局部变量）
	defer func() {
		<-currentTaskSem // 释放到获取时的同一个 channel
		atomic.AddInt64(&d.activeCount, -1)
	}()

	// ━━━ Level 2: 获取设备级信号量 ━━━
	deviceSem := d.getOrCreateDeviceSem(targetIP)

	atomic.AddInt64(&d.waitingCount, 1)

	if options.skipIfBusy {
		// Cron 路径：非阻塞尝试获取
		select {
		case deviceSem <- struct{}{}:
			// 获取成功
		default:
			atomic.AddInt64(&d.waitingCount, -1)
			atomic.AddInt64(&d.skippedCount, 1)
			logger.Info("Dispatcher", targetIP, "设备繁忙，跳过本次轮询")
			result.Skipped = true
			return result
		}
	} else {
		// 手动触发路径：阻塞等待
		select {
		case deviceSem <- struct{}{}:
			// 获取成功
		case <-ctx.Done():
			atomic.AddInt64(&d.waitingCount, -1)
			result.Cancelled = true
			result.Error = ctx.Err()
			return result
		}
	}
	atomic.AddInt64(&d.waitingCount, -1)

	// defer 释放设备级信号量（LIFO 顺序：先释放设备级，再释放任务级）
	defer func() { <-deviceSem }()

	// ━━━ 执行轮询 ━━━
	result.Results, result.Error = d.poller.pollWithRetry(ctx, target)

	return result
}

// Dispatch 异步提交轮询任务（返回 channel 接收结果）
func (d *PollDispatcher) Dispatch(ctx context.Context, target *PollTarget, opts ...DispatchOption) <-chan *PollResult {
	ch := make(chan *PollResult, 1)
	go func() {
		defer close(ch)
		ch <- d.DispatchSync(ctx, target, opts...)
	}()
	return ch
}

// DispatchBatch 批量提交轮询任务
// 为每个目标启动独立 goroutine，两级并发控制由 DispatchSync 内部保证
// 所有任务完成后统一返回结果（部分成功部分失败的情况通过 PollResult.Error 区分）
func (d *PollDispatcher) DispatchBatch(ctx context.Context, targets []*PollTarget) []*PollResult {
	if len(targets) == 0 {
		return nil
	}

	results := make([]*PollResult, len(targets))
	var wg sync.WaitGroup

	for i, t := range targets {
		wg.Add(1)
		go func(idx int, target *PollTarget) {
			defer wg.Done()

			// 每个目标通过 DispatchSync 提交，自动受两级信号量控制
			// 批量场景使用 SkipIfBusy=false（排队等待）
			result := d.DispatchSync(ctx, target, WithSkipIfBusy(false))
			if result == nil {
				result = &PollResult{Target: target, Error: fmt.Errorf("dispatch 返回 nil")}
			}
			results[idx] = result
		}(i, t)
	}

	wg.Wait()
	return results
}

// ============================================================================
// 配置动态更新
// ============================================================================

// UpdateConfig 动态更新分发器配置
// 采用安全更新策略，避免信号量替换竞态：
//   - QueueTimeout、SkipIfBusy: 立即生效
//   - MaxOpsPerDevice: 新设备信号量使用新容量，旧信号量自然过渡
//   - MaxConcurrentDevices: 异步等待排空后重建 taskSem
func (d *PollDispatcher) UpdateConfig(config DispatcherConfig) error {
	// 配置底线校验，防止容量为 0 引发永久阻塞
	if config.MaxOpsPerDevice <= 0 {
		config.MaxOpsPerDevice = 1
	}
	if config.MaxConcurrentDevices <= 0 {
		config.MaxConcurrentDevices = 1
	}
	if config.QueueTimeout <= 0 {
		config.QueueTimeout = DefaultDispatcherConfig.QueueTimeout
	}

	d.configMu.Lock()
	oldConfig := d.config

	// ━━━ 立即生效的配置 ━━━
	d.config.QueueTimeout = config.QueueTimeout
	d.config.SkipIfBusy = config.SkipIfBusy
	d.configMu.Unlock()

	// ━━━ 设备级并发数变更（可安全重建）━━━
	// 旧设备信号量会在其空闲时由 CleanupIdleDeviceSems 回收。
	// 在这期间，已分配的旧设备和新分配的新设备会出现容量不同的短暂不一致。
	// 这属于平滑过渡策略的可接受范围。
	if config.MaxOpsPerDevice != oldConfig.MaxOpsPerDevice {
		d.deviceSemsMu.Lock()
		d.maxOpsPerDevice = config.MaxOpsPerDevice
		// 清空 map，新设备使用新容量创建；旧设备信号量被丢弃后，正在执行的任务
		// 仍持有旧 channel 引用可正常释放，不会阻塞
		d.deviceSems = make(map[string]chan struct{})
		d.deviceSemsMu.Unlock()
		logger.Info("Dispatcher", "-", "设备级并发数已更新: %d -> %d (新任务生效)",
			oldConfig.MaxOpsPerDevice, config.MaxOpsPerDevice)
	}

	// ━━━ 任务级并发数变更（需等待排空）━━━
	if config.MaxConcurrentDevices != oldConfig.MaxConcurrentDevices {
		go d.rebuildTaskSem(config.MaxConcurrentDevices, oldConfig.MaxConcurrentDevices)
	}

	return nil
}

// rebuildTaskSem 等待所有活跃任务完成后重建任务级信号量
// 在后台 goroutine 中执行，不阻塞调用方
func (d *PollDispatcher) rebuildTaskSem(newCapacity, oldCapacity int) {
	logger.Info("Dispatcher", "-", "任务级并发数变更请求: %d -> %d, 等待活跃任务完成...",
		oldCapacity, newCapacity)

	// 超时保护，防止活跃任务长期不结束导致 goroutine 永不退出
	const rebuildTimeout = 5 * time.Minute
	deadline := time.After(rebuildTimeout)

	// 等待所有活跃任务完成（轮询检查，间隔 500ms）
	// 活跃任务的 defer 释放的是 currentTaskSem（局部变量捕获），不受此处影响
	for {
		active := atomic.LoadInt64(&d.activeCount)
		if active == 0 {
			break
		}

		select {
		case <-deadline:
			logger.Warn("Dispatcher", "-",
				"等待活跃任务超时 (%v)，放弃重建 taskSem (仍有 %d 个活跃任务)，当前容量 %d 保持不变",
				rebuildTimeout, active, oldCapacity)
			// A-18: 超时后通知前端用户
			if d.notifier != nil {
				d.notifier.NotifyError(fmt.Errorf("变更任务并发数超时，容量保留为 %d", oldCapacity))
			}
			return
		default:
			logger.Debug("Dispatcher", "-", "等待活跃任务完成: 剩余 %d 个", active)
			time.Sleep(500 * time.Millisecond)
		}
	}

	// 所有任务已完成，安全替换
	d.taskSem = make(chan struct{}, newCapacity)

	d.configMu.Lock()
	d.config.MaxConcurrentDevices = newCapacity
	d.configMu.Unlock()

	logger.Info("Dispatcher", "-", "任务级并发数已生效: %d -> %d",
		oldCapacity, newCapacity)
}

// ============================================================================
// 设备信号量生命周期管理
// ============================================================================

// getOrCreateDeviceSem 获取或创建设备级信号量
// 使用 d.maxOpsPerDevice（在 deviceSemsMu 保护下更新的快照值）而非直接读 config，
// 消除竞态
func (d *PollDispatcher) getOrCreateDeviceSem(targetIP string) chan struct{} {
	d.deviceSemsMu.Lock()
	defer d.deviceSemsMu.Unlock()

	// 更新最后使用时间（用于过期清理）
	d.deviceLastUsed[targetIP] = time.Now()

	if sem, exists := d.deviceSems[targetIP]; exists {
		return sem
	}

	sem := make(chan struct{}, d.maxOpsPerDevice)
	d.deviceSems[targetIP] = sem
	return sem
}

// CleanupIdleDeviceSems 清理长时间未使用的设备信号量
// 建议由外部定时调用（如每小时一次）
// 返回清理的设备信号量数量
func (d *PollDispatcher) CleanupIdleDeviceSems(maxIdleDuration time.Duration) int {
	d.deviceSemsMu.Lock()
	defer d.deviceSemsMu.Unlock()

	now := time.Now()
	cleaned := 0

	for ip, lastUsed := range d.deviceLastUsed {
		if now.Sub(lastUsed) > maxIdleDuration {
			// 检查信号量是否空闲（无人持有令牌）
			sem := d.deviceSems[ip]
			if sem != nil && len(sem) == 0 {
				delete(d.deviceSems, ip)
				delete(d.deviceLastUsed, ip)
				cleaned++
			}
		}
	}

	if cleaned > 0 {
		logger.Info("Dispatcher", "-", "已清理 %d 个空闲设备信号量 (阈值: %v)",
			cleaned, maxIdleDuration)
	}
	return cleaned
}

// cleanupLoop 后台定期清理长时间未使用的设备信号量
// 由 NewPollDispatcher 启动，通过 Close() 优雅关闭
func (d *PollDispatcher) cleanupLoop() {
	ticker := time.NewTicker(10 * time.Minute) // 每 10 分钟清理一次
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			d.CleanupIdleDeviceSems(30 * time.Minute) // 清理 30 分钟无活动的设备
		case <-d.done:
			return
		}
	}
}

// Stop 优雅关闭分发器，停止后台清理 goroutine
// 与 PollerScheduler.Stop()、TrapListener.Stop() 保持一致的生命周期命名
func (d *PollDispatcher) Stop() {
	close(d.done)
}

// RemoveDeviceSem 主动移除指定设备的信号量
// 设备被删除时应调用此方法同步清理
func (d *PollDispatcher) RemoveDeviceSem(targetIP string) {
	d.deviceSemsMu.Lock()
	defer d.deviceSemsMu.Unlock()
	delete(d.deviceSems, targetIP)
	delete(d.deviceLastUsed, targetIP)
}

// ============================================================================
// 状态查询
// ============================================================================

// GetConfig 获取分发器当前配置快照
func (d *PollDispatcher) GetConfig() DispatcherConfig {
	d.configMu.RLock()
	defer d.configMu.RUnlock()
	return d.config
}

// GetStatus 获取分发器运行时状态快照
func (d *PollDispatcher) GetStatus() *DispatcherStatus {
	d.configMu.RLock()
	cfg := d.config
	d.configMu.RUnlock()

	status := &DispatcherStatus{
		ActiveDevices:   int(atomic.LoadInt64(&d.activeCount)),
		MaxDevices:      cfg.MaxConcurrentDevices,
		MaxOpsPerDevice: cfg.MaxOpsPerDevice,
		WaitingTasks:    int(atomic.LoadInt64(&d.waitingCount)),
		SkippedTasks:    atomic.LoadInt64(&d.skippedCount),
		DeviceStatus:    make(map[string]*DeviceSemStatus),
	}

	d.deviceSemsMu.RLock()
	for ip, sem := range d.deviceSems {
		status.DeviceStatus[ip] = &DeviceSemStatus{
			ActiveOps: len(sem),
			MaxOps:    cap(sem),
		}
	}
	d.deviceSemsMu.RUnlock()

	return status
}
