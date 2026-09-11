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
}
