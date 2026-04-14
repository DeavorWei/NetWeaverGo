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
func NewExecutionHistoryService(repo taskexec.Repository) *ExecutionHistoryService {
	return &ExecutionHistoryService{repo: repo}
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

// ==================== 删除操作 ====================

// DeleteRunRecordRequest 删除单条运行记录请求
type DeleteRunRecordRequest struct {
	RunID       string `json:"runId"`
	TaskGroupID string `json:"taskGroupId"` // 可选：用于权限验证
}

// DeleteRunRecordResponse 删除运行记录响应
type DeleteRunRecordResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// DeleteAllRunRecordsRequest 删除所有运行记录请求
type DeleteAllRunRecordsRequest struct {
	TaskGroupID string `json:"taskGroupId"` // 可选：任务组ID筛选
}

// DeleteRunRecord 删除单条运行记录
func (s *ExecutionHistoryService) DeleteRunRecord(req DeleteRunRecordRequest) (*DeleteRunRecordResponse, error) {
	logger.Debug("ExecutionHistoryService", "-", "开始删除运行记录: runID=%s, taskGroupId=%s", req.RunID, req.TaskGroupID)

	if s.repo == nil {
		logger.Error("ExecutionHistoryService", "-", "仓库未初始化，无法删除记录")
		return nil, fmt.Errorf("仓库未初始化")
	}

	if strings.TrimSpace(req.RunID) == "" {
		logger.Warn("ExecutionHistoryService", "-", "删除失败：runID 为空")
		return &DeleteRunRecordResponse{Success: false, Message: "runID 不能为空"}, nil
	}

	// 1. 检查是否正在运行
	run, err := s.repo.GetRun(context.Background(), req.RunID)
	if err != nil {
		logger.Error("ExecutionHistoryService", req.RunID, "获取运行记录失败: %v", err)
		return &DeleteRunRecordResponse{Success: false, Message: fmt.Sprintf("获取运行记录失败: %v", err)}, nil
	}

	// 2. 验证 taskGroupId 匹配（如果提供了）
	if req.TaskGroupID != "" {
		expectedID, err := strconv.ParseUint(req.TaskGroupID, 10, 64)
		if err == nil && run.TaskGroupID != uint(expectedID) {
			logger.Warn("ExecutionHistoryService", req.RunID, "无权删除：taskGroupId 不匹配 (expected=%d, actual=%d)", expectedID, run.TaskGroupID)
			return &DeleteRunRecordResponse{Success: false, Message: "无权删除该记录"}, nil
		}
	}

	logger.Verbose("ExecutionHistoryService", req.RunID, "运行记录状态: status=%s, runKind=%s", run.Status, run.RunKind)

	activeStatuses := taskexec.ActiveRunStatuses()
	for _, status := range activeStatuses {
		if run.Status == string(status) {
			logger.Warn("ExecutionHistoryService", req.RunID, "无法删除正在运行的任务: status=%s", run.Status)
			return &DeleteRunRecordResponse{Success: false, Message: "无法删除正在运行的任务"}, nil
		}
	}

	// 3. 获取关联数据用于文件删除
	units, _ := s.repo.GetUnitsByRun(context.Background(), req.RunID)
	artifacts, _ := s.repo.GetArtifactsByRun(context.Background(), req.RunID)
	logger.Debug("ExecutionHistoryService", req.RunID, "获取关联数据: units=%d, artifacts=%d", len(units), len(artifacts))

	// 4. 删除数据库记录
	if err := s.repo.DeleteRun(context.Background(), req.RunID); err != nil {
		logger.Error("ExecutionHistoryService", req.RunID, "删除数据库记录失败: %v", err)
		return &DeleteRunRecordResponse{Success: false, Message: fmt.Sprintf("删除失败: %v", err)}, nil
	}

	// 5. 异步删除关联文件
	go s.deleteRunFiles(req.RunID, run.RunKind, units, artifacts)

	logger.Info("ExecutionHistoryService", req.RunID, "运行记录删除成功")
	return &DeleteRunRecordResponse{Success: true, Message: "删除成功"}, nil
}

// DeleteAllRunRecords 删除运行记录（支持按任务组筛选）
func (s *ExecutionHistoryService) DeleteAllRunRecords(req DeleteAllRunRecordsRequest) (*DeleteRunRecordResponse, error) {
	logger.Debug("ExecutionHistoryService", "-", "开始删除运行记录: taskGroupId=%s", req.TaskGroupID)

	if s.repo == nil {
		logger.Error("ExecutionHistoryService", "-", "仓库未初始化，无法删除记录")
		return nil, fmt.Errorf("仓库未初始化")
	}

	// 解析 TaskGroupID
	var taskGroupID uint
	if req.TaskGroupID != "" {
		id, err := strconv.ParseUint(req.TaskGroupID, 10, 64)
		if err == nil {
			taskGroupID = uint(id)
		}
	}

	// 检查是否有正在运行的任务
	running, err := s.repo.ListRunningRuns(context.Background())
	if err != nil {
		logger.Error("ExecutionHistoryService", "-", "检查运行状态失败: %v", err)
		return &DeleteRunRecordResponse{Success: false, Message: fmt.Sprintf("检查运行状态失败: %v", err)}, nil
	}

	// 如果指定了 taskGroupId，只检查该任务组的运行状态
	if taskGroupID > 0 {
		var filteredRunning []taskexec.TaskRun
		for _, run := range running {
			if run.TaskGroupID == taskGroupID {
				filteredRunning = append(filteredRunning, run)
			}
		}
		running = filteredRunning
	}

	logger.Verbose("ExecutionHistoryService", "-", "检查运行中的任务: count=%d", len(running))

	if len(running) > 0 {
		logger.Warn("ExecutionHistoryService", "-", "存在正在运行的任务，无法删除: count=%d", len(running))
		return &DeleteRunRecordResponse{Success: false, Message: fmt.Sprintf("存在 %d 个正在运行的任务，无法删除", len(running))}, nil
	}

	// 获取要删除的运行记录
	var runs []taskexec.TaskRun
	if taskGroupID > 0 {
		// 只获取指定任务组的记录
		runs, _ = s.repo.ListRunsFiltered(context.Background(), 0, taskGroupID, "", "")
	} else {
		// 获取所有记录
		runs, _ = s.repo.ListRuns(context.Background(), 0)
	}
	logger.Debug("ExecutionHistoryService", "-", "获取运行记录: count=%d", len(runs))

	if len(runs) == 0 {
		return &DeleteRunRecordResponse{Success: true, Message: "没有可删除的记录"}, nil
	}

	// 删除数据库记录
	if taskGroupID > 0 {
		// 按任务组删除
		if err := s.repo.DeleteRunsByTaskGroup(context.Background(), taskGroupID); err != nil {
			logger.Error("ExecutionHistoryService", "-", "删除数据库记录失败: %v", err)
			return &DeleteRunRecordResponse{Success: false, Message: fmt.Sprintf("删除失败: %v", err)}, nil
		}
	} else {
		// 删除所有记录
		if err := s.repo.DeleteAllRunsBatch(context.Background()); err != nil {
			logger.Error("ExecutionHistoryService", "-", "批量删除数据库记录失败: %v", err)
			return &DeleteRunRecordResponse{Success: false, Message: fmt.Sprintf("删除失败: %v", err)}, nil
		}
	}

	// 异步删除关联文件
	go s.deleteAllRunFiles(runs)

	logger.Info("ExecutionHistoryService", "-", "运行记录删除成功: count=%d", len(runs))
	return &DeleteRunRecordResponse{Success: true, Message: fmt.Sprintf("成功删除 %d 条记录", len(runs))}, nil
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

// ==================== 文件操作常量 ====================

const (
	FileTypeDetail  = "detail"
	FileTypeRaw     = "raw"
	FileTypeSummary = "summary"
	FileTypeJournal = "journal"
	FileTypeReport  = "report"
)

// ==================== 设备执行详情 DTO ====================

// DeviceDetailsRequest 获取任务设备执行详情请求
type DeviceDetailsRequest struct {
	RunID string `json:"runId"` // 任务运行ID
}

// DeviceExecutionView 设备执行详情视图
type DeviceExecutionView struct {
	UnitID       string `json:"unitId"`       // 调度单元ID
	DeviceIP     string `json:"deviceIp"`     // 设备IP
	Status       string `json:"status"`       // 状态: pending/running/completed/failed/cancelled
	Progress     int    `json:"progress"`     // 进度 0-100（根据 DoneSteps/TotalSteps 计算）
	TotalSteps   int    `json:"totalSteps"`   // 总步骤数
	DoneSteps    int    `json:"doneSteps"`    // 已完成步骤数
	ErrorMessage string `json:"errorMessage"` // 错误信息
	StartedAt    string `json:"startedAt"`    // 开始时间
	FinishedAt   string `json:"finishedAt"`   // 结束时间
	DurationMs   int64  `json:"durationMs"`   // 执行时长

	// 日志文件路径
	DetailLogPath  string `json:"detailLogPath"`  // 详细日志路径
	RawLogPath     string `json:"rawLogPath"`     // 原始日志路径
	SummaryLogPath string `json:"summaryLogPath"` // 摘要日志路径
	JournalLogPath string `json:"journalLogPath"` // 流水日志路径

	// 文件存在状态
	DetailLogExists  bool `json:"detailLogExists"`  // 详细日志是否存在
	RawLogExists     bool `json:"rawLogExists"`     // 原始日志是否存在
	SummaryLogExists bool `json:"summaryLogExists"` // 摘要日志是否存在
	JournalLogExists bool `json:"journalLogExists"` // 流水日志是否存在
}

// DeviceDetailsResponse 获取任务设备执行详情响应
type DeviceDetailsResponse struct {
	RunID     string                `json:"runId"`     // 任务运行ID
	RunStatus string                `json:"runStatus"` // 任务整体状态
	Devices   []DeviceExecutionView `json:"devices"`   // 设备执行详情列表
}

// FileLocationRequest 打开文件位置请求
type FileLocationRequest struct {
	RunID    string `json:"runId"`    // 任务运行ID
	UnitID   string `json:"unitId"`   // 调度单元ID（可选，报告类型时为空）
	FileType string `json:"fileType"` // 文件类型: detail/raw/summary/journal/report
}

// FileLocationResponse 打开文件位置响应
type FileLocationResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// ReportPathRequest 获取任务报告路径请求
type ReportPathRequest struct {
	RunID string `json:"runId"` // 任务运行ID
}

// ReportPathResponse 获取任务报告路径响应
type ReportPathResponse struct {
	ReportPath string `json:"reportPath"` // 报告文件路径
	Exists     bool   `json:"exists"`     // 文件是否存在
}

// ==================== 设备执行详情 API ====================

// GetDeviceDetails 获取任务设备执行详情
func (s *ExecutionHistoryService) GetDeviceDetails(req DeviceDetailsRequest) (*DeviceDetailsResponse, error) {
	logger.Debug("ExecutionHistoryService", req.RunID, "获取任务设备执行详情")

	if s.repo == nil {
		return nil, fmt.Errorf("仓库未初始化")
	}

	// 1. 获取任务运行记录
	run, err := s.repo.GetRun(context.Background(), req.RunID)
	if err != nil {
		logger.Error("ExecutionHistoryService", req.RunID, "获取运行记录失败: %v", err)
		return nil, err
	}

	// 2. 获取所有调度单元
	units, err := s.repo.GetUnitsByRun(context.Background(), req.RunID)
	if err != nil {
		logger.Error("ExecutionHistoryService", req.RunID, "获取调度单元失败: %v", err)
		return nil, err
	}

	// 3. 转换为视图
	devices := make([]DeviceExecutionView, 0, len(units))
	for _, unit := range units {
		device := DeviceExecutionView{
			UnitID:         unit.ID,
			DeviceIP:       unit.TargetKey,
			Status:         unit.Status,
			Progress:       calculateUnitProgress(unit),
			TotalSteps:     unit.TotalSteps,
			DoneSteps:      unit.DoneSteps,
			ErrorMessage:   unit.ErrorMessage,
			DetailLogPath:  unit.DetailLogPath,
			RawLogPath:     unit.RawLogPath,
			SummaryLogPath: unit.SummaryLogPath,
			JournalLogPath: unit.JournalLogPath,
			// 检查文件是否存在
			DetailLogExists:  fileExists(unit.DetailLogPath),
			RawLogExists:     fileExists(unit.RawLogPath),
			SummaryLogExists: fileExists(unit.SummaryLogPath),
			JournalLogExists: fileExists(unit.JournalLogPath),
		}

		if unit.StartedAt != nil {
			device.StartedAt = unit.StartedAt.Format("2006-01-02 15:04:05")
		}
		if unit.FinishedAt != nil {
			device.FinishedAt = unit.FinishedAt.Format("2006-01-02 15:04:05")
			if unit.StartedAt != nil {
				device.DurationMs = unit.FinishedAt.Sub(*unit.StartedAt).Milliseconds()
			}
		}

		devices = append(devices, device)
	}

	return &DeviceDetailsResponse{
		RunID:     run.ID,
		RunStatus: run.Status,
		Devices:   devices,
	}, nil
}

// fileExists 检查文件是否存在
func fileExists(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

// calculateUnitProgress 计算单元进度
func calculateUnitProgress(unit taskexec.TaskRunUnit) int {
	if unit.TotalSteps == 0 {
		return 0
	}
	progress := (unit.DoneSteps * 100) / unit.TotalSteps
	if progress > 100 {
		progress = 100
	}
	return progress
}

// ==================== 文件操作 API ====================

// OpenFileLocation 使用系统资源管理器打开文件所在文件夹并选中文件
// 仅支持 Windows 系统
func (s *ExecutionHistoryService) OpenFileLocation(req FileLocationRequest) (*FileLocationResponse, error) {
	logger.Info("ExecutionHistoryService", req.RunID, "打开文件位置: type=%s, unitId=%s", req.FileType, req.UnitID)

	// 1. 从数据库解析文件路径（安全：不直接使用前端传入的路径）
	filePath, err := s.resolveFilePathFromRequest(req)
	if err != nil {
		return &FileLocationResponse{Success: false, Message: err.Error()}, nil
	}

	if filePath == "" {
		return &FileLocationResponse{Success: false, Message: "文件路径为空"}, nil
	}

	// 2. 验证路径安全性
	if err := s.validatePathWithinAllowedDir(filePath); err != nil {
		logger.Error("ExecutionHistoryService", "-", "路径安全验证失败: %v", err)
		return &FileLocationResponse{Success: false, Message: "文件路径不合法"}, nil
	}

	// 3. 检查文件是否存在
	_, err = os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return &FileLocationResponse{Success: false, Message: "文件不存在"}, nil
		}
		return &FileLocationResponse{Success: false, Message: fmt.Sprintf("访问文件失败: %v", err)}, nil
	}

	// 4. Windows: 使用 explorer /select,"filepath" 选中文件
	cmd := exec.Command("explorer", "/select,", filePath)

	if err := cmd.Start(); err != nil {
		logger.Error("ExecutionHistoryService", "-", "打开文件位置失败: %v", err)
		return &FileLocationResponse{Success: false, Message: fmt.Sprintf("打开失败: %v", err)}, nil
	}

	return &FileLocationResponse{Success: true, Message: "已打开文件位置"}, nil
}

// OpenFileWithDefaultAppRequest 使用系统默认应用打开文件请求
type OpenFileWithDefaultAppRequest struct {
	RunID    string `json:"runId"`    // 任务运行ID
	UnitID   string `json:"unitId"`   // 调度单元ID（可选，报告类型时为空）
	FileType string `json:"fileType"` // 文件类型: detail/raw/summary/journal/report
}

// OpenFileWithDefaultApp 使用系统默认应用打开文件
// 安全设计：通过 RunID + UnitID + FileType 在后端解析真实路径
func (s *ExecutionHistoryService) OpenFileWithDefaultApp(req OpenFileWithDefaultAppRequest) (*FileLocationResponse, error) {
	logger.Info("ExecutionHistoryService", req.RunID, "使用默认应用打开文件: type=%s, unitId=%s", req.FileType, req.UnitID)

	// 1. 从数据库解析文件路径（安全：不直接使用前端传入的路径）
	filePath, err := s.resolveFilePathFromRequest(FileLocationRequest{
		RunID:    req.RunID,
		UnitID:   req.UnitID,
		FileType: req.FileType,
	})
	if err != nil {
		return &FileLocationResponse{Success: false, Message: err.Error()}, nil
	}

	if filePath == "" {
		return &FileLocationResponse{Success: false, Message: "文件路径为空"}, nil
	}

	// 2. 验证路径安全性
	if err := s.validatePathWithinAllowedDir(filePath); err != nil {
		logger.Error("ExecutionHistoryService", "-", "路径安全验证失败: %v", err)
		return &FileLocationResponse{Success: false, Message: "文件路径不合法"}, nil
	}

	// 3. 检查文件是否存在
	if _, err := os.Stat(filePath); err != nil {
		if os.IsNotExist(err) {
			return &FileLocationResponse{Success: false, Message: "文件不存在"}, nil
		}
		return &FileLocationResponse{Success: false, Message: fmt.Sprintf("访问文件失败: %v", err)}, nil
	}

	// 4. 使用系统默认应用打开文件
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
		return &FileLocationResponse{Success: false, Message: fmt.Sprintf("打开失败: %v", err)}, nil
	}

	return &FileLocationResponse{Success: true, Message: "已打开文件"}, nil
}

// GetReportPath 获取任务报告路径
func (s *ExecutionHistoryService) GetReportPath(req ReportPathRequest) (*ReportPathResponse, error) {
	logger.Debug("ExecutionHistoryService", req.RunID, "获取任务报告路径")

	if s.repo == nil {
		return nil, fmt.Errorf("仓库未初始化")
	}

	// 从产物索引中查找报告文件
	artifacts, err := s.repo.GetArtifactsByRun(context.Background(), req.RunID)
	if err != nil {
		logger.Error("ExecutionHistoryService", req.RunID, "获取产物索引失败: %v", err)
		return nil, err
	}

	for _, artifact := range artifacts {
		if artifact.ArtifactType == FileTypeReport && artifact.FilePath != "" {
			_, err := os.Stat(artifact.FilePath)
			exists := err == nil
			return &ReportPathResponse{
				ReportPath: artifact.FilePath,
				Exists:     exists,
			}, nil
		}
	}

	return &ReportPathResponse{
		ReportPath: "",
		Exists:     false,
	}, nil
}

// ==================== 安全辅助方法 ====================

// validatePathWithinAllowedDir 验证路径是否在允许的目录范围内
// 防止路径遍历攻击
func (s *ExecutionHistoryService) validatePathWithinAllowedDir(filePath string) error {
	// 获取绝对路径
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return fmt.Errorf("无效的文件路径")
	}

	// 获取允许的基础目录
	pm := config.GetPathManager()
	allowedDirs := []string{
		pm.ExecutionLiveLogDir,
		pm.TopologyRawDir,
		filepath.Join(pm.StorageRoot, "topology", "normalized"),
	}

	for _, allowedDir := range allowedDirs {
		absAllowed, err := filepath.Abs(allowedDir)
		if err != nil {
			continue
		}
		// 检查文件路径是否在允许的目录下
		if strings.HasPrefix(absPath, absAllowed+string(filepath.Separator)) {
			return nil
		}
	}

	return fmt.Errorf("文件路径不在允许的目录范围内")
}

// resolveFilePathFromRequest 根据请求参数解析文件路径
// 安全设计：从数据库获取路径，而非直接使用前端传入的路径
func (s *ExecutionHistoryService) resolveFilePathFromRequest(req FileLocationRequest) (string, error) {
	if s.repo == nil {
		return "", fmt.Errorf("仓库未初始化")
	}

	// 报告类型特殊处理
	if req.FileType == FileTypeReport {
		artifacts, err := s.repo.GetArtifactsByRun(context.Background(), req.RunID)
		if err != nil {
			return "", err
		}
		for _, artifact := range artifacts {
			if artifact.ArtifactType == FileTypeReport && artifact.FilePath != "" {
				return artifact.FilePath, nil
			}
		}
		return "", fmt.Errorf("未找到报告文件")
	}

	// 其他类型从 Unit 获取
	if req.UnitID == "" {
		return "", fmt.Errorf("UnitID 不能为空")
	}

	unit, err := s.repo.GetUnit(context.Background(), req.UnitID)
	if err != nil {
		return "", err
	}

	// 验证 Unit 属于该 Run
	if unit.TaskRunID != req.RunID {
		return "", fmt.Errorf("Unit 不属于该任务运行")
	}

	switch req.FileType {
	case FileTypeDetail:
		return unit.DetailLogPath, nil
	case FileTypeRaw:
		return unit.RawLogPath, nil
	case FileTypeSummary:
		return unit.SummaryLogPath, nil
	case FileTypeJournal:
		return unit.JournalLogPath, nil
	default:
		return "", fmt.Errorf("不支持的文件类型: %s", req.FileType)
	}
}
