package taskexec

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/parser"
	"github.com/NetWeaverGo/core/internal/repository"
	"gorm.io/gorm"
)

// TaskExecutionService 统一任务执行服务
type TaskExecutionService struct {
	runtime        *RuntimeManager
	compiler       *CompilerRegistry
	snapshot       *SnapshotHub
	repo           Repository
	db             *gorm.DB
	deviceRepo     repository.DeviceRepository
	parserProvider parser.ParserProvider
}

// NewTaskExecutionService 创建任务执行服务
func NewTaskExecutionService(db *gorm.DB, parserProvider parser.ParserProvider) *TaskExecutionService {
	repo := NewGormRepository(db)
	eventBus := NewEventBus(1000)
	snapshotHub := NewSnapshotHub(eventBus)
	runtime := NewRuntimeManager(repo, eventBus, snapshotHub)

	// 注册编译器
	compilerReg := NewCompilerRegistry()
	compilerReg.Register(string(RunKindNormal), NewNormalTaskCompiler(nil))
	compilerReg.Register(string(RunKindTopology), NewTopologyTaskCompiler(nil))

	// Register stage executors
	runtime.RegisterExecutor(NewDeviceCommandExecutor(repository.NewDeviceRepository()))
	runtime.RegisterExecutor(NewDeviceCollectExecutor(repository.NewDeviceRepository()))
	runtime.RegisterExecutor(NewParseExecutor(db, parserProvider))
	runtime.RegisterExecutor(NewTopologyBuildExecutor(db))

	service := &TaskExecutionService{
		runtime:        runtime,
		compiler:       compilerReg,
		snapshot:       snapshotHub,
		repo:           repo,
		db:             db,
		deviceRepo:     repository.NewDeviceRepository(),
		parserProvider: parserProvider,
	}

	return service
}

// Start 启动服务
func (s *TaskExecutionService) Start() {
	if s.repo != nil {
		recovered, err := recoverInterruptedRuns(s.repo, time.Now())
		if err != nil {
			logger.Error("TaskExec", "-", "启动恢复遗留活跃运行失败: recovered=%v err=%v", recovered, err)
		} else if len(recovered) > 0 {
			logger.Warn("TaskExec", "-", "启动时已补偿取消遗留活跃运行: runs=%v", recovered)
		}
	}
	s.runtime.Start()
}

// Stop 停止服务
func (s *TaskExecutionService) Stop() {
	s.runtime.Stop()
}

type RunMetadata struct {
	TaskGroupID      uint
	TaskNameSnapshot string
	LaunchSpecJSON   []byte
}

// CreateNormalTask 创建普通任务定义
func (s *TaskExecutionService) CreateNormalTask(name string, config *NormalTaskConfig) (*TaskDefinition, error) {
	configJSON, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("fail to marshal config: %w", err)
	}

	def := &TaskDefinition{
		ID:     newDefinitionID(),
		Name:   name,
		Kind:   string(RunKindNormal),
		Config: configJSON,
	}

	return def, nil
}

// CreateTopologyTask 创建拓扑任务定义
func (s *TaskExecutionService) CreateTopologyTask(name string, config *TopologyTaskConfig) (*TaskDefinition, error) {
	configJSON, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("fail to marshal config: %w", err)
	}

	def := &TaskDefinition{
		ID:     newDefinitionID(),
		Name:   name,
		Kind:   string(RunKindTopology),
		Config: configJSON,
	}

	return def, nil
}

// StartTask 启动任务
func (s *TaskExecutionService) StartTask(ctx context.Context, def *TaskDefinition) (string, error) {
	return s.StartTaskWithMetadata(ctx, def, nil)
}

// StartTaskWithMetadata 启动任务并附带运行元数据
func (s *TaskExecutionService) StartTaskWithMetadata(ctx context.Context, def *TaskDefinition, metadata *RunMetadata) (string, error) {
	plan, err := s.compiler.Compile(ctx, def)
	if err != nil {
		return "", fmt.Errorf("fail to compile plan: %w", err)
	}

	run, err := s.runtime.Execute(ctx, plan, def, metadata)
	if err != nil {
		return "", fmt.Errorf("fail to start task: %w", err)
	}

	return run.ID, nil
}

// CancelRun 取消运行
func (s *TaskExecutionService) CancelRun(runID string) error {
	return s.runtime.CancelRun(runID)
}

// GetSnapshot 获取执行快照
func (s *TaskExecutionService) GetSnapshot(runID string) (*ExecutionSnapshot, error) {
	return s.runtime.GetSnapshot(runID)
}

// GetSnapshotDelta 获取执行快照增量。
func (s *TaskExecutionService) GetSnapshotDelta(runID string) (*SnapshotDelta, error) {
	return s.runtime.GetSnapshotDelta(runID)
}

// ListRunning 获取运行中的任务
func (s *TaskExecutionService) ListRunning() []*ExecutionSnapshot {
	return s.runtime.ListRunningSnapshots()
}

// ListRuns 获取历史运行记录
func (s *TaskExecutionService) ListRuns(limit int) ([]*RunSummary, error) {
	runs, err := s.repo.ListRuns(context.Background(), limit)
	if err != nil {
		return nil, err
	}

	summaries := make([]*RunSummary, 0, len(runs))
	builder := NewSnapshotBuilder()

	for _, run := range runs {
		stages, _ := s.repo.GetStagesByRun(context.Background(), run.ID)
		summary := builder.BuildSummary(&run, stages)
		summaries = append(summaries, summary)
	}

	return summaries, nil
}

// GetRunStatus 获取运行状态
func (s *TaskExecutionService) GetRunStatus(runID string) (*TaskRun, error) {
	return s.repo.GetRun(context.Background(), runID)
}

// GetEventBus 获取事件总线（用于事件桥接）
func (s *TaskExecutionService) GetEventBus() *EventBus {
	return s.runtime.GetEventBus()
}

// GetRunArtifacts 获取指定运行的产物索引。
func (s *TaskExecutionService) GetRunArtifacts(runID string) ([]TaskArtifact, error) {
	if strings.TrimSpace(runID) == "" {
		return nil, fmt.Errorf("runID 不能为空")
	}
	return s.repo.GetArtifactsByRun(context.Background(), runID)
}

// GetRepository 获取仓库实例（供其他服务使用）
func (s *TaskExecutionService) GetRepository() Repository {
	return s.repo
}
