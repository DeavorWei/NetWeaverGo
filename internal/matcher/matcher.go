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
// 重要：如果 chunk 中包含分页符，返回 false（分页处理优先）
func (m *StreamMatcher) IsPrompt(chunk string) bool {
	// DEBUG: 打印原始输入
	logger.Debug("Matcher", "-", "[DEBUG] IsPrompt 输入 chunk 长度=%d, 内容='%s'", len(chunk), truncateString(chunk, 200))

	cleanChunk := normalizeTerminalChunk(chunk)
	logger.Debug("Matcher", "-", "[DEBUG] IsPrompt cleanChunk='%s'", truncateString(cleanChunk, 200))

	promptLine := extractLastNonEmptyLine(cleanChunk)
	logger.Debug("Matcher", "-", "[DEBUG] IsPrompt promptLine='%s'", promptLine)

	if promptLine == "" {
		logger.Debug("Matcher", "-", "[DEBUG] IsPrompt promptLine 为空，返回 false")
		return false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 关键修复：优先检测分页符
	// 如果 chunk 中包含分页符，说明命令输出未完成，不应该认为是提示符
	// 这处理了分页符和提示符在同一 chunk 或相邻 chunk 的情况
	for _, paginationPrompt := range m.PaginationPrompts {
		if strings.Contains(cleanChunk, paginationPrompt) {
			logger.Verbose("Matcher", "-", "Chunk 包含分页符 '%s'，跳过提示符检测", paginationPrompt)
			logger.Debug("Matcher", "-", "[DEBUG] IsPrompt 检测到分页符，返回 false")
			return false
		}
	}

	// 特殊处理：华为格式提示符 <主机名>
	// 这种格式以 > 结尾，但前面有 < 包裹，如 <S2>
	if strings.HasPrefix(promptLine, "<") && strings.HasSuffix(promptLine, ">") {
		// 提取 < 和 > 之间的内容
		inner := strings.TrimPrefix(strings.TrimSuffix(promptLine, ">"), "<")
		// 内部应该有内容（主机名）
		if inner != "" && !strings.Contains(inner, " ") {
			logger.Verbose("Matcher", "-", "检测到华为格式提示符: '%s'", promptLine)
			logger.Debug("Matcher", "-", "[DEBUG] IsPrompt 华为格式匹配成功，返回 true")
			return true
		}
	}

	// 首先检查后缀匹配
	for _, prompt := range m.Prompts {
		if strings.HasSuffix(promptLine, prompt) && looksLikePromptLine(promptLine, prompt) {
			logger.Verbose("Matcher", "-", "Chunk 末缀匹配到了提示符: '%s'", prompt)
			logger.Debug("Matcher", "-", "[DEBUG] IsPrompt 后缀匹配成功 prompt='%s', promptLine='%s', 返回 true", prompt, promptLine)
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

// IsPromptStrict 严格提示符检测（用于初始化阶段和分页后）
// 只接受"整行就是提示符"的格式，不接受混合行
// 华为格式: 整行必须是 <主机名>
// 方括号格式: 整行必须是 [主机名]
// Cisco/Huawei 特权模式: 整行必须是 主机名# 或 主机名>
func (m *StreamMatcher) IsPromptStrict(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return false
	}

	// 华为格式: 整行必须是 <主机名>
	// 排除 "<The current login time...>" 这类混合行
	if strings.HasPrefix(line, "<") && strings.HasSuffix(line, ">") {
		inner := strings.TrimPrefix(strings.TrimSuffix(line, ">"), "<")
		// 内部不能有空格（排除混合行）
		if inner != "" && !strings.Contains(inner, " ") {
			logger.Verbose("Matcher", "-", "严格模式检测到华为格式提示符: '%s'", line)
			return true
		}
	}

	// 方括号格式: 整行必须是 [主机名]
	if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
		inner := strings.TrimPrefix(strings.TrimSuffix(line, "]"), "[")
		if inner != "" && !strings.Contains(inner, " ") {
			logger.Verbose("Matcher", "-", "严格模式检测到方括号格式提示符: '%s'", line)
			return true
		}
	}

	// Cisco/Huawei 特权模式: 整行必须是 主机名# 或 主机名>
	// 必须以 # 或 > 结尾，且前面只有一个单词（主机名）
	for _, suffix := range []string{"#", ">"} {
		if strings.HasSuffix(line, suffix) {
			prefix := strings.TrimSuffix(line, suffix)
			// 主机名不能包含空格，不能包含 <（排除华为格式的变体）
			if prefix != "" && !strings.Contains(prefix, " ") && !strings.Contains(prefix, "<") {
				logger.Verbose("Matcher", "-", "严格模式检测到特权模式提示符: '%s'", line)
				return true
			}
		}
	}

	// 检查正则模式匹配
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, pattern := range m.PromptPatterns {
		if pattern.MatchString(line) {
			logger.Verbose("Matcher", "-", "严格模式正则匹配到了提示符: '%s'", pattern.String())
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
			//logger.Verbose("Matcher", "-", "探测到了分页拦截符: '%s'", prompt)
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
		// Cisco/Huawei 特权模式: 主机名# 或 主机名#
		// 也支持格式如 <主机名>#（虽然少见）
		return trimmed != ""
	case ">":
		// 华为格式: <主机名> 或 主机名>
		// 检查是否是华为格式 <主机名>
		if strings.HasPrefix(trimmed, "<") && !strings.HasSuffix(trimmed, ">") {
			// 格式如 <S2，说明是 <S2> 被截断后的提示符
			return true
		}
		return trimmed != ""
	case "]":
		// 其他格式如 [主机名]
		return trimmed != ""
	default:
		return trimmed != ""
	}
}

// truncateString 截断字符串用于日志输出
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
