package repository

import (
	"github.com/NetWeaverGo/core/internal/models"
	"github.com/NetWeaverGo/core/internal/parser"
	"gorm.io/gorm"
)

// ParseTemplateRepository 解析模板数据访问实现
type ParseTemplateRepository struct {
	db *gorm.DB
}

// 确保 ParseTemplateRepository 实现 parser.UserTemplateSource 接口
var _ parser.UserTemplateSource = (*ParseTemplateRepository)(nil)

// NewParseTemplateRepository 创建解析模板存储层
func NewParseTemplateRepository(db *gorm.DB) *ParseTemplateRepository {
	return &ParseTemplateRepository{db: db}
}

// ListEnabled 获取指定厂商所有已启用的自定义模板（打通断头路 #2）
func (r *ParseTemplateRepository) ListEnabled(vendor string) ([]parser.StoredTemplate, error) {
	var records []models.UserParseTemplate
	err := r.db.Where("vendor = ? AND enabled = ?", vendor, true).Find(&records).Error
	if err != nil {
		return nil, err
	}

	result := make([]parser.StoredTemplate, 0, len(records))
	for _, rec := range records {
		result = append(result, parser.StoredTemplate{
			Vendor:       rec.Vendor,
			CommandKey:   rec.CommandKey,
			Engine:       rec.Engine,
			Pattern:      rec.Pattern,
			Multiline:    rec.Multiline,
			Aggregation:  rec.Aggregation,
			ParseRules:   rec.ParseRules,
			AppliesTo:    rec.AppliesTo,
			FieldMapping: rec.FieldMapping,
			Enabled:      rec.Enabled,
		})
	}
	return result, nil
}
