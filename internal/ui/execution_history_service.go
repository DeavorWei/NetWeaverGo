package ui

import (
	"context"
	"os/exec"
	"runtime"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// ExecutionHistoryService 历史执行记录查询服务
type ExecutionHistoryService struct {
	wailsApp *application.App
}

// NewExecutionHistoryService 创建历史记录服务实例
func NewExecutionHistoryService() *ExecutionHistoryService {
	return &ExecutionHistoryService{}
}

// ServiceStartup Wails 服务启动生命周期钩子
func (s *ExecutionHistoryService) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	s.wailsApp = application.Get()
	return nil
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

// ListExecutionRecords 查询历史执行记录列表
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
