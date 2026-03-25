package ui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/discovery"
	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
	"github.com/NetWeaverGo/core/internal/report"
	"github.com/NetWeaverGo/core/internal/repository"
	"github.com/NetWeaverGo/core/internal/taskexec"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// TaskGroupService 任务组管理服务 - 负责任务组的增删改查和执行
type TaskGroupService struct {
	wailsApp         *application.App
	repo             repository.DeviceRepository
	discoveryService *DiscoveryService
	topologyService  *TopologyService
	v2               *TaskGroupServiceV2
}

// NewTaskGroupService 创建任务组服务实例
func NewTaskGroupService() *TaskGroupService {
	return &TaskGroupService{
		repo: repository.NewDeviceRepository(),
		v2:   NewTaskGroupServiceV2(),
	}
}

// NewTaskGroupServiceWithDeps 使用依赖创建任务组服务实例
func NewTaskGroupServiceWithDeps(
	repo repository.DeviceRepository,
	discoveryService *DiscoveryService,
	topologyService *TopologyService,
) *TaskGroupService {
	return &TaskGroupService{
		repo:             repo,
		discoveryService: discoveryService,
		topologyService:  topologyService,
		v2:               NewTaskGroupServiceV2(),
	}
}

// NewTaskGroupServiceWithRepo 使用指定 Repository 创建任务组服务实例（用于测试）
func NewTaskGroupServiceWithRepo(repo repository.DeviceRepository) *TaskGroupService {
	return &TaskGroupService{
		repo: repo,
		v2:   NewTaskGroupServiceV2(),
	}
}

// SetTaskExecutionService 设置共享的任务执行服务（阶段1：统一运行时服务化）
func (s *TaskGroupService) SetTaskExecutionService(service *taskexec.TaskExecutionService) {
	if s.v2 == nil {
		s.v2 = NewTaskGroupServiceV2()
	}
	s.v2.SetTaskExecutionService(service)
}

// SetTopologyDeps 设置拓扑采集相关依赖
func (s *TaskGroupService) SetTopologyDeps(discoveryService *DiscoveryService, topologyService *TopologyService) {
	s.discoveryService = discoveryService
	s.topologyService = topologyService
}

// ServiceStartup Wails 服务启动生命周期钩子
func (s *TaskGroupService) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	s.wailsApp = application.Get()
	// 统一运行时通过 TaskExecutionEventBridge 自动处理 Wails 事件，无需此处设置
	return nil
}

// ListTaskGroups 获取所有任务组列表
func (s *TaskGroupService) ListTaskGroups() ([]models.TaskGroup, error) {
	return config.ListTaskGroups()
}

// GetTaskGroup 根据 ID 获取单个任务组
func (s *TaskGroupService) GetTaskGroup(id uint) (*models.TaskGroup, error) {
	return config.GetTaskGroup(id)
}

// GetTaskGroupDetail 根据 ID 获取任务详情聚合信息
func (s *TaskGroupService) GetTaskGroupDetail(id uint) (*TaskGroupDetailViewModel, error) {
	taskGroup, err := config.GetTaskGroup(id)
	if err != nil {
		return nil, err
	}

	return s.buildTaskGroupDetail(taskGroup)
}

// CreateTaskGroup 创建新任务组
func (s *TaskGroupService) CreateTaskGroup(group models.TaskGroup) (*models.TaskGroup, error) {
	return config.CreateTaskGroup(group)
}

// UpdateTaskGroup 更新任务组
func (s *TaskGroupService) UpdateTaskGroup(id uint, group models.TaskGroup) (*models.TaskGroup, error) {
	existing, err := config.GetTaskGroup(id)
	if err != nil {
		return nil, err
	}

	if !canEditTaskGroup(existing.Status) {
		return nil, fmt.Errorf("任务执行中不可编辑，当前状态为 %s", existing.Status)
	}

	// 任务编辑只允许修改基础信息和当前模式下的任务项。
	group.ID = existing.ID
	group.CreatedAt = existing.CreatedAt
	group.Status = existing.Status
	group.Mode = existing.Mode
	group.TaskType = existing.TaskType
	group.TopologyVendor = existing.TopologyVendor
	group.AutoBuildTopology = existing.AutoBuildTopology

	return config.UpdateTaskGroup(id, group)
}

// DeleteTaskGroup 删除任务组
func (s *TaskGroupService) DeleteTaskGroup(id uint) error {
	return config.DeleteTaskGroup(id)
}

// ResolveSuspend 被前端调用（当用户在弹窗中选择动作后）
// 注意：暂停功能已随旧执行引擎删除，此方法保留为兼容接口
func (s *TaskGroupService) ResolveSuspend(_sessionIDOrIP string, _action string) {
	// 统一运行时暂不支持暂停功能
}

// StartTaskGroup 启动任务组执行（并行执行模式）
func (s *TaskGroupService) StartTaskGroup(id uint) error {
	if s.v2 == nil {
		s.v2 = NewTaskGroupServiceV2()
	}
	_, err := s.v2.StartTaskGroup(id)
	return err
}

// executeModeA 已删除 - 使用统一运行时替代

// executeModeB 已删除 - 使用统一运行时替代

// executeTopologyTask 执行拓扑采集任务
func (s *TaskGroupService) executeTopologyTask(taskGroup *models.TaskGroup) (string, error) {
	if s.discoveryService == nil {
		logger.Error("TaskGroup", fmt.Sprintf("%d", taskGroup.ID), "拓扑采集依赖未初始化: discoveryService")
		return "", fmt.Errorf("拓扑采集依赖未初始化: discoveryService")
	}

	deviceIDs := collectUniqueDeviceIDs(taskGroup.Items)
	if len(deviceIDs) == 0 {
		logger.Warn("TaskGroup", fmt.Sprintf("%d", taskGroup.ID), "拓扑采集任务设备为空")
		return "", fmt.Errorf("拓扑采集任务中没有可执行设备")
	}
	logger.Info("TaskGroup", fmt.Sprintf("%d", taskGroup.ID), "准备启动拓扑采集: devices=%v vendor=%s autoBuild=%v", deviceIDs, strings.TrimSpace(taskGroup.TopologyVendor), taskGroup.AutoBuildTopology)

	req := discovery.StartDiscoveryRequest{
		DeviceIDs:  make([]string, 0, len(deviceIDs)),
		Vendor:     strings.TrimSpace(taskGroup.TopologyVendor),
		MaxWorkers: taskGroup.MaxWorkers,
		TimeoutSec: taskGroup.Timeout,
	}
	for _, id := range deviceIDs {
		req.DeviceIDs = append(req.DeviceIDs, fmt.Sprintf("%d", id))
	}

	resp, err := s.discoveryService.StartDiscovery(req)
	if err != nil {
		logger.Error("TaskGroup", fmt.Sprintf("%d", taskGroup.ID), "启动拓扑采集失败: %v", err)
		return "", err
	}

	if err := config.BindDiscoveryTaskToTaskGroup(resp.TaskID, taskGroup.ID); err != nil {
		logger.Warn("TaskGroup", fmt.Sprintf("%d", taskGroup.ID), "绑定发现任务与任务组失败: task=%s err=%v", resp.TaskID, err)
	}

	taskStatus, err := s.waitDiscoveryTaskCompleted(resp.TaskID)
	if err != nil {
		logger.Error("TaskGroup", fmt.Sprintf("%d", taskGroup.ID), "等待拓扑采集任务失败: task=%s err=%v", resp.TaskID, err)
		return "", err
	}

	if taskGroup.AutoBuildTopology && s.topologyService != nil {
		if _, err := s.topologyService.BuildTopology(context.Background(), resp.TaskID); err != nil {
			return "failed", fmt.Errorf("拓扑构建失败: %v", err)
		}
	}

	switch strings.ToLower(strings.TrimSpace(taskStatus)) {
	case "completed":
		return "completed", nil
	case "partial":
		logger.Warn("TaskGroup", fmt.Sprintf("%d", taskGroup.ID), "拓扑采集任务部分成功: discoveryTask=%s", resp.TaskID)
		return "partial", nil
	case "failed", "cancelled":
		return "failed", fmt.Errorf("拓扑采集任务结束状态为 %s", taskStatus)
	default:
		return "failed", fmt.Errorf("拓扑采集任务结束状态未知: %s", taskStatus)
	}
}

func (s *TaskGroupService) waitDiscoveryTaskCompleted(taskID string) (string, error) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	timeout := time.After(4 * time.Hour)
	for {
		select {
		case <-timeout:
			return "", fmt.Errorf("等待拓扑采集任务超时: %s", taskID)
		case <-ticker.C:
			task, err := s.discoveryService.GetTaskStatus(taskID)
			if err != nil {
				return "", err
			}
			if task == nil {
				continue
			}
			if isTerminalDiscoveryTaskStatus(task.Status) {
				return strings.ToLower(task.Status), nil
			}
		}
	}
}

func isTerminalDiscoveryTaskStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "completed", "partial", "failed", "cancelled":
		return true
	default:
		return false
	}
}

func collectUniqueDeviceIDs(items []models.TaskItem) []uint {
	if len(items) == 0 {
		return nil
	}

	set := make(map[uint]struct{})
	for _, item := range items {
		for _, deviceID := range item.DeviceIDs {
			if deviceID == 0 {
				continue
			}
			set[deviceID] = struct{}{}
		}
	}

	ids := make([]uint, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		return ids[i] < ids[j]
	})
	return ids
}

func normalizeTaskType(taskType string) string {
	value := strings.ToLower(strings.TrimSpace(taskType))
	if value == "" {
		return "normal"
	}
	return value
}

func deriveTaskGroupStatus(tracker *report.ProgressTracker) string {
	total, finished, successCount, errorCount := tracker.GetStats()

	if total == 0 {
		return "failed"
	}
	if errorCount > 0 {
		return "failed"
	}
	if finished < total {
		return "failed"
	}
	if successCount < total {
		return "failed"
	}

	return "completed"
}

func canEditTaskGroup(status string) bool {
	return status != "running"
}

func (s *TaskGroupService) buildTaskGroupDetail(taskGroup *models.TaskGroup) (*TaskGroupDetailViewModel, error) {
	if taskGroup == nil {
		return nil, fmt.Errorf("任务组不能为空")
	}

	assets, err := s.repo.FindAll()
	if err != nil {
		return nil, err
	}

	assetMap := make(map[uint]models.DeviceAsset, len(assets))
	for _, asset := range assets {
		assetMap[asset.ID] = asset
	}

	uniqueCommandIDs := make(map[uint]struct{})
	for _, item := range taskGroup.Items {
		if item.CommandGroupID != "" {
			uniqueCommandIDs[parseID(item.CommandGroupID)] = struct{}{}
		}
	}

	commandMap := make(map[uint]*models.CommandGroup, len(uniqueCommandIDs))
	missingCommandSet := make(map[uint]struct{})
	for id := range uniqueCommandIDs {
		group, getErr := config.GetCommandGroup(id)
		if getErr != nil {
			missingCommandSet[id] = struct{}{}
			continue
		}
		commandMap[id] = group
	}

	items := make([]TaskGroupItemDetailViewModel, 0, len(taskGroup.Items))
	missingDeviceSet := make(map[uint]struct{})

	for index, item := range taskGroup.Items {
		devices := make([]TaskDeviceOverview, 0, len(item.DeviceIDs))
		for _, deviceID := range item.DeviceIDs {
			if asset, ok := assetMap[deviceID]; ok {
				devices = append(devices, TaskDeviceOverview{
					ID:    asset.ID,
					IP:    asset.IP,
					Group: asset.Group,
				})
				continue
			}

			missingDeviceSet[deviceID] = struct{}{}
			devices = append(devices, TaskDeviceOverview{
				ID:      deviceID,
				IP:      "",
				Missing: true,
			})
		}

		itemDetail := TaskGroupItemDetailViewModel{
			Index:       index,
			Mode:        taskGroup.Mode,
			DeviceCount: len(item.DeviceIDs),
			Devices:     devices,
			Commands:    append([]string(nil), item.Commands...),
		}

		if item.CommandGroupID != "" {
			cmdID := parseID(item.CommandGroupID)
			if commandGroup, ok := commandMap[cmdID]; ok {
				itemDetail.CommandInfo = &TaskCommandOverview{
					ID:          commandGroup.ID,
					Name:        commandGroup.Name,
					Description: commandGroup.Description,
					Commands:    commandGroup.Commands,
				}
			} else {
				itemDetail.CommandInfo = &TaskCommandOverview{
					ID:      cmdID,
					Name:    "命令组不存在",
					Missing: true,
				}
			}
		}

		items = append(items, itemDetail)
	}

	missingDevices := sortedUintKeys(missingDeviceSet)
	missingCommandIDs := sortedUintKeys(missingCommandSet)
	editDisabledReason := ""
	if !canEditTaskGroup(taskGroup.Status) {
		editDisabledReason = fmt.Sprintf("任务执行中不可编辑，当前状态为 %s", taskGroup.Status)
	}

	return &TaskGroupDetailViewModel{
		Task:               *taskGroup,
		ItemCount:          len(taskGroup.Items),
		CanEdit:            canEditTaskGroup(taskGroup.Status),
		EditDisabledReason: editDisabledReason,
		Items:              items,
		MissingDevices:     missingDevices,
		MissingCommandIDs:  missingCommandIDs,
	}, nil
}

func parseID(s string) uint {
	var id uint
	fmt.Sscanf(s, "%d", &id)
	return id
}

func sortedUintKeys(values map[uint]struct{}) []uint {
	result := make([]uint, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i] < result[j]
	})
	return result
}
