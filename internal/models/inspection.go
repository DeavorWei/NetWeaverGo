package models

import (
	"time"
)

// InspectionTemplate 巡检模板（定义特定场景或产品族需要执行的检查项集合）
type InspectionTemplate struct {
	ID          string    `gorm:"primaryKey;size:64" json:"id"`
	Name        string    `gorm:"uniqueIndex;size:128;not null" json:"name"`
	Vendor      string    `gorm:"index;size:64;default:'huawei'" json:"vendor"`
	Category    string    `gorm:"index;size:64;default:'general'" json:"category"` // ce / s / ar / fw / route / wlan / general
	Description string    `gorm:"size:256" json:"description"`
	Enabled     bool      `gorm:"default:true" json:"enabled"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// TableName 指定表名
func (InspectionTemplate) TableName() string {
	return "inspection_templates"
}

// InspectionItem 巡检检查项
type InspectionItem struct {
	ID             uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	TemplateID     string    `gorm:"index;size:64;not null" json:"templateId"`
	Code           string    `gorm:"index;size:128;not null" json:"code"` // 如 PRE_CHECK_CPU_USAGE_AR
	Name           string    `gorm:"size:128;not null" json:"name"`
	Category       string    `gorm:"index;size:64;default:'system'" json:"category"` // system / environment / interface / routing / security
	CommandKey     string    `gorm:"size:128;not null" json:"commandKey"`            // 绑定的 CLI 命令或解析模板 Key (如 display cpu-usage)
	IsPreCollect   bool      `gorm:"default:false" json:"isPreCollect"`              // 前置采集项（优先执行并进入任务级命令缓存供后续复用）
	CheckType      string    `gorm:"size:64;default:'threshold'" json:"checkType"`   // threshold / must_contain / must_not_contain / regex / equals / bound
	Field          string    `gorm:"size:64" json:"field"`                           // 判定目标字段（如 cpu_usage / status / temperature）
	ThresholdsJSON string    `gorm:"column:thresholds_json;type:text" json:"thresholdsJson"` // 序列化的 []Threshold JSON
	Description    string    `gorm:"size:256" json:"description"`                    // 检查项说明
	Problem        string    `gorm:"size:256" json:"problem"`                        // 异常时的默认问题描述
	Advice         string    `gorm:"size:512" json:"advice"`                         // 异常时的默认处置建议（来源于 eDesk 专家知识库）
	Severity       string    `gorm:"size:32;default:'major'" json:"severity"`        // blocker / major / minor / info
	Order          int       `gorm:"column:order_num;default:0" json:"order"`                         // 排序权重
	Enabled        bool      `gorm:"default:true" json:"enabled"`                    // 是否启用
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// TableName 指定表名
func (InspectionItem) TableName() string {
	return "inspection_items"
}

// Threshold 外置阈值结构
type Threshold struct {
	Name         string `json:"name"`         // 阈值名称，如 cpu_usage_max
	DataType     string `json:"dataType"`     // float / int / string
	DefaultValue string `json:"defaultValue"` // 默认标准值
	MinValue     string `json:"minValue"`     // 最小值（区间判定）
	MaxValue     string `json:"maxValue"`     // 最大值（区间判定）
	RangeType    string `json:"rangeType"`    // bound (区间内合格) / outside (区间外合格) / equals (等于合格) / lte (<= MaxValue) / gte (>= MinValue)
	Unit         string `json:"unit"`         // 单位，如 % / °C / MB
}

// InspectionResult 巡检结果条目（与交付闭环合流的唯一结论载体）
type InspectionResult struct {
	ID           uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	RunID        string    `gorm:"index:idx_inspect_run_dev;size:64;not null" json:"runId"`
	DeviceIP     string    `gorm:"index:idx_inspect_run_dev;size:64;not null" json:"deviceIp"`
	ItemCode     string    `gorm:"index;size:128;not null" json:"itemCode"`
	ItemName     string    `gorm:"size:128" json:"itemName"`
	Category     string    `gorm:"size:64" json:"category"`
	Status       string    `gorm:"size:32;not null" json:"status"`   // TEST_PASS / TEST_FAIL / TEST_WARNING / TEST_UNACCORD / TEST_IGNORE / TEST_EXCEPT / TEST_MANUAL / TEST_UNTEST
	Severity     string    `gorm:"size:32;not null" json:"severity"` // blocker / major / minor / info
	Problem      string    `gorm:"type:text" json:"problem"`         // 问题详述
	Advice       string    `gorm:"type:text" json:"advice"`          // 处置指导建议
	EvidenceJSON string    `gorm:"column:evidence_json;type:text" json:"evidenceJson"` // 举证证据行序列化 JSON []string
	ActualValue  string    `gorm:"size:128" json:"actualValue"`      // 实际采集提取到的数值
	ThresholdHit string    `gorm:"size:128" json:"thresholdHit"`     // 触发的阈值规则
	CreatedAt    time.Time `json:"createdAt"`
}

// TableName 指定表名
func (InspectionResult) TableName() string {
	return "inspection_results"
}
