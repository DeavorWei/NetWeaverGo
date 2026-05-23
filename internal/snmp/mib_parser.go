// Package snmp 提供 SNMP 核心业务功能
// mib_parser.go 实现 MIB 文件解析，封装 gomib 库（CGO-Free）
package snmp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/golangsnmp/gomib"
	"github.com/golangsnmp/gomib/mib"

	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
)

// MIBParser MIB 文件解析器
// 使用 gomib 库解析 SMIv1/SMIv2 格式的 MIB 文件
// 支持部分导入策略：即使部分节点解析失败，已成功的节点仍保留
// gomib 是纯 Go 实现，无 CGO 依赖，天然支持并发
type MIBParser struct {
	mu        sync.Mutex
	loadedMib *mib.Mib // 已加载的 MIB 数据
}

// NewMIBParser 创建 MIB 解析器实例
func NewMIBParser() *MIBParser {
	return &MIBParser{}
}

// Init 初始化解析器（保持接口兼容，实际无操作）
// gomib 无全局状态，无需初始化
func (p *MIBParser) Init() {
	// gomib 无全局状态，无需初始化
	// 保留空实现以兼容 MIBManager
}

// Exit 清理解析器资源（保持接口兼容，实际无操作）
// gomib 无全局状态，无需清理
func (p *MIBParser) Exit() {
	// gomib 无全局状态，无需清理
	// 保留空实现以兼容 MIBManager
}

// ParseFile 解析单个 MIB 文件
// 返回解析结果，包含成功解析的节点和错误列表
// 即使有错误，已成功解析的节点仍会返回（部分导入策略）
func (p *MIBParser) ParseFile(filePath string) (*MIBImportResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.parseFileLocked(context.Background(), filePath, nil)
}

// ParseFileWithDependencies 解析 MIB 文件及其依赖
// 先设置依赖搜索路径，再加载目标模块
func (p *MIBParser) ParseFileWithDependencies(filePath string, dependencyDirs []string) (*MIBImportResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.parseFileLocked(context.Background(), filePath, dependencyDirs)
}

// parseFileLocked 解析 MIB 文件的内部实现（调用方需持有锁）
func (p *MIBParser) parseFileLocked(ctx context.Context, filePath string, dependencyDirs []string) (*MIBImportResult, error) {
	parseStartTime := time.Now()
	logger.Info("SNMP-Parser", "-", "开始解析 MIB 文件: 路径=%s", filePath)

	// 1. 检查文件存在性
	if _, err := os.Stat(filePath); err != nil {
		logger.Error("SNMP-Parser", "-", "MIB 文件不存在: %s", filePath)
		return nil, fmt.Errorf("MIB 文件不存在: %s", filePath)
	}

	// 2. 构建加载选项
	fileSrc, err := gomib.File(filePath)
	if err != nil {
		logger.Error("SNMP-Parser", "-", "创建 MIB 文件源失败: %s, %v", filePath, err)
		return nil, fmt.Errorf("创建 MIB 文件源失败: %v", err)
	}
	opts := []gomib.LoadOption{
		gomib.WithSource(fileSrc),
	}

	// 3. 添加依赖搜索路径
	if len(dependencyDirs) > 0 {
		logger.Debug("SNMP-Parser", "-", "添加依赖搜索路径: 数量=%d", len(dependencyDirs))
		var sources []gomib.Source
		for _, dir := range dependencyDirs {
			src, err := gomib.Dir(dir)
			if err == nil {
				sources = append(sources, src)
				logger.Debug("SNMP-Parser", "-", "依赖路径添加成功: %s", dir)
			} else {
				logger.Warn("SNMP-Parser", "-", "依赖路径添加失败: %s, %v", dir, err)
			}
		}
		if len(sources) > 0 {
			opts = append(opts, gomib.WithSource(gomib.Multi(sources...)))
		}
	}

	// 4. 加载 MIB 文件
	loadStartTime := time.Now()
	loadedMib, err := gomib.Load(ctx, opts...)
	if err != nil {
		logger.Error("SNMP-Parser", "-", "加载 MIB 文件失败: %s, 耗时=%v, 错误=%v",
			filePath, time.Since(loadStartTime), err)
		return nil, fmt.Errorf("解析 MIB 文件失败: %s: %v", filePath, err)
	}
	logger.Debug("SNMP-Parser", "-", "MIB 加载完成: 耗时=%v", time.Since(loadStartTime))

	// 5. 保存加载结果
	p.loadedMib = loadedMib

	// 6. 转换为内部数据结构
	result := p.parseMibData(loadedMib, filePath)

	totalLatency := time.Since(parseStartTime)
	logger.Info("SNMP-Parser", "-", "MIB 解析完成: 文件=%s, 模块=%s, 节点数=%d, 错误数=%d, 总耗时=%v",
		filePath, result.Module.Name, result.NodeCount, result.ErrorCount, totalLatency)

	// 打印解析错误详情
	if len(result.Errors) > 0 {
		for _, parseErr := range result.Errors {
			if parseErr.NodeName != "" {
				logger.Warn("SNMP-Parser", "-", "MIB 解析错误: 节点=%s, 消息=%s",
					parseErr.NodeName, parseErr.Message)
			} else {
				logger.Warn("SNMP-Parser", "-", "MIB 解析错误: 消息=%s", parseErr.Message)
			}
		}
	}

	return result, nil
}

// parseMibData 从 gomib.Mib 提取所有节点
func (p *MIBParser) parseMibData(loadedMib *mib.Mib, filePath string) *MIBImportResult {
	result := &MIBImportResult{
		Errors: []MIBParseError{},
	}

	// 使用 recover 捕获 panic
	defer func() {
		if r := recover(); r != nil {
			result.Errors = append(result.Errors, MIBParseError{
				Message: fmt.Sprintf("解析过程发生异常: %v", r),
			})
		}
	}()

	// 获取所有模块
	modules := loadedMib.Modules()
	if len(modules) == 0 {
		result.Errors = append(result.Errors, MIBParseError{
			Message: "未找到任何 MIB 模块",
		})
		return result
	}

	// 找出与当前解析的文件路径匹配的模块作为主模块
	var primaryModule *mib.Module
	targetAbs, err := filepath.Abs(filePath)
	if err == nil {
		targetAbsLower := strings.ToLower(targetAbs)
		for _, m := range modules {
			mPathAbs, err := filepath.Abs(m.SourcePath())
			if err == nil && targetAbsLower == strings.ToLower(mPathAbs) {
				primaryModule = m
				break
			}
		}
	}

	// 如果未找到路径完全匹配的，则尝试回退到第一个模块
	if primaryModule == nil {
		primaryModule = modules[0]
	}

	// 构建模块模型
	mibModule := &models.MIBModule{
		Name:        primaryModule.Name(),
		FileName:    filepath.Base(filePath),
		Description: primaryModule.Description(),
		Source:      "import",
		FilePath:    filePath,
	}

	// 遍历所有节点
	nodes := []models.MIBNode{}
	parseErrors := []MIBParseError{}

	// 从根节点开始遍历整个 OID 树
	root := loadedMib.Root()
	if root != nil {
		for node := range root.Subtree() {
			// 仅保存当前主模块定义的节点
			if node.Module() == nil || node.Module().Name() != primaryModule.Name() {
				continue
			}
			mibNode, err := p.convertMibNodeToMIBNode(node, nil)
			if err != nil {
				parseErrors = append(parseErrors, MIBParseError{
					NodeName: node.Name(),
					Message:  err.Error(),
				})
				continue
			}
			nodes = append(nodes, *mibNode)
		}
	}

	mibModule.NodeCount = len(nodes)
	result.Module = mibModule
	result.Nodes = nodes
	result.NodeCount = len(nodes)
	result.ErrorCount = len(parseErrors)
	result.Errors = parseErrors

	return result
}

// convertMibNodeToMIBNode 将 mib.Node 转换为 models.MIBNode
func (p *MIBParser) convertMibNodeToMIBNode(node *mib.Node, moduleID *uint) (*models.MIBNode, error) {
	name := node.Name()
	if name == "" {
		return nil, fmt.Errorf("节点名称为空")
	}

	// 获取 OID
	oid := node.OID()
	oidStr := oid.String()
	if oidStr == "" {
		return nil, fmt.Errorf("节点 OID 为空")
	}

	// 计算父 OID
	parentOID := calculateParentOID(oidStr)

	// 获取节点类型
	nodeType := convertKind(node.Kind())

	// 获取访问权限和状态
	access := "not-applicable"
	status := "unknown"
	syntax := ""
	description := node.Description()

	if obj := node.Object(); obj != nil {
		access = convertAccess(obj.Access())
		status = convertStatus(obj.Status())
		if typ := obj.Type(); typ != nil {
			syntax = typ.Name()
		}
		if description == "" {
			description = obj.Description()
		}
	} else if node.IsObjectIdentity() {
		if st, ok := node.ObjectIdentityStatus(); ok {
			status = convertStatus(st)
		}
	} else if notif := node.Notification(); notif != nil {
		status = convertStatus(notif.Status())
		if description == "" {
			description = notif.Description()
		}
	} else if grp := node.Group(); grp != nil {
		status = convertStatus(grp.Status())
		if description == "" {
			description = grp.Description()
		}
	} else if comp := node.Compliance(); comp != nil {
		status = convertStatus(comp.Status())
		if description == "" {
			description = comp.Description()
		}
	} else if capObj := node.Capability(); capObj != nil {
		status = convertStatus(capObj.Status())
		if description == "" {
			description = capObj.Description()
		}
	}

	description = cleanDescription(description)

	return &models.MIBNode{
		ModuleID:    moduleID,
		OID:         oidStr,
		Name:        name,
		ParentOID:   parentOID,
		NodeType:    nodeType,
		Syntax:      syntax,
		Access:      access,
		Status:      status,
		Description: description,
		Source:      "import",
	}, nil
}

// calculateParentOID 计算父 OID
func calculateParentOID(oid string) string {
	parts := strings.Split(oid, ".")
	if len(parts) <= 1 {
		return ""
	}
	return strings.Join(parts[:len(parts)-1], ".")
}

// convertKind 转换节点类型
func convertKind(kind mib.Kind) string {
	switch kind {
	case mib.KindNode:
		return "node"
	case mib.KindScalar:
		return "scalar"
	case mib.KindTable:
		return "table"
	case mib.KindRow:
		return "row"
	case mib.KindColumn:
		return "column"
	case mib.KindNotification:
		return "notification"
	case mib.KindGroup:
		return "group"
	case mib.KindCompliance:
		return "compliance"
	case mib.KindCapability:
		return "capability"
	default:
		return "node"
	}
}

// convertAccess 转换访问权限
func convertAccess(access mib.Access) string {
	switch access {
	case mib.AccessReadOnly:
		return "read-only"
	case mib.AccessReadWrite:
		return "read-write"
	case mib.AccessReadCreate:
		return "read-create"
	case mib.AccessNotAccessible:
		return "not-accessible"
	case mib.AccessAccessibleForNotify:
		return "accessible-for-notify"
	case mib.AccessWriteOnly:
		return "write-only"
	case mib.AccessNotImplemented:
		return "not-implemented"
	default:
		return "unknown"
	}
}

// convertStatus 转换状态
func convertStatus(status mib.Status) string {
	switch status {
	case mib.StatusCurrent:
		return "current"
	case mib.StatusDeprecated:
		return "deprecated"
	case mib.StatusObsolete:
		return "obsolete"
	case mib.StatusMandatory:
		return "mandatory"
	case mib.StatusOptional:
		return "optional"
	default:
		return "unknown"
	}
}

// GetNodeByOID 通过 OID 获取节点信息
// 注意：此方法现在依赖已加载的 MIB 数据
func (p *MIBParser) GetNodeByOID(oidStr string) (*models.MIBNode, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.loadedMib == nil {
		return nil, fmt.Errorf("请先导入 MIB 模块")
	}

	// 解析 OID
	oid, err := mib.ParseOID(oidStr)
	if err != nil {
		return nil, fmt.Errorf("无效的 OID 格式: %s", oidStr)
	}

	// 查找节点
	node := p.loadedMib.Root().LongestPrefix(oid)
	if node == nil {
		return nil, fmt.Errorf("未找到节点: %s", oidStr)
	}

	return p.convertMibNodeToMIBNode(node, nil)
}

// GetNodeByName 通过名称获取节点信息
// 注意：此方法现在依赖已加载的 MIB 数据
func (p *MIBParser) GetNodeByName(name string) (*models.MIBNode, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.loadedMib == nil {
		return nil, fmt.Errorf("请先导入 MIB 模块")
	}

	// 通过名称查找节点
	node := p.loadedMib.Node(name)
	if node == nil {
		return nil, fmt.Errorf("未找到节点: %s", name)
	}

	return p.convertMibNodeToMIBNode(node, nil)
}

// ResolveOID 解析 OID 为名称
// 优雅降级：如果找不到对应节点，返回原始 OID 字符串
func (p *MIBParser) ResolveOID(oidStr string) string {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.loadedMib == nil {
		return oidStr // 未加载 MIB，返回原始 OID
	}

	// 解析 OID
	oid, err := mib.ParseOID(oidStr)
	if err != nil {
		return oidStr // 无效 OID，返回原始字符串
	}

	// 查找节点
	node := p.loadedMib.Root().LongestPrefix(oid)
	if node == nil {
		return oidStr // 未找到节点，返回原始 OID
	}

	name := node.Name()
	if name == "" {
		return oidStr
	}

	return name
}

// ClearModules 清除已加载的模块（用于重新导入）
// gomib 无全局状态，只需清空内部引用
func (p *MIBParser) ClearModules() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.loadedMib = nil
}

// cleanDescription 过滤描述文本中多余的换行和连续空格，同时保留合法的段落换行
func cleanDescription(desc string) string {
	if desc == "" {
		return ""
	}

	// 统一换行符
	desc = strings.ReplaceAll(desc, "\r\n", "\n")
	lines := strings.Split(desc, "\n")
	var cleanedParagraphs []string
	var currentParagraphLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			// 空行：当前段落结束
			if len(currentParagraphLines) > 0 {
				cleanedParagraphs = append(cleanedParagraphs, strings.Join(currentParagraphLines, " "))
				currentParagraphLines = nil
			}
		} else {
			// 非空行：清理行内多余的连续空格和 Tab 并合并到当前段落
			words := strings.Fields(trimmed)
			innerCleaned := strings.Join(words, " ")
			currentParagraphLines = append(currentParagraphLines, innerCleaned)
		}
	}

	if len(currentParagraphLines) > 0 {
		cleanedParagraphs = append(cleanedParagraphs, strings.Join(currentParagraphLines, " "))
	}

	return strings.Join(cleanedParagraphs, "\n\n")
}
