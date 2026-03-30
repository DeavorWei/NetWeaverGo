package taskexec

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

// Repository 运行时仓库接口
type Repository interface {
	// Run operations
	CreateRun(ctx context.Context, run *TaskRun) error
	GetRun(ctx context.Context, runID string) (*TaskRun, error)
	UpdateRun(ctx context.Context, runID string, patch *RunPatch) error
	ListRuns(ctx context.Context, limit int) ([]TaskRun, error)
	ListRunningRuns(ctx context.Context) ([]TaskRun, error)

	// Stage operations
	CreateStage(ctx context.Context, stage *TaskRunStage) error
	GetStage(ctx context.Context, stageID string) (*TaskRunStage, error)
	UpdateStage(ctx context.Context, stageID string, patch *StagePatch) error
	GetStagesByRun(ctx context.Context, runID string) ([]TaskRunStage, error)

	// Unit operations
	CreateUnit(ctx context.Context, unit *TaskRunUnit) error
	GetUnit(ctx context.Context, unitID string) (*TaskRunUnit, error)
	UpdateUnit(ctx context.Context, unitID string, patch *UnitPatch) error
	GetUnitsByStage(ctx context.Context, stageID string) ([]TaskRunUnit, error)
	GetUnitsByRun(ctx context.Context, runID string) ([]TaskRunUnit, error)

	// Event operations
	CreateEvent(ctx context.Context, event *TaskRunEvent) error
	GetEventsByRun(ctx context.Context, runID string, limit int) ([]TaskRunEvent, error)

	// Artifact operations
	CreateArtifact(ctx context.Context, artifact *TaskArtifact) error
	GetArtifactsByRun(ctx context.Context, runID string) ([]TaskArtifact, error)
	GetArtifactsByStage(ctx context.Context, stageID string) ([]TaskArtifact, error)
	GetArtifactsByUnit(ctx context.Context, unitID string) ([]TaskArtifact, error)
}

// GormRepository GORM实现的运行时仓库
type GormRepository struct {
	db *gorm.DB
}

// NewGormRepository 创建GORM仓库
func NewGormRepository(db *gorm.DB) *GormRepository {
	return &GormRepository{db: db}
}

// ==================== Run Operations ====================

// CreateRun 创建Run
func (r *GormRepository) CreateRun(ctx context.Context, run *TaskRun) error {
	return r.db.WithContext(ctx).Create(run).Error
}

// GetRun 获取Run
func (r *GormRepository) GetRun(ctx context.Context, runID string) (*TaskRun, error) {
	var run TaskRun
	if err := r.db.WithContext(ctx).First(&run, "id = ?", runID).Error; err != nil {
		return nil, err
	}
	return &run, nil
}

// UpdateRun 更新Run
func (r *GormRepository) UpdateRun(ctx context.Context, runID string, patch *RunPatch) error {
	updates := make(map[string]interface{})
	if patch.Status != nil {
		updates["status"] = *patch.Status
	}
	if patch.CurrentStage != nil {
		updates["current_stage"] = *patch.CurrentStage
	}
	if patch.Progress != nil {
		updates["progress"] = *patch.Progress
	}
	if patch.LastRunSeq != nil {
		updates["last_run_seq"] = *patch.LastRunSeq
	}
	if patch.StartedAt != nil {
		updates["started_at"] = *patch.StartedAt
	}
	if patch.FinishedAt != nil {
		updates["finished_at"] = *patch.FinishedAt
	}

	if len(updates) == 0 {
		return nil
	}

	return r.db.WithContext(ctx).Model(&TaskRun{}).Where("id = ?", runID).Updates(updates).Error
}

// ListRuns 列出Runs
func (r *GormRepository) ListRuns(ctx context.Context, limit int) ([]TaskRun, error) {
	var runs []TaskRun
	err := r.db.WithContext(ctx).Order("created_at DESC").Limit(limit).Find(&runs).Error
	return runs, err
}

// ListRunningRuns 列出运行中的Runs
func (r *GormRepository) ListRunningRuns(ctx context.Context) ([]TaskRun, error) {
	var runs []TaskRun
	err := r.db.WithContext(ctx).Where("status = ?", RunStatusRunning).Find(&runs).Error
	return runs, err
}

// ==================== Stage Operations ====================

// CreateStage 创建Stage
func (r *GormRepository) CreateStage(ctx context.Context, stage *TaskRunStage) error {
	return r.db.WithContext(ctx).Create(stage).Error
}

// GetStage 获取Stage
func (r *GormRepository) GetStage(ctx context.Context, stageID string) (*TaskRunStage, error) {
	var stage TaskRunStage
	if err := r.db.WithContext(ctx).First(&stage, "id = ?", stageID).Error; err != nil {
		return nil, err
	}
	return &stage, nil
}

// UpdateStage 更新Stage
func (r *GormRepository) UpdateStage(ctx context.Context, stageID string, patch *StagePatch) error {
	updates := make(map[string]interface{})
	if patch.Status != nil {
		updates["status"] = *patch.Status
	}
	if patch.Progress != nil {
		updates["progress"] = *patch.Progress
	}
	if patch.CompletedUnits != nil {
		updates["completed_units"] = *patch.CompletedUnits
	}
	if patch.SuccessUnits != nil {
		updates["success_units"] = *patch.SuccessUnits
	}
	if patch.FailedUnits != nil {
		updates["failed_units"] = *patch.FailedUnits
	}
	if patch.CancelledUnits != nil {
		updates["cancelled_units"] = *patch.CancelledUnits
	}
	if patch.PartialUnits != nil {
		updates["partial_units"] = *patch.PartialUnits
	}
	if patch.StartedAt != nil {
		updates["started_at"] = *patch.StartedAt
	}
	if patch.FinishedAt != nil {
		updates["finished_at"] = *patch.FinishedAt
	}

	if len(updates) == 0 {
		return nil
	}

	return r.db.WithContext(ctx).Model(&TaskRunStage{}).Where("id = ?", stageID).Updates(updates).Error
}

// GetStagesByRun 获取Run的所有Stages
func (r *GormRepository) GetStagesByRun(ctx context.Context, runID string) ([]TaskRunStage, error) {
	var stages []TaskRunStage
	err := r.db.WithContext(ctx).Where("task_run_id = ?", runID).Order("stage_order ASC").Find(&stages).Error
	return stages, err
}

// ==================== Unit Operations ====================

// CreateUnit 创建Unit
func (r *GormRepository) CreateUnit(ctx context.Context, unit *TaskRunUnit) error {
	return r.db.WithContext(ctx).Create(unit).Error
}

// GetUnit 获取Unit
func (r *GormRepository) GetUnit(ctx context.Context, unitID string) (*TaskRunUnit, error) {
	var unit TaskRunUnit
	if err := r.db.WithContext(ctx).First(&unit, "id = ?", unitID).Error; err != nil {
		return nil, err
	}
	return &unit, nil
}

// UpdateUnit 更新Unit
func (r *GormRepository) UpdateUnit(ctx context.Context, unitID string, patch *UnitPatch) error {
	updates := make(map[string]interface{})
	if patch.Status != nil {
		updates["status"] = *patch.Status
	}
	if patch.DoneSteps != nil {
		updates["done_steps"] = *patch.DoneSteps
	}
	if patch.ErrorMessage != nil {
		updates["error_message"] = *patch.ErrorMessage
	}
	if patch.StartedAt != nil {
		updates["started_at"] = *patch.StartedAt
	}
	if patch.FinishedAt != nil {
		updates["finished_at"] = *patch.FinishedAt
	}

	if len(updates) == 0 {
		return nil
	}

	return r.db.WithContext(ctx).Model(&TaskRunUnit{}).Where("id = ?", unitID).Updates(updates).Error
}

// GetUnitsByStage 获取Stage的所有Units
func (r *GormRepository) GetUnitsByStage(ctx context.Context, stageID string) ([]TaskRunUnit, error) {
	var units []TaskRunUnit
	err := r.db.WithContext(ctx).Where("task_run_stage_id = ?", stageID).Find(&units).Error
	return units, err
}

// GetUnitsByRun 获取Run的所有Units
func (r *GormRepository) GetUnitsByRun(ctx context.Context, runID string) ([]TaskRunUnit, error) {
	var units []TaskRunUnit
	err := r.db.WithContext(ctx).Where("task_run_id = ?", runID).Find(&units).Error
	return units, err
}

// ==================== Event Operations ====================

// CreateEvent 创建Event
func (r *GormRepository) CreateEvent(ctx context.Context, event *TaskRunEvent) error {
	return r.db.WithContext(ctx).Create(event).Error
}

// GetEventsByRun 获取Run的Events
func (r *GormRepository) GetEventsByRun(ctx context.Context, runID string, limit int) ([]TaskRunEvent, error) {
	var events []TaskRunEvent
	query := r.db.WithContext(ctx).Where("task_run_id = ?", runID).Order("run_seq DESC, session_seq DESC, created_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&events).Error
	return events, err
}

// ==================== Artifact Operations ====================

// CreateArtifact 创建Artifact
func (r *GormRepository) CreateArtifact(ctx context.Context, artifact *TaskArtifact) error {
	return r.db.WithContext(ctx).Create(artifact).Error
}

// GetArtifactsByRun 获取Run的Artifacts
func (r *GormRepository) GetArtifactsByRun(ctx context.Context, runID string) ([]TaskArtifact, error) {
	var artifacts []TaskArtifact
	err := r.db.WithContext(ctx).Where("task_run_id = ?", runID).Find(&artifacts).Error
	return artifacts, err
}

// GetArtifactsByStage 获取Stage的Artifacts
func (r *GormRepository) GetArtifactsByStage(ctx context.Context, stageID string) ([]TaskArtifact, error) {
	var artifacts []TaskArtifact
	err := r.db.WithContext(ctx).Where("stage_id = ?", stageID).Find(&artifacts).Error
	return artifacts, err
}

// GetArtifactsByUnit 获取Unit的Artifacts
func (r *GormRepository) GetArtifactsByUnit(ctx context.Context, unitID string) ([]TaskArtifact, error) {
	var artifacts []TaskArtifact
	err := r.db.WithContext(ctx).Where("unit_id = ?", unitID).Find(&artifacts).Error
	return artifacts, err
}

// AutoMigrate 自动迁移数据库表
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&TaskDefinition{},
		&TaskRun{},
		&TaskRunStage{},
		&TaskRunUnit{},
		&TaskRunEvent{},
		&TaskArtifact{},
		&TaskRunDevice{},
		&TaskRawOutput{},
		&TaskParsedInterface{},
		&TaskParsedLLDPNeighbor{},
		&TaskParsedFDBEntry{},
		&TaskParsedARPEntry{},
		&TaskParsedAggregateGroup{},
		&TaskParsedAggregateMember{},
		&TaskTopologyEdge{},
	)
}

// EnsureRepository 确保仓库可用
func EnsureRepository(db *gorm.DB) (Repository, error) {
	if err := AutoMigrate(db); err != nil {
		return nil, fmt.Errorf("自动迁移失败: %w", err)
	}
	return NewGormRepository(db), nil
}
