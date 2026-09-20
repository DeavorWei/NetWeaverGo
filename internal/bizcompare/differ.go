package bizcompare

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/NetWeaverGo/core/internal/models"
)

// Differ 业务比对引擎
type Differ struct {
	analyzer *ImpactAnalyzer

	// DriftThreshold 数值漂移判定阈值（绝对差值）：差值 >= 阈值才算漂移；<=0 表示任意数值差异均视为漂移
	DriftThreshold float64
}

// NewDiffer 创建比对引擎
func NewDiffer() *Differ {
	return &Differ{
		analyzer: NewImpactAnalyzer(),
	}
}

// CompareSnapshots 比较变更前和变更后的两份快照
func (d *Differ) CompareSnapshots(taskID uint, before, after *DeviceSnapshot) []models.BizCompareItem {
	if before == nil && after == nil {
		return nil
	}

	// P1-3：任一阶段设备级采集失败时不产出差异项，避免"失败 vs 成功"制造整片假差异
	if before.Failed() || after.Failed() {
		return nil
	}

	deviceIP := ""
	domain := ""
	if before != nil {
		deviceIP = before.DeviceIP
		domain = before.Domain
	} else if after != nil {
		deviceIP = after.DeviceIP
		domain = after.Domain
	}

	beforeItems := make(map[string]SnapshotItem)
	if before != nil && before.Items != nil {
		beforeItems = before.Items
	}

	afterItems := make(map[string]SnapshotItem)
	if after != nil && after.Items != nil {
		afterItems = after.Items
	}

	var diffs []models.BizCompareItem

	// 1. 遍历变更前项：检测 删除 (deleted)、修改 (modified)、数值漂移 (drift)
	for key, bItem := range beforeItems {
		aItem, exists := afterItems[key]
		if !exists {
			item := models.BizCompareItem{
				CompareTaskID: taskID,
				DeviceIP:      deviceIP,
				Domain:        domain,
				ItemKey:       key,
				ItemCategory:  bItem.Category,
				DiffType:      "deleted",
				BeforeValue:   bItem.Value,
				AfterValue:    "",
			}
			d.analyzer.Analyze(&item)
			diffs = append(diffs, item)
			continue
		}

		// 两边都存在，比较值
		if bItem.Value != aItem.Value {
			diffType := "modified"
			if d.isNumericDrift(bItem.Value, aItem.Value) {
				diffType = "drift"
			}
			item := models.BizCompareItem{
				CompareTaskID: taskID,
				DeviceIP:      deviceIP,
				Domain:        domain,
				ItemKey:       key,
				ItemCategory:  bItem.Category,
				DiffType:      diffType,
				BeforeValue:   bItem.Value,
				AfterValue:    aItem.Value,
			}
			d.analyzer.Analyze(&item)
			diffs = append(diffs, item)
		}
	}

	// 2. 遍历变更后项：检测 新增 (added)
	for key, aItem := range afterItems {
		if _, exists := beforeItems[key]; !exists {
			item := models.BizCompareItem{
				CompareTaskID: taskID,
				DeviceIP:      deviceIP,
				Domain:        domain,
				ItemKey:       key,
				ItemCategory:  aItem.Category,
				DiffType:      "added",
				BeforeValue:   "",
				AfterValue:    aItem.Value,
			}
			d.analyzer.Analyze(&item)
			diffs = append(diffs, item)
		}
	}

	return diffs
}

// isNumericDrift 判断是否属于数值漂移（含可配置阈值，P1-3）
func (d *Differ) isNumericDrift(v1, v2 string) bool {
	f1, err1 := strconv.ParseFloat(strings.TrimSpace(v1), 64)
	f2, err2 := strconv.ParseFloat(strings.TrimSpace(v2), 64)
	if err1 != nil || err2 != nil {
		return false
	}
	if f1 == f2 {
		return false
	}
	if d.DriftThreshold <= 0 {
		return true
	}
	return math.Abs(f1-f2) >= d.DriftThreshold
}

// FormatDiffSummary 生成比对汇总描述
func FormatDiffSummary(diffs []models.BizCompareItem) string {
	counts := make(map[string]int)
	for _, diff := range diffs {
		counts[diff.DiffType]++
	}
	return fmt.Sprintf("总差异: %d (新增: %d, 减少: %d, 变更: %d, 漂移: %d)",
		len(diffs), counts["added"], counts["deleted"], counts["modified"], counts["drift"])
}
