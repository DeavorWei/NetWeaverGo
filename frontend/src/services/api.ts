/**
 * 统一的后端 API 导出文件
 * 
 * 采用命名空间模式组织，提供类型安全的 API 调用
 * 每个命名空间对应后端的一个独立服务
 * 
 * @example
 * ```ts
 * import { DeviceAPI, EngineAPI } from './services/api'
 * 
 * // 获取设备列表
 * const devices = await DeviceAPI.listDevices()
 * 
 * // 启动引擎
 * await EngineAPI.startEngine()
 * ```
 */

// ==================== 服务命名空间导入 ====================
import * as DeviceServiceBinding from '../bindings/github.com/NetWeaverGo/core/internal/ui/deviceservice.js'
import * as CommandGroupServiceBinding from '../bindings/github.com/NetWeaverGo/core/internal/ui/commandgroupservice.js'
import * as SettingsServiceBinding from '../bindings/github.com/NetWeaverGo/core/internal/ui/settingsservice.js'
import * as EngineServiceBinding from '../bindings/github.com/NetWeaverGo/core/internal/ui/engineservice.js'
import * as TaskGroupServiceBinding from '../bindings/github.com/NetWeaverGo/core/internal/ui/taskgroupservice.js'
import * as ForgeServiceBinding from '../bindings/github.com/NetWeaverGo/core/internal/ui/forgeservice.js'
import * as QueryServiceBinding from '../bindings/github.com/NetWeaverGo/core/internal/ui/queryservice.js'

// ==================== 设备管理 API ====================
/**
 * 设备管理 API
 * @description 提供设备的增删改查操作
 */
export const DeviceAPI = {
  /** 获取设备列表 */
  listDevices: DeviceServiceBinding.ListDevices,
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
  /** 获取命令列表 */
  getCommands: CommandGroupServiceBinding.GetCommands,
  /** 保存命令列表 */
  saveCommands: CommandGroupServiceBinding.SaveCommands,
} as const

// ==================== 设置管理 API ====================
/**
 * 设置管理 API
 * @description 提供应用配置的加载和保存操作
 */
export const SettingsAPI = {
  /** 加载设置 */
  loadSettings: SettingsServiceBinding.LoadSettings,
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
} as const

// ==================== 引擎控制 API ====================
/**
 * 引擎控制 API
 * @description 负责任务执行、状态管理和挂起处理
 */
export const EngineAPI = {
  /** 启动引擎（使用默认配置文件） */
  startEngine: EngineServiceBinding.StartEngine,
  /** 使用选定的设备和命令组启动引擎 */
  startEngineWithSelection: EngineServiceBinding.StartEngineWithSelection,
  /** 启动备份模式 */
  startBackup: EngineServiceBinding.StartBackup,
  /** 解除挂起状态（用户选择操作后调用） */
  resolveSuspend: EngineServiceBinding.ResolveSuspend,
  /** 检查引擎是否正在运行 */
  isRunning: EngineServiceBinding.IsRunning,
  /** 停止当前执行 */
  stopEngine: EngineServiceBinding.StopEngine,
  /** 获取引擎状态 */
  getEngineState: EngineServiceBinding.GetEngineState,
  /** 获取执行快照 */
  getExecutionSnapshot: EngineServiceBinding.GetExecutionSnapshot,
} as const

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
  /** 创建任务组 */
  createTaskGroup: TaskGroupServiceBinding.CreateTaskGroup,
  /** 更新任务组 */
  updateTaskGroup: TaskGroupServiceBinding.UpdateTaskGroup,
  /** 删除任务组 */
  deleteTaskGroup: TaskGroupServiceBinding.DeleteTaskGroup,
  /** 启动任务组执行 */
  startTaskGroup: TaskGroupServiceBinding.StartTaskGroup,
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
  /** 检测是否为绑定模式 */
  detectBindingMode: ForgeServiceBinding.DetectBindingMode,
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
} from '../bindings/github.com/NetWeaverGo/core/internal/ui/models.js'

export type {
  DeviceViewState,
  ExecutionSnapshot,
} from '../bindings/github.com/NetWeaverGo/core/internal/report/models.js'

// ==================== 查询服务 API ====================
/**
 * 查询服务 API
 * @description 提供带条件的列表查询，后端处理过滤、排序、分页
 * 
 * @note 所有过滤、分页逻辑已迁移至后端 QueryService
 */

/** 查询选项类型 */
export interface QueryOptions {
  searchQuery?: string
  filterField?: string
  filterValue?: string
  page?: number
  pageSize?: number
  sortBy?: string
  sortOrder?: 'asc' | 'desc'
}

/** 查询结果类型 */
export interface QueryResult<T> {
  data: T[]
  total: number
  page: number
  pageSize: number
  totalPages: number
}

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

// 导出 Query 相关类型（从正确的 models 文件导入）
export type {
  QueryOptions as QueryOptionsBinding,
  QueryResult as QueryResultBinding,
} from '../bindings/github.com/NetWeaverGo/core/internal/ui/models.js'

// ==================== 类型导出 ====================
export type { 
  DeviceAsset, 
  GlobalSettings,
  CommandGroup,
  TaskGroup,
  TaskItem
} from '../bindings/github.com/NetWeaverGo/core/internal/config/models.js'
