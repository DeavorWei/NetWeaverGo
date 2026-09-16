package device

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProfileRegistry_Load(t *testing.T) {
	reg := GetDefaultProfileRegistry()
	require.NotNil(t, reg)

	cats := reg.GetCategories()
	assert.GreaterOrEqual(t, len(cats), 18, "应当至少加载 18 个产品大类")

	// 验证 index 字段保留
	hasValidIndex := false
	for _, c := range cats {
		if c.Index > 0 && c.Name != "" {
			hasValidIndex = true
			break
		}
	}
	assert.True(t, hasValidIndex, "ProductCategory 必须保留显式 index 序号")

	domains := reg.GetDomains()
	assert.GreaterOrEqual(t, len(domains), 40, "应当加载 40+ 条产品域映射规则")
}

// TestProfileOrderLocked 锁定前 20 条规则的顺序与关键款型匹配结果
// 确保合并或更新时不会出现因错序导致的模型误识别
func TestProfileOrderLocked(t *testing.T) {
	reg := GetDefaultProfileRegistry()
	require.NotNil(t, reg)

	// 规则锁定测试表：测试 CE12800 必须优先匹配为 CE12800 而不是 CE，S 系列匹配为 S 等
	testCases := []struct {
		model          string
		expectedDomain string
	}{
		{"CE12800", "CE12800"},
		{"CE9860", "CE"},
		{"NE8000", "NE8000(IPRAN)"},
		{"NE5000E", "NE-CR"},
		{"CX600", "CX600"},
		{"ME60", "ME60"},
		{"MA5200G", "ME60"},
		{"SIG9800", "SIG"},
		{"PTN6900", "ATN"},
		{"ATN950B", "ATN"},
		{"ETN200", "ATN"},
		{"NE05E", "ATN"},
		{"Eudemon9000E", "FW9000"},
		{"Eudemon1000E-C", "FW1000E"},
		{"Eudemon8000E", "FW8000"},
		{"IPS12000", "IPS12000"},
		{"A800", "ATN"},
		{"NetEngine A816", "ATN"},
		{"NetEngine XH16000", "NE8000(IPRAN)"},
		{"SIG16000", "SIG"},
		{"CE6850", "CE"},
		{"NE40E", "NE-SR"},
		{"S5735", "S"},
	}

	for _, tc := range testCases {
		domain, ok := reg.MatchDomain(tc.model)
		assert.Truef(t, ok, "款型 %s 必须能够匹配到域", tc.model)
		assert.Equalf(t, tc.expectedDomain, domain, "款型 %s 匹配的域不符合保序预期", tc.model)
	}
}

func TestProfileRegistry_Category(t *testing.T) {
	reg := GetDefaultProfileRegistry()
	require.NotNil(t, reg)

	cat, ok := reg.MatchCategory("S5735-L24P4S")
	assert.True(t, ok)
	assert.Equal(t, "Ethernet Switch", cat.Name)
	assert.Equal(t, 4, cat.Index)

	catCE, ok := reg.MatchCategory("CE6865")
	assert.True(t, ok)
	assert.Equal(t, "DC", catCE.Name)
	assert.Equal(t, 6, catCE.Index)
}

func TestProfileRegistry_DeviceSupport(t *testing.T) {
	reg := GetDefaultProfileRegistry()
	require.NotNil(t, reg)

	// S5700 支持 V200RXXX
	supported, reason := reg.IsDeviceSupported("S5700", "V200R019C00SPC500")
	assert.True(t, supported, "S5700 V200R019 应当受支持")
	assert.Empty(t, reason)

	// 不支持的伪造款型
	supported2, reason2 := reg.IsDeviceSupported("FakeModel9999", "V100R001")
	assert.False(t, supported2)
	assert.NotEmpty(t, reason2)
}
