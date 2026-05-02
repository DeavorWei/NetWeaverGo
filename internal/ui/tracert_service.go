//go:build windows

package ui

import (
	"context"
	"encoding/csv"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/NetWeaverGo/core/internal/icmp"
	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// TracertRequest represents the request for tracert operation.
type TracertRequest struct {
	Target         string             `json:"target"`         // IP or domain name
	Config         icmp.TracertConfig `json:"config"`         // Tracert configuration
	Continuous     bool               `json:"continuous"`     // Whether to run continuously
	RepeatInterval *int               `json:"repeatInterval,omitempty"` // Repeat interval in ms (default 1000, min 1, max 60000)
}

// TracertExportResult represents the export result.
type TracertExportResult struct {
	FileName string `json:"fileName"`
	Content  string `json:"content"`
}

// TracertResolveResult represents the DNS resolve result.
type TracertResolveResult struct {
	Target     string `json:"target"`
	ResolvedIP string `json:"resolvedIP"`
	HostName   string `json:"hostName"`
	Error      string `json:"error"`
}

// TracertService provides tracert path tracing functionality.
type TracertService struct {
	wailsApp   *application.App
	engine     *icmp.TracertEngine
	progress   *icmp.TracertProgress
	progressMu sync.RWMutex
	engineMu   sync.RWMutex // 使用 RWMutex 支持并发读操作

	// Continuous probe control
	continuousCancel context.CancelFunc
	continuousMu     sync.Mutex

	// DNS cache
	dnsCache   map[string]dnsCacheEntry
	dnsCacheMu sync.RWMutex

	// Cleanup control
	cleanupStopCh chan struct{}
}

// NewTracertService creates a new TracertService instance.
func NewTracertService() *TracertService {
	return &TracertService{
		dnsCache:      make(map[string]dnsCacheEntry),
		cleanupStopCh: make(chan struct{}),
	}
}

// ServiceStartup Wails service startup lifecycle hook.
func (s *TracertService) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	s.wailsApp = application.Get()
	s.startDNSCacheCleanup()
	return nil
}

// ServiceShutdown Wails service shutdown lifecycle hook.
func (s *TracertService) ServiceShutdown() error {
	s.StopTracert()
	s.stopDNSCacheCleanup()
	return nil
}

// startDNSCacheCleanup starts periodic DNS cache cleanup.
func (s *TracertService) startDNSCacheCleanup() {
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

// stopDNSCacheCleanup stops periodic DNS cache cleanup.
func (s *TracertService) stopDNSCacheCleanup() {
	if s.cleanupStopCh != nil {
		close(s.cleanupStopCh)
		s.cleanupStopCh = nil
	}
}

// cleanupDNSCache removes expired cache entries.
func (s *TracertService) cleanupDNSCache() {
	s.dnsCacheMu.Lock()
	defer s.dnsCacheMu.Unlock()
	now := time.Now()
	for ip, entry := range s.dnsCache {
		if now.Sub(entry.timestamp) > 5*time.Minute {
			delete(s.dnsCache, ip)
		}
	}
}

// resolveHopHostNames 并行解析跳数主机名（带缓存）
func (s *TracertService) resolveHopHostNames(ctx context.Context, hops []icmp.TracertHopResult, timeout time.Duration) {
	var mu sync.Mutex
	var wg sync.WaitGroup

	// 收集需要解析的 IP（去重）
	seen := make(map[string]bool)
	var needResolve []struct {
		ttl int
		ip  string
	}

	for _, hop := range hops {
		if hop.IP == "" || hop.IP == "*" || hop.Status == "pending" || hop.Status == "cancelled" {
			continue
		}
		if seen[hop.IP] {
			continue
		}
		seen[hop.IP] = true

		// 先从缓存获取
		s.dnsCacheMu.RLock()
		entry, ok := s.dnsCache[hop.IP]
		s.dnsCacheMu.RUnlock()

		if ok && time.Since(entry.timestamp) < 5*time.Minute {
			// 缓存命中，直接写回
			for i := range hops {
				if hops[i].IP == hop.IP && hops[i].HostName == "" {
					hops[i].HostName = entry.hostName
				}
			}
			continue
		}

		needResolve = append(needResolve, struct {
			ttl int
			ip  string
		}{ttl: hop.TTL, ip: hop.IP})
	}

	if len(needResolve) == 0 {
		return
	}

	logger.Debug("TracertService", "-", "开始反向 DNS 解析: %d 个 IP", len(needResolve))

	// 并行解析未缓存的 IP
	for _, item := range needResolve {
		select {
		case <-ctx.Done():
			break
		default:
		}

		wg.Add(1)
		go func(ip string) {
			defer wg.Done()

			select {
			case <-ctx.Done():
				return
			default:
			}

			resolveCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			resolver := &net.Resolver{}
			names, err := resolver.LookupAddr(resolveCtx, ip)
			if err == nil && len(names) > 0 {
				hostName := strings.TrimSuffix(names[0], ".")

				// 更新缓存
				s.dnsCacheMu.Lock()
				s.dnsCache[ip] = dnsCacheEntry{
					hostName:  hostName,
					timestamp: time.Now(),
				}
				s.dnsCacheMu.Unlock()

				// 写回 hops
				mu.Lock()
				for i := range hops {
					if hops[i].IP == ip && hops[i].HostName == "" {
						hops[i].HostName = hostName
					}
				}
				mu.Unlock()
			}
		}(item.ip)
	}

	wg.Wait()
	logger.Debug("TracertService", "-", "反向 DNS 解析完成")
}

// StartTracert starts a tracert operation.
func (s *TracertService) StartTracert(req TracertRequest) (*icmp.TracertProgress, error) {
	// Validate target
	target := strings.TrimSpace(req.Target)
	if target == "" {
		return nil, fmt.Errorf("请输入目标地址")
	}

	// Merge config with defaults
	config := s.mergeWithDefaultConfig(req.Config)

	// Validate repeat interval (default 1000ms if not specified)
	repeatInterval := 1000
	if req.RepeatInterval != nil {
		repeatInterval = *req.RepeatInterval
		if repeatInterval < 1 {
			repeatInterval = 1000
		}
		if repeatInterval > 60000 {
			repeatInterval = 60000
		}
	}

	logger.Info("TracertService", target, "收到路径探测请求: maxHops=%d, count=%d, continuous=%v, repeatInterval=%dms",
		config.MaxHops, config.Count, req.Continuous, repeatInterval)

	// Check if already running
	s.engineMu.Lock()
	if s.isRunningLocked() {
		s.engineMu.Unlock()
		logger.Warn("TracertService", target, "路径探测正在运行中，拒绝启动新任务")
		return nil, fmt.Errorf("路径探测正在运行中，请先停止当前任务")
	}

	// Create new engine
	s.engine = icmp.NewTracertEngine(config)

	// Initialize progress
	initialProgress := icmp.NewTracertProgress(target, config.MaxHops)
	initialProgress.IsContinuous = req.Continuous
	s.setProgress(initialProgress)
	s.engineMu.Unlock()

	logger.Info("TracertService", target, "路径探测已启动")

	// Cancel any previous continuous probe
	s.continuousMu.Lock()
	if s.continuousCancel != nil {
		s.continuousCancel()
	}
	continuousCtx, continuousCancel := context.WithCancel(context.Background())
	s.continuousCancel = continuousCancel
	s.continuousMu.Unlock()

	// Background execution
	go func() {
		defer continuousCancel()

		if req.Continuous {
			s.runContinuous(continuousCtx, target, config, time.Duration(repeatInterval)*time.Millisecond)
		} else {
			s.runSingle(continuousCtx, target, config)
		}
	}()

	return s.GetTracertProgress(), nil
}

// runSingle runs a single tracert round.
func (s *TracertService) runSingle(ctx context.Context, target string, config icmp.TracertConfig) {
	logger.Debug("TracertService", target, "执行单轮探测")

	progress := s.engine.Run(ctx, target, icmp.TracertRunOptions{
		OnUpdate: func(p *icmp.TracertProgress) {
			s.setProgress(p)
			s.emitProgress(p)
		},
		OnHopUpdate: func(hu icmp.TracertHopUpdate) {
			s.emitHopUpdate(hu)
		},
	})
	log.Printf("[TRACERT SVC] runSingle() engine.Run returned: MinReachedTTL=%d, hopsCount=%d", progress.MinReachedTTL, len(progress.Hops))

	// 反向 DNS 解析（异步，不阻塞主流程）
	dnsCtx, dnsCancel := context.WithTimeout(ctx, 10*time.Second)
	defer dnsCancel()
	s.resolveHopHostNames(dnsCtx, progress.Hops, 2*time.Second)

	// DNS 解析后发送过滤后的进度
	reachedTTL := progress.MinReachedTTL
	filteredProgress := progress.CloneForDisplay(reachedTTL)
	log.Printf("[TRACERT SVC] runSingle() sending filtered: reachedTTL=%d, filteredHops=%d", reachedTTL, len(filteredProgress.Hops))
	s.setProgress(filteredProgress)
	s.emitProgress(filteredProgress)
	logger.Info("TracertService", target, "单轮探测完成")
}

// runContinuous runs continuous tracert rounds.
func (s *TracertService) runContinuous(ctx context.Context, target string, config icmp.TracertConfig, interval time.Duration) {
	logger.Info("TracertService", target, "启动持续探测模式: interval=%v", interval)

	round := 0
	for {
		select {
		case <-ctx.Done():
			logger.Info("TracertService", target, "持续探测被停止")
			return
		default:
		}

		round++
		logger.Debug("TracertService", target, "开始第 %d 轮探测", round)

		// Update round number in progress
		s.progressMu.Lock()
		if s.progress != nil {
			s.progress.Round = round
		}
		s.progressMu.Unlock()

		// Run single round
		roundProgress := s.engine.Run(ctx, target, icmp.TracertRunOptions{
			OnUpdate: func(p *icmp.TracertProgress) {
				// Merge round number
				s.progressMu.Lock()
				if s.progress != nil {
					p.Round = s.progress.Round
				}
				s.progressMu.Unlock()
				s.mergeRoundResult(p)
				// 发送过滤后的进度（只包含 TTL <= MinReachedTTL 的结果）
				reachedTTL := atomic.LoadInt32(&s.progress.MinReachedTTL)
				s.emitProgress(s.progress.CloneForDisplay(reachedTTL))
			},
			OnHopUpdate: func(hu icmp.TracertHopUpdate) {
				s.emitHopUpdate(hu)
			},
		})

		// Check if cancelled
		select {
		case <-ctx.Done():
			logger.Info("TracertService", target, "持续探测被停止")
			return
		default:
		}

		// 检测路径变化：记录合并前的 MinReachedTTL
		oldMinReachedTTL := atomic.LoadInt32(&s.progress.MinReachedTTL)

		// Final merge for this round
		s.mergeRoundResult(roundProgress)

		// 检测路径变化：如果路径变长，发送扩展通知
		newMinReachedTTL := atomic.LoadInt32(&s.progress.MinReachedTTL)
		if newMinReachedTTL > oldMinReachedTTL && oldMinReachedTTL > 0 {
			log.Printf("[TRACERT SVC] 路径变长: %d → %d 跳", oldMinReachedTTL, newMinReachedTTL)
		}

		// 反向 DNS 解析（异步，不阻塞主流程）
		dnsCtx, dnsCancel := context.WithTimeout(ctx, 10*time.Second)
		s.resolveHopHostNames(dnsCtx, s.progress.Hops, 2*time.Second)
		dnsCancel()

		// 检查是否在 DNS 解析期间被取消
		select {
		case <-ctx.Done():
			logger.Info("TracertService", target, "持续探测被停止")
			return
		default:
		}

		// Update running state for the wait period
		s.progressMu.Lock()
		if s.progress != nil {
			s.progress.IsRunning = true // Still running in continuous mode
		}
		s.progressMu.Unlock()

		// 发送过滤后的进度
		reachedTTL := s.progress.MinReachedTTL
		log.Printf("[TRACERT SVC] runContinuous() round done: MinReachedTTL=%d, hopsCount=%d", s.progress.MinReachedTTL, len(s.progress.Hops))
		s.emitProgress(s.progress.CloneForDisplay(reachedTTL))
		logger.Debug("TracertService", target, "第 %d 轮探测完成，等待 %v", round, interval)

		// Wait for interval
		select {
		case <-ctx.Done():
			logger.Info("TracertService", target, "持续探测被停止")
			return
		case <-time.After(interval):
		}
	}
}

// mergeRoundResult merges a round's results into the accumulated progress.
func (s *TracertService) mergeRoundResult(roundResult *icmp.TracertProgress) {
	s.progressMu.Lock()
	defer s.progressMu.Unlock()

	if s.progress == nil || roundResult == nil {
		return
	}

	// 获取当前 MinReachedTTL，跳过 TTL > MinReachedTTL 的结果
	minReachedTTL := atomic.LoadInt32(&s.progress.MinReachedTTL)

	s.progress.ResolvedIP = roundResult.ResolvedIP
	s.progress.ReachedDest = roundResult.ReachedDest

	for i, newHop := range roundResult.Hops {
		// 跳过 TTL > MinReachedTTL 的结果（这些应该保持 cancelled 状态）
		if minReachedTTL > 0 && int32(newHop.TTL) > minReachedTTL {
			continue
		}

		if i >= len(s.progress.Hops) {
			s.progress.Hops = append(s.progress.Hops, newHop)
			continue
		}

		existing := &s.progress.Hops[i]

		// First round or if existing is still pending, just copy
		if existing.Status == "pending" {
			*existing = newHop
			continue
		}

		// Skip cancelled hops - they should not participate in RTT statistics
		if newHop.Status == "cancelled" {
			// Only update status if existing is also cancelled or pending
			if existing.Status == "cancelled" || existing.Status == "pending" {
				existing.Status = "cancelled"
			}
			continue
		}
		if existing.Status == "cancelled" {
			// If existing was cancelled but new hop has real data, replace
			if newHop.Status != "cancelled" && newHop.Status != "pending" {
				*existing = newHop
			}
			continue
		}

		// Accumulate statistics
		// 注意：必须先保存旧的 RecvCount，再累加，否则加权平均计算会出错
		oldRecvCount := existing.RecvCount
		existing.SentCount += newHop.SentCount
		existing.RecvCount += newHop.RecvCount

		if existing.SentCount > 0 {
			existing.LossRate = float64(existing.SentCount-existing.RecvCount) / float64(existing.SentCount) * 100
		}

		if newHop.RecvCount > 0 {
			// Update RTT statistics
			if existing.MinRtt < 0 || (newHop.MinRtt >= 0 && newHop.MinRtt < existing.MinRtt) {
				existing.MinRtt = newHop.MinRtt
			}
			if newHop.MaxRtt > existing.MaxRtt {
				existing.MaxRtt = newHop.MaxRtt
			}
			// Weighted average: use old RecvCount for correct calculation
			totalRtt := existing.AvgRtt*float64(oldRecvCount) +
				newHop.AvgRtt*float64(newHop.RecvCount)
			if existing.RecvCount > 0 {
				existing.AvgRtt = totalRtt / float64(existing.RecvCount)
			}
			existing.LastRtt = newHop.LastRtt
		}

		// Update IP and hostname (take latest non-empty)
		if newHop.IP != "" && newHop.IP != "*" {
			existing.IP = newHop.IP
		}
		if newHop.HostName != "" {
			existing.HostName = newHop.HostName
		}
		if newHop.Reached {
			existing.Reached = true
		}
		if newHop.Status != "" && newHop.Status != "pending" {
			existing.Status = newHop.Status
		}
		if newHop.ErrorMsg != "" {
			existing.ErrorMsg = newHop.ErrorMsg
		}
	}

	// 更新 MinReachedTTL（如果新轮次发现了更小的到达 TTL）
	if roundResult.MinReachedTTL > 0 {
		currentMin := atomic.LoadInt32(&s.progress.MinReachedTTL)
		if currentMin == 0 || roundResult.MinReachedTTL < currentMin {
			atomic.StoreInt32(&s.progress.MinReachedTTL, roundResult.MinReachedTTL)
		}
	}

	s.progress.UpdateProgress()
}

// StopTracert stops the current tracert operation.
func (s *TracertService) StopTracert() error {
	logger.Info("TracertService", "-", "收到停止路径探测请求")

	// Cancel continuous probe
	s.continuousMu.Lock()
	if s.continuousCancel != nil {
		s.continuousCancel()
		s.continuousCancel = nil
	}
	s.continuousMu.Unlock()

	// Stop engine
	s.engineMu.Lock()
	if s.engine == nil {
		s.engineMu.Unlock()
		logger.Debug("TracertService", "-", "没有正在运行的引擎，无需停止")
		return nil
	}
	engine := s.engine
	s.engine = nil
	s.engineMu.Unlock()

	logger.Debug("TracertService", "-", "调用引擎停止方法")
	engine.Stop()

	// Wait for engine to stop
	timeout := time.After(5 * time.Second)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if !engine.IsRunning() {
				logger.Debug("TracertService", "-", "引擎已停止")
				// Mark progress as stopped
				s.progressMu.Lock()
				if s.progress != nil {
					s.progress.IsRunning = false
					s.progress.UpdateProgress()
				}
				p := s.progress
				s.progressMu.Unlock()
				// 推送最终状态到前端
				s.emitProgress(p)
				return nil
			}
		case <-timeout:
			logger.Warn("TracertService", "-", "等待引擎停止超时(5s)")
			s.progressMu.Lock()
			if s.progress != nil {
				s.progress.IsRunning = false
				s.progress.UpdateProgress()
			}
			p := s.progress
			s.progressMu.Unlock()
			// 推送最终状态到前端
			s.emitProgress(p)
			return nil
		}
	}
}

// GetTracertProgress returns a deep copy of the current progress.
func (s *TracertService) GetTracertProgress() *icmp.TracertProgress {
	s.progressMu.RLock()
	defer s.progressMu.RUnlock()
	if s.progress == nil {
		return nil
	}
	return s.progress.Clone()
}

// isRunningLocked checks if the engine is running (must hold engineMu).
func (s *TracertService) isRunningLocked() bool {
	return s.engine != nil && s.engine.IsRunning()
}

// IsRunning returns whether a tracert operation is running.
// 使用 RLock 允许并发读取，提高性能
func (s *TracertService) IsRunning() bool {
	s.engineMu.RLock()
	defer s.engineMu.RUnlock()
	return s.isRunningLocked()
}

// GetTracertDefaultConfig returns the default tracert configuration.
func (s *TracertService) GetTracertDefaultConfig() icmp.TracertConfig {
	return icmp.DefaultTracertConfig()
}

// ResolveTarget resolves a target (domain name to IP).
func (s *TracertService) ResolveTarget(target string) *TracertResolveResult {
	target = strings.TrimSpace(target)
	if target == "" {
		return &TracertResolveResult{Error: "目标地址为空"}
	}

	// Try parsing as IP first
	ip := net.ParseIP(target)
	if ip != nil {
		if ip4 := ip.To4(); ip4 != nil {
			return &TracertResolveResult{
				Target:     target,
				ResolvedIP: ip4.String(),
			}
		}
		return &TracertResolveResult{Target: target, Error: "仅支持 IPv4 地址"}
	}

	// DNS lookup
	ips, err := net.LookupIP(target)
	if err != nil {
		return &TracertResolveResult{Target: target, Error: fmt.Sprintf("DNS 解析失败: %v", err)}
	}

	for _, ip := range ips {
		if ip4 := ip.To4(); ip4 != nil {
			return &TracertResolveResult{
				Target:     target,
				ResolvedIP: ip4.String(),
			}
		}
	}

	return &TracertResolveResult{Target: target, Error: "未找到 IPv4 地址"}
}

// ExportTracertResultCSV exports the tracert results as CSV.
func (s *TracertService) ExportTracertResultCSV() (*TracertExportResult, error) {
	progress := s.GetTracertProgress()
	if progress == nil || len(progress.Hops) == 0 {
		return nil, fmt.Errorf("没有可导出的结果")
	}

	// 使用 MinReachedTTL 过滤数据
	filteredProgress := progress.CloneForDisplay(progress.MinReachedTTL)

	var buf strings.Builder
	buf.WriteString("\xEF\xBB\xBF") // UTF-8 BOM for Excel

	writer := csv.NewWriter(&buf)

	// Header
	header := []string{
		"跳数", "主机名", "响应IP", "丢包率(%)",
		"发送报文", "接收报文", "最低延迟(ms)", "平均延迟(ms)",
		"最高延迟(ms)", "上次延迟(ms)", "状态", "错误信息",
	}
	if err := writer.Write(header); err != nil {
		return nil, err
	}

	// Data rows
	for _, hop := range filteredProgress.Hops {
		if hop.Status == "pending" || hop.Status == "cancelled" {
			continue
		}

		statusText := hop.Status
		switch hop.Status {
		case "success":
			if hop.Reached {
				statusText = "到达目标"
			} else {
				statusText = "中间路由"
			}
		case "timeout":
			statusText = "超时"
		case "error":
			statusText = "错误"
		}

		row := []string{
			fmt.Sprintf("%d", hop.TTL),
			hop.HostName,
			hop.IP,
			fmt.Sprintf("%.2f", hop.LossRate),
			fmt.Sprintf("%d", hop.SentCount),
			fmt.Sprintf("%d", hop.RecvCount),
			formatTracertRtt(hop.MinRtt),
			formatTracertRtt(hop.AvgRtt),
			formatTracertRtt(hop.MaxRtt),
			formatTracertRtt(hop.LastRtt),
			statusText,
			hop.ErrorMsg,
		}
		if err := writer.Write(row); err != nil {
			return nil, err
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}

	timestamp := time.Now().Format("20060102_150405")
	fileName := fmt.Sprintf("tracert_%s_%s.csv", sanitizeFilename(progress.Target), timestamp)

	return &TracertExportResult{
		FileName: fileName,
		Content:  buf.String(),
	}, nil
}

// ExportTracertResultTXT exports the tracert results as TXT.
func (s *TracertService) ExportTracertResultTXT() (*TracertExportResult, error) {
	progress := s.GetTracertProgress()
	if progress == nil || len(progress.Hops) == 0 {
		return nil, fmt.Errorf("没有可导出的结果")
	}

	// 使用 MinReachedTTL 过滤数据
	filteredProgress := progress.CloneForDisplay(progress.MinReachedTTL)

	var buf strings.Builder

	buf.WriteString("Tracert 路径探测报告\n")
	buf.WriteString("====================\n")
	buf.WriteString(fmt.Sprintf("目标: %s (%s)\n", filteredProgress.Target, filteredProgress.ResolvedIP))
	buf.WriteString(fmt.Sprintf("探测时间: %s\n", filteredProgress.StartTime.Format("2006-01-02 15:04:05")))
	buf.WriteString(fmt.Sprintf("总轮次: %d\n", filteredProgress.Round))
	buf.WriteString(fmt.Sprintf("到达目的地: %v\n", filteredProgress.ReachedDest))
	buf.WriteString("\n")

	// Table header
	buf.WriteString(fmt.Sprintf("%-5s %-25s %-18s %8s %6s %6s %10s %10s %10s %10s\n",
		"跳数", "主机名", "响应IP", "丢包率", "发送", "接收", "最低延迟", "平均延迟", "最高延迟", "上次延迟"))
	buf.WriteString(fmt.Sprintf("%-5s %-25s %-18s %8s %6s %6s %10s %10s %10s %10s\n",
		"----", "-------------------------", "------------------", "--------", "------", "------", "----------", "----------", "----------", "----------"))

	for _, hop := range filteredProgress.Hops {
		if hop.Status == "pending" || hop.Status == "cancelled" {
			continue
		}

		ip := hop.IP
		if ip == "" {
			ip = "*"
		}
		hostName := hop.HostName
		if hostName == "" {
			hostName = "-"
		}

		buf.WriteString(fmt.Sprintf("%-5d %-25s %-18s %7.1f%% %6d %6d %10s %10s %10s %10s\n",
			hop.TTL,
			truncateString(hostName, 25),
			ip,
			hop.LossRate,
			hop.SentCount,
			hop.RecvCount,
			formatTracertRtt(hop.MinRtt),
			formatTracertRtt(hop.AvgRtt),
			formatTracertRtt(hop.MaxRtt),
			formatTracertRtt(hop.LastRtt),
		))
	}

	timestamp := time.Now().Format("20060102_150405")
	fileName := fmt.Sprintf("tracert_%s_%s.txt", sanitizeFilename(filteredProgress.Target), timestamp)

	return &TracertExportResult{
		FileName: fileName,
		Content:  buf.String(),
	}, nil
}

// mergeWithDefaultConfig merges user config with defaults.
func (s *TracertService) mergeWithDefaultConfig(config icmp.TracertConfig) icmp.TracertConfig {
	defaults := icmp.DefaultTracertConfig()

	if config.MaxHops == 0 {
		config.MaxHops = defaults.MaxHops
	}
	if config.MaxHops < 1 {
		config.MaxHops = 1
	}
	if config.MaxHops > 255 {
		config.MaxHops = 255
	}
	if config.Timeout == 0 {
		config.Timeout = defaults.Timeout
	}
	if config.Timeout > 30000 {
		config.Timeout = 30000
	}
	if config.DataSize == 0 {
		config.DataSize = defaults.DataSize
	}
	if config.DataSize > 65500 {
		config.DataSize = 65500
	}
	if config.Count == 0 {
		config.Count = defaults.Count
	}
	if config.Count < 1 {
		config.Count = 1
	}
	if config.Count > 1000000 {
		config.Count = 1000000
	}
	if config.Interval == 0 {
		config.Interval = defaults.Interval
	}
	if config.Interval < 1 {
		config.Interval = 1
	}
	if config.Interval > 60000 {
		config.Interval = 60000
	}

	return config
}

// setProgress sets the current progress.
func (s *TracertService) setProgress(progress *icmp.TracertProgress) {
	s.progressMu.Lock()
	defer s.progressMu.Unlock()
	s.progress = progress
}

// emitProgress emits progress event to frontend.
func (s *TracertService) emitProgress(progress *icmp.TracertProgress) {
	if progress != nil {
		logger.Verbose("TracertService", "-", "发送进度事件: round=%d, completed=%d/%d, running=%v",
			progress.Round, progress.CompletedHops, progress.TotalHops, progress.IsRunning)
		log.Printf("[TRACERT SVC] emitProgress: round=%d, reachedTTL=%d, hopsCount=%d, isRunning=%v", progress.Round, progress.MinReachedTTL, len(progress.Hops), progress.IsRunning)
	}
	if s.wailsApp != nil && s.wailsApp.Event != nil {
		s.wailsApp.Event.Emit("tracert:progress", progress)
	}
}

// emitHopUpdate emits hop update event to frontend.
func (s *TracertService) emitHopUpdate(update icmp.TracertHopUpdate) {
	log.Printf("[TRACERT SVC] emitHopUpdate: TTL=%d, IP=%s, Success=%v, RTT=%.2fms",
		update.TTL, update.IP, update.Success, update.RTT)
	if s.wailsApp != nil && s.wailsApp.Event != nil {
		s.wailsApp.Event.Emit("tracert:hop-update", update)
	}
}

// GetTracertEventTypes returns empty instances of event types for frontend type generation.
func (s *TracertService) GetTracertEventTypes() (icmp.TracertHopUpdate, icmp.TracertHopResult) {
	return icmp.TracertHopUpdate{}, icmp.TracertHopResult{}
}

// Helper functions

func formatTracertRtt(rtt float64) string {
	if rtt < 0 {
		return "-"
	}
	if rtt == 0 {
		return "<1ms"
	}
	if rtt < 1 {
		return fmt.Sprintf("%.3fms", rtt)
	}
	return fmt.Sprintf("%.2fms", rtt)
}

func sanitizeFilename(name string) string {
	// Replace characters that are invalid in filenames
	replacer := strings.NewReplacer(
		":", "_", "/", "_", "\\", "_",
		"*", "_", "?", "_", "\"", "_",
		"<", "_", ">", "_", "|", "_",
		" ", "_",
	)
	result := replacer.Replace(name)
	if len(result) > 50 {
		result = result[:50]
	}
	return result
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
