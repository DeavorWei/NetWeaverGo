package ui

import (
	"context"
	"fmt"
	"time"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/engine"
	"github.com/NetWeaverGo/core/internal/models"
	"github.com/NetWeaverGo/core/internal/report"
	"github.com/NetWeaverGo/core/internal/repository"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// 快照推送配置
const (
	SnapshotInterval       = 200 * time.Millisecond // 快照推送间隔
	SnapshotEventName      = "execution:snapshot"   // 快照事件名称
	ExecutionFinishedEvent = "engine:finished"      // 执行完成事件名称
)

// EngineService 引擎控制服务 - 负责任务执行和状态管理
type EngineService struct {
	wailsApp *application.App
	repo     repository.DeviceRepository
}

// NewEngineService 创建引擎服务实例
func NewEngineService() *EngineService {
	return &EngineService{
		repo: repository.NewDeviceRepository(),
	}
}

// NewEngineServiceWithRepo 使用指定 Repository 创建引擎服务实例（用于测试）
func NewEngineServiceWithRepo(repo repository.DeviceRepository) *EngineService {
	return &EngineService{
		repo: repo,
	}
}

// ServiceStartup Wails 服务启动生命周期钩子
func (s *EngineService) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	s.wailsApp = application.Get()
	getExecutionManager().SetWailsApp(s.wailsApp)
	// 设置全局 SuspendManager 的 Wails App 实例
	GetSuspendManager().SetWailsApp(s.wailsApp)
	return nil
}

// IsRunning 检查引擎是否正在运行（使用全局状态）
func (s *EngineService) IsRunning() bool {
	return getExecutionManager().IsRunning()
}

// StopEngine 停止正在执行的任务
func (s *EngineService) StopEngine() error {
	return getExecutionManager().StopEngine()
}

// ResolveSuspend 被前端调用（当用户在弹窗中选择动作后）
// 委托给全局 SuspendManager 处理
func (s *EngineService) ResolveSuspend(sessionIDOrIP string, action string) {
	GetSuspendManager().Resolve(sessionIDOrIP, action)
}

// StartEngine 启动核心下发动作
func (s *EngineService) StartEngine() error {
	return s.runEngineWithConfig(nil, 0)
}

// StartEngineWithSelection 使用选定的设备和命令组启动引擎
func (s *EngineService) StartEngineWithSelection(deviceIPs []string, commandGroupID uint) error {
	return s.runEngineWithConfig(deviceIPs, commandGroupID)
}

// runEngineWithConfig 统一的引擎执行方法
// deviceIPs 为 nil 时使用全部设备，commandGroupID 为 0 时使用默认命令
func (s *EngineService) runEngineWithConfig(deviceIPs []string, commandGroupID uint) error {
	settings, _, err := config.LoadSettings()
	if err != nil {
		return err
	}

	// 准备设备和命令
	assets, commands, err := s.prepareAssetsAndCommands(deviceIPs, commandGroupID)
	if err != nil {
		return err
	}

	if len(assets) == 0 || len(commands) == 0 {
		return fmt.Errorf("设备资产或命令集为空，请先在设备页和命令组中完成配置")
	}

	// 初始化 Engine
	ng := engine.NewEngine(assets, commands, settings, false)
	ng.CustomSuspendHandler = GetSuspendManager().CreateHandler()

	taskName := "批量执行"
	mode := "manual"
	if commandGroupID > 0 {
		if group, err := config.GetCommandGroup(commandGroupID); err == nil {
			taskName = group.Name
		}
		mode = "group"
	}

	// 构建执行元数据
	meta := &ExecutionMeta{
		RunnerSource: "engine_service",
		RunnerID:     "",
		TaskName:     taskName,
		Mode:         mode,
	}

	_, err = getExecutionManager().RunEngineWithMeta(
		ng,
		meta,
		func(ctx context.Context) error {
			return ng.Run(ctx)
		},
	)

	return err
}

// GetEngineState 获取引擎当前状态
// 已废弃：前端应使用 GetExecutionSnapshot() 获取运行态信息
// 此方法仅保留用于向后兼容
func (s *EngineService) GetEngineState() map[string]interface{} {
	return getExecutionManager().GetEngineState()
}

// prepareAssetsAndCommands 准备设备和命令
func (s *EngineService) prepareAssetsAndCommands(deviceIPs []string, commandGroupID uint) ([]models.DeviceAsset, []string, error) {
	// 获取所有设备
	allAssets, err := s.repo.FindAll()
	if err != nil {
		return nil, nil, err
	}

	var assets []models.DeviceAsset
	var commands []string

	// 筛选设备
	if len(deviceIPs) > 0 {
		ipSet := make(map[string]bool)
		for _, ip := range deviceIPs {
			ipSet[ip] = true
		}
		for _, asset := range allAssets {
			if ipSet[asset.IP] {
				assets = append(assets, asset)
			}
		}
	} else {
		assets = allAssets
	}

	// 获取命令
	if commandGroupID > 0 {
		group, err := config.GetCommandGroup(commandGroupID)
		if err != nil {
			return nil, nil, fmt.Errorf("获取命令组失败: %v", err)
		}
		if len(group.Commands) > 0 {
			commands = group.Commands
		}
	} else {
		cmds, err := config.LoadDefaultCommands()
		if err != nil {
			return nil, nil, err
		}
		commands = cmds
	}

	return assets, commands, nil
}

// StartBackup 启动核心备份动作
func (s *EngineService) StartBackup() error {
	settings, _, err := config.LoadSettings()
	if err != nil {
		return err
	}

	assets, err := s.repo.FindAll()
	if err != nil {
		return err
	}

	if len(assets) == 0 {
		return fmt.Errorf("设备资产为空，请先在设备页添加需要备份的设备")
	}

	// 初始化 Engine
	ng := engine.NewEngine(assets, nil, settings, false)
	ng.CustomSuspendHandler = GetSuspendManager().CreateHandler()

	// 构建执行元数据
	meta := &ExecutionMeta{
		RunnerSource: "backup_service",
		RunnerID:     "",
		TaskName:     "配置备份",
		Mode:         "backup",
	}

	_, err = getExecutionManager().RunEngineWithMeta(
		ng,
		meta,
		func(ctx context.Context) error {
			return ng.RunBackup(ctx)
		},
	)

	return err
}

// GetExecutionSnapshot 获取当前执行的快照
// 前端调用此方法获取完整的执行状态，无需前端计算
func (s *EngineService) GetExecutionSnapshot() *report.ExecutionSnapshot {
	return getExecutionManager().GetExecutionSnapshot()
}
