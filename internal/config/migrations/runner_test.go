package migrations

import (
	"testing"

	"github.com/NetWeaverGo/core/internal/models"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestRunner_Run(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:mem_test_migration?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("连接测试数据库失败: %v", err)
	}

	runner := NewRunner(db)

	// 首次运行迁移
	err = runner.Run()
	if err != nil {
		t.Fatalf("首次执行迁移失败: %v", err)
	}

	// 校验 schema_migrations 表记录
	var records []models.SchemaMigration
	if err := db.Find(&records).Error; err != nil {
		t.Fatalf("查询迁移记录失败: %v", err)
	}
	if len(records) == 0 {
		t.Errorf("预期存在迁移记录，实际为 0")
	}

	baselineFound := false
	for _, rec := range records {
		if rec.Version == "0001_baseline" {
			baselineFound = true
			if !rec.Success {
				t.Errorf("0001_baseline 预期执行成功")
			}
			if rec.Checksum == "" {
				t.Errorf("Checksum 字段不应为空")
			}
		}
	}
	if !baselineFound {
		t.Errorf("未找到 0001_baseline 迁移记录")
	}

	// 校验基线创建的表是否存在
	if !db.Migrator().HasTable("device_assets") {
		t.Errorf("device_assets 表未成功创建")
	}

	// 再次幂等运行迁移，验证无报错
	err = runner.Run()
	if err != nil {
		t.Fatalf("重复执行迁移应幂等无误，实际出错: %v", err)
	}
}
