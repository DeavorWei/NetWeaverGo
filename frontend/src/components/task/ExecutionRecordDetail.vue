<template>
  <Teleport to="body">
    <Transition name="modal">
      <div
        v-if="modelValue && record"
        class="modal-detail"
        @click.self="closeModal"
      >
        <div class="modal-detail-content" @click.stop>
          <!-- 头部 -->
          <div class="modal-header">
            <h3 class="modal-header-title">执行详情</h3>
            <button class="btn-icon-close" @click="closeModal">
              <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2">
                <line x1="18" y1="6" x2="6" y2="18"></line>
                <line x1="6" y1="6" x2="18" y2="18"></line>
              </svg>
            </button>
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
                  class="status-badge"
                  :class="`status-badge-${getStatusClass(record.status)}`"
                >
                  {{ getStatusText(record.status) }}
                </span>
              </div>
              <div class="info-row">
                <span class="info-label">开始时间</span>
                <span class="info-value">{{ formatTime(record.startedAt) }}</span>
              </div>
              <div class="info-row">
                <span class="info-label">结束时间</span>
                <span class="info-value">{{ formatTime(record.finishedAt) }}</span>
              </div>
              <div class="info-row">
                <span class="info-label">执行时长</span>
                <span class="info-value">{{ formatDuration(record.durationMs) }}</span>
              </div>
              <div class="info-row" v-if="record.reportPath">
                <span class="info-label">报告文件</span>
                <div class="info-value-with-action">
                  <span class="info-value font-mono text-[11px] max-w-[200px] truncate">
                    {{ record.reportPath }}
                  </span>
                  <button
                    class="btn-file btn-file-sm"
                    @click="openReportFile"
                    title="打开报告"
                  >
                    <svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2">
                      <path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"></path>
                      <polyline points="15 3 21 3 21 9"></polyline>
                      <line x1="10" y1="14" x2="21" y2="3"></line>
                    </svg>
                  </button>
                </div>
              </div>
            </div>

            <!-- 统计信息 -->
            <div class="mb-5">
              <h4 class="text-sm font-semibold text-text-primary mb-3">执行统计</h4>
              <div class="grid gap-3 grid-cols-[repeat(auto-fit,minmax(100px,1fr))]">
                <div class="stat-card">
                  <span class="stat-value">{{ record.totalDevices }}</span>
                  <span class="stat-label">设备总数</span>
                </div>
                <div class="stat-card stat-card-success">
                  <span class="stat-value">{{ record.successCount }}</span>
                  <span class="stat-label">成功</span>
                </div>
                <div class="stat-card stat-card-error" v-if="record.errorCount > 0">
                  <span class="stat-value">{{ record.errorCount }}</span>
                  <span class="stat-label">失败</span>
                </div>
                <div class="stat-card stat-card-warning" v-if="(record.abortedCount || 0) > 0">
                  <span class="stat-value">{{ record.abortedCount || 0 }}</span>
                  <span class="stat-label">中止</span>
                </div>
                <div class="stat-card" v-if="(record.warningCount || 0) > 0">
                  <span class="stat-value">{{ record.warningCount || 0 }}</span>
                  <span class="stat-label">告警</span>
                </div>
              </div>
            </div>

            <!-- 设备执行进度列表 -->
            <div class="section">
              <div class="section-header">
                <h4 class="text-sm font-semibold text-text-primary">设备执行进度</h4>
                <span class="text-xs text-text-secondary">{{ deviceDetails.length }} 台设备</span>
              </div>
              <DeviceExecutionProgressList
                :run-id="record.id"
                :devices="deviceDetails"
                :loading="loadingDevices"
                @select="handleDeviceSelect"
              />
            </div>

            <!-- 选中设备的文件操作 -->
            <div v-if="selectedDevice" class="section device-files-section">
              <div class="section-header">
                <h4 class="text-sm font-semibold text-text-primary">{{ selectedDevice.deviceIp }} - 文件操作</h4>
              </div>
              <div class="device-files-grid">
                <!-- 详细日志 -->
                <div class="file-item">
                  <span class="file-label">详细日志</span>
                  <FileOperationButtons
                    :run-id="record.id"
                    :unit-id="selectedDevice.unitId"
                    file-type="detail"
                    :has-file="!!selectedDevice.detailLogPath"
                    :exists="selectedDevice.detailLogExists"
                    size="small"
                  />
                </div>

                <!-- 原始日志 -->
                <div v-if="selectedDevice.rawLogPath" class="file-item">
                  <span class="file-label">原始日志</span>
                  <FileOperationButtons
                    :run-id="record.id"
                    :unit-id="selectedDevice.unitId"
                    file-type="raw"
                    :has-file="true"
                    :exists="selectedDevice.rawLogExists"
                    size="small"
                  />
                </div>

                <!-- 摘要日志 -->
                <div v-if="selectedDevice.summaryLogPath" class="file-item">
                  <span class="file-label">摘要日志</span>
                  <FileOperationButtons
                    :run-id="record.id"
                    :unit-id="selectedDevice.unitId"
                    file-type="summary"
                    :has-file="true"
                    :exists="selectedDevice.summaryLogExists"
                    size="small"
                  />
                </div>

                <!-- 流水日志 -->
                <div v-if="selectedDevice.journalLogPath" class="file-item">
                  <span class="file-label">流水日志</span>
                  <FileOperationButtons
                    :run-id="record.id"
                    :unit-id="selectedDevice.unitId"
                    file-type="journal"
                    :has-file="true"
                    :exists="selectedDevice.journalLogExists"
                    size="small"
                  />
                </div>
              </div>
            </div>

            <!-- 报告文件操作 -->
            <div v-if="reportPath" class="section">
              <div class="section-header">
                <h4 class="text-sm font-semibold text-text-primary">执行报告</h4>
                <FileOperationButtons
                  :run-id="record.id"
                  file-type="report"
                  :has-file="true"
                  :exists="reportExists"
                  size="medium"
                  show-text
                />
              </div>
            </div>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, watch } from "vue";
import { ExecutionHistoryAPI } from "../../services/api";
import type {
  ExecutionHistoryRecord,
  DeviceExecutionView,
} from "../../types/executionHistory";
import { useToast } from "../../utils/useToast";
import DeviceExecutionProgressList from "./DeviceExecutionProgressList.vue";
import FileOperationButtons from "./FileOperationButtons.vue";
import { getLogger } from '@/utils/logger'

const logger = getLogger()

const props = defineProps<{
  modelValue: boolean;
  record: ExecutionHistoryRecord | null;
}>();

const emit = defineEmits<{
  (e: "update:modelValue", value: boolean): void;
  (e: "close"): void;
}>();

const toast = useToast();

// 设备详情相关
const deviceDetails = ref<DeviceExecutionView[]>([]);
const loadingDevices = ref(false);
const selectedDevice = ref<DeviceExecutionView | null>(null);
const reportPath = ref("");
const reportExists = ref(false);

const closeModal = () => {
  emit("update:modelValue", false);
  emit("close");
};

// 加载设备详情
const loadDeviceDetails = async () => {
  if (!props.record?.id) return;

  loadingDevices.value = true;
  try {
    const response = await ExecutionHistoryAPI.getDeviceDetails({
      runId: props.record.id,
    });
    deviceDetails.value = response?.devices || [];
  } catch (error) {
    logger.error("加载设备详情失败", 'ExecutionRecordDetail', error);
    deviceDetails.value = [];
  } finally {
    loadingDevices.value = false;
  }
};

// 加载报告路径
const loadReportPath = async () => {
  if (!props.record?.id) return;

  try {
    const response = await ExecutionHistoryAPI.getReportPath({
      runId: props.record.id,
    });
    reportPath.value = response?.reportPath || "";
    reportExists.value = response?.exists || false;
  } catch (error) {
    logger.error("加载报告路径失败", 'ExecutionRecordDetail', error);
    reportPath.value = "";
    reportExists.value = false;
  }
};

// 处理设备选择
const handleDeviceSelect = (device: DeviceExecutionView) => {
  selectedDevice.value = device;
};

// 监听记录变化，重新加载数据
watch(
  () => props.record?.id,
  () => {
    if (props.modelValue && props.record) {
      loadDeviceDetails();
      loadReportPath();
      selectedDevice.value = null;
    }
  },
  { immediate: true },
);

// 打开报告文件
const openReportFile = async () => {
  if (!props.record?.id) return;

  try {
    const result = await ExecutionHistoryAPI.openFileWithDefaultApp({
      runId: props.record.id,
      unitId: "",
      fileType: "report",
    });
    if (result && !result.success) {
      toast.error(result.message || "打开报告文件失败");
    }
  } catch (error) {
    toast.error(`打开报告文件失败: ${error}`);
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

// 获取状态样式类名
const getStatusClass = (status: string) => {
  const classMap: Record<string, string> = {
    completed: "success",
    partial: "warning",
    failed: "error",
    cancelled: "muted",
  };
  return classMap[status] || "muted";
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
</script>

<style scoped lang="postcss">
@reference "../../styles/index.css";

/* 组件特有样式 */
.section {
  @apply mb-5;
}

.section-header {
  @apply flex justify-between items-center mb-3;
}

/* 动画 */
.modal-enter-active,
.modal-leave-active {
  @apply transition-all duration-200;
}

.modal-enter-from,
.modal-leave-to {
  @apply opacity-0;
}

.modal-enter-from .modal-detail-content,
.modal-leave-to .modal-detail-content {
  @apply scale-95;
}
</style>
