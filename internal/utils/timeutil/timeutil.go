package timeutil

import (
	"fmt"
	"time"
)

// 常用时间格式
const (
	DateTimeFormat = "2006-01-02 15:04:05" // 日期时间格式
	DateFormat     = "2006-01-02"          // 日期格式
	TimeFormat     = "15:04:05"            // 时间格式
)

// 时间解析错误提示模板
const (
	ErrTimeFormatInvalid = "时间格式无效: %s, 期望格式: %s"
	ErrTimeRangeInvalid  = "时间范围无效: %s, 有效范围: %s ~ %s"
	ErrTimeValueEmpty    = "时间值不能为空"
)

// ParseDurationWithHint 解析时间间隔，提供友好的错误提示
// input: 输入的时间字符串
// fieldName: 字段名称，用于错误提示
// 返回解析后的时间间隔和可能的错误
func ParseDurationWithHint(input string, fieldName string) (time.Duration, error) {
	if input == "" {
		return 0, fmt.Errorf("%s 不能为空", fieldName)
	}

	d, err := time.ParseDuration(input)
	if err != nil {
		return 0, fmt.Errorf("%s 格式无效: %q (期望格式如 30s, 1m30s, 2h)", fieldName, input)
	}
	return d, nil
}

// ParseDurationWithDefault 解析时间间隔，解析失败时返回默认值并记录警告
// input: 输入的时间字符串
// defaultValue: 解析失败时的默认值
// fieldName: 字段名称，用于错误提示
// 返回解析后的时间间隔（或默认值）和是否解析成功
func ParseDurationWithDefault(input string, defaultValue time.Duration, fieldName string) (time.Duration, bool) {
	if input == "" {
		return defaultValue, false
	}

	d, err := time.ParseDuration(input)
	if err != nil {
		return defaultValue, false
	}
	return d, true
}

// ValidateDurationRange 验证时间间隔是否在有效范围内
// d: 要验证的时间间隔
// min: 最小值
// max: 最大值
// fieldName: 字段名称，用于错误提示
func ValidateDurationRange(d time.Duration, min, max time.Duration, fieldName string) error {
	if d < min || d > max {
		return fmt.Errorf(ErrTimeRangeInvalid, d, min, max)
	}
	return nil
}

// FormatDuration 格式化时间间隔为友好的字符串
// d: 时间间隔
func FormatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		mins := int(d.Minutes())
		secs := int(d.Seconds()) % 60
		if secs == 0 {
			return fmt.Sprintf("%dm", mins)
		}
		return fmt.Sprintf("%dm%ds", mins, secs)
	}
	hours := int(d.Hours())
	mins := int(d.Minutes()) % 60
	if mins == 0 {
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dh%dm", hours, mins)
}
