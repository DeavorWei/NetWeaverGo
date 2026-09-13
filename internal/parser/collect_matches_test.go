package parser

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 阶段二 2.1：CollectMatches 必须收集嵌套子规则命中，且偏移为相对原文的绝对偏移。
func TestTreeEngine_CollectMatches_NestedAbsoluteOffsets(t *testing.T) {
	text := "Slot 1:\n  Board Type : LPU\n  GE1/0/1    up    1000M\nSlot 2:\n  GE1/0/2    down  1000M\n"
	rules := []TreeRule{
		{
			ParseItem:  "slotId",
			ParentItem: "",
			IsList:     true,
			SplitRegex: `(?m)^Slot\s+(\d+):`,
			GroupIndex: 1,
			IsOutput:   true,
			Order:      1,
		},
		{
			ParseItem:  "portName",
			ParentItem: "slotId",
			IsList:     true,
			ParseRegex: `(?m)^\s+((?:GE|MEth)\S+)\s+`,
			GroupIndex: 1,
			IsOutput:   true,
			Order:      2,
		},
	}

	compiledRules, rootRules, err := CompileTreeRules(rules)
	require.NoError(t, err)

	tpl := &CompiledTemplate{
		RegexTemplate: RegexTemplate{
			Engine:     EngineTree,
			TreeConfig: &TreeTemplate{Rules: rules},
		},
		CompiledTreeRules: compiledRules,
		TreeRootRules:     rootRules,
	}

	engine := NewTreeEngine()
	matches := engine.CollectMatches(tpl, text)
	require.NotEmpty(t, matches, "应收集到命中区间")

	// 核心契约：偏移切片必须与原文完全一致（绝对偏移无漂移）
	for _, m := range matches {
		require.GreaterOrEqual(t, m.Start, 0)
		require.LessOrEqual(t, m.End, len(text))
		assert.Equal(t, text[m.Start:m.End], m.Text, "规则 %s 的偏移与原文不一致", m.Rule)
	}

	// 根规则（分块）命中应覆盖两个 Slot 块头
	var slotHits []RuleMatch
	for _, m := range matches {
		if m.Rule == "slotId" {
			slotHits = append(slotHits, m)
		}
	}
	assert.Len(t, slotHits, 2, "应收集到 2 个 Slot 块头命中")

	// 子规则（嵌套）命中必须被收集，且偏移换算为原文绝对位置
	// 说明：命中区间语义为"规则正则的整体匹配"（与正则/聚合引擎一致），故包含前导空白
	var nestedHits []RuleMatch
	for _, m := range matches {
		if m.Rule == "portName" {
			nestedHits = append(nestedHits, m)
		}
	}
	if len(nestedHits) == 0 {
		t.Fatal("嵌套子规则 portName 的命中未被收集")
	}

	slot2PortStart := strings.Index(text, "  GE1/0/2")
	require.Positive(t, slot2PortStart, "样例文本应包含 GE1/0/2 行")

	foundAbsolute := false
	for _, m := range nestedHits {
		assert.Contains(t, m.Text, "GE", "嵌套命中文本应包含端口名: %q", m.Text)
		if m.Start == slot2PortStart {
			foundAbsolute = true
		}
	}
	assert.True(t, foundAbsolute,
		"Slot 2 内子规则命中应为原文绝对偏移 %d（若为块内相对偏移则说明 baseOffset 未透传）", slot2PortStart)

	// 去重：同一 (rule,start,end) 不应重复出现
	seen := make(map[RuleMatch]struct{}, len(matches))
	for _, m := range matches {
		if _, dup := seen[m]; dup {
			t.Errorf("命中区间重复: %+v", m)
		}
		seen[m] = struct{}{}
	}
}

// 阶段二 2.1：空模板/空文本应安全返回 nil
func TestTreeEngine_CollectMatches_EmptyInputs(t *testing.T) {
	engine := NewTreeEngine()
	assert.Nil(t, engine.CollectMatches(nil, "any"))
	assert.Nil(t, engine.CollectMatches(&CompiledTemplate{}, ""))
}
