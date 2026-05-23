// Package ui 提供 Wails 暴露层服务
// snmp_mib_service.go 实现 MIB 管理 Wails 绑定服务
package ui

import (
	"context"
	"encoding/json"
	"fmt"
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
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Version     string    `json:"version"`
	NodeCount   int       `json:"nodeCount"`
	ImportedAt  time.Time `json:"importedAt"`
	Status      string    `json:"status"` // active, error, partial
}

// MIBNodeVM MIB 节点视图模型
type MIBNodeVM struct {
	ID          uint   `json:"id"`
	ModuleID    uint   `json:"moduleId"`
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
	FilePath     string `json:"filePath"`
	ModuleName   string `json:"moduleName,omitempty"`
	PartialImport bool   `json:"partialImport"` // 是否允许部分导入
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
			Name:        m.Name,
			Description: m.Description,
			Version:     "", // 从描述中提取版本信息（如果有）
			NodeCount:   m.NodeCount,
			ImportedAt:  m.CreatedAt,
			Status:      "active",
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
		Name:        module.Name,
		Description: module.Description,
		NodeCount:   module.NodeCount,
		ImportedAt:  module.CreatedAt,
		Status:      "active",
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
	result, err := s.mibManager.ImportMIBFile(ctx, req.FilePath)
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

	return nil
}

// ImportMIBFiles 批量导入 MIB 文件
func (s *SNMPMIBService) ImportMIBFiles(ctx context.Context, filePaths []string) error {
	if len(filePaths) == 0 {
		return nil
	}

	totalFiles := len(filePaths)
	successCount := 0
	errorCount := 0

	for i, filePath := range filePaths {
		// 检查上下文是否已取消
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// 发送整体进度
		s.eventNotifier.NotifyMIBImportProgress(snmp.MIBImportProgress{
			FileName: filePath,
			Phase:    "parsing",
			Progress: float64(i) / float64(totalFiles) * 100,
		})

		err := s.ImportMIB(ctx, ImportMIBRequest{
			FilePath:      filePath,
			PartialImport: true,
		})

		if err != nil {
			errorCount++
			logger.Warn("SNMP", "-", "批量导入文件失败 [%d/%d]: %s, %v",
				i+1, totalFiles, filePath, err)
			continue
		}

		successCount++
	}

	logger.Info("SNMP", "-", "批量导入完成: 成功 %d, 失败 %d, 总计 %d",
		successCount, errorCount, totalFiles)

	if errorCount > 0 {
		return fmt.Errorf("部分文件导入失败: 成功 %d, 失败 %d", successCount, errorCount)
	}

	return nil
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

	moduleName := module.Name

	// 执行删除
	if err := s.mibManager.DeleteModule(moduleID); err != nil {
		logger.Error("SNMP", "-", "删除 MIB 模块失败: %s, %v", moduleName, err)
		return fmt.Errorf("删除模块失败: %v", err)
	}

	// 发送删除事件
	s.eventNotifier.NotifyMIBDeleted(moduleID)

	logger.Info("SNMP", "-", "MIB 模块已删除: %s", moduleName)

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
	treeNodes := make([]snmp.MIBTreeNode, 0)
	nodeMap := make(map[string]*snmp.MIBTreeNode)

	// 第一遍：创建所有节点
	for i := range nodes {
		node := &nodes[i]
		treeNode := &snmp.MIBTreeNode{
			ID:          node.ID,
			OID:         node.OID,
			Name:        node.Name,
			NodeType:    node.NodeType,
			Syntax:      node.Syntax,
			Access:      node.Access,
			Status:      node.Status,
			Description: node.Description,
			Children:    []snmp.MIBTreeNode{},
			HasChildren: false,
		}
		nodeMap[node.OID] = treeNode
	}

	// 第二遍：建立父子关系
	for i := range nodes {
		node := &nodes[i]
		treeNode := nodeMap[node.OID]

		if node.ParentOID == "" {
			// 根节点
			treeNodes = append(treeNodes, *treeNode)
		} else {
			// 添加到父节点
			if parent, ok := nodeMap[node.ParentOID]; ok {
				parent.Children = append(parent.Children, *treeNode)
				parent.HasChildren = true
			}
		}
	}

	logger.Debug("SNMP", "-", "获取 MIB 树: %s (%d 根节点)", module.Name, len(treeNodes))

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
