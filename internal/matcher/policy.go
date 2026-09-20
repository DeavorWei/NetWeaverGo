package matcher

import (
	"embed"
	"encoding/json"
	"regexp"
	"strings"
	"sync"
)

//go:embed policies/*.json
var policiesFS embed.FS

// ErrorRuleDef 策略中的错误规则定义
type ErrorRuleDef struct {
	Name     string `json:"name"`
	Pattern  string `json:"pattern"`
	Severity string `json:"severity"` // warning | critical
	Vendor   string `json:"vendor"`
	Message  string `json:"message"`
}

// MatchPolicy 匹配策略模型
type MatchPolicy struct {
	Scene                string         `json:"scene"`      // collect | inspect | diagnose | *
	Vendor               string         `json:"vendor"`     // huawei | h3c | cisco | *
	DeviceType           string         `json:"deviceType"` // router | switch | firewall | *
	PromptPatterns       []string       `json:"promptPatterns"`
	PagerPatterns        []string       `json:"pagerPatterns"`
	ErrorRules           []ErrorRuleDef `json:"errorRules"`
	ConfirmPatterns      []string       `json:"confirmPatterns"`
	ValidateModel        string         `json:"validateModel"`        // echo_align | exact | any
	NeedPrompt           bool           `json:"needPrompt"`           // 是否需要提示符匹配
	ReceiveFilterKeys    []string       `json:"receiveFilterKeys"`    // ANSI 残留过滤
	ConnectCommandPolicy int            `json:"connectCommandPolicy"` // 回显拼接策略
}

// PolicyMatcher 策略匹配解析器
type PolicyMatcher struct {
	mu       sync.RWMutex
	policies []*MatchPolicy
	fallback *MatchPolicy
}

var (
	defaultPolicyMatcher     *PolicyMatcher
	defaultPolicyMatcherOnce sync.Once
)

// GetDefaultPolicyMatcher 获取策略匹配器单例
func GetDefaultPolicyMatcher() *PolicyMatcher {
	defaultPolicyMatcherOnce.Do(func() {
		pm := NewPolicyMatcher()
		_ = pm.LoadEmbedded()
		defaultPolicyMatcher = pm
	})
	return defaultPolicyMatcher
}

// NewPolicyMatcher 创建策略匹配器
func NewPolicyMatcher() *PolicyMatcher {
	return &PolicyMatcher{}
}

// LoadEmbedded 从嵌入文件系统加载策略
func (pm *PolicyMatcher) LoadEmbedded() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	entries, err := policiesFS.ReadDir("policies")
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".json") {
			data, err := policiesFS.ReadFile("policies/" + entry.Name())
			if err != nil {
				continue
			}
			p := &MatchPolicy{}
			if err := json.Unmarshal(data, p); err == nil {
				if entry.Name() == "default.json" {
					pm.fallback = p
				} else {
					pm.policies = append(pm.policies, p)
				}
			}
		}
	}
	return nil
}

// Resolve 根据场景、厂商、设备类型解析最优策略
func (pm *PolicyMatcher) Resolve(scene, vendor, deviceType string) *MatchPolicy {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	s := strings.ToLower(strings.TrimSpace(scene))
	v := strings.ToLower(strings.TrimSpace(vendor))
	d := strings.ToLower(strings.TrimSpace(deviceType))

	if s == "" {
		s = "*"
	}
	if v == "" {
		v = "*"
	}
	if d == "" {
		d = "*"
	}

	// 查找匹配：优先级 1. 厂商 + 场景 + 设备类型完全匹配
	for _, p := range pm.policies {
		if strings.EqualFold(p.Vendor, v) && strings.EqualFold(p.Scene, s) && strings.EqualFold(p.DeviceType, d) {
			return p
		}
	}

	// 优先级 2. 厂商匹配 + 场景或设备类型通配
	for _, p := range pm.policies {
		if strings.EqualFold(p.Vendor, v) &&
			(p.Scene == s || p.Scene == "*") &&
			(p.DeviceType == d || p.DeviceType == "*") {
			return p
		}
	}

	// 优先级 3. 全局通配策略
	for _, p := range pm.policies {
		if (p.Vendor == v || p.Vendor == "*") &&
			(p.Scene == s || p.Scene == "*") &&
			(p.DeviceType == d || p.DeviceType == "*") {
			return p
		}
	}

	if pm.fallback != nil {
		return pm.fallback
	}

	// 最终保底默认对象
	return &MatchPolicy{
		Scene:          "*",
		Vendor:         "*",
		DeviceType:     "*",
		PromptPatterns: []string{">(?:\\s*)$", "#(?:\\s*)$", "\\](?:\\s*)$"},
		PagerPatterns:  DefaultPaginationPrompts,
		NeedPrompt:     true,
	}
}

// ToCompiledErrorRules 将策略错误规则转为已编译 ErrorRule
func (p *MatchPolicy) ToCompiledErrorRules() []ErrorRule {
	if len(p.ErrorRules) == 0 {
		return DefaultRules
	}

	var rules []ErrorRule
	for _, r := range p.ErrorRules {
		sev := SeverityWarning
		if strings.EqualFold(r.Severity, "critical") {
			sev = SeverityCritical
		}
		re, err := regexp.Compile(r.Pattern)
		if err != nil {
			continue
		}
		rules = append(rules, ErrorRule{
			Name:     r.Name,
			Pattern:  re,
			Severity: sev,
			Vendor:   r.Vendor,
			Message:  r.Message,
		})
	}

	return rules
}

// CompilePromptRegexes 编译提示符正则表达式
func (p *MatchPolicy) CompilePromptRegexes() []*regexp.Regexp {
	var regexes []*regexp.Regexp
	for _, pat := range p.PromptPatterns {
		re, err := regexp.Compile(pat)
		if err == nil {
			regexes = append(regexes, re)
		}
	}
	return regexes
}
