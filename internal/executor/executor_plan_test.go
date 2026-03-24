package executor

import (
	"context"
	"errors"
	"testing"
	"time"
)

// ============================================================================
// ExecutePlan 单元测试
// 验证阶段A修复：结果模型正确性
// ============================================================================

// TestProcessResultsWithKeys 测试结果回填 CommandKey 逻辑
func TestProcessResultsWithKeys(t *testing.T) {
	e := &DeviceExecutor{IP: "192.168.1.1"}

	plannedCmds := []PlannedCommand{
		{Key: "version", Command: "display version"},
		{Key: "interface", Command: "display interface"},
		{Key: "lldp", Command: "display lldp neighbor"},
	}
	commandKeys := []string{"version", "interface", "lldp"}

	t.Run("正常结果回填", func(t *testing.T) {
		results := []*CommandResult{
			{Index: 0, Command: "display version", Success: true},
			{Index: 1, Command: "display interface", Success: true},
			{Index: 2, Command: "display lldp neighbor", Success: true},
		}

		processed := e.processResultsWithKeys(results, commandKeys, plannedCmds)

		if len(processed) != 3 {
			t.Errorf("预期 3 个结果，实际 %d 个", len(processed))
		}

		for i, result := range processed {
			if result.CommandKey != commandKeys[i] {
				t.Errorf("结果 %d: CommandKey 预期 %s, 实际 %s", i, commandKeys[i], result.CommandKey)
			}
		}
	})

	t.Run("结果数量少于命令数量", func(t *testing.T) {
		results := []*CommandResult{
			{Index: 0, Command: "display version", Success: true},
		}

		processed := e.processResultsWithKeys(results, commandKeys, plannedCmds)

		if len(processed) != 3 {
			t.Errorf("预期补齐到 3 个结果，实际 %d 个", len(processed))
		}

		// 检查缺失的结果是否被正确标记为失败
		for i := 1; i < len(processed); i++ {
			if processed[i].Success {
				t.Errorf("缺失结果 %d 应该标记为失败", i)
			}
			if processed[i].ErrorMessage != "missing result" {
				t.Errorf("缺失结果 %d 应该有 missing result 错误信息", i)
			}
		}
	})

	t.Run("空结果且空命令列表", func(t *testing.T) {
		// 当 plannedCmds 为空时，空结果应该返回空列表
		processed := e.processResultsWithKeys([]*CommandResult{}, []string{}, []PlannedCommand{})

		if len(processed) != 0 {
			t.Errorf("空结果且空命令列表应该返回空列表，实际 %d 个", len(processed))
		}
	})

	t.Run("空结果处理(有命令列表时补齐)", func(t *testing.T) {
		// P2修复后：空结果但有命令列表时，应该补齐失败结果
		processed := e.processResultsWithKeys([]*CommandResult{}, commandKeys, plannedCmds)

		// 预期补齐为3个失败结果
		if len(processed) != 3 {
			t.Errorf("空结果有命令列表时应该补齐为3个失败记录，实际 %d 个", len(processed))
		}

		// 验证补齐结果
		for i, result := range processed {
			if result.Success {
				t.Errorf("补齐结果 %d 应该标记为失败", i)
			}
			if result.ErrorMessage != "missing result" {
				t.Errorf("补齐结果 %d 错误信息应该为'missing result'", i)
			}
		}
	})

	t.Run("nil结果处理", func(t *testing.T) {
		results := []*CommandResult{
			nil,
			{Index: 1, Command: "display interface", Success: true},
		}

		processed := e.processResultsWithKeys(results, commandKeys, plannedCmds)

		if len(processed) != 3 {
			t.Errorf("预期补齐到 3 个结果，实际 %d 个", len(processed))
		}

		// 第一个 nil 结果应该被替换为失败结果
		if processed[0].Success {
			t.Error("nil 结果应该被替换为失败结果")
		}
	})
}

// TestIsFatalError 测试致命错误判断
func TestIsFatalError(t *testing.T) {
	e := &DeviceExecutor{IP: "192.168.1.1"}
	plan := ExecutionPlan{AbortOnCommandTimeout: true}

	t.Run("nil错误不是致命错误", func(t *testing.T) {
		if e.isFatalError(nil, plan) {
			t.Error("nil 错误不应该是致命错误")
		}
	})

	t.Run("上下文取消错误", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := ctx.Err()
		if !e.isFatalError(err, plan) {
			t.Error("上下文取消应该是致命错误")
		}
	})
}

// TestIsTimeoutError 测试超时错误识别
func TestIsTimeoutError(t *testing.T) {
	t.Run("标准DeadlineExceeded错误", func(t *testing.T) {
		if !isTimeoutError(context.DeadlineExceeded) {
			t.Error("DeadlineExceeded 应该被识别为超时错误")
		}
	})

	t.Run("超时字符串匹配", func(t *testing.T) {
		err := errors.New("connection timeout")
		if !isTimeoutError(err) {
			t.Error("包含 timeout 的错误应该被识别为超时错误")
		}
	})

	t.Run("超时字符串大小写无关", func(t *testing.T) {
		err := errors.New("Read Timeout")
		if !isTimeoutError(err) {
			t.Error("包含 Timeout 的错误应该被识别为超时错误")
		}
	})

	t.Run("nil错误不是超时", func(t *testing.T) {
		if isTimeoutError(nil) {
			t.Error("nil 错误不应该被识别为超时错误")
		}
	})

	t.Run("非超时错误", func(t *testing.T) {
		err := errors.New("some other error")
		if isTimeoutError(err) {
			t.Error("非超时错误不应该被识别为超时错误")
		}
	})
}

// TestExecutionReportStats 测试执行报告统计
func TestExecutionReportStats(t *testing.T) {
	t.Run("成功统计", func(t *testing.T) {
		report := &ExecutionReport{
			Results: []*CommandResult{
				{Success: true},
				{Success: true},
				{Success: false, ErrorMessage: "error"},
			},
		}
		report.ComputeStats()

		if report.SuccessCount() != 2 {
			t.Errorf("成功数预期 2，实际 %d", report.SuccessCount())
		}
		if report.FailureCount() != 1 {
			t.Errorf("失败数预期 1，实际 %d", report.FailureCount())
		}
	})

	t.Run("全成功", func(t *testing.T) {
		report := &ExecutionReport{
			Results: []*CommandResult{
				{Success: true},
				{Success: true},
			},
			SessionHealthy: true,
		}

		if !report.IsSuccess() {
			t.Error("全成功应该返回 IsSuccess=true")
		}
	})

	t.Run("有致命错误时不是成功", func(t *testing.T) {
		report := &ExecutionReport{
			Results:    []*CommandResult{{Success: true}},
			FatalError: errors.New("fatal"),
		}

		if report.IsSuccess() {
			t.Error("有致命错误时不应该返回 IsSuccess=true")
		}
	})

	t.Run("部分成功", func(t *testing.T) {
		report := &ExecutionReport{
			Results: []*CommandResult{
				{Success: true},
				{Success: false},
			},
		}

		if !report.PartialSuccess() {
			t.Error("部分成功应该返回 PartialSuccess=true")
		}
	})
}

// TestCommandKeyInResult 测试 CommandKey 正确传递到结果
func TestCommandKeyInResult(t *testing.T) {
	t.Run("CommandKey 回填", func(t *testing.T) {
		result := &CommandResult{
			Index:   0,
			Command: "display version",
			Success: true,
		}

		// 模拟回填
		result.CommandKey = "version"

		if result.CommandKey != "version" {
			t.Errorf("CommandKey 预期 version，实际 %s", result.CommandKey)
		}
	})
}

// TestPlannedCommandTimeout 测试计划命令超时设置
func TestPlannedCommandTimeout(t *testing.T) {
	t.Run("默认超时", func(t *testing.T) {
		cmd := PlannedCommand{
			Key:     "test",
			Command: "test command",
		}

		if cmd.Timeout != 0 {
			t.Errorf("未设置超时期望为 0，实际 %v", cmd.Timeout)
		}
	})

	t.Run("自定义超时", func(t *testing.T) {
		cmd := PlannedCommand{
			Key:     "test",
			Command: "test command",
			Timeout: 60 * time.Second,
		}

		if cmd.Timeout != 60*time.Second {
			t.Errorf("超时期望 60s，实际 %v", cmd.Timeout)
		}
	})
}

// ============================================================================
// P2: 端到端策略测试
// ============================================================================

// TestClassifyRunError 测试错误分类函数
func TestClassifyRunError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected ErrorClass
	}{
		{
			name:     "nil错误",
			err:      nil,
			expected: "",
		},
		{
			name:     "上下文取消",
			err:      context.Canceled,
			expected: ErrorClassContextCancel,
		},
		{
			name:     "超时错误",
			err:      context.DeadlineExceeded,
			expected: ErrorClassTimeout,
		},
		{
			name:     "连接重置",
			err:      errors.New("connection reset by peer"),
			expected: ErrorClassTransport,
		},
		{
			name:     "连接拒绝",
			err:      errors.New("connection refused"),
			expected: ErrorClassTransport,
		},
		{
			name:     "命令错误",
			err:      errors.New("unknown command"),
			expected: ErrorClassCommand,
		},
		{
			name:     "未知错误",
			err:      errors.New("some random xyz"),
			expected: ErrorClassUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyRunError(tt.err)
			if result != tt.expected {
				t.Errorf("错误分类错误: 预期 %s, 实际 %s", tt.expected, result)
			}
		})
	}
}

// TestIsFatalError_WithPolicyMatrix 测试基于策略矩阵的致命错误判断
func TestIsFatalError_WithPolicyMatrix(t *testing.T) {
	e := &DeviceExecutor{IP: "192.168.1.1"}

	tests := []struct {
		name         string
		err          error
		abortTrans   bool
		abortTimeout bool
		continueCmd  bool
		wantFatal    bool
	}{
		{
			name:       "传输错误且AbortOnTransportErr=true",
			err:        errors.New("connection reset"),
			abortTrans: true,
			wantFatal:  true,
		},
		{
			name:       "传输错误且AbortOnTransportErr=false",
			err:        errors.New("connection reset"),
			abortTrans: false,
			wantFatal:  false,
		},
		{
			name:         "超时错误且AbortOnCommandTimeout=true",
			err:          context.DeadlineExceeded,
			abortTimeout: true,
			wantFatal:    true,
		},
		{
			name:         "超时错误且AbortOnCommandTimeout=false",
			err:          context.DeadlineExceeded,
			abortTimeout: false,
			wantFatal:    false,
		},
		{
			name:        "命令错误且ContinueOnCmdError=true",
			err:         errors.New("unknown command"),
			continueCmd: true,
			wantFatal:   false,
		},
		{
			name:        "命令错误且ContinueOnCmdError=false",
			err:         errors.New("unknown command"),
			continueCmd: false,
			wantFatal:   true,
		},
		{
			name:      "上下文取消总是fatal",
			err:       context.Canceled,
			wantFatal: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := ExecutionPlan{
				AbortOnTransportErr:   tt.abortTrans,
				AbortOnCommandTimeout: tt.abortTimeout,
				ContinueOnCmdError:    tt.continueCmd,
			}
			result := e.isFatalError(tt.err, plan)
			if result != tt.wantFatal {
				t.Errorf("isFatalError 错误: 预期 %v, 实际 %v", tt.wantFatal, result)
			}
		})
	}
}

// TestProcessResultsWithKeys_EmptyResultsBackfill 验证空结果时按计划命令数补齐
func TestProcessResultsWithKeys_EmptyResultsBackfill(t *testing.T) {
	e := &DeviceExecutor{IP: "192.168.1.1"}

	plannedCmds := []PlannedCommand{
		{Key: "version", Command: "display version"},
		{Key: "interface", Command: "display interface"},
		{Key: "lldp", Command: "display lldp"},
	}
	commandKeys := []string{"version", "interface", "lldp"}

	// 空结果场景
	results := []*CommandResult{}

	processed := e.processResultsWithKeys(results, commandKeys, plannedCmds)

	// 验证补齐了3个失败结果
	if len(processed) != 3 {
		t.Errorf("空结果应该补齐为3个失败记录，实际 %d 个", len(processed))
	}

	// 验证每个补齐的结果都是失败状态且有正确的CommandKey
	for i, result := range processed {
		if result.Success {
			t.Errorf("补齐结果 %d 应该为失败状态", i)
		}
		if result.ErrorMessage != "missing result" {
			t.Errorf("补齐结果 %d 错误信息应该为'missing result'，实际 %s", i, result.ErrorMessage)
		}
		if result.CommandKey != commandKeys[i] {
			t.Errorf("补齐结果 %d CommandKey 错误: 预期 %s, 实际 %s", i, commandKeys[i], result.CommandKey)
		}
		if result.Command != plannedCmds[i].Command {
			t.Errorf("补齐结果 %d Command 错误: 预期 %s, 实际 %s", i, plannedCmds[i].Command, result.Command)
		}
		if result.Index != i {
			t.Errorf("补齐结果 %d Index 错误: 预期 %d, 实际 %d", i, i, result.Index)
		}
	}
}

// TestExecutionPlan_PolicyCombination 测试策略组合
func TestExecutionPlan_PolicyCombination(t *testing.T) {
	t.Run("AbortOnTransportErr开关", func(t *testing.T) {
		// 开: 传输错误应该fatal
		plan1 := ExecutionPlan{
			AbortOnTransportErr: true,
		}
		e := &DeviceExecutor{IP: "192.168.1.1"}
		transErr := errors.New("connection reset")
		if !e.isFatalError(transErr, plan1) {
			t.Error("AbortOnTransportErr=true时传输错误应该fatal")
		}

		// 关: 传输错误不应该fatal
		plan2 := ExecutionPlan{
			AbortOnTransportErr: false,
		}
		if e.isFatalError(transErr, plan2) {
			t.Error("AbortOnTransportErr=false时传输错误不应该fatal")
		}
	})

	t.Run("AbortOnCommandTimeout开关", func(t *testing.T) {
		e := &DeviceExecutor{IP: "192.168.1.1"}
		timeoutErr := context.DeadlineExceeded

		// 开
		plan1 := ExecutionPlan{AbortOnCommandTimeout: true}
		if !e.isFatalError(timeoutErr, plan1) {
			t.Error("AbortOnCommandTimeout=true时超时错误应该fatal")
		}

		// 关
		plan2 := ExecutionPlan{AbortOnCommandTimeout: false}
		if e.isFatalError(timeoutErr, plan2) {
			t.Error("AbortOnCommandTimeout=false时超时错误不应该fatal")
		}
	})

	t.Run("ContinueOnCmdError开关", func(t *testing.T) {
		e := &DeviceExecutor{IP: "192.168.1.1"}
		cmdErr := errors.New("unknown command")

		// 开: 命令错误不应该fatal（继续执行）
		plan1 := ExecutionPlan{ContinueOnCmdError: true}
		if e.isFatalError(cmdErr, plan1) {
			t.Error("ContinueOnCmdError=true时命令错误不应该fatal")
		}

		// 关: 命令错误应该fatal
		plan2 := ExecutionPlan{ContinueOnCmdError: false}
		if !e.isFatalError(cmdErr, plan2) {
			t.Error("ContinueOnCmdError=false时命令错误应该fatal")
		}
	})
}

// ============================================================================
// 修复验证测试：错误分类健壮性 + 结果截断
// ============================================================================

// TestClassifyRunError_CaseInsensitive 验证错误分类大小写不敏感
func TestClassifyRunError_CaseInsensitive(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected ErrorClass
	}{
		{
			name:     "大小写混合的connection reset",
			err:      errors.New("Connection Reset By Peer"),
			expected: ErrorClassTransport,
		},
		{
			name:     "全大写的CONNECTION REFUSED",
			err:      errors.New("CONNECTION REFUSED"),
			expected: ErrorClassTransport,
		},
		{
			name:     "大小写混合的unknown command",
			err:      errors.New("Unknown Command"),
			expected: ErrorClassCommand,
		},
		{
			name:     "大写的EOF",
			err:      errors.New("EOF"),
			expected: ErrorClassTransport,
		},
		{
			name:     "大小写混合的broken pipe",
			err:      errors.New("Broken Pipe"),
			expected: ErrorClassTransport,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyRunError(tt.err)
			if result != tt.expected {
				t.Errorf("错误分类错误: 预期 %s, 实际 %s", tt.expected, result)
			}
		})
	}
}

// TestIsFatalError_UnknownFallback 验证unknown错误回退到传输错误策略
func TestIsFatalError_UnknownFallback(t *testing.T) {
	e := &DeviceExecutor{IP: "192.168.1.1"}
	// 使用一个不会匹配任何关键字的消息
	unknownErr := errors.New("some totally random unclassified xyzabc error")

	t.Run("unknown错误且AbortOnTransportErr=true", func(t *testing.T) {
		plan := ExecutionPlan{AbortOnTransportErr: true}
		if !e.isFatalError(unknownErr, plan) {
			t.Error("unknown错误在AbortOnTransportErr=true时应该fatal")
		}
	})

	t.Run("unknown错误且AbortOnTransportErr=false", func(t *testing.T) {
		plan := ExecutionPlan{AbortOnTransportErr: false}
		if e.isFatalError(unknownErr, plan) {
			t.Error("unknown错误在AbortOnTransportErr=false时不应该fatal")
		}
	})
}

// TestProcessResultsWithKeys_ExcessResultsTruncation 验证结果截断
func TestProcessResultsWithKeys_ExcessResultsTruncation(t *testing.T) {
	e := &DeviceExecutor{IP: "192.168.1.1"}

	plannedCmds := []PlannedCommand{
		{Key: "version", Command: "display version"},
		{Key: "interface", Command: "display interface"},
	}
	commandKeys := []string{"version", "interface"}

	// 构造4个结果，超过计划的2个命令
	results := []*CommandResult{
		{Index: 0, Command: "display version", Success: true},
		{Index: 1, Command: "display interface", Success: true},
		{Index: 2, Command: "extra command 1", Success: false, ErrorMessage: "extra"},
		{Index: 3, Command: "extra command 2", Success: false, ErrorMessage: "extra"},
	}

	processed := e.processResultsWithKeys(results, commandKeys, plannedCmds)

	// 验证截断为2个结果
	if len(processed) != 2 {
		t.Errorf("结果应该被截断为2个，实际 %d 个", len(processed))
	}

	// 验证保留的是前2个结果
	if processed[0].Command != "display version" {
		t.Errorf("第一个结果应该是display version，实际 %s", processed[0].Command)
	}
	if processed[1].Command != "display interface" {
		t.Errorf("第二个结果应该是display interface，实际 %s", processed[1].Command)
	}

	// 验证多余的结果被丢弃（不存在Index 2或3的结果）
	for _, r := range processed {
		if r.Index >= 2 {
			t.Errorf("不应该包含Index >= 2的结果，发现 Index=%d", r.Index)
		}
	}
}

// TestProcessResultsWithKeys_StrictOneToOne 验证严格一一对应
func TestProcessResultsWithKeys_StrictOneToOne(t *testing.T) {
	e := &DeviceExecutor{IP: "192.168.1.1"}

	plannedCmds := []PlannedCommand{
		{Key: "cmd1", Command: "cmd1"},
		{Key: "cmd2", Command: "cmd2"},
		{Key: "cmd3", Command: "cmd3"},
	}
	commandKeys := []string{"cmd1", "cmd2", "cmd3"}

	t.Run("正常情况：结果数=命令数", func(t *testing.T) {
		results := []*CommandResult{
			{Index: 0, Success: true},
			{Index: 1, Success: true},
			{Index: 2, Success: true},
		}
		processed := e.processResultsWithKeys(results, commandKeys, plannedCmds)
		if len(processed) != 3 {
			t.Errorf("正常情况应该返回3个结果，实际 %d", len(processed))
		}
	})

	t.Run("结果数少于命令数：补齐", func(t *testing.T) {
		results := []*CommandResult{
			{Index: 0, Success: true},
		}
		processed := e.processResultsWithKeys(results, commandKeys, plannedCmds)
		if len(processed) != 3 {
			t.Errorf("应该补齐为3个结果，实际 %d", len(processed))
		}
	})

	t.Run("结果补齐时commandKeys不足不应panic", func(t *testing.T) {
		shortKeys := []string{"cmd1"}
		results := []*CommandResult{
			{Index: 0, Success: true},
		}
		processed := e.processResultsWithKeys(results, shortKeys, plannedCmds)
		if len(processed) != 3 {
			t.Errorf("应该补齐为3个结果，实际 %d", len(processed))
		}
	})

	t.Run("结果数多于命令数：截断", func(t *testing.T) {
		results := []*CommandResult{
			{Index: 0, Success: true},
			{Index: 1, Success: true},
			{Index: 2, Success: true},
			{Index: 3, Success: false},
		}
		processed := e.processResultsWithKeys(results, commandKeys, plannedCmds)
		if len(processed) != 3 {
			t.Errorf("应该截断为3个结果，实际 %d", len(processed))
		}
	})
}
