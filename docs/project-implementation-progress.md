# 项目实施进度总览

> 最后更新: 2026-03-23
> 作用: 作为 `docs` 目录下唯一的项目实施进度口径，替代历史计划、分析、验证、阶段状态文档

## 1. 当前结论

项目核心重构已经基本落地，当前状态可归纳为:

- Phase 0-5: 已完成
- Phase 6-7: 已完成
- Phase 8: 已完成，旧状态、旧 flag、旧字段已清理
- 文档状态: 已收口到本文档，历史进度文档已删除

当前更准确的判断不是“只剩收尾”，而是:

- 主执行链路已经切到 `SessionAdapter`
- 运行态已经收敛为单一新实现
- 旧状态清理已完成
- `StreamEngine` 运行态已固定走新架构
- 兼容开关与桥接路径已从生产代码移除

## 2. 代码实况摘要

### 2.1 已落地内容

#### 执行架构

- `StreamEngine` 已接入 `SessionAdapter`
- `SessionAdapter`、`SessionDetector`、`SessionReducer` 均已存在
- 运行态已统一固定到单一会话实现

对应文件:

- [`internal/executor/stream_engine.go`](D:/Document/GO/NetWeaverGo/internal/executor/stream_engine.go)
- [`internal/executor/session_adapter.go`](D:/Document/GO/NetWeaverGo/internal/executor/session_adapter.go)
- [`internal/executor/session_detector.go`](D:/Document/GO/NetWeaverGo/internal/executor/session_detector.go)
- [`internal/executor/session_reducer.go`](D:/Document/GO/NetWeaverGo/internal/executor/session_reducer.go)
- [`internal/config/runtime_config.go`](D:/Document/GO/NetWeaverGo/internal/config/runtime_config.go)

#### UI 与运行态投影

- 前端已统一改为从 `ExecutionSnapshot` 获取运行态
- `GetEngineState()` 已降级为废弃兼容接口
- `executionManager` 已具备轻量级复合执行入口

对应文件:

- [`frontend/src/stores/engineStore.ts`](D:/Document/GO/NetWeaverGo/frontend/src/stores/engineStore.ts)
- [`internal/report/collector.go`](D:/Document/GO/NetWeaverGo/internal/report/collector.go)
- [`internal/ui/engine_service.go`](D:/Document/GO/NetWeaverGo/internal/ui/engine_service.go)
- [`internal/ui/execution_manager.go`](D:/Document/GO/NetWeaverGo/internal/ui/execution_manager.go)

#### 测试资产

- Reducer / Detector / Adapter 测试已存在
- Golden 测试与不变量测试已存在

对应文件:

- [`internal/executor/session_reducer_test.go`](D:/Document/GO/NetWeaverGo/internal/executor/session_reducer_test.go)
- [`internal/executor/session_detector_test.go`](D:/Document/GO/NetWeaverGo/internal/executor/session_detector_test.go)
- [`internal/executor/session_adapter_test.go`](D:/Document/GO/NetWeaverGo/internal/executor/session_adapter_test.go)
- [`internal/executor/session_golden_test.go`](D:/Document/GO/NetWeaverGo/internal/executor/session_golden_test.go)
- [`internal/executor/session_invariants_test.go`](D:/Document/GO/NetWeaverGo/internal/executor/session_invariants_test.go)

### 2.2 已完成的清理项

以下旧状态面和隐式状态已从代码中移除:

- `StateSendCommand`
- `StateHandlingPager`
- `StateHandlingError`
- `PaginationPending`
- `afterPager`
- `errorDecided`
- `errorContinue`

对应文件:

- [`internal/executor/session_state.go`](D:/Document/GO/NetWeaverGo/internal/executor/session_state.go)
- [`internal/executor/session_adapter.go`](D:/Document/GO/NetWeaverGo/internal/executor/session_adapter.go)
- [`internal/executor/session_reducer.go`](D:/Document/GO/NetWeaverGo/internal/executor/session_reducer.go)
- [`internal/executor/command_context.go`](D:/Document/GO/NetWeaverGo/internal/executor/command_context.go)

### 2.3 当前实现边界

以下内容说明项目已完成旧状态清理，也已完成生产代码去兼容化:

- `SessionAdapter` 已收缩为单一 `Replayer + Detector + Reducer` 实现
- 主执行链路直接消费统一的 `SessionAction`
- `UseNewSessionArchitecture` 配置字段已删除
- 初始化流程已通过 `PromptTracker` 接口解耦，仅依赖 `SessionAdapter`
- `SessionMachine` 已从仓库删除

## 3. 分阶段进度判断

### Phase 0: 止血与基线

状态: 已完成

说明:

- 基础测试、基线脚本、关键修复项已经具备

### Phase 1: 抽离纯状态迁移

状态: 已完成

说明:

- `session_types.go` 与 `session_reducer.go` 已存在并配套测试

### Phase 2: 拆分 Detector / Driver

状态: 已完成

说明:

- Detector 能力已保留并接入主链路
- Driver 抽象已删除，动作执行已回收至 `StreamEngine`

### Phase 3: 去掉隐式状态

状态: 已完成

说明:

- 旧状态与旧 flag 已从实现中移除

### Phase 4: 收拢引擎生命周期

状态: 基本完成

说明:

- UI 外部状态推进已明显收窄
- 兼容接口仍保留，但主路径已不是旧的外推方式

### Phase 5: 统一运行态投影

状态: 已完成

说明:

- 前端与后端主口径已收敛到 `ExecutionSnapshot`

### Phase 6: 删除旧代码与文档补齐

状态: 已完成

说明:

- 文档已收口
- 测试资产已补齐
- 旧状态与旧 flag 已清理
- 运行态兼容桥接代码已移除

### Phase 7: 架构迁移

状态: 已完成

说明:

- `StreamEngine` 已迁移到 `SessionAdapter`
- `StreamEngine` 主链路已改为消费统一 `SessionAction`
- 运行态固定走新架构
- `Initializer` 已改为依赖最小提示符跟踪接口
- `SessionAdapter` 已移除双轨切换和 legacy action 转换

### Phase 8: 清理旧代码

状态: 已完成

已完成项:

- 删除旧状态枚举 `StateSendCommand`、`StateHandlingPager`、`StateHandlingError`
- 删除旧分页字段 `PaginationPending`
- 删除旧 flag `afterPager`、`errorDecided`、`errorContinue`
- 将旧分页/错误专用 handler 退出主流程

## 4. 当前可执行结论

如果从“项目能否运行、能否继续开发”看，当前阶段已经可以继续推进新架构验证与后续重构。

如果从“是否完成状态机旧状态清理”看，答案是肯定的。

如果从“是否完成整个架构去兼容化收尾”看，生产代码范围内答案是肯定的。

当前剩余工作不再属于兼容清理，而是后续演进选项:

1. 继续收缩仅用于调试或历史迁移的辅助接口
2. 进一步压缩状态投影和调试 API

## 5. 验证记录

已执行并通过:

- `go test ./internal/executor/...`
- `go test ./internal/ui/...`
- `go test ./internal/config/...`

说明:

- 当前结论基于文档梳理、代码检查和以上测试结果

## 6. docs 目录保留策略

为避免后续混淆，`docs` 目录仅保留:

- 本文: 唯一的项目实施进度口径
- [`docs/state-transition-table.md`](D:/Document/GO/NetWeaverGo/docs/state-transition-table.md): 状态迁移参考文档

其余历史计划、阶段状态、验证、分析文档均已删除。
