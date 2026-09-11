package ui

import (
	"testing"

	"github.com/NetWeaverGo/core/internal/models"
	"github.com/NetWeaverGo/core/internal/parser"
)

// 验证 regex 引擎返回绝对偏移的命中区间（前端高亮不漂移）
func TestTestTemplate_RegexMatches(t *testing.T) {
	svc := &ParseTemplateService{db: nil}
	raw := "sysname Core-01\n#\nsysname Core-02\n"

	res := svc.TestTemplate(models.TestParseTemplateRequest{
		Vendor:     "huawei",
		CommandKey: "sysname",
		Engine:     "regex",
		Pattern:    `(?m)^sysname\s+(\S+)`,
		Multiline:  true,
		RawText:    raw,
	})

	if !res.Success {
		t.Fatalf("解析失败: %s", res.Error)
	}
	if len(res.Matches) != 2 {
		t.Fatalf("应返回 2 个命中区间，实际 %d", len(res.Matches))
	}
	for _, m := range res.Matches {
		if m.Start < 0 || m.End > len(raw) || m.Start >= m.End {
			t.Fatalf("命中区间越界: %+v", m)
		}
		if raw[m.Start:m.End] != m.Text {
			t.Fatalf("命中文本与区间不一致: text=%q, raw[%d:%d]=%q", m.Text, m.Start, m.End, raw[m.Start:m.End])
		}
	}
	if res.Matches[0].Start > res.Matches[1].Start {
		t.Error("命中区间应按起始位置升序排列")
	}
}

// 验证 tree 引擎根规则命中区间
func TestTestTemplate_TreeMatches(t *testing.T) {
	svc := &ParseTemplateService{db: nil}
	raw := "Slot 1:\nBoard Type: CE-MPU\nSlot 2:\nBoard Type: CE-LPU\n"

	res := svc.TestTemplate(models.TestParseTemplateRequest{
		Vendor:     "huawei",
		CommandKey: "display_device",
		Engine:     "tree",
		ParseRules: map[string]interface{}{
			"maxOutputLevel": 2,
			"rules": []interface{}{
				map[string]interface{}{
					"parseItem":  "slot",
					"parentItem": "",
					"isList":     true,
					"splitRegex": `(?m)^Slot\s+(\d+):`,
					"groupIndex": 1,
					"isOutput":   true,
					"order":      1,
				},
				map[string]interface{}{
					"parseItem":  "boardType",
					"parentItem": "slot",
					"isList":     false,
					"parseRegex": `Board Type:\s*(\S+)`,
					"groupIndex": 1,
					"isOutput":   true,
					"order":      2,
				},
			},
		},
		RawText: raw,
	})

	if !res.Success {
		t.Fatalf("解析失败: %s", res.Error)
	}
	if len(res.Matches) == 0 {
		t.Fatal("tree 引擎应返回根规则的命中区间")
	}
	for _, m := range res.Matches {
		if raw[m.Start:m.End] != m.Text {
			t.Fatalf("命中文本与区间不一致: %+v", m)
		}
	}
}

// 验证列表汇聚：内置模板（Source=builtin）与用户模板（Source=user/override）同时可见
func TestListTemplates_IncludesBuiltinSource(t *testing.T) {
	db := setupTestDB(t)
	manager := parser.NewParserManager()
	if err := manager.Bootstrap(); err != nil {
		t.Fatalf("解析器启动失败: %v", err)
	}
	svc := NewParseTemplateService(db, manager)

	// 写入一条用户模板（覆盖一个不存在的键，应为 user）
	if err := svc.CreateTemplate(models.SaveParseTemplateRequest{
		Vendor:     "huawei",
		CommandKey: "my_custom_command",
		Engine:     "regex",
		Pattern:    `custom\s+(\S+)`,
		Enabled:    true,
	}); err != nil {
		t.Fatalf("创建用户模板失败: %v", err)
	}

	list, err := svc.ListTemplates("huawei")
	if err != nil {
		t.Fatalf("查询模板失败: %v", err)
	}

	var builtinCount, userCount int
	for _, vo := range list {
		switch vo.Source {
		case "builtin":
			builtinCount++
		case "user", "override":
			userCount++
		default:
			t.Fatalf("模板 Source 未回填: %+v", vo)
		}
	}
	if builtinCount == 0 {
		t.Error("应汇聚出内置模板（Source=builtin）")
	}
	if userCount == 0 {
		t.Error("应包含用户模板（Source=user/override）")
	}
}
