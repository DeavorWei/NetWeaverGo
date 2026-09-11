package executor

import (
	"regexp"
	"strings"
	"sync"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
)

// compiledRiskRule 预编译的风险规则
type compiledRiskRule struct {
	Rule   models.RiskCommand
	Regexp *regexp.Regexp
}

// RiskValidator 风险命令校验器
type RiskValidator struct {
	mu    sync.RWMutex
	rules []compiledRiskRule
}

var (
	globalRiskValidator *RiskValidator
	onceRiskValidator   sync.Once
)

// NewRiskValidatorFromRules 基于规则列表创建校验器
func NewRiskValidatorFromRules(rawRules []models.RiskCommand) *RiskValidator {
	v := &RiskValidator{
		rules: make([]compiledRiskRule, 0, len(rawRules)),
	}
	for _, r := range rawRules {
		if !r.Enabled {
			continue
		}
		re, err := regexp.Compile(r.Pattern)
		if err != nil {
			logger.Warn("RiskValidator", "-", "跳过无效风险规则正则: pattern=%s, err=%v", r.Pattern, err)
			continue
		}
		v.rules = append(v.rules, compiledRiskRule{
			Rule:   r,
			Regexp: re,
		})
	}
	return v
}

// GetGlobalRiskValidator 获取全局风险命令校验器
func GetGlobalRiskValidator() *RiskValidator {
	onceRiskValidator.Do(func() {
		globalRiskValidator = initGlobalRiskValidator()
	})
	return globalRiskValidator
}

func initGlobalRiskValidator() *RiskValidator {
	db := config.GetDB()
	var rules []models.RiskCommand

	if db != nil {
		if err := db.Where("enabled = ?", true).Find(&rules).Error; err != nil {
			logger.Warn("RiskValidator", "-", "从数据库读取风险规则失败，回退至内置种子: %v", err)
			rules = models.DefaultRiskCommandSeeds()
		}
	}

	if len(rules) == 0 {
		rules = models.DefaultRiskCommandSeeds()
	}

	return NewRiskValidatorFromRules(rules)
}

// ReloadRules 重新加载规则（支持动态修改后即时生效）
func (v *RiskValidator) ReloadRules(rawRules []models.RiskCommand) {
	if v == nil {
		return
	}

	compiled := make([]compiledRiskRule, 0, len(rawRules))
	for _, r := range rawRules {
		if !r.Enabled {
			continue
		}
		re, err := regexp.Compile(r.Pattern)
		if err != nil {
			logger.Warn("RiskValidator", "-", "跳过无效风险规则正则: pattern=%s, err=%v", r.Pattern, err)
			continue
		}
		compiled = append(compiled, compiledRiskRule{
			Rule:   r,
			Regexp: re,
		})
	}

	v.mu.Lock()
	defer v.mu.Unlock()
	v.rules = compiled
	logger.Info("RiskValidator", "-", "成功重载风险命令规则，共 %d 条生效", len(compiled))
}

// Validate 校验命令是否命中风险规则
// 匹配优先级：
// 1. 动作严重性：block > confirm > warn
// 2. 厂商专一性：特定 vendor > 通用 *
func (v *RiskValidator) Validate(cmd string, vendor string) (*models.RiskCommand, models.RiskCommandAction) {
	if v == nil || strings.TrimSpace(cmd) == "" {
		return nil, ""
	}

	v.mu.RLock()
	defer v.mu.RUnlock()

	targetVendor := strings.ToLower(strings.TrimSpace(vendor))
	cleanCmd := strings.TrimSpace(cmd)

	var (
		matchedBlock   *models.RiskCommand
		matchedConfirm *models.RiskCommand
		matchedWarn    *models.RiskCommand
	)

	for _, cr := range v.rules {
		r := cr.Rule
		ruleVendor := strings.ToLower(strings.TrimSpace(r.Vendor))

		// 厂商匹配判定：相同或为全局通配符
		if ruleVendor != "*" && ruleVendor != targetVendor {
			continue
		}

		if cr.Regexp.MatchString(cleanCmd) {
			ruleCopy := r
			switch r.Action {
			case models.RiskActionBlock:
				// 若已有匹配，优先特定厂商而非通配符
				if matchedBlock == nil || (ruleVendor == targetVendor && matchedBlock.Vendor == "*") {
					matchedBlock = &ruleCopy
				}
			case models.RiskActionConfirm:
				if matchedConfirm == nil || (ruleVendor == targetVendor && matchedConfirm.Vendor == "*") {
					matchedConfirm = &ruleCopy
				}
			case models.RiskActionWarn:
				if matchedWarn == nil || (ruleVendor == targetVendor && matchedWarn.Vendor == "*") {
					matchedWarn = &ruleCopy
				}
			}
		}
	}

	// 动作严重度优先返回
	if matchedBlock != nil {
		return matchedBlock, models.RiskActionBlock
	}
	if matchedConfirm != nil {
		return matchedConfirm, models.RiskActionConfirm
	}
	if matchedWarn != nil {
		return matchedWarn, models.RiskActionWarn
	}

	return nil, ""
}
