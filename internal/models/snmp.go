// Package models 包含所有数据库模型定义
// SNMP 相关模型存储在独立数据库 snmp.db 中
package models

import "time"

// ============================================================================
// SNMP 服务器配置
// ============================================================================

// SNMPServerConfig SNMP 服务器全局配置（单行记录）
type SNMPServerConfig struct {
	ID   uint      `json:"id" gorm:"primaryKey"`
	TrapEnabled      bool   `json:"trapEnabled"`          // Trap 监听开关
	TrapPort         int    `json:"trapPort"`             // Trap 监听端口（默认 1162）
	TrapCommunity    string `json:"trapCommunity"`        // v1/v2c Community 过滤（空=接受所有）
	V3Enabled        bool   `json:"v3Enabled"`            // SNMPv3 开关（阶段4实现）
	V3Username       string `json:"v3Username"`
	V3AuthProtocol   string `json:"v3AuthProtocol"`       // MD5/SHA
	V3AuthPassword   string `json:"v3AuthPassword"`       // 加密存储
	V3PrivProtocol   string `json:"v3PrivProtocol"`       // DES/AES
	V3PrivPassword   string `json:"v3PrivPassword"`       // 加密存储
	V3EngineID       string `json:"v3EngineID"`
	MaxStorageDays   int    `json:"maxStorageDays"`       // 告警最大保留天数（0=永久）
	PollingEnabled   bool   `json:"pollingEnabled"`       // 轮询调度总开关
	MaxPollingWorkers int   `json:"maxPollingWorkers"`    // 最大并发轮询数（默认 10）
	PollingResultRetentionDays int `json:"pollingResultRetentionDays"` // 轮询数据保留天数（默认 7）
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

func (SNMPServerConfig) TableName() string { return "snmp_server_config" }

// ============================================================================
// Trap 告警记录（独立存储，不与现有告警系统整合）
// ============================================================================

// SNMPTrapRecord SNMP Trap 告警记录
type SNMPTrapRecord struct {
	ID             uint       `json:"id" gorm:"primaryKey;autoIncrement"`
	SourceIP       string     `json:"sourceIP" gorm:"index"`
	SourcePort     int        `json:"sourcePort"`
	Version        string     `json:"version"`               // v1/v2c（阶段2）；v3（阶段4）
	Community      string     `json:"community"`
	TrapOID        string     `json:"trapOID" gorm:"index"`
	TrapName       string     `json:"trapName"`              // MIB 解析后名称
	Enterprise     string     `json:"enterprise"`            // Enterprise OID（v1）
	GenericTrap    int        `json:"genericTrap"`           // Generic Trap 类型（v1）
	SpecificTrap   int        `json:"specificTrap"`          // Specific Trap 类型（v1）
	Severity       string     `json:"severity" gorm:"index"` // critical/major/minor/info/unknown
	Variables      string     `json:"variables" gorm:"type:text"` // VarBinds JSON 序列化
	RawHex         string     `json:"rawHex" gorm:"type:text"`
	Acknowledged   bool       `json:"acknowledged"`
	AcknowledgedAt *time.Time `json:"acknowledgedAt"`
	ReceivedAt     time.Time  `json:"receivedAt" gorm:"index"`
	CreatedAt      time.Time  `json:"createdAt"`
}

func (SNMPTrapRecord) TableName() string { return "snmp_trap_records" }

// SNMPTrapFilterRule Trap 过滤/分类规则
type SNMPTrapFilterRule struct {
	ID               uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	Name             string    `json:"name" gorm:"uniqueIndex;not null"`
	Enabled          bool      `json:"enabled"`
	Priority         int       `json:"priority"`             // 值越小越先匹配
	Action           string    `json:"action"`               // accept/drop/severity_override
	SourceIPPattern  string    `json:"sourceIPPattern"`      // 来源 IP（支持 CIDR）
	OIDPattern       string    `json:"oidPattern"`           // OID 前缀匹配
	CommunityPattern string    `json:"communityPattern"`
	OverrideSeverity string    `json:"overrideSeverity"`     // 覆盖严重级别
	Description      string    `json:"description"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

func (SNMPTrapFilterRule) TableName() string { return "snmp_trap_filter_rules" }

// ============================================================================
// SNMP 凭据管理
// ============================================================================

// SNMPCredential SNMP 凭据（v1/v2c 阶段2实现，v3 阶段4实现）
// 所有敏感字段均使用 AES-256 加密存储
type SNMPCredential struct {
	ID              uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	Name            string    `json:"name" gorm:"uniqueIndex;not null"`
	Version         string    `json:"version"`               // v1/v2c/v3
	Community       string    `json:"community"`             // v1/v2c community string（AES-256 加密存储）
	SecurityLevel   string    `json:"securityLevel"`         // noAuthNoPriv/authNoPriv/authPriv
	Username        string    `json:"username"`
	AuthProtocol    string    `json:"authProtocol"`          // MD5/SHA/SHA224/SHA256/SHA384/SHA512
	AuthPassword    string    `json:"authPassword"`          // AES-256 加密存储
	PrivProtocol    string    `json:"privProtocol"`          // DES/AES/AES192/AES256/AES192C/AES256C
	PrivPassword    string    `json:"privPassword"`          // AES-256 加密存储
	ContextName     string    `json:"contextName"`
	ContextEngineID string    `json:"contextEngineId"`      // v3 上下文引擎 ID
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

func (SNMPCredential) TableName() string { return "snmp_credentials" }

// ============================================================================
// SNMP 轮询
// ============================================================================

// SNMPPollingTemplate SNMP 采集模板
type SNMPPollingTemplate struct {
	ID          uint          `json:"id" gorm:"primaryKey;autoIncrement"`
	Name        string        `json:"name" gorm:"uniqueIndex;not null"`
	Description string        `json:"description"`
	Category    string        `json:"category"`              // system/interface/cpu/memory/storage/custom
	OIDItems    []SNMPOIDItem `json:"oidItems" gorm:"serializer:json"`
	CreatedAt   time.Time     `json:"createdAt"`
	UpdatedAt   time.Time     `json:"updatedAt"`
}

func (SNMPPollingTemplate) TableName() string { return "snmp_polling_templates" }

// SNMPOIDItem 采集模板中的单个 OID 项
type SNMPOIDItem struct {
	OID         string `json:"oid"`          // 如 1.3.6.1.2.1.1.1.0
	Name        string `json:"name"`         // 如 sysDescr
	Type        string `json:"type"`         // string/integer/gauge/counter/timeticks
	Operation   string `json:"operation"`    // get/walk/bulk
	Description string `json:"description"`
}

// SNMPPollingTarget SNMP 轮询目标
// 通过 TargetIP 逻辑关联主库设备资产，不使用外键
type SNMPPollingTarget struct {
	ID             uint       `json:"id" gorm:"primaryKey;autoIncrement"`
	TargetIP       string     `json:"targetIP" gorm:"index;not null"`
	TargetPort     int        `json:"targetPort"`                  // 默认 161
	DisplayName    string     `json:"displayName"`
	CredentialID   *uint      `json:"credentialId" gorm:"index;constraint:OnDelete:SET NULL"`
	TemplateID     *uint      `json:"templateId" gorm:"index;constraint:OnDelete:SET NULL"`
	PollInterval   int        `json:"pollInterval"`                // 轮询间隔（秒）
	Enabled        bool       `json:"enabled"`
	LastPollAt     *time.Time `json:"lastPollAt"`
	LastPollStatus string     `json:"lastPollStatus"`              // success/error/timeout
	LastPollError  string     `json:"lastPollError"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

func (SNMPPollingTarget) TableName() string { return "snmp_polling_targets" }

// SNMPPollingResult SNMP 轮询结果
type SNMPPollingResult struct {
	ID        uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	TargetID  uint      `json:"targetId" gorm:"index;index:idx_target_polltime,priority:1"` // P2-19: 复合索引
	TargetIP  string    `json:"targetIP" gorm:"index"`
	BatchID   string    `json:"batchId" gorm:"index"`       // 同次轮询的所有结果共享同一 BatchID
	OID       string    `json:"oid"`
	OIDName   string    `json:"oidName"`
	Value     string    `json:"value"`
	ValueType string    `json:"valueType"`
	PollTime  time.Time `json:"pollTime" gorm:"index;index:idx_target_polltime,priority:2"` // P2-19: 复合索引
	CreatedAt time.Time `json:"createdAt"`
}

func (SNMPPollingResult) TableName() string { return "snmp_polling_results" }

// ============================================================================
// MIB 管理（无内置 MIB，全部由用户创建或导入）
// ============================================================================

// MIBModule 已导入的 MIB 模块
type MIBModule struct {
	ID          uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	Name        string    `json:"name" gorm:"uniqueIndex;not null"` // 模块名（如 IF-MIB）
	FileName    string    `json:"fileName"`                         // 原始文件名
	Description string    `json:"description"`
	Source      string    `json:"source"`                           // import/manual
	NodeCount   int       `json:"nodeCount"`
	FilePath    string    `json:"filePath"`                         // MIB 文件存储路径
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func (MIBModule) TableName() string { return "mib_modules" }

// MIBNode MIB 节点
type MIBNode struct {
	ID          uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	ModuleID    *uint     `json:"moduleId" gorm:"index"`
	OID         string    `json:"oid" gorm:"uniqueIndex;not null"`
	Name        string    `json:"name" gorm:"index"`
	ParentOID   string    `json:"parentOid" gorm:"index"`
	NodeType    string    `json:"nodeType"`                        // scalar/table/row/column/notification
	Syntax      string    `json:"syntax"`                          // INTEGER/OCTET STRING 等
	Access      string    `json:"access"`                          // read-only/read-write/not-accessible
	Status      string    `json:"status"`                          // current/deprecated/obsolete
	Description string    `json:"description" gorm:"type:text"`
	Source      string    `json:"source"`                          // import/manual
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func (MIBNode) TableName() string { return "mib_nodes" }
