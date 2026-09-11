package executor

import (
	"testing"

	"github.com/NetWeaverGo/core/internal/models"
)

func TestRiskValidator_Matches(t *testing.T) {
	// 使用内置种子规则进行单元测试
	validator := NewRiskValidatorFromRules(models.DefaultRiskCommandSeeds())

	// 1. 测试通用高危阻断 (Block)
	{
		rule, action := validator.Validate("format flash:", "huawei")
		if action != models.RiskActionBlock {
			t.Fatalf("期望 format 命令被阻断，实际 action=%s", action)
		}
		if rule == nil || rule.Action != models.RiskActionBlock {
			t.Errorf("期望匹配到 block 规则")
		}
	}

	// 2. 测试华为特有高危阻断 (Block)
	{
		_, action := validator.Validate("reset saved-configuration", "huawei")
		if action != models.RiskActionBlock {
			t.Fatalf("期望 reset saved-configuration 被阻断，实际 action=%s", action)
		}
	}

	// 3. 测试思科特有高危阻断 (Block)
	{
		_, action := validator.Validate("write erase", "cisco")
		if action != models.RiskActionBlock {
			t.Fatalf("期望思科 write erase 被阻断，实际 action=%s", action)
		}
	}

	// 4. 测试通用审批确认 (Confirm)
	{
		_, action := validator.Validate("reboot", "huawei")
		if action != models.RiskActionConfirm {
			t.Fatalf("期望 reboot 触发 confirm 审批，实际 action=%s", action)
		}

		_, action2 := validator.Validate("shutdown", "cisco")
		if action2 != models.RiskActionConfirm {
			t.Fatalf("期望 shutdown 触发 confirm 审批，实际 action=%s", action2)
		}
	}

	// 5. 测试核心协议与接口删除触发审批确认 (Confirm)
	{
		_, action := validator.Validate("undo ospf 1", "huawei")
		if action != models.RiskActionConfirm {
			t.Fatalf("期望 undo ospf 1 触发 confirm 审批，实际 action=%s", action)
		}

		_, actionCisco := validator.Validate("no router ospf 100", "cisco")
		if actionCisco != models.RiskActionConfirm {
			t.Fatalf("期望 no router ospf 触发 confirm 审批，实际 action=%s", actionCisco)
		}
	}

	// 6. 测试调试打印告警 (Warn)
	{
		_, action := validator.Validate("debugging all", "h3c")
		if action != models.RiskActionWarn {
			t.Fatalf("期望 debugging all 触发 warn 告警，实际 action=%s", action)
		}
	}

	// 7. 测试常规查询命令正常放行
	{
		safeCommands := []string{
			"display version",
			"display ip interface brief",
			"show run",
			"show ip route",
			"display current-configuration",
		}
		for _, cmd := range safeCommands {
			_, action := validator.Validate(cmd, "huawei")
			if action != "" {
				t.Errorf("常规无害查询命令 %q 不应被拦截或告警，实际 action=%s", cmd, action)
			}
		}
	}

	// 8. 测试新增的 VPN 与破坏性命令规则
	{
		_, actionIke := validator.Validate("reset ike sa", "huawei")
		if actionIke != models.RiskActionConfirm {
			t.Errorf("期望 reset ike sa 触发 confirm，实际 action=%s", actionIke)
		}

		_, actionIpsec := validator.Validate("undo ipsec policy P1", "huawei")
		if actionIpsec != models.RiskActionConfirm {
			t.Errorf("期望 undo ipsec policy 触发 confirm，实际 action=%s", actionIpsec)
		}

		_, actionCrypto := validator.Validate("no crypto isakmp enable", "cisco")
		if actionCrypto != models.RiskActionConfirm {
			t.Errorf("期望 no crypto isakmp 触发 confirm，实际 action=%s", actionCrypto)
		}

		_, actionRm := validator.Validate("rm -rf /", "linux")
		if actionRm != models.RiskActionBlock {
			t.Errorf("期望 rm -rf / 触发 block，实际 action=%s", actionRm)
		}
	}

	// 9. 测试 ReloadRules 动态热重载
	{
		customRules := []models.RiskCommand{
			{
				Vendor:   "*",
				Pattern:  `(?i)^\s*custom-dangerous-cmd\b`,
				Action:   models.RiskActionBlock,
				Reason:   "自定义测试规则",
				Enabled:  true,
			},
		}
		validator.ReloadRules(customRules)

		_, actionCustom := validator.Validate("custom-dangerous-cmd 123", "generic")
		if actionCustom != models.RiskActionBlock {
			t.Errorf("期望自定义规则热重载后生效为 block，实际 action=%s", actionCustom)
		}
	}
}

