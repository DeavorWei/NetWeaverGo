package inspection

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"strings"

	"github.com/NetWeaverGo/core/internal/models"
)

// 巡检报表 CSV 表头（中文基准 / 英文）
var (
	inspectionCSVHeadersZh = []string{
		"设备IP",
		"检查项编码",
		"检查项名称",
		"分类",
		"测试结果状态",
		"严重度",
		"实际采集值",
		"阈值规则/判定基准",
		"异常问题描述",
		"专家修复建议",
		"举证追踪与上下文 (Evidence)",
		"巡检时间",
	}
	inspectionCSVHeadersEn = []string{
		"Device IP",
		"Item Code",
		"Item Name",
		"Category",
		"Status",
		"Severity",
		"Actual Value",
		"Threshold / Baseline",
		"Problem",
		"Advice",
		"Evidence",
		"Inspected At",
	}
)

// isChineseLocale 判断 locale 是否属于中文；空值视为中文，保持历史行为。
func isChineseLocale(locale string) bool {
	l := strings.ToLower(strings.TrimSpace(locale))
	return l == "" || strings.HasPrefix(l, "zh")
}

// csvHeadersForLocale 按 locale 选择 CSV 表头
func csvHeadersForLocale(locale string) []string {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(locale)), "en") {
		return inspectionCSVHeadersEn
	}
	return inspectionCSVHeadersZh
}

// localizeResults 生成按 locale 本地化后的结果副本（名称/建议取自文案表）。
// 中文 locale、无文案表或未命中文案的条目一律回退原始中文内容。
func localizeResults(results []models.InspectionResult, locale string, texts map[string]models.InspectionItemText) []models.InspectionResult {
	if isChineseLocale(locale) || len(texts) == 0 {
		return results
	}
	out := make([]models.InspectionResult, len(results))
	copy(out, results)
	for i := range out {
		t, ok := texts[out[i].ItemCode]
		if !ok {
			continue
		}
		if t.Name != "" {
			out[i].ItemName = t.Name
		}
		if t.Advice != "" {
			out[i].Advice = t.Advice
		}
	}
	return out
}

// ExportInspectionResultsCSV 导出巡检明细列表为 CSV 文本（默认中文，带 UTF-8 BOM 防 Excel 乱码）
func ExportInspectionResultsCSV(results []models.InspectionResult) (string, error) {
	return ExportInspectionResultsCSVWithLocale(results, "", nil)
}

// ExportInspectionResultsCSVWithLocale 按 locale 导出巡检明细 CSV。
// texts 为 (itemCode → 文案) 映射，仅用于非中文 locale；未命中的条目回退原文。
func ExportInspectionResultsCSVWithLocale(results []models.InspectionResult, locale string, texts map[string]models.InspectionItemText) (string, error) {
	localized := localizeResults(results, locale, texts)

	var buf bytes.Buffer
	// 写入 UTF-8 BOM
	buf.WriteString("\xEF\xBB\xBF")

	writer := csv.NewWriter(&buf)
	if err := writer.Write(csvHeadersForLocale(locale)); err != nil {
		return "", err
	}

	for _, r := range localized {
		row := []string{
			r.DeviceIP,
			r.ItemCode,
			r.ItemName,
			r.Category,
			r.Status,
			r.Severity,
			r.ActualValue,
			r.ThresholdHit,
			r.Problem,
			r.Advice,
			r.EvidenceJSON,
			r.CreatedAt.Format("2006-01-02 15:04:05"),
		}
		if err := writer.Write(row); err != nil {
			return "", err
		}
	}
	writer.Flush()
	return buf.String(), nil
}

// ExportInspectionResultsJSON 导出巡检明细为格式化 JSON 文本（默认中文）
func ExportInspectionResultsJSON(results []models.InspectionResult) (string, error) {
	return ExportInspectionResultsJSONWithLocale(results, "", nil)
}

// ExportInspectionResultsJSONWithLocale 按 locale 导出巡检明细 JSON；语义与 CSV 版本一致。
func ExportInspectionResultsJSONWithLocale(results []models.InspectionResult, locale string, texts map[string]models.InspectionItemText) (string, error) {
	localized := localizeResults(results, locale, texts)
	b, err := json.MarshalIndent(localized, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}
