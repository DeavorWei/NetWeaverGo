package optical

import (
	"testing"

	"github.com/NetWeaverGo/core/internal/models"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// P1-10：弱光检测规则种子应存在且幂等落库
func TestEnsureOpticalCheckSeeds_Idempotent(t *testing.T) {
	seeds := DefaultOpticalRules()
	if len(seeds) == 0 {
		t.Fatal("内置弱光检测规则不应为空")
	}
	for _, r := range seeds {
		if r.RuleName == "" || !r.Enabled {
			t.Fatalf("种子规则应命名且默认启用: %+v", r)
		}
		if r.Mode != models.OpticalModeAbsolute && r.Mode != models.OpticalModePercent {
			t.Fatalf("种子规则模式非法: %+v", r)
		}
	}

	db, err := gorm.Open(sqlite.Open("file:optical_seed_test?mode=memory&cache=shared"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("打开内存数据库失败: %v", err)
	}
	if err := db.AutoMigrate(&models.OpticalCheckRule{}); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}

	if err := EnsureOpticalCheckSeeds(db); err != nil {
		t.Fatalf("首次落库失败: %v", err)
	}
	if err := EnsureOpticalCheckSeeds(db); err != nil {
		t.Fatalf("重复落库应幂等: %v", err)
	}

	var count int64
	if err := db.Model(&models.OpticalCheckRule{}).Count(&count).Error; err != nil {
		t.Fatalf("统计失败: %v", err)
	}
	if int(count) != len(seeds) {
		t.Fatalf("落库规则数 = %d, want %d（幂等失效）", count, len(seeds))
	}
}
