package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/engine"
)

func main() {
	fmt.Println(`
    _   __     __ _       __                           ______     
   / | / /__  / /| |     / /__  ____ __   _____  _____/ ____/___  
  /  |/ / _ \/ __/ | /| / / _ \/ __ '/ | / / _ \/ ___/ / __/ __ \ 
 / /|  /  __/ /_ | |/ |/ /  __/ /_/ /| |/ /  __/ /  / /_/ / /_/ / 
/_/ |_/\___/\__/ |__/|__/\___/\__,_/ |___/\___/_/   \____/\____/  
   
              Go 并发网络自动化编排/配置集散部署工具
                 NetWeaverGo - v1.0 Framework
`)

	assets, commands, err := config.ParseOrGenerate()
	if err != nil {
		fmt.Printf("\n[配置/环境提示] %v\n", err)
		os.Exit(0)
	}

	if len(assets) == 0 {
		fmt.Println("\n[系统终止] 配置载入失败：没有找到合法的资产设备清单数据。")
		os.Exit(1)
	}
	if len(commands) == 0 {
		fmt.Println("\n[系统终止] 命令获取失败：需要至少通过文本配置一条待下发命令。")
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\n[!] 收到外界中断请求(CTRL+C), 正在广播取消命令以清理释放所有设备...")
		cancel()
	}()

	ng := engine.NewEngine(assets, commands)
	ng.Run(ctx)
}
