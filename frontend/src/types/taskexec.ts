/**
 * 统一任务执行运行时类型定义
 * 对应后端的 internal/taskexec/snapshot.go
 */

/** 执行快照 */
export interface ExecutionSnapshot {
  runId: string
  taskName: string
  runKind: string
  status: string
  progress: number
  revision: number
  lastRunSeq: number
  updatedAt: string
  currentStage: string
  stages: StageSnapshot[]
  units: UnitSnapshot[]
  startedAt?: string | null
  finishedAt?: string | null
  events: EventSnapshot[]
  lastSessionSeqByUnit?: Record<string, number | undefined>
}

/** Stage 快照 */
export interface StageSnapshot {
  id: string
  kind: string
  name: string
  order: number
  status: string
  progress: number
  totalUnits: number
  completedUnits: number
  successUnits: number
  failedUnits: number
  cancelledUnits: number
  partialUnits: number
  startedAt?: string | null
  finishedAt?: string | null
}

/** Unit 快照 */
export interface UnitSnapshot {
  id: string
  stageId: string
  kind: string
  targetType: string
  targetKey: string
  status: string
  progress: number
  totalSteps: number
  doneSteps: number
  errorMessage: string
  logs?: string[]
  logCount?: number
  truncated?: boolean
  summaryLogPath?: string
  detailLogPath?: string
  rawLogPath?: string
  journalLogPath?: string
  startedAt?: string | null
  finishedAt?: string | null
}

/** 事件快照 */
export interface EventSnapshot {
  id: string
  seq: number
  type: string
  level: string
  stageId?: string
  unitId?: string
  message: string
  timestamp: string
}

export type SnapshotDeltaOpType = 'run_patch' | 'stage_upsert' | 'unit_upsert' | 'event_append'

export interface SnapshotDeltaOp {
  type: SnapshotDeltaOpType
  status?: string
  currentStage?: string
  progress?: number
  startedAt?: string | null
  finishedAt?: string | null
  stage?: StageSnapshot
  unit?: UnitSnapshot
  event?: EventSnapshot
}

export interface SnapshotDelta {
  runId: string
  baseSeq: number
  seq: number
  revision: number
  updatedAt: string
  ops?: SnapshotDeltaOp[]
  snapshot?: ExecutionSnapshot
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
  'cancelled': '已取消',
  'aborted': '已中止',
  'skipped': '已跳过'
}

/** 状态颜色类名 */
export const StatusColorClasses: Record<string, string> = {
  'pending': 'text-text-muted',
  'running': 'text-accent',
  'completed': 'text-success',
  'partial': 'text-warning',
  'failed': 'text-error',
  'cancelled': 'text-text-muted',
  'aborted': 'text-error',
  'skipped': 'text-text-muted'
}

/** 状态背景色类名 */
export const StatusBgClasses: Record<string, string> = {
  'pending': 'bg-bg-panel',
  'running': 'bg-accent/20',
  'completed': 'bg-success/20',
  'partial': 'bg-warning/20',
  'failed': 'bg-error/20',
  'cancelled': 'bg-bg-panel',
  'aborted': 'bg-error/20',
  'skipped': 'bg-bg-panel'
}

// =============================================================================
// 离线重放模式类型定义
// =============================================================================

/** 重放选项 */
export interface ReplayOptions {
  /** 是否清除现有解析结果后重新构建 */
  clearExisting: boolean
  /** 指定解析器版本（空则使用当前版本） */
  parserVersion: string
  /** 指定要重放的设备列表（空则重放所有设备） */
  deviceIps: string[]
  /** 是否跳过构建阶段（仅执行解析） */
  skipBuild: boolean
}

/** 重放统计信息 */
export interface ReplayStatistics {
  totalRawFiles: number
  totalDevices: number
  totalCommandKeys: number
  parsedDevices: number
  parsedCommands: number
  failedCommands: number
  lldpCount: number
  fdbCount: number
  arpCount: number
  interfaceCount: number
  totalCandidates: number
  retainedEdges: number
  rejectedEdges: number
  conflictEdges: number
  scanDurationMs: number
  parseDurationMs: number
  buildDurationMs: number
  totalDurationMs: number
}

/** 重放结果 */
export interface ReplayResult {
  replayRunId: string
  status: string
  statistics: ReplayStatistics
  errors: string[]
  startedAt: string
  finishedAt: string
}

/** 可重放的运行信息 */
export interface ReplayableRunInfo {
  runId: string
  taskName: string
  status: string
  runKind: string
  deviceCount: number
  rawFileCount: number
  createdAt: string
  hasRawFiles: boolean
}

/** 重放记录 */
export interface TopologyReplayRecord {
  id: number
  originalRunId: string
  replayRunId: string
  status: string
  triggerSource: string
  parserVersion: string
  builderVersion: string
  startedAt?: string | null
  finishedAt?: string | null
  errorMessage: string
  statistics: string
  createdAt: string
}

/** Raw文件信息 */
export interface RawFileInfo {
  deviceIp: string
  commandKey: string
  filePath: string
  fileSize: number
}

/** Raw文件预览 */
export interface RawFilePreview {
  deviceIp: string
  commandKey: string
  filePath: string
  content: string
  size: number
  exists: boolean
}
