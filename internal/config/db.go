package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/NetWeaverGo/core/internal/alarm"
	"github.com/NetWeaverGo/core/internal/ceas"
	"github.com/NetWeaverGo/core/internal/config/migrations"
	"github.com/NetWeaverGo/core/internal/inspection"
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

// SetDB 设置全局数据库实例（主要用于测试与自定义生命周期管理）
func SetDB(db *gorm.DB) {
	DB = db
}

// InitDB 初始化 SQLite 数据库
func InitDB() error {
	pm := GetPathManager()
	if err := pm.EnsureDirectories(); err != nil {
		return fmt.Errorf("初始化存储目录失败: %v", err)
	}

	dbPath := pm.GetDBPath()
	logger.Verbose("Config", "-", "开始初始化SQLite存储逻辑，数据根目录: %s", pm.GetStorageRoot())

	// 判定是否既有库必须在打开数据库之前：SQLite 一旦 Open/查询即会创建主库文件，
	// 打开后再 stat 无法区分"首次安装"与"待升级的既有库"。
	isExistingDB := IsExistingDatabaseFile(dbPath)

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

	// 升级前自动备份：必须在任何表迁移之前执行，且必须在 DB = db 之后
	// （需保证连接已就绪，备份内部显式传入 db 执行 wal_checkpoint）。
	// 版本标记的回写刻意滞后到 main.go 全部迁移成功之后，避免迁移失败时漏备。
	if _, backupErr := EnsurePreUpgradeBackup(db, dbPath, isExistingDB); backupErr != nil {
		logger.Warn("Config", "-", "升级前自动备份未完成（不阻断启动）: %v", backupErr)
	}

	logger.Verbose("Config", "-", "连接SQLite数据库引擎已建立！正在扫描并执行结构迁移...")
	// 1. 先执行显式版本化 SQL 迁移
	if err := migrations.RunMigrations(db); err != nil {
		logger.Warn("Config", "-", "执行版本化迁移脚本告警 (将尝试 AutoMigrate 兜底): %v", err)
	}

	// 2. 自动迁移表结构（同步 GORM 实体与新增字段）
	err = autoMigrateAll(db)
	if err != nil {
		return fmt.Errorf("自动迁移表结构失败: %v", err)
	}

	// 填充风险命令内置种子规则
	if err := models.EnsureRiskCommandSeeds(db); err != nil {
		logger.Warn("Config", "-", "初始化风险命令种子规则失败: %v", err)
	}

	// 填充 BOM 观察清单内置预警种子
	if err := ceas.EnsureBOMWatchlistSeeds(db); err != nil {
		logger.Warn("Config", "-", "初始化 BOM 观察清单种子失败: %v", err)
	}

	// 填充巡检内置模板与检查项种子
	if err := inspection.EnsureInspectionSeeds(db); err != nil {
		logger.Warn("Config", "-", "初始化巡检模板与检查项种子失败: %v", err)
	}

	// 填充巡检多语言文案种子（报表级 i18n）
	if err := inspection.EnsureInspectionItemTextSeeds(db); err != nil {
		logger.Warn("Config", "-", "初始化巡检多语言文案种子失败: %v", err)
	}

	// 加载 DB 设备画像覆盖记录（规划方案 §6.2：内置 JSON 兜底 + DB 覆盖表）
	if err := LoadDeviceProfileOverrides(db); err != nil {
		logger.Warn("Config", "-", "加载设备画像 DB 覆盖记录失败: %v", err)
	}

	// 填充设备能力初始种子
	if err := models.EnsureDeviceCapabilitySeeds(db); err != nil {
		logger.Warn("Config", "-", "初始化设备能力种子失败: %v", err)
	}

	// 填充内置告警规则种子
	if err := alarm.EnsureAlarmSeeds(db); err != nil {
		logger.Warn("Config", "-", "初始化告警规则种子失败: %v", err)
	}

	// 创建索引优化查询性能
	createIndexes(db)

	logger.Info("Config", "-", "数据库初始化成功: %s", dbPath)

	return nil
}

func autoMigrateAll(db *gorm.DB) error {
	return db.AutoMigrate(
		// 版本迁移记录
		&models.SchemaMigration{},
		// 基础表
		&models.DeviceAsset{},
		&models.GlobalSettings{},
		&models.CommandGroup{},
		&models.TaskGroup{},
		&models.RuntimeSetting{},
		&models.TopologyVendorFieldCommand{},
		&models.RiskCommand{},
		&models.RiskCommandLog{},
		&models.RiskTrustEntry{},
		&models.UserParseTemplate{},
		&models.DeviceProfileRecord{},
		&models.DeviceCapability{},
		// 规划比对相关表
		&models.PlanFile{},
		&models.PlannedLink{},
		&models.DiffReport{},
		&models.DiffItem{},
		// 文件服务器配置表
		&models.FileServerConfig{},
		// CEAS 硬件清单与预警相关表
		&models.TaskCEASNode{},
		&models.BOMWatchlistItem{},
		// 巡检应用层相关表
		&models.InspectionTemplate{},
		&models.InspectionItem{},
		&models.InspectionResult{},
		&models.InspectionItemText{},
		// 业务比对相关表
		&models.BizSnapshot{},
		&models.BizCompareTask{},
		&models.BizCompareItem{},
		// 光模块检测与LLD拓扑模板表
		&models.OpticalCheckRule{},
		&models.LLDParseTemplate{},
		// 告警与归并相关表（方案 §5.2 B9）
		&models.AlarmRule{},
		&models.AlarmRecord{},
		&models.MergedPhenomenon{},
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
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_topology_vendor_field_scene ON topology_vendor_field_commands(vendor, field_key, scene)",
		// 调度相关索引
		"CREATE INDEX IF NOT EXISTS idx_task_schedule_logs_group_id ON task_schedule_logs(task_group_id)",
		"CREATE INDEX IF NOT EXISTS idx_task_schedule_logs_triggered_at ON task_schedule_logs(triggered_at)",
		"CREATE INDEX IF NOT EXISTS idx_task_schedule_logs_status ON task_schedule_logs(status)",
		"CREATE INDEX IF NOT EXISTS idx_task_groups_schedule_enabled ON task_groups(schedule_enabled)",
		// 规划比对相关索引
		"CREATE INDEX IF NOT EXISTS idx_plan_files_imported_at ON plan_files(imported_at)",
		"CREATE INDEX IF NOT EXISTS idx_planned_links_plan_edge_key ON planned_links(plan_file_id, edge_key)",
		"CREATE INDEX IF NOT EXISTS idx_diff_reports_task_plan ON diff_reports(task_id, plan_file_id)",
		"CREATE INDEX IF NOT EXISTS idx_diff_items_report_type ON diff_items(report_id, diff_type)",
		// 巡检相关索引
		"CREATE INDEX IF NOT EXISTS idx_inspection_items_tpl ON inspection_items(template_id)",
		"CREATE INDEX IF NOT EXISTS idx_inspection_items_code ON inspection_items(code)",
		"CREATE INDEX IF NOT EXISTS idx_inspection_results_run ON inspection_results(run_id)",
		"CREATE INDEX IF NOT EXISTS idx_inspection_results_device ON inspection_results(device_ip)",
		"CREATE INDEX IF NOT EXISTS idx_inspection_results_status ON inspection_results(status)",
		// 告警相关索引
		"CREATE INDEX IF NOT EXISTS idx_alarm_records_run ON alarm_records(run_id)",
		"CREATE INDEX IF NOT EXISTS idx_alarm_records_device ON alarm_records(device_ip)",
		"CREATE INDEX IF NOT EXISTS idx_merged_phenomena_run ON merged_phenomena(run_id)",
	}

	for _, sql := range indexes {
		if err := db.Exec(sql).Error; err != nil {
			logger.Warn("Config", "-", "创建索引失败 [%s]: %v", sql, err)
		}
	}

	logger.Verbose("Config", "-", "数据库索引创建完成")
}

// MirrorDatabaseToPath 将数据库文件镜像到目标路径，供切换 storageRoot 后下次启动继续使用。
// 显式接收 *gorm.DB：解除对包级全局 DB 的隐式时序依赖（A2），
// 保证 wal_checkpoint 一定作用在被镜像的那个连接上。
func MirrorDatabaseToPath(db *gorm.DB, sourceDBPath, targetDBPath string) error {
	if sourceDBPath == "" || targetDBPath == "" || sourceDBPath == targetDBPath {
		return nil
	}

	if db != nil {
		// 先触发 checkpoint，尽量减少 WAL 未落盘造成的快照不一致
		_ = db.Exec("PRAGMA wal_checkpoint(FULL)").Error
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
