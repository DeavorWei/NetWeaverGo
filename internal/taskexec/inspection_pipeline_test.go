package taskexec

import (
	"context"
	"fmt"
	"testing"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/models"
)

// withPipelineMode 临时切换巡检编排模式，返回恢复函数
func withPipelineMode(t *testing.T, mode string) {
	t.Helper()
	original := *config.GetGlobalSettings()
	t.Cleanup(func() { config.SetGlobalSettings(original) })
	st := original
	st.InspectionPipelineMode = mode
	config.SetGlobalSettings(st)
}

func compileInspectionPlanForTest(t *testing.T) *ExecutionPlan {
	t.Helper()
	c := NewInspectionTaskCompiler(nil, nil)
	def := &TaskDefinition{
		ID:     "inspection-test",
		Name:   "巡检测试",
		Kind:   string(RunKindInspection),
		Config: []byte(`{"deviceIps":["10.0.0.1","10.0.0.2"],"templateId":"tpl-huawei-general","concurrency":2,"timeoutSec":60}`),
	}
	plan, err := c.Compile(context.Background(), def)
	if err != nil {
		t.Fatalf("编译巡检计划失败: %v", err)
	}
	return plan
}

// 验证默认编排为单阶段（灰度最保守档）
func TestInspectionCompiler_SingleStageByDefault(t *testing.T) {
	withPipelineMode(t, "single")
	plan := compileInspectionPlanForTest(t)

	if len(plan.Stages) != 1 {
		t.Fatalf("单阶段模式应产出 1 个 Stage，实际 %d", len(plan.Stages))
	}
	if plan.Stages[0].Kind != string(StageKindInspectionCheck) {
		t.Fatalf("单阶段 Stage 类型应为 inspection_check，实际 %s", plan.Stages[0].Kind)
	}
	if len(plan.Stages[0].Units) != 2 {
		t.Fatalf("应产出 2 个设备 Unit，实际 %d", len(plan.Stages[0].Units))
	}
}

// 验证 three_stage 模式产出采集/解析/判定三阶段且顺序正确
func TestInspectionCompiler_ThreeStagePipeline(t *testing.T) {
	withPipelineMode(t, "three_stage")
	plan := compileInspectionPlanForTest(t)

	if len(plan.Stages) != 3 {
		t.Fatalf("三阶段模式应产出 3 个 Stage，实际 %d", len(plan.Stages))
	}
	wantKinds := []string{
		string(StageKindInspectionCollect),
		string(StageKindInspectionParse),
		string(StageKindInspectionCheck),
	}
	for i, want := range wantKinds {
		if plan.Stages[i].Kind != want {
			t.Errorf("Stage[%d].Kind = %s, want %s", i, plan.Stages[i].Kind, want)
		}
		if plan.Stages[i].Order != i+1 {
			t.Errorf("Stage[%d].Order = %d, want %d", i, plan.Stages[i].Order, i+1)
		}
	}

	// 采集阶段：steps 为去重命令；判定阶段：steps 为检查项
	collect := plan.Stages[0]
	if len(collect.Units) != 2 {
		t.Fatalf("采集阶段应含 2 个设备 Unit，实际 %d", len(collect.Units))
	}
	if len(collect.Units[0].Steps) == 0 {
		t.Fatal("采集阶段 Unit 不应无命令步骤")
	}
	check := plan.Stages[2]
	if len(check.Units[0].Steps) < len(collect.Units[0].Steps) {
		t.Errorf("判定步骤数(%d) 不应少于去重命令数(%d)", len(check.Units[0].Steps), len(collect.Units[0].Steps))
	}
}

// 验证内存快照的读写与设备级判定
func TestRunDataHolder_Basic(t *testing.T) {
	runID := "run-holder-test"
	t.Cleanup(func() { ReleaseRunData(runID) })

	holder := GetRunData(runID)
	if holder.HasAnyData() {
		t.Fatal("初始状态不应有数据")
	}

	holder.SetCommandEcho("10.0.0.1", "display version", "VRP V200R019")
	holder.SetParsedRows("10.0.0.1", "display version", []map[string]string{{"version": "V200R019"}})

	echo, ok := holder.GetCommandEcho("10.0.0.1", "display version")
	if !ok || echo != "VRP V200R019" {
		t.Fatalf("读取回显失败: ok=%v, echo=%q", ok, echo)
	}
	rows, ok := holder.GetParsedRows("10.0.0.1", "display version")
	if !ok || len(rows) != 1 || rows[0]["version"] != "V200R019" {
		t.Fatalf("读取解析结果失败: ok=%v, rows=%v", ok, rows)
	}

	if !holder.HasDeviceEchoes("10.0.0.1") {
		t.Error("10.0.0.1 应有回显")
	}
	if holder.HasDeviceEchoes("10.0.0.2") {
		t.Error("10.0.0.2 不应有回显（设备级隔离）")
	}
	if !holder.HasAnyData() {
		t.Error("HasAnyData 应为 true")
	}

	// 同 runID 再次获取应共享同一份数据
	if again := GetRunData(runID); !again.HasDeviceEchoes("10.0.0.1") {
		t.Error("同一 runID 应共享数据快照")
	}
}

// 验证巡检阶段依赖策略：仅前置阶段全量失败才跳过
func TestEvaluateStageDependencyPolicy_InspectionOnlyOnTotalFailure(t *testing.T) {
	// 前置阶段返回错误（全量失败）→ 跳过
	if skip, _ := evaluateStageDependencyPolicy(string(RunKindInspection),
		StagePlan{Kind: string(StageKindInspectionParse)},
		map[string]error{string(StageKindInspectionCollect): fmt.Errorf("all devices failed")}); !skip {
		t.Error("采集阶段全量失败时应跳过解析阶段")
	}

	// 前置阶段无错误 → 不跳过（设备级失败由 Unit 隔离）
	if skip, _ := evaluateStageDependencyPolicy(string(RunKindInspection),
		StagePlan{Kind: string(StageKindInspectionParse)},
		map[string]error{string(StageKindInspectionCollect): nil}); skip {
		t.Error("采集阶段存在成功设备时不应跳过解析阶段")
	}

	// 判定阶段依赖解析阶段
	if skip, _ := evaluateStageDependencyPolicy(string(RunKindInspection),
		StagePlan{Kind: string(StageKindInspectionCheck)},
		map[string]error{string(StageKindInspectionParse): fmt.Errorf("all failed")}); !skip {
		t.Error("解析阶段全量失败时应跳过判定阶段")
	}
}

// 验证巡检失败中止策略不触发（沿用既有：非 topology 不中止）
func TestEvaluateStageFailurePolicy_InspectionNotAborted(t *testing.T) {
	if abort, _ := evaluateStageFailurePolicy(string(RunKindInspection),
		StagePlan{Kind: string(StageKindInspectionCollect)}, fmt.Errorf("boom")); abort {
		t.Error("巡检任务不应因阶段错误触发全局中止")
	}
}

// 验证分组树可持久化（Groups 列序列化/反序列化）
func TestInspectionTemplateGroupsPersistence(t *testing.T) {
	db := setupTestDB(t)
	if err := db.AutoMigrate(&models.InspectionTemplate{}, &models.InspectionItemText{}); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}

	tpl := models.InspectionTemplate{
		ID:   "tpl-group",
		Name: "分组模板",
		Groups: models.InspectionGroups{
			{Code: "sys", Name: "系统", ItemCodes: []string{"A", "B"}},
			{Code: "env", Name: "环境", Children: []models.InspectionGroup{{Code: "temp", Name: "温度"}}},
		},
	}
	if err := db.Create(&tpl).Error; err != nil {
		t.Fatalf("写入模板失败: %v", err)
	}

	var got models.InspectionTemplate
	if err := db.First(&got, "id = ?", "tpl-group").Error; err != nil {
		t.Fatalf("读取模板失败: %v", err)
	}
	if len(got.Groups) != 2 || got.Groups[0].Code != "sys" || len(got.Groups[1].Children) != 1 {
		t.Fatalf("分组树未正确反序列化: %+v", got.Groups)
	}
}
