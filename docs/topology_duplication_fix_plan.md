# 拓扑还原设备重复问题 — 分阶段修复方案

> 基于 [topology_duplication_issue_analysis.md](./topology_duplication_issue_analysis.md) 问题分析报告  
> 创建时间：2026-04-09

---

## 问题概述（速览）

| 维度   | 描述 |
|--------|------|
| 现象   | 5 台设备在拓扑图上渲染为 10 个节点，多出的 5 个以裸 IP 显示 |
| 根因   | **新构建器 `normalizeFacts` 未将 `DeviceIP` 写入 `DeviceByMgmtIP` 索引**；`resolveLLDPPeer` 在 `NeighborIP` 匹配 `DeviceByMgmtIP` 失败后直接返回裸 IP 字符串，且不再尝试其他维度匹配 |
| 对照   | **旧构建器** (`executor_impl.go:1484-1488`) 已正确将 `DeviceIP` 与 `MgmtIP` 同时写入 `deviceByMgmtIP`，因此旧路径不存在此 Bug |
| 影响   | 所有 `MgmtIP` 字段为空（或与 `DeviceIP` 不同）的设备，均可能被错误地重复渲染 |

---

## 修复策略总览

```
阶段 1 — 紧急修复（Hot Fix）
  ├─ 1.1  normalizeFacts: DeviceIP 入库 DeviceByMgmtIP
  ├─ 1.2  resolveLLDPPeer: 多维穿透匹配 + 防御性编程
  └─ 1.3  GetTopologyGraph: 节点去重守卫

阶段 2 — 加固与可观测性
  ├─ 2.1  添加 peer 解析诊断日志
  ├─ 2.2  补充单元测试覆盖
  └─ 2.3  旧构建器同步审查

阶段 3 — 长期架构演进（建议）
  ├─ 3.1  引入 UUID 节点主键
  ├─ 3.2  区分 Managed / Unmanaged 节点
  └─ 3.3  全量 IP/MAC 映射表支持多 IP 设备
```

---

## 阶段 1：紧急修复（Hot Fix）

### 1.1 `normalizeFacts` — 将 DeviceIP 加入 MgmtIP 索引

**文件**: `internal/taskexec/topology_builder.go`  
**位置**: `normalizeFacts` 函数，约第 270-295 行

**问题**: 当 `MgmtIP` 为空时，`DeviceByMgmtIP` 中没有该设备的条目，导致后续 LLDP 对端解析无法通过 `NeighborIP` 匹配到此设备。

**修复**: 在建立 MgmtIP 索引的同时，始终将 `DeviceIP` 也加入索引（与旧构建器 `executor_impl.go:1484-1488` 保持一致）。

```diff
  // 标准化设备
  for _, d := range input.Devices {
      info := &DeviceInfo{...}
      n.Devices[d.DeviceIP] = info

      // 建立名称索引
      if info.NormalizedName != "" {
          n.DeviceByName[info.NormalizedName] = d.DeviceIP
      }
+     // 建立 DeviceIP → 自身 的索引（保证 LLDP NeighborIP 能匹配到本设备）
+     if d.DeviceIP != "" {
+         n.DeviceByMgmtIP[d.DeviceIP] = d.DeviceIP
+     }
      // 建立 MgmtIP 索引
      if info.MgmtIP != "" {
          n.DeviceByMgmtIP[info.MgmtIP] = d.DeviceIP
      }
      // 建立 ChassisID 索引
      if info.ChassisID != "" {
          n.DeviceByChassisID[info.ChassisID] = d.DeviceIP
      }
  }
```

> **设计说明**: `DeviceIP` 索引放在 `MgmtIP` 索引之前。如果 `MgmtIP == DeviceIP`，后续写入不会产生冲突；如果 `MgmtIP != DeviceIP`，两个都会被索引，确保无论 LLDP 上报的是哪一个 IP 都能匹配。

---

### 1.2 `resolveLLDPPeer` — 多维穿透匹配 + 防御性编程

**文件**: `internal/taskexec/topology_builder.go`  
**位置**: `resolveLLDPPeer` 函数（第 507-534 行）

**问题1**: 当 `NeighborIP` 不为空但匹配 `DeviceByMgmtIP` 失败时，函数**直接 `return`**，不再尝试 `NeighborName` 和 `ChassisID` 匹配。  
**问题2**: 匹配优先级不够合理（`NeighborName` 在 `ChassisID` 之前，但 MAC 通常比名称更可靠）。

**修复**: 采用"穿透式"匹配——任何单一维度匹配失败后继续尝试下一个维度，直到所有维度都尝试完毕。同时调整优先级为 `IP → ChassisID → Name`，并在最终 fallback 时标记为 `unmanaged` 设备。

```go
// resolveLLDPPeer 解析 LLDP 对端设备
// 穿透式匹配：IP → ChassisID → Name，任一维度匹配失败不中断
func (b *TopologyBuilder) resolveLLDPPeer(lldp NormalizedLLDPNeighbor, n *NormalizedFacts) (string, string) {
    // 1. 优先尝试 NeighborIP 匹配（依赖 1.1 中 DeviceIP 已入库 DeviceByMgmtIP）
    if lldp.NeighborIP != "" {
        if deviceIP, ok := n.DeviceByMgmtIP[lldp.NeighborIP]; ok {
            return deviceIP, "neighbor_ip"
        }
        // ⚠ 不要在这里 return！继续尝试其他维度
    }

    // 2. 其次尝试 ChassisID 匹配（硬件 MAC 比设备名更可靠）
    if lldp.NeighborChassis != "" {
        if deviceIP, ok := n.DeviceByChassisID[lldp.NeighborChassis]; ok {
            return deviceIP, "chassis_id"
        }
    }

    // 3. 最后尝试 NeighborName 匹配
    if lldp.NeighborName != "" {
        normalizedName := strings.ToLower(strings.TrimSpace(lldp.NeighborName))
        if deviceIP, ok := n.DeviceByName[normalizedName]; ok {
            return deviceIP, "neighbor_name"
        }
    }

    // 4. 全部匹配失败 → 标记为未管设备 (Unmanaged Node)
    //    优先使用 IP 作为占位标识，其次 ChassisID，最后拼接 DeviceIP+LocalIf
    fallbackID := lldp.NeighborIP
    if fallbackID == "" {
        fallbackID = lldp.NeighborChassis
    }
    if fallbackID == "" {
        fallbackID = lldp.DeviceIP + ":" + lldp.LocalIf
    }
    return "unmanaged:" + fallbackID, "unknown_peer"
}
```

**要点说明**:

| 变更点 | 旧行为 | 新行为 | 理由 |
|--------|--------|--------|------|
| NeighborIP 匹配失败 | 直接 `return lldp.NeighborIP` | 继续尝试 ChassisID/Name | 防止因 IP 索引缺失而错失其他可靠匹配 |
| ChassisID 优先级 | 排在 Name **之后** | 排在 Name **之前** | 硬件 MAC 比设备名更稳定、更唯一 |
| 最终 fallback 前缀 | `"unknown:"` | `"unmanaged:"` | 语义更清晰，便于前端识别和渲染 |

---

### 1.3 `GetTopologyGraph` — 节点去重守卫

**文件**: `internal/taskexec/topology_query.go`  
**位置**: `GetTopologyGraph` 函数（约第 40-77 行）

**问题**: 从边中提取的 `BDeviceID` 如果恰好等于某设备的 `DeviceIP`（但字符串格式略有差异，如前导空格），会在 `nodeSet` 中产生两个看似相同的节点。

**修复**: 在节点生成阶段增加 `unmanaged:` 前缀的处理逻辑，并为所有从边中提取的 ID 做 `TrimSpace` 对齐。

```diff
  nodes := make([]models.GraphNode, 0, len(nodeSet))
  for id := range nodeSet {
      node := models.GraphNode{ID: id, Label: id}
      if d, ok := deviceMap[id]; ok {
          node.Label = chooseValue(d.DisplayName, d.Hostname, d.Model, d.DeviceIP)
          node.IP = d.DeviceIP
          node.Vendor = d.Vendor
          node.Model = d.Model
          node.Role = d.Role
          node.Site = d.Site
          node.SerialNumber = d.SerialNumber
      } else if strings.HasPrefix(id, "server:") {
          node.Label = strings.TrimPrefix(id, "server:")
          node.IP = strings.TrimPrefix(id, "server:")
          node.Role = "server-inferred"
          node.Vendor = "endpoint"
      } else if strings.HasPrefix(id, "terminal:") {
          node.Label = strings.TrimPrefix(id, "terminal:")
          node.Role = "terminal-inferred"
          node.Vendor = "endpoint"
+     } else if strings.HasPrefix(id, "unmanaged:") {
+         unmanagedID := strings.TrimPrefix(id, "unmanaged:")
+         node.Label = unmanagedID
+         node.IP = unmanagedID
+         node.Role = "unmanaged"
+         node.Vendor = "unknown"
      }
      nodes = append(nodes, node)
  }
```

---

## 阶段 2：加固与可观测性

### 2.1 添加 peer 解析诊断日志

**文件**: `internal/taskexec/topology_builder.go`

在 `resolveLLDPPeer` 中增加 `logger.Verbose` 日志，记录每次解析的匹配维度和结果，便于排查生产环境中的匹配失败问题。

```go
func (b *TopologyBuilder) resolveLLDPPeer(lldp NormalizedLLDPNeighbor, n *NormalizedFacts) (string, string) {
    // ... 匹配逻辑 ...
    
    // 每个 return 点前记录日志
    logger.Verbose("TopologyBuilder", lldp.DeviceIP,
        "resolveLLDPPeer: localIf=%s, neighborIP=%s, neighborChassis=%s, neighborName=%s → resolved=%s, source=%s",
        lldp.LocalIf, lldp.NeighborIP, lldp.NeighborChassis, lldp.NeighborName,
        resolvedDevice, resolvedSource)
    
    return resolvedDevice, resolvedSource
}
```

### 2.2 补充单元测试覆盖

**文件**: `internal/taskexec/taskexec_test.go`

新增以下测试用例：

| 用例编号 | 场景 | 预期 |
|---------|------|------|
| TC-01 | `MgmtIP` 为空，`NeighborIP == DeviceIP` | 正确匹配，不产生重复节点 |
| TC-02 | `MgmtIP != DeviceIP`，`NeighborIP == MgmtIP` | 通过 `MgmtIP` 索引正确匹配 |
| TC-03 | `MgmtIP != DeviceIP`，`NeighborIP == DeviceIP` | 通过 `DeviceIP` 索引正确匹配 |
| TC-04 | `NeighborIP` 匹配失败，`ChassisID` 匹配成功 | 穿透到 ChassisID 维度匹配 |
| TC-05 | `NeighborIP` 匹配失败，`NeighborName` 匹配成功 | 穿透到 Name 维度匹配 |
| TC-06 | 所有维度均失败 | 返回 `unmanaged:` 前缀，拓扑图中显示为未管设备 |
| TC-07 | 5 台设备互联，所有 `MgmtIP` 为空 | 拓扑图 nodes 数量 == 5（而非 10） |

### 2.3 旧构建器同步审查

**文件**: `internal/taskexec/executor_impl.go`

审查旧构建器 `buildRunTopology`（第 1440-1779 行）中的 LLDP 对端解析逻辑（第 1521-1535 行）：

```go
// 旧构建器的解析逻辑（第 1521-1535 行）
if strings.TrimSpace(n.NeighborIP) != "" {
    remoteDevice = strings.TrimSpace(n.NeighborIP)  // 直接用 IP
    resolutionSource = "neighbor_ip"
} else if candidate := deviceByName[...]; candidate != "" {
    remoteDevice = candidate
    resolutionSource = "neighbor_name"
} else {
    remoteDevice = "unknown:..."
}
```

**发现**: 旧构建器第 1484-1488 行已将 `DeviceIP` 入库 `deviceByMgmtIP`，但其 LLDP 解析逻辑仍然存在以下问题：
1. NeighborIP 匹配是简单的字符串赋值，没有通过 `deviceByMgmtIP` 查找
2. 缺少 ChassisID 维度匹配

**建议**: 如果旧构建器仍在使用，也需要应用穿透式匹配逻辑。但长期建议是完全迁移到新构建器。

---

## 阶段 3：长期架构演进（建议）

> 以下为架构层面的改进建议，不影响当前 Bug 的修复，可作为后续迭代规划。

### 3.1 引入 UUID 节点主键

**痛点**: 当前以 `DeviceIP` 作为拓扑节点的唯一标识。在多 VRF（虚拟路由转发）或多分支网络中，IP 地址可能冲突。

**建议方案**:

```go
// 设备 ID 生成为系统全局唯一 UUID
type DeviceNode struct {
    NodeID      string   // UUID，全局唯一主键
    DeviceIPs   []string // 设备可能拥有的多个 IP（Loopback、SVI、管理口）
    ChassisID   string   // 硬件标识
    Hostname    string   // 设备名
    // ...
}
```

- 拓扑边的 `ADeviceID` / `BDeviceID` 引用 `NodeID`（UUID）
- 无论通过 IP / MAC / Name 匹配，最终都映射到同一个 UUID
- 前端通过 `DeviceIPs[0]` 或 `Hostname` 展示

### 3.2 区分 Managed / Unmanaged 节点

**机制**: 如果通过 LLDP 发现了某设备 B，但 B 不在采集列表中，则 B 是边界设备。

**建议**:

```go
type GraphNode struct {
    // ...现有字段...
    NodeType string `json:"nodeType"` // "managed" | "unmanaged" | "inferred"
}
```

- `managed`: 在采集列表中且已成功采集
- `unmanaged`: LLDP 发现但不在采集列表中的边界设备
- `inferred`: 通过 FDB/ARP 推断的终端设备

**前端渲染建议**: 
- `managed` → 实线框 + 彩色图标
- `unmanaged` → 虚线框 + 灰色图标  
- `inferred` → 小圆点

### 3.3 全量 IP/MAC 映射表

**痛点**: 核心交换机通常有多个 SVI（VLAN 接口 IP）或 Loopback IP，LLDP 报文中的 Management Address 可能只是其中一个。

**建议**:
1. 采集每台设备的接口列表时，提取所有 `IPAddress` 字段
2. 在 `normalizeFacts` 中建立 `AllDeviceIPs map[string]string`（`key: 任意一个IP → value: DeviceIP`）
3. LLDP 对端解析时，查询 `AllDeviceIPs` 进行兜底匹配

```go
// normalizeFacts 中新增
n.AllDeviceIPs = make(map[string]string)
for _, d := range input.Devices {
    n.AllDeviceIPs[d.DeviceIP] = d.DeviceIP
    if d.MgmtIP != "" {
        n.AllDeviceIPs[d.MgmtIP] = d.DeviceIP
    }
}
// 从接口表中补充
for _, iface := range input.Interfaces {
    if iface.IPAddress != "" {
        n.AllDeviceIPs[iface.IPAddress] = iface.DeviceIP
    }
}
```

---

## 修复影响评估

| 阶段 | 改动范围 | 风险等级 | 影响面 |
|------|---------|---------|--------|
| 1.1  | `normalizeFacts` 新增 2 行 | 🟢 低 | 仅影响新构建器的索引建立 |
| 1.2  | `resolveLLDPPeer` 重写 | 🟡 中 | 改变了匹配优先级和 fallback 策略 |
| 1.3  | `GetTopologyGraph` 新增分支 | 🟢 低 | 仅影响节点渲染，向后兼容 |
| 2.1  | 日志增强 | 🟢 低 | 无逻辑变更 |
| 2.2  | 测试新增 | 🟢 低 | 纯测试代码 |
| 2.3  | 旧构建器审查 | 🟡 中 | 可选，取决于是否仍启用旧路径 |
| 3.x  | 架构改造 | 🔴 高 | 涉及数据模型变更、前端适配 |

---

## 验证计划

### 自动化测试

```bash
# 运行拓扑相关单元测试（编译产物放 dist 目录）
go test -v ./internal/taskexec/... -run "TestResolveLLDPPeer|TestBuildRunTopology" -outputdir dist
```

### 手动验证

1. 使用 5 台设备（`MgmtIP` 全部置空）创建拓扑采集任务
2. 执行采集并构建拓扑
3. 验证拓扑图节点数 == 5
4. 检查每条边的 `BDeviceID` 均能正确关联到设备信息
5. 验证前端界面无重复节点显示

---

**文档版本**: v1.0  
**编写时间**: 2026-04-09  
**关联文档**: [问题分析报告](./topology_duplication_issue_analysis.md)
