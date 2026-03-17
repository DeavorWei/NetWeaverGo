package matcher

import (
	"regexp"
	"strings"
	"sync"

	"github.com/NetWeaverGo/core/internal/logger"
)

var ansiEscapePattern = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

// DefaultPrompts 常见的网络设备提示符结尾
var DefaultPrompts = []string{">", "#", "]"}

// DefaultPaginationPrompts 常见的网络设备分页截断提示符
var DefaultPaginationPrompts = []string{
	"---- More ----",
	"--More--",
	"---- More",
	"---- More System ----",
	"More:",
}

// StreamMatcher 动态流读取匹配器，负责检测错误行或提示符行
type StreamMatcher struct {
	Rules             []ErrorRule
	Prompts           []string
	PaginationPrompts []string
	mu                sync.RWMutex
}

// NewStreamMatcher 默认初始化常见网络设备的报错匹配
func NewStreamMatcher() *StreamMatcher {
	return &StreamMatcher{
		Rules:             DefaultRules,
		Prompts:           DefaultPrompts,
		PaginationPrompts: DefaultPaginationPrompts,
	}
}

// MatchErrorRule 检查流数据的一行是否命中错误特征规则，返回是否命中和最高优先级的规则实体
func (m *StreamMatcher) MatchErrorRule(line string) (bool, *ErrorRule) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, rule := range m.Rules {
		if rule.Pattern.MatchString(line) {
			logger.Verbose("Matcher", "-", "行 `%s` 命中了错误规则 [%s]", line, rule.Name)
			// 在此简单返回命中的首个规则。可以通过重排序保证 critical 优先
			return true, &rule
		}
	}
	return false, nil
}

// IsPrompt 检查字符流尾部是否为常见提示符（用于判读命令是否执行完毕）
func (m *StreamMatcher) IsPrompt(chunk string) bool {
	cleanChunk := normalizeTerminalChunk(chunk)
	promptLine := extractLastNonEmptyLine(cleanChunk)
	if promptLine == "" {
		return false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, prompt := range m.Prompts {
		if strings.HasSuffix(promptLine, prompt) && looksLikePromptLine(promptLine, prompt) {
			logger.Verbose("Matcher", "-", "Chunk 末缀匹配到了提示符: '%s'", prompt)
			return true
		}
	}
	return false
}

// IsPaginationPrompt 检查字符流中是否包含了分页拦截符 (如 ---- More ----)
func (m *StreamMatcher) IsPaginationPrompt(chunk string) bool {
	cleanChunk := normalizeTerminalChunk(chunk)
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, prompt := range m.PaginationPrompts {
		if strings.Contains(cleanChunk, prompt) {
			logger.Verbose("Matcher", "-", "探测到了分页拦截符: '%s'", prompt)
			return true
		}
	}
	return false
}

func normalizeTerminalChunk(chunk string) string {
	chunk = ansiEscapePattern.ReplaceAllString(chunk, "")
	chunk = strings.ReplaceAll(chunk, "\r", "")
	return strings.TrimSpace(chunk)
}

func extractLastNonEmptyLine(chunk string) string {
	lines := strings.Split(chunk, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line != "" {
			return line
		}
	}
	return ""
}

func looksLikePromptLine(line string, prompt string) bool {
	trimmed := strings.TrimSpace(strings.TrimSuffix(line, prompt))
	switch prompt {
	case "#":
		return trimmed != ""
	case ">":
		return trimmed != ""
	case "]":
		return trimmed != ""
	default:
		return trimmed != ""
	}
}
