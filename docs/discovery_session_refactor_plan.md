# 拓扑发现会话状态问题彻底修复方案

## 1. 背景

当前拓扑发现链路在同一 SSH Shell 已经建立的前提下，按“单命令调用”反复创建新的 `StreamEngine`，导致会话状态丢失：

- 第一条命令执行成功
- 后续命令重新进入 `InitAwaitPrompt`
- 设备不会主动再次发送初始提示符
- 最终连续读超时，任务卡死或失败

这不是单点 bug，而是执行链路的职责边界不清造成的结构性问题。

## 2. 当前根因

### 2.1 会话生命周期与命令生命周期混在一起

当前 `internal/discovery/runner.go` 在命令循环中反复调用：

- `executor.ExecuteCommandSyncWithResult(...)`

而 `internal/executor/executor.go` 中的该方法每次都会：

- 新建 `StreamEngine`
- 新建 `SessionAdapter`
- 重新从 `InitAwaitPrompt` 开始状态机

这和当前 SSH Shell 的真实状态不一致。

### 2.2 discovery 和 executor 各维护一套命令配置

当前存在两套命令源：

- `internal/discovery/command_profile.go`
- `internal/config/device_profile.go`

它们已经出现命令差异。后续即使修复会话问题，也仍然容易在命令、超时、厂商适配上继续分叉。

### 2.3 批量执行接口能力不足

`ExecutePlaybook(...)` 虽然能复用单个 `StreamEngine`，但只返回整体错误，不返回每条命令结果，无法直接支撑 discovery 对每条命令输出的落库和解析。

### 2.4 超时语义没有成为一等能力

discovery 当前有“每条命令独立超时”的业务要求，但 executor 的主批量接口只接收一个默认超时。现有实现中对内联超时的解析也没有形成稳定的公开语义，不适合作为长期方案。

## 3. 修复目标

本次修复目标不是“让方案A能跑”，而是彻底收敛执行模型，保证后续同类问题不再出现。

目标如下：

1. 同一设备发现过程只创建一个会话状态机。
2. 会话初始化只执行一次。
3. discovery 与 executor 共享同一套命令配置。
4. 每条命令的超时、结果、错误语义都由统一执行接口承载。
5. 即使中途失败，也能保留已完成命令的部分结果。
6. 明确区分“命令级失败”和“会话级失败”。
7. 从 API 设计上避免 discovery 再次误用单命令接口。

## 4. 最终设计

### 4.1 核心原则

执行链路拆成三层：

1. `DeviceExecutor`
   负责连接、持有 SSH Client、提供会话工厂。
2. `CommandSession`
   负责单个设备一次完整交互会话，内部唯一持有一个 `StreamEngine`。
3. `ExecutionPlan`
   负责描述要执行什么、每条命令的策略是什么。

### 4.2 新的统一执行模型

新增统一计划结构：

```go
type ExecutionPlan struct {
    Name                 string
    Commands             []PlannedCommand
    AbortOnTransportErr  bool
    AbortOnCommandTimeout bool
    ContinueOnCmdError   bool
    Mode                 ExecutionMode
}

type PlannedCommand struct {
    Key             string
    Command         string
    Timeout         time.Duration
    ContinueOnError bool
}
```

新增统一结果结构：

```go
type ExecutionReport struct {
    PlanName      string
    Results       []*CommandResult
    FatalError    error
    SessionHealthy bool
    StartedAt     time.Time
    FinishedAt    time.Time
}
```

说明：

- `Results` 必须始终返回已经完成的命令结果
- `FatalError` 表示会话级终止原因
- `SessionHealthy=false` 表示状态机/连接已不可继续复用

### 4.3 新增 CommandSession

建议新增：

- `internal/executor/session_runner.go`

定义大致如下：

```go
type CommandSession struct {
    executor *DeviceExecutor
    engine   *StreamEngine
    inited   bool
}

func (e *DeviceExecutor) NewSession(plan ExecutionPlan) (*CommandSession, error)
func (s *CommandSession) Run(ctx context.Context) (*ExecutionReport, error)
func (s *CommandSession) Close()
```

设计要求：

1. `CommandSession` 在构造时只创建一次 `StreamEngine`
2. `RunInit()` 只允许执行一次
3. `Run(ctx)` 在一个状态机内消费整个 `ExecutionPlan`
4. 不允许按命令重新 new engine

### 4.4 StreamEngine 支持计划化命令

当前 `StreamEngine` 的输入是 `[]string`，建议升级为支持 `[]PlannedCommand`，至少要做到：

1. 当前命令上下文中保存 `Key`
2. 当前命令上下文中保存显式 `Timeout`
3. 发送命令前优先使用命令级 `Timeout`
4. `CommandResult` 中回填 `Key`

建议修改：

- `internal/executor/command_context.go`
- `internal/executor/session_types.go`
- `internal/executor/session_reducer.go`
- `internal/executor/stream_engine.go`

推荐将 `SessionContext.Queue` 从 `[]string` 升级为 `[]PlannedCommand`，避免继续依赖字符串解析附带控制信息。

### 4.5 统一配置源

必须删除 discovery 自己维护的命令配置源，统一改为使用：

- `internal/config/device_profile.go`

具体动作：

1. 保留 `config.DeviceProfile.Commands` 作为唯一命令定义源
2. discovery 根据 `DeviceProfile` 生成 `ExecutionPlan`
3. 删除或废弃 `internal/discovery/command_profile.go`
4. UI 如需查看厂商命令配置，也从 `config` 暴露只读接口，不再经过 discovery 私有结构

这是本次重构的关键点之一。否则会话问题修复后，配置分叉仍会继续制造新的执行偏差。

### 4.6 单命令接口重新定位

保留以下接口仅用于真正的单命令场景：

- `ExecuteCommandSync`
- `ExecuteCommandSyncWithResult`

但要明确规则：

1. discovery 禁止调用这两个接口
2. 文档和注释中注明它们是“临时/便捷/非批处理”接口
3. 如有可能，新增注释或 lint 约定，避免批处理路径再次调用

如果项目允许更彻底收口，后续可进一步把这两个接口迁移成：

- 内部调试工具接口
- 一次性临时命令接口

而主流程统一走 `ExecutionPlan`

## 5. discovery 侧重构方案

### 5.1 discoverDevice 改造

当前 `discoverDevice(...)` 中的“命令 for 循环 + 单条执行”改为：

1. 根据设备厂商读取 `config.DeviceProfile`
2. 构建 `ExecutionPlan`
3. 创建单一 `CommandSession`
4. 一次执行完整 plan
5. 根据 `ExecutionReport.Results` 逐条持久化输出
6. 根据 `ExecutionReport.FatalError` 和结果统计更新设备状态

伪代码：

```go
profile := config.GetDeviceProfile(vendor)
plan := buildDiscoveryPlan(profile, taskCommandTimeout)

session, err := exec.NewSession(plan)
if err != nil { ... }
defer session.Close()

report, runErr := session.Run(ctx)

for _, result := range report.Results {
    saveCommandOutput(...)
    if result.CommandKey == "version" {
        parseAndUpdateDeviceInfo(...)
    }
}

deviceStatus := summarizeDiscoveryStatus(report, runErr)
```

### 5.2 失败语义统一

需要明确三种失败：

1. 命令级失败
   例如设备返回错误提示，但提示符恢复正常
2. 会话级失败
   例如读超时、流关闭、状态机失稳、初始化失败
3. 上下文级失败
   例如 `ctx.Done()`、设备级超时

建议规则：

- 命令级失败：默认继续执行后续命令，设备最终状态可能为 `partial`
- 会话级失败：立即终止该设备发现，设备状态为 `failed` 或 `partial`
- 上下文级失败：标记取消或超时，立即退出

### 5.3 任务状态判定

设备级状态建议统一为：

- `success`: 所有命令成功
- `partial`: 至少一条成功，且有失败
- `failed`: 没有任何命令成功，或初始化/连接/会话级失败
- `cancelled`: 上下文取消

任务级状态继续沿用：

- `completed`
- `partial`
- `failed`
- `cancelled`

## 6. 关键接口设计建议

### 6.1 executor

新增：

```go
func (e *DeviceExecutor) ExecutePlan(ctx context.Context, plan ExecutionPlan) (*ExecutionReport, error)
```

说明：

- 这是 discovery、engine 等批量路径唯一应使用的统一入口
- `error` 只表示调用级异常
- 执行过程中产生的会话级失败要同时体现在 `ExecutionReport.FatalError`

或者更彻底地拆成：

```go
func (e *DeviceExecutor) NewSession(plan ExecutionPlan) (*CommandSession, error)
```

如果从长期演进考虑，推荐 `NewSession + Run` 的形式，更容易表达“一个连接上的单会话执行”。

### 6.2 config

新增辅助方法：

```go
func BuildDiscoveryPlan(profile *config.DeviceProfile, taskTimeout time.Duration) ExecutionPlan
```

作用：

- 将 `profile.Commands` 转换为带超时和策略的统一执行计划
- 统一处理命令默认超时与任务级上限的裁剪

### 6.3 result

建议扩展 `CommandResult`：

```go
type CommandResult struct {
    CommandKey string
    ...
}
```

这样 discovery 落库时不必依赖外层索引强行匹配结果和命令列表，鲁棒性更高。

## 7. 实施步骤

### 阶段 1：建立统一执行抽象

1. 新增 `ExecutionPlan`、`PlannedCommand`、`ExecutionReport`
2. 新增 `CommandSession` 或 `ExecutePlan`
3. `StreamEngine` 改为支持计划化命令
4. `CommandResult` 增加 `CommandKey`

### 阶段 2：统一配置源

1. discovery 不再使用 `internal/discovery/command_profile.go`
2. 全部改用 `config.DeviceProfile`
3. 调整 discovery service 对外暴露的厂商命令配置来源
4. 删除重复定义或把其改成兼容层后尽快移除

### 阶段 3：改造 discovery 主流程

1. `discoverDevice()` 构建 `ExecutionPlan`
2. 一次执行完整设备计划
3. 改为消费 `ExecutionReport`
4. 重写设备状态汇总逻辑

### 阶段 4：收口旧接口

1. 保留 `ExecuteCommandSync*` 仅作为单命令便捷接口
2. 注释中明确禁止批处理链路调用
3. 检查项目内所有调用方，确保 discovery/engine 主流程统一走新接口

### 阶段 5：补测试并回归验证

1. 补 executor 单元测试
2. 补 discovery 集成测试
3. 使用真实日志场景做回归

## 8. 测试方案

### 8.1 executor 单元测试

必须覆盖：

1. 同一会话多命令连续执行成功
2. 第一条成功，第二条命令级失败，第三条继续成功
3. 中间命令超时，返回 partial results
4. 初始化成功后不再重新进入 `InitAwaitPrompt`
5. 每条命令 timeout 生效
6. `CommandKey` 正确透传到 `CommandResult`

### 8.2 discovery 集成测试

必须覆盖：

1. discovery 执行完整命令列表并全部落库
2. 某条命令失败但设备状态为 `partial`
3. 初始化失败或连接失败时设备状态为 `failed`
4. 取消任务时设备/任务状态正确
5. `version` 结果仍能触发设备信息更新

### 8.3 回归验证

使用当前问题日志对应的真实场景进行回归：

1. `display version` 成功后，后续命令不再卡在 `InitAwaitPrompt`
2. 设备完整执行所有发现命令
3. 原始输出和规范化输出均正确落盘
4. 任务最终不再出现假死

## 9. 日志增强建议

重构时同时增强以下日志点：

### discovery

- 开始构建计划
- 计划命令数
- 每条命令 key/timeout
- report 返回结果统计
- 设备状态汇总结果

### executor

- session 创建/关闭
- init 开始/完成
- 每条命令发送/完成/失败
- 会话级 fatal error
- report 汇总输出

建议日志字段统一带：

- `taskID`
- `deviceIP`
- `commandKey`
- `cmdIndex`

## 10. 为什么这是最优方案

相比“只补一个 `ExecutePlaybookWithResults`”的局部修复，本方案更优的原因是：

1. 它从 API 层彻底消除了“批处理路径误用单命令接口”的可能性。
2. 它统一了配置源，避免修完会话问题后继续出现命令配置分叉。
3. 它把每条命令超时、部分结果返回、错误语义统一成正式能力，而不是靠调用方拼装。
4. 它保留了未来扩展空间，例如分阶段采集、命令分组、按能力裁剪计划等。

## 11. 不推荐的方案

### 11.1 在 DeviceExecutor 中缓存 StreamEngine

不推荐原因：

- 状态复用边界不清
- 生命周期复杂
- 容易产生并发安全问题
- 外部调用顺序稍有变化就会引入新问题

### 11.2 继续保留 discovery 的独立 command_profile

不推荐原因：

- 配置双写
- 厂商命令定义持续分叉
- 修复一次问题后很快又会在别处失配

### 11.3 依赖内联命令注释携带控制信息

不推荐原因：

- 可读性差
- 不利于类型约束
- 不适合作为主执行模型

## 12. 预计影响文件

核心会涉及：

- `internal/executor/executor.go`
- `internal/executor/stream_engine.go`
- `internal/executor/command_context.go`
- `internal/executor/session_types.go`
- `internal/executor/session_reducer.go`
- `internal/discovery/runner.go`
- `internal/config/device_profile.go`
- `internal/ui/discovery_service.go`

可能新增：

- `internal/executor/session_runner.go`
- `internal/executor/execution_plan.go`

可能删除或废弃：

- `internal/discovery/command_profile.go`

## 13. 验收标准

满足以下条件才算修复完成：

1. 发现任务不再出现“首条命令成功，后续卡在 `InitAwaitPrompt`”。
2. 单设备发现全程只初始化一次会话状态机。
3. discovery 与 executor 使用同一套厂商命令配置。
4. 每条命令超时策略可控且可测试。
5. 中途失败时，已完成命令结果不会丢失。
6. 设备状态与任务状态判定符合预期。
7. 旧的单命令接口不再被批处理链路使用。

## 14. 推荐实施结论

推荐采用“统一执行计划 + 单会话批量执行 + 单一配置源”的整体重构方案。

这不是最小改动方案，但它是当前项目阶段下最适合的长期解法：

- 改动集中
- 结构清晰
- 易于测试
- 能一次性消灭该问题背后的设计根因

如果只做局部补丁，问题会从“会话状态丢失”转移成“配置漂移”“超时语义不一致”或“错误处理语义混乱”。本方案可以同时把这些问题一起收口。
