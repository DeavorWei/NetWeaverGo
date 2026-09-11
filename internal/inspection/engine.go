package inspection

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/NetWeaverGo/core/internal/models"
)

// EvaluateInput 评估引擎输入
type EvaluateInput struct {
	RunID      string
	DeviceIP   string
	Item       *models.InspectionItem
	RawEcho    string
	ParsedRows []map[string]interface{}
}

// EvaluateItem 判定单台设备针对单个巡检项的执行结论
func EvaluateItem(input *EvaluateInput) models.InspectionResult {
	if input == nil || input.Item == nil {
		runID := ""
		deviceIP := ""
		if input != nil {
			runID = input.RunID
			deviceIP = input.DeviceIP
		}
		return models.InspectionResult{
			RunID:    runID,
			DeviceIP: deviceIP,
			Status:   string(ResultExcept),
			Problem:  "评估输入或检查项定义为空 (item is nil)",
			Advice:   "请检查巡检项配置是否完整有效",
		}
	}

	item := input.Item
	result := models.InspectionResult{
		RunID:       input.RunID,
		DeviceIP:    input.DeviceIP,
		ItemCode:    item.Code,
		ItemName:    item.Name,
		Category:    item.Category,
		Status:      string(ResultUntest),
		Severity:    item.Severity,
		Problem:     item.Problem,
		Advice:      item.Advice,
		ActualValue: "",
	}

	// 1. 反序列化外置阈值定义
	var thresholds []models.Threshold
	if strings.TrimSpace(item.ThresholdsJSON) != "" {
		_ = json.Unmarshal([]byte(item.ThresholdsJSON), &thresholds)
	}

	// 2. 如果既无解析行，也无原始回显，返回异常
	if len(input.ParsedRows) == 0 && strings.TrimSpace(input.RawEcho) == "" {
		result.Status = string(ResultExcept)
		result.Problem = fmt.Sprintf("未获取到命令 [%s] 的回显数据", item.CommandKey)
		result.Advice = "请检查设备连接或账号特权视图"
		return result
	}

	evidenceLines := extractEvidence(input.RawEcho, item.Field, thresholds)

	checkType := strings.ToLower(strings.TrimSpace(item.CheckType))
	switch checkType {
	case "threshold", "bound", "range", "numeric":
		evaluateThreshold(item, thresholds, input, &result)
	case "must_contain":
		evaluateMustContain(item, thresholds, input, &result)
	case "must_not_contain":
		evaluateMustNotContain(item, thresholds, input, &result)
	case "regex":
		evaluateRegex(item, thresholds, input, &result)
	case "equals":
		evaluateEquals(item, thresholds, input, &result)
	default:
		// 默认数值阈值或包含判定
		if len(thresholds) > 0 && (thresholds[0].MinValue != "" || thresholds[0].MaxValue != "") {
			evaluateThreshold(item, thresholds, input, &result)
		} else {
			evaluateMustContain(item, thresholds, input, &result)
		}
	}

	// 序列化举证证据
	if len(evidenceLines) > 0 {
		evidenceJSON, _ := json.Marshal(evidenceLines)
		result.EvidenceJSON = string(evidenceJSON)
	}

	return result
}

func evaluateThreshold(item *models.InspectionItem, thresholds []models.Threshold, input *EvaluateInput, result *models.InspectionResult) {
	hasValidThreshold := false
	for _, t := range thresholds {
		if strings.TrimSpace(t.MinValue) != "" || strings.TrimSpace(t.MaxValue) != "" || strings.TrimSpace(t.DefaultValue) != "" {
			hasValidThreshold = true
			break
		}
	}
	if !hasValidThreshold {
		result.Status = string(ResultExcept)
		result.Severity = string(SeverityMinor)
		result.Problem = fmt.Sprintf("[%s] 阈值规则未配置有效区间", item.Name)
		result.Advice = "请在巡检项设置中配置有效的阈值规则 (Thresholds)"
		return
	}

	allVals := extractAllFieldValues(item.Field, input)
	if len(allVals) > 0 {
		// 多行表格场景：逐行检测数值阈值
		for rowIdx, valStr := range allVals {
			cleanVal := sanitizeNumeric(valStr)
			valFloat, err := strconv.ParseFloat(cleanVal, 64)
			if err != nil {
				continue
			}

			for _, th := range thresholds {
				minF, hasMin := parseOptionalFloat(th.MinValue)
				maxF, hasMax := parseOptionalFloat(th.MaxValue)
				rangeType := strings.ToLower(strings.TrimSpace(th.RangeType))
				if rangeType == "" {
					rangeType = "bound"
				}

				passed := true
				hitRule := ""

				switch rangeType {
				case "bound":
					if hasMin && valFloat < minF {
						passed = false
						hitRule = fmt.Sprintf("低于下限 %.2f", minF)
					}
					if hasMax && valFloat > maxF {
						passed = false
						hitRule = fmt.Sprintf("超出上限 %.2f", maxF)
					}
				case "outside":
					if (hasMin && valFloat >= minF) && (hasMax && valFloat <= maxF) {
						passed = false
						hitRule = fmt.Sprintf("落在禁止区间 [%.2f, %.2f]", minF, maxF)
					}
				case "lte":
					if hasMax && valFloat > maxF {
						passed = false
						hitRule = fmt.Sprintf("大于阈值 %.2f", maxF)
					}
				case "gte":
					if hasMin && valFloat < minF {
						passed = false
						hitRule = fmt.Sprintf("小于阈值 %.2f", minF)
					}
				case "equals":
					if defF, hasDef := parseOptionalFloat(th.DefaultValue); hasDef && valFloat != defF {
						passed = false
						hitRule = fmt.Sprintf("不等于基准值 %.2f", defF)
					}
				}

				if !passed {
					result.ActualValue = fmt.Sprintf("第 %d 行: %s", rowIdx+1, valStr)
					result.ThresholdHit = hitRule
					code, sev := MapSeverityToResult(item.Severity, false)
					result.Status = string(code)
					result.Severity = string(sev)
					result.Problem = item.Problem
					if result.Problem == "" {
						result.Problem = fmt.Sprintf("[%s] 第 %d 行数值 (%s) %s", item.Name, rowIdx+1, valStr, hitRule)
					}
					result.Advice = item.Advice
					return
				}
			}
		}

		result.ActualValue = truncateStr(strings.Join(allVals, ", "), 64)
		code, sev := MapSeverityToResult(item.Severity, true)
		result.Status = string(code)
		result.Severity = string(sev)
		result.Problem = ""
		result.Advice = ""
		return
	}

	// 回退机制：从原始 CLI 回显中抓取数值
	captured, ok := captureNumericFromEcho(item.Field, input.RawEcho)
	if !ok {
		result.Status = string(ResultExcept)
		result.Problem = fmt.Sprintf("无法解析目标字段 [%s] 的数值", item.Field)
		result.Advice = "请检查解析模板字段映射是否匹配或设备回显是否包含该指标"
		return
	}

	valFloat := captured
	result.ActualValue = fmt.Sprintf("%.2f", captured)

	for _, th := range thresholds {
		minF, hasMin := parseOptionalFloat(th.MinValue)
		maxF, hasMax := parseOptionalFloat(th.MaxValue)
		rangeType := strings.ToLower(strings.TrimSpace(th.RangeType))
		if rangeType == "" {
			rangeType = "bound"
		}

		passed := true
		hitRule := ""

		switch rangeType {
		case "bound":
			if hasMin && valFloat < minF {
				passed = false
				hitRule = fmt.Sprintf("低于下限 %.2f", minF)
			}
			if hasMax && valFloat > maxF {
				passed = false
				hitRule = fmt.Sprintf("超出上限 %.2f", maxF)
			}
		case "outside":
			if (hasMin && valFloat >= minF) && (hasMax && valFloat <= maxF) {
				passed = false
				hitRule = fmt.Sprintf("落在禁止区间 [%.2f, %.2f]", minF, maxF)
			}
		case "lte":
			if hasMax && valFloat > maxF {
				passed = false
				hitRule = fmt.Sprintf("大于阈值 %.2f", maxF)
			}
		case "gte":
			if hasMin && valFloat < minF {
				passed = false
				hitRule = fmt.Sprintf("小于阈值 %.2f", minF)
			}
		case "equals":
			if defF, hasDef := parseOptionalFloat(th.DefaultValue); hasDef && valFloat != defF {
				passed = false
				hitRule = fmt.Sprintf("不等于基准值 %.2f", defF)
			}
		}

		result.ThresholdHit = hitRule
		if !passed {
			code, sev := MapSeverityToResult(item.Severity, false)
			result.Status = string(code)
			result.Severity = string(sev)
			result.Problem = item.Problem
			if result.Problem == "" {
				result.Problem = fmt.Sprintf("[%s] 当前值 (%s) %s", item.Name, result.ActualValue, hitRule)
			}
			result.Advice = item.Advice
			return
		}
	}

	code, sev := MapSeverityToResult(item.Severity, true)
	result.Status = string(code)
	result.Severity = string(sev)
	result.Problem = ""
	result.Advice = ""
}

func evaluateMustContain(item *models.InspectionItem, thresholds []models.Threshold, input *EvaluateInput, result *models.InspectionResult) {
	target := ""
	if len(thresholds) > 0 {
		target = thresholds[0].DefaultValue
	}
	if target == "" && item.Field != "" {
		target = item.Field
	}
	result.ThresholdHit = fmt.Sprintf("必须包含: '%s'", target)

	allVals := extractAllFieldValues(item.Field, input)
	if len(allVals) > 0 {
		// 多行场景：逐行核对必须包含目标关键词
		for idx, v := range allVals {
			if target != "" && !strings.Contains(strings.ToLower(v), strings.ToLower(target)) {
				result.ActualValue = fmt.Sprintf("第 %d 行: %s", idx+1, v)
				code, sev := MapSeverityToResult(item.Severity, false)
				result.Status = string(code)
				result.Severity = string(sev)
				result.Problem = item.Problem
				if result.Problem == "" {
					result.Problem = fmt.Sprintf("[%s] 第 %d 行未包含指定关键字 '%s' (实际: '%s')", item.Name, idx+1, target, v)
				}
				result.Advice = item.Advice
				return
			}
		}
		result.ActualValue = truncateStr(strings.Join(allVals, ", "), 64)
	} else {
		valStr := input.RawEcho
		result.ActualValue = truncateStr(valStr, 64)
		if target != "" && !strings.Contains(strings.ToLower(valStr), strings.ToLower(target)) {
			code, sev := MapSeverityToResult(item.Severity, false)
			result.Status = string(code)
			result.Severity = string(sev)
			result.Problem = item.Problem
			if result.Problem == "" {
				result.Problem = fmt.Sprintf("[%s] 未包含指定关键字: '%s'", item.Name, target)
			}
			result.Advice = item.Advice
			return
		}
	}

	code, sev := MapSeverityToResult(item.Severity, true)
	result.Status = string(code)
	result.Severity = string(sev)
}

func evaluateMustNotContain(item *models.InspectionItem, thresholds []models.Threshold, input *EvaluateInput, result *models.InspectionResult) {
	target := ""
	if len(thresholds) > 0 {
		target = thresholds[0].DefaultValue
	}
	result.ThresholdHit = fmt.Sprintf("禁止包含: '%s'", target)

	allVals := extractAllFieldValues(item.Field, input)
	if len(allVals) > 0 {
		// 多行表格场景：逐行排查禁止包含的异常状态
		for idx, v := range allVals {
			if target != "" && strings.Contains(strings.ToLower(v), strings.ToLower(target)) {
				result.ActualValue = fmt.Sprintf("第 %d 行: %s", idx+1, v)
				code, sev := MapSeverityToResult(item.Severity, false)
				result.Status = string(code)
				result.Severity = string(sev)
				result.Problem = item.Problem
				if result.Problem == "" {
					result.Problem = fmt.Sprintf("[%s] 第 %d 行包含禁止关键字 '%s' (实际: '%s')", item.Name, idx+1, target, v)
				}
				result.Advice = item.Advice
				return
			}
		}
		result.ActualValue = truncateStr(strings.Join(allVals, ", "), 64)
	} else {
		valStr := input.RawEcho
		result.ActualValue = truncateStr(valStr, 64)
		if target != "" && strings.Contains(strings.ToLower(valStr), strings.ToLower(target)) {
			code, sev := MapSeverityToResult(item.Severity, false)
			result.Status = string(code)
			result.Severity = string(sev)
			result.Problem = item.Problem
			if result.Problem == "" {
				result.Problem = fmt.Sprintf("[%s] 包含禁止关键字: '%s'", item.Name, target)
			}
			result.Advice = item.Advice
			return
		}
	}

	code, sev := MapSeverityToResult(item.Severity, true)
	result.Status = string(code)
	result.Severity = string(sev)
}

func evaluateRegex(item *models.InspectionItem, thresholds []models.Threshold, input *EvaluateInput, result *models.InspectionResult) {
	pattern := ""
	if len(thresholds) > 0 {
		pattern = thresholds[0].DefaultValue
	}
	result.ThresholdHit = fmt.Sprintf("正则匹配: %s", pattern)

	re, err := regexp.Compile(pattern)
	if err != nil {
		result.Status = string(ResultExcept)
		result.Problem = fmt.Sprintf("无效的正则表达式: %v", err)
		return
	}

	allVals := extractAllFieldValues(item.Field, input)
	if len(allVals) > 0 {
		for idx, v := range allVals {
			if !re.MatchString(v) {
				result.ActualValue = fmt.Sprintf("第 %d 行: %s", idx+1, v)
				code, sev := MapSeverityToResult(item.Severity, false)
				result.Status = string(code)
				result.Severity = string(sev)
				result.Problem = item.Problem
				if result.Problem == "" {
					result.Problem = fmt.Sprintf("[%s] 第 %d 行未通过正则校验 (实际: '%s')", item.Name, idx+1, v)
				}
				result.Advice = item.Advice
				return
			}
		}
		result.ActualValue = truncateStr(strings.Join(allVals, ", "), 64)
	} else {
		valStr := input.RawEcho
		result.ActualValue = truncateStr(valStr, 64)
		if !re.MatchString(valStr) {
			code, sev := MapSeverityToResult(item.Severity, false)
			result.Status = string(code)
			result.Severity = string(sev)
			result.Problem = item.Problem
			if result.Problem == "" {
				result.Problem = fmt.Sprintf("[%s] 正则表达式校验未通过", item.Name)
			}
			result.Advice = item.Advice
			return
		}
	}

	code, sev := MapSeverityToResult(item.Severity, true)
	result.Status = string(code)
	result.Severity = string(sev)
}

func evaluateEquals(item *models.InspectionItem, thresholds []models.Threshold, input *EvaluateInput, result *models.InspectionResult) {
	expected := ""
	if len(thresholds) > 0 {
		expected = thresholds[0].DefaultValue
	}
	result.ThresholdHit = fmt.Sprintf("等于: %s", expected)

	allVals := extractAllFieldValues(item.Field, input)
	if len(allVals) > 0 {
		for idx, v := range allVals {
			if !strings.EqualFold(strings.TrimSpace(v), strings.TrimSpace(expected)) {
				result.ActualValue = fmt.Sprintf("第 %d 行: %s", idx+1, v)
				code, sev := MapSeverityToResult(item.Severity, false)
				result.Status = string(code)
				result.Severity = string(sev)
				result.Problem = item.Problem
				if result.Problem == "" {
					result.Problem = fmt.Sprintf("[%s] 第 %d 期望为 '%s'，实际为 '%s'", item.Name, idx+1, expected, v)
				}
				result.Advice = item.Advice
				return
			}
		}
		result.ActualValue = truncateStr(strings.Join(allVals, ", "), 64)
	} else {
		valStr := extractFieldValue(item.Field, input)
		result.ActualValue = valStr
		if !strings.EqualFold(strings.TrimSpace(valStr), strings.TrimSpace(expected)) {
			code, sev := MapSeverityToResult(item.Severity, false)
			result.Status = string(code)
			result.Severity = string(sev)
			result.Problem = item.Problem
			if result.Problem == "" {
				result.Problem = fmt.Sprintf("[%s] 期望值为 '%s'，实际为 '%s'", item.Name, expected, valStr)
			}
			result.Advice = item.Advice
			return
		}
	}

	code, sev := MapSeverityToResult(item.Severity, true)
	result.Status = string(code)
	result.Severity = string(sev)
}

// extractAllFieldValues 从所有解析行中提取指定字段的字符串切片
func extractAllFieldValues(field string, input *EvaluateInput) []string {
	if len(input.ParsedRows) == 0 || field == "" {
		return nil
	}
	var values []string
	for _, row := range input.ParsedRows {
		if val, ok := row[field]; ok && val != nil {
			values = append(values, fmt.Sprintf("%v", val))
			continue
		}
		// 忽略大小写匹配
		matched := false
		for k, val := range row {
			if strings.EqualFold(k, field) && val != nil {
				values = append(values, fmt.Sprintf("%v", val))
				matched = true
				break
			}
		}
		if !matched {
			// 若某行无此字段，不写入
		}
	}
	return values
}

// extractFieldValue 提取首个匹配字段的取值（兼容单行场景）
func extractFieldValue(field string, input *EvaluateInput) string {
	vals := extractAllFieldValues(field, input)
	if len(vals) > 0 {
		return vals[0]
	}
	return ""
}

func captureNumericFromEcho(field, echo string) (float64, bool) {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(fmt.Sprintf(`(?i)%s[^\d]*(\d+(?:\.\d+)?)`, regexp.QuoteMeta(field))),
		regexp.MustCompile(`(?i)(?:usage|utilization|rate)[^\d]*(\d+(?:\.\d+)?)\s*%`),
		regexp.MustCompile(`(\d+(?:\.\d+)?)\s*%`),
	}
	for _, p := range patterns {
		if m := p.FindStringSubmatch(echo); len(m) > 1 {
			if f, err := strconv.ParseFloat(m[1], 64); err == nil {
				return f, true
			}
		}
	}
	return 0, false
}

// extractEvidence 提取 CLI 原始回显中的命中行与上下文证据（前后各保留 1 行）
func extractEvidence(rawEcho, field string, thresholds []models.Threshold) []string {
	if strings.TrimSpace(rawEcho) == "" {
		return []string{}
	}

	var keywords []string
	if field != "" {
		keywords = append(keywords, strings.ToLower(field))
	}
	for _, th := range thresholds {
		if th.Name != "" {
			keywords = append(keywords, strings.ToLower(th.Name))
		}
		if th.DefaultValue != "" {
			keywords = append(keywords, strings.ToLower(th.DefaultValue))
		}
	}

	lines := strings.Split(rawEcho, "\n")
	evidence := make([]string, 0, 9)
	seen := make(map[int]bool)

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		matched := false
		for _, kw := range keywords {
			if kw != "" && strings.Contains(lower, kw) {
				matched = true
				break
			}
		}
		if matched {
			// 前后各保留 1 行上下文
			start := i - 1
			if start < 0 {
				start = 0
			}
			end := i + 1
			if end >= len(lines) {
				end = len(lines) - 1
			}
			for idx := start; idx <= end; idx++ {
				if !seen[idx] {
					seen[idx] = true
					tCtx := strings.TrimSpace(lines[idx])
					if tCtx != "" {
						evidence = append(evidence, tCtx)
					}
				}
			}
			if len(evidence) >= 9 {
				break
			}
		}
	}

	// 若未匹配到特定关键字，提取首行及前 3 行有效内容
	if len(evidence) == 0 {
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" {
				evidence = append(evidence, trimmed)
				if len(evidence) >= 3 {
					break
				}
			}
		}
	}

	return evidence
}

func sanitizeNumeric(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "%")
	s = strings.TrimSuffix(s, "°C")
	s = strings.TrimSuffix(s, "C")
	s = strings.TrimSuffix(s, "MB")
	s = strings.TrimSuffix(s, "KB")
	s = strings.TrimSuffix(s, "GB")
	return strings.TrimSpace(s)
}

func parseOptionalFloat(s string) (float64, bool) {
	clean := sanitizeNumeric(s)
	if clean == "" {
		return 0, false
	}
	f, err := strconv.ParseFloat(clean, 64)
	return f, err == nil
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
