# 分页竞态修复 - 实施状态报告

> **最后更新**: 2026-03-22
> **状态**: Phase 0-7 已完成，Phase 8 推迟（保留旧代码作为后备）

## 实施概要

### 已完成阶段

| 阶段    | 状态        | 说明                       |
| ------- | ----------- | -------------------------- |
| Phase 0 | ✅ 已完成   | 止血与基线                 |
| Phase 1 | ✅ 已完成   | 抽离纯状态迁移             |
| Phase 2 | ✅ 已完成   | 引入 Detector/Driver       |
| Phase 3 | ✅ 已完成   | 集成与灰度                 |
| Phase 4 | ✅ 已完成   | 收拢引擎生命周期           |
| Phase 5 | ✅ 已完成   | 统一运行态投影             |
| Phase 6 | ⚠️ 部分完成 | 删除旧代码与文档补齐       |
| Phase 7 | ✅ 已完成   | 架构迁移（适配器模式）     |
| Phase 8 | ⏸️ 推迟     | 清理旧代码（保留作为后备） |

### Phase 4 完成详情

#### 4.1 删除 UI 对 EngineState 的外部推进

- **文件**: `internal/ui/task_group_service.go`
- **修改**: 移除 `session.TransitionTo()` 调用，改用轻量级 `BeginCompositeExecution(meta, totalDevices)`

#### 4.2 删除 executionManager 的状态推进

- **文件**: `internal/ui/execution_manager.go`
- **修改**:
  - 新增 `ExecutionSession` 结构体用于轻量级会话管理
  - `managedExecution.TransitionTo()` 改为 no-op
  - 移除 `finishLifecycle()` 状态推送逻辑

#### 4.3 收窄 Engine.TransitionTo() 可见性

- **文件**: `internal/engine/engine.go`
- **修改**: 删除公共 `TransitionTo()` 方法，引擎内部管理自己的生命周期

#### 4.4 重构复合任务执行

- **文件**: `internal/ui/execution_manager.go`
- **修改**: 新增 `beginLightweightSession()` 方法

### Phase 5 完成详情

#### 5.1-5.2 ExecutionSnapshot 结构

- **文件**: `internal/report/collector.go`
- **状态**: 已存在，无需修改

#### 5.3 降级 TaskGroup.Status

- **状态**: 保持现有持久化逻辑

#### 5.4 更新前端 Store

- **文件**: `frontend/src/stores/engineStore.ts`
- **修改**:
  - 移除 `EngineState` 类型
  - 移除 `ACTIVE_ENGINE_STATES` 常量
  - 简化 `syncExecutionState()` 只使用 `ExecutionSnapshot`

#### 5.5 简化 GetEngineState() 接口

- **文件**: `internal/ui/engine_service.go`
- **修改**: 添加废弃注释

### Phase 6 完成详情

#### 6.1 删除旧 handler 分支

- **状态**: ⚠️ 推迟
- **原因**: 需要更大规模重构
- **说明**: 当前架构中存在两套状态管理系统：
  - `SessionMachine` - 当前正在使用，被 `StreamEngine` 直接调用
  - `SessionReducer` - 新的 Reducer 模式，目前只在测试中使用
- **后续工作**: 需要将 `StreamEngine` 从使用 `SessionMachine` 迁移到使用 `SessionReducer`

#### 6.2 删除无用状态和注释

- **状态**: ✅ 已完成
- **修改**: 清理 `session_machine.go` 中的调试日志

#### 6.3 更新设计文档

- **状态**: ✅ 已完成（本文档）

#### 6.4 建立长期 Regression Suite

- **状态**: ✅ 已完成
- **文件**: `internal/executor/session_golden_test.go`
- **测试场景**:
  - 分页符检测
  - 回车覆盖损坏
  - 提示符错位
  - 分页截断
  - 跨 chunk 提示符
  - 初始化残留清理
  - 错误继续处理

---

## Phase 7 完成详情（新增）

> **完成日期**: 2026-03-22
> **目标**: 将 StreamEngine 从 SessionMachine 迁移到适配器架构

### 7.1 创建适配层

- **文件**: `internal/executor/session_adapter.go`（新建）
- **内容**:
  - `SessionAdapter` 结构体：桥接新旧架构
  - `Feed()` 方法：统一入口，根据配置选择架构
  - `SetUseNewArchitecture()` 方法：运行时切换
  - 委托方法：保持与 `SessionMachine` 兼容的接口

```go
type SessionAdapter struct {
    // 旧架构组件
    machine *SessionMachine

    // 新架构组件
    detector *SessionDetector
    reducer  *SessionReducer

    // 迁移模式控制
    useNewArchitecture bool
}
```

### 7.2 迁移 StreamEngine

- **文件**: `internal/executor/stream_engine.go`
- **修改**:
  - 将 `machine *SessionMachine` 替换为 `adapter *SessionAdapter`
  - 所有 `e.machine.X` 调用改为 `e.adapter.X`
  - 从配置读取灰度开关

### 7.3 灰度切换配置

- **文件**: `internal/config/runtime_config.go`
- **修改**:
  - 添加 `UseNewSessionArchitecture bool` 配置项
  - 更新 `applyRuntimeSetting()` 处理布尔值
  - 更新 `SaveRuntimeConfig()` 保存新配置
  - 默认值：`false`（使用旧架构，安全）

### 7.4 集成测试

- **文件**: `internal/executor/session_adapter_test.go`（新建）
- **测试场景**:

| 测试名称                              | 场景       | 验证点       |
| ------------------------------------- | ---------- | ------------ |
| `TestAdapter_DefaultOldArchitecture`  | 默认架构   | 使用旧架构   |
| `TestAdapter_SwitchToNewArchitecture` | 切换架构   | 新架构初始化 |
| `TestAdapter_StateConsistency`        | 状态一致性 | 新旧状态对应 |
| `TestAdapter_CommandQueueConsistency` | 命令队列   | 队列一致性   |
| `TestAdapter_OutputConsistency`       | 输出一致性 | 输出行一致   |
| `TestAdapter_PaginationHandling`      | 分页处理   | 分页检测     |
| `TestAdapter_ErrorHandling`           | 错误处理   | 错误状态     |
| `TestAdapter_MultipleCommands`        | 多命令执行 | 结果收集     |
| `TestAdapter_ArchitectureMode`        | 架构模式   | 模式字符串   |
| `TestAdapter_Stats`                   | 统计信息   | 字段完整性   |
| `TestAdapter_ClearInitResiduals`      | 清空残留   | 无错误       |
| `TestAdapter_MarkFailed`              | 标记失败   | 状态正确     |

---

## Phase 8 推迟说明

> **决定**: 暂时保留旧代码作为后备
> **原因**:
>
> 1. 适配器模式已实现灰度切换，新旧架构可并存
> 2. 旧架构经过生产验证，作为后备方案更安全
> 3. 待新架构经过充分生产验证后再清理旧代码

### 保留的旧代码

| 文件                 | 保留内容              | 说明         |
| -------------------- | --------------------- | ------------ |
| `session_machine.go` | `SessionMachine` 结构 | 旧架构核心   |
| `session_machine.go` | `errorDecided` flag   | 错误决策标志 |
| `session_machine.go` | `errorContinue` flag  | 错误继续标志 |
| `session_machine.go` | `afterPager` flag     | 分页后标志   |
| `session_state.go`   | `StateSendCommand`    | 发送命令状态 |
| `session_state.go`   | `StateHandlingPager`  | 分页处理状态 |
| `session_state.go`   | `StateHandlingError`  | 错误处理状态 |
| `command_context.go` | `PaginationPending`   | 分页等待字段 |

---

## 测试覆盖

### 单元测试

- `session_reducer_test.go` - Reducer 单元测试
- `session_detector_test.go` - Detector 单元测试
- `session_driver_test.go` - Driver 单元测试
- `session_adapter_test.go` - Adapter 集成测试

### Golden 测试

- `session_golden_test.go` - 回归测试套件

### 不变量测试

- `session_invariants_test.go` - 状态不变量验证

---

## 架构对比

### 旧架构（SessionMachine）

```
┌─────────────────────────────────────────────────────┐
│                  SessionMachine                      │
│  ┌─────────┐  ┌─────────┐  ┌─────────────────────┐  │
│  │  State  │→│ Handler │→│ Action (副作用)      │  │
│  └─────────┘  └─────────┘  └─────────────────────┘  │
│       ↑              ↓                               │
│       └──────────────┘                               │
│         (状态 + 副作用耦合)                           │
└─────────────────────────────────────────────────────┘
```

### 新架构（Detector + Reducer + Driver）

```
┌─────────────────────────────────────────────────────┐
│                   SessionAdapter                     │
│  ┌──────────┐  ┌──────────┐  ┌──────────────────┐   │
│  │ Detector │→│ Reducer  │→│ Driver           │   │
│  │ (纯检测) │  │ (纯状态) │  │ (副作用执行)     │   │
│  └──────────┘  └──────────┘  └──────────────────┘   │
│       ↓              ↓               ↓               │
│   Events         NewState        Actions            │
└─────────────────────────────────────────────────────┘
```

---

## 下一步工作

1. **生产验证**: 在生产环境启用新架构（设置 `UseNewSessionArchitecture: true`）
2. **监控指标**: 收集新旧架构的性能和稳定性数据
3. **逐步迁移**: 确认新架构稳定后，逐步增加灰度比例
4. **最终清理**: 新架构完全稳定后，执行 Phase 8 清理旧代码

---

## 变更日志

| 日期       | 阶段    | 变更内容                     |
| ---------- | ------- | ---------------------------- |
| 2026-03-22 | Phase 7 | 创建 session_adapter.go      |
| 2026-03-22 | Phase 7 | 迁移 StreamEngine 使用适配器 |
| 2026-03-22 | Phase 7 | 添加灰度切换配置             |
| 2026-03-22 | Phase 7 | 创建集成测试                 |
| 2026-03-22 | Phase 8 | 决定推迟清理，保留旧代码     |
