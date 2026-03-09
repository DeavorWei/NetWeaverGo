// 命令组类型定义
export interface CommandGroup {
  id: string;
  name: string;
  description: string;
  commands: string[];
  createdAt: string;
  updatedAt: string;
  tags: string[];
}

// 创建命令组请求（不含 id 和时间戳）
export interface CreateCommandGroupRequest {
  name: string;
  description: string;
  commands: string[];
  tags: string[];
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
