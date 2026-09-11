<template>
  <div class="col-span-5 bg-bg-secondary rounded-lg border border-border p-4 flex flex-col min-h-0">
    <div class="flex items-center justify-between pb-3 border-b border-border">
      <span class="font-medium text-sm text-text-primary">模板列表 ({{ templates.length }})</span>
      <el-button size="small" link @click="$emit('refresh')">刷新</el-button>
    </div>
    <div class="flex-1 overflow-y-auto mt-2 space-y-2 pr-1">
      <div
        v-for="tpl in templates"
        :key="(tpl as any).id || (tpl as any).commandKey"
        @click="$emit('select', tpl)"
        :class="[
          'p-3 rounded border cursor-pointer transition-colors',
          selectedId === (tpl as any).id
            ? 'bg-blue-500/10 border-blue-500/50'
            : 'bg-bg-primary/50 border-border hover:border-text-muted/40'
        ]"
      >
        <div class="flex items-center justify-between">
          <span class="font-medium text-sm text-text-primary font-mono">{{ (tpl as any).commandKey }}</span>
          <div class="flex items-center gap-1.5">
            <span class="text-xs px-1.5 py-0.5 rounded font-mono" :class="engineBadgeClass((tpl as any).engine)">
              {{ (tpl as any).engine }}
            </span>
            <span v-if="(tpl as any).source" class="text-[10px] px-1 rounded bg-cyan-500/10 text-cyan-300 border border-cyan-500/20">
              {{ sourceLabel((tpl as any).source) }}
            </span>
            <span v-if="(tpl as any).enabled" class="text-xs text-emerald-400">● 启用</span>
            <span v-else class="text-xs text-text-muted">○ 停用</span>
          </div>
        </div>
        <div class="text-xs text-text-muted mt-1 truncate">
          {{ (tpl as any).description || '无描述' }}
        </div>
        <div v-if="appliesToText((tpl as any).appliesTo)" class="text-[11px] text-cyan-400 font-mono mt-1 truncate">
          <span class="px-1.5 py-0.5 rounded bg-cyan-500/10 border border-cyan-500/20">
            🎯 {{ appliesToText((tpl as any).appliesTo) }}
          </span>
        </div>
        <div class="flex justify-between items-center mt-2 pt-2 border-t border-border/50 text-xs">
          <span class="text-text-muted/60 font-mono text-[11px]">rev.{{ (tpl as any).revision ?? '-' }}</span>
          <div class="flex gap-2">
            <el-button size="small" link type="primary" @click.stop="$emit('edit', tpl)">编辑</el-button>
            <el-button size="small" link type="danger" @click.stop="$emit('delete', tpl)">删除</el-button>
          </div>
        </div>
      </div>
      <div v-if="templates.length === 0" class="text-center py-12 text-text-muted text-xs">
        当前厂商暂无自定义模板，采集时将采用系统内置模板。
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { UserParseTemplateVO } from '@/services/parseTemplateApi';

defineProps<{
  templates: UserParseTemplateVO[]
  selectedId?: number | null
}>()

defineEmits<{
  (e: 'refresh'): void
  (e: 'select', tpl: UserParseTemplateVO): void
  (e: 'edit', tpl: UserParseTemplateVO): void
  (e: 'delete', tpl: UserParseTemplateVO): void
}>()

function engineBadgeClass(engine: string): string {
  switch (engine) {
    case 'tree': return 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20'
    case 'aggregate': return 'bg-amber-500/10 text-amber-400 border border-amber-500/20'
    default: return 'bg-blue-500/10 text-blue-400 border border-blue-500/20'
  }
}

function sourceLabel(source: string): string {
  switch (source) {
    case 'builtin': return '内置'
    case 'override': return '覆盖内置'
    case 'user': return '自定义'
    default: return source
  }
}

function appliesToText(appliesTo?: { models?: string[]; versions?: string[] } | null): string {
  if (!appliesTo) return ''
  const parts: string[] = []
  if (appliesTo.models && appliesTo.models.length > 0) parts.push(`款型: ${appliesTo.models.join(', ')}`)
  if (appliesTo.versions && appliesTo.versions.length > 0) parts.push(`版本: ${appliesTo.versions.join(', ')}`)
  return parts.join(' | ')
}
</script>
