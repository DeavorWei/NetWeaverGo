package optical

import (
	"testing"

	"github.com/NetWeaverGo/core/internal/models"
)

func TestChecker_Normal(t *testing.T) {
	checker := NewChecker()
	metric := TransceiverMetric{
		DeviceIP:        "192.168.1.1",
		Port:            "10GE1/0/1",
		Vendor:          "huawei",
		TransceiverType: "10GE-LR",
		TxPower:         -2.5,
		RxPower:         -8.2,
		Temperature:     42.0,
	}

	res := checker.Check(metric)
	if res.Level != LevelNormal {
		t.Errorf("预期 normal，实际 %s, issues=%v", res.Level, res.Issues)
	}
	if res.IsWeakOptical || res.IsLOS {
		t.Errorf("不应判定为弱光或断光")
	}
}

func TestChecker_WeakOptical_Absolute(t *testing.T) {
	checker := NewChecker()

	// 规则默认 MinRxPower = -18.0
	// 1. -19.5 dBm: 轻度弱光 warning
	resWarn := checker.Check(TransceiverMetric{
		DeviceIP: "10.0.0.1",
		Port:     "GE0/0/1",
		RxPower:  -19.5,
		TxPower:  -3.0,
	})
	if resWarn.Level != LevelWarning || !resWarn.IsWeakOptical {
		t.Errorf("预期 warning 弱光，实际 level=%s, isWeak=%v", resWarn.Level, resWarn.IsWeakOptical)
	}

	// 2. -23.0 dBm: 严重弱光 critical
	resCrit := checker.Check(TransceiverMetric{
		DeviceIP: "10.0.0.1",
		Port:     "GE0/0/2",
		RxPower:  -23.0,
		TxPower:  -3.0,
	})
	if resCrit.Level != LevelCritical || !resCrit.IsWeakOptical {
		t.Errorf("预期 critical 弱光，实际 level=%s, isWeak=%v", resCrit.Level, resCrit.IsWeakOptical)
	}
}

func TestChecker_LOS(t *testing.T) {
	checker := NewChecker()

	// -40 dBm: 断光
	resLOS := checker.Check(TransceiverMetric{
		DeviceIP: "10.0.0.1",
		Port:     "GE0/0/3",
		RxPower:  -40.0,
		TxPower:  -3.0,
	})
	if resLOS.Level != LevelLOS || !resLOS.IsLOS {
		t.Errorf("预期 LOS 断光，实际 level=%s, isLOS=%v", resLOS.Level, resLOS.IsLOS)
	}
}

func TestChecker_PercentMode(t *testing.T) {
	customRule := models.OpticalCheckRule{
		RuleName:        "百分比偏离测试规则",
		Vendor:          "huawei",
		TransceiverType: "100GE-SR4",
		Mode:            models.OpticalModePercent,
		WarningPercent:  15.0,
		CritPercent:     30.0,
		Enabled:         true,
	}

	checker := NewChecker(customRule)

	// 标称 -10.0 dBm，实际 -14.0 dBm -> 偏离 ((-10 - (-14)) / 10) * 100 = 40% >= 30% -> Critical
	resCrit := checker.Check(TransceiverMetric{
		DeviceIP:        "10.10.10.1",
		Port:            "100GE1/0/1",
		Vendor:          "huawei",
		TransceiverType: "100GE-SR4",
		RxPower:         -14.0,
		NominalRxPower:  -10.0,
		TxPower:         -1.0,
	})

	if resCrit.Level != LevelCritical || !resCrit.IsWeakOptical {
		t.Errorf("预期 percent 模式 critical，实际 level=%s, isWeak=%v", resCrit.Level, resCrit.IsWeakOptical)
	}
}

func TestChecker_OverloadAndTemperature(t *testing.T) {
	checker := NewChecker()

	res := checker.Check(TransceiverMetric{
		DeviceIP:    "10.0.0.1",
		Port:        "GE0/0/5",
		RxPower:     1.5, // 超过 MaxRxPower (-1.0)
		Temperature: 82.0,
	})

	if res.Level != LevelWarning {
		t.Errorf("预期 warning，实际 %s", res.Level)
	}
	if len(res.Issues) < 2 {
		t.Errorf("预期至少2个问题(接收过载+高温)，实际 %d", len(res.Issues))
	}
}
