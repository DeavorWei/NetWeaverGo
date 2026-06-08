# SNMP 轮询并发控制设计方案

> **文档类型**: 架构设计 & 重构规划  
> **创建日期**: 2026-06-08  
> **最后修订**: 2026-06-08 (v1.1 审计修订)  
> **状态**: 待评审  

### 修订记录

| 版本 | 日期 | 变更说明 |
|------|------|----------|
| v1.0 | 2026-06-08 | 初版设计 |
| v1.1 | 2026-06-08 | 审计修订：修复 UpdateConfig 竞态（goroutine 永久阻塞）、补充 configMu 锁、time.After 改为 time.NewTimer、SkipIfBusy 扩展到 Level 1、移除未使用的 activeTasks、补充 DispatchBatch 实现、补充 deviceSems 清理机制 |
| v1.2 | 2026-06-08 | 二次审计修订：修复 currentTaskSem 捕获顺序（先捕获再获取）、rebuildTaskSem 增加超时保护、DispatchBatch 正确处理 error |
| v1.3 | 2026-06-08 | 三次审计修订：DispatchSync 返回值去重、补充 Context/超时分层说明、完善设备信号量过渡期说明、增加 waitingCount/skippedCount 统计、防范 MaxOpsPerDevice=0 风险、区分 Skipped 和 Cancelled、优化回滚方案 |

---

## 目录

1. [现状分析](#1-现状分析)
2. [问题与BUG清单](#2-问题与bug清单)
3. [设计目标](#3-设计目标)
4. [整体架构设计](#4-整体架构设计)
5. [详细设计](#5-详细设计)
6. [配置方案](#6-配置方案)
7. [数据模型变更](#7-数据模型变更)
8. [API & 前端变更](#8-api--前端变更)
9. [迁移与兼容策略](#9-迁移与兼容策略)
10. [实施计划](#10-实施计划)

---

## 1. 现状分析

### 1.1 整体架构概览

当前 SNMP 轮询系统采用三层架构：

```
┌──────────────────────────────────────────────────────────────┐
│                     UI 层 (Wails 绑定)                        │
│           snmp_polling_service.go                             │
│   PollNow / PollAllNow / StartScheduler / StopScheduler     │
└───────────────────────┬──────────────────────────────────────┘
                        │
┌───────────────────────▼──────────────────────────────────────┐
│                  调度层 (PollerScheduler)                      │
│           poller_scheduler.go                                 │
│   Cron 定时触发 → createPollFunc → PollSingle                │
│   RunAllNow → PollBatch (批量)                                │
└───────────────────────┬──────────────────────────────────────┘
                        │
┌───────────────────────▼──────────────────────────────────────┐
│                  执行层 (Poller)                               │
│           poller.go                                           │
│   PollSingle / PollBatch / PollBatchNoRetry                  │
│   workerSem (chan struct{}, MaxWorkers) 并发控制              │
└──────────────────────────────────────────────────────────────┘
```

### 1.2 当前并发控制机制

#### Poller 层

| 组件 | 位置 | 机制 | 说明 |
|------|------|------|------|
| `workerSem` | `poller.go:73` | `chan struct{}` 信号量 | 控制 `PollBatch` 批量轮询的全局并发数 |
| `MaxWorkers` | `poller.go:30` | 配置参数，默认 10 | 信号量容量 |
| `mu sync.RWMutex` | `poller.go:74` | 读写锁 | **实际未被任何方法使用** |

#### PollerScheduler 层

| 组件 | 位置 | 机制 | 说明 |
|------|------|------|------|
| `mu sync.RWMutex` | `poller_scheduler.go:69` | 读写锁 | 保护 `jobs` map 和 `running` 状态 |
| Cron 调度器 | `poller_scheduler.go:54` | `robfig/cron/v3` | 每个目标独立 cron 条目 |

### 1.3 轮询执行流程

#### 1.3.1 定时轮询（Cron 触发）

```
Cron Scheduler
  └─► 目标 A 的 cron 触发 → goroutine → createPollFunc() 
  └─► 目标 B 的 cron 触发 → goroutine → createPollFunc()
  └─► 目标 C 的 cron 触发 → goroutine → createPollFunc()
      ↓
  每个 createPollFunc:
    1. ctx = context.WithTimeout(30s)
    2. poller.PollSingle(ctx, target)
    3. 保存结果到 DB
    4. 更新目标状态
```

**关键发现**: Cron 调度器为每个目标独立触发 goroutine，**没有经过 `workerSem` 信号量控制**。`workerSem` 仅在 `PollBatch` / `PollBatchNoRetry` 方法中使用。

#### 1.3.2 手动"立即轮询"（PollNow）

```
前端 PollNow(targetID)
  └─► SNMPPollingService.PollNow()
        └─► PollerScheduler.RunNow()
              └─► poller.PollSingle() — 直接调用，无并发控制
```

#### 1.3.3 手动"全部轮询"（PollAllNow）

```
前端 PollAllNow()
  └─► SNMPPollingService.PollAllNow()
        └─► PollerScheduler.RunAllNow()
              └─► poller.PollBatch() — 使用 workerSem 信号量控制
```

### 1.4 当前并发模型图

```
         ┌─────────────────────────────────────────────┐
         │           Cron Scheduler (无并发限制)          │
         │                                              │
  Timer  │  Target_1 ──► goroutine ──► PollSingle()    │
  Events │  Target_2 ──► goroutine ──► PollSingle()    │  ← 不受 workerSem 控制!
         │  Target_3 ──► goroutine ──► PollSingle()    │
         │  ...                                         │
         │  Target_N ──► goroutine ──► PollSingle()    │
         └─────────────────────────────────────────────┘

         ┌─────────────────────────────────────────────┐
         │       PollBatch (受 workerSem 控制)           │
  Manual │                                              │
  Batch  │  Target_1 ─┐                                │
  Trigger│  Target_2 ─┤──► workerSem(10) ──► PollSingle│
         │  Target_3 ─┤                                │
         │  ...       ─┘                                │
         └─────────────────────────────────────────────┘
```

---

## 2. 问题与BUG清单

### 2.1 严重问题（BUG）

#### BUG-1: Cron 定时轮询无并发控制 🔴 严重

**位置**: `poller_scheduler.go:546-593` (`createPollFunc`)

**现象**: 每个 Cron 任务独立触发 goroutine，直接调用 `poller.PollSingle()`，完全绕过了 `workerSem` 信号量。

**影响**: 
- 如果有 100 个设备、轮询间隔 30 秒，那么理论上可能同时有 100 个 goroutine 并发访问 100 台设备
- 对 SNMP 设备（通常是嵌入式系统）造成巨大压力
- 程序自身的 UDP Socket 和内存消耗不可控
- SQLite 数据库写入也会产生大量并发争抢

**根因**: `createPollFunc` 内部直接调用 `poller.PollSingle`，没有获取 `workerSem` 信号量。`workerSem` 仅在 `PollBatch` 方法中使用，而 Cron 触发走的是完全不同的代码路径。

```go
// poller_scheduler.go:546-593
func (s *PollerScheduler) createPollFunc(job *scheduledJob) func() {
    return func() {
        // ❌ 这里没有获取 workerSem 信号量！
        ctx, cancel := context.WithTimeout(context.Background(), s.pollTimeout)
        defer cancel()
        results, err := s.poller.PollSingle(ctx, pollTarget)  // 直接调用
        // ...
    }
}
```

#### BUG-2: Poller 的 `mu sync.RWMutex` 从未被使用 🟡 中等

**位置**: `poller.go:74`

**现象**: `Poller` 结构体声明了 `mu sync.RWMutex` 但没有任何方法使用它。

**影响**: 死代码，增加认知负担。虽然 `totalPolls`/`successCount`/`failCount` 使用了 `atomic` 操作是安全的，但 `mu` 的存在暗示开发者可能遗忘了某些需要保护的临界区。

#### BUG-3: PollSingle 中统计计数不准确 🟡 中等

**位置**: `poller.go:312-314` 和 `poller.go:955-956`

**现象**: `PollSingle` 在成功时累加 `totalPolls` 和 `successCount`，但在失败的早期返回路径（如凭据获取失败、连接失败）中，通过 `notifyError` 也会累加 `totalPolls` 和 `failCount`。然而，如果 `PollSingle` 在 OID 轮询阶段部分成功（某些 OID 成功、某些失败但被 `continue` 跳过），它仍然被计为完全成功。

```go
// 成功路径
atomic.AddInt64(&p.totalPolls, 1)
atomic.AddInt64(&p.successCount, 1)

// 失败路径 (notifyError)
atomic.AddInt64(&p.totalPolls, 1)
atomic.AddInt64(&p.failCount, 1)
```

**影响**: 统计数据可能误导运维判断。

#### BUG-4: RunNow 与 Cron 任务可能对同一设备并发轮询 🟡 中等

**位置**: `poller_scheduler.go:302-386` (`RunNow`)

**现象**: 手动 `RunNow` 调用 `poller.PollSingle` 时，不检查该目标是否正在被 Cron 任务轮询中。两个 goroutine 可能同时对同一台 SNMP 设备发起请求。

**影响**:
- 某些 SNMP 设备（尤其是低端设备）不支持并发 SNMP 连接，可能导致超时或错误
- 产生重复数据（两批相同的轮询结果）
- SNMP 使用 UDP 协议，同一设备同一端口的多个连接可能导致响应包串扰

### 2.2 设计不合理之处

#### D-1: 单设备并发不可控

**现象**: 没有任何机制限制同一台设备（同一 IP）同时进行多少个 SNMP 请求。如果一个设备关联了多个模板或多个目标条目，它们可能同时发起请求。

#### D-2: 两套并发控制路径不统一

**现象**: `PollBatch` 使用 `workerSem` 控制并发，`createPollFunc`（Cron 路径）和 `RunNow` 完全不受控。系统存在两条完全独立的轮询执行路径，并发控制无法统一。

#### D-3: PollBatch 的信号量获取在主 goroutine 阻塞

**位置**: `poller.go:346`

```go
for i, target := range targets {
    p.workerSem <- struct{}{}  // ← 在主 goroutine 中阻塞
    wg.Add(1)
    go func(...) { ... }
}
```

**影响**: 如果 `workerSem` 已满，主 goroutine 会阻塞在循环中，直到有空闲 worker。这虽然是正确的限速行为，但如果 context 被取消，主 goroutine 无法响应取消信号——它会一直阻塞在信号量获取上。

#### D-4: 每次轮询都创建新连接

**位置**: `poller.go:273-278`

**现象**: `PollSingle` 每次调用都创建一个新的 `gosnmp.GoSNMP` 连接并在结束后关闭。对于高频轮询（如 30 秒间隔），这会带来不必要的 UDP Socket 开销。

> **注**: 这不在本次设计范围内，但值得记录为后续优化点。

#### D-5: Poller 初始化时不使用配置文件中的参数

**位置**: `cmd/netweaver/main.go:123`

```go
poller := snmp.NewPoller(oidResolver, snmpCrypto, snmpEventNotifier)
```

**现象**: `Poller` 在 `main.go` 中初始化时没有传入配置，使用 `DefaultPollerConfig`（硬编码 `MaxWorkers=10`）。虽然 `SNMPServerConfig` 模型中有 `MaxPollingWorkers` 字段，但该值从未被读取并传给 `Poller`。

#### D-6: 排序算法使用冒泡排序

**位置**: `snmp_polling_service.go:1308-1329`

```go
func sortDataPointsByTime(points []PollingDataPoint) {
    for i := 0; i < len(points)-1; i++ {
        for j := i + 1; j < len(points); j++ {
            // O(n²) 冒泡排序
        }
    }
}
```

**影响**: 数据量大时性能低下。应使用 `sort.Slice` 或 `slices.SortFunc`。

---

## 3. 设计目标

### 3.1 核心目标

| 编号 | 目标 | 优先级 |
|------|------|--------|
| G-1 | **任务级并发控制**：控制轮询任务同时并发几台设备 | P0 |
| G-2 | **单设备并发控制**：控制同一台设备同时进行几个 SNMP 拉取操作 | P0 |
| G-3 | 统一所有轮询路径的并发控制（Cron / RunNow / PollBatch） | P0 |
| G-4 | 并发参数可配置、可动态调整 | P1 |
| G-5 | 修复已识别的 BUG | P1 |

### 3.2 约束条件

- 不改变外部 API 签名（保持前端兼容）
- 不引入新的外部依赖
- 保持与现有 Cron 调度器的兼容
- 不影响 Trap 监听等其他 SNMP 功能

---

## 4. 整体架构设计

### 4.1 新架构总览

核心思想：引入 **两级信号量** 机制：

```
┌─────────────────────────────────────────────────────────────────┐
│                      UI 层 (不变)                                │
└───────────────────────┬─────────────────────────────────────────┘
                        │
┌───────────────────────▼─────────────────────────────────────────┐
│                 调度层 (PollerScheduler)                          │
│   所有路径统一通过 submitPoll() 提交轮询任务                       │
│   Cron 触发 / RunNow / RunAllNow → submitPoll()                 │
└───────────────────────┬─────────────────────────────────────────┘
                        │
┌───────────────────────▼─────────────────────────────────────────┐
│              并发控制层 (PollDispatcher) [新增]                    │
│                                                                  │
│   ┌─────────────────────────────────────────────────────┐       │
│   │         Level 1: 任务级并发控制 (taskSem)             │       │
│   │         控制同时有多少台设备在轮询                       │       │
│   │         容量: MaxConcurrentDevices (默认 10)          │       │
│   └───────────────────┬─────────────────────────────────┘       │
│                       │                                          │
│   ┌───────────────────▼─────────────────────────────────┐       │
│   │       Level 2: 单设备并发控制 (deviceSemMap)          │       │
│   │       控制同一设备同时进行几个 SNMP 拉取               │       │
│   │       容量: MaxOpsPerDevice (默认 1)                  │       │
│   │       key: TargetIP                                   │       │
│   └───────────────────┬─────────────────────────────────┘       │
│                       │                                          │
└───────────────────────┼──────────────────────────────────────────┘
                        │
┌───────────────────────▼─────────────────────────────────────────┐
│                    执行层 (Poller)                                │
│    PollSingle() — 纯执行，不再包含并发控制逻辑                     │
└─────────────────────────────────────────────────────────────────┘
```

### 4.2 核心组件交互序列

```
                  ┌──────────┐         ┌──────────────┐      ┌──────────┐
                  │Scheduler │         │PollDispatcher│      │  Poller  │
                  └────┬─────┘         └──────┬───────┘      └────┬─────┘
   Cron触发/RunNow     │                      │                   │
  ────────────────────►│                      │                   │
                       │  submitPoll(target)  │                   │
                       │─────────────────────►│                   │
                       │                      │                   │
                       │            ┌─────────┤                   │
                       │            │获取taskSem                   │
                       │            │(等待设备槽位)                 │
                       │            └─────────┤                   │
                       │                      │                   │
                       │            ┌─────────┤                   │
                       │            │获取deviceSem                 │
                       │            │(等待该设备的                   │
                       │            │ 操作槽位)                     │
                       │            └─────────┤                   │
                       │                      │                   │
                       │                      │  PollSingle()     │
                       │                      │──────────────────►│
                       │                      │                   │
                       │                      │     results/err   │
                       │                      │◄──────────────────│
                       │                      │                   │
                       │            ┌─────────┤                   │
                       │            │释放deviceSem                 │
                       │            │释放taskSem                   │
                       │            └─────────┤                   │
                       │                      │                   │
                       │   result callback    │                   │
                       │◄─────────────────────│                   │
                       │                      │                   │
```

---

## 5. 详细设计

### 5.1 PollDispatcher（新增核心组件）

#### 5.1.1 结构体定义

```go
// PollDispatcher SNMP 轮询分发器
// 实现两级并发控制：任务级（跨设备）和设备级（同设备）
type PollDispatcher struct {
    poller *Poller

    // Level 1: 任务级并发控制
    // 控制全局同时有多少台设备在进行轮询
    taskSem chan struct{}

    // Level 2: 单设备并发控制
    // 控制同一台设备同时进行几个 SNMP 操作
    deviceSems      map[string]chan struct{} // key: targetIP
    deviceLastUsed  map[string]time.Time     // 设备信号量最后使用时间（用于过期清理）
    deviceSemsMu    sync.Mutex
    maxOpsPerDevice int                      // 快照值，创建新设备信号量时使用

    // 配置保护
    configMu sync.RWMutex   // 保护 config 的读写
    config   DispatcherConfig

    // 状态跟踪
    activeCount  int64 // 当前活跃任务数（atomic）
    waitingCount int64 // 正在排队等待槽位的任务数（atomic）
    skippedCount int64 // 因繁忙被主动跳过的任务总数（atomic）
}
```

> **v1.1/v1.3 审计修订**: 移除了未使用的 `activeTasks`/`activeTasksMu`（原为预留的防重入设计，Level 2 信号量已覆盖此需求）。新增 `configMu` 保护配置读写、`deviceLastUsed` 支持过期清理、`activeCount`/`waitingCount`/`skippedCount` 原子计数以支持精准监控。

#### 5.1.2 配置定义

```go
// DispatcherConfig 分发器配置
type DispatcherConfig struct {
    // MaxConcurrentDevices 最大并发设备数
    // 同一时刻最多有多少台设备在进行 SNMP 轮询
    // 默认: 10
    MaxConcurrentDevices int

    // MaxOpsPerDevice 单设备最大并发操作数
    // 同一台设备同时允许多少个 SNMP 操作（GET/WALK/BULK）
    // 对于不支持并发的设备应设为 1
    // 默认: 1
    MaxOpsPerDevice int

    // SkipIfBusy 如果设备繁忙是否跳过
    // true: 设备正在轮询中时跳过本次触发（适用于 Cron 防重叠）
    // false: 排队等待（适用于手动触发和批量轮询）
    // 默认: true（Cron 场景）
    SkipIfBusy bool

    // QueueTimeout 排队超时
    // 等待信号量的最大时间，超时后放弃
    // 默认: 30s
    QueueTimeout time.Duration
}

var DefaultDispatcherConfig = DispatcherConfig{
    MaxConcurrentDevices: 10,
    MaxOpsPerDevice:      1,
    SkipIfBusy:           true,
    QueueTimeout:         30 * time.Second,
}
```

#### 5.1.3 核心方法

```go
// NewPollDispatcher 创建轮询分发器
func NewPollDispatcher(poller *Poller, config ...DispatcherConfig) *PollDispatcher

// Dispatch 提交轮询任务（异步执行，返回 channel 接收结果）
// 会根据配置进行两级并发控制
func (d *PollDispatcher) Dispatch(ctx context.Context, target *PollTarget, 
    opts ...DispatchOption) <-chan *PollResult

// DispatchSync 同步提交轮询任务（阻塞直到完成）
func (d *PollDispatcher) DispatchSync(ctx context.Context, target *PollTarget, 
    opts ...DispatchOption) *PollResult

// DispatchBatch 批量提交轮询任务
// 内部自动进行两级并发控制，无需调用方管理
func (d *PollDispatcher) DispatchBatch(ctx context.Context, 
    targets []*PollTarget) []*PollResult

// UpdateConfig 动态更新并发配置
// 注意：MaxConcurrentDevices 变更需要重建 taskSem
func (d *PollDispatcher) UpdateConfig(config DispatcherConfig) error

// GetStatus 获取分发器状态
func (d *PollDispatcher) GetStatus() *DispatcherStatus
```

#### 5.1.4 结果类型

```go
// PollResult 轮询结果封装
type PollResult struct {
    Target    *PollTarget
    Results   []*models.SNMPPollingResult
    Error     error
    Latency   time.Duration
    Skipped   bool  // 因繁忙被主动跳过（并非错误）
    Cancelled bool  // 因 Context 取消或排队超时而中断
    Queued    bool  // 曾经排队等待
}

// DispatcherStatus 分发器运行状态
type DispatcherStatus struct {
    ActiveDevices    int            `json:"activeDevices"`    // 当前活跃设备数
    MaxDevices       int            `json:"maxDevices"`       // 最大并发设备数
    MaxOpsPerDevice  int            `json:"maxOpsPerDevice"`  // 单设备最大并发
    WaitingTasks     int            `json:"waitingTasks"`     // 排队等待信号量的任务数
    SkippedTasks     int64          `json:"skippedTasks"`     // 累计因繁忙跳过的任务数
    DeviceStatus     map[string]int `json:"deviceStatus"`     // 每台设备的活跃操作数
}
```

#### 5.1.5 Dispatch 核心逻辑伪代码

```go
func (d *PollDispatcher) DispatchSync(ctx context.Context, target *PollTarget, 
    opts ...DispatchOption) *PollResult {
    
    startTime := time.Now()
    targetIP := target.Target.TargetIP
    options := applyOptions(opts...)

    // 读取配置快照（加锁）
    d.configMu.RLock()
    queueTimeout := d.config.QueueTimeout
    d.configMu.RUnlock()

    // ━━━ Level 1: 获取任务级信号量 ━━━
    // 【v1.2 关键修订】先捕获 taskSem 到局部变量，再用局部变量获取令牌
    // 保证获取和释放操作在同一个 channel 上，彻底消除与 UpdateConfig 的竞态窗口
    currentTaskSem := d.taskSem

    atomic.AddInt64(&d.waitingCount, 1)
    if options.SkipIfBusy {
        // Cron 路径：Level 1 非阻塞尝试，系统过载时快速跳过
        select {
        case currentTaskSem <- struct{}{}:
            // 获取成功
        default:
            atomic.AddInt64(&d.waitingCount, -1)
            atomic.AddInt64(&d.skippedCount, 1)
            logger.Info("Dispatcher", targetIP, "系统繁忙（任务槽已满），跳过本次轮询")
            return &PollResult{Skipped: true}
        }
    } else {
        // 手动触发路径：阻塞等待，带超时
        timer := time.NewTimer(queueTimeout)
        select {
        case currentTaskSem <- struct{}{}:
            timer.Stop()
        case <-ctx.Done():
            timer.Stop()
            atomic.AddInt64(&d.waitingCount, -1)
            return &PollResult{Cancelled: true, Error: ctx.Err()}
        case <-timer.C:
            atomic.AddInt64(&d.waitingCount, -1)
            return &PollResult{Cancelled: true, Error: fmt.Errorf("等待任务槽位超时")}
        }
    }
    atomic.AddInt64(&d.waitingCount, -1)
    
    atomic.AddInt64(&d.activeCount, 1)
    defer func() {
        <-currentTaskSem  // 释放到获取时的同一个 channel
        atomic.AddInt64(&d.activeCount, -1)
    }()

    // ━━━ Level 2: 获取设备级信号量 ━━━
    deviceSem := d.getOrCreateDeviceSem(targetIP)
    
    atomic.AddInt64(&d.waitingCount, 1)
    if options.SkipIfBusy {
        // Cron 路径：非阻塞尝试获取
        select {
        case deviceSem <- struct{}{}:
            // 获取成功
        default:
            atomic.AddInt64(&d.waitingCount, -1)
            atomic.AddInt64(&d.skippedCount, 1)
            // 设备繁忙，跳过
            logger.Info("Dispatcher", targetIP, "设备繁忙，跳过本次轮询")
            return &PollResult{Skipped: true}
        }
    } else {
        // 手动触发路径：阻塞等待
        select {
        case deviceSem <- struct{}{}:
            // 获取成功
        case <-ctx.Done():
            atomic.AddInt64(&d.waitingCount, -1)
            return &PollResult{Cancelled: true, Error: ctx.Err()}
        }
    }
    atomic.AddInt64(&d.waitingCount, -1)
    defer func() { <-deviceSem }()

    // ━━━ 执行轮询 ━━━
    results, err := d.poller.pollWithRetry(ctx, target)

    return &PollResult{
        Target:  target,
        Results: results,
        Error:   err,
        Latency: time.Since(startTime),
    }
}

// getOrCreateDeviceSem 获取或创建设备级信号量
// 【v1.1 修订】使用 d.maxOpsPerDevice 而非 d.config.MaxOpsPerDevice，避免竞态
func (d *PollDispatcher) getOrCreateDeviceSem(targetIP string) chan struct{} {
    d.deviceSemsMu.Lock()
    defer d.deviceSemsMu.Unlock()

    // 更新最后使用时间（用于过期清理）
    d.deviceLastUsed[targetIP] = time.Now()

    if sem, exists := d.deviceSems[targetIP]; exists {
        return sem
    }

    // 使用快照值 maxOpsPerDevice，该值在 deviceSemsMu 保护下更新
    sem := make(chan struct{}, d.maxOpsPerDevice)
    d.deviceSems[targetIP] = sem
    return sem
}
```

> **v1.1 审计修订要点**:
> 1. `time.After` → `time.NewTimer` + `timer.Stop()`，防止 timer 资源泄漏
> 2. `SkipIfBusy` 模式下 Level 1 也做非阻塞尝试，系统过载时快速跳过而不是排队 30 秒超时
> 3. 【v1.2 修订】`currentTaskSem := d.taskSem` 在 select **之前**捕获，获取和释放都使用同一个局部变量，彻底消除获取与捕获之间的竞态窗口
> 4. `getOrCreateDeviceSem` 使用 `d.maxOpsPerDevice`（在 `deviceSemsMu` 保护下更新的快照值）而非直接读 `d.config.MaxOpsPerDevice`，消除竞态

#### 5.1.6 DispatchBatch 实现

```go
// DispatchBatch 批量提交轮询任务
// 为每个目标启动独立 goroutine，两级并发控制由 DispatchSync 内部保证
// 所有任务完成后统一返回结果（部分成功部分失败的情况通过 PollResult.Error 区分）
func (d *PollDispatcher) DispatchBatch(ctx context.Context,
    targets []*PollTarget) []*PollResult {

    if len(targets) == 0 {
        return nil
    }

    results := make([]*PollResult, len(targets))
    var wg sync.WaitGroup

    for i, t := range targets {
        wg.Add(1)
        go func(idx int, target *PollTarget) {
            defer wg.Done()

            // 每个目标通过 DispatchSync 提交，自动受两级信号量控制
            // 批量场景使用 SkipIfBusy=false（排队等待）
            // 【v1.3 修订】因为 DispatchSync 只返回 *PollResult，我们只需要处理它
            result := d.DispatchSync(ctx, target, WithSkipIfBusy(false))
            if result == nil {
                result = &PollResult{Target: target, Error: fmt.Errorf("dispatch 返回 nil")}
            }
            results[idx] = result
        }(i, t)
    }

    wg.Wait()
    return results
}
```

> **设计说明**:
> - 所有 goroutine 同时启动，但实际执行受 `taskSem` 限流（最多 `MaxConcurrentDevices` 个并行）
> - 同一设备的多个目标受 `deviceSem` 进一步限流
> - `results[idx]` 通过索引写入，无需额外同步
> - 部分失败时 `result.Error != nil`，`result.Results` 为 nil，调用方需逐个检查

### 5.2 PollerScheduler 改造

#### 5.2.1 结构体变更

```diff
 type PollerScheduler struct {
-    poller    *Poller
+    dispatcher *PollDispatcher
     repo      repository.PollingRepository
     scheduler *cron.Cron
     notifier  EventNotifier
     pollTimeout time.Duration
     jobs map[uint]*scheduledJob
     running    bool
     startTime  time.Time
     totalPolls int64
     mu sync.RWMutex
 }
```

#### 5.2.2 createPollFunc 改造

```diff
 func (s *PollerScheduler) createPollFunc(job *scheduledJob) func() {
     return func() {
         ctx, cancel := context.WithTimeout(context.Background(), s.pollTimeout)
         defer cancel()

         pollTarget := &PollTarget{
             Target:   job.Target,
             Template: job.Template,
             Cred:     job.Cred,
         }

-        results, err := s.poller.PollSingle(ctx, pollTarget)
+        // 通过分发器提交，受两级并发控制
+        result, err := s.dispatcher.DispatchSync(ctx, pollTarget,
+            WithSkipIfBusy(true),  // Cron 触发时，设备繁忙则跳过
+        )
+
+        if result.Skipped {
+            logger.Warn("SNMP-Scheduler", job.Target.TargetIP,
+                "定时轮询跳过: ID=%d (设备繁忙或队列已满)", job.TargetID)
+            return
+        }

+        results := result.Results
         // ... 保存结果逻辑不变
     }
 }
```

#### 5.2.3 RunNow 改造

```diff
 func (s *PollerScheduler) RunNow(ctx context.Context, targetID uint) ([]*models.SNMPPollingResult, error) {
     // ... 加载 job 逻辑不变 ...

-    results, err := s.poller.PollSingle(ctx, pollTarget)
+    // 通过分发器提交，排队等待（手动触发不跳过）
+    result := s.dispatcher.DispatchSync(ctx, pollTarget,
+        WithSkipIfBusy(false),  // 手动触发时排队等待
+    )
+    if result.Error != nil {
+        return nil, result.Error
+    }
+    results := result.Results
     // ... 后续逻辑不变
 }
```

#### 5.2.4 RunAllNow 改造

```diff
 func (s *PollerScheduler) RunAllNow(ctx context.Context) [][]*models.SNMPPollingResult {
     // ... 加载 jobs 逻辑不变 ...

-    allResults := s.poller.PollBatch(ctx, pollTargets)
+    // 通过分发器批量提交
+    pollResults := s.dispatcher.DispatchBatch(ctx, pollTargets)
+    allResults := make([][]*models.SNMPPollingResult, len(pollResults))
+    for i, r := range pollResults {
+        allResults[i] = r.Results
+    }
     // ... 后续保存逻辑不变
 }
```

### 5.3 Poller 简化

移除 Poller 中不再需要的并发控制逻辑：

```diff
 type Poller struct {
     resolver *OIDResolver
     crypto   *CredentialCrypto
     notifier EventNotifier
     config   PollerConfig
     totalPolls   int64
     successCount int64
     failCount    int64
-    workerSem chan struct{}
-    mu        sync.RWMutex
 }
```

- 删除 `PollBatch` 和 `PollBatchNoRetry` 方法（批量调度由 `PollDispatcher` 接管）
- `PollSingle` 和 `pollWithRetry` 保留为纯执行方法
- 删除未使用的 `mu sync.RWMutex`

### 5.4 动态配置更新

> **v1.1 审计修订**: 完全重写此章节。原方案直接替换 `d.taskSem` 会导致正在执行的
> goroutine 的 defer 释放令牌到新的空 channel，造成 **goroutine 永久阻塞**（非写入旧通道，
> 而是从空的新通道读取时死锁）。新方案采用「等待排空后重建」策略。

#### 5.4.1 安全更新策略

**核心原则**: 不在有活跃任务时替换信号量。配置变更分为「立即生效」和「等待排空后生效」两类。

```go
// UpdateConfig 动态更新分发器配置
// 【v1.1 重写】采用安全更新策略，避免信号量替换竞态
func (d *PollDispatcher) UpdateConfig(config DispatcherConfig) error {
    // 【v1.3 新增】配置底线校验，防止容量为0引发永久阻塞
    if config.MaxOpsPerDevice <= 0 {
        config.MaxOpsPerDevice = 1
    }
    if config.MaxConcurrentDevices <= 0 {
        config.MaxConcurrentDevices = 1
    }

    d.configMu.Lock()
    oldConfig := d.config

    // ━━━ 立即生效的配置 ━━━
    // QueueTimeout、SkipIfBusy 等不涉及信号量容量的参数
    // 通过 configMu 保护，DispatchSync 读取时加 RLock
    d.config.QueueTimeout = config.QueueTimeout
    d.config.SkipIfBusy = config.SkipIfBusy
    d.configMu.Unlock()

    // ━━━ 设备级并发数变更（可安全重建）━━━
    // 【v1.3 补充说明】旧设备信号量会在其空闲时由 CleanupIdleDeviceSems 回收。
    // 在这期间，已分配的旧设备和新分配的新设备会出现容量不同的短暂不一致。
    // 这属于平滑过渡策略的可接受范围。
    if config.MaxOpsPerDevice != oldConfig.MaxOpsPerDevice {
        d.deviceSemsMu.Lock()
        // 不清空旧 map！旧信号量让正在执行的任务自然完成
        // 只更新 maxOpsPerDevice，新创建的信号量使用新容量
        d.maxOpsPerDevice = config.MaxOpsPerDevice
        // 标记所有旧信号量为过期，下次 getOrCreate 时如果容量不匹配则重建
        d.deviceSems = make(map[string]chan struct{})
        d.deviceSemsMu.Unlock()
        logger.Info("Dispatcher", "-", "设备级并发数已更新: %d -> %d (新任务生效)",
            oldConfig.MaxOpsPerDevice, config.MaxOpsPerDevice)
    }

    // ━━━ 任务级并发数变更（需等待排空）━━━
    if config.MaxConcurrentDevices != oldConfig.MaxConcurrentDevices {
        // 异步等待排空后重建
        go d.rebuildTaskSem(config.MaxConcurrentDevices, oldConfig.MaxConcurrentDevices)
    }

    return nil
}

// rebuildTaskSem 等待所有活跃任务完成后重建任务级信号量
// 在后台 goroutine 中执行，不阻塞调用方
func (d *PollDispatcher) rebuildTaskSem(newCapacity, oldCapacity int) {
    logger.Info("Dispatcher", "-", "任务级并发数变更请求: %d -> %d, 等待活跃任务完成...",
        oldCapacity, newCapacity)

    // 【v1.2 修订】增加超时保护，防止活跃任务长期不结束导致 goroutine 永不退出
    const rebuildTimeout = 5 * time.Minute
    deadline := time.After(rebuildTimeout)

    // 等待所有活跃任务完成（轮询检查，间隔 500ms）
    // 活跃任务的 defer 释放的是 currentTaskSem（局部变量捕获），不受此处影响
    for {
        active := atomic.LoadInt64(&d.activeCount)
        if active == 0 {
            break
        }

        select {
        case <-deadline:
            logger.Warn("Dispatcher", "-",
                "等待活跃任务超时 (%v)，放弃重建 taskSem (仍有 %d 个活跃任务)，"+
                    "当前容量 %d 保持不变",
                rebuildTimeout, active, oldCapacity)
            // 【v1.3 新增】通过系统通知前端用户配置未能完全生效
            if d.notifier != nil {
                d.notifier.NotifyError(fmt.Errorf("变更任务并发数超时，容量保留为 %d", oldCapacity))
            }
            return
        default:
            logger.Debug("Dispatcher", "-", "等待活跃任务完成: 剩余 %d 个", active)
            time.Sleep(500 * time.Millisecond)
        }
    }

    // 所有任务已完成，安全替换
    newSem := make(chan struct{}, newCapacity)
    d.taskSem = newSem

    d.configMu.Lock()
    d.config.MaxConcurrentDevices = newCapacity
    d.configMu.Unlock()

    logger.Info("Dispatcher", "-", "任务级并发数已生效: %d -> %d",
        oldCapacity, newCapacity)
}
```

#### 5.4.2 DispatchSync 中的局部变量捕获（关键防护）

```go
// 【v1.2 修订】在获取令牌之前就捕获 taskSem 到局部变量
// 获取和释放都使用 currentTaskSem，彻底消除竞态窗口
currentTaskSem := d.taskSem

// 用 currentTaskSem（而非 d.taskSem）获取令牌
select {
case currentTaskSem <- struct{}{}:
    // ...
}

// defer 也用 currentTaskSem 释放
defer func() {
    <-currentTaskSem
}()
```

**为什么必须先捕获再获取**：如果先获取再捕获（v1.1 的写法），在 `d.taskSem <- struct{}{}` 
成功后、`currentTaskSem := d.taskSem` 执行前，`rebuildTaskSem` 可能已替换了 `d.taskSem`。
此时令牌写入了旧 channel，但 `currentTaskSem` 捕获了新的空 channel，defer 从空 channel 
读取会永久阻塞。先捕获再获取保证获取和释放在同一个 channel 上操作。

### 5.5 设备信号量生命周期管理

> **v1.1 新增**: 原设计中 `deviceSems` map 只增不减，长期运行存在内存泄漏。

```go
// CleanupIdleDeviceSems 清理长时间未使用的设备信号量
// 建议由外部定时调用（如每小时一次）
func (d *PollDispatcher) CleanupIdleDeviceSems(maxIdleDuration time.Duration) int {
    d.deviceSemsMu.Lock()
    defer d.deviceSemsMu.Unlock()

    now := time.Now()
    cleaned := 0

    for ip, lastUsed := range d.deviceLastUsed {
        if now.Sub(lastUsed) > maxIdleDuration {
            // 检查信号量是否空闲（无人持有）
            sem := d.deviceSems[ip]
            if sem != nil && len(sem) == 0 {
                delete(d.deviceSems, ip)
                delete(d.deviceLastUsed, ip)
                cleaned++
            }
        }
    }

    if cleaned > 0 {
        logger.Info("Dispatcher", "-", "已清理 %d 个空闲设备信号量 (阈值: %v)",
            cleaned, maxIdleDuration)
    }
    return cleaned
}

// RemoveDeviceSem 主动移除指定设备的信号量（设备被删除时调用）
func (d *PollDispatcher) RemoveDeviceSem(targetIP string) {
    d.deviceSemsMu.Lock()
    defer d.deviceSemsMu.Unlock()
    delete(d.deviceSems, targetIP)
    delete(d.deviceLastUsed, targetIP)
}
```

**调用时机**:
- `CleanupIdleDeviceSems`: 由 `DataCleaner` 的清理循环中附带调用，阈值建议 1 小时
- `RemoveDeviceSem`: 在 `PollerScheduler.RemoveTarget` 中调用，设备被删除时同步清理

---

## 6. 配置方案

### 6.1 配置层级

```
┌────────────────────────────────────────────────────────┐
│  SNMPServerConfig (数据库持久化)                         │
│                                                         │
│  MaxPollingWorkers       → MaxConcurrentDevices         │
│  MaxOpsPerDevice  [新增]  → MaxOpsPerDevice              │
│  PollSkipIfBusy   [新增]  → SkipIfBusy                   │
│  PollQueueTimeout [新增]  → QueueTimeout                 │
└────────────────────────────────────────────────────────┘
          ↓ 启动时读取 & 动态更新
┌────────────────────────────────────────────────────────┐
│  DispatcherConfig (运行时)                               │
│                                                         │
│  MaxConcurrentDevices = 10                              │
│  MaxOpsPerDevice      = 1                               │
│  SkipIfBusy           = true                            │
│  QueueTimeout         = 30s                             │
└────────────────────────────────────────────────────────┘
```

### 6.2 默认值与推荐值

| 参数 | 默认值 | 推荐范围 | 说明 |
|------|--------|----------|------|
| `MaxConcurrentDevices` | 10 | 1-100 | 同时轮询的最大设备数 |
| `MaxOpsPerDevice` | 1 | 1-5 | 单设备同时进行的 SNMP 操作数 |
| `SkipIfBusy` | true | true/false | Cron 触发时设备繁忙是否跳过 |
| `QueueTimeout` | 30s | 10s-120s | 等待信号量的最大超时时间 |

**推荐说明**:
- `MaxOpsPerDevice = 1`: 大多数网络设备（特别是低端交换机、路由器）不支持并发 SNMP 会话，设为 1 最安全
- `MaxOpsPerDevice = 3~5`: 高端设备（如核心交换机、服务器 BMC）可以适当提高
- `MaxConcurrentDevices`: 取决于管理站的网络带宽和 CPU。10 是一个保守的默认值，100 台设备的场景可以设到 20-30

---

## 7. 数据模型变更

### 7.1 SNMPServerConfig 扩展

```diff
 type SNMPServerConfig struct {
     // ... 现有字段不变 ...
     MaxPollingWorkers int  `json:"maxPollingWorkers"`
+    MaxOpsPerDevice   int  `json:"maxOpsPerDevice"    gorm:"default:1"`
+    PollSkipIfBusy    bool `json:"pollSkipIfBusy"     gorm:"default:true"`
+    PollQueueTimeout  int  `json:"pollQueueTimeout"   gorm:"default:30"` // 秒
 }
```

### 7.2 数据库迁移

在 `config/snmp_db.go` 的 `AutoMigrateSNMP` 中自动处理，GORM 的 AutoMigrate 会自动添加新列。

---

## 8. API & 前端变更

### 8.1 后端 API 变更

#### 新增 SchedulerStatusVM 字段

```diff
 type SchedulerStatusVM struct {
     IsRunning    bool   `json:"isRunning"`
     TargetCount  int    `json:"targetCount"`
     TotalPolls   int64  `json:"totalPolls"`
     LastPollTime string `json:"lastPollTime"`
     StartTime    string `json:"startTime"`
+    // 并发控制状态
+    ActiveDevices       int `json:"activeDevices"`
+    MaxConcurrentDevices int `json:"maxConcurrentDevices"`
+    MaxOpsPerDevice     int `json:"maxOpsPerDevice"`
 }
```

#### 新增并发配置 API

```go
// UpdateConcurrencyConfig 更新并发控制配置
func (s *SNMPPollingService) UpdateConcurrencyConfig(ctx context.Context, 
    maxDevices, maxOpsPerDevice int) error

// GetConcurrencyConfig 获取当前并发控制配置
func (s *SNMPPollingService) GetConcurrencyConfig(ctx context.Context) (*ConcurrencyConfigVM, error)
```

### 8.2 前端变更建议

在「SNMP 轮询」页面的设置区域增加并发控制配置项：

```
┌─ 并发控制设置 ──────────────────────────────────────┐
│                                                      │
│  最大并发设备数:    [  10  ] 台  (1-100)               │
│                                                      │
│  单设备并发操作数:  [   1  ] 个  (1-5)                 │
│                                                      │
│  定时触发防重叠:    [✓] 设备繁忙时跳过                  │
│                                                      │
│  排队超时时间:      [  30  ] 秒  (10-120)              │
│                                                      │
│  当前状态: 活跃设备 3/10, 排队任务 0                    │
│                                                      │
│                            [ 保存 ] [ 恢复默认 ]      │
└──────────────────────────────────────────────────────┘
```

---

## 9. 迁移与兼容策略

### 9.1 代码迁移步骤

1. **Phase 1 - 新增 `PollDispatcher`**: 创建 `internal/snmp/poll_dispatcher.go`，完成两级信号量逻辑
2. **Phase 2 - 改造 `PollerScheduler`**: 将所有轮询提交路径改为通过 `PollDispatcher`
3. **Phase 3 - 简化 `Poller`**: 移除 `PollBatch`/`PollBatchNoRetry`/`workerSem`/`mu`
4. **Phase 4 - 数据模型 & 配置**: 扩展 `SNMPServerConfig`，连接配置读取
5. **Phase 5 - 前端**: 添加并发配置 UI

### 9.2 向后兼容

| 项目 | 兼容性 | 说明 |
|------|--------|------|
| 前端 API | ✅ 完全兼容 | 现有前端 API 签名不变，新增字段为可选 |
| 数据库 | ✅ 自动迁移 | GORM AutoMigrate 自动添加新列，带默认值 |
| 轮询行为 | ⚠️ 行为变化 | Cron 触发可能因设备繁忙而跳过（之前不会跳过） |

### 9.3 回滚方案

如果新系统出现问题：
1. `PollDispatcher` 可以配置为 `MaxConcurrentDevices = math.MaxInt32, MaxOpsPerDevice = math.MaxInt32` 退化为极高并发（近似旧行为）
2. 数据库新增字段保留，不影响旧代码

---

## 10. 实施计划

### 10.1 文件变更清单

| 文件 | 操作 | 说明 |
|------|------|------|
| `internal/snmp/poll_dispatcher.go` | **新增** | 两级并发控制分发器 |
| `internal/snmp/poller_scheduler.go` | 修改 | 所有路径通过 dispatcher 提交 |
| `internal/snmp/poller.go` | 修改 | 移除 `PollBatch` / `workerSem` / `mu` |
| `internal/snmp/types.go` | 修改 | 新增 `DispatcherConfig` / `PollResult` / `DispatcherStatus` |
| `internal/models/snmp.go` | 修改 | `SNMPServerConfig` 新增字段 |
| `internal/ui/snmp_polling_service.go` | 修改 | 新增并发配置 API，状态查询扩展 |
| `cmd/netweaver/main.go` | 修改 | 初始化 `PollDispatcher`，读取配置 |

### 10.2 估计工作量

| 阶段 | 内容 | 预估工时 |
|------|------|----------|
| Phase 1 | `PollDispatcher` 实现 | 3h |
| Phase 2 | `PollerScheduler` 改造 | 2h |
| Phase 3 | `Poller` 简化 & BUG 修复 | 1h |
| Phase 4 | 数据模型 & 配置集成 | 1h |
| Phase 5 | UI 服务层 & 前端 | 2h |
| 测试 | 单元测试 & 集成测试 | 2h |
| **总计** | | **~11h** |

### 10.3 测试计划

#### 单元测试

1. `PollDispatcher` 两级信号量正确性
   - 并发数不超过 `MaxConcurrentDevices`
   - 同一设备并发数不超过 `MaxOpsPerDevice`
   - `SkipIfBusy` 模式正确跳过
   - `QueueTimeout` 正确超时
   - Context 取消正确传播

2. `PollerScheduler` 改造后兼容性
   - Cron 触发通过 Dispatcher 正确执行
   - `RunNow` 排队等待行为正确
   - `RunAllNow` 批量执行正确

#### 集成测试

1. 配置更新后并发参数即时生效
2. 100 个目标的压力测试，验证并发控制
3. 同一设备同时被 Cron 和 RunNow 触发时的行为

---

## 附录 A: 额外优化建议（非本次范围）

| 编号 | 建议 | 优先级 | 说明 |
|------|------|--------|------|
| OPT-1 | SNMP 连接池 | P2 | 复用 SNMP 连接减少 UDP Socket 开销 |
| OPT-2 | 排序算法优化 | P3 | 使用 `sort.Slice` 替代冒泡排序 |
| OPT-3 | 轮询结果批量写入 | P2 | 多个设备结果攒批后统一 INSERT 减少 SQLite 压力 |
| OPT-4 | 配置文件驱动 Poller 参数 | P1 | 从 `SNMPServerConfig` 读取 `MaxPollingWorkers` 并传给 `Poller` |
| ~~OPT-5~~ | ~~设备级信号量过期清理~~ | - | ~~已在 v1.1 主设计 5.5 节中解决~~ |

---

## 附录 B: v1.1 审计修订详情

本节记录 v1.1 修订所解决的审计问题。

### B.1 已修复问题清单

| 编号 | 问题 | 严重度 | 修复位置 | 修复策略 |
|------|------|--------|----------|----------|
| A-1 | UpdateConfig 直接替换 taskSem 导致 goroutine 永久阻塞 | 🔴 严重 | §5.4 重写 | 改为「等待排空后重建」+ DispatchSync 中局部变量捕获 |
| A-2 | `d.mu` 在结构体中不存在 | 🔴 严重 | §5.1.1 | 新增 `configMu sync.RWMutex` |
| A-3 | `getOrCreateDeviceSem` 读 `config.MaxOpsPerDevice` 竞态 | 🟡 中等 | §5.1.5 | 改用 `d.maxOpsPerDevice` 快照值，在 `deviceSemsMu` 保护下更新 |
| A-4 | `time.After` 在 select 中导致 timer 泄漏 | 🟡 中等 | §5.1.5 | 改用 `time.NewTimer` + `timer.Stop()` |
| A-5 | SkipIfBusy 仅在 Level 2 生效，Level 1 过载时无法快速跳过 | 🟡 中等 | §5.1.5 | SkipIfBusy 模式下 Level 1 也做非阻塞尝试 |
| A-6 | `activeTasks`/`activeTasksMu` 定义了但未使用 | 🟢 低 | §5.1.1 | 移除，Level 2 信号量已覆盖防重入需求 |
| A-7 | `deviceSems` map 只增不减，长期内存泄漏 | 🟢 低 | §5.5 新增 | 新增 `CleanupIdleDeviceSems` + `RemoveDeviceSem` |
| A-8 | `DispatchBatch` 只有签名无实现 | 🟡 中等 | §5.1.6 新增 | 补充完整实现：goroutine + WaitGroup + 索引写入 |
| A-10 | `currentTaskSem` 捕获在获取之后，存在竞态窗口 | 🔴 严重 | §5.1.5 v1.2 修订 | 改为先捕获再获取，select 使用 `currentTaskSem` |
| A-11 | `rebuildTaskSem` 无超时，可能永不退出 | 🟡 中等 | §5.4.1 v1.2 修订 | 增加 5 分钟超时，超时后放弃重建 |
| A-12 | `DispatchBatch` 忽略 `DispatchSync` 返回的 error | 🟡 中等 | §5.1.6 v1.2 修订 | 正确传递 error 到 `PollResult.Error` |
| A-13 | `DispatchSync` 返回值包含多余且易混淆的 error | 🟡 中等 | §5.1.3 v1.3 修订 | 统一为只返回 `*PollResult` |
| A-14 | `maxOpsPerDevice` 允许配置为 0，会导致无限阻塞 | 🔴 严重 | §5.4.1 v1.3 修订 | 配置时限制下限为 1 |
| A-15 | `QueuedTasks` 在通道 select 中无法精确统计 | 🟡 中等 | §5.1.4 v1.3 修订 | 改为明确的 `WaitingTasks`（在获取前增，获取后减） |
| A-16 | 跳过的任务缺少数值监控 | 🟡 中等 | §5.1.4 v1.3 修订 | 添加 `SkippedTasks` 和 `skippedCount` 原子统计 |
| A-17 | 取消操作导致 `Skipped=true`，语义矛盾 | 🟡 中等 | §5.1.4 v1.3 修订 | 新增 `Cancelled: true` 区分系统繁忙跳过和被强行取消 |
| A-18 | 配置变更超时后，用户无感知 | 🟢 低 | §5.4.1 v1.3 修订 | 超时发生时，调用 `d.notifier.NotifyError` 提醒用户 |

### B.2 审计中不属实的问题

| 编号 | 问题 | 结论 | 理由 |
|------|------|------|------|
| A-9 | 信号量释放顺序导致优先级问题 | ❌ 不属实 | Go defer LIFO 顺序为先释放 deviceSem 再释放 taskSem，这是正确的：先释放设备槽位让同设备的其他操作可以继续，再释放任务槽位让其他设备可以开始。channel 不存在优先级概念。 |
| A-19 | `CleanupIdleDeviceSems` 用 `len(sem)==0` 检查不安全 | ❌ 不属实 | Go 语言的 chan 长度 `len(c)` 对于带缓冲的 chan 表示队列中目前存在多少未读取的元素。在信号量模式下，已填入结构体意味着被占用。`len==0` 就是绝对空闲，安全可用。 |
| A-20 | Timer 创建引发 GC 压力 | ❌ 不属实 | `time.NewTimer` 非常轻量，针对此应用场景（例如每秒调度百个任务）完全可以接受。对于 SNMP 而言，IO 和解析开销才是真正的瓶颈，过早优化不仅增加系统复杂度，实际提升也微乎其微。 |
