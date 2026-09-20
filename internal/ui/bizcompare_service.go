package ui

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/NetWeaverGo/core/internal/bizcompare"
	"github.com/NetWeaverGo/core/internal/config"
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

// ListDomains 获取支持的产品域列表
func (s *BizCompareService) ListDomains() []DomainInfo {
	return []DomainInfo{
		{Code: "S", Name: "S系列园区交换机", Description: "针对二层及基础网络，覆盖接口、MAC表、ARP表、VLAN及生成树"},
		{Code: "NE-SR", Name: "NE/SR核心路由器", Description: "针对关键路由网络，覆盖接口、BGP、OSPF、ISIS及路由统计"},
		{Code: "CE", Name: "CloudEngine数据中心交换机", Description: "针对数据中心网络，覆盖接口、EVPN、VXLAN、LLDP拓扑及大二层表项"},
	}
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

	for _, bSnap := range beforeSnaps {
		aSnap := afterMap[bSnap.DeviceIP]
		diffs := differ.CompareSnapshots(taskRecord.ID, bSnap, aSnap)
		allDiffs = append(allDiffs, diffs...)
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
	// 安全合规校验：若存在 CRITICAL 级别未脱敏敏感数据则阻断导出
	if err := report.ValidateExportContent(csvContent); err != nil {
		return "", err
	}

	return csvContent, nil
}
