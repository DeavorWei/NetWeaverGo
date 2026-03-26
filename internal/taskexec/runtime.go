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

	if !c.applyRunPatchToSnapshot(patch) {
		c.refreshSnapshot()
	}
	return nil
}

// UpdateStage 更新Stage状态
func (c *defaultRuntimeContext) UpdateStage(stageID string, patch *StagePatch) error {
	if err := c.repo.UpdateStage(c.ctx, stageID, patch); err != nil {
		return err
	}
	if !c.applyStagePatchToSnapshot(stageID, patch) {
		c.refreshSnapshot()
	}
	return nil
}

// UpdateUnit 更新Unit状态
func (c *defaultRuntimeContext) UpdateUnit(unitID string, patch *UnitPatch) error {
	if err := c.repo.UpdateUnit(c.ctx, unitID, patch); err != nil {
		return err
	}
	if !c.applyUnitPatchToSnapshot(unitID, patch) {
		c.refreshSnapshot()
	}
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

func (c *defaultRuntimeContext) applyRunPatchToSnapshot(patch *RunPatch) bool {
	if c.snapshotHub == nil || patch == nil {
		return false
	}
	if c.snapshotHub.ApplyRunPatch(c.runID, patch) {
		logger.Verbose("TaskExec", c.runID, "增量更新 Run 快照成功: status=%t currentStage=%t progress=%t started=%t finished=%t",
			patch.Status != nil, patch.CurrentStage != nil, patch.Progress != nil, patch.StartedAt != nil, patch.FinishedAt != nil)
		return true
	}
	logger.Verbose("TaskExec", c.runID, "增量更新 Run 快照未命中，回退到全量重建")
	return false
}

func (c *defaultRuntimeContext) applyStagePatchToSnapshot(stageID string, patch *StagePatch) bool {
	if c.snapshotHub == nil || patch == nil {
		return false
	}
	if c.snapshotHub.ApplyStagePatch(c.runID, stageID, patch) {
		logger.Verbose("TaskExec", c.runID, "增量更新 Stage 快照成功: stage=%s, status=%t progress=%t completed=%t success=%t failed=%t cancelled=%t",
			stageID, patch.Status != nil, patch.Progress != nil, patch.CompletedUnits != nil, patch.SuccessUnits != nil, patch.FailedUnits != nil, patch.CancelledUnits != nil)
		return true
	}
	logger.Verbose("TaskExec", c.runID, "增量更新 Stage 快照未命中，回退到全量重建: stage=%s", stageID)
	return false
}

func (c *defaultRuntimeContext) applyUnitPatchToSnapshot(unitID string, patch *UnitPatch) bool {
	if c.snapshotHub == nil || patch == nil {
		return false
	}
	if c.snapshotHub.ApplyUnitPatch(c.runID, unitID, patch) {
		logger.Verbose("TaskExec", c.runID, "增量更新 Unit 快照成功: unit=%s, status=%t doneSteps=%t err=%t started=%t finished=%t",
			unitID, patch.Status != nil, patch.DoneSteps != nil, patch.ErrorMessage != nil, patch.StartedAt != nil, patch.FinishedAt != nil)
		return true
	}
	logger.Verbose("TaskExec", c.runID, "增量更新 Unit 快照未命中，回退到全量重建: unit=%s", unitID)
	return false
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
	logger.Verbose("TaskExec", c.runID, "通过仓库重建快照: stages=%d, units=%d, events=%d", len(stages), len(units), len(events))
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
	manager := &RuntimeManager{
		repo:        repo,
		eventBus:    eventBus,
		snapshotHub: snapshotHub,
		executorReg: NewExecutorRegistry(),
		runningRuns: make(map[string]*defaultRuntimeContext),
		logStores:   make(map[string]*report.ExecutionLogStore),
	}
	if repo != nil && eventBus != nil {
		eventBus.Subscribe(NewTaskEventRepositoryProjector(repo))
	}
	return manager
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
		m.enrichSnapshotWithLogs(runID, snapshot)
		return snapshot, nil
	}
	return m.rebuildSnapshotFromRepo(runID)
}

// GetSnapshotDelta 获取指定运行的快照增量。
func (m *RuntimeManager) GetSnapshotDelta(runID string) (*SnapshotDelta, error) {
	if m.snapshotHub == nil {
		return nil, fmt.Errorf("snapshot hub not initialized")
	}
	if delta, ok := m.snapshotHub.BuildDelta(runID); ok && delta != nil {
		if delta.Snapshot != nil {
			m.enrichSnapshotWithLogs(runID, delta.Snapshot)
		}
		return delta, nil
	}
	if _, err := m.rebuildSnapshotFromRepo(runID); err != nil {
		return nil, err
	}
	delta, ok := m.snapshotHub.BuildDelta(runID)
	if !ok || delta == nil {
		return nil, fmt.Errorf("snapshot delta not found: %s", runID)
	}
	if delta.Snapshot != nil {
		m.enrichSnapshotWithLogs(runID, delta.Snapshot)
	}
	return delta, nil
}

func (m *RuntimeManager) rebuildSnapshotFromRepo(runID string) (*ExecutionSnapshot, error) {
	run, err := m.repo.GetRun(context.Background(), runID)
	if err != nil || run == nil {
		return nil, fmt.Errorf("snapshot not found: %s", runID)
	}
	stages, _ := m.repo.GetStagesByRun(context.Background(), runID)
	units, _ := m.repo.GetUnitsByRun(context.Background(), runID)
	events, _ := m.repo.GetEventsByRun(context.Background(), runID, 50)
	builder := NewSnapshotBuilder()
	snapshot := builder.Build(run, stages, units, events)
	m.enrichSnapshotWithLogs(runID, snapshot)
	m.snapshotHub.Update(runID, snapshot)
	logger.Verbose("TaskExec", runID, "已从仓库重建快照: stages=%d, units=%d, events=%d", len(stages), len(units), len(events))
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
	if m.snapshotHub != nil {
		m.snapshotHub.EnsureRun(run)
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
	emitProjectedRunEvent(runtimeCtx, EventTypeRunStarted, EventLevelInfo, "任务开始执行")

	// 记录每个阶段的执行结果
	stageResults := make(map[string]error)

	// 按顺序执行Stage
	for i, stagePlan := range plan.Stages {
		if runtimeCtx.IsCancelled() {
			m.handleCancellation(runtimeCtx, run.ID)
			return
		}

		// 检查阶段依赖
		if shouldSkip, reason := evaluateStageDependencyPolicy(plan.RunKind, stagePlan, stageResults); shouldSkip {
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

			if shouldAbort, reason := evaluateStageFailurePolicy(plan.RunKind, stagePlan, err); shouldAbort {
				logger.Error("TaskExec", run.ID, "阶段失败触发计划中止: stage=%s, reason=%s", stagePlan.Name, reason)
				m.abortPlan(runtimeCtx, run.ID, reason)
				return
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
	finishRunWithStatus(handler, runtimeCtx, finalStatus, "写入运行终态")

	// 发射完成事件
	emitProjectedRunEvent(runtimeCtx, EventTypeRunFinished, EventLevelInfo, fmt.Sprintf("任务完成，状态: %s", finalStatus))
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
		unit.JournalLogPath = paths.JournalPath

		if logCount > 0 {
			logger.Verbose("TaskExecLog", unit.TargetKey, "快照附加日志: run=%s, lines=%d, tail=%d, summary=%t, detail=%t, raw=%t, journal=%t",
				snapshot.RunID, logCount, len(logs), unit.SummaryLogPath != "", unit.DetailLogPath != "", unit.RawLogPath != "", unit.JournalLogPath != "")
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
	if m.snapshotHub != nil {
		m.snapshotHub.UpsertStage(runID, stage)
	}

	emitProjectedStageEvent(runtimeCtx, stage.ID, EventTypeStageSkipped, EventLevelWarn,
		fmt.Sprintf("Stage %s 被跳过: %s", stagePlan.Name, reason))
}

// abortPlan 中止计划执行
func (m *RuntimeManager) abortPlan(runtimeCtx *defaultRuntimeContext, runID, reason string) {
	handler := NewErrorHandler(runID)
	finishRunWithStatus(handler, runtimeCtx, string(RunStatusAborted), "写入运行中止状态")
	emitProjectedRunEvent(runtimeCtx, EventTypeRunAborted, EventLevelError, fmt.Sprintf("任务中止: %s", reason))
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
	if m.snapshotHub != nil {
		m.snapshotHub.UpsertStage(runID, stage)
	}

	// 发射Stage开始事件
	emitProjectedStageEvent(runtimeCtx, stage.ID, EventTypeStageStarted, EventLevelInfo, fmt.Sprintf("Stage %s 开始", stagePlan.Name))

	// 为执行器创建运行时Unit记录，并传递运行时Stage/Unit ID
	execStagePlan, err := m.materializeStageUnits(runtimeCtx, runID, stage.ID, stagePlan)
	if err != nil {
		finishStageWithStatus(handler, runtimeCtx, stage.ID, string(StageStatusFailed), "创建 Unit 记录失败后标记 Stage 失败")
		return fmt.Errorf("创建Stage Unit记录失败: %w", err)
	}

	// 查找执行器
	executor, ok := m.executorReg.Get(stagePlan.Kind)
	if !ok {
		finishStageWithStatus(handler, runtimeCtx, stage.ID, string(StageStatusFailed), "Stage 执行器不存在时标记失败")
		return fmt.Errorf("未找到Stage执行器: %s", stagePlan.Kind)
	}

	// 执行Stage
	if err := executor.Run(runtimeCtx, execStagePlan); err != nil {
		if IsContextCancelled(runtimeCtx, err) {
			finishStageWithStatus(handler, runtimeCtx, stage.ID, string(StageStatusCancelled), "Stage 取消后写入取消状态")
			return err
		}
		finishStageWithStatus(handler, runtimeCtx, stage.ID, string(StageStatusFailed), "Stage 执行失败后标记失败")
		return err
	}

	// 基于Unit结果汇总Stage终态
	m.reconcileStageStatus(runtimeCtx, stage.ID)

	// 发射Stage完成事件
	emitProjectedStageEvent(runtimeCtx, stage.ID, EventTypeStageFinished, EventLevelInfo, fmt.Sprintf("Stage %s 完成", stagePlan.Name))

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
		if m.snapshotHub != nil {
			m.snapshotHub.UpsertUnit(runID, runtimeUnit)
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
	applyProjectedStageCompletion(handler, runtimeCtx, stageID, units, runtimeCtx.IsCancelled(), time.Now(), "汇总 Stage 状态")
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
	progress := projectRunProgressFromStages(stages)
	applyProjectedRunProgress(handler, runtimeCtx, progress, "刷新运行进度")
}

// handleCancellation 处理取消
func (m *RuntimeManager) handleCancellation(runtimeCtx *defaultRuntimeContext, runID string) {
	handler := NewErrorHandler(runID)
	finishRunWithStatus(handler, runtimeCtx, string(RunStatusCancelled), "写入运行取消状态")

	stages, err := m.repo.GetStagesByRun(runtimeCtx.ctx, runID)
	if err != nil {
		handler.LogUpdateError("取消时读取 Stage 列表", err)
		emitProjectedRunEvent(runtimeCtx, EventTypeRunFinished, EventLevelWarn, "任务已取消")
		return
	}

	m.projectCancellationToStages(runtimeCtx, handler, stages)
	emitProjectedRunEvent(runtimeCtx, EventTypeRunFinished, EventLevelWarn, "任务已取消")
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
	return projectRunFinalStatus(stages)
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

	now := time.Now()
	if updateErr := applyCompensationCancellation(m.repo, runID, now); updateErr != nil {
		return fmt.Errorf("补偿取消运行失败: %w", updateErr)
	}
	if m.snapshotHub != nil {
		if snapshot, getErr := m.rebuildSnapshotFromRepo(runID); getErr == nil && snapshot != nil {
			logger.Verbose("TaskExec", runID, "补偿取消后已刷新快照缓存")
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

func finishRunWithStatus(handler *ErrorHandler, runtimeCtx *defaultRuntimeContext, status string, operation string) {
	now := time.Now()
	handler.UpdateRunBestEffort(runtimeCtx, &RunPatch{
		Status:     &status,
		FinishedAt: &now,
	}, operation)
}

func finishStageWithStatus(handler *ErrorHandler, runtimeCtx *defaultRuntimeContext, stageID, status string, operation string) {
	now := time.Now()
	handler.UpdateStageBestEffort(runtimeCtx, stageID, &StagePatch{
		Status:     &status,
		FinishedAt: &now,
	}, operation)
}

func (m *RuntimeManager) projectCancellationToStages(runtimeCtx *defaultRuntimeContext, handler *ErrorHandler, stages []TaskRunStage) {
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
		m.projectCancellationToUnits(runtimeCtx, handler, units)
		applyProjectedStageCompletion(handler, runtimeCtx, stage.ID, units, true, time.Now(), fmt.Sprintf("取消时汇总 Stage[%s]", stage.ID))
	}
}

func (m *RuntimeManager) projectCancellationToUnits(runtimeCtx *defaultRuntimeContext, handler *ErrorHandler, units []TaskRunUnit) {
	for idx := range units {
		unit := &units[idx]
		unitStatus := UnitStatus(unit.Status)
		if unitStatus.IsTerminal() {
			continue
		}
		reason := "run cancelled"
		handler.MarkUnitCancelled(runtimeCtx, unit.ID, reason, &unit.DoneSteps)
		unit.Status = string(UnitStatusCancelled)
		unit.ErrorMessage = reason
		finishedAt := time.Now()
		unit.FinishedAt = &finishedAt
	}
}

func strPtr(v string) *string {
	return &v
}
