package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/NetWeaverGo/core/internal/inspection"
	"github.com/NetWeaverGo/core/internal/models"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func setupInspectionTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	require.NoError(t, err)

	err = db.AutoMigrate(
		&models.InspectionTemplate{},
		&models.InspectionItem{},
		&models.InspectionResult{},
	)
	require.NoError(t, err)

	return db
}

func TestInspectionService_TemplateCRUD(t *testing.T) {
	db := setupInspectionTestDB(t)
	svc := NewInspectionService(db, nil)

	// 1. Save new template
	tpl := models.InspectionTemplate{
		Name:        "测试巡检模板",
		Description: "单元测试用途",
		Vendor:      "huawei",
	}
	err := svc.SaveInspectionTemplate(tpl)
	require.NoError(t, err)

	// 2. List templates
	templates, err := svc.ListInspectionTemplates()
	require.NoError(t, err)
	require.Len(t, templates, 1)
	assert.Equal(t, "测试巡检模板", templates[0].Name)
	tplID := templates[0].ID
	assert.NotEmpty(t, tplID)

	// 3. Get template
	gotTpl, err := svc.GetInspectionTemplate(tplID)
	require.NoError(t, err)
	assert.Equal(t, "测试巡检模板", gotTpl.Name)

	// 4. Update template
	gotTpl.Description = "更新后的描述"
	err = svc.SaveInspectionTemplate(*gotTpl)
	require.NoError(t, err)

	updatedTpl, err := svc.GetInspectionTemplate(tplID)
	require.NoError(t, err)
	assert.Equal(t, "更新后的描述", updatedTpl.Description)

	// 5. Delete template
	err = svc.DeleteInspectionTemplate(tplID)
	require.NoError(t, err)

	templatesAfter, err := svc.ListInspectionTemplates()
	require.NoError(t, err)
	assert.Empty(t, templatesAfter)
}

func TestInspectionService_ItemCRUD(t *testing.T) {
	db := setupInspectionTestDB(t)
	svc := NewInspectionService(db, nil)

	// 1. Save new item
	item := models.InspectionItem{
		TemplateID:     "tpl-huawei-ce",
		Code:           "CHECK_CPU",
		Name:           "CPU利用率检查",
		Category:       "performance",
		Severity:       "major",
		CommandKey:     "display cpu-usage",
		IsPreCollect:   true,
		Order:          1,
		Enabled:        true,
		CheckType:      "threshold",
		Field:          "cpu_usage",
		ThresholdsJSON: `[{"rangeType":"bound","maxValue":"80"}]`,
	}
	err := svc.SaveInspectionItem(item)
	require.NoError(t, err)

	// 2. List items
	items, err := svc.ListInspectionItems("tpl-huawei-ce")
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "CHECK_CPU", items[0].Code)
	itemID := items[0].ID

	// 3. Toggle enabled
	err = svc.ToggleInspectionItemEnabled(itemID, false)
	require.NoError(t, err)

	items, err = svc.ListInspectionItems("tpl-huawei-ce")
	require.NoError(t, err)
	assert.False(t, items[0].Enabled)

	// 4. Delete item
	err = svc.DeleteInspectionItem(itemID)
	require.NoError(t, err)

	itemsAfter, err := svc.ListInspectionItems("tpl-huawei-ce")
	require.NoError(t, err)
	assert.Empty(t, itemsAfter)
}

func TestInspectionService_ResultsAndSummary(t *testing.T) {
	db := setupInspectionTestDB(t)
	svc := NewInspectionService(db, nil)

	// 插入巡检结果
	now := time.Now()
	res1 := models.InspectionResult{
		RunID:       "run-100",
		DeviceIP:    "10.0.0.1",
		ItemCode:    "CHECK_CPU",
		ItemName:    "CPU利用率",
		Category:    "performance",
		Status:      string(inspection.ResultPass),
		Severity:    string(inspection.SeverityMajor),
		CreatedAt:   now,
	}
	res2 := models.InspectionResult{
		RunID:       "run-100",
		DeviceIP:    "10.0.0.1",
		ItemCode:    "CHECK_FAN",
		ItemName:    "风扇状态",
		Category:    "environment",
		Status:      string(inspection.ResultFail),
		Severity:    string(inspection.SeverityBlocker),
		Advice:      "检查风扇模块并更换坏件",
		CreatedAt:   now,
	}
	res3 := models.InspectionResult{
		RunID:       "run-100",
		DeviceIP:    "10.0.0.2",
		ItemCode:    "CHECK_MEM",
		ItemName:    "内存利用率",
		Category:    "performance",
		Status:      string(inspection.ResultWarning),
		Severity:    string(inspection.SeverityMinor),
		CreatedAt:   now,
	}
	require.NoError(t, db.Create(&res1).Error)
	require.NoError(t, db.Create(&res2).Error)
	require.NoError(t, db.Create(&res3).Error)

	// 1. 查询结果并过滤
	results, err := svc.GetInspectionResults("run-100", "", "", "")
	require.NoError(t, err)
	assert.Len(t, results, 3)

	failResults, err := svc.GetInspectionResults("run-100", "", string(inspection.ResultFail), "")
	require.NoError(t, err)
	assert.Len(t, failResults, 1)
	assert.Equal(t, "CHECK_FAN", failResults[0].ItemCode)

	// 2. 汇总统计
	summary, err := svc.GetInspectionSummary("run-100")
	require.NoError(t, err)
	assert.Equal(t, 2, summary.TotalDevices)
	assert.Equal(t, 1, summary.FailDevices) // 10.0.0.1 有 fail 项
	assert.Equal(t, 1, summary.WarnDevices) // 10.0.0.2 只有 warn 项
	assert.Equal(t, 3, summary.TotalItems)
	assert.Equal(t, 1, summary.PassItems)
	assert.Equal(t, 1, summary.FailItems)
	assert.Equal(t, 1, summary.WarnItems)
	assert.Equal(t, 0, summary.IgnoreItems)
	assert.Equal(t, 0, summary.ManualItems)
	assert.Equal(t, 0, summary.UntestItems)
	assert.Equal(t, 0, summary.UnaccordItems)
	assert.Equal(t, 0, summary.ExceptItems)
	assert.Equal(t, 1, summary.BlockerCount)
	assert.Equal(t, 1, summary.MajorCount)
	assert.Equal(t, 1, summary.MinorCount)

	// 3. 导出 CSV
	csvText, err := svc.ExportInspectionCSV("run-100")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(csvText, "\xEF\xBB\xBF"))
	assert.Contains(t, csvText, "CHECK_FAN")
	assert.Contains(t, csvText, "检查风扇模块并更换坏件")

	// 4. 导出 JSON
	jsonText, err := svc.ExportInspectionJSON("run-100")
	require.NoError(t, err)
	assert.Contains(t, jsonText, `"itemCode": "CHECK_FAN"`)
}

func TestInspectionService_MultiRunIsolationAnd8States(t *testing.T) {
	db := setupInspectionTestDB(t)
	svc := NewInspectionService(db, nil)

	t1 := time.Now().Add(-10 * time.Minute)
	t2 := time.Now()

	// Run 1: 包含 pass, fail, ignore
	require.NoError(t, db.Create(&models.InspectionResult{
		RunID: "run-1", DeviceIP: "192.168.1.1", ItemCode: "ITEM_1", Status: string(inspection.ResultPass), CreatedAt: t1,
	}).Error)
	require.NoError(t, db.Create(&models.InspectionResult{
		RunID: "run-1", DeviceIP: "192.168.1.1", ItemCode: "ITEM_2", Status: string(inspection.ResultFail), CreatedAt: t1,
	}).Error)
	require.NoError(t, db.Create(&models.InspectionResult{
		RunID: "run-1", DeviceIP: "192.168.1.2", ItemCode: "ITEM_3", Status: string(inspection.ResultIgnore), CreatedAt: t1,
	}).Error)

	// Run 2 (最新): 包含 manual, untest, unaccord, except
	require.NoError(t, db.Create(&models.InspectionResult{
		RunID: "run-2", DeviceIP: "192.168.1.1", ItemCode: "ITEM_4", Status: string(inspection.ResultManual), CreatedAt: t2,
	}).Error)
	require.NoError(t, db.Create(&models.InspectionResult{
		RunID: "run-2", DeviceIP: "192.168.1.1", ItemCode: "ITEM_5", Status: string(inspection.ResultUntest), CreatedAt: t2,
	}).Error)
	require.NoError(t, db.Create(&models.InspectionResult{
		RunID: "run-2", DeviceIP: "192.168.1.2", ItemCode: "ITEM_6", Status: string(inspection.ResultUnaccord), CreatedAt: t2,
	}).Error)
	require.NoError(t, db.Create(&models.InspectionResult{
		RunID: "run-2", DeviceIP: "192.168.1.2", ItemCode: "ITEM_7", Status: string(inspection.ResultExcept), CreatedAt: t2,
	}).Error)

	// 明确指定 run-1: 仅能查到 run-1 的 3 条记录
	resRun1, err := svc.GetInspectionResults("run-1", "", "", "")
	require.NoError(t, err)
	assert.Len(t, resRun1, 3)

	sumRun1, err := svc.GetInspectionSummary("run-1")
	require.NoError(t, err)
	assert.Equal(t, 3, sumRun1.TotalItems)
	assert.Equal(t, 1, sumRun1.PassItems)
	assert.Equal(t, 1, sumRun1.FailItems)
	assert.Equal(t, 1, sumRun1.IgnoreItems)

	// 未指定 runID (""): 自动解析为最新 run-2，不混杂 run-1 的历史记录
	resLatest, err := svc.GetInspectionResults("", "", "", "")
	require.NoError(t, err)
	assert.Len(t, resLatest, 4)
	for _, r := range resLatest {
		assert.Equal(t, "run-2", r.RunID)
	}

	sumLatest, err := svc.GetInspectionSummary("")
	require.NoError(t, err)
	assert.Equal(t, 4, sumLatest.TotalItems)
	assert.Equal(t, 1, sumLatest.ManualItems)
	assert.Equal(t, 1, sumLatest.UntestItems)
	assert.Equal(t, 1, sumLatest.UnaccordItems)
	assert.Equal(t, 1, sumLatest.ExceptItems)
	assert.Equal(t, 0, sumLatest.PassDevices, "无任何测试通过的设备不得计入 PassDevices")

	// Run 3: 纯 Except 设备与纯 Pass 设备健康度聚合测试
	t3 := time.Now().Add(1 * time.Minute)
	require.NoError(t, db.Create(&models.InspectionResult{
		RunID: "run-3", DeviceIP: "10.0.0.1", ItemCode: "EX1", Status: string(inspection.ResultExcept), CreatedAt: t3,
	}).Error)
	require.NoError(t, db.Create(&models.InspectionResult{
		RunID: "run-3", DeviceIP: "10.0.0.1", ItemCode: "EX2", Status: string(inspection.ResultExcept), CreatedAt: t3,
	}).Error)
	require.NoError(t, db.Create(&models.InspectionResult{
		RunID: "run-3", DeviceIP: "10.0.0.2", ItemCode: "OK1", Status: string(inspection.ResultPass), CreatedAt: t3,
	}).Error)

	sumRun3, err := svc.GetInspectionSummary("run-3")
	require.NoError(t, err)
	assert.Equal(t, 2, sumRun3.TotalDevices)
	assert.Equal(t, 1, sumRun3.PassDevices, "10.0.0.2 纯正常应计为 PassDevices")
	assert.Equal(t, 1, sumRun3.ExceptDevices, "10.0.0.1 全异常设备应计为 ExceptDevices，严禁归入 PassDevices")
	assert.Equal(t, 0, sumRun3.FailDevices)
}
