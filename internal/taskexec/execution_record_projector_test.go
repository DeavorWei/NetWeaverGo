package taskexec

import "testing"

func TestProjectStageCompletion(t *testing.T) {
	units := []TaskRunUnit{
		{Status: string(UnitStatusCompleted)},
		{Status: string(UnitStatusFailed)},
		{Status: string(UnitStatusCancelled)},
		{Status: string(UnitStatusPartial)},
	}

	projection := projectStageCompletion(units, false)
	if projection.Status != string(StageStatusPartial) {
		t.Fatalf("stage 状态错误: got=%s want=%s", projection.Status, StageStatusPartial)
	}
	if projection.Progress != 100 {
		t.Fatalf("stage 进度错误: got=%d want=100", projection.Progress)
	}
	if projection.CompletedUnits != 4 || projection.SuccessUnits != 1 || projection.FailedUnits != 1 || projection.CancelledUnits != 1 {
		t.Fatalf("stage 统计错误: %+v", projection)
	}
}

func TestProjectRunFinalStatus(t *testing.T) {
	stages := []TaskRunStage{
		{Status: string(StageStatusCompleted)},
		{Status: string(StageStatusPartial)},
	}
	if status := projectRunFinalStatus(stages); status != string(RunStatusPartial) {
		t.Fatalf("run 状态错误: got=%s want=%s", status, RunStatusPartial)
	}

	allCancelled := []TaskRunStage{
		{Status: string(StageStatusCancelled)},
		{Status: string(StageStatusCancelled)},
	}
	if status := projectRunFinalStatus(allCancelled); status != string(RunStatusCancelled) {
		t.Fatalf("run 取消状态错误: got=%s want=%s", status, RunStatusCancelled)
	}
}

func TestProjectRunProgressFromStages(t *testing.T) {
	stages := []TaskRunStage{
		{Progress: 25},
		{Progress: 75},
	}
	if progress := projectRunProgressFromStages(stages); progress != 50 {
		t.Fatalf("run 进度错误: got=%d want=50", progress)
	}
}
