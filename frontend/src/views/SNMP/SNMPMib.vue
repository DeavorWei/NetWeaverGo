<script setup lang="ts">
/**
 * SNMP MIB 管理页面
 *
 * 功能：
 * - MIB 模块列表管理（左侧面板）
 * - MIB 树形视图（中间面板）
 * - 节点详情面板（右侧面板）
 * - 导入/删除 MIB 模块
 * - OID 搜索功能
 * - 实时导入进度显示
 */
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { ElMessageBox } from 'element-plus'
import { SNMPMIBAPI, SNMPEvents } from '@/services/snmpApi'
import { useMIBTree } from '@/composables/useMIBTree'
import { useToast } from '@/utils/useToast'
import { getLogger } from '@/utils/logger'
import { Dialogs } from '@wailsio/runtime'
import type {
  MIBModuleVM,
  MIBNodeVM,
} from '@/bindings/github.com/NetWeaverGo/core/internal/ui/models'
import type { MIBTreeNode } from '@/bindings/github.com/NetWeaverGo/core/internal/snmp/models'

/** MIB 导入进度接口 */
interface MIBImportProgress {
  fileName: string
  moduleName: string
  phase: 'parsing' | 'saving' | 'caching' | 'completed' | 'error'
  progress: number
  nodesDone: number
  nodesTotal: number
  error?: string
}

const logger = getLogger()
const toast = useToast()

// ==================== 状态 ====================

/** MIB 模块列表 */
const modules = ref<MIBModuleVM[]>([])
const modulesLoading = ref(false)

/** 当前选中的模块 ID */
const selectedModuleId = ref<number | null>(null)

/** MIB 树数据 */
const mibTreeData = ref<MIBTreeNode[]>([])
const treeLoading = ref(false)

/** 当前选中的节点详情 */
const selectedNodeDetail = ref<MIBNodeVM | null>(null)
const nodeDetailLoading = ref(false)

/** OID 解析结果 */
const oidResolveResult = ref<{ oid: string; name: string; moduleName: string; description: string; type: string; access: string; status: string; found: boolean } | null>(null)

/** 搜索相关 */
const searchQuery = ref('')
const searchResults = ref<MIBNodeVM[]>([])
const searchLoading = ref(false)

/** 导入相关 */
const showImportModal = ref(false)
const importFilePaths = ref<string[]>([])
const importProgress = ref<MIBImportProgress | null>(null)
const importing = ref(false)

/** 缓存统计 */
const cacheStats = ref<Record<string, number>>({})

/** 面板宽度 */
const leftPanelWidth = ref(280)
const rightPanelWidth = ref(320)

// ==================== 组合式函数 ====================

const {
  flattenedNodes,
  toggleNode,
  expandAll,
  collapseAll,
  selectNode,
  isNodeExpanded,
  isNodeSelected,
} = useMIBTree(mibTreeData)

// ==================== 计算属性 ====================

/** 当前选中的模块 */
const selectedModule = computed(() => {
  if (!selectedModuleId.value) return null
  return modules.value.find(m => m.id === selectedModuleId.value)
})

/** 搜索结果数量 */
const searchResultCount = computed(() => searchResults.value.length)

// ==================== 方法 ====================

/**
 * 加载 MIB 模块列表
 */
async function loadModules() {
  modulesLoading.value = true
  try {
    modules.value = await SNMPMIBAPI.getMIBModules()
    logger.info(`SNMP: MIB 模块列表已加载 - ${modules.value.length} 个`)
  } catch (error) {
    logger.error(`SNMP: 加载 MIB 模块列表失败 - ${error}`)
    toast.error('加载 MIB 模块列表失败')
  } finally {
    modulesLoading.value = false
  }
}

/**
 * 选择模块并加载其树形结构
 */
async function selectModule(moduleId: number | null) {
  selectedModuleId.value = moduleId
  selectedNodeDetail.value = null
  oidResolveResult.value = null

  if (moduleId === null) {
    mibTreeData.value = []
    return
  }

  treeLoading.value = true
  try {
    mibTreeData.value = await SNMPMIBAPI.getMIBTree(moduleId)
    logger.debug(`SNMP: MIB 树已加载 - ${mibTreeData.value.length} 个节点`)
  } catch (error) {
    logger.error(`SNMP: 加载 MIB 树失败 - ${error}`)
    toast.error('加载 MIB 树失败')
    mibTreeData.value = []
  } finally {
    treeLoading.value = false
  }
}

/**
 * 选择节点并加载详情
 */
async function selectMIBNode(nodeId: number) {
  selectNode(nodeId)
  nodeDetailLoading.value = true
  try {
    selectedNodeDetail.value = await SNMPMIBAPI.getMIBNode(nodeId)
    // 同时解析 OID
    if (selectedNodeDetail.value) {
      oidResolveResult.value = await SNMPMIBAPI.resolveOID(selectedNodeDetail.value.oid)
    }
  } catch (error) {
    logger.error(`SNMP: 加载节点详情失败 - ${error}`)
    toast.error('加载节点详情失败')
    selectedNodeDetail.value = null
  } finally {
    nodeDetailLoading.value = false
  }
}

/**
 * 搜索 MIB 节点
 */
async function handleSearch() {
  const query = searchQuery.value.trim()
  if (!query) {
    searchResults.value = []
    return
  }

  searchLoading.value = true
  try {
    searchResults.value = await SNMPMIBAPI.searchMIBNodes(query)
    logger.debug(`SNMP: 搜索结果 - ${searchResults.value.length} 个`)
  } catch (error) {
    logger.error(`SNMP: 搜索失败 - ${error}`)
    toast.error('搜索失败')
    searchResults.value = []
  } finally {
    searchLoading.value = false
  }
}

// selectSearchResult 函数已移除 - 搜索结果直接点击跳转

/**
 * 打开导入对话框
 */
function openImportModal() {
  importFilePaths.value = []
  importProgress.value = null
  showImportModal.value = true
}

/**
 * 选择 MIB 文件
 * 使用 Wails 文件对话框 API 选择一个或多个 MIB 文件
 */
async function selectMIBFiles() {
  try {
    const selectedFiles = await Dialogs.OpenFile({
      Title: '选择 MIB 文件',
      Message: '请选择要导入的 MIB 文件（支持 .mib, .my, .txt 格式）',
      CanChooseFiles: true,
      CanChooseDirectories: false,
      AllowsMultipleSelection: true,
      ShowHiddenFiles: false,
      Filters: [
        { DisplayName: 'MIB 文件', Pattern: '*.mib;*.my;*.txt' },
        { DisplayName: '所有文件', Pattern: '*.*' },
      ],
    })

    // OpenFile 返回 string | string[]，需要处理两种情况
    if (typeof selectedFiles === 'string') {
      if (selectedFiles) {
        importFilePaths.value = [selectedFiles]
        logger.info(`SNMP: 已选择 MIB 文件 - ${selectedFiles}`)
      }
    } else if (Array.isArray(selectedFiles) && selectedFiles.length > 0) {
      importFilePaths.value = selectedFiles
      logger.info(`SNMP: 已选择 ${selectedFiles.length} 个 MIB 文件`)
    }
  } catch (error) {
    logger.error(`SNMP: 文件选择失败 - ${error}`)
    toast.error('文件选择失败')
  }
}

/**
 * 执行导入
 */
async function executeImport() {
  if (importFilePaths.value.length === 0) {
    toast.warning('请选择要导入的 MIB 文件')
    return
  }

  importing.value = true
  importProgress.value = {
    fileName: '',
    moduleName: '',
    phase: 'parsing',
    progress: 0,
    nodesDone: 0,
    nodesTotal: 0,
  }

  try {
    const firstFile = importFilePaths.value[0]
    if (importFilePaths.value.length === 1 && firstFile) {
      await SNMPMIBAPI.importMIB({
        filePath: firstFile,
        partialImport: true,
      })
    } else {
      await SNMPMIBAPI.importMIBFiles(importFilePaths.value)
    }
    toast.success('MIB 文件导入成功')
    showImportModal.value = false
    await loadModules()
    await loadCacheStats()
  } catch (error) {
    logger.error(`SNMP: 导入 MIB 文件失败 - ${error}`)
    toast.error('导入 MIB 文件失败')
  } finally {
    importing.value = false
    importProgress.value = null
  }
}

/**
 * 删除模块
 */
async function deleteModule(module: MIBModuleVM) {
  try {
    await ElMessageBox.confirm(
      `确定要删除 MIB 模块 "${module.name}" 吗？此操作不可恢复。`,
      '删除确认',
      {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning',
      }
    )
  } catch {
    return // 用户取消
  }

  try {
    await SNMPMIBAPI.deleteMIBModule(module.id)
    toast.success(`MIB 模块 "${module.name}" 已删除`)
    await loadModules()
    if (selectedModuleId.value === module.id) {
      await selectModule(null)
    }
    await loadCacheStats()
  } catch (error) {
    logger.error(`SNMP: 删除 MIB 模块失败 - ${error}`)
    toast.error('删除 MIB 模块失败')
  }
}

/**
 * 清除缓存
 */
async function clearCache() {
  try {
    await ElMessageBox.confirm('确定要清除 OID 解析器缓存吗？', '清除缓存', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning',
    })
  } catch {
    return // 用户取消
  }

  try {
    await SNMPMIBAPI.clearResolverCache()
    toast.success('缓存已清除')
    await loadCacheStats()
  } catch (error) {
    logger.error(`SNMP: 清除缓存失败 - ${error}`)
    toast.error('清除缓存失败')
  }
}

/**
 * 重建缓存
 */
async function rebuildCache() {
  try {
    await SNMPMIBAPI.rebuildResolverCache()
    toast.success('缓存已重建')
    await loadCacheStats()
  } catch (error) {
    logger.error(`SNMP: 重建缓存失败 - ${error}`)
    toast.error('重建缓存失败')
  }
}

/**
 * 加载缓存统计
 */
async function loadCacheStats() {
  try {
    const stats = await SNMPMIBAPI.getCacheStats()
    cacheStats.value = stats as Record<string, number>
  } catch (error) {
    logger.error(`SNMP: 加载缓存统计失败 - ${error}`)
  }
}

/**
 * 格式化时间
 */
function formatTime(timeStr: string): string {
  if (!timeStr) return '-'
  try {
    const date = new Date(timeStr)
    return date.toLocaleString('zh-CN')
  } catch {
    return timeStr
  }
}

/**
 * 获取状态样式类
 */
function getStatusClass(status: string): string {
  switch (status) {
    case 'active':
      return 'text-success'
    case 'error':
      return 'text-error'
    case 'partial':
      return 'text-warning'
    default:
      return 'text-text-muted'
  }
}

/**
 * 获取访问权限样式类
 */
function getAccessClass(access: string): string {
  switch (access.toLowerCase()) {
    case 'read-only':
    case 'readonly':
      return 'bg-info/20 text-info'
    case 'read-write':
    case 'readwrite':
      return 'bg-success/20 text-success'
    case 'write-only':
    case 'writeonly':
      return 'bg-warning/20 text-warning'
    case 'not-accessible':
    case 'notaccessible':
      return 'bg-error/20 text-error'
    default:
      return 'bg-bg-hover text-text-muted'
  }
}

// ==================== 事件监听 ====================

let unsubscribeImportProgress: (() => void) | null = null
let unsubscribeMIBImported: (() => void) | null = null
let unsubscribeMIBDeleted: (() => void) | null = null

function setupEventListeners() {
  // 监听导入进度
  unsubscribeImportProgress = SNMPEvents.onMIBImportProgress((progress: unknown) => {
    importProgress.value = progress as MIBImportProgress
  })

  // 监听导入完成
  unsubscribeMIBImported = SNMPEvents.onMIBImported((module: unknown) => {
    const moduleVM = module as { name: string }
    toast.success(`MIB 模块 "${moduleVM.name}" 导入完成`)
    loadModules()
    loadCacheStats()
  })

  // 监听删除事件
  unsubscribeMIBDeleted = SNMPEvents.onMIBDeleted((moduleID) => {
    if (selectedModuleId.value === moduleID) {
      selectModule(null)
    }
    loadModules()
  })
}

function cleanupEventListeners() {
  unsubscribeImportProgress?.()
  unsubscribeMIBImported?.()
  unsubscribeMIBDeleted?.()
}

// ==================== 生命周期 ====================

onMounted(() => {
  loadModules()
  loadCacheStats()
  setupEventListeners()
})

onUnmounted(() => {
  cleanupEventListeners()
})

// 监听搜索输入（防抖）
let searchTimeout: ReturnType<typeof setTimeout> | null = null
watch(searchQuery, () => {
  if (searchTimeout) clearTimeout(searchTimeout)
  searchTimeout = setTimeout(() => {
    handleSearch()
  }, 300)
})
</script>

<template>
  <div class="animate-slide-in h-full flex gap-4">
    <!-- 左侧面板：MIB 模块列表 -->
    <div
      class="flex-shrink-0 bg-bg-secondary rounded-lg border border-border overflow-hidden flex flex-col"
      :style="{ width: `${leftPanelWidth}px` }"
    >
      <!-- 模块列表头部 -->
      <div class="px-4 py-3 border-b border-border flex items-center justify-between">
        <h2 class="text-sm font-semibold text-text-primary">MIB 模块</h2>
        <button
          @click="openImportModal"
          class="px-2 py-1 text-xs bg-accent text-white rounded hover:bg-accent-hover transition-colors"
        >
          导入
        </button>
      </div>

      <!-- 模块列表 -->
      <div class="flex-1 overflow-y-auto scrollbar-custom">
        <div v-if="modulesLoading" class="p-4 text-center text-text-muted">
          加载中...
        </div>
        <div v-else-if="modules.length === 0" class="p-4 text-center text-text-muted">
          暂无 MIB 模块
        </div>
        <ul v-else class="divide-y divide-border">
          <li
            v-for="module in modules"
            :key="module.id"
            @click="selectModule(module.id)"
            :class="[
              'px-4 py-3 cursor-pointer transition-colors',
              selectedModuleId === module.id
                ? 'bg-accent-bg border-l-2 border-accent'
                : 'hover:bg-bg-hover',
            ]"
          >
            <div class="flex items-center justify-between">
              <span class="text-sm font-medium text-text-primary">{{ module.name }}</span>
              <div class="flex items-center gap-2">
                <span :class="['text-xs', getStatusClass(module.status)]">
                  {{ module.status }}
                </span>
                <button
                  @click.stop="deleteModule(module)"
                  class="p-1 text-text-muted hover:text-red-400 transition-colors"
                  title="删除模块"
                >
                  <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                  </svg>
                </button>
              </div>
            </div>
            <div class="mt-1 flex items-center gap-2 text-xs text-text-muted">
              <span>{{ module.nodeCount }} 节点</span>
              <span>·</span>
              <span>{{ formatTime(module.importedAt) }}</span>
            </div>
          </li>
        </ul>
      </div>

      <!-- 缓存统计 -->
      <div class="px-4 py-2 border-t border-border text-xs text-text-muted">
        <div class="flex items-center justify-between">
          <span>缓存: {{ cacheStats.oidCacheLen }} OID / {{ cacheStats.nameCacheLen }} 名称</span>
          <div class="flex gap-2">
            <button @click="clearCache" class="hover:text-text-primary transition-colors">
              清除
            </button>
            <button @click="rebuildCache" class="hover:text-text-primary transition-colors">
              重建
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- 中间面板：MIB 树形视图 -->
    <div class="flex-1 bg-bg-secondary rounded-lg border border-border overflow-hidden flex flex-col min-w-0">
      <!-- 树形视图头部 -->
      <div class="px-4 py-3 border-b border-border flex items-center justify-between">
        <div class="flex items-center gap-2">
          <h2 class="text-sm font-semibold text-text-primary">
            {{ selectedModule?.name || 'MIB 树' }}
          </h2>
          <span v-if="selectedModule" class="text-xs text-text-muted">
            ({{ selectedModule.nodeCount }} 节点)
          </span>
        </div>
        <div class="flex items-center gap-2">
          <button
            @click="expandAll"
            class="px-2 py-1 text-xs text-text-muted hover:text-text-primary transition-colors"
          >
            全部展开
          </button>
          <button
            @click="collapseAll"
            class="px-2 py-1 text-xs text-text-muted hover:text-text-primary transition-colors"
          >
            全部折叠
          </button>
        </div>
      </div>

      <!-- 搜索栏 -->
      <div class="px-4 py-2 border-b border-border">
        <div class="relative">
          <input
            v-model="searchQuery"
            type="text"
            placeholder="搜索 OID 或名称..."
            class="w-full px-3 py-2 pl-9 text-sm bg-bg-primary border border-border rounded-lg focus:outline-none focus:border-accent transition-colors"
          />
          <svg
            class="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-text-muted"
            xmlns="http://www.w3.org/2000/svg"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
          >
            <circle cx="11" cy="11" r="8" />
            <path d="M21 21l-4.35-4.35" />
          </svg>
          <span
            v-if="searchResultCount > 0"
            class="absolute right-3 top-1/2 -translate-y-1/2 text-xs text-text-muted"
          >
            {{ searchResultCount }} 结果
          </span>
        </div>
      </div>

      <!-- 树形视图内容 -->
      <div class="flex-1 overflow-y-auto scrollbar-custom">
        <div v-if="treeLoading" class="p-4 text-center text-text-muted">
          加载中...
        </div>
        <div v-else-if="!selectedModuleId" class="p-4 text-center text-text-muted">
          请选择一个 MIB 模块
        </div>
        <div v-else-if="mibTreeData.length === 0" class="p-4 text-center text-text-muted">
          该模块没有节点数据
        </div>
        <div v-else class="py-1">
          <template v-for="item in flattenedNodes" :key="item.node.id">
            <div
              @click="selectMIBNode(item.node.id)"
              :class="[
                'px-4 py-1.5 cursor-pointer transition-colors flex items-center gap-2',
                isNodeSelected(item.node.id)
                  ? 'bg-accent-bg text-accent'
                  : 'hover:bg-bg-hover text-text-primary',
              ]"
              :style="{ paddingLeft: `${item.level * 16 + 16}px` }"
            >
              <!-- 展开/折叠按钮 -->
              <button
                v-if="item.node.hasChildren"
                @click.stop="toggleNode(item.node.id)"
                class="w-4 h-4 flex items-center justify-center text-text-muted hover:text-text-primary"
              >
                <svg
                  class="w-3 h-3 transition-transform"
                  :class="{ 'rotate-90': isNodeExpanded(item.node.id) }"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="2"
                >
                  <path d="M9 18l6-6-6-6" />
                </svg>
              </button>
              <span v-else class="w-4"></span>

              <!-- 节点图标 -->
              <span class="w-4 h-4 text-text-muted">
                <svg v-if="item.node.nodeType === 'scalar'" viewBox="0 0 24 24" fill="currentColor">
                  <circle cx="12" cy="12" r="4" />
                </svg>
                <svg v-else-if="item.node.nodeType === 'table'" viewBox="0 0 24 24" fill="currentColor">
                  <rect x="4" y="4" width="16" height="16" rx="2" />
                </svg>
                <svg v-else viewBox="0 0 24 24" fill="currentColor">
                  <path d="M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5" />
                </svg>
              </span>

              <!-- 节点名称 -->
              <span class="text-sm truncate">{{ item.node.name }}</span>
              <span class="text-xs text-text-muted truncate">({{ item.node.oid }})</span>
            </div>
          </template>
        </div>
      </div>
    </div>

    <!-- 右侧面板：节点详情 -->
    <div
      class="flex-shrink-0 bg-bg-secondary rounded-lg border border-border overflow-hidden flex flex-col"
      :style="{ width: `${rightPanelWidth}px` }"
    >
      <!-- 详情头部 -->
      <div class="px-4 py-3 border-b border-border">
        <h2 class="text-sm font-semibold text-text-primary">节点详情</h2>
      </div>

      <!-- 详情内容 -->
      <div class="flex-1 overflow-y-auto scrollbar-custom">
        <div v-if="nodeDetailLoading" class="p-4 text-center text-text-muted">
          加载中...
        </div>
        <div v-else-if="!selectedNodeDetail" class="p-4 text-center text-text-muted">
          选择一个节点查看详情
        </div>
        <div v-else class="p-4 space-y-4">
          <!-- 基本信息 -->
          <div>
            <h3 class="text-xs font-semibold text-text-muted uppercase mb-2">基本信息</h3>
            <dl class="space-y-2">
              <div>
                <dt class="text-xs text-text-muted">名称</dt>
                <dd class="text-sm text-text-primary font-mono">{{ selectedNodeDetail.name }}</dd>
              </div>
              <div>
                <dt class="text-xs text-text-muted">OID</dt>
                <dd class="text-sm text-text-primary font-mono">{{ selectedNodeDetail.oid }}</dd>
              </div>
              <div>
                <dt class="text-xs text-text-muted">类型</dt>
                <dd class="text-sm text-text-primary">{{ selectedNodeDetail.type || '-' }}</dd>
              </div>
              <div>
                <dt class="text-xs text-text-muted">访问权限</dt>
                <dd>
                  <span :class="['px-2 py-0.5 text-xs rounded', getAccessClass(selectedNodeDetail.access)]">
                    {{ selectedNodeDetail.access || '-' }}
                  </span>
                </dd>
              </div>
              <div>
                <dt class="text-xs text-text-muted">状态</dt>
                <dd class="text-sm text-text-primary">{{ selectedNodeDetail.status || '-' }}</dd>
              </div>
            </dl>
          </div>

          <!-- 描述 -->
          <div v-if="selectedNodeDetail.description">
            <h3 class="text-xs font-semibold text-text-muted uppercase mb-2">描述</h3>
            <p class="text-sm text-text-primary whitespace-pre-wrap">{{ selectedNodeDetail.description }}</p>
          </div>

          <!-- OID 解析结果 -->
          <div v-if="oidResolveResult">
            <h3 class="text-xs font-semibold text-text-muted uppercase mb-2">OID 解析</h3>
            <dl class="space-y-2">
              <div>
                <dt class="text-xs text-text-muted">模块</dt>
                <dd class="text-sm text-text-primary">{{ oidResolveResult.moduleName || '-' }}</dd>
              </div>
              <div v-if="oidResolveResult.found">
                <dt class="text-xs text-text-muted">匹配状态</dt>
                <dd class="text-sm text-success">已找到</dd>
              </div>
            </dl>
          </div>
        </div>
      </div>
    </div>

    <!-- 导入模态框 -->
    <div
      v-if="showImportModal"
      class="fixed inset-0 bg-black/50 flex items-center justify-center z-50"
      @click.self="showImportModal = false"
    >
      <div class="bg-bg-secondary rounded-lg border border-border w-[480px] max-h-[80vh] overflow-hidden">
        <div class="px-4 py-3 border-b border-border flex items-center justify-between">
          <h3 class="text-sm font-semibold text-text-primary">导入 MIB 文件</h3>
          <button
            @click="showImportModal = false"
            class="text-text-muted hover:text-text-primary transition-colors"
          >
            <svg class="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M18 6L6 18M6 6l12 12" />
            </svg>
          </button>
        </div>

        <div class="p-4 space-y-4">
          <!-- 文件选择 -->
          <div>
            <label class="block text-sm text-text-muted mb-2">选择 MIB 文件</label>
            <div class="flex gap-2">
              <input
                :value="importFilePaths.join('; ')"
                type="text"
                readonly
                placeholder="点击选择文件..."
                class="flex-1 px-3 py-2 text-sm bg-bg-primary border border-border rounded-lg focus:outline-none"
              />
              <button
                @click="selectMIBFiles"
                class="px-3 py-2 text-sm bg-bg-hover text-text-primary rounded-lg hover:bg-border transition-colors"
              >
                浏览
              </button>
            </div>
          </div>

          <!-- 导入进度 -->
          <div v-if="importProgress" class="space-y-2">
            <div class="flex items-center justify-between text-sm">
              <span class="text-text-muted">{{ importProgress.fileName }}</span>
              <span class="text-text-primary">{{ importProgress.progress.toFixed(0) }}%</span>
            </div>
            <div class="h-2 bg-bg-primary rounded-full overflow-hidden">
              <div
                class="h-full bg-accent transition-all"
                :style="{ width: `${importProgress.progress}%` }"
              ></div>
            </div>
            <div class="text-xs text-text-muted">
              {{ importProgress.phase }} - {{ importProgress.nodesDone }} / {{ importProgress.nodesTotal }} 节点
            </div>
          </div>

          <!-- 操作按钮 -->
          <div class="flex justify-end gap-2 pt-2">
            <button
              @click="showImportModal = false"
              class="px-4 py-2 text-sm text-text-muted hover:text-text-primary transition-colors"
            >
              取消
            </button>
            <button
              @click="executeImport"
              :disabled="importing || importFilePaths.length === 0"
              class="px-4 py-2 text-sm bg-accent text-white rounded-lg hover:bg-accent-hover disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            >
              {{ importing ? '导入中...' : '导入' }}
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>