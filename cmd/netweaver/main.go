package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/engine"
	"github.com/NetWeaverGo/core/internal/logger"
)

func main() {
	if err := logger.InitGlobalLogger(); err != nil {
		fmt.Printf("日志系统初始化失败: %v\n", err)
		os.Exit(1)
	}

	// 提前加载全局配置（如果不存在则生成），由于日志系统初始化不在引擎内，后续日志路径也可以考虑受设置影响，但目前保持现状
	settings, isNewSettings, err := config.LoadSettings()
	if err != nil {
		fmt.Printf("[配置/环境提示] %v\n", err)
		os.Exit(0)
	}

	// 初始化全局随机数种子
	rand.Seed(time.Now().UnixNano())

	logger.Info(`
    _   __     __ _       __                           ______     
   / | / /__  / /| |     / /__  ____ __   _____  _____/ ____/___  
  /  |/ / _ \/ __/ | /| / / _ \/ __ '/ | / / _ \/ ___/ / __/ __ \ 
 / /|  /  __/ /_ | |/ |/ /  __/ /_/ /| |/ /  __/ /  / /_/ / /_/ / 
/_/ |_/\___/\__/ |__/|__/\___/\__,_/ |___/\___/_/   \____/\____/  
   
              Go 并发网络自动化编排/配置集散部署工具
                 NetWeaverGo - v1.0 Framework`)

	isBackup := flag.Bool("b", false, "启动备份模式，自动下载交换机配置并忽略配置命令")
	nonInteractive := flag.Bool("non-interactive", false, "无人值守模式：发生报错时自动执行 error_mode 策略且不挂起等待手动输入")
	flag.BoolVar(nonInteractive, "ni", false, "同 --non-interactive，无人值守模式(简写)")

	debugMode := flag.Bool("debug", false, "启用 DEBUG 级别日志输出到文件和控制台")
	debugAllMode := flag.Bool("debugall", false, "启用全量且详细的 DEBUG 级别日志输出到文件和控制台")

	flag.Parse()

	// 初始化日志全局状态
	if *debugAllMode {
		logger.EnableDebugAll = true
		logger.EnableDebug = true // 开启全量则必定开启普通 debug
	} else if *debugMode {
		logger.EnableDebug = true
	}

	assets, commands, _, missingFiles, err := config.ParseOrGenerate(*isBackup)
	if err != nil {
		logger.Error("[系统错误] %v", err)
		os.Exit(1)
	}

	if isNewSettings {
		missingFiles = append(missingFiles, "settings.yaml")
	}

	if len(missingFiles) > 0 {
		logger.Error("[配置/环境提示] 已在当前目录生成模板文件: %s，请填写内容后重新运行程序", strings.Join(missingFiles, ", "))
		os.Exit(0)
	}

	if len(assets) == 0 {
		logger.Error("[系统终止] 配置载入失败：没有找到合法的资产设备清单数据。")
		os.Exit(1)
	}

	if !*isBackup {
		if len(commands) == 0 {
			logger.Error("[系统终止] 命令获取失败：需要至少通过文本配置一条待下发命令。")
			os.Exit(1)
		}
	} else {
		logger.Info("[!] 检测到 -b 标志，应用已经进入备份模式（备份操作中将忽略 config.txt 指令）。")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		logger.Warn("[!] 收到外界中断请求(CTRL+C), 正在广播取消命令以清理释放所有设备...")
		cancel()
	}()

	ng := engine.NewEngine(assets, commands, settings, *nonInteractive)

	if *isBackup {
		ng.RunBackup(ctx)
	} else {
		ng.Run(ctx)
	}
}
