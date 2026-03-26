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
import type { ExecutionSnapshot, SnapshotDelta } from '../types/taskexec'

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
    const current = snapshots.value[runId]
    const currentSeq = current?.lastRunSeq ?? current?.revision ?? 0
    const nextSeq = snapshot.lastRunSeq ?? snapshot.revision ?? 0
    if (current && nextSeq < currentSeq) {
      return
    }
    snapshots.value[runId] = markRaw({ ...snapshot })
  }

  function applySnapshotDelta(delta: SnapshotDelta) {
    if (!delta?.runId) return
    const current = snapshots.value[delta.runId]
    const currentSeq = current?.lastRunSeq ?? current?.revision ?? 0
    if (delta.seq <= currentSeq) {
      return
    }
    if (delta.snapshot) {
      updateSnapshot(delta.runId, delta.snapshot)
    }
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
    
    // 监听后端直接推送的快照增量
    const unlistenSnapshotDelta = Events.On('task:snapshot_delta', (ev: any) => {
      const data = unwrapEventData<SnapshotDelta>(ev)
      if (data?.runId) {
        applySnapshotDelta(data)
      }
    })
    if (typeof unlistenSnapshotDelta === 'function') {
      cleanupFns.push(unlistenSnapshotDelta)
    }

    // 兼容后端仍会推送的全量快照数据
    const unlistenSnapshotData = Events.On('task:snapshot_data', (ev: any) => {
      const data = unwrapEventData<ExecutionSnapshot>(ev)
      if (data?.runId) {
        updateSnapshot(data.runId, data)
      }
    })
    if (typeof unlistenSnapshotData === 'function') {
      cleanupFns.push(unlistenSnapshotData)
    }
    
    // 监听任务开始
    const unlistenStarted = Events.On('task:started', (ev: any) => {
      const data = unwrapEventData<TaskEvent>(ev)
      if (data) {
        addEventLog(data)
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
    
    // 兼容旧桥接事件，保留监听但不再触发全量刷新。
    const unlistenSnapshotCompat = Events.On('task:snapshot', () => {})
    if (typeof unlistenSnapshotCompat === 'function') {
      cleanupFns.push(unlistenSnapshotCompat)
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
    applySnapshotDelta,
    initListeners,
    cleanupListeners,
    refreshSnapshot,
    loadRunHistory,
    cancelTask,
  }
})
