package inspection_test

import (
	"testing"

	"github.com/NetWeaverGo/core/internal/inspection"
	"github.com/NetWeaverGo/core/internal/models"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// 阶段二 2.3：内置多语言文案种子覆盖度与幂等性。
func TestEnsureInspectionItemTextSeeds(t *testing.T) {
	// 覆盖度：种子应覆盖至少 12 个检查项，且每条具备 Key/Locale/Name。
	want := len(inspection.DefaultItemTexts)
	require.GreaterOrEqual(t, want, 12, "种子应覆盖至少 12 个检查项")
	for _, tx := range inspection.DefaultItemTexts {
		assert.NotEmpty(t, tx.Key, "种子 Key 不能为空（需等于 InspectionItem.Code）")
		assert.NotEmpty(t, tx.Locale, "种子 Locale 不能为空")
		assert.NotEmpty(t, tx.Name, "种子 Name 不能为空")
	}

	// 幂等性：首次写入后行数 = want；二次调用不新增行（OnConflict 幂等 + 包级 Once 守卫）。
	db, err := gorm.Open(sqlite.Open("file:seed-test?mode=memory&cache=shared"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.InspectionItemText{}))

	require.NoError(t, inspection.EnsureInspectionItemTextSeeds(db))
	var n1 int64
	require.NoError(t, db.Model(&models.InspectionItemText{}).Count(&n1).Error)
	assert.Equal(t, int64(want), n1, "首次播种行数应等于种子条数")

	require.NoError(t, inspection.EnsureInspectionItemTextSeeds(db))
	var n2 int64
	require.NoError(t, db.Model(&models.InspectionItemText{}).Count(&n2).Error)
	assert.Equal(t, n1, n2, "二次播种不应新增行（幂等）")
}
