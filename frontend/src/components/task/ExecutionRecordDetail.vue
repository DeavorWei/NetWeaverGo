<template>
  <el-dialog
    v-model="dialogVisible"
    title="执行详情"
    width="800px"
    @closed="closeModal"
    destroy-on-close
  >
    <div v-if="record" class="flex flex-col gap-4 -mt-4">
      <!-- 基本信息 -->
      <div class="grid grid-cols-2 gap-4 p-4 bg-bg-panel rounded-lg">
        <div class="text-sm">
          <span class="text-text-muted mr-2 inline-block w-16">任务名称</span>
          <span class="text-text-primary font-medium">{{ record.taskName }}</span>
        </div>
        <div class="text-sm">
          <span class="text-text-muted mr-2 inline-block w-16">执行模式</span>
          <span class="text-text-primary">{{ getModeText(record.mode) }}</span>
        </div>
        <div class="text-sm flex items-center">
          <span class="text-text-muted mr-2 inline-block w-16">执行状态</span>
          <el-tag size="small" :type="getStatusType(record.status)">
            {{ getStatusText(record.status) }}
          </el-tag>
        </div>
        <div class="text-sm">
          <span class="text-text-muted mr-2 inline-block w-16">执行时长</span>
          <span class="text-text-primary">{{ formatDuration(record.durationMs) }}</span>
        </div>
        <div class="text-sm">
          <span class="text-text-muted mr-2 inline-block w-16">开始时间</span>
          <span class="text-text-primary">{{ formatTime(record.startedAt) }}</span>
        </div>
        <div class="text-sm">
          <span class="text-text-muted mr-2 inline-block w-16">结束时间</span>
          <span class="text-text-primary">{{ formatTime(record.finishedAt) }}</span>
        </div>
        <div class="text-sm col-span-2 flex items-center gap-2" v-if="record.reportPath">
          <span class="text-text-muted mr-2 inline-block w-16 flex-shrink-0">报告文件</span>
          <span class="font-mono text-[11px] text-text-primary truncate" :title="record.reportPath">
            {{ record.reportPath }}
          </span>
          <el-button size="small" type="primary" plain @click="openReportFile" class="ml-2">
            打开报告
          </el-button>
        </div>
      </div>

      <!-- 统计信息 -->
      <div class="flex items-center gap-6 bg-bg-panel rounded-lg p-3">
        <h4 class="text-sm font-semibold text-text-primary m-0 border-r border-border pr-6">执行统计</h4>
        <div class="flex items-center gap-6">
          <div class="flex items-baseline gap-2">
            <span class="text-xs text-text-muted">设备总数</span>
            <span class="text-base font-semibold text-text-primary">{{ record.totalDevices }}</span>
          </div>
          <div class="flex items-baseline gap-2">
            <span class="text-xs text-text-muted">成功</span>
            <span class="text-base font-semibold text-success">{{ record.successCount }}</span>
          </div>
          <div class="flex items-baseline gap-2" v-if="record.errorCount > 0">
            <span class="text-xs text-text-muted">失败</span>
            <span class="text-base font-semibold text-error">{{ record.errorCount }}</span>
          </div>
          <div class="flex items-baseline gap-2" v-if="(record.abortedCount || 0) > 0">
            <span class="text-xs text-text-muted">中止</span>
            <span class="text-base font-semibold text-warning">{{ record.abortedCount || 0 }}</span>
          </div>
          <div class="flex items-baseline gap-2" v-if="(record.warningCount || 0) > 0">
            <span class="text-xs text-text-muted">告警</span>
            <span class="text-base font-semibold text-info">{{ record.warningCount || 0 }}</span>
          </div>
        </div>
      </div>

      <!-- 设备执行进度列表 -->
      <div class="min-h-0 flex flex-col h-[400px]">
        <div class="flex justify-between items-center mb-3 flex-shrink-0">
          <h4 class="text-sm font-semibold text-text-primary">设备执行进度</h4>
          <span class="text-xs text-text-secondary">{{ deviceDetails.length }} 台设备</span>
        </div>
        <div class="flex-1 overflow-auto border border-border rounded-lg bg-bg-panel">
          <DeviceExecutionProgressList
            :run-id="record.id"
            :devices="deviceDetails"
            :loading="loadingDevices"
            @select="handleDeviceSelect"
          />
        </div>
      </div>

      <!-- 选中设备的文件操作 -->
      <div v-if="selectedDevice" class="bg-bg-panel border border-border rounded-lg p-4">
        <h4 class="text-sm font-semibold text-text-primary mb-3">{{ selectedDevice.deviceIp }} - 文件操作</h4>
        <div class="flex flex-wrap gap-4">
          <!-- 详细日志 -->
          <div class="flex items-center gap-2">
            <span class="text-xs text-text-muted">详细日志</span>
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
          <div v-if="selectedDevice.rawLogPath" class="flex items-center gap-2">
            <span class="text-xs text-text-muted">原始日志</span>
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
          <div v-if="selectedDevice.summaryLogPath" class="flex items-center gap-2">
            <span class="text-xs text-text-muted">摘要日志</span>
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
          <div v-if="selectedDevice.journalLogPath" class="flex items-center gap-2">
            <span class="text-xs text-text-muted">流水日志</span>
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
      <div v-if="reportPath" class="bg-bg-panel border border-border rounded-lg p-4 flex justify-between items-center">
        <h4 class="text-sm font-semibold text-text-primary m-0">执行报告</h4>
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
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, watch, computed } from "vue";
import { ExecutionHistoryAPI } from "../../services/api";
import type {
  ExecutionHistoryRecord,
  DeviceExecutionView,
} from "../../types/executionHistory";
import { ElMessage } from "element-plus";
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

const dialogVisible = computed({
  get: () => props.modelValue,
  set: (val) => emit("update:modelValue", val)
});

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
      ElMessage.error(result.message || "打开报告文件失败");
    }
  } catch (error) {
    ElMessage.error(`打开报告文件失败: ${error}`);
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

const getStatusType = (status: string) => {
  const typeMap: Record<string, "success" | "warning" | "danger" | "info"> = {
    completed: "success",
    partial: "warning",
    failed: "danger",
    cancelled: "info",
  };
  return typeMap[status] || "info";
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

<style scoped>
</style>
