package taskexec

import (
	"testing"
	"time"
)

func TestSnapshotHubAssignsMonotonicRevision(t *testing.T) {
	hub := NewSnapshotHub(nil)

	first := &ExecutionSnapshot{
		RunID:     "run-1",
		Status:    string(RunStatusRunning),
		UpdatedAt: time.Now(),
	}
	second := &ExecutionSnapshot{
		RunID:     "run-1",
		Status:    string(RunStatusRunning),
		UpdatedAt: time.Now(),
	}

	hub.Update("run-1", first)
	if first.Revision != 1 {
		t.Fatalf("首次更新 revision 错误: got=%d want=1", first.Revision)
	}

	hub.Update("run-1", second)
	if second.Revision != 2 {
		t.Fatalf("第二次更新 revision 错误: got=%d want=2", second.Revision)
	}

	got, ok := hub.Get("run-1")
	if !ok || got == nil {
		t.Fatal("应能获取到最新快照")
	}
	if got.Revision != 2 {
		t.Fatalf("缓存中的 revision 错误: got=%d want=2", got.Revision)
	}
}

func TestSnapshotHubSupportsIncrementalProjection(t *testing.T) {
	hub := NewSnapshotHub(nil)
	now := time.Now()
	run := &TaskRun{
		ID:           "run-2",
		Name:         "incremental",
		RunKind:      string(RunKindNormal),
		Status:       string(RunStatusPending),
		CurrentStage: "device_command",
		Progress:     0,
		StartedAt:    &now,
	}

	if !hub.EnsureRun(run) {
		t.Fatal("首次 EnsureRun 应写入快照")
	}

	stage := &TaskRunStage{
		ID:         "stage-1",
		TaskRunID:  run.ID,
		StageKind:  string(StageKindDeviceCommand),
		StageName:  "命令执行",
		StageOrder: 1,
		Status:     string(StageStatusRunning),
		TotalUnits: 1,
		StartedAt:  &now,
	}
	if !hub.UpsertStage(run.ID, stage) {
		t.Fatal("首次 UpsertStage 应写入快照")
	}

	unit := &TaskRunUnit{
		ID:             "unit-1",
		TaskRunID:      run.ID,
		TaskRunStageID: stage.ID,
		UnitKind:       string(UnitKindDevice),
		TargetType:     "device_ip",
		TargetKey:      "10.0.0.1",
		Status:         string(UnitStatusRunning),
		TotalSteps:     4,
		DoneSteps:      1,
		StartedAt:      &now,
	}
	if !hub.UpsertUnit(run.ID, unit) {
		t.Fatal("首次 UpsertUnit 应写入快照")
	}

	progress := 25
	doneSteps := 3
	if !hub.ApplyRunPatch(run.ID, &RunPatch{Progress: &progress}) {
		t.Fatal("ApplyRunPatch 应命中已有快照")
	}
	if !hub.ApplyStagePatch(run.ID, stage.ID, &StagePatch{Progress: &progress}) {
		t.Fatal("ApplyStagePatch 应命中已有快照")
	}
	if !hub.ApplyUnitPatch(run.ID, unit.ID, &UnitPatch{DoneSteps: &doneSteps}) {
		t.Fatal("ApplyUnitPatch 应命中已有快照")
	}

	event := NewTaskEvent(run.ID, EventTypeUnitProgress, "命令完成").WithStage(stage.ID).WithUnit(unit.ID)
	if !hub.AppendEvent(event) {
		t.Fatal("AppendEvent 应写入事件投影")
	}

	snapshot, ok := hub.Get(run.ID)
	if !ok || snapshot == nil {
		t.Fatal("应能读取增量投影后的快照")
	}

	if snapshot.Progress != 25 {
		t.Fatalf("Run 进度错误: got=%d want=25", snapshot.Progress)
	}
	if len(snapshot.Stages) != 1 || snapshot.Stages[0].Progress != 25 {
		t.Fatalf("Stage 进度错误: %+v", snapshot.Stages)
	}
	if len(snapshot.Units) != 1 || snapshot.Units[0].DoneSteps != 3 || snapshot.Units[0].Progress != 75 {
		t.Fatalf("Unit 进度错误: %+v", snapshot.Units)
	}
	if len(snapshot.Events) != 1 || snapshot.Events[0].ID != event.ID {
		t.Fatalf("事件投影错误: %+v", snapshot.Events)
	}
}

func TestSnapshotHubGetReturnsClone(t *testing.T) {
	hub := NewSnapshotHub(nil)
	run := &TaskRun{
		ID:       "run-3",
		Name:     "clone",
		RunKind:  string(RunKindNormal),
		Status:   string(RunStatusRunning),
		Progress: 10,
	}
	hub.EnsureRun(run)

	first, ok := hub.Get(run.ID)
	if !ok || first == nil {
		t.Fatal("应能获取快照")
	}
	first.Progress = 99
	first.Events = append(first.Events, EventSnapshot{ID: "event-local"})

	second, ok := hub.Get(run.ID)
	if !ok || second == nil {
		t.Fatal("应能再次获取快照")
	}
	if second.Progress != 10 {
		t.Fatalf("Get 应返回副本，缓存不应被篡改: got=%d want=10", second.Progress)
	}
	if len(second.Events) != 0 {
		t.Fatalf("本地修改不应污染缓存事件: %+v", second.Events)
	}
}

func TestSnapshotHubBuildDeltaTracksRunSeq(t *testing.T) {
	hub := NewSnapshotHub(nil)
	now := time.Now()
	run := &TaskRun{
		ID:        "run-delta",
		Name:      "delta",
		RunKind:   string(RunKindNormal),
		Status:    string(RunStatusRunning),
		Progress:  0,
		StartedAt: &now,
	}

	if !hub.EnsureRun(run) {
		t.Fatal("首次 EnsureRun 应写入快照")
	}

	delta, ok := hub.BuildDelta(run.ID)
	if !ok || delta == nil {
		t.Fatal("应能构建初始 delta")
	}
	if delta.Seq != 1 {
		t.Fatalf("初始 delta seq 错误: got=%d want=1", delta.Seq)
	}
	if delta.Snapshot == nil || delta.Snapshot.LastRunSeq != 1 {
		t.Fatalf("初始快照 run seq 错误: %+v", delta.Snapshot)
	}

	progress := 50
	if !hub.ApplyRunPatch(run.ID, &RunPatch{Progress: &progress}) {
		t.Fatal("ApplyRunPatch 应命中已有快照")
	}

	delta, ok = hub.BuildDelta(run.ID)
	if !ok || delta == nil {
		t.Fatal("应能构建更新后的 delta")
	}
	if delta.Seq != 2 {
		t.Fatalf("更新后 delta seq 错误: got=%d want=2", delta.Seq)
	}
	if delta.Snapshot == nil || delta.Snapshot.Progress != 50 || delta.Snapshot.LastRunSeq != 2 {
		t.Fatalf("更新后快照错误: %+v", delta.Snapshot)
	}
}

func TestSnapshotHubAppendEventProjectsEventSeqAndSessionSeq(t *testing.T) {
	hub := NewSnapshotHub(nil)
	run := &TaskRun{
		ID:       "run-event-seq",
		Name:     "event-seq",
		RunKind:  string(RunKindNormal),
		Status:   string(RunStatusRunning),
		Progress: 0,
	}
	if !hub.EnsureRun(run) {
		t.Fatal("首次 EnsureRun 应写入快照")
	}

	event := NewTaskEvent(run.ID, EventTypeUnitProgress, "命令完成").
		WithStage("stage-1").
		WithUnit("unit-1").
		WithPayload("sessionSeq", 7)
	if !hub.AppendEvent(event) {
		t.Fatal("AppendEvent 应写入事件投影")
	}

	snapshot, ok := hub.Get(run.ID)
	if !ok || snapshot == nil {
		t.Fatal("应能读取事件投影后的快照")
	}
	if snapshot.LastRunSeq != 2 {
		t.Fatalf("事件追加后 run seq 错误: got=%d want=2", snapshot.LastRunSeq)
	}
	if len(snapshot.Events) != 1 {
		t.Fatalf("事件数量错误: %+v", snapshot.Events)
	}
	if snapshot.Events[0].Seq != 2 {
		t.Fatalf("事件 seq 错误: got=%d want=2", snapshot.Events[0].Seq)
	}
	if snapshot.LastSessionSeqByUnit["unit-1"] != 7 {
		t.Fatalf("unit session seq 错误: got=%d want=7", snapshot.LastSessionSeqByUnit["unit-1"])
	}
}
