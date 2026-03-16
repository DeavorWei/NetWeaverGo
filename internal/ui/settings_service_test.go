package ui

import (
	"reflect"
	"testing"
)

func TestBuildAlgorithmOptions_SecurityOrderAndSorting(t *testing.T) {
	secure := []string{"z-secure", "a-secure"}
	insecure := []string{"z-insecure", "a-insecure"}

	got := buildAlgorithmOptions(secure, insecure)

	want := []SSHAlgorithmOption{
		{Name: "a-secure", Security: "secure", Source: "supported"},
		{Name: "z-secure", Security: "secure", Source: "supported"},
		{Name: "a-insecure", Security: "insecure", Source: "insecure"},
		{Name: "z-insecure", Security: "insecure", Source: "insecure"},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected options order/content\nwant=%#v\ngot=%#v", want, got)
	}
}

func TestBuildAlgorithmOptions_Deduplicate(t *testing.T) {
	secure := []string{"same", "secure-only"}
	insecure := []string{"same", "insecure-only"}

	got := buildAlgorithmOptions(secure, insecure)
	if len(got) != 3 {
		t.Fatalf("expected 3 unique options, got %d", len(got))
	}

	for _, item := range got {
		if item.Name == "same" && (item.Security != "secure" || item.Source != "supported") {
			t.Fatalf("duplicate precedence incorrect for 'same': %#v", item)
		}
	}
}

func TestGetSSHAlgorithmOptions_StableAndNonEmpty(t *testing.T) {
	svc := NewSettingsService()

	first := svc.GetSSHAlgorithmOptions()
	second := svc.GetSSHAlgorithmOptions()

	if len(first.Ciphers) == 0 || len(first.KeyExchanges) == 0 || len(first.MACs) == 0 || len(first.HostKeyAlgorithms) == 0 {
		t.Fatalf("expected all algorithm categories to be non-empty")
	}

	if !reflect.DeepEqual(first, second) {
		t.Fatalf("expected stable result across calls")
	}
}

