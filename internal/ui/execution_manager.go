package ui

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/engine"
	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
	"github.com/NetWeaverGo/core/internal/report"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// ExecutionMeta 执行元数据 - 用于统一标识一次执行
type ExecutionMeta struct {
	RunnerSource  string // task_group / engine_service / backup_service
	RunnerID      string // 运行实例ID，可为空
	TaskGroupID   string // 任务组ID，非任务组执行时为空
	TaskGroupName string // 任务组名称快照
	TaskName      string // 执行任务名称快照
	Mode          string // group / binding / manual / backup
}

type executionManager struct {
	appMu    sync.RWMutex
	wailsApp *application.App

	cancelFunc context.CancelFunc
	cancelMu   sync.Mutex

	currentTracker *report.ProgressTracker
	trackerMu      sync.RWMutex

	snapshotTicker *time.Ticker
	snapshotStop   chan struct{}
	stopOnce       sync.Once
	stopMu         sync.Mutex
}

// ExecutionSession 轻量级执行会话 - 不依赖引擎壳
// 用于复合任务执行场景，只管理会话上下文和进度追踪
type ExecutionSession struct {
	manager   *executionManager
	tracker   *report.ProgressTracker
	ctx       context.Context
	cancel    context.CancelFunc
	meta      *ExecutionMeta // 执行元数据
	cancelled bool           // 是否被用户取消
}

// managedExecution 引擎执行 - 用于单引擎执行场景
type managedExecution struct {
	manager   *executionManager
	lifecycle *engine.Engine
	tracker   *report.ProgressTracker
	ctx       context.Context
	meta      *ExecutionMeta // 执行元数据
	cancelled bool           // 是否被用户取消
}

var sharedExecutionManager = &executionManager{}

func getExecutionManager() *executionManager {
	return sharedExecutionManager
}

func (m *executionManager) SetWailsApp(app *application.App) {
	if app == nil {
		return
	}

	m.appMu.Lock()
	m.wailsApp = app
	m.appMu.Unlock()
}

func (m *executionManager) getWailsApp() *application.App {
	m.appMu.RLock()
	defer m.appMu.RUnlock()
	return m.wailsApp
}

func (m *executionManager) IsRunning() bool {
	return engine.IsEngineRunning()
}

func (m *executionManager) StopEngine() error {
	m.cancelMu.Lock()
	defer m.cancelMu.Unlock()

	if m.cancelFunc == nil {
		return fmt.Errorf("引擎未运行")
	}

	m.cancelFunc()
	m.cancelFunc = nil
	logger.Info("Engine", "-", "已发送停止信号")

	return nil
}

func (m *executionManager) GetEngineState() map[string]interface{} {
	return engine.GetGlobalState().GetStatus()
}

func (m *executionManager) GetExecutionSnapshot() *report.ExecutionSnapshot {
	m.trackerMu.RLock()
	defer m.trackerMu.RUnlock()

	if m.currentTracker == nil {
		return nil
	}

	return m.currentTracker.GetSnapshot()
}

// RunEngineWithMeta 启动引擎执行（带元数据）
func (m *executionManager) RunEngineWithMeta(
	ng *engine.Engine,
	meta *ExecutionMeta,
	runFn func(context.Context) error,
) (*report.ProgressTracker, error) {
	tracker := report.NewProgressTracker(len(ng.Devices))
	if meta != nil && meta.TaskName != "" {
		tracker.SetTaskName(meta.TaskName)
	} else {
		tracker.SetTaskName("任务执行")
	}
	ng.SetTracker(tracker)

	session, err := m.beginExecutionWithMeta(ng, meta, tracker)
	if err != nil {
		return nil, err
	}

	frontendDone := m.listenEvents(ng.FrontendBus, nil)
	runErr := runFn(session.ctx)
	<-frontendDone

	// 判断是否是取消状态
	if runErr != nil && runErr == context.Canceled {
		session.cancelled = true
	}

	session.Finish()

	return tracker, runErr
}

// BeginCompositeExecution 启动轻量级复合执行（不依赖引擎壳）
// 用于多引擎协调执行场景，只管理会话上下文和进度追踪
func (m *executionManager) BeginCompositeExecution(
	meta *ExecutionMeta,
	totalDevices int,
) (*ExecutionSession, error) {
	tracker := report.NewProgressTracker(totalDevices)
	if meta != nil && meta.TaskName != "" {
		tracker.SetTaskName(meta.TaskName)
	} else {
		tracker.SetTaskName("任务执行")
	}

	return m.beginLightweightSession(meta, tracker)
}

// BeginCompositeExecutionWithMeta 启动复合执行（带元数据）- 保留兼容性
func (m *executionManager) BeginCompositeExecutionWithMeta(
	lifecycle *engine.Engine,
	meta *ExecutionMeta,
	totalDevices int,
) (*managedExecution, error) {
	tracker := report.NewProgressTracker(totalDevices)
	if meta != nil && meta.TaskName != "" {
		tracker.SetTaskName(meta.TaskName)
	} else {
		tracker.SetTaskName("任务执行")
	}

	return m.beginExecutionWithMeta(lifecycle, meta, tracker)
}

func (m *executionManager) beginExecutionWithMeta(
	lifecycle *engine.Engine,
	meta *ExecutionMeta,
	tracker *report.ProgressTracker,
) (*managedExecution, error) {
	if lifecycle == nil {
		return nil, fmt.Errorf("引擎实例不能为空")
	}
	if tracker == nil {
		return nil, fmt.Errorf("进度追踪器不能为空")
	}

	runnerSrc := ""
	runnerID := ""
	if meta != nil {
		runnerSrc = meta.RunnerSource
		runnerID = meta.RunnerID
	}

	if err := engine.GetGlobalState().SetActiveEngine(lifecycle, runnerSrc, runnerID); err != nil {
		tracker.Close()
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.setCancelFunc(cancel)
	m.setCurrentTracker(tracker)
	m.startSnapshotTicker(ctx)

	// 立即推送一次初始快照，让前端知道执行已开始
	// 防止任务在第一个快照tick之前完成导致前端卡住
	if app := m.getWailsApp(); app != nil {
		initialSnapshot := tracker.GetSnapshot()
		if initialSnapshot != nil {
			app.Event.Emit(SnapshotEventName, initialSnapshot)
		}
	}

	return &managedExecution{
		manager:   m,
		lifecycle: lifecycle,
		tracker:   tracker,
		ctx:       ctx,
		meta:      meta,
	}, nil
}

// RunEngine 启动引擎执行（兼容旧接口）
func (m *executionManager) RunEngine(
	ng *engine.Engine,
	runnerSrc string,
	runnerID string,
	taskName string,
	runFn func(context.Context) error,
) (*report.ProgressTracker, error) {
	meta := &ExecutionMeta{
		RunnerSource: runnerSrc,
		RunnerID:     runnerID,
		TaskName:     taskName,
		Mode:         "manual",
	}
	return m.RunEngineWithMeta(ng, meta, runFn)
}

// beginLightweightSession 创建轻量级执行会话（不依赖引擎壳）
func (m *executionManager) beginLightweightSession(
	meta *ExecutionMeta,
	tracker *report.ProgressTracker,
) (*ExecutionSession, error) {
	if tracker == nil {
		return nil, fmt.Errorf("进度追踪器不能为空")
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.setCancelFunc(cancel)
	m.setCurrentTracker(tracker)
	m.startSnapshotTicker(ctx)

	// 立即推送一次初始快照，让前端知道执行已开始
	if app := m.getWailsApp(); app != nil {
		initialSnapshot := tracker.GetSnapshot()
		if initialSnapshot != nil {
			app.Event.Emit(SnapshotEventName, initialSnapshot)
		}
	}

	return &ExecutionSession{
		manager: m,
		tracker: tracker,
		ctx:     ctx,
		cancel:  cancel,
		meta:    meta,
	}, nil
}

// BeginCompositeExecutionWithEngine 启动复合执行（兼容旧接口 - 带引擎）
func (m *executionManager) BeginCompositeExecutionWithEngine(
	lifecycle *engine.Engine,
	runnerSrc string,
	runnerID string,
	taskName string,
	totalDevices int,
) (*managedExecution, error) {
	meta := &ExecutionMeta{
		RunnerSource: runnerSrc,
		RunnerID:     runnerID,
		TaskName:     taskName,
		Mode:         "group",
	}
	return m.BeginCompositeExecutionWithMeta(lifecycle, meta, totalDevices)
}

func (m *executionManager) listenEvents(
	frontendBus <-chan report.ExecutorEvent,
	onEvent func(report.ExecutorEvent),
) <-chan struct{} {
	done := make(chan struct{})

	go func() {
		defer close(done)

		for ev := range frontendBus {
			if onEvent != nil {
				onEvent(ev)
			}

			if app := m.getWailsApp(); app != nil {
				app.Event.Emit("device:event", ev)
			}
		}
	}()

	return done
}

func (m *executionManager) setCancelFunc(cancel context.CancelFunc) {
	m.cancelMu.Lock()
	defer m.cancelMu.Unlock()
	m.cancelFunc = cancel
}

func (m *executionManager) clearCancelFunc() {
	m.cancelMu.Lock()
	defer m.cancelMu.Unlock()
	m.cancelFunc = nil
}

func (m *executionManager) setCurrentTracker(tracker *report.ProgressTracker) {
	m.trackerMu.Lock()
	defer m.trackerMu.Unlock()
	m.currentTracker = tracker
}

func (m *executionManager) clearCurrentTracker() {
	m.trackerMu.Lock()
	defer m.trackerMu.Unlock()
	m.currentTracker = nil
}

func (m *executionManager) startSnapshotTicker(ctx context.Context) {
	ticker := time.NewTicker(SnapshotInterval)
	stop := make(chan struct{})

	m.stopMu.Lock()
	m.snapshotTicker = ticker
	m.snapshotStop = stop
	m.stopOnce = sync.Once{}
	m.stopMu.Unlock()

	go func() {
		for {
			select {
			case <-ctx.Done():
				m.stopSnapshotTicker()
				return
			case <-stop:
				return
			case <-ticker.C:
				snapshot := m.GetExecutionSnapshot()
				if snapshot == nil {
					continue
				}
				if app := m.getWailsApp(); app != nil {
					app.Event.Emit(SnapshotEventName, snapshot)
				}
			}
		}
	}()
}

func (m *executionManager) stopSnapshotTicker() {
	m.stopOnce.Do(func() {
		m.stopMu.Lock()
		defer m.stopMu.Unlock()

		if m.snapshotTicker != nil {
			m.snapshotTicker.Stop()
			m.snapshotTicker = nil
		}
		if m.snapshotStop != nil {
			close(m.snapshotStop)
			m.snapshotStop = nil
		}
	})
}

func (m *executionManager) emitFinishedEvent() {
	snapshot := m.GetExecutionSnapshot()
	if snapshot != nil {
		snapshot.IsRunning = false
		if app := m.getWailsApp(); app != nil {
			app.Event.Emit(SnapshotEventName, snapshot)
		}
	}

	if app := m.getWailsApp(); app != nil {
		app.Event.Emit(ExecutionFinishedEvent)
	}
}

// ====== ExecutionSession 方法（轻量级会话）======

func (s *ExecutionSession) Context() context.Context {
	return s.ctx
}

func (s *ExecutionSession) Tracker() *report.ProgressTracker {
	return s.tracker
}

func (s *ExecutionSession) SetCancelled(cancelled bool) {
	s.cancelled = cancelled
}

func (s *ExecutionSession) Finish() {
	// 保存历史记录
	if s.meta != nil && s.tracker != nil {
		s.persistExecutionRecord()
	}

	s.manager.stopSnapshotTicker()
	s.manager.clearCancelFunc()
	s.manager.emitFinishedEvent()
	s.manager.clearCurrentTracker()

	if s.tracker != nil {
		s.tracker.Close()
	}
}

// persistExecutionRecord 持久化执行记录到数据库
func (s *ExecutionSession) persistExecutionRecord() {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("ExecutionManager", "-", "保存历史记录时发生panic: %v", r)
		}
	}()

	if s.tracker == nil || s.meta == nil {
		return
	}

	// 生成CSV报告
	reportPath, _ := s.tracker.ExportCSV()

	// 构建执行摘要
	summary := s.tracker.BuildExecutionSummary()
	deviceData := s.tracker.BuildExecutionDevices(30) // 保留30条日志尾部

	// 判定状态
	status := s.deriveExecutionStatus(summary)

	// 构建设备记录
	devices := make([]models.ExecutionDeviceRecord, 0, len(deviceData))
	for _, d := range deviceData {
		devices = append(devices, models.ExecutionDeviceRecord{
			IP:             d.IP,
			Status:         d.Status,
			TotalCmd:       d.TotalCmd,
			ExecCmd:        d.ExecCmd,
			ErrorMsg:       d.ErrorMsg,
			LogCount:       d.LogCount,
			LogTail:        d.LogTail,
			LogFilePath:    d.LogFilePath,
			SummaryLogPath: d.SummaryLogPath,
			DetailLogPath:  d.DetailLogPath,
			RawLogPath:     d.RawLogPath,
		})
	}

	// 计算时长
	finishedAt := time.Now()
	startedAt := summary.StartedAt
	durationMs := finishedAt.Sub(startedAt).Milliseconds()

	// 创建记录
	record := models.ExecutionRecord{
		RunnerSource:  s.meta.RunnerSource,
		RunnerID:      s.meta.RunnerID,
		TaskGroupID:   s.meta.TaskGroupID,
		TaskGroupName: s.meta.TaskGroupName,
		TaskName:      summary.TaskName,
		Mode:          s.meta.Mode,
		Status:        status,
		TotalDevices:  summary.TotalDevices,
		FinishedCount: summary.FinishedCount,
		SuccessCount:  summary.SuccessCount,
		ErrorCount:    summary.ErrorCount,
		AbortedCount:  summary.AbortedCount,
		WarningCount:  summary.WarningCount,
		StartedAt:     startedAt.Format(time.RFC3339),
		FinishedAt:    finishedAt.Format(time.RFC3339),
		DurationMs:    durationMs,
		ReportPath:    reportPath,
		Devices:       devices,
	}

	// 保存到数据库
	if _, err := config.CreateExecutionRecord(record); err != nil {
		logger.Error("ExecutionManager", "-", "保存历史执行记录失败: %v", err)
	} else {
		logger.Info("ExecutionManager", "-", "历史执行记录已保存: %s", record.ID)
	}

	// 执行保留策略清理（异步）
	go func() {
		if err := config.DeleteOldExecutionRecords(100); err != nil {
			logger.Warn("ExecutionManager", "-", "清理旧历史记录失败: %v", err)
		}
	}()
}

// deriveExecutionStatus 根据执行结果判定历史记录状态
func (s *ExecutionSession) deriveExecutionStatus(summary *report.ExecutionSummaryData) string {
	// 如果用户取消，优先标记为 cancelled
	if s.cancelled {
		return "cancelled"
	}

	total := summary.TotalDevices
	if total == 0 {
		return "failed"
	}

	successAndWarning := summary.SuccessCount + summary.WarningCount
	errorAndAborted := summary.ErrorCount + summary.AbortedCount

	// 全部为成功/告警
	if successAndWarning == total && errorAndAborted == 0 {
		return "completed"
	}

	// 全部为失败/中止
	if errorAndAborted == total {
		return "failed"
	}

	// 部分成功，部分失败
	if successAndWarning > 0 && errorAndAborted > 0 {
		return "partial"
	}

	// 其他情况
	if summary.FinishedCount < total {
		if s.cancelled {
			return "cancelled"
		}
		return "failed"
	}

	return "failed"
}

// ====== managedExecution 方法（引擎执行）======

func (s *managedExecution) Context() context.Context {
	return s.ctx
}

func (s *managedExecution) Tracker() *report.ProgressTracker {
	return s.tracker
}

func (s *managedExecution) SetCancelled(cancelled bool) {
	s.cancelled = cancelled
}

// TransitionTo 已废弃 - 引擎自己管理生命周期
// 保留此方法以兼容旧代码，但不再执行任何操作
func (s *managedExecution) TransitionTo(state engine.EngineState) error {
	// 引擎自己管理生命周期，外部不再推进状态
	return nil
}

func (s *managedExecution) Finish() {
	// 保存历史记录
	if s.meta != nil && s.tracker != nil {
		s.persistExecutionRecord()
	}

	s.manager.stopSnapshotTicker()
	s.manager.clearCancelFunc()
	engine.GetGlobalState().ClearActiveEngine()
	s.manager.emitFinishedEvent()
	s.manager.clearCurrentTracker()

	if s.tracker != nil {
		s.tracker.Close()
	}
}

// persistExecutionRecord 持久化执行记录到数据库
func (s *managedExecution) persistExecutionRecord() {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("ExecutionManager", "-", "保存历史记录时发生panic: %v", r)
		}
	}()

	if s.tracker == nil || s.meta == nil {
		return
	}

	// 生成CSV报告
	reportPath, _ := s.tracker.ExportCSV()

	// 构建执行摘要
	summary := s.tracker.BuildExecutionSummary()
	deviceData := s.tracker.BuildExecutionDevices(30) // 保留30条日志尾部

	// 判定状态
	status := s.deriveExecutionStatus(summary)

	// 构建设备记录
	devices := make([]models.ExecutionDeviceRecord, 0, len(deviceData))
	for _, d := range deviceData {
		devices = append(devices, models.ExecutionDeviceRecord{
			IP:             d.IP,
			Status:         d.Status,
			TotalCmd:       d.TotalCmd,
			ExecCmd:        d.ExecCmd,
			ErrorMsg:       d.ErrorMsg,
			LogCount:       d.LogCount,
			LogTail:        d.LogTail,
			LogFilePath:    d.LogFilePath,
			SummaryLogPath: d.SummaryLogPath,
			DetailLogPath:  d.DetailLogPath,
			RawLogPath:     d.RawLogPath,
		})
	}

	// 计算时长
	finishedAt := time.Now()
	startedAt := summary.StartedAt
	durationMs := finishedAt.Sub(startedAt).Milliseconds()

	// 创建记录
	record := models.ExecutionRecord{
		RunnerSource:  s.meta.RunnerSource,
		RunnerID:      s.meta.RunnerID,
		TaskGroupID:   s.meta.TaskGroupID,
		TaskGroupName: s.meta.TaskGroupName,
		TaskName:      summary.TaskName,
		Mode:          s.meta.Mode,
		Status:        status,
		TotalDevices:  summary.TotalDevices,
		FinishedCount: summary.FinishedCount,
		SuccessCount:  summary.SuccessCount,
		ErrorCount:    summary.ErrorCount,
		AbortedCount:  summary.AbortedCount,
		WarningCount:  summary.WarningCount,
		StartedAt:     startedAt.Format(time.RFC3339),
		FinishedAt:    finishedAt.Format(time.RFC3339),
		DurationMs:    durationMs,
		ReportPath:    reportPath,
		Devices:       devices,
	}

	// 保存到数据库
	if _, err := config.CreateExecutionRecord(record); err != nil {
		logger.Error("ExecutionManager", "-", "保存历史执行记录失败: %v", err)
	} else {
		logger.Info("ExecutionManager", "-", "历史执行记录已保存: %s", record.ID)
	}

	// 执行保留策略清理（异步）
	go func() {
		if err := config.DeleteOldExecutionRecords(100); err != nil {
			logger.Warn("ExecutionManager", "-", "清理旧历史记录失败: %v", err)
		}
	}()
}

// deriveExecutionStatus 根据执行结果判定历史记录状态
func (s *managedExecution) deriveExecutionStatus(summary *report.ExecutionSummaryData) string {
	// 如果用户取消，优先标记为 cancelled
	if s.cancelled {
		return "cancelled"
	}

	// 判定规则：
	// - completed: 所有设备成功或告警放行完成，无失败和中止
	// - partial: 存在成功设备，同时存在失败或中止设备
	// - failed: 所有设备均失败或中止，或没有可成功执行的设备
	// - cancelled: 用户主动停止

	total := summary.TotalDevices
	if total == 0 {
		return "failed"
	}

	successAndWarning := summary.SuccessCount + summary.WarningCount
	errorAndAborted := summary.ErrorCount + summary.AbortedCount

	// 全部为成功/告警
	if successAndWarning == total && errorAndAborted == 0 {
		return "completed"
	}

	// 全部为失败/中止
	if errorAndAborted == total {
		return "failed"
	}

	// 部分成功，部分失败
	if successAndWarning > 0 && errorAndAborted > 0 {
		return "partial"
	}

	// 其他情况（如未完成）
	if summary.FinishedCount < total {
		// 如果是因为取消导致的未完成
		if s.cancelled {
			return "cancelled"
		}
		return "failed"
	}

	return "failed"
}

// finishLifecycle 已废弃 - 引擎自己管理生命周期
// 引擎在 Run() 或 RunBackup() 结束时会自动完成状态转换
