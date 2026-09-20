package ui

import (
	"fmt"
	"testing"
	"time"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/executor"
	"github.com/NetWeaverGo/core/internal/models"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func setupRiskTestDB(t *testing.T) *gorm.DB {
	dbName := fmt.Sprintf("file:risk_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	require.NoError(t, err)

	err = db.AutoMigrate(
		&models.RiskTrustEntry{},
		&models.RiskCommandLog{},
		&models.RiskCommand{},
		&models.BizCompareTask{},
		&models.BizCompareItem{},
	)
	require.NoError(t, err)

	// 注入到全局 config
	config.SetDB(db)
	return db
}

func TestRiskCommandService_AddTrustEntryValidation(t *testing.T) {
	_ = setupRiskTestDB(t)
	svc := NewRiskCommandService()

	// 1. 空正则
	err := svc.AddTrustEntry(models.RiskTrustEntry{
		UserID:    "admin",
		Pattern:   "",
		ExpiresAt: time.Now().Add(1 * time.Hour),
		Reason:    "测试",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不能为空")

	// 2. 无效正则
	err = svc.AddTrustEntry(models.RiskTrustEntry{
		UserID:    "admin",
		Pattern:   "[a-z(",
		ExpiresAt: time.Now().Add(1 * time.Hour),
		Reason:    "测试",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "格式无效")

	// 3. 过去时间
	err = svc.AddTrustEntry(models.RiskTrustEntry{
		UserID:    "admin",
		Pattern:   "^reboot$",
		ExpiresAt: time.Now().Add(-1 * time.Hour),
		Reason:    "测试",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "未来")

	// 4. 超过 24 小时
	err = svc.AddTrustEntry(models.RiskTrustEntry{
		UserID:    "admin",
		Pattern:   "^reboot$",
		ExpiresAt: time.Now().Add(25 * time.Hour),
		Reason:    "测试",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "超过24小时")

	// 5. 合法条目
	err = svc.AddTrustEntry(models.RiskTrustEntry{
		UserID:    "admin",
		Pattern:   `^reboot\s+fast$`,
		ExpiresAt: time.Now().Add(2 * time.Hour),
		Reason:    "紧急变更通过",
	})
	assert.NoError(t, err)

	entries, err := svc.ListTrustEntries()
	assert.NoError(t, err)
	assert.Len(t, entries, 1)
}

func TestRiskCommandService_BypassRiskCommand(t *testing.T) {
	_ = setupRiskTestDB(t)
	svc := NewRiskCommandService()

	// 1. 未二次确认
	err := svc.BypassRiskCommand(executor.BypassRequest{
		RunID:           "run-999",
		DeviceIP:        "10.0.0.1",
		Command:         "delete /unreserved flash:/main.bin",
		Operator:        "admin",
		Reason:          "业务升级需要删除旧镜像",
		SecondConfirmed: false,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "二次确认")

	// 2. 理由过短
	err = svc.BypassRiskCommand(executor.BypassRequest{
		RunID:           "run-999",
		DeviceIP:        "10.0.0.1",
		Command:         "delete /unreserved flash:/main.bin",
		Operator:        "admin",
		Reason:          "放行",
		SecondConfirmed: true,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不少于5个字")

	// 3. 合法放行申请，验证闭环逃生生效
	cmd := "delete /unreserved flash:/main.bin"
	err = svc.BypassRiskCommand(executor.BypassRequest{
		RunID:           "run-999",
		DeviceIP:        "10.0.0.1",
		Command:         cmd,
		Operator:        "admin",
		Reason:          "核心升级紧急放行清理空间",
		SecondConfirmed: true,
	})
	assert.NoError(t, err)

	// 校验临时凭据是否已注入且可被校验器消费
	ok, bypass := executor.GetGlobalRiskValidator().CheckBypass("run-999", "10.0.0.1", cmd)
	assert.True(t, ok)
	assert.NotNil(t, bypass)
	assert.Equal(t, "admin", bypass.Operator)
}

func TestBizCompareService_ExportDiffCSVSanitizeGuard(t *testing.T) {
	db := setupRiskTestDB(t)
	svc := NewBizCompareService()

	taskID := "task-test-sanitize-01"
	task := models.BizCompareTask{
		TaskID:    taskID,
		Name:      "敏感数据比对测试",
		Status:    "finished",
		CreatedAt: time.Now(),
	}
	db.Create(&task)

	// 插入包含未脱敏密码的差异项
	db.Create(&models.BizCompareItem{
		CompareTaskID: task.ID,
		DeviceIP:      "192.168.1.10",
		Domain:        "Routing",
		ItemCategory:  "BGP",
		ItemKey:       "Peer-Password",
		DiffType:      "modified",
		BeforeValue:   "password simple PlainSecret888",
		AfterValue:    "password simple PlainSecret999",
		ImpactLevel:   "high",
		ImpactScope:   "全局",
	})

	// 尝试导出 CSV，应被 report.ValidateExportContent 拦截阻断
	csvStr, err := svc.ExportDiffCSV(taskID)
	assert.Error(t, err, "包含未掩码明文口令的 CSV 必须阻断导出")
	assert.Contains(t, err.Error(), "导出安全阻断")
	assert.Empty(t, csvStr)
}
