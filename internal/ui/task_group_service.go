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

	if taskGroup.Mode == "group" {
		// 模式A：一组命令 → 多台设备
		// 优化：合并所有设备和命令，使用单一 Engine 实例
		var allSelectedAssets []config.DeviceAsset
		var allCommands []string
		assetSet := make(map[string]bool)

		for _, item := range taskGroup.Items {
			// 合并设备（去重）
			for _, ip := range item.DeviceIPs {
				if !assetSet[ip] {
					assetSet[ip] = true
					if asset, ok := assetMap[ip]; ok {
						allSelectedAssets = append(allSelectedAssets, asset)
					}
				}
			}
			// 获取第一个命令组（模式A下所有任务项使用相同命令组）
			if len(allCommands) == 0 && item.CommandGroupID != "" {
				group, err := config.GetCommandGroup(item.CommandGroupID)
				if err == nil && len(group.Commands) > 0 {
					allCommands = group.Commands
				}
			}
		}

		if len(allSelectedAssets) > 0 && len(allCommands) > 0 {
			executionWg.Add(1)
			go func() {
				defer executionWg.Done()

				ng := engine.NewEngine(allSelectedAssets, allCommands, settings, false)
				ng.CustomSuspendHandler = GetSuspendManager().CreateHandler()

				// 使用 channel 确保事件监听器在 Run() 之前准备好
				listenerReady := make(chan struct{})

				// 监听 FrontendBus
				go func() {
					// 等待 Wails 应用实例就绪
					for i := 0; i < 100; i++ { // 最多等待 1 秒
						if s.wailsApp != nil {
							break
						}
						time.Sleep(10 * time.Millisecond)
					}
					// 通知已准备进入消费循环
					close(listenerReady)
					// 开始消费 FrontendBus
					for ev := range ng.FrontendBus {
						if s.wailsApp != nil {
							s.wailsApp.Event.Emit("device:event", ev)
						}
					}
				}()

				// 等待监听器准备好
				<-listenerReady
				// 额外等待确保 goroutine 已进入 for-range 循环
				time.Sleep(10 * time.Millisecond)

				ng.Run(rootCtx)
			}()
		}
	} else if taskGroup.Mode == "binding" {
		// 模式B：每台设备独立命令
		// 使用单一 Engine 实例，合并所有设备和命令
		var allSelectedAssets []config.DeviceAsset
		var allCommands []string
		assetSet := make(map[string]bool)

		for _, item := range taskGroup.Items {
			// 过滤空命令
			var filtered []string
			for _, cmd := range item.Commands {
				trimmed := strings.TrimSpace(cmd)
				if trimmed != "" {
					filtered = append(filtered, trimmed)
				}
			}

			// 合并设备（去重）
			for _, ip := range item.DeviceIPs {
				if !assetSet[ip] {
					assetSet[ip] = true
					if asset, ok := assetMap[ip]; ok {
						allSelectedAssets = append(allSelectedAssets, asset)
					}
				}
			}

			// 合并命令
			allCommands = append(allCommands, filtered...)
		}

		if len(allSelectedAssets) > 0 && len(allCommands) > 0 {
			executionWg.Add(1)
			go func() {
				defer executionWg.Done()

				ng := engine.NewEngine(allSelectedAssets, allCommands, settings, false)
				ng.CustomSuspendHandler = GetSuspendManager().CreateHandler()

				// 使用 channel 确保事件监听器在 Run() 之前准备好
				listenerReady := make(chan struct{})

				// 监听 FrontendBus
				go func() {
					// 等待 Wails 应用实例就绪
					for i := 0; i < 100; i++ { // 最多等待 1 秒
						if s.wailsApp != nil {
							break
						}
						time.Sleep(10 * time.Millisecond)
					}
					// 通知已准备进入消费循环
					close(listenerReady)
					// 开始消费 FrontendBus
					for ev := range ng.FrontendBus {
						if s.wailsApp != nil {
							s.wailsApp.Event.Emit("device:event", ev)
						}
					}
				}()

				// 等待监听器准备好
				<-listenerReady
				// 额外等待确保 goroutine 已进入 for-range 循环
				time.Sleep(10 * time.Millisecond)

				ng.Run(rootCtx)
			}()
		}
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
