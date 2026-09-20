package optical

import (
	"fmt"
	"math"
	"strings"
	"sync"

	"github.com/NetWeaverGo/core/internal/models"
)

// Level 告警等级
type Level string

const (
	LevelNormal   Level = "normal"
	LevelWarning  Level = "warning"
	LevelCritical Level = "critical"
	LevelLOS      Level = "los" // Loss of Signal 断光
)

// TransceiverMetric 采集到的单端口光模块指标
type TransceiverMetric struct {
	DeviceIP        string  `json:"deviceIp"`
	Port            string  `json:"port"`
	Vendor          string  `json:"vendor"`
	TransceiverType string  `json:"transceiverType"` // 如 10GE-LR, 100GE-SR4, SFP-GE-SX
	TxPower         float64 `json:"txPower"`         // 发射光功率 (dBm)
	RxPower         float64 `json:"rxPower"`         // 接收光功率 (dBm)
	Temperature     float64 `json:"temperature"`     // 工作温度 (°C)
	Voltage         float64 `json:"voltage"`         // 工作电压 (V)
	BiasCurrent     float64 `json:"biasCurrent"`     // 偏置电流 (mA)
	NominalTxPower  float64 `json:"nominalTxPower"`  // 标称发光基准 (dBm, 供百分比模式)
	NominalRxPower  float64 `json:"nominalRxPower"`  // 标称收光基准 (dBm, 供百分比模式)
}

// CheckResult 光模块健康检测结果
type CheckResult struct {
	DeviceIP        string   `json:"deviceIp"`
	Port            string   `json:"port"`
	Vendor          string   `json:"vendor"`
	TransceiverType string   `json:"transceiverType"`
	TxPower         float64  `json:"txPower"`
	RxPower         float64  `json:"rxPower"`
	Level           Level    `json:"level"`
	IsWeakOptical   bool     `json:"isWeakOptical"`
	IsLOS           bool     `json:"isLos"`
	Issues          []string `json:"issues"`
	Advice          string   `json:"advice"`
}

// Checker 光模块弱光与异常检测器
type Checker struct {
	mu          sync.RWMutex
	rules       []models.OpticalCheckRule
	defaultRule models.OpticalCheckRule
}

// NewChecker 创建检测器实例
func NewChecker(rules ...models.OpticalCheckRule) *Checker {
	c := &Checker{
		rules:       rules,
		defaultRule: models.DefaultOpticalRule(),
	}
	return c
}

// SetRules 更新规则集
func (c *Checker) SetRules(rules []models.OpticalCheckRule) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.rules = rules
}

// matchRule 查找匹配的规则
func (c *Checker) matchRule(vendor, transceiverType string) models.OpticalCheckRule {
	c.mu.RLock()
	defer c.mu.RUnlock()

	v := strings.ToLower(strings.TrimSpace(vendor))
	t := strings.ToLower(strings.TrimSpace(transceiverType))

	// 1. 精确匹配 (vendor + type)
	for _, r := range c.rules {
		if !r.Enabled {
			continue
		}
		if strings.EqualFold(r.Vendor, v) && strings.EqualFold(r.TransceiverType, t) {
			return r
		}
	}

	// 2. 匹配 vendor + 通配 type
	for _, r := range c.rules {
		if !r.Enabled {
			continue
		}
		if strings.EqualFold(r.Vendor, v) && (r.TransceiverType == "*" || r.TransceiverType == "") {
			return r
		}
	}

	// 3. 通配 vendor + 精确 type
	for _, r := range c.rules {
		if !r.Enabled {
			continue
		}
		if (r.Vendor == "*" || r.Vendor == "") && strings.EqualFold(r.TransceiverType, t) {
			return r
		}
	}

	return c.defaultRule
}

// Check 评估单个端口光模块指标
func (c *Checker) Check(metric TransceiverMetric) CheckResult {
	rule := c.matchRule(metric.Vendor, metric.TransceiverType)

	res := CheckResult{
		DeviceIP:        metric.DeviceIP,
		Port:            metric.Port,
		Vendor:          metric.Vendor,
		TransceiverType: metric.TransceiverType,
		TxPower:         metric.TxPower,
		RxPower:         metric.RxPower,
		Level:           LevelNormal,
		IsWeakOptical:   false,
		IsLOS:           false,
		Issues:          make([]string, 0),
		Advice:          "",
	}

	// 1. 绝对断光判定 (LOS: Loss of Signal)
	if metric.RxPower <= -35.0 || metric.RxPower <= (rule.MinRxPower-15.0) {
		res.Level = LevelLOS
		res.IsLOS = true
		res.IsWeakOptical = true
		res.Issues = append(res.Issues, fmt.Sprintf("光信号中断 (LOS): 接收光功率 %.2f dBm 极低或链路断开", metric.RxPower))
		res.Advice = "请检查对端发光、本端光模块是否插紧以及光纤跳线是否折断"
		return res
	}

	// 2. 百分比模式 (Percent Mode)
	if rule.Mode == models.OpticalModePercent && metric.NominalRxPower != 0 {
		dev := (metric.NominalRxPower - metric.RxPower) / math.Abs(metric.NominalRxPower) * 100.0
		if dev >= rule.CritPercent {
			res.Level = LevelCritical
			res.IsWeakOptical = true
			res.Issues = append(res.Issues, fmt.Sprintf("光衰严重偏离标称值: 实际 %.2f dBm, 标称 %.2f dBm, 偏离度 %.1f%% >= %.1f%%",
				metric.RxPower, metric.NominalRxPower, dev, rule.CritPercent))
			res.Advice = "光纤链路损耗过大，建议清洗光纤接头或使用 OTDR 排查中继熔接点"
		} else if dev >= rule.WarningPercent {
			if res.Level == LevelNormal {
				res.Level = LevelWarning
			}
			res.IsWeakOptical = true
			res.Issues = append(res.Issues, fmt.Sprintf("光衰轻微偏离标称值: 实际 %.2f dBm, 标称 %.2f dBm, 偏离度 %.1f%% >= %.1f%%",
				metric.RxPower, metric.NominalRxPower, dev, rule.WarningPercent))
			res.Advice = "光衰接近临界值，建议列入重点观察清单并在割接窗口清洗法兰盘"
		}
		return res
	}

	// 3. 绝对门限模式 (Absolute Mode)
	// 3.1 接收光功率 (RxPower)
	if metric.RxPower < rule.MinRxPower {
		res.IsWeakOptical = true
		if metric.RxPower < rule.MinRxPower-3.0 {
			res.Level = LevelCritical
			res.Issues = append(res.Issues, fmt.Sprintf("严重弱光告警: 接收光功率 %.2f dBm 低于绝对安全下限 %.2f dBm (裕度已穿透)",
				metric.RxPower, rule.MinRxPower))
			res.Advice = "光功率触底严重，极易发生 CRC 错包与链路震荡，请立即检查光纤衰耗"
		} else {
			if res.Level == LevelNormal {
				res.Level = LevelWarning
			}
			res.Issues = append(res.Issues, fmt.Sprintf("轻度弱光预警: 接收光功率 %.2f dBm 低于设计下限 %.2f dBm",
				metric.RxPower, rule.MinRxPower))
			res.Advice = "建议检查光纤法兰盘并用光纤端面显微镜检测端面清洁度"
		}
	} else if metric.RxPower > rule.MaxRxPower {
		if res.Level == LevelNormal {
			res.Level = LevelWarning
		}
		res.Issues = append(res.Issues, fmt.Sprintf("接收过载风险: 接收光功率 %.2f dBm 高于安全上限 %.2f dBm",
			metric.RxPower, rule.MaxRxPower))
		res.Advice = "接收光过强可能导致探测器光电饱和烧毁，短距链路建议加装光衰减器 (Attenuator)"
	}

	// 3.2 发射光功率 (TxPower)
	if metric.TxPower != 0 {
		if metric.TxPower < rule.MinTxPower {
			if res.Level == LevelNormal {
				res.Level = LevelWarning
			}
			res.Issues = append(res.Issues, fmt.Sprintf("发光功率偏低: 发射光功率 %.2f dBm 低于下限 %.2f dBm",
				metric.TxPower, rule.MinTxPower))
			if res.Advice == "" {
				res.Advice = "本端光模块发射激光器可能发生老化，建议观察并适时更换模块"
			}
		} else if metric.TxPower > rule.MaxTxPower {
			if res.Level == LevelNormal {
				res.Level = LevelWarning
			}
			res.Issues = append(res.Issues, fmt.Sprintf("发光功率异常过高: 发射光功率 %.2f dBm 高于上限 %.2f dBm",
				metric.TxPower, rule.MaxTxPower))
		}
	}

	// 3.3 工作温度判定
	if metric.Temperature > 75.0 {
		if res.Level == LevelNormal {
			res.Level = LevelWarning
		}
		res.Issues = append(res.Issues, fmt.Sprintf("光模块温度过高: 当前 %.1f°C 超过安全阈值 75.0°C", metric.Temperature))
		if res.Advice == "" {
			res.Advice = "请检查机箱整体散热与风道"
		}
	}

	return res
}

// CheckBatch 批量评估光模块指标
func (c *Checker) CheckBatch(metrics []TransceiverMetric) []CheckResult {
	results := make([]CheckResult, len(metrics))
	for i, m := range metrics {
		results[i] = c.Check(m)
	}
	return results
}

// BatchCheck 别名兼容
func (c *Checker) BatchCheck(metrics []TransceiverMetric) []CheckResult {
	return c.CheckBatch(metrics)
}
