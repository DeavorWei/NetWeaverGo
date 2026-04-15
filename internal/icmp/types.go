// Package icmp provides Windows ICMP API wrappers for batch ping operations.
package icmp

import "time"

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

// PingHostResult represents the aggregated result for a single host.
type PingHostResult struct {
	IP        string  `json:"ip"`        // Target IP address
	Alive     bool    `json:"alive"`     // Whether the host is alive
	SentCount int     `json:"sentCount"` // Number of packets sent
	RecvCount int     `json:"recvCount"` // Number of packets received
	LossRate  float64 `json:"lossRate"`  // Packet loss rate (0-100)
	MinRtt    float64 `json:"minRtt"`    // Minimum round trip time in ms (支持亚毫秒精度)
	MaxRtt    float64 `json:"maxRtt"`    // Maximum round trip time in ms (支持亚毫秒精度)
	AvgRtt    float64 `json:"avgRtt"`    // Average round trip time in ms (支持亚毫秒精度)
	TTL       uint8   `json:"ttl"`       // Time to live
	Status    string  `json:"status"`    // Status: "online", "offline", "error", "pending"
	ErrorMsg  string  `json:"errorMsg"`  // Error message if any
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
func NewBatchPingProgress(totalIPs int) *BatchPingProgress {
	return &BatchPingProgress{
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
