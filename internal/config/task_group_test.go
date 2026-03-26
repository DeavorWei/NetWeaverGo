package config

import (
	"testing"

	"github.com/NetWeaverGo/core/internal/models"
)

// TestNormalizeTaskGroup_DefaultTaskType 验证任务模板默认类型规范化
func TestNormalizeTaskGroup_DefaultTaskType(t *testing.T) {
	group := &models.TaskGroup{}
	normalizeTaskGroup(group)
	if group.TaskType != "normal" {
		t.Fatalf("TaskType = %q, want %q", group.TaskType, "normal")
	}
}

// TestNormalizeTaskGroup_KeepTaskType 验证已有任务类型不会被覆盖
func TestNormalizeTaskGroup_KeepTaskType(t *testing.T) {
	group := &models.TaskGroup{TaskType: "topology"}
	normalizeTaskGroup(group)
	if group.TaskType != "topology" {
		t.Fatalf("TaskType = %q, want %q", group.TaskType, "topology")
	}
}
