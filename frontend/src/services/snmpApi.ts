/**
 * SNMP MIB 管理 API 服务
 *
 * 提供与后端 SNMPMIBService 对应的 API 调用
 *
 * @note 此文件遵循项目 api.ts 的命名空间模式
 * 绑定路径在 Wails 构建后生成，格式为：
 * ../bindings/github.com/NetWeaverGo/core/internal/ui/snmpmibservice
 *
 * @requires 后端需要将 SNMPMIBService 注册到 Wails 应用中
 * 见 cmd/netweaver/main.go 中的服务注册
 *
 * @todo 构建后需要：
 * 1. 取消注释 SNMPMIBServiceBinding 导入
 * 2. 将 invokeSNMPService 调用替换为静态绑定调用
 */

// ==================== 类型导入 ====================
import type {
  MIBModule,
  MIBNode,
  MIBTreeNode,
  ImportMIBRequest,
  CreateMIBNodeRequest,
  UpdateMIBNodeRequest,
  ResolvedOID,
  OIDResolverCacheStats,
  MIBImportProgress,
  TrapEvent,
  PollingResultEvent,
  TrapRecord,
  TrapRecordList,
  TrapFilter,
  ListenerStatus,
  ServerConfig,
  UpdateServerConfigRequest,
  FilterRule,
  CreateFilterRuleRequest,
  UpdateFilterRuleRequest,
  TrapStatsVM,
  // 轮询相关类型
  Credential,
  PollingTemplate,
  PollingTarget,
  PollingResult,
  PollingResultList,
  SchedulerStatus,
  PollingStats,
  CreateCredentialRequest,
  UpdateCredentialRequest,
  CreatePollingTemplateRequest,
  UpdatePollingTemplateRequest,
  CreatePollingTargetRequest,
  UpdatePollingTargetRequest,
  PollingTargetFilter,
  PollingResultFilter,
  // 历史趋势图类型
  PollingHistory,
  PollingTrend,
  // 清理配置类型
  CleanupConfig,
  CleanupResult,
  // v3 用户管理
  V3User,
  AddV3UserRequest,
} from '../types/snmp'

// ==================== Wails 运行时导入 ====================
// 构建后取消注释以下导入：
// import * as SNMPMIBServiceBinding from '../bindings/github.com/NetWeaverGo/core/internal/ui/snmpmibservice'
import { Call } from '@wailsio/runtime'
import { Events } from '@wailsio/runtime'

// ==================== SNMP MIB API ====================
/**
 * SNMP MIB 管理 API
 * @description 提供 MIB 模块和节点的增删改查操作
 *
 * @note 当前使用动态调用方式，构建后可切换为静态绑定
 */
export const SNMPMIBAPI = {
  // ==================== MIB 模块管理 ====================

  /** 获取所有 MIB 模块列表 */
  getMIBModules: async (): Promise<MIBModule[]> => {
    // TODO: 构建后替换为: return SNMPMIBServiceBinding.GetMIBModules()
    return invokeSNMPService<MIBModule[]>('GetMIBModules')
  },

  /** 获取单个 MIB 模块详情 */
  // P2-20: 返回类型包含 null，调用方需处理 null 情况
  getMIBModule: async (moduleID: number): Promise<MIBModule | null> => {
    return invokeSNMPService<MIBModule | null>('GetMIBModule', moduleID)
  },

  /** 导入 MIB 文件 */
  importMIB: async (req: ImportMIBRequest): Promise<void> => {
    return invokeSNMPService<void>('ImportMIB', req)
  },

  /** 批量导入 MIB 文件 */
  importMIBFiles: async (filePaths: string[]): Promise<void> => {
    return invokeSNMPService<void>('ImportMIBFiles', filePaths)
  },

  /** 删除 MIB 模块 */
  deleteMIBModule: async (moduleID: number): Promise<void> => {
    return invokeSNMPService<void>('DeleteMIBModule', moduleID)
  },

  // ==================== MIB 节点管理 ====================

  /** 获取 MIB 树形结构 */
  getMIBTree: async (moduleID: number): Promise<MIBTreeNode[]> => {
    return invokeSNMPService<MIBTreeNode[]>('GetMIBTree', moduleID)
  },

  /** 获取 MIB 节点详情 */
  // P2-20: 返回类型包含 null，调用方需处理 null 情况
  getMIBNode: async (nodeID: number): Promise<MIBNode | null> => {
    return invokeSNMPService<MIBNode | null>('GetMIBNode', nodeID)
  },

  /** 手动创建 MIB 节点 */
  createMIBNode: async (req: CreateMIBNodeRequest): Promise<void> => {
    return invokeSNMPService<void>('CreateMIBNode', req)
  },

  /** 更新 MIB 节点 */
  updateMIBNode: async (nodeID: number, req: UpdateMIBNodeRequest): Promise<void> => {
    return invokeSNMPService<void>('UpdateMIBNode', nodeID, req)
  },

  /** 删除 MIB 节点 */
  deleteMIBNode: async (nodeID: number): Promise<void> => {
    return invokeSNMPService<void>('DeleteMIBNode', nodeID)
  },

  // ==================== OID 解析 ====================

  /** 解析 OID */
  resolveOID: async (oid: string): Promise<ResolvedOID> => {
    return invokeSNMPService<ResolvedOID>('ResolveOID', oid)
  },

  /** 将名称解析为 OID */
  resolveNameToOID: async (name: string): Promise<string> => {
    return invokeSNMPService<string>('ResolveNameToOID', name)
  },

  /** 搜索 MIB 节点 */
  searchMIBNodes: async (query: string): Promise<MIBNode[]> => {
    return invokeSNMPService<MIBNode[]>('SearchMIBNodes', query)
  },

  // ==================== 缓存管理 ====================

  /** 清除 OID 解析器缓存 */
  clearResolverCache: async (): Promise<void> => {
    return invokeSNMPService<void>('ClearResolverCache')
  },

  /** 重建 OID 解析器缓存 */
  rebuildResolverCache: async (): Promise<void> => {
    return invokeSNMPService<void>('RebuildResolverCache')
  },

  /** 获取缓存统计信息 */
  getCacheStats: async (): Promise<OIDResolverCacheStats> => {
    return invokeSNMPService<OIDResolverCacheStats>('GetCacheStats')
  },

  // ==================== 导出 ====================

  /** 导出 MIB 模块 */
  exportMIB: async (moduleID: number, format: string): Promise<Uint8Array> => {
    return invokeSNMPService<Uint8Array>('ExportMIB', moduleID, format)
  },
} as const

// ==================== SNMP 事件监听 ====================
/**
 * SNMP 事件监听 API
 * @description 提供 Wails 事件的订阅和取消订阅功能
 */
export const SNMPEvents = {
  /** 监听 MIB 导入进度事件 */
  onMIBImportProgress: (
    callback: (progress: MIBImportProgress) => void
  ): (() => void) => {
    const eventName = 'snmp:mib:import:progress'
    Events.On(eventName, (data: unknown) => {
      callback(data as MIBImportProgress)
    })
    // 返回取消订阅函数
    return () => Events.Off(eventName)
  },

  /** 监听 MIB 导入完成事件 */
  onMIBImported: (
    callback: (module: MIBModule) => void
  ): (() => void) => {
    const eventName = 'snmp:mib:imported'
    Events.On(eventName, (data: unknown) => {
      callback(data as MIBModule)
    })
    return () => Events.Off(eventName)
  },

  /** 监听 MIB 删除事件 */
  onMIBDeleted: (
    callback: (moduleID: number) => void
  ): (() => void) => {
    const eventName = 'snmp:mib:deleted'
    Events.On(eventName, (data: unknown) => {
      callback(data as number)
    })
    return () => Events.Off(eventName)
  },

  /** 监听 Trap 接收事件 */
  onTrapReceived: (
    callback: (trap: TrapEvent) => void
  ): (() => void) => {
    const eventName = 'snmp:trap:received'
    Events.On(eventName, (data: unknown) => {
      callback(data as TrapEvent)
    })
    return () => Events.Off(eventName)
  },

  /** 监听轮询结果事件 */
  onPollingResult: (
    callback: (result: PollingResultEvent) => void
  ): (() => void) => {
    const eventName = 'snmp:polling:result'
    Events.On(eventName, (data: unknown) => {
      callback(data as PollingResultEvent)
    })
    return () => Events.Off(eventName)
  },
} as const

// ==================== SNMP Trap API ====================
/**
 * SNMP Trap 管理 API
 * @description 提供 Trap 监听器管理、记录管理、过滤规则管理、服务器配置管理
 *
 * @note 当前使用动态调用方式，构建后可切换为静态绑定
 */
export const SNMPTrapAPI = {
  // ==================== 监听器管理 ====================

  /** 启动 Trap 监听器 */
  startListener: async (config: ServerConfig): Promise<void> => {
    return invokeSNMPTrapService<void>('StartListener', config)
  },

  /** 停止 Trap 监听器 */
  stopListener: async (): Promise<void> => {
    return invokeSNMPTrapService<void>('StopListener')
  },

  /** 获取监听器状态 */
  // P2-20: 返回类型包含 null，调用方需处理 null 情况
  getListenerStatus: async (): Promise<ListenerStatus | null> => {
    return invokeSNMPTrapService<ListenerStatus | null>('GetListenerStatus')
  },

  // ==================== Trap 记录管理 ====================

  /** 获取 Trap 记录列表 */
  getTrapRecords: async (filter: TrapFilter, page: number, pageSize: number): Promise<TrapRecordList> => {
    return invokeSNMPTrapService<TrapRecordList>('GetTrapRecords', filter, page, pageSize)
  },

  /** 获取单个 Trap 记录详情 */
  // P2-20: 返回类型包含 null，调用方需处理 null 情况
  getTrapRecord: async (id: number): Promise<TrapRecord | null> => {
    return invokeSNMPTrapService<TrapRecord | null>('GetTrapRecord', id)
  },

  /** 删除单个 Trap 记录 */
  deleteTrapRecord: async (id: number): Promise<void> => {
    return invokeSNMPTrapService<void>('DeleteTrapRecord', id)
  },

  /** 清理过期 Trap 记录 */
  clearTrapRecords: async (before: string): Promise<number> => {
    return invokeSNMPTrapService<number>('ClearTrapRecords', before)
  },

  /** 确认单个 Trap 记录 */
  acknowledgeTrap: async (id: number): Promise<void> => {
    return invokeSNMPTrapService<void>('AcknowledgeTrap', id)
  },

  /** 批量确认 Trap 记录 */
  batchAcknowledgeTraps: async (ids: number[]): Promise<void> => {
    return invokeSNMPTrapService<void>('BatchAcknowledgeTraps', ids)
  },

  /** 获取 Trap 统计信息 */
  getTrapStats: async (): Promise<TrapStatsVM> => {
    return invokeSNMPTrapService<TrapStatsVM>('GetTrapStats')
  },

  // ==================== 过滤规则管理 ====================

  /** 获取所有过滤规则 */
  getFilterRules: async (): Promise<FilterRule[]> => {
    return invokeSNMPTrapService<FilterRule[]>('GetFilterRules')
  },

  /** 创建过滤规则 */
  createFilterRule: async (req: CreateFilterRuleRequest): Promise<void> => {
    return invokeSNMPTrapService<void>('CreateFilterRule', req)
  },

  /** 更新过滤规则 */
  updateFilterRule: async (id: number, req: UpdateFilterRuleRequest): Promise<void> => {
    return invokeSNMPTrapService<void>('UpdateFilterRule', id, req)
  },

  /** 删除过滤规则 */
  deleteFilterRule: async (id: number): Promise<void> => {
    return invokeSNMPTrapService<void>('DeleteFilterRule', id)
  },

  /** 重新排序过滤规则 */
  reorderFilterRules: async (ids: number[]): Promise<void> => {
    return invokeSNMPTrapService<void>('ReorderFilterRules', ids)
  },

  // ==================== 服务器配置管理 ====================

  /** 获取服务器配置列表 */
  getServerConfigs: async (): Promise<ServerConfig[]> => {
    return invokeSNMPTrapService<ServerConfig[]>('GetServerConfigs')
  },

  /** 获取活动服务器配置 */
  // P2-20: 返回类型包含 null，调用方需处理 null 情况
  getActiveServerConfig: async (): Promise<ServerConfig | null> => {
    return invokeSNMPTrapService<ServerConfig | null>('GetActiveServerConfig')
  },

  /** 更新服务器配置 */
  updateServerConfig: async (id: number, req: UpdateServerConfigRequest): Promise<void> => {
    return invokeSNMPTrapService<void>('UpdateServerConfig', id, req)
  },

  // ==================== v3 用户管理 ====================

  /** 添加 v3 用户 */
  addV3User: async (req: AddV3UserRequest): Promise<void> => {
    return invokeSNMPTrapService<void>('AddV3User', req)
  },

  /** 移除 v3 用户 */
  removeV3User: async (username: string): Promise<void> => {
    return invokeSNMPTrapService<void>('RemoveV3User', username)
  },

  /** 获取所有 v3 用户 */
  listV3Users: async (): Promise<V3User[]> => {
    return invokeSNMPTrapService<V3User[]>('ListV3Users')
  },

  // ==================== 清理配置管理 ====================

  /** 获取清理配置 */
  getCleanupConfig: async (): Promise<CleanupConfig> => {
    return invokeSNMPTrapService<CleanupConfig>('GetCleanupConfig')
  },

  /** 更新清理配置 */
  updateCleanupConfig: async (config: CleanupConfig): Promise<void> => {
    return invokeSNMPTrapService<void>('UpdateCleanupConfig', config)
  },

  /** 立即执行清理 */
  runCleanupNow: async (): Promise<CleanupResult> => {
    return invokeSNMPTrapService<CleanupResult>('RunCleanupNow')
  },
} as const

// ==================== SNMP Trap 事件监听 ====================
/**
 * SNMP Trap 事件监听 API
 * @description 提供 Trap 相关 Wails 事件的订阅和取消订阅功能
 */
export const SNMPTrapEvents = {
  /** 监听 Trap 接收事件 */
  onTrapReceived: (
    callback: (trap: TrapEvent) => void
  ): (() => void) => {
    const eventName = 'snmp:trap:received'
    Events.On(eventName, (data: unknown) => {
      callback(data as TrapEvent)
    })
    return () => Events.Off(eventName)
  },

  /** 监听监听器状态变更事件 */
  onListenerStatusChanged: (
    callback: (status: ListenerStatus) => void
  ): (() => void) => {
    const eventName = 'snmp:listener:status'
    Events.On(eventName, (data: unknown) => {
      callback(data as ListenerStatus)
    })
    return () => Events.Off(eventName)
  },

  /** 监听 Trap 统计更新事件 */
  onTrapStats: (
    callback: (stats: TrapStatsVM) => void
  ): (() => void) => {
    const eventName = 'snmp:trap:stats'
    Events.On(eventName, (data: unknown) => {
      callback(data as TrapStatsVM)
    })
    return () => Events.Off(eventName)
  },
} as const

// ==================== SNMP 轮询 API ====================
/**
 * SNMP 轮询管理 API
 * @description 提供调度器管理、凭据管理、模板管理、目标管理、轮询操作、结果管理
 *
 * @note 当前使用动态调用方式，构建后可切换为静态绑定
 */
export const SNMPPollingAPI = {
  // ==================== 调度器管理 ====================

  /** 启动调度器 */
  startScheduler: async (): Promise<void> => {
    return invokeSNMPPollingService<void>('StartScheduler')
  },

  /** 停止调度器 */
  stopScheduler: async (): Promise<void> => {
    return invokeSNMPPollingService<void>('StopScheduler')
  },

  /** 获取调度器状态 */
  getSchedulerStatus: async (): Promise<SchedulerStatus> => {
    return invokeSNMPPollingService<SchedulerStatus>('GetSchedulerStatus')
  },

  // ==================== 凭据管理 ====================

  /** 获取所有凭据 */
  getCredentials: async (): Promise<Credential[]> => {
    return invokeSNMPPollingService<Credential[]>('GetCredentials')
  },

  /** 获取单个凭据 */
  // P2-20: 返回类型包含 null，调用方需处理 null 情况
  getCredential: async (id: number): Promise<Credential | null> => {
    return invokeSNMPPollingService<Credential | null>('GetCredential', id)
  },

  /** 创建凭据 */
  createCredential: async (req: CreateCredentialRequest): Promise<void> => {
    return invokeSNMPPollingService<void>('CreateCredential', req)
  },

  /** 更新凭据 */
  updateCredential: async (id: number, req: UpdateCredentialRequest): Promise<void> => {
    return invokeSNMPPollingService<void>('UpdateCredential', id, req)
  },

  /** 删除凭据 */
  deleteCredential: async (id: number): Promise<void> => {
    return invokeSNMPPollingService<void>('DeleteCredential', id)
  },

  // ==================== 模板管理 ====================

  /** 获取所有轮询模板 */
  getPollingTemplates: async (): Promise<PollingTemplate[]> => {
    return invokeSNMPPollingService<PollingTemplate[]>('GetPollingTemplates')
  },

  /** 获取单个轮询模板 */
  // P2-20: 返回类型包含 null，调用方需处理 null 情况
  getPollingTemplate: async (id: number): Promise<PollingTemplate | null> => {
    return invokeSNMPPollingService<PollingTemplate | null>('GetPollingTemplate', id)
  },

  /** 创建轮询模板 */
  createPollingTemplate: async (req: CreatePollingTemplateRequest): Promise<void> => {
    return invokeSNMPPollingService<void>('CreatePollingTemplate', req)
  },

  /** 更新轮询模板 */
  updatePollingTemplate: async (id: number, req: UpdatePollingTemplateRequest): Promise<void> => {
    return invokeSNMPPollingService<void>('UpdatePollingTemplate', id, req)
  },

  /** 删除轮询模板 */
  deletePollingTemplate: async (id: number): Promise<void> => {
    return invokeSNMPPollingService<void>('DeletePollingTemplate', id)
  },

  // ==================== 目标管理 ====================

  /** 获取轮询目标列表 */
  getPollingTargets: async (filter?: PollingTargetFilter): Promise<PollingTarget[]> => {
    return invokeSNMPPollingService<PollingTarget[]>('GetPollingTargets', filter)
  },

  /** 获取单个轮询目标 */
  // P2-20: 返回类型包含 null，调用方需处理 null 情况
  getPollingTarget: async (id: number): Promise<PollingTarget | null> => {
    return invokeSNMPPollingService<PollingTarget | null>('GetPollingTarget', id)
  },

  /** 创建轮询目标 */
  createPollingTarget: async (req: CreatePollingTargetRequest): Promise<void> => {
    return invokeSNMPPollingService<void>('CreatePollingTarget', req)
  },

  /** 更新轮询目标 */
  updatePollingTarget: async (id: number, req: UpdatePollingTargetRequest): Promise<void> => {
    return invokeSNMPPollingService<void>('UpdatePollingTarget', id, req)
  },

  /** 删除轮询目标 */
  deletePollingTarget: async (id: number): Promise<void> => {
    return invokeSNMPPollingService<void>('DeletePollingTarget', id)
  },

  /** 启用轮询目标 */
  enablePollingTarget: async (id: number): Promise<void> => {
    return invokeSNMPPollingService<void>('EnablePollingTarget', id)
  },

  /** 禁用轮询目标 */
  disablePollingTarget: async (id: number): Promise<void> => {
    return invokeSNMPPollingService<void>('DisablePollingTarget', id)
  },

  // ==================== 轮询操作 ====================

  /**
   * 立即轮询单个目标
   * @returns 轮询结果数组（一个目标可能产生多个 OID 结果）
   */
  pollNow: async (targetId: number): Promise<PollingResult[]> => {
    return invokeSNMPPollingService<PollingResult[]>('PollNow', targetId)
  },

  /**
   * 立即轮询所有目标
   * @returns 成功轮询的目标数量
   */
  pollAllNow: async (): Promise<number> => {
    return invokeSNMPPollingService<number>('PollAllNow')
  },

  // ==================== 结果管理 ====================

  /** 获取轮询结果列表 */
  getPollingResults: async (
    filter: PollingResultFilter,
    page: number,
    pageSize: number
  ): Promise<PollingResultList> => {
    return invokeSNMPPollingService<PollingResultList>('GetPollingResults', filter, page, pageSize)
  },

  /** 清理过期轮询结果 */
  clearPollingResults: async (before: string): Promise<number> => {
    return invokeSNMPPollingService<number>('ClearPollingResults', before)
  },

  /** 获取轮询统计信息 */
  getPollingStats: async (targetId: number): Promise<PollingStats> => {
    return invokeSNMPPollingService<PollingStats>('GetPollingStats', targetId)
  },

  // ==================== 历史趋势图 ====================

  /** 获取轮询历史数据（用于趋势图） */
  getPollingHistory: async (
    targetId: number,
    duration: string
  ): Promise<PollingHistory> => {
    return invokeSNMPPollingService<PollingHistory>('GetPollingHistory', targetId, duration)
  },

  /** 获取轮询趋势数据（单个 OID 聚合） */
  getPollingTrend: async (
    targetId: number,
    oid: string,
    duration: string
  ): Promise<PollingTrend> => {
    return invokeSNMPPollingService<PollingTrend>('GetPollingTrend', targetId, oid, duration)
  },
} as const

// ==================== SNMP 轮询事件监听 ====================
/**
 * SNMP 轮询事件监听 API
 * @description 提供轮询相关 Wails 事件的订阅和取消订阅功能
 */
export const SNMPPollingEvents = {
  /** 监听轮询结果事件 */
  onPollingResult: (
    callback: (result: PollingResultEvent) => void
  ): (() => void) => {
    const eventName = 'snmp:polling:result'
    Events.On(eventName, (data: unknown) => {
      callback(data as PollingResultEvent)
    })
    return () => Events.Off(eventName)
  },

  /** 监听调度器状态变更事件 */
  onSchedulerStatusChanged: (
    callback: (status: SchedulerStatus) => void
  ): (() => void) => {
    const eventName = 'snmp:scheduler:status'
    Events.On(eventName, (data: unknown) => {
      callback(data as SchedulerStatus)
    })
    return () => Events.Off(eventName)
  },

  /** 监听轮询错误事件 */
  onPollingError: (
    callback: (error: { targetId: number; error: string }) => void
  ): (() => void) => {
    const eventName = 'snmp:poll:error'
    Events.On(eventName, (data: unknown) => {
      callback(data as { targetId: number; error: string })
    })
    return () => Events.Off(eventName)
  },
} as const

// ==================== 辅助函数 ====================

// P3-8: 临时 ID 生成器，使用 Date.now() + 递增计数器避免重复
let _tempIdCounter = 0
function generateTempId(): number {
  return Date.now() * 1000 + (++_tempIdCounter % 1000)
}

/**
 * P2-17: 使用正确的类型索引签名替代 @ts-ignore
 * 动态服务调用类型定义
 */
interface WailsServiceCaller {
  (serviceName: string, methodName: string, ...args: unknown[]): Promise<unknown>
}

/**
 * 动态调用 SNMP MIB 服务方法
 * @note 此函数为临时方案，构建后应替换为静态绑定调用
 *
 * @param methodName - 服务方法名称
 * @param args - 方法参数
 * @returns Promise<T> - 方法返回值
 */
async function invokeSNMPService<T>(methodName: string, ...args: unknown[]): Promise<T> {
  // 使用 Wails runtime 动态调用
  // 构建后会生成静态绑定，可以直接调用 SNMPMIBServiceBinding[methodName](...args)
  // Call.ByID 需要方法 ID，这里使用名称调用
  // P2-17: 使用类型断言替代 @ts-ignore
  const caller = Call.ByName as WailsServiceCaller
  return caller('SNMPMIBService', methodName, ...args) as Promise<T>
}

/**
 * 动态调用 SNMP Trap 服务方法
 * @note 此函数为临时方案，构建后应替换为静态绑定调用
 *
 * @param methodName - 服务方法名称
 * @param args - 方法参数
 * @returns Promise<T> - 方法返回值
 */
async function invokeSNMPTrapService<T>(methodName: string, ...args: unknown[]): Promise<T> {
  // P2-17: 使用类型断言替代 @ts-ignore
  const caller = Call.ByName as WailsServiceCaller
  return caller('SNMPTrapService', methodName, ...args) as Promise<T>
}

/**
 * 动态调用 SNMP 轮询服务方法
 * @note 此函数为临时方案，构建后应替换为静态绑定调用
 *
 * @param methodName - 服务方法名称
 * @param args - 方法参数
 * @returns Promise<T> - 方法返回值
 */
async function invokeSNMPPollingService<T>(methodName: string, ...args: unknown[]): Promise<T> {
  // P2-17: 使用类型断言替代 @ts-ignore
  const caller = Call.ByName as WailsServiceCaller
  return caller('SNMPPollingService', methodName, ...args) as Promise<T>
}

// ==================== 类型导出 ====================
export type {
  MIBModule,
  MIBNode,
  MIBTreeNode,
  ImportMIBRequest,
  CreateMIBNodeRequest,
  UpdateMIBNodeRequest,
  ResolvedOID,
  OIDResolverCacheStats,
  MIBImportProgress,
  TrapEvent,
  PollingResultEvent,
  TrapRecord,
  TrapRecordList,
  TrapFilter,
  ListenerStatus,
  HandlerStats,
  ServerConfig,
  UpdateServerConfigRequest,
  FilterRule,
  CreateFilterRuleRequest,
  UpdateFilterRuleRequest,
  TrapStatsVM,
  // 轮询相关类型
  Credential,
  OIDItem,
  PollingTemplate,
  PollingTarget,
  PollingResult,
  PollingResultList,
  SchedulerStatus,
  PollingStats,
  CreateCredentialRequest,
  UpdateCredentialRequest,
  CreatePollingTemplateRequest,
  UpdatePollingTemplateRequest,
  CreatePollingTargetRequest,
  UpdatePollingTargetRequest,
  PollingTargetFilter,
  PollingResultFilter,
  // v3 用户管理
  V3User,
  AddV3UserRequest,
  // 历史趋势图
  PollingHistory,
  PollingTrend,
  // 清理配置
  CleanupConfig,
  CleanupResult,
} from '../types/snmp'

export { SNMPEventNames, TREND_DURATION_OPTIONS, DEFAULT_NOTIFICATION_SETTINGS } from '../types/snmp'

// P3-8: 导出临时 ID 生成器
export { generateTempId }