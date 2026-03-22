package executor

import (
	"testing"

	"github.com/NetWeaverGo/core/internal/matcher"
)

// MockMatcher 用于测试的 Mock 匹配器
type MockMatcherForDetector struct {
	prompts    map[string]bool
	pagers     map[string]bool
	errorRules map[string]*matcher.ErrorRule
}

func NewMockMatcherForDetector() *MockMatcherForDetector {
	return &MockMatcherForDetector{
		prompts:    make(map[string]bool),
		pagers:     make(map[string]bool),
		errorRules: make(map[string]*matcher.ErrorRule),
	}
}

func (m *MockMatcherForDetector) IsPrompt(line string) bool {
	return m.prompts[line]
}

func (m *MockMatcherForDetector) IsPromptStrict(line string) bool {
	return m.prompts[line]
}

func (m *MockMatcherForDetector) IsPaginationPrompt(line string) bool {
	return m.pagers[line]
}

func (m *MockMatcherForDetector) MatchErrorRule(line string) (bool, *matcher.ErrorRule) {
	rule, ok := m.errorRules[line]
	return ok, rule
}

// TestDetectorDetectPrompt 测试检测提示符
func TestDetectorDetectPrompt(t *testing.T) {
	mockMatcher := NewMockMatcherForDetector()
	mockMatcher.prompts["<SW1>"] = true
	mockMatcher.prompts["[SW1]"] = true

	detector := NewSessionDetector(mockMatcher)

	// 测试已提交行中的提示符
	events := detector.Detect([]string{"<SW1>"}, "")
	if len(events) != 1 {
		t.Fatalf("期望 1 个事件，得到 %d", len(events))
	}

	committedLine, ok := events[0].(EvCommittedLine)
	if !ok {
		t.Fatalf("期望 EvCommittedLine 事件，得到 %T", events[0])
	}
	if committedLine.Line != "<SW1>" {
		t.Errorf("期望行 '<SW1>'，得到 '%s'", committedLine.Line)
	}
}

// TestDetectorDetectPager 测试检测分页符
func TestDetectorDetectPager(t *testing.T) {
	mockMatcher := NewMockMatcherForDetector()
	mockMatcher.pagers["--More--"] = true

	detector := NewSessionDetector(mockMatcher)

	events := detector.Detect([]string{"--More--"}, "")
	if len(events) != 1 {
		t.Fatalf("期望 1 个事件，得到 %d", len(events))
	}

	pagerEvent, ok := events[0].(EvPagerSeen)
	if !ok {
		t.Fatalf("期望 EvPagerSeen 事件，得到 %T", events[0])
	}
	if pagerEvent.Line != "--More--" {
		t.Errorf("期望行 '--More--'，得到 '%s'", pagerEvent.Line)
	}
}

// TestDetectorDetectError 测试检测错误规则
func TestDetectorDetectError(t *testing.T) {
	mockMatcher := NewMockMatcherForDetector()
	mockMatcher.errorRules["Error: Invalid command"] = &matcher.ErrorRule{
		Name:     "invalid_command",
		Pattern:  nil,
		Severity: matcher.SeverityCritical,
		Message:  "无效命令",
	}

	detector := NewSessionDetector(mockMatcher)

	events := detector.Detect([]string{"Error: Invalid command"}, "")
	if len(events) != 1 {
		t.Fatalf("期望 1 个事件，得到 %d", len(events))
	}

	errorEvent, ok := events[0].(EvErrorMatched)
	if !ok {
		t.Fatalf("期望 EvErrorMatched 事件，得到 %T", events[0])
	}
	if errorEvent.Rule.Name != "invalid_command" {
		t.Errorf("期望规则名 'invalid_command'，得到 '%s'", errorEvent.Rule.Name)
	}
}

// TestDetectorDetectActivePrompt 测试检测活动行提示符
func TestDetectorDetectActivePrompt(t *testing.T) {
	mockMatcher := NewMockMatcherForDetector()
	mockMatcher.prompts["<SW1>"] = true

	detector := NewSessionDetector(mockMatcher)

	events := detector.Detect([]string{}, "<SW1>")
	if len(events) != 1 {
		t.Fatalf("期望 1 个事件，得到 %d", len(events))
	}

	activePrompt, ok := events[0].(EvActivePromptSeen)
	if !ok {
		t.Fatalf("期望 EvActivePromptSeen 事件，得到 %T", events[0])
	}
	if activePrompt.Prompt != "<SW1>" {
		t.Errorf("期望提示符 '<SW1>'，得到 '%s'", activePrompt.Prompt)
	}
}

// TestDetectorPriority 测试事件优先级（分页符 > 错误 > 提示符）
func TestDetectorPriority(t *testing.T) {
	mockMatcher := NewMockMatcherForDetector()
	mockMatcher.pagers["--More--"] = true
	mockMatcher.prompts["<SW1>"] = true
	mockMatcher.errorRules["Error: test"] = &matcher.ErrorRule{
		Name:     "test_error",
		Severity: matcher.SeverityCritical,
	}

	detector := NewSessionDetector(mockMatcher)

	// 分页符应该被检测为分页事件，而不是提示符
	events := detector.Detect([]string{"--More--"}, "")
	if len(events) != 1 {
		t.Fatalf("期望 1 个事件，得到 %d", len(events))
	}
	if _, ok := events[0].(EvPagerSeen); !ok {
		t.Errorf("期望 EvPagerSeen 事件，得到 %T", events[0])
	}
}

// TestDetectorMultipleLines 测试多行检测
func TestDetectorMultipleLines(t *testing.T) {
	mockMatcher := NewMockMatcherForDetector()
	mockMatcher.prompts["<SW1>"] = true
	mockMatcher.pagers["--More--"] = true

	detector := NewSessionDetector(mockMatcher)

	lines := []string{
		"display version",
		"Huawei Versatile Routing Platform Software",
		"--More--",
		"<SW1>",
	}

	events := detector.Detect(lines, "")

	// 应该检测到：普通行、普通行、分页符、提示符
	if len(events) != 4 {
		t.Fatalf("期望 4 个事件，得到 %d", len(events))
	}

	// 第三个应该是分页符
	if _, ok := events[2].(EvPagerSeen); !ok {
		t.Errorf("第 3 个事件期望 EvPagerSeen，得到 %T", events[2])
	}

	// 第四个应该是提示符
	if _, ok := events[3].(EvCommittedLine); !ok {
		t.Errorf("第 4 个事件期望 EvCommittedLine，得到 %T", events[3])
	}
}

// TestDetectorClassifyLine 测试行分类
func TestDetectorClassifyLine(t *testing.T) {
	mockMatcher := NewMockMatcherForDetector()
	mockMatcher.prompts["<SW1>"] = true
	mockMatcher.pagers["--More--"] = true
	mockMatcher.errorRules["Error: test"] = &matcher.ErrorRule{
		Name:     "test_error",
		Severity: matcher.SeverityCritical,
	}

	detector := NewSessionDetector(mockMatcher)

	tests := []struct {
		line     string
		expected string
	}{
		{"<SW1>", "prompt"},
		{"--More--", "pager"},
		{"Error: test", "error"},
		{"normal output", "normal"},
	}

	for _, tt := range tests {
		result := detector.ClassifyLine(tt.line)
		if result != tt.expected {
			t.Errorf("ClassifyLine('%s') = '%s', 期望 '%s'", tt.line, result, tt.expected)
		}
	}
}

// TestDetectorEmptyInput 测试空输入
func TestDetectorEmptyInput(t *testing.T) {
	mockMatcher := NewMockMatcherForDetector()
	detector := NewSessionDetector(mockMatcher)

	events := detector.Detect([]string{}, "")
	if len(events) != 0 {
		t.Errorf("期望 0 个事件，得到 %d", len(events))
	}
}
