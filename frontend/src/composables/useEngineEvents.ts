import { Events } from '@wailsio/runtime'
import { onMounted, onUnmounted } from 'vue'
import type { DeviceEvent, SuspendRequiredEvent, EngineFinishedEvent, EventName } from '../types/events'
import { EventNames } from '../types/events'

/** 事件回调接口 */
export interface EngineEventCallbacks {
  /** 引擎执行完成回调 */
  onFinished?: () => void
  /** 设备事件回调 */
  onDeviceEvent?: (data: DeviceEvent) => void
  /** 挂起请求回调 */
  onSuspend?: (data: SuspendRequiredEvent) => void
}

/** 事件监听器清理函数 */
type CleanupFn = () => void

/**
 * 统一管理引擎事件的 Composable
 * @description 提供类型安全的事件订阅和自动清理
 * @param callbacks 事件回调对象
 * @returns 清理函数和手动清理方法
 * 
 * @example
 * ```ts
 * useEngineEvents({
 *   onFinished: () => console.log('完成'),
 *   onDeviceEvent: (data) => console.log(data.IP, data.Message),
 *   onSuspend: (data) => showSuspendModal(data)
 * })
 * ```
 */
export function useEngineEvents(callbacks: EngineEventCallbacks) {
  const cleanupFns: CleanupFn[] = []

  /**
   * 安全订阅事件
   * @param eventName 事件名称
   * @param handler 事件处理器
   */
  function safeSubscribe<T>(
    eventName: EventName,
    handler: (data: T) => void
  ): void {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const result: any = Events.On(eventName, (ev: { data?: [T] }) => {
      const data = ev.data?.[0]
      if (data) {
        handler(data)
      }
    })
    
    // 根据返回值类型确定清理方式
    if (typeof result === 'function') {
      cleanupFns.push(result)
    } else if (result && typeof result.cancel === 'function') {
      cleanupFns.push(() => result.cancel())
    }
  }

  onMounted(() => {
    // 订阅引擎完成事件
    if (callbacks.onFinished) {
      safeSubscribe<EngineFinishedEvent>(EventNames.ENGINE_FINISHED, () => {
        callbacks.onFinished!()
      })
    }

    // 订阅设备事件
    if (callbacks.onDeviceEvent) {
      safeSubscribe<DeviceEvent>(EventNames.DEVICE_EVENT, (data) => {
        callbacks.onDeviceEvent!(data)
      })
    }

    // 订阅挂起请求
    if (callbacks.onSuspend) {
      safeSubscribe<SuspendRequiredEvent>(EventNames.SUSPEND_REQUIRED, (data) => {
        callbacks.onSuspend!(data)
      })
    }
  })

  onUnmounted(() => {
    cleanupFns.forEach((fn) => {
      try {
        fn()
      } catch (e) {
        console.warn('[useEngineEvents] 清理事件监听器时发生警告:', e)
      }
    })
  })

  return {
    /** 手动清理所有事件监听器 */
    cleanupAll: () => cleanupFns.forEach(fn => fn()),
  }
}
