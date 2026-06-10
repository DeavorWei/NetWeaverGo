// Package repository 提供数据访问层的抽象接口和实现
// snmp_repository.go 实现 SNMP Trap 数据访问接口
package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
)

// ============================================================================
// GORM 实现
// ============================================================================

// GormTrapRepository Trap Repository 的 GORM 实现
type GormTrapRepository struct {
	db *gorm.DB // 使用 config.SNMPDB（SNMP 独立数据库）
}

// NewGormTrapRepository 创建 Trap Repository 实例
func NewGormTrapRepository(db *gorm.DB) TrapRepository {
	return &GormTrapRepository{db: db}
}

// GormPollingRepository Polling Repository 的 GORM 实现
// P2-16: 类型别名，明确表示此类型实现 PollingRepository 接口
type GormPollingRepository = GormTrapRepository

// NewGormPollingRepository 创建 Polling Repository 实例
// 注意：PollingRepository 与 TrapRepository 使用相同的数据库连接
func NewGormPollingRepository(db *gorm.DB) PollingRepository {
	return &GormPollingRepository{db: db}
}

// ============================================================================
// Trap 记录管理
// ============================================================================

// CreateTrap 创建 Trap 记录
func (r *GormTrapRepository) CreateTrap(ctx context.Context, trap *models.SNMPTrapRecord) error {
	err := r.db.WithContext(ctx).Create(trap).Error
	if err != nil {
		logger.Error("SNMP-Repo", "-", "创建 Trap 记录失败: %v", err)
		return err
	}
	logger.Debug("SNMP-Repo", "-", "Trap 记录已创建: ID=%d, OID=%s", trap.ID, trap.TrapOID)
	return nil
}

// GetTrap 获取单个 Trap 记录
func (r *GormTrapRepository) GetTrap(ctx context.Context, id uint) (*models.SNMPTrapRecord, error) {
	var trap models.SNMPTrapRecord
	err := r.db.WithContext(ctx).First(&trap, id).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &trap, nil
}

// ListTraps 分页查询 Trap 记录
func (r *GormTrapRepository) ListTraps(ctx context.Context, filter TrapFilter, page, pageSize int) ([]*models.SNMPTrapRecord, int64, error) {
	var traps []*models.SNMPTrapRecord
	var total int64

	query := r.db.WithContext(ctx).Model(&models.SNMPTrapRecord{})

	// 应用过滤条件
	query = r.applyTrapFilter(query, filter)

	// 统计总数
	if err := query.Count(&total).Error; err != nil {
		logger.Error("SNMP-Repo", "-", "统计 Trap 记录失败: %v", err)
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	if offset < 0 {
		offset = 0
	}

	err := query.Order("received_at DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&traps).Error
	if err != nil {
		logger.Error("SNMP-Repo", "-", "查询 Trap 记录失败: %v", err)
		return nil, 0, err
	}

	return traps, total, nil
}

// DeleteTrap 删除单个 Trap 记录
func (r *GormTrapRepository) DeleteTrap(ctx context.Context, id uint) error {
	err := r.db.WithContext(ctx).Delete(&models.SNMPTrapRecord{}, id).Error
	if err != nil {
		logger.Error("SNMP-Repo", "-", "删除 Trap 记录失败: ID=%d, %v", id, err)
		return err
	}
	logger.Info("SNMP-Repo", "-", "Trap 记录已删除: ID=%d", id)
	return nil
}

// DeleteTrapsBefore 删除指定时间之前的 Trap 记录
func (r *GormTrapRepository) DeleteTrapsBefore(ctx context.Context, before time.Time) (int64, error) {
	result := r.db.WithContext(ctx).
		Where("received_at < ?", before).
		Delete(&models.SNMPTrapRecord{})
	if result.Error != nil {
		logger.Error("SNMP-Repo", "-", "批量删除 Trap 记录失败: %v", result.Error)
		return 0, result.Error
	}
	logger.Info("SNMP-Repo", "-", "已删除 %d 条过期 Trap 记录 (早于 %s)", result.RowsAffected, before.Format("2006-01-02 15:04:05"))
	return result.RowsAffected, nil
}

// AcknowledgeTrap 确认 Trap 记录
func (r *GormTrapRepository) AcknowledgeTrap(ctx context.Context, id uint) error {
	now := time.Now()
	err := r.db.WithContext(ctx).
		Model(&models.SNMPTrapRecord{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"acknowledged":   true,
			"acknowledged_at": now,
		}).Error
	if err != nil {
		logger.Error("SNMP-Repo", "-", "确认 Trap 记录失败: ID=%d, %v", id, err)
		return err
	}
	logger.Debug("SNMP-Repo", "-", "Trap 记录已确认: ID=%d", id)
	return nil
}

// BatchAcknowledgeTraps 批量确认 Trap 记录
func (r *GormTrapRepository) BatchAcknowledgeTraps(ctx context.Context, ids []uint) error {
	if len(ids) == 0 {
		return nil
	}

	now := time.Now()
	err := r.db.WithContext(ctx).
		Model(&models.SNMPTrapRecord{}).
		Where("id IN ?", ids).
		Updates(map[string]interface{}{
			"acknowledged":   true,
			"acknowledged_at": now,
		}).Error
	if err != nil {
		logger.Error("SNMP-Repo", "-", "批量确认 Trap 记录失败: %v", err)
		return err
	}
	logger.Info("SNMP-Repo", "-", "已批量确认 %d 条 Trap 记录", len(ids))
	return nil
}

// GetTrapStats 获取 Trap 统计信息
func (r *GormTrapRepository) GetTrapStats(ctx context.Context) (*TrapStats, error) {
	var stats TrapStats
	dbCtx := ctx

	// 总数
	if err := r.db.WithContext(dbCtx).Model(&models.SNMPTrapRecord{}).Count(&stats.TotalCount).Error; err != nil {
		return nil, err
	}

	// 未确认数
	if err := r.db.WithContext(dbCtx).Model(&models.SNMPTrapRecord{}).Where("acknowledged = ?", false).Count(&stats.Unacknowledged).Error; err != nil {
		return nil, err
	}

	// 按严重级别统计
	severityCounts := make(map[string]int64)
	rows, err := r.db.WithContext(dbCtx).
		Model(&models.SNMPTrapRecord{}).
		Select("severity, COUNT(*) as count").
		Group("severity").
		Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var severity string
		var count int64
		if err := rows.Scan(&severity, &count); err != nil {
			continue
		}
		severityCounts[severity] = count
	}

	stats.CriticalCount = severityCounts["critical"]
	stats.MajorCount = severityCounts["major"]
	stats.MinorCount = severityCounts["minor"]
	stats.InfoCount = severityCounts["info"]

	// 今日统计
	today := time.Now().Format("2006-01-02")
	todayStart, _ := time.Parse("2006-01-02", today)
	if err := r.db.WithContext(dbCtx).
		Model(&models.SNMPTrapRecord{}).
		Where("received_at >= ?", todayStart).
		Count(&stats.TodayCount).Error; err != nil {
		return nil, err
	}

	// 最近一小时统计
	oneHourAgo := time.Now().Add(-1 * time.Hour)
	if err := r.db.WithContext(dbCtx).
		Model(&models.SNMPTrapRecord{}).
		Where("received_at >= ?", oneHourAgo).
		Count(&stats.LastHourCount).Error; err != nil {
		return nil, err
	}

	return &stats, nil
}

// ============================================================================
// 过滤规则管理
// ============================================================================

// CreateFilterRule 创建过滤规则
func (r *GormTrapRepository) CreateFilterRule(ctx context.Context, rule *models.SNMPTrapFilterRule) error {
	err := r.db.WithContext(ctx).Create(rule).Error
	if err != nil {
		logger.Error("SNMP-Repo", "-", "创建过滤规则失败: %v", err)
		return err
	}
	logger.Info("SNMP-Repo", "-", "过滤规则已创建: ID=%d, Name=%s", rule.ID, rule.Name)
	return nil
}

// GetFilterRule 获取单个过滤规则
func (r *GormTrapRepository) GetFilterRule(ctx context.Context, id uint) (*models.SNMPTrapFilterRule, error) {
	var rule models.SNMPTrapFilterRule
	err := r.db.WithContext(ctx).First(&rule, id).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &rule, nil
}

// ListFilterRules 获取所有过滤规则（按优先级排序）
func (r *GormTrapRepository) ListFilterRules(ctx context.Context) ([]*models.SNMPTrapFilterRule, error) {
	var rules []*models.SNMPTrapFilterRule
	err := r.db.WithContext(ctx).
		Order("priority ASC, created_at ASC").
		Find(&rules).Error
	if err != nil {
		logger.Error("SNMP-Repo", "-", "查询过滤规则失败: %v", err)
		return nil, err
	}
	return rules, nil
}

// UpdateFilterRule 更新过滤规则
func (r *GormTrapRepository) UpdateFilterRule(ctx context.Context, rule *models.SNMPTrapFilterRule) error {
	err := r.db.WithContext(ctx).Save(rule).Error
	if err != nil {
		logger.Error("SNMP-Repo", "-", "更新过滤规则失败: ID=%d, %v", rule.ID, err)
		return err
	}
	logger.Info("SNMP-Repo", "-", "过滤规则已更新: ID=%d, Name=%s", rule.ID, rule.Name)
	return nil
}

// DeleteFilterRule 删除过滤规则
func (r *GormTrapRepository) DeleteFilterRule(ctx context.Context, id uint) error {
	err := r.db.WithContext(ctx).Delete(&models.SNMPTrapFilterRule{}, id).Error
	if err != nil {
		logger.Error("SNMP-Repo", "-", "删除过滤规则失败: ID=%d, %v", id, err)
		return err
	}
	logger.Info("SNMP-Repo", "-", "过滤规则已删除: ID=%d", id)
	return nil
}

// ReorderFilterRules 重新排序过滤规则
func (r *GormTrapRepository) ReorderFilterRules(ctx context.Context, ids []uint) error {
	if len(ids) == 0 {
		return nil
	}

	dbCtx := ctx
	return r.db.WithContext(dbCtx).Transaction(func(tx *gorm.DB) error {
		for i, id := range ids {
			if err := tx.Model(&models.SNMPTrapFilterRule{}).
				Where("id = ?", id).
				Update("priority", i+1).Error; err != nil {
				logger.Error("SNMP-Repo", "-", "重排序过滤规则失败: ID=%d, %v", id, err)
				return err
			}
		}
		logger.Info("SNMP-Repo", "-", "已重排序 %d 条过滤规则", len(ids))
		return nil
	})
}

// ============================================================================
// 服务器配置管理
// ============================================================================

// CreateServerConfig 创建服务器配置
func (r *GormTrapRepository) CreateServerConfig(ctx context.Context, config *models.SNMPServerConfig) error {
	err := r.db.WithContext(ctx).Create(config).Error
	if err != nil {
		logger.Error("SNMP-Repo", "-", "创建服务器配置失败: %v", err)
		return err
	}
	logger.Info("SNMP-Repo", "-", "服务器配置已创建: ID=%d", config.ID)
	return nil
}

// GetServerConfig 获取单个服务器配置
func (r *GormTrapRepository) GetServerConfig(ctx context.Context, id uint) (*models.SNMPServerConfig, error) {
	var config models.SNMPServerConfig
	err := r.db.WithContext(ctx).First(&config, id).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &config, nil
}

// GetActiveServerConfig 获取当前活动的服务器配置
// 服务器配置通常是单例，返回第一条记录
func (r *GormTrapRepository) GetActiveServerConfig(ctx context.Context) (*models.SNMPServerConfig, error) {
	var config models.SNMPServerConfig
	err := r.db.WithContext(ctx).First(&config).Error
	if err == gorm.ErrRecordNotFound {
		// 如果不存在，创建默认配置
		defaultConfig := &models.SNMPServerConfig{
			TrapEnabled:      false,
			TrapPort:         1162, // 使用非特权端口
			TrapCommunity:    "public",
			V3Enabled:        false,
			MaxStorageDays:  30,
			PollingEnabled:   false,
			MaxPollingWorkers: 10,
			PollingResultRetentionDays: 7,
		}
		if err := r.db.WithContext(ctx).Create(defaultConfig).Error; err != nil {
			return nil, err
		}
		return defaultConfig, nil
	}
	if err != nil {
		return nil, err
	}
	return &config, nil
}

// ListServerConfigs 获取所有服务器配置
func (r *GormTrapRepository) ListServerConfigs(ctx context.Context) ([]*models.SNMPServerConfig, error) {
	var configs []*models.SNMPServerConfig
	err := r.db.WithContext(ctx).Find(&configs).Error
	if err != nil {
		logger.Error("SNMP-Repo", "-", "查询服务器配置失败: %v", err)
		return nil, err
	}
	return configs, nil
}

// UpdateServerConfig 更新服务器配置
func (r *GormTrapRepository) UpdateServerConfig(ctx context.Context, config *models.SNMPServerConfig) error {
	err := r.db.WithContext(ctx).Save(config).Error
	if err != nil {
		logger.Error("SNMP-Repo", "-", "更新服务器配置失败: ID=%d, %v", config.ID, err)
		return err
	}
	logger.Info("SNMP-Repo", "-", "服务器配置已更新: ID=%d", config.ID)
	return nil
}

// DeleteServerConfig 删除服务器配置
func (r *GormTrapRepository) DeleteServerConfig(ctx context.Context, id uint) error {
	err := r.db.WithContext(ctx).Delete(&models.SNMPServerConfig{}, id).Error
	if err != nil {
		logger.Error("SNMP-Repo", "-", "删除服务器配置失败: ID=%d, %v", id, err)
		return err
	}
	logger.Info("SNMP-Repo", "-", "服务器配置已删除: ID=%d", id)
	return nil
}

// ============================================================================
// 辅助方法
// ============================================================================


// applyTrapFilter 应用 Trap 过滤条件
func (r *GormTrapRepository) applyTrapFilter(query *gorm.DB, filter TrapFilter) *gorm.DB {
	if filter.SourceIP != "" {
		query = query.Where("source_ip = ?", filter.SourceIP)
	}
	if filter.TrapOID != "" {
		query = query.Where("trap_oid LIKE ?", filter.TrapOID+"%")
	}
	if filter.Severity != "" {
		query = query.Where("severity = ?", filter.Severity)
	}
	if filter.StartTime != nil {
		query = query.Where("received_at >= ?", filter.StartTime)
	}
	if filter.EndTime != nil {
		query = query.Where("received_at <= ?", filter.EndTime)
	}
	if filter.Acknowledged != nil {
		query = query.Where("acknowledged = ?", *filter.Acknowledged)
	}
	if filter.SearchQuery != "" {
		searchPattern := "%" + filter.SearchQuery + "%"
		query = query.Where(
			query.Where("source_ip LIKE ?", searchPattern).
				Or("trap_oid LIKE ?", searchPattern).
				Or("trap_name LIKE ?", searchPattern).
				Or("community LIKE ?", searchPattern),
		)
	}
	return query
}

// ============================================================================
// 凭据管理（PollingRepository 接口实现）
// ============================================================================

// CreateCredential 创建 SNMP 凭据
func (r *GormTrapRepository) CreateCredential(ctx context.Context, cred *models.SNMPCredential) error {
	err := r.db.WithContext(ctx).Create(cred).Error
	if err != nil {
		logger.Error("SNMP-Repo", "-", "创建凭据失败: %v", err)
		return err
	}
	logger.Info("SNMP-Repo", "-", "凭据已创建: ID=%d, Name=%s", cred.ID, cred.Name)
	return nil
}

// GetCredential 获取单个 SNMP 凭据
func (r *GormTrapRepository) GetCredential(ctx context.Context, id uint) (*models.SNMPCredential, error) {
	var cred models.SNMPCredential
	err := r.db.WithContext(ctx).First(&cred, id).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &cred, nil
}

// ListCredentials 获取所有 SNMP 凭据
func (r *GormTrapRepository) ListCredentials(ctx context.Context) ([]*models.SNMPCredential, error) {
	var creds []*models.SNMPCredential
	err := r.db.WithContext(ctx).
		Order("created_at DESC").
		Find(&creds).Error
	if err != nil {
		logger.Error("SNMP-Repo", "-", "查询凭据列表失败: %v", err)
		return nil, err
	}
	return creds, nil
}

// UpdateCredential 更新 SNMP 凭据
func (r *GormTrapRepository) UpdateCredential(ctx context.Context, cred *models.SNMPCredential) error {
	err := r.db.WithContext(ctx).Save(cred).Error
	if err != nil {
		logger.Error("SNMP-Repo", "-", "更新凭据失败: ID=%d, %v", cred.ID, err)
		return err
	}
	logger.Info("SNMP-Repo", "-", "凭据已更新: ID=%d, Name=%s", cred.ID, cred.Name)
	return nil
}

// DeleteCredential 删除 SNMP 凭据
func (r *GormTrapRepository) DeleteCredential(ctx context.Context, id uint) error {
	err := r.db.WithContext(ctx).Delete(&models.SNMPCredential{}, id).Error
	if err != nil {
		logger.Error("SNMP-Repo", "-", "删除凭据失败: ID=%d, %v", id, err)
		return err
	}
	logger.Info("SNMP-Repo", "-", "凭据已删除: ID=%d", id)
	return nil
}

// ============================================================================
// 轮询模板管理（PollingRepository 接口实现）
// ============================================================================

// CreatePollingTemplate 创建轮询模板
func (r *GormTrapRepository) CreatePollingTemplate(ctx context.Context, template *models.SNMPPollingTemplate) error {
	err := r.db.WithContext(ctx).Create(template).Error
	if err != nil {
		logger.Error("SNMP-Repo", "-", "创建轮询模板失败: %v", err)
		return err
	}
	logger.Info("SNMP-Repo", "-", "轮询模板已创建: ID=%d, Name=%s", template.ID, template.Name)
	return nil
}

// GetPollingTemplate 获取单个轮询模板
func (r *GormTrapRepository) GetPollingTemplate(ctx context.Context, id uint) (*models.SNMPPollingTemplate, error) {
	var template models.SNMPPollingTemplate
	err := r.db.WithContext(ctx).First(&template, id).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &template, nil
}

// ListPollingTemplates 获取所有轮询模板
func (r *GormTrapRepository) ListPollingTemplates(ctx context.Context) ([]*models.SNMPPollingTemplate, error) {
	var templates []*models.SNMPPollingTemplate
	err := r.db.WithContext(ctx).
		Order("category ASC, created_at DESC").
		Find(&templates).Error
	if err != nil {
		logger.Error("SNMP-Repo", "-", "查询轮询模板列表失败: %v", err)
		return nil, err
	}
	return templates, nil
}

// UpdatePollingTemplate 更新轮询模板
func (r *GormTrapRepository) UpdatePollingTemplate(ctx context.Context, template *models.SNMPPollingTemplate) error {
	err := r.db.WithContext(ctx).Save(template).Error
	if err != nil {
		logger.Error("SNMP-Repo", "-", "更新轮询模板失败: ID=%d, %v", template.ID, err)
		return err
	}
	logger.Info("SNMP-Repo", "-", "轮询模板已更新: ID=%d, Name=%s", template.ID, template.Name)
	return nil
}

// DeletePollingTemplate 删除轮询模板
func (r *GormTrapRepository) DeletePollingTemplate(ctx context.Context, id uint) error {
	err := r.db.WithContext(ctx).Delete(&models.SNMPPollingTemplate{}, id).Error
	if err != nil {
		logger.Error("SNMP-Repo", "-", "删除轮询模板失败: ID=%d, %v", id, err)
		return err
	}
	logger.Info("SNMP-Repo", "-", "轮询模板已删除: ID=%d", id)
	return nil
}

// ============================================================================
// 轮询目标管理（PollingRepository 接口实现）
// ============================================================================

// CreatePollingTarget 创建轮询目标
func (r *GormTrapRepository) CreatePollingTarget(ctx context.Context, target *models.SNMPPollingTarget) error {
	err := r.db.WithContext(ctx).Create(target).Error
	if err != nil {
		logger.Error("SNMP-Repo", "-", "创建轮询目标失败: %v", err)
		return err
	}
	logger.Info("SNMP-Repo", "-", "轮询目标已创建: ID=%d, IP=%s", target.ID, target.TargetIP)
	return nil
}

// GetPollingTarget 获取单个轮询目标
func (r *GormTrapRepository) GetPollingTarget(ctx context.Context, id uint) (*models.SNMPPollingTarget, error) {
	var target models.SNMPPollingTarget
	err := r.db.WithContext(ctx).First(&target, id).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &target, nil
}

// ListPollingTargets 分页查询轮询目标
func (r *GormTrapRepository) ListPollingTargets(ctx context.Context, filter PollingTargetFilter) ([]*models.SNMPPollingTarget, int64, error) {
	var targets []*models.SNMPPollingTarget
	var total int64

	query := r.db.WithContext(ctx).Model(&models.SNMPPollingTarget{})

	// 应用过滤条件
	query = r.applyPollingTargetFilter(query, filter)

	// 统计总数
	if err := query.Count(&total).Error; err != nil {
		logger.Error("SNMP-Repo", "-", "统计轮询目标失败: %v", err)
		return nil, 0, err
	}

	err := query.Order("created_at DESC").
		Find(&targets).Error
	if err != nil {
		logger.Error("SNMP-Repo", "-", "查询轮询目标失败: %v", err)
		return nil, 0, err
	}

	return targets, total, nil
}

// UpdatePollingTarget 更新轮询目标
func (r *GormTrapRepository) UpdatePollingTarget(ctx context.Context, target *models.SNMPPollingTarget) error {
	err := r.db.WithContext(ctx).Save(target).Error
	if err != nil {
		logger.Error("SNMP-Repo", "-", "更新轮询目标失败: ID=%d, %v", target.ID, err)
		return err
	}
	logger.Info("SNMP-Repo", "-", "轮询目标已更新: ID=%d, IP=%s", target.ID, target.TargetIP)
	return nil
}

// DeletePollingTarget 删除轮询目标
func (r *GormTrapRepository) DeletePollingTarget(ctx context.Context, id uint) error {
	err := r.db.WithContext(ctx).Delete(&models.SNMPPollingTarget{}, id).Error
	if err != nil {
		logger.Error("SNMP-Repo", "-", "删除轮询目标失败: ID=%d, %v", id, err)
		return err
	}
	logger.Info("SNMP-Repo", "-", "轮询目标已删除: ID=%d", id)
	return nil
}

// ============================================================================
// 轮询结果管理（PollingRepository 接口实现）
// ============================================================================

// CreatePollingResult 创建单条轮询结果
func (r *GormTrapRepository) CreatePollingResult(ctx context.Context, result *models.SNMPPollingResult) error {
	err := r.db.WithContext(ctx).Create(result).Error
	if err != nil {
		logger.Error("SNMP-Repo", "-", "创建轮询结果失败: %v", err)
		return err
	}
	return nil
}

// CreatePollingResults 批量创建轮询结果
func (r *GormTrapRepository) CreatePollingResults(ctx context.Context, results []*models.SNMPPollingResult) error {
	if len(results) == 0 {
		return nil
	}

	err := r.db.WithContext(ctx).CreateInBatches(results, 100).Error
	if err != nil {
		logger.Error("SNMP-Repo", "-", "批量创建轮询结果失败: %v", err)
		return err
	}
	logger.Debug("SNMP-Repo", "-", "已批量创建 %d 条轮询结果", len(results))
	return nil
}

// GetPollingResult 获取单条轮询结果
func (r *GormTrapRepository) GetPollingResult(ctx context.Context, id uint) (*models.SNMPPollingResult, error) {
	var result models.SNMPPollingResult
	err := r.db.WithContext(ctx).First(&result, id).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// ListPollingResults 分页查询轮询结果
func (r *GormTrapRepository) ListPollingResults(ctx context.Context, filter PollingResultFilter, page, pageSize int) ([]*models.SNMPPollingResult, int64, error) {
	var results []*models.SNMPPollingResult
	var total int64

	query := r.db.WithContext(ctx).Model(&models.SNMPPollingResult{})

	// 应用过滤条件
	query = r.applyPollingResultFilter(query, filter)

	// 统计总数
	if err := query.Count(&total).Error; err != nil {
		logger.Error("SNMP-Repo", "-", "统计轮询结果失败: %v", err)
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	if offset < 0 {
		offset = 0
	}

	err := query.Order("poll_time DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&results).Error
	if err != nil {
		logger.Error("SNMP-Repo", "-", "查询轮询结果失败: %v", err)
		return nil, 0, err
	}

	return results, total, nil
}

// DeletePollingResultsBefore 删除指定时间之前的轮询结果
func (r *GormTrapRepository) DeletePollingResultsBefore(ctx context.Context, before time.Time) (int64, error) {
	result := r.db.WithContext(ctx).
		Where("poll_time < ?", before).
		Delete(&models.SNMPPollingResult{})
	if result.Error != nil {
		logger.Error("SNMP-Repo", "-", "批量删除轮询结果失败: %v", result.Error)
		return 0, result.Error
	}
	logger.Info("SNMP-Repo", "-", "已删除 %d 条过期轮询结果 (早于 %s)", result.RowsAffected, before.Format("2006-01-02 15:04:05"))
	return result.RowsAffected, nil
}

// DeleteAllPollingResults 删除所有轮询结果
func (r *GormTrapRepository) DeleteAllPollingResults(ctx context.Context) (int64, error) {
	result := r.db.WithContext(ctx).Where("1 = 1").Delete(&models.SNMPPollingResult{})
	if result.Error != nil {
		logger.Error("SNMP-Repo", "-", "删除所有轮询结果失败: %v", result.Error)
		return 0, result.Error
	}
	logger.Info("SNMP-Repo", "-", "已删除所有轮询结果: %d 条", result.RowsAffected)
	return result.RowsAffected, nil
}

// ============================================================================
// 轮询统计查询（PollingRepository 接口实现）
// ============================================================================

// GetPollingStats 获取指定目标的轮询统计信息
func (r *GormTrapRepository) GetPollingStats(ctx context.Context, targetID uint) (*PollingStats, error) {
	var stats PollingStats
	dbCtx := ctx

	// 1. 统计总轮询次数（按 batch_id 去重统计）
	if err := r.db.WithContext(dbCtx).
		Model(&models.SNMPPollingResult{}).
		Where("target_id = ?", targetID).
		Distinct("batch_id").
		Count(&stats.TotalPolls).Error; err != nil {
		return nil, err
	}

	// 2. 统计失败次数（存在 value_type = 'error' 的 batch_id 数量）
	if err := r.db.WithContext(dbCtx).
		Model(&models.SNMPPollingResult{}).
		Where("target_id = ? AND value_type = ?", targetID, "error").
		Distinct("batch_id").
		Count(&stats.FailCount).Error; err != nil {
		return nil, err
	}

	// 3. 计算成功次数
	stats.SuccessCount = stats.TotalPolls - stats.FailCount
	if stats.SuccessCount < 0 {
		stats.SuccessCount = 0
	}

	// 4. 获取最后轮询时间
	var lastResult models.SNMPPollingResult
	err := r.db.WithContext(dbCtx).
		Where("target_id = ?", targetID).
		Order("poll_time DESC").
		First(&lastResult).Error
	if err == nil {
		stats.LastPollTime = &lastResult.PollTime
	} else if err != gorm.ErrRecordNotFound {
		return nil, err
	}

	// 5. 计算平均延迟（暂时留 0，因为目前未保存单次请求的 Latency）
	stats.AvgLatencyMs = 0

	return &stats, nil
}

// ============================================================================
// 轮询相关辅助方法
// ============================================================================

// applyPollingTargetFilter 应用轮询目标过滤条件
func (r *GormTrapRepository) applyPollingTargetFilter(query *gorm.DB, filter PollingTargetFilter) *gorm.DB {
	if filter.TemplateID != nil {
		query = query.Where("template_id = ?", *filter.TemplateID)
	}
	if filter.Enabled != nil {
		query = query.Where("enabled = ?", *filter.Enabled)
	}
	if filter.SearchIP != "" {
		searchPattern := "%" + filter.SearchIP + "%"
		query = query.Where("target_ip LIKE ?", searchPattern)
	}
	return query
}

// applyPollingResultFilter 应用轮询结果过滤条件
func (r *GormTrapRepository) applyPollingResultFilter(query *gorm.DB, filter PollingResultFilter) *gorm.DB {
	if filter.TargetID != nil {
		query = query.Where("target_id = ?", *filter.TargetID)
	}
	if filter.OID != "" {
		query = query.Where("oid = ?", filter.OID)
	}
	if filter.StartTime != nil {
		query = query.Where("poll_time >= ?", filter.StartTime)
	}
	if filter.EndTime != nil {
		query = query.Where("poll_time <= ?", filter.EndTime)
	}
	if filter.BatchID != "" {
		query = query.Where("batch_id = ?", filter.BatchID)
	}
	return query
}
