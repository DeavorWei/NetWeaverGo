// Package repository 提供数据访问层的抽象接口和实现
// mib_repository.go 实现 MIB 数据访问接口
package repository

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

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
	return r.db.Transaction(func(tx *gorm.DB) error {
		// 级联删除该模块下的所有节点
		if err := tx.Where("module_id = ?", id).Delete(&models.MIBNode{}).Error; err != nil {
			return err
		}
		return tx.Delete(&models.MIBModule{}, id).Error
	})
}

// ============================================================================
// 文件夹管理
// ============================================================================

func (r *GormMIBRepository) GetAllFolders() ([]models.MIBFolder, error) {
	var folders []models.MIBFolder
	err := r.db.Order("name ASC").Find(&folders).Error
	return folders, err
}

func (r *GormMIBRepository) GetFolderByID(id uint) (*models.MIBFolder, error) {
	var folder models.MIBFolder
	err := r.db.First(&folder, id).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &folder, nil
}

func (r *GormMIBRepository) GetFolderByName(name string) (*models.MIBFolder, error) {
	var folder models.MIBFolder
	err := r.db.Where("name = ?", name).First(&folder).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &folder, nil
}

func (r *GormMIBRepository) SaveFolder(folder *models.MIBFolder) error {
	return r.db.Save(folder).Error
}

func (r *GormMIBRepository) DeleteFolder(id uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// 1. 查询该文件夹下的所有模块
		var modules []models.MIBModule
		if err := tx.Where("folder_id = ?", id).Find(&modules).Error; err != nil {
			return err
		}
		
		// 2. 依次删除所有模块及节点
		for _, m := range modules {
			if err := tx.Where("module_id = ?", m.ID).Delete(&models.MIBNode{}).Error; err != nil {
				return err
			}
			if err := tx.Delete(&models.MIBModule{}, m.ID).Error; err != nil {
				return err
			}
		}
		
		// 3. 删除文件夹自身
		return tx.Delete(&models.MIBFolder{}, id).Error
	})
}

func (r *GormMIBRepository) MoveModuleToFolder(moduleID uint, folderID *uint) error {
	return r.db.Model(&models.MIBModule{}).Where("id = ?", moduleID).Update("folder_id", folderID).Error
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

// GetNodesByOIDs 批量查询 OID 对应的节点
func (r *GormMIBRepository) GetNodesByOIDs(oids []string) ([]models.MIBNode, error) {
	if len(oids) == 0 {
		return []models.MIBNode{}, nil
	}
	var nodes []models.MIBNode
	err := r.db.Where("oid IN ?", oids).Find(&nodes).Error
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
	// UPSERT 策略：ON CONFLICT(oid) 时更新节点属性，但不更新 module_id
	// 避免模块归属漂移问题——同一 OID 被不同模块重复导入时，保留首次归属
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "oid"}},
		DoUpdates: clause.AssignmentColumns([]string{"name", "parent_oid", "node_type", "syntax", "access", "status", "description", "source", "updated_at"}),
	}).CreateInBatches(nodes, 100).Error
}

func (r *GormMIBRepository) DeleteNode(id uint) error {
	return r.db.Delete(&models.MIBNode{}, id).Error
}

func (r *GormMIBRepository) DeleteNodesByModule(moduleID uint) error {
	return r.db.Where("module_id = ?", moduleID).Delete(&models.MIBNode{}).Error
}

func (r *GormMIBRepository) SearchNodes(query string) ([]models.MIBNode, error) {
	var nodes []models.MIBNode
	err := r.db.Joins("LEFT JOIN mib_modules ON mib_nodes.module_id = mib_modules.id").
		Where("mib_nodes.name LIKE ? OR mib_nodes.oid LIKE ? OR mib_nodes.description LIKE ? OR mib_modules.name LIKE ?",
			"%"+query+"%", "%"+query+"%", "%"+query+"%", "%"+query+"%").
		Order("mib_nodes.oid ASC").
		Limit(100).
		Find(&nodes).Error
	return nodes, err
}

func (r *GormMIBRepository) SearchNodesInModule(moduleID uint, query string) ([]models.MIBNode, error) {
	var nodes []models.MIBNode
	err := r.db.Where("module_id = ? AND (name LIKE ? OR oid LIKE ? OR description LIKE ?)",
		moduleID, "%"+query+"%", "%"+query+"%", "%"+query+"%").
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

func (r *GormMIBRepository) GetNodesBatch(offset, limit int) ([]models.MIBNode, error) {
	var nodes []models.MIBNode
	err := r.db.Order("oid ASC").Offset(offset).Limit(limit).Find(&nodes).Error
	return nodes, err
}

// ============================================================================
// 批量查询与事务管理
// ============================================================================

func (r *GormMIBRepository) CountChildNodesBatch(parentOIDs []string) (map[string]int64, error) {
	result := make(map[string]int64)
	if len(parentOIDs) == 0 {
		return result, nil
	}

	type Result struct {
		ParentOID string
		Count     int64
	}
	var counts []Result

	err := r.db.Model(&models.MIBNode{}).
		Select("parent_oid as parent_oid, count(id) as count").
		Where("parent_oid IN ?", parentOIDs).
		Group("parent_oid").
		Scan(&counts).Error

	if err != nil {
		return nil, err
	}

	for _, c := range counts {
		result[c.ParentOID] = c.Count
	}
	return result, nil
}

func (r *GormMIBRepository) WithTx(tx *gorm.DB) MIBRepository {
	if tx == nil {
		return r
	}
	return &GormMIBRepository{db: tx}
}

func (r *GormMIBRepository) BeginTx() *gorm.DB {
	return r.db.Begin()
}