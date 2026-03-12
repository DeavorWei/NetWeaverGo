// ConfigForge 服务绑定
// 手动创建的绑定文件，使用 Wails v3 runtime 调用方式

/**
 * ForgeService 配置构建服务 - 负责配置生成、语法糖展开、IP验证
 * @module
 */

// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore: Unused imports
import {Call as $Call} from "@wailsio/runtime";

// ==================== 类型定义 ====================

/** 构建请求 */
export interface BuildRequest {
  template: string;
  variables: VarInput[];
}

/** 变量输入 */
export interface VarInput {
  name: string;
  valueString: string;
}

/** 构建结果 */
export interface BuildResult {
  blocks: string[];
  total: number;
  warnings: string[];
}

/** 展开请求 */
export interface ExpandRequest {
  valueString: string;
  maxLen: number;
}

/** 展开结果 */
export interface ExpandResult {
  values: string[];
  originalLen: number;
  expandedLen: number;
  hasExpanded: boolean;
  hasInferred: boolean;
  warnings: string[];
}

/** IP验证结果 */
export interface ForgeIPValidationResult {
  isValid: boolean;
  type: string;
  message: string;
}

/** IP范围解析结果 */
export interface IPRangeResult {
  isValid: boolean;
  start: string;
  end: string;
  count: number;
  list: string[];
  message: string;
}

/** 批量IP验证结果 */
export interface IPsValidationResult {
  validCount: number;
  invalidCount: number;
  validIPs: string[];
  invalidIPs: string[];
}

/** 绑定预览结果 */
export interface BindingPreview {
  ip: string;
  commands: string;
}

// ==================== 服务方法 ====================

// 使用服务名称动态调用（Wails v3 支持的方式）
const SERVICE_NAME = "github.com/NetWeaverGo/core/internal/ui.ForgeService";

/**
 * BuildConfig 构建配置
 */
export function BuildConfig(req: BuildRequest): Promise<BuildResult> & { cancel(): void } {
  return $Call.ByName(`${SERVICE_NAME}.BuildConfig`, req) as Promise<BuildResult> & { cancel(): void };
}

/**
 * ExpandValues 展开变量值
 */
export function ExpandValues(req: ExpandRequest): Promise<ExpandResult> & { cancel(): void } {
  return $Call.ByName(`${SERVICE_NAME}.ExpandValues`, req) as Promise<ExpandResult> & { cancel(): void };
}

/**
 * ValidateIP 验证IP格式
 */
export function ValidateIP(ip: string): Promise<ForgeIPValidationResult> & { cancel(): void } {
  return $Call.ByName(`${SERVICE_NAME}.ValidateIP`, ip) as Promise<ForgeIPValidationResult> & { cancel(): void };
}

/**
 * ParseIPRange 解析IP范围语法
 */
export function ParseIPRange(ipRange: string): Promise<IPRangeResult> & { cancel(): void } {
  return $Call.ByName(`${SERVICE_NAME}.ParseIPRange`, ipRange) as Promise<IPRangeResult> & { cancel(): void };
}

/**
 * ValidateIPs 批量验证IP
 */
export function ValidateIPs(ipString: string): Promise<IPsValidationResult> & { cancel(): void } {
  return $Call.ByName(`${SERVICE_NAME}.ValidateIPs`, ipString) as Promise<IPsValidationResult> & { cancel(): void };
}

/**
 * DetectBindingMode 检测是否为绑定模式
 */
export function DetectBindingMode(template: string): Promise<boolean> & { cancel(): void } {
  return $Call.ByName(`${SERVICE_NAME}.DetectBindingMode`, template) as Promise<boolean> & { cancel(): void };
}

/**
 * GenerateBindingPreview 生成绑定模式预览
 */
export function GenerateBindingPreview(template: string, variables: VarInput[]): Promise<BindingPreview[]> & { cancel(): void } {
  return $Call.ByName(`${SERVICE_NAME}.GenerateBindingPreview`, template, variables) as Promise<BindingPreview[]> & { cancel(): void };
}
