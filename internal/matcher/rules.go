package matcher

import "regexp"

type ErrorSeverity int

const (
	SeverityWarning  ErrorSeverity = iota // 仅告警，自动跳过并在日志内黄字提示
	SeverityCritical                      // 严重错误，触发 settings.error_mode 策略进行处理
)

// ErrorRule 匹配规则
type ErrorRule struct {
	Name     string         // 规则名称 (如 "命令不完整")
	Pattern  *regexp.Regexp // 正则表达式
	Severity ErrorSeverity  // 严重程度
	Vendor   string         // 适用厂商 (huawei, h3c, cisco, generic)
	Message  string         // 语义化说明
}

var DefaultRules = []ErrorRule{
	// --- 通用的警告提醒 ---
	{
		Name:     "配置未改变或已存在",
		Pattern:  regexp.MustCompile(`(?i)(not changed|already exists|no change)`),
		Severity: SeverityWarning,
		Vendor:   "generic",
		Message:  "配置与当前运行状态一致，无需下发",
	},
	{
		Name:     "Info告警",
		Pattern:  regexp.MustCompile(`(?i)(% ?info:|\binfo:|information:)`),
		Severity: SeverityWarning,
		Vendor:   "generic",
		Message:  "设备返回一般性通知",
	},

	// --- 华为/华三的警告提醒 ---
	{
		Name:     "华为告警提示",
		Pattern:  regexp.MustCompile(`(?i)(% ?warning:|warning:)`),
		Severity: SeverityWarning,
		Vendor:   "huawei_h3c",
		Message:  "设备返回非致命的警告信息",
	},

	// --- 通用的严重错误 ---
	{
		Name:     "命令无法识别",
		Pattern:  regexp.MustCompile(`(?i)(unrecognized command|unknown command)`),
		Severity: SeverityCritical,
		Vendor:   "generic",
		Message:  "设备不认识该命令，可能拼写错误或不支持",
	},
	{
		Name:     "命令不完整",
		Pattern:  regexp.MustCompile(`(?i)(incomplete command|ambiguous command)`),
		Severity: SeverityCritical,
		Vendor:   "generic",
		Message:  "命令缺少参数或存在歧义",
	},
	{
		Name:     "输入错误",
		Pattern:  regexp.MustCompile(`(?i)(\bError:|% ?Error:|^\s*\^\s*$)`),
		Severity: SeverityCritical,
		Vendor:   "generic",
		Message:  "设备执行输入返回 Error",
	},

	// --- 思科特有的严重错误 ---
	{
		Name:     "Cisco无效输入",
		Pattern:  regexp.MustCompile(`(?i)(% Invalid input detected)`),
		Severity: SeverityCritical,
		Vendor:   "cisco",
		Message:  "Cisco 终端报告语法错误",
	},
}
