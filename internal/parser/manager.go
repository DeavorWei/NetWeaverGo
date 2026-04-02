package parser

import (
	"embed"
	"encoding/json"
	"fmt"
	"regexp"
	"sync"
)

//go:embed templates/builtin/*.json
var builtinTemplateFS embed.FS

// ParserManager 模板管理器
// 负责模板装载、覆盖、重载、快照发布
type ParserManager struct {
	mu        sync.RWMutex
	snapshots map[string]*CompositeParser
}

// 确保 ParserManager 实现 ParserProvider 和 ParserReloader 接口
var _ ParserProvider = (*ParserManager)(nil)
var _ ParserReloader = (*ParserManager)(nil)

// NewParserManager 创建模板管理器
func NewParserManager() *ParserManager {
	return &ParserManager{
		snapshots: make(map[string]*CompositeParser),
	}
}

// Bootstrap 启动引导，加载所有厂商的内置模板
func (m *ParserManager) Bootstrap() error {
	for _, vendor := range []string{"huawei", "h3c", "cisco"} {
		if err := m.ReloadVendor(vendor); err != nil {
			return fmt.Errorf("加载厂商 %s 模板失败: %w", vendor, err)
		}
	}
	return nil
}

// GetParser 获取指定厂商的解析器（实现 ParserProvider 接口）
func (m *ParserManager) GetParser(vendor string) (CliParser, error) {
	m.mu.RLock()
	parser := m.snapshots[vendor]
	m.mu.RUnlock()

	if parser == nil {
		return nil, fmt.Errorf("未加载厂商解析器: %s: %w", vendor, ErrVendorNotLoaded)
	}
	return parser, nil
}

// ReloadVendor 重载指定厂商的解析器快照（实现 ParserReloader 接口）
func (m *ParserManager) ReloadVendor(vendor string) error {
	// 1. 加载内置模板
	builtinTemplates, err := m.loadBuiltinTemplates(vendor)
	if err != nil {
		return fmt.Errorf("加载内置模板失败: %w", err)
	}

	// 2. 编译模板
	compiledTemplates := make(map[string]*CompiledTemplate)
	for commandKey, tpl := range builtinTemplates.Templates {
		compiled, err := m.compileTemplate(&tpl)
		if err != nil {
			return fmt.Errorf("编译模板 %s 失败: %w", commandKey, err)
		}
		compiledTemplates[commandKey] = compiled
	}

	// 3. 创建新的组合解析器
	newParser := NewCompositeParser(vendor, compiledTemplates)

	// 4. 原子替换快照
	m.mu.Lock()
	m.snapshots[vendor] = newParser
	m.mu.Unlock()

	return nil
}

// loadBuiltinTemplates 从嵌入的文件系统加载内置模板
func (m *ParserManager) loadBuiltinTemplates(vendor string) (*VendorTemplates, error) {
	path := fmt.Sprintf("templates/builtin/%s.json", vendor)
	data, err := builtinTemplateFS.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取内置模板文件 %s 失败: %w", path, err)
	}

	var vendorTemplates VendorTemplates
	if err := json.Unmarshal(data, &vendorTemplates); err != nil {
		return nil, fmt.Errorf("解析内置模板 JSON %s 失败: %w", path, err)
	}

	// 设置 vendor 字段（如果 JSON 中未指定）
	if vendorTemplates.Vendor == "" {
		vendorTemplates.Vendor = vendor
	}

	return &vendorTemplates, nil
}

// compileTemplate 编译模板
func (m *ParserManager) compileTemplate(tpl *RegexTemplate) (*CompiledTemplate, error) {
	compiled := &CompiledTemplate{
		RegexTemplate: *tpl,
	}

	// 根据引擎类型编译不同的模式
	switch tpl.Engine {
	case EngineRegex:
		// 编译主正则模式
		if tpl.Pattern != "" {
			re, err := regexp.Compile(tpl.Pattern)
			if err != nil {
				return nil, fmt.Errorf("编译正则模式失败: %w", err)
			}
			compiled.CompiledPattern = re
		}

	case EngineAggregate:
		// 编译记录起始模式
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

		// 编译捕获规则
		if tpl.Aggregation != nil && len(tpl.Aggregation.CaptureRules) > 0 {
			compiled.CompiledCaptureRules = make([]CompiledCaptureRule, 0, len(tpl.Aggregation.CaptureRules))
			for _, rule := range tpl.Aggregation.CaptureRules {
				re, err := regexp.Compile(rule.Pattern)
				if err != nil {
					return nil, fmt.Errorf("编译捕获规则模式失败: %w", err)
				}
				compiled.CompiledCaptureRules = append(compiled.CompiledCaptureRules, CompiledCaptureRule{
					Pattern:         re,
					Mode:            rule.Mode,
					OriginalPattern: rule.Pattern,
				})
			}
		}

	default:
		return nil, fmt.Errorf("不支持的模板引擎: %s: %w", tpl.Engine, ErrUnsupportedEngine)
	}

	return compiled, nil
}

// GetSnapshot 获取指定厂商的解析器快照（返回具体类型，便于高级操作）
func (m *ParserManager) GetSnapshot(vendor string) (*CompositeParser, error) {
	m.mu.RLock()
	parser := m.snapshots[vendor]
	m.mu.RUnlock()

	if parser == nil {
		return nil, fmt.Errorf("未加载厂商解析器: %s: %w", vendor, ErrVendorNotLoaded)
	}
	return parser, nil
}

// ListVendors 列出已加载的厂商
func (m *ParserManager) ListVendors() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	vendors := make([]string, 0, len(m.snapshots))
	for v := range m.snapshots {
		vendors = append(vendors, v)
	}
	return vendors
}
