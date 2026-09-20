package inspection

import (
	"embed"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
)

// validAssertTypes 支持的断言类型集合（P2-8 字段完整性校验）
var validAssertTypes = map[string]bool{
	"must_not_contain": true,
	"must_contain":     true,
	"regex":            true,
	"threshold":        true,
	"bound":            true,
	"numeric":          true,
	"equals":           true,
}

//go:embed rules/builtin/*.json
var builtinRulesFS embed.FS

// LocaleText 多语言文案
type LocaleText struct {
	ZH string `json:"zh"`
	EN string `json:"en"`
}

// RuleScope 规则生效范围
type RuleScope struct {
	Vendors  []string `json:"vendors,omitempty"`
	Models   []string `json:"models,omitempty"`   // 正则或前缀
	Versions []string `json:"versions,omitempty"` // 正则
}

// FieldExtract 字段提取规约
type FieldExtract struct {
	Name    string `json:"name"`
	Pattern string `json:"pattern"`
	Group   int    `json:"group"`
}

// ExtractSpec 数据提取规约
type ExtractSpec struct {
	BlockRegex string         `json:"blockRegex,omitempty"` // 分块正则
	Fields     []FieldExtract `json:"fields"`
}

// AssertSpec 断言规约
type AssertSpec struct {
	Type     string      `json:"type"`     // threshold | must_contain | must_not_contain | regex | equals | bound
	Field    string      `json:"field"`    // 判定的字段名
	Expr     interface{} `json:"expr"`     // 判定表达式 (数值阈值或字符串或正则)
	Operator string      `json:"operator"` // < | <= | > | >= | == | != | between
}

// PreCollectItem 前置采集规约（B11：前置采集项，结果进入上下文供后续检查项复用）
type PreCollectItem struct {
	Name        string      `json:"name"`        // 前置项名称/标识
	Command     string      `json:"command"`     // 执行命令
	Extract     ExtractSpec `json:"extract"`     // 提取规约
	StoreAs     string      `json:"storeAs"`     // 存储到上下文中的变量名
	Description string      `json:"description"` // 说明
}

// DSLRule 声明式巡检规则
type DSLRule struct {
	CheckNo       string           `json:"checkno"`
	Title         LocaleText       `json:"title"`
	Category      string           `json:"category"`                // Health | Reliability | BGP | OSPF | BASE
	RiskLevel     string           `json:"riskLevel"`               // critical | major | minor | info
	Commands      []string         `json:"commands"`                // 依赖的 CLI 命令
	Scope         RuleScope        `json:"scope"`                   // 适用范围
	Extract       ExtractSpec      `json:"extract"`                 // 数据提取规约
	Assert        AssertSpec       `json:"assert"`                  // 断言判定规约
	Advice        LocaleText       `json:"advice"`                  // 修复建议
	PreCollects   []PreCollectItem `json:"preCollects,omitempty"`   // B11：前置采集项规约
	ParentCheckNo string           `json:"parentCheckNo,omitempty"` // B11：继承父规则 CheckNo
	IsBig         bool             `json:"isBig,omitempty"`         // B11：大表标记（触发流式落盘保护）
}

// DSLInterpreter 规则 DSL 解释器
type DSLInterpreter struct {
	mu       sync.RWMutex
	rules    []*DSLRule
	builtins map[string]bool // 内置规则编号集合（用于覆盖告警）
	imported map[string]bool // 经导入规则集（用于持久化导出）
}

var (
	globalDSLInterpreter *DSLInterpreter
	dslOnce              sync.Once
)

// GetGlobalDSLInterpreter 获取全局 DSL 解释器单例
func GetGlobalDSLInterpreter() *DSLInterpreter {
	dslOnce.Do(func() {
		globalDSLInterpreter = NewDSLInterpreter()
		_ = globalDSLInterpreter.LoadBuiltin()
	})
	return globalDSLInterpreter
}

// NewDSLInterpreter 创建 DSL 解释器
func NewDSLInterpreter() *DSLInterpreter {
	return &DSLInterpreter{
		rules:    make([]*DSLRule, 0),
		builtins: make(map[string]bool),
		imported: make(map[string]bool),
	}
}

// LoadBuiltin 从内嵌目录加载内置规则库
func (di *DSLInterpreter) LoadBuiltin() error {
	di.mu.Lock()
	defer di.mu.Unlock()

	entries, err := builtinRulesFS.ReadDir("rules/builtin")
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".json") {
			data, err := builtinRulesFS.ReadFile("rules/builtin/" + entry.Name())
			if err != nil {
				continue
			}
			var ruleList []DSLRule
			if err := json.Unmarshal(data, &ruleList); err == nil {
				for i := range ruleList {
					r := ruleList[i]
					di.rules = append(di.rules, &r)
					if r.CheckNo != "" {
						di.builtins[r.CheckNo] = true
					}
				}
			}
		}
	}
	return nil
}

// AllRules 返回所有已加载的规则
func (di *DSLInterpreter) AllRules() []*DSLRule {
	di.mu.RLock()
	defer di.mu.RUnlock()
	res := make([]*DSLRule, len(di.rules))
	copy(res, di.rules)
	return res
}

// RulesByCategory 按分类过滤已加载的规则
func (di *DSLInterpreter) RulesByCategory(category string) []*DSLRule {
	di.mu.RLock()
	defer di.mu.RUnlock()
	if category == "" || category == "*" {
		res := make([]*DSLRule, len(di.rules))
		copy(res, di.rules)
		return res
	}
	var res []*DSLRule
	for _, r := range di.rules {
		if strings.EqualFold(r.Category, category) {
			res = append(res, r)
		}
	}
	return res
}

// ExportRulesJSON 导出所有规则为 JSON
func (di *DSLInterpreter) ExportRulesJSON() (string, error) {
	di.mu.RLock()
	defer di.mu.RUnlock()
	data, err := json.MarshalIndent(di.rules, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ImportRulesJSON 动态导入 DSL 规则，支持去重覆盖
func (di *DSLInterpreter) ImportRulesJSON(data []byte) (int, error) {
	var imported []DSLRule
	if err := json.Unmarshal(data, &imported); err != nil {
		return 0, fmt.Errorf("解析 DSL 规则 JSON 失败: %w", err)
	}

	// 预先校验每条规则中的正则合法性，防止非法正则污染运行时
	for i := range imported {
		rule := &imported[i]
		if rule.Extract.BlockRegex != "" {
			if _, err := regexp.Compile("(?im)" + rule.Extract.BlockRegex); err != nil {
				return 0, fmt.Errorf("规则 [%s] 分块正则非法: %w", rule.CheckNo, err)
			}
		}
		for _, f := range rule.Extract.Fields {
			if f.Pattern != "" {
				if _, err := regexp.Compile("(?im)" + f.Pattern); err != nil {
					return 0, fmt.Errorf("规则 [%s] 字段 [%s] 正则非法: %w", rule.CheckNo, f.Name, err)
				}
			}
		}
		if rule.Assert.Type == "regex" && rule.Assert.Expr != nil {
			if exprStr, ok := rule.Assert.Expr.(string); ok && exprStr != "" {
				if _, err := regexp.Compile(exprStr); err != nil {
					return 0, fmt.Errorf("规则 [%s] 断言正则非法: %w", rule.CheckNo, err)
				}
			}
		}
		// P2-8：字段完整性校验（断言类型合法，且至少声明命令或断言）
		if rule.Assert.Type != "" && !validAssertTypes[strings.ToLower(rule.Assert.Type)] {
			return 0, fmt.Errorf("规则 [%s] 断言类型不受支持: %s", rule.CheckNo, rule.Assert.Type)
		}
		if len(rule.Commands) == 0 && rule.Assert.Type == "" {
			return 0, fmt.Errorf("规则 [%s] 缺少 commands 与 assert，无法执行", rule.CheckNo)
		}
	}

	di.mu.Lock()
	defer di.mu.Unlock()

	count := 0
	for i := range imported {
		rule := imported[i]
		if rule.CheckNo == "" {
			logger.Warn("DSL", "-", "忽略缺少 checkno 的导入规则")
			continue
		}
		if di.builtins[rule.CheckNo] {
			// P2-8：允许覆盖内置规则，但必须留下明确告警（便于审计与回滚）
			logger.Warn("DSL", "-", "导入规则覆盖内置规则: %s", rule.CheckNo)
		}
		// 查找是否已存在，存在则更新，不存在则追加
		found := false
		for idx, existing := range di.rules {
			if existing.CheckNo == rule.CheckNo {
				di.rules[idx] = &rule
				found = true
				break
			}
		}
		if !found {
			di.rules = append(di.rules, &rule)
		}
		di.imported[rule.CheckNo] = true
		count++
	}
	return count, nil
}

// ExportImportedRulesJSON 导出"经导入的规则"为 JSON（用于持久化，重启后可恢复）
func (di *DSLInterpreter) ExportImportedRulesJSON() (string, error) {
	di.mu.RLock()
	defer di.mu.RUnlock()

	rules := make([]*DSLRule, 0, len(di.imported))
	for _, r := range di.rules {
		if di.imported[r.CheckNo] {
			rules = append(rules, r)
		}
	}
	data, err := json.MarshalIndent(rules, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ImportedRuleCount 返回当前生效的导入规则数量
func (di *DSLInterpreter) ImportedRuleCount() int {
	di.mu.RLock()
	defer di.mu.RUnlock()
	return len(di.imported)
}

// FindRule 根据 CheckNo 查找规则
func (di *DSLInterpreter) FindRule(checkNo string) *DSLRule {
	di.mu.RLock()
	defer di.mu.RUnlock()
	for _, r := range di.rules {
		if strings.EqualFold(r.CheckNo, checkNo) {
			return r
		}
	}
	return nil
}

// RulesForCommand 返回依赖指定命令的 DSL 规则（大小写不敏感，支持前缀匹配）。
// 用于按命令维度触发判定：内置规则编号与模板检查项编号不同，不能仅依赖 CheckNo 命中。
func (di *DSLInterpreter) RulesForCommand(command string) []*DSLRule {
	di.mu.RLock()
	defer di.mu.RUnlock()

	cmd := strings.ToLower(strings.TrimSpace(command))
	if cmd == "" {
		return nil
	}
	var res []*DSLRule
	for _, r := range di.rules {
		for _, c := range r.Commands {
			cLower := strings.ToLower(strings.TrimSpace(c))
			if cLower == "" {
				continue
			}
			if cLower == cmd || strings.HasPrefix(cLower, cmd) || strings.HasPrefix(cmd, cLower) {
				res = append(res, r)
				break
			}
		}
	}
	return res
}

// ExecutePreCollect 执行前置采集提取规约，返回存储键名与提取值
func (di *DSLInterpreter) ExecutePreCollect(item *PreCollectItem, rawEcho string) (string, string) {
	if item == nil || item.StoreAs == "" {
		return "", ""
	}
	for _, f := range item.Extract.Fields {
		re, err := regexp.Compile("(?im)" + f.Pattern)
		if err != nil {
			continue
		}
		matches := re.FindStringSubmatch(rawEcho)
		if len(matches) > f.Group && f.Group >= 0 {
			return item.StoreAs, strings.TrimSpace(matches[f.Group])
		} else if len(matches) > 1 {
			return item.StoreAs, strings.TrimSpace(matches[1])
		} else if len(matches) > 0 {
			return item.StoreAs, strings.TrimSpace(matches[0])
		}
	}
	return item.StoreAs, ""
}

// MatchesScope 检查设备画像是否命中规则适用范围
func (r *DSLRule) MatchesScope(vendor, model, version string) bool {
	v := strings.ToLower(strings.TrimSpace(vendor))
	m := strings.ToLower(strings.TrimSpace(model))
	ver := strings.ToLower(strings.TrimSpace(version))

	// 1. 厂商校验
	if len(r.Scope.Vendors) > 0 {
		matched := false
		for _, reqV := range r.Scope.Vendors {
			reqVLower := strings.ToLower(strings.TrimSpace(reqV))
			if reqVLower == "*" || reqVLower == v {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// 2. 款型正则校验
	if len(r.Scope.Models) > 0 {
		matched := false
		for _, pat := range r.Scope.Models {
			if pat == "*" || strings.Contains(m, strings.ToLower(pat)) {
				matched = true
				break
			}
			if re, err := regexp.Compile("(?i)" + pat); err == nil && re.MatchString(m) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// 3. 版本正则校验
	if len(r.Scope.Versions) > 0 {
		matched := false
		for _, pat := range r.Scope.Versions {
			if pat == "*" || strings.Contains(ver, strings.ToLower(pat)) {
				matched = true
				break
			}
			if re, err := regexp.Compile("(?i)" + pat); err == nil && re.MatchString(ver) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	return true
}

// Evaluate 根据 DSL 规则评估回显数据
func (di *DSLInterpreter) Evaluate(rule *DSLRule, input *EvaluateInput) models.InspectionResult {
	if rule == nil || input == nil {
		return models.InspectionResult{
			Status:  string(ResultExcept),
			Problem: "规则或输入为空",
		}
	}

	result := models.InspectionResult{
		RunID:       input.RunID,
		DeviceIP:    input.DeviceIP,
		ItemCode:    rule.CheckNo,
		ItemName:    rule.Title.ZH,
		Category:    rule.Category,
		Severity:    rule.RiskLevel,
		Status:      string(ResultPass), // 默认通过
		Problem:     "",
		Advice:      rule.Advice.ZH,
		ActualValue: "",
	}

	// 0. 支持父规则继承 (B11)
	effectiveRule := *rule
	if effectiveRule.ParentCheckNo != "" {
		if parent := di.FindRule(effectiveRule.ParentCheckNo); parent != nil {
			if effectiveRule.Assert.Type == "" {
				effectiveRule.Assert = parent.Assert
			}
			if len(effectiveRule.Extract.Fields) == 0 {
				effectiveRule.Extract = parent.Extract
			}
			if len(effectiveRule.Scope.Vendors) == 0 {
				effectiveRule.Scope = parent.Scope
			}
		}
	}
	rule = &effectiveRule

	rawEcho := input.RawEcho
	if strings.TrimSpace(rawEcho) == "" && len(input.ParsedRows) == 0 && len(input.ContextVars) == 0 {
		result.Status = string(ResultExcept)
		result.Problem = fmt.Sprintf("未获取到检查项 [%s] 依赖的命令回显", rule.CheckNo)
		return result
	}

	// 1. 数据提取
	extractedValues := make(map[string]string)
	for _, f := range rule.Extract.Fields {
		re, err := regexp.Compile("(?im)" + f.Pattern)
		if err != nil {
			continue
		}
		matches := re.FindStringSubmatch(rawEcho)
		if len(matches) > f.Group && f.Group >= 0 {
			extractedValues[f.Name] = strings.TrimSpace(matches[f.Group])
		} else if len(matches) > 1 {
			extractedValues[f.Name] = strings.TrimSpace(matches[1])
		} else if len(matches) > 0 {
			extractedValues[f.Name] = strings.TrimSpace(matches[0])
		}
	}

	// 2. 执行断言
	targetField := rule.Assert.Field
	actualVal := extractedValues[targetField]
	if actualVal == "" && len(input.ParsedRows) > 0 {
		if v, ok := input.ParsedRows[0][targetField]; ok {
			actualVal = fmt.Sprintf("%v", v)
		}
	}
	// B11: 从前置上下文变量中获取 (跨项复用)
	if actualVal == "" && len(input.ContextVars) > 0 {
		if v, ok := input.ContextVars[targetField]; ok {
			actualVal = v
		}
	}
	result.ActualValue = actualVal

	assertType := strings.ToLower(rule.Assert.Type)
	switch assertType {
	case "must_not_contain":
		exprStr := fmt.Sprintf("%v", rule.Assert.Expr)
		if strings.Contains(rawEcho, exprStr) || (actualVal != "" && strings.Contains(actualVal, exprStr)) {
			result.Status = string(ResultFail)
			result.Problem = fmt.Sprintf("发现异常关键字 [%s]", exprStr)
		}
	case "must_contain":
		exprStr := fmt.Sprintf("%v", rule.Assert.Expr)
		if !strings.Contains(rawEcho, exprStr) && !strings.Contains(actualVal, exprStr) {
			result.Status = string(ResultFail)
			result.Problem = fmt.Sprintf("未匹配到预期关键字 [%s]", exprStr)
		}
	case "regex":
		pat := fmt.Sprintf("%v", rule.Assert.Expr)
		re, err := regexp.Compile("(?im)" + pat)
		if err == nil {
			textToMatch := actualVal
			if textToMatch == "" {
				textToMatch = rawEcho
			}
			if !re.MatchString(textToMatch) {
				result.Status = string(ResultFail)
				result.Problem = fmt.Sprintf("未满足正则预期: %s", pat)
			}
		}
	case "threshold", "bound", "numeric":
		numVal, err := strconv.ParseFloat(actualVal, 64)
		if err != nil {
			// 如果提取到的不是纯数字，在原始文本中寻找数字或按失败处理
			result.Status = string(ResultExcept)
			result.Problem = fmt.Sprintf("无法解析数值指标: %q", actualVal)
		} else {
			limit, _ := strconv.ParseFloat(fmt.Sprintf("%v", rule.Assert.Expr), 64)
			op := rule.Assert.Operator
			if op == "" {
				op = "<=" // 默认上限
			}
			exceeded := false
			switch op {
			case "<":
				exceeded = !(numVal < limit)
			case "<=":
				exceeded = !(numVal <= limit)
			case ">":
				exceeded = !(numVal > limit)
			case ">=":
				exceeded = !(numVal >= limit)
			case "==":
				exceeded = (numVal != limit)
			}
			if exceeded {
				result.Status = string(ResultFail)
				result.Problem = fmt.Sprintf("指标超出安全阈值: 实际值 %v %s 门限 %v", numVal, op, limit)
			}
		}
	case "equals":
		expected := fmt.Sprintf("%v", rule.Assert.Expr)
		if !strings.EqualFold(strings.TrimSpace(actualVal), strings.TrimSpace(expected)) {
			result.Status = string(ResultFail)
			result.Problem = fmt.Sprintf("状态不符: 实际 %q, 期望 %q", actualVal, expected)
		}
	}

	return result
}
