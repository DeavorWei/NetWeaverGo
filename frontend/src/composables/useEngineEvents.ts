import { Events } from '@wailsio/runtime'
import { onMounted, onUnmounted } from 'vue'
import type { DeviceEvent, SuspendRequiredEvent, SuspendTimeoutEvent, EngineFinishedEvent, EventName } from '../types/events'
import { EventNames } from '../types/events'

/** 事件回调接口 */
export interface EngineEventCallbacks {
  /** 引擎执行完成回调 */
  onFinished?: () => void
  /** 设备事件回调 */
  onDeviceEvent?: (data: DeviceEvent) => void
  /** 批量设备事件回调（用于优化性能） */
  onBatchDeviceEvents?: (events: DeviceEvent[]) => void
  /** 挂起请求回调 */
  onSuspend?: (data: SuspendRequiredEvent) => void
  /** 挂起超时回调 */
  onSuspendTimeout?: (data: SuspendTimeoutEvent) => void
}

/** 事件监听器清理函数 */
type CleanupFn = () => void

/**
 * 批量事件处理器
 * 用于合并高频事件，减少 Vue 更新次数
 * 支持 requestAnimationFrame 和 setTimeout 两种刷新模式
 */
export class BatchEventProcessor<T> {
  private buffer: T[] = []
  private timer: ReturnType<typeof setTimeout> | null = null
  private rafId: number | null = null
  private readonly batchSize: number
  private readonly flushInterval: number
  private readonly onFlush: (events: T[]) => void
  private destroyed = false
  private readonly useRaf: boolean

  constructor(
    onFlush: (events: T[]) => void,
    options: { batchSize?: number; flushInterval?: number; useRaf?: boolean } = {}
  ) {
    this.onFlush = onFlush
    this.batchSize = options.batchSize ?? 50
    this.flushInterval = options.flushInterval ?? 100
    this.useRaf = options.useRaf ?? false
  }

  /** 添加事件到缓冲区 */
  push(event: T): void {
    if (this.destroyed) {
      console.warn('[BatchEventProcessor] 已销毁，事件被丢弃')
      return
    }
    
    this.buffer.push(event)

    if (this.buffer.length >= this.batchSize) {
      this.flush()
    } else if (!this.timer && !this.rafId) {
      if (this.useRaf && typeof requestAnimationFrame !== 'undefined') {
        this.rafId = requestAnimationFrame(() => this.flush())
      } else {
        this.timer = setTimeout(() => this.flush(), this.flushInterval)
      }
    }
  }

  /** 刷新缓冲区，处理所有待处理的事件 */
  flush(): void {
    // 清理定时器
    if (this.timer) {
      clearTimeout(this.timer)
      this.timer = null
    }
    if (this.rafId) {
      cancelAnimationFrame(this.rafId)
      this.rafId = null
    }

    if (this.buffer.length > 0) {
      const events = this.buffer.splice(0, this.buffer.length)
      try {
        this.onFlush(events)
      } catch (e) {
        console.warn('[BatchEventProcessor] flush error:', e)
      }
    }
  }

  /** 销毁处理器，先刷新剩余事件再标记销毁 */
  destroy(): void {
    // 先刷新剩余事件
    this.flush()
    // 再标记销毁
    this.destroyed = true
  }

  /** 获取当前缓冲区大小 */
  get size(): number {
    return this.buffer.length
  }
}

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
  let deviceEventProcessor: BatchEventProcessor<DeviceEvent> | null = null

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
      }, false)  // 明确指定此事件不需要数据载荷
    }

    // 订阅设备事件
    if (callbacks.onDeviceEvent || callbacks.onBatchDeviceEvents) {
      // 如果提供了批量处理回调，使用批处理器
      if (callbacks.onBatchDeviceEvents) {
        // 使用 requestAnimationFrame 优化 UI 更新性能
        deviceEventProcessor = new BatchEventProcessor<DeviceEvent>(
          callbacks.onBatchDeviceEvents,
          { batchSize: 30, flushInterval: 50, useRaf: true }
        )

        safeSubscribe<DeviceEvent>(EventNames.DEVICE_EVENT, (data) => {
          deviceEventProcessor!.push(data)
        })
      } else if (callbacks.onDeviceEvent) {
        // 否则使用单个事件处理
        safeSubscribe<DeviceEvent>(EventNames.DEVICE_EVENT, (data) => {
          callbacks.onDeviceEvent!(data)
        })
      }
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
    // 先销毁批处理器（会自动 flush 剩余事件）
    if (deviceEventProcessor) {
      deviceEventProcessor.destroy()
      deviceEventProcessor = null
    }
    
    // 清理所有事件监听器
    cleanupFns.forEach((fn) => {
      try {
        fn()
      } catch (e) {
        console.warn('[useEngineEvents] 清理事件监听器时发生警告:', e)
      }
    })
    // 清空清理函数数组
    cleanupFns.length = 0
  })

  return {
    /** 手动清理所有事件监听器 */
    cleanupAll: () => {
      if (deviceEventProcessor) {
        deviceEventProcessor.destroy()
        deviceEventProcessor = null
      }
      cleanupFns.forEach(fn => fn())
    },
    /** 手动刷新设备事件缓冲区 */
    flushDeviceEvents: () => {
      if (deviceEventProcessor) {
        deviceEventProcessor.flush()
      }
    },
    /** 获取当前缓冲区大小 */
    getBufferSize: () => deviceEventProcessor?.size ?? 0,
  }
}

/**
 * 创建独立的批量事件处理器
 * 用于需要在组件外部使用的场景
 */
export function createBatchEventProcessor<T>(
  onFlush: (events: T[]) => void,
  options?: { batchSize?: number; flushInterval?: number }
): BatchEventProcessor<T> {
  return new BatchEventProcessor(onFlush, options)
}
