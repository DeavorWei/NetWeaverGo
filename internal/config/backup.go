package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	// preUpgradeBackupVersionKey 记录"最近一次已成功备份（且已完成全部迁移）的版本"的 runtime_settings.key
	preUpgradeBackupVersionKey = "last_backup_version"
	// preUpgradeBackupKeep 升级前备份保留份数（超出按修改时间倒序淘汰）
	preUpgradeBackupKeep = 5
	// preUpgradeBackupSlowThreshold 备份耗时告警阈值
	preUpgradeBackupSlowThreshold = 3 * time.Second
)

// preUpgradeBackupDirResolver 解析备份目录；抽为变量便于单测注入临时目录。
var preUpgradeBackupDirResolver = func() string {
	return GetPathManager().GetDBBackupDir()
}

// IsExistingDatabaseFile 判断给定路径是否已存在非空数据库文件。
// 必须在打开数据库之前调用：SQLite 一旦被 Open/查询就会创建主库文件，
// 打开后再 stat 无法区分"首次安装"与"既有库"。
func IsExistingDatabaseFile(dbPath string) bool {
	if strings.TrimSpace(dbPath) == "" {
		return false
	}
	fi, err := os.Stat(dbPath)
	if err != nil {
		return false
	}
	return fi.Size() > 0
}

// EnsurePreUpgradeBackup 在**任何表迁移之前**执行升级前自动备份。
//
// 语义：
//   - isExistingDB=false（首次安装）→ 不做任何操作；
//   - 探针命中且版本与当前一致 → 视为已备份，跳过；
//   - 其余情况（含无 runtime_settings 表的极早期库）→ 备份主库到 <StorageRoot>/backup/db/。
//
// 返回 (是否实际执行了备份, error)。任何失败都只返回 error 供调用方告警，
// 调用方必须保证不因此阻断启动。
func EnsurePreUpgradeBackup(db *gorm.DB, dbPath string, isExistingDB bool) (bool, error) {
	if db == nil || !isExistingDB {
		return false, nil
	}

	if preUpgradeBackupAlreadyDone(db) {
		logger.Verbose("Config", "-", "检测到当前版本已完成升级前备份，跳过: version=%s", appVersion)
		return false, nil
	}

	target := buildPreUpgradeBackupPath()
	start := time.Now()
	// MirrorDatabaseToPath 内部依赖包级 DB 做 WAL checkpoint，
	// 因此调用前必须已执行 DB = db（InitDB 中 DB 赋值早于本函数调用）。
	if err := MirrorDatabaseToPath(dbPath, target); err != nil {
		return false, fmt.Errorf("复制主库失败: %w", err)
	}

	cost := time.Since(start)
	if cost > preUpgradeBackupSlowThreshold {
		logger.Warn("Config", "-", "升级前数据库备份耗时较长: %s, target=%s", cost, target)
	} else {
		logger.Info("Config", "-", "已完成升级前数据库备份: %s (耗时 %s)", target, cost)
	}

	rotatePreUpgradeBackups(filepath.Dir(target))
	return true, nil
}

// CommitPreUpgradeBackupVersion 在所有迁移（config.InitDB + taskexec.AutoMigrate）全部成功后
// 回写"已备份版本"标记。刻意滞后写入：宁可多备份一次，也绝不在迁移失败时误判为已备份。
func CommitPreUpgradeBackupVersion(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	record := models.RuntimeSetting{
		Category: "system",
		Key:      preUpgradeBackupVersionKey,
		Value:    appVersion,
	}
	return db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.Assignments(map[string]interface{}{"value": appVersion, "updated_at": time.Now()}),
	}).Create(&record).Error
}

// preUpgradeBackupAlreadyDone 使用底层只读探针判断是否已备份。
// 任何报错（含 no such table / no rows）一律视为"尚未备份"。
func preUpgradeBackupAlreadyDone(db *gorm.DB) bool {
	sqlDB, err := db.DB()
	if err != nil || sqlDB == nil {
		return false
	}
	var lastVersion string
	row := sqlDB.QueryRow("SELECT value FROM runtime_settings WHERE key = ? LIMIT 1", preUpgradeBackupVersionKey)
	if err := row.Scan(&lastVersion); err != nil {
		return false
	}
	return strings.TrimSpace(lastVersion) == appVersion
}

// buildPreUpgradeBackupPath 生成备份文件路径：<备份目录>/netweaver_<version>_<ts>.db
func buildPreUpgradeBackupPath() string {
	dir := preUpgradeBackupDirResolver()
	version := sanitizeVersionToken(appVersion)
	ts := time.Now().Format("20060102_150405")
	return filepath.Join(dir, fmt.Sprintf("netweaver_%s_%s.db", version, ts))
}

// sanitizeVersionToken 将版本号中的非文件名字符替换为下划线；空值回退 dev。
func sanitizeVersionToken(v string) string {
	trimmed := strings.TrimSpace(v)
	if trimmed == "" {
		return "dev"
	}
	var b strings.Builder
	for _, r := range trimmed {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return b.String()
}

// rotatePreUpgradeBackups 仅保留最近 preUpgradeBackupKeep 份备份（连同 -wal/-shm 一并清理）。
func rotatePreUpgradeBackups(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	type backupFile struct {
		name    string
		modTime time.Time
	}
	var mains []backupFile
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, "netweaver_") || !strings.HasSuffix(name, ".db") {
			continue
		}
		info, infoErr := e.Info()
		if infoErr != nil {
			continue
		}
		mains = append(mains, backupFile{name: name, modTime: info.ModTime()})
	}

	if len(mains) <= preUpgradeBackupKeep {
		return
	}

	sort.Slice(mains, func(i, j int) bool { return mains[i].modTime.After(mains[j].modTime) })
	for _, stale := range mains[preUpgradeBackupKeep:] {
		// WAL/SHM 文件名为 <主库文件>-wal / -shm（含 .db 后缀）
		for _, suffix := range []string{"", "-wal", "-shm"} {
			_ = os.Remove(filepath.Join(dir, stale.name+suffix))
		}
	}
}
