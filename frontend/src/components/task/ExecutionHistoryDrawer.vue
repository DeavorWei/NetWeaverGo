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
              <span v-if="taskGroupName" class="task-name">{{ taskGroupName }}</span>
            </h3>
            <button class="btn-close" @click="closeDrawer">
              <span>×</span>
            </button>
          </div>

          <!-- 筛选栏 -->
          <div class="drawer-filter">
            <select v-model="filterStatus" class="filter-select" @change="loadRecords">
              <option value="">全部状态</option>
              <option value="completed">成功</option>
              <option value="partial">部分成功</option>
              <option value="failed">失败</option>
              <option value="cancelled">已取消</option>
            </select>
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
                  <span class="record-status" :class="`status-${record.status}`">
                    {{ getStatusText(record.status) }}
                  </span>
                  <span class="record-time">{{ formatTime(record.startedAt) }}</span>
                </div>

                <div class="record-info">
                  <div class="record-task">{{ record.taskName }}</div>
                  <div class="record-stats">
                    <span>设备: {{ record.totalDevices }}</span>
                    <span class="success">成功: {{ record.successCount }}</span>
                    <span class="error" v-if="record.errorCount > 0">失败: {{ record.errorCount }}</span>
                    <span class="duration">{{ formatDuration(record.durationMs) }}</span>
                  </div>
                </div>
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
  </Teleport>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { ExecutionHistoryAPI } from '../../services/api'
import type { ExecutionRecord, ListExecutionRecordsRequest } from '../../services/api'
import ExecutionRecordDetail from './ExecutionRecordDetail.vue'

const props = defineProps<{
  modelValue: boolean
  taskGroupId?: string
  taskGroupName?: string
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void
}>()

// 状态
const loading = ref(false)
const records = ref<ExecutionRecord[]>([])
const filterStatus = ref('')
const currentPage = ref(1)
const pageSize = 10
const total = ref(0)
const totalPages = ref(0)

// 详情弹窗
const showDetailModal = ref(false)
const selectedRecord = ref<ExecutionRecord | null>(null)

// 计算总页数
const calculateTotalPages = () => {
  totalPages.value = Math.ceil(total.value / pageSize)
}

// 加载记录
const loadRecords = async () => {
  if (!props.modelValue) return

  loading.value = true
  try {
    const req: ListExecutionRecordsRequest = {
      taskGroupId: props.taskGroupId || '',
      runnerSource: '',
      status: filterStatus.value,
      page: currentPage.value,
      pageSize: pageSize,
      sortBy: 'started_at',
      sortOrder: 'desc',
    }

    const result = await ExecutionHistoryAPI.listExecutionRecords(req)
    records.value = result?.data || []
    total.value = result?.total || 0
    calculateTotalPages()
  } catch (error) {
    console.error('加载历史记录失败:', error)
  } finally {
    loading.value = false
  }
}

// 切换页码
const changePage = (page: number) => {
  currentPage.value = page
  loadRecords()
}

// 关闭抽屉
const closeDrawer = () => {
  emit('update:modelValue', false)
}

// 显示详情
const showDetail = (record: ExecutionRecord) => {
  selectedRecord.value = record
  showDetailModal.value = true
}

// 处理遮罩层点击
const handleOverlayClick = () => {
  // 当详情弹窗打开时，不关闭抽屉
  if (!showDetailModal.value) {
    closeDrawer()
  }
}

// 格式化时间
const formatTime = (timeStr: string) => {
  if (!timeStr) return '-'
  const date = new Date(timeStr)
  return date.toLocaleString('zh-CN', {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}

// 格式化时长
const formatDuration = (ms: number) => {
  if (!ms || ms < 0) return '-'
  const seconds = Math.floor(ms / 1000)
  const minutes = Math.floor(seconds / 60)
  const hours = Math.floor(minutes / 60)

  if (hours > 0) {
    return `${hours}小时${minutes % 60}分`
  } else if (minutes > 0) {
    return `${minutes}分${seconds % 60}秒`
  } else {
    return `${seconds}秒`
  }
}

// 获取状态文本
const getStatusText = (status: string) => {
  const statusMap: Record<string, string> = {
    completed: '成功',
    partial: '部分成功',
    failed: '失败',
    cancelled: '已取消',
  }
  return statusMap[status] || status
}

// 监听显示状态
watch(() => props.modelValue, (visible) => {
  if (visible) {
    currentPage.value = 1
    filterStatus.value = ''
    loadRecords()
  }
})
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
  background: rgba(0, 0, 0, 0.5);
}

.drawer-content {
  position: relative;
  width: 480px;
  max-width: 90vw;
  height: 100%;
  background: var(--card-bg, #1a1d23);
  border-left: 1px solid var(--border-color, #2d333b);
  display: flex;
  flex-direction: column;
}

.drawer-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 20px;
  border-bottom: 1px solid var(--border-color, #2d333b);
}

.drawer-title {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: var(--text-primary, #e6edf3);
}

.task-name {
  font-size: 13px;
  font-weight: normal;
  color: var(--text-secondary, #8b949e);
  padding: 2px 8px;
  background: var(--bg-tertiary, #21262d);
  border-radius: 4px;
  margin-left: 8px;
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

.drawer-filter {
  padding: 12px 20px;
  border-bottom: 1px solid var(--border-color, #2d333b);
}

.filter-select {
  width: 100%;
  padding: 8px 12px;
  background: var(--bg-secondary, #161b22);
  border: 1px solid var(--border-color, #2d333b);
  border-radius: 6px;
  color: var(--text-primary, #e6edf3);
  font-size: 13px;
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
  color: var(--text-secondary, #8b949e);
  gap: 12px;
}

.spinner {
  width: 32px;
  height: 32px;
  border: 2px solid var(--border-color, #2d333b);
  border-top-color: var(--primary-color, #58a6ff);
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
  background: var(--bg-secondary, #161b22);
  border: 1px solid var(--border-color, #2d333b);
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s ease;
}

.record-item:hover {
  border-color: var(--primary-color, #58a6ff);
  transform: translateX(4px);
}

.record-item.status-completed {
  border-left: 3px solid #238636;
}

.record-item.status-partial {
  border-left: 3px solid #9e6a03;
}

.record-item.status-failed {
  border-left: 3px solid #da3633;
}

.record-item.status-cancelled {
  border-left: 3px solid #8b949e;
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
  background: rgba(35, 134, 54, 0.2);
  color: #3fb950;
}

.record-status.status-partial {
  background: rgba(158, 106, 3, 0.2);
  color: #d29922;
}

.record-status.status-failed {
  background: rgba(218, 54, 51, 0.2);
  color: #f85149;
}

.record-status.status-cancelled {
  background: rgba(139, 148, 158, 0.2);
  color: #8b949e;
}

.record-time {
  font-size: 12px;
  color: var(--text-secondary, #8b949e);
}

.record-task {
  font-size: 14px;
  font-weight: 500;
  color: var(--text-primary, #e6edf3);
  margin-bottom: 8px;
}

.record-stats {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  font-size: 12px;
  color: var(--text-secondary, #8b949e);
}

.record-stats .success {
  color: #3fb950;
}

.record-stats .error {
  color: #f85149;
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
  border-top: 1px solid var(--border-color, #2d333b);
}

.pagination button {
  padding: 6px 12px;
  background: var(--bg-secondary, #161b22);
  border: 1px solid var(--border-color, #2d333b);
  border-radius: 6px;
  color: var(--text-primary, #e6edf3);
  font-size: 13px;
  cursor: pointer;
}

.pagination button:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.pagination button:hover:not(:disabled) {
  background: var(--bg-tertiary, #21262d);
}

.pagination span {
  font-size: 13px;
  color: var(--text-secondary, #8b949e);
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
