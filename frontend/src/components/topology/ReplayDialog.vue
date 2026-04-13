<template>
  <div
    v-if="visible"
    class="fixed inset-0 bg-black/50 flex items-center justify-center z-50"
    @click.self="handleClose"
  >
    <div class="bg-bg-card border border-border rounded-xl p-6 w-[520px] max-h-[80vh] overflow-auto">
      <!-- 标题 -->
      <div class="flex items-center justify-between mb-4">
        <h3 class="text-lg font-semibold text-text-primary">离线重放配置</h3>
        <button
          @click="handleClose"
          class="text-text-muted hover:text-text-primary"
          :disabled="replaying"
        >
          <svg class="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor">
            <path d="M6 18L18 6M6 6l12 12" stroke-width="2" stroke-linecap="round" />
          </svg>
        </button>
      </div>

      <!-- 原始运行信息 -->
      <div class="mb-4 p-3 bg-bg-panel rounded-lg">
        <div class="text-xs text-text-muted mb-1">原始运行ID</div>
        <div class="text-text-primary font-mono text-sm break-all">{{ originalRunId }}</div>
        <div v-if="runInfo" class="mt-2 text-xs text-text-muted">
          设备数: {{ runInfo.deviceCount }} | Raw文件: {{ runInfo.rawFileCount }}
        </div>
      </div>

      <!-- 重放选项 -->
      <div class="space-y-4 mb-6">
        <div>
          <label class="block text-sm text-text-secondary mb-1">解析器版本</label>
          <select
            v-model="options.parserVersion"
            class="w-full px-3 py-2 rounded-lg bg-bg-panel border border-border text-sm text-text-primary"
            :disabled="replaying"
          >
            <option value="">当前版本</option>
          </select>
          <p class="text-xs text-text-muted mt-1">留空使用当前解析器版本</p>
        </div>

        <div class="flex items-center gap-2">
          <input
            type="checkbox"
            id="clearExisting"
            v-model="options.clearExisting"
            class="rounded border-border bg-bg-panel"
            :disabled="replaying"
          />
          <label for="clearExisting" class="text-sm text-text-secondary">
            清除现有解析结果后重新构建
          </label>
        </div>

        <div class="flex items-center gap-2">
          <input
            type="checkbox"
            id="skipBuild"
            v-model="options.skipBuild"
            class="rounded border-border bg-bg-panel"
            :disabled="replaying"
          />
          <label for="skipBuild" class="text-sm text-text-secondary">
            跳过构建阶段（仅执行解析）
          </label>
        </div>
      </div>

      <!-- 进度显示 -->
      <div v-if="replaying" class="mb-6">
        <div class="flex items-center justify-between mb-2">
          <span class="text-sm text-text-secondary">{{ phaseLabel }}</span>
          <span class="text-sm text-text-primary">{{ progress }}%</span>
        </div>
        <div class="w-full bg-bg-panel rounded-full h-2 overflow-hidden">
          <div
            class="bg-accent h-2 rounded-full transition-all duration-300"
            :style="{ width: `${progress}%` }"
          ></div>
        </div>
        <p class="text-xs text-text-muted mt-2">{{ message }}</p>
      </div>

      <!-- 结果显示 -->
      <div v-if="result && !replaying" class="mb-6">
        <div
          :class="[
            'p-3 rounded-lg',
            result.status === 'completed' ? 'bg-success/10 border border-success/30' : '',
            result.status === 'failed' ? 'bg-error/10 border border-error/30' : '',
            result.status === 'cancelled' ? 'bg-warning/10 border border-warning/30' : ''
          ]"
        >
          <div class="flex items-center gap-2 mb-2">
            <svg
              v-if="result.status === 'completed'"
              class="w-5 h-5 text-success"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
            >
              <path d="M5 13l4 4L19 7" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" />
            </svg>
            <svg
              v-else-if="result.status === 'failed'"
              class="w-5 h-5 text-error"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
            >
              <circle cx="12" cy="12" r="10" stroke-width="2" />
              <path d="M15 9l-6 6M9 9l6 6" stroke-width="2" stroke-linecap="round" />
            </svg>
            <span
              :class="[
                'text-sm font-medium',
                result.status === 'completed' ? 'text-success' : '',
                result.status === 'failed' ? 'text-error' : '',
                result.status === 'cancelled' ? 'text-warning' : ''
              ]"
            >
              {{ result.status === 'completed' ? '重放完成' : result.status === 'failed' ? '重放失败' : '已取消' }}
            </span>
          </div>
          
          <!-- 统计信息 -->
          <div v-if="result.status === 'completed'" class="text-xs text-text-muted space-y-1">
            <div class="grid grid-cols-2 gap-2">
              <div>解析设备: {{ result.statistics.parsedDevices }}</div>
              <div>LLDP邻居: {{ result.statistics.lldpCount }}</div>
              <div>FDB条目: {{ result.statistics.fdbCount }}</div>
              <div>ARP条目: {{ result.statistics.arpCount }}</div>
              <div>拓扑边: {{ result.statistics.retainedEdges }}</div>
              <div>耗时: {{ result.statistics.totalDurationMs }}ms</div>
            </div>
          </div>
          
          <!-- 错误信息 -->
          <div v-if="result.errors && result.errors.length > 0" class="mt-2">
            <div class="text-xs text-error/80 font-medium mb-1">错误信息:</div>
            <div class="max-h-24 overflow-auto text-xs text-error/70 space-y-1">
              <div v-for="(err, idx) in result.errors" :key="idx" class="font-mono">
                {{ err }}
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- 操作按钮 -->
      <div class="flex justify-end gap-2">
        <button
          @click="handleClose"
          :disabled="replaying"
          class="px-4 py-2 rounded-lg text-sm font-medium border border-border text-text-secondary hover:text-text-primary disabled:opacity-50"
        >
          {{ result ? '关闭' : '取消' }}
        </button>
        <button
          v-if="!result"
          @click="handleStartReplay"
          :disabled="replaying"
          class="px-4 py-2 rounded-lg text-sm font-medium bg-accent text-white hover:bg-accent/90 disabled:opacity-50"
        >
          {{ replaying ? '重放中...' : '开始重放' }}
        </button>
        <button
          v-else-if="result.status === 'completed'"
          @click="handleViewResult"
          class="px-4 py-2 rounded-lg text-sm font-medium bg-accent text-white hover:bg-accent/90"
        >
          查看结果
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useTopologyReplay } from '@/composables/useTopologyReplay'
import type { ReplayOptions, ReplayableRunInfo } from '@/types/taskexec'

// Props
const props = defineProps<{
  visible: boolean
  originalRunId: string
  runInfo?: ReplayableRunInfo | null
}>()

// Emits
const emit = defineEmits<{
  (e: 'close'): void
  (e: 'complete', result: { replayRunId: string }): void
}>()

// Composable
const {
  replaying,
  replayProgress: progress,
  replayPhase: phase,
  replayMessage: message,
  replayResult: result,
  replayTopology,
  resetState
} = useTopologyReplay()

// 选项
const options = ref<ReplayOptions>({
  clearExisting: false,
  parserVersion: '',
  deviceIps: [],
  skipBuild: false
})

// 阶段标签
const phaseLabel = computed(() => {
  switch (phase.value) {
    case 'scan':
      return '扫描Raw文件'
    case 'parse':
      return '解析数据'
    case 'build':
      return '构建拓扑'
    case 'finalize':
      return '完成处理'
    default:
      return '处理中'
  }
})

// 监听visible变化重置状态
watch(
  () => props.visible,
  (newVal) => {
    if (newVal) {
      resetState()
      result.value = null
    }
  }
)

// 开始重放
const handleStartReplay = async () => {
  if (!props.originalRunId) return
  
  const res = await replayTopology(props.originalRunId, options.value)
  
  if (res.status === 'completed') {
    // 重放成功，通知父组件
    emit('complete', { replayRunId: res.replayRunId })
  }
}

// 关闭对话框
const handleClose = () => {
  if (replaying.value) return
  emit('close')
}

// 查看结果
const handleViewResult = () => {
  if (result.value) {
    emit('complete', { replayRunId: result.value.replayRunId })
    emit('close')
  }
}
</script>
