<template>
  <div class="border border-border rounded p-3 bg-bg-secondary/40 mb-3">
    <div class="flex items-center justify-between mb-2">
      <div class="flex items-center gap-3">
        <span class="font-medium text-xs text-text-primary">规则树规则定义 (Tree Rules)</span>
        <div class="flex items-center gap-1.5">
          <span class="text-xs text-text-muted">拍平最大深度 (MaxLevel):</span>
          <el-input-number
            :model-value="maxLevel"
            @update:model-value="$emit('update:maxLevel', $event ?? 0)"
            :min="0" :max="10" size="small" controls-position="right" class="w-20"
          />
          <span class="text-[11px] text-text-muted">(0 为不限深度)</span>
        </div>
      </div>
      <el-button type="primary" size="small" link @click="$emit('add')">+ 添加规则项</el-button>
    </div>

    <el-table :data="rules" size="small" border class="w-full" max-height="260">
      <el-table-column label="字段名" min-width="95">
        <template #default="{ row }"><el-input v-model="row.parseItem" size="small" placeholder="如 slot" /></template>
      </el-table-column>
      <el-table-column label="父字段" min-width="85">
        <template #default="{ row }"><el-input v-model="row.parentItem" size="small" placeholder="空为根" /></template>
      </el-table-column>
      <el-table-column label="列表?" width="55" align="center">
        <template #default="{ row }"><el-checkbox v-model="row.isList" /></template>
      </el-table-column>
      <el-table-column label="分块正则 (SplitRegex)" min-width="135">
        <template #default="{ row }">
          <el-input v-model="row.splitRegex" size="small" placeholder="如 (?m)^Slot\s+(\d+):" />
          <div v-if="checkUnsupportedRegex(row.splitRegex)" class="text-[10px] text-amber-400">⚠ 包含不支持的前瞻/后顾</div>
        </template>
      </el-table-column>
      <el-table-column label="取值正则 (ParseRegex)" min-width="135">
        <template #default="{ row }">
          <el-input v-model="row.parseRegex" size="small" placeholder="如 Type:\s*(\S+)" />
          <div v-if="checkUnsupportedRegex(row.parseRegex)" class="text-[10px] text-amber-400">⚠ 包含不支持的前瞻/后顾</div>
        </template>
      </el-table-column>
      <el-table-column label="组号" width="60" align="center">
        <template #default="{ row }">
          <el-input-number v-model="row.groupIndex" :min="1" :max="9" size="small" controls-position="right" class="w-full" />
        </template>
      </el-table-column>
      <el-table-column label="输出?" width="55" align="center">
        <template #default="{ row }"><el-checkbox v-model="row.isOutput" /></template>
      </el-table-column>
      <el-table-column label="默认值" min-width="70">
        <template #default="{ row }"><el-input v-model="row.defaultValue" size="small" placeholder="空或N/A" /></template>
      </el-table-column>
      <el-table-column label="操作" width="50" align="center">
        <template #default="{ $index }">
          <el-button type="danger" link size="small" @click="$emit('remove', $index)">删</el-button>
        </template>
      </el-table-column>
    </el-table>

    <div class="text-[11px] text-text-muted mt-2">
      💡 规则说明：分块正则 (SplitRegex) 匹配各块头；取值正则 (ParseRegex) 匹配属性；组号指定抽取第几捕获组（缺省 1）；Go RE2 建议使用 <code class="text-amber-300">(?m)</code> 开启多行，不支持正向/负向预查。
    </div>
  </div>
</template>

<script setup lang="ts">
import type { TreeRule } from '@/services/parseTemplateApi'

defineProps<{
  rules: TreeRule[]
  maxLevel: number
}>()

defineEmits<{
  (e: 'update:maxLevel', v: number): void
  (e: 'add'): void
  (e: 'remove', index: number): void
}>()

function checkUnsupportedRegex(pat?: string): boolean {
  if (!pat) return false
  return /\(\?[<>=!]/.test(pat)
}
</script>
