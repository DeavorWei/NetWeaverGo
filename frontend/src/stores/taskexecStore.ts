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
import type { ExecutionSnapshot, SnapshotDelta, SnapshotDeltaOp } from '../types/taskexec'

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

  function cloneSnapshot(snapshot: ExecutionSnapshot): ExecutionSnapshot {
    return {
      ...snapshot,
      stages: (snapshot.stages ?? []).map(stage => ({ ...stage })),
      units: (snapshot.units ?? []).map(unit => ({
        ...unit,
        logs: unit.logs ? [...unit.logs] : undefined,
      })),
      events: (snapshot.events ?? []).map(event => ({ ...event })),
      lastSessionSeqByUnit: snapshot.lastSessionSeqByUnit
        ? { ...snapshot.lastSessionSeqByUnit }
        : undefined,
    }
  }

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
    snapshots.value[runId] = markRaw(cloneSnapshot(snapshot))
  }

  function sortStages(stages: ExecutionSnapshot['stages']) {
    stages.sort((a, b) => {
      if (a.order === b.order) {
        return a.id.localeCompare(b.id)
      }
      return a.order - b.order
    })
  }

  function upsertStage(snapshot: ExecutionSnapshot, stage: NonNullable<SnapshotDeltaOp['stage']>) {
    const idx = snapshot.stages.findIndex(item => item.id === stage.id)
    if (idx >= 0) {
      snapshot.stages[idx] = { ...stage }
    } else {
      snapshot.stages.push({ ...stage })
    }
    sortStages(snapshot.stages)
  }

  function upsertUnit(snapshot: ExecutionSnapshot, unit: NonNullable<SnapshotDeltaOp['unit']>) {
    const idx = snapshot.units.findIndex(item => item.id === unit.id)
    if (idx >= 0) {
      snapshot.units[idx] = {
        ...unit,
        logs: unit.logs ? [...unit.logs] : undefined,
      }
      return
    }
    snapshot.units.push({
      ...unit,
      logs: unit.logs ? [...unit.logs] : undefined,
    })
  }

  function appendEvent(snapshot: ExecutionSnapshot, event: NonNullable<SnapshotDeltaOp['event']>) {
    snapshot.events = snapshot.events.filter(item => item.id !== event.id)
    snapshot.events.unshift({ ...event })
    if (snapshot.events.length > 50) {
      snapshot.events = snapshot.events.slice(0, 50)
    }
  }

  function applyDeltaOps(snapshot: ExecutionSnapshot, ops: SnapshotDeltaOp[]) {
    for (const op of ops) {
      switch (op.type) {
        case 'run_patch':
          if (op.status !== undefined) snapshot.status = op.status
          if (op.currentStage !== undefined) snapshot.currentStage = op.currentStage
          if (op.progress !== undefined) snapshot.progress = op.progress
          if ('startedAt' in op) snapshot.startedAt = op.startedAt ?? null
          if ('finishedAt' in op) snapshot.finishedAt = op.finishedAt ?? null
          break
        case 'stage_upsert':
          if (op.stage) upsertStage(snapshot, op.stage)
          break
        case 'unit_upsert':
          if (op.unit) upsertUnit(snapshot, op.unit)
          break
        case 'event_append':
          if (op.event) appendEvent(snapshot, op.event)
          break
        default:
          break
      }
    }
  }

  function applySnapshotDelta(delta: SnapshotDelta): boolean {
    if (!delta?.runId) return false
    const current = snapshots.value[delta.runId]
    const currentSeq = current?.lastRunSeq ?? current?.revision ?? 0
    if (delta.seq <= currentSeq) {
      return false
    }

    if (delta.snapshot) {
      updateSnapshot(delta.runId, delta.snapshot)
      return true
    }

    if (!current) {
      return false
    }

    if ((delta.baseSeq ?? 0) !== currentSeq) {
      return false
    }

    const nextSnapshot = cloneSnapshot(current)
    applyDeltaOps(nextSnapshot, delta.ops ?? [])
    nextSnapshot.lastRunSeq = delta.seq
    nextSnapshot.revision = delta.revision
    nextSnapshot.updatedAt = delta.updatedAt
    snapshots.value[delta.runId] = markRaw(nextSnapshot)
    return true
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
        const applied = applySnapshotDelta(data)
        if (!applied) {
          refreshSnapshot(data.runId).catch(err => {
            console.error('Failed to refresh snapshot after delta gap:', err)
          })
        }
      }
    })
    if (typeof unlistenSnapshotDelta === 'function') {
      cleanupFns.push(unlistenSnapshotDelta)
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
