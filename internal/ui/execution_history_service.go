package ui

import (
	"context"
	"os/exec"
	"runtime"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
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

// ListExecutionRecordsRequest 列表查询请求
type ListExecutionRecordsRequest struct {
	TaskGroupID  string `json:"taskGroupId"`
	RunnerSource string `json:"runnerSource"`
	Status       string `json:"status"`
	Page         int    `json:"page"`
	PageSize     int    `json:"pageSize"`
	SortBy       string `json:"sortBy"`
	SortOrder    string `json:"sortOrder"`
}

// ListExecutionRecordsResponse 列表查询响应
type ListExecutionRecordsResponse struct {
	Data       []models.ExecutionRecord `json:"data"`
	Total      int64                    `json:"total"`
	Page       int                      `json:"page"`
	PageSize   int                      `json:"pageSize"`
	TotalPages int                      `json:"totalPages"`
}

// ListExecutionRecords 查询历史执行记录列表（兼容旧接口）
func (s *ExecutionHistoryService) ListExecutionRecords(req ListExecutionRecordsRequest) (*ListExecutionRecordsResponse, error) {
	opts := config.ExecutionQueryOptions{
		TaskGroupID:  req.TaskGroupID,
		RunnerSource: req.RunnerSource,
		Status:       req.Status,
		Page:         req.Page,
		PageSize:     req.PageSize,
		SortBy:       req.SortBy,
		SortOrder:    req.SortOrder,
	}

	result, err := config.ListExecutionRecords(opts)
	if err != nil {
		return nil, err
	}

	return &ListExecutionRecordsResponse{
		Data:       result.Data,
		Total:      result.Total,
		Page:       result.Page,
		PageSize:   result.PageSize,
		TotalPages: result.TotalPages,
	}, nil
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

// GetExecutionRecord 根据 ID 获取历史执行记录详情
func (s *ExecutionHistoryService) GetExecutionRecord(id string) (*models.ExecutionRecord, error) {
	return config.GetExecutionRecord(id)
}

// DeleteExecutionRecord 删除历史执行记录
func (s *ExecutionHistoryService) DeleteExecutionRecord(id string) error {
	return config.DeleteExecutionRecord(id)
}

// GetExecutionRecordStats 获取执行记录统计信息
func (s *ExecutionHistoryService) GetExecutionRecordStats(taskGroupID string) (map[string]interface{}, error) {
	return config.GetExecutionRecordStats(taskGroupID)
}

// ExecutionRecordStatus 历史记录状态常量
type ExecutionRecordStatus struct{}

// GetStatusList 获取所有可用的状态列表
func (s *ExecutionHistoryService) GetStatusList() []string {
	return []string{
		"completed", // 成功
		"partial",   // 部分成功
		"failed",    // 失败
		"cancelled", // 已取消
	}
}

// GetModeList 获取所有可用的执行模式列表
func (s *ExecutionHistoryService) GetModeList() []string {
	return []string{
		"group",     // 任务组模式A
		"binding",   // 任务组模式B
		"topology",  // 任务组拓扑采集
		"discovery", // 发现任务
		"manual",    // 普通执行
		"backup",    // 备份执行
	}
}

// GetRunnerSourceList 获取所有可用的执行来源列表
func (s *ExecutionHistoryService) GetRunnerSourceList() []string {
	return []string{
		"task_group",        // 任务组服务
		"discovery_service", // 发现服务
		"engine_service",    // 引擎服务
		"backup_service",    // 备份服务
	}
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
