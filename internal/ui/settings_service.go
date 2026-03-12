package ui

import (
	"context"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// SettingsService 设置管理服务 - 负责全局配置的加载和保存
type SettingsService struct {
	wailsApp *application.App
}

// NewSettingsService 创建设置服务实例
func NewSettingsService() *SettingsService {
	return &SettingsService{}
}

// ServiceStartup Wails 服务启动生命周期钩子
func (s *SettingsService) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	s.wailsApp = application.Get()
	return nil
}

// LoadSettings 获取合并后的主配置
func (s *SettingsService) LoadSettings() (*config.GlobalSettings, error) {
	logger.Debug("SettingsService", "-", "收到前端 LoadSettings 调用请求")
	settings, isNew, err := config.LoadSettings()
	if err != nil {
		logger.Error("SettingsService", "-", "LoadSettings 失败: %v", err)
		return nil, err
	}
	if isNew {
		logger.Debug("SettingsService", "-", "返回新创建的默认设置")
	} else {
		logger.DebugAll("SettingsService", "-", "返回已有设置: debug=%v, debugAll=%v", settings.Debug, settings.DebugAll)
	}
	return settings, err
}

// SaveSettings 保存全局设置到配置文件
func (s *SettingsService) SaveSettings(settings config.GlobalSettings) error {
	logger.Debug("SettingsService", "-", "收到前端 SaveSettings 调用请求")
	logger.DebugAll("SettingsService", "-", "接收到的设置: maxWorkers=%d, timeout=%s/%s, debug=%v, debugAll=%v, sshPreset=%s",
		settings.MaxWorkers,
		settings.ConnectTimeout,
		settings.CommandTimeout,
		settings.Debug,
		settings.DebugAll,
		settings.SSHAlgorithms.PresetMode)

	err := config.SaveSettings(settings)
	if err != nil {
		logger.Error("SettingsService", "-", "SaveSettings 处理失败: %v", err)
		return err
	}

	logger.Debug("SettingsService", "-", "SaveSettings 处理成功完成")
	return nil
}

// EnsureConfig 检查必需配置文件并返回是否有文件遗漏，以便前端提示
func (s *SettingsService) EnsureConfig() ([]config.DeviceAsset, []string, []string, error) {
	assets, commands, _, missingFiles, err := config.ParseOrGenerate(false)
	return assets, commands, missingFiles, err
}

// GetAppInfo 获取应用信息
func (s *SettingsService) GetAppInfo() map[string]string {
	return map[string]string{
		"name":    "NetWeaverGo",
		"version": "1.0.0",
	}
}

// LogInfo 记录信息日志（前端调用）
func (s *SettingsService) LogInfo(category, ip, message string) {
	logger.Info(category, ip, "%s", message)
}

// LogWarn 记录警告日志（前端调用）
func (s *SettingsService) LogWarn(category, ip, message string) {
	logger.Warn(category, ip, "%s", message)
}

// LogError 记录错误日志（前端调用）
func (s *SettingsService) LogError(category, ip, message string) {
	logger.Error(category, ip, "%s", message)
}
