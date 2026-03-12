// 命令组类型定义 - 从 api.ts 统一导出
export type { CommandGroup } from '../services/api'

// 创建命令组请求类型（使用 Partial 允许省略 id 和时间戳）
import type { CommandGroup as CommandGroupBase } from '../services/api'
export type CreateCommandGroupRequest = Partial<CommandGroupBase> & {
  name: string;
  commands: string[];
}

// 设备筛选方式
export type DeviceFilterType = "all" | "group" | "tag" | "protocol" | "manual";

// 设备筛选选项
export interface DeviceFilterOption {
  label: string;
  value: string;
  count?: number;
}

// 任务项（一组命令绑定一组设备）
export interface TaskItem {
  commandGroupId: string;
  commands: string[];
  deviceIPs: string[];
}

// 任务组
export interface TaskGroup {
  id: string;
  name: string;
  description: string;
  mode: "group" | "binding";
  items: TaskItem[];
  tags: string[];
  status: "pending" | "running" | "completed" | "failed";
  createdAt: string;
  updatedAt: string;
}
