# 执行事件链路重构代码审计问题清单

## 1. 审计范围

本次审计仅覆盖本轮改动涉及的模块：

- `internal/executor/session_types.go`
- `internal/executor/session_reducer.go`
- `internal/executor/session_adapter.go`
- `internal/executor/stream_engine.go`
- `internal/taskexec/eventbus.go`
- `internal/taskexec/snapshot.go`
- `internal/taskexec/runtime.go`
- `internal/taskexec/execution_record_projector.go`
- `internal/taskexec/executor_impl.go`
- `internal/taskexec/service.go`
- `internal/ui/taskexec_event_bridge.go`
- `internal/ui/taskexec_ui_service.go`
- `frontend/src/types/taskexec.ts`
- `frontend/src/stores/taskexecStore.ts`

审计目标：

1. 反向核查本轮改造后的流程闭环是否成立
2. 检查架构目标与当前实现是否存在偏差
3. 查找明显逻辑漏洞、时序问题、状态一致性问题、测试盲区
4. 输出后续修复优先级建议

---

## 2. 总体结论

本轮实现已经把系统推进到了更明确的中间态：

- 前端已具备 `snapshot delta + seq` 的接入骨架
- 执行引擎已具备兼容式 `TransitionBatch + SessionEffect` 过渡层
- `runtime` 与 `executor_impl` 的终态/生命周期写法已经开始收口

但从代码审计角度看，当前仍存在几个高风险问题：

1. **事件订阅执行顺序不稳定，可能导致桥接层读取到“当前事件之前”的旧快照**
2. **当前 `SnapshotDelta` 不是严格意义上的 delta，而是“最新全量快照 + 序号”**
3. **增量序号完全依赖内存，不具备重启后连续性，repo 重建也无法恢复 seq 语义**
4. **前端并未真正实现 gap 检测与补拉机制，协议只完成了表层接入**
5. **`TaskEvent` 仍然不是完整的命令事实流，UI 无法还原完整的 dispatched/completed 顺序**

结论：

**当前实现可编译、可运行、可继续演进，但尚不能判定为“Journal-first + delta protocol + effect model”已经可靠闭环。**

---

## 3. 问题清单

## 问题 1：事件总线订阅执行顺序不稳定，可能让 UI 桥接读取到旧快照

- 严重级别：高
- 涉及模块：
  - `internal/taskexec/eventbus.go`
  - `internal/ui/taskexec_event_bridge.go`
- 现象：
  - `SnapshotHub` 与 UI Bridge 都通过 `EventBus.Subscribe(...)` 订阅同一事件流
  - `EventBus.snapshotHandlers()` 从 `map` 中遍历 handler，执行顺序不稳定
  - `EmitSync()` 虽然串行调用 handler，但**串行顺序本身并不可靠**
- 风险：
  - 某次事件到达时，UI Bridge 可能先于 `SnapshotHub.AppendEvent(...)` 执行
  - Bridge 随后调用 `GetSnapshotDelta(...)` / `GetSnapshot(...)`，拿到的是**尚未应用当前事件**的旧快照
  - 前端看到的 `task:event` 与 `task:snapshot_delta` 可能不是同一时点的状态
- 代码证据：
  - `internal/taskexec/eventbus.go` 中 `snapshotHandlers()` 直接遍历 `handlers map`
  - `internal/ui/taskexec_event_bridge.go` 中 `handleEvent()` 在收到事件后立即读取快照/增量
- 结论：
  - 当前实现依赖“事件处理器顺序天然稳定”的假设，但代码并未保证
- 修复建议：
  1. `EventBus` 改为有序订阅结构，至少保证订阅顺序稳定
  2. 将 `SnapshotHub`、仓储投影、UI Bridge 划分为固定顺序的 projector pipeline
  3. 最理想方案是：先投影，后桥接，而不是在 bridge 中二次回查快照

---

## 问题 2：`SnapshotDelta` 仍是伪 delta，不是真正的增量协议

- 严重级别：高
- 涉及模块：
  - `internal/taskexec/eventbus.go`
  - `internal/taskexec/runtime.go`
  - `internal/ui/taskexec_event_bridge.go`
  - `frontend/src/stores/taskexecStore.ts`
- 现象：
  - `BuildDelta(runID)` 直接返回最新快照副本
  - `SnapshotDelta` 不包含 patch 集，也不接受 `fromSeq`
  - Bridge 每次事件到达时，都会取“当前最新快照”并发给前端
- 风险：
  - 如果两个事件在短时间内连续到达，前端可能直接收到后态，跳过中间态
  - 前端无法知道自己是“逐条消费”还是“被最新状态覆盖”
  - 这不满足文档中“按 seq 增量应用 delta”的目标语义
- 代码证据：
  - `internal/taskexec/eventbus.go` 中 `BuildDelta()` 只返回 `Snapshot`
  - `frontend/src/stores/taskexecStore.ts` 中 `applySnapshotDelta()` 只做“新 seq 覆盖旧快照”
- 结论：
  - 当前协议本质上是“带 seq 标签的全量快照推送”，不是严格 delta 协议
- 修复建议：
  1. 定义 `SnapshotPatch` / `DeltaOp[]`
  2. `BuildDelta(fromSeq)` 返回从指定序号之后的 patch 集合
  3. 前端 store 只消费 patch，不再对每个事件覆盖整份 snapshot

---

## 问题 3：增量序号完全依赖内存，进程重启或 repo 重建后会失真

- 严重级别：高
- 涉及模块：
  - `internal/taskexec/eventbus.go`
  - `internal/taskexec/snapshot.go`
  - `internal/taskexec/runtime.go`
  - `internal/taskexec/service.go`
- 现象：
  - `SnapshotHub.revisions` 是纯内存 map
  - `LastRunSeq` 与 `EventSnapshot.Seq` 都由 `touchSnapshotLocked()` 和 `AppendEvent()` 动态赋值
  - `SnapshotBuilder.Build(...)` 从 repo 重建快照时，无法恢复这些 seq 字段
- 风险：
  - 服务重启后 `revisions` 归零，`lastRunSeq` 重新开始计数
  - repo 回建的快照不具备连续 seq，前端无法判断是否丢包或回退
  - `GetSnapshotDelta()` 在 cache miss -> rebuild 的路径下，拿到的是重新编号后的增量
- 代码证据：
  - `internal/taskexec/eventbus.go` 中 `revisions map[string]uint64`
  - `internal/taskexec/snapshot.go` 中 `Build(...)` 没有从 repo 恢复 `lastRunSeq` / `event seq`
- 结论：
  - 当前序号系统只适用于“单进程内存态”，不适用于真正的可重放、可恢复增量协议
- 修复建议：
  1. 把 run seq / session seq / event seq 纳入 journal 或事件索引持久化
  2. repo 回建必须能够恢复最后应用到的 seq
  3. `GetSnapshotDelta()` 应显式区分“热内存 delta”和“冷启动回放 delta”

---

## 问题 4：`GetSnapshot()` 缓存未命中回建路径返回的快照序号不一致

- 严重级别：中
- 涉及模块：
  - `internal/taskexec/eventbus.go`
  - `internal/taskexec/runtime.go`
- 现象：
  - `SnapshotHub.Update()` 会对 clone 调用 `touchSnapshotLocked()`
  - 但只把 `Revision`、`UpdatedAt` 回写给调用方 snapshot
  - `LastRunSeq`、`LastSessionSeqByUnit` 没有同步回原对象
- 风险：
  - `rebuildSnapshotFromRepo()` 返回的对象与缓存中对象的序号字段不一致
  - API 直接返回给前端的 snapshot 可能是 `lastRunSeq=0`，而缓存里已经是递增序号
  - 这会使前端第一帧状态与后续 delta 序列不一致
- 结论：
  - 这是明显的一致性漏洞，虽然当前可能被后续 delta 覆盖，但依然是错误状态
- 修复建议：
  1. `Update()` 回写时同步 `LastRunSeq` 与 `LastSessionSeqByUnit`
  2. 或统一返回 clone 后对象，禁止返回原始 rebuild 结果

---

## 问题 5：前端没有真正实现 gap 检测与回源补拉

- 严重级别：高
- 涉及模块：
  - `frontend/src/stores/taskexecStore.ts`
  - `internal/ui/taskexec_event_bridge.go`
- 现象：
  - `applySnapshotDelta()` 只做 `delta.seq <= currentSeq` 丢弃判断
  - 对 `delta.seq > currentSeq + 1` 的 gap 没有任何处理
  - 文档目标中明确要求发现 gap 时回源拉全量快照，但代码未实现
- 风险：
  - 丢事件时前端不会自愈
  - 只要 bridge、Wails、前端任一侧发生抖动，页面就可能长期停留在错误状态
- 结论：
  - 当前前端增量消费只是“接受新 seq”，还不是“可靠消费协议”
- 修复建议：
  1. store 中增加 `if delta.seq > currentSeq + 1` 的 gap 分支
  2. gap 时自动调用 `GetTaskSnapshot(...)` 或新的 `GetTaskSnapshotDelta(fromSeq)`
  3. 把 gap、回补成功、回补失败记录到调试日志

---

## 问题 6：UI Bridge 同时推 `task:snapshot_delta` 与 `task:snapshot_data`，导致重复且掩盖协议问题

- 严重级别：中
- 涉及模块：
  - `internal/ui/taskexec_event_bridge.go`
  - `frontend/src/stores/taskexecStore.ts`
- 现象：
  - 每次事件到来时，Bridge 会：
    - 发送 `task:snapshot_delta`
    - 再发送 `task:snapshot_data`
  - 前端 Store 两条通道都会消费
- 风险：
  - 传输体积翻倍
  - “delta 是否正确”被全量 snapshot 覆盖掩盖
  - 调试时难以判断页面是靠 delta 更新，还是靠 full snapshot 修复
- 结论：
  - 当前兼容策略虽然降低了接入风险，但对审计和后续演进是不利的
- 修复建议：
  1. 明确阶段切换：默认只推 `task:snapshot_delta`
  2. `task:snapshot_data` 仅用于首次加载和 gap 回补
  3. 若保留双发，也应加 debug 标识便于区分来源

---

## 问题 7：UI 侧仍拿不到完整命令事实流，`sessionSeq` 也不是完整序列

- 严重级别：中高
- 涉及模块：
  - `internal/taskexec/execution_record_projector.go`
  - `internal/taskexec/eventbus.go`
  - `frontend/src/stores/taskexecStore.ts`
- 现象：
  - `projectExecutorRecord(...)` 在 `RecordCommandDispatched` 时只写 summary，不发 `TaskEvent`
  - `TaskEvent` 的 `sessionSeq` 只在 `RecordCommandCompleted / RecordCommandFailed` 路径里通过 payload 带出
  - `SnapshotHub.updateSnapshotSessionSeq(...)` 依赖 `TaskEvent.payload.sessionSeq`
- 风险：
  - UI 拿不到 `CommandDispatched` 事实流
  - `lastSessionSeqByUnit` 只记录部分命令生命周期，而不是完整单调序列
  - 无法在前端严格校验 `Completed(n) < Dispatched(n+1)`
- 结论：
  - 当前 UI 看到的是“部分命令事实”，不是完整的会话事实流
- 修复建议：
  1. `CommandDispatched` 也应投影为结构化 `TaskEvent`
  2. `sessionSeq` 应成为完整事件统一字段，而不是零散放在 payload 中
  3. 前端 event log 应能够展示完整命令开始/完成顺序

---

## 问题 8：`Journal` 仍然是混合 schema，未形成单一结构化事实模型

- 严重级别：中
- 涉及模块：
  - `internal/taskexec/execution_record_projector.go`
  - `internal/executor/execution_plan.go`
- 现象：
  - `projectTaskexecLifecycleRecord(...)` 写入的是 `taskexecLifecycleRecord`
  - `projectExecutorRecord(...)` 写入的是 `executor.ExecutionEvent`
  - 同一个 journal 文件中存在两种结构
- 风险：
  - 回放、重建、解析会变复杂
  - 后续如果继续加 schema，会放大解析分支成本
  - 这与“ExecutionRecord 统一事实源”的目标不一致
- 结论：
  - 当前 journal 仍是“结构化日志集合”，还不是“统一事实源模型”
- 修复建议：
  1. 收敛到单一 `ExecutionRecord` 结构
  2. 所有 lifecycle / command / progress 事件都落同一 schema
  3. 由 projector 做展示翻译，不在 journal 中保留多种事件模型

---

## 问题 9：`TransitionBatch / SessionEffect` 仍然是浅包装，尚未形成真正的 reducer-transition-effect 边界

- 严重级别：中
- 涉及模块：
  - `internal/executor/session_types.go`
  - `internal/executor/session_reducer.go`
  - `internal/executor/session_adapter.go`
  - `internal/executor/stream_engine.go`
- 现象：
  - `TransitionBatch` 当前只有 `Effects`
  - `SessionEffect` 当前只是 `SessionAction` 的包装层
  - `executeSessionEffect(...)` 仍然立即回退为 `executeSessionAction(...)`
- 风险：
  - 代码表面上“已经有 batch/effect”，但领域事实与副作用并未真正拆开
  - 后续团队容易误判当前阶段完成度
- 结论：
  - 当前是合理过渡层，但绝不能把它当成最终架构
- 修复建议：
  1. 在 reducer 中显式引入 `Transitions[]`
  2. 把 effect 执行结果回流为新的 `SessionEvent`
  3. 逐步淘汰 `AsAction()` 兼容回退路径

---

## 问题 10：测试覆盖仍缺少 UI Bridge / 前端 Store / 跨层时序回归

- 严重级别：中
- 涉及模块：
  - `internal/ui/taskexec_event_bridge.go`
  - `frontend/src/stores/taskexecStore.ts`
  - `internal/taskexec/eventbus.go`
- 现象：
  - 当前已经补了 `SnapshotDelta`、`TransitionBatch`、runtime helper 的测试
  - 但缺少以下关键用例：
    - Bridge 订阅顺序导致旧快照被发送
    - 同一 run 快速连续事件下 delta 序号是否单调
    - 前端 gap 检测与回补
    - `task:snapshot_delta` 与 `task:snapshot_data` 双发下的最终一致性
- 风险：
  - 现在最脆弱的部分恰恰是“跨层交互”，但自动化覆盖不足
- 修复建议：
  1. 为 Bridge 增加后端单测或集成测试
  2. 为 Store 增加前端单测，覆盖乱序、重复、gap、补拉
  3. 增加端到端验证：事件 -> SnapshotHub -> Bridge -> Store 的全链路时序测试

---

## 4. 新增专项审计：兼容/旧路径/双轨逻辑残留

## 问题 11：Bridge 仍保留运行时双发与旧桥接事件

- 严重级别：P0
- 涉及模块：
  - `internal/ui/taskexec_event_bridge.go`
  - `frontend/src/stores/taskexecStore.ts`
- 现象：
  - 每次事件到达时仍会同时发送：
    - `task:snapshot_delta`
    - `task:snapshot_data`
    - `task:snapshot`
  - 前端 Store 仍同时监听 `task:snapshot_delta`、`task:snapshot_data`、`task:snapshot`
- 代码证据：
  - `internal/ui/taskexec_event_bridge.go` 中 `handleEvent()` 同时发 `task:snapshot_delta` 与 `task:snapshot_data`，并继续发兼容事件 `task:snapshot`
  - `frontend/src/stores/taskexecStore.ts` 中 `initListeners()` 同时监听三条链路
- 风险：
  - 前端状态正确性无法证明来自 delta 主链路
  - 任何 delta 缺陷都会被全量快照覆盖掩盖
  - 旧桥接事件会让后续硬切换成本继续升高
- 硬切建议：
  1. 删除运行时 `task:snapshot_data` 双发
  2. 删除运行时 `task:snapshot` 兼容事件
  3. `task:snapshot_data` 仅允许作为页面首次 init API 返回值，不允许作为事件桥接常态输出
  4. 前端 Store 删除对 `task:snapshot_data` / `task:snapshot` 的运行时监听

---

## 问题 12：执行基座仍保留明显兼容接口，旧动作路径没有被真正切掉

- 严重级别：P0
- 涉及模块：
  - `internal/executor/session_types.go`
  - `internal/executor/session_reducer.go`
  - `internal/executor/session_adapter.go`
  - `internal/executor/stream_engine.go`
- 现象：
  - `SessionEffect` 仍定义了 `AsAction()`
  - `TransitionBatch` 仍通过 `ToActions()` 回退到旧动作数组
  - `SessionReducer.Reduce()` 仍作为兼容出口存在
  - `SessionAdapter.FeedSessionActions()`、`ResolveErrorActions()`、`ReduceEvent()` 仍保留旧接口
  - `StreamEngine.executeSessionEffect()` 只是把 effect 回退成 action 再执行
- 代码证据：
  - `internal/executor/session_types.go` 中 `AsAction()`、`ToActions()`
  - `internal/executor/session_reducer.go` 中 `Reduce()` 注释明确写着“兼容旧接口”
  - `internal/executor/session_adapter.go` 中 `FeedSessionActions()`、`ResolveErrorActions()`、`ReduceEvent()`、`ResolveError()`
  - `internal/executor/stream_engine.go` 中 `processChunk()` 仍返回 `[]SessionAction`
- 风险：
  - 当前所谓 `TransitionBatch + SessionEffect` 仍然是“包装旧实现”
  - 团队很容易误以为已经完成架构切换
  - 后续继续在旧接口上叠逻辑，会让硬切越来越难
- 硬切建议：
  1. 删除 `AsAction()`
  2. 删除 `ToActions()`
  3. 删除 `Reduce()`、`FeedSessionActions()`、`ResolveErrorActions()`、`ReduceEvent()`、`ResolveError()` 等兼容接口
  4. `StreamEngine` 只接受 `TransitionBatch` 和 `SessionEffect`
  5. effect 执行结果必须回流 actor，不再经由旧 action 主路径

---

## 问题 13：`Results()` 与 `emitNewCommandCompleteEvents()` 仍是旧链路残留

- 严重级别：P0
- 涉及模块：
  - `internal/executor/session_adapter.go`
  - `internal/executor/stream_engine.go`
- 现象：
  - `SessionAdapter.Results()` 仍对外暴露结果归档列表
  - `StreamEngine` 仍频繁调用 `emitNewCommandCompleteEvents()`，从 `Results()` 增量补发完成事件
- 代码证据：
  - `internal/executor/session_adapter.go` 中 `Results()`
  - `internal/executor/stream_engine.go` 中大量 `emitNewCommandCompleteEvents()` 调用
- 风险：
  - 这与目标架构“禁止从 Results 派生完成事件”直接冲突
  - 说明命令完成事实仍不是 actor/reducer 事务内直接写 journal 的结果
  - 这是旧链路未删除的最核心证据之一
- 硬切建议：
  1. 删除 `emitNewCommandCompleteEvents()`
  2. 删除通过 `Results()` 推导完成事件的逻辑
  3. `CommandCompleted` / `CommandFailed` 必须由 reducer transition 直接产生并写 journal
  4. `Results()` 只保留最终报表用途，禁止参与事实流构建

---

## 问题 14：测试代码仍大量绑定旧接口，说明硬切换尚未完成

- 严重级别：P1
- 涉及模块：
  - `internal/executor/session_adapter_test.go`
  - `internal/executor/session_invariants_test.go`
  - `internal/executor/session_golden_test.go`
  - `internal/executor/session_reducer_test.go`
- 现象：
  - 多个测试仍直接调用：
    - `FeedSessionActions(...)`
    - `Reduce(...)`
    - `Results()`
    - `ToActions()`
- 风险：
  - 测试正在替旧路径背书
  - 即使业务代码切掉旧接口，测试也会成为反向阻力
- 硬切建议：
  1. 所有执行基座测试统一改为：
     - `ReduceBatch(...)`
     - `FeedTransitionBatch(...)`
     - `Effects[]`
  2. 删除针对兼容接口的测试
  3. 新增 actor + effect 回流测试取代旧 action 测试

---

## 问题 15：文档与注释中仍明确标记“兼容旧接口/旧链路”，说明架构边界未冻结

- 严重级别：P1
- 涉及模块：
  - `internal/executor/session_reducer.go`
  - `internal/executor/session_adapter.go`
  - `internal/executor/execution_plan.go`
  - `frontend/src/stores/taskexecStore.ts`
- 现象：
  - 注释仍大量出现：
    - “兼容旧接口”
    - “兼容旧桥接事件”
    - “兼容旧链路”
- 风险：
  - 代码层和团队认知层都会默认“旧接口暂时留着没关系”
  - 在新建项目阶段，这是极高风险的惯性来源
- 硬切建议：
  1. 删除所有与执行链路重构相关的兼容注释和兼容出口
  2. 新文档只允许描述目标主链，不允许把中间态写成默认路径

---

## 5. 优先级建议

### P0 立即处理

1. 修复事件总线订阅顺序不稳定问题
2. 删除 Bridge 的 `task:snapshot_data` / `task:snapshot` 运行时双发
3. 删除执行基座兼容接口：`Reduce()`、`FeedSessionActions()`、`ResolveErrorActions()`、`ReduceEvent()`、`AsAction()`、`ToActions()`
4. 删除 `emitNewCommandCompleteEvents()` 与从 `Results()` 派生完成事件的旧链路
5. 定义真正的 delta 协议或至少引入 gap 检测与补拉
6. 明确 seq 的持久化策略，避免重启后序号失真

### P1 尽快处理

1. 让 UI 拿到完整的命令事实流，而不是只有部分 completion 事件
2. 统一 journal schema
3. 删除所有测试和注释中的旧接口依赖，防止兼容逻辑回流

### P2 后续演进

1. 从兼容式 `TransitionBatch / SessionEffect` 演进到真正的 `Transitions + Effects`
2. 把 `runtime` 继续壳化，减少业务状态判断残留
3. 完善 UI / Store / Bridge 的跨层回归测试矩阵
4. 清理项目中其他非本链路的兼容别名与旧接口残留

---

## 5. 审计结论

当前代码并非“有明显无法运行的大面积坏死问题”，而是典型的**中间态系统风险**：

- 本地编译、单测、构建均可通过
- 但多个关键承诺仍停留在“骨架接入”而不是“严格语义闭环”
- 一旦进入高频事件、多设备并发、进程重启、桥接抖动等真实运行场景，现有缺口会被迅速放大

因此建议将本问题清单作为下一轮修复基线，优先按 `P0 -> P1 -> P2` 顺序推进。
