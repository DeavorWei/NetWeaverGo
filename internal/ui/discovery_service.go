package ui

import (
	"context"
	"sync"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/discovery"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// DiscoveryService 发现任务服务 - 负责拓扑发现任务的管理
type DiscoveryService struct {
	wailsApp *application.App
	runner   *discovery.Runner
	mu       sync.RWMutex
}

// NewDiscoveryService 创建发现服务实例
func NewDiscoveryService() *DiscoveryService {
	return &DiscoveryService{
		runner: discovery.NewRunner(config.DB),
	}
}

// ServiceStartup Wails 服务启动生命周期钩子
func (s *DiscoveryService) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	s.wailsApp = application.Get()
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

	// 启动事件监听器
	go s.listenEvents(taskID)

	return discovery.TaskStartResponse{TaskID: taskID}, nil
}

// listenEvents 监听发现事件并转发到前端
func (s *DiscoveryService) listenEvents(taskID string) {
	for {
		select {
		case event, ok := <-s.runner.FrontendBus:
			if !ok {
				return
			}
			if event.TaskID == taskID || event.TaskID == "" {
				// 发送事件到前端
				if s.wailsApp != nil {
					s.wailsApp.Event.Emit("discovery:event", event)
				}
			}
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
func (s *DiscoveryService) GetVendorProfiles() []*discovery.VendorCommandProfile {
	return discovery.GetAllVendorProfiles()
}

// GetSupportedVendors 获取支持的厂商列表
func (s *DiscoveryService) GetSupportedVendors() []string {
	return discovery.SupportedVendors
}
