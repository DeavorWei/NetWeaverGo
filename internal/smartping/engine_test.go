package smartping

import (
	"testing"
)

func TestSmartPingEngine_LoadBuiltin(t *testing.T) {
	engine := NewEngine()
	err := engine.LoadBuiltin()
	if err != nil {
		t.Fatalf("加载内置规则失败: %v", err)
	}

	if len(engine.rules) < 5 {
		t.Errorf("预期内置规则数 >= 5，实际: %d", len(engine.rules))
	}
}

func TestSmartPingEngine_Analyze_Healthy(t *testing.T) {
	engine := GetGlobalEngine()

	res := &PingHostMetric{
		IP:        "192.168.1.1",
		Alive:     true,
		SentCount: 10,
		RecvCount: 10,
		LossRate:  0.0,
		MinRtt:    1.2,
		MaxRtt:    2.5,
		AvgRtt:    1.8,
	}

	report := engine.Analyze(res, "lan")
	if report.Status != "healthy" {
		t.Errorf("预期 healthy，实际: %s", report.Status)
	}
	if report.HealthScore != 100 {
		t.Errorf("预期健康分 100，实际: %d", report.HealthScore)
	}
	if len(report.Anomalies) != 0 {
		t.Errorf("健康主机不应产生异常项")
	}
}

func TestSmartPingEngine_Analyze_Unreachable(t *testing.T) {
	engine := GetGlobalEngine()

	res := &PingHostMetric{
		IP:        "192.168.1.254",
		Alive:     false,
		SentCount: 4,
		RecvCount: 0,
		LossRate:  100.0,
		MinRtt:    -1,
		MaxRtt:    0,
		AvgRtt:    0,
	}

	report := engine.Analyze(res, "lan")
	if report.Status != "unreachable" {
		t.Errorf("预期 unreachable，实际: %s", report.Status)
	}
	if report.HealthScore != 0 {
		t.Errorf("预期健康分 0，实际: %d", report.HealthScore)
	}
}

func TestSmartPingEngine_Analyze_DatacenterJitterAndLoss(t *testing.T) {
	engine := GetGlobalEngine()

	// 数据中心场景：maxLossRate = 0%, maxAvgRtt = 3ms, maxJitter = 2ms
	res := &PingHostMetric{
		IP:        "10.200.1.10",
		Alive:     true,
		SentCount: 20,
		RecvCount: 18,
		LossRate:  10.0, // 丢包
		MinRtt:    1.0,
		MaxRtt:    8.5,  // Jitter = 7.5 > 2.0
		AvgRtt:    4.2,  // > 3.0
	}

	report := engine.Analyze(res, "datacenter")
	if report.Status != "critical" && report.Status != "degraded" {
		t.Errorf("预期异常状态，实际: %s", report.Status)
	}
	if report.HealthScore >= 90 {
		t.Errorf("健康分应显著扣减，实际: %d", report.HealthScore)
	}
	if len(report.Anomalies) < 2 {
		t.Errorf("预期至少发现丢包和时延/抖动异常，实际: %d", len(report.Anomalies))
	}
}

func TestSmartPingEngine_CustomRule(t *testing.T) {
	engine := NewEngine()

	customRule := &SmartPingRule{
		RuleID:            "CUSTOM-001",
		Scene:             "ultra_strict",
		Name:              "极度严苛场景",
		MaxLossRate:       0.0,
		MaxAvgRTT:         1.0,
		MaxJitter:         0.5,
		MaxFailedCount:    0,
		Severity:          SeverityCritical,
		DiagnosisTemplate: "极严场景时延抖动超标",
		AdviceTemplate:    "调优内核参数",
	}
	engine.RegisterRule(customRule)

	res := &PingHostMetric{
		IP:        "127.0.0.1",
		Alive:     true,
		SentCount: 5,
		RecvCount: 5,
		LossRate:  0.0,
		MinRtt:    0.2,
		MaxRtt:    1.5,
		AvgRtt:    1.2, // > 1.0
	}

	report := engine.Analyze(res, "ultra_strict")
	if len(report.Anomalies) == 0 {
		t.Errorf("自定义极严场景应捕获异常")
	}
}
