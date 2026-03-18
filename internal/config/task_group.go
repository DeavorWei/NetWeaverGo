package config

import (
	"fmt"

	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/google/uuid"
)

// 有效状态值常量
var validStatuses = map[string]bool{
	"pending":   true,
	"running":   true,
	"completed": true,
	"failed":    true,
}

// TaskItem 单个任务项（一组命令绑定一组设备）
type TaskItem struct {
	CommandGroupID string   `json:"commandGroupId"` // 命令组ID（模式A使用）
	Commands       []string `json:"commands"`       // 直接命令列表（模式B使用）
	DeviceIDs      []uint   `json:"deviceIDs"`      // 绑定的设备ID列表
}

// TaskGroup 任务组
type TaskGroup struct {
	ID           string     `json:"id" gorm:"primaryKey"`
	Name         string     `json:"name"`
	Description  string     `json:"description"`
	Mode         string     `json:"mode"`                         // "group" 模式A | "binding" 模式B
	Items        []TaskItem `json:"items" gorm:"serializer:json"` // 任务项列表
	Tags         []string   `json:"tags" gorm:"serializer:json"`  // 标签
	EnableRawLog bool       `json:"enableRawLog"`
	Status       string     `json:"status"` // "pending" | "running" | "completed" | "failed"
	CreatedAt    string     `json:"createdAt"`
	UpdatedAt    string     `json:"updatedAt"`
}

// ========== 任务组管理 API ==========

// ListTaskGroups 获取所有任务组列表
func ListTaskGroups() ([]TaskGroup, error) {
	if DB == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}
	var groups []TaskGroup
	if err := DB.Find(&groups).Error; err != nil {
		return nil, err
	}
	return groups, nil
}

// GetTaskGroup 根据 ID 获取单个任务组
func GetTaskGroup(id string) (*TaskGroup, error) {
	if DB == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}
	var group TaskGroup
	if err := DB.First(&group, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("未找到任务组: %s", id)
	}
	return &group, nil
}

// CreateTaskGroup 创建新任务组
func CreateTaskGroup(group TaskGroup) (*TaskGroup, error) {
	if DB == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}

	if err := validateTaskGroup(group); err != nil {
		return nil, err
	}

	group.ID = uuid.New().String()
	group.CreatedAt = nowFormatted()
	group.UpdatedAt = nowFormatted()
	if group.Status == "" {
		group.Status = "pending"
	}
	if group.Tags == nil {
		group.Tags = []string{}
	}

	if err := DB.Create(&group).Error; err != nil {
		return nil, err
	}

	logger.Info("Config", "-", "创建任务组: %s (%s)", group.Name, group.ID)
	return &group, nil
}

// UpdateTaskGroup 更新任务组
func UpdateTaskGroup(id string, group TaskGroup) (*TaskGroup, error) {
	if DB == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}

	var existing TaskGroup
	if err := DB.First(&existing, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("任务组不存在: %s", id)
	}

	group.ID = id
	group.CreatedAt = existing.CreatedAt
	group.UpdatedAt = nowFormatted()
	if group.Tags == nil {
		group.Tags = []string{}
	}

	if err := DB.Save(&group).Error; err != nil {
		return nil, err
	}

	logger.Info("Config", "-", "更新任务组: %s (%s)", group.Name, id)
	return &group, nil
}

// DeleteTaskGroup 删除任务组
func DeleteTaskGroup(id string) error {
	if DB == nil {
		return fmt.Errorf("数据库未初始化")
	}

	var existing TaskGroup
	if err := DB.First(&existing, "id = ?", id).Error; err != nil {
		return fmt.Errorf("任务组不存在: %s", id)
	}

	if err := DB.Delete(&TaskGroup{}, "id = ?", id).Error; err != nil {
		return err
	}
	logger.Info("Config", "-", "删除任务组: %s (%s)", existing.Name, id)
	return nil
}

// UpdateTaskGroupStatus 更新任务组状态
func UpdateTaskGroupStatus(id string, status string) error {
	if !validStatuses[status] {
		return fmt.Errorf("无效的状态值: %s (应为 pending/running/completed/failed)", status)
	}

	if DB == nil {
		return fmt.Errorf("数据库未初始化")
	}

	var group TaskGroup
	if err := DB.First(&group, "id = ?", id).Error; err != nil {
		return fmt.Errorf("任务组不存在: %s", id)
	}

	group.Status = status
	group.UpdatedAt = nowFormatted()

	return DB.Save(&group).Error
}

// validateTaskGroup 校验任务组
func validateTaskGroup(group TaskGroup) error {
	if group.Name == "" {
		return fmt.Errorf("任务组名称不能为空")
	}
	if group.Mode != "group" && group.Mode != "binding" {
		return fmt.Errorf("无效的任务模式: %s (应为 group 或 binding)", group.Mode)
	}
	if len(group.Items) == 0 {
		return fmt.Errorf("任务项不能为空")
	}
	for i, item := range group.Items {
		if len(item.DeviceIDs) == 0 {
			return fmt.Errorf("第 %d 个任务项的设备列表不能为空", i+1)
		}
		if group.Mode == "group" && item.CommandGroupID == "" {
			return fmt.Errorf("模式A下命令组ID不能为空")
		}
		if group.Mode == "binding" && len(item.Commands) == 0 {
			return fmt.Errorf("第 %d 个任务项的命令列表不能为空", i+1)
		}
	}
	return nil
}
