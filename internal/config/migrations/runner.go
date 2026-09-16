package migrations

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
	"gorm.io/gorm"
)

//go:embed *.sql
var sqlFS embed.FS

// Runner 数据库迁移执行器
type Runner struct {
	db *gorm.DB
}

// NewRunner 创建迁移执行器实例
func NewRunner(db *gorm.DB) *Runner {
	return &Runner{db: db}
}

// RunMigrations 执行所有待执行的嵌入式 SQL 迁移脚本
func RunMigrations(db *gorm.DB) error {
	runner := NewRunner(db)
	return runner.Run()
}

// Run 依次执行所有版本迁移
func (r *Runner) Run() error {
	if r.db == nil {
		return fmt.Errorf("数据库连接为空")
	}

	// 1. 确保迁移记录表存在
	if err := r.ensureMigrationTable(); err != nil {
		return fmt.Errorf("初始化迁移记录表失败: %w", err)
	}

	// 2. 读取并排序所有 SQL 文件
	entries, err := sqlFS.ReadDir(".")
	if err != nil {
		return fmt.Errorf("读取嵌入式迁移脚本失败: %w", err)
	}

	var sqlFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			sqlFiles = append(sqlFiles, entry.Name())
		}
	}
	sort.Strings(sqlFiles)

	// 3. 逐个文件执行
	for _, filename := range sqlFiles {
		if err := r.applyMigrationFile(filename); err != nil {
			return fmt.Errorf("执行迁移脚本 [%s] 失败: %w", filename, err)
		}
	}

	return nil
}

func (r *Runner) ensureMigrationTable() error {
	initSQL := `CREATE TABLE IF NOT EXISTS schema_migrations (
		version VARCHAR(64) PRIMARY KEY,
		description VARCHAR(255),
		applied_at DATETIME NOT NULL,
		checksum VARCHAR(64),
		success BOOLEAN DEFAULT 1
	);`
	return r.db.Exec(initSQL).Error
}

func (r *Runner) applyMigrationFile(filename string) error {
	version := strings.TrimSuffix(filename, ".sql")
	contentBytes, err := sqlFS.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("读取脚本内容失败: %w", err)
	}

	h := sha256.Sum256(contentBytes)
	checksum := hex.EncodeToString(h[:])

	// 检查是否已经执行过
	var existing models.SchemaMigration
	err = r.db.Where("version = ?", version).First(&existing).Error
	if err == nil {
		// 已执行过
		if existing.Success {
			if existing.Checksum != checksum {
				logger.Warn("Migration", "-", "迁移脚本 [%s] 内容已被修改，原Checksum=%s, 新Checksum=%s", version, existing.Checksum, checksum)
			}
			return nil
		}
	}

	logger.Info("Migration", "-", "正在执行数据库版本迁移: %s", version)

	// 在事务中执行该 SQL 文件
	tx := r.db.Begin()
	if tx.Error != nil {
		return tx.Error
	}

	contentStr := string(contentBytes)
	statements := splitSQLStatements(contentStr)

	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if err := tx.Exec(stmt).Error; err != nil {
			tx.Rollback()
			// 记录失败
			_ = r.db.Save(&models.SchemaMigration{
				Version:     version,
				Description: filename,
				AppliedAt:   time.Now(),
				Checksum:    checksum,
				Success:     false,
			}).Error
			return fmt.Errorf("执行 SQL 语句失败 [%s]: %w", stmt, err)
		}
	}

	// 记录成功
	record := models.SchemaMigration{
		Version:     version,
		Description: filename,
		AppliedAt:   time.Now(),
		Checksum:    checksum,
		Success:     true,
	}
	if err := tx.Save(&record).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("保存迁移记录失败: %w", err)
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("提交迁移事务失败: %w", err)
	}

	logger.Info("Migration", "-", "数据库版本迁移 [%s] 执行成功", version)
	return nil
}

// splitSQLStatements 简易切分 SQL 语句
func splitSQLStatements(sqlText string) []string {
	var statements []string
	lines := strings.Split(sqlText, "\n")
	var current strings.Builder

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// 忽略纯注释行
		if strings.HasPrefix(trimmed, "--") {
			continue
		}
		current.WriteString(line)
		current.WriteString("\n")
		if strings.HasSuffix(trimmed, ";") {
			statements = append(statements, current.String())
			current.Reset()
		}
	}
	if current.Len() > 0 && strings.TrimSpace(current.String()) != "" {
		statements = append(statements, current.String())
	}
	return statements
}
