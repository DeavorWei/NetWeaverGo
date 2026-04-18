//go:build windows

package icmp

import (
	"context"
	"log"
	"net"
	"sync"
	"time"

	"github.com/NetWeaverGo/core/internal/logger"
)

// BatchPingEngine handles batch ping operations with concurrency control.
type BatchPingEngine struct {
	config    PingConfig
	cancel    context.CancelFunc
	running   bool
	runningMu sync.RWMutex
}

// NewBatchPingEngine creates a new BatchPingEngine with the given configuration.
func NewBatchPingEngine(config PingConfig) *BatchPingEngine {
	// Apply defaults for invalid values
	if config.Timeout == 0 {
		config.Timeout = DefaultPingConfig().Timeout
	}
	if config.DataSize == 0 {
		config.DataSize = DefaultPingConfig().DataSize
	}
	if config.Count == 0 {
		config.Count = DefaultPingConfig().Count
	}
	if config.Concurrency == 0 {
		config.Concurrency = DefaultPingConfig().Concurrency
	}
	// Limit maximum concurrency
	if config.Concurrency > 256 {
		config.Concurrency = 256
	}

	return &BatchPingEngine{
		config: config,
	}
}

// safeCallback safely invokes the progress callback with panic recovery.
func safeCallback(fn func(*BatchPingProgress), progress *BatchPingProgress) {
	if fn == nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			// 记录 panic 信息，便于排查问题
			log.Printf("Ping progress callback panic recovered: %v", r)
		}
	}()
	fn(progress)
}

// Run executes the batch ping operation for the given IP addresses.
// The onUpdate callback is called whenever progress is made.
// This method maintains backward compatibility with existing code.
func (e *BatchPingEngine) Run(ctx context.Context, ips []string, onUpdate func(*BatchPingProgress), onSinglePing func(SinglePingResult)) *BatchPingProgress {
	return e.RunWithOptions(ctx, ips, RunOptions{
		OnUpdate:     onUpdate,
		OnSinglePing: onSinglePing,
	})
}

// RunWithOptions executes the batch ping operation with extended callback options.
// This is the preferred method for new code that needs host intermediate state updates.
func (e *BatchPingEngine) RunWithOptions(ctx context.Context, ips []string, opts RunOptions) *BatchPingProgress {
	logger.Info("BatchPing", "-", "批量 Ping 开始: totalIPs=%d, concurrency=%d, timeout=%dms, count=%d",
		len(ips), e.config.Concurrency, e.config.Timeout, e.config.Count)

	// Create cancellable context
	runCtx, cancel := context.WithCancel(ctx)

	e.runningMu.Lock()
	// 双重检查：如果已经在运行，不启动新任务
	if e.running {
		logger.Warn("BatchPing", "-", "引擎已在运行中，拒绝启动新任务")
		e.runningMu.Unlock()
		cancel()
		return NewBatchPingProgress(ips)
	}
	e.running = true
	e.cancel = cancel
	e.runningMu.Unlock()

	progress := NewBatchPingProgress(ips)

	// 统一的清理逻辑 - 这是唯一设置 running = false 的地方
	defer func() {
		logger.Debug("BatchPing", "-", "批量 Ping 清理资源，标记完成状态")
		e.runningMu.Lock()
		e.running = false
		e.cancel = nil
		e.runningMu.Unlock()

		progress.Finish()
		logger.Info("BatchPing", "-", "批量 Ping 完成: total=%d, online=%d, offline=%d, error=%d, elapsed=%dms",
			progress.TotalIPs, progress.OnlineCount, progress.OfflineCount, progress.ErrorCount, progress.ElapsedMs)
		safeCallback(opts.OnUpdate, progress)
	}()

	if len(ips) == 0 {
		logger.Warn("BatchPing", "-", "IP 列表为空，立即返回")
		return progress
	}

	// Semaphore for concurrency control
	sem := make(chan struct{}, e.config.Concurrency)
	var wg sync.WaitGroup
	var progressMu sync.Mutex

	logger.Debug("BatchPing", "-", "并发控制初始化: concurrency=%d", e.config.Concurrency)

	for i, ipStr := range ips {
		// Check for cancellation
		select {
		case <-runCtx.Done():
			logger.Debug("BatchPing", "-", "检测到取消信号，停止添加新任务")
			return progress
		default:
		}

		logger.Verbose("BatchPing", ipStr, "等待获取信号量, index=%d", i)
		sem <- struct{}{}
		logger.Verbose("BatchPing", ipStr, "获取信号量成功, index=%d", i)
		wg.Add(1)

		go func(index int, targetIP string) {
			defer wg.Done()
			defer func() {
				logger.Verbose("BatchPing", targetIP, "释放信号量, index=%d", index)
				<-sem
			}()

			logger.Debug("BatchPing", targetIP, "开始处理, index=%d", index)

			// Check for cancellation before starting
			select {
			case <-runCtx.Done():
				logger.Debug("BatchPing", targetIP, "任务被取消, index=%d", index)
				progressMu.Lock()
				progress.SetResult(index, PingHostResult{
					IP:        targetIP,
					Status:    "error",
					ErrorMsg:  "Cancelled",
					SentCount: 1,
					RecvCount: 0,
					LossRate:  100,
				})
				safeCallback(opts.OnUpdate, progress)
				progressMu.Unlock()
				return
			default:
			}

			// Parse IP
			ip := net.ParseIP(targetIP)
			if ip == nil {
				logger.Error("BatchPing", targetIP, "无效的 IP 地址, index=%d", index)
				progressMu.Lock()
				progress.SetResult(index, PingHostResult{
					IP:        targetIP,
					Alive:     false,
					Status:    "error",
					ErrorMsg:  "Invalid IP address",
					SentCount: 1,
					RecvCount: 0,
					LossRate:  100,
				})
				safeCallback(opts.OnUpdate, progress)
				progressMu.Unlock()
				return
			}

			// Perform ping attempts with host update callback
			result := e.pingHostWithOptions(runCtx, ip, index, opts.OnSinglePing, opts.OnHostUpdate)

			progressMu.Lock()
			progress.SetResult(index, result) // 按索引存储，保持顺序
			logger.Debug("BatchPing", targetIP, "处理完成, index=%d, status=%s, errorMsg=%s",
				index, result.Status, result.ErrorMsg)
			// 只在非取消状态下触发回调，避免取消后不必要的更新
			select {
			case <-runCtx.Done():
				logger.Verbose("BatchPing", targetIP, "已取消，不触发回调, index=%d", index)
			default:
				safeCallback(opts.OnUpdate, progress)
			}
			progressMu.Unlock()
		}(i, ipStr)
	}

	wg.Wait()
	logger.Debug("BatchPing", "-", "所有 worker 已完成")
	// 完成后触发最终回调
	safeCallback(opts.OnUpdate, progress)
	return progress
}

// pingHost performs multiple ping attempts to a single host and aggregates results.
// This method is kept for backward compatibility.
func (e *BatchPingEngine) pingHost(ctx context.Context, ip net.IP, onSinglePing func(SinglePingResult)) PingHostResult {
	return e.pingHostWithOptions(ctx, ip, 0, onSinglePing, nil)
}

// pingHostWithOptions performs multiple ping attempts to a single host with intermediate state callbacks.
// The onHostUpdate callback is called after each ping attempt to provide real-time progress updates.
func (e *BatchPingEngine) pingHostWithOptions(ctx context.Context, ip net.IP, index int, onSinglePing func(SinglePingResult), onHostUpdate func(HostPingUpdate)) PingHostResult {
	ipStr := ip.String()
	logger.Debug("BatchPing", ipStr, "开始 ping 主机: count=%d", e.config.Count)

	result := PingHostResult{
		IP:        ipStr,
		Status:    "pending",
		SentCount: e.config.Count,
		MinRtt:    -1, // 使用 -1 表示无效值
	}

	var successCount int
	var failedCount int
	var rttSum float64
	var minRtt float64 = -1 // 使用 -1 表示无效值
	var maxRtt float64
	var lastTTL uint8
	var lastError string
	var lastRtt float64
	var lastSucceedAt int64
	var lastFailedAt int64

	// Helper function to emit host update
	emitHostUpdate := func(currentSeq int, isComplete bool) {
		if onHostUpdate == nil {
			return
		}

		// Calculate partial statistics
		partialStats := PartialStats{
			SentCount:   currentSeq,
			RecvCount:   successCount,
			FailedCount: failedCount,
			LastRtt:     lastRtt,
			MinRtt:      minRtt,
			MaxRtt:      maxRtt,
		}

		// Calculate loss rate
		if currentSeq > 0 {
			partialStats.LossRate = float64(currentSeq-successCount) / float64(currentSeq) * 100
		}

		// Calculate average RTT
		if successCount > 0 {
			partialStats.AvgRtt = rttSum / float64(successCount)
		}

		onHostUpdate(HostPingUpdate{
			IP:           ipStr,
			Index:        index,
			CurrentSeq:   currentSeq,
			PartialStats: partialStats,
			IsComplete:   isComplete,
			Timestamp:    time.Now().UnixMilli(),
		})
	}

	for i := 0; i < e.config.Count; i++ {
		// Check for cancellation
		select {
		case <-ctx.Done():
			logger.Debug("BatchPing", ipStr, "ping 被取消")
			result.Status = "error"
			result.ErrorMsg = "Cancelled"
			return result
		default:
		}

		logger.Verbose("BatchPing", ipStr, "第 %d/%d 次 ping", i+1, e.config.Count)

		// Perform single ping
		pingResult, err := PingOne(ip, e.config.Timeout, e.config.DataSize)

		singleResult := SinglePingResult{
			IP:        ipStr,
			Seq:       i + 1,
			Timestamp: time.Now().UnixMilli(),
		}

		// Handle PingOne function errors (API level errors)
		if err != nil {
			logger.Debug("BatchPing", ipStr, "第 %d 次 ping 返回错误: %v", i+1, err)
			failedCount++
			lastFailedAt = time.Now().UnixMilli()
			lastError = err.Error()

			singleResult.Success = false
			singleResult.Status = "error"
			singleResult.Error = err.Error()
			if onSinglePing != nil {
				onSinglePing(singleResult)
			}

			// Emit intermediate state update
			emitHostUpdate(i+1, false)
			continue
		}

		// Critical fix: Handle nil pingResult (should not happen, but safety check)
		if pingResult == nil {
			logger.Debug("BatchPing", ipStr, "第 %d 次 ping 返回 nil 结果", i+1)
			failedCount++
			lastFailedAt = time.Now().UnixMilli()
			lastError = "Ping returned nil result"

			singleResult.Success = false
			singleResult.Status = "error"
			singleResult.Error = "Ping returned nil result"
			if onSinglePing != nil {
				onSinglePing(singleResult)
			}

			// Emit intermediate state update
			emitHostUpdate(i+1, false)
			continue
		}

		logger.Verbose("BatchPing", ipStr, "第 %d 次 ping 结果: success=%v, rtt=%.2fms, status=%s, error=%s",
			i+1, pingResult.Success, pingResult.RoundTripTime, pingResult.Status, pingResult.Error)

		if pingResult.Success {
			successCount++

			// 更新 RTT 统计
			rttSum += pingResult.RoundTripTime
			if minRtt < 0 || pingResult.RoundTripTime < minRtt {
				minRtt = pingResult.RoundTripTime
			}
			if pingResult.RoundTripTime > maxRtt {
				maxRtt = pingResult.RoundTripTime
			}

			// 记录最后一次成功信息
			lastSucceedAt = time.Now().UnixMilli()
			lastRtt = pingResult.RoundTripTime
			lastTTL = pingResult.TTL

			singleResult.Success = true
			singleResult.RoundTripTime = pingResult.RoundTripTime
			singleResult.TTL = pingResult.TTL
			singleResult.Status = "success"
		} else {
			failedCount++

			// 记录最后一次失败信息
			lastFailedAt = time.Now().UnixMilli()
			if pingResult.Error != "" {
				lastError = pingResult.Error
			} else if pingResult.Status != "" {
				lastError = pingResult.Status
			}
			logger.Debug("BatchPing", ipStr, "第 %d 次 ping 失败: status=%s, error=%s", i+1, pingResult.Status, pingResult.Error)

			singleResult.Success = false
			singleResult.Status = pingResult.Status
			singleResult.Error = lastError
		}

		if onSinglePing != nil {
			onSinglePing(singleResult)
		}

		// Emit intermediate state update after each ping
		emitHostUpdate(i+1, false)

		// Wait for interval between pings (if specified)
		if i < e.config.Count-1 && e.config.Interval > 0 {
			select {
			case <-ctx.Done():
				logger.Debug("BatchPing", ipStr, "等待间隔时被取消")
				result.Status = "error"
				result.ErrorMsg = "Cancelled"
				return result
			case <-time.After(time.Duration(e.config.Interval) * time.Millisecond):
			}
		}
	}

	// Calculate statistics
	result.RecvCount = successCount
	result.FailedCount = failedCount
	result.LastSucceedAt = lastSucceedAt
	result.LastFailedAt = lastFailedAt
	result.LastRtt = lastRtt
	result.TTL = lastTTL

	if result.SentCount > 0 {
		result.LossRate = float64(result.SentCount-successCount) / float64(result.SentCount) * 100
	}

	if successCount > 0 {
		result.Alive = true
		result.Status = "online"
		result.AvgRtt = rttSum / float64(successCount)
		result.MinRtt = minRtt
		result.MaxRtt = maxRtt
		logger.Debug("BatchPing", ipStr, "主机在线: success=%d/%d, avgRtt=%.2fms, minRtt=%.2fms, maxRtt=%.2fms",
			successCount, e.config.Count, result.AvgRtt, result.MinRtt, result.MaxRtt)
	} else {
		result.Alive = false
		result.Status = "offline"
		// 关键修复：设置错误信息
		if lastError != "" {
			result.ErrorMsg = lastError
		}
		logger.Debug("BatchPing", ipStr, "主机离线: error=%s", result.ErrorMsg)
	}

	// Emit final host update with completion flag
	emitHostUpdate(e.config.Count, true)

	return result
}

// Stop 停止正在运行的批量 Ping 操作
// 仅触发 context 取消，状态管理由 Run() 生命周期控制
func (e *BatchPingEngine) Stop() {
	e.runningMu.Lock()
	cancel := e.cancel
	running := e.running
	e.runningMu.Unlock()

	// 在锁外调用 cancel，避免死锁
	if cancel != nil {
		logger.Info("BatchPing", "-", "正在停止批量 Ping 操作...")
		cancel()
	} else {
		logger.Debug("BatchPing", "-", "停止请求已收到，但没有活动的取消函数 (running=%v)", running)
	}
	// 注意：不直接修改 running 状态，由 Run() 的 defer 统一处理
}

// IsRunning returns whether the engine is currently running.
func (e *BatchPingEngine) IsRunning() bool {
	e.runningMu.RLock()
	defer e.runningMu.RUnlock()
	return e.running
}

// setRunning sets the running state.
func (e *BatchPingEngine) setRunning(running bool) {
	e.runningMu.Lock()
	defer e.runningMu.Unlock()
	e.running = running
}

// GetConfig returns the current configuration.
func (e *BatchPingEngine) GetConfig() PingConfig {
	return e.config
}

// SetConfig updates the configuration.
func (e *BatchPingEngine) SetConfig(config PingConfig) {
	e.config = config
}
