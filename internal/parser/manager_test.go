package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockUserTemplateSource struct {
	templates []StoredTemplate
}

func (m *mockUserTemplateSource) ListEnabled(vendor string) ([]StoredTemplate, error) {
	var res []StoredTemplate
	for _, t := range m.templates {
		if t.Vendor == vendor && t.Enabled {
			res = append(res, t)
		}
	}
	return res, nil
}

func TestParserManager_UserTemplateOverride(t *testing.T) {
	mgr := NewParserManager()
	require.NoError(t, mgr.Bootstrap())

	// 内置模板正常可获取
	p, err := mgr.GetParser("huawei")
	require.NoError(t, err)
	assert.NotNil(t, p)

	// 模拟用户新增了一个自定义 tree 规则模板，或覆盖已有命令
	customRulesJSON := `{
		"rules": [
			{"parseItem": "slot", "parentItem": "", "isList": true, "splitRegex": "(?m)^Slot (\\d+):", "groupIndex": 1, "isOutput": true},
			{"parseItem": "type", "parentItem": "slot", "isList": false, "parseRegex": "Type: (\\S+)", "groupIndex": 1, "isOutput": true}
		]
	}`

	mockSource := &mockUserTemplateSource{
		templates: []StoredTemplate{
			{
				Vendor:      "huawei",
				CommandKey:  "custom_test_cmd",
				Engine:      "tree",
				ParseRules:  customRulesJSON,
				Enabled:     true,
			},
		},
	}

	// 注入数据源（打通断头路 #2）
	mgr.SetUserTemplateSource(mockSource)

	// 重载华为
	err = mgr.ReloadVendor("huawei")
	require.NoError(t, err)

	// 获取更新后的快照
	pUpdated, err := mgr.GetParser("huawei")
	require.NoError(t, err)

	testEcho := `
Slot 10:
  Type: MainControl
Slot 20:
  Type: LineCard
`
	rows, err := pUpdated.Parse("custom_test_cmd", testEcho)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, "10", rows[0]["slot"])
	assert.Equal(t, "MainControl", rows[0]["type"])
	assert.Equal(t, "20", rows[1]["slot"])
	assert.Equal(t, "LineCard", rows[1]["type"])
}

func TestParserManager_EngineModeAndMetrics(t *testing.T) {
	mgr := NewParserManager()
	require.NoError(t, mgr.Bootstrap())

	// 准备用户模板：同时配置 tree 规则与 legacy 正则的命令
	customRulesJSON := `{
		"rules": [
			{"parseItem": "slot", "parentItem": "", "isList": true, "splitRegex": "(?m)^Slot (\\d+):", "groupIndex": 1, "isOutput": true}
		]
	}`

	mockSource := &mockUserTemplateSource{
		templates: []StoredTemplate{
			{
				Vendor:     "huawei",
				CommandKey: "multi_engine_cmd",
				Engine:     "tree",
				ParseRules: customRulesJSON,
				Pattern:    `(?m)^Slot (?P<slot>\d+):`,
				Enabled:    true,
			},
			{
				Vendor:     "huawei",
				CommandKey: "pure_tree_cmd",
				Engine:     "tree",
				ParseRules: customRulesJSON,
				Enabled:    true,
			},
		},
	}
	mgr.SetUserTemplateSource(mockSource)
	require.NoError(t, mgr.ReloadVendor("huawei"))

	p, err := mgr.GetParser("huawei")
	require.NoError(t, err)

	testEcho := "Slot 1:\nSlot 2:\n"

	// 1. 默认 Auto 模式：优先走 Tree 引擎，指标正常累加
	mgr.SetEngineMode(EngineModeAuto)
	mgr.ResetMetrics()

	rows, err := p.Parse("multi_engine_cmd", testEcho)
	require.NoError(t, err)
	assert.Len(t, rows, 2)

	m := mgr.GetMetrics()
	assert.Equal(t, uint64(1), m.TotalParsed)
	assert.Equal(t, uint64(1), m.SuccessCount)
	assert.Equal(t, uint64(0), m.FailureCount)
	assert.Equal(t, uint64(0), m.FallbackCount)

	// 2. 应急回退模式 LegacyOnly：拒绝 tree，fallback 到 legacy 正则
	mgr.SetEngineMode(EngineModeLegacyOnly)
	rows, err = p.Parse("multi_engine_cmd", testEcho)
	require.NoError(t, err)
	assert.Len(t, rows, 2)
	assert.Equal(t, "1", rows[0]["slot"])

	m = mgr.GetMetrics()
	assert.Equal(t, uint64(2), m.TotalParsed)
	assert.Equal(t, uint64(2), m.SuccessCount)
	assert.Equal(t, uint64(1), m.FallbackCount) // 发生了一次 fallback

	// 若只有 tree 规则且无 legacy 规则，在 LegacyOnly 下应直接拦截报错
	_, err = p.Parse("pure_tree_cmd", testEcho)
	assert.Error(t, err)

	m = mgr.GetMetrics()
	assert.Equal(t, uint64(3), m.TotalParsed)
	assert.Equal(t, uint64(1), m.FailureCount) // 累加了一次错误

	// 3. TreeOnly 模式：只允许 tree 引擎，内置 regex 命令将被拦截
	mgr.SetEngineMode(EngineModeTreeOnly)
	_, err = p.Parse("display_version", "Huawei Vers...") // 内置纯 regex 命令
	assert.Error(t, err)

	m = mgr.GetMetrics()
	assert.Equal(t, uint64(4), m.TotalParsed)
	assert.Equal(t, uint64(2), m.FailureCount)

	// 4. 重置指标
	mgr.ResetMetrics()
	m = mgr.GetMetrics()
	assert.Equal(t, uint64(0), m.TotalParsed)
	assert.Equal(t, uint64(0), m.SuccessCount)
}
