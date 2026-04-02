package taskexec

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/NetWeaverGo/core/internal/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNormalTaskCompileAndExecute tests normal task compilation and execution flow
func TestNormalTaskCompileAndExecute(t *testing.T) {
	db := setupTestDB(t)
	repo := NewGormRepository(db)

	// Create a normal task definition
	config := &NormalTaskConfig{
		Mode:       "group",
		DeviceIDs:  []uint{1, 2, 3},
		DeviceIPs:  []string{"192.168.1.1", "192.168.1.2", "192.168.1.3"},
		Commands:   []string{"show version", "show interface"},
		TimeoutSec: 60,
	}

	configJSON, _ := json.Marshal(config)
	def := &TaskDefinition{
		ID:     "test-normal-task",
		Name:   "Test Normal Task",
		Kind:   string(RunKindNormal),
		Config: configJSON,
	}

	// Compile the task
	compiler := NewNormalTaskCompiler(nil)
	plan, err := compiler.Compile(context.Background(), def)
	require.NoError(t, err)
	assert.NotNil(t, plan)
	assert.Equal(t, string(RunKindNormal), plan.RunKind)
	assert.Equal(t, 1, len(plan.Stages))
	assert.Equal(t, 3, len(plan.Stages[0].Units))

	// Create and start runtime
	eventBus := NewEventBus(100)
	snapshotHub := NewSnapshotHub(eventBus)
	runtime := NewRuntimeManager(repo, eventBus, snapshotHub)

	// Execute plan
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	run, err := runtime.Execute(ctx, plan, def, nil)
	require.NoError(t, err)
	assert.NotEmpty(t, run.ID)

	// Wait a bit for execution
	time.Sleep(100 * time.Millisecond)

	// Verify run was created
	foundRun, err := repo.GetRun(context.Background(), run.ID)
	require.NoError(t, err)
	assert.Equal(t, run.ID, foundRun.ID)
}

// TestTopologyTaskCompile tests topology task compilation
func TestTopologyTaskCompile(t *testing.T) {
	config := &TopologyTaskConfig{
		DeviceIDs:         []uint{1, 2},
		DeviceIPs:         []string{"192.168.1.1", "192.168.1.2"},
		Vendor:            "huawei",
		MaxWorkers:        5,
		TimeoutSec:        300,
		AutoBuildTopology: true,
	}

	configJSON, _ := json.Marshal(config)
	def := &TaskDefinition{
		ID:     "test-topology-task",
		Name:   "Test Topology Task",
		Kind:   string(RunKindTopology),
		Config: configJSON,
	}

	// Compile the task
	compiler := NewTopologyTaskCompiler(nil)
	plan, err := compiler.Compile(context.Background(), def)
	require.NoError(t, err)
	assert.NotNil(t, plan)
	assert.Equal(t, string(RunKindTopology), plan.RunKind)
	assert.Equal(t, 3, len(plan.Stages)) // collect, parse, build

	// Verify stage order
	assert.Equal(t, string(StageKindDeviceCollect), plan.Stages[0].Kind)
	assert.Equal(t, string(StageKindParse), plan.Stages[1].Kind)
	assert.Equal(t, string(StageKindTopologyBuild), plan.Stages[2].Kind)
}

// TestExecutionSnapshotBuild tests snapshot building from run state
func TestExecutionSnapshotBuild(t *testing.T) {
	db := setupTestDB(t)
	repo := NewGormRepository(db)
	ctx := context.Background()

	// Create run
	run := &TaskRun{
		ID:       "test-run-1",
		Name:     "Test Run",
		RunKind:  string(RunKindNormal),
		Status:   string(RunStatusRunning),
		Progress: 50,
	}
	err := repo.CreateRun(ctx, run)
	require.NoError(t, err)

	// Create stages
	stage1 := &TaskRunStage{
		ID:             "stage-1",
		TaskRunID:      run.ID,
		StageKind:      string(StageKindDeviceCommand),
		StageName:      "Command Execution",
		StageOrder:     1,
		Status:         string(StageStatusRunning),
		Progress:       60,
		TotalUnits:     5,
		CompletedUnits: 3,
		SuccessUnits:   3,
	}
	err = repo.CreateStage(ctx, stage1)
	require.NoError(t, err)

	// Create units
	unit1 := &TaskRunUnit{
		ID:             "unit-1",
		TaskRunID:      run.ID,
		TaskRunStageID: stage1.ID,
		UnitKind:       string(UnitKindDevice),
		TargetType:     "device_ip",
		TargetKey:      "192.168.1.1",
		Status:         string(UnitStatusCompleted),
		TotalSteps:     10,
		DoneSteps:      10,
	}
	err = repo.CreateUnit(ctx, unit1)
	require.NoError(t, err)

	// Build snapshot
	builder := NewSnapshotBuilder()
	stages, _ := repo.GetStagesByRun(ctx, run.ID)
	units, _ := repo.GetUnitsByRun(ctx, run.ID)
	events := []TaskRunEvent{}

	snapshot := builder.Build(run, stages, units, events)

	assert.Equal(t, run.ID, snapshot.RunID)
	assert.Equal(t, 50, snapshot.Progress)
	assert.Equal(t, 1, len(snapshot.Stages))
	assert.Equal(t, 1, len(snapshot.Units))
	assert.Equal(t, 100, snapshot.Units[0].Progress) // 10/10 * 100
}

// TestTaskExecutionService tests the unified service
func TestTaskExecutionService(t *testing.T) {
	db := setupTestDB(t)
	parserManager := parser.NewParserManager()
	if err := parserManager.Bootstrap(); err != nil {
		t.Fatalf("解析器管理器启动失败: %v", err)
	}
	service := NewTaskExecutionService(db, parserManager)

	// Create normal task
	normalConfig := &NormalTaskConfig{
		Mode:      "group",
		DeviceIDs: []uint{1, 2},
		DeviceIPs: []string{"192.168.1.1", "192.168.1.2"},
		Commands:  []string{"show version"},
	}

	def, err := service.CreateNormalTask("Test Task", normalConfig)
	require.NoError(t, err)
	assert.Equal(t, string(RunKindNormal), def.Kind)

	// Create topology task
	topologyConfig := &TopologyTaskConfig{
		DeviceIDs: []uint{1},
		DeviceIPs: []string{"192.168.1.1"},
		Vendor:    "huawei",
	}

	def2, err := service.CreateTopologyTask("Test Topology", topologyConfig)
	require.NoError(t, err)
	assert.Equal(t, string(RunKindTopology), def2.Kind)
}

// TestEventBusSubscribeAndEmit tests event bus functionality
func TestEventBusSubscribeAndEmit(t *testing.T) {
	bus := NewEventBus(100)
	bus.Start()
	defer bus.Stop()

	received := make(chan *TaskEvent, 10)
	bus.Subscribe(func(event *TaskEvent) {
		received <- event
	})

	// Emit multiple events
	events := []*TaskEvent{
		NewTaskEvent("run-1", EventTypeRunStarted, "Task started"),
		NewTaskEvent("run-1", EventTypeStageStarted, "Stage started").WithStage("stage-1"),
		NewTaskEvent("run-1", EventTypeUnitStarted, "Unit started").WithStage("stage-1").WithUnit("unit-1"),
	}

	for _, e := range events {
		bus.EmitSync(e)
	}

	// Verify events received
	for i := 0; i < len(events); i++ {
		select {
		case e := <-received:
			assert.NotEmpty(t, e.ID)
			assert.Equal(t, "run-1", e.RunID)
		case <-time.After(time.Second):
			t.Fatal("Timeout waiting for event")
		}
	}
}
