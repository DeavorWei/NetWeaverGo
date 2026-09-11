package repository

import (
	"testing"

	"github.com/NetWeaverGo/core/internal/models"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestParseTemplateRepository_ListEnabled_WithAppliesTo(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("连接内存数据库失败: %v", err)
	}

	if err := db.AutoMigrate(&models.UserParseTemplate{}); err != nil {
		t.Fatalf("自动迁移失败: %v", err)
	}

	appliesJSON := `{"models":["S5700","S5735"],"versions":["V200R019*"]}`
	rulesJSON := `{"rules":[{"parseItem":"port","parseRegex":"interface (\\S+)","groupIndex":1}]}`

	templates := []models.UserParseTemplate{
		{
			Vendor:      "huawei",
			CommandKey:  "display_interface",
			Engine:      "tree",
			ParseRules:  rulesJSON,
			AppliesTo:   appliesJSON,
			Enabled:     true,
		},
		{
			Vendor:      "huawei",
			CommandKey:  "display_device",
			Engine:      "tree",
			Enabled:     false, // 未启用
		},
		{
			Vendor:      "cisco",
			CommandKey:  "show_version",
			Engine:      "regex",
			Enabled:     true,
		},
	}

	for i := range templates {
		if err := db.Create(&templates[i]).Error; err != nil {
			t.Fatalf("写入测试模板失败: %v", err)
		}
	}
	if err := db.Exec("UPDATE net_user_parse_templates SET enabled = 0 WHERE command_key = ?", "display_device").Error; err != nil {
		t.Fatalf("更新 enabled 为 0 失败: %v", err)
	}

	repo := NewParseTemplateRepository(db)
	enabled, err := repo.ListEnabled("huawei")
	if err != nil {
		t.Fatalf("ListEnabled 失败: %v", err)
	}

	if len(enabled) != 1 {
		t.Fatalf("预期 1 条已启用的华为模板，实际得到 %d 条", len(enabled))
	}

	got := enabled[0]
	if got.CommandKey != "display_interface" {
		t.Errorf("预期 CommandKey display_interface, 得到 %s", got.CommandKey)
	}
	if got.AppliesTo != appliesJSON {
		t.Errorf("预期 AppliesTo %s, 得到 %s", appliesJSON, got.AppliesTo)
	}
	if got.ParseRules != rulesJSON {
		t.Errorf("预期 ParseRules %s, 得到 %s", rulesJSON, got.ParseRules)
	}
}
