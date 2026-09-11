package taskexec

import (
	"context"
	"encoding/json"
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

func TestInspectionTaskCompiler_Supports(t *testing.T) {
	compiler := NewInspectionTaskCompiler(nil, nil)
	assert.True(t, compiler.Supports(string(RunKindInspection)))
	assert.False(t, compiler.Supports(string(RunKindNormal)))
	assert.False(t, compiler.Supports(string(RunKindTopology)))
	assert.False(t, compiler.Supports(string(RunKindBackup)))
	assert.False(t, compiler.Supports(string(RunKindCEAS)))
}

func TestInspectionTaskCompiler_Compile_Success(t *testing.T) {
	compiler := NewInspectionTaskCompiler(nil, nil) // db=nil 会走 DefaultItems 回退

	cfg := InspectionTaskConfig{
		DeviceIPs:   []string{"10.0.0.1", "10.0.0.2", "10.0.0.1", " "},
		TemplateID:  "tpl-huawei-general",
		Concurrency: 4,
		TimeoutSec:  30,
	}
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	def := &TaskDefinition{
		ID:     "insp-def-1",
		Name:   "设备巡检测试",
		Kind:   string(RunKindInspection),
		Config: cfgBytes,
	}

	plan, err := compiler.Compile(context.Background(), def)
	require.NoError(t, err)
	require.NotNil(t, plan)
	assert.Equal(t, string(RunKindInspection), plan.RunKind)
	assert.Equal(t, "设备巡检测试", plan.Name)
	require.Len(t, plan.Stages, 1)

	stage := plan.Stages[0]
	assert.Equal(t, string(StageKindInspectionCheck), stage.Kind)
	assert.Equal(t, 4, stage.Concurrency)
	require.Len(t, stage.Units, 2) // 重复与空字符串已被去重过滤

	// 验证第一台设备单元配置
	unit1 := stage.Units[0]
	assert.Equal(t, "10.0.0.1", unit1.Target.Key)
	assert.Equal(t, 30*time.Second, unit1.Timeout)
	require.NotEmpty(t, unit1.Steps)

	// 验证步骤中前置项优先排序 (IsPreCollect = true 应排在 false 之前)
	seenFalse := false
	for _, step := range unit1.Steps {
		isPre := step.Params["isPreCollect"] == "true"
		if seenFalse {
			assert.False(t, isPre, "IsPreCollect=true 项应当排在所有普通项前面")
		}
		if !isPre {
			seenFalse = true
		}
	}

	// 验证第二台设备
	assert.Equal(t, "10.0.0.2", stage.Units[1].Target.Key)
}

func TestInspectionTaskCompiler_Compile_Errors(t *testing.T) {
	compiler := NewInspectionTaskCompiler(nil, nil)

	// 1. 无效 JSON
	defInvalidJSON := &TaskDefinition{
		Name:   "bad-json",
		Config: []byte("{invalid"),
	}
	_, err := compiler.Compile(context.Background(), defInvalidJSON)
	assert.Error(t, err)

	// 2. 空设备列表
	cfgEmpty := InspectionTaskConfig{
		DeviceIPs: []string{" ", ""},
	}
	cfgBytes, _ := json.Marshal(cfgEmpty)
	defEmpty := &TaskDefinition{
		Name:   "empty-devices",
		Config: cfgBytes,
	}
	_, err = compiler.Compile(context.Background(), defEmpty)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "至少一台设备")
}

func TestInspectionCheckExecutor_Kind(t *testing.T) {
	exec := NewInspectionCheckExecutor(nil, nil, nil)
	assert.Equal(t, string(StageKindInspectionCheck), exec.Kind())
}

func TestInspectionPersistence_IdempotentRunIsolation(t *testing.T) {
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=private", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.InspectionResult{}))

	// 1. 模拟历史运行 run-1 对 10.0.0.1 产生的巡检结果
	historyRes := []models.InspectionResult{
		{RunID: "run-1", DeviceIP: "10.0.0.1", ItemCode: "ITEM_1", Status: "TEST_PASS"},
		{RunID: "run-1", DeviceIP: "10.0.0.1", ItemCode: "ITEM_2", Status: "TEST_FAIL"},
	}
	require.NoError(t, db.Create(&historyRes).Error)

	// 2. 模拟新运行 run-2 执行并持久化（使用与 executor 相同的 run_id + device_ip 幂等清理事务）
	persistResults := func(runID, deviceIP string, items []models.InspectionResult) error {
		return db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Where("run_id = ? AND device_ip = ?", runID, deviceIP).Delete(&models.InspectionResult{}).Error; err != nil {
				return err
			}
			if len(items) > 0 {
				return tx.CreateInBatches(items, 100).Error
			}
			return nil
		})
	}

	newRes := []models.InspectionResult{
		{RunID: "run-2", DeviceIP: "10.0.0.1", ItemCode: "ITEM_1", Status: "TEST_PASS"},
		{RunID: "run-2", DeviceIP: "10.0.0.1", ItemCode: "ITEM_3", Status: "TEST_PASS"},
	}
	require.NoError(t, persistResults("run-2", "10.0.0.1", newRes))

	// 验证 run-1 的历史记录没有被删除或覆盖
	var run1Results []models.InspectionResult
	require.NoError(t, db.Where("run_id = ?", "run-1").Find(&run1Results).Error)
	assert.Len(t, run1Results, 2, "run-1 的历史记录应被完整保留")

	// 验证 run-2 的记录正确写入
	var run2Results []models.InspectionResult
	require.NoError(t, db.Where("run_id = ?", "run-2").Find(&run2Results).Error)
	assert.Len(t, run2Results, 2)

	// 3. 验证对 run-2 重新执行时的幂等性（同 run_id + device_ip 覆盖自身）
	newResUpdated := []models.InspectionResult{
		{RunID: "run-2", DeviceIP: "10.0.0.1", ItemCode: "ITEM_1", Status: "TEST_FAIL"},
	}
	require.NoError(t, persistResults("run-2", "10.0.0.1", newResUpdated))

	require.NoError(t, db.Where("run_id = ?", "run-1").Find(&run1Results).Error)
	assert.Len(t, run1Results, 2, "重试 run-2 绝不可影响 run-1")

	require.NoError(t, db.Where("run_id = ?", "run-2").Find(&run2Results).Error)
	require.Len(t, run2Results, 1)
	assert.Equal(t, "TEST_FAIL", run2Results[0].Status, "run-2 应被幂等更新为最新状态")
}
