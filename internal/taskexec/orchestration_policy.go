package taskexec

import (
	"context"
	"fmt"
	"time"

	"github.com/NetWeaverGo/core/internal/logger"
)

func evaluateStageDependencyPolicy(runKind string, stage StagePlan, results map[string]error) (bool, string) {
	if runKind != string(RunKindTopology) {
		return false, ""
	}

	switch StageKind(stage.Kind) {
	case StageKindParse:
		if err, ok := results[string(StageKindDeviceCollect)]; ok && err != nil {
			reason := fmt.Sprintf("依赖阶段 %s 执行失败", StageKindDeviceCollect)
			logger.Verbose("TaskExec", "-", "阶段依赖命中跳过策略: stage=%s, dependency=%s, reason=%s",
				stage.Kind, StageKindDeviceCollect, reason)
			return true, reason
		}
	case StageKindTopologyBuild:
		if err, ok := results[string(StageKindParse)]; ok && err != nil {
			reason := fmt.Sprintf("依赖阶段 %s 执行失败", StageKindParse)
			logger.Verbose("TaskExec", "-", "阶段依赖命中跳过策略: stage=%s, dependency=%s, reason=%s",
				stage.Kind, StageKindParse, reason)
			return true, reason
		}
	}

	return false, ""
}

func evaluateStageFailurePolicy(runKind string, stage StagePlan, stageErr error) (bool, string) {
	if stageErr == nil || runKind != string(RunKindTopology) {
		return false, ""
	}

	switch StageKind(stage.Kind) {
	case StageKindDeviceCollect, StageKindParse:
		reason := fmt.Sprintf("关键阶段 %s 失败", stage.Name)
		logger.Verbose("TaskExec", "-", "阶段失败命中中止策略: stage=%s, reason=%s", stage.Kind, reason)
		return true, reason
	default:
		return false, ""
	}
}

func applyCompensationCancellation(repo Repository, runID string, finishedAt time.Time) error {
	if repo == nil {
		return fmt.Errorf("repository is nil")
	}

	cancelledStatus := string(RunStatusCancelled)
	if err := repo.UpdateRun(context.Background(), runID, &RunPatch{
		Status:     &cancelledStatus,
		FinishedAt: &finishedAt,
	}); err != nil {
		return err
	}

	stages, err := repo.GetStagesByRun(context.Background(), runID)
	if err != nil {
		return nil
	}

	for _, stage := range stages {
		stageStatus := string(StageStatusCancelled)
		if err := repo.UpdateStage(context.Background(), stage.ID, &StagePatch{
			Status:     &stageStatus,
			FinishedAt: &finishedAt,
		}); err != nil {
			logger.Warn("TaskExec", runID, "补偿取消更新 Stage 失败: stage=%s, err=%v", stage.ID, err)
		}

		units, unitErr := repo.GetUnitsByStage(context.Background(), stage.ID)
		if unitErr != nil {
			logger.Warn("TaskExec", runID, "补偿取消读取 Stage Unit 失败: stage=%s, err=%v", stage.ID, unitErr)
			continue
		}
		for _, unit := range units {
			if UnitStatus(unit.Status).IsTerminal() {
				continue
			}
			unitStatus := string(UnitStatusCancelled)
			reason := "run cancelled by compensation"
			if err := repo.UpdateUnit(context.Background(), unit.ID, &UnitPatch{
				Status:       &unitStatus,
				ErrorMessage: &reason,
				FinishedAt:   &finishedAt,
			}); err != nil {
				logger.Warn("TaskExec", runID, "补偿取消更新 Unit 失败: unit=%s, err=%v", unit.ID, err)
			}
		}
	}

	logger.Verbose("TaskExec", runID, "已完成离线运行的补偿取消投影")
	return nil
}
