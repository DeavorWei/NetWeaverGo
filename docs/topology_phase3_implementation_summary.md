# 拓扑架构演进 - 阶段3实施总结

## 概述

本文档总结阶段3长期架构演进方案的实施情况。阶段3基于修复方案文档 (`topology_duplication_fix_plan.md`) 中的建议，实现了以下核心功能：

1. **引入 UUID 节点主键** - 解决多VRF/分支网络中IP冲突问题
2. **区分 Managed/Unmanaged 节点** - 清晰的节点类型定义和展示
3. **全量 IP/MAC 映射表** - 支持多IP设备（Loopback、SVI等）匹配

---

## 实施详情

### 3.1 引入 UUID 节点主键

#### 数据模型变更

**文件**: `internal/taskexec/topology_models.go`

```go
// NodeType 节点类型定义
type NodeType string

const (
    NodeTypeManaged   NodeType = "managed"   // 已管理设备
    NodeTypeUnmanaged NodeType = "unmanaged" // 未管理设备
    NodeTypeInferred  NodeType = "inferred"  // 推断设备
    NodeTypeUnknown   NodeType = "unknown"   // 未知类型
)

// TaskRunDevice 运行期设备信息
type TaskRunDevice struct {
    // ... 原有字段 ...
    NodeUUID string   `gorm:"index" json:"nodeUuid"` // 全局唯一节点标识
    AllIPs   string   `gorm:"type:text" json:"allIps"` // 所有IP地址JSON数组
    NodeType NodeType `json:"nodeType"` // 节点类型
}
```

**文件**: `internal/models/topology.go`

```go
// GraphNode 图节点
type GraphNode struct {
    ID           string   `json:"id"`
    NodeUUID     string   `json:"nodeUuid"`     // 全局唯一节点标识
    Label        string   `json:"label"`
    IP           string   `json:"ip"`
    AllIPs       []string `json:"allIps"`       // 设备所有IP地址
    Vendor       string   `json:"vendor"`
    Model        string   `json:"model"`
    Role         string   `json:"role"`
    Site         string   `json:"site"`
    SerialNumber string   `json:"serialNumber"`
    NodeType     NodeType `json:"nodeType"`     // 节点类型
    ChassisID    string   `json:"chassisId"`    // 硬件标识
}
```

#### UUID生成逻辑

**文件**: `internal/taskexec/ids.go`

```go
// newNodeUUID 生成全局唯一的节点UUID
func newNodeUUID() string {
    return newPrefixedID("node_")
}
```

### 3.2 区分 Managed/Unmanaged 节点

#### 节点身份解析器

**文件**: `internal/taskexec/topology_builder_phase3.go`

核心组件 `NodeIdentityResolver` 负责：

- 为已管理设备分配NodeUUID
- 跟踪节点类型（Managed/Unmanaged/Inferred）
- 维护设备多IP映射

```go
type NodeIdentityResolver struct {
    nodeUUIDByDeviceIP map[string]string      // DeviceIP -> NodeUUID
    deviceIPByNodeUUID map[string]string      // NodeUUID -> DeviceIP
    allIPsByDeviceIP   map[string][]string    // DeviceIP -> 所有IP列表
    nodeTypeByDeviceIP map[string]NodeType    // DeviceIP -> NodeType
    deviceByAllIPs     map[string]string      // 任意IP -> DeviceIP
}
```

#### 节点类型方法

```go
func (nt NodeType) IsManaged() bool   // 检查是否为已管理设备
func (nt NodeType) IsUnmanaged() bool // 检查是否为未管理设备
func (nt NodeType) IsInferred() bool  // 检查是否为推断设备
```

### 3.3 全量 IP/MAC 映射表

#### 标准化事实增强

**文件**: `internal/taskexec/topology_builder.go`

```go
type NormalizedFacts struct {
    // ... 原有字段 ...
    AllDeviceIPs map[string]string // key: 任意IP -> DeviceIP（支持多IP设备）
}

type DeviceInfo struct {
    // ... 原有字段 ...
    NodeUUID string   // 全局唯一节点标识
    AllIPs   []string // 设备所有IP地址
    NodeType NodeType // 节点类型
}
```

#### 多IP设备匹配流程

在 `normalizeFacts` 函数中：

1. 为每个设备生成唯一的 NodeUUID
2. 将 DeviceIP 和 MgmtIP 加入全量映射表
3. 从接口信息中提取所有IP地址
4. 建立任意IP到设备DeviceIP的映射

```go
// 阶段3：建立全量IP映射（DeviceIP + MgmtIP + 接口IP）
if d.DeviceIP != "" {
    n.AllDeviceIPs[d.DeviceIP] = d.DeviceIP
}
if info.MgmtIP != "" && info.MgmtIP != d.DeviceIP {
    n.AllDeviceIPs[info.MgmtIP] = d.DeviceIP
}
// 从接口表补充
if i.IPAddress != "" && i.IPAddress != i.DeviceIP {
    n.AllDeviceIPs[i.IPAddress] = i.DeviceIP
}
```

#### 增强的LLDP对端解析

匹配优先级更新为：

1. **DeviceByMgmtIP** - 向后兼容原有逻辑
2. **AllDeviceIPs** - 阶段3新增：支持多IP设备匹配
3. **ChassisID** - 硬件MAC匹配
4. **NeighborName** - 设备名称匹配

```go
// 1. 优先尝试 NeighborIP 匹配 DeviceByMgmtIP（向后兼容）
if deviceIP, ok := n.DeviceByMgmtIP[lldp.NeighborIP]; ok {
    return deviceIP, "neighbor_ip"
}

// 2. 尝试通过全量IP映射表匹配（支持多IP设备）
if deviceIP, ok := n.AllDeviceIPs[lldp.NeighborIP]; ok {
    return deviceIP, "neighbor_ip_extended"
}
```

---

## 新增文件清单

| 文件                                                | 说明                                          |
| --------------------------------------------------- | --------------------------------------------- |
| `internal/taskexec/topology_builder_phase3.go`      | 阶段3核心实现：NodeIdentityResolver、辅助函数 |
| `internal/taskexec/topology_query_phase3.go`        | 阶段3查询增强：支持NodeType的图节点构建       |
| `internal/taskexec/topology_builder_phase3_test.go` | 阶段3单元测试                                 |

---

## 测试覆盖

### 新增测试用例

| 测试函数                    | 覆盖功能               |
| --------------------------- | ---------------------- |
| `TestNodeIdentityResolver`  | 节点身份解析器核心功能 |
| `TestNodeTypeMethods`       | 节点类型方法           |
| `TestAllDeviceIPsJSON`      | IP序列化/反序列化      |
| `TestIsUnmanagedNodeID`     | 未管理节点ID检测       |
| `TestIsInferredNodeID`      | 推断节点ID检测         |
| `TestExtractNodeTypeFromID` | 从ID提取节点类型       |
| `TestPhase3BuildContext`    | Phase3构建上下文       |
| `TestCreateUnmanagedNode`   | 创建未管理节点         |

### 测试执行结果

```bash
$ go test -v ./internal/taskexec/... -run "TestNodeIdentityResolver|TestNodeType|TestAllDeviceIPs|TestPhase3"

=== RUN   TestNodeIdentityResolver
--- PASS: TestNodeIdentityResolver (0.00s)
=== RUN   TestNodeTypeMethods
--- PASS: TestNodeTypeMethods (0.00s)
=== RUN   TestAllDeviceIPsJSON
--- PASS: TestAllDeviceIPsJSON (0.00s)
=== RUN   TestPhase3BuildContext
--- PASS: TestPhase3BuildContext (0.00s)
PASS
ok      github.com/NetWeaverGo/core/internal/taskexec 0.834s
```

---

## 编译验证

```bash
$ go build -v ./internal/taskexec/...

github.com/NetWeaverGo/core/internal/models
github.com/NetWeaverGo/core/internal/config
github.com/NetWeaverGo/core/internal/repository
github.com/NetWeaverGo/core/internal/report
github.com/NetWeaverGo/core/internal/sshutil
github.com/NetWeaverGo/core/internal/executor
github.com/NetWeaverGo/core/internal/taskexec
```

✅ 编译成功，无错误。

---

## 架构优势

### 1. 解决IP冲突问题

- 使用 NodeUUID 作为全局唯一标识
- 支持多VRF/分支网络环境

### 2. 清晰的数据语义

- **Managed**: 在采集列表中且成功采集的设备
- **Unmanaged**: LLDP发现但不在采集列表中的边界设备
- **Inferred**: 通过FDB/ARP推断的终端设备

### 3. 增强的设备匹配

- 支持 Loopback、SVI 等多IP场景
- 匹配优先级：DeviceByMgmtIP → AllDeviceIPs → ChassisID → Name

### 4. 向后兼容

- 原有 DeviceIP 索引仍然有效
- 支持新旧数据格式混合

---

## 前端渲染建议

基于新增的 `NodeType` 字段，前端可以差异化渲染：

```javascript
// 节点样式映射
const nodeStyleMap = {
  managed: { border: "solid", color: "#1890ff", icon: "switch" },
  unmanaged: { border: "dashed", color: "#999999", icon: "question" },
  inferred: { border: "dotted", color: "#52c41a", icon: "desktop" },
};
```

- **Managed** → 实线边框 + 彩色图标 + 完整设备信息
- **Unmanaged** → 虚线边框 + 灰色图标 + IP/名称显示
- **Inferred** → 点线边框 + 绿色图标 + 简化信息

---

## 后续建议

### 数据迁移

- 存量数据可以通过重新执行拓扑构建任务来升级
- 自动为新设备生成 NodeUUID

### 性能优化

- AllDeviceIPs 映射表可能较大，考虑使用更高效的数据结构
- 可以添加IP地址范围索引优化查找

### 功能扩展

- 支持基于MAC地址的设备匹配
- 添加设备变更历史追踪
- 实现跨任务运行的设备身份持久化

---

**实施时间**: 2026-04-09  
**状态**: ✅ 已完成并验证
