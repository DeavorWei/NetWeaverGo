package taskexec

import (
	"github.com/NetWeaverGo/core/internal/logger"
)

// ErrorHandler 错误处理器
type ErrorHandler struct {
	runID string
}

// NewErrorHandler 创建错误处理器
func NewErrorHandler(runID string) *ErrorHandler {
	return &ErrorHandler{runID: runID}
}

// LogUpdateError 记录更新错误（非致命）
func (h *ErrorHandler) LogUpdateError(operation string, err error) {
	if err != nil {
		logger.Error("TaskExec", h.runID, "%s 失败: %v", operation, err)
	}
}

// MustUpdate 必须成功的更新（关键操作）
func (h *ErrorHandler) MustUpdate(operation string, err error) {
	if err != nil {
		logger.Error("TaskExec", h.runID, "关键操作 %s 失败: %v", operation, err)
	}
}
