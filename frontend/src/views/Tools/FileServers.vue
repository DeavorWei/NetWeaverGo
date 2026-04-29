<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from 'vue'
import { Events, Dialogs } from '@wailsio/runtime'
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
const runningStatus = ref<Record<string, boolean>>({ sftp: false, ftp: false, tftp: false, http: false })
const logs = ref<LogEvent[]>([])
const maxLogs = 1000
const autoScroll = ref(true)
const logsContainer = ref<HTMLElement | null>(null)

// 密码显示状态
const showPassword = ref(false)

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
  if (protocol === 'http') { cfg.port = 8080; cfg.username = ''; cfg.password = ''; cfg.allowRename = false }
  return cfg
}

// 初始化配置，如果 homeDir 为空则填充默认目录
async function initConfigs() {
  for (const p of ['sftp', 'ftp', 'tftp', 'http']) {
    configs.value[p] = getDefaultConfig(p)
  }
  // 为所有协议填充默认根目录
  await fillDefaultHomeDirForAll()
}

// 为所有协议填充默认根目录
async function fillDefaultHomeDirForAll() {
  try {
    const defaultDir = await FileServerService.GetDefaultHomeDir()
    for (const p of ['sftp', 'ftp', 'tftp', 'http']) {
      if (configs.value[p] && !configs.value[p].homeDir) {
        configs.value[p].homeDir = defaultDir
      }
    }
  } catch (e) {
    console.warn('获取默认根目录失败:', e)
  }
}

// 选择目录
async function selectHomeDir() {
  if (isRunning.value) return
  try {
    const result = await Dialogs.OpenFile({
      CanChooseDirectories: true,
      CanChooseFiles: false,
      CanCreateDirectories: true,
      Title: '选择文件服务器根目录',
      ButtonText: '选择',
      Directory: currentConfig.value?.homeDir || ''
    })
    if (result && typeof result === 'string' && result !== '') {
      currentConfig.value!.homeDir = result
    }
  } catch (e) {
    console.warn('选择目录失败:', e)
  }
}

const currentConfig = computed(() => configs.value[activeProtocol.value])
const isRunning = computed(() => runningStatus.value[activeProtocol.value])
const isFTP = computed(() => activeProtocol.value === 'ftp')
const isHTTP = computed(() => activeProtocol.value === 'http')
const hasAuth = computed(() => isHTTP.value && currentConfig.value && (currentConfig.value.username || currentConfig.value.password))

const protocols = [
  { key: 'sftp', label: 'SFTP' },
  { key: 'ftp', label: 'FTP' },
  { key: 'tftp', label: 'TFTP' },
  { key: 'http', label: 'HTTP' },
]

async function loadConfigAndStatus() {
  for (const p of ['sftp', 'ftp', 'tftp', 'http']) {
    try {
      const cfg = await FileServerService.GetServerConfig(p)
      if (cfg) {
        configs.value[p] = cfg
        // 如果根目录为空，填充默认目录
        if (!cfg.homeDir || cfg.homeDir === '') {
          try {
            const defaultDir = await FileServerService.GetDefaultHomeDir()
            if (configs.value[p]) {
              configs.value[p].homeDir = defaultDir
            }
          } catch (e) { console.warn(`获取默认目录失败:`, e) }
        }
      }
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
    if (start) {
      // 启动前先保存配置，确保认证信息等已持久化
      await FileServerService.SaveServerConfig(currentConfig.value)
    }
    await FileServerService.ToggleServer(activeProtocol.value, start)
    runningStatus.value[activeProtocol.value] = start
    
    // 启动成功后重新加载配置，确保认证信息正确显示
    if (start) {
      const cfg = await FileServerService.GetServerConfig(activeProtocol.value)
      if (cfg) {
        configs.value[activeProtocol.value] = cfg
      }
    }
    
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

// 是否启用认证（HTTP专用）
const enableAuth = computed({
  get: () => isHTTP.value && hasAuth.value,
  set: (val: boolean) => {
    if (!isHTTP.value || !currentConfig.value) return
    if (val) {
      // 启用认证时设置默认凭据
      currentConfig.value.username = currentConfig.value.username || 'admin'
      currentConfig.value.password = currentConfig.value.password || 'admin'
    } else {
      // 禁用认证时清空凭据
      currentConfig.value.username = ''
      currentConfig.value.password = ''
    }
  }
})

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
  const m: Record<string, string> = { info: 'text-info', warn: 'text-warning', error: 'text-error', success: 'text-success' }
  return m[level] || 'text-text-muted'
}

function getActionClass(action: string): string {
  const m: Record<string, string> = { CONNECT: 'text-info', DISCONNECT: 'text-text-muted', UPLOAD: 'text-success', DOWNLOAD: 'text-accent', DELETE: 'text-error', ERROR: 'text-error', BROWSE: 'text-warning' }
  return m[action] || 'text-text-muted'
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
  <div class="animate-slide-in space-y-5 h-full flex flex-col">
    <!-- 控制面板 -->
    <div class="bg-bg-card rounded-xl border border-border p-5 shadow-card">
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
      <div v-if="currentConfig" class="flex gap-4 mb-4">
        <!-- 端口 - 占 1/6 -->
        <div class="space-y-1.5" :class="activeProtocol === 'tftp' ? 'w-[25%]' : activeProtocol === 'http' ? 'w-[15%]' : 'w-[16.666%]'">
          <label class="text-sm font-medium text-text-primary">监听端口</label>
          <input v-model.number="currentConfig.port" type="number"
            class="w-full px-3 py-2 rounded-lg bg-bg-panel border border-border text-sm text-text-primary focus:border-accent focus:outline-none focus:ring-1 focus:ring-accent/20 transition-all"
            :disabled="isRunning" />
        </div>
        <!-- HTTP 认证开关 -->
        <div v-if="isHTTP" class="space-y-1.5 w-[15%]">
          <label class="text-sm font-medium text-text-primary">基本认证</label>
          <select v-model="enableAuth"
            class="w-full px-3 py-2 rounded-lg bg-bg-panel border border-border text-sm text-text-primary focus:border-accent focus:outline-none focus:ring-1 focus:ring-accent/20 transition-all"
            :disabled="isRunning">
            <option :value="false">无认证</option>
            <option :value="true">基本认证</option>
          </select>
        </div>
        <!-- 用户名 - 占 1/6 (TFTP 时不显示) -->
        <div v-if="activeProtocol !== 'tftp' && (!isHTTP || enableAuth)" class="space-y-1.5" :class="isHTTP ? 'w-[15%]' : 'w-[16.666%]'">
          <label class="text-sm font-medium text-text-primary">用户名</label>
          <input v-model="currentConfig.username" type="text"
            class="w-full px-3 py-2 rounded-lg bg-bg-panel border border-border text-sm text-text-primary focus:border-accent focus:outline-none focus:ring-1 focus:ring-accent/20 transition-all"
            :disabled="isRunning || (isHTTP && !enableAuth)" />
        </div>
        <!-- 密码 - 占 1/6 (TFTP 时不显示) -->
        <div v-if="activeProtocol !== 'tftp' && (!isHTTP || enableAuth)" class="space-y-1.5" :class="isHTTP ? 'w-[15%]' : 'w-[16.666%]'">
          <label class="text-sm font-medium text-text-primary">密码</label>
          <div class="relative">
            <input v-model="currentConfig.password" :type="showPassword ? 'text' : 'password'"
              class="w-full px-3 py-2 pr-10 rounded-lg bg-bg-panel border border-border text-sm text-text-primary focus:border-accent focus:outline-none focus:ring-1 focus:ring-accent/20 transition-all"
              :disabled="isRunning || (isHTTP && !enableAuth)" />
            <button
              type="button"
              @click="showPassword = !showPassword"
              :title="showPassword ? '隐藏密码' : '查看密码'"
              class="absolute right-2 top-1/2 -translate-y-1/2 p-1 text-text-muted hover:text-text-primary transition-colors"
              :disabled="isHTTP && !enableAuth"
            >
              <!-- 睁眼图标 - 密码可见 -->
              <svg v-if="showPassword" xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24"/>
                <line x1="1" y1="1" x2="23" y2="23"/>
              </svg>
              <!-- 闭眼图标 - 密码隐藏 -->
              <svg v-else xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/>
                <circle cx="12" cy="12" r="3"/>
              </svg>
            </button>
          </div>
        </div>
        <!-- 根目录 - 占 1/2 -->
        <div class="space-y-1.5" :class="activeProtocol === 'tftp' ? 'w-3/4' : isHTTP ? 'flex-1' : 'w-1/2'">
          <label class="text-sm font-medium text-text-primary">根目录</label>
          <div class="relative flex gap-2">
            <input v-model="currentConfig.homeDir" type="text"
              class="flex-1 px-3 py-2 rounded-lg bg-bg-panel border border-border text-sm text-text-primary placeholder:text-text-muted focus:border-accent focus:outline-none focus:ring-1 focus:ring-accent/20 transition-all"
              :disabled="isRunning" placeholder="文件服务根目录" />
            <button
              @click="selectHomeDir"
              :disabled="isRunning"
              class="px-3 py-2 rounded-lg bg-bg-hover border border-border text-text-secondary hover:text-text-primary hover:border-accent/50 transition-all disabled:opacity-50 disabled:cursor-not-allowed"
              title="浏览目录"
            >
              <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/>
              </svg>
            </button>
          </div>
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

      <!-- HTTP 权限控制 -->
      <div v-if="isHTTP && currentConfig" class="flex gap-4 mb-4">
        <label class="flex items-center gap-2 text-sm text-text-secondary">
          <input v-model="currentConfig.allowGet" type="checkbox" class="w-4 h-4 rounded border-border accent-accent" :disabled="isRunning" />
          允许 GET（下载/浏览）
        </label>
        <label class="flex items-center gap-2 text-sm text-text-secondary">
          <input v-model="currentConfig.allowPut" type="checkbox" class="w-4 h-4 rounded border-border accent-accent" :disabled="isRunning" />
          允许 PUT/POST（上传）
        </label>
        <label class="flex items-center gap-2 text-sm text-text-secondary">
          <input v-model="currentConfig.allowDel" type="checkbox" class="w-4 h-4 rounded border-border accent-accent" :disabled="isRunning" />
          允许 DELETE（删除）
        </label>
      </div>

      <!-- 操作按钮和状态 -->
      <div class="flex items-center justify-between">
        <!-- 操作按钮 -->
        <div class="flex gap-3">
          <button @click="toggleServer"
            :class="[
              'px-5 py-2 rounded-lg text-sm font-semibold transition-all duration-200',
              isRunning
                ? 'bg-error/10 text-error border border-error/30 hover:bg-error/20'
                : 'bg-success/10 text-success border border-success/30 hover:bg-success/20'
            ]"
          >{{ isRunning ? '停止服务' : '启动服务' }}</button>
          <button @click="saveConfig" :disabled="isRunning"
            class="px-4 py-2 rounded-lg text-sm font-semibold bg-accent text-white border border-accent/30 hover:bg-accent/90 transition-all duration-200 disabled:opacity-50 disabled:cursor-not-allowed"
          >保存配置</button>
          <button @click="disconnectAll" :disabled="!isRunning"
            class="px-4 py-2 rounded-lg text-sm font-medium bg-bg-panel text-text-secondary border border-border hover:text-text-primary hover:border-accent/50 transition-all duration-200 disabled:opacity-50 disabled:cursor-not-allowed"
          >断开所有连接</button>
        </div>

        <!-- 状态指示 - 右上角 -->
        <div class="flex items-center gap-2">
          <span :class="['w-2 h-2 rounded-full', isRunning ? 'bg-success animate-pulse' : 'bg-text-muted']"></span>
          <span class="text-sm text-text-secondary">
            {{ isRunning ? '运行中' : '已停止' }} - 端口 {{ currentConfig?.port || 0 }}
          </span>
        </div>
      </div>
    </div>

    <!-- 日志面板 -->
    <div class="flex-1 bg-bg-card rounded-xl border border-border overflow-hidden flex flex-col shadow-card">
      <div class="flex items-center justify-between px-4 py-3 border-b border-border bg-bg-panel">
        <span class="text-sm font-semibold text-text-primary">服务日志</span>
        <div class="flex gap-2">
          <label class="flex items-center gap-2 text-xs text-text-secondary">
            <input v-model="autoScroll" type="checkbox" class="w-3 h-3 rounded border-border accent-accent" />
            自动滚动
          </label>
          <button @click="clearLogs" class="px-3 py-1 rounded-lg text-xs bg-bg-hover text-text-muted hover:text-text-primary transition-colors">清除日志</button>
        </div>
      </div>
      <div ref="logsContainer" class="flex-1 overflow-auto p-3 font-mono text-xs bg-terminal-bg text-terminal-text scrollbar-custom">
        <div v-if="logs.length === 0" class="text-center text-text-muted py-8">暂无日志记录</div>
        <div v-for="(log, index) in logs" :key="index" class="flex gap-2 py-1 hover:bg-bg-hover/30 px-2 rounded transition-colors">
          <span class="text-text-muted">{{ formatTime(log.timestamp) }}</span>
          <span :class="getLevelClass(log.level)">[{{ log.level.toUpperCase() }}]</span>
          <span class="text-accent">[{{ log.protocol.toUpperCase() }}]</span>
          <span v-if="log.clientIp" class="text-warning">{{ log.clientIp }}</span>
          <span :class="getActionClass(log.action)">{{ log.action }}</span>
          <span class="text-text-primary">{{ log.message }}</span>
          <span v-if="log.file" class="text-info">{{ log.file }}</span>
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