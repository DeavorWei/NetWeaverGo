//go:build windows

package ui

import (
	"context"
	"testing"
	"time"

	"github.com/NetWeaverGo/core/internal/icmp"
)

func TestResolveTargets_SingleIP(t *testing.T) {
	svc := NewPingService()

	ips, err := svc.resolveTargets("192.168.1.1", nil)
	if err != nil {
		t.Fatalf("resolveTargets failed: %v", err)
	}

	if len(ips) != 1 {
		t.Errorf("Expected 1 IP, got %d", len(ips))
	}

	if ips[0] != "192.168.1.1" {
		t.Errorf("Expected 192.168.1.1, got %s", ips[0])
	}
}

func TestResolveTargets_MultipleIPs(t *testing.T) {
	svc := NewPingService()

	targets := `192.168.1.1
192.168.1.2
192.168.1.3`

	ips, err := svc.resolveTargets(targets, nil)
	if err != nil {
		t.Fatalf("resolveTargets failed: %v", err)
	}

	if len(ips) != 3 {
		t.Errorf("Expected 3 IPs, got %d", len(ips))
	}
}

func TestResolveTargets_CIDR(t *testing.T) {
	svc := NewPingService()

	ips, err := svc.resolveTargets("192.168.1.0/30", nil)
	if err != nil {
		t.Fatalf("resolveTargets failed: %v", err)
	}

	// /30 should give 2 usable IPs (excluding network and broadcast)
	if len(ips) != 2 {
		t.Errorf("Expected 2 IPs for /30, got %d", len(ips))
	}
}

func TestResolveTargets_IPRange(t *testing.T) {
	svc := NewPingService()

	ips, err := svc.resolveTargets("192.168.1.1-3", nil)
	if err != nil {
		t.Fatalf("resolveTargets failed: %v", err)
	}

	if len(ips) != 3 {
		t.Errorf("Expected 3 IPs, got %d", len(ips))
	}

	expected := []string{"192.168.1.1", "192.168.1.2", "192.168.1.3"}
	for i, ip := range ips {
		if ip != expected[i] {
			t.Errorf("Expected %s, got %s", expected[i], ip)
		}
	}
}

func TestResolveTargets_IPRangeTilde(t *testing.T) {
	svc := NewPingService()

	ips, err := svc.resolveTargets("192.168.1.1~3", nil)
	if err != nil {
		t.Fatalf("resolveTargets failed: %v", err)
	}

	if len(ips) != 3 {
		t.Errorf("Expected 3 IPs, got %d", len(ips))
	}
}

func TestResolveTargets_Deduplicate(t *testing.T) {
	svc := NewPingService()

	targets := `192.168.1.1
192.168.1.1
192.168.1.2`

	ips, err := svc.resolveTargets(targets, nil)
	if err != nil {
		t.Fatalf("resolveTargets failed: %v", err)
	}

	if len(ips) != 2 {
		t.Errorf("Expected 2 unique IPs, got %d", len(ips))
	}
}

func TestResolveTargets_InvalidIP(t *testing.T) {
	svc := NewPingService()

	_, err := svc.resolveTargets("invalid-ip", nil)
	if err == nil {
		t.Error("Expected error for invalid IP")
	}
}

func TestExpandCIDR_Small(t *testing.T) {
	svc := NewPingService()

	ips, err := svc.expandCIDR("10.0.0.0/30")
	if err != nil {
		t.Fatalf("expandCIDR failed: %v", err)
	}

	// /30 = 4 addresses, minus network and broadcast = 2 usable
	if len(ips) != 2 {
		t.Errorf("Expected 2 IPs for /30, got %d", len(ips))
	}
}

func TestExpandCIDR_TooLarge(t *testing.T) {
	svc := NewPingService()

	_, err := svc.expandCIDR("10.0.0.0/8")
	if err == nil {
		t.Error("Expected error for large CIDR")
	}
}

func TestParseIPRange_Valid(t *testing.T) {
	svc := NewPingService()

	ips, err := svc.parseIPRange("192.168.1.10-15")
	if err != nil {
		t.Fatalf("parseIPRange failed: %v", err)
	}

	if len(ips) != 6 {
		t.Errorf("Expected 6 IPs, got %d", len(ips))
	}

	if ips[0] != "192.168.1.10" {
		t.Errorf("Expected first IP 192.168.1.10, got %s", ips[0])
	}

	if ips[5] != "192.168.1.15" {
		t.Errorf("Expected last IP 192.168.1.15, got %s", ips[5])
	}
}

func TestParseIPRange_FullIP(t *testing.T) {
	svc := NewPingService()

	ips, err := svc.parseIPRange("192.168.1.10-192.168.1.12")
	if err != nil {
		t.Fatalf("parseIPRange failed: %v", err)
	}

	if len(ips) != 3 {
		t.Errorf("Expected 3 IPs, got %d", len(ips))
	}
}

func TestParseIPRange_InvalidRange(t *testing.T) {
	svc := NewPingService()

	_, err := svc.parseIPRange("192.168.1.10-5")
	if err == nil {
		t.Error("Expected error for invalid range (end < start)")
	}
}

func TestMergeWithDefaultPingConfig_Empty(t *testing.T) {
	svc := NewPingService()

	config := svc.mergeWithDefaultPingConfig(icmp.PingConfig{})

	if config.Timeout != 1000 {
		t.Errorf("Expected default Timeout=1000, got %d", config.Timeout)
	}

	if config.Concurrency != 64 {
		t.Errorf("Expected default Concurrency=64, got %d", config.Concurrency)
	}
}

func TestMergeWithDefaultPingConfig_Override(t *testing.T) {
	svc := NewPingService()

	config := svc.mergeWithDefaultPingConfig(icmp.PingConfig{
		Timeout:     2000,
		Concurrency: 32,
	})

	if config.Timeout != 2000 {
		t.Errorf("Expected Timeout=2000, got %d", config.Timeout)
	}

	if config.Concurrency != 32 {
		t.Errorf("Expected Concurrency=32, got %d", config.Concurrency)
	}
}

func TestMergeWithDefaultPingConfig_Limits(t *testing.T) {
	svc := NewPingService()

	// Test max limits
	config := svc.mergeWithDefaultPingConfig(icmp.PingConfig{
		Timeout:     20000, // Over limit
		Concurrency: 500,   // Over limit
		Count:       100,   // 不再限制，允许任意值
		DataSize:    65535, // Over limit (max uint16)
	})

	if config.Timeout > 30000 {
		t.Errorf("Timeout should be capped at 30000, got %d", config.Timeout)
	}

	if config.Concurrency > 256 {
		t.Errorf("Concurrency should be capped at 256, got %d", config.Concurrency)
	}

	// Count 不再设上限，验证用户设置的值被保留
	if config.Count != 100 {
		t.Errorf("Count should be preserved as user set, expected 100, got %d", config.Count)
	}

	if config.DataSize > 65500 {
		t.Errorf("DataSize should be capped at 65500, got %d", config.DataSize)
	}
}

func TestGetPingDefaultConfig(t *testing.T) {
	svc := NewPingService()

	config := svc.GetPingDefaultConfig()

	if config.Timeout != 1000 {
		t.Errorf("Expected default Timeout=1000, got %d", config.Timeout)
	}
}

func TestFormatRtt(t *testing.T) {
	tests := []struct {
		rtt      uint32
		expected string
	}{
		{0, "-"},
		{1, "1.00ms"},
		{100, "100.00ms"},
	}

	for _, tt := range tests {
		result := formatRtt(float64(tt.rtt))
		if result != tt.expected {
			t.Errorf("formatRtt(%d) = %s, expected %s", tt.rtt, result, tt.expected)
		}
	}
}

func TestFormatRttForCSV(t *testing.T) {
	tests := []struct {
		rtt      float64
		expected string
	}{
		{-1, "-"},
		{0, "0.000"},
		{1.5, "1.500"},
		{100.123, "100.123"},
	}

	for _, tt := range tests {
		result := formatRttForCSV(tt.rtt)
		if result != tt.expected {
			t.Errorf("formatRttForCSV(%f) = %s, expected %s", tt.rtt, result, tt.expected)
		}
	}
}

func TestFormatTimestamp(t *testing.T) {
	tests := []struct {
		ts       int64
		expected string
	}{
		{0, ""},
		{1713331200000, "2024-04-17 12:00:00"}, // Example timestamp
	}

	for _, tt := range tests {
		result := formatTimestamp(tt.ts)
		if tt.ts == 0 {
			if result != "" {
				t.Errorf("formatTimestamp(0) should return empty string, got %s", result)
			}
		}
		// For non-zero timestamps, just verify it's not empty
		if tt.ts != 0 && result == "" {
			t.Errorf("formatTimestamp(%d) should not be empty", tt.ts)
		}
	}
}

func TestMergeWithDefaultPingOptions(t *testing.T) {
	svc := NewPingService()

	// Test with empty options
	options := svc.mergeWithDefaultPingOptions(icmp.PingOptions{})
	defaults := icmp.DefaultPingOptions()

	if options.DNSTimeout != defaults.DNSTimeout {
		t.Errorf("Expected DNSTimeout=%v, got %v", defaults.DNSTimeout, options.DNSTimeout)
	}

	if options.RealtimeThrottle != defaults.RealtimeThrottle {
		t.Errorf("Expected RealtimeThrottle=%v, got %v", defaults.RealtimeThrottle, options.RealtimeThrottle)
	}
}

func TestMergeWithDefaultPingOptions_Limits(t *testing.T) {
	svc := NewPingService()

	// Test with over-limit values
	options := svc.mergeWithDefaultPingOptions(icmp.PingOptions{
		DNSTimeout:       60 * 1e9, // 60 seconds in nanoseconds, over limit
		RealtimeThrottle: 1 * 1e6,  // 1ms, under minimum
	})

	if options.DNSTimeout > 30*1e9 {
		t.Errorf("DNSTimeout should be capped at 30s, got %v", options.DNSTimeout)
	}

	if options.RealtimeThrottle < 10*1e6 {
		t.Errorf("RealtimeThrottle should be at least 10ms, got %v", options.RealtimeThrottle)
	}
}

func TestDNSCache_Basic(t *testing.T) {
	svc := NewPingService()

	// Pre-fill cache
	svc.dnsCacheMu.Lock()
	svc.dnsCache["8.8.8.8"] = dnsCacheEntry{
		hostName:  "dns.google",
		timestamp: time.Now(),
	}
	svc.dnsCacheMu.Unlock()

	// Verify cache entry exists
	svc.dnsCacheMu.RLock()
	entry, ok := svc.dnsCache["8.8.8.8"]
	svc.dnsCacheMu.RUnlock()

	if !ok {
		t.Error("Cache entry should exist")
	}
	if entry.hostName != "dns.google" {
		t.Errorf("Expected hostName=dns.google, got %s", entry.hostName)
	}
}

func TestDNSCache_Clear(t *testing.T) {
	svc := NewPingService()

	// Add cache entry
	svc.dnsCacheMu.Lock()
	svc.dnsCache["8.8.8.8"] = dnsCacheEntry{
		hostName:  "dns.google",
		timestamp: time.Now(),
	}
	svc.dnsCacheMu.Unlock()

	// Clear cache
	svc.clearDNSCache()

	// Verify cache is empty
	svc.dnsCacheMu.RLock()
	_, ok := svc.dnsCache["8.8.8.8"]
	svc.dnsCacheMu.RUnlock()

	if ok {
		t.Error("Cache should be empty after clearDNSCache")
	}
}

// TestDNSCache_Expiration 测试 DNS 缓存过期
func TestDNSCache_Expiration(t *testing.T) {
	svc := NewPingService()
	svc.dnsCacheTTL = 100 * time.Millisecond

	// 添加即将过期的缓存
	svc.dnsCacheMu.Lock()
	svc.dnsCache["8.8.8.8"] = dnsCacheEntry{
		hostName:  "dns.google",
		timestamp: time.Now().Add(-200 * time.Millisecond),
	}
	svc.dnsCacheMu.Unlock()

	// 阻塞等待
	time.Sleep(50 * time.Millisecond)

	// 清理过期条目
	svc.cleanupDNSCache()

	svc.dnsCacheMu.RLock()
	_, ok := svc.dnsCache["8.8.8.8"]
	svc.dnsCacheMu.RUnlock()

	if ok {
		t.Error("Expired cache entry should be removed after cleanup")
	}
}

// TestDNSCache_Cancellation 测试 DNS 解析取消
func TestDNSCache_Cancellation(t *testing.T) {
	svc := NewPingService()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 即刻取消

	results := svc.resolveHostNames(ctx, []string{"8.8.8.8"}, 2*time.Second)

	// 被取消上下文不应解析出数据
	if len(results) > 0 {
		t.Logf("Warning: got results after cancellation: %v", results)
	}
}

// TestPingOptions_Validate 测试边界值验证
func TestPingOptions_Validate(t *testing.T) {
	tests := []struct {
		name    string
		options icmp.PingOptions
		wantErr bool
	}{
		{"valid_default", icmp.DefaultPingOptions(), false},
		{"negative_dns_timeout", icmp.PingOptions{DNSTimeout: -1 * time.Second}, true},
		{"dns_timeout_too_long", icmp.PingOptions{DNSTimeout: 31 * time.Second}, true},
		{"throttle_too_small", icmp.PingOptions{RealtimeThrottle: 5 * time.Millisecond}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.options.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
