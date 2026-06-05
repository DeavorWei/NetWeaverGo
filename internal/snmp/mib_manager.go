// Package snmp 提供 SNMP 核心业务功能
// mib_manager.go 实现 MIB 生命周期管理，包括导入/删除/缓存/树构建
package snmp

import (
	"context"
	"embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"golang.org/x/sync/errgroup"

	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
	"github.com/NetWeaverGo/core/internal/repository"
)

//go:embed mibs/*.mib
var coreMIBsFS embed.FS

// DefaultLRUCacheSize LRU 缓存默认容量
const DefaultLRUCacheSize = 10000

// currentPhaseToPhase 将 CurrentPhase 转换为对应的 Phase 值
// 用于批量导入进度通知
func currentPhaseToPhase(currentPhase string) string {
	switch currentPhase {
	case "copy", "parse":
		return "parsing"
	case "save":
		return "saving"
	case "cache":
		return "caching"
	case "done":
		return "completed"
	default:
		return currentPhase
	}
}

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
	inFlight     sync.Map             // 防重入映射：保护同一个 MIB 文件不被并发重复导入
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

	// 异步确保内置核心 MIB 已加载
	go mgr.EnsureCoreMIBsLoaded(context.Background())

	return mgr
}

// Close 清理 MIB 管理器资源
func (m *MIBManager) Close() {
	m.parser.Exit()
}

// GetMIBStoreDir 获取 MIB 文件存储目录路径
func (m *MIBManager) GetMIBStoreDir() string {
	return m.mibStoreDir
}

// ============================================================================
// MIB 文件导入
// ============================================================================

// ImportMIBFile 导入 MIB 文件
// 流程：复制文件到存储目录 → 解析 MIB → 存入数据库 → 更新缓存
// 支持部分导入策略：即使部分节点解析失败，已成功的节点仍保留
func (m *MIBManager) ImportMIBFile(ctx context.Context, filePath string, folderID *uint) (*MIBImportResult, error) {
	importStartTime := time.Now()

	// [L2] 防重入锁
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		absPath = filePath
	}
	if _, loaded := m.inFlight.LoadOrStore(absPath, true); loaded {
		logger.Warn("SNMP-MIB", "-", "文件正在导入中，跳过重复请求: %s", filePath)
		return nil, fmt.Errorf("文件正在导入中: %s", filePath)
	}
	defer m.inFlight.Delete(absPath)

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
	storedPath, err := m.copyMIBFile(filePath, folderID)
	if err != nil {
		logger.Error("SNMP-MIB", "-", "复制 MIB 文件失败: %s, %v", filePath, err)
		return nil, fmt.Errorf("复制 MIB 文件失败: %v", err)
	}
	logger.Debug("SNMP-MIB", "-", "文件复制完成: 源=%s, 目标=%s, 耗时=%v", filePath, storedPath, time.Since(copyStartTime))

	// 解析 MIB 文件 (此时无主锁保护，C1)
	parseStartTime := time.Now()
	dependencyDirs := []string{m.mibStoreDir, filepath.Dir(filePath)}
	result, err := m.parser.ParseFileWithDependencies(storedPath, dependencyDirs)
	if err != nil {
		// [E1] 解析失败，删除已复制的文件（软清理失败不阻断报错）
		if remErr := os.Remove(storedPath); remErr != nil {
			logger.Warn("SNMP-MIB", "-", "解析失败后清理临时文件失败: %s, 错误: %v", storedPath, remErr)
		}
		logger.Error("SNMP-MIB", "-", "解析 MIB 文件失败: %s, 耗时=%v, 错误=%v", filePath, time.Since(parseStartTime), err)
		return nil, fmt.Errorf("解析 MIB 文件失败: %v", err)
	}
	logger.Debug("SNMP-MIB", "-", "MIB 解析完成: 模块=%s, 节点数=%d, 错误数=%d, 耗时=%v",
		result.Module.Name, result.NodeCount, result.ErrorCount, time.Since(parseStartTime))

	// [C1] 解析完成后获取主锁，并在整个事务期间保持
	m.mu.Lock()
	defer m.mu.Unlock()

	var oldFilePath string

	// 检查是否已存在同名模块
	existing, _ := m.mibRepo.GetModuleByName(result.Module.Name)
	if existing != nil {
		if existing.IsBuiltIn {
			logger.Info("SNMP-MIB", "-", "检测到同名模块为内置核心库，禁止覆盖跳过: 模块=%s", result.Module.Name)
			if remErr := os.Remove(storedPath); remErr != nil {
				logger.Warn("SNMP-MIB", "-", "跳过覆盖后清理临时文件失败: %s, 错误: %v", storedPath, remErr)
			}
			return nil, fmt.Errorf("禁止覆盖内置核心库: %s", result.Module.Name)
		}

		logger.Info("SNMP-MIB", "-", "检测到同名模块，将覆盖: 模块=%s, 旧ID=%d", result.Module.Name, existing.ID)
		oldFilePath = existing.FilePath
	}

	// [E2] 开启数据库事务
	tx := m.mibRepo.BeginTx()
	if tx.Error != nil {
		if remErr := os.Remove(storedPath); remErr != nil {
			logger.Warn("SNMP-MIB", "-", "开启事务失败后清理临时文件失败: %s", storedPath)
		}
		return nil, fmt.Errorf("开启事务失败: %v", tx.Error)
	}
	repoTx := m.mibRepo.WithTx(tx)

	// [E1, L1] 事务和文件清理 defer
	txCommitted := false
	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			if remErr := os.Remove(storedPath); remErr != nil {
				logger.Warn("SNMP-MIB", "-", "Panic 回滚后清理新文件失败: %v", remErr)
			}
			panic(p)
		} else if !txCommitted {
			tx.Rollback()
			// 回滚时删除新复制的文件
			if remErr := os.Remove(storedPath); remErr != nil {
				logger.Warn("SNMP-MIB", "-", "回滚后清理新文件失败: %v", remErr)
			}
		} else {
			// 成功提交时物理删除旧文件
			if oldFilePath != "" {
				if remErr := os.Remove(oldFilePath); remErr != nil {
					logger.Warn("SNMP-MIB", "-", "提交后删除作废旧文件失败(需手动清理): %s, %v", oldFilePath, remErr)
				}
			}
		}
	}()

	if existing != nil {
		// 删除已存在的模块节点和记录
		if err := repoTx.DeleteNodesByModule(existing.ID); err != nil {
			return nil, fmt.Errorf("覆盖旧节点失败: %v", err)
		}
		if err := repoTx.DeleteModule(existing.ID); err != nil {
			return nil, fmt.Errorf("覆盖旧模块失败: %v", err)
		}
	}


	// 保存模块到数据库
	saveStartTime := time.Now()
	result.Module.FolderID = folderID
	if err := repoTx.SaveModule(result.Module); err != nil {
		logger.Error("SNMP-MIB", "-", "保存 MIB 模块失败: %s, %v", result.Module.Name, err)
		return nil, fmt.Errorf("保存 MIB 模块失败: %v", err)
	}
	logger.Debug("SNMP-MIB", "-", "模块保存完成: ID=%d, 名称=%s, 耗时=%v", result.Module.ID, result.Module.Name, time.Since(saveStartTime))

	// 设置节点的 ModuleID
	moduleID := result.Module.ID
	for i := range result.Nodes {
		result.Nodes[i].ModuleID = &moduleID
	}

	// 检测并创建虚拟父节点
	virtualStartTime := time.Now()
	virtualNodes, err := m.detectAndCreateVirtualParentNodes(ctx, repoTx, result.Nodes)
	if err != nil {
		logger.Warn("SNMP-MIB", "-", "检测虚拟父节点失败: %v", err)
		// 不阻断导入流程，继续执行
	} else if len(virtualNodes) > 0 {
		// 保存虚拟父节点
		if err := repoTx.SaveNodes(virtualNodes); err != nil {
			logger.Warn("SNMP-MIB", "-", "保存虚拟父节点失败: %v", err)
		} else {
			logger.Info("SNMP-MIB", "-", "虚拟父节点创建完成: 数量=%d, 耗时=%v", len(virtualNodes), time.Since(virtualStartTime))
		}
	}

	// 合并虚拟节点（如果真实节点对应的虚拟节点已存在）
	mergeStartTime := time.Now()
	if err := m.mergeVirtualNodes(repoTx, result.Nodes); err != nil {
		logger.Error("SNMP-MIB", "-", "合并虚拟节点遇到严重失败: %v", err)
		return nil, fmt.Errorf("合并虚拟节点失败: %v", err)
	}
	logger.Debug("SNMP-MIB", "-", "虚拟节点合并完成: 耗时=%v", time.Since(mergeStartTime))

	// 过滤掉已经合并保存过的虚拟节点（ID != 0），避免触发 SQLite UNIQUE constraint failed: mib_nodes.id
	newNodes := make([]models.MIBNode, 0, len(result.Nodes))
	for _, node := range result.Nodes {
		if node.ID == 0 {
			newNodes = append(newNodes, node)
		}
	}

	if len(newNodes) > 0 {
		saveNodesStartTime := time.Now()
		if err := repoTx.SaveNodes(newNodes); err != nil {
			logger.Error("SNMP-MIB", "-", "保存 MIB 节点失败: 模块=%s, %v", result.Module.Name, err)
			return nil, fmt.Errorf("保存 MIB 节点失败: %v", err)
		}
		logger.Debug("SNMP-MIB", "-", "节点保存完成: 数量=%d, 耗时=%v", len(newNodes), time.Since(saveNodesStartTime))
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("提交数据库事务失败: %v", err)
	}
	txCommitted = true

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

// ============================================================================
// MIB 批量导入（优化版）
// ============================================================================

// ImportMIBFilesBatch 批量导入 MIB 文件（核心优化方法）
// 采用 5 阶段流水线：复制 → 构建依赖源 → 并发解析 → 批量保存 → 缓存更新
// 通过 errgroup 控制并发度，通过 context 支持取消
//
// 锁策略：
//   - 阶段零（复制）：无锁
//   - 阶段一（构建依赖源）：无锁
//   - 阶段二（并发解析）：无锁（使用不可变的 depSource）
//   - 阶段三（批量保存）：短锁（仅 DB 写入时）
//   - 阶段四（缓存更新）：独立锁（cacheMu）
func (m *MIBManager) ImportMIBFilesBatch(ctx context.Context, filePaths []string, opts MIBBatchImportOptions, notifier EventNotifier) (*MIBBatchImportResult, error) {
	startTime := time.Now()
	result := &MIBBatchImportResult{
		TotalFiles: len(filePaths),
		Results:    make([]FileImportResult, 0, len(filePaths)),
		Errors:     make([]FileImportError, 0),
	}

	if len(filePaths) == 0 {
		return result, nil
	}

	// 设置默认并发度
	if opts.Concurrency <= 0 {
		opts.Concurrency = 4
	}
	// [S1] 添加并发度上限检查，防止资源耗尽
	if opts.Concurrency > 16 {
		opts.Concurrency = 16
	}

	// 生成批次 ID
	batchID := fmt.Sprintf("batch-%d", time.Now().UnixMilli())

	// 调试日志：追踪 folderID 传递
	if opts.FolderID != nil {
		logger.Info("SNMP-MIB", "-", "开始批量导入 MIB 文件: 批次ID=%s, 文件数=%d, 并发度=%d, 目标文件夹ID=%d",
			batchID, len(filePaths), opts.Concurrency, *opts.FolderID)
	} else {
		logger.Info("SNMP-MIB", "-", "开始批量导入 MIB 文件: 批次ID=%s, 文件数=%d, 并发度=%d, 目标文件夹ID=nil",
			batchID, len(filePaths), opts.Concurrency)
	}

	// === 阶段零：批量复制文件（无锁） ===
	if notifier != nil {
		notifier.NotifyMIBImportProgress(MIBImportProgress{
			BatchID:      batchID,
			TotalFiles:   len(filePaths),
			Phase:        currentPhaseToPhase("copy"),
			CurrentPhase: "copy",
			Progress:     5, // 复制阶段 5%
			Message:      "正在复制 MIB 文件...",
		})
	}

	copiedPaths := make([]string, 0, len(filePaths))
	copyErrors := make([]FileImportError, 0)

	for _, fp := range filePaths {
		dstPath, err := m.copyMIBFile(fp, opts.FolderID)
		if err != nil {
			copyErrors = append(copyErrors, FileImportError{
				FileName:  filepath.Base(fp),
				Error:     err.Error(),
				ErrorType: "copy",
			})
			result.FailedCount++
			continue
		}
		copiedPaths = append(copiedPaths, dstPath)
	}

	// 记录复制错误
	if len(copyErrors) > 0 {
		result.Errors = append(result.Errors, copyErrors...)
		logger.Warn("SNMP-MIB", "-", "批量复制完成: 成功=%d, 失败=%d",
			len(copiedPaths), len(copyErrors))
	}

	if len(copiedPaths) == 0 {
		result.TotalDuration = time.Since(startTime).Milliseconds()
		return result, fmt.Errorf("所有文件复制失败")
	}

	// === 阶段一：预构建依赖源（无锁） ===
	if notifier != nil {
		notifier.NotifyMIBImportProgress(MIBImportProgress{
			BatchID:      batchID,
			TotalFiles:   len(filePaths),
			Phase:        currentPhaseToPhase("parse"),
			CurrentPhase: "parse",
			Progress:     8, // 依赖源构建阶段 8%
			Message:      "正在构建 MIB 依赖源...",
		})
	}

	depDirs := opts.DependencyDirs
	if len(depDirs) == 0 {
		targetDir := m.mibStoreDir
		if opts.FolderID != nil {
			folder, err := m.mibRepo.GetFolderByID(*opts.FolderID)
			if err == nil && folder != nil {
				dirName := folder.Name
				if dirName == "内置核心库" {
					dirName = "core"
				}
				targetDir = filepath.Join(m.mibStoreDir, dirName)
			}
		}
		depDirs = []string{m.mibStoreDir, targetDir}
	}
	depSource, err := m.parser.BuildDependencySource(depDirs)
	if err != nil {
		result.TotalDuration = time.Since(startTime).Milliseconds()
		return result, fmt.Errorf("构建依赖源失败: %w", err)
	}
	logger.Debug("SNMP-MIB", "-", "依赖源构建完成: 目录数=%d", len(depDirs))

	// === 阶段二：并发解析（无锁） ===
	if notifier != nil {
		notifier.NotifyMIBImportProgress(MIBImportProgress{
			BatchID:      batchID,
			TotalFiles:   len(copiedPaths),
			Phase:        currentPhaseToPhase("parse"),
			CurrentPhase: "parse",
			Progress:     10, // 解析阶段开始 10%
			Message:      "正在并发解析 MIB 文件...",
		})
	}

	type parseOutput struct {
		filePath   string
		result     *MIBImportResult
		err        error
		duration   time.Duration
	}

	parseOutputs := make([]parseOutput, len(copiedPaths))
	
	// 尝试全局批量解析（优化路径）
	batchStartTime := time.Now()
	batchResults, batchErr := m.parser.ParseFilesBatchConcurrent(ctx, copiedPaths, depSource)

	if batchErr == nil {
		// 成功，将结果转为 parseOutputs 数组
		for i, fp := range copiedPaths {
			if res, ok := batchResults[fp]; ok {
				parseOutputs[i] = parseOutput{
					filePath: fp,
					result:   res,
					err:      nil,
					duration: time.Since(batchStartTime), // 记录整体耗时即可
				}
			} else {
				parseOutputs[i] = parseOutput{
					filePath: fp,
					result:   nil,
					err:      fmt.Errorf("未在解析结果中找到模块信息"),
					duration: time.Since(batchStartTime),
				}
			}
		}
	} else {
		logger.Warn("SNMP-MIB", "-", "批量解析失败(可能存在语法错误的坏文件), 降级回独立并发解析: %v", batchErr)
		// 降级回独立并发解析
		g, gctx := errgroup.WithContext(ctx)
		g.SetLimit(opts.Concurrency)

		for i, fp := range copiedPaths {
			i, fp := i, fp // 捕获循环变量
			g.Go(func() error {
				start := time.Now()
				parseResult, err := m.parser.ParseFileConcurrent(gctx, fp, depSource)
				parseOutputs[i] = parseOutput{
					filePath: fp,
					result:   parseResult,
					err:      err,
					duration: time.Since(start),
				}
				return nil // 不返回错误，让其他 goroutine 继续
			})
		}
		g.Wait() // 忽略 group 错误，我们通过 parseOutputs 收集结果
	}

	// 收集成功的解析结果
	type moduleNodes struct {
		moduleName string
		nodes      []models.MIBNode
	}
	var allModuleNodes []moduleNodes
	parsedCount := 0

	for _, po := range parseOutputs {
		fileName := filepath.Base(po.filePath)
		if po.err != nil {
			errType := "parse"
			if strings.Contains(po.err.Error(), "dependency") || strings.Contains(po.err.Error(), "依赖") {
				errType = "dependency"
			}
			result.Errors = append(result.Errors, FileImportError{
				FileName:  fileName,
				Error:     po.err.Error(),
				ErrorType: errType,
			})
			result.FailedCount++
			_ = os.Remove(po.filePath) // 解析失败，清理已物理复制的文件
			continue
		}

		result.Results = append(result.Results, FileImportResult{
			FileName:   fileName,
			ModuleName: po.result.Module.Name,
			NodeCount:  len(po.result.Nodes),
			Duration:   po.duration.Milliseconds(),
			Status:     "success",
		})
		result.SuccessCount++

		allModuleNodes = append(allModuleNodes, moduleNodes{
			moduleName: po.result.Module.Name,
			nodes:      po.result.Nodes,
		})
		parsedCount++

		if notifier != nil {
			// 计算进度百分比：解析阶段占 10%-60%
			progress := 10.0 + float64(parsedCount)/float64(len(copiedPaths))*50.0
			notifier.NotifyMIBImportProgress(MIBImportProgress{
				BatchID:        batchID,
				TotalFiles:     len(copiedPaths),
				ProcessedFiles: parsedCount,
				Phase:          currentPhaseToPhase("parse"),
				CurrentPhase:   "parse",
				Progress:       progress,
				Message:        fmt.Sprintf("已解析 %d/%d 文件", parsedCount, len(copiedPaths)),
			})
		}
	}

	logger.Info("SNMP-MIB", "-", "并发解析完成: 成功=%d, 失败=%d, 耗时=%v",
		result.SuccessCount, result.FailedCount, time.Since(startTime))

	// === 阶段三：批量保存节点（单事务优化） ===
	if notifier != nil {
		notifier.NotifyMIBImportProgress(MIBImportProgress{
			BatchID:        batchID,
			TotalFiles:     len(copiedPaths),
			ProcessedFiles: parsedCount,
			Phase:          currentPhaseToPhase("save"),
			CurrentPhase:   "save",
			Progress:       65.0, // 保存阶段开始 65%
			Message:        "正在批量保存 MIB 节点...",
		})
	}

	// [性能优化] 使用单个全局事务包裹所有模块保存，将 N 次 fsync 降为 1 次
	m.mu.Lock()
	globalTx := m.mibRepo.BeginTx()
	if globalTx.Error != nil {
		m.mu.Unlock()
		result.TotalDuration = time.Since(startTime).Milliseconds()
		return result, fmt.Errorf("开启全局事务失败: %v", globalTx.Error)
	}
	globalRepoTx := m.mibRepo.WithTx(globalTx)
	globalTxCommitted := false
	defer func() {
		if !globalTxCommitted {
			globalTx.Rollback()
		}
		m.mu.Unlock()
	}()

	var allNodes []models.MIBNode
	for _, mn := range allModuleNodes {
		// 查找对应的解析结果以获取模块信息
		var moduleInfo *models.MIBModule
		var copiedFilePath string
		for _, po := range parseOutputs {
			if po.result != nil && po.result.Module.Name == mn.moduleName {
				moduleInfo = po.result.Module
				copiedFilePath = po.filePath
				break
			}
		}
		if moduleInfo == nil {
			continue
		}

		// [性能优化] 在全局事务内保存，避免每个模块独立事务开销
		err := m.saveNodesBatchInTx(ctx, globalRepoTx, moduleInfo, mn.nodes, opts.OverwriteExisting, opts.FolderID)

		// 如果保存失败，或者因为不覆盖而跳过保存 (ID 仍为 0)，清理已物理复制的文件
		if err != nil || moduleInfo.ID == 0 {
			if copiedFilePath != "" {
				_ = os.Remove(copiedFilePath)
			}
		}

		if err != nil {
			result.Errors = append(result.Errors, FileImportError{
				FileName:  mn.moduleName,
				Error:     err.Error(),
				ErrorType: "database",
			})
			result.FailedCount++
			// [M3] 防止 SuccessCount 变为负数
			if result.SuccessCount > 0 {
				result.SuccessCount--
			}
			continue
		}
		allNodes = append(allNodes, mn.nodes...)
	}

	// 统一提交全局事务
	if err := globalTx.Commit().Error; err != nil {
		result.TotalDuration = time.Since(startTime).Milliseconds()
		return result, fmt.Errorf("提交全局事务失败: %v", err)
	}
	globalTxCommitted = true

	// === 阶段四：缓存更新（独立锁） ===
	if notifier != nil {
		notifier.NotifyMIBImportProgress(MIBImportProgress{
			BatchID:        batchID,
			TotalFiles:     len(copiedPaths),
			ProcessedFiles: len(copiedPaths),
			Phase:          currentPhaseToPhase("cache"),
			CurrentPhase:   "cache",
			Progress:       85.0, // 缓存阶段 85%
			Message:        "正在更新 MIB 缓存...",
		})
	}

	m.UpdateCacheBatch(allNodes)

	// 计算跳过数
	result.SkippedCount = result.TotalFiles - result.SuccessCount - result.FailedCount
	result.TotalDuration = time.Since(startTime).Milliseconds()

	logger.Info("SNMP-MIB", "-", "批量导入完成: 成功=%d, 失败=%d, 跳过=%d, 总耗时=%dms",
		result.SuccessCount, result.FailedCount, result.SkippedCount, result.TotalDuration)

	if notifier != nil {
		notifier.NotifyMIBImportProgress(MIBImportProgress{
			BatchID:        batchID,
			TotalFiles:     len(copiedPaths),
			ProcessedFiles: len(copiedPaths),
			Phase:          currentPhaseToPhase("done"),
			CurrentPhase:   "done",
			Progress:       100.0, // 完成 100%
			Message:        fmt.Sprintf("批量导入完成，成功 %d，失败 %d，耗时 %dms", result.SuccessCount, result.FailedCount, result.TotalDuration),
		})
	}

	return result, nil
}

// saveNodesBatchInTx 在已有事务内保存单个模块的节点（不开启新事务、不加锁）
// 专供 ImportMIBFilesBatch 的全局事务调用，避免每个模块独立事务的 fsync 开销
func (m *MIBManager) saveNodesBatchInTx(ctx context.Context, repoTx repository.MIBRepository, module *models.MIBModule, nodes []models.MIBNode, overwrite bool, folderID *uint) error {
	if len(nodes) == 0 || module == nil {
		return nil
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	existing, _ := repoTx.GetModuleByName(module.Name)
	if existing != nil {
		if existing.IsBuiltIn {
			logger.Info("SNMP-MIB", "-", "检测到同名模块为内置核心库，禁止覆盖跳过: 模块=%s", module.Name)
			return fmt.Errorf("禁止覆盖内置核心库: %s", module.Name)
		}

		if overwrite {
			logger.Info("SNMP-MIB", "-", "检测到同名模块，将覆盖: 模块=%s, 旧ID=%d", module.Name, existing.ID)
		} else {
			logger.Info("SNMP-MIB", "-", "检测到同名模块，跳过: 模块=%s", module.Name)
			return nil
		}
	}

	if existing != nil && overwrite {
		if err := repoTx.DeleteNodesByModule(existing.ID); err != nil {
			return fmt.Errorf("删除旧节点失败: %w", err)
		}
		if err := repoTx.DeleteModule(existing.ID); err != nil {
			return fmt.Errorf("删除旧模块失败: %w", err)
		}
	}

	// 设置文件夹 ID 并保存模块到数据库
	module.ID = 0
	module.FolderID = folderID
	if folderID != nil {
		logger.Debug("SNMP-MIB", "-", "SaveNodesBatch: 模块 %s 设置 FolderID=%d", module.Name, *folderID)
	} else {
		logger.Debug("SNMP-MIB", "-", "SaveNodesBatch: 模块 %s 设置 FolderID=nil", module.Name)
	}
	if err := repoTx.SaveModule(module); err != nil {
		return fmt.Errorf("保存 MIB 模块失败: %w", err)
	}
	logger.Debug("SNMP-MIB", "-", "模块保存完成: ID=%d, 名称=%s, FolderID=%v", module.ID, module.Name, module.FolderID)

	moduleID := module.ID
	for i := range nodes {
		nodes[i].ID = 0
		nodes[i].ModuleID = &moduleID
	}

	virtualNodes, err := m.detectAndCreateVirtualParentNodes(ctx, repoTx, nodes)
	if err != nil {
		logger.Warn("SNMP-MIB", "-", "检测虚拟父节点失败: %v", err)
	} else if len(virtualNodes) > 0 {
		if err := repoTx.SaveNodes(virtualNodes); err != nil {
			logger.Warn("SNMP-MIB", "-", "保存虚拟父节点失败: %v", err)
		} else {
			logger.Info("SNMP-MIB", "-", "虚拟父节点创建完成: 数量=%d", len(virtualNodes))
		}
	}

	if err := m.mergeVirtualNodes(repoTx, nodes); err != nil {
		return fmt.Errorf("合并虚拟节点遇到严重失败: %v", err)
	}

	newNodes := make([]models.MIBNode, 0, len(nodes))
	for _, node := range nodes {
		if node.ID == 0 {
			newNodes = append(newNodes, node)
		}
	}

	if len(newNodes) > 0 {
		if err := repoTx.SaveNodes(newNodes); err != nil {
			return fmt.Errorf("保存 MIB 节点失败: %w", err)
		}
		logger.Debug("SNMP-MIB", "-", "节点保存完成: 数量=%d", len(newNodes))
	}

	return nil
}

// SaveNodesBatch 批量保存节点
func (m *MIBManager) SaveNodesBatch(ctx context.Context, module *models.MIBModule, nodes []models.MIBNode, overwrite bool, folderID *uint) error {
	if len(nodes) == 0 || module == nil {
		return nil
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	var oldFilePath string

	existing, _ := m.mibRepo.GetModuleByName(module.Name)
	if existing != nil {
		if existing.IsBuiltIn {
			logger.Info("SNMP-MIB", "-", "检测到同名模块为内置核心库，禁止覆盖跳过: 模块=%s", module.Name)
			return fmt.Errorf("禁止覆盖内置核心库: %s", module.Name)
		}

		if overwrite {
			logger.Info("SNMP-MIB", "-", "检测到同名模块，将覆盖: 模块=%s, 旧ID=%d", module.Name, existing.ID)
			oldFilePath = existing.FilePath
		} else {
			logger.Info("SNMP-MIB", "-", "检测到同名模块，跳过: 模块=%s", module.Name)
			return nil
		}
	}

	tx := m.mibRepo.BeginTx()
	if tx.Error != nil {
		return fmt.Errorf("开启事务失败: %v", tx.Error)
	}
	repoTx := m.mibRepo.WithTx(tx)

	txCommitted := false
	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p)
		} else if !txCommitted {
			tx.Rollback()
		} else {
			if oldFilePath != "" {
				if remErr := os.Remove(oldFilePath); remErr != nil {
					logger.Warn("SNMP-MIB", "-", "提交后删除作废旧文件失败(需手动清理): %s, %v", oldFilePath, remErr)
				}
			}
		}
	}()

	if existing != nil && overwrite {
		if err := repoTx.DeleteNodesByModule(existing.ID); err != nil {
			return fmt.Errorf("删除旧节点失败: %w", err)
		}
		if err := repoTx.DeleteModule(existing.ID); err != nil {
			return fmt.Errorf("删除旧模块失败: %w", err)
		}
	}

	// 2. 设置文件夹 ID 并保存模块到数据库
	module.ID = 0 // 确保是新记录
	module.FolderID = folderID // 设置目标文件夹
	// 调试日志：追踪 folderID 设置
	if folderID != nil {
		logger.Debug("SNMP-MIB", "-", "SaveNodesBatch: 模块 %s 设置 FolderID=%d", module.Name, *folderID)
	} else {
		logger.Debug("SNMP-MIB", "-", "SaveNodesBatch: 模块 %s 设置 FolderID=nil", module.Name)
	}
	if err := repoTx.SaveModule(module); err != nil {
		return fmt.Errorf("保存 MIB 模块失败: %w", err)
	}
	logger.Debug("SNMP-MIB", "-", "模块保存完成: ID=%d, 名称=%s, FolderID=%v", module.ID, module.Name, module.FolderID)

	moduleID := module.ID
	for i := range nodes {
		nodes[i].ID = 0 // 确保是新记录
		nodes[i].ModuleID = &moduleID
	}

	virtualNodes, err := m.detectAndCreateVirtualParentNodes(ctx, repoTx, nodes)
	if err != nil {
		logger.Warn("SNMP-MIB", "-", "检测虚拟父节点失败: %v", err)
	} else if len(virtualNodes) > 0 {
		if err := repoTx.SaveNodes(virtualNodes); err != nil {
			logger.Warn("SNMP-MIB", "-", "保存虚拟父节点失败: %v", err)
		} else {
			logger.Info("SNMP-MIB", "-", "虚拟父节点创建完成: 数量=%d", len(virtualNodes))
		}
	}

	if err := m.mergeVirtualNodes(repoTx, nodes); err != nil {
		return fmt.Errorf("合并虚拟节点遇到严重失败: %v", err)
	}

	newNodes := make([]models.MIBNode, 0, len(nodes))
	for _, node := range nodes {
		if node.ID == 0 {
			newNodes = append(newNodes, node)
		}
	}

	if len(newNodes) > 0 {
		if err := repoTx.SaveNodes(newNodes); err != nil {
			return fmt.Errorf("保存 MIB 节点失败: %w", err)
		}
		logger.Debug("SNMP-MIB", "-", "节点保存完成: 数量=%d", len(newNodes))
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("提交数据库事务失败: %v", err)
	}
	txCommitted = true

	return nil
}

// UpdateCacheBatch 批量更新缓存（独立锁）
// 使用独立的 cacheMu 锁，避免阻塞主锁 mu
func (m *MIBManager) UpdateCacheBatch(nodes []models.MIBNode) {
	if len(nodes) == 0 {
		return
	}

	m.cacheMu.Lock()
	defer m.cacheMu.Unlock()

	addedCount := 0
	for i := range nodes {
		if nodes[i].OID != "" {
			m.nodeCache.Add(nodes[i].OID, &nodes[i])
			addedCount++
		}
		if nodes[i].Name != "" {
			m.nameCache.Add(nodes[i].Name, nodes[i].OID)
		}
	}

	logger.Debug("SNMP-MIB", "-", "缓存批量更新完成: 新增=%d, 总计=%d", addedCount, m.nodeCache.Len())
}

// copyMIBFile 复制 MIB 文件到存储目录
func (m *MIBManager) copyMIBFile(srcPath string, folderID *uint) (string, error) {
	targetDir := m.mibStoreDir
	if folderID != nil {
		folder, err := m.mibRepo.GetFolderByID(*folderID)
		if err == nil && folder != nil {
			dirName := folder.Name
			if dirName == "内置核心库" {
				dirName = "core"
			}
			targetDir = filepath.Join(m.mibStoreDir, dirName)
		}
	}

	// 确保存储目录存在
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return "", fmt.Errorf("创建 MIB 存储目录失败: %v", err)
	}

	fileName := filepath.Base(srcPath)
	dstPath := filepath.Join(targetDir, fileName)

	// 如果目标文件已存在，添加序号
	counter := 1
	for {
		if _, err := os.Stat(dstPath); os.IsNotExist(err) {
			break
		}
		ext := filepath.Ext(fileName)
		base := fileName[:len(fileName)-len(ext)]
		dstPath = filepath.Join(targetDir, fmt.Sprintf("%s_%d%s", base, counter, ext))
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

	// [S1-S2] 更新缓存：使用 cacheMu 锁保护缓存操作
	m.cacheMu.Lock()
	m.nodeCache.Add(node.OID, node)
	m.nameCache.Add(node.Name, node.OID)
	m.cacheMu.Unlock()

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

	// [S1-S2] 更新缓存：使用 cacheMu 锁保护缓存操作
	m.cacheMu.Lock()
	m.nodeCache.Add(existing.OID, existing)
	m.nameCache.Add(existing.Name, existing.OID)
	m.cacheMu.Unlock()

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

	// [S1-S2] 清除缓存：使用 cacheMu 锁保护缓存操作
	m.cacheMu.Lock()
	m.nodeCache.Remove(node.OID)
	m.nameCache.Remove(node.Name)
	m.cacheMu.Unlock()

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

	// [S1-S2] 清除缓存：使用 cacheMu 锁保护缓存操作
	m.cacheMu.Lock()
	for _, node := range nodes {
		m.nodeCache.Remove(node.OID)
		m.nameCache.Remove(node.Name)
	}
	m.cacheMu.Unlock()

	logger.Info("SNMP", "-", "MIB 模块已删除: %s", module.Name)

	return nil
}

// ============================================================================
// 查询操作
// ============================================================================

// ResolveOID 解析 OID 为可读名称
// 如果用户未导入对应 MIB，返回原始 OID 字符串（优雅降级，不报错）
func (m *MIBManager) ResolveOID(oid string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 先查缓存
	if cached, ok := m.nodeCache.Get(oid); ok {
		return cached.Name
	}

	// 查数据库
	node, err := m.mibRepo.GetNodeByOID(oid)
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
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 先查缓存
	if cached, ok := m.nameCache.Get(name); ok {
		return cached, nil
	}

	// 查数据库
	node, err := m.mibRepo.GetNodeByName(name)
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

	// 收集 OID 进行批量查询（解决 P1: N+1 查询问题）
	parentOIDs := make([]string, 0, len(children))
	for i := range children {
		parentOIDs = append(parentOIDs, children[i].OID)
	}

	childCounts, err := m.mibRepo.CountChildNodesBatch(parentOIDs)
	if err != nil {
		logger.Warn("SNMP-MIB", "-", "批量查询子节点数量失败: %v", err)
		childCounts = make(map[string]int64)
	}

	treeNodes := make([]MIBTreeNode, 0, len(children))
	for i := range children {
		// 直接从批量查询结果中获取
		hasChildren := childCounts[children[i].OID] > 0

		treeNodes = append(treeNodes, MIBTreeNode{
			ID:          children[i].ID,
			OID:         children[i].OID,
			Name:        children[i].Name,
			NodeType:    children[i].NodeType,
			Syntax:      children[i].Syntax,
			Access:      children[i].Access,
			Status:      children[i].Status,
			Description: children[i].Description,
			HasChildren: hasChildren,
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
	// [P2] 缓存重建性能优化：采用分页加载，避免 OOM
	m.cacheMu.Lock()
	defer m.cacheMu.Unlock()

	// 清空现有缓存
	m.nodeCache.Purge()
	m.nameCache.Purge()

	limit := 2000 // 每页加载 2000 条
	offset := 0
	totalNodes := 0

	for {
		m.mu.RLock()
		nodes, err := m.mibRepo.GetNodesBatch(offset, limit)
		m.mu.RUnlock()

		if err != nil {
			return fmt.Errorf("分页加载节点失败: %v", err)
		}

		if len(nodes) == 0 {
			break
		}

		// 填充缓存
		for i := range nodes {
			m.nodeCache.Add(nodes[i].OID, &nodes[i])
			m.nameCache.Add(nodes[i].Name, nodes[i].OID)
		}

		totalNodes += len(nodes)
		offset += limit
	}

	logger.Info("SNMP", "-", "MIB 缓存重建完成: %d 节点", totalNodes)

	return nil
}

// CacheStats 返回缓存统计信息
func (m *MIBManager) CacheStats() (nodeCacheLen, nameCacheLen int) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.nodeCache.Len(), m.nameCache.Len()
}

// ============================================================================
// 虚拟父节点管理
// ============================================================================

// detectAndCreateVirtualParentNodes 检测并创建缺失的父节点
// 当导入的 MIB 节点引用了其他模块的父节点时，创建虚拟父节点以确保树结构完整
// [性能优化] 使用批量查询替代逐条查询，大幅减少数据库 IO
func (m *MIBManager) detectAndCreateVirtualParentNodes(ctx context.Context, repo repository.MIBRepository, nodes []models.MIBNode) ([]models.MIBNode, error) {
	requiredParentOIDs := make(map[string]bool)

	// 收集所有需要的 ParentOID
	for _, node := range nodes {
		if node.ParentOID != "" {
			requiredParentOIDs[node.ParentOID] = true
		}
	}

	// 移除当前批次中已存在的 OID
	nodeOIDSet := make(map[string]bool, len(nodes))
	for _, node := range nodes {
		nodeOIDSet[node.OID] = true
	}
	for oid := range requiredParentOIDs {
		if nodeOIDSet[oid] {
			delete(requiredParentOIDs, oid)
		}
	}

	// [性能优化] 批量查询数据库中已存在的 OID
	parentOIDList := make([]string, 0, len(requiredParentOIDs))
	for oid := range requiredParentOIDs {
		parentOIDList = append(parentOIDList, oid)
	}

	existingSet := make(map[string]bool)
	if len(parentOIDList) > 0 {
		existingNodes, err := repo.GetNodesByOIDs(parentOIDList)
		if err != nil {
			logger.Warn("SNMP-MIB", "-", "批量查询父节点失败: %v", err)
		} else {
			for _, n := range existingNodes {
				existingSet[n.OID] = true
			}
		}
	}

	// 收集需要创建的虚拟节点
	virtualNodes := []models.MIBNode{}
	for oid := range requiredParentOIDs {
		if !existingSet[oid] {
			virtualNode := models.MIBNode{
				OID:         oid,
				Name:        m.generateVirtualNodeName(oid),
				ParentOID:   m.calculateParentOID(oid),
				NodeType:    "node",
				Source:      "virtual",
				ModuleID:    nil,
				Description: "虚拟节点（待实际节点导入后替换）",
			}
			virtualNodes = append(virtualNodes, virtualNode)
		}
	}

	// 递归处理虚拟节点的父节点
	allVirtualNodes := []models.MIBNode{}
	allVirtualNodes = append(allVirtualNodes, virtualNodes...)
	allVirtualOIDSet := make(map[string]bool, len(virtualNodes))
	for _, vn := range virtualNodes {
		allVirtualOIDSet[vn.OID] = true
	}

	// 递归创建祖先虚拟节点
	for len(virtualNodes) > 0 {
		// 收集所有需要检查的祖先 OID
		ancestorOIDsToCheck := make(map[string]bool)
		for _, vnode := range virtualNodes {
			if vnode.ParentOID == "" {
				continue
			}
			if !nodeOIDSet[vnode.ParentOID] && !allVirtualOIDSet[vnode.ParentOID] && !existingSet[vnode.ParentOID] {
				ancestorOIDsToCheck[vnode.ParentOID] = true
			}
		}

		if len(ancestorOIDsToCheck) == 0 {
			break
		}

		// [性能优化] 批量查询祖先 OID
		ancestorOIDList := make([]string, 0, len(ancestorOIDsToCheck))
		for oid := range ancestorOIDsToCheck {
			ancestorOIDList = append(ancestorOIDList, oid)
		}
		ancestorExisting, err := repo.GetNodesByOIDs(ancestorOIDList)
		if err == nil {
			for _, n := range ancestorExisting {
				existingSet[n.OID] = true
			}
		}

		ancestorVirtualNodes := []models.MIBNode{}
		for oid := range ancestorOIDsToCheck {
			if !existingSet[oid] {
				ancestorNode := models.MIBNode{
					OID:         oid,
					Name:        m.generateVirtualNodeName(oid),
					ParentOID:   m.calculateParentOID(oid),
					NodeType:    "node",
					Source:      "virtual",
					ModuleID:    nil,
					Description: "虚拟节点（待实际节点导入后替换）",
				}
				ancestorVirtualNodes = append(ancestorVirtualNodes, ancestorNode)
			}
		}

		if len(ancestorVirtualNodes) > 0 {
			allVirtualNodes = append(allVirtualNodes, ancestorVirtualNodes...)
			for _, vn := range ancestorVirtualNodes {
				allVirtualOIDSet[vn.OID] = true
			}
		}
		virtualNodes = ancestorVirtualNodes
	}

	return allVirtualNodes, nil
}

// generateVirtualNodeName 为虚拟节点生成名称
func (m *MIBManager) generateVirtualNodeName(oid string) string {
	parts := strings.Split(oid, ".")
	if len(parts) > 0 {
		return "virtual_" + parts[len(parts)-1]
	}
	return "virtual_" + strings.ReplaceAll(oid, ".", "_")
}

// calculateParentOID 根据 OID 计算父节点 OID
func (m *MIBManager) calculateParentOID(oid string) string {
	if oid == "" {
		return ""
	}

	parts := strings.Split(oid, ".")
	if len(parts) <= 1 {
		return "" // 根节点没有父节点
	}

	// 移除最后一个部分得到父节点 OID
	return strings.Join(parts[:len(parts)-1], ".")
}

// mergeVirtualNodes 合并虚拟节点与真实节点
// 当导入真实节点时，如果存在对应的虚拟节点，则更新为真实节点
// [性能优化] 使用批量查询替代逐条查询，将 N 次 SELECT 降为 1 次
func (m *MIBManager) mergeVirtualNodes(repo repository.MIBRepository, nodes []models.MIBNode) error {
	if len(nodes) == 0 {
		return nil
	}

	// [性能优化] 收集所有 OID，一次性批量查询
	oids := make([]string, len(nodes))
	for i, n := range nodes {
		oids[i] = n.OID
	}

	existingNodes, err := repo.GetNodesByOIDs(oids)
	if err != nil {
		logger.Warn("SNMP-MIB", "-", "批量查询已有节点失败: %v", err)
		return nil // 查询失败时不阻断导入，仅跳过合并
	}

	// 构建 OID → 虚拟节点 映射（仅 source=virtual 的节点）
	virtualMap := make(map[string]*models.MIBNode, len(existingNodes))
	for i := range existingNodes {
		if existingNodes[i].Source == "virtual" {
			virtualMap[existingNodes[i].OID] = &existingNodes[i]
		}
	}

	if len(virtualMap) == 0 {
		return nil // 没有虚拟节点需要合并
	}

	// 在内存中匹配并合并
	var mergeErrors []error
	for i := range nodes {
		existing, ok := virtualMap[nodes[i].OID]
		if !ok {
			continue
		}

		// 更新虚拟节点为真实节点
		existing.Name = nodes[i].Name
		existing.NodeType = nodes[i].NodeType
		existing.Syntax = nodes[i].Syntax
		existing.Access = nodes[i].Access
		existing.Status = nodes[i].Status
		existing.Description = nodes[i].Description
		existing.Source = nodes[i].Source
		existing.ModuleID = nodes[i].ModuleID

		if err := repo.SaveNode(existing); err != nil {
			logger.Warn("SNMP-MIB", "-", "合并虚拟节点失败: OID=%s, 错误=%v", nodes[i].OID, err)
			mergeErrors = append(mergeErrors, fmt.Errorf("OID=%s 错误=%v", nodes[i].OID, err))
			continue
		}

		// 更新节点引用，使用已存在的 ID
		nodes[i].ID = existing.ID
		logger.Info("SNMP-MIB", "-", "虚拟节点已合并: OID=%s, 名称=%s", nodes[i].OID, nodes[i].Name)
	}
	if len(mergeErrors) > 0 {
		return fmt.Errorf("合并虚拟节点发生 %d 个错误，如: %v", len(mergeErrors), mergeErrors[0])
	}
	return nil
}
// ============================================================================
// 内置核心库加载
// ============================================================================

// EnsureCoreMIBsLoaded 确保内置核心 MIB 库已加载
// 遍历 embed 目录中的 mibs，如果发现未导入则自动导入并打上 IsBuiltIn 标记
func (m *MIBManager) EnsureCoreMIBsLoaded(ctx context.Context) {
	logger.Info("SNMP-MIB", "-", "开始检查内置核心 MIB 库...")
	entries, err := coreMIBsFS.ReadDir("mibs")
	if err != nil {
		logger.Error("SNMP-MIB", "-", "读取内置 MIB 目录失败: %v", err)
		return
	}

	// 1. 创建临时目录释放内嵌文件，以便解析器能使用依赖源
	tempDir, err := os.MkdirTemp("", "core-mibs-*")
	if err != nil {
		logger.Error("SNMP-MIB", "-", "创建内置库临时解压目录失败: %v", err)
		return
	}
	defer os.RemoveAll(tempDir)

	var filePaths []string
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".mib" {
			continue
		}
		fileName := entry.Name()

		// 新增提前对比逻辑：推测模块名等于无后缀的文件名
		moduleName := strings.TrimSuffix(fileName, ".mib")
		existingMod, err := m.mibRepo.GetModuleByName(moduleName)
		if err == nil && existingMod != nil && existingMod.IsBuiltIn {
			// 如果数据库里已经有了这个内置库，检查物理文件是否被手动删除
			targetPath := filepath.Join(m.mibStoreDir, "core", fileName)
			if _, err := os.Stat(targetPath); err == nil {
				continue // 物理文件存在，跳过
			}
			// 物理文件丢失，从内置 FS 恢复
			content, err := coreMIBsFS.ReadFile("mibs/" + fileName)
			if err == nil {
				os.MkdirAll(filepath.Join(m.mibStoreDir, "core"), 0755)
				os.WriteFile(targetPath, content, 0644)
				logger.Info("SNMP-MIB", "-", "恢复丢失的内置核心库物理文件: %s", fileName)
			}
			continue
		}

		content, err := coreMIBsFS.ReadFile("mibs/" + fileName)
		if err != nil {
			logger.Warn("SNMP-MIB", "-", "读取内置文件失败 %s: %v", fileName, err)
			continue
		}

		tmpFilePath := filepath.Join(tempDir, fileName)
		if err := os.WriteFile(tmpFilePath, content, 0644); err != nil {
			logger.Warn("SNMP-MIB", "-", "释放内置文件失败 %s: %v", fileName, err)
			continue
		}
		filePaths = append(filePaths, tmpFilePath)
	}

	if len(filePaths) == 0 {
		return
	}

	// 获取或创建“内置核心库”文件夹
	folderName := "内置核心库"
	folder, err := m.mibRepo.GetFolderByName(folderName)
	if err != nil || folder == nil {
		folder = &models.MIBFolder{
			Name: folderName,
		}
		if err := m.mibRepo.SaveFolder(folder); err != nil {
			logger.Error("SNMP-MIB", "-", "创建内置核心库文件夹失败: %v", err)
			return
		}
	}

	// 2. 批量导入
	opts := MIBBatchImportOptions{
		Concurrency:       4,
		SkipErrors:        true,
		OverwriteExisting: false, // 已经存在的就不覆盖了
		DependencyDirs:    []string{tempDir},
		FolderID:          &folder.ID,
	}

	result, err := m.ImportMIBFilesBatch(ctx, filePaths, opts, nil)
	if err != nil {
		logger.Error("SNMP-MIB", "-", "内置核心 MIB 库导入失败: %v", err)
		return
	}

	// 3. 将相关模块标记为内置库
	m.mu.Lock()
	for _, res := range result.Results {
		if res.Status == "success" {
			mod, err := m.mibRepo.GetModuleByName(res.ModuleName)
			if err == nil && mod != nil {
				mod.IsBuiltIn = true
				_ = m.mibRepo.SaveModule(mod)
			}
		}
	}
	m.mu.Unlock()

	logger.Info("SNMP-MIB", "-", "内置核心 MIB 库检查完成: 成功=%d, 失败=%d, 跳过=%d",
		result.SuccessCount, result.FailedCount, result.SkippedCount)
}
