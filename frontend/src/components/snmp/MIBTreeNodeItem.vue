<template>
  <div class="mib-tree-node-item">
    <!-- 节点行 -->
    <div
      class="node-row"
      :style="{ paddingLeft: `${level * 16 + 8}px` }"
      :class="{ 'is-selected': isSelected }"
      @click="handleSelect"
    >
      <!-- 展开/折叠按钮 -->
      <span
        v-if="node.hasChildren"
        class="expand-icon"
        @click.stop="handleToggle"
      >
        <el-icon v-if="isExpanded"><ArrowDown /></el-icon>
        <el-icon v-else><ArrowRight /></el-icon>
      </span>
      <span v-else class="expand-placeholder"></span>

      <!-- 节点类型图标 -->
      <span class="node-type-icon" :class="getTypeIconClass()">
        <el-icon>
          <component :is="getTypeIcon()" />
        </el-icon>
      </span>

      <!-- OID -->
      <span class="node-oid">{{ node.oid }}</span>

      <!-- 名称 -->
      <span class="node-name">{{ node.name }}</span>

      <!-- 类型标签 -->
      <el-tag
        v-if="node.nodeType"
        :type="getTypeTagType()"
        size="small"
        effect="plain"
        class="node-type-tag"
      >
        {{ node.nodeType }}
      </el-tag>
    </div>

    <!-- 子节点 -->
    <div v-if="isExpanded && node.children && node.children.length > 0" class="node-children">
      <MIBTreeNodeItem
        v-for="child in node.children"
        :key="child.id"
        :node="child"
        :level="level + 1"
        :selected-node-id="selectedNodeId"
        :expanded-ids="expandedIds"
        @toggle="(n) => emit('toggle', n)"
        @select="(n) => emit('select', n)"
        @load-children="(n) => emit('load-children', n)"
      />
    </div>

    <!-- 懒加载提示 -->
    <div v-if="isExpanded && node.hasChildren && (!node.children || node.children.length === 0)" class="loading-children">
      <el-icon class="is-loading"><Loading /></el-icon>
      <span>加载中...</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { ArrowDown, ArrowRight, Loading, Document, Folder, FolderOpened, Tickets, Warning } from '@element-plus/icons-vue'
import type { MIBTreeNode } from '@/types/snmp'

// Props
interface Props {
  node: MIBTreeNode
  level: number
  selectedNodeId?: number | null
  expandedIds: Set<number>
}

const props = defineProps<Props>()

// Emits
const emit = defineEmits<{
  (e: 'toggle', node: MIBTreeNode): void
  (e: 'select', node: MIBTreeNode): void
  (e: 'load-children', node: MIBTreeNode): void
}>()

// 是否展开
const isExpanded = computed(() => props.expandedIds.has(props.node.id))

// 是否选中
const isSelected = computed(() => props.selectedNodeId === props.node.id)

// 展开/折叠
function handleToggle() {
  emit('toggle', props.node)
  // 如果是首次展开且没有子节点数据，触发懒加载
  if (!isExpanded.value && props.node.hasChildren && (!props.node.children || props.node.children.length === 0)) {
    emit('load-children', props.node)
  }
}

// 选择节点
function handleSelect() {
  emit('select', props.node)
}

// 获取节点类型图标
function getTypeIcon() {
  const type = props.node.nodeType?.toLowerCase() || ''
  switch (type) {
    case 'scalar':
      return Document
    case 'table':
      return Folder
    case 'row':
      return FolderOpened
    case 'column':
      return Tickets
    case 'notification':
      return Warning
    default:
      return Document
  }
}

// 获取节点类型图标样式类
function getTypeIconClass() {
  const type = props.node.nodeType?.toLowerCase() || ''
  switch (type) {
    case 'scalar':
      return 'type-scalar'
    case 'table':
      return 'type-table'
    case 'row':
      return 'type-row'
    case 'column':
      return 'type-column'
    case 'notification':
      return 'type-notification'
    default:
      return 'type-default'
  }
}

// 获取类型标签样式
function getTypeTagType(): 'success' | 'warning' | 'danger' | 'info' | '' {
  const type = props.node.nodeType?.toLowerCase() || ''
  switch (type) {
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
      return ''
  }
}
</script>

<style scoped>
.mib-tree-node-item {
  user-select: none;
}

.node-row {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 8px;
  border-radius: 4px;
  cursor: pointer;
  transition: background-color 0.15s;
}

.node-row:hover {
  background-color: var(--color-bg-hover);
}

.node-row.is-selected {
  background-color: var(--color-accent-bg);
  color: var(--color-accent-primary);
}

.expand-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 16px;
  height: 16px;
  color: var(--color-text-muted);
  cursor: pointer;
}

.expand-placeholder {
  width: 16px;
}

.node-type-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 18px;
  height: 18px;
  font-size: 14px;
}

.type-scalar {
  color: var(--color-info);
}

.type-table {
  color: var(--color-success);
}

.type-row {
  color: var(--color-warning);
}

.type-column {
  color: var(--color-text-muted);
}

.type-notification {
  color: var(--color-error);
}

.type-default {
  color: var(--color-text-secondary);
}

.node-oid {
  font-size: 12px;
  font-family: 'Consolas', monospace;
  color: var(--color-text-secondary);
}

.node-name {
  font-size: 12px;
  color: var(--color-text-primary);
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.node-type-tag {
  font-size: 10px;
  flex-shrink: 0;
}

.node-children {
  border-left: 1px dashed var(--color-border-default);
  margin-left: 12px;
}

.loading-children {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 16px;
  color: var(--color-text-muted);
  font-size: 12px;
}
</style>