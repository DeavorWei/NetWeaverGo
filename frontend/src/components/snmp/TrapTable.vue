<template>
  <div class="trap-table">
    <!-- 表格工具栏 -->
    <div class="table-toolbar">
      <div class="toolbar-left">
        <el-checkbox
          v-model="isAllSelected"
          :indeterminate="isIndeterminate"
          @change="handleSelectAllChange"
        >
          全选
        </el-checkbox>
        <span class="selected-count" v-if="selectedIds.size > 0">
          已选择 {{ selectedIds.size }} 条
        </span>
      </div>
      <div class="toolbar-right">
        <el-button
          v-if="selectedIds.size > 0"
          type="primary"
          size="small"
          :loading="batchAcknowledging"
          @click="handleBatchAcknowledge"
        >
          批量确认
        </el-button>
        <el-button
          v-if="selectedIds.size > 0"
          type="danger"
          size="small"
          :loading="batchDeleting"
          @click="handleBatchDelete"
        >
          批量删除
        </el-button>
      </div>
    </div>

    <!-- 表格主体 -->
    <el-table
      ref="tableRef"
      :data="data"
      :loading="loading"
      :row-class-name="getRowClassName"
      @selection-change="handleSelectionChange"
      @row-click="handleRowClick"
      highlight-current-row
      stripe
      class="trap-data-table"
    >
      <!-- 选择列 -->
      <el-table-column type="selection" width="50" />

      <!-- 来源IP -->
      <el-table-column prop="sourceIP" label="来源 IP" min-width="140">
        <template #default="{ row }">
          <span class="source-ip">{{ row.sourceIP }}:{{ row.sourcePort }}</span>
        </template>
      </el-table-column>

      <!-- Trap OID -->
      <el-table-column prop="trapOID" label="Trap OID" min-width="180">
        <template #default="{ row }">
          <div class="trap-oid-cell">
            <span class="trap-oid">{{ row.trapOID }}</span>
            <span class="trap-name text-text-muted text-xs">{{ row.trapName }}</span>
          </div>
        </template>
      </el-table-column>

      <!-- 严重级别 -->
      <el-table-column prop="severity" label="严重级别" width="100">
        <template #default="{ row }">
          <el-tag :type="getSeverityTagType(row.severity)" size="small" effect="plain">
            {{ getSeverityText(row.severity) }}
          </el-tag>
        </template>
      </el-table-column>

      <!-- 接收时间 -->
      <el-table-column prop="receivedAt" label="接收时间" width="160">
        <template #default="{ row }">
          <span class="receive-time">{{ formatTime(row.receivedAt) }}</span>
        </template>
      </el-table-column>

      <!-- 确认状态 -->
      <el-table-column prop="acknowledged" label="确认状态" width="100">
        <template #default="{ row }">
          <el-tag
            :type="row.acknowledged ? 'success' : 'warning'"
            size="small"
            effect="plain"
          >
            {{ row.acknowledged ? '已确认' : '未确认' }}
          </el-tag>
        </template>
      </el-table-column>

      <!-- 操作列 -->
      <el-table-column label="操作" width="120" fixed="right">
        <template #default="{ row }">
          <div class="action-buttons">
            <el-button
              v-if="!row.acknowledged"
              type="success"
              size="small"
              link
              @click.stop="handleAcknowledge(row)"
              :loading="row._acknowledging"
            >
              确认
            </el-button>
            <el-button
              type="primary"
              size="small"
              link
              @click.stop="handleViewDetail(row)"
            >
              详情
            </el-button>
            <el-button
              type="danger"
              size="small"
              link
              @click.stop="handleDelete(row)"
              :loading="row._deleting"
            >
              删除
            </el-button>
          </div>
        </template>
      </el-table-column>
    </el-table>

    <!-- 分页 -->
    <div class="pagination-wrapper">
      <el-pagination
        v-model:current-page="currentPage"
        v-model:page-size="pageSize"
        :page-sizes="[10, 20, 50, 100]"
        :total="total"
        :background="true"
        layout="total, sizes, prev, pager, next, jumper"
        @size-change="handleSizeChange"
        @current-change="handleCurrentChange"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import type { ElTable } from 'element-plus'
import type { TrapRecord } from '@/types/snmp'

// Props
interface Props {
  data: TrapRecord[]
  loading?: boolean
  total?: number
  page?: number
  pageSize?: number
  isNewTrap?: (id: number) => boolean
}

const props = withDefaults(defineProps<Props>(), {
  loading: false,
  total: 0,
  page: 1,
  pageSize: 20,
  isNewTrap: () => false,
})

// Emits
const emit = defineEmits<{
  (e: 'acknowledge', id: number): void
  (e: 'batch-acknowledge', ids: number[]): void
  (e: 'delete', id: number): void
  (e: 'batch-delete', ids: number[]): void
  (e: 'view-detail', trap: TrapRecord): void
  (e: 'page-change', page: number): void
  (e: 'size-change', size: number): void
  (e: 'selection-change', ids: number[]): void
}>()

// 表格引用
const tableRef = ref<InstanceType<typeof ElTable>>()

// 分页状态
const currentPage = ref(props.page)
const pageSize = ref(props.pageSize)

// 选择状态
const selectedIds = ref<Set<number>>(new Set())
const batchAcknowledging = ref(false)
const batchDeleting = ref(false)

// 全选状态
const isAllSelected = computed(() => {
  return props.data.length > 0 && props.data.every(row => selectedIds.value.has(row.id))
})

const isIndeterminate = computed(() => {
  const selectedCount = props.data.filter(row => selectedIds.value.has(row.id)).length
  return selectedCount > 0 && selectedCount < props.data.length
})

// 监听 props 变化
watch(() => props.page, (val) => {
  currentPage.value = val
})

watch(() => props.pageSize, (val) => {
  pageSize.value = val
})

// 处理选择变化
function handleSelectionChange(selection: TrapRecord[]) {
  selectedIds.value = new Set(selection.map(row => row.id))
  emit('selection-change', Array.from(selectedIds.value))
}

// 处理全选变化
function handleSelectAllChange(val: boolean) {
  if (val) {
    props.data.forEach(row => {
      tableRef.value?.toggleRowSelection(row, true)
    })
  } else {
    tableRef.value?.clearSelection()
  }
}

// 处理行点击
function handleRowClick(row: TrapRecord) {
  emit('view-detail', row)
}

// 处理确认
function handleAcknowledge(row: TrapRecord) {
  row._acknowledging = true
  emit('acknowledge', row.id)
  setTimeout(() => {
    row._acknowledging = false
  }, 500)
}

// 处理批量确认
async function handleBatchAcknowledge() {
  if (selectedIds.value.size === 0) return
  batchAcknowledging.value = true
  emit('batch-acknowledge', Array.from(selectedIds.value))
  setTimeout(() => {
    batchAcknowledging.value = false
    tableRef.value?.clearSelection()
  }, 500)
}

// 处理删除
function handleDelete(row: TrapRecord) {
  row._deleting = true
  emit('delete', row.id)
  setTimeout(() => {
    row._deleting = false
  }, 500)
}

// 处理批量删除
async function handleBatchDelete() {
  if (selectedIds.value.size === 0) return
  batchDeleting.value = true
  emit('batch-delete', Array.from(selectedIds.value))
  setTimeout(() => {
    batchDeleting.value = false
    tableRef.value?.clearSelection()
  }, 500)
}

// 处理查看详情
function handleViewDetail(row: TrapRecord) {
  emit('view-detail', row)
}

// 处理分页变化
function handleCurrentChange(val: number) {
  emit('page-change', val)
}

// 处理每页条数变化
function handleSizeChange(val: number) {
  emit('size-change', val)
}

// 获取严重级别标签类型
function getSeverityTagType(severity: string): 'danger' | 'warning' | 'info' | 'success' | '' {
  switch (severity.toLowerCase()) {
    case 'critical':
      return 'danger'
    case 'major':
      return 'warning'
    case 'minor':
      return 'info'
    case 'info':
      return 'success'
    default:
      return ''
  }
}

// 获取严重级别文本
function getSeverityText(severity: string): string {
  switch (severity.toLowerCase()) {
    case 'critical':
      return '严重'
    case 'major':
      return '重要'
    case 'minor':
      return '次要'
    case 'info':
      return '信息'
    default:
      return '未知'
  }
}

// 格式化时间
function formatTime(timeStr: string): string {
  if (!timeStr) return '-'
  try {
    const date = new Date(timeStr)
    return date.toLocaleString('zh-CN')
  } catch {
    return timeStr
  }
}

// 获取行样式类名
function getRowClassName({ row }: { row: TrapRecord }): string {
  if (props.isNewTrap(row.id)) {
    return 'new-trap-row'
  }
  return ''
}
</script>

<style scoped>
.trap-table {
  display: flex;
  flex-direction: column;
  height: 100%;
  gap: 12px;
}

.table-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 12px;
  background-color: var(--bg-secondary);
  border-radius: 6px;
}

.toolbar-left {
  display: flex;
  align-items: center;
  gap: 12px;
}

.selected-count {
  font-size: 12px;
  color: var(--text-muted);
}

.toolbar-right {
  display: flex;
  gap: 8px;
}

.trap-data-table {
  flex: 1;
  overflow: auto;
}

.trap-oid-cell {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.trap-oid {
  font-size: 12px;
  color: var(--text-primary);
  word-break: break-all;
}

.trap-name {
  font-size: 11px;
  color: var(--text-muted);
}

.source-ip {
  font-size: 12px;
  font-family: 'Consolas', monospace;
}

.receive-time {
  font-size: 12px;
  color: var(--text-secondary);
}

.action-buttons {
  display: flex;
  gap: 4px;
  justify-content: center;
}

.pagination-wrapper {
  display: flex;
  justify-content: flex-end;
  padding: 8px 0;
}

/* 新 Trap 高亮样式 */
.new-trap-row {
  background-color: rgba(var(--accent-rgb), 0.1) !important;
  animation: highlight-fade 3s ease-out forwards;
}

@keyframes highlight-fade {
  0% {
    background-color: rgba(var(--accent-rgb), 0.3);
  }
  100% {
    background-color: transparent;
  }
}
</style>