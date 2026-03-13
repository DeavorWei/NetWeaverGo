package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

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
	logger.DebugAll("Config", "-", "开始初始化SQLite存储逻辑，锁定本地工作区: %s", dataDir)
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("无法创建数据目录: %v", err)
	}

	dbPath := filepath.Join(dataDir, "netweaver.db")

	// SQLite 性能优化参数
	// _journal=WAL: 使用 WAL 模式提升并发性能
	// _busy_timeout=5000:  busy 超时 5 秒
	// _cache_size=10000:  缓存 10000 页 (约 40MB)
	// _foreign_keys=1:    启用外键约束
	// _synchronous=NORMAL: 同步模式，平衡性能和安全性
	dsn := dbPath + "?_journal=WAL&_busy_timeout=5000&_cache_size=10000&_foreign_keys=1&_synchronous=NORMAL"

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
		// 禁用默认事务提升性能
		SkipDefaultTransaction: true,
		// 预编译语句缓存
		PrepareStmt: true,
	})
	if err != nil {
		return fmt.Errorf("无法连接数据库: %v", err)
	}

	// 配置连接池
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("获取底层数据库连接失败: %v", err)
	}

	// 连接池配置 - SQLite 是单文件数据库，连接数不宜过多
	sqlDB.SetMaxOpenConns(10)                  // 最大打开连接数
	sqlDB.SetMaxIdleConns(5)                   // 最大空闲连接数
	sqlDB.SetConnMaxLifetime(time.Hour)        // 连接最大生命周期
	sqlDB.SetConnMaxIdleTime(10 * time.Minute) // 空闲连接超时

	DB = db

	logger.DebugAll("Config", "-", "连接SQLite数据库引擎已建立！正在扫描并校验内部表结构约束...")
	// 自动迁移表结构
	err = db.AutoMigrate(
		&DeviceAsset{},
		&GlobalSettings{},
		&CommandGroup{},
		&TaskGroup{},
		&RuntimeSetting{}, // 运行时配置表
	)
	if err != nil {
		return fmt.Errorf("自动迁移表结构失败: %v", err)
	}

	// 创建索引优化查询性能
	createIndexes(db)

	logger.Info("Config", "-", "数据库初始化成功: %s", dbPath)

	// 异步执行数据迁移，不阻塞启动
	go func() {
		// 延迟执行，确保应用已完全启动
		time.Sleep(100 * time.Millisecond)
		logger.DebugAll("Config", "-", "启动异步数据迁移...")
		MigrateLegacyDataIfNeeded()
	}()

	return nil
}

// createIndexes 创建数据库索引优化查询性能
func createIndexes(db *gorm.DB) {
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_devices_status ON device_assets(status)",
		"CREATE INDEX IF NOT EXISTS idx_devices_ip ON device_assets(ip)",
		"CREATE INDEX IF NOT EXISTS idx_cmd_groups_name ON command_groups(name)",
		"CREATE INDEX IF NOT EXISTS idx_task_groups_name ON task_groups(name)",
		"CREATE INDEX IF NOT EXISTS idx_runtime_category ON runtime_settings(category)",
		"CREATE INDEX IF NOT EXISTS idx_runtime_key ON runtime_settings(key)",
	}

	for _, sql := range indexes {
		if err := db.Exec(sql).Error; err != nil {
			logger.Warn("Config", "-", "创建索引失败 [%s]: %v", sql, err)
		}
	}

	logger.DebugAll("Config", "-", "数据库索引创建完成")
}

// MigrateLegacyDataIfNeeded 从旧文件系统迁移数据到数据库
func MigrateLegacyDataIfNeeded() {
	var count int64
	logger.DebugAll("Config", "-", "执行自检模块，侦测是否存在遗留旧文件待转换为数据库对象格式...")

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
			var file struct {
				Groups []CommandGroup `json:"groups"`
			}
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
			var file struct {
				Groups []TaskGroup `json:"groups"`
			}
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

	logger.DebugAll("Config", "-", "本地平滑升级巡检流程执行完毕！")
}
