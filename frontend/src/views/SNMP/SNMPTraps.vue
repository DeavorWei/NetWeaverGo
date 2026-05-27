<script setup lang="ts">
/**
 * SNMP Trap 告警中心页面
 *
 * 功能：
 * - 监听器状态监控和启停控制
 * - Trap 记录列表展示（支持分页、筛选、排序）
 * - Trap 详情查看
 * - 过滤规则管理
 * - 服务器配置管理
 * - 实时 Trap 推送和高亮显示
 */
import { ref, computed, onMounted } from 'vue'
import { ElMessageBox } from 'element-plus'
import { SNMPTrapAPI } from '@/services/snmpApi'
import { useSNMPTrapStream } from '@/composables/useSNMPTrapStream'
import { useToast } from '@/utils/useToast'
import { getLogger } from '@/utils/logger'
import type {
  TrapRecordVM,
  TrapFilterVM,
  ServerConfigVM,
  FilterRuleVM,
  CreateFilterRuleRequest,
  UpdateFilterRuleRequest,
  V3UserVM,
  AddV3UserRequest,
} from '@/bindings/github.com/NetWeaverGo/core/internal/ui/models'

const logger = getLogger()
const toast = useToast()

// ==================== 组合式函数 ====================

const {
  listenerStatus,
  trapStats,
  unacknowledgedCount,
  refreshStatus,
  refreshStats,
  acknowledgeTrap,
  batchAcknowledge,
  isNewTrap,
} = useSNMPTrapStream()

// ==================== 状态 ====================

/** Trap 记录列表 */
const trapRecords = ref<TrapRecordVM[]>([])
const recordsLoading = ref(false)
const totalRecords = ref(0)
const currentPage = ref(1)
const pageSize = ref(20)
const totalPages = ref(0)

/** 选中的 Trap 记录 */
const selectedTrap = ref<TrapRecordVM | null>(null)
const selectedTrapIds = ref<Set<number>>(new Set())

/** 过滤条件 */
const filter = ref<TrapFilterVM>({
  sourceIP: '',
  trapOID: '',
  severity: '',
  startTime: '',
  endTime: '',
  acknowledged: null,
  searchQuery: '',
})

/** 服务器配置 */
const serverConfigs = ref<ServerConfigVM[]>([])
const activeConfig = ref<ServerConfigVM | null>(null)

/** 过滤规则 */
const filterRules = ref<FilterRuleVM[]>([])

/** 面板显示状态 */
const showFilterPanel = ref(true)
const showDetailPanel = ref(true)
const showConfigModal = ref(false)
const showRuleModal = ref(false)
const showV3UserModal = ref(false)

/** v3 用户管理 */
const v3Users = ref<V3UserVM[]>([])
const editingV3User = ref<V3UserVM | null>(null)
const v3UserForm = ref<AddV3UserRequest>({
  username: '',
  authProtocol: 'MD5',
  authKey: '',
  privProtocol: 'AES',
  privKey: '',
  securityLevel: 'authPriv',
})

/** 编辑状态 */
const editingConfig = ref<ServerConfigVM | null>(null)
const editingRule = ref<FilterRuleVM | null>(null)

/** 面板宽度 */
const leftPanelWidth = ref(280)
const rightPanelWidth = ref(360)

/** 操作加载状态 */
const listenerOperating = ref(false)
const acknowledging = ref(false)

// ==================== 计算属性 ====================

/** 监听器运行状态 */
const isListenerRunning = computed(() => listenerStatus.value?.isRunning ?? false)

/** 严重级别选项 */
const severityOptions = [
  { value: '', label: '全部' },
  { value: 'critical', label: '严重' },
  { value: 'major', label: '重要' },
  { value: 'minor', label: '次要' },
  { value: 'info', label: '信息' },
]

/** 确认状态选项 */
const acknowledgedOptions = [
  { value: undefined, label: '全部' },
  { value: false, label: '未确认' },
  { value: true, label: '已确认' },
]

/** 是否有选中记录 */
const hasSelection = computed(() => selectedTrapIds.value.size > 0)

/** 全选状态 */
const isAllSelected = computed(() => {
  return trapRecords.value.length > 0 &&
    trapRecords.value.every(r => selectedTrapIds.value.has(r.id))
})

// ==================== 方法 ====================

/**
 * 加载 Trap 记录列表
 */
async function loadTrapRecords() {
  recordsLoading.value = true
  try {
    const result = await SNMPTrapAPI.getTrapRecords(
      filter.value,
      currentPage.value,
      pageSize.value
    )
    if (result) {
      trapRecords.value = result.data
      totalRecords.value = result.total
      totalPages.value = result.totalPages
      logger.debug(`SNMP-Trap: Trap 记录已加载 - ${result.data.length} 条`)
    }
  } catch (error) {
    logger.error(`SNMP-Trap: 加载 Trap 记录失败 - ${error}`)
    toast.error('加载 Trap 记录失败')
  } finally {
    recordsLoading.value = false
  }
}

/**
 * 选择 Trap 记录
 */
function selectTrap(trap: TrapRecordVM) {
  selectedTrap.value = trap
}

/**
 * 切换记录选择
 */
function toggleSelection(id: number) {
  if (selectedTrapIds.value.has(id)) {
    selectedTrapIds.value.delete(id)
  } else {
    selectedTrapIds.value.add(id)
  }
}

/**
 * 全选/取消全选
 */
function toggleSelectAll() {
  if (isAllSelected.value) {
    selectedTrapIds.value.clear()
  } else {
    for (const record of trapRecords.value) {
      selectedTrapIds.value.add(record.id)
    }
  }
}

/**
 * 应用过滤条件
 */
function applyFilter() {
  currentPage.value = 1
  loadTrapRecords()
}

/**
 * 重置过滤条件
 */
function resetFilter() {
  filter.value = {
    sourceIP: '',
    trapOID: '',
    severity: '',
    startTime: '',
    endTime: '',
    acknowledged: null,
    searchQuery: '',
  }
  currentPage.value = 1
  loadTrapRecords()
}

/**
 * 切换页面
 */
function changePage(page: number) {
  currentPage.value = page
  loadTrapRecords()
}

/**
 * 切换每页条数
 */
function changePageSize(size: number) {
  pageSize.value = size
  currentPage.value = 1
  loadTrapRecords()
}

/**
 * 启动监听器
 */
async function startListener() {
  if (!activeConfig.value) {
    toast.warning('请先配置服务器参数')
    return
  }

  listenerOperating.value = true
  try {
    await SNMPTrapAPI.startListener(activeConfig.value)
    toast.success('Trap 监听器已启动')
    await refreshStatus()
  } catch (error) {
    logger.error('SNMP-Trap', '启动监听器失败', error)
    toast.error('启动监听器失败')
  } finally {
    listenerOperating.value = false
  }
}

/**
 * 停止监听器
 */
async function stopListener() {
  listenerOperating.value = true
  try {
    await SNMPTrapAPI.stopListener()
    toast.success('Trap 监听器已停止')
    await refreshStatus()
  } catch (error) {
    logger.error('SNMP-Trap', '停止监听器失败', error)
    toast.error('停止监听器失败')
  } finally {
    listenerOperating.value = false
  }
}

/**
 * 确认选中的记录
 */
async function acknowledgeSelected() {
  if (selectedTrapIds.value.size === 0) return

  acknowledging.value = true
  try {
    const ids = Array.from(selectedTrapIds.value)
    await batchAcknowledge(ids)
    toast.success(`已确认 ${ids.length} 条记录`)
    selectedTrapIds.value.clear()
    await loadTrapRecords()
    await refreshStats()
  } catch (error) {
    logger.error('SNMP-Trap', '批量确认失败', error)
    toast.error('批量确认失败')
  } finally {
    acknowledging.value = false
  }
}

/**
 * 确认单个记录
 */
async function acknowledgeSingle(trap: TrapRecordVM) {
  try {
    await acknowledgeTrap(trap.id)
    toast.success('已确认')
    trap.acknowledged = true
    await refreshStats()
  } catch (error) {
    logger.error('SNMP-Trap', '确认失败', error)
    toast.error('确认失败')
  }
}

/**
 * 删除单个记录
 */
async function deleteTrapRecord(trap: TrapRecordVM) {
  try {
    await ElMessageBox.confirm('确定要删除此 Trap 记录吗？', '删除确认', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning',
    })
  } catch {
    return // 用户取消
  }

  try {
    await SNMPTrapAPI.deleteTrapRecord(trap.id)
    toast.success('已删除')
    await loadTrapRecords()
    await refreshStats()
  } catch (error) {
    logger.error('SNMP-Trap', '删除失败', error)
    toast.error('删除失败')
  }
}

/**
 * 清理过期记录
 */
async function clearOldRecords() {
  try {
    const { value: days } = await ElMessageBox.prompt(
      '请输入要保留的天数（之前的记录将被删除）：',
      '清理过期记录',
      {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        inputPattern: /^\d+$/,
        inputErrorMessage: '请输入有效的数字',
        inputPlaceholder: '30',
        inputValue: '30',
      }
    )

    const daysNum = parseInt(days)
    if (isNaN(daysNum)) return

    const before = new Date()
    before.setDate(before.getDate() - daysNum)
    const beforeStr = before.toISOString().split('T')[0] || ''

    const count = await SNMPTrapAPI.clearTrapRecords(beforeStr)
    toast.success(`已清理 ${count} 条记录`)
    await loadTrapRecords()
    await refreshStats()
  } catch {
    // 用户取消或输入无效
  }
}

/**
 * 加载服务器配置
 */
async function loadServerConfigs() {
  try {
    serverConfigs.value = await SNMPTrapAPI.getServerConfigs()
    const firstConfig = serverConfigs.value[0]
    if (firstConfig) {
      activeConfig.value = firstConfig
    }
  } catch (error) {
    logger.error(`SNMP-Trap: 加载服务器配置失败 - ${error}`)
  }
}

/**
 * 加载过滤规则
 */
async function loadFilterRules() {
  try {
    filterRules.value = await SNMPTrapAPI.getFilterRules()
  } catch (error) {
    logger.error('SNMP-Trap', '加载过滤规则失败', error)
  }
}

// ==================== v3 用户管理 ====================

/**
 * 加载 v3 用户列表
 */
async function loadV3Users() {
  try {
    v3Users.value = await SNMPTrapAPI.listV3Users()
  } catch (error) {
    logger.error('SNMP-Trap', '加载 v3 用户失败', error)
  }
}

/**
 * 打开 v3 用户添加对话框
 */
function openV3UserModal(user: V3UserVM | null = null) {
  if (user) {
    editingV3User.value = user
    v3UserForm.value = {
      username: user.username,
      authProtocol: user.authProtocol,
      authKey: '',
      privProtocol: user.privProtocol || '',
      privKey: '',
      securityLevel: user.securityLevel as 'noAuthNoPriv' | 'authNoPriv' | 'authPriv',
    }
  } else {
    editingV3User.value = null
    v3UserForm.value = {
      username: '',
      authProtocol: 'MD5',
      authKey: '',
      privProtocol: 'AES',
      privKey: '',
      securityLevel: 'authPriv',
    }
  }
  showV3UserModal.value = true
}

/**
 * 保存 v3 用户
 */
async function saveV3User() {
  try {
    if (editingV3User.value) {
      // 先删除再添加（更新）
      await SNMPTrapAPI.removeV3User(editingV3User.value.username)
    }
    await SNMPTrapAPI.addV3User(v3UserForm.value)
    toast.success(editingV3User.value ? 'v3 用户已更新' : 'v3 用户已添加')
    showV3UserModal.value = false
    await loadV3Users()
  } catch (error) {
    logger.error('SNMP-Trap', '保存 v3 用户失败', error)
    toast.error('保存 v3 用户失败')
  }
}

/**
 * 删除 v3 用户
 */
async function deleteV3User(username: string) {
  try {
    await ElMessageBox.confirm(`确定要删除 v3 用户「${username}」吗？`, '删除确认', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning',
    })
  } catch {
    return // 用户取消
  }

  try {
    await SNMPTrapAPI.removeV3User(username)
    toast.success('v3 用户已删除')
    await loadV3Users()
  } catch (error) {
    logger.error('SNMP-Trap', '删除 v3 用户失败', error)
    toast.error('删除 v3 用户失败')
  }
}

/**
 * 打开配置编辑对话框
 */
function openConfigModal(config: ServerConfigVM | null = null) {
  if (config) {
    editingConfig.value = { ...config }
  } else {
    editingConfig.value = {
      id: 0,
      trapEnabled: true,
      trapPort: 162,
      trapCommunity: 'public',
      maxStorageDays: 30,
    }
  }
  showConfigModal.value = true
}

/**
 * 保存服务器配置
 */
async function saveConfig() {
  if (!editingConfig.value) return

  try {
    // id > 0 表示更新现有配置，id === 0 表示创建新配置
    if (editingConfig.value.id > 0) {
      await SNMPTrapAPI.updateServerConfig(editingConfig.value.id, {
        trapEnabled: editingConfig.value.trapEnabled,
        trapPort: editingConfig.value.trapPort,
        trapCommunity: editingConfig.value.trapCommunity,
        maxStorageDays: editingConfig.value.maxStorageDays,
      })
      toast.success('配置已更新')
    } else {
      // id === 0 时创建新配置
      await SNMPTrapAPI.createServerConfig({
        trapEnabled: editingConfig.value.trapEnabled,
        trapPort: editingConfig.value.trapPort,
        trapCommunity: editingConfig.value.trapCommunity,
        maxStorageDays: editingConfig.value.maxStorageDays,
      })
      toast.success('配置已创建')
    }
    showConfigModal.value = false
    await loadServerConfigs()
  } catch (error) {
    logger.error('SNMP-Trap', '保存配置失败', error)
    toast.error('保存配置失败')
  }
}

/**
 * 打开规则编辑对话框
 */
function openRuleModal(rule: FilterRuleVM | null = null) {
  editingRule.value = rule ? { ...rule } : null
  showRuleModal.value = true
}

/**
 * 保存过滤规则
 */
async function saveRule() {
  if (!editingRule.value) return

  try {
    if (editingRule.value.id) {
      const req: UpdateFilterRuleRequest = {
        name: editingRule.value.name,
        enabled: editingRule.value.enabled,
        priority: editingRule.value.priority,
        action: editingRule.value.action,
        sourceIPPattern: editingRule.value.sourceIPPattern,
        oidPattern: editingRule.value.oidPattern,
        communityPattern: editingRule.value.communityPattern,
        overrideSeverity: editingRule.value.overrideSeverity,
        description: editingRule.value.description,
      }
      await SNMPTrapAPI.updateFilterRule(editingRule.value.id, req)
      toast.success('规则已更新')
    } else {
      const req: CreateFilterRuleRequest = {
        name: editingRule.value.name,
        enabled: editingRule.value.enabled,
        priority: editingRule.value.priority,
        action: editingRule.value.action,
        sourceIPPattern: editingRule.value.sourceIPPattern,
        oidPattern: editingRule.value.oidPattern,
        communityPattern: editingRule.value.communityPattern,
        overrideSeverity: editingRule.value.overrideSeverity,
        description: editingRule.value.description,
      }
      await SNMPTrapAPI.createFilterRule(req)
      toast.success('规则已创建')
    }
    showRuleModal.value = false
    await loadFilterRules()
  } catch (error) {
    logger.error('SNMP-Trap', '保存规则失败', error)
    toast.error('保存规则失败')
  }
}

/**
 * 删除过滤规则
 */
async function deleteRule(rule: FilterRuleVM) {
  try {
    await ElMessageBox.confirm(`确定要删除规则 "${rule.name}" 吗？`, '删除确认', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning',
    })
  } catch {
    return // 用户取消
  }

  try {
    await SNMPTrapAPI.deleteFilterRule(rule.id)
    toast.success('规则已删除')
    await loadFilterRules()
  } catch (error) {
    logger.error('SNMP-Trap', '删除规则失败', error)
    toast.error('删除规则失败')
  }
}

/**
 * 切换规则启用状态
 */
async function toggleRuleEnabled(rule: FilterRuleVM) {
  try {
    await SNMPTrapAPI.updateFilterRule(rule.id, {
      name: rule.name,
      enabled: !rule.enabled,
      priority: rule.priority,
      action: rule.action,
      sourceIPPattern: rule.sourceIPPattern,
      oidPattern: rule.oidPattern,
      communityPattern: rule.communityPattern,
      overrideSeverity: rule.overrideSeverity,
      description: rule.description,
    })
    rule.enabled = !rule.enabled
    toast.success(rule.enabled ? '规则已启用' : '规则已禁用')
  } catch (error) {
    logger.error('SNMP-Trap', '切换规则状态失败', error)
    toast.error('操作失败')
  }
}

/**
 * 获取严重级别样式类
 */
function getSeverityClass(severity: string): string {
  switch (severity.toLowerCase()) {
    case 'critical':
      return 'bg-red-500/20 text-red-400 border-red-500/30'
    case 'major':
      return 'bg-orange-500/20 text-orange-400 border-orange-500/30'
    case 'minor':
      return 'bg-yellow-500/20 text-yellow-400 border-yellow-500/30'
    case 'info':
      return 'bg-blue-500/20 text-blue-400 border-blue-500/30'
    default:
      return 'bg-gray-500/20 text-gray-400 border-gray-500/30'
  }
}

/**
 * 获取严重级别文本
 */
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

/**
 * 格式化时间
 */
function formatTime(timeStr: string): string {
  if (!timeStr) return '-'
  return timeStr
}

/**
 * 解析 Variables JSON
 */
function parseVariables(variables: string): TrapVarBind[] {
  if (!variables) return []
  try {
    return JSON.parse(variables)
  } catch {
    return []
  }
}

interface TrapVarBind {
  oid: string
  oidName?: string
  type?: string
  value: unknown
}

// ==================== 生命周期 ====================

onMounted(async () => {
  await Promise.all([
    loadTrapRecords(),
    loadServerConfigs(),
    loadFilterRules(),
    loadV3Users(),
    refreshStatus(),
    refreshStats(),
  ])
})

</script>

<template>
  <div class="flex flex-col h-full w-full overflow-hidden">
    <!-- 顶部工具栏 -->
    <header class="flex items-center justify-between px-4 py-3 bg-bg-secondary border-b border-border/50 flex-shrink-0">
      <!-- 左侧：监听器状态 -->
      <div class="flex items-center gap-4">
        <div class="flex items-center gap-2">
          <span
            :class="[
              'w-2.5 h-2.5 rounded-full',
              isListenerRunning ? 'bg-green-500 animate-pulse' : 'bg-gray-500'
            ]"
          ></span>
          <span class="text-sm text-text-secondary">
            {{ isListenerRunning ? '监听中' : '已停止' }}
          </span>
        </div>

        <div v-if="listenerStatus" class="text-xs text-text-muted">
          {{ listenerStatus.listenAddr }} | 接收: {{ listenerStatus.totalTraps }} | 过滤: {{ listenerStatus.filteredOut }}
        </div>

        <!-- 启停按钮 -->
        <button
          v-if="!isListenerRunning"
          @click="startListener"
          :disabled="listenerOperating"
          class="px-3 py-1.5 text-sm bg-green-600 hover:bg-green-700 text-white rounded-md transition-colors disabled:opacity-50"
        >
          启动监听
        </button>
        <button
          v-else
          @click="stopListener"
          :disabled="listenerOperating"
          class="px-3 py-1.5 text-sm bg-red-600 hover:bg-red-700 text-white rounded-md transition-colors disabled:opacity-50"
        >
          停止监听
        </button>
      </div>

      <!-- 右侧：操作按钮 -->
      <div class="flex items-center gap-2">
        <!-- 统计信息 -->
        <div class="flex items-center gap-3 px-3 py-1.5 bg-bg-tertiary rounded-md text-xs">
          <span class="text-text-muted">未确认: <span class="text-yellow-400 font-medium">{{ unacknowledgedCount }}</span></span>
          <span class="text-text-muted">总计: <span class="text-text-primary font-medium">{{ trapStats?.totalCount ?? 0 }}</span></span>
        </div>

        <button
          @click="loadTrapRecords"
          class="p-2 text-text-secondary hover:text-text-primary hover:bg-bg-hover rounded-md transition-colors"
          title="刷新"
        >
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m0 0H9" />
          </svg>
        </button>

        <button
          v-if="hasSelection"
          @click="acknowledgeSelected"
          :disabled="acknowledging"
          class="px-3 py-1.5 text-sm bg-accent hover:bg-accent-dark text-white rounded-md transition-colors disabled:opacity-50"
        >
          确认选中 ({{ selectedTrapIds.size }})
        </button>

        <button
          @click="clearOldRecords"
          class="px-3 py-1.5 text-sm bg-bg-tertiary hover:bg-bg-hover text-text-secondary rounded-md transition-colors"
        >
          清理记录
        </button>

        <button
          @click="openConfigModal(activeConfig)"
          class="p-2 text-text-secondary hover:text-text-primary hover:bg-bg-hover rounded-md transition-colors"
          title="服务器配置"
        >
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
          </svg>
        </button>
      </div>
    </header>

    <!-- 主内容区域 -->
    <div class="flex flex-1 min-h-0 overflow-hidden">
      <!-- 左侧面板：过滤规则 -->
      <aside
        v-if="showFilterPanel"
        :style="{ width: `${leftPanelWidth}px` }"
        class="flex flex-col bg-bg-secondary border-r border-border/50 flex-shrink-0"
      >
        <!-- 过滤条件 -->
        <div class="p-3 border-b border-border/30">
          <h3 class="text-sm font-medium text-text-primary mb-3">过滤条件</h3>

          <div class="space-y-2">
            <input
              v-model="filter.searchQuery"
              type="text"
              placeholder="搜索 OID / IP / 名称..."
              class="w-full px-3 py-2 text-sm bg-bg-tertiary border border-border/50 rounded-md text-text-primary placeholder-text-muted focus:outline-none focus:border-accent"
              @keyup.enter="applyFilter"
            />

            <select
              v-model="filter.severity"
              class="w-full px-3 py-2 text-sm bg-bg-tertiary border border-border/50 rounded-md text-text-primary focus:outline-none focus:border-accent"
            >
              <option v-for="opt in severityOptions" :key="opt.value" :value="opt.value">
                {{ opt.label }}
              </option>
            </select>

            <select
              v-model="filter.acknowledged"
              class="w-full px-3 py-2 text-sm bg-bg-tertiary border border-border/50 rounded-md text-text-primary focus:outline-none focus:border-accent"
            >
              <option v-for="opt in acknowledgedOptions" :key="String(opt.value)" :value="opt.value">
                {{ opt.label }}
              </option>
            </select>

            <div class="flex gap-2">
              <button
                @click="applyFilter"
                class="flex-1 px-3 py-1.5 text-sm bg-accent hover:bg-accent-dark text-white rounded-md transition-colors"
              >
                应用
              </button>
              <button
                @click="resetFilter"
                class="px-3 py-1.5 text-sm bg-bg-tertiary hover:bg-bg-hover text-text-secondary rounded-md transition-colors"
              >
                重置
              </button>
            </div>
          </div>
        </div>

        <!-- 过滤规则列表 -->
        <div class="flex-1 overflow-y-auto p-3">
          <div class="flex items-center justify-between mb-3">
            <h3 class="text-sm font-medium text-text-primary">过滤规则</h3>
            <button
              @click="openRuleModal(null)"
              class="p-1 text-text-muted hover:text-accent transition-colors"
              title="添加规则"
            >
              <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
              </svg>
            </button>
          </div>

          <div class="space-y-2">
            <div
              v-for="rule in filterRules"
              :key="rule.id"
              class="p-2 bg-bg-tertiary rounded-md border border-border/30"
            >
              <div class="flex items-center justify-between">
                <span class="text-sm text-text-primary truncate">{{ rule.name }}</span>
                <div class="flex items-center gap-1">
                  <button
                    @click="toggleRuleEnabled(rule)"
                    :class="[
                      'w-8 h-4 rounded-full transition-colors relative',
                      rule.enabled ? 'bg-green-500' : 'bg-gray-600'
                    ]"
                  >
                    <span
                      :class="[
                        'absolute top-0.5 w-3 h-3 rounded-full bg-white transition-transform',
                        rule.enabled ? 'left-4' : 'left-0.5'
                      ]"
                    ></span>
                  </button>
                  <button
                    @click="deleteRule(rule)"
                    class="p-1 text-text-muted hover:text-red-400 transition-colors"
                    title="删除规则"
                  >
                    <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                    </svg>
                  </button>
                </div>
              </div>
              <div class="text-xs text-text-muted mt-1">
                {{ rule.action }} | {{ rule.oidPattern || '所有 OID' }}
              </div>
            </div>

            <div v-if="filterRules.length === 0" class="text-center text-text-muted text-sm py-4">
              暂无过滤规则
            </div>
          </div>
        </div>

        <!-- v3 用户管理 -->
        <div class="border-t border-border/30 p-3">
          <div class="flex items-center justify-between mb-3">
            <h3 class="text-sm font-medium text-text-primary">SNMPv3 用户</h3>
            <button
              @click="openV3UserModal(null)"
              class="p-1 text-text-muted hover:text-accent transition-colors"
              title="添加 v3 用户"
            >
              <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
              </svg>
            </button>
          </div>

          <div class="space-y-2">
            <div
              v-for="user in v3Users"
              :key="user.username"
              class="p-2 bg-bg-tertiary rounded-md border border-border/30"
            >
              <div class="flex items-center justify-between">
                <span class="text-sm text-text-primary truncate">{{ user.username }}</span>
                <div class="flex items-center gap-1">
                  <button
                    @click="openV3UserModal(user)"
                    class="p-1 text-text-muted hover:text-accent transition-colors"
                    title="编辑用户"
                  >
                    <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
                    </svg>
                  </button>
                  <button
                    @click="deleteV3User(user.username)"
                    class="p-1 text-text-muted hover:text-red-400 transition-colors"
                    title="删除用户"
                  >
                    <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                    </svg>
                  </button>
                </div>
              </div>
              <div class="text-xs text-text-muted mt-1">
                {{ user.securityLevel }} | {{ user.authProtocol }}/{{ user.privProtocol }}
              </div>
            </div>

            <div v-if="v3Users.length === 0" class="text-center text-text-muted text-sm py-4">
              暂无 v3 用户
            </div>
          </div>
        </div>
      </aside>

      <!-- 中间面板：Trap 记录列表 -->
      <main class="flex-1 flex flex-col min-w-0 overflow-hidden">
        <!-- 表格 -->
        <div class="flex-1 overflow-auto">
          <table class="w-full text-sm">
            <thead class="sticky top-0 bg-bg-secondary border-b border-border/50">
              <tr>
                <th class="w-10 px-3 py-2 text-left">
                  <input
                    type="checkbox"
                    :checked="isAllSelected"
                    @change="toggleSelectAll"
                    class="w-4 h-4 rounded border-border/50 bg-bg-tertiary"
                  />
                </th>
                <th class="px-3 py-2 text-left text-text-muted font-medium">时间</th>
                <th class="px-3 py-2 text-left text-text-muted font-medium">源 IP</th>
                <th class="px-3 py-2 text-left text-text-muted font-medium">Trap OID</th>
                <th class="px-3 py-2 text-left text-text-muted font-medium">名称</th>
                <th class="px-3 py-2 text-left text-text-muted font-medium">级别</th>
                <th class="px-3 py-2 text-left text-text-muted font-medium">状态</th>
                <th class="px-3 py-2 text-left text-text-muted font-medium">操作</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="trap in trapRecords"
                :key="trap.id"
                @click="selectTrap(trap)"
                :class="[
                  'border-b border-border/30 cursor-pointer transition-colors',
                  selectedTrap?.id === trap.id ? 'bg-accent-bg' : 'hover:bg-bg-hover',
                  isNewTrap(trap.id) ? 'animate-highlight bg-yellow-500/10' : ''
                ]"
              >
                <td class="px-3 py-2" @click.stop>
                  <input
                    type="checkbox"
                    :checked="selectedTrapIds.has(trap.id)"
                    @change="toggleSelection(trap.id)"
                    class="w-4 h-4 rounded border-border/50 bg-bg-tertiary"
                  />
                </td>
                <td class="px-3 py-2 text-text-secondary whitespace-nowrap">
                  {{ formatTime(trap.receivedAt) }}
                </td>
                <td class="px-3 py-2 text-text-primary font-mono">
                  {{ trap.sourceIP }}:{{ trap.sourcePort }}
                </td>
                <td class="px-3 py-2 text-text-secondary font-mono text-xs truncate max-w-[200px]">
                  {{ trap.trapOID }}
                </td>
                <td class="px-3 py-2 text-text-primary truncate max-w-[150px]">
                  {{ trap.trapName || '-' }}
                </td>
                <td class="px-3 py-2">
                  <span
                    :class="[
                      'inline-flex px-2 py-0.5 text-xs rounded border',
                      getSeverityClass(trap.severity)
                    ]"
                  >
                    {{ getSeverityText(trap.severity) }}
                  </span>
                </td>
                <td class="px-3 py-2">
                  <span
                    :class="[
                      'text-xs',
                      trap.acknowledged ? 'text-green-400' : 'text-yellow-400'
                    ]"
                  >
                    {{ trap.acknowledged ? '已确认' : '未确认' }}
                  </span>
                </td>
                <td class="px-3 py-2" @click.stop>
                  <div class="flex items-center gap-1">
                    <button
                      v-if="!trap.acknowledged"
                      @click="acknowledgeSingle(trap)"
                      class="p-1 text-text-muted hover:text-green-400 transition-colors"
                      title="确认"
                    >
                      <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
                      </svg>
                    </button>
                    <button
                      @click="deleteTrapRecord(trap)"
                      class="p-1 text-text-muted hover:text-red-400 transition-colors"
                      title="删除"
                    >
                      <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                      </svg>
                    </button>
                  </div>
                </td>
              </tr>

              <tr v-if="trapRecords.length === 0 && !recordsLoading">
                <td colspan="8" class="px-3 py-8 text-center text-text-muted">
                  暂无 Trap 记录
                </td>
              </tr>

              <tr v-if="recordsLoading">
                <td colspan="8" class="px-3 py-8 text-center text-text-muted">
                  加载中...
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <!-- 分页 -->
        <div class="flex items-center justify-between px-4 py-3 bg-bg-secondary border-t border-border/50 flex-shrink-0">
          <div class="text-sm text-text-muted">
            共 {{ totalRecords }} 条记录
          </div>
          <div class="flex items-center gap-2">
            <select
              :value="pageSize"
              @change="changePageSize(parseInt(($event.target as HTMLSelectElement).value))"
              class="px-2 py-1 text-sm bg-bg-tertiary border border-border/50 rounded text-text-primary"
            >
              <option :value="10">10 条/页</option>
              <option :value="20">20 条/页</option>
              <option :value="50">50 条/页</option>
              <option :value="100">100 条/页</option>
            </select>

            <div class="flex items-center gap-1">
              <button
                :disabled="currentPage === 1"
                @click="changePage(currentPage - 1)"
                class="px-2 py-1 text-sm bg-bg-tertiary hover:bg-bg-hover rounded disabled:opacity-50 disabled:cursor-not-allowed"
              >
                上一页
              </button>
              <span class="px-3 py-1 text-sm text-text-secondary">
                {{ currentPage }} / {{ totalPages || 1 }}
              </span>
              <button
                :disabled="currentPage >= totalPages"
                @click="changePage(currentPage + 1)"
                class="px-2 py-1 text-sm bg-bg-tertiary hover:bg-bg-hover rounded disabled:opacity-50 disabled:cursor-not-allowed"
              >
                下一页
              </button>
            </div>
          </div>
        </div>
      </main>

      <!-- 右侧面板：Trap 详情 -->
      <aside
        v-if="showDetailPanel && selectedTrap"
        :style="{ width: `${rightPanelWidth}px` }"
        class="flex flex-col bg-bg-secondary border-l border-border/50 flex-shrink-0"
      >
        <div class="p-4 border-b border-border/30">
          <div class="flex items-center justify-between mb-2">
            <h3 class="text-sm font-medium text-text-primary">Trap 详情</h3>
            <button
              @click="selectedTrap = null"
              class="p-1 text-text-muted hover:text-text-primary transition-colors"
            >
              <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>
        </div>

        <div class="flex-1 overflow-y-auto p-4 space-y-4">
          <!-- 基本信息 -->
          <div class="space-y-2">
            <div class="flex justify-between text-sm">
              <span class="text-text-muted">接收时间</span>
              <span class="text-text-primary">{{ selectedTrap.receivedAt }}</span>
            </div>
            <div class="flex justify-between text-sm">
              <span class="text-text-muted">源地址</span>
              <span class="text-text-primary font-mono">{{ selectedTrap.sourceIP }}:{{ selectedTrap.sourcePort }}</span>
            </div>
            <div class="flex justify-between text-sm">
              <span class="text-text-muted">SNMP 版本</span>
              <span class="text-text-primary">{{ selectedTrap.version }}</span>
            </div>
            <div class="flex justify-between text-sm">
              <span class="text-text-muted">Community</span>
              <span class="text-text-primary">{{ selectedTrap.community }}</span>
            </div>
            <div class="flex justify-between text-sm">
              <span class="text-text-muted">Trap OID</span>
              <span class="text-text-primary font-mono text-xs break-all">{{ selectedTrap.trapOID }}</span>
            </div>
            <div class="flex justify-between text-sm">
              <span class="text-text-muted">Trap 名称</span>
              <span class="text-text-primary">{{ selectedTrap.trapName || '-' }}</span>
            </div>
            <div class="flex justify-between text-sm">
              <span class="text-text-muted">严重级别</span>
              <span
                :class="[
                  'inline-flex px-2 py-0.5 text-xs rounded border',
                  getSeverityClass(selectedTrap.severity)
                ]"
              >
                {{ getSeverityText(selectedTrap.severity) }}
              </span>
            </div>
            <div class="flex justify-between text-sm">
              <span class="text-text-muted">确认状态</span>
              <span :class="selectedTrap.acknowledged ? 'text-green-400' : 'text-yellow-400'">
                {{ selectedTrap.acknowledged ? '已确认' : '未确认' }}
              </span>
            </div>
            <div v-if="selectedTrap.acknowledgedAt" class="flex justify-between text-sm">
              <span class="text-text-muted">确认时间</span>
              <span class="text-text-primary">{{ selectedTrap.acknowledgedAt }}</span>
            </div>
          </div>

          <!-- VarBinds -->
          <div>
            <h4 class="text-sm font-medium text-text-primary mb-2">变量绑定</h4>
            <div class="space-y-2">
              <div
                v-for="(vb, idx) in parseVariables(selectedTrap.variables)"
                :key="idx"
                class="p-2 bg-bg-tertiary rounded text-xs"
              >
                <div class="font-mono text-text-secondary break-all">{{ vb.oid }}</div>
                  <div v-if="vb.oidName" class="text-xs text-text-muted mt-0.5">{{ vb.oidName }}</div>
                  <div class="text-text-primary mt-1">{{ vb.value }}</div>
              </div>
              <div v-if="parseVariables(selectedTrap.variables).length === 0" class="text-text-muted text-xs">
                无变量绑定数据
              </div>
            </div>
          </div>

          <!-- 操作按钮 -->
          <div class="flex gap-2 pt-2">
            <button
              v-if="!selectedTrap.acknowledged"
              @click="acknowledgeSingle(selectedTrap)"
              class="flex-1 px-3 py-2 text-sm bg-green-600 hover:bg-green-700 text-white rounded-md transition-colors"
            >
              确认
            </button>
            <button
              @click="deleteTrapRecord(selectedTrap)"
              class="flex-1 px-3 py-2 text-sm bg-red-600 hover:bg-red-700 text-white rounded-md transition-colors"
            >
              删除
            </button>
          </div>
        </div>
      </aside>
    </div>

    <!-- 服务器配置对话框 -->
    <div
      v-if="showConfigModal"
      class="fixed inset-0 bg-black/50 flex items-center justify-center z-50"
      @click.self="showConfigModal = false"
    >
      <div class="bg-bg-secondary rounded-lg shadow-xl w-[400px] max-h-[80vh] overflow-hidden">
        <div class="flex items-center justify-between px-4 py-3 border-b border-border/50">
          <h3 class="text-base font-medium text-text-primary">服务器配置</h3>
          <button
            @click="showConfigModal = false"
            class="p-1 text-text-muted hover:text-text-primary transition-colors"
          >
            <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        <div class="p-4 space-y-4">
          <div>
            <label class="block text-sm text-text-muted mb-1">监听端口</label>
            <input
              v-model.number="editingConfig!.trapPort"
              type="number"
              class="w-full px-3 py-2 text-sm bg-bg-tertiary border border-border/50 rounded-md text-text-primary focus:outline-none focus:border-accent"
            />
          </div>
          <div>
            <label class="block text-sm text-text-muted mb-1">Community</label>
            <input
              v-model="editingConfig!.trapCommunity"
              type="text"
              class="w-full px-3 py-2 text-sm bg-bg-tertiary border border-border/50 rounded-md text-text-primary focus:outline-none focus:border-accent"
            />
          </div>
          <div>
            <label class="block text-sm text-text-muted mb-1">保留天数</label>
            <input
              v-model.number="editingConfig!.maxStorageDays"
              type="number"
              class="w-full px-3 py-2 text-sm bg-bg-tertiary border border-border/50 rounded-md text-text-primary focus:outline-none focus:border-accent"
            />
          </div>
        </div>

        <div class="flex justify-end gap-2 px-4 py-3 border-t border-border/50">
          <button
            @click="showConfigModal = false"
            class="px-4 py-2 text-sm bg-bg-tertiary hover:bg-bg-hover text-text-secondary rounded-md transition-colors"
          >
            取消
          </button>
          <button
            @click="saveConfig"
            class="px-4 py-2 text-sm bg-accent hover:bg-accent-dark text-white rounded-md transition-colors"
          >
            保存
          </button>
        </div>
      </div>
    </div>

    <!-- v3 用户管理对话框 -->
    <div
      v-if="showV3UserModal"
      class="fixed inset-0 bg-black/50 flex items-center justify-center z-50"
      @click.self="showV3UserModal = false"
    >
      <div class="bg-bg-secondary rounded-lg shadow-xl w-[520px] max-h-[85vh] overflow-hidden">
        <div class="flex items-center justify-between px-4 py-3 border-b border-border/50">
          <h3 class="text-base font-medium text-text-primary">
            {{ editingV3User ? '编辑 SNMPv3 用户' : '添加 SNMPv3 用户' }}
          </h3>
          <button
            @click="showV3UserModal = false"
            class="p-1 text-text-muted hover:text-text-primary transition-colors"
          >
            <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        <div class="p-4 space-y-4">
          <!-- 用户名 -->
          <div>
            <label class="block text-sm text-text-muted mb-1">用户名 *</label>
            <input
              v-model="v3UserForm.username"
              type="text"
              :disabled="!!editingV3User"
              placeholder="输入 SNMPv3 用户名"
              class="w-full px-3 py-2 text-sm bg-bg-tertiary border border-border/50 rounded-md text-text-primary placeholder-text-muted focus:outline-none focus:border-accent disabled:opacity-50"
            />
          </div>

          <!-- 安全级别 -->
          <div>
            <label class="block text-sm text-text-muted mb-1">安全级别 *</label>
            <select
              v-model="v3UserForm.securityLevel"
              class="w-full px-3 py-2 text-sm bg-bg-tertiary border border-border/50 rounded-md text-text-primary focus:outline-none focus:border-accent"
            >
              <option value="noAuthNoPriv">无认证无加密 (noAuthNoPriv)</option>
              <option value="authNoPriv">认证无加密 (authNoPriv)</option>
              <option value="authPriv">认证且加密 (authPriv)</option>
            </select>
          </div>

          <!-- 认证协议和密钥 -->
          <div v-if="v3UserForm.securityLevel !== 'noAuthNoPriv'">
            <label class="block text-sm text-text-muted mb-1">认证协议 *</label>
            <select
              v-model="v3UserForm.authProtocol"
              class="w-full px-3 py-2 text-sm bg-bg-tertiary border border-border/50 rounded-md text-text-primary focus:outline-none focus:border-accent"
            >
              <option value="MD5">MD5</option>
              <option value="SHA">SHA</option>
              <option value="SHA224">SHA-224</option>
              <option value="SHA256">SHA-256</option>
              <option value="SHA384">SHA-384</option>
              <option value="SHA512">SHA-512</option>
            </select>
          </div>

          <div v-if="v3UserForm.securityLevel !== 'noAuthNoPriv'">
            <label class="block text-sm text-text-muted mb-1">认证密钥 {{ editingV3User ? '(留空保持原值)' : '*' }}</label>
            <input
              v-model="v3UserForm.authKey"
              type="password"
              placeholder="输入认证密钥"
              class="w-full px-3 py-2 text-sm bg-bg-tertiary border border-border/50 rounded-md text-text-primary placeholder-text-muted focus:outline-none focus:border-accent font-mono"
            />
          </div>

          <!-- 加密协议和密钥 -->
          <div v-if="v3UserForm.securityLevel === 'authPriv'">
            <label class="block text-sm text-text-muted mb-1">加密协议 *</label>
            <select
              v-model="v3UserForm.privProtocol"
              class="w-full px-3 py-2 text-sm bg-bg-tertiary border border-border/50 rounded-md text-text-primary focus:outline-none focus:border-accent"
            >
              <option value="DES">DES</option>
              <option value="AES">AES</option>
              <option value="AES192">AES-192</option>
              <option value="AES256">AES-256</option>
              <option value="AES192C">AES-192C</option>
              <option value="AES256C">AES-256C</option>
            </select>
          </div>

          <div v-if="v3UserForm.securityLevel === 'authPriv'">
            <label class="block text-sm text-text-muted mb-1">加密密钥 {{ editingV3User ? '(留空保持原值)' : '*' }}</label>
            <input
              v-model="v3UserForm.privKey"
              type="password"
              placeholder="输入加密密钥"
              class="w-full px-3 py-2 text-sm bg-bg-tertiary border border-border/50 rounded-md text-text-primary placeholder-text-muted focus:outline-none focus:border-accent font-mono"
            />
          </div>
        </div>

        <div class="flex justify-end gap-2 px-4 py-3 border-t border-border/50">
          <button
            @click="showV3UserModal = false"
            class="px-4 py-2 text-sm bg-bg-tertiary hover:bg-bg-hover text-text-secondary rounded-md transition-colors"
          >
            取消
          </button>
          <button
            @click="saveV3User"
            :disabled="!v3UserForm.username || (v3UserForm.securityLevel !== 'noAuthNoPriv' && !v3UserForm.authKey && !editingV3User) || (v3UserForm.securityLevel === 'authPriv' && !v3UserForm.privKey && !editingV3User)"
            class="px-4 py-2 text-sm bg-accent hover:bg-accent-dark text-white rounded-md transition-colors disabled:opacity-50"
          >
            保存
          </button>
        </div>
      </div>
    </div>

    <!-- 过滤规则对话框 -->
    <div
      v-if="showRuleModal"
      class="fixed inset-0 bg-black/50 flex items-center justify-center z-50"
      @click.self="showRuleModal = false"
    >
      <div class="bg-bg-secondary rounded-lg shadow-xl w-[500px] max-h-[80vh] overflow-hidden">
        <div class="flex items-center justify-between px-4 py-3 border-b border-border/50">
          <h3 class="text-base font-medium text-text-primary">
            {{ editingRule?.id ? '编辑规则' : '新建规则' }}
          </h3>
          <button
            @click="showRuleModal = false"
            class="p-1 text-text-muted hover:text-text-primary transition-colors"
          >
            <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        <div class="p-4 space-y-4 max-h-[60vh] overflow-y-auto">
          <div>
            <label class="block text-sm text-text-muted mb-1">规则名称</label>
            <input
              v-model="editingRule!.name"
              type="text"
              class="w-full px-3 py-2 text-sm bg-bg-tertiary border border-border/50 rounded-md text-text-primary focus:outline-none focus:border-accent"
            />
          </div>
          <div>
            <label class="block text-sm text-text-muted mb-1">动作</label>
            <select
              v-model="editingRule!.action"
              class="w-full px-3 py-2 text-sm bg-bg-tertiary border border-border/50 rounded-md text-text-primary focus:outline-none focus:border-accent"
            >
              <option value="accept">接受</option>
              <option value="drop">丢弃</option>
              <option value="severity_override">修改级别</option>
            </select>
          </div>
          <div>
            <label class="block text-sm text-text-muted mb-1">OID 模式</label>
            <input
              v-model="editingRule!.oidPattern"
              type="text"
              placeholder="例如: 1.3.6.1.4.1.*"
              class="w-full px-3 py-2 text-sm bg-bg-tertiary border border-border/50 rounded-md text-text-primary focus:outline-none focus:border-accent"
            />
          </div>
          <div>
            <label class="block text-sm text-text-muted mb-1">源 IP 模式</label>
            <input
              v-model="editingRule!.sourceIPPattern"
              type="text"
              placeholder="例如: 192.168.1.*"
              class="w-full px-3 py-2 text-sm bg-bg-tertiary border border-border/50 rounded-md text-text-primary focus:outline-none focus:border-accent"
            />
          </div>
          <div>
            <label class="block text-sm text-text-muted mb-1">Community 模式</label>
            <input
              v-model="editingRule!.communityPattern"
              type="text"
              class="w-full px-3 py-2 text-sm bg-bg-tertiary border border-border/50 rounded-md text-text-primary focus:outline-none focus:border-accent"
            />
          </div>
          <div v-if="editingRule!.action === 'severity_override'">
            <label class="block text-sm text-text-muted mb-1">覆盖级别</label>
            <select
              v-model="editingRule!.overrideSeverity"
              class="w-full px-3 py-2 text-sm bg-bg-tertiary border border-border/50 rounded-md text-text-primary focus:outline-none focus:border-accent"
            >
              <option value="critical">严重</option>
              <option value="major">重要</option>
              <option value="minor">次要</option>
              <option value="info">信息</option>
            </select>
          </div>
          <div>
            <label class="block text-sm text-text-muted mb-1">优先级</label>
            <input
              v-model.number="editingRule!.priority"
              type="number"
              class="w-full px-3 py-2 text-sm bg-bg-tertiary border border-border/50 rounded-md text-text-primary focus:outline-none focus:border-accent"
            />
          </div>
          <div>
            <label class="block text-sm text-text-muted mb-1">描述</label>
            <textarea
              v-model="editingRule!.description"
              rows="2"
              class="w-full px-3 py-2 text-sm bg-bg-tertiary border border-border/50 rounded-md text-text-primary focus:outline-none focus:border-accent resize-none"
            ></textarea>
          </div>
        </div>

        <div class="flex justify-end gap-2 px-4 py-3 border-t border-border/50">
          <button
            @click="showRuleModal = false"
            class="px-4 py-2 text-sm bg-bg-tertiary hover:bg-bg-hover text-text-secondary rounded-md transition-colors"
          >
            取消
          </button>
          <button
            @click="saveRule"
            class="px-4 py-2 text-sm bg-accent hover:bg-accent-dark text-white rounded-md transition-colors"
          >
            保存
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
@keyframes highlight {
  0% {
    background-color: rgba(234, 179, 8, 0.2);
  }
  100% {
    background-color: transparent;
  }
}

.animate-highlight {
  animation: highlight 5s ease-out;
}
</style>
