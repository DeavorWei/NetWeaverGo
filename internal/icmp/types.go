// Package icmp provides Windows ICMP API wrappers for batch ping operations.
package icmp

import (
	"fmt"
	"time"
)

// PingResult represents the result of a single ICMP echo request.
type PingResult struct {
	IP            string  // Target IP address
	Success       bool    // Whether the ping was successful
	RoundTripTime float64 // Round trip time in milliseconds (支持亚毫秒精度)
	TTL           uint8   // Time to live
	Status        string  // Status message (e.g., "Success", "Timeout", "Network Unreachable")
	Error         string  // Error message if any
}

// PingConfig holds the configuration for ping operations.
type PingConfig struct {
	Timeout     uint32 // Timeout in milliseconds for each ping
	DataSize    uint16 // Size of the data to send in bytes
	Count       int    // Number of ping attempts per IP
	Interval    uint32 // Interval between pings in milliseconds
	Concurrency int    // Maximum concurrent goroutines
}

// DefaultPingConfig returns the default ping configuration.
func DefaultPingConfig() PingConfig {
	return PingConfig{
		Timeout:     1000,  // 1 second
		DataSize:    32,    // 32 bytes
		Count:       1,     // 1 attempt
		Interval:    0,     // No interval
		Concurrency: 64,    // 64 concurrent goroutines
	}
}

// PingOptions holds extended options for ping operations (UI layer features).
type PingOptions struct {
	ResolveHostName  bool          `json:"resolveHostName"`  // Whether to resolve hostnames via reverse DNS
	DNSTimeout       time.Duration `json:"dnsTimeout"`       // DNS resolution timeout (default 2s)
	EnableRealtime   bool          `json:"enableRealtime"`   // Whether to enable single ping realtime progress
	RealtimeThrottle time.Duration `json:"realtimeThrottle"` // Minimum interval between realtime updates (default 100ms)
}

// DefaultPingOptions returns the default ping options.
func DefaultPingOptions() PingOptions {
	return PingOptions{
		ResolveHostName:  false,
		DNSTimeout:       2 * time.Second,
		EnableRealtime:   false,
		RealtimeThrottle: 100 * time.Millisecond,
	}
}

// Validate validates the ping options.
func (o *PingOptions) Validate() error {
	if o.DNSTimeout < 0 {
		return fmt.Errorf("DNS timeout cannot be negative")
	}
	if o.DNSTimeout > 30*time.Second {
		return fmt.Errorf("DNS timeout too long: %v (max 30s)", o.DNSTimeout)
	}
	if o.RealtimeThrottle < 10*time.Millisecond {
		return fmt.Errorf("realtime throttle too small: %v (min 10ms)", o.RealtimeThrottle)
	}
	return nil
}

// SinglePingResult represents the result of a single ping attempt, used for realtime progress callbacks.
type SinglePingResult struct {
	IP            string  `json:"ip"`            // Target IP address
	Seq           int     `json:"seq"`           // Sequence number (1-based)
	Success       bool    `json:"success"`       // Whether the ping was successful
	RoundTripTime float64 `json:"roundTripTime"` // Round trip time in milliseconds
	TTL           uint8   `json:"ttl"`           // Time to live
	Status        string  `json:"status"`        // Status description
	Error         string  `json:"error"`         // Error message
	Timestamp     int64   `json:"timestamp"`     // Unix millisecond timestamp
}

// PingHostResult represents the aggregated result for a single host.
type PingHostResult struct {
	// === Basic Information ===
	IP       string `json:"ip"`                 // Target IP address
	HostName string `json:"hostName,omitempty"` // Reverse DNS resolved hostname (new)

	// === Status Information ===
	Alive    bool   `json:"alive"`              // Whether the host is alive
	Status   string `json:"status"`             // Status: "online", "offline", "error", "pending"
	ErrorMsg string `json:"errorMsg,omitempty"` // Error message if any

	// === Count Statistics ===
	SentCount   int `json:"sentCount"`   // Number of packets sent
	RecvCount   int `json:"recvCount"`   // Number of packets received (success count)
	FailedCount int `json:"failedCount"` // Number of failed pings (new)

	// === Packet Loss ===
	LossRate float64 `json:"lossRate"` // Packet loss rate (0-100)

	// === RTT Statistics ===
	MinRtt  float64 `json:"minRtt"`            // Minimum RTT (ms), -1 indicates invalid
	MaxRtt  float64 `json:"maxRtt"`            // Maximum RTT (ms)
	AvgRtt  float64 `json:"avgRtt"`            // Average RTT (ms)
	LastRtt float64 `json:"lastRtt,omitempty"` // Last ping RTT (ms) (new)

	// === TTL Information ===
	TTL uint8 `json:"ttl"` // Last successful TTL, 0 indicates no valid value

	// === Timestamps (Unix milliseconds) ===
	LastSucceedAt int64 `json:"lastSucceedAt,omitempty"` // Last success timestamp (new)
	LastFailedAt  int64 `json:"lastFailedAt,omitempty"`  // Last failure timestamp (new)
}

// BatchPingProgress represents the progress of a batch ping operation.
type BatchPingProgress struct {
	TotalIPs      int               `json:"totalIPs"`      // Total number of IPs to ping
	CompletedIPs  int               `json:"completedIPs"`  // Number of IPs completed
	OnlineCount   int               `json:"onlineCount"`   // Number of online hosts
	OfflineCount  int               `json:"offlineCount"`  // Number of offline hosts
	ErrorCount    int               `json:"errorCount"`    // Number of errors
	Progress      float64           `json:"progress"`      // Progress percentage (0-100)
	IsRunning     bool              `json:"isRunning"`     // Whether the operation is running
	StartTime     time.Time         `json:"startTime"`     // Start time of the operation
	ElapsedMs     int64             `json:"elapsedMs"`     // Elapsed time in milliseconds
	Results       []PingHostResult  `json:"results"`       // Results for each IP
}

// NewBatchPingProgress creates a new BatchPingProgress instance.
// ips is the list of target IP addresses, used to pre-populate Results with IP fields
// so that frontend realtime event handlers can match results by IP immediately.
func NewBatchPingProgress(ips []string) *BatchPingProgress {
	totalIPs := len(ips)
	p := &BatchPingProgress{
		TotalIPs:     totalIPs,
		CompletedIPs: 0,
		OnlineCount:  0,
		OfflineCount: 0,
		ErrorCount:   0,
		Progress:     0,
		IsRunning:    true,
		StartTime:    time.Now(),
		ElapsedMs:    0,
		Results:      make([]PingHostResult, totalIPs), // 预分配固定大小，保持顺序
	}
	// 初始化每个 result 的业务初始值，避免 Go 零值导致前端误判
	for i := range p.Results {
		p.Results[i].IP = ips[i]     // 预填充 IP，确保前端实时事件能按 IP 匹配
		p.Results[i].Status = "pending"
		p.Results[i].MinRtt = -1
	}
	return p
}

// UpdateProgress updates the progress based on completed IPs.
func (p *BatchPingProgress) UpdateProgress() {
	if p.TotalIPs > 0 {
		p.Progress = float64(p.CompletedIPs) / float64(p.TotalIPs) * 100
	}
	p.ElapsedMs = time.Since(p.StartTime).Milliseconds()
}

// AddResult adds a result to the progress and updates counters.
// Deprecated: Use SetResult for ordered results.
func (p *BatchPingProgress) AddResult(result PingHostResult) {
	p.Results = append(p.Results, result)
	p.CompletedIPs++

	switch result.Status {
	case "online":
		p.OnlineCount++
	case "offline":
		p.OfflineCount++
	case "error":
		p.ErrorCount++
	}

	p.UpdateProgress()
}

// SetResult sets a result at the specified index and updates counters.
// This method ensures results are stored in input order.
// This method is thread-safe when called with the progressMu lock held.
func (p *BatchPingProgress) SetResult(index int, result PingHostResult) {
	if index < 0 || index >= p.TotalIPs {
		return
	}

	// 检查是否已设置（防止重复计数）
	if p.Results[index].Status != "" && p.Results[index].Status != "pending" {
		return
	}

	p.Results[index] = result
	p.CompletedIPs++

	switch result.Status {
	case "online":
		p.OnlineCount++
	case "offline":
		p.OfflineCount++
	case "error":
		p.ErrorCount++
	}

	p.UpdateProgress()
}

// Finish marks the operation as finished.
func (p *BatchPingProgress) Finish() {
	p.IsRunning = false
	p.UpdateProgress()
}

// Clone creates a deep copy of BatchPingProgress.
func (p *BatchPingProgress) Clone() *BatchPingProgress {
	if p == nil {
		return nil
	}

	clone := &BatchPingProgress{
		TotalIPs:     p.TotalIPs,
		CompletedIPs: p.CompletedIPs,
		OnlineCount:  p.OnlineCount,
		OfflineCount: p.OfflineCount,
		ErrorCount:   p.ErrorCount,
		Progress:     p.Progress,
		IsRunning:    p.IsRunning,
		StartTime:    p.StartTime,
		ElapsedMs:    p.ElapsedMs,
		Results:      make([]PingHostResult, len(p.Results)),
	}
	copy(clone.Results, p.Results)
	return clone
}

// PartialStats represents partial statistics during ping progress.
// Used for real-time updates before a host completes all ping attempts.
type PartialStats struct {
	SentCount   int     `json:"sentCount"`   // Number of packets sent so far
	RecvCount   int     `json:"recvCount"`   // Number of packets received (success count)
	FailedCount int     `json:"failedCount"` // Number of failed pings
	LossRate    float64 `json:"lossRate"`    // Packet loss rate (0-100)
	LastRtt     float64 `json:"lastRtt"`     // Last ping RTT (ms)
	MinRtt      float64 `json:"minRtt"`      // Minimum RTT (ms), -1 indicates invalid
	MaxRtt      float64 `json:"maxRtt"`      // Maximum RTT (ms)
	AvgRtt      float64 `json:"avgRtt"`      // Average RTT (ms)
}

// HostPingUpdate represents intermediate state update during host ping progress.
// This is emitted after each individual ping attempt to provide real-time feedback.
// Named distinctly from PingHostResult to avoid confusion (this is for updates, not final results).
type HostPingUpdate struct {
	IP           string       `json:"ip"`           // Target IP address
	Index        int          `json:"index"`        // Index in the IP list (0-based)
	CurrentSeq   int          `json:"currentSeq"`   // Current ping sequence number (1-based)
	PartialStats PartialStats `json:"partialStats"` // Partial statistics
	IsComplete   bool         `json:"isComplete"`   // Whether this host has completed all pings
	Timestamp    int64        `json:"timestamp"`    // Update timestamp (Unix milliseconds, for ordering)
}

// RunOptions holds optional callbacks for batch ping operations.
// Uses functional options pattern for backward compatibility.
type RunOptions struct {
	OnUpdate     func(*BatchPingProgress) // Called when overall progress updates
	OnSinglePing func(SinglePingResult)   // Called after each individual ping attempt
	OnHostUpdate func(HostPingUpdate)     // Called when host intermediate state changes (new)
}
