//go:build windows

package ui

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/time/rate"

	"github.com/NetWeaverGo/core/internal/icmp"
	"github.com/NetWeaverGo/core/internal/logger"
)

// ---------------------------------------------------------------------------
// 测试辅助函数
// ---------------------------------------------------------------------------

// mockEventBridge 创建一个模拟的事件桥接函数，用于捕获推送的事件
func mockEventBridge() (func(string, any), *[]TracertGeoResolvedEvent) {
	var events []TracertGeoResolvedEvent
	var mu sync.Mutex
	bridge := func(event string, data any) {
		if event == "tracert:geo-resolved" {
			mu.Lock()
			events = append(events, data.(TracertGeoResolvedEvent))
			mu.Unlock()
		}
	}
	return bridge, &events
}

// testTransport 自定义 Transport 用于将请求重定向到测试服务器
type testTransport struct {
	serverURL string
	original  http.RoundTripper
}

func (t *testTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// 替换 URL 的 Host 为测试服务器
	req.URL.Scheme = "http"
	req.URL.Host = strings.TrimPrefix(t.serverURL, "http://")
	return t.original.RoundTrip(req)
}

// newTestResolver 创建一个用于测试的 TracertGeoResolver 实例
func newTestResolver(serverURL string) *TracertGeoResolver {
	// 重置全局单例
	globalGeoResolver = nil
	globalGeoResolverOnce = sync.Once{}

	bridge, _ := mockEventBridge()

	// 创建自定义 HTTP Client，使用测试 Transport
	httpClient := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &testTransport{
			serverURL: serverURL,
			original:  http.DefaultTransport,
		},
	}

	resolver := &TracertGeoResolver{
		cache:       make(map[string]*GeoResolveResult),
		pending:     make(map[string]struct{}),
		httpClient:  httpClient,
		eventBridge: bridge,
		limiter:     rate.NewLimiter(rate.Inf, 1), // 不限流
		cancelFunc:  func() {},
	}

	return resolver
}

// ---------------------------------------------------------------------------
// 辅助函数测试
// ---------------------------------------------------------------------------

func TestFormatIPListForLog(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		ips      []string
		expected string
	}{
		{
			name:     "空列表",
			ips:      []string{},
			expected: "[]",
		},
		{
			name:     "少于10个IP",
			ips:      []string{"1.1.1.1", "8.8.8.8", "9.9.9.9"},
			expected: "[1.1.1.1 8.8.8.8 9.9.9.9]",
		},
		{
			name:     "正好10个IP",
			ips:      []string{"1.1.1.1", "2.2.2.2", "3.3.3.3", "4.4.4.4", "5.5.5.5", "6.6.6.6", "7.7.7.7", "8.8.8.8", "9.9.9.9", "10.10.10.10"},
			expected: "[1.1.1.1 2.2.2.2 3.3.3.3 4.4.4.4 5.5.5.5 6.6.6.6 7.7.7.7 8.8.8.8 9.9.9.9 10.10.10.10]",
		},
		{
			name:     "超过10个IP",
			ips:      generateIPs(15),
			expected: "[ip-0 ip-1 ip-2 ip-3 ip-4 ip-5 ip-6 ip-7 ip-8 ip-9] ... (共 15 个，已截断)",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := formatIPListForLog(tt.ips)
			if result != tt.expected {
				t.Errorf("formatIPListForLog() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestTruncateBody(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		body     string
		maxLen   int
		expected string
	}{
		{
			name:     "短内容不截断",
			body:     "short",
			maxLen:   100,
			expected: "short",
		},
		{
			name:     "正好达到限制",
			body:     "12345",
			maxLen:   5,
			expected: "12345",
		},
		{
			name:     "超过限制需要截断",
			body:     "this is a long content that needs to be truncated",
			maxLen:   10,
			expected: "this is a ...",
		},
		{
			name:     "空内容",
			body:     "",
			maxLen:   10,
			expected: "",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := truncateBody([]byte(tt.body), tt.maxLen)
			if result != tt.expected {
				t.Errorf("truncateBody() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 成功解析场景测试
// ---------------------------------------------------------------------------

func TestTracertGeoResolver_SuccessResolve(t *testing.T) {
	// 开启 Debug 和 Verbose 日志
	logger.EnableDebug = true
	logger.EnableVerbose = true
	defer func() {
		logger.EnableDebug = false
		logger.EnableVerbose = false
	}()

	// 创建模拟服务器返回有效 JSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"success","country":"中国","countryCode":"CN","region":"北京","regionName":"北京市","city":"北京","isp":"ChinaNet","ip":"8.8.8.8"}`))
	}))
	defer server.Close()

	// 创建测试 resolver
	resolver := newTestResolver(server.URL)

	// 执行异步解析
	ctx := context.Background()
	resolver.ResolveAsync(ctx, []string{"8.8.8.8"})

	// 等待解析完成
	time.Sleep(200 * time.Millisecond)

	// 验证缓存结果
	result := resolver.GetCachedResult("8.8.8.8")
	if result == nil {
		t.Fatal("期望缓存结果不为 nil")
	}

	if result.GeoInfo == nil {
		t.Fatal("期望 GeoInfo 不为 nil")
	}

	if result.GeoInfo.Country != "中国" {
		t.Errorf("期望 Country='中国', got '%s'", result.GeoInfo.Country)
	}

	if result.FailType != "" {
		t.Errorf("期望 FailType='', got '%s'", result.FailType)
	}
}

// ---------------------------------------------------------------------------
// API 限频(429)场景测试
// ---------------------------------------------------------------------------

func TestTracertGeoResolver_RateLimit429(t *testing.T) {
	// 开启 Debug 日志
	logger.EnableDebug = true
	defer func() {
		logger.EnableDebug = false
	}()

	// 创建模拟服务器返回 429
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	resolver := newTestResolver(server.URL)

	ctx := context.Background()
	resolver.ResolveAsync(ctx, []string{"1.2.3.4"})

	// 等待解析完成
	time.Sleep(200 * time.Millisecond)

	// 验证缓存结果
	result := resolver.GetCachedResult("1.2.3.4")
	if result == nil {
		t.Fatal("期望缓存结果不为 nil")
	}

	if result.FailType != "rate_limit" {
		t.Errorf("期望 FailType='rate_limit', got '%s'", result.FailType)
	}

	if result.GeoInfo != nil {
		t.Error("期望 GeoInfo 为 nil")
	}
}

// ---------------------------------------------------------------------------
// 网络错误场景测试
// ---------------------------------------------------------------------------

func TestTracertGeoResolver_NetworkError(t *testing.T) {
	// 开启 Debug 日志
	logger.EnableDebug = true
	defer func() {
		logger.EnableDebug = false
	}()

	// 创建一个立即关闭连接的服务器模拟网络错误
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 使用 Hijacker 关闭连接
		if hijacker, ok := w.(http.Hijacker); ok {
			conn, _, _ := hijacker.Hijack()
			conn.Close()
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	resolver := newTestResolver(server.URL)

	ctx := context.Background()
	resolver.ResolveAsync(ctx, []string{"5.6.7.8"})

	// 等待解析完成（包括重试）
	time.Sleep(300 * time.Millisecond)

	// 验证缓存结果
	result := resolver.GetCachedResult("5.6.7.8")
	if result == nil {
		t.Fatal("期望缓存结果不为 nil")
	}

	if result.FailType != "network" {
		t.Errorf("期望 FailType='network', got '%s'", result.FailType)
	}

	if result.GeoInfo != nil {
		t.Error("期望 GeoInfo 为 nil")
	}
}

// ---------------------------------------------------------------------------
// 非 JSON 响应场景测试
// ---------------------------------------------------------------------------

func TestTracertGeoResolver_NonJSONResponse(t *testing.T) {
	// 开启 Verbose 日志
	logger.EnableVerbose = true
	defer func() {
		logger.EnableVerbose = false
	}()

	// 创建模拟服务器返回 HTML
	htmlContent := `<html><head><title>Error</title></head><body><h1>404 Not Found</h1></body></html>`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(htmlContent))
	}))
	defer server.Close()

	resolver := newTestResolver(server.URL)

	ctx := context.Background()
	resolver.ResolveAsync(ctx, []string{"9.10.11.12"})

	// 等待解析完成
	time.Sleep(200 * time.Millisecond)

	// 验证缓存结果
	result := resolver.GetCachedResult("9.10.11.12")
	if result == nil {
		t.Fatal("期望缓存结果不为 nil")
	}

	if result.FailType != "api_fail" {
		t.Errorf("期望 FailType='api_fail', got '%s'", result.FailType)
	}

	if result.GeoInfo != nil {
		t.Error("期望 GeoInfo 为 nil")
	}
}

// ---------------------------------------------------------------------------
// 缓存命中跳过场景测试
// ---------------------------------------------------------------------------

func TestTracertGeoResolver_CacheHitSkip(t *testing.T) {
	// 开启 Verbose 日志
	logger.EnableVerbose = true
	defer func() {
		logger.EnableVerbose = false
	}()

	// 创建模拟服务器，记录请求次数
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"success","country":"中国","ip":"8.8.8.8"}`))
	}))
	defer server.Close()

	resolver := newTestResolver(server.URL)

	// 预先缓存一个成功结果
	resolver.mu.Lock()
	resolver.cache["8.8.8.8"] = &GeoResolveResult{
		GeoInfo:  &icmp.GeoInfo{Country: "中国", Status: "success"},
		CachedAt: time.Now(),
	}
	resolver.mu.Unlock()

	ctx := context.Background()
	// 尝试解析已缓存的 IP
	resolver.ResolveAsync(ctx, []string{"8.8.8.8"})

	// 等待一下
	time.Sleep(100 * time.Millisecond)

	// 验证服务器没有被调用
	if requestCount != 0 {
		t.Errorf("期望服务器不被调用，但调用了 %d 次", requestCount)
	}

	// 验证缓存结果仍然存在
	result := resolver.GetCachedResult("8.8.8.8")
	if result == nil {
		t.Fatal("期望缓存结果不为 nil")
	}
}

// ---------------------------------------------------------------------------
// pending 跳过场景测试
// ---------------------------------------------------------------------------

func TestTracertGeoResolver_PendingSkip(t *testing.T) {
	// 开启 Verbose 日志
	logger.EnableVerbose = true
	defer func() {
		logger.EnableVerbose = false
	}()

	// 创建一个延迟响应的服务器
	var requestCount int
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()
		// 延迟响应
		time.Sleep(300 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"success","country":"中国","ip":"1.1.1.1"}`))
	}))
	defer server.Close()

	resolver := newTestResolver(server.URL)

	ctx := context.Background()
	// 第一次请求
	resolver.ResolveAsync(ctx, []string{"1.1.1.1"})

	// 等待一小段时间确保第一次请求进入 pending
	time.Sleep(50 * time.Millisecond)

	// 第二次请求（应该被跳过）
	resolver.ResolveAsync(ctx, []string{"1.1.1.1"})

	// 等待第一次请求完成
	time.Sleep(400 * time.Millisecond)

	// 验证服务器只被调用一次（第二次请求被跳过）
	mu.Lock()
	count := requestCount
	mu.Unlock()
	if count != 1 {
		t.Errorf("期望服务器只被调用 1 次，但调用了 %d 次", count)
	}
}

// ---------------------------------------------------------------------------
// 保留 IP 跳过场景测试
// ---------------------------------------------------------------------------

func TestTracertGeoResolver_ReservedIPSkip(t *testing.T) {
	// 开启 Debug 日志
	logger.EnableDebug = true
	defer func() {
		logger.EnableDebug = false
	}()

	// 创建模拟服务器，记录请求次数
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	resolver := newTestResolver(server.URL)

	// 测试各种保留 IP
	reservedIPs := []string{
		"192.168.1.1", // 私有 IP
		"10.0.0.1",    // 私有 IP
		"172.16.0.1",  // 私有 IP
		"127.0.0.1",   // 回环地址
		"100.64.0.1",  // CGNAT
		"169.254.1.1", // 链路本地
	}

	ctx := context.Background()
	resolver.ResolveAsync(ctx, reservedIPs)

	// 等待解析完成
	time.Sleep(200 * time.Millisecond)

	// 验证服务器没有被调用
	if requestCount != 0 {
		t.Errorf("期望服务器不被调用，但调用了 %d 次", requestCount)
	}

	// 验证所有保留 IP 都被缓存为 private_ip
	for _, ip := range reservedIPs {
		result := resolver.GetCachedResult(ip)
		if result == nil {
			t.Errorf("期望 IP %s 有缓存结果", ip)
			continue
		}
		if result.FailType != "private_ip" {
			t.Errorf("IP %s: 期望 FailType='private_ip', got '%s'", ip, result.FailType)
		}
	}
}

// ---------------------------------------------------------------------------
// 空IP和通配符跳过测试
// ---------------------------------------------------------------------------

func TestTracertGeoResolver_EmptyAndWildcardSkip(t *testing.T) {
	// 开启 Verbose 日志
	logger.EnableVerbose = true
	defer func() {
		logger.EnableVerbose = false
	}()

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"success","country":"中国","ip":"8.8.8.8"}`))
	}))
	defer server.Close()

	resolver := newTestResolver(server.URL)

	ctx := context.Background()
	// 测试空 IP 和通配符
	resolver.ResolveAsync(ctx, []string{"", "*", "8.8.8.8"})

	// 等待解析完成
	time.Sleep(200 * time.Millisecond)

	// 验证只有有效 IP 被请求（空 IP 和通配符被跳过）
	if requestCount != 1 {
		t.Errorf("期望服务器被调用 1 次，但调用了 %d 次", requestCount)
	}
}

// ---------------------------------------------------------------------------
// 失败缓存未过期跳过测试
// ---------------------------------------------------------------------------

func TestTracertGeoResolver_FailCacheNotExpiredSkip(t *testing.T) {
	// 开启 Verbose 日志
	logger.EnableVerbose = true
	defer func() {
		logger.EnableVerbose = false
	}()

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"success","country":"中国","ip":"1.1.1.1"}`))
	}))
	defer server.Close()

	resolver := newTestResolver(server.URL)

	// 预先缓存一个未过期的失败结果
	resolver.mu.Lock()
	resolver.cache["1.1.1.1"] = &GeoResolveResult{
		GeoInfo:      nil,
		CachedAt:     time.Now(),
		FailType:     "network",
		LastFailTime: time.Now(),
	}
	resolver.mu.Unlock()

	ctx := context.Background()
	resolver.ResolveAsync(ctx, []string{"1.1.1.1"})

	// 等待一下
	time.Sleep(100 * time.Millisecond)

	// 验证服务器没有被调用（失败缓存未过期）
	if requestCount != 0 {
		t.Errorf("期望服务器不被调用，但调用了 %d 次", requestCount)
	}
}

// ---------------------------------------------------------------------------
// 失败缓存已过期重新查询测试
// ---------------------------------------------------------------------------

func TestTracertGeoResolver_ExpiredFailCacheRetry(t *testing.T) {
	// 开启 Verbose 日志
	logger.EnableVerbose = true
	defer func() {
		logger.EnableVerbose = false
	}()

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"success","country":"中国","ip":"1.1.1.1"}`))
	}))
	defer server.Close()

	resolver := newTestResolver(server.URL)

	// 预先缓存一个已过期的失败结果（超过 60 秒）
	resolver.mu.Lock()
	resolver.cache["1.1.1.1"] = &GeoResolveResult{
		GeoInfo:      nil,
		CachedAt:     time.Now().Add(-120 * time.Second),
		FailType:     "network",
		LastFailTime: time.Now().Add(-120 * time.Second),
	}
	resolver.mu.Unlock()

	ctx := context.Background()
	resolver.ResolveAsync(ctx, []string{"1.1.1.1"})

	// 等待解析完成
	time.Sleep(200 * time.Millisecond)

	// 验证服务器被调用（失败缓存已过期，需要重新查询）
	if requestCount != 1 {
		t.Errorf("期望服务器被调用 1 次，但调用了 %d 次", requestCount)
	}

	// 验证缓存结果已更新为成功
	result := resolver.GetCachedResult("1.1.1.1")
	if result == nil {
		t.Fatal("期望缓存结果不为 nil")
	}
	if result.GeoInfo == nil {
		t.Fatal("期望 GeoInfo 不为 nil")
	}
}

// ---------------------------------------------------------------------------
// isReservedIP 函数测试
// ---------------------------------------------------------------------------

func TestIsReservedIP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		ip       string
		expected bool
	}{
		{"私有IP_10", "10.0.0.1", true},
		{"私有IP_172", "172.16.0.1", true},
		{"私有IP_192", "192.168.1.1", true},
		{"回环地址", "127.0.0.1", true},
		{"CGNAT", "100.64.0.1", true},
		{"链路本地", "169.254.1.1", true},
		{"公网IP", "8.8.8.8", false},
		{"公网IP_2", "1.1.1.1", false},
		{"无效IP", "invalid", true},
		{"空IP", "", true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := isReservedIP(tt.ip)
			if result != tt.expected {
				t.Errorf("isReservedIP(%q) = %v, want %v", tt.ip, result, tt.expected)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// GeoResolveResult.isFailCacheExpired 测试
// ---------------------------------------------------------------------------

func TestGeoResolveResult_IsFailCacheExpired(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		result   *GeoResolveResult
		expected bool
	}{
		{
			name: "成功缓存不过期",
			result: &GeoResolveResult{
				FailType: "",
			},
			expected: false,
		},
		{
			name: "private_ip永不过期",
			result: &GeoResolveResult{
				FailType:     "private_ip",
				LastFailTime: time.Now().Add(-24 * time.Hour),
			},
			expected: false,
		},
		{
			name: "rate_limit未过期",
			result: &GeoResolveResult{
				FailType:     "rate_limit",
				LastFailTime: time.Now().Add(-30 * time.Second),
			},
			expected: false,
		},
		{
			name: "rate_limit已过期",
			result: &GeoResolveResult{
				FailType:     "rate_limit",
				LastFailTime: time.Now().Add(-120 * time.Second),
			},
			expected: true,
		},
		{
			name: "network未过期",
			result: &GeoResolveResult{
				FailType:     "network",
				LastFailTime: time.Now().Add(-30 * time.Second),
			},
			expected: false,
		},
		{
			name: "network已过期",
			result: &GeoResolveResult{
				FailType:     "network",
				LastFailTime: time.Now().Add(-120 * time.Second),
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := tt.result.isFailCacheExpired()
			if result != tt.expected {
				t.Errorf("isFailCacheExpired() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// collectNewIPs 测试
// ---------------------------------------------------------------------------

func TestTracertGeoResolver_CollectNewIPs(t *testing.T) {
	t.Parallel()

	resolver := newTestResolver("")

	// 设置初始缓存状态
	resolver.mu.Lock()
	resolver.cache["cached-success"] = &GeoResolveResult{
		GeoInfo:  &icmp.GeoInfo{Status: "success"},
		CachedAt: time.Now(),
	}
	resolver.cache["cached-fail-fresh"] = &GeoResolveResult{
		FailType:     "network",
		LastFailTime: time.Now(),
	}
	resolver.cache["cached-fail-expired"] = &GeoResolveResult{
		FailType:     "network",
		LastFailTime: time.Now().Add(-120 * time.Second),
	}
	resolver.pending["pending-ip"] = struct{}{}
	resolver.mu.Unlock()

	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "空输入",
			input:    []string{},
			expected: []string{},
		},
		{
			name:     "空IP和通配符被过滤",
			input:    []string{"", "*", "valid-ip"},
			expected: []string{"valid-ip"},
		},
		{
			name:     "成功缓存被跳过",
			input:    []string{"cached-success", "new-ip"},
			expected: []string{"new-ip"},
		},
		{
			name:     "未过期失败缓存被跳过",
			input:    []string{"cached-fail-fresh", "new-ip"},
			expected: []string{"new-ip"},
		},
		{
			name:     "已过期失败缓存重新查询",
			input:    []string{"cached-fail-expired"},
			expected: []string{"cached-fail-expired"},
		},
		{
			name:     "pending状态被跳过",
			input:    []string{"pending-ip", "new-ip"},
			expected: []string{"new-ip"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := resolver.collectNewIPs(context.Background(), tt.input)
			if !equalStringSlices(result, tt.expected) {
				t.Errorf("collectNewIPs() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 辅助函数
// ---------------------------------------------------------------------------

// generateIPs 生成指定数量的测试 IP
func generateIPs(count int) []string {
	ips := make([]string, count)
	for i := 0; i < count; i++ {
		ips[i] = fmt.Sprintf("ip-%d", i)
	}
	return ips
}

// equalStringSlices 比较两个字符串切片是否相等
func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------------------
// 日志缓冲区捕获（用于验证日志输出）
// ---------------------------------------------------------------------------

// logBuffer 用于捕获日志输出的缓冲区
type logBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (lb *logBuffer) Write(p []byte) (n int, err error) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	return lb.buf.Write(p)
}

func (lb *logBuffer) String() string {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	return lb.buf.String()
}

func (lb *logBuffer) Contains(substr string) bool {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	return strings.Contains(lb.buf.String(), substr)
}

// ---------------------------------------------------------------------------
// 集成测试：完整流程验证
// ---------------------------------------------------------------------------

func TestTracertGeoResolver_FullIntegration(t *testing.T) {
	// 开启所有日志
	logger.EnableDebug = true
	logger.EnableVerbose = true
	defer func() {
		logger.EnableDebug = false
		logger.EnableVerbose = false
	}()

	// 创建模拟服务器
	callCount := 0
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callCount++
		mu.Unlock()
		// 提取 URL 中的 IP
		path := r.URL.Path
		ip := strings.TrimPrefix(path, "/json/")

		if ip == "4.4.4.4" {
			// 模拟 429
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}

		if ip == "9.10.11.12" {
			// 返回非 JSON
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte("<html>Error</html>"))
			return
		}

		// 正常响应
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(fmt.Sprintf(`{"status":"success","country":"中国","city":"北京","ip":"%s"}`, ip)))
	}))
	defer server.Close()

	resolver := newTestResolver(server.URL)

	// 测试多种场景
	testIPs := []string{
		"192.168.1.1", // 保留 IP，不请求
		"1.1.1.1",     // 成功
		"4.4.4.4",     // 429 限频
		"9.10.11.12",  // 非 JSON 响应（使用有效公网 IP）
		"",            // 空 IP，跳过
		"*",           // 通配符，跳过
	}

	ctx := context.Background()
	resolver.ResolveAsync(ctx, testIPs)

	// 等待所有请求完成
	time.Sleep(500 * time.Millisecond)

	// 验证结果
	// 保留 IP
	if result := resolver.GetCachedResult("192.168.1.1"); result == nil || result.FailType != "private_ip" {
		t.Error("保留 IP 应被缓存为 private_ip")
	}

	// 成功 IP
	if result := resolver.GetCachedResult("1.1.1.1"); result == nil || result.GeoInfo == nil {
		t.Error("成功 IP 应有 GeoInfo")
	}

	// 429 限频
	if result := resolver.GetCachedResult("4.4.4.4"); result == nil || result.FailType != "rate_limit" {
		t.Error("429 IP 应被缓存为 rate_limit")
	}

	// 非 JSON（使用有效公网 IP 9.10.11.12）
	if result := resolver.GetCachedResult("9.10.11.12"); result == nil || result.FailType != "api_fail" {
		t.Error("非 JSON IP 应被缓存为 api_fail")
	}

	// 空 IP 和通配符不应有缓存
	if result := resolver.GetCachedResult(""); result != nil {
		t.Error("空 IP 不应有缓存")
	}
	if result := resolver.GetCachedResult("*"); result != nil {
		t.Error("通配符不应有缓存")
	}

	// 验证请求次数
	// 保留 IP 不请求，空 IP 和通配符不请求
	// 1.1.1.1: 1 次
	// 4.4.4.4: 1 次（429 不重试）
	// 9.10.11.12: 2 次（非 JSON 会重试一次）
	// 总计: 4 次
	mu.Lock()
	count := callCount
	mu.Unlock()
	if count != 4 {
		t.Errorf("期望调用 4 次，实际调用 %d 次", count)
	}
}

// ---------------------------------------------------------------------------
// 并发安全测试
// ---------------------------------------------------------------------------

func TestTracertGeoResolver_ConcurrentSafety(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond) // 模拟延迟
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"success","country":"中国","ip":"test"}`))
	}))
	defer server.Close()

	resolver := newTestResolver(server.URL)

	// 并发发起多次请求
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ip := fmt.Sprintf("1.1.%d.%d", i/256, i%256)
			ctx := context.Background()
			resolver.ResolveAsync(ctx, []string{ip})
		}(i)
	}

	wg.Wait()

	// 等待所有请求完成
	time.Sleep(500 * time.Millisecond)

	// 验证没有 panic 或竞态条件（通过 -race 标志检测）
}

// ---------------------------------------------------------------------------
// Context 取消测试
// ---------------------------------------------------------------------------

func TestTracertGeoResolver_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond) // 长延迟
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	resolver := newTestResolver(server.URL)

	ctx, cancel := context.WithCancel(context.Background())

	// 发起请求
	resolver.ResolveAsync(ctx, []string{"1.1.1.1"})

	// 立即取消
	cancel()

	// 等待一下
	time.Sleep(50 * time.Millisecond)

	// 验证 pending 被清除
	if resolver.HasPendingIPs() {
		t.Error("期望 pending 被清除")
	}
}
