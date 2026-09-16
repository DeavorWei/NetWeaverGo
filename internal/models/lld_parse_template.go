package models

import "time"

// LLDParseTemplate LLD 拓扑发现解析模板实体
type LLDParseTemplate struct {
	ID          uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Vendor      string    `gorm:"size:64;index" json:"vendor"`
	Protocol    string    `gorm:"size:32;index" json:"protocol"` // LLDP | CDP
	Scene       string    `gorm:"size:64;default:'default';index" json:"scene"`
	TemplateID  string    `gorm:"size:128;uniqueIndex" json:"templateId"`
	ContentJSON string    `gorm:"type:text" json:"contentJson"` // 模板定义 JSON
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// TableName 指定表名
func (LLDParseTemplate) TableName() string {
	return "lld_parse_templates"
}
