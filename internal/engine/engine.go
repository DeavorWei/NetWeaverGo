package engine

import (
	"bufio"
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/executor"
	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/report"
	"github.com/NetWeaverGo/core/internal/sftputil"
	"github.com/NetWeaverGo/core/internal/sshutil"
)

// 敏感信息脱敏正则表达式
var sensitivePatterns = []struct {
	pattern *regexp.Regexp
	replace string
}{
	// password xxx cipher/plain xxx -> password xxx cipher ****
	{regexp.MustCompile(`(?i)(password\s+\S+\s+cipher\s+)(\S+)`), "${1}****"},
	{regexp.MustCompile(`(?i)(password\s+\S+\s+plain\s+)(\S+)`), "${1}****"},
	// password xxx xxx (简写形式) -> password xxx ****
	{regexp.MustCompile(`(?i)(password\s+\S+\s+)(\S+)`), "${1}****"},
	// cipher xxx -> cipher ****
	{regexp.MustCompile(`(?i)(cipher\s+)(\S+)`), "${1}****"},
	// key xxx xxx -> key xxx **** (密钥配置)
	{regexp.MustCompile(`(?i)(key\s+\S+\s+)(\S+)`), "${1}****"},
	// secret xxx -> secret ****
	{regexp.MustCompile(`(?i)(secret\s+)(\S+)`), "${1}****"},
	// credential xxx -> credential ****
	{regexp.MustCompile(`(?i)(credential\s+)(\S+)`), "${1}****"},
}

// sanitizeMessage 脱敏消息中的敏感信息
func sanitizeMessage(msg string) string {
	sanitized := msg
	for _, p := range sensitivePatterns {
		sanitized = p.pattern.ReplaceAllString(sanitized, p.replace)
	}
	return sanitized
}

// Engine 全局中央调度器，控制所有物理设备的并发执行流
// 重构后：精简状态管理，使用 EngineStateManager + RingBuffer + Context
type Engine struct {
	Devices        []config.DeviceAsset
	Commands       []string
	MaxWorkers     int // 最大并发协程数量
	Settings       *config.GlobalSettings
	NonInteractive bool

	// 用于在控制台同一时刻串行化"挂起询问"操作，避免终端文字输出错位混淆
	promptMu sync.Mutex

	// EventBus 事件采集挂载（内部使用，由 tracker 消费）
	EventBus chan report.ExecutorEvent
	// FrontendBus 前端事件通道（外部使用，用于转发到 Wails 前端）
	FrontendBus chan report.ExecutorEvent
	tracker     *report.ProgressTracker

	failedBackups sync.Map // 记录备份失败的设备和原因

	// CustomSuspendHandler 允许外部（如 Wails UI）注入自定义的异常挂起处理逻辑
	CustomSuspendHandler executor.SuspendHandler

	// ====== 重构后的生命周期管理 ======
	// 1. 状态管理 - 仅使用状态机
	stateManager *EngineStateManager

	// 2. Context 控制 - 单一生命周期控制
	ctx    context.Context
	cancel context.CancelFunc

	// 3. 事件发送控制 - WaitGroup 保护
	emitWg sync.WaitGroup

	// 4. 后备存储 - 环形缓冲区 O(1)
	fallback *RingBuffer

	// 5. 关闭保护 - 只使用 sync.Once
	closeOnce sync.Once
}

// SetTracker 设置外部 Tracker 并连接事件总线
// 当 EngineService 需要获取快照时，调用此方法设置 Tracker
func (e *Engine) SetTracker(tracker *report.ProgressTracker) {
	if tracker == nil {
		return
	}
	e.tracker = tracker
	e.tracker.EventBus = e.EventBus
}

// TransitionTo 暴露状态转移能力，供编排层协调组合执行生命周期
func (e *Engine) TransitionTo(newState EngineState) error {
	return e.stateManager.TransitionTo(newState)
}

func (e *Engine) ensureTracker(taskName string) *report.ProgressTracker {
	if e.tracker == nil {
		e.tracker = report.NewProgressTracker(len(e.Devices))
	}
	e.tracker.EventBus = e.EventBus
	if taskName != "" {
		e.tracker.SetTaskName(taskName)
	}
	return e.tracker
}

// NewEngine 初始化并行执行引擎（重构后）
func NewEngine(assets []config.DeviceAsset, commands []string, settings *config.GlobalSettings, nonInteractive bool) *Engine {
	workers := settings.MaxWorkers
	if workers <= 0 {
		workers = 32
	}

	return &Engine{
		Devices:        assets,
		Commands:       commands,
		MaxWorkers:     workers, // 使用配置的并发限制
		Settings:       settings,
		NonInteractive: nonInteractive,
		EventBus:       make(chan report.ExecutorEvent, 1000), // 队列化 EventBus（内部 tracker 使用）
		FrontendBus:    make(chan report.ExecutorEvent, 1000), // 前端事件通道（外部转发使用）
		stateManager:   NewEngineStateManager(),
		fallback:       NewRingBuffer(500), // 环形缓冲区
	}
}

// State 获取引擎当前状态（委托给状态管理器）
func (e *Engine) State() EngineState {
	return e.stateManager.State()
}

// IsRunning 检查引擎是否正在运行（委托给状态管理器）
func (e *Engine) IsRunning() bool {
	return e.stateManager.IsRunning()
}

// IsClosing 检查引擎是否正在关闭（委托给状态管理器）
func (e *Engine) IsClosing() bool {
	return e.stateManager.IsClosing()
}

// GetFallbackEvents 获取后备事件列表（使用环形缓冲区）
func (e *Engine) GetFallbackEvents() []report.ExecutorEvent {
	return e.fallback.GetAll()
}

// ClearFallbackEvents 清空后备事件
func (e *Engine) ClearFallbackEvents() {
	e.fallback.Clear()
}

// emitEvent 重构后 - 消除 TOCTOU 竞态
// 同时向 EventBus 和 FrontendBus 发送事件（广播）
func (e *Engine) emitEvent(ev report.ExecutorEvent) {
	// 脱敏处理
	ev.Message = sanitizeMessage(ev.Message)

	// 【核心修复】：先占位，再检查。消除 TOCTOU 竞态
	e.emitWg.Add(1)
	defer e.emitWg.Done()

	// 检查 Context 是否已初始化
	if e.ctx == nil {
		// 引擎未启动，直接写入后备
		e.fallback.Push(ev)
		return
	}

	// 统一检查 Context 是否已取消
	select {
	case <-e.ctx.Done():
		// 引擎正在关闭，直接写入后备
		e.fallback.Push(ev)
		return
	default:
	}

	// 尝试发送到 FrontendBus（非阻塞）
	select {
	case e.FrontendBus <- ev:
		// 成功发送
	case <-e.ctx.Done():
		// 发送期间被取消
		e.fallback.Push(ev)
	default:
		// 通道已满，写入后备存储
		logger.Warn("Engine", ev.IP, "FrontendBus 已满，事件转入后备存储")
		e.fallback.Push(ev)
	}

	// 发送到 EventBus（tracker 使用，阻塞式）
	select {
	case e.EventBus <- ev:
	case <-e.ctx.Done():
	}
}

// emitEventDirect 直接发送高优先级事件
func (e *Engine) emitEventDirect(ev report.ExecutorEvent) {
	ev.Message = sanitizeMessage(ev.Message)

	// 检查 Context 是否已初始化
	if e.ctx == nil {
		e.fallback.Push(ev)
		return
	}

	// 统一检查 Context
	select {
	case <-e.ctx.Done():
		e.fallback.Push(ev)
		return
	default:
	}

	// 强制发送到 EventBus
	select {
	case e.EventBus <- ev:
	case <-e.ctx.Done():
		return
	}

	// 强制发送到 FrontendBus
	select {
	case e.FrontendBus <- ev:
	case <-e.ctx.Done():
		return
	}
}

// Run 启动 WorkerPool，正式分发任务
func (e *Engine) Run(ctx context.Context) error {
	logger.Debug("Engine", "-", "Run() 开始，将向 %d 台设备分发任务 (MaxWorkers=%d)", len(e.Devices), e.MaxWorkers)
	logger.Info("Engine", "-", "控制台引擎启动，共准备向 %d 台设备下发 %d 条命令...", len(e.Devices), len(e.Commands))
	logger.Info("Engine", "-", "当前已配置全局并发安全限制 (MaxWorkers=%d)。\n设备回显位于 output/ 目录，系统日志位于 logs/app.log，正在分批并发下发中...", e.MaxWorkers)

	logger.ConsoleMuted = true
	defer func() { logger.ConsoleMuted = false }()

	if err := e.stateManager.TransitionTo(StateStarting); err != nil {
		e.gracefulCloseWithoutCancel()
		return err
	}

	// 初始化 context 和状态
	e.ctx, e.cancel = context.WithCancel(ctx)
	e.ensureTracker("")
	if err := e.stateManager.TransitionTo(StateRunning); err != nil {
		if e.cancel != nil {
			e.cancel()
		}
		e.gracefulCloseWithoutCancel()
		return err
	}

	// 【修复】启动 Tracker 监听协程（与 RunBackup 保持一致）
	var trackerWg sync.WaitGroup
	if e.EventBus != nil && e.tracker != nil {
		trackerWg.Add(1)
		go func() {
			defer trackerWg.Done()
			e.tracker.Listen(e.ctx)
		}()
	}

	var wg sync.WaitGroup
	// 创建带缓冲的 channel 作为并发令牌桶
	sem := make(chan struct{}, e.MaxWorkers)

	for _, dev := range e.Devices {
		wg.Add(1)

		// 阻塞等待获取并发执行令牌，如果超过 MaxWorkers 则会在这里等待
		// 添加 Context 感知，避免在 Context 取消后仍然阻塞等待令牌
		select {
		case sem <- struct{}{}:
			logger.DebugAll("Engine", dev.IP, "获取到执行令牌，准备启动 worker")
		case <-e.ctx.Done():
			wg.Done()
			logger.Debug("Engine", dev.IP, "Context 已取消，跳过获取令牌")
			continue
		}

		// 将 dev 作为参数传递，避免在闭包内捕获循环变量
		go func(device config.DeviceAsset) {
			defer func() {
				// 执行完毕后，归还令牌
				<-sem
				logger.DebugAll("Engine", device.IP, "worker 执行完毕，已归还令牌")
			}()

			// 增加抖动，平滑 SSH 突发连接压力 (Jitter Delay, 0-500ms)
			time.Sleep(time.Duration(rand.Intn(500)) * time.Millisecond)

			e.worker(e.ctx, device, &wg)
		}(dev)
	}

	wg.Wait()

	// 【修复】先取消 Context，让 Tracker 退出，然后再等待它完成
	if e.cancel != nil {
		e.cancel()
	}

	// 等待 Tracker 监听完成
	trackerWg.Wait()

	// 使用统一的优雅关闭方法（不再调用 cancel，因为已在上面的步骤取消）
	e.gracefulCloseWithoutCancel()

	if e.tracker != nil {
		e.tracker.ExportCSV(e.Settings.OutputDir)
	}

	// 最终谢幕，保留一条普通的记录
	logger.Info("Engine", "-", "所有设备的通信投递线程均已结束。安全退出。")
	return nil
}

// gracefulClose 重构后 - 极简关闭流程
func (e *Engine) gracefulClose() {
	e.gracefulCloseWithCancel(true)
}

// gracefulCloseWithoutCancel 优雅关闭（不调用 cancel，因为已在外部取消）
func (e *Engine) gracefulCloseWithoutCancel() {
	e.gracefulCloseWithCancel(false)
}

// gracefulCloseWithCancel 统一的优雅关闭逻辑
func (e *Engine) gracefulCloseWithCancel(shouldCancel bool) {
	e.closeOnce.Do(func() {
		// 1. 转移到 Closing 状态（防止重复关闭）
		if err := e.stateManager.TransitionTo(StateClosing); err != nil {
			logger.Warn("Engine", "-", "引擎已在关闭中: %v", err)
			return
		}

		// 2. 取消 Context（这会立刻中断所有 select <-ctx.Done()）
		if shouldCancel && e.cancel != nil {
			e.cancel()
		}

		// 3. 等待所有进行中的发送完成
		// 因为我们修复了 emitEvent 的 TOCTOU，这里 Wait 返回时绝对安全
		e.emitWg.Wait()

		// 4. 安全关闭 Channel（此时 100% 没有 goroutine 在发送）
		// 先关闭内部 EventBus，再关闭外部 FrontendBus
		close(e.EventBus)
		close(e.FrontendBus)

		// 5. 标记为彻底关闭
		_ = e.stateManager.TransitionTo(StateClosed)

		logger.Info("Engine", "-", "引擎优雅关闭完成")
	})
}

// gracefulCloseForBackup 备份模式的优雅关闭方法
func (e *Engine) gracefulCloseForBackup(uiWg *sync.WaitGroup) {
	e.closeOnce.Do(func() {
		// 1. 转移到 Closing 状态
		if err := e.stateManager.TransitionTo(StateClosing); err != nil {
			logger.Warn("Engine", "-", "引擎已在关闭中: %v", err)
			return
		}

		// 2. 取消 Context
		if e.cancel != nil {
			e.cancel()
		}

		// 3. 等待所有进行中的发送完成
		e.emitWg.Wait()

		// 4. 关闭 channels 并等待 UI 监听器
		// 先关闭内部 EventBus，再关闭外部 FrontendBus
		close(e.EventBus)
		close(e.FrontendBus)
		if uiWg != nil {
			uiWg.Wait()
		}

		// 5. 标记为彻底关闭
		_ = e.stateManager.TransitionTo(StateClosed)

		logger.Info("Engine", "-", "引擎优雅关闭完成")
	})
}

// RunBackup 启动基于 `-b` 参数的交换机备份流程专线
func (e *Engine) RunBackup(ctx context.Context) error {
	logger.Debug("Engine", "-", "RunBackup() 启动备份模式")
	logger.Info("Engine", "-", "=======================================")
	logger.Info("Engine", "-", "备份模式启动")
	logger.Info("Engine", "-", "开始向 %d 台设备提取配置文件...", len(e.Devices))
	logger.Info("Engine", "-", "=======================================")

	if err := e.stateManager.TransitionTo(StateStarting); err != nil {
		e.gracefulCloseWithoutCancel()
		return err
	}

	// 初始化 context 和状态
	e.ctx, e.cancel = context.WithCancel(ctx)
	e.ensureTracker("配置备份")
	if err := e.stateManager.TransitionTo(StateRunning); err != nil {
		if e.cancel != nil {
			e.cancel()
		}
		e.gracefulCloseWithoutCancel()
		return err
	}

	// 启动后台事件收归与界面的渲染 (如果启用了 EventBus)
	var uiWg sync.WaitGroup
	if e.EventBus != nil {
		uiWg.Add(1)
		go func() {
			defer uiWg.Done()
			e.tracker.Listen(e.ctx)
		}()
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, e.MaxWorkers)

	for _, dev := range e.Devices {
		wg.Add(1)
		sem <- struct{}{}
		go func(device config.DeviceAsset) {
			defer func() { <-sem }()
			// 增加抖动，平滑 SSH 突发连接压力
			time.Sleep(time.Duration(rand.Intn(500)) * time.Millisecond)
			e.backupWorker(e.ctx, device, &wg)
		}(dev)
	}

	wg.Wait()

	// 使用统一的优雅关闭方法
	e.gracefulCloseForBackup(&uiWg)

	// 备份结束，汇总失败信息
	logger.Info("Engine", "-", "\n========== [备份任务结束] ==========")
	var hasFailures bool
	e.failedBackups.Range(func(key, value interface{}) bool {
		hasFailures = true
		return false
	})

	if hasFailures {
		logger.Warn("Engine", "-", "[警告] 以下设备备份失败:")
		e.failedBackups.Range(func(key, value interface{}) bool {
			logger.Warn("Engine", "-", "  - IP: %-15s | 原因: %s", key.(string), value.(string))
			return true
		})
	} else {
		logger.Info("Engine", "-", "[成功] 所有设备的配置均已成功备份。")
	}
	logger.Info("Engine", "-", "====================================\n")
	return nil
}

func (e *Engine) worker(ctx context.Context, dev config.DeviceAsset, wg *sync.WaitGroup) {
	defer wg.Done()
	logger.Debug("Worker", dev.IP, "worker 启动")

	manager := config.GetRuntimeManager()
	connectTimeout := manager.GetConnectionTimeout()
	commandTimeout := manager.GetCommandTimeout()

	suspendHandler := e.handleSuspend
	if e.CustomSuspendHandler != nil {
		suspendHandler = e.CustomSuspendHandler
	}

	// 创建一个包装的 EventBus，同时发送到 tracker 和 FrontendBus
	workerEventBus := make(chan report.ExecutorEvent, 100)

	// 启动事件转发器 - 带优先级处理和改进的排空逻辑
	go func() {
		defer func() {
			// 确保排空 channel 中剩余的所有事件
			for {
				select {
				case ev, ok := <-workerEventBus:
					if !ok {
						return
					}
					if ev.Type == report.EventDeviceError || ev.Type == report.EventDeviceAbort {
						e.emitEventDirect(ev)
					} else {
						e.emitEvent(ev)
					}
				default:
					return
				}
			}
		}()

		for {
			select {
			case ev, ok := <-workerEventBus:
				if !ok {
					// channel 已关闭，defer 会处理剩余事件
					return
				}
				// 高优先级事件（如 Error/Abort）直接发送
				if ev.Type == report.EventDeviceError || ev.Type == report.EventDeviceAbort {
					e.emitEventDirect(ev)
				} else {
					e.emitEvent(ev)
				}
			case <-ctx.Done():
				// Context 取消，defer 会处理剩余事件
				return
			}
		}
	}()

	exec := executor.NewDeviceExecutor(dev.IP, dev.Port, dev.Username, dev.Password, workerEventBus, suspendHandler)
	exec.SetAlgorithms(&e.Settings.SSHAlgorithms)
	defer exec.Close()

	// 发送开始事件
	workerEventBus <- report.ExecutorEvent{IP: dev.IP, Type: report.EventDeviceStart, TotalCmd: len(e.Commands), Message: "Connecting SSH..."}

	if err := exec.Connect(ctx, connectTimeout); err != nil {
		// 使用统一错误处理
		if execErr, ok := executor.IsExecutionError(err); ok {
			// 已经是 ExecutionError，记录并发送事件
			handler := executor.NewErrorHandler()
			handler.Handle(ctx, execErr)
			workerEventBus <- report.ExecutorEvent{IP: dev.IP, Type: report.EventDeviceError, Message: execErr.Message}
			workerEventBus <- report.ExecutorEvent{IP: dev.IP, Type: report.EventDeviceAbort, Message: execErr.Message, TotalCmd: len(e.Commands)}
		} else {
			// 包装为 ExecutionError
			execErr := executor.NewError(dev.IP).
				WithStage(executor.StageConnect).
				WithType(executor.ClassifyError(err)).
				WithError(err).
				Build()
			handler := executor.NewErrorHandler()
			handler.Handle(ctx, execErr)
			workerEventBus <- report.ExecutorEvent{IP: dev.IP, Type: report.EventDeviceError, Message: fmt.Sprintf("SSH 建连失败: %v", err)}
			workerEventBus <- report.ExecutorEvent{IP: dev.IP, Type: report.EventDeviceAbort, Message: fmt.Sprintf("连接失败: %v", err), TotalCmd: len(e.Commands)}
		}
		close(workerEventBus)
		return
	}

	if err := exec.ExecutePlaybook(ctx, e.Commands, commandTimeout); err != nil {
		logger.Debug("Engine", dev.IP, "worker 播放命令集结束，返回了 error: %v", err)
		// 使用统一错误处理
		if execErr, ok := executor.IsExecutionError(err); ok {
			handler := executor.NewErrorHandler()
			shouldContinue := handler.Handle(ctx, execErr)
			if !shouldContinue && !execErr.IsWarning() {
				logger.Error("Engine", dev.IP, "严重错误，终止设备执行: %v", execErr.Message)
			}
		} else {
			// 包装为 ExecutionError
			execErr := executor.NewError(dev.IP).
				WithStage(executor.StageExecute).
				WithType(executor.ClassifyError(err)).
				WithError(err).
				Build()
			handler := executor.NewErrorHandler()
			handler.Handle(ctx, execErr)
		}
	} else {
		logger.Debug("Engine", dev.IP, "worker 播放命令集成功完成")
		workerEventBus <- report.ExecutorEvent{IP: dev.IP, Type: report.EventDeviceSuccess, Message: "执行完毕", TotalCmd: len(e.Commands)}
	}
	close(workerEventBus)
}

// backupWorker 是基于单个设备进行交互的备份动作集散流
func (e *Engine) backupWorker(ctx context.Context, dev config.DeviceAsset, wg *sync.WaitGroup) {
	defer wg.Done()

	manager := config.GetRuntimeManager()

	// 备份模块不初始化 ProgressTracker，因此 EventBus 必须设为 nil 以避免死锁。
	// 修正：如果外部注入了 EventBus，则使用它。
	exec := executor.NewDeviceExecutor(dev.IP, dev.Port, dev.Username, dev.Password, e.EventBus, func(ctx context.Context, ip, log, cmd string) executor.ErrorAction {
		return executor.ActionContinue
	})
	defer exec.Close()

	if e.EventBus != nil {
		e.EventBus <- report.ExecutorEvent{IP: dev.IP, Type: report.EventDeviceStart, Message: "Connecting SSH for Backup..."}
	}

	connectTimeout := manager.GetConnectionTimeout()

	if err := exec.Connect(ctx, connectTimeout); err != nil {
		logger.Error("Worker", dev.IP, "SSH建连失败: %v", err)
		e.failedBackups.Store(dev.IP, fmt.Sprintf("SSH连通信失败: %v", err))
		if e.EventBus != nil {
			e.EventBus <- report.ExecutorEvent{IP: dev.IP, Type: report.EventDeviceError, Message: fmt.Sprintf("SSH 建连失败: %v", err)}
		}
		return
	}

	// 1. 挂载 SFTP 作为初步连接检测 (使用独立的新通道避免 PTY 被占用)
	sftpCfg := sshutil.Config{
		IP:         dev.IP,
		Port:       dev.Port,
		Username:   dev.Username,
		Password:   dev.Password,
		Timeout:    connectTimeout,
		Algorithms: &e.Settings.SSHAlgorithms,
	}
	sftpClient, err := sftputil.NewSFTPClient(ctx, sftpCfg)
	if err != nil {
		// 如果 SFTP 连接失败，则探测 sftp 是否开启
		logger.Warn("Worker", dev.IP, "SFTP 挂载异常(底层异常: %v)，开始提取服务状态原因...", err)
		if e.EventBus != nil {
			e.EventBus <- report.ExecutorEvent{IP: dev.IP, Message: fmt.Sprintf("SFTP 挂载异常: %v", err)}
		}
		out, _ := exec.ExecuteCommandSync(ctx, "disp cur | inc sftp", 10*time.Second)
		errMsg := ""
		if strings.Contains(strings.ToLower(out), "sftp server enable") {
			errMsg = fmt.Sprintf("SFTP建连失败（服务已配置，可能存在其他连通性或权限问题）。底层报错: %v", err)
		} else {
			errMsg = fmt.Sprintf("sftp服务未启动（配置文件无 sftp server enable）。底层报错: %v", err)
		}
		e.failedBackups.Store(dev.IP, errMsg)
		if e.EventBus != nil {
			e.EventBus <- report.ExecutorEvent{IP: dev.IP, Type: report.EventDeviceError, Message: errMsg}
		}
		return
	}
	defer sftpClient.Close()

	// 2. 正常读取配置文件名称
	logger.Info("Worker", dev.IP, "SFTP会话成功挂载，准备查询下次启动配置文件...")
	// 某些设备需要禁用分屏，或者我们依赖"Next startup saved-configuration file"出现在第一屏回显中
	exec.ExecuteCommandSync(ctx, "screen-length 0 temporary", manager.GetShortCommandTimeout()) // 尽可能的规避翻页问题
	output, err := exec.ExecuteCommandSync(ctx, "display startup", manager.GetLongCommandTimeout())
	if err != nil {
		errMsg := fmt.Sprintf("采集 startup 信息超时或失败: %v", err)
		e.failedBackups.Store(dev.IP, errMsg)
		if e.EventBus != nil {
			e.EventBus <- report.ExecutorEvent{IP: dev.IP, Type: report.EventDeviceError, Message: errMsg}
		}
		return
	}

	// 3. 解析 "Next startup saved-configuration file:"
	re := regexp.MustCompile(`(?i)Next\s+startup\s+saved-configuration\s+file:\s+([^\s]+)`)
	matches := re.FindStringSubmatch(output)
	if len(matches) < 2 {
		errMsg := "未能在 display startup 回显中寻寻找下次启动配置文件声明"
		e.failedBackups.Store(dev.IP, errMsg)
		if e.EventBus != nil {
			e.EventBus <- report.ExecutorEvent{IP: dev.IP, Type: report.EventDeviceError, Message: errMsg}
		}
		return
	}

	remotePath := matches[1]

	// 移除非标准的基础存储器前缀（兼容 flash:/, cfcard:/, sd1:/, cfa0:/ 等带有 :/ 的根路径）
	// 因为绝大多数交换机的 SFTP 子系统都是以这些存储器的根作为虚拟根目录的，因此我们只需要请求相对路径即可
	cleanRemotePath := remotePath
	if idx := strings.Index(remotePath, ":/"); idx > 0 {
		cleanRemotePath = remotePath[idx+2:] // 如从 flash:/backup/1.cfg 截取出 backup/1.cfg
	}

	if cleanRemotePath == "NULL" || strings.TrimSpace(cleanRemotePath) == "" {
		errMsg := "未配置下次启动配置文件(NULL)"
		e.failedBackups.Store(dev.IP, errMsg)
		if e.EventBus != nil {
			e.EventBus <- report.ExecutorEvent{IP: dev.IP, Type: report.EventDeviceError, Message: errMsg}
		}
		return
	}

	// 4. 开始构造下载本地路径与下载操作
	nowStr := time.Now().Format("20060102_150405")
	dateDir := time.Now().Format("20060102")
	baseName := filepath.Base(cleanRemotePath)
	ext := filepath.Ext(baseName)
	if ext == "" {
		ext = ".cfg"
	}

	fileName := fmt.Sprintf("%s_%s%s", dev.IP, nowStr, ext)
	localPath := filepath.Join("confBakup", dateDir, fileName)

	// 因为大多数华为/华三设备上的 SFTP 需要准确的 flash 位置：
	// 根据 SFTP 子系统不同，"flash:/1.cfg" 需要作为 "1.cfg" 或 "/1.cfg" 来获取。
	// pkg/sftp 通常基准当前目录解析 `1.cfg`，这对于 flash 根目录来说是没问题的。

	err = sftpClient.DownloadFile(cleanRemotePath, localPath)
	if err != nil {
		errMsg := fmt.Sprintf("SFTP 下载 %s 失败: %v", cleanRemotePath, err)
		e.failedBackups.Store(dev.IP, errMsg)
		if e.EventBus != nil {
			e.EventBus <- report.ExecutorEvent{IP: dev.IP, Type: report.EventDeviceError, Message: errMsg}
		}
		return
	}

	logger.Info("Worker", dev.IP, "配置备份完成 -> %s", localPath)
	if e.EventBus != nil {
		e.EventBus <- report.ExecutorEvent{IP: dev.IP, Type: report.EventDeviceSuccess, Message: fmt.Sprintf("备份成功: %s", fileName)}
	}
}

// handleSuspend 被传递到每一个 `executor`，一旦匹配到 error 正则则回调该函数，挂起当前设备的 Goroutine。
// 使用 `promptMu` 互斥锁包围控制台的 STDIN 输入，保证多个设备同时发生 error 时，命令行不会争抢标准输入光标。
func (e *Engine) handleSuspend(ctx context.Context, ip string, logLine string, cmd string) executor.ErrorAction {
	// 记录到应用日志中
	logger.Warn("Engine", ip, "==================== [异常设备挂起干预] ====================")
	logger.Warn("Engine", ip, "=> 触发指令: %s", cmd)
	logger.Warn("Engine", ip, "=> 回显日志: %s", strings.TrimSpace(logLine))

	cleanStr := strings.TrimSpace(strings.ToUpper(logLine))
	// 如果是无人值守模式，直接静默根据配置执行对应的动作
	if e.NonInteractive {
		switch e.Settings.ErrorMode {
		case "skip":
			logger.Info("Engine", ip, "-> [Non-Interactive] 触发全局 skip 策略: 已跳过当前报错动作。")
			return executor.ActionSkip
		case "abort":
			logger.Warn("Engine", ip, "-> [Non-Interactive] 触发全局 abort 策略: 正在终止异常设备的运行流。")
			return executor.ActionAbort
		case "pause":
			// Non-interactive 模式下 pause 应该导致整机中止，因为无人值守不会有人去按下继续
			logger.Warn("Engine", ip, "-> [Non-Interactive] 触发全局 pause 策略: 无人值守状态下无法挂起，降级为 abort，正在终止异常设备。")
			return executor.ActionAbort
		default:
			logger.Warn("Engine", "-", "-> [Non-Interactive] 未知全局策略 %s，回退采用 abort 流。", e.Settings.ErrorMode)
			return executor.ActionAbort
		}
	}

	// 此时需要阻塞前台，加锁保护以免乱打
	e.promptMu.Lock()
	if e.tracker != nil {
		e.tracker.Suspend()
	}
	// 交互时需要控制台输出，解开静音
	logger.ConsoleMuted = false
	defer func() {
		logger.ConsoleMuted = true
		if e.tracker != nil {
			e.tracker.Resume()
		}
		e.promptMu.Unlock()
	}()

	// 交互模式下的标准提示
	fmt.Printf("\n==================== [异常设备挂起干预] ====================\n")
	fmt.Printf("=> 目标设备: %s\n", ip)
	fmt.Printf("=> 触发指令: %s\n", cmd)
	fmt.Printf("=> 回显日志: %s\n", strings.TrimSpace(logLine))
	fmt.Print("\n==> (当前错误只挂起了这台设备，全局其他设备仍在继续运行中)\n")
	fmt.Print("请选择动作:\n  [C] 继续发送下一条命令 (Continue)\n  [S] 放弃此报错动作强行放行 (Skip)\n  [A] 退下并切断此设备连接 (Abort)\n>> 请输入并回车[C/S/A]: ")

	reader := bufio.NewReader(os.Stdin)
	for {
		input, err := reader.ReadString('\n')
		if err != nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		cleanStr = strings.TrimSpace(strings.ToUpper(input))
		switch cleanStr {
		case "C":
			return executor.ActionContinue
		case "S":
			return executor.ActionSkip
		case "A":
			return executor.ActionAbort
		}
		fmt.Print(">> 输入无效，仅支持 C、S 或 A: ")
	}
}
