/**
 * 可取消的 API 调用 Composable
 * @description 自动管理 API 请求的取消，组件卸载时自动取消未完成的请求
 */
import { ref, onUnmounted, computed, type Ref, readonly } from 'vue'

/** 可取消的 Promise 类型（Wails3 绑定返回类型） */
interface CancellablePromise<T> extends Promise<T> {
  cancel(): void
}

/** 调用状态 */
export interface CallState<T> {
  /** 是否正在加载 */
  loading: boolean
  /** 是否已取消 */
  cancelled: boolean
  /** 错误信息 */
  error: string | null
  /** 结果数据 */
  data: T | null
}

/**
 * 创建可取消的 API 调用器
 * @description 包装 Wails3 绑定函数，提供自动取消功能
 * 
 * @example
 * ```ts
 * const { execute, loading, data, cancel } = useCancellable(DeviceAPI.listDevices)
 * 
 * onMounted(() => execute())
 * // 组件卸载时自动取消未完成的请求
 * ```
 */
export function useCancellable<T, Args extends unknown[]>(
  fn: (...args: Args) => CancellablePromise<T>
) {
  const state: Ref<CallState<T>> = ref({
    loading: false,
    cancelled: false,
    error: null,
    data: null,
  })
  
  let currentPromise: CancellablePromise<T> | null = null

  /**
   * 执行调用
   * @param args 函数参数
   * @returns 结果数据或 null（取消/出错时）
   */
  async function execute(...args: Args): Promise<T | null> {
    // 取消之前的请求
    cancel()
    
    state.value = {
      loading: true,
      cancelled: false,
      error: null,
      data: null,
    }
    
    try {
      currentPromise = fn(...args)
      const result = await currentPromise
      
      state.value.loading = false
      state.value.data = result
      
      return result
    } catch (err: unknown) {
      // 如果是取消导致的错误，不记录
      if (state.value.cancelled) {
        return null
      }
      
      state.value.loading = false
      state.value.error = err instanceof Error ? err.message : String(err)
      
      return null
    } finally {
      currentPromise = null
    }
  }

  /**
   * 取消当前请求
   */
  function cancel(): void {
    if (currentPromise && typeof currentPromise.cancel === 'function') {
      state.value.cancelled = true
      currentPromise.cancel()
      currentPromise = null
    }
  }
  
  // 组件卸载时自动取消
  onUnmounted(() => {
    cancel()
  })

  return {
    /** 执行调用 */
    execute,
    /** 取消调用 */
    cancel,
    /** 调用状态（只读） */
    state: readonly(state) as Readonly<Ref<CallState<T>>>,
    /** 是否正在加载 */
    loading: computed(() => state.value.loading),
    /** 结果数据 */
    data: computed(() => state.value.data),
    /** 错误信息 */
    error: computed(() => state.value.error),
    /** 是否已取消 */
    cancelled: computed(() => state.value.cancelled),
  }
}

/**
 * 批量可取消调用管理器
 * @description 管理多个并发请求的取消
 * 
 * @example
 * ```ts
 * const { track, cancelAll } = useCancellablePool()
 * 
 * // 跟踪请求
 * const promise = track(DeviceAPI.listDevices())
 * 
 * // 取消所有请求
 * cancelAll()
 * ```
 */
export function useCancellablePool() {
  const promises = new Set<CancellablePromise<unknown>>()

  /**
   * 跟踪一个可取消的 Promise
   * @param promise 可取消的 Promise
   * @returns 同一个 Promise
   */
  function track<T>(promise: CancellablePromise<T>): CancellablePromise<T> {
    promises.add(promise)
    
    promise.finally(() => {
      promises.delete(promise)
    })
    
    return promise
  }

  /**
   * 取消所有请求
   */
  function cancelAll(): void {
    promises.forEach(p => {
      if (typeof p.cancel === 'function') {
        p.cancel()
      }
    })
    promises.clear()
  }
  
  onUnmounted(() => {
    cancelAll()
  })

  return {
    /** 跟踪请求 */
    track,
    /** 取消所有请求 */
    cancelAll,
    /** 当前请求数量 */
    size: () => promises.size,
  }
}
