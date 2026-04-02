package models

import (
	"time"
)

// UserParseTemplate 用户自定义解析模板
type UserParseTemplate struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Vendor       string    `gorm:"column:vendor;not null;index;uniqueIndex:uk_vendor_command" json:"vendor"`
	CommandKey   string    `gorm:"column:command_key;not null;index;uniqueIndex:uk_vendor_command" json:"commandKey"`
	Engine       string    `gorm:"column:engine;not null" json:"engine"`
	Pattern      string    `gorm:"column:pattern;type:text" json:"pattern"`
	Multiline    bool      `gorm:"column:multiline;default:true" json:"multiline"`
	Aggregation  string    `gorm:"column:aggregation;type:text" json:"aggregation"`
	FieldMapping string    `gorm:"column:field_mapping;type:text" json:"fieldMapping"`
	Description  string    `gorm:"column:description" json:"description"`
	Enabled      bool      `gorm:"column:enabled;default:true" json:"enabled"`
	Revision     uint      `gorm:"column:revision;default:1" json:"revision"`
	CreatedAt    time.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt    time.Time `gorm:"column:updated_at" json:"updatedAt"`
}

// TableName 返回表名
func (UserParseTemplate) TableName() string {
	return "net_user_parse_templates"
}

// SaveParseTemplateRequest 保存模板请求
type SaveParseTemplateRequest struct {
	Vendor       string                 `json:"vendor"`
	CommandKey   string                 `json:"commandKey"`
	Engine       string                 `json:"engine"`
	Pattern      string                 `json:"pattern"`
	Multiline    bool                   `json:"multiline"`
	Aggregation  map[string]interface{} `json:"aggregation"`
	FieldMapping map[string]string      `json:"fieldMapping"`
	Description  string                 `json:"description"`
	Enabled      bool                   `json:"enabled"`
}

// TestParseTemplateRequest 测试模板请求
type TestParseTemplateRequest struct {
	Vendor       string                 `json:"vendor"`
	CommandKey   string                 `json:"commandKey"`
	Engine       string                 `json:"engine"`
	Pattern      string                 `json:"pattern"`
	Multiline    bool                   `json:"multiline"`
	Aggregation  map[string]interface{} `json:"aggregation"`
	FieldMapping map[string]string      `json:"fieldMapping"`
	RawText      string                 `json:"rawText"`
}

// TestParseTemplateResult 测试模板结果
type TestParseTemplateResult struct {
	Success bool                `json:"success"`
	Results []map[string]string `json:"results"`
	Count   int                 `json:"count"`
	Error   string              `json:"error,omitempty"`
}

// UserParseTemplateVO 用户模板视图对象
type UserParseTemplateVO struct {
	ID           uint                   `json:"id"`
	Vendor       string                 `json:"vendor"`
	CommandKey   string                 `json:"commandKey"`
	Engine       string                 `json:"engine"`
	Pattern      string                 `json:"pattern"`
	Multiline    bool                   `json:"multiline"`
	Aggregation  map[string]interface{} `json:"aggregation"`
	FieldMapping map[string]string      `json:"fieldMapping"`
	Description  string                 `json:"description"`
	Enabled      bool                   `json:"enabled"`
	Revision     uint                   `json:"revision"`
	CreatedAt    time.Time              `json:"createdAt"`
	UpdatedAt    time.Time              `json:"updatedAt"`
}
