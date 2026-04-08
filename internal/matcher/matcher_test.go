package matcher

import "testing"

func TestIsPromptRejectsStandaloneConfigSeparator(t *testing.T) {
	m := NewStreamMatcher()

	if m.IsPrompt("#\r\n") {
		t.Fatalf("standalone # should not be treated as a device prompt")
	}
}

func TestIsPromptAcceptsDevicePromptWithAnsiSequence(t *testing.T) {
	m := NewStreamMatcher()

	if !m.IsPrompt("\x1b[16D<Huawei>\r\n") {
		t.Fatalf("device prompt wrapped by ANSI cursor control should be detected")
	}
}

func TestIsPaginationPromptIgnoresAnsiNoise(t *testing.T) {
	m := NewStreamMatcher()

	if !m.IsPaginationPrompt("\x1b[16D---- More ----") {
		t.Fatalf("pagination prompt should still be detected after ANSI cleanup")
	}
}

// TestIsPromptStrict_HRPFormat 测试华为 HRP 双机热备格式提示符
func TestIsPromptStrict_HRPFormat(t *testing.T) {
	m := NewStreamMatcher()

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		// 标准华为格式
		{"standard_angle_brackets", "<INT_SW>", true},
		{"standard_square_brackets", "[FW-1]", true},

		// HRP 双机热备格式 - 尖括号
		{"hrp_master_angle", "HRP_M<FW-1>", true},
		{"hrp_slave_angle", "HRP_S<FW-2>", true},

		// HRP 双机热备格式 - 方括号
		{"hrp_master_square", "HRP_M[FW-1]", true},
		{"hrp_slave_square", "HRP_S[FW-2]", true},

		// HRP + 多级视图
		{"hrp_multi_view", "HRP_M[FW-1-GE0/0/1]", true},

		// HRP + 未保存标记
		{"hrp_unsaved", "HRP_M[~FW-1]", true},

		// 特权模式
		{"privilege_mode_hash", "INT_SW#", true},
		{"privilege_mode_gt", "INT_SW>", true},

		// 应该排除的混合行
		{"mixed_line_angle", "<The current login time>", false},
		{"mixed_line_square", "[The current login time]", false},
		{"info_prefix_angle", "Info: <text>", false},
		{"info_prefix_square", "Info: [text]", false},

		// 普通命令行（不应匹配）
		{"command_line", "display version", false},
		{"empty_line", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.IsPromptStrict(tt.input)
			if result != tt.expected {
				t.Errorf("IsPromptStrict(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

// TestIsPromptStrict_EdgeCases 测试边界情况
func TestIsPromptStrict_EdgeCases(t *testing.T) {
	m := NewStreamMatcher()

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		// 空内容
		{"empty_angle", "<>", false},
		{"empty_square", "[]", false},

		// 只有空格
		{"space_in_angle", "< >", false},
		{"space_in_square", "[ ]", false},

		// 前缀含空格（应排除）
		{"space_prefix_angle", "Info <FW-1>", false},
		{"space_prefix_square", "Info [FW-1]", false},

		// 带换行符（应被 trim 后处理）
		{"with_newline", "<FW-1>\r\n", true},

		// 特殊字符
		{"dash_in_name", "<FW-1-Backup>", true},
		{"underscore_in_name", "<FW_1>", true},
		{"number_in_name", "<SW123>", true},

		// 特权模式排除华为格式变体
		{"privilege_with_angle", "test<SW>", false},
		{"privilege_with_square", "test[SW]", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.IsPromptStrict(tt.input)
			if result != tt.expected {
				t.Errorf("IsPromptStrict(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}
