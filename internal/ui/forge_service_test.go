package ui

import "testing"

func TestParseIPRange_Dash(t *testing.T) {
	svc := NewForgeService()
	res := svc.ParseIPRange("192.168.58.201-202")
	if !res.IsValid {
		t.Fatalf("expected valid result, got invalid: %s", res.Message)
	}
	if res.Count != 2 {
		t.Fatalf("expected count=2, got %d", res.Count)
	}
	if res.Start != "192.168.58.201" || res.End != "192.168.58.202" {
		t.Fatalf("unexpected range boundaries: start=%s end=%s", res.Start, res.End)
	}
}

func TestParseIPRange_Tilde(t *testing.T) {
	svc := NewForgeService()
	res := svc.ParseIPRange("192.168.58.201~202")
	if !res.IsValid {
		t.Fatalf("expected valid result, got invalid: %s", res.Message)
	}
	if res.Count != 2 {
		t.Fatalf("expected count=2, got %d", res.Count)
	}
}

func TestParseIPRange_DescendingRejected(t *testing.T) {
	svc := NewForgeService()
	res := svc.ParseIPRange("192.168.58.202-201")
	if res.IsValid {
		t.Fatalf("expected invalid result, got valid: %#v", res)
	}
	if res.Message == "" {
		t.Fatal("expected error message for descending range")
	}
}

func TestParseIPRange_OutOfRangeRejected(t *testing.T) {
	svc := NewForgeService()
	res := svc.ParseIPRange("192.168.58.201-300")
	if res.IsValid {
		t.Fatalf("expected invalid result, got valid: %#v", res)
	}
	if res.Message == "" {
		t.Fatal("expected error message for out-of-range value")
	}
}
