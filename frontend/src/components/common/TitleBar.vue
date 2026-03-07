<template>
  <div class="titlebar" @dblclick="toggleMaximize">
    <!-- 左侧：Logo 和标题 -->
    <!-- <div class="titlebar-left">
      <div class="titlebar-logo">
        <svg
          xmlns="http://www.w3.org/2000/svg"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2.5"
          stroke-linecap="round"
          stroke-linejoin="round"
        >
          <circle cx="12" cy="12" r="3" />
          <path
            d="M12 2v3M12 19v3M4.22 4.22l2.12 2.12M17.66 17.66l2.12 2.12M2 12h3M19 12h3M4.22 19.78l2.12-2.12M17.66 6.34l2.12-2.12"
          />
        </svg>
      </div>
      <span class="titlebar-title">NetWeaverGo Control Center</span>
    </div> -->
<div class="titlebar-left">
  
</div>
    <!-- 右侧：窗口控制按钮 -->
    <div class="titlebar-controls">
      <button class="titlebar-btn" @click="minimize" title="最小化">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M5 12h14" />
        </svg>
      </button>
      <button class="titlebar-btn" @click="toggleMaximize" :title="isMaximized ? '还原' : '最大化'">
        <svg v-if="isMaximized" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <rect x="5" y="9" width="10" height="10" rx="1" />
          <path d="M9 9V5a1 1 0 0 1 1-1h9a1 1 0 0 1 1 1v9a1 1 0 0 1-1 1h-4" />
        </svg>
        <svg v-else viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <rect x="4" y="4" width="16" height="16" rx="1" />
        </svg>
      </button>
      <button class="titlebar-btn titlebar-btn-close" @click="close" title="关闭">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M18 6L6 18M6 6l12 12" />
        </svg>
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from "vue";
import { Window } from "@wailsio/runtime";

const isMaximized = ref(false);

// 检查窗口是否已最大化
onMounted(async () => {
  try {
    isMaximized.value = await Window.IsMaximised();
  } catch (e) {
    console.warn("Failed to check window maximize state:", e);
  }
});

// 最小化窗口
async function minimize() {
  try {
    await Window.Minimise();
  } catch (e) {
    console.warn("Failed to minimize window:", e);
  }
}

// 切换最大化状态
async function toggleMaximize() {
  try {
    if (isMaximized.value) {
      await Window.UnMaximise();
      isMaximized.value = false;
    } else {
      await Window.Maximise();
      isMaximized.value = true;
    }
  } catch (e) {
    console.warn("Failed to toggle maximize:", e);
  }
}

// 关闭窗口
async function close() {
  try {
    await Window.Close();
  } catch (e) {
    console.warn("Failed to close window:", e);
  }
}
</script>

<style scoped>
.titlebar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 32px;
  padding-left: 12px;
  background-color: var(--color-bg-secondary);
  border-bottom: 1px solid var(--color-border-muted);
  user-select: none;
  --wails-draggable: drag;
  flex-shrink: 0;
}

.titlebar-left {
  display: flex;
  align-items: center;
  gap: 8px;
}

.titlebar-logo {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  border-radius: 4px;
  background-color: var(--color-accent-primary);
  color: white;
}

.titlebar-logo svg {
  width: 12px;
  height: 12px;
}

.titlebar-title {
  font-size: 12px;
  font-weight: 500;
  color: var(--color-text-secondary);
  letter-spacing: 0.01em;
}

.titlebar-controls {
  display: flex;
  align-items: stretch;
  height: 100%;
  --wails-draggable: no-drag;
}

.titlebar-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 46px;
  height: 100%;
  padding: 0;
  border: none;
  background: transparent;
  color: var(--color-text-secondary);
  cursor: pointer;
  transition: background-color 150ms ease;
}

.titlebar-btn svg {
  width: 16px;
  height: 16px;
}

.titlebar-btn:hover {
  background-color: var(--color-bg-hover);
  color: var(--color-text-primary);
}

.titlebar-btn-close:hover {
  background-color: #e81123;
  color: #ffffff;
}
</style>
