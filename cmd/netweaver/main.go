package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/engine"
	"github.com/NetWeaverGo/core/internal/logger"
)

func main() {
	if err := logger.InitGlobalLogger(); err != nil {
		fmt.Printf("日志系统初始化失败: %v\n", err)
		os.Exit(1)
	}

	logger.Info(`
    _   __     __ _       __                           ______     
   / | / /__  / /| |     / /__  ____ __   _____  _____/ ____/___  
  /  |/ / _ \/ __/ | /| / / _ \/ __ '/ | / / _ \/ ___/ / __/ __ \ 
 / /|  /  __/ /_ | |/ |/ /  __/ /_/ /| |/ /  __/ /  / /_/ / /_/ / 
/_/ |_/\___/\__/ |__/|__/\___/\__,_/ |___/\___/_/   \____/\____/  
   
              Go 并发网络自动化编排/配置集散部署工具
                 NetWeaverGo - v1.0 Framework`)

	isBackup := flag.Bool("b", false, "启动备份模式，自动下载交换机配置并忽略配置命令")
	flag.Parse()

	assets, commands, err := config.ParseOrGenerate(*isBackup)
	if err != nil {
		logger.Error("[配置/环境提示] %v", err)
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

	ng := engine.NewEngine(assets, commands)

	if *isBackup {
		ng.RunBackup(ctx)
	} else {
		ng.Run(ctx)
	}
}
