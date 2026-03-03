package engine

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/executor"
	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/sftputil"
	"github.com/NetWeaverGo/core/internal/sshutil"
)

// Engine 全局中央调度器，控制所有物理设备的并发执行流
type Engine struct {
	Devices    []config.DeviceAsset
	Commands   []string
	MaxWorkers int // 最大并发协程数量

	// 用于在控制台同一时刻串行化“挂起询问”操作，避免终端文字输出错位混淆
	promptMu sync.Mutex

	failedBackups sync.Map // 记录备份失败的设备和原因
}

// NewEngine 初始化并行执行引擎
func NewEngine(assets []config.DeviceAsset, commands []string) *Engine {
	return &Engine{
		Devices:    assets,
		Commands:   commands,
		MaxWorkers: 32, // 默认并发连接上限，未来可以提取到配置文件中
	}
}

// Run 启动 WorkerPool，正式分发任务
func (e *Engine) Run(ctx context.Context) {
	logger.Info("[NetWeaverGo] 控制台引擎启动，共准备向 %d 台设备下发 %d 条命令...", len(e.Devices), len(e.Commands))
	logger.Info("当前已配置全局并发安全限制 (MaxWorkers=%d)。\n设备回显位于 output/ 目录，系统日志位于 logs/app.log，正在分批并发下发中...", e.MaxWorkers)

	var wg sync.WaitGroup
	// 创建带缓冲的 channel 作为并发令牌桶
	sem := make(chan struct{}, e.MaxWorkers)

	for _, dev := range e.Devices {
		wg.Add(1)

		// 阻塞等待获取并发执行令牌，如果超过 MaxWorkers 则会在这里等待
		sem <- struct{}{}

		// 将 dev 作为参数传递，避免在闭包内捕获循环变量
		go func(device config.DeviceAsset) {
			defer func() {
				// 执行完毕后，归还令牌
				<-sem
			}()
			e.worker(ctx, device, &wg)
		}(dev)
	}

	wg.Wait()
	logger.Info("[NetWeaverGo] 所有设备的通信投递线程均已结束。安全退出。")
}

// RunBackup 启动基于 `-b` 参数的交换机备份流程专线
func (e *Engine) RunBackup(ctx context.Context) {
	logger.Info("=======================================")
	logger.Info("[NetWeaverGo SFTP-Backup] 备份模式启动")
	logger.Info("开始向 %d 台设备提取配置文件...", len(e.Devices))
	logger.Info("=======================================")

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

	// End of Backup, summarize failures
	logger.Info("\n========== [备份任务结束] ==========")
	var hasFailures bool
	e.failedBackups.Range(func(key, value interface{}) bool {
		hasFailures = true
		return false
	})

	if hasFailures {
		logger.Warn("[警告] 以下设备备份失败:")
		e.failedBackups.Range(func(key, value interface{}) bool {
			logger.Warn("  - IP: %-15s | 原因: %s", key, value)
			return true
		})
	} else {
		logger.Info("[成功] 所有设备的配置均已成功备份。")
	}
	logger.Info("====================================\n")
}

func (e *Engine) worker(ctx context.Context, dev config.DeviceAsset, wg *sync.WaitGroup) {
	defer wg.Done()

	exec := executor.NewDeviceExecutor(dev.IP, dev.Port, dev.Username, dev.Password, e.handleSuspend)
	defer exec.Close()

	if err := exec.Connect(ctx); err != nil {
		logger.Error("[!] 无法连接到设备 %s: %v", dev.IP, err)
		return
	}

	logger.Info("[+] 成功打通设备 %s 面板连接，开始执行命令脚本...", dev.IP)
	if err := exec.ExecutePlaybook(ctx, e.Commands); err != nil {
		logger.Error("[-] 设备 %s 终端流异常退出: %v", dev.IP, err)
	} else {
		logger.Info("[*] 设备 %s 命令全部下发成功。", dev.IP)
	}
}

// backupWorker 是基于单个设备进行交互的备份动作集散流
func (e *Engine) backupWorker(ctx context.Context, dev config.DeviceAsset, wg *sync.WaitGroup) {
	defer wg.Done()

	// 通过 Executor 建立底座 SSH 连接（为了防止阻塞中断，回调暂时传入默认略过日志）
	exec := executor.NewDeviceExecutor(dev.IP, dev.Port, dev.Username, dev.Password, func(ip, log, cmd string) executor.ErrorAction {
		return executor.ActionContinue
	})
	defer exec.Close()

	if err := exec.Connect(ctx); err != nil {
		logger.Error("[-] 设备 %s SSH建连失败: %v", dev.IP, err)
		e.failedBackups.Store(dev.IP, fmt.Sprintf("SSH连通信失败: %v", err))
		return
	}

	// 1. 挂载 SFTP 作为初步连接检测 (使用独立的新通道避免 PTY 被占用)
	sftpCfg := sshutil.Config{
		IP:       dev.IP,
		Port:     dev.Port,
		Username: dev.Username,
		Password: dev.Password,
		Timeout:  10 * time.Second,
	}
	sftpClient, err := sftputil.NewSFTPClient(ctx, sftpCfg)
	if err != nil {
		// 如果 SFTP 连接失败，则探测 sftp 是否开启
		logger.Warn("[%s] SFTP 挂载异常(底层异常: %v)，开始提取服务状态原因...", dev.IP, err)
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
	logger.Info("[%s] SFTP会话成功挂载，准备查询下次启动配置文件...", dev.IP)
	// Some devices need screen-length disable, or we rely on Next startup saved-configuration file occurring within first screen
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

	// Since SFTP on most Huawei/H3C requires the exact flash location:
	// "flash:/1.cfg" needs to be fetched as "1.cfg" or "/1.cfg" depending on the SFTP subsystem.
	// pkg/sftp typically resolves `1.cfg` against the current directory, which is fine for flash root.

	err = sftpClient.DownloadFile(cleanRemotePath, localPath)
	if err != nil {
		e.failedBackups.Store(dev.IP, fmt.Sprintf("SFTP 下载 %s 失败: %v", cleanRemotePath, err))
		return
	}

	logger.Info("[*] %s 配置备份完成 -> %s", dev.IP, localPath)
}

// handleSuspend 被传递到每一个 `executor`，一旦匹配到 error 正则则回调该函数，挂起当前设备的 Goroutine。
// 使用 `promptMu` 互斥锁包围控制台的 STDIN 输入，保证多个设备同时发生 error 时，命令行不会争抢标准输入光标。
func (e *Engine) handleSuspend(ip string, logLine string, cmd string) executor.ErrorAction {
	e.promptMu.Lock()
	defer e.promptMu.Unlock()

	// 记录到应用日志中
	logger.Warn("==================== [异常设备挂起干预] ====================")
	logger.Warn("=> 目标设备: %s", ip)
	logger.Warn("=> 触发指令: %s", cmd)
	logger.Warn("=> 回显日志: %s", strings.TrimSpace(logLine))

	// 控制台前台交互保持 fmt.Printf 不带时间格式化等前缀
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

		cleanStr := strings.TrimSpace(strings.ToUpper(input))
		switch cleanStr {
		case "C":
			logger.Info("-> 指令已接收：放行设备 %s，强制继续。", ip)
			return executor.ActionContinue
		case "S":
			logger.Info("-> 指令已接收：跳过设备 %s 的当前报错步骤，继续下一条。", ip)
			return executor.ActionSkip
		case "A":
			logger.Warn("-> 指令已接收：终止异常设备 %s 的运行流并脱离连接。", ip)
			return executor.ActionAbort
		}
		fmt.Print(">> 输入无效，仅支持 C、S 或 A: ")
	}
}
