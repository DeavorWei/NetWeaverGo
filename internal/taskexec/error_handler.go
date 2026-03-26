package taskexec

import (
	"context"
	"fmt"
	"time"

	"github.com/NetWeaverGo/core/internal/logger"
	"gorm.io/gorm"
)

// ErrorHandler 错误处理器
//
// 用于统一运行时关键状态更新、非关键数据库写入、取消态落库等行为，
// 避免执行链路中大量 `_ = err` 静默吞错。
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

// UpdateUnitRequired 更新 Unit，失败则返回错误
func (h *ErrorHandler) UpdateUnitRequired(ctx RuntimeContext, unitID string, patch *UnitPatch, operation string) error {
	if err := ctx.UpdateUnit(unitID, patch); err != nil {
		h.MustUpdate(operation, err)
		return fmt.Errorf("%s: %w", operation, err)
	}
	return nil
}

// UpdateStageRequired 更新 Stage，失败则返回错误
func (h *ErrorHandler) UpdateStageRequired(ctx RuntimeContext, stageID string, patch *StagePatch, operation string) error {
	if err := ctx.UpdateStage(stageID, patch); err != nil {
		h.MustUpdate(operation, err)
		return fmt.Errorf("%s: %w", operation, err)
	}
	return nil
}

// UpdateRunRequired 更新 Run，失败则返回错误
func (h *ErrorHandler) UpdateRunRequired(ctx RuntimeContext, patch *RunPatch, operation string) error {
	if err := ctx.UpdateRun(patch); err != nil {
		h.MustUpdate(operation, err)
		return fmt.Errorf("%s: %w", operation, err)
	}
	return nil
}

// UpdateStageBestEffort 更新 Stage，失败仅记录日志
func (h *ErrorHandler) UpdateStageBestEffort(ctx RuntimeContext, stageID string, patch *StagePatch, operation string) {
	if err := ctx.UpdateStage(stageID, patch); err != nil {
		h.LogUpdateError(operation, err)
	}
}

// UpdateRunBestEffort 更新 Run，失败仅记录日志
func (h *ErrorHandler) UpdateRunBestEffort(ctx RuntimeContext, patch *RunPatch, operation string) {
	if err := ctx.UpdateRun(patch); err != nil {
		h.LogUpdateError(operation, err)
	}
}

// DBBestEffort 执行非关键数据库写入，失败仅记录日志
func (h *ErrorHandler) DBBestEffort(operation string, exec func() error) {
	if err := exec(); err != nil {
		h.LogUpdateError(operation, err)
	}
}

// ArtifactBestEffort 创建产物索引，失败仅记录日志
func (h *ErrorHandler) ArtifactBestEffort(repo Repository, ctx context.Context, artifact *TaskArtifact) {
	if repo == nil || artifact == nil {
		return
	}
	if err := repo.CreateArtifact(ctx, artifact); err != nil {
		h.LogUpdateError(fmt.Sprintf("创建产物索引[%s:%s]", artifact.ArtifactType, artifact.ArtifactKey), err)
	}
}

// LogDBErrorWithContext 记录带上下文的数据库写入错误
func (h *ErrorHandler) LogDBErrorWithContext(operation string, err error, fields map[string]interface{}) {
	if err == nil {
		return
	}
	if len(fields) == 0 {
		h.LogUpdateError(operation, err)
		return
	}
	logger.Error("TaskExec", h.runID, "%s 失败: %v, context=%v", operation, err, fields)
}

// IsContextCancelled 判断错误或上下文是否表示取消
func IsContextCancelled(ctx RuntimeContext, err error) bool {
	if ctx != nil && ctx.IsCancelled() {
		return true
	}
	if err == nil {
		return false
	}
	return err == context.Canceled || err == context.DeadlineExceeded
}

// MarkUnitCancelled 将 Unit 标记为取消态，失败仅记录日志
func (h *ErrorHandler) MarkUnitCancelled(ctx RuntimeContext, unitID string, reason string, doneSteps *int) {
	cancelled := string(UnitStatusCancelled)
	now := timeNow()
	patch := &UnitPatch{
		Status:       &cancelled,
		ErrorMessage: &reason,
		FinishedAt:   &now,
	}
	if doneSteps != nil {
		patch.DoneSteps = doneSteps
	}
	if err := ctx.UpdateUnit(unitID, patch); err != nil {
		h.LogUpdateError("标记 Unit 已取消", err)
	}
}

// IsNotFoundError 判断是否为记录不存在
func IsNotFoundError(err error) bool {
	return err == gorm.ErrRecordNotFound
}

// timeNow 为便于后续测试替换预留
var timeNow = func() time.Time {
	return time.Now()
}
