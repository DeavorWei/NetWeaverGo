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

<style scoped>
.device-progress-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.loading-state,
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 40px;
  color: var(--text-secondary, #8b949e);
}

.empty-state svg {
  margin-bottom: 12px;
  opacity: 0.5;
}

.empty-state p {
  margin: 0;
  font-size: 14px;
}

.spinner {
  width: 32px;
  height: 32px;
  border: 3px solid var(--border-color, #2d333b);
  border-top-color: var(--primary-color, #58a6ff);
  border-radius: 50%;
  animation: spin 1s linear infinite;
  margin-bottom: 12px;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

.devices-container {
  display: flex;
  flex-direction: column;
  gap: 8px;
  max-height: 400px;
  overflow-y: auto;
}

.device-card {
  padding: 12px;
  background: var(--bg-secondary, #161b22);
  border: 1px solid var(--border-color, #2d333b);
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s ease;
}

.device-card:hover {
  border-color: var(--primary-color, #58a6ff);
  background: var(--bg-tertiary, #21262d);
}

.device-card.selected {
  border-color: var(--primary-color, #58a6ff);
  background: rgba(88, 166, 255, 0.1);
  box-shadow: 0 0 0 2px rgba(88, 166, 255, 0.3);
}

.device-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.device-info {
  display: flex;
  align-items: center;
  gap: 8px;
}

.device-ip {
  font-weight: 600;
  color: var(--text-primary, #e6edf3);
  font-family: monospace;
}

.device-status-badge {
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 11px;
  font-weight: 500;
}

.device-status-badge.status-pending {
  background: rgba(139, 148, 158, 0.2);
  color: #8b949e;
}

.device-status-badge.status-running {
  background: rgba(88, 166, 255, 0.2);
  color: #58a6ff;
}

.device-status-badge.status-completed {
  background: rgba(35, 134, 54, 0.2);
  color: #3fb950;
}

.device-status-badge.status-failed,
.device-status-badge.status-cancelled {
  background: rgba(218, 54, 51, 0.2);
  color: #f85149;
}

.device-status-badge.status-partial {
  background: rgba(158, 106, 3, 0.2);
  color: #d29922;
}

.device-progress-text {
  font-size: 12px;
  color: var(--text-secondary, #8b949e);
  font-family: monospace;
}

.progress-bar-container {
  height: 6px;
  background: var(--bg-tertiary, #21262d);
  border-radius: 3px;
  overflow: hidden;
  margin-bottom: 8px;
}

.progress-bar {
  height: 100%;
  border-radius: 3px;
  transition: width 0.3s ease;
}

.progress-bar.status-pending {
  background: #8b949e;
}

.progress-bar.status-running {
  background: #58a6ff;
}

.progress-bar.status-completed {
  background: #3fb950;
}

.progress-bar.status-failed,
.progress-bar.status-cancelled {
  background: #f85149;
}

.progress-bar.status-partial {
  background: #d29922;
}

.device-error {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px;
  background: rgba(218, 54, 51, 0.1);
  border-radius: 4px;
  margin-top: 8px;
  font-size: 12px;
  color: #f85149;
}

.device-time-info {
  display: flex;
  gap: 16px;
  margin-top: 8px;
  font-size: 11px;
  color: var(--text-secondary, #8b949e);
}
</style>
