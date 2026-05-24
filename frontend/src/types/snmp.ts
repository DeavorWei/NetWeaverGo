/**
 * SNMP 相关类型定义
 *
 * 类型来源规则：
 * 1. 与后端 View Model 对应的类型：定义在此文件
 * 2. Wails 绑定生成的类型：从 bindings 导出（构建后可用）
 *
 * @note 后端 View Model 定义见：
 * - internal/ui/snmp_mib_service.go (MIBModuleVM, MIBNodeVM 等)
 * - internal/snmp/types.go (MIBImportProgress, TrapEvent 等)
 */

// ============================================================================
// MIB 模块相关类型
// ============================================================================

/**
 * MIB 模块视图模型
 * @对应后端 MIBModuleVM
 */
export interface MIBModule {
  id: number
  name: string
  description: string
  version: string
  nodeCount: number
  importedAt: string // ISO 8601 时间字符串
  status: 'active' | 'error' | 'partial'
}

/**
 * MIB 节点视图模型
 * @对应后端 MIBNodeVM
 */
export interface MIBNode {
  id: number
  moduleId: number
  oid: string
  name: string
  description: string
  type: string // 对应后端 Syntax 字段
  access: string
  status: string
  parentId: number | null
  childrenIds: number[]
}

/**
 * MIB 树节点（用于树形视图）
 * @对应后端 snmp.MIBTreeNode
 */
export interface MIBTreeNode {
  id: number
  oid: string
  name: string
  nodeType: string
  syntax: string
  access: string
  status: string
  description: string
  children: MIBTreeNode[]
  hasChildren: boolean
}

/**
 * 导入 MIB 请求
 * @对应后端 ImportMIBRequest
 */
export interface ImportMIBRequest {
  filePath: string
  moduleName?: string
  partialImport: boolean
}

/**
 * 创建 MIB 节点请求
 * @对应后端 CreateMIBNodeRequest
 */
export interface CreateMIBNodeRequest {
  moduleId: number
  oid: string
  name: string
  description: string
  type: string
  access: string
  status: string
  parentId: number | null
}

/**
 * 更新 MIB 节点请求
 * @对应后端 UpdateMIBNodeRequest
 */
export interface UpdateMIBNodeRequest {
  name?: string
  description?: string
  type?: string
  access?: string
  status?: string
}

// ============================================================================
// OID 解析相关类型
// ============================================================================

/**
 * OID 解析结果视图模型
 * @对应后端 ResolvedOIDVM
 */
export interface ResolvedOID {
  oid: string
  name: string
  moduleName: string
  description: string
  type: string
  access: string
  status: string
  found: boolean
}

// ============================================================================
// SNMP 事件相关类型
// ============================================================================

/**
 * MIB 导入进度事件
 * @对应后端 MIBImportProgress
 */
export interface MIBImportProgress {
  fileName: string
  moduleName: string
  phase: 'parsing' | 'saving' | 'caching' | 'completed' | 'error'
  progress: number // 0-100
  nodesDone: number
  nodesTotal: number
  error?: string
  message?: string

  // 批量导入扩展字段
  batchId?: string
  totalFiles?: number
  processedFiles?: number
  currentPhase?: 'copy' | 'parse' | 'save' | 'cache' | 'done'
}

/**
 * 批量导入结果
 * @对应后端 MIBBatchImportResult
 */
export interface MIBBatchImportResult {
  totalFiles: number
  successCount: number
  failedCount: number
  skippedCount: number
  results: FileImportResult[]
  errors: FileImportError[]
  totalDuration: number // 毫秒
}

/**
 * 单文件导入结果
 * @对应后端 FileImportResult
 */
export interface FileImportResult {
  fileName: string
  moduleName: string
  nodeCount: number
  duration: number // 毫秒
  status: 'success' | 'failed' | 'skipped'
}

/**
 * 文件导入错误
 * @对应后端 FileImportError
 */
export interface FileImportError {
  fileName: string
  error: string
  errorType: 'parse' | 'dependency' | 'database' | 'unknown'
}

/**
 * 批量导入选项
 * @对应后端 MIBBatchImportOptions
 */
export interface MIBBatchImportOptions {
  concurrency: number // 1-8
  skipErrors: boolean
  overwriteExisting: boolean
  dependencyDirs: string[]
}

/**
 * 批量导入请求
 * @对应后端 ImportMIBFilesRequest
 */
export interface ImportMIBFilesRequest {
  filePaths: string[]
  folderId?: number
  concurrency: number
  skipErrors: boolean
  overwriteExisting: boolean
  dependencyDirs: string[]
}

/**
 * Trap 事件（轻量级，用于实时推送）
 * @对应后端 TrapEvent
 */
export interface TrapEvent {
  sourceIP: string
  sourcePort: number
  trapOID: string
  trapName: string
  severity: string
  community: string
  version: string
  receivedAt: number // Unix 毫秒时间戳
}

/**
 * 轮询结果事件（轻量级，用于实时推送）
 * @对应后端 PollingResultEvent
 */
export interface PollingResultEvent {
  targetId: number
  targetIP: string
  status: 'success' | 'error' | 'timeout'
  error?: string
  pollTime: number // Unix 毫秒时间戳
  oidCount: number
  batchId: string
}

/**
 * Trap 统计信息
 * @对应后端 TrapStats
 */
export interface TrapStats {
  totalCount: number
  unacknowledged: number
  criticalCount: number
  majorCount: number
  minorCount: number
  infoCount: number
  todayCount: number
  lastHourCount: number
}

// ============================================================================
// Trap 记录管理类型
// ============================================================================

/**
 * Trap 记录视图模型
 * @对应后端 TrapRecordVM
 */
export interface TrapRecord {
  id: number
  sourceIP: string
  sourcePort: number
  version: string
  community: string
  trapOID: string
  trapName: string
  enterprise: string
  genericTrap: number
  specificTrap: number
  severity: string
  variables: string
  acknowledged: boolean
  acknowledgedAt: string
  receivedAt: string
  /** UI 状态：正在确认中 */
  _acknowledging?: boolean
  /** UI 状态：正在删除中 */
  _deleting?: boolean
}

/**
 * Trap 记录列表视图模型
 * @对应后端 TrapRecordListVM
 */
export interface TrapRecordList {
  data: TrapRecord[]
  total: number
  page: number
  pageSize: number
  totalPages: number
}

/**
 * Trap 过滤条件视图模型
 * @对应后端 TrapFilterVM
 */
export interface TrapFilter {
  sourceIP?: string
  trapOID?: string
  severity?: string
  startTime?: string
  endTime?: string
  acknowledged?: boolean
  searchQuery?: string
}

/**
 * Trap VarBind 信息
 * @对应后端 trap_handler.go VarBind 结构
 */
export interface TrapVarBind {
  oid: string
  oidName?: string // 对应后端 OIDName 字段（可选）
  type: string // 对应后端 Type 字段
  value: unknown // 对应后端 Value 字段
}

// ============================================================================
// 监听器状态类型
// ============================================================================

/**
 * 监听器状态视图模型
 * @对应后端 ListenerStatusVM
 */
export interface ListenerStatus {
  isRunning: boolean
  listenAddr: string
  totalTraps: number
  filteredOut: number
  lastTrapTime: string
  startTime: string
  handlerStats: HandlerStats
}

/**
 * 处理统计视图模型
 * @对应后端 HandlerStatsVM
 */
export interface HandlerStats {
  totalReceived: number
  totalStored: number
  totalFiltered: number
  totalErrors: number
}

// ============================================================================
// 服务器配置类型
// ============================================================================

/**
 * 服务器配置视图模型
 * @对应后端 ServerConfigVM
 */
export interface ServerConfig {
  id: number
  trapEnabled: boolean
  trapPort: number
  trapCommunity: string
  maxStorageDays: number
}

/**
 * 更新服务器配置请求
 * @对应后端 UpdateServerConfigRequest
 */
export interface UpdateServerConfigRequest {
  trapEnabled: boolean
  trapPort: number
  trapCommunity: string
  maxStorageDays: number
}

// ============================================================================
// 过滤规则类型
// ============================================================================

/**
 * 过滤规则视图模型
 * @对应后端 FilterRuleVM
 */
export interface FilterRule {
  id: number
  name: string
  enabled: boolean
  priority: number
  action: 'accept' | 'drop' | 'severity_override'
  sourceIPPattern: string
  oidPattern: string
  communityPattern: string
  overrideSeverity: string
  description: string
  createdAt: string
  updatedAt: string
}

/**
 * 创建过滤规则请求
 * @对应后端 CreateFilterRuleRequest
 */
export interface CreateFilterRuleRequest {
  name: string
  enabled: boolean
  priority: number
  action: string
  sourceIPPattern: string
  oidPattern: string
  communityPattern: string
  overrideSeverity: string
  description: string
}

/**
 * 更新过滤规则请求
 * @对应后端 UpdateFilterRuleRequest
 */
export interface UpdateFilterRuleRequest {
  name: string
  enabled: boolean
  priority: number
  action: string
  sourceIPPattern: string
  oidPattern: string
  communityPattern: string
  overrideSeverity: string
  description: string
}

/**
 * Trap 统计视图模型
 * @对应后端 TrapStatsVM
 */
export interface TrapStatsVM {
  totalCount: number
  unacknowledged: number
  criticalCount: number
  majorCount: number
  minorCount: number
  infoCount: number
  todayCount: number
  lastHourCount: number
}

// ============================================================================
// 缓存统计类型
// ============================================================================

/**
 * OID 解析器缓存统计
 */
export interface OIDResolverCacheStats {
  oidCacheLen: number
  nameCacheLen: number
}

// ============================================================================
// SNMP 事件常量
// ============================================================================

/**
 * SNMP 事件名称常量
 * @对应后端 EventSNMP* 常量
 */
export const SNMPEventNames = {
  // Trap 事件
  TrapReceived: 'snmp:trap:received',
  TrapStats: 'snmp:trap:stats',
  ListenerStatus: 'snmp:listener:status',

  // 轮询事件
  PollResult: 'snmp:poll:result',
  PollError: 'snmp:poll:error',
  SchedulerStatus: 'snmp:scheduler:status',
  PollingResult: 'snmp:polling:result',

  // MIB 事件
  MIBImported: 'snmp:mib:imported',
  MIBDeleted: 'snmp:mib:deleted',
  MIBImportProgress: 'snmp:mib:import:progress',
} as const

export type SNMPEventName = typeof SNMPEventNames[keyof typeof SNMPEventNames]

// ============================================================================
// SNMP 轮询相关类型
// ============================================================================

/**
 * 凭据视图模型
 * @对应后端 CredentialVM (internal/ui/snmp_polling_service.go)
 * @note 字段名必须与后端 JSON 标签完全匹配
 */
export interface Credential {
  id: number
  name: string
  displayName: string // UI 显示名称 (JSON: "displayName")
  description: string // 描述 (JSON: "description")
  version: 'v1' | 'v2c' | 'v3' // 对应后端 Version 字段 (JSON: "version")
  snmpVersion: 'v1' | 'v2c' | 'v3' // 别名，用于 UI 显示 (JSON: "snmpVersion")
  community: string // 已脱敏 (JSON: "community") - v1/v2c
  securityLevel: 'noAuthNoPriv' | 'authNoPriv' | 'authPriv' // 对应后端 SecurityLevel 字段 (JSON: "securityLevel")
  username: string // 对应后端 Username 字段 (JSON: "username") - v3
  authProtocol: string // 对应后端 AuthProtocol 字段 (JSON: "authProtocol") - MD5/SHA/SHA224/SHA256/SHA384/SHA512
  hasAuthKey: boolean // 对应后端 HasAuthKey 字段 (JSON: "hasAuthKey")
  privProtocol: string // 对应后端 PrivProtocol 字段 (JSON: "privProtocol") - DES/AES/AES192/AES256/AES192C/AES256C
  hasPrivKey: boolean // 对应后端 HasPrivKey 字段 (JSON: "hasPrivKey")
  contextName: string // 对应后端 ContextName 字段 (JSON: "contextName")
  contextEngineId: string // 对应后端 ContextEngineID 字段 (JSON: "contextEngineId")
  createdAt: string
  updatedAt: string
}

/**
 * OID 项视图模型
 * @对应后端 OIDItemVM (internal/ui/snmp_polling_service.go)
 * @note 字段名必须与后端 JSON 标签完全匹配
 */
export interface OIDItem {
  oid: string // JSON: "oid"
  name: string // JSON: "name"
  type: string // JSON: "type" (对应后端 Type 字段)
  operation: string // JSON: "operation" (get/walk/bulk)
  description: string // JSON: "description"
}

/**
 * 轮询 OID 项视图模型
 * @对应后端 OIDItemVM (internal/ui/snmp_polling_service.go)
 * @note PollingOID 是 OIDItem 的别名，用于向后兼容
 */
export type PollingOID = OIDItem

/**
 * 轮询模板视图模型
 * @对应后端 PollingTemplateVM (internal/ui/snmp_polling_service.go)
 * @note 字段名必须与后端 JSON 标签完全匹配
 */
export interface PollingTemplate {
  id: number
  name: string
  displayName: string // UI 显示名称 (JSON: "displayName")
  description: string
  category: string // JSON: "category"
  oidItems: OIDItem[] // JSON: "oidItems" (对应后端 OIDItems 字段)
  createdAt: string
  updatedAt: string // JSON: "updatedAt"
}

/**
 * 轮询目标视图模型
 * @对应后端 PollingTargetVM (internal/ui/snmp_polling_service.go)
 * @note 字段名必须与后端 JSON 标签完全匹配
 */
export interface PollingTarget {
  id: number
  targetIP: string // JSON: "targetIP" (对应后端 TargetIP 字段)
  targetPort: number // JSON: "targetPort" (对应后端 TargetPort 字段)
  displayName: string // JSON: "displayName" (对应后端 DisplayName 字段)
  credentialId: number | null // JSON: "credentialId"
  credentialName: string // JSON: "credentialName"
  templateId: number | null // JSON: "templateId"
  templateName: string // JSON: "templateName"
  pollInterval: number // JSON: "pollInterval" (对应后端 PollInterval 字段，单位秒)
  enabled: boolean
  lastPollAt: string // JSON: "lastPollAt" (对应后端 LastPollAt 字段)
  lastPollStatus: string // JSON: "lastPollStatus"
  lastPollError: string // JSON: "lastPollError"
  lastSuccessAt: string // JSON: "lastSuccessAt" (对应后端 LastSuccessAt 字段)
  createdAt: string
  updatedAt: string
}

/**
 * 轮询结果视图模型
 * @对应后端 PollingResultVM (internal/ui/snmp_polling_service.go)
 * @note 字段名必须与后端 JSON 标签完全匹配
 */
export interface PollingResult {
  id: number
  targetId: number // JSON: "targetId"
  targetIP: string // JSON: "targetIP"
  batchId: string // JSON: "batchId"
  oid: string // JSON: "oid"
  oidName: string // JSON: "oidName"
  value: string // JSON: "value"
  valueType: string // JSON: "valueType"
  pollTime: string // JSON: "pollTime"
  createdAt: string
}

/**
 * 轮询结果列表视图模型
 * @对应后端 PollingResultListVM (internal/ui/snmp_polling_service.go)
 */
export interface PollingResultList {
  data: PollingResult[]
  total: number
  page: number
  pageSize: number
  totalPages: number
}

/**
 * 调度器状态视图模型
 * @对应后端 SchedulerStatusVM (internal/ui/snmp_polling_service.go)
 */
export interface SchedulerStatus {
  isRunning: boolean
  targetCount: number
  totalPolls: number
  lastPollTime: string
  startTime: string
}

/**
 * 轮询统计视图模型
 * @对应后端 PollingStatsVM (internal/ui/snmp_polling_service.go)
 */
export interface PollingStats {
  totalPolls: number
  successCount: number
  failCount: number
  avgLatencyMs: number
  lastPollTime: string
}

// ============================================================================
// SNMP 轮询请求类型
// ============================================================================

/**
 * 创建凭据请求
 * @对应后端 CreateCredentialRequest (internal/ui/snmp_polling_service.go)
 * @note 字段名必须与后端 JSON 标签完全匹配
 */
export interface CreateCredentialRequest {
  name: string
  version: 'v1' | 'v2c' | 'v3' // JSON: "version" (对应后端 Version 字段)
  community?: string // JSON: "community" - v1/v2c
  securityLevel?: 'noAuthNoPriv' | 'authNoPriv' | 'authPriv' // JSON: "securityLevel" - v3
  username?: string // JSON: "username" - v3
  authProtocol?: string // JSON: "authProtocol" - MD5/SHA/SHA224/SHA256/SHA384/SHA512
  authPassword?: string // JSON: "authPassword" - v3
  privProtocol?: string // JSON: "privProtocol" - DES/AES/AES192/AES256/AES192C/AES256C
  privPassword?: string // JSON: "privPassword" - v3
  contextName?: string // JSON: "contextName"
  contextEngineId?: string // JSON: "contextEngineId"
}

/**
 * 更新凭据请求
 * @对应后端 UpdateCredentialRequest (internal/ui/snmp_polling_service.go)
 */
export interface UpdateCredentialRequest {
  name: string
  version: 'v1' | 'v2c' | 'v3' // JSON: "version"
  community?: string // JSON: "community" - v1/v2c
  securityLevel?: 'noAuthNoPriv' | 'authNoPriv' | 'authPriv' // JSON: "securityLevel" - v3
  username?: string // JSON: "username" - v3
  authProtocol?: string // JSON: "authProtocol"
  authPassword?: string // JSON: "authPassword" - v3（为空则保持原值）
  privProtocol?: string // JSON: "privProtocol"
  privPassword?: string // JSON: "privPassword" - v3（为空则保持原值）
  contextName?: string // JSON: "contextName"
  contextEngineId?: string // JSON: "contextEngineId"
}

/**
 * 创建轮询模板请求
 * @对应后端 CreatePollingTemplateRequest (internal/ui/snmp_polling_service.go)
 */
export interface CreatePollingTemplateRequest {
  name: string
  displayName: string // UI 显示名称 (JSON: "displayName")
  description: string
  category: string // JSON: "category"
  oidItems: OIDItem[] // JSON: "oidItems" (对应后端 OIDItems 字段)
}

/**
 * 更新轮询模板请求
 * @对应后端 UpdatePollingTemplateRequest (internal/ui/snmp_polling_service.go)
 */
export interface UpdatePollingTemplateRequest {
  name: string
  description: string
  category: string
  oidItems: OIDItem[]
}

/**
 * 创建轮询目标请求
 * @对应后端 CreatePollingTargetRequest (internal/ui/snmp_polling_service.go)
 */
export interface CreatePollingTargetRequest {
  targetIP: string // JSON: "targetIP" (对应后端 TargetIP 字段)
  targetPort: number // JSON: "targetPort" (对应后端 TargetPort 字段)
  displayName: string // JSON: "displayName" (对应后端 DisplayName 字段)
  credentialId: number | null // JSON: "credentialId"
  templateId: number | null // JSON: "templateId"
  pollInterval: number // JSON: "pollInterval" (对应后端 PollInterval 字段，单位秒)
  enabled: boolean
}

/**
 * 更新轮询目标请求
 * @对应后端 UpdatePollingTargetRequest (internal/ui/snmp_polling_service.go)
 */
export interface UpdatePollingTargetRequest {
  targetIP: string
  targetPort: number
  displayName: string
  credentialId: number | null
  templateId: number | null
  pollInterval: number
  enabled: boolean
}

// ============================================================================
// SNMP 轮询过滤类型
// ============================================================================

/**
 * 轮询目标过滤条件
 * @对应后端 PollingTargetFilterVM (internal/ui/snmp_polling_service.go)
 */
export interface PollingTargetFilter {
  templateId?: number | null
  enabled?: boolean | null
  searchIP?: string // JSON: "searchIP"
}

/**
 * 轮询结果过滤条件
 * @对应后端 PollingResultFilterVM (internal/ui/snmp_polling_service.go)
 */
export interface PollingResultFilter {
  targetId?: number | null
  oid?: string // JSON: "oid"
  startTime?: string // JSON: "startTime"
  endTime?: string // JSON: "endTime"
  batchId?: string // JSON: "batchId"
}

// ============================================================================
// SNMPv3 用户管理类型
// ============================================================================

/**
 * v3 用户视图模型
 * @对应后端 V3UserVM (internal/ui/snmp_trap_service.go)
 */
export interface V3User {
  username: string
  authProtocol: string // MD5/SHA/SHA224/SHA256/SHA384/SHA512
  privProtocol: string // DES/AES/AES192/AES256/AES192C/AES256C
  securityLevel: 'noAuthNoPriv' | 'authNoPriv' | 'authPriv'
}

/**
 * 添加 v3 用户请求
 * @对应后端 AddV3UserRequest (internal/ui/snmp_trap_service.go)
 */
export interface AddV3UserRequest {
  username: string
  authProtocol: string // MD5/SHA/SHA224/SHA256/SHA384/SHA512
  authKey: string // 认证密钥
  privProtocol?: string // DES/AES/AES192/AES256/AES192C/AES256C
  privKey?: string // 加密密钥
  securityLevel: 'noAuthNoPriv' | 'authNoPriv' | 'authPriv'
}

// ============================================================================
// SNMPv3 常量
// ============================================================================

/** SNMPv3 认证协议选项 */
export const V3_AUTH_PROTOCOLS = [
  { value: 'MD5', label: 'MD5' },
  { value: 'SHA', label: 'SHA' },
  { value: 'SHA224', label: 'SHA-224' },
  { value: 'SHA256', label: 'SHA-256' },
  { value: 'SHA384', label: 'SHA-384' },
  { value: 'SHA512', label: 'SHA-512' },
] as const

/** SNMPv3 加密协议选项 */
export const V3_PRIV_PROTOCOLS = [
  { value: 'DES', label: 'DES' },
  { value: 'AES', label: 'AES' },
  { value: 'AES192', label: 'AES-192' },
  { value: 'AES256', label: 'AES-256' },
  { value: 'AES192C', label: 'AES-192C' },
  { value: 'AES256C', label: 'AES-256C' },
] as const

/** SNMPv3 安全级别选项 */
export const V3_SECURITY_LEVELS = [
  { value: 'noAuthNoPriv', label: '无认证无加密 (noAuthNoPriv)' },
  { value: 'authNoPriv', label: '认证无加密 (authNoPriv)' },
  { value: 'authPriv', label: '认证且加密 (authPriv)' },
] as const

// ============================================================================
// 轮询历史趋势图类型
// ============================================================================

/**
 * 轮询历史数据（用于趋势图）
 * @对应后端 PollingHistoryVM (internal/ui/snmp_polling_service.go)
 */
export interface PollingHistory {
  targetId: number
  targetName: string
  dataPoints: PollingDataPoint[]
}

/**
 * 轮询数据点
 * @对应后端 PollingDataPoint (internal/ui/snmp_polling_service.go)
 */
export interface PollingDataPoint {
  timestamp: string // ISO 8601 时间字符串
  success: boolean
  latencyMs: number
  values: Record<string, string> // OID -> Value
}

/**
 * 轮询趋势数据（单个 OID 聚合）
 * @对应后端 PollingTrendVM (internal/ui/snmp_polling_service.go)
 */
export interface PollingTrend {
  oid: string
  oidName: string
  dataPoints: TrendDataPoint[]
}

/**
 * 趋势数据点
 * @对应后端 TrendDataPoint (internal/ui/snmp_polling_service.go)
 */
export interface TrendDataPoint {
  timestamp: string // ISO 8601 时间字符串
  value: string
  numeric: boolean // 是否可转换为数值
}

/** 时间范围选项 */
export const TREND_DURATION_OPTIONS = [
  { value: '1h', label: '1 小时' },
  { value: '6h', label: '6 小时' },
  { value: '24h', label: '24 小时' },
  { value: '7d', label: '7 天' },
  { value: '30d', label: '30 天' },
] as const

// ============================================================================
// 清理配置类型
// ============================================================================

/**
 * 清理配置
 * @对应后端 CleanupConfigVM (internal/ui/snmp_trap_service.go)
 */
export interface CleanupConfig {
  trapRetentionDays: number // Trap 保留天数
  pollResultRetentionDays: number // 轮询结果保留天数
  cleanupIntervalHours: number // 清理间隔（小时）
  enabled: boolean // 是否启用自动清理
}

/**
 * 清理执行结果
 * @对应后端 CleanupResultVM (internal/ui/snmp_trap_service.go)
 */
export interface CleanupResult {
  trapDeleted: number // 删除的 Trap 记录数
  pollResultDeleted: number // 删除的轮询结果数
  executedAt: string // 执行时间
  durationMs: number // 执行耗时（毫秒）
}

// ============================================================================
// 通知设置类型
// ============================================================================

/**
 * 通知设置
 */
export interface NotificationSettings {
  soundEnabled: boolean // 声音通知开关
  desktopEnabled: boolean // 桌面通知开关
  criticalSound: boolean // critical 级别声音
  warningSound: boolean // warning 级别声音
  infoSound: boolean // info 级别声音
}

/** 默认通知设置 */
export const DEFAULT_NOTIFICATION_SETTINGS: NotificationSettings = {
  soundEnabled: true,
  desktopEnabled: false,
  criticalSound: true,
  warningSound: true,
  infoSound: false,
}