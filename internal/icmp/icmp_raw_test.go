//go:build !rawicmp

package icmp

import (
	"net"
	"testing"
)

func TestPingOne_Localhost_Raw(t *testing.T) {
	ip := net.ParseIP("127.0.0.1")
	if ip == nil {
		t.Fatal("Failed to parse localhost IP")
	}

	result, err := PingOne(ip, 3000, 32)
	if err != nil {
		t.Fatalf("PingOne failed: %v", err)
	}

	if !result.Success {
		t.Errorf("Expected success for localhost, got: %s (error: %s)", result.Status, result.Error)
	}

	if result.IP != "127.0.0.1" {
		t.Errorf("Expected IP 127.0.0.1, got: %s", result.IP)
	}

	if result.RoundTripTime > 100 {
		t.Logf("Warning: localhost RTT is high: %.2fms", result.RoundTripTime)
	}
}

func TestPingOne_Timeout_Raw(t *testing.T) {
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

func TestPingOne_LargeDataSize_Raw(t *testing.T) {
	testCases := []struct {
		name     string
		dataSize uint16
	}{
		{"Small_32", 32},
		{"Medium_300", 300},
		{"Large_1000", 1000},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ip := net.ParseIP("127.0.0.1")
			if ip == nil {
				t.Fatal("Failed to parse localhost IP")
			}

			result, err := PingOne(ip, 3000, tc.dataSize)
			if err != nil {
				t.Fatalf("PingOne failed for dataSize=%d: %v", tc.dataSize, err)
			}

			if !result.Success {
				t.Errorf("Expected success for dataSize=%d, got: %s (error: %s)",
					tc.dataSize, result.Status, result.Error)
			}

			t.Logf("dataSize=%d: success=%v, rtt=%.2fms, ttl=%d",
				tc.dataSize, result.Success, result.RoundTripTime, result.TTL)
		})
	}
}

func TestGetBackendName_Raw(t *testing.T) {
	name := GetBackendName()
	if name != "RawSocket(golang.org/x/net/icmp)" {
		t.Errorf("Expected RawSocket backend name, got: %s", name)
	}
}

func TestGetBackend_Raw(t *testing.T) {
	bt := GetBackend()
	if bt != BackendRawSocket {
		t.Errorf("Expected BackendRawSocket, got: %d", bt)
	}
}
