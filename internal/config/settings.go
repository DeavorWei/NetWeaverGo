package config

import (
	"fmt"
	"strings"

	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
	"gorm.io/gorm"
)

// SSHAlgorithmSettings 是 models.SSHAlgorithmSettings 的别名，保持向后兼容
type SSHAlgorithmSettings = models.SSHAlgorithmSettings

// GlobalSettings 是 models.GlobalSettings 的别名，保持向后兼容
type GlobalSettings = models.GlobalSettings

// DefaultSettings 返回默认配置
func DefaultSettings() models.GlobalSettings {
	return models.GlobalSettings{
		MaxWorkers:        32,
		ConnectTimeout:    "10s",
		CommandTimeout:    "30s",
		StorageRoot:       GetPathManager().GetStorageRoot(),
		ErrorMode:         "pause",
		Debug:             false,
		Verbose:           false,
		SSHHostKeyPolicy:  "accept_new",
		SSHKnownHostsPath: GetPathManager().GetSSHKnownHostsPath(),
		SSHAlgorithms: models.SSHAlgorithmSettings{
			PresetMode: "secure", // 默认使用安全模式
		},
	}
}

// GetDefaultSSHAlgorithms 根据预设模式返回对应的算法配置
// 如果 presetMode 为空或 custom，返回 nil 表示使用代码内置的默认算法
func GetDefaultSSHAlgorithms(presetMode string) *models.SSHAlgorithmSettings {
	switch presetMode {
	case "secure":
		return &models.SSHAlgorithmSettings{
			PresetMode: "secure",
		}
	case "compatible":
		return &models.SSHAlgorithmSettings{
			PresetMode: "compatible",
		}
	case "custom":
		return &models.SSHAlgorithmSettings{
			PresetMode: "custom",
		}
	default:
		return nil
	}
}

// LoadSettings 从数据库读取设置，如果不存在则自动创建默认模板
func LoadSettings() (*models.GlobalSettings, bool, error) {
	logger.Verbose("Config", "-", "开始从数据库加载系统全局运行参数..")
	if DB == nil {
		return nil, false, fmt.Errorf("数据库未初始化")
	}

	var st models.GlobalSettings
	err := DB.First(&st, 1).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			logger.Debug("Config", "-", "数据库内未发现预存全局配置，正自动初始分配默认模板设置")
			st = DefaultSettings()
			st.ID = 1
			DB.Create(&st)
			// 应用默认调试设置
			ApplyDebugSettings(st.Debug, st.Verbose)
			return &st, true, nil
		}
		logger.Error("Config", "-", "从数据库加载全局设置失败: %v", err)
		return nil, false, err
	}

	if strings.TrimSpace(st.StorageRoot) == "" {
		st.StorageRoot = GetPathManager().GetStorageRoot()
	}
	if strings.TrimSpace(st.SSHHostKeyPolicy) == "" {
		st.SSHHostKeyPolicy = "accept_new"
	}
	if strings.TrimSpace(st.SSHKnownHostsPath) == "" {
		st.SSHKnownHostsPath = GetPathManager().GetSSHKnownHostsPath()
	}

	// 应用数据库中的调试设置
	ApplyDebugSettings(st.Debug, st.Verbose)

	logger.Verbose("Config", "-", "成功将现有全局设置从数据库载入内存")
	return &st, false, nil
}

// ApplyDebugSettings 应用调试日志设置到 logger 包
func ApplyDebugSettings(debug, verbose bool) {
	logger.EnableDebug = debug || verbose
	logger.EnableVerbose = verbose
	if verbose {
		logger.Verbose("Config", "-", "Verbose 模式已启用，将输出详细调试日志")
	} else if debug {
		logger.Debug("Config", "-", "Debug 模式已启用，将输出调试日志")
	} else {
		logger.Verbose("Config", "-", "调试日志已禁用")
	}
}

// SaveSettings 保存全局设置到数据库
func SaveSettings(settings models.GlobalSettings) error {
	logger.Debug("Config", "-", "准备将更新后的全局参数覆盖保存至本地数据库...")
	logger.Verbose("Config", "-", "保存内容: workers=%d, connect=%s, cmd=%s, error=%s, storageRoot=%s, debug=%v, verbose=%v",
		settings.MaxWorkers, settings.ConnectTimeout, settings.CommandTimeout, settings.ErrorMode,
		settings.StorageRoot, settings.Debug, settings.Verbose)
	logger.Verbose("Config", "-", "SSH主机密钥策略: policy=%s, known_hosts=%s", settings.SSHHostKeyPolicy, settings.SSHKnownHostsPath)
	logger.Verbose("Config", "-", "SSH算法配置: presetMode=%s, ciphers=%d, keyExchanges=%d, macs=%d, hostKeys=%d",
		settings.SSHAlgorithms.PresetMode,
		len(settings.SSHAlgorithms.Ciphers),
		len(settings.SSHAlgorithms.KeyExchanges),
		len(settings.SSHAlgorithms.MACs),
		len(settings.SSHAlgorithms.HostKeyAlgorithms))

	if DB == nil {
		logger.Error("Config", "-", "数据库未初始化，无法保存设置")
		return fmt.Errorf("数据库未初始化")
	}

	normalizedStorageRoot := NormalizeStorageRoot(settings.StorageRoot)
	if err := ValidateStorageRootWritable(normalizedStorageRoot); err != nil {
		return err
	}
	settings.StorageRoot = normalizedStorageRoot
	if strings.TrimSpace(settings.SSHHostKeyPolicy) == "" {
		settings.SSHHostKeyPolicy = "accept_new"
	}
	if strings.TrimSpace(settings.SSHKnownHostsPath) == "" {
		settings.SSHKnownHostsPath = GetPathManager().GetSSHKnownHostsPath()
	}

	pm := GetPathManager()
	currentStorageRoot := pm.GetStorageRoot()
	currentDBPath := pm.GetDBPath()

	settings.ID = 1
	err := DB.Save(&settings).Error
	if err != nil {
		logger.Error("Config", "-", "全局配置保存产生错误: %v", err)
		return err
	}

	if settings.StorageRoot != currentStorageRoot {
		if err := pm.UpdateStorageRoot(settings.StorageRoot); err != nil {
			return fmt.Errorf("更新存储根目录失败: %w", err)
		}

		if err := logger.ReconfigureGlobalLogger(pm.GetAppLogPath()); err != nil {
			return fmt.Errorf("切换日志目录失败: %w", err)
		}

		newDBPath := pm.GetDBPath()
		if err := MirrorDatabaseToPath(currentDBPath, newDBPath); err != nil {
			return fmt.Errorf("迁移数据库到新存储目录失败: %w", err)
		}
		logger.Info("Config", "-", "存储根目录已更新: %s", settings.StorageRoot)
	}

	// 保存后立即应用调试设置
	ApplyDebugSettings(settings.Debug, settings.Verbose)
	logger.Verbose("Config", "-", "全局参数保存落库完毕，ID=%d", settings.ID)
	return nil
}

// ResolveSSHHostKeyPolicy 解析 SSH 主机密钥校验策略与 known_hosts 路径。
func ResolveSSHHostKeyPolicy() (policy string, knownHostsPath string) {
	policy = "accept_new"
	knownHostsPath = GetPathManager().GetSSHKnownHostsPath()

	st, _, err := LoadSettings()
	if err != nil || st == nil {
		return policy, knownHostsPath
	}

	if p := strings.ToLower(strings.TrimSpace(st.SSHHostKeyPolicy)); p != "" {
		policy = p
	}
	if path := strings.TrimSpace(st.SSHKnownHostsPath); path != "" {
		knownHostsPath = path
	}
	return policy, knownHostsPath
}
