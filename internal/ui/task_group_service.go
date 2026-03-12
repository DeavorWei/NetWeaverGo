package ui

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/engine"
	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/report"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// TaskGroupService 任务组管理服务 - 负责任务组的增删改查和执行
type TaskGroupService struct {
	wailsApp *application.App

	// 控制运行状态
	isRunning bool
	mu        sync.Mutex
}

// NewTaskGroupService 创建任务组服务实例
func NewTaskGroupService() *TaskGroupService {
	return &TaskGroupService{}
}

// ServiceStartup Wails 服务启动生命周期钩子
func (s *TaskGroupService) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	s.wailsApp = application.Get()
	// 设置全局 SuspendManager 的 Wails App 实例（如果 EngineService 尚未设置）
	GetSuspendManager().SetWailsApp(s.wailsApp)
	logger.Info("UI", "-", "任务组服务已就绪")
	return nil
}

// ListTaskGroups 获取所有任务组列表
func (s *TaskGroupService) ListTaskGroups() ([]config.TaskGroup, error) {
	return config.ListTaskGroups()
}

// GetTaskGroup 根据 ID 获取单个任务组
func (s *TaskGroupService) GetTaskGroup(id string) (*config.TaskGroup, error) {
	return config.GetTaskGroup(id)
}

// CreateTaskGroup 创建新任务组
func (s *TaskGroupService) CreateTaskGroup(group config.TaskGroup) (*config.TaskGroup, error) {
	return config.CreateTaskGroup(group)
}

// UpdateTaskGroup 更新任务组
func (s *TaskGroupService) UpdateTaskGroup(id string, group config.TaskGroup) (*config.TaskGroup, error) {
	return config.UpdateTaskGroup(id, group)
}

// DeleteTaskGroup 删除任务组
func (s *TaskGroupService) DeleteTaskGroup(id string) error {
	return config.DeleteTaskGroup(id)
}

// ResolveSuspend 被前端调用（当用户在弹窗中选择动作后）
// 委托给全局 SuspendManager 处理
func (s *TaskGroupService) ResolveSuspend(sessionIDOrIP string, action string) {
	GetSuspendManager().Resolve(sessionIDOrIP, action)
}

// StartTaskGroup 启动任务组执行（并行执行模式）
func (s *TaskGroupService) StartTaskGroup(id string) error {
	// 首先获取全局引擎锁，防止与 EngineService 冲突
	if err := engine.TryAcquireEngine("taskgroup_" + id); err != nil {
		return err
	}
	// 确保在函数退出时释放全局锁
	defer engine.ReleaseEngine()

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

	// 获取任务组
	taskGroup, err := config.GetTaskGroup(id)
	if err != nil {
		return fmt.Errorf("获取任务组失败: %v", err)
	}

	// 更新状态为运行中
	config.UpdateTaskGroupStatus(id, "running")

	settings, _, err := config.LoadSettings()
	if err != nil {
		config.UpdateTaskGroupStatus(id, "failed")
		return err
	}

	// 获取所有设备
	allAssets, _, _, _, err := config.ParseOrGenerate(false)
	if err != nil {
		config.UpdateTaskGroupStatus(id, "failed")
		return err
	}

	// 创建设备 IP 到 Asset 的映射，用于快速查找
	assetMap := make(map[string]config.DeviceAsset)
	for _, asset := range allAssets {
		assetMap[asset.IP] = asset
	}

	// 用于追踪执行状态
	var executionWg sync.WaitGroup

	// 创建根 context 用于取消所有并行任务
	rootCtx, rootCancel := context.WithCancel(context.Background())
	defer rootCancel()

	// 使用 engine 实例 ID 追踪
	engineInstanceID := fmt.Sprintf("taskgroup_%s", id)

	switch taskGroup.Mode {
	case "group":
		// 模式A：一组命令 → 多台设备（所有设备执行相同命令集）
		err = s.executeModeA(taskGroup, assetMap, settings, rootCtx, &executionWg)
	case "binding":
		// 模式B：每台设备独立命令（每个任务项的设备执行各自的命令）
		err = s.executeModeB(taskGroup, assetMap, settings, rootCtx, &executionWg)
	default:
		err = fmt.Errorf("未知的任务组模式: %s", taskGroup.Mode)
	}

	if err != nil {
		config.UpdateTaskGroupStatus(id, "failed")
		return err
	}

	// 等待所有并行任务完成
	executionWg.Wait()

	// 确保所有事件都被处理完毕
	time.Sleep(100 * time.Millisecond)

	// 更新最终状态
	config.UpdateTaskGroupStatus(id, "completed")
	s.wailsApp.Event.Emit("engine:finished", map[string]interface{}{
		"instanceID": engineInstanceID,
		"status":     "completed",
	})

	return nil
}

// executeModeA 模式A执行：一组命令发送给所有设备
func (s *TaskGroupService) executeModeA(
	taskGroup *config.TaskGroup,
	assetMap map[string]config.DeviceAsset,
	settings *config.GlobalSettings,
	ctx context.Context,
	wg *sync.WaitGroup,
) error {
	// 收集所有选中的设备（去重）
	assetSet := make(map[string]bool)
	var allSelectedAssets []config.DeviceAsset

	// 获取命令组（模式A下所有任务项使用相同命令组）
	var commands []string

	for _, item := range taskGroup.Items {
		// 收集设备
		for _, ip := range item.DeviceIPs {
			if !assetSet[ip] {
				assetSet[ip] = true
				if asset, ok := assetMap[ip]; ok {
					allSelectedAssets = append(allSelectedAssets, asset)
				}
			}
		}

		// 获取第一个有效命令组
		if len(commands) == 0 && item.CommandGroupID != "" {
			group, err := config.GetCommandGroup(item.CommandGroupID)
			if err == nil && len(group.Commands) > 0 {
				commands = group.Commands
			}
		}
	}

	if len(allSelectedAssets) == 0 {
		return fmt.Errorf("未选择任何有效设备")
	}

	if len(commands) == 0 {
		return fmt.Errorf("命令组为空或未配置")
	}

	logger.Info("TaskGroup", "-", "模式A执行: %d 台设备, %d 条命令", len(allSelectedAssets), len(commands))

	// 创建单个 Engine 实例执行所有设备
	wg.Add(1)
	go func() {
		defer wg.Done()

		ng := engine.NewEngine(allSelectedAssets, commands, settings, false)
		ng.CustomSuspendHandler = GetSuspendManager().CreateHandler()

		// 启动事件监听
		go s.listenEvents(ng.FrontendBus)

		ng.Run(ctx)
	}()

	return nil
}

// executeModeB 模式B执行：每台设备执行各自的独立命令
func (s *TaskGroupService) executeModeB(
	taskGroup *config.TaskGroup,
	assetMap map[string]config.DeviceAsset,
	settings *config.GlobalSettings,
	ctx context.Context,
	wg *sync.WaitGroup,
) error {
	logger.Info("TaskGroup", "-", "模式B执行: %d 个任务项", len(taskGroup.Items))

	// 为每个任务项创建独立的 Engine 实例
	for _, item := range taskGroup.Items {
		// 过滤空命令
		var commands []string
		for _, cmd := range item.Commands {
			trimmed := strings.TrimSpace(cmd)
			if trimmed != "" {
				commands = append(commands, trimmed)
			}
		}

		if len(commands) == 0 {
			logger.Warn("TaskGroup", "-", "任务项命令为空，跳过")
			continue
		}

		// 收集该任务项的设备
		var itemAssets []config.DeviceAsset
		for _, ip := range item.DeviceIPs {
			if asset, ok := assetMap[ip]; ok {
				itemAssets = append(itemAssets, asset)
			}
		}

		if len(itemAssets) == 0 {
			logger.Warn("TaskGroup", "-", "任务项设备为空，跳过")
			continue
		}

		logger.Info("TaskGroup", "-", "启动独立任务: %d 台设备, %d 条命令", len(itemAssets), len(commands))

		// 为每个任务项创建独立的 Engine 实例
		// 注意：这里需要捕获循环变量，使用局部变量传递
		assets := itemAssets
		cmds := commands

		wg.Add(1)
		go func() {
			defer wg.Done()

			ng := engine.NewEngine(assets, cmds, settings, false)
			ng.CustomSuspendHandler = GetSuspendManager().CreateHandler()

			// 启动事件监听
			go s.listenEvents(ng.FrontendBus)

			ng.Run(ctx)
		}()
	}

	return nil
}

// listenEvents 事件监听协程（简化版）
func (s *TaskGroupService) listenEvents(frontendBus chan report.ExecutorEvent) {
	for ev := range frontendBus {
		if s.wailsApp != nil {
			s.wailsApp.Event.Emit("device:event", ev)
		}
	}
}
