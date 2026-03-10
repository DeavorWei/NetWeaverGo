/**
 * 全局 Toast 服务
 * @description 提供全局访问的 Toast 通知功能
 */
import { ref, type Ref, readonly } from 'vue'

export interface ToastState {
  visible: boolean
  message: string
  type: 'success' | 'error' | 'warning' | 'info'
}

const state = ref<ToastState>({
  visible: false,
  message: '',
  type: 'info',
})

let hideTimer: ReturnType<typeof setTimeout> | null = null

/**
 * Toast 服务 Hook
 * @description 提供全局 Toast 通知功能
 * 
 * @example
 * ```ts
 * const toast = useToast()
 * toast.success('操作成功')
 * toast.error('操作失败')
 * ```
 */
export function useToast() {
  /**
   * 显示 Toast
   * @param message 消息内容
   * @param type 类型
   * @param duration 持续时间（毫秒）
   */
  function show(message: string, type: ToastState['type'] = 'info', duration = 3000): void {
    if (hideTimer) {
      clearTimeout(hideTimer)
    }
    
    state.value = {
      visible: true,
      message,
      type,
    }
    
    hideTimer = setTimeout(() => {
      state.value.visible = false
    }, duration)
  }
  
  return {
    /** Toast 响应式状态（只读） */
    state: readonly(state) as Readonly<Ref<ToastState>>,
    /** 显示成功提示 */
    success: (message: string, duration?: number) => show(message, 'success', duration),
    /** 显示错误提示 */
    error: (message: string, duration?: number) => show(message, 'error', duration ?? 5000),
    /** 显示警告提示 */
    warning: (message: string, duration?: number) => show(message, 'warning', duration),
    /** 显示信息提示 */
    info: (message: string, duration?: number) => show(message, 'info', duration),
    /** 手动隐藏 */
    hide: (): void => {
      state.value.visible = false
      if (hideTimer) {
        clearTimeout(hideTimer)
        hideTimer = null
      }
    },
  }
}
