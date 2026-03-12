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

// ==================== 运行时配置管理接口 ====================

// RuntimeConfigData 运行时配置数据结构（用于Wails绑定）
type RuntimeConfigData struct {
	Timeouts struct {
		Command    int `json:"command"`
		Connection int `json:"connection"`
		Handshake  int `json:"handshake"`
		ShortCmd   int `json:"shortCmd"`
		LongCmd    int `json:"longCmd"`
	} `json:"timeouts"`
	Limits struct {
		MaxLogsPerDevice     int `json:"maxLogsPerDevice"`
		MaxLogLength         int `json:"maxLogLength"`
		LogTruncateThreshold int `json:"logTruncateThreshold"`
		MaxConcurrentDevices int `json:"maxConcurrentDevices"`
	} `json:"limits"`
	Engine struct {
		WorkerCount           int `json:"workerCount"`
		EventBufferSize       int `json:"eventBufferSize"`
		FallbackEventCapacity int `json:"fallbackEventCapacity"`
	} `json:"engine"`
	Buffers struct {
		DefaultSize int `json:"defaultSize"`
		SmallSize   int `json:"smallSize"`
		LargeSize   int `json:"largeSize"`
	} `json:"buffers"`
	Pagination struct {
		LineThreshold int `json:"lineThreshold"`
		CheckInterval int `json:"checkInterval"`
	} `json:"pagination"`
}

// GetRuntimeConfig 获取运行时配置
func (s *SettingsService) GetRuntimeConfig() (RuntimeConfigData, error) {
	logger.Debug("SettingsService", "-", "收到前端 GetRuntimeConfig 调用请求")

	manager := config.GetRuntimeManager()
	cfg := manager.GetConfig()

	var response RuntimeConfigData
	response.Timeouts.Command = cfg.Timeouts.Command
	response.Timeouts.Connection = cfg.Timeouts.Connection
	response.Timeouts.Handshake = cfg.Timeouts.Handshake
	response.Timeouts.ShortCmd = cfg.Timeouts.ShortCmd
	response.Timeouts.LongCmd = cfg.Timeouts.LongCmd
	response.Limits.MaxLogsPerDevice = cfg.Limits.MaxLogsPerDevice
	response.Limits.MaxLogLength = cfg.Limits.MaxLogLength
	response.Limits.LogTruncateThreshold = cfg.Limits.LogTruncateThreshold
	response.Limits.MaxConcurrentDevices = cfg.Limits.MaxConcurrentDevices
	response.Engine.WorkerCount = cfg.Engine.WorkerCount
	response.Engine.EventBufferSize = cfg.Engine.EventBufferSize
	response.Engine.FallbackEventCapacity = cfg.Engine.FallbackEventCapacity
	response.Buffers.DefaultSize = cfg.Buffers.DefaultSize
	response.Buffers.SmallSize = cfg.Buffers.SmallSize
	response.Buffers.LargeSize = cfg.Buffers.LargeSize
	response.Pagination.LineThreshold = cfg.Pagination.LineThreshold
	response.Pagination.CheckInterval = cfg.Pagination.CheckInterval

	logger.DebugAll("SettingsService", "-", "返回运行时配置: timeouts=[cmd=%d, conn=%d, hs=%d, short=%d, long=%d], "+
		"limits=[logs=%d, len=%d, trunc=%d, dev=%d], engine=[workers=%d, buf=%d, fallback=%d], "+
		"buffers=[def=%d, small=%d, large=%d], pagination=[lines=%d, interval=%d]",
		response.Timeouts.Command, response.Timeouts.Connection, response.Timeouts.Handshake,
		response.Timeouts.ShortCmd, response.Timeouts.LongCmd,
		response.Limits.MaxLogsPerDevice, response.Limits.MaxLogLength,
		response.Limits.LogTruncateThreshold, response.Limits.MaxConcurrentDevices,
		response.Engine.WorkerCount, response.Engine.EventBufferSize, response.Engine.FallbackEventCapacity,
		response.Buffers.DefaultSize, response.Buffers.SmallSize, response.Buffers.LargeSize,
		response.Pagination.LineThreshold, response.Pagination.CheckInterval)

	return response, nil
}

// UpdateRuntimeConfig 更新运行时配置（热更新）
func (s *SettingsService) UpdateRuntimeConfig(data RuntimeConfigData) error {
	logger.Debug("SettingsService", "-", "收到前端 UpdateRuntimeConfig 调用请求")
	logger.DebugAll("SettingsService", "-", "接收到的运行时配置: timeouts=[cmd=%d, conn=%d, hs=%d, short=%d, long=%d], "+
		"limits=[logs=%d, len=%d, trunc=%d, dev=%d], engine=[workers=%d, buf=%d, fallback=%d], "+
		"buffers=[def=%d, small=%d, large=%d], pagination=[lines=%d, interval=%d]",
		data.Timeouts.Command, data.Timeouts.Connection, data.Timeouts.Handshake,
		data.Timeouts.ShortCmd, data.Timeouts.LongCmd,
		data.Limits.MaxLogsPerDevice, data.Limits.MaxLogLength,
		data.Limits.LogTruncateThreshold, data.Limits.MaxConcurrentDevices,
		data.Engine.WorkerCount, data.Engine.EventBufferSize, data.Engine.FallbackEventCapacity,
		data.Buffers.DefaultSize, data.Buffers.SmallSize, data.Buffers.LargeSize,
		data.Pagination.LineThreshold, data.Pagination.CheckInterval)

	cfg := config.RuntimeConfig{}
	cfg.Timeouts.Command = data.Timeouts.Command
	cfg.Timeouts.Connection = data.Timeouts.Connection
	cfg.Timeouts.Handshake = data.Timeouts.Handshake
	cfg.Timeouts.ShortCmd = data.Timeouts.ShortCmd
	cfg.Timeouts.LongCmd = data.Timeouts.LongCmd
	cfg.Limits.MaxLogsPerDevice = data.Limits.MaxLogsPerDevice
	cfg.Limits.MaxLogLength = data.Limits.MaxLogLength
	cfg.Limits.LogTruncateThreshold = data.Limits.LogTruncateThreshold
	cfg.Limits.MaxConcurrentDevices = data.Limits.MaxConcurrentDevices
	cfg.Engine.WorkerCount = data.Engine.WorkerCount
	cfg.Engine.EventBufferSize = data.Engine.EventBufferSize
	cfg.Engine.FallbackEventCapacity = data.Engine.FallbackEventCapacity
	cfg.Buffers.DefaultSize = data.Buffers.DefaultSize
	cfg.Buffers.SmallSize = data.Buffers.SmallSize
	cfg.Buffers.LargeSize = data.Buffers.LargeSize
	cfg.Pagination.LineThreshold = data.Pagination.LineThreshold
	cfg.Pagination.CheckInterval = data.Pagination.CheckInterval

	manager := config.GetRuntimeManager()
	if err := manager.UpdateConfig(cfg); err != nil {
		logger.Error("SettingsService", "-", "UpdateRuntimeConfig 失败: %v", err)
		return err
	}

	logger.Info("SettingsService", "-", "运行时配置更新成功")
	return nil
}

// ResetRuntimeConfigToDefault 重置运行时配置为默认值
func (s *SettingsService) ResetRuntimeConfigToDefault() error {
	logger.Debug("SettingsService", "-", "收到前端 ResetRuntimeConfigToDefault 调用请求")

	if err := config.ResetRuntimeSettingsToDefault(config.DB); err != nil {
		logger.Error("SettingsService", "-", "重置运行时配置失败: %v", err)
		return err
	}

	// 重新加载配置到内存
	manager := config.GetRuntimeManager()
	cfg, err := config.LoadRuntimeConfig(config.DB)
	if err != nil {
		logger.Error("SettingsService", "-", "重新加载配置失败: %v", err)
		return err
	}

	// 更新内存配置
	manager.UpdateConfig(cfg)

	logger.DebugAll("SettingsService", "-", "重置后的运行时配置: timeouts=[cmd=%d, conn=%d, hs=%d, short=%d, long=%d], "+
		"limits=[logs=%d, len=%d, trunc=%d, dev=%d], engine=[workers=%d, buf=%d, fallback=%d], "+
		"buffers=[def=%d, small=%d, large=%d], pagination=[lines=%d, interval=%d]",
		cfg.Timeouts.Command, cfg.Timeouts.Connection, cfg.Timeouts.Handshake,
		cfg.Timeouts.ShortCmd, cfg.Timeouts.LongCmd,
		cfg.Limits.MaxLogsPerDevice, cfg.Limits.MaxLogLength,
		cfg.Limits.LogTruncateThreshold, cfg.Limits.MaxConcurrentDevices,
		cfg.Engine.WorkerCount, cfg.Engine.EventBufferSize, cfg.Engine.FallbackEventCapacity,
		cfg.Buffers.DefaultSize, cfg.Buffers.SmallSize, cfg.Buffers.LargeSize,
		cfg.Pagination.LineThreshold, cfg.Pagination.CheckInterval)

	logger.Info("SettingsService", "-", "运行时配置已重置为默认值")
	return nil
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
