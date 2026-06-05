// Package config 提供应用配置和数据库初始化
// snmp_db.go 管理 SNMP 独立数据库的初始化和生命周期
package config

import (
	"fmt"

	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// SNMPDB SNMP 专用数据库连接（独立于主数据库）
var SNMPDB *gorm.DB

// snmpDBVersion SNMP 数据库版本号
// 当表结构发生重大变更时递增此版本号
const snmpDBVersion = 1

// InitSNMPDB 初始化 SNMP 专用数据库
// 与主库完全独立，使用相同的 SQLite 优化参数
func InitSNMPDB(dbPath string) error {
	dsn := dbPath + "?_journal=WAL&_busy_timeout=5000&_cache_size=-64000&_foreign_keys=1&_synchronous=NORMAL&_mmap_size=268435456&_temp_store=2"

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger:                 gormlogger.Default.LogMode(gormlogger.Silent), // 静默GORM日志，避免输出到控制台
		SkipDefaultTransaction: true,
		PrepareStmt:            true,
	})
	if err != nil {
		return fmt.Errorf("SNMP 数据库连接失败: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("获取 SNMP 数据库底层连接失败: %v", err)
	}
	sqlDB.SetMaxOpenConns(10)
	sqlDB.SetMaxIdleConns(5)

	SNMPDB = db
	return nil
}

// AutoMigrateSNMP 自动迁移 SNMP 数据表结构
// 包含完整的迁移逻辑：AutoMigrate + 缺失列修复 + 索引创建
func AutoMigrateSNMP() error {
	if SNMPDB == nil {
		return fmt.Errorf("SNMP 数据库未初始化")
	}

	logger.Info("SNMP-DB", "-", "开始 SNMP 数据库迁移...")

	// 第一步：执行 GORM AutoMigrate
	err := SNMPDB.AutoMigrate(
		&models.SNMPServerConfig{},
		&models.SNMPTrapRecord{},
		&models.SNMPTrapFilterRule{},
		&models.SNMPCredential{},
		&models.SNMPPollingTemplate{},
		&models.SNMPPollingTarget{},
		&models.SNMPPollingResult{},
		&models.MIBFolder{},
		&models.MIBModule{},
		&models.MIBNode{},
	)
	if err != nil {
		return fmt.Errorf("SNMP 数据表迁移失败: %v", err)
	}

	logger.Info("SNMP-DB", "-", "GORM AutoMigrate 完成")

	// 第二步：检查并修复缺失的列（针对旧数据库）
	if err := repairMissingColumns(SNMPDB); err != nil {
		logger.Warn("SNMP-DB", "-", "修复缺失列时出现警告: %v", err)
		// 不返回错误，继续执行
	}

	// 第三步：创建额外索引
	createSNMPIndexes(SNMPDB)

	logger.Info("SNMP-DB", "-", "SNMP 数据库迁移完成")
	return nil
}

// repairMissingColumns 检查并修复缺失的列
// 这是针对 GORM AutoMigrate 在某些情况下无法正确添加列的问题
func repairMissingColumns(db *gorm.DB) error {
	// 定义需要检查的表和列
	// key: 表名, value: 需要检查的列定义列表
	tableColumns := map[string][]columnDef{
		"snmp_trap_records": {
			{name: "source_ip", sqlType: "TEXT"},
			{name: "source_port", sqlType: "INTEGER"},
			{name: "version", sqlType: "TEXT"},
			{name: "community", sqlType: "TEXT"},
			{name: "trap_oid", sqlType: "TEXT"},
			{name: "trap_name", sqlType: "TEXT"},
			{name: "enterprise", sqlType: "TEXT"},
			{name: "generic_trap", sqlType: "INTEGER"},
			{name: "specific_trap", sqlType: "INTEGER"},
			{name: "severity", sqlType: "TEXT"},
			{name: "variables", sqlType: "TEXT"},
			{name: "raw_hex", sqlType: "TEXT"},
			{name: "acknowledged", sqlType: "INTEGER"},
			{name: "acknowledged_at", sqlType: "DATETIME"},
			{name: "received_at", sqlType: "DATETIME"},
			{name: "created_at", sqlType: "DATETIME"},
		},
		"mib_nodes": {
			{name: "module_id", sqlType: "INTEGER"},
			{name: "oid", sqlType: "TEXT"},
			{name: "name", sqlType: "TEXT"},
			{name: "parent_oid", sqlType: "TEXT"},
			{name: "node_type", sqlType: "TEXT"},
			{name: "syntax", sqlType: "TEXT"},
			{name: "access", sqlType: "TEXT"},
			{name: "status", sqlType: "TEXT"},
			{name: "description", sqlType: "TEXT"},
			{name: "source", sqlType: "TEXT"},
			{name: "created_at", sqlType: "DATETIME"},
			{name: "updated_at", sqlType: "DATETIME"},
		},
		"snmp_polling_results": {
			{name: "target_id", sqlType: "INTEGER"},
			{name: "target_ip", sqlType: "TEXT"},
			{name: "batch_id", sqlType: "TEXT"},
			{name: "oid", sqlType: "TEXT"},
			{name: "oid_name", sqlType: "TEXT"},
			{name: "value", sqlType: "TEXT"},
			{name: "value_type", sqlType: "TEXT"},
			{name: "poll_time", sqlType: "DATETIME"},
			{name: "created_at", sqlType: "DATETIME"},
		},
	}

	for tableName, columns := range tableColumns {
		for _, col := range columns {
			if !columnExists(db, tableName, col.name) {
				logger.Warn("SNMP-DB", "-", "检测到缺失列: %s.%s, 正在添加...", tableName, col.name)
				sql := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", tableName, col.name, col.sqlType)
				if err := db.Exec(sql).Error; err != nil {
					logger.Error("SNMP-DB", "-", "添加列失败: %s.%s, 错误: %v", tableName, col.name, err)
					// 继续处理其他列，不中断
				} else {
					logger.Info("SNMP-DB", "-", "成功添加列: %s.%s", tableName, col.name)
				}
			}
		}
	}

	return nil
}

// columnDef 列定义
type columnDef struct {
	name    string
	sqlType string
}

// columnExists 检查表中是否存在指定列
func columnExists(db *gorm.DB, tableName, columnName string) bool {
	var count int64
	err := db.Raw(`
		SELECT COUNT(*) FROM pragma_table_info(?)
		WHERE name = ?
	`, tableName, columnName).Scan(&count).Error
	if err != nil {
		logger.Warn("SNMP-DB", "-", "检查列存在性失败: %s.%s, 错误: %v", tableName, columnName, err)
		return false
	}
	return count > 0
}

// createSNMPIndexes 创建 SNMP 数据表的额外索引
// 在创建索引前会先验证列是否存在
func createSNMPIndexes(db *gorm.DB) {
	// 定义索引及其依赖的列
	// key: 索引SQL, value: 依赖的列信息
	indexDefs := []struct {
		sql       string
		tableName string
		columns   []string
	}{
		// Trap 记录索引
		{
			sql:       "CREATE INDEX IF NOT EXISTS idx_trap_source_ip ON snmp_trap_records(source_ip)",
			tableName: "snmp_trap_records",
			columns:   []string{"source_ip"},
		},
		{
			sql:       "CREATE INDEX IF NOT EXISTS idx_trap_oid ON snmp_trap_records(trap_oid)",
			tableName: "snmp_trap_records",
			columns:   []string{"trap_oid"},
		},
		{
			sql:       "CREATE INDEX IF NOT EXISTS idx_trap_severity ON snmp_trap_records(severity)",
			tableName: "snmp_trap_records",
			columns:   []string{"severity"},
		},
		{
			sql:       "CREATE INDEX IF NOT EXISTS idx_trap_received_at ON snmp_trap_records(received_at)",
			tableName: "snmp_trap_records",
			columns:   []string{"received_at"},
		},
		{
			sql:       "CREATE INDEX IF NOT EXISTS idx_trap_acknowledged ON snmp_trap_records(acknowledged)",
			tableName: "snmp_trap_records",
			columns:   []string{"acknowledged"},
		},
		// 轮询结果索引
		{
			sql:       "CREATE INDEX IF NOT EXISTS idx_poll_result_target ON snmp_polling_results(target_id)",
			tableName: "snmp_polling_results",
			columns:   []string{"target_id"},
		},
		{
			sql:       "CREATE INDEX IF NOT EXISTS idx_poll_result_time ON snmp_polling_results(poll_time)",
			tableName: "snmp_polling_results",
			columns:   []string{"poll_time"},
		},
		{
			sql:       "CREATE INDEX IF NOT EXISTS idx_poll_result_ip_time ON snmp_polling_results(target_ip, poll_time)",
			tableName: "snmp_polling_results",
			columns:   []string{"target_ip", "poll_time"},
		},
		{
			sql:       "CREATE INDEX IF NOT EXISTS idx_poll_result_batch ON snmp_polling_results(batch_id)",
			tableName: "snmp_polling_results",
			columns:   []string{"batch_id"},
		},
		// MIB 节点索引
		{
			sql:       "CREATE INDEX IF NOT EXISTS idx_mib_node_oid ON mib_nodes(oid)",
			tableName: "mib_nodes",
			columns:   []string{"oid"},
		},
		{
			sql:       "CREATE INDEX IF NOT EXISTS idx_mib_node_name ON mib_nodes(name)",
			tableName: "mib_nodes",
			columns:   []string{"name"},
		},
		{
			sql:       "CREATE INDEX IF NOT EXISTS idx_mib_node_parent ON mib_nodes(parent_oid)",
			tableName: "mib_nodes",
			columns:   []string{"parent_oid"},
		},
		{
			sql:       "CREATE INDEX IF NOT EXISTS idx_mib_node_module ON mib_nodes(module_id)",
			tableName: "mib_nodes",
			columns:   []string{"module_id"},
		},
	}

	for _, idx := range indexDefs {
		// 检查所有依赖的列是否存在
		allColumnsExist := true
		for _, col := range idx.columns {
			if !columnExists(db, idx.tableName, col) {
				logger.Warn("SNMP-DB", "-", "跳过索引创建，列不存在: %s.%s", idx.tableName, col)
				allColumnsExist = false
				break
			}
		}

		if allColumnsExist {
			result := db.Exec(idx.sql)
			if result.Error != nil {
				logger.Warn("SNMP-DB", "-", "创建索引失败: %s, 错误: %v", idx.sql, result.Error)
			} else {
				logger.Debug("SNMP-DB", "-", "索引创建成功: %s", idx.sql)
			}
		}
	}
}

// CloseSNMPDB 关闭 SNMP 数据库连接
func CloseSNMPDB() error {
	if SNMPDB != nil {
		sqlDB, err := SNMPDB.DB()
		if err != nil {
			return err
		}
		return sqlDB.Close()
	}
	return nil
}

// InitDefaultSNMPConfig 初始化默认 SNMP 服务器配置（首次启动时调用）
func InitDefaultSNMPConfig() error {
	if SNMPDB == nil {
		return fmt.Errorf("SNMP 数据库未初始化")
	}

	// 检查是否已存在配置
	var count int64
	SNMPDB.Model(&models.SNMPServerConfig{}).Count(&count)
	if count > 0 {
		return nil // 已存在配置，跳过
	}

	// 创建默认配置
	defaultConfig := models.SNMPServerConfig{
		TrapEnabled:              false,
		TrapPort:                 1162,
		TrapCommunity:            "",
		MaxStorageDays:           30,
		PollingEnabled:           false,
		MaxPollingWorkers:        10,
		PollingResultRetentionDays: 7,
	}

	return SNMPDB.Create(&defaultConfig).Error
}