package taskexec

import (
	"testing"

	"github.com/NetWeaverGo/core/internal/models"
	"github.com/stretchr/testify/assert"
)

func TestCheckDeviceEligibility_BizCompare(t *testing.T) {
	// 受支持设备与版本
	supportedDev := &models.DeviceAsset{
		Model:   "CE6800",
		Version: "V200R019C00SPC800",
	}
	ok, reason := CheckDeviceEligibility(supportedDev, "bizcompare")
	assert.True(t, ok)
	assert.Empty(t, reason)

	// 不受支持的款型
	unsupportedDev := &models.DeviceAsset{
		Model:   "Cisco_Catalyst_2960",
		Version: "15.0",
	}
	ok2, reason2 := CheckDeviceEligibility(unsupportedDev, "bizcompare")
	assert.False(t, ok2)
	assert.NotEmpty(t, reason2)
}

func TestCheckDeviceEligibility_SoftwareFormFactor(t *testing.T) {
	softDev := &models.DeviceAsset{
		Model:      "CE1800V-SOFT",
		FormFactor: "software",
	}

	// 软件设备不允许 optical 弱光检查
	okOpt, reasonOpt := CheckDeviceEligibility(softDev, "optical")
	assert.False(t, okOpt)
	assert.Contains(t, reasonOpt, "软件形态设备")

	// 软件设备允许常规 inspection
	okInsp, _ := CheckDeviceEligibility(softDev, "inspection")
	assert.True(t, okInsp)
}
