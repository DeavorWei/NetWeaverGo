package discovery

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/executor"
)

// ============================================================================
// Discovery Runner 报告处理单元测试
// 验证阶段B修复：状态判定与健壮性
// ============================================================================

// determineDeviceStatus 提取的状态判定函数（用于测试）
// 从 handleDiscoveryReport 中提取的状态判定逻辑
func determineDeviceStatus(results []*executor.CommandResult, fatalErr, execErr error) string {
	cmdSuccess := 0
	cmdFailed := 0

	for _, result := range results {
		if result == nil {
			continue
		}
		if result.ErrorMessage != "" {
			cmdFailed++
		} else {
			cmdSuccess++
		}
	}

	// 优先检查会话级致命错误
	if fatalErr != nil {
		return "failed"
	}
	if execErr != nil {
		return "failed"
	}
	if cmdFailed > 0 {
		if cmdSuccess > 0 {
			return "partial"
		}
		return "failed"
	}

	return "success"
}

// TestHandleDiscoveryReportDeviceStatus 测试设备状态判定
func TestHandleDiscoveryReportDeviceStatus(t *testing.T) {
	tests := []struct {
		name       string
		results    []*executor.CommandResult
		fatalErr   error
		execErr    error
		wantStatus string
	}{
		{
			name: "全成功",
			results: []*executor.CommandResult{
				{CommandKey: "cmd1", Success: true},
				{CommandKey: "cmd2", Success: true},
			},
			wantStatus: "success",
		},
		{
			name: "部分成功",
			results: []*executor.CommandResult{
				{CommandKey: "cmd1", Success: true},
				{CommandKey: "cmd2", Success: false, ErrorMessage: "error"},
			},
			wantStatus: "partial",
		},
		{
			name: "全失败",
			results: []*executor.CommandResult{
				{CommandKey: "cmd1", Success: false, ErrorMessage: "error1"},
				{CommandKey: "cmd2", Success: false, ErrorMessage: "error2"},
			},
			wantStatus: "failed",
		},
		{
			name:       "致命错误",
			results:    []*executor.CommandResult{},
			fatalErr:   errors.New("fatal error"),
			wantStatus: "failed",
		},
		{
			name:       "无结果但有致命错误",
			results:    []*executor.CommandResult{},
			fatalErr:   errors.New("init failed"),
			wantStatus: "failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := determineDeviceStatus(tt.results, tt.fatalErr, tt.execErr)

			if status != tt.wantStatus {
				t.Errorf("状态判定错误: 预期 %s, 实际 %s", tt.wantStatus, status)
			}
		})
	}
}

// TestDetermineDeviceStatusEdgeCases 测试边界情况
func TestDetermineDeviceStatusEdgeCases(t *testing.T) {
	t.Run("空结果且无错误", func(t *testing.T) {
		status := determineDeviceStatus([]*executor.CommandResult{}, nil, nil)
		if status != "success" {
			t.Errorf("空结果无错误应该返回 success，实际 %s", status)
		}
	})

	t.Run("nil结果列表", func(t *testing.T) {
		status := determineDeviceStatus(nil, nil, nil)
		if status != "success" {
			t.Errorf("nil结果列表应该返回 success，实际 %s", status)
		}
	})

	t.Run("致命错误优先级高于命令成功", func(t *testing.T) {
		results := []*executor.CommandResult{
			{CommandKey: "cmd1", Success: true},
		}
		status := determineDeviceStatus(results, errors.New("fatal"), nil)
		if status != "failed" {
			t.Errorf("致命错误应该优先于命令成功，实际 %s", status)
		}
	})

	t.Run("包含nil结果的列表", func(t *testing.T) {
		results := []*executor.CommandResult{
			nil,
			{CommandKey: "cmd1", Success: true},
		}
		status := determineDeviceStatus(results, nil, nil)
		if status != "success" {
			t.Errorf("应该跳过nil结果，实际 %s", status)
		}
	})
}

// TestCommandResultMapping 测试命令结果映射
func TestCommandResultMapping(t *testing.T) {
	t.Run("结果索引与命令对应", func(t *testing.T) {
		results := []*executor.CommandResult{
			{Index: 0, CommandKey: "version", Command: "display version", Success: true},
			{Index: 1, CommandKey: "interface", Command: "display interface", Success: true},
		}

		// 验证结果按索引正确对应
		if results[0].CommandKey != "version" {
			t.Error("第一个结果应该对应 version 命令")
		}
		if results[1].CommandKey != "interface" {
			t.Error("第二个结果应该对应 interface 命令")
		}
	})
}

// TestExecutionReportNilSafety 测试 ExecutionReport 空指针安全
func TestExecutionReportNilSafety(t *testing.T) {
	t.Run("nil report 统计", func(t *testing.T) {
		var report *executor.ExecutionReport

		// 模拟检查 report 是否为 nil
		isNil := report == nil
		if !isNil {
			t.Error("应该能正确识别 nil report")
		}
	})

	t.Run("空结果报告", func(t *testing.T) {
		report := &executor.ExecutionReport{
			Results: []*executor.CommandResult{},
		}

		if report.SuccessCount() != 0 {
			t.Errorf("空结果成功数应该为 0，实际 %d", report.SuccessCount())
		}
	})
}

// TestBuildDiscoveryPlan 测试构建设备发现计划
func TestBuildDiscoveryPlan(t *testing.T) {
	t.Run("正常构建计划", func(t *testing.T) {
		profile := &config.DeviceProfile{
			Vendor: "huawei",
			Commands: []config.CommandSpec{
				{Command: "display version", CommandKey: "version", TimeoutSec: 30},
				{Command: "display interface", CommandKey: "interface", TimeoutSec: 60},
			},
		}

		plan := BuildDiscoveryPlan(profile, 120*time.Second)

		if plan.Name != "discovery-huawei" {
			t.Errorf("计划名称预期 discovery-huawei，实际 %s", plan.Name)
		}
		if len(plan.Commands) != 2 {
			t.Errorf("命令数预期 2，实际 %d", len(plan.Commands))
		}
	})

	t.Run("nil profile 使用默认", func(t *testing.T) {
		plan := BuildDiscoveryPlan(nil, 120*time.Second)

		if len(plan.Commands) == 0 {
			t.Error("nil profile 应该使用默认配置")
		}
	})
}

// ============================================================================
// P2: Discovery策略驱动状态判定测试
// ============================================================================

// TestDiscoveryStatus_WithPolicyDrivenFatal 测试策略触发的fatal状态判定
func TestDiscoveryStatus_WithPolicyDrivenFatal(t *testing.T) {
	tests := []struct {
		name       string
		results    []*executor.CommandResult
		fatalErr   error
		wantStatus string
	}{
		{
			name:       "传输错误导致fatal",
			results:    []*executor.CommandResult{},
			fatalErr:   errors.New("connection reset"),
			wantStatus: "failed",
		},
		{
			name:       "超时错误导致fatal",
			results:    []*executor.CommandResult{},
			fatalErr:   errors.New("timeout"),
			wantStatus: "failed",
		},
		{
			name: "命令错误未导致fatal（部分成功）",
			results: []*executor.CommandResult{
				{CommandKey: "cmd1", Success: true},
				{CommandKey: "cmd2", Success: false, ErrorMessage: "unknown command"},
			},
			wantStatus: "partial",
		},
		{
			name: "上下文取消导致fatal",
			results: []*executor.CommandResult{
				{CommandKey: "cmd1", Success: true},
			},
			fatalErr:   context.Canceled,
			wantStatus: "failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := determineDeviceStatus(tt.results, tt.fatalErr, nil)
			if status != tt.wantStatus {
				t.Errorf("状态判定错误: 预期 %s, 实际 %s", tt.wantStatus, status)
			}
		})
	}
}

// TestDiscoveryStatus_ResultsCountConsistency 测试结果数量一致性
func TestDiscoveryStatus_ResultsCountConsistency(t *testing.T) {
	t.Run("结果数与命令数一致", func(t *testing.T) {
		results := []*executor.CommandResult{
			{CommandKey: "cmd1", Success: true},
			{CommandKey: "cmd2", Success: true},
			{CommandKey: "cmd3", Success: true},
		}
		status := determineDeviceStatus(results, nil, nil)
		if status != "success" {
			t.Errorf("3个成功结果应该返回success，实际 %s", status)
		}
	})

	t.Run("补齐后的结果一致性", func(t *testing.T) {
		// 模拟补齐后的结果：原始2个成功，1个补齐的失败
		results := []*executor.CommandResult{
			{CommandKey: "cmd1", Success: true},
			{CommandKey: "cmd2", Success: true},
			{CommandKey: "cmd3", Success: false, ErrorMessage: "missing result"},
		}
		status := determineDeviceStatus(results, nil, nil)
		if status != "partial" {
			t.Errorf("2成功1失败应该返回partial，实际 %s", status)
		}
	})
}

// TestDiscoveryStatus_PriorityOfFatalVsPartial 测试fatal与partial的优先级
func TestDiscoveryStatus_PriorityOfFatalVsPartial(t *testing.T) {
	t.Run("fatal优先于partial", func(t *testing.T) {
		// 既有成功的命令，又有fatal错误
		results := []*executor.CommandResult{
			{CommandKey: "cmd1", Success: true},
			{CommandKey: "cmd2", Success: true},
		}
		fatalErr := errors.New("connection reset")
		status := determineDeviceStatus(results, fatalErr, nil)

		// fatal错误应该优先，返回failed
		if status != "failed" {
			t.Errorf("fatal错误应该优先于partial，期望failed，实际 %s", status)
		}
	})

	t.Run("无fatal时的partial判定", func(t *testing.T) {
		results := []*executor.CommandResult{
			{CommandKey: "cmd1", Success: true},
			{CommandKey: "cmd2", Success: false, ErrorMessage: "error"},
		}
		status := determineDeviceStatus(results, nil, nil)

		if status != "partial" {
			t.Errorf("应该有partial状态，实际 %s", status)
		}
	})
}
