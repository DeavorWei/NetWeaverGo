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
		Count:       3,     // 3 attempts (retry count)
		Interval:    1000,  // 1000ms interval between pings
		Concurrency: 0,     // 0 = auto (same as target count)
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

// SetResult sets a result at the specified index and updates counters.
// This method ensures results are stored in input order.
// This method is thread-safe when called with the progressMu lock held.
func (p *BatchPingProgress) SetResult(index int, result PingHostResult) {
	if index < 0 || index >= p.TotalIPs {
		return
	}

	// 防御性防护：防止未来代码变更引入对同一 index 的重复调用
	// 当前 RunWithOptions 中每个 goroutine 的三条 SetResult 调用路径互斥，不会重复调用
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
	SentCount     int     `json:"sentCount"`               // Number of packets sent so far
	RecvCount     int     `json:"recvCount"`               // Number of packets received (success count)
	FailedCount   int     `json:"failedCount"`             // Number of failed pings
	LossRate      float64 `json:"lossRate"`                // Packet loss rate (0-100)
	LastRtt       float64 `json:"lastRtt"`                 // Last ping RTT (ms)
	MinRtt        float64 `json:"minRtt"`                  // Minimum RTT (ms), -1 indicates invalid
	MaxRtt        float64 `json:"maxRtt"`                  // Maximum RTT (ms)
	AvgRtt        float64 `json:"avgRtt"`                  // Average RTT (ms)
	ErrorMsg      string  `json:"errorMsg,omitempty"`      // Last error message
	LastSucceedAt int64   `json:"lastSucceedAt,omitempty"` // Last success timestamp (Unix ms)
	LastFailedAt  int64   `json:"lastFailedAt,omitempty"` // Last failure timestamp (Unix ms)
	TTL           uint8   `json:"ttl"`                     // Last successful TTL
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

// ==================== Tracert 路径探测类型定义 ====================

// TracertConfig tracert 探测配置
type TracertConfig struct {
	MaxHops     int    `json:"maxHops"`     // 最大跳数 (1-255, 默认 30)
	Timeout     uint32 `json:"timeout"`     // 每跳超时(ms) (默认 3000)
	DataSize    uint16 `json:"dataSize"`    // 数据包大小 (默认 32)
	Count       int    `json:"count"`       // 探测轮次 (1-1000000, 默认 1)，每轮每个 TTL 发送 1 个包
	Interval    uint32 `json:"interval"`    // 探测轮次间隔(ms) (1ms-60000ms, 默认 1000)
	Concurrency int    `json:"concurrency"` // TTL 并发数 (默认 0=全量并发)
}

// DefaultTracertConfig 返回默认 tracert 配置
func DefaultTracertConfig() TracertConfig {
	return TracertConfig{
		MaxHops:     30,
		Timeout:     1500,
		DataSize:    32,
		Count:       1,
		Interval:    1000,
		Concurrency: 0,
	}
}

// TracertHopResult 单跳探测结果
type TracertHopResult struct {
	TTL      int     `json:"ttl"`          // 第几跳
	IP       string  `json:"ip"`           // 响应 IP
	HostName string  `json:"hostName"`     // 主机名 (反向DNS)
	Status   string  `json:"status"`       // "success" / "timeout" / "error" / "pending" / "cancelled"
	SentCount int    `json:"sentCount"`    // 发送报文数量
	RecvCount int    `json:"recvCount"`    // 接收报文数量
	LossRate  float64 `json:"lossRate"`    // 丢包率 (0-100)
	MinRtt    float64 `json:"minRtt"`      // 最低延迟(ms), -1 表示无效
	MaxRtt    float64 `json:"maxRtt"`      // 最高延迟(ms)
	AvgRtt    float64 `json:"avgRtt"`      // 平均延迟(ms)
	LastRtt   float64 `json:"lastRtt"`     // 上次探测延迟(ms)
	Reached   bool    `json:"reached"`     // 是否到达目标
	ErrorMsg  string  `json:"errorMsg"`    // 错误信息
}

// TracertProgress tracert 探测进度
type TracertProgress struct {
	Target        string             `json:"target"`        // 目标地址（用户输入）
	ResolvedIP    string             `json:"resolvedIP"`    // 解析后的 IP
	Round         int                `json:"round"`         // 当前第几轮探测
	TotalHops     int                `json:"totalHops"`     // 总跳数（配置的最大跳数）
	CompletedHops int                `json:"completedHops"` // 已完成跳数
	IsRunning     bool               `json:"isRunning"`     // 是否运行中
	IsContinuous  bool               `json:"isContinuous"`  // 是否持续模式
	StartTime     time.Time          `json:"startTime"`     // 开始时间
	ElapsedMs     int64              `json:"elapsedMs"`     // 已用时间(ms)
	Hops          []TracertHopResult `json:"hops"`          // 各跳结果
	ReachedDest   bool               `json:"reachedDest"`   // 是否到达目的地
	MinReachedTTL int32              `json:"minReachedTtl"` // 所有轮次中到达目标的最小 TTL（0 表示未到达）
}

// NewTracertProgress 创建新的 TracertProgress 实例
func NewTracertProgress(target string, maxHops int) *TracertProgress {
	hops := make([]TracertHopResult, maxHops)
	for i := range hops {
		hops[i] = TracertHopResult{
			TTL:    i + 1,
			Status: "pending",
			MinRtt: -1,
		}
	}
	return &TracertProgress{
		Target:     target,
		TotalHops:  maxHops,
		IsRunning:  true,
		StartTime:  time.Now(),
		Hops:       hops,
	}
}

// UpdateProgress 更新进度统计
func (p *TracertProgress) UpdateProgress() {
	completed := 0
	for _, hop := range p.Hops {
		if hop.Status != "pending" {
			completed++
		}
	}
	p.CompletedHops = completed
	p.ElapsedMs = time.Since(p.StartTime).Milliseconds()
}

// Clone 深拷贝 TracertProgress
func (p *TracertProgress) Clone() *TracertProgress {
	if p == nil {
		return nil
	}
	clone := &TracertProgress{
		Target:        p.Target,
		ResolvedIP:    p.ResolvedIP,
		Round:         p.Round,
		TotalHops:     p.TotalHops,
		CompletedHops: p.CompletedHops,
		IsRunning:     p.IsRunning,
		IsContinuous:  p.IsContinuous,
		StartTime:     p.StartTime,
		ElapsedMs:     p.ElapsedMs,
		ReachedDest:   p.ReachedDest,
		MinReachedTTL: p.MinReachedTTL,
		Hops:          make([]TracertHopResult, len(p.Hops)),
	}
	copy(clone.Hops, p.Hops)
	return clone
}

// CloneForDisplay 创建一个用于前端显示的过滤副本
// 只包含 TTL <= reachedTTL 的跳数结果
func (p *TracertProgress) CloneForDisplay(reachedTTL int32) *TracertProgress {
	if p == nil {
		return nil
	}

	clone := &TracertProgress{
		Target:        p.Target,
		ResolvedIP:    p.ResolvedIP,
		Round:         p.Round,
		TotalHops:     p.TotalHops,
		CompletedHops: p.CompletedHops,
		IsRunning:     p.IsRunning,
		IsContinuous:  p.IsContinuous,
		StartTime:     p.StartTime,
		ElapsedMs:     p.ElapsedMs,
		ReachedDest:   p.ReachedDest,
		MinReachedTTL: p.MinReachedTTL,
	}

	// 如果未到达目标或 reachedTTL 无效，返回全部数据
	if reachedTTL <= 0 || len(p.Hops) == 0 {
		clone.Hops = make([]TracertHopResult, len(p.Hops))
		copy(clone.Hops, p.Hops)
		return clone
	}

	// 只包含 TTL <= reachedTTL 的跳数
	maxTTL := int(reachedTTL)
	if maxTTL > len(p.Hops) {
		maxTTL = len(p.Hops)
	}
	clone.Hops = make([]TracertHopResult, maxTTL)
	copy(clone.Hops, p.Hops[:maxTTL])

	return clone
}

// TracertHopUpdate 单跳中间状态更新（实时推送用）
type TracertHopUpdate struct {
	TTL        int     `json:"ttl"`        // 第几跳
	IP         string  `json:"ip"`         // 响应 IP
	CurrentSeq int     `json:"currentSeq"` // 当前探测序号 (1-based)
	Success    bool    `json:"success"`    // 本次是否成功
	RTT        float64 `json:"rtt"`        // 本次 RTT (ms)
	IsComplete bool    `json:"isComplete"` // 该跳是否全部完成
	Timestamp  int64   `json:"timestamp"`  // 更新时间戳 (Unix ms)
}

// TracertRunOptions tracert 探测回调选项
type TracertRunOptions struct {
	OnUpdate    func(*TracertProgress) // 整体进度回调
	OnHopUpdate func(TracertHopUpdate) // 单跳中间状态回调
}
