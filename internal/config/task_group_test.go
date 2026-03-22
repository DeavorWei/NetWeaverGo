package config

import (
	"testing"
)

// TestUpdateTaskGroupStatus_Validation 测试状态验证
func TestUpdateTaskGroupStatus_Validation(t *testing.T) {
	// 注意：这个测试需要数据库连接才能完整测试
	// 这里只测试状态验证逻辑

	tests := []struct {
		name      string
		status    string
		wantValid bool
	}{
		{"有效状态-pending", "pending", true},
		{"有效状态-running", "running", true},
		{"有效状态-completed", "completed", true},
		{"有效状态-failed", "failed", true},
		{"无效状态-空字符串", "", false},
		{"无效状态-未知状态", "unknown", false},
		{"无效状态-in_progress", "in_progress", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 验证状态是否在 validStatuses 中
			_, exists := validStatuses[tt.status]
			if exists != tt.wantValid {
				t.Errorf("validStatuses[%q] = %v, want %v", tt.status, exists, tt.wantValid)
			}
		})
	}
}

// TestValidStatuses 测试有效状态集合
func TestValidStatuses(t *testing.T) {
	expectedStatuses := []string{"pending", "running", "completed", "failed"}

	for _, status := range expectedStatuses {
		if !validStatuses[status] {
			t.Errorf("validStatuses[%q] should be true", status)
		}
	}

	// 确保只有这 4 个有效状态
	if len(validStatuses) != len(expectedStatuses) {
		t.Errorf("validStatuses has %d entries, want %d", len(validStatuses), len(expectedStatuses))
	}
}
