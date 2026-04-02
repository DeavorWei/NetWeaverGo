package parser

import (
	"regexp"
	"strings"
)

// RegexParser 纯正则解析引擎
// 只负责对单个已编译模板执行正则匹配
// 不负责模板加载、模板来源判断、模板覆盖逻辑
type RegexParser struct{}

// NewRegexParser 创建纯正则解析器
func NewRegexParser() *RegexParser {
	return &RegexParser{}
}

// ParseWithTemplate 使用已编译模板解析原始文本
func (p *RegexParser) ParseWithTemplate(tpl *CompiledTemplate, rawText string) ([]map[string]string, error) {
	if tpl == nil {
		return nil, ErrNilTemplate
	}

	if tpl.CompiledPattern == nil {
		return nil, ErrPatternNotCompiled
	}

	// 根据是否多行模式选择匹配方式
	var matches [][]string
	if tpl.Multiline {
		matches = tpl.CompiledPattern.FindAllStringSubmatch(rawText, -1)
	} else {
		// 单行模式：逐行匹配
		lines := strings.Split(rawText, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			match := tpl.CompiledPattern.FindStringSubmatch(line)
			if match != nil {
				matches = append(matches, match)
			}
		}
	}

	// 提取命名捕获组
	results := make([]map[string]string, 0, len(matches))
	for _, match := range matches {
		row := p.extractNamedGroups(tpl.CompiledPattern, match)
		if len(row) > 0 {
			results = append(results, row)
		}
	}

	// 应用字段映射
	if len(tpl.FieldMapping) > 0 {
		results = p.applyFieldMapping(results, tpl.FieldMapping)
	}

	return results, nil
}

// extractNamedGroups 从匹配结果中提取命名捕获组
func (p *RegexParser) extractNamedGroups(re *regexp.Regexp, match []string) map[string]string {
	if len(match) == 0 {
		return nil
	}

	row := make(map[string]string)
	subexpNames := re.SubexpNames()

	for i, name := range subexpNames {
		if i == 0 || name == "" {
			continue // 跳过整个匹配和未命名组
		}
		if i < len(match) {
			row[name] = strings.TrimSpace(match[i])
		}
	}

	return row
}

// applyFieldMapping 应用字段映射
func (p *RegexParser) applyFieldMapping(results []map[string]string, mapping map[string]string) []map[string]string {
	for _, row := range results {
		for oldKey, newKey := range mapping {
			if val, ok := row[oldKey]; ok {
				delete(row, oldKey)
				row[newKey] = val
			}
		}
	}
	return results
}
