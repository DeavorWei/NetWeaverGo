package taskexec

import (
	"testing"

	"github.com/NetWeaverGo/core/internal/inspection"
	"github.com/NetWeaverGo/core/internal/models"
)

// P0-8：内置 DSL 规则必须能在巡检链路中按命令真实执行（此前 CheckNo 不同名导致规则永不触发）
func TestEvaluateDSLRules_TriggersByCommand(t *testing.T) {
	interp := inspection.GetGlobalDSLInterpreter()
	var target *inspection.DSLRule
	for _, r := range interp.AllRules() {
		if len(r.Commands) > 0 && r.Commands[0] != "" {
			target = r
			break
		}
	}
	if target == nil {
		t.Skip("内置 DSL 规则未声明依赖命令")
	}

	items := []models.InspectionItem{{Code: "GEN_TEST", CommandKey: target.Commands[0]}}
	out := evaluateDSLRules("run-1", "10.0.0.1", items,
		func(cmd string) (string, bool) { return "sample echo output for " + cmd, true },
		func(string) []map[string]interface{} { return nil },
		map[string]string{})
	if len(out) == 0 {
		t.Fatal("应至少执行一条 DSL 规则")
	}
	found := false
	for _, r := range out {
		if r.ItemCode == target.CheckNo {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("应包含规则 %s 的判定结果", target.CheckNo)
	}

	// 回显缺失时不应产出结果（避免误报）
	empty := evaluateDSLRules("run-1", "10.0.0.1", items,
		func(string) (string, bool) { return "", false },
		func(string) []map[string]interface{} { return nil },
		map[string]string{})
	if len(empty) != 0 {
		t.Fatalf("无回显时不应执行规则，实际产出 %d 条", len(empty))
	}
}
