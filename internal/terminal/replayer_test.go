package terminal

import (
	"strings"
	"testing"
)

// TestLineBufferBasic 测试 LineBuffer 基本功能
func TestLineBufferBasic(t *testing.T) {
	tests := []struct {
		name     string
		ops      func(*LineBuffer)
		expected string
	}{
		{
			name: "基本写入",
			ops: func(lb *LineBuffer) {
				lb.Put('a')
				lb.Put('b')
				lb.Put('c')
			},
			expected: "abc",
		},
		{
			name: "覆盖写",
			ops: func(lb *LineBuffer) {
				lb.Put('a')
				lb.Put('b')
				lb.Put('c')
				lb.MoveLeft(1)
				lb.Put('X')
			},
			expected: "abX",
		},
		{
			name: "回车覆盖",
			ops: func(lb *LineBuffer) {
				lb.Put('a')
				lb.Put('b')
				lb.Put('c')
				lb.CarriageReturn()
				lb.Put('X')
				lb.Put('Y')
			},
			expected: "XYc",
		},
		{
			name: "退格覆盖",
			ops: func(lb *LineBuffer) {
				lb.Put('h')
				lb.Put('e')
				lb.Put('l')
				lb.Put('l')
				lb.Put('o')
				lb.Backspace()
				lb.Backspace()
				lb.Put('X')
				lb.Put('Y')
			},
			expected: "helXY",
		},
		{
			name: "擦除到行尾",
			ops: func(lb *LineBuffer) {
				lb.Put('a')
				lb.Put('b')
				lb.Put('c')
				lb.Put('d')
				lb.Put('e')
				lb.MoveLeft(2)
				lb.EraseToEnd()
			},
			expected: "abc",
		},
		{
			name: "擦除整行",
			ops: func(lb *LineBuffer) {
				lb.Put('a')
				lb.Put('b')
				lb.Put('c')
				lb.EraseAll()
			},
			expected: "",
		},
		{
			name: "光标边界-左移不超过行首",
			ops: func(lb *LineBuffer) {
				lb.Put('a')
				lb.MoveLeft(10)
				lb.Put('X')
			},
			expected: "X",
		},
		{
			name: "光标边界-右移不超过行尾",
			ops: func(lb *LineBuffer) {
				lb.Put('a')
				lb.MoveRight(10)
				lb.Put('b')
			},
			expected: "ab",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lb := NewLineBuffer()
			tt.ops(lb)
			if got := lb.String(); got != tt.expected {
				t.Errorf("got %q, want %q", got, tt.expected)
			}
		})
	}
}

// TestANSIParserBasic 测试 ANSI 解析器基本功能
func TestANSIParserBasic(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		wantTextCount  int
		wantCmdCount   int
		wantCmdType    CommandType
		wantFirstParam int
	}{
		{
			name:          "纯文本",
			input:         "hello",
			wantTextCount: 1,
			wantCmdCount:  0,
		},
		{
			name:           "光标左移",
			input:          "\x1b[3D",
			wantTextCount:  0,
			wantCmdCount:   1,
			wantCmdType:    CmdCursorLeft,
			wantFirstParam: 3,
		},
		{
			name:          "擦除到行尾",
			input:         "\x1b[K",
			wantTextCount: 0,
			wantCmdCount:  1,
			wantCmdType:   CmdEraseInLine,
		},
		{
			name:           "混合内容",
			input:          "abc\x1b[2DXY",
			wantTextCount:  2,
			wantCmdCount:   1,
			wantCmdType:    CmdCursorLeft,
			wantFirstParam: 2,
		},
		{
			name:          "SGR样式",
			input:         "\x1b[0m",
			wantTextCount: 0,
			wantCmdCount:  1,
			wantCmdType:   CmdSGR,
		},
		{
			name:          "擦除整行",
			input:         "\x1b[2K",
			wantTextCount: 0,
			wantCmdCount:  1,
			wantCmdType:   CmdEraseInLine,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewANSIParser()
			tokens := p.Parse(tt.input)

			textCount := 0
			cmdCount := 0
			var firstCmd ANSICommand

			for _, token := range tokens {
				if token.IsText {
					textCount++
				} else {
					cmdCount++
					if cmdCount == 1 {
						firstCmd = token.Cmd
					}
				}
			}

			if textCount != tt.wantTextCount {
				t.Errorf("text token count = %d, want %d", textCount, tt.wantTextCount)
			}

			if cmdCount != tt.wantCmdCount {
				t.Errorf("command count = %d, want %d", cmdCount, tt.wantCmdCount)
			}

			if tt.wantCmdCount > 0 {
				if firstCmd.Type != tt.wantCmdType {
					t.Errorf("command type = %v, want %v", firstCmd.Type, tt.wantCmdType)
				}
				if tt.wantFirstParam > 0 && len(firstCmd.Params) > 0 {
					if firstCmd.Params[0] != tt.wantFirstParam {
						t.Errorf("first param = %d, want %d", firstCmd.Params[0], tt.wantFirstParam)
					}
				}
			}
		})
	}
}

// TestANSIParserUnknown 测试未支持序列
func TestANSIParserUnknown(t *testing.T) {
	p := NewANSIParser()
	tokens := p.Parse("\x1b[5A") // 光标上移，第一版不支持

	cmdCount := 0
	for _, token := range tokens {
		if !token.IsText {
			cmdCount++
		}
	}

	if cmdCount != 1 {
		t.Fatalf("expected 1 command token, got %d", cmdCount)
	}

	for _, token := range tokens {
		if !token.IsText {
			if token.Cmd.Type != CmdCursorUp {
				t.Errorf("command type = %v, want CmdCursorUp", token.Cmd.Type)
			}
		}
	}

	if p.UnknownCount() != 1 {
		t.Errorf("unknown count = %d, want 1", p.UnknownCount())
	}
}

// TestReplayerBasic 测试 Replayer 基础功能
func TestReplayerBasic(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantLines  []string
		wantActive string
	}{
		{
			name:       "回车覆盖",
			input:      "abc\rXYZ",
			wantLines:  []string{},
			wantActive: "XYZ",
		},
		{
			name:       "回车覆盖并换行",
			input:      "abc\rXYZ\n",
			wantLines:  []string{"XYZ"},
			wantActive: "",
		},
		{
			name:       "退格覆盖",
			input:      "hello\b\bXY",
			wantLines:  []string{},
			wantActive: "helXY",
		},
		{
			name:       "光标左移后覆盖",
			input:      "abcdef\x1b[3D123",
			wantLines:  []string{},
			wantActive: "abc123",
		},
		{
			name:       "多行文本",
			input:      "line1\nline2\nline3",
			wantLines:  []string{"line1", "line2"},
			wantActive: "line3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewReplayer(80)
			r.Process(tt.input)

			lines := r.Lines()
			if len(lines) != len(tt.wantLines) {
				t.Errorf("lines count = %d, want %d", len(lines), len(tt.wantLines))
			}
			for i, want := range tt.wantLines {
				if i < len(lines) && lines[i] != want {
					t.Errorf("lines[%d] = %q, want %q", i, lines[i], want)
				}
			}

			if active := r.ActiveLine(); active != tt.wantActive {
				t.Errorf("active line = %q, want %q", active, tt.wantActive)
			}
		})
	}
}

// TestReplayerPagination 测试分页覆盖写场景
func TestReplayerPagination(t *testing.T) {
	// 设计文档中的故障样本
	// "---- More ----\x1b[16D                \x1b[16DGE1/0/8"
	// 语义：
	// 1. 输出 "---- More ----" (13个字符)
	// 2. 光标左移 16 列（超过行首，停在行首）
	// 3. 输出 16 个空格覆盖
	// 4. 光标再次左移 16 列（回到行首）
	// 5. 输出 "GE1/0/8" 覆盖前 7 个空格
	// 最终结果应该是 "GE1/0/8" + 末尾空格

	r := NewReplayer(80)
	r.Process("---- More ----\x1b[16D                \x1b[16DGE1/0/8")

	active := r.ActiveLine()

	// 核心验证：分页符 "---- More ----" 应该被完全覆盖
	if strings.Contains(active, "---- More") {
		t.Errorf("active line should not contain '---- More', got %q", active)
	}

	// 核心验证：应该包含 "GE1/0/8"
	if !strings.HasPrefix(active, "GE1/0/8") {
		t.Errorf("active line should start with 'GE1/0/8', got %q", active)
	}

	// 验证 trim 后的结果（用户视角）
	trimmed := strings.TrimSpace(active)
	if trimmed != "GE1/0/8" {
		t.Errorf("trimmed active line = %q, want %q", trimmed, "GE1/0/8")
	}
}

// TestReplayerRealSample 测试真实故障样本
func TestReplayerRealSample(t *testing.T) {
	// 模拟真实分页场景：
	// 设备输出 "PHY: Physical" 后被分页符覆盖
	// 分页符 "---- More ----" 被空格覆盖后显示实际内容

	r := NewReplayer(80)

	// 模拟分页覆盖写
	input := "PHY: Physical\n---- More ----\x1b[16D                \x1b[16DGE1/0/8 up  up       1000M  1000M"
	r.Process(input)

	lines := r.Lines()

	// 第一行应该是 "PHY: Physical"
	if len(lines) < 1 {
		t.Fatal("expected at least 1 committed line")
	}
	if lines[0] != "PHY: Physical" {
		t.Errorf("first line = %q, want %q", lines[0], "PHY: Physical")
	}

	// 活动行应该包含 "GE1/0/8" 而不是 "---- More ----"
	active := r.ActiveLine()
	if strings.Contains(active, "---- More") {
		t.Errorf("active line should not contain '---- More', got %q", active)
	}
	if !strings.Contains(active, "GE1/0/8") {
		t.Errorf("active line should contain 'GE1/0/8', got %q", active)
	}
}

// TestReplayerEraseSequence 测试擦除序列
func TestReplayerEraseSequence(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantActive string
	}{
		{
			name:       "ESC[K 擦除到行尾",
			input:      "abcdef\x1b[3D\x1b[K",
			wantActive: "abc",
		},
		{
			name:       "ESC[2K 擦除整行",
			input:      "abcdef\x1b[2K",
			wantActive: "",
		},
		{
			name:       "擦除后重写",
			input:      "old text\x1b[2Knew text",
			wantActive: "new text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewReplayer(80)
			r.Process(tt.input)

			if active := r.ActiveLine(); active != tt.wantActive {
				t.Errorf("active line = %q, want %q", active, tt.wantActive)
			}
		})
	}
}

// TestReplayerTab 测试 Tab 展开
func TestReplayerTab(t *testing.T) {
	r := NewReplayer(80)
	r.Process("\tX")

	// Tab 应该展开为 8 个空格，然后写入 X
	expected := "        X"
	if active := r.ActiveLine(); active != expected {
		t.Errorf("active line = %q, want %q", active, expected)
	}
}

// TestReplayerReset 测试重置功能
func TestReplayerReset(t *testing.T) {
	r := NewReplayer(80)
	r.Process("line1\nline2\n")

	if len(r.Lines()) != 2 {
		t.Errorf("expected 2 lines before reset, got %d", len(r.Lines()))
	}

	r.Reset()

	if len(r.Lines()) != 0 {
		t.Errorf("expected 0 lines after reset, got %d", len(r.Lines()))
	}
	if r.ActiveLine() != "" {
		t.Errorf("expected empty active line after reset, got %q", r.ActiveLine())
	}
}

// TestReplayerUnknownCount 测试未支持序列计数
func TestReplayerUnknownCount(t *testing.T) {
	r := NewReplayer(80)

	// 光标上移和光标下移是未支持的
	r.Process("\x1b[5A\x1b[3B")

	if count := r.UnknownCount(); count != 2 {
		t.Errorf("unknown count = %d, want 2", count)
	}
}

// TestReplayerComplexScenario 测试复杂场景
func TestReplayerComplexScenario(t *testing.T) {
	// 模拟华为设备显示接口详细信息的分页场景
	r := NewReplayer(80)

	// 第一行正常输出
	r.Process("Interface                   PHY   Protocol  Description\n")

	// 第二行被分页符覆盖
	r.Process("GE1/0/1                     down  down      Uplink-1\n")

	// 分页符出现并被覆盖
	r.Process("---- More ----\x1b[16D                \x1b[16D")

	// 继续输出下一行
	r.Process("GE1/0/2                     up    up        Uplink-2\n")

	lines := r.Lines()

	// 验证所有行都正确提交
	expectedLines := []string{
		"Interface                   PHY   Protocol  Description",
		"GE1/0/1                     down  down      Uplink-1",
		"GE1/0/2                     up    up        Uplink-2",
	}

	if len(lines) != len(expectedLines) {
		t.Errorf("lines count = %d, want %d", len(lines), len(expectedLines))
	}

	for i, want := range expectedLines {
		if i < len(lines) && lines[i] != want {
			t.Errorf("lines[%d] = %q, want %q", i, lines[i], want)
		}
	}
}
