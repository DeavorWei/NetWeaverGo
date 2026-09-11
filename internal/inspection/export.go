package inspection

import (
	"bytes"
	"encoding/csv"
	"encoding/json"

	"github.com/NetWeaverGo/core/internal/models"
)

// ExportInspectionResultsCSV 导出巡检明细列表为 CSV 文本（带 UTF-8 BOM 防 Excel 乱码）
func ExportInspectionResultsCSV(results []models.InspectionResult) (string, error) {
	var buf bytes.Buffer
	// 写入 UTF-8 BOM
	buf.WriteString("\xEF\xBB\xBF")

	writer := csv.NewWriter(&buf)
	headers := []string{
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
	if err := writer.Write(headers); err != nil {
		return "", err
	}

	for _, r := range results {
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

// ExportInspectionResultsJSON 导出巡检明细为格式化 JSON 文本
func ExportInspectionResultsJSON(results []models.InspectionResult) (string, error) {
	b, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}
