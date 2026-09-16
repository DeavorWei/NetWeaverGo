package ui

import (
	"errors"
	"strings"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/executor"
	"github.com/NetWeaverGo/core/internal/models"
)

// RiskCommandService 高危命令前端服务
type RiskCommandService struct{}

// NewRiskCommandService 创建高危命令服务实例
func NewRiskCommandService() *RiskCommandService {
	return &RiskCommandService{}
}

// ListRiskLogs 查询高危命令执行/拦截留痕日志
func (s *RiskCommandService) ListRiskLogs(runID, deviceIP string, limit, offset int) ([]models.RiskCommandLog, int64, error) {
	db := config.GetDB()
	if db == nil {
		return nil, 0, errors.New("数据库未初始化")
	}

	query := db.Model(&models.RiskCommandLog{})
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
	if offset < 0 {
		offset = 0
	}

	var logs []models.RiskCommandLog
	if err := query.Order("id desc").Limit(limit).Offset(offset).Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}

// ListTrustEntries 获取当前信任清单条目
func (s *RiskCommandService) ListTrustEntries() ([]models.RiskTrustEntry, error) {
	db := config.GetDB()
	if db == nil {
		return nil, errors.New("数据库未初始化")
	}

	var entries []models.RiskTrustEntry
	if err := db.Order("id desc").Find(&entries).Error; err != nil {
		return nil, err
	}
	return entries, nil
}

// AddTrustEntry 新增信任清单条目并热重载到运行时
func (s *RiskCommandService) AddTrustEntry(entry models.RiskTrustEntry) error {
	db := config.GetDB()
	if db == nil {
		return errors.New("数据库未初始化")
	}

	if strings.TrimSpace(entry.Pattern) == "" {
		return errors.New("信任规则正则不能为空")
	}

	if err := db.Create(&entry).Error; err != nil {
		return err
	}

	// 热重载到校验器
	var all []models.RiskTrustEntry
	_ = db.Find(&all).Error
	executor.GetGlobalRiskValidator().ReloadTrustEntries(all)

	return nil
}

// DeleteTrustEntry 删除信任清单条目并热重载
func (s *RiskCommandService) DeleteTrustEntry(id uint) error {
	db := config.GetDB()
	if db == nil {
		return errors.New("数据库未初始化")
	}

	if err := db.Delete(&models.RiskTrustEntry{}, id).Error; err != nil {
		return err
	}

	var all []models.RiskTrustEntry
	_ = db.Find(&all).Error
	executor.GetGlobalRiskValidator().ReloadTrustEntries(all)

	return nil
}

// BypassRiskCommand 紧急放行高危命令（校验二次确认与理由，记录留痕）
func (s *RiskCommandService) BypassRiskCommand(req executor.BypassRequest) error {
	if err := executor.ValidateBypass(req); err != nil {
		return err
	}

	executor.RecordRiskLog(models.RiskCommandLog{
		RunID:    req.RunID,
		DeviceIP: req.DeviceIP,
		Command:  req.Command,
		RuleID:   req.RuleID,
		Action:   "bypassed",
		Operator: req.Operator,
		Reason:   req.Reason,
	})

	return nil
}
