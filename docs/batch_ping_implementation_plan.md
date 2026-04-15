# 批量 Ping 功能 — 分阶段实施方案

> **版本**: v1.0  
> **日期**: 2026-04-15  
> **基于文档**: `batch_ping_design.md`

---

## 一、实施概述

### 1.1 实施目标

根据 [`batch_ping_design.md`](batch_ping_design.md) 设计文档，将批量 Ping 功能开发工作分解为 **5 个阶段**，每个阶段独立可测试、可交付，确保开发过程可控、风险可控。

### 1.2 阶段划分

| 阶段 | 名称 | 预估工时 | 核心交付物 |
|------|------|----------|-----------|
| Phase 1 | ICMP 核心引擎 | 1 天 | `internal/icmp/` 模块，可独立测试 |
| Phase 2 | 服务层与 API | 0.5 天 | `internal/ui/ping_service.go`，Wails Binding |
| Phase 3 | 前端页面开发 | 1.5 天 | `BatchPing.vue`，完整 UI 交互 |
| Phase 4 | 集成与路由 | 0.5 天 | 路由注册、侧栏菜单、端到端联调 |
| Phase 5 | 测试与优化 | 1 天 | 单元测试、边界测试、性能优化 |

**总计**: 约 4.5 个工作日

---

## 二、Phase 1: ICMP 核心引擎

### 2.1 阶段目标

实现 Windows ICMP API 的底层封装，提供单次 Ping 和批量 Ping 的核心能力。

### 2.2 任务清单

| # | 任务 | 文件 | 说明 |
|---|------|------|------|
| 1.1 | 创建 ICMP 类型定义 | `internal/icmp/types.go` | 定义 `PingResult`、`PingConfig`、`PingHostResult`、`BatchPingProgress` |
| 1.2 | 实现 Windows API 封装 | `internal/icmp/icmp_windows.go` | 封装 `IcmpCreateFile`、`IcmpSendEcho`、`IcmpCloseHandle` |
| 1.3 | 实现 `PingOne()` 函数 | `internal/icmp/icmp_windows.go` | 单次 ICMP Echo Request |
| 1.4 | 实现状态码转换 | `internal/icmp/icmp_windows.go` | `icmpStatusToString()` |
| 1.5 | 实现批量 Ping 引擎 | `internal/icmp/engine.go` | `BatchPingEngine` 结构体及方法 |
| 1.6 | 实现并发控制 | `internal/icmp/engine.go` | goroutine pool + semaphore |
| 1.7 | 实现进度回调 | `internal/icmp/engine.go` | `onUpdate` 回调机制 |
| 1.8 | 实现取消机制 | `internal/icmp/engine.go` | `context.Context` 取消支持 |

### 2.3 文件结构

```
internal/icmp/
├── types.go              # 类型定义
├── icmp_windows.go       # Windows ICMP API 封装
└── engine.go             # 批量 Ping 引擎
```

### 2.4 关键代码规范

#### 2.4.1 Windows 结构体对齐

```go
// ⚠️ 关键：64 位平台必须使用 32 位版本结构体
type ICMP_ECHO_REPLY struct {
    Address       uint32 // 网络字节序
    Status        uint32
    RoundTripTime uint32
    DataSize      uint16
    Reserved      uint16
    DataPointer   uint32 // ⚠️ 必须用 uint32，非 uintptr
    Options       IP_OPTION_INFORMATION32
}

type IP_OPTION_INFORMATION32 struct {
    TTL         uint8
    Tos         uint8
    Flags       uint8
    OptionsSize uint8
    OptionsData uint32 // ⚠️ 必须用 uint32
}
```

#### 2.4.2 并发模型

```go
// 使用 semaphore 控制并发数
sem := make(chan struct{}, e.config.Concurrency)
var wg sync.WaitGroup

for i, ip := range ips {
    sem <- struct{}{}
    wg.Add(1)
    go func(index int, targetIP string) {
        defer wg.Done()
        defer func() { <-sem }()
        // 执行 Ping
    }(i, ip)
}
wg.Wait()
```

### 2.5 验收标准

| 验收项 | 验证方法 |
|--------|----------|
| 单 IP Ping 成功 | 调用 `PingOne("127.0.0.1", 1000, 32)` 返回 `Success=true` |
| 单 IP Ping 超时 | 调用 `PingOne("10.255.255.1", 500, 32)` 返回 `Success=false` |
| 批量 Ping 进度回调 | `BatchPingEngine.Run()` 每完成一个 IP 触发回调 |
| 取消机制有效 | 调用 `Stop()` 后立即停止，未完成的 IP 不再探测 |
| 并发数限制有效 | 设置 `Concurrency=10`，同时运行的 goroutine 不超过 10 |

### 2.6 独立测试

创建 `internal/icmp/icmp_windows_test.go`：

```go
func TestPingOne_Localhost(t *testing.T) {
    ip := net.ParseIP("127.0.0.1")
    result, err := PingOne(ip, 1000, 32)
    assert.NoError(t, err)
    assert.True(t, result.Success)
    assert.Equal(t, "127.0.0.1", result.IP)
}

func TestBatchPingEngine_SmallRange(t *testing.T) {
    config := DefaultPingConfig()
    config.Concurrency = 4
    engine := NewBatchPingEngine(config)
    
    ips := []string{"127.0.0.1", "127.0.0.2"}
    var progressSnapshots []*BatchPingProgress
    
    engine.Run(context.Background(), ips, func(p *BatchPingProgress) {
        progressSnapshots = append(progressSnapshots, p)
    })
    
    assert.Equal(t, 2, len(progressSnapshots[len(progressSnapshots)-1].Results))
}
```

---

## 三、Phase 2: 服务层与 API

### 3.1 阶段目标

实现 Wails Binding 服务层，提供前端可调用的 API 接口。

### 3.2 任务清单

| # | 任务 | 文件 | 说明 |
|---|------|------|------|
| 2.1 | 创建 PingService | `internal/ui/ping_service.go` | 服务结构体定义 |
| 2.2 | 实现 `StartBatchPing()` | `internal/ui/ping_service.go` | 启动批量 Ping |
| 2.3 | 实现 `StopBatchPing()` | `internal/ui/ping_service.go` | 停止批量 Ping |
| 2.4 | 实现 `GetPingProgress()` | `internal/ui/ping_service.go` | 获取当前进度 |
| 2.5 | 实现 `ExportPingResultCSV()` | `internal/ui/ping_service.go` | 导出 CSV |
| 2.6 | 实现 `GetPingDefaultConfig()` | `internal/ui/ping_service.go` | 获取默认配置 |
| 2.7 | 实现 `GetDeviceIPsForPing()` | `internal/ui/ping_service.go` | 从设备库导入 IP |
| 2.8 | 实现 `resolveTargets()` | `internal/ui/ping_service.go` | 解析目标 IP 列表 |
| 2.9 | 实现 `expandCIDR()` | `internal/ui/ping_service.go` | CIDR 展开 |
| 2.10 | 实现 `mergeWithDefaultPingConfig()` | `internal/ui/ping_service.go` | 配置合并与校验 |

### 3.3 API 接口定义

| 方法 | 参数 | 返回值 | 说明 |
|------|------|--------|------|
| `StartBatchPing` | `PingRequest` | `*BatchPingProgress, error` | 启动批量 Ping |
| `StopBatchPing` | - | `error` | 停止批量 Ping |
| `GetPingProgress` | - | `*BatchPingProgress` | 获取当前进度 |
| `ExportPingResultCSV` | - | `*PingCSVResult, error` | 导出 CSV |
| `GetPingDefaultConfig` | - | `PingConfig` | 获取默认配置 |
| `GetDeviceIPsForPing` | `[]uint` | `[]string, error` | 获取设备 IP |

### 3.4 IP 解析逻辑

```
输入文本
    │
    ├─► 检测 CIDR 格式 (含 "/") ──► expandCIDR()
    │
    ├─► 检测 IP 范围 (含 "-" 或 "~") ──► parseIPv4LastOctetRange()
    │
    └─► 单个 IP ──► net.ParseIP()
    
合并去重 ──► 最终 IP 列表
```

### 3.5 Wails Event 推送

```go
// 在 engine.Run() 回调中推送事件
go func() {
    s.engine.Run(context.Background(), ips, func(progress *icmp.BatchPingProgress) {
        if s.wailsApp != nil && s.wailsApp.Event != nil {
            s.wailsApp.Event.Emit("ping:progress", progress)
        }
    })
}()
```

### 3.6 验收标准

| 验收项 | 验证方法 |
|--------|----------|
| 单 IP 解析 | `resolveTargets("192.168.1.1")` 返回 1 个 IP |
| CIDR 解析 | `resolveTargets("192.168.1.0/30")` 返回 2 个 IP |
| IP 范围解析 | `resolveTargets("192.168.1.1-3")` 返回 3 个 IP |
| 设备库导入 | `GetDeviceIPsForPing([1,2])` 返回对应设备 IP |
| CSV 导出 | `ExportPingResultCSV()` 返回 UTF-8 BOM 格式 CSV |
| 事件推送 | 前端收到 `ping:progress` 事件 |

---

## 四、Phase 3: 前端页面开发

### 4.1 阶段目标

开发完整的批量 Ping 前端页面，实现目标输入、配置面板、进度展示、结果列表等功能。

### 4.2 任务清单

| # | 任务 | 说明 |
|---|------|------|
| 3.1 | 创建页面文件 | `frontend/src/views/Tools/BatchPing.vue` |
| 3.2 | 实现目标输入区 | textarea + 格式提示 |
| 3.3 | 实现配置面板 | 超时、重试、并发、包大小、间隔 |
| 3.4 | 实现进度条 | 进度百分比 + 统计信息 |
| 3.5 | 实现结果列表 | 表格展示 + 状态图标 |
| 3.6 | 实现导出功能 | CSV 下载 |
| 3.7 | 实现设备导入 | 弹窗选择设备 |
| 3.8 | 实现取消功能 | 停止按钮 |
| 3.9 | 实现事件监听 | Wails Events 监听 `ping:progress` |
| 3.10 | 实现轮询兜底 | 防止 Event 丢失的轮询机制 |

### 4.3 页面布局

```
┌──────────────────────────────────────────────────────────┐
│  批量 Ping 检测                           [▶ 开始] [■ 停止] │
├────────────────────┬─────────────────────────────────────┤
│                    │  ┌── 配置面板 ────────────────────┐  │
│  目标输入区         │  │ 超时:  [1000] ms               │  │
│  ┌──────────────┐  │  │ 重试:  [1]    次               │  │
│  │192.168.1.0/24│  │  │ 并发:  [64]                    │  │
│  │10.0.0.1-100  │  │  │ 包大小: [32]  bytes            │  │
│  │172.16.1.1    │  │  │ 间隔:  [0]   ms                │  │
│  └──────────────┘  │  └────────────────────────────────┘  │
│  [导入设备IP]       │                                      │
├────────────────────┴─────────────────────────────────────┤
│  ┌── 进度条 ───────────────────────────────────────────┐  │
│  │ ████████████░░░░  75%  150/200  12.3s              │  │
│  │ 🟢 120 在线  🔴 30 离线  ⚠️ 0 错误                  │  │
│  └─────────────────────────────────────────────────────┘  │
├──────────────────────────────────────────────────────────┤
│  结果列表                             [导出CSV] [清空]     │
│  ┌──┬──────────────┬────┬─────┬────┬──────────┐         │
│  │# │ IP 地址       │状态│延迟  │TTL │ 丢包率   │         │
│  ├──┼──────────────┼────┼─────┼────┼──────────┤         │
│  │1 │192.168.1.1   │🟢  │2ms  │64  │ 0%       │         │
│  │2 │192.168.1.2   │🔴  │ -   │ -  │ 100%     │         │
│  └──┴──────────────┴────┴─────┴────┴──────────┘         │
└──────────────────────────────────────────────────────────┘
```

### 4.4 核心数据结构

```typescript
interface PingConfig {
  timeout: number      // 超时 (ms)
  interval: number     // 间隔 (ms)
  count: number        // 重试次数
  dataSize: number     // 包大小 (bytes)
  concurrency: number  // 并发数
}

interface PingHostResult {
  ip: string
  alive: boolean
  sentCount: number
  recvCount: number
  lossRate: number
  minRtt: number
  maxRtt: number
  avgRtt: number
  ttl: number
  status: 'online' | 'offline' | 'error' | 'pending'
  errorMsg?: string
}

interface BatchPingProgress {
  totalIPs: number
  completedIPs: number
  onlineCount: number
  offlineCount: number
  errorCount: number
  progress: number       // 0-100
  isRunning: boolean
  elapsedMs: number
  results: PingHostResult[]
}
```

### 4.5 核心逻辑流程

```typescript
// 1. 启动 Ping
async function startPing() {
  const request = {
    targets: targetInput.value,
    config: config.value,
    deviceIds: selectedDeviceIds.value
  }
  const result = await PingService.StartBatchPing(request)
  updateProgress(result)
}

// 2. 监听实时进度
import { Events } from '@wailsio/runtime'

Events.On('ping:progress', (progress: BatchPingProgress) => {
  updateProgress(progress)
})

// 3. 停止 Ping
async function stopPing() {
  await PingService.StopBatchPing()
}

// 4. 导出 CSV
async function exportCSV() {
  const csv = await PingService.ExportPingResultCSV()
  downloadCSV(csv.fileName, csv.content)
}

// 5. 轮询兜底
const pollInterval = setInterval(async () => {
  const progress = await PingService.GetPingProgress()
  if (progress && !progress.isRunning) {
    clearInterval(pollInterval)
  }
  updateProgress(progress)
}, 1000)
```

### 4.6 验收标准

| 验收项 | 验证方法 |
|--------|----------|
| 页面正常渲染 | 访问 `/tools/ping` 页面显示正常 |
| 目标输入有效 | 输入多种格式 IP，解析正确 |
| 配置修改生效 | 修改配置后 Ping 行为符合预期 |
| 进度实时更新 | 执行过程中进度条实时刷新 |
| 结果正确展示 | 在线/离线状态正确，延迟显示正确 |
| CSV 导出成功 | 导出文件 Excel 打开中文不乱码 |
| 停止功能有效 | 点击停止后立即停止探测 |

---

## 五、Phase 4: 集成与路由

### 5.1 阶段目标

将批量 Ping 功能集成到主应用，完成路由注册和侧栏菜单配置。

### 5.2 任务清单

| # | 任务 | 文件 | 说明 |
|---|------|------|------|
| 4.1 | 注册 PingService | `cmd/netweaver/main.go` | 在 Wails 应用中注册服务 |
| 4.2 | 添加前端路由 | `frontend/src/router/index.ts` | 新增 `/tools/ping` 路由 |
| 4.3 | 添加侧栏菜单 | `frontend/src/App.vue` | 在"工具箱"分组下添加菜单项 |
| 4.4 | 端到端联调 | - | 完整流程测试 |

### 5.3 代码修改

#### 5.3.1 后端服务注册 (`cmd/netweaver/main.go`)

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

#### 5.3.2 前端路由 (`frontend/src/router/index.ts`)

```typescript
const BatchPing = () => import('../views/Tools/BatchPing.vue')

// 在 routes 数组中添加:
{
    path: '/tools/ping',
    name: 'BatchPing',
    component: BatchPing
}
```

#### 5.3.3 侧栏菜单 (`frontend/src/App.vue`)

```typescript
// 在工具箱分组下添加:
{
    icon: '🏓',
    label: '批量 Ping',
    path: '/tools/ping'
}
```

### 5.4 验收标准

| 验收项 | 验证方法 |
|--------|----------|
| 服务注册成功 | 应用启动无报错，前端可调用 API |
| 路由跳转正常 | 点击侧栏菜单跳转到 `/tools/ping` |
| 页面加载正常 | 页面渲染正确，无控制台错误 |
| 完整流程可用 | 从输入到导出完整流程测试通过 |

---

## 六、Phase 5: 测试与优化

### 6.1 阶段目标

完成单元测试、边界测试、性能优化，确保功能稳定可靠。

### 6.2 任务清单

| # | 任务 | 说明 |
|---|------|------|
| 5.1 | ICMP 单元测试 | `internal/icmp/icmp_windows_test.go` |
| 5.2 | 服务层单元测试 | `internal/ui/ping_service_test.go` |
| 5.3 | 边界测试 | 大量 IP、极端参数、异常输入 |
| 5.4 | 性能测试 | 并发 256 探测性能 |
| 5.5 | 内存泄漏检查 | 长时间运行无内存泄漏 |
| 5.6 | 代码审查 | 代码规范、注释完整性 |

### 6.3 测试用例

#### 6.3.1 单元测试

| 测试文件 | 测试用例 |
|---------|---------|
| `icmp_windows_test.go` | `TestPingOne_Localhost` - 本机 Ping |
| `icmp_windows_test.go` | `TestPingOne_Timeout` - 超时测试 |
| `icmp_windows_test.go` | `TestBatchPingEngine_Cancel` - 取消测试 |
| `ping_service_test.go` | `TestResolveTargets_SingleIP` - 单 IP 解析 |
| `ping_service_test.go` | `TestResolveTargets_CIDR` - CIDR 解析 |
| `ping_service_test.go` | `TestResolveTargets_Range` - IP 范围解析 |
| `ping_service_test.go` | `TestExpandCIDR_Boundary` - CIDR 边界 |
| `ping_service_test.go` | `TestMergeConfig` - 配置合并 |

#### 6.3.2 手动验证

| 验证场景 | 预期结果 |
|---------|---------|
| Ping 本机 127.0.0.1 | 在线，RTT < 1ms |
| Ping 网关 | 在线，RTT 正常 |
| Ping 不存在的 IP | 离线，显示超时 |
| CIDR /24 展开 | 生成 254 个 IP |
| CIDR /16 拒绝 | 提示范围过大 |
| IP 数量 > 10000 | 拒绝执行 |
| 并发 256 探测 | 正常完成无崩溃 |
| 执行中停止 | 已完成的保留，立即停止 |
| CSV 导出 | Excel 打开中文不乱码 |
| 设备库导入 | 正确提取设备 IP |

### 6.4 性能指标

| 指标 | 目标值 |
|------|--------|
| 单次 Ping 延迟 | < 1ms (本机) |
| 254 IP 探测时间 | < 10s (并发 64) |
| 内存占用 | < 50MB (1000 IP) |
| CPU 占用 | < 30% (并发 256) |

### 6.5 验收标准

| 验收项 | 验证方法 |
|--------|----------|
| 单元测试通过 | `go test ./internal/icmp/... ./internal/ui/...` |
| 覆盖率达标 | 测试覆盖率 > 70% |
| 性能达标 | 254 IP 探测 < 10s |
| 无内存泄漏 | 长时间运行内存稳定 |

---

## 七、风险与应对

### 7.1 技术风险

| 风险 | 影响 | 应对措施 |
|------|------|---------|
| Windows API 兼容性 | 部分系统可能不支持 | 测试多版本 Windows |
| 64 位结构体对齐 | 数据解析错误 | 严格遵循 32 位结构体布局 |
| 高并发资源耗尽 | 系统卡顿 | 限制最大并发数 256 |
| Wails Event 丢失 | 前端进度不更新 | 轮询兜底机制 |

### 7.2 进度风险

| 风险 | 影响 | 应对措施 |
|------|------|---------|
| 前端开发超期 | 整体延期 | 优先实现核心功能，UI 美化后置 |
| 测试发现重大问题 | 返工 | 每个 Phase 完成后立即验证 |

---

## 八、交付清单

### 8.1 代码文件

| 文件 | 类型 | 说明 |
|------|------|------|
| `internal/icmp/types.go` | 新增 | ICMP 类型定义 |
| `internal/icmp/icmp_windows.go` | 新增 | Windows API 封装 |
| `internal/icmp/engine.go` | 新增 | 批量 Ping 引擎 |
| `internal/ui/ping_service.go` | 新增 | Wails 服务层 |
| `frontend/src/views/Tools/BatchPing.vue` | 新增 | 前端页面 |
| `cmd/netweaver/main.go` | 修改 | 服务注册 |
| `frontend/src/router/index.ts` | 修改 | 路由配置 |
| `frontend/src/App.vue` | 修改 | 侧栏菜单 |

### 8.2 测试文件

| 文件 | 说明 |
|------|------|
| `internal/icmp/icmp_windows_test.go` | ICMP 单元测试 |
| `internal/ui/ping_service_test.go` | 服务层单元测试 |

### 8.3 文档

| 文件 | 说明 |
|------|------|
| `docs/batch_ping_design.md` | 设计文档 |
| `docs/batch_ping_implementation_plan.md` | 本实施方案 |

---

## 九、里程碑

```
Day 1  ──► Phase 1 完成 ──► ICMP 核心引擎可用
Day 2  ──► Phase 2 完成 ──► 服务层 API 可用
Day 3  ──► Phase 3 完成 ──► 前端页面可用
Day 4  ──► Phase 4 完成 ──► 集成联调完成
Day 5  ──► Phase 5 完成 ──► 测试通过，功能交付
```

---

## 十、附录

### 10.1 参考文档

- [Windows ICMP API](https://learn.microsoft.com/en-us/windows/win32/api/iphlpapi/)
- [ICMP_ECHO_REPLY32 structure](https://learn.microsoft.com/en-us/windows/win32/api/ipexport/ns-ipexport-icmp_echo_reply)
- [Wails v3 Documentation](https://wails.io/docs/next/)

### 10.2 相关代码参考

- [`internal/ui/network_calc_service.go`](../internal/ui/network_calc_service.go) - IP 解析函数
- [`internal/ui/forge_service.go`](../internal/ui/forge_service.go) - IP 范围解析
- [`internal/ui/taskexec_event_bridge.go`](../internal/ui/taskexec_event_bridge.go) - Wails Event 推送
