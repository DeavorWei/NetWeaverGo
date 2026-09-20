package executor

import (
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
)

// compiledRiskRule 预编译的风险规则
type compiledRiskRule struct {
	Rule   models.RiskCommand
	Regexp *regexp.Regexp
}

type compiledTrustRule struct {
	Entry  models.RiskTrustEntry
	Regexp *regexp.Regexp
}

// TemporaryBypass 动态临时放行条目
type TemporaryBypass struct {
	RunID     string    `json:"runId"`
	DeviceIP  string    `json:"deviceIp"`
	Command   string    `json:"command"`
	Operator  string    `json:"operator"`
	Reason    string    `json:"reason"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// RiskValidator 风险命令校验器
type RiskValidator struct {
	mu         sync.RWMutex
	rules      []compiledRiskRule
	trustRules []compiledTrustRule
	bypasses   []TemporaryBypass
}

var (
	globalRiskValidator *RiskValidator
	onceRiskValidator   sync.Once
)

// NewRiskValidatorFromRules 基于规则列表创建校验器
func NewRiskValidatorFromRules(rawRules []models.RiskCommand) *RiskValidator {
	v := &RiskValidator{
		rules:      make([]compiledRiskRule, 0, len(rawRules)),
		trustRules: make([]compiledTrustRule, 0),
		bypasses:   make([]TemporaryBypass, 0),
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
	var trustEntries []models.RiskTrustEntry

	if db != nil {
		if err := db.Where("enabled = ?", true).Find(&rules).Error; err != nil {
			logger.Warn("RiskValidator", "-", "从数据库读取风险规则失败，回退至内置种子: %v", err)
			rules = models.DefaultRiskCommandSeeds()
		}
		_ = db.Find(&trustEntries).Error
	}

	if len(rules) == 0 {
		rules = models.DefaultRiskCommandSeeds()
	}

	v := NewRiskValidatorFromRules(rules)
	v.ReloadTrustEntries(trustEntries)
	return v
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

// ReloadTrustEntries 重载信任清单
func (v *RiskValidator) ReloadTrustEntries(entries []models.RiskTrustEntry) {
	if v == nil {
		return
	}
	compiled := make([]compiledTrustRule, 0, len(entries))
	for _, e := range entries {
		if e.IsExpired() {
			continue
		}
		re, err := regexp.Compile(e.Pattern)
		if err != nil {
			logger.Warn("RiskValidator", "-", "跳过无效信任清单正则: pattern=%s, err=%v", e.Pattern, err)
			continue
		}
		compiled = append(compiled, compiledTrustRule{
			Entry:  e,
			Regexp: re,
		})
	}

	v.mu.Lock()
	defer v.mu.Unlock()
	v.trustRules = compiled
	logger.Info("RiskValidator", "-", "成功重载信任清单，共 %d 条生效", len(compiled))
}

// CheckTrust 检查命令是否命中有效信任清单
func (v *RiskValidator) CheckTrust(cmd string) (bool, *models.RiskTrustEntry) {
	if v == nil || strings.TrimSpace(cmd) == "" {
		return false, nil
	}
	v.mu.RLock()
	defer v.mu.RUnlock()

	cleanCmd := strings.TrimSpace(cmd)
	for _, tr := range v.trustRules {
		if tr.Entry.IsExpired() {
			continue
		}
		if tr.Regexp.MatchString(cleanCmd) {
			entryCopy := tr.Entry
			return true, &entryCopy
		}
	}
	return false, nil
}

// AddTemporaryBypass 注入单次/短期临时放行凭据
func (v *RiskValidator) AddTemporaryBypass(b TemporaryBypass) {
	if v == nil {
		return
	}
	v.mu.Lock()
	defer v.mu.Unlock()

	now := time.Now()
	active := make([]TemporaryBypass, 0, len(v.bypasses)+1)
	for _, item := range v.bypasses {
		if now.Before(item.ExpiresAt) {
			active = append(active, item)
		}
	}
	if b.ExpiresAt.IsZero() {
		b.ExpiresAt = now.Add(10 * time.Minute)
	}
	active = append(active, b)
	v.bypasses = active
	logger.Info("RiskValidator", b.DeviceIP, "已注册临时放行凭证: runID=%s cmd=%q 有效期至=%s", b.RunID, b.Command, b.ExpiresAt.Format(time.RFC3339))
}

// CheckBypass 检查是否有针对当前执行的有效临时放行凭据（命中则放行并消费）
func (v *RiskValidator) CheckBypass(runID, deviceIP, cmd string) (bool, *TemporaryBypass) {
	if v == nil || strings.TrimSpace(cmd) == "" {
		return false, nil
	}
	v.mu.Lock()
	defer v.mu.Unlock()

	now := time.Now()
	cleanCmd := strings.TrimSpace(cmd)
	for i, b := range v.bypasses {
		if now.After(b.ExpiresAt) {
			continue
		}
		if (b.RunID == "" || b.RunID == runID) &&
			(b.DeviceIP == "" || b.DeviceIP == deviceIP) &&
			(b.Command == cleanCmd || strings.EqualFold(b.Command, cleanCmd)) {
			matched := b
			// 单次放行生效后移除，保障逃生不产生持久漏洞
			v.bypasses = append(v.bypasses[:i], v.bypasses[i+1:]...)
			return true, &matched
		}
	}
	return false, nil
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
