package taskexec

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestCompiler_DeviceEligibility_Unsupported(t *testing.T) {
	db := setupTestDB(t)
	config.DB = db
	require.NoError(t, db.AutoMigrate(&models.DeviceAsset{}))

	dev := models.DeviceAsset{
		IP:         "10.254.1.1",
		Vendor:     "cisco",
		Model:      "Cisco_Catalyst_2960",
		Version:    "15.0",
		FormFactor: "hardware",
	}
	require.NoError(t, db.Create(&dev).Error)

	compiler := NewBizCompareTaskCompiler(nil)
	cfg := &BizCompareTaskConfig{
		DeviceIPs: []string{"10.254.1.1"},
		Domain:    "S",
		SceneID:   "default",
		Phase:     "before",
	}
	cfgBytes, _ := json.Marshal(cfg)
	def := &TaskDefinition{
		ID:     "test-bizcompare-eligibility",
		Name:   "Test BizCompare Eligibility",
		Kind:   string(RunKindBizCompare),
		Config: cfgBytes,
	}

	plan, err := compiler.Compile(context.Background(), def)
	require.NoError(t, err)
	require.NotNil(t, plan)
	require.Equal(t, 1, len(plan.Stages))
	require.Equal(t, 1, len(plan.Stages[0].Units))
	unit := plan.Stages[0].Units[0]
	assert.Equal(t, string(UnitStatusUnsupported), unit.InitialStatus)
	assert.NotEmpty(t, unit.ErrorMessage)
}
