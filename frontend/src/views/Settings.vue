<template>
  <div class="settings-page animate-slide-in">
    <!-- 页面标题 -->
    <div class="settings-page-header">
      <div class="space-y-1">
        <h2 class="settings-page-title">系统设置</h2>
        <p class="settings-page-subtitle">管理应用全局参数、日志策略与 SSH 安全策略</p>
      </div>
      <div class="settings-page-badge">Global Preferences</div>
    </div>

    <!-- 加载状态 -->
    <div v-if="loading" class="settings-loading">
      <div class="w-8 h-8 border-2 border-accent border-t-transparent rounded-full animate-spin"></div>
      <span class="ml-3 text-text-muted">加载设置中...</span>
    </div>

    <!-- 设置表单 -->
    <div v-else class="settings-content">
      <div class="global-settings-panels-flow">
      <!-- 执行参数 -->
      <div class="settings-card bg-bg-card border border-border rounded-xl p-5 shadow-card settings-panel-card">
        <h3 class="text-sm font-semibold text-text-secondary mb-4 flex items-center gap-2">
          <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4 text-accent" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"/>
          </svg>
          执行参数
        </h3>
        <div class="settings-auto-grid">
          <!-- 最大并发数 -->
          <div class="space-y-2">
            <label class="settings-label">最大并发数 <HelpTip text="控制全局并发上限。若运行时配置设置了工作协程数，执行链会优先采用运行时配置。" /></label>
          <input
            type="number"
            v-model.number="settings.maxWorkers"
            min="1"
            max="256"
            class="w-full px-3 py-2 bg-bg-panel border border-border rounded-lg text-sm text-text-primary focus:outline-none focus:border-accent focus:ring-1 focus:ring-accent/30 transition-all"
          />
          </div>

          <!-- 错误处理模式 -->
          <div class="space-y-2">
            <label class="settings-label">错误处理模式 <HelpTip text="命令执行出错时的处理策略：挂起询问、跳过继续或终止执行。" /></label>
          <select
            v-model="settings.errorMode"
            class="w-full px-3 py-2 bg-bg-panel border border-border rounded-lg text-sm text-text-primary focus:outline-none focus:border-accent focus:ring-1 focus:ring-accent/30 transition-all"
          >
              <option value="pause">挂起询问</option>
              <option value="skip">跳过继续</option>
              <option value="abort">终止执行</option>
            </select>
          </div>
        </div>
      </div>

      <!-- 超时设置 -->
      <div class="settings-card bg-bg-card border border-border rounded-xl p-5 shadow-card settings-panel-card">
        <h3 class="text-sm font-semibold text-text-secondary mb-4 flex items-center gap-2">
          <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4 text-warning" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/>
          </svg>
          超时设置
        </h3>
        <div class="settings-auto-grid">
          <!-- 连接超时 -->
          <div class="space-y-2">
            <label class="settings-label">连接超时 <HelpTip text="SSH/SFTP 连接阶段允许等待的最长时间，例如 10s。" /></label>
          <input
            type="text"
            v-model="settings.connectTimeout"
            placeholder="如: 10s"
            class="w-full px-3 py-2 bg-bg-panel border border-border rounded-lg text-sm text-text-primary focus:outline-none focus:border-accent focus:ring-1 focus:ring-accent/30 transition-all"
          />
          </div>

          <!-- 命令超时 -->
          <div class="space-y-2">
            <label class="settings-label">命令超时 <HelpTip text="单条命令执行允许持续的最长时间，例如 30s。" /></label>
          <input
            type="text"
            v-model="settings.commandTimeout"
            placeholder="如: 30s"
            class="w-full px-3 py-2 bg-bg-panel border border-border rounded-lg text-sm text-text-primary focus:outline-none focus:border-accent focus:ring-1 focus:ring-accent/30 transition-all"
          />
          </div>
        </div>
      </div>

      <!-- 存储路径 -->
      <div class="settings-card bg-bg-card border border-border rounded-xl p-5 shadow-card settings-panel-card">
        <h3 class="text-sm font-semibold text-text-secondary mb-4 flex items-center gap-2">
          <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4 text-success" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/>
          </svg>
          存储路径
        </h3>
        <div class="settings-auto-grid">
          <!-- 输出目录 -->
          <div class="space-y-2">
            <label class="settings-label">输出目录 <HelpTip text="回显输出与配置备份文件的保存目录。" /></label>
          <input
            type="text"
            v-model="settings.outputDir"
            placeholder="如: output"
            class="w-full px-3 py-2 bg-bg-panel border border-border rounded-lg text-sm text-text-primary focus:outline-none focus:border-accent focus:ring-1 focus:ring-accent/30 transition-all"
          />
          </div>

          <!-- 日志目录 -->
          <div class="space-y-2">
            <label class="settings-label">日志目录 <HelpTip text="系统运行日志文件保存目录，默认 logs。" /></label>
          <input
            type="text"
            v-model="settings.logDir"
            placeholder="如: logs"
            class="w-full px-3 py-2 bg-bg-panel border border-border rounded-lg text-sm text-text-primary focus:outline-none focus:border-accent focus:ring-1 focus:ring-accent/30 transition-all"
          />
          </div>
        </div>
      </div>

      <!-- 调试日志设置 -->
      <div class="settings-card bg-bg-card border border-border rounded-xl p-5 shadow-card settings-panel-card">
        <h3 class="text-sm font-semibold text-text-secondary mb-4 flex items-center gap-2">
          <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4 text-info" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/>
            <polyline points="14 2 14 8 20 8"/>
            <line x1="16" y1="13" x2="8" y2="13"/>
            <line x1="16" y1="17" x2="8" y2="17"/>
            <polyline points="10 9 9 9 8 9"/>
          </svg>
          调试日志
        </h3>
        <div class="space-y-4">
          <!-- Debug 开关 -->
          <div class="flex items-center justify-between p-3 rounded-lg bg-bg-panel/50 border border-border/50">
            <div class="space-y-1">
              <div class="flex items-center gap-2">
                <span class="text-sm font-medium text-text-primary">启用 Debug 日志</span>
                <HelpTip text="输出一般调试信息，适用于常规问题排查。" />
                <span class="px-2 py-0.5 text-xs bg-info/20 text-info rounded">Debug</span>
              </div>
            </div>
            <label class="relative inline-flex items-center cursor-pointer">
              <input
                type="checkbox"
                v-model="settings.debug"
                class="sr-only peer"
              />
              <div class="w-11 h-6 bg-bg-panel border border-border peer-focus:outline-none peer-focus:ring-2 peer-focus:ring-accent/30 rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-accent"></div>
            </label>
          </div>

          <!-- DebugAll 开关 -->
          <div class="flex items-center justify-between p-3 rounded-lg bg-bg-panel/50 border border-border/50">
            <div class="space-y-1">
              <div class="flex items-center gap-2">
                <span class="text-sm font-medium text-text-primary">启用 DebugAll 日志</span>
                <HelpTip text="输出全量详细日志，包含底层通信数据，日志体积会显著增加。" />
                <span class="px-2 py-0.5 text-xs bg-warning/20 text-warning rounded">Verbose</span>
              </div>
            </div>
            <label class="relative inline-flex items-center cursor-pointer">
              <input
                type="checkbox"
                v-model="settings.debugAll"
                @change="onDebugAllChange"
                class="sr-only peer"
              />
              <div class="w-11 h-6 bg-bg-panel border border-border peer-focus:outline-none peer-focus:ring-2 peer-focus:ring-warning/30 rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-warning"></div>
            </label>
          </div>

          <!-- 提示信息 -->
          <div class="p-3 rounded-lg bg-info/10 border border-info/30">
            <div class="flex items-start gap-2">
              <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4 text-info mt-0.5 flex-shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <circle cx="12" cy="12" r="10"/>
                <line x1="12" y1="16" x2="12" y2="12"/>
                <line x1="12" y1="8" x2="12.01" y2="8"/>
              </svg>
              <p class="text-xs text-info">
                启用 DebugAll 会自动同时启用 Debug。日志文件保存在 <code class="bg-bg-panel px-1 py-0.5 rounded">{{ settings.logDir || 'logs' }}/app.log</code>
              </p>
            </div>
          </div>
        </div>
      </div>

      <!-- SSH 算法配置 -->
      <div class="settings-card bg-bg-card border border-border rounded-xl p-5 shadow-card settings-panel-card">
        <h3 class="text-sm font-semibold text-text-secondary mb-4 flex items-center gap-2">
          <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4 text-accent" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <rect x="3" y="11" width="18" height="11" rx="2" ry="2"/>
            <path d="M7 11V7a5 5 0 0 1 10 0v4"/>
          </svg>
          SSH 算法配置
        </h3>

        <!-- 预设模式选择 -->
        <div class="space-y-2 mb-4">
          <label class="settings-label">预设模式 <HelpTip :text="presetModeDescription || '选择 SSH 算法策略：安全优先、兼容模式或自定义。'" /></label>
          <select
            v-model="settings.sshAlgorithms.presetMode"
            class="w-full px-3 py-2 bg-bg-panel border border-border rounded-lg text-sm text-text-primary focus:outline-none focus:border-accent focus:ring-1 focus:ring-accent/30 transition-all"
          >
            <option value="secure">安全优先</option>
            <option value="compatible">兼容模式（推荐）</option>
            <option value="custom">自定义</option>
          </select>
        </div>

        <!-- 预设模式说明 -->
        <div class="mb-4 p-3 rounded-lg bg-bg-panel/50 border border-border/50">
          <div class="text-xs space-y-1">
            <p v-if="settings.sshAlgorithms.presetMode === 'secure'" class="text-success">
              <span class="font-medium">安全优先：</span>仅使用现代安全算法（AEAD加密、椭圆曲线密钥交换、ED25519主机密钥），适用于新设备和追求最高安全性的场景。
            </p>
            <p v-else-if="settings.sshAlgorithms.presetMode === 'compatible'" class="text-accent">
              <span class="font-medium">兼容模式：</span>包含老旧设备支持的算法（CBC加密、SHA1密钥交换、RSA/DSA主机密钥），兼容性最佳，推荐用于网络设备管理。
            </p>
            <p v-else class="text-warning">
              <span class="font-medium">自定义：</span>手动指定所有算法配置，适用于特殊场景。请确保了解每个算法的安全性和兼容性影响。
            </p>
          </div>
        </div>

        <!-- 自定义算法配置（仅自定义模式时显示） -->
        <div v-if="settings.sshAlgorithms.presetMode === 'custom'" class="space-y-4">
          <!-- 安全警告 -->
          <div class="p-3 rounded-lg bg-warning/10 border border-warning/30">
            <div class="flex items-start gap-2">
              <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4 text-warning mt-0.5 flex-shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/>
                <line x1="12" y1="9" x2="12" y2="13"/>
                <line x1="12" y1="17" x2="12.01" y2="17"/>
              </svg>
              <p class="text-xs text-warning">
                自定义算法配置可能导致连接失败或安全风险。请确保输入正确的算法名称，多个算法用英文逗号分隔。
              </p>
            </div>
          </div>

          <!-- 加密算法 -->
          <div class="space-y-2">
            <label class="settings-label">加密算法 (Ciphers) <HelpTip text="SSH 会话加密算法列表，多个算法使用英文逗号分隔。" /></label>
            <input
              type="text"
              :value="arrayToString(settings.sshAlgorithms.ciphers)"
              @input="settings.sshAlgorithms.ciphers = stringToArray(($event.target as HTMLInputElement)?.value || '')"
              placeholder="如: aes128-ctr,aes192-ctr,aes256-ctr"
              class="w-full px-3 py-2 bg-bg-panel border border-border rounded-lg text-sm text-text-primary focus:outline-none focus:border-accent focus:ring-1 focus:ring-accent/30 transition-all font-mono"
            />
          </div>

          <!-- 密钥交换算法 -->
          <div class="space-y-2">
            <label class="settings-label">密钥交换算法 (Key Exchanges) <HelpTip text="SSH 密钥交换算法列表，多个算法使用英文逗号分隔。" /></label>
            <input
              type="text"
              :value="arrayToString(settings.sshAlgorithms.keyExchanges)"
              @input="settings.sshAlgorithms.keyExchanges = stringToArray(($event.target as HTMLInputElement)?.value || '')"
              placeholder="如: curve25519-sha256,ecdh-sha2-nistp256"
              class="w-full px-3 py-2 bg-bg-panel border border-border rounded-lg text-sm text-text-primary focus:outline-none focus:border-accent focus:ring-1 focus:ring-accent/30 transition-all font-mono"
            />
          </div>

          <!-- MAC 算法 -->
          <div class="space-y-2">
            <label class="settings-label">MAC 算法 (Message Authentication Codes) <HelpTip text="SSH 消息认证码算法列表，多个算法使用英文逗号分隔。" /></label>
            <input
              type="text"
              :value="arrayToString(settings.sshAlgorithms.macs)"
              @input="settings.sshAlgorithms.macs = stringToArray(($event.target as HTMLInputElement)?.value || '')"
              placeholder="如: hmac-sha2-256,hmac-sha2-512"
              class="w-full px-3 py-2 bg-bg-panel border border-border rounded-lg text-sm text-text-primary focus:outline-none focus:border-accent focus:ring-1 focus:ring-accent/30 transition-all font-mono"
            />
          </div>

          <!-- 主机密钥算法 -->
          <div class="space-y-2">
            <label class="settings-label">主机密钥算法 (Host Key Algorithms) <HelpTip text="主机密钥验证算法列表，多个算法使用英文逗号分隔。" /></label>
            <input
              type="text"
              :value="arrayToString(settings.sshAlgorithms.hostKeyAlgorithms)"
              @input="settings.sshAlgorithms.hostKeyAlgorithms = stringToArray(($event.target as HTMLInputElement)?.value || '')"
              placeholder="如: ssh-ed25519,ecdsa-sha2-nistp256"
              class="w-full px-3 py-2 bg-bg-panel border border-border rounded-lg text-sm text-text-primary focus:outline-none focus:border-accent focus:ring-1 focus:ring-accent/30 transition-all font-mono"
            />
          </div>
        </div>
      </div>
      </div>

      <!-- 全局设置操作按钮 -->
      <div class="settings-actions">
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

      <!-- 运行时配置面板 -->
      <div class="runtime-panel-wrap">
        <RuntimeConfigPanel />
      </div>
    </div>

    <!-- Toast 提示 -->
    <div
      v-if="toast.show"
      class="fixed top-20 left-1/2 -translate-x-1/2 px-4 py-3 rounded-lg shadow-lg text-sm font-medium animate-slide-up z-50"
      :class="toast.type === 'success' ? 'bg-success text-white' : 'bg-error text-white'"
    >
      {{ toast.message }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { SettingsAPI } from '../services/api'
import type { GlobalSettings as BackendSettings } from '../services/api'
import RuntimeConfigPanel from '../components/settings/RuntimeConfigPanel.vue'
import HelpTip from '../components/common/HelpTip.vue'

// SSH 算法配置接口
interface SSHAlgorithmSettings {
  ciphers: string[]
  keyExchanges: string[]
  macs: string[]
  hostKeyAlgorithms: string[]
  presetMode: string
}

// 前端使用的设置接口（小写字段名，与后端绑定类保持一致）
interface GlobalSettings {
  maxWorkers: number
  connectTimeout: string
  commandTimeout: string
  outputDir: string
  logDir: string
  errorMode: string
  debug: boolean
  debugAll: boolean
  sshAlgorithms: SSHAlgorithmSettings
}

const loading = ref(true)
const saving = ref(false)

// 默认 SSH 算法配置
const defaultSSHAlgorithms: SSHAlgorithmSettings = {
  ciphers: [],
  keyExchanges: [],
  macs: [],
  hostKeyAlgorithms: [],
  presetMode: 'compatible'
}

const settings = ref<GlobalSettings>({
  maxWorkers: 32,
  connectTimeout: '10s',
  commandTimeout: '30s',
  outputDir: 'output',
  logDir: 'logs',
  errorMode: 'pause',
  debug: false,
  debugAll: false,
  sshAlgorithms: { ...defaultSSHAlgorithms }
})

const defaultSettings: GlobalSettings = {
  maxWorkers: 32,
  connectTimeout: '10s',
  commandTimeout: '30s',
  outputDir: 'output',
  logDir: 'logs',
  errorMode: 'pause',
  debug: false,
  debugAll: false,
  sshAlgorithms: { ...defaultSSHAlgorithms }
}

const toast = ref({
  show: false,
  message: '',
  type: 'success' as 'success' | 'error'
})

// 预设模式说明
const presetModeDescription = computed(() => {
  switch (settings.value.sshAlgorithms.presetMode) {
    case 'secure':
      return '仅使用现代安全算法，适用于新设备'
    case 'compatible':
      return '兼容老旧设备，推荐用于网络设备管理'
    case 'custom':
      return '手动指定算法配置'
    default:
      return ''
  }
})

// DebugAll 变更处理：启用 DebugAll 时自动启用 Debug
function onDebugAllChange() {
  if (settings.value.debugAll) {
    settings.value.debug = true
  }
}

// 数组转逗号分隔字符串
function arrayToString(arr: string[]): string {
  return arr.join(', ')
}

// 逗号分隔字符串转数组
function stringToArray(str: string): string[] {
  if (!str || !str.trim()) return []
  return str.split(',').map(s => s.trim()).filter(s => s.length > 0)
}

function showToast(message: string, type: 'success' | 'error' = 'success') {
  toast.value = { show: true, message, type }
  setTimeout(() => {
    toast.value.show = false
  }, 3000)
}

async function loadSettings() {
  try {
    loading.value = true
    const result: BackendSettings | null = await SettingsAPI.loadSettings()
    if (result) {
      // 后端返回小写字段名，直接赋值给前端（现在统一使用小写）
      const rawResult = result as any
      settings.value = {
        maxWorkers: result.maxWorkers || 32,
        connectTimeout: result.connectTimeout || '10s',
        commandTimeout: result.commandTimeout || '30s',
        outputDir: result.outputDir || 'output',
        logDir: result.logDir || 'logs',
        errorMode: result.errorMode || 'pause',
        debug: rawResult.debug || false,
        debugAll: rawResult.debugAll || false,
        sshAlgorithms: rawResult.sshAlgorithms || { ...defaultSSHAlgorithms }
      }
    }
  } catch (err) {
    console.error('Failed to load settings:', err)
    showToast('加载设置失败', 'error')
  } finally {
    loading.value = false
  }
}

// 类型转换函数：前端格式 → 后端格式
// 由于现在前端和后端都使用小写字段名，直接返回即可
function toBackendSettings(frontend: GlobalSettings): BackendSettings {
  return {
    id: 1,
    maxWorkers: frontend.maxWorkers,
    connectTimeout: frontend.connectTimeout,
    commandTimeout: frontend.commandTimeout,
    outputDir: frontend.outputDir,
    logDir: frontend.logDir,
    errorMode: frontend.errorMode,
    debug: frontend.debug,
    debugAll: frontend.debugAll,
    sshAlgorithms: {
      presetMode: frontend.sshAlgorithms.presetMode,
      ciphers: frontend.sshAlgorithms.ciphers,
      keyExchanges: frontend.sshAlgorithms.keyExchanges,
      macs: frontend.sshAlgorithms.macs,
      hostKeyAlgorithms: frontend.sshAlgorithms.hostKeyAlgorithms
    }
  } as unknown as BackendSettings
}

async function saveSettings() {
  try {
    saving.value = true
    const backendSettings = toBackendSettings(settings.value)
    await SettingsAPI.saveSettings(backendSettings)
    showToast('设置已保存')
  } catch (err) {
    console.error('Failed to save settings:', err)
    const errorMessage = err instanceof Error ? err.message : '未知错误'
    showToast(`保存设置失败: ${errorMessage}`, 'error')
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

<style scoped>
.settings-page {
  display: flex;
  flex-direction: column;
  gap: 1.25rem;
  max-width: 1400px;
  margin: 0 auto;
  padding-bottom: 1rem;
}

.settings-page-header {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 1rem;
  padding: 0.25rem 0.25rem 0;
}

.settings-page-title {
  font-size: 1.35rem;
  font-weight: 700;
  letter-spacing: -0.02em;
  color: var(--color-text-primary);
}

.settings-page-subtitle {
  font-size: 0.82rem;
  color: var(--color-text-muted);
}

.settings-page-badge {
  padding: 0.35rem 0.7rem;
  border-radius: 9999px;
  border: 1px solid var(--color-border-default);
  background: var(--color-bg-secondary);
  color: var(--color-text-secondary);
  font-size: 0.72rem;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.settings-loading {
  min-height: 220px;
  border: 1px solid var(--color-border-default);
  border-radius: 1rem;
  background: var(--color-bg-secondary);
  display: flex;
  align-items: center;
  justify-content: center;
}

.settings-content {
  display: flex;
  flex-direction: column;
  gap: 1.25rem;
}

.global-settings-panels-flow {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(330px, 1fr));
  gap: 1rem;
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

.settings-card {
  overflow: visible;
  border-radius: 1rem;
  border: 1px solid var(--color-border-default);
  background: var(--color-bg-secondary);
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.06);
  transition: box-shadow 0.2s ease, border-color 0.2s ease;
}

.settings-card:hover {
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

.settings-actions {
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

.runtime-panel-wrap {
  border-top: 1px solid var(--color-border-default);
  padding-top: 1rem;
}

@media (max-width: 960px) {
  .settings-page-header {
    flex-direction: column;
    align-items: flex-start;
  }

  .global-settings-panels-flow {
    grid-template-columns: 1fr;
  }

  .settings-actions {
    position: static;
    flex-wrap: wrap;
    justify-content: flex-start;
  }
}
</style>
