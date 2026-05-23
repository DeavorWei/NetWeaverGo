/**
 * MIB 树形视图组合式函数
 *
 * 提供 MIB 树的状态管理、展开/折叠、搜索和选择功能
 */

import { ref, computed, type Ref } from 'vue'
import type { MIBTreeNode } from '../bindings/github.com/NetWeaverGo/core/internal/snmp/models'

/**
 * 树节点展开状态
 */
export interface TreeNodeState {
  id: number
  expanded: boolean
  visible: boolean
  level: number
  parentPath: number[]
}

/**
 * 搜索匹配结果
 */
export interface SearchResult {
  node: MIBTreeNode
  matchType: 'name' | 'oid' | 'description'
  matchText: string
}

/**
 * useMIBTree 选项
 */
export interface UseMIBTreeOptions {
  /** 初始展开的节点 ID 列表 */
  initialExpanded?: number[]
  /** 是否默认展开所有节点 */
  expandAll?: boolean
  /** 搜索防抖延迟（毫秒） */
  searchDebounce?: number
}

/**
 * MIB 树组合式函数
 *
 * @param treeData - 树形数据源（响应式）
 * @param options - 配置选项
 */
export function useMIBTree(
  treeData: Ref<MIBTreeNode[]>,
  options: UseMIBTreeOptions = {}
) {
  const {
    initialExpanded = [],
    expandAll: initialExpandAll = false,
  } = options

  // ==================== 状态 ====================

  /** 当前选中的节点 ID */
  const selectedNodeId = ref<number | null>(null)

  /** 展开的节点 ID 集合 */
  const expandedNodeIds = ref<Set<number>>(new Set(initialExpanded))

  /** 搜索关键词 */
  const searchQuery = ref('')

  /** 搜索结果 */
  const searchResults = ref<SearchResult[]>([])

  /** 当前搜索结果索引 */
  const currentSearchIndex = ref(-1)

  /** 高亮匹配的节点 ID */
  const highlightedNodeIds = ref<Set<number>>(new Set())

  // ==================== 计算属性 ====================

  /**
   * 扁平化的树节点列表（用于渲染）
   * 根据展开状态过滤可见节点
   */
  const flattenedNodes = computed(() => {
    const result: Array<TreeNodeState & { node: MIBTreeNode }> = []

    function flatten(
      nodes: MIBTreeNode[],
      level: number = 0,
      parentPath: number[] = [],
      parentExpanded: boolean = true
    ) {
      for (const node of nodes) {
        const nodeState: TreeNodeState = {
          id: node.id,
          expanded: expandedNodeIds.value.has(node.id),
          visible: parentExpanded,
          level,
          parentPath,
        }

        result.push({ ...nodeState, node })

        // 如果节点展开且有子节点，递归处理
        if (node.children && node.children.length > 0 && expandedNodeIds.value.has(node.id)) {
          flatten(node.children, level + 1, [...parentPath, node.id], true)
        }
      }
    }

    flatten(treeData.value)
    return result
  })

  /** 当前选中的节点 */
  const selectedNode = computed(() => {
    if (selectedNodeId.value === null) return null
    return findNodeById(treeData.value, selectedNodeId.value)
  })

  /** 当前搜索结果 */
  const currentSearchResult = computed(() => {
    if (currentSearchIndex.value < 0 || currentSearchIndex.value >= searchResults.value.length) {
      return null
    }
    return searchResults.value[currentSearchIndex.value]
  })

  // ==================== 方法 ====================

  /**
   * 切换节点展开状态
   */
  function toggleNode(nodeId: number) {
    if (expandedNodeIds.value.has(nodeId)) {
      expandedNodeIds.value.delete(nodeId)
    } else {
      expandedNodeIds.value.add(nodeId)
    }
  }

  /**
   * 展开节点
   */
  function expandNode(nodeId: number) {
    expandedNodeIds.value.add(nodeId)
  }

  /**
   * 折叠节点
   */
  function collapseNode(nodeId: number) {
    expandedNodeIds.value.delete(nodeId)
  }

  /**
   * 展开所有节点
   */
  function expandAllNodes() {
    function collectIds(nodes: MIBTreeNode[]) {
      for (const node of nodes) {
        if (node.children && node.children.length > 0) {
          expandedNodeIds.value.add(node.id)
          collectIds(node.children)
        }
      }
    }
    collectIds(treeData.value)
  }

  /**
   * 折叠所有节点
   */
  function collapseAllNodes() {
    expandedNodeIds.value.clear()
  }

  /**
   * 展开到指定节点
   */
  function expandToNode(nodeId: number) {
    const path = findNodePath(treeData.value, nodeId)
    if (path) {
      for (const id of path) {
        expandedNodeIds.value.add(id)
      }
    }
  }

  /**
   * 选中节点
   */
  function selectNode(nodeId: number | null) {
    selectedNodeId.value = nodeId
    if (nodeId !== null) {
      // 展开到选中节点
      expandToNode(nodeId)
    }
  }

  /**
   * 搜索节点
   */
  function searchNodes(query: string) {
    searchQuery.value = query
    searchResults.value = []
    currentSearchIndex.value = -1
    highlightedNodeIds.value.clear()

    if (!query.trim()) return

    const results = searchInNodes(treeData.value, query.toLowerCase())
    searchResults.value = results

    // 高亮所有匹配的节点
    for (const result of results) {
      highlightedNodeIds.value.add(result.node.id)
    }

    // 如果有结果，选中第一个
    if (results.length > 0) {
      currentSearchIndex.value = 0
      jumpToSearchResult(0)
    }
  }

  /**
   * 下一个搜索结果
   */
  function nextSearchResult() {
    if (searchResults.value.length === 0) return
    currentSearchIndex.value = (currentSearchIndex.value + 1) % searchResults.value.length
    jumpToSearchResult(currentSearchIndex.value)
  }

  /**
   * 上一个搜索结果
   */
  function prevSearchResult() {
    if (searchResults.value.length === 0) return
    currentSearchIndex.value = currentSearchIndex.value === 0 
      ? searchResults.value.length - 1 
      : currentSearchIndex.value - 1
    jumpToSearchResult(currentSearchIndex.value)
  }

  /**
   * 清除搜索
   */
  function clearSearch() {
    searchQuery.value = ''
    searchResults.value = []
    currentSearchIndex.value = -1
    highlightedNodeIds.value.clear()
  }

  /**
   * 根据ID查找节点
   */
  function findNodeById(nodes: MIBTreeNode[], id: number): MIBTreeNode | null {
    for (const node of nodes) {
      if (node.id === id) return node
      if (node.children) {
        const found = findNodeById(node.children, id)
        if (found) return found
      }
    }
    return null
  }

  /**
   * 查找节点路径
   */
  function findNodePath(
    nodes: MIBTreeNode[],
    targetId: number,
    path: number[] = []
  ): number[] | null {
    for (const node of nodes) {
      const currentPath = [...path, node.id]
      if (node.id === targetId) return currentPath
      if (node.children) {
        const found = findNodePath(node.children, targetId, currentPath)
        if (found) return found
      }
    }
    return null
  }

  /**
   * 收集所有节点ID
   */
  function collectNodeIds(nodes: MIBTreeNode[], ids: Set<number>) {
    for (const node of nodes) {
      ids.add(node.id)
      if (node.children) {
        collectNodeIds(node.children, ids)
      }
    }
  }

  /**
   * 展开所有节点（别名）
   */
  function expandAll() {
    expandAllNodes()
  }

  /**
   * 折叠所有节点（别名）
   */
  function collapseAll() {
    collapseAllNodes()
  }

  /**
   * 检查节点是否展开
   */
  function isNodeExpanded(nodeId: number): boolean {
    return expandedNodeIds.value.has(nodeId)
  }

  /**
   * 检查节点是否选中
   */
  function isNodeSelected(nodeId: number): boolean {
    return selectedNodeId.value === nodeId
  }

  /**
   * 在节点中搜索
   */
  function searchInNodes(
    nodes: MIBTreeNode[],
    query: string
  ): SearchResult[] {
    const results: SearchResult[] = []

    for (const node of nodes) {
      // 搜索名称
      if (node.name.toLowerCase().includes(query)) {
        results.push({
          node,
          matchType: 'name',
          matchText: node.name,
        })
      }
      // 搜索 OID
      else if (node.oid.toLowerCase().includes(query)) {
        results.push({
          node,
          matchType: 'oid',
          matchText: node.oid,
        })
      }
      // 搜索描述
      else if (node.description && node.description.toLowerCase().includes(query)) {
        results.push({
          node,
          matchType: 'description',
          matchText: node.description,
        })
      }

      // 递归搜索子节点
      if (node.children) {
        results.push(...searchInNodes(node.children, query))
      }
    }

    return results
  }

  /**
   * 跳转到搜索结果
   */
  function jumpToSearchResult(index: number) {
    const result = searchResults.value[index]
    if (result) {
      selectNode(result.node.id)
    }
  }

  // 初始化：如果设置了 expandAll，展开所有节点
  if (initialExpandAll) {
    expandAllNodes()
  }

  return {
    // 状态
    selectedNodeId,
    expandedNodeIds,
    searchQuery,
    searchResults,
    currentSearchIndex,
    highlightedNodeIds,

    // 计算属性
    flattenedNodes,
    selectedNode,
    currentSearchResult,

    // 方法
    toggleNode,
    expandNode,
    collapseNode,
    expandAllNodes,
    collapseAllNodes,
    expandAll,
    collapseAll,
    isNodeExpanded,
    isNodeSelected,
    expandToNode,
    selectNode,
    searchNodes,
    nextSearchResult,
    prevSearchResult,
    clearSearch,
    findNodeById,
    findNodePath,
    collectNodeIds,
  }
}
