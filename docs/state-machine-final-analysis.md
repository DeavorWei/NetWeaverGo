# NetWeaverGo 状态机架构最终分析报告

## 文档信息
- **生成时间**: 2026-03-23
- **分析范围**: 全项目状态机运行逻辑
- **分析目标**: 识别调用异常、卡死风险、异常调用节点
- **整合来源**: state-machine-analysis.md + state-machine-detailed-analysis.md

---

## 一、状态机总体架构

项目中共存在以下独立的状态机系统：

```mermaid
flowchart TB
    subgraph 顶层状态机
        E[EngineStateManager<br/>引擎生命周期状态]
        G[GlobalEngineState<br/>全局引擎状态]
    end
    
    subgraph 会话层状态机
        S[SessionReducer<br/>命令执行会话状态]
        A[SessionAdapter<br/>会话适配器]
    end
    
    subgraph 追踪层状态机
        P[ProgressTracker<br/>进度追踪状态]
    end
    
    subgraph 挂起管理层
        U[SuspendManager<br/>异常挂起管理]
    end
    
    subgraph 发现任务状态机
        D[Discovery Runner<br/>发现任务状态]
    end
    
    E --> G
    E --> S
    S --> A
    A --> P
    S --> U
    D -.独立.-> D
```

### 1.1 组件职责总览

| 组件 | 文件 | 职责 |
|------|------|------|
| [`EngineStateManager`](internal/engine/engine_state.go:11) | engine_state.go | 引擎生命周期状态管理 |
| [`GlobalEngineState`](internal/engine/global_state.go) | global_state.go | 全局引擎状态单例 |
| [`SessionReducer`](internal/executor/session_reducer.go:15) | session_reducer.go | 纯函数式会话状态机 |
| [`SessionAdapter`](internal/executor/session_adapter.go:13) | session_adapter.go | 统一封装 Replayer+Detector+Reducer |
| [`ProgressTracker`](internal/report/collector.go:62) | collector.go | 进度追踪状态管理 |
| [`SuspendManager`](internal/ui/suspend_manager.go:26) | suspend_manager.go | 异常挂起会话管理 |
| [`Discovery Runner`](internal/discovery/runner.go:57) | runner.go | 发现任务状态管理 |

---

## 二、EngineStateManager 引擎顶层状态机

### 2.1 状态定义

**文件位置**: [`internal/engine/engine_state.go:11-18`](internal/engine/engine_state.go:11)

```go
type EngineState int

const (
    StateIdle EngineState = iota      // 0 - 空闲
    StateStarting                      // 1 - 启动中
    StateRunning                       // 2 - 运行中
    StatePaused                        // 3 - 已暂停（预留但未实现）
    StateClosing                       // 4 - 关闭中
    StateClosed                        // 5 - 已关闭
)
```

### 2.2 状态转移矩阵

**文件位置**: [`internal/engine/engine_state.go:42-63`](internal/engine/engine_state.go:42)

```
当前状态        | 可转移目标状态
----------------|----------------------------------
StateIdle       | StateStarting, StateClosing
StateStarting   | StateRunning, StateClosing
StateRunning    | StatePaused, StateClosing
StatePaused     | StateRunning, StateClosing
StateClosing    | StateClosed
StateClosed     | (终态，无转移)
```

### 2.3 状态转换调用链

```mermaid
flowchart LR
    A[StateIdle] -->|TransitionTo| B[StateStarting]
    B -->|TransitionTo| C[StateRunning]
    C -->|gracefulClose| D[StateClosing]
    D -->|TransitionTo| E[StateClosed]
```

**调用位置详细追踪**:

| 调用点 | 文件位置 | 状态转换 | 条件/触发 |
|--------|----------|----------|-----------|
| Run()入口 | `engine.go:238` | Idle→Starting | Run()方法入口 |
| Run()初始化完成 | `engine.go:246` | Starting→Running | 初始化成功 |
| Run()正常结束 | `engine.go:329` | Running→Closing | wg.Wait()完成 |
| Run()最终状态 | `engine.go:349` | Closing→Closed | 通道关闭后 |

---

## 三、SessionReducer 会话命令执行状态机

### 3.1 状态定义

**文件位置**: [`internal/executor/session_types.go:18-45`](internal/executor/session_types.go:18)

```go
type NewSessionState int

const (
    NewStateInitAwaitPrompt         // 等待初始提示符
    NewStateInitAwaitWarmupPrompt   // 等待预热后提示符
    NewStateReady                   // 就绪状态
    NewStateRunning                 // 命令执行中
    NewStateAwaitPagerContinueAck   // 等待分页续页确认
    NewStateAwaitFinalPromptConfirm // 等待最终提示符确认
    NewStateSuspended               // 挂起状态
    NewStateCompleted               // 完成状态
    NewStateFailed                  // 失败状态
)
```

### 3.2 状态分类

```
┌─────────────────────────────────────────────────────────────┐
│                      状态分类                                │
├─────────────────────────────────────────────────────────────┤
│  初始化阶段:                                                 │
│    - NewStateInitAwaitPrompt                                │
│    - NewStateInitAwaitWarmupPrompt                          │
├─────────────────────────────────────────────────────────────┤
│  运行阶段:                                                   │
│    - NewStateReady                                          │
│    - NewStateRunning                                        │
│    - NewStateAwaitPagerContinueAck                          │
│    - NewStateAwaitFinalPromptConfirm                        │
├─────────────────────────────────────────────────────────────┤
│  挂起阶段:                                                   │
│    - NewStateSuspended                                      │
├─────────────────────────────────────────────────────────────┤
│  终态:                                                       │
│    - NewStateCompleted                                      │
│    - NewStateFailed                                         │
└─────────────────────────────────────────────────────────────┘
```

### 3.3 状态转移图

```mermaid
stateDiagram-v2
    [*] --> InitAwaitPrompt: 创建 Reducer
    
    InitAwaitPrompt --> InitAwaitWarmupPrompt: EvInitPromptStable
    InitAwaitPrompt --> Failed: EvTimeout/EvStreamClosed
    
    InitAwaitWarmupPrompt --> Ready: EvWarmupPromptSeen
    InitAwaitWarmupPrompt --> Failed: EvTimeout/EvStreamClosed
    
    Ready --> Running: trySendCommand
    Ready --> Completed: 无更多命令
    Ready --> AwaitPagerContinueAck: EvPagerSeen
    
    Running --> Ready: 命令完成
    Running --> AwaitPagerContinueAck: EvPagerSeen
    Running --> Suspended: EvErrorMatched
    Running --> Failed: EvTimeout/EvStreamClosed
    
    AwaitPagerContinueAck --> AwaitFinalPromptConfirm: EvActivePromptSeen
    AwaitPagerContinueAck --> AwaitPagerContinueAck: EvPagerSeen
    AwaitPagerContinueAck --> Failed: EvTimeout/EvStreamClosed
    
    AwaitFinalPromptConfirm --> Ready: EvActivePromptSeen
    AwaitFinalPromptConfirm --> Failed: EvTimeout/EvStreamClosed
    
    Suspended --> Running: EvUserContinue
    Suspended --> Failed: EvUserAbort
    
    Completed --> [*]
    Failed --> [*]
```

### 3.4 事件类型详解

| 事件类型 | 触发条件 | 携带数据 |
|----------|----------|----------|
| `EvInitPromptStable` | 初始化阶段检测到稳定提示符 | Prompt string |
| `EvWarmupPromptSeen` | 预热后检测到提示符 | Prompt string |
| `EvCommandPromptSeen` | 命令完成后检测到提示符 | Prompt string |
| `EvCommittedLine` | 行被提交（换行符） | Line string |
| `EvActivePromptSeen` | 活动行检测到提示符 | Prompt string |
| `EvPagerSeen` | 检测到分页符 | Line string |
| `EvErrorMatched` | 检测到错误规则命中 | Line, Rule |
| `EvTimeout` | 命令执行超时 | CommandIndex |
| `EvUserContinue` | 用户选择继续 | CommandIndex |
| `EvUserAbort` | 用户选择中止 | CommandIndex |
| `EvStreamClosed` | 流关闭 | - |

### 3.5 动作类型列表

| 动作类型 | 执行操作 |
|----------|----------|
| `ActSendWarmup` | 发送预热空行 |
| `ActSendCommand` | 发送命令 |
| `ActSendPagerContinue` | 发送分页续页（空格） |
| `ActEmitCommandStart` | 发送命令开始事件 |
| `ActEmitCommandDone` | 发送命令完成事件 |
| `ActEmitDeviceError` | 发送设备错误事件 |
| `ActRequestSuspendDecision` | 请求挂起决策 |
| `ActAbortSession` | 中止会话 |
| `ActResetReadTimeout` | 重置读取超时 |
| `ActFlushDetailLog` | 刷新详细日志 |
| `ActClearInitResiduals` | 清理初始化残留 |

---

## 四、ProgressTracker 进度追踪状态机

### 4.1 状态定义

**文件位置**: [`internal/report/collector.go:62-79`](internal/report/collector.go:62)

ProgressTracker 通过以下字段追踪状态：

```go
type ProgressTracker struct {
    status      map[string]*DeviceSummary  // 各设备状态
    finished    int                        // 已完成设备数
    total       int                        // 总设备数
    paused      bool                       // 是否暂停刷新
}
```

**设备状态值** (`DeviceSummary.Status`):
- `"Init"` - 初始化
- `"Running"` - 执行中
- `"Success"` - 成功完成
- `"Error"` - 错误（但非终态，引擎会继续）
- `"Aborted"` - 中止（终态）
- `"Warning"` - 警告/跳过
- `"Suspended"` - 挂起等待用户决策

---

## 五、SuspendManager 异常挂起状态机

### 5.1 状态定义

**文件位置**: [`internal/ui/suspend_manager.go:15-23`](internal/ui/suspend_manager.go:15)

```go
type SuspendSession struct {
    ID        string
    IP        string
    CreatedAt time.Time
    ActionCh  chan executor.ErrorAction
    timedOut  atomic.Bool  // 超时标记
    resolved  atomic.Bool  // 已响应标记
}
```

状态通过以下组合隐式定义：
- `timedOut == false && resolved == false` - 等待中
- `resolved == true` - 已决策
- `timedOut == true` - 已超时

### 5.2 状态转移图

```mermaid
stateDiagram-v2
    [*] --> Waiting: CreateHandler调用
    Waiting --> Resolved: 用户响应/超时前
    Waiting --> TimedOut: 5分钟超时
    Resolved --> [*]: 返回Action
    TimedOut --> [*]: 返回ActionAbort
```

---

## 六、Discovery Runner 发现任务状态机

### 6.1 状态定义

**任务状态** (数据库字段):
- `pending` - 待执行
- `running` - 执行中
- `completed` - 完成
- `failed` - 失败
- `cancelled` - 已取消
- `partial` - 部分成功

### 6.2 状态转换调用链

```mermaid
flowchart LR
    A[pending] -->|Start| B[running]
    B -->|success| C[completed]
    B -->|failed| D[failed]
    B -->|Cancel| E[cancelled]
    B -->|partial| F[partial]
```

---

## 七、跨状态机交互调用链

### 7.1 完整执行流程调用链

```mermaid
sequenceDiagram
    participant UI as UI层
    participant ES as EngineService
    participant EM as ExecutionManager
    participant GE as GlobalEngineState
    participant E as Engine
    participant SM as StateManager
    participant W as Worker
    participant SE as StreamEngine
    participant SA as SessionAdapter
    participant SR as SessionReducer
    participant PT as ProgressTracker
    participant SuM as SuspendManager

    UI->>ES: StartEngineWithSelection()
    ES->>EM: RunEngineWithMeta()
    EM->>GE: SetActiveEngine()
    GE--xEM: 检查是否已在运行
    EM->>E: Run()
    E->>SM: TransitionTo(StateStarting)
    SM-->>E: OK
    E->>SM: TransitionTo(StateRunning)
    SM-->>E: OK
    
    par 启动多个 Worker
        E->>W: go worker()
        W->>SE: ExecutePlaybook()
        SE->>SA: RunInit()
        SA->>SR: Reduce(EvInitPromptStable)
        SR-->>SA: ActSendWarmup
        SE->>SA: RunPlaybook()
        loop 命令执行循环
            SA->>SR: Reduce(EvCommittedLine)
            SR-->>SA: ActSendCommand/ActSendPagerContinue
            alt 发生错误
                SR->>SR: NewStateSuspended
                SE->>SuM: CreateHandler()()
                SuM-->>SE: ActionContinue/ActionAbort
                SE->>SR: ResolveErrorActions()
            end
        end
        W->>PT: emitEvent(EventDeviceSuccess/Abort)
    end
    
    E->>SM: TransitionTo(StateClosing)
    E->>SM: TransitionTo(StateClosed)
    EM->>GE: ClearActiveEngine()
```

### 7.2 实现态校正（2026-03-23 复核）

> 本节用于修正文档前文的“设计态状态图”和“当前实现态”之间的偏差。

当前 `SessionAdapter` 的实际行为并不是“Replayer 产出规范化行 -> Detector.Detect(lines, activeLine) -> Reducer 处理运行态事件”，而是：

1. `replayer.Process(chunk)` 仅用于收集 `newCommittedLines` 供日志写入
2. `detector` 当前调用的是 `DetectFromChunk(chunk)`，而不是 `Detect(lines, activeLine)`
3. `DetectFromChunk(chunk)` 当前只会产出初始化阶段相关的 `EvInitPromptStable`

对应代码位置：
- [`internal/executor/session_adapter.go:41-60`](internal/executor/session_adapter.go:41)
- [`internal/executor/session_detector.go:42-89`](internal/executor/session_detector.go:42)
- [`internal/executor/session_detector.go:91-112`](internal/executor/session_detector.go:91)

这意味着：

- `EvCommittedLine` / `EvPagerSeen` / `EvActivePromptSeen` 这组运行态事件，在当前生产调用链中并未真正由 `SessionAdapter` 驱动
- 因此，后续对 `SessionReducer` 的部分风险判断只能视为“设计态/潜在风险”，不能直接当作“当前实现态已触发的问题”
- FIX-002 / FIX-003 / FIX-004 的优先级，都必须晚于“接通 Detector -> Reducer 运行态事件流”

---

## 八、风险点分析汇总

### 8.1 风险等级分类

| 等级 | 说明 | 数量 |
|------|------|------|
| 🔴 高 | 可能导致 panic 或严重功能故障 | 0 |
| 🟡 中 | 已确认的条件性/潜在功能风险 | 3 |
| 🟢 低 | 设计问题、误判项或轻微影响 | 5 |

### 8.2 已修复的高风险项

#### ✅ 已修复项1: SuspendManager Channel Panic

**原始位置**: [`internal/ui/suspend_manager.go`](internal/ui/suspend_manager.go)

**原问题描述**:
- 旧实现通过 `close(actionCh)` 表示会话结束
- `Resolve()` 在会话查找与发送之间已解锁，存在拿到旧会话指针后再向已关闭 channel 发送的竞态窗口

**已实施修复**:
- 不再通过关闭 `actionCh` 表示结束
- 会话结束时改为从 `sessions / sessionsByIP` 注销，并增加 `finished` 标记
- `Resolve()` 在发送前检查 `finished / timedOut / resolved`

**修复后结论**:
- 该高风险项已不再成立
- `recover` 不是主修复方案，主修复是调整会话生命周期协议

**历史时序问题（已消除）**:
```
t=0:  超时触发，返回 ActionAbort
t=1:  执行 defer，关闭 actionCh
t=2:  前端收到超时事件，但用户已点击响应
t=3:  Resolve() 被调用，尝试向已关闭 channel 发送 -> panic!
```

---

### 8.3 中风险点（含条件性风险）

#### 🟡 风险2: 防串台门禁可能导致命令发送阻塞

**位置**: [`internal/executor/session_reducer.go:280-285`](internal/executor/session_reducer.go:280)

**复核结论（2026-03-23）**:
- 在 `SessionReducer` 代码层面，这个风险是成立的
- 但在当前实现态中，`SessionAdapter` 并未把运行态 `EvCommittedLine` 事件稳定送入 `Reducer`
- 因此它目前更准确地属于“条件性潜在风险”，而不是已证实的生产故障

**代码片段**:
```go
// session_reducer.go:280-285
func (r *SessionReducer) trySendCommand() []SessionAction {
    if r.ctx.HasPendingLines() {
        logger.Debug("SessionReducer", "-", "防串台门禁：存在 %d 行未消费输出，禁止发送新命令", ...)
        return nil  // 不发送命令！
    }
    ...
}
```

**前置条件**:
- 需先把 [`SessionAdapter`](internal/executor/session_adapter.go:41) 改为使用 [`SessionDetector.Detect(lines, activeLine)`](internal/executor/session_detector.go:42)

**修复建议**:
- 先修运行态事件接线
- 接线完成后，再决定是否需要门禁超时或强制推进逻辑

---

#### 🟢 风险3: AwaitFinalPromptConfirm 状态卡死（当前实现态不成立）

**位置**: [`internal/executor/session_reducer.go:178-180`](internal/executor/session_reducer.go:178)

**复核结论（2026-03-23）**:
- 当前实现态下，这个问题不能作为已成立缺陷
- 原因一：进入 `AwaitFinalPromptConfirm` 依赖 `EvPagerSeen -> EvActivePromptSeen` 这条运行态事件链，而该事件链当前未真正接通
- 原因二：即使未来该状态可达，外层 [`StreamEngine`](internal/executor/stream_engine.go:184) 仍有读取超时兜底，因此“永远卡死”表述不准确

**代码片段**:
```go
// session_reducer.go:178-180
case NewStateAwaitFinalPromptConfirm:
    // 二次确认提示符，命令完成
    return r.completeCurrentCommand()
```

**处理建议**:
- 暂不作为独立修复项实施
- 待运行态事件接线修复后，再根据真实设备行为决定是否需要状态级超时回退

---

#### 🟡 风险4: 分页循环无上限

**位置**: [`internal/executor/session_reducer.go:143-163`](internal/executor/session_reducer.go:143)

**复核结论（2026-03-23）**:
- 在 `SessionReducer` 代码层面，这个风险成立
- 但与风险2相同，它依赖 `EvPagerSeen` 真正进入 `Reducer`
- 由于当前 `SessionAdapter` 尚未接通运行态分页事件，这属于“接线后需要优先补上的潜在风险”

**代码片段**:
```go
// session_reducer.go:143-163
func (r *SessionReducer) handlePagerSeen(e EvPagerSeen) []SessionAction {
    switch r.state {
    case NewStateRunning, NewStateReady, NewStateAwaitFinalPromptConfirm:
        r.state = NewStateAwaitPagerContinueAck
        return []SessionAction{ActSendPagerContinue{}}
    
    case NewStateAwaitPagerContinueAck:
        return []SessionAction{ActSendPagerContinue{}}
    }
    return nil
}
```

**建议修复**:
```go
const MaxPaginationCount = 100

func (r *SessionReducer) handlePagerSeen(e EvPagerSeen) []SessionAction {
    if r.ctx.Current.PaginationCount > MaxPaginationCount {
        r.state = NewStateFailed
        return []SessionAction{ActAbortSession{Reason: "pagination_limit_exceeded"}}
    }
    // ...
}
```

**前置条件**:
- 先修 [`SessionAdapter`](internal/executor/session_adapter.go:41) 对运行态事件的接线

---

#### 🟡 风险5: 挂起状态无超时机制

**位置**: [`internal/executor/session_reducer.go:186-209`](internal/executor/session_reducer.go:186)

**问题描述**:
- 当检测到严重错误时，状态机进入 `Suspended` 状态
- 需要 `SuspendManager` 外部响应才能继续
- 虽然 `SuspendManager` 有 5 分钟超时，但 Reducer 层面无感知

**建议修复**: 在 Reducer 中添加挂起超时状态追踪

---

### 8.4 低风险点

#### 🟢 风险6: StatePaused 状态未实现

**位置**: [`internal/engine/engine_state.go:15`](internal/engine/engine_state.go:15)

**问题描述**:
- 状态矩阵允许 Running→Paused 转移，但引擎层面无暂停/恢复实现
- 这是一个预留状态，实际不会触发

**建议**: 移除未使用的状态或实现暂停功能

---

#### 🟢 风险7: 提示符检测误判

**位置**: [`internal/matcher/matcher.go:192-242`](internal/matcher/matcher.go:192)

**问题描述**:
- 严格模式检测可能误判
- 华为格式 `<主机名>` 和普通文本 `<The current login time...>` 可能混淆

**现状**: 已有空格检查，但可能遗漏特殊情况

---

#### 🟢 风险8: 终态后事件处理

**位置**: [`internal/executor/session_reducer.go:52-55`](internal/executor/session_reducer.go:52)

**问题描述**:
- 终态后收到事件会被忽略
- 可能丢失重要信息

**现状**: 设计如此，终态不应处理事件

---

#### 🟢 风险9: Discovery 重试任务竞态

**位置**: [`internal/discovery/runner.go:239-291`](internal/discovery/runner.go:239)

**问题描述**:
- 重试任务复用原任务ID
- 如果此时有其他地方查询任务状态，可能看到不一致的状态

---

## 九、状态转换完整性检查

### 9.1 状态转换矩阵

| 当前状态 | EvInitPromptStable | EvWarmupPromptSeen | EvCommittedLine | EvPagerSeen | EvActivePromptSeen | EvErrorMatched | EvTimeout | EvUserContinue | EvUserAbort | EvStreamClosed |
|----------|-------------------|-------------------|-----------------|-------------|-------------------|----------------|-----------|----------------|-------------|----------------|
| InitAwaitPrompt | →InitAwaitWarmupPrompt | - | - | - | - | - | →Failed | - | - | →Failed |
| InitAwaitWarmupPrompt | - | →Ready | - | - | - | - | →Failed | - | - | →Failed |
| Ready | - | - | - | →AwaitPagerContinueAck | - | →Suspended | →Failed | - | - | →Failed |
| Running | - | - | 处理 | →AwaitPagerContinueAck | →Ready | →Suspended | →Failed | - | - | →Failed |
| AwaitPagerContinueAck | - | - | - | →AwaitPagerContinueAck | →AwaitFinalPromptConfirm | →Suspended | →Failed | - | - | →Failed |
| AwaitFinalPromptConfirm | - | - | - | →AwaitPagerContinueAck | →Ready | →Suspended | →Failed | - | - | →Failed |
| Suspended | - | - | - | - | - | - | - | →Running | →Failed | - |
| Completed | 忽略 | 忽略 | 忽略 | 忽略 | 忽略 | 忽略 | 忽略 | 忽略 | 忽略 | 忽略 |
| Failed | 忽略 | 忽略 | 忽略 | 忽略 | 忽略 | 忽略 | 忽略 | 忽略 | 忽略 | 忽略 |

### 9.2 未覆盖的状态转换

以下条目经复核后，需要区分“真实缺陷”和“文档误判”：

1. **Ready 状态收到 EvCommittedLine**
   当前并不会“丢行”，因为 [`handleCommittedLine()`](internal/executor/session_reducer.go:131) 会先把行写入 `PendingLines`
2. **Suspended 状态收到 EvStreamClosed**
   当前并非“未处理”，[`handleStreamClosed()`](internal/executor/session_reducer.go:250) 会统一将非终态推进到 `Failed`
3. **AwaitPagerContinueAck 状态收到 EvErrorMatched**
   当前也不构成独立缺陷，`Reducer` 会直接进入 `Suspended`；文档中提到的“分页状态未清理”目前没有额外状态字段需要复位

结论：
- FIX-006 中的 3 个子问题，在当前代码下均不足以构成独立修复项
- 真正需要优先修的是运行态事件链未接通，导致文档中的多个状态转换只存在于设计层面

---

## 十、安全防护机制确认

### 10.1 已确认的安全点

| 安全点 | 位置 | 说明 |
|--------|------|------|
| closeOnce 保护 | `engine.go:327-353` | 确保多次调用 gracefulClose 不会导致通道重复关闭 panic |
| 终态检查 | `session_reducer.go:52-55` | 终态不处理任何事件，防止状态混乱 |
| 警告级别放行 | `session_reducer.go:193-196` | 警告级别错误直接放行，避免不必要的挂起 |
| 终态重复计数防护 | `collector.go:356-366` | 通过 finishedIPs map 防止重复计数 |
| 死锁设备兜底 | `engine.go:537-544` | 执行链路异常退出时补发 Abort 事件 |
| 上下文传播 | `runner.go:196-208` | 取消信号正确传递到下层 |

---

## 十一、改进建议

### 11.1 高优先级改进

1. **接通 SessionAdapter 的运行态事件链**
   ```go
   // 目标方向：使用规范化后的 lines / activeLine 驱动 Detect(...)
   protocolEvents := a.detector.Detect(a.replayer.Lines(), a.replayer.ActiveLine())
   ```

### 11.2 中优先级改进

1. **在运行态事件接线完成后，再评估防串台门禁超时机制**
   ```go
   // 先验证 PendingLines 在真实链路中是否会持续积压
   ```

2. **在运行态事件接线完成后，为分页增加上限**
   ```go
   const MaxPaginationCount = 100
   ```

3. **完善状态转换日志**
   ```go
   func (r *SessionReducer) Reduce(event SessionEvent) []SessionAction {
       oldState := r.state
       // ... 状态转换
       logger.Debug("SessionReducer", "-", "状态转换: %s → %s (事件: %s)", 
           oldState, r.state, event.EventType())
   }
   ```

### 11.3 低优先级改进

1. **清理未使用的 StatePaused 状态**
2. **添加状态机健康检查接口**
3. **添加状态转换钩子机制**

---

## 十二、监控建议

建议添加以下监控点：

```go
// 1. 命令执行时间监控
if time.Since(cmdStartTime) > maxCmdDuration {
    logger.Warn("命令执行超时", ip, "cmd: %s", cmd)
}

// 2. 状态停留时间监控
if time.Since(stateEnterTime) > maxStateDuration {
    logger.Warn("状态停留过长", ip, "state: %s", state)
}

// 3. SuspendManager 会话生命周期监控
logger.Info("SuspendManager", ip, "会话创建到决策耗时: %v", decisionTime)
```

---

## 十三、总结

### 13.1 状态机设计总体评价

| 状态机 | 设计质量 | 主要问题 |
|--------|----------|----------|
| EngineStateManager | 良好 | StatePaused 未实现 |
| SessionAdapter | 需改进 | 运行态事件链未接通 |
| SessionReducer | 良好 | 若接线完成，需重新验证门禁与分页保护 |
| ProgressTracker | 良好 | 无显式问题 |
| SuspendManager | 良好 | channel panic 风险已修复 |
| Discovery Runner | 良好 | 重试竞态 |

### 13.2 优点

- 纯函数式状态转换，易于测试
- 事件驱动架构，解耦良好
- 完整的错误处理和挂起机制
- 多层安全防护机制

### 13.3 潜在风险

- SessionAdapter 运行态事件链未接通，导致文档中的部分状态转换仍停留在设计态
- 防串台门禁是条件性风险，需在接线修复后复核
- 分页循环无上限是条件性风险，需在接线修复后补保护
- AwaitFinalPromptConfirm 卡死与状态转换遗漏两项，经复核不构成当前独立缺陷

---

## 十四、附录: 关键代码路径速查

### 14.1 引擎状态机

| 功能 | 文件 | 行号 |
|------|------|------|
| 状态定义 | `internal/engine/engine_state.go` | 11-18 |
| 转移矩阵 | `internal/engine/engine_state.go` | 42-63 |
| 状态转换 | `internal/engine/engine_state.go` | 79-90 |
| Run()入口 | `internal/engine/engine.go` | 229-313 |
| 优雅关闭 | `internal/engine/engine.go` | 316-385 |

### 14.2 会话状态机

| 功能 | 文件 | 行号 |
|------|------|------|
| 状态定义 | `internal/executor/session_types.go` | 18-45 |
| Reducer主逻辑 | `internal/executor/session_reducer.go` | 48-95 |
| 事件处理器 | `internal/executor/session_reducer.go` | 100-373 |
| 会话适配器 | `internal/executor/session_adapter.go` | 13-206 |
| 事件检测器 | `internal/executor/session_detector.go` | 15-158 |

### 14.3 挂起管理

| 功能 | 文件 | 行号 |
|------|------|------|
| SuspendManager | `internal/ui/suspend_manager.go` | 26-204 |
| Handler创建 | `internal/ui/suspend_manager.go` | 59-132 |
| 响应处理 | `internal/ui/suspend_manager.go` | 136-189 |

### 14.4 进度追踪

| 功能 | 文件 | 行号 |
|------|------|------|
| ProgressTracker | `internal/report/collector.go` | 63-809 |
| 事件处理 | `internal/report/collector.go` | 164-236 |
| 终态防护 | `internal/report/collector.go` | 176-179 |

---

*报告结束*
