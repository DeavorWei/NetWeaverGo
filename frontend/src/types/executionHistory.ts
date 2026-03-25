import type { TaskRunRecordView } from '../services/api'

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

export interface ExecutionHistoryRecord extends TaskRunRecordView {
  runnerId?: string
  devices?: ExecutionHistoryDeviceRecord[]
  reportPath?: string
  abortedCount?: number
  warningCount?: number
  createdAt?: string
}
