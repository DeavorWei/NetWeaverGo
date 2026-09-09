// Package models 包含所有数据库模型定义
// SNMP 凭据存储在独立数据库 snmp.db 中
package models

import "time"

// SNMPCredential SNMP 查询凭据（v1/v2c/v3）
// 所有敏感字段均使用 AES-256-GCM 加密存储
type SNMPCredential struct {
	ID              uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	Name            string    `json:"name" gorm:"uniqueIndex;not null"`
	Version         string    `json:"version"`         // v1/v2c/v3
	Community       string    `json:"community"`       // v1/v2c community string（加密存储）
	SecurityLevel   string    `json:"securityLevel"`   // noAuthNoPriv/authNoPriv/authPriv
	Username        string    `json:"username"`
	AuthProtocol    string    `json:"authProtocol"`    // MD5/SHA/SHA224/SHA256/SHA384/SHA512
	AuthPassword    string    `json:"authPassword"`    // 加密存储
	PrivProtocol    string    `json:"privProtocol"`    // DES/AES/AES192/AES256/AES192C/AES256C
	PrivPassword    string    `json:"privPassword"`    // 加密存储
	ContextName     string    `json:"contextName"`
	ContextEngineID string    `json:"contextEngineId"` // v3 上下文引擎 ID
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

func (SNMPCredential) TableName() string { return "snmp_credentials" }
