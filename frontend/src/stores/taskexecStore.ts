/**
 * 统一任务执行运行时 Store (Pinia)
 * 
 * 用于管理统一执行框架 (taskexec) 的状态
 * 这是前端唯一的执行态真相源
 */

import { defineStore } from 'pinia'
import { ref, computed, markRaw } from 'vue'
import { Events } from '@wailsio/runtime'
import { TaskExecutionAPI } from '../services/api'
import type { ExecutionSnapshot } from '../types/taskexec'

// 前端事件格式
export interface TaskEvent {
  id: string
  runId: string
  stageId?: string
  unitId?: string
  type: string
  level: 'info' | 'warn' | 'error'
  message: string
  payload?: Record<string, any>
  timestamp: number
}

// 运行摘要
export interface RunSummary {
  runId: string
  taskGroupId: number
  taskName: string
  taskNameSnapshot: string
  runKind: 'normal' | 'topology'
  status: string
  progress: number
  totalStages: number
  completedStages: number
  totalUnits: number
  successUnits: number
  failedUnits: number
  startedAt: string
  finishedAt: string
  durationMs: number
}

export const useTaskexecStore = defineStore('taskexec', () => {
  // ================== 状态 ==================
  
  // 当前运行的快照（按runId索引）
  const snapshots = ref<Record<string, ExecutionSnapshot>>({})
  
  // 当前关注的runId
  const currentRunId = ref<string | null>(null)
  
  // 运行历史列表
  const runHistory = ref<RunSummary[]>([])
  
  // 事件日志
  const eventLogs = ref<TaskEvent[]>([])
  
  // 清理函数列表
  let cleanupFns: (() => void)[] = []

  // ================== 计算属性 ==================
  
  // 当前快照
  const currentSnapshot = computed<ExecutionSnapshot | null>(() => {
    if (!currentRunId.value) return null
    return snapshots.value[currentRunId.value] ?? null
  })
  
  // 是否正在运行
  const isRunning = computed(() => {
    const snapshot = currentSnapshot.value
    return snapshot?.status === 'running' || snapshot?.status === 'pending'
  })
  
  // 当前运行的任务列表
  const runningRuns = computed(() => {
    return Object.values(snapshots.value).filter(
      s => s.status === 'running' || s.status === 'pending'
    )
  })
  
  // 进度百分比
  const progress = computed(() => currentSnapshot.value?.progress ?? 0)
  
  // 当前stage
  const currentStage = computed(() => {
    const snapshot = currentSnapshot.value
    if (!snapshot?.stages) return null
    return snapshot.stages.find(s => s.status === 'running')
  })

  // ================== 辅助函数 ==================
  
  function unwrapEventData<T = any>(ev: any): T | null {
    if (!ev) return null
    const raw = Array.isArray(ev.data) ? (ev.data[0] ?? null) : (ev.data ?? null)
    if (typeof raw === 'string') {
      try {
        return JSON.parse(raw) as T
      } catch {
        return null
      }
    }
    return raw as T | null
  }
  
  // ================== 状态更新 ==================
  
  function setCurrentRunId(runId: string | null) {
    currentRunId.value = runId
  }
  
  function updateSnapshot(runId: string, snapshot: ExecutionSnapshot) {
    snapshots.value[runId] = markRaw({ ...snapshot })
  }
  
  function removeSnapshot(runId: string) {
    delete snapshots.value[runId]
  }
  
  function addEventLog(event: TaskEvent) {
    eventLogs.value.push(event)
    // 限制日志数量，防止内存溢出
    if (eventLogs.value.length > 1000) {
      eventLogs.value = eventLogs.value.slice(-500)
    }
  }
  
  function clearEventLogs() {
    eventLogs.value = []
  }
  
  function setRunHistory(runs: RunSummary[]) {
    runHistory.value = runs
  }

  // ================== 事件监听 ==================
  
  function initListeners() {
    cleanupListeners()
    
    // 监听任务快照更新
    const unlistenSnapshot = Events.On('task:snapshot', (ev: any) => {
      const data = unwrapEventData<{ runId: string; timestamp: number; eventType: string }>(ev)
      if (data?.runId) {
        // 触发快照刷新
        refreshSnapshot(data.runId)
      }
    })
    if (typeof unlistenSnapshot === 'function') {
      cleanupFns.push(unlistenSnapshot)
    }
    
    // 监听任务开始
    const unlistenStarted = Events.On('task:started', (ev: any) => {
      const data = unwrapEventData<TaskEvent>(ev)
      if (data) {
        addEventLog(data)
        // 如果是当前关注的run，刷新快照
        if (data.runId === currentRunId.value) {
          refreshSnapshot(data.runId)
        }
      }
    })
    if (typeof unlistenStarted === 'function') {
      cleanupFns.push(unlistenStarted)
    }
    
    // 监听任务完成
    const unlistenFinished = Events.On('task:finished', (ev: any) => {
      const data = unwrapEventData<TaskEvent>(ev)
      if (data) {
        addEventLog(data)
        refreshSnapshot(data.runId)
      }
    })
    if (typeof unlistenFinished === 'function') {
      cleanupFns.push(unlistenFinished)
    }
    
    // 监听通用任务事件
    const unlistenTaskEvent = Events.On('task:event', (ev: any) => {
      const data = unwrapEventData<TaskEvent>(ev)
      if (data) {
        addEventLog(data)
        console.log('[TaskexecStore] Event:', data)
      }
    })
    if (typeof unlistenTaskEvent === 'function') {
      cleanupFns.push(unlistenTaskEvent)
    }
    
    // 监听Stage更新
    const unlistenStage = Events.On('task:stage_updated', (ev: any) => {
      const data = unwrapEventData<TaskEvent>(ev)
      if (data?.runId) {
        refreshSnapshot(data.runId)
      }
    })
    if (typeof unlistenStage === 'function') {
      cleanupFns.push(unlistenStage)
    }
    
    // 监听Unit更新
    const unlistenUnit = Events.On('task:unit_updated', (ev: any) => {
      const data = unwrapEventData<TaskEvent>(ev)
      if (data?.runId) {
        refreshSnapshot(data.runId)
      }
    })
    if (typeof unlistenUnit === 'function') {
      cleanupFns.push(unlistenUnit)
    }
  }
  
  function cleanupListeners() {
    cleanupFns.forEach(fn => {
      try {
        fn()
      } catch (e) {
        console.warn('清理事件监听器时发生警告:', e)
      }
    })
    cleanupFns = []
  }

  // ================== API 调用 ==================
  
  async function refreshSnapshot(runId: string): Promise<ExecutionSnapshot | null> {
    try {
      const snapshot = await TaskExecutionAPI.getTaskSnapshot(runId)
      if (snapshot) {
        updateSnapshot(runId, snapshot)
      }
      return snapshot
    } catch (err) {
      console.error('Failed to refresh snapshot:', err)
      return null
    }
  }
  
  async function loadRunHistory(limit: number = 50): Promise<RunSummary[]> {
    try {
      const runs = await TaskExecutionAPI.listTaskRuns(limit)
      setRunHistory(runs)
      return runs
    } catch (err) {
      console.error('Failed to load run history:', err)
      return []
    }
  }
  
  async function cancelTask(runId: string): Promise<boolean> {
    try {
      await TaskExecutionAPI.cancelTask(runId)
      return true
    } catch (err) {
      console.error('Failed to cancel task:', err)
      return false
    }
  }

  // ================== 导出 ==================
  return {
    // 状态
    snapshots,
    currentRunId,
    runHistory,
    eventLogs,
    
    // 计算属性
    currentSnapshot,
    isRunning,
    runningRuns,
    progress,
    currentStage,
    
    // 方法
    setCurrentRunId,
    updateSnapshot,
    removeSnapshot,
    addEventLog,
    clearEventLogs,
    setRunHistory,
    initListeners,
    cleanupListeners,
    refreshSnapshot,
    loadRunHistory,
    cancelTask,
  }
})
