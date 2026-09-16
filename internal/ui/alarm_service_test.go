package ui

import (
	"testing"
)

func TestAlarmService_ListRules(t *testing.T) {
	service := NewAlarmService()
	if service == nil {
		t.Fatalf("NewAlarmService returned nil")
	}

	rules, err := service.ListAlarmRules("CE")
	if err != nil {
		t.Fatalf("ListAlarmRules failed: %v", err)
	}
	if len(rules) == 0 {
		t.Errorf("expected CE rules, got 0")
	}
}
