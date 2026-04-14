package ui

import (
	"context"
	"testing"
	"time"

	"github.com/NetWeaverGo/core/internal/taskexec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockRepository 是 taskexec.Repository 的 mock 实现
type MockRepository struct {
	mock.Mock
}

func (m *MockRepository) CreateRun(ctx context.Context, run *taskexec.TaskRun) error {
	args := m.Called(ctx, run)
	return args.Error(0)
}

func (m *MockRepository) GetRun(ctx context.Context, runID string) (*taskexec.TaskRun, error) {
	args := m.Called(ctx, runID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*taskexec.TaskRun), args.Error(1)
}

func (m *MockRepository) UpdateRun(ctx context.Context, runID string, patch *taskexec.RunPatch) error {
	args := m.Called(ctx, runID, patch)
	return args.Error(0)
}

func (m *MockRepository) ListRuns(ctx context.Context, limit int) ([]taskexec.TaskRun, error) {
	args := m.Called(ctx, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]taskexec.TaskRun), args.Error(1)
}

func (m *MockRepository) ListRunsFiltered(ctx context.Context, limit int, taskGroupID uint, runKind, status string) ([]taskexec.TaskRun, error) {
	args := m.Called(ctx, limit, taskGroupID, runKind, status)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]taskexec.TaskRun), args.Error(1)
}

func (m *MockRepository) ListRunningRuns(ctx context.Context) ([]taskexec.TaskRun, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]taskexec.TaskRun), args.Error(1)
}

func (m *MockRepository) CreateStage(ctx context.Context, stage *taskexec.TaskRunStage) error {
	args := m.Called(ctx, stage)
	return args.Error(0)
}

func (m *MockRepository) GetStage(ctx context.Context, stageID string) (*taskexec.TaskRunStage, error) {
	args := m.Called(ctx, stageID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*taskexec.TaskRunStage), args.Error(1)
}

func (m *MockRepository) UpdateStage(ctx context.Context, stageID string, patch *taskexec.StagePatch) error {
	args := m.Called(ctx, stageID, patch)
	return args.Error(0)
}

func (m *MockRepository) GetStagesByRun(ctx context.Context, runID string) ([]taskexec.TaskRunStage, error) {
	args := m.Called(ctx, runID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]taskexec.TaskRunStage), args.Error(1)
}

func (m *MockRepository) CreateUnit(ctx context.Context, unit *taskexec.TaskRunUnit) error {
	args := m.Called(ctx, unit)
	return args.Error(0)
}

func (m *MockRepository) GetUnit(ctx context.Context, unitID string) (*taskexec.TaskRunUnit, error) {
	args := m.Called(ctx, unitID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*taskexec.TaskRunUnit), args.Error(1)
}

func (m *MockRepository) UpdateUnit(ctx context.Context, unitID string, patch *taskexec.UnitPatch) error {
	args := m.Called(ctx, unitID, patch)
	return args.Error(0)
}

func (m *MockRepository) GetUnitsByStage(ctx context.Context, stageID string) ([]taskexec.TaskRunUnit, error) {
	args := m.Called(ctx, stageID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]taskexec.TaskRunUnit), args.Error(1)
}

func (m *MockRepository) GetUnitsByRun(ctx context.Context, runID string) ([]taskexec.TaskRunUnit, error) {
	args := m.Called(ctx, runID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]taskexec.TaskRunUnit), args.Error(1)
}

func (m *MockRepository) CreateEvent(ctx context.Context, event *taskexec.TaskRunEvent) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

func (m *MockRepository) GetEventsByRun(ctx context.Context, runID string, limit int) ([]taskexec.TaskRunEvent, error) {
	args := m.Called(ctx, runID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]taskexec.TaskRunEvent), args.Error(1)
}

func (m *MockRepository) CreateArtifact(ctx context.Context, artifact *taskexec.TaskArtifact) error {
	args := m.Called(ctx, artifact)
	return args.Error(0)
}

func (m *MockRepository) GetArtifactsByRun(ctx context.Context, runID string) ([]taskexec.TaskArtifact, error) {
	args := m.Called(ctx, runID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]taskexec.TaskArtifact), args.Error(1)
}

func (m *MockRepository) GetArtifactsByStage(ctx context.Context, stageID string) ([]taskexec.TaskArtifact, error) {
	args := m.Called(ctx, stageID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]taskexec.TaskArtifact), args.Error(1)
}

func (m *MockRepository) GetArtifactsByUnit(ctx context.Context, unitID string) ([]taskexec.TaskArtifact, error) {
	args := m.Called(ctx, unitID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]taskexec.TaskArtifact), args.Error(1)
}

func (m *MockRepository) DeleteRun(ctx context.Context, runID string) error {
	args := m.Called(ctx, runID)
	return args.Error(0)
}

func (m *MockRepository) DeleteAllRuns(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockRepository) DeleteAllRunsBatch(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockRepository) DeleteRunsByKind(ctx context.Context, runKind string) error {
	args := m.Called(ctx, runKind)
	return args.Error(0)
}

func (m *MockRepository) DeleteRunsByTaskGroup(ctx context.Context, taskGroupID uint) error {
	args := m.Called(ctx, taskGroupID)
	return args.Error(0)
}

// ==================== 测试用例 ====================

func TestDeleteRunRecord_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	service := NewExecutionHistoryService(mockRepo)

	runID := "test-run-id"
	now := time.Now()
	run := &taskexec.TaskRun{
		ID:        runID,
		Status:    "completed",
		RunKind:   "normal",
		StartedAt: &now,
	}

	// 设置 mock 期望
	mockRepo.On("GetRun", context.Background(), runID).Return(run, nil)
	mockRepo.On("GetUnitsByRun", context.Background(), runID).Return([]taskexec.TaskRunUnit{}, nil)
	mockRepo.On("GetArtifactsByRun", context.Background(), runID).Return([]taskexec.TaskArtifact{}, nil)
	mockRepo.On("DeleteRun", context.Background(), runID).Return(nil)

	// 执行测试
	result, err := service.DeleteRunRecord(DeleteRunRecordRequest{RunID: runID})

	// 验证结果
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Success)
	assert.Equal(t, "删除成功", result.Message)

	// 验证 mock 调用
	mockRepo.AssertExpectations(t)
}

func TestDeleteRunRecord_EmptyRunID(t *testing.T) {
	mockRepo := new(MockRepository)
	service := NewExecutionHistoryService(mockRepo)

	// 执行测试
	result, err := service.DeleteRunRecord(DeleteRunRecordRequest{RunID: ""})

	// 验证结果
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Equal(t, "runID 不能为空", result.Message)
}

func TestDeleteRunRecord_RunningTask(t *testing.T) {
	mockRepo := new(MockRepository)
	service := NewExecutionHistoryService(mockRepo)

	runID := "test-run-id"
	now := time.Now()
	run := &taskexec.TaskRun{
		ID:        runID,
		Status:    "running", // 正在运行的任务
		RunKind:   "normal",
		StartedAt: &now,
	}

	// 设置 mock 期望
	mockRepo.On("GetRun", context.Background(), runID).Return(run, nil)

	// 执行测试
	result, err := service.DeleteRunRecord(DeleteRunRecordRequest{RunID: runID})

	// 验证结果
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Equal(t, "无法删除正在运行的任务", result.Message)

	// 验证 mock 调用
	mockRepo.AssertExpectations(t)
}

func TestDeleteRunRecord_RepositoryNil(t *testing.T) {
	service := NewExecutionHistoryService(nil)

	// 执行测试
	result, err := service.DeleteRunRecord(DeleteRunRecordRequest{RunID: "test-run-id"})

	// 验证结果
	assert.Error(t, err)
	assert.Equal(t, "仓库未初始化", err.Error())
	assert.Nil(t, result)
}

func TestDeleteRunRecord_TaskGroupIDMismatch(t *testing.T) {
	mockRepo := new(MockRepository)
	service := NewExecutionHistoryService(mockRepo)

	runID := "test-run-id"
	now := time.Now()
	run := &taskexec.TaskRun{
		ID:          runID,
		TaskGroupID: 100, // 实际属于任务组 100
		Status:      "completed",
		RunKind:     "normal",
		StartedAt:   &now,
	}

	// 设置 mock 期望
	mockRepo.On("GetRun", context.Background(), runID).Return(run, nil)

	// 执行测试 - 传入不同的 taskGroupId
	result, err := service.DeleteRunRecord(DeleteRunRecordRequest{
		RunID:       runID,
		TaskGroupID: "200", // 请求声称属于任务组 200
	})

	// 验证结果
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Equal(t, "无权删除该记录", result.Message)

	// 验证 mock 调用
	mockRepo.AssertExpectations(t)
}

func TestDeleteAllRunRecords_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	service := NewExecutionHistoryService(mockRepo)

	now := time.Now()
	runs := []taskexec.TaskRun{
		{
			ID:        "run-1",
			Status:    "completed",
			StartedAt: &now,
		},
	}

	// 设置 mock 期望
	mockRepo.On("ListRunningRuns", context.Background()).Return([]taskexec.TaskRun{}, nil)
	mockRepo.On("ListRuns", context.Background(), 0).Return(runs, nil)
	mockRepo.On("DeleteAllRunsBatch", context.Background()).Return(nil)

	// 执行测试
	result, err := service.DeleteAllRunRecords(DeleteAllRunRecordsRequest{})

	// 验证结果
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Success)
	assert.Equal(t, "成功删除 1 条记录", result.Message)

	// 验证 mock 调用
	mockRepo.AssertExpectations(t)
}

func TestDeleteAllRunRecords_NoRecords(t *testing.T) {
	mockRepo := new(MockRepository)
	service := NewExecutionHistoryService(mockRepo)

	// 设置 mock 期望 - 返回空列表
	mockRepo.On("ListRunningRuns", context.Background()).Return([]taskexec.TaskRun{}, nil)
	mockRepo.On("ListRuns", context.Background(), 0).Return([]taskexec.TaskRun{}, nil)

	// 执行测试
	result, err := service.DeleteAllRunRecords(DeleteAllRunRecordsRequest{})

	// 验证结果
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Success)
	assert.Equal(t, "没有可删除的记录", result.Message)

	// 验证 mock 调用
	mockRepo.AssertExpectations(t)
}

func TestDeleteAllRunRecords_WithTaskGroupID(t *testing.T) {
	mockRepo := new(MockRepository)
	service := NewExecutionHistoryService(mockRepo)

	now := time.Now()
	runs := []taskexec.TaskRun{
		{
			ID:          "run-1",
			TaskGroupID: 100,
			Status:      "completed",
			StartedAt:   &now,
		},
	}

	// 设置 mock 期望
	mockRepo.On("ListRunningRuns", context.Background()).Return([]taskexec.TaskRun{}, nil)
	mockRepo.On("ListRunsFiltered", context.Background(), 0, uint(100), "", "").Return(runs, nil)
	mockRepo.On("DeleteRunsByTaskGroup", context.Background(), uint(100)).Return(nil)

	// 执行测试 - 指定 taskGroupId
	result, err := service.DeleteAllRunRecords(DeleteAllRunRecordsRequest{TaskGroupID: "100"})

	// 验证结果
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Success)
	assert.Equal(t, "成功删除 1 条记录", result.Message)

	// 验证 mock 调用
	mockRepo.AssertExpectations(t)
}

func TestDeleteAllRunRecords_HasRunningTasks(t *testing.T) {
	mockRepo := new(MockRepository)
	service := NewExecutionHistoryService(mockRepo)

	now := time.Now()
	runningTasks := []taskexec.TaskRun{
		{
			ID:        "running-1",
			Status:    "running",
			StartedAt: &now,
		},
	}

	// 设置 mock 期望
	mockRepo.On("ListRunningRuns", context.Background()).Return(runningTasks, nil)

	// 执行测试
	result, err := service.DeleteAllRunRecords(DeleteAllRunRecordsRequest{})

	// 验证结果
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Contains(t, result.Message, "存在 1 个正在运行的任务")

	// 验证 mock 调用
	mockRepo.AssertExpectations(t)
}

func TestDeleteAllRunRecords_RepositoryNil(t *testing.T) {
	service := NewExecutionHistoryService(nil)

	// 执行测试
	result, err := service.DeleteAllRunRecords(DeleteAllRunRecordsRequest{})

	// 验证结果
	assert.Error(t, err)
	assert.Equal(t, "仓库未初始化", err.Error())
	assert.Nil(t, result)
}
