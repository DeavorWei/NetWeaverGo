/**
 * Wails3 事件系统类型定义
 * @description 定义后端发射到前端的所有事件数据结构
 */

/** 事件类型枚举 */
export type EventType = 'start' | 'cmd' | 'success' | 'error' | 'skip' | 'abort'

/** 设备执行事件 - 与后端 ExecutorEvent 结构保持一致 */
export interface DeviceEvent {
  /** 设备 IP 地址 */
  IP: string
  /** 事件类型 */
  Type: EventType
  /** 事件消息内容（已脱敏处理） */
  Message: string
  /** 当前命令处于组中的索引 (1-based) */
  CmdIndex: number
  /** 组内总命令数量 */
  TotalCmd: number
}

/** 挂起请求事件 */
export interface SuspendRequiredEvent {
  /** 会话 ID（用于唯一标识挂起会话，防止重复挂起时的信号泄漏） */
  sessionId?: string
  /** 设备 IP */
  ip: string
  /** 错误详情 */
  error: string
  /** 当前执行的命令 */
  command: string
}

/** 引擎完成事件（无数据载荷） */
export type EngineFinishedEvent = void

/** 挂起超时事件 */
export interface SuspendTimeoutEvent {
  /** 会话 ID */
  sessionId: string
  /** 设备 IP */
  ip: string
  /** 超时消息 */
  message: string
}

/** 事件名称常量 */
export const EventNames = {
  /** 引擎执行完成 */
  ENGINE_FINISHED: 'engine:finished',
  /** 设备执行事件 */
  DEVICE_EVENT: 'device:event',
  /** 挂起请求 */
  SUSPEND_REQUIRED: 'engine:suspend_required',
  /** 挂起超时 */
  SUSPEND_TIMEOUT: 'engine:suspend_timeout',
} as const

/** 事件名称类型 */
export type EventName = typeof EventNames[keyof typeof EventNames]

/** 挂起操作类型 */
export type SuspendAction = 'C' | 'S' | 'A'

/** 挂起操作描述 */
export const SuspendActionLabels: Record<SuspendAction, string> = {
  C: '继续发送下一条命令 (Continue)',
  S: '放弃此报错动作强行放行 (Skip)',
  A: '退下并切断此设备连接 (Abort)',
}
