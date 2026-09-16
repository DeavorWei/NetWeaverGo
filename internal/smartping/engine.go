package smartping

import (
	"embed"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"sync"
)

//go:embed rules/*.json
var rulesFS embed.FS

// Engine 智能 Ping 分析与诊断引擎
type Engine struct {
	mu    sync.RWMutex
	rules []*SmartPingRule
}

var (
	globalEngine *Engine
	engineOnce   sync.Once
)

// GetGlobalEngine 获取全局单例引擎
func GetGlobalEngine() *Engine {
	engineOnce.Do(func() {
		globalEngine = NewEngine()
		_ = globalEngine.LoadBuiltin()
	})
	return globalEngine
}

// NewEngine 创建引擎实例
func NewEngine() *Engine {
	return &Engine{
		rules: make([]*SmartPingRule, 0),
	}
}

// LoadBuiltin 加载内置规则库
func (e *Engine) LoadBuiltin() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	entries, err := rulesFS.ReadDir("rules")
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".json") {
			data, err := rulesFS.ReadFile("rules/" + entry.Name())
			if err != nil {
				continue
			}
			var list []SmartPingRule
			if err := json.Unmarshal(data, &list); err == nil {
				for i := range list {
					r := list[i]
					e.rules = append(e.rules, &r)
				}
			}
		}
	}
	return nil
}

// RegisterRule 注册或覆盖自定义规则
func (e *Engine) RegisterRule(rule *SmartPingRule) {
	if rule == nil || rule.RuleID == "" {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()

	for idx, r := range e.rules {
		if r.RuleID == rule.RuleID {
			e.rules[idx] = rule
			return
		}
	}
	e.rules = append(e.rules, rule)
}

// matchRule 为指定场景查找最佳适配规则
func (e *Engine) matchRule(scene string) *SmartPingRule {
	e.mu.RLock()
	defer e.mu.RUnlock()

	s := strings.ToLower(strings.TrimSpace(scene))
	if s == "" {
		s = "default"
	}

	// 1. 精确匹配场景
	for _, r := range e.rules {
		if strings.EqualFold(r.Scene, s) {
			return r
		}
	}

	// 2. 匹配 default
	for _, r := range e.rules {
		if strings.EqualFold(r.Scene, "default") {
			return r
		}
	}

	// 3. 兜底默认规则
	return &SmartPingRule{
		RuleID:            "FALLBACK-001",
		Scene:             "default",
		Name:              "兜底默认规则",
		MaxLossRate:       1.0,
		MaxAvgRTT:         80.0,
		MaxJitter:         30.0,
		MaxFailedCount:    1,
		Severity:          SeverityMajor,
		DiagnosisTemplate: "网络质量有轻微偏离",
		AdviceTemplate:    "建议检查网络连接",
	}
}

// Analyze 分析单台主机的探测结果并生成诊断报告
func (e *Engine) Analyze(res *PingHostMetric, scene string) *AnalysisReport {
	if res == nil {
		return nil
	}

	rule := e.matchRule(scene)
	usedScene := rule.Scene
	if scene != "" {
		usedScene = scene
	}

	report := &AnalysisReport{
		IP:          res.IP,
		HostName:    res.HostName,
		Scene:       usedScene,
		HealthScore: 100,
		Status:      "healthy",
		LossRate:    res.LossRate,
		AvgRTT:      res.AvgRtt,
		MinRTT:      res.MinRtt,
		MaxRTT:      res.MaxRtt,
		Jitter:      0,
		Anomalies:   make([]AnomalyItem, 0),
	}

	// 计算抖动 Jitter = MaxRTT - MinRTT
	if res.MinRtt >= 0 && res.MaxRtt >= res.MinRtt {
		report.Jitter = math.Round((res.MaxRtt-res.MinRtt)*100) / 100
	}

	// 1. 完全不可达判定
	if !res.Alive || res.LossRate >= 100.0 {
		report.HealthScore = 0
		report.Status = "unreachable"
		report.RootCause = "目标节点完全不响应 ICMP (离线或沿途核心阻断)"
		report.Advice = "检查对端设备电源、管理接口物理状态以及防火墙 ACL 策略"
		report.Anomalies = append(report.Anomalies, AnomalyItem{
			RuleID:   rule.RuleID,
			Metric:   "loss_rate",
			Actual:   100.0,
			Limit:    rule.MaxLossRate,
			Severity: SeverityCritical,
			Message:  "链路完全不可达，丢包率 100%",
		})
		return report
	}

	// 2. 丢包判定
	deduct := 0.0
	if res.LossRate > rule.MaxLossRate {
		deduct += res.LossRate * 1.5
		sev := rule.Severity
		if res.LossRate >= 10.0 {
			sev = SeverityCritical
		}
		report.Anomalies = append(report.Anomalies, AnomalyItem{
			RuleID:   rule.RuleID,
			Metric:   "loss_rate",
			Actual:   res.LossRate,
			Limit:    rule.MaxLossRate,
			Severity: sev,
			Message:  fmt.Sprintf("丢包率 %.1f%% 超过门限 %.1f%%", res.LossRate, rule.MaxLossRate),
		})
	}

	// 3. 时延判定
	if res.AvgRtt > rule.MaxAvgRTT {
		diff := res.AvgRtt - rule.MaxAvgRTT
		deduct += diff * 0.4
		report.Anomalies = append(report.Anomalies, AnomalyItem{
			RuleID:   rule.RuleID,
			Metric:   "avg_rtt",
			Actual:   res.AvgRtt,
			Limit:    rule.MaxAvgRTT,
			Severity: rule.Severity,
			Message:  fmt.Sprintf("平均往返时延 %.2f ms 超过门限 %.2f ms", res.AvgRtt, rule.MaxAvgRTT),
		})
	}

	// 4. 抖动判定
	if report.Jitter > rule.MaxJitter {
		diff := report.Jitter - rule.MaxJitter
		deduct += diff * 0.3
		report.Anomalies = append(report.Anomalies, AnomalyItem{
			RuleID:   rule.RuleID,
			Metric:   "jitter",
			Actual:   report.Jitter,
			Limit:    rule.MaxJitter,
			Severity: SeverityMinor,
			Message:  fmt.Sprintf("网络抖动 %.2f ms 超过门限 %.2f ms", report.Jitter, rule.MaxJitter),
		})
	}

	// 5. 失败包数判定
	if rule.MaxFailedCount >= 0 && res.FailedCount > rule.MaxFailedCount {
		deduct += float64(res.FailedCount) * 2.0
		report.Anomalies = append(report.Anomalies, AnomalyItem{
			RuleID:   rule.RuleID,
			Metric:   "failed_count",
			Actual:   float64(res.FailedCount),
			Limit:    float64(rule.MaxFailedCount),
			Severity: SeverityMinor,
			Message:  fmt.Sprintf("探测失败包数 %d 超过允许上限 %d", res.FailedCount, rule.MaxFailedCount),
		})
	}

	// 计算健康度
	finalScore := int(math.Max(0, math.Min(100, 100.0-deduct)))
	report.HealthScore = finalScore

	// 推断主要诱因与处置建议
	if len(report.Anomalies) == 0 {
		report.Status = "healthy"
		report.RootCause = "各项网络质量指标均在 SLA 合规基线内"
		report.Advice = "链路正常，保持周期性例行监控即可"
	} else {
		if finalScore < 60 {
			report.Status = "critical"
		} else {
			report.Status = "degraded"
		}

		// 智能根因推断
		if res.LossRate > 5.0 && report.Jitter > rule.MaxJitter {
			report.RootCause = "突发微突发流量引起交换机出队列拥塞或突发丢包"
			report.Advice = "建议检查骨干端口流量速率峰值，并在核心节点开启 QoS 队列保障"
		} else if res.LossRate > 0 && report.Jitter <= rule.MaxJitter {
			report.RootCause = "物理传输介质存在微弱光衰或误码衰减"
			report.Advice = "建议排查光模块光功率 (RxPower) 及光纤接头端面"
		} else if report.Jitter > rule.MaxJitter && res.LossRate == 0 {
			report.RootCause = "存在多路径 ECMP 负载分担不均或动态路由震荡"
			report.Advice = "检查路由下一跳平衡度及沿途路由器 CPU 调度开销"
		} else {
			report.RootCause = rule.DiagnosisTemplate
			report.Advice = rule.AdviceTemplate
		}
	}

	return report
}

// AnalyzeBatch 批量分析探测结果
func (e *Engine) AnalyzeBatch(results []*PingHostMetric, scene string) []*AnalysisReport {
	reports := make([]*AnalysisReport, len(results))
	for i, r := range results {
		reports[i] = e.Analyze(r, scene)
	}
	return reports
}
