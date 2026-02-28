package matcher

import (
	"regexp"
	"strings"
	"sync"
)

// DefaultPrompts 常见的网络设备提示符结尾
var DefaultPrompts = []string{">", "#", "]"}

// StreamMatcher 动态流读取匹配器，负责检测错误行或提示符行
type StreamMatcher struct {
	ErrorPatterns []*regexp.Regexp
	Prompts       []string
	mu            sync.RWMutex
}

// NewStreamMatcher 默认初始化常见网络设备的报错匹配
func NewStreamMatcher() *StreamMatcher {
	return &StreamMatcher{
		ErrorPatterns: []*regexp.Regexp{
			// TODO: 用户要求暂时关闭报错暂停执行的逻辑，默认放行所有命令
			// 这部分内容后续再重新设计补充
		},
		Prompts: DefaultPrompts,
	}
}

// MatchError 检查流数据的一行是否命中错误特征正则表达式
func (m *StreamMatcher) MatchError(line string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, pattern := range m.ErrorPatterns {
		if pattern.MatchString(line) {
			return true
		}
	}
	return false
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
