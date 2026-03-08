// Type declarations for models.js
// 后端使用小写 json tag，但 Wails 绑定使用大写字段名

export interface DeviceAsset {
  IP: string;
  Port: number;
  Protocol: string;
  Username: string;
  Password: string;
  Group: string;
  Tag: string;
}

export interface GlobalSettings {
  MaxWorkers: number;
  ConnectTimeout: string;
  CommandTimeout: string;
  OutputDir: string;
  LogDir: string;
  ErrorMode: string;
}

export class DeviceAsset {
  constructor(source?: Partial<DeviceAsset>);
  static createFrom(source?: any): DeviceAsset;
}

export class GlobalSettings {
  constructor(source?: Partial<GlobalSettings>);
  static createFrom(source?: any): GlobalSettings;
}
