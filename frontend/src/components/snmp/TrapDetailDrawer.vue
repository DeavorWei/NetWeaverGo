<template>
  <el-drawer
    v-model="visible"
    title="Trap 详情"
    direction="rtl"
    :size="480"
    @close="handleClose"
  >
    <div v-if="trap" class="trap-detail-content">
      <!-- 基本信息 -->
      <div class="detail-section">
        <h4 class="section-title">基本信息</h4>
        <div class="detail-grid">
          <div class="detail-item">
            <span class="detail-label">来源 IP</span>
            <span class="detail-value">{{ trap.sourceIP }}:{{ trap.sourcePort }}</span>
          </div>
          <div class="detail-item">
            <span class="detail-label">SNMP 版本</span>
            <span class="detail-value">{{ trap.version }}</span>
          </div>
          <div class="detail-item">
            <span class="detail-label">Community</span>
            <span class="detail-value">{{ trap.community || '-' }}</span>
          </div>
          <div class="detail-item">
            <span class="detail-label">接收时间</span>
            <span class="detail-value">{{ formatTime(trap.receivedAt) }}</span>
          </div>
        </div>
      </div>

      <!-- Trap 信息 -->
      <div class="detail-section">
        <h4 class="section-title">Trap 信息</h4>
        <div class="detail-grid">
          <div class="detail-item full-width">
            <span class="detail-label">Trap OID</span>
            <span class="detail-value oid-value">{{ trap.trapOID }}</span>
          </div>
          <div class="detail-item full-width">
            <span class="detail-label">Trap 名称</span>
            <span class="detail-value">{{ trap.trapName || '-' }}</span>
          </div>
          <div class="detail-item">
            <span class="detail-label">企业 OID</span>
            <span class="detail-value">{{ trap.enterprise || '-' }}</span>
          </div>
          <div class="detail-item">
            <span class="detail-label">严重级别</span>
            <el-tag :type="getSeverityTagType(trap.severity)" size="small" effect="plain">
              {{ getSeverityText(trap.severity) }}
            </el-tag>
          </div>
          <div class="detail-item">
            <span class="detail-label">通用 Trap</span>
            <span class="detail-value">{{ trap.genericTrap }}</span>
          </div>
          <div class="detail-item">
            <span class="detail-label">特定 Trap</span>
            <span class="detail-value">{{ trap.specificTrap }}</span>
          </div>
        </div>
      </div>

      <!-- 确认状态 -->
      <div class="detail-section">
        <h4 class="section-title">确认状态</h4>
        <div class="detail-grid">
          <div class="detail-item">
            <span class="detail-label">确认状态</span>
            <el-tag :type="trap.acknowledged ? 'success' : 'warning'" size="small" effect="plain">
              {{ trap.acknowledged ? '已确认' : '未确认' }}
            </el-tag>
          </div>
          <div class="detail-item" v-if="trap.acknowledged && trap.acknowledgedAt">
            <span class="detail-label">确认时间</span>
            <span class="detail-value">{{ formatTime(trap.acknowledgedAt) }}</span>
          </div>
        </div>
      </div>

      <!-- 变量绑定 -->
      <div class="detail-section">
        <h4 class="section-title">变量绑定 (VarBinds)</h4>
        <div class="varbinds-container">
          <div
            v-for="(varbind, index) in parsedVarBinds"
            :key="index"
            class="varbind-item"
          >
            <div class="varbind-header">
              <span class="varbind-index">#{{ index + 1 }}</span>
              <span class="varbind-oid">{{ varbind.oid }}</span>
            </div>
            <div class="varbind-body">
              <div v-if="varbind.oidName" class="varbind-row">
                <span class="varbind-label">名称:</span>
                <span class="varbind-value">{{ varbind.oidName }}</span>
              </div>
              <div v-if="varbind.type" class="varbind-row">
                <span class="varbind-label">类型:</span>
                <span class="varbind-value">{{ varbind.type }}</span>
              </div>
              <div class="varbind-row">
                <span class="varbind-label">值:</span>
                <span class="varbind-value">{{ formatValue(varbind.value) }}</span>
              </div>
            </div>
          </div>
          <div v-if="parsedVarBinds.length === 0" class="varbinds-empty">
            无变量绑定数据
          </div>
        </div>
      </div>

      <!-- 原始数据 -->
      <div class="detail-section">
        <h4 class="section-title">原始数据</h4>
        <div class="raw-data-container">
          <pre class="raw-data-pre">{{ JSON.stringify(trap, null, 2) }}</pre>
        </div>
      </div>
    </div>

    <template #footer>
      <el-button v-if="!trap?.acknowledged" type="primary" @click="handleAcknowledge">
        确认
      </el-button>
      <el-button @click="handleClose">关闭</el-button>
    </template>
  </el-drawer>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { TrapRecord, TrapVarBind } from '@/types/snmp'

// Props
interface Props {
  modelValue: boolean
  trap: TrapRecord | null
}

const props = defineProps<Props>()

// Emits
const emit = defineEmits<{
  (e: 'update:modelValue', val: boolean): void
  (e: 'acknowledge', id: number): void
}>()

// 对话框可见性
const visible = computed({
  get: () => props.modelValue,
  set: (val) => emit('update:modelValue', val),
})

// 解析变量绑定
const parsedVarBinds = computed<TrapVarBind[]>(() => {
  if (!props.trap?.variables) return []
  try {
    return JSON.parse(props.trap.variables)
  } catch {
    return []
  }
})

// 确认
function handleAcknowledge() {
  if (props.trap) {
    emit('acknowledge', props.trap.id)
  }
}

// 关闭
function handleClose() {
  visible.value = false
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

// 格式化值
function formatValue(value: unknown): string {
  if (value === null || value === undefined) return '-'
  if (typeof value === 'object') return JSON.stringify(value)
  return String(value)
}

// 获取严重级别标签类型
function getSeverityTagType(severity: string): 'danger' | 'warning' | 'info' | 'success' | '' {
  switch (severity?.toLowerCase()) {
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
  switch (severity?.toLowerCase()) {
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
</script>

<style scoped>
.trap-detail-content {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.detail-section {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.section-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary);
  padding-bottom: 8px;
  border-bottom: 1px solid var(--border);
  margin: 0;
}

.detail-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 8px 16px;
}

.detail-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.detail-item.full-width {
  grid-column: span 2;
}

.detail-label {
  font-size: 12px;
  color: var(--text-muted);
}

.detail-value {
  font-size: 13px;
  color: var(--text-primary);
  word-break: break-all;
}

.detail-value.oid-value {
  font-family: 'Consolas', monospace;
  font-size: 12px;
}

.varbinds-container {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.varbind-item {
  background-color: var(--bg-tertiary);
  border-radius: 6px;
  border: 1px solid var(--border);
  overflow: hidden;
}

.varbind-header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  background-color: var(--bg-hover);
  border-bottom: 1px solid var(--border);
}

.varbind-index {
  font-size: 11px;
  font-weight: 600;
  color: var(--accent);
}

.varbind-oid {
  font-size: 11px;
  font-family: 'Consolas', monospace;
  color: var(--text-secondary);
}

.varbind-body {
  padding: 8px 12px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.varbind-row {
  display: flex;
  gap: 8px;
  font-size: 12px;
}

.varbind-label {
  color: var(--text-muted);
  flex-shrink: 0;
}

.varbind-value {
  color: var(--text-primary);
  word-break: break-all;
}

.varbinds-empty {
  text-align: center;
  padding: 16px;
  color: var(--text-muted);
  font-size: 12px;
}

.raw-data-container {
  background-color: var(--bg-tertiary);
  border-radius: 6px;
  border: 1px solid var(--border);
  overflow: auto;
  max-height: 200px;
}

.raw-data-pre {
  margin: 0;
  padding: 12px;
  font-size: 11px;
  font-family: 'Consolas', monospace;
  color: var(--text-secondary);
  white-space: pre-wrap;
  word-break: break-all;
}
</style>