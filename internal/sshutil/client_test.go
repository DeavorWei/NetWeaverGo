package sshutil

import (
	"strings"
	"testing"
)

func TestBuildAuthMethods(t *testing.T) {
	tests := []struct {
		name     string
		password string
	}{
		{name: "normal password", password: "MySecret123"},
		{name: "empty password", password: ""},
		{name: "password with special chars", password: "P@$$w0rd!#$%"},
		{name: "password with unicode", password: "密码测试123"},
		{name: "long password", password: strings.Repeat("a", 256)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			methods := buildAuthMethods(tt.password)

			// 验证返回两种认证方法
			if len(methods) != 2 {
				t.Errorf("buildAuthMethods() returned %d methods, expected 2", len(methods))
				return
			}

			// 验证第一个是 Password 认证
			if methods[0] == nil {
				t.Error("first auth method (Password) is nil")
			}

			// 验证第二个是 KeyboardInteractive 认证
			if methods[1] == nil {
				t.Error("second auth method (KeyboardInteractive) is nil")
			}
		})
	}
}

func TestCreateKeyboardInteractiveHandler(t *testing.T) {
	password := "MySecretPassword"

	// 创建处理器
	handler := createKeyboardInteractiveHandler(password)

	tests := []struct {
		name        string
		user        string
		instruction string
		questions   []string
		echos       []bool
		expected    []string
	}{
		{
			name:        "single password question",
			user:        "admin",
			instruction: "Please enter your password",
			questions:   []string{"Password: "},
			echos:       []bool{false},
			expected:    []string{password},
		},
		{
			name:        "multiple questions with password",
			user:        "admin",
			instruction: "Authentication required",
			questions:   []string{"Username: ", "Password: ", "Token: "},
			echos:       []bool{true, false, false},
			expected:    []string{"", password, ""}, // 只有 password 问题返回密码
		},
		{
			name:        "case insensitive password",
			user:        "admin",
			instruction: "",
			questions:   []string{"PASSWORD: ", "Enter your password:"},
			echos:       []bool{false, false},
			expected:    []string{password, password},
		},
		{
			name:        "no password question",
			user:        "admin",
			instruction: "Enter token",
			questions:   []string{"Token: "},
			echos:       []bool{false},
			expected:    []string{""},
		},
		{
			name:        "empty questions",
			user:        "admin",
			instruction: "",
			questions:   []string{},
			echos:       []bool{},
			expected:    []string{},
		},
		{
			name:        "password in middle of question",
			user:        "admin",
			instruction: "",
			questions:   []string{"Please enter your password now: "},
			echos:       []bool{false},
			expected:    []string{password},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			answers, err := handler(tt.user, tt.instruction, tt.questions, tt.echos)

			if err != nil {
				t.Errorf("handler returned error: %v", err)
				return
			}

			if len(answers) != len(tt.expected) {
				t.Errorf("handler returned %d answers, expected %d", len(answers), len(tt.expected))
				return
			}

			for i, ans := range answers {
				if ans != tt.expected[i] {
					t.Errorf("answer[%d] = %q, expected %q", i, ans, tt.expected[i])
				}
			}
		})
	}
}

func TestCreateKeyboardInteractiveHandler_EmptyPassword(t *testing.T) {
	// 测试空密码场景
	handler := createKeyboardInteractiveHandler("")

	questions := []string{"Password: "}
	answers, err := handler("admin", "", questions, []bool{false})

	if err != nil {
		t.Errorf("handler returned error: %v", err)
		return
	}

	if len(answers) != 1 {
		t.Errorf("expected 1 answer, got %d", len(answers))
		return
	}

	// 空密码也应该正确返回
	if answers[0] != "" {
		t.Errorf("expected empty password, got %q", answers[0])
	}
}

func TestCreateKeyboardInteractiveHandler_NilSafety(t *testing.T) {
	// 测试处理器不会 panic
	handler := createKeyboardInteractiveHandler("test")

	// 空问题列表
	answers, err := handler("admin", "", nil, nil)
	if err != nil {
		t.Errorf("handler returned error for nil questions: %v", err)
	}
	if answers == nil {
		t.Error("expected non-nil answers slice")
	}
	if len(answers) != 0 {
		t.Errorf("expected 0 answers for nil questions, got %d", len(answers))
	}
}
