package ui

import (
	"context"
	"strings"

	"github.com/NetWeaverGo/core/internal/models"
	"github.com/NetWeaverGo/core/internal/repository"
	"github.com/NetWeaverGo/core/internal/taskexec"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// TaskExecutionUIService 统一任务执行UI服务
// 作为Wails服务暴露给前端，提供统一的任务执行接口
type TaskExecutionUIService struct {
	service  *taskexec.TaskExecutionService
	bridge   *TaskExecutionEventBridge
	wailsApp *application.App
}

// NewTaskExecutionUIService 创建统一任务执行UI服务
// 接收共享的 TaskExecutionService 实例
func NewTaskExecutionUIService(service *taskexec.TaskExecutionService) *TaskExecutionUIService {
	// 创建事件桥接器
	bridge := NewTaskExecutionEventBridge(service.GetEventBus())

	return &TaskExecutionUIService{
		service: service,
		bridge:  bridge,
	}
}

// ServiceStartup Wails服务启动生命周期钩子
func (s *TaskExecutionUIService) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	s.wailsApp = application.Get()

	// 设置事件桥接器的Wails应用实例并启动
	if s.bridge != nil {
		s.bridge.SetWailsApp(s.wailsApp)
		s.bridge.Start()
	}

	return nil
}

// ServiceShutdown Wails服务停止生命周期钩子
func (s *TaskExecutionUIService) ServiceShutdown() error {
	if s.bridge != nil {
		s.bridge.Stop()
	}
	return nil
}

// StartNormalTask 启动普通任务
// 从任务组配置创建并启动普通任务
func (s *TaskExecutionUIService) StartNormalTask(taskGroup *models.TaskGroup, mode string) (string, error) {
	deviceRepo := repository.NewDeviceRepository()

	// 转换为统一运行时配置
	taskConfig := &taskexec.NormalTaskConfig{
		Mode:         mode,
		Concurrency:  taskGroup.MaxWorkers,
		TimeoutSec:   taskGroup.Timeout,
		EnableRawLog: taskGroup.EnableRawLog,
	}

	if mode == "group" {
		// Mode A: 一组命令发送给所有设备
		for _, item := range taskGroup.Items {
			taskConfig.DeviceIDs = append(taskConfig.DeviceIDs, item.DeviceIDs...)
			taskConfig.DeviceIPs = append(taskConfig.DeviceIPs, loadDeviceIPs(deviceRepo, item.DeviceIDs)...)
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
				DeviceIPs:      loadDeviceIPs(deviceRepo, item.DeviceIDs),
				CommandGroupID: item.CommandGroupID,
				Commands:       item.Commands,
			})
		}
	}

	// 创建任务定义
	def, err := s.service.CreateNormalTask(taskGroup.Name, taskConfig)
	if err != nil {
		return "", err
	}

	// 启动任务
	runID, err := s.service.StartTask(context.Background(), def)
	if err != nil {
		return "", err
	}

	// 订阅该run的事件
	if s.bridge != nil {
		s.bridge.SubscribeRun(runID)
	}

	return runID, nil
}

// StartTopologyTask 启动拓扑任务
// 从任务组配置创建并启动拓扑任务
func (s *TaskExecutionUIService) StartTopologyTask(taskGroup *models.TaskGroup) (string, error) {
	deviceRepo := repository.NewDeviceRepository()

	// 收集设备信息
	var deviceIDs []uint
	for _, item := range taskGroup.Items {
		deviceIDs = append(deviceIDs, item.DeviceIDs...)
	}

	topologyConfig := &taskexec.TopologyTaskConfig{
		DeviceIDs:         deviceIDs,
		DeviceIPs:         loadDeviceIPs(deviceRepo, deviceIDs),
		Vendor:            taskGroup.TopologyVendor,
		MaxWorkers:        taskGroup.MaxWorkers,
		TimeoutSec:        taskGroup.Timeout,
		AutoBuildTopology: taskGroup.AutoBuildTopology,
		EnableRawLog:      taskGroup.EnableRawLog,
	}

	// 创建任务定义
	def, err := s.service.CreateTopologyTask(taskGroup.Name, topologyConfig)
	if err != nil {
		return "", err
	}

	// 启动任务
	runID, err := s.service.StartTask(context.Background(), def)
	if err != nil {
		return "", err
	}

	// 订阅该run的事件
	if s.bridge != nil {
		s.bridge.SubscribeRun(runID)
	}

	return runID, nil
}

// GetTaskSnapshot 获取任务快照
func (s *TaskExecutionUIService) GetTaskSnapshot(runID string) (*taskexec.ExecutionSnapshot, error) {
	return s.service.GetSnapshot(runID)
}

// ListRunningTasks 列出正在运行的任务
func (s *TaskExecutionUIService) ListRunningTasks() []*taskexec.ExecutionSnapshot {
	return s.service.ListRunning()
}

// ListTaskRuns 获取历史运行记录
func (s *TaskExecutionUIService) ListTaskRuns(limit int) ([]*taskexec.RunSummary, error) {
	return s.service.ListRuns(limit)
}

// CancelTask 取消任务
func (s *TaskExecutionUIService) CancelTask(runID string) error {
	return s.service.CancelRun(runID)
}

// GetRunStatus 获取运行状态
func (s *TaskExecutionUIService) GetRunStatus(runID string) (*taskexec.TaskRun, error) {
	return s.service.GetRunStatus(runID)
}

// SubscribeRunEvents 订阅指定run的事件
func (s *TaskExecutionUIService) SubscribeRunEvents(runID string) error {
	if s.bridge != nil {
		s.bridge.SubscribeRun(runID)
	}
	return nil
}

// UnsubscribeRunEvents 取消订阅指定run的事件
func (s *TaskExecutionUIService) UnsubscribeRunEvents(runID string) error {
	if s.bridge != nil {
		s.bridge.UnsubscribeRun(runID)
	}
	return nil
}

// loadDeviceIPs 加载设备IP列表
func loadDeviceIPs(deviceRepo repository.DeviceRepository, deviceIDs []uint) []string {
	ips := make([]string, 0, len(deviceIDs))
	for _, id := range deviceIDs {
		device, err := deviceRepo.FindByID(id)
		if err != nil || device == nil {
			continue
		}
		ip := strings.TrimSpace(device.IP)
		if ip != "" {
			ips = append(ips, ip)
		}
	}
	return ips
}
