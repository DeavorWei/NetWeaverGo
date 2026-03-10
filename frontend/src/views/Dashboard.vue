<template>
  <div class="animate-slide-in space-y-6">
    <!-- 页面标题 -->
    <div>
      <p class="text-sm text-text-muted">当前配置状态与资产统计</p>
    </div>

    <!-- 统计卡片组 -->
    <div class="grid grid-cols-3 gap-5">
      <!-- 注册设备资产 -->
      <div class="group relative bg-bg-card border border-border rounded-xl p-5 shadow-card hover:border-accent/50 hover:shadow-glow transition-all duration-300 cursor-default overflow-hidden">
        <div class="absolute inset-0 bg-gradient-to-br from-accent/5 to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-300 pointer-events-none"></div>
        <div class="flex items-start justify-between">
          <div>
            <p class="text-xs font-medium text-text-muted uppercase tracking-wider">注册设备资产</p>
            <div class="mt-3 text-4xl font-bold text-accent tabular-nums">{{ deviceCount }}</div>
          </div>
          <div class="w-10 h-10 rounded-lg bg-accent/10 flex items-center justify-center flex-shrink-0">
            <svg xmlns="http://www.w3.org/2000/svg" class="w-5 h-5 text-accent" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <rect x="2" y="2" width="20" height="8" rx="2"/><rect x="2" y="14" width="20" height="8" rx="2"/>
              <line x1="6" y1="6" x2="6.01" y2="6"/><line x1="6" y1="18" x2="6.01" y2="18"/>
            </svg>
          </div>
        </div>
        <div class="mt-4 text-xs text-text-muted">共计 {{ deviceCount }} 台网络设备</div>
      </div>

      <!-- 下发命令集合 -->
      <div class="group relative bg-bg-card border border-border rounded-xl p-5 shadow-card hover:border-success/50 hover:shadow-glow transition-all duration-300 cursor-default overflow-hidden">
        <div class="absolute inset-0 bg-gradient-to-br from-success/5 to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-300 pointer-events-none"></div>
        <div class="flex items-start justify-between">
          <div>
            <p class="text-xs font-medium text-text-muted uppercase tracking-wider">下发命令集合</p>
            <div class="mt-3 text-4xl font-bold text-success tabular-nums">{{ commandCount }}</div>
          </div>
          <div class="w-10 h-10 rounded-lg bg-success/10 flex items-center justify-center flex-shrink-0">
            <svg xmlns="http://www.w3.org/2000/svg" class="w-5 h-5 text-success" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <polyline points="4 17 10 11 4 5"/><line x1="12" y1="19" x2="20" y2="19"/>
            </svg>
          </div>
        </div>
        <div class="mt-4 text-xs text-text-muted">共计 {{ commandCount }} 条待执行命令</div>
      </div>

      <!-- 最后运行状态 -->
      <div class="group relative bg-bg-card border border-border rounded-xl p-5 shadow-card hover:border-warning/50 hover:shadow-glow transition-all duration-300 cursor-default overflow-hidden">
        <div class="absolute inset-0 bg-gradient-to-br from-warning/5 to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-300 pointer-events-none"></div>
        <div class="flex items-start justify-between">
          <div>
            <p class="text-xs font-medium text-text-muted uppercase tracking-wider">最后运行状态</p>
            <div class="mt-3 text-3xl font-bold tabular-nums" :class="statusColor">{{ statusLabel }}</div>
          </div>
          <div class="w-10 h-10 rounded-lg flex items-center justify-center flex-shrink-0" :class="statusBg">
            <span class="w-3 h-3 rounded-full" :class="statusDot"></span>
          </div>
        </div>
        <div class="mt-4 text-xs text-text-muted">{{ statusDesc }}</div>
      </div>
    </div>

    <!-- 快速操作入口 -->
    <div class="bg-bg-card border border-border rounded-xl p-5 shadow-card">
      <h3 class="text-sm font-semibold text-text-secondary mb-4">快速导航</h3>
      <div class="grid grid-cols-2 gap-3">
        <button
          @click="$router.push({ name: 'Devices' })"
          class="flex items-center gap-3 p-4 rounded-lg bg-bg-panel border border-border hover:border-accent/40 hover:bg-bg-hover transition-all duration-200 group"
        >
          <div class="w-8 h-8 rounded-lg bg-accent/10 flex items-center justify-center">
            <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4 text-accent" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <rect x="2" y="2" width="20" height="8" rx="2"/><rect x="2" y="14" width="20" height="8" rx="2"/>
            </svg>
          </div>
          <div class="text-left">
            <div class="text-sm font-medium text-text-primary group-hover:text-accent transition-colors">查看设备资产</div>
            <div class="text-xs text-text-muted">{{ deviceCount }} 台设备</div>
          </div>
        </button>
        <button
          @click="$router.push({ name: 'Tasks' })"
          class="flex items-center gap-3 p-4 rounded-lg bg-bg-panel border border-border hover:border-success/40 hover:bg-bg-hover transition-all duration-200 group"
        >
          <div class="w-8 h-8 rounded-lg bg-success/10 flex items-center justify-center">
            <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4 text-success" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <polyline points="4 17 10 11 4 5"/><line x1="12" y1="19" x2="20" y2="19"/>
            </svg>
          </div>
          <div class="text-left">
            <div class="text-sm font-medium text-text-primary group-hover:text-success transition-colors">开始任务执行</div>
            <div class="text-xs text-text-muted">{{ commandCount }} 条命令待下发</div>
          </div>
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { EnsureConfig } from '../services/api'

const deviceCount  = ref(0)
const commandCount = ref(0)
const status       = ref('Idle')

const statusColor = computed(() => {
  switch (status.value) {
    case 'Ready':          return 'text-success'
    case 'Config Missing': return 'text-warning'
    case 'Error':          return 'text-error'
    default:               return 'text-text-muted'
  }
})
const statusBg = computed(() => {
  switch (status.value) {
    case 'Ready':          return 'bg-success/10'
    case 'Config Missing': return 'bg-warning/10'
    case 'Error':          return 'bg-error/10'
    default:               return 'bg-bg-panel'
  }
})
const statusDot = computed(() => {
  switch (status.value) {
    case 'Ready':          return 'bg-success animate-pulse'
    case 'Config Missing': return 'bg-warning'
    case 'Error':          return 'bg-error'
    default:               return 'bg-text-muted'
  }
})
const statusLabel = computed(() => {
  const map: Record<string, string> = { Ready: 'Ready', 'Config Missing': 'Missing', Error: 'Error', Idle: 'Idle' }
  return map[status.value] ?? status.value
})
const statusDesc = computed(() => {
  switch (status.value) {
    case 'Ready':          return '配置完整，可以开始任务'
    case 'Config Missing': return '缺少必要配置文件'
    case 'Error':          return '上次运行出现错误'
    default:               return '等待配置加载'
  }
})

onMounted(async () => {
  try {
    const [assets, commands, missingFiles] = await EnsureConfig()
    deviceCount.value  = assets   ? assets.length   : 0
    commandCount.value = commands ? commands.length  : 0
    if (missingFiles && missingFiles.length > 0) {
      status.value = 'Config Missing'
    } else if (deviceCount.value > 0 && commandCount.value > 0) {
      status.value = 'Ready'
    }
  } catch (err) {
    console.error('Failed to load dashboard data:', err)
    status.value = 'Error'
  }
})
</script>
