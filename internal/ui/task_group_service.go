package ui

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/engine"
	"github.com/NetWeaverGo/core/internal/executor"
	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// TaskGroupService 任务组管理服务 - 负责任务组的增删改查和执行
type TaskGroupService struct {
	wailsApp *application.App

	// 控制运行状态
	isRunning bool
	mu        sync.Mutex

	// 挂起交互的通信频道
	suspendSignals map[string]chan executor.ErrorAction
	suspendMu      sync.Mutex
}

// NewTaskGroupService 创建任务组服务实例
func NewTaskGroupService() *TaskGroupService {
	return &TaskGroupService{
		suspendSignals: make(map[string]chan executor.ErrorAction),
	}
}

// ServiceStartup Wails 服务启动生命周期钩子
func (s *TaskGroupService) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	s.wailsApp = application.Get()
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

// Suspend 超时时间常量
const suspendTimeout = 5 * time.Minute

// WailsSuspendHandler 构建代理 Suspend 钩子替换原先的控制台询问方式
func (s *TaskGroupService) WailsSuspendHandler() executor.SuspendHandler {
	return func(ip string, logLine string, cmd string) executor.ErrorAction {
		actionCh := make(chan executor.ErrorAction, 1)

		s.suspendMu.Lock()
		s.suspendSignals[ip] = actionCh
		s.suspendMu.Unlock()

		defer func() {
			s.suspendMu.Lock()
			delete(s.suspendSignals, ip)
			s.suspendMu.Unlock()
		}()

		// 抛出悬停事件前台处理
		s.wailsApp.Event.Emit("engine:suspend_required", map[string]interface{}{
			"ip":      ip,
			"error":   logLine,
			"command": cmd,
		})

		logger.Warn("Engine", ip, "已向界面发射阻断警告，等待用户操作...")

		// 添加超时机制，防止 goroutine 永久阻塞
		select {
		case action := <-actionCh:
			return action
		case <-time.After(suspendTimeout):
			logger.Warn("Engine", ip, "Suspend 等待超时，自动中断设备连接")
			return executor.ActionAbort
		}
	}
}

// ResolveSuspend 被前端调用（当用户在弹窗中选择动作后）
func (s *TaskGroupService) ResolveSuspend(ip string, action string) {
	s.suspendMu.Lock()
	ch, exists := s.suspendSignals[ip]
	s.suspendMu.Unlock()

	if !exists {
		logger.Warn("Engine", ip, "找不到对应的挂机通信频道，可能任务已结束或超时")
		return
	}

	var errAction executor.ErrorAction
	switch action {
	case "C":
		errAction = executor.ActionContinue
	case "S":
		errAction = executor.ActionSkip
	case "A":
		errAction = executor.ActionAbort
	}

	select {
	case ch <- errAction:
	default:
	}
}

// StartTaskGroup 启动任务组执行
func (s *TaskGroupService) StartTaskGroup(id string) error {
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

	finalStatus := "completed"

	if taskGroup.Mode == "group" {
		// 模式A：一组命令 → 多台设备
		for _, item := range taskGroup.Items {
			// 根据 IP 筛选设备
			var selectedAssets []config.DeviceAsset
			ipSet := make(map[string]bool)
			for _, ip := range item.DeviceIPs {
				ipSet[ip] = true
			}
			for _, asset := range allAssets {
				if ipSet[asset.IP] {
					selectedAssets = append(selectedAssets, asset)
				}
			}

			if len(selectedAssets) == 0 {
				continue
			}

			// 获取命令组
			group, err := config.GetCommandGroup(item.CommandGroupID)
			if err != nil {
				logger.Warn("UI", "-", "获取命令组 %s 失败: %v", item.CommandGroupID, err)
				finalStatus = "failed"
				continue
			}

			ng := engine.NewEngine(selectedAssets, group.Commands, settings, false)
			ng.CustomSuspendHandler = s.WailsSuspendHandler()

			// 使用 WaitGroup 确保事件监听器在 Run() 之前准备好
			var listenerReady sync.WaitGroup
			listenerReady.Add(1)

			// 监听 FrontendBus 而不是 EventBus
			go func() {
				time.Sleep(50 * time.Millisecond) // 确保 Wails 事件系统准备好
				listenerReady.Done()              // 通知监听器已准备好
				for ev := range ng.FrontendBus {
					s.wailsApp.Event.Emit("device:event", ev)
				}
			}()

			// 等待监听器准备好再开始执行
			listenerReady.Wait()

			ctx, cancel := context.WithCancel(context.Background())
			ng.Run(ctx)
			// 确保所有事件都被处理完毕
			time.Sleep(100 * time.Millisecond)
			cancel()
		}
	} else if taskGroup.Mode == "binding" {
		// 模式B：每台设备独立命令
		for _, item := range taskGroup.Items {
			// 根据 IP 筛选设备
			var selectedAssets []config.DeviceAsset
			ipSet := make(map[string]bool)
			for _, ip := range item.DeviceIPs {
				ipSet[ip] = true
			}
			for _, asset := range allAssets {
				if ipSet[asset.IP] {
					selectedAssets = append(selectedAssets, asset)
				}
			}

			if len(selectedAssets) == 0 || len(item.Commands) == 0 {
				continue
			}

			// 过滤空命令
			var filtered []string
			for _, cmd := range item.Commands {
				trimmed := strings.TrimSpace(cmd)
				if trimmed != "" {
					filtered = append(filtered, trimmed)
				}
			}

			ng := engine.NewEngine(selectedAssets, filtered, settings, false)
			ng.CustomSuspendHandler = s.WailsSuspendHandler()

			// 使用 WaitGroup 确保事件监听器在 Run() 之前准备好
			var listenerReady sync.WaitGroup
			listenerReady.Add(1)

			// 监听 FrontendBus 而不是 EventBus
			go func() {
				time.Sleep(50 * time.Millisecond) // 确保 Wails 事件系统准备好
				listenerReady.Done()              // 通知监听器已准备好
				for ev := range ng.FrontendBus {
					s.wailsApp.Event.Emit("device:event", ev)
				}
			}()

			// 等待监听器准备好再开始执行
			listenerReady.Wait()

			ctx, cancel := context.WithCancel(context.Background())
			ng.Run(ctx)
			// 确保所有事件都被处理完毕
			time.Sleep(100 * time.Millisecond)
			cancel()
		}
	}

	config.UpdateTaskGroupStatus(id, finalStatus)
	s.wailsApp.Event.Emit("engine:finished")

	return nil
}
