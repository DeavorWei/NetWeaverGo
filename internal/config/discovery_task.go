package config

import (
	"fmt"

	"github.com/NetWeaverGo/core/internal/models"
)

// BindDiscoveryTaskToTaskGroup 绑定发现任务到任务组
func BindDiscoveryTaskToTaskGroup(taskID string, taskGroupID uint) error {
	if DB == nil {
		return fmt.Errorf("数据库未初始化")
	}

	if taskID == "" || taskGroupID == 0 {
		return fmt.Errorf("无效参数: taskID=%q taskGroupID=%d", taskID, taskGroupID)
	}

	if err := DB.Model(&models.DiscoveryTask{}).
		Where("id = ?", taskID).
		Update("task_group_id", taskGroupID).Error; err != nil {
		return fmt.Errorf("绑定发现任务与任务组失败: %v", err)
	}

	return nil
}
