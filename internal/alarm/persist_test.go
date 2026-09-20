package alarm

import (
	"fmt"
	"testing"
	"time"

	"github.com/NetWeaverGo/core/internal/models"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func setupAlarmTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbName := fmt.Sprintf("file:alarm_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.AlarmRecord{}, &models.MergedPhenomenon{}))
	return db
}

// P0-6：归并结果必须落库、回填关联 ID，且重复分析保持幂等
func TestPersistMergedPhenomena_Idempotent(t *testing.T) {
	db := setupAlarmTestDB(t)
	now := time.Now()
	records := []models.AlarmRecord{
		{RunID: "run-1", DeviceIP: "10.0.0.1", Family: "CE", AlarmName: "CE_POWER_FAIL", Severity: "critical", Category: "power", Summary: "电源模块故障", Status: "active", OccurredAt: now, CreatedAt: now},
		{RunID: "run-1", DeviceIP: "10.0.0.1", Family: "CE", AlarmName: "CE_FAN_FAIL", Severity: "major", Category: "fan", Summary: "风扇故障", Status: "active", OccurredAt: now, CreatedAt: now},
	}
	require.NoError(t, db.Create(&records).Error)

	phenomena, err := PersistMergedPhenomena(db, nil, records)
	require.NoError(t, err)
	require.NotEmpty(t, phenomena)

	var count int64
	require.NoError(t, db.Model(&models.MergedPhenomenon{}).Count(&count).Error)
	assert.Equal(t, int64(len(phenomena)), count, "归并现象应全部落库")

	var backfilled int64
	require.NoError(t, db.Model(&models.AlarmRecord{}).
		Where("merged_phenomenon_id IS NOT NULL").
		Count(&backfilled).Error)
	assert.Greater(t, backfilled, int64(0), "原始告警应回填 MergedPhenomenonID")

	// 幂等：同一批告警重复归并不产生重复现象
	again, err := PersistMergedPhenomena(db, nil, records)
	require.NoError(t, err)
	assert.Equal(t, len(phenomena), len(again))

	var countAfter int64
	require.NoError(t, db.Model(&models.MergedPhenomenon{}).Count(&countAfter).Error)
	assert.Equal(t, count, countAfter, "重复归并不得产生重复现象")
}
