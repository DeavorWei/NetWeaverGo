package config

import (
	"fmt"
	"strings"

	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
)

// ========== 任务组管理 API ==========

// ListTaskGroups 获取所有任务组列表
func ListTaskGroups() ([]models.TaskGroup, error) {
	if DB == nil {
		logger.Error("Config", "-", "ListTaskGroups 失败: 数据库未初始化")
		return nil, fmt.Errorf("数据库未初始化")
	}

	logger.Debug("Config", "-", "开始查询任务组列表")
	var groups []models.TaskGroup
	if err := DB.Order("created_at DESC").Find(&groups).Error; err != nil {
		logger.Error("Config", "-", "查询任务组列表失败: %v", err)
		return nil, err
	}

	for i := range groups {
		normalizeTaskGroup(&groups[i])
		logger.Verbose("Config", "-", "任务组列表项[%d]: id=%d, name=%s, mode=%s, taskType=%s, items=%d, tags=%d", i, groups[i].ID, groups[i].Name, groups[i].Mode, groups[i].TaskType, len(groups[i].Items), len(groups[i].Tags))
	}
	logger.Debug("Config", "-", "任务组列表查询完成: count=%d", len(groups))
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
		logger.Error("Config", "-", "CreateTaskGroup 失败: 数据库未初始化")
		return nil, fmt.Errorf("数据库未初始化")
	}

	logger.Debug("Config", "-", "准备创建任务组: name=%s, mode=%s, taskType=%s, items=%d, tags=%d", group.Name, group.Mode, group.TaskType, len(group.Items), len(group.Tags))
	normalizeTaskGroup(&group)
	logger.Verbose("Config", "-", "任务组归一化完成: name=%s, mode=%s, taskType=%s, commandGroup=%s, items=%d, tags=%d, enableRawLog=%t", group.Name, group.Mode, group.TaskType, group.CommandGroup, len(group.Items), len(group.Tags), group.EnableRawLog)

	if err := validateTaskGroup(&group); err != nil {
		logger.Error("Config", "-", "任务组校验失败: name=%s, err=%v", group.Name, err)
		return nil, err
	}

	if err := DB.Create(&group).Error; err != nil {
		logger.Error("Config", "-", "任务组落库失败: name=%s, err=%v", group.Name, err)
		return nil, err
	}

	logger.Info("Config", "-", "创建任务组: %s (%d)", group.Name, group.ID)
	logger.Debug("Config", "-", "任务组创建成功: id=%d, mode=%s, taskType=%s, items=%d", group.ID, group.Mode, group.TaskType, len(group.Items))
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
	if err := validateTaskGroup(&group); err != nil {
		return nil, err
	}

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
	if group == nil {
		return fmt.Errorf("任务组不能为空")
	}
	if strings.TrimSpace(group.Name) == "" {
		return fmt.Errorf("任务组名称不能为空")
	}
	if strings.TrimSpace(group.TaskType) != "topology" {
		return nil
	}

	catalog := make(map[string]models.TopologyFieldSpec)
	for _, spec := range topologyFieldCatalogForValidation() {
		fieldKey := strings.TrimSpace(spec.FieldKey)
		if fieldKey == "" {
			continue
		}
		catalog[fieldKey] = spec
	}
	if len(catalog) == 0 {
		return fmt.Errorf("拓扑字段目录为空")
	}

	overrides := group.TopologyFieldOverrides
	seen := make(map[string]struct{}, len(overrides))
	for _, item := range overrides {
		fieldKey := strings.TrimSpace(item.FieldKey)
		if fieldKey == "" {
			return fmt.Errorf("拓扑字段覆盖存在空 fieldKey")
		}
		spec, ok := catalog[fieldKey]
		if !ok {
			return fmt.Errorf("拓扑字段覆盖包含无效字段: %s", fieldKey)
		}
		if _, exists := seen[fieldKey]; exists {
			return fmt.Errorf("拓扑字段覆盖存在重复字段: %s", fieldKey)
		}
		seen[fieldKey] = struct{}{}
		if item.TimeoutSec < 0 {
			return fmt.Errorf("拓扑字段 %s 的超时时间不能为负数", fieldKey)
		}
		if item.Enabled != nil && *item.Enabled && strings.TrimSpace(item.Command) == "" {
			return fmt.Errorf("拓扑字段 %s 已启用但命令为空", fieldKey)
		}
		if spec.Required && item.Enabled != nil && !*item.Enabled {
			return fmt.Errorf("关键拓扑字段 %s 不允许被禁用", fieldKey)
		}
	}

	enabledByDefault := make(map[string]bool, len(catalog))
	for key, spec := range catalog {
		enabledByDefault[key] = spec.DefaultEnabled
	}
	for _, item := range overrides {
		fieldKey := strings.TrimSpace(item.FieldKey)
		if fieldKey == "" {
			continue
		}
		if item.Enabled != nil {
			enabledByDefault[fieldKey] = *item.Enabled
		}
	}

	enabledCount := 0
	for _, enabled := range enabledByDefault {
		if enabled {
			enabledCount++
		}
	}
	if enabledCount == 0 {
		return fmt.Errorf("至少需要保留一个启用的拓扑采集字段")
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
	if group.Mode == "" {
		group.Mode = "group"
	}
	if group.Items == nil {
		group.Items = make([]models.TaskItem, 0)
	}
	if group.Tags == nil {
		group.Tags = make([]string, 0)
	}
}

func topologyFieldCatalogForValidation() []models.TopologyFieldSpec {
	return []models.TopologyFieldSpec{
		{FieldKey: "version", Required: true, DefaultEnabled: true},
		{FieldKey: "sysname", Required: true, DefaultEnabled: true},
		{FieldKey: "esn", Required: false, DefaultEnabled: true},
		{FieldKey: "device_info", Required: true, DefaultEnabled: true},
		{FieldKey: "interface_brief", Required: true, DefaultEnabled: true},
		{FieldKey: "interface_detail", Required: false, DefaultEnabled: true},
		{FieldKey: "lldp_neighbor", Required: true, DefaultEnabled: true},
		{FieldKey: "mac_address", Required: true, DefaultEnabled: true},
		{FieldKey: "arp_all", Required: true, DefaultEnabled: true},
		{FieldKey: "eth_trunk", Required: false, DefaultEnabled: true},
	}
}
