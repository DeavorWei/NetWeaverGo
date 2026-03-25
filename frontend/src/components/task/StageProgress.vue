<template>
  <div class="space-y-3">
    <!-- Stage 列表 -->
    <div
      v-for="stage in stages"
      :key="stage.id"
      class="bg-bg-card border border-border rounded-lg overflow-hidden"
      :class="{ 'border-accent/50': stage.status === 'running' }"
    >
      <!-- Stage 头部 -->
      <div class="px-4 py-3 flex items-center justify-between bg-bg-panel/50">
        <div class="flex items-center gap-3">
          <!-- Stage 序号 -->
          <div
            class="w-6 h-6 rounded-full flex items-center justify-center text-xs font-medium"
            :class="getStageNumberClass(stage)"
          >
            {{ stage.order }}
          </div>
          
          <!-- Stage 名称 -->
          <span class="font-medium text-text-primary">
            {{ StageKindNames[stage.kind] || stage.name }}
          </span>
          
          <!-- Stage 状态标签 -->
          <span
            class="px-2 py-0.5 rounded text-xs"
            :class="getStatusBadgeClass(stage.status)"
          >
            {{ StatusNames[stage.status] || stage.status }}
          </span>
        </div>
        
        <!-- 进度文本 -->
        <div class="text-sm text-text-muted">
          <span v-if="stage.totalUnits > 0">
            {{ stage.completedUnits }}/{{ stage.totalUnits }} 单元
          </span>
          <span v-else-if="stage.progress > 0">
            {{ stage.progress }}%
          </span>
        </div>
      </div>
      
      <!-- Stage 进度条 -->
      <div class="px-4 py-2">
        <div class="h-2 bg-bg-panel rounded-full overflow-hidden">
          <div
            class="h-full transition-all duration-300 rounded-full"
            :class="getProgressBarClass(stage.status)"
            :style="{ width: `${stage.progress}%` }"
          />
        </div>
      </div>
      
      <!-- Unit 列表 (仅展开运行中的 Stage) -->
      <div
        v-if="stage.status === 'running' && getStageUnits(stage.id).length > 0"
        class="px-4 pb-3"
      >
        <div class="space-y-1 mt-2">
          <div
            v-for="unit in getStageUnits(stage.id).slice(0, 5)"
            :key="unit.id"
            class="flex items-center gap-3 text-sm py-1"
          >
            <!-- Unit 状态图标 -->
            <div class="w-4 h-4 flex-shrink-0">
              <svg
                v-if="unit.status === 'completed'"
                class="w-4 h-4 text-success"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
              >
                <polyline points="20 6 9 17 4 12" />
              </svg>
              <svg
                v-else-if="unit.status === 'failed'"
                class="w-4 h-4 text-error"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
              >
                <circle cx="12" cy="12" r="10" />
                <line x1="15" y1="9" x2="9" y2="15" />
                <line x1="9" y1="9" x2="15" y2="15" />
              </svg>
              <svg
                v-else-if="unit.status === 'running'"
                class="w-4 h-4 text-accent animate-spin"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
              >
                <path d="M12 2v4m0 12v4M4.93 4.93l2.83 2.83m8.48 8.48l2.83 2.83M2 12h4m12 0h4M4.93 19.07l2.83-2.83m8.48-8.48l2.83-2.83" />
              </svg>
              <div
                v-else
                class="w-2 h-2 rounded-full bg-text-muted/30 mx-auto mt-1"
              />
            </div>
            
            <!-- Unit 目标 -->
            <span class="text-text-secondary truncate flex-1" :title="unit.targetKey">
              {{ unit.targetKey }}
            </span>
            
            <!-- Unit 进度 -->
            <span class="text-text-muted text-xs">
              {{ unit.doneSteps }}/{{ unit.totalSteps }}
            </span>
          </div>
          
          <!-- 更多 Unit 提示 -->
          <div
            v-if="getStageUnits(stage.id).length > 5"
            class="text-xs text-text-muted pl-7"
          >
            还有 {{ getStageUnits(stage.id).length - 5 }} 个单元...
          </div>
        </div>
      </div>
    </div>
    
    <!-- 无 Stage 提示 -->
    <div v-if="stages.length === 0" class="text-center py-8 text-text-muted">
      暂无阶段信息
    </div>
  </div>
</template>

<script setup lang="ts">
import type { StageSnapshot, UnitSnapshot } from '../../types/taskexec'
import { StageKindNames, StatusNames } from '../../types/taskexec'

const props = defineProps<{
  stages: StageSnapshot[]
  units: UnitSnapshot[]
}>()

// 获取 Stage 对应的 Units
const getStageUnits = (stageId: string): UnitSnapshot[] => {
  return props.units.filter(u => u.stageId === stageId)
}

// Stage 序号样式
const getStageNumberClass = (stage: StageSnapshot): string => {
  const baseClasses = 'text-xs font-bold'
  switch (stage.status) {
    case 'completed':
      return `${baseClasses} bg-success text-white`
    case 'running':
      return `${baseClasses} bg-accent text-white`
    case 'failed':
      return `${baseClasses} bg-error text-white`
    default:
      return `${baseClasses} bg-bg-panel text-text-muted border border-border`
  }
}

// 状态标签样式
const getStatusBadgeClass = (status: string): string => {
  const classes: Record<string, string> = {
    'pending': 'bg-bg-panel text-text-muted',
    'running': 'bg-accent/20 text-accent',
    'completed': 'bg-success/20 text-success',
    'partial': 'bg-warning/20 text-warning',
    'failed': 'bg-error/20 text-error',
    'cancelled': 'bg-bg-panel text-text-muted'
  }
  return classes[status] ?? 'bg-bg-panel text-text-muted'
}

// 进度条样式
const getProgressBarClass = (status: string): string => {
  const classes: Record<string, string> = {
    'pending': 'bg-text-muted/30',
    'running': 'bg-accent',
    'completed': 'bg-success',
    'partial': 'bg-warning',
    'failed': 'bg-error',
    'cancelled': 'bg-text-muted/30'
  }
  return classes[status] ?? 'bg-text-muted/30'
}
</script>
