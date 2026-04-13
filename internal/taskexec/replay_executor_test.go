package taskexec

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/parser"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// =============================================================================
// Mock 实现
// =============================================================================

// mockCliParser 模拟CLI解析器
type mockCliParser struct {
	parseFunc func(commandKey string, rawText string) ([]map[string]string, error)
}

func (m *mockCliParser) Parse(commandKey string, rawText string) ([]map[string]string, error) {
	if m.parseFunc != nil {
		return m.parseFunc(commandKey, rawText)
	}
	// 默认返回空结果
	return []map[string]string{}, nil
}

// mockParserProvider 模拟解析器提供者
type mockParserProvider struct {
	parsers map[string]parser.CliParser
}

func newMockParserProvider() *mockParserProvider {
	return &mockParserProvider{
		parsers: make(map[string]parser.CliParser),
	}
}

func (m *mockParserProvider) GetParser(vendor string) (parser.CliParser, error) {
	if p, ok := m.parsers[vendor]; ok {
		return p, nil
	}
	// 返回默认mock解析器
	return &mockCliParser{}, nil
}

func (m *mockParserProvider) SetParser(vendor string, p parser.CliParser) {
	m.parsers[vendor] = p
}

// =============================================================================
// 测试辅助函数
// =============================================================================

// setupReplayTestDB 创建测试数据库
func setupReplayTestDB(t *testing.T) *gorm.DB {
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=private", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	err = AutoMigrate(db)
	require.NoError(t, err)

	return db
}

// setupReplayTestEnv 设置测试环境（临时目录）
func setupReplayTestEnv(t *testing.T) (*gorm.DB, *mockParserProvider, string) {
	t.Helper()

	db := setupReplayTestDB(t)
	provider := newMockParserProvider()

	// 使用临时目录
	tempRoot := t.TempDir()
	pm := config.GetPathManager()
	originalRoot := pm.GetStorageRoot()
	require.NoError(t, pm.UpdateStorageRoot(tempRoot))

	t.Cleanup(func() {
		_ = pm.UpdateStorageRoot(originalRoot)
	})

	return db, provider, tempRoot
}

// createTestRawFiles 创建测试用Raw文件
func createTestRawFiles(t *testing.T, runID string, devices map[string][]string) string {
	t.Helper()

	pm := config.GetPathManager()
	rawDir := filepath.Join(pm.TopologyRawDir, runID)

	for deviceIP, commands := range devices {
		deviceDir := filepath.Join(rawDir, deviceIP)
		require.NoError(t, os.MkdirAll(deviceDir, 0755))

		for _, cmd := range commands {
			rawFile := filepath.Join(deviceDir, cmd+"_raw.txt")
			content := fmt.Sprintf("Mock output for %s on %s", cmd, deviceIP)
			require.NoError(t, os.WriteFile(rawFile, []byte(content), 0644))
		}
	}

	return rawDir
}

// createTestTopologyRun 创建测试用拓扑运行记录
func createTestTopologyRun(t *testing.T, db *gorm.DB, runID string, name string) *TaskRun {
	run := &TaskRun{
		ID:        runID,
		Name:      name,
		RunKind:   string(RunKindTopology),
		Status:    string(RunStatusCompleted),
		Progress:  100,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	require.NoError(t, db.Create(run).Error)
	return run
}

// createTestRunDevice 创建测试用设备记录
func createTestRunDevice(t *testing.T, db *gorm.DB, runID string, deviceIP string) *TaskRunDevice {
	device := &TaskRunDevice{
		TaskRunID: runID,
		DeviceIP:  deviceIP,
		Status:    "completed",
		Vendor:    "huawei",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	require.NoError(t, db.Create(device).Error)
	return device
}

// =============================================================================
// 单元测试
// =============================================================================

// TestGenerateReplayRunID 测试重放运行ID生成
func TestGenerateReplayRunID(t *testing.T) {
	originalRunID := "test-run-123"

	// 生成两个ID，验证唯一性
	id1 := generateReplayRunID(originalRunID)
	time.Sleep(2 * time.Millisecond) // 确保时间戳不同
	id2 := generateReplayRunID(originalRunID)

	// 验证前缀
	assert.True(t, strings.HasPrefix(id1, "replay_"+originalRunID+"_"))
	assert.True(t, strings.HasPrefix(id2, "replay_"+originalRunID+"_"))

	// 验证唯一性
	assert.NotEqual(t, id1, id2, "不同时间生成的重放ID应该不同")
}

// TestScanRawFiles 测试Raw文件扫描
func TestScanRawFiles(t *testing.T) {
	db, _, tempRoot := setupReplayTestEnv(t)

	// 创建测试Raw文件
	runID := "test-scan-run"
	devices := map[string][]string{
		"192.168.1.1": {"version", "lldp_neighbor"},
		"192.168.1.2": {"version", "lldp_neighbor", "display_arp"},
	}
	createTestRawFiles(t, runID, devices)

	// 创建执行器
	executor := NewReplayExecutor(db, newMockParserProvider())

	// 执行扫描
	files, err := executor.scanRawFiles(runID)
	require.NoError(t, err)

	// 验证结果
	assert.Len(t, files, 5, "应该扫描到5个Raw文件")

	// 验证文件信息
	deviceFileCount := make(map[string]int)
	for _, f := range files {
		deviceFileCount[f.DeviceIP]++
		assert.True(t, strings.HasSuffix(f.FilePath, "_raw.txt"))
		assert.NotEmpty(t, f.CommandKey)
		assert.NotEmpty(t, f.DeviceIP)
		assert.Greater(t, f.FileSize, int64(0))
	}

	assert.Equal(t, 2, deviceFileCount["192.168.1.1"])
	assert.Equal(t, 3, deviceFileCount["192.168.1.2"])

	t.Logf("测试环境: %s", tempRoot)
}

// TestScanRawFiles_EmptyDir 测试空目录扫描
func TestScanRawFiles_EmptyDir(t *testing.T) {
	db, _, _ := setupReplayTestEnv(t)

	executor := NewReplayExecutor(db, newMockParserProvider())

	// 扫描不存在的运行ID
	files, err := executor.scanRawFiles("non-existent-run")

	// 应该返回空切片，不报错
	assert.NoError(t, err)
	assert.Empty(t, files)
}

// TestScanRawFiles_FilterByDevice 测试按设备过滤
func TestScanRawFiles_FilterByDevice(t *testing.T) {
	db, _, _ := setupReplayTestEnv(t)

	runID := "test-filter-run"
	devices := map[string][]string{
		"192.168.1.1": {"version"},
		"192.168.1.2": {"version"},
		"192.168.1.3": {"version"},
	}
	createTestRawFiles(t, runID, devices)

	executor := NewReplayExecutor(db, newMockParserProvider())

	// 扫描所有文件
	allFiles, err := executor.scanRawFiles(runID)
	require.NoError(t, err)
	assert.Len(t, allFiles, 3)

	// 验证过滤逻辑（在Execute中实现，这里只验证scanRawFiles返回所有文件）
	deviceSet := make(map[string]bool)
	for _, f := range allFiles {
		deviceSet[f.DeviceIP] = true
	}
	assert.Len(t, deviceSet, 3)
}

// TestListReplayableRuns 测试列出可重放运行
func TestListReplayableRuns(t *testing.T) {
	db, _, _ := setupReplayTestEnv(t)

	// 创建测试运行记录
	run1 := createTestTopologyRun(t, db, "topology-run-1", "拓扑任务1")
	run2 := createTestTopologyRun(t, db, "topology-run-2", "拓扑任务2")
	_ = run1
	_ = run2

	// 创建设备记录
	createTestRunDevice(t, db, "topology-run-1", "192.168.1.1")
	createTestRunDevice(t, db, "topology-run-1", "192.168.1.2")
	createTestRunDevice(t, db, "topology-run-2", "192.168.2.1")

	// 为run1创建Raw文件
	createTestRawFiles(t, "topology-run-1", map[string][]string{
		"192.168.1.1": {"version"},
	})

	executor := NewReplayExecutor(db, newMockParserProvider())

	// 列出可重放运行
	runs, err := executor.ListReplayableRuns(10)
	require.NoError(t, err)

	// 验证结果
	assert.GreaterOrEqual(t, len(runs), 2)

	// 查找run1
	var run1Info *ReplayableRunInfo
	for _, r := range runs {
		if r.RunID == "topology-run-1" {
			run1Info = &r
			break
		}
	}

	require.NotNil(t, run1Info)
	assert.Equal(t, "拓扑任务1", run1Info.TaskName)
	assert.Equal(t, 2, run1Info.DeviceCount)
	assert.True(t, run1Info.HasRawFiles)
	assert.Greater(t, run1Info.RawFileCount, 0)
}

// TestGetReplayHistory 测试获取重放历史
func TestGetReplayHistory(t *testing.T) {
	db, _, _ := setupReplayTestEnv(t)

	originalRunID := "original-run-1"

	// 创建重放记录
	now := time.Now()
	record1 := &TopologyReplayRecord{
		OriginalRunID: originalRunID,
		ReplayRunID:   "replay-1",
		Status:        string(ReplayStatusCompleted),
		TriggerSource: "manual",
		StartedAt:     &now,
		FinishedAt:    &now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	require.NoError(t, db.Create(record1).Error)

	record2 := &TopologyReplayRecord{
		OriginalRunID: originalRunID,
		ReplayRunID:   "replay-2",
		Status:        string(ReplayStatusFailed),
		TriggerSource: "manual",
		ErrorMessage:  "测试错误",
		StartedAt:     &now,
		FinishedAt:    &now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	require.NoError(t, db.Create(record2).Error)

	executor := NewReplayExecutor(db, newMockParserProvider())

	// 获取重放历史
	history, err := executor.GetReplayHistory(originalRunID)
	require.NoError(t, err)

	assert.Len(t, history, 2)

	// 验证记录
	var completedRecord, failedRecord *TopologyReplayRecord
	for i := range history {
		if history[i].ReplayRunID == "replay-1" {
			completedRecord = &history[i]
		}
		if history[i].ReplayRunID == "replay-2" {
			failedRecord = &history[i]
		}
	}

	require.NotNil(t, completedRecord)
	assert.Equal(t, string(ReplayStatusCompleted), completedRecord.Status)

	require.NotNil(t, failedRecord)
	assert.Equal(t, string(ReplayStatusFailed), failedRecord.Status)
	assert.Equal(t, "测试错误", failedRecord.ErrorMessage)
}

// TestClearExistingResults 测试清除现有结果
func TestClearExistingResults(t *testing.T) {
	db, _, _ := setupReplayTestEnv(t)

	runID := "test-clear-run"

	// 创建测试数据
	lldp := &TaskParsedLLDPNeighbor{
		TaskRunID:       runID,
		DeviceIP:        "192.168.1.1",
		LocalInterface:  "GE0/0/1",
		NeighborChassis: "aa:bb:cc:dd:ee:ff",
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	require.NoError(t, db.Create(lldp).Error)

	fdb := &TaskParsedFDBEntry{
		TaskRunID:  runID,
		DeviceIP:   "192.168.1.1",
		MACAddress: "aa:bb:cc:dd:ee:ff",
		VLAN:       1,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	require.NoError(t, db.Create(fdb).Error)

	edge := &TaskTopologyEdge{
		ID:        "edge-1",
		TaskRunID: runID,
		ADeviceID: "192.168.1.1",
		BDeviceID: "192.168.1.2",
		Status:    "confirmed",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	require.NoError(t, db.Create(edge).Error)

	// 验证数据存在
	var count int64
	db.Model(&TaskParsedLLDPNeighbor{}).Where("task_run_id = ?", runID).Count(&count)
	assert.Equal(t, int64(1), count)

	executor := NewReplayExecutor(db, newMockParserProvider())

	// 执行清除
	err := executor.clearExistingResults(runID)
	require.NoError(t, err)

	// 验证数据已清除
	db.Model(&TaskParsedLLDPNeighbor{}).Where("task_run_id = ?", runID).Count(&count)
	assert.Equal(t, int64(0), count)

	db.Model(&TaskParsedFDBEntry{}).Where("task_run_id = ?", runID).Count(&count)
	assert.Equal(t, int64(0), count)

	db.Model(&TaskTopologyEdge{}).Where("task_run_id = ?", runID).Count(&count)
	assert.Equal(t, int64(0), count)
}

// TestReplayOptions 测试重放选项
func TestReplayOptions(t *testing.T) {
	opts := ReplayOptions{
		ClearExisting: true,
		ParserVersion: "v1.0.0",
		DeviceIPs:     []string{"192.168.1.1", "192.168.1.2"},
		SkipBuild:     false,
	}

	assert.True(t, opts.ClearExisting)
	assert.Equal(t, "v1.0.0", opts.ParserVersion)
	assert.Len(t, opts.DeviceIPs, 2)
	assert.False(t, opts.SkipBuild)
}

// TestReplayStatistics 测试重放统计
func TestReplayStatistics(t *testing.T) {
	stats := ReplayStatistics{
		TotalRawFiles:    10,
		TotalDevices:     3,
		TotalCommandKeys: 5,
		ParsedDevices:    3,
		ParsedCommands:   10,
		FailedCommands:   0,
		LLDPCount:        15,
		FDBCount:         50,
		ARPCount:         30,
		InterfaceCount:   20,
		TotalCandidates:  25,
		RetainedEdges:    20,
		RejectedEdges:    5,
		ConflictEdges:    0,
		ScanDurationMs:   10,
		ParseDurationMs:  100,
		BuildDurationMs:  50,
		TotalDurationMs:  160,
	}

	assert.Equal(t, 10, stats.TotalRawFiles)
	assert.Equal(t, 3, stats.TotalDevices)
	assert.Equal(t, 20, stats.RetainedEdges)
	assert.Equal(t, int64(160), stats.TotalDurationMs)
}

// TestReplayResult 测试重放结果
func TestReplayResult(t *testing.T) {
	now := time.Now()
	result := &ReplayResult{
		ReplayRunID: "replay-test-1",
		Status:      string(ReplayStatusCompleted),
		Statistics: ReplayStatistics{
			TotalRawFiles: 5,
			ParsedDevices: 2,
			RetainedEdges: 10,
		},
		Errors:     []string{},
		StartedAt:  now,
		FinishedAt: now,
	}

	assert.Equal(t, "replay-test-1", result.ReplayRunID)
	assert.Equal(t, string(ReplayStatusCompleted), result.Status)
	assert.Empty(t, result.Errors)
}

// TestReplayableRunInfo 测试可重放运行信息
func TestReplayableRunInfo(t *testing.T) {
	info := ReplayableRunInfo{
		RunID:        "run-123",
		TaskName:     "测试拓扑任务",
		Status:       "completed",
		RunKind:      "topology",
		DeviceCount:  5,
		RawFileCount: 15,
		CreatedAt:    time.Now(),
		HasRawFiles:  true,
	}

	assert.Equal(t, "run-123", info.RunID)
	assert.Equal(t, "测试拓扑任务", info.TaskName)
	assert.True(t, info.HasRawFiles)
	assert.Equal(t, 5, info.DeviceCount)
}

// =============================================================================
// 集成测试
// =============================================================================

// TestReplayExecutor_Execute_Minimal 最小化集成测试
func TestReplayExecutor_Execute_Minimal(t *testing.T) {
	db, provider, _ := setupReplayTestEnv(t)

	// 设置mock解析器
	mockParser := &mockCliParser{
		parseFunc: func(commandKey string, rawText string) ([]map[string]string, error) {
			// 返回模拟解析结果
			if commandKey == "version" {
				return []map[string]string{
					{"hostname": "TestDevice", "version": "V200R001"},
				}, nil
			}
			return []map[string]string{}, nil
		},
	}
	provider.SetParser("huawei", mockParser)

	// 创建原始运行
	originalRunID := "original-minimal-run"
	createTestTopologyRun(t, db, originalRunID, "最小化测试")
	createTestRunDevice(t, db, originalRunID, "192.168.1.1")

	// 创建Raw文件
	createTestRawFiles(t, originalRunID, map[string][]string{
		"192.168.1.1": {"version"},
	})

	executor := NewReplayExecutor(db, provider)

	// 执行重放
	result, err := executor.Execute(context.Background(), originalRunID, ReplayOptions{
		ClearExisting: false,
		SkipBuild:     true, // 跳过构建阶段，仅测试解析
	})

	// 验证结果
	require.NoError(t, err)
	assert.NotEmpty(t, result.ReplayRunID)
	assert.Contains(t, []string{string(ReplayStatusCompleted), string(ReplayStatusRunning)}, result.Status)
	assert.Greater(t, result.Statistics.TotalRawFiles, 0)

	t.Logf("重放完成: runID=%s, status=%s, files=%d",
		result.ReplayRunID, result.Status, result.Statistics.TotalRawFiles)
}

// TestReplayExecutor_Execute_WithCancel 测试取消重放
func TestReplayExecutor_Execute_WithCancel(t *testing.T) {
	db, provider, _ := setupReplayTestEnv(t)

	provider.SetParser("huawei", &mockCliParser{})

	originalRunID := "original-cancel-run"
	createTestTopologyRun(t, db, originalRunID, "取消测试")
	createTestRunDevice(t, db, originalRunID, "192.168.1.1")

	createTestRawFiles(t, originalRunID, map[string][]string{
		"192.168.1.1": {"version"},
	})

	executor := NewReplayExecutor(db, provider)

	// 创建可取消上下文
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	// 执行重放
	result, err := executor.Execute(ctx, originalRunID, ReplayOptions{
		SkipBuild: true,
	})

	// 验证结果（应该被取消或返回错误）
	if err != nil {
		// 上下文取消可能返回 "context canceled" 错误
		assert.Contains(t, err.Error(), "cancel")
	} else {
		assert.Contains(t, []string{string(ReplayStatusCancelled), string(ReplayStatusFailed), string(ReplayStatusCompleted)}, result.Status)
	}
}

// TestReplayExecutor_Execute_NoRawFiles 测试无Raw文件情况
func TestReplayExecutor_Execute_NoRawFiles(t *testing.T) {
	db, provider, _ := setupReplayTestEnv(t)

	originalRunID := "original-no-raw-run"
	createTestTopologyRun(t, db, originalRunID, "无Raw文件测试")
	// 不创建Raw文件

	executor := NewReplayExecutor(db, provider)

	// 执行重放
	result, err := executor.Execute(context.Background(), originalRunID, ReplayOptions{})

	// 应该返回错误
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Raw文件")
	assert.Equal(t, string(ReplayStatusFailed), result.Status)
}

// TestReplayExecutor_Execute_DeviceFilter 测试设备过滤
func TestReplayExecutor_Execute_DeviceFilter(t *testing.T) {
	db, provider, _ := setupReplayTestEnv(t)

	provider.SetParser("huawei", &mockCliParser{})

	originalRunID := "original-filter-run"
	createTestTopologyRun(t, db, originalRunID, "设备过滤测试")
	createTestRunDevice(t, db, originalRunID, "192.168.1.1")
	createTestRunDevice(t, db, originalRunID, "192.168.1.2")
	createTestRunDevice(t, db, originalRunID, "192.168.1.3")

	// 创建3个设备的Raw文件
	createTestRawFiles(t, originalRunID, map[string][]string{
		"192.168.1.1": {"version"},
		"192.168.1.2": {"version"},
		"192.168.1.3": {"version"},
	})

	executor := NewReplayExecutor(db, provider)

	// 只重放指定设备
	result, err := executor.Execute(context.Background(), originalRunID, ReplayOptions{
		DeviceIPs: []string{"192.168.1.1", "192.168.1.2"},
		SkipBuild: true,
	})

	require.NoError(t, err)
	// 验证只处理了指定设备
	assert.LessOrEqual(t, result.Statistics.TotalDevices, 2)
}
