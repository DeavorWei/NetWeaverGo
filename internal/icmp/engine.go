//go:build windows

package icmp

import (
	"context"
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

// Run executes the batch ping operation for the given IP addresses.
// The onUpdate callback is called whenever progress is made.
func (e *BatchPingEngine) Run(ctx context.Context, ips []string, onUpdate func(*BatchPingProgress)) *BatchPingProgress {
	// Create cancellable context
	runCtx, cancel := context.WithCancel(ctx)
	e.setRunning(true)
	e.cancel = cancel

	progress := NewBatchPingProgress(len(ips))

	// Ensure we mark as finished when done
	defer func() {
		e.setRunning(false)
		progress.Finish()
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
				return
			default:
			}

			// Parse IP
			ip := net.ParseIP(targetIP)
			if ip == nil {
				progressMu.Lock()
				progress.AddResult(PingHostResult{
					IP:        targetIP,
					Alive:     false,
					Status:    "error",
					ErrorMsg:  "Invalid IP address",
					SentCount: 1,
					RecvCount: 0,
					LossRate:  100,
				})
				if onUpdate != nil {
					onUpdate(progress.Clone()) // 深拷贝
				}
				progressMu.Unlock()
				return
			}

			// Perform ping attempts
			result := e.pingHost(runCtx, ip)

			progressMu.Lock()
			progress.AddResult(result)
			if onUpdate != nil {
				onUpdate(progress.Clone()) // 深拷贝
			}
			progressMu.Unlock()
		}(i, ipStr)
	}

	wg.Wait()
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
	var rttSum uint32
	var minRtt uint32 = ^uint32(0) // Max uint32
	var maxRtt uint32
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
			if pingResult.RoundTripTime < minRtt {
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
		result.AvgRtt = rttSum / uint32(successCount)
		result.MinRtt = minRtt
		result.MaxRtt = maxRtt
		result.TTL = lastTTL
	} else {
		result.Alive = false
		result.Status = "offline"
	}

	return result
}

// Stop cancels the running batch ping operation.
func (e *BatchPingEngine) Stop() {
	e.runningMu.Lock()
	defer e.runningMu.Unlock()
	if e.cancel != nil {
		e.cancel()
	}
	e.running = false
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
