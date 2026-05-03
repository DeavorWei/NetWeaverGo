package icmp

import (
	"context"
	"net"
	"os"
	"testing"
	"time"
)

// TestPingHost_Statistics 测试统计逻辑
func TestPingHost_Statistics(t *testing.T) {
	if os.Getenv("CI") == "true" {
		t.Skip("Skipping network test in CI environment")
	}

	engine := NewBatchPingEngine(PingConfig{
		Timeout:  1000,
		Count:    3,
		DataSize: 32,
	})

	ip := net.ParseIP("127.0.0.1")
	if ip == nil {
		t.Skip("Cannot parse 127.0.0.1")
	}

	result := engine.pingHost(context.Background(), ip, nil)

	if result.SentCount != 3 {
		t.Errorf("SentCount = %d, want 3", result.SentCount)
	}
	if result.RecvCount+result.FailedCount != result.SentCount {
		t.Errorf("RecvCount + FailedCount != SentCount")
	}
	if result.RecvCount > 0 {
		if result.MinRtt < 0 {
			t.Error("MinRtt should be valid when RecvCount > 0")
		}
		if result.AvgRtt <= 0 && result.MinRtt > 0 {
			t.Error("AvgRtt should be positive when MinRtt > 0 and RecvCount > 0")
		}
	}
}

// TestPingHost_Timestamp 测试时间戳记录
func TestPingHost_Timestamp(t *testing.T) {
	if os.Getenv("CI") == "true" {
		t.Skip("Skipping network test in CI environment")
	}

	engine := NewBatchPingEngine(PingConfig{
		Timeout:  1000,
		Count:    2,
		DataSize: 32,
	})

	ip := net.ParseIP("127.0.0.1")
	if ip == nil {
		t.Skip("Cannot parse 127.0.0.1")
	}

	result := engine.pingHost(context.Background(), ip, nil)

	now := time.Now().UnixMilli()
	if result.LastSucceedAt > 0 {
		if result.LastSucceedAt > now+1000 || result.LastSucceedAt < now-10000 {
			t.Errorf("LastSucceedAt = %d, expected near %d", result.LastSucceedAt, now)
		}
	}
}
