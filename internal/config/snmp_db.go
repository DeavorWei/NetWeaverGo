// Package config 提供应用配置和数据库初始化
// snmp_db.go 管理 SNMP 独立数据库（仅存储查询凭据）的初始化和生命周期
package config

import (
	"fmt"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
)

// SNMPDB SNMP 专用数据库连接（独立于主数据库，仅存储 snmp_credentials）
var SNMPDB *gorm.DB

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
func AutoMigrateSNMP() error {
	if SNMPDB == nil {
		return fmt.Errorf("SNMP 数据库未初始化")
	}

	if err := SNMPDB.AutoMigrate(&models.SNMPCredential{}); err != nil {
		return fmt.Errorf("SNMP 数据表迁移失败: %v", err)
	}

	logger.Info("SNMP-DB", "-", "SNMP 数据库迁移完成")
	return nil
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
