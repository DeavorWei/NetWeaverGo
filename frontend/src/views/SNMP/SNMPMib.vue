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
import { Search } from '@element-plus/icons-vue'
import { SNMPMIBAPI, SNMPEvents } from '@/services/snmpApi'
import { useMIBTree } from '@/composables/useMIBTree'
import { useToast } from '@/utils/useToast'
import { getLogger } from '@/utils/logger'
import { Dialogs } from '@wailsio/runtime'
import type {
  MIBFolderVM,
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

/** MIB 文件夹列表 */
const folders = ref<MIBFolderVM[]>([])
const foldersLoading = ref(false)
const expandedFolders = ref<Record<string | number, boolean>>({
  'uncategorized': true
})

/** 导入目标文件夹 ID */
const targetFolderId = ref<number | null>(null)

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

/** 树本地搜索相关 */
const treeSearchQuery = ref('')

/** 导入相关 */
const showImportModal = ref(false)
const importType = ref<'files' | 'folder'>('files')
const importFilePaths = ref<string[]>([])
const importFolderPath = ref('')
const importProgress = ref<MIBImportProgress | null>(null)
const importing = ref(false)

/** 缓存统计 */
const cacheStats = ref<Record<string, number>>({})

/** 模态框搜索相关 */
const showSearchModal = ref(false)
const searchScope = ref<'global' | 'module'>('global')
const modalSearchQuery = ref('')
const modalSearchResults = ref<MIBNodeVM[]>([])
const modalSearchLoading = ref(false)
const modalSelectedNode = ref<MIBNodeVM | null>(null)

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
  searchNodes,
  searchResults: searchResultsLocal,
  highlightedNodeIds,
  nextSearchResult,
} = useMIBTree(mibTreeData)

// ==================== 计算属性 ====================

/** 当前选中的模块 */
const selectedModule = computed(() => {
  if (!selectedModuleId.value) return null
  return modules.value.find(m => m.id === selectedModuleId.value)
})

/** 树内本地搜索结果数量 */
const treeSearchCount = computed(() => searchResultsLocal.value.length)

/** 分类后的 MIB 模块 */
const groupedModules = computed(() => {
  const groups: Record<string | number, MIBModuleVM[]> = {}
  const uncategorized: MIBModuleVM[] = []
  
  folders.value.forEach(f => {
    groups[f.id] = []
  })
  
  modules.value.forEach(m => {
    if (m.folderId !== null && m.folderId !== undefined && groups[m.folderId]) {
      groups[m.folderId]!.push(m)
    } else {
      uncategorized.push(m)
    }
  })
  
  groups['uncategorized'] = uncategorized
  return groups
})

// ==================== 方法 ====================

/**
 * 加载 MIB 文件夹列表
 */
async function loadFolders() {
  foldersLoading.value = true
  try {
    const list = await SNMPMIBAPI.getMIBFolders()
    folders.value = list
    list.forEach(f => {
      if (expandedFolders.value[f.id] === undefined) {
        expandedFolders.value[f.id] = true
      }
    })
  } catch (error) {
    logger.error(`SNMP: 加载 MIB 文件夹失败 - ${error}`)
    toast.error('加载 MIB 文件夹失败')
  } finally {
    foldersLoading.value = false
  }
}

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
    expandAll() // 默认全部展开
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
 * 打开全局/模块搜索弹窗
 */
function openSearchModal(scope: 'global' | 'module') {
  searchScope.value = scope
  if (scope === 'module' && !selectedModuleId.value) {
    searchScope.value = 'global'
  }
  
  if (treeSearchQuery.value) {
    modalSearchQuery.value = treeSearchQuery.value
  } else {
    modalSearchQuery.value = ''
  }
  
  modalSearchResults.value = []
  modalSelectedNode.value = null
  showSearchModal.value = true
  
  if (modalSearchQuery.value) {
    executeModalSearch()
  }
}

/**
 * 执行弹窗搜索
 */
async function executeModalSearch() {
  const query = modalSearchQuery.value.trim()
  if (!query) {
    modalSearchResults.value = []
    return
  }

  modalSearchLoading.value = true
  modalSelectedNode.value = null
  try {
    if (searchScope.value === 'module' && selectedModuleId.value !== null) {
      modalSearchResults.value = await SNMPMIBAPI.searchMIBNodesInModule(selectedModuleId.value, query)
    } else {
      modalSearchResults.value = await SNMPMIBAPI.searchMIBNodes(query)
    }
  } catch (error) {
    logger.error(`SNMP: 搜索失败 - ${error}`)
    toast.error('搜索失败')
    modalSearchResults.value = []
  } finally {
    modalSearchLoading.value = false
  }
}

function handleModalSelectNode(row: MIBNodeVM | null) {
  modalSelectedNode.value = row
}

async function handleModalJumpToNode(node: MIBNodeVM) {
  if (!node || !node.moduleId) return
  
  showSearchModal.value = false
  
  if (selectedModuleId.value !== node.moduleId) {
    await selectModule(node.moduleId)
  }
  
  await selectMIBNode(node.id)
  
  setTimeout(() => {
    const el = document.querySelector(`[data-node-id="${node.id}"]`)
    if (el) {
      el.scrollIntoView({ behavior: 'smooth', block: 'center' })
    }
  }, 100)
}

/**
 * 在树中按回车跳转下一个匹配项
 */
function handleTreeSearchEnter() {
  nextSearchResult()
}

/**
 * 打开导入对话框
 */
function openImportModal() {
  importFilePaths.value = []
  importFolderPath.value = ''
  importProgress.value = null
  targetFolderId.value = null
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
 * 选择 MIB 文件夹
 */
async function selectMIBFolder() {
  try {
    const selectedDir = await Dialogs.OpenFile({
      Title: '选择 MIB 文件夹',
      Message: '请选择包含 MIB 文件的文件夹',
      CanChooseDirectories: true,
      CanChooseFiles: false,
      CanCreateDirectories: false,
      AllowsMultipleSelection: false,
      ShowHiddenFiles: false,
    })

    if (selectedDir && typeof selectedDir === 'string') {
      importFolderPath.value = selectedDir
      importFilePaths.value = []
      logger.info(`SNMP: 已选择 MIB 文件夹 - ${selectedDir}`)
    }
  } catch (error) {
    logger.error(`SNMP: 文件夹选择失败 - ${error}`)
    toast.error('文件夹选择失败')
  }
}

/**
 * 执行导入
 */
async function executeImport() {
  if (importType.value === 'files' && importFilePaths.value.length === 0) {
    toast.warning('请选择要导入的 MIB 文件')
    return
  }
  if (importType.value === 'folder' && !importFolderPath.value) {
    toast.warning('请选择要导入的 MIB 文件夹')
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
    if (importType.value === 'folder') {
      await SNMPMIBAPI.importMIBFolder(importFolderPath.value, targetFolderId.value)
    } else {
      const firstFile = importFilePaths.value[0]
      if (importFilePaths.value.length === 1 && firstFile) {
        await SNMPMIBAPI.importMIB({
          filePath: firstFile,
          partialImport: true,
          folderId: targetFolderId.value ?? undefined,
        })
      } else {
        await SNMPMIBAPI.importMIBFiles(importFilePaths.value, targetFolderId.value)
      }
    }
    toast.success('MIB 导入成功')
    showImportModal.value = false
    await loadFolders()
    await loadModules()
    await loadCacheStats()
  } catch (error) {
    logger.error(`SNMP: 导入 MIB 失败 - ${error}`)
    toast.error('导入 MIB 失败')
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
 * 创建文件夹
 */
async function createFolder() {
  try {
    const { value: name } = await ElMessageBox.prompt('请输入文件夹名称', '新建文件夹', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      inputPattern: /\S+/,
      inputErrorMessage: '名称不能为空',
    })
    
    if (name) {
      const folderId = await SNMPMIBAPI.createMIBFolder(name)
      toast.success('文件夹创建成功')
      await loadFolders()
      expandedFolders.value[folderId] = true
    }
  } catch (error) {
    if (error !== 'cancel') {
      logger.error(`SNMP: 创建文件夹失败 - ${error}`)
      toast.error('创建文件夹失败')
    }
  }
}

/**
 * 重命名文件夹
 */
async function renameFolder(folder: MIBFolderVM) {
  try {
    const { value: name } = await ElMessageBox.prompt('请输入文件夹新名称', '重命名文件夹', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      inputValue: folder.name,
      inputPattern: /\S+/,
      inputErrorMessage: '名称不能为空',
    })
    
    if (name && name !== folder.name) {
      await SNMPMIBAPI.renameMIBFolder(folder.id, name)
      toast.success('文件夹已重命名')
      await loadFolders()
    }
  } catch (error) {
    if (error !== 'cancel') {
      logger.error(`SNMP: 重命名文件夹失败 - ${error}`)
      toast.error('重命名文件夹失败')
    }
  }
}

/**
 * 删除文件夹
 */
async function deleteFolder(folder: MIBFolderVM) {
  try {
    await ElMessageBox.confirm(
      `确定要删除文件夹 "${folder.name}" 吗？这将会级联删除该文件夹下的所有 MIB 模块和节点，此操作不可恢复！`,
      '删除确认',
      {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning',
      }
    )
    
    await SNMPMIBAPI.deleteMIBFolder(folder.id)
    toast.success('文件夹已删除')
    
    // 如果选中的模块在被删除的文件夹下，清空当前选中模块
    const affectedModules = groupedModules.value[folder.id] || []
    const affectedIds = affectedModules.map(m => m.id)
    if (selectedModuleId.value && affectedIds.includes(selectedModuleId.value)) {
      await selectModule(null)
    }
    
    await loadFolders()
    await loadModules()
    await loadCacheStats()
  } catch (error) {
    if (error !== 'cancel') {
      logger.error(`SNMP: 删除文件夹失败 - ${error}`)
      toast.error('删除文件夹失败')
    }
  }
}

/**
 * 移动模块到文件夹
 */
async function moveModule(module: MIBModuleVM, folderId: number | null) {
  try {
    await SNMPMIBAPI.moveMIBModuleToFolder(module.id, folderId)
    toast.success('已移动 MIB 模块')
    await loadModules()
  } catch (error) {
    logger.error(`SNMP: 移动模块失败 - ${error}`)
    toast.error('移动模块失败')
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
    loadFolders()
    loadModules()
    loadCacheStats()
  })

  // 监听删除事件
  unsubscribeMIBDeleted = SNMPEvents.onMIBDeleted((moduleID) => {
    if (selectedModuleId.value === moduleID) {
      selectModule(null)
    }
    loadFolders()
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
  loadFolders()
  loadModules()
  loadCacheStats()
  setupEventListeners()
})

onUnmounted(() => {
  cleanupEventListeners()
})

// 监听树本地搜索输入
watch(treeSearchQuery, (val) => {
  searchNodes(val)
})

// 监听导入方式切换
watch(importType, () => {
  importFilePaths.value = []
  importFolderPath.value = ''
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
        <div class="flex items-center gap-2">
          <button
            @click="createFolder"
            class="px-2 py-1 text-xs bg-bg-hover text-text-primary border border-border rounded hover:bg-border transition-colors cursor-pointer flex items-center gap-1 font-medium"
            title="新建文件夹"
          >
            新建
          </button>
          <button
            @click="openImportModal"
            class="px-2 py-1 text-xs bg-accent text-white rounded hover:bg-accent-hover transition-colors cursor-pointer font-medium"
          >
            导入
          </button>
          <button
            @click="openSearchModal('global')"
            class="px-2 py-1 text-xs bg-bg-hover text-text-primary border border-border rounded hover:bg-border transition-colors cursor-pointer flex items-center gap-1"
            title="全局搜索 MIB 节点"
          >
            <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
              <circle cx="11" cy="11" r="8" />
              <path d="M21 21l-4.35-4.35" />
            </svg>
            搜索
          </button>
        </div>
      </div>

      <!-- 模块列表 -->
      <div class="flex-1 overflow-y-auto scrollbar-custom p-2 space-y-2">
        <div v-if="modulesLoading || foldersLoading" class="p-4 text-center text-text-muted">
          加载中...
        </div>
        <div v-else-if="modules.length === 0 && folders.length === 0" class="p-4 text-center text-text-muted">
          暂无 MIB 模块和文件夹
        </div>
        <div v-else class="space-y-2">
          
          <!-- 文件夹列表 -->
          <div v-for="folder in folders" :key="folder.id" class="border border-border/40 rounded bg-bg-primary/20">
            <!-- 文件夹 Header -->
            <div 
              @click="expandedFolders[folder.id] = !expandedFolders[folder.id]"
              class="flex items-center justify-between px-3 py-2 cursor-pointer hover:bg-bg-hover rounded select-none group"
            >
              <div class="flex items-center gap-2 min-w-0">
                <!-- 展开/折叠箭头 -->
                <svg 
                  class="w-3.5 h-3.5 text-text-muted transition-transform duration-200 shrink-0"
                  :class="{ 'rotate-90': expandedFolders[folder.id] }"
                  fill="none" stroke="currentColor" stroke-width="2.5" viewBox="0 0 24 24"
                >
                  <path d="M9 5l7 7-7 7" />
                </svg>
                
                <!-- 文件夹图标 -->
                <svg class="w-4 h-4 text-accent shrink-0" fill="currentColor" viewBox="0 0 20 20">
                  <path d="M2 6a2 2 0 012-2h5l2 2h5a2 2 0 012 2v6a2 2 0 01-2 2H4a2 2 0 01-2-2V6z" />
                </svg>
                
                <!-- 文件夹名称 -->
                <span class="text-sm font-semibold text-text-primary truncate" :title="folder.name">
                  {{ folder.name }}
                </span>
                
                <!-- 模块数量 -->
                <span class="text-xs text-text-muted font-normal">({{ (groupedModules[folder.id] || []).length }})</span>
              </div>
              
              <!-- 文件夹操作按钮 -->
              <div class="hidden group-hover:flex items-center gap-1">
                <button
                  @click.stop="renameFolder(folder)"
                  class="p-0.5 text-text-muted hover:text-text-primary hover:bg-bg-hover rounded transition-colors cursor-pointer"
                  title="重命名文件夹"
                >
                  <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                    <path d="M16.862 4.487l1.687-1.688a1.875 1.875 0 112.652 2.652L6.832 19.82a4.5 4.5 0 01-1.897 1.13l-2.685.8.8-2.685a4.5 4.5 0 011.13-1.897L16.863 4.487zm0 0L19.5 7.125" />
                  </svg>
                </button>
                <button
                  @click.stop="deleteFolder(folder)"
                  class="p-0.5 text-text-muted hover:text-red-400 hover:bg-bg-hover rounded transition-colors cursor-pointer"
                  title="删除文件夹"
                >
                  <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                    <path d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                  </svg>
                </button>
              </div>
            </div>
            
            <!-- 文件夹下的 MIB 模块列表 -->
            <ul v-if="expandedFolders[folder.id]" class="pl-4 pr-1 pb-1 space-y-0.5 border-t border-border/20 pt-1">
              <li v-if="(groupedModules[folder.id] || []).length === 0" class="py-2 px-3 text-xs text-text-muted italic">
                没有模块
              </li>
              <li
                v-for="module in groupedModules[folder.id]"
                :key="module.id"
                @click="selectModule(module.id)"
                :class="[
                  'px-3 py-2 cursor-pointer transition-all rounded flex flex-col group/module relative',
                  selectedModuleId === module.id
                    ? 'bg-accent-bg border-l-2 border-accent text-accent'
                    : 'hover:bg-bg-hover text-text-primary',
                ]"
              >
                <div class="flex items-center justify-between min-w-0">
                  <span class="text-sm font-medium truncate flex-1" :title="module.name">{{ module.name }}</span>
                  <div class="flex items-center gap-1.5 shrink-0 ml-2">
                    <span :class="['text-[10px] px-1 py-0.5 rounded bg-bg-primary font-normal', getStatusClass(module.status)]">
                      {{ module.status }}
                    </span>
                    
                    <!-- 移动到/删除模块操作 -->
                    <div class="hidden group-hover/module:flex items-center gap-0.5">
                      <!-- 移动文件夹下拉菜单 -->
                      <el-dropdown trigger="click" @command="(cmd: any) => moveModule(module, cmd)">
                        <button
                          @click.stop
                          class="p-0.5 text-text-muted hover:text-text-primary rounded cursor-pointer"
                          title="移动到文件夹"
                        >
                          <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                            <path d="M9 13h6m-3-3v6m-9 1V4a2 2 0 012-2h6l2 2h6a2 2 0 012 2v8a2 2 0 01-2 2H5a2 2 0 01-2-2z" />
                          </svg>
                        </button>
                        <template #dropdown>
                          <el-dropdown-menu>
                            <el-dropdown-item :command="null">未分类</el-dropdown-item>
                            <el-dropdown-item 
                              v-for="f in folders.filter(x => x.id !== folder.id)" 
                              :key="f.id" 
                              :command="f.id"
                            >
                              {{ f.name }}
                            </el-dropdown-item>
                          </el-dropdown-menu>
                        </template>
                      </el-dropdown>
                      
                      <!-- 删除按钮 -->
                      <button
                        @click.stop="deleteModule(module)"
                        class="p-0.5 text-text-muted hover:text-red-400 rounded cursor-pointer"
                        title="删除模块"
                      >
                        <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                        </svg>
                      </button>
                    </div>
                  </div>
                </div>
                <div class="mt-0.5 flex items-center gap-1.5 text-[10px] text-text-muted">
                  <span>{{ module.nodeCount }} 节点</span>
                  <span>·</span>
                  <span>{{ formatTime(module.importedAt) }}</span>
                </div>
              </li>
            </ul>
          </div>
          
          <!-- 未分类部分 -->
          <div class="border border-border/40 rounded bg-bg-primary/20">
            <!-- 未分类 Header -->
            <div 
              @click="expandedFolders['uncategorized'] = !expandedFolders['uncategorized']"
              class="flex items-center justify-between px-3 py-2 cursor-pointer hover:bg-bg-hover rounded select-none group"
            >
              <div class="flex items-center gap-2 min-w-0">
                <svg 
                  class="w-3.5 h-3.5 text-text-muted transition-transform duration-200 shrink-0"
                  :class="{ 'rotate-90': expandedFolders['uncategorized'] }"
                  fill="none" stroke="currentColor" stroke-width="2.5" viewBox="0 0 24 24"
                >
                  <path d="M9 5l7 7-7 7" />
                </svg>
                
                <svg class="w-4 h-4 text-text-muted shrink-0" fill="currentColor" viewBox="0 0 20 20">
                  <path fill-rule="evenodd" d="M2 6a2 2 0 012-2h4l2 2h4a2 2 0 012 2v6a2 2 0 01-2 2H4a2 2 0 01-2-2V6zm10.3 5.7a1 1 0 00-1.4-1.4L9 12.2l-1.9-1.9a1 1 0 00-1.4 1.4l2.6 2.6a1 1 0 001.4 0l3.6-3.6z" clip-rule="evenodd" />
                </svg>
                
                <span class="text-sm font-semibold text-text-primary truncate">
                  未分类
                </span>
                
                <span class="text-xs text-text-muted font-normal">({{ (groupedModules['uncategorized'] || []).length }})</span>
              </div>
            </div>
            
            <!-- 未分类下的 MIB 模块列表 -->
            <ul v-if="expandedFolders['uncategorized']" class="pl-4 pr-1 pb-1 space-y-0.5 border-t border-border/20 pt-1">
              <li v-if="(groupedModules['uncategorized'] || []).length === 0" class="py-2 px-3 text-xs text-text-muted italic">
                没有模块
              </li>
              <li
                v-for="module in groupedModules['uncategorized']"
                :key="module.id"
                @click="selectModule(module.id)"
                :class="[
                  'px-3 py-2 cursor-pointer transition-all rounded flex flex-col group/module relative',
                  selectedModuleId === module.id
                    ? 'bg-accent-bg border-l-2 border-accent text-accent'
                    : 'hover:bg-bg-hover text-text-primary',
                ]"
              >
                <div class="flex items-center justify-between min-w-0">
                  <span class="text-sm font-medium truncate flex-1" :title="module.name">{{ module.name }}</span>
                  <div class="flex items-center gap-1.5 shrink-0 ml-2">
                    <span :class="['text-[10px] px-1 py-0.5 rounded bg-bg-primary font-normal', getStatusClass(module.status)]">
                      {{ module.status }}
                    </span>
                    
                    <div class="hidden group-hover/module:flex items-center gap-0.5">
                      <!-- 移动到文件夹下拉菜单 -->
                      <el-dropdown trigger="click" @command="(cmd: any) => moveModule(module, cmd)">
                        <button
                          @click.stop
                          class="p-0.5 text-text-muted hover:text-text-primary rounded cursor-pointer"
                          title="移动到文件夹"
                        >
                          <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                            <path d="M9 13h6m-3-3v6m-9 1V4a2 2 0 012-2h6l2 2h6a2 2 0 012 2v8a2 2 0 01-2 2H5a2 2 0 01-2-2z" />
                          </svg>
                        </button>
                        <template #dropdown>
                          <el-dropdown-menu>
                            <el-dropdown-item v-for="f in folders" :key="f.id" :command="f.id">
                              {{ f.name }}
                            </el-dropdown-item>
                          </el-dropdown-menu>
                        </template>
                      </el-dropdown>
                      
                      <!-- 删除按钮 -->
                      <button
                        @click.stop="deleteModule(module)"
                        class="p-0.5 text-text-muted hover:text-red-400 rounded cursor-pointer"
                        title="删除模块"
                      >
                        <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                        </svg>
                      </button>
                    </div>
                  </div>
                </div>
                <div class="mt-0.5 flex items-center gap-1.5 text-[10px] text-text-muted">
                  <span>{{ module.nodeCount }} 节点</span>
                  <span>·</span>
                  <span>{{ formatTime(module.importedAt) }}</span>
                </div>
              </li>
            </ul>
          </div>
          
        </div>
      </div>

      <!-- 缓存统计 -->
      <div class="px-4 py-2 border-t border-border text-xs text-text-muted">
        <div class="flex items-center justify-between">
          <span>缓存: {{ cacheStats['oidCacheLen'] ?? 0 }} OID / {{ cacheStats['nameCacheLen'] ?? 0 }} 名称</span>
          <div class="flex gap-2">
            <button @click="clearCache" class="hover:text-text-primary transition-colors cursor-pointer">
              清除
            </button>
            <button @click="rebuildCache" class="hover:text-text-primary transition-colors cursor-pointer">
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
            class="px-2 py-1 text-xs text-text-muted hover:text-text-primary transition-colors cursor-pointer"
          >
            全部展开
          </button>
          <button
            @click="collapseAll"
            class="px-2 py-1 text-xs text-text-muted hover:text-text-primary transition-colors cursor-pointer"
          >
            全部折叠
          </button>
        </div>
      </div>

      <!-- 搜索栏 -->
      <div class="px-4 py-2 border-b border-border flex items-center gap-2">
        <div class="relative flex-1">
          <input
            v-model="treeSearchQuery"
            type="text"
            placeholder="搜索名称、OID 或描述..."
            @keyup.enter="handleTreeSearchEnter"
            class="w-full px-3 py-2 pl-9 pr-8 text-sm bg-bg-primary border border-border rounded-lg focus:outline-none focus:border-accent transition-colors"
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
            v-if="treeSearchCount > 0"
            class="absolute right-3 top-1/2 -translate-y-1/2 text-xs text-text-muted bg-bg-secondary px-1.5 py-0.5 rounded border border-border"
          >
            {{ treeSearchCount }} 结果
          </span>
        </div>
        <button
          @click="openSearchModal('module')"
          :disabled="!selectedModuleId"
          class="px-3 py-2 text-sm bg-bg-hover text-text-primary border border-border rounded-lg hover:bg-border transition-colors disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer flex items-center gap-1 shrink-0"
          title="在当前模块内深度搜索"
        >
          <svg class="w-4 h-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
          </svg>
          高级搜索
        </button>
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
              :data-node-id="item.node.id"
              @click="selectMIBNode(item.node.id)"
              :class="[
                'px-4 py-1.5 cursor-pointer transition-colors flex items-center gap-2',
                isNodeSelected(item.node.id)
                  ? 'bg-accent-bg text-accent font-semibold'
                  : highlightedNodeIds.has(item.node.id)
                    ? 'bg-yellow-500/10 text-yellow-600 border-r-2 border-yellow-500 font-medium'
                    : 'hover:bg-bg-hover text-text-primary',
              ]"
              :style="{ paddingLeft: `${item.level * 16 + 16}px` }"
            >
              <!-- 展开/折叠按钮 -->
              <button
                v-if="item.node.hasChildren"
                @click.stop="toggleNode(item.node.id)"
                class="w-4 h-4 flex items-center justify-center text-text-muted hover:text-text-primary cursor-pointer"
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
                <dd class="text-sm text-text-primary font-mono select-all">{{ selectedNodeDetail.name }}</dd>
              </div>
              <div>
                <dt class="text-xs text-text-muted">OID</dt>
                <dd class="text-sm text-text-primary font-mono select-all">{{ selectedNodeDetail.oid }}</dd>
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
            <p class="text-sm text-text-primary whitespace-pre-wrap leading-relaxed max-h-[300px] overflow-y-auto scrollbar-custom border border-border p-2 rounded bg-bg-primary">
              {{ selectedNodeDetail.description }}
            </p>
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
      class="fixed inset-0 bg-black/50 flex items-center justify-center z-50 animate-fade-in"
      @click.self="showImportModal = false"
    >
      <div class="bg-bg-secondary rounded-lg border border-border w-[480px] max-h-[80vh] overflow-hidden shadow-2xl flex flex-col">
        <!-- 头部 -->
        <div class="px-4 py-3 border-b border-border flex items-center justify-between">
          <h3 class="text-sm font-semibold text-text-primary">导入 MIB</h3>
          <button
            @click="showImportModal = false"
            class="text-text-muted hover:text-text-primary transition-colors cursor-pointer"
          >
            <svg class="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M18 6L6 18M6 6l12 12" />
            </svg>
          </button>
        </div>

        <div class="p-4 space-y-4 flex-1 overflow-y-auto">
          <!-- 导入类型选择 -->
          <div>
            <label class="block text-sm text-text-muted mb-2">导入类型</label>
            <el-radio-group v-model="importType" size="small" class="w-full">
              <el-radio-button value="files">导入文件</el-radio-button>
              <el-radio-button value="folder">导入文件夹</el-radio-button>
            </el-radio-group>
          </div>

          <!-- 导入目标文件夹 -->
          <div>
            <label class="block text-sm text-text-muted mb-2">导入至文件夹</label>
            <el-select v-model="targetFolderId" placeholder="选择目标文件夹（可选，默认为未分类）" clearable class="w-full">
              <el-option :value="null" label="未分类" />
              <el-option
                v-for="folder in folders"
                :key="folder.id"
                :label="folder.name"
                :value="folder.id"
              />
            </el-select>
          </div>

          <!-- 文件选择 -->
          <div v-if="importType === 'files'">
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
                class="px-3 py-2 text-sm bg-bg-hover text-text-primary rounded-lg hover:bg-border transition-colors cursor-pointer font-medium"
              >
                浏览
              </button>
            </div>
          </div>

          <!-- 文件夹选择 -->
          <div v-else>
            <label class="block text-sm text-text-muted mb-2">选择 MIB 文件夹</label>
            <div class="flex gap-2">
              <input
                :value="importFolderPath"
                type="text"
                readonly
                placeholder="点击选择文件夹..."
                class="flex-1 px-3 py-2 text-sm bg-bg-primary border border-border rounded-lg focus:outline-none"
              />
              <button
                @click="selectMIBFolder"
                class="px-3 py-2 text-sm bg-bg-hover text-text-primary rounded-lg hover:bg-border transition-colors cursor-pointer font-medium"
              >
                浏览
              </button>
            </div>
          </div>

          <!-- 导入进度 -->
          <div v-if="importProgress" class="space-y-2 pt-2 border-t border-border">
            <div class="flex items-center justify-between text-sm">
              <span class="text-text-muted truncate max-w-[70%]">{{ importProgress.fileName }}</span>
              <span class="text-text-primary font-semibold">{{ ((importProgress?.progress) ?? 0).toFixed(0) }}%</span>
            </div>
            <div class="h-2 bg-bg-primary rounded-full overflow-hidden">
              <div
                class="h-full bg-accent transition-all duration-300"
                :style="{ width: `${importProgress?.progress ?? 0}%` }"
              ></div>
            </div>
            <div class="text-xs text-text-muted flex justify-between">
              <span>状态: {{ importProgress.phase }}</span>
              <span>已导入: {{ importProgress.nodesDone }} / {{ importProgress.nodesTotal }} 节点</span>
            </div>
            <div v-if="importProgress.error" class="text-xs text-error mt-1 whitespace-pre-wrap">
              错误: {{ importProgress.error }}
            </div>
          </div>
        </div>

        <!-- 底部按钮 -->
        <div class="px-4 py-3 border-t border-border flex justify-end gap-2 bg-bg-secondary">
          <button
            @click="showImportModal = false"
            class="px-4 py-2 text-sm text-text-muted hover:text-text-primary transition-colors cursor-pointer"
          >
            取消
          </button>
          <button
            @click="executeImport"
            :disabled="importing || (importType === 'files' ? importFilePaths.length === 0 : !importFolderPath)"
            class="px-4 py-2 text-sm bg-accent text-white rounded-lg hover:bg-accent-hover disabled:opacity-50 disabled:cursor-not-allowed transition-colors cursor-pointer font-medium"
          >
            {{ importing ? '导入中...' : '导入' }}
          </button>
        </div>
      </div>
    </div>

    <!-- MIB 搜索弹窗 -->
    <el-dialog
      v-model="showSearchModal"
      :title="searchScope === 'global' ? '全局 MIB 节点搜索' : `模块 [${selectedModule?.name}] 节点搜索`"
      width="950px"
      :close-on-click-modal="true"
      align-center
    >
      <div class="flex flex-col gap-4 h-[550px]">
        <!-- 搜索输入和范围选择 -->
        <div class="flex items-center gap-4">
          <el-input
            v-model="modalSearchQuery"
            placeholder="输入名称、OID 或描述进行搜索..."
            clearable
            @keyup.enter="executeModalSearch"
            class="flex-1"
          >
            <template #prefix>
              <el-icon><Search /></el-icon>
            </template>
          </el-input>
          
          <el-radio-group v-model="searchScope" :disabled="!selectedModuleId">
            <el-radio-button value="global">全局搜索</el-radio-button>
            <el-radio-button value="module">当前模块</el-radio-button>
          </el-radio-group>
          
          <el-button type="primary" :loading="modalSearchLoading" @click="executeModalSearch">
            搜索
          </el-button>
        </div>

        <!-- 结果展示区域：左边表格，右边详情 -->
        <div class="flex-1 flex gap-4 min-h-0">
          <!-- 左侧表格 -->
          <div class="flex-1 border border-border rounded-lg overflow-hidden flex flex-col">
            <el-table
              v-loading="modalSearchLoading"
              :data="modalSearchResults"
              height="100%"
              style="width: 100%"
              highlight-current-row
              @current-change="handleModalSelectNode"
              @row-dblclick="handleModalJumpToNode"
            >
              <el-table-column prop="name" label="名称" min-width="180" show-overflow-tooltip />
              <el-table-column prop="oid" label="OID" min-width="180" show-overflow-tooltip />
              <el-table-column prop="moduleName" label="所属模块" min-width="140" show-overflow-tooltip />
            </el-table>
          </div>

          <!-- 右侧详情 -->
          <div class="w-[340px] border border-border rounded-lg p-4 overflow-y-auto bg-bg-secondary flex flex-col gap-4">
            <div v-if="!modalSelectedNode" class="text-center text-text-muted my-auto flex flex-col items-center gap-2">
              <svg class="w-8 h-8 text-text-muted" fill="none" stroke="currentColor" stroke-width="1.5" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" d="M11.25 11.25l.041-.02a.75.75 0 111.063.852l-.708 2.836a.75.75 0 001.063.852l.041-.021M21 12a9 9 0 11-18 0 9 9 0 0118 0zm-9-3.75h.008v.008H12V8.25z" />
              </svg>
              <span>单击选择节点查看详情<br>双击跳转到树节点</span>
            </div>
            <div v-else class="space-y-4">
              <div>
                <h4 class="text-xs font-semibold text-text-muted uppercase mb-2 border-b border-border pb-1">基本信息</h4>
                <dl class="space-y-2">
                  <div>
                    <dt class="text-xs text-text-muted font-medium">名称</dt>
                    <dd class="text-sm font-mono text-text-primary select-all break-all">{{ modalSelectedNode.name }}</dd>
                  </div>
                  <div>
                    <dt class="text-xs text-text-muted font-medium">OID</dt>
                    <dd class="text-sm font-mono text-text-primary select-all break-all">{{ modalSelectedNode.oid }}</dd>
                  </div>
                  <div>
                    <dt class="text-xs text-text-muted font-medium">所属模块</dt>
                    <dd class="text-sm text-text-primary">{{ modalSelectedNode.moduleName || '-' }}</dd>
                  </div>
                  <div>
                    <dt class="text-xs text-text-muted font-medium">类型 / 语法</dt>
                    <dd class="text-sm text-text-primary font-mono">{{ modalSelectedNode.type || '-' }}</dd>
                  </div>
                  <div>
                    <dt class="text-xs text-text-muted font-medium">访问权限</dt>
                    <dd class="mt-0.5">
                      <span :class="['px-2 py-0.5 text-xs rounded', getAccessClass(modalSelectedNode.access)]">
                        {{ modalSelectedNode.access || '-' }}
                      </span>
                    </dd>
                  </div>
                  <div>
                    <dt class="text-xs text-text-muted font-medium">状态</dt>
                    <dd class="text-sm text-text-primary">{{ modalSelectedNode.status || '-' }}</dd>
                  </div>
                </dl>
              </div>

              <div v-if="modalSelectedNode.description">
                <h4 class="text-xs font-semibold text-text-muted uppercase mb-2 border-b border-border pb-1">描述</h4>
                <p class="text-sm text-text-primary whitespace-pre-wrap leading-relaxed max-h-[220px] overflow-y-auto scrollbar-custom border border-border p-2 rounded bg-bg-primary">
                  {{ modalSelectedNode.description }}
                </p>
              </div>
            </div>
          </div>
        </div>
      </div>
    </el-dialog>
  </div>
</template>