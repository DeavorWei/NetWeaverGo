package config

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/NetWeaverGo/core/internal/logger"
	"gorm.io/gorm"
)

// RuntimeSetting 运行时配置表
type RuntimeSetting struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Category  string    `gorm:"index" json:"category"` // 配置分类: timeout, limit, engine, buffer, ssh, pagination
	Key       string    `gorm:"index" json:"key"`      // 配置键名
	Value     string    `json:"value"`                 // JSON序列化的值
	UpdatedAt time.Time `json:"updatedAt"`
}

// RuntimeConfig 运行时配置结构体（用于前后端交互）
type RuntimeConfig struct {
	// 超时配置（毫秒）
	Timeouts struct {
		Command    int `json:"command"`    // 命令执行超时（毫秒）
		Connection int `json:"connection"` // SSH连接超时（毫秒）
		Handshake  int `json:"handshake"`  // SSH握手超时（毫秒）
		ShortCmd   int `json:"shortCmd"`   // 短命令超时（毫秒）
		LongCmd    int `json:"longCmd"`    // 长命令超时（毫秒）
	} `json:"timeouts"`

	// 限制配置
	Limits struct {
		MaxLogsPerDevice     int `json:"maxLogsPerDevice"`     // 每设备最大日志数
		MaxLogLength         int `json:"maxLogLength"`         // 单条日志最大长度
		LogTruncateThreshold int `json:"logTruncateThreshold"` // 日志截断阈值(0-100)
		MaxConcurrentDevices int `json:"maxConcurrentDevices"` // 最大并发设备数
	} `json:"limits"`

	// 引擎配置
	Engine struct {
		WorkerCount           int `json:"workerCount"`           // 工作协程数
		EventBufferSize       int `json:"eventBufferSize"`       // 事件缓冲区大小
		FallbackEventCapacity int `json:"fallbackEventCapacity"` // 后备事件容量
	} `json:"engine"`

	// 缓冲区配置
	Buffers struct {
		DefaultSize int `json:"defaultSize"` // 默认缓冲区大小
		SmallSize   int `json:"smallSize"`   // 小缓冲区大小
		LargeSize   int `json:"largeSize"`   // 大缓冲区大小
	} `json:"buffers"`

	// 分页检测配置
	Pagination struct {
		LineThreshold int `json:"lineThreshold"` // 行数阈值
		CheckInterval int `json:"checkInterval"` // 检测间隔（毫秒）
	} `json:"pagination"`
}

// DefaultRuntimeConfig 返回默认运行时配置
func DefaultRuntimeConfig() RuntimeConfig {
	cfg := RuntimeConfig{}

	// 超时配置（转换为毫秒）
	cfg.Timeouts.Command = int(DefaultCommandTimeout.Milliseconds())
	cfg.Timeouts.Connection = int(ConnectionTimeout.Milliseconds())
	cfg.Timeouts.Handshake = int(HandshakeTimeout.Milliseconds())
	cfg.Timeouts.ShortCmd = int(ShortCommandTimeout.Milliseconds())
	cfg.Timeouts.LongCmd = int(LongCommandTimeout.Milliseconds())

	// 限制配置
	cfg.Limits.MaxLogsPerDevice = MaxLogsPerDevice
	cfg.Limits.MaxLogLength = MaxLogLength
	cfg.Limits.LogTruncateThreshold = LogTruncateThreshold
	cfg.Limits.MaxConcurrentDevices = MaxConcurrentDevices

	// 引擎配置
	cfg.Engine.WorkerCount = DefaultWorkerCount
	cfg.Engine.EventBufferSize = EventBufferSize
	cfg.Engine.FallbackEventCapacity = FallbackEventCapacity

	// 缓冲区配置
	cfg.Buffers.DefaultSize = DefaultBufferSize
	cfg.Buffers.SmallSize = SmallBufferSize
	cfg.Buffers.LargeSize = LargeBufferSize

	// 分页检测配置
	cfg.Pagination.LineThreshold = PaginationLineThreshold
	cfg.Pagination.CheckInterval = int(PaginationCheckInterval.Milliseconds())

	return cfg
}

// InitRuntimeSettings 初始化运行时配置表
func InitRuntimeSettings(db *gorm.DB) error {
	logger.Debug("Config", "-", "初始化运行时配置表")

	// 自动迁移表结构
	if err := db.AutoMigrate(&RuntimeSetting{}); err != nil {
		return fmt.Errorf("运行时配置表迁移失败: %v", err)
	}

	// 检查是否需要初始化默认值
	var count int64
	db.Model(&RuntimeSetting{}).Count(&count)

	if count == 0 {
		logger.Info("Config", "-", "运行时配置表为空，初始化默认值")
		return ResetRuntimeSettingsToDefault(db)
	}

	return nil
}

// ResetRuntimeSettingsToDefault 重置所有配置为默认值
func ResetRuntimeSettingsToDefault(db *gorm.DB) error {
	defaults := DefaultRuntimeConfig()

	// 清除旧配置
	db.Where("1 = 1").Delete(&RuntimeSetting{})

	// 插入新配置
	settings := []RuntimeSetting{
		// 超时配置
		{Category: "timeout", Key: "command", Value: strconv.Itoa(defaults.Timeouts.Command)},
		{Category: "timeout", Key: "connection", Value: strconv.Itoa(defaults.Timeouts.Connection)},
		{Category: "timeout", Key: "handshake", Value: strconv.Itoa(defaults.Timeouts.Handshake)},
		{Category: "timeout", Key: "shortCmd", Value: strconv.Itoa(defaults.Timeouts.ShortCmd)},
		{Category: "timeout", Key: "longCmd", Value: strconv.Itoa(defaults.Timeouts.LongCmd)},

		// 限制配置
		{Category: "limit", Key: "maxLogsPerDevice", Value: strconv.Itoa(defaults.Limits.MaxLogsPerDevice)},
		{Category: "limit", Key: "maxLogLength", Value: strconv.Itoa(defaults.Limits.MaxLogLength)},
		{Category: "limit", Key: "logTruncateThreshold", Value: strconv.Itoa(defaults.Limits.LogTruncateThreshold)},
		{Category: "limit", Key: "maxConcurrentDevices", Value: strconv.Itoa(defaults.Limits.MaxConcurrentDevices)},

		// 引擎配置
		{Category: "engine", Key: "workerCount", Value: strconv.Itoa(defaults.Engine.WorkerCount)},
		{Category: "engine", Key: "eventBufferSize", Value: strconv.Itoa(defaults.Engine.EventBufferSize)},
		{Category: "engine", Key: "fallbackEventCapacity", Value: strconv.Itoa(defaults.Engine.FallbackEventCapacity)},

		// 缓冲区配置
		{Category: "buffer", Key: "defaultSize", Value: strconv.Itoa(defaults.Buffers.DefaultSize)},
		{Category: "buffer", Key: "smallSize", Value: strconv.Itoa(defaults.Buffers.SmallSize)},
		{Category: "buffer", Key: "largeSize", Value: strconv.Itoa(defaults.Buffers.LargeSize)},

		// 分页配置
		{Category: "pagination", Key: "lineThreshold", Value: strconv.Itoa(defaults.Pagination.LineThreshold)},
		{Category: "pagination", Key: "checkInterval", Value: strconv.Itoa(defaults.Pagination.CheckInterval)},
	}

	for _, s := range settings {
		if err := db.Create(&s).Error; err != nil {
			return fmt.Errorf("创建运行时配置失败 [%s.%s]: %v", s.Category, s.Key, err)
		}
	}

	logger.Info("Config", "-", "运行时配置默认值初始化完成，共 %d 项", len(settings))
	return nil
}

// LoadRuntimeConfig 从数据库加载运行时配置
func LoadRuntimeConfig(db *gorm.DB) (RuntimeConfig, error) {
	config := DefaultRuntimeConfig()

	var settings []RuntimeSetting
	if err := db.Find(&settings).Error; err != nil {
		return config, fmt.Errorf("加载运行时配置失败: %v", err)
	}

	for _, s := range settings {
		if err := applyRuntimeSetting(&config, s); err != nil {
			logger.Warn("Config", "-", "应用运行时配置失败 [%s.%s]: %v", s.Category, s.Key, err)
		}
	}

	return config, nil
}

// applyRuntimeSetting 将单个设置应用到配置对象
func applyRuntimeSetting(cfg *RuntimeConfig, setting RuntimeSetting) error {
	val, err := strconv.Atoi(setting.Value)
	if err != nil {
		return fmt.Errorf("无效的配置值: %s", setting.Value)
	}

	switch setting.Category {
	case "timeout":
		switch setting.Key {
		case "command":
			cfg.Timeouts.Command = val
		case "connection":
			cfg.Timeouts.Connection = val
		case "handshake":
			cfg.Timeouts.Handshake = val
		case "shortCmd":
			cfg.Timeouts.ShortCmd = val
		case "longCmd":
			cfg.Timeouts.LongCmd = val
		}
	case "limit":
		switch setting.Key {
		case "maxLogsPerDevice":
			cfg.Limits.MaxLogsPerDevice = val
		case "maxLogLength":
			cfg.Limits.MaxLogLength = val
		case "logTruncateThreshold":
			cfg.Limits.LogTruncateThreshold = val
		case "maxConcurrentDevices":
			cfg.Limits.MaxConcurrentDevices = val
		}
	case "engine":
		switch setting.Key {
		case "workerCount":
			cfg.Engine.WorkerCount = val
		case "eventBufferSize":
			cfg.Engine.EventBufferSize = val
		case "fallbackEventCapacity":
			cfg.Engine.FallbackEventCapacity = val
		}
	case "buffer":
		switch setting.Key {
		case "defaultSize":
			cfg.Buffers.DefaultSize = val
		case "smallSize":
			cfg.Buffers.SmallSize = val
		case "largeSize":
			cfg.Buffers.LargeSize = val
		}
	case "pagination":
		switch setting.Key {
		case "lineThreshold":
			cfg.Pagination.LineThreshold = val
		case "checkInterval":
			cfg.Pagination.CheckInterval = val
		}
	}

	return nil
}

// SaveRuntimeConfig 保存运行时配置到数据库
func SaveRuntimeConfig(db *gorm.DB, config RuntimeConfig) error {
	settings := []RuntimeSetting{
		// 超时配置
		{Category: "timeout", Key: "command", Value: strconv.Itoa(config.Timeouts.Command)},
		{Category: "timeout", Key: "connection", Value: strconv.Itoa(config.Timeouts.Connection)},
		{Category: "timeout", Key: "handshake", Value: strconv.Itoa(config.Timeouts.Handshake)},
		{Category: "timeout", Key: "shortCmd", Value: strconv.Itoa(config.Timeouts.ShortCmd)},
		{Category: "timeout", Key: "longCmd", Value: strconv.Itoa(config.Timeouts.LongCmd)},

		// 限制配置
		{Category: "limit", Key: "maxLogsPerDevice", Value: strconv.Itoa(config.Limits.MaxLogsPerDevice)},
		{Category: "limit", Key: "maxLogLength", Value: strconv.Itoa(config.Limits.MaxLogLength)},
		{Category: "limit", Key: "logTruncateThreshold", Value: strconv.Itoa(config.Limits.LogTruncateThreshold)},
		{Category: "limit", Key: "maxConcurrentDevices", Value: strconv.Itoa(config.Limits.MaxConcurrentDevices)},

		// 引擎配置
		{Category: "engine", Key: "workerCount", Value: strconv.Itoa(config.Engine.WorkerCount)},
		{Category: "engine", Key: "eventBufferSize", Value: strconv.Itoa(config.Engine.EventBufferSize)},
		{Category: "engine", Key: "fallbackEventCapacity", Value: strconv.Itoa(config.Engine.FallbackEventCapacity)},

		// 缓冲区配置
		{Category: "buffer", Key: "defaultSize", Value: strconv.Itoa(config.Buffers.DefaultSize)},
		{Category: "buffer", Key: "smallSize", Value: strconv.Itoa(config.Buffers.SmallSize)},
		{Category: "buffer", Key: "largeSize", Value: strconv.Itoa(config.Buffers.LargeSize)},

		// 分页配置
		{Category: "pagination", Key: "lineThreshold", Value: strconv.Itoa(config.Pagination.LineThreshold)},
		{Category: "pagination", Key: "checkInterval", Value: strconv.Itoa(config.Pagination.CheckInterval)},
	}

	// 使用事务批量更新
	return db.Transaction(func(tx *gorm.DB) error {
		for _, s := range settings {
			result := tx.Model(&RuntimeSetting{}).
				Where("category = ? AND key = ?", s.Category, s.Key).
				Update("value", s.Value)

			if result.Error != nil {
				return fmt.Errorf("更新配置失败 [%s.%s]: %v", s.Category, s.Key, result.Error)
			}

			if result.RowsAffected == 0 {
				// 如果不存在则创建
				if err := tx.Create(&s).Error; err != nil {
					return fmt.Errorf("创建配置失败 [%s.%s]: %v", s.Category, s.Key, err)
				}
			}
		}
		return nil
	})
}

// RuntimeConfigManager 运行时配置管理器（支持热更新）
type RuntimeConfigManager struct {
	db     *gorm.DB
	config RuntimeConfig
	mu     sync.RWMutex
}

var (
	runtimeManager *RuntimeConfigManager
	once           sync.Once
)

// InitRuntimeManager 初始化全局配置管理器
func InitRuntimeManager(db *gorm.DB) error {
	var err error
	once.Do(func() {
		// 初始化表结构
		if err = InitRuntimeSettings(db); err != nil {
			return
		}

		// 加载配置
		cfg, loadErr := LoadRuntimeConfig(db)
		if loadErr != nil {
			err = loadErr
			return
		}

		runtimeManager = &RuntimeConfigManager{
			db:     db,
			config: cfg,
		}

		logger.Info("Config", "-", "运行时配置管理器初始化完成")
	})
	return err
}

// GetRuntimeManager 获取配置管理器实例
func GetRuntimeManager() *RuntimeConfigManager {
	if runtimeManager == nil {
		panic("运行时配置管理器未初始化")
	}
	return runtimeManager
}

// GetConfig 获取当前配置（只读副本）
func (m *RuntimeConfigManager) GetConfig() RuntimeConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// UpdateConfig 更新配置（热更新）
func (m *RuntimeConfigManager) UpdateConfig(config RuntimeConfig) error {
	// 保存到数据库
	if err := SaveRuntimeConfig(m.db, config); err != nil {
		return err
	}

	// 更新内存中的配置
	m.mu.Lock()
	m.config = config
	m.mu.Unlock()

	logger.Info("Config", "-", "运行时配置已热更新")
	return nil
}

// GetCommandTimeout 获取命令超时时间（辅助方法）
func (m *RuntimeConfigManager) GetCommandTimeout() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return time.Duration(m.config.Timeouts.Command) * time.Millisecond
}

// GetConnectionTimeout 获取连接超时时间
func (m *RuntimeConfigManager) GetConnectionTimeout() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return time.Duration(m.config.Timeouts.Connection) * time.Millisecond
}

// GetMaxLogsPerDevice 获取每设备最大日志数
func (m *RuntimeConfigManager) GetMaxLogsPerDevice() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config.Limits.MaxLogsPerDevice
}

// GetMaxLogLength 获取最大日志长度
func (m *RuntimeConfigManager) GetMaxLogLength() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config.Limits.MaxLogLength
}

// GetWorkerCount 获取工作协程数
func (m *RuntimeConfigManager) GetWorkerCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config.Engine.WorkerCount
}

// GetEventBufferSize 获取事件缓冲区大小
func (m *RuntimeConfigManager) GetEventBufferSize() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config.Engine.EventBufferSize
}

// GetFallbackEventCapacity 获取后备事件容量
func (m *RuntimeConfigManager) GetFallbackEventCapacity() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config.Engine.FallbackEventCapacity
}

// GetBufferSize 获取缓冲区大小
func (m *RuntimeConfigManager) GetBufferSize() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config.Buffers.DefaultSize
}

// GetHandshakeTimeout 获取握手超时时间
func (m *RuntimeConfigManager) GetHandshakeTimeout() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return time.Duration(m.config.Timeouts.Handshake) * time.Millisecond
}

// GetShortCommandTimeout 获取短命令超时时间
func (m *RuntimeConfigManager) GetShortCommandTimeout() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return time.Duration(m.config.Timeouts.ShortCmd) * time.Millisecond
}

// GetLongCommandTimeout 获取长命令超时时间
func (m *RuntimeConfigManager) GetLongCommandTimeout() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return time.Duration(m.config.Timeouts.LongCmd) * time.Millisecond
}

// GetPaginationCheckInterval 获取分页检测间隔
func (m *RuntimeConfigManager) GetPaginationCheckInterval() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return time.Duration(m.config.Pagination.CheckInterval) * time.Millisecond
}
