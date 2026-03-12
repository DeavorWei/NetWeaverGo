import { Events } from '@wailsio/runtime'
import { onMounted, onUnmounted } from 'vue'
import type { SuspendRequiredEvent, SuspendTimeoutEvent, EngineFinishedEvent, EventName, ExecutionSnapshot } from '../types/events'
import { EventNames } from '../types/events'

/** 事件回调接口 - 简化版 */
export interface EngineEventCallbacks {
  /** 引擎执行完成回调 */
  onFinished?: () => void
  /** 执行快照回调（200ms 定时推送） */
  onSnapshot?: (snapshot: ExecutionSnapshot) => void
  /** 挂起请求回调 */
  onSuspend?: (data: SuspendRequiredEvent) => void
  /** 挂起超时回调 */
  onSuspendTimeout?: (data: SuspendTimeoutEvent) => void
}

/** 事件监听器清理函数 */
type CleanupFn = () => void

/**
 * 统一管理引擎事件的 Composable（简化版）
 * @description 使用快照机制替代批量事件处理，前端只订阅控制类事件
 * @param callbacks 事件回调对象
 * @returns 清理函数
 * 
 * @example
 * ```ts
 * useEngineEvents({
 *   onFinished: () => console.log('完成'),
 *   onSnapshot: (snapshot) => updateUI(snapshot),
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
   * @param requiresData 是否需要数据载荷（默认 true）
   */
  function safeSubscribe<T>(
    eventName: EventName,
    handler: (data: T) => void,
    requiresData: boolean = true
  ): void {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const result: any = Events.On(eventName, (ev: { data?: [T] }) => {
      // 对于不需要数据载荷的事件（如 engine:finished），直接调用处理器
      if (!requiresData) {
        handler(undefined as T)
        return
      }
      // 对于需要数据载荷的事件，检查数据是否存在
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
    // 订阅引擎完成事件（此事件无数据载荷）
    if (callbacks.onFinished) {
      safeSubscribe<EngineFinishedEvent>(EventNames.ENGINE_FINISHED, () => {
        callbacks.onFinished!()
      }, false)
    }

    // 订阅执行快照（200ms 定时推送）
    if (callbacks.onSnapshot) {
      safeSubscribe<ExecutionSnapshot>(EventNames.EXECUTION_SNAPSHOT, (data) => {
        callbacks.onSnapshot!(data)
      })
    }

    // 订阅挂起请求
    if (callbacks.onSuspend) {
      safeSubscribe<SuspendRequiredEvent>(EventNames.SUSPEND_REQUIRED, (data) => {
        callbacks.onSuspend!(data)
      })
    }

    // 订阅挂起超时
    if (callbacks.onSuspendTimeout) {
      safeSubscribe<SuspendTimeoutEvent>(EventNames.SUSPEND_TIMEOUT, (data) => {
        callbacks.onSuspendTimeout!(data)
      })
    }
  })

  onUnmounted(() => {
    // 清理所有事件监听器
    cleanupFns.forEach((fn) => {
      try {
        fn()
      } catch (e) {
        console.warn('[useEngineEvents] 清理事件监听器时发生警告:', e)
      }
    })
    cleanupFns.length = 0
  })

  return {
    /** 手动清理所有事件监听器 */
    cleanupAll: () => {
      cleanupFns.forEach(fn => fn())
      cleanupFns.length = 0
    },
  }
}
