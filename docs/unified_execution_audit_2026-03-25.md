# 统一执行框架模块审计报告

审计时间：2026-03-25

## 1. 结论摘要

当前项目已经完成了“任务组主执行入口切到 `taskexec`”这一步，但项目整体还没有完成统一执行框架收口。

本轮审计确认：

- `TaskGroupService.StartTaskGroup()` 已委托到 `TaskGroupServiceV2`，任务组普通任务和拓扑任务的主启动路径已进入 `taskexec`。
- 但系统内仍并行存在 3 套执行体系：
  - `taskexec` 新统一运行时
  - `executionManager + engine.Engine` 旧执行体系
  - `discovery.Runner + TopologyService` 旧发现/拓扑体系
- 前端主执行态、停止控制、历史记录、拓扑查看、规划比对，仍大量依赖旧体系。
- 拓扑数据链虽然已由 `taskexec` 驱动执行，但事实落库和图谱查询仍复用旧 `DiscoveryTask` / `Topology*` 数据模型，属于“执行框架切换了，领域模型没收口”。

结论：目前是“执行主入口部分统一，运行态展示、控制面、历史面、拓扑面仍混用旧框架”的状态，不应判定为统一执行框架已完成。

## 2. 审计范围

本次审计聚焦“与执行框架直接相关的功能模块”：

- 任务组执行
- 手工执行 / 批量执行 / 备份
- 发现任务 / 拓扑采集 / 拓扑图谱
- 执行历史
- 规划比对
- 前端执行态与事件通道
- Wails 服务注册与后端运行时暴露方式

以下模块不纳入“统一执行框架是否完成”的核心判断：

- 设备管理
- 命令组管理
- 设置管理
- 配置生成
- 子网计算
- 端口速查

这些模块本身不是执行运行时，不存在是否统一到执行框架的问题。

## 3. 当前架构判断

### 3.1 已完成统一的部分

以下部分已经进入 `taskexec`：

- 任务组启动主入口
  - `cmd/netweaver/main.go`
  - `internal/ui/task_group_service.go`
  - `internal/ui/task_group_service_v2.go`
- 统一运行时核心
  - `internal/taskexec/service.go`
  - `internal/taskexec/runtime.go`
  - `internal/taskexec/normal_compiler.go`
  - `internal/taskexec/topology_compiler.go`
  - `internal/taskexec/executor_impl.go`

已核实的事实：

- `TaskGroupService.StartTaskGroup()` 直接转发到 `TaskGroupServiceV2.StartTaskGroup()`。
- `TaskGroupServiceV2` 会根据任务类型调用 `TaskExecutionAdapter.StartNormalTask()` 或 `StartTopologyTask()`。
- `taskexec` 已完成运行时 stage/unit 生命周期落库、快照构建、普通任务执行、拓扑采集执行。

### 3.2 仍未统一的核心事实

- Wails 前端主状态并没有消费 `taskexec` 快照，而是继续消费旧 `execution:snapshot`。
- 取消/停止链路仍指向旧 `EngineService.StopEngine()`。
- 拓扑页和规划比对页仍以旧 `DiscoveryTaskView` / `taskID` 为主键。
- 历史记录仍基于旧 `ExecutionRecord` 聚合模型，不读取 `taskexec` 的 run/stage/unit。
- 旧 `DiscoveryService`、`TopologyService`、`EngineService` 仍是正式注册并可用的业务服务，不是纯兼容壳。

## 4. 未统一执行框架的功能模块

## 4.1 手工执行 / 批量执行 / 备份模块

状态：未统一

证据：

- `internal/ui/engine_service.go`
- `internal/ui/execution_manager.go`
- `frontend/src/stores/engineStore.ts`
- `frontend/src/App.vue`

问题：

- `EngineService` 仍使用 `engine.Engine` + `executionManager` 作为独立执行体系。
- 事件仍通过 `execution:snapshot`、`engine:finished`、`engine:suspend_required` 推给前端。
- 备份功能也仍然挂在 `EngineService.StartBackup()`，没有进入 `taskexec`。
- `App.vue` 启动时只初始化 `engineStore` 监听器，没有初始化统一运行时桥接。

结论：

手工执行、批量执行、备份仍完全属于旧执行框架，不属于统一运行时。

## 4.2 发现任务 / 拓扑服务模块

状态：未统一

证据：

- `internal/ui/discovery_service.go`
- `internal/discovery/runner.go`
- `internal/ui/topology_service.go`
- `frontend/src/services/api.ts`

问题：

- `DiscoveryService.StartDiscovery()` 仍直接启动 `discovery.Runner`。
- `DiscoveryService.forwardEvents()` 仍向前端发 `discovery:event`。
- `TopologyService` 仍以 `taskID` 为输入执行 parse/build/query。
- `DiscoveryAPI` 与 `TopologyServiceBinding` 仍保留完整旧接口面。

结论：

发现任务与拓扑服务仍是一套独立旧框架，尚未收敛进统一运行时服务层。

## 4.3 任务执行前端页面

状态：部分统一，整体仍未统一

证据：

- `frontend/src/views/TaskExecution.vue`
- `frontend/src/stores/engineStore.ts`
- `frontend/src/composables/useTaskExecution.ts`

问题：

- `TaskExecution.vue` 的运行态主要仍来自 `engineStore.executionSnapshot`。
- `engineStore` 只监听旧事件 `execution:snapshot` 和 `engine:finished`。
- `useTaskExecution.ts` 名义上是“统一任务执行运行时”，但实际上仍监听旧 `execution:snapshot`，`task:event` 只做日志输出，没有形成真正状态驱动。
- 拓扑任务在 `TaskExecution.vue` 中仍保留特殊分支，启动后立即提示“拓扑采集任务已完成”并跳转 `/topology`，并非基于统一运行时真实完成态。

结论：

页面外观开始兼容新运行时字段，但状态源、事件源、控制链路仍是旧框架，属于“UI 层半接入”。

## 4.4 拓扑图谱模块

状态：未统一

证据：

- `frontend/src/views/Topology.vue`
- `internal/ui/discovery_service.go`

问题：

- 拓扑页面下拉框仍展示 `DiscoveryTaskView`，文案仍是“选择发现任务”。
- 页面通过 `DiscoveryAPI.listDiscoveryTasks()` 拉取任务列表。
- 图谱、边详情、设备详情仍通过旧 `taskID` 查询。

结论：

拓扑结果页仍然围绕旧发现任务模型工作，没有切换到统一运行时 `runID`。

## 4.5 规划比对模块

状态：未统一

证据：

- `frontend/src/views/PlanCompare.vue`

问题：

- `PlanCompare.vue` 通过 `DiscoveryAPI.listDiscoveryTasks(50)` 获取可比对任务。
- 对比输入仍要求旧发现任务 `taskID`，不是统一运行时 run。

结论：

规划比对是明确依赖旧发现任务模型的下游模块，尚未切换。

## 4.6 执行历史模块

状态：未统一

证据：

- `internal/ui/execution_history_service.go`
- `internal/ui/discovery_service.go`
- `internal/ui/engine_service.go`
- `internal/taskexec/service.go`

问题：

- 历史记录服务读取的是 `models.ExecutionRecord`。
- `RunnerSource` 仍按 `task_group`、`discovery_service`、`engine_service`、`backup_service` 分裂。
- `taskexec` 虽然已有 `ListRuns()`、`GetRunStatus()`，但没有接入历史记录服务。
- 发现任务完成后仍由 `DiscoveryService.persistDiscoveryExecutionSummary()` 写旧历史摘要。

结论：

当前历史面板展示的是旧聚合视图，不是统一运行时历史。

## 4.7 Wails 服务暴露层

状态：未统一

证据：

- `cmd/netweaver/main.go`
- `internal/ui/task_execution_adapter.go`

问题：

- Wails 注册的仍是 `TaskGroupService`、`EngineService`、`DiscoveryService`、`TopologyService` 等旧服务面。
- `taskexec.TaskExecutionService` 没有作为共享应用服务直接暴露给 Wails。
- `TaskExecutionAdapter` 在构造时会自行 `NewTaskExecutionService(config.DB)` 并 `Start()`，说明统一运行时目前不是全局单例，而是适配层私有实例。

结论：

统一运行时还没有成为应用级唯一执行服务，服务层设计仍然是旧服务包裹新服务。

## 5. 当前最混乱的问题

## 5.1 三套执行框架并行存在

这是当前最大的结构性问题。

并行体系如下：

- `taskexec`：任务组执行新主路径
- `executionManager + engine.Engine`：手工执行、批量执行、备份、前端主快照
- `discovery.Runner + TopologyService`：发现任务、拓扑查询、图谱页、规划比对

风险：

- 状态语义不一致
- 事件名不一致
- 停止/取消接口不一致
- 历史记录来源不一致
- 下游页面不知道应该消费哪一套 ID

## 5.2 统一运行时没有前端桥接

`taskexec` 内部有 `EventBus`、`SnapshotHub`、`GetSnapshot()`，但当前没有看到后端把这些状态通过 Wails 事件桥正式发到前端。

直接结果：

- 前端只能继续依赖旧 `execution:snapshot`
- `useTaskExecution.ts` 监听 `task:event` 但后端没有对应桥接实现
- 新运行时状态无法成为前端主真相源

## 5.3 停止控制走错通道

证据：

- `TaskGroupServiceV2` 已提供 `CancelTask(runID string)`
- `TaskExecution.vue` 的 `stopExecution()` 仍调用 `engineStore.stopEngine()`
- `engineStore.stopEngine()` 实际调用 `EngineAPI.stopEngine()`

问题：

- 统一运行时任务的取消接口已经存在，但前端主页面没有接这条链。
- 当前“停止任务”按钮逻辑上仍然偏向旧引擎。

## 5.4 运行态 ID 模型仍然分裂

当前至少存在以下几类 ID：

- `taskGroupID`
- `runID`
- `DiscoveryTask.taskID`
- `ExecutionRecord.RunnerID`

问题：

- 任务组启动返回的是 `runID`
- 拓扑页、规划比对页、发现服务查询用的是 `taskID`
- 历史记录页用的是 `RunnerID + RunnerSource`

这导致跨模块关联关系不清晰，前端和服务层都要做额外映射。

## 5.5 拓扑执行已进入新框架，但数据模型仍复用旧领域表

从执行链看，拓扑 collect/parse/build 已经在 `taskexec` 下运行；但从落库和查询看，仍复用了旧模型：

- `DiscoveryDevice`
- `RawCommandOutput`
- `TopologyEdge` / `Topology*`

而且当前是用 `runID` 充当这些旧表中的 `task_id`。

问题：

- 短期可用
- 长期语义混乱
- 领域边界不清晰
- 新旧框架共表后，迁移和回收会很困难

## 5.6 旧代码已经不是主路径，但仍作为正式实现保留

`internal/ui/task_group_service.go` 中旧的：

- `executeModeA`
- `executeModeB`
- `executeTopologyTask`

仍完整保留。

问题：

- 容易让后续维护者误判当前主路径
- 旧逻辑和新逻辑并存，增加回归风险
- 容易继续被其他模块错误复用

## 5.7 前端类型模型混用

当前前端同时存在：

- 旧 `report.ExecutionSnapshot`
- 新 `types/taskexec.ExecutionSnapshot`

问题：

- `engineStore` 仍以旧 `ExecutionSnapshot` 为主
- `useTaskExecution.ts` 使用的是新类型定义
- 同一页面中出现“字段兼容拼接”的写法，说明前端真相源尚未统一

## 6. 模块统一状态总表

| 模块 | 当前状态 | 判断 |
| --- | --- | --- |
| 任务组主执行入口 | 已切到 `taskexec` | 已统一主路径 |
| `taskexec` 运行时核心 | 已建立 | 已统一 |
| 手工执行 / 批量执行 | 仍走 `EngineService` | 未统一 |
| 配置备份 | 仍走 `EngineService.StartBackup()` | 未统一 |
| 发现任务服务 | 仍走 `DiscoveryService` | 未统一 |
| 拓扑解析/构图查询服务 | 仍走 `TopologyService` | 未统一 |
| 任务执行页运行态 | 仍依赖 `engineStore` | 部分统一 |
| 停止/取消控制 | 仍走 `EngineAPI.stopEngine()` | 未统一 |
| 拓扑图谱页 | 仍依赖 `DiscoveryTaskView` | 未统一 |
| 规划比对页 | 仍依赖 `DiscoveryTaskView` | 未统一 |
| 执行历史页 | 仍依赖 `ExecutionRecord` | 未统一 |
| Wails 服务层 | 旧服务面保留且仍活跃 | 未统一 |

## 7. 建议整改顺序

### 7.1 第一优先级

把 `taskexec` 提升为应用级唯一运行时服务：

- 在 Wails 层注册统一执行服务
- 只保留一个共享 `TaskExecutionService`
- 增加统一的 Wails 事件桥和快照查询接口

如果这一步不做，前端和历史层都无法真正收口。

### 7.2 第二优先级

把前端执行态改成只消费统一运行时：

- 用 `runID` 作为唯一运行实例主键
- 重写 `engineStore` 或替换为 `taskexecStore`
- `TaskExecution.vue` 的停止按钮切到 `CancelTask(runID)`
- 删除拓扑任务特殊完成提示和跳转捷径

### 7.3 第三优先级

统一拓扑下游模块：

- 拓扑图谱页改为消费统一运行时 run 列表或 run 映射
- 规划比对改为选择统一运行时拓扑 run
- 历史记录接入 `taskexec.ListRuns()` 和 run 摘要

### 7.4 第四优先级

清退旧框架残留：

- 删除 `TaskGroupService` 中旧执行实现
- 评估 `DiscoveryService` / `TopologyService` 是否降级为查询壳，或直接并入统一执行服务
- 逐步退出 `executionManager` 在任务执行页中的核心地位

## 8. 最终结论

截至本次审计，项目不是“统一执行框架已完成”，而是：

- 任务组执行主入口已切到新框架
- 但控制面、展示面、历史面、拓扑面、下游消费面仍未统一
- 系统仍存在明显的新旧框架并存和职责交叉问题

如果按工程状态判断，当前更准确的结论应为：

`统一执行框架已完成核心运行时替换，但尚未完成全链路收口。`
