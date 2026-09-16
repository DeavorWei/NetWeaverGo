package models

import "time"

// ============================================================================
// 告警规则与归并模型（方案 §5.2 B9）
// ============================================================================

// AlarmRule 告警规则定义表
type AlarmRule struct {
	ID          uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	Vendor      string    `json:"vendor" gorm:"size:64;index;not null"`      // 厂商 (huawei/h3c/cisco/etc)
	Family      string    `json:"family" gorm:"size:64;index;not null"`      // 产品族 (CE/Router/S/USG/VNE)
	AlarmName   string    `json:"alarmName" gorm:"size:128;index;not null"`   // 告警标识名称
	Pattern     string    `json:"pattern" gorm:"type:text;not null"`         // 正则匹配模式
	Severity    string    `json:"severity" gorm:"size:32;not null"`          // critical | major | minor | warning | info
	Category    string    `json:"category" gorm:"size:64;not null"`          // power | fan | interface | routing | resource | system
	Description string    `json:"description" gorm:"type:text"`              // 告警详细描述
	Advice      string    `json:"advice" gorm:"type:text"`                   // 排障修复建议
	Threshold   string    `json:"threshold,omitempty" gorm:"size:64"`        // 可选阈值
	Enabled     bool      `json:"enabled" gorm:"default:true"`               // 是否启用
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// TableName 指定表名
func (AlarmRule) TableName() string {
	return "alarm_rules"
}

// AlarmRecord 原始告警实例记录
type AlarmRecord struct {
	ID                 uint       `json:"id" gorm:"primaryKey;autoIncrement"`
	RunID              string     `json:"runId" gorm:"size:64;index;not null"`         // 所属运行任务 ID
	DeviceIP           string     `json:"deviceIp" gorm:"size:64;index;not null"`      // 设备 IP
	Family             string     `json:"family" gorm:"size:64;index"`                 // 产品族
	AlarmName          string     `json:"alarmName" gorm:"size:128;index;not null"`    // 告警名称
	Severity           string     `json:"severity" gorm:"size:32;not null"`           // 严重级别
	Category           string     `json:"category" gorm:"size:64;not null"`           // 分类
	Summary            string     `json:"summary" gorm:"type:text"`                   // 告警摘要
	RawEcho            string     `json:"rawEcho" gorm:"type:text"`                   // 原始回显片段
	OccurredAt         time.Time  `json:"occurredAt"`                                 // 告警发生时间
	Status             string     `json:"status" gorm:"size:32;default:active"`       // active | cleared | suppressed
	MergedPhenomenonID *uint      `json:"mergedPhenomenonId,omitempty" gorm:"index"`  // 关联的归并故障现象 ID
	CreatedAt          time.Time  `json:"createdAt"`
}

// TableName 指定表名
func (AlarmRecord) TableName() string {
	return "alarm_records"
}

// MergedPhenomenon 归并聚合后的故障现象（根因聚合）
type MergedPhenomenon struct {
	ID          uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	RunID       string    `json:"runId" gorm:"size:64;index;not null"`         // 任务运行 ID
	DeviceIP    string    `json:"deviceIp" gorm:"size:64;index;not null"`      // 设备 IP
	Family      string    `json:"family" gorm:"size:64;index"`                 // 产品族
	Title       string    `json:"title" gorm:"size:256;not null"`              // 故障现象标题
	Severity    string    `json:"severity" gorm:"size:32;not null"`           // 归并后综合严重度 (可升级)
	Category    string    `json:"category" gorm:"size:64;not null"`           // 故障类别
	RootCause   string    `json:"rootCause" gorm:"type:text;not null"`        // 推断根因
	Advice      string    `json:"advice" gorm:"type:text"`                    // 整体修复与处置建议
	ImpactScope string    `json:"impactScope" gorm:"type:text"`               // 影响范围描述
	RecordCount int       `json:"recordCount"`                                // 聚合的原始告警数量
	RecordIDs   []uint    `json:"recordIds" gorm:"serializer:json"`           // 关联原始告警 ID 列表
	CreatedAt   time.Time `json:"createdAt"`
}

// TableName 指定表名
func (MergedPhenomenon) TableName() string {
	return "merged_phenomena"
}
