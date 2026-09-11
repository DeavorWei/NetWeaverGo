package metrics

import (
	"sync"
	"testing"
)

func TestRegistry_IncAndSnapshot(t *testing.T) {
	r := NewRegistry()
	r.Inc("run-1", "parse.template_hit", 3)
	r.Inc("run-1", "parse.template_hit", 2)
	r.LabelInc("run-1", "inspection.result_code", "TEST_PASS")

	snap := r.Snapshot("run-1")
	if snap.Counters["parse.template_hit"] != 5 {
		t.Fatalf("计数器应为 5，实际 %d", snap.Counters["parse.template_hit"])
	}
	if snap.Labels["inspection.result_code"]["TEST_PASS"] != 1 {
		t.Fatalf("label 计数应为 1")
	}
}

// 多任务并发：各自分桶，互不串扰（禁止全局区间差值的核心契约）
func TestRegistry_IsolatedByRunID(t *testing.T) {
	r := NewRegistry()
	r.Inc("run-a", "cache.hit", 1)
	r.Inc("run-b", "cache.hit", 7)

	if got := r.Snapshot("run-a").Counters["cache.hit"]; got != 1 {
		t.Fatalf("run-a 应为 1，实际 %d（发生串扰）", got)
	}
	if got := r.Snapshot("run-b").Counters["cache.hit"]; got != 7 {
		t.Fatalf("run-b 应为 7，实际 %d（发生串扰）", got)
	}
}

func TestRegistry_ConcurrentInc(t *testing.T) {
	r := NewRegistry()
	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.Inc("run-concurrent", "cache.hit", 1)
			r.LabelInc("run-concurrent", "risk.hit", "warn")
		}()
	}
	wg.Wait()

	if got := r.Snapshot("run-concurrent").Counters["cache.hit"]; got != 200 {
		t.Fatalf("并发累加应为 200，实际 %d", got)
	}
}

func TestRegistry_Release(t *testing.T) {
	r := NewRegistry()
	r.Inc("run-x", "cache.hit", 1)
	if r.ActiveRuns() != 1 {
		t.Fatalf("应存在 1 个分桶")
	}
	r.Release("run-x")
	if r.ActiveRuns() != 0 {
		t.Fatalf("Release 后应无分桶，实际 %d", r.ActiveRuns())
	}
	if len(r.Snapshot("run-x").Counters) != 0 {
		t.Fatal("Release 后快照应为空")
	}
}

func TestRegistry_LabelCardinalityBounded(t *testing.T) {
	r := NewRegistry()
	for i := 0; i < maxLabelsPerKey+50; i++ {
		r.LabelInc("run-y", "device.handler", labelName(i))
	}
	labels := r.Snapshot("run-y").Labels["device.handler"]
	if len(labels) > maxLabelsPerKey {
		t.Fatalf("label 基数越界: %d > %d", len(labels), maxLabelsPerKey)
	}
	if labels["__overflow__"] == 0 {
		t.Fatal("超出基数的标签应归入 __overflow__")
	}
}

func TestRegistry_EmptyRunIDIgnored(t *testing.T) {
	r := NewRegistry()
	r.Inc("", "cache.hit", 1)
	r.LabelInc("", "risk.hit", "warn")
	if r.ActiveRuns() != 0 {
		t.Fatal("空 runID 不应创建分桶")
	}
}

func labelName(i int) string {
	return "handler-" + itoa(i)
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var buf []byte
	for i > 0 {
		buf = append([]byte{byte('0' + i%10)}, buf...)
		i /= 10
	}
	return string(buf)
}
