package config

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CreateExecutionRecord 创建历史执行记录
func CreateExecutionRecord(record models.ExecutionRecord) (*models.ExecutionRecord, error) {
	if record.ID == "" {
		record.ID = uuid.New().String()
	}
	if record.CreatedAt == "" {
		record.CreatedAt = time.Now().Format(time.RFC3339)
	}

	if err := DB.Create(&record).Error; err != nil {
		logger.Error("ExecutionRecord", "-", "创建历史执行记录失败: %v", err)
		return nil, fmt.Errorf("创建历史执行记录失败: %v", err)
	}

	logger.Info("ExecutionRecord", "-", "创建历史执行记录成功: %s", record.ID)
	return &record, nil
}

// GetExecutionRecord 根据 ID 获取历史执行记录
func GetExecutionRecord(id string) (*models.ExecutionRecord, error) {
	var record models.ExecutionRecord
	if err := DB.First(&record, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("历史执行记录不存在: %s", id)
		}
		return nil, fmt.Errorf("查询历史执行记录失败: %v", err)
	}
	return &record, nil
}

// ExecutionQueryOptions 查询选项
type ExecutionQueryOptions struct {
	TaskGroupID  string
	RunnerSource string
	Status       string
	Page         int
	PageSize     int
	SortBy       string
	SortOrder    string // asc / desc
}

// ExecutionQueryResult 查询结果
type ExecutionQueryResult struct {
	Data       []models.ExecutionRecord `json:"data"`
	Total      int64                    `json:"total"`
	Page       int                      `json:"page"`
	PageSize   int                      `json:"pageSize"`
	TotalPages int                      `json:"totalPages"`
}

// ListExecutionRecords 查询历史执行记录列表
func ListExecutionRecords(opts ExecutionQueryOptions) (*ExecutionQueryResult, error) {
	// 默认值处理
	if opts.Page <= 0 {
		opts.Page = 1
	}
	if opts.PageSize <= 0 {
		opts.PageSize = 20
	}
	if opts.SortBy == "" {
		opts.SortBy = "started_at"
	}
	if opts.SortOrder == "" {
		opts.SortOrder = "desc"
	}

	query := DB.Model(&models.ExecutionRecord{})

	// 应用筛选条件
	if opts.TaskGroupID != "" {
		query = query.Where("task_group_id = ?", opts.TaskGroupID)
	}
	if opts.RunnerSource != "" {
		query = query.Where("runner_source = ?", opts.RunnerSource)
	}
	if opts.Status != "" {
		query = query.Where("status = ?", opts.Status)
	}

	// 计算总数
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("查询历史记录总数失败: %v", err)
	}

	// 应用排序
	orderClause := opts.SortBy + " " + opts.SortOrder
	query = query.Order(orderClause)

	// 应用分页
	offset := (opts.Page - 1) * opts.PageSize
	var records []models.ExecutionRecord
	if err := query.Offset(offset).Limit(opts.PageSize).Find(&records).Error; err != nil {
		return nil, fmt.Errorf("查询历史记录列表失败: %v", err)
	}

	// 计算总页数
	totalPages := int(total) / opts.PageSize
	if int(total)%opts.PageSize > 0 {
		totalPages++
	}

	return &ExecutionQueryResult{
		Data:       records,
		Total:      total,
		Page:       opts.Page,
		PageSize:   opts.PageSize,
		TotalPages: totalPages,
	}, nil
}

// DeleteExecutionRecord 删除单条历史执行记录
func DeleteExecutionRecord(id string) error {
	if err := DB.Delete(&models.ExecutionRecord{}, "id = ?", id).Error; err != nil {
		return fmt.Errorf("删除历史执行记录失败: %v", err)
	}
	logger.Info("ExecutionRecord", "-", "删除历史执行记录: %s", id)
	return nil
}

// DeleteOldExecutionRecords 删除旧的历史执行记录（保留策略）
// keepCount: 保留的最新记录数量
func DeleteOldExecutionRecords(keepCount int) error {
	if keepCount <= 0 {
		keepCount = 100 // 默认保留100条
	}

	// 获取需要删除的记录ID
	var idsToDelete []string
	subQuery := DB.Model(&models.ExecutionRecord{}).
		Select("id").
		Order("started_at DESC").
		Offset(keepCount)

	if err := DB.Model(&models.ExecutionRecord{}).
		Where("id IN (?)", subQuery).
		Pluck("id", &idsToDelete).Error; err != nil {
		return fmt.Errorf("查询待删除记录失败: %v", err)
	}

	if len(idsToDelete) == 0 {
		return nil
	}

	// 批量删除
	if err := DB.Delete(&models.ExecutionRecord{}, "id IN ?", idsToDelete).Error; err != nil {
		return fmt.Errorf("批量删除历史记录失败: %v", err)
	}

	logger.Info("ExecutionRecord", "-", "清理旧历史记录 %d 条，保留最新 %d 条", len(idsToDelete), keepCount)
	return nil
}

// DeleteExecutionRecordsByTaskGroup 删除指定任务组的所有历史记录
func DeleteExecutionRecordsByTaskGroup(taskGroupID string) error {
	if err := DB.Delete(&models.ExecutionRecord{}, "task_group_id = ?", taskGroupID).Error; err != nil {
		return fmt.Errorf("删除任务组历史记录失败: %v", err)
	}
	logger.Info("ExecutionRecord", "-", "删除任务组历史记录: %s", taskGroupID)
	return nil
}

// GetExecutionRecordStats 获取执行记录统计信息
func GetExecutionRecordStats(taskGroupID string) (map[string]interface{}, error) {
	query := DB.Model(&models.ExecutionRecord{})
	if taskGroupID != "" {
		query = query.Where("task_group_id = ?", taskGroupID)
	}

	var total int64
	var completed int64
	var partial int64
	var failed int64
	var cancelled int64

	if err := query.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("统计总记录数失败: %v", err)
	}

	DB.Model(&models.ExecutionRecord{}).Where("status = ?", "completed").Count(&completed)
	DB.Model(&models.ExecutionRecord{}).Where("status = ?", "partial").Count(&partial)
	DB.Model(&models.ExecutionRecord{}).Where("status = ?", "failed").Count(&failed)
	DB.Model(&models.ExecutionRecord{}).Where("status = ?", "cancelled").Count(&cancelled)

	return map[string]interface{}{
		"total":     total,
		"completed": completed,
		"partial":   partial,
		"failed":    failed,
		"cancelled": cancelled,
	}, nil
}

// ExecutionRecordToJSON 将记录转换为 JSON 字符串（用于调试）
func ExecutionRecordToJSON(r *models.ExecutionRecord) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data)
}
