package snmp

import (
	"os"
	"regexp"

	"github.com/NetWeaverGo/core/internal/logger"
)

var (
	// MIB 模块名提取正则: 模块名后跟着 DEFINITIONS ::= BEGIN
	moduleNameRegex = regexp.MustCompile(`(?m)^([a-zA-Z0-9_-]+)\s+DEFINITIONS\s*::=\s*BEGIN`)
	// IMPORTS 提取正则: 匹配 IMPORTS 到 第一个分号
	importsRegex = regexp.MustCompile(`(?s)IMPORTS\s+(.*?);`)
	// FROM 依赖提取正则: 匹配 FROM 后的模块名
	fromRegex = regexp.MustCompile(`(?m)FROM\s+([a-zA-Z0-9_-]+)`)
)

type MIBDependencyInfo struct {
	FilePath   string
	ModuleName string
	DependsOn  []string
}

// BuildMIBTopology 扫描文件构建有向无环图，并通过拓扑排序返回分批导入顺序
func BuildMIBTopology(filePaths []string) ([][]string, error) {
	if len(filePaths) == 0 {
		return nil, nil
	}

	logger.Debug("SNMP-MIB", "-", "开始预扫描并构建 MIB 依赖拓扑: 文件数=%d", len(filePaths))

	// 1. 预处理提取每个文件的模块名和依赖
	fileInfoMap := make(map[string]*MIBDependencyInfo) // 模块名 -> 信息
	pathModuleMap := make(map[string]string)           // 文件路径 -> 模块名

	for _, path := range filePaths {
		contentBytes, err := os.ReadFile(path)
		if err != nil {
			logger.Warn("SNMP-MIB", "-", "预扫描读取文件失败: %s, %v", path, err)
			continue // 读取失败略过，后续解析时会报错
		}
		content := string(contentBytes)

		// 提取模块名
		modMatches := moduleNameRegex.FindStringSubmatch(content)
		if len(modMatches) < 2 {
			continue // 找不到模块定义
		}
		modName := modMatches[1]

		info := &MIBDependencyInfo{
			FilePath:   path,
			ModuleName: modName,
			DependsOn:  []string{},
		}

		// 提取 IMPORTS 块
		impMatches := importsRegex.FindStringSubmatch(content)
		if len(impMatches) >= 2 {
			importsBlock := impMatches[1]
			// 提取所有 FROM 的依赖
			fromMatches := fromRegex.FindAllStringSubmatch(importsBlock, -1)
			for _, fm := range fromMatches {
				if len(fm) >= 2 {
					dep := fm[1]
					// 避免自依赖
					if dep != modName {
						info.DependsOn = append(info.DependsOn, dep)
					}
				}
			}
		}

		fileInfoMap[modName] = info
		pathModuleMap[path] = modName
	}

	// 2. 构建依赖图 (Kahn's Algorithm)
	inDegree := make(map[string]int)          // 模块名 -> 入度 (依赖了多少个在这个文件列表中的模块)
	graph := make(map[string][]string)        // 被依赖的模块 -> 依赖它的模块列表 (A -> [B, C])

	// 初始化节点（只关心在当前上传列表中的文件间的依赖关系，外部依赖我们不管）
	for modName := range fileInfoMap {
		inDegree[modName] = 0
	}

	for modName, info := range fileInfoMap {
		for _, dep := range info.DependsOn {
			if _, exists := fileInfoMap[dep]; exists {
				// 这个被依赖的模块在本次导入列表中
				graph[dep] = append(graph[dep], modName)
				inDegree[modName]++
			}
		}
	}

	// 3. 拓扑排序 (分层输出)
	var batches [][]string
	var currentBatch []string

	// 找到所有入度为 0 的节点作为起点
	for modName, degree := range inDegree {
		if degree == 0 {
			currentBatch = append(currentBatch, modName)
		}
	}

	for len(currentBatch) > 0 {
		var currentFilePaths []string
		var nextBatch []string

		for _, modName := range currentBatch {
			info := fileInfoMap[modName]
			currentFilePaths = append(currentFilePaths, info.FilePath)
			delete(inDegree, modName)

			for _, dependent := range graph[modName] {
				inDegree[dependent]--
				if inDegree[dependent] == 0 {
					nextBatch = append(nextBatch, dependent)
				}
			}
		}
		
		// 对批次内的路径排序，保证稳定性
		// 为了简单这里就不 sort 了，因为前面已经确定了批次
		batches = append(batches, currentFilePaths)
		currentBatch = nextBatch
	}

	// 处理成环或未能提取到所有前置依赖的残留文件（强行放入最后一批）
	var residualPaths []string
	for modName := range inDegree {
		residualPaths = append(residualPaths, fileInfoMap[modName].FilePath)
		logger.Warn("SNMP-MIB", "-", "检测到循环依赖或无法解开的依赖模块: %s", modName)
	}

	// 对于那些连模块名都没解析出来的文件，也强行放入最后一批，让 gomib 去处理
	for _, path := range filePaths {
		if _, ok := pathModuleMap[path]; !ok {
			residualPaths = append(residualPaths, path)
			logger.Warn("SNMP-MIB", "-", "未能正则提取出模块名的文件，强行放入降级批次: %s", path)
		}
	}

	if len(residualPaths) > 0 {
		batches = append(batches, residualPaths)
	}

	logger.Debug("SNMP-MIB", "-", "拓扑排序完成: 共切分为 %d 个批次", len(batches))
	for i, batch := range batches {
		logger.Debug("SNMP-MIB", "-", "  Batch %d: %d 个文件", i, len(batch))
	}

	return batches, nil
}
