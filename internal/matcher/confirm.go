package matcher

import (
	"regexp"
	"strings"
)

// DefaultConfirmPatterns 常见的命令行确认提示符正则模式
// 必须出现在行末，满足严格的行尾定界
var DefaultConfirmPatterns = []string{
	`\[[Yy]/[Nn]\][:\?]?\s*$`,
	`\([Yy]es/[Nn]o\)[:\?]?\s*$`,
	`\[[Yy]es/[Nn]o\][:\?]?\s*$`,
	`[Cc]ontinue\?[:\?]?\s*$`,
	`[Aa]re you sure.*[:\?]\s*$`,
}

// CompileConfirmPatterns 编译确认提示符正则列表
func CompileConfirmPatterns(patterns []string) []*regexp.Regexp {
	if len(patterns) == 0 {
		patterns = DefaultConfirmPatterns
	}
	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if re, err := regexp.Compile(p); err == nil {
			compiled = append(compiled, re)
		}
	}
	return compiled
}

// CheckConfirmPrompt 三重保险检查交互确认提示符：
// 1. 数据非空
// 2. 检查尾部窗口（最多 500 字符）
// 3. 必须命中行末确认正则
func CheckConfirmPrompt(text string, patterns []*regexp.Regexp) (bool, string) {
	if len(text) == 0 {
		return false, ""
	}

	// 窗口限制：最多取尾部 500 字符
	window := text
	if len(window) > 500 {
		window = window[len(window)-500:]
	}

	// 去除最右侧空白以提取有效行尾
	trimmedWindow := strings.TrimRight(window, "\r\n")
	// 取出最后一行文本
	lastLine := trimmedWindow
	if idx := strings.LastIndex(trimmedWindow, "\n"); idx != -1 {
		lastLine = trimmedWindow[idx+1:]
	}
	lastLine = strings.TrimRight(lastLine, " \t")

	if len(patterns) == 0 {
		patterns = CompileConfirmPatterns(DefaultConfirmPatterns)
	}

	for _, re := range patterns {
		if loc := re.FindStringIndex(lastLine); loc != nil {
			matched := lastLine[loc[0]:loc[1]]
			return true, matched
		}
	}

	return false, ""
}
