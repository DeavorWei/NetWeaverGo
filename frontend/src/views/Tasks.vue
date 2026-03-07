<template>
  <div class="animate-slide-in space-y-5 h-full flex flex-col">
    <!-- 标题栏 + 操作按钮 -->
    <div class="flex items-center justify-between flex-shrink-0">
      <div class="flex items-center gap-4">
        <p class="text-sm text-text-muted">并发连接设备并下发配置命令</p>
        <!-- 命令编辑器入口 -->
        <CommandEditor @saved="onCommandsSaved" />
      </div>
      <div class="flex gap-3">
        <button
          @click="startEngine"
          :disabled="isRunning"
          class="flex items-center gap-2 px-5 py-2.5 rounded-lg text-sm font-semibold transition-all duration-200 shadow-card"
          :class="isRunning
            ? 'bg-bg-card border border-border text-text-muted cursor-not-allowed'
            : 'bg-accent hover:bg-accent-glow text-white border border-accent/30 hover:shadow-glow'"
        >
          <svg v-if="!isRunning" xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor">
            <polygon points="5 3 19 12 5 21 5 3"/>
          </svg>
          <span v-if="isRunning" class="w-4 h-4 border-2 border-text-muted border-t-transparent rounded-full animate-spin"></span>
          {{ isRunning ? '执行中...' : '开始下发命令' }}
        </button>
        <button
          @click="startBackup"
          :disabled="isRunning"
          class="flex items-center gap-2 px-5 py-2.5 rounded-lg text-sm font-semibold transition-all duration-200 shadow-card bg-bg-card border border-border text-text-muted hover:text-text-primary hover:border-accent/50 group"
          :class="isRunning ? 'cursor-not-allowed opacity-50' : ''"
        >
          <svg v-if="!isRunning" xmlns="http://www.w3.org/2000/svg" class="w-4 h-4 text-accent transition-transform group-hover:scale-110" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
            <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/>
          </svg>
          备份交换机配置
        </button>
      </div>
    </div>

    <!-- 进度条 -->
    <div v-if="isRunning || progressPercent > 0" class="flex-shrink-0 space-y-1.5">
      <div class="flex items-center justify-between text-xs text-text-muted">
        <span>总体进度</span>
        <span class="font-mono">{{ progressPercent }}%</span>
      </div>
      <div class="h-2 bg-bg-card rounded-full overflow-hidden border border-border">
        <div
          class="h-full rounded-full transition-all duration-500 ease-out"
          :class="progressPercent === 100 ? 'bg-success' : 'bg-accent'"
          :style="{ width: progressPercent + '%' }"
        ></div>
      </div>
    </div>

    <!-- 设备卡片网格 -->
    <div class="flex-1 overflow-auto scrollbar-custom min-h-0">
      <div v-if="devices.length === 0" class="flex flex-col items-center justify-center h-48 text-text-muted gap-3">
        <svg xmlns="http://www.w3.org/2000/svg" class="w-12 h-12 text-text-muted/30" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
          <polyline points="4 17 10 11 4 5"/><line x1="12" y1="19" x2="20" y2="19"/>
        </svg>
        <p class="text-sm">点击「开始下发命令」启动任务，设备卡片将在此实时显示</p>
      </div>
      <div v-else class="grid grid-cols-3 gap-4">
        <div
          v-for="dev in devices"
          :key="dev.ip"
          class="bg-bg-card border rounded-xl overflow-hidden shadow-card transition-all duration-300"
          :class="statusBorder(dev.status)"
        >
          <!-- 卡片头部 -->
          <div class="flex items-center justify-between px-4 py-3 border-b border-border bg-bg-panel">
            <span class="font-mono text-sm font-semibold text-text-primary">{{ dev.ip }}</span>
            <span
              class="flex items-center gap-1.5 text-xs font-medium px-2.5 py-1 rounded-full border"
              :class="statusBadge(dev.status)"
            >
              <span class="w-1.5 h-1.5 rounded-full" :class="statusDot(dev.status)"></span>
              {{ statusLabel(dev.status) }}
            </span>
          </div>
          <!-- 终端日志区 -->
          <div
            class="h-52 overflow-y-auto scrollbar-custom bg-terminal-bg p-3 font-mono text-xs leading-relaxed"
            :ref="el => setTerminalRef(dev.ip, el)"
          >
            <div
              v-for="(log, idx) in dev.logs"
              :key="idx"
              class="whitespace-pre-wrap break-all mb-0.5"
              :class="logColor(log)"
            >{{ log }}</div>
          </div>
        </div>
      </div>
    </div>

    <!-- 自定义 Modal 对话框 -->
    <Transition name="modal">
      <div v-if="modal.show" class="fixed inset-0 z-50 flex items-center justify-center">
        <div class="absolute inset-0 bg-black/60 backdrop-blur-sm" @click="modal.show = false"></div>
        <div class="relative bg-bg-card border border-warning/50 rounded-xl shadow-card max-w-md w-full mx-4 overflow-hidden animate-slide-in">
          <!-- 对话框头部 -->
          <div class="flex items-center gap-3 px-5 py-4 border-b border-border bg-warning/5">
            <div class="w-9 h-9 rounded-lg bg-warning/15 flex items-center justify-center flex-shrink-0">
              <svg xmlns="http://www.w3.org/2000/svg" class="w-5 h-5 text-warning" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>
            </div>
            <div>
              <h3 class="text-sm font-semibold text-text-primary">异常干预（阻塞流）</h3>
              <p class="text-xs text-text-muted mt-0.5">当前设备已被挂起，全局其余设备继续执行</p>
            </div>
          </div>
          <!-- 内容 -->
          <div class="px-5 py-4 space-y-3">
            <div class="bg-bg-panel border border-border rounded-lg p-3 font-mono text-xs text-text-secondary leading-relaxed whitespace-pre-wrap">{{ modal.content }}</div>
            <p class="text-xs text-text-muted">请选择如何处理该设备的挂起任务：</p>
          </div>
          <!-- 操作按钮 -->
          <div class="flex gap-3 px-5 py-4 border-t border-border">
            <button
              @click="resolveModal('S')"
              class="flex-1 py-2.5 text-sm font-medium rounded-lg bg-accent/20 border border-accent/40 text-accent hover:bg-accent hover:text-white transition-all duration-200"
            >放弃此指令继续 (Skip)</button>
            <button
              @click="resolveModal('A')"
              class="flex-1 py-2.5 text-sm font-medium rounded-lg bg-error/20 border border-error/40 text-error hover:bg-error hover:text-white transition-all duration-200"
            >掐断设备连接 (Abort)</button>
          </div>
        </div>
      </div>
    </Transition>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, nextTick } from 'vue'
// @ts-ignore
import { StartEngineWails, StartBackupWails, ResolveSuspend } from '../bindings/github.com/NetWeaverGo/core/internal/ui/appservice.js'
import { Events } from '@wailsio/runtime'
import CommandEditor from '../components/task/CommandEditor.vue'

const isRunning      = ref(false)
const progressPercent= ref(0)
const devices        = ref<any[]>([])

// 终端 DOM 引用
const terminalRefs = new Map<string, Element>()
function setTerminalRef(ip: string, el: any) {
  if (el) terminalRefs.set(ip, el as Element)
  else terminalRefs.delete(ip)
}

// 自定义 modal 状态
const modal = ref({ show: false, ip: '', content: '' })
function resolveModal(action: 'S' | 'A') {
  ResolveSuspend(modal.value.ip, action)
  modal.value.show = false
}

// 状态样式映射
function statusBorder(s: string) {
  switch (s) {
    case 'running': return 'border-accent/50'
    case 'success': return 'border-success/50'
    case 'error':   return 'border-error/50'
    case 'waiting': return 'border-warning/40'
    default:        return 'border-border'
  }
}
function statusBadge(s: string) {
  switch (s) {
    case 'running': return 'bg-accent/10 border-accent/30 text-accent'
    case 'success': return 'bg-success/10 border-success/30 text-success'
    case 'error':   return 'bg-error/10 border-error/30 text-error'
    case 'waiting': return 'bg-warning/10 border-warning/30 text-warning'
    default:        return 'bg-bg-panel border-border text-text-muted'
  }
}
function statusDot(s: string) {
  switch (s) {
    case 'running': return 'bg-accent animate-pulse'
    case 'success': return 'bg-success'
    case 'error':   return 'bg-error'
    case 'waiting': return 'bg-warning animate-pulse'
    default:        return 'bg-text-muted'
  }
}
function statusLabel(s: string) {
  const map: Record<string, string> = {
    running: '执行中', success: '成功', error: '失败', waiting: '等待', idle: '空闲'
  }
  return map[s] ?? s
}
function logColor(log: string) {
  const l = log.toLowerCase()
  if (l.includes('error') || l.includes('失败') || l.includes('错误')) return 'text-error'
  if (l.includes('warn')  || l.includes('警告'))                        return 'text-warning'
  if (l.includes('success')|| l.includes('成功') || l.includes('完成')) return 'text-success'
  if (log.startsWith('[')) return 'text-info'
  return 'text-text-muted'
}

function onCommandsSaved(commands: string[]) {
  console.log('命令已更新，共', commands.length, '条')
}

async function startEngine() {
  if (isRunning.value) return
  isRunning.value    = true
  progressPercent.value = 5
  devices.value      = []
  try {
    await StartEngineWails()
  } catch (err: any) {
    console.error('启动失败:', err)
    isRunning.value = false
  }
}

async function startBackup() {
  if (isRunning.value) return
  isRunning.value = true
  progressPercent.value = 5
  devices.value = []
  try {
    await StartBackupWails()
  } catch (err: any) {
    console.error('备份启动失败:', err)
    isRunning.value = false
  }
}

let eventHandlers: any[] = []
onMounted(() => {
  const hFinished = Events.On('engine:finished', () => {
    isRunning.value = false
    progressPercent.value = 100
  })

  const hEvent = Events.On('device:event', (ev: any) => {
    const data = ev.data[0]
    let dev = devices.value.find(d => d.ip === data.IP)
    if (!dev) {
      dev = { ip: data.IP, status: 'waiting', logs: [] }
      devices.value.push(dev)
    }
    if (data.Message) {
      dev.logs.push(data.Message)
      nextTick(() => {
        const el = terminalRefs.get(dev.ip)
        if (el) el.scrollTop = el.scrollHeight
      })
    }
    if (data.Type === 'start') dev.status = 'running'
    else if ((data.Type === 'success' || data.Type === 'skip') && dev.status !== 'error') dev.status = 'success'
    else if (data.Type === 'error' || data.Type === 'abort') dev.status = 'error'

    const totalComplete = devices.value.reduce((acc, curr) =>
      acc + (curr.status === 'success' || curr.status === 'error' ? 1 : 0), 0)
    if (devices.value.length > 0) {
      progressPercent.value = Math.min(95, Math.floor((totalComplete / devices.value.length) * 100))
    }
  })

  const hSuspend = Events.On('engine:suspend_required', (ev: any) => {
    const data = ev.data[0]
    modal.value = {
      show:    true,
      ip:      data.ip,
      content: `设备: ${data.ip}\n命令: ${data.command}\n\n错误详情:\n${data.error}`,
    }
  })

  eventHandlers.push(hFinished, hEvent, hSuspend)
})

onUnmounted(() => {
  eventHandlers.forEach(h => {
    if (h && typeof h === 'function') h()
    else if (h && h.cancel) h.cancel()
  })
})
</script>

<style scoped>
.modal-enter-active, .modal-leave-active {
  transition: opacity 0.2s ease;
}
.modal-enter-from, .modal-leave-to {
  opacity: 0;
}
</style>
