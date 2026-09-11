package taskexec

import (
	"context"
	"testing"

	"github.com/NetWeaverGo/core/internal/metrics"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// 验证 RunPatch.MetricsJSON 能真正落库（persistence.UpdateRun 字段映射闭环）
func TestUpdateRun_PersistsMetricsJSON(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	if err := db.AutoMigrate(&TaskRun{}); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}

	run := &TaskRun{ID: "run-metrics", Name: "metrics-test", RunKind: "normal", Status: "running"}
	if err := db.Create(run).Error; err != nil {
		t.Fatalf("创建 Run 失败: %v", err)
	}

	repo := NewGormRepository(db)
	payload := `{"counters":{"cache.hit":3},"labels":{}}`
	if err := repo.UpdateRun(context.Background(), run.ID, &RunPatch{MetricsJSON: &payload}); err != nil {
		t.Fatalf("更新 Run 失败: %v", err)
	}

	var got TaskRun
	if err := db.First(&got, "id = ?", run.ID).Error; err != nil {
		t.Fatalf("读取 Run 失败: %v", err)
	}
	if got.MetricsJSON != payload {
		t.Fatalf("metrics_json 未落库: got=%q, want=%q（检查 UpdateRun 字段映射）", got.MetricsJSON, payload)
	}
}

func TestMatchPathLevel(t *testing.T) {
	cases := map[string]string{
		"exact:S5735":    "exact",
		"series:S5700":   "series",
		"vendor:huawei":  "vendor",
		"global:default": "global",
		"":               "unknown",
	}
	for in, want := range cases {
		if got := matchPathLevel(in); got != want {
			t.Errorf("matchPathLevel(%q) = %q, want %q", in, got, want)
		}
	}
}

// 验证指标按 RunID 分桶且 Release 后释放（多任务并发不串扰）
func TestRunMetrics_IsolatedAndReleased(t *testing.T) {
	metrics.Default.Inc("run-m1", "cache.hit", 2)
	metrics.Default.Inc("run-m2", "cache.hit", 9)

	if got := metrics.Default.Snapshot("run-m1").Counters["cache.hit"]; got != 2 {
		t.Fatalf("run-m1 应为 2，实际 %d", got)
	}
	if got := metrics.Default.Snapshot("run-m2").Counters["cache.hit"]; got != 9 {
		t.Fatalf("run-m2 应为 9，实际 %d", got)
	}

	metrics.Default.Release("run-m1")
	metrics.Default.Release("run-m2")
	if len(metrics.Default.Snapshot("run-m1").Counters) != 0 {
		t.Fatal("Release 后快照应为空")
	}
}
