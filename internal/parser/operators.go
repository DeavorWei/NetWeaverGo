package parser

import (
	"regexp"
	"strconv"
	"strings"
)

// OperatorContext 算子执行上下文
type OperatorContext struct {
	Row     map[string]string
	Dropped bool // 若被 Filter 标记为丢弃，则该行不输出
}

// FieldOperator 字段算子统一接口
type FieldOperator interface {
	Name() string
	Execute(ctx *OperatorContext) error
}

// ----------------------------------------------------------------------------
// 1. ToUpper 算子
// ----------------------------------------------------------------------------
type ToUpperOperator struct {
	Fields []string
}

func (o *ToUpperOperator) Name() string { return "ToUpper" }
func (o *ToUpperOperator) Execute(ctx *OperatorContext) error {
	for _, f := range o.Fields {
		f = strings.TrimSpace(f)
		if val, ok := ctx.Row[f]; ok {
			ctx.Row[f] = strings.ToUpper(val)
		}
	}
	return nil
}

// ----------------------------------------------------------------------------
// 2. ToLower 算子
// ----------------------------------------------------------------------------
type ToLowerOperator struct {
	Fields []string
}

func (o *ToLowerOperator) Name() string { return "ToLower" }
func (o *ToLowerOperator) Execute(ctx *OperatorContext) error {
	for _, f := range o.Fields {
		f = strings.TrimSpace(f)
		if val, ok := ctx.Row[f]; ok {
			ctx.Row[f] = strings.ToLower(val)
		}
	}
	return nil
}

// ----------------------------------------------------------------------------
// 3. MatchAndSet 算子
// ----------------------------------------------------------------------------
type MatchAndSetOperator struct {
	Names []string
	Value string
}

func (o *MatchAndSetOperator) Name() string { return "MatchAndSet" }
func (o *MatchAndSetOperator) Execute(ctx *OperatorContext) error {
	for _, name := range o.Names {
		name = strings.TrimSpace(name)
		if name != "" {
			ctx.Row[name] = o.Value
		}
	}
	return nil
}

// ----------------------------------------------------------------------------
// 4. ValueMapping 算子
// ----------------------------------------------------------------------------
type ValueMappingOperator struct {
	Names   []string
	Mapping map[string]string
}

func (o *ValueMappingOperator) Name() string { return "ValueMapping" }
func (o *ValueMappingOperator) Execute(ctx *OperatorContext) error {
	for _, name := range o.Names {
		name = strings.TrimSpace(name)
		if val, ok := ctx.Row[name]; ok {
			if dst, matched := o.Mapping[val]; matched {
				ctx.Row[name] = dst
			} else if dst, matchedCI := o.Mapping[strings.ToLower(val)]; matchedCI {
				ctx.Row[name] = dst
			}
		}
	}
	return nil
}

// ----------------------------------------------------------------------------
// 5. StrExtract 算子
// ----------------------------------------------------------------------------
type StrExtractOperator struct {
	NameField string
	Regex     *regexp.Regexp
	GroupID   int
}

func (o *StrExtractOperator) Name() string { return "StrExtract" }
func (o *StrExtractOperator) Execute(ctx *OperatorContext) error {
	if o.Regex == nil {
		return nil
	}
	val, ok := ctx.Row[o.NameField]
	if !ok {
		return nil
	}
	sub := o.Regex.FindStringSubmatch(val)
	if len(sub) > o.GroupID {
		ctx.Row[o.NameField] = sub[o.GroupID]
	}
	return nil
}

// ----------------------------------------------------------------------------
// 6. DefaultValue 算子
// ----------------------------------------------------------------------------
type DefaultValueOperator struct {
	Defaults map[string]string
}

func (o *DefaultValueOperator) Name() string { return "DefaultValue" }
func (o *DefaultValueOperator) Execute(ctx *OperatorContext) error {
	for k, v := range o.Defaults {
		if cur, ok := ctx.Row[k]; !ok || strings.TrimSpace(cur) == "" {
			ctx.Row[k] = v
		}
	}
	return nil
}

// ----------------------------------------------------------------------------
// 7. Filter 算子
// ----------------------------------------------------------------------------
type FilterOperator struct {
	FieldName  string
	FilterType string // EQUALS, NOT_EQUALS, CONTAINS, NOT_CONTAINS, NULL, NOT_NULL, IN
	Value      string
}

func (o *FilterOperator) Name() string { return "Filter" }
func (o *FilterOperator) Execute(ctx *OperatorContext) error {
	val, exists := ctx.Row[o.FieldName]
	ft := strings.ToUpper(strings.TrimSpace(o.FilterType))

	drop := false
	switch ft {
	case "EQUALS":
		if val != o.Value {
			drop = true
		}
	case "NOT_EQUALS":
		if val == o.Value {
			drop = true
		}
	case "CONTAINS":
		if !strings.Contains(val, o.Value) {
			drop = true
		}
	case "NOT_CONTAINS":
		if strings.Contains(val, o.Value) {
			drop = true
		}
	case "NULL":
		if exists && strings.TrimSpace(val) != "" {
			drop = true
		}
	case "NOT_NULL":
		if !exists || strings.TrimSpace(val) == "" {
			drop = true
		}
	case "IN":
		parts := strings.Split(o.Value, ",")
		found := false
		for _, p := range parts {
			if strings.TrimSpace(p) == val {
				found = true
				break
			}
		}
		if !found {
			drop = true
		}
	}

	if drop {
		ctx.Dropped = true
	}
	return nil
}

// ----------------------------------------------------------------------------
// 8. SplitField 算子
// ----------------------------------------------------------------------------
type SplitFieldOperator struct {
	SrcField  string
	Delimiter string
	DstFields []string
}

func (o *SplitFieldOperator) Name() string { return "SplitField" }
func (o *SplitFieldOperator) Execute(ctx *OperatorContext) error {
	val, ok := ctx.Row[o.SrcField]
	if !ok || !strings.Contains(val, o.Delimiter) {
		return nil
	}
	parts := strings.Split(val, o.Delimiter)
	for i, dst := range o.DstFields {
		dst = strings.TrimSpace(dst)
		if dst == "" {
			continue
		}
		if i < len(parts) {
			ctx.Row[dst] = strings.TrimSpace(parts[i])
		}
	}
	return nil
}

// ----------------------------------------------------------------------------
// 9. MergeField 算子
// ----------------------------------------------------------------------------
type MergeFieldOperator struct {
	SrcFields []string
	Delimiter string
	DstField  string
}

func (o *MergeFieldOperator) Name() string { return "MergeField" }
func (o *MergeFieldOperator) Execute(ctx *OperatorContext) error {
	var vals []string
	for _, src := range o.SrcFields {
		src = strings.TrimSpace(src)
		if v, ok := ctx.Row[src]; ok {
			vals = append(vals, v)
		}
	}
	ctx.Row[o.DstField] = strings.Join(vals, o.Delimiter)
	return nil
}

// ----------------------------------------------------------------------------
// 10. ReplaceAll 算子
// ----------------------------------------------------------------------------
type ReplaceAllOperator struct {
	Names       []string
	Regex       *regexp.Regexp
	Replacement string
}

func (o *ReplaceAllOperator) Name() string { return "ReplaceAll" }
func (o *ReplaceAllOperator) Execute(ctx *OperatorContext) error {
	if o.Regex == nil {
		return nil
	}
	for _, name := range o.Names {
		name = strings.TrimSpace(name)
		if val, ok := ctx.Row[name]; ok {
			ctx.Row[name] = o.Regex.ReplaceAllString(val, o.Replacement)
		}
	}
	return nil
}

// ----------------------------------------------------------------------------
// 11. Assign 算子
// ----------------------------------------------------------------------------
type AssignOperator struct {
	AssignMap       map[string]string // dstField -> templateValue
	FilterFieldName string
	FilterType      string
	FilterValue     string
}

func (o *AssignOperator) Name() string { return "Assign" }
func (o *AssignOperator) Execute(ctx *OperatorContext) error {
	// 若存在过滤条件，则需先判定
	if o.FilterFieldName != "" && o.FilterType != "" {
		val := ctx.Row[o.FilterFieldName]
		ft := strings.ToUpper(strings.TrimSpace(o.FilterType))
		pass := false
		switch ft {
		case "EQUALS":
			pass = (val == o.FilterValue)
		case "NOT_EQUALS":
			pass = (val != o.FilterValue)
		case "CONTAINS":
			pass = strings.Contains(val, o.FilterValue)
		case "START_WITH":
			pass = strings.HasPrefix(val, o.FilterValue)
		case "END_WITH":
			pass = strings.HasSuffix(val, o.FilterValue)
		case "NULL":
			pass = (strings.TrimSpace(val) == "")
		case "NOT_NULL":
			pass = (strings.TrimSpace(val) != "")
		}
		if !pass {
			return nil
		}
	}

	// 执行赋值，支持 #{fieldName} 引用
	for dst, tmpl := range o.AssignMap {
		finalVal := tmpl
		for k, v := range ctx.Row {
			placeholder := "#{" + k + "}"
			if strings.Contains(finalVal, placeholder) {
				finalVal = strings.ReplaceAll(finalVal, placeholder, v)
			}
		}
		ctx.Row[dst] = finalVal
	}
	return nil
}

// ----------------------------------------------------------------------------
// 12. RenameField 算子
// ----------------------------------------------------------------------------
type RenameFieldOperator struct {
	FieldMap map[string]string // old -> new
}

func (o *RenameFieldOperator) Name() string { return "RenameField" }
func (o *RenameFieldOperator) Execute(ctx *OperatorContext) error {
	for oldK, newK := range o.FieldMap {
		if val, ok := ctx.Row[oldK]; ok {
			delete(ctx.Row, oldK)
			ctx.Row[newK] = val
		}
	}
	return nil
}

// ----------------------------------------------------------------------------
// 13. StrConcat 算子
// ----------------------------------------------------------------------------
type StrConcatOperator struct {
	SrcField  string
	PrefixStr string
	SuffixStr string
	DstField  string
}

func (o *StrConcatOperator) Name() string { return "StrConcat" }
func (o *StrConcatOperator) Execute(ctx *OperatorContext) error {
	val := ctx.Row[o.SrcField]
	ctx.Row[o.DstField] = o.PrefixStr + val + o.SuffixStr
	return nil
}

// ----------------------------------------------------------------------------
// 有序算子管线 (OperatorPipeline)
// ----------------------------------------------------------------------------
type OperatorPipeline struct {
	operators []FieldOperator
}

func NewOperatorPipeline(ops ...FieldOperator) *OperatorPipeline {
	return &OperatorPipeline{operators: ops}
}

func (p *OperatorPipeline) Add(op FieldOperator) {
	if op != nil {
		p.operators = append(p.operators, op)
	}
}

// ExecuteRow 对单行属性执行算子管线处理
func (p *OperatorPipeline) ExecuteRow(row map[string]string) (map[string]string, bool) {
	ctx := &OperatorContext{
		Row:     row,
		Dropped: false,
	}

	for _, op := range p.operators {
		_ = op.Execute(ctx)
		if ctx.Dropped {
			return nil, false
		}
	}

	return ctx.Row, true
}

// ParseOperatorOrder 将字符串 order 转换为 int 权重
func ParseOperatorOrder(ordStr string, defaultOrder int) int {
	if ordStr == "" {
		return defaultOrder
	}
	if v, err := strconv.Atoi(strings.TrimSpace(ordStr)); err == nil {
		return v
	}
	return defaultOrder
}
