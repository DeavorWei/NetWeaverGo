package ui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/models"
	"github.com/NetWeaverGo/core/internal/repository"
	"github.com/NetWeaverGo/core/internal/taskexec"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// TaskGroupService 任务模板服务
// 仅负责任务模板的 CRUD、详情聚合与按 taskGroupID 发起执行。
type TaskGroupService struct {
	wailsApp      *application.App
	repo          repository.DeviceRepository
	taskexec      *taskexec.TaskExecutionService
	launchService *taskexec.TaskLaunchService
}

// NewTaskGroupService 创建任务组服务实例
func NewTaskGroupService() *TaskGroupService {
	return &TaskGroupService{
		repo: repository.NewDeviceRepository(),
	}
}

// NewTaskGroupServiceWithDeps 使用依赖创建任务组服务实例
func NewTaskGroupServiceWithDeps(repo repository.DeviceRepository) *TaskGroupService {
	return &TaskGroupService{
		repo: repo,
	}
}

// NewTaskGroupServiceWithRepo 使用指定 Repository 创建任务组服务实例（用于测试）
func NewTaskGroupServiceWithRepo(repo repository.DeviceRepository) *TaskGroupService {
	return &TaskGroupService{
		repo: repo,
	}
}

// SetTaskExecutionService 设置共享的任务执行服务
func (s *TaskGroupService) SetTaskExecutionService(service *taskexec.TaskExecutionService) {
	s.taskexec = service
	if service != nil {
		s.launchService = taskexec.NewTaskLaunchService(service)
	}
}

// ServiceStartup Wails 服务启动生命周期钩子
func (s *TaskGroupService) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	s.wailsApp = application.Get()
	return nil
}

// ListTaskGroups 获取所有任务模板列表
func (s *TaskGroupService) ListTaskGroups() ([]TaskGroupListView, error) {
	groups, err := config.ListTaskGroups()
	if err != nil {
		return nil, err
	}

	runsByTaskGroup := s.latestRunsByTaskGroup()
	activeRunCount := s.activeRunCounts()
	result := make([]TaskGroupListView, 0, len(groups))
	for _, group := range groups {
		latestRun, hasRun := runsByTaskGroup[group.ID]
		status := "pending"
		latestRunID := ""
		latestRunStatus := ""
		latestRunStartedAt := ""
		latestRunFinishedAt := ""
		if hasRun {
			status = latestRun.Status
			latestRunID = latestRun.RunID
			latestRunStatus = latestRun.Status
			latestRunStartedAt = formatRunTime(latestRun.StartedAt)
			latestRunFinishedAt = formatRunTime(latestRun.FinishedAt)
		}
		activeCount := activeRunCount[group.ID]
		result = append(result, TaskGroupListView{
			ID:                  group.ID,
			Name:                group.Name,
			Description:         group.Description,
			DeviceGroup:         group.DeviceGroup,
			CommandGroup:        group.CommandGroup,
			MaxWorkers:          group.MaxWorkers,
			Timeout:             group.Timeout,
			TaskType:            group.TaskType,
			TopologyVendor:      group.TopologyVendor,
			AutoBuildTopology:   group.AutoBuildTopology,
			Mode:                group.Mode,
			Items:               append([]models.TaskItem(nil), group.Items...),
			Status:              status,
			LatestRunID:         latestRunID,
			LatestRunStatus:     latestRunStatus,
			LatestRunStartedAt:  latestRunStartedAt,
			LatestRunFinishedAt: latestRunFinishedAt,
			ActiveRunCount:      activeCount,
			CanEdit:             activeCount == 0,
			Tags:                append([]string(nil), group.Tags...),
			EnableRawLog:        group.EnableRawLog,
			CreatedAt:           group.CreatedAt.Format(time.RFC3339),
			UpdatedAt:           group.UpdatedAt.Format(time.RFC3339),
		})
	}
	return result, nil
}

// GetTaskGroup 根据 ID 获取单个任务模板
func (s *TaskGroupService) GetTaskGroup(id uint) (*models.TaskGroup, error) {
	group, err := config.GetTaskGroup(id)
	if err != nil {
		return nil, err
	}
	group.Status = ""
	return group, nil
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
	group.Status = ""
	return config.CreateTaskGroup(group)
}

// UpdateTaskGroup 更新任务组
func (s *TaskGroupService) UpdateTaskGroup(id uint, group models.TaskGroup) (*models.TaskGroup, error) {
	existing, err := config.GetTaskGroup(id)
	if err != nil {
		return nil, err
	}

	if !s.canEditTaskGroup(existing.ID) {
		return nil, fmt.Errorf("任务存在活跃运行，当前不可编辑")
	}

	group.ID = existing.ID
	group.CreatedAt = existing.CreatedAt
	group.Status = ""
	group.Mode = existing.Mode
	group.TaskType = existing.TaskType
	group.TopologyVendor = existing.TopologyVendor
	group.AutoBuildTopology = existing.AutoBuildTopology

	return config.UpdateTaskGroup(id, group)
}

// DeleteTaskGroup 删除任务组
func (s *TaskGroupService) DeleteTaskGroup(id uint) error {
	if !s.canEditTaskGroup(id) {
		return fmt.Errorf("任务存在活跃运行，当前不可删除")
	}
	return config.DeleteTaskGroup(id)
}

// StartTaskGroup 按 taskGroupID 启动任务，返回 runID
func (s *TaskGroupService) StartTaskGroup(id uint) (string, error) {
	if s.launchService == nil {
		return "", fmt.Errorf("task launch service not initialized")
	}
	return s.launchService.StartTaskGroup(context.Background(), id)
}

func (s *TaskGroupService) canEditTaskGroup(taskGroupID uint) bool {
	if s.taskexec == nil || taskGroupID == 0 {
		return true
	}

	runs, err := s.taskexec.ListRuns(200)
	if err != nil {
		return true
	}
	for _, run := range runs {
		status := strings.TrimSpace(run.Status)
		if run.TaskGroupID == taskGroupID && (status == string(taskexec.RunStatusPending) || status == string(taskexec.RunStatusRunning)) {
			return false
		}
	}
	return true
}

func (s *TaskGroupService) latestRunsByTaskGroup() map[uint]*taskexec.RunSummary {
	result := make(map[uint]*taskexec.RunSummary)
	if s.taskexec == nil {
		return result
	}

	runs, err := s.taskexec.ListRuns(500)
	if err != nil {
		return result
	}

	for _, run := range runs {
		if run == nil || run.TaskGroupID == 0 {
			continue
		}
		if _, exists := result[run.TaskGroupID]; !exists {
			result[run.TaskGroupID] = run
		}
	}
	return result
}

func (s *TaskGroupService) activeRunCounts() map[uint]int {
	result := make(map[uint]int)
	if s.taskexec == nil {
		return result
	}

	running := s.taskexec.ListRunning()
	for _, snapshot := range running {
		if snapshot == nil {
			continue
		}
		for _, run := range s.latestRunsByTaskGroup() {
			if run != nil && run.RunID == snapshot.RunID && run.TaskGroupID != 0 {
				result[run.TaskGroupID]++
			}
		}
	}
	return result
}

func formatRunTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339)
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
					Tags:  append([]string(nil), asset.Tags...),
				})
				continue
			}

			missingDeviceSet[deviceID] = struct{}{}
			devices = append(devices, TaskDeviceOverview{
				ID:      deviceID,
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
					Commands:    append([]string(nil), commandGroup.Commands...),
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
	canEdit := s.canEditTaskGroup(taskGroup.ID)
	editDisabledReason := ""
	if !canEdit {
		editDisabledReason = "任务存在活跃运行，当前不可编辑"
	}

	latestRuns := s.latestRunsByTaskGroup()
	latestRun := latestRuns[taskGroup.ID]
	activeRunCount := s.activeRunCounts()[taskGroup.ID]
	latestRunID := ""
	latestRunStatus := "pending"
	if latestRun != nil {
		latestRunID = latestRun.RunID
		latestRunStatus = latestRun.Status
	}

	return &TaskGroupDetailViewModel{
		Task:               *taskGroup,
		ItemCount:          len(taskGroup.Items),
		CanEdit:            canEdit,
		EditDisabledReason: editDisabledReason,
		LatestRunID:        latestRunID,
		LatestRunStatus:    latestRunStatus,
		ActiveRunCount:     activeRunCount,
		Items:              items,
		MissingDevices:     missingDevices,
		MissingCommandIDs:  missingCommandIDs,
	}, nil
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
