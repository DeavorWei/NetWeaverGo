package ui

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/engine"
	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/report"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// 快照推送配置
const (
	SnapshotInterval       = 200 * time.Millisecond // 快照推送间隔
	SnapshotEventName      = "execution:snapshot"   // 快照事件名称
	ExecutionFinishedEvent = "engine:finished"      // 执行完成事件名称
)

// EngineService 引擎控制服务 - 负责任务执行和状态管理
type EngineService struct {
	wailsApp *application.App

	// Context 取消函数，用于停止正在执行的任务
	cancelFunc context.CancelFunc
	cancelMu   sync.Mutex

	// 当前执行的 Tracker 引用（用于快照查询）
	currentTracker *report.ProgressTracker
	trackerMu      sync.RWMutex

	// 快照推送控制
	snapshotTicker *time.Ticker
	snapshotStop   chan struct{}

	// 事件桥接器
	eventBridge *EventBridge
}

// NewEngineService 创建引擎服务实例
func NewEngineService() *EngineService {
	return &EngineService{
		eventBridge: NewEventBridge(),
	}
}

// ServiceStartup Wails 服务启动生命周期钩子
func (s *EngineService) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	s.wailsApp = application.Get()
	s.eventBridge.SetWailsApp(s.wailsApp)
	// 设置全局 SuspendManager 的 Wails App 实例
	GetSuspendManager().SetWailsApp(s.wailsApp)
	logger.Info("Engine", "-", "引擎控制服务已就绪")
	return nil
}

// IsRunning 检查引擎是否正在运行（使用全局状态）
func (s *EngineService) IsRunning() bool {
	return engine.IsEngineRunning()
}

// StopEngine 停止正在执行的任务
func (s *EngineService) StopEngine() error {
	s.cancelMu.Lock()
	defer s.cancelMu.Unlock()

	if !engine.IsEngineRunning() {
		return fmt.Errorf("引擎未运行")
	}

	if s.cancelFunc != nil {
		s.cancelFunc()
		s.cancelFunc = nil
		logger.Info("Engine", "-", "已发送停止信号")
	}

	return nil
}

// ResolveSuspend 被前端调用（当用户在弹窗中选择动作后）
// 委托给全局 SuspendManager 处理
func (s *EngineService) ResolveSuspend(sessionIDOrIP string, action string) {
	GetSuspendManager().Resolve(sessionIDOrIP, action)
}

// StartEngine 启动核心下发动作
func (s *EngineService) StartEngine() error {
	return s.runEngineWithConfig(nil, "")
}

// StartEngineWithSelection 使用选定的设备和命令组启动引擎
func (s *EngineService) StartEngineWithSelection(deviceIPs []string, commandGroupID string) error {
	return s.runEngineWithConfig(deviceIPs, commandGroupID)
}

// runEngineWithConfig 统一的引擎执行方法
// deviceIPs 为 nil 时使用全部设备，commandGroupID 为空时使用默认命令
func (s *EngineService) runEngineWithConfig(deviceIPs []string, commandGroupID string) error {
	settings, _, err := config.LoadSettings()
	if err != nil {
		return err
	}

	// 准备设备和命令
	assets, commands, err := s.prepareAssetsAndCommands(deviceIPs, commandGroupID)
	if err != nil {
		return err
	}

	if len(assets) == 0 || len(commands) == 0 {
		return fmt.Errorf("资产池或命令集为空。请检查 csv 和 txt 文件！")
	}

	// 初始化 Engine
	ng := engine.NewEngine(assets, commands, settings, false)
	ng.CustomSuspendHandler = GetSuspendManager().CreateHandler()

	// 【修改】设置到全局状态
	if err := engine.GetGlobalState().SetActiveEngine(ng, "engine_service", ""); err != nil {
		return err
	}
	defer engine.GetGlobalState().ClearActiveEngine()

	// 创建并设置 Tracker
	taskName := "批量执行"
	if commandGroupID != "" {
		if group, err := config.GetCommandGroup(commandGroupID); err == nil {
			taskName = group.Name
		}
	}
	tracker := report.NewProgressTracker(len(assets))
	tracker.SetTaskName(taskName)
	ng.SetTracker(tracker)

	// 设置当前 Tracker 引用
	s.setCurrentTracker(tracker)

	// 使用事件桥接器启动事件监听
	ctx, cancel := context.WithCancel(context.Background())

	// 保存 cancelFunc
	s.cancelMu.Lock()
	s.cancelFunc = cancel
	s.cancelMu.Unlock()

	// 启动快照定时推送
	s.startSnapshotTicker(ctx)

	// 使用简化的事件监听器
	go s.listenEvents(ng.FrontendBus)

	// 开始执行并发任务
	ng.Run(ctx)

	// 停止快照推送
	s.stopSnapshotTicker()

	// 发送执行完成事件
	s.emitFinishedEvent()

	// 清理 Tracker 引用
	s.clearCurrentTracker()

	// 清理 cancelFunc
	s.cancelMu.Lock()
	s.cancelFunc = nil
	s.cancelMu.Unlock()

	return nil
}

// GetEngineState 获取引擎当前状态（供前端调用）
func (s *EngineService) GetEngineState() map[string]interface{} {
	return engine.GetGlobalState().GetStatus()
}

// prepareAssetsAndCommands 准备设备和命令
func (s *EngineService) prepareAssetsAndCommands(deviceIPs []string, commandGroupID string) ([]config.DeviceAsset, []string, error) {
	// 获取所有设备
	allAssets, _, _, _, err := config.ParseOrGenerate(false)
	if err != nil {
		return nil, nil, err
	}

	var assets []config.DeviceAsset
	var commands []string

	// 筛选设备
	if deviceIPs != nil && len(deviceIPs) > 0 {
		ipSet := make(map[string]bool)
		for _, ip := range deviceIPs {
			ipSet[ip] = true
		}
		for _, asset := range allAssets {
			if ipSet[asset.IP] {
				assets = append(assets, asset)
			}
		}
	} else {
		assets = allAssets
	}

	// 获取命令
	if commandGroupID != "" {
		group, err := config.GetCommandGroup(commandGroupID)
		if err != nil {
			return nil, nil, fmt.Errorf("获取命令组失败: %v", err)
		}
		commands = group.Commands
	} else {
		_, cmds, _, _, err := config.ParseOrGenerate(false)
		if err != nil {
			return nil, nil, err
		}
		commands = cmds
	}

	return assets, commands, nil
}

// listenEvents 事件监听协程
func (s *EngineService) listenEvents(frontendBus chan report.ExecutorEvent) {
	for ev := range frontendBus {
		if s.wailsApp != nil {
			s.wailsApp.Event.Emit("device:event", ev)
		}
	}
}

// StartBackup 启动核心备份动作
func (s *EngineService) StartBackup() error {
	settings, _, err := config.LoadSettings()
	if err != nil {
		return err
	}

	assets, _, _, _, err := config.ParseOrGenerate(true) // isBackup = true
	if err != nil {
		return err
	}

	if len(assets) == 0 {
		return fmt.Errorf("资产池为空。请检查 csv 文件！")
	}

	// 初始化 Engine
	ng := engine.NewEngine(assets, nil, settings, false)
	ng.CustomSuspendHandler = GetSuspendManager().CreateHandler()

	// 【修改】设置到全局状态
	if err := engine.GetGlobalState().SetActiveEngine(ng, "backup_service", ""); err != nil {
		return err
	}
	defer engine.GetGlobalState().ClearActiveEngine()

	// 创建并设置 Tracker
	tracker := report.NewProgressTracker(len(assets))
	tracker.SetTaskName("配置备份")
	ng.SetTracker(tracker)

	// 设置当前 Tracker 引用
	s.setCurrentTracker(tracker)

	// 使用事件桥接器启动事件监听
	ctx, cancel := context.WithCancel(context.Background())

	// 保存 cancelFunc
	s.cancelMu.Lock()
	s.cancelFunc = cancel
	s.cancelMu.Unlock()

	// 启动快照定时推送
	s.startSnapshotTicker(ctx)

	// 使用简化的事件监听器
	go s.listenEvents(ng.FrontendBus)

	// 开始执行备份任务
	ng.RunBackup(ctx)

	// 停止快照推送
	s.stopSnapshotTicker()

	// 发送执行完成事件
	s.emitFinishedEvent()

	// 清理 Tracker 引用
	s.clearCurrentTracker()

	// 清理 cancelFunc
	s.cancelMu.Lock()
	s.cancelFunc = nil
	s.cancelMu.Unlock()

	return nil
}

// GetExecutionSnapshot 获取当前执行的快照
// 前端调用此方法获取完整的执行状态，无需前端计算
func (s *EngineService) GetExecutionSnapshot() *report.ExecutionSnapshot {
	s.trackerMu.RLock()
	defer s.trackerMu.RUnlock()

	if s.currentTracker == nil {
		return nil
	}

	return s.currentTracker.GetSnapshot()
}

// setCurrentTracker 设置当前的 Tracker 引用
func (s *EngineService) setCurrentTracker(tracker *report.ProgressTracker) {
	s.trackerMu.Lock()
	defer s.trackerMu.Unlock()
	s.currentTracker = tracker
}

// clearCurrentTracker 清除当前的 Tracker 引用
func (s *EngineService) clearCurrentTracker() {
	s.trackerMu.Lock()
	defer s.trackerMu.Unlock()
	s.currentTracker = nil
}

// startSnapshotTicker 启动快照定时推送
func (s *EngineService) startSnapshotTicker(ctx context.Context) {
	s.snapshotTicker = time.NewTicker(SnapshotInterval)
	s.snapshotStop = make(chan struct{})

	go func() {
		for {
			select {
			case <-s.snapshotTicker.C:
				snapshot := s.GetExecutionSnapshot()
				if snapshot != nil && s.wailsApp != nil {
					s.wailsApp.Event.Emit(SnapshotEventName, snapshot)
				}
			case <-ctx.Done():
				s.stopSnapshotTicker()
				return
			case <-s.snapshotStop:
				return
			}
		}
	}()
}

// stopSnapshotTicker 停止快照定时推送
func (s *EngineService) stopSnapshotTicker() {
	if s.snapshotTicker != nil {
		s.snapshotTicker.Stop()
		s.snapshotTicker = nil
	}
	if s.snapshotStop != nil {
		close(s.snapshotStop)
		s.snapshotStop = nil
	}
}

// emitFinishedEvent 发送执行完成事件
func (s *EngineService) emitFinishedEvent() {
	// 发送最终快照（100% 进度）
	snapshot := s.GetExecutionSnapshot()
	if snapshot != nil {
		snapshot.Progress = 100
		snapshot.IsRunning = false
		if s.wailsApp != nil {
			s.wailsApp.Event.Emit(SnapshotEventName, snapshot)
		}
	}

	// 发送完成事件
	if s.wailsApp != nil {
		s.wailsApp.Event.Emit(ExecutionFinishedEvent)
	}
}
