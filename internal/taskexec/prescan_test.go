package taskexec

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/NetWeaverGo/core/internal/device"
	"github.com/NetWeaverGo/core/internal/models"
	"github.com/NetWeaverGo/core/internal/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseExecutor_PreScanAndPersistence(t *testing.T) {
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.DeviceAsset{}))

	// 1. 初始化待解析数据与资产记录
	runID := "run-prescan-test-01"
	deviceIP := "192.168.10.1"

	asset := models.DeviceAsset{
		IP:     deviceIP,
		Vendor: "huawei",
	}
	require.NoError(t, db.Create(&asset).Error)

	runDev := TaskRunDevice{
		TaskRunID: runID,
		DeviceIP:  deviceIP,
		Vendor:    "huawei",
		Status:    "running",
	}
	require.NoError(t, db.Create(&runDev).Error)

	// 创建临时 version 输出落盘文件
	tmpDir := t.TempDir()
	verFile := filepath.Join(tmpDir, "version.txt")
	verContent := `Huawei Versatile Routing Platform Software
VRP (R) software, Version 5.170 (S5735-L24P4S-A2 V200R019C00SPC500)
HUAWEI S5735-L24P4S-A2 uptime is 120 days
`
	require.NoError(t, os.WriteFile(verFile, []byte(verContent), 0644))

	output := TaskRawOutput{
		TaskRunID:     runID,
		DeviceIP:      deviceIP,
		CommandKey:    "version",
		Status:        "success",
		ParseFilePath: verFile,
	}
	require.NoError(t, db.Create(&output).Error)

	// 2. 初始化真实 ParserManager
	mgr := parser.NewParserManager()
	require.NoError(t, mgr.Bootstrap())

	parseExec := NewParseExecutor(db, mgr)

	// 3. 执行单设备解析
	ctx := newProjectorTestRuntimeContext(runID)
	err := parseExec.parseAndSaveRunDevice(ctx, deviceIP, "huawei")
	require.NoError(t, err)

	// 4. 验证 TaskRunDevice 是否成功落库归一化系列和形态认知信息
	var updatedDev TaskRunDevice
	err = db.Where("task_run_id = ? AND device_ip = ?", runID, deviceIP).First(&updatedDev).Error
	require.NoError(t, err)

	assert.Equal(t, "completed", updatedDev.Status)
	assert.Equal(t, "S5735", updatedDev.Model)
	assert.Equal(t, "S5700", updatedDev.ModelSeries) // 归一化系列
	assert.Equal(t, "V200R019C00SPC500", updatedDev.Version)
	assert.NotEmpty(t, updatedDev.ProfileMatchPath)

	// 5. 验证 DeviceAsset 是否同步更新了系列信息
	var updatedAsset models.DeviceAsset
	err = db.Where("ip = ?", deviceIP).First(&updatedAsset).Error
	require.NoError(t, err)
	assert.Equal(t, "S5700", updatedAsset.ModelSeries)
}

func TestMapCommandOutput_DeviceIdentifyBridge(t *testing.T) {
	identity := &parser.DeviceIdentity{
		Vendor: "huawei",
		MgmtIP: "10.0.0.1",
	}

	rawVersion := `Huawei Versatile Routing Platform Software
VRP (R) software, Version 8.180 (CE6866 V200R005C10SPC600)
HUAWEI CE6866-48S8CQ-EI uptime is 10 days
`
	mapper := parser.GetMapper("huawei")
	rows := []map[string]string{
		{"model": "CE6866", "version": "V200R005C10SPC600"},
	}

	batch, err := MapCommandOutput(mapper, "version", rows, identity, "ref-1", rawVersion)
	require.NoError(t, err)
	require.NotNil(t, batch)

	assert.NotNil(t, identity.DeviceRef)
	assert.Equal(t, "CE6866", identity.DeviceRef.Model)
	assert.Equal(t, "CE6800", identity.DeviceRef.Series) // 归一化系列
	assert.Equal(t, "V200R005C10SPC600", identity.DeviceRef.Version)
	assert.Equal(t, "CE6800", identity.ModelSeries)
	assert.Equal(t, "series:CE6800", identity.ProfileMatchPath)
	assert.NotEmpty(t, identity.IdentityEvidence)
}

func TestSaveDeviceIdentity_PersistsFields(t *testing.T) {
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.DeviceAsset{}))
	persister := NewTopologyFactsPersister(db)

	runID := "run-save-test"
	ip := "172.16.1.1"

	require.NoError(t, db.Create(&TaskRunDevice{
		TaskRunID: runID,
		DeviceIP:  ip,
	}).Error)

	require.NoError(t, db.Create(&models.DeviceAsset{
		IP: ip,
	}).Error)

	id := &parser.DeviceIdentity{
		Vendor:           "huawei",
		Model:            "NE40E",
		Version:          "V800R011C00SPC200",
		Hostname:         "Core-Router-01",
		MgmtIP:           ip,
		ModelSeries:      "NE40E",
		PatchVersion:     "V800R011SPH010",
		ProfileMatchPath: "series:NE40E",
		IdentityEvidence: "命中 _REG2HANDLER: NE40E/CX600",
		DeviceRef: &device.Identity{
			Vendor:   "huawei",
			Model:    "NE40E",
			Series:   "NE40E",
			Version:  "V800R011C00SPC200",
			Patch:    "V800R011SPH010",
			Evidence: []string{"命中 _REG2HANDLER: NE40E/CX600"},
		},
	}

	err := persister.SaveDeviceIdentity(runID, ip, id)
	require.NoError(t, err)

	var runDev TaskRunDevice
	require.NoError(t, db.Where("task_run_id = ? AND device_ip = ?", runID, ip).First(&runDev).Error)
	assert.Equal(t, "NE40E", runDev.Model)
	assert.Equal(t, "NE40E", runDev.ModelSeries)
	assert.Equal(t, "V800R011SPH010", runDev.PatchVersion)
	assert.Equal(t, "series:NE40E", runDev.ProfileMatchPath)
	assert.Equal(t, "命中 _REG2HANDLER: NE40E/CX600", runDev.IdentityEvidence)

	var asset models.DeviceAsset
	require.NoError(t, db.Where("ip = ?", ip).First(&asset).Error)
	assert.Equal(t, "NE40E", asset.Model)
	assert.Equal(t, "NE40E", asset.ModelSeries)
	assert.Equal(t, "V800R011C00SPC200", asset.Version)
	assert.Equal(t, "V800R011SPH010", asset.PatchVersion)
}

func TestParseExecutor_MultiFilePreScanAndPatch(t *testing.T) {
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.DeviceAsset{}))

	runID := "run-multi-scan-01"
	deviceIP := "192.168.20.1"

	asset := models.DeviceAsset{
		IP:     deviceIP,
		Vendor: "huawei",
	}
	require.NoError(t, db.Create(&asset).Error)

	runDev := TaskRunDevice{
		TaskRunID: runID,
		DeviceIP:  deviceIP,
		Vendor:    "huawei",
		Status:    "running",
	}
	require.NoError(t, db.Create(&runDev).Error)

	tmpDir := t.TempDir()
	verFile := filepath.Join(tmpDir, "version.txt")
	verContent := `Huawei Versatile Routing Platform Software
VRP (R) software, Version 8.180 (CE6866 V200R005C10SPC600)
HUAWEI CE6866-48S8CQ-EI uptime is 50 days
`
	require.NoError(t, os.WriteFile(verFile, []byte(verContent), 0644))

	patchFile := filepath.Join(tmpDir, "patch.txt")
	patchContent := `Patch Package Version: V200R005SPH020
Patch Package State: Running
`
	require.NoError(t, os.WriteFile(patchFile, []byte(patchContent), 0644))

	require.NoError(t, db.Create(&TaskRawOutput{
		TaskRunID:     runID,
		DeviceIP:      deviceIP,
		CommandKey:    "version",
		Status:        "success",
		ParseFilePath: verFile,
	}).Error)

	require.NoError(t, db.Create(&TaskRawOutput{
		TaskRunID:     runID,
		DeviceIP:      deviceIP,
		CommandKey:    "patch_info",
		Status:        "success",
		ParseFilePath: patchFile,
	}).Error)

	mgr := parser.NewParserManager()
	require.NoError(t, mgr.Bootstrap())

	parseExec := NewParseExecutor(db, mgr)
	ctx := newProjectorTestRuntimeContext(runID)
	err := parseExec.parseAndSaveRunDevice(ctx, deviceIP, "huawei")
	require.NoError(t, err)

	var updatedDev TaskRunDevice
	err = db.Where("task_run_id = ? AND device_ip = ?", runID, deviceIP).First(&updatedDev).Error
	require.NoError(t, err)

	assert.Equal(t, "completed", updatedDev.Status)
	assert.Equal(t, "CE6866", updatedDev.Model)
	assert.Equal(t, "CE6800", updatedDev.ModelSeries)
	assert.Equal(t, "V200R005C10SPC600", updatedDev.Version)
	assert.Equal(t, "V200R005SPH020", updatedDev.PatchVersion)
	assert.Contains(t, updatedDev.ProfileMatchPath, "series:CE6800")
	assert.NotEmpty(t, updatedDev.IdentityEvidence)

	var updatedAsset models.DeviceAsset
	err = db.Where("ip = ?", deviceIP).First(&updatedAsset).Error
	require.NoError(t, err)
	assert.Equal(t, "CE6866", updatedAsset.Model)
	assert.Equal(t, "CE6800", updatedAsset.ModelSeries)
	assert.Equal(t, "V200R005C10SPC600", updatedAsset.Version)
	assert.Equal(t, "V200R005SPH020", updatedAsset.PatchVersion)
}
