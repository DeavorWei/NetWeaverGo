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

// 导出 Query 相关类型
export type {
  QueryOptions as QueryOptionsBinding,
  QueryResult as QueryResultBinding,
} from '../bindings/github.com/NetWeaverGo/core/internal/ui/queryservice.js'

// ==================== 类型导出 ====================
export type { 
  DeviceAsset, 
  GlobalSettings,
  CommandGroup,
  TaskGroup,
  TaskItem
} from '../bindings/github.com/NetWeaverGo/core/internal/config/models.js'


// ==================== 向后兼容导出（Deprecated） ====================
// ⚠️ 警告：以下导出将在 v2.0 版本移除，请迁移到新的 API 命名空间
// 详见文档：docs/API_MIGRATION.md

// 开发环境显示弃用警告，生产环境静默
const isDev = typeof import.meta !== 'undefined' && import.meta.env?.DEV
const showDeprecationWarning = (oldApi: string, newApi: string): void => {
  if (isDev) {
    console.warn(`[NetWeaverGo 弃用警告] ${oldApi} 将在 v2.0 移除，请迁移至 ${newApi}。详见 docs/API_MIGRATION.md`)
  }
}

/**
 * @deprecated 将在 v2.0 移除，请使用 {@link DeviceAPI.listDevices}
 * @see {@link DeviceAPI.listDevices}
 * @example
 * // 迁移前
 * import { ListDevices } from '@/services/api'
 * const devices = await ListDevices()
 *
 * // 迁移后
 * import { DeviceAPI } from '@/services/api'
 * const devices = await DeviceAPI.listDevices()
 */
export const ListDevices = (...args: Parameters<typeof DeviceAPI.listDevices>) => {
  showDeprecationWarning('ListDevices', 'DeviceAPI.listDevices')
  return DeviceAPI.listDevices(...args)
}

/**
 * @deprecated 将在 v2.0 移除，请使用 {@link DeviceAPI.addDevice}
 * @see {@link DeviceAPI.addDevice}
 */
export const AddDevice = (...args: Parameters<typeof DeviceAPI.addDevice>) => {
  showDeprecationWarning('AddDevice', 'DeviceAPI.addDevice')
  return DeviceAPI.addDevice(...args)
}

/**
 * @deprecated 将在 v2.0 移除，请使用 {@link DeviceAPI.updateDevice}
 * @see {@link DeviceAPI.updateDevice}
 */
export const UpdateDevice = (...args: Parameters<typeof DeviceAPI.updateDevice>) => {
  showDeprecationWarning('UpdateDevice', 'DeviceAPI.updateDevice')
  return DeviceAPI.updateDevice(...args)
}

/**
 * @deprecated 将在 v2.0 移除，请使用 {@link DeviceAPI.deleteDevice}
 * @see {@link DeviceAPI.deleteDevice}
 */
export const DeleteDevice = (...args: Parameters<typeof DeviceAPI.deleteDevice>) => {
  showDeprecationWarning('DeleteDevice', 'DeviceAPI.deleteDevice')
  return DeviceAPI.deleteDevice(...args)
}

/**
 * @deprecated 将在 v2.0 移除，请使用 {@link DeviceAPI.saveDevices}
 * @see {@link DeviceAPI.saveDevices}
 */
export const SaveDevices = (...args: Parameters<typeof DeviceAPI.saveDevices>) => {
  showDeprecationWarning('SaveDevices', 'DeviceAPI.saveDevices')
  return DeviceAPI.saveDevices(...args)
}

/**
 * @deprecated 将在 v2.0 移除，请使用 {@link DeviceAPI.getProtocolDefaultPorts}
 * @see {@link DeviceAPI.getProtocolDefaultPorts}
 */
export const GetProtocolDefaultPorts = () => {
  showDeprecationWarning('GetProtocolDefaultPorts', 'DeviceAPI.getProtocolDefaultPorts')
  return DeviceAPI.getProtocolDefaultPorts()
}

/**
 * @deprecated 将在 v2.0 移除，请使用 {@link DeviceAPI.getValidProtocols}
 * @see {@link DeviceAPI.getValidProtocols}
 */
export const GetValidProtocols = () => {
  showDeprecationWarning('GetValidProtocols', 'DeviceAPI.getValidProtocols')
  return DeviceAPI.getValidProtocols()
}

/**
 * @deprecated 将在 v2.0 移除，请使用 {@link CommandGroupAPI.listCommandGroups}
 * @see {@link CommandGroupAPI.listCommandGroups}
 */
export const ListCommandGroups = (...args: Parameters<typeof CommandGroupAPI.listCommandGroups>) => {
  showDeprecationWarning('ListCommandGroups', 'CommandGroupAPI.listCommandGroups')
  return CommandGroupAPI.listCommandGroups(...args)
}

/**
 * @deprecated 将在 v2.0 移除，请使用 {@link CommandGroupAPI.getCommandGroup}
 * @see {@link CommandGroupAPI.getCommandGroup}
 */
export const GetCommandGroup = (...args: Parameters<typeof CommandGroupAPI.getCommandGroup>) => {
  showDeprecationWarning('GetCommandGroup', 'CommandGroupAPI.getCommandGroup')
  return CommandGroupAPI.getCommandGroup(...args)
}

/**
 * @deprecated 将在 v2.0 移除，请使用 {@link CommandGroupAPI.createCommandGroup}
 * @see {@link CommandGroupAPI.createCommandGroup}
 */
export const CreateCommandGroup = (...args: Parameters<typeof CommandGroupAPI.createCommandGroup>) => {
  showDeprecationWarning('CreateCommandGroup', 'CommandGroupAPI.createCommandGroup')
  return CommandGroupAPI.createCommandGroup(...args)
}

/**
 * @deprecated 将在 v2.0 移除，请使用 {@link CommandGroupAPI.updateCommandGroup}
 * @see {@link CommandGroupAPI.updateCommandGroup}
 */
export const UpdateCommandGroup = (...args: Parameters<typeof CommandGroupAPI.updateCommandGroup>) => {
  showDeprecationWarning('UpdateCommandGroup', 'CommandGroupAPI.updateCommandGroup')
  return CommandGroupAPI.updateCommandGroup(...args)
}

/**
 * @deprecated 将在 v2.0 移除，请使用 {@link CommandGroupAPI.deleteCommandGroup}
 * @see {@link CommandGroupAPI.deleteCommandGroup}
 */
export const DeleteCommandGroup = (...args: Parameters<typeof CommandGroupAPI.deleteCommandGroup>) => {
  showDeprecationWarning('DeleteCommandGroup', 'CommandGroupAPI.deleteCommandGroup')
  return CommandGroupAPI.deleteCommandGroup(...args)
}

/**
 * @deprecated 将在 v2.0 移除，请使用 {@link CommandGroupAPI.duplicateCommandGroup}
 * @see {@link CommandGroupAPI.duplicateCommandGroup}
 */
export const DuplicateCommandGroup = (...args: Parameters<typeof CommandGroupAPI.duplicateCommandGroup>) => {
  showDeprecationWarning('DuplicateCommandGroup', 'CommandGroupAPI.duplicateCommandGroup')
  return CommandGroupAPI.duplicateCommandGroup(...args)
}

/**
 * @deprecated 将在 v2.0 移除，请使用 {@link CommandGroupAPI.importCommandGroup}
 * @see {@link CommandGroupAPI.importCommandGroup}
 */
export const ImportCommandGroup = (...args: Parameters<typeof CommandGroupAPI.importCommandGroup>) => {
  showDeprecationWarning('ImportCommandGroup', 'CommandGroupAPI.importCommandGroup')
  return CommandGroupAPI.importCommandGroup(...args)
}

/**
 * @deprecated 将在 v2.0 移除，请使用 {@link CommandGroupAPI.exportCommandGroup}
 * @see {@link CommandGroupAPI.exportCommandGroup}
 */
export const ExportCommandGroup = (...args: Parameters<typeof CommandGroupAPI.exportCommandGroup>) => {
  showDeprecationWarning('ExportCommandGroup', 'CommandGroupAPI.exportCommandGroup')
  return CommandGroupAPI.exportCommandGroup(...args)
}

/**
 * @deprecated 将在 v2.0 移除，请使用 {@link CommandGroupAPI.getCommands}
 * @see {@link CommandGroupAPI.getCommands}
 */
export const GetCommands = (...args: Parameters<typeof CommandGroupAPI.getCommands>) => {
  showDeprecationWarning('GetCommands', 'CommandGroupAPI.getCommands')
  return CommandGroupAPI.getCommands(...args)
}

/**
 * @deprecated 将在 v2.0 移除，请使用 {@link CommandGroupAPI.saveCommands}
 * @see {@link CommandGroupAPI.saveCommands}
 */
export const SaveCommands = (...args: Parameters<typeof CommandGroupAPI.saveCommands>) => {
  showDeprecationWarning('SaveCommands', 'CommandGroupAPI.saveCommands')
  return CommandGroupAPI.saveCommands(...args)
}

/**
 * @deprecated 将在 v2.0 移除，请使用 {@link SettingsAPI.loadSettings}
 * @see {@link SettingsAPI.loadSettings}
 */
export const LoadSettings = (...args: Parameters<typeof SettingsAPI.loadSettings>) => {
  showDeprecationWarning('LoadSettings', 'SettingsAPI.loadSettings')
  return SettingsAPI.loadSettings(...args)
}

/**
 * @deprecated 将在 v2.0 移除，请使用 {@link SettingsAPI.saveSettings}
 * @see {@link SettingsAPI.saveSettings}
 */
export const SaveSettings = (...args: Parameters<typeof SettingsAPI.saveSettings>) => {
  showDeprecationWarning('SaveSettings', 'SettingsAPI.saveSettings')
  return SettingsAPI.saveSettings(...args)
}

/**
 * @deprecated 将在 v2.0 移除，请使用 {@link SettingsAPI.ensureConfig}
 * @see {@link SettingsAPI.ensureConfig}
 */
export const EnsureConfig = (...args: Parameters<typeof SettingsAPI.ensureConfig>) => {
  showDeprecationWarning('EnsureConfig', 'SettingsAPI.ensureConfig')
  return SettingsAPI.ensureConfig(...args)
}

/**
 * @deprecated 将在 v2.0 移除，请使用 {@link SettingsAPI.getAppInfo}
 * @see {@link SettingsAPI.getAppInfo}
 */
export const GetAppInfo = (...args: Parameters<typeof SettingsAPI.getAppInfo>) => {
  showDeprecationWarning('GetAppInfo', 'SettingsAPI.getAppInfo')
  return SettingsAPI.getAppInfo(...args)
}

/**
 * @deprecated 将在 v2.0 移除，请使用 {@link SettingsAPI.logInfo}
 * @see {@link SettingsAPI.logInfo}
 */
export const LogInfo = (...args: Parameters<typeof SettingsAPI.logInfo>) => {
  showDeprecationWarning('LogInfo', 'SettingsAPI.logInfo')
  return SettingsAPI.logInfo(...args)
}

/**
 * @deprecated 将在 v2.0 移除，请使用 {@link SettingsAPI.logWarn}
 * @see {@link SettingsAPI.logWarn}
 */
export const LogWarn = (...args: Parameters<typeof SettingsAPI.logWarn>) => {
  showDeprecationWarning('LogWarn', 'SettingsAPI.logWarn')
  return SettingsAPI.logWarn(...args)
}

/**
 * @deprecated 将在 v2.0 移除，请使用 {@link SettingsAPI.logError}
 * @see {@link SettingsAPI.logError}
 */
export const LogError = (...args: Parameters<typeof SettingsAPI.logError>) => {
  showDeprecationWarning('LogError', 'SettingsAPI.logError')
  return SettingsAPI.logError(...args)
}

/**
 * @deprecated 将在 v2.0 移除，请使用 {@link EngineAPI.startEngine}
 * @see {@link EngineAPI.startEngine}
 */
export const StartEngine = (...args: Parameters<typeof EngineAPI.startEngine>) => {
  showDeprecationWarning('StartEngine', 'EngineAPI.startEngine')
  return EngineAPI.startEngine(...args)
}

/**
 * @deprecated 将在 v2.0 移除，请使用 {@link EngineAPI.startEngineWithSelection}
 * @see {@link EngineAPI.startEngineWithSelection}
 */
export const StartEngineWithSelection = (...args: Parameters<typeof EngineAPI.startEngineWithSelection>) => {
  showDeprecationWarning('StartEngineWithSelection', 'EngineAPI.startEngineWithSelection')
  return EngineAPI.startEngineWithSelection(...args)
}

/**
 * @deprecated 将在 v2.0 移除，请使用 {@link EngineAPI.startBackup}
 * @see {@link EngineAPI.startBackup}
 */
export const StartBackup = (...args: Parameters<typeof EngineAPI.startBackup>) => {
  showDeprecationWarning('StartBackup', 'EngineAPI.startBackup')
  return EngineAPI.startBackup(...args)
}

/**
 * @deprecated 将在 v2.0 移除，请使用 {@link EngineAPI.resolveSuspend}
 * @see {@link EngineAPI.resolveSuspend}
 */
export const ResolveSuspend = (...args: Parameters<typeof EngineAPI.resolveSuspend>) => {
  showDeprecationWarning('ResolveSuspend', 'EngineAPI.resolveSuspend')
  return EngineAPI.resolveSuspend(...args)
}

/**
 * @deprecated 将在 v2.0 移除，请使用 {@link EngineAPI.isRunning}
 * @see {@link EngineAPI.isRunning}
 */
export const IsRunning = (...args: Parameters<typeof EngineAPI.isRunning>) => {
  showDeprecationWarning('IsRunning', 'EngineAPI.isRunning')
  return EngineAPI.isRunning(...args)
}

/**
 * @deprecated 将在 v2.0 移除，请使用 {@link TaskGroupAPI.listTaskGroups}
 * @see {@link TaskGroupAPI.listTaskGroups}
 */
export const ListTaskGroups = (...args: Parameters<typeof TaskGroupAPI.listTaskGroups>) => {
  showDeprecationWarning('ListTaskGroups', 'TaskGroupAPI.listTaskGroups')
  return TaskGroupAPI.listTaskGroups(...args)
}

/**
 * @deprecated 将在 v2.0 移除，请使用 {@link TaskGroupAPI.getTaskGroup}
 * @see {@link TaskGroupAPI.getTaskGroup}
 */
export const GetTaskGroup = (...args: Parameters<typeof TaskGroupAPI.getTaskGroup>) => {
  showDeprecationWarning('GetTaskGroup', 'TaskGroupAPI.getTaskGroup')
  return TaskGroupAPI.getTaskGroup(...args)
}

/**
 * @deprecated 将在 v2.0 移除，请使用 {@link TaskGroupAPI.createTaskGroup}
 * @see {@link TaskGroupAPI.createTaskGroup}
 */
export const CreateTaskGroup = (...args: Parameters<typeof TaskGroupAPI.createTaskGroup>) => {
  showDeprecationWarning('CreateTaskGroup', 'TaskGroupAPI.createTaskGroup')
  return TaskGroupAPI.createTaskGroup(...args)
}

/**
 * @deprecated 将在 v2.0 移除，请使用 {@link TaskGroupAPI.updateTaskGroup}
 * @see {@link TaskGroupAPI.updateTaskGroup}
 */
export const UpdateTaskGroup = (...args: Parameters<typeof TaskGroupAPI.updateTaskGroup>) => {
  showDeprecationWarning('UpdateTaskGroup', 'TaskGroupAPI.updateTaskGroup')
  return TaskGroupAPI.updateTaskGroup(...args)
}

/**
 * @deprecated 将在 v2.0 移除，请使用 {@link TaskGroupAPI.deleteTaskGroup}
 * @see {@link TaskGroupAPI.deleteTaskGroup}
 */
export const DeleteTaskGroup = (...args: Parameters<typeof TaskGroupAPI.deleteTaskGroup>) => {
  showDeprecationWarning('DeleteTaskGroup', 'TaskGroupAPI.deleteTaskGroup')
  return TaskGroupAPI.deleteTaskGroup(...args)
}

/**
 * @deprecated 将在 v2.0 移除，请使用 {@link TaskGroupAPI.startTaskGroup}
 * @see {@link TaskGroupAPI.startTaskGroup}
 */
export const StartTaskGroup = (...args: Parameters<typeof TaskGroupAPI.startTaskGroup>) => {
  showDeprecationWarning('StartTaskGroup', 'TaskGroupAPI.startTaskGroup')
  return TaskGroupAPI.startTaskGroup(...args)
}

// ==================== v2.0 移除警告 ====================
// 以上所有向后兼容导出将在 v2.0 版本完全移除
// 请尽快迁移到新的命名空间 API
// 迁移指南详见：docs/API_MIGRATION.md
