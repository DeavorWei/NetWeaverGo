<template>
  <div class="animate-slide-in space-y-6">
    <!-- 页面标题 -->
    <div>
      <p class="text-sm text-text-muted">管理应用程序全局运行参数</p>
    </div>

    <!-- 加载状态 -->
    <div v-if="loading" class="flex items-center justify-center py-12">
      <div class="w-8 h-8 border-2 border-accent border-t-transparent rounded-full animate-spin"></div>
      <span class="ml-3 text-text-muted">加载设置中...</span>
    </div>

    <!-- 设置表单 -->
    <div v-else class="space-y-6">
      <!-- 执行参数 -->
      <div class="bg-bg-card border border-border rounded-xl p-5 shadow-card">
        <h3 class="text-sm font-semibold text-text-secondary mb-4 flex items-center gap-2">
          <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4 text-accent" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"/>
          </svg>
          执行参数
        </h3>
        <div class="grid grid-cols-2 gap-4">
          <!-- 最大并发数 -->
          <div class="space-y-2">
            <label class="text-xs font-medium text-text-secondary">最大并发数</label>
            <input
              type="number"
              v-model.number="settings.MaxWorkers"
              min="1"
              max="256"
              class="w-full px-3 py-2 bg-bg-panel border border-border rounded-lg text-sm text-text-primary focus:outline-none focus:border-accent focus:ring-1 focus:ring-accent/30 transition-all"
            />
            <p class="text-xs text-text-muted">同时执行的协程数量 (1-256)</p>
          </div>

          <!-- 错误处理模式 -->
          <div class="space-y-2">
            <label class="text-xs font-medium text-text-secondary">错误处理模式</label>
            <select
              v-model="settings.ErrorMode"
              class="w-full px-3 py-2 bg-bg-panel border border-border rounded-lg text-sm text-text-primary focus:outline-none focus:border-accent focus:ring-1 focus:ring-accent/30 transition-all"
            >
              <option value="pause">挂起询问</option>
              <option value="skip">跳过继续</option>
              <option value="abort">终止执行</option>
            </select>
            <p class="text-xs text-text-muted">命令执行出错时的处理策略</p>
          </div>
        </div>
      </div>

      <!-- 超时设置 -->
      <div class="bg-bg-card border border-border rounded-xl p-5 shadow-card">
        <h3 class="text-sm font-semibold text-text-secondary mb-4 flex items-center gap-2">
          <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4 text-warning" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/>
          </svg>
          超时设置
        </h3>
        <div class="grid grid-cols-2 gap-4">
          <!-- 连接超时 -->
          <div class="space-y-2">
            <label class="text-xs font-medium text-text-secondary">连接超时</label>
            <input
              type="text"
              v-model="settings.ConnectTimeout"
              placeholder="如: 10s"
              class="w-full px-3 py-2 bg-bg-panel border border-border rounded-lg text-sm text-text-primary focus:outline-none focus:border-accent focus:ring-1 focus:ring-accent/30 transition-all"
            />
            <p class="text-xs text-text-muted">SSH/SFTP 连接阶段超时时间</p>
          </div>

          <!-- 命令超时 -->
          <div class="space-y-2">
            <label class="text-xs font-medium text-text-secondary">命令超时</label>
            <input
              type="text"
              v-model="settings.CommandTimeout"
              placeholder="如: 30s"
              class="w-full px-3 py-2 bg-bg-panel border border-border rounded-lg text-sm text-text-primary focus:outline-none focus:border-accent focus:ring-1 focus:ring-accent/30 transition-all"
            />
            <p class="text-xs text-text-muted">单条命令执行超时时间</p>
          </div>
        </div>
      </div>

      <!-- 存储路径 -->
      <div class="bg-bg-card border border-border rounded-xl p-5 shadow-card">
        <h3 class="text-sm font-semibold text-text-secondary mb-4 flex items-center gap-2">
          <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4 text-success" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/>
          </svg>
          存储路径
        </h3>
        <div class="grid grid-cols-2 gap-4">
          <!-- 输出目录 -->
          <div class="space-y-2">
            <label class="text-xs font-medium text-text-secondary">输出目录</label>
            <input
              type="text"
              v-model="settings.OutputDir"
              placeholder="如: output"
              class="w-full px-3 py-2 bg-bg-panel border border-border rounded-lg text-sm text-text-primary focus:outline-none focus:border-accent focus:ring-1 focus:ring-accent/30 transition-all"
            />
            <p class="text-xs text-text-muted">回显输出与配置备份的存放目录</p>
          </div>

          <!-- 日志目录 -->
          <div class="space-y-2">
            <label class="text-xs font-medium text-text-secondary">日志目录</label>
            <input
              type="text"
              v-model="settings.LogDir"
              placeholder="如: logs"
              class="w-full px-3 py-2 bg-bg-panel border border-border rounded-lg text-sm text-text-primary focus:outline-none focus:border-accent focus:ring-1 focus:ring-accent/30 transition-all"
            />
            <p class="text-xs text-text-muted">系统运行日志存放目录</p>
          </div>
        </div>
      </div>

      <!-- 操作按钮 -->
      <div class="flex items-center justify-end gap-3 pt-2">
        <button
          @click="resetSettings"
          :disabled="saving"
          class="px-4 py-2 text-sm font-medium text-text-secondary bg-bg-panel border border-border rounded-lg hover:bg-bg-hover hover:text-text-primary disabled:opacity-50 disabled:cursor-not-allowed transition-all duration-200"
        >
          重置默认
        </button>
        <button
          @click="saveSettings"
          :disabled="saving"
          class="px-4 py-2 text-sm font-medium text-white bg-accent rounded-lg hover:bg-accent/90 disabled:opacity-50 disabled:cursor-not-allowed transition-all duration-200 flex items-center gap-2"
        >
          <svg v-if="saving" class="w-4 h-4 animate-spin" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <circle cx="12" cy="12" r="10" stroke-opacity="0.25"/>
            <path d="M12 2a10 10 0 0 1 10 10" stroke-linecap="round"/>
          </svg>
          {{ saving ? '保存中...' : '保存设置' }}
        </button>
      </div>
    </div>

    <!-- Toast 提示 -->
    <div
      v-if="toast.show"
      class="fixed bottom-6 right-6 px-4 py-3 rounded-lg shadow-lg text-sm font-medium animate-slide-up"
      :class="toast.type === 'success' ? 'bg-success text-white' : 'bg-error text-white'"
    >
      {{ toast.message }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Call } from '@wailsio/runtime'

interface GlobalSettings {
  MaxWorkers: number
  ConnectTimeout: string
  CommandTimeout: string
  OutputDir: string
  LogDir: string
  ErrorMode: string
}

const loading = ref(true)
const saving = ref(false)

const settings = ref<GlobalSettings>({
  MaxWorkers: 32,
  ConnectTimeout: '10s',
  CommandTimeout: '30s',
  OutputDir: 'output',
  LogDir: 'logs',
  ErrorMode: 'pause'
})

const defaultSettings: GlobalSettings = {
  MaxWorkers: 32,
  ConnectTimeout: '10s',
  CommandTimeout: '30s',
  OutputDir: 'output',
  LogDir: 'logs',
  ErrorMode: 'pause'
}

const toast = ref({
  show: false,
  message: '',
  type: 'success' as 'success' | 'error'
})

function showToast(message: string, type: 'success' | 'error' = 'success') {
  toast.value = { show: true, message, type }
  setTimeout(() => {
    toast.value.show = false
  }, 3000)
}

async function loadSettings() {
  try {
    loading.value = true
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const result: any = await Call.ByName(
      'github.com/NetWeaverGo/core/internal/ui.AppService.LoadSettings'
    )
    if (result) {
      settings.value = {
        MaxWorkers: result.MaxWorkers || result.max_workers || 32,
        ConnectTimeout: result.ConnectTimeout || result.connect_timeout || '10s',
        CommandTimeout: result.CommandTimeout || result.command_timeout || '30s',
        OutputDir: result.OutputDir || result.output_dir || 'output',
        LogDir: result.LogDir || result.log_dir || 'logs',
        ErrorMode: result.ErrorMode || result.error_mode || 'pause'
      }
    }
  } catch (err) {
    console.error('Failed to load settings:', err)
    showToast('加载设置失败', 'error')
  } finally {
    loading.value = false
  }
}

async function saveSettings() {
  try {
    saving.value = true
    await Call.ByName(
      'github.com/NetWeaverGo/core/internal/ui.AppService.SaveSettings',
      settings.value
    )
    showToast('设置已保存')
  } catch (err) {
    console.error('Failed to save settings:', err)
    showToast('保存设置失败', 'error')
  } finally {
    saving.value = false
  }
}

function resetSettings() {
  settings.value = { ...defaultSettings }
  showToast('已重置为默认设置')
}

onMounted(() => {
  loadSettings()
})
</script>
