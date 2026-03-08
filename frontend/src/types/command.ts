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
export type DeviceFilterType = 'all' | 'group' | 'tag' | 'protocol' | 'manual';

// 设备筛选选项
export interface DeviceFilterOption {
  label: string;
  value: string;
  count?: number;
}
