package ui

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/engine"
	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// EngineService 引擎控制服务 - 负责任务执行和状态管理
type EngineService struct {
	wailsApp *application.App

	// Context 取消函数，用于停止正在执行的任务
	cancelFunc context.CancelFunc
	cancelMu   sync.Mutex
}

// NewEngineService 创建引擎服务实例
func NewEngineService() *EngineService {
	return &EngineService{}
}

// ServiceStartup Wails 服务启动生命周期钩子
func (s *EngineService) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	s.wailsApp = application.Get()
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
	// 使用全局引擎状态管理器尝试获取锁
	if err := engine.TryAcquireEngine("engine_service"); err != nil {
		return err
	}

	// 确保在函数退出时释放锁
	defer engine.ReleaseEngine()

	settings, _, err := config.LoadSettings()
	if err != nil {
		return err
	}

	assets, commands, _, _, err := config.ParseOrGenerate(false)
	if err != nil {
		return err
	}

	if len(assets) == 0 || len(commands) == 0 {
		return fmt.Errorf("资产池或命令集为空。请检查 csv 和 txt 文件！")
	}

	// 初始化 Engine，开启了非交互模式的参数设定为 false（因为前端接管了交互）
	ng := engine.NewEngine(assets, commands, settings, false)
	ng.CustomSuspendHandler = GetSuspendManager().CreateHandler()

	// 使用双重同步机制确保事件监听器完全就绪
	type eventListenerState struct {
		ready     chan struct{} // Wails App 就绪
		active    chan struct{} // 事件循环确认启动
		listening chan struct{} // 确保进入读取循环
	}

	listenerState := &eventListenerState{
		ready:     make(chan struct{}),
		active:    make(chan struct{}),
		listening: make(chan struct{}),
	}

	// 桥接事件：监听 FrontendBus 转发给前端 Vue
	go func() {
		// 等待 Wails 应用实例就绪
		for i := 0; i < 100; i++ { // 最多等待 1 秒
			if s.wailsApp != nil {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		// 通知 App 已就绪
		close(listenerState.ready)

		// 进入事件循环前，等待启动信号
		<-listenerState.active
		
		close(listenerState.listening)

		// 开始消费 FrontendBus
		for ev := range ng.FrontendBus {
			if s.wailsApp != nil {
				s.wailsApp.Event.Emit("device:event", ev)
			}
		}
	}()

	// 等待 Wails App 就绪
	<-listenerState.ready

	// 发送启动信号，让事件循环开始
	close(listenerState.active)

	// 精确阻塞等待确切就绪完毕
	<-listenerState.listening

	ctx, cancel := context.WithCancel(context.Background())

	// 保存 cancelFunc 以便 StopEngine 可以调用
	s.cancelMu.Lock()
	s.cancelFunc = cancel
	s.cancelMu.Unlock()

	// 开始执行并发任务
	ng.Run(ctx)

	// 清理 cancelFunc
	s.cancelMu.Lock()
	s.cancelFunc = nil
	s.cancelMu.Unlock()

	// 确保所有事件都被处理完毕
	time.Sleep(100 * time.Millisecond)

	s.wailsApp.Event.Emit("engine:finished")

	return nil
}

// StartEngineWithSelection 使用选定的设备和命令组启动引擎
func (s *EngineService) StartEngineWithSelection(deviceIPs []string, commandGroupID string) error {
	// 使用全局引擎状态管理器尝试获取锁
	if err := engine.TryAcquireEngine("task_group_service"); err != nil {
		return err
	}

	// 确保在函数退出时释放锁
	defer engine.ReleaseEngine()

	settings, _, err := config.LoadSettings()
	if err != nil {
		return err
	}

	// 获取所有设备
	allAssets, _, _, _, err := config.ParseOrGenerate(false)
	if err != nil {
		return err
	}

	// 根据 IP 筛选设备
	var selectedAssets []config.DeviceAsset
	ipSet := make(map[string]bool)
	for _, ip := range deviceIPs {
		ipSet[ip] = true
	}
	for _, asset := range allAssets {
		if ipSet[asset.IP] {
			selectedAssets = append(selectedAssets, asset)
		}
	}

	if len(selectedAssets) == 0 {
		return fmt.Errorf("未选择任何有效设备")
	}

	// 获取命令组
	group, err := config.GetCommandGroup(commandGroupID)
	if err != nil {
		return fmt.Errorf("获取命令组失败: %v", err)
	}

	if len(group.Commands) == 0 {
		return fmt.Errorf("命令组为空")
	}

	// 初始化 Engine
	ng := engine.NewEngine(selectedAssets, group.Commands, settings, false)
	ng.CustomSuspendHandler = GetSuspendManager().CreateHandler()

	// 使用双重同步机制确保事件监听器完全就绪
	type eventListenerState struct {
		ready     chan struct{} // Wails App 就绪
		active    chan struct{} // 事件循环确认启动
		listening chan struct{} // 确保进入读取循环
	}

	listenerState := &eventListenerState{
		ready:     make(chan struct{}),
		active:    make(chan struct{}),
		listening: make(chan struct{}),
	}

	// 桥接事件：监听 FrontendBus 转发给前端 Vue
	go func() {
		// 等待 Wails 应用实例就绪
		for i := 0; i < 100; i++ { // 最多等待 1 秒
			if s.wailsApp != nil {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		// 通知 App 已就绪
		close(listenerState.ready)

		// 进入事件循环前，等待启动信号
		<-listenerState.active
		close(listenerState.listening)

		// 开始消费 FrontendBus
		for ev := range ng.FrontendBus {
			if s.wailsApp != nil {
				s.wailsApp.Event.Emit("device:event", ev)
			}
		}
	}()

	// 等待 Wails App 就绪
	<-listenerState.ready

	// 发送启动信号，让事件循环开始
	close(listenerState.active)

	// 精确阻塞等待确切就绪完毕
	<-listenerState.listening

	ctx, cancel := context.WithCancel(context.Background())

	// 保存 cancelFunc 以便 StopEngine 可以调用
	s.cancelMu.Lock()
	s.cancelFunc = cancel
	s.cancelMu.Unlock()

	// 开始执行并发任务
	ng.Run(ctx)

	// 清理 cancelFunc
	s.cancelMu.Lock()
	s.cancelFunc = nil
	s.cancelMu.Unlock()

	// 确保所有事件都被处理完毕
	time.Sleep(100 * time.Millisecond)

	s.wailsApp.Event.Emit("engine:finished")

	return nil
}

// StartBackup 启动核心备份动作
func (s *EngineService) StartBackup() error {
	// 使用全局引擎状态管理器尝试获取锁
	if err := engine.TryAcquireEngine("backup_service"); err != nil {
		return err
	}

	// 确保在函数退出时释放锁
	defer engine.ReleaseEngine()

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

	// 使用双重同步机制确保事件监听器完全就绪
	type eventListenerState struct {
		ready     chan struct{} // Wails App 就绪
		active    chan struct{} // 事件循环确认启动
		listening chan struct{} // 确保进入读取循环
	}

	listenerState := &eventListenerState{
		ready:     make(chan struct{}),
		active:    make(chan struct{}),
		listening: make(chan struct{}),
	}

	// 桥接事件：监听 FrontendBus 转发给前端 Vue
	go func() {
		// 等待 Wails 应用实例就绪
		for i := 0; i < 100; i++ { // 最多等待 1 秒
			if s.wailsApp != nil {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		// 通知 App 已就绪
		close(listenerState.ready)

		// 进入事件循环前，等待启动信号
		<-listenerState.active
		close(listenerState.listening)

		// 开始消费 FrontendBus
		for ev := range ng.FrontendBus {
			if s.wailsApp != nil {
				s.wailsApp.Event.Emit("device:event", ev)
			}
		}
	}()

	// 等待 Wails App 就绪
	<-listenerState.ready

	// 发送启动信号，让事件循环开始
	close(listenerState.active)

	// 精确阻塞等待确切就绪完毕
	<-listenerState.listening

	ctx, cancel := context.WithCancel(context.Background())

	// 保存 cancelFunc 以便 StopEngine 可以调用
	s.cancelMu.Lock()
	s.cancelFunc = cancel
	s.cancelMu.Unlock()

	// 开始执行备份任务
	ng.RunBackup(ctx)

	// 清理 cancelFunc
	s.cancelMu.Lock()
	s.cancelFunc = nil
	s.cancelMu.Unlock()

	// 确保所有事件都被处理完毕
	time.Sleep(100 * time.Millisecond)

	s.wailsApp.Event.Emit("engine:finished")

	return nil
}
