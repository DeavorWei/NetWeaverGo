package taskexec

import (
	"os"
	"testing"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/models"
)

// 阶段一 1.1：三阶段纯判定路径必须补齐产物登记（raw_output + 报表），
// 使任务产物可见性与单阶段路径 executeInspectionUnit 一致。
func TestRegisterCheckOnlyArtifacts(t *testing.T) {
	db := setupTestDB(t)
	useTempStorageRoot(t)

	exec := &InspectionCheckExecutor{
		db:          db,
		pathManager: config.GetPathManager(),
	}

	const runID = "run-artifacts-test"
	t.Cleanup(func() { ReleaseRunData(runID) })

	holder := GetRunData(runID)
	holder.SetCommandEcho("10.0.0.1", "display version", "VRP V200R019C00")
	holder.SetCommandEcho("10.0.0.1", "display cpu-usage", "CPU utilization 10%")

	items := []models.InspectionItem{
		{CommandKey: "display version"},
		{CommandKey: "display cpu-usage"},
		{CommandKey: "display version"}, // 重复命令应被去重
	}
	results := []models.InspectionResult{
		{RunID: runID, DeviceIP: "10.0.0.1", ItemCode: "GEN_DEVICE_UPTIME", Status: "TEST_PASS", Severity: "minor"},
	}

	exec.registerCheckOnlyArtifacts(runID, "stage-check", "unit-1", "10.0.0.1", items, holder, results)

	var rawCount, reportCount int64
	if err := db.Model(&TaskArtifact{}).Where("artifact_type = ?", string(ArtifactTypeRawOutput)).Count(&rawCount).Error; err != nil {
		t.Fatalf("统计 raw_output 产物失败: %v", err)
	}
	if err := db.Model(&TaskArtifact{}).Where("artifact_type = ?", string(ArtifactTypeInspectionReport)).Count(&reportCount).Error; err != nil {
		t.Fatalf("统计 inspection_report 产物失败: %v", err)
	}

	if rawCount != 2 {
		t.Errorf("raw_output 产物数 = %d, want 2（命令去重后）", rawCount)
	}
	if reportCount != 2 {
		t.Errorf("inspection_report 产物数 = %d, want 2（CSV + JSON）", reportCount)
	}

	// 产物记录指向的文件必须真实落盘
	var arts []TaskArtifact
	if err := db.Find(&arts).Error; err != nil {
		t.Fatalf("读取产物失败: %v", err)
	}
	for _, a := range arts {
		if a.FilePath == "" {
			t.Errorf("产物 %s 的 FilePath 为空", a.ArtifactKey)
			continue
		}
		if _, err := os.Stat(a.FilePath); err != nil {
			t.Errorf("产物文件不存在: %s (%v)", a.FilePath, err)
		}
	}
}

// 阶段一 1.2：内存快照满载后应置位 full（只置位一次）并丢弃后续写入。
func TestRunDataStore_MarkFullDropsSubsequentWrites(t *testing.T) {
	store := newRunDataStore("run-full-test")

	// 通过把 size 调到临界值，触发 SetCommandEcho 的超限分支
	store.size = maxRunDataBytes - 1
	store.SetCommandEcho("10.0.0.1", "display version", "0123456789")
	if !store.full {
		t.Fatal("超过容量上限后 full 应为 true")
	}
	if _, ok := store.GetCommandEcho("10.0.0.1", "display version"); ok {
		t.Error("满载后不应再写入回显数据")
	}

	// 重复触发不应改变状态（幂等），且不再重复写日志
	store.markFullLocked("命令回显")
	if !store.full {
		t.Fatal("full 状态不应被重置")
	}
}
