package plancompare

import (
	"os"
	"strings"
	"testing"

	"github.com/NetWeaverGo/core/internal/models"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestParsePlanRowsDeduplicateUndirectedLink(t *testing.T) {
	rows := [][]string{
		{"本端设备名", "本端管理IP", "本端接口", "对端设备名", "对端管理IP", "对端接口", "链路类型", "备注"},
		{"Core1", "10.0.0.1", "GigabitEthernet1/0/1", "Agg1", "10.0.0.2", "Eth-Trunk10", "aggregate", ""},
		{"Agg1", "10.0.0.2", "Eth-Trunk10", "Core1", "10.0.0.1", "GE1/0/1", "aggregate", "反向重复"},
	}

	links, warnings, err := parsePlanRows(rows)
	if err != nil {
		t.Fatalf("parse rows failed: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("expected deduplicated 1 link, got %d", len(links))
	}
	if len(warnings) == 0 {
		t.Fatalf("expected duplicate warning")
	}
	if links[0].NormAIf == "" || links[0].NormBIf == "" || links[0].EdgeKey == "" {
		t.Fatalf("expected normalized interfaces and edge key, got %+v", links[0])
	}
}

func TestCompareMatchedLink(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db failed: %v", err)
	}
	if err := db.AutoMigrate(
		&models.DiscoveryDevice{},
		&models.TopologyEdge{},
		&models.PlanFile{},
		&models.PlannedLink{},
		&models.DiffReport{},
		&models.DiffItem{},
	); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}

	plan := models.PlanFile{FileName: "test.xlsx", FilePath: "test.xlsx", TotalLinks: 1}
	if err := db.Create(&plan).Error; err != nil {
		t.Fatalf("create plan failed: %v", err)
	}
	planID := "1"
	if err := db.Create(&models.PlannedLink{
		PlanFileID:  planID,
		ADeviceName: "Core1",
		AMgmtIP:     "10.0.0.1",
		AIf:         "GE1/0/1",
		BDeviceName: "Agg1",
		BMgmtIP:     "10.0.0.2",
		BIf:         "Trunk10",
		NormAIf:     "GE1/0/1",
		NormBIf:     "Trunk10",
		EdgeKey:     "10.0.0.1:GE1/0/1<->10.0.0.2:Trunk10",
	}).Error; err != nil {
		t.Fatalf("create planned link failed: %v", err)
	}

	if err := db.Create(&models.DiscoveryDevice{
		TaskID:         "task1",
		DeviceIP:       "10.0.0.1",
		MgmtIP:         "10.0.0.1",
		Hostname:       "Core1",
		NormalizedName: "CORE1",
		Status:         "success",
	}).Error; err != nil {
		t.Fatalf("create device1 failed: %v", err)
	}
	if err := db.Create(&models.DiscoveryDevice{
		TaskID:         "task1",
		DeviceIP:       "10.0.0.2",
		MgmtIP:         "10.0.0.2",
		Hostname:       "Agg1",
		NormalizedName: "AGG1",
		Status:         "success",
	}).Error; err != nil {
		t.Fatalf("create device2 failed: %v", err)
	}
	if err := db.Create(&models.TopologyEdge{
		ID:        "edge1",
		TaskID:    "task1",
		ADeviceID: "10.0.0.1",
		AIf:       "GE1/0/1",
		BDeviceID: "10.0.0.2",
		BIf:       "Trunk10",
		EdgeType:  "physical",
		Status:    "confirmed",
	}).Error; err != nil {
		t.Fatalf("create edge failed: %v", err)
	}

	svc := NewService(db)
	result, err := svc.Compare("task1", planID)
	if err != nil {
		t.Fatalf("compare failed: %v", err)
	}
	if result.Matched != 1 {
		t.Fatalf("expected matched=1, got %d", result.Matched)
	}
	if len(result.MissingLinks) != 0 || len(result.UnexpectedLinks) != 0 || len(result.InconsistentItems) != 0 {
		t.Fatalf("expected no diffs, got %+v", result)
	}
}

func TestExportDiffReportExcelAndHTML(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db failed: %v", err)
	}
	if err := db.AutoMigrate(
		&models.DiffReport{},
		&models.DiffItem{},
	); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}

	report := models.DiffReport{
		ID:                "report_test_export",
		TaskID:            "task1",
		PlanFileID:        "1",
		TotalPlanned:      1,
		TotalActual:       1,
		Matched:           0,
		MissingLinks:      1,
		UnexpectedLinks:   0,
		InconsistentItems: 0,
	}
	if err := db.Create(&report).Error; err != nil {
		t.Fatalf("create report failed: %v", err)
	}
	if err := db.Create(&models.DiffItem{
		ReportID:    report.ID,
		DiffType:    "missing_link",
		ADeviceName: "Core1",
		AIf:         "GE1/0/1",
		BDeviceName: "Agg1",
		BIf:         "Trunk10",
		Reason:      "missing",
	}).Error; err != nil {
		t.Fatalf("create diff item failed: %v", err)
	}

	svc := NewService(db)
	excelPath, err := svc.ExportDiffReport(report.ID, "excel")
	if err != nil {
		t.Fatalf("export excel failed: %v", err)
	}
	if !strings.HasSuffix(strings.ToLower(excelPath), ".xlsx") {
		t.Fatalf("expected .xlsx output, got %s", excelPath)
	}
	if _, err := os.Stat(excelPath); err != nil {
		t.Fatalf("expected excel file exists: %v", err)
	}

	htmlPath, err := svc.ExportDiffReport(report.ID, "html")
	if err != nil {
		t.Fatalf("export html failed: %v", err)
	}
	if !strings.HasSuffix(strings.ToLower(htmlPath), ".html") {
		t.Fatalf("expected .html output, got %s", htmlPath)
	}
	if _, err := os.Stat(htmlPath); err != nil {
		t.Fatalf("expected html file exists: %v", err)
	}
}

// TestCompare_MultipleLinks_SameDevicePair 测试规划1条链路但实际存在2条同设备对链路时，
// 额外链路必须被正确标记为 unexpected_link
func TestCompare_MultipleLinks_SameDevicePair(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db failed: %v", err)
	}
	if err := db.AutoMigrate(
		&models.DiscoveryDevice{},
		&models.TopologyEdge{},
		&models.PlanFile{},
		&models.PlannedLink{},
		&models.DiffReport{},
		&models.DiffItem{},
	); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}

	// 创建规划：1条链路
	plan := models.PlanFile{FileName: "test.xlsx", FilePath: "test.xlsx", TotalLinks: 1}
	if err := db.Create(&plan).Error; err != nil {
		t.Fatalf("create plan failed: %v", err)
	}
	planID := "1"

	// 规划链路：Core1 GE1/0/1 <-> Agg1 Trunk10
	if err := db.Create(&models.PlannedLink{
		PlanFileID:  planID,
		ADeviceName: "Core1",
		AMgmtIP:     "10.0.0.1",
		AIf:         "GE1/0/1",
		BDeviceName: "Agg1",
		BMgmtIP:     "10.0.0.2",
		BIf:         "Trunk10",
		NormAIf:     "GE1/0/1",
		NormBIf:     "Trunk10",
		EdgeKey:     "10.0.0.1:GE1/0/1<->10.0.0.2:Trunk10",
	}).Error; err != nil {
		t.Fatalf("create planned link failed: %v", err)
	}

	// 创建设备
	if err := db.Create(&models.DiscoveryDevice{
		TaskID:         "task1",
		DeviceIP:       "10.0.0.1",
		MgmtIP:         "10.0.0.1",
		Hostname:       "Core1",
		NormalizedName: "CORE1",
		Status:         "success",
	}).Error; err != nil {
		t.Fatalf("create device1 failed: %v", err)
	}
	if err := db.Create(&models.DiscoveryDevice{
		TaskID:         "task1",
		DeviceIP:       "10.0.0.2",
		MgmtIP:         "10.0.0.2",
		Hostname:       "Agg1",
		NormalizedName: "AGG1",
		Status:         "success",
	}).Error; err != nil {
		t.Fatalf("create device2 failed: %v", err)
	}

	// 实际拓扑：同设备对存在2条链路
	// 第一条：匹配规划的链路
	if err := db.Create(&models.TopologyEdge{
		ID:        "edge1",
		TaskID:    "task1",
		ADeviceID: "10.0.0.1",
		AIf:       "GE1/0/1",
		BDeviceID: "10.0.0.2",
		BIf:       "Trunk10",
		EdgeType:  "physical",
		Status:    "confirmed",
	}).Error; err != nil {
		t.Fatalf("create edge1 failed: %v", err)
	}
	// 第二条：同设备对的额外链路（GE1/0/2 <-> Trunk20）
	if err := db.Create(&models.TopologyEdge{
		ID:        "edge2",
		TaskID:    "task1",
		ADeviceID: "10.0.0.1",
		AIf:       "GE1/0/2",
		BDeviceID: "10.0.0.2",
		BIf:       "Trunk20",
		EdgeType:  "physical",
		Status:    "confirmed",
	}).Error; err != nil {
		t.Fatalf("create edge2 failed: %v", err)
	}

	svc := NewService(db)
	result, err := svc.Compare("task1", planID)
	if err != nil {
		t.Fatalf("compare failed: %v", err)
	}

	// 验证：1条匹配，0条缺失，1条意外
	if result.Matched != 1 {
		t.Fatalf("expected matched=1, got %d", result.Matched)
	}
	if len(result.MissingLinks) != 0 {
		t.Fatalf("expected 0 missing links, got %d", len(result.MissingLinks))
	}
	if len(result.UnexpectedLinks) != 1 {
		t.Fatalf("expected 1 unexpected link, got %d", len(result.UnexpectedLinks))
	}

	// 验证意外链路的原因描述包含"设备对"关键词
	unexpected := result.UnexpectedLinks[0]
	if unexpected.ADeviceName != "10.0.0.1" || unexpected.BDeviceName != "10.0.0.2" {
		t.Errorf("expected unexpected link between 10.0.0.1 and 10.0.0.2, got %s <-> %s",
			unexpected.ADeviceName, unexpected.BDeviceName)
	}
	// 原因应包含"设备对"或"额外链路"等关键词
	if !strings.Contains(unexpected.Reason, "设备对") && !strings.Contains(unexpected.Reason, "额外链路") {
		t.Errorf("unexpected link reason should mention '设备对' or '额外链路', got: %s", unexpected.Reason)
	}
}
