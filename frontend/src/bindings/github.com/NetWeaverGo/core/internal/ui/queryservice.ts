// QueryService 查询服务绑定
// 手动创建的绑定文件，使用 Wails v3 runtime 调用方式

/**
 * QueryService 查询服务 - 提供带条件的列表查询
 * 后端处理过滤、排序、分页，前端无需本地计算
 * @module
 */

// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore: Unused imports
import {Call as $Call} from "@wailsio/runtime";

// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore: Unused imports
import * as config$0 from "../config/models.js";

// ==================== 类型定义 ====================

/** 查询选项 */
export interface QueryOptions {
  searchQuery: string;    // 搜索关键词
  filterField: string;    // 搜索字段 (如 group, ip, tag)
  filterValue: string;    // 过滤值
  page: number;           // 页码 (1-based)
  pageSize: number;       // 每页条数
  sortBy: string;         // 排序字段
  sortOrder: string;      // 排序方向: asc/desc
}

/** 查询结果 */
export interface QueryResult<T> {
  data: T[];              // 数据列表
  total: number;          // 总记录数
  page: number;           // 当前页码
  pageSize: number;       // 每页条数
  totalPages: number;     // 总页数
}

// ==================== 服务方法 ====================

// 使用服务名称动态调用（Wails v3 支持的方式）
const SERVICE_NAME = "github.com/NetWeaverGo/core/internal/ui.QueryService";

/**
 * ListDevices 查询设备列表（支持搜索、过滤、分页）
 * @param opts 查询选项
 * @returns 分页查询结果
 */
export function ListDevices(opts: QueryOptions): Promise<QueryResult<config$0.DeviceAsset>> & { cancel(): void } {
  return $Call.ByName(`${SERVICE_NAME}.ListDevices`, opts) as Promise<QueryResult<config$0.DeviceAsset>> & { cancel(): void };
}

/**
 * ListTaskGroups 查询任务组列表（支持搜索、过滤、分页）
 * @param opts 查询选项
 * @returns 分页查询结果
 */
export function ListTaskGroups(opts: QueryOptions): Promise<QueryResult<config$0.TaskGroup>> & { cancel(): void } {
  return $Call.ByName(`${SERVICE_NAME}.ListTaskGroups`, opts) as Promise<QueryResult<config$0.TaskGroup>> & { cancel(): void };
}

/**
 * ListCommandGroups 查询命令组列表（支持搜索、过滤、分页）
 * @param opts 查询选项
 * @returns 分页查询结果
 */
export function ListCommandGroups(opts: QueryOptions): Promise<QueryResult<config$0.CommandGroup>> & { cancel(): void } {
  return $Call.ByName(`${SERVICE_NAME}.ListCommandGroups`, opts) as Promise<QueryResult<config$0.CommandGroup>> & { cancel(): void };
}

/**
 * GetDeviceGroups 获取所有设备分组名称（用于前端下拉选项）
 * @returns 分组名称列表
 */
export function GetDeviceGroups(): Promise<string[]> & { cancel(): void } {
  return $Call.ByName(`${SERVICE_NAME}.GetDeviceGroups`) as Promise<string[]> & { cancel(): void };
}

/**
 * GetDeviceTags 获取所有设备标签（用于前端下拉选项）
 * @returns 标签列表
 */
export function GetDeviceTags(): Promise<string[]> & { cancel(): void } {
  return $Call.ByName(`${SERVICE_NAME}.GetDeviceTags`) as Promise<string[]> & { cancel(): void };
}
