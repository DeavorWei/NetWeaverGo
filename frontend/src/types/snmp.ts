/**
 * SNMP 即时查询相关类型定义
 *
 * 定位：交付现场的设备上线确认与资产信息采集。
 * 类型直接复用 Wails 生成的绑定模型，避免与后端结构脱节。
 */
import type {
  SNMPBatchQueryRequest,
  SNMPBatchQueryResponse,
  SNMPBatchResultVM,
  SNMPCredentialInputVM,
  SNMPCredentialVM,
  SNMPQueryRequest,
  SNMPQueryResponse,
  SNMPResultVM,
} from "@/bindings/github.com/NetWeaverGo/core/internal/ui/models";

export type {
  SNMPBatchQueryRequest,
  SNMPBatchQueryResponse,
  SNMPBatchResultVM,
  SNMPCredentialInputVM,
  SNMPCredentialVM,
  SNMPQueryRequest,
  SNMPQueryResponse,
  SNMPResultVM,
};

/** SNMP 查询操作类型 */
export type SNMPOperation = "device_info" | "get" | "walk";

/** 查询操作选项 */
export const SNMP_OPERATION_OPTIONS: { value: SNMPOperation; label: string }[] = [
  { value: "device_info", label: "设备信息（sysDescr / sysUpTime / sysName）" },
  { value: "get", label: "GET（指定 OID）" },
  { value: "walk", label: "WALK（指定根 OID）" },
];

/** SNMP 版本选项 */
export const SNMP_VERSION_OPTIONS = [
  { value: "v1", label: "v1" },
  { value: "v2c", label: "v2c" },
  { value: "v3", label: "v3" },
];

/** v3 安全级别选项 */
export const SNMP_SECURITY_LEVEL_OPTIONS = [
  { value: "noAuthNoPriv", label: "noAuthNoPriv（无认证无加密）" },
  { value: "authNoPriv", label: "authNoPriv（认证不加密）" },
  { value: "authPriv", label: "authPriv（认证且加密）" },
];

/** v3 认证协议选项 */
export const SNMP_AUTH_PROTOCOL_OPTIONS = ["MD5", "SHA", "SHA224", "SHA256", "SHA384", "SHA512"];

/** v3 加密协议选项 */
export const SNMP_PRIV_PROTOCOL_OPTIONS = ["DES", "AES", "AES192", "AES256", "AES192C", "AES256C"];

/** 创建空白临时凭据 */
export function createEmptyCredentialInput(): SNMPCredentialInputVM {
  return {
    version: "v2c",
    community: "public",
    securityLevel: "authNoPriv",
    username: "",
    authProtocol: "SHA",
    authPassword: "",
    privProtocol: "AES",
    privPassword: "",
    contextName: "",
    contextEngineId: "",
  };
}

/** 创建空白凭据表单（用于新增/编辑） */
export function createEmptyCredentialForm(): SNMPCredentialVM {
  return {
    id: 0,
    name: "",
    version: "v2c",
    community: "",
    securityLevel: "authNoPriv",
    username: "",
    authProtocol: "SHA",
    authPassword: "",
    privProtocol: "AES",
    privPassword: "",
    contextName: "",
    contextEngineId: "",
    createdAt: "",
  };
}
