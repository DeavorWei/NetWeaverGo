<template>
  <div class="runtime-config-panel runtime-shell">

    <!-- 运行时配置标题 -->
    <div class="runtime-header">
      <svg xmlns="http://www.w3.org/2000/svg" class="w-5 h-5 text-warning" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <path d="M13 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V9z"/>
        <polyline points="13 2 13 9 20 9"/>
      </svg>
      <h2 class="runtime-title">运行时配置（热更新）</h2>
    </div>
    <p class="runtime-subtitle">修改这些配置将立即生效，无需重启应用。引擎工作协程数与事件缓冲会直接驱动执行链；上方全局最大并发数仅作为兼容回退值。</p>

    <div class="runtime-settings-panels-flow">
    <!-- 超时配置 -->
    <div class="runtime-card bg-bg-card border border-border rounded-xl p-5 shadow-card settings-panel-card">
      <h3 class="text-sm font-semibold text-text-secondary mb-4 flex items-center gap-2">
        <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4 text-warning" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/>
        </svg>
        超时设置（毫秒）
      </h3>
      <div class="settings-auto-grid">
        <div class="space-y-2">
          <label class="settings-label">命令执行超时 <HelpTip text="单条命令执行最长等待时间，默认 30000 毫秒。" /></label>
          <input
            v-model.number="config.timeouts.command"
            type="number"
            min="1000"
            step="1000"
            class="w-full px-3 py-2 bg-bg-panel border border-border rounded-lg text-sm text-text-primary focus:outline-none focus:border-warning focus:ring-1 focus:ring-warning/30 transition-all"
          />
        </div>
        <div class="space-y-2">
          <label class="settings-label">连接超时 <HelpTip text="建立 SSH/SFTP 网络连接的最长等待时间，默认 10000 毫秒。" /></label>
          <input
            v-model.number="config.timeouts.connection"
            type="number"
            min="1000"
            step="1000"
            class="w-full px-3 py-2 bg-bg-panel border border-border rounded-lg text-sm text-text-primary focus:outline-none focus:border-warning focus:ring-1 focus:ring-warning/30 transition-all"
          />
        </div>
        <div class="space-y-2">
          <label class="settings-label">握手超时 <HelpTip text="SSH 协议握手阶段的最长等待时间，默认 10000 毫秒。" /></label>
          <input
            v-model.number="config.timeouts.handshake"
            type="number"
            min="1000"
            step="1000"
            class="w-full px-3 py-2 bg-bg-panel border border-border rounded-lg text-sm text-text-primary focus:outline-none focus:border-warning focus:ring-1 focus:ring-warning/30 transition-all"
          />
        </div>
        <div class="space-y-2">
          <label class="settings-label">短命令超时 <HelpTip text="用于快速命令的超时阈值，默认 10000 毫秒。" /></label>
          <input
            v-model.number="config.timeouts.shortCmd"
            type="number"
            min="1000"
            step="1000"
            class="w-full px-3 py-2 bg-bg-panel border border-border rounded-lg text-sm text-text-primary focus:outline-none focus:border-warning focus:ring-1 focus:ring-warning/30 transition-all"
          />
        </div>
        <div class="space-y-2">
          <label class="settings-label">长命令超时 <HelpTip text="用于大输出或慢设备命令的超时阈值，默认 60000 毫秒。" /></label>
          <input
            v-model.number="config.timeouts.longCmd"
            type="number"
            min="1000"
            step="1000"
            class="w-full px-3 py-2 bg-bg-panel border border-border rounded-lg text-sm text-text-primary focus:outline-none focus:border-warning focus:ring-1 focus:ring-warning/30 transition-all"
          />
        </div>
      </div>
    </div>

    <!-- 限制配置 -->
    <div class="runtime-card bg-bg-card border border-border rounded-xl p-5 shadow-card settings-panel-card">
      <h3 class="text-sm font-semibold text-text-secondary mb-4 flex items-center gap-2">
        <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4 text-accent" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <rect x="3" y="3" width="18" height="18" rx="2" ry="2"/>
          <line x1="3" y1="9" x2="21" y2="9"/>
          <line x1="9" y1="21" x2="9" y2="9"/>
        </svg>
        限制设置
      </h3>
      <div class="settings-auto-grid">
        <div class="space-y-2">
          <label class="settings-label">每设备最大日志数 <HelpTip text="单台设备缓存的最大日志条目数，超过后会按策略截断，默认 500。" /></label>
          <input
            v-model.number="config.limits.maxLogsPerDevice"
            type="number"
            min="100"
            step="100"
            class="w-full px-3 py-2 bg-bg-panel border border-border rounded-lg text-sm text-text-primary focus:outline-none focus:border-accent focus:ring-1 focus:ring-accent/30 transition-all"
          />
        </div>
        <div class="space-y-2">
          <label class="settings-label">最大日志长度 <HelpTip text="单条日志保留的最大字符数，默认 2000。" /></label>
          <input
            v-model.number="config.limits.maxLogLength"
            type="number"
            min="1000"
            step="1000"
            class="w-full px-3 py-2 bg-bg-panel border border-border rounded-lg text-sm text-text-primary focus:outline-none focus:border-accent focus:ring-1 focus:ring-accent/30 transition-all"
          />
        </div>
        <div class="space-y-2">
          <label class="settings-label">日志截断阈值 <HelpTip text="当日志接近上限时触发截断的百分比阈值，默认 95% 。" /></label>
          <input
            v-model.number="config.limits.logTruncateThreshold"
            type="number"
            min="50"
            max="100"
            step="5"
            class="w-full px-3 py-2 bg-bg-panel border border-border rounded-lg text-sm text-text-primary focus:outline-none focus:border-accent focus:ring-1 focus:ring-accent/30 transition-all"
          />
        </div>
        <div class="space-y-2">
          <label class="settings-label">最大并发设备数 <HelpTip text="执行任务时同时处理的设备数量上限，默认 32。" /></label>
          <input
            v-model.number="config.limits.maxConcurrentDevices"
            type="number"
            min="1"
            max="500"
            step="1"
            class="w-full px-3 py-2 bg-bg-panel border border-border rounded-lg text-sm text-text-primary focus:outline-none focus:border-accent focus:ring-1 focus:ring-accent/30 transition-all"
          />
        </div>
      </div>
    </div>

    <!-- 引擎配置 -->
    <div class="runtime-card bg-bg-card border border-border rounded-xl p-5 shadow-card settings-panel-card">
      <h3 class="text-sm font-semibold text-text-secondary mb-4 flex items-center gap-2">
        <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4 text-info" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"/>
        </svg>
        引擎设置
      </h3>
      <div class="settings-auto-grid">
        <div class="space-y-2">
          <label class="settings-label">工作协程数 <HelpTip text="引擎用于并发执行的协程数量，默认 10。" /></label>
          <input
            v-model.number="config.engine.workerCount"
            type="number"
            min="1"
            max="100"
            step="1"
            class="w-full px-3 py-2 bg-bg-panel border border-border rounded-lg text-sm text-text-primary focus:outline-none focus:border-info focus:ring-1 focus:ring-info/30 transition-all"
          />
        </div>
        <div class="space-y-2">
          <label class="settings-label">事件缓冲区大小 <HelpTip text="主事件队列容量，过小会增加阻塞风险，默认 1000。" /></label>
          <input
            v-model.number="config.engine.eventBufferSize"
            type="number"
            min="100"
            max="10000"
            step="100"
            class="w-full px-3 py-2 bg-bg-panel border border-border rounded-lg text-sm text-text-primary focus:outline-none focus:border-info focus:ring-1 focus:ring-info/30 transition-all"
          />
        </div>
        <div class="space-y-2">
          <label class="settings-label">后备事件容量 <HelpTip text="主缓冲耗尽时的兜底事件容量，默认 500。" /></label>
          <input
            v-model.number="config.engine.fallbackEventCapacity"
            type="number"
            min="100"
            max="2000"
            step="100"
            class="w-full px-3 py-2 bg-bg-panel border border-border rounded-lg text-sm text-text-primary focus:outline-none focus:border-info focus:ring-1 focus:ring-info/30 transition-all"
          />
        </div>
      </div>
    </div>

    <!-- 缓冲区配置 -->
    <div class="runtime-card bg-bg-card border border-border rounded-xl p-5 shadow-card settings-panel-card">
      <h3 class="text-sm font-semibold text-text-secondary mb-4 flex items-center gap-2">
        <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4 text-success" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"/>
          <polyline points="3.27 6.96 12 12.01 20.73 6.96"/>
          <line x1="12" y1="22.08" x2="12" y2="12"/>
        </svg>
        缓冲区设置（字节）
      </h3>
      <div class="settings-auto-grid">
        <div class="space-y-2">
          <label class="settings-label">默认缓冲区大小 <HelpTip text="通用场景的基础缓冲区大小，默认 4096 字节。" /></label>
          <input
            v-model.number="config.buffers.defaultSize"
            type="number"
            min="1024"
            max="65536"
            step="1024"
            class="w-full px-3 py-2 bg-bg-panel border border-border rounded-lg text-sm text-text-primary focus:outline-none focus:border-success focus:ring-1 focus:ring-success/30 transition-all"
          />
        </div>
        <div class="space-y-2">
          <label class="settings-label">小缓冲区大小 <HelpTip text="轻量任务使用的小缓冲区大小，默认 1024 字节。" /></label>
          <input
            v-model.number="config.buffers.smallSize"
            type="number"
            min="512"
            max="32768"
            step="512"
            class="w-full px-3 py-2 bg-bg-panel border border-border rounded-lg text-sm text-text-primary focus:outline-none focus:border-success focus:ring-1 focus:ring-success/30 transition-all"
          />
        </div>
        <div class="space-y-2">
          <label class="settings-label">大缓冲区大小 <HelpTip text="大数据量输出场景使用的大缓冲区，默认 8192 字节。" /></label>
          <input
            v-model.number="config.buffers.largeSize"
            type="number"
            min="2048"
            max="131072"
            step="2048"
            class="w-full px-3 py-2 bg-bg-panel border border-border rounded-lg text-sm text-text-primary focus:outline-none focus:border-success focus:ring-1 focus:ring-success/30 transition-all"
          />
        </div>
      </div>
    </div>

    <!-- 分页检测配置 -->
    <div class="runtime-card bg-bg-card border border-border rounded-xl p-5 shadow-card settings-panel-card">
      <h3 class="text-sm font-semibold text-text-secondary mb-4 flex items-center gap-2">
        <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4 text-text-secondary" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <line x1="8" y1="6" x2="21" y2="6"/>
          <line x1="8" y1="12" x2="21" y2="12"/>
          <line x1="8" y1="18" x2="21" y2="18"/>
          <line x1="3" y1="6" x2="3.01" y2="6"/>
          <line x1="3" y1="12" x2="3.01" y2="12"/>
          <line x1="3" y1="18" x2="3.01" y2="18"/>
        </svg>
        分页检测设置
      </h3>
      <div class="settings-auto-grid">
        <div class="space-y-2">
          <label class="settings-label">行数阈值 <HelpTip text="输出行数超过该阈值后触发分页检测，默认 50 行。" /></label>
          <input
            v-model.number="config.pagination.lineThreshold"
            type="number"
            min="10"
            max="200"
            step="10"
            class="w-full px-3 py-2 bg-bg-panel border border-border rounded-lg text-sm text-text-primary focus:outline-none focus:border-accent focus:ring-1 focus:ring-accent/30 transition-all"
          />
        </div>
        <div class="space-y-2">
          <label class="settings-label">检测间隔（毫秒） <HelpTip text="分页检测轮询间隔，越小响应越快但开销更高，默认 100ms。" /></label>
          <input
            v-model.number="config.pagination.checkInterval"
            type="number"
            min="10"
            max="1000"
            step="10"
            class="w-full px-3 py-2 bg-bg-panel border border-border rounded-lg text-sm text-text-primary focus:outline-none focus:border-accent focus:ring-1 focus:ring-accent/30 transition-all"
          />
        </div>
      </div>
    </div>
    </div>

    <!-- 操作按钮 -->
    <div class="runtime-actions">
      <button 
        @click="reloadConfig" 
        :disabled="saving"
        class="px-4 py-2 text-sm font-medium text-text-secondary bg-bg-panel border border-border rounded-lg hover:bg-bg-hover hover:text-text-primary disabled:opacity-50 disabled:cursor-not-allowed transition-all duration-200"
      >
        刷新配置
      </button>
      <button 
        @click="resetToDefault" 
        :disabled="saving"
        class="px-4 py-2 text-sm font-medium text-text-secondary bg-bg-panel border border-border rounded-lg hover:bg-bg-hover hover:text-text-primary disabled:opacity-50 disabled:cursor-not-allowed transition-all duration-200"
      >
        重置为默认值
      </button>
      <button 
        @click="saveConfig" 
        :disabled="saving || !hasChanges"
        class="px-4 py-2 text-sm font-medium text-white bg-warning rounded-lg hover:bg-warning/90 disabled:opacity-50 disabled:cursor-not-allowed transition-all duration-200 flex items-center gap-2"
      >
        <svg v-if="saving" class="w-4 h-4 animate-spin" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <circle cx="12" cy="12" r="10" stroke-opacity="0.25"/>
          <path d="M12 2a10 10 0 0 1 10 10" stroke-linecap="round"/>
        </svg>
        <span v-if="saving">保存中...</span>
        <span v-else>保存配置</span>
      </button>
    </div>

    <!-- 状态提示 -->
    <div v-if="message" class="fixed top-20 left-1/2 -translate-x-1/2 z-50 animate-slide-up">
      <div
        class="flex items-center gap-2 px-5 py-3 rounded-xl shadow-2xl border transition-all"
        :class="messageType === 'success' ? 'bg-success/10 border-success/30 text-success' : 'bg-error/10 border-error/30 text-error'"
      >
        <svg v-if="messageType === 'success'" class="w-5 h-5 flex-shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <polyline points="20 6 9 17 4 12"/>
        </svg>
        <svg v-else class="w-5 h-5 flex-shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <circle cx="12" cy="12" r="10"/>
          <line x1="15" y1="9" x2="9" y2="15"/>
          <line x1="9" y1="9" x2="15" y2="15"/>
        </svg>
        <span class="text-sm font-medium">{{ message }}</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import {
  GetRuntimeConfig,
  UpdateRuntimeConfig,
  ResetRuntimeConfigToDefault
} from '../../bindings/github.com/NetWeaverGo/core/internal/ui/settingsservice'
import { RuntimeConfigData } from '../../bindings/github.com/NetWeaverGo/core/internal/ui/models'
import HelpTip from '../common/HelpTip.vue'

// 配置数据
const config = ref({
  timeouts: {
    command: 30000,
    connection: 10000,
    handshake: 10000,
    shortCmd: 10000,
    longCmd: 60000
  },
  limits: {
    maxLogsPerDevice: 500,
    maxLogLength: 2000,
    logTruncateThreshold: 95,
    maxConcurrentDevices: 32
  },
  engine: {
    workerCount: 10,
    eventBufferSize: 1000,
    fallbackEventCapacity: 500
  },
  buffers: {
    defaultSize: 4096,
    smallSize: 1024,
    largeSize: 8192
  },
  pagination: {
    lineThreshold: 50,
    checkInterval: 100
  }
})

// 原始配置（用于比较变化）
const originalConfig = ref('')
const saving = ref(false)
const message = ref('')
const messageType = ref<'success' | 'error'>('success')

// 检查是否有变化
const hasChanges = computed(() => {
  return JSON.stringify(config.value) !== originalConfig.value
})

// 显示提示消息
function showMessage(msg: string, type: 'success' | 'error' = 'success') {
  message.value = msg
  messageType.value = type
  setTimeout(() => {
    message.value = ''
  }, 3000)
}

// 加载配置
async function loadConfig() {
  try {
    const data = await GetRuntimeConfig()
    config.value = {
      timeouts: {
        command: data.timeouts?.command || 30000,
        connection: data.timeouts?.connection || 10000,
        handshake: data.timeouts?.handshake || 10000,
        shortCmd: data.timeouts?.shortCmd || 10000,
        longCmd: data.timeouts?.longCmd || 60000
      },
      limits: {
        maxLogsPerDevice: data.limits?.maxLogsPerDevice || 500,
        maxLogLength: data.limits?.maxLogLength || 2000,
        logTruncateThreshold: data.limits?.logTruncateThreshold || 95,
        maxConcurrentDevices: data.limits?.maxConcurrentDevices || 32
      },
      engine: {
        workerCount: data.engine?.workerCount || 10,
        eventBufferSize: data.engine?.eventBufferSize || 1000,
        fallbackEventCapacity: data.engine?.fallbackEventCapacity || 500
      },
      buffers: {
        defaultSize: data.buffers?.defaultSize || 4096,
        smallSize: data.buffers?.smallSize || 1024,
        largeSize: data.buffers?.largeSize || 8192
      },
      pagination: {
        lineThreshold: data.pagination?.lineThreshold || 50,
        checkInterval: data.pagination?.checkInterval || 100
      }
    }
    originalConfig.value = JSON.stringify(config.value)
  } catch (err) {
    console.error('加载配置失败:', err)
    showMessage('加载配置失败', 'error')
  }
}

// 保存配置
async function saveConfig() {
  saving.value = true
  try {
    const data = new RuntimeConfigData({
      timeouts: {
        command: config.value.timeouts.command,
        connection: config.value.timeouts.connection,
        handshake: config.value.timeouts.handshake,
        shortCmd: config.value.timeouts.shortCmd,
        longCmd: config.value.timeouts.longCmd
      },
      limits: {
        maxLogsPerDevice: config.value.limits.maxLogsPerDevice,
        maxLogLength: config.value.limits.maxLogLength,
        logTruncateThreshold: config.value.limits.logTruncateThreshold,
        maxConcurrentDevices: config.value.limits.maxConcurrentDevices
      },
      engine: {
        workerCount: config.value.engine.workerCount,
        eventBufferSize: config.value.engine.eventBufferSize,
        fallbackEventCapacity: config.value.engine.fallbackEventCapacity
      },
      buffers: {
        defaultSize: config.value.buffers.defaultSize,
        smallSize: config.value.buffers.smallSize,
        largeSize: config.value.buffers.largeSize
      },
      pagination: {
        lineThreshold: config.value.pagination.lineThreshold,
        checkInterval: config.value.pagination.checkInterval
      }
    })
    await UpdateRuntimeConfig(data)
    originalConfig.value = JSON.stringify(config.value)
    showMessage('配置保存成功，已立即生效')
  } catch (err) {
    console.error('保存配置失败:', err)
    showMessage('保存配置失败', 'error')
  } finally {
    saving.value = false
  }
}

// 重置为默认值
async function resetToDefault() {
  if (!confirm('确定要重置为默认值吗？所有自定义配置将丢失。')) {
    return
  }
  saving.value = true
  try {
    await ResetRuntimeConfigToDefault()
    await loadConfig()
    showMessage('已重置为默认值')
  } catch (err) {
    console.error('重置配置失败:', err)
    showMessage('重置配置失败', 'error')
  } finally {
    saving.value = false
  }
}

// 刷新配置
async function reloadConfig() {
  await loadConfig()
  showMessage('配置已刷新')
}

onMounted(() => {
  loadConfig()
})
</script>

<style scoped>
.runtime-config-panel {
  width: 100%;
  max-width: none;
}

.runtime-shell {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  max-width: 1400px;
  margin: 0 auto;
}

.runtime-header {
  display: flex;
  align-items: center;
  gap: 0.55rem;
}

.runtime-title {
  font-size: 1rem;
  font-weight: 700;
  letter-spacing: -0.01em;
  color: var(--color-text-primary);
}

.runtime-subtitle {
  font-size: 0.78rem;
  color: var(--color-text-muted);
  margin: 0;
}

.runtime-settings-panels-flow {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(330px, 1fr));
  gap: 1rem;
  margin-bottom: 0.5rem;
}

.settings-auto-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 0.85rem 1rem;
  align-items: start;
}

.settings-auto-grid > * {
  min-width: 0;
}

.settings-panel-card {
  width: 100%;
}

.runtime-card {
  overflow: visible;
  border-radius: 1rem;
  border: 1px solid var(--color-border-default);
  background: var(--color-bg-secondary);
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.06);
  transition: box-shadow 0.2s ease, border-color 0.2s ease;
}

.runtime-card:hover {
  box-shadow: 0 6px 16px rgba(0, 0, 0, 0.1);
  border-color: var(--color-border-focus);
}

.settings-panel-card :is(input:not([type="checkbox"]):not([type="radio"]), select) {
  width: 100%;
  min-height: 2.35rem;
  border-radius: 0.75rem;
  border: 1px solid var(--color-border-default);
  background: var(--color-bg-primary);
  padding-inline: 0.8rem;
  transition: border-color 0.2s ease, box-shadow 0.2s ease, background-color 0.2s ease;
}

.settings-panel-card :is(input:not([type="checkbox"]):not([type="radio"]), select):focus {
  border-color: var(--color-accent);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--color-accent) 18%, transparent);
}

.settings-panel-card :is(label) {
  font-size: 0.76rem;
  font-weight: 600;
  color: var(--color-text-secondary);
}

.settings-label {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  font-size: 0.76rem;
  font-weight: 600;
  color: var(--color-text-secondary);
}

.settings-panel-card :is(p.text-xs) {
  line-height: 1.45;
}

.runtime-actions {
  position: sticky;
  bottom: 0;
  z-index: 5;
  display: flex;
  justify-content: flex-end;
  gap: 0.75rem;
  padding: 0.75rem;
  border: 1px solid var(--color-border-default);
  border-radius: 1rem;
  background: color-mix(in srgb, var(--color-bg-primary) 84%, transparent);
  backdrop-filter: blur(8px);
}

@media (max-width: 960px) {
  .runtime-settings-panels-flow {
    grid-template-columns: 1fr;
  }

  .runtime-actions {
    position: static;
    flex-wrap: wrap;
    justify-content: flex-start;
  }
}
</style>
