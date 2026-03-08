// Type declarations for appservice.js
import { DeviceAsset, GlobalSettings } from '../config/models';

export function EnsureConfig(): Promise<[DeviceAsset[], string[], string[]]>;
export function LoadSettings(): Promise<GlobalSettings | null>;
export function ResolveSuspend(ip: string, action: string): Promise<void>;
export function StartEngineWails(): Promise<void>;
export function StartBackupWails(): Promise<void>;
export function GetCommands(): Promise<string[]>;
export function SaveCommands(commands: string[]): Promise<void>;
export function ListDevices(): Promise<DeviceAsset[]>;
export function AddDevice(device: DeviceAsset): Promise<void>;
export function UpdateDevice(index: number, device: DeviceAsset): Promise<void>;
export function DeleteDevice(index: number): Promise<void>;
export function SaveDevices(devices: DeviceAsset[]): Promise<void>;
export function GetProtocolDefaultPorts(): Promise<Record<string, number>>;
export function GetValidProtocols(): Promise<string[]>;

// CommandGroup APIs
export interface CommandGroup {
  id: string;
  name: string;
  description: string;
  commands: string[];
  createdAt: string;
  updatedAt: string;
  tags: string[];
}

// 创建/更新命令组请求（不含 id 和时间戳）
export interface CommandGroupRequest {
  name: string;
  description: string;
  commands: string[];
  tags: string[];
}

export function ListCommandGroups(): Promise<CommandGroup[]>;
export function GetCommandGroup(id: string): Promise<CommandGroup>;
export function CreateCommandGroup(group: CommandGroupRequest): Promise<CommandGroup>;
export function UpdateCommandGroup(id: string, group: CommandGroupRequest): Promise<CommandGroup>;
export function DeleteCommandGroup(id: string): Promise<void>;
export function DuplicateCommandGroup(id: string): Promise<CommandGroup>;
export function ImportCommandGroup(filePath: string): Promise<CommandGroup>;
export function ExportCommandGroup(id: string, filePath: string): Promise<void>;
export function StartEngineWithSelection(deviceIPs: string[], commandGroupID: string): Promise<void>;
