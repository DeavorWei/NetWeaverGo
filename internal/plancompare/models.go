// Package plancompare 提供规划比对功能
// 数据库模型已迁移到 internal/models 包
package plancompare

import (
	"github.com/NetWeaverGo/core/internal/models"
)

// 重新导出 models 包中的类型，保持向后兼容
type PlannedLink = models.PlannedLink
type PlanFile = models.PlanFile
type DiffReport = models.DiffReport
type DiffItem = models.DiffItem
type PlanImportResult = models.PlanImportResult
type CompareResult = models.CompareResult
type DiffReportView = models.DiffReportView
type PlanUploadView = models.PlanUploadView
