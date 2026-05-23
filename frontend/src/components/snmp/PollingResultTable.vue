<template>
  <div class="polling-result-table">
    <!-- 筛选栏 -->
    <div class="filter-bar">
      <el-select
        v-model="targetFilter"
        placeholder="按目标筛选"
        clearable
        size="small"
        class="filter-select"
        @change="handleFilterChange"
      >
        <el-option
          v-for="target in targetOptions"
          :key="target.id"
          :label="target.displayName || target.targetIP"
          :value="target.id"
        />
      </el-select>
    </div>

    <!-- 表格主体 -->
    <el-table
      :data="data"
      :loading="loading"
      stripe
      highlight-current-row
      class="result-data-table"
      @row-click="handleRowClick"
    >
      <!-- OID -->
      <el-table-column prop="oid" label="OID" min-width="180">
        <template #default="{ row }">
          <span class="oid-value">{{ row.oid }}</span>
        </template>
      </el-table-column>

      <!-- OID 名称 -->
      <el-table-column prop="oidName" label="OID 名称" min-width="140">
        <template #default="{ row }">
          <span class="oid-name">{{ row.oidName || '-' }}</span>
        </template>
      </el-table-column>

      <!-- 值 -->
      <el-table-column prop="value" label="值" min-width="160">
        <template #default="{ row }">
          <span class="result-value" :title="row.value">{{ row.value }}</span>
        </template>
      </el-table-column>

      <!-- 值类型 -->
      <el-table-column prop="valueType" label="值类型" width="120">
        <template #default="{ row }">
          <el-tag
            :type="getValueTypeTagType(row.valueType)"
            size="small"
            effect="plain"
          >
            {{ row.valueType || 'unknown' }}
          </el-tag>
        </template>
      </el-table-column>

      <!-- 采集时间 -->
      <el-table-column prop="pollTime" label="采集时间" width="160">
        <template #default="{ row }">
          <span class="poll-time">{{ formatTime(row.pollTime) }}</span>
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
import { ref, watch } from 'vue'
import type { PollingResult, PollingTarget } from '@/types/snmp'

// Props
interface Props {
  data: PollingResult[]
  targets?: PollingTarget[]
  loading?: boolean
  total?: number
  page?: number
  pageSize?: number
}

const props = withDefaults(defineProps<Props>(), {
  targets: () => [],
  loading: false,
  total: 0,
  page: 1,
  pageSize: 20,
})

// Emits
const emit = defineEmits<{
  (e: 'filter-change', targetId: number | null): void
  (e: 'page-change', page: number): void
  (e: 'size-change', size: number): void
  (e: 'row-click', result: PollingResult): void
}>()

// 分页状态
const currentPage = ref(props.page)
const pageSize = ref(props.pageSize)

// 目标筛选
const targetFilter = ref<number | null>(null)

// 目标选项
const targetOptions = ref(props.targets)

// 监听 props 变化
watch(() => props.page, (val) => {
  currentPage.value = val
})

watch(() => props.pageSize, (val) => {
  pageSize.value = val
})

watch(() => props.targets, (val) => {
  targetOptions.value = val
})

// 处理筛选变化
function handleFilterChange(targetId: number | null) {
  emit('filter-change', targetId)
}

// 处理行点击
function handleRowClick(row: PollingResult) {
  emit('row-click', row)
}

// 处理分页变化
function handleCurrentChange(val: number) {
  emit('page-change', val)
}

// 处理每页条数变化
function handleSizeChange(val: number) {
  emit('size-change', val)
}

// 获取值类型标签样式
function getValueTypeTagType(valueType: string): 'success' | 'warning' | 'info' | 'danger' | '' {
  switch (valueType?.toLowerCase()) {
    case 'string':
      return 'success'
    case 'integer':
      return 'warning'
    case 'gauge':
      return 'info'
    case 'counter':
      return ''
    case 'timeticks':
      return 'danger'
    default:
      return 'info'
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
</script>

<style scoped>
.polling-result-table {
  display: flex;
  flex-direction: column;
  height: 100%;
  gap: 12px;
}

.filter-bar {
  display: flex;
  gap: 8px;
  align-items: center;
}

.filter-select {
  width: 200px;
}

.result-data-table {
  flex: 1;
  overflow: auto;
}

.oid-value {
  font-size: 12px;
  font-family: 'Consolas', monospace;
  color: var(--text-secondary);
  word-break: break-all;
}

.oid-name {
  font-size: 12px;
  color: var(--text-primary);
}

.result-value {
  font-size: 12px;
  font-family: 'Consolas', monospace;
  color: var(--text-primary);
  max-width: 300px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  display: inline-block;
}

.poll-time {
  font-size: 12px;
  color: var(--text-secondary);
}

.pagination-wrapper {
  display: flex;
  justify-content: flex-end;
  padding: 8px 0;
}
</style>