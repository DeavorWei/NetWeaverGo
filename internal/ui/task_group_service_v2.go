package ui

import (
	"context"
	"fmt"
	"time"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
	"github.com/NetWeaverGo/core/internal/repository"
	"github.com/NetWeaverGo/core/internal/taskexec"
)

// TaskGroupServiceV2 使用新统一运行时的任务组服务
type TaskGroupServiceV2 struct {
	taskExecService *taskexec.TaskExecutionService
	deviceRepo      repository.DeviceRepository
}

// NewTaskGroupServiceV2 创建V2服务
func NewTaskGroupServiceV2() *TaskGroupServiceV2 {
	return &TaskGroupServiceV2{
		deviceRepo: repository.NewDeviceRepository(),
	}
}

// NewTaskGroupServiceV2WithRuntime 使用共享运行时创建V2服务
func NewTaskGroupServiceV2WithRuntime(service *taskexec.TaskExecutionService) *TaskGroupServiceV2 {
	return &TaskGroupServiceV2{
		taskExecService: service,
		deviceRepo:      repository.NewDeviceRepository(),
	}
}

// SetTaskExecutionService 设置共享的任务执行服务
func (s *TaskGroupServiceV2) SetTaskExecutionService(service *taskexec.TaskExecutionService) {
	s.taskExecService = service
}

// StartTaskGroup 启动任务组（使用新运行时）
func (s *TaskGroupServiceV2) StartTaskGroup(id uint) (string, error) {
	taskGroup, err := config.GetTaskGroup(id)
	if err != nil {
		logger.Error("TaskGroupV2", fmt.Sprintf("%d", id), "获取任务组失败: %v", err)
		return "", fmt.Errorf("获取任务组失败: %v", err)
	}

	logger.Info("TaskGroupV2", fmt.Sprintf("%d", id), "开始执行任务组: name=%s type=%s mode=%s",
		taskGroup.Name, normalizeTaskType(taskGroup.TaskType), taskGroup.Mode)

	// 更新任务组状态
	if err := config.UpdateTaskGroupStatus(id, "running"); err != nil {
		logger.Error("TaskGroupV2", fmt.Sprintf("%d", id), "更新任务组状态失败: %v", err)
		return "", err
	}

	// 根据任务类型选择执行方式
	taskType := normalizeTaskType(taskGroup.TaskType)
	var runID string

	switch taskType {
	case "topology":
		runID, err = s.startTopologyTask(taskGroup)
	default:
		// 普通任务：根据模式调用
		runID, err = s.startNormalTask(taskGroup, taskGroup.Mode)
	}

	if err != nil {
		logger.Error("TaskGroupV2", fmt.Sprintf("%d", id), "启动任务失败: %v", err)
		config.UpdateTaskGroupStatus(id, "failed")
		return "", err
	}

	logger.Info("TaskGroupV2", fmt.Sprintf("%d", id), "任务启动成功: runID=%s", runID)

	// 异步监控任务完成并更新状态
	go s.monitorTaskCompletion(runID, id)

	return runID, nil
}

// startNormalTask 启动普通任务
func (s *TaskGroupServiceV2) startNormalTask(taskGroup *models.TaskGroup, mode string) (string, error) {
	if s.taskExecService == nil {
		return "", fmt.Errorf("task execution service not initialized")
	}

	// 转换为统一运行时配置
	taskConfig := &taskexec.NormalTaskConfig{
		Mode:         mode,
		Concurrency:  taskGroup.MaxWorkers,
		TimeoutSec:   taskGroup.Timeout,
		EnableRawLog: taskGroup.EnableRawLog,
	}

	// 加载设备IP
	loadIPs := func(deviceIDs []uint) []string {
		ips := make([]string, 0, len(deviceIDs))
		for _, id := range deviceIDs {
			device, err := s.deviceRepo.FindByID(id)
			if err != nil || device == nil {
				continue
			}
			ip := device.IP
			if ip != "" {
				ips = append(ips, ip)
			}
		}
		return ips
	}

	if mode == "group" {
		// Mode A: 一组命令发送给所有设备
		for _, item := range taskGroup.Items {
			taskConfig.DeviceIDs = append(taskConfig.DeviceIDs, item.DeviceIDs...)
			taskConfig.DeviceIPs = append(taskConfig.DeviceIPs, loadIPs(item.DeviceIDs)...)
			if len(taskConfig.Commands) == 0 && len(item.Commands) > 0 {
				taskConfig.Commands = append(taskConfig.Commands, item.Commands...)
			}
			if taskConfig.CommandGroupID == "" && item.CommandGroupID != "" {
				taskConfig.CommandGroupID = item.CommandGroupID
			}
		}
	} else {
		// Mode B: 创建任务项
		for _, item := range taskGroup.Items {
			taskConfig.Items = append(taskConfig.Items, taskexec.NormalTaskItem{
				DeviceIDs:      item.DeviceIDs,
				DeviceIPs:      loadIPs(item.DeviceIDs),
				CommandGroupID: item.CommandGroupID,
				Commands:       item.Commands,
			})
		}
	}

	// 创建任务定义
	def, err := s.taskExecService.CreateNormalTask(taskGroup.Name, taskConfig)
	if err != nil {
		return "", err
	}

	// 启动任务
	ctx := context.Background()
	runID, err := s.taskExecService.StartTask(ctx, def)
	if err != nil {
		return "", err
	}

	return runID, nil
}

// startTopologyTask 启动拓扑任务
func (s *TaskGroupServiceV2) startTopologyTask(taskGroup *models.TaskGroup) (string, error) {
	if s.taskExecService == nil {
		return "", fmt.Errorf("task execution service not initialized")
	}

	// 收集设备信息
	var deviceIDs []uint
	var deviceIPs []string

	for _, item := range taskGroup.Items {
		deviceIDs = append(deviceIDs, item.DeviceIDs...)
		for _, id := range item.DeviceIDs {
			device, err := s.deviceRepo.FindByID(id)
			if err != nil || device == nil {
				continue
			}
			ip := device.IP
			if ip != "" {
				deviceIPs = append(deviceIPs, ip)
			}
		}
	}

	topologyConfig := &taskexec.TopologyTaskConfig{
		DeviceIDs:         deviceIDs,
		DeviceIPs:         deviceIPs,
		Vendor:            taskGroup.TopologyVendor,
		MaxWorkers:        taskGroup.MaxWorkers,
		TimeoutSec:        taskGroup.Timeout,
		AutoBuildTopology: taskGroup.AutoBuildTopology,
		EnableRawLog:      taskGroup.EnableRawLog,
	}

	// 创建任务定义
	def, err := s.taskExecService.CreateTopologyTask(taskGroup.Name, topologyConfig)
	if err != nil {
		return "", err
	}

	// 启动任务
	ctx := context.Background()
	runID, err := s.taskExecService.StartTask(ctx, def)
	if err != nil {
		return "", err
	}

	return runID, nil
}

// monitorTaskCompletion 监控任务完成状态
func (s *TaskGroupServiceV2) monitorTaskCompletion(runID string, taskGroupID uint) {
	if s.taskExecService == nil {
		logger.Error("TaskGroupV2", runID, "任务执行服务未初始化")
		config.UpdateTaskGroupStatus(taskGroupID, "failed")
		return
	}

	// 轮询检查任务状态
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		snapshot, err := s.taskExecService.GetSnapshot(runID)
		if err != nil {
			logger.Error("TaskGroupV2", runID, "获取任务快照失败: %v", err)
			config.UpdateTaskGroupStatus(taskGroupID, "failed")
			return
		}

		// 检查是否完成
		if snapshot.Status == string(taskexec.RunStatusCompleted) {
			config.UpdateTaskGroupStatus(taskGroupID, "completed")
			logger.Info("TaskGroupV2", runID, "任务完成")
			return
		} else if snapshot.Status == string(taskexec.RunStatusFailed) {
			config.UpdateTaskGroupStatus(taskGroupID, "failed")
			logger.Info("TaskGroupV2", runID, "任务失败")
			return
		} else if snapshot.Status == string(taskexec.RunStatusPartial) {
			config.UpdateTaskGroupStatus(taskGroupID, "completed")
			logger.Info("TaskGroupV2", runID, "任务部分完成")
			return
		} else if snapshot.Status == string(taskexec.RunStatusCancelled) {
			config.UpdateTaskGroupStatus(taskGroupID, "cancelled")
			logger.Info("TaskGroupV2", runID, "任务已取消")
			return
		}
	}
}

// GetTaskSnapshot 获取任务快照
func (s *TaskGroupServiceV2) GetTaskSnapshot(runID string) (*taskexec.ExecutionSnapshot, error) {
	if s.taskExecService == nil {
		return nil, fmt.Errorf("task execution service not initialized")
	}
	return s.taskExecService.GetSnapshot(runID)
}

// ListRunningTasks 列出正在运行的任务
func (s *TaskGroupServiceV2) ListRunningTasks() []*taskexec.ExecutionSnapshot {
	if s.taskExecService == nil {
		return nil
	}
	return s.taskExecService.ListRunning()
}

// CancelTask 取消任务
func (s *TaskGroupServiceV2) CancelTask(runID string) error {
	if s.taskExecService == nil {
		return fmt.Errorf("task execution service not initialized")
	}
	return s.taskExecService.CancelRun(runID)
}
