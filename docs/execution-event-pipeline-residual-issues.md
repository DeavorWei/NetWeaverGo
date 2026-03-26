# 执行事件链路硬切后残留问题清单

> 目标：聚焦当前仍未闭环的问题，作为后续收敛基线。
> 范围：仅覆盖执行链路与 taskexec 前后端增量协议主线。

## P0（必须优先清理）

### 1. EventBus 订阅处理顺序仍不稳定

- 现象：处理器快照来自 map 遍历，顺序不保证稳定。
- 证据：[`snapshotHandlers()`](../internal/taskexec/eventbus.go:245)、[`handleEvent()`](../internal/ui/taskexec_event_bridge.go:60)。
- 风险：UI Bridge 可能先于 SnapshotHub 投影，导致读取到旧状态。
- 建议：改为有序订阅结构（slice + id），明确固定 pipeline 顺序。

### 2. 增量 seq 仍是内存态，未持久化

- 现象：seq 仍依赖内存 map。
- 证据：[`SnapshotHub.revisions`](../internal/taskexec/eventbus.go:273)。
- 风险：重启后 seq 语义断裂，无法严格保证可恢复增量消费。
- 建议：把 run_seq/session_seq 写入统一 journal 索引并可回放恢复。

### 3. 命令事实流对 UI 仍不完整

- 现象：`dispatched` 仍未等价投影为 UI 事实事件，主要看到 completion 侧事件。
- 证据：[`projectExecutorRecord()`](../internal/taskexec/execution_record_projector.go:83)。
- 风险：前端无法严格重建 `Dispatched -> Completed/Failed` 完整序列。
- 建议：统一把关键命令生命周期投影为结构化 UI 事件，并使用统一序号字段。

## P1（应尽快清理）

### 4. 执行链路仍保留兼容残留接口语义

- 现象：`Results()` 接口仍对外暴露，且 `Run` 返回路径仍依赖它。
- 证据：[`Results()`](../internal/executor/session_adapter.go:88)、[`Run()`](../internal/executor/stream_engine.go:91)。
- 风险：团队认知上容易继续把结果集合与事实事件混用。
- 建议：将 `Results()` 严格标注为 report-only，必要时收敛访问边界。

### 5. 兼容旧链路语义注释仍在核心模型中

- 现象：执行事件类型注释仍含“兼容旧链路”表述。
- 证据：[`ExecutionEventType`](../internal/executor/execution_plan.go:169)。
- 风险：弱化硬切边界，后续变更容易回流兼容思路。
- 建议：清理兼容注释，统一以新主链术语描述。

### 6. Projector 拆分处于进行中，边界尚未完全冻结

- 现象：已拆出模块，但 runtime/executor_impl 仍有部分混合职责调用链。
- 证据：[`progress_projector.go`](../internal/taskexec/progress_projector.go)、[`ui_event_projector.go`](../internal/taskexec/ui_event_projector.go)、[`runtime.go`](../internal/taskexec/runtime.go:769)、[`executor_impl.go`](../internal/taskexec/executor_impl.go:255)。
- 风险：继续演进时可能出现职责回流。
- 建议：补齐 projector 职责文档与单测分层，禁止跨层写状态。

### 7. 跨层时序回归测试仍不足

- 现象：已有单元覆盖，但 Actor -> Projector -> Bridge -> Store 全链路时序场景不足。
- 证据：现有测试集中在 [`snapshot_hub_test.go`](../internal/taskexec/snapshot_hub_test.go) 与 [`taskexec_test.go`](../internal/taskexec/taskexec_test.go)。
- 风险：高并发或抖动场景下问题不易提前暴露。
- 建议：补全乱序、重复、gap、快速连续事件等集成回归用例。

## P2（全局治理项）

### 8. 项目中仍有非主线兼容/legacy 注释与逻辑

- 现象：仓库其他模块仍存在“兼容旧版/兼容老旧设备/fallback”描述。
- 证据示例：[`execution_plan.go`](../internal/executor/execution_plan.go:169)（主线内）；其他模块如 SSH/配置/历史服务仍有兼容描述。
- 风险：影响“新项目不保留兼容层”的全局约束一致性。
- 建议：分批次做仓库级兼容清理，主线优先、外围模块次之。

---

## 当前结论

- 核心硬切成果已经成立：前端双发删除、delta patch 协议落地、旧补发链移除。
- 仍需围绕“顺序稳定性、序号持久化、事实流完整性、测试矩阵”继续收口，才能达到 docs 目标中的严格闭环状态。
