# 拓扑发现命令超时问题修复实施方案（方案三）

> 文档版本：v1.0  
> 创建日期：2026-03-24  
> 适用阶段：开发期（允许结构性重构）

## 当前进度（2026-03-24）

1. 已完成：初始化命令并入 `ExecutePlan -> StreamEngine.RunPlaybook` 统一执行链。
2. 已完成：移除 `StreamEngine.RunInit` / `SetProfile` 旧旁路入口。
3. 已完成：`Initializer` 降级为兼容壳，直连 SSH I/O 方法默认返回废弃错误。
4. 已完成：新增回归测试（统一路径命令发送、初始化结果剥离、ExecutePlan 入口行为）。
5. 已完成：补充多厂商回放用例（huawei/h3c/cisco）。

## 1. 背景与目标

当前发现任务存在初始化阶段与运行阶段执行路径不一致的问题：

- 初始化阶段由 `Initializer` 直接调用 `client.SendCommand + waitForPrompt`
- 运行阶段由 `StreamEngine + SessionAdapter + SessionReducer` 驱动

该“双路径”导致状态机上下文与真实 SSH 会话行为可能失配，出现“首条禁分页成功，后续批量命令超时”的故障。

**方案三目标**：统一初始化与业务命令为同一状态机驱动路径，消除双路径副作用，形成单一可验证执行内核。

## 2. 方案三总体设计

### 2.1 核心原则

1. 只有一个命令发送通道：`StreamEngine.executeSessionAction(ActSend*)`
2. 只有一个状态推进来源：`SessionAdapter.FeedSessionActions -> SessionReducer.Reduce`
3. `Initializer` 不再直接执行 SSH I/O，仅保留画像配置解析能力（如禁分页命令列表）

### 2.2 目标态执行流

`ExecutePlan -> StreamEngine.RunUnified -> [InitPhase(state machine)] -> [PlaybookPhase(state machine)] -> Results`

即：
- 初始化（等待初始 prompt、预热、禁分页）由 Reducer 发出动作并执行
- 初始化完成后直接衔接业务命令，不做上下文“清理式补丁”

## 3. 重构范围

### 3.1 主要改造文件

- `internal/executor/stream_engine.go`
- `internal/executor/session_reducer.go`
- `internal/executor/session_types.go`
- `internal/executor/session_adapter.go`
- `internal/executor/initializer.go`（降级为配置提供者/兼容壳）
- `internal/executor/executor.go`（入口调用统一化）

### 3.2 非目标范围

- 不改 discovery 任务编排（`internal/discovery/runner.go`）主流程
- 不改数据库模型与前端展示协议
- 不改供应商命令集配置语义

## 4. 分阶段实施计划

## 阶段A：状态机扩展（先建能力，不切流）

### A.1 新增初始化动作与事件

在 `session_types.go` 增加初始化阶段动作/事件：
- 事件：`EvInitPromptStable`、`EvWarmupPromptSeen`、`EvInitPagerDisabled`
- 动作：`ActSendWarmup`、`ActSendInitCommand`（或复用 `ActSendCommand` 并加 phase 字段）

### A.2 Reducer 明确 Init 子状态

在 `session_reducer.go` 引入明确的初始化子流程：
1. `InitAwaitPrompt`
2. `InitAwaitWarmupPrompt`
3. `InitDisablePagerPending`
4. `Ready`

要求：
- 初始化命令执行完成后才能进入 `Ready`
- `Ready` 后立即发首条业务命令
- 禁止在 `Ready` 前推进业务命令索引

### A.3 单测

新增/补强测试：
- 初始化流程完整闭环（prompt -> warmup -> pager off -> ready）
- 初始化结束后首条业务命令自动发出
- 初始化阶段出现告警行不阻塞推进
- 初始化失败时返回 fatal 错误并停止

## 阶段B：StreamEngine 统一执行（切换主路径）

### B.1 合并 RunInit 与 RunPlaybook 调度

改造 `stream_engine.go`：
- 将 `RunInit` 与 `Run` 的分离流程合并为统一事件循环
- 初始化动作也通过 `executeSessionAction` 执行
- 删除“初始化后清理残留再开跑”的依赖路径

### B.2 约束命令发送

所有命令（含禁分页）只能经由：
- `executeSessionAction -> client.SendCommand`

禁止：
- 在 Initializer 或其他旁路中直接发送命令

### B.3 兼容层

`initializer.go` 过渡为：
- 仅提供 profile/init 配置读取与校验
- 不直接 Read/Write SSH 流（可保留 deprecated 接口，内部 no-op 或转发）

### B.4 单测与集成测试

新增测试：
- `ExecutePlan` 场景下，包含 `DisablePagerCommands + 11条业务命令`，验证全部触发发送
- 验证 raw/detail 日志同时记录初始化命令与业务命令
- 验证超时时 `CurrentCommand` 精准对应当前命令

## 阶段C：清理旧路径与文档更新

### C.1 删除临时补丁逻辑

移除/废弃：
- `ClearInitResiduals` 对问题语义的关键依赖
- 任何“初始化后 sleep/flush 规避”代码

### C.2 文档与注释更新

- 更新 `docs/discovery_command_failure_analysis.md` 结论状态（标记已实施方案三）
- 在 executor 目录补充统一执行架构图与状态机说明

## 5. 验收标准（DoD）

满足以下全部条件才可合并：

1. 代码层面不存在初始化旁路发送命令（全局检索 `initializer.*SendCommand` 为 0 处有效调用）
2. `ExecutePlan` 在真实设备/模拟回放中，初始化后业务命令可连续推进
3. 不再出现“results=1, commands=11”此类结构性错位
4. `go test ./internal/executor ./internal/discovery` 全绿
5. 新增回归测试覆盖“禁分页成功但业务命令未发出”历史故障场景

## 6. 风险与应对

### 风险1：状态机改造引入新死锁/停滞

- 应对：增加状态迁移断言与超时监控日志（state + nextIndex + pendingLines）

### 风险2：多厂商 prompt 差异导致 init 阶段误判

- 应对：按 vendor 增加 golden case（huawei/h3c/cisco）并进行回放测试

### 风险3：日志行为变化影响排障

- 应对：保留现有日志字段并新增 phase 字段，不做破坏式字段删除

## 7. 回滚策略

在重构分支保留开关：
- `executor.unified_init_enabled`（默认 true）

若上线验证失败，可临时切回旧路径（仅用于紧急恢复，不作为长期方案），并保留日志对比数据用于二次修复。

## 8. 建议实施顺序与工期（开发期）

1. 第1天：阶段A（状态机能力 + 单测）
2. 第2天：阶段B（主路径切换 + 集成测试）
3. 第3天：阶段C（清理旧逻辑 + 文档收口 + 联调）

预计 2~3 个工作日可完成可合并版本。

## 9. 执行清单（可直接用于任务拆分）

1. 新增初始化阶段状态与动作定义  
2. 改造 Reducer 初始化闭环与首条命令推进  
3. 合并 StreamEngine 初始化与运行循环  
4. 移除 Initializer 直接 I/O 行为  
5. 补齐回归测试（历史故障场景必测）  
6. 更新故障分析文档与开发说明  
7. 联调验证并准备灰度开关
