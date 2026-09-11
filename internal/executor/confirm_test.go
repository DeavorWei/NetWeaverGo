package executor

import (
	"testing"

	"github.com/NetWeaverGo/core/internal/matcher"
)

func TestConfirmPrompt_TripleProtection(t *testing.T) {
	m := matcher.NewStreamMatcher()
	m.SetConfirmPatterns(matcher.DefaultConfirmPatterns)

	// 1. 正常行尾 [Y/N] 应该命中
	line1 := "Warning: The operation will take effect immediately. Continue? [Y/N]:"
	matched, prompt := m.CheckConfirmPrompt(line1)
	if !matched {
		t.Fatalf("期望命中确认提示符，但未命中: %s", line1)
	}
	if prompt == "" {
		t.Errorf("期望返回匹配的提示符文本")
	}

	// 2. 正常行尾 (yes/no) 应该命中
	line2 := "Do you want to proceed? (yes/no)"
	matched, _ = m.CheckConfirmPrompt(line2)
	if !matched {
		t.Fatalf("期望命中确认提示符: %s", line2)
	}

	// 3. 正文中间包含 [Y/N] 但行尾不是，不应该误触发（三重保险之行尾定界）
	line3 := "The option [Y/N] is used to configure port status on interface GigabitEthernet0/0/1"
	matched, _ = m.CheckConfirmPrompt(line3)
	if matched {
		t.Fatalf("正文中包含 [Y/N] 但非行尾，不应误触发: %s", line3)
	}

	// 4. 空文本不触发
	matched, _ = m.CheckConfirmPrompt("")
	if matched {
		t.Fatalf("空文本不应触发确认匹配")
	}
}

func TestConfirmPrompt_ReducerPolicy(t *testing.T) {
	m := matcher.NewStreamMatcher()
	m.SetConfirmPatterns(matcher.DefaultConfirmPatterns)

	// 场景 1: auto_yes 自动产生 ActAnswerConfirm Y
	{
		reducer := NewSessionReducer([]string{"reset saved-configuration"}, m)
		reducer.ctx.SetConfirmPolicy("auto_yes")
		// 模拟进入 Running
		reducer.state = NewStateRunning
		reducer.ctx.AdvanceCommand()

		batch := reducer.ReduceBatch(EvConfirmSeen{
			Prompt: "Continue? [Y/N]:",
		})

		if len(batch.Effects) != 1 {
			t.Fatalf("期望产生 1 个动作，实际产生 %d", len(batch.Effects))
		}
		act, ok := batch.Effects[0].(ActAnswerConfirm)
		if !ok {
			t.Fatalf("期望产生 ActAnswerConfirm，得到 %T", batch.Effects[0])
		}
		if string(act.AnswerBytes) != "Y\n" {
			t.Errorf("期望应答 Y\\n，实际得到 %q", string(act.AnswerBytes))
		}
	}

	// 场景 2: auto_no 自动产生 ActAnswerConfirm N
	{
		reducer := NewSessionReducer([]string{"reset saved-configuration"}, m)
		reducer.ctx.SetConfirmPolicy("auto_no")
		reducer.state = NewStateRunning
		reducer.ctx.AdvanceCommand()

		batch := reducer.ReduceBatch(EvConfirmSeen{
			Prompt: "Continue? [Y/N]:",
		})

		if len(batch.Effects) != 1 {
			t.Fatalf("期望产生 1 个动作，实际产生 %d", len(batch.Effects))
		}
		act, ok := batch.Effects[0].(ActAnswerConfirm)
		if !ok {
			t.Fatalf("期望产生 ActAnswerConfirm，得到 %T", batch.Effects[0])
		}
		if string(act.AnswerBytes) != "N\n" {
			t.Errorf("期望应答 N\\n，实际得到 %q", string(act.AnswerBytes))
		}
	}

	// 场景 3: ask_user 转入挂起并产生 ActRequestConfirmDecision
	{
		reducer := NewSessionReducer([]string{"reset saved-configuration"}, m)
		reducer.ctx.SetConfirmPolicy("ask_user")
		reducer.state = NewStateRunning
		reducer.ctx.AdvanceCommand()

		batch := reducer.ReduceBatch(EvConfirmSeen{
			Prompt: "Continue? [Y/N]:",
		})

		if reducer.State() != NewStateSuspended {
			t.Errorf("期望状态转入 NewStateSuspended，实际为 %s", reducer.State())
		}
		if len(batch.Effects) != 1 {
			t.Fatalf("期望产生 1 个动作，实际产生 %d", len(batch.Effects))
		}
		_, ok := batch.Effects[0].(ActRequestConfirmDecision)
		if !ok {
			t.Fatalf("期望产生 ActRequestConfirmDecision，得到 %T", batch.Effects[0])
		}
	}

	// 场景 4: off 策略忽略提示符，不挂起也不应答
	{
		reducer := NewSessionReducer([]string{"reset saved-configuration"}, m)
		reducer.ctx.SetConfirmPolicy("off")
		reducer.state = NewStateRunning
		reducer.ctx.AdvanceCommand()

		batch := reducer.ReduceBatch(EvConfirmSeen{
			Prompt: "Continue? [Y/N]:",
		})

		if reducer.State() != NewStateRunning {
			t.Errorf("期望状态维持 NewStateRunning，实际为 %s", reducer.State())
		}
		if len(batch.Effects) != 0 {
			t.Errorf("期望产生 0 个动作，实际产生 %d", len(batch.Effects))
		}
	}
}

func TestConfirmPrompt_Deduplication(t *testing.T) {
	m := matcher.NewStreamMatcher()
	m.SetConfirmPatterns(matcher.DefaultConfirmPatterns)

	reducer := NewSessionReducer([]string{"reset saved-configuration"}, m)
	reducer.ctx.SetConfirmPolicy("auto_yes")
	reducer.state = NewStateRunning
	reducer.ctx.AdvanceCommand()

	// 首次触发确认
	batch1 := reducer.ReduceBatch(EvConfirmSeen{Prompt: "Continue? [Y/N]:"})
	if len(batch1.Effects) != 1 {
		t.Fatalf("首次期望产生 1 个动作，实际产生 %d", len(batch1.Effects))
	}

	// 单命令生命周期内再次收到相同提示，应被有效去重，避免重复发送 Y\n
	batch2 := reducer.ReduceBatch(EvConfirmSeen{Prompt: "Continue? [Y/N]:"})
	if len(batch2.Effects) != 0 {
		t.Errorf("重复触发期望被去重（产生 0 动作），实际产生 %d", len(batch2.Effects))
	}
}

func TestConfirmPrompt_ReducerDoesNotTriggerOnPendingLines(t *testing.T) {
	m := matcher.NewStreamMatcher()
	m.SetConfirmPatterns(matcher.DefaultConfirmPatterns)

	reducer := NewSessionReducer([]string{"display current-configuration"}, m)
	reducer.ctx.SetConfirmPolicy("auto_yes")
	reducer.state = NewStateRunning
	reducer.ctx.AdvanceCommand()

	// 历史行或者已提交行中包含了看起来像确认提示符的行，但不应该在 processPendingLines 中触发确认动作
	batch := reducer.ReduceBatch(EvCommittedLine{
		Line: "Warning: The operation will take effect immediately. Continue? [Y/N]:",
	})

	for _, act := range batch.Effects {
		if _, ok := act.(ActAnswerConfirm); ok {
			t.Fatalf("processPendingLines 不应触发 ActAnswerConfirm，确认应严格由 EvConfirmSeen 驱动")
		}
		if _, ok := act.(ActRequestConfirmDecision); ok {
			t.Fatalf("processPendingLines 不应触发 ActRequestConfirmDecision，确认应严格由 EvConfirmSeen 驱动")
		}
	}
}

func TestSessionAdapter_RawBufferLimitEnforcement(t *testing.T) {
	m := matcher.NewStreamMatcher()
	adapter := NewSessionAdapter(80, []string{"disp cur"}, m)
	adapter.SetRawBufferLimitBytes(50) // 限制 50 字节

	ctx := adapter.newContext
	ctx.AdvanceCommand()

	// 喂入超过 50 字节的数据
	largeChunk := []byte("123456789012345678901234567890123456789012345678901234567890") // 60 bytes
	ctx.Current.AppendRawData(largeChunk)

	if len(ctx.Current.RawBuffer) > 50 {
		t.Fatalf("RawBuffer 大小超过了上限 50 字节: %d", len(ctx.Current.RawBuffer))
	}
	if !ctx.Current.Truncated {
		t.Fatalf("期望 Truncated 标志被置为 true")
	}
}


