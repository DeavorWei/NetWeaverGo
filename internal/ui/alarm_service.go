package ui

import (
	"errors"
	"strings"

	"github.com/NetWeaverGo/core/internal/alarm"
	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/models"
)

// AlarmService 告警规则与归并分析前端服务
type AlarmService struct {
	merger *alarm.AlarmMerger
}

// NewAlarmService 创建告警服务实例
func NewAlarmService() *AlarmService {
	return &AlarmService{
		merger: alarm.NewAlarmMerger(),
	}
}

// ListAlarmRules 查询告警规则列表（支持按产品族过滤）
func (s *AlarmService) ListAlarmRules(family string) ([]models.AlarmRule, error) {
	registry := alarm.GetDefaultRegistry()
	compiled := registry.GetRules(family)
	res := make([]models.AlarmRule, 0, len(compiled))
	for _, c := range compiled {
		res = append(res, c.AlarmRule)
	}
	return res, nil
}

// ListAlarmRecords 查询指定运行任务或设备的原始告警记录
func (s *AlarmService) ListAlarmRecords(runID, deviceIP string, limit, offset int) ([]models.AlarmRecord, int64, error) {
	db := config.GetDB()
	if db == nil {
		return nil, 0, errors.New("数据库未初始化")
	}

	query := db.Model(&models.AlarmRecord{})
	if strings.TrimSpace(runID) != "" {
		query = query.Where("run_id = ?", strings.TrimSpace(runID))
	}
	if strings.TrimSpace(deviceIP) != "" {
		query = query.Where("device_ip = ?", strings.TrimSpace(deviceIP))
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	var list []models.AlarmRecord
	if err := query.Order("id desc").Limit(limit).Offset(offset).Find(&list).Error; err != nil {
		return nil, 0, err
	}

	return list, total, nil
}

// ListMergedPhenomena 查询指定任务的归并故障现象
func (s *AlarmService) ListMergedPhenomena(runID string) ([]models.MergedPhenomenon, error) {
	db := config.GetDB()
	if db == nil {
		return nil, errors.New("数据库未初始化")
	}

	var list []models.MergedPhenomenon
	query := db.Model(&models.MergedPhenomenon{})
	if strings.TrimSpace(runID) != "" {
		query = query.Where("run_id = ?", strings.TrimSpace(runID))
	}

	if err := query.Order("id desc").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// AnalyzeAndMerge 对指定运行任务的告警执行归并分析并持久化入库
func (s *AlarmService) AnalyzeAndMerge(runID string) ([]models.MergedPhenomenon, error) {
	db := config.GetDB()
	if db == nil {
		return nil, errors.New("数据库未初始化")
	}

	var records []models.AlarmRecord
	if err := db.Where("run_id = ?", strings.TrimSpace(runID)).Find(&records).Error; err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, nil
	}

	phenomena := s.merger.Merge(records)
	for i := range phenomena {
		if err := db.Create(&phenomena[i]).Error; err != nil {
			return nil, err
		}
		// 回填 AlarmRecord 的 MergedPhenomenonID
		if len(phenomena[i].RecordIDs) > 0 {
			_ = db.Model(&models.AlarmRecord{}).
				Where("id IN ?", phenomena[i].RecordIDs).
				Update("merged_phenomenon_id", phenomena[i].ID).Error
		}
	}

	return phenomena, nil
}
