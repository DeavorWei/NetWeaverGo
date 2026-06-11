package taskexec

import (
	"fmt"
	"strings"

	"github.com/robfig/cron/v3"

	"github.com/NetWeaverGo/core/internal/models"
)

// ScheduleValidator 调度校验器
type ScheduleValidator struct{}

// NewScheduleValidator 创建调度校验器
func NewScheduleValidator() *ScheduleValidator {
	return &ScheduleValidator{}
}

// ValidateScheduleConfig 校验调度配置的合法性
func (v *ScheduleValidator) ValidateScheduleConfig(group *models.TaskGroup) error {
	if group == nil {
		return fmt.Errorf("任务组不能为空")
	}

	if !group.ScheduleEnabled {
		// 未启用调度，不需要校验
		return nil
	}

	switch group.ScheduleType {
	case "cron":
		return v.validateCronSchedule(group)
	case "once":
		return v.validateOnceSchedule(group)
	case "":
		return fmt.Errorf("调度类型不能为空")
	default:
		return fmt.Errorf("不支持的调度类型: %s", group.ScheduleType)
	}
}

// validateCronSchedule 校验 Cron 调度配置
func (v *ScheduleValidator) validateCronSchedule(group *models.TaskGroup) error {
	expr := strings.TrimSpace(group.CronExpression)
	if expr == "" {
		return fmt.Errorf("Cron 表达式不能为空")
	}

	// 使用 cron.Parser 校验表达式合法性
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	_, err := parser.Parse(expr)
	if err != nil {
		return fmt.Errorf("Cron 表达式格式错误: %w", err)
	}

	return nil
}

// validateOnceSchedule 校验一次性调度配置
func (v *ScheduleValidator) validateOnceSchedule(group *models.TaskGroup) error {
	if group.OnceScheduledAt == nil {
		return fmt.Errorf("一次性调度必须指定计划执行时间")
	}

	// 允许过去的时间（用户可能在配置后延迟确认），
	// 但调度器注册时会跳过已过期的一次性调度
	return nil
}

// DescribeCronExpression 将 Cron 表达式转换为人类可读的中文描述
// 供前端展示使用
// 注意：此方法为简化实现，仅覆盖常见模式。对于复杂表达式，前端可使用更完善的 JS 库（如 cronstrue）做展示。
func (v *ScheduleValidator) DescribeCronExpression(expr string) string {
	// 此方法返回简单的中文描述，前端也可使用更复杂的描述库
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return "未配置"
	}

	parts := strings.Fields(expr)
	if len(parts) != 5 {
		return expr
	}

	// 常见模式的快速匹配
	minute, hour, dom, month, dow := parts[0], parts[1], parts[2], parts[3], parts[4]

	// 每天固定时间
	if dom == "*" && month == "*" && dow == "*" && !strings.Contains(minute, "/") {
		return fmt.Sprintf("每天 %s:%s", padZero(hour), padZero(minute))
	}

	// 每小时
	if hour == "*" && dom == "*" && month == "*" && dow == "*" && minute == "0" {
		return "每小时整点"
	}

	// 每N分钟
	if strings.HasPrefix(minute, "*/") && hour == "*" && dom == "*" && month == "*" {
		desc := fmt.Sprintf("每 %s 分钟", minute[2:])
		if dow != "*" {
			desc += fmt.Sprintf("（星期%s）", dow)
		}
		return desc
	}

	// 每周一到五
	if dow == "1-5" && dom == "*" && month == "*" {
		return fmt.Sprintf("工作日 %s:%s", padZero(hour), padZero(minute))
	}

	// 默认返回原始表达式
	return fmt.Sprintf("Cron: %s", expr)
}

func padZero(s string) string {
	if len(s) == 1 {
		return "0" + s
	}
	return s
}

