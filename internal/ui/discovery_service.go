package ui

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/discovery"
	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
	"github.com/NetWeaverGo/core/internal/parser"
	"github.com/NetWeaverGo/core/internal/topology"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// DiscoveryService 发现任务服务 - 负责拓扑发现任务的管理
type DiscoveryService struct {
	wailsApp *application.App
	runner   *discovery.Runner
	parser   *parser.Service
	builder  *topology.Builder
	mu       sync.RWMutex
	once     sync.Once
}

// NewDiscoveryService 创建发现服务实例
func NewDiscoveryService() *DiscoveryService {
	runner := discovery.NewRunner(config.DB)
	runner.SetPathProvider(config.GetPathManager())
	builder := topology.NewBuilder(config.DB)
	if runtimeManager := config.GetRuntimeManagerIfInitialized(); runtimeManager != nil {
		runner.SetRuntimeProvider(runtimeManager)
		runner.SetMaxWorkers(runtimeManager.GetDiscoveryWorkerCount())
		builder.SetRuntimeProvider(runtimeManager)
	}

	return &DiscoveryService{
		runner:  runner,
		parser:  parser.NewService(config.DB),
		builder: builder,
	}
}

// ServiceStartup Wails 服务启动生命周期钩子
func (s *DiscoveryService) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	s.wailsApp = application.Get()
	s.once.Do(func() {
		go s.forwardEvents()
	})
	return nil
}

// StartDiscovery 启动发现任务
func (s *DiscoveryService) StartDiscovery(req discovery.StartDiscoveryRequest) (discovery.TaskStartResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	taskID, err := s.runner.Start(context.Background(), req)
	if err != nil {
		return discovery.TaskStartResponse{}, err
	}

	return discovery.TaskStartResponse{TaskID: taskID}, nil
}

// forwardEvents 监听发现事件并统一转发到前端。
func (s *DiscoveryService) forwardEvents() {
	for event := range s.runner.FrontendBus {
		if s.wailsApp != nil {
			s.wailsApp.Event.Emit("discovery:event", event)
		}
		if event.Type == "completed" && event.TaskID != "" {
			go s.persistDiscoveryExecutionSummary(event.TaskID)
		}
	}
}

// CancelDiscovery 取消发现任务
func (s *DiscoveryService) CancelDiscovery(taskID string) error {
	return s.runner.Cancel(taskID)
}

// RetryFailedDevices 重试失败的设备
func (s *DiscoveryService) RetryFailedDevices(taskID string) error {
	return s.runner.RetryFailed(context.Background(), taskID)
}

// GetTaskStatus 获取任务状态
func (s *DiscoveryService) GetTaskStatus(taskID string) (*discovery.DiscoveryTaskView, error) {
	return s.runner.GetTaskStatus(taskID)
}

// ListDiscoveryTasks 列出所有发现任务
func (s *DiscoveryService) ListDiscoveryTasks(limit int) ([]discovery.DiscoveryTaskView, error) {
	return s.runner.ListTasks(limit)
}

// GetTaskDevices 获取任务下的设备列表
func (s *DiscoveryService) GetTaskDevices(taskID string) ([]discovery.DiscoveryDeviceView, error) {
	return s.runner.GetTaskDevices(taskID)
}

// GetRawOutput 获取原始命令输出
func (s *DiscoveryService) GetRawOutput(taskID, deviceIP, commandKey string) (string, error) {
	return s.runner.GetRawOutput(taskID, deviceIP, commandKey)
}

// IsDiscoveryRunning 检查是否有发现任务在运行
func (s *DiscoveryService) IsDiscoveryRunning() bool {
	return s.runner.IsRunning()
}

// GetCurrentDiscoveryTask 获取当前运行的任务ID
func (s *DiscoveryService) GetCurrentDiscoveryTask() string {
	return s.runner.GetCurrentTask()
}

// GetVendorProfiles 获取所有厂商命令配置
func (s *DiscoveryService) GetVendorProfiles() []*config.DeviceProfile {
	return config.GetAllDeviceProfiles()
}

// GetSupportedVendors 获取支持的厂商列表
func (s *DiscoveryService) GetSupportedVendors() []string {
	return discovery.SupportedVendors
}

// ParseDiscoveryTask 解析发现任务的原始输出
func (s *DiscoveryService) ParseDiscoveryTask(taskID string) error {
	return s.parser.ParseTask(taskID)
}

// BuildTopology 构建拓扑图
func (s *DiscoveryService) BuildTopology(taskID string) (*models.TopologyBuildResult, error) {
	// 先解析任务
	var parseErrors []string
	if err := s.ParseDiscoveryTask(taskID); err != nil {
		// 解析失败收集错误，继续构建
		parseErrors = append(parseErrors, fmt.Sprintf("解析任务失败: %v", err))
	}

	// 构建拓扑
	result, err := s.builder.Build(taskID)
	if err != nil {
		return nil, err
	}

	// 将解析错误添加到结果中
	if len(parseErrors) > 0 {
		result.Errors = append(parseErrors, result.Errors...)
	}

	return result, nil
}

// GetTopologyGraph 获取拓扑图视图
func (s *DiscoveryService) GetTopologyGraph(taskID string) (*models.TopologyGraphView, error) {
	return s.builder.BuildGraphView(taskID)
}

// GetEdgeDetail 获取边详情
func (s *DiscoveryService) GetEdgeDetail(taskID string, edgeID string) (*models.TopologyEdgeDetailView, error) {
	return s.builder.GetEdgeDetail(taskID, edgeID)
}

// GetDeviceTopologyDetail 获取设备的拓扑详情
func (s *DiscoveryService) GetDeviceTopologyDetail(taskID string, deviceIP string) (*parser.ParsedResult, error) {
	return s.parser.GetParsedDeviceDetail(taskID, deviceIP)
}

func (s *DiscoveryService) persistDiscoveryExecutionSummary(taskID string) {
	task, err := s.runner.GetTaskStatus(taskID)
	if err != nil || task == nil {
		logger.Warn("DiscoveryService", "-", "写入发现执行摘要失败，任务不存在: %s err=%v", taskID, err)
		return
	}

	devices, err := s.runner.GetTaskDevices(taskID)
	if err != nil {
		logger.Warn("DiscoveryService", "-", "写入发现执行摘要失败，获取设备列表失败: task=%s err=%v", taskID, err)
		return
	}

	warnings := 0
	deviceRecords := make([]models.ExecutionDeviceRecord, 0, len(devices))
	for _, d := range devices {
		if d.Status == "partial" {
			warnings++
		}
		deviceRecords = append(deviceRecords, models.ExecutionDeviceRecord{
			IP:       d.DeviceIP,
			Status:   d.Status,
			ErrorMsg: d.ErrorMessage,
		})
	}

	startedAt := formatTime(task.StartedAt)
	finishedAt := formatTime(task.FinishedAt)
	durationMs := int64(0)
	if task.StartedAt != nil && task.FinishedAt != nil {
		durationMs = task.FinishedAt.Sub(*task.StartedAt).Milliseconds()
	}

	record := models.ExecutionRecord{
		RunnerSource:  "discovery_service",
		RunnerID:      task.ID,
		TaskGroupID:   "",
		TaskGroupName: "",
		TaskName:      task.Name,
		Mode:          "discovery",
		Status:        task.Status,
		TotalDevices:  task.TotalCount,
		FinishedCount: task.SuccessCount + task.FailedCount,
		SuccessCount:  task.SuccessCount,
		ErrorCount:    task.FailedCount,
		AbortedCount:  0,
		WarningCount:  warnings,
		StartedAt:     startedAt,
		FinishedAt:    finishedAt,
		DurationMs:    durationMs,
		Devices:       deviceRecords,
	}

	if _, err := config.CreateExecutionRecord(record); err != nil {
		logger.Warn("DiscoveryService", "-", "写入发现执行摘要失败: task=%s err=%v", taskID, err)
	}
}

func formatTime(ts *time.Time) string {
	if ts == nil {
		return ""
	}
	return ts.Format(time.RFC3339)
}
