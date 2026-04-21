<template>
  <div class="device-progress-list">
    <!-- 加载状态 -->
    <div v-if="loading" class="loading-state">
      <div class="spinner"></div>
      <span>加载设备执行详情...</span>
    </div>

    <!-- 空状态 -->
    <div v-else-if="devices.length === 0" class="empty-state">
      <svg
        viewBox="0 0 24 24"
        width="48"
        height="48"
        fill="none"
        stroke="currentColor"
        stroke-width="1.5"
      >
        <rect x="2" y="3" width="20" height="14" rx="2" ry="2"></rect>
        <line x1="8" y1="21" x2="16" y2="21"></line>
        <line x1="12" y1="17" x2="12" y2="21"></line>
      </svg>
      <p>暂无设备执行记录</p>
    </div>

    <!-- 设备列表 -->
    <div v-else class="devices-container">
      <div
        v-for="device in devices"
        :key="device.unitId"
        class="device-card"
        :class="[
          `status-${device.status}`,
          { selected: selectedDevice?.unitId === device.unitId },
        ]"
        @click="selectDevice(device)"
      >
        <!-- 设备头部信息 -->
        <div class="device-header">
          <div class="device-info">
            <span class="device-ip">{{ device.deviceIp }}</span>
            <span
              class="device-status-badge"
              :class="`status-${device.status}`"
            >
              {{ getStatusText(device.status) }}
            </span>
          </div>
          <div class="device-progress-text">
            {{ device.doneSteps }}/{{ device.totalSteps }}
          </div>
        </div>

        <!-- 进度条 -->
        <div class="progress-bar-container">
          <div
            class="progress-bar"
            :style="{ width: `${device.progress}%` }"
            :class="`status-${device.status}`"
          ></div>
        </div>

        <!-- 错误信息 -->
        <div v-if="device.errorMessage" class="device-error">
          <svg
            viewBox="0 0 24 24"
            width="12"
            height="12"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
          >
            <circle cx="12" cy="12" r="10"></circle>
            <line x1="12" y1="8" x2="12" y2="12"></line>
            <line x1="12" y1="16" x2="12.01" y2="16"></line>
          </svg>
          <span>{{ device.errorMessage }}</span>
        </div>

        <!-- 时间信息 -->
        <div class="device-time-info">
          <span v-if="device.startedAt"
            >开始: {{ formatTime(device.startedAt) }}</span
          >
          <span v-if="device.durationMs"
            >耗时: {{ formatDuration(device.durationMs) }}</span
          >
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from "vue";
import type { DeviceExecutionView } from "../../types/executionHistory";

const props = defineProps<{
  runId: string;
  devices: DeviceExecutionView[];
  loading: boolean;
}>();

const emit = defineEmits<{
  (e: "select", device: DeviceExecutionView): void;
}>();

const selectedDevice = ref<DeviceExecutionView | null>(null);

// 选中设备
const selectDevice = (device: DeviceExecutionView) => {
  selectedDevice.value = device;
  emit("select", device);
};

// 获取状态文本
const getStatusText = (status: string): string => {
  const statusMap: Record<string, string> = {
    pending: "等待中",
    running: "执行中",
    completed: "已完成",
    failed: "失败",
    cancelled: "已取消",
    partial: "部分完成",
  };
  return statusMap[status] || status;
};

// 格式化时间
const formatTime = (timeStr: string): string => {
  if (!timeStr) return "-";
  const date = new Date(timeStr);
  return date.toLocaleString("zh-CN", {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
};

// 格式化时长
const formatDuration = (ms: number): string => {
  if (!ms || ms < 0) return "-";
  const seconds = Math.floor(ms / 1000);
  const minutes = Math.floor(seconds / 60);
  const hours = Math.floor(minutes / 60);

  if (hours > 0) {
    return `${hours}h ${minutes % 60}m`;
  } else if (minutes > 0) {
    return `${minutes}m ${seconds % 60}s`;
  } else {
    return `${seconds}s`;
  }
};
</script>

<style scoped lang="postcss">
@reference "../../styles/index.css";

/* 容器 */
.device-progress-list {
  @apply flex flex-col gap-3;
}

/* 加载/空状态 */
.loading-state,
.empty-state {
  @apply flex flex-col items-center justify-center p-10 text-text-muted;
}

.empty-state svg {
  @apply mb-3 opacity-50;
}

.empty-state p {
  @apply m-0 text-sm;
}

/* 加载动画 */
.spinner {
  @apply w-8 h-8 mb-3 rounded-full animate-spin;
  border: 3px solid var(--color-border-default);
  border-top-color: var(--color-accent-primary);
}

/* 设备列表容器 */
.devices-container {
  @apply flex flex-col gap-2 max-h-[400px] overflow-y-auto;
}

/* 设备卡片 */
.device-card {
  @apply p-3 bg-bg-secondary border border-border rounded-lg cursor-pointer;
  @apply transition-all duration-200;
}

.device-card:hover {
  @apply border-accent bg-bg-tertiary;
}

.device-card.selected {
  @apply border-accent;
  background: var(--color-accent-bg);
  box-shadow: 0 0 0 2px var(--color-accent-glow);
}

/* 设备头部 */
.device-header {
  @apply flex justify-between items-center mb-2;
}

.device-info {
  @apply flex items-center gap-2;
}

.device-ip {
  @apply font-semibold text-text-primary font-mono;
}

/* 状态徽章 */
.device-status-badge {
  @apply px-2 py-0.5 rounded text-[11px] font-medium;
}

.device-status-badge.status-pending {
  background: var(--color-bg-hover);
  @apply text-text-muted;
}

.device-status-badge.status-running {
  background: var(--color-accent-bg);
  @apply text-accent;
}

.device-status-badge.status-completed {
  background: var(--color-success-bg);
  @apply text-success;
}

.device-status-badge.status-failed,
.device-status-badge.status-cancelled {
  background: var(--color-error-bg);
  @apply text-error;
}

.device-status-badge.status-partial {
  background: var(--color-warning-bg);
  @apply text-warning;
}

/* 进度文本 */
.device-progress-text {
  @apply text-xs text-text-secondary font-mono;
}

/* 进度条容器 */
.progress-bar-container {
  @apply h-1.5 bg-bg-tertiary rounded overflow-hidden mb-2;
}

/* 进度条 */
.progress-bar {
  @apply h-full rounded transition-[width] duration-300;
}

.progress-bar.status-pending {
  @apply bg-text-muted;
}

.progress-bar.status-running {
  @apply bg-accent;
}

.progress-bar.status-completed {
  @apply bg-success;
}

.progress-bar.status-failed,
.progress-bar.status-cancelled {
  @apply bg-error;
}

.progress-bar.status-partial {
  @apply bg-warning;
}

/* 错误信息 */
.device-error {
  @apply flex items-center gap-1.5 p-2 rounded mt-2 text-xs text-error;
  background: var(--color-error-bg);
}

/* 时间信息 */
.device-time-info {
  @apply flex gap-4 mt-2 text-[11px] text-text-secondary;
}
</style>
