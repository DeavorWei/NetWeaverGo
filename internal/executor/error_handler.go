package executor

import (
	"context"

	"github.com/NetWeaverGo/core/internal/logger"
)

// ErrorHandler 统一错误处理器
type ErrorHandler struct {
	// 可以扩展添加事件总线等
}

// NewErrorHandler 创建错误处理器
func NewErrorHandler() *ErrorHandler {
	return &ErrorHandler{}
}

// Handle 统一处理错误
// 返回：是否继续执行（true=继续，false=中断）
func (h *ErrorHandler) Handle(ctx context.Context, err *ExecutionError) bool {
	if err == nil {
		return true
	}

	// 1. 记录日志
	h.logError(err)

	// 2. 根据策略决定处理方式
	if err.ShouldContinue() {
		return true // 警告级别，继续执行
	}
	return false // 严重级别，中断
}

// logError 记录错误日志
func (h *ErrorHandler) logError(err *ExecutionError) {
	switch err.Type {
	case ErrorTypeWarning:
		logger.Warn("Executor", err.IP, "[%s] Device=%s Stage=%s Command=%s: %s",
			err.Type, err.IP, err.Stage, err.Command, err.Message)
	case ErrorTypeCritical:
		logger.Error("Executor", err.IP, "[%s] Device=%s Stage=%s Command=%s: %s",
			err.Type, err.IP, err.Stage, err.Command, err.Message)
	case ErrorTypeFatal:
		logger.Error("Executor", err.IP, "[%s] Device=%s Stage=%s Command=%s: %s",
			err.Type, err.IP, err.Stage, err.Command, err.Message)
	}
}

// HandleError 便捷函数：创建并处理错误
func HandleError(ctx context.Context, ip, stage string, originalErr error) bool {
	if originalErr == nil {
		return true
	}

	errType := ClassifyError(originalErr)
	execErr := NewError(ip).
		WithStage(stage).
		WithType(errType).
		WithError(originalErr).
		Build()

	handler := NewErrorHandler()
	return handler.Handle(ctx, execErr)
}

// HandleErrorWithCommand 便捷函数：带命令信息的错误处理
func HandleErrorWithCommand(ctx context.Context, ip, cmd, stage string, originalErr error) bool {
	if originalErr == nil {
		return true
	}

	errType := ClassifyError(originalErr)
	execErr := NewError(ip).
		WithCommand(cmd).
		WithStage(stage).
		WithType(errType).
		WithError(originalErr).
		Build()

	handler := NewErrorHandler()
	return handler.Handle(ctx, execErr)
}
