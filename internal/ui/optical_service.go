package ui

import (
	"errors"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/models"
	"github.com/NetWeaverGo/core/internal/optical"
)

// OpticalService 弱光检测与光模块健康服务
type OpticalService struct {
	checker *optical.Checker
}

// NewOpticalService 创建光模块检测服务实例
func NewOpticalService() *OpticalService {
	return &OpticalService{
		checker: optical.NewChecker(),
	}
}

// ListRules 获取光模块检测规则列表
func (s *OpticalService) ListRules() ([]models.OpticalCheckRule, error) {
	db := config.GetDB()
	if db == nil {
		return nil, errors.New("数据库未初始化")
	}
	var rules []models.OpticalCheckRule
	if err := db.Find(&rules).Error; err != nil {
		return nil, err
	}
	return rules, nil
}

// SaveRule 新建或更新光模块检测规则
func (s *OpticalService) SaveRule(rule models.OpticalCheckRule) error {
	db := config.GetDB()
	if db == nil {
		return errors.New("数据库未初始化")
	}
	if rule.ID > 0 {
		return db.Save(&rule).Error
	}
	return db.Create(&rule).Error
}

// DeleteRule 删除光模块检测规则
func (s *OpticalService) DeleteRule(id uint) error {
	db := config.GetDB()
	if db == nil {
		return errors.New("数据库未初始化")
	}
	return db.Delete(&models.OpticalCheckRule{}, id).Error
}

// CheckTransceiver 单端口光模块健康检查
func (s *OpticalService) CheckTransceiver(metric optical.TransceiverMetric) (*optical.CheckResult, error) {
	// 若 DB 中有规则，同步更新 checker
	if db := config.GetDB(); db != nil {
		var dbRules []models.OpticalCheckRule
		if err := db.Find(&dbRules).Error; err == nil && len(dbRules) > 0 {
			s.checker.SetRules(dbRules)
		}
	}
	res := s.checker.Check(metric)
	return &res, nil
}

// BatchCheck 批量端口光模块健康检查
func (s *OpticalService) BatchCheck(metrics []optical.TransceiverMetric) ([]optical.CheckResult, error) {
	if db := config.GetDB(); db != nil {
		var dbRules []models.OpticalCheckRule
		if err := db.Find(&dbRules).Error; err == nil && len(dbRules) > 0 {
			s.checker.SetRules(dbRules)
		}
	}
	return s.checker.BatchCheck(metrics), nil
}
