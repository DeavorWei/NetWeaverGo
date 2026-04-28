<script setup lang="ts">
import { ref, computed } from 'vue'
import type { PingConfig } from '@/bindings/github.com/NetWeaverGo/core/internal/icmp/models'
import * as PingService from '@/bindings/github.com/NetWeaverGo/core/internal/ui/pingservice'

// 数据包大小限制常量
const RECOMMENDED_MAX_SIZE = 8000
const MTU_LIMIT = 1472

interface Props {
  show: boolean
  targetInput: string
  config: PingConfig
  resolveHostName: boolean
  enableRealtime: boolean
  realtimeThrottle: number
  disabled: boolean
}

const props = defineProps<Props>()

const emit = defineEmits<{
  'update:show': [value: boolean]
  'update:targetInput': [value: string]
  'update:config': [value: PingConfig]
  'update:resolveHostName': [value: boolean]
  'update:enableRealtime': [value: boolean]
  'update:realtimeThrottle': [value: number]
  confirm: []
  importDevices: []
}>()

// 语法糖展开状态
const expandStatus = ref<'idle' | 'expanding' | 'done' | 'error'>('idle')
const expandMessage = ref('')

// 数据包大小警告
const dataSizeWarning = computed(() => {
  const size = props.config.DataSize
  if (size > RECOMMENDED_MAX_SIZE) {
    return {
      type: 'error',
      message: `数据包大小超过推荐值 ${RECOMMENDED_MAX_SIZE} 字节，可能因 MTU 限制或系统资源不足而失败`
    }
  } else if (size > MTU_LIMIT) {
    return {
      type: 'warning',
      message: `数据包大小超过 ${MTU_LIMIT} 字节，需要 IP 分片，可能在某些网络环境下失败`
    }
  }
  return null
})

// 并发数警告（高并发时提示）
const concurrencyWarning = computed(() => {
  const concurrency = props.config.Concurrency
  if (concurrency > 256) {
    return {
      type: 'warning',
      message: `高并发设置 (${concurrency})，可能影响系统稳定性或触发网络设备防护`
    }
  }
  return null
})

// 处理目标输入框失去焦点 - 自动展开语法糖
const handleTargetBlur = async () => {
  const input = props.targetInput.trim()
  if (!input) return
  
  // 检查是否包含语法糖特征（CIDR、范围、逗号分隔等）
  const hasSyntaxSugar = /[-~/,\/]/.test(input)
  if (!hasSyntaxSugar) return
  
  expandStatus.value = 'expanding'
  try {
    const result = await PingService.ExpandPingTargets(input)
    if (result && result.ips && result.ips.length > 0) {
      emit('update:targetInput', result.ips.join('\n'))
      expandStatus.value = 'done'
      expandMessage.value = result.message || `已展开为 ${result.count} 个 IP`
    } else {
      expandStatus.value = 'error'
      expandMessage.value = result?.message || '无法展开'
    }
  } catch (err) {
    expandStatus.value = 'error'
    expandMessage.value = '展开失败'
    console.error('ExpandPingTargets failed:', err)
  }
}

const handleConfirm = () => {
  emit('confirm')
  emit('update:show', false)
}

const handleCancel = () => {
  emit('update:show', false)
}

const updateConfig = <K extends keyof PingConfig>(key: K, value: PingConfig[K]) => {
  emit('update:config', { ...props.config, [key]: value })
}
</script>

<template>
  <Teleport to="body">
    <div v-if="show" class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm" @click.self="handleCancel">
      <div class="bg-bg-secondary border border-border rounded-xl shadow-xl w-[640px] max-h-[85vh] flex flex-col glass-strong">
        <!-- Header -->
        <div class="flex items-center justify-between p-4 border-b border-border">
          <h3 class="text-lg font-semibold text-text-primary flex items-center gap-2">
            <svg class="w-5 h-5 text-accent" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
            </svg>
            Ping 检测设置
          </h3>
          <button @click="handleCancel" class="text-text-muted hover:text-text-primary transition-colors">
            <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        <!-- Body -->
        <div class="flex-1 overflow-y-auto p-4 space-y-4">
            <!-- 目标输入 -->
            <div class="glass-card p-4">
              <div class="flex items-center justify-between mb-3">
                <h3 class="text-sm font-semibold text-text-primary flex items-center">
                  <svg class="w-5 h-5 mr-2 text-accent" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2" />
                  </svg>
                  目标输入
                </h3>
                <button
                  @click="emit('importDevices')"
                  :disabled="disabled"
                  class="px-3 py-1 bg-bg-tertiary hover:bg-bg-hover border border-border rounded-lg text-sm text-text-primary transition-colors flex items-center gap-1.5 disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197m13.5-9a2.5 2.5 0 11-5 0 2.5 2.5 0 015 0z" />
                  </svg>
                  导入设备
                </button>
              </div>
              <textarea
                :value="targetInput"
                @input="emit('update:targetInput', ($event.target as HTMLTextAreaElement).value)"
                @blur="handleTargetBlur"
                :disabled="disabled"
                placeholder="输入 IP 地址&#10;支持格式：&#10;• 单个 IP: 192.168.1.1&#10;• CIDR: 192.168.1.0/24&#10;• 范围: 192.168.1.1-100 或 192.168.1.1~100&#10;• 多个 IP: 192.168.1.1, 192.168.1.2&#10;• 混合: 192.168.1.1, 192.168.1.0/30&#10;&#10;失去焦点时自动展开语法糖"
                class="w-full h-40 bg-bg-tertiary/50 border border-border rounded-lg p-3 text-sm text-text-primary placeholder-text-muted resize-none focus:outline-none focus:border-accent transition-colors font-mono"
              ></textarea>
              <!-- 语法糖展开状态提示 -->
              <div v-if="expandStatus !== 'idle'" class="mt-2 text-xs flex items-center gap-1"
                   :class="{
                     'text-text-muted': expandStatus === 'expanding',
                     'text-success': expandStatus === 'done',
                     'text-error': expandStatus === 'error'
                   }">
                <svg v-if="expandStatus === 'expanding'" class="w-3 h-3 animate-spin" fill="none" viewBox="0 0 24 24">
                  <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                  <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                </svg>
                <svg v-else-if="expandStatus === 'done'" class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
                </svg>
                <svg v-else-if="expandStatus === 'error'" class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                </svg>
                <span>{{ expandMessage }}</span>
              </div>
            </div>

            <!-- 配置参数 -->
            <div class="glass-card p-4">
              <h3 class="text-sm font-semibold text-text-primary mb-3 flex items-center">
                <svg class="w-5 h-5 mr-2 text-accent" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
                </svg>
                配置参数
              </h3>
              <div class="space-y-3">
                <div class="flex items-center justify-between">
                  <label class="text-sm text-text-secondary">超时 (ms)</label>
                  <input
                    :value="config.Timeout"
                    @input="updateConfig('Timeout', Number(($event.target as HTMLInputElement).value))"
                    type="number"
                    :disabled="disabled"
                    min="100"
                    max="30000"
                    class="w-24 bg-bg-tertiary/50 border border-border rounded px-2 py-1 text-sm text-text-primary text-right focus:outline-none focus:border-accent"
                  />
                </div>
                <div class="flex items-center justify-between">
                  <label class="text-sm text-text-secondary">重试次数</label>
                  <input
                    :value="config.Count"
                    @input="updateConfig('Count', Number(($event.target as HTMLInputElement).value))"
                    type="number"
                    :disabled="disabled"
                    min="1"
                    max="1000"
                    class="w-24 bg-bg-tertiary/50 border border-border rounded px-2 py-1 text-sm text-text-primary text-right focus:outline-none focus:border-accent"
                  />
                </div>
                <div class="flex items-center justify-between">
                  <label class="text-sm text-text-secondary">
                    并发数
                    <span class="text-xs text-text-muted ml-1">（0=自动）</span>
                  </label>
                  <input
                    :value="config.Concurrency"
                    @input="updateConfig('Concurrency', Number(($event.target as HTMLInputElement).value))"
                    type="number"
                    :disabled="disabled"
                    min="0"
                    placeholder="自动"
                    class="w-24 bg-bg-tertiary/50 border border-border rounded px-2 py-1 text-sm text-text-primary text-right focus:outline-none focus:border-accent"
                  />
                </div>
                <!-- 并发数警告 -->
                <div v-if="concurrencyWarning" class="mt-1 p-2 rounded-lg text-xs bg-warning/20 text-warning border border-warning/30">
                  <div class="flex items-start gap-2">
                    <svg class="w-4 h-4 flex-shrink-0 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                    </svg>
                    <span>{{ concurrencyWarning.message }}</span>
                  </div>
                </div>
                <div class="flex items-center justify-between">
                  <label class="text-sm text-text-secondary">包大小 (bytes)</label>
                  <input
                    :value="config.DataSize"
                    @input="updateConfig('DataSize', Number(($event.target as HTMLInputElement).value))"
                    type="number"
                    :disabled="disabled"
                    min="32"
                    max="65500"
                    class="w-24 bg-bg-tertiary/50 border border-border rounded px-2 py-1 text-sm text-text-primary text-right focus:outline-none focus:border-accent"
                  />
                </div>
                <!-- 数据包大小警告 -->
                <div v-if="dataSizeWarning" class="mt-2 p-2 rounded-lg text-xs"
                     :class="dataSizeWarning.type === 'error' ? 'bg-error/20 text-error border border-error/30' : 'bg-warning/20 text-warning border border-warning/30'">
                  <div class="flex items-start gap-2">
                    <svg class="w-4 h-4 flex-shrink-0 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                    </svg>
                    <span>{{ dataSizeWarning.message }}</span>
                  </div>
                </div>
                <div class="flex items-center justify-between">
                  <label class="text-sm text-text-secondary">间隔 (ms)</label>
                  <input
                    :value="config.Interval"
                    @input="updateConfig('Interval', Number(($event.target as HTMLInputElement).value))"
                    type="number"
                    :disabled="disabled"
                    min="0"
                    max="5000"
                    class="w-24 bg-bg-tertiary/50 border border-border rounded px-2 py-1 text-sm text-text-primary text-right focus:outline-none focus:border-accent"
                  />
                </div>
                <!-- 解析主机名选项 -->
                <div class="flex items-center justify-between pt-2 border-t border-border/50">
                  <label class="text-sm text-text-secondary flex items-center gap-2">
                    <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 12a9 9 0 01-9 9m9-9a9 9 0 00-9-9m9 9H3m9 9a9 9 0 01-9-9m9 9c1.657 0 3-4.03 3-9s-1.343-9-3-9m0 18c-1.657 0-3-4.03-3-9s1.343-9 3-9m-9 9a9 9 0 019-9" />
                    </svg>
                    解析主机名
                  </label>
                  <button
                    @click="emit('update:resolveHostName', !resolveHostName)"
                    :disabled="disabled"
                    class="relative w-10 h-5 rounded-full transition-colors"
                    :class="resolveHostName ? 'bg-accent' : 'bg-bg-tertiary'"
                  >
                    <span
                      class="absolute top-0.5 left-0.5 w-4 h-4 bg-text-inverse rounded-full transition-transform"
                      :class="resolveHostName ? 'translate-x-5' : ''"
                    ></span>
                  </button>
                </div>
                <!-- 实时进度选项 -->
                <div class="flex items-center justify-between pt-2 border-t border-border/50">
                  <label class="text-sm text-text-secondary flex items-center gap-2">
                    <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
                    </svg>
                    启用实时进度
                  </label>
                  <button
                    @click="emit('update:enableRealtime', !enableRealtime)"
                    :disabled="disabled"
                    class="relative w-10 h-5 rounded-full transition-colors"
                    :class="enableRealtime ? 'bg-accent' : 'bg-bg-tertiary'"
                  >
                    <span
                      class="absolute top-0.5 left-0.5 w-4 h-4 bg-text-inverse rounded-full transition-transform"
                      :class="enableRealtime ? 'translate-x-5' : ''"
                    ></span>
                  </button>
                </div>
                <div v-if="enableRealtime" class="flex items-center justify-between">
                  <label class="text-sm text-text-secondary">更新间隔(ms)</label>
                  <input
                    :value="realtimeThrottle"
                    @input="emit('update:realtimeThrottle', Number(($event.target as HTMLInputElement).value))"
                    type="number"
                    :disabled="disabled"
                    min="10"
                    max="5000"
                    class="w-24 bg-bg-tertiary/50 border border-border rounded px-2 py-1 text-sm text-text-primary text-right focus:outline-none focus:border-accent"
                  />
                </div>
              </div>
            </div>
          </div>

          <!-- Footer -->
          <div class="flex justify-end gap-2 p-4 border-t border-border">
            <button class="px-4 py-2 text-text-secondary hover:text-text-primary transition-colors" @click="handleCancel">取消</button>
            <button class="px-4 py-2 bg-accent hover:bg-accent-hover text-white rounded-lg transition-colors" @click="handleConfirm">确定</button>
          </div>
      </div>
    </div>
  </Teleport>
</template>
