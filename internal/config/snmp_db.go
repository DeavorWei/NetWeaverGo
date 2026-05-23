// Package config 提供应用配置和数据库初始化
// snmp_db.go 管理 SNMP 独立数据库的初始化和生命周期
package config

import (
	"fmt"

	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// SNMPDB SNMP 专用数据库连接（独立于主数据库）
var SNMPDB *gorm.DB

// InitSNMPDB 初始化 SNMP 专用数据库
// 与主库完全独立，使用相同的 SQLite 优化参数
func InitSNMPDB(dbPath string) error {
	dsn := dbPath + "?_journal=WAL&_busy_timeout=5000&_cache_size=10000&_foreign_keys=1&_synchronous=NORMAL"

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
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
func AutoMigrateSNMP() error {
	if SNMPDB == nil {
		return fmt.Errorf("SNMP 数据库未初始化")
	}

	err := SNMPDB.AutoMigrate(
		&models.SNMPServerConfig{},
		&models.SNMPTrapRecord{},
		&models.SNMPTrapFilterRule{},
		&models.SNMPCredential{},
		&models.SNMPPollingTemplate{},
		&models.SNMPPollingTarget{},
		&models.SNMPPollingResult{},
		&models.MIBModule{},
		&models.MIBNode{},
	)
	if err != nil {
		return fmt.Errorf("SNMP 数据表迁移失败: %v", err)
	}

	// 创建额外索引
	createSNMPIndexes(SNMPDB)

	return nil
}

// createSNMPIndexes 创建 SNMP 数据表的额外索引
func createSNMPIndexes(db *gorm.DB) {
	indexes := []string{
		// Trap 记录索引
		"CREATE INDEX IF NOT EXISTS idx_trap_source_ip ON snmp_trap_records(source_ip)",
		"CREATE INDEX IF NOT EXISTS idx_trap_oid ON snmp_trap_records(trap_oid)",
		"CREATE INDEX IF NOT EXISTS idx_trap_severity ON snmp_trap_records(severity)",
		"CREATE INDEX IF NOT EXISTS idx_trap_received_at ON snmp_trap_records(received_at)",
		"CREATE INDEX IF NOT EXISTS idx_trap_acknowledged ON snmp_trap_records(acknowledged)",
		// 轮询结果索引
		"CREATE INDEX IF NOT EXISTS idx_poll_result_target ON snmp_polling_results(target_id)",
		"CREATE INDEX IF NOT EXISTS idx_poll_result_time ON snmp_polling_results(poll_time)",
		"CREATE INDEX IF NOT EXISTS idx_poll_result_ip_time ON snmp_polling_results(target_ip, poll_time)",
		"CREATE INDEX IF NOT EXISTS idx_poll_result_batch ON snmp_polling_results(batch_id)",
		// MIB 节点索引
		"CREATE INDEX IF NOT EXISTS idx_mib_node_oid ON mib_nodes(oid)",
		"CREATE INDEX IF NOT EXISTS idx_mib_node_name ON mib_nodes(name)",
		"CREATE INDEX IF NOT EXISTS idx_mib_node_parent ON mib_nodes(parent_oid)",
		"CREATE INDEX IF NOT EXISTS idx_mib_node_module ON mib_nodes(module_id)",
	}
	for _, sql := range indexes {
		result := db.Exec(sql)
		if result.Error != nil {
			logger.Warn("SNMP-DB", "-", "创建索引失败: %s, 错误: %v", sql, result.Error)
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