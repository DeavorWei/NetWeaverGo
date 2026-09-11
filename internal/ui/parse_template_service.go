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
	if req.Engine != "regex" && req.Engine != "aggregate" && req.Engine != "tree" {
		return fmt.Errorf("不支持的引擎类型: %s", req.Engine)
	}

	tpl := &parser.RegexTemplate{
		Vendor:     req.Vendor,
		CommandKey: req.CommandKey,
		Engine:     parser.TemplateEngine(req.Engine),
		Pattern:    req.Pattern,
		Multiline:  req.Multiline,
	}

	var aggregationJSON, parseRulesJSON, fieldMappingJSON string

	switch req.Engine {
	case "regex":
		if req.Pattern != "" {
			if _, err := regexp.Compile(req.Pattern); err != nil {
				return fmt.Errorf("正则模式编译失败: %w", err)
			}
		}
		if req.FieldMapping != nil {
			data, err := json.Marshal(req.FieldMapping)
			if err != nil {
				return fmt.Errorf("序列化字段映射失败: %w", err)
			}
			fieldMappingJSON = string(data)
			tpl.FieldMapping = req.FieldMapping
		}
	case "aggregate":
		if req.Aggregation != nil {
			aggConfig, err := s.parseAggregationConfig(req.Aggregation)
			if err != nil {
				return fmt.Errorf("聚合配置解析失败: %w", err)
			}
			tpl.Aggregation = aggConfig
			data, err := json.Marshal(req.Aggregation)
			if err != nil {
				return fmt.Errorf("序列化聚合配置失败: %w", err)
			}
			aggregationJSON = string(data)
		}
	case "tree":
		if req.ParseRules != nil {
			treeConfig, err := s.parseTreeConfig(req.ParseRules)
			if err != nil {
				return fmt.Errorf("规则树配置解析失败: %w", err)
			}
			tpl.TreeConfig = treeConfig
			data, err := json.Marshal(req.ParseRules)
			if err != nil {
				return fmt.Errorf("序列化规则树配置失败: %w", err)
			}
			parseRulesJSON = string(data)
		}
	}

	// 编译预检
	if _, err := s.compileTemplate(tpl); err != nil {
		return fmt.Errorf("模板预编译校验失败: %w", err)
	}

	// 检查唯一键冲突
	var count int64
	s.db.Model(&models.UserParseTemplate{}).
		Where("vendor = ? AND command_key = ?", req.Vendor, req.CommandKey).
		Count(&count)
	if count > 0 {
		return fmt.Errorf("模板已存在: vendor=%s commandKey=%s", req.Vendor, req.CommandKey)
	}

	var appliesToJSON string
	if req.AppliesTo != nil {
		if data, err := json.Marshal(req.AppliesTo); err == nil {
			appliesToJSON = string(data)
		}
	}

	t := models.UserParseTemplate{
		Vendor:       req.Vendor,
		CommandKey:   req.CommandKey,
		Engine:       req.Engine,
		Pattern:      req.Pattern,
		Multiline:    req.Multiline,
		Aggregation:  aggregationJSON,
		ParseRules:   parseRulesJSON,
		FieldMapping: fieldMappingJSON,
		AppliesTo:    appliesToJSON,
		Description:  req.Description,
		Enabled:      req.Enabled,
		Revision:     1,
	}

	if err := s.db.Create(&t).Error; err != nil {
		return fmt.Errorf("创建模板失败: %w", err)
	}

	// 刷新解析器快照，若失败则撤销创建回滚 DB，防止脏数据
	if err := s.reloader.ReloadVendor(req.Vendor); err != nil {
		_ = s.db.Delete(&t).Error
		return fmt.Errorf("刷新解析器失败，已撤销模板创建: %w", err)
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
	if req.Engine != "regex" && req.Engine != "aggregate" && req.Engine != "tree" {
		return fmt.Errorf("不支持的引擎类型: %s", req.Engine)
	}

	// 构造预编译模板
	tpl := &parser.RegexTemplate{
		Vendor:     t.Vendor,
		CommandKey: t.CommandKey,
		Engine:     parser.TemplateEngine(req.Engine),
		Pattern:    req.Pattern,
		Multiline:  req.Multiline,
	}

	// 更新规则配置，未提供则保留原配置，绝不写入 "null"（解决 H1）
	aggregationJSON := t.Aggregation
	parseRulesJSON := t.ParseRules
	fieldMappingJSON := t.FieldMapping

	switch req.Engine {
	case "regex":
		if req.Pattern != "" {
			if _, err := regexp.Compile(req.Pattern); err != nil {
				return fmt.Errorf("正则模式编译失败: %w", err)
			}
		}
		if req.FieldMapping != nil {
			data, err := json.Marshal(req.FieldMapping)
			if err != nil {
				return fmt.Errorf("序列化字段映射失败: %w", err)
			}
			fieldMappingJSON = string(data)
			tpl.FieldMapping = req.FieldMapping
		} else if t.FieldMapping != "" {
			_ = json.Unmarshal([]byte(t.FieldMapping), &tpl.FieldMapping)
		}
	case "aggregate":
		if req.Aggregation != nil {
			if len(req.Aggregation) == 0 {
				// 显式清空聚合配置
				tpl.Aggregation = nil
				aggregationJSON = ""
			} else {
				aggConfig, err := s.parseAggregationConfig(req.Aggregation)
				if err != nil {
					return fmt.Errorf("聚合配置解析失败: %w", err)
				}
				tpl.Aggregation = aggConfig
				data, err := json.Marshal(req.Aggregation)
				if err != nil {
					return fmt.Errorf("序列化聚合配置失败: %w", err)
				}
				aggregationJSON = string(data)
			}
		} else if t.Aggregation != "" {
			var rawMap map[string]interface{}
			if err := json.Unmarshal([]byte(t.Aggregation), &rawMap); err == nil {
				aggConfig, _ := s.parseAggregationConfig(rawMap)
				tpl.Aggregation = aggConfig
			}
		}
	case "tree":
		if req.ParseRules != nil {
			treeConfig, err := s.parseTreeConfig(req.ParseRules)
			if err != nil {
				return fmt.Errorf("规则树配置解析失败: %w", err)
			}
			tpl.TreeConfig = treeConfig
			data, err := json.Marshal(req.ParseRules)
			if err != nil {
				return fmt.Errorf("序列化规则树配置失败: %w", err)
			}
			parseRulesJSON = string(data)
		} else if t.ParseRules != "" {
			var rawMap map[string]interface{}
			if err := json.Unmarshal([]byte(t.ParseRules), &rawMap); err == nil {
				treeConfig, _ := s.parseTreeConfig(rawMap)
				tpl.TreeConfig = treeConfig
			}
		}
	}

	// 编译校验
	if _, err := s.compileTemplate(tpl); err != nil {
		return fmt.Errorf("更新模板预编译校验失败: %w", err)
	}

	// 记录旧状态
	oldT := t

	// 更新字段（锁定 vendor 与 commandKey 禁止篡改，解决 M5）
	t.Engine = req.Engine
	t.Pattern = req.Pattern
	t.Multiline = req.Multiline
	t.Aggregation = aggregationJSON
	t.ParseRules = parseRulesJSON
	t.FieldMapping = fieldMappingJSON
	// 适用范围采用"全量显式"语义：未提供(nil)或条件为空均视为清空，
	// 保证用户可把模板还原为"全款型/全版本通用"，避免一旦设置就无法清除。
	if req.AppliesTo == nil || (len(req.AppliesTo.Models) == 0 && len(req.AppliesTo.Versions) == 0) {
		t.AppliesTo = ""
	} else if data, err := json.Marshal(req.AppliesTo); err == nil {
		t.AppliesTo = string(data)
	}
	t.Description = req.Description
	t.Enabled = req.Enabled
	t.Revision++

	if err := s.db.Save(&t).Error; err != nil {
		return fmt.Errorf("更新模板失败: %w", err)
	}

	// 刷新快照，若失败回滚 DB
	if err := s.reloader.ReloadVendor(t.Vendor); err != nil {
		_ = s.db.Save(&oldT).Error
		return fmt.Errorf("刷新解析器失败，已回滚更新: %w", err)
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

	// 解析树形配置
	if req.Engine == "tree" && req.ParseRules != nil {
		treeConfig, err := s.parseTreeConfig(req.ParseRules)
		if err != nil {
			result.Error = fmt.Sprintf("解析规则树配置失败: %v", err)
			return result
		}
		tpl.TreeConfig = treeConfig
	}

	// 字段映射（解决 M3）
	tpl.FieldMapping = req.FieldMapping

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
	case parser.EngineTree:
		engine := parser.NewTreeEngine()
		rows, err = engine.ParseWithTemplate(compiled, req.RawText)
	default:
		result.Error = fmt.Sprintf("不支持的引擎类型: %s", tpl.Engine)
		return result
	}

	if err != nil {
		result.Error = fmt.Sprintf("解析失败: %v", err)
		return result
	}

	// 应用字段映射
	if len(compiled.FieldMapping) > 0 {
		for i, row := range rows {
			mappedRow := make(map[string]string, len(row))
			for k, v := range row {
				if targetKey, ok := compiled.FieldMapping[k]; ok && targetKey != "" {
					mappedRow[targetKey] = v
				} else {
					mappedRow[k] = v
				}
			}
			rows[i] = mappedRow
		}
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
	if t.ParseRules != "" {
		if err := json.Unmarshal([]byte(t.ParseRules), &vo.ParseRules); err != nil {
			return vo, fmt.Errorf("解析规则树配置失败: %w", err)
		}
	}
	if t.FieldMapping != "" {
		if err := json.Unmarshal([]byte(t.FieldMapping), &vo.FieldMapping); err != nil {
			return vo, fmt.Errorf("解析字段映射失败: %w", err)
		}
	}
	if t.AppliesTo != "" {
		if err := json.Unmarshal([]byte(t.AppliesTo), &vo.AppliesTo); err != nil {
			return vo, fmt.Errorf("解析适用范围失败: %w", err)
		}
	}

	return vo, nil
}

// parseTreeConfig 解析规则树配置
func (s *ParseTemplateService) parseTreeConfig(data map[string]interface{}) (*parser.TreeTemplate, error) {
	if data == nil {
		return nil, fmt.Errorf("规则树配置为空")
	}
	bytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("序列化规则树配置失败: %w", err)
	}

	var treeConfig parser.TreeTemplate
	if err := json.Unmarshal(bytes, &treeConfig); err != nil {
		return nil, fmt.Errorf("反序列化规则树配置失败: %w", err)
	}
	return &treeConfig, nil
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

	case parser.EngineTree:
		if tpl.TreeConfig != nil && len(tpl.TreeConfig.Rules) > 0 {
			compiledRules, rootRules, err := parser.CompileTreeRules(tpl.TreeConfig.Rules)
			if err != nil {
				return nil, fmt.Errorf("编译规则树失败: %w", err)
			}
			compiled.CompiledTreeRules = compiledRules
			compiled.TreeRootRules = rootRules
		}
	}

	return compiled, nil
}
