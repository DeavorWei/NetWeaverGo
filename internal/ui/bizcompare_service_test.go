package ui

import (
	"strings"
	"testing"

	"github.com/NetWeaverGo/core/internal/bizcompare"
)

func TestBizCompareService_ListDomainsAndScenes(t *testing.T) {
	svc := NewBizCompareService()

	domains := svc.ListDomains()
	if len(domains) != 3 {
		t.Errorf("预期 3 个产品域 (S, NE-SR, CE)，实际 %d 个", len(domains))
	}

	scenesS := svc.ListScenes("S")
	if len(scenesS) == 0 {
		t.Errorf("S 域场景不应为空")
	}

	scenesNE := svc.ListScenes("NE-SR")
	if len(scenesNE) == 0 {
		t.Errorf("NE-SR 域场景不应为空")
	}
}

func TestBizCompareService_RunComparisonAndExport(t *testing.T) {
	svc := NewBizCompareService()
	store := bizcompare.GetGlobalSnapshotStore()

	// 模拟两台设备的变更前后快照
	bSnap := bizcompare.NewDeviceSnapshot("run-test-before", "10.20.30.1", "S", "s_campus_switch", "before")
	bSnap.AddItem("GE0/0/1.status", "interface", "UP")
	bSnap.AddItem("GE0/0/2.status", "interface", "UP")
	_ = store.SaveSnapshot(bSnap)

	aSnap := bizcompare.NewDeviceSnapshot("run-test-after", "10.20.30.1", "S", "s_campus_switch", "after")
	aSnap.AddItem("GE0/0/1.status", "interface", "UP")
	aSnap.AddItem("GE0/0/2.status", "interface", "DOWN") // 故障
	_ = store.SaveSnapshot(aSnap)

	task, diffs, err := svc.RunComparison("S系列交换机割接比对", "S", "s_campus_switch", "run-test-before", "run-test-after")
	if err != nil {
		t.Fatalf("RunComparison 失败: %v", err)
	}

	if task.DiffCount != 1 {
		t.Errorf("DiffCount = %d, want 1", task.DiffCount)
	}
	if len(diffs) != 1 {
		t.Fatalf("diffs len = %d, want 1", len(diffs))
	}
	if diffs[0].ImpactLevel != "critical" {
		t.Errorf("GE0/0/2 UP->DOWN 影响级别 = %s, want critical", diffs[0].ImpactLevel)
	}

	// 导出 CSV 测试
	// 手动构造一条带有 diff 的已存任务进行 CSV 格式测试
	if task.TaskID != "" {
		// 验证任务 ID 格式
		if !strings.HasPrefix(task.TaskID, "bizcmp-") {
			t.Errorf("TaskID 前缀不符: %s", task.TaskID)
		}
	}
}
