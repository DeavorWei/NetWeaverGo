package forge

import (
	"strings"
)

// BuildRequest 构建请求
type BuildRequest struct {
	Template  string     `json:"template"`  // 模板文本
	Variables []VarInput `json:"variables"` // 变量列表
}

// VarInput 变量输入
type VarInput struct {
	Name        string `json:"name"`        // 变量名，如 "[A]", "[B]"
	ValueString string `json:"valueString"` // 值字符串（逗号/换行分隔）
}

// BuildResult 构建结果
type BuildResult struct {
	Blocks   []string `json:"blocks"`   // 生成的配置块
	Total    int      `json:"total"`    // 总数量
	Warnings []string `json:"warnings"` // 警告信息
}

// ConfigBuilder 配置构建器
type ConfigBuilder struct{}

// NewConfigBuilder 创建配置构建器
func NewConfigBuilder() *ConfigBuilder {
	return &ConfigBuilder{}
}

// Build 执行配置构建
// 核心逻辑：
// 1. 解析变量值（逗号/换行分隔）
// 2. 展开语法糖（1-10 -> 1,2,3,...,10）
// 3. 推断等差数列补全
// 4. 精确变量替换
// 5. 返回构建结果
func (b *ConfigBuilder) Build(req *BuildRequest) (*BuildResult, error) {
	result := &BuildResult{
		Blocks:   []string{},
		Warnings: []string{},
	}

	// 1. 转换变量输入格式
	variables := make([]Variable, len(req.Variables))
	for i, v := range req.Variables {
		variables[i] = Variable{
			Name:        v.Name,
			ValueString: v.ValueString,
		}
	}

	// 2. 解析变量并展开语法糖
	parsedVars, err := ParseVariables(variables)
	if err != nil {
		return nil, err
	}

	// 3. 过滤掉完全没有输入值的变量
	activeVars := make([]Variable, 0)
	for _, v := range parsedVars {
		if len(v.Values) > 0 {
			activeVars = append(activeVars, v)
		}
	}

	// 如果没有活跃变量或模板为空，返回空结果
	if len(activeVars) == 0 || strings.TrimSpace(req.Template) == "" {
		return result, nil
	}

	// 4. 确定生成批次（取最大值数量）
	maxLen := 0
	for _, v := range activeVars {
		if len(v.Values) > maxLen {
			maxLen = len(v.Values)
		}
	}

	// 5. 按变量名长度降序排列，防止短变量优先匹配导致覆盖
	sortedVars := SortVariablesByLength(activeVars)

	// 6. 精确替换循环
	blocks := make([]string, 0, maxLen)
	for i := 0; i < maxLen; i++ {
		currentBlock := req.Template

		for _, v := range sortedVars {
			// 如果当前变量的值数量不足，取最后一个值循环补齐
			valIndex := i
			if valIndex >= len(v.Values) {
				valIndex = len(v.Values) - 1
			}
			val := v.Values[valIndex]

			// 精确替换，直接匹配带括号的变量名进行替换
			currentBlock = strings.ReplaceAll(currentBlock, v.Name, val)
		}

		// 清除头部及尾部可能导致多余空行的空白字符
		blocks = append(blocks, strings.TrimSpace(currentBlock))
	}

	result.Blocks = blocks
	result.Total = len(blocks)

	return result, nil
}

// ExpandRequest 语法糖展开请求
type ExpandRequest struct {
	ValueString string `json:"valueString"` // 值字符串
	MaxLen      int    `json:"maxLen"`      // 目标最大长度（用于等差数列补全）
}

// ExpandResult 语法糖展开结果
type ExpandResult struct {
	Values      []string `json:"values"`      // 展开后的值数组
	OriginalLen int      `json:"originalLen"` // 原始长度
	ExpandedLen int      `json:"expandedLen"` // 展开后长度
	HasExpanded bool     `json:"hasExpanded"` // 是否发生了展开
	HasInferred bool     `json:"hasInferred"` // 是否进行了等差推断补全
	Warnings    []string `json:"warnings"`    // 警告信息
}

// ExpandValues 展开变量值（单独调用）
// 用于前端实时预览展开结果
func (b *ConfigBuilder) ExpandValues(req *ExpandRequest) (*ExpandResult, error) {
	result := &ExpandResult{
		Warnings: []string{},
	}

	// 先计算原始值的数量（展开前），用于判断是否发生展开
	originalParts := splitByCommaOrNewline(req.ValueString)
	result.OriginalLen = len(originalParts)

	// 解析并展开语法糖
	values, err := ParseVariableValues(req.ValueString)
	if err != nil {
		return nil, err
	}

	result.HasExpanded = len(values) > result.OriginalLen

	// 如果指定了目标长度且当前值数量不足，尝试等差数列补全
	if req.MaxLen > 0 && len(values) < req.MaxLen && len(values) >= 2 {
		completed, err := InferArithmeticSequence(values, req.MaxLen)
		if err == nil && len(completed) > len(values) {
			values = completed
			result.HasInferred = true
		}
	}

	result.Values = values
	result.ExpandedLen = len(values)

	return result, nil
}

// PreviewRequest 预览请求
type PreviewRequest struct {
	Template     string   `json:"template"`     // 模板文本
	VariableName string   `json:"variableName"` // 变量名
	Values       []string `json:"values"`       // 变量值
	Index        int      `json:"index"`        // 预览索引
}

// PreviewResult 预览结果
type PreviewResult struct {
	Block string `json:"block"` // 预览的配置块
}

// PreviewBlock 预览单个配置块
func (b *ConfigBuilder) PreviewBlock(req *PreviewRequest) (*PreviewResult, error) {
	block := req.Template

	// 精确替换
	valIndex := req.Index
	if valIndex >= len(req.Values) {
		valIndex = len(req.Values) - 1
	}
	if valIndex < 0 {
		valIndex = 0
	}

	if len(req.Values) > 0 {
		val := req.Values[valIndex]
		block = strings.ReplaceAll(block, req.VariableName, val)
	}

	return &PreviewResult{
		Block: strings.TrimSpace(block),
	}, nil
}
