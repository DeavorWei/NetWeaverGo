package parser

import (
	"embed"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
)

//go:embed templates/builtin/*.json
var builtinTemplateFS embed.FS

// StoredTemplate 用户自定义存储模板数据传输对象
type StoredTemplate struct {
	Vendor       string
	CommandKey   string
	Engine       string
	Pattern      string
	Multiline    bool
	Aggregation  string
	ParseRules   string
	FieldMapping string
	Enabled      bool
}

// UserTemplateSource 用户自定义模板持久化数据源接口（打通断头路 #2）
type UserTemplateSource interface {
	ListEnabled(vendor string) ([]StoredTemplate, error)
}

// ParserManager 模板管理器
// 负责模板装载、覆盖、重载、快照发布
type ParserManager struct {
	mu         sync.RWMutex
	snapshots  map[string]*CompositeParser
	userSource UserTemplateSource
	engineMode EngineMode
	metrics    ParserMetrics
}

// 确保 ParserManager 实现 ParserProvider 和 ParserReloader 接口
var _ ParserProvider = (*ParserManager)(nil)
var _ ParserReloader = (*ParserManager)(nil)

// NewParserManager 创建模板管理器
func NewParserManager() *ParserManager {
	return &ParserManager{
		snapshots:  make(map[string]*CompositeParser),
		engineMode: EngineModeAuto,
	}
}

// SetUserTemplateSource 设置用户模板数据源（打通断头路 #2）
func (m *ParserManager) SetUserTemplateSource(src UserTemplateSource) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.userSource = src
}

// SetEngineMode 设置灰度引擎模式（方案 §10.3）
func (m *ParserManager) SetEngineMode(mode EngineMode) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.engineMode = mode
}

// GetEngineMode 获取当前灰度引擎模式
func (m *ParserManager) GetEngineMode() EngineMode {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.engineMode == "" {
		return EngineModeAuto
	}
	return m.engineMode
}

// GetMetrics 获取解析运行指标（方案 §10.2）
func (m *ParserManager) GetMetrics() ParserMetrics {
	return ParserMetrics{
		TotalParsed:     atomic.LoadUint64(&m.metrics.TotalParsed),
		SuccessCount:    atomic.LoadUint64(&m.metrics.SuccessCount),
		FailureCount:    atomic.LoadUint64(&m.metrics.FailureCount),
		FallbackCount:   atomic.LoadUint64(&m.metrics.FallbackCount),
		TotalDurationMs: atomic.LoadInt64(&m.metrics.TotalDurationMs),
	}
}

// RecordParse 累加解析运行指标（并发原子安全）
func (m *ParserManager) RecordParse(success bool, fallback bool, durationMs int64) {
	atomic.AddUint64(&m.metrics.TotalParsed, 1)
	if success {
		atomic.AddUint64(&m.metrics.SuccessCount, 1)
	} else {
		atomic.AddUint64(&m.metrics.FailureCount, 1)
	}
	if fallback {
		atomic.AddUint64(&m.metrics.FallbackCount, 1)
	}
	atomic.AddInt64(&m.metrics.TotalDurationMs, durationMs)
}

// ResetMetrics 重置解析运行指标
func (m *ParserManager) ResetMetrics() {
	atomic.StoreUint64(&m.metrics.TotalParsed, 0)
	atomic.StoreUint64(&m.metrics.SuccessCount, 0)
	atomic.StoreUint64(&m.metrics.FailureCount, 0)
	atomic.StoreUint64(&m.metrics.FallbackCount, 0)
	atomic.StoreInt64(&m.metrics.TotalDurationMs, 0)
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
// 执行流程：加载内置模板 -> 合并数据库用户模板（同 commandKey 覆盖） -> 编译 -> 原子发布快照
func (m *ParserManager) ReloadVendor(vendor string) error {
	// 1. 加载内置模板
	builtinTemplates, err := m.loadBuiltinTemplates(vendor)
	if err != nil {
		return fmt.Errorf("加载内置模板失败: %w", err)
	}

	mergedTemplates := make(map[string]RegexTemplate, len(builtinTemplates.Templates))
	for k, v := range builtinTemplates.Templates {
		mergedTemplates[k] = v
	}

	// 2. 合并用户持久化模板（断头路 #2 修复）
	m.mu.RLock()
	src := m.userSource
	m.mu.RUnlock()

	if src != nil {
		userList, err := src.ListEnabled(vendor)
		if err != nil {
			return fmt.Errorf("加载厂商 %s 用户模板失败: %w", vendor, err)
		}

		for _, ut := range userList {
			if !ut.Enabled {
				continue
			}
			t := RegexTemplate{
				Vendor:      ut.Vendor,
				CommandKey:  ut.CommandKey,
				Engine:      TemplateEngine(ut.Engine),
				Pattern:     ut.Pattern,
				Multiline:   ut.Multiline,
				Description: "User Template",
			}

			if ut.FieldMapping != "" {
				var fm map[string]string
				if err := json.Unmarshal([]byte(ut.FieldMapping), &fm); err == nil {
					t.FieldMapping = fm
				}
			}

			if ut.Aggregation != "" && t.Engine == EngineAggregate {
				var agg AggregationConfig
				if err := json.Unmarshal([]byte(ut.Aggregation), &agg); err == nil {
					t.Aggregation = &agg
				}
			}

			if ut.ParseRules != "" && t.Engine == EngineTree {
				var treeConfig TreeTemplate
				if err := json.Unmarshal([]byte(ut.ParseRules), &treeConfig); err == nil {
					t.TreeConfig = &treeConfig
				}
			}

			mergedTemplates[ut.CommandKey] = t
		}
	}

	// 3. 编译所有模板
	compiledTemplates := make(map[string]*CompiledTemplate, len(mergedTemplates))
	for commandKey, tpl := range mergedTemplates {
		compiled, err := m.compileTemplate(&tpl)
		if err != nil {
			return fmt.Errorf("编译模板 %s 失败: %w", commandKey, err)
		}
		compiledTemplates[commandKey] = compiled
	}

	// 4. 创建新的组合解析器并注入模式与指标记录器
	newParser := NewCompositeParser(vendor, compiledTemplates)
	newParser.SetModeProvider(m.GetEngineMode)
	newParser.SetMetricsRecorder(m.RecordParse)

	// 5. 原子替换快照
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
		// 未知厂商/第三方厂商无内置文件时，优雅返回空模板集合，支持纯用户模板
		return &VendorTemplates{
			Vendor:    vendor,
			Templates: make(map[string]RegexTemplate),
		}, nil
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
			pattern := tpl.Pattern
			if tpl.Multiline && !strings.HasPrefix(pattern, "(?m)") {
				pattern = "(?m)" + pattern
			}
			re, err := regexp.Compile(pattern)
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

	case EngineTree:
		// 编译规则树
		if tpl.TreeConfig != nil && len(tpl.TreeConfig.Rules) > 0 {
			compiledRules, rootRules, err := CompileTreeRules(tpl.TreeConfig.Rules)
			if err != nil {
				return nil, fmt.Errorf("编译规则树失败: %w", err)
			}
			compiled.CompiledTreeRules = compiledRules
			compiled.TreeRootRules = rootRules
		}
		// 若同时提供了主正则 Pattern，编译之以备应急 fallback 使用
		if tpl.Pattern != "" {
			pattern := tpl.Pattern
			if tpl.Multiline && !strings.HasPrefix(pattern, "(?m)") {
				pattern = "(?m)" + pattern
			}
			re, err := regexp.Compile(pattern)
			if err == nil {
				compiled.CompiledPattern = re
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
