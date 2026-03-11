package config

import (
	"fmt"

	"github.com/NetWeaverGo/core/internal/logger"
	"gorm.io/gorm"
)

// GlobalSettings 全局运行参数
type GlobalSettings struct {
	ID             uint   `json:"id" gorm:"primaryKey"`
	MaxWorkers     int    `json:"maxWorkers"`     // 并发数 (当前硬编码为 32)
	ConnectTimeout string `json:"connectTimeout"` // SSH/SFTP 连接超时 (如 "10s")
	CommandTimeout string `json:"commandTimeout"` // 单条命令默认超时 (如 "30s")
	OutputDir      string `json:"outputDir"`      // 回显输出与配置备份的根目录
	LogDir         string `json:"logDir"`         // 系统运行日志存放目录
	ErrorMode      string `json:"errorMode"`      // "pause" | "skip" | "abort"
}

// DefaultSettings 返回默认配置
func DefaultSettings() GlobalSettings {
	return GlobalSettings{
		MaxWorkers:     32,
		ConnectTimeout: "10s",
		CommandTimeout: "30s",
		OutputDir:      "output",
		LogDir:         "logs",
		ErrorMode:      "pause",
	}
}

// LoadSettings 从数据库读取设置，如果不存在则自动创建默认模板
func LoadSettings() (*GlobalSettings, bool, error) {
	logger.DebugAll("Config", "-", "开始从数据库加载系统全局运行参数..")
	if DB == nil {
		return nil, false, fmt.Errorf("数据库未初始化")
	}

	var st GlobalSettings
	err := DB.First(&st, 1).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			logger.Debug("Config", "-", "数据库内未发现预存全局配置，正自动初始分配默认模板设置")
			st = DefaultSettings()
			st.ID = 1
			DB.Create(&st)
			return &st, true, nil
		}
		logger.Error("Config", "-", "从数据库加载全局设置失败: %v", err)
		return nil, false, err
	}

	logger.DebugAll("Config", "-", "成功将现有全局设置从数据库载入内存")
	return &st, false, nil
}

// SaveSettings 保存全局设置到数据库
func SaveSettings(settings GlobalSettings) error {
	logger.Debug("Config", "-", "准备将更新后的全局参数覆盖保存至本地数据库...")
	if DB == nil {
		return fmt.Errorf("数据库未初始化")
	}
	settings.ID = 1
	err := DB.Save(&settings).Error
	if err != nil {
		logger.Error("Config", "-", "全局配置保存产生错误: %v", err)
		return err
	}
	logger.DebugAll("Config", "-", "全局参数保存落库完毕")
	return nil
}
