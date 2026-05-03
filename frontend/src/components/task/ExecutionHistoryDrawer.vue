<template>
  <Teleport to="body">
    <Transition name="drawer">
      <div v-if="modelValue" class="fixed inset-0 z-[1000] flex justify-end">
        <!-- 遮罩 -->
        <div class="drawer-overlay" @click="handleOverlayClick" />

        <!-- 抽屉内容 -->
        <div class="drawer-container">
          <!-- 头部 -->
          <div class="drawer-header">
            <h3 class="drawer-title">
              <i class="icon-history"></i>
              执行历史记录
              <span v-if="taskGroupName" class="text-xs font-normal text-text-secondary px-2 py-0.5 bg-bg-tertiary rounded ml-2">
                {{ taskGroupName }}
              </span>
            </h3>
            <button class="btn-icon-close" @click="closeDrawer">
              <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2">
                <line x1="18" y1="6" x2="6" y2="18"></line>
                <line x1="6" y1="6" x2="18" y2="18"></line>
              </svg>
            </button>
          </div>

          <!-- 筛选栏 -->
          <div class="px-5 py-3 border-b border-border flex gap-3 items-center">
            <select
              v-model="filterStatus"
              class="input flex-1"
              @change="loadRecords"
            >
              <option value="">全部状态</option>
              <option value="completed">成功</option>
              <option value="partial">部分成功</option>
              <option value="failed">失败</option>
              <option value="cancelled">已取消</option>
            </select>
            <button
              v-if="records.length > 0"
              class="btn-danger-solid"
              @click="deleteAllRecords"
              title="删除全部记录"
            >
              <i class="icon-trash"></i>
              清空全部
            </button>
          </div>

          <!-- 列表内容 -->
          <div class="drawer-body">
            <!-- 加载状态 -->
            <div v-if="loading" class="flex flex-col items-center justify-center py-16 gap-3">
              <div class="w-8 h-8 border-2 border-border border-t-accent rounded-full animate-spin"></div>
              <span class="text-text-secondary">加载中...</span>
            </div>

            <!-- 空状态 -->
            <div v-else-if="records.length === 0" class="flex flex-col items-center justify-center py-16 gap-3">
              <i class="icon-empty text-text-muted"></i>
              <p class="text-text-secondary">暂无历史执行记录</p>
            </div>

            <!-- 记录列表 -->
            <div v-else class="flex flex-col gap-3">
              <div
                v-for="record in records"
                :key="record.id"
                class="record-item"
                :class="`record-item-${record.status}`"
                @click="showDetail(record)"
              >
                <div class="record-header">
                  <span
                    class="status-badge"
                    :class="`status-badge-${getStatusClass(record.status)}`"
                  >
                    {{ getStatusText(record.status) }}
                  </span>
                  <span class="record-time">{{ formatTime(record.startedAt) }}</span>
                </div>

                <div class="record-info">
                  <div class="record-task">{{ record.taskName }}</div>
                  <div class="record-stats">
                    <span>设备: {{ record.totalDevices }}</span>
                    <span class="text-success">成功: {{ record.successCount }}</span>
                    <span v-if="record.errorCount > 0" class="text-error">失败: {{ record.errorCount }}</span>
                    <span class="duration">{{ formatDuration(record.durationMs) }}</span>
                  </div>
                </div>

                <!-- 删除按钮 -->
                <button
                  class="btn-delete absolute top-1/2 right-3 -translate-y-1/2"
                  @click="deleteRecord(record, $event)"
                  title="删除此记录"
                >
                  <i class="icon-trash"></i>
                  <span>删除</span>
                </button>
              </div>
            </div>

            <!-- 分页 -->
            <div v-if="totalPages > 1" class="pagination">
              <button
                class="btn btn-sm"
                :disabled="currentPage <= 1"
                @click="changePage(currentPage - 1)"
              >
                上一页
              </button>
              <span>{{ currentPage }} / {{ totalPages }}</span>
              <button
                class="btn btn-sm"
                :disabled="currentPage >= totalPages"
                @click="changePage(currentPage + 1)"
              >
                下一页
              </button>
            </div>
          </div>
        </div>
      </div>
    </Transition>

    <!-- 详情弹窗 -->
    <ExecutionRecordDetail
      v-model="showDetailModal"
      :record="selectedRecord"
      @close="selectedRecord = null"
      @click.stop
    />

    <!-- 删除确认弹窗 -->
    <ConfirmModal
      v-model:show="deleteConfirmModal.show"
      :type="deleteConfirmModal.isBatch ? 'danger' : 'warning'"
      :title="deleteConfirmModal.isBatch ? '清空全部记录' : '确认删除'"
      subtitle="此操作不可恢复"
      :loading="deleteConfirmModal.deleting"
      confirm-text="确认删除"
      @confirm="executeDelete"
    >
      <template v-if="deleteConfirmModal.isBatch">
        确定要删除全部
        <span class="font-mono text-accent font-bold">{{ total }}</span>
        条记录吗？
      </template>
      <template v-else>
        确定要删除记录「<span class="font-mono text-accent">{{
          deleteConfirmModal.targetRecord?.taskName
        }}</span
        >」吗？
      </template>
    </ConfirmModal>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, watch } from "vue";
import { ExecutionHistoryAPI } from "../../services/api";
import { useTaskexecStore } from "../../stores/taskexecStore";
import { useToast } from "../../utils/useToast";
import type { ExecutionHistoryRecord } from "../../types/executionHistory";
import ExecutionRecordDetail from "./ExecutionRecordDetail.vue";
import ConfirmModal from "../common/ConfirmModal.vue";
import { getLogger } from '@/utils/logger'

const logger = getLogger()

const props = defineProps<{
  modelValue: boolean;
  taskGroupId?: string;
  taskGroupName?: string;
}>();

const emit = defineEmits<{
  (e: "update:modelValue", value: boolean): void;
}>();

const taskexecStore = useTaskexecStore();
const toast = useToast();

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

// 删除确认弹窗状态
const deleteConfirmModal = ref({
  show: false,
  isBatch: false,
  targetRecord: null as ExecutionHistoryRecord | null,
  deleting: false,
});

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
      runKind: "",
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

// 显示详情
const showDetail = (record: ExecutionHistoryRecord) => {
  selectedRecord.value = record;
  showDetailModal.value = true;
};

// 处理遮罩层点击
const handleOverlayClick = () => {
  // 当详情弹窗打开时，不关闭抽屉
  if (!showDetailModal.value) {
    closeDrawer();
  }
};

// 删除单条记录 - 打开确认弹窗
const deleteRecord = (record: ExecutionHistoryRecord, event: Event) => {
  event.stopPropagation(); // 阻止触发 showDetail
  deleteConfirmModal.value = {
    show: true,
    isBatch: false,
    targetRecord: record,
    deleting: false,
  };
};

// 删除全部记录 - 打开确认弹窗
const deleteAllRecords = () => {
  if (records.value.length === 0) {
    toast.warning("暂无记录可删除");
    return;
  }
  deleteConfirmModal.value = {
    show: true,
    isBatch: true,
    targetRecord: null,
    deleting: false,
  };
};

// 执行删除确认
const executeDelete = async () => {
  deleteConfirmModal.value.deleting = true;

  try {
    if (deleteConfirmModal.value.isBatch) {
      // 删除全部 - 传递 taskGroupId 进行筛选
      const result = await ExecutionHistoryAPI.deleteAllRunRecords({
        taskGroupId: props.taskGroupId || "",
      });
      if (result?.success) {
        toast.success(result.message || "删除成功");
        records.value = [];
        total.value = 0;
        taskexecStore.clearAllHistory();
        deleteConfirmModal.value.show = false;
      } else {
        toast.error(result?.message || "删除失败");
      }
    } else {
      // 删除单条 - 传递 taskGroupId 进行权限验证
      const record = deleteConfirmModal.value.targetRecord;
      if (record) {
        const result = await ExecutionHistoryAPI.deleteRunRecord({
          runId: record.id,
          taskGroupId: props.taskGroupId || "",
        });
        if (result?.success) {
          toast.success("删除成功");
          const index = records.value.findIndex((r) => r.id === record.id);
          if (index !== -1) {
            records.value.splice(index, 1);
            total.value--;
          }
          taskexecStore.removeRunFromHistory(record.id);
          deleteConfirmModal.value.show = false;
        } else {
          toast.error(result?.message || "删除失败");
        }
      }
    }
  } catch (error) {
    logger.error("删除记录失败", 'ExecutionHistoryDrawer', error);
    toast.error("删除失败");
  } finally {
    deleteConfirmModal.value.deleting = false;
  }
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

<style scoped lang="postcss">
@reference "../../styles/index.css";

/* 组件特有样式 */
.drawer-title {
  @apply flex items-center gap-2 m-0 text-base font-semibold text-text-primary;
}

/* 动画 */
.drawer-enter-active,
.drawer-leave-active {
  @apply transition-all duration-300;
}

.drawer-enter-from,
.drawer-leave-to {
  @apply opacity-0;
}

.drawer-enter-from .drawer-container,
.drawer-leave-to .drawer-container {
  transform: translateX(100%);
}
</style>
