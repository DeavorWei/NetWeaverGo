package matcher

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/NetWeaverGo/core/internal/logger"
)

// DiscrepancyRecord 差异记录
type DiscrepancyRecord struct {
	Timestamp time.Time `json:"timestamp"`
	Line      string    `json:"line"`
	OldHit    bool      `json:"oldHit"`
	OldRule   string    `json:"oldRule,omitempty"`
	NewHit    bool      `json:"newHit"`
	NewRule   string    `json:"newRule,omitempty"`
}

// ShadowMetrics 统计指标
type ShadowMetrics struct {
	TotalEvaluations uint64              `json:"totalEvaluations"`
	Discrepancies    uint64              `json:"discrepancies"`
	DiscrepancyRate  float64             `json:"discrepancyRate"`
	RecentSamples    []DiscrepancyRecord `json:"recentSamples"`
}

// ShadowMatcher 影子匹配器，用于在新旧策略并行期间收集裁决分歧
type ShadowMatcher struct {
	mu            sync.RWMutex
	enabled       bool
	oldRules      []ErrorRule
	newRules      []ErrorRule
	totalCount    uint64
	diffCount     uint64
	maxSamples    int
	recentSamples []DiscrepancyRecord
}

var (
	defaultShadowMatcher *ShadowMatcher
	shadowOnce           sync.Once
)

// GetDefaultShadowMatcher 获取全局影子匹配器单例
func GetDefaultShadowMatcher() *ShadowMatcher {
	shadowOnce.Do(func() {
		defaultShadowMatcher = NewShadowMatcher(DefaultRules, nil, false)
	})
	return defaultShadowMatcher
}

// NewShadowMatcher 创建影子匹配器
func NewShadowMatcher(oldRules, newRules []ErrorRule, enabled bool) *ShadowMatcher {
	if len(oldRules) == 0 {
		oldRules = DefaultRules
	}
	return &ShadowMatcher{
		enabled:       enabled,
		oldRules:      oldRules,
		newRules:      newRules,
		maxSamples:    100,
		recentSamples: make([]DiscrepancyRecord, 0, 100),
	}
}

// SetEnabled 启用或禁用影子模式
func (sm *ShadowMatcher) SetEnabled(enabled bool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.enabled = enabled
}

// IsEnabled 是否启用
func (sm *ShadowMatcher) IsEnabled() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.enabled
}

// SetNewRules 设置新策略规则
func (sm *ShadowMatcher) SetNewRules(rules []ErrorRule) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.newRules = rules
}

// MatchBoth 并行评估单行文本
func (sm *ShadowMatcher) MatchBoth(line string) (oldHit bool, oldRule *ErrorRule, newHit bool, newRule *ErrorRule, hasDiff bool) {
	atomic.AddUint64(&sm.totalCount, 1)

	// 旧规则匹配
	sm.mu.RLock()
	for i := range sm.oldRules {
		if sm.oldRules[i].Pattern.MatchString(line) {
			oldHit = true
			oldRule = &sm.oldRules[i]
			break
		}
	}

	// 新规则匹配
	for i := range sm.newRules {
		if sm.newRules[i].Pattern.MatchString(line) {
			newHit = true
			newRule = &sm.newRules[i]
			break
		}
	}
	sm.mu.RUnlock()

	hasDiff = (oldHit != newHit)
	if !hasDiff && oldHit && newHit {
		if oldRule.Name != newRule.Name || oldRule.Severity != newRule.Severity {
			hasDiff = true
		}
	}

	if hasDiff {
		atomic.AddUint64(&sm.diffCount, 1)

		oldName := ""
		if oldRule != nil {
			oldName = fmt.Sprintf("%s(%v)", oldRule.Name, oldRule.Severity)
		}
		newName := ""
		if newRule != nil {
			newName = fmt.Sprintf("%s(%v)", newRule.Name, newRule.Severity)
		}

		rec := DiscrepancyRecord{
			Timestamp: time.Now(),
			Line:      line,
			OldHit:    oldHit,
			OldRule:   oldName,
			NewHit:    newHit,
			NewRule:   newName,
		}

		sm.mu.Lock()
		if len(sm.recentSamples) >= sm.maxSamples {
			sm.recentSamples = sm.recentSamples[1:]
		}
		sm.recentSamples = append(sm.recentSamples, rec)
		sm.mu.Unlock()

		logger.Warn("MatcherShadow", "-", "规则裁决差异! 行: %q, 旧规则: %s, 新规则: %s", line, oldName, newName)
	}

	return oldHit, oldRule, newHit, newRule, hasDiff
}

// GetMetrics 获取影子模式统计指标
func (sm *ShadowMatcher) GetMetrics() ShadowMetrics {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	tot := atomic.LoadUint64(&sm.totalCount)
	diff := atomic.LoadUint64(&sm.diffCount)
	var rate float64
	if tot > 0 {
		rate = float64(diff) / float64(tot)
	}

	samples := make([]DiscrepancyRecord, len(sm.recentSamples))
	copy(samples, sm.recentSamples)

	return ShadowMetrics{
		TotalEvaluations: tot,
		Discrepancies:    diff,
		DiscrepancyRate:  rate,
		RecentSamples:    samples,
	}
}

// Reset 重置统计计数与采样
func (sm *ShadowMatcher) Reset() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	atomic.StoreUint64(&sm.totalCount, 0)
	atomic.StoreUint64(&sm.diffCount, 0)
	sm.recentSamples = make([]DiscrepancyRecord, 0, sm.maxSamples)
}
