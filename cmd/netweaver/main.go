package main

import (
	"fmt"
	"io/fs"
	"os"

	"github.com/NetWeaverGo/core"
	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/parser"
	"github.com/NetWeaverGo/core/internal/taskexec"
	"github.com/NetWeaverGo/core/internal/ui"
	"github.com/wailsapp/wails/v3/pkg/application"
)

func main() {
	pm := config.GetPathManager()
	if err := pm.EnsureDirectories(); err != nil {
		fmt.Printf("存储目录初始化失败: %v\n", err)
		os.Exit(1)
	}

	if err := logger.InitGlobalLogger(pm.GetAppLogPath()); err != nil {
		fmt.Printf("日志系统初始化失败: %v\n", err)
		os.Exit(1)
	}

	// 启动数据库并进行数据迁移校验
	if err := config.InitDB(); err != nil {
		logger.Error("System", "-", "数据库初始化失败: %v", err)
		os.Exit(1)
	}
	if err := taskexec.AutoMigrate(config.DB); err != nil {
		logger.Error("System", "-", "统一运行时数据库迁移失败: %v", err)
		os.Exit(1)
	}

	// 初始化运行时配置管理器
	if err := config.InitRuntimeManager(config.DB); err != nil {
		logger.Error("System", "-", "运行时配置管理器初始化失败: %v", err)
		os.Exit(1)
	}

	runGUI()
}

func runGUI() {
	logger.Info("System", "-", "正在初始化 Wails GUI 环境...")

	// 初始化解析器管理器
	parserManager := parser.NewParserManager()
	if err := parserManager.Bootstrap(); err != nil {
		logger.Error("System", "-", "解析器管理器初始化失败: %v", err)
		os.Exit(1)
	}
	logger.Info("System", "-", "解析器管理器已启动")

	// 创建应用级共享的统一任务执行服务（阶段1：统一运行时服务化）
	taskExecutionService := taskexec.NewTaskExecutionService(config.DB, parserManager)
	taskExecutionService.Start()
	logger.Info("System", "-", "统一任务执行服务已启动")

	// 创建各独立服务实例
	deviceService := ui.NewDeviceService()
	commandGroupService := ui.NewCommandGroupService()
	settingsService := ui.NewSettingsService()
	queryService := ui.NewQueryService()
	forgeService := ui.NewForgeService()
	executionHistoryService := ui.NewExecutionHistoryService()
	executionHistoryService.SetTaskExecutionService(taskExecutionService)       // 设置统一运行时服务（阶段5）
	executionHistoryService.SetRepository(taskExecutionService.GetRepository()) // 注入 Repository
	taskGroupService := ui.NewTaskGroupService()
	taskGroupService.SetTaskExecutionService(taskExecutionService) // 设置共享运行时（阶段1）
	topologyCommandService := ui.NewTopologyCommandService()
	planCompareService := ui.NewPlanCompareService()
	// 创建统一任务执行UI服务（Wails暴露层）
	taskExecutionUIService := ui.NewTaskExecutionUIService(taskExecutionService)

	// 修正：修正嵌入文件系统的路径级联问题
	// core.FrontendAssets 包含了 "frontend/dist" 这一层，我们需要提取其子 FS
	assetsFS, err := fs.Sub(core.FrontendAssets, "frontend/dist")
	if err != nil {
		logger.Error("System", "-", "无法初始化嵌入资产子集: %v", err)
		return
	}

	// 调试：打印资源列表，确保 index.html 存在于子 FS 根目录
	logger.Info("System", "-", "--- [自检] 嵌入资源列表 ---")
	fs.WalkDir(assetsFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		logger.Info("System", "-", "  > Asset File: %s (IsDir: %v)", path, d.IsDir())
		return nil
	})
	logger.Info("System", "-", "--- [自检] 结束 ---")

	app := application.New(application.Options{
		Name:        "NetWeaverGo",
		Description: "网络自动化巡检与动作集散引擎",
		Icon:        core.AppIcon,
		Services: []application.Service{
			application.NewService(deviceService),
			application.NewService(commandGroupService),
			application.NewService(settingsService),
			application.NewService(taskGroupService),
			application.NewService(queryService),
			application.NewService(forgeService),
			application.NewService(executionHistoryService),
			application.NewService(topologyCommandService),
			application.NewService(planCompareService),
			application.NewService(taskExecutionUIService), // 统一任务执行UI服务（阶段1）
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assetsFS),
		},
	})

	// 通过 app.Window.NewWithOptions 创建窗口，使其注册到 app 实例中
	// 注意：必须使用 app 的 WindowManager 方法，而非顶层 application.NewWindow()
	// 后者仅构造对象但不会被 app 管理，导致 Run 时无窗口可显示
	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "NetWeaverGo Control Center",
		BackgroundColour: application.NewRGB(15, 17, 23),
		URL:              "/",
		Width:            1440,
		Height:           900,
		Frameless:        true,
		DisableResize:    false,
		MinWidth:         1024,
		MinHeight:        768,
	})

	logger.Info("System", "-", "正在启动 Wails 应用主循环...")
	if err := app.Run(); err != nil {
		logger.Error("System", "-", "GUI 应用程序崩溃或异常退出: %v", err)
	}
}
