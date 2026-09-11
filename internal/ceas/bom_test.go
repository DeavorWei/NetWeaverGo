package ceas

import (
	"strings"
	"testing"

	"github.com/NetWeaverGo/core/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetInvolvedSlots(t *testing.T) {
	tree := &HardwareTree{
		DeviceIP: "192.168.1.1",
		AllNodes: []*Node{
			{
				Slot:        "1",
				Path:        "1/0",
				Type:        "slot",
				Name:        "Slot_1",
				Item:        "02311UHE",
				BarCode:     "BC001",
				Description: "CE Main Card",
			},
			{
				Slot:        "2",
				Path:        "2/0",
				Type:        "slot",
				Name:        "Slot_2",
				Item:        "02352BBR",
				BarCode:     "BC002",
				Description: "Safe Card",
			},
		},
	}

	watchlist := []models.BOMWatchlistItem{
		{
			Item:        "02311UHE",
			Severity:    "critical",
			Description: "电容隐患",
			Enabled:     true,
			BatchNo:     "PCN-001",
		},
		{
			Item:        "02352BBR",
			Severity:    "warning",
			Description: "已禁用规则",
			Enabled:     false,
		},
	}

	alerts := GetInvolvedSlots(tree, watchlist)
	require.Equal(t, 1, len(alerts))
	assert.Equal(t, "192.168.1.1", alerts[0].DeviceIP)
	assert.Equal(t, "1", alerts[0].Slot)
	assert.Equal(t, "02311UHE", alerts[0].Item)
	assert.Equal(t, "critical", alerts[0].Severity)
}

func TestGetInvolvedSlotsFromRaw(t *testing.T) {
	raw := `[Slot_1]
BoardType=CE6800
Item=02311UHE

[Slot_2]
BoardType=CE6800
Item=02312DWR

[Slot_3]
BoardType=CE6800
Item=99999999
`
	items := []string{"02311UHE", "02312DWR"}
	slots := GetInvolvedSlotsFromRaw(raw, items)
	assert.Equal(t, []string{"1", "2"}, slots)
}

func TestExportBOMAlertsToCSV(t *testing.T) {
	alerts := []BOMAlertItem{
		{
			DeviceIP:    "10.1.1.1",
			Slot:        "1",
			Path:        "1/0",
			NodeType:    "slot",
			NodeName:    "Slot_1",
			Item:        "02311UHE",
			BarCode:     "2102311UHE10001",
			Severity:    "critical",
			BatchNo:     "PCN-2023-01",
			Description: "批次预警测试",
		},
	}

	csvStr, err := ExportBOMAlertsToCSV(alerts)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(csvStr, "\xEF\xBB\xBF"), "必须包含 UTF-8 BOM")
	assert.Contains(t, csvStr, "10.1.1.1")
	assert.Contains(t, csvStr, "02311UHE")
	assert.Contains(t, csvStr, "PCN-2023-01")
}
