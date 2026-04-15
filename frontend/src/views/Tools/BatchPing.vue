<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from 'vue'
import { Events } from '@wailsio/runtime'
import * as PingService from '@/bindings/github.com/NetWeaverGo/core/internal/ui/pingservice'
import * as DeviceService from '@/bindings/github.com/NetWeaverGo/core/internal/ui/deviceservice'
import type { PingConfig, BatchPingProgress } from '@/bindings/github.com/NetWeaverGo/core/internal/icmp/models'
import type { PingRequest } from '@/bindings/github.com/NetWeaverGo/core/internal/ui/models'
import type { DeviceAssetListItem } from '@/bindings/github.com/NetWeaverGo/core/internal/models/models'
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

  try {
    const request: PingRequest = {
      targets: targetInput.value,
      config: config.value,
      deviceIds: []
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

// Lifecycle
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

  // Subscribe to events - Events.On 返回取消函数
  unlistenProgress = Events.On('ping:progress', handleProgressEvent)
})

onUnmounted(() => {
  // 移除事件监听器 - 调用取消函数
  if (unlistenProgress) {
    unlistenProgress()
    unlistenProgress = null
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
                  <th class="py-2 px-3 font-medium">#</th>
                  <th class="py-2 px-3 font-medium">IP 地址</th>
                  <th class="py-2 px-3 font-medium">状态</th>
                  <th class="py-2 px-3 font-medium">延迟</th>
                  <th class="py-2 px-3 font-medium">TTL</th>
                  <th class="py-2 px-3 font-medium">丢包率</th>
                  <th class="py-2 px-3 font-medium">错误信息</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="(result, index) in progress.results"
                  :key="result.ip"
                  class="border-b border-border/50 hover:bg-bg-hover/50 transition-colors"
                >
                  <td class="py-2 px-3 text-text-muted">{{ index + 1 }}</td>
                  <td class="py-2 px-3 text-text-primary font-mono">{{ result.ip }}</td>
                  <td class="py-2 px-3">
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
                  <td class="py-2 px-3 text-text-primary">{{ formatRtt(result.avgRtt) }}</td>
                  <td class="py-2 px-3 text-text-primary">{{ result.ttl || '-' }}</td>
                  <td class="py-2 px-3">
                    <span :class="{
                      'text-green-400': result.lossRate === 0,
                      'text-yellow-400': result.lossRate > 0 && result.lossRate < 100,
                      'text-red-400': result.lossRate === 100
                    }">{{ result.lossRate.toFixed(1) }}%</span>
                  </td>
                  <td class="py-2 px-3 text-text-muted text-xs">{{ result.errorMsg || '-' }}</td>
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
