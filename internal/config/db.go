package config

import (
	"encoding/json"
	"fmt"
	"io"
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
	pm := GetPathManager()
	if err := pm.EnsureDirectories(); err != nil {
		return fmt.Errorf("初始化存储目录失败: %v", err)
	}

	dbPath := pm.GetDBPath()
	logger.Verbose("Config", "-", "开始初始化SQLite存储逻辑，数据根目录: %s", pm.GetStorageRoot())

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

	logger.Verbose("Config", "-", "连接SQLite数据库引擎已建立！正在扫描并校验内部表结构约束...")
	// 自动迁移表结构
	err = db.AutoMigrate(
		&DeviceAsset{},
		&GlobalSettings{},
		&CommandGroup{},
		&TaskGroup{},
		&RuntimeSetting{},  // 运行时配置表
		&ExecutionRecord{}, // 历史执行记录表
	)
	if err != nil {
		return fmt.Errorf("自动迁移表结构失败: %v", err)
	}

	// 创建索引优化查询性能
	createIndexes(db)

	logger.Info("Config", "-", "数据库初始化成功: %s", dbPath)

	return nil
}

// createIndexes 创建数据库索引优化查询性能
func createIndexes(db *gorm.DB) {
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_devices_ip ON device_assets(ip)",
		"CREATE INDEX IF NOT EXISTS idx_devices_group_name ON device_assets(group_name)",
		"CREATE INDEX IF NOT EXISTS idx_devices_protocol ON device_assets(protocol)",
		"CREATE INDEX IF NOT EXISTS idx_cmd_groups_name ON command_groups(name)",
		"CREATE INDEX IF NOT EXISTS idx_task_groups_name ON task_groups(name)",
		"CREATE INDEX IF NOT EXISTS idx_runtime_category ON runtime_settings(category)",
		"CREATE INDEX IF NOT EXISTS idx_runtime_key ON runtime_settings(key)",
		// 历史执行记录索引
		"CREATE INDEX IF NOT EXISTS idx_execution_records_task_group_id ON execution_records(task_group_id)",
		"CREATE INDEX IF NOT EXISTS idx_execution_records_runner_source ON execution_records(runner_source)",
		"CREATE INDEX IF NOT EXISTS idx_execution_records_status ON execution_records(status)",
		"CREATE INDEX IF NOT EXISTS idx_execution_records_started_at ON execution_records(started_at)",
		"CREATE INDEX IF NOT EXISTS idx_execution_records_created_at ON execution_records(created_at)",
	}

	for _, sql := range indexes {
		if err := db.Exec(sql).Error; err != nil {
			logger.Warn("Config", "-", "创建索引失败 [%s]: %v", sql, err)
		}
	}

	logger.Verbose("Config", "-", "数据库索引创建完成")
}

// MigrateLegacyDataIfNeeded 从旧文件系统迁移数据到数据库
func MigrateLegacyDataIfNeeded() {
	var count int64
	logger.Verbose("Config", "-", "执行自检模块，侦测是否存在遗留旧文件待转换为数据库对象格式...")
	pm := GetPathManager()
	inventoryPath := pm.GetLegacyInventoryFile()

	// 1. 迁移设备清单
	if _, err := os.Stat(inventoryPath); err == nil {
		DB.Model(&DeviceAsset{}).Count(&count)
		if count == 0 {
			if devs, err := readInventoryLegacy(inventoryPath); err == nil && len(devs) > 0 {
				DB.Create(&devs)
				logger.Info("Config", "-", "成功迁移 %d 条设备记录到数据库", len(devs))
			}
			_ = os.Rename(inventoryPath, inventoryPath+".bak")
		}
	}

	// 2. [已移除] 迁移全局设置（因配置文件已被彻底删除不依赖）

	// 3. 迁移命令组
	cmdPath := pm.GetLegacyCommandGroupsFile()
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
			_ = os.Rename(cmdPath, cmdPath+".bak")
		}
	}

	// 4. 迁移任务组
	tskPath := pm.GetLegacyTaskGroupsFile()
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
			_ = os.Rename(tskPath, tskPath+".bak")
		}
	}

	// 5. 迁移旧的 config.txt 到默认命令组
	if err := MigrateLegacyCommands(); err != nil {
		logger.Warn("Config", "-", "迁移 config.txt 失败: %v", err)
	}

	logger.Verbose("Config", "-", "本地平滑升级巡检流程执行完毕！")
}

// MirrorDatabaseToPath 将当前数据库文件镜像到目标路径，供切换 storageRoot 后下次启动继续使用
func MirrorDatabaseToPath(sourceDBPath, targetDBPath string) error {
	if sourceDBPath == "" || targetDBPath == "" || sourceDBPath == targetDBPath {
		return nil
	}

	if DB != nil {
		// 先触发 checkpoint，尽量减少 WAL 未落盘造成的快照不一致
		_ = DB.Exec("PRAGMA wal_checkpoint(FULL)").Error
	}

	if err := os.MkdirAll(filepath.Dir(targetDBPath), 0755); err != nil {
		return err
	}
	if err := copyFile(sourceDBPath, targetDBPath); err != nil {
		return err
	}

	for _, suffix := range []string{"-wal", "-shm"} {
		src := sourceDBPath + suffix
		if _, err := os.Stat(src); err != nil {
			continue
		}
		if err := copyFile(src, targetDBPath+suffix); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		_ = out.Close()
	}()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}
