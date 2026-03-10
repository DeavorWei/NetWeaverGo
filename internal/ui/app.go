package ui

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/executor"
	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// AppService 包装所有要暴露给 Wails Frontend 的绑定方法
// 作为门面(Facade)，内部委托给各个独立服务处理具体业务
// 这样既保持了前端绑定的兼容性，又实现了后端代码的职责分离
type AppService struct {
	wailsApp *application.App
	ctx      context.Context

	// 控制运行状态
	isRunning bool
	mu        sync.Mutex

	// 挂起交互的通信频道
	suspendSignals map[string]chan executor.ErrorAction
	suspendMu      sync.Mutex

	// 内部服务实例
	deviceService       *DeviceService
	commandGroupService *CommandGroupService
	settingsService     *SettingsService
	engineService       *EngineService
}

// NewAppService 创建应用服务实例
func NewAppService() *AppService {
	return &AppService{
		suspendSignals:      make(map[string]chan executor.ErrorAction),
		deviceService:       NewDeviceService(),
		commandGroupService: NewCommandGroupService(),
		settingsService:     NewSettingsService(),
		engineService:       NewEngineService(),
	}
}

// ServiceStartup Wails 应用启动时的生命周期钩子
func (a *AppService) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	a.ctx = ctx
	a.wailsApp = application.Get()

	// 初始化内部服务
	a.engineService.wailsApp = a.wailsApp

	logger.Info("UI", "-", "Wails 图形界面服务已就绪...")
	return nil
}

// ========== 设置管理 API (委托给 SettingsService) ==========

// LoadSettings 获取合并后的主配置
func (a *AppService) LoadSettings() (*config.GlobalSettings, error) {
	return a.settingsService.LoadSettings()
}

// SaveSettings 保存全局设置到配置文件
func (a *AppService) SaveSettings(settings config.GlobalSettings) error {
	return a.settingsService.SaveSettings(settings)
}

// EnsureConfig 检查必需配置文件并返回是否有文件遗漏，以便前端提示
func (a *AppService) EnsureConfig() ([]config.DeviceAsset, []string, []string, error) {
	return a.settingsService.EnsureConfig()
}

// ========== 设备管理 API (委托给 DeviceService) ==========

// ListDevices 获取设备列表
func (a *AppService) ListDevices() ([]config.DeviceAsset, error) {
	return a.deviceService.ListDevices()
}

// AddDevice 新增设备
func (a *AppService) AddDevice(device config.DeviceAsset) error {
	return a.deviceService.AddDevice(device)
}

// UpdateDevice 更新设备
func (a *AppService) UpdateDevice(index int, device config.DeviceAsset) error {
	return a.deviceService.UpdateDevice(index, device)
}

// DeleteDevice 删除设备
func (a *AppService) DeleteDevice(index int) error {
	return a.deviceService.DeleteDevice(index)
}

// SaveDevices 批量保存设备列表
func (a *AppService) SaveDevices(devices []config.DeviceAsset) error {
	return a.deviceService.SaveDevices(devices)
}

// GetProtocolDefaultPorts 获取协议默认端口映射
func (a *AppService) GetProtocolDefaultPorts() map[string]int {
	return a.deviceService.GetProtocolDefaultPorts()
}

// GetValidProtocols 获取有效协议列表
func (a *AppService) GetValidProtocols() []string {
	return a.deviceService.GetValidProtocols()
}

// ========== 命令管理 API (委托给 CommandGroupService) ==========

// GetCommands 获取命令列表
func (a *AppService) GetCommands() ([]string, error) {
	return a.commandGroupService.GetCommands()
}

// SaveCommands 保存命令列表
func (a *AppService) SaveCommands(commands []string) error {
	// 过滤空行
	var filtered []string
	for _, cmd := range commands {
		trimmed := strings.TrimSpace(cmd)
		if trimmed != "" {
			filtered = append(filtered, trimmed)
		}
	}

	if len(filtered) == 0 {
		return fmt.Errorf("命令列表不能为空")
	}

	return a.commandGroupService.SaveCommands(filtered)
}

// ========== 命令组管理 API (委托给 CommandGroupService) ==========

// ListCommandGroups 获取所有命令组列表
func (a *AppService) ListCommandGroups() ([]config.CommandGroup, error) {
	return a.commandGroupService.ListCommandGroups()
}

// GetCommandGroup 根据 ID 获取单个命令组
func (a *AppService) GetCommandGroup(id string) (*config.CommandGroup, error) {
	return a.commandGroupService.GetCommandGroup(id)
}

// CreateCommandGroup 创建新命令组
func (a *AppService) CreateCommandGroup(group config.CommandGroup) (*config.CommandGroup, error) {
	return a.commandGroupService.CreateCommandGroup(group)
}

// UpdateCommandGroup 更新命令组
func (a *AppService) UpdateCommandGroup(id string, group config.CommandGroup) (*config.CommandGroup, error) {
	return a.commandGroupService.UpdateCommandGroup(id, group)
}

// DeleteCommandGroup 删除命令组
func (a *AppService) DeleteCommandGroup(id string) error {
	return a.commandGroupService.DeleteCommandGroup(id)
}

// DuplicateCommandGroup 复制命令组
func (a *AppService) DuplicateCommandGroup(id string) (*config.CommandGroup, error) {
	return a.commandGroupService.DuplicateCommandGroup(id)
}

// ImportCommandGroup 从文件导入命令组
func (a *AppService) ImportCommandGroup(filePath string) (*config.CommandGroup, error) {
	return a.commandGroupService.ImportCommandGroup(filePath)
}

// ExportCommandGroup 导出命令组到文件
func (a *AppService) ExportCommandGroup(id string, filePath string) error {
	return a.commandGroupService.ExportCommandGroup(id, filePath)
}

// ========== 引擎控制 API (委托给 EngineService) ==========

// ResolveSuspend 被前端调用（当用户在弹窗中选择动作后）
func (a *AppService) ResolveSuspend(ip string, action string) {
	a.engineService.ResolveSuspend(ip, action)
}

// StartEngineWails 启动核心下发动作（UI包裹层）
func (a *AppService) StartEngineWails() error {
	// 设置事件发射器
	a.engineService.wailsApp = a.wailsApp
	return a.engineService.StartEngine()
}

// StartEngineWithSelection 使用选定的设备和命令组启动引擎
func (a *AppService) StartEngineWithSelection(deviceIPs []string, commandGroupID string) error {
	a.engineService.wailsApp = a.wailsApp
	return a.engineService.StartEngineWithSelection(deviceIPs, commandGroupID)
}

// StartBackupWails 启动核心备份动作（UI包裹层）
func (a *AppService) StartBackupWails() error {
	a.engineService.wailsApp = a.wailsApp
	return a.engineService.StartBackup()
}

// ========== 任务组管理 API ==========

// ListTaskGroups 获取所有任务组列表
func (a *AppService) ListTaskGroups() ([]config.TaskGroup, error) {
	return config.ListTaskGroups()
}

// GetTaskGroup 根据 ID 获取单个任务组
func (a *AppService) GetTaskGroup(id string) (*config.TaskGroup, error) {
	return config.GetTaskGroup(id)
}

// CreateTaskGroup 创建新任务组
func (a *AppService) CreateTaskGroup(group config.TaskGroup) (*config.TaskGroup, error) {
	return config.CreateTaskGroup(group)
}

// UpdateTaskGroup 更新任务组
func (a *AppService) UpdateTaskGroup(id string, group config.TaskGroup) (*config.TaskGroup, error) {
	return config.UpdateTaskGroup(id, group)
}

// DeleteTaskGroup 删除任务组
func (a *AppService) DeleteTaskGroup(id string) error {
	return config.DeleteTaskGroup(id)
}

// StartTaskGroup 启动任务组执行
func (a *AppService) StartTaskGroup(id string) error {
	a.mu.Lock()
	if a.isRunning {
		a.mu.Unlock()
		return fmt.Errorf("引擎正在运行中，请勿重复启动")
	}
	a.isRunning = true
	a.mu.Unlock()

	defer func() {
		a.mu.Lock()
		a.isRunning = false
		a.mu.Unlock()
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
			ng.CustomSuspendHandler = a.WailsSuspendHandler()

			go func() {
				for ev := range ng.EventBus {
					a.wailsApp.Event.Emit("device:event", ev)
				}
			}()

			ctx, cancel := context.WithCancel(context.Background())
			ng.Run(ctx)
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

			ng := engine.NewEngine(selectedAssets, item.Commands, settings, false)
			ng.CustomSuspendHandler = a.WailsSuspendHandler()

			go func() {
				for ev := range ng.EventBus {
					a.wailsApp.Event.Emit("device:event", ev)
				}
			}()

			ctx, cancel := context.WithCancel(context.Background())
			ng.Run(ctx)
			cancel()
		}
	}

	config.UpdateTaskGroupStatus(id, finalStatus)
	a.wailsApp.Event.Emit("engine:finished")

	return nil
}
