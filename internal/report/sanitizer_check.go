package report

import (
	"fmt"
	"regexp"
	"strings"
)

// 未脱敏特征检测正则（用于导出前自检/阻断，呼应规划方案 §5.2 P1-6）
var unmaskedPatterns = []struct {
	name    string
	pattern *regexp.Regexp
}{
	{
		name:    "配置密码未掩码",
		pattern: regexp.MustCompile(`(?i)\bpassword\s+(?:simple|cipher|plain)\s+([^\s\*]{2,})`),
	},
	{
		name:    "通用密码未掩码",
		pattern: regexp.MustCompile(`(?i)\bpassword\s+([^\s\*]{2,})`),
	},
	{
		name:    "明文 shared-key / secret 未掩码",
		pattern: regexp.MustCompile(`(?i)\b(?:shared-key|secret)\s+(?:cipher\s+)?([^\s\*]{2,})`),
	},
	{
		name:    "JSON 敏感字段未掩码",
		pattern: regexp.MustCompile(`(?i)"(?:password|secret|token|api_key|private_key)"\s*:\s*"([^\*"]+)"`),
	},
}

// CheckContentSanitized 抽检文本内容是否完全脱敏
// 返回:
//   - ok: true 表示通过脱敏检查，false 表示发现未脱敏敏感内容
//   - violations: 命中的违规特征描述与摘要
func CheckContentSanitized(content string) (bool, []string) {
	if len(content) == 0 {
		return true, nil
	}

	var violations []string
	lines := strings.Split(content, "\n")

	for lineIdx, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		for _, up := range unmaskedPatterns {
			if m := up.pattern.FindStringSubmatch(trimmed); len(m) > 1 {
				matchedVal := strings.TrimSpace(m[1])
				// 若匹配值包含 **** 或为模式指示词 (cipher/simple/plain)，则忽略
				if strings.Contains(matchedVal, "****") ||
					strings.EqualFold(matchedVal, "cipher") ||
					strings.EqualFold(matchedVal, "simple") ||
					strings.EqualFold(matchedVal, "plain") {
					continue
				}
				violation := fmt.Sprintf("行 %d 命中 [%s]: %s", lineIdx+1, up.name, trimmed)
				violations = append(violations, violation)
				if len(violations) >= 10 {
					return false, violations
				}
			}
		}
	}

	return len(violations) == 0, violations
}

// ValidateExportContent 校验即将导出的内容，未脱敏时返回阻断错误
func ValidateExportContent(content string) error {
	ok, violations := CheckContentSanitized(content)
	if !ok {
		return fmt.Errorf("导出安全阻断: 检测到 %d 处未脱敏敏感数据 (例如: %s)，禁止导出", len(violations), violations[0])
	}
	return nil
}
