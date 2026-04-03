package ui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/taskexec"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// ExecutionHistoryService 历史执行记录查询服务
type ExecutionHistoryService struct {
	wailsApp             *application.App
	taskExecutionService *taskexec.TaskExecutionService
	repo                 taskexec.Repository // 新增：直接依赖 Repository
}

// NewExecutionHistoryService 创建历史记录服务实例
func NewExecutionHistoryService() *ExecutionHistoryService {
	return &ExecutionHistoryService{}
}

// SetRepository 设置仓库（由应用启动时注入）
func (s *ExecutionHistoryService) SetRepository(repo taskexec.Repository) {
	s.repo = repo
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
	RunKind     string `json:"runKind"`     // normal / topology / 空表示全部
	Status      string `json:"status"`      // 状态筛选
	Limit       int    `json:"limit"`       // 返回数量限制
	TaskGroupID string `json:"taskGroupId"` // 任务组ID筛选
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

	// 解析 TaskGroupID
	var taskGroupID uint
	if req.TaskGroupID != "" {
		id, err := strconv.ParseUint(req.TaskGroupID, 10, 64)
		if err == nil {
			taskGroupID = uint(id)
		}
	}

	// 从统一运行时获取运行记录（支持筛选）
	runs, err := s.taskExecutionService.ListRunsFiltered(req.Limit, taskGroupID, req.RunKind, req.Status)
	if err != nil {
		return nil, err
	}

	// 转换
	views := make([]TaskRunRecordView, 0, len(runs))
	for _, run := range runs {
		view := TaskRunRecordView{
			ID:            run.RunID,
			RunnerSource:  "taskexec",
			TaskGroupID:   fmt.Sprintf("%d", run.TaskGroupID),
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

// ==================== 删除操作 ====================

// DeleteRunRecordResponse 删除运行记录响应
type DeleteRunRecordResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// DeleteRunRecord 删除单条运行记录
func (s *ExecutionHistoryService) DeleteRunRecord(runID string) (*DeleteRunRecordResponse, error) {
	logger.Debug("ExecutionHistoryService", "-", "开始删除运行记录: runID=%s", runID)

	if s.repo == nil {
		logger.Error("ExecutionHistoryService", "-", "仓库未初始化，无法删除记录")
		return nil, fmt.Errorf("仓库未初始化")
	}

	if strings.TrimSpace(runID) == "" {
		logger.Warn("ExecutionHistoryService", "-", "删除失败：runID 为空")
		return &DeleteRunRecordResponse{Success: false, Message: "runID 不能为空"}, nil
	}

	// 1. 检查是否正在运行
	run, err := s.repo.GetRun(context.Background(), runID)
	if err != nil {
		logger.Error("ExecutionHistoryService", runID, "获取运行记录失败: %v", err)
		return &DeleteRunRecordResponse{Success: false, Message: fmt.Sprintf("获取运行记录失败: %v", err)}, nil
	}

	logger.Verbose("ExecutionHistoryService", runID, "运行记录状态: status=%s, runKind=%s", run.Status, run.RunKind)

	activeStatuses := taskexec.ActiveRunStatuses()
	for _, status := range activeStatuses {
		if run.Status == string(status) {
			logger.Warn("ExecutionHistoryService", runID, "无法删除正在运行的任务: status=%s", run.Status)
			return &DeleteRunRecordResponse{Success: false, Message: "无法删除正在运行的任务"}, nil
		}
	}

	// 2. 获取关联数据用于文件删除
	units, _ := s.repo.GetUnitsByRun(context.Background(), runID)
	artifacts, _ := s.repo.GetArtifactsByRun(context.Background(), runID)
	logger.Debug("ExecutionHistoryService", runID, "获取关联数据: units=%d, artifacts=%d", len(units), len(artifacts))

	// 3. 删除数据库记录
	if err := s.repo.DeleteRun(context.Background(), runID); err != nil {
		logger.Error("ExecutionHistoryService", runID, "删除数据库记录失败: %v", err)
		return &DeleteRunRecordResponse{Success: false, Message: fmt.Sprintf("删除失败: %v", err)}, nil
	}

	// 4. 异步删除关联文件
	go s.deleteRunFiles(runID, run.RunKind, units, artifacts)

	logger.Info("ExecutionHistoryService", runID, "运行记录删除成功")
	return &DeleteRunRecordResponse{Success: true, Message: "删除成功"}, nil
}

// DeleteAllRunRecords 删除所有运行记录
func (s *ExecutionHistoryService) DeleteAllRunRecords() (*DeleteRunRecordResponse, error) {
	logger.Debug("ExecutionHistoryService", "-", "开始删除所有运行记录")

	if s.repo == nil {
		logger.Error("ExecutionHistoryService", "-", "仓库未初始化，无法删除记录")
		return nil, fmt.Errorf("仓库未初始化")
	}

	// 检查是否有正在运行的任务
	running, err := s.repo.ListRunningRuns(context.Background())
	if err != nil {
		logger.Error("ExecutionHistoryService", "-", "检查运行状态失败: %v", err)
		return &DeleteRunRecordResponse{Success: false, Message: fmt.Sprintf("检查运行状态失败: %v", err)}, nil
	}

	logger.Verbose("ExecutionHistoryService", "-", "检查运行中的任务: count=%d", len(running))

	if len(running) > 0 {
		logger.Warn("ExecutionHistoryService", "-", "存在正在运行的任务，无法删除全部: count=%d", len(running))
		return &DeleteRunRecordResponse{Success: false, Message: fmt.Sprintf("存在 %d 个正在运行的任务，无法删除全部", len(running))}, nil
	}

	// 获取所有运行记录用于文件删除
	runs, _ := s.repo.ListRuns(context.Background(), 0)
	logger.Debug("ExecutionHistoryService", "-", "获取所有运行记录: count=%d", len(runs))

	// 删除数据库记录（使用批量删除优化）
	if err := s.repo.DeleteAllRunsBatch(context.Background()); err != nil {
		logger.Error("ExecutionHistoryService", "-", "批量删除数据库记录失败: %v", err)
		return &DeleteRunRecordResponse{Success: false, Message: fmt.Sprintf("删除失败: %v", err)}, nil
	}

	// 异步删除所有关联文件
	go s.deleteAllRunFiles(runs)

	logger.Info("ExecutionHistoryService", "-", "所有运行记录删除成功: count=%d", len(runs))
	return &DeleteRunRecordResponse{Success: true, Message: "删除成功"}, nil
}

// deleteRunFiles 删除运行关联的文件
func (s *ExecutionHistoryService) deleteRunFiles(runID, runKind string, units []taskexec.TaskRunUnit, artifacts []taskexec.TaskArtifact) {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("ExecutionHistoryService", runID, "删除文件时发生panic: %v", r)
		}
	}()

	pm := config.GetPathManager()

	// 1. 删除拓扑采集文件目录
	if runKind == "topology" {
		rawDir := filepath.Join(pm.TopologyRawDir, "run_"+runID)
		normalizedDir := filepath.Join(pm.StorageRoot, "topology", "normalized", "run_"+runID)

		if err := os.RemoveAll(rawDir); err != nil {
			logger.Error("ExecutionHistoryService", runID, "删除原始数据目录失败: %v", err)
		}
		if err := os.RemoveAll(normalizedDir); err != nil {
			logger.Error("ExecutionHistoryService", runID, "删除标准化数据目录失败: %v", err)
		}
	}

	// 2. 删除执行日志文件（从 Unit 的日志路径）
	for _, unit := range units {
		s.deleteLogFile(unit.SummaryLogPath, runID, "summary")
		s.deleteLogFile(unit.DetailLogPath, runID, "detail")
		s.deleteLogFile(unit.RawLogPath, runID, "raw")
		s.deleteLogFile(unit.JournalLogPath, runID, "journal")
	}

	// 3. 删除产物文件
	for _, artifact := range artifacts {
		if artifact.FilePath != "" {
			if err := os.Remove(artifact.FilePath); err != nil && !os.IsNotExist(err) {
				logger.Error("ExecutionHistoryService", runID, "删除产物文件失败 [%s]: %v", artifact.FilePath, err)
			}
		}
	}
}

// deleteLogFile 安全删除日志文件
func (s *ExecutionHistoryService) deleteLogFile(path, runID, logType string) {
	if path == "" {
		return
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		logger.Error("ExecutionHistoryService", runID, "删除%s日志失败 [%s]: %v", logType, path, err)
	}
}

// deleteAllRunFiles 删除所有运行的文件
func (s *ExecutionHistoryService) deleteAllRunFiles(runs []taskexec.TaskRun) {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("ExecutionHistoryService", "-", "批量删除文件时发生panic: %v", r)
		}
	}()

	pm := config.GetPathManager()

	// 删除整个拓扑目录
	if err := os.RemoveAll(pm.TopologyRawDir); err != nil {
		logger.Error("ExecutionHistoryService", "-", "删除原始数据目录失败: %v", err)
	}

	normalizedDir := filepath.Join(pm.StorageRoot, "topology", "normalized")
	if err := os.RemoveAll(normalizedDir); err != nil {
		logger.Error("ExecutionHistoryService", "-", "删除标准化数据目录失败: %v", err)
	}

	// 删除执行日志目录
	if err := os.RemoveAll(pm.ExecutionLiveLogDir); err != nil {
		logger.Error("ExecutionHistoryService", "-", "删除执行日志目录失败: %v", err)
	}

	// 重新创建空目录
	os.MkdirAll(pm.TopologyRawDir, 0755)
	os.MkdirAll(normalizedDir, 0755)
	os.MkdirAll(pm.ExecutionLiveLogDir, 0755)
}
