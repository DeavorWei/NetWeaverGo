package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

var DB *gorm.DB

// GetDB 获取数据库实例
// 供 Repository 层使用，避免直接访问 DB 变量
func GetDB() *gorm.DB {
	return DB
}

// InitDB 初始化 SQLite 数据库
func InitDB() error {
	pm := GetPathManager()
	if err := pm.EnsureDirectories(); err != nil {
		return fmt.Errorf("初始化存储目录失败: %v", err)
	}

	dbPath := pm.GetDBPath()
	logger.Verbose("Config", "-", "开始初始化SQLite存储逻辑，数据根目录: %s", pm.GetStorageRoot())

	// SQLite 性能优化参数
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
	sqlDB.SetMaxOpenConns(10)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(time.Hour)
	sqlDB.SetConnMaxIdleTime(10 * time.Minute)

	DB = db

	logger.Verbose("Config", "-", "连接SQLite数据库引擎已建立！正在扫描并校验内部表结构约束...")
	// 自动迁移表结构
	err = autoMigrateAll(db)
	if err != nil {
		return fmt.Errorf("自动迁移表结构失败: %v", err)
	}

	// 创建索引优化查询性能
	createIndexes(db)

	logger.Info("Config", "-", "数据库初始化成功: %s", dbPath)

	return nil
}

func autoMigrateAll(db *gorm.DB) error {
	return db.AutoMigrate(
		// 基础表
		&models.DeviceAsset{},
		&models.GlobalSettings{},
		&models.CommandGroup{},
		&models.TaskGroup{},
		&models.RuntimeSetting{},
		&models.ExecutionRecord{},
		// 发现任务相关表
		&models.DiscoveryTask{},
		&models.DiscoveryDevice{},
		&models.RawCommandOutput{},
		// 拓扑相关表
		&models.TopologyEdge{},
		&models.TopologyInterface{},
		&models.TopologyLLDPNeighbor{},
		&models.TopologyFDBEntry{},
		&models.TopologyARPEntry{},
		&models.TopologyAggregateGroup{},
		&models.TopologyAggregateMember{},
		// 规划比对相关表
		&models.PlanFile{},
		&models.PlannedLink{},
		&models.DiffReport{},
		&models.DiffItem{},
	)
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
		// 拓扑发现相关索引
		"CREATE INDEX IF NOT EXISTS idx_discovery_tasks_status ON discovery_tasks(status)",
		"CREATE INDEX IF NOT EXISTS idx_discovery_tasks_task_group_id ON discovery_tasks(task_group_id)",
		"CREATE INDEX IF NOT EXISTS idx_discovery_devices_task_status ON discovery_devices(task_id, status)",
		"CREATE INDEX IF NOT EXISTS idx_discovery_devices_mgmt_ip ON discovery_devices(mgmt_ip)",
		"CREATE INDEX IF NOT EXISTS idx_discovery_devices_normalized_name ON discovery_devices(normalized_name)",
		"CREATE INDEX IF NOT EXISTS idx_raw_outputs_task_device_cmd ON raw_command_outputs(task_id, device_ip, command_key)",
		"CREATE INDEX IF NOT EXISTS idx_raw_outputs_parse_status ON raw_command_outputs(parse_status)",
		"CREATE INDEX IF NOT EXISTS idx_topology_edges_task_status ON topology_edges(task_id, status)",
		"CREATE INDEX IF NOT EXISTS idx_topology_neighbors_task_device ON topology_lldp_neighbors(task_id, device_ip)",
		"CREATE INDEX IF NOT EXISTS idx_topology_fdb_task_device_if ON topology_fdb_entries(task_id, device_ip, interface)",
		// 规划比对相关索引
		"CREATE INDEX IF NOT EXISTS idx_plan_files_imported_at ON plan_files(imported_at)",
		"CREATE INDEX IF NOT EXISTS idx_planned_links_plan_edge_key ON planned_links(plan_file_id, edge_key)",
		"CREATE INDEX IF NOT EXISTS idx_diff_reports_task_plan ON diff_reports(task_id, plan_file_id)",
		"CREATE INDEX IF NOT EXISTS idx_diff_items_report_type ON diff_items(report_id, diff_type)",
	}

	for _, sql := range indexes {
		if err := db.Exec(sql).Error; err != nil {
			logger.Warn("Config", "-", "创建索引失败 [%s]: %v", sql, err)
		}
	}

	logger.Verbose("Config", "-", "数据库索引创建完成")
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
