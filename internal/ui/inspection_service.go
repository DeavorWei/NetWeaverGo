package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/NetWeaverGo/core/internal/inspection"
	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
	"github.com/NetWeaverGo/core/internal/report"
	"github.com/NetWeaverGo/core/internal/taskexec"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// InspectionSummaryVO 巡检指标汇总
type InspectionSummaryVO struct {
	TotalDevices  int `json:"totalDevices"`
	PassDevices   int `json:"passDevices"`
	FailDevices   int `json:"failDevices"`
	WarnDevices   int `json:"warnDevices"`
	ExceptDevices int `json:"exceptDevices"`
	TotalItems    int `json:"totalItems"`
	PassItems     int `json:"passItems"`
	FailItems     int `json:"failItems"`
	WarnItems     int `json:"warnItems"`
	ExceptItems   int `json:"exceptItems"`
	IgnoreItems   int `json:"ignoreItems"`
	ManualItems   int `json:"manualItems"`
	UntestItems   int `json:"untestItems"`
	UnaccordItems int `json:"unaccordItems"`
	BlockerCount  int `json:"blockerCount"`
	MajorCount    int `json:"majorCount"`
	MinorCount    int `json:"minorCount"`
	InfoCount     int `json:"infoCount"`
}

// InspectionService 巡检管理与报告UI服务
type InspectionService struct {
	db       *gorm.DB
	taskexec *taskexec.TaskExecutionService
}

// NewInspectionService 创建巡检服务实例
func NewInspectionService(db *gorm.DB, taskexec *taskexec.TaskExecutionService) *InspectionService {
	return &InspectionService{
		db:       db,
		taskexec: taskexec,
	}
}

// SetTaskExecutionService 注入统一任务执行服务
func (s *InspectionService) SetTaskExecutionService(svc *taskexec.TaskExecutionService) {
	s.taskexec = svc
}

// ListInspectionTemplates 获取所有巡检模板列表
func (s *InspectionService) ListInspectionTemplates() ([]models.InspectionTemplate, error) {
	if s.db == nil {
		return []models.InspectionTemplate{}, nil
	}
	var templates []models.InspectionTemplate
	if err := s.db.Order("name asc").Find(&templates).Error; err != nil {
		return nil, fmt.Errorf("查询巡检模板失败: %w", err)
	}
	return templates, nil
}

// GetInspectionTemplate 获取指定巡检模板详情
func (s *InspectionService) GetInspectionTemplate(id string) (*models.InspectionTemplate, error) {
	if s.db == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}
	var tpl models.InspectionTemplate
	if err := s.db.Where("id = ?", id).First(&tpl).Error; err != nil {
		return nil, fmt.Errorf("查询巡检模板失败: %w", err)
	}
	return &tpl, nil
}

// SaveInspectionTemplate 创建或保存巡检模板
func (s *InspectionService) SaveInspectionTemplate(tpl models.InspectionTemplate) error {
	if s.db == nil {
		return fmt.Errorf("数据库未初始化")
	}
	tpl.Name = strings.TrimSpace(tpl.Name)
	if tpl.Name == "" {
		return fmt.Errorf("模板名称不能为空")
	}

	// 分组树校验与规范化（方案 §5 5.2）：Code 唯一、ItemCodes 必须属于本模板检查项、
	// Children 递归去重，避免脏数据写入 groups JSON 列。
	if len(tpl.Groups) > 0 {
		if err := validateInspectionGroups(tpl.Groups, s.loadTemplateItemCodes(tpl.ID)); err != nil {
			return err
		}
		tpl.Groups = normalizeInspectionGroups(tpl.Groups)
	}

	if tpl.ID == "" {
		tpl.ID = "tpl-" + uuid.New().String()[:8]
		tpl.CreatedAt = time.Now()
		tpl.UpdatedAt = time.Now()
		return s.db.Create(&tpl).Error
	}

	tpl.UpdatedAt = time.Now()
	return s.db.Save(&tpl).Error
}

// loadTemplateItemCodes 加载模板已有检查项的 Code 集合，用于分组树 ItemCodes 校验。
// 返回 nil 表示该模板尚无检查项（无法校验，放行），避免阻断「先建模板后补项」的合法流程。
func (s *InspectionService) loadTemplateItemCodes(templateID string) map[string]struct{} {
	if s.db == nil || templateID == "" {
		return nil
	}
	var items []models.InspectionItem
	if err := s.db.Where("template_id = ?", templateID).Find(&items).Error; err != nil || len(items) == 0 {
		return nil
	}
	codes := make(map[string]struct{}, len(items))
	for _, it := range items {
		codes[it.Code] = struct{}{}
	}
	return codes
}

// validateInspectionGroups 校验分组树：Code 非空且全局唯一（含嵌套）、ItemCodes 必须属于该模板。
// validItemCodes 为 nil 时跳过 ItemCodes 校验。
func validateInspectionGroups(groups models.InspectionGroups, validItemCodes map[string]struct{}) error {
	seen := make(map[string]struct{})
	var walk func(gs models.InspectionGroups) error
	walk = func(gs models.InspectionGroups) error {
		for i := range gs {
			code := strings.TrimSpace(gs[i].Code)
			if code == "" {
				return fmt.Errorf("分组节点缺少 Code")
			}
			if _, dup := seen[code]; dup {
				return fmt.Errorf("分组 Code 重复: %s", code)
			}
			seen[code] = struct{}{}
			if validItemCodes != nil {
				for _, ic := range gs[i].ItemCodes {
					if _, ok := validItemCodes[ic]; !ok {
						return fmt.Errorf("分组 %q 引用了不属于本模板的检查项: %s", code, ic)
					}
				}
			}
			if err := walk(gs[i].Children); err != nil {
				return err
			}
		}
		return nil
	}
	return walk(groups)
}

// normalizeInspectionGroups 递归规范化分组树：裁剪空白节点、兄弟节点 Code 去重、ItemCodes 去重保序。
func normalizeInspectionGroups(groups models.InspectionGroups) models.InspectionGroups {
	seen := make(map[string]struct{}, len(groups))
	out := make(models.InspectionGroups, 0, len(groups))
	for i := range groups {
		code := strings.TrimSpace(groups[i].Code)
		if code == "" {
			continue
		}
		if _, dup := seen[code]; dup {
			continue
		}
		seen[code] = struct{}{}
		groups[i].Code = code
		groups[i].Name = strings.TrimSpace(groups[i].Name)
		groups[i].Children = normalizeInspectionGroups(groups[i].Children)
		groups[i].ItemCodes = dedupeStrings(groups[i].ItemCodes)
		out = append(out, groups[i])
	}
	return out
}

// dedupeStrings 对字符串切片去重保序并剔除空白项。
func dedupeStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, dup := seen[s]; dup {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

// DeleteInspectionTemplate 删除巡检模板及其关联项
func (s *InspectionService) DeleteInspectionTemplate(id string) error {
	if s.db == nil {
		return fmt.Errorf("数据库未初始化")
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("template_id = ?", id).Delete(&models.InspectionItem{}).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", id).Delete(&models.InspectionTemplate{}).Error
	})
}

// ListInspectionItems 获取指定模板的所有检查项
func (s *InspectionService) ListInspectionItems(templateID string) ([]models.InspectionItem, error) {
	if s.db == nil {
		return []models.InspectionItem{}, nil
	}
	var items []models.InspectionItem
	query := s.db.Model(&models.InspectionItem{})
	if strings.TrimSpace(templateID) != "" {
		query = query.Where("template_id = ?", templateID)
	}
	if err := query.Order("order_num asc, id asc").Find(&items).Error; err != nil {
		return nil, fmt.Errorf("查询巡检项失败: %w", err)
	}
	return items, nil
}

// SaveInspectionItem 创建或保存巡检项
func (s *InspectionService) SaveInspectionItem(item models.InspectionItem) error {
	if s.db == nil {
		return fmt.Errorf("数据库未初始化")
	}
	item.Name = strings.TrimSpace(item.Name)
	item.Code = strings.TrimSpace(item.Code)
	item.CommandKey = strings.TrimSpace(item.CommandKey)
	if item.Name == "" || item.Code == "" || item.CommandKey == "" {
		return fmt.Errorf("巡检项名称、编码与执行命令均不能为空")
	}

	if item.ID > 0 {
		item.UpdatedAt = time.Now()
		return s.db.Save(&item).Error
	}

	item.CreatedAt = time.Now()
	item.UpdatedAt = time.Now()
	return s.db.Create(&item).Error
}

// DeleteInspectionItem 删除指定巡检项
func (s *InspectionService) DeleteInspectionItem(id uint) error {
	if s.db == nil {
		return fmt.Errorf("数据库未初始化")
	}
	return s.db.Delete(&models.InspectionItem{}, id).Error
}

// ToggleInspectionItemEnabled 切换巡检项启用/禁用状态
func (s *InspectionService) ToggleInspectionItemEnabled(id uint, enabled bool) error {
	if s.db == nil {
		return fmt.Errorf("数据库未初始化")
	}
	return s.db.Model(&models.InspectionItem{}).Where("id = ?", id).Update("enabled", enabled).Error
}

// TriggerInspection 触发巡检任务
func (s *InspectionService) TriggerInspection(templateID string, deviceIPs []string, concurrency int) (string, error) {
	if s.taskexec == nil {
		return "", fmt.Errorf("任务执行服务未初始化")
	}

	cleanIPs := make([]string, 0, len(deviceIPs))
	seen := make(map[string]bool)
	for _, ip := range deviceIPs {
		ip = strings.TrimSpace(ip)
		if ip != "" && !seen[ip] {
			seen[ip] = true
			cleanIPs = append(cleanIPs, ip)
		}
	}

	if len(cleanIPs) == 0 {
		return "", fmt.Errorf("至少需要选择一台设备进行巡检")
	}

	templateID = strings.TrimSpace(templateID)
	if templateID == "" {
		templateID = "tpl-huawei-general"
	}
	if concurrency <= 0 {
		concurrency = 10
	}

	taskName := fmt.Sprintf("设备巡检-%s", time.Now().Format("20060102-150405"))
	def, err := s.taskexec.CreateInspectionTask(taskName, &taskexec.InspectionTaskConfig{
		DeviceIPs:   cleanIPs,
		TemplateID:  templateID,
		Concurrency: concurrency,
		TimeoutSec:  60,
	})
	if err != nil {
		logger.Error("InspectionService", "-", "创建巡检任务失败: %v", err)
		return "", fmt.Errorf("创建巡检任务失败: %w", err)
	}

	runID, err := s.taskexec.StartTask(context.Background(), def)
	if err != nil {
		logger.Error("InspectionService", "-", "启动巡检任务失败: %v", err)
		return "", fmt.Errorf("启动巡检任务失败: %w", err)
	}

	logger.Info("InspectionService", runID, "已触发巡检任务: template=%s, devices=%d", templateID, len(cleanIPs))
	return runID, nil
}

// GetInspectionResults 查询巡检结果列表
func (s *InspectionService) GetInspectionResults(runID, deviceIP, status, severity string) ([]models.InspectionResult, error) {
	if s.db == nil {
		return []models.InspectionResult{}, nil
	}

	targetRunID := strings.TrimSpace(runID)
	if targetRunID == "" {
		var latestRun taskexec.TaskRun
		if err := s.db.Where("run_kind = ?", string(taskexec.RunKindInspection)).Order("created_at desc").First(&latestRun).Error; err == nil && latestRun.ID != "" {
			targetRunID = latestRun.ID
		} else {
			var latestResult models.InspectionResult
			if err := s.db.Where("run_id != ''").Order("created_at desc, id desc").First(&latestResult).Error; err == nil && latestResult.RunID != "" {
				targetRunID = latestResult.RunID
			}
		}
	}

	query := s.db.Model(&models.InspectionResult{})
	if targetRunID != "" {
		query = query.Where("run_id = ?", targetRunID)
	}
	if strings.TrimSpace(deviceIP) != "" {
		query = query.Where("device_ip = ?", strings.TrimSpace(deviceIP))
	}
	if strings.TrimSpace(status) != "" {
		query = query.Where("status = ?", strings.TrimSpace(status))
	}
	if strings.TrimSpace(severity) != "" {
		query = query.Where("severity = ?", strings.TrimSpace(severity))
	}

	var results []models.InspectionResult
	if err := query.Order("device_ip asc, id asc").Find(&results).Error; err != nil {
		return nil, fmt.Errorf("查询巡检结果失败: %w", err)
	}
	return results, nil
}

// GetInspectionSummary 获取指定运行或全局最新巡检汇总指标
func (s *InspectionService) GetInspectionSummary(runID string) (*InspectionSummaryVO, error) {
	if s.db == nil {
		return &InspectionSummaryVO{}, nil
	}

	targetRunID := strings.TrimSpace(runID)
	if targetRunID == "" {
		var latestRun taskexec.TaskRun
		if err := s.db.Where("run_kind = ?", string(taskexec.RunKindInspection)).Order("created_at desc").First(&latestRun).Error; err == nil && latestRun.ID != "" {
			targetRunID = latestRun.ID
		} else {
			var latestResult models.InspectionResult
			if err := s.db.Where("run_id != ''").Order("created_at desc, id desc").First(&latestResult).Error; err == nil && latestResult.RunID != "" {
				targetRunID = latestResult.RunID
			}
		}
	}

	var results []models.InspectionResult
	query := s.db.Model(&models.InspectionResult{})
	if targetRunID != "" {
		query = query.Where("run_id = ?", targetRunID)
	}
	if err := query.Find(&results).Error; err != nil {
		return nil, fmt.Errorf("查询巡检指标数据失败: %w", err)
	}

	summary := &InspectionSummaryVO{
		TotalItems: len(results),
	}

	deviceStatusMap := make(map[string]string) // "fail" > "warn" > "pass"

	for _, r := range results {
		// 统计项状态 (完整覆盖 8 种评估状态)
		switch r.Status {
		case string(inspection.ResultPass):
			summary.PassItems++
		case string(inspection.ResultFail):
			summary.FailItems++
		case string(inspection.ResultWarning):
			summary.WarnItems++
		case string(inspection.ResultIgnore):
			summary.IgnoreItems++
		case string(inspection.ResultManual):
			summary.ManualItems++
		case string(inspection.ResultUntest):
			summary.UntestItems++
		case string(inspection.ResultUnaccord):
			summary.UnaccordItems++
		default:
			summary.ExceptItems++
		}

		// 统计严重级别
		switch r.Severity {
		case string(inspection.SeverityBlocker):
			summary.BlockerCount++
		case string(inspection.SeverityMajor):
			summary.MajorCount++
		case string(inspection.SeverityMinor):
			summary.MinorCount++
		case string(inspection.SeverityInfo):
			summary.InfoCount++
		}

		// 聚合设备整体状态 (优先级: fail > warn > except > pass > other)
		curr := deviceStatusMap[r.DeviceIP]
		if r.Status == string(inspection.ResultFail) || r.Status == string(inspection.ResultUnaccord) {
			deviceStatusMap[r.DeviceIP] = "fail"
		} else if r.Status == string(inspection.ResultWarning) {
			if curr != "fail" {
				deviceStatusMap[r.DeviceIP] = "warn"
			}
		} else if r.Status == string(inspection.ResultExcept) {
			if curr != "fail" && curr != "warn" {
				deviceStatusMap[r.DeviceIP] = "except"
			}
		} else if r.Status == string(inspection.ResultPass) {
			if curr != "fail" && curr != "warn" && curr != "except" {
				deviceStatusMap[r.DeviceIP] = "pass"
			}
		} else {
			// ignore, manual, untest
			if curr == "" {
				deviceStatusMap[r.DeviceIP] = "other"
			}
		}
	}

	summary.TotalDevices = len(deviceStatusMap)
	for _, status := range deviceStatusMap {
		switch status {
		case "fail":
			summary.FailDevices++
		case "warn":
			summary.WarnDevices++
		case "except":
			summary.ExceptDevices++
		case "pass":
			summary.PassDevices++
		}
	}

	return summary, nil
}

// ListRecentInspectionRuns 查询最近的巡检运行记录
func (s *InspectionService) ListRecentInspectionRuns(limit int) ([]taskexec.TaskRun, error) {
	if s.db == nil {
		return []taskexec.TaskRun{}, nil
	}
	if limit <= 0 {
		limit = 20
	}
	var runs []taskexec.TaskRun
	err := s.db.Where("run_kind = ?", string(taskexec.RunKindInspection)).
		Order("created_at desc").
		Limit(limit).
		Find(&runs).Error
	return runs, err
}

// ExportInspectionCSV 导出巡检报告为带 BOM 的 CSV 文本
func (s *InspectionService) ExportInspectionCSV(runID string) (string, error) {
	results, err := s.GetInspectionResults(runID, "", "", "")
	if err != nil {
		return "", err
	}
	csvText, err := inspection.ExportInspectionResultsCSV(results)
	if err != nil {
		return "", err
	}
	// 导出前脱敏自检：命中未脱敏敏感内容则阻断导出（规划方案 §5.2 P1-6）
	if err := report.ValidateExportContent(csvText); err != nil {
		return "", err
	}
	return csvText, nil
}

// ExportInspectionJSON 导出巡检报告为结构化 JSON 文本
func (s *InspectionService) ExportInspectionJSON(runID string) (string, error) {
	results, err := s.GetInspectionResults(runID, "", "", "")
	if err != nil {
		return "", err
	}
	jsonText, err := inspection.ExportInspectionResultsJSON(results)
	if err != nil {
		return "", err
	}
	// 导出前脱敏自检：命中未脱敏敏感内容则阻断导出（规划方案 §5.2 P1-6）
	if err := report.ValidateExportContent(jsonText); err != nil {
		return "", err
	}
	return jsonText, nil
}

// loadItemTexts 按 locale 加载巡检检查项文案种子（key = InspectionItem.Code）。
// 中文 locale 或加载失败返回 nil，由导出层回退原始中文内容。
func (s *InspectionService) loadItemTexts(locale string) map[string]models.InspectionItemText {
	if s.db == nil || isChineseLocaleForText(locale) {
		return nil
	}
	var rows []models.InspectionItemText
	if err := s.db.Where("locale = ?", locale).Find(&rows).Error; err != nil {
		logger.Warn("Inspection", "-", "加载巡检多语言文案失败: locale=%s, err=%v", locale, err)
		return nil
	}
	out := make(map[string]models.InspectionItemText, len(rows))
	for _, r := range rows {
		out[r.Key] = r
	}
	return out
}

// isChineseLocaleForText 与 inspection 包保持一致的 locale 判定
func isChineseLocaleForText(locale string) bool {
	l := strings.ToLower(strings.TrimSpace(locale))
	return l == "" || strings.HasPrefix(l, "zh")
}

// ExportInspectionCSVWithLocale 按 locale 导出巡检报告 CSV（locale 为空等价于中文，行为与 ExportInspectionCSV 一致）
func (s *InspectionService) ExportInspectionCSVWithLocale(runID string, locale string) (string, error) {
	results, err := s.GetInspectionResults(runID, "", "", "")
	if err != nil {
		return "", err
	}
	csvText, err := inspection.ExportInspectionResultsCSVWithLocale(results, locale, s.loadItemTexts(locale))
	if err != nil {
		return "", err
	}
	// 导出前脱敏自检：命中未脱敏敏感内容则阻断导出（规划方案 §5.2 P1-6）
	if err := report.ValidateExportContent(csvText); err != nil {
		return "", err
	}
	return csvText, nil
}

// ExportInspectionJSONWithLocale 按 locale 导出巡检报告 JSON（locale 为空等价于中文）
func (s *InspectionService) ExportInspectionJSONWithLocale(runID string, locale string) (string, error) {
	results, err := s.GetInspectionResults(runID, "", "", "")
	if err != nil {
		return "", err
	}
	jsonText, err := inspection.ExportInspectionResultsJSONWithLocale(results, locale, s.loadItemTexts(locale))
	if err != nil {
		return "", err
	}
	// 导出前脱敏自检：命中未脱敏敏感内容则阻断导出（规划方案 §5.2 P1-6）
	if err := report.ValidateExportContent(jsonText); err != nil {
		return "", err
	}
	return jsonText, nil
}

// ListDSLRules 获取已注册的 DSL 巡检规则
func (s *InspectionService) ListDSLRules(category string) ([]*inspection.DSLRule, error) {
	interp := inspection.GetGlobalDSLInterpreter()
	return interp.RulesByCategory(category), nil
}

// ExportDSLRules 导出所有 DSL 巡检规则为 JSON 文本
func (s *InspectionService) ExportDSLRules() (string, error) {
	interp := inspection.GetGlobalDSLInterpreter()
	return interp.ExportRulesJSON()
}

// ImportDSLRules 导入 DSL 巡检规则 JSON 文本
func (s *InspectionService) ImportDSLRules(jsonData string) (int, error) {
	interp := inspection.GetGlobalDSLInterpreter()
	return interp.ImportRulesJSON([]byte(jsonData))
}
