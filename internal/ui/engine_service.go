package ui

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/engine"
	"github.com/NetWeaverGo/core/internal/executor"
	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// EngineService 引擎控制服务 - 负责任务执行和状态管理
type EngineService struct {
	wailsApp *application.App

	// 控制运行状态
	isRunning bool
	mu        sync.Mutex

	// 挂起交互的通信频道
	suspendSignals map[string]chan executor.ErrorAction
	suspendMu      sync.Mutex
}

// NewEngineService 创建引擎服务实例
func NewEngineService() *EngineService {
	return &EngineService{
		suspendSignals: make(map[string]chan executor.ErrorAction),
	}
}

// ServiceStartup Wails 服务启动生命周期钩子
func (s *EngineService) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	s.wailsApp = application.Get()
	logger.Info("Engine", "-", "引擎控制服务已就绪")
	return nil
}

// IsRunning 检查引擎是否正在运行
func (s *EngineService) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.isRunning
}

// ResolveSuspend 被前端调用（当用户在弹窗中选择动作后）
func (s *EngineService) ResolveSuspend(ip string, action string) {
	s.suspendMu.Lock()
	ch, exists := s.suspendSignals[ip]
	s.suspendMu.Unlock()

	if !exists {
		logger.Warn("Engine", ip, "找不到对应的挂机通信频道，可能任务已结束或超时")
		return
	}

	var errAction executor.ErrorAction
	switch action {
	case "C":
		errAction = executor.ActionContinue
	case "S":
		errAction = executor.ActionSkip
	case "A":
		errAction = executor.ActionAbort
	}

	select {
	case ch <- errAction:
	default:
	}
}

// wailsSuspendHandler 构建代理 Suspend 钩子替换原先的控制台询问方式
// 注意：此方法为私有方法（首字母小写），不会被 Wails 自动绑定到前端
func (s *EngineService) wailsSuspendHandler() executor.SuspendHandler {
	return func(ip string, logLine string, cmd string) executor.ErrorAction {
		// 阻断 Channel 预留
		actionCh := make(chan executor.ErrorAction, 1)

		s.suspendMu.Lock()
		s.suspendSignals[ip] = actionCh
		s.suspendMu.Unlock()

		defer func() {
			s.suspendMu.Lock()
			delete(s.suspendSignals, ip)
			s.suspendMu.Unlock()
		}()

		// 抛出悬停事件前台处理
		s.wailsApp.Event.Emit("engine:suspend_required", map[string]interface{}{
			"ip":      ip,
			"error":   logLine,
			"command": cmd,
		})

		// 无限阻塞等待前端回传决策，或者可设计一个 5分钟 的自动 abort 控制
		logger.Warn("Engine", ip, "已向界面发射阻断警告，等待用户操作...")

		// 带 5 分钟超时保护，避免用户关闭窗口或长时不操作导致 goroutine 永久挂起
		select {
		case action := <-actionCh:
			return action
		case <-time.After(5 * time.Minute):
			logger.Warn("Engine", ip, "挂起等待超时（5分钟），自动执行 Abort 策略")
			return executor.ActionAbort
		}
	}
}

// StartEngine 启动核心下发动作
func (s *EngineService) StartEngine() error {
	s.mu.Lock()
	if s.isRunning {
		s.mu.Unlock()
		return fmt.Errorf("引擎正在运行中，请勿重复启动")
	}
	s.isRunning = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.isRunning = false
		s.mu.Unlock()
	}()

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
	ng.CustomSuspendHandler = s.wailsSuspendHandler()

	// 使用 WaitGroup 确保事件监听器在 Run() 之前准备好
	var listenerReady sync.WaitGroup
	listenerReady.Add(1)

	// 桥接事件：监听 FrontendBus 转发给前端 Vue
	go func() {
		time.Sleep(50 * time.Millisecond) // 确保 Wails 事件系统准备好
		listenerReady.Done()              // 通知监听器已准备好
		for ev := range ng.FrontendBus {
			s.wailsApp.Event.Emit("device:event", ev)
		}
	}()

	// 等待监听器准备好再开始执行
	listenerReady.Wait()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 开始执行并发任务
	ng.Run(ctx)
	// 确保所有事件都被处理完毕
	time.Sleep(100 * time.Millisecond)

	s.wailsApp.Event.Emit("engine:finished")

	return nil
}

// StartEngineWithSelection 使用选定的设备和命令组启动引擎
func (s *EngineService) StartEngineWithSelection(deviceIPs []string, commandGroupID string) error {
	s.mu.Lock()
	if s.isRunning {
		s.mu.Unlock()
		return fmt.Errorf("引擎正在运行中，请勿重复启动")
	}
	s.isRunning = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.isRunning = false
		s.mu.Unlock()
	}()

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
	ng.CustomSuspendHandler = s.wailsSuspendHandler()

	// 使用 WaitGroup 确保事件监听器在 Run() 之前准备好
	var listenerReady sync.WaitGroup
	listenerReady.Add(1)

	// 桥接事件：监听 FrontendBus 转发给前端 Vue
	go func() {
		time.Sleep(50 * time.Millisecond) // 确保 Wails 事件系统准备好
		listenerReady.Done()              // 通知监听器已准备好
		for ev := range ng.FrontendBus {
			s.wailsApp.Event.Emit("device:event", ev)
		}
	}()

	// 等待监听器准备好再开始执行
	listenerReady.Wait()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 开始执行并发任务
	ng.Run(ctx)
	// 确保所有事件都被处理完毕
	time.Sleep(100 * time.Millisecond)

	s.wailsApp.Event.Emit("engine:finished")

	return nil
}

// StartBackup 启动核心备份动作
func (s *EngineService) StartBackup() error {
	s.mu.Lock()
	if s.isRunning {
		s.mu.Unlock()
		return fmt.Errorf("引擎正在运行中，请勿重复启动")
	}
	s.isRunning = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.isRunning = false
		s.mu.Unlock()
	}()

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
	ng.CustomSuspendHandler = s.wailsSuspendHandler()

	// 使用 WaitGroup 确保事件监听器在 Run() 之前准备好
	var listenerReady sync.WaitGroup
	listenerReady.Add(1)

	// 桥接事件：监听 FrontendBus 转发给前端 Vue
	go func() {
		time.Sleep(50 * time.Millisecond) // 确保 Wails 事件系统准备好
		listenerReady.Done()              // 通知监听器已准备好
		for ev := range ng.FrontendBus {
			s.wailsApp.Event.Emit("device:event", ev)
		}
	}()

	// 等待监听器准备好再开始执行
	listenerReady.Wait()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 开始执行备份任务
	ng.RunBackup(ctx)
	// 确保所有事件都被处理完毕
	time.Sleep(100 * time.Millisecond)

	s.wailsApp.Event.Emit("engine:finished")

	return nil
}
