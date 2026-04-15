//go:build windows

package ui

import (
	"testing"

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
		Count:        20,    // Over limit
		DataSize:     65535, // Over limit (max uint16)
	})

	if config.Timeout > 10000 {
		t.Errorf("Timeout should be capped at 10000, got %d", config.Timeout)
	}

	if config.Concurrency > 256 {
		t.Errorf("Concurrency should be capped at 256, got %d", config.Concurrency)
	}

	if config.Count > 10 {
		t.Errorf("Count should be capped at 10, got %d", config.Count)
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
