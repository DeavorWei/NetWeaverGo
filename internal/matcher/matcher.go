package matcher

import (
	"strings"
	"sync"
)

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
			// 在此简单返回命中的首个规则。可以通过重排序保证 critical 优先
			return true, &rule
		}
	}
	return false, nil
}

// IsPrompt 检查字符流尾部是否为常见提示符（用于判读命令是否执行完毕）
func (m *StreamMatcher) IsPrompt(chunk string) bool {
	cleanChunk := strings.TrimSpace(chunk)
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, prompt := range m.Prompts {
		if strings.HasSuffix(cleanChunk, prompt) {
			return true
		}
	}
	return false
}

// IsPaginationPrompt 检查字符流中是否包含了分页拦截符 (如 ---- More ----)
func (m *StreamMatcher) IsPaginationPrompt(chunk string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, prompt := range m.PaginationPrompts {
		if strings.Contains(chunk, prompt) {
			return true
		}
	}
	return false
}
