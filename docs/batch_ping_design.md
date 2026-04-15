# 批量 Ping 功能 — 详细规划设计文档

> **版本**: v1.0  
> **日期**: 2026-04-15  
> **状态**: 设计阶段

---

## 一、功能概述

### 1.1 目标

为 NetWeaverGo 添加一个**批量 Ping 检测**工具，使用户能够快速检测多台主机/网段的网络可达性。该功能通过调用 Windows 原生 `Iphlpapi.dll` 中的 ICMP API（`IcmpCreateFile`、`IcmpSendEcho`）实现，**无需管理员权限**，性能优于 `exec.Command("ping", ...)` 方式。

### 1.2 核心能力

| 能力 | 说明 |
|------|------|
| 单 IP 探测 | 支持输入单个 IPv4 地址进行 Ping |
| IP 范围探测 | 支持 `192.168.1.1-254` 或 `192.168.1.10~20` 格式 |
| CIDR 网段探测 | 支持 `192.168.1.0/24` 格式，自动展开为主机列表 |
| 多行/混合输入 | 支持换行、逗号分隔混合输入多种格式 |
| 自定义参数 | 超时时间、请求间隔、并发数、数据包大小、重试次数 |
| 实时结果反馈 | 前端实时刷新每个 IP 的探测结果 |
| 结果导出 | 支持导出 CSV 格式结果 |
| 从设备库导入 | 可一键导入已管理设备的 IP 进行探测 |

### 1.3 技术选型依据

| 方案 | 优势 | 劣势 |
|------|------|------|
| **Windows ICMP API (选用)** | 不需要管理员权限；封装好底层协议；性能优秀 | 仅限 Windows 平台 |
| Raw Socket | 跨平台 | 需要管理员权限 |
| exec.Command("ping") | 实现简单 | 性能差；需解析命令行输出；并发困难 |
| 第三方 Go Ping 库 | 跨平台 | 引入外部依赖；部分仍需管理员权限 |

> 由于本项目 (Wails 桌面应用) 已锁定 Windows 平台，选用 ICMP API 是最优方案。

---

## 二、项目架构对齐分析

### 2.1 现有架构概览

```
NetWeaverGo/
├── cmd/netweaver/main.go        # Wails 应用入口，注册所有 Service
├── internal/
│   ├── config/                  # 配置管理、数据库、设备校验
│   ├── models/                  # GORM 数据模型
│   ├── ui/                      # Wails Binding 服务层 (前端直接调用)
│   │   ├── device_service.go    # 设备管理服务
│   │   ├── forge_service.go     # 配置构建 + IP 验证/解析
│   │   ├── network_calc_service.go  # 子网计算
│   │   ├── settings_service.go  # 全局设置
│   │   └── view_models.go       # 视图模型
│   ├── taskexec/                # 统一任务执行引擎
│   ├── logger/                  # 日志系统
│   └── ...
├── frontend/src/
│   ├── views/Tools/             # 工具页面 (NetworkCalc, ConfigForge, ProtocolRef)
│   ├── router/index.ts          # Vue Router 路由配置
│   └── ...
```

### 2.2 可复用资源

| 已有组件 | 位置 | 复用方式 |
|---------|------|---------|
| `parseIPv4Addr()` | `ui/network_calc_service.go:531` | 直接调用，IPv4 地址验证 |
| `ipv4ToUint32()` / `uint32ToIPv4()` | `ui/network_calc_service.go:608-614` | 直接调用，IP 与整数互转 |
| `cidrToMaskUint32()` | `ui/network_calc_service.go:617` | 直接调用，CIDR 转子网掩码 |
| `parseIPv4LastOctetRange()` | `ui/forge_service.go:92` | 直接调用，IP 范围解析（如 `1.1.1.1-10`） |
| `ValidateIP()` | `ui/forge_service.go:46` | 直接调用，单 IP 校验 |
| `ForgeService` 架构模式 | `ui/forge_service.go` | 参照其模式创建 `PingService` |
| `logger` 模块 | `internal/logger/` | 直接使用项目统一日志 |
| `Tools/` 前端目录 | `frontend/src/views/Tools/` | 在此目录下新增 `BatchPing.vue` |

### 2.3 新模块定位

批量 Ping 功能作为**独立工具**，不依赖任务执行引擎 (`taskexec`)，其定位与 `NetworkCalc`、`ConfigForge` 平级：

```
新增模块:
├── internal/
│   ├── icmp/                        # [NEW] ICMP 核心引擎
│   │   ├── icmp_windows.go          # Windows ICMP API 封装 (syscall)
│   │   └── types.go                 # ICMP 相关类型定义
│   └── ui/
│       └── ping_service.go          # [NEW] Wails Binding 服务层
├── frontend/src/
│   └── views/Tools/
│       └── BatchPing.vue            # [NEW] 批量 Ping 前端页面
```

---

## 三、后端详细设计

### 3.1 ICMP 核心引擎 (`internal/icmp/`)

#### 3.1.1 Windows API 封装 — `icmp_windows.go`

通过 `syscall.LazyDLL` 加载 `Iphlpapi.dll`，封装以下 Windows API：

```go
package icmp

import (
    "fmt"
    "net"
    "sync"
    "syscall"
    "time"
    "unsafe"
)

var (
    iphlpapi        = syscall.NewLazyDLL("Iphlpapi.dll")
    procCreateFile  = iphlpapi.NewProc("IcmpCreateFile")
    procSendEcho    = iphlpapi.NewProc("IcmpSendEcho")
    procCloseHandle = iphlpapi.NewProc("IcmpCloseHandle")
)

// ICMP_ECHO_REPLY Windows ICMP 回显应答结构
//
// ⚠️ 关键：IcmpSendEcho 在 64 位平台上仍然使用 32 位版本的结构体布局
// 即 ICMP_ECHO_REPLY32 / IP_OPTION_INFORMATION32，所有指针字段用 uint32 表示。
// 参考: https://learn.microsoft.com/en-us/windows/win32/api/ipexport/ns-ipexport-icmp_echo_reply
//   "On a 64-bit platform, the ICMP_ECHO_REPLY32 structure should be used."
// 参考: IcmpSendEcho RequestOptions 参数说明:
//   "On a 64-bit platform, this parameter is in the form for an IP_OPTION_INFORMATION32 structure."
type ICMP_ECHO_REPLY struct {
    Address       uint32 // 4 bytes — 应答 IP（网络字节序）
    Status        uint32 // 4 bytes — IP 状态码
    RoundTripTime uint32 // 4 bytes — 往返时间（毫秒）
    DataSize      uint16 // 2 bytes — 应答数据大小
    Reserved      uint16 // 2 bytes — 保留
    DataPointer   uint32 // 4 bytes — ⚠️ 必须用 uint32，非 uintptr！匹配 ICMP_ECHO_REPLY32
    Options       IP_OPTION_INFORMATION32
}

// IP_OPTION_INFORMATION32 对应 Windows IP_OPTION_INFORMATION32 结构
// IcmpSendEcho 在 64 位平台上实际使用此 32 位版本
type IP_OPTION_INFORMATION32 struct {
    TTL         uint8  // 1 byte
    Tos         uint8  // 1 byte
    Flags       uint8  // 1 byte
    OptionsSize uint8  // 1 byte
    OptionsData uint32 // 4 bytes — ⚠️ 必须用 uint32，非 uintptr！匹配 32 位指针
}

// PingOne 对单个 IPv4 地址发送一次 ICMP Echo Request
// timeout: 超时时间(毫秒), dataSize: 数据包大小(字节)
func PingOne(ip net.IP, timeout uint32, dataSize uint16) (*PingResult, error) {
    // 1. IcmpCreateFile 创建 ICMP 句柄
    handle, _, err := procCreateFile.Call()
    if handle == uintptr(syscall.InvalidHandle) {
        return nil, fmt.Errorf("IcmpCreateFile 失败: %v", err)
    }
    defer procCloseHandle.Call(handle)

    // 2. 构造发送数据
    sendData := make([]byte, dataSize)
    for i := range sendData {
        sendData[i] = 'A'
    }

    // 3. 准备接收缓冲区
    replySize := unsafe.Sizeof(ICMP_ECHO_REPLY{}) + uintptr(dataSize) + 8
    replyBuf := make([]byte, replySize)

    // 4. 将 IP 转为 uint32 (网络字节序)
    ip4 := ip.To4()
    if ip4 == nil {
        return nil, fmt.Errorf("不是有效的 IPv4 地址: %s", ip.String())
    }
    destAddr := uint32(ip4[0]) | uint32(ip4[1])<<8 | uint32(ip4[2])<<16 | uint32(ip4[3])<<24

    // 5. IcmpSendEcho 发送并等待应答
    ret, _, _ := procSendEcho.Call(
        handle,
        uintptr(destAddr),
        uintptr(unsafe.Pointer(&sendData[0])),
        uintptr(dataSize),
        0, // 不设置选项
        uintptr(unsafe.Pointer(&replyBuf[0])),
        uintptr(replySize),
        uintptr(timeout),
    )

    result := &PingResult{
        IP:        ip.String(),
        Timestamp: time.Now(),
    }

    if ret == 0 {
        result.Success = false
        result.ErrorMsg = "目标主机不可达或超时"
        return result, nil
    }

    // 6. 解析应答
    reply := (*ICMP_ECHO_REPLY)(unsafe.Pointer(&replyBuf[0]))
    result.RTT       = time.Duration(reply.RoundTripTime) * time.Millisecond
    result.TTL        = int(reply.Options.TTL)
    result.DataSize   = int(reply.DataSize)
    result.Status     = reply.Status
    result.Success    = reply.Status == 0 // IP_SUCCESS = 0

    if !result.Success {
        result.ErrorMsg = icmpStatusToString(reply.Status)
    }

    return result, nil
}

// icmpStatusToString 将 ICMP 状态码转为可读字符串
func icmpStatusToString(status uint32) string {
    switch status {
    case 0:
        return "成功"
    case 11001:
        return "缓冲区太小"
    case 11002:
        return "目标网络不可达"
    case 11003:
        return "目标主机不可达"
    case 11004:
        return "目标协议不可达"
    case 11005:
        return "目标端口不可达"
    case 11010:
        return "请求超时"
    case 11013:
        return "TTL 过期"
    case 11050:
        return "一般性错误"
    default:
        return fmt.Sprintf("未知状态码: %d", status)
    }
}
```

#### 3.1.2 类型定义 — `types.go`

```go
package icmp

import "time"

// PingResult 单次 Ping 结果
type PingResult struct {
    IP        string        `json:"ip"`
    Success   bool          `json:"success"`
    RTT       time.Duration `json:"rtt"`
    TTL       int           `json:"ttl"`
    DataSize  int           `json:"dataSize"`
    Status    uint32        `json:"status"`
    ErrorMsg  string        `json:"errorMsg,omitempty"`
    Timestamp time.Time     `json:"timestamp"`
}

// PingConfig 批量 Ping 配置参数
type PingConfig struct {
    Timeout     uint32 `json:"timeout"`     // 单次超时（毫秒），默认 1000
    Interval    uint32 `json:"interval"`    // 请求间隔（毫秒），默认 0（无间隔）
    Count       int    `json:"count"`       // 每个 IP 重试次数，默认 1
    DataSize    uint16 `json:"dataSize"`    // 数据包大小（字节），默认 32
    Concurrency int    `json:"concurrency"` // 并发数，默认 64
}

// DefaultPingConfig 返回默认配置
func DefaultPingConfig() PingConfig {
    return PingConfig{
        Timeout:     1000,
        Interval:    0,
        Count:       1,
        DataSize:    32,
        Concurrency: 64,
    }
}

// PingHostResult 单个主机的汇总探测结果（多次 Ping 汇总）
type PingHostResult struct {
    IP          string  `json:"ip"`
    Alive       bool    `json:"alive"`       // 是否在线
    SentCount   int     `json:"sentCount"`   // 发送次数
    RecvCount   int     `json:"recvCount"`   // 成功次数
    LossRate    float64 `json:"lossRate"`    // 丢包率 (0.0-100.0)
    MinRTT      float64 `json:"minRtt"`      // 最小延迟 (ms)
    MaxRTT      float64 `json:"maxRtt"`      // 最大延迟 (ms)
    AvgRTT      float64 `json:"avgRtt"`      // 平均延迟 (ms)
    TTL         int     `json:"ttl"`         // 最后一次 TTL
    Status      string  `json:"status"`      // "online" | "offline" | "error" | "pending"
    ErrorMsg    string  `json:"errorMsg,omitempty"`
}

// BatchPingProgress 批量 Ping 进度信息
type BatchPingProgress struct {
    TotalIPs      int              `json:"totalIPs"`
    CompletedIPs  int              `json:"completedIPs"`
    OnlineCount   int              `json:"onlineCount"`
    OfflineCount  int              `json:"offlineCount"`
    ErrorCount    int              `json:"errorCount"`
    Progress      int              `json:"progress"`     // 0-100 百分比
    IsRunning     bool             `json:"isRunning"`
    ElapsedMs     int64            `json:"elapsedMs"`    // 已用时间(毫秒)
    Results       []PingHostResult `json:"results"`
}
```

#### 3.1.3 批量 Ping 引擎 — `engine.go`

```go
package icmp

import (
    "context"
    "math"
    "net"
    "sync"
    "sync/atomic"
    "time"
)

// BatchPingEngine 批量 Ping 执行引擎
type BatchPingEngine struct {
    config   PingConfig
    mu       sync.RWMutex
    cancel   context.CancelFunc
    running  atomic.Bool
    progress *BatchPingProgress
}

// NewBatchPingEngine 创建批量 Ping 引擎
func NewBatchPingEngine(config PingConfig) *BatchPingEngine {
    return &BatchPingEngine{
        config: config,
    }
}

// Run 执行批量 Ping（异步，可取消）
// ips: 待探测的 IP 列表
// onUpdate: 每完成一个 IP 后的回调（用于实时通知前端）
func (e *BatchPingEngine) Run(ctx context.Context, ips []string, onUpdate func(*BatchPingProgress)) {
    if e.running.Load() {
        return
    }
    e.running.Store(true)
    defer e.running.Store(false)

    ctx, cancel := context.WithCancel(ctx)
    e.cancel = cancel
    defer cancel()

    startTime := time.Now()

    // 初始化进度
    e.mu.Lock()
    e.progress = &BatchPingProgress{
        TotalIPs:  len(ips),
        IsRunning: true,
        Results:   make([]PingHostResult, len(ips)),
    }
    for i, ip := range ips {
        e.progress.Results[i] = PingHostResult{
            IP:     ip,
            Status: "pending",
        }
    }
    e.mu.Unlock()

    // 并发控制
    sem := make(chan struct{}, e.config.Concurrency)
    var wg sync.WaitGroup
    var completed atomic.Int32

    for i, ip := range ips {
        select {
        case <-ctx.Done():
            goto done
        default:
        }

        sem <- struct{}{}
        wg.Add(1)
        go func(index int, targetIP string) {
            defer wg.Done()
            defer func() { <-sem }()

            result := e.pingHost(ctx, targetIP)

            e.mu.Lock()
            e.progress.Results[index] = result
            c := int(completed.Add(1))
            e.progress.CompletedIPs = c
            e.progress.Progress = c * 100 / len(ips)
            e.progress.ElapsedMs = time.Since(startTime).Milliseconds()
            // 统计
            online, offline, errCount := 0, 0, 0
            for _, r := range e.progress.Results {
                switch r.Status {
                case "online":
                    online++
                case "offline":
                    offline++
                case "error":
                    errCount++
                }
            }
            e.progress.OnlineCount = online
            e.progress.OfflineCount = offline
            e.progress.ErrorCount = errCount
            snapshot := e.cloneProgressLocked()
            e.mu.Unlock()

            if onUpdate != nil {
                onUpdate(snapshot)
            }
        }(i, ip)

        // 请求间隔
        if e.config.Interval > 0 {
            time.Sleep(time.Duration(e.config.Interval) * time.Millisecond)
        }
    }

    wg.Wait()

done:
    e.mu.Lock()
    e.progress.IsRunning = false
    e.progress.ElapsedMs = time.Since(startTime).Milliseconds()
    snapshot := e.cloneProgressLocked()
    e.mu.Unlock()

    if onUpdate != nil {
        onUpdate(snapshot)
    }
}

// Stop 取消正在执行的批量 Ping
func (e *BatchPingEngine) Stop() {
    if e.cancel != nil {
        e.cancel()
    }
}

// GetProgress 获取当前进度快照
func (e *BatchPingEngine) GetProgress() *BatchPingProgress {
    e.mu.RLock()
    defer e.mu.RUnlock()
    if e.progress == nil {
        return nil
    }
    return e.cloneProgressLocked()
}

// pingHost 对单个主机执行多次 Ping 并汇总
func (e *BatchPingEngine) pingHost(ctx context.Context, ip string) PingHostResult {
    result := PingHostResult{
        IP:     ip,
        Status: "pending",
    }

    parsedIP := net.ParseIP(ip)
    if parsedIP == nil || parsedIP.To4() == nil {
        result.Status = "error"
        result.ErrorMsg = "无效的 IPv4 地址"
        return result
    }

    var rtts []float64
    sentCount := e.config.Count
    recvCount := 0
    var lastTTL int

    for i := 0; i < sentCount; i++ {
        select {
        case <-ctx.Done():
            result.Status = "error"
            result.ErrorMsg = "已取消"
            return result
        default:
        }

        pr, err := PingOne(parsedIP, e.config.Timeout, e.config.DataSize)
        if err != nil {
            continue
        }
        if pr.Success {
            recvCount++
            rttMs := float64(pr.RTT.Milliseconds())
            // 对于 <1ms 的结果，使用微秒精度
            if pr.RTT < time.Millisecond {
                rttMs = float64(pr.RTT.Microseconds()) / 1000.0
            }
            rtts = append(rtts, rttMs)
            lastTTL = pr.TTL
        }
    }

    result.SentCount = sentCount
    result.RecvCount = recvCount
    result.TTL = lastTTL

    if recvCount > 0 {
        result.Alive = true
        result.Status = "online"
        result.LossRate = float64(sentCount-recvCount) / float64(sentCount) * 100.0

        minRTT, maxRTT, sum := math.MaxFloat64, 0.0, 0.0
        for _, rtt := range rtts {
            sum += rtt
            if rtt < minRTT { minRTT = rtt }
            if rtt > maxRTT { maxRTT = rtt }
        }
        result.MinRTT = math.Round(minRTT*100) / 100
        result.MaxRTT = math.Round(maxRTT*100) / 100
        result.AvgRTT = math.Round(sum/float64(len(rtts))*100) / 100
    } else {
        result.Alive = false
        result.Status = "offline"
        result.LossRate = 100.0
        result.ErrorMsg = "目标主机不可达"
    }

    return result
}

func (e *BatchPingEngine) cloneProgressLocked() *BatchPingProgress {
    results := make([]PingHostResult, len(e.progress.Results))
    copy(results, e.progress.Results)
    return &BatchPingProgress{
        TotalIPs:     e.progress.TotalIPs,
        CompletedIPs: e.progress.CompletedIPs,
        OnlineCount:  e.progress.OnlineCount,
        OfflineCount: e.progress.OfflineCount,
        ErrorCount:   e.progress.ErrorCount,
        Progress:     e.progress.Progress,
        IsRunning:    e.progress.IsRunning,
        ElapsedMs:    e.progress.ElapsedMs,
        Results:      results,
    }
}
```

---

### 3.2 Wails Binding 服务层 (`internal/ui/ping_service.go`)

```go
package ui

import (
    "context"
    "encoding/csv"
    "fmt"
    "net"
    "net/netip"
    "strconv"
    "strings"
    "sync"

    "github.com/NetWeaverGo/core/internal/icmp"
    "github.com/NetWeaverGo/core/internal/logger"
    "github.com/NetWeaverGo/core/internal/repository"
    "github.com/wailsapp/wails/v3/pkg/application"
)

// PingService 批量 Ping 服务 (Wails Binding)
type PingService struct {
    wailsApp *application.App
    engine   *icmp.BatchPingEngine
    mu       sync.Mutex
    repo     repository.DeviceRepository
}

// NewPingService 创建批量 Ping 服务
func NewPingService() *PingService {
    return &PingService{
        repo: repository.NewDeviceRepository(),
    }
}

// ServiceStartup Wails 服务启动生命周期钩子
func (s *PingService) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
    s.wailsApp = application.Get()
    return nil
}

// ========== 前端 Binding API ==========

// PingRequest 批量 Ping 请求
type PingRequest struct {
    Targets     string          `json:"targets"`     // IP/网段/范围（多行或逗号分隔）
    Config      icmp.PingConfig `json:"config"`      // Ping 配置参数
    DeviceIDs   []uint          `json:"deviceIds"`   // 从设备库导入的设备 ID
}

// PingCSVResult CSV 导出结果
type PingCSVResult struct {
    FileName string `json:"fileName"`
    Content  string `json:"content"`
}

// StartBatchPing 启动批量 Ping (Wails Binding)
// 前端调用后异步执行，通过 Wails Events 推送进度
func (s *PingService) StartBatchPing(req PingRequest) (*icmp.BatchPingProgress, error) {
    s.mu.Lock()
    if s.engine != nil {
        progress := s.engine.GetProgress()
        if progress != nil && progress.IsRunning {
            s.mu.Unlock()
            return nil, fmt.Errorf("已有 Ping 任务正在执行中")
        }
    }
    s.mu.Unlock()

    // 解析目标 IP 列表
    ips, err := s.resolveTargets(req)
    if err != nil {
        return nil, err
    }
    if len(ips) == 0 {
        return nil, fmt.Errorf("未解析出有效的 IP 地址")
    }
    if len(ips) > 10000 {
        return nil, fmt.Errorf("IP 数量过多（%d），最大支持 10000 个", len(ips))
    }

    // 合并默认配置
    config := mergeWithDefaultPingConfig(req.Config)

    logger.Info("Ping", "-", "启动批量 Ping: 目标 %d 个 IP, 超时=%dms, 并发=%d, 重试=%d",
        len(ips), config.Timeout, config.Concurrency, config.Count)

    s.mu.Lock()
    s.engine = icmp.NewBatchPingEngine(config)
    s.mu.Unlock()

    // 异步执行
    go func() {
        s.engine.Run(context.Background(), ips, func(progress *icmp.BatchPingProgress) {
            // 通过 Wails Events 推送实时进度到前端
            // 注意：Wails v3 的事件 API 为 app.Event.Emit()，非 app.EmitEvent()
            // 与项目现有 taskexec_event_bridge.go 保持一致
            if s.wailsApp != nil && s.wailsApp.Event != nil {
                s.wailsApp.Event.Emit("ping:progress", progress)
            }
        })

        logger.Info("Ping", "-", "批量 Ping 执行完毕")
    }()

    // 立即返回初始进度状态
    return s.engine.GetProgress(), nil
}

// StopBatchPing 停止批量 Ping (Wails Binding)
func (s *PingService) StopBatchPing() error {
    s.mu.Lock()
    defer s.mu.Unlock()
    if s.engine == nil {
        return fmt.Errorf("无正在执行的 Ping 任务")
    }
    s.engine.Stop()
    logger.Info("Ping", "-", "批量 Ping 已手动停止")
    return nil
}

// GetPingProgress 获取当前 Ping 进度 (Wails Binding)
// 前端轮询调用，获取最新进度快照
func (s *PingService) GetPingProgress() *icmp.BatchPingProgress {
    s.mu.Lock()
    defer s.mu.Unlock()
    if s.engine == nil {
        return nil
    }
    return s.engine.GetProgress()
}

// ExportPingResultCSV 导出 Ping 结果为 CSV (Wails Binding)
func (s *PingService) ExportPingResultCSV() (*PingCSVResult, error) {
    s.mu.Lock()
    engine := s.engine
    s.mu.Unlock()
    if engine == nil {
        return nil, fmt.Errorf("无可导出的 Ping 结果")
    }
    progress := engine.GetProgress()
    if progress == nil || len(progress.Results) == 0 {
        return nil, fmt.Errorf("暂无可导出的 Ping 结果数据")
    }

    var sb strings.Builder
    sb.WriteString("\xef\xbb\xbf") // UTF-8 BOM
    cw := csv.NewWriter(&sb)

    headers := []string{"IP 地址", "状态", "延迟(ms)", "TTL", "丢包率(%)", "发送", "接收", "备注"}
    if err := cw.Write(headers); err != nil {
        return nil, fmt.Errorf("写入 CSV 表头失败: %w", err)
    }

    for _, r := range progress.Results {
        status := "离线"
        if r.Alive {
            status = "在线"
        }
        rttStr := "-"
        if r.Alive {
            rttStr = fmt.Sprintf("%.2f", r.AvgRTT)
        }
        row := []string{
            r.IP,
            status,
            rttStr,
            strconv.Itoa(r.TTL),
            fmt.Sprintf("%.1f", r.LossRate),
            strconv.Itoa(r.SentCount),
            strconv.Itoa(r.RecvCount),
            r.ErrorMsg,
        }
        if err := cw.Write(row); err != nil {
            return nil, fmt.Errorf("写入 CSV 行失败: %w", err)
        }
    }

    cw.Flush()
    if err := cw.Error(); err != nil {
        return nil, fmt.Errorf("生成 CSV 失败: %w", err)
    }

    return &PingCSVResult{
        FileName: "BatchPing_结果.csv",
        Content:  sb.String(),
    }, nil
}

// GetPingDefaultConfig 获取默认 Ping 配置 (Wails Binding)
func (s *PingService) GetPingDefaultConfig() icmp.PingConfig {
    return icmp.DefaultPingConfig()
}

// GetDeviceIPsForPing 获取设备库中的设备 IP 列表（供导入使用）
func (s *PingService) GetDeviceIPsForPing(deviceIDs []uint) ([]string, error) {
    if len(deviceIDs) == 0 {
        // 返回全部设备 IP
        devices, err := s.repo.FindAll()
        if err != nil {
            return nil, err
        }
        ips := make([]string, 0, len(devices))
        for _, d := range devices {
            ips = append(ips, d.IP)
        }
        return ips, nil
    }

    ips := make([]string, 0, len(deviceIDs))
    for _, id := range deviceIDs {
        device, err := s.repo.FindByID(id)
        if err != nil {
            continue
        }
        ips = append(ips, device.IP)
    }
    return ips, nil
}

// ========== 私有方法 ==========

// resolveTargets 解析用户输入的目标为 IP 列表
// 支持格式：单 IP、IP 范围、CIDR、多行/逗号混合输入、设备库 ID
func (s *PingService) resolveTargets(req PingRequest) ([]string, error) {
    ipSet := make(map[string]struct{})
    var result []string

    addIP := func(ip string) {
        ip = strings.TrimSpace(ip)
        if ip == "" {
            return
        }
        if _, exists := ipSet[ip]; !exists {
            ipSet[ip] = struct{}{}
            result = append(result, ip)
        }
    }

    // 1. 从设备库导入
    if len(req.DeviceIDs) > 0 {
        deviceIPs, err := s.GetDeviceIPsForPing(req.DeviceIDs)
        if err != nil {
            return nil, fmt.Errorf("从设备库获取 IP 失败: %w", err)
        }
        for _, ip := range deviceIPs {
            addIP(ip)
        }
    }

    // 2. 解析文本输入
    lines := strings.FieldsFunc(req.Targets, func(r rune) bool {
        return r == '\n' || r == ','
    })

    for _, line := range lines {
        line = strings.TrimSpace(line)
        if line == "" {
            continue
        }

        // 尝试 CIDR 解析: 192.168.1.0/24
        if strings.Contains(line, "/") {
            cidrIPs, err := expandCIDR(line)
            if err != nil {
                return nil, fmt.Errorf("CIDR 解析失败 [%s]: %w", line, err)
            }
            for _, ip := range cidrIPs {
                addIP(ip)
            }
            continue
        }

        // 尝试 IP 范围: 192.168.1.1-254 或 192.168.1.1~20
        rangeResult, rangeErr := parseIPv4LastOctetRange(line)
        if rangeErr != nil {
            return nil, fmt.Errorf("IP 范围解析失败 [%s]: %w", line, rangeErr)
        }
        if rangeResult != nil {
            for _, ip := range rangeResult.List {
                addIP(ip)
            }
            continue
        }

        // 单个 IP
        parsed := net.ParseIP(line)
        if parsed != nil && parsed.To4() != nil {
            addIP(line)
            continue
        }

        return nil, fmt.Errorf("无法识别的输入格式: %s", line)
    }

    return result, nil
}

// expandCIDR 将 CIDR 展开为主机 IP 列表
func expandCIDR(cidr string) ([]string, error) {
    prefix, err := netip.ParsePrefix(cidr)
    if err != nil {
        return nil, fmt.Errorf("无效的 CIDR: %s", cidr)
    }
    if !prefix.Addr().Is4() {
        return nil, fmt.Errorf("仅支持 IPv4 CIDR: %s", cidr)
    }

    bits := prefix.Bits()
    if bits < 16 {
        return nil, fmt.Errorf("CIDR 范围过大 (/%d)，最小支持 /16", bits)
    }

    hostBits := 32 - bits
    totalHosts := uint32(1) << uint(hostBits)
    
    // 获取网络地址
    addr := prefix.Addr()
    addrBytes := addr.As4()
    networkLong := uint32(addrBytes[0])<<24 | uint32(addrBytes[1])<<16 |
        uint32(addrBytes[2])<<8 | uint32(addrBytes[3])

    // 展开为主机列表（排除网络地址和广播地址）
    var ips []string
    start := uint32(1)
    end := totalHosts - 1
    if bits >= 31 {
        start = 0
        end = totalHosts
    }
    for i := start; i < end; i++ {
        hostIP := networkLong + i
        ip := fmt.Sprintf("%d.%d.%d.%d",
            byte(hostIP>>24), byte(hostIP>>16), byte(hostIP>>8), byte(hostIP))
        ips = append(ips, ip)
    }
    return ips, nil
}

// mergeWithDefaultPingConfig 将用户配置与默认配置合并
func mergeWithDefaultPingConfig(userConfig icmp.PingConfig) icmp.PingConfig {
    defaults := icmp.DefaultPingConfig()
    if userConfig.Timeout == 0 {
        userConfig.Timeout = defaults.Timeout
    }
    if userConfig.Count == 0 {
        userConfig.Count = defaults.Count
    }
    if userConfig.DataSize == 0 {
        userConfig.DataSize = defaults.DataSize
    }
    if userConfig.Concurrency == 0 {
        userConfig.Concurrency = defaults.Concurrency
    }
    // 安全边界
    if userConfig.Timeout < 100 {
        userConfig.Timeout = 100
    }
    if userConfig.Timeout > 30000 {
        userConfig.Timeout = 30000
    }
    if userConfig.Count < 1 {
        userConfig.Count = 1
    }
    if userConfig.Count > 10 {
        userConfig.Count = 10
    }
    if userConfig.Concurrency < 1 {
        userConfig.Concurrency = 1
    }
    if userConfig.Concurrency > 256 {
        userConfig.Concurrency = 256
    }
    if userConfig.DataSize < 1 {
        userConfig.DataSize = 1
    }
    if userConfig.DataSize > 1024 {
        userConfig.DataSize = 1024
    }
    return userConfig
}
```

---

### 3.3 Wails 主入口注册

在 `cmd/netweaver/main.go` 的 `runGUI()` 中注册新服务：

```diff
  // 创建各独立服务实例
  deviceService := ui.NewDeviceService()
+ pingService := ui.NewPingService()
  commandGroupService := ui.NewCommandGroupService()
  ...

  Services: []application.Service{
      application.NewService(deviceService),
+     application.NewService(pingService),
      application.NewService(commandGroupService),
      ...
  },
```

---

## 四、前端详细设计

### 4.1 页面路由

在 `frontend/src/router/index.ts` 中新增路由：

```typescript
const BatchPing = () => import('../views/Tools/BatchPing.vue')

// 在 routes 数组中添加:
{
    path: '/tools/ping',
    name: 'BatchPing',
    component: BatchPing
}
```

### 4.2 侧边栏导航

在 `App.vue` 侧栏 "工具箱" 分组下新增菜单项：

```
🏓 批量 Ping   →  /tools/ping
```

### 4.3 前端页面结构 (`BatchPing.vue`)

页面分为三个主要区域：

```
┌──────────────────────────────────────────────────┐
│  批量 Ping 检测                          [▶ 开始] │
├────────────────────┬─────────────────────────────┤
│                    │  ┌── 配置面板 ────────────┐  │
│  目标输入区         │  │ 超时:  [1000] ms       │  │
│  ┌──────────────┐  │  │ 重试:  [1]    次       │  │
│  │192.168.1.0/24│  │  │ 并发:  [64]            │  │
│  │10.0.0.1-100  │  │  │ 包大小: [32]  bytes    │  │
│  │172.16.1.1    │  │  │ 间隔:  [0]   ms       │  │
│  └──────────────┘  │  └───────────────────────┘  │
│  [导入设备IP]       │                             │
├────────────────────┴─────────────────────────────┤
│  ┌── 进度条 ────────────────────────────────┐    │
│  │ ████████████░░░░  75%  150/200  12.3s   │    │
│  │ 🟢 120 在线  🔴 30 离线  ⚠️ 0 错误       │    │
│  └──────────────────────────────────────────┘    │
├──────────────────────────────────────────────────┤
│  结果列表                       [导出CSV] [清空]  │
│  ┌──┬──────────────┬────┬─────┬────┬──────────┐ │
│  │# │ IP 地址       │状态│延迟  │TTL │ 丢包率   │ │
│  ├──┼──────────────┼────┼─────┼────┼──────────┤ │
│  │1 │192.168.1.1   │🟢  │2ms  │64  │ 0%       │ │
│  │2 │192.168.1.2   │🔴  │ -   │ -  │ 100%     │ │
│  │3 │192.168.1.3   │🟢  │5ms  │128 │ 0%       │ │
│  └──┴──────────────┴────┴─────┴────┴──────────┘ │
└──────────────────────────────────────────────────┘
```

### 4.4 前端核心逻辑

```typescript
// 关键数据流

// 1. 调用后端启动 Ping
const result = await PingService.StartBatchPing(request)

// 2. 监听 Wails Events 实时更新
wails.Events.On("ping:progress", (progress) => {
    updateResults(progress)
})

// 3. 停止 Ping
await PingService.StopBatchPing()

// 4. 导出 CSV
const csv = await PingService.ExportPingResultCSV()
downloadCSV(csv.fileName, csv.content)

// 5. 轮询兜底（可选，防止 Event 丢失）
const poll = setInterval(async () => {
    const progress = await PingService.GetPingProgress()
    if (progress && !progress.isRunning) clearInterval(poll)
    updateResults(progress)
}, 1000)
```

---

## 五、IP 解析能力汇总

| 输入格式 | 示例 | 解析结果 |
|---------|------|---------|
| 单个 IP | `192.168.1.1` | 1 个 IP |
| 最后八位范围 (连字符) | `192.168.1.1-254` | 254 个 IP |
| 最后八位范围 (波浪号) | `10.0.0.10~20` | 11 个 IP |
| CIDR | `192.168.1.0/24` | 254 个 IP (去除网络/广播) |
| CIDR (小范围) | `10.0.0.0/30` | 2 个 IP |
| 多行混合 | 多行文本 | 合并去重 |
| 设备库导入 | 选择设备 ID | 自动提取 IP |

> **复用说明**: 
> - `192.168.1.1-254` 格式复用项目已有 `parseIPv4LastOctetRange()` 函数
> - CIDR 格式使用新增的 `expandCIDR()` 函数，基于 Go 标准库 `net/netip`
> - 自动去重，避免重复 Ping 同一 IP

---

## 六、配置参数说明

| 参数 | 字段名 | 默认值 | 范围 | 说明 |
|------|-------|--------|------|------|
| 超时时间 | `timeout` | 1000ms | 100-30000ms | 单次 ICMP Echo 等待超时 |
| 请求间隔 | `interval` | 0ms | 0-5000ms | 相邻 IP 请求发起间隔 |
| 重试次数 | `count` | 1 | 1-10 | 每个 IP 发送 ICMP 请求的次数 |
| 数据包大小 | `dataSize` | 32 bytes | 1-1024 | ICMP 数据包 payload 大小 |
| 并发数 | `concurrency` | 64 | 1-256 | 同时探测的最大主机数 |

---

## 七、错误处理策略

### 7.1 输入阶段

| 场景 | 处理方式 |
|------|---------|
| 空输入 | 前端禁用「开始」按钮 |
| CIDR 掩码 < /16 | 拒绝展开，提示 IP 数量过大 |
| IP 数量 > 10000 | 拒绝执行，提示数量超限 |
| 非法格式 | 返回具体格式错误信息 |
| 重复 IP | 自动去重，不报错 |

### 7.2 执行阶段

| 场景 | 处理方式 |
|------|---------|
| 目标不可达 | 标记为 `offline`，记录状态码 |
| ICMP API 调用失败 | 标记为 `error`，记录错误信息 |
| 已有任务执行中 | 拒绝启动，提示需先停止 |
| 用户手动停止 | 已完成的保留结果，未执行的不再探测 |

---

## 八、文件清单与修改范围

### 8.1 新增文件

| 文件 | 说明 |
|------|------|
| `internal/icmp/types.go` | ICMP 类型定义（PingResult, PingConfig 等） |
| `internal/icmp/icmp_windows.go` | Windows ICMP API syscall 封装 |
| `internal/icmp/engine.go` | 批量 Ping 执行引擎 |
| `internal/ui/ping_service.go` | Wails Binding 服务层 |
| `frontend/src/views/Tools/BatchPing.vue` | 前端批量 Ping 页面 |

### 8.2 修改文件

| 文件 | 修改内容 |
|------|---------|
| `cmd/netweaver/main.go` | 注册 `PingService` |
| `frontend/src/router/index.ts` | 新增 `/tools/ping` 路由 |
| `frontend/src/App.vue` | 侧栏新增 "批量 Ping" 菜单 |

---

## 九、数据流架构图

```
┌─────────────┐     Wails Binding      ┌──────────────────┐
│  BatchPing   │ ───────────────────►  │   PingService     │
│   .vue       │  StartBatchPing()     │  (ping_service.go)│
│  (前端页面)   │  StopBatchPing()      │                  │
│              │  GetPingProgress()    │  resolveTargets() │
│              │  ExportPingResultCSV()│  expandCIDR()     │
│              │ ◄─────────────────── │  mergeConfig()    │
│              │  Wails Event          │                  │
│              │  "ping:progress"      └────────┬─────────┘
└─────────────┘                                 │
                                                │ 调用
                                                ▼
                                    ┌──────────────────────┐
                                    │  BatchPingEngine      │
                                    │  (engine.go)          │
                                    │                      │
                                    │  goroutine pool      │
                                    │  ├─ worker 1 ──┐     │
                                    │  ├─ worker 2 ──┤     │
                                    │  ├─ ...       ...    │
                                    │  └─ worker N ──┘     │
                                    └────────┬─────────────┘
                                             │ 调用
                                             ▼
                                 ┌─────────────────────────┐
                                 │  PingOne()               │
                                 │  (icmp_windows.go)       │
                                 │                         │
                                 │  syscall Iphlpapi.dll   │
                                 │  ├─ IcmpCreateFile      │
                                 │  ├─ IcmpSendEcho        │
                                 │  └─ IcmpCloseHandle     │
                                 └─────────────────────────┘
```

---

## 十、与 PingInfoView 功能对标

| 功能 | PingInfoView | 本方案 |
|------|-------------|--------|
| 批量 IP | ✅ | ✅ |
| CIDR 网段 | ✅ | ✅ |
| IP 范围 | ✅ | ✅ |
| Windows ICMP API | ✅ (推测) | ✅ |
| 无需管理员权限 | ✅ | ✅ |
| 实时结果显示 | ✅ | ✅ (Wails Events) |
| 自定义超时/大小 | ✅ | ✅ |
| 导出 CSV | ✅ | ✅ |
| 从设备库导入 | ❌ | ✅ (本项目特色) |
| GUI 集成 | 独立工具 | 集成在 NetWeaverGo 内 |

---

## 十一、测试计划

### 11.1 单元测试

| 测试文件 | 覆盖内容 |
|---------|---------|
| `internal/icmp/icmp_windows_test.go` | `PingOne()` 对 localhost 127.0.0.1 的基本测试 |
| `internal/ui/ping_service_test.go` | `resolveTargets()` 各种格式解析、`expandCIDR()` 边界情况 |

### 11.2 手动验证

| 验证场景 | 预期 |
|---------|------|
| Ping 本机 127.0.0.1 | 在线，RTT < 1ms |
| Ping 网关 | 在线，RTT 正常 |
| Ping 不存在的 IP | 离线，显示超时 |
| CIDR /24 展开 | 生成 254 个 IP |
| 并发 256 探测 | 正常完成无崩溃 |
| 执行中停止 | 已完成的保留，立即停止 |
| CSV 导出 | Excel 打开中文不乱码 |

---

## 十二、后续演进方向

| 方向 | 说明 | 优先级 |
|------|------|--------|
| 定时巡检 | 周期性自动 Ping 指定目标 | P2 |
| 历史记录 | 持久化保存 Ping 结果到数据库 | P2 |
| Traceroute | 路由追踪功能 | P3 |
| IPv6 支持 | 使用 `Icmp6CreateFile` API | P3 |
| 结果可视化 | 延迟趋势图、网段热力图 | P3 |
| 告警通知 | 主机下线时桌面通知 | P3 |
