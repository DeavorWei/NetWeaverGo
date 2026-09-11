package parser

import (
	"regexp"
	"testing"
)

func compileTestPattern(t *testing.T, pattern string) *regexp.Regexp {
	t.Helper()
	if pattern == "" {
		return nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		t.Fatalf("编译正则失败: %v", err)
	}
	return re
}

// TestParseDetail_RegexOutcome 验证 regex 引擎的解析元信息
func TestParseDetail_RegexOutcome(t *testing.T) {
	tpl := &CompiledTemplate{
		RegexTemplate:   RegexTemplate{CommandKey: "sysname", Engine: EngineRegex},
		CompiledPattern: compileTestPattern(t, `(?m)sysname\s+(\S+)`),
	}
	p := NewCompositeParser("huawei", map[string]*CompiledTemplate{"sysname": tpl})

	rows, outcome, err := p.ParseDetail("sysname", "sysname Core-01")
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(rows) == 0 {
		t.Fatal("应有解析结果")
	}
	if outcome.Engine != string(EngineRegex) {
		t.Errorf("outcome.Engine = %q, want regex", outcome.Engine)
	}
	if outcome.Fallback {
		t.Error("regex 引擎不应标记 fallback")
	}
	if outcome.DurationMs < 0 {
		t.Errorf("耗时不应为负: %d", outcome.DurationMs)
	}
}

// TestParseDetail_TreeFallbackOutcome 验证 tree 解析为空且存在 legacy 备选时标记 fallback
func TestParseDetail_TreeFallbackOutcome(t *testing.T) {
	rules := []TreeRule{
		{ParseItem: "block", IsList: true, SplitRegex: `(?m)^NEVER_MATCH_`, GroupIndex: 1, IsOutput: true, Order: 1},
	}
	compiledRules, rootRules, err := CompileTreeRules(rules)
	if err != nil {
		t.Fatalf("编译规则树失败: %v", err)
	}
	tpl := &CompiledTemplate{
		RegexTemplate: RegexTemplate{
			CommandKey: "test_cmd",
			Engine:     EngineTree,
			Pattern:    `(?m)sysname\s+(\S+)`,
			TreeConfig: &TreeTemplate{Rules: rules},
		},
		CompiledTreeRules: compiledRules,
		TreeRootRules:     rootRules,
		CompiledPattern:   compileTestPattern(t, `(?m)sysname\s+(\S+)`),
	}

	p := NewCompositeParser("huawei", map[string]*CompiledTemplate{"test_cmd": tpl})
	rows, outcome, err := p.ParseDetail("test_cmd", "sysname Core-01")
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(rows) == 0 {
		t.Fatal("应回退到 regex 并产出结果")
	}
	if !outcome.Fallback {
		t.Error("tree 无产出且存在 legacy 备选时应标记 fallback=true")
	}
}

// TestParse_EquivalentToParseDetail 保证 Parse 与 ParseDetail 结果一致（向后兼容）
func TestParse_EquivalentToParseDetail(t *testing.T) {
	tpl := &CompiledTemplate{
		RegexTemplate:   RegexTemplate{CommandKey: "sysname", Engine: EngineRegex},
		CompiledPattern: compileTestPattern(t, `(?m)sysname\s+(\S+)`),
	}
	p := NewCompositeParser("huawei", map[string]*CompiledTemplate{"sysname": tpl})

	rowsA, errA := p.Parse("sysname", "sysname Core-01")
	rowsB, _, errB := p.ParseDetail("sysname", "sysname Core-01")
	if (errA == nil) != (errB == nil) || len(rowsA) != len(rowsB) {
		t.Fatalf("Parse 与 ParseDetail 结果不一致: %v/%v", rowsA, rowsB)
	}
}
