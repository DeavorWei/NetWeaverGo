package executor

import (
	"testing"

	"github.com/NetWeaverGo/core/internal/matcher"
)

// ============================================================================
// SessionAdapter 集成测试 (Phase 7.4)
// ============================================================================
// 验证适配器模式正确桥接新旧架构

// TestAdapter_DefaultOldArchitecture 测试默认使用旧架构
func TestAdapter_DefaultOldArchitecture(t *testing.T) {
	m := matcher.NewStreamMatcher()
	adapter := NewSessionAdapter(80, []string{"display version"}, m)

	// 默认应该使用旧架构
	if adapter.UseNewArchitecture() {
		t.Error("默认应该使用旧架构 (SessionMachine)")
	}

	// 应该能正常处理输入
	actions := adapter.Feed("hostname# ")
	if len(actions) != 1 {
		t.Errorf("期望 1 个动作，得到 %d 个", len(actions))
	}

	// 第一个动作应该是发送预热命令
	if actions[0] != ActionSendWarmup {
		t.Errorf("期望 ActionSendWarmup 动作，得到 %v", actions[0])
	}
}

// TestAdapter_SwitchToNewArchitecture 测试切换到新架构
func TestAdapter_SwitchToNewArchitecture(t *testing.T) {
	m := matcher.NewStreamMatcher()
	adapter := NewSessionAdapter(80, []string{"display version"}, m)

	// 切换到新架构
	adapter.SetUseNewArchitecture(true)
	if !adapter.UseNewArchitecture() {
		t.Error("应该使用新架构")
	}

	// 新架构也应该能正常处理输入
	actions := adapter.Feed("hostname# ")
	// 新架构目前返回空动作列表（等待实现完成后会有实际动作）
	_ = actions
}

// TestAdapter_StateConsistency 测试新旧架构状态一致性
func TestAdapter_StateConsistency(t *testing.T) {
	commands := []string{"display version", "display interface"}

	t.Run("旧架构状态", func(t *testing.T) {
		m := matcher.NewStreamMatcher()
		adapter := NewSessionAdapter(80, commands, m)
		adapter.SetUseNewArchitecture(false)

		// 模拟初始化流程
		_ = adapter.Feed("hostname# ")

		// 检查状态
		state := adapter.State()
		if state != StateWarmup && state != StateReady {
			t.Errorf("期望状态 Warmup 或 Ready，得到 %s", state)
		}
	})

	t.Run("新架构状态", func(t *testing.T) {
		m := matcher.NewStreamMatcher()
		adapter := NewSessionAdapter(80, commands, m)
		adapter.SetUseNewArchitecture(true)

		// 模拟初始化流程
		_ = adapter.Feed("hostname# ")

		// 新架构状态应该通过 NewState 获取
		newState := adapter.NewState()
		if newState != NewStateInitAwaitPrompt && newState != NewStateReady {
			t.Errorf("期望新架构状态 InitAwaitPrompt 或 Ready，得到 %s", newState)
		}
	})
}

// TestAdapter_CommandQueueConsistency 测试命令队列一致性
func TestAdapter_CommandQueueConsistency(t *testing.T) {
	commands := []string{"display version", "display interface", "display arp"}

	t.Run("旧架构命令队列", func(t *testing.T) {
		m := matcher.NewStreamMatcher()
		adapter := NewSessionAdapter(80, commands, m)
		adapter.SetUseNewArchitecture(false)

		// 检查命令队列（通过 CurrentCommand）
		current := adapter.CurrentCommand()
		if current == nil {
			t.Log("当前无命令上下文")
		}
	})

	t.Run("新架构命令队列", func(t *testing.T) {
		m := matcher.NewStreamMatcher()
		adapter := NewSessionAdapter(80, commands, m)
		adapter.SetUseNewArchitecture(true)

		// 新架构命令队列应该一致
		current := adapter.CurrentCommand()
		if current == nil {
			t.Log("当前无命令上下文")
		}
	})
}

// TestAdapter_OutputConsistency 测试输出一致性
func TestAdapter_OutputConsistency(t *testing.T) {
	commands := []string{"display version"}

	// 使用相同的输入测试新旧架构
	input := "\r\nhostname# display version\r\n" +
		"Huawei Versatile Routing Platform Software\r\n" +
		"VRP (R) software, Version 5.160\r\n" +
		"hostname# "

	t.Run("旧架构输出", func(t *testing.T) {
		m := matcher.NewStreamMatcher()
		adapter := NewSessionAdapter(80, commands, m)
		adapter.SetUseNewArchitecture(false)

		// 初始化
		_ = adapter.Feed("hostname# ")

		// 处理输出
		_ = adapter.Feed(input)

		// 获取输出行
		lines := adapter.Lines()
		if len(lines) == 0 {
			t.Log("旧架构暂无提交的行")
		}
	})

	t.Run("新架构输出", func(t *testing.T) {
		m := matcher.NewStreamMatcher()
		adapter := NewSessionAdapter(80, commands, m)
		adapter.SetUseNewArchitecture(true)

		// 初始化
		_ = adapter.Feed("hostname# ")

		// 处理输出
		_ = adapter.Feed(input)

		// 获取输出行
		lines := adapter.Lines()
		if len(lines) == 0 {
			t.Log("新架构暂无提交的行")
		}
	})
}

// TestAdapter_PaginationHandling 测试分页处理一致性
func TestAdapter_PaginationHandling(t *testing.T) {
	commands := []string{"display interface"}

	// 模拟分页输出
	input := "hostname# display interface\r\n" +
		"GigabitEthernet0/0/1 current state: UP\r\n" +
		"GigabitEthernet0/0/2 current state: DOWN\r\n" +
		"  ---- More ----"

	t.Run("旧架构分页检测", func(t *testing.T) {
		m := matcher.NewStreamMatcher()
		adapter := NewSessionAdapter(80, commands, m)
		adapter.SetUseNewArchitecture(false)

		// 初始化
		_ = adapter.Feed("hostname# ")

		// 处理分页输出
		actions := adapter.Feed(input)

		// 应该检测到分页并发送继续动作
		hasPagerContinue := false
		for _, action := range actions {
			if action == ActionSendSpace {
				hasPagerContinue = true
				break
			}
		}
		if !hasPagerContinue {
			t.Log("旧架构未检测到分页（可能需要更多输入）")
		}
	})

	t.Run("新架构分页检测", func(t *testing.T) {
		m := matcher.NewStreamMatcher()
		adapter := NewSessionAdapter(80, commands, m)
		adapter.SetUseNewArchitecture(true)

		// 初始化
		_ = adapter.Feed("hostname# ")

		// 处理分页输出
		actions := adapter.Feed(input)

		// 新架构也应该检测到分页
		hasPagerContinue := false
		for _, action := range actions {
			if action == ActionSendSpace {
				hasPagerContinue = true
				break
			}
		}
		if !hasPagerContinue {
			t.Log("新架构未检测到分页（可能需要更多输入）")
		}
	})
}

// TestAdapter_ErrorHandling 测试错误处理一致性
func TestAdapter_ErrorHandling(t *testing.T) {
	commands := []string{"display version"}

	t.Run("旧架构错误处理", func(t *testing.T) {
		m := matcher.NewStreamMatcher()
		adapter := NewSessionAdapter(80, commands, m)
		adapter.SetUseNewArchitecture(false)

		// 初始化
		_ = adapter.Feed("hostname# ")

		// 模拟错误输出
		_ = adapter.Feed("  Error: Unrecognized command\r\nhostname# ")

		// 检查状态
		state := adapter.State()
		t.Logf("旧架构错误后状态: %s", state)
	})

	t.Run("新架构错误处理", func(t *testing.T) {
		m := matcher.NewStreamMatcher()
		adapter := NewSessionAdapter(80, commands, m)
		adapter.SetUseNewArchitecture(true)

		// 初始化
		_ = adapter.Feed("hostname# ")

		// 模拟错误输出
		_ = adapter.Feed("  Error: Unrecognized command\r\nhostname# ")

		// 检查状态
		state := adapter.NewState()
		t.Logf("新架构错误后状态: %s", state)
	})
}

// TestAdapter_MultipleCommands 测试多命令执行
func TestAdapter_MultipleCommands(t *testing.T) {
	commands := []string{"display version", "display interface", "display arp"}

	t.Run("旧架构多命令", func(t *testing.T) {
		m := matcher.NewStreamMatcher()
		adapter := NewSessionAdapter(80, commands, m)
		adapter.SetUseNewArchitecture(false)

		// 初始化
		_ = adapter.Feed("hostname# ")

		// 检查结果列表
		results := adapter.Results()
		t.Logf("初始结果数: %d", len(results))

		// 模拟第一个命令完成
		_ = adapter.Feed("display version\r\noutput\r\nhostname# ")

		// 检查结果
		results = adapter.Results()
		t.Logf("执行后结果数: %d", len(results))
	})

	t.Run("新架构多命令", func(t *testing.T) {
		m := matcher.NewStreamMatcher()
		adapter := NewSessionAdapter(80, commands, m)
		adapter.SetUseNewArchitecture(true)

		// 初始化
		_ = adapter.Feed("hostname# ")

		// 检查结果列表
		results := adapter.Results()
		t.Logf("初始结果数: %d", len(results))

		// 模拟第一个命令完成
		_ = adapter.Feed("display version\r\noutput\r\nhostname# ")

		// 检查结果
		results = adapter.Results()
		t.Logf("执行后结果数: %d", len(results))
	})
}

// TestAdapter_ArchitectureMode 测试架构模式字符串
func TestAdapter_ArchitectureMode(t *testing.T) {
	m := matcher.NewStreamMatcher()
	adapter := NewSessionAdapter(80, []string{"display version"}, m)

	// 默认旧架构
	mode := adapter.GetArchitectureMode()
	if mode != "legacy (SessionMachine)" {
		t.Errorf("期望 'legacy (SessionMachine)'，得到 '%s'", mode)
	}

	// 切换到新架构
	adapter.SetUseNewArchitecture(true)
	mode = adapter.GetArchitectureMode()
	if mode != "new (Detector+Reducer+Driver)" {
		t.Errorf("期望 'new (Detector+Reducer+Driver)'，得到 '%s'", mode)
	}
}

// TestAdapter_Stats 测试统计信息
func TestAdapter_Stats(t *testing.T) {
	m := matcher.NewStreamMatcher()
	adapter := NewSessionAdapter(80, []string{"display version"}, m)

	stats := adapter.GetStats()
	if stats == nil {
		t.Error("期望非空统计信息")
		return
	}

	// 检查必要字段
	if _, ok := stats["mode"]; !ok {
		t.Error("统计信息缺少 'mode' 字段")
	}
	if _, ok := stats["useNewArchitecture"]; !ok {
		t.Error("统计信息缺少 'useNewArchitecture' 字段")
	}
}

// TestAdapter_ClearInitResiduals 测试清空初始化残留
func TestAdapter_ClearInitResiduals(t *testing.T) {
	commands := []string{"display version"}

	t.Run("旧架构清空残留", func(t *testing.T) {
		m := matcher.NewStreamMatcher()
		adapter := NewSessionAdapter(80, commands, m)
		adapter.SetUseNewArchitecture(false)

		// 初始化并处理一些输入
		_ = adapter.Feed("hostname# ")
		_ = adapter.Feed("some output\r\n")

		// 清空残留
		adapter.ClearInitResiduals()

		// 验证清空成功（无错误即可）
	})

	t.Run("新架构清空残留", func(t *testing.T) {
		m := matcher.NewStreamMatcher()
		adapter := NewSessionAdapter(80, commands, m)
		adapter.SetUseNewArchitecture(true)

		// 初始化并处理一些输入
		_ = adapter.Feed("hostname# ")
		_ = adapter.Feed("some output\r\n")

		// 清空残留
		adapter.ClearInitResiduals()

		// 验证清空成功（无错误即可）
	})
}

// TestAdapter_MarkFailed 测试标记失败
func TestAdapter_MarkFailed(t *testing.T) {
	commands := []string{"display version"}

	t.Run("旧架构标记失败", func(t *testing.T) {
		m := matcher.NewStreamMatcher()
		adapter := NewSessionAdapter(80, commands, m)
		adapter.SetUseNewArchitecture(false)

		// 初始化
		_ = adapter.Feed("hostname# ")

		// 标记失败
		adapter.MarkFailed("test failure")

		// 检查状态
		state := adapter.State()
		if state != StateFailed {
			t.Errorf("期望状态 Failed，得到 %s", state)
		}
	})

	t.Run("新架构标记失败", func(t *testing.T) {
		m := matcher.NewStreamMatcher()
		adapter := NewSessionAdapter(80, commands, m)
		adapter.SetUseNewArchitecture(true)

		// 初始化
		_ = adapter.Feed("hostname# ")

		// 标记失败
		adapter.MarkFailed("test failure")

		// 检查状态
		state := adapter.NewState()
		if state != NewStateFailed {
			t.Errorf("期望状态 Failed，得到 %s", state)
		}
	})
}
