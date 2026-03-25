/**
 * 统一任务执行运行时类型定义
 * 对应后端的 internal/taskexec/snapshot.go
 */

/** 执行快照 */
export interface ExecutionSnapshot {
  runId: string
  taskName: string
  runKind: 'normal' | 'topology'
  status: 'pending' | 'running' | 'completed' | 'partial' | 'failed' | 'cancelled'
  progress: number
  currentStage: string
  stages: StageSnapshot[]
  units: UnitSnapshot[]
  startedAt: string
  finishedAt: string
  events: EventSnapshot[]
}

/** Stage 快照 */
export interface StageSnapshot {
  id: string
  kind: 'device_command' | 'device_collect' | 'parse' | 'topology_build'
  name: string
  order: number
  status: string
  progress: number
  totalUnits: number
  completedUnits: number
  successUnits: number
  failedUnits: number
  startedAt: string
  finishedAt: string
}

/** Unit 快照 */
export interface UnitSnapshot {
  id: string
  stageId: string
  kind: 'device' | 'run' | 'dataset'
  targetType: 'device_ip' | 'task_run' | 'dataset_key'
  targetKey: string
  status: string
  progress: number
  totalSteps: number
  doneSteps: number
  errorMessage: string
  startedAt: string
  finishedAt: string
}

/** 事件快照 */
export interface EventSnapshot {
  id: string
  type: string
  level: 'info' | 'warn' | 'error'
  stageId?: string
  unitId?: string
  message: string
  timestamp: string
}

/** Stage 类型显示名称 */
export const StageKindNames: Record<string, string> = {
  'device_command': '命令执行',
  'device_collect': '设备采集',
  'parse': '数据解析',
  'topology_build': '拓扑构建'
}

/** 状态显示名称 */
export const StatusNames: Record<string, string> = {
  'pending': '等待中',
  'running': '执行中',
  'completed': '已完成',
  'partial': '部分完成',
  'failed': '失败',
  'cancelled': '已取消'
}

/** 状态颜色类名 */
export const StatusColorClasses: Record<string, string> = {
  'pending': 'text-text-muted',
  'running': 'text-accent',
  'completed': 'text-success',
  'partial': 'text-warning',
  'failed': 'text-error',
  'cancelled': 'text-text-muted'
}

/** 状态背景色类名 */
export const StatusBgClasses: Record<string, string> = {
  'pending': 'bg-bg-panel',
  'running': 'bg-accent/20',
  'completed': 'bg-success/20',
  'partial': 'bg-warning/20',
  'failed': 'bg-error/20',
  'cancelled': 'bg-bg-panel'
}
