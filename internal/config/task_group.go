package config

import (
	"fmt"

	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
)

// ========== 任务组管理 API ==========

// ListTaskGroups 获取所有任务组列表
func ListTaskGroups() ([]models.TaskGroup, error) {
	if DB == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}
	var groups []models.TaskGroup
	if err := DB.Find(&groups).Error; err != nil {
		return nil, err
	}
	return groups, nil
}

// GetTaskGroup 根据 ID 获取单个任务组
func GetTaskGroup(id uint) (*models.TaskGroup, error) {
	if DB == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}
	var group models.TaskGroup
	if err := DB.First(&group, id).Error; err != nil {
		return nil, fmt.Errorf("未找到任务组: %d", id)
	}
	return &group, nil
}

// CreateTaskGroup 创建新任务组
func CreateTaskGroup(group models.TaskGroup) (*models.TaskGroup, error) {
	if DB == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}

	normalizeTaskGroup(&group)

	if err := validateTaskGroup(&group); err != nil {
		return nil, err
	}

	if err := DB.Create(&group).Error; err != nil {
		return nil, err
	}

	logger.Info("Config", "-", "创建任务组: %s (%d)", group.Name, group.ID)
	return &group, nil
}

// UpdateTaskGroup 更新任务组
func UpdateTaskGroup(id uint, group models.TaskGroup) (*models.TaskGroup, error) {
	if DB == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}

	var existing models.TaskGroup
	if err := DB.First(&existing, id).Error; err != nil {
		return nil, fmt.Errorf("任务组不存在: %d", id)
	}

	group.ID = id
	group.CreatedAt = existing.CreatedAt
	normalizeTaskGroup(&group)

	if err := DB.Save(&group).Error; err != nil {
		return nil, err
	}

	logger.Info("Config", "-", "更新任务组: %s (%d)", group.Name, id)
	return &group, nil
}

// DeleteTaskGroup 删除任务组
func DeleteTaskGroup(id uint) error {
	if DB == nil {
		return fmt.Errorf("数据库未初始化")
	}

	var existing models.TaskGroup
	if err := DB.First(&existing, id).Error; err != nil {
		return fmt.Errorf("任务组不存在: %d", id)
	}

	if err := DB.Delete(&models.TaskGroup{}, id).Error; err != nil {
		return err
	}
	logger.Info("Config", "-", "删除任务组: %s (%d)", existing.Name, id)
	return nil
}

// validateTaskGroup 校验任务组
func validateTaskGroup(group *models.TaskGroup) error {
	if group.Name == "" {
		return fmt.Errorf("任务组名称不能为空")
	}
	return nil
}

func normalizeTaskGroup(group *models.TaskGroup) {
	if group == nil {
		return
	}

	if group.TaskType == "" {
		group.TaskType = "normal"
	}
}
