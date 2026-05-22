// Package snmp 提供 SNMP 核心业务功能
// mib_parser.go 实现 MIB 文件解析，封装 gosmi 库
package snmp

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	gosmi "github.com/sleepinggenius2/gosmi"
	"github.com/sleepinggenius2/gosmi/types"

	"github.com/NetWeaverGo/core/internal/models"
)

// MIBParser MIB 文件解析器
// 使用 gosmi 库解析 SMIv1/SMIv2 格式的 MIB 文件
// 支持部分导入策略：即使部分节点解析失败，已成功的节点仍保留
type MIBParser struct {
	mu sync.Mutex
}

// NewMIBParser 创建 MIB 解析器实例
func NewMIBParser() *MIBParser {
	return &MIBParser{}
}

// Init 初始化 gosmi 库（应用启动时调用一次）
func (p *MIBParser) Init() {
	gosmi.Init()
}

// Exit 清理 gosmi 库资源（应用关闭时调用）
func (p *MIBParser) Exit() {
	gosmi.Exit()
}

// ParseFile 解析单个 MIB 文件
// 返回解析结果，包含成功解析的节点和错误列表
// 即使有错误，已成功解析的节点仍会返回（部分导入策略）
func (p *MIBParser) ParseFile(filePath string) (*MIBImportResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.parseFileLocked(filePath)
}

// ParseFileWithDependencies 解析 MIB 文件及其依赖
// 先设置依赖搜索路径，再加载目标模块
func (p *MIBParser) ParseFileWithDependencies(filePath string, dependencyDirs []string) (*MIBImportResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 设置 MIB 搜索路径（包含依赖目录）
	for _, dir := range dependencyDirs {
		gosmi.AppendPath(dir)
	}

	return p.parseFileLocked(filePath)
}

// parseFileLocked 解析 MIB 文件的内部实现（调用方需持有锁）
func (p *MIBParser) parseFileLocked(filePath string) (*MIBImportResult, error) {
	// 检查文件是否存在
	if _, err := os.Stat(filePath); err != nil {
		return nil, fmt.Errorf("MIB 文件不存在: %s", filePath)
	}

	// 加载 MIB 模块
	moduleName, err := gosmi.LoadModule(filePath)
	if err != nil {
		return nil, fmt.Errorf("加载 MIB 模块失败: %s: %v", filePath, err)
	}

	// 获取模块信息
	module, err := gosmi.GetModule(moduleName)
	if err != nil {
		return nil, fmt.Errorf("获取模块信息失败: %s: %v", moduleName, err)
	}

	return p.parseModuleNodes(module, filePath), nil
}

// parseModuleNodes 从 gosmi 模块中提取所有节点
func (p *MIBParser) parseModuleNodes(module gosmi.SmiModule, filePath string) *MIBImportResult {
	result := &MIBImportResult{
		Errors: []MIBParseError{},
	}

	// 构建模块模型
	mibModule := &models.MIBModule{
		Name:        module.Name,
		FileName:    filepath.Base(filePath),
		Description: module.Description,
		Source:      "import",
		FilePath:    filePath,
	}

	// 获取模块根节点
	identityNode, ok := module.GetIdentityNode()
	if !ok {
		result.Errors = append(result.Errors, MIBParseError{
			Message: "模块缺少 identity 节点定义",
		})
	}

	// 遍历并解析所有节点
	nodes := []models.MIBNode{}
	parseErrors := []MIBParseError{}

	if ok && identityNode.Name != "" {
		// 从 identity 节点开始遍历子树
		subtreeNodes := identityNode.GetSubtree()
		for _, smiNode := range subtreeNodes {
			node, err := p.convertSmiNodeToMIBNode(smiNode, nil)
			if err != nil {
				parseErrors = append(parseErrors, MIBParseError{
					NodeName: smiNode.Name,
					Message:  err.Error(),
				})
				continue
			}
			nodes = append(nodes, *node)
		}
	} else {
		// 尝试获取模块中的所有节点
		allNodes := module.GetNodes()
		for _, smiNode := range allNodes {
			node, err := p.convertSmiNodeToMIBNode(smiNode, nil)
			if err != nil {
				parseErrors = append(parseErrors, MIBParseError{
					NodeName: smiNode.Name,
					Message:  err.Error(),
				})
				continue
			}
			nodes = append(nodes, *node)
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

// convertSmiNodeToMIBNode 将 gosmi SmiNode 转换为 models.MIBNode
func (p *MIBParser) convertSmiNodeToMIBNode(smiNode gosmi.SmiNode, moduleID *uint) (*models.MIBNode, error) {
	if smiNode.Name == "" {
		return nil, fmt.Errorf("节点名称为空")
	}

	// 获取 OID 字符串
	oidStr := smiNode.Oid.String()
	if oidStr == "" {
		return nil, fmt.Errorf("节点 OID 为空")
	}

	// 计算父 OID
	parentOID := calculateParentOID(oidStr)

	// 获取节点类型
	nodeType := getNodeType(smiNode.Kind.String())

	// 获取访问权限
	access := getAccessLevel(smiNode.Access.String())

	// 获取状态
	status := getStatus(smiNode.Status.String())

	// 获取语法类型
	syntax := ""
	if smiNode.Type != nil {
		syntax = smiNode.Type.Name
		if syntax == "" {
			syntax = smiNode.Type.BaseType.String()
		}
	}

	node := &models.MIBNode{
		ModuleID:    moduleID,
		OID:         oidStr,
		Name:        smiNode.Name,
		ParentOID:   parentOID,
		NodeType:    nodeType,
		Syntax:      syntax,
		Access:      access,
		Status:      status,
		Description: smiNode.Description,
		Source:      "import",
	}

	return node, nil
}

// calculateParentOID 计算父 OID
func calculateParentOID(oid string) string {
	parts := strings.Split(oid, ".")
	if len(parts) <= 1 {
		return ""
	}
	return strings.Join(parts[:len(parts)-1], ".")
}

// getNodeType 转换节点类型
func getNodeType(kind string) string {
	switch kind {
	case "Node":
		return "node"
	case "Scalar":
		return "scalar"
	case "Table":
		return "table"
	case "Row":
		return "row"
	case "Column":
		return "column"
	case "Notification":
		return "notification"
	case "Group":
		return "group"
	case "Compliance":
		return "compliance"
	case "Capabilities":
		return "capabilities"
	default:
		return "node"
	}
}

// getAccessLevel 转换访问权限
func getAccessLevel(access string) string {
	switch access {
	case "ReadOnly":
		return "read-only"
	case "ReadWrite":
		return "read-write"
	case "NotAccessible":
		return "not-accessible"
	case "Notify":
		return "notify"
	case "ReportOnly":
		return "report-only"
	case "EventOnly":
		return "event-only"
	default:
		return "unknown"
	}
}

// getStatus 转换状态
func getStatus(status string) string {
	switch status {
	case "Current":
		return "current"
	case "Deprecated":
		return "deprecated"
	case "Obsolete":
		return "obsolete"
	case "Mandatory":
		return "mandatory"
	case "Optional":
		return "optional"
	default:
		return "unknown"
	}
}

// GetNodeByOID 通过 OID 获取节点信息（直接从 gosmi 查询）
func (p *MIBParser) GetNodeByOID(oidStr string) (*models.MIBNode, error) {
	oid, err := types.OidFromString(oidStr)
	if err != nil {
		return nil, fmt.Errorf("无效的 OID 格式: %s", oidStr)
	}

	smiNode, err := gosmi.GetNodeByOID(oid)
	if err != nil {
		return nil, err
	}

	return p.convertSmiNodeToMIBNode(smiNode, nil)
}

// GetNodeByName 通过名称获取节点信息
func (p *MIBParser) GetNodeByName(name string) (*models.MIBNode, error) {
	smiNode, err := gosmi.GetNode(name)
	if err != nil {
		return nil, fmt.Errorf("未找到节点: %s", name)
	}

	return p.convertSmiNodeToMIBNode(smiNode, nil)
}

// ResolveOID 解析 OID 为名称（优雅降级）
// 如果找不到对应节点，返回原始 OID 字符串
func (p *MIBParser) ResolveOID(oidStr string) string {
	oid, err := types.OidFromString(oidStr)
	if err != nil {
		return oidStr // 无效 OID，返回原始字符串
	}

	smiNode, err := gosmi.GetNodeByOID(oid)
	if err != nil {
		return oidStr // 未找到节点，返回原始 OID
	}

	return smiNode.Name
}

// ClearModules 清除已加载的模块（用于重新导入）
func (p *MIBParser) ClearModules() {
	// gosmi 没有提供清除单个模块的方法，需要重新初始化
	gosmi.Exit()
	gosmi.Init()
}