package taskexec

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/models"
	"github.com/NetWeaverGo/core/internal/repository"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupTopologyResolverConfigDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=private", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.TopologyVendorFieldCommand{}))

	originalDB := config.DB
	config.DB = db
	t.Cleanup(func() {
		config.DB = originalDB
	})

	// 重置种子单例，避免跨测试污染
	topologyCommandSeedOnce = sync.Once{}
	topologyCommandSeedErr = nil

	return db
}

func TestCreateTaskDefinitionFromLaunchSpec_UsesTopologyVendorFallback(t *testing.T) {
	setupTopologyResolverConfigDB(t)

	service := &TaskExecutionService{}
	spec := &CanonicalLaunchSpec{
		TaskGroupID:       1001,
		TaskNameSnapshot:  "topology-task",
		RunKind:           string(RunKindTopology),
		TopologyVendor:    "h3c",
		Concurrency:       4,
		TimeoutSec:        180,
		EnableRawLog:      true,
		AutoBuildTopology: true,
		Topology: &CanonicalTopology{
			DeviceIDs: []uint{11, 12},
			DeviceIPs: []string{"10.10.10.11", "10.10.10.12"},
			Vendor:    "",
			FieldOverrides: []models.TopologyTaskFieldOverride{
				{FieldKey: "lldp_neighbor", Command: "display lldp neighbor verbose", TimeoutSec: 90},
			},
		},
	}

	def, err := service.CreateTaskDefinitionFromLaunchSpec(spec)
	require.NoError(t, err)
	require.NotNil(t, def)

	var cfg TopologyTaskConfig
	require.NoError(t, json.Unmarshal(def.Config, &cfg))
	assert.Equal(t, "h3c", cfg.Vendor)
	assert.Equal(t, []uint{11, 12}, cfg.DeviceIDs)
	assert.Equal(t, []string{"10.10.10.11", "10.10.10.12"}, cfg.DeviceIPs)
	require.Len(t, cfg.FieldOverrides, 1)
	assert.Equal(t, "lldp_neighbor", cfg.FieldOverrides[0].FieldKey)
	require.NotEmpty(t, cfg.ResolvedCommands)

	foundVendor := false
	for _, cmd := range cfg.ResolvedCommands {
		if cmd.ResolvedVendor == "h3c" {
			foundVendor = true
			break
		}
	}
	assert.True(t, foundVendor, "resolved commands should include h3c vendor context")
}

func TestCreateTaskDefinitionFromLaunchSpec_PrefersCanonicalTopologyVendor(t *testing.T) {
	setupTopologyResolverConfigDB(t)

	service := &TaskExecutionService{}
	spec := &CanonicalLaunchSpec{
		TaskGroupID:      1002,
		TaskNameSnapshot: "topology-task-explicit-vendor",
		RunKind:          string(RunKindTopology),
		TopologyVendor:   "h3c",
		Topology: &CanonicalTopology{
			DeviceIDs: []uint{21},
			DeviceIPs: []string{"10.20.20.21"},
			Vendor:    "cisco",
		},
	}

	def, err := service.CreateTaskDefinitionFromLaunchSpec(spec)
	require.NoError(t, err)
	require.NotNil(t, def)

	var cfg TopologyTaskConfig
	require.NoError(t, json.Unmarshal(def.Config, &cfg))
	assert.Equal(t, "cisco", cfg.Vendor)
	require.NotEmpty(t, cfg.ResolvedCommands)

	foundCisco := false
	for _, cmd := range cfg.ResolvedCommands {
		if cmd.ResolvedVendor == "cisco" {
			foundCisco = true
			break
		}
	}
	assert.True(t, foundCisco, "resolved commands should include cisco vendor context")
}

func TestNormalizeTaskGroup_PropagatesTopologyOverrides(t *testing.T) {
	mockRepo := repository.NewMockDeviceRepository()
	mockRepo.Devices[1] = models.DeviceAsset{ID: 1, IP: "192.168.100.1", Vendor: "huawei"}
	mockRepo.Devices[2] = models.DeviceAsset{ID: 2, IP: "192.168.100.2", Vendor: "huawei"}

	normalizer := &LaunchNormalizer{deviceRepo: mockRepo}
	enabled := true
	taskGroup := &models.TaskGroup{
		ID:             2001,
		Name:           "topology-group",
		TaskType:       string(RunKindTopology),
		TopologyVendor: "huawei",
		Items: []models.TaskItem{
			{DeviceIDs: []uint{2, 1, 1}},
		},
		TopologyFieldOverrides: []models.TopologyTaskFieldOverride{
			{FieldKey: "lldp_neighbor", Command: "display lldp neighbor verbose", TimeoutSec: 45, Enabled: &enabled},
		},
	}

	spec, err := normalizer.NormalizeTaskGroup(taskGroup)
	require.NoError(t, err)
	require.NotNil(t, spec)
	require.NotNil(t, spec.Topology)

	assert.Equal(t, []uint{1, 2}, spec.Topology.DeviceIDs)
	assert.Equal(t, []string{"192.168.100.1", "192.168.100.2"}, spec.Topology.DeviceIPs)
	assert.Equal(t, "huawei", spec.Topology.Vendor)
	require.Len(t, spec.Topology.FieldOverrides, 1)
	assert.Equal(t, "lldp_neighbor", spec.Topology.FieldOverrides[0].FieldKey)
	assert.Equal(t, 45, spec.Topology.FieldOverrides[0].TimeoutSec)
	require.NotNil(t, spec.Topology.FieldOverrides[0].Enabled)
	assert.True(t, *spec.Topology.FieldOverrides[0].Enabled)
}
