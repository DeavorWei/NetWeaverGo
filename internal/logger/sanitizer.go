package logger

import (
	"regexp"
	"sync"
)

// Sanitizer 敏感信息脱敏器
type Sanitizer struct {
	mu      sync.RWMutex
	rules   []SanitizeRule
	enabled bool
}

// SanitizeRule 单条脱敏规则
type SanitizeRule struct {
	Name        string         // 规则名称
	Pattern     *regexp.Regexp // 匹配正则
	Replacement string         // 替换模板
	Enabled     bool           // 是否启用
}

// 敏感字段关键词清单（用于文档和扩展参考）
var defaultSensitiveKeywords = []string{
	"password", "passwd", "pwd",
	"secret", "token", "apikey", "api_key",
	"credential", "private_key", "privatekey",
	"auth", "authorization",
	"access_key", "secret_key",
	"session_id", "sessionid",
}

// 默认脱敏规则
// 注意：规则按顺序执行，更具体的规则应放在前面
var defaultRules = []SanitizeRule{
	// 配置命令中的密码（网络设备常见格式）- 更具体的规则优先
	// 注意：password_cipher 和 password_plain 必须在 cipher_value 之前
	{Name: "password_cipher", Pattern: regexp.MustCompile(`(?i)(password\s+\S+\s+cipher\s+)(\S+)`), Replacement: "${1}****", Enabled: true},
	{Name: "password_plain", Pattern: regexp.MustCompile(`(?i)(password\s+\S+\s+plain\s+)(\S+)`), Replacement: "${1}****", Enabled: true},

	// 密钥和凭证
	{Name: "cipher_value", Pattern: regexp.MustCompile(`(?i)(cipher\s+)(\S+)`), Replacement: "${1}****", Enabled: true},
	{Name: "key_config", Pattern: regexp.MustCompile(`(?i)(key\s+\S+\s+)(\S+)`), Replacement: "${1}****", Enabled: true},
	{Name: "secret_value", Pattern: regexp.MustCompile(`(?i)(secret\s+)(\S+)`), Replacement: "${1}****", Enabled: true},
	{Name: "credential_value", Pattern: regexp.MustCompile(`(?i)(credential\s+)(\S+)`), Replacement: "${1}****", Enabled: true},
	{Name: "token_value", Pattern: regexp.MustCompile(`(?i)(token\s+)(\S+)`), Replacement: "${1}****", Enabled: true},

	// JSON 字段脱敏
	{Name: "json_password", Pattern: regexp.MustCompile(`"password"\s*:\s*"[^"]*"`), Replacement: `"password":"****"`, Enabled: true},
	{Name: "json_token", Pattern: regexp.MustCompile(`"token"\s*:\s*"[^"]*"`), Replacement: `"token":"****"`, Enabled: true},
	{Name: "json_secret", Pattern: regexp.MustCompile(`"secret"\s*:\s*"[^"]*"`), Replacement: `"secret":"****"`, Enabled: true},
	{Name: "json_api_key", Pattern: regexp.MustCompile(`"api_key"\s*:\s*"[^"]*"`), Replacement: `"api_key":"****"`, Enabled: true},
	{Name: "json_apikey", Pattern: regexp.MustCompile(`"apikey"\s*:\s*"[^"]*"`), Replacement: `"apikey":"****"`, Enabled: true},
	{Name: "json_credential", Pattern: regexp.MustCompile(`"credential"\s*:\s*"[^"]*"`), Replacement: `"credential":"****"`, Enabled: true},
	{Name: "json_private_key", Pattern: regexp.MustCompile(`"private_key"\s*:\s*"[^"]*"`), Replacement: `"private_key":"****"`, Enabled: true},
	{Name: "json_access_key", Pattern: regexp.MustCompile(`"access_key"\s*:\s*"[^"]*"`), Replacement: `"access_key":"****"`, Enabled: true},
	{Name: "json_secret_key", Pattern: regexp.MustCompile(`"secret_key"\s*:\s*"[^"]*"`), Replacement: `"secret_key":"****"`, Enabled: true},

	// URL 中的密码（如 ssh://user:pass@host）- 匹配到第一个 @ 符号
	{Name: "url_password", Pattern: regexp.MustCompile(`(?i)(://[^:@]+:)([^@]+)(@)`), Replacement: "${1}****${3}", Enabled: true},
}

// 全局脱敏器实例
var globalSanitizer = NewSanitizer()

// NewSanitizer 创建脱敏器实例
func NewSanitizer() *Sanitizer {
	rules := make([]SanitizeRule, len(defaultRules))
	copy(rules, defaultRules)
	return &Sanitizer{
		rules:   rules,
		enabled: true,
	}
}

// GetGlobalSanitizer 获取全局脱敏器实例
func GetGlobalSanitizer() *Sanitizer {
	return globalSanitizer
}

// Sanitize 对消息进行脱敏处理
func (s *Sanitizer) Sanitize(msg string) string {
	if !s.enabled || msg == "" {
		return msg
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	result := msg
	for _, rule := range s.rules {
		if rule.Enabled {
			result = rule.Pattern.ReplaceAllString(result, rule.Replacement)
		}
	}
	return result
}

// AddRule 添加自定义脱敏规则
func (s *Sanitizer) AddRule(rule SanitizeRule) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rules = append(s.rules, rule)
}

// RemoveRule 移除指定名称的规则
func (s *Sanitizer) RemoveRule(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, rule := range s.rules {
		if rule.Name == name {
			s.rules = append(s.rules[:i], s.rules[i+1:]...)
			return true
		}
	}
	return false
}

// SetRuleEnabled 设置指定规则的启用状态
func (s *Sanitizer) SetRuleEnabled(name string, enabled bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.rules {
		if s.rules[i].Name == name {
			s.rules[i].Enabled = enabled
			return true
		}
	}
	return false
}

// SetEnabled 设置脱敏器总开关
func (s *Sanitizer) SetEnabled(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.enabled = enabled
}

// IsEnabled 返回脱敏器是否启用
func (s *Sanitizer) IsEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.enabled
}

// GetRules 返回所有规则（只读副本）
func (s *Sanitizer) GetRules() []SanitizeRule {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rules := make([]SanitizeRule, len(s.rules))
	copy(rules, s.rules)
	return rules
}

// Reset 重置为默认规则
func (s *Sanitizer) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	rules := make([]SanitizeRule, len(defaultRules))
	copy(rules, defaultRules)
	s.rules = rules
	s.enabled = true
}

// SetGlobalSanitizerEnabled 设置全局脱敏器开关
func SetGlobalSanitizerEnabled(enabled bool) {
	globalSanitizer.SetEnabled(enabled)
}

// Sanitize 使用全局脱敏器进行脱敏（便捷函数）
func Sanitize(msg string) string {
	return globalSanitizer.Sanitize(msg)
}
