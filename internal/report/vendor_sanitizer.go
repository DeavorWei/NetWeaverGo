package report

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/NetWeaverGo/core/internal/logger"
)

//go:embed rules/sensitive_cmd.json
var embeddedSensitiveCmdJSON []byte

func init() {
	logger.RegisterExtraSanitizer(func(vendor, command, text string) string {
		cat := ResolveCategory(vendor, "")
		return GetDefaultVendorSanitizer().Sanitize(cat, command, text)
	})
}

// VendorSanitizeRule 单条厂商脱敏规则
type VendorSanitizeRule struct {
	Category    string         `json:"category"`
	Commands    []string       `json:"commands"`
	RawPattern  string         `json:"pattern"`
	Replacement string         `json:"replacement"`
	IsRegex     bool           `json:"isRegex"`
	Pattern     *regexp.Regexp `json:"-"`
	Literal     string         `json:"-"` // 前导字面量（性能优化：不包含则跳过正则）
}

// literalPrefix 提取正则的"必然出现"字面量前缀（跳过零宽前缀后遇到元字符即停止），
// 用于快速预筛：文本中不含该字面量时，正则必然无法匹配，可安全跳过。
// 返回空串表示无法预筛（该规则始终执行）。
func literalPrefix(raw string) string {
	s := strings.TrimSpace(raw)
	// 1. 剥离零宽前缀（不引入字面量约束）
	zeroWidth := []string{"(?im)", "(?mi)", "(?i)", "(?m)", `\A`, "^", `\s*`, `\s+`, `\s`, `\b`}
	for {
		trimmed := false
		for _, p := range zeroWidth {
			if strings.HasPrefix(s, p) {
				s = s[len(p):]
				trimmed = true
			}
		}
		if !trimmed {
			break
		}
	}

	// 2. 提取到第一个元字符为止
	end := len(s)
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\', '(', '[', '{', '.', '*', '+', '?', '|', '^', '$':
			end = i
			i = len(s)
		}
	}
	lit := strings.ToLower(strings.TrimSpace(s[:end]))
	if len(lit) < 3 {
		// 过短字面量（如空串、"a"）误筛风险与收益均低，不做预筛
		return ""
	}
	return lit
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
			logger.Error("VendorSanitizer", "-", "加载内置分厂商脱敏规则失败: %v", err)
			defaultVendorSanitizer = &VendorSanitizer{
				rulesByCat: make(map[string][]VendorSanitizeRule),
			}
			return
		}
		if len(vs.broken) > 0 {
			logger.Warn("VendorSanitizer", "-", "内置分厂商脱敏规则包含 %d 条不兼容/损坏正则 (有效: %d 条)", len(vs.broken), len(vs.rules))
		}
		defaultVendorSanitizer = vs
	})
	return defaultVendorSanitizer
}

// NewVendorSanitizer 创建脱敏引擎
func NewVendorSanitizer() (*VendorSanitizer, error) {
	return NewVendorSanitizerFromBytes(embeddedSensitiveCmdJSON)
}

// SanitizeContent 对导出/落盘内容执行"全局规则 + 分厂商规则"两级脱敏（P0-1 生产统一入口）。
// vendor/model 用于解析厂商品类（为空时仅应用通用规则）；command 用于命令维度规则过滤。
// 返回值为脱敏后的文本，调用方随后应执行 ValidateExportContent 兜底自检。
func SanitizeContent(vendor, model, command, content string) string {
	if content == "" {
		return content
	}
	// 1. 全局基础规则（与日志管道同源，保证口径一致）
	masked := logger.GetGlobalSanitizer().Sanitize(content)
	// 2. 厂商/命令维度规则（引擎内部会追加通用 category 规则）
	category := ResolveCategory(vendor, model)
	return GetDefaultVendorSanitizer().Sanitize(category, command, masked)
}

// LogSanitizerHealth 输出脱敏引擎启动自检结果：
// 无损坏规则时 INFO 汇总，存在损坏规则时逐条输出 WARN 明细（P2-3 启动告警闭环）。
func LogSanitizerHealth() {
	vs := GetDefaultVendorSanitizer()
	broken := vs.BrokenRules()
	if len(broken) == 0 {
		logger.Info("VendorSanitizer", "-", "分厂商脱敏规则加载完成: 有效规则 %d 条, 损坏规则 0 条", vs.TotalRules())
		return
	}
	for _, r := range broken {
		logger.Warn("VendorSanitizer", "-", "脱敏规则编译失败(已跳过): category=%s pattern=%s err=%s", r.Category, r.Pattern, r.Error)
	}
	logger.Warn("VendorSanitizer", "-", "分厂商脱敏规则自检完成: 有效 %d 条, 损坏 %d 条", vs.TotalRules(), len(broken))
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

		// 编译正则表达式（若显式标记非正则或原串为字面量，转义以保证匹配）
		var re *regexp.Regexp
		var err error
		if r.IsRegex {
			re, err = regexp.Compile(r.RawPattern)
		} else {
			re, err = regexp.Compile(regexp.QuoteMeta(r.RawPattern))
		}

		if err != nil {
			vs.broken = append(vs.broken, BrokenRule{
				Category: r.Category,
				Pattern:  r.RawPattern,
				Error:    err.Error(),
			})
			continue
		}
		r.Pattern = re
		r.Literal = literalPrefix(r.RawPattern)
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
		return "" // 锐捷在 eDesk 数据中无独立 category 规则，对齐数据回退为通用分类
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
	case v == "":
		// 无厂商上下文时不下发任何厂商推断，仅应用通用（空 category）规则，避免跨厂商规则误用
		return ""
	case strings.Contains(v, "huawei") || v == "hw":
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
	// 性能优化：脱敏规则的前导字面量若在文本中不存在，则可安全跳过该正则；
	// 掩码替换只会删除原文/插入 ****，不会引入新的敏感锚点，故可基于初始文本判定一次。
	lowerText := strings.ToLower(currentText)

	shouldRun := func(rule VendorSanitizeRule) bool {
		if rule.Pattern == nil {
			return false
		}
		if rule.Literal != "" && !strings.Contains(lowerText, rule.Literal) {
			return false
		}
		return matchCommand(rule.Commands, command)
	}

	// 1. 先应用该 Category 匹配到的规则
	for _, rule := range matchedRules {
		if !shouldRun(rule) {
			continue
		}
		currentText = applyMaskRule(rule.Pattern, rule.Replacement, currentText)
	}

	// 2. 补充通用（空 category）规则
	for _, rule := range vs.genericRules {
		if !shouldRun(rule) {
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
