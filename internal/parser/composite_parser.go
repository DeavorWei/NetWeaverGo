package parser

import (
	"fmt"
)

// CompositeParser 厂商级只读解析器快照
// 内部只保存已编译模板
// Parse 时根据模板 engine 选择 RegexParser 或 AggregateEngine
type CompositeParser struct {
	vendor    string
	templates map[string]*CompiledTemplate
	regex     *RegexParser
	aggregate *AggregateEngine
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
	}
}

// Parse 实现 CliParser 接口
func (p *CompositeParser) Parse(commandKey string, rawText string) ([]map[string]string, error) {
	tpl, ok := p.templates[commandKey]
	if !ok {
		return nil, fmt.Errorf("未找到模板: vendor=%s commandKey=%s: %w", p.vendor, commandKey, ErrTemplateNotFound)
	}

	switch tpl.Engine {
	case EngineRegex:
		return p.regex.ParseWithTemplate(tpl, rawText)
	case EngineAggregate:
		return p.aggregate.ParseWithTemplate(tpl, rawText)
	default:
		return nil, fmt.Errorf("不支持的模板引擎: %s: %w", tpl.Engine, ErrUnsupportedEngine)
	}
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
