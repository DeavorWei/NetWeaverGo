<template>
  <div class="list-panel">
    <!-- 头部 -->
    <div class="list-panel-header">
      <span class="list-panel-title">{{ title }}</span>
      <span class="list-panel-count">{{ items.length }}</span>
    </div>

    <!-- 列表 -->
    <div class="list-panel-body scrollbar-custom">
      <div
        v-for="item in items"
        :key="item.key"
        v-memo="[selected.has(item.key)]"
        @click="!item.disabled && $emit('toggle', item.key)"
        :class="[
          'list-item',
          {
            selected: selected.has(item.key),
            disabled: item.disabled,
          },
        ]"
      >
        <!-- 复选框 -->
        <div
          :class="[
            'w-4 h-4 rounded border flex items-center justify-center transition-colors shrink-0',
            selected.has(item.key)
              ? 'bg-accent border-accent'
              : 'border-border',
          ]"
        >
          <svg
            v-if="selected.has(item.key)"
            class="w-3 h-3 text-white"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="3"
          >
            <polyline points="20 6 9 17 4 12" />
          </svg>
        </div>

        <!-- 内容 -->
        <div class="flex-1 min-w-0">
          <div class="text-sm text-text-primary truncate">{{ item.label }}</div>
          <div v-if="item.description" class="text-xs text-text-muted truncate">
            {{ item.description }}
          </div>
        </div>

        <!-- 标签 -->
        <div v-if="item.tags?.length" class="flex gap-1 shrink-0">
          <span
            v-for="tag in item.tags"
            :key="tag"
            class="text-[10px] px-1.5 py-0.5 rounded bg-accent/10 text-accent"
          >
            {{ tag }}
          </span>
        </div>
      </div>

      <!-- 空状态 -->
      <div
        v-if="items.length === 0"
        class="flex flex-col items-center justify-center h-full text-text-muted"
      >
        <svg
          class="w-12 h-12 mb-2 opacity-30"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="1"
        >
          <rect x="3" y="3" width="18" height="18" rx="2" />
          <path d="M9 12l2 2 4-4" />
        </svg>
        <span class="text-sm">暂无数据</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { ListItem, ItemKey } from "../types/dualListSelector";

interface Props {
  title: string;
  items: ListItem[];
  selected: Set<ItemKey>;
  stats: { total: number; filtered: number; selected: number };
}

defineProps<Props>();

defineEmits<{
  toggle: [key: ItemKey];
}>();
</script>

<style scoped>
.list-panel {
  @apply flex flex-col w-[340px] h-[350px] bg-bg-panel border border-border rounded-lg overflow-hidden;
}

.list-panel-header {
  @apply flex items-center justify-between px-4 py-3 border-b border-border bg-bg-card;
}

.list-panel-title {
  @apply text-sm font-medium text-text-primary;
}

.list-panel-count {
  @apply text-xs text-text-muted bg-bg-secondary px-2 py-0.5 rounded-full;
}

.list-panel-body {
  @apply flex-1 overflow-y-auto scrollbar-custom p-2;
}

.list-item {
  @apply flex items-center gap-3 px-3 py-2.5 rounded-md cursor-pointer transition-all duration-200;
  @apply hover:bg-bg-hover;
}

.list-item.selected {
  @apply bg-accent/5 border-l-2 border-l-accent;
}

.list-item.disabled {
  @apply opacity-50 cursor-not-allowed;
}
</style>
