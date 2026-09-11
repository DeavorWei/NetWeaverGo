package taskexec

import "strings"

// matchPathLevel 将 ResolveProfile 返回的匹配路径归一化为有界的档位标签。
// 例：exact:S5735 → exact；series:S5700 → series；vendor:huawei → vendor；global:default → global。
func matchPathLevel(matchPath string) string {
	trimmed := strings.TrimSpace(matchPath)
	if trimmed == "" {
		return "unknown"
	}
	if idx := strings.Index(trimmed, ":"); idx > 0 {
		return trimmed[:idx]
	}
	return trimmed
}
