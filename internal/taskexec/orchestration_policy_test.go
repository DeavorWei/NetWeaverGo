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
	if gotRun.FinishedAt == nil || !gotRun.FinishedAt.Equal(finishedAt) {
		t.Fatalf("run finishedAt 未写入补偿时间: got=%v want=%v", gotRun.FinishedAt, finishedAt)
	}

	gotStage, _ := repo.GetStage(context.Background(), stage.ID)
	if gotStage.Status != string(StageStatusCancelled) {
		t.Fatalf("stage 状态错误: got=%s want=%s", gotStage.Status, StageStatusCancelled)
	}
	if gotStage.FinishedAt == nil || !gotStage.FinishedAt.Equal(finishedAt) {
		t.Fatalf("stage finishedAt 未写入补偿时间: got=%v want=%v", gotStage.FinishedAt, finishedAt)
	}

	gotUnit, _ := repo.GetUnit(context.Background(), unit.ID)
	if gotUnit.Status != string(UnitStatusCancelled) {
		t.Fatalf("unit 状态错误: got=%s want=%s", gotUnit.Status, UnitStatusCancelled)
	}
	if gotUnit.ErrorMessage != "run cancelled by compensation" {
		t.Fatalf("unit 取消原因错误: got=%s", gotUnit.ErrorMessage)
	}
	if gotUnit.FinishedAt == nil || !gotUnit.FinishedAt.Equal(finishedAt) {
		t.Fatalf("unit finishedAt 未写入补偿时间: got=%v want=%v", gotUnit.FinishedAt, finishedAt)
	}
}

func TestRecoverInterruptedRuns(t *testing.T) {
	db := setupTestDB(t)
	repo := NewGormRepository(db)
	now := time.Now()

	seedRun := func(runID string, status RunStatus) {
		run := &TaskRun{
			ID:        runID,
			Name:      runID,
			RunKind:   string(RunKindNormal),
			Status:    string(status),
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := repo.CreateRun(context.Background(), run); err != nil {
			t.Fatalf("create run failed: %v", err)
		}
		stage := &TaskRunStage{
			ID:         "stage-" + runID,
			TaskRunID:  runID,
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
			ID:             "unit-" + runID,
			TaskRunID:      runID,
			TaskRunStageID: stage.ID,
			UnitKind:       string(UnitKindDevice),
			TargetType:     "device_ip",
			TargetKey:      runID,
			Status:         string(UnitStatusRunning),
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		if err := repo.CreateUnit(context.Background(), unit); err != nil {
			t.Fatalf("create unit failed: %v", err)
		}
	}

	seedRun("run-pending", RunStatusPending)
	seedRun("run-running", RunStatusRunning)
	seedRun("run-completed", RunStatusCompleted)

	finishedAt := now.Add(2 * time.Minute)
	recovered, err := recoverInterruptedRuns(repo, finishedAt)
	if err != nil {
		t.Fatalf("recoverInterruptedRuns failed: %v", err)
	}
	if len(recovered) != 2 {
		t.Fatalf("恢复运行数量错误: got=%d want=2", len(recovered))
	}
	gotRecovered := map[string]struct{}{}
	for _, runID := range recovered {
		gotRecovered[runID] = struct{}{}
	}
	if _, ok := gotRecovered["run-pending"]; !ok {
		t.Fatalf("缺少 pending 运行恢复记录: %v", recovered)
	}
	if _, ok := gotRecovered["run-running"]; !ok {
		t.Fatalf("缺少 running 运行恢复记录: %v", recovered)
	}

	assertCancelled := func(runID string) {
		run, err := repo.GetRun(context.Background(), runID)
		if err != nil {
			t.Fatalf("get run failed: %v", err)
		}
		if run.Status != string(RunStatusCancelled) {
			t.Fatalf("run 状态未恢复为 cancelled: run=%s status=%s", runID, run.Status)
		}
		if run.FinishedAt == nil || !run.FinishedAt.Equal(finishedAt) {
			t.Fatalf("run finishedAt 未写入: run=%s got=%v want=%v", runID, run.FinishedAt, finishedAt)
		}
	}
	assertCancelled("run-pending")
	assertCancelled("run-running")

	completed, err := repo.GetRun(context.Background(), "run-completed")
	if err != nil {
		t.Fatalf("get completed run failed: %v", err)
	}
	if completed.Status != string(RunStatusCompleted) {
		t.Fatalf("终态运行不应被恢复改写: got=%s", completed.Status)
	}
}
