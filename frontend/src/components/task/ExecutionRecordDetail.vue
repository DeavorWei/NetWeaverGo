<template>
  <Teleport to="body">
    <Transition name="modal">
      <div
        v-if="modelValue && record"
        class="record-detail-modal"
        @click.self="closeModal"
      >
        <div class="modal-content" @click.stop>
          <!-- 头部 -->
          <div class="modal-header">
            <h3 class="modal-title">执行详情</h3>
            <button class="btn-close" @click="closeModal">×</button>
          </div>

          <!-- 内容 -->
          <div class="modal-body">
            <!-- 基本信息 -->
            <div class="info-section">
              <div class="info-row">
                <span class="info-label">任务名称</span>
                <span class="info-value">{{ record.taskName }}</span>
              </div>
              <div class="info-row">
                <span class="info-label">执行模式</span>
                <span class="info-value">{{ getModeText(record.mode) }}</span>
              </div>
              <div class="info-row">
                <span class="info-label">执行状态</span>
                <span
                  class="info-value status"
                  :class="`status-${record.status}`"
                >
                  {{ getStatusText(record.status) }}
                </span>
              </div>
              <div class="info-row">
                <span class="info-label">开始时间</span>
                <span class="info-value">{{
                  formatTime(record.startedAt)
                }}</span>
              </div>
              <div class="info-row">
                <span class="info-label">结束时间</span>
                <span class="info-value">{{
                  formatTime(record.finishedAt)
                }}</span>
              </div>
              <div class="info-row">
                <span class="info-label">执行时长</span>
                <span class="info-value">{{
                  formatDuration(record.durationMs)
                }}</span>
              </div>
              <div class="info-row" v-if="record.reportPath">
                <span class="info-label">报告文件</span>
                <div class="info-value-with-action">
                  <span class="info-value report-path">{{
                    record.reportPath
                  }}</span>
                  <button
                    class="btn-open"
                    @click="openReportFile"
                    title="打开报告"
                  >
                    <svg
                      viewBox="0 0 24 24"
                      width="14"
                      height="14"
                      fill="none"
                      stroke="currentColor"
                      stroke-width="2"
                    >
                      <path
                        d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"
                      ></path>
                      <polyline points="15 3 21 3 21 9"></polyline>
                      <line x1="10" y1="14" x2="21" y2="3"></line>
                    </svg>
                  </button>
                </div>
              </div>
            </div>

            <!-- 统计信息 -->
            <div class="stats-section">
              <h4>执行统计</h4>
              <div class="stats-grid">
                <div class="stat-item">
                  <span class="stat-value">{{ record.totalDevices }}</span>
                  <span class="stat-label">设备总数</span>
                </div>
                <div class="stat-item success">
                  <span class="stat-value">{{ record.successCount }}</span>
                  <span class="stat-label">成功</span>
                </div>
                <div class="stat-item error" v-if="record.errorCount > 0">
                  <span class="stat-value">{{ record.errorCount }}</span>
                  <span class="stat-label">失败</span>
                </div>
                <div class="stat-item warning" v-if="(record.abortedCount || 0) > 0">
                  <span class="stat-value">{{ record.abortedCount || 0 }}</span>
                  <span class="stat-label">中止</span>
                </div>
                <div class="stat-item" v-if="(record.warningCount || 0) > 0">
                  <span class="stat-value">{{ record.warningCount || 0 }}</span>
                  <span class="stat-label">告警</span>
                </div>
              </div>
            </div>

            <!-- 设备列表 -->
            <div
              class="devices-section"
              v-if="record.devices && record.devices.length > 0"
            >
              <h4>设备执行明细</h4>
              <div class="device-list">
                <div
                  v-for="device in record.devices"
                  :key="device.ip"
                  class="device-item"
                  :class="`status-${getDeviceStatusClass(device.status)}`"
                >
                  <div class="device-header">
                    <span class="device-ip">{{ device.ip }}</span>
                    <div class="device-actions">
                      <button
                        v-if="resolveDetailLogPath(device)"
                        class="btn-open-log-icon"
                        @click="openDeviceDetailLog(device)"
                        title="打开详细日志"
                      >
                        <svg
                          viewBox="0 0 24 24"
                          width="12"
                          height="12"
                          fill="none"
                          stroke="currentColor"
                          stroke-width="2"
                        >
                          <path
                            d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"
                          ></path>
                          <polyline points="15 3 21 3 21 9"></polyline>
                          <line x1="10" y1="14" x2="21" y2="3"></line>
                        </svg>
                      </button>
                      <button
                        v-if="device.rawLogPath"
                        class="btn-open-log-icon"
                        @click="openRawLog(device)"
                        title="打开原始日志"
                      >
                        RAW
                      </button>
                      <span
                        class="device-status"
                        :class="`status-${getDeviceStatusClass(device.status)}`"
                      >
                        {{ device.status }}
                      </span>
                    </div>
                  </div>
                  <div class="device-info">
                    <span
                      >命令: {{ device.execCmd }}/{{ device.totalCmd }}</span
                    >
                    <span v-if="(device.logCount || 0) > 0"
                      >日志: {{ device.logCount }}条</span
                    >
                  </div>
                  <div v-if="device.errorMsg" class="device-error">
                    {{ device.errorMsg }}
                  </div>
                  <div
                    class="device-logs-header"
                    v-if="device.logTail && device.logTail.length > 0"
                  >
                    <span class="logs-label">日志预览</span>
                  </div>
                  <div
                    v-if="device.logTail && device.logTail.length > 0"
                    class="device-logs"
                  >
                    <div
                      v-for="(log, idx) in device.logTail"
                      :key="idx"
                      class="log-line"
                    >
                      {{ log }}
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { ExecutionHistoryAPI } from "../../services/api";
import type { ExecutionHistoryDeviceRecord, ExecutionHistoryRecord } from "../../types/executionHistory";
import { useToast } from "../../utils/useToast";

const props = defineProps<{
  modelValue: boolean;
  record: ExecutionHistoryRecord | null;
}>();

const emit = defineEmits<{
  (e: "update:modelValue", value: boolean): void;
  (e: "close"): void;
}>();

const toast = useToast();

const closeModal = () => {
  emit("update:modelValue", false);
  emit("close");
};

// 打开报告文件
const openReportFile = async () => {
  if (!props.record?.reportPath) return;

  try {
    await ExecutionHistoryAPI.openFileWithDefaultApp(props.record.reportPath);
  } catch (error) {
    toast.error(`打开报告文件失败: ${error}`);
  }
};

const resolveDetailLogPath = (device: ExecutionHistoryDeviceRecord) => {
  return device.detailLogPath || device.logFilePath;
};

// 打开设备详细日志文件
const openDeviceDetailLog = async (device: ExecutionHistoryDeviceRecord) => {
  const detailLogPath = resolveDetailLogPath(device);
  if (!detailLogPath) return;

  try {
    await ExecutionHistoryAPI.openFileWithDefaultApp(detailLogPath);
  } catch (error) {
    toast.error(`打开详细日志失败: ${error}`);
  }
};

// 打开设备原始日志文件
const openRawLog = async (device: ExecutionHistoryDeviceRecord) => {
  if (!device.rawLogPath) return;

  try {
    await ExecutionHistoryAPI.openFileWithDefaultApp(device.rawLogPath);
  } catch (error) {
    toast.error(`打开原始日志失败: ${error}`);
  }
};

// 格式化时间
const formatTime = (timeStr: string) => {
  if (!timeStr) return "-";
  const date = new Date(timeStr);
  return date.toLocaleString("zh-CN");
};

// 格式化时长
const formatDuration = (ms: number) => {
  if (!ms || ms < 0) return "-";
  const seconds = Math.floor(ms / 1000);
  const minutes = Math.floor(seconds / 60);
  const hours = Math.floor(minutes / 60);

  if (hours > 0) {
    return `${hours}小时${minutes % 60}分${seconds % 60}秒`;
  } else if (minutes > 0) {
    return `${minutes}分${seconds % 60}秒`;
  } else {
    return `${seconds}秒`;
  }
};

// 获取状态文本
const getStatusText = (status: string) => {
  const statusMap: Record<string, string> = {
    completed: "成功",
    partial: "部分成功",
    failed: "失败",
    cancelled: "已取消",
  };
  return statusMap[status] || status;
};

// 获取模式文本
const getModeText = (mode: string) => {
  const modeMap: Record<string, string> = {
    group: "任务组模式A",
    binding: "任务组模式B",
    manual: "普通执行",
    backup: "备份执行",
  };
  return modeMap[mode] || mode;
};

// 获取设备状态样式类
const getDeviceStatusClass = (status: string) => {
  const statusMap: Record<string, string> = {
    Success: "success",
    Error: "error",
    Aborted: "error",
    Warning: "warning",
    Running: "running",
    Init: "waiting",
  };
  return statusMap[status] || "waiting";
};
</script>

<style scoped>
.record-detail-modal {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  z-index: 1100;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 20px;
  background: rgba(0, 0, 0, 0.7);
}

.modal-content {
  position: relative;
  width: 100%;
  max-width: 800px;
  max-height: 90vh;
  background: var(--card-bg, #1a1d23);
  border: 1px solid var(--border-color, #2d333b);
  border-radius: 12px;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 20px;
  border-bottom: 1px solid var(--border-color, #2d333b);
}

.modal-title {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: var(--text-primary, #e6edf3);
}

.btn-close {
  background: none;
  border: none;
  color: var(--text-secondary, #8b949e);
  font-size: 24px;
  cursor: pointer;
  padding: 0;
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 6px;
}

.btn-close:hover {
  background: var(--bg-tertiary, #21262d);
  color: var(--text-primary, #e6edf3);
}

.modal-body {
  flex: 1;
  overflow-y: auto;
  padding: 20px;
  scrollbar-width: thin;
  scrollbar-color: rgba(255, 255, 255, 0.2) transparent;
}

.modal-body::-webkit-scrollbar {
  width: 6px;
  height: 6px;
}

.modal-body::-webkit-scrollbar-track {
  background: transparent;
}

.modal-body::-webkit-scrollbar-thumb {
  background: rgba(255, 255, 255, 0.2);
  border-radius: 3px;
}

.modal-body::-webkit-scrollbar-thumb:hover {
  background: rgba(255, 255, 255, 0.35);
}

.info-section {
  background: var(--bg-secondary, #161b22);
  border: 1px solid var(--border-color, #2d333b);
  border-radius: 8px;
  padding: 16px;
  margin-bottom: 20px;
}

.info-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 0;
  border-bottom: 1px solid var(--border-color, #2d333b);
}

.info-row:last-child {
  border-bottom: none;
}

.info-label {
  font-size: 13px;
  color: var(--text-secondary, #8b949e);
}

.info-value {
  font-size: 13px;
  color: var(--text-primary, #e6edf3);
  font-weight: 500;
}

.info-value.status {
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 12px;
}

.info-value.status-completed {
  background: rgba(35, 134, 54, 0.2);
  color: #3fb950;
}

.info-value.status-partial {
  background: rgba(158, 106, 3, 0.2);
  color: #d29922;
}

.info-value.status-failed {
  background: rgba(218, 54, 51, 0.2);
  color: #f85149;
}

.info-value.status-cancelled {
  background: rgba(139, 148, 158, 0.2);
  color: #8b949e;
}

.info-value.report-path {
  font-family: monospace;
  font-size: 11px;
  max-width: 200px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.info-value-with-action {
  display: flex;
  align-items: center;
  gap: 8px;
}

.btn-open {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  padding: 0;
  background: transparent;
  border: 1px solid var(--border-color, #2d333b);
  border-radius: 4px;
  color: var(--text-secondary, #8b949e);
  cursor: pointer;
  transition: all 0.2s ease;
}

.btn-open:hover {
  background: var(--bg-tertiary, #21262d);
  border-color: var(--color-accent-primary, #58a6ff);
  color: var(--color-accent-primary, #58a6ff);
}

.btn-open:active {
  transform: scale(0.95);
}

.stats-section {
  margin-bottom: 20px;
}

.stats-section h4,
.devices-section h4 {
  margin: 0 0 12px 0;
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary, #e6edf3);
}

.stats-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(100px, 1fr));
  gap: 12px;
}

.stat-item {
  background: var(--bg-secondary, #161b22);
  border: 1px solid var(--border-color, #2d333b);
  border-radius: 8px;
  padding: 16px;
  text-align: center;
}

.stat-value {
  display: block;
  font-size: 24px;
  font-weight: 700;
  color: var(--text-primary, #e6edf3);
  margin-bottom: 4px;
}

.stat-item.success .stat-value {
  color: #3fb950;
}

.stat-item.error .stat-value {
  color: #f85149;
}

.stat-item.warning .stat-value {
  color: #d29922;
}

.stat-label {
  font-size: 12px;
  color: var(--text-secondary, #8b949e);
}

.device-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.device-item {
  background: var(--bg-secondary, #161b22);
  border: 1px solid var(--border-color, #2d333b);
  border-radius: 8px;
  padding: 12px 16px;
}

.device-item.status-success {
  border-left: 3px solid #238636;
}

.device-item.status-error {
  border-left: 3px solid #da3633;
}

.device-item.status-warning {
  border-left: 3px solid #9e6a03;
}

.device-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.device-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.btn-open-log-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  padding: 0;
  background: transparent;
  border: 1px solid var(--border-color, #2d333b);
  border-radius: 4px;
  color: var(--text-secondary, #8b949e);
  cursor: pointer;
  transition: all 0.2s ease;
}

.btn-open-log-icon:hover {
  background: var(--bg-tertiary, #21262d);
  border-color: var(--color-accent-primary, #58a6ff);
  color: var(--color-accent-primary, #58a6ff);
}

.btn-open-log-icon:active {
  transform: scale(0.95);
}

.device-ip {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary, #e6edf3);
  font-family: monospace;
}

.device-status {
  font-size: 11px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 4px;
  text-transform: uppercase;
}

.device-status.status-success {
  background: rgba(35, 134, 54, 0.2);
  color: #3fb950;
}

.device-status.status-error {
  background: rgba(218, 54, 51, 0.2);
  color: #f85149;
}

.device-status.status-warning {
  background: rgba(158, 106, 3, 0.2);
  color: #d29922;
}

.device-status.status-running {
  background: rgba(88, 166, 255, 0.2);
  color: #58a6ff;
}

.device-status.status-waiting {
  background: rgba(139, 148, 158, 0.2);
  color: #8b949e;
}

.device-info {
  display: flex;
  gap: 16px;
  font-size: 12px;
  color: var(--text-secondary, #8b949e);
  margin-bottom: 8px;
}

.device-error {
  font-size: 12px;
  color: #f85149;
  padding: 8px;
  background: rgba(218, 54, 51, 0.1);
  border-radius: 4px;
  margin-bottom: 8px;
}

.device-logs-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 6px 8px;
  background: var(--bg-tertiary, #21262d);
  border-radius: 4px 4px 0 0;
  border-bottom: 1px solid var(--border-color, #2d333b);
}

.logs-label {
  font-size: 11px;
  color: var(--text-secondary, #8b949e);
  font-weight: 500;
}

.btn-open-log {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 2px 8px;
  background: transparent;
  border: 1px solid var(--border-color, #2d333b);
  border-radius: 4px;
  color: var(--text-secondary, #8b949e);
  font-size: 11px;
  cursor: pointer;
  transition: all 0.2s ease;
}

.btn-open-log:hover {
  background: var(--bg-secondary, #161b22);
  border-color: var(--color-accent-primary, #58a6ff);
  color: var(--color-accent-primary, #58a6ff);
}

.btn-open-log:active {
  transform: scale(0.95);
}

.device-logs {
  background: var(--bg-tertiary, #21262d);
  border-radius: 4px;
  padding: 8px;
  font-family: monospace;
  font-size: 11px;
  max-height: 120px;
  overflow-y: auto;
  scrollbar-width: thin;
  scrollbar-color: rgba(255, 255, 255, 0.2) transparent;
}

.device-logs::-webkit-scrollbar {
  width: 4px;
  height: 4px;
}

.device-logs::-webkit-scrollbar-track {
  background: transparent;
}

.device-logs::-webkit-scrollbar-thumb {
  background: rgba(255, 255, 255, 0.2);
  border-radius: 2px;
}

.device-logs::-webkit-scrollbar-thumb:hover {
  background: rgba(255, 255, 255, 0.35);
}

.log-line {
  color: var(--text-secondary, #8b949e);
  padding: 2px 0;
  border-bottom: 1px solid var(--border-color, #2d333b);
  white-space: pre-wrap;
  word-break: break-all;
}

.log-line:last-child {
  border-bottom: none;
}

/* 动画 */
.modal-enter-active,
.modal-leave-active {
  transition: all 0.2s ease;
}

.modal-enter-from,
.modal-leave-to {
  opacity: 0;
}

.modal-enter-from .modal-content,
.modal-leave-to .modal-content {
  transform: scale(0.95);
}
</style>
