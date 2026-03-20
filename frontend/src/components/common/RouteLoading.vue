<template>
  <div class="route-loading-container">
    <div class="route-loading-spinner"></div>
  </div>
</template>

<script setup lang="ts">
// 路由懒加载时的全局 Loading 组件
// 复用项目现有的 spinner 样式
</script>

<style scoped>
.route-loading-container {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 200px;
  width: 100%;
}

.route-loading-spinner {
  width: 2rem;
  height: 2rem;
  border: 2px solid var(--accent, #3b82f6);
  border-top-color: transparent;
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}
</style>
