/**
 * SNMP MIB 管理 API 服务
 *
 * 提供与后端 SNMPMIBService、SNMPTrapService、SNMPPollingService 对应的 API 调用
 *
 * @note 使用 Wails v3 静态绑定方式，绑定文件由构建时自动生成
 * 绑定路径：../bindings/github.com/NetWeaverGo/core/internal/ui/
 *
 * @important 类型说明：
 * 所有返回类型直接使用绑定生成的类型（$models.XXX），确保与后端完全匹配
 * 前端组件应导入此文件导出的类型，而非从 types/snmp.ts 导入
 */

// ==================== 静态绑定导入 ====================
import * as SNMPMIBServiceBinding from '../bindings/github.com/NetWeaverGo/core/internal/ui/snmpmibservice'
import * as SNMPTrapServiceBinding from '../bindings/github.com/NetWeaverGo/core/internal/ui/snmptrapservice'
import * as SNMPPollingServiceBinding from '../bindings/github.com/NetWeaverGo/core/internal/ui/snmppollingservice'
import { Events } from '@wailsio/runtime'

// ==================== 绑定类型重导出 ====================
/**
 * 从绑定生成的 models.ts 重导出类型
 * 前端组件应使用这些类型而非本地定义的类型
 */
import * as $models from '../bindings/github.com/NetWeaverGo/core/internal/ui/models'
import * as $snmpModels from '../bindings/github.com/NetWeaverGo/core/internal/snmp/models'

// 重导出绑定类型供前端使用
export type {
  MIBFolderVM,
  MIBModuleVM,
  MIBNodeVM,
  ResolvedOIDVM,
  TrapRecordVM,
  TrapRecordListVM,
  TrapFilterVM,
  TrapStatsVM,
  ListenerStatusVM,
  ServerConfigVM,
  FilterRuleVM,
  CreateFilterRuleRequest,
  UpdateFilterRuleRequest,
  UpdateServerConfigRequest,
  CleanupConfigVM,
  CleanupResultVM,
  V3UserVM,
  AddV3UserRequest,
  CredentialVM,
  CreateCredentialRequest,
  UpdateCredentialRequest,
  PollingTemplateVM,
  CreatePollingTemplateRequest,
  UpdatePollingTemplateRequest,
  PollingTargetVM,
  CreatePollingTargetRequest,
  UpdatePollingTargetRequest,
  PollingTargetFilterVM,
  PollingResultFilterVM,
  PollingResultListVM,
  PollingResultVM,
  PollingStatsVM,
  PollingHistoryVM,
  PollingTrendVM,
  SchedulerStatusVM,
  ImportMIBRequest,
  CreateMIBNodeRequest,
  UpdateMIBNodeRequest,
} from '../bindings/github.com/NetWeaverGo/core/internal/ui/models'

export type { MIBTreeNode } from '../bindings/github.com/NetWeaverGo/core/internal/snmp/models'

// ==================== SNMP MIB API ====================
/**
 * SNMP MIB 管理 API
 * @description 提供 MIB 模块和节点的增删改查操作
 */
export const SNMPMIBAPI = {
  // ==================== MIB 模块管理 ====================

  /** 获取所有 MIB 模块列表 */
  getMIBModules: async (): Promise<$models.MIBModuleVM[]> => {
    return SNMPMIBServiceBinding.GetMIBModules()
  },

  /** 获取单个 MIB 模块详情 */
  getMIBModule: async (moduleID: number): Promise<$models.MIBModuleVM | null> => {
    return SNMPMIBServiceBinding.GetMIBModule(moduleID)
  },

  /** 导入 MIB 文件 */
  importMIB: async (req: $models.ImportMIBRequest): Promise<void> => {
    return SNMPMIBServiceBinding.ImportMIB(req)
  },

  /** 批量导入 MIB 文件 */
  importMIBFiles: async (filePaths: string[], folderID: number | null): Promise<void> => {
    return SNMPMIBServiceBinding.ImportMIBFiles(filePaths, folderID)
  },

  /** 删除 MIB 模块 */
  deleteMIBModule: async (moduleID: number): Promise<void> => {
    return SNMPMIBServiceBinding.DeleteMIBModule(moduleID)
  },

  // ==================== MIB 文件夹管理 ====================

  /** 获取所有 MIB 文件夹 */
  getMIBFolders: async (): Promise<$models.MIBFolderVM[]> => {
    return SNMPMIBServiceBinding.GetMIBFolders()
  },

  /** 创建 MIB 文件夹 */
  createMIBFolder: async (name: string): Promise<number> => {
    return SNMPMIBServiceBinding.CreateMIBFolder(name)
  },

  /** 重命名 MIB 文件夹 */
  renameMIBFolder: async (id: number, name: string): Promise<void> => {
    return SNMPMIBServiceBinding.RenameMIBFolder(id, name)
  },

  /** 删除 MIB 文件夹 */
  deleteMIBFolder: async (id: number): Promise<void> => {
    return SNMPMIBServiceBinding.DeleteMIBFolder(id)
  },

  /** 移动 MIB 模块到文件夹 */
  moveMIBModuleToFolder: async (moduleID: number, folderID: number | null): Promise<void> => {
    return SNMPMIBServiceBinding.MoveMIBModuleToFolder(moduleID, folderID)
  },

  // ==================== MIB 节点管理 ====================

  /** 获取 MIB 树形结构 */
  getMIBTree: async (moduleID: number): Promise<$snmpModels.MIBTreeNode[]> => {
    return SNMPMIBServiceBinding.GetMIBTree(moduleID)
  },

  /** 获取 MIB 节点详情 */
  getMIBNode: async (nodeID: number): Promise<$models.MIBNodeVM | null> => {
    return SNMPMIBServiceBinding.GetMIBNode(nodeID)
  },

  /** 手动创建 MIB 节点 */
  createMIBNode: async (req: $models.CreateMIBNodeRequest): Promise<void> => {
    return SNMPMIBServiceBinding.CreateMIBNode(req)
  },

  /** 更新 MIB 节点 */
  updateMIBNode: async (nodeID: number, req: $models.UpdateMIBNodeRequest): Promise<void> => {
    return SNMPMIBServiceBinding.UpdateMIBNode(nodeID, req)
  },

  /** 删除 MIB 节点 */
  deleteMIBNode: async (nodeID: number): Promise<void> => {
    return SNMPMIBServiceBinding.DeleteMIBNode(nodeID)
  },

  // ==================== OID 解析 ====================

  /** 解析 OID */
  resolveOID: async (oid: string): Promise<$models.ResolvedOIDVM | null> => {
    return SNMPMIBServiceBinding.ResolveOID(oid)
  },

  /** 将名称解析为 OID */
  resolveNameToOID: async (name: string): Promise<string> => {
    return SNMPMIBServiceBinding.ResolveNameToOID(name)
  },

  /** 搜索 MIB 节点 */
  searchMIBNodes: async (query: string): Promise<$models.MIBNodeVM[]> => {
    return SNMPMIBServiceBinding.SearchMIBNodes(query)
  },

  /** 在指定模块中搜索 MIB 节点 */
  searchMIBNodesInModule: async (moduleID: number, query: string): Promise<$models.MIBNodeVM[]> => {
    return SNMPMIBServiceBinding.SearchMIBNodesInModule(moduleID, query)
  },

  /** 导入 MIB 文件夹下的所有 MIB 文件 */
  importMIBFolder: async (folderPath: string, folderID: number | null): Promise<void> => {
    return SNMPMIBServiceBinding.ImportMIBFolder(folderPath, folderID)
  },


  // ==================== 缓存管理 ====================

  /** 清除 OID 解析器缓存 */
  clearResolverCache: async (): Promise<void> => {
    return SNMPMIBServiceBinding.ClearResolverCache()
  },

  /** 重建 OID 解析器缓存 */
  rebuildResolverCache: async (): Promise<void> => {
    return SNMPMIBServiceBinding.RebuildResolverCache()
  },

  /** 获取缓存统计信息 */
  getCacheStats: async (): Promise<{ [_ in string]?: number }> => {
    return SNMPMIBServiceBinding.GetCacheStats()
  },

  // ==================== 导出 ====================

  /** 导出 MIB 模块 */
  exportMIB: async (moduleID: number, format: string): Promise<Uint8Array> => {
    const result = await SNMPMIBServiceBinding.ExportMIB(moduleID, format)
    // 绑定返回 string (Base64)，转换为 Uint8Array
    if (typeof result === 'string') {
      return Uint8Array.from(atob(result), c => c.charCodeAt(0))
    }
    return result as unknown as Uint8Array
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
    callback: (progress: unknown) => void
  ): (() => void) => {
    const eventName = 'snmp:mib:import:progress'
    Events.On(eventName, (data: unknown) => {
      callback(data)
    })
    return () => Events.Off(eventName)
  },

  /** 监听 MIB 导入完成事件 */
  onMIBImported: (
    callback: (module: unknown) => void
  ): (() => void) => {
    const eventName = 'snmp:mib:imported'
    Events.On(eventName, (data: unknown) => {
      callback(data)
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
    callback: (trap: unknown) => void
  ): (() => void) => {
    const eventName = 'snmp:trap:received'
    Events.On(eventName, (data: unknown) => {
      callback(data)
    })
    return () => Events.Off(eventName)
  },

  /** 监听轮询结果事件 */
  onPollingResult: (
    callback: (result: unknown) => void
  ): (() => void) => {
    const eventName = 'snmp:polling:result'
    Events.On(eventName, (data: unknown) => {
      callback(data)
    })
    return () => Events.Off(eventName)
  },
} as const

// ==================== SNMP Trap API ====================
/**
 * SNMP Trap 管理 API
 * @description 提供 Trap 监听器管理、记录管理、过滤规则管理、服务器配置管理
 */
export const SNMPTrapAPI = {
  // ==================== 监听器管理 ====================

  /** 启动 Trap 监听器 */
  startListener: async (config: $models.ServerConfigVM): Promise<void> => {
    return SNMPTrapServiceBinding.StartListener(config)
  },

  /** 停止 Trap 监听器 */
  stopListener: async (): Promise<void> => {
    return SNMPTrapServiceBinding.StopListener()
  },

  /** 获取监听器状态 */
  getListenerStatus: async (): Promise<$models.ListenerStatusVM | null> => {
    return SNMPTrapServiceBinding.GetListenerStatus()
  },

  // ==================== Trap 记录管理 ====================

  /** 获取 Trap 记录列表 */
  getTrapRecords: async (filter: $models.TrapFilterVM, page: number, pageSize: number): Promise<$models.TrapRecordListVM | null> => {
    return SNMPTrapServiceBinding.GetTrapRecords(filter, page, pageSize)
  },

  /** 获取单个 Trap 记录详情 */
  getTrapRecord: async (id: number): Promise<$models.TrapRecordVM | null> => {
    return SNMPTrapServiceBinding.GetTrapRecord(id)
  },

  /** 删除单个 Trap 记录 */
  deleteTrapRecord: async (id: number): Promise<void> => {
    return SNMPTrapServiceBinding.DeleteTrapRecord(id)
  },

  /** 清理过期 Trap 记录 */
  clearTrapRecords: async (before: string): Promise<number> => {
    return SNMPTrapServiceBinding.ClearTrapRecords(before)
  },

  /** 确认单个 Trap 记录 */
  acknowledgeTrap: async (id: number): Promise<void> => {
    return SNMPTrapServiceBinding.AcknowledgeTrap(id)
  },

  /** 批量确认 Trap 记录 */
  batchAcknowledgeTraps: async (ids: number[]): Promise<void> => {
    return SNMPTrapServiceBinding.BatchAcknowledgeTraps(ids)
  },

  /** 获取 Trap 统计信息 */
  getTrapStats: async (): Promise<$models.TrapStatsVM | null> => {
    return SNMPTrapServiceBinding.GetTrapStats()
  },

  // ==================== 过滤规则管理 ====================

  /** 获取所有过滤规则 */
  getFilterRules: async (): Promise<$models.FilterRuleVM[]> => {
    return SNMPTrapServiceBinding.GetFilterRules()
  },

  /** 创建过滤规则 */
  createFilterRule: async (req: $models.CreateFilterRuleRequest): Promise<void> => {
    return SNMPTrapServiceBinding.CreateFilterRule(req)
  },

  /** 更新过滤规则 */
  updateFilterRule: async (id: number, req: $models.UpdateFilterRuleRequest): Promise<void> => {
    return SNMPTrapServiceBinding.UpdateFilterRule(id, req)
  },

  /** 删除过滤规则 */
  deleteFilterRule: async (id: number): Promise<void> => {
    return SNMPTrapServiceBinding.DeleteFilterRule(id)
  },

  /** 重新排序过滤规则 */
  reorderFilterRules: async (ids: number[]): Promise<void> => {
    return SNMPTrapServiceBinding.ReorderFilterRules(ids)
  },

  // ==================== 服务器配置管理 ====================

  /** 获取服务器配置列表 */
  getServerConfigs: async (): Promise<$models.ServerConfigVM[]> => {
    return SNMPTrapServiceBinding.GetServerConfigs()
  },

  /** 获取活动服务器配置 */
  getActiveServerConfig: async (): Promise<$models.ServerConfigVM | null> => {
    return SNMPTrapServiceBinding.GetActiveServerConfig()
  },

  /** 创建服务器配置 */
  createServerConfig: async (req: $models.CreateServerConfigRequest): Promise<void> => {
    return SNMPTrapServiceBinding.CreateServerConfig(req)
  },

  /** 更新服务器配置 */
  updateServerConfig: async (id: number, req: $models.UpdateServerConfigRequest): Promise<void> => {
    return SNMPTrapServiceBinding.UpdateServerConfig(id, req)
  },

  // ==================== v3 用户管理 ====================

  /** 添加 v3 用户 */
  addV3User: async (req: $models.AddV3UserRequest): Promise<void> => {
    return SNMPTrapServiceBinding.AddV3User(req)
  },

  /** 移除 v3 用户 */
  removeV3User: async (username: string): Promise<void> => {
    return SNMPTrapServiceBinding.RemoveV3User(username)
  },

  /** 获取所有 v3 用户 */
  listV3Users: async (): Promise<$models.V3UserVM[]> => {
    return SNMPTrapServiceBinding.ListV3Users()
  },

  // ==================== 清理配置管理 ====================

  /** 获取清理配置 */
  getCleanupConfig: async (): Promise<$models.CleanupConfigVM | null> => {
    return SNMPTrapServiceBinding.GetCleanupConfig()
  },

  /** 更新清理配置 */
  updateCleanupConfig: async (config: $models.CleanupConfigVM): Promise<void> => {
    return SNMPTrapServiceBinding.UpdateCleanupConfig(config)
  },

  /** 立即执行清理 */
  runCleanupNow: async (): Promise<$models.CleanupResultVM | null> => {
    return SNMPTrapServiceBinding.RunCleanupNow()
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
    callback: (trap: unknown) => void
  ): (() => void) => {
    const eventName = 'snmp:trap:received'
    Events.On(eventName, (data: unknown) => {
      callback(data)
    })
    return () => Events.Off(eventName)
  },

  /** 监听监听器状态变更事件 */
  onListenerStatusChanged: (
    callback: (status: unknown) => void
  ): (() => void) => {
    const eventName = 'snmp:listener:status'
    Events.On(eventName, (data: unknown) => {
      callback(data)
    })
    return () => Events.Off(eventName)
  },

  /** 监听 Trap 统计更新事件 */
  onTrapStats: (
    callback: (stats: unknown) => void
  ): (() => void) => {
    const eventName = 'snmp:trap:stats'
    Events.On(eventName, (data: unknown) => {
      callback(data)
    })
    return () => Events.Off(eventName)
  },
} as const

// ==================== SNMP 轮询 API ====================
/**
 * SNMP 轮询管理 API
 * @description 提供调度器管理、凭据管理、模板管理、目标管理、轮询操作、结果管理
 */
export const SNMPPollingAPI = {
  // ==================== 调度器管理 ====================

  /** 启动调度器 */
  startScheduler: async (): Promise<void> => {
    return SNMPPollingServiceBinding.StartScheduler()
  },

  /** 停止调度器 */
  stopScheduler: async (): Promise<void> => {
    return SNMPPollingServiceBinding.StopScheduler()
  },

  /** 获取调度器状态 */
  getSchedulerStatus: async (): Promise<$models.SchedulerStatusVM | null> => {
    return SNMPPollingServiceBinding.GetSchedulerStatus()
  },

  // ==================== 凭据管理 ====================

  /** 获取所有凭据 */
  getCredentials: async (): Promise<$models.CredentialVM[]> => {
    return SNMPPollingServiceBinding.GetCredentials()
  },

  /** 获取单个凭据 */
  getCredential: async (id: number): Promise<$models.CredentialVM | null> => {
    return SNMPPollingServiceBinding.GetCredential(id)
  },

  /** 创建凭据 */
  createCredential: async (req: $models.CreateCredentialRequest): Promise<void> => {
    return SNMPPollingServiceBinding.CreateCredential(req)
  },

  /** 更新凭据 */
  updateCredential: async (id: number, req: $models.UpdateCredentialRequest): Promise<void> => {
    return SNMPPollingServiceBinding.UpdateCredential(id, req)
  },

  /** 删除凭据 */
  deleteCredential: async (id: number): Promise<void> => {
    return SNMPPollingServiceBinding.DeleteCredential(id)
  },

  // ==================== 模板管理 ====================

  /** 获取所有轮询模板 */
  getPollingTemplates: async (): Promise<$models.PollingTemplateVM[]> => {
    return SNMPPollingServiceBinding.GetPollingTemplates()
  },

  /** 获取单个轮询模板 */
  getPollingTemplate: async (id: number): Promise<$models.PollingTemplateVM | null> => {
    return SNMPPollingServiceBinding.GetPollingTemplate(id)
  },

  /** 创建轮询模板 */
  createPollingTemplate: async (req: $models.CreatePollingTemplateRequest): Promise<void> => {
    return SNMPPollingServiceBinding.CreatePollingTemplate(req)
  },

  /** 更新轮询模板 */
  updatePollingTemplate: async (id: number, req: $models.UpdatePollingTemplateRequest): Promise<void> => {
    return SNMPPollingServiceBinding.UpdatePollingTemplate(id, req)
  },

  /** 删除轮询模板 */
  deletePollingTemplate: async (id: number): Promise<void> => {
    return SNMPPollingServiceBinding.DeletePollingTemplate(id)
  },

  // ==================== 目标管理 ====================

  /** 获取轮询目标列表 */
  getPollingTargets: async (filter?: $models.PollingTargetFilterVM): Promise<$models.PollingTargetVM[]> => {
    // 如果没有提供 filter，创建一个空的 filter
    const actualFilter = filter ?? new $models.PollingTargetFilterVM()
    return SNMPPollingServiceBinding.GetPollingTargets(actualFilter)
  },

  /** 获取单个轮询目标 */
  getPollingTarget: async (id: number): Promise<$models.PollingTargetVM | null> => {
    return SNMPPollingServiceBinding.GetPollingTarget(id)
  },

  /** 创建轮询目标 */
  createPollingTarget: async (req: $models.CreatePollingTargetRequest): Promise<void> => {
    return SNMPPollingServiceBinding.CreatePollingTarget(req)
  },

  /** 更新轮询目标 */
  updatePollingTarget: async (id: number, req: $models.UpdatePollingTargetRequest): Promise<void> => {
    return SNMPPollingServiceBinding.UpdatePollingTarget(id, req)
  },

  /** 删除轮询目标 */
  deletePollingTarget: async (id: number): Promise<void> => {
    return SNMPPollingServiceBinding.DeletePollingTarget(id)
  },

  /** 启用轮询目标 */
  enablePollingTarget: async (id: number): Promise<void> => {
    return SNMPPollingServiceBinding.EnablePollingTarget(id)
  },

  /** 禁用轮询目标 */
  disablePollingTarget: async (id: number): Promise<void> => {
    return SNMPPollingServiceBinding.DisablePollingTarget(id)
  },

  // ==================== 轮询操作 ====================

  /**
   * 立即轮询单个目标
   * @returns 轮询结果数组（一个目标可能产生多个 OID 结果）
   */
  pollNow: async (targetId: number): Promise<$models.PollingResultVM[]> => {
    return SNMPPollingServiceBinding.PollNow(targetId)
  },

  /**
   * 立即轮询所有目标
   * @returns 成功轮询的目标数量
   */
  pollAllNow: async (): Promise<number> => {
    return SNMPPollingServiceBinding.PollAllNow()
  },

  // ==================== 结果管理 ====================

  /** 获取轮询结果列表 */
  getPollingResults: async (
    filter: $models.PollingResultFilterVM,
    page: number,
    pageSize: number
  ): Promise<$models.PollingResultListVM | null> => {
    return SNMPPollingServiceBinding.GetPollingResults(filter, page, pageSize)
  },

  /** 清理过期轮询结果 */
  clearPollingResults: async (before: string): Promise<number> => {
    return SNMPPollingServiceBinding.ClearPollingResults(before)
  },

  /** 获取轮询统计信息 */
  getPollingStats: async (targetId: number): Promise<$models.PollingStatsVM | null> => {
    return SNMPPollingServiceBinding.GetPollingStats(targetId)
  },

  // ==================== 历史趋势图 ====================

  /** 获取轮询历史数据（用于趋势图） */
  getPollingHistory: async (
    targetId: number,
    duration: string
  ): Promise<$models.PollingHistoryVM | null> => {
    // 默认参数：limit=1000, offset=0
    return SNMPPollingServiceBinding.GetPollingHistory(targetId, duration, 1000, 0)
  },

  /** 获取轮询趋势数据（单个 OID 聚合） */
  getPollingTrend: async (
    targetId: number,
    oid: string,
    duration: string
  ): Promise<$models.PollingTrendVM | null> => {
    return SNMPPollingServiceBinding.GetPollingTrend(targetId, oid, duration)
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
    callback: (result: unknown) => void
  ): (() => void) => {
    const eventName = 'snmp:polling:result'
    Events.On(eventName, (data: unknown) => {
      callback(data)
    })
    return () => Events.Off(eventName)
  },

  /** 监听调度器状态变更事件 */
  onSchedulerStatusChanged: (
    callback: (status: unknown) => void
  ): (() => void) => {
    const eventName = 'snmp:scheduler:status'
    Events.On(eventName, (data: unknown) => {
      callback(data)
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

// P3-8: 导出临时 ID 生成器
export { generateTempId }

// ==================== 兼容性别名（已弃用） ====================
/**
 * 为向后兼容，提供类型别名
 * @deprecated 请直接使用绑定类型 MIBModuleVM, TrapRecordVM 等
 */
export type MIBModule = $models.MIBModuleVM
export type MIBNode = $models.MIBNodeVM
export type TrapRecord = $models.TrapRecordVM
export type TrapRecordList = $models.TrapRecordListVM
export type TrapFilter = $models.TrapFilterVM
export type TrapStats = $models.TrapStatsVM
export type ListenerStatus = $models.ListenerStatusVM
export type ServerConfig = $models.ServerConfigVM
export type FilterRule = $models.FilterRuleVM
export type Credential = $models.CredentialVM
export type PollingTemplate = $models.PollingTemplateVM
export type PollingTarget = $models.PollingTargetVM
export type PollingResult = $models.PollingResultVM
export type PollingResultList = $models.PollingResultListVM
export type SchedulerStatus = $models.SchedulerStatusVM
export type PollingStats = $models.PollingStatsVM
export type PollingHistory = $models.PollingHistoryVM
export type PollingTrend = $models.PollingTrendVM
export type CleanupConfig = $models.CleanupConfigVM
export type CleanupResult = $models.CleanupResultVM
export type V3User = $models.V3UserVM
export type MIBFolder = $models.MIBFolderVM
