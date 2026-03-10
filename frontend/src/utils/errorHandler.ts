/**
 * 统一错误处理工具
 * @description 提供类型安全的 API 调用包装和错误处理
 */

/** 错误处理选项 */
export interface ErrorHandlerOptions {
  /** 是否显示 Toast 提示 */
  showToast?: boolean
  /** 自定义错误消息 */
  customMessage?: string
  /** 错误回调 */
  onError?: (error: Error, message: string) => void
  /** 成功回调 */
  onSuccess?: (result: unknown) => void
}

/** API 调用结果 */
export interface CallResult<T> {
  /** 是否成功 */
  success: boolean
  /** 结果数据 */
  data?: T
  /** 错误消息 */
  error?: string
}

/**
 * 解析错误消息
 * @param error 未知错误对象
 * @returns 格式化的错误消息
 */
function parseErrorMessage(error: unknown): string {
  if (error instanceof Error) {
    return error.message
  }
  if (typeof error === 'string') {
    return error
  }
  if (typeof error === 'object' && error !== null) {
    if ('message' in error) {
      return String((error as Record<string, unknown>).message)
    }
    if ('error' in error) {
      return String((error as Record<string, unknown>).error)
    }
  }
  return '未知错误'
}

/**
 * 安全调用 API
 * @description 包装异步函数，提供统一的错误处理
 * 
 * @example
 * ```ts
 * const result = await safeCall(
 *   () => DeviceAPI.listDevices(),
 *   { customMessage: '获取设备列表失败' }
 * )
 * if (result.success) {
 *   console.log(result.data)
 * } else {
 *   console.log(result.error)
 * }
 * ```
 */
export async function safeCall<T>(
  fn: () => Promise<T>,
  options: ErrorHandlerOptions = {}
): Promise<CallResult<T>> {
  const { showToast = false, customMessage, onError, onSuccess } = options

  try {
    const result = await fn()
    
    if (onSuccess) {
      onSuccess(result)
    }
    
    return {
      success: true,
      data: result,
    }
  } catch (err: unknown) {
    const errorMessage = customMessage 
      ? `${customMessage}: ${parseErrorMessage(err)}`
      : parseErrorMessage(err)
    
    if (showToast) {
      // 动态导入 Toast 服务避免循环依赖
      import('./useToast').then(({ useToast }) => {
        useToast().error(errorMessage)
      })
    }
    
    if (onError) {
      onError(err instanceof Error ? err : new Error(String(err)), errorMessage)
    }
    
    console.error('[API Error]', errorMessage, err)
    
    return {
      success: false,
      error: errorMessage,
    }
  }
}

/**
 * 创建带默认选项的调用器
 * @description 工厂函数，创建预设错误处理行为的调用器
 * 
 * @example
 * ```ts
 * // 创建默认显示 Toast 的调用器
 * const callWithToast = createCaller({ showToast: true })
 * const result = await callWithToast(() => DeviceAPI.addDevice(device))
 * ```
 */
export function createCaller(defaultOptions: ErrorHandlerOptions) {
  return <T>(fn: () => Promise<T>, options?: ErrorHandlerOptions): Promise<CallResult<T>> =>
    safeCall(fn, { ...defaultOptions, ...options })
}
