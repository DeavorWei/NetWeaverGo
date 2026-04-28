<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from 'vue'
import { Events } from '@wailsio/runtime'
import { useToast } from '@/utils/useToast'
import * as FileServerService from '@/bindings/github.com/NetWeaverGo/core/internal/ui/fileserverservice'
import { FileServerConfig } from '@/bindings/github.com/NetWeaverGo/core/internal/models/models'

interface LogEvent {
  timestamp: number
  level: string
  protocol: string
  clientIp: string
  action: string
  message: string
  file?: string
}

const toast = useToast()

const activeProtocol = ref<string>('sftp')
const configs = ref<Record<string, FileServerConfig>>({})
const runningStatus = ref<Record<string, boolean>>({ sftp: false, ftp: false, tftp: false })
const logs = ref<LogEvent[]>([])
const maxLogs = 1000
const autoScroll = ref(true)
const logsContainer = ref<HTMLElement | null>(null)

function getDefaultConfig(protocol: string): FileServerConfig {
  const cfg = new FileServerConfig({
    protocol,
    enabled: false,
    port: 2121,
    homeDir: '',
    username: 'admin',
    password: 'admin',
    allowGet: true,
    allowPut: true,
    allowDel: true,
    allowRename: true,
  })
  if (protocol === 'sftp') cfg.port = 2222
  if (protocol === 'tftp') { cfg.port = 6969; cfg.username = ''; cfg.password = '' }
  return cfg
}

function initConfigs() {
  for (const p of ['sftp', 'ftp', 'tftp']) {
    configs.value[p] = getDefaultConfig(p)
  }
}

const currentConfig = computed(() => configs.value[activeProtocol.value])
const isRunning = computed(() => runningStatus.value[activeProtocol.value])
const isFTP = computed(() => activeProtocol.value === 'ftp')

const protocols = [
  { key: 'sftp', label: 'SFTP' },
  { key: 'ftp', label: 'FTP' },
  { key: 'tftp', label: 'TFTP' },
]

async function loadConfigAndStatus() {
  for (const p of ['sftp', 'ftp', 'tftp']) {
    try {
      const cfg = await FileServerService.GetServerConfig(p)
      if (cfg) configs.value[p] = cfg
    } catch (e) { console.warn(`加载 ${p} 配置失败:`, e) }
    try {
      runningStatus.value[p] = await FileServerService.GetServerStatus(p)
    } catch (e) { console.warn(`获取 ${p} 状态失败:`, e) }
  }
}

async function saveConfig() {
  if (!currentConfig.value) return
  try {
    await FileServerService.SaveServerConfig(currentConfig.value)
    toast.success('配置已保存')
  } catch (error) {
    toast.error('保存配置失败')
  }
}

async function toggleServer() {
  if (!currentConfig.value) return
  try {
    const start = !isRunning.value
    await FileServerService.ToggleServer(activeProtocol.value, start)
    runningStatus.value[activeProtocol.value] = start
    toast.success(start ? `${activeProtocol.value.toUpperCase()} 服务器已启动` : `${activeProtocol.value.toUpperCase()} 服务器已停止`)
  } catch (error) {
    toast.error(`操作失败: ${error}`)
  }
}

async function disconnectAll() {
  try {
    await FileServerService.DisconnectAll(activeProtocol.value)
    toast.success('所有连接已断开')
  } catch (error) {
    toast.error('断开连接失败')
  }
}

function clearLogs() { logs.value = [] }

// 解包 Wails 事件数据
function unwrapEventData<T = any>(ev: any): T | null {
  if (!ev) return null
  const raw = Array.isArray(ev.data) ? (ev.data[0] ?? null) : (ev.data ?? null)
  if (typeof raw === 'string') {
    try {
      return JSON.parse(raw) as T
    } catch {
      return null
    }
  }
  return raw as T | null
}

function handleLogEvent(ev: any) {
  const data = unwrapEventData<LogEvent>(ev)
  if (!data) return
  
  logs.value.push({
    timestamp: data.timestamp || Date.now(),
    level: data.level || 'info',
    protocol: data.protocol || '',
    clientIp: data.clientIp || '',
    action: data.action || '',
    message: data.message || '',
    file: data.file,
  })
  if (logs.value.length > maxLogs) logs.value.shift()
  if (autoScroll.value && logsContainer.value) {
    setTimeout(() => { logsContainer.value?.scrollTo({ top: logsContainer.value?.scrollHeight || 0, behavior: 'smooth' }) }, 50)
  }
}

function getLevelClass(level: string): string {
  const m: Record<string, string> = { info: 'text-blue-400', warn: 'text-yellow-400', error: 'text-red-400', success: 'text-green-400' }
  return m[level] || 'text-gray-400'
}

function getActionClass(action: string): string {
  const m: Record<string, string> = { CONNECT: 'text-cyan-400', DISCONNECT: 'text-gray-500', UPLOAD: 'text-green-400', DOWNLOAD: 'text-blue-400', DELETE: 'text-red-400', ERROR: 'text-red-500' }
  return m[action] || 'text-gray-400'
}

function formatTime(ts: number): string {
  return new Date(ts).toLocaleTimeString('zh-CN', { hour12: false })
}

onMounted(async () => {
  initConfigs()
  await loadConfigAndStatus()
  Events.On('fileserver:log', handleLogEvent)
})

onUnmounted(() => { Events.Off('fileserver:log') })
</script>

<template>
  <div class="flex flex-col h-full gap-4">
    <!-- 控制面板 -->
    <div class="bg-bg-secondary rounded-lg border border-border p-4">
      <!-- 协议切换 -->
      <div class="flex gap-2 mb-4">
        <button
          v-for="p in protocols" :key="p.key"
          @click="activeProtocol = p.key"
          :class="[
            'px-4 py-2 rounded-lg text-sm font-medium transition-all duration-200',
            activeProtocol === p.key
              ? 'bg-accent-bg text-accent border border-accent/30'
              : 'bg-bg-hover text-text-secondary hover:text-text-primary border border-border'
          ]"
        >{{ p.label }}</button>
      </div>

      <!-- 配置表单 -->
      <div v-if="currentConfig" class="grid grid-cols-2 gap-4 mb-4">
        <div>
          <label class="block text-sm text-text-muted mb-1">监听端口</label>
          <input v-model.number="currentConfig.port" type="number"
            class="w-full px-3 py-2 rounded-lg bg-bg-primary border border-border text-text-primary focus:border-accent focus:outline-none"
            :disabled="isRunning" />
        </div>
        <div>
          <label class="block text-sm text-text-muted mb-1">根目录</label>
          <input v-model="currentConfig.homeDir" type="text"
            class="w-full px-3 py-2 rounded-lg bg-bg-primary border border-border text-text-primary focus:border-accent focus:outline-none"
            :disabled="isRunning" placeholder="文件服务根目录" />
        </div>
        <div v-if="activeProtocol !== 'tftp'">
          <label class="block text-sm text-text-muted mb-1">用户名</label>
          <input v-model="currentConfig.username" type="text"
            class="w-full px-3 py-2 rounded-lg bg-bg-primary border border-border text-text-primary focus:border-accent focus:outline-none"
            :disabled="isRunning" />
        </div>
        <div v-if="activeProtocol !== 'tftp'">
          <label class="block text-sm text-text-muted mb-1">密码</label>
          <input v-model="currentConfig.password" type="password"
            class="w-full px-3 py-2 rounded-lg bg-bg-primary border border-border text-text-primary focus:border-accent focus:outline-none"
            :disabled="isRunning" />
        </div>
      </div>

      <!-- FTP 权限控制 -->
      <div v-if="isFTP && currentConfig" class="flex gap-4 mb-4">
        <label class="flex items-center gap-2 text-sm text-text-secondary">
          <input v-model="currentConfig.allowGet" type="checkbox" class="w-4 h-4 rounded border-border accent-accent" :disabled="isRunning" />
          允许下载 (GET)
        </label>
        <label class="flex items-center gap-2 text-sm text-text-secondary">
          <input v-model="currentConfig.allowPut" type="checkbox" class="w-4 h-4 rounded border-border accent-accent" :disabled="isRunning" />
          允许上传 (PUT)
        </label>
        <label class="flex items-center gap-2 text-sm text-text-secondary">
          <input v-model="currentConfig.allowDel" type="checkbox" class="w-4 h-4 rounded border-border accent-accent" :disabled="isRunning" />
          允许删除 (DEL)
        </label>
        <label class="flex items-center gap-2 text-sm text-text-secondary">
          <input v-model="currentConfig.allowRename" type="checkbox" class="w-4 h-4 rounded border-border accent-accent" :disabled="isRunning" />
          允许重命名 (RENAME)
        </label>
      </div>

      <!-- 操作按钮 -->
      <div class="flex gap-3">
        <button @click="toggleServer"
          :class="[
            'px-6 py-2.5 rounded-lg font-medium transition-all duration-200',
            isRunning
              ? 'bg-red-500/20 text-red-400 border border-red-500/30 hover:bg-red-500/30'
              : 'bg-green-500/20 text-green-400 border border-green-500/30 hover:bg-green-500/30'
          ]"
        >{{ isRunning ? '停止服务' : '启动服务' }}</button>
        <button @click="saveConfig" :disabled="isRunning"
          class="px-4 py-2.5 rounded-lg bg-accent-bg text-accent border border-accent/30 font-medium hover:bg-accent/20 transition-all duration-200 disabled:opacity-50 disabled:cursor-not-allowed"
        >保存配置</button>
        <button @click="disconnectAll" :disabled="!isRunning"
          class="px-4 py-2.5 rounded-lg bg-bg-hover text-text-secondary border border-border font-medium hover:text-text-primary transition-all duration-200 disabled:opacity-50 disabled:cursor-not-allowed"
        >断开所有连接</button>
      </div>

      <!-- 状态指示 -->
      <div class="flex items-center gap-2 mt-4">
        <span :class="['w-2 h-2 rounded-full', isRunning ? 'bg-green-500 animate-pulse' : 'bg-gray-500']"></span>
        <span class="text-sm text-text-muted">
          {{ isRunning ? '运行中' : '已停止' }} - 端口 {{ currentConfig?.port || 0 }}
        </span>
      </div>
    </div>

    <!-- 日志面板 -->
    <div class="flex-1 bg-bg-secondary rounded-lg border border-border overflow-hidden flex flex-col">
      <div class="flex items-center justify-between px-4 py-2 border-b border-border">
        <span class="text-sm font-medium text-text-primary">服务日志</span>
        <div class="flex gap-2">
          <label class="flex items-center gap-2 text-sm text-text-secondary">
            <input v-model="autoScroll" type="checkbox" class="w-3 h-3 rounded border-border accent-accent" />
            自动滚动
          </label>
          <button @click="clearLogs" class="px-3 py-1 rounded text-xs bg-bg-hover text-text-muted hover:text-text-primary transition-colors">清除日志</button>
        </div>
      </div>
      <div ref="logsContainer" class="flex-1 overflow-auto p-3 font-mono text-xs bg-bg-primary scrollbar-custom">
        <div v-if="logs.length === 0" class="text-center text-text-muted py-8">暂无日志记录</div>
        <div v-for="(log, index) in logs" :key="index" class="flex gap-2 py-1 hover:bg-bg-hover/50 px-1 rounded">
          <span class="text-text-muted">{{ formatTime(log.timestamp) }}</span>
          <span :class="getLevelClass(log.level)">[{{ log.level.toUpperCase() }}]</span>
          <span class="text-purple-400">[{{ log.protocol.toUpperCase() }}]</span>
          <span v-if="log.clientIp" class="text-orange-400">{{ log.clientIp }}</span>
          <span :class="getActionClass(log.action)">{{ log.action }}</span>
          <span class="text-text-primary">{{ log.message }}</span>
          <span v-if="log.file" class="text-cyan-400">{{ log.file }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.scrollbar-custom::-webkit-scrollbar { width: 6px; }
.scrollbar-custom::-webkit-scrollbar-track { background: transparent; }
.scrollbar-custom::-webkit-scrollbar-thumb { background: var(--color-border); border-radius: 3px; }
.scrollbar-custom::-webkit-scrollbar-thumb:hover { background: var(--color-text-muted); }
</style>