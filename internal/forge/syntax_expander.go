package forge

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// ExpandSyntaxSugar 展开语法糖
// 支持格式:
// - "1-10" -> ["1", "2", ..., "10"]
// - "vlan10-13" -> ["vlan10", "vlan11", "vlan12", "vlan13"]
// - "192.168.1.1-3" -> ["192.168.1.1", "192.168.1.2", "192.168.1.3"]
// - 支持 "-" 和 "~" 作为范围分隔符
// - 支持前导零对齐 (如 "01-03" -> ["01", "02", "03"])
func ExpandSyntaxSugar(input string) ([]string, error) {
	// 正则匹配: 前缀 + 起始数字 + 分隔符(-或~) + 结束数字 + 后缀
	// 例如: vlan10-13, 192.168.1.1-3, 1~10
	re := regexp.MustCompile(`^(.*?)(\d+)([-~])(\d+)(.*?)$`)

	matches := re.FindStringSubmatch(strings.TrimSpace(input))
	if matches == nil {
		// 不是语法糖格式，直接返回原值
		return []string{input}, nil
	}

	prefix := matches[1]
	startStr := matches[2]
	separator := matches[3] // "-" 或 "~"
	endStr := matches[4]
	suffix := matches[5]

	start, err := strconv.Atoi(startStr)
	if err != nil {
		return []string{input}, nil
	}

	end, err := strconv.Atoi(endStr)
	if err != nil {
		return []string{input}, nil
	}

	// 防呆：范围太大会卡死，最大支持 1000 级展开
	if abs(end-start) > 1000 {
		return []string{input}, nil
	}

	// 检测是否需要前导零对齐
	padLen := max(len(startStr), len(endStr))
	hasLeadingZero := strings.HasPrefix(startStr, "0") || strings.HasPrefix(endStr, "0")

	// 确定步进方向
	var step int
	if start <= end {
		step = 1
	} else {
		step = -1
	}

	// 生成展开结果
	var result []string
	for i := start; ; i += step {
		var numStr string
		if hasLeadingZero {
			numStr = strconv.Itoa(i)
			// 补零对齐
			for len(numStr) < padLen {
				numStr = "0" + numStr
			}
		} else {
			numStr = strconv.Itoa(i)
		}
		result = append(result, prefix+numStr+suffix)

		// 到达结束值时退出
		if step == 1 && i >= end {
			break
		}
		if step == -1 && i <= end {
			break
		}
	}

	_ = separator // 分隔符仅用于识别，不影响展开逻辑

	return result, nil
}

// ArithmeticSequence 等差数列推断结果
type ArithmeticSequence struct {
	IsArithmetic   bool     // 是否为等差数列
	Prefix         string   // 公共前缀
	Suffix         string   // 公共后缀
	Numbers        []int    // 解析出的数字序列
	PadLen         int      // 数字部分填充长度
	HasLeadingZero bool     // 是否有前导零
	CommonDiff     int      // 公差
	Values         []string // 原始值数组
}

// DetectArithmeticSequence 检测是否为等差数列
// 输入: 已拆分的值数组
// 输出: 等差数列检测结果
func DetectArithmeticSequence(values []string) *ArithmeticSequence {
	result := &ArithmeticSequence{
		IsArithmetic: false,
		Values:       values,
	}

	if len(values) < 2 {
		return result
	}

	re := regexp.MustCompile(`^(.*?)(\d+)(.*?)$`)

	var prefix, suffix string
	var numbers []int
	var padLen int
	var hasLeadingZero bool

	// 解析每个值
	for i, val := range values {
		matches := re.FindStringSubmatch(val)
		if matches == nil {
			// 不符合 数字+前缀+后缀 的模式，不是等差数列
			return result
		}

		p := matches[1]
		nStr := matches[2]
		s := matches[3]
		n, err := strconv.Atoi(nStr)
		if err != nil {
			return result
		}

		if i == 0 {
			prefix = p
			suffix = s
			padLen = len(nStr)
			hasLeadingZero = strings.HasPrefix(nStr, "0")
		} else {
			// 前后缀必须完全一致
			if p != prefix || s != suffix {
				return result
			}
		}

		numbers = append(numbers, n)
	}

	// 验证是否构成等差数列
	commonDiff := numbers[1] - numbers[0]
	for i := 2; i < len(numbers); i++ {
		if numbers[i]-numbers[i-1] != commonDiff {
			return result
		}
	}

	// 确认是等差数列
	result.IsArithmetic = true
	result.Prefix = prefix
	result.Suffix = suffix
	result.Numbers = numbers
	result.PadLen = padLen
	result.HasLeadingZero = hasLeadingZero
	result.CommonDiff = commonDiff

	return result
}

// InferArithmeticSequence 推断等差数列并补全到目标长度
// 输入: 已拆分的值数组和目标长度
// 输出: 补全后的值数组
func InferArithmeticSequence(values []string, targetLen int) ([]string, error) {
	if targetLen <= len(values) {
		return values, nil
	}

	// 限制最大推断长度为 1000，防止性能问题
	if targetLen > 1000 {
		targetLen = 1000
	}

	seq := DetectArithmeticSequence(values)
	if !seq.IsArithmetic {
		return values, nil
	}

	// 补全到目标长度
	result := make([]string, len(values))
	copy(result, values)

	lastNum := seq.Numbers[len(seq.Numbers)-1]
	for i := len(values); i < targetLen; i++ {
		lastNum += seq.CommonDiff
		var numStr string
		if seq.HasLeadingZero {
			numStr = strconv.Itoa(lastNum)
			// 补零对齐
			for len(numStr) < seq.PadLen {
				numStr = "0" + numStr
			}
		} else {
			numStr = strconv.Itoa(lastNum)
		}
		result = append(result, seq.Prefix+numStr+seq.Suffix)
	}

	return result, nil
}

// ParseVariableValues 解析变量值字符串
// 支持逗号和换行分隔，支持语法糖展开
func ParseVariableValues(valueString string) ([]string, error) {
	// 按逗号或换行拆分
	parts := splitByCommaOrNewline(valueString)

	var result []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// 尝试展开语法糖
		expanded, err := ExpandSyntaxSugar(part)
		if err != nil {
			// 展开失败，保留原值
			result = append(result, part)
		} else {
			result = append(result, expanded...)
		}
	}

	return result, nil
}

// splitByCommaOrNewline 按逗号或换行拆分字符串
func splitByCommaOrNewline(s string) []string {
	// 先按换行拆分
	lines := strings.Split(s, "\n")
	var result []string

	for _, line := range lines {
		// 再按逗号拆分
		parts := strings.Split(line, ",")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part != "" {
				result = append(result, part)
			}
		}
	}

	return result
}

// Variable 变量定义
type Variable struct {
	Name        string   // 变量名，如 "[A]", "[B]"
	ValueString string   // 原始值字符串
	Values      []string // 解析后的值数组
}

// ParseVariables 解析变量列表并展开语法糖
func ParseVariables(variables []Variable) ([]Variable, error) {
	result := make([]Variable, len(variables))

	// 先解析所有变量的值
	for i, v := range variables {
		values, err := ParseVariableValues(v.ValueString)
		if err != nil {
			return nil, err
		}
		result[i] = Variable{
			Name:        v.Name,
			ValueString: v.ValueString,
			Values:      values,
		}
	}

	// 找出最大的值数量作为目标长度
	maxLen := 0
	for _, v := range result {
		if len(v.Values) > maxLen {
			maxLen = len(v.Values)
		}
	}

	// 对每个变量尝试等差数列补全
	for i, v := range result {
		if len(v.Values) >= 2 && len(v.Values) < maxLen {
			// 尝试推断等差数列并补全
			completed, err := InferArithmeticSequence(v.Values, maxLen)
			if err == nil {
				result[i].Values = completed
			}
		}
	}

	return result, nil
}

// SortVariablesByLength 按变量名长度降序排列
// 防止短变量优先匹配导致覆盖（如 [A] 误替换 [AA] 内部的 [A]）
func SortVariablesByLength(variables []Variable) []Variable {
	sorted := make([]Variable, len(variables))
	copy(sorted, variables)

	sort.Slice(sorted, func(i, j int) bool {
		return len(sorted[i].Name) > len(sorted[j].Name)
	})

	return sorted
}

// abs 返回绝对值
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// max 返回最大值
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
