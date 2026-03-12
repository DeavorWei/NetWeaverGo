<template>
  <div class="runtime-config-panel">

    <!-- 运行时配置标题 -->
    <div class="flex items-center gap-2 mb-4">
      <svg xmlns="http://www.w3.org/2000/svg" class="w-5 h-5 text-warning" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <path d="M13 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V9z"/>
        <polyline points="13 2 13 9 20 9"/>
      </svg>
      <h2 class="text-base font-semibold text-text-primary">运行时配置（热更新）</h2>
    </div>
    <p class="text-xs text-text-muted mb-5">修改这些配置将立即生效，无需重启应用。此配置独立于上方全局设置，需要单独保存。</p>

    <!-- 超时配置 -->
    <div class="bg-bg-card border border-border rounded-xl p-5 shadow-card mb-5">
      <h3 class="text-sm font-semibold text-text-secondary mb-4 flex items-center gap-2">
        <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4 text-warning" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/>
        </svg>
        超时设置（毫秒）
      </h3>
      <div class="grid grid-cols-2 gap-4">
        <div class="space-y-2">
          <label class="text-xs font-medium text-text-secondary">命令执行超时</label>
          <input
            v-model.number="config.timeouts.command"
            type="number"
            min="1000"
            step="1000"
            class="w-full px-3 py-2 bg-bg-panel border border-border rounded-lg text-sm text-text-primary focus:outline-none focus:border-warning focus:ring-1 focus:ring-warning/30 transition-all"
          />
          <p class="text-xs text-text-muted">默认: 30秒</p>
        </div>
        <div class="space-y-2">
          <label class="text-xs font-medium text-text-secondary">连接超时</label>
          <input
            v-model.number="config.timeouts.connection"
            type="number"
            min="1000"
            step="1000"
            class="w-full px-3 py-2 bg-bg-panel border border-border rounded-lg text-sm text-text-primary focus:outline-none focus:border-warning focus:ring-1 focus:ring-warning/30 transition-all"
          />
          <p class="text-xs text-text-muted">默认: 10秒</p>
        </div>
        <div class="space-y-2">
          <label class="text-xs font-medium text-text-secondary">握手超时</label>
          <input
            v-model.number="config.timeouts.handshake"
            type="number"
            min="1000"
            step="1000"
            class="w-full px-3 py-2 bg-bg-panel border border-border rounded-lg text-sm text-text-primary focus:outline-none focus:border-warning focus:ring-1 focus:ring-warning/30 transition-all"
          />
          <p class="text-xs text-text-muted">默认: 10秒</p>
        </div>
        <div class="space-y-2">
          <label class="text-xs font-medium text-text-secondary">短命令超时</label>
          <input
            v-model.number="config.timeouts.shortCmd"
            type="number"
            min="1000"
            step="1000"
            class="w-full px-3 py-2 bg-bg-panel border border-border rounded-lg text-sm text-text-primary focus:outline-none focus:border-warning focus:ring-1 focus:ring-warning/30 transition-all"
          />
          <p class="text-xs text-text-muted">默认: 10秒</p>
        </div>
        <div class="space-y-2">
          <label class="text-xs font-medium text-text-secondary">长命令超时</label>
          <input
            v-model.number="config.timeouts.longCmd"
            type="number"
            min="1000"
            step="1000"
            class="w-full px-3 py-2 bg-bg-panel border border-border rounded-lg text-sm text-text-primary focus:outline-none focus:border-warning focus:ring-1 focus:ring-warning/30 transition-all"
          />
          <p class="text-xs text-text-muted">默认: 60秒</p>
        </div>
      </div>
    </div>

    <!-- 限制配置 -->
    <div class="bg-bg-card border border-border rounded-xl p-5 shadow-card mb-5">
      <h3 class="text-sm font-semibold text-text-secondary mb-4 flex items-center gap-2">
        <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4 text-accent" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <rect x="3" y="3" width="18" height="18" rx="2" ry="2"/>
          <line x1="3" y1="9" x2="21" y2="9"/>
          <line x1="9" y1="21" x2="9" y2="9"/>
        </svg>
        限制设置
      </h3>
      <div class="grid grid-cols-2 gap-4">
        <div class="space-y-2">
          <label class="text-xs font-medium text-text-secondary">每设备最大日志数</label>
          <input
            v-model.number="config.limits.maxLogsPerDevice"
            type="number"
            min="100"
            step="100"
            class="w-full px-3 py-2 bg-bg-panel border border-border rounded-lg text-sm text-text-primary focus:outline-none focus:border-accent focus:ring-1 focus:ring-accent/30 transition-all"
          />
          <p class="text-xs text-text-muted">默认: 500</p>
        </div>
        <div class="space-y-2">
          <label class="text-xs font-medium text-text-secondary">最大日志长度</label>
          <input
            v-model.number="config.limits.maxLogLength"
            type="number"
            min="1000"
            step="1000"
            class="w-full px-3 py-2 bg-bg-panel border border-border rounded-lg text-sm text-text-primary focus:outline-none focus:border-accent focus:ring-1 focus:ring-accent/30 transition-all"
          />
          <p class="text-xs text-text-muted">默认: 2000字符</p>
        </div>
        <div class="space-y-2">
          <label class="text-xs font-medium text-text-secondary">日志截断阈值</label>
          <input
            v-model.number="config.limits.logTruncateThreshold"
            type="number"
            min="50"
            max="100"
            step="5"
            class="w-full px-3 py-2 bg-bg-panel border border-border rounded-lg text-sm text-text-primary focus:outline-none focus:border-accent focus:ring-1 focus:ring-accent/30 transition-all"
          />
          <p class="text-xs text-text-muted">默认: 95%</p>
        </div>
        <div class="space-y-2">
          <label class="text-xs font-medium text-text-secondary">最大并发设备数</label>
          <input
            v-model.number="config.limits.maxConcurrentDevices"
            type="number"
            min="1"
            max="500"
            step="1"
            class="w-full px-3 py-2 bg-bg-panel border border-border rounded-lg text-sm text-text-primary focus:outline-none focus:border-accent focus:ring-1 focus:ring-accent/30 transition-all"
          />
          <p class="text-xs text-text-muted">默认: 32</p>
        </div>
      </div>
    </div>

    <!-- 引擎配置 -->
    <div class="bg-bg-card border border-border rounded-xl p-5 shadow-card mb-5">
      <h3 class="text-sm font-semibold text-text-secondary mb-4 flex items-center gap-2">
        <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4 text-info" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"/>
        </svg>
        引擎设置
      </h3>
      <div class="grid grid-cols-2 gap-4">
        <div class="space-y-2">
          <label class="text-xs font-medium text-text-secondary">工作协程数</label>
          <input
            v-model.number="config.engine.workerCount"
            type="number"
            min="1"
            max="100"
            step="1"
            class="w-full px-3 py-2 bg-bg-panel border border-border rounded-lg text-sm text-text-primary focus:outline-none focus:border-info focus:ring-1 focus:ring-info/30 transition-all"
          />
          <p class="text-xs text-text-muted">默认: 10</p>
        </div>
        <div class="space-y-2">
          <label class="text-xs font-medium text-text-secondary">事件缓冲区大小</label>
          <input
            v-model.number="config.engine.eventBufferSize"
            type="number"
            min="100"
            max="10000"
            step="100"
            class="w-full px-3 py-2 bg-bg-panel border border-border rounded-lg text-sm text-text-primary focus:outline-none focus:border-info focus:ring-1 focus:ring-info/30 transition-all"
          />
          <p class="text-xs text-text-muted">默认: 1000</p>
        </div>
        <div class="space-y-2">
          <label class="text-xs font-medium text-text-secondary">后备事件容量</label>
          <input
            v-model.number="config.engine.fallbackEventCapacity"
            type="number"
            min="100"
            max="2000"
            step="100"
            class="w-full px-3 py-2 bg-bg-panel border border-border rounded-lg text-sm text-text-primary focus:outline-none focus:border-info focus:ring-1 focus:ring-info/30 transition-all"
          />
          <p class="text-xs text-text-muted">默认: 500</p>
        </div>
      </div>
    </div>

    <!-- 缓冲区配置 -->
    <div class="bg-bg-card border border-border rounded-xl p-5 shadow-card mb-5">
      <h3 class="text-sm font-semibold text-text-secondary mb-4 flex items-center gap-2">
        <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4 text-success" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"/>
          <polyline points="3.27 6.96 12 12.01 20.73 6.96"/>
          <line x1="12" y1="22.08" x2="12" y2="12"/>
        </svg>
        缓冲区设置（字节）
      </h3>
      <div class="grid grid-cols-2 gap-4">
        <div class="space-y-2">
          <label class="text-xs font-medium text-text-secondary">默认缓冲区大小</label>
          <input
            v-model.number="config.buffers.defaultSize"
            type="number"
            min="1024"
            max="65536"
            step="1024"
            class="w-full px-3 py-2 bg-bg-panel border border-border rounded-lg text-sm text-text-primary focus:outline-none focus:border-success focus:ring-1 focus:ring-success/30 transition-all"
          />
          <p class="text-xs text-text-muted">默认: 4096</p>
        </div>
        <div class="space-y-2">
          <label class="text-xs font-medium text-text-secondary">小缓冲区大小</label>
          <input
            v-model.number="config.buffers.smallSize"
            type="number"
            min="512"
            max="32768"
            step="512"
            class="w-full px-3 py-2 bg-bg-panel border border-border rounded-lg text-sm text-text-primary focus:outline-none focus:border-success focus:ring-1 focus:ring-success/30 transition-all"
          />
          <p class="text-xs text-text-muted">默认: 1024</p>
        </div>
        <div class="space-y-2">
          <label class="text-xs font-medium text-text-secondary">大缓冲区大小</label>
          <input
            v-model.number="config.buffers.largeSize"
            type="number"
            min="2048"
            max="131072"
            step="2048"
            class="w-full px-3 py-2 bg-bg-panel border border-border rounded-lg text-sm text-text-primary focus:outline-none focus:border-success focus:ring-1 focus:ring-success/30 transition-all"
          />
          <p class="text-xs text-text-muted">默认: 8192</p>
        </div>
      </div>
    </div>

    <!-- 分页检测配置 -->
    <div class="bg-bg-card border border-border rounded-xl p-5 shadow-card mb-5">
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
      <div class="grid grid-cols-2 gap-4">
        <div class="space-y-2">
          <label class="text-xs font-medium text-text-secondary">行数阈值</label>
          <input
            v-model.number="config.pagination.lineThreshold"
            type="number"
            min="10"
            max="200"
            step="10"
            class="w-full px-3 py-2 bg-bg-panel border border-border rounded-lg text-sm text-text-primary focus:outline-none focus:border-accent focus:ring-1 focus:ring-accent/30 transition-all"
          />
          <p class="text-xs text-text-muted">默认: 50行</p>
        </div>
        <div class="space-y-2">
          <label class="text-xs font-medium text-text-secondary">检测间隔（毫秒）</label>
          <input
            v-model.number="config.pagination.checkInterval"
            type="number"
            min="10"
            max="1000"
            step="10"
            class="w-full px-3 py-2 bg-bg-panel border border-border rounded-lg text-sm text-text-primary focus:outline-none focus:border-accent focus:ring-1 focus:ring-accent/30 transition-all"
          />
          <p class="text-xs text-text-muted">默认: 100ms</p>
        </div>
      </div>
    </div>

    <!-- 操作按钮 -->
    <div class="flex items-center justify-end gap-3 pt-2 pb-2">
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
    <div v-if="message" :class="['fixed bottom-6 right-6 px-4 py-3 rounded-lg shadow-lg text-sm font-medium animate-slide-up', messageType === 'success' ? 'bg-success text-white' : 'bg-error text-white']">
      {{ message }}
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
  max-width: 1200px;
}
</style>
