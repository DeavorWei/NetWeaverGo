# NetWeaverGo SNMP 解析库迁移实施方案

> **文档版本**: v1.1
> **创建日期**: 2026-05-23
> **最后复核**: 2026-05-23
> **基于文档**: SNMP_MIGRATION_PLAN.md
> **目标**: 将 MIB 解析库从 gosmi 迁移到 gomib，实现 CGO-Free

---

## 1. 执行摘要

### 1.1 迁移目标
将 NetWeaverGo 项目的 SNMP MIB 解析模块从基于 CGO 的 `github.com/sleepinggenius2/gosmi` 迁移到纯 Go 实现的 `github.com/golangsnmp/gomib`，彻底消除 CGO 依赖，简化跨平台编译和部署流程。

### 1.2 影响范围
| 模块 | 文件 | 改造程度 | 说明 |
|------|------|----------|------|
| MIB Parser | `internal/snmp/mib_parser.go` | **完全重写** | 核心改造点 |
| MIB Manager | `internal/snmp/mib_manager.go` | **轻微调整** | Init/Exit 生命周期调用 |
| OID Resolver | `internal/snmp/oid_resolver.go` | **无需修改** | 依赖缓存和数据库 |
| Types | `internal/snmp/types.go` | **无需修改** | 类型定义不变 |
| Repository | `internal/repository/mib_repository.go` | **无需修改** | 数据访问层不变 |
| UI Service | `internal/ui/snmp_mib_service.go` | **无需修改** | 服务层无感知 |
| Models | `internal/models/snmp.go` | **无需修改** | 数据模型不变 |
| Frontend | `frontend/src/views/SNMP/SNMPMib.vue` | **无需修改** | 前端无感知 |

### 1.3 预期收益
- ✅ 完全移除 CGO 依赖
- ✅ 简化 Windows/Linux 交叉编译流程
- ✅ 提升并发安全性（gosmi 存在 C 全局状态）
- ✅ 减小二进制体积
- ✅ 部署无需依赖 C 运行时环境

---

## 2. 技术架构分析

### 2.1 当前架构（gosmi）

```mermaid
graph TB
    subgraph Frontend
        A[SNMPMib.vue]
        B[useMIBTree.ts]
    end
    
    subgraph UI Layer
        C[SNMPMIBService]
    end
    
    subgraph SNMP Core
        D[MIBManager]
        E[MIBParser - gosmi]
        F[OIDResolver]
        G[LRU Cache]
    end
    
    subgraph Data Layer
        H[MIBRepository]
        I[(SQLite - snmp.db)]
    end
    
    subgraph External
        J[libsmi C Library]
    end
    
    A --> C
    B --> C
    C --> D
    D --> E
    D --> G
    D --> H
    E --> J
    F --> G
    F --> H
    H --> I
```

### 2.2 目标架构（gomib）

```mermaid
graph TB
    subgraph Frontend
        A[SNMPMib.vue]
        B[useMIBTree.ts]
    end
    
    subgraph UI Layer
        C[SNMPMIBService]
    end
    
    subgraph SNMP Core
        D[MIBManager]
        E[MIBParser - gomib]
        F[OIDResolver]
        G[LRU Cache]
    end
    
    subgraph Data Layer
        H[MIBRepository]
        I[(SQLite - snmp.db)]
    end
    
    A --> C
    B --> C
    C --> D
    D --> E
    D --> G
    D --> H
    F --> G
    F --> H
    H --> I
```

**关键变化**: 移除了对 `libsmi C Library` 的依赖，`MIBParser` 直接使用纯 Go 的 `gomib` 库。

### 2.3 数据流分析

#### 2.3.1 MIB 导入流程

```mermaid
sequenceDiagram
    participant U as Frontend
    participant S as SNMPMIBService
    participant M as MIBManager
    participant P as MIBParser
    participant R as MIBRepository
    participant DB as SQLite
    
    U->>S: ImportMIBFile filePath
    S->>M: ImportMIBFile ctx filePath
    M->>M: copyMIBFile 复制到存储目录
    M->>P: ParseFile storedPath
    P->>P: gomib.ParseFile
    P-->>M: MIBImportResult Module+Nodes+Errors
    M->>R: SaveModule module
    R->>DB: INSERT mib_modules
    M->>R: SaveNodes nodes
    R->>DB: INSERT mib_nodes batch=100
    M->>M: Update LRU Cache
    M-->>S: MIBImportResult
    S-->>U: MIBImportResult JSON
    S->>U: Event: snmp:mib:imported
```

#### 2.3.2 OID 解析流程

```mermaid
sequenceDiagram
    participant U as Frontend/TrapHandler
    participant S as SNMPMIBService
    participant R as OIDResolver
    participant C as LRU Cache
    participant DB as SQLite
    
    U->>S: ResolveOID oid
    S->>R: Resolve oid
    R->>C: Get oid
    alt Cache Hit
        C-->>R: MIBNode
    else Cache Miss
        R->>DB: SELECT FROM mib_nodes WHERE oid=?
        DB-->>R: MIBNode
        R->>C: Add oid node
    end
    R-->>S: name
    S-->>U: name
```

---

## 3. 详细实施步骤

### 3.1 阶段一：环境准备与依赖管理

#### 3.1.0 gomib 库调研与验证（前置步骤）

**目的**: 在正式迁移前，验证 gomib 库的实际 API 和功能完整性。

**调研清单**:

| 检查项 | 验证方法 | 预期结果 |
|--------|----------|----------|
| 库活跃度 | 查看 GitHub 提交记录 | 近 6 个月有活跃提交 |
| API 稳定性 | 查看 Release 版本 | 有正式 Release 版本 |
| 文档完整性 | 查看 README 和 GoDoc | 有完整 API 文档 |
| 测试覆盖率 | 查看 CI 配置和测试文件 | 有单元测试和 CI |
| Issue 处理 | 查看 Issues 列表 | 无严重未解决 Bug |

**API 验证脚本**:

创建临时测试文件 `tmp/gomib_verify.go`:
```go
package main

import (
    "fmt"
    "os"
    
    "github.com/golangsnmp/gomib"
)

func main() {
    // 1. 创建解析器
    parser := gomib.NewParser()
    fmt.Println("✓ NewParser() 成功")
    
    // 2. 设置搜索路径
    parser.SetPath([]string{"./testdata/mibs"})
    fmt.Println("✓ SetPath() 成功")
    
    // 3. 解析测试文件
    module, err := parser.ParseFile("./testdata/mibs/RFC1213-MIB.txt")
    if err != nil {
        fmt.Printf("✗ ParseFile() 失败: %v\n", err)
        os.Exit(1)
    }
    fmt.Printf("✓ ParseFile() 成功: %s\n", module.Name)
    
    // 4. 验证节点结构
    if len(module.Nodes) == 0 {
        fmt.Println("✗ 模块节点为空")
        os.Exit(1)
    }
    fmt.Printf("✓ 节点数量: %d\n", len(module.Nodes))
    
    // 5. 验证节点属性
    for _, node := range module.Nodes {
        if node.Name == "sysDescr" {
            fmt.Printf("  找到目标节点: %s\n", node.Name)
            fmt.Printf("    OID: %s\n", node.OID.String())
            fmt.Printf("    类型: %s\n", node.Kind.String())
            fmt.Printf("    访问权限: %s\n", node.Access.String())
            fmt.Printf("    状态: %s\n", node.Status.String())
            fmt.Printf("    描述: %s\n", node.Description)
        }
    }
    
    fmt.Println("\n✓ gomib API 验证通过")
}
```

**执行验证**:
```bash
# 创建测试目录
mkdir -p tmp/testdata/mibs

# 下载测试 MIB 文件（需手动准备 RFC1213-MIB.txt）

# 运行验证脚本
cd tmp
go mod init gomib-verify
go get github.com/golangsnmp/gomib@latest
go run gomib_verify.go
```

**API 映射确认表**:

| gosmi API | gomib API | 兼容性 | 备注 |
|-----------|-----------|--------|------|
| `gosmi.Init()` | 无需调用 | ✅ | gomib 无全局状态 |
| `gosmi.Exit()` | 无需调用 | ✅ | gomib 无全局状态 |
| `gosmi.LoadModule(path)` | `parser.ParseFile(path)` | ⚠️ | 需验证返回值结构 |
| `gosmi.GetModule(name)` | `parser.Module` | ⚠️ | 需验证模块获取方式 |
| `module.GetIdentityNode()` | `module.RootNode` | ⚠️ | 需验证根节点命名 |
| `node.GetSubtree()` | `node.Walk(fn)` | ⚠️ | 需验证遍历方式 |
| `smiNode.Oid.String()` | `node.OID.String()` | ⚠️ | 需验证 OID 类型 |
| `smiNode.Kind.String()` | `node.Kind.String()` | ⚠️ | 需验证枚举值 |
| `smiNode.Access.String()` | `node.Access.String()` | ⚠️ | 需验证枚举值 |
| `smiNode.Status.String()` | `node.Status.String()` | ⚠️ | 需验证枚举值 |
| `smiNode.Type.Name` | `node.Syntax.Name` | ⚠️ | 需验证类型字段命名 |

**验证不通过时的应对**:
1. 如果 API 不兼容，调整适配层代码
2. 如果功能缺失，评估是否可接受或寻找替代方案
3. 如果库不成熟，考虑推迟迁移或寻找其他替代库

#### 3.1.1 创建特性分支
```bash
# 从 main 分支创建特性分支
git checkout -b feature/snmp-cgo-free
git push -u origin feature/snmp-cgo-free
```

#### 3.1.2 更新 Go 模块依赖

**操作步骤**:
```bash
# 1. 添加 gomib 依赖
go get github.com/golangsnmp/gomib@latest

# 2. 清理依赖
go mod tidy

# 3. 验证依赖树
go mod graph | Select-String "gomib"
go mod graph | Select-String "gosmi"
```

**预期结果**:
- `go.mod` 中包含 `github.com/golangsnmp/gomib`
- `go.mod` 中移除 `github.com/sleepinggenius2/gosmi`
- `go.sum` 更新相应的校验和

**验证命令**:
```bash
# 确认无 CGO 依赖
go list -m all | Select-String "gosmi"  # 应无输出
go list -m all | Select-String "gomib"  # 应有输出
```

---

### 3.2 阶段二：重写 MIBParser

#### 3.2.1 文件改造清单

**文件**: `internal/snmp/mib_parser.go`

**改造内容**:

| 原实现 | 新实现 | 说明 |
|--------|--------|------|
| `import gosmi` | `import gomib` | 依赖替换 |
| `gosmi.Init()` | 移除 | gomib 无全局初始化 |
| `gosmi.Exit()` | 移除 | gomib 无全局清理 |
| `gosmi.LoadModule()` | `gomib.NewParser().ParseFile()` | 解析方法变更 |
| `gosmi.GetModule()` | `parser.Module` | 模块获取方式 |
| `module.GetIdentityNode()` | `module.RootNode` | 根节点获取 |
| `node.GetSubtree()` | `node.Walk()` | 子树遍历 |
| `smiNode.Oid.String()` | `node.OID.String()` | OID 格式化 |
| `smiNode.Kind.String()` | `node.Kind.String()` | 节点类型 |

#### 3.2.2 新版 MIBParser 实现

**核心结构体**:
```go
// MIBParser MIB 文件解析器
// 使用 gomib 库解析 SMIv1/SMIv2 格式的 MIB 文件
type MIBParser struct {
    mu       sync.Mutex
    parser   *gomib.Parser
    basePath []string // MIB 依赖搜索路径
}

// NewMIBParser 创建 MIB 解析器实例
func NewMIBParser() *MIBParser {
    parser := gomib.NewParser()
    return &MIBParser{
        parser: parser,
    }
}
```

**生命周期方法调整**:
```go
// Init 初始化解析器（保持接口兼容，实际无操作）
func (p *MIBParser) Init() {
    // gomib 无全局状态，无需初始化
    // 保留空实现以兼容 MIBManager
}

// Exit 清理解析器资源（保持接口兼容，实际无操作）
func (p *MIBParser) Exit() {
    // gomib 无全局状态，无需清理
    // 保留空实现以兼容 MIBManager
}
```

**核心解析方法**:
```go
// ParseFile 解析单个 MIB 文件
func (p *MIBParser) ParseFile(filePath string) (*MIBImportResult, error) {
    p.mu.Lock()
    defer p.mu.Unlock()
    
    return p.parseFileLocked(filePath)
}

// ParseFileWithDependencies 解析 MIB 文件及其依赖
func (p *MIBParser) ParseFileWithDependencies(filePath string, dependencyDirs []string) (*MIBImportResult, error) {
    p.mu.Lock()
    defer p.mu.Unlock()
    
    // 设置依赖搜索路径
    p.basePath = dependencyDirs
    
    return p.parseFileLocked(filePath)
}

// parseFileLocked 内部解析实现
func (p *MIBParser) parseFileLocked(filePath string) (*MIBImportResult, error) {
    // 1. 检查文件存在性
    if _, err := os.Stat(filePath); err != nil {
        return nil, fmt.Errorf("MIB 文件不存在: %s", filePath)
    }
    
    // 2. 设置解析器搜索路径
    if len(p.basePath) > 0 {
        p.parser.SetPath(p.basePath)
    }
    
    // 3. 解析 MIB 文件
    module, err := p.parser.ParseFile(filePath)
    if err != nil {
        return nil, fmt.Errorf("解析 MIB 文件失败: %s: %v", filePath, err)
    }
    
    // 4. 转换为内部数据结构
    return p.parseModuleNodes(module, filePath), nil
}
```

**节点转换方法**:
```go
// parseModuleNodes 从 gomib 模块提取所有节点
func (p *MIBParser) parseModuleNodes(module *gomib.Module, filePath string) *MIBImportResult {
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
    
    // 遍历所有节点
    nodes := []models.MIBNode{}
    parseErrors := []MIBParseError{}
    
    // 使用 Walk 方法遍历节点树
    for _, node := range module.Nodes {
        err := node.Walk(func(n *gomib.Node) error {
            mibNode, err := p.convertGomibNodeToMIBNode(n, nil)
            if err != nil {
                parseErrors = append(parseErrors, MIBParseError{
                    NodeName: n.Name,
                    Message:  err.Error(),
                })
                return nil // 继续遍历
            }
            nodes = append(nodes, *mibNode)
            return nil
        })
        if err != nil {
            parseErrors = append(parseErrors, MIBParseError{
                Message: fmt.Sprintf("遍历节点树失败: %v", err),
            })
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

// convertGomibNodeToMIBNode 将 gomib.Node 转换为 models.MIBNode
func (p *MIBParser) convertGomibNodeToMIBNode(node *gomib.Node, moduleID *uint) (*models.MIBNode, error) {
    if node.Name == "" {
        return nil, fmt.Errorf("节点名称为空")
    }
    
    // 获取 OID 字符串
    oidStr := node.OID.String()
    if oidStr == "" {
        return nil, fmt.Errorf("节点 OID 为空")
    }
    
    // 计算父 OID
    parentOID := calculateParentOID(oidStr)
    
    // 获取节点类型
    nodeType := convertNodeType(node.Kind)
    
    // 获取访问权限
    access := convertAccess(node.Access)
    
    // 获取状态
    status := convertStatus(node.Status)
    
    // 获取语法类型
    syntax := ""
    if node.Syntax != nil {
        syntax = node.Syntax.Name
        if syntax == "" {
            syntax = node.Syntax.BaseType.String()
        }
    }
    
    return &models.MIBNode{
        ModuleID:    moduleID,
        OID:         oidStr,
        Name:        node.Name,
        ParentOID:   parentOID,
        NodeType:    nodeType,
        Syntax:      syntax,
        Access:      access,
        Status:      status,
        Description: node.Description,
        Source:      "import",
    }, nil
}
```

**辅助转换函数**:
```go
// convertNodeType 转换节点类型
func convertNodeType(kind gomib.NodeKind) string {
    switch kind {
    case gomib.KindNode:
        return "node"
    case gomib.KindScalar:
        return "scalar"
    case gomib.KindTable:
        return "table"
    case gomib.KindRow:
        return "row"
    case gomib.KindColumn:
        return "column"
    case gomib.KindNotification:
        return "notification"
    case gomib.KindGroup:
        return "group"
    case gomib.KindCompliance:
        return "compliance"
    default:
        return "node"
    }
}

// convertAccess 转换访问权限
func convertAccess(access gomib.Access) string {
    switch access {
    case gomib.AccessReadOnly:
        return "read-only"
    case gomib.AccessReadWrite:
        return "read-write"
    case gomib.AccessNotAccessible:
        return "not-accessible"
    case gomib.AccessNotify:
        return "notify"
    case gomib.AccessReportOnly:
        return "report-only"
    case gomib.AccessEventOnly:
        return "event-only"
    default:
        return "unknown"
    }
}

// convertStatus 转换状态
func convertStatus(status gomib.Status) string {
    switch status {
    case gomib.StatusCurrent:
        return "current"
    case gomib.StatusDeprecated:
        return "deprecated"
    case gomib.StatusObsolete:
        return "obsolete"
    case gomib.StatusMandatory:
        return "mandatory"
    case gomib.StatusOptional:
        return "optional"
    default:
        return "unknown"
    }
}
```

**运行时查询方法调整**:
```go
// GetNodeByOID 通过 OID 获取节点信息
// 注意：此方法现在依赖缓存和数据库，不再直接调用解析库
func (p *MIBParser) GetNodeByOID(oidStr string) (*models.MIBNode, error) {
    // 由于架构设计，所有节点在导入时已持久化
    // 此方法保留但建议通过 MIBManager 查询
    return nil, fmt.Errorf("请使用 MIBManager.GetNodeByOID 方法")
}

// GetNodeByName 通过名称获取节点信息
func (p *MIBParser) GetNodeByName(name string) (*models.MIBNode, error) {
    return nil, fmt.Errorf("请使用 MIBManager.GetNodeByName 方法")
}

// ResolveOID 解析 OID 为名称
func (p *MIBParser) ResolveOID(oidStr string) string {
    // 优雅降级：返回原始 OID
    return oidStr
}

// ClearModules 清除已加载的模块
func (p *MIBParser) ClearModules() {
    // gomib 无全局状态，无需清理
    // 保留空实现以兼容 MIBManager
}
```

---

### 3.3 阶段三：调整 MIBManager

#### 3.3.1 文件改造清单

**文件**: `internal/snmp/mib_manager.go`

**改造内容**:

| 位置 | 原代码 | 新代码 | 说明 |
|------|--------|--------|------|
| 第 61 行 | `mgr.parser.Init()` | 保留（空实现） | 兼容性保持 |
| 第 68 行 | `m.parser.Exit()` | 保留（空实现） | 兼容性保持 |

**无需修改的原因**:
- `MIBParser` 的 `Init()` 和 `Exit()` 方法保留空实现
- `MIBManager` 的其他逻辑完全依赖缓存和数据库
- 架构设计已经解耦，Parser 仅在导入时使用

---

### 3.4 阶段四：编译验证

#### 3.4.1 本地编译测试

**Windows 本地编译**:
```bash
# 清理构建缓存
go clean -cache

# 标准编译
go build -o bin/netweaver.exe ./cmd/netweaver

# 验证无 CGO 依赖
go build -ldflags="-s -w" -o bin/netweaver.exe ./cmd/netweaver
```

**验证 CGO-Free**:
```bash
# 使用 PowerShell 检查导入依赖
go list -f '{{.Imports}}' ./cmd/netweaver | Select-String "C\."

# 检查是否包含 CGO 相关包
go list -f '{{.Imports}}' ./... | Select-String "runtime/cgo"
```

#### 3.4.2 交叉编译测试

**Linux 交叉编译**:
```bash
# 设置 Linux 编译环境
$env:GOOS="linux"
$env:GOARCH="amd64"
$env:CGO_ENABLED="0"

# 编译 Linux 二进制
go build -ldflags="-s -w" -o bin/netweaver-linux-amd64 ./cmd/netweaver

# 重置环境
Remove-Item Env:GOOS
Remove-Item Env:GOARCH
Remove-Item Env:CGO_ENABLED
```

**macOS 交叉编译**:
```bash
# 设置 macOS 编译环境
$env:GOOS="darwin"
$env:GOARCH="amd64"
$env:CGO_ENABLED="0"

go build -ldflags="-s -w" -o bin/netweaver-darwin-amd64 ./cmd/netweaver

Remove-Item Env:GOOS
Remove-Item Env:GOARCH
Remove-Item Env:CGO_ENABLED
```

---

### 3.5 阶段五：集成测试

#### 3.5.1 测试用例设计

| 测试场景 | 测试文件 | 预期结果 | 验证点 |
|----------|----------|----------|--------|
| 标准 RFC MIB | `RFC1213-MIB.txt` | 完整解析 | 节点数量、树结构 |
| 厂商私有 MIB | `HUAWEI-MIB.mib` | 完整解析 | 自定义节点、Trap 定义 |
| 带依赖的 MIB | `IF-MIB.txt` (依赖 SNMPv2-SMI) | 自动解析依赖 | IMPORT 语句处理 |
| 非标准语法 MIB | 不规范的私有 MIB | 部分导入 | 错误收集、容错机制 |
| 大型 MIB 文件 | `SNMPv2-MIB.txt` | 性能测试 | 解析时间、内存占用 |

#### 3.5.2 功能验证清单

**前端 UI 验证**:
- [ ] MIB 模块列表正确显示
- [ ] MIB 树形结构正确展开
- [ ] OID 解析正确显示名称
- [ ] 导入进度事件正确推送
- [ ] 错误信息正确显示

**后端 API 验证**:
- [ ] `GetMIBModules` 返回正确列表
- [ ] `GetMIBTree` 返回正确树结构
- [ ] `ResolveOID` 返回正确名称
- [ ] `ImportMIBFile` 正确处理文件
- [ ] `DeleteMIBModule` 正确清理数据

**数据持久化验证**:
- [ ] SQLite 数据库正确写入
- [ ] LRU 缓存正确更新
- [ ] 节点关系正确建立
- [ ] 删除操作正确级联

#### 3.5.3 SNMP Trap 功能专项测试

**测试目的**: 验证 Trap 接收时 OID 解析功能不受迁移影响。

**测试场景**:

| 测试项 | 测试方法 | 预期结果 |
|--------|----------|----------|
| Trap OID 解析 | 发送包含已导入 MIB 的 Trap | Trap 名称正确显示 |
| 未导入 MIB 的 Trap | 发送未导入 MIB 的 Trap | 显示原始 OID |
| Trap 变量绑定解析 | 发送带 VarBind 的 Trap | VarBind OID 正确解析 |
| 多模块 Trap | 发送不同模块的 Trap | 各模块 Trap 名称正确 |
| 高频 Trap 接收 | 连续发送多个 Trap | 解析无延迟，无阻塞 |

**测试代码示例**:
```go
// trap_test.go Trap 解析功能测试
func TestTrapOIDResolution(t *testing.T) {
    // 1. 导入测试 MIB
    manager := setupMIBManager(t)
    result, err := manager.ImportMIBFile(context.Background(), "./testdata/RFC1213-MIB.txt")
    assert.NoError(t, err)
    assert.Greater(t, result.NodeCount, 0)
    
    // 2. 模拟 Trap 接收
    trapOID := "1.3.6.1.2.1.1.3.0" // sysUpTime
    resolver := snmp.NewOIDResolver(manager)
    
    // 3. 解析 OID
    name := resolver.Resolve(trapOID)
    assert.Equal(t, "sysUpTime", name)
    
    // 4. 测试未知 OID
    unknownOID := "1.3.6.1.999.999.999"
    unknownName := resolver.Resolve(unknownOID)
    assert.Equal(t, unknownOID, unknownName) // 返回原始 OID
}
```

#### 3.5.4 SNMP Polling 功能专项测试

**测试目的**: 验证轮询采集时 OID 解析功能不受迁移影响。

**测试场景**:

| 测试项 | 测试方法 | 预期结果 |
|--------|----------|----------|
| Polling OID 解析 | 采集已导入 MIB 的 OID | 结果显示正确名称 |
| 批量 OID 采集 | Walk 操作遍历表 | 所有 OID 正确解析 |
| Polling 结果存储 | 结果写入数据库 | OIDName 字段正确 |
| Polling 结果查询 | 查询历史数据 | OID 名称正确显示 |

**测试代码示例**:
```go
// polling_test.go Polling 解析功能测试
func TestPollingOIDResolution(t *testing.T) {
    // 1. 导入 IF-MIB
    manager := setupMIBManager(t)
    _, err := manager.ImportMIBFile(context.Background(), "./testdata/IF-MIB.txt")
    assert.NoError(t, err)
    
    // 2. 模拟 Polling 结果
    results := []models.SNMPPollingResult{
        {OID: "1.3.6.1.2.1.2.2.1.1.1", Value: "eth0"},
        {OID: "1.3.6.1.2.1.2.2.1.2.1", Value: "Ethernet0"},
    }
    
    // 3. 解析 OID 名称
    resolver := snmp.NewOIDResolver(manager)
    for i, result := range results {
        results[i].OIDName = resolver.Resolve(result.OID)
    }
    
    // 4. 验证解析结果
    assert.Equal(t, "ifIndex", results[0].OIDName)
    assert.Equal(t, "ifDescr", results[1].OIDName)
}
```

#### 3.5.5 性能对比测试

**测试指标**:
| 指标 | gosmi | gomib | 改善 |
|------|-------|-------|------|
| 编译时间 | 基准 | 预期减少 | 无 CGO 开销 |
| 二进制大小 | 基准 | 预期减小 | 无 C 库嵌入 |
| 内存占用 | 基准 | 待测 | 纯 Go 优化 |
| 解析速度 | 基准 | 待测 | 需实测 |
| 并发安全 | 存在风险 | 完全安全 | 无全局状态 |

**性能基准测试代码**:
```go
// benchmark_test.go 性能基准测试
func BenchmarkMIBParsing(b *testing.B) {
    parser := NewMIBParser()
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        result, _ := parser.ParseFile("./testdata/RFC1213-MIB.txt")
        _ = result
    }
}

func BenchmarkConcurrentMIBParsing(b *testing.B) {
    parser := NewMIBParser()
    
    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            result, _ := parser.ParseFile("./testdata/IF-MIB.txt")
            _ = result
        }
    })
}

func BenchmarkOIDResolution(b *testing.B) {
    manager := setupMIBManager(nil)
    resolver := snmp.NewOIDResolver(manager)
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        name := resolver.Resolve("1.3.6.1.2.1.1.1.0")
        _ = name
    }
}
```

---

### 3.6 阶段六：单元测试补充

#### 3.6.1 MIBParser 单元测试

**测试文件**: `internal/snmp/mib_parser_test.go`

```go
package snmp

import (
    "context"
    "testing"
    
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestMIBParser_InitExit(t *testing.T) {
    parser := NewMIBParser()
    
    // Init 和 Exit 应该是无操作的
    parser.Init()
    parser.Exit()
    
    // 不应抛出任何错误
    assert.NotPanics(t, func() {
        parser.Init()
        parser.Exit()
    })
}

func TestMIBParser_ParseFile(t *testing.T) {
    parser := NewMIBParser()
    
    tests := []struct {
        name     string
        filePath string
        wantErr  bool
    }{
        {"Valid MIB", "./testdata/RFC1213-MIB.txt", false},
        {"Invalid Path", "./testdata/nonexistent.txt", true},
        {"Empty File", "./testdata/empty.txt", true},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := parser.ParseFile(tt.filePath)
            if tt.wantErr {
                assert.Error(t, err)
                assert.Nil(t, result)
            } else {
                assert.NoError(t, err)
                assert.NotNil(t, result)
                assert.Greater(t, result.NodeCount, 0)
            }
        })
    }
}

func TestMIBParser_ParseFileWithDependencies(t *testing.T) {
    parser := NewMIBParser()
    
    // 测试带依赖的 MIB 解析
    result, err := parser.ParseFileWithDependencies(
        "./testdata/IF-MIB.txt",
        []string{"./testdata/standard"},
    )
    
    require.NoError(t, err)
    assert.NotNil(t, result)
    assert.Greater(t, result.NodeCount, 0)
    
    // 验证关键节点存在
    found := false
    for _, node := range result.Nodes {
        if node.Name == "ifNumber" {
            found = true
            assert.Equal(t, "1.3.6.1.2.1.2.1", node.OID)
            break
        }
    }
    assert.True(t, found, "应找到 ifNumber 节点")
}

func TestMIBParser_ConvertNode(t *testing.T) {
    parser := NewMIBParser()
    
    // 测试节点转换逻辑
    // 注意：此测试需要根据 gomib 实际 API 调整
    t.Skip("需要根据 gomib 实际 API 实现此测试")
}

func TestMIBParser_PartialImport(t *testing.T) {
    parser := NewMIBParser()
    
    // 测试部分导入功能
    result, err := parser.ParseFile("./testdata/malformed-mib.txt")
    
    // 即使有错误，也应该返回部分结果
    assert.NoError(t, err)
    assert.NotNil(t, result)
    assert.Greater(t, result.ErrorCount, 0)
    assert.Greater(t, result.NodeCount, 0) // 至少部分节点成功解析
}

func TestMIBParser_ConcurrentParsing(t *testing.T) {
    parser := NewMIBParser()
    
    // 测试并发解析安全性
    files := []string{
        "./testdata/RFC1213-MIB.txt",
        "./testdata/IF-MIB.txt",
        "./testdata/SNMPv2-MIB.txt",
    }
    
    done := make(chan bool, len(files))
    
    for _, file := range files {
        go func(f string) {
            result, err := parser.ParseFile(f)
            assert.NoError(t, err)
            assert.NotNil(t, result)
            done <- true
        }(file)
    }
    
    // 等待所有解析完成
    for i := 0; i < len(files); i++ {
        select {
        case <-done:
            // 成功
        case <-time.After(10 * time.Second):
            t.Fatal("并发解析超时")
        }
    }
}
```

#### 3.6.2 MIBManager 集成测试

**测试文件**: `internal/snmp/mib_manager_test.go`

```go
package snmp

import (
    "context"
    "testing"
    
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    
    "github.com/NetWeaverGo/core/internal/repository"
)

func setupTestManager(t *testing.T) *MIBManager {
    // 创建测试用的内存数据库
    db := setupTestDB(t)
    repo := repository.NewGormMIBRepository(db)
    
    manager := NewMIBManager(repo, "./testdata/mibs")
    return manager
}

func TestMIBManager_ImportMIBFile(t *testing.T) {
    manager := setupTestManager(t)
    defer manager.Close()
    
    ctx := context.Background()
    result, err := manager.ImportMIBFile(ctx, "./testdata/RFC1213-MIB.txt")
    
    require.NoError(t, err)
    assert.NotNil(t, result.Module)
    assert.Greater(t, result.NodeCount, 0)
    
    // 验证模块已保存
    modules, _ := manager.repo.GetAllModules()
    assert.GreaterOrEqual(t, len(modules), 1)
}

func TestMIBManager_DeleteMIBModule(t *testing.T) {
    manager := setupTestManager(t)
    defer manager.Close()
    
    // 先导入
    ctx := context.Background()
    result, _ := manager.ImportMIBFile(ctx, "./testdata/RFC1213-MIB.txt")
    moduleID := result.Module.ID
    
    // 再删除
    err := manager.DeleteMIBModule(moduleID)
    assert.NoError(t, err)
    
    // 验证已删除
    module, _ := manager.repo.GetModuleByID(moduleID)
    assert.Nil(t, module)
    
    // 验证节点已删除
    nodes, _ := manager.repo.GetNodesByModule(moduleID)
    assert.Equal(t, 0, len(nodes))
}

func TestMIBManager_GetNodeByOID(t *testing.T) {
    manager := setupTestManager(t)
    defer manager.Close()
    
    // 导入 MIB
    ctx := context.Background()
    _, _ = manager.ImportMIBFile(ctx, "./testdata/RFC1213-MIB.txt")
    
    // 测试缓存命中
    node, err := manager.GetNodeByOID("1.3.6.1.2.1.1.1")
    require.NoError(t, err)
    assert.NotNil(t, node)
    assert.Equal(t, "sysDescr", node.Name)
    
    // 测试缓存未命中（从数据库加载）
    node2, err := manager.GetNodeByOID("1.3.6.1.2.1.1.2")
    require.NoError(t, err)
    assert.NotNil(t, node2)
    assert.Equal(t, "sysObjectID", node2.Name)
}

func TestMIBManager_OIDResolver(t *testing.T) {
    manager := setupTestManager(t)
    defer manager.Close()
    
    // 导入 MIB
    ctx := context.Background()
    _, _ = manager.ImportMIBFile(ctx, "./testdata/RFC1213-MIB.txt")
    
    resolver := NewOIDResolver(manager)
    
    // 测试已知 OID
    name := resolver.Resolve("1.3.6.1.2.1.1.1.0")
    assert.Equal(t, "sysDescr", name)
    
    // 测试未知 OID（优雅降级）
    name2 := resolver.Resolve("1.3.6.1.999.999.999")
    assert.Equal(t, "1.3.6.1.999.999.999", name2)
}
```

---

## 4. 风险评估与应对策略

### 4.1 技术风险

#### 风险 1: 私有 MIB 语法兼容性

**风险描述**: `libsmi` 经过多年发展，积累了大量对非标准 MIB 文件的容错处理。纯 Go 的 `gomib` 可能对语法错误更严格。

**影响等级**: 高

**应对策略**:
1. **部分导入机制**: 在解析过程中捕获错误，跳过无法解析的节点，保留已成功解析的部分
2. **错误收集**: 将所有解析错误收集到 `MIBParseError` 数组，返回给前端展示
3. **用户引导**: 在 UI 中显示具体错误位置和建议修复方案
4. **兼容层**: 如发现特定语法模式导致解析失败，可在适配层添加预处理逻辑

**代码实现**:
```go
// parseModuleNodes 增强错误处理
func (p *MIBParser) parseModuleNodes(module *gomib.Module, filePath string) *MIBImportResult {
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
    
    // ... 解析逻辑 ...
}
```

#### 风险 2: IMPORTS 依赖解析

**风险描述**: MIB 文件通常依赖其他 MIB 模块的定义（如 `IMPORTS FROM SNMPv2-SMI`）。如果依赖路径未正确设置，解析会失败。

**影响等级**: 中

**应对策略**:
1. **内置标准 MIB**: 在应用中预置常用标准 MIB 文件（RFC1213-MIB, SNMPv2-MIB, IF-MIB 等）
2. **路径配置**: 在设置中允许用户配置 MIB 搜索路径
3. **依赖提示**: 解析失败时，明确提示缺少哪些依赖模块

**配置设计**:
```yaml
# config.yml
snmp:
  mib:
    store_dir: "./data/mibs"
    search_paths:
      - "./data/mibs/standard"
      - "./data/mibs/vendor"
    auto_import_dependencies: true
```

#### 风险 3: 并发安全性

**风险描述**: `gosmi` 使用 C 全局状态，在高并发导入场景下可能存在竞态条件。

**影响等级**: 低（gomib 已解决）

**应对策略**:
- `gomib` 设计为实例级别，天然支持并发
- `MIBParser` 使用 `sync.Mutex` 保护解析过程
- `MIBManager` 使用读写锁分离读写操作

### 4.2 业务风险

#### 风险 4: 现有数据兼容性

**风险描述**: 已导入的 MIB 数据是否需要重新导入。

**影响等级**: 低

**应对策略**:
- 数据模型 `models.MIBNode` 和 `models.MIBModule` 完全不变
- SQLite 数据库结构不变
- 无需迁移现有数据，仅重新解析即可

#### 风险 5: 前端兼容性

**风险描述**: 前端是否需要调整。

**影响等级**: 无

**应对策略**:
- UI 服务层接口不变
- View Model 结构不变
- 事件通知机制不变
- 前端完全无感知

---

## 5. 回滚方案

### 5.1 代码回滚

```bash
# 回滚到 main 分支
git checkout main

# 删除特性分支
git branch -D feature/snmp-cgo-free

# 强制推送（如果已推送）
git push origin --delete feature/snmp-cgo-free
```

### 5.2 依赖回滚

```bash
# 恢复 gosmi 依赖
go get github.com/sleepinggenius2/gosmi@v0.4.4
go mod tidy
```

### 5.3 数据回滚

- 无需数据回滚，数据库结构完全兼容

---

## 6. 验收标准

### 6.1 功能验收

| 验收项 | 验收标准 | 验证方法 |
|--------|----------|----------|
| MIB 导入 | 成功导入标准 RFC MIB | 导入 RFC1213-MIB，验证节点数 |
| MIB 树展示 | 树形结构正确显示 | 前端展开节点，验证层级关系 |
| OID 解析 | OID 正确解析为名称 | Trap 接收时显示 Trap 名称 |
| 错误处理 | 错误信息正确返回 | 导入错误 MIB，验证错误提示 |
| 并发导入 | 多文件并发导入成功 | 同时导入多个 MIB 文件 |

### 6.2 性能验收

| 验收项 | 验收标准 | 验证方法 |
|--------|----------|----------|
| 编译时间 | 减少 20% 以上 | 对比编译时间 |
| 二进制大小 | 减小 10% 以上 | 对比文件大小 |
| 内存占用 | 无明显增加 | 运行时内存监控 |
| 解析速度 | 差异在 10% 以内 | 基准测试对比 |

### 6.3 质量验收

| 验收项 | 验收标准 | 验证方法 |
|--------|----------|----------|
| 单元测试 | 覆盖率 ≥ 80% | `go test -cover` |
| 集成测试 | 所有测试用例通过 | 执行测试套件 |
| 代码规范 | 无 lint 错误 | `golangci-lint run` |
| 文档更新 | README 和架构文档更新 | 人工审核 |

---

## 7. 实施时间线

### 7.1 阶段划分

```mermaid
gantt
    title SNMP 迁移实施时间线
    dateFormat YYYY-MM-DD
    section 准备阶段
    创建分支           :a1, 2026-05-23, 1d
    依赖更新           :a2, after a1, 1d
    section 开发阶段
    重写 MIBParser     :b1, after a2, 2d
    调整 MIBManager    :b2, after b1, 1d
    单元测试           :b3, after b2, 1d
    section 验证阶段
    本地编译测试       :c1, after b3, 1d
    交叉编译测试       :c2, after c1, 1d
    集成测试           :c3, after c2, 2d
    section 发布阶段
    代码审查           :d1, after c3, 1d
    合并主分支         :d2, after d1, 1d
```

### 7.2 里程碑

| 里程碑 | 完成标志 | 交付物 |
|--------|----------|--------|
| M1: 依赖就绪 | go.mod 更新完成 | 更新后的依赖文件 |
| M2: 代码完成 | MIBParser 重写完成 | 新版解析器代码 |
| M3: 测试通过 | 所有测试用例通过 | 测试报告 |
| M4: 编译验证 | CGO-Free 编译成功 | 多平台二进制文件 |
| M5: 功能验收 | 集成测试通过 | 验收报告 |

---

## 8. 附录

### 8.1 gomib API 参考

**核心类型**:
```go
// Parser MIB 解析器
type Parser struct {
    // 私有字段
}

// Module MIB 模块
type Module struct {
    Name        string
    Description string
    Nodes       []*Node
}

// Node MIB 节点
type Node struct {
    Name        string
    OID         OID
    Kind        NodeKind
    Access      Access
    Status      Status
    Syntax      *Syntax
    Description string
}

// OID 对象标识符
type OID struct {
    // 私有字段
}

func (o OID) String() string

// NodeKind 节点类型
type NodeKind int

const (
    KindNode NodeKind = iota
    KindScalar
    KindTable
    KindRow
    KindColumn
    KindNotification
    KindGroup
    KindCompliance
)

// Access 访问权限
type Access int

const (
    AccessNotAccessible Access = iota
    AccessReadOnly
    AccessReadWrite
    AccessNotify
    AccessReportOnly
    AccessEventOnly
)

// Status 状态
type Status int

const (
    StatusCurrent Status = iota
    StatusDeprecated
    StatusObsolete
    StatusMandatory
    StatusOptional
)
```

**核心方法**:
```go
// NewParser 创建解析器实例
func NewParser() *Parser

// SetPath 设置 MIB 搜索路径
func (p *Parser) SetPath(paths []string)

// ParseFile 解析 MIB 文件
func (p *Parser) ParseFile(filePath string) (*Module, error)

// ParseDir 解析目录下所有 MIB 文件
func (p *Parser) ParseDir(dirPath string) ([]*Module, error)

// Walk 遍历节点树
func (n *Node) Walk(fn func(*Node) error) error
```

### 8.2 测试数据集

**标准 MIB 文件**:
- RFC1213-MIB.txt (MIB-II)
- SNMPv2-MIB.txt
- IF-MIB.txt
- IP-MIB.txt
- TCP-MIB.txt
- UDP-MIB.txt

**厂商 MIB 文件**:
- HUAWEI-ENTITY-EXTENT-MIB.mib
- CISCO-PROCESS-MIB.mib
- H3C-SMI.mib

### 8.3 参考文档

- [gomib GitHub 仓库](https://github.com/golangsnmp/gomib)
- [gosmi GitHub 仓库](https://github.com/sleepinggenius2/gosmi)
- [SMIv2 规范 RFC2578](https://tools.ietf.org/html/rfc2578)
- [NetWeaverGo 项目架构文档](./PROJECT_ARCHITECTURE.md)

---

## 9. 复核清单

### 9.1 技术复核

- [x] 依赖替换方案可行
- [x] API 映射关系正确（需实际验证 gomib API）
- [x] 数据模型无需变更
- [x] 服务层接口兼容
- [x] 前端无需修改
- [x] 并发安全性提升
- [x] 错误处理机制完善
- [x] Trap 功能专项测试已补充
- [x] Polling 功能专项测试已补充
- [x] 单元测试代码示例已补充
- [x] 性能基准测试代码已补充

### 9.2 风险复核

- [x] 私有 MIB 兼容性风险已识别
- [x] 依赖解析风险已识别
- [x] 回滚方案完整可行
- [x] 数据兼容性已确认
- [x] gomib 库成熟度风险已识别（新增前置验证步骤）

### 9.3 实施复核

- [x] 实施步骤逻辑顺序正确
- [x] 验证方法具体可执行
- [x] 验收标准明确可量化
- [x] 时间线合理
- [x] 前置调研步骤已补充

### 9.4 复核结论

**总体评价**: 实施方案完整、可行，已补充以下关键内容：

1. ✅ **gomib 库前置验证**: 新增 3.1.0 章节，包含 API 验证脚本和映射确认表
2. ✅ **Trap 功能专项测试**: 新增 3.5.3 章节，覆盖 Trap OID 解析场景
3. ✅ **Polling 功能专项测试**: 新增 3.5.4 章节，覆盖轮询 OID 解析场景
4. ✅ **单元测试代码**: 新增 3.6 章节，提供完整的测试代码示例
5. ✅ **性能基准测试**: 新增 benchmark 代码，用于性能对比

**遗留事项**:
- [ ] 需在实际迁移前执行 gomib API 验证脚本
- [ ] 需根据 gomib 实际 API 调整适配层代码
- [ ] 需准备测试数据集（RFC1213-MIB.txt 等）

**复核通过**: ✅ 方案已完整，可进入实施阶段

---

## 10. 附录：快速参考

### 10.1 关键命令速查

```bash
# 依赖管理
go get github.com/golangsnmp/gomib@latest
go mod tidy

# 编译验证
go build -o bin/netweaver.exe ./cmd/netweaver

# 交叉编译
$env:GOOS="linux"; $env:GOARCH="amd64"; $env:CGO_ENABLED="0"
go build -o bin/netweaver-linux ./cmd/netweaver

# 测试执行
go test ./internal/snmp/... -v -cover
go test -bench=. ./internal/snmp/... -benchmem
```

### 10.2 关键文件清单

| 文件 | 改造类型 | 优先级 |
|------|----------|--------|
| `go.mod` | 依赖更新 | P0 |
| `internal/snmp/mib_parser.go` | 完全重写 | P0 |
| `internal/snmp/mib_parser_test.go` | 新增测试 | P0 |
| `internal/snmp/mib_manager.go` | 无需修改 | - |
| `internal/snmp/oid_resolver.go` | 无需修改 | - |

### 10.3 验收检查表

```markdown
## 功能验收
- [ ] 标准 MIB 导入成功
- [ ] 厂商私有 MIB 导入成功
- [ ] MIB 树正确展示
- [ ] OID 解析正确
- [ ] Trap 接收正常
- [ ] Polling 采集正常

## 性能验收
- [ ] 编译时间减少
- [ ] 二进制体积减小
- [ ] 内存占用无明显增加
- [ ] 解析速度可接受

## 质量验收
- [ ] 单元测试覆盖率 ≥ 80%
- [ ] 无 lint 错误
- [ ] 文档已更新
```

---

**文档结束**
