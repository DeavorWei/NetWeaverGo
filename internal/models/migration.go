package models

import (
	"time"
)

// SchemaMigration 数据库结构版本化迁移记录
type SchemaMigration struct {
	Version     string    `gorm:"primaryKey;size:64;comment:迁移版本号(如0001)" json:"version"`
	Description string    `gorm:"size:255;comment:迁移描述" json:"description"`
	AppliedAt   time.Time `gorm:"not null;comment:执行时间" json:"appliedAt"`
	Checksum    string    `gorm:"size:64;comment:脚本内容哈希" json:"checksum"`
	Success     bool      `gorm:"not null;default:true;comment:是否执行成功" json:"success"`
}

// TableName 指定表名
func (SchemaMigration) TableName() string {
	return "schema_migrations"
}
