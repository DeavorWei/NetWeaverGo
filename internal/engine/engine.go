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

// Engine 全局中央调度器，控制所有物理设备的并发执行流
type Engine struct {
	Devices        []config.DeviceAsset
	Commands       []string
	MaxWorkers     int // 最大并发协程数量
	Settings       *config.GlobalSettings
	NonInteractive bool

	// 用于在控制台同一时刻串行化“挂起询问”操作，避免终端文字输出错位混淆
	promptMu sync.Mutex

	// EventBus 事件采集挂载
	EventBus chan report.ExecutorEvent
	tracker  *report.ProgressTracker

	failedBackups sync.Map // 记录备份失败的设备和原因

	// CustomSuspendHandler 允许外部（如 Wails UI）注入自定义的异常挂起处理逻辑
	CustomSuspendHandler executor.SuspendHandler
}

// NewEngine 初始化并行执行引擎
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
		EventBus:       make(chan report.ExecutorEvent, 1000), // 队列化 EventBus
	}
}

// Run 启动 WorkerPool，正式分发任务
func (e *Engine) Run(ctx context.Context) {
	logger.Debug("Engine", "-", "Run() 开始，将向 %d 台设备分发任务 (MaxWorkers=%d)", len(e.Devices), e.MaxWorkers)
	logger.Info("Engine", "-", "控制台引擎启动，共准备向 %d 台设备下发 %d 条命令...", len(e.Devices), len(e.Commands))
	logger.Info("Engine", "-", "当前已配置全局并发安全限制 (MaxWorkers=%d)。\n设备回显位于 output/ 目录，系统日志位于 logs/app.log，正在分批并发下发中...", e.MaxWorkers)

	logger.ConsoleMuted = true
	defer func() { logger.ConsoleMuted = false }()

	e.tracker = report.NewProgressTracker(len(e.Devices))
	e.tracker.EventBus = e.EventBus

	// 启动后台事件收归与界面的渲染
	var uiWg sync.WaitGroup
	uiWg.Add(1)
	go func() {
		defer uiWg.Done()
		e.tracker.Listen(ctx)
	}()

	var wg sync.WaitGroup
	// 创建带缓冲的 channel 作为并发令牌桶
	sem := make(chan struct{}, e.MaxWorkers)

	for _, dev := range e.Devices {
		wg.Add(1)

		// 阻塞等待获取并发执行令牌，如果超过 MaxWorkers 则会在这里等待
		sem <- struct{}{}
		logger.DebugAll("Engine", dev.IP, "获取到执行令牌，准备启动 worker")

		// 将 dev 作为参数传递，避免在闭包内捕获循环变量
		go func(device config.DeviceAsset) {
			defer func() {
				// 执行完毕后，归还令牌
				<-sem
				logger.DebugAll("Engine", device.IP, "worker 执行完毕，已归还令牌")
			}()

			// 增加抖动，平滑 SSH 突发连接压力 (Jitter Delay, 0-500ms)
			time.Sleep(time.Duration(rand.Intn(500)) * time.Millisecond)

			e.worker(ctx, device, &wg)
		}(dev)
	}

	wg.Wait()
	close(e.EventBus) // 安全关闭通道，引发界面结算
	uiWg.Wait()

	e.tracker.ExportCSV(e.Settings.OutputDir)

	// 最终谢幕，保留一条普通的记录
	logger.Info("Engine", "-", "所有设备的通信投递线程均已结束。安全退出。")
}

// RunBackup 启动基于 `-b` 参数的交换机备份流程专线
func (e *Engine) RunBackup(ctx context.Context) {
	logger.Debug("Engine", "-", "RunBackup() 启动备份模式")
	logger.Info("Engine", "-", "=======================================")
	logger.Info("Engine", "-", "备份模式启动")
	logger.Info("Engine", "-", "开始向 %d 台设备提取配置文件...", len(e.Devices))
	logger.Info("Engine", "-", "=======================================")

	var wg sync.WaitGroup
	sem := make(chan struct{}, e.MaxWorkers)

	for _, dev := range e.Devices {
		wg.Add(1)
		sem <- struct{}{}
		go func(device config.DeviceAsset) {
			defer func() { <-sem }()
			e.backupWorker(ctx, device, &wg)
		}(dev)
	}

	wg.Wait()

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
}

func (e *Engine) worker(ctx context.Context, dev config.DeviceAsset, wg *sync.WaitGroup) {
	defer wg.Done()
	logger.Debug("Worker", dev.IP, "worker 启动")

	connectTimeout, err := time.ParseDuration(e.Settings.ConnectTimeout)
	if err != nil {
		connectTimeout = 10 * time.Second
	}
	commandTimeout, err := time.ParseDuration(e.Settings.CommandTimeout)
	if err != nil {
		commandTimeout = 30 * time.Second
	}

	suspendHandler := e.handleSuspend
	if e.CustomSuspendHandler != nil {
		suspendHandler = e.CustomSuspendHandler
	}

	exec := executor.NewDeviceExecutor(dev.IP, dev.Port, dev.Username, dev.Password, e.EventBus, suspendHandler)
	defer exec.Close()

	e.EventBus <- report.ExecutorEvent{IP: dev.IP, Type: report.EventDeviceStart, TotalCmd: len(e.Commands), Message: "Connecting SSH..."}

	if err := exec.Connect(ctx, connectTimeout); err != nil {
		e.EventBus <- report.ExecutorEvent{IP: dev.IP, Type: report.EventDeviceError, Message: fmt.Sprintf("SSH 建连失败: %v", err)}
		return
	}

	if err := exec.ExecutePlaybook(ctx, e.Commands, commandTimeout); err != nil {
		logger.Debug("Engine", dev.IP, "worker 播放命令集结束，返回了 error: %v", err)
		// Event 已经被底层的 action (Abort|Error)抛出过，此处无需重复抛出全量错误
	} else {
		logger.Debug("Engine", dev.IP, "worker 播放命令集成功完成")
		e.EventBus <- report.ExecutorEvent{IP: dev.IP, Type: report.EventDeviceSuccess, Message: "执行完毕", TotalCmd: len(e.Commands)}
	}
}

// backupWorker 是基于单个设备进行交互的备份动作集散流
func (e *Engine) backupWorker(ctx context.Context, dev config.DeviceAsset, wg *sync.WaitGroup) {
	defer wg.Done()

	// 备份模块不初始化 ProgressTracker，因此 EventBus 必须设为 nil 以避免死锁。
	exec := executor.NewDeviceExecutor(dev.IP, dev.Port, dev.Username, dev.Password, nil, func(ip, log, cmd string) executor.ErrorAction {
		return executor.ActionContinue
	})
	defer exec.Close()

	connectTimeout, err := time.ParseDuration(e.Settings.ConnectTimeout)
	if err != nil {
		connectTimeout = 10 * time.Second
	}

	if err := exec.Connect(ctx, connectTimeout); err != nil {
		logger.Error("Worker", dev.IP, "SSH建连失败: %v", err)
		e.failedBackups.Store(dev.IP, fmt.Sprintf("SSH连通信失败: %v", err))
		return
	}

	// 1. 挂载 SFTP 作为初步连接检测 (使用独立的新通道避免 PTY 被占用)
	sftpCfg := sshutil.Config{
		IP:       dev.IP,
		Port:     dev.Port,
		Username: dev.Username,
		Password: dev.Password,
		Timeout:  connectTimeout,
	}
	sftpClient, err := sftputil.NewSFTPClient(ctx, sftpCfg)
	if err != nil {
		// 如果 SFTP 连接失败，则探测 sftp 是否开启
		logger.Warn("Worker", dev.IP, "SFTP 挂载异常(底层异常: %v)，开始提取服务状态原因...", err)
		out, _ := exec.ExecuteCommandSync(ctx, "disp cur | inc sftp", 10*time.Second)
		if strings.Contains(strings.ToLower(out), "sftp server enable") {
			e.failedBackups.Store(dev.IP, fmt.Sprintf("SFTP建连失败（服务已配置，可能存在其他连通性或权限问题）。底层报错: %v", err))
		} else {
			e.failedBackups.Store(dev.IP, fmt.Sprintf("sftp服务未启动（配置文件无 sftp server enable）。底层报错: %v", err))
		}
		return
	}
	defer sftpClient.Close()

	// 2. 正常读取配置文件名称
	logger.Info("Worker", dev.IP, "SFTP会话成功挂载，准备查询下次启动配置文件...")
	// 某些设备需要禁用分屏，或者我们依赖“Next startup saved-configuration file”出现在第一屏回显中
	exec.ExecuteCommandSync(ctx, "screen-length 0 temporary", 2*time.Second) // 尽可能的规避翻页问题
	output, err := exec.ExecuteCommandSync(ctx, "display startup", 15*time.Second)
	if err != nil {
		e.failedBackups.Store(dev.IP, fmt.Sprintf("采集 startup 信息超时或失败: %v", err))
		return
	}

	// 3. 解析 "Next startup saved-configuration file:"
	re := regexp.MustCompile(`(?i)Next\s+startup\s+saved-configuration\s+file:\s+([^\s]+)`)
	matches := re.FindStringSubmatch(output)
	if len(matches) < 2 {
		e.failedBackups.Store(dev.IP, "未能在 display startup 回显中寻找到下次启动配置文件声明。")
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
		e.failedBackups.Store(dev.IP, "未配置下次启动配置文件(NULL)。")
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
	// 根据 SFTP 子系统不同，“flash:/1.cfg” 需要作为 “1.cfg” 或 “/1.cfg” 来获取。
	// pkg/sftp 通常基准当前目录解析 `1.cfg`，这对于 flash 根目录来说是没问题的。

	err = sftpClient.DownloadFile(cleanRemotePath, localPath)
	if err != nil {
		e.failedBackups.Store(dev.IP, fmt.Sprintf("SFTP 下载 %s 失败: %v", cleanRemotePath, err))
		return
	}

	logger.Info("Worker", dev.IP, "配置备份完成 -> %s", localPath)
}

// handleSuspend 被传递到每一个 `executor`，一旦匹配到 error 正则则回调该函数，挂起当前设备的 Goroutine。
// 使用 `promptMu` 互斥锁包围控制台的 STDIN 输入，保证多个设备同时发生 error 时，命令行不会争抢标准输入光标。
func (e *Engine) handleSuspend(ip string, logLine string, cmd string) executor.ErrorAction {
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
