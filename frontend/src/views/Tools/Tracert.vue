<script setup lang="ts">
import { ref, shallowRef, onMounted, onUnmounted, computed, triggerRef } from 'vue'
import { Events } from '@wailsio/runtime'
import * as TracertService from '@/bindings/github.com/NetWeaverGo/core/internal/ui/tracertservice'
import type { TracertConfig, TracertProgress, TracertHopUpdate, TracertHopResult } from '@/bindings/github.com/NetWeaverGo/core/internal/icmp/models'
import type { TracertRequest } from '@/bindings/github.com/NetWeaverGo/core/internal/ui/models'
import { useToast } from '@/utils/useToast'
import { getLogger } from '@/utils/logger'
import TracertSettingsModal from '@/components/tools/TracertSettingsModal.vue'

const logger = getLogger()

const toast = useToast()

// State
const target = ref('')
const config = ref<TracertConfig>({
  maxHops: 30,
  timeout: 3000,
  dataSize: 32,
  count: 3,
  interval: 1000,
  concurrency: 0
})
const continuous = ref(true)

// Column config
export interface ColumnConfig {
  key: string
  label: string
  visible: boolean
  width?: number
}

const defaultColumns: ColumnConfig[] = [
  { key: 'index', label: '#', visible: true, width: 50 },
  { key: 'hostName', label: '主机名', visible: true, width: 180 },
  { key: 'ip', label: '响应 IP', visible: true, width: 140 },
  { key: 'ttl', label: '第几跳', visible: true, width: 60 },
  { key: 'lossRate', label: '丢包率', visible: true, width: 80 },
  { key: 'sentCount', label: '发送报文', visible: true, width: 80 },
  { key: 'recvCount', label: '接收报文', visible: true, width: 80 },
  { key: 'minRtt', label: '最低延迟', visible: true, width: 90 },
  { key: 'avgRtt', label: '平均延迟', visible: true, width: 90 },
  { key: 'maxRtt', label: '最高延迟', visible: true, width: 90 },
  { key: 'lastRtt', label: '上次延迟', visible: true, width: 90 },
  { key: 'status', label: '状态', visible: true, width: 80 },
  { key: 'errorMsg', label: '错误信息', visible: false, width: 200 },
]

const columns = ref<ColumnConfig[]>([...defaultColumns])
const showColumnConfig = ref(false)
const showSettingsModal = ref(false)

const loadColumnConfig = () => {
  const saved = localStorage.getItem('tracertColumns')
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
  localStorage.setItem('tracertColumns', JSON.stringify(columns.value))
  showColumnConfig.value = false
  toast.success('列配置已保存')
}

const resetColumnConfig = () => {
  columns.value = defaultColumns.map(col => ({ ...col }))
  localStorage.removeItem('tracertColumns')
}

const isColumnVisible = (key: string): boolean => {
  return columns.value.find(c => c.key === key)?.visible ?? false
}

// 使用 shallowRef 替代 ref，避免深层响应式带来的性能开销
// 配合 triggerRef 手动触发更新，实现精确的响应式控制
const progress = shallowRef<TracertProgress | null>(null)
const isRunning = computed(() => progress.value?.isRunning ?? false)

// Hop update overlay for real-time tracking
const hopOverlay = ref<Map<number, { lastUpdateTimestamp: number; status: 'probing' | 'completed' }>>(new Map())

// Display results with overlay
// 过滤 pending 状态，并过滤 TTL > MinReachedTTL 的结果作为最终防线
const displayResults = computed(() => {
  if (!progress.value?.hops) return []
  const minTTL = progress.value.minReachedTtl
  return progress.value.hops
    .filter((hop) => hop.status !== 'pending')
    .filter((hop) => !minTTL || minTTL <= 0 || hop.ttl <= minTTL)
    .map((hop) => {
      const overlay = hopOverlay.value.get(hop.ttl)
      const isProbing = overlay && overlay.status === 'probing'
      return { ...hop, isProbing }
    })
})

// Realtime stats
const realtimeStats = computed(() => {
  if (!progress.value) return null
  const hops = progress.value.hops.filter(h => h.status !== 'pending')
  const totalSent = hops.reduce((sum, h) => sum + (h.sentCount || 0), 0)
  const totalRecv = hops.reduce((sum, h) => sum + (h.recvCount || 0), 0)
  const hopsWithRtt = hops.filter(h => h.avgRtt > 0)
  const avgRtt = hopsWithRtt.length > 0
    ? hopsWithRtt.reduce((sum, h) => sum + h.avgRtt, 0) / hopsWithRtt.length
    : 0
  return {
    totalHops: hops.length,
    totalSent,
    totalRecv,
    avgRtt,
    reached: progress.value.reachedDest,
    round: progress.value.round
  }
})

// Event handling
const POLLING_INTERVAL = 2000
let pollingTimer: ReturnType<typeof setInterval> | null = null
let lastProgressTime = 0
let unlistenProgress: (() => void) | null = null
let unlistenHopUpdate: (() => void) | null = null

// Hop update batch processing
const pendingHopUpdates = new Map<number, TracertHopUpdate>()
let flushScheduled = false

const scheduleFlush = () => {
  if (flushScheduled) return
  flushScheduled = true
  // 使用 queueMicrotask 替代 requestAnimationFrame
  // 微任务优先级更高，不会被渲染阻塞，确保每个 TTL 探测结果实时更新
  queueMicrotask(() => {
    pendingHopUpdates.forEach((update) => {
      applyHopUpdate(update)
    })
    pendingHopUpdates.clear()
    flushScheduled = false
  })
}

const applyHopUpdate = (update: TracertHopUpdate) => {
  if (!progress.value?.hops) return

  // 守卫：当已探测到目标时，忽略 TTL > MinReachedTTL 的更新，防止超范围数据泄漏到前端
  const minTTL = progress.value.minReachedTtl
  if (minTTL > 0 && update.ttl > minTTL) return

  // 动态扩展 hops 数组
  while (progress.value.hops.length < update.ttl) {
    progress.value.hops.push({
      ttl: progress.value.hops.length + 1,
      ip: '*',
      hostName: '-',
      status: 'pending',
      sentCount: 0,
      recvCount: 0,
      lossRate: 0,
      minRtt: -1,
      maxRtt: -1,
      avgRtt: -1,
      lastRtt: -1,
      reached: false,
      errorMsg: ''
    })
  }

  const hop = progress.value.hops[update.ttl - 1]
  if (!hop) return

  // 防乱序检查
  const overlay = hopOverlay.value.get(update.ttl)
  if (overlay && update.timestamp < overlay.lastUpdateTimestamp) {
    return
  }

  // 更新对应跳的状态
  if (update.success && update.rtt > 0) {
    hop.recvCount++
    hop.status = 'success'
    if (hop.minRtt < 0 || update.rtt < hop.minRtt) hop.minRtt = update.rtt
    if (hop.maxRtt < 0 || update.rtt > hop.maxRtt) hop.maxRtt = update.rtt
    hop.lastRtt = update.rtt
  } else if (!update.success) {
    hop.status = 'timeout'
  }
  hop.ip = update.ip || hop.ip

  // 更新 overlay 状态
  hopOverlay.value.set(update.ttl, {
    status: update.isComplete ? 'completed' : 'probing',
    lastUpdateTimestamp: update.timestamp
  })

  // 使用 triggerRef 手动触发 shallowRef 的响应式更新
  // 相比展开对象 { ...progress.value }，这种方式更高效且语义明确
  triggerRef(progress)
}

const handleProgressEvent = (ev: { name: string; data: TracertProgress }) => {
  lastProgressTime = Date.now()

  const incoming = ev.data

  if (!progress.value) {
    progress.value = ev.data
    return
  }

  const current = progress.value

  // 检测路径变化
  const oldMinReachedTTL = current.minReachedTtl
  const newMinReachedTTL = incoming.minReachedTtl

  if (newMinReachedTTL > oldMinReachedTTL && oldMinReachedTTL > 0) {
    logger.info(`路径变长: ${oldMinReachedTTL} → ${newMinReachedTTL} 跳`, 'Tracert')
  }

  // Update top-level fields
  current.target = incoming.target
  current.resolvedIP = incoming.resolvedIP
  current.round = incoming.round
  current.totalHops = incoming.totalHops
  current.completedHops = incoming.completedHops
  current.isRunning = incoming.isRunning
  current.isContinuous = incoming.isContinuous
  current.elapsedMs = incoming.elapsedMs
  current.reachedDest = incoming.reachedDest
  current.minReachedTtl = incoming.minReachedTtl

  // 如果后端发送的 hops 数组比当前短，截断当前数组
  // 这确保 TTL > MinReachedTTL 的结果被删除
  if (incoming.hops && current.hops.length > incoming.hops.length) {
    current.hops = current.hops.slice(0, incoming.hops.length)
    // 清理超出范围的 overlay 状态
    for (const ttl of hopOverlay.value.keys()) {
      if (ttl > incoming.hops.length) {
        hopOverlay.value.delete(ttl)
      }
    }
  }

  // Merge hops
  if (incoming.hops) {
    for (let i = 0; i < incoming.hops.length; i++) {
      const incomingHop = incoming.hops[i]
      if (!incomingHop) continue
      if (incomingHop.status === 'pending') continue

      // Check overlay - don't overwrite real-time data with stale backend data
      const overlay = hopOverlay.value.get(incomingHop.ttl)
      if (overlay && overlay.status === 'probing') {
        // Only update non-RTT fields
        const currentHop = current.hops[i]
        if (currentHop) {
          currentHop.ip = incomingHop.ip || currentHop.ip
          currentHop.hostName = incomingHop.hostName || currentHop.hostName
          currentHop.reached = incomingHop.reached || currentHop.reached
        }
        continue
      }

      current.hops[i] = incomingHop
      hopOverlay.value.set(incomingHop.ttl, {
        lastUpdateTimestamp: Date.now(),
        status: 'completed',
      })
    }
  }

  // 使用 triggerRef 手动触发 shallowRef 的响应式更新
  triggerRef(progress)
}

const handleHopUpdateEvent = (ev: { name: string; data: TracertHopUpdate }) => {
  lastProgressTime = Date.now()
  const update = ev.data
  pendingHopUpdates.set(update.ttl, update)
  scheduleFlush()
}

// Polling fallback
const startPolling = () => {
  if (pollingTimer) return
  pollingTimer = setInterval(async () => {
    if (!isRunning.value) {
      stopPolling()
      return
    }
    try {
      const currentProgress = await TracertService.GetTracertProgress()
      if (currentProgress) {
        const now = Date.now()
        if (now - lastProgressTime > 3000) {
          progress.value = currentProgress
        }
      }
    } catch (err) {
      logger.error('Polling progress failed', 'Tracert', err)
    }
  }, POLLING_INTERVAL)
}

const stopPolling = () => {
  if (pollingTimer) {
    clearInterval(pollingTimer)
    pollingTimer = null
  }
}

// Methods
const startTracert = async () => {
  if (!target.value.trim()) {
    toast.error('请输入目标地址')
    return
  }

  hopOverlay.value.clear()

  try {
    const request: TracertRequest = {
      target: target.value,
      config: config.value,
      continuous: continuous.value
    }
    const result = await TracertService.StartTracert(request)
    progress.value = result
    lastProgressTime = Date.now()
    startPolling()
    toast.success('路径探测已启动')
  } catch (err: any) {
    logger.error('Failed to start tracert', 'Tracert', err)
    const errorMsg = err?.message || err?.toString() || '启动失败'
    toast.error(`启动失败: ${errorMsg}`)
  }
}

const stopTracert = async () => {
  try {
    await TracertService.StopTracert()
    stopPolling()
    toast.info('正在停止...')
  } catch (err: any) {
    logger.error('Failed to stop tracert', 'Tracert', err)
    toast.error(`停止失败: ${err?.message || '未知错误'}`)
  }
}

const exportCSV = async () => {
  try {
    const result = await TracertService.ExportTracertResultCSV()
    if (!result || !result.content) {
      toast.warning('没有可导出的数据')
      return
    }
    downloadFile(result.content, result.fileName || 'tracert_result.csv')
    toast.success('CSV 导出成功')
  } catch (err: any) {
    logger.error('Failed to export CSV', 'Tracert', err)
    toast.error(`导出失败: ${err?.message || '未知错误'}`)
  }
}

const exportTXT = async () => {
  try {
    const result = await TracertService.ExportTracertResultTXT()
    if (!result || !result.content) {
      toast.warning('没有可导出的数据')
      return
    }
    downloadFile(result.content, result.fileName || 'tracert_result.txt')
    toast.success('TXT 导出成功')
  } catch (err: any) {
    logger.error('Failed to export TXT', 'Tracert', err)
    toast.error(`导出失败: ${err?.message || '未知错误'}`)
  }
}

const downloadFile = (content: string, fileName: string) => {
  const blob = new Blob([content], { type: 'text/plain;charset=utf-8;' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.setAttribute('href', url)
  link.setAttribute('download', fileName)
  link.style.visibility = 'hidden'
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(url)
}

const clearResults = () => {
  progress.value = null
  hopOverlay.value.clear()
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

const formatRtt = (rtt: number): string => {
  if (rtt < 0) return '-'
  if (rtt === 0) return '<1ms'
  return `${rtt.toFixed(1)}ms`
}

const getStatusIcon = (hop: TracertHopResult & { isProbing?: boolean }): string => {
  if (hop.isProbing) return '🔍'
  if (hop.reached) return '🎯'
  if (hop.status === 'success') return '🟢'
  if (hop.status === 'timeout') return '🔴'
  if (hop.status === 'error') return '⚠️'
  return '⏳'
}

const getStatusText = (hop: TracertHopResult & { isProbing?: boolean }): string => {
  if (hop.isProbing) return '探测中'
  if (hop.reached) return '到达目标'
  if (hop.status === 'success') return '中间路由'
  if (hop.status === 'timeout') return '超时'
  if (hop.status === 'error') return '错误'
  return '等待中'
}

// Lifecycle
onMounted(async () => {
  loadColumnConfig()

  try {
    const defaultConfig = await TracertService.GetTracertDefaultConfig()
    if (defaultConfig) {
      config.value = defaultConfig
    }
  } catch (err) {
    logger.error('Failed to get default config', 'Tracert', err)
  }

  if (!target.value.trim()) {
    showSettingsModal.value = true
  }

  unlistenProgress = Events.On('tracert:progress', handleProgressEvent)
  unlistenHopUpdate = Events.On('tracert:hop-update', handleHopUpdateEvent)
})

onUnmounted(() => {
  if (unlistenProgress) {
    unlistenProgress()
    unlistenProgress = null
  }
  if (unlistenHopUpdate) {
    unlistenHopUpdate()
    unlistenHopUpdate = null
  }
  stopPolling()
  hopOverlay.value.clear()
})
</script>

<template>
  <div class="h-full w-full flex flex-col relative bg-transparent">
    <!-- Header -->
    <div class="w-full relative z-10 mb-4 flex items-center justify-between">
      <h1 class="text-xl font-bold text-text-primary flex items-center">
        <span class="mr-2">🔍</span>
        Tracert 路径探测
      </h1>
      <div class="flex gap-2">
        <button
          @click="showSettingsModal = true"
          :disabled="isRunning"
          class="px-4 py-2 bg-bg-tertiary hover:bg-bg-hover border border-border text-text-primary rounded-lg font-medium transition-all duration-200 flex items-center gap-2 disabled:opacity-50 disabled:cursor-not-allowed"
        >
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
          </svg>
          设置
        </button>
        <button
          v-if="!isRunning"
          @click="startTracert"
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
          @click="stopTracert"
          class="px-4 py-2 bg-error hover:opacity-90 text-text-inverse rounded-lg font-medium transition-all duration-200 flex items-center gap-2"
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
    <div class="flex-1 flex flex-col gap-4 overflow-hidden">
      <!-- Stats Panel -->
      <section v-if="progress" class="bg-bg-secondary/60 backdrop-blur-sm border border-border rounded-xl shadow-md p-4">
        <div class="flex items-center justify-between mb-2">
          <div class="flex items-center gap-4">
            <span class="text-sm text-text-secondary">
              目标: <span class="font-mono text-text-primary">{{ progress.target }}</span>
              <span v-if="progress.resolvedIP" class="text-text-muted ml-1">({{ progress.resolvedIP }})</span>
            </span>
            <span v-if="progress.round > 0" class="text-sm text-text-secondary">
              轮次: <span class="font-medium text-accent">#{{ progress.round }}</span>
            </span>
            <span class="text-sm text-text-secondary">
              耗时: {{ formatElapsed(progress.elapsedMs) }}
            </span>
          </div>
          <div class="flex items-center gap-3">
            <span v-if="progress.isContinuous && progress.isRunning" class="text-xs px-2 py-0.5 bg-accent/20 text-accent rounded-full animate-pulse">
              持续探测中
            </span>
            <span v-if="progress.reachedDest" class="text-xs px-2 py-0.5 bg-success/20 text-success rounded-full">
              已到达目的地
            </span>
          </div>
        </div>

        <!-- Realtime stats -->
        <div v-if="realtimeStats" class="mt-3 pt-3 border-t border-border/50">
          <div class="grid grid-cols-6 gap-4 text-center">
            <div class="bg-bg-tertiary/30 rounded-lg p-2">
              <div class="text-xs text-text-muted">已完成跳数</div>
              <div class="text-lg font-bold text-text-primary">{{ realtimeStats.totalHops }}</div>
            </div>
            <div class="bg-bg-tertiary/30 rounded-lg p-2">
              <div class="text-xs text-text-muted">总发送</div>
              <div class="text-lg font-bold text-info">{{ realtimeStats.totalSent }}</div>
            </div>
            <div class="bg-bg-tertiary/30 rounded-lg p-2">
              <div class="text-xs text-text-muted">总接收</div>
              <div class="text-lg font-bold text-success">{{ realtimeStats.totalRecv }}</div>
            </div>
            <div class="bg-bg-tertiary/30 rounded-lg p-2">
              <div class="text-xs text-text-muted">平均延迟</div>
              <div class="text-lg font-bold text-text-primary">{{ realtimeStats.avgRtt > 0 ? realtimeStats.avgRtt.toFixed(1) + 'ms' : '-' }}</div>
            </div>
            <div class="bg-bg-tertiary/30 rounded-lg p-2">
              <div class="text-xs text-text-muted">到达目标</div>
              <div class="text-lg font-bold" :class="realtimeStats.reached ? 'text-success' : 'text-text-muted'">{{ realtimeStats.reached ? '是' : '否' }}</div>
            </div>
            <div class="bg-bg-tertiary/30 rounded-lg p-2">
              <div class="text-xs text-text-muted">当前轮次</div>
              <div class="text-lg font-bold text-accent">{{ realtimeStats.round }}</div>
            </div>
          </div>
        </div>
      </section>

      <!-- Results Table -->
      <section class="flex-1 bg-bg-secondary/60 backdrop-blur-sm border border-border rounded-xl shadow-md p-4 overflow-hidden flex flex-col">
        <div class="flex items-center justify-between mb-3">
          <h2 class="text-sm font-semibold text-text-primary flex items-center">
            <svg class="w-5 h-5 mr-2 text-accent" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 17v-2m3 2v-4m3 4v-6m2 10H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
            </svg>
            探测结果
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
              @click="exportTXT"
              :disabled="!progress || progress.hops.length === 0"
              class="px-3 py-1.5 bg-bg-tertiary hover:bg-bg-hover border border-border rounded-lg text-sm text-text-primary transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-1"
            >
              <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
              </svg>
              导出 TXT
            </button>
            <button
              @click="exportCSV"
              :disabled="!progress || progress.hops.length === 0"
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
          <table v-if="progress && progress.hops.length > 0" class="w-full text-sm">
            <thead class="sticky top-0 bg-bg-secondary">
              <tr class="text-left text-text-muted border-b border-border">
                <th v-if="isColumnVisible('index')" class="py-2 px-3 font-medium">#</th>
                <th v-if="isColumnVisible('hostName')" class="py-2 px-3 font-medium">主机名</th>
                <th v-if="isColumnVisible('ip')" class="py-2 px-3 font-medium">响应 IP</th>
                <th v-if="isColumnVisible('ttl')" class="py-2 px-3 font-medium">第几跳</th>
                <th v-if="isColumnVisible('lossRate')" class="py-2 px-3 font-medium">丢包率</th>
                <th v-if="isColumnVisible('sentCount')" class="py-2 px-3 font-medium">发送报文</th>
                <th v-if="isColumnVisible('recvCount')" class="py-2 px-3 font-medium">接收报文</th>
                <th v-if="isColumnVisible('minRtt')" class="py-2 px-3 font-medium">最低延迟</th>
                <th v-if="isColumnVisible('avgRtt')" class="py-2 px-3 font-medium">平均延迟</th>
                <th v-if="isColumnVisible('maxRtt')" class="py-2 px-3 font-medium">最高延迟</th>
                <th v-if="isColumnVisible('lastRtt')" class="py-2 px-3 font-medium">上次延迟</th>
                <th v-if="isColumnVisible('status')" class="py-2 px-3 font-medium">状态</th>
                <th v-if="isColumnVisible('errorMsg')" class="py-2 px-3 font-medium">错误信息</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="(hop, index) in displayResults"
                :key="hop.ttl"
                class="border-b border-border/50 hover:bg-bg-hover/50 transition-colors"
                :class="{ 'bg-accent/5': hop.isProbing }"
              >
                <td v-if="isColumnVisible('index')" class="py-2 px-3 text-text-muted">{{ index + 1 }}</td>
                <td v-if="isColumnVisible('hostName')" class="py-2 px-3 text-text-secondary text-xs">{{ hop.hostName || '-' }}</td>
                <td v-if="isColumnVisible('ip')" class="py-2 px-3 text-text-primary font-mono text-xs">{{ hop.ip || '-' }}</td>
                <td v-if="isColumnVisible('ttl')" class="py-2 px-3 text-text-primary font-mono font-medium">{{ hop.ttl }}</td>
                <td v-if="isColumnVisible('lossRate')" class="py-2 px-3">
                  <span v-if="hop.status !== 'pending'" :class="{
                    'text-success': hop.lossRate === 0,
                    'text-warning': hop.lossRate > 0 && hop.lossRate < 100,
                    'text-error': hop.lossRate === 100
                  }">{{ hop.lossRate.toFixed(1) }}%</span>
                  <span v-else class="text-text-muted">-</span>
                </td>
                <td v-if="isColumnVisible('sentCount')" class="py-2 px-3 text-text-primary">{{ hop.status !== 'pending' ? hop.sentCount : '-' }}</td>
                <td v-if="isColumnVisible('recvCount')" class="py-2 px-3 text-text-primary">{{ hop.status !== 'pending' ? hop.recvCount : '-' }}</td>
                <td v-if="isColumnVisible('minRtt')" class="py-2 px-3 font-mono text-xs">
                  <span v-if="hop.status !== 'pending' && hop.minRtt >= 0" class="text-info">{{ formatRtt(hop.minRtt) }}</span>
                  <span v-else class="text-text-muted">-</span>
                </td>
                <td v-if="isColumnVisible('avgRtt')" class="py-2 px-3 font-mono text-xs">
                  <span v-if="hop.status !== 'pending' && hop.avgRtt > 0" class="text-text-primary">{{ formatRtt(hop.avgRtt) }}</span>
                  <span v-else class="text-text-muted">-</span>
                </td>
                <td v-if="isColumnVisible('maxRtt')" class="py-2 px-3 font-mono text-xs">
                  <span v-if="hop.status !== 'pending' && hop.maxRtt > 0" class="text-warning">{{ formatRtt(hop.maxRtt) }}</span>
                  <span v-else class="text-text-muted">-</span>
                </td>
                <td v-if="isColumnVisible('lastRtt')" class="py-2 px-3 font-mono text-xs">
                  <span v-if="hop.status !== 'pending' && hop.lastRtt > 0" class="text-text-secondary">{{ formatRtt(hop.lastRtt) }}</span>
                  <span v-else class="text-text-muted">-</span>
                </td>
                <td v-if="isColumnVisible('status')" class="py-2 px-3">
                  <span class="flex items-center gap-1">
                    <span>{{ getStatusIcon(hop) }}</span>
                    <span :class="{
                      'text-accent animate-pulse': hop.isProbing,
                      'text-success': !hop.isProbing && (hop.status === 'success'),
                      'text-error': !hop.isProbing && hop.status === 'timeout',
                      'text-warning': !hop.isProbing && hop.status === 'error',
                      'text-text-muted': !hop.isProbing && hop.status === 'pending'
                    }">{{ getStatusText(hop) }}</span>
                  </span>
                </td>
                <td v-if="isColumnVisible('errorMsg')" class="py-2 px-3 text-text-muted text-xs">{{ hop.errorMsg || '-' }}</td>
              </tr>
            </tbody>
          </table>
          <div v-else class="h-full flex items-center justify-center text-text-muted">
            <div class="text-center">
              <svg class="w-16 h-16 mx-auto mb-4 opacity-30" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1" d="M22 12h-4l-3 9L9 3l-3 9H2" />
              </svg>
              <p>输入目标地址后点击"开始"进行路径探测</p>
            </div>
          </div>
        </div>
      </section>
    </div>

    <!-- Settings Modal -->
    <TracertSettingsModal
      v-model:show="showSettingsModal"
      v-model:target="target"
      v-model:config="config"
      v-model:continuous="continuous"
      :disabled="isRunning"
      @confirm="showSettingsModal = false"
    />

    <!-- Column Config Modal -->
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
                <span class="absolute top-0.5 left-0.5 w-4 h-4 bg-text-inverse rounded-full transition-transform" :class="col.visible ? 'translate-x-5' : ''"></span>
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
  </div>
</template>
