package models

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// P3-4：能力键口径统一 —— 落库种子使用的键必须全部来自 Capability* 常量集合
func TestDeviceCapabilitySeeds_KeyUnified(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:device_capability_test?mode=memory&cache=shared"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("打开内存数据库失败: %v", err)
	}
	if err := db.AutoMigrate(&DeviceCapability{}); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	if err := EnsureDeviceCapabilitySeeds(db); err != nil {
		t.Fatalf("落库种子失败: %v", err)
	}

	valid := map[string]bool{
		CapabilityInspection:  true,
		CapabilityTopology:    true,
		CapabilityBizCompare:  true,
		CapabilityBatchPing:   true,
		CapabilityConfigBuild: true,
		CapabilityOptical:     true,
		CapabilityHardware:    true,
	}

	var rows []DeviceCapability
	if err := db.Find(&rows).Error; err != nil {
		t.Fatalf("读取种子失败: %v", err)
	}
	if len(rows) == 0 {
		t.Fatal("能力种子不应为空")
	}
	for _, r := range rows {
		if !valid[r.CapabilityKey] {
			t.Fatalf("种子使用了未登记的能力键: %s", r.CapabilityKey)
		}
	}
}
