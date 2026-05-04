# 路径跟踪 (Tracert) 功能审计报告

**审计日期**: 2026-05-04  
**审计范围**: `internal/icmp/tracert_engine.go`, `internal/ui/tracert_service.go`, `internal/icmp/types.go`, `internal/icmp/icmp_windows.go`, `internal/icmp/icmp_raw.go`, `internal/icmp/icmp_backend.go`  
**审计目标**: 检查路径跟踪功能的逻辑漏洞、并发安全、资源管理和架构问题

---

## 一、架构概览

路径跟踪功能采用三层架构：

```
┌──────────────────────────┐
│   TracertService (UI层)  │  internal/ui/tracert_service.go
│  Wails 服务生命周期管理   │  DNS 缓存、持续探测、前端事件
├──────────────────────────┤
│   TracertEngine (引擎层)  │  internal/icmp/tracert_engine.go
│  探测调度、结果合并、状态  │  并发 TTL 探测、多轮控制
├──────────────────────────┤
│   ICMP Backend (传输层)   │  icmp_windows.go / icmp_raw.go
│  Windows API / Raw Socket │  单包发送与响应解析
└──────────────────────────┘
```

---

## 二、发现的问题

### 🔴 严重 (Critical)

#### C-1: `runContinuous` 中对 `s.progress` 的数据竞争

**文件**: `tracert_service.go:353, 370, 376, 383, 402-403`

在 `runContinuous` 中，多处直接读取 `s.progress` 的字段而**未持有 `progressMu` 锁**：

```go
// 第 353 行 — OnUpdate 回调中未加锁读取 s.progress
reachedTTL := atomic.LoadInt32(&s.progress.MinReachedTTL)

// 第 370 行 — 外层循环中未加锁读取
oldMinReachedTTL := atomic.LoadInt32(&s.progress.MinReachedTTL)

// 第 376 行 — 同上
newMinReachedTTL := atomic.LoadInt32(&s.progress.MinReachedTTL)

// 第 383 行 — 直接传入 s.progress.Hops 切片，无锁
s.resolveHopHostNames(dnsCtx, s.progress.Hops, 2*time.Second)

// 第 402-403 行 — 未加锁读取 MinReachedTTL 和 Hops
reachedTTL := s.progress.MinReachedTTL
```

**风险**: `s.progress` 指针本身可被 `setProgress()` 替换，而 `MinReachedTTL` 虽为 atomic int32 但对 `s.progress` 指针的读取不安全。当 `mergeRoundResult` 在同一 goroutine 中持锁修改 `Hops` 切片，而 `resolveHopHostNames` 在另一侧并行修改同一切片时，存在**切片内容数据竞争**。

**建议**: 所有对 `s.progress` 指针及其 `Hops` 字段的读写都应在 `progressMu` 保护下进行，或在传递给 `resolveHopHostNames` 前先 Clone。

---

#### C-2: `resolveHopHostNames` 对 `hops` 切片的并发写入与外部读取竞争

**文件**: `tracert_service.go:125-219`

`resolveHopHostNames` 接收的 `hops []icmp.TracertHopResult` 是 `s.progress.Hops` 的直接引用（见第 383 行）。该方法内部启动多个 goroutine 修改 `hops[i].HostName`（第 208-211 行），使用局部 `mu` 互斥。但与此同时：

- `mergeRoundResult`（持 `progressMu`）可能正在读写同一切片
- `emitProgress` 可能正在 Clone 同一切片
- `OnUpdate` 回调可能正在读取同一切片

局部 `mu` 只保护 `resolveHopHostNames` 内部多个 goroutine 之间的互斥，不保护与外部调用者的竞争。

**建议**: 传入 `hops` 前先做深拷贝，或者将 DNS 解析结果通过 channel 回写到主 goroutine 中串行化应用。

---

#### C-3: `OnUpdate` 回调中的锁嵌套与竞争

**文件**: `tracert_service.go:344-354`

```go
roundProgress := s.engine.Run(ctx, target, icmp.TracertRunOptions{
    OnUpdate: func(p *icmp.TracertProgress) {
        s.progressMu.Lock()
        if s.progress != nil {
            p.Round = s.progress.Round
        }
        s.progressMu.Unlock()
        s.mergeRoundResult(p)  // mergeRoundResult 内部也会获取 progressMu
        reachedTTL := atomic.LoadInt32(&s.progress.MinReachedTTL) // 无锁
        s.emitProgress(s.progress.CloneForDisplay(reachedTTL))    // 无锁访问 s.progress
    },
```

问题：
1. `mergeRoundResult` 内部获取 `progressMu.Lock()`，但回调中先获取后释放锁，再调用 `mergeRoundResult`——这两次加锁之间存在间隙，`s.progress` 可能被替换
2. 第 353-354 行在 `mergeRoundResult` 返回后无锁读取 `s.progress`

---

### 🟠 高危 (High)

#### H-1: `runRound` 中 `break` 仅跳出 `select` 而非 `for` 循环

**文件**: `tracert_engine.go:225-241`

```go
for ttl := 1; ttl <= maxHops; ttl++ {
    select {
    case <-ctx.Done():
        // ...
        for t := ttl; t <= maxHops; t++ { ... }
        break  // ← 仅跳出 select，不跳出 for
    default:
    }
    wg.Add(1)
    go func(ttlVal int) { ... }(ttl)
}
```

当 `ctx.Done()` 触发时，`break` 只跳出 `select`，随后代码继续执行 `wg.Add(1)` 并启动新 goroutine。已取消的 cancelled 结果被写入 `resultChan`，但 for 循环继续运行会**额外启动不必要的 goroutine**。

**影响**: 取消时不会立即停止启动新探测 goroutine，导致资源浪费和可能的 cancelled 结果数量不正确（部分 TTL 既发送了 cancelled 又启动了实际探测）。

**建议**: 使用 `goto` 或将 `break` 改为带标签的 `break`，或在 `default` 分支后添加 `continue` 逻辑：

```go
select {
case <-ctx.Done():
    for t := ttl; t <= maxHops; t++ { ... }
    goto doneSpawning
default:
}
```

---

#### H-2: `mergeRoundResult` 中 `atomic` 与 `progressMu` 混用

**文件**: `tracert_service.go:418-521`

`mergeRoundResult` 已持有 `progressMu.Lock()`，但内部仍使用 `atomic.LoadInt32` / `atomic.StoreInt32` 访问 `MinReachedTTL`：

```go
func (s *TracertService) mergeRoundResult(...) {
    s.progressMu.Lock()
    defer s.progressMu.Unlock()
    minReachedTTL := atomic.LoadInt32(&s.progress.MinReachedTTL) // 已持锁，atomic 多余
    // ...
    atomic.StoreInt32(&s.progress.MinReachedTTL, roundResult.MinReachedTTL)
}
```

同时在 `tracert_engine.go` 的 `runRound` 中，`MinReachedTTL` 在无锁情况下用 `atomic` 被多个 goroutine 并发修改（collect loop、probeHop goroutines）——这意味着同一个字段在不同层级有两套同步机制（锁 vs atomic），容易导致语义混乱和隐蔽 bug。

**建议**: 统一同步策略。引擎层内部用 atomic 是合理的（并发收集结果），但服务层在已持锁时应直接读写而非 atomic，以减少认知负担。

---

#### H-3: `StopTracert` 中 `s.engine = nil` 后竞争

**文件**: `tracert_service.go:524-583`

```go
func (s *TracertService) StopTracert() error {
    s.continuousMu.Lock()
    if s.continuousCancel != nil {
        s.continuousCancel()
        s.continuousCancel = nil
    }
    s.continuousMu.Unlock()

    s.engineMu.Lock()
    // ...
    engine := s.engine
    s.engine = nil          // ← 将 engine 设为 nil
    s.engineMu.Unlock()

    engine.Stop()           // ← 调用 Stop 后等待
```

问题在于 `s.engine = nil` 后，正在执行的 `runSingle` 或 `runContinuous` goroutine 中的 `s.engine.Run(...)` 调用仍然持有旧 engine 引用，这是安全的。但如果此时用户快速调用 `StartTracert`，`isRunningLocked()` 会因 `s.engine == nil` 返回 false，允许创建新 engine——而旧 engine 可能仍在运行（`engine.Stop()` 是异步的）。

**影响**: 两个 engine 可能短暂并发运行，旧 goroutine 的回调会修改已被新任务替换的 `s.progress`。

**建议**: 在 `StopTracert` 中等待旧 engine 完全停止后再返回（当前的 polling 逻辑检查的是 `engine.IsRunning()`，但 `runContinuous` goroutine 可能在 engine.Stop 后仍在执行间隔等待）。

---

#### H-4: ICMP Handle 每次探测重复创建/销毁

**文件**: `icmp_windows.go:381-485`, `icmp_raw.go:77-196`

每次 `probeHop` → `PingOneWithTTL` 调用都会：
1. `IcmpCreateFile()` 创建新 ICMP 句柄
2. 执行探测
3. `IcmpCloseHandle()` 关闭句柄

在并发探测 30 个 TTL 时，会同时创建 30 个 ICMP 句柄。Raw Socket 后端同理（每次 `icmp.ListenPacket`）。

**影响**: 
- 系统资源开销大
- Windows 下高并发 ICMP 句柄可能触发系统限制
- Raw Socket 模式下并发监听同一端口会导致**响应包被错误的 goroutine 接收**

**建议**: 考虑使用句柄池或复用单一连接。

---

### 🟡 中危 (Medium)

#### M-1: `truncateString` 按字节截断，可能破坏 UTF-8 字符

**文件**: `tracert_service.go:903-908`

```go
func truncateString(s string, maxLen int) string {
    if len(s) <= maxLen {
        return s
    }
    return s[:maxLen-3] + "..."
}
```

`len(s)` 返回字节长度，`s[:maxLen-3]` 按字节截断。对于包含中文主机名的场景，可能在 UTF-8 多字节字符中间截断，产生无效字符串。

**建议**: 使用 `[]rune` 转换后按 rune 计数截断。

---

#### M-2: DNS 缓存无容量上限

**文件**: `tracert_service.go:56-57, 113-122`

```go
dnsCache   map[string]dnsCacheEntry
```

DNS 缓存仅有 5 分钟 TTL 清理机制，但无容量上限。在持续模式下长时间运行探测不同目标时，缓存可能无限增长。

**建议**: 添加缓存容量上限（如 1000 条），达到上限时清除最旧条目。

---

#### M-3: `stopDNSCacheCleanup` 非幂等，重复调用会 panic

**文件**: `tracert_service.go:104-110`

```go
func (s *TracertService) stopDNSCacheCleanup() {
    if s.cleanupStopCh != nil {
        close(s.cleanupStopCh)
        s.cleanupStopCh = nil
    }
}
```

虽然有 nil 检查，但如果被并发调用（如 `ServiceShutdown` 与其他清理路径），可能在检查和关闭之间发生竞争。

**建议**: 使用 `sync.Once` 或加锁保护。

---

#### M-4: `ExportTracertResultCSV` 双重过滤

**文件**: `tracert_service.go:652-727`

```go
func (s *TracertService) ExportTracertResultCSV() (*TracertExportResult, error) {
    progress := s.GetTracertProgress()       // 已经做了 CloneForDisplay 过滤
    // ...
    filteredProgress := progress.CloneForDisplay(progress.MinReachedTTL)  // 再次过滤
```

`GetTracertProgress()` 内部已调用 `CloneForDisplay`，返回的 progress 已是过滤后的副本。再次调用 `CloneForDisplay` 是冗余操作，不会产生错误但浪费内存和 CPU。TXT 导出同理（第 738 行）。

**建议**: 移除第二次 `CloneForDisplay` 调用。

---

#### M-5: Config 验证重复且不一致

**文件**: `tracert_service.go:794-837` vs `tracert_engine.go:24-63`

配置验证在 `mergeWithDefaultConfig`（服务层）和 `NewTracertEngine`（引擎层）中重复执行。两处的默认值逻辑一致，但验证范围不完全相同：

| 参数 | 服务层 | 引擎层 |
|------|--------|--------|
| `Timeout` 下限 | 无下限验证 | 无下限验证 |
| `DataSize` 下限 | 无下限验证 | 无下限验证 |
| `Concurrency` | **未验证** | **未验证** |

`Timeout` 可被设置为 1ms（过短可能导致全部超时），`DataSize` 可被设置为 1 字节。`Concurrency` 字段在配置中定义但从未被使用。

**建议**: 统一配置验证到一处，添加合理的下限检查。

---

#### M-6: 引擎 `Run` 方法的 `defer` 发送最终更新可能触发重复推送

**文件**: `tracert_engine.go:108-127`

```go
defer func() {
    // ...
    if opts.OnUpdate != nil {
        finalReachedTTL := atomic.LoadInt32(&progress.MinReachedTTL)
        filteredProgress := progress.CloneForDisplay(finalReachedTTL)
        opts.OnUpdate(filteredProgress)
    }
}()
```

在 `runRound` 结束时已发送了一次过滤后的进度（第 378-383 行），`defer` 又发送一次。对于单轮单 round 的场景，前端会收到两次完成通知。

**建议**: 在 `defer` 中检查是否已发送过最终进度，或统一由 `defer` 作为唯一的最终推送点。

---

### 🟢 低危 (Low)

#### L-1: `log.Printf` 与 `logger` 日志系统混用

**文件**: 多处（`tracert_service.go:303, 313, 378` 等, `tracert_engine.go:70, 119, 140, 145` 等）

大量使用 `log.Printf("[TRACERT SVC]...")` 的标准库日志，与项目的 `logger` 系统并存。这些 `log.Printf` 调用绕过了 logger 的日志级别控制。

**建议**: 将所有 `log.Printf` 调试日志迁移到 `logger.Debug`，生产环境移除或降级。

---

#### L-2: `Concurrency` 配置字段未使用

**文件**: `types.go:267`, `tracert_engine.go:69`

`TracertConfig.Concurrency` 字段在日志中记录了值（第 69 行），但探测逻辑中并未使用——所有 TTL 始终全量并发启动。

**建议**: 要么实现并发控制（使用 semaphore 限制并发 goroutine 数），要么移除该字段避免混淆。

---

#### L-3: 缺少 Tracert 专用单元测试

**现状**: `engine_test.go` 仅包含 Ping 相关测试，无任何 Tracert 测试。`tracert_service.go` 也无测试文件。

**影响**: 合并逻辑、状态转换、边界条件（如 MaxHops=1, Count=1000000）等关键路径未经自动化验证。

**建议**: 补充以下测试用例：
- `mergeHopResult` 的多轮累积统计正确性
- `CloneForDisplay` 的过滤边界
- `mergeRoundResult` 的 MinReachedTTL 更新逻辑
- 取消场景下的状态一致性

---

#### L-4: `pingOneWithTTLWindows` 中 Address Mismatch 检查在 TTL Expired 场景下有误

**文件**: `icmp_windows.go:448-460`

```go
if reply.Status == IP_SUCCESS {
    if reply.Address != destAddr {
        // Address Mismatch
    }
```

当 `reply.Status == IP_SUCCESS` 时检查 Address Mismatch 是正确的。但注意在 tracert 场景下，中间路由器的响应 `reply.Address` 与 `destAddr` **天然不同**——这个逻辑在 `IP_TTL_EXPIRED_TRANSIT` 分支（第 464 行）被正确处理。此处无 bug，但设计意图可更清晰。

---

#### L-5: Raw Socket 后端的 RTT 精度较低

**文件**: `icmp_raw.go:152, 169`

```go
rtt := time.Since(sendTime).Milliseconds()
```

使用 `Milliseconds()` 返回整数毫秒，丢失亚毫秒精度。而 Windows 后端从 `reply.RoundTripTime`（uint32 毫秒）获取 RTT 同样是整数精度。两个后端一致，但对于局域网场景（RTT < 1ms），显示精度不足。

**建议**: 使用 `time.Since(sendTime).Seconds() * 1000` 获取浮点毫秒值。

---

## 三、安全相关

| 项目 | 状态 | 说明 |
|------|------|------|
| 输入验证 | ✅ 基本覆盖 | Target 做了 TrimSpace 和空值检查；Config 有范围约束 |
| 注入攻击 | ✅ 安全 | Target 通过 `net.ParseIP` / `net.LookupIP` 处理，不拼接到命令行 |
| 权限管理 | ⚠️ 需注意 | UAC 提权后 `os.Exit(0)`，不会返回到调用者——如果 `RequestElevation` 被非 init 代码调用需注意 |
| 资源泄漏 | ⚠️ 需注意 | ICMP Handle 泄漏风险低（有 defer close），但 goroutine 泄漏风险见 H-1 |
| DoS 防护 | ⚠️ 需改进 | `Count` 上限为 100 万，`MaxHops` 上限 255——极端配置可长时间占用系统资源 |

---

## 四、问题汇总

| 编号 | 严重性 | 类别 | 描述 |
|------|--------|------|------|
| C-1 | 🔴 严重 | 并发 | `runContinuous` 无锁访问 `s.progress` |
| C-2 | 🔴 严重 | 并发 | `resolveHopHostNames` 与外部共享切片竞争 |
| C-3 | 🔴 严重 | 并发 | `OnUpdate` 回调中锁间隙与无锁读取 |
| H-1 | 🟠 高危 | 逻辑 | `break` 仅跳出 select 导致取消失效 |
| H-2 | 🟠 高危 | 设计 | atomic 与 Mutex 混用语义混乱 |
| H-3 | 🟠 高危 | 并发 | Stop 后快速 Start 可能并发运行 |
| H-4 | 🟠 高危 | 性能 | ICMP Handle 每次创建/销毁 |
| M-1 | 🟡 中危 | 正确性 | UTF-8 字符截断 |
| M-2 | 🟡 中危 | 资源 | DNS 缓存无容量上限 |
| M-3 | 🟡 中危 | 并发 | `stopDNSCacheCleanup` 非幂等 |
| M-4 | 🟡 中危 | 冗余 | 导出功能双重过滤 |
| M-5 | 🟡 中危 | 设计 | 配置验证重复且不一致 |
| M-6 | 🟡 中危 | 冗余 | defer 重复推送最终进度 |
| L-1 | 🟢 低危 | 规范 | log.Printf 与 logger 混用 |
| L-2 | 🟢 低危 | 冗余 | Concurrency 字段未使用 |
| L-3 | 🟢 低危 | 质量 | 缺少 Tracert 单元测试 |
| L-4 | 🟢 低危 | 可读性 | Address Mismatch 检查意图可更清晰 |
| L-5 | 🟢 低危 | 精度 | Raw Socket RTT 精度不足 |

---

## 五、优先修复建议

1. **立即修复** (C-1, C-2, C-3): 统一 `s.progress` 的并发访问策略——所有读写都通过 `progressMu` 保护，传递给异步操作前先 Clone
2. **尽快修复** (H-1): 将 `break` 改为带标签的 break 或 goto
3. **尽快修复** (H-3): `StopTracert` 应阻塞等待 goroutine 完全退出后再返回
4. **计划修复** (H-2, H-4): 统一同步策略；考虑 ICMP Handle 池化
5. **持续改进** (M/L 级别): 补充测试、清理冗余代码、统一日志
