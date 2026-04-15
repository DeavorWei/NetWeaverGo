<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from 'vue'
import { Events } from '@wailsio/runtime'
import * as PingService from '@/bindings/github.com/NetWeaverGo/core/internal/ui/pingservice'
import type { PingConfig, BatchPingProgress } from '@/bindings/github.com/NetWeaverGo/core/internal/icmp/models'
import type { PingRequest } from '@/bindings/github.com/NetWeaverGo/core/internal/ui/models'

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

// Methods
const startPing = async () => {
  try {
    const request: PingRequest = {
      targets: targetInput.value,
      config: config.value,
      deviceIds: []
    }
    const result = await PingService.StartBatchPing(request)
    progress.value = result
  } catch (err) {
    console.error('Failed to start ping:', err)
  }
}

const stopPing = async () => {
  try {
    await PingService.StopBatchPing()
  } catch (err) {
    console.error('Failed to stop ping:', err)
  }
}

const exportCSV = async () => {
  try {
    const result = await PingService.ExportPingResultCSV()
    if (!result || !result.content) return

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
  } catch (err) {
    console.error('Failed to export CSV:', err)
  }
}

const clearResults = () => {
  progress.value = null
}

const formatRtt = (rtt: number): string => {
  if (rtt === 0) return '-'
  return `${rtt}ms`
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

  // Subscribe to events
  Events.On('ping:progress', handleProgressEvent)
})

onUnmounted(() => {
  Events.Off('ping:progress')
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
            placeholder="输入 IP 地址，每行一个&#10;支持格式：&#10;• 单个 IP: 192.168.1.1&#10;• CIDR: 192.168.1.0/24&#10;• 范围: 192.168.1.1-100"
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
                max="10000"
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
                min="1"
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
                max="10000"
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
