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

// AppService 包装所有要暴露给 Wails Frontend 的绑定方法
type AppService struct {
	wailsApp *application.App
	ctx      context.Context

	// 控制运行状态
	isRunning bool
	mu        sync.Mutex

	// 挂起交互的通信频道
	suspendSignals map[string]chan executor.ErrorAction
	suspendMu      sync.Mutex
}

func NewAppService() *AppService {
	return &AppService{
		suspendSignals: make(map[string]chan executor.ErrorAction),
	}
}

// ServiceStartup Wails 应用启动时的生命周期钩子
func (a *AppService) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	a.ctx = ctx
	a.wailsApp = application.Get()
	logger.Info("UI", "-", "Wails 图形界面服务已就绪...")
	return nil
}

// LoadSettings 获取合并后的主配置
func (a *AppService) LoadSettings() (*config.GlobalSettings, error) {
	settings, _, err := config.LoadSettings()
	return settings, err
}

// EnsureConfig 检查必需配置文件并返回是否有文件遗漏，以便前端提示
func (a *AppService) EnsureConfig() ([]config.DeviceAsset, []string, []string, error) {
	assets, commands, _, missingFiles, err := config.ParseOrGenerate(false)
	return assets, commands, missingFiles, err
}

// ResolveSuspend 被前端调用（当用户在弹窗中选择动作后）
func (a *AppService) ResolveSuspend(ip string, action string) {
	a.suspendMu.Lock()
	ch, exists := a.suspendSignals[ip]
	a.suspendMu.Unlock()

	if !exists {
		logger.Warn("UI", ip, "找不到对应的挂机通信频道，可能任务已结束或超时")
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

// WailsSuspendHandler 构建代理 Suspend 钩子替换原先的控制台询问方式
func (a *AppService) WailsSuspendHandler() executor.SuspendHandler {
	return func(ip string, logLine string, cmd string) executor.ErrorAction {
		// 阻断 Channel 预留
		actionCh := make(chan executor.ErrorAction, 1)

		a.suspendMu.Lock()
		a.suspendSignals[ip] = actionCh
		a.suspendMu.Unlock()

		defer func() {
			a.suspendMu.Lock()
			delete(a.suspendSignals, ip)
			a.suspendMu.Unlock()
		}()

		// 抛出悬停事件前台处理
		a.wailsApp.Event.Emit("engine:suspend_required", map[string]interface{}{
			"ip":      ip,
			"error":   logLine,
			"command": cmd,
		})

		// 无限阻塞等待前端回传决策，或者可设计一个 5分钟 的自动 abort 控制
		// 这里暂以常阻塞处理
		logger.Warn("UI", ip, "已向界面发射阻断警告，等待用户操作...")

		// 带 5 分钟超时保护，避免用户关闭窗口或长时不操作导致 goroutine 永久挂起
		select {
		case action := <-actionCh:
			return action
		case <-time.After(5 * time.Minute):
			logger.Warn("UI", ip, "挂起等待超时（5分钟），自动执行 Abort 策略")
			return executor.ActionAbort
		}
	}
}

// StartEngineWails 启动核心下发动作（UI包裹层）
func (a *AppService) StartEngineWails() error {
	a.mu.Lock()
	if a.isRunning {
		a.mu.Unlock()
		return fmt.Errorf("引擎正在运行中，请勿重复启动")
	}
	a.isRunning = true
	a.mu.Unlock()

	defer func() {
		a.mu.Lock()
		a.isRunning = false
		a.mu.Unlock()
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
	ng.CustomSuspendHandler = a.WailsSuspendHandler()

	// 桥接事件：监听底层的 EventBus 转发给前端 Vue
	go func() {
		for ev := range ng.EventBus {
			a.wailsApp.Event.Emit("device:event", ev)
		}
	}()

	// 强制注入我们的 Wails UI 层 Suspend 引擎覆盖掉 Executor 的原本回调逻辑
	// 方案: 注意：原来的 Engine 其实是在 run worker 的时候构建 executor 提供 SuspendHandler，我们需要进行一定的侵入覆盖。
	// 但鉴于我们这里不需要修改 engine.go 源码太多，暂时只能让 engine 的 newExecutor 方法接受全局替代。

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 开始执行并发任务
	ng.Run(ctx)

	a.wailsApp.Event.Emit("engine:finished")

	return nil
}
