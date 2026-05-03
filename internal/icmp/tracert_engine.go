//go:build windows

package icmp

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/NetWeaverGo/core/internal/logger"
)

// TracertEngine tracert 路径探测引擎
type TracertEngine struct {
	config    TracertConfig
	cancel    context.CancelFunc
	running   bool
	runningMu sync.RWMutex
}

// NewTracertEngine 创建新的 TracertEngine
func NewTracertEngine(config TracertConfig) *TracertEngine {
	// 应用默认值
	if config.MaxHops == 0 {
		config.MaxHops = DefaultTracertConfig().MaxHops
	}
	if config.MaxHops < 1 {
		config.MaxHops = 1
	}
	if config.MaxHops > 255 {
		config.MaxHops = 255
	}
	if config.Timeout == 0 {
		config.Timeout = DefaultTracertConfig().Timeout
	}
	if config.DataSize == 0 {
		config.DataSize = DefaultTracertConfig().DataSize
	}
	if config.Count == 0 {
		config.Count = DefaultTracertConfig().Count
	}
	if config.Count < 1 {
		config.Count = 1
	}
	if config.Count > 1000000 {
		config.Count = 1000000
	}
	if config.Interval == 0 {
		config.Interval = DefaultTracertConfig().Interval
	}
	if config.Interval < 1 {
		config.Interval = 1
	}
	if config.Interval > 60000 {
		config.Interval = 60000
	}

	return &TracertEngine{
		config: config,
	}
}

// Run 执行 tracert 路径探测
// Count 参数控制探测轮次：每轮对所有 TTL 各发送 1 个包，结果累积到 progress 中
func (e *TracertEngine) Run(ctx context.Context, target string, opts TracertRunOptions) *TracertProgress {
	logger.Info("Tracert", target, "路径探测开始: maxHops=%d, count=%d, timeout=%dms, interval=%dms, concurrency=%d",
		e.config.MaxHops, e.config.Count, e.config.Timeout, e.config.Interval, e.config.Concurrency)
	log.Printf("[TRACERT] Run() start: target=%s, maxHops=%d, count=%d", target, e.config.MaxHops, e.config.Count)

	// 创建可取消的 context
	runCtx, cancel := context.WithCancel(ctx)

	e.runningMu.Lock()
	if e.running {
		logger.Warn("Tracert", target, "引擎已在运行中，拒绝启动新任务")
		e.runningMu.Unlock()
		cancel()
		return NewTracertProgress(target, e.config.MaxHops)
	}
	e.running = true
	e.cancel = cancel
	e.runningMu.Unlock()

	// DNS 解析
	resolvedIP, err := e.resolveTarget(target)
	if err != nil {
		logger.Error("Tracert", target, "DNS 解析失败: %v", err)
		e.runningMu.Lock()
		e.running = false
		e.cancel = nil
		e.runningMu.Unlock()
		cancel()

		progress := NewTracertProgress(target, e.config.MaxHops)
		progress.IsRunning = false
		progress.ElapsedMs = time.Since(progress.StartTime).Milliseconds()
		return progress
	}

	logger.Info("Tracert", target, "DNS 解析完成: resolvedIP=%s", resolvedIP)

	progress := NewTracertProgress(target, e.config.MaxHops)
	progress.ResolvedIP = resolvedIP

	// 统一清理逻辑
	defer func() {
		logger.Debug("Tracert", target, "清理资源，标记完成状态")
		e.runningMu.Lock()
		e.running = false
		e.cancel = nil
		e.runningMu.Unlock()

		progress.IsRunning = false
		progress.UpdateProgress()
		logger.Info("Tracert", target, "路径探测完成: round=%d, hops=%d, reached=%v, elapsed=%dms",
			progress.Round, progress.CompletedHops, progress.ReachedDest, progress.ElapsedMs)
		log.Printf("[TRACERT] Run() end: MinReachedTTL=%d, hopsCount=%d", atomic.LoadInt32(&progress.MinReachedTTL), len(progress.Hops))
		// 发送最终过滤后的进度
		if opts.OnUpdate != nil {
			finalReachedTTL := atomic.LoadInt32(&progress.MinReachedTTL)
			filteredProgress := progress.CloneForDisplay(finalReachedTTL)
			log.Printf("[TRACERT] Run() sending final update: reachedTTL=%d, filteredHops=%d", finalReachedTTL, len(filteredProgress.Hops))
			opts.OnUpdate(filteredProgress)
		}
	}()

	// 执行多轮探测（Count 控制轮次）
	for round := 1; round <= e.config.Count; round++ {
		// 检查取消
		select {
		case <-runCtx.Done():
			logger.Debug("Tracert", target, "探测被取消，已完成 %d 轮", round-1)
			return progress
		default:
		}

		logger.Debug("Tracert", target, "开始第 %d/%d 轮探测", round, e.config.Count)
		log.Printf("[TRACERT] runRound() start: round=%d", round)
		progress.Round = round

		// 执行单轮探测
		e.runRound(runCtx, resolvedIP, progress, opts)
		log.Printf("[TRACERT] runRound() end: round=%d, reachedTTL=%d, hopsCount=%d", round, atomic.LoadInt32(&progress.MinReachedTTL), len(progress.Hops))

		// 注意：不再提前结束探测，保持全量探测以应对网络路径变化
		// 前端根据 reachedTTL 过滤显示结果

		// 轮次间隔（最后一轮不需要等待）
		if round < e.config.Count && e.config.Interval > 0 {
			select {
			case <-runCtx.Done():
				logger.Debug("Tracert", target, "探测被取消")
				return progress
			case <-time.After(time.Duration(e.config.Interval) * time.Millisecond):
			}
		}
	}

	// 后处理：标记 TTL > MinReachedTTL 的结果状态，供前端过滤显示
	// 在所有轮次完成后统一执行，避免多轮探测时 cancelled 状态被后续轮次覆盖
	minReachedTTL := atomic.LoadInt32(&progress.MinReachedTTL)
	if minReachedTTL > 0 && minReachedTTL <= int32(e.config.MaxHops) {
		logger.Debug("Tracert", target, "后处理清理: MinReachedTTL=%d，标记 TTL > %d 的结果为 cancelled", minReachedTTL, minReachedTTL)
		for i := int(minReachedTTL); i < len(progress.Hops); i++ {
			hop := &progress.Hops[i]
			// 保留到达目标的那一跳（如果有多个 TTL 都到达目标）
			if hop.Reached {
				continue
			}
			// 将 TTL > MinReachedTTL 的结果标记为 cancelled
			hop.Status = "cancelled"
			hop.IP = "*"
			hop.ErrorMsg = ""
			hop.MinRtt = -1
			hop.AvgRtt = 0
			hop.MaxRtt = 0
			hop.LastRtt = 0
			hop.SentCount = 0
			hop.RecvCount = 0
			hop.LossRate = 0
		}
	}

	// 处理剩余 pending 状态
	// 只在已到达目标时才将 pending 标记为 cancelled
	// 如果未到达目标，pending 状态应改为 timeout（表示探测完成但无响应）
	for i := range progress.Hops {
		if progress.Hops[i].Status == "pending" {
			if progress.ReachedDest {
				progress.Hops[i].Status = "cancelled"
			} else {
				progress.Hops[i].Status = "timeout"
			}
		}
	}

	return progress
}

// runRound 执行单轮 tracert 探测（并发探测模式）
func (e *TracertEngine) runRound(ctx context.Context, targetIP string, progress *TracertProgress, opts TracertRunOptions) {
	ip := net.ParseIP(targetIP)
	if ip == nil {
		logger.Error("Tracert", targetIP, "无效的 IP 地址")
		return
	}

	maxHops := e.config.MaxHops

	logger.Debug("Tracert", targetIP, "开始并发 TTL 探测: maxHops=%d", maxHops)

	// 用于标记是否已到达目标（原子操作）
	var reachedDest int32 = 0
	var reachedTTL int32 = int32(maxHops + 1) // 记录到达目标的最小 TTL，初始化为较大值

	// 结果通道
	resultChan := make(chan TracertHopResult, maxHops)

	// 并发控制
	var wg sync.WaitGroup

	// 启动并发探测（全量探测，不提前停止）
	for ttl := 1; ttl <= maxHops; ttl++ {
		// 检查取消（仅响应外部取消请求）
		select {
		case <-ctx.Done():
			logger.Debug("Tracert", targetIP, "检测到取消信号，停止启动新探测")
			// 发送 cancelled 结果到 channel（串行化处理）
			for t := ttl; t <= maxHops; t++ {
				resultChan <- TracertHopResult{
					TTL:    t,
					Status: "cancelled",
					MinRtt: -1,
				}
			}
			break
		default:
		}

		wg.Add(1)
		go func(ttlVal int) {
			defer wg.Done()

			// 检查取消（仅响应外部取消请求）
			select {
			case <-ctx.Done():
				resultChan <- TracertHopResult{
					TTL:    ttlVal,
					Status: "cancelled",
					MinRtt: -1,
				}
				return
			default:
			}

			// 探测当前 TTL
			hopResult := e.probeHop(ctx, ip, ttlVal, opts)

			// 如果到达目标，记录最小 TTL（用于前端显示过滤）
			if hopResult.Reached {
				// 使用原子操作记录最小 TTL
				for {
					oldReachedTTL := atomic.LoadInt32(&reachedTTL)
					if int32(ttlVal) >= oldReachedTTL {
						// 当前 TTL 不是更小，不更新
						break
					}
					if atomic.CompareAndSwapInt32(&reachedTTL, oldReachedTTL, int32(ttlVal)) {
						// 成功更新为更小的 TTL
						atomic.StoreInt32(&reachedDest, 1)
						logger.Debug("Tracert", targetIP, "TTL=%d 到达目标（记录最小 TTL，之前=%d）", ttlVal, oldReachedTTL)
						break
					}
					// CAS 失败，重试
				}
			}

			resultChan <- hopResult
		}(ttl)
	}

	// 等待所有探测完成
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// 收集结果（所有对 progress.Hops 的修改都在此处串行执行）
	// 注意：不再丢弃 TTL > reachedTTL 的结果，保持全量探测结果
	// 后处理清理会根据 reachedTTL 标记超出部分的状态，供前端过滤显示
	// 【实时逐跳显示】每收到一个 TTL 结果立即发送更新到前端
	for hopResult := range resultChan {
		// 1. 立即更新 progress 中的对应跳
		if hopResult.TTL-1 < len(progress.Hops) {
			existing := &progress.Hops[hopResult.TTL-1]
			e.mergeHopResult(existing, hopResult)

			// 2. 更新进度统计
			progress.CompletedHops++
			progress.ElapsedMs = time.Since(progress.StartTime).Milliseconds()

			// 3. 如果到达目标，更新 MinReachedTTL
			if hopResult.Reached {
				currentMin := atomic.LoadInt32(&progress.MinReachedTTL)
				if currentMin == 0 || int32(hopResult.TTL) < currentMin {
					atomic.StoreInt32(&progress.MinReachedTTL, int32(hopResult.TTL))
					atomic.StoreInt32(&reachedTTL, int32(hopResult.TTL))
				}
			}

			// 4. 立即发送实时更新到前端（关键修改！）
			if opts.OnUpdate != nil {
				currentReachedTTL := atomic.LoadInt32(&progress.MinReachedTTL)
				opts.OnUpdate(progress.CloneForDisplay(currentReachedTTL))
			}

			// 5. 发送单跳更新事件（仅当 TTL <= MinReachedTTL 时发送，防止泄漏超范围数据到前端）
			hopReachedTTL := atomic.LoadInt32(&progress.MinReachedTTL)
			if opts.OnHopUpdate != nil && (hopReachedTTL <= 0 || hopResult.TTL <= int(hopReachedTTL)) {
				safeHopUpdateCallback(opts.OnHopUpdate, TracertHopUpdate{
					TTL:        hopResult.TTL,
					IP:         existing.IP,
					CurrentSeq: hopResult.SentCount,
					Success:    hopResult.Status == "success",
					RTT:        existing.LastRtt,
					IsComplete: true,
					Timestamp:  time.Now().UnixMilli(),
				})
			}
		}
	}

	// 检查是否到达目标（通过 reachedTTL 是否被更新来判断）
	finalReachedTTL := atomic.LoadInt32(&reachedTTL)
	if finalReachedTTL <= int32(maxHops) {
		progress.ReachedDest = true
		// 更新所有轮次中到达目标的最小 TTL（原子操作）
		for {
			oldMin := atomic.LoadInt32(&progress.MinReachedTTL)
			if oldMin > 0 && oldMin <= finalReachedTTL {
				// 已有更小或相等的 TTL 记录，无需更新
				break
			}
			if atomic.CompareAndSwapInt32(&progress.MinReachedTTL, oldMin, finalReachedTTL) {
				logger.Debug("Tracert", targetIP, "更新最小到达 TTL: %d（之前=%d）", finalReachedTTL, oldMin)
				break
			}
			// CAS 失败，重试
		}
	}

	// 注意：后处理清理逻辑移到 Run 函数中，在所有轮次完成后统一执行
	// 这样可以避免多轮探测时 cancelled 状态被后续轮次覆盖的问题

	progress.UpdateProgress()
	// 发送过滤后的进度到前端
	if opts.OnUpdate != nil {
		currentReachedTTL := atomic.LoadInt32(&progress.MinReachedTTL)
		filtered := progress.CloneForDisplay(currentReachedTTL)
		log.Printf("[TRACERT] runRound() OnUpdate: round=%d, reachedTTL=%d, originalHops=%d, filteredHops=%d", progress.Round, currentReachedTTL, len(progress.Hops), len(filtered.Hops))
		opts.OnUpdate(filtered)
	}

	logger.Debug("Tracert", targetIP, "所有 TTL 探测完成")
}

// mergeHopResult 合并单跳探测结果到累积结果中
func (e *TracertEngine) mergeHopResult(existing *TracertHopResult, newHop TracertHopResult) {
	// 第一轮：直接赋值
	if existing.Status == "pending" {
		*existing = newHop
		return
	}

	// 后续轮次：累积统计
	// 跳过 cancelled 状态
	if newHop.Status == "cancelled" {
		return
	}

	// 累积发送/接收计数
	// 注意：必须先保存旧的 RecvCount，再累加，否则加权平均计算会出错
	oldRecvCount := existing.RecvCount
	existing.SentCount += newHop.SentCount
	existing.RecvCount += newHop.RecvCount

	// 重新计算丢包率
	if existing.SentCount > 0 {
		existing.LossRate = float64(existing.SentCount-existing.RecvCount) / float64(existing.SentCount) * 100
	}

	// 累积 RTT 统计
	if newHop.RecvCount > 0 && newHop.AvgRtt >= 0 {
		// 更新 Min/Max
		if existing.MinRtt < 0 || (newHop.MinRtt >= 0 && newHop.MinRtt < existing.MinRtt) {
			existing.MinRtt = newHop.MinRtt
		}
		if newHop.MaxRtt > existing.MaxRtt {
			existing.MaxRtt = newHop.MaxRtt
		}
		// 加权平均：使用旧的 RecvCount 计算
		totalRtt := existing.AvgRtt*float64(oldRecvCount) + newHop.AvgRtt*float64(newHop.RecvCount)
		if existing.RecvCount > 0 {
			existing.AvgRtt = totalRtt / float64(existing.RecvCount)
		}
		existing.LastRtt = newHop.LastRtt
	}

	// 更新 IP：只有成功状态才更新 IP
	// 超时/错误状态不覆盖 IP，保留之前成功的 IP 或保持空
	if newHop.Status == "success" && newHop.IP != "" && newHop.IP != "*" {
		existing.IP = newHop.IP
	}

	// 更新状态（success 优先）
	if newHop.Status == "success" {
		existing.Status = "success"
	} else if existing.Status != "success" && newHop.Status != "" {
		existing.Status = newHop.Status
	}

	// 更新其他字段
	if newHop.Reached {
		existing.Reached = true
	}
	if newHop.ErrorMsg != "" && existing.Status != "success" {
		existing.ErrorMsg = newHop.ErrorMsg
	}
}

// probeHop 探测单跳：发送 1 个 ICMP 包并返回结果
// 注意：每个 TTL 只发送 1 个包，多轮探测由上层 runRound 控制
func (e *TracertEngine) probeHop(ctx context.Context, destIP net.IP, ttl int, opts TracertRunOptions) TracertHopResult {
	ipStr := destIP.String()
	logger.Verbose("Tracert", ipStr, "开始探测 TTL=%d", ttl)

	hop := TracertHopResult{
		TTL:    ttl,
		Status: "pending",
		MinRtt: -1,
	}

	// 检查取消
	select {
	case <-ctx.Done():
		logger.Debug("Tracert", ipStr, "TTL=%d 探测被取消", ttl)
		hop.SentCount = 0
		hop.RecvCount = 0
		hop.Status = "error"
		hop.ErrorMsg = "Cancelled"
		return hop
	default:
	}

	// 发送单个 ICMP Echo with TTL
	result, err := PingOneWithTTL(destIP, e.config.Timeout, e.config.DataSize, uint8(ttl))

	if err != nil {
		logger.Verbose("Tracert", ipStr, "TTL=%d API错误: %v", ttl, err)
		hop.SentCount = 1
		hop.RecvCount = 0
		hop.LossRate = 100
		hop.Status = "error"
		hop.ErrorMsg = err.Error()
		hop.IP = "*"

		// 发送单跳更新
		safeHopUpdateCallback(opts.OnHopUpdate, TracertHopUpdate{
			TTL:        ttl,
			IP:         "*",
			CurrentSeq: 1,
			Success:    false,
			RTT:        0,
			IsComplete: true,
			Timestamp:  time.Now().UnixMilli(),
		})
		return hop
	}

	if result == nil {
		logger.Verbose("Tracert", ipStr, "TTL=%d 返回 nil 结果", ttl)
		hop.SentCount = 1
		hop.RecvCount = 0
		hop.LossRate = 100
		hop.Status = "timeout"
		hop.ErrorMsg = "No reply"
		hop.IP = "*"

		safeHopUpdateCallback(opts.OnHopUpdate, TracertHopUpdate{
			TTL:        ttl,
			IP:         "*",
			CurrentSeq: 1,
			Success:    false,
			RTT:        0,
			IsComplete: true,
			Timestamp:  time.Now().UnixMilli(),
		})
		return hop
	}

	logger.Verbose("Tracert", ipStr, "TTL=%d 结果: success=%v, rtt=%.2fms, status=%s",
		ttl, result.Success, result.RoundTripTime, result.Status)

	hop.SentCount = 1

	if result.Success {
		// 成功到达目标
		hop.RecvCount = 1
		hop.LossRate = 0
		hop.LastRtt = result.RoundTripTime
		hop.AvgRtt = result.RoundTripTime
		hop.MinRtt = result.RoundTripTime
		hop.MaxRtt = result.RoundTripTime
		hop.IP = result.IP
		hop.Status = "success"

		// 检查是否到达目标
		if result.IP == ipStr {
			hop.Reached = true
		}

		safeHopUpdateCallback(opts.OnHopUpdate, TracertHopUpdate{
			TTL:        ttl,
			IP:         result.IP,
			CurrentSeq: 1,
			Success:    true,
			RTT:        result.RoundTripTime,
			IsComplete: true,
			Timestamp:  time.Now().UnixMilli(),
		})
	} else if result.Status == "TTLExpired" {
		// TTL Expired 是 Tracert 的正常行为，表示成功获取到中间路由器信息
		hop.RecvCount = 1
		hop.LossRate = 0
		hop.LastRtt = result.RoundTripTime
		hop.AvgRtt = result.RoundTripTime
		hop.MinRtt = result.RoundTripTime
		hop.MaxRtt = result.RoundTripTime
		hop.IP = result.IP // 使用响应 IP（中间路由器）
		hop.Status = "success"

		safeHopUpdateCallback(opts.OnHopUpdate, TracertHopUpdate{
			TTL:        ttl,
			IP:         result.IP,
			CurrentSeq: 1,
			Success:    true, // 标记为成功获取中间路由
			RTT:        result.RoundTripTime,
			IsComplete: true,
			Timestamp:  time.Now().UnixMilli(),
		})
	} else {
		// 真正的错误（超时、不可达等）
		hop.RecvCount = 0
		hop.LossRate = 100
		hop.Status = "timeout"
		if result.Error != "" {
			hop.ErrorMsg = result.Error
		} else if result.Status != "" {
			hop.ErrorMsg = result.Status
		}
		if result.IP != "" && result.IP != "*" {
			hop.IP = result.IP
		} else {
			hop.IP = "*"
		}

		safeHopUpdateCallback(opts.OnHopUpdate, TracertHopUpdate{
			TTL:        ttl,
			IP:         hop.IP,
			CurrentSeq: 1,
			Success:    false,
			RTT:        0,
			IsComplete: true,
			Timestamp:  time.Now().UnixMilli(),
		})
	}

	logger.Debug("Tracert", ipStr, "TTL=%d 探测完成: ip=%s, rtt=%.2fms, reached=%v",
		ttl, hop.IP, hop.AvgRtt, hop.Reached)

	return hop
}

// resolveTarget 解析目标（域名→IP）
func (e *TracertEngine) resolveTarget(target string) (string, error) {
	// 先尝试直接解析为 IP
	ip := net.ParseIP(target)
	if ip != nil {
		ip4 := ip.To4()
		if ip4 != nil {
			return ip4.String(), nil
		}
		return "", fmt.Errorf("仅支持 IPv4 地址: %s", target)
	}

	// DNS 解析
	logger.Debug("Tracert", target, "开始 DNS 解析")
	ips, err := net.LookupIP(target)
	if err != nil {
		return "", fmt.Errorf("DNS 解析失败 '%s': %w", target, err)
	}

	for _, ip := range ips {
		if ip4 := ip.To4(); ip4 != nil {
			logger.Info("Tracert", target, "DNS 解析成功: %s -> %s", target, ip4.String())
			return ip4.String(), nil
		}
	}

	return "", fmt.Errorf("未找到 IPv4 地址: %s", target)
}

// Stop 停止正在运行的 tracert 探测
func (e *TracertEngine) Stop() {
	e.runningMu.Lock()
	cancel := e.cancel
	running := e.running
	e.runningMu.Unlock()

	if cancel != nil {
		logger.Info("Tracert", "-", "正在停止路径探测...")
		cancel()
	} else {
		logger.Debug("Tracert", "-", "停止请求已收到，但没有活动的取消函数 (running=%v)", running)
	}
}

// IsRunning 返回引擎是否正在运行
func (e *TracertEngine) IsRunning() bool {
	e.runningMu.RLock()
	defer e.runningMu.RUnlock()
	return e.running
}

// GetConfig 返回当前配置
func (e *TracertEngine) GetConfig() TracertConfig {
	return e.config
}

// safeTracertCallback 安全调用进度回调（带 panic 恢复）
func safeTracertCallback(fn func(*TracertProgress), progress *TracertProgress) {
	if fn == nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			logger.Error("Tracert", "-", "进度回调 panic: %v", r)
		}
	}()
	fn(progress)
}

// safeHopUpdateCallback 安全调用单跳更新回调（带 panic 恢复）
func safeHopUpdateCallback(fn func(TracertHopUpdate), update TracertHopUpdate) {
	if fn == nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			logger.Error("Tracert", "-", "单跳更新回调 panic: %v", r)
		}
	}()
	fn(update)
}
