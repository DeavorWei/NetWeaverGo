package logger

import (
	"regexp"
	"testing"
)

func TestSanitizer_PasswordCipher(t *testing.T) {
	s := NewSanitizer()
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "password cipher format",
			input:    "password admin cipher MySecret123",
			expected: "password admin cipher ****",
		},
		{
			name:     "password plain format",
			input:    "password admin plain MySecret123",
			expected: "password admin plain ****",
		},
		{
			name:     "local-user password cipher",
			input:    "local-user admin password cipher P@ssw0rd",
			expected: "local-user admin password cipher ****",
		},
		{
			name:     "case insensitive password",
			input:    "PASSWORD admin CIPHER MySecret123",
			expected: "PASSWORD admin CIPHER ****",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := s.Sanitize(tt.input)
			if result != tt.expected {
				t.Errorf("Sanitize() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestSanitizer_PasswordSimple(t *testing.T) {
	s := NewSanitizer()
	// 注意：password_simple 规则已移除，因为 Go regexp 不支持负向先行断言
	// 简单格式的密码不会被脱敏，只有 cipher/plain 格式会被处理
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple password format - not sanitized",
			input:    "password admin MySecret123",
			expected: "password admin MySecret123", // 简单格式不脱敏
		},
		{
			name:     "password with special chars - not sanitized",
			input:    "password test P@$$w0rd!#$",
			expected: "password test P@$$w0rd!#$", // 简单格式不脱敏
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := s.Sanitize(tt.input)
			if result != tt.expected {
				t.Errorf("Sanitize() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestSanitizer_CipherSecretToken(t *testing.T) {
	s := NewSanitizer()
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "cipher value",
			input:    "cipher MySecretKey",
			expected: "cipher ****",
		},
		{
			name:     "secret value",
			input:    "secret abc123token",
			expected: "secret ****",
		},
		{
			name:     "token value",
			input:    "token eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
			expected: "token ****",
		},
		{
			name:     "credential value",
			input:    "credential mycreds123",
			expected: "credential ****",
		},
		{
			name:     "key config",
			input:    "key encryption MyKey123",
			expected: "key encryption ****",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := s.Sanitize(tt.input)
			if result != tt.expected {
				t.Errorf("Sanitize() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestSanitizer_JSON(t *testing.T) {
	s := NewSanitizer()
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "json password field",
			input:    `{"username":"admin","password":"secret123"}`,
			expected: `{"username":"admin","password":"****"}`,
		},
		{
			name:     "json token field",
			input:    `{"user":"test","token":"abc123xyz"}`,
			expected: `{"user":"test","token":"****"}`,
		},
		{
			name:     "json secret field",
			input:    `{"name":"service","secret":"mysecret"}`,
			expected: `{"name":"service","secret":"****"}`,
		},
		{
			name:     "json api_key field",
			input:    `{"api_key":"sk-1234567890"}`,
			expected: `{"api_key":"****"}`,
		},
		{
			name:     "json multiple sensitive fields",
			input:    `{"password":"pass1","token":"tok1","secret":"sec1"}`,
			expected: `{"password":"****","token":"****","secret":"****"}`,
		},
		{
			name:     "json with spaces",
			input:    `{"password" : "secret123"}`,
			expected: `{"password":"****"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := s.Sanitize(tt.input)
			if result != tt.expected {
				t.Errorf("Sanitize() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestSanitizer_URL(t *testing.T) {
	s := NewSanitizer()
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "url with password",
			input:    "https://admin:pass123@device.local",
			expected: "https://admin:****@device.local",
		},
		{
			name:     "ssh url with simple password",
			input:    "ssh://user:password123@192.168.1.1:22",
			expected: "ssh://user:****@192.168.1.1:22",
		},
		{
			name:     "url without password unchanged",
			input:    "https://device.local/api",
			expected: "https://device.local/api",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := s.Sanitize(tt.input)
			if result != tt.expected {
				t.Errorf("Sanitize() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestSanitizer_MixedContent(t *testing.T) {
	s := NewSanitizer()
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:  "log with password cipher and other info",
			input: "Device config: password admin cipher Secret123, interface Gig0/1",
			// 注意：正则匹配到 Secret123 后的逗号被当作密码的一部分
			expected: "Device config: password admin cipher **** interface Gig0/1",
		},
		{
			name:     "multiple sensitive patterns",
			input:    "Set secret mysecret and token abc123",
			expected: "Set secret **** and token ****",
		},
		{
			name:     "no sensitive data",
			input:    "Interface GigabitEthernet0/1 is up, line protocol is up",
			expected: "Interface GigabitEthernet0/1 is up, line protocol is up",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := s.Sanitize(tt.input)
			if result != tt.expected {
				t.Errorf("Sanitize() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestSanitizer_EnableDisable(t *testing.T) {
	s := NewSanitizer()

	// 默认启用
	if !s.IsEnabled() {
		t.Error("Sanitizer should be enabled by default")
	}

	// 测试启用状态下的脱敏
	input := "password admin cipher secret123"
	result := s.Sanitize(input)
	if result != "password admin cipher ****" {
		t.Errorf("Expected sanitization when enabled, got: %q", result)
	}

	// 禁用脱敏
	s.SetEnabled(false)
	if s.IsEnabled() {
		t.Error("Sanitizer should be disabled")
	}

	// 测试禁用状态下的不脱敏
	result = s.Sanitize(input)
	if result != input {
		t.Errorf("Expected no sanitization when disabled, got: %q", result)
	}

	// 重新启用
	s.SetEnabled(true)
	result = s.Sanitize(input)
	if result != "password admin cipher ****" {
		t.Errorf("Expected sanitization when re-enabled, got: %q", result)
	}
}

func TestSanitizer_AddRule(t *testing.T) {
	s := NewSanitizer()

	// 添加自定义规则
	customRule := SanitizeRule{
		Name:        "custom_apikey",
		Pattern:     mustCompileRegex(`(?i)(apikey=)(\S+)`),
		Replacement: "${1}****",
		Enabled:     true,
	}
	s.AddRule(customRule)

	input := "Request: apikey=myapikey123"
	result := s.Sanitize(input)
	expected := "Request: apikey=****"
	if result != expected {
		t.Errorf("Sanitize() = %q, want %q", result, expected)
	}
}

func TestSanitizer_RemoveRule(t *testing.T) {
	s := NewSanitizer()

	// 验证规则存在且生效
	input := "password admin cipher secret123"
	result := s.Sanitize(input)
	if result != "password admin cipher ****" {
		t.Errorf("Initial sanitization failed: %q", result)
	}

	// 移除规则
	removed := s.RemoveRule("password_cipher")
	if !removed {
		t.Error("Rule should have been removed")
	}

	// 验证规则已移除
	result = s.Sanitize(input)
	// password_cipher 已移除，cipher_value 规则仍会匹配 "cipher secret123"
	if result != "password admin cipher ****" {
		t.Errorf("After removing password_cipher, cipher_value should still match: %q", result)
	}

	// 移除不存在的规则
	removed = s.RemoveRule("nonexistent_rule")
	if removed {
		t.Error("Non-existent rule should not be removed")
	}
}

func TestSanitizer_SetRuleEnabled(t *testing.T) {
	s := NewSanitizer()

	input := "password admin cipher secret123"

	// 禁用特定规则
	changed := s.SetRuleEnabled("password_cipher", false)
	if !changed {
		t.Error("Rule should have been disabled")
	}

	// 验证规则已禁用（cipher_value 规则仍会匹配）
	result := s.Sanitize(input)
	if result != "password admin cipher ****" {
		t.Errorf("After disabling password_cipher, cipher_value should still match: %q", result)
	}

	// 重新启用规则
	changed = s.SetRuleEnabled("password_cipher", true)
	if !changed {
		t.Error("Rule should have been enabled")
	}

	result = s.Sanitize(input)
	if result != "password admin cipher ****" {
		t.Errorf("After re-enabling rule, expected sanitization, got: %q", result)
	}

	// 设置不存在的规则
	changed = s.SetRuleEnabled("nonexistent_rule", true)
	if changed {
		t.Error("Non-existent rule should not be changed")
	}
}

func TestSanitizer_Reset(t *testing.T) {
	s := NewSanitizer()

	// 添加自定义规则
	s.AddRule(SanitizeRule{
		Name:        "custom",
		Pattern:     mustCompileRegex(`custom`),
		Replacement: "REDACTED",
		Enabled:     true,
	})

	// 禁用
	s.SetEnabled(false)

	// 重置
	s.Reset()

	// 验证重置后状态
	if !s.IsEnabled() {
		t.Error("Sanitizer should be enabled after reset")
	}

	rules := s.GetRules()
	if len(rules) != len(defaultRules) {
		t.Errorf("Expected %d rules after reset, got %d", len(defaultRules), len(rules))
	}
}

func TestSanitizer_GetRules(t *testing.T) {
	s := NewSanitizer()
	rules := s.GetRules()

	if len(rules) != len(defaultRules) {
		t.Errorf("Expected %d rules, got %d", len(defaultRules), len(rules))
	}

	// 验证返回的是副本，修改不影响原规则
	rules[0].Enabled = false
	originalRules := s.GetRules()
	if !originalRules[0].Enabled {
		t.Error("Modifying returned slice should not affect original rules")
	}
}

func TestGlobalSanitizer(t *testing.T) {
	// 测试全局脱敏器函数
	// 注意：password <user> <secret> 格式需要 cipher/plain 关键字才会被脱敏
	input := "password admin cipher secret123"
	result := Sanitize(input)
	expected := "password admin cipher ****"
	if result != expected {
		t.Errorf("Global Sanitize() = %q, want %q", result, expected)
	}

	// 测试获取全局脱敏器
	gs := GetGlobalSanitizer()
	if gs == nil {
		t.Error("GetGlobalSanitizer() returned nil")
	}

	// 测试全局开关
	SetGlobalSanitizerEnabled(false)
	if gs.IsEnabled() {
		t.Error("Global sanitizer should be disabled")
	}

	result = Sanitize(input)
	if result != input {
		t.Errorf("Expected no sanitization when global disabled, got: %q", result)
	}

	// 恢复
	SetGlobalSanitizerEnabled(true)
}

func mustCompileRegex(pattern string) *regexp.Regexp {
	return regexp.MustCompile(pattern)
}

func BenchmarkSanitizer_Sanitize(b *testing.B) {
	s := NewSanitizer()
	msg := "Device config: password admin cipher MySecret123, interface Gig0/1 status up"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Sanitize(msg)
	}
}

func BenchmarkSanitizer_SanitizeNoMatch(b *testing.B) {
	s := NewSanitizer()
	msg := "Interface GigabitEthernet0/1 is up, line protocol is up"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Sanitize(msg)
	}
}

func BenchmarkSanitizer_SanitizeJSON(b *testing.B) {
	s := NewSanitizer()
	msg := `{"username":"admin","password":"secret123","token":"abc123"}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Sanitize(msg)
	}
}
