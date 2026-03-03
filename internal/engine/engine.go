package engine

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/executor"
)

// Engine 全局中央调度器，控制所有物理设备的并发执行流
type Engine struct {
	Devices    []config.DeviceAsset
	Commands   []string
	MaxWorkers int // 最大并发协程数量

	// 用于在控制台同一时刻串行化“挂起询问”操作，避免终端文字输出错位混淆
	promptMu sync.Mutex
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
	fmt.Printf("\n[NetWeaverGo] 控制台引擎启动，共准备向 %d 台设备下发 %d 条命令...\n", len(e.Devices), len(e.Commands))
	fmt.Printf("当前已配置全局并发安全限制 (MaxWorkers=%d)。\n设备日志将实时记录于 logs/ 目录，正在分批并发下发中...\n", e.MaxWorkers)

	var wg sync.WaitGroup
	// 创建带缓冲的 channel 作为并发令牌桶
	sem := make(chan struct{}, e.MaxWorkers)

	for _, dev := range e.Devices {
		wg.Add(1)

		// 阻塞等待获取并发执行令牌，如果超过 MaxWorkers 则会在这里等待
		sem <- struct{}{}

		// 将 dev 作为参数传递，避免在闭包内捕获循环变量（虽然 Go 1.22+ 已经修复了这个由循环变量导致的问题，但显式传递也是好的编码习惯）
		go func(device config.DeviceAsset) {
			defer func() {
				// 执行完毕后，归还令牌
				<-sem
			}()
			e.worker(ctx, device, &wg)
		}(dev)
	}

	wg.Wait()
	fmt.Println("\n[NetWeaverGo] 所有设备的通信投递线程均已结束。安全退出。")
}

func (e *Engine) worker(ctx context.Context, dev config.DeviceAsset, wg *sync.WaitGroup) {
	defer wg.Done()

	exec := executor.NewDeviceExecutor(dev.IP, dev.Port, dev.Username, dev.Password, e.handleSuspend)
	defer exec.Close()

	if err := exec.Connect(ctx); err != nil {
		fmt.Printf("\n[!] 无法连接到设备 %s: %v\n", dev.IP, err)
		return
	}

	fmt.Printf("[+] 成功打通设备 %s 面板连接，开始执行命令脚本...\n", dev.IP)
	if err := exec.ExecutePlaybook(ctx, e.Commands); err != nil {
		fmt.Printf("\n[-] 设备 %s 终端流异常退出: %v\n", dev.IP, err)
	} else {
		fmt.Printf("[*] 设备 %s 命令全部下发成功，完成审计日志。\n", dev.IP)
	}
}

// handleSuspend 被传递到每一个 `executor`，一旦匹配到 error 正则则回调该函数，挂起当前设备的 Goroutine。
// 使用 `promptMu` 互斥锁包围控制台的 STDIN 输入，保证多个设备同时发生 error 时，命令行不会争抢标准输入光标。
func (e *Engine) handleSuspend(ip string, logLine string, cmd string) executor.ErrorAction {
	e.promptMu.Lock()
	defer e.promptMu.Unlock()

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
			fmt.Println("-> 指令已接收：放行此设备，强制继续。")
			return executor.ActionContinue
		case "S":
			fmt.Println("-> 指令已接收：忽略当前步骤返回继续。")
			return executor.ActionSkip
		case "A":
			fmt.Println("-> 指令已接收：终止当前异常设备的运行流并脱离连接。")
			return executor.ActionAbort
		}
		fmt.Print(">> 输入无效，仅支持 C、S 或 A: ")
	}
}
