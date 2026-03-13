package ui

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/engine"
	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/report"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// TaskGroupService 任务组管理服务 - 负责任务组的增删改查和执行
type TaskGroupService struct {
	wailsApp *application.App
}

// NewTaskGroupService 创建任务组服务实例
func NewTaskGroupService() *TaskGroupService {
	return &TaskGroupService{}
}

// ServiceStartup Wails 服务启动生命周期钩子
func (s *TaskGroupService) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	s.wailsApp = application.Get()
	getExecutionManager().SetWailsApp(s.wailsApp)
	// 设置全局 SuspendManager 的 Wails App 实例（如果 EngineService 尚未设置）
	GetSuspendManager().SetWailsApp(s.wailsApp)
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
	taskGroup, err := config.GetTaskGroup(id)
	if err != nil {
		return fmt.Errorf("获取任务组失败: %v", err)
	}

	if err := config.UpdateTaskGroupStatus(id, "running"); err != nil {
		return err
	}

	settings, _, err := config.LoadSettings()
	if err != nil {
		config.UpdateTaskGroupStatus(id, "failed")
		return err
	}

	allAssets, err := config.LoadDeviceAssets()
	if err != nil {
		config.UpdateTaskGroupStatus(id, "failed")
		return err
	}

	assetMap := make(map[string]config.DeviceAsset, len(allAssets))
	for _, asset := range allAssets {
		assetMap[asset.IP] = asset
	}

	var finalStatus string
	switch taskGroup.Mode {
	case "group":
		finalStatus, err = s.executeModeA(taskGroup, assetMap, settings)
	case "binding":
		finalStatus, err = s.executeModeB(taskGroup, assetMap, settings)
	default:
		err = fmt.Errorf("未知的任务组模式: %s", taskGroup.Mode)
	}

	if err != nil {
		config.UpdateTaskGroupStatus(id, "failed")
		return err
	}

	return config.UpdateTaskGroupStatus(id, finalStatus)
}

// executeModeA 模式A执行：一组命令发送给所有设备
func (s *TaskGroupService) executeModeA(
	taskGroup *config.TaskGroup,
	assetMap map[string]config.DeviceAsset,
	settings *config.GlobalSettings,
) (string, error) {
	assetSet := make(map[string]bool)
	var allSelectedAssets []config.DeviceAsset
	var commands []string

	for _, item := range taskGroup.Items {
		for _, ip := range item.DeviceIPs {
			if !assetSet[ip] {
				assetSet[ip] = true
				if asset, ok := assetMap[ip]; ok {
					allSelectedAssets = append(allSelectedAssets, asset)
				}
			}
		}

		if len(commands) == 0 && item.CommandGroupID != "" {
			group, err := config.GetCommandGroup(item.CommandGroupID)
			if err == nil && len(group.Commands) > 0 {
				commands = group.Commands
			}
		}
	}

	if len(allSelectedAssets) == 0 {
		return "", fmt.Errorf("未选择任何有效设备")
	}
	if len(commands) == 0 {
		return "", fmt.Errorf("命令组为空或未配置")
	}

	logger.Info("TaskGroup", "-", "模式A执行: %d 台设备, %d 条命令", len(allSelectedAssets), len(commands))

	ng := engine.NewEngine(allSelectedAssets, commands, settings, false)
	ng.CustomSuspendHandler = GetSuspendManager().CreateHandler()

	tracker, err := getExecutionManager().RunEngine(
		ng,
		"task_group",
		taskGroup.ID,
		taskGroup.Name,
		func(ctx context.Context) error {
			return ng.Run(ctx)
		},
	)
	if err != nil {
		return "", err
	}

	return deriveTaskGroupStatus(tracker), nil
}

// executeModeB 模式B执行：每台设备执行各自的独立命令
func (s *TaskGroupService) executeModeB(
	taskGroup *config.TaskGroup,
	assetMap map[string]config.DeviceAsset,
	settings *config.GlobalSettings,
) (string, error) {
	logger.Info("TaskGroup", "-", "模式B执行: %d 个任务项", len(taskGroup.Items))

	type taskRun struct {
		assets   []config.DeviceAsset
		commands []string
	}

	var runs []taskRun
	uniqueIPs := make(map[string]struct{})

	for _, item := range taskGroup.Items {
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

		var itemAssets []config.DeviceAsset
		for _, ip := range item.DeviceIPs {
			if asset, ok := assetMap[ip]; ok {
				itemAssets = append(itemAssets, asset)
				uniqueIPs[asset.IP] = struct{}{}
			}
		}
		if len(itemAssets) == 0 {
			logger.Warn("TaskGroup", "-", "任务项设备为空，跳过")
			continue
		}

		runs = append(runs, taskRun{
			assets:   itemAssets,
			commands: commands,
		})
	}

	if len(runs) == 0 {
		return "", fmt.Errorf("任务组中没有可执行的任务项")
	}

	totalDevices := len(uniqueIPs)
	if totalDevices == 0 {
		return "", fmt.Errorf("任务组中没有可执行设备")
	}

	coordinator := engine.NewEngine(nil, nil, settings, false)
	session, err := getExecutionManager().BeginCompositeExecution(
		coordinator,
		"task_group",
		taskGroup.ID,
		taskGroup.Name,
		totalDevices,
	)
	if err != nil {
		return "", err
	}
	defer session.Finish()

	if err := session.TransitionTo(engine.StateStarting); err != nil {
		return "", err
	}
	if err := session.TransitionTo(engine.StateRunning); err != nil {
		return "", err
	}

	var (
		runWG       sync.WaitGroup
		errMu       sync.Mutex
		firstRunErr error
		forwarders  = make([]<-chan struct{}, 0, len(runs))
	)

	for _, run := range runs {
		logger.Info("TaskGroup", "-", "启动独立任务: %d 台设备, %d 条命令", len(run.assets), len(run.commands))

		ng := engine.NewEngine(run.assets, run.commands, settings, false)
		ng.CustomSuspendHandler = GetSuspendManager().CreateHandler()

		forwarders = append(forwarders, getExecutionManager().listenEvents(ng.FrontendBus, session.Tracker().TrackEvent))

		runWG.Add(1)
		go func(ng *engine.Engine) {
			defer runWG.Done()

			if err := ng.Run(session.Context()); err != nil {
				errMu.Lock()
				if firstRunErr == nil {
					firstRunErr = err
				}
				errMu.Unlock()
			}
		}(ng)
	}

	runWG.Wait()
	for _, done := range forwarders {
		<-done
	}

	if firstRunErr != nil {
		return "", firstRunErr
	}

	return deriveTaskGroupStatus(session.Tracker()), nil
}

func deriveTaskGroupStatus(tracker *report.ProgressTracker) string {
	total, finished, successCount, errorCount := tracker.GetStats()

	if total == 0 {
		return "failed"
	}
	if errorCount > 0 {
		return "failed"
	}
	if finished < total {
		return "failed"
	}
	if successCount < total {
		return "failed"
	}

	return "completed"
}
