package ui

import (
	"context"

	"github.com/NetWeaverGo/core/internal/models"
	"github.com/NetWeaverGo/core/internal/parser"
	"github.com/NetWeaverGo/core/internal/taskexec"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// TaskExecutionUIService 统一任务执行UI服务
// 仅暴露 run 维度查询、取消、订阅与拓扑查询能力。
type TaskExecutionUIService struct {
	service  *taskexec.TaskExecutionService
	bridge   *TaskExecutionEventBridge
	wailsApp *application.App
}

func NewTaskExecutionUIService(service *taskexec.TaskExecutionService) *TaskExecutionUIService {
	bridge := NewTaskExecutionEventBridge(service.GetEventBus(), service.GetSnapshotDelta)
	return &TaskExecutionUIService{
		service: service,
		bridge:  bridge,
	}
}

func (s *TaskExecutionUIService) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	s.wailsApp = application.Get()
	if s.bridge != nil {
		s.bridge.SetWailsApp(s.wailsApp)
		s.bridge.Start()
	}
	return nil
}

func (s *TaskExecutionUIService) ServiceShutdown() error {
	if s.bridge != nil {
		s.bridge.Stop()
	}
	return nil
}

func (s *TaskExecutionUIService) GetTaskSnapshot(runID string) (*taskexec.ExecutionSnapshot, error) {
	return s.service.GetSnapshot(runID)
}

func (s *TaskExecutionUIService) GetTaskSnapshotDelta(runID string) (*taskexec.SnapshotDelta, error) {
	return s.service.GetSnapshotDelta(runID)
}

func (s *TaskExecutionUIService) ListRunningTasks() []*taskexec.ExecutionSnapshot {
	return s.service.ListRunning()
}

func (s *TaskExecutionUIService) ListTaskRuns(limit int) ([]*taskexec.RunSummary, error) {
	return s.service.ListRuns(limit)
}

func (s *TaskExecutionUIService) CancelTask(runID string) error {
	return s.service.CancelRun(runID)
}

func (s *TaskExecutionUIService) GetRunStatus(runID string) (*taskexec.TaskRun, error) {
	return s.service.GetRunStatus(runID)
}

func (s *TaskExecutionUIService) SubscribeRunEvents(runID string) error {
	if s.bridge != nil {
		s.bridge.SubscribeRun(runID)
	}
	return nil
}

func (s *TaskExecutionUIService) UnsubscribeRunEvents(runID string) error {
	if s.bridge != nil {
		s.bridge.UnsubscribeRun(runID)
	}
	return nil
}

func (s *TaskExecutionUIService) GetTopologyGraph(runID string) (*models.TopologyGraphView, error) {
	return s.service.GetTopologyGraph(runID)
}

func (s *TaskExecutionUIService) GetTopologyEdgeDetail(runID string, edgeID string) (*models.TopologyEdgeDetailView, error) {
	return s.service.GetTopologyEdgeDetail(runID, edgeID)
}

func (s *TaskExecutionUIService) GetTopologyDeviceDetail(runID string, deviceIP string) (*parser.ParsedResult, error) {
	return s.service.GetTopologyDeviceDetail(runID, deviceIP)
}

func (s *TaskExecutionUIService) GetSupportedTopologyVendors() []string {
	return s.service.GetSupportedTopologyVendors()
}

func (s *TaskExecutionUIService) GetRunArtifacts(runID string) ([]taskexec.TaskArtifact, error) {
	return s.service.GetRunArtifacts(runID)
}

func (s *TaskExecutionUIService) GetTopologyCollectionPlans(runID string) ([]taskexec.TopologyCollectionPlanArtifact, error) {
	return s.service.ListTopologyCollectionPlans(runID)
}



// GetTopologyEdgeExplain 获取边解释视图（包含候选和决策轨迹）
func (s *TaskExecutionUIService) GetTopologyEdgeExplain(runID string, edgeID string) (*models.TopologyEdgeExplainView, error) {
	return s.service.GetTopologyEdgeExplain(runID, edgeID)
}

// GetTopologyDecisionTracesByRun 获取运行的所有决策轨迹
func (s *TaskExecutionUIService) GetTopologyDecisionTracesByRun(runID string) ([]models.TopologyDecisionTraceView, error) {
	return s.service.GetTopologyDecisionTracesByRun(runID)
}
