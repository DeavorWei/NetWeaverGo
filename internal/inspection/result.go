package inspection

import (
	"strings"

	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
)

// ResultCode 巡检测试结果码
type ResultCode string

const (
	ResultPass     ResultCode = "TEST_PASS"     // 测试通过
	ResultFail     ResultCode = "TEST_FAIL"     // 测试不通过 / 严重不符合
	ResultWarning  ResultCode = "TEST_WARNING"  // 警告但不阻断（合规"建议改进"级别）
	ResultUnaccord ResultCode = "TEST_UNACCORD" // 不符合
	ResultIgnore   ResultCode = "TEST_IGNORE"   // 忽略
	ResultExcept   ResultCode = "TEST_EXCEPT"   // 异常/无法判定
	ResultManual   ResultCode = "TEST_MANUAL"   // 需人工确认
	ResultUntest   ResultCode = "TEST_UNTEST"   // 未测试（默认）
)

// Severity 严重度（承载 A2合规 / A5验收 的分级语义）
type Severity string

const (
	SeverityBlocker Severity = "blocker" // 阻断级
	SeverityMajor   Severity = "major"   // 主要严重
	SeverityMinor   Severity = "minor"   // 次要/警告
	SeverityInfo    Severity = "info"    // 提示
)

// String 返回字符串
func (r ResultCode) String() string {
	return string(r)
}

// IsFail 判断是否为不合格终态
func (r ResultCode) IsFail() bool {
	return r == ResultFail || r == ResultUnaccord
}

// IsPass 判断是否为通过态
func (r ResultCode) IsPass() bool {
	return r == ResultPass
}

// MapSeverityToResult 根据来源严重度映射标准结果码与严重度
// 对接规划方案 §8.2 P4-2 级别映射协议：
// blocker -> TEST_FAIL, blocker
// major   -> TEST_FAIL, major
// minor   -> TEST_WARNING, minor
// info    -> TEST_PASS, info
func MapSeverityToResult(sev string, passed bool) (ResultCode, Severity) {
	s := strings.ToLower(strings.TrimSpace(sev))
	switch s {
	case "blocker", "critical": // "critical" 防御性兼容映射为 blocker
		if passed {
			return ResultPass, SeverityBlocker
		}
		return ResultFail, SeverityBlocker
	case "major":
		if passed {
			return ResultPass, SeverityMajor
		}
		return ResultFail, SeverityMajor
	case "minor":
		if passed {
			return ResultPass, SeverityMinor
		}
		return ResultWarning, SeverityMinor
	case "info":
		if passed {
			return ResultPass, SeverityInfo
		}
		return ResultIgnore, SeverityInfo
	default:
		logger.Warn("InspectionResult", "-", "未知的严重度级别: '%s'，已降级回退处理", sev)
		if passed {
			return ResultPass, SeverityInfo
		}
		return ResultFail, SeverityMajor
	}
}

// FromComplianceResult 转换合规检查结果至统一 InspectionResult 载体
func FromComplianceResult(runID, deviceIP, ruleCode, ruleName, severity, actual, standard string, passed bool, evidence []string) models.InspectionResult {
	code, sev := MapSeverityToResult(severity, passed)
	problem := ""
	advice := ""
	if !passed {
		problem = "合规基线检查不符合: 期望 " + standard + "，实际采集为 " + actual
		advice = "请按照合规基线标准修正设备配置"
	}
	return models.InspectionResult{
		RunID:        runID,
		DeviceIP:     deviceIP,
		ItemCode:     ruleCode,
		ItemName:     ruleName,
		Category:     "compliance",
		Status:       string(code),
		Severity:     string(sev),
		Problem:      problem,
		Advice:       advice,
		ActualValue:  actual,
		ThresholdHit: standard,
	}
}

// FromAcceptanceResult 转换验收清单结果至统一 InspectionResult 载体
func FromAcceptanceResult(runID, deviceIP, itemCode, itemName, severity, actual, expected string, passed bool, problem, advice string) models.InspectionResult {
	code, sev := MapSeverityToResult(severity, passed)
	if !passed && problem == "" {
		problem = "工程验收项未达标: 期望 " + expected + "，实际 " + actual
	}
	return models.InspectionResult{
		RunID:        runID,
		DeviceIP:     deviceIP,
		ItemCode:     itemCode,
		ItemName:     itemName,
		Category:     "acceptance",
		Status:       string(code),
		Severity:     string(sev),
		Problem:      problem,
		Advice:       advice,
		ActualValue:  actual,
		ThresholdHit: expected,
	}
}
