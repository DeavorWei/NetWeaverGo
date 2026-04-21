<script setup lang="ts">
/**
 * 主题切换组件
 * Theme Switch Component
 * 
 * 提供主题切换功能，支持明亮/黑暗模式切换
 */
import { useTheme } from '@/composables/useTheme'

const { isDark, toggleTheme } = useTheme()
</script>

<template>
  <button
    @click="toggleTheme"
    class="group flex items-center justify-center w-9 h-9 rounded-lg border border-border-default bg-bg-tertiary hover:bg-bg-hover text-text-muted hover:text-text-primary transition-all duration-200 shadow-sm"
    :title="isDark ? '切换到浅色模式' : '切换到深色模式'"
    :aria-label="isDark ? '切换到浅色模式' : '切换到深色模式'"
  >
    <!-- 太阳图标 (黑暗模式时显示) -->
    <svg
      v-if="isDark"
      xmlns="http://www.w3.org/2000/svg"
      class="w-[18px] h-[18px] transition-transform duration-300 group-hover:rotate-[15deg]"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      stroke-width="2"
      stroke-linecap="round"
      stroke-linejoin="round"
    >
      <circle cx="12" cy="12" r="5" />
      <line x1="12" y1="1" x2="12" y2="3" />
      <line x1="12" y1="21" x2="12" y2="23" />
      <line x1="4.22" y1="4.22" x2="5.64" y2="5.64" />
      <line x1="18.36" y1="18.36" x2="19.78" y2="19.78" />
      <line x1="1" y1="12" x2="3" y2="12" />
      <line x1="21" y1="12" x2="23" y2="12" />
      <line x1="4.22" y1="19.78" x2="5.64" y2="18.36" />
      <line x1="18.36" y1="5.64" x2="19.78" y2="4.22" />
    </svg>

    <!-- 月亮图标 (明亮模式时显示) -->
    <svg
      v-else
      xmlns="http://www.w3.org/2000/svg"
      class="w-[18px] h-[18px] transition-transform duration-300 group-hover:rotate-[15deg]"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      stroke-width="2"
      stroke-linecap="round"
      stroke-linejoin="round"
    >
      <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z" />
    </svg>
  </button>
</template>

<!-- 旋转动画已迁移至 Tailwind group-hover 类，无需 style 块 -->
