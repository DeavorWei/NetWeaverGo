package ui

import (
	"fmt"
	"testing"

	"github.com/NetWeaverGo/core/internal/models"
	"github.com/NetWeaverGo/core/internal/parser"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type mockReloader struct {
	reloadedVendors []string
	shouldFail      bool
}

func (m *mockReloader) ReloadVendor(vendor string) error {
	if m.shouldFail {
		return assert.AnError
	}
	m.reloadedVendors = append(m.reloadedVendors, vendor)
	return nil
}

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=private", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.UserParseTemplate{}))
	return db
}

// 验证 H2：第三方厂商（无内置模板）创建模板正常成功，且测试刷新快照
func TestParseTemplateService_CreateAndList_ThirdPartyVendor(t *testing.T) {
	db := setupTestDB(t)
	manager := parser.NewParserManager()
	svc := NewParseTemplateService(db, manager)

	req := models.SaveParseTemplateRequest{
		Vendor:      "ruijie",
		CommandKey:  "show_version",
		Engine:      "regex",
		Pattern:     `System version:\s*(\S+)`,
		Multiline:   true,
		Description: "锐捷版本模板",
		Enabled:     true,
	}

	err := svc.CreateTemplate(req)
	require.NoError(t, err, "第三方未知厂商应能成功创建模板且不应报错")

	list, err := svc.ListTemplates("ruijie")
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "ruijie", list[0].Vendor)
	assert.Equal(t, "show_version", list[0].CommandKey)
}

// 验证 H1 & M5：更新模板时未提供的引擎配置不被置为 null 破坏，且不允许篡改 vendor/commandKey
func TestParseTemplateService_UpdateTemplate_PreserveConfigs(t *testing.T) {
	db := setupTestDB(t)
	reloader := &mockReloader{}
	svc := NewParseTemplateService(db, reloader)

	// 1. 创建聚合引擎模板
	createReq := models.SaveParseTemplateRequest{
		Vendor:     "huawei",
		CommandKey: "eth_trunk",
		Engine:     "aggregate",
		Aggregation: map[string]interface{}{
			"recordStart": []interface{}{"^Eth-Trunk"},
		},
		Description: "原始聚合模板",
		Enabled:     true,
	}
	err := svc.CreateTemplate(createReq)
	require.NoError(t, err)

	list, err := svc.ListTemplates("huawei")
	require.NoError(t, err)
	require.Len(t, list, 1)
	tplID := list[0].ID
	assert.NotNil(t, list[0].Aggregation)

	// 2. 用户只编辑了 description，未传 aggregation（模拟前端弹窗保存）
	updateReq := models.SaveParseTemplateRequest{
		Vendor:      "hacked_vendor",     // 试图篡改 vendor（M5）
		CommandKey:  "hacked_command",    // 试图篡改 commandKey（M5）
		Engine:      "aggregate",
		Aggregation: nil,                 // 模拟未传或为 nil（H1）
		Description: "更新后的聚合模板说明",
		Enabled:     true,
	}

	err = svc.UpdateTemplate(tplID, updateReq)
	require.NoError(t, err)

	// 3. 重新获取，验证原配置完好保留（未变成 "null"），且 key 未被篡改
	updated, err := svc.GetTemplate(tplID)
	require.NoError(t, err)
	assert.Equal(t, "huawei", updated.Vendor, "Vendor 不应被篡改")
	assert.Equal(t, "eth_trunk", updated.CommandKey, "CommandKey 不应被篡改")
	assert.Equal(t, "更新后的聚合模板说明", updated.Description)
	assert.NotNil(t, updated.Aggregation, "Aggregation 配置不应被置为 null 破坏")
}

// 验证 M3：TestTemplate 支持 FieldMapping 字段重命名映射生效
func TestParseTemplateService_TestTemplate_WithFieldMapping(t *testing.T) {
	db := setupTestDB(t)
	reloader := &mockReloader{}
	svc := NewParseTemplateService(db, reloader)

	req := models.TestParseTemplateRequest{
		Vendor:     "huawei",
		CommandKey: "sysname",
		Engine:     "regex",
		Pattern:    `sysname\s+(\S+)`,
		Multiline:  true,
		FieldMapping: map[string]string{
			"1": "hostname",
		},
		RawText: "sysname Core-Switch-01",
	}

	res := svc.TestTemplate(req)
	require.True(t, res.Success)
	require.Len(t, res.Results, 1)
	assert.Equal(t, "Core-Switch-01", res.Results[0]["hostname"], "FieldMapping 应成功将字段重命名为 hostname")
}

// 验证 H2 回滚：若 ReloadVendor 失败，CreateTemplate 应当撤回 DB 写入
func TestParseTemplateService_CreateTemplate_RollbackOnReloadFailure(t *testing.T) {
	db := setupTestDB(t)
	reloader := &mockReloader{shouldFail: true}
	svc := NewParseTemplateService(db, reloader)

	req := models.SaveParseTemplateRequest{
		Vendor:     "huawei",
		CommandKey: "test_cmd",
		Engine:     "regex",
		Pattern:    `test`,
	}

	err := svc.CreateTemplate(req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "已撤销模板创建")

	// 验证 DB 中没有留下脏数据
	list, err := svc.ListTemplates("huawei")
	require.NoError(t, err)
	assert.Len(t, list, 0, "失败后 DB 记录应被清理回滚")
}