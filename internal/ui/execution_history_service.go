package ui

import (
	"context"
	"os/exec"
	"runtime"

	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/taskexec"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// ExecutionHistoryService 历史执行记录查询服务
type ExecutionHistoryService struct {
	wailsApp             *application.App
	taskExecutionService *taskexec.TaskExecutionService
}

// NewExecutionHistoryService 创建历史记录服务实例
func NewExecutionHistoryService() *ExecutionHistoryService {
	return &ExecutionHistoryService{}
}

// SetTaskExecutionService 设置统一任务执行服务（阶段5：统一执行历史）
func (s *ExecutionHistoryService) SetTaskExecutionService(service *taskexec.TaskExecutionService) {
	s.taskExecutionService = service
}

// ServiceStartup Wails 服务启动生命周期钩子
func (s *ExecutionHistoryService) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	s.wailsApp = application.Get()
	return nil
}

// TaskRunRecordView 统一运行时历史记录视图（DTO）
type TaskRunRecordView struct {
	ID            string `json:"id"`
	RunnerSource  string `json:"runnerSource"`
	TaskGroupID   string `json:"taskGroupId"`
	TaskGroupName string `json:"taskGroupName"`
	TaskName      string `json:"taskName"`
	Mode          string `json:"mode"`
	Status        string `json:"status"`
	TotalDevices  int    `json:"totalDevices"`
	FinishedCount int    `json:"finishedCount"`
	SuccessCount  int    `json:"successCount"`
	ErrorCount    int    `json:"errorCount"`
	StartedAt     string `json:"startedAt"`
	FinishedAt    string `json:"finishedAt"`
	DurationMs    int64  `json:"durationMs"`
	RunKind       string `json:"runKind"`
}

// ListTaskRunRecordsRequest 统一运行时历史记录查询请求
type ListTaskRunRecordsRequest struct {
	RunKind string `json:"runKind"` // normal / topology / 空表示全部
	Status  string `json:"status"`  // 状态筛选
	Limit   int    `json:"limit"`   // 返回数量限制
}

// ListTaskRunRecordsResponse 统一运行时历史记录查询响应
type ListTaskRunRecordsResponse struct {
	Data  []TaskRunRecordView `json:"data"`
	Total int                 `json:"total"`
}

// ListTaskRunRecords 从统一运行时查询历史记录（阶段5：统一执行历史）
func (s *ExecutionHistoryService) ListTaskRunRecords(req ListTaskRunRecordsRequest) (*ListTaskRunRecordsResponse, error) {
	if s.taskExecutionService == nil {
		return &ListTaskRunRecordsResponse{
			Data:  []TaskRunRecordView{},
			Total: 0,
		}, nil
	}

	// 从统一运行时获取运行记录
	runs, err := s.taskExecutionService.ListRuns(req.Limit)
	if err != nil {
		return nil, err
	}

	// 筛选和转换
	views := make([]TaskRunRecordView, 0, len(runs))
	for _, run := range runs {
		// 按 runKind 筛选
		if req.RunKind != "" && run.RunKind != req.RunKind {
			continue
		}
		// 按状态筛选
		if req.Status != "" && run.Status != req.Status {
			continue
		}

		view := TaskRunRecordView{
			ID:            run.RunID,
			RunnerSource:  "taskexec",
			TaskGroupID:   "",
			TaskGroupName: "",
			TaskName:      run.TaskName,
			Mode:          run.RunKind,
			Status:        run.Status,
			TotalDevices:  run.TotalUnits,
			FinishedCount: run.SuccessUnits + run.FailedUnits,
			SuccessCount:  run.SuccessUnits,
			ErrorCount:    run.FailedUnits,
			StartedAt:     run.StartedAt.Format("2006-01-02 15:04:05"),
			FinishedAt:    run.FinishedAt.Format("2006-01-02 15:04:05"),
			DurationMs:    run.DurationMs,
			RunKind:       run.RunKind,
		}
		views = append(views, view)
	}

	return &ListTaskRunRecordsResponse{
		Data:  views,
		Total: len(views),
	}, nil
}

// OpenFileWithDefaultApp 使用系统默认应用打开文件
func (s *ExecutionHistoryService) OpenFileWithDefaultApp(filePath string) error {
	logger.Info("ExecutionHistoryService", "-", "打开文件: %s", filePath)

	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", filePath)
	case "darwin":
		cmd = exec.Command("open", filePath)
	default: // linux and others
		cmd = exec.Command("xdg-open", filePath)
	}

	if err := cmd.Start(); err != nil {
		logger.Error("ExecutionHistoryService", "-", "打开文件失败: %v", err)
		return err
	}

	return nil
}
