<template>
  <Teleport to="body">
    <Transition name="drawer">
      <div v-if="modelValue" class="execution-history-drawer">
        <!-- 遮罩 -->
        <div class="drawer-overlay" @click="handleOverlayClick" />

        <!-- 抽屉内容 -->
        <div class="drawer-content">
          <!-- 头部 -->
          <div class="drawer-header">
            <h3 class="drawer-title">
              <i class="icon-history"></i>
              执行历史记录
              <span v-if="taskGroupName" class="task-name">{{
                taskGroupName
              }}</span>
            </h3>
            <button class="btn-close" @click="closeDrawer">
              <span>×</span>
            </button>
          </div>

          <!-- 筛选栏 -->
          <div class="drawer-filter">
            <select
              v-model="filterStatus"
              class="filter-select"
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
              class="btn-delete-all"
              @click="deleteAllRecords"
              title="删除全部记录"
            >
              <i class="icon-trash"></i>
              清空全部
            </button>
          </div>

          <!-- 列表内容 -->
          <div class="drawer-body">
            <div v-if="loading" class="loading-state">
              <div class="spinner"></div>
              <span>加载中...</span>
            </div>

            <div v-else-if="records.length === 0" class="empty-state">
              <i class="icon-empty"></i>
              <p>暂无历史执行记录</p>
            </div>

            <div v-else class="record-list">
              <div
                v-for="record in records"
                :key="record.id"
                class="record-item"
                :class="`status-${record.status}`"
                @click="showDetail(record)"
              >
                <div class="record-header">
                  <span
                    class="record-status"
                    :class="`status-${record.status}`"
                  >
                    {{ getStatusText(record.status) }}
                  </span>
                  <span class="record-time">{{
                    formatTime(record.startedAt)
                  }}</span>
                </div>

                <div class="record-info">
                  <div class="record-task">{{ record.taskName }}</div>
                  <div class="record-stats">
                    <span>设备: {{ record.totalDevices }}</span>
                    <span class="success">成功: {{ record.successCount }}</span>
                    <span class="error" v-if="record.errorCount > 0"
                      >失败: {{ record.errorCount }}</span
                    >
                    <span class="duration">{{
                      formatDuration(record.durationMs)
                    }}</span>
                  </div>
                </div>

                <!-- 删除按钮 -->
                <button
                  class="btn-delete"
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
                :disabled="currentPage <= 1"
                @click="changePage(currentPage - 1)"
              >
                上一页
              </button>
              <span>{{ currentPage }} / {{ totalPages }}</span>
              <button
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
    console.error("加载历史记录失败:", error);
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
      // 删除全部
      const result = await ExecutionHistoryAPI.deleteAllRunRecords();
      if (result?.success) {
        toast.success("删除成功");
        records.value = [];
        total.value = 0;
        taskexecStore.clearAllHistory();
        deleteConfirmModal.value.show = false;
      } else {
        toast.error(result?.message || "删除失败");
      }
    } else {
      // 删除单条
      const record = deleteConfirmModal.value.targetRecord;
      if (record) {
        const result = await ExecutionHistoryAPI.deleteRunRecord(record.id);
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
    console.error("删除记录失败:", error);
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
.execution-history-drawer {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  z-index: 1000;
  display: flex;
  justify-content: flex-end;
}

.drawer-overlay {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background-color: var(--overlay-bg);
  backdrop-filter: blur(4px);
}

.drawer-content {
  position: relative;
  width: 480px;
  max-width: 90vw;
  height: 100%;
  background-color: var(--color-bg-secondary);
  border-left: 1px solid var(--color-border-default);
  display: flex;
  flex-direction: column;
}

.drawer-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 20px;
  border-bottom: 1px solid var(--color-border-default);
}

.drawer-title {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: var(--color-text-primary);
}

.task-name {
  font-size: 13px;
  font-weight: normal;
  color: var(--color-text-secondary);
  padding: 2px 8px;
  background-color: var(--color-bg-tertiary);
  border-radius: 4px;
  margin-left: 8px;
}

.btn-close {
  background: none;
  border: none;
  color: var(--color-text-secondary);
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
  background-color: var(--color-bg-tertiary);
  color: var(--color-text-primary);
}

.drawer-filter {
  padding: 12px 20px;
  border-bottom: 1px solid var(--color-border-default);
  display: flex;
  gap: 12px;
  align-items: center;
}

.filter-select {
  flex: 1;
  padding: 8px 12px;
  background-color: var(--color-bg-secondary);
  border: 1px solid var(--color-border-default);
  border-radius: 6px;
  color: var(--color-text-primary);
  font-size: 13px;
}

.btn-delete-all {
  padding: 8px 12px;
  background-color: var(--color-error);
  border: none;
  border-radius: 6px;
  color: white;
  font-size: 13px;
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 6px;
  transition: all 0.2s ease;
  white-space: nowrap;
}

.btn-delete-all:hover {
  background-color: var(--color-error);
  opacity: 0.9;
}

.drawer-body {
  flex: 1;
  overflow-y: auto;
  padding: 16px 20px;
}

.loading-state,
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 60px 20px;
  color: var(--color-text-secondary);
  gap: 12px;
}

.spinner {
  width: 32px;
  height: 32px;
  border: 2px solid var(--color-border-default);
  border-top-color: var(--color-accent-primary);
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

.record-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.record-item {
  padding: 16px;
  padding-right: 70px; /* 为删除按钮留出空间 */
  background-color: var(--color-bg-tertiary);
  border: 1px solid var(--color-border-default);
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s ease;
  position: relative;
}

.record-item:hover {
  border-color: var(--color-accent-primary);
  transform: translateX(4px);
}

.btn-delete {
  position: absolute;
  top: 50%;
  right: 12px;
  transform: translateY(-50%);
  background-color: var(--color-bg-tertiary);
  border: 1px solid var(--color-border-default);
  color: var(--color-text-muted);
  cursor: pointer;
  padding: 6px 10px;
  border-radius: 6px;
  opacity: 1;
  transition: all 0.2s ease;
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
}

.btn-delete:hover {
  background-color: var(--color-error-bg);
  border-color: var(--color-error);
  color: var(--color-error);
}

.record-item.status-completed {
  border-left: 3px solid var(--color-success);
}

.record-item.status-partial {
  border-left: 3px solid var(--color-warning);
}

.record-item.status-failed {
  border-left: 3px solid var(--color-error);
}

.record-item.status-cancelled {
  border-left: 3px solid var(--color-text-muted);
}

.record-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}

.record-status {
  font-size: 12px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 4px;
}

.record-status.status-completed {
  background-color: var(--color-success-bg);
  color: var(--color-success);
}

.record-status.status-partial {
  background-color: var(--color-warning-bg);
  color: var(--color-warning);
}

.record-status.status-failed {
  background-color: var(--color-error-bg);
  color: var(--color-error);
}

.record-status.status-cancelled {
  background-color: var(--color-bg-hover);
  color: var(--color-text-muted);
}

.record-time {
  font-size: 12px;
  color: var(--color-text-secondary);
}

.record-task {
  font-size: 14px;
  font-weight: 500;
  color: var(--color-text-primary);
  margin-bottom: 8px;
}

.record-stats {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  font-size: 12px;
  color: var(--color-text-secondary);
}

.record-stats .success {
  color: var(--color-success);
}

.record-stats .error {
  color: var(--color-error);
}

.record-stats .duration {
  margin-left: auto;
  font-family: monospace;
}

.pagination {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 16px;
  margin-top: 20px;
  padding-top: 16px;
  border-top: 1px solid var(--color-border-default);
}

.pagination button {
  padding: 6px 12px;
  background-color: var(--color-bg-secondary);
  border: 1px solid var(--color-border-default);
  border-radius: 6px;
  color: var(--color-text-primary);
  font-size: 13px;
  cursor: pointer;
}

.pagination button:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.pagination button:hover:not(:disabled) {
  background-color: var(--color-bg-tertiary);
}

.pagination span {
  font-size: 13px;
  color: var(--color-text-secondary);
}

/* 动画 */
.drawer-enter-active,
.drawer-leave-active {
  transition: all 0.3s ease;
}

.drawer-enter-from,
.drawer-leave-to {
  opacity: 0;
}

.drawer-enter-from .drawer-content,
.drawer-leave-to .drawer-content {
  transform: translateX(100%);
}
</style>
