package plancompare

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/models"
	"github.com/NetWeaverGo/core/internal/normalize"
	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

// Service 规划比对服务。
type Service struct {
	db *gorm.DB
	pm *config.PathManager
}

// NewService 创建规划比对服务。
func NewService(db *gorm.DB) *Service {
	return &Service{
		db: db,
		pm: config.GetPathManager(),
	}
}

// ListPlanFiles 列出已导入的规划文件。
func (s *Service) ListPlanFiles(limit int) ([]models.PlanUploadView, error) {
	if limit <= 0 {
		limit = 50
	}
	var plans []models.PlanFile
	if err := s.db.Order("imported_at DESC").Limit(limit).Find(&plans).Error; err != nil {
		return nil, err
	}

	result := make([]models.PlanUploadView, 0, len(plans))
	for _, p := range plans {
		result = append(result, models.PlanUploadView{
			ID:         strconv.FormatUint(uint64(p.ID), 10),
			FileName:   p.FileName,
			FilePath:   p.FilePath,
			TotalLinks: p.TotalLinks,
			Warnings:   append([]string(nil), p.Warnings...),
			ImportedAt: p.ImportedAt,
		})
	}
	return result, nil
}

// ImportPlanExcel 导入固定模板 Excel 规划链路。
func (s *Service) ImportPlanExcel(filePath string) (*models.PlanImportResult, error) {
	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		return nil, fmt.Errorf("filePath 不能为空")
	}
	if _, err := os.Stat(filePath); err != nil {
		return nil, fmt.Errorf("规划文件不存在: %w", err)
	}

	if err := os.MkdirAll(s.pm.GetPlanImportDir(), 0755); err != nil {
		return nil, fmt.Errorf("创建规划目录失败: %w", err)
	}

	copiedPath, err := s.copyImportedFile(filePath)
	if err != nil {
		return nil, err
	}

	workbook, err := excelize.OpenFile(copiedPath)
	if err != nil {
		return nil, fmt.Errorf("打开Excel失败: %w", err)
	}
	defer func() { _ = workbook.Close() }()

	sheetName := workbook.GetSheetName(0)
	if strings.TrimSpace(sheetName) == "" {
		return nil, fmt.Errorf("Excel 不包含可用工作表")
	}
	rows, err := workbook.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("读取Excel行失败: %w", err)
	}

	links, warnings, err := parsePlanRows(rows)
	if err != nil {
		return nil, err
	}
	if len(links) == 0 {
		return nil, fmt.Errorf("未解析到有效规划链路")
	}

	plan := models.PlanFile{
		FileName:   filepath.Base(copiedPath),
		FilePath:   copiedPath,
		TotalLinks: len(links),
		Warnings:   warnings,
		ImportedAt: time.Now(),
	}

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&plan).Error; err != nil {
			return err
		}
		planID := strconv.FormatUint(uint64(plan.ID), 10)
		for i := range links {
			links[i].PlanFileID = planID
		}
		if err := tx.Create(&links).Error; err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("保存规划文件失败: %w", err)
	}

	return &models.PlanImportResult{
		PlanFileID: strconv.FormatUint(uint64(plan.ID), 10),
		TotalLinks: len(links),
		Warnings:   warnings,
	}, nil
}

// Compare 执行规划与实际拓扑的无向比对。
func (s *Service) Compare(taskID string, planID string) (*models.CompareResult, error) {
	planID = normalizePlanID(planID)
	if strings.TrimSpace(taskID) == "" || planID == "" {
		return nil, fmt.Errorf("taskID/planID 不能为空")
	}

	var plans []models.PlannedLink
	if err := s.db.Where("plan_file_id = ?", planID).Find(&plans).Error; err != nil {
		return nil, fmt.Errorf("加载规划链路失败: %w", err)
	}
	if len(plans) == 0 {
		return nil, fmt.Errorf("规划文件 %s 不存在或无链路", planID)
	}

	var edges []models.TopologyEdge
	if err := s.db.Where("task_id = ?", taskID).Find(&edges).Error; err != nil {
		return nil, fmt.Errorf("加载实际拓扑失败: %w", err)
	}

	var devices []models.DiscoveryDevice
	if err := s.db.Where("task_id = ?", taskID).Find(&devices).Error; err != nil {
		return nil, fmt.Errorf("加载发现设备失败: %w", err)
	}
	resolver := newDeviceResolver(devices)

	plannedRefs := make([]planRef, 0, len(plans))
	plannedByPair := make(map[string][]planRef)
	for _, p := range plans {
		ref := buildPlanRef(p, resolver)
		plannedRefs = append(plannedRefs, ref)
		plannedByPair[ref.PairKey] = append(plannedByPair[ref.PairKey], ref)
	}

	actualRefs := make([]edgeRef, 0, len(edges))
	actualByKey := make(map[string][]edgeRef)
	actualByPair := make(map[string][]edgeRef)
	for _, e := range edges {
		ref, ok := buildEdgeRef(e)
		if !ok {
			continue
		}
		actualRefs = append(actualRefs, ref)
		actualByKey[ref.EdgeKey] = append(actualByKey[ref.EdgeKey], ref)
		actualByPair[ref.PairKey] = append(actualByPair[ref.PairKey], ref)
	}

	reportID := "diff_" + uuid.NewString()[:8]
	matched := 0
	usedActualKeys := make(map[string]struct{})
	var missing []models.DiffItem
	var inconsistent []models.DiffItem
	var unexpected []models.DiffItem

	// 用于追踪同一设备对已消费的边，用于处理多链路场景
	pairConsumedCount := make(map[string]int)

	for _, p := range plannedRefs {
		if refs := actualByKey[p.EdgeKey]; len(refs) > 0 {
			matched++
			usedActualKeys[p.EdgeKey] = struct{}{}
			pairConsumedCount[p.PairKey]++
			continue
		}

		if refs := actualByPair[p.PairKey]; len(refs) > 0 {
			// 寻找第一个未消费的边进行匹配
			var matchedRef *edgeRef
			for i := range refs {
				if _, used := usedActualKeys[refs[i].EdgeKey]; !used {
					matchedRef = &refs[i]
					usedActualKeys[refs[i].EdgeKey] = struct{}{}
					pairConsumedCount[p.PairKey]++
					break
				}
			}

			if matchedRef != nil {
				item := models.DiffItem{
					ReportID:    reportID,
					DiffType:    classifyPairMismatch(p, *matchedRef),
					ADeviceName: p.Raw.ADeviceName,
					AMgmtIP:     p.Raw.AMgmtIP,
					AIf:         p.Raw.AIf,
					BDeviceName: p.Raw.BDeviceName,
					BMgmtIP:     p.Raw.BMgmtIP,
					BIf:         p.Raw.BIf,
					ExpectedIf:  fmt.Sprintf("%s <-> %s", p.NormAIf, p.NormBIf),
					ActualIf:    fmt.Sprintf("%s <-> %s", matchedRef.AIf, matchedRef.BIf),
					Reason:      "设备对存在，但接口或聚合口不一致",
					Evidence:    []string{matchedRef.EdgeID},
				}
				inconsistent = append(inconsistent, item)
				continue
			}
		}

		reason := "未在实际拓扑中找到对应链路"
		diffType := "missing_link"
		if p.Ambiguous || p.Unresolved {
			diffType = "device_mismatch"
			reason = "规划设备未能唯一映射到发现设备"
		}
		missing = append(missing, models.DiffItem{
			ReportID:    reportID,
			DiffType:    diffType,
			ADeviceName: p.Raw.ADeviceName,
			AMgmtIP:     p.Raw.AMgmtIP,
			AIf:         p.Raw.AIf,
			BDeviceName: p.Raw.BDeviceName,
			BMgmtIP:     p.Raw.BMgmtIP,
			BIf:         p.Raw.BIf,
			ExpectedIf:  fmt.Sprintf("%s <-> %s", p.NormAIf, p.NormBIf),
			Reason:      reason,
		})
	}

	for _, a := range actualRefs {
		if _, ok := usedActualKeys[a.EdgeKey]; ok {
			continue
		}

		// 检查是否属于已匹配的设备对（只要规划中有该设备对，多余的链路都标记为额外链路）
		if plannedCount, ok := plannedByPair[a.PairKey]; ok && len(plannedCount) > 0 {
			consumed := pairConsumedCount[a.PairKey]
			diffType := "unexpected_link"
			// [修复] 增强原因描述，明确说明设备对已规划的链路数量和实际额外链路
			reason := fmt.Sprintf("实际拓扑中存在规划未声明的额外链路（设备对已规划%d条链路，已匹配%d条，此为额外链路）", len(plannedCount), consumed)
			if a.Status == "semi_confirmed" {
				diffType = "one_side_only"
				reason = fmt.Sprintf("单向发现链路，设备对已规划%d条链路", len(plannedCount))
			}
			unexpected = append(unexpected, models.DiffItem{
				ReportID:    reportID,
				DiffType:    diffType,
				ADeviceName: a.ADeviceID,
				AIf:         a.AIf,
				BDeviceName: a.BDeviceID,
				BIf:         a.BIf,
				ActualIf:    fmt.Sprintf("%s <-> %s", a.AIf, a.BIf),
				Reason:      reason,
				Evidence:    []string{a.EdgeID},
			})
			continue
		}

		// 完全不匹配的设备对（规划中不存在该设备对）
		diffType := "unexpected_link"
		reason := "实际拓扑中存在规划未声明链路（规划中无此设备对）"
		if a.Status == "semi_confirmed" {
			diffType = "one_side_only"
			reason = "单向发现链路，规划中不存在此设备对"
		}
		unexpected = append(unexpected, models.DiffItem{
			ReportID:    reportID,
			DiffType:    diffType,
			ADeviceName: a.ADeviceID,
			AIf:         a.AIf,
			BDeviceName: a.BDeviceID,
			BIf:         a.BIf,
			ActualIf:    fmt.Sprintf("%s <-> %s", a.AIf, a.BIf),
			Reason:      reason,
			Evidence:    []string{a.EdgeID},
		})
	}

	items := make([]models.DiffItem, 0, len(missing)+len(unexpected)+len(inconsistent))
	items = append(items, missing...)
	items = append(items, unexpected...)
	items = append(items, inconsistent...)

	report := models.DiffReport{
		ID:                reportID,
		TaskID:            taskID,
		PlanFileID:        planID,
		TotalPlanned:      len(plannedRefs),
		TotalActual:       len(actualRefs),
		Matched:           matched,
		MissingLinks:      len(missing),
		UnexpectedLinks:   len(unexpected),
		InconsistentItems: len(inconsistent),
	}

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id = ?", reportID).Delete(&models.DiffReport{}).Error; err != nil {
			return err
		}
		if err := tx.Where("report_id = ?", reportID).Delete(&models.DiffItem{}).Error; err != nil {
			return err
		}
		if err := tx.Create(&report).Error; err != nil {
			return err
		}
		for i := range items {
			if err := tx.Create(&items[i]).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("保存比对结果失败: %w", err)
	}

	return &models.CompareResult{
		ReportID:          reportID,
		TotalPlanned:      len(plannedRefs),
		TotalActual:       len(actualRefs),
		Matched:           matched,
		MissingLinks:      missing,
		UnexpectedLinks:   unexpected,
		InconsistentItems: inconsistent,
	}, nil
}

// GetDiffReport 获取差异报告摘要。
func (s *Service) GetDiffReport(reportID string) (*models.DiffReportView, error) {
	var report models.DiffReport
	if err := s.db.Where("id = ?", reportID).First(&report).Error; err != nil {
		return nil, err
	}

	view := &models.DiffReportView{
		ID:                report.ID,
		TaskID:            report.TaskID,
		PlanFileID:        report.PlanFileID,
		TotalPlanned:      report.TotalPlanned,
		TotalActual:       report.TotalActual,
		Matched:           report.Matched,
		MissingLinks:      report.MissingLinks,
		UnexpectedLinks:   report.UnexpectedLinks,
		InconsistentItems: report.InconsistentItems,
		CreatedAt:         report.CreatedAt,
	}

	if planID, err := strconv.ParseUint(report.PlanFileID, 10, 64); err == nil {
		var plan models.PlanFile
		if err := s.db.Where("id = ?", uint(planID)).First(&plan).Error; err == nil {
			view.PlanFileName = plan.FileName
		}
	}
	return view, nil
}

// GetCompareResult 获取报告完整明细。
func (s *Service) GetCompareResult(reportID string) (*models.CompareResult, error) {
	var report models.DiffReport
	if err := s.db.Where("id = ?", reportID).First(&report).Error; err != nil {
		return nil, err
	}
	var items []models.DiffItem
	if err := s.db.Where("report_id = ?", reportID).Find(&items).Error; err != nil {
		return nil, err
	}

	result := &models.CompareResult{
		ReportID:     report.ID,
		TotalPlanned: report.TotalPlanned,
		TotalActual:  report.TotalActual,
		Matched:      report.Matched,
	}
	for _, item := range items {
		switch item.DiffType {
		case "missing_link", "device_mismatch":
			result.MissingLinks = append(result.MissingLinks, item)
		case "unexpected_link", "one_side_only":
			result.UnexpectedLinks = append(result.UnexpectedLinks, item)
		default:
			result.InconsistentItems = append(result.InconsistentItems, item)
		}
	}
	return result, nil
}

// ExportDiffReport 导出差异报告（json/csv/excel/html）。
func (s *Service) ExportDiffReport(reportID string, format string) (string, error) {
	result, err := s.GetCompareResult(reportID)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(s.pm.GetTopologyExportDir(), 0755); err != nil {
		return "", err
	}

	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" {
		format = "json"
	}

	switch format {
	case "json":
		outputPath := filepath.Join(s.pm.GetTopologyExportDir(), fmt.Sprintf("diff_%s.json", reportID))
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return "", err
		}
		if err := os.WriteFile(outputPath, data, 0644); err != nil {
			return "", err
		}
		return outputPath, nil
	case "csv":
		outputPath := filepath.Join(s.pm.GetTopologyExportDir(), fmt.Sprintf("diff_%s.csv", reportID))
		file, err := os.Create(outputPath)
		if err != nil {
			return "", err
		}
		defer func() { _ = file.Close() }()

		w := csv.NewWriter(file)
		_ = w.Write([]string{"type", "a_device", "a_mgmt_ip", "a_if", "b_device", "b_mgmt_ip", "b_if", "expected_if", "actual_if", "reason"})
		writeItemsCSV(w, result.MissingLinks)
		writeItemsCSV(w, result.UnexpectedLinks)
		writeItemsCSV(w, result.InconsistentItems)
		w.Flush()
		if err := w.Error(); err != nil {
			return "", err
		}
		return outputPath, nil
	case "excel", "xlsx":
		return s.exportDiffReportExcel(reportID, result)
	case "html", "htm":
		return s.exportDiffReportHTML(reportID, result)
	default:
		return "", fmt.Errorf("不支持的导出格式: %s", format)
	}
}

func writeItemsCSV(w *csv.Writer, items []models.DiffItem) {
	for _, item := range items {
		_ = w.Write([]string{
			item.DiffType,
			item.ADeviceName,
			item.AMgmtIP,
			item.AIf,
			item.BDeviceName,
			item.BMgmtIP,
			item.BIf,
			item.ExpectedIf,
			item.ActualIf,
			item.Reason,
		})
	}
}

func collectDiffItems(result *models.CompareResult) []models.DiffItem {
	items := make([]models.DiffItem, 0, len(result.MissingLinks)+len(result.UnexpectedLinks)+len(result.InconsistentItems))
	items = append(items, result.MissingLinks...)
	items = append(items, result.UnexpectedLinks...)
	items = append(items, result.InconsistentItems...)
	return items
}

func (s *Service) exportDiffReportExcel(reportID string, result *models.CompareResult) (string, error) {
	outputPath := filepath.Join(s.pm.GetTopologyExportDir(), fmt.Sprintf("diff_%s.xlsx", reportID))
	workbook := excelize.NewFile()
	defer func() { _ = workbook.Close() }()

	summarySheet := "Summary"
	workbook.SetSheetName("Sheet1", summarySheet)
	workbook.SetCellValue(summarySheet, "A1", "Report ID")
	workbook.SetCellValue(summarySheet, "B1", result.ReportID)
	workbook.SetCellValue(summarySheet, "A2", "Total Planned")
	workbook.SetCellValue(summarySheet, "B2", result.TotalPlanned)
	workbook.SetCellValue(summarySheet, "A3", "Total Actual")
	workbook.SetCellValue(summarySheet, "B3", result.TotalActual)
	workbook.SetCellValue(summarySheet, "A4", "Matched")
	workbook.SetCellValue(summarySheet, "B4", result.Matched)
	workbook.SetCellValue(summarySheet, "A5", "Missing")
	workbook.SetCellValue(summarySheet, "B5", len(result.MissingLinks))
	workbook.SetCellValue(summarySheet, "A6", "Unexpected")
	workbook.SetCellValue(summarySheet, "B6", len(result.UnexpectedLinks))
	workbook.SetCellValue(summarySheet, "A7", "Inconsistent")
	workbook.SetCellValue(summarySheet, "B7", len(result.InconsistentItems))

	detailSheet := "DiffItems"
	_, _ = workbook.NewSheet(detailSheet)
	headers := []string{"Type", "A Device", "A Mgmt IP", "A IF", "B Device", "B Mgmt IP", "B IF", "Expected IF", "Actual IF", "Reason"}
	for idx, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(idx+1, 1)
		workbook.SetCellValue(detailSheet, cell, header)
	}

	items := collectDiffItems(result)
	for rowIdx, item := range items {
		row := rowIdx + 2
		values := []string{
			item.DiffType,
			item.ADeviceName,
			item.AMgmtIP,
			item.AIf,
			item.BDeviceName,
			item.BMgmtIP,
			item.BIf,
			item.ExpectedIf,
			item.ActualIf,
			item.Reason,
		}
		for colIdx, value := range values {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, row)
			workbook.SetCellValue(detailSheet, cell, value)
		}
	}

	if err := workbook.SaveAs(outputPath); err != nil {
		return "", err
	}
	return outputPath, nil
}

func (s *Service) exportDiffReportHTML(reportID string, result *models.CompareResult) (string, error) {
	outputPath := filepath.Join(s.pm.GetTopologyExportDir(), fmt.Sprintf("diff_%s.html", reportID))
	items := collectDiffItems(result)

	var b strings.Builder
	b.WriteString("<!doctype html><html><head><meta charset=\"utf-8\"><title>Diff Report</title>")
	b.WriteString("<style>body{font-family:Segoe UI,Arial,sans-serif;padding:24px;background:#f6f8fa;color:#24292f}table{border-collapse:collapse;width:100%;background:#fff}th,td{border:1px solid #d0d7de;padding:8px 10px;font-size:12px;text-align:left}th{background:#f3f4f6}.card{background:#fff;border:1px solid #d0d7de;border-radius:8px;padding:12px;margin-bottom:16px}</style>")
	b.WriteString("</head><body>")
	b.WriteString("<h2>Topology Diff Report</h2>")
	b.WriteString("<div class=\"card\">")
	b.WriteString(fmt.Sprintf("<div><strong>Report ID:</strong> %s</div>", html.EscapeString(result.ReportID)))
	b.WriteString(fmt.Sprintf("<div><strong>Total Planned:</strong> %d</div>", result.TotalPlanned))
	b.WriteString(fmt.Sprintf("<div><strong>Total Actual:</strong> %d</div>", result.TotalActual))
	b.WriteString(fmt.Sprintf("<div><strong>Matched:</strong> %d</div>", result.Matched))
	b.WriteString(fmt.Sprintf("<div><strong>Missing:</strong> %d | <strong>Unexpected:</strong> %d | <strong>Inconsistent:</strong> %d</div>", len(result.MissingLinks), len(result.UnexpectedLinks), len(result.InconsistentItems)))
	b.WriteString("</div>")
	b.WriteString("<table><thead><tr><th>Type</th><th>A Device</th><th>A Mgmt IP</th><th>A IF</th><th>B Device</th><th>B Mgmt IP</th><th>B IF</th><th>Expected IF</th><th>Actual IF</th><th>Reason</th></tr></thead><tbody>")

	for _, item := range items {
		b.WriteString("<tr>")
		b.WriteString("<td>" + html.EscapeString(item.DiffType) + "</td>")
		b.WriteString("<td>" + html.EscapeString(item.ADeviceName) + "</td>")
		b.WriteString("<td>" + html.EscapeString(item.AMgmtIP) + "</td>")
		b.WriteString("<td>" + html.EscapeString(item.AIf) + "</td>")
		b.WriteString("<td>" + html.EscapeString(item.BDeviceName) + "</td>")
		b.WriteString("<td>" + html.EscapeString(item.BMgmtIP) + "</td>")
		b.WriteString("<td>" + html.EscapeString(item.BIf) + "</td>")
		b.WriteString("<td>" + html.EscapeString(item.ExpectedIf) + "</td>")
		b.WriteString("<td>" + html.EscapeString(item.ActualIf) + "</td>")
		b.WriteString("<td>" + html.EscapeString(item.Reason) + "</td>")
		b.WriteString("</tr>")
	}

	b.WriteString("</tbody></table></body></html>")

	if err := os.WriteFile(outputPath, []byte(b.String()), 0644); err != nil {
		return "", err
	}
	return outputPath, nil
}

func normalizePlanID(planID string) string {
	trimmed := strings.TrimSpace(planID)
	if trimmed == "" {
		return ""
	}
	if n, err := strconv.ParseUint(trimmed, 10, 64); err == nil {
		return strconv.FormatUint(n, 10)
	}
	return trimmed
}

func (s *Service) copyImportedFile(sourcePath string) (string, error) {
	source, err := os.Open(sourcePath)
	if err != nil {
		return "", err
	}
	defer func() { _ = source.Close() }()

	base := filepath.Base(sourcePath)
	target := filepath.Join(
		s.pm.GetPlanImportDir(),
		fmt.Sprintf("%s_%s", time.Now().Format("20060102_150405"), base),
	)
	out, err := os.Create(target)
	if err != nil {
		return "", err
	}
	defer func() { _ = out.Close() }()

	if _, err := io.Copy(out, source); err != nil {
		return "", err
	}
	return target, nil
}

type planColumnIndex struct {
	aName int
	aIP   int
	aIf   int
	bName int
	bIP   int
	bIf   int
	typ   int
	note  int
}

func parsePlanRows(rows [][]string) ([]models.PlannedLink, []string, error) {
	if len(rows) == 0 {
		return nil, nil, fmt.Errorf("Excel 为空")
	}

	indices, hasHeader := detectPlanHeader(rows[0])
	startRow := 0
	if hasHeader {
		startRow = 1
	} else {
		indices = planColumnIndex{
			aName: 0, aIP: 1, aIf: 2,
			bName: 3, bIP: 4, bIf: 5,
			typ: 6, note: 7,
		}
	}

	links := make([]models.PlannedLink, 0, len(rows))
	warnings := make([]string, 0, 8)
	seen := make(map[string]struct{})
	for i := startRow; i < len(rows); i++ {
		row := rows[i]
		aName := at(row, indices.aName)
		aIP := at(row, indices.aIP)
		aIf := at(row, indices.aIf)
		bName := at(row, indices.bName)
		bIP := at(row, indices.bIP)
		bIf := at(row, indices.bIf)
		linkType := strings.ToLower(strings.TrimSpace(at(row, indices.typ)))
		remark := at(row, indices.note)

		if isRowEmpty(aName, aIP, aIf, bName, bIP, bIf) {
			continue
		}
		if aIf == "" || bIf == "" {
			warnings = append(warnings, fmt.Sprintf("第 %d 行接口为空，已跳过", i+1))
			continue
		}
		if aIP != "" && net.ParseIP(aIP) == nil {
			warnings = append(warnings, fmt.Sprintf("第 %d 行本端管理IP格式无效: %s", i+1, aIP))
		}
		if bIP != "" && net.ParseIP(bIP) == nil {
			warnings = append(warnings, fmt.Sprintf("第 %d 行对端管理IP格式无效: %s", i+1, bIP))
		}

		normAIf := normalize.NormalizeInterfaceName(aIf)
		normBIf := normalize.NormalizeInterfaceName(bIf)
		if normAIf == "" || normBIf == "" {
			warnings = append(warnings, fmt.Sprintf("第 %d 行接口标准化失败，已跳过", i+1))
			continue
		}
		edgeKey := makeUndirectedEndpointKey(normalizeEndpointID(aIP, aName), normAIf, normalizeEndpointID(bIP, bName), normBIf)
		if _, ok := seen[edgeKey]; ok {
			warnings = append(warnings, fmt.Sprintf("第 %d 行与已存在链路重复，已去重", i+1))
			continue
		}
		seen[edgeKey] = struct{}{}

		links = append(links, models.PlannedLink{
			ADeviceName: strings.TrimSpace(aName),
			AMgmtIP:     strings.TrimSpace(aIP),
			AIf:         strings.TrimSpace(aIf),
			BDeviceName: strings.TrimSpace(bName),
			BMgmtIP:     strings.TrimSpace(bIP),
			BIf:         strings.TrimSpace(bIf),
			LinkType:    linkType,
			Remark:      strings.TrimSpace(remark),
			NormAIf:     normAIf,
			NormBIf:     normBIf,
			EdgeKey:     edgeKey,
		})
	}
	return links, warnings, nil
}

func detectPlanHeader(header []string) (planColumnIndex, bool) {
	idx := map[string]int{}
	for i, c := range header {
		key := normalizeHeader(c)
		if key != "" {
			idx[key] = i
		}
	}

	find := func(keys ...string) int {
		for _, k := range keys {
			if v, ok := idx[k]; ok {
				return v
			}
		}
		return -1
	}

	result := planColumnIndex{
		aName: find("本端设备名", "本端设备", "a设备", "a设备名"),
		aIP:   find("本端管理ip", "本端ip", "a管理ip"),
		aIf:   find("本端接口", "a接口"),
		bName: find("对端设备名", "对端设备", "b设备", "b设备名"),
		bIP:   find("对端管理ip", "对端ip", "b管理ip"),
		bIf:   find("对端接口", "b接口"),
		typ:   find("链路类型", "类型"),
		note:  find("备注"),
	}
	required := []int{result.aName, result.aIf, result.bName, result.bIf}
	for _, v := range required {
		if v < 0 {
			return planColumnIndex{}, false
		}
	}
	return result, true
}

func normalizeHeader(v string) string {
	v = strings.TrimSpace(strings.ToLower(v))
	replacer := strings.NewReplacer(" ", "", "_", "", "-", "", "：", "", ":", "")
	return replacer.Replace(v)
}

func at(row []string, idx int) string {
	if idx < 0 || idx >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[idx])
}

func isRowEmpty(values ...string) bool {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return false
		}
	}
	return true
}

type deviceResolver struct {
	byMgmtIP map[string]models.DiscoveryDevice
	byName   map[string][]models.DiscoveryDevice
}

func newDeviceResolver(devices []models.DiscoveryDevice) *deviceResolver {
	r := &deviceResolver{
		byMgmtIP: make(map[string]models.DiscoveryDevice),
		byName:   make(map[string][]models.DiscoveryDevice),
	}
	for _, d := range devices {
		if ip := strings.TrimSpace(chooseNonEmpty(d.MgmtIP, d.DeviceIP)); ip != "" {
			r.byMgmtIP[ip] = d
		}
		name := normalize.NormalizeDeviceName(chooseNonEmpty(d.DisplayName, d.NormalizedName, d.Hostname))
		if name != "" {
			r.byName[name] = append(r.byName[name], d)
		}
	}
	return r
}

func (r *deviceResolver) resolve(name, mgmtIP string) (deviceID string, unresolved bool, ambiguous bool) {
	if ip := strings.TrimSpace(mgmtIP); ip != "" {
		if d, ok := r.byMgmtIP[ip]; ok {
			return d.DeviceIP, false, false
		}
	}
	normName := normalize.NormalizeDeviceName(name)
	candidates := r.byName[normName]
	if len(candidates) == 1 {
		return candidates[0].DeviceIP, false, false
	}
	if len(candidates) > 1 {
		return candidates[0].DeviceIP, false, true
	}
	return normalizeEndpointID(mgmtIP, name), true, false
}

type planRef struct {
	Raw        models.PlannedLink
	ADeviceID  string
	BDeviceID  string
	NormAIf    string
	NormBIf    string
	EdgeKey    string
	PairKey    string
	Unresolved bool
	Ambiguous  bool
}

func buildPlanRef(link models.PlannedLink, resolver *deviceResolver) planRef {
	aDeviceID, aUnresolved, aAmbiguous := resolver.resolve(link.ADeviceName, link.AMgmtIP)
	bDeviceID, bUnresolved, bAmbiguous := resolver.resolve(link.BDeviceName, link.BMgmtIP)
	normAIf := chooseNonEmpty(link.NormAIf, normalize.NormalizeInterfaceName(link.AIf))
	normBIf := chooseNonEmpty(link.NormBIf, normalize.NormalizeInterfaceName(link.BIf))

	return planRef{
		Raw:        link,
		ADeviceID:  aDeviceID,
		BDeviceID:  bDeviceID,
		NormAIf:    normAIf,
		NormBIf:    normBIf,
		EdgeKey:    makeUndirectedEndpointKey(aDeviceID, normAIf, bDeviceID, normBIf),
		PairKey:    makeUndirectedPairKey(aDeviceID, bDeviceID),
		Unresolved: aUnresolved || bUnresolved,
		Ambiguous:  aAmbiguous || bAmbiguous,
	}
}

type edgeRef struct {
	EdgeID    string
	ADeviceID string
	AIf       string
	BDeviceID string
	BIf       string
	EdgeKey   string
	PairKey   string
	LinkType  string
	Status    string
}

func buildEdgeRef(edge models.TopologyEdge) (edgeRef, bool) {
	if strings.TrimSpace(edge.BDeviceID) == "" {
		return edgeRef{}, false
	}
	aIf := normalize.NormalizeInterfaceName(chooseNonEmpty(edge.LogicalAIf, edge.AIf))
	bIf := normalize.NormalizeInterfaceName(chooseNonEmpty(edge.LogicalBIf, edge.BIf))
	if aIf == "" || bIf == "" {
		return edgeRef{}, false
	}
	return edgeRef{
		EdgeID:    edge.ID,
		ADeviceID: edge.ADeviceID,
		AIf:       aIf,
		BDeviceID: edge.BDeviceID,
		BIf:       bIf,
		EdgeKey:   makeUndirectedEndpointKey(edge.ADeviceID, aIf, edge.BDeviceID, bIf),
		PairKey:   makeUndirectedPairKey(edge.ADeviceID, edge.BDeviceID),
		LinkType:  edge.EdgeType,
		Status:    edge.Status,
	}, true
}

func classifyPairMismatch(plan planRef, actual edgeRef) string {
	lowerType := strings.ToLower(plan.Raw.LinkType)
	if strings.Contains(lowerType, "agg") || strings.Contains(lowerType, "trunk") || strings.Contains(actual.LinkType, "aggregate") {
		return "aggregation_mismatch"
	}
	return "interface_mismatch"
}

func normalizeEndpointID(mgmtIP, name string) string {
	if ip := strings.TrimSpace(mgmtIP); ip != "" {
		return ip
	}
	n := normalize.NormalizeDeviceName(name)
	if n != "" {
		return "name:" + n
	}
	return "unknown"
}

func makeUndirectedEndpointKey(aDevice, aIf, bDevice, bIf string) string {
	left := strings.TrimSpace(aDevice) + ":" + strings.TrimSpace(aIf)
	right := strings.TrimSpace(bDevice) + ":" + strings.TrimSpace(bIf)
	keys := []string{left, right}
	sort.Strings(keys)
	return keys[0] + "<->" + keys[1]
}

func makeUndirectedPairKey(aDevice, bDevice string) string {
	keys := []string{strings.TrimSpace(aDevice), strings.TrimSpace(bDevice)}
	sort.Strings(keys)
	return keys[0] + "<->" + keys[1]
}

func chooseNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
