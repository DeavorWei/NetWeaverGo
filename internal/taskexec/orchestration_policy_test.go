package taskexec

import (
	"context"
	"testing"
	"time"
)

func TestEvaluateStageDependencyPolicy(t *testing.T) {
	results := map[string]error{
		string(StageKindDeviceCollect): context.Canceled,
	}
	stage := StagePlan{Kind: string(StageKindParse), Name: "parse"}

	shouldSkip, reason := evaluateStageDependencyPolicy(string(RunKindTopology), stage, results)
	if !shouldSkip {
		t.Fatal("拓扑解析阶段应因采集失败被跳过")
	}
	if reason == "" {
		t.Fatal("跳过原因不应为空")
	}

	shouldSkip, _ = evaluateStageDependencyPolicy(string(RunKindNormal), stage, results)
	if shouldSkip {
		t.Fatal("普通任务不应命中拓扑依赖跳过策略")
	}
}

func TestEvaluateStageFailurePolicy(t *testing.T) {
	shouldAbort, reason := evaluateStageFailurePolicy(string(RunKindTopology), StagePlan{
		Kind: string(StageKindDeviceCollect),
		Name: "采集",
	}, context.DeadlineExceeded)
	if !shouldAbort {
		t.Fatal("拓扑关键阶段失败应触发中止")
	}
	if reason == "" {
		t.Fatal("中止原因不应为空")
	}

	shouldAbort, _ = evaluateStageFailurePolicy(string(RunKindTopology), StagePlan{
		Kind: string(StageKindTopologyBuild),
		Name: "构图",
	}, context.DeadlineExceeded)
	if shouldAbort {
		t.Fatal("拓扑非关键阶段失败不应触发中止")
	}
}

func TestApplyCompensationCancellation(t *testing.T) {
	db := setupTestDB(t)
	repo := NewGormRepository(db)
	now := time.Now()

	run := &TaskRun{
		ID:        "run-compensate",
		Name:      "compensate",
		RunKind:   string(RunKindNormal),
		Status:    string(RunStatusRunning),
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := repo.CreateRun(context.Background(), run); err != nil {
		t.Fatalf("create run failed: %v", err)
	}

	stage := &TaskRunStage{
		ID:         "stage-compensate",
		TaskRunID:  run.ID,
		StageKind:  string(StageKindDeviceCommand),
		StageName:  "命令执行",
		StageOrder: 1,
		Status:     string(StageStatusRunning),
		TotalUnits: 1,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := repo.CreateStage(context.Background(), stage); err != nil {
		t.Fatalf("create stage failed: %v", err)
	}

	unit := &TaskRunUnit{
		ID:             "unit-compensate",
		TaskRunID:      run.ID,
		TaskRunStageID: stage.ID,
		UnitKind:       string(UnitKindDevice),
		TargetType:     "device_ip",
		TargetKey:      "10.0.0.1",
		Status:         string(UnitStatusRunning),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := repo.CreateUnit(context.Background(), unit); err != nil {
		t.Fatalf("create unit failed: %v", err)
	}

	finishedAt := now.Add(time.Minute)
	if err := applyCompensationCancellation(repo, run.ID, finishedAt); err != nil {
		t.Fatalf("applyCompensationCancellation failed: %v", err)
	}

	gotRun, _ := repo.GetRun(context.Background(), run.ID)
	if gotRun.Status != string(RunStatusCancelled) {
		t.Fatalf("run 状态错误: got=%s want=%s", gotRun.Status, RunStatusCancelled)
	}

	gotStage, _ := repo.GetStage(context.Background(), stage.ID)
	if gotStage.Status != string(StageStatusCancelled) {
		t.Fatalf("stage 状态错误: got=%s want=%s", gotStage.Status, StageStatusCancelled)
	}

	gotUnit, _ := repo.GetUnit(context.Background(), unit.ID)
	if gotUnit.Status != string(UnitStatusCancelled) {
		t.Fatalf("unit 状态错误: got=%s want=%s", gotUnit.Status, UnitStatusCancelled)
	}
	if gotUnit.ErrorMessage != "run cancelled by compensation" {
		t.Fatalf("unit 取消原因错误: got=%s", gotUnit.ErrorMessage)
	}
}
