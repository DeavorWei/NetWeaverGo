package taskexec

import (
	"testing"

	"github.com/NetWeaverGo/core/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTopologyCommandResolverResolveVendorPriority_Task(t *testing.T) {
	setupTopologyResolverConfigDB(t)

	resolver := NewTopologyCommandResolver()
	device := &models.DeviceAsset{Vendor: "cisco", DisplayName: "Cisco-N9K"}

	resolution, err := resolver.Resolve("h3c", device, nil)
	require.NoError(t, err)
	require.NotNil(t, resolution)

	assert.Equal(t, "h3c", resolution.ResolvedVendor)
	assert.Equal(t, TopologyVendorSourceTask, resolution.VendorSource)
}

func TestTopologyCommandResolverResolveVendorPriority_Inventory(t *testing.T) {
	setupTopologyResolverConfigDB(t)

	resolver := NewTopologyCommandResolver()
	device := &models.DeviceAsset{Vendor: "cisco", DisplayName: "edge-sw"}

	resolution, err := resolver.Resolve("", device, nil)
	require.NoError(t, err)
	require.NotNil(t, resolution)

	assert.Equal(t, "cisco", resolution.ResolvedVendor)
	assert.Equal(t, TopologyVendorSourceInventory, resolution.VendorSource)
}

func TestTopologyCommandResolverResolveVendorPriority_Detect(t *testing.T) {
	setupTopologyResolverConfigDB(t)

	resolver := NewTopologyCommandResolver()
	device := &models.DeviceAsset{
		Vendor:      "unknown-vendor",
		DisplayName: "Cisco core switch",
		Description: "distribution layer",
	}

	resolution, err := resolver.Resolve("", device, nil)
	require.NoError(t, err)
	require.NotNil(t, resolution)

	assert.Equal(t, "cisco", resolution.ResolvedVendor)
	assert.Equal(t, TopologyVendorSourceDetect, resolution.VendorSource)
}

func TestTopologyCommandResolverResolveVendorPriority_Fallback(t *testing.T) {
	setupTopologyResolverConfigDB(t)

	resolver := NewTopologyCommandResolver()
	device := &models.DeviceAsset{
		Vendor:      "unknown-vendor",
		DisplayName: "core-sw-01",
		Description: "aggregation node",
	}

	resolution, err := resolver.Resolve("", device, nil)
	require.NoError(t, err)
	require.NotNil(t, resolution)

	assert.Equal(t, "huawei", resolution.ResolvedVendor)
	assert.Equal(t, TopologyVendorSourceFallback, resolution.VendorSource)
}
