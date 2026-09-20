package matcher

import (
	"regexp"
	"testing"
)

func TestPolicyMatcher_Resolve(t *testing.T) {
	pm := NewPolicyMatcher()
	if err := pm.LoadEmbedded(); err != nil {
		t.Fatalf("LoadEmbedded 失败: %v", err)
	}

	tests := []struct {
		scene      string
		vendor     string
		deviceType string
		wantVendor string
	}{
		{"collect", "huawei", "switch", "huawei"},
		{"inspect", "huawei", "router", "huawei"},
		{"*", "h3c", "*", "h3c"},
		{"*", "cisco", "*", "cisco"},
		{"*", "unknown_vendor", "*", "*"}, // 兜底到 default
	}

	for _, tt := range tests {
		p := pm.Resolve(tt.scene, tt.vendor, tt.deviceType)
		if p == nil {
			t.Fatalf("Resolve(%s, %s, %s) 返回 nil", tt.scene, tt.vendor, tt.deviceType)
		}
		if p.Vendor != tt.wantVendor {
			t.Errorf("Resolve(%s, %s, %s) vendor = %s, want %s", tt.scene, tt.vendor, tt.deviceType, p.Vendor, tt.wantVendor)
		}
		if len(p.PromptPatterns) == 0 {
			t.Errorf("策略 PromptPatterns 不应为空")
		}
	}
}

func TestStreamMatcher_WithPolicy(t *testing.T) {
	pm := GetDefaultPolicyMatcher()
	p := pm.Resolve("collect", "huawei", "switch")
	if p == nil {
		t.Fatalf("无法解析 huawei 策略")
	}

	m := NewStreamMatcherWithPolicy(p)
	if m.Policy == nil {
		t.Fatalf("Policy 字段应已设置")
	}

	// 验证提示符识别
	matched, _ := m.MatchPrompt("<Huawei>")
	if !matched {
		t.Errorf("预期匹配华为提示符 <Huawei>")
	}

	// 验证错误识别
	hit, rule := m.MatchErrorRule("Error: Unrecognized command found at '^' position.")
	if !hit || rule == nil {
		t.Errorf("预期命中错误规则")
	}
}

func TestShadowMatcher(t *testing.T) {
	oldRules := []ErrorRule{
		{
			Name:     "OldRule",
			Pattern:  regexp.MustCompile(`(?i)fail`),
			Severity: SeverityCritical,
			Vendor:   "generic",
		},
	}
	newRules := []ErrorRule{
		{
			Name:     "NewRule",
			Pattern:  regexp.MustCompile(`(?i)(fail|error)`),
			Severity: SeverityCritical,
			Vendor:   "generic",
		},
	}

	shadow := NewShadowMatcher(oldRules, newRules, true)

	// 两边都命中
	oldHit, _, newHit, _, hasDiff := shadow.MatchBoth("Task failed successfully")
	if !oldHit || !newHit {
		t.Errorf("两边应该都命中")
	}
	// 名称不同算差异
	if !hasDiff {
		t.Errorf("规则名不同应被视为差异")
	}

	// 仅新规则命中
	oldHit2, _, newHit2, _, hasDiff2 := shadow.MatchBoth("syntax error occurred")
	if oldHit2 || !newHit2 || !hasDiff2 {
		t.Errorf("应仅新规则命中且记录差异: oldHit=%v, newHit=%v, hasDiff=%v", oldHit2, newHit2, hasDiff2)
	}

	metrics := shadow.GetMetrics()
	if metrics.TotalEvaluations != 2 {
		t.Errorf("TotalEvaluations = %d, want 2", metrics.TotalEvaluations)
	}
	if metrics.Discrepancies != 2 {
		t.Errorf("Discrepancies = %d, want 2", metrics.Discrepancies)
	}
	if metrics.DiscrepancyRate != 1.0 {
		t.Errorf("DiscrepancyRate = %f, want 1.0", metrics.DiscrepancyRate)
	}
	if len(metrics.RecentSamples) != 2 {
		t.Errorf("RecentSamples len = %d, want 2", len(metrics.RecentSamples))
	}
}

// P0-4：先开启影子再应用策略时，影子"新规则集"必须同步刷新（修复规则过期缺陷）
func TestStreamMatcher_ShadowRefreshOnApplyPolicy(t *testing.T) {
	m := NewStreamMatcher()
	m.EnableShadowMode(true)

	policy := &MatchPolicy{
		Vendor: "test",
		ErrorRules: []ErrorRuleDef{
			{Name: "PolicyOnlyRule", Pattern: `(?i)policy-only-token`, Severity: "critical"},
		},
	}
	m.ApplyPolicy(policy)

	m.MatchErrorRule("policy-only-token appeared")

	metrics, ok := m.ShadowMetrics()
	if !ok {
		t.Fatal("影子模式应处于启用状态")
	}
	if metrics.TotalEvaluations == 0 {
		t.Fatal("影子模式应完成评估")
	}
	if metrics.Discrepancies == 0 {
		t.Fatal("策略更新后影子新规则集应生效并记录差异（旧实现会因规则过期漏记）")
	}
}

// P0-4：未启用影子模式时不得返回统计
func TestStreamMatcher_ShadowDisabled(t *testing.T) {
	m := NewStreamMatcher()
	if _, ok := m.ShadowMetrics(); ok {
		t.Fatal("未启用影子模式时 ShadowMetrics 应返回 ok=false")
	}
}

// P2-5：Resolve 返回副本，调用方修改不得污染内部策略单例
func TestPolicyMatcher_ResolveReturnsCopy(t *testing.T) {
	pm := NewPolicyMatcher()
	if err := pm.LoadEmbedded(); err != nil {
		t.Fatalf("LoadEmbedded 失败: %v", err)
	}
	p1 := pm.Resolve("*", "huawei", "*")
	if p1 == nil {
		t.Fatal("应解析到 huawei 策略")
	}
	origVendor := p1.Vendor

	p1.Vendor = "tampered"
	p2 := pm.Resolve("*", "huawei", "*")
	if p2 == nil {
		t.Fatal("二次解析不应返回 nil")
	}
	if p2.Vendor != origVendor {
		t.Fatalf("Resolve 应返回副本，内部策略被污染: want %s got %s", origVendor, p2.Vendor)
	}
}
