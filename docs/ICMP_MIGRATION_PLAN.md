# ICMP 引擎迁移实施方案

> **版本**: v2.0
> **日期**: 2026-05-03
> **目标**: 为 `internal/icmp/` 新增基于 `golang.org/x/net/icmp` 的跨平台发包实现，与现有 Windows `iphlpapi.dll` 实现永久并行共存
> **状态**: 待实施
> **核心原则**: **双实现永久并行**，不删除 Windows API 实现，通过构建标签和运行时策略选择

---

## 目录

- [1. 迁移背景与目标](#1-迁移背景与目标)
- [2. 现有实现深度分析](#2-现有实现深度分析)
  - [2.1 文件结构总览](#21-文件结构总览)
  - [2.2 Windows API 调用链](#22-windows-api-调用链)
  - [2.3 关键数据结构](#23-关键数据结构)
  - [2.4 PingOne 发包流程](#24-pingone-发包流程)
  - [2.5 PingOneWithTTL 发包流程](#25-pingonewithttl-发包流程)
  - [2.6 批量Ping引擎调用链](#26-批量ping引擎调用链)
  - [2.7 Traceroute引擎调用链](#27-traceroute引擎调用链)
  - [2.8 关键技术细节汇总](#28-关键技术细节汇总)
- [3. 新旧架构对比](#3-新旧架构对比)
- [4. 双实现并行架构设计](#4-双实现并行架构设计)
  - [4.1 架构总览](#41-架构总览)
  - [4.2 实现选择策略](#42-实现选择策略)
  - [4.3 构建标签规则](#43-构建标签规则)
  - [4.4 迁移分阶段计划](#44-迁移分阶段计划)
- [5. 详细代码设计](#5-详细代码设计)
  - [5.1 接口抽象层 icmp_backend.go](#51-接口抽象层-icmp_backendgo)
  - [5.2 Windows 后端 icmp_windows.go](#52-windows-后端-icmp_windowsgo)
  - [5.3 Raw Socket 后端 icmp_raw.go](#53-raw-socket-后端-icmp_rawgo)
  - [5.4 后端选择器 icmp_selector.go](#54-后端选择器-icmp_selectorgo)
  - [5.5 连接管理器设计](#55-连接管理器设计)
  - [5.6 ICMP消息构造与解析](#56-icmp消息构造与解析)
  - [5.7 TTL设置与Traceroute支持](#57-ttl设置与traceroute支持)
  - [5.8 状态码映射](#58-状态码映射)
  - [5.9 响应匹配与防伪](#59-响应匹配与防伪)
  - [5.10 权限检测机制](#510-权限检测机制)
- [6. 测试方案](#6-测试方案)
- [7. 风险分析与缓解措施](#7-风险分析与缓解措施)
- [8. 实施步骤清单](#8-实施步骤清单)
- [9. 代码变更量估算](#9-代码变更量估算)
- [10. 验收标准](#10-验收标准)

---

## 1. 迁移背景与目标

### 1.1 迁移背景

当前 `internal/icmp/` 模块通过 Go 的 `syscall` 包直接调用 Windows `iphlpapi.dll` 的三个原生 API（`IcmpCreateFile`、`IcmpSendEcho`、`IcmpCloseHandle`）实现 ICMP Echo 发包。该方案存在以下局限：

1. **平台锁定**：仅支持 Windows，无法在 Linux/macOS 上运行
2. **unsafe 依赖**：使用 5 处 `unsafe.Pointer` 操作原始内存缓冲区
3. **结构体对齐脆弱**：依赖 32 位对齐的 `IP_OPTION_INFORMATION32` 和 `ICMP_ECHO_REPLY`，跨架构风险
4. **缓冲区手动计算**：需手动计算回复缓冲区大小，含 padding 和对齐逻辑
5. **字节序转换**：需手动处理 `binary.LittleEndian` 与 `in_addr` 结构的转换
6. **无IPv6支持**：Windows ICMP API 仅支持 IPv4

但 Windows API 方案也有其**不可替代的优势**：

1. **无需管理员权限**：`iphlpapi.dll` 是用户态 API，普通用户即可使用
2. **内核栈处理**：封包/解包由 Windows 内核完成，稳定可靠
3. **RTT 精确**：内核级计时，无应用层延迟
4. **生产验证**：已通过大量测试和实际使用验证

### 1.2 迁移目标

| 目标         | 优先级   | 说明                                                        |
| ------------ | -------- | ----------------------------------------------------------- |
| 跨平台支持   | 高       | 新增 Linux/macOS 支持                                       |
| 双实现并行   | **最高** | **Windows API 实现永久保留**，raw socket 实现作为跨平台补充 |
| 保持接口兼容 | 高       | `PingOne` / `PingOneWithTTL` 签名不变，引擎层零改动         |
| 保持功能等价 | 高       | 两种后端的功能行为完全一致                                  |
| 运行时可选   | 高       | 支持构建时和运行时切换后端实现                              |
| IPv6 支持    | 低       | 后续扩展（仅 raw socket 后端）                              |

---

## 2. 现有实现深度分析

### 2.1 文件结构总览

```
internal/icmp/
├── types.go              # 类型定义（平台无关，无构建标签）
├── icmp_windows.go       # Windows ICMP API 封装（核心底层）  //go:build windows
├── icmp_windows_test.go  # Windows ICMP 测试                 //go:build windows
├── engine.go             # 批量Ping引擎                      //go:build windows
├── engine_test.go        # 引擎测试                          //go:build windows
└── tracert_engine.go     # Traceroute引擎                   //go:build windows
```

> **注意**：`engine.go` 和 `tracert_engine.go` 本身在逻辑上是平台无关的，但标注了 `//go:build windows` 是因为它们调用的 `PingOne`/`PingOneWithTTL` 仅在 `icmp_windows.go` 中实现。

### 2.2 Windows API 调用链

**文件**：`internal/icmp/icmp_windows.go`

```go
var (
    iphlpapi           = syscall.NewLazyDLL("iphlpapi.dll")
    procIcmpCreateFile  = iphlpapi.NewProc("IcmpCreateFile")
    procIcmpSendEcho    = iphlpapi.NewProc("IcmpSendEcho")
    procIcmpCloseHandle = iphlpapi.NewProc("IcmpCloseHandle")
)
```

| API 函数          | DLL          | 作用                                  | Go 封装位置           |
| ----------------- | ------------ | ------------------------------------- | --------------------- |
| `IcmpCreateFile`  | iphlpapi.dll | 创建 ICMP 句柄，返回 `syscall.Handle` | `icmp_windows.go:126` |
| `IcmpSendEcho`    | iphlpapi.dll | 同步发送 ICMP Echo 并等待回复         | `icmp_windows.go:150` |
| `IcmpCloseHandle` | iphlpapi.dll | 关闭 ICMP 句柄                        | `icmp_windows.go:138` |

**每次 Ping 调用序列**：

1. `IcmpCreateFile()` → 获取句柄
2. 构造发送数据（`prepareSendData`）
3. 计算回复缓冲区大小并分配
4. `IcmpSendEcho(handle, destAddr, sendData, options, replyBuffer, timeout)` → 同步等待
5. 解析 `ICMP_ECHO_REPLY` 结构体
6. `IcmpCloseHandle(handle)` → 释放句柄

### 2.3 关键数据结构

#### IP_OPTION_INFORMATION32（发送选项）

```go
// icmp_windows.go:49
type IP_OPTION_INFORMATION32 struct {
    TTL         uint8  // 生存时间
    Tos         uint8  // 服务类型
    Flags       uint8  // IP 标志位
    OptionsSize uint8  // 选项数据大小
    OptionsData uint32 // 选项数据指针（必须用 uint32，不能用 uintptr）
}
```

#### ICMP_ECHO_REPLY（接收响应）

```go
// icmp_windows.go:59
type ICMP_ECHO_REPLY struct {
    Address       uint32 // 响应地址（网络字节序，小端存储）
    Status        uint32 // ICMP 状态码
    RoundTripTime uint32 // 往返时间（毫秒）
    DataSize      uint16 // 返回数据大小
    Reserved      uint16 // 保留字段
    DataPointer   uint32 // 数据指针（必须用 uint32，不能用 uintptr）
    Options       IP_OPTION_INFORMATION32
}
```

#### ICMP 状态码常量

```go
// icmp_windows.go:17-44
const (
    IP_SUCCESS              = 0
    IP_BUF_TOO_SMALL        = 11001
    IP_DEST_NET_UNREACHABLE = 11002
    IP_DEST_HOST_UNREACHABLE = 11003
    IP_DEST_PROT_UNREACHABLE = 11004
    IP_DEST_PORT_UNREACHABLE = 11005
    IP_TTL_EXPIRED_TRANSIT  = 11013  // Traceroute 核心
    IP_TTL_EXPIRED_REASSEM  = 11014
    IP_REQ_TIMED_OUT        = 11010
    IP_GENERAL_FAILURE      = 11050
    // ... 共 20 种状态码
)
```

#### PingResult（公开返回类型）

```go
// types.go:10
type PingResult struct {
    IP            string  // 目标 IP 地址
    Success       bool    // 是否成功
    RoundTripTime float64 // 往返时间（毫秒）
    TTL           uint8   // 生存时间
    Status        string  // 状态描述
    Error         string  // 错误信息
}
```

### 2.4 PingOne 发包流程

**函数签名**：`func PingOne(ip net.IP, timeout uint32, dataSize uint16) (*PingResult, error)`

**位置**：`icmp_windows.go:265`

```
PingOne(ip, timeout, dataSize)
│
├─ 1. 参数校验：dataSize=0 时修正为 32
├─ 2. IP 转换：ip.To4()，失败则返回 "invalid IPv4 address"
├─ 3. 创建句柄：IcmpCreateFile()
│     └─ defer IcmpCloseHandle(handle)
├─ 4. 构造发送数据：prepareSendData(dataSize)
│     ├─ 前 8 字节：time.Now().UnixNano() 时间戳
│     └─ 剩余字节：A-Z 字母循环填充（模拟 Windows ping 模式）
├─ 5. 地址转换：destAddr = binary.LittleEndian.Uint32(ip)
├─ 6. 发送请求：IcmpSendEcho(handle, destAddr, sendData, timeout, ttl=128)
│     ├─ 缓冲区计算：sizeof(REPLY) + data + 8 + 128(padding), 8字节对齐, 最小256
│     └─ syscall 调用
├─ 7. 错误处理：ret==0 时解析 replyBuffer 中的状态码
├─ 8. 解析响应：reply = (*ICMP_ECHO_REPLY)(unsafe.Pointer(&replyBuffer[0]))
│     ├─ RoundTripTime = reply.RoundTripTime
│     ├─ TTL = reply.Options.TTL
│     └─ 提取 replyData
├─ 9. 状态判断：
│     ├─ IP_SUCCESS → 校验 reply.Address == destAddr（防交叉投递）
│     └─ 非 IP_SUCCESS → icmpStatusToString(status)
└─ 10. 返回 *PingResult
```

### 2.5 PingOneWithTTL 发包流程

**函数签名**：`func PingOneWithTTL(ip net.IP, timeout uint32, dataSize uint16, ttl uint8) (*PingResult, error)`

**位置**：`icmp_windows.go:361`

与 `PingOne` 基本一致，关键差异：

- **步骤 6**：`IcmpSendEcho(handle, destAddr, sendData, timeout, ttl)` — TTL 可指定
- **步骤 9**：增加 TTL 过期处理
  - `IP_TTL_EXPIRED_TRANSIT` → `Status="TTLExpired"`, `IP=中间路由器`
  - `IP_TTL_EXPIRED_REASSEM` → 同上

### 2.6 批量Ping引擎调用链

```
前端 BatchPing.vue
  → PingService.StartBatchPing()         [internal/ui/ping_service.go]
    → BatchPingEngine.RunWithOptions(ctx, ips, opts)
      ├─ 创建可取消 context
      ├─ 初始化 BatchPingProgress
      ├─ 信号量并发控制
      └─ 对每个 IP 启动 goroutine：
          └─ pingHostWithOptions(ctx, ip, index, ...)
              └─ 循环 config.Count 次：
                  ├─ PingOne(ip, config.Timeout, config.DataSize)  ← 底层调用
                  ├─ 聚合 RTT 统计（Min/Max/Avg）
                  ├─ 触发 OnSinglePing / OnHostUpdate 回调
                  └─ 间隔等待 config.Interval
```

### 2.7 Traceroute引擎调用链

```
前端 Tracert.vue
  → TracertService.StartTracert()        [internal/ui/tracert_service.go]
    → TracertEngine.Run(ctx, target, opts)
      ├─ resolveTarget(target)  // DNS 解析
      └─ 循环 config.Count 轮：
          └─ runRound(ctx, resolvedIP, progress, opts)
              ├─ 对 TTL=1..maxHops 全量并发启动 goroutine：
              │   └─ probeHop(ctx, destIP, ttl, opts)
              │       └─ PingOneWithTTL(destIP, timeout, dataSize, ttl)  ← 底层调用
              ├─ 通过 resultChan 串行收集结果
              └─ 实时触发 OnUpdate / OnHopUpdate 回调
```

### 2.8 关键技术细节汇总

| 特性         | 当前实现                                                   | 涉及位置                              |
| ------------ | ---------------------------------------------------------- | ------------------------------------- |
| 权限要求     | 无需管理员权限                                             | iphlpapi.dll 是用户态 API             |
| 协议栈       | Windows 内核 ICMP 栈处理封包/解包                          | IcmpSendEcho                          |
| 缓冲区计算   | 手动：`sizeof(REPLY) + data + 8 + 128`, 8字节对齐, 最小256 | `icmp_windows.go:163-180`             |
| 字节序转换   | `binary.LittleEndian.Uint32(ip)` → in_addr                 | `icmp_windows.go:298`                 |
| 响应地址校验 | `reply.Address != destAddr` 防止交叉投递                   | `icmp_windows.go:336,431`             |
| 64位兼容     | 结构体使用 uint32 而非 uintptr                             | `icmp_windows.go:49-67`               |
| unsafe 使用  | 5 处 `unsafe.Pointer`                                      | `icmp_windows.go:168,187,193,204,219` |
| 句柄模式     | 每次请求创建/关闭句柄                                      | `icmp_windows.go:282-287`             |
| 数据填充     | 8字节时间戳 + A-Z字母循环                                  | `icmp_windows.go:242-262`             |
| 状态码       | 20+种 Windows ICMP 状态码                                  | `icmp_windows.go:17-44`               |
| 并发安全     | 无共享状态（每次请求独立句柄）                             | 天然安全                              |

---

## 3. 新旧架构对比

| 维度         | Windows 后端 (iphlpapi.dll)      | Raw Socket 后端 (golang.org/x/net/icmp) |
| ------------ | -------------------------------- | --------------------------------------- |
| **平台**     | Windows 专用                     | 跨平台（Windows/Linux/macOS）           |
| **权限**     | 无需管理员 ✅                    | 需要 root/administrator ❌              |
| **封包**     | Windows 内核处理                 | 应用层构造 ICMP 消息                    |
| **协议**     | 仅 IPv4                          | IPv4 + IPv6                             |
| **依赖**     | syscall + unsafe                 | golang.org/x/net/icmp                   |
| **句柄管理** | IcmpCreateFile / IcmpCloseHandle | icmp.ListenPacket / conn.Close          |
| **TTL 设置** | IP_OPTION_INFORMATION.TTL        | ipv4.NewPacketConn(conn).SetTTL()       |
| **RTT 来源** | 内核返回 reply.RoundTripTime     | 应用层 time.Since(sendTime)             |
| **缓冲区**   | 手动计算大小 + unsafe.Pointer    | ReadFrom(rb) 固定 1500 字节             |
| **unsafe**   | 5 处                             | 0 处                                    |
| **生产验证** | 已验证 ✅                        | 待验证                                  |
| **并发模型** | 每次请求独立句柄                 | 连接复用 + ID/Seq 匹配                  |

---

## 4. 双实现并行架构设计

### 4.1 架构总览

```
                    ┌─────────────────────────────────┐
                    │     engine.go / tracert_engine.go│  ← 引擎层（不改动）
                    │   调用 PingOne / PingOneWithTTL  │
                    └──────────────┬──────────────────┘
                                   │
                    ┌──────────────┴──────────────────┐
                    │     icmp_backend.go (接口抽象)    │  ← 新增
                    │  ICMPSender 接口定义              │
                    │  PingOne / PingOneWithTTL 签名    │
                    └──────────────┬──────────────────┘
                                   │
              ┌────────────────────┼────────────────────┐
              │                    │                     │
   ┌──────────┴──────────┐  ┌─────┴──────┐  ┌──────────┴──────────┐
   │  icmp_windows.go    │  │ icmp_raw.go│  │ icmp_selector.go    │
   │  Windows API 后端   │  │ Raw Socket │  │ 后端选择器          │
   │  (永久保留)         │  │ 跨平台后端  │  │ 运行时/构建时选择   │
   │  //go:build windows │  │ //go:build │  │ //go:build !windows │
   │  && !rawicmp        │  │ rawicmp    │  │ || rawicmp          │
   └─────────────────────┘  └────────────┘  └─────────────────────┘
```

### 4.2 实现选择策略

采用**三层选择策略**，优先级从高到低：

| 优先级 | 选择方式       | 场景                     | 说明                                               |
| ------ | -------------- | ------------------------ | -------------------------------------------------- |
| 1      | **构建标签**   | `go build -tags rawicmp` | 编译时强制选择后端                                 |
| 2      | **运行时配置** | `SetBackend(RawSocket)`  | 程序启动时指定                                     |
| 3      | **自动检测**   | 默认行为                 | Windows 默认 iphlpapi；Linux/macOS 默认 raw socket |

**自动检测逻辑**：

```
程序启动
  ├─ GOOS=windows && 未指定 rawicmp 标签
  │   └─ 默认使用 iphlpapi 后端（无需管理员权限）
  ├─ GOOS=windows && 指定 rawicmp 标签
  │   └─ 使用 raw socket 后端（需管理员权限）
  ├─ GOOS=linux || GOOS=darwin
  │   └─ 使用 raw socket 后端（唯一可用）
  └─ 运行时调用 SetBackend() 切换
      ├─ 切换到 RawSocket → 检测权限，失败则返回错误
      └─ 切换到 WindowsAPI → 仅 Windows 可用
```

### 4.3 构建标签规则

| 文件                       | 当前标签             | 新标签                           | 说明                          |
| -------------------------- | -------------------- | -------------------------------- | ----------------------------- | -------- | -------------------------------- |
| `icmp_backend.go`          | （新建）             | 无标签（所有平台编译）           | 接口定义 + 公开入口函数       |
| `icmp_windows.go`          | `//go:build windows` | `//go:build windows && !rawicmp` | Windows API 后端，永久保留    |
| `icmp_raw.go`              | （新建）             | `//go:build !windows             |                               | rawicmp` | Raw socket 后端                  |
| `icmp_selector_default.go` | （新建）             | `//go:build windows && !rawicmp` | 默认选择器：选择 Windows 后端 |
| `icmp_selector_raw.go`     | （新建）             | `//go:build !windows             |                               | rawicmp` | 默认选择器：选择 Raw socket 后端 |
| `engine.go`                | `//go:build windows` | 无标签                           | 引擎层变为平台无关            |
| `tracert_engine.go`        | `//go:build windows` | 无标签                           | 同上                          |
| `types.go`                 | 无标签               | 无标签                           | 不变                          |

**编译效果**：

```bash
# Windows 默认编译 → 使用 iphlpapi（无需管理员）
go build ./...

# Windows + rawicmp 标签 → 使用 raw socket（需管理员）
go build -tags rawicmp ./...

# Linux → 自动使用 raw socket
GOOS=linux go build ./...

# macOS → 自动使用 raw socket
GOOS=darwin go build ./...
```

### 4.4 迁移分阶段计划

#### 阶段一：接口抽象 + Raw Socket 后端实现

**优先级**：高
**目标**：建立接口抽象层，实现 raw socket 后端，双实现并行可用

| 文件                                     | 操作     | 说明                                                                          |
| ---------------------------------------- | -------- | ----------------------------------------------------------------------------- |
| `internal/icmp/icmp_backend.go`          | **新建** | 接口定义 + 公开入口函数（委托给选定后端）                                     |
| `internal/icmp/icmp_raw.go`              | **新建** | 基于 golang.org/x/net/icmp 的跨平台实现                                       |
| `internal/icmp/icmp_selector_default.go` | **新建** | Windows 默认选择器                                                            |
| `internal/icmp/icmp_selector_raw.go`     | **新建** | Raw socket 选择器                                                             |
| `internal/icmp/icmp_windows.go`          | **修改** | 添加 `&& !rawicmp` 构建标签；函数改为内部实现（小写化或保留大写由选择器调用） |
| `internal/icmp/engine.go`                | **修改** | 移除 `//go:build windows` 标签                                                |
| `internal/icmp/tracert_engine.go`        | **修改** | 移除 `//go:build windows` 标签                                                |
| `go.mod`                                 | **修改** | 添加 `golang.org/x/net` 依赖                                                  |

#### 阶段二：连接生命周期管理

**优先级**：高
**目标**：优化 raw socket 后端的连接管理

| 当前模式                                             | 迁移后模式                            |
| ---------------------------------------------------- | ------------------------------------- |
| 每次 Ping 创建/关闭 IcmpCreateFile / IcmpCloseHandle | 全局连接复用（raw socket 可长期持有） |
| 句柄生命周期 = 单次请求                              | 连接生命周期 = 后端生命周期           |

**变更**：

- 新增 `icmp_conn.go`：连接管理器，负责创建/复用/关闭 `icmp.PacketConn`
- 连接管理器内置健康检查：连接异常时自动重建
- Windows 后端不受影响（保持原有句柄模式）

#### 阶段三：运行时后端切换

**优先级**：中
**目标**：支持运行时动态切换后端实现

**变更**：

- `icmp_backend.go` 添加 `SetBackend(BackendType)` 函数
- `BackendType` 枚举：`BackendAuto` / `BackendWindowsAPI` / `BackendRawSocket`
- 切换时关闭旧后端连接，初始化新后端
- 添加 `GetBackend() BackendType` 查询当前后端
- UI 设置页添加"ICMP 后端"选择

#### 阶段四：扩展IPv6支持

**优先级**：低
**目标**：利用 golang.org/x/net/icmp 的 IPv6 能力

- 仅 raw socket 后端支持 IPv6（Windows API 不支持）
- 新增 `PingOneV6(ip net.IP, timeout uint32, dataSize uint16)`
- `TracertEngine` 增加 IPv6 目标支持
- 前端增加 IPv6 配置选项

---

## 5. 详细代码设计

### 5.1 接口抽象层 icmp_backend.go

```go
// icmp_backend.go — 无构建标签，所有平台编译
package icmp

import "net"

// BackendType 表示 ICMP 后端实现类型
type BackendType int

const (
    BackendAuto       BackendType = iota // 自动选择（默认）
    BackendWindowsAPI                     // Windows iphlpapi.dll
    BackendRawSocket                      // golang.org/x/net/icmp
)

// icmpBackend ICMP 后端接口（内部接口，不导出）
type icmpBackend interface {
    // pingOne 执行单次 ICMP Echo 请求
    pingOne(ip net.IP, timeout uint32, dataSize uint16) (*PingResult, error)
    // pingOneWithTTL 执行带指定 TTL 的 ICMP Echo 请求
    pingOneWithTTL(ip net.IP, timeout uint32, dataSize uint16, ttl uint8) (*PingResult, error)
    // close 关闭后端资源
    close() error
    // name 返回后端名称
    name() string
}

// 全局后端实例
var (
    currentBackend icmpBackend
    backendType    BackendType = BackendAuto
)

// PingOne 执行单次 ICMP Echo 请求（公开接口，委托给当前后端）
func PingOne(ip net.IP, timeout uint32, dataSize uint16) (*PingResult, error) {
    return currentBackend.pingOne(ip, timeout, dataSize)
}

// PingOneWithTTL 执行带指定 TTL 的 ICMP Echo 请求（公开接口，委托给当前后端）
func PingOneWithTTL(ip net.IP, timeout uint32, dataSize uint16, ttl uint8) (*PingResult, error) {
    return currentBackend.pingOneWithTTL(ip, timeout, dataSize, ttl)
}

// GetBackend 返回当前后端类型
func GetBackend() BackendType {
    return backendType
}

// GetBackendName 返回当前后端名称
func GetBackendName() string {
    if currentBackend != nil {
        return currentBackend.name()
    }
    return "none"
}
```

> **关键设计**：`engine.go` 和 `tracert_engine.go` 中调用的是 `PingOne` / `PingOneWithTTL`，重构后这两个函数变成委托入口，调用 `currentBackend` 的对应方法。**引擎层代码无需任何修改**。

### 5.2 Windows 后端 icmp_windows.go

```go
//go:build windows && !rawicmp

package icmp

// windowsBackend Windows iphlpapi.dll 后端
type windowsBackend struct{}

func (b *windowsBackend) pingOne(ip net.IP, timeout uint32, dataSize uint16) (*PingResult, error) {
    return pingOneWindows(ip, timeout, dataSize)
}

func (b *windowsBackend) pingOneWithTTL(ip net.IP, timeout uint32, dataSize uint16, ttl uint8) (*PingResult, error) {
    return pingOneWithTTLWindows(ip, timeout, dataSize, ttl)
}

func (b *windowsBackend) close() error {
    return nil // Windows 后端无需全局资源清理
}

func (b *windowsBackend) name() string {
    return "WindowsAPI(iphlpapi)"
}

// initWindowsBackend 初始化 Windows 后端
func initWindowsBackend() icmpBackend {
    return &windowsBackend{}
}

// pingOneWindows 原有 PingOne 实现（逻辑不变，仅重命名）
func pingOneWindows(ip net.IP, timeout uint32, dataSize uint16) (*PingResult, error) {
    // ... 原有 icmp_windows.go 中 PingOne 的完整实现 ...
}

// pingOneWithTTLWindows 原有 PingOneWithTTL 实现（逻辑不变，仅重命名）
func pingOneWithTTLWindows(ip net.IP, timeout uint32, dataSize uint16, ttl uint8) (*PingResult, error) {
    // ... 原有 icmp_windows.go 中 PingOneWithTTL 的完整实现 ...
}
```

> **重要**：Windows 后端的核心实现代码**完全保留**，包括所有 Windows API 调用、结构体、状态码、缓冲区计算、unsafe 操作等。仅做以下调整：
>
> 1. 原 `PingOne` → 重命名为 `pingOneWindows`（小写，内部函数）
> 2. 原 `PingOneWithTTL` → 重命名为 `pingOneWithTTLWindows`（小写，内部函数）
> 3. 新增 `windowsBackend` 结构体，实现 `icmpBackend` 接口
> 4. 新增 `initWindowsBackend()` 工厂函数

### 5.3 Raw Socket 后端 icmp_raw.go

```go
//go:build !windows || rawicmp

package icmp

import (
    "encoding/binary"
    "fmt"
    "net"
    "os"
    "sync"
    "sync/atomic"
    "time"

    "golang.org/x/net/icmp"
    "golang.org/x/net/ipv4"

    "github.com/NetWeaverGo/core/internal/logger"
)

const (
    protocolICMP  = 1    // IPv4 ICMP 协议号
    maxMessageSize = 1500 // 最大消息长度
)

// rawSocketBackend Raw Socket 后端
type rawSocketBackend struct {
    mu   sync.Mutex
    conn *icmp.PacketConn
}

// 全局序列号计数器
var globalSeq atomic.Uint32

func nextSeq() int {
    return int(globalSeq.Add(1))
}

func icmpID() int {
    return os.Getpid() & 0xffff
}

// initRawSocketBackend 初始化 Raw Socket 后端
func initRawSocketBackend() (icmpBackend, error) {
    b := &rawSocketBackend{}
    conn, err := b.getOrCreateConn()
    if err != nil {
        return nil, err
    }
    // 验证连接可用后立即关闭（仅检测权限）
    // 实际使用时 getOrCreateConn 会重建
    _ = conn
    return b, nil
}

func (b *rawSocketBackend) pingOne(ip net.IP, timeout uint32, dataSize uint16) (*PingResult, error) {
    return b.pingOneRaw(ip, timeout, dataSize, 128)
}

func (b *rawSocketBackend) pingOneWithTTL(ip net.IP, timeout uint32, dataSize uint16, ttl uint8) (*PingResult, error) {
    return b.pingOneRaw(ip, timeout, dataSize, ttl)
}

func (b *rawSocketBackend) close() error {
    b.mu.Lock()
    defer b.mu.Unlock()
    if b.conn != nil {
        err := b.conn.Close()
        b.conn = nil
        return err
    }
    return nil
}

func (b *rawSocketBackend) name() string {
    return "RawSocket(golang.org/x/net/icmp)"
}

// getOrCreateConn 获取或创建 ICMP 连接
func (b *rawSocketBackend) getOrCreateConn() (*icmp.PacketConn, error) {
    b.mu.Lock()
    defer b.mu.Unlock()
    if b.conn != nil {
        return b.conn, nil
    }
    c, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
    if err != nil {
        return nil, fmt.Errorf("failed to listen on ip4:icmp (需要管理员/root 权限): %w", err)
    }
    b.conn = c
    return b.conn, nil
}

// prepareSendData 构造 ICMP 发送数据（与 Windows 实现一致）
func prepareSendData(dataSize uint16) []byte {
    sendData := make([]byte, dataSize)
    if dataSize == 0 {
        return sendData
    }
    timestamp := time.Now().UnixNano()
    for i := 0; i < 8 && i < int(dataSize); i++ {
        sendData[i] = byte(timestamp >> (i * 8))
    }
    pattern := []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZ")
    for i := 8; i < int(dataSize); i++ {
        sendData[i] = pattern[(i-8)%len(pattern)]
    }
    return sendData
}

// pingOneRaw 核心实现
func (b *rawSocketBackend) pingOneRaw(ip net.IP, timeout uint32, dataSize uint16, ttl uint8) (*PingResult, error) {
    if dataSize == 0 {
        dataSize = 32
    }

    ipStr := ip.String()
    ip = ip.To4()
    if ip == nil {
        return nil, fmt.Errorf("invalid IPv4 address")
    }

    conn, err := b.getOrCreateConn()
    if err != nil {
        return nil, err
    }

    // 设置 TTL
    pconn := ipv4.NewPacketConn(conn)
    if err := pconn.SetTTL(int(ttl)); err != nil {
        return nil, fmt.Errorf("failed to set TTL: %w", err)
    }

    // 构造 ICMP Echo Request
    seq := nextSeq()
    msg := icmp.Message{
        Type: ipv4.ICMPTypeEcho,
        Code: 0,
        Body: &icmp.Echo{
            ID:   icmpID(),
            Seq:  seq,
            Data: prepareSendData(dataSize),
        },
    }
    wb, err := msg.Marshal(nil)
    if err != nil {
        return nil, fmt.Errorf("failed to marshal ICMP message: %w", err)
    }

    // 发送
    dst := &net.IPAddr{IP: ip}
    sendTime := time.Now()
    if _, err := conn.WriteTo(wb, dst); err != nil {
        return nil, fmt.Errorf("failed to send ICMP: %w", err)
    }

    // 设置读取超时
    deadline := sendTime.Add(time.Duration(timeout) * time.Millisecond)
    conn.SetReadDeadline(deadline)

    // 读取响应
    rb := make([]byte, maxMessageSize)
    for {
        n, peer, err := conn.ReadFrom(rb)
        if err != nil {
            if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
                return &PingResult{
                    IP:     "*",
                    Status: "Request Timed Out",
                    Error:  "Request Timed Out",
                }, nil
            }
            return &PingResult{
                IP:     "*",
                Status: "Error",
                Error:  err.Error(),
            }, nil
        }

        rm, err := icmp.ParseMessage(protocolICMP, rb[:n])
        if err != nil {
            continue
        }

        switch rm.Type {
        case ipv4.ICMPTypeEchoReply:
            reply, ok := rm.Body.(*icmp.Echo)
            if !ok || reply.ID != icmpID() || reply.Seq != seq {
                continue
            }
            rtt := time.Since(sendTime).Milliseconds()
            replyIP := extractIP(peer)

            return &PingResult{
                IP:            ipStr,
                Success:       true,
                RoundTripTime: float64(rtt),
                TTL:           ttl,
                Status:        "Success",
            }, nil

        case ipv4.ICMPTypeTimeExceeded:
            // 从 TimeExceeded Body 中提取原始 Echo ID/Seq 校验匹配
            if !matchTimeExceeded(rm, icmpID(), seq) {
                continue
            }
            rtt := time.Since(sendTime).Milliseconds()
            replyIP := extractIP(peer)

            return &PingResult{
                IP:            replyIP,
                Success:       false,
                RoundTripTime: float64(rtt),
                TTL:           ttl,
                Status:        "TTLExpired",
            }, nil

        case ipv4.ICMPTypeDestinationUnreachable:
            if !matchDestUnreachable(rm, icmpID(), seq) {
                continue
            }
            return &PingResult{
                IP:     "*",
                Status: "Destination Host Unreachable",
                Error:  "Destination Host Unreachable",
            }, nil

        default:
            continue
        }
    }
}

// extractIP 从 peer 地址中提取 IP 字符串
func extractIP(peer net.Addr) string {
    s := peer.String()
    if addr, _, err := net.SplitHostPort(s); err == nil {
        return addr
    }
    return s
}

// matchTimeExceeded 校验 TimeExceeded 消息是否匹配我们的请求
func matchTimeExceeded(rm *icmp.Message, expectID, expectSeq int) bool {
    body, ok := rm.Body.(*icmp.TimeExceeded)
    if !ok {
        return false
    }
    // body.Data: 原始 IP 头(20B) + ICMP 头(8B)
    if len(body.Data) >= 28 {
        origID := int(binary.BigEndian.Uint16(body.Data[24:26]))
        origSeq := int(binary.BigEndian.Uint16(body.Data[26:28]))
        return origID == expectID && origSeq == expectSeq
    }
    return false
}

// matchDestUnreachable 校验 DestUnreachable 消息是否匹配
func matchDestUnreachable(rm *icmp.Message, expectID, expectSeq int) bool {
    body, ok := rm.Body.(*icmp.DstUnreach)
    if !ok {
        return false
    }
    if len(body.Data) >= 28 {
        origID := int(binary.BigEndian.Uint16(body.Data[24:26]))
        origSeq := int(binary.BigEndian.Uint16(body.Data[26:28]))
        return origID == expectID && origSeq == expectSeq
    }
    return false
}
```

### 5.4 后端选择器 icmp_selector.go

#### Windows 默认选择器

```go
//go:build windows && !rawicmp

package icmp

import "github.com/NetWeaverGo/core/internal/logger"

func init() {
    // Windows 默认使用 iphlpapi 后端
    currentBackend = initWindowsBackend()
    backendType = BackendWindowsAPI
    logger.Info("ICMP", "-", "ICMP 后端初始化: %s", currentBackend.name())
}
```

#### Raw Socket 选择器

```go
//go:build !windows || rawicmp

package icmp

import "github.com/NetWeaverGo/core/internal/logger"

func init() {
    backend, err := initRawSocketBackend()
    if err != nil {
        logger.Error("ICMP", "-", "Raw Socket 后端初始化失败: %v", err)
        // 在非 Windows 平台上，这是致命错误（无 fallback）
        // 在 Windows + rawicmp 标签下，用户明确要求 raw socket
        panic("ICMP Raw Socket 后端初始化失败: " + err.Error())
    }
    currentBackend = backend
    backendType = BackendRawSocket
    logger.Info("ICMP", "-", "ICMP 后端初始化: %s", currentBackend.name())
}
```

#### 运行时切换支持（阶段三）

```go
// icmp_backend.go 中添加

// SetBackend 切换 ICMP 后端实现
func SetBackend(bt BackendType) error {
    switch bt {
    case BackendWindowsAPI:
        // 仅 Windows 可用
        backend, err := initWindowsBackendSelector()
        if err != nil {
            return err
        }
        if currentBackend != nil {
            currentBackend.close()
        }
        currentBackend = backend
        backendType = BackendWindowsAPI
        return nil

    case BackendRawSocket:
        backend, err := initRawSocketBackend()
        if err != nil {
            return fmt.Errorf("切换到 Raw Socket 后端失败（需要管理员权限）: %w", err)
        }
        if currentBackend != nil {
            currentBackend.close()
        }
        currentBackend = backend
        backendType = BackendRawSocket
        return nil

    default:
        return fmt.Errorf("未知后端类型: %d", bt)
    }
}
```

### 5.5 连接管理器设计

```go
// icmp_conn.go — Raw Socket 后端的连接管理
//go:build !windows || rawicmp

package icmp

import (
    "fmt"
    "sync"

    "golang.org/x/net/icmp"
    "github.com/NetWeaverGo/core/internal/logger"
)

// connManager 连接管理器（嵌入 rawSocketBackend 使用）
type connManager struct {
    mu   sync.Mutex
    conn *icmp.PacketConn
}

func (m *connManager) getOrCreate() (*icmp.PacketConn, error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    if m.conn != nil {
        return m.conn, nil
    }
    c, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
    if err != nil {
        return nil, fmt.Errorf("failed to listen on ip4:icmp: %w", err)
    }
    m.conn = c
    logger.Info("ICMP", "-", "ICMP raw socket 连接创建成功")
    return c, nil
}

func (m *connManager) close() error {
    m.mu.Lock()
    defer m.mu.Unlock()
    if m.conn == nil {
        return nil
    }
    err := m.conn.Close()
    m.conn = nil
    logger.Info("ICMP", "-", "ICMP raw socket 连接已关闭")
    return err
}

// reset 重置连接（异常时调用）
func (m *connManager) reset() {
    m.mu.Lock()
    defer m.mu.Unlock()
    if m.conn != nil {
        m.conn.Close()
        m.conn = nil
    }
    logger.Info("ICMP", "-", "ICMP raw socket 连接已重置")
}
```

### 5.6 ICMP消息构造与解析

#### 构造 Echo Request

```go
msg := icmp.Message{
    Type: ipv4.ICMPTypeEcho,  // Type = 8
    Code: 0,                   // Code = 0
    Body: &icmp.Echo{
        ID:   os.Getpid() & 0xffff,
        Seq:  seq,
        Data: prepareSendData(dataSize),
    },
}
wb, _ := msg.Marshal(nil)  // 自动计算校验和
```

#### 解析 Echo Reply

```go
rb := make([]byte, 1500)
n, peer, _ := conn.ReadFrom(rb)
rm, _ := icmp.ParseMessage(1, rb[:n])

switch rm.Type {
case ipv4.ICMPTypeEchoReply:              // Type = 0
    echo := rm.Body.(*icmp.Echo)
case ipv4.ICMPTypeTimeExceeded:           // Type = 11
    body := rm.Body.(*icmp.TimeExceeded)
case ipv4.ICMPTypeDestinationUnreachable: // Type = 3
    body := rm.Body.(*icmp.DstUnreach)
}
```

### 5.7 TTL设置与Traceroute支持

```go
pconn := ipv4.NewPacketConn(conn)
pconn.SetTTL(int(ttl))

// Traceroute: TTL=1→第1跳, TTL=2→第2跳, ..., TTL=N→目标
```

### 5.8 状态码映射

Windows ICMP 状态码 → 标准 ICMP 类型码 映射：

| Windows 状态码                   | ICMP Type                  | 映射后 Status                        |
| -------------------------------- | -------------------------- | ------------------------------------ |
| 0 (IP_SUCCESS)                   | EchoReply (0)              | `"Success"`                          |
| 11013 (IP_TTL_EXPIRED_TRANSIT)   | TimeExceeded (11)          | `"TTLExpired"`                       |
| 11014 (IP_TTL_EXPIRED_REASSEM)   | TimeExceeded (11)          | `"TTLExpired"`                       |
| 11010 (IP_REQ_TIMED_OUT)         | (超时)                     | `"Request Timed Out"`                |
| 11002 (IP_DEST_NET_UNREACHABLE)  | DestUnreachable (3) Code=0 | `"Destination Network Unreachable"`  |
| 11003 (IP_DEST_HOST_UNREACHABLE) | DestUnreachable (3) Code=1 | `"Destination Host Unreachable"`     |
| 11004 (IP_DEST_PROT_UNREACHABLE) | DestUnreachable (3) Code=2 | `"Destination Protocol Unreachable"` |
| 11005 (IP_DEST_PORT_UNREACHABLE) | DestUnreachable (3) Code=3 | `"Destination Port Unreachable"`     |

### 5.9 响应匹配与防伪

**策略**：Echo.ID + Echo.Seq 双重匹配

- **Echo ID**：`os.Getpid() & 0xffff`，同一进程内固定
- **Echo Seq**：全局原子递增，每次请求唯一
- **TimeExceeded 匹配**：从 Body.Data 中提取原始 IP头+ICMP头，校验 ID/Seq
- **DestUnreachable 匹配**：同上

### 5.10 权限检测机制

```go
// checkRawSocketPermission 检测 raw socket 权限
func checkRawSocketPermission() error {
    conn, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
    if err != nil {
        return fmt.Errorf("需要管理员/root 权限: %w", err)
    }
    conn.Close()
    return nil
}
```

---

## 6. 测试方案

### 6.1 单元测试

**Windows 后端测试**（`icmp_windows_test.go`，保持现有）：

| 测试用例                    | 验证点                     |
| --------------------------- | -------------------------- |
| `TestPingOne_Localhost`     | Success=true, IP=127.0.0.1 |
| `TestPingOne_Timeout`       | Success=false              |
| `TestPingOne_LargeDataSize` | 32/300/1000/8000 字节      |
| `TestBatchPingEngine_*`     | 并发/取消/无效IP           |
| `TestIcmpStatusToString`    | 状态码映射                 |

**Raw Socket 后端测试**（`icmp_raw_test.go`，新建）：

| 测试用例                            | 验证点                     |
| ----------------------------------- | -------------------------- |
| `TestPingOne_Localhost_Raw`         | Success=true, IP=127.0.0.1 |
| `TestPingOne_Timeout_Raw`           | Success=false              |
| `TestPingOne_LargeDataSize_Raw`     | 各种数据大小               |
| `TestPingOneWithTTL_TTLExpired_Raw` | Status="TTLExpired"        |
| `TestResponseMatching_Raw`          | ID+Seq 不匹配时丢弃        |
| `TestPermissionCheck_Raw`           | 无权限时返回错误           |
| `TestBackendSwitch`                 | 运行时切换后端             |

**后端抽象层测试**（`icmp_backend_test.go`，新建）：

| 测试用例                 | 验证点            |
| ------------------------ | ----------------- |
| `TestGetBackendName`     | 返回正确后端名称  |
| `TestPingOne_Delegation` | 委托给正确后端    |
| `TestSetBackend`         | 切换后端成功/失败 |

### 6.2 集成测试

| 场景                 | 方法                           | 验证点                   |
| -------------------- | ------------------------------ | ------------------------ |
| Windows 默认编译     | `go build ./...`               | 使用 iphlpapi 后端       |
| Windows rawicmp 编译 | `go build -tags rawicmp ./...` | 使用 raw socket 后端     |
| Linux 编译           | `GOOS=linux go build ./...`    | 使用 raw socket 后端     |
| 双后端对比           | 同一目标，两种后端结果一致     | RTT 偏差 ≤ 1ms，跳数一致 |

### 6.3 回归测试

| 对比项           | Windows API 后端 | Raw Socket 后端 |
| ---------------- | ---------------- | --------------- |
| Ping 127.0.0.1   | 基准             | ≤基准+1ms       |
| Traceroute 跳数  | 基准             | 一致            |
| 批量 Ping 吞吐量 | 基准             | ≥基准×0.8       |

---

## 7. 风险分析与缓解措施

### 7.1 高风险

| 风险                        | 影响                             | 缓解措施                                                                                           |
| --------------------------- | -------------------------------- | -------------------------------------------------------------------------------------------------- |
| **Raw Socket 需管理员权限** | 普通用户无法使用 raw socket 后端 | **双实现并行**：Windows 默认用 iphlpapi（无需权限），raw socket 为可选；Linux 通过 setcap 设置能力 |
| **并发安全**                | 多 goroutine 共享连接竞态        | 阶段一用互斥锁；阶段二优化为连接池                                                                 |
| **响应误匹配**              | 高并发下收到其他请求的响应       | ID+Seq 双重匹配；TimeExceeded/DestUnreachable 中提取原始头校验                                     |

### 7.2 中风险

| 风险             | 影响                   | 缓解措施                                      |
| ---------------- | ---------------------- | --------------------------------------------- |
| **RTT 精度差异** | 应用层计时 vs 内核计时 | 偏差 ~1ms，可接受                             |
| **防火墙拦截**   | raw ICMP 被防火墙阻止  | 检测失败时提示；Windows 默认不使用 raw socket |
| **MTU/分片**     | 大数据包可能分片       | 限制 DataSize ≤ 1472                          |

### 7.3 低风险

| 风险                 | 影响                              | 缓解措施                       |
| -------------------- | --------------------------------- | ------------------------------ |
| Windows API 后端回归 | 重构可能导致 Windows 后端功能异常 | 保持原有实现不变，仅重命名函数 |
| 接口委托开销         | 多一层函数调用                    | 内联优化，开销可忽略           |

---

## 8. 实施步骤清单

### 阶段一（接口抽象 + Raw Socket 后端）

| 步骤 | 操作                                                                       | 验证                                             |
| ---- | -------------------------------------------------------------------------- | ------------------------------------------------ |
| 1    | `go get golang.org/x/net@latest`                                           | `go mod tidy` 成功                               |
| 2    | 新建 `icmp_backend.go`（接口定义 + 委托函数）                              | 编译通过                                         |
| 3    | 修改 `icmp_windows.go`：重命名 PingOne→pingOneWindows，新增 windowsBackend | 原有测试通过                                     |
| 4    | 新建 `icmp_selector_default.go`（Windows 默认选择器）                      | 编译通过                                         |
| 5    | 新建 `icmp_raw.go`（Raw Socket 后端实现）                                  | 编译通过                                         |
| 6    | 新建 `icmp_selector_raw.go`（Raw Socket 选择器）                           | `go build -tags rawicmp` 通过                    |
| 7    | 新建 `icmp_conn.go`（连接管理器）                                          | 编译通过                                         |
| 8    | 修改 `engine.go` 构建标签（移除 `//go:build windows`）                     | 编译通过                                         |
| 9    | 修改 `tracert_engine.go` 构建标签同上                                      | 编译通过                                         |
| 10   | 新建 `icmp_raw_test.go`                                                    | `go test -tags rawicmp ./internal/icmp/...` 通过 |
| 11   | 修改 `icmp_windows_test.go`（适配重命名）                                  | `go test ./internal/icmp/...` 通过               |
| 12   | 手动测试：Windows 默认编译 → iphlpapi 后端                                 | 功能正常                                         |
| 13   | 手动测试：Windows rawicmp 编译 → raw socket 后端                           | 功能正常                                         |
| 14   | Linux 交叉编译测试                                                         | `GOOS=linux go build ./...` 通过                 |

### 阶段二（连接生命周期管理）

| 步骤 | 操作                       | 验证             |
| ---- | -------------------------- | ---------------- |
| 15   | 优化 `icmp_conn.go` 连接池 | 单元测试通过     |
| 16   | 连接健康检查 + 自动重建    | 模拟连接断开     |
| 17   | 性能基准测试               | 对比阶段一吞吐量 |

### 阶段三（运行时后端切换）

| 步骤 | 操作                                | 验证           |
| ---- | ----------------------------------- | -------------- |
| 18   | `icmp_backend.go` 添加 SetBackend() | 切换成功/失败  |
| 19   | UI 设置页添加"ICMP 后端"选项        | 前端功能正常   |
| 20   | 端到端测试                          | 切换后功能正常 |

### 阶段四（IPv6 支持）

| 步骤 | 操作                          | 验证                   |
| ---- | ----------------------------- | ---------------------- |
| 21   | Raw Socket 后端增加 PingOneV6 | Ping ::1 成功          |
| 22   | TracertEngine IPv6 支持       | Traceroute IPv6 目标   |
| 23   | 前端 IPv6 UI                  | 组件更新               |
| 24   | TypeScript 绑定更新           | `wails3 generate` 成功 |

---

## 9. 代码变更量估算

### 阶段一

| 文件                       | 操作                  | 新增     | 修改    | 删除   |
| -------------------------- | --------------------- | -------- | ------- | ------ |
| `icmp_backend.go`          | 新建                  | ~60      | 0       | 0      |
| `icmp_raw.go`              | 新建                  | ~280     | 0       | 0      |
| `icmp_conn.go`             | 新建                  | ~80      | 0       | 0      |
| `icmp_selector_default.go` | 新建                  | ~15      | 0       | 0      |
| `icmp_selector_raw.go`     | 新建                  | ~15      | 0       | 0      |
| `icmp_raw_test.go`         | 新建                  | ~220     | 0       | 0      |
| `icmp_backend_test.go`     | 新建                  | ~50      | 0       | 0      |
| `icmp_windows.go`          | 重命名函数+新增结构体 | ~25      | ~4      | 0      |
| `icmp_windows_test.go`     | 适配重命名            | 0        | ~10     | 0      |
| `engine.go`                | 移除构建标签          | 0        | 1       | 1      |
| `tracert_engine.go`        | 移除构建标签          | 0        | 1       | 1      |
| `go.mod` / `go.sum`        | 添加依赖              | ~6       | 0       | 0      |
| **合计**                   |                       | **~751** | **~16** | **~2** |

### 关键结论

- **Windows API 实现完全保留**：`icmp_windows.go` 的核心逻辑零删除，仅重命名函数和新增 `windowsBackend` 适配器
- **引擎层几乎零改动**：`engine.go` / `tracert_engine.go` 仅移除构建标签
- **types.go 零改动**
- **UI 服务层零改动**
- **前端零改动**

---

## 10. 验收标准

### 功能验收

| 验收项                          | 标准                                           | 测试方法                                    |
| ------------------------------- | ---------------------------------------------- | ------------------------------------------- |
| Windows 默认编译使用 iphlpapi   | `GetBackendName()` 返回 "WindowsAPI(iphlpapi)" | `go build ./...`                            |
| Windows rawicmp 使用 raw socket | `GetBackendName()` 返回 "RawSocket(...)"       | `go build -tags rawicmp`                    |
| Linux 编译使用 raw socket       | 编译成功，功能正常                             | `GOOS=linux go build`                       |
| 双后端 Ping 结果一致            | 同一目标，两种后端 Success 状态一致            | 对比测试                                    |
| 双后端 Traceroute 跳数一致      | 同一目标，跳数一致                             | 对比测试                                    |
| Windows 后端原有测试全部通过    | 0 失败                                         | `go test ./internal/icmp/...`               |
| Raw Socket 后端测试全部通过     | 0 失败                                         | `go test -tags rawicmp ./internal/icmp/...` |

### 性能验收

| 验收项               | 标准                 |
| -------------------- | -------------------- |
| 双后端 Ping RTT 偏差 | ≤ 1ms                |
| Raw Socket 吞吐量    | ≥ Windows API 的 80% |

### 安全验收

| 验收项                       | 标准                             |
| ---------------------------- | -------------------------------- |
| Windows 后端 unsafe 使用不变 | 保持现有 5 处 unsafe（预期行为） |
| Raw Socket 后端无 unsafe     | `icmp_raw.go` 无 `unsafe` import |
| Raw Socket 无权限时明确报错  | 返回含"权限"的错误信息           |

---

## 附录 A：迁移后文件结构

```
internal/icmp/
├── types.go                    # 类型定义（不变）
├── icmp_backend.go             # 接口抽象 + 委托入口（新增）
├── icmp_backend_test.go        # 后端抽象测试（新增）
│
├── icmp_windows.go             # Windows API 后端实现（保留，微调）
├── icmp_windows_test.go        # Windows 后端测试（保留，微调）
├── icmp_selector_default.go    # 默认选择器→Windows（新增）
│   //go:build windows && !rawicmp
│
├── icmp_raw.go                 # Raw Socket 后端实现（新增）
├── icmp_raw_test.go            # Raw Socket 后端测试（新增）
├── icmp_conn.go                # 连接管理器（新增）
├── icmp_selector_raw.go        # 默认选择器→Raw Socket（新增）
│   //go:build !windows || rawicmp
│
├── engine.go                   # 批量Ping引擎（移除构建标签）
├── engine_test.go              # 引擎测试
└── tracert_engine.go           # Traceroute引擎（移除构建标签）
```

## 附录 B：golang.org/x/net/icmp 关键 API 速查

```go
import (
    "golang.org/x/net/icmp"
    "golang.org/x/net/ipv4"
)

conn, _ := icmp.ListenPacket("ip4:icmp", "0.0.0.0")    // 创建连接
msg := icmp.Message{Type: ipv4.ICMPTypeEcho, Code: 0,  // 构造消息
    Body: &icmp.Echo{ID: id, Seq: seq, Data: data}}
wb, _ := msg.Marshal(nil)                               // 序列化（含校验和）
conn.WriteTo(wb, &net.IPAddr{IP: ip})                   // 发送
n, peer, _ := conn.ReadFrom(rb)                         // 接收
rm, _ := icmp.ParseMessage(1, rb[:n])                   // 解析
pconn := ipv4.NewPacketConn(conn)                       // IPv4 控制
pconn.SetTTL(ttl)                                       // 设置 TTL
conn.SetReadDeadline(time.Now().Add(timeout))            // 设置超时
conn.Close()                                            // 关闭
```

## 附录 C：ICMP Echo 消息格式

```
 0                   1                   2                   3
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|     Type      |     Code      |          Checksum             |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|           Identifier          |        Sequence Number        |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                             Data ...                         |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+

Echo Request:  Type = 8, Code = 0
Echo Reply:   Type = 0, Code = 0
Time Exceeded: Type = 11, Code = 0 (TTL expired in transit)
Dest Unreachable: Type = 3, Code = 0-15
```

## 附录 D：Linux/Windows 权限配置

**Linux**：

```bash
# 方法一（推荐）：设置文件能力，无需 root 运行
sudo setcap cap_net_raw+ep ./netweaver

# 方法二：允许非特权 ICMP socket
sudo sysctl -w net.ipv4.ping_group_range="0 2147483647"
```

**Windows**：

```
# Raw Socket 需要管理员权限。选项：
1. 以管理员身份运行
2. 应用清单设置 requireAdministrator
3. 使用 SeNetworkConfigurationPrivilege 组策略
4. 不使用 rawicmp 标签 → 默认使用 iphlpapi（无需管理员）← 推荐
```
