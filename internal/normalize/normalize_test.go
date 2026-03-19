package normalize

import "testing"

func TestNormalizeInterfaceName(t *testing.T) {
	tests := []struct {
		raw      string
		expected string
	}{
		{raw: "GigabitEthernet1/0/1", expected: "GE1/0/1"},
		{raw: "XGigabitEthernet1/0/1", expected: "XGE1/0/1"},
		{raw: "Eth-Trunk10", expected: "Trunk10"},
		{raw: "GE1/0/1", expected: "GE1/0/1"},
	}

	for _, tt := range tests {
		if got := NormalizeInterfaceName(tt.raw); got != tt.expected {
			t.Fatalf("NormalizeInterfaceName(%q)=%q, expected=%q", tt.raw, got, tt.expected)
		}
	}
}

func TestNormalizeMAC(t *testing.T) {
	tests := []struct {
		raw      string
		expected string
	}{
		{raw: "00-11-22-33-44-55", expected: "00:11:22:33:44:55"},
		{raw: "0011.2233.4455", expected: "00:11:22:33:44:55"},
		{raw: "00:11:22:33:44:55", expected: "00:11:22:33:44:55"},
	}

	for _, tt := range tests {
		if got := NormalizeMAC(tt.raw); got != tt.expected {
			t.Fatalf("NormalizeMAC(%q)=%q, expected=%q", tt.raw, got, tt.expected)
		}
	}
}

func TestNormalizeLLDPRemotePortAndAggregateName(t *testing.T) {
	if got := NormalizeLLDPRemotePort("-"); got != "" {
		t.Fatalf("expected empty remote port for '-', got %q", got)
	}
	if got := NormalizeLLDPRemotePort("GigabitEthernet1/0/24"); got != "GE1/0/24" {
		t.Fatalf("unexpected LLDP remote port: %q", got)
	}

	if got := NormalizeAggregateName("Eth-Trunk10"); got != "Trunk10" {
		t.Fatalf("unexpected aggregate name for Eth-Trunk10: %q", got)
	}
	if got := NormalizeAggregateName("Port-channel2"); got != "Po2" {
		t.Fatalf("unexpected aggregate name for Port-channel2: %q", got)
	}
}
