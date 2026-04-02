package parser

import (
	"regexp"
	"strings"
)

// AggregateEngine 多行聚合解析引擎
// 读取 AggregationConfig，按 recordStart 切换记录上下文，
// 按 captureRules 更新当前记录，按 filldown 填充字段，
// 按 emitWhen 决定何时输出记录
type AggregateEngine struct{}

// NewAggregateEngine 创建多行聚合解析引擎
func NewAggregateEngine() *AggregateEngine {
	return &AggregateEngine{}
}

// ParseWithTemplate 使用已编译模板解析原始文本
func (e *AggregateEngine) ParseWithTemplate(tpl *CompiledTemplate, rawText string) ([]map[string]string, error) {
	if tpl == nil {
		return nil, ErrNilTemplate
	}

	if tpl.Aggregation == nil {
		return nil, ErrInvalidAggregationConfig
	}

	if len(tpl.CompiledRecordStart) == 0 {
		return nil, ErrInvalidAggregationConfig
	}

	// 按行处理
	lines := strings.Split(rawText, "\n")

	var results []map[string]string
	var currentRecord map[string]string
	var filldownValues map[string]string

	// 初始化 filldown 存储
	if len(tpl.Aggregation.Filldown) > 0 {
		filldownValues = make(map[string]string)
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// 检查是否匹配记录起始模式
		if matched, captures := e.matchAndExtractFromRecordStart(tpl.CompiledRecordStart, line); matched {
			// 输出上一条记录（如果满足 emitWhen 条件）
			if currentRecord != nil && e.shouldEmit(currentRecord, tpl.Aggregation.EmitWhen) {
				results = append(results, currentRecord)
			}

			// 开始新记录
			currentRecord = make(map[string]string)

			// 应用 filldown 值
			if filldownValues != nil {
				for _, field := range tpl.Aggregation.Filldown {
					if val, ok := filldownValues[field]; ok {
						currentRecord[field] = val
					}
				}
			}

			// 从 recordStart 模式中提取的命名捕获组
			for k, v := range captures {
				currentRecord[k] = v
				// 更新 filldown 值
				if filldownValues != nil {
					for _, field := range tpl.Aggregation.Filldown {
						if field == k {
							filldownValues[k] = v
							break
						}
					}
				}
			}

			// 尝试从起始行捕获字段
			e.applyCaptureRules(tpl.CompiledCaptureRules, line, currentRecord)
			continue
		}

		// 如果当前有记录，尝试应用捕获规则
		if currentRecord != nil {
			e.applyCaptureRules(tpl.CompiledCaptureRules, line, currentRecord)
		}
	}

	// 输出最后一条记录
	if currentRecord != nil && e.shouldEmit(currentRecord, tpl.Aggregation.EmitWhen) {
		results = append(results, currentRecord)
	}

	// 应用字段映射
	if len(tpl.FieldMapping) > 0 {
		results = e.applyFieldMapping(results, tpl.FieldMapping)
	}

	return results, nil
}

// matchAndExtractFromRecordStart 检查行是否匹配任意一个记录起始模式，并提取命名捕获组
func (e *AggregateEngine) matchAndExtractFromRecordStart(patterns []*regexp.Regexp, line string) (bool, map[string]string) {
	for _, pattern := range patterns {
		match := pattern.FindStringSubmatch(line)
		if match == nil {
			continue
		}

		// 提取命名捕获组
		captures := make(map[string]string)
		subexpNames := pattern.SubexpNames()
		for i, name := range subexpNames {
			if i == 0 || name == "" {
				continue
			}
			if i < len(match) {
				value := strings.TrimSpace(match[i])
				if value != "" {
					captures[name] = value
				}
			}
		}
		return true, captures
	}
	return false, nil
}

// applyCaptureRules 应用捕获规则到当前记录
func (e *AggregateEngine) applyCaptureRules(rules []CompiledCaptureRule, line string, record map[string]string) {
	for _, rule := range rules {
		match := rule.Pattern.FindStringSubmatch(line)
		if match == nil {
			continue
		}

		// 提取命名捕获组
		subexpNames := rule.Pattern.SubexpNames()
		for i, name := range subexpNames {
			if i == 0 || name == "" {
				continue
			}
			if i < len(match) {
				value := strings.TrimSpace(match[i])
				if value == "" {
					continue
				}

				switch rule.Mode {
				case "set":
					record[name] = value
				case "append":
					if existing, ok := record[name]; ok && existing != "" {
						record[name] = existing + "," + value
					} else {
						record[name] = value
					}
				default:
					// 默认为 set 模式
					record[name] = value
				}
			}
		}
	}
}

// shouldEmit 检查记录是否满足输出条件
func (e *AggregateEngine) shouldEmit(record map[string]string, emitWhen []string) bool {
	if len(emitWhen) == 0 {
		return true // 没有条件则总是输出
	}

	for _, field := range emitWhen {
		if val, ok := record[field]; ok && val != "" {
			return true
		}
	}
	return false
}

// applyFieldMapping 应用字段映射
func (e *AggregateEngine) applyFieldMapping(results []map[string]string, mapping map[string]string) []map[string]string {
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
