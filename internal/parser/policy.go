package parser

import (
	"regexp"
	"strings"

	"github.com/NetWeaverGo/core/internal/parser/xmlcfg"
)

// ParsePolicy 解析策略统一接口
type ParsePolicy interface {
	PolicyName() string
	Execute(text string, node *xmlcfg.ParseNode) ([]map[string]string, error)
}

// ----------------------------------------------------------------------------
// ConfigParsePolicy 非表格块文本属性抽取策略
// ----------------------------------------------------------------------------
type ConfigPolicyExecutor struct{}

func (e *ConfigPolicyExecutor) PolicyName() string { return "ConfigParsePolicy" }

func (e *ConfigPolicyExecutor) Execute(text string, node *xmlcfg.ParseNode) ([]map[string]string, error) {
	if node.ConfigPolicy == nil {
		return nil, nil
	}

	row := make(map[string]string)
	pipeline := buildFieldPipeline(node.ConfigPolicy.Fields)

	// 对每个 Field 使用其独立的 Regex 尝试提取
	for _, f := range node.ConfigPolicy.Fields {
		if f.Regex == "" {
			continue
		}
		cleanPat, re, _, err := xmlcfg.CleanAndCompileRegex(f.Regex)
		if err != nil || re == nil {
			continue
		}

		names := strings.Split(f.Name, ",")
		sub := re.FindStringSubmatch(text)
		if len(sub) == 0 {
			continue
		}

		// 优先从命名捕获组取值
		subNames := re.SubexpNames()
		matchedNamed := false
		for idx, name := range subNames {
			if idx > 0 && name != "" && idx < len(sub) {
				row[name] = strings.TrimSpace(sub[idx])
				matchedNamed = true
			}
		}

		// 若无命名组，按 group 1 赋值给第一个 fieldName
		if !matchedNamed && len(sub) > 1 && len(names) > 0 {
			row[strings.TrimSpace(names[0])] = strings.TrimSpace(sub[1])
		}
		_ = cleanPat
	}

	if len(row) == 0 {
		return nil, nil
	}

	// 跑算子管线
	finalRow, ok := pipeline.ExecuteRow(row)
	if !ok || len(finalRow) == 0 {
		return nil, nil
	}

	return []map[string]string{finalRow}, nil
}

// ----------------------------------------------------------------------------
// TableLineParsePolicy 逐行表格正则抽取策略
// ----------------------------------------------------------------------------
type TableLinePolicyExecutor struct{}

func (e *TableLinePolicyExecutor) PolicyName() string { return "TableLineParsePolicy" }

func (e *TableLinePolicyExecutor) Execute(text string, node *xmlcfg.ParseNode) ([]map[string]string, error) {
	if node.TableLinePolicy == nil || strings.TrimSpace(node.TableLinePolicy.Regex) == "" {
		return nil, nil
	}

	_, re, _, err := xmlcfg.CleanAndCompileRegex(node.TableLinePolicy.Regex)
	if err != nil || re == nil {
		return nil, err
	}

	subexpNames := re.SubexpNames()
	matches := re.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return nil, nil
	}

	pipeline := buildFieldPipeline(node.TableLinePolicy.Fields)
	var results []map[string]string

	for _, match := range matches {
		row := make(map[string]string)
		for idx, name := range subexpNames {
			if idx > 0 && name != "" && idx < len(match) {
				row[name] = strings.TrimSpace(match[idx])
			}
		}

		// 跑字段级正则与算子
		for _, f := range node.TableLinePolicy.Fields {
			if f.Regex != "" {
				_, fieldRe, _, fErr := xmlcfg.CleanAndCompileRegex(f.Regex)
				if fErr == nil && fieldRe != nil {
					val := row[f.Name]
					if fMatch := fieldRe.FindStringSubmatch(val); len(fMatch) > 1 {
						row[f.Name] = strings.TrimSpace(fMatch[1])
					}
				}
			}
		}

		finalRow, ok := pipeline.ExecuteRow(row)
		if ok && len(finalRow) > 0 {
			results = append(results, finalRow)
		}
	}

	return results, nil
}

// buildFieldPipeline 根据 Field 节点集合构造有序算子管线
func buildFieldPipeline(fields []xmlcfg.FieldNode) *OperatorPipeline {
	pipeline := NewOperatorPipeline()

	for _, f := range fields {
		// ToUpper
		if f.ToUpper != nil {
			names := strings.Split(f.ToUpper.Name, ",")
			if len(names) == 0 || names[0] == "" {
				names = strings.Split(f.Name, ",")
			}
			pipeline.Add(&ToUpperOperator{Fields: names})
		}
		// ToLower
		if f.ToLower != nil {
			names := strings.Split(f.ToLower.Name, ",")
			if len(names) == 0 || names[0] == "" {
				names = strings.Split(f.Name, ",")
			}
			pipeline.Add(&ToLowerOperator{Fields: names})
		}
		// MatchAndSet
		if f.MatchAndSet != nil {
			names := strings.Split(f.MatchAndSet.Name, ",")
			if len(names) == 0 || names[0] == "" {
				names = strings.Split(f.Name, ",")
			}
			pipeline.Add(&MatchAndSetOperator{Names: names, Value: f.MatchAndSet.Value})
		}
		// ValueMapping
		if f.ValueMapping != nil {
			names := strings.Split(f.ValueMapping.Name, ",")
			if len(names) == 0 || names[0] == "" {
				names = strings.Split(f.Name, ",")
			}
			srcs := strings.Split(f.ValueMapping.SrcValue, ",")
			dsts := strings.Split(f.ValueMapping.DstValue, ",")
			m := make(map[string]string)
			for i := 0; i < len(srcs) && i < len(dsts); i++ {
				m[strings.TrimSpace(srcs[i])] = strings.TrimSpace(dsts[i])
			}
			pipeline.Add(&ValueMappingOperator{Names: names, Mapping: m})
		}
		// StrExtract
		if f.StrExtract != nil {
			fieldName := f.StrExtract.Name
			if fieldName == "" {
				fieldName = f.Name
			}
			re, rErr := regexp.Compile(f.StrExtract.Regex)
			if rErr == nil {
				gid := 0
				if f.StrExtract.GroupId != "" {
					gid = ParseOperatorOrder(f.StrExtract.GroupId, 0)
				}
				pipeline.Add(&StrExtractOperator{NameField: fieldName, Regex: re, GroupID: gid})
			}
		}
		// SplitField
		if f.SplitField != nil {
			src := f.SplitField.SrcField
			if src == "" {
				src = f.Name
			}
			dsts := strings.Split(f.SplitField.DstFields, ",")
			pipeline.Add(&SplitFieldOperator{SrcField: src, Delimiter: f.SplitField.Delimiter, DstFields: dsts})
		}
		// MergeField
		if f.MergeField != nil {
			srcs := strings.Split(f.MergeField.SrcFields, ",")
			pipeline.Add(&MergeFieldOperator{SrcFields: srcs, Delimiter: f.MergeField.Delimiter, DstField: f.MergeField.DstField})
		}
		// ReplaceAll
		if f.ReplaceAll != nil {
			names := strings.Split(f.ReplaceAll.Name, ",")
			if len(names) == 0 || names[0] == "" {
				names = strings.Split(f.Name, ",")
			}
			re, rErr := regexp.Compile(f.ReplaceAll.Regex)
			if rErr == nil {
				pipeline.Add(&ReplaceAllOperator{Names: names, Regex: re, Replacement: f.ReplaceAll.Replacement})
			}
		}
		// Assign
		for _, a := range f.Assigns {
			m := make(map[string]string)
			pairs := strings.Split(a.AssignMap, ",")
			for _, pair := range pairs {
				kv := strings.SplitN(pair, ":", 2)
				if len(kv) == 2 {
					m[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
				}
			}
			pipeline.Add(&AssignOperator{
				AssignMap:       m,
				FilterFieldName: a.FilterFieldName,
				FilterType:      a.FilterType,
				FilterValue:     a.FilterValue,
			})
		}
		// StrConcat
		if f.StrConcat != nil {
			src := f.StrConcat.SrcField
			if src == "" {
				src = f.Name
			}
			pipeline.Add(&StrConcatOperator{
				SrcField:  src,
				PrefixStr: f.StrConcat.PrefixStr,
				SuffixStr: f.StrConcat.SuffixStr,
				DstField:  f.StrConcat.DstField,
			})
		}
		// DefaultValue
		if f.DefaultValue != nil {
			names := strings.Split(f.DefaultValue.Name, ",")
			vals := strings.Split(f.DefaultValue.Value, ",")
			dm := make(map[string]string)
			for i := 0; i < len(names) && i < len(vals); i++ {
				dm[strings.TrimSpace(names[i])] = strings.TrimSpace(vals[i])
			}
			pipeline.Add(&DefaultValueOperator{Defaults: dm})
		}
	}

	return pipeline
}
