// Package ui 提供 Wails 暴露层服务
// snmp_mib_service.go 实现 MIB 管理 Wails 绑定服务
package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
	"github.com/NetWeaverGo/core/internal/repository"
	"github.com/NetWeaverGo/core/internal/snmp"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// ============================================================================
// View Models（视图模型）
// ============================================================================

// MIBModuleVM MIB 模块视图模型
type MIBModuleVM struct {
	ID          uint      `json:"id"`
	FolderID    *uint     `json:"folderId"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Version     string    `json:"version"`
	NodeCount   int       `json:"nodeCount"`
	ImportedAt  time.Time `json:"importedAt"`
	Status      string    `json:"status"` // active, error, partial
	IsBuiltIn   bool      `json:"isBuiltIn"`
}

// MIBNodeVM MIB 节点视图模型
type MIBNodeVM struct {
	ID          uint   `json:"id"`
	ModuleID    uint   `json:"moduleId"`
	ModuleName  string `json:"moduleName"`
	OID         string `json:"oid"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Access      string `json:"access"`
	Status      string `json:"status"`
	ParentID    *uint  `json:"parentId"`
	ChildrenIDs []uint `json:"childrenIds"`
}

// ImportMIBRequest 导入 MIB 请求
type ImportMIBRequest struct {
	FilePath      string `json:"filePath"`
	ModuleName    string `json:"moduleName,omitempty"`
	PartialImport bool   `json:"partialImport"` // 是否允许部分导入
	FolderID      *uint  `json:"folderId,omitempty"`
}

// ImportMIBFilesRequest 批量导入 MIB 文件请求
type ImportMIBFilesRequest struct {
	FilePaths         []string `json:"filePaths"`           // 文件路径列表
	FolderID          *uint    `json:"folderId,omitempty"`  // 目标文件夹 ID
	Concurrency       int      `json:"concurrency"`         // 并发度（1-8）
	SkipErrors        bool     `json:"skipErrors"`          // 是否跳过错误继续��入
	OverwriteExisting bool     `json:"overwriteExisting"`   // 是否覆盖已存在的模块
	DependencyDirs    []string `json:"dependencyDirs"`      // 依赖目录列表
}

// CreateMIBNodeRequest 创建 MIB 节点请求
type CreateMIBNodeRequest struct {
	ModuleID    uint   `json:"moduleId"`
	OID         string `json:"oid"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Access      string `json:"access"`
	Status      string `json:"status"`
	ParentID    *uint  `json:"parentId"`
}

// UpdateMIBNodeRequest 更新 MIB 节点请求
type UpdateMIBNodeRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Access      string `json:"access"`
	Status      string `json:"status"`
}

// ResolvedOIDVM 解析结果视图模型
type ResolvedOIDVM struct {
	OID         string `json:"oid"`
	Name        string `json:"name"`
	ModuleName  string `json:"moduleName"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Access      string `json:"access"`
	Status      string `json:"status"`
	Found       bool   `json:"found"`
}

// ============================================================================
// SNMPMIBService 实现
// ============================================================================

// SNMPMIBService MIB 管理服务 (Wails 绑定)
// 暴露 MIB 管理功能给前端，包括导入/删除/查询/解析等操作
type SNMPMIBService struct {
	wailsApp      *application.App // Wails 应用实例（由 ServiceStartup 设置）
	mibManager    *snmp.MIBManager
	oidResolver   *snmp.OIDResolver
	repo          repository.MIBRepository
	eventNotifier *SNMPEventNotifier
}

// NewSNMPMIBService 创建 MIB 服务实例
func NewSNMPMIBService(
	mibManager *snmp.MIBManager,
	oidResolver *snmp.OIDResolver,
	repo repository.MIBRepository,
	notifier *SNMPEventNotifier,
) *SNMPMIBService {
	service := &SNMPMIBService{
		mibManager:    mibManager,
		oidResolver:   oidResolver,
		repo:          repo,
		eventNotifier: notifier,
	}

	logger.Info("SNMP", "-", "MIB 服务已初始化")

	return service
}

// ServiceStartup Wails 服务启动生命周期钩子
func (s *SNMPMIBService) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	s.wailsApp = application.Get()
	logger.Info("SNMP", "-", "SNMPMIBService 服务已启动")
	return nil
}

// ============================================================================
// MIB 模块管理
// ============================================================================

// GetMIBModules 获取所有 MIB 模块列表
func (s *SNMPMIBService) GetMIBModules(ctx context.Context) ([]MIBModuleVM, error) {
	modules, err := s.repo.GetAllModules()
	if err != nil {
		logger.Error("SNMP", "-", "获取 MIB 模块列表失败: %v", err)
		return nil, fmt.Errorf("获取 MIB 模块列表失败: %v", err)
	}

	vms := make([]MIBModuleVM, 0, len(modules))
	for _, m := range modules {
		vms = append(vms, MIBModuleVM{
			ID:          m.ID,
			FolderID:    m.FolderID,
			Name:        m.Name,
			Description: m.Description,
			Version:     "", // 从描述中提取版本信息（如果有）
			NodeCount:   m.NodeCount,
			ImportedAt:  m.CreatedAt,
			Status:      "active",
			IsBuiltIn:   m.IsBuiltIn,
		})
	}

	logger.Debug("SNMP", "-", "获取 MIB 模块列表: %d 个", len(vms))

	return vms, nil
}

// GetMIBModule 获取单个 MIB 模块详情
func (s *SNMPMIBService) GetMIBModule(ctx context.Context, moduleID uint) (*MIBModuleVM, error) {
	module, err := s.repo.GetModuleByID(moduleID)
	if err != nil {
		return nil, fmt.Errorf("查询模块失败: %v", err)
	}

	if module == nil {
		return nil, fmt.Errorf("模块不存在: ID %d", moduleID)
	}

	vm := &MIBModuleVM{
		ID:          module.ID,
		FolderID:    module.FolderID,
		Name:        module.Name,
		Description: module.Description,
		NodeCount:   module.NodeCount,
		ImportedAt:  module.CreatedAt,
		Status:      "active",
		IsBuiltIn:   module.IsBuiltIn,
	}

	return vm, nil
}

// ImportMIB 导入 MIB 文件
func (s *SNMPMIBService) ImportMIB(ctx context.Context, req ImportMIBRequest) error {
	// 发送导入开始事件
	s.eventNotifier.NotifyMIBImportProgress(snmp.MIBImportProgress{
		FileName: req.FilePath,
		Phase:    "parsing",
		Progress: 0,
	})

	// 执行导入
	result, err := s.mibManager.ImportMIBFile(ctx, req.FilePath, req.FolderID)
	if err != nil {
		// 发送错误事件
		s.eventNotifier.NotifyMIBImportProgress(snmp.MIBImportProgress{
			FileName: req.FilePath,
			Phase:    "error",
			Error:    err.Error(),
		})
		return fmt.Errorf("导入 MIB 文件失败: %v", err)
	}

	// 发送保存进度
	s.eventNotifier.NotifyMIBImportProgress(snmp.MIBImportProgress{
		FileName:   req.FilePath,
		ModuleName: result.Module.Name,
		Phase:      "saving",
		Progress:   50,
		NodesTotal: result.NodeCount,
	})

	// 发送缓存进度
	s.eventNotifier.NotifyMIBImportProgress(snmp.MIBImportProgress{
		FileName:   req.FilePath,
		ModuleName: result.Module.Name,
		Phase:      "caching",
		Progress:   80,
		NodesDone:  result.NodeCount,
		NodesTotal: result.NodeCount,
	})

	// 发送完成事件
	s.eventNotifier.NotifyMIBImportProgress(snmp.MIBImportProgress{
		FileName:   req.FilePath,
		ModuleName: result.Module.Name,
		Phase:      "completed",
		Progress:   100,
		NodesDone:  result.NodeCount,
		NodesTotal: result.NodeCount,
	})

	// 发送模块导入完成事件
	s.eventNotifier.NotifyMIBImported(result.Module)

	logger.Info("SNMP", "-", "MIB 模块导入成功: %s (%d 节点, %d 错误)",
		result.Module.Name, result.NodeCount, result.ErrorCount)

	// 清除 OID 解析器缓存，确保新导入的节点和状态能被立即查到
	s.oidResolver.ClearCache()

	return nil
}

// ImportMIBFiles 批量导入 MIB 文件（简化版本）
// 使用默认配置调用批量导入方法
func (s *SNMPMIBService) ImportMIBFiles(ctx context.Context, filePaths []string, folderID *uint) error {
	if len(filePaths) == 0 {
		return nil
	}

	// 调试日志：追踪 folderID 传递
	if folderID != nil {
		logger.Debug("SNMP", "-", "ImportMIBFiles: 接收到 folderID=%d", *folderID)
	} else {
		logger.Debug("SNMP", "-", "ImportMIBFiles: 接收到 folderID=nil")
	}

	// 使用默认配置调用批量导入
	opts := snmp.MIBBatchImportOptions{
		Concurrency:       4, // 默认并发度
		SkipErrors:        true,
		OverwriteExisting: true,
		DependencyDirs:    []string{s.mibManager.GetMIBStoreDir()},
		FolderID:          folderID, // 传递目标文件夹 ID
	}

	result, err := s.mibManager.ImportMIBFilesBatch(ctx, filePaths, opts, s.eventNotifier)
	if err != nil {
		return err
	}

	// 清除 OID 解析器缓存，确保新导入的节点能被立即查到
	s.oidResolver.ClearCache()

	logger.Info("SNMP", "-", "批量导入完成: 成功 %d, 失败 %d, 跳过 %d, 总耗时 %dms",
		result.SuccessCount, result.FailedCount, result.SkippedCount, result.TotalDuration)

	// [M6] 在错误信息中包含失败文件列表摘要
	if result.FailedCount > 0 {
		var failedFiles []string
		for _, e := range result.Errors {
			failedFiles = append(failedFiles, e.FileName)
		}
		if len(failedFiles) > 5 {
			failedFiles = append(failedFiles[:5], fmt.Sprintf("... 等 %d 个文件", len(failedFiles)-5))
		}
		return fmt.Errorf("部分文件导入失败: 成功 %d, 失败 %d [%s]", result.SuccessCount, result.FailedCount, strings.Join(failedFiles, ", "))
	}

	return nil
}

// ImportMIBFilesWithOptions 批量导入 MIB 文件（完整版本）
// 支持自定义并发度、错误处理策略等选项
func (s *SNMPMIBService) ImportMIBFilesWithOptions(ctx context.Context, req ImportMIBFilesRequest) (*snmp.MIBBatchImportResult, error) {
	if len(req.FilePaths) == 0 {
		return &snmp.MIBBatchImportResult{
			TotalFiles: 0,
			Results:    []snmp.FileImportResult{},
			Errors:     []snmp.FileImportError{},
		}, nil
	}

	// 构建批量导入选项
	opts := snmp.MIBBatchImportOptions{
		Concurrency:       req.Concurrency,
		SkipErrors:        req.SkipErrors,
		OverwriteExisting: req.OverwriteExisting,
		DependencyDirs:    req.DependencyDirs,
		FolderID:          req.FolderID, // 传递目标文件夹 ID
	}

	// 如果未指定并发度，使用默认值
	if opts.Concurrency <= 0 {
		opts.Concurrency = 4
	}
	// [S4] 添加并发度范围验证
	if opts.Concurrency > 16 {
		opts.Concurrency = 16
	}

	// 如果未指定依赖目录，添加默认目录
	if len(opts.DependencyDirs) == 0 {
		opts.DependencyDirs = []string{s.mibManager.GetMIBStoreDir()}
	}

	// 调用 MIBManager 的批量导入方法
	result, err := s.mibManager.ImportMIBFilesBatch(ctx, req.FilePaths, opts, s.eventNotifier)
	if err != nil {
		return nil, err
	}

	// 清除 OID 解析器缓存，确保新导入的节点能被立即查到
	s.oidResolver.ClearCache()

	logger.Info("SNMP", "-", "批量导入完成: 成功 %d, 失败 %d, 跳过 %d, 总耗时 %dms",
		result.SuccessCount, result.FailedCount, result.SkippedCount, result.TotalDuration)

	return result, nil
}

// DeleteMIBModule 删除 MIB 模块
func (s *SNMPMIBService) DeleteMIBModule(ctx context.Context, moduleID uint) error {
	// 获取模块信息（用于日志）
	module, err := s.repo.GetModuleByID(moduleID)
	if err != nil {
		return fmt.Errorf("查询模块失败: %v", err)
	}

	if module == nil {
		return fmt.Errorf("模块不存在: ID %d", moduleID)
	}

	if module.IsBuiltIn {
		return fmt.Errorf("内置核心库模块禁止删除")
	}

	moduleName := module.Name

	// 执行删除
	if err := s.mibManager.DeleteModule(moduleID); err != nil {
		logger.Error("SNMP", "-", "删除 MIB 模块失败: %s, %v", moduleName, err)
		return fmt.Errorf("删除模块失败: %v", err)
	}

	// 发送删除事件
	s.eventNotifier.NotifyMIBDeleted(moduleID)

	logger.Info("SNMP", "-", "MIB 模块已删除: %s", moduleName)

	// 清除 OID 解析器缓存，确保被删除的节点不会在缓存中残留
	s.oidResolver.ClearCache()

	return nil
}

// ============================================================================
// MIB 文件夹管理
// ============================================================================

// MIBFolderVM MIB 文件夹视图模型
type MIBFolderVM struct {
	ID        uint      `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// GetMIBFolders 获取所有 MIB 文件夹列表
func (s *SNMPMIBService) GetMIBFolders(ctx context.Context) ([]MIBFolderVM, error) {
	folders, err := s.repo.GetAllFolders()
	if err != nil {
		logger.Error("SNMP", "-", "获取 MIB 文件夹列表失败: %v", err)
		return nil, fmt.Errorf("获取 MIB 文件夹列表失败: %v", err)
	}

	vms := make([]MIBFolderVM, 0, len(folders))
	for _, f := range folders {
		vms = append(vms, MIBFolderVM{
			ID:        f.ID,
			Name:      f.Name,
			CreatedAt: f.CreatedAt,
			UpdatedAt: f.UpdatedAt,
		})
	}

	logger.Debug("SNMP", "-", "获取 MIB 文件夹列表: %d 个", len(vms))
	return vms, nil
}

// CreateMIBFolder 创建 MIB 文件夹
func (s *SNMPMIBService) CreateMIBFolder(ctx context.Context, name string) (uint, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, fmt.Errorf("文件夹名称不能为空")
	}

	// 检查重名
	existing, err := s.repo.GetFolderByName(name)
	if err != nil {
		return 0, fmt.Errorf("检查文件夹重名失败: %v", err)
	}
	if existing != nil {
		return 0, fmt.Errorf("已存在同名文件夹: %s", name)
	}

	folder := &models.MIBFolder{
		Name: name,
	}
	if err := s.repo.SaveFolder(folder); err != nil {
		logger.Error("SNMP", "-", "创建 MIB 文件夹失败: %v", err)
		return 0, fmt.Errorf("创建 MIB 文件夹失败: %v", err)
	}

	logger.Info("SNMP", "-", "MIB 文件夹已创建: ID=%d, 名称=%s", folder.ID, folder.Name)
	return folder.ID, nil
}

// RenameMIBFolder 重命名 MIB 文件夹
func (s *SNMPMIBService) RenameMIBFolder(ctx context.Context, id uint, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("文件夹名称不能为空")
	}

	folder, err := s.repo.GetFolderByID(id)
	if err != nil {
		return fmt.Errorf("查询文件夹失败: %v", err)
	}
	if folder == nil {
		return fmt.Errorf("文件夹不存在: ID %d", id)
	}

	if folder.Name == "内置核心库" {
		return fmt.Errorf("内置核心库文件夹禁止重命名")
	}

	if folder.Name == name {
		return nil
	}

	// 检查重名
	existing, err := s.repo.GetFolderByName(name)
	if err != nil {
		return fmt.Errorf("检查文件夹重名失败: %v", err)
	}
	if existing != nil {
		return fmt.Errorf("已存在同名文件夹: %s", name)
	}

	folder.Name = name
	if err := s.repo.SaveFolder(folder); err != nil {
		logger.Error("SNMP", "-", "重命名 MIB 文件夹失败: ID=%d, %v", id, err)
		return fmt.Errorf("重命名 MIB 文件夹失败: %v", err)
	}

	logger.Info("SNMP", "-", "MIB 文件夹已重命名: ID=%d, 新名称=%s", id, name)
	return nil
}

// DeleteMIBFolder 删除 MIB 文件夹
func (s *SNMPMIBService) DeleteMIBFolder(ctx context.Context, id uint) error {
	folder, err := s.repo.GetFolderByID(id)
	if err != nil {
		return fmt.Errorf("查询文件夹失败: %v", err)
	}
	if folder == nil {
		return fmt.Errorf("文件夹不存在: ID %d", id)
	}

	if folder.Name == "内置核心库" {
		return fmt.Errorf("内置核心库文件夹禁止删除")
	}

	// 1. 获取文件夹下的所有模块并调用 mibManager 删除 (这会删除物理文件和缓存)
	modules, err := s.repo.GetModulesByFolder(id)
	if err == nil {
		for _, m := range modules {
			_ = s.mibManager.DeleteModule(m.ID)
		}
	} else {
		logger.Warn("SNMP", "-", "获取待删除文件夹中的模块失败: %v", err)
	}

	// 2. 删除物理文件夹目录
	folderDir := filepath.Join(s.mibManager.GetMIBStoreDir(), folder.Name)
	_ = os.RemoveAll(folderDir)

	// 3. 删除数据库记录
	if err := s.repo.DeleteFolder(id); err != nil {
		logger.Error("SNMP", "-", "删除 MIB 文件夹失败: ID=%d, %v", id, err)
		return fmt.Errorf("删除 MIB 文件夹失败: %v", err)
	}

	// 重建缓存以释放已删除节点
	_ = s.oidResolver.RebuildCache()

	logger.Info("SNMP", "-", "MIB 文件夹已删除: ID=%d, 名称=%s", id, folder.Name)
	return nil
}

// MoveMIBModuleToFolder 移动 MIB 模块到文件夹
func (s *SNMPMIBService) MoveMIBModuleToFolder(ctx context.Context, moduleID uint, folderID *uint) error {
	module, err := s.repo.GetModuleByID(moduleID)
	if err != nil {
		return fmt.Errorf("查询模块失败: %v", err)
	}
	if module == nil {
		return fmt.Errorf("模块不存在: ID %d", moduleID)
	}

	if module.IsBuiltIn {
		return fmt.Errorf("内置核心库模块禁止移动")
	}

	if folderID != nil {
		folder, err := s.repo.GetFolderByID(*folderID)
		if err != nil {
			return fmt.Errorf("查询目标文件夹失败: %v", err)
		}
		if folder == nil {
			return fmt.Errorf("目标文件夹不存在: ID %d", *folderID)
		}
	}

	if err := s.repo.MoveModuleToFolder(moduleID, folderID); err != nil {
		logger.Error("SNMP", "-", "移动 MIB 模块失败: module=%d, folder=%v, %v", moduleID, folderID, err)
		return fmt.Errorf("移动 MIB 模块失败: %v", err)
	}

	logger.Info("SNMP", "-", "MIB 模块已移动: module=%d, folder=%v", moduleID, folderID)
	return nil
}

// ============================================================================
// MIB 节点管理
// ============================================================================

// GetMIBTree 获取 MIB 树形结构
func (s *SNMPMIBService) GetMIBTree(ctx context.Context, moduleID uint) ([]snmp.MIBTreeNode, error) {
	// 获取模块信息
	module, err := s.repo.GetModuleByID(moduleID)
	if err != nil {
		return nil, fmt.Errorf("查询模块失败: %v", err)
	}

	if module == nil {
		return nil, fmt.Errorf("模块不存在: ID %d", moduleID)
	}

	// 获取模块下的所有节点
	nodes, err := s.repo.GetNodesByModule(moduleID)
	if err != nil {
		return nil, fmt.Errorf("查询节点失败: %v", err)
	}

	// 构建树形结构
	// 使用 map 存储所有临时节点的指针
	type tempNode struct {
		ID          uint
		OID         string
		Name        string
		NodeType    string
		Syntax      string
		Access      string
		Status      string
		Description string
		Children    []*tempNode
	}

	nodeMap := make(map[string]*tempNode)

	// 第一遍：创建所有模块内的节点
	for i := range nodes {
		node := &nodes[i]
		nodeMap[node.OID] = &tempNode{
			ID:          node.ID,
			OID:         node.OID,
			Name:        node.Name,
			NodeType:    node.NodeType,
			Syntax:      node.Syntax,
			Access:      node.Access,
			Status:      node.Status,
			Description: node.Description,
			Children:    []*tempNode{},
		}
	}

	// 第二遍：为所有缺失的父节点创建全局父节点
	for i := range nodes {
		node := &nodes[i]
		if node.ParentOID != "" {
			if _, ok := nodeMap[node.ParentOID]; !ok {
				// 父节点不在当前模块 of nodeMap 中，尝试从全局查询（包括虚拟节点）
				globalParent, err := s.repo.GetNodeByOID(node.ParentOID)
				if err == nil && globalParent != nil {
					// 将全局父节点加入 nodeMap
					nodeMap[node.ParentOID] = &tempNode{
						ID:          globalParent.ID,
						OID:         globalParent.OID,
						Name:        globalParent.Name,
						NodeType:    globalParent.NodeType,
						Syntax:      globalParent.Syntax,
						Access:      globalParent.Access,
						Status:      globalParent.Status,
						Description: globalParent.Description,
						Children:    []*tempNode{},
					}
					logger.Debug("SNMP", "-", "MIB 节点引用全局父节点: OID=%s, ParentOID=%s, Name=%s, ParentSource=%s",
						node.OID, node.ParentOID, node.Name, globalParent.Source)
				} else {
					logger.Warn("SNMP", "-", "MIB 节点父节点缺失: OID=%s, ParentOID=%s, Name=%s",
						node.OID, node.ParentOID, node.Name)
				}
			}
		}
	}

	// 第三遍：建立父子关系
	for i := range nodes {
		node := &nodes[i]
		tNode := nodeMap[node.OID]
		if tNode == nil {
			continue
		}

		if node.ParentOID != "" {
			if parent, ok := nodeMap[node.ParentOID]; ok {
				parent.Children = append(parent.Children, tNode)
			}
		}
	}

	// 第四遍：收集所有根节点
	// 根节点定义：ParentOID 为空，或者 ParentOID 不在 nodeMap 中
	// 重要：必须在所有父子关系建立完成后，再从 nodeMap 提取根节点
	rootNodes := make([]*tempNode, 0)
	collectedRoots := make(map[string]bool)

	// 收集模块内节点的根节点
	for i := range nodes {
		node := &nodes[i]
		if node.ParentOID == "" {
			// ParentOID 为空，是根节点
			if !collectedRoots[node.OID] {
				if tNode, ok := nodeMap[node.OID]; ok {
					rootNodes = append(rootNodes, tNode)
					collectedRoots[node.OID] = true
				}
			}
		} else if _, ok := nodeMap[node.ParentOID]; !ok {
			// ParentOID 不在 nodeMap 中，说明父节点缺失，该节点作为根节点
			if !collectedRoots[node.OID] {
				if tNode, ok := nodeMap[node.OID]; ok {
					rootNodes = append(rootNodes, tNode)
					collectedRoots[node.OID] = true
				}
			}
		}
	}

	// 收集全局父节点作为根节点（它们没有父节点在 nodeMap 中）
	for oid, tNode := range nodeMap {
		// 检查是否是模块内的节点
		isModuleNode := false
		for _, n := range nodes {
			if n.OID == oid {
				isModuleNode = true
				break
			}
		}
		// 如果不是模块内的节点，说明是全局父节点，应该作为根节点
		if !isModuleNode && !collectedRoots[oid] {
			rootNodes = append(rootNodes, tNode)
			collectedRoots[oid] = true
		}
	}

	// 递归转换为 snmp.MIBTreeNode
	var convert func(*tempNode) snmp.MIBTreeNode
	convert = func(tn *tempNode) snmp.MIBTreeNode {
		children := make([]snmp.MIBTreeNode, 0, len(tn.Children))
		for _, child := range tn.Children {
			children = append(children, convert(child))
		}
		return snmp.MIBTreeNode{
			ID:          tn.ID,
			OID:         tn.OID,
			Name:        tn.Name,
			NodeType:    tn.NodeType,
			Syntax:      tn.Syntax,
			Access:      tn.Access,
			Status:      tn.Status,
			Description: tn.Description,
			Children:    children,
			HasChildren: len(children) > 0,
		}
	}

	treeNodes := make([]snmp.MIBTreeNode, 0, len(rootNodes))
	for _, rn := range rootNodes {
		treeNodes = append(treeNodes, convert(rn))
	}

	logger.Debug("SNMP", "-", "获取 MIB 树: %s (%d 根节点)", module.Name, len(treeNodes))

	// 收集该模块在树中的所有最顶层入口节点（即节点本身属于该模块，且其祖先节点不属于该模块）
	moduleOIDs := make(map[string]bool)
	for i := range nodes {
		moduleOIDs[nodes[i].OID] = true
	}

	var findModuleRoots func(node snmp.MIBTreeNode) []snmp.MIBTreeNode
	findModuleRoots = func(node snmp.MIBTreeNode) []snmp.MIBTreeNode {
		if moduleOIDs[node.OID] {
			return []snmp.MIBTreeNode{node}
		}
		var roots []snmp.MIBTreeNode
		for _, child := range node.Children {
			roots = append(roots, findModuleRoots(child)...)
		}
		return roots
	}

	moduleRoots := make([]snmp.MIBTreeNode, 0)
	for _, rn := range treeNodes {
		moduleRoots = append(moduleRoots, findModuleRoots(rn)...)
	}

	if len(moduleRoots) > 0 {
		logger.Debug("SNMP", "-", "找到模块主入口节点: %d 个", len(moduleRoots))
		return moduleRoots, nil
	}

	logger.Debug("SNMP", "-", "未找到模块内节点，返回完整树")

	return treeNodes, nil
}

// GetMIBNode 获取 MIB 节点详情
func (s *SNMPMIBService) GetMIBNode(ctx context.Context, nodeID uint) (*MIBNodeVM, error) {
	node, err := s.repo.GetNodeByID(nodeID)
	if err != nil {
		return nil, fmt.Errorf("查询节点失败: %v", err)
	}

	if node == nil {
		return nil, fmt.Errorf("节点不存在: ID %d", nodeID)
	}

	vm := &MIBNodeVM{
		ID:          node.ID,
		OID:         node.OID,
		Name:        node.Name,
		Description: node.Description,
		Type:        node.Syntax,
		Access:      node.Access,
		Status:      node.Status,
		ParentID:    nil,
		ChildrenIDs: []uint{},
	}

	if node.ModuleID != nil {
		vm.ModuleID = *node.ModuleID
	}

	// 获取子节点 ID 列表
	children, err := s.repo.GetChildNodes(node.OID)
	if err == nil {
		for _, child := range children {
			vm.ChildrenIDs = append(vm.ChildrenIDs, child.ID)
		}
	}

	return vm, nil
}

// CreateMIBNode 手动创建 MIB 节点
func (s *SNMPMIBService) CreateMIBNode(ctx context.Context, req CreateMIBNodeRequest) error {
	moduleInfo, err := s.repo.GetModuleByID(req.ModuleID)
	if err == nil && moduleInfo != nil && moduleInfo.IsBuiltIn {
		return fmt.Errorf("内置核心库模块禁止添加节点")
	}

	node := &models.MIBNode{
		ModuleID:    &req.ModuleID,
		OID:         req.OID,
		Name:        req.Name,
		Description: req.Description,
		NodeType:    req.Type,
		Syntax:      req.Type,
		Access:      req.Access,
		Status:      req.Status,
		Source:      "manual",
	}

	if err := s.mibManager.AddManualNode(node); err != nil {
		logger.Error("SNMP", "-", "创建 MIB 节点失败: %s, %v", req.OID, err)
		return fmt.Errorf("创建节点失败: %v", err)
	}

	logger.Info("SNMP", "-", "MIB 节点已创建: %s (%s)", req.Name, req.OID)

	return nil
}

// UpdateMIBNode 更新 MIB 节点
func (s *SNMPMIBService) UpdateMIBNode(ctx context.Context, nodeID uint, req UpdateMIBNodeRequest) error {
	nodeInfo, err := s.repo.GetNodeByID(nodeID)
	if err == nil && nodeInfo != nil && nodeInfo.ModuleID != nil {
		moduleInfo, err := s.repo.GetModuleByID(*nodeInfo.ModuleID)
		if err == nil && moduleInfo != nil && moduleInfo.IsBuiltIn {
			return fmt.Errorf("内置核心库模块节点禁止修改")
		}
	}

	node := &models.MIBNode{
		Name:        req.Name,
		Description: req.Description,
		Syntax:      req.Type,
		Access:      req.Access,
		Status:      req.Status,
	}

	if err := s.mibManager.UpdateManualNode(nodeID, node); err != nil {
		logger.Error("SNMP", "-", "更新 MIB 节点失败: ID=%d, %v", nodeID, err)
		return fmt.Errorf("更新节点失败: %v", err)
	}

	logger.Info("SNMP", "-", "MIB 节点已更新: ID=%d", nodeID)

	return nil
}

// DeleteMIBNode 删除 MIB 节点
func (s *SNMPMIBService) DeleteMIBNode(ctx context.Context, nodeID uint) error {
	nodeInfo, err := s.repo.GetNodeByID(nodeID)
	if err == nil && nodeInfo != nil && nodeInfo.ModuleID != nil {
		moduleInfo, err := s.repo.GetModuleByID(*nodeInfo.ModuleID)
		if err == nil && moduleInfo != nil && moduleInfo.IsBuiltIn {
			return fmt.Errorf("内置核心库模块节点禁止删除")
		}
	}

	if err := s.mibManager.DeleteNode(nodeID); err != nil {
		logger.Error("SNMP", "-", "删除 MIB 节点失败: ID=%d, %v", nodeID, err)
		return fmt.Errorf("删除节点失败: %v", err)
	}

	logger.Info("SNMP", "-", "MIB 节点已删除: ID=%d", nodeID)

	return nil
}

// ============================================================================
// OID 解析
// ============================================================================

// ResolveOID 解析 OID
func (s *SNMPMIBService) ResolveOID(ctx context.Context, oid string) (*ResolvedOIDVM, error) {
	result, err := s.oidResolver.ResolveOID(oid)
	if err != nil {
		return nil, fmt.Errorf("解析 OID 失败: %v", err)
	}

	vm := &ResolvedOIDVM{
		OID:         result.OID,
		Name:        result.Name,
		ModuleName:  result.ModuleName,
		Description: result.Description,
		Type:        result.Type,
		Access:      result.Access,
		Status:      result.Status,
		Found:       result.Found,
	}

	return vm, nil
}

// ResolveNameToOID 将名称解析为 OID
func (s *SNMPMIBService) ResolveNameToOID(ctx context.Context, name string) (string, error) {
	oid, err := s.oidResolver.ResolveNameToOID(name)
	if err != nil {
		return "", fmt.Errorf("解析名称失败: %v", err)
	}

	return oid, nil
}

// SearchMIBNodes 搜索 MIB 节点
func (s *SNMPMIBService) SearchMIBNodes(ctx context.Context, query string) ([]MIBNodeVM, error) {
	results, err := s.oidResolver.SearchNodes(query)
	if err != nil {
		return nil, fmt.Errorf("搜索节点失败: %v", err)
	}

	vms := make([]MIBNodeVM, 0, len(results))
	for _, r := range results {
		vm := MIBNodeVM{
			ID:          r.ID,
			ModuleID:    r.ModuleID,
			ModuleName:  r.ModuleName,
			OID:         r.OID,
			Name:        r.Name,
			Description: r.Description,
			Type:        r.Type,
			Access:      r.Access,
			Status:      r.Status,
		}
		vms = append(vms, vm)
	}

	logger.Debug("SNMP", "-", "搜索 MIB 节点: '%s' -> %d 结果", query, len(vms))

	return vms, nil
}

// SearchMIBNodesInModule 在指定 MIB 模块中搜索节点
func (s *SNMPMIBService) SearchMIBNodesInModule(ctx context.Context, moduleID uint, query string) ([]MIBNodeVM, error) {
	nodes, err := s.repo.SearchNodesInModule(moduleID, query)
	if err != nil {
		return nil, fmt.Errorf("模块内搜索节点失败: %v", err)
	}

	// 获取模块名称
	moduleName := ""
	module, err := s.repo.GetModuleByID(moduleID)
	if err == nil && module != nil {
		moduleName = module.Name
	}

	vms := make([]MIBNodeVM, 0, len(nodes))
	for _, node := range nodes {
		vm := MIBNodeVM{
			ID:          node.ID,
			ModuleID:    moduleID,
			ModuleName:  moduleName,
			OID:         node.OID,
			Name:        node.Name,
			Description: node.Description,
			Type:        node.Syntax,
			Access:      node.Access,
			Status:      node.Status,
		}
		vms = append(vms, vm)
	}

	logger.Debug("SNMP", "-", "模块内搜索 MIB 节点: module=%d, '%s' -> %d 结果", moduleID, query, len(vms))

	return vms, nil
}

// ImportMIBFolder 导入文件夹下的所有 MIB 文件
func (s *SNMPMIBService) ImportMIBFolder(ctx context.Context, folderPath string, folderID *uint) error {
	var filePaths []string
	err := filepath.WalkDir(folderPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			ext := strings.ToLower(filepath.Ext(path))
			if ext == ".mib" || ext == ".my" || ext == ".txt" {
				filePaths = append(filePaths, path)
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("读取文件夹失败: %v", err)
	}

	if len(filePaths) == 0 {
		return fmt.Errorf("在该文件夹下未找到有效的 MIB 文件 (*.mib, *.my, *.txt)")
	}

	// 调试日志：追踪 folderID 传递
	if folderID != nil {
		logger.Info("SNMP", "-", "开始从文件夹 %s 导入 %d 个 MIB 文件, 目标文件夹ID=%d", folderPath, len(filePaths), *folderID)
	} else {
		logger.Info("SNMP", "-", "开始从文件夹 %s 导入 %d 个 MIB 文件, 目标文件夹ID=nil(未分类)", folderPath, len(filePaths))
	}

	return s.ImportMIBFiles(ctx, filePaths, folderID)
}

// ============================================================================
// 导出功能
// ============================================================================

// ExportMIB 导出 MIB 模块
func (s *SNMPMIBService) ExportMIB(ctx context.Context, moduleID uint, format string) ([]byte, error) {
	// 获取模块信息
	module, err := s.repo.GetModuleByID(moduleID)
	if err != nil {
		return nil, fmt.Errorf("查询模块失败: %v", err)
	}

	if module == nil {
		return nil, fmt.Errorf("模块不存在: ID %d", moduleID)
	}

	// 获取模块下的所有节点
	nodes, err := s.repo.GetNodesByModule(moduleID)
	if err != nil {
		return nil, fmt.Errorf("查询节点失败: %v", err)
	}

	// 根据格式导出
	switch format {
	case "json":
		return s.exportAsJSON(module, nodes)
	default:
		return nil, fmt.Errorf("不支持的导出格式: %s", format)
	}
}

// exportAsJSON 导出为 JSON 格式
func (s *SNMPMIBService) exportAsJSON(module *models.MIBModule, nodes []models.MIBNode) ([]byte, error) {
	exportData := struct {
		Module MIBModuleVM `json:"module"`
		Nodes  []MIBNodeVM `json:"nodes"`
	}{
		Module: MIBModuleVM{
			ID:          module.ID,
			Name:        module.Name,
			Description: module.Description,
			NodeCount:   module.NodeCount,
			ImportedAt:  module.CreatedAt,
			Status:      "active",
		},
		Nodes: make([]MIBNodeVM, 0, len(nodes)),
	}

	for _, node := range nodes {
		exportData.Nodes = append(exportData.Nodes, MIBNodeVM{
			ID:          node.ID,
			OID:         node.OID,
			Name:        node.Name,
			Description: node.Description,
			Type:        node.Syntax,
			Access:      node.Access,
			Status:      node.Status,
		})
	}

	data, err := json.MarshalIndent(exportData, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("序列化失败: %v", err)
	}

	logger.Info("SNMP", "-", "MIB 模块已导出: %s (JSON, %d 节点)",
		module.Name, len(nodes))

	return data, nil
}

// ============================================================================
// 缓存管理
// ============================================================================

// ClearResolverCache 清除 OID 解析器缓存
func (s *SNMPMIBService) ClearResolverCache(ctx context.Context) error {
	s.oidResolver.ClearCache()
	logger.Info("SNMP", "-", "OID 解析器缓存已清除")
	return nil
}

// RebuildResolverCache 重建 OID 解析器缓存
func (s *SNMPMIBService) RebuildResolverCache(ctx context.Context) error {
	if err := s.oidResolver.RebuildCache(); err != nil {
		return fmt.Errorf("重建缓存失败: %v", err)
	}
	logger.Info("SNMP", "-", "OID 解析器缓存已重建")
	return nil
}

// GetCacheStats 获取缓存统计信息
func (s *SNMPMIBService) GetCacheStats(ctx context.Context) map[string]int {
	oidLen, nameLen := s.oidResolver.CacheStats()
	return map[string]int{
		"oidCacheLen":  oidLen,
		"nameCacheLen": nameLen,
	}
}
