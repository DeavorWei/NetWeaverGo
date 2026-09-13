package parser

import "testing"

// 阶段一 1.4：BuildRegexWithFlags 为唯一实现（原 ui.applyRegexFlags 已收敛到此处）。
func TestBuildRegexWithFlags(t *testing.T) {
	cases := []struct {
		name    string
		pattern string
		flags   string
		want    string
	}{
		{"空 flags 原样返回", "^abc$", "", "^abc$"},
		{"单标志", "^abc$", "i", "(?i)^abc$"},
		{"多标志", "^abc$", "mis", "(?mis)^abc$"},
		{"标志去重", "^abc$", "mii", "(?mi)^abc$"},
		{"标志大小写与空白归一", "  ^abc$  ", " MIS ", "(?mis)  ^abc$  "},
		{"已含内联前缀不重复添加", "(?i)^abc$", "i", "(?i)^abc$"},
		{"非法标志被忽略", "^abc$", "xyz", "^abc$"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := BuildRegexWithFlags(c.pattern, c.flags); got != c.want {
				t.Fatalf("BuildRegexWithFlags(%q, %q) = %q, want %q", c.pattern, c.flags, got, c.want)
			}
		})
	}
}
