package config

import (
	"fmt"

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
	if DB == nil {
		return nil, false, fmt.Errorf("数据库未初始化")
	}

	var st GlobalSettings
	err := DB.First(&st, 1).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			st = DefaultSettings()
			st.ID = 1
			DB.Create(&st)
			return &st, true, nil
		}
		return nil, false, err
	}

	return &st, false, nil
}

// SaveSettings 保存全局设置到数据库
func SaveSettings(settings GlobalSettings) error {
	if DB == nil {
		return fmt.Errorf("数据库未初始化")
	}
	settings.ID = 1
	return DB.Save(&settings).Error
}
