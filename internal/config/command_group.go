package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/google/uuid"
)

// CommandGroup 命令组定义
type CommandGroup struct {
	ID          string   `json:"id" gorm:"primaryKey"`          // 唯一标识（UUID）
	Name        string   `json:"name" gorm:"uniqueIndex"`       // 命令组名称
	Description string   `json:"description"`                   // 描述信息
	Commands    []string `json:"commands" gorm:"serializer:json"` // 命令列表
	CreatedAt   string   `json:"createdAt"`                     // 创建时间
	UpdatedAt   string   `json:"updatedAt"`                     // 更新时间
	Tags        []string `json:"tags" gorm:"serializer:json"`   // 标签（用于分类）
}

// 时间格式
const TimeFormat = "2006-01-02T15:04:05"

// nowFormatted 获取当前时间格式化字符串
func nowFormatted() string {
	return time.Now().Format(TimeFormat)
}

// generateID 生成唯一标识
func generateID() string {
	return uuid.New().String()
}

// ========== 命令组管理 API ==========

// ListCommandGroups 获取所有命令组列表
func ListCommandGroups() ([]CommandGroup, error) {
	if DB == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}
	var groups []CommandGroup
	if err := DB.Find(&groups).Error; err != nil {
		return nil, err
	}
	return groups, nil
}

// GetCommandGroup 根据 ID 获取单个命令组
func GetCommandGroup(id string) (*CommandGroup, error) {
	if DB == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}
	var group CommandGroup
	if err := DB.First(&group, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("未找到命令组: %s", id)
	}
	return &group, nil
}

// CreateCommandGroup 创建新命令组
func CreateCommandGroup(group CommandGroup) (*CommandGroup, error) {
	if DB == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}

	group.ID = generateID()
	group.CreatedAt = nowFormatted()
	group.UpdatedAt = group.CreatedAt

	if err := validateCommandGroup(group); err != nil {
		return nil, err
	}

	var count int64
	DB.Model(&CommandGroup{}).Where("name = ?", group.Name).Count(&count)
	if count > 0 {
		return nil, fmt.Errorf("命令组名称已存在: %s", group.Name)
	}

	if err := DB.Create(&group).Error; err != nil {
		return nil, err
	}

	logger.Info("Config", "-", "成功创建命令组: %s", group.Name)
	return &group, nil
}

// UpdateCommandGroup 更新命令组
func UpdateCommandGroup(id string, group CommandGroup) (*CommandGroup, error) {
	if DB == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}

	var existing CommandGroup
	if err := DB.First(&existing, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("未找到命令组: %s", id)
	}

	group.ID = id
	group.CreatedAt = existing.CreatedAt
	group.UpdatedAt = nowFormatted()

	if err := validateCommandGroup(group); err != nil {
		return nil, err
	}

	var count int64
	DB.Model(&CommandGroup{}).Where("name = ? AND id != ?", group.Name, id).Count(&count)
	if count > 0 {
		return nil, fmt.Errorf("命令组名称已存在: %s", group.Name)
	}

	if err := DB.Save(&group).Error; err != nil {
		return nil, err
	}

	logger.Info("Config", "-", "成功更新命令组: %s", group.Name)
	return &group, nil
}

// DeleteCommandGroup 删除命令组
func DeleteCommandGroup(id string) error {
	if DB == nil {
		return fmt.Errorf("数据库未初始化")
	}

	if err := DB.Delete(&CommandGroup{}, "id = ?", id).Error; err != nil {
		return err
	}

	logger.Info("Config", "-", "成功删除命令组: %s", id)
	return nil
}

// DuplicateCommandGroup 复制命令组
func DuplicateCommandGroup(id string) (*CommandGroup, error) {
	if DB == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}

	var source CommandGroup
	if err := DB.First(&source, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("未找到命令组: %s", id)
	}

	newGroup := source
	newGroup.ID = generateID()
	newGroup.CreatedAt = nowFormatted()
	newGroup.UpdatedAt = nowFormatted()
	newGroup.Name = source.Name + " - 副本"

	baseName := newGroup.Name
	counter := 1
	for {
		var count int64
		DB.Model(&CommandGroup{}).Where("name = ?", newGroup.Name).Count(&count)
		if count == 0 {
			break
		}
		counter++
		newGroup.Name = fmt.Sprintf("%s (%d)", baseName, counter)
	}

	if err := DB.Create(&newGroup).Error; err != nil {
		return nil, err
	}

	logger.Info("Config", "-", "成功复制命令组: %s -> %s", source.Name, newGroup.Name)
	return &newGroup, nil
}

// ImportCommandGroup 从 JSON 文件导入命令组
func ImportCommandGroup(filePath string) (*CommandGroup, error) {
	if DB == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取导入文件失败: %v", err)
	}

	var group CommandGroup
	if err := json.Unmarshal(data, &group); err != nil {
		return nil, fmt.Errorf("解析导入文件失败: %v", err)
	}

	group.ID = generateID()
	group.CreatedAt = nowFormatted()
	group.UpdatedAt = nowFormatted()

	if err := validateCommandGroup(group); err != nil {
		return nil, err
	}

	baseName := group.Name
	counter := 1
	for {
		var count int64
		DB.Model(&CommandGroup{}).Where("name = ?", group.Name).Count(&count)
		if count == 0 {
			break
		}
		counter++
		group.Name = fmt.Sprintf("%s (%d)", baseName, counter)
	}

	if err := DB.Create(&group).Error; err != nil {
		return nil, err
	}

	logger.Info("Config", "-", "成功导入命令组: %s", group.Name)
	return &group, nil
}

// ExportCommandGroup 导出命令组到 JSON 文件
func ExportCommandGroup(id string, filePath string) error {
	if DB == nil {
		return fmt.Errorf("数据库未初始化")
	}

	var group CommandGroup
	if err := DB.First(&group, "id = ?", id).Error; err != nil {
		return fmt.Errorf("未找到命令组: %s", id)
	}

	jsonData, err := json.MarshalIndent(group, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化命令组失败: %v", err)
	}

	dir := filepath.Dir(filePath)
	if dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("创建目录失败: %v", err)
		}
	}

	if err := os.WriteFile(filePath, jsonData, 0666); err != nil {
		return fmt.Errorf("写入导出文件失败: %v", err)
	}

	logger.Info("Config", "-", "成功导出命令组到: %s", filePath)
	return nil
}

// validateCommandGroup 校验命令组
func validateCommandGroup(group CommandGroup) error {
	if strings.TrimSpace(group.Name) == "" {
		return fmt.Errorf("命令组名称不能为空")
	}

	if len(group.Commands) == 0 {
		return fmt.Errorf("命令列表不能为空")
	}

	var validCommands []string
	for _, cmd := range group.Commands {
		trimmed := strings.TrimSpace(cmd)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			validCommands = append(validCommands, trimmed)
		}
	}

	if len(validCommands) == 0 {
		return fmt.Errorf("命令列表不能为空")
	}
	return nil
}

// GetCommandGroupCommands 获取指定命令组的命令列表
func GetCommandGroupCommands(id string) ([]string, error) {
	group, err := GetCommandGroup(id)
	if err != nil {
		return nil, err
	}
	return group.Commands, nil
}

// MigrateLegacyCommands 将 config.txt 迁移到命令组（已简化，由 db.go 调用）
func MigrateLegacyCommands() error {
	commands, err := readCommandsLegacy()
	if err != nil {
		return nil
	}

	var count int64
	DB.Model(&CommandGroup{}).Where("name = ?", "默认命令组").Count(&count)
	if count > 0 {
		return nil
	}

	defaultGroup := CommandGroup{
		ID:          generateID(),
		Name:        "默认命令组",
		Description: "从 config.txt 自动迁移的默认命令组",
		Commands:    commands,
		CreatedAt:   nowFormatted(),
		UpdatedAt:   nowFormatted(),
		Tags:        []string{"系统默认"},
	}

	return DB.Create(&defaultGroup).Error
}
