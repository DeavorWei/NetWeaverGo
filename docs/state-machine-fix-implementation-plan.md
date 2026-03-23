# NetWeaverGo 状态机修复实施方案

## 文档信息
- **生成时间**: 2026-03-23
- **基于分析**: state-machine-final-analysis.md
- **核查方式**: 基于当前代码实现复核并直接回写本文档
- **实施优先级**: 高风险 > 中风险 > 低风险

---

## 一、修复问题总览

| 编号 | 问题 | 风险等级 | 影响范围 | 修复复杂度 | 核查状态 |
|------|------|----------|----------|------------|----------|
| FIX-001 | SuspendManager Channel Panic | 🔴 高 | 全局 | 低 | ✅ 已实施 |
| FIX-002 | 防串台门禁阻塞（条件性） | 🟡 中 | SessionAdapter / SessionReducer | 高 | ⏸ 需先修运行态事件接线 |
| FIX-003 | 分页循环无上限（条件性） | 🟡 中 | SessionReducer | 低 | ⚠️ 接线后再实施 |
| FIX-004 | AwaitFinalPromptConfirm 状态卡死 | 🟢 低 | SessionReducer | - | ❌ 当前实现态不成立 |
| FIX-005 | 挂起状态无超时感知 | 🟡 中 | SessionReducer | 中 | ⚠️ 需改进方案 |
| FIX-006 | 状态转换遗漏处理 | 🟢 低 | SessionReducer | - | ❌ 当前实现态不成立 |
| FIX-007 | StatePaused 未实现 | 🟢 低 | EngineStateManager | 低 | ✅ 可直接实施 |

---

## 一点五、前置说明

经复核，当前运行态的关键前提与原文档不同：

1. [`SessionAdapter.FeedSessionActions()`](internal/executor/session_adapter.go:41) 当前只调用 [`SessionDetector.DetectFromChunk(chunk)`](internal/executor/session_detector.go:91)
2. `DetectFromChunk(chunk)` 当前只产出初始化阶段事件，不产出 `EvCommittedLine` / `EvPagerSeen` / `EvActivePromptSeen`
3. 因此 FIX-002 / FIX-003 / FIX-004 的大部分触发路径，目前只存在于“设计态”，并未在真实运行链中接通

这意味着修复优先级需要调整：

- 第一优先级：修复 `SessionAdapter -> SessionDetector -> SessionReducer` 的运行态事件接线
- 第二优先级：在接线完成后，再重新验证门禁阻塞和分页上限问题
- FIX-004 / FIX-006 暂不应作为独立修复项实施

## 二、详细修复方案

### FIX-001: SuspendManager Channel Panic

> 状态：已实施

#### 问题描述
在 [`SuspendManager.Resolve()`](internal/ui/suspend_manager.go:136) 方法中，当超时发生后，`actionCh` 在 defer 中被关闭。如果前端在超时后、关闭前响应，向已关闭 channel 发送会导致 panic。

#### 问题根因分析

```
时序问题：
t=0:  超时触发，session.timedOut.Store(true)
t=1:  返回 ActionAbort
t=2:  defer 执行，close(actionCh)
t=3:  前端收到超时事件，但用户已点击响应
t=4:  Resolve() 被调用，检查 timedOut 通过（已设为 true）
t=5:  尝试向 actionCh 发送 -> 可能 panic
```

当前代码已有 `timedOut` 和 `resolved` 标记保护，但存在竞态窗口。

#### 实际修复方案

最终没有采用 `recover` 作为主修复，而是调整了 `SuspendManager` 的会话结束协议：

1. 不再通过 `close(actionCh)` 表示会话结束
2. 会话结束时只做 map 注销，并设置 `finished` 标记
3. `Resolve()` 在发送前检查 `finished / timedOut / resolved`
4. 新会话顶替旧会话时，也只对“未结束且未处理”的旧会话尝试发送 `ActionAbort`

这样修复后，`Resolve()` 即使拿到旧会话指针，也不会再出现“向已关闭 channel 发送”的 panic。

#### 测试验证

```go
go test ./internal/ui ./internal/executor
```

---

### FIX-002: 防串台门禁阻塞

#### 复核结论

- 该问题在 `SessionReducer` 代码层面成立
- 但当前实现态并未满足触发前提，因为 [`SessionAdapter.FeedSessionActions()`](internal/executor/session_adapter.go:41) 目前没有把运行态 `EvCommittedLine` 持续送入 `Reducer`
- 因此该项应从“直接实施”改为“前置问题修复后再验证”

#### 为什么原方案暂不应实施

原方案直接在 `Reducer` 层增加等待超时与 `ClearPendingLines()`，存在两个问题：

1. 它没有修掉真正的前置缺陷：运行态事件链未接通
2. `ClearPendingLines()` 可能直接丢弃设备输出，带来新的日志与错误检测缺口

#### 正确实施顺序

1. 先把 [`SessionAdapter`](internal/executor/session_adapter.go:41) 改为基于 `replayer` 的规范化结果调用 [`SessionDetector.Detect(lines, activeLine)`](internal/executor/session_detector.go:42)
2. 增加集成测试，确认 `EvCommittedLine` 在真实命令执行路径中能持续进入 `Reducer`
3. 接线完成后，再评估是否需要：
   - 命令等待发送超时
   - 有界 `PendingLines`
   - 严格的数据保留策略（不能直接清空）

#### 当前建议

- 本项暂缓实施
- 将其转为“运行态事件接线完成后的二次验证项”

---

### FIX-003: 分页循环无上限

#### 复核结论

- 该问题在 `SessionReducer.handlePagerSeen()` 层面成立
- 但它同样依赖运行态 `EvPagerSeen` 事件真正进入 `Reducer`
- 当前 [`SessionAdapter`](internal/executor/session_adapter.go:41) 尚未接通这条事件链，因此应视为“接线完成后必须补上的保护”，不是当前第一优先级

#### 修复方案评估

原方案的核心思路是正确的：

- 为分页次数设置上限
- 上限超出后中止会话，避免无限发送空格

这个方案可以保留，但实施顺序需要后移到“运行态事件接线修复”之后。

#### 建议落地方式

1. 在 [`SessionContext`](internal/executor/session_types.go:259) 增加 `MaxPaginationCount`
2. 在 [`handlePagerSeen()`](internal/executor/session_reducer.go:143) 中检查 `PaginationCount`
3. 为默认值和自定义值分别补测试

#### 当前建议

- 保留本项
- 但明确其前置条件为：`SessionAdapter -> Detector -> Reducer` 的分页事件链已接通

---

### FIX-004: AwaitFinalPromptConfirm 状态卡死

#### 复核结论

- 本项不应继续作为当前修复计划的一部分
- 原报告把它描述为“状态永远卡死”，这一点与当前实现态不符

原因有两点：

1. 进入 `AwaitFinalPromptConfirm` 需要 `EvPagerSeen -> EvActivePromptSeen` 这条运行态事件链，而这条链当前没有真正从 `SessionAdapter` 驱动
2. 即使未来该状态可达，外层 [`StreamEngine`](internal/executor/stream_engine.go:184) 也有读取超时兜底，因此不会无限期停留而完全无退出路径

#### 对原方案的评估

原方案的问题不只是复杂，而是与当前代码脱节：

- 方案中引用的 `CheckStateTimeout()`、`a.newReducer`、`ExecutePlaybook()` 等接口在当前代码中并不存在
- 即便强行补齐，也是在未证实真实故障前引入一套额外的状态超时子系统

#### 当前建议

- 从立即修复清单中移除本项
- 等运行态事件接线修复后，如果真实设备日志证明该状态会长时间停留，再单独立项分析

---

### FIX-005: 挂起状态无超时感知

#### 问题描述
当检测到严重错误时，状态机进入 `Suspended` 状态，需要 `SuspendManager` 外部响应才能继续。虽然 `SuspendManager` 有 5 分钟超时，但 Reducer 层面无感知。

#### ⚠️ 核查发现的新问题

**问题: 事件传递机制缺失**

原修复方案中 `SuspendManager` 发送的是前端事件 `engine:suspend_timeout`，但没有机制将 `EvSuspendTimeout` 传递到 Reducer。

#### 改进后的修复方案

**方案: 添加挂起超时事件 + 事件传递机制**

修改文件: [`internal/executor/session_types.go`](internal/executor/session_types.go:78)

```go
// EvSuspendTimeout 挂起超时事件
type EvSuspendTimeout struct {
    CommandIndex int
    Reason       string  // 【新增】超时原因
}

func (e EvSuspendTimeout) EventType() string { return "SuspendTimeout" }
```

修改文件: [`internal/executor/session_reducer.go`](internal/executor/session_reducer.go:51)

```go
// Reduce 状态迁移核心函数
func (r *SessionReducer) Reduce(event SessionEvent) []SessionAction {
    // 终态不处理任何事件
    if r.state.IsTerminal() {
        return nil
    }

    switch e := event.(type) {
    // ... 现有事件处理 ...

    case EvSuspendTimeout:
        return r.handleSuspendTimeout(e)

    default:
        logger.Debug("SessionReducer", "-", "未知事件类型: %s", event.EventType())
        return nil
    }
}

// handleSuspendTimeout 处理挂起超时事件
func (r *SessionReducer) handleSuspendTimeout(e EvSuspendTimeout) []SessionAction {
    if r.state != NewStateSuspended {
        return nil
    }

    logger.Warn("SessionReducer", "-", "挂起超时（%s），自动中止", e.Reason)
    r.ctx.FailCurrentCommand("挂起超时: " + e.Reason)
    r.state = NewStateFailed

    return []SessionAction{ActAbortSession{Reason: "suspend_timeout: " + e.Reason}}
}
```

修改文件: [`internal/ui/suspend_manager.go`](internal/ui/suspend_manager.go:116)

```go
// SuspendHandler 返回类型需要携带超时信息
type SuspendResult struct {
    Action    executor.ErrorAction
    TimedOut  bool
}

// CreateHandler 创建 SuspendHandler（供 Engine/TaskGroupService 使用）
func (m *SuspendManager) CreateHandler() executor.SuspendHandler {
    return func(ctx context.Context, ip string, logLine string, cmd string) executor.ErrorAction {
        sessionID := m.generateSessionID()
        actionCh := make(chan executor.ErrorAction, 1)

        session := &SuspendSession{
            ID:        sessionID,
            IP:        ip,
            CreatedAt: time.Now(),
            ActionCh:  actionCh,
        }

        m.mu.Lock()
        // 清理该 IP 的旧会话
        if oldSessionID, exists := m.sessionsByIP[ip]; exists {
            if oldSession, ok := m.sessions[oldSessionID]; ok {
                select {
                case oldSession.ActionCh <- executor.ActionAbort:
                    logger.Debug("SuspendManager", ip, "旧的挂起会话 %s 已被终止", oldSessionID)
                default:
                }
            }
        }
        m.sessions[sessionID] = session
        m.sessionsByIP[ip] = sessionID
        app := m.wailsApp
        m.mu.Unlock()

        defer func() {
            m.mu.Lock()
            delete(m.sessions, sessionID)
            if m.sessionsByIP[ip] == sessionID {
                delete(m.sessionsByIP, ip)
            }
            m.mu.Unlock()
            close(actionCh)
        }()

        // 发射事件到前端
        if app != nil {
            app.Event.Emit("engine:suspend_required", map[string]interface{}{
                "sessionId": sessionID,
                "ip":        ip,
                "error":     logLine,
                "command":   cmd,
            })
        }

        logger.Warn("SuspendManager", ip, "挂起会话创建 (sessionID: %s)，等待用户操作...", sessionID)

        select {
        case action := <-actionCh:
            logger.Debug("SuspendManager", ip, "挂起会话 %s 已收到用户响应", sessionID)
            return action
        case <-ctx.Done():
            logger.Warn("SuspendManager", ip, "引擎任务结束，强制释放挂起的会话 (sessionID: %s)", sessionID)
            return executor.ActionAbort
        case <-time.After(5 * time.Minute):
            // 设置超时标记，防止前端后续响应
            session.timedOut.Store(true)

            // 发射超时事件到前端
            if app != nil {
                app.Event.Emit("engine:suspend_timeout", map[string]interface{}{
                    "sessionId": sessionID,
                    "ip":        ip,
                    "message":   "挂起超时（5分钟），已自动终止设备连接",
                })
            }

            logger.Warn("SuspendManager", ip, "挂起超时（5分钟），自动 Abort")
            return executor.ActionAbort
        }
    }
}
```

修改文件: [`internal/executor/stream_engine.go`](internal/executor/stream_engine.go)

```go
// ExecutePlaybook 执行命令剧本
func (e *StreamEngine) ExecutePlaybook(ctx context.Context, ip string, commands []string) error {
    // ... 现有代码 ...
    
    // 当 SuspendHandler 返回时，检查是否是超时
    action := suspendHandler(ctx, ip, logLine, cmd)
    if action == executor.ActionAbort {
        // 【新增】检查是否是超时导致的 Abort
        if session.TimedOut() {
            // 发送超时事件到 Reducer
            e.adapter.Reduce(executor.EvSuspendTimeout{
                CommandIndex: e.adapter.Context().NextIndex - 1,
                Reason:       "5分钟超时",
            })
        }
    }
    
    // ... 现有代码 ...
}
```

#### 测试验证

```go
func TestSessionReducer_SuspendTimeout(t *testing.T) {
    reducer := NewSessionReducer([]string{"show run"}, &mockMatcher{})
    
    // 进入 Running 状态
    reducer.Reduce(EvInitPromptStable{Prompt: "<test>"})
    reducer.Reduce(EvWarmupPromptSeen{Prompt: "<test>"})
    
    // 模拟错误导致挂起
    reducer.Reduce(EvErrorMatched{
        Line: "Error: test error",
        Rule: &matcher.ErrorRule{Severity: matcher.SeverityError},
    })
    assert.Equal(t, NewStateSuspended, reducer.State())
    
    // 发送超时事件
    actions := reducer.Reduce(EvSuspendTimeout{
        CommandIndex: 0,
        Reason:       "5分钟超时",
    })
    
    // 应该进入失败状态
    assert.Equal(t, NewStateFailed, reducer.State())
    assert.IsType(t, ActAbortSession{}, actions[0])
}
```

---

### FIX-006: 状态转换遗漏处理

#### 复核结论

本项经复核后不成立，不应继续作为独立修复项。

#### 逐条说明

1. **Ready 状态收到 `EvCommittedLine` 会丢行**
   不成立。[`handleCommittedLine()`](internal/executor/session_reducer.go:131) 会先执行 `AddPendingLine(e.Line)`，行已被保留
2. **Suspended 状态收到 `EvStreamClosed` 未处理**
   不成立。[`handleStreamClosed()`](internal/executor/session_reducer.go:250) 对所有非终态统一转为 `Failed`
3. **AwaitPagerContinueAck 状态收到 `EvErrorMatched` 分页状态未清理**
   不成立。当前不存在需要额外复位的分页 flag；进入 `Suspended` 后逻辑状态已足够明确

#### 当前建议

- 将 FIX-006 从修复计划中移除
- 不做代码改动
- 若后续接线修复后出现新的真实状态不一致，再按具体场景重新立项

---

### FIX-007: StatePaused 未实现

#### 问题描述
状态矩阵允许 `Running→Paused` 转移，但引擎层面无暂停/恢复实现。这是一个预留状态，实际不会触发。

#### 修复方案

**方案A: 移除未使用的状态（推荐）**

修改文件: [`internal/engine/engine_state.go`](internal/engine/engine_state.go:11)

```go
const (
    StateIdle EngineState = iota
    StateStarting
    StateRunning
    // StatePaused  // 【修复】移除未实现的状态
    StateClosing
    StateClosed
)
```

同时更新状态转移矩阵：

```go
var stateTransitionMatrix = map[EngineState]map[EngineState]bool{
    StateIdle: {
        StateStarting: true,
        StateClosing:  true,
    },
    StateStarting: {
        StateRunning: true,
        StateClosing: true,
    },
    StateRunning: {
        // StatePaused: true,  // 【修复】移除
        StateClosing: true,
    },
    // StatePaused: {  // 【修复】移除
    //     StateRunning: true,
    //     StateClosing: true,
    // },
    StateClosing: {
        StateClosed: true,
    },
    StateClosed: {}, // 终态，不可转移
}
```

同时更新 `String()` 方法：

```go
func (s EngineState) String() string {
    switch s {
    case StateIdle:
        return "Idle"
    case StateStarting:
        return "Starting"
    case StateRunning:
        return "Running"
    // StatePaused 已移除
    case StateClosing:
        return "Closing"
    case StateClosed:
        return "Closed"
    default:
        return "Unknown"
    }
}
```

#### 测试验证

```go
func TestEngineState_NoPausedState(t *testing.T) {
    sm := NewEngineStateManager()
    
    // 验证状态转移
    err := sm.TransitionTo(StateStarting)
    assert.NoError(t, err)
    
    err = sm.TransitionTo(StateRunning)
    assert.NoError(t, err)
    
    // StatePaused 应该不存在
    // 验证状态转移矩阵中没有 StatePaused
    _, exists := stateTransitionMatrix[StateRunning][StatePaused]
    assert.False(t, exists, "StatePaused should not exist in transition matrix")
}
```

---

## 三、实施计划

### 阶段一：已完成

| 任务 | 文件 | 结果 |
|------|------|------|
| FIX-001 | `internal/ui/suspend_manager.go`, `internal/ui/suspend_manager_test.go` | 已通过调整会话结束协议修复 |

### 阶段二：前置修复

| 任务 | 文件 | 目标 |
|------|------|------|
| PRE-001 | `internal/executor/session_adapter.go`, `internal/executor/session_detector.go` | 接通运行态 `Detect(lines, activeLine)` 事件链 |

### 阶段三：接线完成后二次验证

| 任务 | 文件 | 当前结论 |
|------|------|----------|
| FIX-002 | `internal/executor/session_reducer.go`, `internal/executor/session_types.go` | 条件性风险，待 PRE-001 完成后验证 |
| FIX-003 | `internal/executor/session_reducer.go`, `internal/executor/session_types.go` | 条件性风险，待 PRE-001 完成后实施 |
| FIX-005 | `internal/executor/session_reducer.go`, `internal/executor/stream_engine.go` | 暂保留，需独立复核 |

### 阶段四：不进入当前实施范围

| 任务 | 原因 |
|------|------|
| FIX-004 | 当前实现态不成立，且原方案与现有代码脱节 |
| FIX-006 | 复核后为文档误判，不构成独立修复项 |

### 阶段五：低优先级清理

| 任务 | 文件 | 当前结论 |
|------|------|----------|
| FIX-007 | `internal/engine/engine_state.go` | 可在后续清理预留状态时处理 |

---

## 四、测试策略

### 单元测试

当前应优先补以下测试：

1. `internal/ui/suspend_manager_test.go`
   验证会话结束与 `Resolve()` 并发时无 panic
2. `internal/executor/session_adapter_test.go`
   验证 `FeedSessionActions()` 会把运行态 `lines / activeLine` 转成 `EvCommittedLine / EvPagerSeen / EvActivePromptSeen`
3. `internal/executor/session_reducer_test.go`
   在 PRE-001 完成后，再增加 FIX-002 / FIX-003 的真实触发测试

### 集成测试

建议优先新增以下回归场景：

| 场景 | 目标 |
|------|------|
| `issue_001_channel_panic` | 验证挂起结束与前端响应并发无 panic |
| `runtime_event_wiring` | 验证运行态分页、提示符、提交行事件能进入 Reducer |
| `issue_002_pending_lines` | 在真实事件链接通后验证门禁阻塞是否可复现 |
| `issue_003_pagination_limit` | 在真实事件链接通后验证分页上限保护 |

### 回归测试

```bash
go test ./internal/executor/... -v
go test ./internal/ui/... -v
go test ./internal/engine/... -v
```

---

## 五、监控增强建议

现阶段不建议先引入状态超时或健康检查大改，建议先做两类轻量监控：

1. 记录 `SessionAdapter` 每次实际送入 `Reducer` 的事件类型分布
2. 记录 `SessionReducer` 的 `state / pendingLines / current pagination count`

示例：

```go
logger.Debug("SessionAdapter", "-", "runtime events: %v", protocolEvents)
logger.Debug("SessionReducer", "-", "state=%s pending=%d", r.state, len(r.ctx.PendingLines))
```

---

## 六、风险评估

| 项目 | 当前风险 | 备注 |
|------|----------|------|
| FIX-001 | 低 | 已落地并通过并发测试 |
| PRE-001 | 中 | 是 FIX-002 / FIX-003 / FIX-004 复核的前置条件 |
| FIX-002 | 中 | 仅为条件性风险，且原方案存在丢数据副作用 |
| FIX-003 | 低 | 方案本身可行，但应晚于 PRE-001 |
| FIX-004 | 低 | 当前实现态不成立，暂不实施 |
| FIX-006 | 低 | 已确认为文档误判 |
| FIX-007 | 低 | 纯清理项 |

---

## 七、附录：修改文件清单

| 文件 | 修改类型 | 说明 |
|------|----------|------|
| `internal/ui/suspend_manager.go` | 已修改 | FIX-001 主修复 |
| `internal/ui/suspend_manager_test.go` | 已新增 | FIX-001 并发回归测试 |
| `internal/executor/session_adapter.go` | 待修改 | PRE-001 运行态事件接线 |
| `internal/executor/session_detector.go` | 待修改/复核 | PRE-001 运行态事件检测接入点 |
| `internal/executor/session_reducer.go` | 待修改 | FIX-002 / FIX-003（前提是 PRE-001 完成） |
| `internal/executor/session_types.go` | 待修改 | FIX-003 及可能的后续上下文字段 |
| `internal/engine/engine_state.go` | 待修改 | FIX-007 清理预留状态 |

---

## 八、核查报告参考

本次核查结论已直接并入本文档与 [`state-machine-final-analysis.md`](state-machine-final-analysis.md)。

---

*文档结束*
