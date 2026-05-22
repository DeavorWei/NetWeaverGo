// Package repository 提供数据访问层的抽象接口和实现
// mib_repository.go 实现 MIB 数据访问接口
package repository

import (
	"gorm.io/gorm"

	"github.com/NetWeaverGo/core/internal/models"
)

// GormMIBRepository MIB Repository 的 GORM 实现
type GormMIBRepository struct {
	db *gorm.DB // 使用 config.SNMPDB（SNMP 独立数据库）
}

// NewGormMIBRepository 创建 MIB Repository 实例
func NewGormMIBRepository(db *gorm.DB) MIBRepository {
	return &GormMIBRepository{db: db}
}

// ============================================================================
// 模块管理
// ============================================================================

func (r *GormMIBRepository) GetAllModules() ([]models.MIBModule, error) {
	var modules []models.MIBModule
	err := r.db.Order("created_at DESC").Find(&modules).Error
	return modules, err
}

func (r *GormMIBRepository) GetModuleByID(id uint) (*models.MIBModule, error) {
	var module models.MIBModule
	err := r.db.First(&module, id).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &module, nil
}

func (r *GormMIBRepository) GetModuleByName(name string) (*models.MIBModule, error) {
	var module models.MIBModule
	err := r.db.Where("name = ?", name).First(&module).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &module, nil
}

func (r *GormMIBRepository) SaveModule(module *models.MIBModule) error {
	return r.db.Save(module).Error
}

func (r *GormMIBRepository) DeleteModule(id uint) error {
	return r.db.Delete(&models.MIBModule{}, id).Error
}

// ============================================================================
// 节点管理
// ============================================================================

func (r *GormMIBRepository) GetNodeByID(id uint) (*models.MIBNode, error) {
	var node models.MIBNode
	err := r.db.First(&node, id).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &node, nil
}

func (r *GormMIBRepository) GetNodeByOID(oid string) (*models.MIBNode, error) {
	var node models.MIBNode
	err := r.db.Where("oid = ?", oid).First(&node).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &node, nil
}

func (r *GormMIBRepository) GetNodeByName(name string) (*models.MIBNode, error) {
	var node models.MIBNode
	err := r.db.Where("name = ?", name).First(&node).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &node, nil
}

func (r *GormMIBRepository) GetNodesByModule(moduleID uint) ([]models.MIBNode, error) {
	var nodes []models.MIBNode
	err := r.db.Where("module_id = ?", moduleID).Order("oid ASC").Find(&nodes).Error
	return nodes, err
}

func (r *GormMIBRepository) GetChildNodes(parentOID string) ([]models.MIBNode, error) {
	var nodes []models.MIBNode
	err := r.db.Where("parent_oid = ?", parentOID).Order("oid ASC").Find(&nodes).Error
	return nodes, err
}

func (r *GormMIBRepository) CountChildNodes(parentOID string) (int64, error) {
	var count int64
	err := r.db.Model(&models.MIBNode{}).Where("parent_oid = ?", parentOID).Count(&count).Error
	return count, err
}

func (r *GormMIBRepository) SaveNode(node *models.MIBNode) error {
	return r.db.Save(node).Error
}

func (r *GormMIBRepository) SaveNodes(nodes []models.MIBNode) error {
	if len(nodes) == 0 {
		return nil
	}
	return r.db.CreateInBatches(nodes, 100).Error
}

func (r *GormMIBRepository) DeleteNode(id uint) error {
	return r.db.Delete(&models.MIBNode{}, id).Error
}

func (r *GormMIBRepository) DeleteNodesByModule(moduleID uint) error {
	return r.db.Where("module_id = ?", moduleID).Delete(&models.MIBNode{}).Error
}

func (r *GormMIBRepository) SearchNodes(query string) ([]models.MIBNode, error) {
	var nodes []models.MIBNode
	err := r.db.Where("name LIKE ? OR oid LIKE ?", "%"+query+"%", "%"+query+"%").
		Order("oid ASC").
		Limit(100).
		Find(&nodes).Error
	return nodes, err
}

func (r *GormMIBRepository) GetAllNodes() ([]models.MIBNode, error) {
	var nodes []models.MIBNode
	err := r.db.Order("oid ASC").Find(&nodes).Error
	return nodes, err
}