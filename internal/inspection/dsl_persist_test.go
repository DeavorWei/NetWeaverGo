package inspection

import (
	"os"
	"testing"
)

// P2-8：导入规则应可持久化并在重启后恢复，且非法规则被拒绝
func TestImportedRulesPersistRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := DefaultImportedRulesPath(dir)
	if path == "" {
		t.Fatal("持久化路径不应为空")
	}

	rules := `[{"checkno":"IMP-001","title":{"zh":"导入规则"},"category":"Health","riskLevel":"major",
		"commands":["display cpu-usage"],"assert":{"type":"must_not_contain","expr":"ERROR"}}]`

	interp := GetGlobalDSLInterpreter()
	if n, err := interp.ImportRulesJSON([]byte(rules)); err != nil || n != 1 {
		t.Fatalf("导入失败: n=%d err=%v", n, err)
	}
	if err := SaveImportedRules(path); err != nil {
		t.Fatalf("持久化失败: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取持久化文件失败: %v", err)
	}

	// 模拟重启：新解释器加载内置规则后从文件恢复
	fresh := NewDSLInterpreter()
	if err := fresh.LoadBuiltin(); err != nil {
		t.Fatalf("加载内置规则失败: %v", err)
	}
	n, err := fresh.ImportRulesJSON(data)
	if err != nil || n != 1 {
		t.Fatalf("恢复导入规则失败: n=%d err=%v", n, err)
	}
	if fresh.FindRule("IMP-001") == nil {
		t.Fatal("重启恢复后应能查到导入规则 IMP-001")
	}

	// 字段完整性校验：缺少 commands 与 assert 应被拒绝
	if _, err := NewDSLInterpreter().ImportRulesJSON([]byte(`[{"checkno":"BAD-1"}]`)); err == nil {
		t.Fatal("缺少 commands/assert 的规则应被拒绝")
	}
	// 非法断言类型应被拒绝
	if _, err := NewDSLInterpreter().ImportRulesJSON([]byte(`[{"checkno":"BAD-2","commands":["x"],"assert":{"type":"bogus"}}]`)); err == nil {
		t.Fatal("非法断言类型应被拒绝")
	}
	// 缺少 checkno 应被跳过而非报错
	if _, err := NewDSLInterpreter().ImportRulesJSON([]byte(`[{"commands":["display version"]}]`)); err != nil {
		t.Fatalf("缺少 checkno 应跳过而非报错: %v", err)
	}
}
