package executor

import (
	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/matcher"
)

// 负责从 replayer 输出提取协议事件
// 只负责"看见了什么"，不参与状态决策

// SessionDetector 协议事件检测器
type SessionDetector struct {
	matcher MatcherInterface
}

// NewSessionDetector 创建新的检测器
func NewSessionDetector(matcher MatcherInterface) *SessionDetector {
	return &SessionDetector{
		matcher: matcher,
	}
}

// DetectResult 检测结果
type DetectResult struct {
	// Events 检测到的事件列表
	Events []SessionEvent
	// HasPrompt 是否检测到提示符
	HasPrompt bool
	// HasPager 是否检测到分页符
	HasPager bool
	// HasError 是否检测到错误
	HasError bool
}

// DetectInitPrompt 从规范化输出中检测初始化阶段提示符。
func (d *SessionDetector) DetectInitPrompt(lines []string, activeLine string) []SessionEvent {
	if prompt, ok := d.findPrompt(lines, activeLine); ok {
		//logger.Debug("SessionDetector", "-", "检测到初始提示符: %s", truncateDebug(prompt, 50))
		return []SessionEvent{EvInitPromptStable{Prompt: prompt}}
	}
	return nil
}

// DetectWarmupPrompt 从规范化输出中检测预热后的提示符。
func (d *SessionDetector) DetectWarmupPrompt(lines []string, activeLine string) []SessionEvent {
	if prompt, ok := d.findPrompt(lines, activeLine); ok {
		//logger.Debug("SessionDetector", "-", "检测到预热后提示符: %s", truncateDebug(prompt, 50))
		return []SessionEvent{EvWarmupPromptSeen{Prompt: prompt}}
	}
	return nil
}

// Detect 从规范化行中提取协议事件
// lines: 已提交的规范化行
// activeLine: 当前活动行（未提交的行）
// 返回检测到的事件列表
func (d *SessionDetector) Detect(lines []string, activeLine string) []SessionEvent {
	events := make([]SessionEvent, 0)

	// 1. 处理已提交的行
	for _, line := range lines {
		// 检测分页符（优先级最高）
		if d.matcher.IsPaginationPrompt(line) {
			//logger.Debug("SessionDetector", "-", "检测到分页符: %s", truncateDebug(line, 50))
			events = append(events, EvPagerSeen{Line: line})
			continue
		}

		// 检测错误规则
		if matched, rule := d.matcher.MatchErrorRule(line); matched {
			logger.Debug("SessionDetector", "-", "检测到错误规则命中: %s", rule.Name)
			events = append(events, EvErrorMatched{
				Line: line,
				Rule: rule,
			})
			continue
		}

		// 检测提示符
		if d.matcher.IsPromptStrict(line) {
			//logger.Debug("SessionDetector", "-", "检测到提示符(已提交行): %s", truncateDebug(line, 50))
			events = append(events, EvCommittedLine{Line: line})
			continue
		}

		// 普通行
		events = append(events, EvCommittedLine{Line: line})
	}

	// 2. 处理活动行
	if activeLine != "" {
		// 检测分页符
		if d.matcher.IsPaginationPrompt(activeLine) {
			//logger.Debug("SessionDetector", "-", "检测到分页符(活动行): %s", truncateDebug(activeLine, 50))
			events = append(events, EvPagerSeen{Line: activeLine})
		} else if d.matcher.IsPromptStrict(activeLine) {
			// 检测提示符
			//logger.Debug("SessionDetector", "-", "检测到提示符(活动行): %s", truncateDebug(activeLine, 50))
			events = append(events, EvActivePromptSeen{Prompt: activeLine})
		}
	}

	return events
}

// DetectFromChunk 从原始 chunk 检测初始化阶段的提示符
// 用于 NewStateInitAwaitPrompt 和 NewStateInitAwaitWarmupPrompt 状态
func (d *SessionDetector) DetectFromChunk(chunk string) []SessionEvent {
	events := make([]SessionEvent, 0)

	// 检测提示符（非严格模式，用于初始化阶段）
	if d.matcher.IsPrompt(chunk) {
		//logger.Debug("SessionDetector", "-", "检测到初始提示符: %s", truncateDebug(chunk, 50))
		// 提取提示符
		prompt := extractPromptHint(chunk)
		events = append(events, EvInitPromptStable{Prompt: prompt})
	}

	// 严格模式检测
	if d.matcher.IsPromptStrict(chunk) {
		//logger.Debug("SessionDetector", "-", "严格模式检测到初始提示符: %s", truncateDebug(chunk, 50))
		prompt := extractPromptHint(chunk)
		events = append(events, EvInitPromptStable{Prompt: prompt})
	}

	return events
}

// DetectPromptInLine 检测行中的提示符
// 返回是否为提示符和提取的提示符内容
func (d *SessionDetector) DetectPromptInLine(line string) (bool, string) {
	if d.matcher.IsPromptStrict(line) {
		prompt := extractPromptHint(line)
		return true, prompt
	}
	return false, ""
}

// DetectPagerInLine 检测行中的分页符
func (d *SessionDetector) DetectPagerInLine(line string) bool {
	return d.matcher.IsPaginationPrompt(line)
}

// DetectErrorInLine 检测行中的错误规则
func (d *SessionDetector) DetectErrorInLine(line string) (bool, *matcher.ErrorRule) {
	return d.matcher.MatchErrorRule(line)
}

// ClassifyLine 分类单行内容
// 返回行类型：prompt, pager, error, normal
func (d *SessionDetector) ClassifyLine(line string) string {
	if d.matcher.IsPaginationPrompt(line) {
		return "pager"
	}
	if matched, _ := d.matcher.MatchErrorRule(line); matched {
		return "error"
	}
	if d.matcher.IsPromptStrict(line) {
		return "prompt"
	}
	return "normal"
}

// truncateDebug 截断字符串用于调试输出（Detector 专用）
func truncateDebug(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

func (d *SessionDetector) findPrompt(lines []string, activeLine string) (string, bool) {
	if activeLine != "" && d.matcher.IsPromptStrict(activeLine) {
		return extractPromptHint(activeLine), true
	}

	for i := len(lines) - 1; i >= 0; i-- {
		if d.matcher.IsPromptStrict(lines[i]) {
			return extractPromptHint(lines[i]), true
		}
	}

	return "", false
}
