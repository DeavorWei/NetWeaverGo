# 分页竞态修复 - 剩余工作实施计划

> **创建日期**: 2026-03-22
> **基于**: `implementation-analysis-report.md` 分析结果
> **目标**: 完成未实施的 13% 工作，实现 100% 架构迁移

---

## 一、剩余工作概览

### 当前状态

```
已完成: ██████████████████░░  87%
待完成: ██░░░░░░░░░░░░░░░░░░  13%
```

### 核心问题

| 问题                        | 影响                   | 优先级 |
| --------------------------- | ---------------------- | ------ |
| StreamEngine 未迁移到新架构 | 两套系统并存，代码冗余 | P0     |
| 旧 flag 未清理              | 状态管理混乱           | P1     |
| 旧状态枚举未删除            | 代码可维护性差         | P2     |

---

## 二、实施阶段规划

### Phase 7: 架构迁移（新增）

**目标**: 将 StreamEngine 从 SessionMachine 迁移到 SessionDetector + SessionReducer + SessionDriver 架构

**依赖**: Phase 1-2 已完成（组件已实现）

#### 7.1 创建适配层

**任务**: 创建兼容层，使新架构可以无缝替换旧实现

**新建文件**: `internal/executor/session_adapter.go`

```go
// SessionAdapter 适配器 - 桥接新旧架构
type SessionAdapter struct {
    // 旧架构
    machine *SessionMachine

    // 新架构
    detector *SessionDetector
    reducer  *SessionReducer
    driver   *SessionDriver

    // 迁移模式
    useNewArchitecture bool
}

// Feed 统一入口
func (a *SessionAdapter) Feed(chunk string) []Action {
    if a.useNewArchitecture {
        return a.feedNew(chunk)
    }
    return a.machine.Feed(chunk)
}

// feedNew 新架构处理流程
func (a *SessionAdapter) feedNew(chunk string) []Action {
    // 1. Replayer 处理 chunk
    lines := a.machine.replayer.Process(chunk)

    // 2. Detector 检测事件
    events := a.detector.Detect(lines, a.machine.ActiveLine())

    // 3. Reducer 状态归约
    var actions []SessionAction
    for _, event := range events {
        newActions := a.reducer.Reduce(event)
        actions = append(actions, newActions...)
    }

    // 4. 转换为旧格式 Action（兼容层）
    return a.convertActions(actions)
}
```

**验收标准**:

- [ ] 适配层实现完成
- [ ] 新旧架构可切换
- [ ] 所有现有测试通过

---

#### 7.2 迁移 StreamEngine

**文件**: [`internal/executor/stream_engine.go`](../internal/executor/stream_engine.go)

**修改内容**:

```go
// 修改前
type StreamEngine struct {
    machine *SessionMachine
    // ...
}

// 修改后
type StreamEngine struct {
    adapter *SessionAdapter  // 使用适配器
    // ...
}

// processChunk 修改
func (e *StreamEngine) processChunk(chunk string) []Action {
    return e.adapter.Feed(chunk)  // 通过适配器调用
}
```

**验收标准**:

- [ ] StreamEngine 使用适配器
- [ ] 功能与修改前一致
- [ ] 所有测试通过

---

#### 7.3 灰度切换

**任务**: 添加运行时开关，支持新旧架构切换

**修改文件**: `internal/config/runtime_config.go`

```go
type RuntimeConfig struct {
    // ...

    // UseNewSessionArchitecture 使用新会话架构
    // true: 使用 SessionDetector + SessionReducer + SessionDriver
    // false: 使用 SessionMachine（默认）
    UseNewSessionArchitecture bool
}
```

**验收标准**:

- [ ] 配置项添加完成
- [ ] 可通过配置切换架构
- [ ] 默认使用旧架构（安全）

---

#### 7.4 新架构集成测试

**新建文件**: `internal/executor/session_integration_test.go`

**测试场景**:

| 测试名称                             | 场景         | 验证点         |
| ------------------------------------ | ------------ | -------------- |
| `TestIntegration_InitToReady`        | 初始化到就绪 | 状态迁移正确   |
| `TestIntegration_CommandExecution`   | 命令执行     | 输出收集正确   |
| `TestIntegration_PaginationHandling` | 分页处理     | 自动续页       |
| `TestIntegration_ErrorHandling`      | 错误处理     | 挂起/继续/中止 |
| `TestIntegration_MultipleCommands`   | 多命令执行   | 队列消费正确   |

**验收标准**:

- [ ] 所有集成测试通过
- [ ] 新旧架构结果一致

---

### Phase 8: 清理旧代码

**目标**: 删除冗余的 flag、状态和 handler

**依赖**: Phase 7 完成，新架构稳定运行

#### 8.1 删除旧 flag

**文件**: [`internal/executor/session_machine.go`](../internal/executor/session_machine.go)

**删除内容**:

```go
// 第 64-71 行 - 删除以下字段
errorDecided bool      // ❌ 删除
errorContinue bool     // ❌ 删除
afterPager bool        // ❌ 删除
```

**替代方案**:

| 旧 flag         | 新机制                               |
| --------------- | ------------------------------------ |
| `afterPager`    | `NewStateAwaitPagerContinueAck` 状态 |
| `errorDecided`  | `NewStateSuspended` 状态             |
| `errorContinue` | `EvUserContinue` 事件                |

**验收标准**:

- [ ] 三个 flag 已删除
- [ ] 相关逻辑已迁移到状态机
- [ ] 测试通过

---

#### 8.2 删除旧状态枚举

**文件**: [`internal/executor/session_state.go`](../internal/executor/session_state.go)

**删除内容**:

```go
// 删除以下状态
StateSendCommand      // ❌ 删除 - 发送命令是动作，不是状态
StateHandlingPager    // ❌ 删除 - 用 NewStateAwaitPagerContinueAck 替代
StateHandlingError    // ❌ 删除 - 用 NewStateSuspended 替代
```

**验收标准**:

- [ ] 三个状态已删除
- [ ] 所有引用已更新
- [ ] 测试通过

---

#### 8.3 删除 PaginationPending 字段

**文件**: [`internal/executor/command_context.go`](../internal/executor/command_context.go)

**删除内容**:

```go
// 第 37-40 行 - 删除以下字段
PaginationPending bool  // ❌ 删除
```

**替代方案**: 分页状态由 `NewStateAwaitPagerContinueAck` 表达

**验收标准**:

- [ ] 字段已删除
- [ ] 相关方法已更新
- [ ] 测试通过

---

#### 8.4 清理旧 handler 方法

**文件**: [`internal/executor/session_machine.go`](../internal/executor/session_machine.go)

**保留**: `Feed()` 方法作为适配入口

**删除**: 以下 handler 内部逻辑迁移到 Reducer

```go
// 保留方法签名，内部委托给 Reducer
func (m *SessionMachine) handleReady() []Action {
    // 迁移后：调用 reducer.Reduce(EvActivePromptSeen{})
}

func (m *SessionMachine) handleCollecting() []Action {
    // 迁移后：调用 reducer.Reduce() 处理各种事件
}
```

**验收标准**:

- [ ] handler 逻辑已迁移
- [ ] SessionMachine 变为薄包装层
- [ ] 测试通过

---

### Phase 9: 最终收尾

**目标**: 完成文档更新，确保长期可维护

#### 9.1 更新状态迁移表

**文件**: [`docs/state-transition-table.md`](state-transition-table.md)

**更新内容**:

- 删除旧状态迁移路径
- 添加新状态迁移图
- 更新不变量规则

**验收标准**:

- [ ] 文档与代码一致
- [ ] 迁移图完整

---

#### 9.2 更新实施状态报告

**文件**: [`docs/pagination-race-fix-implementation-status.md`](pagination-race-fix-implementation-status.md)

**更新内容**:

- 标记 Phase 6 为已完成
- 添加 Phase 7-9 完成记录
- 更新架构图

**验收标准**:

- [ ] 状态报告准确
- [ ] 架构图更新

---

#### 9.3 运行完整回归测试

**命令**:

```powershell
# 运行所有测试
go test -race ./...

# 运行 Golden 测试
go test ./internal/executor -run TestGolden

# 运行不变量测试
go test ./internal/executor -run TestInvariant
```

**验收标准**:

- [ ] 所有测试通过
- [ ] 无竞态警告
- [ ] 无内存泄漏

---

## 三、任务清单汇总

### Phase 7: 架构迁移

| #   | 任务              | 文件                          | 状态 |
| --- | ----------------- | ----------------------------- | ---- |
| 7.1 | 创建适配层        | `session_adapter.go`          | [ ]  |
| 7.2 | 迁移 StreamEngine | `stream_engine.go`            | [ ]  |
| 7.3 | 灰度切换配置      | `runtime_config.go`           | [ ]  |
| 7.4 | 集成测试          | `session_integration_test.go` | [ ]  |

### Phase 8: 清理旧代码

| #   | 任务                   | 文件                 | 状态 |
| --- | ---------------------- | -------------------- | ---- |
| 8.1 | 删除旧 flag            | `session_machine.go` | [ ]  |
| 8.2 | 删除旧状态枚举         | `session_state.go`   | [ ]  |
| 8.3 | 删除 PaginationPending | `command_context.go` | [ ]  |
| 8.4 | 清理旧 handler         | `session_machine.go` | [ ]  |

### Phase 9: 最终收尾

| #   | 任务             | 文件                                           | 状态 |
| --- | ---------------- | ---------------------------------------------- | ---- |
| 9.1 | 更新状态迁移表   | `state-transition-table.md`                    | [ ]  |
| 9.2 | 更新实施状态报告 | `pagination-race-fix-implementation-status.md` | [ ]  |
| 9.3 | 完整回归测试     | -                                              | [ ]  |

---

## 四、风险评估与缓解

| 风险             | 概率 | 影响 | 缓解措施           |
| ---------------- | ---- | ---- | ------------------ |
| 新架构行为不一致 | 中   | 高   | 灰度切换，A/B 测试 |
| 迁移过程中断生产 | 低   | 高   | 保留回滚能力       |
| 测试覆盖不足     | 中   | 中   | 增加集成测试       |
| 性能回退         | 低   | 中   | 添加性能基准测试   |

---

## 五、回滚方案

### 快速回滚

如果新架构出现问题，可通过以下方式快速回滚：

1. **配置回滚**: 设置 `UseNewSessionArchitecture = false`
2. **代码回滚**: 恢复 `StreamEngine` 直接使用 `SessionMachine`
3. **完整回滚**: Git revert 到迁移前版本

### 回滚验证

```powershell
# 验证回滚后功能正常
go test ./internal/executor -run TestGolden
```

---

## 六、预期成果

### 完成后架构

```
┌─────────────────────────────────────────────────────────────┐
│                      StreamEngine                            │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────┐ │
│  │ SessionDetector │→│ SessionReducer  │→│SessionDriver│ │
│  │ (事件检测)      │  │ (状态归约)      │  │ (动作执行)  │ │
│  └─────────────────┘  └─────────────────┘  └─────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

### 完成后状态

```
Phase 0-6: ████████████████████ 100%
Phase 7:   ████████████████████ 100%
Phase 8:   ████████████████████ 100%
Phase 9:   ████████████████████ 100%

总体完成度: ████████████████████ 100%
```

### 验收标准

| 标准           | 验证方式              |
| -------------- | --------------------- |
| 所有测试通过   | `go test -race ./...` |
| 新架构稳定运行 | 集成测试通过          |
| 无冗余代码     | 代码审查              |
| 文档与代码一致 | 文档审查              |
| 可回滚         | 回滚测试              |

---

## 七、实施建议

### 实施顺序

1. **先完成 Phase 7**: 架构迁移是核心，其他工作依赖于此
2. **再完成 Phase 8**: 清理工作需要新架构稳定后进行
3. **最后完成 Phase 9**: 文档更新确保长期可维护

### 实施原则

1. **渐进式迁移**: 每一步都可验证、可回滚
2. **保持兼容**: 迁移过程中保持 API 兼容
3. **测试驱动**: 每个阶段完成后运行完整测试
4. **文档同步**: 代码变更同步更新文档

### 预计工作量

| 阶段    | 任务数 | 复杂度 |
| ------- | ------ | ------ |
| Phase 7 | 4      | 高     |
| Phase 8 | 4      | 中     |
| Phase 9 | 3      | 低     |

---

## 附录：相关文件清单

### 需要修改的文件

| 文件                                   | 修改类型 | 阶段    |
| -------------------------------------- | -------- | ------- |
| `internal/executor/stream_engine.go`   | 重构     | Phase 7 |
| `internal/executor/session_machine.go` | 清理     | Phase 8 |
| `internal/executor/session_state.go`   | 删除     | Phase 8 |
| `internal/executor/command_context.go` | 删除字段 | Phase 8 |
| `internal/config/runtime_config.go`    | 添加配置 | Phase 7 |

### 需要新建的文件

| 文件                                            | 说明     | 阶段    |
| ----------------------------------------------- | -------- | ------- |
| `internal/executor/session_adapter.go`          | 适配层   | Phase 7 |
| `internal/executor/session_integration_test.go` | 集成测试 | Phase 7 |

### 需要更新的文档

| 文件                                                | 说明       | 阶段    |
| --------------------------------------------------- | ---------- | ------- |
| `docs/state-transition-table.md`                    | 状态迁移表 | Phase 9 |
| `docs/pagination-race-fix-implementation-status.md` | 实施状态   | Phase 9 |
