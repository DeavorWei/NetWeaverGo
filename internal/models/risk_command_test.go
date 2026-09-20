package models

import (
	"regexp"
	"testing"
)

// P1-9：风险命令种子已扩充，且所有内置规则正则必须可编译
func TestDefaultRiskCommandSeeds_ExpandedAndCompilable(t *testing.T) {
	seeds := DefaultRiskCommandSeeds()
	if len(seeds) <= 18 {
		t.Fatalf("风险命令种子应已扩充（>18），当前 %d", len(seeds))
	}
	for _, s := range seeds {
		if _, err := regexp.Compile(s.Pattern); err != nil {
			t.Errorf("内置风险命令规则正则不可编译: vendor=%s pattern=%s err=%v", s.Vendor, s.Pattern, err)
		}
		if !s.Builtin || !s.Enabled {
			t.Errorf("内置种子应默认启用且标记 Builtin: %+v", s)
		}
	}
}
