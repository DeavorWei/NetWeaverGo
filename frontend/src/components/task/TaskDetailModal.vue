<template>
  <Transition name="modal">
    <div v-if="modelValue" class="fixed inset-0 z-50 flex items-center justify-center">
      <div class="absolute inset-0 bg-black/60 backdrop-blur-sm" @click="closeModal"></div>
      <div class="relative w-full max-w-5xl mx-4 max-h-[88vh] overflow-hidden rounded-2xl border border-border bg-bg-card shadow-2xl animate-slide-in flex flex-col">
        <div class="flex items-start justify-between gap-4 px-6 py-5 border-b border-border bg-bg-panel">
          <div>
            <h3 class="text-base font-semibold text-text-primary">任务详情</h3>
            <p class="text-xs text-text-muted mt-1">查看任务基础信息、设备概览与命令内容</p>
          </div>
          <button
            @click="closeModal"
            class="p-2 rounded-lg text-text-muted hover:text-text-primary hover:bg-bg-secondary transition-colors"
          >
            <svg xmlns="http://www.w3.org/2000/svg" class="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <line x1="18" y1="6" x2="6" y2="18"/>
              <line x1="6" y1="6" x2="18" y2="18"/>
            </svg>
          </button>
        </div>

        <div class="px-6 py-4 border-b border-border bg-bg-secondary/30 flex items-center gap-2 flex-wrap">
          <button
            v-for="tab in tabs"
            :key="tab.key"
            @click="activeTab = tab.key"
            class="px-3 py-1.5 rounded-lg text-sm font-medium transition-all"
            :class="activeTab === tab.key ? 'bg-accent text-white' : 'bg-bg-card border border-border text-text-muted hover:text-text-primary'"
          >
            {{ tab.label }}
          </button>
        </div>

        <div class="flex-1 overflow-y-auto scrollbar-custom p-6">
          <div v-if="loading" class="h-64 flex items-center justify-center">
            <div class="w-8 h-8 border-2 border-accent border-t-transparent rounded-full animate-spin"></div>
          </div>
          <div v-else-if="!detail" class="h-64 flex items-center justify-center text-sm text-text-muted">
            暂无任务详情
          </div>
          <template v-else>
            <section v-if="activeTab === 'info'" class="space-y-5">
              <div class="grid grid-cols-2 gap-4">
                <div class="rounded-xl border border-border bg-bg-panel p-4 space-y-3">
                  <div class="flex items-center gap-2 flex-wrap">
                    <h4 class="text-sm font-semibold text-text-primary">{{ detail.task.name }}</h4>
                    <span class="px-2 py-0.5 rounded-full text-xs border" :class="modeBadge(detail.task.mode)">
                      {{ modeLabel(detail.task.mode) }}
                    </span>
                    <span class="px-2 py-0.5 rounded-full text-xs border" :class="statusBadge(detail.latestRunStatus || 'pending')">
                      {{ statusLabel(detail.latestRunStatus || 'pending') }}
                    </span>
                  </div>
                  <p class="text-sm text-text-secondary">{{ detail.task.description || '暂无描述' }}</p>
                  <div class="flex flex-wrap gap-2">
                    <span
                      v-for="tag in detail.task.tags"
                      :key="tag"
                      class="px-2 py-0.5 rounded-full text-xs bg-accent/10 border border-accent/20 text-accent"
                    >
                      {{ tag }}
                    </span>
                    <span v-if="detail.task.tags.length === 0" class="text-xs text-text-muted">无标签</span>
                  </div>
                </div>

                <div class="rounded-xl border border-border bg-bg-panel p-4 space-y-3 text-sm">
                  <div class="flex items-center justify-between">
                    <span class="text-text-muted">任务项数量</span>
                    <span class="font-mono text-text-primary">{{ detail.itemCount }}</span>
                  </div>
                  <div class="flex items-center justify-between">
                    <span class="text-text-muted">创建时间</span>
                    <span class="text-text-primary">{{ formatDate(detail.task.createdAt) }}</span>
                  </div>
                  <div class="flex items-center justify-between">
                    <span class="text-text-muted">更新时间</span>
                    <span class="text-text-primary">{{ formatDate(detail.task.updatedAt) }}</span>
                  </div>
                  <div class="flex items-center justify-between gap-4">
                    <span class="text-text-muted">编辑状态</span>
                    <span :class="detail.canEdit ? 'text-success' : 'text-warning'">
                      {{ detail.canEdit ? '可编辑' : detail.editDisabledReason || '不可编辑' }}
                    </span>
                  </div>
                  <div class="flex items-center justify-between">
                    <span class="text-text-muted">原始日志</span>
                    <span :class="detail.task.enableRawLog ? 'text-warning' : 'text-text-primary'">
                      {{ detail.task.enableRawLog ? '已开启' : '已关闭' }}
                    </span>
                  </div>
                  <div class="pt-1 border-t border-border/60">
                    <p class="text-xs text-text-muted">编辑规则：存在活跃运行时不可编辑，状态展示来源于最近一次运行。</p>
                  </div>
                </div>
              </div>

              <div v-if="detail.missingDevices.length > 0 || detail.missingCommandIds.length > 0" class="rounded-xl border border-warning/30 bg-warning/5 p-4 space-y-2">
                <h4 class="text-sm font-semibold text-warning">引用缺失提示</h4>
                <p v-if="detail.missingDevices.length > 0" class="text-sm text-text-secondary">
                  缺失设备: <span class="font-mono">{{ detail.missingDevices.join(', ') }}</span>
                </p>
                <p v-if="detail.missingCommandIds.length > 0" class="text-sm text-text-secondary">
                  缺失命令组: <span class="font-mono">{{ detail.missingCommandIds.join(', ') }}</span>
                </p>
              </div>
            </section>

            <section v-else-if="activeTab === 'devices'" class="space-y-4">
              <div
                v-for="item in detail.items"
                :key="item.index"
                class="rounded-xl border border-border bg-bg-panel p-4"
              >
                <div class="flex items-center justify-between mb-3">
                  <h4 class="text-sm font-semibold text-text-primary">任务项 {{ item.index + 1 }}</h4>
                  <span class="text-xs text-text-muted">{{ item.deviceCount }} 台设备</span>
                </div>
                <div v-if="item.devices.length === 0" class="text-sm text-text-muted">未绑定设备</div>
                <div v-else class="grid grid-cols-2 gap-3">
                  <div
                    v-for="device in item.devices"
                    :key="`${item.index}-${device.ip}`"
                    class="rounded-lg border p-3"
                    :class="device.missing ? 'border-warning/40 bg-warning/5' : 'border-border bg-bg-card'"
                  >
                    <div class="flex items-center gap-2 flex-wrap">
                      <span class="font-mono text-sm text-text-primary">{{ device.ip }}</span>
                      <span v-if="device.missing" class="px-2 py-0.5 rounded-full text-xs border border-warning/40 text-warning bg-warning/10">
                        已失效
                      </span>
                    </div>
                    <div class="mt-2 text-xs text-text-muted">分组: {{ device.group || '未分组' }}</div>
                    <div class="flex flex-wrap gap-1 mt-2">
                      <span
                        v-for="tag in device.tags"
                        :key="tag"
                        class="px-1.5 py-0.5 rounded bg-accent/10 text-accent text-xs"
                      >
                        {{ tag }}
                      </span>
                      <span v-if="device.tags.length === 0" class="text-xs text-text-muted">无标签</span>
                    </div>
                  </div>
                </div>
              </div>
            </section>

            <section v-else class="space-y-4">
              <div
                v-for="item in detail.items"
                :key="`command-${item.index}`"
                class="rounded-xl border border-border bg-bg-panel p-4 space-y-3"
              >
                <div class="flex items-center justify-between">
                  <h4 class="text-sm font-semibold text-text-primary">任务项 {{ item.index + 1 }}</h4>
                  <span class="text-xs text-text-muted">{{ modeLabel(item.mode) }}</span>
                </div>

                <template v-if="detail.task.mode === 'group'">
                  <div v-if="item.commandInfo" class="rounded-lg border p-3" :class="item.commandInfo.missing ? 'border-warning/40 bg-warning/5' : 'border-border bg-bg-card'">
                    <div class="flex items-center gap-2 flex-wrap">
                      <span class="text-sm font-semibold text-text-primary">{{ item.commandInfo.name }}</span>
                      <span v-if="item.commandInfo.missing" class="px-2 py-0.5 rounded-full text-xs border border-warning/40 text-warning bg-warning/10">
                        已失效
                      </span>
                    </div>
                    <p class="text-xs text-text-muted mt-1">{{ item.commandInfo.description || '暂无描述' }}</p>
                    <div class="mt-3 rounded-lg bg-terminal-bg p-3 max-h-64 overflow-y-auto scrollbar-custom">
                      <div
                        v-for="(command, index) in item.commandInfo.commands"
                        :key="`${item.index}-${index}`"
                        class="font-mono text-sm text-terminal-text py-1 border-b border-white/5 last:border-0"
                      >
                        <span class="text-text-muted mr-2">{{ index + 1 }}.</span>{{ command }}
                      </div>
                      <div v-if="item.commandInfo.commands.length === 0" class="text-sm text-text-muted">命令组内暂无命令</div>
                    </div>
                  </div>
                </template>

                <template v-else>
                  <div class="rounded-lg bg-terminal-bg p-3 max-h-64 overflow-y-auto scrollbar-custom">
                    <div
                      v-for="(command, index) in item.commands"
                      :key="`${item.index}-binding-${index}`"
                      class="font-mono text-sm text-terminal-text py-1 border-b border-white/5 last:border-0"
                    >
                      <span class="text-text-muted mr-2">{{ index + 1 }}.</span>{{ command }}
                    </div>
                    <div v-if="item.commands.length === 0" class="text-sm text-text-muted">当前任务项暂无命令</div>
                  </div>
                </template>
              </div>
            </section>
          </template>
        </div>

        <div class="flex items-center justify-between gap-3 px-6 py-4 border-t border-border bg-bg-panel">
          <p v-if="detail && !detail.canEdit && detail.editDisabledReason" class="text-xs text-warning">
            {{ detail.editDisabledReason }}
          </p>
          <div class="ml-auto flex gap-3">
            <button
              v-if="detail?.canEdit"
              @click="emit('edit')"
              class="px-4 py-2 rounded-lg text-sm font-semibold bg-accent hover:bg-accent-glow text-white transition-all"
            >
              编辑任务
            </button>
            <button
              @click="closeModal"
              class="px-4 py-2 rounded-lg text-sm font-medium bg-bg-card border border-border text-text-secondary hover:text-text-primary"
            >
              关闭
            </button>
          </div>
        </div>
      </div>
    </div>
  </Transition>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import type { TaskGroupDetailViewModel } from '../../services/api'

const props = defineProps<{
  modelValue: boolean
  loading: boolean
  detail: TaskGroupDetailViewModel | null
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void
  (e: 'edit'): void
}>()

const tabs = [
  { key: 'info', label: '任务信息' },
  { key: 'devices', label: '设备信息' },
  { key: 'commands', label: '命令信息' }
] as const

const activeTab = ref<typeof tabs[number]['key']>('info')

watch(
  () => props.modelValue,
  (value) => {
    if (value) {
      activeTab.value = 'info'
    }
  }
)

function closeModal() {
  activeTab.value = 'info'
  emit('update:modelValue', false)
}

function modeLabel(mode: string) {
  return mode === 'group' ? '模式A' : mode === 'binding' ? '模式B' : mode
}

function modeBadge(mode: string) {
  return mode === 'group'
    ? 'bg-info/10 border-info/30 text-info'
    : 'bg-warning/10 border-warning/30 text-warning'
}

function statusLabel(status: string) {
  const mapping: Record<string, string> = {
    pending: '待执行',
    running: '执行中',
    completed: '已完成',
    partial: '部分成功',
    failed: '失败',
    cancelled: '已取消',
    aborted: '已中止'
  }
  return mapping[status] ?? status
}

function statusBadge(status: string) {
  switch (status) {
    case 'pending':
      return 'bg-bg-card border-border text-text-muted'
    case 'running':
      return 'bg-accent/10 border-accent/30 text-accent'
    case 'completed':
      return 'bg-success/10 border-success/30 text-success'
    case 'partial':
      return 'bg-warning/10 border-warning/30 text-warning'
    case 'failed':
    case 'aborted':
      return 'bg-error/10 border-error/30 text-error'
    case 'cancelled':
      return 'bg-bg-panel border-border text-text-muted'
    default:
      return 'bg-bg-card border-border text-text-muted'
  }
}

function formatDate(value: string) {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')} ${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`
}
</script>

<style scoped>
.modal-enter-active,
.modal-leave-active {
  transition: opacity 0.2s ease;
}

.modal-enter-from,
.modal-leave-to {
  opacity: 0;
}

.bg-terminal-bg {
  background-color: var(--color-terminal-bg);
}

.text-terminal-text {
  color: var(--color-terminal-text);
}
</style>
