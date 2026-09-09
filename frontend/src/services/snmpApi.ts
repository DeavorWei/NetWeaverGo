/**
 * SNMP 即时查询 API 封装
 *
 * 面向交付现场的设备上线确认与资产信息采集，
 * 仅提供即时查询与查询凭据管理能力。
 */
import {
  SNMPBatchQueryRequest,
  SNMPCredentialVM,
  SNMPQueryRequest,
} from "@/bindings/github.com/NetWeaverGo/core/internal/ui/models";
import * as SNMPQueryService from "@/bindings/github.com/NetWeaverGo/core/internal/ui/snmpqueryservice";
import type {
  SNMPBatchQueryResponse,
  SNMPCredentialInputVM,
  SNMPOperation,
  SNMPQueryResponse,
  SNMPResultVM,
} from "@/types/snmp";

export const SNMPQueryAPI = {
  /**
   * 执行单次 SNMP 查询
   */
  async query(params: {
    address: string;
    operation: SNMPOperation;
    oid?: string;
    credentialId?: number | null;
    tempCredential?: SNMPCredentialInputVM | null;
  }): Promise<SNMPQueryResponse> {
    const req = new SNMPQueryRequest({
      address: params.address,
      operation: params.operation,
      oid: params.oid ?? "",
      credentialId: params.credentialId ?? null,
      tempCredential: params.tempCredential ?? null,
    });
    const resp = await SNMPQueryService.Query(req);
    if (!resp) {
      throw new Error("查询未返回结果");
    }
    return resp;
  },

  /**
   * 批量采集设备基本信息（设备上线确认）
   */
  async batchQueryDeviceInfo(params: {
    addresses: string[];
    credentialId?: number | null;
    tempCredential?: SNMPCredentialInputVM | null;
  }): Promise<SNMPBatchQueryResponse> {
    const req = new SNMPBatchQueryRequest({
      addresses: params.addresses,
      credentialId: params.credentialId ?? null,
      tempCredential: params.tempCredential ?? null,
    });
    const resp = await SNMPQueryService.BatchQueryDeviceInfo(req);
    if (!resp) {
      throw new Error("批量查询未返回结果");
    }
    return resp;
  },

  /**
   * 获取设备基本信息 OID 列表
   */
  async getDeviceInfoOIDs(): Promise<SNMPResultVM[]> {
    return await SNMPQueryService.GetDeviceInfoOIDs();
  },

  /**
   * 获取所有查询凭据
   */
  async getCredentials(): Promise<SNMPCredentialVM[]> {
    return await SNMPQueryService.GetCredentials();
  },

  /**
   * 创建查询凭据
   */
  async createCredential(vm: SNMPCredentialVM): Promise<SNMPCredentialVM | null> {
    return await SNMPQueryService.CreateCredential(vm);
  },

  /**
   * 更新查询凭据（敏感字段留空表示保持原值不变）
   */
  async updateCredential(vm: SNMPCredentialVM): Promise<void> {
    await SNMPQueryService.UpdateCredential(vm);
  },

  /**
   * 删除查询凭据
   */
  async deleteCredential(id: number): Promise<void> {
    await SNMPQueryService.DeleteCredential(id);
  },
};
