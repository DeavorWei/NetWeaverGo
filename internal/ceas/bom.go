package ceas

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"regexp"
	"strings"

	"github.com/NetWeaverGo/core/internal/models"
)

var (
	reSlotBlock = regexp.MustCompile(`(?i)\[(?:Slot|Unit)_?(\S+)\]`)
	reItemInBlock = regexp.MustCompile(`(?im)^\s*Item\s*=\s*(\S+)`)
)

// GetInvolvedSlots 从硬件树中比对 BOM 观察清单，输出命中告警列表
func GetInvolvedSlots(tree *HardwareTree, watchlist []models.BOMWatchlistItem) []BOMAlertItem {
	if tree == nil || len(watchlist) == 0 {
		return nil
	}

	watchMap := make(map[string]models.BOMWatchlistItem, len(watchlist))
	for _, w := range watchlist {
		if w.Enabled {
			key := strings.ToUpper(strings.TrimSpace(w.Item))
			watchMap[key] = w
		}
	}

	if len(watchMap) == 0 {
		return nil
	}

	var alerts []BOMAlertItem
	for _, node := range tree.AllNodes {
		itemKey := strings.ToUpper(strings.TrimSpace(node.Item))
		if itemKey == "" {
			continue
		}

		if match, ok := watchMap[itemKey]; ok {
			desc := node.Description
			if desc == "" {
				desc = match.Description
			}
			alerts = append(alerts, BOMAlertItem{
				DeviceIP:    tree.DeviceIP,
				Slot:        node.Slot,
				Path:        node.Path,
				NodeType:    node.Type,
				NodeName:    node.Name,
				Item:        node.Item,
				BarCode:     node.BarCode,
				Description: desc,
				Severity:    match.Severity,
				BatchNo:     match.BatchNo,
			})
		}
	}

	return alerts
}

// GetInvolvedSlotsFromRaw 严格对齐 eDeskPro get_involved_elabel_slots 原生算法
// 按 [Slot_x] 切分并提取块内所有 Item，返回命中的槽位列表
func GetInvolvedSlotsFromRaw(msg string, items []string) []string {
	if strings.TrimSpace(msg) == "" || len(items) == 0 {
		return nil
	}

	itemSet := make(map[string]struct{}, len(items))
	for _, it := range items {
		itemSet[strings.ToUpper(strings.TrimSpace(it))] = struct{}{}
	}

	blocks := SplitBlocks(msg)
	var slotList []string
	seen := make(map[string]bool)

	for _, b := range blocks {
		if m := reSlotBlock.FindStringSubmatch(b.Header); len(m) > 1 {
			slotNum := NormalizeSlot(m[1])
			content := strings.Join(b.Lines, "\n")
			itemMatches := reItemInBlock.FindAllStringSubmatch(content, -1)

			hit := false
			for _, im := range itemMatches {
				if len(im) > 1 {
					itKey := strings.ToUpper(strings.TrimSpace(im[1]))
					if _, ok := itemSet[itKey]; ok {
						hit = true
						break
					}
				}
			}

			if hit && !seen[slotNum] {
				slotList = append(slotList, slotNum)
				seen[slotNum] = true
			}
		}
	}

	return slotList
}

// ExportBOMAlertsToCSV 将预警命中明细转换为 CSV 文本
func ExportBOMAlertsToCSV(alerts []BOMAlertItem) (string, error) {
	var buf bytes.Buffer
	// 写入 UTF-8 BOM，防止 Excel 打开中文乱码
	buf.WriteString("\xEF\xBB\xBF")

	w := csv.NewWriter(&buf)
	header := []string{
		"设备IP", "槽位", "物理路径", "节点类型", "部件名称", "BOM编码", "序列号/条形码", "风险级别", "预警批次号", "物料描述",
	}
	if err := w.Write(header); err != nil {
		return "", fmt.Errorf("写入 CSV 表头失败: %w", err)
	}

	for _, a := range alerts {
		row := []string{
			a.DeviceIP,
			a.Slot,
			a.Path,
			a.NodeType,
			a.NodeName,
			a.Item,
			a.BarCode,
			a.Severity,
			a.BatchNo,
			a.Description,
		}
		if err := w.Write(row); err != nil {
			return "", fmt.Errorf("写入 CSV 行失败: %w", err)
		}
	}

	w.Flush()
	return buf.String(), w.Error()
}
