/**
 * 统一的后端 API 导出文件
 *
 * 采用命名空间模式组织，提供类型安全的 API 调用
 * 每个命名空间对应后端的一个独立服务
 *
 * 类型来源规则：
 * 1. 后端 DTO 类型：全部从 bindings 导出，不在本文件重复定义
 * 2. 前端视图态/表单态类型：定义在 types/ 目录
 * 3. 本文件仅负责聚合导出，保持类型来源唯一
 *
 * @example
 * ```ts
 * // 后端 DTO 从 api.ts 导入
 * import type { DeviceAsset, CommandGroup } from './services/api'
 *
 * // 前端特有类型从 types/ 导入
 * import type { DeviceFormData } from './types/command'
 * ```
 */

// ==================== 服务命名空间导入 ====================
import * as DeviceServiceBinding from '../bindings/github.com/NetWeaverGo/core/internal/ui/deviceservice'
import * as CommandGroupServiceBinding from '../bindings/github.com/NetWeaverGo/core/internal/ui/commandgroupservice'
import * as SettingsServiceBinding from '../bindings/github.com/NetWeaverGo/core/internal/ui/settingsservice'
import * as TaskGroupServiceBinding from '../bindings/github.com/NetWeaverGo/core/internal/ui/taskgroupservice'
import * as ForgeServiceBinding from '../bindings/github.com/NetWeaverGo/core/internal/ui/forgeservice'
import * as QueryServiceBinding from '../bindings/github.com/NetWeaverGo/core/internal/ui/queryservice'
import * as ExecutionHistoryServiceBinding from '../bindings/github.com/NetWeaverGo/core/internal/ui/executionhistoryservice'
import * as PlanCompareServiceBinding from '../bindings/github.com/NetWeaverGo/core/internal/ui/plancompareservice'
import * as TaskExecutionUIServiceBinding from '../bindings/github.com/NetWeaverGo/core/internal/ui/taskexecutionuiservice'
import * as TopologyCommandServiceBinding from '../bindings/github.com/NetWeaverGo/core/internal/ui/topologycommandservice'

// ==================== 设备管理 API ====================
/**
 * 设备管理 API
 * @description 提供设备的增删改查操作
 */
export const DeviceAPI = {
  /** 获取设备列表 */
  listDevices: DeviceServiceBinding.ListDevices,
  /** 根据 ID 获取单个设备详情（包含密码，用于编辑） */
  getDeviceById: DeviceServiceBinding.GetDeviceByID,
  /** 重置指定设备的 SSH 主机密钥 */
  resetDeviceSSHHostKey: DeviceServiceBinding.ResetDeviceSSHHostKey,
  /** 新增设备 */
  addDevice: DeviceServiceBinding.AddDevice,
  /** 批量新增设备 */
  addDevices: DeviceServiceBinding.AddDevices,
  /** 更新设备 */
  updateDevice: DeviceServiceBinding.UpdateDevice,
  /** 批量更新设备 */
  updateDevices: DeviceServiceBinding.UpdateDevices,
  /** 删除单台设备 */
  deleteDevice: DeviceServiceBinding.DeleteDevice,
  /** 批量删除设备 */
  deleteDevices: DeviceServiceBinding.DeleteDevices,
  /** 获取协议默认端口映射 */
  getProtocolDefaultPorts: DeviceServiceBinding.GetProtocolDefaultPorts,
  /** 获取有效协议列表 */
  getValidProtocols: DeviceServiceBinding.GetValidProtocols,
} as const

// ==================== 命令组管理 API ====================
/**
 * 命令组管理 API
 * @description 提供命令组的增删改查和导入导出操作
 */
export const CommandGroupAPI = {
  /** 获取所有命令组 */
  listCommandGroups: CommandGroupServiceBinding.ListCommandGroups,
  /** 获取单个命令组详情 */
  getCommandGroup: CommandGroupServiceBinding.GetCommandGroup,
  /** 创建命令组 */
  createCommandGroup: CommandGroupServiceBinding.CreateCommandGroup,
  /** 更新命令组 */
  updateCommandGroup: CommandGroupServiceBinding.UpdateCommandGroup,
  /** 删除命令组 */
  deleteCommandGroup: CommandGroupServiceBinding.DeleteCommandGroup,
  /** 复制命令组 */
  duplicateCommandGroup: CommandGroupServiceBinding.DuplicateCommandGroup,
  /** 导入命令组 */
  importCommandGroup: CommandGroupServiceBinding.ImportCommandGroup,
  /** 导出命令组 */
  exportCommandGroup: CommandGroupServiceBinding.ExportCommandGroup,
} as const

// ==================== 设置管理 API ====================
/**
 * 设置管理 API
 * @description 提供应用配置的加载和保存操作
 */
export const SettingsAPI = {
  /** 加载设置 */
  loadSettings: SettingsServiceBinding.LoadSettings,
  /** 获取 SSH 算法候选列表 */
  getSSHAlgorithmOptions: SettingsServiceBinding.GetSSHAlgorithmOptions,
  /** 保存设置 */
  saveSettings: SettingsServiceBinding.SaveSettings,
  /** 确保配置文件存在 */
  ensureConfig: SettingsServiceBinding.EnsureConfig,
  /** 获取应用信息 */
  getAppInfo: SettingsServiceBinding.GetAppInfo,
  /** 记录 INFO 级别日志 */
  logInfo: SettingsServiceBinding.LogInfo,
  /** 记录 WARN 级别日志 */
  logWarn: SettingsServiceBinding.LogWarn,
  /** 记录 ERROR 级别日志 */
  logError: SettingsServiceBinding.LogError,
  /** 获取运行时配置 */
  getRuntimeConfig: SettingsServiceBinding.GetRuntimeConfig,
  /** 更新运行时配置 */
  updateRuntimeConfig: SettingsServiceBinding.UpdateRuntimeConfig,
  /** 重置运行时配置为默认值 */
  resetRuntimeConfigToDefault: SettingsServiceBinding.ResetRuntimeConfigToDefault,
} as const

// 导出运行时配置类型（RuntimeConfigData 是类，需要作为值导出）
export { RuntimeConfigData } from '../bindings/github.com/NetWeaverGo/core/internal/ui/models.js'

// ==================== 任务组管理 API ====================
/**
 * 任务组管理 API
 * @description 提供任务组的增删改查和执行操作
 */
export const TaskGroupAPI = {
  /** 获取所有任务组 */
  listTaskGroups: TaskGroupServiceBinding.ListTaskGroups,
  /** 获取单个任务组详情 */
  getTaskGroup: TaskGroupServiceBinding.GetTaskGroup,
  /** 获取任务详情聚合数据 */
  getTaskGroupDetail: TaskGroupServiceBinding.GetTaskGroupDetail,
  /** 创建任务组 */
  createTaskGroup: TaskGroupServiceBinding.CreateTaskGroup,
  /** 更新任务组 */
  updateTaskGroup: TaskGroupServiceBinding.UpdateTaskGroup,
  /** 删除任务组 */
  deleteTaskGroup: TaskGroupServiceBinding.DeleteTaskGroup,
  /** 启动任务组执行 */
  startTaskGroup: (id: number): Promise<string> => TaskGroupServiceBinding.StartTaskGroup(id) as unknown as Promise<string>,
} as const

// ==================== 统一任务执行 API (阶段2/3) ====================
/**
 * 统一任务执行 API
 * @description 提供统一运行时的任务执行、状态查询和控制功能
 * @note 此API绑定由Wails自动生成，需要在构建后更新
 */
export const TaskExecutionAPI = {
  /** 获取任务快照 */
  getTaskSnapshot: TaskExecutionUIServiceBinding.GetTaskSnapshot,
  /** 列出正在运行的任务 */
  listRunningTasks: async (): Promise<any[]> =>
    (await TaskExecutionUIServiceBinding.ListRunningTasks()).filter(Boolean),
  /** 获取历史运行记录 */
  listTaskRuns: async (limit: number = 50): Promise<any[]> =>
    (await TaskExecutionUIServiceBinding.ListTaskRuns(limit)).filter(Boolean),
  /** 取消任务 */
  cancelTask: TaskExecutionUIServiceBinding.CancelTask,
  /** 订阅任务事件 */
  subscribeRunEvents: TaskExecutionUIServiceBinding.SubscribeRunEvents,
  /** 取消订阅任务事件 */
  unsubscribeRunEvents: TaskExecutionUIServiceBinding.UnsubscribeRunEvents,
  /** 获取拓扑图 */
  getTopologyGraph: TaskExecutionUIServiceBinding.GetTopologyGraph,
  /** 获取链路详情 */
  getTopologyEdgeDetail: TaskExecutionUIServiceBinding.GetTopologyEdgeDetail,
  /** 获取设备拓扑详情 */
  getTopologyDeviceDetail: TaskExecutionUIServiceBinding.GetTopologyDeviceDetail,
  /** 获取支持的厂商列表 */
  getSupportedTopologyVendors: TaskExecutionUIServiceBinding.GetSupportedTopologyVendors,
  /** 获取运行产物索引 */
  getRunArtifacts: TaskExecutionUIServiceBinding.GetRunArtifacts,
  /** 获取拓扑采集计划快照 */
  getTopologyCollectionPlans:
    TaskExecutionUIServiceBinding.GetTopologyCollectionPlans,
  // 离线重放模式
  /** 从Raw文件重放拓扑构建 */
  replayTopologyFromRaw: TaskExecutionUIServiceBinding.ReplayTopologyFromRaw,
  /** 列出可重放的运行记录 */
  listReplayableRuns: TaskExecutionUIServiceBinding.ListReplayableRuns,
  /** 获取重放历史 */
  getReplayHistory: TaskExecutionUIServiceBinding.GetReplayHistory,
  /** 获取Raw文件预览 */
  getRawFilePreview: TaskExecutionUIServiceBinding.GetRawFilePreview,
  /** 列出指定设备的Raw文件 */
  listRawFiles: TaskExecutionUIServiceBinding.ListRawFiles,
  // 决策轨迹
  /** 获取边解释视图（包含候选和决策轨迹） */
  getTopologyEdgeExplain: TaskExecutionUIServiceBinding.GetTopologyEdgeExplain,
  /** 获取运行的所有决策轨迹 */
  getTopologyDecisionTracesByRun: TaskExecutionUIServiceBinding.GetTopologyDecisionTracesByRun,
} as const

// ==================== 拓扑命令 API ====================
/**
 * 拓扑命令配置 API
 * @description 提供字段目录、厂商默认命令查询/保存/重置能力
 */
export const TopologyCommandConfigAPI = {
  /** 获取支持的拓扑厂商列表 */
  getSupportedTopologyVendors: TopologyCommandServiceBinding.GetSupportedTopologyVendors,
  /** 获取拓扑字段目录 */
  getTopologyFieldCatalog: TopologyCommandServiceBinding.GetTopologyFieldCatalog,
  /** 获取厂商默认命令配置 */
  getVendorCommandConfig: TopologyCommandServiceBinding.GetVendorCommandConfig,
  /** 保存厂商默认命令配置 */
  saveVendorCommandConfig: TopologyCommandServiceBinding.SaveVendorCommandConfig,
  /** 重置厂商默认命令配置 */
  resetVendorCommandConfig: TopologyCommandServiceBinding.ResetVendorCommandConfig,
} as const

/**
 * 拓扑命令预览 API
 * @description 提供任务级拓扑命令解析预览
 */
export const TopologyCommandAPI = {
  /** 获取任务创建/编辑可选厂商列表（系统支持 + 资产厂商并集） */
  getTaskTopologyVendors: TopologyCommandServiceBinding.GetTaskTopologyVendors,
  /** 预览当前任务配置下的拓扑命令解析结果 */
  previewTopologyCommands: TopologyCommandServiceBinding.PreviewTopologyCommands,
} as const

// ==================== ConfigForge 服务 API ====================
/**
 * ConfigForge 服务 API
 * @description 提供配置生成、语法糖展开、IP验证等功能
 * 
 * @note 核心计算逻辑已迁移至后端，前端仅负责表单提交和结果渲染
 */
export const ForgeAPI = {
  /** 构建配置 */
  buildConfig: ForgeServiceBinding.BuildConfig,
  /** 展开变量值 */
  expandValues: ForgeServiceBinding.ExpandValues,
  /** 验证IP格式 */
  validateIP: ForgeServiceBinding.ValidateIP,
  /** 解析IP范围语法 */
  parseIPRange: ForgeServiceBinding.ParseIPRange,
  /** 批量验证IP */
  validateIPs: ForgeServiceBinding.ValidateIPs,
  /** 生成绑定模式预览 */
  generateBindingPreview: ForgeServiceBinding.GenerateBindingPreview,
  /** IPv4 子网计算（后端计算，前端只渲染） */
  calculateIPv4: ForgeServiceBinding.CalculateIPv4,
  /** 导出 IPv4 子网 CSV（后端生成） */
  exportIPv4SubnetsCSV: ForgeServiceBinding.ExportIPv4SubnetsCSV,
  /** IPv6 子网计算（后端计算，前端只渲染） */
  calculateIPv6: ForgeServiceBinding.CalculateIPv6,
} as const

// 导出 Forge 相关类型（从正确的 models 文件导入）
export type {
  BuildRequest,
  VarInput,
  BuildResult,
  ExpandRequest,
  ExpandResult,
} from '../bindings/github.com/NetWeaverGo/core/internal/forge/models.js'

export type {
  ForgeIPValidationResult,
  IPRangeResult,
  IPsValidationResult,
  BindingPreview,
  TaskCommandOverview,
  TaskDeviceOverview,
  TaskGroupDetailViewModel,
  TaskGroupItemDetailViewModel,
  TaskGroupListView,
} from '../bindings/github.com/NetWeaverGo/core/internal/ui/models.js'

// ExecutionSnapshot 从统一运行时导出
export type { ExecutionSnapshot, StageSnapshot, UnitSnapshot, EventSnapshot } from '../types/taskexec'

export interface TopologyBuildResult {
  taskId: string
  totalEdges: number
  confirmedEdges: number
  semiConfirmedEdges: number
  inferredEdges: number
  conflictEdges: number
  buildTime: number
  errors?: string[]
}

export type {
  DeviceAssetResponse,
} from '../bindings/github.com/NetWeaverGo/core/internal/models/models'

// ==================== 历史执行记录 API ====================
/**
 * 历史执行记录 API
 * @description 提供历史执行记录的查询和管理
 */
export const ExecutionHistoryAPI = {
  /** 从统一运行时查询历史记录（阶段5） */
  listTaskRunRecords: ExecutionHistoryServiceBinding.ListTaskRunRecords,
  /** 使用系统默认应用打开文件 */
  openFileWithDefaultApp: ExecutionHistoryServiceBinding.OpenFileWithDefaultApp,
  /** 删除单条运行记录 */
  deleteRunRecord: ExecutionHistoryServiceBinding.DeleteRunRecord,
  /** 删除所有运行记录 */
  deleteAllRunRecords: ExecutionHistoryServiceBinding.DeleteAllRunRecords,
  /** 获取任务设备执行详情 */
  getDeviceDetails: ExecutionHistoryServiceBinding.GetDeviceDetails,
  /** 获取任务报告路径 */
  getReportPath: ExecutionHistoryServiceBinding.GetReportPath,
  /** 打开文件所在文件夹并选中文件（安全版：通过 RunID+UnitID+FileType 解析路径） */
  openFileLocation: ExecutionHistoryServiceBinding.OpenFileLocation,
} as const

// 导出历史执行记录相关类型
export type {
  ListTaskRunRecordsRequest,
  ListTaskRunRecordsResponse,
  TaskRunRecordView,
  DeviceDetailsRequest,
  DeviceDetailsResponse,
  DeviceExecutionView,
  FileLocationRequest,
  FileLocationResponse,
  OpenFileWithDefaultAppRequest,
  ReportPathRequest,
  ReportPathResponse,
} from '../bindings/github.com/NetWeaverGo/core/internal/ui/models.js'

// ==================== 查询服务 API ====================
/**
 * 查询服务 API
 * @description 提供带条件的列表查询，后端处理过滤、排序、分页
 *
 * @note 所有过滤、分页逻辑已迁移至后端 QueryService
 */

/**
 * QueryAPI - 后端查询实现
 * @description 后端处理过滤、排序、分页，前端无需本地计算
 */
export const QueryAPI = {
  /** 查询设备列表（后端过滤） */
  listDevices: QueryServiceBinding.ListDevices,
  
  /** 查询任务组列表（后端过滤） */
  listTaskGroups: QueryServiceBinding.ListTaskGroups,
  
  /** 查询命令组列表（后端过滤） */
  listCommandGroups: QueryServiceBinding.ListCommandGroups,
  
  /** 获取所有设备分组名称（后端聚合） */
  getDeviceGroups: QueryServiceBinding.GetDeviceGroups,
  
  /** 获取所有设备标签（后端聚合） */
  getDeviceTags: QueryServiceBinding.GetDeviceTags,
} as const

// 导出 Query 相关类型（从 bindings 导入，保持类型来源唯一）
export type {
  QueryOptions,
  QueryResult,
} from '../bindings/github.com/NetWeaverGo/core/internal/ui/models.js'

// ==================== 规划比对 API ====================
/**
 * 规划比对 API
 * @description 提供 Excel 导入、拓扑比对、差异报告导出能力
 */
export const PlanCompareAPI = {
  /** 导入规划 Excel */
  importPlanExcel: PlanCompareServiceBinding.ImportPlanExcel,
  /** 列出规划文件 */
  listPlanFiles: PlanCompareServiceBinding.ListPlanFiles,
  /** 执行规划比对 */
  compare: PlanCompareServiceBinding.Compare,
  /** 获取报告摘要 */
  getDiffReport: PlanCompareServiceBinding.GetDiffReport,
  /** 获取报告明细 */
  getCompareResult: PlanCompareServiceBinding.GetCompareResult,
  /** 导出报告 */
  exportDiffReport: PlanCompareServiceBinding.ExportDiffReport,
} as const

// ==================== 类型导出 ====================
export type {
  DeviceAsset,
  GlobalSettings,
  CommandGroup,
  TaskGroup,
  TaskItem,
  SSHAlgorithmSettings,
  TopologyGraphView,
  TopologyEdgeDetailView,
  PlanImportResult,
  PlanUploadView,
  CompareResult,
  DiffReportView,
  DiffItem,
  TopologyTaskFieldOverride,
} from '../bindings/github.com/NetWeaverGo/core/internal/models/models.js'

export type {
  TopologyCommandPreviewView,
  TopologyCommandResolutionView,
  TopologyPreviewDeviceView,
  TopologyResolvedCommandView,
  TopologyVendorCommandItemView,
  TopologyVendorCommandSaveRequest,
  TopologyVendorCommandSetView,
} from '../bindings/github.com/NetWeaverGo/core/internal/ui/models.js'

export type {
  TaskArtifact,
  TopologyCollectionPlanArtifact,
  TopologyCollectionPlanCommand,
} from '../bindings/github.com/NetWeaverGo/core/internal/taskexec/models.js'

export type { ParsedResult } from '../bindings/github.com/NetWeaverGo/core/internal/parser/models.js'
