/**
 * SNMP 轮询状态管理组合式函数
 *
 * 提供轮询调度器状态管理、目标管理、实时结果监听等功能
 * 基于 Wails Events 实现前后端实时通信
 */

import { ref, computed, onMounted, onUnmounted } from 'vue'
import { SNMPPollingAPI, SNMPPollingEvents } from '../services/snmpApi'
import { getLogger } from '../utils/logger'
import type {
  SchedulerStatusVM,
  PollingTargetVM,
  PollingResultVM,
  PollingStatsVM,
  CredentialVM,
  PollingTemplateVM,
  PollingTargetFilterVM,
} from '../bindings/github.com/NetWeaverGo/core/internal/ui/models'

const logger = getLogger()

/** 新结果高亮持续时间（毫秒） */
const HIGHLIGHT_DURATION = 5000

/** 轮询结果事件接口 */
export interface PollingResultEvent {
  targetId: number
  targetIP: string
  status: 'success' | 'failure' | 'timeout'
  pollTime: number
  oidCount: number
  batchId: string
  error?: string
}

/**
 * SNMP 轮询状态管理组合式函数
 *
 * @description
 * - 监听 Wails 事件实现轮询结果实时推送
 * - 维护调度器状态同步
 * - 提供目标管理方法
 * - 新结果高亮效果管理
 */
export function useSNMPPolling() {
  // ==================== 状态 ====================

  /** 调度器状态 */
  const schedulerStatus = ref<SchedulerStatusVM | null>(null)

  /** 轮询目标列表 */
  const targets = ref<PollingTargetVM[]>([])

  /** 凭据列表 */
  const credentials = ref<CredentialVM[]>([])

  /** 模板列表 */
  const templates = ref<PollingTemplateVM[]>([])

  /** 最新轮询结果事件（按目标 ID 索引） */
  const latestResults = ref<Map<number, PollingResultEvent>>(new Map())

  /** 目标统计信息（按目标 ID 索引） */
  const targetStats = ref<Map<number, PollingStatsVM>>(new Map())

  /** 事件连接状态 */
  const isConnected = ref(false)

  /** 新结果目标 ID 集合（用于高亮） */
  const newResultTargetIds = ref<Set<number>>(new Set())

  /** 加载状态 */
  const isLoading = ref(false)
  const statusLoading = ref(false)
  const targetsLoading = ref(false)

  // ==================== 内部变量 ====================

  /** 取消订阅函数列表 */
  const unsubscribers: (() => void)[] = []

  /** 高亮定时器映射 */
  const highlightTimers = new Map<number, ReturnType<typeof setTimeout>>()

  // ==================== 计算属性 ====================

  /** 调度器是否运行中 */
  const isRunning = computed(() => schedulerStatus.value?.isRunning ?? false)

  /** 启用的目标数量 */
  const enabledTargetCount = computed(() =>
    targets.value.filter(t => t.enabled).length
  )

  /** 目标总数 */
  const targetCount = computed(() => targets.value.length)

  /** 成功目标数量 */
  const successTargetCount = computed(() => {
    let count = 0
    for (const target of targets.value) {
      const result = latestResults.value.get(target.id)
      if (result?.status === 'success') count++
    }
    return count
  })

  // ==================== 方法 ====================

  /**
   * 加载调度器状态
   */
  async function loadSchedulerStatus() {
    statusLoading.value = true
    try {
      const status = await SNMPPollingAPI.getSchedulerStatus()
      schedulerStatus.value = status
      if (status) {
        logger.debug(`SNMP-Polling: 调度器状态已加载 - ${status.isRunning}`)
      }
    } catch (error) {
      logger.error('SNMP-Polling', '加载调度器状态失败', error)
    } finally {
      statusLoading.value = false
    }
  }

  /**
   * 加载目标列表
   */
  async function loadTargets(filter?: PollingTargetFilterVM) {
    targetsLoading.value = true
    try {
      targets.value = await SNMPPollingAPI.getPollingTargets(filter)
      logger.debug(`SNMP-Polling: 目标列表已加载 - ${targets.value.length} 个`)
    } catch (error) {
      logger.error('SNMP-Polling', '加载目标列表失败', error)
    } finally {
      targetsLoading.value = false
    }
  }

  /**
   * 加载凭据列表
   */
  async function loadCredentials() {
    try {
      credentials.value = await SNMPPollingAPI.getCredentials()
      logger.debug(`SNMP-Polling: 凭据列表已加载 - ${credentials.value.length} 个`)
    } catch (error) {
      logger.error('SNMP-Polling', '加载凭据列表失败', error)
    }
  }

  /**
   * 加载模板列表
   */
  async function loadTemplates() {
    try {
      templates.value = await SNMPPollingAPI.getPollingTemplates()
      logger.debug(`SNMP-Polling: 模板列表已加载 - ${templates.value.length} 个`)
    } catch (error) {
      logger.error('SNMP-Polling', '加载模板列表失败', error)
    }
  }

  /**
   * 加载所有数据
   */
  async function loadAll() {
    isLoading.value = true
    try {
      await Promise.all([
        loadSchedulerStatus(),
        loadTargets(),
        loadCredentials(),
        loadTemplates(),
      ])
    } finally {
      isLoading.value = false
    }
  }

  /**
   * 启动调度器
   */
  async function startScheduler() {
    try {
      await SNMPPollingAPI.startScheduler()
      await loadSchedulerStatus()
      logger.info('SNMP-Polling: 调度器已启动')
    } catch (error) {
      logger.error('SNMP-Polling', '启动调度器失败', error)
      throw error
    }
  }

  /**
   * 停止调度器
   */
  async function stopScheduler() {
    try {
      await SNMPPollingAPI.stopScheduler()
      await loadSchedulerStatus()
      logger.info('SNMP-Polling: 调度器已停止')
    } catch (error) {
      logger.error('SNMP-Polling', '停止调度器失败', error)
      throw error
    }
  }

  /**
   * 立即轮询单个目标
   */
  async function pollNow(targetId: number): Promise<PollingResultVM[]> {
    try {
      const results = await SNMPPollingAPI.pollNow(targetId)
      // 更新最新结果事件（构造成功事件）
      const firstResult = results[0]
      if (firstResult) {
        latestResults.value.set(targetId, {
          targetId,
          targetIP: firstResult.targetIP,
          status: 'success',
          pollTime: Date.now(),
          oidCount: results.length,
          batchId: firstResult.batchId,
        })
      }
      // 添加高亮效果
      addHighlight(targetId)
      logger.debug(`SNMP-Polling: 目标 ${targetId} 轮询完成 - ${results.length} 条结果`)
      return results
    } catch (error) {
      logger.error('SNMP-Polling', `轮询目标 ${targetId} 失败`, error)
      throw error
    }
  }

  /**
   * 立即轮询所有目标
   */
  async function pollAllNow(): Promise<number> {
    try {
      const successCount = await SNMPPollingAPI.pollAllNow()
      // 刷新目标列表以获取最新状态
      await loadTargets()
      logger.debug(`SNMP-Polling: 批量轮询完成 - ${successCount} 个目标成功`)
      return successCount
    } catch (error) {
      logger.error('SNMP-Polling', '批量轮询失败', error)
      throw error
    }
  }

  /**
   * 启用目标
   */
  async function enableTarget(targetId: number) {
    try {
      await SNMPPollingAPI.enablePollingTarget(targetId)
      await loadTargets()
      logger.info(`SNMP-Polling: 目标 ${targetId} 已启用`)
    } catch (error) {
      logger.error('SNMP-Polling', `启用目标 ${targetId} 失败`, error)
      throw error
    }
  }

  /**
   * 禁用目标
   */
  async function disableTarget(targetId: number) {
    try {
      await SNMPPollingAPI.disablePollingTarget(targetId)
      await loadTargets()
      logger.info(`SNMP-Polling: 目标 ${targetId} 已禁用`)
    } catch (error) {
      logger.error('SNMP-Polling', `禁用目标 ${targetId} 失败`, error)
      throw error
    }
  }

  /**
   * 切换目标启用状态
   */
  async function toggleTarget(targetId: number, enabled: boolean) {
    if (enabled) {
      await enableTarget(targetId)
    } else {
      await disableTarget(targetId)
    }
  }

  /**
   * 获取目标统计信息
   */
  async function loadTargetStats(targetId: number) {
    try {
      const stats = await SNMPPollingAPI.getPollingStats(targetId)
      if (stats) {
        targetStats.value.set(targetId, stats)
      }
      return stats
    } catch (error) {
      logger.error('SNMP-Polling', `获取目标 ${targetId} 统计失败`, error)
      return null
    }
  }

  /**
   * 检查是否为新结果（用于高亮）
   */
  function isNewResult(targetId: number): boolean {
    return newResultTargetIds.value.has(targetId)
  }

  /**
   * 添加高亮效果
   */
  function addHighlight(targetId: number) {
    newResultTargetIds.value.add(targetId)
    
    // 清除之前的定时器
    const existingTimer = highlightTimers.get(targetId)
    if (existingTimer) {
      clearTimeout(existingTimer)
    }
    
    // 设置新的定时器
    const timer = setTimeout(() => {
      newResultTargetIds.value.delete(targetId)
      highlightTimers.delete(targetId)
    }, HIGHLIGHT_DURATION)
    
    highlightTimers.set(targetId, timer)
  }

  /**
   * 启动事件监听
   */
  function startListening() {
    if (isConnected.value) return

    try {
      // 监听轮询结果事件
      const unsubResult = SNMPPollingEvents.onPollingResult((result: unknown) => {
        const resultEvent = result as PollingResultEvent
        logger.debug(`SNMP-Polling: 收到轮询结果 - 目标 ${resultEvent.targetId}`)
        latestResults.value.set(resultEvent.targetId, resultEvent)
        addHighlight(resultEvent.targetId)
      })
      unsubscribers.push(unsubResult)

      // 监听调度器状态变更事件
      const unsubStatus = SNMPPollingEvents.onSchedulerStatusChanged((status: unknown) => {
        const statusVM = status as SchedulerStatusVM
        logger.debug(`SNMP-Polling: 调度器状态变更 - ${statusVM.isRunning}`)
        schedulerStatus.value = statusVM
      })
      unsubscribers.push(unsubStatus)

      // 监听轮询错误事件
      const unsubError = SNMPPollingEvents.onPollingError((error) => {
        logger.error('SNMP-Polling', `目标 ${error.targetId} 轮询错误`, error.error)
      })
      unsubscribers.push(unsubError)

      isConnected.value = true
      logger.info('SNMP-Polling: 轮询事件监听已启动')
    } catch (error) {
      logger.error('SNMP-Polling', '启动轮询事件监听失败', error)
      isConnected.value = false
    }
  }

  /**
   * 停止事件监听
   */
  function stopListening() {
    // 执行所有取消订阅函数
    for (const unsub of unsubscribers) {
      try {
        unsub()
      } catch {
        // 忽略取消订阅错误
      }
    }
    unsubscribers.length = 0

    // 清理高亮定时器
    for (const timer of highlightTimers.values()) {
      clearTimeout(timer)
    }
    highlightTimers.clear()

    isConnected.value = false
    logger.info('SNMP-Polling: 轮询事件监听已停止')
  }

  /**
   * 获取目标的最新结果
   */
  function getLatestResult(targetId: number): PollingResultEvent | undefined {
    return latestResults.value.get(targetId)
  }

  /**
   * 获取目标的统计信息
   */
  function getTargetStats(targetId: number): PollingStatsVM | undefined {
    return targetStats.value.get(targetId)
  }

  /**
   * 根据凭据 ID 获取凭据
   */
  function getCredentialById(id: number): CredentialVM | undefined {
    return credentials.value.find(c => c.id === id)
  }

  /**
   * 根据模板 ID 获取模板
   */
  function getTemplateById(id: number): PollingTemplateVM | undefined {
    return templates.value.find(t => t.id === id)
  }

  // ==================== 生命周期 ====================

  onMounted(() => {
    startListening()
  })

  onUnmounted(() => {
    stopListening()
  })

  // ==================== 返回 ====================

  return {
    // 状态
    schedulerStatus,
    targets,
    credentials,
    templates,
    latestResults,
    targetStats,
    isConnected,
    isLoading,
    statusLoading,
    targetsLoading,
    
    // 计算属性
    isRunning,
    enabledTargetCount,
    targetCount,
    successTargetCount,
    
    // 方法
    loadSchedulerStatus,
    loadTargets,
    loadCredentials,
    loadTemplates,
    loadAll,
    startScheduler,
    stopScheduler,
    pollNow,
    pollAllNow,
    enableTarget,
    disableTarget,
    toggleTarget,
    loadTargetStats,
    isNewResult,
    startListening,
    stopListening,
    getLatestResult,
    getTargetStats,
    getCredentialById,
    getTemplateById,
  }
}
