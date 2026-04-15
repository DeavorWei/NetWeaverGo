//go:build windows

package icmp

import (
	"context"
	"log"
	"net"
	"sync"
	"time"
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
func (e *BatchPingEngine) Run(ctx context.Context, ips []string, onUpdate func(*BatchPingProgress)) *BatchPingProgress {
	// Create cancellable context
	runCtx, cancel := context.WithCancel(ctx)

	e.runningMu.Lock()
	// 双重检查：如果已经在运行，不启动新任务
	if e.running {
		e.runningMu.Unlock()
		cancel()
		return NewBatchPingProgress(len(ips))
	}
	e.running = true
	e.cancel = cancel
	e.runningMu.Unlock()

	progress := NewBatchPingProgress(len(ips))

	// 统一的清理逻辑 - 这是唯一设置 running = false 的地方
	defer func() {
		e.runningMu.Lock()
		e.running = false
		e.cancel = nil
		e.runningMu.Unlock()

		progress.Finish()
		safeCallback(onUpdate, progress)
	}()

	if len(ips) == 0 {
		return progress
	}

	// Semaphore for concurrency control
	sem := make(chan struct{}, e.config.Concurrency)
	var wg sync.WaitGroup
	var progressMu sync.Mutex

	for i, ipStr := range ips {
		// Check for cancellation
		select {
		case <-runCtx.Done():
			return progress
		default:
		}

		sem <- struct{}{}
		wg.Add(1)

		go func(index int, targetIP string) {
			defer wg.Done()
			defer func() { <-sem }()

			// Check for cancellation before starting
			select {
			case <-runCtx.Done():
				progressMu.Lock()
				progress.SetResult(index, PingHostResult{
					IP:        targetIP,
					Status:    "error",
					ErrorMsg:  "Cancelled",
					SentCount: 1,
					RecvCount: 0,
					LossRate:  100,
				})
				safeCallback(onUpdate, progress)
				progressMu.Unlock()
				return
			default:
			}

			// Parse IP
			ip := net.ParseIP(targetIP)
			if ip == nil {
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
				safeCallback(onUpdate, progress)
				progressMu.Unlock()
				return
			}

			// Perform ping attempts
			result := e.pingHost(runCtx, ip)

			progressMu.Lock()
			progress.SetResult(index, result) // 按索引存储，保持顺序
			// 只在非取消状态下触发回调，避免取消后不必要的更新
			select {
			case <-runCtx.Done():
				// 已取消，不触发回调
			default:
				safeCallback(onUpdate, progress)
			}
			progressMu.Unlock()
		}(i, ipStr)
	}

	wg.Wait()
	// 完成后触发最终回调
	safeCallback(onUpdate, progress)
	return progress
}

// pingHost performs multiple ping attempts to a single host and aggregates results.
func (e *BatchPingEngine) pingHost(ctx context.Context, ip net.IP) PingHostResult {
	result := PingHostResult{
		IP:        ip.String(),
		Status:    "pending",
		SentCount: e.config.Count,
	}

	var successCount int
	var rttSum float64
	var minRtt float64 = 0 // 初始化为 0 表示无效值
	var maxRtt float64
	var lastTTL uint8

	for i := 0; i < e.config.Count; i++ {
		// Check for cancellation
		select {
		case <-ctx.Done():
			result.Status = "error"
			result.ErrorMsg = "Cancelled"
			return result
		default:
		}

		// Perform single ping
		pingResult, err := PingOne(ip, e.config.Timeout, e.config.DataSize)
		if err != nil {
			result.ErrorMsg = err.Error()
			continue
		}

		if pingResult.Success {
			successCount++
			rttSum += pingResult.RoundTripTime
			// 仅在第一次成功或更小时更新 minRtt
			if minRtt == 0 || pingResult.RoundTripTime < minRtt {
				minRtt = pingResult.RoundTripTime
			}
			if pingResult.RoundTripTime > maxRtt {
				maxRtt = pingResult.RoundTripTime
			}
			lastTTL = pingResult.TTL
		}

		// Wait for interval between pings (if specified)
		if i < e.config.Count-1 && e.config.Interval > 0 {
			select {
			case <-ctx.Done():
				result.Status = "error"
				result.ErrorMsg = "Cancelled"
				return result
			case <-time.After(time.Duration(e.config.Interval) * time.Millisecond):
			}
		}
	}

	// Calculate statistics
	result.RecvCount = successCount
	if result.SentCount > 0 {
		result.LossRate = float64(result.SentCount-successCount) / float64(result.SentCount) * 100
	}

	if successCount > 0 {
		result.Alive = true
		result.Status = "online"
		result.AvgRtt = rttSum / float64(successCount)
		result.MinRtt = minRtt
		result.MaxRtt = maxRtt
		result.TTL = lastTTL
	} else {
		result.Alive = false
		result.Status = "offline"
		// minRtt 保持为 0，前端会显示为 "-"
	}

	return result
}

// Stop 停止正在运行的批量 Ping 操作
// 仅触发 context 取消，状态管理由 Run() 生命周期控制
func (e *BatchPingEngine) Stop() {
	e.runningMu.Lock()
	cancel := e.cancel
	e.runningMu.Unlock()

	// 在锁外调用 cancel，避免死锁
	if cancel != nil {
		cancel()
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
