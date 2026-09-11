package executor

import (
	"strings"
	"testing"
)

func TestCommandContext_8MBTruncate(t *testing.T) {
	ctx := NewCommandContext(0, "display big-output")
	ctx.SetCommand("display big-output")

	// 生成 1MB 的单行
	oneMBLine := strings.Repeat("A", 1024*1024)

	// 正常追加 7 行 (约 7MB)，不触发截断
	for i := 0; i < 7; i++ {
		ctx.AddNormalizedLine(oneMBLine)
		if ctx.Truncated {
			t.Fatalf("追加 7MB 内不应触发 Truncated 标记，第 %d 次", i+1)
		}
	}
	if len(ctx.NormalizedLines) != 7 {
		t.Fatalf("期望已保存 7 行，实际 %d 行", len(ctx.NormalizedLines))
	}

	// 再追加 2 行 (累加将超过 8MB MaxRawBufferSize)，必须触发 Truncated 并停止向内存追加
	ctx.AddNormalizedLine(oneMBLine) // 第 8 次 (约 8MB)
	ctx.AddNormalizedLine(oneMBLine) // 第 9 次 (超出 8MB)

	if !ctx.Truncated {
		t.Errorf("超出 8MB 警戒线后，必须将 Truncated 标记置为 true")
	}
	if len(ctx.NormalizedLines) > 8 {
		t.Errorf("超限后必须停止向 NormalizedLines 追加内容保全内存，当前行数: %d", len(ctx.NormalizedLines))
	}

	// 验证结果对象中的 Truncated 传递
	res := ctx.ToResult()
	if !res.Truncated {
		t.Errorf("ToResult 中未正确同步 Truncated 标记")
	}
}

func TestCommandContext_CachedFlag(t *testing.T) {
	ctx := NewCommandContext(1, "display version")
	ctx.SetCommand("display version")
	ctx.Cached = true

	res := ctx.ToResult()
	if !res.Cached {
		t.Errorf("ToResult 应当包含 Cached=true 标记")
	}
}
