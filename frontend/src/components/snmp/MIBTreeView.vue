<template>
  <div class="mib-tree-view">
    <!-- 搜索栏 -->
    <div class="tree-search">
      <el-input
        v-model="searchQuery"
        placeholder="搜索 OID 或名称..."
        clearable
        size="small"
        prefix-icon="Search"
        @input="handleSearchInput"
      >
        <template #suffix>
          <span v-if="searchResults.length > 0" class="search-count">
            {{ searchResults.length }} 结果
          </span>
        </template>
      </el-input>
    </div>

    <!-- 工具栏 -->
    <div class="tree-toolbar">
      <el-button size="small" link @click="handleExpandAll">全部展开</el-button>
      <el-button size="small" link @click="handleCollapseAll">全部折叠</el-button>
    </div>

    <!-- 搜索结果列表 -->
    <div v-if="isSearchMode && searchResults.length > 0" class="search-results">
      <div
        v-for="node in searchResults"
        :key="node.id"
        class="search-result-item"
        :class="{ 'is-selected': isNodeSelected(node.id) }"
        @click="handleNodeClick(node)"
      >
        <div class="result-oid">{{ node.oid }}</div>
        <div class="result-name">{{ node.name }}</div>
        <el-tag v-if="node.type" size="small" type="info" effect="plain" class="result-type">
          {{ node.type }}
        </el-tag>
      </div>
    </div>

    <!-- 无搜索结果 -->
    <div v-else-if="isSearchMode && searchResults.length === 0 && !searchLoading" class="tree-empty">
      <span>未找到匹配的节点</span>
    </div>

    <!-- 树形视图 -->
    <div v-else class="tree-content">
      <div v-if="loading" class="tree-loading">
        <el-icon class="is-loading"><Loading /></el-icon>
        <span>加载中...</span>
      </div>
      <div v-else-if="treeData.length === 0" class="tree-empty">
        <span>请选择 MIB 模块</span>
      </div>
      <div v-else class="tree-nodes">
        <MIBTreeNodeItem
          v-for="node in treeData"
          :key="node.id"
          :node="node"
          :level="0"
          :selected-node-id="selectedNodeId"
          :expanded-ids="expandedIds"
          @toggle="handleToggle"
          @select="handleNodeClick"
          @load-children="handleLoadChildren"
        />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { Loading } from '@element-plus/icons-vue'
import { SNMPMIBAPI } from '@/services/snmpApi'
import type { MIBTreeNode, MIBNode } from '@/types/snmp'

// 子树节点组件
import MIBTreeNodeItem from './MIBTreeNodeItem.vue'

// Props
interface Props {
  treeData: MIBTreeNode[]
  loading?: boolean
  selectedNodeId?: number | null
}

const props = withDefaults(defineProps<Props>(), {
  loading: false,
  selectedNodeId: null,
})

// Emits
const emit = defineEmits<{
  (e: 'node-click', node: MIBTreeNode | MIBNode): void
  (e: 'expand-all'): void
  (e: 'collapse-all'): void
}>()

// 搜索状态
const searchQuery = ref('')
const searchResults = ref<MIBNode[]>([])
const searchLoading = ref(false)
const isSearchMode = computed(() => searchQuery.value.trim().length > 0)

// 展开状态
const expandedIds = ref<Set<number>>(new Set())

// 是否选中节点
function isNodeSelected(id: number): boolean {
  return props.selectedNodeId === id
}

// 搜索输入处理（防抖）
let searchTimeout: ReturnType<typeof setTimeout> | null = null
function handleSearchInput() {
  if (searchTimeout) clearTimeout(searchTimeout)
  searchTimeout = setTimeout(() => {
    handleSearch()
  }, 300)
}

// 执行搜索
async function handleSearch() {
  const query = searchQuery.value.trim()
  if (!query) {
    searchResults.value = []
    return
  }

  searchLoading.value = true
  try {
    searchResults.value = await SNMPMIBAPI.searchMIBNodes(query)
  } catch {
    searchResults.value = []
  } finally {
    searchLoading.value = false
  }
}

// 展开/折叠节点
function handleToggle(node: MIBTreeNode) {
  if (expandedIds.value.has(node.id)) {
    expandedIds.value.delete(node.id)
  } else {
    expandedIds.value.add(node.id)
  }
}

// 点击节点
function handleNodeClick(node: MIBTreeNode | MIBNode) {
  emit('node-click', node)
}

// 懒加载子节点
async function handleLoadChildren(node: MIBTreeNode) {
  if (node.children && node.children.length > 0) return

  try {
    // 通过获取节点详情来加载子节点
    const detail = await SNMPMIBAPI.getMIBNode(node.id)
    if (detail && detail.childrenIds && detail.childrenIds.length > 0) {
      // 标记为已加载
      node.hasChildren = true
    }
  } catch {
    // 静默处理
  }
}

// 全部展开
function handleExpandAll() {
  if (isSearchMode.value) return
  emit('expand-all')
}

// 全部折叠
function handleCollapseAll() {
  if (isSearchMode.value) return
  expandedIds.value.clear()
  emit('collapse-all')
}

// 监听树数据变化，重置展开状态
watch(() => props.treeData, () => {
  expandedIds.value.clear()
  searchQuery.value = ''
  searchResults.value = []
})
</script>

<style scoped>
.mib-tree-view {
  display: flex;
  flex-direction: column;
  height: 100%;
  gap: 8px;
}

.tree-search {
  padding: 0 4px;
}

.search-count {
  font-size: 11px;
  color: var(--color-text-muted);
}

.tree-toolbar {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  padding: 0 4px;
}

.tree-content {
  flex: 1;
  overflow-y: auto;
}

.tree-nodes {
  padding: 4px 0;
}

.tree-loading,
.tree-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 24px;
  color: var(--color-text-muted);
  font-size: 13px;
}

.search-results {
  flex: 1;
  overflow-y: auto;
  padding: 4px;
}

.search-result-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 8px;
  border-radius: 4px;
  cursor: pointer;
  transition: background-color 0.15s;
}

.search-result-item:hover {
  background-color: var(--color-bg-hover);
}

.search-result-item.is-selected {
  background-color: var(--color-accent-bg);
  color: var(--color-accent-primary);
}

.result-oid {
  font-size: 12px;
  font-family: 'Consolas', monospace;
  color: var(--color-text-secondary);
  flex-shrink: 0;
}

.result-name {
  font-size: 12px;
  color: var(--color-text-primary);
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.result-type {
  flex-shrink: 0;
}
</style>