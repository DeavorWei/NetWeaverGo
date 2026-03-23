# 状态迁移表文档

## 概述

本文档记录当前会话状态模型及其向旧状态枚举的投影规则，用于验证重构后的单一实现。

---

## 当前状态枚举（旧）

| 状态                      | 说明               | 类型                   |
| ------------------------- | ------------------ | ---------------------- |
| `StateWaitInitialPrompt`  | 等待初始提示符     | 稳态                   |
| `StateWarmup`             | 预热状态           | 稳态                   |
| `StateReady`              | 就绪状态           | 稳态                   |
| `StateCollecting`         | 收集输出状态       | 稳态                   |
| `StateWaitingFinalPrompt` | 等待最终提示符状态 | 稳态                   |
| `StateCompleted`          | 完成状态           | 终态                   |
| `StateFailed`             | 失败状态           | 终态                   |

## 目标状态枚举（新）

| 状态                              | 说明                  | 类型 |
| --------------------------------- | --------------------- | ---- |
| `NewStateInitAwaitPrompt`         | 等待初始提示符        | 稳态 |
| `NewStateInitAwaitWarmupPrompt`   | 等待预热后提示符      | 稳态 |
| `NewStateReady`                   | 就绪状态              | 稳态 |
| `NewStateRunning`                 | 命令执行中            | 稳态 |
| `NewStateAwaitPagerContinueAck`   | 等待分页续页确认      | 稳态 |
| `NewStateAwaitFinalPromptConfirm` | 等待最终提示符确认    | 稳态 |
| `NewStateSuspended`               | 挂起状态（错误/超时） | 稳态 |
| `NewStateCompleted`               | 完成状态              | 终态 |
| `NewStateFailed`                  | 失败状态              | 终态 |

---

## 状态迁移规则

### 初始化阶段

```
[Start] → NewStateInitAwaitPrompt
         │
         │ EvInitPromptStable (检测到初始提示符)
         ▼
    NewStateInitAwaitWarmupPrompt
         │
         │ EvWarmupPromptSeen (检测到预热后提示符)
         ▼
       NewStateReady
```

### 命令执行阶段

```
    NewStateReady
         │
         │ 有更多命令 && pendingLines == 0
         │ → ActSendCommand
         ▼
     NewStateRunning
         │
         ├─ EvPagerSeen → NewStateAwaitPagerContinueAck
         │                    │
         │                    │ ActSendPagerContinue
         │                    ▼
         │               NewStateRunning (继续收集)
         │
         ├─ EvErrorMatched (严重) → NewStateSuspended
         │
         ├─ EvActivePromptSeen → NewStateReady (命令完成)
         │
         └─ EvTimeout → NewStateSuspended
```

### 分页处理

```
    NewStateRunning
         │
         │ EvPagerSeen (检测到分页符)
         ▼
  NewStateAwaitPagerContinueAck
         │
         │ 发送空格后自动回到
         ▼
     NewStateRunning
```

### 错误处理

```
    NewStateRunning
         │
         │ EvErrorMatched (严重错误)
         ▼
     NewStateSuspended
         │
         ├─ EvUserContinue → NewStateRunning (继续执行)
         │
         └─ EvUserAbort → NewStateFailed (中止)
```

### 终态

```
    NewStateRunning
         │
         │ 所有命令完成
         ▼
    NewStateCompleted

    任意状态
         │
         │ EvStreamClosed 或 不可恢复错误
         ▼
      NewStateFailed
```

---

## 不变量规则

### 1. pendingLines 阻塞规则

**规则**：`pendingLines > 0` 时不可发送新命令

**验证**：

```go
// 在 NewStateReady 状态下
if ctx.HasPendingLines() {
    // 不产生 ActSendCommand
    return nil
}
```

### 2. 单命令完成规则

**规则**：每个命令最多完成一次

**验证**：

```go
// 命令完成后从队列移除
if ctx.Current != nil && ctx.Current.IsComplete {
    // 不重复完成
}
```

### 3. 终态不可回退规则

**规则**：`Completed`/`Failed` 状态不接受任何事件

**验证**：

```go
if state.IsTerminal() {
    // 忽略所有事件
    return nil
}
```

### 4. 分页优先级规则

**规则**：分页符检测优先于提示符检测

**验证**：

```go
// 在处理行时
if matcher.IsPaginationPrompt(line) {
    // 先处理分页
    return EvPagerSeen
}
if matcher.IsPromptStrict(line) {
    // 后处理提示符
    return EvActivePromptSeen
}
```

---

## Flag 迁移映射

| 旧 Flag                     | 新状态/机制                                        |
| --------------------------- | -------------------------------------------------- |
| `afterPager`                | 已删除，分页后确认改由阶段状态与分页计数表达       |
| `errorDecided`              | 已删除，错误决策改由外部动作直接驱动               |
| `errorContinue`             | 已删除，继续执行不再通过挂起 flag 表达             |
| `current.PaginationPending` | 已删除，改由状态机阶段表达                         |

---

## 状态迁移日志格式

```
[SessionReducer] [状态迁移] WaitInitialPrompt → InitAwaitWarmupPrompt (事件: EvInitPromptStable)
[SessionReducer] [动作产生] ActSendWarmup
[SessionReducer] [状态迁移] InitAwaitWarmupPrompt → Ready (事件: EvWarmupPromptSeen)
[SessionReducer] [动作产生] ActSendCommand {Index: 0, Command: "display version"}
[SessionReducer] [状态迁移] Ready → Running (事件: EvCommandSent)
```

---

## 测试覆盖

### 单元测试场景

1. **初始化流程**
   - `TestReducerInitPromptToWarmup`
   - `TestReducerWarmupToReady`

2. **命令发送**
   - `TestReducerSendCommand`
   - `TestReducerPendingLinesBlocksCommand`

3. **分页处理**
   - `TestReducerPagerSeen`

4. **错误处理**
   - `TestReducerErrorMatched`
   - `TestReducerWarningPass`
   - `TestReducerUserContinue`
   - `TestReducerUserAbort`

5. **终态**
   - `TestReducerTerminalState`
   - `TestReducerAllCommandsComplete`

### 验收测试场景

1. **分页不丢上下文**
   - `TestAcceptancePaginationNoContextLoss`

2. **连续分页**
   - `TestAcceptanceMultiplePaginationSequence`

3. **回车覆盖**
   - `TestAcceptanceCarriageReturnOverwrite`

---

## 重构检查清单

- [x] 删除 `StateSendCommand` 状态
- [x] 删除 `StateHandlingPager` 状态
- [x] 删除 `StateHandlingError` 状态
- [x] 删除 `afterPager` flag
- [x] 删除 `errorDecided`/`errorContinue` flag
- [x] 删除 `current.PaginationPending` 字段
- [x] 更新所有状态迁移代码
- [x] 所有测试通过
