<template>
  <el-card v-if="parsed" shadow="never" :body-style="{ padding: '16px' }">
    <template #header>
      <div class="flex items-center justify-between">
        <span class="text-sm font-semibold text-text-primary">本次运行指标</span>
        <span class="text-xs text-text-muted">
          计数器 {{ counterEntries.length }} 项 / 标签 {{ labelEntries.length }} 组
        </span>
      </div>
    </template>

    <div class="space-y-3">
      <div>
        <div class="text-xs text-text-muted mb-1.5">计数器</div>
        <div v-if="counterEntries.length === 0" class="text-xs text-text-muted">无</div>
        <div v-else class="grid grid-cols-2 md:grid-cols-3 gap-2">
          <div
            v-for="[key, value] in counterEntries"
            :key="key"
            class="px-2.5 py-2 rounded border border-border bg-bg-secondary/40"
          >
            <div class="text-xs text-text-muted truncate" :title="key">{{ metricLabel(key) }}</div>
            <div class="text-sm font-mono text-text-primary">{{ value }}</div>
          </div>
        </div>
      </div>

      <div v-if="labelEntries.length > 0">
        <div class="text-xs text-text-muted mb-1.5">标签分布</div>
        <div class="grid grid-cols-2 md:grid-cols-3 gap-2">
          <div
            v-for="[key, value] in labelEntries"
            :key="key"
            class="px-2.5 py-2 rounded border border-border bg-bg-secondary/40"
          >
            <div class="text-xs text-text-muted truncate" :title="key">{{ key }}</div>
            <div class="text-sm font-mono text-text-primary">{{ value }}</div>
          </div>
        </div>
      </div>
    </div>
  </el-card>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { parseMetrics, metricLabel } from '@/utils/metrics'

const props = defineProps<{ metricsJson?: string | null }>()

const parsed = computed(() => parseMetrics(props.metricsJson))

function sortedEntries(map: Record<string, number>): [string, number][] {
  return Object.entries(map).sort((left, right) => left[0].localeCompare(right[0]))
}

const counterEntries = computed<[string, number][]>(() => sortedEntries(parsed.value?.counters ?? {}))
const labelEntries = computed<[string, number][]>(() => sortedEntries(parsed.value?.labels ?? {}))
</script>
