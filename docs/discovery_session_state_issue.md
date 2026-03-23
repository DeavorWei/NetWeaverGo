# 拓扑发现会话状态丢失问题分析

## 问题现象

从日志可以看到拓扑发现任务卡死：

```
[2026/03/23 22:45:41] [Debug] [SessionReducer] [-] 所有命令执行完成
[2026/03/23 22:46:01] [Debug] [StreamEngine] [-] 读取超时，当前状态: InitAwaitPrompt
[2026/03/23 22:46:01] [Debug] [SessionAdapter] [-] 标记失败: 读取超时
[2026/03/23 22:46:21] [Debug] [StreamEngine] [-] 读取超时，当前状态: InitAwaitPrompt
...
```

- 第一个命令 `display version` 执行成功
- 后续命令一直卡在 `InitAwaitPrompt` 状态超时

## 问题根因

### 核心问题：每次调用都创建新的 StreamEngine

[`ExecuteCommandSyncWithResult`](../internal/executor/executor.go:196) 的实现：

```go
func (e *DeviceExecutor) ExecuteCommandSyncWithResult(ctx context.Context, cmd string, timeout time.Duration) (*CommandResult, error) {
    if e.Client == nil {
        return nil, fmt.Errorf("执行器未安全建连")
    }

    // 问题所在：每次调用都创建新的 StreamEngine
    engine := NewStreamEngine(e, e.Client, []string{cmd}, 80)
    result, err := engine.RunSingle(ctx, cmd, timeout)
    if err != nil {
        return nil, err
    }

    return result, nil
}
```

### 状态丢失分析

每次创建 `StreamEngine` 时：

1. 创建新的 `SessionAdapter`
2. `SessionAdapter` 初始状态为 `NewStateInitAwaitPrompt`
3. `SessionAdapter` 内部创建新的 `SessionReducer` 和 `SessionContext`

**执行流程对比**：

| 步骤 | Engine (Playbook 模式) | Discovery Runner (单命令模式) |
| ---- | ---------------------- | ----------------------------- |
| 1    | 创建 1 个 StreamEngine | 创建第 1 个 StreamEngine      |
| 2    | RunInit() 等待提示符   | RunSingle() 内部等待提示符    |
| 3    | 执行所有命令           | 执行第 1 条命令 ✓             |
| 4    | 返回结果               | 创建第 2 个 StreamEngine      |
| 5    | -                      | 初始状态 InitAwaitPrompt      |
| 6    | -                      | 等待提示符超时 ✗              |

**为什么第二次会超时？**

设备在第一条命令执行完毕后已经处于提示符状态，不会再主动发送提示符。新的 StreamEngine 期望收到提示符才能离开 `InitAwaitPrompt` 状态，但设备不会发送，导致超时。

## 架构对比

### Engine 的正确做法

```go
// internal/engine/engine.go:594
if err := exec.ExecutePlaybook(ctx, e.Commands, commandTimeout); err != nil {
    // ...
}
```

`ExecutePlaybook` 只创建一次 `StreamEngine`：

```go
// internal/executor/executor.go:136-172
func (e *DeviceExecutor) ExecutePlaybook(ctx context.Context, commands []string, cmdTimeout time.Duration) error {
    // 只创建一次 StreamEngine
    engine := NewStreamEngine(e, e.Client, commands, 80)

    // 初始化一次
    if err := engine.RunInit(ctx, cmdTimeout); err != nil {
        return fmt.Errorf("初始化失败: %w", err)
    }

    // 执行所有命令
    _, err := engine.RunPlaybook(ctx, cmdTimeout)
    return err
}
```

### Discovery Runner 的错误做法

```go
// internal/discovery/runner.go:498-524
for _, cmd := range profile.Commands {
    // 每次循环都创建新的 StreamEngine！
    result, err := exec.ExecuteCommandSyncWithResult(ctx, cmd.Command, commandTimeout)
    // ...
}
```

## 修复方案

### 方案 A：添加 ExecutePlaybookWithResults 方法（推荐）

在 `DeviceExecutor` 中添加新方法，返回每条命令的结果：

```go
// ExecutePlaybookWithResults 执行命令队列并返回每条命令的结果
func (e *DeviceExecutor) ExecutePlaybookWithResults(ctx context.Context, commands []string, cmdTimeout time.Duration) ([]*CommandResult, error) {
    logger.Debug("Executor", e.IP, "开始执行 PlaybookWithResults (%d 条)", len(commands))
    if e.Client == nil {
        return nil, fmt.Errorf("执行器未安全建连")
    }
    defer func() {
        if err := e.flushDetailLog(); err != nil {
            logger.Warn("Executor", e.IP, "刷新详细日志失败: %v", err)
        }
    }()

    engine := NewStreamEngine(e, e.Client, commands, 80)

    if e.OnSuspend != nil {
        engine.SetSuspendHandler(e.OnSuspend)
    }
    if e.Matcher != nil {
        engine.SetErrorMatcher(e.Matcher)
    }
    if e.deviceProfile != nil {
        engine.SetProfile(e.deviceProfile)
    }

    if err := engine.RunInit(ctx, cmdTimeout); err != nil {
        return nil, fmt.Errorf("初始化失败: %w", err)
    }

    return engine.RunPlaybook(ctx, cmdTimeout)
}
```

修改 Discovery Runner：

```go
// internal/discovery/runner.go
func (r *Runner) discoverDevice(...) error {
    // ...

    // 提取命令列表
    commandList := make([]string, len(profile.Commands))
    commandKeyMap := make(map[string]string) // command -> commandKey
    for i, cmd := range profile.Commands {
        commandList[i] = cmd.Command
        commandKeyMap[cmd.Command] = cmd.CommandKey
    }

    // 一次性执行所有命令
    results, err := exec.ExecutePlaybookWithResults(ctx, commandList, taskCommandTimeout)
    if err != nil {
        r.updateDeviceError(taskID, device.IP, fmt.Sprintf("执行失败: %v", err))
        return err
    }

    // 保存每条命令的结果
    for i, result := range results {
        cmd := profile.Commands[i]
        if result.Error != nil {
            r.saveCommandOutput(taskID, device.IP, cmd.CommandKey, cmd.Command, nil, "failed", result.Error.Error())
        } else {
            r.saveCommandOutput(taskID, device.IP, cmd.CommandKey, cmd.Command, result, "success", "")
        }

        if cmd.CommandKey == "version" && result != nil {
            r.parseAndUpdateDeviceInfo(taskID, device.IP, vendor, result.NormalizedText)
        }
    }
    // ...
}
```

### 方案 B：复用 StreamEngine（不推荐）

在 `DeviceExecutor` 中缓存 `StreamEngine`，但这会增加状态管理的复杂性。

## 其他发现的问题

### 1. 缺少详细日志

Discovery Runner 缺少足够的 debug/verbose 日志，难以排查问题。

**建议添加的日志点**：

```go
func (r *Runner) discoverDevice(...) error {
    logger.Debug("Discovery", device.IP, "开始发现设备, vendor=%s, commands=%d", vendor, len(profile.Commands))

    for _, cmd := range profile.Commands {
        logger.Verbose("Discovery", device.IP, "准备执行命令: %s (key=%s, timeout=%d)", cmd.Command, cmd.CommandKey, commandTimeout)
        // ...
    }

    logger.Debug("Discovery", device.IP, "设备发现完成: success=%d, failed=%d", cmdSuccess, cmdFailed)
}
```

### 2. 错误处理不完善

当前实现中，如果某条命令失败，后续命令仍会继续执行，但最终状态判断逻辑可能不够清晰。

## 修复优先级

| 优先级 | 问题         | 影响           |
| ------ | ------------ | -------------- |
| P0     | 会话状态丢失 | 核心功能不可用 |
| P1     | 缺少详细日志 | 难以排查问题   |
| P2     | 错误处理优化 | 用户体验       |

## 总结

**根本原因**：`ExecuteCommandSyncWithResult` 每次调用都创建新的 `StreamEngine`，导致会话状态丢失。

**解决方案**：添加 `ExecutePlaybookWithResults` 方法，让 Discovery Runner 一次性执行所有命令并获取每条命令的结果。

**影响范围**：

- [`internal/executor/executor.go`](../internal/executor/executor.go) - 添加新方法
- [`internal/discovery/runner.go`](../internal/discovery/runner.go) - 修改调用方式
