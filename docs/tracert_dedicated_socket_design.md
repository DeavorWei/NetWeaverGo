# Tracert 专属 Socket 组架构重构设计方案

> **文档版本**: 1.0  
> **更新日期**: 2026-05-18  
> **主题**: 基于 TTL-Dedicated Sockets（按 TTL 绑定的专属 Socket 组）的 Tracert 性能优化与重构方案

---

## 1. 背景与问题陈述

在目前的 NetWeaverGo Tracert 实现中，探针发送采用了“短连接”模式：每次发送一跳（Hop）的探测包时，都会瞬间请求操作系统创建一个新的 Raw Socket，设置对应的 TTL，发送接收完毕后再关闭该 Socket。

这种模式在单次、串行探测时没有问题。但在**持续并发探测（Continuous Tracert）**的场景下暴露出严重的缺陷：
1. **系统开销大**：每秒钟可能会发生数十到数百次的内核态 Socket 创建与销毁。
2. **底层崩溃风险**：极高的频率瞬间挤压底层 Netpoller，引发了 `sync.Pool` 内存分配损坏和段错误（ACCESS_VIOLATION）。
3. **不可使用常规连接池**：如果使用常规的连接池复用 Socket，由于每次发送都需要调用 `SetTTL()` 改变 IP 头部，多协程并发复用会导致 TTL 互相覆盖（数据竞争），导致探测结果完全错乱。

## 2. 架构设计核心思想

本方案采用 **TTL-Dedicated Sockets（按 TTL 绑定的专属 Socket 组）** 架构。

**核心思想**：
* **零动态分配**：在一次 Tracert 任务的生命周期内（不论持续多少轮），仅在启动时创建必要数量的 Socket，期间不销毁、不新建。
* **专属绑定**：根据最大跳数（MaxHops，如 30），直接创建 30 个独立的 Raw Socket。
* **提前固化**：在创建完成后，立即将 `sockets[1]` 的 TTL 设置为 1，`sockets[2]` 的 TTL 设置为 2，依次类推，此后不再修改。
* **物理隔离**：当执行某一轮（Round）的探测时，`TTL=i` 的探测协程**永远只读写 `sockets[i]`**。这样既实现了 Socket 重用，又通过物理隔离完美避开了多个 Goroutine 并发争抢同一个 Socket 导致的 TTL 竞争问题。

## 3. 具体重构步骤

### 3.1 阶段一：Engine 结构扩展与生命周期管理

在 `internal/icmp/tracert_engine.go` 中，为 `TracertEngine` 增加专属连接池字段：

```go
type TracertEngine struct {
    config    TracertConfig
    cancel    context.CancelFunc
    running   bool
    runningMu sync.RWMutex
    
    // 新增字段：专属 Socket 组。索引为对应的 TTL，大小为 maxHops + 1
    sockets   []*icmp.PacketConn 
}
```

在 `Run()` 方法执行实际的探测循环之前，进行初始化；在 `Run()` 结束时，统一清理：

```go
func (e *TracertEngine) Run(...) *TracertProgress {
    // ... 前置准备 ...

    // 1. 初始化专属 Socket 组
    err := e.initSockets()
    if err != nil {
        // 处理初始化失败（权限不足或系统资源枯竭）
        return progress
    }

    // 2. 确保任务结束时关闭所有 Socket
    defer e.closeSockets()

    // ... 执行持续探测的 for 循环 ...
}
```

### 3.2 阶段二：连接的初始化与销毁逻辑

```go
func (e *TracertEngine) initSockets() error {
    e.sockets = make([]*icmp.PacketConn, e.config.MaxHops+1)
    
    for i := 1; i <= e.config.MaxHops; i++ {
        conn, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
        if err != nil {
            e.closeSockets() // 发生错误，清理已创建的部分
            return fmt.Errorf("创建 TTL=%d 的 Socket 失败: %w", i, err)
        }
        
        // 提前固化 TTL
        err = conn.IPv4PacketConn().SetTTL(i)
        if err != nil {
            e.closeSockets()
            return fmt.Errorf("设置 TTL=%d 失败: %w", i, err)
        }
        
        e.sockets[i] = conn
    }
    return nil
}

func (e *TracertEngine) closeSockets() {
    if e.sockets == nil {
        return
    }
    for i := 1; i < len(e.sockets); i++ {
        if e.sockets[i] != nil {
            e.sockets[i].Close()
            e.sockets[i] = nil
        }
    }
}
```

### 3.3 阶段三：探测底层逻辑的适配

当前的 `PingOneWithTTL` 函数（位于 `icmp_raw.go`）属于无状态的单次工具函数，我们需要改造为**使用已有的连接发送和接收**：

我们需要在底层（或引擎层面）实现一个新的 `probeHopWithConn` 方法：
1. **提取对应的专属连接**：`conn := e.sockets[ttl]`
2. **构建 ICMP 报文**：生成带有特定 `ID`（如进程 ID）和自增 `Seq` 的 ICMP Echo Request 报文。
3. **发送**：调用 `conn.WriteTo(wb, dst)`。
4. **设置读取超时**：调用 `conn.SetReadDeadline(time.Now().Add(timeout))`。
5. **阻塞接收**：调用 `conn.ReadFrom(rb)` 循环读取。

### 3.4 阶段四：处理“脏包（幽灵报文）”的极高风险点

这是复用长连接必须面对的核心痛点。

**场景描述**：
在第 1 轮探测中，`TTL=5` 的请求发出去后超时了，探测协程 5 退出。紧接着马上开始第 2 轮探测，探测协程 5 刚刚把第 2 轮的请求用 `sockets[5]` 发出去，结果这个时候，第 1 轮迟到的响应报文回来了，正好被 `sockets[5]` 的 `ReadFrom` 读到。如果直接当作有效报文，测出的 RTT 会极度荒谬，甚至 IP 也是错的。

**防范方案（核心必做）**：
复用连接后，`ReadFrom` 收到的报文**绝对不能只看 IP**，必须严格执行包的特征校验：
1. **生成全局唯一的序列号**：
   每一轮探测中的每一跳，生成的 ICMP Echo `Seq` 必须是**全局递增且唯一**的。
2. **严格过滤机制**：
   `ReadFrom` 解析出的 `ICMP Message`：
   * 必须校验 `Echo.ID == currentProcessID`。
   * 必须校验 `Echo.Seq == currentExpectedSeq`。
   * 对于 `TimeExceeded`（TTL 过期报文），由于它封装了出错的原始请求头，必须剥离其载荷，读取其中内嵌的 `OrigID` 和 `OrigSeq` 并与当前发送的一致，否则立刻丢弃并继续 `ReadFrom`。

## 4. 异常处理与系统边界

1. **并发支持**：此方案仅隔离了一个 Tracert 任务内部的 TTL 冲突。如果用户通过界面同时启动 3 个针对不同目标的 Tracert 任务，将会开启 `3 * 30 = 90` 个 Raw Socket。这是完全可以接受的（系统句柄上限往往在数万），无需担心。
2. **系统权限保护**：如果 `initSockets` 阶段第一个 Socket 就由于需要 Administrator/Root 权限报错，应立即将友好的错误信息抛给 UI 侧。
3. **跨平台兼容性**：在非 Windows 平台上，普通用户可能可以使用非特权的 UDP ICMP（`"udp4:icmp"` 或 ping socket），但 Windows 必须使用 Raw Socket（需要管理员提权）。该逻辑应该继续由平台编译标签（`//go:build windows` 等）在底层处理，上层引擎不应强耦合具体的网络协议字符串。

## 5. 预期优化收益

| 指标 | 优化前（原实现） | 优化后（专属 Socket 组） | 提升幅度 |
| --- | --- | --- | --- |
| **内核态上下文切换** | 极高（每次发送1次创建，1次销毁） | 趋近于零 | 极大地降低 CPU 使用率 |
| **底层 Netpoller 负荷** | 过载，极易诱发致命的内存踩踏 | 极低且恒定 | **100% 解决 0xc0000005 崩溃漏洞** |
| **TTL 竞争覆盖风险** | 存在（如果采用公共池） | 不存在（彻底物理隔离） | 理论上完美避开数据污染 |
| **资源句柄占用** | 频繁激增和跌落 | 恒定的 30 个 | 让操作系统的调度更平稳 |

## 6. 后续实施建议

1. **第一步**：抽离 `icmp_raw.go` 中的发送和接收逻辑，将其从高度耦合的 `pingOneRaw` 函数中拆分为 `SendRequest(conn, ...)` 和 `WaitReply(conn, expectedSeq, ...)` 两部分，方便引擎复用。
2. **第二步**：修改 `TracertEngine.Run` 生命周期，引入 `initSockets` 和 `closeSockets`。
3. **第三步**：使用专门的网络测试工具（如 Wireshark 或者网络损伤模拟工具）注入延迟，验证防范“脏包”的 `Seq` 校验机制是否坚不可摧。
