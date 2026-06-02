<template>
  <el-drawer
    v-model="drawerVisible"
    size="500px"
    @closed="closeDrawer"
  >
    <template #header>
      <div class="flex items-center gap-2">
        <span class="text-base font-semibold text-text-primary">执行历史记录</span>
        <el-tag v-if="taskGroupName" size="small" type="info">{{ taskGroupName }}</el-tag>
      </div>
    </template>

    <div class="flex flex-col h-full -mt-4">
      <!-- 筛选栏 -->
      <div class="flex gap-3 items-center mb-4">
        <el-select v-model="filterStatus" class="flex-1" @change="loadRecords" placeholder="全部状态">
          <el-option label="全部状态" value="" />
          <el-option label="成功" value="completed" />
          <el-option label="部分成功" value="partial" />
          <el-option label="失败" value="failed" />
          <el-option label="已取消" value="cancelled" />
        </el-select>
        <el-button
          v-if="records.length > 0"
          type="danger"
          plain
          @click="deleteAllRecords"
        >
          清空全部
        </el-button>
      </div>

      <!-- 列表内容 -->
      <div class="flex-1 min-h-0 overflow-auto scrollbar-custom" v-loading="loading">
        <el-empty v-if="records.length === 0 && !loading" description="暂无历史执行记录" />
        <div v-else class="flex flex-col gap-3 pr-2">
          <div
            v-for="record in records"
            :key="record.id"
            class="border border-border rounded-lg p-3 cursor-pointer hover:border-accent transition-all relative group bg-bg-panel"
            @click="showDetail(record)"
          >
            <div class="flex justify-between items-center mb-2">
              <el-tag size="small" :type="getStatusType(record.status)">
                {{ getStatusText(record.status) }}
              </el-tag>
              <span class="text-xs text-text-muted">{{ formatTime(record.startedAt) }}</span>
            </div>
            <div class="text-sm font-medium text-text-primary mb-2">{{ record.taskName }}</div>
            <div class="flex items-center gap-3 text-xs text-text-secondary">
              <span>设备: {{ record.totalDevices }}</span>
              <span class="text-success">成功: {{ record.successCount }}</span>
              <span v-if="record.errorCount > 0" class="text-error">失败: {{ record.errorCount }}</span>
              <span class="ml-auto">{{ formatDuration(record.durationMs) }}</span>
            </div>
            <el-button
              type="danger"
              circle
              size="small"
              class="absolute top-1/2 right-3 -translate-y-1/2 opacity-0 group-hover:opacity-100 transition-opacity shadow-sm"
              @click.stop="deleteRecord(record)"
              :icon="Delete"
              title="删除此记录"
            />
          </div>
        </div>
      </div>

      <!-- 分页 -->
      <div class="mt-4 flex justify-end flex-shrink-0" v-if="total > 0">
        <el-pagination
          v-model:current-page="currentPage"
          :page-size="pageSize"
          :total="total"
          layout="prev, pager, next"
          size="small"
          @current-change="changePage"
        />
      </div>
    </div>
  </el-drawer>

  <!-- 详情弹窗 -->
  <ExecutionRecordDetail
    v-model="showDetailModal"
    :record="selectedRecord"
    @close="selectedRecord = null"
  />
</template>

<script setup lang="ts">
import { ref, watch, computed } from "vue";
import { ExecutionHistoryAPI } from "../../services/api";
import { useTaskexecStore } from "../../stores/taskexecStore";
import { ElMessage, ElMessageBox } from "element-plus";
import { Delete } from "@element-plus/icons-vue";
import type { ExecutionHistoryRecord } from "../../types/executionHistory";
import ExecutionRecordDetail from "./ExecutionRecordDetail.vue";
import { getLogger } from '@/utils/logger'

const logger = getLogger()

const props = defineProps<{
  modelValue: boolean;
  taskGroupId?: string;
  taskGroupName?: string;
  runKind?: string;
}>();

const emit = defineEmits<{
  (e: "update:modelValue", value: boolean): void;
}>();

const drawerVisible = computed({
  get: () => props.modelValue,
  set: (val) => emit("update:modelValue", val)
});

const taskexecStore = useTaskexecStore();

// 状态
const loading = ref(false);
const records = ref<ExecutionHistoryRecord[]>([]);
const filterStatus = ref("");
const currentPage = ref(1);
const pageSize = 10;
const total = ref(0);
const totalPages = ref(0);

// 详情弹窗
const showDetailModal = ref(false);
const selectedRecord = ref<ExecutionHistoryRecord | null>(null);

const showDetail = (record: ExecutionHistoryRecord) => {
  selectedRecord.value = record;
  showDetailModal.value = true;
};

// 计算总页数
const calculateTotalPages = () => {
  totalPages.value = Math.ceil(total.value / pageSize);
};

// 加载记录
const loadRecords = async () => {
  if (!props.modelValue) return;

  loading.value = true;
  try {
    const result = await ExecutionHistoryAPI.listTaskRunRecords({
      runKind: props.runKind || "",
      status: filterStatus.value,
      limit: pageSize * currentPage.value,
      taskGroupId: props.taskGroupId || "",
    });
    records.value = (result?.data || []).map(
      (item: any): ExecutionHistoryRecord => ({
        id: item.id,
        runnerSource: item.runnerSource,
        runnerId: item.id,
        taskGroupId: item.taskGroupId,
        taskGroupName: item.taskGroupName,
        taskName: item.taskName,
        mode: item.mode,
        status: item.status,
        totalDevices: item.totalDevices,
        finishedCount: item.finishedCount,
        successCount: item.successCount,
        errorCount: item.errorCount,
        startedAt: item.startedAt,
        finishedAt: item.finishedAt,
        durationMs: item.durationMs,
        runKind: item.runKind,
        devices: [],
        reportPath: "",
        abortedCount: 0,
        warningCount: 0,
        createdAt: item.startedAt,
      }),
    );
    total.value = result?.total || 0;
    calculateTotalPages();
  } catch (error) {
    logger.error("加载历史记录失败", 'ExecutionHistoryDrawer', error);
  } finally {
    loading.value = false;
  }
};

// 切换页码
const changePage = (page: number) => {
  currentPage.value = page;
  loadRecords();
};

// 关闭抽屉
const closeDrawer = () => {
  emit("update:modelValue", false);
};

// 删除单条记录
const deleteRecord = (record: ExecutionHistoryRecord) => {
  ElMessageBox.confirm(
    `确定要删除记录「${record.taskName}」吗？此操作不可恢复。`,
    '确认删除',
    {
      confirmButtonText: '确认',
      cancelButtonText: '取消',
      type: 'warning',
    }
  ).then(async () => {
    try {
      const result = await ExecutionHistoryAPI.deleteRunRecord({
        runId: record.id,
        taskGroupId: props.taskGroupId || "",
      });
      if (result?.success) {
        ElMessage.success("删除成功");
        const index = records.value.findIndex((r) => r.id === record.id);
        if (index !== -1) {
          records.value.splice(index, 1);
          total.value--;
        }
        taskexecStore.removeRunFromHistory(record.id);
      } else {
        ElMessage.error(result?.message || "删除失败");
      }
    } catch (error) {
      logger.error("删除记录失败", 'ExecutionHistoryDrawer', error);
      ElMessage.error("删除失败");
    }
  }).catch(() => {});
};

// 删除全部记录
const deleteAllRecords = () => {
  if (records.value.length === 0) {
    ElMessage.warning("暂无记录可删除");
    return;
  }
  ElMessageBox.confirm(
    `确定要删除全部 ${total.value} 条记录吗？此操作不可恢复。`,
    '清空全部记录',
    {
      confirmButtonText: '确认删除',
      cancelButtonText: '取消',
      type: 'warning',
    }
  ).then(async () => {
    try {
      const result = await ExecutionHistoryAPI.deleteAllRunRecords({
        taskGroupId: props.taskGroupId || "",
        runKind: props.runKind || "",
      });
      if (result?.success) {
        ElMessage.success(result.message || "删除成功");
        records.value = [];
        total.value = 0;
        taskexecStore.clearAllHistory();
      } else {
        ElMessage.error(result?.message || "删除失败");
      }
    } catch (error) {
      logger.error("清空记录失败", 'ExecutionHistoryDrawer', error);
      ElMessage.error("删除失败");
    }
  }).catch(() => {});
};

// 格式化时间
const formatTime = (timeStr: string) => {
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
const formatDuration = (ms: number) => {
  if (!ms || ms < 0) return "-";
  const seconds = Math.floor(ms / 1000);
  const minutes = Math.floor(seconds / 60);
  const hours = Math.floor(minutes / 60);

  if (hours > 0) {
    return `${hours}小时${minutes % 60}分`;
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

// 监听显示状态
watch(
  () => props.modelValue,
  (visible) => {
    if (visible) {
      currentPage.value = 1;
      filterStatus.value = "";
      loadRecords();
    }
  },
);
</script>

<style scoped>
</style>
