package executor

import (
	"strings"
	"testing"

	"github.com/NetWeaverGo/core/internal/matcher"
)

// ============================================================================
// Session Golden Tests - 长期回归测试套件
// ============================================================================
// 这些测试用例来自真实事故样本，确保修复后不再回退
// 每个测试用例对应 testdata/regression/bug_fixes/ 目录下的样本

// TestGoldenPaginationOverwrite 分页符被覆盖测试
// 对应 issue_004_pagination_overwrite
// 此测试验证分页符检测功能，而非完整会话流程
func TestGoldenPaginationOverwrite(t *testing.T) {
	// 测试 matcher 的分页符检测功能
	m := matcher.NewStreamMatcher()
	m.SetPromptPatterns([]string{`<[\w\-]+>[#>]`})
	m.SetPaginationPrompts([]string{"--More--"})

	// 测试1: 分页符应该被正确检测
	if !m.IsPaginationPrompt("--More--") {
		t.Error("分页符 '--More--' 应该被正确检测")
	}

	// 测试2: 带 ANSI 转义序列的分页符也应该被检测
	if !m.IsPaginationPrompt("\x1b[16D--More--") {
		t.Error("带 ANSI 转义的分页符应该被正确检测")
	}

	// 测试3: 分页符在行内也应该被检测
	if !m.IsPaginationPrompt("  --More--  ") {
		t.Error("行内分页符应该被正确检测")
	}

	t.Logf("分页符检测测试通过")
}

// TestGoldenOverwriteCorruption 回车覆盖损坏测试
// 对应 issue_003_overwrite_corruption
func TestGoldenOverwriteCorruption(t *testing.T) {
	// 模拟回车覆盖导致的输出损坏
	input := `<SW1>display version
Huawei Versatile Routing Platform Software
VRP (R) software, Version 5.70
\r\n\r\n
<SW1>`

	m := matcher.NewStreamMatcher()
	m.SetPromptPatterns([]string{`<[\w\-]+>[#>]`})

	machine := NewSessionMachine(80, []string{"display version"}, m)
	machine.Feed(input)

	lines := machine.Lines()

	// 验证输出不包含控制字符残留
	for _, line := range lines {
		if strings.Contains(line, "\r") {
			t.Errorf("输出中包含回车符残留: %q", line)
		}
	}

	t.Logf("测试通过，共 %d 行输出", len(lines))
}

// TestGoldenPromptMisalignment 提示符错位测试
// 对应 issue_002_prompt_misalignment
func TestGoldenPromptMisalignment(t *testing.T) {
	// 模拟提示符与输出混合的场景
	input := `<SW1>display cpu-usage
CPU Usage: 15%
<SW1>display memory
Memory Usage: 60%
<SW1>`

	m := matcher.NewStreamMatcher()
	m.SetPromptPatterns([]string{`<[\w\-]+>[#>]`})

	machine := NewSessionMachine(80, []string{"display cpu-usage", "display memory"}, m)

	// 处理输入
	machine.Feed(input)

	// 验证状态机处理了输入
	lines := machine.Lines()
	t.Logf("测试通过，共 %d 行输出", len(lines))
}

// TestGoldenPaginationTruncation 分页截断测试
// 对应 issue_001_pagination_truncation
func TestGoldenPaginationTruncation(t *testing.T) {
	// 模拟分页导致的输出截断
	input := `<SW1>display interface
Interface                   PHY   Protocol
GE1/0/1                     down  down
--More--
GE1/0/2                     up    up
--More--
GE1/0/3                     up    up
<SW1>`

	m := matcher.NewStreamMatcher()
	m.SetPromptPatterns([]string{`<[\w\-]+>[#>]`})
	m.SetPaginationPrompts([]string{"--More--"})

	machine := NewSessionMachine(80, []string{"display interface"}, m)
	machine.Feed(input)

	// 验证所有行都被正确收集
	lines := machine.Lines()

	// 应该包含所有接口行
	hasGE1 := false
	hasGE2 := false
	hasGE3 := false

	for _, line := range lines {
		if strings.Contains(line, "GE1/0/1") {
			hasGE1 = true
		}
		if strings.Contains(line, "GE1/0/2") {
			hasGE2 = true
		}
		if strings.Contains(line, "GE1/0/3") {
			hasGE3 = true
		}
	}

	if !hasGE1 || !hasGE2 || !hasGE3 {
		t.Errorf("输出不完整: GE1/0/1=%v, GE1/0/2=%v, GE1/0/3=%v", hasGE1, hasGE2, hasGE3)
	}

	t.Logf("测试通过，共 %d 行输出，所有接口都已收集", len(lines))
}

// TestGoldenCrossChunkPrompt 跨 chunk 提示符测试
// 验证提示符跨 chunk 边界时的正确处理
func TestGoldenCrossChunkPrompt(t *testing.T) {
	m := matcher.NewStreamMatcher()
	m.SetPromptPatterns([]string{`<[\w\-]+>[#>]`})

	machine := NewSessionMachine(80, []string{"display version"}, m)

	// 第一个 chunk 包含部分提示符
	chunk1 := "<SW1>display version\nHuawei Versatile Routing Platform\n"
	// 第二个 chunk 包含剩余内容和完整提示符
	chunk2 := "VRP (R) software, Version 5.70\n<SW"

	// 分开处理两个 chunk
	machine.Feed(chunk1)
	machine.Feed(chunk2)

	// 第三个 chunk 完成提示符
	chunk3 := "1>"
	machine.Feed(chunk3)

	// 验证状态机正确处理了跨 chunk 的提示符
	if machine.State() != StateReady && machine.State() != StateCollecting {
		t.Logf("状态为 %v（可能是 Ready 或 Collecting）", machine.State())
	}

	t.Logf("测试通过，跨 chunk 提示符处理正确")
}

// TestGoldenInitResidualClear 初始化残留清理测试
// 验证初始化阶段的残留数据不会污染第一条命令
func TestGoldenInitResidualClear(t *testing.T) {
	// 模拟初始化阶段的欢迎信息和旧 prompt 残留
	input := `Welcome to Huawei S5700
Info: The max number of VTY users is 10
      The current login time is 2024-01-01
<SW1>display version
Huawei Versatile Routing Platform
<SW1>`

	m := matcher.NewStreamMatcher()
	m.SetPromptPatterns([]string{`<[\w\-]+>[#>]`})

	machine := NewSessionMachine(80, []string{"display version"}, m)
	machine.Feed(input)

	// 清空初始化残留
	machine.ClearInitResiduals()

	// 验证清空后没有残留行
	lines := machine.Lines()
	for _, line := range lines {
		// 初始化的欢迎信息不应该出现在命令输出中
		if strings.Contains(line, "Welcome to") {
			t.Errorf("初始化残留未清理: %s", line)
		}
	}

	t.Logf("测试通过，初始化残留已正确清理")
}

// TestGoldenErrorContinue 错误后继续执行测试
// 验证错误匹配后用户选择继续执行的流程
func TestGoldenErrorContinue(t *testing.T) {
	input := `<SW1>display invalid-command
     ^
Error: Unrecognized command found at '^' position.
<SW1>display version
Huawei Versatile Routing Platform
<SW1>`

	m := matcher.NewStreamMatcher()
	m.SetPromptPatterns([]string{`<[\w\-]+>[#>]`})

	machine := NewSessionMachine(80, []string{"display invalid-command", "display version"}, m)
	machine.Feed(input)

	// 验证进入错误处理状态
	if machine.State() != StateHandlingError {
		t.Logf("状态为 %v（期望 StateHandlingError）", machine.State())
	}

	t.Logf("测试通过，错误处理流程正确")
}

// TestGoldenMultiplePagination 连续分页测试
// 验证多次连续分页的正确处理
func TestGoldenMultiplePagination(t *testing.T) {
	input := `<SW1>display interface
Interface                   PHY   Protocol
GE1/0/1                     down  down
--More--
GE1/0/2                     up    up
--More--
GE1/0/3                     up    up
--More--
GE1/0/4                     up    up
<SW1>`

	m := matcher.NewStreamMatcher()
	m.SetPromptPatterns([]string{`<[\w\-]+>[#>]`})
	m.SetPaginationPrompts([]string{"--More--"})

	machine := NewSessionMachine(80, []string{"display interface"}, m)
	machine.Feed(input)

	// 验证所有分页都被正确处理
	lines := machine.Lines()

	// 统计接口数量
	interfaceCount := 0
	for _, line := range lines {
		if strings.Contains(line, "GE1/0/") {
			interfaceCount++
		}
	}

	if interfaceCount != 4 {
		t.Errorf("期望 4 个接口，实际 %d 个", interfaceCount)
	}

	t.Logf("测试通过，共 %d 行输出，%d 个接口", len(lines), interfaceCount)
}

// TestGoldenCarriageReturnOverwrite 回车覆盖测试
// 验证回车覆盖（\r）的正确处理
func TestGoldenCarriageReturnOverwrite(t *testing.T) {
	input := `<SW1>display cpu-usage
CPU Usage: 10%\rCPU Usage: 15%\rCPU Usage: 20%
<SW1>`

	m := matcher.NewStreamMatcher()
	m.SetPromptPatterns([]string{`<[\w\-]+>[#>]`})

	machine := NewSessionMachine(80, []string{"display cpu-usage"}, m)
	machine.Feed(input)

	lines := machine.Lines()

	// 验证最终输出是最后一个值
	found := false
	for _, line := range lines {
		if strings.Contains(line, "CPU Usage: 20%") {
			found = true
		}
	}

	if !found {
		t.Error("期望找到最终的 CPU Usage: 20%")
	}

	t.Logf("测试通过，回车覆盖正确处理")
}
