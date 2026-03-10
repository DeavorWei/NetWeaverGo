/**
 * Wails3 事件系统类型定义
 * @description 定义后端发射到前端的所有事件数据结构
 */

/** 设备执行事件 */
export interface DeviceEvent {
  /** 设备 IP 地址 */
  IP: string
  /** 事件类型: start | success | error | skip | abort */
  Type: 'start' | 'success' | 'error' | 'skip' | 'abort'
  /** 事件消息内容 */
  Message: string
  /** 时间戳（可选） */
  Timestamp?: string
}

/** 挂起请求事件 */
export interface SuspendRequiredEvent {
  /** 设备 IP */
  ip: string
  /** 错误详情 */
  error: string
  /** 当前执行的命令 */
  command: string
}

/** 引擎完成事件（无数据载荷） */
export type EngineFinishedEvent = void

/** 事件名称常量 */
export const EventNames = {
  /** 引擎执行完成 */
  ENGINE_FINISHED: 'engine:finished',
  /** 设备执行事件 */
  DEVICE_EVENT: 'device:event',
  /** 挂起请求 */
  SUSPEND_REQUIRED: 'engine:suspend_required',
} as const

/** 事件名称类型 */
export type EventName = typeof EventNames[keyof typeof EventNames]
