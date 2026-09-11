package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/NetWeaverGo/core/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupHardwareTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	err = db.AutoMigrate(
		&models.DeviceAsset{},
		&models.TaskCEASNode{},
		&models.BOMWatchlistItem{},
	)
	require.NoError(t, err)

	return db
}

func TestHardwareInventoryService_ListCEASDevices(t *testing.T) {
	db := setupHardwareTestDB(t)
	svc := NewHardwareInventoryService(db, nil)

	// 插入测试设备
	db.Create(&models.DeviceAsset{
		IP:          "192.168.1.1",
		Group:       "Core",
		Vendor:      "huawei",
		Model:       "CE6800",
		ModelSeries: "CE6800",
		ESN:         "ESN-001",
	})
	db.Create(&models.DeviceAsset{
		IP:     "192.168.1.2",
		Group:  "Access",
		Vendor: "huawei",
		Model:  "S5700",
	})

	// 插入硬件节点
	now := time.Now()
	db.Create(&models.TaskCEASNode{
		TaskRunID: "run-1",
		DeviceIP:  "192.168.1.1",
		NodeID:    "0_1",
		Level:     1,
		Type:      "slot",
		Name:      "Slot_1",
		Item:      "02311UHE",
		CreatedAt: now,
	})

	// 插入预警 Watchlist
	db.Create(&models.BOMWatchlistItem{
		Item:     "02311UHE",
		Severity: "critical",
		Enabled:  true,
	})

	devices, err := svc.ListCEASDevices()
	require.NoError(t, err)
	require.Len(t, devices, 2)

	devMap := make(map[string]CEASDeviceOverviewVO)
	for _, d := range devices {
		devMap[d.IP] = d
	}

	d1 := devMap["192.168.1.1"]
	assert.True(t, d1.HasCEASData)
	assert.Equal(t, 1, d1.NodeCount)
	assert.Equal(t, 1, d1.AlertCount)

	d2 := devMap["192.168.1.2"]
	assert.False(t, d2.HasCEASData)
	assert.Equal(t, 0, d2.NodeCount)
	assert.Equal(t, 0, d2.AlertCount)
}

func TestHardwareInventoryService_GetHardwareTree(t *testing.T) {
	db := setupHardwareTestDB(t)
	svc := NewHardwareInventoryService(db, nil)

	db.Create(&models.DeviceAsset{
		IP:  "10.0.0.1",
		ESN: "CHASSIS-ESN-999",
	})

	// 构造父子层级：0 (frame) -> 0_1 (slot) -> 0_1_1 (card)
	db.Create(&models.TaskCEASNode{
		DeviceIP:  "10.0.0.1",
		NodeID:    "0",
		ParentID:  "",
		Level:     1,
		Type:      "frame",
		Name:      "Frame_0",
		AttrsJSON: `{"Cabinet":"Cab-A"}`,
	})
	db.Create(&models.TaskCEASNode{
		DeviceIP: "10.0.0.1",
		NodeID:   "0_1",
		ParentID: "0",
		Level:    2,
		Type:     "slot",
		Name:     "Slot_1",
		Slot:     "1",
		Path:     "1/0",
	})
	db.Create(&models.TaskCEASNode{
		DeviceIP:    "10.0.0.1",
		NodeID:      "0_1_1",
		ParentID:    "0_1",
		Level:       3,
		Type:        "card",
		Name:        "Card_1",
		Slot:        "1",
		Path:        "1/1",
		Item:        "02311ABC",
		BoardType:   "CE-MPU",
		Description: "Main Processing Unit",
	})

	tree, err := svc.GetHardwareTree("10.0.0.1")
	require.NoError(t, err)
	require.NotNil(t, tree)
	assert.Equal(t, "10.0.0.1", tree.DeviceIP)
	assert.Equal(t, "CHASSIS-ESN-999", tree.ChassisESN)
	assert.Equal(t, 3, tree.TotalNodes)
	require.Len(t, tree.Roots, 1)

	root := tree.Roots[0]
	assert.Equal(t, "0", root.ID)
	assert.Equal(t, "Cab-A", root.Attrs["Cabinet"])
	require.Len(t, root.Children, 1)

	slot := root.Children[0]
	assert.Equal(t, "0_1", slot.ID)
	assert.Equal(t, "Slot_1", slot.Name)
	require.Len(t, slot.Children, 1)

	card := slot.Children[0]
	assert.Equal(t, "0_1_1", card.ID)
	assert.Equal(t, "02311ABC", card.Item)
	assert.Equal(t, "CE-MPU", card.BoardType)
}

func TestHardwareInventoryService_GetHardwareNodeChildren(t *testing.T) {
	db := setupHardwareTestDB(t)
	svc := NewHardwareInventoryService(db, nil)

	db.Create(&models.TaskCEASNode{
		DeviceIP: "10.0.0.1",
		NodeID:   "0",
		ParentID: "",
		Level:    1,
		Type:     "frame",
		Name:     "Frame_0",
	})
	db.Create(&models.TaskCEASNode{
		DeviceIP: "10.0.0.1",
		NodeID:   "0_1",
		ParentID: "0",
		Level:    2,
		Type:     "slot",
		Name:     "Slot_1",
	})

	// 获取根节点
	roots, err := svc.GetHardwareNodeChildren("10.0.0.1", "")
	require.NoError(t, err)
	require.Len(t, roots, 1)
	assert.Equal(t, "0", roots[0].ID)

	// 获取子节点
	children, err := svc.GetHardwareNodeChildren("10.0.0.1", "0")
	require.NoError(t, err)
	require.Len(t, children, 1)
	assert.Equal(t, "0_1", children[0].ID)
}

func TestHardwareInventoryService_BOMWatchlistCRUD(t *testing.T) {
	db := setupHardwareTestDB(t)
	svc := NewHardwareInventoryService(db, nil)

	// 1. 新增
	err := svc.SaveBOMWatchlistItem(models.BOMWatchlistItem{
		Item:        "02311XYZ",
		Category:    "user_custom",
		Description: "测试预警物料",
		Severity:    "danger",
		Enabled:     true,
		BatchNo:     "BATCH-2026",
	})
	require.NoError(t, err)

	items, err := svc.ListBOMWatchlist()
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "02311XYZ", items[0].Item)
	assert.Equal(t, "danger", items[0].Severity)
	itemID := items[0].ID

	// 2. 切换启用状态
	err = svc.ToggleBOMWatchlistEnabled(itemID, false)
	require.NoError(t, err)

	items, _ = svc.ListBOMWatchlist()
	assert.False(t, items[0].Enabled)

	// 3. 更新
	err = svc.SaveBOMWatchlistItem(models.BOMWatchlistItem{
		ID:          itemID,
		Item:        "02311XYZ",
		Description: "更新后的描述",
		Severity:    "critical",
		Enabled:     true,
	})
	require.NoError(t, err)

	items, _ = svc.ListBOMWatchlist()
	assert.Equal(t, "critical", items[0].Severity)
	assert.Equal(t, "更新后的描述", items[0].Description)

	// 4. 删除
	err = svc.DeleteBOMWatchlistItem(itemID)
	require.NoError(t, err)

	items, _ = svc.ListBOMWatchlist()
	assert.Empty(t, items)
}

func TestHardwareInventoryService_BOMAlertsAndExport(t *testing.T) {
	db := setupHardwareTestDB(t)
	svc := NewHardwareInventoryService(db, nil)

	// 预警规则
	db.Create(&models.BOMWatchlistItem{
		Item:        "02311UHE",
		Severity:    "critical",
		Description: "电容鼓包风险",
		Enabled:     true,
		BatchNo:     "PCN-2026-001",
	})
	db.Create(&models.BOMWatchlistItem{
		Item:        "02312ABC",
		Severity:    "warning",
		Description: "普通建议更换",
		Enabled:     true,
		BatchNo:     "PCN-2026-002",
	})

	// 节点数据
	db.Create(&models.TaskCEASNode{
		DeviceIP:    "10.1.1.1",
		NodeID:      "0_1",
		Slot:        "1",
		Path:        "1/0",
		Type:        "slot",
		Name:        "Slot_1",
		Item:        "02311UHE",
		BarCode:     "2102311UHE1001",
		BoardType:   "CE6800",
		Description: "Main Board",
	})
	db.Create(&models.TaskCEASNode{
		DeviceIP: "10.1.1.2",
		NodeID:   "0_2",
		Slot:     "2",
		Path:     "2/0",
		Type:     "slot",
		Name:     "Slot_2",
		Item:     "02312ABC",
		BarCode:  "2102312ABC1002",
	})

	// 1. 查询全部预警
	alerts, err := svc.ListBOMAlerts("", "")
	require.NoError(t, err)
	require.Len(t, alerts, 2)
	assert.Equal(t, "critical", alerts[0].Severity)
	assert.Equal(t, "02311UHE", alerts[0].Item)
	assert.Equal(t, "warning", alerts[1].Severity)

	// 2. 按严重度过滤
	critAlerts, err := svc.ListBOMAlerts("", "critical")
	require.NoError(t, err)
	require.Len(t, critAlerts, 1)
	assert.Equal(t, "10.1.1.1", critAlerts[0].DeviceIP)

	// 3. 导出预警 CSV
	csvStr, err := svc.ExportBOMAlertsCSV("", "")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(csvStr, "\xEF\xBB\xBF"))
	assert.Contains(t, csvStr, "10.1.1.1")
	assert.Contains(t, csvStr, "02311UHE")
	assert.Contains(t, csvStr, "PCN-2026-001")

	// 4. 导出设备硬件清单 CSV
	invCsv, err := svc.ExportHardwareInventoryCSV("10.1.1.1")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(invCsv, "\xEF\xBB\xBF"))
	assert.Contains(t, invCsv, "10.1.1.1")
	assert.Contains(t, invCsv, "Slot_1")
	assert.Contains(t, invCsv, "02311UHE")
	assert.Contains(t, invCsv, "CE6800")
}

func TestHardwareInventoryService_TriggerCEASCollect_Errors(t *testing.T) {
	db := setupHardwareTestDB(t)
	svc := NewHardwareInventoryService(db, nil)

	// 1. 空设备
	_, err := svc.TriggerCEASCollect([]string{" ", ""})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "至少")

	// 2. 未初始化任务执行服务
	_, err = svc.TriggerCEASCollect([]string{"10.0.0.1"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "未初始化")
}
