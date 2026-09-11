package inspection

import (
	"testing"

	"github.com/NetWeaverGo/core/internal/models"
	"github.com/stretchr/testify/assert"
)

func TestEvaluateItem_NumericThreshold(t *testing.T) {
	item := &models.InspectionItem{
		Code:      "CHECK_CPU",
		Name:      "CPU 使用率",
		Category:  "system",
		CheckType: "threshold",
		Field:     "cpu_usage",
		Severity:  "major",
		Enabled:   true,
		ThresholdsJSON: mustJSON([]models.Threshold{
			{Name: "cpu_usage", DataType: "float", MaxValue: "80.0", RangeType: "lte", Unit: "%"},
		}),
	}

	// 1. 正常值 45% -> 通过
	inputPass := &EvaluateInput{
		RunID:    "run-1",
		DeviceIP: "10.0.0.1",
		Item:     item,
		ParsedRows: []map[string]interface{}{
			{"cpu_usage": "45.0%"},
		},
		RawEcho: "CPU utilization: 45.0%\nTask 1: 5%\nTask 2: 2%",
	}
	resPass := EvaluateItem(inputPass)
	assert.Equal(t, string(ResultPass), resPass.Status)
	assert.Equal(t, string(SeverityMajor), resPass.Severity)
	assert.Equal(t, "45.0%", resPass.ActualValue)
	assert.Empty(t, resPass.Problem)

	// 2. 超标值 92.5% -> 不通过
	inputFail := &EvaluateInput{
		RunID:    "run-1",
		DeviceIP: "10.0.0.1",
		Item:     item,
		ParsedRows: []map[string]interface{}{
			{"cpu_usage": "92.5%"},
		},
		RawEcho: "CPU utilization: 92.5%\nTask HIGH: 80%",
	}
	resFail := EvaluateItem(inputFail)
	assert.Equal(t, string(ResultFail), resFail.Status)
	assert.Equal(t, string(SeverityMajor), resFail.Severity)
	assert.Contains(t, resFail.Problem, "第 1 行数值 (92.5%)")
	assert.NotEmpty(t, resFail.EvidenceJSON)
}

func TestEvaluateItem_MultiRow_TableDetection(t *testing.T) {
	// 场景 1: 多风扇状态，第 1 行正常，第 2 行异常 -> 必须判定为 FAIL
	fanItem := &models.InspectionItem{
		Code:      "GEN_FAN_STATUS",
		Name:      "风扇状态检查",
		Category:  "environment",
		CheckType: "must_not_contain",
		Field:     "status",
		Severity:  "blocker",
		Enabled:   true,
		ThresholdsJSON: mustJSON([]models.Threshold{
			{Name: "fan_abnormal", DataType: "string", DefaultValue: "Abnormal"},
		}),
	}

	inputMultiFan := &EvaluateInput{
		RunID:    "run-2",
		DeviceIP: "10.0.0.2",
		Item:     fanItem,
		ParsedRows: []map[string]interface{}{
			{"fan_id": "Fan 1", "status": "Normal"},
			{"fan_id": "Fan 2", "status": "Abnormal"},
			{"fan_id": "Fan 3", "status": "Normal"},
		},
		RawEcho: "Fan 1: Normal\nFan 2: Abnormal\nFan 3: Normal",
	}

	resFan := EvaluateItem(inputMultiFan)
	assert.Equal(t, string(ResultFail), resFan.Status, "多行回显第 2 行 Abnormal 必须判定为失败")
	assert.Equal(t, string(SeverityBlocker), resFan.Severity)
	assert.Contains(t, resFan.Problem, "第 2 行包含禁止关键字 'Abnormal'")

	// 场景 2: 多单板温度检测，第 1、2 块正常，第 3 块 72°C 超标 (阈值 lte 65) -> 必须判定为 FAIL
	tempItem := &models.InspectionItem{
		Code:      "GEN_TEMPERATURE",
		Name:      "单板环境温度检查",
		Category:  "environment",
		CheckType: "threshold",
		Field:     "temperature",
		Severity:  "major",
		Enabled:   true,
		ThresholdsJSON: mustJSON([]models.Threshold{
			{Name: "temp", DataType: "float", MaxValue: "65.0", RangeType: "lte", Unit: "°C"},
		}),
	}

	inputMultiTemp := &EvaluateInput{
		RunID:    "run-2",
		DeviceIP: "10.0.0.2",
		Item:     tempItem,
		ParsedRows: []map[string]interface{}{
			{"slot": "Slot 1", "temperature": "42°C"},
			{"slot": "Slot 2", "temperature": "50°C"},
			{"slot": "Slot 3", "temperature": "72°C"},
		},
		RawEcho: "Slot 1: 42°C\nSlot 2: 50°C\nSlot 3: 72°C",
	}

	resTemp := EvaluateItem(inputMultiTemp)
	assert.Equal(t, string(ResultFail), resTemp.Status, "第 3 块板卡 72°C 超标必须判定为失败")
	assert.Equal(t, string(SeverityMajor), resTemp.Severity)
	assert.Contains(t, resTemp.ActualValue, "第 3 行")
}

func TestEvaluateItem_MustNotContain(t *testing.T) {
	item := &models.InspectionItem{
		Code:      "CHECK_FAN",
		Name:      "风扇状态检查",
		Category:  "environment",
		CheckType: "must_not_contain",
		Field:     "status",
		Severity:  "blocker",
		Enabled:   true,
		ThresholdsJSON: mustJSON([]models.Threshold{
			{Name: "fan_abnormal", DataType: "string", DefaultValue: "Abnormal"},
		}),
	}

	// 1. 正常状态
	inputPass := &EvaluateInput{
		RunID:    "run-1",
		DeviceIP: "10.0.0.1",
		Item:     item,
		RawEcho:  "FanBox 1 State: Normal\nFanBox 2 State: Normal",
	}
	resPass := EvaluateItem(inputPass)
	assert.Equal(t, string(ResultPass), resPass.Status)

	// 2. 异常状态
	inputFail := &EvaluateInput{
		RunID:    "run-1",
		DeviceIP: "10.0.0.1",
		Item:     item,
		RawEcho:  "FanBox 1 State: Normal\nFanBox 2 State: Abnormal",
	}
	resFail := EvaluateItem(inputFail)
	assert.Equal(t, string(ResultFail), resFail.Status)
	assert.Equal(t, string(SeverityBlocker), resFail.Severity)
}

func TestEvaluateItem_EmptyThreshold_Defensive(t *testing.T) {
	// 当 checkType 为 threshold 时，若 ThresholdsJSON 为空，不应静默 PASS，应返回 EXCEPT
	item := &models.InspectionItem{
		Code:           "CHECK_BROKEN_RULE",
		Name:           "缺失阈值配置项",
		CheckType:      "threshold",
		Field:          "some_val",
		Severity:       "major",
		Enabled:        true,
		ThresholdsJSON: "", // 空配置
	}

	input := &EvaluateInput{
		RunID:    "run-1",
		DeviceIP: "10.0.0.1",
		Item:     item,
		ParsedRows: []map[string]interface{}{
			{"some_val": "100"},
		},
		RawEcho: "some_val: 100",
	}

	res := EvaluateItem(input)
	assert.Equal(t, string(ResultExcept), res.Status, "未配置阈值时不应静默通过，需返回异常状态")
	assert.Contains(t, res.Problem, "阈值规则未配置")

	// 测试阈值项存在但区间值全为空的场景
	itemEmptyRange := &models.InspectionItem{
		Code:           "CHECK_EMPTY_RANGE",
		Name:           "空区间检查项",
		CheckType:      "threshold",
		Field:          "some_val",
		Severity:       "major",
		ThresholdsJSON: `[{"name":"cpu","minValue":"","maxValue":"","defaultValue":""}]`,
	}
	inputEmptyRange := &EvaluateInput{
		RunID:      "run-1",
		DeviceIP:   "10.0.0.1",
		Item:       itemEmptyRange,
		ParsedRows: []map[string]interface{}{{"some_val": "50"}},
	}
	resEmptyRange := EvaluateItem(inputEmptyRange)
	assert.Equal(t, string(ResultExcept), resEmptyRange.Status)
	assert.Contains(t, resEmptyRange.Problem, "阈值规则未配置有效区间")

	// 测试 input == nil 与 input.Item == nil 防御
	assert.Equal(t, string(ResultExcept), EvaluateItem(nil).Status)
	assert.Equal(t, string(ResultExcept), EvaluateItem(&EvaluateInput{}).Status)
}

func TestEvaluateItem_Regex(t *testing.T) {
	item := &models.InspectionItem{
		Code:      "CHECK_VERSION",
		Name:      "软件版本正则检查",
		CheckType: "regex",
		Severity:  "minor",
		Enabled:   true,
		ThresholdsJSON: mustJSON([]models.Threshold{
			{Name: "version_re", DefaultValue: `VRP.*Version\s+\d+\.\d+`},
		}),
	}

	inputPass := &EvaluateInput{
		RunID:    "run-1",
		DeviceIP: "10.0.0.1",
		Item:     item,
		RawEcho:  "Huawei Versatile Routing Platform Software\nVRP (R) software, Version 8.180\nCopyright (C) 2012-2023 Huawei",
	}
	resPass := EvaluateItem(inputPass)
	assert.Equal(t, string(ResultPass), resPass.Status)

	inputFail := &EvaluateInput{
		RunID:    "run-1",
		DeviceIP: "10.0.0.1",
		Item:     item,
		RawEcho:  "Unknown OS Kernel 3.10",
	}
	resFail := EvaluateItem(inputFail)
	// minor 级别不合格映射为 TEST_WARNING
	assert.Equal(t, string(ResultWarning), resFail.Status)
	assert.Equal(t, string(SeverityMinor), resFail.Severity)
}

func TestMapSeverityToResult(t *testing.T) {
	// Blocker
	pCode, pSev := MapSeverityToResult("blocker", true)
	assert.Equal(t, ResultPass, pCode)
	assert.Equal(t, SeverityBlocker, pSev)

	fCode, fSev := MapSeverityToResult("blocker", false)
	assert.Equal(t, ResultFail, fCode)
	assert.Equal(t, SeverityBlocker, fSev)

	// critical 别名兼容 -> 映射为 Blocker
	cCode, cSev := MapSeverityToResult("critical", false)
	assert.Equal(t, ResultFail, cCode)
	assert.Equal(t, SeverityBlocker, cSev)

	// Minor -> warning when failed
	mCode, mSev := MapSeverityToResult("minor", false)
	assert.Equal(t, ResultWarning, mCode)
	assert.Equal(t, SeverityMinor, mSev)

	// Info -> ignore when failed
	iCode, iSev := MapSeverityToResult("info", false)
	assert.Equal(t, ResultIgnore, iCode)
	assert.Equal(t, SeverityInfo, iSev)
}
