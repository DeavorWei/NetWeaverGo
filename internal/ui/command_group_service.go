package ui

import (
	"context"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/models"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// CommandGroupService 命令组管理服务 - 负责命令组的增删改查和导入导出
type CommandGroupService struct {
	wailsApp *application.App
}

// NewCommandGroupService 创建命令组服务实例
func NewCommandGroupService() *CommandGroupService {
	return &CommandGroupService{}
}

// ServiceStartup Wails 服务启动生命周期钩子
func (s *CommandGroupService) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	s.wailsApp = application.Get()
	return nil
}

// ListCommandGroups 获取所有命令组列表
func (s *CommandGroupService) ListCommandGroups() ([]models.CommandGroup, error) {
	return config.ListCommandGroups()
}

// GetCommandGroup 根据 ID 获取单个命令组
func (s *CommandGroupService) GetCommandGroup(id uint) (*models.CommandGroup, error) {
	return config.GetCommandGroup(id)
}

// CreateCommandGroup 创建新命令组
func (s *CommandGroupService) CreateCommandGroup(group models.CommandGroup) (*models.CommandGroup, error) {
	return config.CreateCommandGroup(group)
}

// UpdateCommandGroup 更新命令组
func (s *CommandGroupService) UpdateCommandGroup(id uint, group models.CommandGroup) (*models.CommandGroup, error) {
	return config.UpdateCommandGroup(id, group)
}

// DeleteCommandGroup 删除命令组
func (s *CommandGroupService) DeleteCommandGroup(id uint) error {
	return config.DeleteCommandGroup(id)
}

// DuplicateCommandGroup 复制命令组
func (s *CommandGroupService) DuplicateCommandGroup(id uint) (*models.CommandGroup, error) {
	return config.DuplicateCommandGroup(id)
}

// ImportCommandGroup 从文件导入命令组
func (s *CommandGroupService) ImportCommandGroup(filePath string) (*models.CommandGroup, error) {
	return config.ImportCommandGroup(filePath)
}

// ExportCommandGroup 导出命令组到文件
func (s *CommandGroupService) ExportCommandGroup(id uint, filePath string) error {
	return config.ExportCommandGroup(id, filePath)
}

// GetCommands 获取命令列表（兼容旧接口）
func (s *CommandGroupService) GetCommands() ([]string, error) {
	return config.LoadDefaultCommands()
}

// SaveCommands 保存命令列表（兼容旧接口）
func (s *CommandGroupService) SaveCommands(commands []string) error {
	return config.SaveCommands(commands)
}
