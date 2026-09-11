// Package metrics 提供轻量、无外部依赖的运行指标采集能力。
//
// 设计约束（对应规划方案 §10.2）：
//   - 零外部依赖，仅使用 sync 原语；
//   - 按 RunID 分桶，禁止全局区间差值（多任务并发会串指标）；
//   - label 维度有界，禁止使用 IP/命令行等高基数维度。
package metrics

import "sync"

// maxLabelsPerKey 单个 label 指标允许的最大标签数，超出归入 __overflow__，防止基数爆炸
const maxLabelsPerKey = 64

// Snapshot 一次运行的指标快照（固定 Schema，供前端与报表稳定解析）
type Snapshot struct {
	Counters map[string]int64            `json:"counters"`
	Labels   map[string]map[string]int64 `json:"labels"`
}

// RunBucket 单个运行的指标桶，独立加锁，避免多任务互相争用
type RunBucket struct {
	mu       sync.Mutex
	counters map[string]int64
	labels   map[string]map[string]int64
}

func newRunBucket() *RunBucket {
	return &RunBucket{
		counters: make(map[string]int64),
		labels:   make(map[string]map[string]int64),
	}
}

// Inc 累加计数器
func (b *RunBucket) Inc(key string, delta int64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.counters[key] += delta
}

// LabelInc 累加带标签的计数器
func (b *RunBucket) LabelInc(key, label string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	labels, ok := b.labels[key]
	if !ok {
		labels = make(map[string]int64)
		b.labels[key] = labels
	}
	// 预留一个槽位给 __overflow__，保证单个 key 的标签总数不超过 maxLabelsPerKey
	if _, exists := labels[label]; !exists && len(labels) >= maxLabelsPerKey-1 {
		label = "__overflow__"
	}
	labels[label]++
}

// Snapshot 生成当前桶的不可变快照
func (b *RunBucket) Snapshot() Snapshot {
	b.mu.Lock()
	defer b.mu.Unlock()

	counters := make(map[string]int64, len(b.counters))
	for k, v := range b.counters {
		counters[k] = v
	}
	labels := make(map[string]map[string]int64, len(b.labels))
	for k, lv := range b.labels {
		inner := make(map[string]int64, len(lv))
		for ik, iv := range lv {
			inner[ik] = iv
		}
		labels[k] = inner
	}
	return Snapshot{Counters: counters, Labels: labels}
}

// Registry 运行指标注册表
type Registry struct {
	mu   sync.Mutex
	runs map[string]*RunBucket
}

// NewRegistry 创建注册表
func NewRegistry() *Registry {
	return &Registry{runs: make(map[string]*RunBucket)}
}

// Default 全局默认注册表
var Default = NewRegistry()

func (r *Registry) bucket(runID string) *RunBucket {
	r.mu.Lock()
	defer r.mu.Unlock()

	b, ok := r.runs[runID]
	if !ok {
		b = newRunBucket()
		r.runs[runID] = b
	}
	return b
}

// Inc 对指定运行累加计数器（runID 为空时静默忽略）
func (r *Registry) Inc(runID, key string, delta int64) {
	if runID == "" || key == "" {
		return
	}
	r.bucket(runID).Inc(key, delta)
}

// LabelInc 对指定运行累加带标签的计数器
func (r *Registry) LabelInc(runID, key, label string) {
	if runID == "" || key == "" {
		return
	}
	if label == "" {
		label = "unknown"
	}
	r.bucket(runID).LabelInc(key, label)
}

// Snapshot 获取指定运行的指标快照；不存在时返回空快照
func (r *Registry) Snapshot(runID string) Snapshot {
	if runID == "" {
		return Snapshot{}
	}
	r.mu.Lock()
	b, ok := r.runs[runID]
	r.mu.Unlock()
	if !ok || b == nil {
		return Snapshot{}
	}
	return b.Snapshot()
}

// Release 释放指定运行的指标桶，防止内存泄漏
func (r *Registry) Release(runID string) {
	if runID == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.runs, runID)
}

// ActiveRuns 返回当前存活的指标桶数量（便于测试与排障）
func (r *Registry) ActiveRuns() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.runs)
}
