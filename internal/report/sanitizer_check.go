package report

import (
	"fmt"
	"regexp"
	"strings"
)

// CheckSeverity 脱敏违规严重度
type CheckSeverity string

const (
	SeverityCritical CheckSeverity = "CRITICAL" // 密码密文、密钥凭证未脱敏（阻断导出）
	SeverityWarn     CheckSeverity = "WARN"     // 提示词/通用敏感词可能未脱敏（仅告警）
)

// CheckViolation 脱敏违规明细
type CheckViolation struct {
	Line     int           `json:"line"`
	Name     string        `json:"name"`
	Severity CheckSeverity `json:"severity"`
	Snippet  string        `json:"snippet"`
}

// 未脱敏特征检测正则（用于导出前自检/阻断，呼应规划方案 §5.2 P1-6）
var unmaskedPatterns = []struct {
	name     string
	severity CheckSeverity
	pattern  *regexp.Regexp
}{
	{
		name:     "配置密码未掩码",
		severity: SeverityCritical,
		pattern:  regexp.MustCompile(`(?i)\bpassword\s+(?:simple|cipher|plain)\s+([^\s\*]{2,})`),
	},
	{
		name:     "通用密码未掩码",
		severity: SeverityCritical, // 提升为 Critical 级别，杜绝明文口令泄漏阻断逃逸
		pattern:  regexp.MustCompile(`(?i)\bpassword\s+([^\s\*]{2,})`),
	},
	{
		name:     "明文 shared-key / secret 未掩码",
		severity: SeverityCritical,
		pattern:  regexp.MustCompile(`(?i)\b(?:shared-key|secret)\s+(?:cipher\s+)?([^\s\*]{2,})`),
	},
	{
		name:     "SNMP 团体名未掩码",
		severity: SeverityCritical,
		pattern:  regexp.MustCompile(`(?i)\b(?:snmp-agent\s+community\s+(?:read|write)|snmp-server\s+community)\s+(?:cipher\s+|simple\s+)?([^\s\*]{2,})`),
	},
	{
		name:     "SNMPv3 认证或加密凭证未掩码",
		severity: SeverityCritical,
		pattern:  regexp.MustCompile(`(?i)\b(?:authentication-mode|privacy-mode)\s+(?:md5|sha|sha2-256|aes128|des56)\s+(?:cipher\s+)?([^\s\*]{2,})`),
	},
	{
		name:     "认证模式明文未掩码",
		severity: SeverityCritical,
		pattern:  regexp.MustCompile(`(?i)\bauthentication-mode\s+simple\s+([^\s\*]{2,})`),
	},
	{
		name:     "私钥明文未脱敏",
		severity: SeverityCritical,
		pattern:  regexp.MustCompile(`(?i)(-----BEGIN\s+(?:RSA\s+|EC\s+|OPENSSH\s+)?PRIVATE\s+KEY-----)`),
	},
	{
		name:     "JSON 敏感字段未掩码",
		severity: SeverityCritical,
		pattern:  regexp.MustCompile(`(?i)"(?:password|secret|token|api_key|private_key)"\s*:\s*"([^\*"]+)"`),
	},
}

// CheckContentSanitizedDetailed 抽检文本内容是否完全脱敏（含分级）
func CheckContentSanitizedDetailed(content string) (bool, []CheckViolation) {
	if len(content) == 0 {
		return true, nil
	}

	var violations []CheckViolation
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
				violations = append(violations, CheckViolation{
					Line:     lineIdx + 1,
					Name:     up.name,
					Severity: up.severity,
					Snippet:  trimmed,
				})
				if len(violations) >= 20 {
					return false, violations
				}
			}
		}
	}

	return len(violations) == 0, violations
}

// CheckContentSanitized 抽检文本内容是否完全脱敏（兼容历史接口）
func CheckContentSanitized(content string) (bool, []string) {
	ok, details := CheckContentSanitizedDetailed(content)
	if ok {
		return true, nil
	}
	var res []string
	for _, v := range details {
		res = append(res, fmt.Sprintf("行 %d 命中 [%s][%s]: %s", v.Line, v.Severity, v.Name, v.Snippet))
	}
	return false, res
}

// ValidateExportContent 校验即将导出的内容，仅在存在 CRITICAL 级别未脱敏时阻断导出
func ValidateExportContent(content string) error {
	ok, violations := CheckContentSanitizedDetailed(content)
	if ok {
		return nil
	}

	var criticals []CheckViolation
	for _, v := range violations {
		if v.Severity == SeverityCritical {
			criticals = append(criticals, v)
		}
	}

	if len(criticals) > 0 {
		return fmt.Errorf("导出安全阻断: 检测到 %d 处 CRITICAL 级未脱敏敏感数据 (例如: %s)，禁止导出", len(criticals), criticals[0].Snippet)
	}
	return nil
}
