package ui

import (
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/NetWeaverGo/core/internal/models"
	"github.com/NetWeaverGo/core/internal/parser"
	"gorm.io/gorm"
)

// ParseTemplateService 解析模板管理服务
type ParseTemplateService struct {
	db       *gorm.DB
	reloader parser.ParserReloader
}

// NewParseTemplateService 创建解析模板服务
func NewParseTemplateService(db *gorm.DB, reloader parser.ParserReloader) *ParseTemplateService {
	return &ParseTemplateService{
		db:       db,
		reloader: reloader,
	}
}

// ListTemplates 列出模板
func (s *ParseTemplateService) ListTemplates(vendor string) ([]models.UserParseTemplateVO, error) {
	var templates []models.UserParseTemplate
	query := s.db.Model(&models.UserParseTemplate{})
	if vendor != "" {
		query = query.Where("vendor = ?", vendor)
	}
	if err := query.Order("vendor, command_key").Find(&templates).Error; err != nil {
		return nil, fmt.Errorf("查询模板失败: %w", err)
	}

	vos := make([]models.UserParseTemplateVO, 0, len(templates))
	for _, t := range templates {
		vo, err := s.toVO(t)
		if err != nil {
			return nil, err
		}
		vos = append(vos, vo)
	}
	return vos, nil
}

// GetTemplate 获取单个模板
func (s *ParseTemplateService) GetTemplate(id uint) (*models.UserParseTemplateVO, error) {
	var t models.UserParseTemplate
	if err := s.db.First(&t, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("模板不存在")
		}
		return nil, fmt.Errorf("查询模板失败: %w", err)
	}
	vo, err := s.toVO(t)
	if err != nil {
		return nil, err
	}
	return &vo, nil
}

// CreateTemplate 创建模板
func (s *ParseTemplateService) CreateTemplate(req models.SaveParseTemplateRequest) error {
	// 校验引擎类型
	if req.Engine != "regex" && req.Engine != "aggregate" {
		return fmt.Errorf("不支持的引擎类型: %s", req.Engine)
	}

	// 校验正则模式
	if req.Engine == "regex" && req.Pattern != "" {
		if _, err := regexp.Compile(req.Pattern); err != nil {
			return fmt.Errorf("正则模式编译失败: %w", err)
		}
	}

	// 检查唯一键冲突
	var count int64
	s.db.Model(&models.UserParseTemplate{}).
		Where("vendor = ? AND command_key = ?", req.Vendor, req.CommandKey).
		Count(&count)
	if count > 0 {
		return fmt.Errorf("模板已存在: vendor=%s commandKey=%s", req.Vendor, req.CommandKey)
	}

	// 序列化 JSON 字段
	aggregationJSON, err := json.Marshal(req.Aggregation)
	if err != nil {
		return fmt.Errorf("序列化聚合配置失败: %w", err)
	}
	fieldMappingJSON, err := json.Marshal(req.FieldMapping)
	if err != nil {
		return fmt.Errorf("序列化字段映射失败: %w", err)
	}

	t := models.UserParseTemplate{
		Vendor:       req.Vendor,
		CommandKey:   req.CommandKey,
		Engine:       req.Engine,
		Pattern:      req.Pattern,
		Multiline:    req.Multiline,
		Aggregation:  string(aggregationJSON),
		FieldMapping: string(fieldMappingJSON),
		Description:  req.Description,
		Enabled:      req.Enabled,
		Revision:     1,
	}

	if err := s.db.Create(&t).Error; err != nil {
		return fmt.Errorf("创建模板失败: %w", err)
	}

	// 刷新解析器快照
	if err := s.reloader.ReloadVendor(req.Vendor); err != nil {
		return fmt.Errorf("刷新解析器失败: %w", err)
	}

	return nil
}

// UpdateTemplate 更新模板
func (s *ParseTemplateService) UpdateTemplate(id uint, req models.SaveParseTemplateRequest) error {
	var t models.UserParseTemplate
	if err := s.db.First(&t, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("模板不存在")
		}
		return fmt.Errorf("查询模板失败: %w", err)
	}

	// 校验引擎类型
	if req.Engine != "regex" && req.Engine != "aggregate" {
		return fmt.Errorf("不支持的引擎类型: %s", req.Engine)
	}

	// 校验正则模式
	if req.Engine == "regex" && req.Pattern != "" {
		if _, err := regexp.Compile(req.Pattern); err != nil {
			return fmt.Errorf("正则模式编译失败: %w", err)
		}
	}

	// 序列化 JSON 字段
	aggregationJSON, err := json.Marshal(req.Aggregation)
	if err != nil {
		return fmt.Errorf("序列化聚合配置失败: %w", err)
	}
	fieldMappingJSON, err := json.Marshal(req.FieldMapping)
	if err != nil {
		return fmt.Errorf("序列化字段映射失败: %w", err)
	}

	// 更新字段
	t.Engine = req.Engine
	t.Pattern = req.Pattern
	t.Multiline = req.Multiline
	t.Aggregation = string(aggregationJSON)
	t.FieldMapping = string(fieldMappingJSON)
	t.Description = req.Description
	t.Enabled = req.Enabled
	t.Revision++

	if err := s.db.Save(&t).Error; err != nil {
		return fmt.Errorf("更新模板失败: %w", err)
	}

	// 刷新解析器快照
	if err := s.reloader.ReloadVendor(t.Vendor); err != nil {
		return fmt.Errorf("刷新解析器失败: %w", err)
	}

	return nil
}

// DeleteTemplate 删除模板
func (s *ParseTemplateService) DeleteTemplate(id uint) error {
	var t models.UserParseTemplate
	if err := s.db.First(&t, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("模板不存在")
		}
		return fmt.Errorf("查询模板失败: %w", err)
	}

	vendor := t.Vendor

	if err := s.db.Delete(&t).Error; err != nil {
		return fmt.Errorf("删除模板失败: %w", err)
	}

	// 刷新解析器快照（恢复内置模板）
	if err := s.reloader.ReloadVendor(vendor); err != nil {
		return fmt.Errorf("刷新解析器失败: %w", err)
	}

	return nil
}

// TestTemplate 测试模板
func (s *ParseTemplateService) TestTemplate(req models.TestParseTemplateRequest) *models.TestParseTemplateResult {
	result := &models.TestParseTemplateResult{}

	// 构建临时模板
	tpl := &parser.RegexTemplate{
		Vendor:     req.Vendor,
		CommandKey: req.CommandKey,
		Engine:     parser.TemplateEngine(req.Engine),
		Pattern:    req.Pattern,
		Multiline:  req.Multiline,
	}

	// 解析聚合配置
	if req.Engine == "aggregate" && req.Aggregation != nil {
		aggConfig, err := s.parseAggregationConfig(req.Aggregation)
		if err != nil {
			result.Error = fmt.Sprintf("解析聚合配置失败: %v", err)
			return result
		}
		tpl.Aggregation = aggConfig
	}

	// 编译模板
	compiled, err := s.compileTemplate(tpl)
	if err != nil {
		result.Error = fmt.Sprintf("编译模板失败: %v", err)
		return result
	}

	// 执行解析
	var rows []map[string]string
	switch tpl.Engine {
	case parser.EngineRegex:
		engine := parser.NewRegexParser()
		rows, err = engine.ParseWithTemplate(compiled, req.RawText)
	case parser.EngineAggregate:
		engine := parser.NewAggregateEngine()
		rows, err = engine.ParseWithTemplate(compiled, req.RawText)
	default:
		result.Error = fmt.Sprintf("不支持的引擎类型: %s", tpl.Engine)
		return result
	}

	if err != nil {
		result.Error = fmt.Sprintf("解析失败: %v", err)
		return result
	}

	result.Success = true
	result.Results = rows
	result.Count = len(rows)
	return result
}

// toVO 转换为视图对象
func (s *ParseTemplateService) toVO(t models.UserParseTemplate) (models.UserParseTemplateVO, error) {
	vo := models.UserParseTemplateVO{
		ID:          t.ID,
		Vendor:      t.Vendor,
		CommandKey:  t.CommandKey,
		Engine:      t.Engine,
		Pattern:     t.Pattern,
		Multiline:   t.Multiline,
		Description: t.Description,
		Enabled:     t.Enabled,
		Revision:    t.Revision,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
	}

	// 解析 JSON 字段
	if t.Aggregation != "" {
		if err := json.Unmarshal([]byte(t.Aggregation), &vo.Aggregation); err != nil {
			return vo, fmt.Errorf("解析聚合配置失败: %w", err)
		}
	}
	if t.FieldMapping != "" {
		if err := json.Unmarshal([]byte(t.FieldMapping), &vo.FieldMapping); err != nil {
			return vo, fmt.Errorf("解析字段映射失败: %w", err)
		}
	}

	return vo, nil
}

// parseAggregationConfig 解析聚合配置
func (s *ParseTemplateService) parseAggregationConfig(data map[string]interface{}) (*parser.AggregationConfig, error) {
	config := &parser.AggregationConfig{}

	if recordStart, ok := data["recordStart"].([]interface{}); ok {
		for _, rs := range recordStart {
			if str, ok := rs.(string); ok {
				config.RecordStart = append(config.RecordStart, str)
			}
		}
	}

	if captureRules, ok := data["captureRules"].([]interface{}); ok {
		for _, cr := range captureRules {
			if ruleMap, ok := cr.(map[string]interface{}); ok {
				rule := parser.CaptureRule{}
				if pattern, ok := ruleMap["pattern"].(string); ok {
					rule.Pattern = pattern
				}
				if mode, ok := ruleMap["mode"].(string); ok {
					rule.Mode = mode
				}
				config.CaptureRules = append(config.CaptureRules, rule)
			}
		}
	}

	if filldown, ok := data["filldown"].([]interface{}); ok {
		for _, fd := range filldown {
			if str, ok := fd.(string); ok {
				config.Filldown = append(config.Filldown, str)
			}
		}
	}

	if emitWhen, ok := data["emitWhen"].([]interface{}); ok {
		for _, ew := range emitWhen {
			if str, ok := ew.(string); ok {
				config.EmitWhen = append(config.EmitWhen, str)
			}
		}
	}

	return config, nil
}

// compileTemplate 编译模板
func (s *ParseTemplateService) compileTemplate(tpl *parser.RegexTemplate) (*parser.CompiledTemplate, error) {
	compiled := &parser.CompiledTemplate{
		RegexTemplate: *tpl,
	}

	switch tpl.Engine {
	case parser.EngineRegex:
		if tpl.Pattern != "" {
			re, err := regexp.Compile(tpl.Pattern)
			if err != nil {
				return nil, fmt.Errorf("编译正则模式失败: %w", err)
			}
			compiled.CompiledPattern = re
		}

	case parser.EngineAggregate:
		if tpl.Aggregation != nil && len(tpl.Aggregation.RecordStart) > 0 {
			compiled.CompiledRecordStart = make([]*regexp.Regexp, 0, len(tpl.Aggregation.RecordStart))
			for _, pattern := range tpl.Aggregation.RecordStart {
				re, err := regexp.Compile(pattern)
				if err != nil {
					return nil, fmt.Errorf("编译记录起始模式失败: %w", err)
				}
				compiled.CompiledRecordStart = append(compiled.CompiledRecordStart, re)
			}
		}

		if tpl.Aggregation != nil && len(tpl.Aggregation.CaptureRules) > 0 {
			compiled.CompiledCaptureRules = make([]parser.CompiledCaptureRule, 0, len(tpl.Aggregation.CaptureRules))
			for _, rule := range tpl.Aggregation.CaptureRules {
				re, err := regexp.Compile(rule.Pattern)
				if err != nil {
					return nil, fmt.Errorf("编译捕获规则模式失败: %w", err)
				}
				compiled.CompiledCaptureRules = append(compiled.CompiledCaptureRules, parser.CompiledCaptureRule{
					Pattern:         re,
					Mode:            rule.Mode,
					OriginalPattern: rule.Pattern,
				})
			}
		}
	}

	return compiled, nil
}
