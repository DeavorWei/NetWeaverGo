package executor

import (
	"strings"
	"testing"

	"github.com/NetWeaverGo/core/internal/matcher"
)

// TestSessionStateString 测试状态字符串表示
func TestSessionStateString(t *testing.T) {
	tests := []struct {
		state    SessionState
		expected string
	}{
		{StateWaitInitialPrompt, "WaitInitialPrompt"},
		{StateWarmup, "Warmup"},
		{StateReady, "Ready"},
		{StateSendCommand, "SendCommand"},
		{StateCollecting, "Collecting"},
		{StateHandlingPager, "HandlingPager"},
		{StateWaitingFinalPrompt, "WaitingFinalPrompt"},
		{StateCompleted, "Completed"},
		{StateFailed, "Failed"},
	}

	for _, tt := range tests {
		if got := tt.state.String(); got != tt.expected {
			t.Errorf("SessionState(%d).String() = %q, want %q", tt.state, got, tt.expected)
		}
	}
}

// TestSessionStateIsTerminal 测试终态判断
func TestSessionStateIsTerminal(t *testing.T) {
	terminalStates := []SessionState{StateCompleted, StateFailed}
	nonTerminalStates := []SessionState{
		StateWaitInitialPrompt, StateWarmup, StateReady,
		StateSendCommand, StateCollecting, StateHandlingPager, StateWaitingFinalPrompt,
	}

	for _, s := range terminalStates {
		if !s.IsTerminal() {
			t.Errorf("SessionState(%s).IsTerminal() = false, want true", s)
		}
	}

	for _, s := range nonTerminalStates {
		if s.IsTerminal() {
			t.Errorf("SessionState(%s).IsTerminal() = true, want false", s)
		}
	}
}

// TestCommandContext 测试命令上下文
func TestCommandContext(t *testing.T) {
	ctx := NewCommandContext(0, "display version")

	if ctx.Index != 0 {
		t.Errorf("Index = %d, want 0", ctx.Index)
	}
	if ctx.Command != "" {
		t.Errorf("Command = %q, want empty", ctx.Command)
	}
	if ctx.RawCommand != "display version" {
		t.Errorf("RawCommand = %q, want %q", ctx.RawCommand, "display version")
	}

	ctx.SetCommand("display version")
	if ctx.Command != "display version" {
		t.Errorf("Command = %q, want %q", ctx.Command, "display version")
	}

	ctx.AddNormalizedLine("line1")
	ctx.AddNormalizedLine("line2")
	if len(ctx.NormalizedLines) != 2 {
		t.Errorf("NormalizedLines count = %d, want 2", len(ctx.NormalizedLines))
	}

	text := ctx.NormalizedText()
	if text != "line1\nline2" {
		t.Errorf("NormalizedText = %q, want %q", text, "line1\nline2")
	}

	ctx.MarkCompleted()
	if !ctx.IsCompleted() {
		t.Error("IsCompleted() = false, want true")
	}
	if !ctx.PromptMatched {
		t.Error("PromptMatched = false, want true")
	}
}

// TestSessionMachineInitialState 测试状态机初始状态
func TestSessionMachineInitialState(t *testing.T) {
	m := matcher.NewStreamMatcher()
	machine := NewSessionMachine(80, []string{"cmd1", "cmd2"}, m)

	if machine.State() != StateWaitInitialPrompt {
		t.Errorf("Initial state = %s, want WaitInitialPrompt", machine.State())
	}
	if len(machine.queue) != 2 {
		t.Errorf("Queue length = %d, want 2", len(machine.queue))
	}
}

// TestSessionMachineWaitInitialPrompt 测试等待初始提示符状态
func TestSessionMachineWaitInitialPrompt(t *testing.T) {
	m := matcher.NewStreamMatcher()
	machine := NewSessionMachine(80, []string{"cmd1"}, m)

	// 模拟收到提示符
	actions := machine.Feed("<SW1>\n")

	if machine.State() != StateWarmup {
		t.Errorf("State after prompt = %s, want Warmup", machine.State())
	}

	// 应该返回发送预热的动作
	found := false
	for _, a := range actions {
		if a == ActionSendWarmup {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected ActionSendWarmup in actions")
	}
}

// TestSessionMachineWarmup 测试预热状态
func TestSessionMachineWarmup(t *testing.T) {
	m := matcher.NewStreamMatcher()
	machine := NewSessionMachine(80, []string{"cmd1"}, m)

	// 先进入预热状态
	machine.Feed("<SW1>\n")
	if machine.State() != StateWarmup {
		t.Fatalf("State = %s, want Warmup", machine.State())
	}

	// 模拟预热后收到提示符
	machine.Feed("<SW1>\n")

	if machine.State() != StateReady {
		t.Errorf("State after warmup = %s, want Ready", machine.State())
	}
}

// TestSessionMachineReadyToSendCommand 测试就绪状态发送命令
func TestSessionMachineReadyToSendCommand(t *testing.T) {
	m := matcher.NewStreamMatcher()
	machine := NewSessionMachine(80, []string{"display version"}, m)

	// 进入就绪状态
	machine.Feed("<SW1>\n") // 初始提示符 -> Warmup
	machine.Feed("<SW1>\n") // 预热后提示符 -> Ready

	if machine.State() != StateReady {
		t.Fatalf("State = %s, want Ready", machine.State())
	}

	// 触发发送命令（空 chunk 也会触发 Ready 状态处理）
	actions := machine.Feed("")

	// StateSendCommand 是瞬时状态，直接转换到 StateCollecting
	if machine.State() != StateCollecting {
		t.Errorf("State = %s, want Collecting", machine.State())
	}

	// 应该返回发送命令的动作
	found := false
	for _, a := range actions {
		if a == ActionSendCommand {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected ActionSendCommand in actions")
	}

	// 检查当前命令上下文
	cmd := machine.CurrentCommand()
	if cmd == nil {
		t.Fatal("CurrentCommand() = nil")
	}
	if cmd.Command != "display version" {
		t.Errorf("Command = %q, want %q", cmd.Command, "display version")
	}
}

// TestSessionMachineMarkFailed 测试失败标记
func TestSessionMachineMarkFailed(t *testing.T) {
	m := matcher.NewStreamMatcher()
	machine := NewSessionMachine(80, []string{"cmd1"}, m)

	machine.MarkFailed("test error")

	if machine.State() != StateFailed {
		t.Errorf("State = %s, want Failed", machine.State())
	}

	results := machine.Results()
	if len(results) != 1 {
		t.Errorf("Results count = %d, want 1", len(results))
	}
	if results[0].Success {
		t.Error("Result.Success = true, want false")
	}
	if results[0].ErrorMessage != "test error" {
		t.Errorf("ErrorMessage = %q, want %q", results[0].ErrorMessage, "test error")
	}
}

// TestSessionMachineReset 测试重置
func TestSessionMachineReset(t *testing.T) {
	m := matcher.NewStreamMatcher()
	machine := NewSessionMachine(80, []string{"cmd1"}, m)

	// 进入就绪状态
	machine.Feed("<SW1>\n")
	machine.Feed("<SW1>\n")

	// 重置
	machine.Reset()

	if machine.State() != StateWaitInitialPrompt {
		t.Errorf("State after reset = %s, want WaitInitialPrompt", machine.State())
	}
	if machine.nextIndex != 0 {
		t.Errorf("nextIndex after reset = %d, want 0", machine.nextIndex)
	}
	if len(machine.results) != 0 {
		t.Errorf("results count after reset = %d, want 0", len(machine.results))
	}
}

// TestParseInlineCommand 测试内联命令解析
func TestParseInlineCommand(t *testing.T) {
	tests := []struct {
		rawCmd      string
		wantCmd     string
		wantTimeout bool
	}{
		{
			rawCmd:      "display version",
			wantCmd:     "display version",
			wantTimeout: false,
		},
		{
			rawCmd:      "display startup // nw-timeout=30s",
			wantCmd:     "display startup",
			wantTimeout: true,
		},
		{
			rawCmd:      "long command // nw-timeout=1m30s",
			wantCmd:     "long command",
			wantTimeout: true,
		},
	}

	for _, tt := range tests {
		cmd, timeout := parseInlineCommand(tt.rawCmd)
		if cmd != tt.wantCmd {
			t.Errorf("parseInlineCommand(%q) cmd = %q, want %q", tt.rawCmd, cmd, tt.wantCmd)
		}
		hasTimeout := timeout > 0
		if hasTimeout != tt.wantTimeout {
			t.Errorf("parseInlineCommand(%q) hasTimeout = %v, want %v", tt.rawCmd, hasTimeout, tt.wantTimeout)
		}
	}
}

// TestActionData 测试动作数据
func TestActionData(t *testing.T) {
	m := matcher.NewStreamMatcher()
	machine := NewSessionMachine(80, []string{"display version"}, m)

	// 进入就绪状态
	machine.Feed("<SW1>\n")
	machine.Feed("<SW1>\n")

	// 触发发送命令
	machine.Feed("")

	// 获取动作数据
	actionData := machine.GetActionData(ActionSendCommand)
	if actionData.Type != ActionSendCommand {
		t.Errorf("ActionData.Type = %d, want %d", actionData.Type, ActionSendCommand)
	}
	if actionData.Command != "display version" {
		t.Errorf("ActionData.Command = %q, want %q", actionData.Command, "display version")
	}
}

// TestSessionMachineLines 测试逻辑行收集
func TestSessionMachineLines(t *testing.T) {
	m := matcher.NewStreamMatcher()
	machine := NewSessionMachine(80, []string{"cmd1"}, m)

	// 输入多行文本
	machine.Feed("line1\nline2\nline3\n")

	lines := machine.Lines()
	if len(lines) != 3 {
		t.Errorf("Lines count = %d, want 3", len(lines))
	}

	expected := []string{"line1", "line2", "line3"}
	for i, line := range lines {
		if line != expected[i] {
			t.Errorf("Lines[%d] = %q, want %q", i, line, expected[i])
		}
	}
}

// TestSessionMachineActiveLine 测试活动行
func TestSessionMachineActiveLine(t *testing.T) {
	m := matcher.NewStreamMatcher()
	machine := NewSessionMachine(80, []string{"cmd1"}, m)

	// 输入未完成的行
	machine.Feed("partial line")

	active := machine.ActiveLine()
	if !strings.Contains(active, "partial line") {
		t.Errorf("ActiveLine = %q, should contain %q", active, "partial line")
	}
}

// TestCommandResult 测试命令结果转换
func TestCommandResult(t *testing.T) {
	ctx := NewCommandContext(0, "display version")
	ctx.SetCommand("display version")
	ctx.AddNormalizedLine("Version 1.0")
	ctx.AddNormalizedLine("Build 123")
	ctx.MarkCompleted()

	result := ctx.ToResult()

	if result.Index != 0 {
		t.Errorf("Result.Index = %d, want 0", result.Index)
	}
	if result.Command != "display version" {
		t.Errorf("Result.Command = %q, want %q", result.Command, "display version")
	}
	if !result.Success {
		t.Error("Result.Success = false, want true")
	}
	if result.NormalizedText != "Version 1.0\nBuild 123" {
		t.Errorf("Result.NormalizedText = %q, want %q", result.NormalizedText, "Version 1.0\nBuild 123")
	}
	if len(result.NormalizedLines) != 2 {
		t.Errorf("Result.NormalizedLines count = %d, want 2", len(result.NormalizedLines))
	}
}

// TestCommandContextError 测试命令上下文错误标记
func TestCommandContextError(t *testing.T) {
	ctx := NewCommandContext(0, "bad command")
	ctx.MarkFailed("command failed")

	if !ctx.HasError() {
		t.Error("HasError() = false, want true")
	}
	if ctx.ErrorMessage != "command failed" {
		t.Errorf("ErrorMessage = %q, want %q", ctx.ErrorMessage, "command failed")
	}

	result := ctx.ToResult()
	if result.Success {
		t.Error("Result.Success = true, want false")
	}
	if result.ErrorMessage != "command failed" {
		t.Errorf("Result.ErrorMessage = %q, want %q", result.ErrorMessage, "command failed")
	}
}

// TestCommandContextPagination 测试分页计数
func TestCommandContextPagination(t *testing.T) {
	ctx := NewCommandContext(0, "display interface")

	if ctx.PaginationCount != 0 {
		t.Errorf("Initial PaginationCount = %d, want 0", ctx.PaginationCount)
	}

	ctx.IncrementPagination()
	if ctx.PaginationCount != 1 {
		t.Errorf("PaginationCount after increment = %d, want 1", ctx.PaginationCount)
	}

	ctx.IncrementPagination()
	ctx.IncrementPagination()
	if ctx.PaginationCount != 3 {
		t.Errorf("PaginationCount after 3 increments = %d, want 3", ctx.PaginationCount)
	}
}

// TestCommandContextDuration 测试执行时长
func TestCommandContextDuration(t *testing.T) {
	ctx := NewCommandContext(0, "test")

	// 未完成时，Duration 应该返回从开始到现在的时间
	d := ctx.Duration()
	if d < 0 {
		t.Errorf("Duration = %v, should be >= 0", d)
	}

	// 完成后，Duration 应该返回固定值
	ctx.MarkCompleted()
	d2 := ctx.Duration()
	if d2 < 0 {
		t.Errorf("Duration after completion = %v, should be >= 0", d2)
	}
}

// TestAcceptancePaginationNoContextLoss 验收测试：确保分页续页后无上下文丢失
// 这是 Phase 1 和 Phase 2 的核心验收测试
func TestAcceptancePaginationNoContextLoss(t *testing.T) {
	// 场景：模拟华为设备 display interface brief 输出
	// 分页符 "---- More ----" 出现后被空格覆盖，继续输出下一行
	// 验证：最终输出中不包含分页符，所有接口信息完整

	m := matcher.NewStreamMatcher()
	machine := NewSessionMachine(80, []string{"display interface brief"}, m)

	// 模拟设备初始提示符和命令回显
	machine.Feed("<SW1>display interface brief\n")

	// 模拟表头输出
	header := "Interface                   PHY   Protocol  Description\n"
	machine.Feed(header)

	// 模拟第一行接口信息
	line1 := "GE1/0/1                     down  down      Uplink-1\n"
	machine.Feed(line1)

	// 模拟分页符出现并被覆盖（核心测试场景）
	// "---- More ----" 被光标左移 + 空格覆盖 + 光标左移 + 新内容覆盖
	paginationOverwrite := "---- More ----\x1b[16D                \x1b[16D"
	machine.Feed(paginationOverwrite)

	// 模拟第二行接口信息（覆盖在分页符位置）
	line2 := "GE1/0/2                     up    up        Uplink-2\n"
	machine.Feed(line2)

	// 模拟再次出现分页符并覆盖
	machine.Feed("---- More ----\x1b[16D                \x1b[16D")

	// 模拟第三行接口信息
	line3 := "GE1/0/3                     up    up        Uplink-3\n"
	machine.Feed(line3)

	// 模拟最终提示符
	machine.Feed("<SW1>")

	// 获取所有逻辑行
	lines := machine.Lines()

	// 验证点 0：命令回显行
	if len(lines) < 1 {
		t.Fatal("expected at least 1 line (command echo)")
	}
	if !strings.Contains(lines[0], "display interface brief") {
		t.Errorf("command echo line should contain 'display interface brief', got %q", lines[0])
	}

	// 验证点 1：表头应该完整保留
	if len(lines) < 2 {
		t.Fatal("expected at least 2 lines (command echo + header)")
	}
	if !strings.Contains(lines[1], "Interface") {
		t.Errorf("header line should contain 'Interface', got %q", lines[1])
	}

	// 验证点 2：第一行接口信息应该完整
	if len(lines) < 3 {
		t.Fatal("expected at least 3 lines (command echo + header + line1)")
	}
	if !strings.Contains(lines[2], "GE1/0/1") {
		t.Errorf("line 2 should contain 'GE1/0/1', got %q", lines[2])
	}
	if !strings.Contains(lines[2], "down") {
		t.Errorf("line 2 should contain 'down', got %q", lines[2])
	}

	// 验证点 3：第二行接口信息应该完整，且不包含分页符
	if len(lines) < 4 {
		t.Fatal("expected at least 4 lines")
	}
	if strings.Contains(lines[3], "---- More") {
		t.Errorf("line 3 should NOT contain '---- More', got %q", lines[3])
	}
	if !strings.Contains(lines[3], "GE1/0/2") {
		t.Errorf("line 3 should contain 'GE1/0/2', got %q", lines[3])
	}
	if !strings.Contains(lines[3], "up") {
		t.Errorf("line 3 should contain 'up', got %q", lines[3])
	}

	// 验证点 4：第三行接口信息应该完整
	if len(lines) < 5 {
		t.Fatal("expected at least 5 lines")
	}
	if strings.Contains(lines[4], "---- More") {
		t.Errorf("line 4 should NOT contain '---- More', got %q", lines[4])
	}
	if !strings.Contains(lines[4], "GE1/0/3") {
		t.Errorf("line 4 should contain 'GE1/0/3', got %q", lines[4])
	}

	// 验证点 5：活动行应该包含最终提示符
	activeLine := machine.ActiveLine()
	if !strings.Contains(activeLine, "<SW1>") {
		t.Errorf("active line should contain '<SW1>', got %q", activeLine)
	}

	// 核心验收：所有行都不应该包含分页符
	for i, line := range lines {
		if strings.Contains(line, "---- More") {
			t.Errorf("line[%d] should NOT contain '---- More', got %q", i, line)
		}
	}

	t.Logf("验收测试通过：共 %d 行输出，分页符已正确处理", len(lines))
	for i, line := range lines {
		t.Logf("  line[%d]: %q", i, line)
	}
}

// TestAcceptanceMultiplePaginationSequence 验收测试：连续多次分页场景
func TestAcceptanceMultiplePaginationSequence(t *testing.T) {
	// 场景：模拟长输出，连续多次分页
	// 验证：每次分页后上下文都正确保留

	m := matcher.NewStreamMatcher()
	machine := NewSessionMachine(80, []string{"display version"}, m)

	// 模拟命令回显
	machine.Feed("<SW1>display version\n")

	// 模拟版本信息第一部分
	machine.Feed("Huawei Versatile Routing Platform Software\n")
	machine.Feed("VRP (R) software, Version 5.70\n")

	// 第一次分页
	machine.Feed("---- More ----\x1b[16D                \x1b[16D")

	// 继续版本信息
	machine.Feed("Copyright (C) 2003-2010 Huawei Technologies Co., Ltd.\n")

	// 第二次分页
	machine.Feed("---- More ----\x1b[16D                \x1b[16D")

	// 继续版本信息
	machine.Feed("Quidway S5300 Series Ethernet Switch\n")

	// 第三次分页
	machine.Feed("---- More ----\x1b[16D                \x1b[16D")

	// 最后信息
	machine.Feed("uptime is 0 days, 0 hours, 5 minutes\n")

	// 最终提示符
	machine.Feed("<SW1>")

	lines := machine.Lines()

	// 验证：所有版本信息行都应该存在，且不包含分页符
	expectedContents := []string{
		"Huawei Versatile Routing Platform Software",
		"VRP (R) software, Version 5.70",
		"Copyright (C) 2003-2010 Huawei Technologies Co., Ltd.",
		"Quidway S5300 Series Ethernet Switch",
		"uptime is 0 days, 0 hours, 5 minutes",
	}

	for _, expected := range expectedContents {
		found := false
		for _, line := range lines {
			if strings.Contains(line, expected) {
				found = true
				// 确保不包含分页符
				if strings.Contains(line, "---- More") {
					t.Errorf("line containing '%s' should not contain '---- More', got %q", expected, line)
				}
				break
			}
		}
		if !found {
			t.Errorf("expected to find line containing '%s' in output", expected)
		}
	}

	// 验证：没有分页符残留
	for i, line := range lines {
		if strings.Contains(line, "---- More") {
			t.Errorf("line[%d] should not contain '---- More', got %q", i, line)
		}
	}

	t.Logf("连续分页验收测试通过：共 %d 行输出", len(lines))
}

// TestAcceptanceCarriageReturnOverwrite 验收测试：回车覆盖场景
func TestAcceptanceCarriageReturnOverwrite(t *testing.T) {
	// 场景：设备使用回车覆盖更新当前行（如进度条、状态更新）
	// 验证：最终输出只保留最后一次覆盖的内容

	m := matcher.NewStreamMatcher()
	machine := NewSessionMachine(80, []string{"test"}, m)

	// 模拟进度更新场景
	machine.Feed("Processing... 0%\rProcessing... 25%\rProcessing... 50%\rProcessing... 75%\rProcessing... 100%\n")
	machine.Feed("Done!\n")

	lines := machine.Lines()

	// 验证：应该有两行（最终进度和 Done）
	if len(lines) < 2 {
		t.Errorf("expected at least 2 lines, got %d", len(lines))
	}

	// 第一行应该是最终进度状态
	if len(lines) >= 1 {
		if !strings.Contains(lines[0], "Processing... 100%") {
			t.Errorf("first line should contain 'Processing... 100%%', got %q", lines[0])
		}
		// 不应该包含中间状态（检查 "0%" 在开头位置，排除 "100%" 中的 0）
		if strings.HasPrefix(lines[0], "Processing... 0%") {
			t.Errorf("first line should not start with 'Processing... 0%%', got %q", lines[0])
		}
	}

	// 第二行应该是 Done
	if len(lines) >= 2 {
		if !strings.Contains(lines[1], "Done") {
			t.Errorf("second line should contain 'Done', got %q", lines[1])
		}
	}

	t.Logf("回车覆盖验收测试通过")
}
