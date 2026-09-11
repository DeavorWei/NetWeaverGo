import { Call } from '@wailsio/runtime';
import * as HardwareInventoryService from "@/bindings/github.com/NetWeaverGo/core/internal/ui/hardwareinventoryservice";
import type { CEASDeviceOverviewVO } from "@/bindings/github.com/NetWeaverGo/core/internal/ui/models";
import type { HardwareTreeVO, NodeVO, BOMAlertVO } from "@/bindings/github.com/NetWeaverGo/core/internal/ceas/models";
import type { BOMWatchlistItem } from "@/bindings/github.com/NetWeaverGo/core/internal/models/models";

export type { CEASDeviceOverviewVO, HardwareTreeVO, NodeVO, BOMAlertVO, BOMWatchlistItem };

export const HardwareInventoryAPI = {
  /**
   * 列出所有设备及 CEAS 采集状态概览
   */
  async listCEASDevices(): Promise<CEASDeviceOverviewVO[]> {
    return HardwareInventoryService.ListCEASDevices();
  },

  /**
   * 获取指定设备的完整硬件树
   */
  async getHardwareTree(deviceIP: string): Promise<HardwareTreeVO | null> {
    return HardwareInventoryService.GetHardwareTree(deviceIP);
  },

  /**
   * 获取指定设备的硬件树，并按 Item / BarCode 白名单过滤（规划方案 §7.2 P3-1）
   */
  async getHardwareTreeFiltered(deviceIP: string, itemWhitelist: string[], barcodeWhitelist: string[]): Promise<HardwareTreeVO | null> {
    return Call.ByName('ui.HardwareInventoryService.GetHardwareTreeFiltered', deviceIP, itemWhitelist, barcodeWhitelist);
  },

  /**
   * 懒加载获取子节点
   */
  async getHardwareNodeChildren(deviceIP: string, parentID: string): Promise<(NodeVO | null)[]> {
    return HardwareInventoryService.GetHardwareNodeChildren(deviceIP, parentID);
  },

  /**
   * 查询 BOM 观察清单列表
   */
  async listBOMWatchlist(): Promise<BOMWatchlistItem[]> {
    return HardwareInventoryService.ListBOMWatchlist();
  },

  /**
   * 新增或更新 BOM 观察清单条目
   */
  async saveBOMWatchlistItem(item: BOMWatchlistItem): Promise<void> {
    return HardwareInventoryService.SaveBOMWatchlistItem(item);
  },

  /**
   * 删除 BOM 观察清单条目
   */
  async deleteBOMWatchlistItem(id: number): Promise<void> {
    return HardwareInventoryService.DeleteBOMWatchlistItem(id);
  },

  /**
   * 切换 BOM 观察清单启用状态
   */
  async toggleBOMWatchlistEnabled(id: number, enabled: boolean): Promise<void> {
    return HardwareInventoryService.ToggleBOMWatchlistEnabled(id, enabled);
  },

  /**
   * 查询 BOM 批次预警命中明细
   */
  async listBOMAlerts(deviceIP = "", severity = ""): Promise<BOMAlertVO[]> {
    return HardwareInventoryService.ListBOMAlerts(deviceIP, severity);
  },

  /**
   * 导出预警清单为 CSV
   */
  async exportBOMAlertsCSV(deviceIP = "", severity = ""): Promise<string> {
    return HardwareInventoryService.ExportBOMAlertsCSV(deviceIP, severity);
  },

  /**
   * 导出指定设备硬件清单为 CSV
   */
  async exportHardwareInventoryCSV(deviceIP: string): Promise<string> {
    return HardwareInventoryService.ExportHardwareInventoryCSV(deviceIP);
  },

  /**
   * 触发设备 CEAS 硬件采集任务
   */
  async triggerCEASCollect(deviceIPs: string[]): Promise<string> {
    return HardwareInventoryService.TriggerCEASCollect(deviceIPs);
  },
};
