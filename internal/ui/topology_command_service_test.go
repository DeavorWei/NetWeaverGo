package ui

import (
	"sort"
	"testing"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/models"
	"github.com/NetWeaverGo/core/internal/repository"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupTopologyCommandServiceConfigDB(t *testing.T) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.TopologyVendorFieldCommand{}))

	originalDB := config.DB
	config.DB = db
	t.Cleanup(func() {
		config.DB = originalDB
	})
}

func TestTopologyCommandServiceGetTaskTopologyVendors_MergeUnion(t *testing.T) {
	repo := repository.NewMockDeviceRepository()
	repo.AddDevice(models.DeviceAsset{IP: "10.0.0.1", Vendor: "Juniper"})
	repo.AddDevice(models.DeviceAsset{IP: "10.0.0.2", Vendor: "huawei"})
	repo.AddDevice(models.DeviceAsset{IP: "10.0.0.3", Vendor: ""})

	svc := NewTopologyCommandService()
	svc.repo = repo

	vendors := svc.GetTaskTopologyVendors()
	require.NotEmpty(t, vendors)

	sorted := append([]string(nil), vendors...)
	sort.Strings(sorted)
	assert.Equal(t, sorted, vendors)

	assert.Contains(t, vendors, "huawei")
	assert.Contains(t, vendors, "h3c")
	assert.Contains(t, vendors, "cisco")
	assert.Contains(t, vendors, "juniper")
}

func TestTopologyCommandServicePreviewTopologyCommands_ReturnsNormalizedTaskOverrides(t *testing.T) {
	setupTopologyCommandServiceConfigDB(t)

	repo := repository.NewMockDeviceRepository()
	repo.AddDevice(models.DeviceAsset{ID: 1, IP: "10.1.1.1", Vendor: "huawei"})

	svc := NewTopologyCommandService()
	svc.repo = repo

	enabled := true
	preview, err := svc.PreviewTopologyCommands("", []uint{1}, []models.TopologyTaskFieldOverride{
		{FieldKey: " lldp_neighbor ", Command: " display lldp neighbor ", TimeoutSec: 60, Enabled: &enabled},
		{FieldKey: "", Command: "display version", TimeoutSec: 30},
		{FieldKey: "lldp_neighbor", Command: "display lldp", TimeoutSec: 30},
	})
	require.NoError(t, err)
	require.NotNil(t, preview)
	require.Len(t, preview.TaskOverrides, 1)

	override := preview.TaskOverrides[0]
	assert.Equal(t, "lldp_neighbor", override.FieldKey)
	assert.Equal(t, "display lldp neighbor", override.Command)
	assert.Equal(t, 60, override.TimeoutSec)
	require.NotNil(t, override.Enabled)
	assert.True(t, *override.Enabled)
}
