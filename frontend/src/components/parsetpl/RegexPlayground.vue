<template>
  <div class="border border-border rounded p-3 bg-bg-secondary/40 mb-3 space-y-2">
    <div class="flex items-center justify-between">
      <span class="font-medium text-xs text-text-primary">单条正则匹配定义</span>
      <el-checkbox :model-value="multiline" @update:model-value="$emit('update:multiline', !!$event)">启用多行匹配 (Multiline)</el-checkbox>
    </div>
    <el-form-item label="正则模式" class="!mb-0">
      <el-input
        :model-value="pattern"
        @update:model-value="$emit('update:pattern', $event)"
        type="textarea"
        :rows="3"
        placeholder="如 (?m)^(?P<interface>\S+)\s+(?P<status>UP|DOWN)\s+(?P<protocol>UP|DOWN)"
      />
    </el-form-item>
    <div v-if="checkUnsupportedRegex(pattern)" class="text-[11px] text-amber-400">
      ⚠ 检测到模式中包含 Go RE2 不支持的前瞻 (?= 或后顾 (?<= 语法，请改写为捕获组。
    </div>
    <div class="text-[11px] text-text-muted">
      💡 支持 Go 语法命名捕获组 <code class="text-amber-300">(?P&lt;name&gt;...)</code> 或顺序捕获组（无命名组将按 "1", "2" 数字键输出）。
    </div>
  </div>
</template>

<script setup lang="ts">
defineProps<{
  pattern: string
  multiline: boolean
}>()

defineEmits<{
  (e: 'update:pattern', v: string): void
  (e: 'update:multiline', v: boolean): void
}>()

function checkUnsupportedRegex(pat?: string): boolean {
  if (!pat) return false
  return /\(\?[<>=!]/.test(pat)
}
</script>
