package parser

import (
	"fmt"
	"time"
)

// ModeProvider 提供当前引擎模式获取函数
type ModeProvider func() EngineMode

// MetricsRecorder 提供解析运行指标记录函数
type MetricsRecorder func(success bool, fallback bool, durationMs int64)

// CompositeParser 厂商级只读解析器快照
// 内部保存已编译模板
// Parse 时根据 EngineMode 和模板声明智能路由分发，并记录指标
type CompositeParser struct {
	vendor          string
	templates       map[string]*CompiledTemplate
	regex           *RegexParser
	aggregate       *AggregateEngine
	tree            *TreeEngine
	modeProvider    ModeProvider
	metricsRecorder MetricsRecorder
}

// 确保 CompositeParser 实现 CliParser 接口
var _ CliParser = (*CompositeParser)(nil)

// NewCompositeParser 创建组合解析器
func NewCompositeParser(vendor string, templates map[string]*CompiledTemplate) *CompositeParser {
	return &CompositeParser{
		vendor:    vendor,
		templates: templates,
		regex:     NewRegexParser(),
		aggregate: NewAggregateEngine(),
		tree:      NewTreeEngine(),
	}
}

// SetModeProvider 设置灰度引擎模式提供者
func (p *CompositeParser) SetModeProvider(mp ModeProvider) {
	p.modeProvider = mp
}

// SetMetricsRecorder 设置指标记录器
func (p *CompositeParser) SetMetricsRecorder(mr MetricsRecorder) {
	p.metricsRecorder = mr
}

// Parse 实现 CliParser 接口
// 严格按照 EngineMode (auto/legacy_only/tree_only) 执行路由分派，并自动记录指标
func (p *CompositeParser) Parse(commandKey string, rawText string) ([]map[string]string, error) {
	start := time.Now()
	var (
		results    []map[string]string
		err        error
		isFallback bool
	)
	defer func() {
		if p.metricsRecorder != nil {
			durationMs := time.Since(start).Milliseconds()
			p.metricsRecorder(err == nil, isFallback, durationMs)
		}
	}()

	tpl, ok := p.templates[commandKey]
	if !ok {
		err = fmt.Errorf("未找到模板: vendor=%s commandKey=%s: %w", p.vendor, commandKey, ErrTemplateNotFound)
		return nil, err
	}

	mode := EngineModeAuto
	if p.modeProvider != nil {
		mode = p.modeProvider()
	}

	switch mode {
	case EngineModeLegacyOnly:
		// 方案 §10.3 应急回退模式：完全拒绝 tree 引擎，强制走 legacy (regex/aggregate)
		if tpl.Engine == EngineTree {
			if tpl.CompiledPattern != nil {
				isFallback = true
				results, err = p.regex.ParseWithTemplate(tpl, rawText)
			} else if len(tpl.CompiledCaptureRules) > 0 {
				isFallback = true
				results, err = p.aggregate.ParseWithTemplate(tpl, rawText)
			} else {
				err = fmt.Errorf("EngineMode 为 legacy_only，但模板 %s 仅提供 tree 引擎且无 legacy 降级规则: %w", commandKey, ErrUnsupportedEngine)
				return nil, err
			}
		} else if tpl.Engine == EngineRegex {
			results, err = p.regex.ParseWithTemplate(tpl, rawText)
		} else if tpl.Engine == EngineAggregate {
			results, err = p.aggregate.ParseWithTemplate(tpl, rawText)
		} else {
			err = fmt.Errorf("不支持的模板引擎: %s: %w", tpl.Engine, ErrUnsupportedEngine)
			return nil, err
		}

	case EngineModeTreeOnly:
		// 强制只使用 tree 引擎
		if tpl.Engine == EngineTree {
			results, err = p.tree.ParseWithTemplate(tpl, rawText)
		} else {
			err = fmt.Errorf("EngineMode 为 tree_only，但模板 %s 的声明引擎为 %s: %w", commandKey, tpl.Engine, ErrUnsupportedEngine)
			return nil, err
		}

	default: // EngineModeAuto 默认自适应模式
		switch tpl.Engine {
		case EngineRegex:
			results, err = p.regex.ParseWithTemplate(tpl, rawText)
		case EngineAggregate:
			results, err = p.aggregate.ParseWithTemplate(tpl, rawText)
		case EngineTree:
			results, err = p.tree.ParseWithTemplate(tpl, rawText)
			// 若 tree 引擎解析失败或无数据产出，且模板配置了 legacy 备选方案，执行应急 fallback
			if (err != nil || len(results) == 0) && (tpl.CompiledPattern != nil || len(tpl.CompiledCaptureRules) > 0) {
				if tpl.CompiledPattern != nil {
					fallbackRes, fallbackErr := p.regex.ParseWithTemplate(tpl, rawText)
					if fallbackErr == nil && len(fallbackRes) > 0 {
						isFallback = true
						results = fallbackRes
						err = nil
					}
				} else if len(tpl.CompiledCaptureRules) > 0 {
					fallbackRes, fallbackErr := p.aggregate.ParseWithTemplate(tpl, rawText)
					if fallbackErr == nil && len(fallbackRes) > 0 {
						isFallback = true
						results = fallbackRes
						err = nil
					}
				}
			}
		default:
			err = fmt.Errorf("不支持的模板引擎: %s: %w", tpl.Engine, ErrUnsupportedEngine)
			return nil, err
		}
	}

	return results, err
}

// GetTemplate 获取指定命令的已编译模板
func (p *CompositeParser) GetTemplate(commandKey string) (*CompiledTemplate, bool) {
	tpl, ok := p.templates[commandKey]
	return tpl, ok
}

// ListCommandKeys 列出所有支持的命令键
func (p *CompositeParser) ListCommandKeys() []string {
	keys := make([]string, 0, len(p.templates))
	for k := range p.templates {
		keys = append(keys, k)
	}
	return keys
}

// Vendor 返回厂商名称
func (p *CompositeParser) Vendor() string {
	return p.vendor
}
