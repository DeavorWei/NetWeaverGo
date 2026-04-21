//go:build windows

package icmp

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"
)

func TestPingOne_Localhost(t *testing.T) {
	ip := net.ParseIP("127.0.0.1")
	if ip == nil {
		t.Fatal("Failed to parse localhost IP")
	}

	result, err := PingOne(ip, 1000, 32)
	if err != nil {
		t.Fatalf("PingOne failed: %v", err)
	}

	if !result.Success {
		t.Errorf("Expected success for localhost, got: %s", result.Status)
	}

	if result.IP != "127.0.0.1" {
		t.Errorf("Expected IP 127.0.0.1, got: %s", result.IP)
	}

	if result.RoundTripTime > 100 {
		t.Logf("Warning: localhost RTT is high: %.2fms", result.RoundTripTime)
	}
}

func TestPingOne_Timeout(t *testing.T) {
	// Use a non-routable IP that should timeout
	ip := net.ParseIP("10.255.255.1")
	if ip == nil {
		t.Fatal("Failed to parse IP")
	}

	result, err := PingOne(ip, 500, 32)
	if err != nil {
		t.Fatalf("PingOne failed: %v", err)
	}

	if result.Success {
		t.Error("Expected timeout for non-routable IP")
	}
}

func TestBatchPingEngine_SmallRange(t *testing.T) {
	config := DefaultPingConfig()
	config.Concurrency = 4
	config.Timeout = 1000

	engine := NewBatchPingEngine(config)

	ips := []string{"127.0.0.1"}
	var progressSnapshots []*BatchPingProgress

	progress := engine.Run(context.Background(), ips, func(p *BatchPingProgress) {
		progressSnapshots = append(progressSnapshots, p)
	}, nil)

	if progress.TotalIPs != 1 {
		t.Errorf("Expected TotalIPs=1, got %d", progress.TotalIPs)
	}

	if progress.CompletedIPs != 1 {
		t.Errorf("Expected CompletedIPs=1, got %d", progress.CompletedIPs)
	}

	if len(progress.Results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(progress.Results))
	}

	if !progress.Results[0].Alive {
		t.Error("Expected localhost to be alive")
	}

	if progress.IsRunning {
		t.Error("Expected IsRunning=false after completion")
	}
}

func TestBatchPingEngine_Cancel(t *testing.T) {
	config := DefaultPingConfig()
	config.Concurrency = 1
	config.Timeout = 5000 // Long timeout

	engine := NewBatchPingEngine(config)

	// Create many IPs to give time for cancellation
	ips := make([]string, 100)
	for i := range ips {
		ips[i] = "10.255.255.1" // Non-routable, will timeout
	}

	ctx, cancel := context.WithCancel(context.Background())
	
	// Cancel after a short delay
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	progress := engine.Run(ctx, ips, nil, nil)

	// Should have stopped before completing all
	if progress.CompletedIPs >= 100 {
		t.Error("Expected cancellation to stop before completion")
	}

	if progress.IsRunning {
		t.Error("Expected IsRunning=false after cancellation")
	}
}

func TestBatchPingEngine_InvalidIP(t *testing.T) {
	config := DefaultPingConfig()
	engine := NewBatchPingEngine(config)

	ips := []string{"invalid-ip"}
	progress := engine.Run(context.Background(), ips, nil, nil)

	if len(progress.Results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(progress.Results))
	}

	if progress.Results[0].Status != "error" {
		t.Errorf("Expected error status for invalid IP, got: %s", progress.Results[0].Status)
	}
}

func TestBatchPingEngine_ConcurrencyLimit(t *testing.T) {
	config := DefaultPingConfig()
	config.Concurrency = 2
	config.Timeout = 1000

	engine := NewBatchPingEngine(config)

	// Verify config is applied correctly
	if engine.GetConfig().Concurrency != 2 {
		t.Errorf("Expected Concurrency=2, got %d", engine.GetConfig().Concurrency)
	}
}

func TestBatchPingEngine_MaxConcurrency(t *testing.T) {
	config := DefaultPingConfig()
	config.Concurrency = 1000 // Over limit

	engine := NewBatchPingEngine(config)

	// Should be capped at 256
	if engine.GetConfig().Concurrency != 256 {
		t.Errorf("Expected Concurrency to be capped at 256, got %d", engine.GetConfig().Concurrency)
	}
}

func TestDefaultPingConfig(t *testing.T) {
	config := DefaultPingConfig()

	if config.Timeout != 1000 {
		t.Errorf("Expected default Timeout=1000, got %d", config.Timeout)
	}

	if config.DataSize != 32 {
		t.Errorf("Expected default DataSize=32, got %d", config.DataSize)
	}

	if config.Count != 1 {
		t.Errorf("Expected default Count=1, got %d", config.Count)
	}

	if config.Concurrency != 64 {
		t.Errorf("Expected default Concurrency=64, got %d", config.Concurrency)
	}
}

func TestBatchPingProgress_UpdateProgress(t *testing.T) {
	// 构造 100 个测试 IP
	testIPs := make([]string, 100)
	for i := range testIPs {
		testIPs[i] = fmt.Sprintf("192.168.0.%d", i+1)
	}
	progress := NewBatchPingProgress(testIPs)

	if progress.TotalIPs != 100 {
		t.Errorf("Expected TotalIPs=100, got %d", progress.TotalIPs)
	}

	if progress.Progress != 0 {
		t.Errorf("Expected initial Progress=0, got %f", progress.Progress)
	}

	progress.CompletedIPs = 50
	progress.UpdateProgress()

	if progress.Progress != 50 {
		t.Errorf("Expected Progress=50, got %f", progress.Progress)
	}
}

func TestBatchPingProgress_SetResult(t *testing.T) {
	progress := NewBatchPingProgress([]string{"192.168.1.1"})

	result := PingHostResult{
		IP:     "192.168.1.1",
		Alive:  true,
		Status: "online",
	}

	// 模拟引擎实际行为（SetResult 内部已更新计数器，无需手动 ++）
	progress.SetResult(0, result)

	if progress.CompletedIPs != 1 {
		t.Errorf("Expected CompletedIPs=1, got %d", progress.CompletedIPs)
	}

	if progress.OnlineCount != 1 {
		t.Errorf("Expected OnlineCount=1, got %d", progress.OnlineCount)
	}

	if len(progress.Results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(progress.Results))
	}
}

func TestIcmpStatusToString(t *testing.T) {
	tests := []struct {
		status   uint32
		expected string
	}{
		{IP_SUCCESS, "Success"},
		{IP_REQ_TIMED_OUT, "Request Timed Out"},
		{IP_DEST_NET_UNREACHABLE, "Destination Network Unreachable"},
		{IP_DEST_HOST_UNREACHABLE, "Destination Host Unreachable"},
		{IP_TTL_EXPIRED_TRANSIT, "TTL Expired in Transit"},
		{99999, "Unknown Error (99999)"},
	}

	for _, tt := range tests {
		result := icmpStatusToString(tt.status)
		if result != tt.expected {
			t.Errorf("icmpStatusToString(%d) = %s, expected %s", tt.status, result, tt.expected)
		}
	}
}

func TestPingOne_LargeDataSize(t *testing.T) {
	testCases := []struct {
		name        string
		dataSize    uint16
		expectError bool // localhost may not support very large packets
	}{
		{"Small_32", 32, false},
		{"Medium_300", 300, false},      // Previously failed before fix
		{"Large_1000", 1000, false},
		{"Large_8000", 8000, false},     // Test larger but reasonable size
		// Note: 65500 is skipped because localhost loopback interface
		// typically doesn't support maximum-sized ICMP packets
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ip := net.ParseIP("127.0.0.1")
			if ip == nil {
				t.Fatal("Failed to parse localhost IP")
			}

			result, err := PingOne(ip, 2000, tc.dataSize)
			if err != nil {
				t.Fatalf("PingOne failed for dataSize=%d: %v", tc.dataSize, err)
			}

			if !result.Success && !tc.expectError {
				t.Errorf("Expected success for dataSize=%d, got: %s (error: %s)",
					tc.dataSize, result.Status, result.Error)
			}

			if result.RoundTripTime > 100 {
				t.Logf("Warning: localhost RTT is high for dataSize=%d: %.2fms",
					tc.dataSize, result.RoundTripTime)
			}

			t.Logf("dataSize=%d: success=%v, rtt=%.2fms, ttl=%d",
				tc.dataSize, result.Success, result.RoundTripTime, result.TTL)
		})
	}
}

func TestBatchPingEngine_LargeDataSize(t *testing.T) {
	config := DefaultPingConfig()
	config.DataSize = 1000
	config.Concurrency = 4
	config.Timeout = 2000

	engine := NewBatchPingEngine(config)

	ips := []string{"127.0.0.1"}
	progress := engine.Run(context.Background(), ips, nil, nil)

	if len(progress.Results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(progress.Results))
	}

	if !progress.Results[0].Alive {
		t.Errorf("Expected localhost to be alive with dataSize=1000, got: %s",
			progress.Results[0].ErrorMsg)
	}
}
