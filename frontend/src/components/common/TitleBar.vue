<template>
  <div 
    class="titlebar" 
    :style="{ '--wails-draggable': isMaximized ? 'no-drag' : 'drag' }"
    @dblclick="toggleMaximize" 
    @mousedown="handleDragStart"
  >
    <!-- 左侧：Logo 和标题 -->
    <div class="titlebar-left">
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
    </div>

    <!-- 右侧：窗口控制按钮 -->
    <div class="titlebar-controls">
      <button class="titlebar-btn" @click.stop="minimize" title="最小化">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M5 12h14" />
        </svg>
      </button>
      <button class="titlebar-btn" @click.stop="toggleMaximize" :title="isMaximized ? '还原' : '最大化'">
        <svg v-if="isMaximized" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <rect x="5" y="9" width="10" height="10" rx="1" />
          <path d="M9 9V5a1 1 0 0 1 1-1h9a1 1 0 0 1 1 1v9a1 1 0 0 1-1 1h-4" />
        </svg>
        <svg v-else viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <rect x="4" y="4" width="16" height="16" rx="1" />
        </svg>
      </button>
      <button class="titlebar-btn titlebar-btn-close" @click.stop="close" title="关闭">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M18 6L6 18M6 6l12 12" />
        </svg>
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from "vue";
import { Window } from "@wailsio/runtime";

const isMaximized = ref(false);

// 保存窗口还原时的尺寸和位置
interface WindowState {
  width: number;
  height: number;
  x: number;
  y: number;
}

const normalWindowState = ref<WindowState | null>(null);

// 检查窗口是否已最大化
onMounted(async () => {
  try {
    isMaximized.value = await Window.IsMaximised();
    // 如果窗口不是最大化状态，保存当前窗口状态
    if (!isMaximized.value) {
      await saveWindowState();
    }
  } catch (e) {
    console.warn("Failed to check window maximize state:", e);
  }
});

// 保存当前窗口状态（尺寸和位置）
async function saveWindowState() {
  try {
    const size = await Window.Size();
    const position = await Window.Position();
    normalWindowState.value = {
      width: size.width,
      height: size.height,
      x: position.x,
      y: position.y,
    };
  } catch (e) {
    console.warn("Failed to save window state:", e);
  }
}

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
      // 最大化前保存当前窗口状态
      await saveWindowState();
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

// 处理拖拽开始 - 实现从最大化状态拖拽时自动还原
async function handleDragStart(event: MouseEvent) {
  // 如果点击的是控制按钮区域，不处理
  if ((event.target as HTMLElement).closest(".titlebar-controls")) {
    return;
  }

  try {
    const maximised = await Window.IsMaximised();
    
    if (maximised) {
      // 阻止默认拖拽行为
      event.preventDefault();
      
      // 窗口处于最大化状态，需要还原
      isMaximized.value = false;
      
      // 先还原窗口
      await Window.UnMaximise();
      
      // 获取当前窗口尺寸
      const currentSize = await Window.Size();
      const windowWidth = currentSize.width;
      const windowHeight = currentSize.height;
      
      // 获取屏幕尺寸
      const screenWidth = window.screen.width;
      
      // 计算新的窗口位置，使窗口水平居中于鼠标
      const mouseX = event.screenX;
      const mouseY = event.screenY;
      
      let newX = Math.max(0, Math.min(screenWidth - windowWidth, mouseX - windowWidth / 2));
      let newY = Math.max(0, mouseY - 16); // 稍微偏移，让标题栏在鼠标下方
      
      // 设置窗口位置和大小
      await Window.SetSize(windowWidth, windowHeight);
      await Window.SetPosition(newX, newY);
      
      // 更新保存的状态
      normalWindowState.value = {
        width: windowWidth,
        height: windowHeight,
        x: newX,
        y: newY,
      };
      
      // 还原后，标题栏的 CSS 会自动变为可拖拽
      // 由于我们已经处理了 mousedown，需要手动触发拖拽
      // 使用 Restore 方法可以让系统继续处理拖拽
    }
    // 如果不是最大化状态，CSS --wails-draggable: drag 会自动处理拖拽
  } catch (e) {
    console.warn("Failed to handle drag start:", e);
  }
}

// 监听窗口状态变化（用户通过其他方式还原窗口）
onMounted(() => {
  // 定期检查窗口状态
  const checkInterval = setInterval(async () => {
    try {
      const currentMaximized = await Window.IsMaximised();
      if (isMaximized.value !== currentMaximized) {
        isMaximized.value = currentMaximized;
        if (!currentMaximized) {
          await saveWindowState();
        }
      }
    } catch (e) {
      // 忽略错误
    }
  }, 500);
  
  // 保存 interval ID 以便清理
  (window as any).__windowCheckInterval = checkInterval;
});

onUnmounted(() => {
  const checkInterval = (window as any).__windowCheckInterval;
  if (checkInterval) {
    clearInterval(checkInterval);
  }
});
</script>

<style scoped lang="postcss">
@reference "../../styles/index.css";

.titlebar {
  @apply flex items-center justify-between h-8 pl-3;
  @apply bg-bg-secondary border-b border-border-muted;
  @apply select-none shrink-0 cursor-default;
}

.titlebar-left {
  @apply flex items-center gap-2;
}

.titlebar-logo {
  @apply flex items-center justify-center w-5 h-5 rounded;
  @apply bg-accent text-white;
}

.titlebar-logo svg {
  @apply w-3 h-3;
}

.titlebar-title {
  @apply text-xs font-medium text-text-secondary tracking-wide;
}

.titlebar-controls {
  @apply flex items-stretch h-full;
  --wails-draggable: no-drag;
}

.titlebar-btn {
  @apply flex items-center justify-center w-[46px] h-full p-0;
  @apply border-none bg-transparent text-text-secondary;
  @apply cursor-pointer transition-colors duration-150;
}

.titlebar-btn svg {
  @apply w-4 h-4;
}

.titlebar-btn:hover {
  @apply bg-bg-hover text-text-primary;
}

.titlebar-btn-close:hover {
  @apply bg-window-close text-white;
}
</style>
