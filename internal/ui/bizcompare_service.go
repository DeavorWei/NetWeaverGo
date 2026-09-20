package ui

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/NetWeaverGo/core/internal/bizcompare"
	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
	"github.com/NetWeaverGo/core/internal/report"
)

// BizCompareService 业务比对前端服务
type BizCompareService struct{}

// NewBizCompareService 创建业务比对服务实例
func NewBizCompareService() *BizCompareService {
	return &BizCompareService{}
}

// DomainInfo 产品域信息
type DomainInfo struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ListDomains 获取支持的产品域列表（P3-6：由场景种子派生，消除硬编码重复定义）
func (s *BizCompareService) ListDomains() []DomainInfo {
	mgr := bizcompare.GetGlobalSceneManager()
	seen := make(map[string]struct{})
	domains := make([]DomainInfo, 0, 3)
	for _, sc := range mgr.ListByDomain("") {
		code := strings.TrimSpace(sc.Domain)
		if code == "" || code == "*" {
			continue
		}
		if _, ok := seen[code]; ok {
			continue
		}
		seen[code] = struct{}{}
		domains = append(domains, DomainInfo{
			Code:        code,
			Name:        sc.Name,
			Description: sc.Description,
		})
	}
	sort.Slice(domains, func(i, j int) bool { return domains[i].Code < domains[j].Code })
	return domains
}

// ListScenes 获取指定域支持的场景列表
func (s *BizCompareService) ListScenes(domain string) []*bizcompare.SceneDefinition {
	mgr := bizcompare.GetGlobalSceneManager()
	return mgr.ListByDomain(domain)
}

// RunComparison 执行变更前与变更后快照比对
func (s *BizCompareService) RunComparison(name, domain, sceneID, beforeRunID, afterRunID string) (*models.BizCompareTask, []models.BizCompareItem, error) {
	if strings.TrimSpace(beforeRunID) == "" || strings.TrimSpace(afterRunID) == "" {
		return nil, nil, errors.New("变更前与变更后的 RunID 均不能为空")
	}

	store := bizcompare.GetGlobalSnapshotStore()
	beforeSnaps, err := store.ListSnapshots(beforeRunID)
	if err != nil || len(beforeSnaps) == 0 {
		return nil, nil, fmt.Errorf("未找到变更前快照数据 (runID=%s)", beforeRunID)
	}

	afterSnaps, err := store.ListSnapshots(afterRunID)
	if err != nil || len(afterSnaps) == 0 {
		return nil, nil, fmt.Errorf("未找到变更后快照数据 (runID=%s)", afterRunID)
	}

	for _, bSnap := range beforeSnaps {
		if bSnap.Phase != "" && bSnap.Phase != "before" {
			return nil, nil, fmt.Errorf("变更前快照 phase 异常: 期望 before, 实际 %s", bSnap.Phase)
		}
	}
	for _, aSnap := range afterSnaps {
		if aSnap.Phase != "" && aSnap.Phase != "after" {
			return nil, nil, fmt.Errorf("变更后快照 phase 异常: 期望 after, 实际 %s", aSnap.Phase)
		}
	}

	afterMap := make(map[string]*bizcompare.DeviceSnapshot)
	for _, snap := range afterSnaps {
		afterMap[snap.DeviceIP] = snap
	}

	differ := bizcompare.NewDiffer()
	// P1-3：数值漂移阈值可配置（全局设置 bizCompareDriftThreshold，<=0 表示任意数值差异均视为漂移）
	if st := config.GetGlobalSettings(); st != nil && st.BizCompareDriftThreshold > 0 {
		differ.DriftThreshold = st.BizCompareDriftThreshold
	}
	var allDiffs []models.BizCompareItem

	taskRecord := models.BizCompareTask{
		TaskID:      fmt.Sprintf("bizcmp-%d", time.Now().UnixNano()),
		Name:        name,
		Domain:      domain,
		SceneID:     sceneID,
		BeforeRunID: beforeRunID,
		AfterRunID:  afterRunID,
		Status:      "completed",
	}

	db := config.GetDB()
	if db != nil {
		if err := db.Create(&taskRecord).Error; err != nil {
			return nil, nil, fmt.Errorf("保存比对任务记录失败: %w", err)
		}
	}

	skippedFailed := 0
	for _, bSnap := range beforeSnaps {
		aSnap := afterMap[bSnap.DeviceIP]
		// P1-3：任一侧采集失败或缺失时跳过该设备，避免"失败 vs 成功"制造整片假差异
		if aSnap == nil || bSnap.Failed() || aSnap.Failed() {
			skippedFailed++
			logger.Warn("BizCompare", "-", "跳过采集失败/缺失的设备: ip=%s before=%s after=%s",
				bSnap.DeviceIP, snapshotStatus(bSnap), snapshotStatus(aSnap))
			continue
		}
		diffs := differ.CompareSnapshots(taskRecord.ID, bSnap, aSnap)
		allDiffs = append(allDiffs, diffs...)
	}
	if skippedFailed > 0 {
		logger.Warn("BizCompare", "-", "本次比对跳过 %d 台采集失败/缺失设备（未计入差异）", skippedFailed)
	}

	taskRecord.DiffCount = len(allDiffs)
	if db != nil {
		if err := db.Model(&taskRecord).Update("diff_count", len(allDiffs)).Error; err != nil {
			return nil, nil, fmt.Errorf("更新比对差异计数失败: %w", err)
		}
		if len(allDiffs) > 0 {
			if err := db.Create(&allDiffs).Error; err != nil {
				return nil, nil, fmt.Errorf("保存比对差异详情失败: %w", err)
			}
		}
	}

	return &taskRecord, allDiffs, nil
}

// snapshotStatus 返回快照状态描述（用于跳过日志）
func snapshotStatus(s *bizcompare.DeviceSnapshot) string {
	if s == nil {
		return "missing"
	}
	if s.Failed() {
		if s.Error != "" {
			return "failed(" + s.Error + ")"
		}
		return "failed"
	}
	return "ok"
}

// GetCompareTask 查询比对任务详情及差异项
func (s *BizCompareService) GetCompareTask(taskID string) (*models.BizCompareTask, []models.BizCompareItem, error) {
	db := config.GetDB()
	if db == nil {
		return nil, nil, errors.New("数据库未初始化")
	}

	var task models.BizCompareTask
	if err := db.Where("task_id = ?", taskID).First(&task).Error; err != nil {
		return nil, nil, err
	}

	var items []models.BizCompareItem
	if err := db.Where("compare_task_id = ?", task.ID).Find(&items).Error; err != nil {
		return nil, nil, err
	}

	return &task, items, nil
}

// ExportDiffCSV 导出比对差异项为 CSV 文本
func (s *BizCompareService) ExportDiffCSV(taskID string) (string, error) {
	task, items, err := s.GetCompareTask(taskID)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	// UTF-8 BOM
	buf.WriteString("\xEF\xBB\xBF")

	writer := csv.NewWriter(&buf)
	_ = writer.Write([]string{
		"任务名称", "设备IP", "产品域", "指标分类", "采集项Key", "差异类型", "变更前状态/数值", "变更后状态/数值", "影响级别", "影响范围说明",
	})

	for _, item := range items {
		_ = writer.Write([]string{
			task.Name,
			item.DeviceIP,
			item.Domain,
			item.ItemCategory,
			item.ItemKey,
			item.DiffType,
			item.BeforeValue,
			item.AfterValue,
			item.ImpactLevel,
			item.ImpactScope,
		})
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", err
	}

	csvContent := buf.String()
	// 导出前脱敏（P0-1）：先应用全局 + 分厂商/通用规则掩码，再执行自检兜底
	csvContent = report.SanitizeContent("", "", "", csvContent)
	// 安全合规校验：若存在 CRITICAL 级别未脱敏敏感数据则阻断导出
	if err := report.ValidateExportContent(csvContent); err != nil {
		return "", err
	}

	return csvContent, nil
}
