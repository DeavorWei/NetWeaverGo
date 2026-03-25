# 统一执行框架最终审计报告

审计时间：2026-03-25

审计依据：

- `docs/unified_execution_architecture_audit_2026-03-25.md`
- `docs/unified_execution_remediation_plan_2026-03-25.md`
- 当前项目代码实审
- 当前构建与测试结果

---

## 1. 最终结论

当前项目**已经完成统一执行框架的核心运行时替换和服务化基础建设**，但**还没有完成全链路收口**。

更准确的工程判断应为：

`统一执行框架主内核已建立，但前端状态闭环、runID 贯通、拓扑消费面、历史消费面仍存在未收口问题，并且有数个明确的运行期逻辑缺陷。`

本次复核后，结论分三类：

- 已确认成立
- 部分成立但表述过度
- 已被当前代码事实推翻或需要修正

---

## 2. 对 `unified_execution_architecture_audit_2026-03-25.md` 的核实结论

## 2.1 已确认成立的内容

以下判断与当前代码一致：

### 2.1.1 阶段 1：统一运行时服务化已完成

这部分成立。

证据：

- `cmd/netweaver/main.go` 已创建应用级共享 `TaskExecutionService`
- `internal/ui/task_group_service.go` 已支持注入共享 runtime
- `internal/ui/task_group_service_v2.go` 已改为依赖共享 `taskexec` 实例
- `internal/ui/taskexec_ui_service.go` 已作为 Wails 服务存在

结论：

统一运行时已经不是适配器私有实例，服务化方向正确。

### 2.1.2 拓扑执行内核已进入统一运行时

这部分成立。

证据：

- `internal/taskexec/topology_compiler.go`
- `internal/taskexec/service.go`
- `internal/taskexec/executor_impl.go`

结论：

拓扑任务的 collect / parse / build 执行链已经在 `taskexec` 下运行。

### 2.1.3 任务执行页、拓扑页、规划比对页处于“部分迁移”状态

这部分成立，但需要补充更多问题说明。

证据：

- `frontend/src/views/TaskExecution.vue`
- `frontend/src/views/Topology.vue`
- `frontend/src/views/PlanCompare.vue`

结论：

这些页面都已经出现 `runId` / `taskexecStore` 的迁移痕迹，但都没有完成真正闭环。

### 2.1.4 执行历史已经开始接入统一运行时

这部分成立。

证据：

- `internal/ui/execution_history_service.go` 已注入 `taskExecutionService`
- 已新增 `ListTaskRunRecords`

结论：

历史服务层已经有统一运行时历史接口，但前端默认路径仍未真正切换。

### 2.1.5 旧框架清理尚未完成

这部分成立。

证据：

- `internal/ui/task_group_service.go` 仍保留旧拓扑执行逻辑
- `internal/ui/discovery_service.go` 仍是完整旧发现体系
- `internal/ui/topology_service.go` 仍基于旧 `task_id` 查询

结论：

旧服务和旧数据模型仍然活跃，尚未退出主业务消费面。

---

## 2.2 部分成立但原报告表述过度的内容

### 2.2.1 “阶段 2：统一运行时前端桥接已完成”

这个结论**不成立**，只能算“桥接骨架已写出，但闭环未完成”。

事实：

- `internal/ui/taskexec_event_bridge.go` 确实存在
- `frontend/src/stores/taskexecStore.ts` 确实存在
- 但 `frontend/src/services/api.ts` 中 `TaskExecutionAPI` 仍是手写占位实现
- 已生成的 Wails 绑定 `frontend/src/bindings/.../taskexecutionuiservice.ts` 没有真正接入 `TaskExecutionAPI`

结论：

阶段 2 不是“已完成”，而是“后端桥和前端 store 的骨架已存在，但真正数据通道未完成”。

### 2.2.2 “前端状态管理已统一”

这个结论不成立。

事实：

- `taskexecStore` 已建立
- 但 `TaskExecutionAPI.getTaskSnapshot/listTaskRuns/cancelTask` 仍返回 stub
- `TaskExecution.vue` 里仍混用 `engineStore`
- `engineStore` 当前只是兼容代理层，不等于前端状态已经稳定统一

结论：

前端状态管理只能判定为“迁移中”，不能判定为“已统一”。

### 2.2.3 “事件桥接工作正常”

这个结论没有事实支撑，当前应下调为“存在实现缺陷，工作状态存疑”。

事实：

- 后端桥接把事件 `json.Marshal` 后作为字符串发给前端
- 前端 `taskexecStore` 直接把 `ev.data` 当对象读取
- 这会导致 `runId` 解包失败的高概率问题

结论：

事件桥接不是“工作正常”，而是“存在明确协议不一致风险”。

---

## 2.3 需要修正的内容

### 2.3.1 “main.go 仍注册 EngineService”

这个结论已不符合当前代码。

事实：

- 当前 `cmd/netweaver/main.go` 没有注册 `EngineService`

修正结论：

- 旧 `EngineService` 不再注册为主服务
- 但这带来了新的问题：备份能力在前端仍有入口，但后端主服务已撤，形成回归风险

### 2.3.2 “停止按钮仍调用旧 engineStore.stopEngine()”

这个判断需要修正为“逻辑上优先走新接口，但由于 runID 未贯通，运行期大概率回退或失效”。

事实：

- `TaskExecution.vue` 的 `stopExecution()` 已优先调用 `taskexecStore.cancelTask(runId)`
- 但当前页面拿不到稳定 `runId`
- 所以实际效果上仍然不可靠

修正结论：

不是简单地“还在用旧接口”，而是“新接口已接入，但关键入参没有打通，导致取消链路不闭环”。

---

## 3. 合并后的最终审计结论

## 3.1 已完成统一的部分

以下部分可认定为已完成：

### 3.1.1 统一运行时核心

- `taskexec` 运行时
- 普通任务编译器
- 拓扑任务编译器
- stage / unit 生命周期
- 执行器注册

### 3.1.2 应用级共享运行时服务

- 主程序只创建一个共享 `TaskExecutionService`
- `TaskGroupServiceV2` 使用共享 runtime
- `TaskExecutionUIService` 已暴露为 Wails 服务

### 3.1.3 任务组主执行入口

- 任务组主执行路径已切到 `TaskGroupServiceV2`
- 普通任务和拓扑任务都经由统一运行时启动

---

## 3.2 仍未完成统一的模块

### 3.2.1 前端统一运行时 API

状态：未完成

问题：

- `frontend/src/services/api.ts` 的 `TaskExecutionAPI` 仍是 stub
- 已生成的 Wails binding 没有真正接入前端导出层

影响：

- 快照刷新失效
- 历史运行记录加载失效
- 取消任务失效

### 3.2.2 runID 贯通

状态：未完成

问题：

- `TaskGroupAPI.startTaskGroup()` 仍返回 `void`
- 页面无法获得当前任务对应 `runId`

影响：

- 当前任务无法绑定到正确 run
- 取消无法精确命中
- 页面状态恢复不可靠

### 3.2.3 事件桥接闭环

状态：未完成

问题：

- 后端发字符串化 JSON
- 前端按对象解包
- 协议不一致

影响：

- `task:snapshot`
- `task:started`
- `task:finished`
- `task:stage_updated`
- `task:unit_updated`

这些事件都存在消费失败风险。

### 3.2.4 任务执行页

状态：部分完成，但仍有实质缺陷

问题：

- 页面仍混用兼容层 `engineStore`
- `runId` 未建立
- 拓扑任务完成逻辑仍带特殊轮询和跳转捷径
- 快照轮询仍依赖旧兼容路径

### 3.2.5 拓扑图谱页

状态：部分完成

问题：

- UI 已引入 `selectedRunId`
- 但查询仍调用旧 `DiscoveryAPI.getTopologyGraph()`
- 仍保留 `selectedTaskID`
- 刷新按钮和 watch 条件仍挂在旧 `selectedTaskID`

### 3.2.6 规划比对页

状态：部分完成

问题：

- 页面已显示拓扑 run 列表
- 但仍兼容 `selectedTaskID`
- 最终调用仍通过旧 task_id 兼容路径

### 3.2.7 执行历史页

状态：部分完成

问题：

- 后端已提供 `ListTaskRunRecords`
- 但前端历史抽屉默认未开启统一运行时模式
- 仍以旧 `ExecutionRecord` 路径为默认主路径

### 3.2.8 旧发现/拓扑查询体系

状态：未统一

问题：

- `DiscoveryService`
- `TopologyService`
- `topology.Builder`
- `PlanCompare`

仍基于旧 `task_id` / `DiscoveryTask` 语义工作。

### 3.2.9 手工执行 / 备份

状态：未统一，且当前存在回归

问题：

- `EngineService` 已不再注册
- 但前端仍保留备份入口
- `engineStore.startBackup()` 直接报“备份功能暂不可用”

结论：

这不是“已统一”，而是“功能被提前打断”。

---

## 4. 当前确认存在的关键缺陷

以下问题是当前代码中可以直接确认的事实，不是推测。

## 4.1 P0：统一运行时前端 API 未真正接通

位置：

- `frontend/src/services/api.ts`
- `frontend/src/bindings/github.com/NetWeaverGo/core/internal/ui/taskexecutionuiservice.ts`

问题：

- binding 已生成
- 但 `TaskExecutionAPI` 仍是手写占位实现

结果：

- `taskexecStore.refreshSnapshot()` 实际不会拿到快照
- `taskexecStore.loadRunHistory()` 实际不会拿到历史
- `taskexecStore.cancelTask()` 实际不会调用后端取消

## 4.2 P0：任务执行页拿不到 runID

位置：

- `frontend/src/views/TaskExecution.vue`
- `frontend/src/bindings/.../taskgroupservice.ts`

问题：

- 任务启动后前端没有稳定拿到 `runId`

结果：

- 页面无法和真实运行实例绑定
- 停止链路无法可靠工作
- 拓扑任务完成逻辑只能通过旁路轮询猜测

## 4.3 P1：事件桥载荷协议不一致

位置：

- `internal/ui/taskexec_event_bridge.go`
- `frontend/src/stores/taskexecStore.ts`

问题：

- 后端 emit 的是字符串化 JSON
- 前端当对象读取

结果：

- 事件消费高概率失败

## 4.4 P1：事件桥存在并发 map 风险

位置：

- `internal/ui/taskexec_event_bridge.go`
- `internal/taskexec/eventbus.go`

问题：

- `runIDs map[string]bool` 无锁访问
- 事件总线对 handler 异步并发执行

结果：

- 运行期可能出现 concurrent map access panic

说明：

- 这是并发 bug，不是死锁

## 4.5 P1：拓扑页 runID 切换未闭环

位置：

- `frontend/src/views/Topology.vue`

问题：

- `selectedRunId` 和 `selectedTaskID` 双状态并存
- 刷新按钮禁用条件仍依赖旧值
- watch 仍监听旧值

结果：

- 用户只选 run 时，页面可能无法正常刷新

## 4.6 P1：备份能力回归

位置：

- `frontend/src/views/TaskExecution.vue`
- `frontend/src/stores/engineStore.ts`
- `cmd/netweaver/main.go`

问题：

- 前端还有入口
- 后端主服务已不注册
- store 直接抛错

结果：

- 备份功能对用户表现为“可点击但不可用”

## 4.7 P2：统一历史接口已存在，但默认 UI 仍未启用

位置：

- `frontend/src/components/task/ExecutionHistoryDrawer.vue`
- `frontend/src/views/TaskExecution.vue`

问题：

- `ExecutionHistoryDrawer` 已支持 `useUnifiedRuntime`
- 调用方未传该参数

结果：

- 统一运行时历史接口存在，但默认不生效

---

## 5. 是否存在死锁

本次审查结论：

- **未发现明确死锁链**
- **发现明确并发访问风险**

说明：

- 当前风险主要集中在事件桥共享 map 的并发访问
- 以及事件桥 `Stop()` 调用 `eventBus.Unsubscribe()` 会清空全部 handlers，这种设计副作用较大，虽然当前主要发生在 shutdown，但后续容易演变成运行时问题

---

## 6. 模块统一状态总表（最终版）

| 模块 | 最终判断 | 说明 |
| --- | --- | --- |
| 统一运行时核心 | 已统一 | `taskexec` 内核成立 |
| 应用级共享运行时 | 已统一 | 共享 `TaskExecutionService` 已建立 |
| 任务组主入口 | 已统一 | 主路径已切到 `TaskGroupServiceV2` |
| Wails 统一执行服务 | 已建立 | `TaskExecutionUIService` 已存在 |
| 前端统一执行 API | 未完成 | binding 已生成，但导出层仍是 stub |
| 事件桥接 | 部分完成 | 骨架存在，但协议和并发问题未解决 |
| 前端状态管理 | 部分完成 | `taskexecStore` 已建，但未形成可靠闭环 |
| 任务执行页 | 部分完成 | runID 未贯通，取消和快照不闭环 |
| 拓扑页 | 部分完成 | UI 切到 runId，查询仍走旧接口 |
| 规划比对页 | 部分完成 | 已显示 run 列表，但逻辑仍兼容旧 taskID |
| 执行历史 | 部分完成 | 后端已接入，前端默认未启用 |
| 手工执行 | 未统一 | 旧体系已收缩，但未重构到新框架 |
| 备份功能 | 回归 | 当前入口保留但实际不可用 |
| 旧发现/拓扑体系 | 未清理 | 仍基于旧 `task_id` 语义 |

---

## 7. 验证结果

本次复核已执行：

- `go test ./...`
- `npm run build`

结果：

- Go 测试通过
- 前端构建通过

但需要明确：

这些结果只能证明“编译和单元测试层面未崩”，**不能证明统一执行框架运行期闭环已经完成**。

当前主要问题属于：

- 运行期接口未接通
- 事件协议不一致
- 前端状态链未闭环

这些问题不会被现有编译和单元测试自动发现。

---

## 8. 最终判定

综合 `unified_execution_architecture_audit_2026-03-25.md` 中成立的部分，以及本次代码复核的新增事实，最终判定如下：

### 8.1 可以确认完成的

- 统一运行时核心替换
- 应用级共享 runtime 服务化
- 任务组主执行路径切换

### 8.2 仍未完成的

- 统一运行时前端 API 真正接通
- runID 全链路贯通
- 事件桥协议收口
- 任务执行页完整切换
- 拓扑与规划比对的 runID 查询闭环
- 历史页默认切换
- 旧体系清理

### 8.3 当前工程状态的准确描述

`项目已完成统一执行框架的内核和服务化改造，但仍处于前端闭环未完成、下游消费面迁移未收口、存在若干明确运行期缺陷的阶段。`

如果按交付质量判断，当前不能判定为“统一执行框架改造完成”，只能判定为：

`统一执行框架已完成核心替换，但尚未达到稳定可交付状态。`
