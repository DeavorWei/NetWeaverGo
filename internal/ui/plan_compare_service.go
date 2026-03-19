package ui

import (
	"context"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/models"
	"github.com/NetWeaverGo/core/internal/plancompare"
)

// PlanCompareService 规划比对服务。
type PlanCompareService struct {
	service *plancompare.Service
}

// NewPlanCompareService 创建规划比对服务实例。
func NewPlanCompareService() *PlanCompareService {
	return &PlanCompareService{
		service: plancompare.NewService(config.DB),
	}
}

// ImportPlanExcel 导入规划Excel。
func (s *PlanCompareService) ImportPlanExcel(ctx context.Context, filePath string) (*models.PlanImportResult, error) {
	return s.service.ImportPlanExcel(filePath)
}

// ListPlanFiles 列出已导入规划文件。
func (s *PlanCompareService) ListPlanFiles(ctx context.Context, limit int) ([]models.PlanUploadView, error) {
	return s.service.ListPlanFiles(limit)
}

// Compare 执行规划比对。
func (s *PlanCompareService) Compare(ctx context.Context, taskID string, planID string) (*models.CompareResult, error) {
	return s.service.Compare(taskID, planID)
}

// GetDiffReport 获取差异报告摘要。
func (s *PlanCompareService) GetDiffReport(ctx context.Context, reportID string) (*models.DiffReportView, error) {
	return s.service.GetDiffReport(reportID)
}

// GetCompareResult 获取差异报告明细。
func (s *PlanCompareService) GetCompareResult(ctx context.Context, reportID string) (*models.CompareResult, error) {
	return s.service.GetCompareResult(reportID)
}

// ExportDiffReport 导出差异报告（json/csv）。
func (s *PlanCompareService) ExportDiffReport(ctx context.Context, reportID string, format string) (string, error) {
	return s.service.ExportDiffReport(reportID, format)
}
