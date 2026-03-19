package topology

import "testing"

func TestInferRemoteDeviceByFDBMAC(t *testing.T) {
	ownerByMAC := map[string]map[string]struct{}{
		"aa:aa:aa:aa:aa:01": {"10.0.0.1": {}, "10.0.0.2": {}},
		"aa:aa:aa:aa:aa:02": {"10.0.0.2": {}},
		"aa:aa:aa:aa:aa:03": {"10.0.0.3": {}},
	}

	if got := inferRemoteDeviceByFDBMAC(ownerByMAC, "10.0.0.1", []string{"aa:aa:aa:aa:aa:01", "aa:aa:aa:aa:aa:02"}, 8); got != "10.0.0.2" {
		t.Fatalf("expected 10.0.0.2, got %s", got)
	}

	if got := inferRemoteDeviceByFDBMAC(ownerByMAC, "10.0.0.1", []string{"aa:aa:aa:aa:aa:01", "aa:aa:aa:aa:aa:03"}, 8); got != "" {
		t.Fatalf("expected tie to return empty, got %s", got)
	}

	if got := inferRemoteDeviceByFDBMAC(ownerByMAC, "10.0.0.1", []string{"aa:aa:aa:aa:aa:01", "aa:aa:aa:aa:aa:02", "aa:aa:aa:aa:aa:03"}, 1); got != "" {
		t.Fatalf("expected candidate-overflow to return empty, got %s", got)
	}
}
