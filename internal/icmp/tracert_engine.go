package icmp

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/NetWeaverGo/core/internal/logger"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

// TracertEngine tracert 路径探测引擎
type TracertEngine struct {
	config    TracertConfig
	cancel    context.CancelFunc
	running   bool
	runningMu sync.RWMutex

	sockets   []*icmp.PacketConn
	socketsMu sync.RWMutex
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
	// 【心跳更新】在超时等待期间定期推送 elapsedMs，保持前端活跃
	heartbeatTicker := time.NewTicker(500 * time.Millisecond)
	defer heartbeatTicker.Stop()

collectLoop:
	for {
		select {
		case hopResult, ok := <-resultChan:
			if !ok {
				break collectLoop
			}
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

		case <-heartbeatTicker.C:
			// 心跳更新：在超时等待期间定期推送 elapsedMs，让前端知道探测仍在活跃运行
			progress.ElapsedMs = time.Since(progress.StartTime).Milliseconds()
			if opts.OnUpdate != nil {
				currentReachedTTL := atomic.LoadInt32(&progress.MinReachedTTL)
				opts.OnUpdate(progress.CloneForDisplay(currentReachedTTL))
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

	e.socketsMu.RLock()
	if e.sockets == nil || ttl < 1 || ttl > len(e.sockets) {
		e.socketsMu.RUnlock()
		hop.Status = "error"
		hop.ErrorMsg = "Socket未初始化"
		return hop
	}
	conn := e.sockets[ttl-1]
	e.socketsMu.RUnlock()

	seq := nextSeq()
	id := icmpID()

	msg := icmp.Message{
		Type: ipv4.ICMPTypeEcho,
		Code: 0,
		Body: &icmp.Echo{
			ID:   id,
			Seq:  seq,
			Data: prepareSendDataRaw(e.config.DataSize),
		},
	}
	wb, err := msg.Marshal(nil)
	if err != nil {
		hop.Status = "error"
		hop.ErrorMsg = err.Error()
		return hop
	}

	dst := &net.IPAddr{IP: destIP}
	sendTime := time.Now()

	if _, err := conn.WriteTo(wb, dst); err != nil {
		if e.isSocketInvalidError(err) {
			logger.Warn("Tracert", ipStr, "TTL=%d Socket 可能失效，尝试重建", ttl)
			e.rebuildSocket(ttl)
		}
		hop.SentCount = 1
		hop.LossRate = 100
		hop.Status = "error"
		hop.ErrorMsg = err.Error()
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

	deadline := sendTime.Add(time.Duration(e.config.Timeout) * time.Millisecond)
	conn.SetReadDeadline(deadline)

	rb := make([]byte, maxMessageSize)
	for {
		n, peer, err := conn.ReadFrom(rb)
		if err != nil {
			hop.SentCount = 1
			hop.RecvCount = 0
			hop.LossRate = 100
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				hop.Status = "timeout"
				hop.ErrorMsg = "Request Timed Out"
			} else {
				hop.Status = "error"
				hop.ErrorMsg = err.Error()
			}
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

		rm, err := icmp.ParseMessage(protocolICMP, rb[:n])
		if err != nil {
			continue
		}

		switch rm.Type {
		case ipv4.ICMPTypeEchoReply:
			reply, ok := rm.Body.(*icmp.Echo)
			if !ok || reply.ID != id || reply.Seq != seq {
				continue
			}
			rtt := time.Since(sendTime).Milliseconds()
			replyIP := extractIP(peer)

			hop.SentCount = 1
			hop.RecvCount = 1
			hop.LossRate = 0
			hop.LastRtt = float64(rtt)
			hop.AvgRtt = float64(rtt)
			hop.MinRtt = float64(rtt)
			hop.MaxRtt = float64(rtt)
			hop.IP = replyIP
			hop.Status = "success"
			if replyIP == ipStr {
				hop.Reached = true
			}

			safeHopUpdateCallback(opts.OnHopUpdate, TracertHopUpdate{
				TTL:        ttl,
				IP:         replyIP,
				CurrentSeq: 1,
				Success:    true,
				RTT:        float64(rtt),
				IsComplete: true,
				Timestamp:  time.Now().UnixMilli(),
			})
			return hop

		case ipv4.ICMPTypeTimeExceeded:
			if !matchTimeExceeded(rm, id, seq) {
				continue
			}
			rtt := time.Since(sendTime).Milliseconds()
			replyIP := extractIP(peer)

			hop.SentCount = 1
			hop.RecvCount = 1
			hop.LossRate = 0
			hop.LastRtt = float64(rtt)
			hop.AvgRtt = float64(rtt)
			hop.MinRtt = float64(rtt)
			hop.MaxRtt = float64(rtt)
			hop.IP = replyIP
			hop.Status = "success"

			safeHopUpdateCallback(opts.OnHopUpdate, TracertHopUpdate{
				TTL:        ttl,
				IP:         replyIP,
				CurrentSeq: 1,
				Success:    true, // 标记为成功获取中间路由
				RTT:        float64(rtt),
				IsComplete: true,
				Timestamp:  time.Now().UnixMilli(),
			})
			return hop

		case ipv4.ICMPTypeDestinationUnreachable:
			if !matchDestUnreachable(rm, id, seq) {
				continue
			}
			hop.SentCount = 1
			hop.RecvCount = 0
			hop.LossRate = 100
			hop.Status = "error"
			hop.ErrorMsg = "Destination Host Unreachable"
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
	}
}

// ResolveTarget 解析目标（域名→IP）
func (e *TracertEngine) ResolveTarget(ctx context.Context, target string) (string, error) {
	return e.resolveTarget(ctx, target)
}

// resolveTarget 解析目标（域名→IP），带超时控制和Context级联取消
// P1-优化修复：增加ctx参数，继承父context实现级联取消
func (e *TracertEngine) resolveTarget(ctx context.Context, target string) (string, error) {
	// 先尝试直接解析为 IP
	ip := net.ParseIP(target)
	if ip != nil {
		ip4 := ip.To4()
		if ip4 != nil {
			return ip4.String(), nil
		}
		return "", fmt.Errorf("仅支持 IPv4 地址: %s", target)
	}

	// DNS 解析（带3秒超时 + 继承父context）
	logger.Debug("Tracert", target, "开始 DNS 解析")
	// P1-优化修复：使用传入的ctx而非Background()，实现级联取消
	resolveCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	resolver := &net.Resolver{}
	ips, err := resolver.LookupIPAddr(resolveCtx, target)
	if err != nil {
		// 区分超时错误、取消错误和其他DNS错误
		if errors.Is(err, context.DeadlineExceeded) {
			return "", fmt.Errorf("DNS 解析超时 '%s'（3秒）", target)
		}
		if errors.Is(err, context.Canceled) {
			return "", fmt.Errorf("DNS 解析被取消 '%s'", target)
		}
		return "", fmt.Errorf("DNS 解析失败 '%s': %w", target, err)
	}

	for _, ipAddr := range ips {
		if ip4 := ipAddr.IP.To4(); ip4 != nil {
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

// InitSockets 初始化专属 Socket 组
func (e *TracertEngine) InitSockets() error {
	e.socketsMu.Lock()
	defer e.socketsMu.Unlock()

	if e.sockets != nil {
		return nil
	}

	maxHops := e.config.MaxHops
	e.sockets = make([]*icmp.PacketConn, maxHops)

	for ttl := 1; ttl <= maxHops; ttl++ {
		conn, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
		if err != nil {
			for i := 0; i < ttl-1; i++ {
				if e.sockets[i] != nil {
					e.sockets[i].Close()
				}
			}
			e.sockets = nil
			return fmt.Errorf("创建 TTL=%d 的 Socket 失败: %w", ttl, err)
		}

		pconn := conn.IPv4PacketConn()
		if err := pconn.SetTTL(ttl); err != nil {
			conn.Close()
			for i := 0; i < ttl-1; i++ {
				if e.sockets[i] != nil {
					e.sockets[i].Close()
				}
			}
			e.sockets = nil
			return fmt.Errorf("设置 TTL=%d 失败: %w", ttl, err)
		}

		e.sockets[ttl-1] = conn
	}

	logger.Info("Tracert", "-", "专属 Socket 组初始化完成: %d 个 Socket", maxHops)
	return nil
}

// Close 销毁专属 Socket 组并清理资源
func (e *TracertEngine) Close() {
	e.socketsMu.Lock()
	defer e.socketsMu.Unlock()

	if e.sockets == nil {
		return
	}

	closedCount := 0
	for i, conn := range e.sockets {
		if conn != nil {
			conn.Close()
			e.sockets[i] = nil
			closedCount++
		}
	}
	e.sockets = nil

	logger.Info("Tracert", "-", "专属 Socket 组已关闭: %d 个 Socket", closedCount)
}

// RunRound 执行单轮 tracert 探测
func (e *TracertEngine) RunRound(ctx context.Context, target string, resolvedIP string, opts TracertRunOptions) *TracertProgress {
	if !e.socketsInitialized() {
		progress := NewTracertProgress(target, e.config.MaxHops)
		progress.IsRunning = false
		return progress
	}

	progress := NewTracertProgress(target, e.config.MaxHops)
	progress.ResolvedIP = resolvedIP

	e.runRound(ctx, resolvedIP, progress, opts)

	minReachedTTL := atomic.LoadInt32(&progress.MinReachedTTL)
	if minReachedTTL > 0 && minReachedTTL <= int32(e.config.MaxHops) {
		for i := int(minReachedTTL); i < len(progress.Hops); i++ {
			hop := &progress.Hops[i]
			if hop.Reached {
				continue
			}
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

func (e *TracertEngine) socketsInitialized() bool {
	e.socketsMu.RLock()
	defer e.socketsMu.RUnlock()
	return e.sockets != nil
}

func (e *TracertEngine) drainSocket(conn *icmp.PacketConn) {
	conn.SetReadDeadline(time.Now().Add(1 * time.Millisecond))
	buf := make([]byte, maxMessageSize)
	for {
		_, _, err := conn.ReadFrom(buf)
		if err != nil {
			break
		}
	}
	conn.SetReadDeadline(time.Time{})
}

func (e *TracertEngine) DrainAllSockets() {
	e.socketsMu.RLock()
	defer e.socketsMu.RUnlock()

	if e.sockets == nil {
		return
	}
	for _, conn := range e.sockets {
		if conn != nil {
			e.drainSocket(conn)
		}
	}
}

func (e *TracertEngine) isSocketInvalidError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "use of closed") ||
		strings.Contains(errStr, "bad file descriptor") ||
		strings.Contains(errStr, "not a socket") ||
		strings.Contains(errStr, "WSAENOTSOCK") ||
		strings.Contains(errStr, "WSAECANCELLED")
}

func (e *TracertEngine) rebuildSocket(ttl int) error {
	e.socketsMu.Lock()
	defer e.socketsMu.Unlock()

	if e.sockets == nil || ttl < 1 || ttl > len(e.sockets) {
		return fmt.Errorf("无法重建 Socket: Socket 组未初始化或 TTL 越界")
	}

	if e.sockets[ttl-1] != nil {
		e.sockets[ttl-1].Close()
	}

	conn, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		return fmt.Errorf("重建 TTL=%d Socket 失败: %w", ttl, err)
	}

	pconn := conn.IPv4PacketConn()
	if err := pconn.SetTTL(ttl); err != nil {
		conn.Close()
		return fmt.Errorf("重建 TTL=%d 设置失败: %w", ttl, err)
	}

	e.sockets[ttl-1] = conn
	logger.Info("Tracert", "-", "Socket 重建成功: TTL=%d", ttl)
	return nil
}
