package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

var DB *gorm.DB

// InitDB 初始化 SQLite 数据库
func InitDB() error {
	cwd, _ := os.Getwd()
	dataDir := filepath.Join(cwd, "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("无法创建数据目录: %v", err)
	}

	dbPath := filepath.Join(dataDir, "netweaver.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		return fmt.Errorf("无法连接数据库: %v", err)
	}

	DB = db

	// 自动迁移表结构
	err = db.AutoMigrate(
		&DeviceAsset{},
		&GlobalSettings{},
		&CommandGroup{},
		&TaskGroup{},
	)
	if err != nil {
		return fmt.Errorf("自动迁移表结构失败: %v", err)
	}

	logger.Info("Config", "-", "数据库初始化成功: %s", dbPath)
	return nil
}

// MigrateLegacyDataIfNeeded 从旧文件系统迁移数据到数据库
func MigrateLegacyDataIfNeeded() {
	var count int64

	// 1. 迁移设备清单
	if _, err := os.Stat(inventoryFile); err == nil {
		DB.Model(&DeviceAsset{}).Count(&count)
		if count == 0 {
			if devs, err := readInventoryLegacy(); err == nil && len(devs) > 0 {
				DB.Create(&devs)
				logger.Info("Config", "-", "成功迁移 %d 条设备记录到数据库", len(devs))
			}
			os.Rename(inventoryFile, inventoryFile+".bak")
		}
	}

	// 2. [已移除] 迁移全局设置（因配置文件已被彻底删除不依赖）

	// 3. 迁移命令组
	cwd, _ := os.Getwd()
	cmdPath := filepath.Join(cwd, "commands", "groups.json")
	if data, err := os.ReadFile(cmdPath); err == nil {
		DB.Model(&CommandGroup{}).Count(&count)
		if count == 0 {
			var file struct { Groups []CommandGroup `json:"groups"` }
			if err := json.Unmarshal(data, &file); err == nil && len(file.Groups) > 0 {
				DB.Create(&file.Groups)
				logger.Info("Config", "-", "成功迁移 %d 个命令组到数据库", len(file.Groups))
			}
			os.Rename(cmdPath, cmdPath+".bak")
		}
	}

	// 4. 迁移任务组
	tskPath := filepath.Join(cwd, "commands", "tasks.json")
	if data, err := os.ReadFile(tskPath); err == nil {
		DB.Model(&TaskGroup{}).Count(&count)
		if count == 0 {
			var file struct { Groups []TaskGroup `json:"groups"` }
			if err := json.Unmarshal(data, &file); err == nil && len(file.Groups) > 0 {
				DB.Create(&file.Groups)
				logger.Info("Config", "-", "成功迁移 %d 个任务组到数据库", len(file.Groups))
			}
			os.Rename(tskPath, tskPath+".bak")
		}
	}

	// 5. 迁移旧的 config.txt 到默认命令组
	if err := MigrateLegacyCommands(); err != nil {
		logger.Warn("Config", "-", "迁移 config.txt 失败: %v", err)
	}
}
