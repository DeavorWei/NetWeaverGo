<script setup lang="ts">
import type { TracertConfig } from '@/bindings/github.com/NetWeaverGo/core/internal/icmp/models'

interface Props {
  show: boolean
  target: string
  config: TracertConfig
  continuous: boolean
  disabled: boolean
}

const props = defineProps<Props>()

const emit = defineEmits<{
  'update:show': [value: boolean]
  'update:target': [value: string]
  'update:config': [value: TracertConfig]
  'update:continuous': [value: boolean]
  confirm: []
}>()

const handleConfirm = () => {
  emit('confirm')
  emit('update:show', false)
}

const handleCancel = () => {
  emit('update:show', false)
}

const updateConfig = <K extends keyof TracertConfig>(key: K, value: TracertConfig[K]) => {
  emit('update:config', { ...props.config, [key]: value })
}
</script>

<template>
  <Teleport to="body">
    <div v-if="show" class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm" @click.self="handleCancel">
      <div class="bg-bg-secondary border border-border rounded-xl shadow-xl w-[560px] max-h-[85vh] flex flex-col glass-strong">
        <!-- Header -->
        <div class="flex items-center justify-between p-4 border-b border-border">
          <h3 class="text-lg font-semibold text-text-primary flex items-center gap-2">
            <svg class="w-5 h-5 text-accent" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
            </svg>
            Tracert 探测设置
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
            <h3 class="text-sm font-semibold text-text-primary mb-3 flex items-center">
              <svg class="w-5 h-5 mr-2 text-accent" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2" />
              </svg>
              目标地址
            </h3>
            <input
              :value="target"
              @input="emit('update:target', ($event.target as HTMLInputElement).value)"
              :disabled="disabled"
              placeholder="输入 IP 地址或域名，如: 8.8.8.8 或 www.example.com"
              class="w-full bg-bg-tertiary/50 border border-border rounded-lg px-3 py-2 text-sm text-text-primary placeholder-text-muted focus:outline-none focus:border-accent transition-colors font-mono"
            />
          </div>

          <!-- 探测参数 -->
          <div class="glass-card p-4">
            <h3 class="text-sm font-semibold text-text-primary mb-3 flex items-center">
              <svg class="w-5 h-5 mr-2 text-accent" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
              </svg>
              探测参数
            </h3>
            <div class="space-y-3">
              <div class="flex items-center justify-between">
                <label class="text-sm text-text-secondary">最大跳数 <span class="text-xs text-text-muted">(1-255)</span></label>
                <input
                  :value="config.maxHops"
                  @input="updateConfig('maxHops', Number(($event.target as HTMLInputElement).value))"
                  type="number"
                  :disabled="disabled"
                  min="1"
                  max="255"
                  class="w-24 bg-bg-tertiary/50 border border-border rounded px-2 py-1 text-sm text-text-primary text-right focus:outline-none focus:border-accent"
                />
              </div>
              <div class="flex items-center justify-between">
                <label class="text-sm text-text-secondary">超时 <span class="text-xs text-text-muted">(ms)</span></label>
                <input
                  :value="config.timeout"
                  @input="updateConfig('timeout', Number(($event.target as HTMLInputElement).value))"
                  type="number"
                  :disabled="disabled"
                  min="500"
                  max="30000"
                  class="w-24 bg-bg-tertiary/50 border border-border rounded px-2 py-1 text-sm text-text-primary text-right focus:outline-none focus:border-accent"
                />
              </div>
              <div class="flex items-center justify-between">
                <label class="text-sm text-text-secondary">包大小 <span class="text-xs text-text-muted">(bytes)</span></label>
                <input
                  :value="config.dataSize"
                  @input="updateConfig('dataSize', Number(($event.target as HTMLInputElement).value))"
                  type="number"
                  :disabled="disabled"
                  min="1"
                  max="65500"
                  class="w-24 bg-bg-tertiary/50 border border-border rounded px-2 py-1 text-sm text-text-primary text-right focus:outline-none focus:border-accent"
                />
              </div>
              <div class="flex items-center justify-between">
                <label class="text-sm text-text-secondary">探测间隔 <span class="text-xs text-text-muted">(ms, 1-60000)</span></label>
                <input
                  :value="config.interval"
                  @input="updateConfig('interval', Number(($event.target as HTMLInputElement).value))"
                  type="number"
                  :disabled="disabled"
                  min="1"
                  max="60000"
                  class="w-24 bg-bg-tertiary/50 border border-border rounded px-2 py-1 text-sm text-text-primary text-right focus:outline-none focus:border-accent"
                />
              </div>
              <div v-show="!continuous" class="flex items-center justify-between">
                <label class="text-sm text-text-secondary">探测次数</label>
                <input
                  :value="config.count"
                  @input="updateConfig('count', Number(($event.target as HTMLInputElement).value))"
                  type="number"
                  :disabled="disabled"
                  min="1"
                  max="1000000"
                  class="w-24 bg-bg-tertiary/50 border border-border rounded px-2 py-1 text-sm text-text-primary text-right focus:outline-none focus:border-accent"
                />
              </div>
              <div class="flex items-center justify-between">
                <label class="text-sm text-text-secondary">启用持续探测</label>
                <button
                  @click="emit('update:continuous', !continuous)"
                  :disabled="disabled"
                  class="relative w-10 h-5 rounded-full transition-colors"
                  :class="continuous ? 'bg-accent' : 'bg-bg-tertiary'"
                >
                  <span
                    class="absolute top-0.5 left-0.5 w-4 h-4 bg-text-inverse rounded-full transition-transform"
                    :class="continuous ? 'translate-x-5' : ''"
                  ></span>
                </button>
              </div>
            </div>
          </div>
        </div>

        <!-- Footer -->
        <div class="flex justify-end gap-2 p-4 border-t border-border">
          <button @click="handleCancel" class="px-4 py-2 text-text-secondary hover:text-text-primary transition-colors">
            取消
          </button>
          <button @click="handleConfirm" class="px-4 py-2 bg-accent hover:bg-accent-hover text-white rounded-lg transition-colors">
            确认
          </button>
        </div>
      </div>
    </div>
  </Teleport>
</template>
