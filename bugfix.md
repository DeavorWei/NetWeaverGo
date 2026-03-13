# NetWeaverGo 修复计划与实施方案

## 当前进度

- 已完成：P0-1 引擎状态机修复
- 已完成：P0-2 Tracker 单一数据源修复
- 已完成：P0-3 任务组执行接入统一快照/完成事件链
- 已完成：P0-4 设备数据写入语义修复
- 已完成：P1-1 QueryService 字段映射与排序白名单修复
- 已完成：P1-2 任务组最终状态汇总修复
- 已完成：P1-3 旧迁移逻辑去重与启动状态收敛
- 待继续：P2 其余项

## 1. 修复目标

本轮修复的目标不是零散打补丁，而是把以下四条主链重新拉直：

1. 引擎生命周期链
2. 任务执行快照链
3. 设备数据写入链
4. 前端状态消费链

验收标准：

- 手动启动执行与任务组执行都能稳定进入 `Starting -> Running -> Closing -> Closed`
- 前端执行页能持续收到同一份 Tracker 生成的快照
- 设备新增、编辑、删除、批量操作不再覆盖整表或误伤分页外设备
- 未使用声明只保留“马上要接入”的必要能力，其余删除

## 2. 分阶段修复计划

### P0：先修阻断主流程的高优问题

#### P0-1 引擎状态机修复

问题：

- `EngineStateManager` 只允许 `Idle -> Starting -> Running`
- `Run()` / `RunBackup()` 直接从 `Idle` 切到 `Running`
- 这会让执行入口在状态校验处直接失败

方案：

1. 在 `Run()` / `RunBackup()` 开始时显式执行 `TransitionTo(StateStarting)`
2. 完成上下文、Tracker、监听器初始化后，再转 `StateRunning`
3. 在统一关闭逻辑中始终保证 `StateClosing -> StateClosed`
4. 把状态迁移失败视为硬错误返回，而不是只打印日志

建议改动点：

- `internal/engine/engine.go`
- `internal/engine/engine_state.go`

验收：

- 启动执行不再立刻返回
- `GetGlobalState().GetStatus()` 能看到完整状态迁移

#### P0-2 Tracker 单一数据源修复

问题：

- `EngineService` / `TaskGroupService` 会先注入 Tracker
- `Engine.Run()` / `RunBackup()` 又重新创建一个新 Tracker
- 导致快照推送和真实执行事件不在同一个 Tracker 上

方案：

1. 把 Tracker 初始化收敛到一个地方
2. `Engine` 增加 `ensureTracker(totalDevices, taskName)`，仅在未注入时创建
3. `SetTracker()` 只负责绑定已有 Tracker，不允许在 `Run()` 中覆盖
4. 所有前端快照、最终快照、报告导出都使用同一个 `tracker`

建议改动点：

- `internal/engine/engine.go`
- `internal/ui/engine_service.go`
- `internal/ui/task_group_service.go`

验收：

- `GetExecutionSnapshot()` 与前端收到的 `execution:snapshot` 内容一致
- 任务页不会出现“执行中但快照空白”

#### P0-3 统一执行入口

问题：

- `EngineService` 和 `TaskGroupService` 分别维护各自的执行流程
- 任务组执行没有接入全局 active engine、快照 ticker、完成事件

方案：

1. 抽一个统一的执行协调层，例如 `runManagedEngine(...)`
2. 负责：
   - 设置全局 active engine
   - 注入 Tracker
   - 启动/停止快照推送
   - 转发 `device:event`
   - 发送 `engine:finished`
   - 清理 `cancelFunc`
3. `EngineService.StartEngine/StartBackup` 与 `TaskGroupService.StartTaskGroup` 全部走同一套编排

建议改动点：

- `internal/ui/engine_service.go`
- `internal/ui/task_group_service.go`
- 如有必要可新增 `internal/ui/execution_coordinator.go`

验收：

- 两种执行入口在前端表现一致
- 任务执行页刷新/重进后仍可同步当前状态

#### P0-4 设备数据写入语义修复

问题：

- 前端把 `SaveDevices` 当作“局部更新”使用
- 后端实际是“删除全量再重建”
- 批量编辑和 IP 范围新增会造成整表覆盖

方案：

1. 明确拆分 API 语义：
   - `ReplaceDevices([]DeviceAsset)` 仅用于导入/整体替换
   - `CreateDevices([]DeviceAsset)` 用于追加
   - `UpdateDeviceByID(id)` / `DeleteDeviceByID(id)` 用于单条变更
2. 前端设备页停止调用全量覆盖接口做局部编辑
3. 所有表格操作改为基于稳定主键，而不是页内索引
4. 分页、筛选、排序后的操作都只传真实设备 ID

建议改动点：

- `internal/ui/device_service.go`
- `internal/config/config.go`
- `frontend/src/views/Devices.vue`

验收：

- 任意分页下编辑/删除命中正确设备
- 批量编辑不会丢失未展示数据

### P1：修复数据正确性与结果表达

#### P1-1 QueryService 字段映射修复

问题：

- 数据库列为 `group_name`
- 查询服务仍使用原始字段 `group`
- `sortBy` 直接拼 SQL，缺少白名单

方案：

1. 所有原生 SQL 统一改成 `group_name`
2. 为排序字段建立白名单映射
3. 对前端传参做列名归一化，禁止裸拼接

建议改动点：

- `internal/ui/query_service.go`
- `internal/config/config.go`

验收：

- 按分组筛选、聚合、排序恢复正确
- 非法 `sortBy` 不会进入 SQL

#### P1-2 任务组最终状态汇总修复

问题：

- 任务组当前只等 goroutine 结束
- 没汇总设备成功/失败/中止结果
- 所以存在失败设备时仍被标记为 `completed`

方案：

1. 使用 Tracker 的最终统计结果计算任务状态
2. 汇总规则建议：
   - 全成功/跳过：`completed`
   - 存在失败或中止：`failed`
   - 被手动取消：`cancelled` 或保持 `failed`
3. 在任务结束时写回完成时间、失败摘要、设备统计

建议改动点：

- `internal/ui/task_group_service.go`
- `internal/report/collector.go`
- `internal/config/task_group.go`

验收：

- 前端任务卡片状态与设备结果一致

#### P1-3 旧迁移逻辑去重

问题：

- 旧数据迁移被初始化和 `main()` 双重调用
- Dashboard/CLI 还保留旧文件缺失逻辑，已经与 DB 架构不一致

方案：

1. `MigrateLegacyDataIfNeeded()` 只保留一个调用入口
2. 去掉已失效的 `missingFiles/settings.yaml` 分支
3. 启动日志改成基于数据库与迁移版本输出

建议改动点：

- `internal/config/db.go`
- `cmd/netweaver/main.go`
- `frontend/src/views/Dashboard.vue`

### P2：结构优化与一致性清理

#### P2-1 运行时配置真正接入执行链

问题：

- 运行时配置已经持久化，但引擎里不少缓冲和并发值仍硬编码

方案：

1. `NewEngine()` 使用 `RuntimeConfigManager`
2. 将 `EventBus`、`FrontendBus`、RingBuffer 容量改为配置驱动
3. 统一 `settings.MaxWorkers` 与 runtime config 的优先级

#### P2-2 前端执行态收口

问题：

- `engineStore` 同时维护快照状态和一份未消费的事件日志
- 手写事件类型与 Wails bindings 重复

方案：

1. 前端执行态只保留快照、挂起弹窗、停止动作
2. 优先使用 bindings 类型，减少手写重复定义
3. 去掉页面尾部重复的 `loadTasks()`

建议改动点：

- `frontend/src/stores/engineStore.ts`
- `frontend/src/views/TaskExecution.vue`
- `frontend/src/services/api.ts`

## 3. 未使用声明评估

### 已删除

这些声明已经脱离主链，删除能直接降低维护成本：

- `internal/ui/event_bridge.go`
  - 原因：仅剩 `SetWailsApp` 被调用，核心桥接逻辑从未接入真实执行链
- `frontend/src/composables/useEngineEvents.ts`
  - 原因：前端已经统一走 `engineStore` 订阅
- `frontend/src/types/events.ts`
  - 原因：只被已删除的 composable 使用，且与现有 store/bindings 重复
- `internal/config/runtime_config.go` 中的 `GetRuntimeConfigValue` / `SetRuntimeConfigValue`
  - 原因：没有任何调用方，且已被结构化配置读写接口替代
- `internal/config/command_group.go` 中的 `GetCommandGroupCommands`
  - 原因：只是 `GetCommandGroup()` 的薄包装，没有调用方
- `internal/engine/engine_state.go` 中的 `EngineStateContext`
  - 原因：状态管理已经直接通过 `EngineStateManager` 完成
- `internal/report/collector.go` 中的 `CollectEvent` / `GetDeviceSnapshot` / `AddDeviceLog`
  - 原因：没有引用，且分别被 `handleEvent`、`GetSnapshot`、`addDeviceLogLocked` 覆盖
- `frontend/src/stores/engineStore.ts` 中的 `eventLogs` / `closeSuspendModal`
  - 原因：没有任何 UI 消费方，保留只会制造额外状态和内存开销

### 保留并计划接入

这些现在暂未被引用，但保留是有价值的：

- `frontend/src/stores/engineStore.ts` 的 `stopEngine`
  - 结论：保留
  - 原因：这是明确的产品能力，应该在执行页接“停止任务”按钮，而不是删除
- `internal/report/collector.go` 的 `GetStats` / `IsFinished`
  - 结论：保留
  - 原因：任务组最终状态汇总会直接使用
- `internal/report/collector.go` 的 `Close`
  - 结论：保留
  - 原因：后续需要在引擎统一关闭时释放日志资源
- `internal/report/collector.go` 的 `RegisterDevice`
  - 结论：保留
  - 原因：后续可以用于任务启动前预注册设备，保证快照顺序稳定

## 4. 建议实施顺序

建议按下面顺序落地，风险最低：

1. 修状态机与 Tracker 单一数据源
2. 统一 `EngineService` / `TaskGroupService` 执行入口
3. 修设备页 ID 化写入与分页操作
4. 修任务组最终状态汇总
5. 修 QueryService 字段与排序白名单
6. 清理旧迁移逻辑与多余前端请求

## 5. 本次已完成的清理

本次已经完成低风险无用声明清理，后续可以直接在更干净的基础上推进主修复：

- 删除未接入的 `EventBridge`
- 删除未使用的前端事件 composable 与重复事件类型文件
- 删除若干无调用方的配置/命令组/状态包装函数
- 清理 `engineStore` 中未消费的事件日志状态

## 6. 下一步建议

下一步直接进入 P0，优先修三件事：

1. 状态机迁移
2. Tracker 单一实例
3. 任务组执行链与快照链统一

这三项修完以后，前后端主流程会先恢复一致，后面的设备页和查询修复就会顺很多。
