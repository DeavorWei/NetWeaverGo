package report

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckContentSanitized(t *testing.T) {
	// 1. 已完全脱敏的文本
	cleanText := `
sysname Switch-A
#
user-interface vty 0 4
 authentication-mode password
 set authentication password cipher ****
#
radius-server shared-key cipher ****
#
`
	ok, violations := CheckContentSanitized(cleanText)
	assert.True(t, ok)
	assert.Empty(t, violations)
	assert.NoError(t, ValidateExportContent(cleanText))

	// 2. 包含明文/未脱敏口令的文本
	dirtyText := `
sysname Switch-B
#
set authentication password simple Admin@12345
#
`
	ok, violations = CheckContentSanitized(dirtyText)
	assert.False(t, ok)
	require.NotEmpty(t, violations)
	assert.Contains(t, violations[0], "Admin@12345")

	err := ValidateExportContent(dirtyText)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "导出安全阻断")

	// 3. 包含通用密码（未带 simple/cipher）必须阻断 (P0-2 修复验证)
	genericPasswordText := "local-user operator password SecretPass123"
	ok, _ = CheckContentSanitized(genericPasswordText)
	assert.False(t, ok)
	err = ValidateExportContent(genericPasswordText)
	require.Error(t, err, "通用密码未掩码必须触发 CRITICAL 导出阻断")

	// 4. 包含未掩码的 SNMP 团体名必须阻断
	snmpText := "snmp-agent community read public123"
	ok, _ = CheckContentSanitized(snmpText)
	assert.False(t, ok)
	err = ValidateExportContent(snmpText)
	require.Error(t, err, "SNMP 团体名未掩码必须触发导出阻断")
}
