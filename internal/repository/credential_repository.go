// Package repository 提供数据访问层的抽象接口和实现
// credential_repository.go 实现 SNMP 查询凭据的数据访问
package repository

import (
	"context"

	"gorm.io/gorm"

	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
)

// GormCredentialRepository 凭据仓库的 GORM 实现
type GormCredentialRepository struct {
	db *gorm.DB // 使用 config.SNMPDB（SNMP 独立数据库）
}

// NewGormCredentialRepository 创建凭据仓库实例
func NewGormCredentialRepository(db *gorm.DB) CredentialRepository {
	return &GormCredentialRepository{db: db}
}

// CreateCredential 创建 SNMP 凭据
func (r *GormCredentialRepository) CreateCredential(ctx context.Context, cred *models.SNMPCredential) error {
	if err := r.db.WithContext(ctx).Create(cred).Error; err != nil {
		logger.Error("SNMP-Repo", "-", "创建凭据失败: %v", err)
		return err
	}
	logger.Info("SNMP-Repo", "-", "凭据已创建: ID=%d, Name=%s", cred.ID, cred.Name)
	return nil
}

// GetCredential 获取单个 SNMP 凭据，不存在时返回 nil, nil
func (r *GormCredentialRepository) GetCredential(ctx context.Context, id uint) (*models.SNMPCredential, error) {
	var cred models.SNMPCredential
	err := r.db.WithContext(ctx).First(&cred, id).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		logger.Error("SNMP-Repo", "-", "获取凭据失败: ID=%d, %v", id, err)
		return nil, err
	}
	return &cred, nil
}

// GetCredentialByName 按名称获取 SNMP 凭据，不存在时返回 nil, nil
func (r *GormCredentialRepository) GetCredentialByName(ctx context.Context, name string) (*models.SNMPCredential, error) {
	var cred models.SNMPCredential
	err := r.db.WithContext(ctx).Where("name = ?", name).First(&cred).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		logger.Error("SNMP-Repo", "-", "获取凭据失败: Name=%s, %v", name, err)
		return nil, err
	}
	return &cred, nil
}

// ListCredentials 获取所有 SNMP 凭据
func (r *GormCredentialRepository) ListCredentials(ctx context.Context) ([]*models.SNMPCredential, error) {
	var creds []*models.SNMPCredential
	if err := r.db.WithContext(ctx).Order("name ASC").Find(&creds).Error; err != nil {
		logger.Error("SNMP-Repo", "-", "获取凭据列表失败: %v", err)
		return nil, err
	}
	return creds, nil
}

// UpdateCredential 更新 SNMP 凭据
func (r *GormCredentialRepository) UpdateCredential(ctx context.Context, cred *models.SNMPCredential) error {
	if err := r.db.WithContext(ctx).Save(cred).Error; err != nil {
		logger.Error("SNMP-Repo", "-", "更新凭据失败: ID=%d, %v", cred.ID, err)
		return err
	}
	logger.Info("SNMP-Repo", "-", "凭据已更新: ID=%d, Name=%s", cred.ID, cred.Name)
	return nil
}

// DeleteCredential 删除 SNMP 凭据
func (r *GormCredentialRepository) DeleteCredential(ctx context.Context, id uint) error {
	if err := r.db.WithContext(ctx).Delete(&models.SNMPCredential{}, id).Error; err != nil {
		logger.Error("SNMP-Repo", "-", "删除凭据失败: ID=%d, %v", id, err)
		return err
	}
	logger.Info("SNMP-Repo", "-", "凭据已删除: ID=%d", id)
	return nil
}
