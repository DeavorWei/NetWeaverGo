package report

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVendorSanitizer_Load(t *testing.T) {
	vs := GetDefaultVendorSanitizer()
	require.NotNil(t, vs)

	totalRules := vs.TotalRules()
	t.Logf("Total rules loaded: %d", totalRules)
	assert.Greater(t, totalRules, 500)

	cats := vs.Categories()
	t.Logf("Categories loaded: %d (%v)", len(cats), cats)
	assert.GreaterOrEqual(t, len(cats), 15)

	broken := vs.BrokenRules()
	t.Logf("Broken rules count: %d", len(broken))
	assert.LessOrEqual(t, len(broken), 5, "编译失败规则应当极少")
}

func TestVendorSanitizer_HuaweiMasking(t *testing.T) {
	vs := GetDefaultVendorSanitizer()

	rawEcho := `
sysname Core-Switch-01
#
radius-server template default
 radius-server shared-key cipher %^%#K5_8d*h9W!@#X91238471289371289371289%^%#
#
user-interface vty 0 4
 authentication-mode aaa
#
aaa
 local-user admin password irreversible-cipher $1a$X#K$819238129381928391823918239128391283
 local-user admin privilege level 15
 local-user admin service-type terminal ssh http
 local-user guest password simple Huawei@123
#
dldp authentication-mode simple PlainKey999
#
`
	masked := vs.Sanitize("Ethernet Switch", "display current-configuration", rawEcho)

	// 验证敏感信息被替换
	assert.NotContains(t, masked, "%^%#K5_8d*h9W!@#X91238471289371289371289%^%#")
	assert.NotContains(t, masked, "$1a$X#K$819238129381928391823918239128391283")
	assert.NotContains(t, masked, "Huawei@123")
	assert.NotContains(t, masked, "PlainKey999")

	// 验证替换为 ****
	assert.Contains(t, masked, "shared-key ****")
	assert.Contains(t, masked, "password irreversible-cipher ****")
	assert.Contains(t, masked, "password simple ****")
	assert.Contains(t, masked, "dldp authentication-mode simple ****")
}

func TestVendorSanitizer_ResolveCategory(t *testing.T) {
	assert.Equal(t, "AR router", ResolveCategory("Huawei", "AR6140"))
	assert.Equal(t, "Ethernet Switch", ResolveCategory("Huawei", "S5735"))
	assert.Equal(t, "DC", ResolveCategory("Huawei", "CE6865"))
	assert.Equal(t, "Firewall", ResolveCategory("Huawei", "USG6600"))
	assert.Equal(t, "WLAN", ResolveCategory("Huawei", "AirEngine9700"))
	assert.Equal(t, "CISCO", ResolveCategory("Cisco", "Catalyst9300"))
	assert.Equal(t, "H3C", ResolveCategory("H3C", "S5500"))
	assert.Equal(t, "ZTE", ResolveCategory("ZTE", "ZXR10"))
}

func TestVendorSanitizer_Performance(t *testing.T) {
	vs := GetDefaultVendorSanitizer()

	// 构造 1MB 的回显
	chunk := "local-user test password irreversible-cipher $1a$1234567890123456789012345678901234567890\n"
	repeatCount := 1024 * 1024 / len(chunk)
	bigEcho := strings.Repeat(chunk, repeatCount)

	start := time.Now()
	res := vs.Sanitize("Ethernet Switch", "display current-configuration", bigEcho)
	duration := time.Since(start)

	t.Logf("1MB 脱敏耗时: %v", duration)
	assert.NotEmpty(t, res)
	assert.Less(t, duration, 200*time.Millisecond, "1MB 数据脱敏耗时应在合理范围内")
}
