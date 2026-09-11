package matcher

import "testing"

// 验证 MatchPrompt 与既有 IsPrompt 语义完全一致，且能返回命中的提示符整行
func TestMatchPrompt_EquivalentToIsPrompt(t *testing.T) {
	m := NewStreamMatcher()

	cases := []string{
		"<SW1>",
		"[SW1]",
		"[SW1-GigabitEthernet0/0/1]",
		"SW1>",
		"SW1#",
		"user@ubuntu-server:~$",
		"Some random output",
		"",
		"---- More ----\n<SW1>",
	}

	for _, chunk := range cases {
		want := m.IsPrompt(chunk)
		got, line := m.MatchPrompt(chunk)
		if got != want {
			t.Fatalf("MatchPrompt(%q) 命中=%v，与 IsPrompt 的 %v 不一致", chunk, got, want)
		}
		if want && line == "" {
			t.Fatalf("MatchPrompt(%q) 命中但返回的提示符行为空", chunk)
		}
		if !want && line != "" {
			t.Fatalf("MatchPrompt(%q) 未命中却返回了提示符行 %q", chunk, line)
		}
	}

	// 具体取值校验
	if _, line := m.MatchPrompt("show version\r\n<SW1>"); line != "<SW1>" {
		t.Fatalf("期望提取提示符行 <SW1>，实际 %q", line)
	}
}
