<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from 'vue'
import { Events } from '@wailsio/runtime'
import * as PingService from '@/bindings/github.com/NetWeaverGo/core/internal/ui/pingservice'
import * as DeviceService from '@/bindings/github.com/NetWeaverGo/core/internal/ui/deviceservice'
import type { PingConfig, BatchPingProgress, SinglePingResult, PingHostResult } from '@/bindings/github.com/NetWeaverGo/core/internal/icmp/models'
import type { PingRequest } from '@/bindings/github.com/NetWeaverGo/core/internal/ui/models'
import type { DeviceAssetListItem } from '@/bindings/github.com/NetWeaverGo/core/internal/models/models'
import { Duration } from '@/bindings/time/models'
import { useToast } from '@/utils/useToast'

const toast = useToast()

// 数据包大小限制常量
const MAX_DATA_SIZE = 65500  // Windows API 理论最大值
const RECOMMENDED_MAX_SIZE = 8000  // 推荐最大值（考虑 MTU 和分片）
const MTU_LIMIT = 1472  // 以太网 MTU 边界（不分片最大 ICMP 数据）

// State
const targetInput = ref('')
const config = ref<PingConfig>({
  Timeout: 1000,
  Interval: 0,
  Count: 1,
  DataSize: 32,
  Concurrency: 64
})

// Ping options
const resolveHostName = ref(false)
const enableRealtime = ref(false)
const realtimeThrottle = ref(100) // ms

// 列配置
export interface ColumnConfig {
  key: string
  label: string
  visible: boolean
  width?: number
}

const defaultColumns: ColumnConfig[] = [
  { key: 'index', label: '#', visible: true, width: 50 },
  { key: 'ip', label: 'IP 地址', visible: true, width: 120 },
  { key: 'hostName', label: '主机名', visible: false, width: 150 },
  { key: 'status', label: '状态', visible: true, width: 80 },
  { key: 'successFailed', label: '成功/失败', visible: true, width: 100 },
  { key: 'latency', label: '延迟(Avg/Last)', visible: true, width: 120 },
  { key: 'ttl', label: 'TTL', visible: true, width: 60 },
  { key: 'lossRate', label: '丢包率', visible: true, width: 80 },
  { key: 'lastSucceedAt', label: '最后成功', visible: false, width: 140 },
  { key: 'lastFailedAt', label: '最后失败', visible: false, width: 140 },
  { key: 'errorMsg', label: '错误信息', visible: true, width: 200 },
]

const columns = ref<ColumnConfig[]>([...defaultColumns])
const showColumnConfig = ref(false)

const loadColumnConfig = () => {
  const saved = localStorage.getItem('pingColumns')
  if (saved) {
    try {
      const parsed = JSON.parse(saved)
      columns.value = defaultColumns.map((col: ColumnConfig) => {
        const savedCol = parsed.find((c: ColumnConfig) => c.key === col.key)
        return savedCol ? { ...col, visible: savedCol.visible } : col
      })
    } catch { /* ignored */ }
  }
}

const saveColumnConfig = () => {
  localStorage.setItem('pingColumns', JSON.stringify(columns.value))
  showColumnConfig.value = false
  toast.success('列配置已保存')
}

const resetColumnConfig = () => {
  columns.value = defaultColumns.map(col => ({ ...col }))
  localStorage.removeItem('pingColumns')
}

const isColumnVisible = (key: string): boolean => {
  return columns.value.find(c => c.key === key)?.visible ?? false
}

// 数据包大小警告状态
const dataSizeWarning = computed(() => {
  const size = config.value.DataSize
  if (size > RECOMMENDED_MAX_SIZE) {
    return {
      type: 'error',
      message: `数据包大小超过推荐值 ${RECOMMENDED_MAX_SIZE} 字节，可能因 MTU 限制或系统资源不足而失败`
    }
  } else if (size > MTU_LIMIT) {
    return {
      type: 'warning',
      message: `数据包大小超过 ${MTU_LIMIT} 字节，需要 IP 分片，可能在某些网络环境下失败`
    }
  }
  return null
})

const progress = ref<BatchPingProgress | null>(null)
const isRunning = computed(() => progress.value?.isRunning ?? false)

// 轮询相关
const POLLING_INTERVAL = 2000 // 2秒轮询间隔
let pollingTimer: ReturnType<typeof setInterval> | null = null
let lastProgressTime = 0 // 上次收到进度的时间戳
let unlistenProgress: (() => void) | null = null // 取消事件监听的函数

// 设备导入相关状态
const showDeviceModal = ref(false)
const devices = ref<DeviceAssetListItem[]>([])
const selectedDeviceIds = ref<number[]>([])
const loadingDevices = ref(false)
const deviceSearchQuery = ref('')

// 过滤后的设备列表
const filteredDevices = computed(() => {
  if (!deviceSearchQuery.value) return devices.value
  const query = deviceSearchQuery.value.toLowerCase()
  return devices.value.filter((d: DeviceAssetListItem) =>
    d.displayName.toLowerCase().includes(query) ||
    d.ip.toLowerCase().includes(query)
  )
})

// 加载设备列表
const loadDevices = async () => {
  loadingDevices.value = true
  try {
    const result = await DeviceService.ListDevices()
    devices.value = result || []
  } catch (err) {
    toast.error('加载设备列表失败')
    console.error('Failed to load devices:', err)
  } finally {
    loadingDevices.value = false
  }
}

// 打开设备选择弹窗
const openDeviceModal = async () => {
  await loadDevices()
  selectedDeviceIds.value = []
  deviceSearchQuery.value = ''
  showDeviceModal.value = true
}

// 确认导入设备
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

// 启动轮询兜底
const startPolling = () => {
  if (pollingTimer) return

  pollingTimer = setInterval(async () => {
    // 只在运行中时轮询
    if (!isRunning.value) {
      stopPolling()
      return
    }

    try {
      const currentProgress = await PingService.GetPingProgress()
      if (currentProgress) {
        // 检查是否需要更新（Event 可能已更新）
        const now = Date.now()
        // 如果超过 3 秒没收到 Event，使用轮询数据
        if (now - lastProgressTime > 3000) {
          progress.value = currentProgress
        }
      }
    } catch (err) {
      console.error('Polling progress failed:', err)
    }
  }, POLLING_INTERVAL)
}

// 停止轮询
const stopPolling = () => {
  if (pollingTimer) {
    clearInterval(pollingTimer)
    pollingTimer = null
  }
}

// Methods
const startPing = async () => {
  // 验证输入
  if (!targetInput.value.trim()) {
    toast.error('请输入目标 IP 地址')
    return
  }

  // 验证数据包大小
  if (config.value.DataSize > MAX_DATA_SIZE) {
    toast.error(`数据包大小超过 Windows API 限制 (最大 ${MAX_DATA_SIZE} 字节)`)
    return
  }

  // 大数据包警告提示
  if (config.value.DataSize > RECOMMENDED_MAX_SIZE) {
    toast.warning(`数据包大小 ${config.value.DataSize} 字节超过推荐值，可能因 MTU 限制或系统资源不足而失败`)
  } else if (config.value.DataSize > MTU_LIMIT) {
    toast.warning(`数据包大小 ${config.value.DataSize} 字节需要 IP 分片，某些网络环境可能失败`)
  }

  try {
    const request: PingRequest = {
      targets: targetInput.value,
      config: config.value,
      deviceIds: [],
      options: {
        resolveHostName: resolveHostName.value,
        dnsTimeout: 2 * Duration.Second, // 2 seconds
        enableRealtime: enableRealtime.value,
        realtimeThrottle: realtimeThrottle.value * Duration.Millisecond
      }
    }
    const result = await PingService.StartBatchPing(request)
    progress.value = result
    lastProgressTime = Date.now()

    // 启动轮询兜底
    startPolling()

    toast.success('批量 Ping 已启动')
  } catch (err: any) {
    console.error('Failed to start ping:', err)
    const errorMsg = err?.message || err?.toString() || '启动失败'
    toast.error(`启动失败: ${errorMsg}`)
  }
}

const stopPing = async () => {
  try {
    await PingService.StopBatchPing()
    stopPolling()
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

    const blob = new Blob([result.content], { type: 'text/csv;charset=utf-8;' })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.setAttribute('href', url)
    link.setAttribute('download', result.fileName || 'ping_result.csv')
    link.style.visibility = 'hidden'
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    URL.revokeObjectURL(url)
    toast.success('导出成功')
  } catch (err: any) {
    console.error('Failed to export CSV:', err)
    toast.error(`导出失败: ${err?.message || '未知错误'}`)
  }
}

const clearResults = () => {
  progress.value = null
}

const formatRtt = (rtt: number, status?: string): string => {
  // 离线或错误状态，或 rtt 为 0 或负数时显示 "-"
  if (status !== 'online' || rtt <= 0) return '-'
  // 支持亚毫秒精度显示
  if (rtt < 1) {
    return `${rtt.toFixed(3)}ms`
  }
  return `${rtt.toFixed(2)}ms`
}

const formatTime = (timestamp: number | undefined): string => {
  if (!timestamp || timestamp === 0) return '-'
  
  const date = new Date(timestamp)
  if (isNaN(date.getTime())) return '-'
  
  const now = new Date()
  const diffMs = now.getTime() - date.getTime()
  
  // 处理未来时间或时钟偏差，直接显示绝对时间
  if (diffMs < 0) {
    return date.toLocaleString('zh-CN', {
      month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit'
    })
  }

  const diffSec = Math.floor(diffMs / 1000)
  
  if (diffSec < 60) return '刚刚'
  if (diffSec < 3600) return `${Math.floor(diffSec / 60)} 分钟前`
  if (diffSec < 86400) return `${Math.floor(diffSec / 3600)} 小时前`
  
  // 超过 24 小时显示具体时间
  return date.toLocaleString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit'
  })
}

const formatElapsed = (ms: number): string => {
  if (ms < 1000) return `${ms}ms`
  const seconds = Math.floor(ms / 1000)
  const minutes = Math.floor(seconds / 60)
  if (minutes > 0) {
    return `${minutes}m ${seconds % 60}s`
  }
  return `${seconds}s`
}

const getStatusIcon = (status: string): string => {
  switch (status) {
    case 'online':
      return '🟢'
    case 'offline':
      return '🔴'
    case 'error':
      return '⚠️'
    default:
      return '⏳'
  }
}

const getStatusText = (status: string): string => {
  switch (status) {
    case 'online':
      return '在线'
    case 'offline':
      return '离线'
    case 'error':
      return '错误'
    default:
      return '等待中'
  }
}

// Event handling
const handleProgressEvent = (ev: { name: string; data: BatchPingProgress }) => {
  progress.value = ev.data
  lastProgressTime = Date.now() // 记录 Event 收到时间
}

let unlistenRealtime: (() => void) | null = null

const handleRealtimeEvent = (ev: { name: string; data: SinglePingResult }) => {
  const result = ev.data
  if (progress.value) {
    // 根据单条请求去反查本地表并呈现某些微动画，或简单替换
    const exist = progress.value.results?.find((r: PingHostResult) => r.ip === result.ip)
    if (exist) {
        // 这里仅为了展示可达性，更全面的更新应依赖 progress 推进
        // 如果你需要细粒度可独立更新 UI，那可以在此处将单项数据挂载在 vue ref 上
    }
  }
}

// Lifecycle
onMounted(async () => {
  loadColumnConfig()

  // Get default config
  try {
    const defaultConfig = await PingService.GetPingDefaultConfig()
    if (defaultConfig) {
      config.value = defaultConfig
    }
  } catch (err) {
    console.error('Failed to get default config:', err)
  }

  // Subscribe to events - Events.On 返回取消函数
  unlistenProgress = Events.On('ping:progress', handleProgressEvent)
  unlistenRealtime = Events.On('ping:realtime', handleRealtimeEvent)
})

onUnmounted(() => {
  // 移除事件监听器 - 调用取消函数
  if (unlistenProgress) {
    unlistenProgress()
    unlistenProgress = null
  }
  if (unlistenRealtime) {
    unlistenRealtime()
    unlistenRealtime = null
  }
  // 停止轮询
  stopPolling()
})
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
        <!-- 导入设备按钮 -->
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
        <button
          v-if="!isRunning"
          @click="startPing"
          class="px-4 py-2 bg-accent hover:bg-accent-hover text-white rounded-lg font-medium transition-all duration-200 flex items-center gap-2"
        >
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z" />
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
          开始
        </button>
        <button
          v-else
          @click="stopPing"
          class="px-4 py-2 bg-red-500 hover:bg-red-600 text-white rounded-lg font-medium transition-all duration-200 flex items-center gap-2"
        >
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 10a1 1 0 011-1h4a1 1 0 011 1v4a1 1 0 01-1 1h-4a1 1 0 01-1-1v-4z" />
          </svg>
          停止
        </button>
      </div>
    </div>

    <!-- Main Content -->
    <div class="flex-1 flex gap-4 overflow-hidden">
      <!-- Left Panel: Input -->
      <div class="w-80 flex flex-col gap-4">
        <!-- Target Input -->
        <section class="bg-bg-secondary/60 glass border border-border rounded-xl shadow-card p-4">
          <h2 class="text-sm font-semibold text-text-primary mb-3 flex items-center">
            <svg class="w-5 h-5 mr-2 text-accent" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2" />
            </svg>
            目标输入
          </h2>
          <textarea
            v-model="targetInput"
            :disabled="isRunning"
            placeholder="输入 IP 地址&#10;支持格式：&#10;• 单个 IP: 192.168.1.1&#10;• CIDR: 192.168.1.0/24&#10;• 范围: 192.168.1.1-100&#10;• 多个 IP: 192.168.1.1, 192.168.1.2&#10;• 混合: 192.168.1.1, 192.168.1.0/30"
            class="w-full h-40 bg-bg-tertiary/50 border border-border rounded-lg p-3 text-sm text-text-primary placeholder-text-muted resize-none focus:outline-none focus:border-accent transition-colors"
          ></textarea>
        </section>

        <!-- Config Panel -->
        <section class="bg-bg-secondary/60 glass border border-border rounded-xl shadow-card p-4">
          <h2 class="text-sm font-semibold text-text-primary mb-3 flex items-center">
            <svg class="w-5 h-5 mr-2 text-accent" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
            </svg>
            配置参数
          </h2>
          <div class="space-y-3">
            <div class="flex items-center justify-between">
              <label class="text-sm text-text-secondary">超时 (ms)</label>
              <input
                v-model.number="config.Timeout"
                type="number"
                :disabled="isRunning"
                min="100"
                max="30000"
                class="w-24 bg-bg-tertiary/50 border border-border rounded px-2 py-1 text-sm text-text-primary text-right focus:outline-none focus:border-accent"
              />
            </div>
            <div class="flex items-center justify-between">
              <label class="text-sm text-text-secondary">重试次数</label>
              <input
                v-model.number="config.Count"
                type="number"
                :disabled="isRunning"
                min="1"
                max="10"
                class="w-24 bg-bg-tertiary/50 border border-border rounded px-2 py-1 text-sm text-text-primary text-right focus:outline-none focus:border-accent"
              />
            </div>
            <div class="flex items-center justify-between">
              <label class="text-sm text-text-secondary">并发数</label>
              <input
                v-model.number="config.Concurrency"
                type="number"
                :disabled="isRunning"
                min="1"
                max="256"
                class="w-24 bg-bg-tertiary/50 border border-border rounded px-2 py-1 text-sm text-text-primary text-right focus:outline-none focus:border-accent"
              />
            </div>
            <div class="flex items-center justify-between">
              <label class="text-sm text-text-secondary">包大小 (bytes)</label>
              <input
                v-model.number="config.DataSize"
                type="number"
                :disabled="isRunning"
                min="32"
                max="65500"
                class="w-24 bg-bg-tertiary/50 border border-border rounded px-2 py-1 text-sm text-text-primary text-right focus:outline-none focus:border-accent"
              />
            </div>
            <!-- 数据包大小警告 -->
            <div v-if="dataSizeWarning" class="mt-2 p-2 rounded-lg text-xs"
                 :class="dataSizeWarning.type === 'error' ? 'bg-red-500/20 text-red-400 border border-red-500/30' : 'bg-yellow-500/20 text-yellow-400 border border-yellow-500/30'">
              <div class="flex items-start gap-2">
                <svg class="w-4 h-4 flex-shrink-0 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                </svg>
                <span>{{ dataSizeWarning.message }}</span>
              </div>
            </div>
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
            <!-- 解析主机名选项 -->
            <div class="flex items-center justify-between pt-2 border-t border-border/50">
              <label class="text-sm text-text-secondary flex items-center gap-2">
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 12a9 9 0 01-9 9m9-9a9 9 0 00-9-9m9 9H3m9 9a9 9 0 01-9-9m9 9c1.657 0 3-4.03 3-9s-1.343-9-3-9m0 18c-1.657 0-3-4.03-3-9s1.343-9 3-9m-9 9a9 9 0 019-9" />
                </svg>
                解析主机名
              </label>
              <button
                @click="resolveHostName = !resolveHostName"
                :disabled="isRunning"
                class="relative w-10 h-5 rounded-full transition-colors"
                :class="resolveHostName ? 'bg-accent' : 'bg-bg-tertiary'"
              >
                <span
                  class="absolute top-0.5 left-0.5 w-4 h-4 bg-white rounded-full transition-transform"
                  :class="resolveHostName ? 'translate-x-5' : ''"
                ></span>
              </button>
            </div>
            <!-- 实时进度选项 -->
            <div class="flex items-center justify-between pt-2 border-t border-border/50">
              <label class="text-sm text-text-secondary flex items-center gap-2">
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
                启用实时进度
              </label>
              <button
                @click="enableRealtime = !enableRealtime"
                :disabled="isRunning"
                class="relative w-10 h-5 rounded-full transition-colors"
                :class="enableRealtime ? 'bg-accent' : 'bg-bg-tertiary'"
              >
                <span
                  class="absolute top-0.5 left-0.5 w-4 h-4 bg-white rounded-full transition-transform"
                  :class="enableRealtime ? 'translate-x-5' : ''"
                ></span>
              </button>
            </div>
            <div v-if="enableRealtime" class="flex items-center justify-between">
              <label class="text-sm text-text-secondary">更新间隔(ms)</label>
              <input
                v-model.number="realtimeThrottle"
                type="number"
                :disabled="isRunning"
                min="10"
                max="5000"
                class="w-24 bg-bg-tertiary/50 border border-border rounded px-2 py-1 text-sm text-text-primary text-right focus:outline-none focus:border-accent"
              />
            </div>
          </div>
        </section>
      </div>

      <!-- Right Panel: Results -->
      <div class="flex-1 flex flex-col gap-4 overflow-hidden">
        <!-- Progress Bar -->
        <section v-if="progress" class="bg-bg-secondary/60 glass border border-border rounded-xl shadow-card p-4">
          <div class="flex items-center justify-between mb-2">
            <div class="flex items-center gap-4">
              <span class="text-sm text-text-secondary">
                进度: {{ progress.completedIPs }} / {{ progress.totalIPs }}
              </span>
              <span class="text-sm text-text-secondary">
                耗时: {{ formatElapsed(progress.elapsedMs) }}
              </span>
            </div>
            <span class="text-sm font-medium text-accent">
              {{ progress.progress.toFixed(1) }}%
            </span>
          </div>
          <div class="w-full h-2 bg-bg-tertiary rounded-full overflow-hidden">
            <div
              class="h-full bg-accent transition-all duration-300"
              :style="{ width: `${progress.progress}%` }"
            ></div>
          </div>
          <div class="flex items-center gap-6 mt-3">
            <span class="text-sm flex items-center gap-1">
              <span>🟢</span>
              <span class="text-text-secondary">在线:</span>
              <span class="text-green-400 font-medium">{{ progress.onlineCount }}</span>
            </span>
            <span class="text-sm flex items-center gap-1">
              <span>🔴</span>
              <span class="text-text-secondary">离线:</span>
              <span class="text-red-400 font-medium">{{ progress.offlineCount }}</span>
            </span>
            <span class="text-sm flex items-center gap-1">
              <span>⚠️</span>
              <span class="text-text-secondary">错误:</span>
              <span class="text-yellow-400 font-medium">{{ progress.errorCount }}</span>
            </span>
          </div>
        </section>

        <!-- Results Table -->
        <section class="flex-1 bg-bg-secondary/60 glass border border-border rounded-xl shadow-card p-4 overflow-hidden flex flex-col">
          <div class="flex items-center justify-between mb-3">
            <h2 class="text-sm font-semibold text-text-primary flex items-center">
              <svg class="w-5 h-5 mr-2 text-accent" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 17v-2m3 2v-4m3 4v-6m2 10H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
              </svg>
              检测结果
            </h2>
            <div class="flex gap-2">
              <button
                @click="showColumnConfig = true"
                :disabled="isRunning"
                class="px-3 py-1.5 bg-bg-tertiary hover:bg-bg-hover border border-border rounded-lg text-sm text-text-primary transition-colors disabled:opacity-50"
              >
                列配置
              </button>
              <button
                @click="exportCSV"
                :disabled="!progress || progress.results.length === 0"
                class="px-3 py-1.5 bg-bg-tertiary hover:bg-bg-hover border border-border rounded-lg text-sm text-text-primary transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-1"
              >
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
                </svg>
                导出 CSV
              </button>
              <button
                @click="clearResults"
                :disabled="isRunning"
                class="px-3 py-1.5 bg-bg-tertiary hover:bg-bg-hover border border-border rounded-lg text-sm text-text-primary transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
              >
                清空
              </button>
            </div>
          </div>

          <div class="flex-1 overflow-auto">
            <table v-if="progress && progress.results.length > 0" class="w-full text-sm">
              <thead class="sticky top-0 bg-bg-secondary">
                <tr class="text-left text-text-muted border-b border-border">
                  <th v-if="isColumnVisible('index')" class="py-2 px-3 font-medium">#</th>
                  <th v-if="isColumnVisible('ip')" class="py-2 px-3 font-medium">IP 地址</th>
                  <th v-if="isColumnVisible('hostName') && resolveHostName" class="py-2 px-3 font-medium">主机名</th>
                  <th v-if="isColumnVisible('status')" class="py-2 px-3 font-medium">状态</th>
                  <th v-if="isColumnVisible('successFailed')" class="py-2 px-3 font-medium">成功/失败</th>
                  <th v-if="isColumnVisible('latency')" class="py-2 px-3 font-medium">延迟(Avg/Last)</th>
                  <th v-if="isColumnVisible('ttl')" class="py-2 px-3 font-medium">TTL</th>
                  <th v-if="isColumnVisible('lossRate')" class="py-2 px-3 font-medium">丢包率</th>
                  <th v-if="isColumnVisible('lastSucceedAt')" class="py-2 px-3 font-medium">最后成功</th>
                  <th v-if="isColumnVisible('lastFailedAt')" class="py-2 px-3 font-medium">最后失败</th>
                  <th v-if="isColumnVisible('errorMsg')" class="py-2 px-3 font-medium">错误信息</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="(result, index) in progress.results"
                  :key="result.ip"
                  class="border-b border-border/50 hover:bg-bg-hover/50 transition-colors"
                >
                  <td v-if="isColumnVisible('index')" class="py-2 px-3 text-text-muted">{{ index + 1 }}</td>
                  <td v-if="isColumnVisible('ip')" class="py-2 px-3 text-text-primary font-mono">{{ result.ip }}</td>
                  <td v-if="isColumnVisible('hostName') && resolveHostName" class="py-2 px-3 text-text-secondary text-xs">{{ result.hostName || '-' }}</td>
                  <td v-if="isColumnVisible('status')" class="py-2 px-3">
                    <span class="flex items-center gap-1">
                      <span>{{ getStatusIcon(result.status) }}</span>
                      <span :class="{
                        'text-green-400': result.status === 'online',
                        'text-red-400': result.status === 'offline',
                        'text-yellow-400': result.status === 'error',
                        'text-text-muted': result.status === 'pending'
                      }">{{ getStatusText(result.status) }}</span>
                    </span>
                  </td>
                  <td v-if="isColumnVisible('successFailed')" class="py-2 px-3 text-text-primary">
                    <span class="text-green-400">{{ result.recvCount }}</span>
                    <span class="text-text-muted">/</span>
                    <span class="text-red-400">{{ result.failedCount }}</span>
                  </td>
                  <td v-if="isColumnVisible('latency')" class="py-2 px-3 text-text-primary">
                    <span>{{ formatRtt(result.avgRtt) }}</span>
                    <span v-if="result.lastRtt && result.lastRtt > 0" class="text-text-muted text-xs">({{ formatRtt(result.lastRtt!) }})</span>
                  </td>
                  <td v-if="isColumnVisible('ttl')" class="py-2 px-3 text-text-primary">{{ result.ttl || '-' }}</td>
                  <td v-if="isColumnVisible('lossRate')" class="py-2 px-3">
                    <span :class="{
                      'text-green-400': result.lossRate === 0,
                      'text-yellow-400': result.lossRate > 0 && result.lossRate < 100,
                      'text-red-400': result.lossRate === 100
                    }">{{ result.lossRate.toFixed(1) }}%</span>
                  </td>
                  <td v-if="isColumnVisible('lastSucceedAt')" class="py-2 px-3 text-text-secondary text-xs">
                    {{ formatTime(result.lastSucceedAt) }}
                  </td>
                  <td v-if="isColumnVisible('lastFailedAt')" class="py-2 px-3 text-text-secondary text-xs">
                    {{ formatTime(result.lastFailedAt) }}
                  </td>
                  <td v-if="isColumnVisible('errorMsg')" class="py-2 px-3 text-text-muted text-xs">{{ result.errorMsg || '-' }}</td>
                </tr>
              </tbody>
            </table>
            <div v-else class="h-full flex items-center justify-center text-text-muted">
              <div class="text-center">
                <svg class="w-16 h-16 mx-auto mb-4 opacity-30" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1" d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
                </svg>
                <p>输入目标 IP 后点击"开始"进行检测</p>
              </div>
            </div>
          </div>
        </section>
      </div>
    </div>

    <!-- 列配置弹窗 -->
    <Teleport to="body">
      <div v-if="showColumnConfig" class="fixed inset-0 z-[60] flex items-center justify-center bg-black/50" @click.self="showColumnConfig = false">
        <div class="bg-bg-secondary border border-border rounded-xl shadow-xl w-[400px]">
          <div class="flex items-center justify-between p-4 border-b border-border">
            <h3 class="text-lg font-semibold text-text-primary">列配置</h3>
            <button @click="showColumnConfig = false" class="text-text-muted hover:text-text-primary">
              <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" /></svg>
            </button>
          </div>
          <div class="p-4 space-y-2 max-h-[60vh] overflow-auto">
            <div v-for="col in columns" :key="col.key" class="flex items-center justify-between p-2 bg-bg-tertiary/30 rounded-lg">
              <span class="text-sm text-text-primary">{{ col.label }}</span>
              <button @click="col.visible = !col.visible" class="relative w-10 h-5 rounded-full transition-colors" :class="col.visible ? 'bg-accent' : 'bg-bg-tertiary'">
                <span class="absolute top-0.5 left-0.5 w-4 h-4 bg-white rounded-full transition-transform" :class="col.visible ? 'translate-x-5' : ''"></span>
              </button>
            </div>
          </div>
          <div class="flex justify-end gap-2 p-4 border-t border-border">
            <button @click="resetColumnConfig" class="px-4 py-2 text-text-secondary hover:text-text-primary transition-colors">重置</button>
            <button @click="saveColumnConfig" class="px-4 py-2 bg-accent hover:bg-accent-hover text-white rounded-lg transition-colors">保存</button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- 设备选择弹窗 -->
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
                  @change="(e: Event) => selectedDeviceIds = (e.target as HTMLInputElement).checked ? filteredDevices.map((d: DeviceAssetListItem) => d.id) : []"
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
                  <div class="text-text-primary font-medium">{{ device.displayName }}</div>
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
  </div>
</template>

<style scoped>
.glass {
  backdrop-filter: blur(10px);
}

.shadow-card {
  box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.1), 0 2px 4px -1px rgba(0, 0, 0, 0.06);
}

.shadow-glow {
  box-shadow: 0 0 20px rgba(59, 130, 246, 0.3);
}
</style>
