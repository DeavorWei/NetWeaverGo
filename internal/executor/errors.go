package executor

import (
	"errors"
	"fmt"
	"strings"
)

// ErrorType 错误类型枚举
type ErrorType int

const (
	// ErrorTypeNone 无错误
	ErrorTypeNone ErrorType = iota

	// ErrorTypeWarning 警告级别（可继续执行）
	ErrorTypeWarning

	// ErrorTypeCritical 严重错误（需要中断）
	ErrorTypeCritical

	// ErrorTypeFatal 致命错误（系统级）
	ErrorTypeFatal
)

func (et ErrorType) String() string {
	switch et {
	case ErrorTypeWarning:
		return "WARNING"
	case ErrorTypeCritical:
		return "CRITICAL"
	case ErrorTypeFatal:
		return "FATAL"
	default:
		return "NONE"
	}
}

// MarshalJSON 实现 JSON 序列化
func (et ErrorType) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("\"%s\"", et.String())), nil
}

// ExecutionError 统一的执行错误类型
type ExecutionError struct {
	Type    ErrorType              `json:"type"`    // 错误类型
	IP      string                 `json:"ip"`      // 设备IP
	Command string                 `json:"command"` // 执行的命令
	Stage   string                 `json:"stage"`   // 错误发生的阶段: connect, execute, read, parse, etc.
	Message string                 `json:"message"` // 错误消息
	Err     error                  `json:"-"`       // 原始错误
	Context map[string]interface{} `json:"context"` // 额外的上下文信息
}

func (e *ExecutionError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] Device=%s Stage=%s Command=%s: %v",
			e.Type, e.IP, e.Stage, e.Command, e.Err)
	}
	return fmt.Sprintf("[%s] Device=%s Stage=%s Command=%s: %s",
		e.Type, e.IP, e.Stage, e.Command, e.Message)
}

// Unwrap 实现错误链
func (e *ExecutionError) Unwrap() error {
	return e.Err
}

// IsWarning 是否为警告级别
func (e *ExecutionError) IsWarning() bool {
	return e.Type == ErrorTypeWarning
}

// IsCritical 是否为严重级别
func (e *ExecutionError) IsCritical() bool {
	return e.Type == ErrorTypeCritical || e.Type == ErrorTypeFatal
}

// ShouldContinue 是否应该继续执行
func (e *ExecutionError) ShouldContinue() bool {
	return e.Type == ErrorTypeWarning || e.Type == ErrorTypeNone
}

// ToMap 转换为Map（用于前端展示）
func (e *ExecutionError) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"type":    e.Type.String(),
		"ip":      e.IP,
		"command": e.Command,
		"stage":   e.Stage,
		"message": e.Message,
		"context": e.Context,
	}
}

// ==================== 错误构建器 ====================

// ErrorBuilder 错误构建器
type ErrorBuilder struct {
	err *ExecutionError
}

// NewError 创建新的错误构建器
func NewError(ip string) *ErrorBuilder {
	return &ErrorBuilder{
		err: &ExecutionError{
			IP:      ip,
			Type:    ErrorTypeWarning,
			Context: make(map[string]interface{}),
		},
	}
}

// WithCommand 设置命令
func (b *ErrorBuilder) WithCommand(cmd string) *ErrorBuilder {
	b.err.Command = cmd
	return b
}

// WithStage 设置阶段
func (b *ErrorBuilder) WithStage(stage string) *ErrorBuilder {
	b.err.Stage = stage
	return b
}

// WithType 设置错误类型
func (b *ErrorBuilder) WithType(t ErrorType) *ErrorBuilder {
	b.err.Type = t
	return b
}

// WithError 设置原始错误
func (b *ErrorBuilder) WithError(err error) *ErrorBuilder {
	b.err.Err = err
	if err != nil {
		b.err.Message = err.Error()
	}
	return b
}

// WithMessage 设置错误消息
func (b *ErrorBuilder) WithMessage(msg string) *ErrorBuilder {
	b.err.Message = msg
	return b
}

// WithContext 添加上下文
func (b *ErrorBuilder) WithContext(key string, value interface{}) *ErrorBuilder {
	b.err.Context[key] = value
	return b
}

// Build 构建错误
func (b *ErrorBuilder) Build() *ExecutionError {
	return b.err
}

// ==================== 错误分类函数 ====================

// ClassifyError 根据错误特征自动分类
func ClassifyError(err error) ErrorType {
	if err == nil {
		return ErrorTypeNone
	}

	errStr := strings.ToLower(err.Error())

	// 致命错误
	if strings.Contains(errStr, "out of memory") ||
		strings.Contains(errStr, "fatal") ||
		strings.Contains(errStr, "panic") {
		return ErrorTypeFatal
	}

	// 连接错误 - 严重
	if strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "no route to host") ||
		strings.Contains(errStr, "network is unreachable") ||
		strings.Contains(errStr, "i/o timeout") {
		return ErrorTypeCritical
	}

	// 认证错误 - 严重
	if strings.Contains(errStr, "authentication failed") ||
		strings.Contains(errStr, "permission denied") ||
		strings.Contains(errStr, "unable to authenticate") {
		return ErrorTypeCritical
	}

	// 超时错误 - 警告（可重试）
	if strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "deadline exceeded") {
		return ErrorTypeWarning
	}

	// EOF 或连接重置 - 警告（可能临时问题）
	if strings.Contains(errStr, "eof") ||
		strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "broken pipe") {
		return ErrorTypeWarning
	}

	// 命令执行错误 - 警告（命令本身问题）
	if strings.Contains(errStr, "command not found") ||
		strings.Contains(errStr, "syntax error") {
		return ErrorTypeWarning
	}

	// 默认视为警告
	return ErrorTypeWarning
}

// IsExecutionError 检查是否为ExecutionError
func IsExecutionError(err error) (*ExecutionError, bool) {
	var execErr *ExecutionError
	if errors.As(err, &execErr) {
		return execErr, true
	}
	return nil, false
}

// ==================== 预定义错误阶段 ====================

const (
	StageConnect      = "connect"      // 连接阶段
	StageAuthenticate = "authenticate" // 认证阶段
	StageExecute      = "execute"      // 执行阶段
	StageRead         = "read"         // 读取阶段
	StageParse        = "parse"        // 解析阶段
	StageClose        = "close"        // 关闭阶段
	StageCleanup      = "cleanup"      // 清理阶段
)

// ==================== 便捷函数 ====================

// NewWarningError 创建警告错误
func NewWarningError(ip, stage string, err error) *ExecutionError {
	return NewError(ip).
		WithStage(stage).
		WithType(ErrorTypeWarning).
		WithError(err).
		Build()
}

// NewCriticalError 创建严重错误
func NewCriticalError(ip, stage string, err error) *ExecutionError {
	return NewError(ip).
		WithStage(stage).
		WithType(ErrorTypeCritical).
		WithError(err).
		Build()
}

// NewFatalError 创建致命错误
func NewFatalError(ip, stage string, err error) *ExecutionError {
	return NewError(ip).
		WithStage(stage).
		WithType(ErrorTypeFatal).
		WithError(err).
		Build()
}
