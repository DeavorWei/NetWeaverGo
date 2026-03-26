package taskexec

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/report"
)

// RuntimeContext 运行时上下文 - 供StageExecutor使用
type RuntimeContext interface {
	// 基础信息
	RunID() string
	Context() context.Context

	// 状态更新
	UpdateRun(patch *RunPatch) error
	UpdateStage(stageID string, patch *StagePatch) error
	UpdateUnit(unitID string, patch *UnitPatch) error

	// 事件发射
	Emit(event *TaskEvent)

	// 日志
	Logger(scope LogScope) RuntimeLogger

	// 取消检查
	IsCancelled() bool
}

// defaultRuntimeContext 默认运行时上下文实现
type defaultRuntimeContext struct {
	runID       string
	ctx         context.Context
	cancel      context.CancelFunc
	repo        Repository
	eventBus    *EventBus
	logger      RuntimeLogger
	logStore    *report.ExecutionLogStore
	snapshotHub *SnapshotHub
}

// RunID 获取RunID
func (c *defaultRuntimeContext) RunID() string {
	return c.runID
}

// Context 获取上下文
func (c *defaultRuntimeContext) Context() context.Context {
	return c.ctx
}

// UpdateRun 更新Run状态
func (c *defaultRuntimeContext) UpdateRun(patch *RunPatch) error {
	err := c.repo.UpdateRun(c.ctx, c.runID, patch)
	if err != nil {
		return err
	}

	// 触发快照更新
	c.refreshSnapshot()
	return nil
}

// UpdateStage 更新Stage状态
func (c *defaultRuntimeContext) UpdateStage(stageID string, patch *StagePatch) error {
	if err := c.repo.UpdateStage(c.ctx, stageID, patch); err != nil {
		return err
	}
	c.refreshSnapshot()
	return nil
}

// UpdateUnit 更新Unit状态
func (c *defaultRuntimeContext) UpdateUnit(unitID string, patch *UnitPatch) error {
	if err := c.repo.UpdateUnit(c.ctx, unitID, patch); err != nil {
		return err
	}
	c.refreshSnapshot()
	return nil
}

// Emit 发射事件
func (c *defaultRuntimeContext) Emit(event *TaskEvent) {
	c.eventBus.EmitSync(event)
}

// Logger 获取日志记录器
func (c *defaultRuntimeContext) Logger(scope LogScope) RuntimeLogger {
	return c.logger
}

// IsCancelled 检查是否已取消
func (c *defaultRuntimeContext) IsCancelled() bool {
	select {
	case <-c.ctx.Done():
		return true
	default:
		return false
	}
}

// refreshSnapshot 刷新快照
func (c *defaultRuntimeContext) refreshSnapshot() {
	run, err := c.repo.GetRun(c.ctx, c.runID)
	if err != nil {
		log.Printf("[Runtime] refreshSnapshot: GetRun failed, runID=%s, err=%v", c.runID, err)
		return
	}
	if run == nil {
		log.Printf("[Runtime] refreshSnapshot: run not found, runID=%s", c.runID)
		return
	}

	stages, err := c.repo.GetStagesByRun(c.ctx, c.runID)
	if err != nil {
		log.Printf("[Runtime] refreshSnapshot: GetStagesByRun failed, runID=%s, err=%v", c.runID, err)
	}

	units, err := c.repo.GetUnitsByRun(c.ctx, c.runID)
	if err != nil {
		log.Printf("[Runtime] refreshSnapshot: GetUnitsByRun failed, runID=%s, err=%v", c.runID, err)
	}

	events, err := c.repo.GetEventsByRun(c.ctx, c.runID, 50)
	if err != nil {
		log.Printf("[Runtime] refreshSnapshot: GetEventsByRun failed, runID=%s, err=%v", c.runID, err)
	}

	builder := NewSnapshotBuilder()
	snapshot := builder.Build(run, stages, units, events)
	enrichSnapshotWithStore(c.logStore, snapshot)
	c.snapshotHub.Update(run.ID, snapshot)
}

// StageExecutor Stage执行器接口
type StageExecutor interface {
	// Kind 返回执行器类型
	Kind() string
	// Run 执行Stage
	Run(ctx RuntimeContext, stage *StagePlan) error
}

// ExecutorRegistry Stage执行器注册表
type ExecutorRegistry struct {
	executors map[string]StageExecutor
	mu        sync.RWMutex
}

// NewExecutorRegistry 创建执行器注册表
func NewExecutorRegistry() *ExecutorRegistry {
	return &ExecutorRegistry{
		executors: make(map[string]StageExecutor),
	}
}

// Register 注册执行器
func (r *ExecutorRegistry) Register(executor StageExecutor) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.executors[executor.Kind()] = executor
}

// Get 获取执行器
func (r *ExecutorRegistry) Get(kind string) (StageExecutor, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	executor, ok := r.executors[kind]
	return executor, ok
}

// RuntimeManager 运行时管理器
type RuntimeManager struct {
	repo          Repository
	eventBus      *EventBus
	snapshotHub   *SnapshotHub
	executorReg   *ExecutorRegistry
	loggerFactory LoggerFactory

	runningRuns map[string]*defaultRuntimeContext
	logStores   map[string]*report.ExecutionLogStore
	mu          sync.RWMutex
}

// NewRuntimeManager 创建运行时管理器
func NewRuntimeManager(repo Repository, eventBus *EventBus, snapshotHub *SnapshotHub) *RuntimeManager {
	return &RuntimeManager{
		repo:        repo,
		eventBus:    eventBus,
		snapshotHub: snapshotHub,
		executorReg: NewExecutorRegistry(),
		runningRuns: make(map[string]*defaultRuntimeContext),
		logStores:   make(map[string]*report.ExecutionLogStore),
	}
}

// SetLoggerFactory 设置日志工厂
func (m *RuntimeManager) SetLoggerFactory(factory LoggerFactory) {
	m.loggerFactory = factory
}

// RegisterExecutor 注册Stage执行器
func (m *RuntimeManager) RegisterExecutor(executor StageExecutor) {
	m.executorReg.Register(executor)
}

// Start 启动事件总线
func (m *RuntimeManager) Start() {
	m.eventBus.Start()
}

// GetEventBus 获取事件总线（用于事件桥接）
func (m *RuntimeManager) GetEventBus() *EventBus {
	return m.eventBus
}

// Stop 停止运行时
func (m *RuntimeManager) Stop() {
	m.eventBus.Stop()

	// 取消所有运行中的任务
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, ctx := range m.runningRuns {
		ctx.cancel()
	}
	for runID, store := range m.logStores {
		if store == nil {
			continue
		}
		logger.Debug("TaskExec", runID, "关闭运行日志存储")
		store.Close()
	}
}

// GetSnapshot 获取指定运行的快照
func (m *RuntimeManager) GetSnapshot(runID string) (*ExecutionSnapshot, error) {
	if m.snapshotHub == nil {
		return nil, fmt.Errorf("snapshot hub not initialized")
	}
	snapshot, ok := m.snapshotHub.Get(runID)
	if ok && snapshot != nil {
		return snapshot, nil
	}

	run, err := m.repo.GetRun(context.Background(), runID)
	if err != nil || run == nil {
		return nil, fmt.Errorf("snapshot not found: %s", runID)
	}
	stages, _ := m.repo.GetStagesByRun(context.Background(), runID)
	units, _ := m.repo.GetUnitsByRun(context.Background(), runID)
	events, _ := m.repo.GetEventsByRun(context.Background(), runID, 50)
	builder := NewSnapshotBuilder()
	snapshot = builder.Build(run, stages, units, events)
	m.enrichSnapshotWithLogs(runID, snapshot)
	m.snapshotHub.Update(runID, snapshot)
	return snapshot, nil
}

// ListRunningSnapshots 列出运行中的任务快照
func (m *RuntimeManager) ListRunningSnapshots() []*ExecutionSnapshot {
	if m.snapshotHub == nil {
		return []*ExecutionSnapshot{}
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*ExecutionSnapshot, 0, len(m.runningRuns))
	for runID := range m.runningRuns {
		snapshot, err := m.GetSnapshot(runID)
		if err == nil && snapshot != nil {
			result = append(result, snapshot)
		}
	}
	return result
}

// CreateRun 创建新的运行实例
func (m *RuntimeManager) CreateRun(definitionID, name, runKind string, metadata *RunMetadata) (*TaskRun, error) {
	launchSpecJSON := ""
	taskGroupID := uint(0)
	taskNameSnapshot := name
	if metadata != nil {
		launchSpecJSON = string(metadata.LaunchSpecJSON)
		taskGroupID = metadata.TaskGroupID
		if metadata.TaskNameSnapshot != "" {
			taskNameSnapshot = metadata.TaskNameSnapshot
		}
	}

	run := &TaskRun{
		ID:               newRunID(),
		TaskDefinitionID: definitionID,
		TaskGroupID:      taskGroupID,
		TaskNameSnapshot: taskNameSnapshot,
		LaunchSpecJSON:   launchSpecJSON,
		Name:             name,
		RunKind:          runKind,
		Status:           string(RunStatusPending),
		Progress:         0,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	if err := m.repo.CreateRun(context.Background(), run); err != nil {
		return nil, fmt.Errorf("创建运行实例失败: %w", err)
	}

	return run, nil
}

// Execute 执行计划
func (m *RuntimeManager) Execute(ctx context.Context, plan *ExecutionPlan, def *TaskDefinition, metadata *RunMetadata) (*TaskRun, error) {
	definitionID := ""
	if def != nil {
		definitionID = def.ID
	}

	run, err := m.CreateRun(definitionID, plan.Name, plan.RunKind, metadata)
	if err != nil {
		return nil, err
	}

	// 创建运行时上下文
	runCtx, cancel := context.WithCancel(ctx)
	runtimeCtx := &defaultRuntimeContext{
		runID:       run.ID,
		ctx:         runCtx,
		cancel:      cancel,
		repo:        m.repo,
		eventBus:    m.eventBus,
		logger:      newNoopRuntimeLogger(),
		snapshotHub: m.snapshotHub,
	}

	enableRawLog := extractEnableRawLog(def)
	if store, err := report.NewExecutionLogStore(run.Name, time.Now()); err != nil {
		logger.Error("TaskExec", run.ID, "创建执行日志存储失败: %v", err)
	} else {
		runtimeCtx.logStore = store
		factory := NewDefaultLoggerFactory(config.GetPathManager().GetStorageRoot(), store, enableRawLog)
		runtimeCtx.logger = NewDefaultRuntimeLogger(factory)
		logger.Debug("TaskExec", run.ID, "执行日志存储已创建: raw=%t", enableRawLog)
		m.mu.Lock()
		m.logStores[run.ID] = store
		m.runningRuns[run.ID] = runtimeCtx
		m.mu.Unlock()
		goto schedule
	}

	if m.loggerFactory != nil {
		runtimeCtx.logger = NewDefaultRuntimeLogger(m.loggerFactory)
	}

	// 注册到运行中集合
	m.mu.Lock()
	m.runningRuns[run.ID] = runtimeCtx
	m.mu.Unlock()

schedule:
	// 异步执行
	go m.executePlan(runtimeCtx, run, plan)

	return run, nil
}

// executePlan 执行计划
func (m *RuntimeManager) executePlan(runtimeCtx *defaultRuntimeContext, run *TaskRun, plan *ExecutionPlan) {
	handler := NewErrorHandler(run.ID)
	defer func() {
		m.mu.Lock()
		delete(m.runningRuns, run.ID)
		m.mu.Unlock()
	}()

	// 更新状态为运行中
	now := time.Now()
	if err := handler.UpdateRunRequired(runtimeCtx, &RunPatch{
		Status:    strPtr(string(RunStatusRunning)),
		StartedAt: &now,
	}, "更新运行状态为 running"); err != nil {
		m.emitError(run.ID, "", "", fmt.Sprintf("更新运行状态失败: %v", err))
		return
	}

	// 发射开始事件
	m.eventBus.EmitSync(NewTaskEvent(run.ID, EventTypeRunStarted, "任务开始执行"))

	// 记录每个阶段的执行结果
	stageResults := make(map[string]error)

	// 按顺序执行Stage
	for i, stagePlan := range plan.Stages {
		if runtimeCtx.IsCancelled() {
			m.handleCancellation(runtimeCtx, run.ID)
			return
		}

		// 检查阶段依赖
		if shouldSkip, reason := m.checkStageDependencies(plan, stagePlan, stageResults); shouldSkip {
			logger.Warn("TaskExec", run.ID, "跳过 Stage %s: %s", stagePlan.Name, reason)
			m.skipStage(runtimeCtx, run.ID, &stagePlan, reason)
			stageResults[stagePlan.Kind] = fmt.Errorf("skipped: %s", reason)
			continue
		}

		// 更新当前Stage
		handler.UpdateRunBestEffort(runtimeCtx, &RunPatch{
			CurrentStage: &stagePlan.Kind,
		}, "更新当前阶段")

		// 执行Stage
		if err := m.executeStage(runtimeCtx, run.ID, &stagePlan, i, len(plan.Stages)); err != nil {
			stageResults[stagePlan.Kind] = err
			m.handleStageError(runtimeCtx, run.ID, stagePlan.ID, err)

			// 对于拓扑任务，关键阶段失败则中止
			if plan.RunKind == string(RunKindTopology) {
				switch StageKind(stagePlan.Kind) {
				case StageKindDeviceCollect, StageKindParse:
					logger.Error("TaskExec", run.ID, "拓扑任务关键阶段 %s 失败，中止执行", stagePlan.Name)
					m.abortPlan(runtimeCtx, run.ID, fmt.Sprintf("关键阶段 %s 失败", stagePlan.Name))
					return
				}
			}
		} else {
			stageResults[stagePlan.Kind] = nil
		}

		m.refreshRunProgress(runtimeCtx, run.ID)
	}

	if runtimeCtx.IsCancelled() {
		m.handleCancellation(runtimeCtx, run.ID)
		return
	}

	// 计算最终状态
	finalStatus := m.calculateFinalStatus(runtimeCtx, run.ID)
	finishedAt := time.Now()
	handler.UpdateRunBestEffort(runtimeCtx, &RunPatch{
		Status:     &finalStatus,
		FinishedAt: &finishedAt,
	}, "写入运行终态")

	// 发射完成事件
	m.eventBus.EmitSync(NewTaskEvent(run.ID, EventTypeRunFinished, fmt.Sprintf("任务完成，状态: %s", finalStatus)))
}

func (m *RuntimeManager) enrichSnapshotWithLogs(runID string, snapshot *ExecutionSnapshot) {
	if snapshot == nil {
		return
	}

	m.mu.RLock()
	store := m.logStores[runID]
	m.mu.RUnlock()
	enrichSnapshotWithStore(store, snapshot)
}

func enrichSnapshotWithStore(store *report.ExecutionLogStore, snapshot *ExecutionSnapshot) {
	if store == nil {
		return
	}

	maxLogs := 30
	if manager := config.GetRuntimeManagerIfInitialized(); manager != nil {
		if configured := manager.GetMaxLogsPerDevice(); configured > 0 {
			maxLogs = configured
		}
	}

	for idx := range snapshot.Units {
		unit := &snapshot.Units[idx]
		if unit.TargetType != "device_ip" || unit.TargetKey == "" {
			continue
		}

		logs, logCount := storeLogsForUnit(store, unit.TargetKey, maxLogs)
		paths := store.GetDeviceLogPaths(unit.TargetKey)

		unit.Logs = logs
		unit.LogCount = logCount
		unit.Truncated = logCount > len(logs)
		unit.SummaryLogPath = paths.SummaryPath
		unit.DetailLogPath = paths.DetailPath
		unit.RawLogPath = paths.RawPath

		if logCount > 0 {
			logger.Verbose("TaskExecLog", unit.TargetKey, "快照附加日志: run=%s, lines=%d, tail=%d, summary=%t, detail=%t, raw=%t",
				snapshot.RunID, logCount, len(logs), unit.SummaryLogPath != "", unit.DetailLogPath != "", unit.RawLogPath != "")
		}
	}
}

func storeLogsForUnit(store *report.ExecutionLogStore, unitKey string, maxLogs int) ([]string, int) {
	if store == nil || unitKey == "" || maxLogs <= 0 {
		return nil, 0
	}

	summaryCount := store.GetSummaryLogCount(unitKey)
	if summaryCount > 0 {
		logs, err := store.GetSummaryLastLogs(unitKey, maxLogs)
		if err != nil {
			logger.Warn("TaskExecLog", unitKey, "读取摘要日志尾部失败: %v", err)
			return nil, summaryCount
		}
		logger.Verbose("TaskExecLog", unitKey, "任务执行大屏使用 summary.log 尾部: lines=%d, tail=%d", summaryCount, len(logs))
		return logs, summaryCount
	}

	return nil, 0
}

func extractEnableRawLog(def *TaskDefinition) bool {
	if def == nil || len(def.Config) == 0 {
		return false
	}

	switch stringsTrim(def.Kind) {
	case string(RunKindTopology):
		var cfg TopologyTaskConfig
		if err := json.Unmarshal(def.Config, &cfg); err != nil {
			logger.Warn("TaskExec", "-", "解析拓扑任务 raw 日志配置失败: %v", err)
			return false
		}
		return cfg.EnableRawLog
	default:
		var cfg NormalTaskConfig
		if err := json.Unmarshal(def.Config, &cfg); err != nil {
			logger.Warn("TaskExec", "-", "解析普通任务 raw 日志配置失败: %v", err)
			return false
		}
		return cfg.EnableRawLog
	}
}

func stringsTrim(value string) string {
	if value == "" {
		return ""
	}
	return strings.TrimSpace(value)
}

// checkStageDependencies 检查阶段依赖
func (m *RuntimeManager) checkStageDependencies(plan *ExecutionPlan, stage StagePlan, results map[string]error) (bool, string) {
	// 拓扑任务的阶段依赖定义
	if plan.RunKind != string(RunKindTopology) {
		return false, ""
	}

	// 解析阶段依赖采集阶段
	if StageKind(stage.Kind) == StageKindParse {
		if err, ok := results[string(StageKindDeviceCollect)]; ok && err != nil {
			return true, fmt.Sprintf("依赖阶段 %s 执行失败", StageKindDeviceCollect)
		}
	}

	// 拓扑构建阶段依赖解析阶段
	if StageKind(stage.Kind) == StageKindTopologyBuild {
		if err, ok := results[string(StageKindParse)]; ok && err != nil {
			return true, fmt.Sprintf("依赖阶段 %s 执行失败", StageKindParse)
		}
	}

	return false, ""
}

// skipStage 跳过阶段
func (m *RuntimeManager) skipStage(runtimeCtx *defaultRuntimeContext, runID string, stagePlan *StagePlan, reason string) {
	// 创建跳过的 Stage 记录
	stage := &TaskRunStage{
		ID:         newStageID(),
		TaskRunID:  runID,
		StageKind:  stagePlan.Kind,
		StageName:  stagePlan.Name,
		StageOrder: stagePlan.Order,
		Status:     string(StageStatusSkipped),
		TotalUnits: 0,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	m.repo.CreateStage(runtimeCtx.ctx, stage)

	m.eventBus.EmitSync(NewTaskEvent(runID, EventTypeStageSkipped,
		fmt.Sprintf("Stage %s 被跳过: %s", stagePlan.Name, reason)).WithStage(stage.ID))
}

// abortPlan 中止计划执行
func (m *RuntimeManager) abortPlan(runtimeCtx *defaultRuntimeContext, runID, reason string) {
	handler := NewErrorHandler(runID)
	abortedStatus := string(RunStatusAborted)
	now := time.Now()
	handler.UpdateRunBestEffort(runtimeCtx, &RunPatch{
		Status:     &abortedStatus,
		FinishedAt: &now,
	}, "写入运行中止状态")
	m.eventBus.EmitSync(NewTaskEvent(runID, EventTypeRunAborted, fmt.Sprintf("任务中止: %s", reason)))
}

// executeStage 执行Stage
func (m *RuntimeManager) executeStage(runtimeCtx *defaultRuntimeContext, runID string, stagePlan *StagePlan, index, total int) error {
	handler := NewErrorHandler(runID)
	startedAt := time.Now()
	// 创建Stage记录
	stage := &TaskRunStage{
		ID:         newStageID(),
		TaskRunID:  runID,
		StageKind:  stagePlan.Kind,
		StageName:  stagePlan.Name,
		StageOrder: stagePlan.Order,
		Status:     string(StageStatusRunning),
		TotalUnits: len(stagePlan.Units),
		StartedAt:  &startedAt,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	if err := m.repo.CreateStage(runtimeCtx.ctx, stage); err != nil {
		return fmt.Errorf("创建Stage记录失败: %w", err)
	}

	// 发射Stage开始事件
	m.eventBus.EmitSync(NewTaskEvent(runID, EventTypeStageStarted, fmt.Sprintf("Stage %s 开始", stagePlan.Name)).WithStage(stage.ID))

	// 为执行器创建运行时Unit记录，并传递运行时Stage/Unit ID
	execStagePlan, err := m.materializeStageUnits(runtimeCtx, runID, stage.ID, stagePlan)
	if err != nil {
		failedStatus := string(StageStatusFailed)
		now := time.Now()
		handler.UpdateStageBestEffort(runtimeCtx, stage.ID, &StagePatch{
			Status:     &failedStatus,
			FinishedAt: &now,
		}, "创建 Unit 记录失败后标记 Stage 失败")
		return fmt.Errorf("创建Stage Unit记录失败: %w", err)
	}

	// 查找执行器
	executor, ok := m.executorReg.Get(stagePlan.Kind)
	if !ok {
		failedStatus := string(StageStatusFailed)
		now := time.Now()
		handler.UpdateStageBestEffort(runtimeCtx, stage.ID, &StagePatch{
			Status:     &failedStatus,
			FinishedAt: &now,
		}, "Stage 执行器不存在时标记失败")
		return fmt.Errorf("未找到Stage执行器: %s", stagePlan.Kind)
	}

	// 执行Stage
	if err := executor.Run(runtimeCtx, execStagePlan); err != nil {
		if IsContextCancelled(runtimeCtx, err) {
			cancelledStatus := string(StageStatusCancelled)
			now := time.Now()
			handler.UpdateStageBestEffort(runtimeCtx, stage.ID, &StagePatch{
				Status:     &cancelledStatus,
				FinishedAt: &now,
			}, "Stage 取消后写入取消状态")
			return err
		}
		failedStatus := string(StageStatusFailed)
		now := time.Now()
		handler.UpdateStageBestEffort(runtimeCtx, stage.ID, &StagePatch{
			Status:     &failedStatus,
			FinishedAt: &now,
		}, "Stage 执行失败后标记失败")
		return err
	}

	// 基于Unit结果汇总Stage终态
	m.reconcileStageStatus(runtimeCtx, stage.ID)

	// 发射Stage完成事件
	m.eventBus.EmitSync(NewTaskEvent(runID, EventTypeStageFinished, fmt.Sprintf("Stage %s 完成", stagePlan.Name)).WithStage(stage.ID))

	return nil
}

func (m *RuntimeManager) materializeStageUnits(runtimeCtx *defaultRuntimeContext, runID, runtimeStageID string, stagePlan *StagePlan) (*StagePlan, error) {
	stage := *stagePlan
	stage.ID = runtimeStageID
	stage.Units = make([]UnitPlan, 0, len(stagePlan.Units))

	now := time.Now()
	for _, unitPlan := range stagePlan.Units {
		runtimeUnit := &TaskRunUnit{
			ID:             newUnitID(),
			TaskRunID:      runID,
			TaskRunStageID: runtimeStageID,
			UnitKind:       unitPlan.Kind,
			TargetType:     unitPlan.Target.Type,
			TargetKey:      unitPlan.Target.Key,
			Status:         string(UnitStatusPending),
			TotalSteps:     len(unitPlan.Steps),
			DoneSteps:      0,
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		if err := m.repo.CreateUnit(runtimeCtx.ctx, runtimeUnit); err != nil {
			return nil, err
		}

		execUnit := unitPlan
		execUnit.ID = runtimeUnit.ID
		stage.Units = append(stage.Units, execUnit)
	}
	return &stage, nil
}

func (m *RuntimeManager) reconcileStageStatus(runtimeCtx *defaultRuntimeContext, stageID string) {
	handler := NewErrorHandler(runtimeCtx.runID)
	units, err := m.repo.GetUnitsByStage(runtimeCtx.ctx, stageID)
	if err != nil {
		handler.LogUpdateError("读取 Stage 单元用于状态汇总", err)
		return
	}

	total := len(units)
	completed := 0
	success := 0
	failed := 0
	cancelled := 0
	for _, unit := range units {
		switch UnitStatus(unit.Status) {
		case UnitStatusCompleted:
			completed++
			success++
		case UnitStatusFailed:
			completed++
			failed++
		case UnitStatusCancelled:
			completed++
			cancelled++
		case UnitStatusPartial:
			completed++
		}
	}

	progress := 0
	if total > 0 {
		progress = completed * 100 / total
	}

	status := string(StageStatusCompleted)
	if total == 0 {
		status = string(StageStatusCompleted)
	} else if cancelled == total {
		status = string(StageStatusCancelled)
	} else if failed > 0 && success == 0 && cancelled == 0 {
		status = string(StageStatusFailed)
	} else if failed > 0 || cancelled > 0 {
		status = string(StageStatusPartial)
	}
	if runtimeCtx.IsCancelled() && completed < total {
		if cancelled+failed == total && cancelled > 0 {
			status = string(StageStatusCancelled)
		} else {
			status = string(StageStatusPartial)
		}
	}
	if total == 0 && runtimeCtx.IsCancelled() {
		status = string(StageStatusCancelled)
	}

	now := time.Now()
	handler.UpdateStageBestEffort(runtimeCtx, stageID, &StagePatch{
		Status:         &status,
		Progress:       &progress,
		CompletedUnits: &completed,
		SuccessUnits:   &success,
		FailedUnits:    &failed,
		CancelledUnits: &cancelled,
		FinishedAt:     &now,
	}, "汇总 Stage 状态")
}

func (m *RuntimeManager) refreshRunProgress(runtimeCtx *defaultRuntimeContext, runID string) {
	handler := NewErrorHandler(runtimeCtx.runID)
	stages, err := m.repo.GetStagesByRun(runtimeCtx.ctx, runID)
	if err != nil || len(stages) == 0 {
		if err != nil {
			handler.LogUpdateError("刷新 Run 进度时读取 Stage 列表", err)
		}
		return
	}

	progress := 0
	for _, stage := range stages {
		progress += stage.Progress
	}
	progress = progress / len(stages)
	handler.UpdateRunBestEffort(runtimeCtx, &RunPatch{Progress: &progress}, "刷新运行进度")
}

// handleCancellation 处理取消
func (m *RuntimeManager) handleCancellation(runtimeCtx *defaultRuntimeContext, runID string) {
	handler := NewErrorHandler(runID)
	cancelledStatus := string(RunStatusCancelled)
	now := time.Now()
	handler.UpdateRunBestEffort(runtimeCtx, &RunPatch{
		Status:     &cancelledStatus,
		FinishedAt: &now,
	}, "写入运行取消状态")

	stages, err := m.repo.GetStagesByRun(runtimeCtx.ctx, runID)
	if err != nil {
		handler.LogUpdateError("取消时读取 Stage 列表", err)
		m.eventBus.EmitSync(NewTaskEvent(runID, EventTypeRunFinished, "任务已取消"))
		return
	}

	for _, stage := range stages {
		status := StageStatus(stage.Status)
		if status.IsTerminal() {
			continue
		}
		units, unitErr := m.repo.GetUnitsByStage(runtimeCtx.ctx, stage.ID)
		if unitErr != nil {
			handler.LogUpdateError(fmt.Sprintf("取消时读取 Stage[%s] Unit 列表", stage.ID), unitErr)
			continue
		}

		cancelled := 0
		failed := 0
		success := 0
		completed := 0
		for _, unit := range units {
			unitStatus := UnitStatus(unit.Status)
			if !unitStatus.IsTerminal() {
				reason := "run cancelled"
				handler.MarkUnitCancelled(runtimeCtx, unit.ID, reason, &unit.DoneSteps)
				unitStatus = UnitStatusCancelled
			}
			switch unitStatus {
			case UnitStatusCompleted:
				completed++
				success++
			case UnitStatusPartial:
				completed++
			case UnitStatusFailed:
				completed++
				failed++
			case UnitStatusCancelled:
				completed++
				cancelled++
			}
		}

		progress := 0
		if stage.TotalUnits > 0 {
			progress = completed * 100 / stage.TotalUnits
		}

		stageStatus := string(StageStatusCancelled)
		if failed > 0 || success > 0 {
			stageStatus = string(StageStatusPartial)
		}
		finishedAt := time.Now()
		handler.UpdateStageBestEffort(runtimeCtx, stage.ID, &StagePatch{
			Status:         &stageStatus,
			Progress:       &progress,
			CompletedUnits: &completed,
			SuccessUnits:   &success,
			FailedUnits:    &failed,
			CancelledUnits: &cancelled,
			FinishedAt:     &finishedAt,
		}, fmt.Sprintf("取消时汇总 Stage[%s]", stage.ID))
	}

	m.eventBus.EmitSync(NewTaskEvent(runID, EventTypeRunFinished, "任务已取消"))
}

// handleStageError 处理Stage错误
func (m *RuntimeManager) handleStageError(runtimeCtx *defaultRuntimeContext, runID, stageID string, err error) {
	m.emitError(runID, stageID, "", fmt.Sprintf("Stage执行失败: %v", err))
}

// calculateFinalStatus 计算最终状态
func (m *RuntimeManager) calculateFinalStatus(runtimeCtx *defaultRuntimeContext, runID string) string {
	stages, err := m.repo.GetStagesByRun(runtimeCtx.ctx, runID)
	if err != nil {
		return string(RunStatusFailed)
	}

	var failedCount, partialCount, cancelledCount int
	for _, stage := range stages {
		switch stage.Status {
		case string(StageStatusFailed):
			failedCount++
		case string(StageStatusPartial):
			partialCount++
		case string(StageStatusCancelled):
			cancelledCount++
		}
	}

	if cancelledCount == len(stages) && len(stages) > 0 {
		return string(RunStatusCancelled)
	}
	if failedCount == len(stages) && len(stages) > 0 {
		return string(RunStatusFailed)
	}
	if failedCount > 0 || partialCount > 0 || cancelledCount > 0 {
		return string(RunStatusPartial)
	}
	return string(RunStatusCompleted)
}

// CancelRun 取消运行
func (m *RuntimeManager) CancelRun(runID string) error {
	m.mu.RLock()
	runtimeCtx, ok := m.runningRuns[runID]
	m.mu.RUnlock()

	if ok {
		runtimeCtx.cancel()
		logger.Info("TaskExec", runID, "取消运行实例")
		return nil
	}

	run, err := m.repo.GetRun(context.Background(), runID)
	if err != nil {
		return fmt.Errorf("运行实例不存在: %s", runID)
	}
	if RunStatus(run.Status).IsTerminal() {
		return nil
	}

	cancelledStatus := string(RunStatusCancelled)
	now := time.Now()
	if updateErr := m.repo.UpdateRun(context.Background(), runID, &RunPatch{Status: &cancelledStatus, FinishedAt: &now}); updateErr != nil {
		return fmt.Errorf("补偿取消运行失败: %w", updateErr)
	}

	stages, _ := m.repo.GetStagesByRun(context.Background(), runID)
	for _, stage := range stages {
		stageStatus := string(StageStatusCancelled)
		finishedAt := now
		_ = m.repo.UpdateStage(context.Background(), stage.ID, &StagePatch{Status: &stageStatus, FinishedAt: &finishedAt})
		units, _ := m.repo.GetUnitsByStage(context.Background(), stage.ID)
		for _, unit := range units {
			if UnitStatus(unit.Status).IsTerminal() {
				continue
			}
			unitStatus := string(UnitStatusCancelled)
			reason := "run cancelled by compensation"
			_ = m.repo.UpdateUnit(context.Background(), unit.ID, &UnitPatch{Status: &unitStatus, ErrorMessage: &reason, FinishedAt: &finishedAt})
		}
	}

	m.emitError(runID, "", "", "运行实例不在内存中，已执行补偿取消")
	return nil
}

// emitError 发射错误事件
func (m *RuntimeManager) emitError(runID, stageID, unitID, message string) {
	event := NewTaskEvent(runID, EventTypeError, message)
	if stageID != "" {
		event.WithStage(stageID)
	}
	if unitID != "" {
		event.WithUnit(unitID)
	}
	m.eventBus.EmitSync(event)
}

func strPtr(v string) *string {
	return &v
}
