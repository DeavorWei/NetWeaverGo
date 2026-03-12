<template>
  <div class="runtime-config-panel">
    <h3 class="panel-title">运行时配置</h3>
    <p class="panel-description">修改这些配置将立即生效，无需重启应用</p>

    <!-- 超时配置 -->
    <section class="config-section">
      <h4 class="section-title">超时设置（毫秒）</h4>
      <div class="config-grid">
        <div class="config-group">
          <label>命令执行超时</label>
          <input
            v-model.number="config.timeouts.command"
            type="number"
            min="1000"
            step="1000"
            class="config-input"
          />
          <span class="config-hint">默认: 30秒</span>
        </div>
        <div class="config-group">
          <label>连接超时</label>
          <input
            v-model.number="config.timeouts.connection"
            type="number"
            min="1000"
            step="1000"
            class="config-input"
          />
          <span class="config-hint">默认: 10秒</span>
        </div>
        <div class="config-group">
          <label>握手超时</label>
          <input
            v-model.number="config.timeouts.handshake"
            type="number"
            min="1000"
            step="1000"
            class="config-input"
          />
          <span class="config-hint">默认: 10秒</span>
        </div>
        <div class="config-group">
          <label>短命令超时</label>
          <input
            v-model.number="config.timeouts.shortCmd"
            type="number"
            min="1000"
            step="1000"
            class="config-input"
          />
          <span class="config-hint">默认: 10秒</span>
        </div>
        <div class="config-group">
          <label>长命令超时</label>
          <input
            v-model.number="config.timeouts.longCmd"
            type="number"
            min="1000"
            step="1000"
            class="config-input"
          />
          <span class="config-hint">默认: 60秒</span>
        </div>
      </div>
    </section>

    <!-- 限制配置 -->
    <section class="config-section">
      <h4 class="section-title">限制设置</h4>
      <div class="config-grid">
        <div class="config-group">
          <label>每设备最大日志数</label>
          <input
            v-model.number="config.limits.maxLogsPerDevice"
            type="number"
            min="100"
            step="100"
            class="config-input"
          />
          <span class="config-hint">默认: 500</span>
        </div>
        <div class="config-group">
          <label>最大日志长度</label>
          <input
            v-model.number="config.limits.maxLogLength"
            type="number"
            min="1000"
            step="1000"
            class="config-input"
          />
          <span class="config-hint">默认: 2000字符</span>
        </div>
        <div class="config-group">
          <label>日志截断阈值</label>
          <input
            v-model.number="config.limits.logTruncateThreshold"
            type="number"
            min="50"
            max="100"
            step="5"
            class="config-input"
          />
          <span class="config-hint">默认: 95%</span>
        </div>
        <div class="config-group">
          <label>最大并发设备数</label>
          <input
            v-model.number="config.limits.maxConcurrentDevices"
            type="number"
            min="1"
            max="500"
            step="1"
            class="config-input"
          />
          <span class="config-hint">默认: 32</span>
        </div>
      </div>
    </section>

    <!-- 引擎配置 -->
    <section class="config-section">
      <h4 class="section-title">引擎设置</h4>
      <div class="config-grid">
        <div class="config-group">
          <label>工作协程数</label>
          <input
            v-model.number="config.engine.workerCount"
            type="number"
            min="1"
            max="100"
            step="1"
            class="config-input"
          />
          <span class="config-hint">默认: 10</span>
        </div>
        <div class="config-group">
          <label>事件缓冲区大小</label>
          <input
            v-model.number="config.engine.eventBufferSize"
            type="number"
            min="100"
            max="10000"
            step="100"
            class="config-input"
          />
          <span class="config-hint">默认: 1000</span>
        </div>
        <div class="config-group">
          <label>后备事件容量</label>
          <input
            v-model.number="config.engine.fallbackEventCapacity"
            type="number"
            min="100"
            max="2000"
            step="100"
            class="config-input"
          />
          <span class="config-hint">默认: 500</span>
        </div>
      </div>
    </section>

    <!-- 缓冲区配置 -->
    <section class="config-section">
      <h4 class="section-title">缓冲区设置（字节）</h4>
      <div class="config-grid">
        <div class="config-group">
          <label>默认缓冲区大小</label>
          <input
            v-model.number="config.buffers.defaultSize"
            type="number"
            min="1024"
            max="65536"
            step="1024"
            class="config-input"
          />
          <span class="config-hint">默认: 4096</span>
        </div>
        <div class="config-group">
          <label>小缓冲区大小</label>
          <input
            v-model.number="config.buffers.smallSize"
            type="number"
            min="512"
            max="32768"
            step="512"
            class="config-input"
          />
          <span class="config-hint">默认: 1024</span>
        </div>
        <div class="config-group">
          <label>大缓冲区大小</label>
          <input
            v-model.number="config.buffers.largeSize"
            type="number"
            min="2048"
            max="131072"
            step="2048"
            class="config-input"
          />
          <span class="config-hint">默认: 8192</span>
        </div>
      </div>
    </section>

    <!-- 分页检测配置 -->
    <section class="config-section">
      <h4 class="section-title">分页检测设置</h4>
      <div class="config-grid">
        <div class="config-group">
          <label>行数阈值</label>
          <input
            v-model.number="config.pagination.lineThreshold"
            type="number"
            min="10"
            max="200"
            step="10"
            class="config-input"
          />
          <span class="config-hint">默认: 50行</span>
        </div>
        <div class="config-group">
          <label>检测间隔（毫秒）</label>
          <input
            v-model.number="config.pagination.checkInterval"
            type="number"
            min="10"
            max="1000"
            step="10"
            class="config-input"
          />
          <span class="config-hint">默认: 100ms</span>
        </div>
      </div>
    </section>

    <div class="actions">
      <button 
        @click="saveConfig" 
        :disabled="saving || !hasChanges"
        class="btn btn-primary"
      >
        <span v-if="saving">保存中...</span>
        <span v-else>保存配置</span>
      </button>
      <button 
        @click="resetToDefault" 
        :disabled="saving"
        class="btn btn-secondary"
      >
        重置为默认值
      </button>
      <button 
        @click="reloadConfig" 
        :disabled="saving"
        class="btn btn-secondary"
      >
        刷新配置
      </button>
    </div>

    <!-- 状态提示 -->
    <div v-if="message" :class="['status-message', messageType]">
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
  padding: 20px;
  max-width: 1200px;
  margin: 0 auto;
}

.panel-title {
  font-size: 1.5rem;
  font-weight: 600;
  color: var(--text-primary);
  margin-bottom: 8px;
}

.panel-description {
  color: var(--text-secondary);
  font-size: 0.9rem;
  margin-bottom: 24px;
}

.config-section {
  background: var(--bg-secondary);
  border-radius: 12px;
  padding: 20px;
  margin-bottom: 20px;
  border: 1px solid var(--border-color);
}

.section-title {
  font-size: 1.1rem;
  font-weight: 600;
  color: var(--text-primary);
  margin-bottom: 16px;
  padding-bottom: 8px;
  border-bottom: 1px solid var(--border-color);
}

.config-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(250px, 1fr));
  gap: 16px;
}

.config-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.config-group label {
  font-size: 0.9rem;
  font-weight: 500;
  color: var(--text-primary);
}

.config-input {
  padding: 10px 12px;
  border: 1px solid var(--border-color);
  border-radius: 8px;
  background: var(--bg-primary);
  color: var(--text-primary);
  font-size: 0.95rem;
  transition: border-color 0.2s, box-shadow 0.2s;
}

.config-input:focus {
  outline: none;
  border-color: var(--accent-primary);
  box-shadow: 0 0 0 3px rgba(var(--accent-primary-rgb), 0.1);
}

.config-hint {
  font-size: 0.8rem;
  color: var(--text-tertiary);
}

.actions {
  display: flex;
  gap: 12px;
  margin-top: 24px;
  padding-top: 20px;
  border-top: 1px solid var(--border-color);
}

.btn {
  padding: 12px 24px;
  border-radius: 8px;
  font-size: 0.95rem;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s;
  border: none;
}

.btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.btn-primary {
  background: var(--accent-primary);
  color: white;
}

.btn-primary:hover:not(:disabled) {
  background: var(--accent-primary-hover);
  transform: translateY(-1px);
}

.btn-secondary {
  background: var(--bg-tertiary);
  color: var(--text-primary);
  border: 1px solid var(--border-color);
}

.btn-secondary:hover:not(:disabled) {
  background: var(--bg-hover);
}

.status-message {
  margin-top: 16px;
  padding: 12px 16px;
  border-radius: 8px;
  font-size: 0.95rem;
}

.status-message.success {
  background: rgba(34, 197, 94, 0.1);
  color: #16a34a;
  border: 1px solid rgba(34, 197, 94, 0.2);
}

.status-message.error {
  background: rgba(239, 68, 68, 0.1);
  color: #dc2626;
  border: 1px solid rgba(239, 68, 68, 0.2);
}
</style>
