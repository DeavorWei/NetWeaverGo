package inspection

import (
	"testing"
)

func TestDSLInterpreter_LoadBuiltin(t *testing.T) {
	interp := NewDSLInterpreter()
	err := interp.LoadBuiltin()
	if err != nil {
		t.Fatalf("LoadBuiltin 失败: %v", err)
	}

	rules := interp.AllRules()
	if len(rules) < 60 {
		t.Errorf("预期加载规则数 >= 60，实际加载: %d", len(rules))
	}
	t.Logf("成功内嵌加载 %d 条 DSL 巡检规则", len(rules))

	// 检查各类目规则是否存在
	categories := make(map[string]int)
	for _, r := range rules {
		categories[r.Category]++
	}
	if categories["Health"] < 20 {
		t.Errorf("Health 规则数量不足: %d", categories["Health"])
	}
	if categories["Reliability"] < 10 {
		t.Errorf("Reliability 规则数量不足: %d", categories["Reliability"])
	}
	if categories["BASE"] < 8 {
		t.Errorf("BASE 规则数量不足: %d", categories["BASE"])
	}
	if categories["BGP"]+categories["OSPF"]+categories["Routing"] < 10 {
		t.Errorf("路由类规则数量不足")
	}
}

func TestDSLRule_MatchesScope(t *testing.T) {
	rule := &DSLRule{
		CheckNo: "TEST-001",
		Scope: RuleScope{
			Vendors:  []string{"huawei", "h3c"},
			Models:   []string{"^CE.*", "S67.*"},
			Versions: []string{"V200R.*"},
		},
	}

	// 命中
	if !rule.MatchesScope("huawei", "CE6800", "V200R019C00") {
		t.Errorf("预期命中 scope")
	}
	if !rule.MatchesScope("H3C", "S6720", "V200R020") {
		t.Errorf("预期命中 scope")
	}

	// 厂商不匹配
	if rule.MatchesScope("cisco", "CE6800", "V200R019C00") {
		t.Errorf("cisco 不应命中")
	}

	// 款型不匹配
	if rule.MatchesScope("huawei", "NE40E", "V200R019C00") {
		t.Errorf("NE40E 不应命中")
	}

	// 版本不匹配
	if rule.MatchesScope("huawei", "CE6800", "V100R001") {
		t.Errorf("V100R001 不应命中")
	}
}

func TestDSLInterpreter_Evaluate_Threshold(t *testing.T) {
	interp := NewDSLInterpreter()

	rule := &DSLRule{
		CheckNo:   "HEALTH-001",
		Category:  "Health",
		RiskLevel: "critical",
		Extract: ExtractSpec{
			Fields: []FieldExtract{
				{Name: "cpu_usage", Pattern: "CPU usage\\s*:\\s*([0-9.]+)%", Group: 1},
			},
		},
		Assert: AssertSpec{
			Type:     "threshold",
			Field:    "cpu_usage",
			Expr:     85.0,
			Operator: "<=",
		},
	}

	// 正常情况: 45.5% <= 85.0
	resPass := interp.Evaluate(rule, &EvaluateInput{
		RunID:    "run-1",
		DeviceIP: "10.0.0.1",
		RawEcho:  "Slot 1 CPU usage : 45.5% (Max: 60%)",
	})
	if resPass.Status != string(ResultPass) {
		t.Errorf("预期 Pass，实际: %s, 原因: %s", resPass.Status, resPass.Problem)
	}

	// 超标情况: 92.1% > 85.0
	resFail := interp.Evaluate(rule, &EvaluateInput{
		RunID:    "run-2",
		DeviceIP: "10.0.0.1",
		RawEcho:  "Slot 1 CPU usage : 92.1% (Max: 99%)",
	})
	if resFail.Status != string(ResultFail) {
		t.Errorf("预期 Fail，实际: %s", resFail.Status)
	}
}

func TestDSLInterpreter_Evaluate_KeywordsAndRegex(t *testing.T) {
	interp := NewDSLInterpreter()

	// must_not_contain
	rulePower := &DSLRule{
		CheckNo: "HEALTH-003",
		Extract: ExtractSpec{
			Fields: []FieldExtract{
				{Name: "power_status", Pattern: "Power\\s+\\S+\\s+(Normal|Fail)", Group: 1},
			},
		},
		Assert: AssertSpec{
			Type:  "must_not_contain",
			Field: "power_status",
			Expr:  "Fail",
		},
	}

	resPowerOk := interp.Evaluate(rulePower, &EvaluateInput{
		RawEcho: "Power PWR1 Normal",
	})
	if resPowerOk.Status != string(ResultPass) {
		t.Errorf("预期 Pass")
	}

	resPowerBad := interp.Evaluate(rulePower, &EvaluateInput{
		RawEcho: "Power PWR1 Fail",
	})
	if resPowerBad.Status != string(ResultFail) {
		t.Errorf("预期 Fail")
	}

	// regex
	ruleIP := &DSLRule{
		CheckNo: "BASE-007",
		Extract: ExtractSpec{
			Fields: []FieldExtract{
				{Name: "syslog_host", Pattern: "info-center loghost\\s+([0-9.]+)", Group: 1},
			},
		},
		Assert: AssertSpec{
			Type:  "regex",
			Field: "syslog_host",
			Expr:  `^\d+\.\d+\.\d+\.\d+$`,
		},
	}

	resIPOk := interp.Evaluate(ruleIP, &EvaluateInput{
		RawEcho: "info-center loghost 192.168.1.100",
	})
	if resIPOk.Status != string(ResultPass) {
		t.Errorf("预期 Pass")
	}
}

func TestDSLInterpreter_PreCollectAndInheritance(t *testing.T) {
	interp := NewDSLInterpreter()

	// 1. 前置采集执行
	preItem := &PreCollectItem{
		Name:    "get_sys_version",
		Command: "display version",
		Extract: ExtractSpec{
			Fields: []FieldExtract{
				{Name: "patch_str", Pattern: "Patch Version: (V[0-9A-Z]+)", Group: 1},
			},
		},
		StoreAs: "global_patch_version",
	}
	k, v := interp.ExecutePreCollect(preItem, "Huawei Versatile OS\nPatch Version: V200R019SPH001\nUptime: 10 days")
	if k != "global_patch_version" || v != "V200R019SPH001" {
		t.Fatalf("前置采集提取异常: key=%s, val=%s", k, v)
	}

	// 2. 父规则与子规则继承
	childRule := &DSLRule{
		CheckNo:       "BASE-CHILD-01",
		Category:      "BASE",
		ParentCheckNo: "BASE-PARENT-01", // 继承父规则断言
	}

	// 注册父规则到解释器
	_ = interp.LoadBuiltin()
	_, _ = interp.ImportRulesJSON([]byte(`[{"checkno":"BASE-PARENT-01","category":"BASE","assert":{"type":"must_contain","field":"global_patch_version","expr":"SPH001"}}]`))

	// 子规则无命令回显，但直接消费 ContextVars 中的前置变量
	res := interp.Evaluate(childRule, &EvaluateInput{
		ContextVars: map[string]string{
			"global_patch_version": v,
		},
	})

	if res.Status != string(ResultPass) {
		t.Errorf("预期继承父规则并消费 ContextVars 评估 Pass，实际: %s, 原因: %s", res.Status, res.Problem)
	}
}
