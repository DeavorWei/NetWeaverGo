package parser

import (
	"testing"

	"github.com/NetWeaverGo/core/internal/models"
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

func TestParserManager_GetParserForDevice_AppliesTo(t *testing.T) {
	mgr := NewParserManager()
	require.NoError(t, mgr.Bootstrap())

	// 模拟针对 CE 系列特定定制的 version 模板
	mockSource := &mockUserTemplateSource{
		templates: []StoredTemplate{
			{
				Vendor:     "huawei",
				CommandKey: "version",
				Engine:     "regex",
				Pattern:    `(?m)CloudEngine\s+(?P<model>\S+)\s+Version\s+(?P<version>\S+)`,
				AppliesTo:  `{"models": ["CE*"]}`,
				Enabled:    true,
			},
		},
	}
	mgr.SetUserTemplateSource(mockSource)
	require.NoError(t, mgr.ReloadVendor("huawei"))

	// 1. 对于 CE 设备，选配到 CE 定制模板
	ceParser, err := mgr.GetParserForDevice("huawei", "CE6866", "V200R005")
	require.NoError(t, err)
	ceEcho := "CloudEngine CE6866 Version V200R005C10SPC600"
	ceRows, err := ceParser.Parse("version", ceEcho)
	require.NoError(t, err)
	require.Len(t, ceRows, 1)
	assert.Equal(t, "CE6866", ceRows[0]["model"])

	// 2. 对于 S5735 设备，不满足 CE* 约束，回退默认基础模板
	sParser, err := mgr.GetParserForDevice("huawei", "S5735", "V200R019")
	require.NoError(t, err)
	sEcho := "Huawei Versatile Routing Platform Software\nVRP (R) software, Version 5.170 (S5735 V200R019C00SPC500)"
	sRows, err := sParser.Parse("version", sEcho)
	require.NoError(t, err)
	require.NotEmpty(t, sRows)
}

func TestParserManager_DefaultFallback(t *testing.T) {
	mgr := NewParserManager()
	require.NoError(t, mgr.Bootstrap())

	// 未知厂商获取解析器，优雅回退到 default.json 保守解析器，杜绝 ErrVendorNotLoaded 崩溃
	unknownParser, err := mgr.GetParser("unknown_switch_brand")
	require.NoError(t, err)
	require.NotNil(t, unknownParser)

	echo := "Switch Software, Version 1.2.3\n<Switch-01> uptime is 1 day\n"
	rows, err := unknownParser.Parse("version", echo)
	require.NoError(t, err)
	require.NotEmpty(t, rows)
	assert.Equal(t, "1.2.3", rows[0]["version"])
}

func TestMatchesAppliesTo_WildcardAndSeries(t *testing.T) {
	// 1. nil appliesTo
	assert.True(t, matchesAppliesTo(nil, "S5735", "V200R019"))

	// 2. 空 AppliesTo
	assert.True(t, matchesAppliesTo(&models.TemplateAppliesTo{}, "S5735", "V200R019"))

	// 3. 通配符 *
	assert.True(t, matchesAppliesTo(&models.TemplateAppliesTo{
		Models:   []string{"*"},
		Versions: []string{"*"},
	}, "S5735", "V200R019"))

	// 4. 款型前缀通配符 S57*
	prefixApplies := &models.TemplateAppliesTo{
		Models: []string{"S57*"},
	}
	assert.True(t, matchesAppliesTo(prefixApplies, "S5735-L24P4S-A2", "V200R019"))
	assert.False(t, matchesAppliesTo(prefixApplies, "CE6866", "V200R019"))

	// 5. 系列归一化匹配 S5700
	seriesApplies := &models.TemplateAppliesTo{
		Models: []string{"S5700"},
	}
	assert.True(t, matchesAppliesTo(seriesApplies, "S5735-S", "V200R019"))
	assert.True(t, matchesAppliesTo(seriesApplies, "S5700", "V200R019"))
	assert.False(t, matchesAppliesTo(seriesApplies, "AR6280", "V200R019"))

	// 6. 版本前缀通配符 V200*
	verApplies := &models.TemplateAppliesTo{
		Versions: []string{"V200*"},
	}
	assert.True(t, matchesAppliesTo(verApplies, "S5735", "V200R019C00SPC500"))
	assert.False(t, matchesAppliesTo(verApplies, "S5735", "V300R019C11SPC200"))

	// 7. 款型 + 版本联合判定
	comboApplies := &models.TemplateAppliesTo{
		Models:   []string{"CE*", "S5700"},
		Versions: []string{"V200*"},
	}
	assert.True(t, matchesAppliesTo(comboApplies, "CE6866", "V200R005"))
	assert.True(t, matchesAppliesTo(comboApplies, "S5720", "V200R019"))
	assert.False(t, matchesAppliesTo(comboApplies, "CE6866", "V300R005"))
	assert.False(t, matchesAppliesTo(comboApplies, "AR6280", "V200R019"))
}
