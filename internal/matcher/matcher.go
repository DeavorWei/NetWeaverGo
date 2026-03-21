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
	PromptPatterns    []*regexp.Regexp // 正则模式（可选）
	mu                sync.RWMutex
}

// NewStreamMatcher 默认初始化常见网络设备的报错匹配
func NewStreamMatcher() *StreamMatcher {
	return &StreamMatcher{
		Rules:             DefaultRules,
		Prompts:           DefaultPrompts,
		PaginationPrompts: DefaultPaginationPrompts,
		PromptPatterns:    nil,
	}
}

// NewStreamMatcherWithConfig 使用配置创建匹配器
func NewStreamMatcherWithConfig(prompts []string, paginationPrompts []string, promptPatterns []string) *StreamMatcher {
	m := &StreamMatcher{
		Rules:             DefaultRules,
		Prompts:           prompts,
		PaginationPrompts: paginationPrompts,
		PromptPatterns:    make([]*regexp.Regexp, 0, len(promptPatterns)),
	}

	// 编译正则模式
	for _, pattern := range promptPatterns {
		if re, err := regexp.Compile(pattern); err == nil {
			m.PromptPatterns = append(m.PromptPatterns, re)
		}
	}

	return m
}

// SetPrompts 设置提示符后缀
func (m *StreamMatcher) SetPrompts(prompts []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Prompts = prompts
}

// SetPaginationPrompts 设置分页提示符
func (m *StreamMatcher) SetPaginationPrompts(prompts []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.PaginationPrompts = prompts
}

// SetPromptPatterns 设置提示符正则模式
func (m *StreamMatcher) SetPromptPatterns(patterns []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.PromptPatterns = make([]*regexp.Regexp, 0, len(patterns))
	for _, pattern := range patterns {
		if re, err := regexp.Compile(pattern); err == nil {
			m.PromptPatterns = append(m.PromptPatterns, re)
		}
	}
}

// ConfigureFromProfile 从设备画像配置匹配器
func (m *StreamMatcher) ConfigureFromProfile(promptSuffixes []string, promptPatterns []string, paginationPatterns []string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(promptSuffixes) > 0 {
		m.Prompts = promptSuffixes
	}
	if len(paginationPatterns) > 0 {
		m.PaginationPrompts = paginationPatterns
	}
	if len(promptPatterns) > 0 {
		m.PromptPatterns = make([]*regexp.Regexp, 0, len(promptPatterns))
		for _, pattern := range promptPatterns {
			if re, err := regexp.Compile(pattern); err == nil {
				m.PromptPatterns = append(m.PromptPatterns, re)
			}
		}
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

	// 首先检查后缀匹配
	for _, prompt := range m.Prompts {
		if strings.HasSuffix(promptLine, prompt) && looksLikePromptLine(promptLine, prompt) {
			logger.Verbose("Matcher", "-", "Chunk 末缀匹配到了提示符: '%s'", prompt)
			return true
		}
	}

	// 然后检查正则模式匹配
	for _, pattern := range m.PromptPatterns {
		if pattern.MatchString(promptLine) {
			logger.Verbose("Matcher", "-", "Chunk 正则匹配到了提示符: '%s'", pattern.String())
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
