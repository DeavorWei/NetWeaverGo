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
          <div class="space-y-2">
            <label class="settings-label">数据根目录 <HelpTip text="统一数据根目录，系统会在其下自动创建 db/logs/execution/backup 子目录。" /></label>
          <input
            type="text"
            v-model="settings.storageRoot"
            placeholder="如: ./netWeaverGoData"
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

          <!-- Verbose 开关 -->
          <div class="flex items-center justify-between p-3 rounded-lg bg-bg-panel/50 border border-border/50">
            <div class="space-y-1">
              <div class="flex items-center gap-2">
                <span class="text-sm font-medium text-text-primary">启用 Verbose 日志</span>
                <HelpTip text="输出全量详细日志，包含底层通信数据，日志体积会显著增加。" />
                <span class="px-2 py-0.5 text-xs bg-warning/20 text-warning rounded">Verbose</span>
              </div>
            </div>
            <label class="relative inline-flex items-center cursor-pointer">
              <input
                type="checkbox"
                v-model="settings.verbose"
                @change="onVerboseChange"
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
              <p class="text-xs text-info break-words">
                启用 Verbose 会自动同时启用 Debug。日志文件保存在 <code class="bg-bg-panel px-1 py-0.5 rounded break-all">{{ settings.storageRoot || './netWeaverGoData' }}/logs/app/app.log</code>
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
                自定义算法配置可能导致连接失败或安全风险。不安全算法会被显式标注；历史算法会保留并标记。
              </p>
            </div>
          </div>

          <div class="algo-custom-summary">
            <div class="algo-summary-grid">
              <div
                v-for="section in algorithmSections"
                :key="section.key"
                class="algo-summary-item"
              >
                <div class="algo-summary-title">
                  {{ section.label }}
                </div>
                <div class="algo-summary-count">
                  已选 {{ getSelectedCount(section.key) }} / {{ getAlgorithmOptions(section.key).length }}
                </div>
              </div>
            </div>
            <button
              type="button"
              class="algo-open-modal-btn"
              @click="openCustomAlgorithmModal"
            >
              打开算法选择窗口
            </button>
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

    <Transition name="algo-modal">
      <div
        v-if="showCustomAlgorithmModal && settings.sshAlgorithms.presetMode === 'custom'"
        class="algo-modal-overlay"
        @click.self="closeCustomAlgorithmModal"
      >
        <div class="algo-modal-panel settings-card" @click.stop>
          <div class="algo-modal-header">
            <div>
              <h3 class="algo-modal-title">自定义 SSH 算法</h3>
              <p class="algo-modal-subtitle">请按算法类别进行筛选与多选</p>
            </div>
            <button
              type="button"
              class="algo-modal-close"
              @click="closeCustomAlgorithmModal"
            >
              关闭
            </button>
          </div>

          <div class="algo-modal-body">
            <div class="algo-modal-grid">
              <div
                v-for="section in algorithmSections"
                :key="section.key"
                class="algo-section space-y-2"
              >
                <label class="settings-label">{{ section.label }} <HelpTip :text="section.help" /></label>

                <div class="algo-toolbar">
                  <input
                    v-model="sshAlgorithmSearch[section.key]"
                    type="text"
                    :placeholder="section.searchPlaceholder"
                    class="algo-search-input"
                  />
                  <button
                    type="button"
                    class="algo-action-btn"
                    @click="selectAllAlgorithms(section.key)"
                  >
                    全选
                  </button>
                  <button
                    type="button"
                    class="algo-action-btn"
                    @click="clearAllAlgorithms(section.key)"
                  >
                    清空
                  </button>
                </div>

                <div class="algo-count-line">
                  已选 {{ getSelectedCount(section.key) }} / {{ getAlgorithmOptions(section.key).length }}
                </div>

                <div class="algo-options-list">
                  <label
                    v-for="option in getFilteredAlgorithmOptions(section.key)"
                    :key="option.name"
                    class="algo-option-item"
                  >
                    <input
                      type="checkbox"
                      class="algo-option-checkbox"
                      :checked="isAlgorithmSelected(section.key, option.name)"
                      @change="toggleAlgorithm(section.key, option.name)"
                    />
                    <span class="algo-option-name">{{ option.name }}</span>
                    <span v-if="option.security === 'insecure'" class="algo-badge algo-badge-insecure">不安全</span>
                    <span v-else-if="option.security === 'legacy'" class="algo-badge algo-badge-legacy">历史</span>
                    <span v-else class="algo-badge algo-badge-secure">安全</span>
                  </label>

                  <div v-if="getFilteredAlgorithmOptions(section.key).length === 0" class="algo-empty">
                    未匹配到算法
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </Transition>

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
import { ref, computed, onMounted, watch } from 'vue'
import { SettingsAPI } from '../services/api'
import type { GlobalSettings as BackendSettings } from '../services/api'
import RuntimeConfigPanel from '../components/settings/RuntimeConfigPanel.vue'
import HelpTip from '../components/common/HelpTip.vue'

type AlgorithmField = 'ciphers' | 'keyExchanges' | 'macs' | 'hostKeyAlgorithms'
type AlgorithmSecurity = 'secure' | 'insecure' | 'legacy'
type AlgorithmSource = 'supported' | 'insecure' | 'legacy'

interface SSHAlgorithmOption {
  name: string
  security: AlgorithmSecurity
  source: AlgorithmSource
}

// SSH 算法配置接口
interface SSHAlgorithmSettings {
  ciphers: string[]
  keyExchanges: string[]
  macs: string[]
  hostKeyAlgorithms: string[]
  presetMode: string
}

interface SSHAlgorithmOptions {
  ciphers: SSHAlgorithmOption[]
  keyExchanges: SSHAlgorithmOption[]
  macs: SSHAlgorithmOption[]
  hostKeyAlgorithms: SSHAlgorithmOption[]
}

interface AlgorithmSection {
  key: AlgorithmField
  label: string
  help: string
  searchPlaceholder: string
}

// 前端使用的设置接口（小写字段名，与后端绑定类保持一致）
interface GlobalSettings {
  connectTimeout: string
  commandTimeout: string
  storageRoot: string
  errorMode: string
  debug: boolean
  verbose: boolean
  sshAlgorithms: SSHAlgorithmSettings
}

const loading = ref(true)
const saving = ref(false)
const showCustomAlgorithmModal = ref(false)
const initialPresetMode = ref<string | null>(null)
const sshAlgorithmOptions = ref<SSHAlgorithmOptions>({
  ciphers: [],
  keyExchanges: [],
  macs: [],
  hostKeyAlgorithms: []
})
const sshAlgorithmSearch = ref<Record<AlgorithmField, string>>({
  ciphers: '',
  keyExchanges: '',
  macs: '',
  hostKeyAlgorithms: ''
})

const algorithmSections: AlgorithmSection[] = [
  {
    key: 'ciphers',
    label: '加密算法 (Ciphers)',
    help: 'SSH 会话加密算法，支持多选。',
    searchPlaceholder: '搜索加密算法...'
  },
  {
    key: 'keyExchanges',
    label: '密钥交换算法 (Key Exchanges)',
    help: 'SSH 密钥交换算法，支持多选。',
    searchPlaceholder: '搜索密钥交换算法...'
  },
  {
    key: 'macs',
    label: 'MAC 算法 (Message Authentication Codes)',
    help: 'SSH 消息认证码算法，支持多选。',
    searchPlaceholder: '搜索 MAC 算法...'
  },
  {
    key: 'hostKeyAlgorithms',
    label: '主机密钥算法 (Host Key Algorithms)',
    help: '主机密钥验证算法，支持多选。',
    searchPlaceholder: '搜索主机密钥算法...'
  }
]

// 默认 SSH 算法配置
const defaultSSHAlgorithms: SSHAlgorithmSettings = {
  ciphers: [],
  keyExchanges: [],
  macs: [],
  hostKeyAlgorithms: [],
  presetMode: 'compatible'
}

const settings = ref<GlobalSettings>({
  connectTimeout: '10s',
  commandTimeout: '30s',
  storageRoot: './netWeaverGoData',
  errorMode: 'pause',
  debug: false,
  verbose: false,
  sshAlgorithms: { ...defaultSSHAlgorithms }
})

const defaultSettings: GlobalSettings = {
  connectTimeout: '10s',
  commandTimeout: '30s',
  storageRoot: './netWeaverGoData',
  errorMode: 'pause',
  debug: false,
  verbose: false,
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

// Verbose 变更处理：启用 Verbose 时自动启用 Debug
function onVerboseChange() {
  if (settings.value.verbose) {
    settings.value.debug = true
  }
}

function normalizeAlgorithmOptions(raw: any): SSHAlgorithmOptions {
  const normalizeList = (list: any): SSHAlgorithmOption[] => {
    if (!Array.isArray(list)) return []
    return list
      .map(item => ({
        name: String(item?.name || ''),
        security: item?.security === 'insecure' ? 'insecure' : 'secure',
        source: item?.source === 'insecure' ? 'insecure' : 'supported'
      }) as SSHAlgorithmOption)
      .filter(item => item.name.length > 0)
  }

  return {
    ciphers: normalizeList(raw?.ciphers),
    keyExchanges: normalizeList(raw?.keyExchanges),
    macs: normalizeList(raw?.macs),
    hostKeyAlgorithms: normalizeList(raw?.hostKeyAlgorithms)
  }
}

async function loadSSHAlgorithmOptions() {
  try {
    const result = await SettingsAPI.getSSHAlgorithmOptions()
    sshAlgorithmOptions.value = normalizeAlgorithmOptions(result)
  } catch (err) {
    console.error('Failed to load SSH algorithm options:', err)
    showToast('加载 SSH 算法候选失败，将保留已有配置', 'error')
  }
}

function getAlgorithmOptions(field: AlgorithmField): SSHAlgorithmOption[] {
  const currentOptions = [...sshAlgorithmOptions.value[field]]
  const selected = settings.value.sshAlgorithms[field] || []
  const optionMap = new Map<string, SSHAlgorithmOption>()

  for (const option of currentOptions) {
    optionMap.set(option.name, option)
  }

  for (const name of selected) {
    if (!optionMap.has(name)) {
      optionMap.set(name, {
        name,
        security: 'legacy',
        source: 'legacy'
      })
    }
  }

  const rank: Record<AlgorithmSecurity, number> = {
    secure: 0,
    insecure: 1,
    legacy: 2
  }

  return Array.from(optionMap.values()).sort((a, b) => {
    if (a.security !== b.security) {
      return rank[a.security] - rank[b.security]
    }
    return a.name.localeCompare(b.name)
  })
}

function getFilteredAlgorithmOptions(field: AlgorithmField): SSHAlgorithmOption[] {
  const keyword = sshAlgorithmSearch.value[field].trim().toLowerCase()
  const options = getAlgorithmOptions(field)
  if (!keyword) return options
  return options.filter(option => option.name.toLowerCase().includes(keyword))
}

function isAlgorithmSelected(field: AlgorithmField, name: string): boolean {
  return settings.value.sshAlgorithms[field].includes(name)
}

function toggleAlgorithm(field: AlgorithmField, name: string) {
  const current = new Set(settings.value.sshAlgorithms[field])
  if (current.has(name)) {
    current.delete(name)
  } else {
    current.add(name)
  }
  settings.value.sshAlgorithms[field] = Array.from(current)
}

function selectAllAlgorithms(field: AlgorithmField) {
  settings.value.sshAlgorithms[field] = getAlgorithmOptions(field).map(item => item.name)
}

function clearAllAlgorithms(field: AlgorithmField) {
  settings.value.sshAlgorithms[field] = []
}

function getSelectedCount(field: AlgorithmField): number {
  return settings.value.sshAlgorithms[field].length
}

function openCustomAlgorithmModal() {
  showCustomAlgorithmModal.value = true
}

function closeCustomAlgorithmModal() {
  showCustomAlgorithmModal.value = false
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
      const loadedPresetMode = rawResult.sshAlgorithms?.presetMode || 'compatible'
      // 保存初始值，用于判断是否是用户主动切换
      initialPresetMode.value = loadedPresetMode
      settings.value = {
        connectTimeout: result.connectTimeout || '10s',
        commandTimeout: result.commandTimeout || '30s',
        storageRoot: rawResult.storageRoot || './netWeaverGoData',
        errorMode: result.errorMode || 'pause',
        debug: rawResult.debug || false,
        verbose: rawResult.verbose || false,
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
    connectTimeout: frontend.connectTimeout,
    commandTimeout: frontend.commandTimeout,
    storageRoot: frontend.storageRoot,
    errorMode: frontend.errorMode,
    debug: frontend.debug,
    verbose: frontend.verbose,
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

watch(
  () => settings.value.sshAlgorithms.presetMode,
  (newMode) => {
    // initialPresetMode.value 为 null 表示还没加载完成
    if (initialPresetMode.value === null) {
      return
    }
    // 如果新值和初始值相同，说明是数据恢复（页面加载），不弹窗
    if (newMode === initialPresetMode.value) {
      return
    }
    // 只有用户主动从其他模式切换到自定义时才弹窗
    if (newMode === 'custom') {
      openCustomAlgorithmModal()
      return
    }
    if (newMode !== 'custom') {
      closeCustomAlgorithmModal()
    }
  }
)

onMounted(() => {
  Promise.all([loadSettings(), loadSSHAlgorithmOptions()])
})
</script>

<style scoped lang="postcss">
@reference "../styles/index.css";

/* 页面容器 */
.settings-page {
  @apply flex flex-col gap-5 max-w-[1400px] mx-auto pb-4;
}

/* 页面头部 */
.settings-page-header {
  @apply flex items-end justify-between gap-4 pt-1 pl-1;
}

.settings-page-title {
  @apply text-[1.35rem] font-bold tracking-tight text-text-primary;
}

.settings-page-subtitle {
  @apply text-[0.82rem] text-text-muted;
}

.settings-page-badge {
  @apply px-2.5 py-1.5 rounded-full border border-border bg-bg-secondary;
  @apply text-text-secondary text-[0.72rem] tracking-wide uppercase;
}

/* 加载状态 */
.settings-loading {
  @apply min-h-[220px] border border-border rounded-xl bg-bg-secondary;
  @apply flex items-center justify-center;
}

/* 内容区域 */
.settings-content {
  @apply flex flex-col gap-5;
}

/* 网格布局 */
.global-settings-panels-flow {
  @apply grid gap-4;
  grid-template-columns: repeat(auto-fit, minmax(330px, 1fr));
}

.settings-auto-grid {
  @apply grid gap-3.5 items-start;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
}

.settings-auto-grid > * {
  @apply min-w-0;
}

/* 卡片 */
.settings-panel-card {
  @apply w-full;
}

.settings-card {
  @apply overflow-visible rounded-xl border border-border bg-bg-secondary;
  @apply shadow-sm transition-shadow duration-200 transition-colors;
}

.settings-card:hover {
  @apply shadow-md border-border-focus;
}

/* 表单元素 */
.settings-panel-card :is(input:not([type="checkbox"]):not([type="radio"]), select) {
  @apply w-full min-h-[2.35rem] rounded-xl border border-border bg-bg-primary;
  @apply px-3.5 transition-colors duration-200;
}

.settings-panel-card :is(input:not([type="checkbox"]):not([type="radio"]), select):focus {
  @apply border-accent;
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--color-accent) 18%, transparent);
}

.settings-panel-card :is(label) {
  @apply text-[0.76rem] font-semibold text-text-secondary;
}

.settings-label {
  @apply inline-flex items-center gap-1.5 text-[0.76rem] font-semibold text-text-secondary;
}

.settings-panel-card :is(p.text-xs) {
  @apply leading-relaxed;
}

/* 算法配置区域 */
.algo-custom-summary {
  @apply border border-border rounded-xl p-3.5 flex flex-col gap-3;
  background: color-mix(in srgb, var(--color-bg-secondary) 70%, transparent);
}

.algo-summary-grid {
  @apply grid gap-2.5;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
}

.algo-summary-item {
  @apply border border-border rounded-[0.65rem] p-2 px-2.5 bg-bg-primary;
}

.algo-summary-title {
  @apply text-[0.74rem] text-text-secondary leading-tight;
}

.algo-summary-count {
  @apply mt-1 text-[0.72rem] text-text-muted;
}

.algo-open-modal-btn {
  @apply self-start border border-accent rounded-[0.65rem];
  @apply text-[0.78rem] font-semibold px-2 py-1.5;
  @apply transition-colors duration-200;
  background: color-mix(in srgb, var(--color-accent) 14%, transparent);
  @apply text-accent;
}

.algo-open-modal-btn:hover {
  background: color-mix(in srgb, var(--color-accent) 24%, transparent);
}

/* 算法区块 */
.algo-section {
  @apply border border-border rounded-xl p-3;
  background: color-mix(in srgb, var(--color-bg-secondary) 70%, transparent);
}

.algo-toolbar {
  @apply flex gap-2 items-center;
}

.algo-search-input {
  @apply flex-1 min-h-8 rounded-[0.65rem] border border-border bg-bg-primary;
  @apply text-text-primary px-2.5 py-1.5 text-sm;
}

.algo-action-btn {
  @apply border border-border bg-bg-secondary text-text-secondary;
  @apply rounded-[0.6rem] text-[0.75rem] px-2 py-1;
  @apply transition-colors duration-200;
}

.algo-action-btn:hover {
  @apply border-border-focus text-text-primary;
}

.algo-count-line {
  @apply text-[0.72rem] text-text-muted;
}

.algo-options-list {
  @apply max-h-44 overflow-y-auto border border-border rounded-[0.65rem] bg-bg-primary;
}

.algo-option-item {
  @apply flex items-center gap-2 px-2.5 py-1.5 cursor-pointer;
  border-bottom: 1px solid color-mix(in srgb, var(--color-border-default) 70%, transparent);
}

.algo-option-item:last-child {
  border-bottom: none;
}

.algo-option-item:hover {
  background: color-mix(in srgb, var(--color-accent) 7%, transparent);
}

.algo-option-checkbox {
  @apply w-3.5 h-3.5 flex-shrink-0;
  accent-color: var(--color-accent);
}

.algo-option-name {
  @apply flex-1 min-w-0 text-[0.78rem] text-text-primary font-mono;
  @apply break-all;
}

.algo-badge {
  @apply text-[0.65rem] rounded-full px-1.5 py-0.5 leading-tight;
}

.algo-badge-secure {
  background: color-mix(in srgb, var(--color-success) 15%, transparent);
  @apply text-success;
}

.algo-badge-insecure {
  background: color-mix(in srgb, var(--color-warning) 20%, transparent);
  @apply text-warning;
}

.algo-badge-legacy {
  background: color-mix(in srgb, var(--color-info) 18%, transparent);
  @apply text-info;
}

.algo-empty {
  @apply p-3 text-center text-[0.75rem] text-text-muted;
}

/* 算法模态框 */
.algo-modal-overlay {
  @apply fixed inset-0 z-[1300] flex items-center justify-center;
  @apply bg-black/55 p-4;
}

.algo-modal-panel {
  @apply w-[min(1120px,96vw)] max-h-[88vh] flex flex-col;
}

.algo-modal-panel:hover {
  @apply shadow-2xl border-border;
}

.algo-modal-header {
  @apply flex items-center justify-between gap-4 px-4 py-4;
  @apply border-b border-border;
}

.algo-modal-title {
  @apply text-[0.95rem] font-bold text-text-primary;
}

.algo-modal-subtitle {
  @apply mt-1 text-[0.75rem] text-text-muted;
}

.algo-modal-close {
  @apply border border-border bg-bg-primary text-text-secondary;
  @apply rounded-[0.6rem] text-[0.75rem] px-2 py-1;
  @apply transition-colors duration-200;
}

.algo-modal-close:hover {
  @apply text-text-primary border-border-focus;
}

.algo-modal-body {
  @apply overflow-y-auto p-4;
}

.algo-modal-grid {
  @apply grid gap-3;
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.algo-modal-body .algo-options-list {
  @apply max-h-60;
}

/* algo-modal 过渡动画已移至全局 _animations.css */

/* 操作按钮区域 */
.settings-actions {
  @apply sticky bottom-0 z-[5] flex justify-end gap-3 p-3;
  @apply border border-border rounded-xl;
  background: color-mix(in srgb, var(--color-bg-primary) 84%, transparent);
  @apply backdrop-blur-sm;
}

.runtime-panel-wrap {
  @apply border-t border-border pt-4;
}

/* 响应式 */
@media (max-width: 960px) {
  .settings-page-header {
    @apply flex-col items-start;
  }

  .global-settings-panels-flow {
    grid-template-columns: 1fr;
  }

  .settings-actions {
    @apply static flex-wrap justify-start;
  }

  .algo-modal-grid {
    grid-template-columns: 1fr;
  }
}
</style>
