/**
 * SNMP Trap 实时流组合式函数
 *
 * 提供 Trap 事件的实时监听、监听器状态同步、确认操作等功能
 * 基于 Wails Events 实现前后端实时通信
 * 支持声音和桌面通知功能
 */

import { ref, onMounted, onUnmounted, computed } from 'vue'
import { SNMPTrapAPI, SNMPTrapEvents, generateTempId } from '../services/snmpApi'
import type { TrapRecordVM, ListenerStatusVM, TrapStatsVM } from '../bindings/github.com/NetWeaverGo/core/internal/ui/models'
import { getLogger } from '../utils/logger'

const logger = getLogger()

/** 最大保留的实时 Trap 缓存数量 */
const MAX_LATEST_TRAPS = 100

/** 新 Trap 高亮持续时间（毫秒） */
const HIGHLIGHT_DURATION = 5000

/** 通知设置接口 */
export interface NotificationSettings {
  soundEnabled: boolean
  desktopEnabled: boolean
  criticalSound: boolean
  warningSound: boolean
  infoSound: boolean
}

const DEFAULT_NOTIFICATION_SETTINGS: NotificationSettings = {
  soundEnabled: true,
  desktopEnabled: false,
  criticalSound: true,
  warningSound: true,
  infoSound: false,
}

/** Trap 事件接口（轻量级，用于实时推送） */
export interface TrapEvent {
  sourceIP: string
  sourcePort: number
  trapOID: string
  trapName: string
  severity: string
  community: string
  version: string
  receivedAt: string
}

/**
 * SNMP Trap 实时流组合式函数
 *
 * @description
 * - 监听 Wails 事件实现 Trap 实时推送
 * - 维护监听器状态同步
 * - 提供确认操作方法
 * - 新 Trap 高亮效果管理
 */
export function useSNMPTrapStream() {
  // ==================== 状态 ====================

  /** 最新接收的 Trap 列表（实时缓存） */
  const latestTraps = ref<TrapRecordVM[]>([])

  /** 监听器状态 */
  const listenerStatus = ref<ListenerStatusVM | null>(null)

  /** Trap 统计信息 */
  const trapStats = ref<TrapStatsVM | null>(null)

  /** 事件连接状态 */
  const isConnected = ref(false)

  /** 新 Trap ID 集合（用于高亮） */
  const newTrapIds = ref<Set<number>>(new Set())

  /** 加载状态 */
  const statusLoading = ref(false)

  /** 未确认数量 */
  const unacknowledgedCount = ref(0)

  /** 通知设置 */
  const notificationSettings = ref<NotificationSettings>({ ...DEFAULT_NOTIFICATION_SETTINGS })

  /** 桌面通知权限状态 */
  const notificationPermission = ref<NotificationPermission>('default')

  /** 通知权限已授予 */
  const hasNotificationPermission = computed(() => notificationPermission.value === 'granted')

  // ==================== 内部变量 ====================

  /** 取消订阅函数列表 */
  const unsubscribers: (() => void)[] = []

  /** 高亮定时器映射 */
  const highlightTimers = new Map<number, ReturnType<typeof setTimeout>>()

  // ==================== 方法 ====================

  /**
   * 启动事件监听
   */
  function startListening() {
    if (isConnected.value) return

    try {
      // 监听 Trap 接收事件
      const unsubTrap = SNMPTrapEvents.onTrapReceived((trap: unknown) => {
        const trapEvent = trap as TrapEvent
        logger.debug(`SNMP-Trap: 收到 Trap 事件 - ${trapEvent.trapOID}`)
        handleTrapReceived(trapEvent)
      })
      unsubscribers.push(unsubTrap)

      // 监听监听器状态变更事件
      const unsubStatus = SNMPTrapEvents.onListenerStatusChanged((status: unknown) => {
        const statusVM = status as ListenerStatusVM
        logger.debug(`SNMP-Trap: 监听器状态变更 - ${statusVM.isRunning}`)
        listenerStatus.value = statusVM
      })
      unsubscribers.push(unsubStatus)

      // 监听统计更新事件
      const unsubStats = SNMPTrapEvents.onTrapStats((stats: unknown) => {
        const statsVM = stats as TrapStatsVM
        logger.debug(`SNMP-Trap: 统计信息更新 - ${statsVM.totalCount}`)
        trapStats.value = statsVM
        unacknowledgedCount.value = statsVM.unacknowledged
      })
      unsubscribers.push(unsubStats)

      isConnected.value = true
      logger.info('SNMP-Trap: Trap 事件监听已启动')
    } catch (error) {
      logger.error('SNMP-Trap', '启动 Trap 事件监听失败', error)
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
    logger.info('SNMP-Trap: Trap 事件监听已停止')
  }

  /**
   * 处理接收到的 Trap 事件
   */
  function handleTrapReceived(trap: TrapEvent) {
    logger.debug(`SNMP-Trap: 处理 Trap - ${trap.sourceIP} -> ${trap.trapOID}`)
    // 将 TrapEvent 转换为 TrapRecordVM 格式并添加到最新列表
    const record: Partial<TrapRecordVM> = {
      id: generateTempId(), // P3-8: 使用改进的临时 ID 生成器
      sourceIP: trap.sourceIP,
      sourcePort: trap.sourcePort,
      version: trap.version,
      community: trap.community,
      trapOID: trap.trapOID,
      trapName: trap.trapName,
      enterprise: '',
      genericTrap: 0,
      specificTrap: 0,
      severity: trap.severity,
      variables: '',
      acknowledged: false,
      acknowledgedAt: '',
      receivedAt: new Date(trap.receivedAt).toLocaleString('zh-CN'),
    }

    // 添加到最新列表头部
    latestTraps.value.unshift(record as TrapRecordVM)

    // 限制缓存大小
    if (latestTraps.value.length > MAX_LATEST_TRAPS) {
      latestTraps.value = latestTraps.value.slice(0, MAX_LATEST_TRAPS)
    }

    // 添加高亮效果
    addHighlight(record.id!)

    // 更新未确认计数
    unacknowledgedCount.value++
  }

  /**
   * 添加新 Trap 高亮效果
   */
  function addHighlight(id: number) {
    newTrapIds.value.add(id)

    // 清除之前的定时器
    if (highlightTimers.has(id)) {
      clearTimeout(highlightTimers.get(id)!)
    }

    // 设置高亮过期定时器
    const timer = setTimeout(() => {
      newTrapIds.value.delete(id)
      highlightTimers.delete(id)
    }, HIGHLIGHT_DURATION)

    highlightTimers.set(id, timer)
  }

  /**
   * 刷新监听器状态
   */
  async function refreshStatus() {
    statusLoading.value = true
    try {
      listenerStatus.value = await SNMPTrapAPI.getListenerStatus()
      logger.debug(`SNMP-Trap: 监听器状态已刷新`)
    } catch (error) {
      logger.error(`SNMP-Trap: 刷新监听器状态失败 - ${error}`)
    } finally {
      statusLoading.value = false
    }
  }

  /**
   * 刷新统计信息
   */
  async function refreshStats() {
    try {
      const stats = await SNMPTrapAPI.getTrapStats()
      if (stats) {
        trapStats.value = stats
        unacknowledgedCount.value = stats.unacknowledged
      }
    } catch (error) {
      logger.error(`SNMP-Trap: 刷新统计信息失败 - ${error}`)
    }
  }

  /**
   * 确认单个 Trap
   */
  async function acknowledgeTrap(id: number) {
    try {
      await SNMPTrapAPI.acknowledgeTrap(id)
      // 更新本地缓存
      const trap = latestTraps.value.find(t => t.id === id)
      if (trap) {
        trap.acknowledged = true
        trap.acknowledgedAt = new Date().toLocaleString('zh-CN')
      }
      unacknowledgedCount.value = Math.max(0, unacknowledgedCount.value - 1)
      logger.info(`SNMP-Trap: Trap 已确认 - ID ${id}`)
    } catch (error) {
      logger.error(`SNMP-Trap: 确认 Trap 失败 - ${error}`)
      throw error
    }
  }

  /**
   * 批量确认 Trap
   */
  async function batchAcknowledge(ids: number[]) {
    try {
      await SNMPTrapAPI.batchAcknowledgeTraps(ids)
      // 更新本地缓存
      const idSet = new Set(ids)
      for (const trap of latestTraps.value) {
        if (idSet.has(trap.id)) {
          trap.acknowledged = true
          trap.acknowledgedAt = new Date().toLocaleString('zh-CN')
        }
      }
      unacknowledgedCount.value = Math.max(0, unacknowledgedCount.value - ids.length)
      logger.info(`SNMP-Trap: 批量确认 Trap 完成 - ${ids.length} 条`)
    } catch (error) {
      logger.error(`SNMP-Trap: 批量确认 Trap 失败 - ${error}`)
      throw error
    }
  }

  /**
   * 检查 Trap 是否为新（高亮状态）
   */
  function isNewTrap(id: number): boolean {
    return newTrapIds.value.has(id)
  }

  /**
   * 清空最新 Trap 缓存
   */
  function clearLatestTraps() {
    latestTraps.value = []
    newTrapIds.value.clear()
    highlightTimers.forEach(timer => clearTimeout(timer))
    highlightTimers.clear()
  }

  // ==================== 通知功能 ====================

  /**
   * 请求桌面通知权限
   */
  async function requestNotificationPermission(): Promise<boolean> {
    if (!('Notification' in window)) {
      logger.warn('SNMP-Trap: 浏览器不支持桌面通知')
      return false
    }

    try {
      const permission = await Notification.requestPermission()
      notificationPermission.value = permission
      logger.info(`SNMP-Trap: 桌面通知权限状态 - ${permission}`)
      return permission === 'granted'
    } catch (error) {
      logger.error('请求通知权限失败', 'SNMP-Trap', error)
      return false
    }
  }

  /**
   * 播放告警声音
   */
  function playAlertSound(severity: 'critical' | 'warning' | 'info'): void {
    if (!notificationSettings.value.soundEnabled) return

    // 根据严重级别检查是否启用对应声音
    if (severity === 'critical' && !notificationSettings.value.criticalSound) return
    if (severity === 'warning' && !notificationSettings.value.warningSound) return
    if (severity === 'info' && !notificationSettings.value.infoSound) return

    try {
      // 使用 Web Audio API 生成简单的提示音
      const audioContext = new (window.AudioContext || (window as any).webkitAudioContext)()
      const oscillator = audioContext.createOscillator()
      const gainNode = audioContext.createGain()

      oscillator.connect(gainNode)
      gainNode.connect(audioContext.destination)

      // 根据严重级别设置不同的频率
      const frequencies: Record<string, number> = {
        critical: 880, // A5
        warning: 660, // E5
        info: 440,    // A4
      }

      oscillator.frequency.value = frequencies[severity] || 440
      oscillator.type = 'sine'
      gainNode.gain.value = 0.3

      oscillator.start()
      oscillator.stop(audioContext.currentTime + 0.15)
    } catch (error) {
      // 忽略音频播放错误（可能是浏览器限制）
      logger.debug('播放声音失败', 'SNMP-Trap')
    }
  }

  /**
   * 发送桌面通知
   */
  function sendDesktopNotification(trap: TrapRecordVM): void {
    if (!notificationSettings.value.desktopEnabled) return
    if (notificationPermission.value !== 'granted') return

    try {
      const notification = new Notification(`SNMP Trap: ${trap.severity.toUpperCase()}`, {
        body: `${trap.sourceIP} - ${trap.trapName || trap.trapOID}`,
        icon: '/logo.ico',
        tag: `trap-${trap.id}`,
        requireInteraction: trap.severity === 'critical',
      })

      // 点击通知时聚焦窗口
      notification.onclick = () => {
        window.focus()
        notification.close()
      }

      // 5秒后自动关闭（非 critical 级别）
      if (trap.severity !== 'critical') {
        setTimeout(() => notification.close(), 5000)
      }
    } catch (error) {
      logger.debug('发送桌面通知失败', 'SNMP-Trap')
    }
  }

  /**
   * 处理 Trap 通知
   */
  function handleTrapNotification(trap: TrapRecordVM): void {
    const severity = trap.severity as 'critical' | 'warning' | 'info'
    playAlertSound(severity)
    sendDesktopNotification(trap)
  }

  /**
   * 更新通知设置
   */
  function updateNotificationSettings(settings: Partial<NotificationSettings>): void {
    notificationSettings.value = { ...notificationSettings.value, ...settings }
    logger.info('通知设置已更新', 'SNMP-Trap')
  }

  // ==================== 生命周期 ====================

  onMounted(() => {
    startListening()
    refreshStatus()
    refreshStats()

    // 检查通知权限状态
    if ('Notification' in window) {
      notificationPermission.value = Notification.permission
    }
  })

  onUnmounted(() => {
    stopListening()
  })

  return {
    // 状态
    latestTraps,
    listenerStatus,
    trapStats,
    isConnected,
    newTrapIds,
    statusLoading,
    unacknowledgedCount,
    notificationSettings,
    notificationPermission,
    hasNotificationPermission,

    // 方法
    startListening,
    stopListening,
    refreshStatus,
    refreshStats,
    acknowledgeTrap,
    batchAcknowledge,
    isNewTrap,
    clearLatestTraps,
    addHighlight,
    requestNotificationPermission,
    playAlertSound,
    sendDesktopNotification,
    handleTrapNotification,
    updateNotificationSettings,
  }
}
