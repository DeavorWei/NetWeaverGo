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
  /** 更新设备 */
  updateDevice: DeviceServiceBinding.UpdateDevice,
  /** 删除设备 */
  deleteDevice: DeviceServiceBinding.DeleteDevice,
  /** 批量保存设备列表 */
  saveDevices: DeviceServiceBinding.SaveDevices,
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
} as const

// 导出 Forge 相关类型
export type {
  BuildRequest,
  VarInput,
  BuildResult,
  ExpandRequest,
  ExpandResult,
  ForgeIPValidationResult,
  IPRangeResult,
  IPsValidationResult,
  BindingPreview,
} from '../bindings/github.com/NetWeaverGo/core/internal/ui/forgeservice.js'

// ==================== 查询服务 API ====================
/**
 * 查询服务 API
 * @description 提供带条件的列表查询，后端处理过滤、排序、分页
 * 
 * @note 目前使用前端过滤，后续可迁移至后端 QueryService
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
 * QueryAPI - 前端过滤实现
 * @note 后续可通过后端 QueryService 优化大数据量场景
 */
export const QueryAPI = {
  /** 查询设备列表（当前使用前端过滤） */
  listDevices: async (_opts: QueryOptions): Promise<QueryResult<DeviceAsset>> => {
    const devices = await DeviceAPI.listDevices()
    // TODO: 实现后端过滤后移除此逻辑
    return {
      data: devices,
      total: devices.length,
      page: _opts.page || 1,
      pageSize: _opts.pageSize || 10,
      totalPages: Math.ceil(devices.length / (_opts.pageSize || 10))
    }
  },
  /** 查询任务组列表（当前使用前端过滤） */
  listTaskGroups: async (_opts: QueryOptions): Promise<QueryResult<TaskGroup>> => {
    const groups = await TaskGroupAPI.listTaskGroups()
    return {
      data: groups,
      total: groups.length,
      page: _opts.page || 1,
      pageSize: _opts.pageSize || 10,
      totalPages: Math.ceil(groups.length / (_opts.pageSize || 10))
    }
  },
  /** 查询命令组列表（当前使用前端过滤） */
  listCommandGroups: async (_opts: QueryOptions): Promise<QueryResult<CommandGroup>> => {
    const groups = await CommandGroupAPI.listCommandGroups()
    return {
      data: groups,
      total: groups.length,
      page: _opts.page || 1,
      pageSize: _opts.pageSize || 10,
      totalPages: Math.ceil(groups.length / (_opts.pageSize || 10))
    }
  },
  /** 获取所有设备分组名称（用于下拉选项） */
  getDeviceGroups: async (): Promise<string[]> => {
    const devices = await DeviceAPI.listDevices()
    const groupSet = new Set<string>()
    devices.forEach(d => { if (d.group) groupSet.add(d.group) })
    return Array.from(groupSet).sort()
  },
  /** 获取所有设备标签（用于下拉选项） */
  getDeviceTags: async (): Promise<string[]> => {
    const devices = await DeviceAPI.listDevices()
    const tagSet = new Set<string>()
    devices.forEach(d => d.tags?.forEach(t => { if (t) tagSet.add(t) }))
    return Array.from(tagSet).sort()
  },
} as const

// ==================== 类型导出 ====================
export type { 
  DeviceAsset, 
  GlobalSettings,
  CommandGroup,
  TaskGroup,
  TaskItem
} from '../bindings/github.com/NetWeaverGo/core/internal/config/models.js'

// 导入类型用于 QueryAPI
import type { DeviceAsset, CommandGroup, TaskGroup } from '../bindings/github.com/NetWeaverGo/core/internal/config/models.js'

// ==================== 向后兼容导出（Deprecated） ====================
/**
 * @deprecated 请使用 DeviceAPI.listDevices
 * 向后兼容导出，将在 v2.0 版本移除
 */
export const ListDevices = DeviceAPI.listDevices
/** @deprecated 请使用 DeviceAPI.addDevice */
export const AddDevice = DeviceAPI.addDevice
/** @deprecated 请使用 DeviceAPI.updateDevice */
export const UpdateDevice = DeviceAPI.updateDevice
/** @deprecated 请使用 DeviceAPI.deleteDevice */
export const DeleteDevice = DeviceAPI.deleteDevice
/** @deprecated 请使用 DeviceAPI.saveDevices */
export const SaveDevices = DeviceAPI.saveDevices
/** @deprecated 请使用 DeviceAPI.getProtocolDefaultPorts */
export const GetProtocolDefaultPorts = DeviceAPI.getProtocolDefaultPorts
/** @deprecated 请使用 DeviceAPI.getValidProtocols */
export const GetValidProtocols = DeviceAPI.getValidProtocols

/** @deprecated 请使用 CommandGroupAPI.listCommandGroups */
export const ListCommandGroups = CommandGroupAPI.listCommandGroups
/** @deprecated 请使用 CommandGroupAPI.getCommandGroup */
export const GetCommandGroup = CommandGroupAPI.getCommandGroup
/** @deprecated 请使用 CommandGroupAPI.createCommandGroup */
export const CreateCommandGroup = CommandGroupAPI.createCommandGroup
/** @deprecated 请使用 CommandGroupAPI.updateCommandGroup */
export const UpdateCommandGroup = CommandGroupAPI.updateCommandGroup
/** @deprecated 请使用 CommandGroupAPI.deleteCommandGroup */
export const DeleteCommandGroup = CommandGroupAPI.deleteCommandGroup
/** @deprecated 请使用 CommandGroupAPI.duplicateCommandGroup */
export const DuplicateCommandGroup = CommandGroupAPI.duplicateCommandGroup
/** @deprecated 请使用 CommandGroupAPI.importCommandGroup */
export const ImportCommandGroup = CommandGroupAPI.importCommandGroup
/** @deprecated 请使用 CommandGroupAPI.exportCommandGroup */
export const ExportCommandGroup = CommandGroupAPI.exportCommandGroup
/** @deprecated 请使用 CommandGroupAPI.getCommands */
export const GetCommands = CommandGroupAPI.getCommands
/** @deprecated 请使用 CommandGroupAPI.saveCommands */
export const SaveCommands = CommandGroupAPI.saveCommands

/** @deprecated 请使用 SettingsAPI.loadSettings */
export const LoadSettings = SettingsAPI.loadSettings
/** @deprecated 请使用 SettingsAPI.saveSettings */
export const SaveSettings = SettingsAPI.saveSettings
/** @deprecated 请使用 SettingsAPI.ensureConfig */
export const EnsureConfig = SettingsAPI.ensureConfig
/** @deprecated 请使用 SettingsAPI.getAppInfo */
export const GetAppInfo = SettingsAPI.getAppInfo
/** @deprecated 请使用 SettingsAPI.logInfo */
export const LogInfo = SettingsAPI.logInfo
/** @deprecated 请使用 SettingsAPI.logWarn */
export const LogWarn = SettingsAPI.logWarn
/** @deprecated 请使用 SettingsAPI.logError */
export const LogError = SettingsAPI.logError

/** @deprecated 请使用 EngineAPI.startEngine */
export const StartEngine = EngineAPI.startEngine
/** @deprecated 请使用 EngineAPI.startEngineWithSelection */
export const StartEngineWithSelection = EngineAPI.startEngineWithSelection
/** @deprecated 请使用 EngineAPI.startBackup */
export const StartBackup = EngineAPI.startBackup
/** @deprecated 请使用 EngineAPI.resolveSuspend */
export const ResolveSuspend = EngineAPI.resolveSuspend
/** @deprecated 请使用 EngineAPI.isRunning */
export const IsRunning = EngineAPI.isRunning

/** @deprecated 请使用 TaskGroupAPI.listTaskGroups */
export const ListTaskGroups = TaskGroupAPI.listTaskGroups
/** @deprecated 请使用 TaskGroupAPI.getTaskGroup */
export const GetTaskGroup = TaskGroupAPI.getTaskGroup
/** @deprecated 请使用 TaskGroupAPI.createTaskGroup */
export const CreateTaskGroup = TaskGroupAPI.createTaskGroup
/** @deprecated 请使用 TaskGroupAPI.updateTaskGroup */
export const UpdateTaskGroup = TaskGroupAPI.updateTaskGroup
/** @deprecated 请使用 TaskGroupAPI.deleteTaskGroup */
export const DeleteTaskGroup = TaskGroupAPI.deleteTaskGroup
/** @deprecated 请使用 TaskGroupAPI.startTaskGroup */
export const StartTaskGroup = TaskGroupAPI.startTaskGroup
