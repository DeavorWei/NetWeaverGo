package config

import (
	"sync"
	"testing"

	"github.com/NetWeaverGo/core/internal/models"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("创建内存数据库失败: %v", err)
	}
	if err := db.AutoMigrate(&models.GlobalSettings{}); err != nil {
		t.Fatalf("数据库迁移失败: %v", err)
	}
	oldDB := DB
	DB = db
	t.Cleanup(func() {
		DB = oldDB
		settingsMu.Lock()
		cachedSettings = nil
		settingsMu.Unlock()
	})
	return db
}

// TestGetGlobalSettings_NoDeadlock 验证缓存为空时调用 GetGlobalSettings 绝不死锁
func TestGetGlobalSettings_NoDeadlock(t *testing.T) {
	setupTestDB(t)

	// 强制清空缓存模拟冷启动
	settingsMu.Lock()
	cachedSettings = nil
	settingsMu.Unlock()

	// 此时 cachedSettings=nil，必须能无死锁从 DB 初始化并返回设置
	st := GetGlobalSettings()
	if st == nil {
		t.Fatal("GetGlobalSettings 返回 nil")
	}
	if st.RiskCommandMode != "warn" {
		t.Errorf("默认 RiskCommandMode 期望 warn，得到 %s", st.RiskCommandMode)
	}

	// 再次调用（走缓存路径）
	st2 := GetGlobalSettings()
	if st2 == nil || st2.RiskCommandMode != "warn" {
		t.Fatal("从缓存读取设置异常")
	}
}

// TestGetGlobalSettings_Concurrency 并发读写压力测试
func TestGetGlobalSettings_Concurrency(t *testing.T) {
	setupTestDB(t)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			st := GetGlobalSettings()
			if st == nil {
				t.Errorf("并发获取设置得到 nil")
				return
			}
			if idx%3 == 0 {
				newSt := *st
				newSt.CommandTimeout = "45s"
				SetGlobalSettings(newSt)
			}
		}(i)
	}
	wg.Wait()

	finalSt := GetGlobalSettings()
	if finalSt == nil {
		t.Fatal("最终设置不应为 nil")
	}
}
