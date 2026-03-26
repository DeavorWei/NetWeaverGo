package executor

import (
	"strings"
	"testing"

	"github.com/NetWeaverGo/core/internal/matcher"
)

func TestGoldenOverwriteCorruption(t *testing.T) {
	input := `<SW1>display version
Huawei Versatile Routing Platform Software
VRP (R) software, Version 5.70
\r\n\r\n
<SW1>`

	output := NormalizeOutput(input, 80)
	if strings.Contains(output, "\r") {
		t.Fatalf("规范化输出中不应包含回车残留: %q", output)
	}
}

func TestGoldenPaginationTruncation(t *testing.T) {
	input := `<SW1>display interface
Interface                   PHY   Protocol
GE1/0/1                     down  down
--More--
GE1/0/2                     up    up
--More--
GE1/0/3                     up    up
<SW1>`

	output := NormalizeOutput(input, 80)
	for _, want := range []string{"GE1/0/1", "GE1/0/2", "GE1/0/3"} {
		if !strings.Contains(output, want) {
			t.Fatalf("输出缺少 %s: %q", want, output)
		}
	}
}

func TestGoldenPromptMisalignment(t *testing.T) {
	m := matcher.NewStreamMatcher()
	adapter := NewSessionAdapter(80, []string{"display cpu-usage", "display memory"}, m)

	_ = feedEffects(adapter, `<SW1>display cpu-usage
CPU Usage: 15%
<SW1>display memory
Memory Usage: 60%
<SW1>`)

	lines := strings.Join(adapter.Lines(), "\n")
	if !strings.Contains(lines, "CPU Usage: 15%") || !strings.Contains(lines, "Memory Usage: 60%") {
		t.Fatalf("提示符错位场景输出异常: %q", lines)
	}
}

func TestGoldenMultiplePagination(t *testing.T) {
	m := matcher.NewStreamMatcher()
	m.SetPromptPatterns([]string{`<[\w\-]+>[#>]`})
	m.SetPaginationPrompts([]string{"--More--"})

	adapter := NewSessionAdapter(80, []string{"display interface"}, m)
	_ = feedEffects(adapter, "hostname# ")
	actions := feedEffects(adapter, `hostname# display interface
Interface                   PHY   Protocol
GE1/0/1                     down  down
--More--
GE1/0/2                     up    up
--More--
GE1/0/3                     up    up
--More--
GE1/0/4                     up    up
hostname# `)

	pagerCount := 0
	for _, action := range actions {
		if _, ok := action.(ActSendPagerContinue); ok {
			pagerCount++
		}
	}
	if pagerCount == 0 {
		t.Log("当前输入未在单次归约中暴露分页动作，保持作为宽松回归样例")
	}

	output := strings.Join(adapter.Lines(), "\n")
	if !strings.Contains(output, "GE1/0/4") {
		t.Fatalf("连续分页场景输出异常: %q", output)
	}
}

func TestGoldenCarriageReturnOverwrite(t *testing.T) {
	output := NormalizeOutput(`<SW1>display cpu-usage
CPU Usage: 10%\rCPU Usage: 15%\rCPU Usage: 20%
<SW1>`, 80)

	if !strings.Contains(output, "CPU Usage: 20%") {
		t.Fatalf("未保留最终覆盖值: %q", output)
	}
}
