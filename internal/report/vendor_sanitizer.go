package report

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
)

//go:embed rules/sensitive_cmd.json
var embeddedSensitiveCmdJSON []byte

// VendorSanitizeRule 单条厂商脱敏规则
type VendorSanitizeRule struct {
	Category    string         `json:"category"`
	Commands    []string       `json:"commands"`
	RawPattern  string         `json:"pattern"`
	Replacement string         `json:"replacement"`
	IsRegex     bool           `json:"isRegex"`
	Pattern     *regexp.Regexp `json:"-"`
}

// BrokenRule 记录编译失败的规则
type BrokenRule struct {
	Category string `json:"category"`
	Pattern  string `json:"pattern"`
	Error    string `json:"error"`
}

// VendorSanitizer 分厂商与命令维度的敏感信息脱敏引擎
type VendorSanitizer struct {
	mu           sync.RWMutex
	rules        []VendorSanitizeRule
	rulesByCat   map[string][]VendorSanitizeRule
	genericRules []VendorSanitizeRule
	broken       []BrokenRule
}

var (
	defaultVendorSanitizer     *VendorSanitizer
	defaultVendorSanitizerOnce sync.Once
)

// GetDefaultVendorSanitizer 获取全局单例分厂商脱敏引擎
func GetDefaultVendorSanitizer() *VendorSanitizer {
	defaultVendorSanitizerOnce.Do(func() {
		vs, err := NewVendorSanitizerFromBytes(embeddedSensitiveCmdJSON)
		if err != nil {
			defaultVendorSanitizer = &VendorSanitizer{
				rulesByCat: make(map[string][]VendorSanitizeRule),
			}
			return
		}
		defaultVendorSanitizer = vs
	})
	return defaultVendorSanitizer
}

// NewVendorSanitizer 创建脱敏引擎
func NewVendorSanitizer() (*VendorSanitizer, error) {
	return NewVendorSanitizerFromBytes(embeddedSensitiveCmdJSON)
}

// NewVendorSanitizerFromBytes 从 JSON 字节流初始化脱敏引擎
func NewVendorSanitizerFromBytes(data []byte) (*VendorSanitizer, error) {
	if len(data) == 0 {
		return &VendorSanitizer{
			rulesByCat: make(map[string][]VendorSanitizeRule),
		}, nil
	}

	var rawRules []VendorSanitizeRule
	if err := json.Unmarshal(data, &rawRules); err != nil {
		return nil, fmt.Errorf("反序列化脱敏规则失败: %w", err)
	}

	vs := &VendorSanitizer{
		rulesByCat: make(map[string][]VendorSanitizeRule),
	}

	for _, r := range rawRules {
		replacement := r.Replacement
		if replacement == "" {
			replacement = "****"
		}
		r.Replacement = replacement

		// 编译正则表达式
		re, err := regexp.Compile(r.RawPattern)
		if err != nil {
			vs.broken = append(vs.broken, BrokenRule{
				Category: r.Category,
				Pattern:  r.RawPattern,
				Error:    err.Error(),
			})
			continue
		}
		r.Pattern = re
		vs.rules = append(vs.rules, r)

		catKey := strings.ToLower(strings.TrimSpace(r.Category))
		if catKey == "" {
			vs.genericRules = append(vs.genericRules, r)
		} else {
			vs.rulesByCat[catKey] = append(vs.rulesByCat[catKey], r)
		}
	}

	return vs, nil
}

// BrokenRules 返回编译失败的规则清单
func (vs *VendorSanitizer) BrokenRules() []BrokenRule {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	res := make([]BrokenRule, len(vs.broken))
	copy(res, vs.broken)
	return res
}

// TotalRules 返回已加载规则总数
func (vs *VendorSanitizer) TotalRules() int {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	return len(vs.rules)
}

// Categories 返回所包含的所有品类/厂商标识
func (vs *VendorSanitizer) Categories() []string {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	var cats []string
	for k := range vs.rulesByCat {
		cats = append(cats, k)
	}
	return cats
}

// ResolveCategory 根据厂商和型号/系列映射到 eDeskPro Category
func ResolveCategory(vendor, seriesOrModel string) string {
	v := strings.ToLower(strings.TrimSpace(vendor))
	s := strings.ToUpper(strings.TrimSpace(seriesOrModel))

	switch {
	case strings.Contains(v, "cisco"):
		return "CISCO"
	case strings.Contains(v, "h3c"):
		return "H3C"
	case strings.Contains(v, "zte"):
		return "ZTE"
	case strings.Contains(v, "juniper"):
		return "JUNIPER"
	case strings.Contains(v, "ruijie"):
		return "RUIJIE"
	case strings.Contains(v, "alu") || strings.Contains(v, "alcatel"):
		return "ALU"
	case strings.Contains(v, "nokia"):
		return "NOKIA"
	case strings.Contains(v, "sangfor"):
		return "SANGFOR"
	case strings.Contains(v, "dptech"):
		return "DPTECH"
	case strings.Contains(v, "dell"):
		return "DELL"
	case strings.Contains(v, "ruckus"):
		return "RUCKUS"
	case strings.Contains(v, "huawei") || v == "hw" || v == "":
		// 华为细分
		if strings.HasPrefix(s, "AR") || strings.Contains(s, "ROUTER") {
			return "AR router"
		}
		if strings.HasPrefix(s, "NE") || strings.Contains(s, "NETENGINE") {
			return "NE router"
		}
		if strings.HasPrefix(s, "CE") || strings.Contains(s, "CLOUDENGINE") {
			return "DC"
		}
		if strings.HasPrefix(s, "USG") || strings.Contains(s, "SEC") || strings.Contains(s, "FIREWALL") {
			return "Firewall"
		}
		if strings.HasPrefix(s, "AC") || strings.HasPrefix(s, "AP") || strings.HasPrefix(s, "AIRENGINE") {
			return "WLAN"
		}
		if strings.HasPrefix(s, "S") || strings.Contains(s, "SWITCH") {
			return "Ethernet Switch"
		}
		return "Ethernet Switch" // 默认交换机
	default:
		return ""
	}
}

// matchCommand 判定规则是否适用于指定命令
func matchCommand(ruleCmds []string, targetCmd string) bool {
	if len(ruleCmds) == 0 {
		return true // 无命令限制，全量适用
	}
	t := strings.ToLower(strings.TrimSpace(targetCmd))
	if t == "" {
		return true
	}

	for _, rc := range ruleCmds {
		rcLower := strings.ToLower(strings.TrimSpace(rc))
		if strings.HasSuffix(rcLower, "*") {
			prefix := strings.TrimSuffix(rcLower, "*")
			if strings.HasPrefix(t, prefix) {
				return true
			}
		} else if strings.Contains(rcLower, "|") {
			// 支持 include 管道类
			if strings.Contains(t, rcLower) {
				return true
			}
		} else {
			if strings.HasPrefix(t, rcLower) || strings.EqualFold(t, rcLower) {
				return true
			}
		}
	}
	return false
}

// Sanitize 对给定内容按厂商分类和命令进行敏感信息掩码
func (vs *VendorSanitizer) Sanitize(category, command, text string) string {
	if text == "" {
		return ""
	}

	vs.mu.RLock()
	defer vs.mu.RUnlock()

	catKey := strings.ToLower(strings.TrimSpace(category))
	matchedRules := vs.rulesByCat[catKey]

	currentText := text

	// 1. 先应用该 Category 匹配到的规则
	for _, rule := range matchedRules {
		if rule.Pattern == nil {
			continue
		}
		if !matchCommand(rule.Commands, command) {
			continue
		}
		currentText = applyMaskRule(rule.Pattern, rule.Replacement, currentText)
	}

	// 2. 补充通用（空 category）规则
	for _, rule := range vs.genericRules {
		if rule.Pattern == nil {
			continue
		}
		if !matchCommand(rule.Commands, command) {
			continue
		}
		currentText = applyMaskRule(rule.Pattern, rule.Replacement, currentText)
	}

	return currentText
}

// applyMaskRule 执行单条规则正则替换，支持捕获组掩码
func applyMaskRule(re *regexp.Regexp, replacement, text string) string {
	indices := re.FindAllStringSubmatchIndex(text, -1)
	if len(indices) == 0 {
		return text
	}

	var sb strings.Builder
	lastIdx := 0

	for _, loc := range indices {
		// loc: [matchStart, matchEnd, group1Start, group1End, ...]
		if len(loc) >= 4 && loc[2] >= 0 && loc[3] >= 0 {
			// 有捕获组 1，只替换捕获组 1 内容
			g1Start := loc[2]
			g1End := loc[3]
			sb.WriteString(text[lastIdx:g1Start])
			sb.WriteString(replacement)
			lastIdx = g1End
		} else {
			// 无有效捕获组，替换整个匹配段
			matchStart := loc[0]
			matchEnd := loc[1]
			sb.WriteString(text[lastIdx:matchStart])
			sb.WriteString(replacement)
			lastIdx = matchEnd
		}
	}
	sb.WriteString(text[lastIdx:])
	return sb.String()
}
