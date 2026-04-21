//go:build windows

package ui

import (
	"context"
	"encoding/csv"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/NetWeaverGo/core/internal/icmp"
	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/repository"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// Data size limits for ICMP ping
const (
	MaxAllowedDataSize     = 65500 // Windows API maximum allowed value
	MaxRecommendedDataSize = 8000  // Recommended maximum (considering MTU and fragmentation)
	MTULimit               = 1472  // Ethernet MTU boundary (max ICMP data without fragmentation)
)

// PingRequest represents the request for batch ping operation.
type PingRequest struct {
	Targets   string         `json:"targets"`   // IP addresses, CIDR, or ranges (newline separated)
	Config    icmp.PingConfig `json:"config"`    // Ping configuration
	DeviceIDs []uint         `json:"deviceIds"` // Optional: device IDs to import IPs from
	Options   icmp.PingOptions `json:"options"`   // Ping options (new)
}

// PingCSVResult represents the CSV export result.
type PingCSVResult struct {
	FileName string `json:"fileName"`
	Content  string `json:"content"`
}

// dnsCacheEntry DNS 缓存条目
type dnsCacheEntry struct {
	hostName  string
	timestamp time.Time
}

// PingService provides batch ping functionality.
type PingService struct {
	wailsApp   *application.App
	engine     *icmp.BatchPingEngine
	progress   *icmp.BatchPingProgress
	progressMu sync.RWMutex
	engineMu   sync.Mutex // 保护 engine 创建和状态检查
	repo       repository.DeviceRepository

	// DNS 预解析取消控制
	dnsCancelMu sync.Mutex
	dnsCancel    context.CancelFunc

	// DNS 缓存
	dnsCache    map[string]dnsCacheEntry
	dnsCacheMu  sync.RWMutex
	dnsCacheTTL time.Duration

	// 清理控制
	cleanupStopCh chan struct{}
}

// NewPingService creates a new PingService instance.
func NewPingService() *PingService {
	return &PingService{
		repo:          repository.NewDeviceRepository(),
		dnsCache:      make(map[string]dnsCacheEntry),
		dnsCacheTTL:   5 * time.Minute,
		cleanupStopCh: make(chan struct{}),
	}
}

// ServiceStartup Wails service startup lifecycle hook.
func (s *PingService) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	s.wailsApp = application.Get()
	s.startDNSCacheCleanup() // 启动定期清理
	return nil
}

// ServiceShutdown Wails service shutdown lifecycle hook.
func (s *PingService) ServiceShutdown() error {
	s.stopDNSCacheCleanup() // 终止定时清理，防泄漏
	return nil
}

// 启动定期清理
func (s *PingService) startDNSCacheCleanup() {
	if s.cleanupStopCh == nil {
		s.cleanupStopCh = make(chan struct{})
	}
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.cleanupDNSCache()
			case <-s.cleanupStopCh:
				return
			}
		}
	}()
}

// 停止定期清理
func (s *PingService) stopDNSCacheCleanup() {
	if s.cleanupStopCh != nil {
		close(s.cleanupStopCh)
		s.cleanupStopCh = nil
	}
}

// StartBatchPing starts a batch ping operation.
func (s *PingService) StartBatchPing(req PingRequest) (*icmp.BatchPingProgress, error) {
	// 辅助函数：截断字符串
	truncateString := func(s string, maxLen int) string {
		if len(s) > maxLen {
			return s[:maxLen] + "..."
		}
		return s
	}

	logger.Info("PingService", "-", "收到批量 Ping 请求: targets=%s, deviceIds=%v, resolveHostName=%v",
		truncateString(req.Targets, 100), req.DeviceIDs, req.Options.ResolveHostName)

	// 1. 前置参数处理（无需锁，减少锁持有时间）
	ips, err := s.resolveTargets(req.Targets, req.DeviceIDs)
	if err != nil {
		logger.Error("PingService", "-", "解析目标失败: %v", err)
		return nil, err
	}

	logger.Debug("PingService", "-", "解析目标完成: ipCount=%d", len(ips))
	for i, ip := range ips {
		if i >= 10 {
			logger.Debug("PingService", "-", "  ... 还有 %d 个 IP", len(ips)-10)
			break
		}
		logger.Debug("PingService", "-", "  [%d] %s", i, ip)
	}

	if len(ips) == 0 {
		logger.Error("PingService", "-", "未提供有效的 IP 地址")
		return nil, fmt.Errorf("未提供有效的 IP 地址")
	}

	// Limit maximum IPs
	if len(ips) > 10000 {
		logger.Error("PingService", "-", "IP 数量超过限制: %d (最大 10000)", len(ips))
		return nil, fmt.Errorf("IP 数量超过限制 (最大 10000): 当前 %d 个", len(ips))
	}

	// Merge config with defaults
	config := s.mergeWithDefaultPingConfig(req.Config)

	// 动态并发数：Concurrency == 0 表示"自动"（与目标数一致）
	if config.Concurrency == 0 {
		config.Concurrency = len(ips)
		// 移除硬编码上限，仅在高并发时记录警告
		if config.Concurrency > 256 {
			logger.Warn("PingService", "-", "自动并发数较高: %d，可能影响系统稳定性或触发网络设备防护", config.Concurrency)
		}
	}

	logger.Debug("PingService", "-", "Ping 配置: timeout=%dms, count=%d, dataSize=%d, concurrency=%d",
		config.Timeout, config.Count, config.DataSize, config.Concurrency)

	// Validate data size limits
	if config.DataSize > MaxAllowedDataSize {
		logger.Error("PingService", "-", "数据包大小超过 Windows API 限制: dataSize=%d (最大 %d)",
			config.DataSize, MaxAllowedDataSize)
		return nil, fmt.Errorf("数据包大小超过 Windows API 限制 (最大 %d): 当前 %d",
			MaxAllowedDataSize, config.DataSize)
	}

	// Log warning for large data sizes that may fail due to MTU limits
	if config.DataSize > MaxRecommendedDataSize {
		logger.Warn("PingService", "-", "⚠️ 大数据包警告: dataSize=%d 超过推荐值 %d，可能因 MTU 限制或系统资源不足而失败",
			config.DataSize, MaxRecommendedDataSize)
	} else if config.DataSize > MTULimit {
		logger.Warn("PingService", "-", "⚠️ 数据包大小 %d 超过 MTU 边界 %d，需要 IP 分片，某些网络环境可能失败",
			config.DataSize, MTULimit)
	}

	// Merge options with defaults
	options := s.mergeWithDefaultPingOptions(req.Options)

	// 调试日志：确认选项值
	logger.Debug("PingService", "-", "Ping 选项: EnableRealtime=%v, ResolveHostName=%v, DNSTimeout=%v, RealtimeThrottle=%v",
		options.EnableRealtime, options.ResolveHostName, options.DNSTimeout, options.RealtimeThrottle)
	logger.Debug("PingService", "-", "原始请求选项: %+v", req.Options)

	// 2. 关键区域加锁：检查-设置过程
	s.engineMu.Lock()
	if s.isRunningLocked() {
		s.engineMu.Unlock()
		logger.Warn("PingService", "-", "批量 Ping 正在运行中，拒绝启动新任务")
		return nil, fmt.Errorf("批量 Ping 正在运行中，请先停止当前任务")
	}

	// Create new engine（在锁保护下创建）
	s.engine = icmp.NewBatchPingEngine(config)
	logger.Debug("PingService", "-", "创建新的 Ping 引擎")

	// 初始化 progress
	initialProgress := icmp.NewBatchPingProgress(ips)
	s.setProgress(initialProgress)
	s.engineMu.Unlock() // 尽早释放锁

	logger.Info("PingService", "-", "批量 Ping 已启动: totalIPs=%d", len(ips))

	// 3. 后台执行（无需锁）
	go func() {
		logger.Debug("PingService", "-", "启动后台执行 goroutine")

		// DNS 预解析（如果启用）
		var hostNameMap map[string]string
		if options.ResolveHostName {
			logger.Debug("PingService", "-", "开始 DNS 预解析: ipCount=%d, timeout=%v", len(ips), options.DNSTimeout)
			dnsCtx, dnsCancel := context.WithCancel(context.Background())
			s.dnsCancelMu.Lock()
			s.dnsCancel = dnsCancel
			s.dnsCancelMu.Unlock()
			hostNameMap = s.resolveHostNames(dnsCtx, ips, options.DNSTimeout)
			logger.Debug("PingService", "-", "DNS 预解析完成: resolvedCount=%d", len(hostNameMap))
		}

		// 自适应节流：根据IP数量调整节流间隔
		adaptiveThrottle := s.calculateAdaptiveThrottle(len(ips), options.RealtimeThrottle)
		logger.Debug("PingService", "-", "自适应节流间隔: ipCount=%d, throttle=%v", len(ips), adaptiveThrottle)

		// 准备 HostUpdate 回调（用于实时状态更新）
		var lastHostUpdate sync.Map // map[string]int64 IP -> time
		var onHostUpdate func(icmp.HostPingUpdate)
		if options.EnableRealtime {
			logger.Debug("PingService", "-", "启用主机状态更新回调: EnableRealtime=true")
			onHostUpdate = func(hpu icmp.HostPingUpdate) {
				now := time.Now().UnixMilli()

				// isComplete=true 的事件豁免节流，确保最终状态始终送达前端
				if !hpu.IsComplete {
					// 节流：避免同一IP过于频繁的更新
					if val, ok := lastHostUpdate.Load(hpu.IP); ok {
						if now-val.(int64) < adaptiveThrottle.Milliseconds() {
							logger.Verbose("PingService", hpu.IP, "主机更新被节流: seq=%d", hpu.CurrentSeq)
							return // Throttled
						}
					}
				}

				lastHostUpdate.Store(hpu.IP, now)
				logger.Verbose("PingService", hpu.IP, "发送主机更新: seq=%d, isComplete=%v", hpu.CurrentSeq, hpu.IsComplete)
				s.emitHostUpdate(hpu)
			}
		}

		progress := s.engine.RunWithOptions(context.Background(), ips, icmp.RunOptions{
			OnUpdate: func(p *icmp.BatchPingProgress) {
				// 合并 DNS 结果到 progress（加锁保护，避免与 GetPingProgress 并发读取竞争）
				if hostNameMap != nil {
					s.progressMu.Lock()
					for i := range p.Results {
						if hostName, ok := hostNameMap[p.Results[i].IP]; ok {
							p.Results[i].HostName = hostName
						}
					}
					s.progressMu.Unlock()
				}
				s.setProgress(p)
				s.emitProgress(p)
			},
			OnSinglePing: nil, // 不再使用 realtime 事件
			OnHostUpdate: onHostUpdate,
		})

		// 最终结果也合并 DNS
		if hostNameMap != nil {
			for i := range progress.Results {
				if hostName, ok := hostNameMap[progress.Results[i].IP]; ok {
					progress.Results[i].HostName = hostName
				}
			}
		}

		s.setProgress(progress)
		s.emitProgress(progress)
		logger.Info("PingService", "-", "批量 Ping 后台执行完成")
	}()

	return s.GetPingProgress(), nil
}

// StopBatchPing stops the current batch ping operation.
func (s *PingService) StopBatchPing() error {
	logger.Info("PingService", "-", "收到停止批量 Ping 请求")

	// 取消 DNS 预解析（并发安全）
	s.dnsCancelMu.Lock()
	if s.dnsCancel != nil {
		s.dnsCancel()
		s.dnsCancel = nil
	}
	s.dnsCancelMu.Unlock()

	s.engineMu.Lock()
	if s.engine == nil {
		s.engineMu.Unlock()
		logger.Debug("PingService", "-", "没有正在运行的引擎，无需停止")
		return nil
	}
	engine := s.engine
	s.engine = nil // 立即清理引用，防止后续误用已停止的引擎
	s.engineMu.Unlock()

	logger.Debug("PingService", "-", "调用引擎停止方法")
	engine.Stop()

	// 等待引擎实际停止（带超时，避免永久阻塞）
	timeout := time.After(5 * time.Second)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if !engine.IsRunning() {
				logger.Debug("PingService", "-", "引擎已停止")
				return nil
			}
		case <-timeout:
			logger.Warn("PingService", "-", "等待引擎停止超时(5s)，引擎最终会自行停止")
			return nil
		}
	}
}

// GetPingProgress returns a deep copy of the current progress.
// 返回深拷贝以避免调用者与 OnUpdate 回调之间的数据竞争。
func (s *PingService) GetPingProgress() *icmp.BatchPingProgress {
	s.progressMu.RLock()
	defer s.progressMu.RUnlock()
	if s.progress == nil {
		return nil
	}
	return s.progress.Clone()
}

// isRunningLocked 在已持有 engineMu 锁的情况下检查运行状态
// 必须在 engineMu.Lock() 保护下调用
func (s *PingService) isRunningLocked() bool {
	return s.engine != nil && s.engine.IsRunning()
}

// IsRunning returns whether a ping operation is running.
func (s *PingService) IsRunning() bool {
	s.engineMu.Lock()
	defer s.engineMu.Unlock()
	return s.isRunningLocked()
}

// ExportPingResultCSV exports the ping results as CSV with extended columns.
func (s *PingService) ExportPingResultCSV() (*PingCSVResult, error) {
	progress := s.GetPingProgress()
	if progress == nil || len(progress.Results) == 0 {
		return nil, fmt.Errorf("没有可导出的结果")
	}

	// Create CSV content with UTF-8 BOM for Excel compatibility
	var buf strings.Builder
	buf.WriteString("\xEF\xBB\xBF") // UTF-8 BOM

	writer := csv.NewWriter(&buf)

	// Write header - 扩展表头
	header := []string{
		"序号", "IP 地址", "主机名", "状态",
		"发送次数", "成功次数", "失败次数", "丢包率(%)",
		"最小延迟(ms)", "最大延迟(ms)", "平均延迟(ms)", "最后延迟(ms)",
		"TTL", "最后成功时间", "最后失败时间", "错误信息",
	}
	if err := writer.Write(header); err != nil {
		return nil, err
	}

	// Write data rows
	for i, result := range progress.Results {
		status := "离线"
		if result.Status == "online" {
			status = "在线"
		} else if result.Status == "error" {
			status = "错误"
		}

		row := []string{
			strconv.Itoa(i + 1),
			result.IP,
			result.HostName,
			status,
			strconv.Itoa(result.SentCount),
			strconv.Itoa(result.RecvCount),
			strconv.Itoa(result.FailedCount),
			fmt.Sprintf("%.2f", result.LossRate),
			formatRttForCSV(result.MinRtt),
			formatRttForCSV(result.MaxRtt),
			formatRttForCSV(result.AvgRtt),
			formatRttForCSV(result.LastRtt),
			strconv.Itoa(int(result.TTL)),
			formatTimestamp(result.LastSucceedAt),
			formatTimestamp(result.LastFailedAt),
			result.ErrorMsg,
		}
		if err := writer.Write(row); err != nil {
			return nil, err
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}

	// Generate filename with timestamp
	timestamp := time.Now().Format("20060102_150405")
	fileName := fmt.Sprintf("ping_result_%s.csv", timestamp)

	return &PingCSVResult{
		FileName: fileName,
		Content:  buf.String(),
	}, nil
}

// GetPingDefaultConfig returns the default ping configuration.
func (s *PingService) GetPingDefaultConfig() icmp.PingConfig {
	return icmp.DefaultPingConfig()
}

// PingTargetExpandResult represents the result of expanding ping target syntax sugar.
type PingTargetExpandResult struct {
	IPs     []string `json:"ips"`     // Expanded IP list
	Count   int      `json:"count"`   // Total IP count
	Message string   `json:"message"` // Status message
}

// ExpandPingTargets expands ping target syntax sugar (Wails Binding).
// Supports: CIDR notation, IP ranges (192.168.1.1-10), comma/newline separated IPs.
// Returns expanded IP list for frontend to display.
func (s *PingService) ExpandPingTargets(targets string) *PingTargetExpandResult {
	targets = strings.TrimSpace(targets)
	if targets == "" {
		return &PingTargetExpandResult{
			IPs:     []string{},
			Count:   0,
			Message: "目标地址为空",
		}
	}

	ips, err := s.resolveTargets(targets, nil)
	if err != nil {
		return &PingTargetExpandResult{
			IPs:     []string{},
			Count:   0,
			Message: fmt.Sprintf("解析失败: %v", err),
		}
	}

	if len(ips) == 0 {
		return &PingTargetExpandResult{
			IPs:     []string{},
			Count:   0,
			Message: "未找到有效的 IP 地址",
		}
	}

	message := fmt.Sprintf("成功展开为 %d 个 IP", len(ips))
	if len(ips) > 256 {
		message = fmt.Sprintf("已展开为 %d 个 IP（数量较多，请注意并发设置）", len(ips))
	}

	return &PingTargetExpandResult{
		IPs:     ips,
		Count:   len(ips),
		Message: message,
	}
}

// GetDeviceIPsForPing returns IP addresses for the specified device IDs.
func (s *PingService) GetDeviceIPsForPing(deviceIDs []uint) ([]string, error) {
	if len(deviceIDs) == 0 {
		return []string{}, nil
	}

	var ips []string
	for _, id := range deviceIDs {
		device, err := s.repo.FindByID(id)
		if err != nil {
			continue
		}
		if device.IP != "" {
			ips = append(ips, device.IP)
		}
	}
	return ips, nil
}

// resolveTargets resolves the target string to a list of IP addresses.
func (s *PingService) resolveTargets(targets string, deviceIDs []uint) ([]string, error) {
	var allIPs []string
	seen := make(map[string]struct{})

	// Add IPs from device IDs
	deviceIPs, err := s.GetDeviceIPsForPing(deviceIDs)
	if err != nil {
		return nil, fmt.Errorf("获取设备 IP 失败: %w", err)
	}
	for _, ip := range deviceIPs {
		if _, exists := seen[ip]; !exists {
			seen[ip] = struct{}{}
			allIPs = append(allIPs, ip)
		}
	}

	// 使用 FieldsFunc 同时处理换行、逗号、空格和分号分隔符
	lines := strings.FieldsFunc(targets, func(r rune) bool {
		return r == '\n' || r == ',' || r == ' ' || r == '\t' || r == ';'
	})

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		ips, err := s.parseTargetLine(line)
		if err != nil {
			return nil, fmt.Errorf("解析目标 '%s' 失败: %w", line, err)
		}

		for _, ip := range ips {
			if _, exists := seen[ip]; !exists {
				seen[ip] = struct{}{}
				allIPs = append(allIPs, ip)
			}
		}
	}

	return allIPs, nil
}

// parseTargetLine parses a single target line (IP, CIDR, or range).
func (s *PingService) parseTargetLine(line string) ([]string, error) {
	// Check for CIDR notation
	if strings.Contains(line, "/") {
		return s.expandCIDR(line)
	}

	// Check for IP range (e.g., 192.168.1.1-10 or 192.168.1.1~10)
	if strings.Contains(line, "-") || strings.Contains(line, "~") {
		return s.parseIPRange(line)
	}

	// Single IP
	ip := net.ParseIP(line)
	if ip == nil {
		return nil, fmt.Errorf("无效的 IP 地址: %s", line)
	}
	ip = ip.To4()
	if ip == nil {
		return nil, fmt.Errorf("仅支持 IPv4 地址: %s", line)
	}

	return []string{ip.String()}, nil
}

// expandCIDR expands a CIDR notation to individual IP addresses.
func (s *PingService) expandCIDR(cidr string) ([]string, error) {
	prefix, err := netip.ParsePrefix(cidr)
	if err != nil {
		return nil, fmt.Errorf("无效的 CIDR 格式: %s", cidr)
	}

	// Calculate number of IPs
	addrCount := 1 << (32 - prefix.Bits())

	// Limit expansion size
	if addrCount > 65536 {
		return nil, fmt.Errorf("CIDR 范围过大 (%d 个地址)，最大支持 /16", addrCount)
	}

	// Warn for large ranges
	if addrCount > 4096 {
		// Still allow but could log warning
	}

	var ips []string
	for addr := prefix.Addr(); prefix.Contains(addr); addr = addr.Next() {
		// CIDR 前缀长度的边界处理：
		// - /32（单主机）：返回 1 个 IP，不跳过
		// - /31（点对点）：返回 2 个 IP（RFC 3021），不跳过
		// - /30 及更小：跳过网络地址和广播地址
		if prefix.Bits() < 31 {
			if addr == prefix.Addr() || !prefix.Contains(addr.Next()) {
				continue
			}
		}
		ips = append(ips, addr.String())
	}

	return ips, nil
}

// parseIPRange parses an IP range like 192.168.1.1-10 or 192.168.1.1~10.
func (s *PingService) parseIPRange(rangeStr string) ([]string, error) {
	// Support both - and ~ as separators
	var separator string
	if strings.Contains(rangeStr, "-") {
		separator = "-"
	} else {
		separator = "~"
	}

	parts := strings.Split(rangeStr, separator)
	if len(parts) != 2 {
		return nil, fmt.Errorf("无效的 IP 范围格式: %s", rangeStr)
	}

	baseIP := strings.TrimSpace(parts[0])
	endPart := strings.TrimSpace(parts[1])

	// Parse base IP
	ip := net.ParseIP(baseIP)
	if ip == nil {
		return nil, fmt.Errorf("无效的 IP 地址: %s", baseIP)
	}
	ip = ip.To4()
	if ip == nil {
		return nil, fmt.Errorf("仅支持 IPv4 地址: %s", baseIP)
	}

	// Check if endPart is a full IP or just last octet
	endIP := net.ParseIP(endPart)
	if endIP != nil {
		endIP = endIP.To4()
		if endIP == nil {
			return nil, fmt.Errorf("仅支持 IPv4 地址: %s", endPart)
		}
		// Full IP range - extract last octet
		if !ip.Equal(endIP) {
			// Verify same network (first 3 octets)
			if ip[0] != endIP[0] || ip[1] != endIP[1] || ip[2] != endIP[2] {
				return nil, fmt.Errorf("IP 范围仅支持同一 /24 子网内，如 192.168.1.1-100")
			}
		}
		endPart = strconv.Itoa(int(endIP[3]))
	}

	// Parse end octet
	endOctet, err := strconv.Atoi(endPart)
	if err != nil {
		return nil, fmt.Errorf("无效的范围结束值: %s", endPart)
	}

	startOctet := int(ip[3])

	// Validate range
	if endOctet < startOctet {
		return nil, fmt.Errorf("范围结束值不能小于起始值: %d < %d", endOctet, startOctet)
	}

	// Limit range size
	if endOctet-startOctet > 255 {
		return nil, fmt.Errorf("IP 范围过大，最大支持 256 个地址")
	}

	var ips []string
	for i := startOctet; i <= endOctet; i++ {
		newIP := make(net.IP, 4)
		copy(newIP, ip)
		newIP[3] = byte(i)
		ips = append(ips, newIP.String())
	}

	return ips, nil
}

// mergeWithDefaultPingConfig merges user config with defaults.
// Note: Concurrency == 0 means "auto" (same as target count), not filled with default here.
func (s *PingService) mergeWithDefaultPingConfig(config icmp.PingConfig) icmp.PingConfig {
	defaults := icmp.DefaultPingConfig()

	if config.Timeout == 0 {
		config.Timeout = defaults.Timeout
	}
	if config.DataSize == 0 {
		config.DataSize = defaults.DataSize
	}
	if config.Count == 0 {
		config.Count = defaults.Count
	}
	// Concurrency == 0 means "auto", will be set dynamically in StartBatchPing
	// Do NOT fill with default value here
	if config.Interval == 0 {
		config.Interval = defaults.Interval
	}

	// Apply limits - 允许最大 ICMP 数据包
	if config.Timeout > 30000 {
		config.Timeout = 30000 // Max 30 seconds
	}
	if config.DataSize > 65500 {
		config.DataSize = 65500 // Max ICMP payload size
	}
	// Count 不设上限，允许用户自由配置重试次数
	// 但设置合理上限防止资源耗尽
	if config.Count > 1000 {
		config.Count = 1000
	}
	// 移除并发数硬编码上限，改为警告日志
	// 用户可自行决定并发数，但高并发时记录警告
	if config.Concurrency > 256 {
		logger.Warn("PingService", "-", "⚠️ 高并发设置: concurrency=%d，可能影响系统稳定性或触发网络设备防护", config.Concurrency)
	}
	if config.Interval > 5000 {
		config.Interval = 5000 // Max 5 seconds between pings
	}

	return config
}

// mergeWithDefaultPingOptions merges user options with defaults.
func (s *PingService) mergeWithDefaultPingOptions(options icmp.PingOptions) icmp.PingOptions {
	defaults := icmp.DefaultPingOptions()

	if options.DNSTimeout == 0 {
		options.DNSTimeout = defaults.DNSTimeout
	}
	if options.RealtimeThrottle == 0 {
		options.RealtimeThrottle = defaults.RealtimeThrottle
	}

	// Apply limits
	if options.DNSTimeout > 30*time.Second {
		options.DNSTimeout = 30 * time.Second
	}
	if options.RealtimeThrottle < 10*time.Millisecond {
		options.RealtimeThrottle = 10 * time.Millisecond
	}

	return options
}

// setProgress sets the current progress.
func (s *PingService) setProgress(progress *icmp.BatchPingProgress) {
	s.progressMu.Lock()
	defer s.progressMu.Unlock()
	s.progress = progress
}

// emitProgress emits progress event to frontend.
func (s *PingService) emitProgress(progress *icmp.BatchPingProgress) {
	if progress != nil {
		logger.Verbose("PingService", "-", "发送进度事件: progress=%.1f%%, completed=%d/%d, running=%v",
			progress.Progress, progress.CompletedIPs, progress.TotalIPs, progress.IsRunning)
	}
	if s.wailsApp != nil && s.wailsApp.Event != nil {
		s.wailsApp.Event.Emit("ping:progress", progress)
	}
}

// emitHostUpdate 发送主机中间状态更新到前端
func (s *PingService) emitHostUpdate(update icmp.HostPingUpdate) {
	if s.wailsApp != nil && s.wailsApp.Event != nil {
		s.wailsApp.Event.Emit("ping:host-update", update)
	}
}

// formatRtt formats RTT value for display.
func formatRtt(rtt float64) string {
	if rtt < 0 {
		return "-"
	}
	// rtt == 0 是有效值（Windows IcmpSendEcho 毫秒精度限制，实际延迟 <1ms）
	if rtt == 0 {
		return "<1ms"
	}
	// 支持显示小数点后两位
	if rtt < 1 {
		return fmt.Sprintf("%.3fms", rtt)
	}
	return fmt.Sprintf("%.2fms", rtt)
}

// formatRttForCSV formats RTT value for CSV export.
func formatRttForCSV(rtt float64) string {
	if rtt < 0 {
		return "-"
	}
	return fmt.Sprintf("%.3f", rtt)
}

// formatTimestamp formats Unix millisecond timestamp for display.
func formatTimestamp(ts int64) string {
	if ts == 0 {
		return ""
	}
	return time.UnixMilli(ts).Format("2006-01-02 15:04:05")
}

// resolveHostNames 并行解析主机名（带缓存）
func (s *PingService) resolveHostNames(ctx context.Context, ips []string, timeout time.Duration) map[string]string {
	results := make(map[string]string)
	var mu sync.Mutex
	var wg sync.WaitGroup

	// 先从缓存获取
	var needResolve []string
	now := time.Now()
	for _, ip := range ips {
		s.dnsCacheMu.RLock()
		entry, ok := s.dnsCache[ip]
		s.dnsCacheMu.RUnlock()

		if ok && now.Sub(entry.timestamp) < s.dnsCacheTTL {
			results[ip] = entry.hostName
		} else {
			needResolve = append(needResolve, ip)
		}
	}

	// 并行解析未缓存的 IP（无并发限制，依赖 context 超时控制）
	for _, ip := range needResolve {
		select {
		case <-ctx.Done():
			// 上下文已取消，停止添加新任务
			break
		default:
		}

		wg.Add(1)
		go func(targetIP string) {
			defer wg.Done()

			// 检查上下文是否已取消
			select {
			case <-ctx.Done():
				return
			default:
			}

			resolveCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			// 使用 net.Resolver 支持带 context 的 DNS 查询
			resolver := &net.Resolver{}
			names, err := resolver.LookupAddr(resolveCtx, targetIP)
			if err == nil && len(names) > 0 {
				hostName := strings.TrimSuffix(names[0], ".")
				mu.Lock()
				results[targetIP] = hostName
				mu.Unlock()

				// 更新缓存
				s.dnsCacheMu.Lock()
				s.dnsCache[targetIP] = dnsCacheEntry{
					hostName:  hostName,
					timestamp: time.Now(),
				}
				s.dnsCacheMu.Unlock()
			}
		}(ip)
	}

	wg.Wait()
	return results
}

// clearDNSCache 清除 DNS 缓存
func (s *PingService) clearDNSCache() {
	s.dnsCacheMu.Lock()
	defer s.dnsCacheMu.Unlock()
	s.dnsCache = make(map[string]dnsCacheEntry)
}

// cleanupDNSCache 清理过期缓存条目（可定期调用）
func (s *PingService) cleanupDNSCache() {
	s.dnsCacheMu.Lock()
	defer s.dnsCacheMu.Unlock()

	now := time.Now()
	for ip, entry := range s.dnsCache {
		if now.Sub(entry.timestamp) > s.dnsCacheTTL {
			delete(s.dnsCache, ip)
		}
	}
}

// GetPingDefaultOptions returns the default ping options.
func (s *PingService) GetPingDefaultOptions() icmp.PingOptions {
	return icmp.DefaultPingOptions()
}

// calculateAdaptiveThrottle calculates adaptive throttle interval based on IP count.
// This helps balance UI responsiveness with performance for large batch operations.
func (s *PingService) calculateAdaptiveThrottle(ipCount int, baseThrottle time.Duration) time.Duration {
	// If baseThrottle is already set, use it as a reference
	minThrottle := baseThrottle

	// Adaptive throttling based on IP count
	switch {
	case ipCount < 100:
		// Small batch: frequent updates for better UX
		if minThrottle < 50*time.Millisecond {
			minThrottle = 50 * time.Millisecond
		}
	case ipCount < 500:
		// Medium batch: moderate throttling
		if minThrottle < 100*time.Millisecond {
			minThrottle = 100 * time.Millisecond
		}
	default:
		// Large batch: aggressive throttling to prevent UI lag
		if minThrottle < 200*time.Millisecond {
			minThrottle = 200 * time.Millisecond
		}
	}

	return minThrottle
}

// GetPingEventTypes returns empty instances of event types for frontend type generation.
// This method exists solely to ensure Wails3 generates TypeScript bindings for these types.
func (s *PingService) GetPingEventTypes() (icmp.SinglePingResult, icmp.HostPingUpdate, icmp.PartialStats) {
	return icmp.SinglePingResult{}, icmp.HostPingUpdate{}, icmp.PartialStats{}
}