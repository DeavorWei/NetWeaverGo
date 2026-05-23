<template>
  <div class="mib-node-detail">
    <div v-if="loading" class="detail-loading">
      <el-icon class="is-loading"><Loading /></el-icon>
      <span>加载中...</span>
    </div>

    <div v-else-if="!node" class="detail-empty">
      <span>请选择一个节点查看详情</span>
    </div>

    <div v-else class="detail-content">
      <!-- 节点名称和 OID -->
      <div class="detail-header">
        <h3 class="node-name">{{ node.name }}</h3>
        <span class="node-oid">{{ node.oid }}</span>
      </div>

      <!-- 基本信息 -->
      <div class="detail-section">
        <h4 class="section-title">基本信息</h4>
        <div class="info-grid">
          <div class="info-item">
            <span class="info-label">类型</span>
            <el-tag :type="getTypeTagType(node.type)" size="small" effect="plain">
              {{ node.type || '-' }}
            </el-tag>
          </div>
          <div class="info-item">
            <span class="info-label">语法</span>
            <span class="info-value">{{ node.description || '-' }}</span>
          </div>
          <div class="info-item">
            <span class="info-label">访问权限</span>
            <el-tag :type="getAccessTagType(node.access)" size="small" effect="plain">
              {{ node.access || '-' }}
            </el-tag>
          </div>
          <div class="info-item">
            <span class="info-label">状态</span>
            <el-tag :type="getStatusTagType(node.status)" size="small" effect="plain">
              {{ node.status || '-' }}
            </el-tag>
          </div>
        </div>
      </div>

      <!-- 描述 -->
      <div class="detail-section" v-if="node.description">
        <h4 class="section-title">描述</h4>
        <div class="description-content">
          {{ node.description }}
        </div>
      </div>

      <!-- 所属模块 -->
      <div class="detail-section" v-if="resolvedOID">
        <h4 class="section-title">所属模块</h4>
        <div class="info-grid">
          <div class="info-item full-width">
            <span class="info-label">模块名称</span>
            <span class="info-value">{{ resolvedOID.moduleName || '-' }}</span>
          </div>
        </div>
      </div>

      <!-- OID 解析信息 -->
      <div class="detail-section" v-if="resolvedOID">
        <h4 class="section-title">OID 解析</h4>
        <div class="info-grid">
          <div class="info-item full-width">
            <span class="info-label">完整 OID</span>
            <span class="info-value oid-value">{{ resolvedOID.oid }}</span>
          </div>
          <div class="info-item full-width">
            <span class="info-label">解析名称</span>
            <span class="info-value">{{ resolvedOID.name }}</span>
          </div>
          <div class="info-item">
            <span class="info-label">类型</span>
            <span class="info-value">{{ resolvedOID.type || '-' }}</span>
          </div>
          <div class="info-item">
            <span class="info-label">访问权限</span>
            <span class="info-value">{{ resolvedOID.access || '-' }}</span>
          </div>
        </div>
      </div>

      <!-- 操作按钮 -->
      <div class="detail-actions">
        <el-button type="primary" size="small" @click="handleEdit">
          编辑
        </el-button>
        <el-button type="danger" size="small" @click="handleDelete">
          删除
        </el-button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { Loading } from '@element-plus/icons-vue'
import { SNMPMIBAPI } from '@/services/snmpApi'
import type { MIBNode, ResolvedOID } from '@/types/snmp'

// Props
interface Props {
  node: MIBNode | null
}

const props = defineProps<Props>()

// Emits
const emit = defineEmits<{
  (e: 'edit', node: MIBNode): void
  (e: 'delete', node: MIBNode): void
}>()

// 状态
const loading = ref(false)
const resolvedOID = ref<ResolvedOID | null>(null)

// 监听节点变化，加载详情
watch(
  () => props.node,
  async (node) => {
    if (node) {
      loading.value = true
      resolvedOID.value = null
      try {
        resolvedOID.value = await SNMPMIBAPI.resolveOID(node.oid)
      } catch {
        resolvedOID.value = null
      } finally {
        loading.value = false
      }
    } else {
      resolvedOID.value = null
    }
  },
  { immediate: true }
)

// 编辑
function handleEdit() {
  if (props.node) {
    emit('edit', props.node)
  }
}

// 删除
function handleDelete() {
  if (props.node) {
    emit('delete', props.node)
  }
}

// 获取类型标签样式
function getTypeTagType(type: string): 'success' | 'warning' | 'info' | 'danger' | '' {
  switch (type?.toLowerCase()) {
    case 'scalar':
      return 'info'
    case 'table':
      return 'success'
    case 'row':
      return 'warning'
    case 'column':
      return ''
    case 'notification':
      return 'danger'
    default:
      return 'info'
  }
}

// 获取访问权限标签样式
function getAccessTagType(access: string): 'success' | 'warning' | 'info' | 'danger' | '' {
  switch (access?.toLowerCase()) {
    case 'read-only':
    case 'readonly':
      return 'info'
    case 'read-write':
    case 'readwrite':
      return 'success'
    case 'write-only':
    case 'writeonly':
      return 'warning'
    case 'not-accessible':
    case 'notaccessible':
      return 'danger'
    default:
      return ''
  }
}

// 获取状态标签样式
function getStatusTagType(status: string): 'success' | 'warning' | 'info' | '' {
  switch (status?.toLowerCase()) {
    case 'current':
    case 'active':
      return 'success'
    case 'deprecated':
      return 'warning'
    case 'obsolete':
      return 'info'
    default:
      return ''
  }
}
</script>

<style scoped>
.mib-node-detail {
  display: flex;
  flex-direction: column;
  height: 100%;
  gap: 16px;
}

.detail-loading,
.detail-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 24px;
  color: var(--text-muted);
  font-size: 13px;
}

.detail-content {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.detail-header {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding-bottom: 12px;
  border-bottom: 1px solid var(--border);
}

.node-name {
  font-size: 16px;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0;
}

.node-oid {
  font-size: 12px;
  font-family: 'Consolas', monospace;
  color: var(--text-muted);
}

.detail-section {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.section-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-secondary);
  margin: 0;
}

.info-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 8px;
}

.info-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.info-item.full-width {
  grid-column: span 2;
}

.info-label {
  font-size: 11px;
  color: var(--text-muted);
}

.info-value {
  font-size: 12px;
  color: var(--text-primary);
  word-break: break-all;
}

.info-value.oid-value {
  font-family: 'Consolas', monospace;
}

.description-content {
  font-size: 12px;
  color: var(--text-secondary);
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-word;
  background-color: var(--bg-tertiary);
  padding: 12px;
  border-radius: 6px;
  border: 1px solid var(--border);
}

.detail-actions {
  display: flex;
  gap: 8px;
  padding-top: 12px;
  border-top: 1px solid var(--border);
}
</style>