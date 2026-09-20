package taskexec

import (
	"strings"

	"github.com/NetWeaverGo/core/internal/inspection"
	"github.com/NetWeaverGo/core/internal/models"
)

// evaluateDSLRules 按"规则依赖命令 == 已执行命令"维度执行 DSL 规则判定（A4/P0-8）。
//
// 背景：内置 DSL 规则编号（HEALTH-/REL-/ROUTE-/BASE-）与巡检模板检查项编号（GEN_/CE_/S_/AR_）
// 不同名，`EvaluateItem` 中的 CheckNo 精确分支无法命中，因此内置 62 条规则此前在生产链路中
// 永不执行。本函数以"命令"为触发维度：凡是已执行命令被某条 DSL 规则声明依赖，即用该命令回显
// 执行规则判定，保证内置规则真实跑在生产巡检链路中。
func evaluateDSLRules(
	runID, deviceIP string,
	items []models.InspectionItem,
	echoProvider func(cmd string) (string, bool),
	parsedProvider func(cmd string) []map[string]interface{},
	contextVars map[string]string,
) []models.InspectionResult {
	interp := inspection.GetGlobalDSLInterpreter()
	if interp == nil {
		return nil
	}

	seenCmd := make(map[string]struct{}, len(items))
	seenRule := make(map[string]struct{})
	var out []models.InspectionResult

	for _, it := range items {
		cmd := strings.TrimSpace(it.CommandKey)
		if cmd == "" {
			continue
		}
		if _, ok := seenCmd[cmd]; ok {
			continue
		}
		seenCmd[cmd] = struct{}{}

		rules := interp.RulesForCommand(cmd)
		if len(rules) == 0 {
			continue
		}
		echo, ok := echoProvider(cmd)
		if !ok || strings.TrimSpace(echo) == "" {
			continue
		}
		rows := parsedProvider(cmd)
		for _, rule := range rules {
			if _, dup := seenRule[rule.CheckNo]; dup {
				continue
			}
			seenRule[rule.CheckNo] = struct{}{}
			out = append(out, interp.Evaluate(rule, &inspection.EvaluateInput{
				RunID:       runID,
				DeviceIP:    deviceIP,
				RawEcho:     echo,
				ParsedRows:  rows,
				ContextVars: contextVars,
			}))
		}
	}
	return out
}
