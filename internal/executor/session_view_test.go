package executor

import (
	"testing"

	"github.com/NetWeaverGo/core/internal/matcher"
)

// 验证视图状态收敛在会话级 SessionContext（跨命令继承），
// 且命令上下文保存的是下发时刻的视图快照。
func TestSessionContext_CurrentViewPersistsAcrossCommands(t *testing.T) {
	ctx := NewSessionContext([]string{"display version", "system-view", "display this"})

	// 初始状态归一化为 unknown
	if got := ctx.GetCurrentView(); got != matcher.ViewUnknown {
		t.Fatalf("初始视图应为 unknown，实际 %q", got)
	}

	// 预热提示符初始化视图
	ctx.SetCurrentView(matcher.ResolveView("huawei", "<SW1>"))
	if got := ctx.GetCurrentView(); got != matcher.ViewUser {
		t.Fatalf("预热后视图应为 user，实际 %q", got)
	}

	// 第一条命令：快照应为 user
	first := ctx.AdvanceCommand()
	if first == nil {
		t.Fatal("第一条命令上下文不应为空")
	}
	if first.View != matcher.ViewUser {
		t.Fatalf("第一条命令视图快照应为 user，实际 %q", first.View)
	}

	// 视图切换到系统视图后，第二条命令快照应更新，且状态跨命令保持
	ctx.SetCurrentView(matcher.ResolveView("huawei", "[SW1]"))
	second := ctx.AdvanceCommand()
	if second == nil {
		t.Fatal("第二条命令上下文不应为空")
	}
	if second.View != matcher.ViewSystem {
		t.Fatalf("第二条命令视图快照应为 system，实际 %q", second.View)
	}

	// 第三条命令仍继承 system（验证跨命令继承，而非单命令即焚）
	third := ctx.AdvanceCommand()
	if third == nil {
		t.Fatal("第三条命令上下文不应为空")
	}
	if third.View != matcher.ViewSystem {
		t.Fatalf("第三条命令应继承 system 视图，实际 %q", third.View)
	}
}

// 验证视图维度参与缓存键组装：同命令不同视图互不复用
func TestStreamEngine_CacheKeyIsViewScoped(t *testing.T) {
	adapter := NewSessionAdapter(80, []string{"display this"}, matcher.NewStreamMatcher())
	engine := &StreamEngine{adapter: adapter}
	ctx := adapter.reducer.Context()

	ctx.SetCurrentView(matcher.ViewInterface)
	keyInterface := engine.currentCacheKey("display this")

	ctx.SetCurrentView(matcher.ViewRouting)
	keyRouting := engine.currentCacheKey("display this")

	if keyInterface == keyRouting {
		t.Fatalf("不同视图的缓存键必须不同，实际都是 %q", keyInterface)
	}

	cache := DefaultCommandCache()
	cache.Put(keyInterface, &CommandResult{Command: "display this"})
	if _, found := cache.Get(keyRouting); found {
		t.Fatal("跨视图不应命中同一命令的缓存")
	}
	if _, found := cache.Get(keyInterface); !found {
		t.Fatal("同视图同命令应命中缓存")
	}
}
