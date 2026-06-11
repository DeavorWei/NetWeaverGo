// Package snmp 提供 SNMP 核心业务功能
// oid_resolver.go 实现 OID 解析服务，提供 OID 到名称的双向解析
package snmp

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"

	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
	"github.com/NetWeaverGo/core/internal/repository"
)

// DefaultResolverCacheSize OID 解析器 LRU 缓存默认容量
const DefaultResolverCacheSize = 5000

// ============================================================================
// 解析结果类型
// ============================================================================

// ResolvedOID OID 解析结果
type ResolvedOID struct {
	ID          uint     `json:"id"`          // 数据库节点 ID
	ModuleID    uint     `json:"moduleId"`    // 数据库模块 ID
	OID         string   `json:"oid"`         // 原始 OID
	Name        string   `json:"name"`        // 解析后的名称
	ModuleName  string   `json:"moduleName"`  // 所属 MIB 模块
	Description string   `json:"description"` // 描述
	Type        string   `json:"type"`        // 数据类型
	Access      string   `json:"access"`      // 访问权限 (read-only, read-write, etc.)
	Status      string   `json:"status"`      // 状态 (current, deprecated, etc.)
	ParentOID   string   `json:"parentOid"`   // 父节点 OID
	Children    []string `json:"children"`    // 子节点 OID 列表
	Found       bool     `json:"found"`       // 是否找到解析结果
}

// ============================================================================
// OIDResolver 实现
// ============================================================================

// OIDResolver 提供 OID 解析功能
// 支持 OID 到名称的双向解析，使用 LRU 缓存优化性能
type OIDResolver struct {
	mibManager *MIBManager
	repo       repository.MIBRepository

	// LRU 缓存
	oidCache  *lru.Cache[string, *ResolvedOID] // OID -> 解析结果
	nameCache *lru.Cache[string, string]       // Name -> OID

	// 并发控制：协调缓存读写与 ClearCache/RebuildCache 操作
	// 注意：LRU 缓存自身已线程安全，此互斥锁用于协调整体操作的原子性
	mu sync.RWMutex
}

// NewOIDResolver 创建 OID 解析器实例
func NewOIDResolver(mibManager *MIBManager, repo repository.MIBRepository) *OIDResolver {
	oidCache, err := lru.New[string, *ResolvedOID](DefaultResolverCacheSize)
	if err != nil {
		logger.Error("SNMP", "-", "创建 OID 解析缓存失败: %v", err)
		oidCache, _ = lru.New[string, *ResolvedOID](1000)
	}

	nameCache, err := lru.New[string, string](DefaultResolverCacheSize)
	if err != nil {
		logger.Error("SNMP", "-", "创建名称解析缓存失败: %v", err)
		nameCache, _ = lru.New[string, string](1000)
	}

	resolver := &OIDResolver{
		mibManager: mibManager,
		repo:       repo,
		oidCache:   oidCache,
		nameCache:  nameCache,
	}

	logger.Info("SNMP", "-", "OID 解析器已初始化 (缓存容量: %d)", DefaultResolverCacheSize)

	return resolver
}

// ============================================================================
// 核心解析方法
// ============================================================================

// ResolveOID 解析 OID 到名称和信息
// 如果未找到对应 MIB 节点，返回优雅降级结果（Found=false）
// 采用 double-check locking 模式：先查缓存→释放锁→查数据库→再加锁写缓存，
// 避免在持有读锁期间执行数据库查询导致写操作饥饿。
func (r *OIDResolver) ResolveOID(oid string) (*ResolvedOID, error) {
	resolveStartTime := time.Now()
	oid = normalizeOID(oid)

	// Step 1: 先加读锁查缓存，命中则直接返回
	r.mu.RLock()
	if cached, ok := r.oidCache.Get(oid); ok {
		r.mu.RUnlock()
		logger.Verbose("SNMP-Resolver", "-", "OID 缓存命中: %s -> %s (命中=%v)", oid, cached.Name, cached.Found)
		return cached, nil
	}
	r.mu.RUnlock()

	logger.Verbose("SNMP-Resolver", "-", "OID 缓存未命中: %s, 查询数据库", oid)

	// Step 2: 无锁状态下执行数据库查询
	dbStartTime := time.Now()
	node, err := r.repo.GetNodeByOID(oid)
	if err != nil {
		logger.Error("SNMP-Resolver", "-", "查询 OID 节点失败: %s, 耗时=%v, 错误=%v", oid, time.Since(dbStartTime), err)
		return nil, fmt.Errorf("查询 OID 节点失败: %v", err)
	}

	// 在锁外构建完整解析结果（buildResolvedOID 可能触发额外 DB 查询）
	var result *ResolvedOID
	if node == nil {
		result = &ResolvedOID{
			OID:   oid,
			Name:  oid,
			Found: false,
		}
	} else {
		result = r.buildResolvedOID(node)
		result.Found = true
	}

	// Step 3: 加写锁，double-check 缓存，写入结果
	r.mu.Lock()
	defer r.mu.Unlock()

	// 再次检查缓存（防止并发写入导致重复查询）
	if cached, ok := r.oidCache.Get(oid); ok {
		logger.Verbose("SNMP-Resolver", "-", "OID 缓存命中(double-check): %s -> %s", oid, cached.Name)
		return cached, nil
	}

	// 写入缓存
	r.oidCache.Add(oid, result)
	if node != nil {
		r.nameCache.Add(node.Name, oid)
	}

	resolveLatency := time.Since(resolveStartTime)
	if node != nil {
		logger.Debug("SNMP-Resolver", "-", "OID 解析成功: %s -> %s (模块: %s), 耗时=%v",
			oid, result.Name, result.ModuleName, resolveLatency)
	} else {
		logger.Debug("SNMP-Resolver", "-", "OID 未找到: %s, 耗时=%v", oid, resolveLatency)
	}

	return result, nil
}

// ResolveNameToOID 将名称解析为 OID
// 如果未找到，返回空字符串和 nil 错误（优雅降级）
// 采用 double-check locking 模式，避免在持有读锁期间执行数据库查询。
func (r *OIDResolver) ResolveNameToOID(name string) (string, error) {
	if name == "" {
		return "", nil
	}

	// Step 1: 先加读锁查缓存
	r.mu.RLock()
	if cached, ok := r.nameCache.Get(name); ok {
		r.mu.RUnlock()
		return cached, nil
	}
	r.mu.RUnlock()

	// Step 2: 无锁状态下执行数据库查询
	node, err := r.repo.GetNodeByName(name)
	if err != nil {
		return "", fmt.Errorf("查询名称节点失败: %v", err)
	}

	// 在锁外构建解析结果
	var resolved *ResolvedOID
	oid := ""
	if node != nil {
		oid = node.OID
		resolved = r.buildResolvedOID(node)
	}

	// Step 3: 加写锁，double-check，写缓存
	r.mu.Lock()
	defer r.mu.Unlock()

	// Double-check
	if cached, ok := r.nameCache.Get(name); ok {
		return cached, nil
	}

	if node == nil {
		// 未找到，缓存空结果避免重复查询
		r.nameCache.Add(name, "")
		return "", nil
	}

	// 缓存结果
	r.nameCache.Add(name, oid)
	r.oidCache.Add(oid, resolved)

	return oid, nil
}

// ResolveBatch 批量解析 OID
// 返回解析结果列表，失败的 OID 也会包含在结果中（Found=false）
func (r *OIDResolver) ResolveBatch(oids []string) ([]*ResolvedOID, error) {
	if len(oids) == 0 {
		return []*ResolvedOID{}, nil
	}

	results := make([]*ResolvedOID, 0, len(oids))

	for _, oid := range oids {
		result, err := r.ResolveOID(oid)
		if err != nil {
			// 单个解析失败不中断批量操作
			results = append(results, &ResolvedOID{
				OID:   oid,
				Name:  oid,
				Found: false,
			})
			logger.Warn("SNMP", "-", "批量解析 OID 失败: %s, %v", oid, err)
			continue
		}
		results = append(results, result)
	}

	return results, nil
}

// GetSubtree 获取 OID 子树
// 返回指定 OID 及其所有子节点的解析结果
// 采用无锁查询 + 写锁批量缓存模式，避免在持有锁期间执行数据库查询。
func (r *OIDResolver) GetSubtree(oid string) ([]*ResolvedOID, error) {
	oid = normalizeOID(oid)

	// 无锁查询子节点
	children, err := r.repo.GetChildNodes(oid)
	if err != nil {
		return nil, fmt.Errorf("查询子节点失败: %v", err)
	}

	results := make([]*ResolvedOID, 0, len(children)+1)

	// 解析根节点（ResolveOID 内部处理缓存和锁的协调）
	rootResult, err := r.ResolveOID(oid)
	if err != nil {
		return nil, err
	}
	results = append(results, rootResult)

	// 在锁外构建所有子节点解析结果（buildResolvedOID 可能触发 DB 查询）
	childResults := make([]*ResolvedOID, 0, len(children))
	for i := range children {
		childResult := r.buildResolvedOID(&children[i])
		childResult.Found = true
		childResults = append(childResults, childResult)
	}

	// 加写锁，缓存子节点
	r.mu.Lock()
	for i, childResult := range childResults {
		// 缓存子节点（LRU 自身线程安全，此处写锁用于与 RebuildCache 协调）
		r.oidCache.Add(children[i].OID, childResult)
		r.nameCache.Add(children[i].Name, children[i].OID)
	}
	r.mu.Unlock()

	return results, nil
}

// ============================================================================
// 辅助方法
// ============================================================================

// buildResolvedOID 从 MIBNode 构建解析结果
// 注意：此方法会执行额外的数据库查询（获取模块名和子节点列表），调用方应确保不持有锁
func (r *OIDResolver) buildResolvedOID(node *models.MIBNode) *ResolvedOID {
	result := &ResolvedOID{
		ID:          node.ID,
		OID:         node.OID,
		Name:        node.Name,
		Description: node.Description,
		Type:        node.Syntax,
		Access:      node.Access,
		Status:      node.Status,
		ParentOID:   node.ParentOID,
		Children:    []string{},
		Found:       true,
	}

	// 获取模块名称
	if node.ModuleID != nil {
		result.ModuleID = *node.ModuleID
		module, err := r.repo.GetModuleByID(*node.ModuleID)
		if err == nil && module != nil {
			result.ModuleName = module.Name
		}
	}

	// 获取子节点 OID 列表
	children, err := r.repo.GetChildNodes(node.OID)
	if err == nil {
		for _, child := range children {
			result.Children = append(result.Children, child.OID)
		}
	}

	return result
}

// normalizeOID 标准化 OID 格式
// 移除前导点，确保格式一致
func normalizeOID(oid string) string {
	oid = strings.TrimSpace(oid)
	oid = strings.TrimPrefix(oid, ".")
	return oid
}

// ============================================================================
// 缓存管理
// ============================================================================

// ClearCache 清除所有缓存
func (r *OIDResolver) ClearCache() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.oidCache.Purge()
	r.nameCache.Purge()

	logger.Info("SNMP", "-", "OID 解析器缓存已清除")
}

// CacheStats 返回缓存统计信息
func (r *OIDResolver) CacheStats() (oidCacheLen, nameCacheLen int) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.oidCache.Len(), r.nameCache.Len()
}

// RebuildCache 重建缓存（从数据库重新加载）
// 采用分页加载模式，避免一次性加载所有节点导致内存峰值过高和启动延迟。
// 每批在锁外构建解析结果，加锁后写入缓存，最小化锁持有时间。
func (r *OIDResolver) RebuildCache() error {
	// 加写锁清空缓存
	r.mu.Lock()
	r.oidCache.Purge()
	r.nameCache.Purge()
	r.mu.Unlock()

	// 分页加载节点，每批构建结果后写入缓存
	batchSize := 1000
	offset := 0
	totalNodes := 0

	for {
		// 无锁状态下执行数据库查询
		nodes, err := r.repo.GetNodesBatch(offset, batchSize)
		if err != nil {
			return fmt.Errorf("分页加载节点失败: %v", err)
		}

		if len(nodes) == 0 {
			break
		}

		// 在锁外构建解析结果（buildResolvedOID 可能触发额外 DB 查询）
		results := make([]*ResolvedOID, len(nodes))
		for i := range nodes {
			result := r.buildResolvedOID(&nodes[i])
			result.Found = true
			results[i] = result
		}

		// 加写锁写入缓存
		r.mu.Lock()
		for i := range nodes {
			r.oidCache.Add(nodes[i].OID, results[i])
			r.nameCache.Add(nodes[i].Name, nodes[i].OID)
		}
		r.mu.Unlock()

		totalNodes += len(nodes)
		offset += batchSize
	}

	logger.Info("SNMP", "-", "OID 解析器缓存重建完成: %d 节点", totalNodes)

	return nil
}

// WarmUpCache 预热缓存
// 将一组节点批量转换并存入缓存，并附带模块名与子节点关系，实现 0 查询解析
func (r *OIDResolver) WarmUpCache(nodes []models.MIBNode, moduleName string) {
	if len(nodes) == 0 {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// 提前在内存中计算所有的父子关系，避免每次查询数据库
	childrenMap := make(map[string][]string)
	for i := range nodes {
		parentOID := nodes[i].ParentOID
		if parentOID != "" {
			childrenMap[parentOID] = append(childrenMap[parentOID], nodes[i].OID)
		}
	}

	for i := range nodes {
		node := &nodes[i]
		
		result := &ResolvedOID{
			ID:          node.ID,
			OID:         node.OID,
			Name:        node.Name,
			Description: node.Description,
			Type:        node.Syntax,
			Access:      node.Access,
			Status:      node.Status,
			ParentOID:   node.ParentOID,
			Children:    childrenMap[node.OID],
			Found:       true,
		}

		if node.ModuleID != nil {
			result.ModuleID = *node.ModuleID
			result.ModuleName = moduleName
		}

		if result.Children == nil {
			result.Children = []string{}
		}

		r.oidCache.Add(node.OID, result)
		r.nameCache.Add(node.Name, node.OID)
	}

	logger.Debug("SNMP", "-", "OID 解析器缓存预热完成: 模块=%s, 节点数=%d", moduleName, len(nodes))
}

// ============================================================================
// 高级查询方法
// ============================================================================

// SearchNodes 搜索 MIB 节点（按名称或 OID 模糊匹配）
// 采用无锁查询 + 写锁缓存模式，避免在持有锁期间执行数据库查询。
func (r *OIDResolver) SearchNodes(query string) ([]*ResolvedOID, error) {
	if query == "" {
		return []*ResolvedOID{}, nil
	}

	// 无锁查询数据库
	nodes, err := r.repo.SearchNodes(query)
	if err != nil {
		return nil, fmt.Errorf("搜索节点失败: %v", err)
	}

	// 在锁外构建解析结果
	results := make([]*ResolvedOID, 0, len(nodes))
	for i := range nodes {
		result := r.buildResolvedOID(&nodes[i])
		result.Found = true
		results = append(results, result)
	}

	// 加写锁缓存搜索结果
	r.mu.Lock()
	for i, result := range results {
		r.oidCache.Add(nodes[i].OID, result)
		r.nameCache.Add(nodes[i].Name, nodes[i].OID)
	}
	r.mu.Unlock()

	return results, nil
}

// GetNodeByID 通过节点 ID 获取解析结果
func (r *OIDResolver) GetNodeByID(id uint) (*ResolvedOID, error) {
	// 无锁查询数据库
	node, err := r.repo.GetNodeByID(id)
	if err != nil {
		return nil, fmt.Errorf("查询节点失败: %v", err)
	}

	if node == nil {
		return nil, fmt.Errorf("节点不存在: ID %d", id)
	}

	result := r.buildResolvedOID(node)
	result.Found = true

	return result, nil
}

// GetModuleNodes 获取指定模块的所有节点
func (r *OIDResolver) GetModuleNodes(moduleID uint) ([]*ResolvedOID, error) {
	// 无锁查询数据库
	nodes, err := r.repo.GetNodesByModule(moduleID)
	if err != nil {
		return nil, fmt.Errorf("查询模块节点失败: %v", err)
	}

	results := make([]*ResolvedOID, 0, len(nodes))
	for i := range nodes {
		result := r.buildResolvedOID(&nodes[i])
		result.Found = true
		results = append(results, result)
	}

	return results, nil
}

// ResolveWithContext 带上下文的 OID 解析（支持取消）
func (r *OIDResolver) ResolveWithContext(ctx context.Context, oid string) (*ResolvedOID, error) {
	// 检查上下文是否已取消
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	return r.ResolveOID(oid)
}

// ResolveBatchWithContext 带上下文的批量解析（支持取消）
func (r *OIDResolver) ResolveBatchWithContext(ctx context.Context, oids []string) ([]*ResolvedOID, error) {
	if len(oids) == 0 {
		return []*ResolvedOID{}, nil
	}

	results := make([]*ResolvedOID, 0, len(oids))

	for _, oid := range oids {
		// 检查上下文是否已取消
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		result, err := r.ResolveOID(oid)
		if err != nil {
			results = append(results, &ResolvedOID{
				OID:   oid,
				Name:  oid,
				Found: false,
			})
			continue
		}
		results = append(results, result)
	}

	return results, nil
}
