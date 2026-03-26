package taskexec

import (
	"context"
	"fmt"
	"testing"
	"time"

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
	err = repo.UpdateRun(context.Background(), run.ID, &RunPatch{
		Status:    &newStatus,
		Progress:  intPtr(50),
		StartedAt: &now,
	})
	require.NoError(t, err)

	// 验证更新
	found, err := repo.GetRun(context.Background(), run.ID)
	require.NoError(t, err)
	assert.Equal(t, string(RunStatusRunning), found.Status)
	assert.Equal(t, 50, found.Progress)
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

func TestTaskEventRepositoryProjector(t *testing.T) {
	db := setupTestDB(t)
	repo := NewGormRepository(db)
	projector := NewTaskEventRepositoryProjector(repo)

	event := NewTaskEvent("run-1", EventTypeRunProgress, "任务推进").
		WithStage("stage-1").
		WithUnit("unit-1").
		WithPayload("progress", 50)

	projector(event)

	events, err := repo.GetEventsByRun(context.Background(), "run-1", 10)
	require.NoError(t, err)
	if len(events) != 1 {
		t.Fatalf("事件持久化数量错误: got=%d want=1", len(events))
	}
	assert.Equal(t, string(EventTypeRunProgress), events[0].EventType)
	assert.Equal(t, "stage-1", events[0].StageID)
	assert.Equal(t, "unit-1", events[0].UnitID)
	assert.Contains(t, events[0].PayloadJSON, "\"progress\":50")
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
