<template>
  <div class="route-loading-container">
    <div class="route-loading-spinner"></div>
  </div>
</template>

<script setup lang="ts">
// 路由懒加载时的全局 Loading 组件
// 复用项目现有的 spinner 样式
</script>

<style scoped lang="postcss">
@reference "../../styles/index.css";

.route-loading-container {
  @apply flex items-center justify-center min-h-[200px] w-full;
}

.route-loading-spinner {
  @apply w-8 h-8 rounded-full animate-spin;
  border: 2px solid var(--color-accent-primary);
  border-top-color: transparent;
}
</style>
