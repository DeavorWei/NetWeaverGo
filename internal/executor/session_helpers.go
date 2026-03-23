package executor

import (
	"strings"
	"time"

	"github.com/NetWeaverGo/core/internal/matcher"
)

// ErrorContext 错误上下文，用于传递错误检测信息给外部决策处理器。
type ErrorContext struct {
	Line     string
	Rule     *matcher.ErrorRule
	CmdIndex int
	Cmd      string
}

// parseInlineCommand 解析内联命令注释，返回实际命令和自定义超时。
func parseInlineCommand(rawCmd string) (string, time.Duration) {
	cmdToSend := rawCmd
	var customTimeout time.Duration

	if idx := strings.Index(rawCmd, "// nw-timeout="); idx != -1 {
		cmdToSend = strings.TrimSpace(rawCmd[:idx])
		timeoutStr := strings.TrimSpace(rawCmd[idx+len("// nw-timeout="):])
		if pd, err := time.ParseDuration(timeoutStr); err == nil {
			customTimeout = pd
		}
	}

	return cmdToSend, customTimeout
}

// extractPromptHint 从行中提取提示符提示。
func extractPromptHint(line string) string {
	line = strings.TrimSpace(line)
	if len(line) > 20 {
		return line[len(line)-20:]
	}
	return line
}

// truncateStringDebug 截断字符串用于调试日志。
func truncateStringDebug(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
