# 拓扑还原工具 — 功能模块内部逻辑架构说明书

## 一、整体架构概览

### 1.1 架构分层

拓扑还原工具采用 **分层架构**，从底向上分为四层：

```
┌─────────────────────────────────────────────────────────┐
│                    UI 层 (Wails Frontend)                │
│    TopologyCommandService + TaskGroupService             │
│    ┌──────────────────────────────────────────────┐     │
│    │ View Models (topology_command_view_models.go) │     │
│    └──────────────────────────────────────────────┘     │
├─────────────────────────────────────────────────────────┤
│                 业务逻辑层 (taskexec)                     │
│    TopologyCommandResolver → 命令解析与厂商决策          │
│    TaskExecutionService   → 拓扑图查询与边详情           │
├─────────────────────────────────────────────────────────┤
│                 配置层 (config)                           │
│    topology_command.go → 拓扑命令 CRUD 与种子管理        │
│    device_profile.go   → 设备画像（厂商/命令模板）       │
│    task_group.go       → 任务模板管理                    │
├─────────────────────────────────────────────────────────┤
│                 模型层 (models)                           │
│    topology.go         → 拓扑图/节点/边/证据 DTO         │
│    topology_command.go → 字段目录/覆盖/厂商命令 ORM 模型 │
└─────────────────────────────────────────────────────────┘
```

### 1.2 核心数据流

系统包含三条核心数据流：

1. **命令配置流**：UI → TopologyCommandService → TopologyCommandResolver → config → DB
2. **拓扑查询流**：UI → TaskExecutionService.GetTopologyGraph() → DB → TopologyGraphView
3. **任务启动流**：UI → TaskGroupService.StartTaskGroup() → TaskLaunchService → Resolve → Execute → Build Topology

## 二、模型层详解

### 2.1 拓扑命令模型 ([`internal/models/topology_command.go`](internal/models/topology_command.go))

**职责**：定义拓扑命令体系的底层数据模型，包含字段目录规范、任务级覆盖配置和厂商命令 ORM 模型。

#### 核心结构体

| 结构体 | 用途 |
|--------|------|
| [`TopologyFieldSpec`](internal/models/topology_command.go) | 拓扑固定字段目录定义，描述每个拓扑采集字段的元信息 |
| [`TopologyTaskFieldOverride`](internal/models/topology_command.go) | 任务级字段覆盖配置，允许用户在任务维度覆盖特定字段的命令 |
| [`TopologyVendorFieldCommand`](internal/models/topology_command.go) | 厂商默认字段命令映射 ORM 模型，持久化到 `topology_vendor_field_commands` 表 |

#### TopologyFieldSpec 字段详解

| 字段 | 类型 | 含义 |
|------|------|------|
| `FieldKey` | string | 字段唯一标识，如 `"version"`, `"lldp_neighbor"` |
| `Name` | string | 中文显示名称 |
| `Phase` | string | 所属阶段（当前均为 `"collect"`） |
| `Required` | bool | 是否必需字段 |
| `ParserBinding` | string | 绑定的解析器名称，用于自动解析原始输出 |
| `DefaultEnabled` | bool | 是否默认启用 |
| `Description` | string | 字段描述 |

#### 预定义字段目录

系统内置 6 个固定拓扑字段（`defaultTopologyFieldCatalog`）：

| FieldKey | 名称 | Required | ParserBinding |
|----------|------|----------|---------------|
| `version` | 系统版本 | ✅ | `version` |
| `sysname` | 设备名称 | ✅ | `sysname` |
| `interface_brief` | 接口概要 | ✅ | `interface_brief` |
| `lldp_neighbor` | LLDP 邻居 | ✅ | `lldp_neighbor` |
| `arp_all` | ARP 表 | ✅ | `arp_all` |
| `eth_trunk` | 聚合链路 | ❌ | `eth_trunk` |

#### TopologyVendorFieldCommand ORM 映射

- **表名**：`topology_vendor_field_commands`（由 `TableName()` 指定）
- **唯一索引**：`(vendor, field_key)` 联合唯一
- **语义**：每个厂商的每个拓扑字段对应一条命令配置记录

### 2.2 拓扑图模型 ([`internal/models/topology.go`](internal/models/topology.go))

**职责**：定义拓扑图的前端展示 DTO，包括节点（Node）、边（Edge）、边证据、决策轨迹等视图模型。

#### 核心结构体

| 结构体 | 用途 |
|--------|------|
| [`TopologyGraphView`](internal/models/topology.go) | 拓扑图整体视图，包含 TaskID、Nodes 和 Edges |
| [`GraphNode`](internal/models/topology.go) | 拓扑图节点，包含设备标识、IP、厂商、型号、角色等信息 |
| [`GraphEdge`](internal/models/topology.go) | 拓扑图边，连接两个节点，包含接口、类型、状态、置信度等 |
| [`TopologyEdgeDetailView`](internal/models/topology.go) | 边的详细视图，包含两端设备和证据列表 |
| [`TopologyEdgeExplainView`](internal/models/topology.go) | 边的完整决策解释视图 |
| [`EdgeEvidence`](internal/models/topology.go) | 边的证据，记录命令输出中的链路关系 |
| [`TopologyCandidateView`](internal/models/topology.go) | 候选边视图，记录候选边的评分和状态 |
| [`TopologyDecisionTraceView`](internal/models/topology.go) | 决策轨迹视图，记录决策类型、结果和原因 |
| [`TopologyBuildResult`](internal/models/topology.go) | 拓扑构建结果统计 |

#### GraphNode 字段详解

| 字段 | 类型 | 含义 |
|------|------|------|
| `ID` | string | 节点唯一标识 |
| `NodeUUID` | string | 节点 UUID |
| `Label` | string | 显示标签 |
| `IP` | string | 主 IP 地址 |
| `AllIPs` | []string | 所有 IP 地址列表 |
| `Vendor` | string | 设备厂商 |
| `Model` | string | 设备型号 |
| `Role` | string | 设备角色 |
| `NodeType` | NodeType | 节点类型 |
| `ChassisID` | string | 底盘 ID |
| `MACAddress` | string | MAC 地址 |

#### GraphEdge 字段详解

| 字段 | 类型 | 含义 |
|------|------|------|
| `ID` | string | 边唯一标识 |
| `Source` | string | 源节点 ID |
| `Target` | string | 目标节点 ID |
| `SourceIf` | string | 源端接口名称 |
| `TargetIf` | string | 目标端接口名称 |
| `EdgeType` | string | 边类型 |
| `Status` | string | 边状态 |
| `Confidence` | float64 | 置信度评分 |

#### 节点类型（NodeType）

| 常量 | 值 | 含义 |
|------|-----|------|
| `NodeTypeManaged` | `"managed"` | 已管理设备（在采集列表中） |
| `NodeTypeUnmanaged` | `"unmanaged"` | 未管理设备（LLDP 发现但不在采集列表） |
| `NodeTypeInferred` | `"inferred"` | 推断设备（通过 FDB/ARP 推断出的终端） |
| `NodeTypeUnknown` | `"unknown"` | 未知类型 |

#### TopologyBuildResult 字段详解

| 字段 | 类型 | 含义 |
|------|------|------|
| `TotalEdges` | int | 总边数 |
| `ConfirmedEdges` | int | 已确认边数 |
| `SemiConfirmedEdges` | int | 半确认边数 |
| `InferredEdges` | int | 推断边数 |
| `ConflictEdges` | int | 冲突边数 |
| `BuildTime` | time.Duration | 构建耗时 |

#### Phase A 决策可解释性扩展

[`TopologyEdgeExplainView`](internal/models/topology.go) 提供了边的完整决策解释：
- **候选边列表**（[`TopologyCandidateView`](internal/models/topology.go)）：包括所有候选（保留/淘汰/合并/冲突）
- **决策轨迹**（[`TopologyDecisionTraceView`](internal/models/topology.go)）：记录决策类型（`conflict_resolution`/`candidate_selection`/`edge_merge`）、结果、原因和依据

## 三、UI 服务层详解

### 3.1 前端视图模型 ([`internal/ui/topology_command_view_models.go`](internal/ui/topology_command_view_models.go))

**职责**：定义前端 UI 展示所需的 DTO 结构，用于 Wails 前后端数据传输。

#### 视图模型列表

| 结构体 | 用途 | JSON 字段 |
|--------|------|----------|
| `TopologyResolvedCommandView` | 单条解析后的拓扑命令视图 | fieldKey, displayName, command, timeoutSec, enabled, commandSource, parserBinding, resolvedVendor, vendorSource, required, description |
| `TopologyCommandResolutionView` | 命令解析结果集 | resolvedVendor, vendorSource, profileVendor, commands[] |
| `TopologyPreviewDeviceView` | 单设备命令预览 | deviceId, deviceIP, inventoryVendor, resolution |
| `TopologyCommandPreviewView` | 完整命令预览 | supportedVendors, fieldCatalog, taskOverrides, defaultResolution, devices[] |
| `TopologyVendorCommandItemView` | 厂商字段命令配置项 | fieldKey, displayName, parserBinding, description, required, command, timeoutSec, enabled, notes, source, updatedAt |
| `TopologyVendorCommandSetView` | 厂商命令配置集 | vendor, commands[] |
| `TopologyVendorCommandSaveRequest` | 保存请求 | vendor, commands[] |

#### 命令来源标识

视图模型中的 `Source` 字段用于标识命令来源：

| 值 | 含义 |
|-----|------|
| `"field_default"` | 来自字段目录默认值 |
| `"vendor_config"` | 来自数据库中的厂商配置 |
| `"profile_seed"` | 来自设备画像种子 |

### 3.2 拓扑命令服务 ([`internal/ui/topology_command_service.go`](internal/ui/topology_command_service.go))

**职责**：作为 Wails 暴露给前端的拓扑命令配置服务，负责命令预览、厂商配置管理、命令解析等。是 **前端与业务逻辑之间的桥接层**。

#### 结构体定义

```go
type TopologyCommandService struct {
    wailsApp *application.App              // Wails 应用实例
    repo     repository.DeviceRepository   // 设备资产仓库
}
```

#### 函数清单

| 函数 | 功能 | 参数 | 返回值 |
|------|------|------|--------|
| `GetSupportedTopologyVendors()` | 获取系统支持的拓扑厂商列表 | 无 | `[]string` |
| `GetTopologyFieldCatalog()` | 获取拓扑字段目录 | 无 | `[]models.TopologyFieldSpec` |
| `GetVendorCommandConfig(vendor)` | 查询厂商命令配置 | `vendor string` | `(*TopologyVendorCommandSetView, error)` |
| `SaveVendorCommandConfig(request)` | 保存厂商命令配置 | `TopologyVendorCommandSaveRequest` | `(*TopologyVendorCommandSetView, error)` |
| `ResetVendorCommandConfig(vendor)` | 重置厂商命令为系统种子 | `vendor string` | `(*TopologyVendorCommandSetView, error)` |
| `PreviewTopologyCommands(...)` | 命令预览（含设备级别） | `taskVendor, deviceIDs, overrides` | `(*TopologyCommandPreviewView, error)` |
| `GetTaskTopologyVendors()` | 获取任务可选厂商列表 | 无 | `[]string` |

#### 【核心】GetVendorCommandConfig() 数据合并逻辑

该函数执行 **三层数据合并**：

```
优先级从高到低：
1. 数据库厂商配置 (vendor_config)  ← 用户自定义保存的
2. 设备画像种子 (profile_seed)     ← 系统内置的厂商命令模板
3. 字段目录默认 (field_default)    ← 仅显示字段元信息，无命令
```

**执行流程**：
1. 调用 `EnsureTopologyCommandSeeds()` 确保种子数据已初始化
2. 调用 `resolveVendorProfile(vendor)` 获取规范化厂商名 + 设备画像
3. 从 DB 加载厂商命令记录 → 构建 `recordMap[fieldKey]`
4. 从画像加载命令模板 → 构建 `profileMap[fieldKey]`
5. 遍历字段目录，按优先级合并

#### 【核心】SaveVendorCommandConfig() 保存逻辑

保存时执行 **合并-验证-持久化** 流程：
1. 获取当前已有配置作为 base
2. 将请求中的修改项 merge 到 base
3. 遍历字段目录构建最终持久化列表
4. 校验：启用的字段命令不能为空、超时必须 > 0
5. 调用 `config.SaveTopologyVendorFieldCommands()` 覆盖写入（事务性先删后插）

#### 【核心】PreviewTopologyCommands() 预览逻辑

该函数实现了 **设备级别的命令解析预览**：
1. 创建 `TopologyCommandResolver` 实例
2. 先解析默认设备（`device=nil`）的命令 → `defaultResolution`
3. 遍历指定 `deviceIDs`，逐设备查询资产 → 逐设备调用 `resolver.Resolve()`
4. 结果按设备 IP 排序

#### 辅助函数

| 函数 | 功能 |
|------|------|
| `findDevicesByIDs()` | 根据 ID 批量查询设备资产 |
| `convertTopologyResolution()` | 将内部解析结果转换为前端视图模型 |
| `resolveVendorProfile()` | 规范化厂商名并获取设备画像 |
| `profileCommandsByKey()` | 将画像命令列表转为 `map[fieldKey]CommandSpec` |
| `normalizeTopologyTaskOverrides()` | 规范化任务覆盖配置（去重、排序） |
| `clampTimeout()` | 超时值钳制（≤0 时使用默认 30s） |

### 3.3 任务组服务 ([`internal/ui/task_group_service.go`](internal/ui/task_group_service.go))

**职责**：负责任务模板（TaskGroup）的完整生命周期管理——CRUD、详情聚合、执行启动。与拓扑的关联点在于 `TaskGroup` 模型包含 `TopologyVendor`、`AutoBuildTopology` 等拓扑相关字段。

#### 结构体定义

```go
type TaskGroupService struct {
    wailsApp      *application.App
    repo          repository.DeviceRepository        // 设备资产仓库
    taskexec      *taskexec.TaskExecutionService     // 任务执行服务
    launchService *taskexec.TaskLaunchService        // 任务启动服务
}
```

#### 函数清单

| 函数 | 功能 | 关键参数 | 返回值 |
|------|------|----------|--------|
| `ListTaskGroups()` | 获取所有任务模板列表（含运行状态聚合） | 无 | `([]TaskGroupListView, error)` |
| `GetTaskGroup(id)` | 获取单个任务模板 | `id uint` | `(*models.TaskGroup, error)` |
| `GetTaskGroupDetail(id)` | 获取任务详情（含设备/命令组聚合） | `id uint` | `(*TaskGroupDetailViewModel, error)` |
| `CreateTaskGroup(group)` | 创建任务组 | `models.TaskGroup` | `(*models.TaskGroup, error)` |
| `UpdateTaskGroup(id, group)` | 更新任务组（活跃运行时禁止） | `id, models.TaskGroup` | `(*models.TaskGroup, error)` |
| `DeleteTaskGroup(id)` | 删除任务组（活跃运行时禁止） | `id uint` | `error` |
| `StartTaskGroup(id)` | 启动任务执行 | `id uint` | `(string, error)` — 返回 `runID` |

#### ListTaskGroups() 列表聚合逻辑

该函数实现了 **任务模板 + 运行状态** 的联合聚合：
1. 从 `config.ListTaskGroups()` 加载所有模板
2. 调用 `latestRunsByTaskGroup()` 获取每个模板的最新运行
3. 调用 `activeRunCounts()` 统计活跃运行数
4. 遍历模板，组装 `TaskGroupListView`，包含状态、最新运行 ID/状态、是否可编辑等
5. 按创建时间倒序排序

#### buildTaskGroupDetail() 详情构建逻辑

该函数实现了 **多数据源聚合**：
1. 加载所有设备资产 → `assetMap[deviceID]`
2. 加载所有命令组 → `commandMap[commandGroupID]`
3. 遍历 TaskGroup.Items，逐项聚合设备信息和命令信息
4. 标记缺失的设备和命令组
5. 组装 `TaskGroupDetailViewModel`

#### 拓扑相关联动

`TaskGroupListView` 中包含以下拓扑相关字段：
- `TopologyVendor` — 拓扑任务的厂商
- `AutoBuildTopology` — 是否自动构建拓扑
- `TaskType` — 任务类型（`"topology"` 或 `"normal"`）

## 四、业务逻辑层详解

### 4.1 命令解析器 ([`internal/taskexec/topology_command_resolver.go`](internal/taskexec/topology_command_resolver.go))

**职责**：拓扑命令解析的核心引擎，负责 **厂商决策** 和 **命令解析**。所有 UI 层和执行层的命令解析最终都通过此模块。

#### 厂商解析优先级

厂商解析采用四级回退策略：

```
优先级从高到低：
1. task       — 任务级指定的 vendor（用户在任务配置中指定）
2. inventory — 设备资产中的 vendor（设备入库时记录）
3. detect    — 从设备元数据推断的 vendor
4. fallback_default — 使用默认厂商 "huawei"
```

#### 命令解析四层优先级

命令解析采用四层优先级合并策略：

```
优先级从高到低：
1. task_override  — 任务级覆盖（用户在任务配置中指定）
2. vendor_config  — 厂商配置（数据库中保存的用户自定义命令）
3. profile_seed   — 画像种子（设备画像中内置的命令模板）
4. builtin_seed   — 内置种子（画像加载失败时的回退）
```

#### 关键常量

| 常量 | 值 | 含义 |
|------|-----|------|
| `TopologyVendorSourceTask` | `"task"` | 厂商来自任务配置 |
| `TopologyVendorSourceInventory` | `"inventory"` | 厂商来自设备资产 |
| `TopologyVendorSourceDetect` | `"detect"` | 厂商从设备元数据推断 |
| `TopologyVendorSourceFallback` | `"fallback_default"` | 使用默认厂商 |

## 五、配置层详解

### 5.1 拓扑命令配置持久化 ([`internal/config/topology_command.go`](internal/config/topology_command.go))

**职责**：拓扑命令配置的数据访问层，通过 GORM 操作 `topology_vendor_field_commands` 表。

#### 函数清单

| 函数 | 功能 |
|------|------|
| `GetTopologyVendorFieldCommands(vendor)` | 查询指定厂商的所有字段命令 |
| `ListTopologyVendorFieldCommands()` | 列出所有厂商字段命令 |
| `SaveTopologyVendorFieldCommands(vendor, commands)` | **覆盖保存**（事务：先删后插） |
| `EnsureTopologyVendorCommandSeeds(seed)` | **增量种子**（仅插入不存在的记录） |

## 六、模块间依赖与调用关系

### 6.1 依赖关系图

```
┌─────────────────────────────────────────────────────────────────┐
│                     Frontend (Wails Bindings)                    │
│                         Vue.js UI                                │
└────────────┬──────────────────────────┬──────────────────────────┘
             │                          │
             ▼                          ▼
┌────────────────────────┐  ┌────────────────────────┐
│ TopologyCommandService │  │   TaskGroupService     │
│   拓扑命令配置服务       │  │   任务模板管理服务      │
└──────────┬─────────────┘  └──────────┬─────────────┘
           │                           │
           ▼                           ▼
┌──────────────────────────────────────────────────────────────────┐
│              TopologyCommandResolver (命令解析引擎)                │
│              TaskLaunchService (任务启动服务)                      │
└──────────┬──────────────────────────────────┬────────────────────┘
           │                                  │
           ▼                                  ▼
┌────────────────────────┐  ┌────────────────────────┐
│  config/topology_cmd   │  │  config/device_profile │
│  厂商命令 CRUD          │  │  设备画像管理           │
└──────────┬─────────────┘  └──────────┬─────────────┘
           │                           │
           ▼                           ▼
┌──────────────────────────────────────────────────────────────────┐
│                    Database (SQLite)                               │
│  topology_vendor_field_commands | device_profiles | task_groups   │
└──────────────────────────────────────────────────────────────────┘
```

### 6.2 调用链示例

#### 场景 1：用户打开拓扑命令配置页面

```
Frontend → GetVendorCommandConfig("huawei")
  → EnsureTopologyCommandSeeds()              // 确保种子已初始化
    → config.EnsureTopologyVendorCommandSeeds(seeds)
  → resolveVendorProfile("huawei")            // 获取设备画像
    → config.GetDeviceProfileByVendor("huawei")
  → taskexec.GetTopologyFieldCatalog()        // 获取字段目录
  → config.GetTopologyVendorFieldCommands("huawei")  // 从 DB 读取
  → profileCommandsByKey(profile)             // 从画像提取命令模板
  → 合并三层数据 → 返回 TopologyVendorCommandSetView
```

#### 场景 2：用户预览拓扑命令

```
Frontend → PreviewTopologyCommands("huawei", [1,2,3], overrides)
  → resolver.Resolve("huawei", nil, overrides)     // 默认解析
  → findDevicesByIDs([1,2,3])                       // 查询设备资产
  → 遍历设备:
      resolver.Resolve("huawei", &device1, overrides)  // 每设备独立解析
        → resolveVendor(taskVendor, device)            // 厂商决策
        → 从 DB 加载 vendor_commands
        → 从画像加载 profile_commands
        → 合并 task_override
        → 返回 TopologyCommandResolution
  → 返回 TopologyCommandPreviewView（含默认解析 + 每设备解析）
```

#### 场景 3：启动拓扑任务

```
Frontend → StartTaskGroup(taskGroupID)
  → TaskLaunchService.StartTaskGroup()
    → 从 config 加载 TaskGroup
    → TopologyCommandResolver.Resolve(taskVendor, nil, overrides)
    → DeviceCollectExecutor 遍历设备:
        resolver.Resolve(taskVendor, &device, overrides)  // 确定每设备命令
        → 执行命令采集
        → 解析输出
        → 构建拓扑边
    → 持久化拓扑图到 DB
```

## 七、数据流转全景

### 7.1 命令配置流

```
Frontend → TopologyCommandService
  → TopologyCommandResolver.EnsureTopologyCommandSeeds()
  → config.GetTopologyVendorFieldCommands(vendor)  → DB
  → config.GetDeviceProfileByVendor(vendor)        → DB
  → 合并三层数据
  → 返回 TopologyVendorCommandSetView → Frontend
```

### 7.2 命令解析流（执行时）

```
Frontend → PreviewTopologyCommands(vendor, deviceIDs, overrides)
  → TopologyCommandResolver.Resolve(vendor, device, overrides)
    → resolveVendor() → 厂商决策
    → 从 DB 加载 vendor_commands
    → 从画像加载 profile_commands
    → 四层优先级合并
    → 返回 TopologyCommandResolution
  → 返回 TopologyCommandPreviewView → Frontend
```

### 7.3 拓扑图查询流

```
Frontend → TaskExecutionService.GetTopologyGraph(runID)
  → DB: SELECT task_topology_edges WHERE runID
  → DB: SELECT task_run_devices WHERE runID
  → 构建 GraphNode[] + GraphEdge[]
  → 返回 TopologyGraphView → Frontend
```

## 八、总结

| 维度 | 说明 |
|------|------|
| **架构风格** | 分层架构 + 服务导向，UI 层通过 Wails 暴露为前端可调用的服务方法 |
| **核心设计** | 四层命令解析优先级（task_override > vendor_config > profile_seed > builtin_seed），确保灵活性和可回退性 |
| **厂商决策** | 四级回退（task > inventory > detect > fallback），通过设备画像匹配厂商 |
| **数据持久化** | 厂商命令使用 GORM + SQLite，表级事务保证一致性 |
| **前端交互** | 通过 View Model 层解耦内部模型和前端 DTO，支持 JSON 序列化 |
| **拓扑图模型** | 支持 4 种节点类型（managed/unmanaged/inferred/unknown），边具有置信度评分和决策可解释性（Phase A） |
| **任务联动** | TaskGroup 中通过 `TopologyVendor` + `AutoBuildTopology` 字段关联拓扑功能，`StartTaskGroup()` 触发完整采集-解析-建图流程 |

---

## 九、核心执行引擎详解（taskexec 层）

拓扑还原的执行引擎位于 `internal/taskexec/` 目录，包含 7 个核心文件，实现了从「命令编译 → 事实采集/解析 → 拓扑构建 → 图查询」的完整流水线。

### 9.1 文件职责总览

```
topology_command_resolver.go  → 命令解析器（厂商决策 + 命令解析）
topology_compiler.go          → 拓扑编译器（TaskDefinition → ExecutionPlan）
topology_facts.go             → 事实持久化器（解析输出 → 结构化事实入库）
topology_builder.go           → 拓扑构建器（事实 → 候选 → 评分 → 冲突消解 → 边）
topology_build_models.go      → 构建数据模型（候选/评分/轨迹/配置）
topology_models.go            → 运行时 ORM 模型（设备/输出/事实/边）
topology_query.go             → 拓扑查询服务（DB → 前端视图）
```

| 文件 | 核心职责 |
|------|----------|
| `topology_command_resolver.go` | 厂商决策 + 命令解析，生成每台设备的最终采集命令列表 |
| `topology_compiler.go` | 将 TaskDefinition 编译为三阶段 ExecutionPlan（采集→解析→构建） |
| `topology_facts.go` | 解析器输出 → 结构化事实的映射 + 数据库持久化（在线/重放共享） |
| `topology_builder.go` | 拓扑构建核心引擎：事实标准化 → 候选生成 → 评分 → 冲突消解 → 边落库 |
| `topology_build_models.go` | 构建阶段的数据模型：候选、评分明细、决策轨迹、配置、统计 |
| `topology_models.go` | 运行期 ORM 模型：设备、原始输出、解析事实（LLDP/FDB/ARP/接口/聚合）、拓扑边 |
| `topology_query.go` | 拓扑图查询服务：从 DB 组装前端视图（Graph/EdgeDetail/Explain/DeviceDetail） |

---

## 十、拓扑编译流程详解（topology_compiler.go）

### 10.1 编译器结构

`TopologyTaskCompiler` 实现了编译器接口，通过 `Supports()` 判断是否支持 `RunKindTopology` 类型的任务。

```
TopologyTaskCompiler
├── options *CompileOptions    // 编译选项（默认并发数等）
└── Compile(ctx, def) → *ExecutionPlan
```

### 10.2 三阶段编译模型

`Compile()` 将 `TaskDefinition` 编译为三阶段 `ExecutionPlan`：

```
Stage 1: DeviceCollect (设备信息采集)
  ├── Unit: collect-{deviceIP_1}  → Steps: [command_1, command_2, ...]
  ├── Unit: collect-{deviceIP_2}  → Steps: [command_1, command_2, ...]
  └── ...（每台设备一个 Unit，并发执行）

Stage 2: Parse (信息解析)
  ├── Unit: parse-{deviceIP_1}    → Steps: [parse]
  ├── Unit: parse-{deviceIP_2}    → Steps: [parse]
  └── ...（每台设备一个 Unit，全并发）

Stage 3: TopologyBuild (拓扑构建)
  └── Unit: build-1               → Steps: [build]
      （单 Unit，单线程，汇聚所有设备数据）
```

### 10.3 编译阶段详解

#### Stage 1: buildCollectStage()

- **并发度**：`config.MaxWorkers` 或 `options.DefaultDiscoveryWorkers`
- **Unit 数量**：每台设备一个 Unit
- **Step 生成**：由 `buildCollectSteps()` 根据 `config.ResolvedCommands` 生成，每个启用的命令对应一个 `StepPlan`
- **Step 参数**：包含 `displayName`、`commandSource`、`resolvedVendor`、`parserBinding`、`fieldOverrides` 等

#### Stage 2: buildParseStage()

- **并发度**：等于 Unit 数量（全并发）
- **Step 参数**：包含 `resolvedVendor` 和 `vendorSource`（由 `topologyParseMetadata()` 从命令列表中提取）

#### Stage 3: buildTopologyBuildStage()

- **并发度**：1（单线程构建）
- **Unit Kind**：`UnitKindDataset`（非设备级，汇聚级）
- **Target**：`{Type: "task_run", Key: "all_devices"}`

### 10.4 编译配置来源

`TopologyTaskConfig` 由 `TaskDefinition.Config` JSON 反序列化而来，包含：
- `DeviceIPs` — 设备 IP 列表
- `GroupNames` — 设备组名
- `ResolvedCommands` — 已解析的命令列表（由 `TopologyCommandResolver` 生成）
- `MaxWorkers`、`TimeoutSec`、`Vendor`、`FieldOverrides` 等

---

## 十一、拓扑构建流程详解（topology_builder.go）

### 11.1 构建器结构

`TopologyBuilder` 是拓扑构建的核心引擎，实现了 10 步流水线。

```
TopologyBuilder
├── db     *gorm.DB
├── config TopologyBuildConfig
└── Build(ctx, runID) → *TopologyBuildOutput
```

### 11.2 十步构建流水线

`Build()` 方法执行以下步骤：

```
Step 1: collectBuildInputs    — 从 DB 收集所有构建输入
Step 2: createFactSnapshot    — 创建事实快照（含 SHA256 哈希）
Step 3: normalizeFacts        — 标准化事实数据
Step 4: buildLLDPCandidates   — 生成 LLDP 候选边
Step 5: buildFDBARPCandidates — 生成 FDB/ARP 推断候选边
Step 6: 合并候选              — 合并所有候选
Step 7: enrichCandidatesWithInterfaceFacts — 用接口事实丰富候选
Step 8: resolveCandidatesGlobal — 全局冲突消解
Step 9: materializeEdges      — 生成最终边
Step 10: persistResults       — 持久化结果到 DB
```

### 11.3 Step 1: 收集构建输入

`collectBuildInputs()` 使用 GORM Session 保证读一致性，从 DB 查询以下数据：

| 数据源 | ORM 模型 | 表名 |
|--------|----------|------|
| 设备 | `TaskRunDevice` | `task_run_devices` |
| LLDP 邻居 | `TaskParsedLLDPNeighbor` | `task_parsed_lldp_neighbors` |
| FDB 条目 | `TaskParsedFDBEntry` | `task_parsed_fdb_entries` |
| ARP 条目 | `TaskParsedARPEntry` | `task_parsed_arp_entries` |
| 聚合成员 | `TaskParsedAggregateMember` | `task_parsed_aggregate_members` |
| 聚合组 | `TaskParsedAggregateGroup` | `task_parsed_aggregate_groups` |
| 接口 | `TaskParsedInterface` | `task_parsed_interfaces` |

### 11.4 Step 2: 创建事实快照

`createFactSnapshot()` 创建 `TopologyFactSnapshot`，包含：
- 各类事实数量统计
- **SHA256 事实哈希**：对所有设备/LLDP/FDB/ARP 数据排序后计算哈希，用于快速比对两次构建输入是否相同
- 构建配置快照（JSON 序列化）

### 11.5 Step 3: 标准化事实

`normalizeFacts()` 将原始输入转换为 `NormalizedFacts` 结构，包含：

**索引映射**（用于后续快速查找）：

| 索引 | Key | Value | 用途 |
|------|-----|-------|------|
| `DeviceByName` | NormalizedName | DeviceIP | LLDP 邻居名匹配 |
| `DeviceByMgmtIP` | DeviceIP/MgmtIP | DeviceIP | LLDP IP 匹配 + ARP IP 交叉匹配 |
| `DeviceByChassisID` | ChassisID | DeviceIP | LLDP ChassisID 匹配 |
| `ARPMACToDevice` | MAC | DeviceIP | FDB MAC → 设备映射 |
| `ARPMACToIP` | MAC | IP | FDB MAC → IP 映射 |
| `AggregateMembers` | DeviceIP\|MemberPort | AggregateName | 接口聚合解析 |
| `Interfaces` | DeviceIP\|InterfaceName | InterfaceInfo | 接口状态查询 |

**ARP MAC → Device 映射策略**（O(N) 复杂度）：
1. 预计算 ChassisID 标准化 MAC → DeviceIP 索引
2. 方式一：通过 ChassisID MAC 匹配已知设备
3. 方式二：通过 ARP IP 交叉匹配已知设备（补充 ChassisID 无法覆盖的场景）

### 11.6 Step 4: LLDP 候选生成

`buildLLDPCandidates()` 为每条 LLDP 邻居生成候选边：

**核心流程**：
1. `resolveLLDPPeer()` — 穿透式匹配对端设备
2. `resolveAggregateInterface()` — 解析逻辑聚合接口
3. `buildCandidateKey()` — 生成候选键（排序保证双向一致）
4. `scoreLLDPCandidate()` — 计算评分
5. 双向合并：相同候选键的双向 LLDP 证据合并

#### 对端设备解析（穿透式匹配）

`resolveLLDPPeer()` 按优先级尝试三种匹配：

```
1. NeighborIP 匹配  → DeviceByMgmtIP[NeighborIP]     → source: "neighbor_ip"
2. ChassisID 匹配   → DeviceByChassisID[ChassisID]    → source: "chassis_id"
3. NeighborName 匹配 → DeviceByName[NormalizedName]   → source: "neighbor_name"
4. 全部失败          → "unmanaged:{fallbackID}"        → source: "unknown_peer"
```

> **关键设计**：任一维度匹配失败不中断，继续尝试下一维度。最终失败时生成 `unmanaged:` 前缀的占位节点。

#### LLDP 评分机制

`scoreLLDPCandidate()` 计算 `ScoreBreakdown`：

| 评分项 | 权重常量 | 条件 |
|--------|----------|------|
| 基础分（已知对端） | `wLLDPBaseSingleSide = 75.0` | IP/Name/ChassisID 匹配成功 |
| 基础分（未知对端） | `75.0 × 0.8 = 60.0` | 匹配失败 |
| IP 匹配加分 | `wLLDPIPMatch = 5.0` | source == "neighbor_ip" |
| 名称匹配加分 | `wLLDPNameMatch = 3.0` | source == "neighbor_name" |
| Chassis 匹配加分 | `wLLDPChassisMatch = 5.0` | ChassisID 在已知设备中 |
| 远端接口存在加分 | `wLLDPRemoteIfPresent = 2.0` | NeighborPort 非空 |
| 聚合接口加分 | `wAggLACPModeBonus = 5.0` | 本地或远端有聚合接口 |
| **双向确认加分** | `wLLDPBidirectionalBonus = 25.0` | 双向 LLDP 证据合并后 |

**LLDP 单边最高分**：75 + 5 + 3 + 5 + 2 + 5 = 95.0
**LLDP 双向最高分**：95 + 25 = 120.0（但置信度上限 1.0）

### 11.7 Step 5: FDB/ARP 候选生成

`buildFDBARPCandidates()` 基于 FDB 表和 ARP 表推断链路：

**核心流程**：
1. 按 `(DeviceIP, Interface)` 分组 FDB 条目
2. 对每组 FDB，通过 `resolveFDBRemoteEndpoint()` 解析远端端点
3. 统计候选对端出现次数（MAC 计数）
4. 超过 `MaxInferenceCandidates`（默认 5）则跳过
5. 为每个候选对端生成候选边

#### FDB 远端端点解析

`resolveFDBRemoteEndpoint()` 返回 `(nodeID, kind, ip, mac)`：

```
MAC 在 ARPMACToDevice 中 → (DeviceIP, "device", "", mac)
MAC 在 ARPMACToIP 中     → ("endpoint:{IP}", "endpoint", IP, mac)
MAC 未知                  → ("unknown:{MAC}", "unknown", "", mac)
```

#### FDB/ARP 评分机制

`scoreFDBARPCandidate()`：

| 评分项 | 权重常量 | 条件 |
|--------|----------|------|
| 基础分 | `wFDBBaseScore = 20.0` | 所有候选 |
| MAC 数量因子 | `wFDBMACCountFactor = 2.0` | 每个 MAC × 2.0 |
| 设备类型加分 | `wFDBDeviceBonus = 30.0` | remoteKind == "device" |
| 端点类型加分 | `wFDBEndpointBonus = 10.0` | remoteKind == "endpoint" |
| 未知类型加分 | `wFDBUnknownBonus = 3.0` | remoteKind == "unknown" |
| VLAN 一致性加分 | `wFDBVLANBonus = 3.0` | 所有 MAC 在同一 VLAN |
| 聚合接口加分 | `wFDBLogicalIfBonus = 5.0` | 本地有聚合接口 |
| 远端 IP 加分 | `wFDBRemoteIPBonus = 5.0` | 有远端 IP |

**置信度范围**：0.35 ~ 0.95（FDB 推断永远不会达到 `confirmed` 阈值 0.95）

### 11.8 Step 7: 接口事实丰富

`enrichCandidatesWithInterfaceFacts()` 检查候选边两端接口的 up/down 状态：

- 本端接口 up → `wIfUpBonus = 3.0`
- 远端接口 up → `wIfUpBonus = 3.0`

### 11.9 Step 8: 全局冲突消解

`resolveCandidatesGlobal()` 是拓扑构建的核心决策算法：

**算法详解**：

1. **端点冲突图构建**：每个候选的 A/B 端作为 endpointKey，记录哪些候选共享同一端点
2. **全局排序**：按 `TotalScore` 降序，分数相同按 `CandidateID` 排序（保证确定性）
3. **逐候选决策**：
   - 两端均空闲 → `retained`，标记端点为已占用
   - 至少一端被占 → `rejected`，记录淘汰原因
4. **冲突窗口检测**（`traceConflictWindow()`）：
   - 找出与保留候选共享端点、且分数差在 `ConflictWindow`（默认 10.0）内的候选
   - 标记为 `conflict`（不直接淘汰，保留供人工审查）
   - 锁定 conflict 候选的端点，防止后续候选误占

### 11.10 Step 9: 边物化

`materializeEdges()` 将保留的候选转换为 `TaskTopologyEdge`：

**边状态判定**：
```
confidence > 0.95  → "confirmed"
confidence > 0.75  → "semi_confirmed"
candidate.status == "conflict" → "conflict"
其他               → "inferred"
```

### 11.11 Step 10: 持久化

`persistResults()` 在事务中保存：
1. 清理旧边 → 保存新边（`task_topology_edges`）
2. 可选保存候选（`topology_edge_candidates`，由 `SaveCandidates` 配置控制）
3. 可选保存决策轨迹（`topology_decision_traces`，由 `SaveDecisionTraces` 配置控制）
4. 保存事实快照（`topology_fact_snapshots`）

### 11.12 入口函数

`BuildTopologyWithNewLogic()` 是外部调用入口：
1. 创建默认 `TopologyBuildConfig`
2. 从运行时配置加载参数（`config.ResolveTopologyConfig()`）
3. 创建 `TopologyBuilder` 并执行 `Build()`

---

## 十二、构建数据模型详解（topology_build_models.go）

### 12.1 核心数据模型

#### TopologyFactSnapshot — 事实快照

| 字段 | 类型 | 含义 |
|------|------|------|
| `TaskRunID` | string | 关联的运行 ID（唯一索引） |
| `SnapshotAt` | time.Time | 快照时间 |
| `DeviceCount` / `LLDPCount` / `FDBCount` / `ARPCount` / `AggCount` / `IfCount` | int | 各类事实数量 |
| `FactHash` | string | SHA256 事实摘要哈希 |
| `CollectionPlanSummary` | string | 采集计划元数据快照 |
| `BuildConfigSnapshot` | string | 构建配置快照 |

#### TopologyEdgeCandidate — 候选边

| 字段 | 类型 | 含义 |
|------|------|------|
| `CandidateID` | string | 候选唯一标识 |
| `ADeviceID` / `BDeviceID` | string | A/B 端设备 ID |
| `AIf` / `BIf` | string | A/B 端物理接口 |
| `LogicalAIf` / `LogicalBIf` | string | A/B 端逻辑接口（聚合组名） |
| `EdgeType` | string | "physical" / "logical" |
| `Source` | string | "lldp" / "fdb_arp" / "manual" |
| `Status` | string | "pending" / "retained" / "rejected" / "merged" / "conflict" |
| `ScoreBreakdown` | string | JSON 序列化的评分明细 |
| `TotalScore` | float64 | 总评分 |
| `Features` | []string | 特征标签（如 "lldp_bidirectional", "aggregate_mapped"） |
| `EvidenceRefs` | []EdgeEvidence | 证据引用列表 |
| `DecisionReason` | string | 为何被保留或淘汰 |
| `BDeviceMACs` | []string | B 端设备 MAC 地址列表 |

#### ScoreBreakdown — 评分明细

```
ScoreBreakdown
├── LLDPScore      LLDPScoreDetail       // LLDP 评分
├── FDBARPScore    FDBARPScoreDetail     // FDB/ARP 评分
├── InterfaceScore InterfaceScoreDetail  // 接口评分
├── AggregateScore AggregateScoreDetail  // 聚合评分
├── TotalScore     float64               // 总分
├── Confidence     float64               // 置信度（0-1）
└── Version        string                // 评分版本
```

#### TopologyDecisionTrace — 决策轨迹

| 字段 | 类型 | 含义 |
|------|------|------|
| `DecisionType` | string | "conflict_resolution" / "candidate_selection" / "edge_merge" |
| `DecisionGroup` | string | 决策分组标识（如同一端点的多个候选） |
| `DecisionResult` | string | "retained" / "rejected" / "merged" / "conflict" |
| `RetainedCandidateIDs` | []string | 被保留的候选 ID |
| `RejectedCandidateIDs` | []string | 被淘汰的候选 ID |
| `DecisionReason` | string | 决策原因明细 |
| `DecisionBasis` | string | 决策依据（量化数据） |

### 12.2 评分权重常量

```
// LLDP 评分权重
wLLDPBaseSingleSide      = 75.0  // 单边基础分
wLLDPBidirectionalBonus  = 25.0  // 双向确认额外加分
wLLDPChassisMatch        = 5.0   // chassis 匹配加分
wLLDPNameMatch           = 3.0   // 名称匹配加分
wLLDPIPMatch             = 5.0   // IP 匹配加分
wLLDPRemoteIfPresent     = 2.0   // 远端接口存在加分

// FDB/ARP 评分权重
wFDBBaseScore      = 20.0  // 基础分
wFDBMACCountFactor = 2.0   // MAC 数量因子
wFDBDeviceBonus    = 30.0  // 设备类型加分
wFDBEndpointBonus  = 10.0  // 端点类型加分
wFDBUnknownBonus   = 3.0   // 未知类型加分
wFDBVLANBonus      = 3.0   // VLAN 一致性加分
wFDBLogicalIfBonus = 5.0   // 聚合接口加分
wFDBRemoteIPBonus  = 5.0   // 远端 IP 加分

// 接口评分权重
wIfUpBonus = 3.0           // 接口 up 加分

// 聚合评分权重
wAggLACPModeBonus = 5.0    // LACP 模式加分

// 置信度阈值
confidenceConfirmed     = 0.95  // 确认阈值
confidenceSemiConfirmed = 0.75  // 半确认阈值
```

### 12.3 构建配置

`TopologyBuildConfig`：

| 字段 | 默认值 | 含义 |
|------|--------|------|
| `MaxInferenceCandidates` | 5 | 每个 FDB 组的最大推断候选数 |
| `ConflictWindow` | 10.0 | 分差小于此值视为竞争 |
| `SaveCandidates` | true | 是否保存候选轨迹到 DB |
| `SaveDecisionTraces` | true | 是否保存决策轨迹到 DB |

---

## 十三、拓扑事实收集详解（topology_facts.go）

### 13.1 事实持久化器

`TopologyFactsPersister` 是在线执行器和重放执行器共享的解析结果持久化逻辑。

#### 核心方法

| 方法 | 功能 | 关键逻辑 |
|------|------|----------|
| `SaveDeviceIdentity()` | 保存设备身份信息 | 更新 `task_run_devices` 表的 vendor/model/hostname/mgmt_ip/chassis_id |
| `SaveParsedFacts()` | 保存解析后的事实数据 | 事务性：先清理旧数据 → 批量插入（batch=100）接口/LLDP/FDB/ARP/聚合 |
| `ClearDeviceFacts()` | 清除指定设备的事实数据 | 事务性删除 6 类事实表 |
| `ClearAllFacts()` | 清除指定运行的所有事实和拓扑数据 | 事务性删除 10 类表 |

#### SaveParsedFacts() 事务流程

```
开始事务
  → clearDeviceFactsInTx（清理旧数据）
  → 批量保存接口（TaskParsedInterface）
  → 批量保存 LLDP（TaskParsedLLDPNeighbor）
  → 批量保存 FDB（TaskParsedFDBEntry）
  → 批量保存 ARP（TaskParsedARPEntry）
  → 保存聚合组（TaskParsedAggregateGroup）
  → 批量保存聚合成员（TaskParsedAggregateMember）
提交事务
```

### 13.2 命令输出映射

`MapCommandOutput()` 将解析器输出映射为 `ParsedFactBatch`：

| commandKey | 解析器方法 | 输出类型 |
|------------|-----------|----------|
| `version` | `mapper.ToDeviceInfo()` | DeviceIdentity |
| `sysname` | 直接字段合并 | DeviceIdentity 补充 |
| `interface_brief` | `mapper.ToInterfaces()` | []InterfaceFact |
| `lldp_neighbor` / `lldp_neighbor_verbose` | `mapper.ToLLDP()` | []LLDPFact |
| `arp_all` / `arp` | `mapper.ToARP()` | []ARPFact |
| `eth_trunk` / `eth_trunk_verbose` | `mapper.ToAggregate()` | []AggregateFact |

> 每个事实都附加 `CommandKey` 和 `RawRefID` 用于追溯。

---

## 十四、拓扑运行时模型详解（topology_models.go）

### 14.1 ORM 模型总览

| 结构体 | 表名 | 用途 |
|--------|------|------|
| `TaskRunDevice` | `task_run_devices` | 运行期设备信息 |
| `TaskRawOutput` | `task_raw_outputs` | 运行期命令输出索引 |
| `TaskParsedInterface` | `task_parsed_interfaces` | 解析后的接口事实 |
| `TaskParsedLLDPNeighbor` | `task_parsed_lldp_neighbors` | 解析后的 LLDP 事实 |
| `TaskParsedFDBEntry` | `task_parsed_fdb_entries` | 解析后的 FDB 事实 |
| `TaskParsedARPEntry` | `task_parsed_arp_entries` | 解析后的 ARP 事实 |
| `TaskParsedAggregateGroup` | `task_parsed_aggregate_groups` | 解析后的聚合组 |
| `TaskParsedAggregateMember` | `task_parsed_aggregate_members` | 解析后的聚合成员 |
| `TaskTopologyEdge` | `task_topology_edges` | 运行期拓扑边 |

### 14.2 关键模型字段详解

#### TaskRunDevice — 运行期设备

| 字段 | 类型 | 含义 |
|------|------|------|
| `TaskRunID` | string | 关联的运行 ID |
| `DeviceIP` | string | 设备主 IP |
| `AllIPs` | string | 设备所有 IP（文本） |
| `Vendor` / `VendorSource` | string | 厂商 / 厂商来源 |
| `Hostname` / `NormalizedName` | string | 主机名 / 标准化名称 |
| `ChassisID` | string | 底盘 ID（LLDP 匹配关键） |
| `MgmtIP` | string | 管理 IP |
| `NodeType` | models.NodeType | 节点类型 |

#### TaskTopologyEdge — 拓扑边

| 字段 | 类型 | 含义 |
|------|------|------|
| `ADeviceID` / `BDeviceID` | string | A/B 端设备 ID |
| `AIf` / `BIf` | string | A/B 端物理接口 |
| `LogicalAIf` / `LogicalBIf` | string | A/B 端逻辑接口 |
| `Status` | string | "confirmed" / "semi_confirmed" / "inferred" / "conflict" |
| `Confidence` | float64 | 置信度（0-1） |
| `DiscoveryMethods` | []string | 发现方法标签 |
| `Evidence` | []EdgeEvidence | 证据列表 |
| `ConfidenceBreakdown` | string | JSON 评分明细 |
| `DecisionReason` | string | 决策原因 |

---

## 十五、拓扑查询服务详解（topology_query.go）

### 15.1 查询服务概述

`TaskExecutionService` 上的查询方法负责从 DB 组装前端视图。

### 15.2 核心查询方法

#### GetTopologyGraph() — 拓扑图查询

**输入**：`runID string`
**输出**：`*models.TopologyGraphView`

**执行流程**：
1. 查询 edges + devices
2. 构建 nodeSet
3. 遍历 nodeSet 构建 GraphNode
4. 节点类型判断
5. 构建 GraphEdge 列表
6. 返回 TopologyGraphView

**节点类型判断逻辑**：

| BDeviceID 前缀 | NodeType | Label 来源 | 特殊处理 |
|----------------|----------|------------|----------|
| 无前缀（在 deviceMap 中） | `managed` | DisplayName > Hostname > Model > DeviceIP | 填充 Vendor/Model/Role/Site |
| `endpoint:` | `inferred` | IP 部分 | 提取 MAC 从边信息 |
| `server:` | `inferred` | IP 部分 | 兼容旧格式 |
| `terminal:` | `inferred` | IP 或 MAC | 兼容旧格式 |
| `unknown:` | `unknown` | MAC 部分 | — |
| `unmanaged:` | `unmanaged` | ID 部分 | — |

**无边诊断**：当查询到 0 条边时，额外查询 LLDP 事实数、原始输出数、解析成功/失败数，输出诊断日志。

#### GetTopologyEdgeDetail() — 边详情查询

返回 `TopologyEdgeDetailView`，包含：
- A/B 端设备节点
- 接口、类型、状态、置信度
- 证据列表、评分明细、决策原因

#### GetTopologyEdgeExplain() — 边解释视图

返回 `TopologyEdgeExplainView`，包含：
- 完整边详情
- 关联候选列表（共享端点的所有候选）
- 决策轨迹

#### GetTopologyDeviceDetail() — 设备详情查询

返回 `parser.ParsedResult`，聚合查询：
- 设备身份（`task_run_devices`）
- 接口事实（`task_parsed_interfaces`）
- LLDP 事实（`task_parsed_lldp_neighbors`）
- FDB 事实（`task_parsed_fdb_entries`）
- ARP 事实（`task_parsed_arp_entries`）
- 聚合事实（`task_parsed_aggregate_groups` + `task_parsed_aggregate_members`）

#### ListTopologyCollectionPlans() — 采集计划查询

从 Artifact 表查询拓扑采集计划快照文件，反序列化为 `TopologyCollectionPlanArtifact`。

### 15.3 辅助函数

| 函数 | 功能 |
|------|------|
| `getGraphNode()` | 根据 deviceID 构建 GraphNode（支持 endpoint/server/terminal 前缀） |
| `chooseValue()` | 从多个字符串中返回第一个非空值 |
| `extractMACFromEdges()` | 从边信息中提取推断节点的 MAC 地址 |
| `isValidIP()` | 简单 IP 地址验证 |

---

## 十六、命令解析器增强详解（topology_command_resolver.go）

### 16.1 核心结构体

#### ResolvedTopologyCommand — 解析后的命令

| 字段 | 类型 | 含义 |
|------|------|------|
| `FieldKey` | string | 字段标识（如 "lldp_neighbor"） |
| `DisplayName` | string | 显示名称 |
| `Command` | string | 最终命令字符串 |
| `TimeoutSec` | int | 超时秒数 |
| `Enabled` | bool | 是否启用 |
| `CommandSource` | string | 命令来源标识 |
| `ParserBinding` | string | 绑定的解析器 |
| `ResolvedVendor` | string | 解析后的厂商 |
| `VendorSource` | string | 厂商来源 |
| `Required` | bool | 是否必需 |

#### TopologyCommandResolution — 解析结果集

| 字段 | 类型 | 含义 |
|------|------|------|
| `ResolvedVendor` | string | 最终解析的厂商 |
| `VendorSource` | string | 厂商来源 |
| `ProfileVendor` | string | 画像厂商 |
| `Commands` | []ResolvedTopologyCommand | 解析后的命令列表 |

### 16.2 Resolve() 完整执行流程

1. **确保种子初始化**：调用 `EnsureTopologyCommandSeeds()`，使用 `sync.Once` 保证幂等
2. **厂商解析**：调用 `resolveVendor()`，四级回退
3. **加载设备画像**：`config.GetDeviceProfileByVendor(resolvedVendor)`
4. **加载厂商命令**：`config.GetTopologyVendorFieldCommands(profile.Vendor)` → 构建 `vendorCommandMap`
5. **加载画像命令**：从 `profile.Commands` 构建 `profileCommandMap`
6. **规范化覆盖**：`normalizeTopologyOverrides()` → `overrideMap`
7. **遍历字段目录**：对每个 `TopologyFieldSpec`，按四层优先级合并
8. **返回结果**：`TopologyCommandResolution`

### 16.3 detectVendorFromDevice() 深度解析

该函数实现了 **多维度文本匹配**：
1. 从设备 6 个字段（Vendor/DisplayName/Description/Role/Site/Group）拼接文本
2. 先检查是否包含任何厂商关键词（`containsVendorHint()`）
3. 优先使用 `config.DetectVendorFromOutput()` 进行输出级检测
4. 降级使用 `inferVendorFromHint()` 进行关键词推断

### 16.4 命令来源标识常量

| 常量 | 值 | 含义 |
|------|-----|------|
| `TopologyCommandSourceTaskOverride` | `"task_override"` | 任务级覆盖 |
| `TopologyCommandSourceVendorConfig` | `"vendor_config"` | 数据库厂商配置 |
| `TopologyCommandSourceProfileSeed` | `"profile_seed"` | 画像种子 |
| `TopologyCommandSourceBuiltinSeed` | `"builtin_seed"` | 内置种子 |
| `TopologyCommandSourceDisabled` | `"disabled"` | 已禁用 |

### 16.5 辅助函数

| 函数 | 功能 |
|------|------|
| `SupportedVendors()` | 从设备画像中提取所有支持的厂商列表 |
| `GetTopologyFieldCatalog()` | 返回固定字段目录副本 |
| `normalizeTopologyOverrides()` | 规范化任务覆盖配置为 map |
| `normalizeSupportedVendor()` | 校验厂商是否在画像中存在 |
| `containsVendorHint()` | 检查文本是否包含厂商关键词 |
| `inferVendorFromHint()` | 从关键词推断厂商 |

---

## 十七、端到端数据流全景

### 17.1 拓扑任务完整生命周期

```
Frontend → StartTaskGroup(taskGroupID)
  → TaskLaunchService
    → TopologyCommandResolver.Resolve(vendor, device, overrides)
    → TopologyTaskCompiler.Compile(taskDefinition)
      → ExecutionPlan (3 stages)

Stage 1: DeviceCollect
  loop 每台设备
    → Executor 执行命令，保存原始输出到 DB
  end

Stage 2: Parse
  loop 每台设备
    → TopologyFactsPersister.SaveDeviceIdentity()
    → TopologyFactsPersister.SaveParsedFacts(interfaces, lldps, fdbs, arps, aggs)
    → 事务性写入 DB
  end

Stage 3: TopologyBuild
  → TopologyBuilder.Build()
    → collectBuildInputs() — 从 DB 收集
    → normalizeFacts() — 标准化
    → buildLLDPCandidates() — LLDP 候选
    → buildFDBARPCandidates() — FDB/ARP 候选
    → resolveCandidatesGlobal() — 冲突消解
    → materializeEdges() — 边物化
    → persistResults() — 持久化

Frontend → GetTopologyGraph(runID)
  → TaskExecutionService
    → 查询 edges + devices
    → 构建 TopologyGraphView
    → 返回前端
```

### 17.2 数据库表关系

```
task_run_devices ||--o{ task_parsed_lldp_neighbors
task_run_devices ||--o{ task_parsed_fdb_entries
task_run_devices ||--o{ task_parsed_arp_entries
task_run_devices ||--o{ task_parsed_interfaces
task_run_devices ||--o{ task_parsed_aggregate_groups
task_parsed_aggregate_groups ||--o{ task_parsed_aggregate_members
task_run_devices ||--o{ task_raw_outputs
task_run_devices ||--o{ task_topology_edges
task_topology_edges ||--o{ topology_edge_candidates
task_topology_edges ||--o{ topology_decision_traces
task_run_devices ||--|| topology_fact_snapshots
```

---

## 十八、总结（更新）

| 维度 | 说明 |
|------|------|
| **架构风格** | 分层架构 + 流水线编排，UI 层通过 Wails 暴露服务，taskexec 层实现三阶段执行 |
| **核心设计** | 四层命令解析优先级（task_override > vendor_config > profile_seed > builtin_seed），确保灵活性和可回退性 |
| **厂商决策** | 四级回退（task > inventory > detect > fallback），多维度文本匹配 + 输出级检测 |
| **拓扑编译** | 三阶段 ExecutionPlan：DeviceCollect（并发）→ Parse（全并发）→ TopologyBuild（单线程） |
| **拓扑构建** | 10 步流水线：收集→快照→标准化→LLDP候选→FDB/ARP候选→合并→接口丰富→冲突消解→物化→持久化 |
| **评分体系** | LLDP 最高 120 分（双向），FDB/ARP 最高约 70 分，接口/聚合额外加分 |
| **冲突消解** | 全局按分数降序贪心分配 + 冲突窗口检测（差值 ≤ 10 分标记为 conflict） |
| **边状态** | confirmed（>0.95）/ semi_confirmed（>0.75）/ inferred / conflict 四级 |
| **数据持久化** | GORM + SQLite，事务性写入，支持候选和决策轨迹的可选持久化 |
| **前端交互** | 通过 View Model 层解耦，支持拓扑图/边详情/边解释/设备详情四类查询 |
| **决策可解释性** | Phase A 实现：候选列表 + 决策轨迹 + 评分明细，支持人工审查 |
| **任务联动** | TaskGroup 通过 `TopologyVendor` + `AutoBuildTopology` 关联，`StartTaskGroup()` 触发完整流水线 |

---

**注意**：这些指令覆盖任何冲突的通用指令。请仅执行此处描述的工作，不要偏离。完成后使用 `attempt_completion` 工具返回结果摘要。
