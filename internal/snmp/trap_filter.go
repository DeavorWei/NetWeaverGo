// Package snmp 提供 SNMP 核心业务功能
// trap_filter.go 实现 Trap 过滤规则引擎
package snmp

import (
	"net"
	"regexp"
	"strings"
	"sync"

	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
)

// ============================================================================
// 过滤引擎
// ============================================================================

// TrapFilterEngine Trap 过滤规则引擎
// 支持多种匹配类型：OID前缀、源IP(CIDR)、严重级别、Community、正则表达式
type TrapFilterEngine struct {
	rules []*models.SNMPTrapFilterRule
	mu    sync.RWMutex

	// 预编译的正则表达式缓存
	regexCache map[uint]*regexp.Regexp
	// 预编译的 CIDR 网络缓存
	cidrCache map[uint]*net.IPNet
}

// NewTrapFilterEngine 创建过滤引擎
func NewTrapFilterEngine(rules []*models.SNMPTrapFilterRule) *TrapFilterEngine {
	engine := &TrapFilterEngine{
		rules:      rules,
		regexCache: make(map[uint]*regexp.Regexp),
		cidrCache:  make(map[uint]*net.IPNet),
	}

	// 预编译规则
	engine.compileRules()

	// 统计编译结果
	regexCount := len(engine.regexCache)
	cidrCount := len(engine.cidrCache)
	logger.Info("SNMP-Filter", "-", "过滤引擎已初始化: 规则数=%d, 正则规则=%d, CIDR规则=%d",
		len(rules), regexCount, cidrCount)
	return engine
}

// ============================================================================
// 核心匹配方法
// ============================================================================

// Match 检查 Trap 是否匹配任何过滤规则
// 返回: (是否匹配, 匹配的规则, 错误)
// 按优先级顺序检查规则，返回第一个匹配的规则
func (e *TrapFilterEngine) Match(trap *models.SNMPTrapRecord) (bool, *models.SNMPTrapFilterRule, error) {
	if trap == nil {
		return false, nil, nil
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, rule := range e.rules {
		// 跳过禁用的规则
		if !rule.Enabled {
			continue
		}

		matched, err := e.matchRule(trap, rule)
		if err != nil {
			logger.Error("SNMP-Filter", "-", "规则匹配错误: RuleID=%d, %v", rule.ID, err)
			continue
		}

		if matched {
			logger.Debug("SNMP-Filter", "-", "Trap 匹配规则: OID=%s, RuleID=%d, Action=%s",
				trap.TrapOID, rule.ID, rule.Action)
			return true, rule, nil
		}
	}

	return false, nil, nil
}

// ShouldFilter 判断 Trap 是否应被过滤掉
// 如果匹配到 action=drop 的规则，返回 true
// 如果匹配到 action=accept 的规则，返回 false
// 如果没有匹配任何规则，返回 false（默认接受）
func (e *TrapFilterEngine) ShouldFilter(trap *models.SNMPTrapRecord) bool {
	matched, rule, _ := e.Match(trap)
	if !matched || rule == nil {
		return false // 默认接受
	}

	return rule.Action == "drop"
}

// GetSeverityOverride 获取严重级别覆盖值
// 如果匹配到 action=severity_override 的规则，返回覆盖的严重级别
func (e *TrapFilterEngine) GetSeverityOverride(trap *models.SNMPTrapRecord) string {
	matched, rule, _ := e.Match(trap)
	if !matched || rule == nil {
		return ""
	}

	if rule.Action == "severity_override" && rule.OverrideSeverity != "" {
		return rule.OverrideSeverity
	}

	return ""
}

// ============================================================================
// 规则管理
// ============================================================================

// AddRule 添加过滤规则
func (e *TrapFilterEngine) AddRule(rule *models.SNMPTrapFilterRule) {
	e.mu.Lock()
	defer e.mu.Unlock()

	ruleCount := len(e.rules)
	e.rules = append(e.rules, rule)
	e.compileSingleRule(rule)

	// 按优先级排序
	e.sortRules()

	logger.Info("SNMP-Filter", "-", "添加过滤规则: ID=%d, 名称=%s, 动作=%s, 优先级=%d (规则数: %d -> %d)",
		rule.ID, rule.Name, rule.Action, rule.Priority, ruleCount, len(e.rules))
}

// RemoveRule 移除过滤规则
func (e *TrapFilterEngine) RemoveRule(ruleID uint) {
	e.mu.Lock()
	defer e.mu.Unlock()

	ruleCount := len(e.rules)
	for i, rule := range e.rules {
		if rule.ID == ruleID {
			// 清理缓存
			delete(e.regexCache, ruleID)
			delete(e.cidrCache, ruleID)

			// 移除规则
			e.rules = append(e.rules[:i], e.rules[i+1:]...)
			logger.Info("SNMP-Filter", "-", "移除过滤规则: ID=%d, 名称=%s (规则数: %d -> %d)",
				ruleID, rule.Name, ruleCount, len(e.rules))
			return
		}
	}
	logger.Warn("SNMP-Filter", "-", "移除过滤规则失败: 规则不存在 ID=%d", ruleID)
}

// UpdateRules 更新所有规则（完全替换）
func (e *TrapFilterEngine) UpdateRules(rules []*models.SNMPTrapFilterRule) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// 清理所有缓存
	e.regexCache = make(map[uint]*regexp.Regexp)
	e.cidrCache = make(map[uint]*net.IPNet)

	e.rules = rules
	e.compileRules()

	logger.Info("SNMP-Filter", "-", "更新所有过滤规则，规则数: %d", len(rules))
}

// UpdateSingleRule 更新单个规则
func (e *TrapFilterEngine) UpdateSingleRule(rule *models.SNMPTrapFilterRule) {
	e.mu.Lock()
	defer e.mu.Unlock()

	for i, existingRule := range e.rules {
		if existingRule.ID == rule.ID {
			// 清理旧缓存
			delete(e.regexCache, rule.ID)
			delete(e.cidrCache, rule.ID)

			// 更新规则
			e.rules[i] = rule
			e.compileSingleRule(rule)

			// 重新排序
			e.sortRules()

			logger.Info("SNMP-Filter", "-", "更新过滤规则: ID=%d, Name=%s", rule.ID, rule.Name)
			return
		}
	}

	// 如果规则不存在，添加它
	e.rules = append(e.rules, rule)
	e.compileSingleRule(rule)
	e.sortRules()
}

// GetRules 获取所有规则（按优先级排序）
func (e *TrapFilterEngine) GetRules() []*models.SNMPTrapFilterRule {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// 返回副本，避免外部修改
	rulesCopy := make([]*models.SNMPTrapFilterRule, len(e.rules))
	copy(rulesCopy, e.rules)
	return rulesCopy
}

// ============================================================================
// 内部方法
// ============================================================================

// matchRule 检查单个规则是否匹配
func (e *TrapFilterEngine) matchRule(trap *models.SNMPTrapRecord, rule *models.SNMPTrapFilterRule) (bool, error) {
	// 所有条件都是可选的，只有设置的条件才需要匹配
	// 多个条件之间是 AND 关系

	// 1. OID 前缀匹配
	if rule.OIDPattern != "" {
		if !e.matchOIDPrefix(trap.TrapOID, rule.OIDPattern) {
			return false, nil
		}
	}

	// 2. 源 IP 匹配（支持 CIDR）
	if rule.SourceIPPattern != "" {
		if !e.matchSourceIP(trap.SourceIP, rule) {
			return false, nil
		}
	}

	// 3. Community 匹配
	if rule.CommunityPattern != "" {
		if !e.matchCommunity(trap.Community, rule.CommunityPattern) {
			return false, nil
		}
	}

	// 所有设置的条件都匹配，返回 true
	return true, nil
}

// matchOIDPrefix OID 前缀匹配
func (e *TrapFilterEngine) matchOIDPrefix(trapOID, pattern string) bool {
	// 标准化 OID 格式
	trapOID = normalizeOID(trapOID)
	pattern = normalizeOID(pattern)

	// 前缀匹配
	return strings.HasPrefix(trapOID, pattern)
}

// matchSourceIP 源 IP 匹配（支持 CIDR）
func (e *TrapFilterEngine) matchSourceIP(sourceIP string, rule *models.SNMPTrapFilterRule) bool {
	// 检查是否是 CIDR 格式
	if strings.Contains(rule.SourceIPPattern, "/") {
		// 验证 CIDR 格式
		if !validateCIDR(rule.SourceIPPattern) {
			logger.Error("SNMP-Filter", "-", "CIDR 格式无效: %s", rule.SourceIPPattern)
			return false
		}

		// 使用预编译的 CIDR 网络
		if cidrNet, ok := e.cidrCache[rule.ID]; ok {
			ip := net.ParseIP(sourceIP)
			if ip == nil {
				return false
			}
			return cidrNet.Contains(ip)
		}

		// 动态解析 CIDR
		_, cidrNet, err := net.ParseCIDR(rule.SourceIPPattern)
		if err != nil {
			logger.Error("SNMP-Filter", "-", "CIDR 解析失败: %s, %v", rule.SourceIPPattern, err)
			return false
		}

		ip := net.ParseIP(sourceIP)
		if ip == nil {
			return false
		}
		return cidrNet.Contains(ip)
	}

	// 精确 IP 匹配
	return sourceIP == rule.SourceIPPattern
}

// validateCIDR 验证 CIDR 格式是否有效
func validateCIDR(cidr string) bool {
	_, _, err := net.ParseCIDR(cidr)
	return err == nil
}

// matchCommunity Community 匹配
func (e *TrapFilterEngine) matchCommunity(trapCommunity, pattern string) bool {
	// 精确匹配（忽略大小写）
	return strings.EqualFold(trapCommunity, pattern)
}

// compileRules 预编译所有规则
func (e *TrapFilterEngine) compileRules() {
	for _, rule := range e.rules {
		e.compileSingleRule(rule)
	}

	// 按优先级排序
	e.sortRules()
}

// compileSingleRule 预编译单个规则
func (e *TrapFilterEngine) compileSingleRule(rule *models.SNMPTrapFilterRule) {
	// 预编译 CIDR 网络
	if rule.SourceIPPattern != "" && strings.Contains(rule.SourceIPPattern, "/") {
		_, cidrNet, err := net.ParseCIDR(rule.SourceIPPattern)
		if err == nil {
			e.cidrCache[rule.ID] = cidrNet
		} else {
			logger.Error("SNMP-Filter", "-", "CIDR 预编译失败: RuleID=%d, Pattern=%s, %v",
				rule.ID, rule.SourceIPPattern, err)
		}
	}
}

// sortRules 按优先级排序规则
func (e *TrapFilterEngine) sortRules() {
	// 按优先级升序排序（值越小越先匹配）
	// 使用简单的插入排序
	for i := 1; i < len(e.rules); i++ {
		for j := i; j > 0 && e.rules[j-1].Priority > e.rules[j].Priority; j-- {
			e.rules[j-1], e.rules[j] = e.rules[j], e.rules[j-1]
		}
	}
}


// ============================================================================
// 过滤结果类型
// ============================================================================

// FilterResult 过滤结果
type FilterResult struct {
	Action         string // accept/drop/severity_override
	OverrideSeverity string // 覆盖的严重级别（仅 severity_override 时有效）
	RuleID         uint   // 匹配的规则 ID
	RuleName       string // 匹配的规则名称
}

// ApplyFilter 应用过滤规则并返回结果
func (e *TrapFilterEngine) ApplyFilter(trap *models.SNMPTrapRecord) FilterResult {
	matched, rule, _ := e.Match(trap)

	if !matched || rule == nil {
		// 默认接受
		return FilterResult{
			Action: "accept",
		}
	}

	return FilterResult{
		Action:         rule.Action,
		OverrideSeverity: rule.OverrideSeverity,
		RuleID:         rule.ID,
		RuleName:       rule.Name,
	}
}