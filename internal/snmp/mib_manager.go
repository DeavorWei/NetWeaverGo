// Package snmp 提供 SNMP 核心业务功能
// mib_manager.go 实现 MIB 生命周期管理，包括导入/删除/缓存/树构建
package snmp

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"

	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
	"github.com/NetWeaverGo/core/internal/repository"
)

// DefaultLRUCacheSize LRU 缓存默认容量
const DefaultLRUCacheSize = 10000

// MIBManager MIB 生命周期管理器
// 负责 MIB 文件导入/删除、LRU 缓存、OID 树构建
type MIBManager struct {
	mibRepo      repository.MIBRepository
	mibStoreDir  string               // MIB 文件存储目录
	parser       *MIBParser           // MIB 解析器
	nodeCache    *lru.Cache[string, *models.MIBNode] // OID → Node LRU 缓存
	nameCache    *lru.Cache[string, string]           // Name → OID LRU 缓存
	maxCacheSize int                  // 缓存容量上限
	mu           sync.RWMutex         // 主锁：保护 mibRepo 访问和整体操作
	cacheMu      sync.Mutex           // 缓存专用锁：保护 LRU cache 写操作
}

// NewMIBManager 创建 MIB 管理器实例
func NewMIBManager(mibRepo repository.MIBRepository, mibStoreDir string) *MIBManager {
	nodeCache, err := lru.New[string, *models.MIBNode](DefaultLRUCacheSize)
	if err != nil {
		logger.Error("SNMP", "-", "创建 MIB 节点缓存失败: %v", err)
		nodeCache, _ = lru.New[string, *models.MIBNode](1000)
	}

	nameCache, err := lru.New[string, string](DefaultLRUCacheSize)
	if err != nil {
		logger.Error("SNMP", "-", "创建 MIB 名称缓存失败: %v", err)
		nameCache, _ = lru.New[string, string](1000)
	}

	mgr := &MIBManager{
		mibRepo:      mibRepo,
		mibStoreDir:  mibStoreDir,
		parser:       NewMIBParser(),
		nodeCache:    nodeCache,
		nameCache:    nameCache,
		maxCacheSize: DefaultLRUCacheSize,
	}

	// 初始化 gosmi 库
	mgr.parser.Init()

	return mgr
}

// Close 清理 MIB 管理器资源
func (m *MIBManager) Close() {
	m.parser.Exit()
}

// ============================================================================
// MIB 文件导入
// ============================================================================

// ImportMIBFile 导入 MIB 文件
// 流程：复制文件到存储目录 → 解析 MIB → 存入数据库 → 更新缓存
// 支持部分导入策略：即使部分节点解析失败，已成功的节点仍保留
func (m *MIBManager) ImportMIBFile(ctx context.Context, filePath string) (*MIBImportResult, error) {
	importStartTime := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查上下文是否已取消
	if ctx.Err() != nil {
		logger.Warn("SNMP-MIB", "-", "MIB 导入取消: 文件=%s, 原因=%v", filePath, ctx.Err())
		return nil, ctx.Err()
	}

	logger.Info("SNMP-MIB", "-", "开始导入 MIB 文件: 路径=%s", filePath)

	// 检查文件是否存在
	if _, err := os.Stat(filePath); err != nil {
		logger.Error("SNMP-MIB", "-", "MIB 文件不存在: %s", filePath)
		return nil, fmt.Errorf("MIB 文件不存在: %s", filePath)
	}

	// 复制文件到 MIB 存储目录
	copyStartTime := time.Now()
	storedPath, err := m.copyMIBFile(filePath)
	if err != nil {
		logger.Error("SNMP-MIB", "-", "复制 MIB 文件失败: %s, %v", filePath, err)
		return nil, fmt.Errorf("复制 MIB 文件失败: %v", err)
	}
	logger.Debug("SNMP-MIB", "-", "文件复制完成: 源=%s, 目标=%s, 耗时=%v", filePath, storedPath, time.Since(copyStartTime))

	// 解析 MIB 文件
	parseStartTime := time.Now()
	result, err := m.parser.ParseFile(storedPath)
	if err != nil {
		// 解析失败，删除已复制的文件
		_ = os.Remove(storedPath)
		logger.Error("SNMP-MIB", "-", "解析 MIB 文件失败: %s, 耗时=%v, 错误=%v", filePath, time.Since(parseStartTime), err)
		return nil, fmt.Errorf("解析 MIB 文件失败: %v", err)
	}
	logger.Debug("SNMP-MIB", "-", "MIB 解析完成: 模块=%s, 节点数=%d, 错误数=%d, 耗时=%v",
		result.Module.Name, result.NodeCount, result.ErrorCount, time.Since(parseStartTime))

	// 检查是否已存在同名模块
	existing, _ := m.mibRepo.GetModuleByName(result.Module.Name)
	if existing != nil {
		logger.Info("SNMP-MIB", "-", "检测到同名模块，将覆盖: 模块=%s, 旧ID=%d", result.Module.Name, existing.ID)
		// 删除已存在的模块和文件
		_ = os.Remove(existing.FilePath)
		_ = m.mibRepo.DeleteNodesByModule(existing.ID)
		_ = m.mibRepo.DeleteModule(existing.ID)
	}

	// 保存模块到数据库
	saveStartTime := time.Now()
	if err := m.mibRepo.SaveModule(result.Module); err != nil {
		_ = os.Remove(storedPath)
		logger.Error("SNMP-MIB", "-", "保存 MIB 模块失败: %s, %v", result.Module.Name, err)
		return nil, fmt.Errorf("保存 MIB 模块失败: %v", err)
	}
	logger.Debug("SNMP-MIB", "-", "模块保存完成: ID=%d, 名称=%s, 耗时=%v", result.Module.ID, result.Module.Name, time.Since(saveStartTime))

	// 设置节点的 ModuleID 并保存
	moduleID := result.Module.ID
	for i := range result.Nodes {
		result.Nodes[i].ModuleID = &moduleID
	}

	if len(result.Nodes) > 0 {
		saveNodesStartTime := time.Now()
		if err := m.mibRepo.SaveNodes(result.Nodes); err != nil {
			_ = m.mibRepo.DeleteModule(moduleID)
			_ = os.Remove(storedPath)
			logger.Error("SNMP-MIB", "-", "保存 MIB 节点失败: 模块=%s, %v", result.Module.Name, err)
			return nil, fmt.Errorf("保存 MIB 节点失败: %v", err)
		}
		logger.Debug("SNMP-MIB", "-", "节点保存完成: 数量=%d, 耗时=%v", len(result.Nodes), time.Since(saveNodesStartTime))
	}

	// 更新缓存
	cacheStartTime := time.Now()
	for i := range result.Nodes {
		m.nodeCache.Add(result.Nodes[i].OID, &result.Nodes[i])
		m.nameCache.Add(result.Nodes[i].Name, result.Nodes[i].OID)
	}
	logger.Debug("SNMP-MIB", "-", "缓存更新完成: 新增=%d, 总计=%d, 耗时=%v",
		len(result.Nodes), m.nodeCache.Len(), time.Since(cacheStartTime))

	totalLatency := time.Since(importStartTime)
	logger.Info("SNMP-MIB", "-", "MIB 模块导入成功: 模块=%s, 节点数=%d, 错误数=%d, 总耗时=%v",
		result.Module.Name, result.NodeCount, result.ErrorCount, totalLatency)

	return result, nil
}

// copyMIBFile 复制 MIB 文件到存储目录
func (m *MIBManager) copyMIBFile(srcPath string) (string, error) {
	// 确保存储目录存在
	if err := os.MkdirAll(m.mibStoreDir, 0755); err != nil {
		return "", fmt.Errorf("创建 MIB 存储目录失败: %v", err)
	}

	fileName := filepath.Base(srcPath)
	dstPath := filepath.Join(m.mibStoreDir, fileName)

	// 如果目标文件已存在，添加序号
	counter := 1
	for {
		if _, err := os.Stat(dstPath); os.IsNotExist(err) {
			break
		}
		ext := filepath.Ext(fileName)
		base := fileName[:len(fileName)-len(ext)]
		dstPath = filepath.Join(m.mibStoreDir, fmt.Sprintf("%s_%d%s", base, counter, ext))
		counter++
	}

	// 复制文件
	src, err := os.Open(srcPath)
	if err != nil {
		return "", err
	}
	defer src.Close()

	dst, err := os.Create(dstPath)
	if err != nil {
		return "", err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		_ = os.Remove(dstPath)
		return "", err
	}

	return dstPath, nil
}

// ============================================================================
// 手动节点管理
// ============================================================================

// AddManualNode 手动添加 MIB 节点
func (m *MIBManager) AddManualNode(node *models.MIBNode) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	node.Source = "manual"

	// 检查 OID 是否已存在
	existing, err := m.mibRepo.GetNodeByOID(node.OID)
	if err != nil {
		return fmt.Errorf("查询节点失败: %v", err)
	}
	if existing != nil {
		return fmt.Errorf("OID 已存在: %s", node.OID)
	}

	// 保存节点
	if err := m.mibRepo.SaveNode(node); err != nil {
		return fmt.Errorf("保存节点失败: %v", err)
	}

	// 更新缓存
	m.nodeCache.Add(node.OID, node)
	m.nameCache.Add(node.Name, node.OID)

	return nil
}

// UpdateManualNode 更新手动添加的节点
func (m *MIBManager) UpdateManualNode(id uint, node *models.MIBNode) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 查询现有节点
	existing, err := m.mibRepo.GetNodeByID(id)
	if err != nil {
		return fmt.Errorf("查询节点失败: %v", err)
	}
	if existing == nil {
		return fmt.Errorf("节点不存在: ID %d", id)
	}

	// 只允许更新手动添加的节点
	if existing.Source != "manual" {
		return fmt.Errorf("不允许修改导入的节点，仅可编辑手动添加的节点")
	}

	// 更新字段
	existing.Name = node.Name
	existing.ParentOID = node.ParentOID
	existing.NodeType = node.NodeType
	existing.Syntax = node.Syntax
	existing.Access = node.Access
	existing.Status = node.Status
	existing.Description = node.Description

	if err := m.mibRepo.SaveNode(existing); err != nil {
		return fmt.Errorf("更新节点失败: %v", err)
	}

	// 更新缓存
	m.nodeCache.Add(existing.OID, existing)
	m.nameCache.Add(existing.Name, existing.OID)

	return nil
}

// DeleteNode 删除节点
func (m *MIBManager) DeleteNode(id uint) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 查询节点
	node, err := m.mibRepo.GetNodeByID(id)
	if err != nil {
		return fmt.Errorf("查询节点失败: %v", err)
	}
	if node == nil {
		return fmt.Errorf("节点不存在: ID %d", id)
	}

	// 删除节点
	if err := m.mibRepo.DeleteNode(id); err != nil {
		return fmt.Errorf("删除节点失败: %v", err)
	}

	// 清除缓存
	m.nodeCache.Remove(node.OID)
	m.nameCache.Remove(node.Name)

	return nil
}

// DeleteModule 删除整个 MIB 模块及其所有节点
func (m *MIBManager) DeleteModule(id uint) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 查询模块
	module, err := m.mibRepo.GetModuleByID(id)
	if err != nil {
		return fmt.Errorf("查询模块失败: %v", err)
	}
	if module == nil {
		return fmt.Errorf("模块不存在: ID %d", id)
	}

	// 获取模块下所有节点（用于清除缓存）
	nodes, err := m.mibRepo.GetNodesByModule(id)
	if err != nil {
		return fmt.Errorf("查询模块节点失败: %v", err)
	}

	// 删除模块的所有节点
	if err := m.mibRepo.DeleteNodesByModule(id); err != nil {
		return fmt.Errorf("删除模块节点失败: %v", err)
	}

	// 删除模块记录
	if err := m.mibRepo.DeleteModule(id); err != nil {
		return fmt.Errorf("删除模块失败: %v", err)
	}

	// 删除 MIB 文件
	if module.FilePath != "" {
		_ = os.Remove(module.FilePath)
	}

	// 清除缓存
	for _, node := range nodes {
		m.nodeCache.Remove(node.OID)
		m.nameCache.Remove(node.Name)
	}

	logger.Info("SNMP", "-", "MIB 模块已删除: %s", module.Name)

	return nil
}

// ============================================================================
// 查询操作
// ============================================================================

// ResolveOID 解析 OID 为可读名称
// 如果用户未导入对应 MIB，返回原始 OID 字符串（优雅降级，不报错）
func (m *MIBManager) ResolveOID(oid string) string {
	// 先查缓存（使用 RLock 保护读操作）
	m.mu.RLock()
	if cached, ok := m.nodeCache.Get(oid); ok {
		m.mu.RUnlock()
		return cached.Name
	}
	m.mu.RUnlock()

	// 查数据库（需要 RLock）
	m.mu.RLock()
	node, err := m.mibRepo.GetNodeByOID(oid)
	m.mu.RUnlock()

	if err != nil || node == nil {
		return oid // 未找到，返回原始 OID
	}

	// 更新缓存（使用独立的 cacheMu Lock）
	m.cacheMu.Lock()
	m.nodeCache.Add(oid, node)
	m.cacheMu.Unlock()

	return node.Name
}

// ResolveName 解析名称为 OID
// 如果未找到，返回空字符串
func (m *MIBManager) ResolveName(name string) (string, error) {
	// 先查缓存（使用 RLock 保护读操作）
	m.mu.RLock()
	if cached, ok := m.nameCache.Get(name); ok {
		m.mu.RUnlock()
		return cached, nil
	}
	m.mu.RUnlock()

	// 查数据库（需要 RLock）
	m.mu.RLock()
	node, err := m.mibRepo.GetNodeByName(name)
	m.mu.RUnlock()

	if err != nil {
		return "", err
	}
	if node == nil {
		return "", nil // 未找到
	}

	// 更新缓存（使用独立的 cacheMu Lock）
	m.cacheMu.Lock()
	m.nameCache.Add(name, node.OID)
	m.nodeCache.Add(node.OID, node)
	m.cacheMu.Unlock()

	return node.OID, nil
}

// SearchNodes 搜索 MIB 节点（按名称或 OID 模糊匹配）
func (m *MIBManager) SearchNodes(query string) ([]models.MIBNode, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	nodes, err := m.mibRepo.SearchNodes(query)
	if err != nil {
		return nil, err
	}

	// P3-4: 按名称排序返回结果，确保顺序一致
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Name < nodes[j].Name
	})

	return nodes, nil
}

// GetOIDTree 获取指定父 OID 下的子树结构
func (m *MIBManager) GetOIDTree(parentOID string) ([]MIBTreeNode, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 获取直接子节点
	children, err := m.mibRepo.GetChildNodes(parentOID)
	if err != nil {
		return nil, fmt.Errorf("查询子节点失败: %v", err)
	}

	treeNodes := make([]MIBTreeNode, 0, len(children))
	for i := range children {
		// 检查是否有子节点
		hasChildren, err := m.mibRepo.CountChildNodes(children[i].OID)
		if err != nil {
			hasChildren = 0
		}

		treeNodes = append(treeNodes, MIBTreeNode{
			ID:          children[i].ID,
			OID:         children[i].OID,
			Name:        children[i].Name,
			NodeType:    children[i].NodeType,
			Syntax:      children[i].Syntax,
			Access:      children[i].Access,
			Status:      children[i].Status,
			Description: children[i].Description,
			HasChildren: hasChildren > 0,
		})
	}

	return treeNodes, nil
}

// GetModules 获取所有已导入的 MIB 模块
func (m *MIBManager) GetModules() ([]models.MIBModule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.mibRepo.GetAllModules()
}

// GetNodeByOID 通过 OID 获取节点详情
func (m *MIBManager) GetNodeByOID(oid string) (*models.MIBNode, error) {
	// 先查缓存（使用 RLock 保护读操作）
	m.mu.RLock()
	if cached, ok := m.nodeCache.Get(oid); ok {
		m.mu.RUnlock()
		return cached, nil
	}
	m.mu.RUnlock()

	// 查数据库（需要 RLock）
	m.mu.RLock()
	node, err := m.mibRepo.GetNodeByOID(oid)
	m.mu.RUnlock()

	if err != nil {
		return nil, err
	}
	if node != nil {
		// 更新缓存（使用独立的 cacheMu Lock）
		m.cacheMu.Lock()
		m.nodeCache.Add(oid, node)
		m.cacheMu.Unlock()
	}
	return node, nil
}

// ============================================================================
// 缓存管理
// ============================================================================

// RebuildCache 重建内存缓存（从数据库重新加载所有节点）
func (m *MIBManager) RebuildCache() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 清空现有缓存
	m.nodeCache.Purge()
	m.nameCache.Purge()

	// 从数据库加载所有节点
	nodes, err := m.mibRepo.GetAllNodes()
	if err != nil {
		return fmt.Errorf("加载节点失败: %v", err)
	}

	// 填充缓存
	for i := range nodes {
		m.nodeCache.Add(nodes[i].OID, &nodes[i])
		m.nameCache.Add(nodes[i].Name, nodes[i].OID)
	}

	logger.Info("SNMP", "-", "MIB 缓存重建完成: %d 节点", len(nodes))

	return nil
}

// CacheStats 返回缓存统计信息
func (m *MIBManager) CacheStats() (nodeCacheLen, nameCacheLen int) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.nodeCache.Len(), m.nameCache.Len()
}