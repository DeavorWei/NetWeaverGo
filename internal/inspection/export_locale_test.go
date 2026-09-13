package inspection

import (
	"strings"
	"testing"

	"github.com/NetWeaverGo/core/internal/models"
)

// 阶段二 2.2：默认导出保持中文行为（含 BOM 与中文表头）
func TestExportInspectionResults_DefaultIsChinese(t *testing.T) {
	results := []models.InspectionResult{
		{ItemCode: "GEN_CPU_USAGE", ItemName: "主控 CPU 使用率检查", Advice: "排查高 CPU 进程", Status: "TEST_PASS", Severity: "major"},
	}

	csvText, err := ExportInspectionResultsCSV(results)
	if err != nil {
		t.Fatalf("导出 CSV 失败: %v", err)
	}
	if !strings.HasPrefix(csvText, "\xEF\xBB\xBF") {
		t.Error("默认 CSV 应带 UTF-8 BOM")
	}
	if !strings.Contains(csvText, "设备IP") {
		t.Error("默认应为中文表头")
	}
	if !strings.Contains(csvText, "主控 CPU 使用率检查") {
		t.Error("默认应保留中文名称")
	}
}

// 阶段二 2.2：en-US 使用文案表并本地化表头，未命中文案的条目回退原文
func TestExportInspectionResults_EnglishLocale(t *testing.T) {
	results := []models.InspectionResult{
		{ItemCode: "GEN_CPU_USAGE", ItemName: "主控 CPU 使用率检查", Advice: "排查高 CPU 进程", Status: "TEST_PASS", Severity: "major"},
		{ItemCode: "UNKNOWN_CODE", ItemName: "未知检查项", Advice: "未知建议", Status: "TEST_PASS", Severity: "minor"},
	}
	texts := map[string]models.InspectionItemText{
		"GEN_CPU_USAGE": {Key: "GEN_CPU_USAGE", Locale: "en-US", Name: "CPU check", Advice: "Check CPU load"},
	}

	csvText, err := ExportInspectionResultsCSVWithLocale(results, "en-US", texts)
	if err != nil {
		t.Fatalf("导出 CSV 失败: %v", err)
	}
	if !strings.Contains(csvText, "Device IP") {
		t.Error("en-US 应使用英文表头")
	}
	if !strings.Contains(csvText, "CPU check") || !strings.Contains(csvText, "Check CPU load") {
		t.Error("命中文案的条目应输出英文名称与建议")
	}
	if !strings.Contains(csvText, "未知检查项") {
		t.Error("未命中文案的条目应回退原文")
	}

	jsonText, err := ExportInspectionResultsJSONWithLocale(results, "en-US", texts)
	if err != nil {
		t.Fatalf("导出 JSON 失败: %v", err)
	}
	if !strings.Contains(jsonText, "CPU check") {
		t.Error("en-US JSON 应输出英文名称")
	}
	if !strings.Contains(jsonText, "未知检查项") {
		t.Error("en-US JSON 未命中文案的条目应回退原文")
	}
}

// 阶段二 2.3：文案种子 Key 必须与内置检查项编码一一对应（防止"永不命中"回归）
func TestDefaultItemTexts_KeysMatchItemCodes(t *testing.T) {
	if len(DefaultItemTexts) < 12 {
		t.Fatalf("英文文案种子应覆盖全部内置检查项，当前仅 %d 条", len(DefaultItemTexts))
	}

	codes := make(map[string]struct{}, len(DefaultItems))
	for _, it := range DefaultItems {
		codes[it.Code] = struct{}{}
	}

	for _, txt := range DefaultItemTexts {
		if txt.Locale != "en-US" {
			t.Errorf("文案 %s 的 Locale = %q, want en-US", txt.Key, txt.Locale)
		}
		if _, ok := codes[txt.Key]; !ok {
			t.Errorf("文案 Key %q 在内置检查项编码中不存在", txt.Key)
		}
	}

	covered := make(map[string]struct{}, len(DefaultItemTexts))
	for _, txt := range DefaultItemTexts {
		covered[txt.Key] = struct{}{}
	}
	for _, it := range DefaultItems {
		if _, ok := covered[it.Code]; !ok {
			t.Errorf("内置检查项 %s 缺少 en-US 文案", it.Code)
		}
	}
}
