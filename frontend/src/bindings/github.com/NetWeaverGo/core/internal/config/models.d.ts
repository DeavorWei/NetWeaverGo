// Type declarations for models.js
// 自动生成的类型定义 - 与后端 Go 结构体保持一致

export interface DeviceAsset {
  ip: string;
  port: number;
  protocol: string;
  username: string;
  password: string;
  group: string;
  tags: string[];
}

export interface GlobalSettings {
  maxWorkers: number;
  connectTimeout: string;
  commandTimeout: string;
  outputDir: string;
  logDir: string;
  errorMode: string;
}

export interface CommandGroup {
  id: string;
  name: string;
  description: string;
  commands: string[];
  createdAt: string;
  updatedAt: string;
  tags: string[];
}

export class DeviceAsset {
  constructor(source?: Partial<DeviceAsset>);
  static createFrom(source?: any): DeviceAsset;
}

export class GlobalSettings {
  constructor(source?: Partial<GlobalSettings>);
  static createFrom(source?: any): GlobalSettings;
}
