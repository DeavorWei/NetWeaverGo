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
	"github.com/NetWeaverGo/core/internal/repository"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// PingRequest represents the request for batch ping operation.
type PingRequest struct {
	Targets   string     `json:"targets"`   // IP addresses, CIDR, or ranges (newline separated)
	Config    icmp.PingConfig `json:"config"`    // Ping configuration
	DeviceIDs []uint     `json:"deviceIds"` // Optional: device IDs to import IPs from
}

// PingCSVResult represents the CSV export result.
type PingCSVResult struct {
	FileName string `json:"fileName"`
	Content  string `json:"content"`
}

// PingService provides batch ping functionality.
type PingService struct {
	wailsApp   *application.App
	engine     *icmp.BatchPingEngine
	progress   *icmp.BatchPingProgress
	progressMu sync.RWMutex
	engineMu   sync.Mutex // 保护 engine 创建和状态检查
	repo       repository.DeviceRepository
}

// NewPingService creates a new PingService instance.
func NewPingService() *PingService {
	return &PingService{
		repo: repository.NewDeviceRepository(),
	}
}

// ServiceStartup Wails service startup lifecycle hook.
func (s *PingService) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	s.wailsApp = application.Get()
	return nil
}

// StartBatchPing starts a batch ping operation.
func (s *PingService) StartBatchPing(req PingRequest) (*icmp.BatchPingProgress, error) {
	// 1. 前置参数处理（无需锁，减少锁持有时间）
	ips, err := s.resolveTargets(req.Targets, req.DeviceIDs)
	if err != nil {
		return nil, err
	}

	if len(ips) == 0 {
		return nil, fmt.Errorf("未提供有效的 IP 地址")
	}

	// Limit maximum IPs
	if len(ips) > 10000 {
		return nil, fmt.Errorf("IP 数量超过限制 (最大 10000): 当前 %d 个", len(ips))
	}

	// Merge config with defaults
	config := s.mergeWithDefaultPingConfig(req.Config)

	// 2. 关键区域加锁：检查-设置过程
	s.engineMu.Lock()
	if s.isRunningLocked() {
		s.engineMu.Unlock()
		return nil, fmt.Errorf("批量 Ping 正在运行中，请先停止当前任务")
	}

	// Create new engine（在锁保护下创建）
	s.engine = icmp.NewBatchPingEngine(config)

	// 初始化 progress
	initialProgress := icmp.NewBatchPingProgress(len(ips))
	s.setProgress(initialProgress)
	s.engineMu.Unlock() // 尽早释放锁

	// 3. 后台执行（无需锁）
	go func() {
		progress := s.engine.Run(context.Background(), ips, func(p *icmp.BatchPingProgress) {
			s.setProgress(p)
			s.emitProgress(p)
		})
		s.setProgress(progress)
		s.emitProgress(progress)
	}()

	return s.GetPingProgress(), nil
}

// StopBatchPing stops the current batch ping operation.
func (s *PingService) StopBatchPing() error {
	s.engineMu.Lock()
	defer s.engineMu.Unlock()
	if s.engine == nil {
		return nil
	}
	s.engine.Stop()
	return nil
}

// GetPingProgress returns the current progress.
func (s *PingService) GetPingProgress() *icmp.BatchPingProgress {
	s.progressMu.RLock()
	defer s.progressMu.RUnlock()
	return s.progress
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

// ExportPingResultCSV exports the ping results as CSV.
func (s *PingService) ExportPingResultCSV() (*PingCSVResult, error) {
	progress := s.GetPingProgress()
	if progress == nil || len(progress.Results) == 0 {
		return nil, fmt.Errorf("没有可导出的结果")
	}

	// Create CSV content with UTF-8 BOM for Excel compatibility
	var buf strings.Builder
	buf.WriteString("\xEF\xBB\xBF") // UTF-8 BOM

	writer := csv.NewWriter(&buf)

	// Write header - 修正表头
	header := []string{"序号", "IP 地址", "状态", "平均延迟 (ms)", "最小延迟 (ms)", "最大延迟 (ms)", "TTL", "发送次数", "接收次数", "丢包率 (%)", "错误信息"}
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

		// 修正数据行 - 删除重复的 AvgRtt
		row := []string{
			strconv.Itoa(i + 1),
			result.IP,
			status,
			formatRtt(result.AvgRtt), // 平均延迟
			formatRtt(result.MinRtt), // 最小延迟
			formatRtt(result.MaxRtt), // 最大延迟
			strconv.Itoa(int(result.TTL)),
			strconv.Itoa(result.SentCount),
			strconv.Itoa(result.RecvCount),
			fmt.Sprintf("%.1f", result.LossRate),
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
		// Skip network and broadcast addresses for /31 and larger
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
				return nil, fmt.Errorf("IP 范围必须在同一子网: %s - %s", baseIP, endPart)
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
	if config.Concurrency == 0 {
		config.Concurrency = defaults.Concurrency
	}
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
	if config.Count > 10 {
		config.Count = 10 // Max 10 attempts
	}
	if config.Concurrency > 256 {
		config.Concurrency = 256 // Max 256 concurrent
	}
	if config.Interval > 5000 {
		config.Interval = 5000 // Max 5 seconds between pings
	}

	return config
}

// setProgress sets the current progress.
func (s *PingService) setProgress(progress *icmp.BatchPingProgress) {
	s.progressMu.Lock()
	defer s.progressMu.Unlock()
	s.progress = progress
}

// emitProgress emits progress event to frontend.
func (s *PingService) emitProgress(progress *icmp.BatchPingProgress) {
	if s.wailsApp != nil && s.wailsApp.Event != nil {
		s.wailsApp.Event.Emit("ping:progress", progress)
	}
}

// formatRtt formats RTT value for display.
func formatRtt(rtt float64) string {
	if rtt <= 0 {
		return "-"
	}
	// 支持显示小数点后两位
	if rtt < 1 {
		return fmt.Sprintf("%.3fms", rtt)
	}
	return fmt.Sprintf("%.2fms", rtt)
}