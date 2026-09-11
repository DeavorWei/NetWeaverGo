package config

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/NetWeaverGo/core/internal/models"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// withBackupDir 将备份目录临时重定向到 dir，返回恢复函数。
func withBackupDir(t *testing.T, dir string) {
	t.Helper()
	original := preUpgradeBackupDirResolver
	preUpgradeBackupDirResolver = func() string { return dir }
	t.Cleanup(func() { preUpgradeBackupDirResolver = original })
}

// withAppVersion 临时设置应用版本，返回恢复函数。
func withAppVersion(t *testing.T, v string) {
	t.Helper()
	original := appVersion
	SetAppVersion(v)
	t.Cleanup(func() { appVersion = original })
}

func openFileTestDB(t *testing.T, dbPath string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	// 注册清理并关闭底层连接：Windows 下未关闭句柄会导致 t.TempDir 清理失败。
	sqlDB, err := db.DB()
	if err == nil && sqlDB != nil {
		t.Cleanup(func() { _ = sqlDB.Close() })
	}
	return db
}

func TestIsExistingDatabaseFile(t *testing.T) {
	dir := t.TempDir()

	missing := filepath.Join(dir, "missing.db")
	if IsExistingDatabaseFile(missing) {
		t.Fatal("不存在的文件不应判定为既有库")
	}

	empty := filepath.Join(dir, "empty.db")
	if err := os.WriteFile(empty, nil, 0644); err != nil {
		t.Fatalf("创建空文件失败: %v", err)
	}
	if IsExistingDatabaseFile(empty) {
		t.Fatal("空文件不应判定为既有库")
	}

	nonEmpty := filepath.Join(dir, "full.db")
	if err := os.WriteFile(nonEmpty, []byte("sqlite-header"), 0644); err != nil {
		t.Fatalf("创建非空文件失败: %v", err)
	}
	if !IsExistingDatabaseFile(nonEmpty) {
		t.Fatal("非空文件应判定为既有库")
	}
}

func TestEnsurePreUpgradeBackup_FirstInstallSkips(t *testing.T) {
	dir := t.TempDir()
	backupDir := filepath.Join(dir, "backup")
	withBackupDir(t, backupDir)

	dbPath := filepath.Join(dir, "netweaver.db")
	db := openFileTestDB(t, dbPath)

	// isExistingDB=false 模拟首次安装
	backedUp, err := EnsurePreUpgradeBackup(db, dbPath, false)
	if err != nil {
		t.Fatalf("首次安装不应报错: %v", err)
	}
	if backedUp {
		t.Fatal("首次安装不应触发备份")
	}
	entries, _ := os.ReadDir(backupDir)
	if len(entries) != 0 {
		t.Fatalf("首次安装不应产生备份文件，实际 %d 个", len(entries))
	}
}

func TestEnsurePreUpgradeBackup_ExistingDBWithoutTableBacksUp(t *testing.T) {
	dir := t.TempDir()
	backupDir := filepath.Join(dir, "backup", "db")
	withBackupDir(t, backupDir)
	withAppVersion(t, "v9.9.9")

	dbPath := filepath.Join(dir, "netweaver.db")
	db := openFileTestDB(t, dbPath)
	// 制造非空库，且**不创建** runtime_settings 表（模拟极早期版本）
	if err := db.Exec("CREATE TABLE legacy_only (id INTEGER PRIMARY KEY)").Error; err != nil {
		t.Fatalf("创建历史表失败: %v", err)
	}

	if !IsExistingDatabaseFile(dbPath) {
		t.Fatal("测试前置：库文件应为非空")
	}

	backedUp, err := EnsurePreUpgradeBackup(db, dbPath, true)
	if err != nil {
		t.Fatalf("既有库备份不应报错: %v", err)
	}
	if !backedUp {
		t.Fatal("无 runtime_settings 表时必须触发备份")
	}

	entries, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatalf("读取备份目录失败: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("预期 1 个备份文件，实际 %d 个", len(entries))
	}
	if filepath.Ext(entries[0].Name()) != ".db" {
		t.Fatalf("备份文件名应以 .db 结尾，实际 %s", entries[0].Name())
	}
}

func TestEnsurePreUpgradeBackup_SkipWhenVersionMatched(t *testing.T) {
	dir := t.TempDir()
	backupDir := filepath.Join(dir, "backup", "db")
	withBackupDir(t, backupDir)
	withAppVersion(t, "v1.2.3")

	dbPath := filepath.Join(dir, "netweaver.db")
	db := openFileTestDB(t, dbPath)
	if err := db.Exec("CREATE TABLE seed (id INTEGER PRIMARY KEY)").Error; err != nil {
		t.Fatalf("建表失败: %v", err)
	}
	if err := db.AutoMigrate(&models.RuntimeSetting{}); err != nil {
		t.Fatalf("迁移 runtime_settings 失败: %v", err)
	}
	if err := CommitPreUpgradeBackupVersion(db); err != nil {
		t.Fatalf("写入标记失败: %v", err)
	}

	backedUp, err := EnsurePreUpgradeBackup(db, dbPath, true)
	if err != nil {
		t.Fatalf("已备份版本不应报错: %v", err)
	}
	if backedUp {
		t.Fatal("版本标记一致时应跳过备份")
	}
	entries, _ := os.ReadDir(backupDir)
	if len(entries) != 0 {
		t.Fatalf("跳过后不应产生备份文件，实际 %d 个", len(entries))
	}
}

func TestCommitPreUpgradeBackupVersion_Upsert(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "netweaver.db")
	db := openFileTestDB(t, dbPath)
	if err := db.AutoMigrate(&models.RuntimeSetting{}); err != nil {
		t.Fatalf("迁移 runtime_settings 失败: %v", err)
	}

	withAppVersion(t, "v2.0.0")
	if err := CommitPreUpgradeBackupVersion(db); err != nil {
		t.Fatalf("首次提交失败: %v", err)
	}
	// 二次提交应走 Upsert，不报唯一键冲突
	withAppVersion(t, "v2.0.1")
	if err := CommitPreUpgradeBackupVersion(db); err != nil {
		t.Fatalf("二次提交（Upsert）失败: %v", err)
	}

	var rec models.RuntimeSetting
	if err := db.Where("key = ?", preUpgradeBackupVersionKey).First(&rec).Error; err != nil {
		t.Fatalf("查询标记失败: %v", err)
	}
	if rec.Value != "v2.0.1" {
		t.Fatalf("标记应被更新为 v2.0.1，实际 %s", rec.Value)
	}
}

func TestRotatePreUpgradeBackups_KeepLatest(t *testing.T) {
	dir := t.TempDir()
	base := time.Now().Add(-10 * time.Hour)
	for i := 0; i < 7; i++ {
		name := fmt.Sprintf("netweaver_v1_2026010%d_000000.db", i)
		full := filepath.Join(dir, name)
		if err := os.WriteFile(full, []byte("x"), 0644); err != nil {
			t.Fatalf("创建备份文件失败: %v", err)
		}
		ts := base.Add(time.Duration(i) * time.Hour)
		if err := os.Chtimes(full, ts, ts); err != nil {
			t.Fatalf("设置文件时间失败: %v", err)
		}
	}

	rotatePreUpgradeBackups(dir)

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("读取目录失败: %v", err)
	}
	if len(entries) != preUpgradeBackupKeep {
		t.Fatalf("应保留 %d 份备份，实际 %d 份", preUpgradeBackupKeep, len(entries))
	}
	// 最旧的 i=0、i=1 应被删除
	for _, gone := range []string{"netweaver_v1_20260100_000000.db", "netweaver_v1_20260101_000000.db"} {
		if _, statErr := os.Stat(filepath.Join(dir, gone)); !os.IsNotExist(statErr) {
			t.Fatalf("最旧备份 %s 应被淘汰", gone)
		}
	}
}
