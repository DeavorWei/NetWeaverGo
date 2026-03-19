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

	var count int64
	DB.Model(&models.CommandGroup{}).Where("name = ?", group.Name).Count(&count)
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
func UpdateCommandGroup(id uint, group models.CommandGroup) (*models.CommandGroup, error) {
	if DB == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}

	var existing models.CommandGroup
	if err := DB.First(&existing, id).Error; err != nil {
		return nil, fmt.Errorf("未找到命令组: %d", id)
	}

	group.ID = id
	group.Name = strings.TrimSpace(group.Name)
	if group.Name == "" {
		return nil, fmt.Errorf("命令组名称不能为空")
	}

	var count int64
	DB.Model(&models.CommandGroup{}).Where("name = ? AND id != ?", group.Name, id).Count(&count)
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

	var source models.CommandGroup
	if err := DB.First(&source, id).Error; err != nil {
		return nil, fmt.Errorf("未找到命令组: %d", id)
	}

	newGroup := source
	newGroup.ID = 0
	newGroup.Name = source.Name + " - 副本"

	baseName := newGroup.Name
	counter := 1
	for {
		var count int64
		DB.Model(&models.CommandGroup{}).Where("name = ?", newGroup.Name).Count(&count)
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

	group.Name = strings.TrimSpace(group.Name)
	if group.Name == "" {
		return nil, fmt.Errorf("命令组名称不能为空")
	}

	baseName := group.Name
	counter := 1
	for {
		var count int64
		DB.Model(&models.CommandGroup{}).Where("name = ?", group.Name).Count(&count)
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
