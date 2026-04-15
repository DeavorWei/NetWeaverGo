# 批量 Ping 功能 - 中优先级问题修复方案

> **文档版本**: 2.0  
> **创建日期**: 2026-04-15  
> **更新日期**: 2026-04-15  
> **关联文档**: `batch_ping_final_issues.md`, `batch_ping_medium_priority_fix_plan_analysis.md`

---

## 一、问题汇总

中优先级共 **8个问题**（原 10 个问题中 #5 和 #7 经验证不存在），涉及后端逻辑、数据处理和前端交互。

| 序号 | 问题 | 严重程度 | 文件位置 | 修复复杂度 | 状态 |
|------|------|---------|---------|-----------|------|
| 6 | 缺少 Interval 参数上限校验 | 🟡 中 | `ping_service.go:398-412` | 低 | 待修复 |
| 8 | minRtt 初始值问题 | 🟡 中 | `engine.go:138` | 低 | 待修复 |
| 9 | 结果顺序与输入顺序不一致 | 🟡 中 | `engine.go:81-121` | 中 | 待修复 |
| 10 | 缺少回调 panic 恢复机制 | 🟡 中 | `engine.go:105-108` | 低 | 待修复 |
| 11 | IP 范围解析不支持逗号分隔 | 🟡 中 | `ping_service.go:225` | 低 | 待修复 |
| 12 | 前端缺少设备导入 UI | 🟡 中 | `BatchPing.vue` | 中 | 待修复 |
| 13 | 前端未处理启动错误提示 | 🟡 中 | `BatchPing.vue:22-34` | 低 | 待修复 |
| 14 | Events.Off 未传入回调引用 | 🟡 中 | `BatchPing.vue:131` | 低 | 待修复 |

### 已验证无问题

| 序号 | 问题 | 验证结果 |
|------|------|---------|
| 5 | expandCIDR 对 /31 和 /32 处理逻辑有误 | ❌ 当前代码正确，/31 返回 2 个地址，/32 返回 1 个地址 |
| 7 | 取消时未保留已完成进度 | ❌ 当前代码已有 defer 调用 Finish()，状态会被正确设置 |

---

## 二、详细修复方案

### 问题 #6: 缺少 Interval 参数上限校验

#### 问题分析

当前 `mergeWithDefaultPingConfig` 对 Timeout、DataSize、Count、Concurrency 设置了上限，但没有对 Interval 设置上限。

```go
// 当前代码 (ping_service.go:398-412)
if config.Timeout > 10000 {
    config.Timeout = 10000
}
if config.DataSize > 65500 {
    config.DataSize = 65500
}
if config.Count > 10 {
    config.Count = 10
}
if config.Concurrency > 256 {
    config.Concurrency = 256
}
// ❌ 缺少 Interval 校验
```

#### 修复方案

```go
// ping_service.go:382-420
func (s *PingService) mergeWithDefaultPingConfig(config icmp.PingConfig) icmp.PingConfig {
    defaults := icmp.DefaultPingConfig()

    if config.Timeout == 0 {
        config.Timeout = defaults.Timeout
    }
    if config.DataSize == 0 {
        config.DataSize = defaults.DataSize
    }
    if config.Count == 0 {
        config.Count = defaults.Count
    }
    if config.Concurrency == 0 {
        config.Concurrency = defaults.Concurrency
    }
    if config.Interval == 0 {
        config.Interval = defaults.Interval
    }

    // Apply limits
    if config.Timeout > 10000 {
        config.Timeout = 10000 // Max 10 seconds
    }
    if config.DataSize > 65500 {
        config.DataSize = 65500 // Max ICMP data size
    }
    if config.Count > 10 {
        config.Count = 10 // Max 10 attempts
    }
    if config.Concurrency > 256 {
        config.Concurrency = 256 // Max 256 concurrent
    }
    // ✅ 添加 Interval 上限校验
    if config.Interval > 5000 {
        config.Interval = 5000 // Max 5 seconds between pings
    }

    return config
}
```

#### 前端同步更新

```vue
<!-- BatchPing.vue:243-253 -->
<div class="flex items-center justify-between">
  <label class="text-sm text-text-secondary">间隔 (ms)</label>
  <input
    v-model.number="config.Interval"
    type="number"
    :disabled="isRunning"
    min="0"
    max="5000"
    class="w-24 bg-bg-tertiary/50 border border-border rounded px-2 py-1 text-sm text-text-primary text-right focus:outline-none focus:border-accent"
  />
</div>
```

#### 测试用例

```go
// ping_service_test.go
func TestMergeWithDefaultPingConfig_IntervalLimit(t *testing.T) {
    svc := NewPingService()
    
    tests := []struct {
        name     string
        input    uint32
        expected uint32
    }{
        {"正常值", 100, 100},
        {"超过上限", 10000, 5000},
        {"零值使用默认", 0, 0},
        {"边界值5000", 5000, 5000},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            config := icmp.PingConfig{Interval: tt.input}
            result := svc.mergeWithDefaultPingConfig(config)
            if result.Interval != tt.expected {
                t.Errorf("Expected Interval %d, got %d", tt.expected, result.Interval)
            }
        })
    }
}
```

---

### 问题 #8: minRtt 初始值问题

#### 问题分析

当所有 ping 都失败时，`minRtt` 保持为 `^uint32(0)`（4294967295），这是一个无效值。

```go
// 当前问题代码 (engine.go:138)
var minRtt uint32 = ^uint32(0) // Max uint32 = 4294967295
// ...
if successCount > 0 {
    result.MinRtt = minRtt  // 正常
} else {
    // ❌ minRtt 保持为最大值，没有处理
    result.Alive = false
    result.Status = "offline"
}
```

#### 修复方案

使用 `0` 表示无效值，并在前端显示时处理：

```go
// engine.go:128-205
func (e *BatchPingEngine) pingHost(ctx context.Context, ip net.IP) PingHostResult {
    result := PingHostResult{
        IP:        ip.String(),
        Status:    "pending",
        SentCount: e.config.Count,
    }

    var successCount int
    var rttSum uint32
    var minRtt uint32 = 0 // ✅ 初始化为 0 表示无效
    var maxRtt uint32
    var lastTTTTL uint8

    for i := 0; i < e.config.Count; i++ {
        // ... ping logic ...
        
        if pingResult.Success {
            successCount++
            rttSum += pingResult.RoundTripTime
            // ✅ 仅在第一次成功或更小时更新 minRtt
            if minRtt == 0 || pingResult.RoundTripTime < minRtt {
                minRtt = pingResult.RoundTripTime
            }
            if pingResult.RoundTripTime > maxRtt {
                maxRtt = pingResult.RoundTripTime
            }
            lastTTL = pingResult.TTL
        }
        // ...
    }

    // Calculate statistics
    result.RecvCount = successCount
    if result.SentCount > 0 {
        result.LossRate = float64(result.SentCount-successCount) / float64(result.SentCount) * 100
    }

    if successCount > 0 {
        result.Alive = true
        result.Status = "online"
        result.AvgRtt = rttSum / uint32(successCount)
        result.MinRtt = minRtt
        result.MaxRtt = maxRtt
        result.TTL = lastTTL
    } else {
        result.Alive = false
        result.Status = "offline"
        // ✅ minRtt 保持为 0，前端会显示为 "-"
    }

    return result
}
```

#### 前端显示优化

```typescript
// BatchPing.vue
const formatRtt = (rtt: number, status: string): string => {
  // 离线或错误状态，或 rtt 为 0 时显示 "-"
  if (status !== 'online' || rtt === 0) return '-'
  return `${rtt}ms`
}

// 在模板中使用
<td class="py-2 px-3 text-text-primary">{{ formatRtt(result.minRtt, result.status) }}</td>
```

#### 测试用例

```go
// engine_test.go
func TestPingHost_MinRttWhenAllFail(t *testing.T) {
    config := PingConfig{
        Timeout: 100,
        Count:   2,
    }
    engine := NewBatchPingEngine(config)
    
    // 使用不存在的 IP
    ip := net.ParseIP("10.255.255.1")
    result := engine.pingHost(context.Background(), ip)
    
    if result.Alive {
        t.Error("Expected host to be offline")
    }
    if result.MinRtt != 0 {
        t.Errorf("Expected MinRtt to be 0 for failed ping, got %d", result.MinRtt)
    }
    if result.Status != "offline" {
        t.Errorf("Expected status 'offline', got %s", result.Status)
    }
}
```

---

### 问题 #9: 结果顺序与输入顺序不一致

#### 问题分析

并发执行导致结果按完成顺序添加到 Results 切片，而非输入顺序。

```go
// 当前问题代码 (engine.go:81-121)
go func(index int, targetIP string) {
    // ... ping ...
    progressMu.Lock()
    progress.AddResult(result)  // ❌ 按完成顺序添加
    progressMu.Unlock()
}(i, ipStr)
```

#### 修复方案

预分配固定大小的切片，按索引存储结果：

```go
// types.go:66-79 - 修改 NewBatchPingProgress
func NewBatchPingProgress(totalIPs int) *BatchPingProgress {
    return &BatchPingProgress{
        TotalIPs:     totalIPs,
        CompletedIPs: 0,
        OnlineCount:  0,
        OfflineCount: 0,
        ErrorCount:   0,
        Progress:     0,
        IsRunning:    true,
        StartTime:    time.Now(),
        ElapsedMs:    0,
        Results:      make([]PingHostResult, totalIPs), // ✅ 预分配固定大小
    }
}
```

```go
// types.go:89-115 - 新增 SetResult 方法
// SetResult sets a result at the specified index and updates counters.
// This method is thread-safe when called with the progressMu lock held.
func (p *BatchPingProgress) SetResult(index int, result PingHostResult) {
    if index < 0 || index >= p.TotalIPs {
        return
    }
    
    // 检查是否已设置（防止重复计数）
    if p.Results[index].Status != "" && p.Results[index].Status != "pending" {
        return
    }
    
    p.Results[index] = result
    p.CompletedIPs++

    switch result.Status {
    case "online":
        p.OnlineCount++
    case "offline":
        p.OfflineCount++
    case "error":
        p.ErrorCount++
    }

    p.UpdateProgress()
}
```

```go
// engine.go:70-130 - 修改 Run 方法
for i, ipStr := range ips {
    // Check for cancellation
    select {
    case <-runCtx.Done():
        return progress
    default:
    }

    sem <- struct{}{}
    wg.Add(1)

    go func(index int, targetIP string) {
        defer wg.Done()
        defer func() { <-sem }()

        // Check for cancellation before starting
        select {
        case <-runCtx.Done():
            progressMu.Lock()
            progress.SetResult(index, PingHostResult{
                IP:        targetIP,
                Status:    "error",
                ErrorMsg:  "Cancelled",
                SentCount: 1,
                RecvCount: 0,
                LossRate:  100,
            })
            progressMu.Unlock()
            return
        default:
        }

        // Parse IP
        ip := net.ParseIP(targetIP)
        if ip == nil {
            progressMu.Lock()
            progress.SetResult(index, PingHostResult{
                IP:        targetIP,
                Alive:     false,
                Status:    "error",
                ErrorMsg:  "Invalid IP address",
                SentCount: 1,
                RecvCount: 0,
                LossRate:  100,
            })
            safeCallback(onUpdate, progress)
            progressMu.Unlock()
            return
        }

        // Perform ping attempts
        result := e.pingHost(runCtx, ip)

        progressMu.Lock()
        progress.SetResult(index, result)  // ✅ 按索引存储
        safeCallback(onUpdate, progress)
        progressMu.Unlock()
    }(i, ipStr)
}
```

#### 测试用例

```go
// engine_test.go
func TestBatchPingEngine_ResultOrder(t *testing.T) {
    config := PingConfig{
        Timeout:     1000,
        Count:       1,
        Concurrency: 4,
    }
    engine := NewBatchPingEngine(config)
    
    ips := []string{"127.0.0.1", "10.0.0.1", "10.0.0.2", "10.0.0.3"}
    
    progress := engine.Run(context.Background(), ips, nil)
    
    // 验证结果顺序与输入一致
    for i, result := range progress.Results {
        if result.IP != ips[i] {
            t.Errorf("Result order mismatch at index %d: expected %s, got %s", 
                i, ips[i], result.IP)
        }
    }
}
```

---

### 问题 #10: 缺少回调 panic 恢复机制

#### 问题分析

`onUpdate` 回调如果发生 panic，会导致整个程序崩溃。

```go
// 当前问题代码 (engine.go:105-108)
if onUpdate != nil {
    onUpdate(progress)  // ❌ 没有 recover 保护
}
```

#### 修复方案

添加 defer recover 保护：

```go
// engine.go - 添加安全回调函数
func safeCallback(fn func(*BatchPingProgress), progress *BatchPingProgress) {
    if fn == nil {
        return
    }
    defer func() {
        if r := recover(); r != nil {
            // 记录 panic 信息，便于排查问题
            log.Printf("Ping progress callback panic recovered: %v", r)
        }
    }()
    fn(progress)
}

// 在 Run 方法中使用
progressMu.Lock()
progress.SetResult(index, result)
safeCallback(onUpdate, progress)  // ✅ 使用安全包装
progressMu.Unlock()
```

#### 测试用例

```go
// engine_test.go
func TestBatchPingEngine_CallbackPanicRecovery(t *testing.T) {
    config := PingConfig{
        Timeout:     1000,
        Count:       1,
        Concurrency: 1,
    }
    engine := NewBatchPingEngine(config)
    
    ips := []string{"127.0.0.1"}
    
    // 模拟 panic 的回调
    panicCallback := func(p *BatchPingProgress) {
        panic("test panic")
    }
    
    // 不应该 panic
    progress := engine.Run(context.Background(), ips, panicCallback)
    
    if progress == nil {
        t.Error("Expected progress to be returned even with panic callback")
    }
    
    // 验证结果正常
    if len(progress.Results) != 1 {
        t.Errorf("Expected 1 result, got %d", len(progress.Results))
    }
}
```

---

### 问题 #11: IP 范围解析不支持逗号分隔

#### 问题分析

当前只支持换行符分割，不支持逗号分隔的混合输入。

```go
// 当前代码 (ping_service.go:225)
lines := strings.Split(targets, "\n")
```

#### 修复方案

使用 `strings.FieldsFunc` 同时处理换行和逗号：

```go
// ping_service.go:207-246
func (s *PingService) resolveTargets(targets string, deviceIDs []uint) ([]string, error) {
    var allIPs []string
    seen := make(map[string]struct{})

    // Add IPs from device IDs
    deviceIPs, err := s.GetDeviceIPsForPing(deviceIDs)
    if err != nil {
        return nil, fmt.Errorf("获取设备 IP 失败: %w", err)
    }
    for _, ip := range deviceIPs {
        if _, exists := seen[ip]; !exists {
            seen[ip] = struct{}{}
            allIPs = append(allIPs, ip)
        }
    }

    // ✅ 使用 FieldsFunc 同时处理换行、逗号和空格
    lines := strings.FieldsFunc(targets, func(r rune) bool {
        return r == '\n' || r == ',' || r == ' ' || r == '\t' || r == ';'
    })
    
    for _, line := range lines {
        line = strings.TrimSpace(line)
        if line == "" {
            continue
        }

        ips, err := s.parseTargetLine(line)
        if err != nil {
            return nil, fmt.Errorf("解析目标 '%s' 失败: %w", line, err)
        }

        for _, ip := range ips {
            if _, exists := seen[ip]; !exists {
                seen[ip] = struct{}{}
                allIPs = append(allIPs, ip)
            }
        }
    }

    return allIPs, nil
}
```

#### 前端 UI 更新

```vue
<!-- BatchPing.vue - 更新 placeholder -->
<textarea
  v-model="targetInput"
  :disabled="isRunning"
  placeholder="输入 IP 地址&#10;支持格式：&#10;• 单个 IP: 192.168.1.1&#10;• CIDR: 192.168.1.0/24&#10;• 范围: 192.168.1.1-100&#10;• 多个 IP: 192.168.1.1, 192.168.1.2&#10;• 混合: 192.168.1.1, 192.168.1.0/30"
  class="w-full h-40 bg-bg-tertiary/50 border border-border rounded-lg p-3 text-sm text-text-primary placeholder-text-muted resize-none focus:outline-none focus:border-accent transition-colors"
></textarea>
```

#### 测试用例

```go
// ping_service_test.go
func TestResolveTargets_CommaSeparated(t *testing.T) {
    svc := NewPingService()

    tests := []struct {
        name     string
        input    string
        expected int
    }{
        {"换行分隔", "192.168.1.1\n192.168.1.2", 2},
        {"逗号分隔", "192.168.1.1,192.168.1.2,192.168.1.3", 3},
        {"混合分隔", "192.168.1.1, 192.168.1.2\n192.168.1.3;192.168.1.4", 4},
        {"带空格", "192.168.1.1 , 192.168.1.2 , 192.168.1.3", 3},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            ips, err := svc.resolveTargets(tt.input, nil)
            if err != nil {
                t.Fatalf("resolveTargets failed: %v", err)
            }
            if len(ips) != tt.expected {
                t.Errorf("Expected %d IPs, got %d: %v", tt.expected, len(ips), ips)
            }
        })
    }
}
```

---

### 问题 #12: 前端缺少设备导入 UI

#### 问题分析

后端已实现 `GetDeviceIPsForPing` API，但前端没有提供设备导入的界面。

#### 修复方案

添加设备选择弹窗组件：

```vue
<!-- BatchPing.vue -->
<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from 'vue'
import { Events } from '@wailsio/runtime'
import * as PingService from '@/bindings/github.com/NetWeaverGo/core/internal/ui/pingservice'
import * as DeviceService from '@/bindings/github.com/NetWeaverGo/core/internal/ui/deviceservice'
import type { PingConfig, BatchPingProgress } from '@/bindings/github.com/NetWeaverGo/core/internal/icmp/models'
import type { PingRequest } from '@/bindings/github.com/NetWeaverGo/core/internal/ui/models'
import type { Device } from '@/bindings/github.com/NetWeaverGo/core/internal/models/models'
import { useToast } from '@/utils/useToast'

const toast = useToast()

// State
const targetInput = ref('')
const config = ref<PingConfig>({
  Timeout: 1000,
  Interval: 0,
  Count: 1,
  DataSize: 32,
  Concurrency: 64
})

const progress = ref<BatchPingProgress | null>(null)
const isRunning = computed(() => progress.value?.isRunning ?? false)

// ✅ 设备导入相关状态
const showDeviceModal = ref(false)
const devices = ref<Device[]>([])
const selectedDeviceIds = ref<number[]>([])
const loadingDevices = ref(false)
const deviceSearchQuery = ref('')

// ✅ 过滤后的设备列表
const filteredDevices = computed(() => {
  if (!deviceSearchQuery.value) return devices.value
  const query = deviceSearchQuery.value.toLowerCase()
  return devices.value.filter(d => 
    d.name.toLowerCase().includes(query) ||
    d.ip.toLowerCase().includes(query)
  )
})

// ✅ 加载设备列表
const loadDevices = async () => {
  loadingDevices.value = true
  try {
    const result = await DeviceService.GetAllDevices()
    devices.value = result || []
  } catch (err) {
    toast.error('加载设备列表失败')
    console.error('Failed to load devices:', err)
  } finally {
    loadingDevices.value = false
  }
}

// ✅ 打开设备选择弹窗
const openDeviceModal = async () => {
  await loadDevices()
  selectedDeviceIds.value = []
  deviceSearchQuery.value = ''
  showDeviceModal.value = true
}

// ✅ 确认导入设备
const importDevices = async () => {
  if (selectedDeviceIds.value.length === 0) {
    toast.warning('请选择至少一个设备')
    return
  }
  
  try {
    const ips = await PingService.GetDeviceIPsForPing(selectedDeviceIds.value)
    if (ips && ips.length > 0) {
      const existing = targetInput.value.trim()
      const newIps = ips.join('\n')
      targetInput.value = existing ? existing + '\n' + newIps : newIps
      toast.success(`已导入 ${ips.length} 个设备 IP`)
      showDeviceModal.value = false
    } else {
      toast.warning('所选设备没有有效的 IP 地址')
    }
  } catch (err) {
    toast.error('导入设备 IP 失败')
    console.error('Failed to import devices:', err)
  }
}

// ... 其他方法
</script>

<template>
  <div class="h-full w-full flex flex-col relative bg-transparent">
    <!-- Header -->
    <div class="w-full relative z-10 mb-4 flex items-center justify-between">
      <h1 class="text-xl font-bold text-text-primary flex items-center">
        <span class="mr-2">🏓</span>
        批量 Ping 检测
      </h1>
      <div class="flex gap-2">
        <!-- ✅ 导入设备按钮 -->
        <button
          v-if="!isRunning"
          @click="openDeviceModal"
          class="px-4 py-2 bg-bg-tertiary hover:bg-bg-hover border border-border text-text-primary rounded-lg font-medium transition-all duration-200 flex items-center gap-2"
        >
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197m13.5-9a2.5 2.5 0 11-5 0 2.5 2.5 0 015 0z" />
          </svg>
          导入设备
        </button>
        <!-- 开始/停止按钮 -->
      </div>
    </div>

    <!-- ✅ 设备选择弹窗 -->
    <Teleport to="body">
      <div v-if="showDeviceModal" class="fixed inset-0 z-50 flex items-center justify-center bg-black/50" @click.self="showDeviceModal = false">
        <div class="bg-bg-secondary border border-border rounded-xl shadow-xl w-[600px] max-h-[80vh] flex flex-col">
          <div class="flex items-center justify-between p-4 border-b border-border">
            <h3 class="text-lg font-semibold text-text-primary">选择设备</h3>
            <button @click="showDeviceModal = false" class="text-text-muted hover:text-text-primary">
              <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>
          
          <!-- 搜索框 -->
          <div class="p-4 border-b border-border">
            <input
              v-model="deviceSearchQuery"
              type="text"
              placeholder="搜索设备名称或 IP..."
              class="w-full bg-bg-tertiary/50 border border-border rounded-lg px-3 py-2 text-sm text-text-primary placeholder-text-muted focus:outline-none focus:border-accent"
            />
          </div>
          
          <div class="flex-1 overflow-auto p-4">
            <div v-if="loadingDevices" class="flex items-center justify-center py-8">
              <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-accent"></div>
            </div>
            
            <div v-else-if="devices.length === 0" class="text-center py-8 text-text-muted">
              暂无设备数据
            </div>
            
            <div v-else-if="filteredDevices.length === 0" class="text-center py-8 text-text-muted">
              未找到匹配的设备
            </div>
            
            <div v-else class="space-y-2">
              <div class="flex items-center gap-2 p-2 bg-bg-tertiary/50 rounded-lg text-sm text-text-secondary">
                <input 
                  type="checkbox" 
                  :checked="selectedDeviceIds.length === filteredDevices.length"
                  @change="e => selectedDeviceIds = (e.target as HTMLInputElement).checked ? filteredDevices.map(d => d.id) : []"
                  class="rounded border-border"
                />
                <span>全选</span>
                <span class="ml-auto">已选择 {{ selectedDeviceIds.length }} 个</span>
              </div>
              
              <div v-for="device in filteredDevices" :key="device.id" 
                   class="flex items-center gap-3 p-3 border border-border rounded-lg hover:bg-bg-tertiary/30 transition-colors">
                <input 
                  type="checkbox" 
                  :value="device.id"
                  v-model="selectedDeviceIds"
                  class="rounded border-border"
                />
                <div class="flex-1">
                  <div class="text-text-primary font-medium">{{ device.name }}</div>
                  <div class="text-sm text-text-secondary">{{ device.ip }} · {{ device.vendor }}</div>
                </div>
              </div>
            </div>
          </div>
          
          <div class="flex justify-end gap-2 p-4 border-t border-border">
            <button @click="showDeviceModal = false" class="px-4 py-2 text-text-secondary hover:text-text-primary transition-colors">
              取消
            </button>
            <button 
              @click="importDevices"
              :disabled="selectedDeviceIds.length === 0"
              class="px-4 py-2 bg-accent hover:bg-accent-hover disabled:opacity-50 disabled:cursor-not-allowed text-white rounded-lg transition-colors"
            >
              导入选中设备
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- ... 其他模板内容 ... -->
  </div>
</template>
```

---

### 问题 #13: 前端未处理启动错误提示

#### 问题分析

`startPing` 中 catch 错误只 `console.error`，没有用户提示。

```typescript
// 当前问题代码 (BatchPing.vue:22-34)
const startPing = async () => {
  try {
    const result = await PingService.StartBatchPing(request)
    progress.value = result
  } catch (err) {
    console.error('Failed to start ping:', err)  // ❌ 缺少用户提示
  }
}
```

#### 修复方案

使用 Toast 组件显示错误信息：

```typescript
// BatchPing.vue
import { useToast } from '@/utils/useToast'
const toast = useToast()

const startPing = async () => {
  // 验证输入
  if (!targetInput.value.trim()) {
    toast.error('请输入目标 IP 地址')
    return
  }

  try {
    const request: PingRequest = {
      targets: targetInput.value,
      config: config.value,
      deviceIds: []
    }
    const result = await PingService.StartBatchPing(request)
    progress.value = result
    toast.success('批量 Ping 已启动')
  } catch (err: any) {
    console.error('Failed to start ping:', err)
    // ✅ 显示错误提示
    const errorMsg = err?.message || err?.toString() || '启动失败'
    toast.error(`启动失败: ${errorMsg}`)
  }
}

const stopPing = async () => {
  try {
    await PingService.StopBatchPing()
    toast.info('正在停止...')
  } catch (err: any) {
    console.error('Failed to stop ping:', err)
    toast.error(`停止失败: ${err?.message || '未知错误'}`)
  }
}

const exportCSV = async () => {
  try {
    const result = await PingService.ExportPingResultCSV()
    if (!result || !result.content) {
      toast.warning('没有可导出的数据')
      return
    }
    // ... download logic ...
    toast.success('导出成功')
  } catch (err: any) {
    console.error('Failed to export CSV:', err)
    toast.error(`导出失败: ${err?.message || '未知错误'}`)
  }
}
```

---

### 问题 #14: Events.Off 未传入回调引用

#### 问题分析

`Events.Off('ping:progress')` 没有传入回调引用，无法正确移除监听器。

```typescript
// 当前问题代码 (BatchPing.vue:130-132)
onUnmounted(() => {
  Events.Off('ping:progress')  // ❌ 未传入回调引用
})
```

#### 修复方案

传入正确的回调引用：

```typescript
// BatchPing.vue:115-135
// 定义回调函数引用
const handleProgressEvent = (ev: { name: string; data: BatchPingProgress }) => {
  progress.value = ev.data
}

onMounted(async () => {
  // Get default config
  try {
    const defaultConfig = await PingService.GetPingDefaultConfig()
    if (defaultConfig) {
      config.value = defaultConfig
    }
  } catch (err) {
    console.error('Failed to get default config:', err)
  }

  // Subscribe to events
  Events.On('ping:progress', handleProgressEvent)
})

onUnmounted(() => {
  // ✅ 传入回调引用
  Events.Off('ping:progress', handleProgressEvent)
})
```

---

## 三、测试策略

### 单元测试清单

```go
// ping_service_test.go 新增测试
func TestMergeWithDefaultPingConfig_IntervalLimit(t *testing.T)  // Interval 上限
func TestResolveTargets_CommaSeparated(t *testing.T)              // 逗号分隔
func TestResolveTargets_MixedSeparators(t *testing.T)             // 混合分隔符

// engine_test.go 新增测试（需新建）
func TestBatchPingEngine_ResultOrder(t *testing.T)                // 结果顺序
func TestBatchPingEngine_CallbackPanicRecovery(t *testing.T)      // panic 恢复
func TestPingHost_MinRttWhenAllFail(t *testing.T)                 // 全失败 minRtt
```

### 集成测试清单

1. **并发结果顺序测试**: 同时 ping 多个 IP，验证结果顺序与输入一致
2. **错误提示测试**: 验证各种错误场景下的 Toast 提示
3. **设备导入测试**: 从前端导入设备并执行 ping

---

## 四、修复优先级建议

### 阶段一：数据正确性（最高优先级）
1. **问题 #8**: minRtt 初始值 - 影响统计结果
2. **问题 #9**: 结果顺序 - 影响用户体验

### 阶段二：稳定性与健壮性
3. **问题 #6**: Interval 上限校验 - 防止异常配置
4. **问题 #10**: panic 恢复机制 - 防止程序崩溃

### 阶段三：用户体验
5. **问题 #11**: 逗号分隔支持 - 输入便利性
6. **问题 #13**: 错误提示 - 用户反馈
7. **问题 #14**: Events.Off 修复 - 内存泄漏防护
8. **问题 #12**: 设备导入 UI - 功能完整性

---

## 五、文件修改清单

| 文件 | 修改内容 | 行数范围 |
|------|---------|---------|
| `internal/ui/ping_service.go` | Interval 上限校验 | 410-412 |
| `internal/ui/ping_service.go` | 多分隔符支持 | 225 |
| `internal/icmp/engine.go` | 结果顺序保持 | 70-130 |
| `internal/icmp/engine.go` | panic 恢复机制 | 新增函数 |
| `internal/icmp/engine.go` | minRtt 初始化 | 138 |
| `internal/icmp/types.go` | SetResult 方法 | 89-115 |
| `internal/icmp/types.go` | 预分配切片 | 77 |
| `frontend/src/views/Tools/BatchPing.vue` | 设备导入 UI | 新增 |
| `frontend/src/views/Tools/BatchPing.vue` | Toast 错误提示 | 多处 |
| `frontend/src/views/Tools/BatchPing.vue` | Events.Off 修复 | 131 |
| `frontend/src/views/Tools/BatchPing.vue` | Interval 上限 | 250 |
| `internal/ui/ping_service_test.go` | 新增单元测试 | 新增 |

---

## 六、总结

本修复方案针对批量 Ping 功能的 **8个** 中优先级问题（原 10 个中 #5 和 #7 经验证不存在），提供了详细的代码修复方案和测试策略。所有修复均考虑了：

1. **数据正确性**: 确保 RTT 计算、结果顺序符合预期
2. **健壮性**: 添加边界校验、panic 恢复处理
3. **用户体验**: 完善错误提示、支持多种输入格式、提供设备导入功能

建议按照阶段优先级逐步实施修复，每阶段完成后进行回归测试。
