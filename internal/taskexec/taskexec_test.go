package taskexec

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/report"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// setupTestDB 创建测试数据库 (使用glebarez/sqlite纯Go驱动)
func setupTestDB(t *testing.T) *gorm.DB {
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=private", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	err = AutoMigrate(db)
	require.NoError(t, err)

	return db
}

func useTempStorageRoot(t *testing.T) {
	t.Helper()
	pm := config.GetPathManager()
	originalRoot := pm.GetStorageRoot()
	_, originalStatErr := os.Stat(originalRoot)
	originalExists := originalStatErr == nil
	tempRoot := t.TempDir()
	require.NoError(t, pm.UpdateStorageRoot(tempRoot))
	t.Cleanup(func() {
		if err := pm.UpdateStorageRoot(originalRoot); err != nil {
			t.Fatalf("恢复 storage root 失败: %v", err)
		}
		if !originalExists {
			_ = os.RemoveAll(originalRoot)
		}
	})
}

// TestAutoMigrate 测试数据库迁移
func TestAutoMigrate(t *testing.T) {
	db := setupTestDB(t)
	assert.NotNil(t, db)

	// 验证表存在
	var count int64
	db.Table("task_runs").Count(&count)
	db.Table("task_run_stages").Count(&count)
	db.Table("task_run_units").Count(&count)
	db.Table("task_run_events").Count(&count)
	db.Table("task_artifacts").Count(&count)
	db.Table("task_definitions").Count(&count)
}

// TestCreateRun 测试创建运行实例
func TestCreateRun(t *testing.T) {
	db := setupTestDB(t)
	repo := NewGormRepository(db)

	run := &TaskRun{
		ID:               "test-run-1",
		TaskDefinitionID: "def-1",
		Name:             "测试任务",
		RunKind:          string(RunKindNormal),
		Status:           string(RunStatusPending),
		Progress:         0,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	err := repo.CreateRun(context.Background(), run)
	require.NoError(t, err)

	// 查询验证
	found, err := repo.GetRun(context.Background(), run.ID)
	require.NoError(t, err)
	assert.Equal(t, run.Name, found.Name)
	assert.Equal(t, run.Status, found.Status)
}

// TestUpdateRun 测试更新运行状态
func TestUpdateRun(t *testing.T) {
	db := setupTestDB(t)
	repo := NewGormRepository(db)

	run := &TaskRun{
		ID:        "test-run-2",
		Name:      "测试任务",
		Status:    string(RunStatusPending),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	err := repo.CreateRun(context.Background(), run)
	require.NoError(t, err)

	// 更新状态
	newStatus := string(RunStatusRunning)
	now := time.Now()
	lastRunSeq := uint64(12)
	err = repo.UpdateRun(context.Background(), run.ID, &RunPatch{
		Status:     &newStatus,
		Progress:   intPtr(50),
		LastRunSeq: &lastRunSeq,
		StartedAt:  &now,
	})
	require.NoError(t, err)

	// 验证更新
	found, err := repo.GetRun(context.Background(), run.ID)
	require.NoError(t, err)
	assert.Equal(t, string(RunStatusRunning), found.Status)
	assert.Equal(t, 50, found.Progress)
	assert.Equal(t, lastRunSeq, found.LastRunSeq)
	assert.NotNil(t, found.StartedAt)
}

// TestCreateStage 测试创建Stage
func TestCreateStage(t *testing.T) {
	db := setupTestDB(t)
	repo := NewGormRepository(db)

	stage := &TaskRunStage{
		ID:         "stage-1",
		TaskRunID:  "run-1",
		StageKind:  string(StageKindDeviceCommand),
		StageName:  "命令执行",
		StageOrder: 1,
		Status:     string(StageStatusPending),
		TotalUnits: 5,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	err := repo.CreateStage(context.Background(), stage)
	require.NoError(t, err)

	// 查询验证
	found, err := repo.GetStage(context.Background(), stage.ID)
	require.NoError(t, err)
	assert.Equal(t, stage.StageName, found.StageName)
	assert.Equal(t, 5, found.TotalUnits)
}

// TestCreateUnit 测试创建Unit
func TestCreateUnit(t *testing.T) {
	db := setupTestDB(t)
	repo := NewGormRepository(db)

	unit := &TaskRunUnit{
		ID:             "unit-1",
		TaskRunID:      "run-1",
		TaskRunStageID: "stage-1",
		UnitKind:       string(UnitKindDevice),
		TargetType:     "device_ip",
		TargetKey:      "192.168.1.1",
		Status:         string(UnitStatusPending),
		TotalSteps:     10,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	err := repo.CreateUnit(context.Background(), unit)
	require.NoError(t, err)

	// 查询验证
	found, err := repo.GetUnit(context.Background(), unit.ID)
	require.NoError(t, err)
	assert.Equal(t, unit.TargetKey, found.TargetKey)
	assert.Equal(t, 10, found.TotalSteps)
}

// TestEventBus 测试事件总线
func TestEventBus(t *testing.T) {
	bus := NewEventBus(100)
	bus.Start()
	defer bus.Stop()

	received := make(chan *TaskEvent, 1)
	bus.Subscribe(func(event *TaskEvent) {
		received <- event
	})

	// 发送事件
	event := NewTaskEvent("run-1", EventTypeRunStarted, "任务开始")
	bus.EmitSync(event)

	// 验证接收
	select {
	case e := <-received:
		assert.Equal(t, EventTypeRunStarted, e.Type)
		assert.Equal(t, "任务开始", e.Message)
	case <-time.After(time.Second):
		t.Fatal("未收到事件")
	}
}

func TestEventBusEmitMaintainsHandlerAndEventOrder(t *testing.T) {
	bus := NewEventBus(10)
	bus.Start()
	defer bus.Stop()

	orderCh := make(chan string, 4)
	bus.Subscribe(func(event *TaskEvent) {
		orderCh <- "h1:" + event.Message
		time.Sleep(10 * time.Millisecond)
	})
	bus.Subscribe(func(event *TaskEvent) {
		orderCh <- "h2:" + event.Message
	})

	bus.Emit(NewTaskEvent("run-1", EventTypeRunProgress, "A"))
	bus.Emit(NewTaskEvent("run-1", EventTypeRunProgress, "B"))

	got := make([]string, 0, 4)
	timeout := time.After(2 * time.Second)
	for len(got) < 4 {
		select {
		case msg := <-orderCh:
			got = append(got, msg)
		case <-timeout:
			t.Fatalf("未在超时时间内收到完整顺序，当前=%v", got)
		}
	}

	assert.Equal(t, []string{"h1:A", "h2:A", "h1:B", "h2:B"}, got)
}

func TestTaskEventRepositoryProjector(t *testing.T) {
	db := setupTestDB(t)
	repo := NewGormRepository(db)
	projector := NewTaskEventRepositoryProjector(repo)

	run := &TaskRun{
		ID:        "run-1",
		Name:      "投影测试",
		RunKind:   string(RunKindNormal),
		Status:    string(RunStatusRunning),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	require.NoError(t, repo.CreateRun(context.Background(), run))

	event := NewTaskEvent("run-1", EventTypeRunProgress, "任务推进").
		WithStage("stage-1").
		WithUnit("unit-1").
		WithPayload("progress", 50).
		WithPayload("runSeq", 7).
		WithPayload("sessionSeq", 3)

	projector(event)

	events, err := repo.GetEventsByRun(context.Background(), "run-1", 10)
	require.NoError(t, err)
	if len(events) != 1 {
		t.Fatalf("事件持久化数量错误: got=%d want=1", len(events))
	}
	assert.Equal(t, string(EventTypeRunProgress), events[0].EventType)
	assert.Equal(t, "stage-1", events[0].StageID)
	assert.Equal(t, "unit-1", events[0].UnitID)
	assert.Equal(t, uint64(7), events[0].RunSeq)
	assert.Equal(t, uint64(3), events[0].SessionSeq)
	assert.Contains(t, events[0].PayloadJSON, "\"progress\":50")

	updatedRun, err := repo.GetRun(context.Background(), "run-1")
	require.NoError(t, err)
	assert.Equal(t, uint64(7), updatedRun.LastRunSeq)
}

// TestSnapshotBuilder 测试快照构建
func TestSnapshotBuilder(t *testing.T) {
	builder := NewSnapshotBuilder()

	run := &TaskRun{
		ID:       "run-1",
		Name:     "测试任务",
		RunKind:  string(RunKindNormal),
		Status:   string(RunStatusRunning),
		Progress: 50,
	}

	stages := []TaskRunStage{
		{
			ID:             "stage-1",
			StageKind:      string(StageKindDeviceCommand),
			StageName:      "命令执行",
			StageOrder:     1,
			Status:         string(StageStatusRunning),
			Progress:       60,
			TotalUnits:     5,
			CompletedUnits: 3,
		},
	}

	units := []TaskRunUnit{
		{
			ID:             "unit-1",
			TaskRunStageID: "stage-1",
			TargetKey:      "192.168.1.1",
			Status:         string(UnitStatusRunning),
			TotalSteps:     10,
			DoneSteps:      6,
		},
	}

	events := []TaskRunEvent{
		{
			ID:        "event-1",
			TaskRunID: "run-1",
			EventType: string(EventTypeRunStarted),
			Message:   "任务开始",
			CreatedAt: time.Now(),
		},
	}

	snapshot := builder.Build(run, stages, units, events)

	assert.Equal(t, run.ID, snapshot.RunID)
	assert.Equal(t, 1, len(snapshot.Stages))
	assert.Equal(t, 1, len(snapshot.Units))
	assert.Equal(t, 1, len(snapshot.Events))
	assert.Equal(t, 60, snapshot.Units[0].Progress) // 6/10 * 100
}

// TestStatusTransitions 测试状态转换
func TestStatusTransitions(t *testing.T) {
	// 测试终态判断
	assert.True(t, RunStatusCompleted.IsTerminal())
	assert.True(t, RunStatusFailed.IsTerminal())
	assert.True(t, RunStatusCancelled.IsTerminal())
	assert.True(t, RunStatusPartial.IsTerminal())
	assert.False(t, RunStatusPending.IsTerminal())
	assert.False(t, RunStatusRunning.IsTerminal())

	// Stage和Unit状态
	assert.True(t, StageStatusCompleted.IsTerminal())
	assert.True(t, UnitStatusCompleted.IsTerminal())
}

// TestExecutionPlan 测试执行计划结构
func TestExecutionPlan(t *testing.T) {
	plan := &ExecutionPlan{
		RunKind: string(RunKindNormal),
		Name:    "测试计划",
		Stages: []StagePlan{
			{
				ID:          "stage-1",
				Kind:        string(StageKindDeviceCommand),
				Name:        "命令执行",
				Order:       1,
				Concurrency: 5,
				Units: []UnitPlan{
					{
						ID:     "unit-1",
						Kind:   string(UnitKindDevice),
						Target: TargetRef{Type: "device_ip", Key: "192.168.1.1"},
						Steps: []StepPlan{
							{
								ID:         "step-1",
								Kind:       "command",
								Name:       "显示版本",
								Command:    "display version",
								CommandKey: "version",
							},
						},
					},
				},
			},
		},
	}

	assert.Equal(t, string(RunKindNormal), plan.RunKind)
	assert.Equal(t, 1, len(plan.Stages))
	assert.Equal(t, 1, len(plan.Stages[0].Units))
	assert.Equal(t, 1, len(plan.Stages[0].Units[0].Steps))
}

// TestCompilerRegistry 测试编译器注册表
func TestCompilerRegistry(t *testing.T) {
	registry := NewCompilerRegistry()

	// 创建一个模拟编译器
	mockCompiler := &mockPlanCompiler{kind: string(RunKindNormal)}
	registry.Register(string(RunKindNormal), mockCompiler)

	// 测试获取
	compiler, ok := registry.Get(string(RunKindNormal))
	assert.True(t, ok)
	assert.NotNil(t, compiler)

	// 测试不存在的类型
	_, ok = registry.Get("unknown")
	assert.False(t, ok)
}

// TestExecutorRegistry 测试执行器注册表
func TestExecutorRegistry(t *testing.T) {
	registry := NewExecutorRegistry()

	// 创建一个模拟执行器
	mockExecutor := &mockStageExecutor{kind: string(StageKindDeviceCommand)}
	registry.Register(mockExecutor)

	// 测试获取
	executor, ok := registry.Get(string(StageKindDeviceCommand))
	assert.True(t, ok)
	assert.NotNil(t, executor)

	// 测试不存在的类型
	_, ok = registry.Get("unknown")
	assert.False(t, ok)
}

func TestRuntimeManagerGetSnapshotDelta(t *testing.T) {
	db := setupTestDB(t)
	repo := NewGormRepository(db)
	eventBus := NewEventBus(100)
	snapshotHub := NewSnapshotHub(eventBus)
	runtime := NewRuntimeManager(repo, eventBus, snapshotHub)

	run := &TaskRun{
		ID:        "delta-run-1",
		Name:      "delta-test",
		RunKind:   string(RunKindNormal),
		Status:    string(RunStatusRunning),
		Progress:  0,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	require.NoError(t, repo.CreateRun(context.Background(), run))
	snapshotHub.EnsureRun(run)

	delta, err := runtime.GetSnapshotDelta(run.ID)
	require.NoError(t, err)
	require.NotNil(t, delta)
	require.NotNil(t, delta.Snapshot)
	assert.Equal(t, run.ID, delta.RunID)
	assert.Equal(t, delta.Seq, delta.Snapshot.LastRunSeq)

	progress := 35
	require.True(t, snapshotHub.ApplyRunPatch(run.ID, &RunPatch{Progress: &progress}))

	delta, err = runtime.GetSnapshotDelta(run.ID)
	require.NoError(t, err)
	require.NotNil(t, delta)
	assert.Equal(t, uint64(2), delta.Seq)
	assert.Equal(t, uint64(1), delta.BaseSeq)
	require.Nil(t, delta.Snapshot)
	require.Len(t, delta.Ops, 1)
	require.Equal(t, SnapshotDeltaOpRunPatch, delta.Ops[0].Type)
	require.NotNil(t, delta.Ops[0].Progress)
	assert.Equal(t, 35, *delta.Ops[0].Progress)
}

func TestRuntimeManagerGetSnapshotDeltaEnrichesUnitUpsertLogs(t *testing.T) {
	useTempStorageRoot(t)

	db := setupTestDB(t)
	repo := NewGormRepository(db)
	eventBus := NewEventBus(100)
	snapshotHub := NewSnapshotHub(eventBus)
	runtime := NewRuntimeManager(repo, eventBus, snapshotHub)

	run := &TaskRun{
		ID:        "delta-run-logs-1",
		Name:      "delta-log-enrich-test",
		RunKind:   string(RunKindNormal),
		Status:    string(RunStatusRunning),
		Progress:  0,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	require.NoError(t, repo.CreateRun(context.Background(), run))
	require.True(t, snapshotHub.EnsureRun(run))

	unit := &TaskRunUnit{
		ID:             "delta-unit-logs-1",
		TaskRunID:      run.ID,
		TaskRunStageID: "stage-logs-1",
		UnitKind:       string(UnitKindDevice),
		TargetType:     "device_ip",
		TargetKey:      "10.0.0.9",
		Status:         string(UnitStatusRunning),
		TotalSteps:     3,
		DoneSteps:      1,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	require.True(t, snapshotHub.UpsertUnit(run.ID, unit))

	store, err := report.NewExecutionLogStore(run.Name, time.Now())
	require.NoError(t, err)
	defer store.Close()

	session, err := store.EnsureDevice(unit.TargetKey, false)
	require.NoError(t, err)
	require.NoError(t, session.WriteSummary("增量日志补齐验证"))

	runtime.mu.Lock()
	runtime.logStores[run.ID] = store
	runtime.mu.Unlock()

	delta, err := runtime.GetSnapshotDelta(run.ID)
	require.NoError(t, err)
	require.NotNil(t, delta)
	require.Nil(t, delta.Snapshot)
	require.NotEmpty(t, delta.Ops)

	foundUnitUpsert := false
	for _, op := range delta.Ops {
		if op.Type != SnapshotDeltaOpUnitUpsert || op.Unit == nil {
			continue
		}
		if op.Unit.ID != unit.ID {
			continue
		}
		foundUnitUpsert = true
		assert.Equal(t, 1, op.Unit.LogCount)
		require.Len(t, op.Unit.Logs, 1)
		assert.Contains(t, op.Unit.Logs[0], "增量日志补齐验证")
		assert.NotEmpty(t, op.Unit.SummaryLogPath)
		break
	}
	assert.True(t, foundUnitUpsert)
}

func TestTaskExecutionServiceGetSnapshotDelta(t *testing.T) {
	db := setupTestDB(t)
	service := NewTaskExecutionService(db)
	run := &TaskRun{
		ID:        "delta-run-2",
		Name:      "service-delta-test",
		RunKind:   string(RunKindNormal),
		Status:    string(RunStatusRunning),
		Progress:  0,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	require.NoError(t, service.repo.CreateRun(context.Background(), run))
	require.True(t, service.snapshot.EnsureRun(run))

	delta, err := service.GetSnapshotDelta(run.ID)
	require.NoError(t, err)
	require.NotNil(t, delta)
	require.NotNil(t, delta.Snapshot)
	assert.Equal(t, run.ID, delta.RunID)
	assert.Equal(t, delta.Seq, delta.Snapshot.LastRunSeq)
}

func TestRuntimeManagerGetSnapshotWithoutHubDataFails(t *testing.T) {
	db := setupTestDB(t)
	repo := NewGormRepository(db)
	eventBus := NewEventBus(10)
	snapshotHub := NewSnapshotHub(eventBus)
	runtime := NewRuntimeManager(repo, eventBus, snapshotHub)

	run := &TaskRun{
		ID:        "snapshot-miss-run-1",
		Name:      "snapshot-miss-test",
		RunKind:   string(RunKindNormal),
		Status:    string(RunStatusRunning),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	require.NoError(t, repo.CreateRun(context.Background(), run))

	snapshot, err := runtime.GetSnapshot(run.ID)
	require.Error(t, err)
	assert.Nil(t, snapshot)
	assert.Contains(t, err.Error(), "snapshot not found")
}

func TestRuntimeManagerProjectCancellationToStages(t *testing.T) {
	db := setupTestDB(t)
	repo := NewGormRepository(db)
	eventBus := NewEventBus(100)
	snapshotHub := NewSnapshotHub(eventBus)
	runtime := NewRuntimeManager(repo, eventBus, snapshotHub)

	run := &TaskRun{
		ID:        "cancel-run-1",
		Name:      "cancel-test",
		RunKind:   string(RunKindNormal),
		Status:    string(RunStatusRunning),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	require.NoError(t, repo.CreateRun(context.Background(), run))

	stage := &TaskRunStage{
		ID:         "cancel-stage-1",
		TaskRunID:  run.ID,
		StageKind:  string(StageKindDeviceCommand),
		StageName:  "命令执行",
		StageOrder: 1,
		Status:     string(StageStatusRunning),
		TotalUnits: 1,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	require.NoError(t, repo.CreateStage(context.Background(), stage))

	unit := &TaskRunUnit{
		ID:             "cancel-unit-1",
		TaskRunID:      run.ID,
		TaskRunStageID: stage.ID,
		UnitKind:       string(UnitKindDevice),
		TargetType:     "device_ip",
		TargetKey:      "10.0.0.1",
		Status:         string(UnitStatusRunning),
		DoneSteps:      2,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	require.NoError(t, repo.CreateUnit(context.Background(), unit))

	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runtimeCtx := &defaultRuntimeContext{
		runID:       run.ID,
		ctx:         runCtx,
		cancel:      cancel,
		repo:        repo,
		eventBus:    eventBus,
		logger:      newNoopRuntimeLogger(),
		snapshotHub: snapshotHub,
	}
	handler := NewErrorHandler(run.ID)

	runtime.projectCancellationToStages(runtimeCtx, handler, []TaskRunStage{*stage})

	gotUnit, err := repo.GetUnit(context.Background(), unit.ID)
	require.NoError(t, err)
	assert.Equal(t, string(UnitStatusCancelled), gotUnit.Status)
	assert.Equal(t, "run cancelled", gotUnit.ErrorMessage)

	gotStage, err := repo.GetStage(context.Background(), stage.ID)
	require.NoError(t, err)
	assert.Equal(t, string(StageStatusCancelled), gotStage.Status)
}

func TestRuntimeManagerFinalizeRunResourcesPersistsLogsAndEvictsRuntimeState(t *testing.T) {
	useTempStorageRoot(t)

	db := setupTestDB(t)
	repo := NewGormRepository(db)
	eventBus := NewEventBus(10)
	snapshotHub := NewSnapshotHub(eventBus)
	runtime := NewRuntimeManager(repo, eventBus, snapshotHub)

	run := &TaskRun{
		ID:        "finalize-run-1",
		Name:      "finalize-test",
		RunKind:   string(RunKindNormal),
		Status:    string(RunStatusRunning),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	require.NoError(t, repo.CreateRun(context.Background(), run))

	snapshot := NewExecutionSnapshotFromRun(run)
	snapshot.Units = []UnitSnapshot{
		{
			ID:         "unit-finalize-1",
			StageID:    "stage-finalize-1",
			TargetType: "device_ip",
			TargetKey:  "10.0.0.1",
			Status:     string(UnitStatusRunning),
		},
	}
	snapshotHub.Update(run.ID, snapshot)

	store, err := report.NewExecutionLogStore(run.Name, time.Now())
	require.NoError(t, err)
	session, err := store.EnsureDevice("10.0.0.1", false)
	require.NoError(t, err)
	require.NoError(t, session.WriteSummary("命令执行失败，进入收尾"))

	runCtx, cancel := context.WithCancel(context.Background())
	runtimeCtx := &defaultRuntimeContext{
		runID:       run.ID,
		ctx:         runCtx,
		cancel:      cancel,
		repo:        repo,
		eventBus:    eventBus,
		logger:      newNoopRuntimeLogger(),
		logStore:    store,
		snapshotHub: snapshotHub,
	}
	defer cancel()

	runtime.mu.Lock()
	runtime.runningRuns[run.ID] = runtimeCtx
	runtime.logStores[run.ID] = store
	runtime.mu.Unlock()

	runtime.finalizeRunResources(run.ID, nil)

	runtime.mu.RLock()
	_, runExists := runtime.runningRuns[run.ID]
	_, storeExists := runtime.logStores[run.ID]
	runtime.mu.RUnlock()
	assert.False(t, runExists)
	assert.False(t, storeExists)

	got, err := runtime.GetSnapshot(run.ID)
	require.NoError(t, err)
	require.Len(t, got.Units, 1)
	assert.Equal(t, 1, got.Units[0].LogCount)
	require.Len(t, got.Units[0].Logs, 1)
	assert.Contains(t, got.Units[0].Logs[0], "命令执行失败，进入收尾")
	assert.NotEmpty(t, got.Units[0].SummaryLogPath)
}

func TestRuntimeManagerStopFinalizesAllRunResources(t *testing.T) {
	useTempStorageRoot(t)

	db := setupTestDB(t)
	repo := NewGormRepository(db)
	eventBus := NewEventBus(10)
	snapshotHub := NewSnapshotHub(eventBus)
	runtime := NewRuntimeManager(repo, eventBus, snapshotHub)

	run := &TaskRun{
		ID:        "stop-finalize-run-1",
		Name:      "stop-finalize-test",
		RunKind:   string(RunKindNormal),
		Status:    string(RunStatusRunning),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	require.NoError(t, repo.CreateRun(context.Background(), run))

	snapshot := NewExecutionSnapshotFromRun(run)
	snapshot.Units = []UnitSnapshot{
		{
			ID:         "unit-stop-1",
			StageID:    "stage-stop-1",
			TargetType: "device_ip",
			TargetKey:  "10.0.0.2",
			Status:     string(UnitStatusRunning),
		},
	}
	snapshotHub.Update(run.ID, snapshot)

	store, err := report.NewExecutionLogStore(run.Name, time.Now())
	require.NoError(t, err)
	session, err := store.EnsureDevice("10.0.0.2", false)
	require.NoError(t, err)
	require.NoError(t, session.WriteSummary("Stop 收尾写入摘要"))

	runCtx, cancel := context.WithCancel(context.Background())
	runtimeCtx := &defaultRuntimeContext{
		runID:       run.ID,
		ctx:         runCtx,
		cancel:      cancel,
		repo:        repo,
		eventBus:    eventBus,
		logger:      newNoopRuntimeLogger(),
		logStore:    store,
		snapshotHub: snapshotHub,
	}
	defer cancel()

	runtime.mu.Lock()
	runtime.runningRuns[run.ID] = runtimeCtx
	runtime.logStores[run.ID] = store
	runtime.mu.Unlock()

	runtime.Stop()

	runtime.mu.RLock()
	assert.Empty(t, runtime.runningRuns)
	assert.Empty(t, runtime.logStores)
	runtime.mu.RUnlock()

	got, err := runtime.GetSnapshot(run.ID)
	require.NoError(t, err)
	require.Len(t, got.Units, 1)
	assert.Equal(t, 1, got.Units[0].LogCount)
	require.Len(t, got.Units[0].Logs, 1)
	assert.Contains(t, got.Units[0].Logs[0], "Stop 收尾写入摘要")
}

func TestFinishRunAndStageWithStatus(t *testing.T) {
	db := setupTestDB(t)
	repo := NewGormRepository(db)
	eventBus := NewEventBus(10)
	snapshotHub := NewSnapshotHub(eventBus)

	run := &TaskRun{
		ID:        "finish-run-1",
		Name:      "finish-test",
		RunKind:   string(RunKindNormal),
		Status:    string(RunStatusRunning),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	require.NoError(t, repo.CreateRun(context.Background(), run))

	stage := &TaskRunStage{
		ID:         "finish-stage-1",
		TaskRunID:  run.ID,
		StageKind:  string(StageKindDeviceCommand),
		StageName:  "命令执行",
		StageOrder: 1,
		Status:     string(StageStatusRunning),
		TotalUnits: 0,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	require.NoError(t, repo.CreateStage(context.Background(), stage))

	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runtimeCtx := &defaultRuntimeContext{
		runID:       run.ID,
		ctx:         runCtx,
		cancel:      cancel,
		repo:        repo,
		eventBus:    eventBus,
		logger:      newNoopRuntimeLogger(),
		snapshotHub: snapshotHub,
	}
	handler := NewErrorHandler(run.ID)

	finishRunWithStatus(handler, runtimeCtx, string(RunStatusCompleted), "测试 run 终态收口")
	finishStageWithStatus(handler, runtimeCtx, stage.ID, string(StageStatusFailed), "测试 stage 终态收口")

	gotRun, err := repo.GetRun(context.Background(), run.ID)
	require.NoError(t, err)
	assert.Equal(t, string(RunStatusCompleted), gotRun.Status)
	assert.NotNil(t, gotRun.FinishedAt)

	gotStage, err := repo.GetStage(context.Background(), stage.ID)
	require.NoError(t, err)
	assert.Equal(t, string(StageStatusFailed), gotStage.Status)
	assert.NotNil(t, gotStage.FinishedAt)
}

// mockPlanCompiler 模拟计划编译器
type mockPlanCompiler struct {
	kind string
}

func (m *mockPlanCompiler) Compile(ctx context.Context, def *TaskDefinition) (*ExecutionPlan, error) {
	return &ExecutionPlan{RunKind: m.kind}, nil
}

func (m *mockPlanCompiler) Supports(kind string) bool {
	return kind == m.kind
}

// mockStageExecutor 模拟Stage执行器
type mockStageExecutor struct {
	kind string
}

func (m *mockStageExecutor) Kind() string {
	return m.kind
}

func (m *mockStageExecutor) Run(ctx RuntimeContext, stage *StagePlan) error {
	return nil
}

// intPtr int指针辅助函数
func intPtr(i int) *int {
	return &i
}
