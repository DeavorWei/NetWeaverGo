package smartping

// Severity 告警严重级别
type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityMajor    Severity = "major"
	SeverityMinor    Severity = "minor"
	SeverityInfo     Severity = "info"
)

// PingHostMetric 主机探测指标事实
type PingHostMetric struct {
	IP          string  `json:"ip"`
	HostName    string  `json:"hostName,omitempty"`
	Alive       bool    `json:"alive"`
	SentCount   int     `json:"sentCount"`
	RecvCount   int     `json:"recvCount"`
	FailedCount int     `json:"failedCount"`
	LossRate    float64 `json:"lossRate"`
	MinRtt      float64 `json:"minRtt"`
	MaxRtt      float64 `json:"maxRtt"`
	AvgRtt      float64 `json:"avgRtt"`
}

// SmartPingRule 智能 Ping 判定规则
type SmartPingRule struct {
	RuleID            string   `json:"ruleId"`
	Scene             string   `json:"scene"` // lan | wan | datacenter | leased_line | default | *
	Name              string   `json:"name"`
	Description       string   `json:"description"`
	MaxLossRate       float64  `json:"maxLossRate"`       // 丢包率门限 (%)，超出触发
	MaxAvgRTT         float64  `json:"maxAvgRtt"`         // 平均时延门限 (ms)，超出触发
	MaxJitter         float64  `json:"maxJitter"`         // 最大抖动门限 (ms, MaxRtt - MinRtt)，超出触发
	MaxFailedCount    int      `json:"maxFailedCount"`    // 失败包数门限，超出触发
	Severity          Severity `json:"severity"`          // critical | major | minor | info
	DiagnosisTemplate string   `json:"diagnosisTemplate"` // 诊断归因结论
	AdviceTemplate    string   `json:"adviceTemplate"`    // 处置建议
}

// AnomalyItem 异常项描述
type AnomalyItem struct {
	RuleID   string   `json:"ruleId"`
	Metric   string   `json:"metric"`   // loss_rate | avg_rtt | jitter | failed_count
	Actual   float64  `json:"actual"`   // 实际指标值
	Limit    float64  `json:"limit"`    // 阈值限制
	Severity Severity `json:"severity"` // 严重度
	Message  string   `json:"message"`  // 详细异常说明
}

// AnalysisReport 单目标智能 Ping 诊断报告
type AnalysisReport struct {
	IP          string        `json:"ip"`
	HostName    string        `json:"hostName,omitempty"`
	Scene       string        `json:"scene"`
	HealthScore int           `json:"healthScore"` // 健康度评分 (0 - 100)
	Status      string        `json:"status"`      // healthy | degraded | critical | unreachable
	LossRate    float64       `json:"lossRate"`
	AvgRTT      float64       `json:"avgRtt"`
	MinRTT      float64       `json:"minRtt"`
	MaxRTT      float64       `json:"maxRtt"`
	Jitter      float64       `json:"jitter"`
	Anomalies   []AnomalyItem `json:"anomalies"`
	RootCause   string        `json:"rootCause"` // 推断主要诱因
	Advice      string        `json:"advice"`    // 建议措施
}
