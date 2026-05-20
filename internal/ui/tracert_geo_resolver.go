//go:build windows

package ui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/NetWeaverGo/core/internal/icmp"
	"github.com/NetWeaverGo/core/internal/logger"
)

// ---------------------------------------------------------------------------
// 包级全局变量：额外保留网段
// ---------------------------------------------------------------------------

// extraReservedNets 存储除 IsPrivate/IsLoopback 之外的保留IP网段
var extraReservedNets []*net.IPNet

func init() {
	// 100.64.0.0/10 CGNAT (运营商级NAT)
	_, cgnat, _ := net.ParseCIDR("100.64.0.0/10")
	// 169.254.0.0/16 链路本地
	_, linkLocal, _ := net.ParseCIDR("169.254.0.0/16")
	extraReservedNets = []*net.IPNet{cgnat, linkLocal}
}

// ---------------------------------------------------------------------------
// 失败缓存TTL配置
// ---------------------------------------------------------------------------

// failCacheTTL 定义各类失败缓存的过期时间
var failCacheTTL = map[string]time.Duration{
	"rate_limit": 60 * time.Second,
	"network":    60 * time.Second,
	"api_fail":   120 * time.Second,
	"private_ip": 0, // 永不过期
}

// ---------------------------------------------------------------------------
// 数据结构
// ---------------------------------------------------------------------------

// GeoResolveResult 地理位置查询结果（含缓存元数据）
type GeoResolveResult struct {
	GeoInfo      *icmp.GeoInfo // 查询结果（nil表示失败）
	CachedAt     time.Time     // 缓存写入时间
	FailType     string        // 失败类型: ""/"network"/"rate_limit"/"api_fail"/"private_ip"
	LastFailTime time.Time     // 最近失败时间
}

// isFailCacheExpired 判断失败缓存是否已过期
func (r *GeoResolveResult) isFailCacheExpired() bool {
	if r.FailType == "" {
		return false // 成功缓存不过期（由清理goroutine管理）
	}
	if r.FailType == "private_ip" {
		return false // 永不过期
	}
	ttl, ok := failCacheTTL[r.FailType]
	if !ok {
		ttl = 60 * time.Second // 默认60秒
	}
	return time.Since(r.LastFailTime) > ttl
}

// TracertGeoResolvedEvent 推送到前端的geo解析完成事件
type TracertGeoResolvedEvent struct {
	SessionID string        `json:"sessionId"`
	IP        string        `json:"ip"`
	Geo       *icmp.GeoInfo `json:"geo"`
}

// ---------------------------------------------------------------------------
// 错误类型
// ---------------------------------------------------------------------------

// rateLimitError 表示API返回429限频
var rateLimitError = errors.New("ip-api rate limit exceeded (429)")

// ---------------------------------------------------------------------------
// TracertGeoResolver 结构体
// ---------------------------------------------------------------------------

// TracertGeoResolver 管理IP地理位置的异步查询、缓存和事件推送。
// 采用进程级单例模式，通过 GetGlobalGeoResolver() 获取实例。
type TracertGeoResolver struct {
	mu          sync.RWMutex                   // 保护 cache、pending、sessionID
	cache       map[string]*GeoResolveResult   // IP → 查询结果缓存
	pending     map[string]struct{}            // 正在查询的IP集合
	sessionID   string                         // 当前会话ID
	httpClient  *http.Client                   // HTTP客户端（超时5秒）
	eventBridge func(string, any)              // Wails事件推送函数
	limiter     *rate.Limiter                  // 全局请求频率限制器（每4秒1次）
	cancelFunc  context.CancelFunc             // 用于取消所有进行中的请求
	stopCleanup context.CancelFunc             // 用于停止缓存清理goroutine
}

// ---------------------------------------------------------------------------
// 全局单例
// ---------------------------------------------------------------------------

var (
	globalGeoResolver     *TracertGeoResolver
	globalGeoResolverOnce sync.Once
)

// GetGlobalGeoResolver 返回全局 TracertGeoResolver 单例。
// 首次调用时初始化：创建HTTP客户端、限流器，并启动缓存清理goroutine。
//
// Parameters:
//   - eventBridge: Wails事件推送函数，用于向前端发送 tracert:geo-resolved 事件
func GetGlobalGeoResolver(eventBridge func(string, any)) *TracertGeoResolver {
	globalGeoResolverOnce.Do(func() {
		_, cancel := context.WithCancel(context.Background())

		resolver := &TracertGeoResolver{
			cache:       make(map[string]*GeoResolveResult),
			pending:     make(map[string]struct{}),
			httpClient:  &http.Client{Timeout: 5 * time.Second},
			eventBridge: eventBridge,
			limiter:     rate.NewLimiter(rate.Every(4*time.Second), 1),
			cancelFunc:  cancel,
		}

		resolver.startCacheCleanup()

		globalGeoResolver = resolver
	})
	return globalGeoResolver
}

// ---------------------------------------------------------------------------
// 公共方法
// ---------------------------------------------------------------------------

// SetSessionID 设置当前会话ID，用于事件推送时标识会话。
func (r *TracertGeoResolver) SetSessionID(sessionID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessionID = sessionID
}

// GetSessionID 获取当前会话ID。
func (r *TracertGeoResolver) GetSessionID() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.sessionID
}

// ResolveAsync 异步查询IP地理位置。
// 仅对未缓存且未在查询中的新IP发起查询，每个IP启动独立goroutine。
// 查询完成后通过 eventBridge 推送 tracert:geo-resolved 事件。
//
// Parameters:
//   - ctx: 上下文，用于取消查询
//   - ips: 需要查询的IP地址列表
func (r *TracertGeoResolver) ResolveAsync(ctx context.Context, ips []string) {
	newIPs := r.collectNewIPs(ctx, ips)
	if len(newIPs) == 0 {
		return
	}

	// 标记新IP为pending
	r.mu.Lock()
	for _, ip := range newIPs {
		r.pending[ip] = struct{}{}
	}
	r.mu.Unlock()

	// 为每个IP启动独立goroutine
	for _, ip := range newIPs {
		go r.resolveSingleIP(ctx, ip)
	}
}

// GetCachedResult 从缓存获取结果（线程安全）。
//
// Parameters:
//   - ip: 要查询的IP地址
//
// Returns:
//   - 缓存结果，如果未找到返回nil
func (r *TracertGeoResolver) GetCachedResult(ip string) *GeoResolveResult {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result, exists := r.cache[ip]
	if !exists {
		return nil
	}

	// 返回副本防止外部修改
	resultCopy := *result
	return &resultCopy
}

// ClearPending 清除所有pending状态（StopTracert时调用，不清除缓存）。
func (r *TracertGeoResolver) ClearPending() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pending = make(map[string]struct{})
}

// HasPendingIPs 检查是否有正在进行Geo查询的IP。
func (r *TracertGeoResolver) HasPendingIPs() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.pending) > 0
}

// CancelAll 取消所有进行中的请求。
func (r *TracertGeoResolver) CancelAll() {
	if r.cancelFunc != nil {
		r.cancelFunc()
	}
}

// ---------------------------------------------------------------------------
// 核心查询方法
// ---------------------------------------------------------------------------

// resolveSingleIP 对单个IP执行地理位置查询。
// 1. 等待限流器许可
// 2. 执行HTTP请求
// 3. 更新缓存
// 4. 从pending集合移除
// 5. 推送事件
func (r *TracertGeoResolver) resolveSingleIP(ctx context.Context, ip string) {
	// 等待限流器许可
	if err := r.limiter.Wait(ctx); err != nil {
		// context取消，不写入缓存，直接返回
		r.mu.Lock()
		delete(r.pending, ip)
		r.mu.Unlock()
		return
	}

	// 执行查询
	geoInfo, failType := r.queryGeoInfo(ctx, ip)

	// 检查context是否已取消
	select {
	case <-ctx.Done():
		// context取消，不写入缓存
		r.mu.Lock()
		delete(r.pending, ip)
		r.mu.Unlock()
		return
	default:
	}

	// 更新缓存
	now := time.Now()
	result := &GeoResolveResult{
		CachedAt: now,
		FailType: failType,
	}
	if geoInfo != nil {
		result.GeoInfo = geoInfo
	} else {
		result.LastFailTime = now
	}

	r.mu.Lock()
	r.cache[ip] = result
	delete(r.pending, ip)
	sessionID := r.sessionID
	r.mu.Unlock()

	// 推送事件
	if r.eventBridge != nil {
		r.eventBridge("tracert:geo-resolved", TracertGeoResolvedEvent{
			SessionID: sessionID,
			IP:        ip,
			Geo:       geoInfo,
		})
	}
}

// queryGeoInfo 执行HTTP请求查询IP地理位置。
// 最多重试1次（仅网络错误时重试，429不重试）。
//
// Parameters:
//   - ctx: 上下文
//   - ip: 要查询的IP地址
//
// Returns:
//   - geoInfo: 查询结果（失败时为nil）
//   - failType: 失败类型（成功时为""）
func (r *TracertGeoResolver) queryGeoInfo(ctx context.Context, ip string) (*icmp.GeoInfo, string) {
	// 检查保留IP
	if isReservedIP(ip) {
		logger.Debug("TracertGeoResolver", ip, "保留IP，跳过查询")
		return nil, "private_ip"
	}

	url := "http://ip-api.com/json/" + ip + "?lang=zh-CN"

	// 首次请求
	geoInfo, failType, err := r.doQueryAttempt(ctx, url)
	if err == nil {
		return geoInfo, ""
	}

	// 429不重试
	if errors.Is(err, rateLimitError) {
		logger.Debug("TracertGeoResolver", ip, "API限频(429)，不重试")
		return nil, "rate_limit"
	}

	// context取消不重试
	if ctx.Err() != nil {
		return nil, ""
	}

	// 网络错误：重试1次
	logger.Debug("TracertGeoResolver", ip, "首次请求失败，重试: %s", err.Error())
	geoInfo, failType, err = r.doQueryAttempt(ctx, url)
	if err == nil {
		return geoInfo, ""
	}

	if errors.Is(err, rateLimitError) {
		return nil, "rate_limit"
	}

	if ctx.Err() != nil {
		return nil, ""
	}

	// 重试仍然失败
	if failType != "" {
		return nil, failType
	}
	return nil, "network"
}

// doQueryAttempt 执行单次HTTP查询尝试。
//
// Returns:
//   - geoInfo: 解析结果
//   - failType: 失败类型
//   - err: 错误（rateLimitError 或网络错误）
func (r *TracertGeoResolver) doQueryAttempt(ctx context.Context, url string) (*icmp.GeoInfo, string, error) {
	resp, err := r.doSingleRequest(ctx, url)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("读取响应体失败: %w", err)
	}

	// 解析JSON
	var geoInfo icmp.GeoInfo
	if err := json.Unmarshal(body, &geoInfo); err != nil {
		return nil, "api_fail", fmt.Errorf("解析JSON失败: %w", err)
	}

	// 检查status字段
	if geoInfo.Status == "success" {
		return &geoInfo, "", nil
	}

	if geoInfo.Status == "fail" {
		msg := geoInfo.Message
		if msg == "" {
			msg = "unknown error"
		}
		return nil, "api_fail", fmt.Errorf("API返回fail: %s", msg)
	}

	return nil, "api_fail", fmt.Errorf("未知的API状态: %s", geoInfo.Status)
}

// doSingleRequest 执行单次HTTP GET请求。
//
// Parameters:
//   - ctx: 上下文
//   - url: 请求URL
//
// Returns:
//   - 200: 返回响应
//   - 429: 返回 rateLimitError
//   - 其他: 返回错误
func (r *TracertGeoResolver) doSingleRequest(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP请求失败: %w", err)
	}

	if resp.StatusCode == http.StatusOK {
		return resp, nil
	}

	// 非200状态码，立即关闭响应体
	resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, rateLimitError
	}

	return nil, fmt.Errorf("HTTP状态码: %d", resp.StatusCode)
}

// ---------------------------------------------------------------------------
// IP过滤方法
// ---------------------------------------------------------------------------

// collectNewIPs 过滤出需要查询的新IP列表。
// 跳过：空IP、"*"、已缓存（含未过期的失败缓存）、pending中、保留IP。
func (r *TracertGeoResolver) collectNewIPs(_ context.Context, ips []string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var newIPs []string
	for _, ip := range ips {
		// 跳过空IP和"*"
		if ip == "" || ip == "*" {
			continue
		}

		// 检查缓存
		if cached, exists := r.cache[ip]; exists {
			// 成功缓存 → 跳过
			if cached.FailType == "" {
				continue
			}
			// 失败缓存未过期 → 跳过
			if !cached.isFailCacheExpired() {
				continue
			}
			// 失败缓存已过期 → 需要重新查询
		}

		// 跳过正在pending中的IP
		if _, isPending := r.pending[ip]; isPending {
			continue
		}

		// 跳过保留IP（但仍允许进入缓存，由queryGeoInfo处理）
		// 注意：这里不跳过保留IP，让resolveSingleIP中的queryGeoInfo来处理
		// 这样保留IP也会被缓存，避免重复检查

		newIPs = append(newIPs, ip)
	}

	return newIPs
}

// isReservedIP 检查IP是否为保留地址（私有、回环、CGNAT、链路本地等）。
// 无效IP也视为保留。
func isReservedIP(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return true // 无效IP视为保留
	}
	if ip.IsPrivate() || ip.IsLoopback() {
		return true
	}
	for _, network := range extraReservedNets {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// 缓存管理
// ---------------------------------------------------------------------------

// startCacheCleanup 启动缓存清理goroutine。
// 每5分钟清理超过30分钟的成功缓存条目。
func (r *TracertGeoResolver) startCacheCleanup() {
	ctx, cancel := context.WithCancel(context.Background())
	r.stopCleanup = cancel

	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				r.cleanupExpiredCache()
			}
		}
	}()
}

// stopCacheCleanup 停止缓存清理goroutine。
func (r *TracertGeoResolver) stopCacheCleanup() {
	if r.stopCleanup != nil {
		r.stopCleanup()
	}
}

// cleanupExpiredCache 清理过期的缓存条目。
// 成功缓存超过30分钟则删除；失败缓存由 isFailCacheExpired 判断。
func (r *TracertGeoResolver) cleanupExpiredCache() {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	for ip, result := range r.cache {
		if result.FailType == "" {
			// 成功缓存：超过30分钟删除
			if now.Sub(result.CachedAt) > 30*time.Minute {
				delete(r.cache, ip)
			}
		} else {
			// 失败缓存：按TTL判断
			if result.isFailCacheExpired() {
				delete(r.cache, ip)
			}
		}
	}
}
