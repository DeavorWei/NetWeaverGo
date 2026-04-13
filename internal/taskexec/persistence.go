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
	ListRunsFiltered(ctx context.Context, limit int, taskGroupID uint, runKind, status string) ([]TaskRun, error)
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

	// ==================== 删除操作 ====================

	// DeleteRun 删除运行记录（含级联删除所有关联数据）
	DeleteRun(ctx context.Context, runID string) error

	// DeleteAllRuns 删除所有运行记录
	DeleteAllRuns(ctx context.Context) error

	// DeleteAllRunsBatch 批量删除所有运行记录（单事务，性能优化版本）
	DeleteAllRunsBatch(ctx context.Context) error

	// DeleteRunsByKind 按类型删除运行记录
	DeleteRunsByKind(ctx context.Context, runKind string) error

	// DeleteRunsByTaskGroup 按任务组删除运行记录
	DeleteRunsByTaskGroup(ctx context.Context, taskGroupID uint) error
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

// ListRunsFiltered 按条件筛选运行记录
func (r *GormRepository) ListRunsFiltered(ctx context.Context, limit int, taskGroupID uint, runKind, status string) ([]TaskRun, error) {
	var runs []TaskRun
	query := r.db.WithContext(ctx).Order("created_at DESC")

	// 按 TaskGroupID 筛选
	if taskGroupID > 0 {
		query = query.Where("task_group_id = ?", taskGroupID)
	}

	// 按 RunKind 筛选
	if runKind != "" {
		query = query.Where("run_kind = ?", runKind)
	}

	// 按 Status 筛选
	if status != "" {
		query = query.Where("status = ?", status)
	}

	// 应用限制
	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Find(&runs).Error
	return runs, err
}

// ListRunningRuns 列出所有活跃 Runs（包括 pending / running）。
func (r *GormRepository) ListRunningRuns(ctx context.Context) ([]TaskRun, error) {
	var runs []TaskRun
	statuses := ActiveRunStatuses()
	values := make([]string, 0, len(statuses))
	for _, status := range statuses {
		values = append(values, string(status))
	}
	err := r.db.WithContext(ctx).Where("status IN ?", values).Find(&runs).Error
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

// ==================== Delete Operations ====================

// DeleteRun 删除运行记录（含级联删除所有关联数据）
func (r *GormRepository) DeleteRun(ctx context.Context, runID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. 删除解析数据表（拓扑采集特有）
		// 1.1 删除拓扑边
		if err := tx.Where("task_run_id = ?", runID).Delete(&TaskTopologyEdge{}).Error; err != nil {
			return fmt.Errorf("删除拓扑边失败: %w", err)
		}

		// 1.2 删除聚合组成员
		if err := tx.Where("task_run_id = ?", runID).Delete(&TaskParsedAggregateMember{}).Error; err != nil {
			return fmt.Errorf("删除聚合组成员失败: %w", err)
		}

		// 1.3 删除聚合组
		if err := tx.Where("task_run_id = ?", runID).Delete(&TaskParsedAggregateGroup{}).Error; err != nil {
			return fmt.Errorf("删除聚合组失败: %w", err)
		}

		// 1.4 删除ARP条目
		if err := tx.Where("task_run_id = ?", runID).Delete(&TaskParsedARPEntry{}).Error; err != nil {
			return fmt.Errorf("删除ARP条目失败: %w", err)
		}

		// 1.5 删除FDB条目
		if err := tx.Where("task_run_id = ?", runID).Delete(&TaskParsedFDBEntry{}).Error; err != nil {
			return fmt.Errorf("删除FDB条目失败: %w", err)
		}

		// 1.6 删除LLDP邻居
		if err := tx.Where("task_run_id = ?", runID).Delete(&TaskParsedLLDPNeighbor{}).Error; err != nil {
			return fmt.Errorf("删除LLDP邻居失败: %w", err)
		}

		// 1.7 删除接口数据
		if err := tx.Where("task_run_id = ?", runID).Delete(&TaskParsedInterface{}).Error; err != nil {
			return fmt.Errorf("删除接口数据失败: %w", err)
		}

		// 1.8 删除原始输出
		if err := tx.Where("task_run_id = ?", runID).Delete(&TaskRawOutput{}).Error; err != nil {
			return fmt.Errorf("删除原始输出失败: %w", err)
		}

		// 1.9 删除设备记录
		if err := tx.Where("task_run_id = ?", runID).Delete(&TaskRunDevice{}).Error; err != nil {
			return fmt.Errorf("删除设备记录失败: %w", err)
		}

		// 2. 删除通用关联数据
		// 2.1 删除产物
		if err := tx.Where("task_run_id = ?", runID).Delete(&TaskArtifact{}).Error; err != nil {
			return fmt.Errorf("删除产物失败: %w", err)
		}

		// 2.2 删除事件
		if err := tx.Where("task_run_id = ?", runID).Delete(&TaskRunEvent{}).Error; err != nil {
			return fmt.Errorf("删除事件失败: %w", err)
		}

		// 2.3 删除单元
		if err := tx.Where("task_run_id = ?", runID).Delete(&TaskRunUnit{}).Error; err != nil {
			return fmt.Errorf("删除单元失败: %w", err)
		}

		// 2.4 删除阶段
		if err := tx.Where("task_run_id = ?", runID).Delete(&TaskRunStage{}).Error; err != nil {
			return fmt.Errorf("删除阶段失败: %w", err)
		}

		// 3. 删除主记录
		if err := tx.Where("id = ?", runID).Delete(&TaskRun{}).Error; err != nil {
			return fmt.Errorf("删除运行记录失败: %w", err)
		}

		return nil
	})
}

// DeleteAllRuns 删除所有运行记录
func (r *GormRepository) DeleteAllRuns(ctx context.Context) error {
	var runIDs []string
	if err := r.db.WithContext(ctx).Model(&TaskRun{}).Pluck("id", &runIDs).Error; err != nil {
		return fmt.Errorf("获取运行记录列表失败: %w", err)
	}

	for _, runID := range runIDs {
		if err := r.DeleteRun(ctx, runID); err != nil {
			return fmt.Errorf("删除运行记录 %s 失败: %w", runID, err)
		}
	}
	return nil
}

// DeleteAllRunsBatch 批量删除所有运行记录（单事务，性能优化版本）
func (r *GormRepository) DeleteAllRunsBatch(ctx context.Context) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 批量删除所有关联表（按依赖顺序）
		tables := []interface{}{
			&TaskTopologyEdge{},
			&TaskParsedAggregateMember{},
			&TaskParsedAggregateGroup{},
			&TaskParsedARPEntry{},
			&TaskParsedFDBEntry{},
			&TaskParsedLLDPNeighbor{},
			&TaskParsedInterface{},
			&TaskRawOutput{},
			&TaskRunDevice{},
			&TaskArtifact{},
			&TaskRunEvent{},
			&TaskRunUnit{},
			&TaskRunStage{},
			&TaskRun{},
		}

		for _, table := range tables {
			if err := tx.Where("1 = 1").Delete(table).Error; err != nil {
				return fmt.Errorf("清空表 %T 失败: %w", table, err)
			}
		}

		return nil
	})
}

// DeleteRunsByKind 按类型删除运行记录
func (r *GormRepository) DeleteRunsByKind(ctx context.Context, runKind string) error {
	var runIDs []string
	if err := r.db.WithContext(ctx).Model(&TaskRun{}).Where("run_kind = ?", runKind).Pluck("id", &runIDs).Error; err != nil {
		return fmt.Errorf("获取运行记录列表失败: %w", err)
	}

	for _, runID := range runIDs {
		if err := r.DeleteRun(ctx, runID); err != nil {
			return fmt.Errorf("删除运行记录 %s 失败: %w", runID, err)
		}
	}
	return nil
}

// DeleteRunsByTaskGroup 按任务组删除运行记录
func (r *GormRepository) DeleteRunsByTaskGroup(ctx context.Context, taskGroupID uint) error {
	var runIDs []string
	if err := r.db.WithContext(ctx).Model(&TaskRun{}).Where("task_group_id = ?", taskGroupID).Pluck("id", &runIDs).Error; err != nil {
		return fmt.Errorf("获取运行记录列表失败: %w", err)
	}

	for _, runID := range runIDs {
		if err := r.DeleteRun(ctx, runID); err != nil {
			return fmt.Errorf("删除运行记录 %s 失败: %w", runID, err)
		}
	}
	return nil
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
		// 离线重放模式新增表
		&TopologyReplayRecord{},
	)
}

// EnsureRepository 确保仓库可用
func EnsureRepository(db *gorm.DB) (Repository, error) {
	if err := AutoMigrate(db); err != nil {
		return nil, fmt.Errorf("自动迁移失败: %w", err)
	}
	return NewGormRepository(db), nil
}
