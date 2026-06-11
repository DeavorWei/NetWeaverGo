package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
	"github.com/google/uuid"
)

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

// isDuplicateKeyError 检测是否为唯一约束冲突错误
func isDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()
	return strings.Contains(errMsg, "UNIQUE constraint failed") ||
		strings.Contains(errMsg, "Duplicate entry") ||
		strings.Contains(errMsg, "duplicate key")
}

// handleDuplicateError 处理唯一约束冲突，返回友好错误信息
func handleDuplicateError(err error, name string) error {
	if isDuplicateKeyError(err) {
		return fmt.Errorf("命令组名称已存在: %s", name)
	}
	return err
}

// generateUniqueName 生成唯一名称（单次查询，避免N+1问题）
func generateUniqueName(baseName string) (string, error) {
	var existing []models.CommandGroup
	// 单次查询所有相关名称
	if err := DB.Where("name LIKE ?", baseName+"%").Find(&existing).Error; err != nil {
		return "", err
	}

	// 构建已存在名称的集合
	nameSet := make(map[string]bool)
	for _, g := range existing {
		nameSet[g.Name] = true
	}

	// 尝试生成唯一名称（最多尝试100次）
	if !nameSet[baseName] {
		return baseName, nil
	}

	for i := 1; i <= 100; i++ {
		candidate := fmt.Sprintf("%s (%d)", baseName, i)
		if !nameSet[candidate] {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("无法生成唯一名称，已存在过多同名命令组")
}

// ========== 命令组管理 API ==========

// ListCommandGroups 获取所有命令组列表
func ListCommandGroups() ([]models.CommandGroup, error) {
	if DB == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}
	var groups []models.CommandGroup
	if err := DB.Find(&groups).Error; err != nil {
		return nil, err
	}
	return groups, nil
}

// GetCommandGroup 根据 ID 获取单个命令组
func GetCommandGroup(id uint) (*models.CommandGroup, error) {
	if DB == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}
	var group models.CommandGroup
	if err := DB.First(&group, id).Error; err != nil {
		return nil, fmt.Errorf("未找到命令组: %d", id)
	}
	return &group, nil
}

// CreateCommandGroup 创建新命令组
func CreateCommandGroup(group models.CommandGroup) (*models.CommandGroup, error) {
	if DB == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}

	group.Name = strings.TrimSpace(group.Name)
	if group.Name == "" {
		return nil, fmt.Errorf("命令组名称不能为空")
	}

	// 移除手动检查，直接创建，让数据库唯一约束处理（修复问题05竞态条件）
	if err := DB.Create(&group).Error; err != nil {
		return nil, handleDuplicateError(err, group.Name)
	}

	logger.Info("Config", "-", "成功创建命令组: %s", group.Name)
	return &group, nil
}

// UpdateCommandGroup 更新命令组
func UpdateCommandGroup(id uint, group models.CommandGroup) (*models.CommandGroup, error) {
	if DB == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}

	var existing models.CommandGroup
	if err := DB.First(&existing, id).Error; err != nil {
		return nil, fmt.Errorf("未找到命令组: %d", id)
	}

	group.ID = id
	// 保留原有创建时间（修复问题11：更新操作未保留创建时间）
	group.CreatedAt = existing.CreatedAt
	group.Name = strings.TrimSpace(group.Name)
	if group.Name == "" {
		return nil, fmt.Errorf("命令组名称不能为空")
	}

	// 移除手动检查，直接保存，让数据库唯一约束处理（修复问题05竞态条件）
	if err := DB.Save(&group).Error; err != nil {
		return nil, handleDuplicateError(err, group.Name)
	}

	logger.Info("Config", "-", "成功更新命令组: %s", group.Name)
	return &group, nil
}

// DeleteCommandGroup 删除命令组
func DeleteCommandGroup(id uint) error {
	if DB == nil {
		return fmt.Errorf("数据库未初始化")
	}

	if err := DB.Delete(&models.CommandGroup{}, id).Error; err != nil {
		return err
	}

	logger.Info("Config", "-", "成功删除命令组: %d", id)
	return nil
}

// DuplicateCommandGroup 复制命令组
func DuplicateCommandGroup(id uint) (*models.CommandGroup, error) {
	if DB == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}

	var original models.CommandGroup
	if err := DB.First(&original, id).Error; err != nil {
		return nil, fmt.Errorf("未找到命令组: %d", id)
	}

	newGroup := models.CommandGroup{
		Name:        original.Name,
		Commands:    make([]string, len(original.Commands)),
		Tags:        make([]string, len(original.Tags)),
		Description: original.Description,
	}
	copy(newGroup.Commands, original.Commands)
	copy(newGroup.Tags, original.Tags)
	newGroup.ID = 0

	// 使用 generateUniqueName 生成唯一名称（修复问题06循环查询）
	uniqueName, err := generateUniqueName(original.Name)
	if err != nil {
		return nil, err
	}
	newGroup.Name = uniqueName

	// 尝试创建，如果仍然冲突则重试（处理并发场景）
	maxRetries := 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		if err := DB.Create(&newGroup).Error; err != nil {
			if isDuplicateKeyError(err) && attempt < maxRetries-1 {
				// 重试：重新生成名称
				uniqueName, err = generateUniqueName(original.Name + " 副本")
				if err != nil {
					return nil, err
				}
				newGroup.Name = uniqueName
				newGroup.ID = 0 // 重置ID
				continue
			}
			return nil, handleDuplicateError(err, newGroup.Name)
		}
		logger.Info("Config", "-", "成功复制命令组: %s -> %s", original.Name, newGroup.Name)
		return &newGroup, nil
	}

	return nil, fmt.Errorf("复制失败，请重试")
}

// ImportCommandGroup 从 JSON 文件导入命令组
func ImportCommandGroup(filePath string) (*models.CommandGroup, error) {
	if DB == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取导入文件失败: %v", err)
	}

	var group models.CommandGroup
	if err := json.Unmarshal(data, &group); err != nil {
		return nil, fmt.Errorf("解析导入文件失败: %v", err)
	}

	// 清除ID和时间戳字段（修复问题10：导入时ID字段未清除）
	group.ID = 0
	group.CreatedAt = time.Time{}
	group.UpdatedAt = time.Time{}

	group.Name = strings.TrimSpace(group.Name)
	if group.Name == "" {
		return nil, fmt.Errorf("命令组名称不能为空")
	}

	// 使用 generateUniqueName 生成唯一名称（修复问题09循环查询）
	uniqueName, err := generateUniqueName(group.Name)
	if err != nil {
		return nil, err
	}
	group.Name = uniqueName

	// 尝试创建，如果仍然冲突则重试（处理并发场景）
	maxRetries := 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		if err := DB.Create(&group).Error; err != nil {
			if isDuplicateKeyError(err) && attempt < maxRetries-1 {
				uniqueName, err = generateUniqueName(group.Name + " 导入")
				if err != nil {
					return nil, err
				}
				group.Name = uniqueName
				group.ID = 0
				continue
			}
			return nil, handleDuplicateError(err, group.Name)
		}
		logger.Info("Config", "-", "成功导入命令组: %s", group.Name)
		return &group, nil
	}

	return nil, fmt.Errorf("导入失败，请重试")
}

// ExportCommandGroup 导出命令组到 JSON 文件
func ExportCommandGroup(id uint, filePath string) error {
	if DB == nil {
		return fmt.Errorf("数据库未初始化")
	}

	var group models.CommandGroup
	if err := DB.First(&group, id).Error; err != nil {
		return fmt.Errorf("未找到命令组: %d", id)
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
