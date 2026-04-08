import type { TaskRunRecordView } from '../services/api'

// 设备执行状态
export type DeviceExecutionStatus =
  | 'pending' // 等待中
  | 'running' // 执行中
  | 'completed' // 已完成
  | 'failed' // 失败
  | 'cancelled' // 已取消
  | 'partial' // 部分完成

// 文件类型
export type FileType = 'detail' | 'raw' | 'summary' | 'journal' | 'report'

// 设备执行详情（与后端 DeviceExecutionView 对应）
export interface DeviceExecutionView {
  unitId: string
  deviceIp: string
  status: DeviceExecutionStatus
  progress: number // 0-100
  totalSteps: number
  doneSteps: number
  errorMessage?: string
  startedAt?: string
  finishedAt?: string
  durationMs?: number

  // 日志文件路径
  detailLogPath?: string
  rawLogPath?: string
  summaryLogPath?: string
  journalLogPath?: string

  // 文件存在状态
  detailLogExists?: boolean
  rawLogExists?: boolean
  summaryLogExists?: boolean
  journalLogExists?: boolean
}

// 设备执行详情响应（与后端 DeviceDetailsResponse 对应）
export interface DeviceDetailsResponse {
  runId: string
  runStatus: string
  devices: DeviceExecutionView[]
}

// 文件位置请求（与后端 FileLocationRequest 对应）
export interface FileLocationRequest {
  runId: string
  unitId?: string
  fileType: FileType
}

// 文件位置响应（与后端 FileLocationResponse 对应）
export interface FileLocationResponse {
  success: boolean
  message: string
}

// 报告路径请求（与后端 ReportPathRequest 对应）
export interface ReportPathRequest {
  runId: string
}

// 报告路径响应（与后端 ReportPathResponse 对应）
export interface ReportPathResponse {
  reportPath: string
  exists: boolean
}

// 历史记录设备记录（保留向后兼容）
export interface ExecutionHistoryDeviceRecord {
  ip: string
  status: string
  errorMsg?: string
  execCmd?: number
  totalCmd?: number
  logCount?: number
  logTail?: string[]
  detailLogPath?: string
  logFilePath?: string
  rawLogPath?: string
}

// 历史记录（保留向后兼容）
export interface ExecutionHistoryRecord extends TaskRunRecordView {
  runnerId?: string
  devices?: ExecutionHistoryDeviceRecord[]
  reportPath?: string
  abortedCount?: number
  warningCount?: number
  createdAt?: string
}
