/**
 * 统一任务执行运行时 Composable
 * 用于监听和处理统一运行时的事件
 */

import { ref, computed, onMounted, onUnmounted } from 'vue'
import { Events } from '@wailsio/runtime'
import type { ExecutionSnapshot } from '../types/taskexec'

export interface TaskExecutionState {
  snapshot: ExecutionSnapshot | null
  isRunning: boolean
  error: string | null
}

export function useTaskExecution() {
  const state = ref<TaskExecutionState>({
    snapshot: null,
    isRunning: false,
    error: null
  })

  let cleanupFns: (() => void)[] = []

  // 解析事件数据
  function unwrapEventData<T = any>(ev: any): T | null {
    if (!ev) return null
    if (Array.isArray(ev.data)) {
      return (ev.data[0] ?? null) as T | null
    }
    return (ev.data ?? null) as T | null
  }

  // 应用快照
  function applySnapshot(snapshot: ExecutionSnapshot | null) {
    state.value.snapshot = snapshot
    state.value.isRunning = snapshot?.status === 'running' || snapshot?.status === 'pending'
  }

  // 标记执行完成
  function markExecutionFinished() {
    if (state.value.snapshot) {
      state.value.isRunning = false
    }
  }

  // 初始化监听器
  function initListeners() {
    cleanupListeners()

    // 监听快照更新
    const unlistenSnapshot = Events.On('execution:snapshot', (ev: any) => {
      const data = unwrapEventData<ExecutionSnapshot>(ev)
      if (data) {
        applySnapshot(data)
      }
    })
    if (typeof unlistenSnapshot === 'function') {
      cleanupFns.push(unlistenSnapshot)
    }

    // 监听执行完成
    const unlistenFinished = Events.On('engine:finished', () => {
      markExecutionFinished()
    })
    if (typeof unlistenFinished === 'function') {
      cleanupFns.push(unlistenFinished)
    }

    // 监听执行事件 (新运行时事件)
    const unlistenTaskEvent = Events.On('task:event', (ev: any) => {
      const data = unwrapEventData(ev)
      if (data) {
        console.log('[TaskExecution] Event:', data)
      }
    })
    if (typeof unlistenTaskEvent === 'function') {
      cleanupFns.push(unlistenTaskEvent)
    }
  }

  // 清理监听器
  function cleanupListeners() {
    cleanupFns.forEach(fn => fn())
    cleanupFns = []
  }

  // 计算属性
  const stages = computed(() => state.value.snapshot?.stages ?? [])
  const units = computed(() => state.value.snapshot?.units ?? [])
  const currentStage = computed(() => 
    stages.value.find(s => s.status === 'running')
  )
  const progress = computed(() => state.value.snapshot?.progress ?? 0)

  // 生命周期
  onMounted(() => {
    initListeners()
  })

  onUnmounted(() => {
    cleanupListeners()
  })

  return {
    // State
    state,
    snapshot: computed(() => state.value.snapshot),
    isRunning: computed(() => state.value.isRunning),
    error: computed(() => state.value.error),
    
    // Computed
    stages,
    units,
    currentStage,
    progress,
    
    // Methods
    applySnapshot,
    markExecutionFinished,
    initListeners,
    cleanupListeners
  }
}
