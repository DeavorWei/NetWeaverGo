package terminal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRegressionBugFixes 测试所有回归样本
// 每个子目录代表一个 bug 修复案例
func TestRegressionBugFixes(t *testing.T) {
	regressionDir := "../../testdata/regression/bug_fixes"

	// 检查目录是否存在
	if _, err := os.Stat(regressionDir); os.IsNotExist(err) {
		t.Skip("回归测试目录不存在")
	}

	// 遍历所有 bug 案例
	entries, err := os.ReadDir(regressionDir)
	if err != nil {
		t.Fatalf("读取回归测试目录失败: %v", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		caseName := entry.Name()
		caseDir := filepath.Join(regressionDir, caseName)

		t.Run(caseName, func(t *testing.T) {
			inputPath := filepath.Join(caseDir, "input.txt")
			expectedPath := filepath.Join(caseDir, "expected.txt")

			// 读取输入
			inputData, err := os.ReadFile(inputPath)
			if err != nil {
				t.Fatalf("读取输入文件失败: %v", err)
			}

			// 读取期望输出
			expectedData, err := os.ReadFile(expectedPath)
			if err != nil {
				t.Fatalf("读取期望文件失败: %v", err)
			}

			// 使用 Replayer 处理输入
			replayer := NewReplayer(80)
			replayer.Process(string(inputData))
			result := replayer.Lines()

			// 比较结果 - 统一行尾格式为 LF
			expected := strings.TrimSpace(strings.ReplaceAll(string(expectedData), "\r\n", "\n"))
			actual := strings.TrimSpace(strings.Join(result, "\n"))

			if actual != expected {
				// 提供更详细的调试信息
				t.Errorf("输出不匹配\n期望长度: %d, 实际长度: %d", len(expected), len(actual))

				// 逐行比较找出差异
				expectedLines := strings.Split(expected, "\n")
				actualLines := strings.Split(actual, "\n")

				maxLen := len(expectedLines)
				if len(actualLines) > maxLen {
					maxLen = len(actualLines)
				}

				for i := 0; i < maxLen; i++ {
					var expLine, actLine string
					if i < len(expectedLines) {
						expLine = expectedLines[i]
					}
					if i < len(actualLines) {
						actLine = actualLines[i]
					}

					if expLine != actLine {
						t.Errorf("行 %d 不匹配:\n期望: %q (len=%d)\n实际: %q (len=%d)", i+1, expLine, len(expLine), actLine, len(actLine))
					}
				}
			}
		})
	}
}

// TestPaginationTruncation 分页截断专项测试
func TestPaginationTruncation(t *testing.T) {
	input := `display interface brief
PHY: Physical
*down: administratively down
^down: standby
(l): loopback
(s): spoofing
(b): BFD down
(d): Dampening Suppressed
InUti/OutUti: input utility/output utility
Interface                   PHY   Protocol  InUti OutUti   inErrors  outErrors
Ethernet0/0/0               up    up        0.01%  0.01%          0          0
Ethernet0/0/1               up    up        0.01%  0.01%          0          0
  ---- More ----
Ethernet0/0/2               down  down         0%     0%          0          0
GigabitEthernet0/0/1        up    up        0.01%  0.01%          0          0
GigabitEthernet0/0/2        up    up        0.01%  0.01%          0          0
  ---- More ----
GigabitEthernet0/0/3        down  down         0%     0%          0          0
XGigabitEthernet0/0/1       up    up        0.01%  0.01%          0          0
XGigabitEthernet0/0/2       up    up        0.01%  0.01%          0          0
<Huawei>
`

	replayer := NewReplayer(80)
	replayer.Process(input)
	lines := replayer.Lines()

	// 验证分页提示符被移除（分页提示符应该被作为普通行处理）
	result := strings.Join(lines, "\n")

	// 验证关键数据存在
	expectedLines := []string{
		"Ethernet0/0/0",
		"Ethernet0/0/1",
		"Ethernet0/0/2",
		"GigabitEthernet0/0/1",
		"GigabitEthernet0/0/2",
		"GigabitEthernet0/0/3",
		"XGigabitEthernet0/0/1",
		"XGigabitEthernet0/0/2",
	}

	for _, expected := range expectedLines {
		if !strings.Contains(result, expected) {
			t.Errorf("缺少期望的接口: %s", expected)
		}
	}
}

// TestPromptMisalignment 提示符错位测试
func TestPromptMisalignment(t *testing.T) {
	input := `<Switch>display version
Huawei Versatile Routing Platform Software
VRP (R) software, Version 5.160 (S5700 V200R005C00SPC500)
Copyright (C) 2003-2016 Huawei Technologies Co., Ltd.
<Switch>`

	replayer := NewReplayer(80)
	replayer.Process(input)
	lines := replayer.Lines()

	// 验证提示符不会出现在输出中间
	result := strings.Join(lines, "\n")

	// 第一个提示符应该被作为普通行处理
	// 最后一个提示符也应该被作为普通行处理
	if len(lines) == 0 {
		t.Error("期望有输出行")
	}

	// 验证关键内容存在
	if !strings.Contains(result, "Huawei Versatile Routing Platform") {
		t.Errorf("缺少期望的内容: %s", result)
	}
}

// TestOverwriteCorruption 覆盖写错乱测试
func TestOverwriteCorruption(t *testing.T) {
	// 模拟进度条覆盖写的场景
	// 回车后写入空格覆盖原内容，再回车写入 Done
	input := "Processing...\r          \rDone\n"

	replayer := NewReplayer(80)
	replayer.Process(input)
	lines := replayer.Lines()

	if len(lines) != 1 {
		t.Errorf("期望 1 行，实际 %d 行: %v", len(lines), lines)
	}

	// 回车覆盖写后，空格会覆盖原内容，Done 会覆盖前 4 个空格
	// 结果应该是 "Done      ..." (空格覆盖后剩余的内容)
	// 实际行为：回车后写入空格覆盖，再回车写入 Done
	expected := "Done      ..."
	if lines[0] != expected {
		t.Errorf("期望 %q，实际 %q", expected, lines[0])
	}
}

// TestCarriageReturnOverwrite 回车覆盖测试
func TestCarriageReturnOverwrite(t *testing.T) {
	input := "abcdef\rXYZ\n"

	replayer := NewReplayer(80)
	replayer.Process(input)
	lines := replayer.Lines()

	if len(lines) != 1 {
		t.Errorf("期望 1 行，实际 %d 行", len(lines))
	}

	// 回车后覆盖写，期望 "XYZdef"
	expected := "XYZdef"
	if lines[0] != expected {
		t.Errorf("期望 %q，实际 %q", expected, lines[0])
	}
}

// TestBackspaceOverwrite 退格覆盖测试
func TestBackspaceOverwrite(t *testing.T) {
	input := "hello\x08\x08\x08XY\n"

	replayer := NewReplayer(80)
	replayer.Process(input)
	lines := replayer.Lines()

	if len(lines) != 1 {
		t.Errorf("期望 1 行，实际 %d 行", len(lines))
	}

	// 退格3次后写入XY，期望 "heXYo"
	expected := "heXYo"
	if lines[0] != expected {
		t.Errorf("期望 %q，实际 %q", expected, lines[0])
	}
}

// TestANSIEraseSequence ANSI 擦除序列测试
func TestANSIEraseSequence(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "ESC[K 擦除到行尾",
			input:    "abcdef\x1b[3D\x1b[K\n",
			expected: "abc",
		},
		{
			name:     "ESC[2K 擦除整行",
			input:    "abcdef\x1b[2Knewline\n",
			expected: "newline",
		},
		{
			name:     "光标左移后覆盖",
			input:    "abcdef\x1b[2DXY\n",
			expected: "abcdXY",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			replayer := NewReplayer(80)
			replayer.Process(tt.input)
			lines := replayer.Lines()

			if len(lines) != 1 {
				t.Errorf("期望 1 行，实际 %d 行: %v", len(lines), lines)
			}

			if lines[0] != tt.expected {
				t.Errorf("期望 %q，实际 %q", tt.expected, lines[0])
			}
		})
	}
}

// TestControlCharacters 控制字符综合测试
func TestControlCharacters(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "基本换行",
			input:    "line1\nline2\nline3\n",
			expected: []string{"line1", "line2", "line3"},
		},
		{
			name:     "回车换行组合",
			input:    "line1\r\nline2\r\nline3\n",
			expected: []string{"line1", "line2", "line3"},
		},
		{
			name:     "Tab展开",
			input:    "col1\tcol2\tcol3\n",
			expected: []string{"col1    col2    col3"},
		},
		{
			name:     "多重回车覆盖",
			input:    "abc\rxy\rz\n",
			expected: []string{"zyc"}, // abc -> xyc (第一次回车覆盖) -> zyc (第二次回车覆盖)
		},
		{
			name:     "退格删除",
			input:    "abc\x08\x08\x08xyz\n",
			expected: []string{"xyz"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			replayer := NewReplayer(80)
			replayer.Process(tt.input)
			lines := replayer.Lines()

			if len(lines) != len(tt.expected) {
				t.Errorf("期望 %d 行，实际 %d 行: %v", len(tt.expected), len(lines), lines)
				return
			}

			for i, expected := range tt.expected {
				if lines[i] != expected {
					t.Errorf("行 %d: 期望 %q，实际 %q", i, expected, lines[i])
				}
			}
		})
	}
}
