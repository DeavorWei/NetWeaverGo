import * as InspectionService from "@/bindings/github.com/NetWeaverGo/core/internal/ui/inspectionservice";
import type {
  InspectionTemplate,
  InspectionItem,
  InspectionResult,
} from "@/bindings/github.com/NetWeaverGo/core/internal/models/models";
import type { InspectionSummaryVO } from "@/bindings/github.com/NetWeaverGo/core/internal/ui/models";
import type { TaskRun } from "@/bindings/github.com/NetWeaverGo/core/internal/taskexec/models";

export type {
  InspectionTemplate,
  InspectionItem,
  InspectionResult,
  InspectionSummaryVO,
  TaskRun,
};

export const InspectionAPI = {
  /**
   * 获取所有巡检模板列表
   */
  async listTemplates(): Promise<InspectionTemplate[]> {
    return InspectionService.ListInspectionTemplates();
  },

  /**
   * 获取指定巡检模板详情
   */
  async getTemplate(id: string): Promise<InspectionTemplate | null> {
    return InspectionService.GetInspectionTemplate(id);
  },

  /**
   * 保存巡检模板
   */
  async saveTemplate(tpl: InspectionTemplate): Promise<void> {
    return InspectionService.SaveInspectionTemplate(tpl);
  },

  /**
   * 删除巡检模板
   */
  async deleteTemplate(id: string): Promise<void> {
    return InspectionService.DeleteInspectionTemplate(id);
  },

  /**
   * 获取指定模板的巡检检查项
   */
  async listItems(templateId: string): Promise<InspectionItem[]> {
    return InspectionService.ListInspectionItems(templateId);
  },

  /**
   * 保存巡检检查项
   */
  async saveItem(item: InspectionItem): Promise<void> {
    return InspectionService.SaveInspectionItem(item);
  },

  /**
   * 删除巡检检查项
   */
  async deleteItem(id: number): Promise<void> {
    return InspectionService.DeleteInspectionItem(id);
  },

  /**
   * 切换检查项启用状态
   */
  async toggleItemEnabled(id: number, enabled: boolean): Promise<void> {
    return InspectionService.ToggleInspectionItemEnabled(id, enabled);
  },

  /**
   * 触发设备巡检任务
   */
  async triggerInspection(
    templateId: string,
    deviceIps: string[],
    concurrency = 10
  ): Promise<string> {
    return InspectionService.TriggerInspection(templateId, deviceIps, concurrency);
  },

  /**
   * 查询巡检结果列表
   */
  async getResults(
    runId = "",
    deviceIp = "",
    status = "",
    severity = ""
  ): Promise<InspectionResult[]> {
    return InspectionService.GetInspectionResults(runId, deviceIp, status, severity);
  },

  /**
   * 获取巡检报告汇总数据
   */
  async getSummary(runId = ""): Promise<InspectionSummaryVO | null> {
    return InspectionService.GetInspectionSummary(runId);
  },

  /**
   * 查询最近巡检任务执行记录
   */
  async listRecentRuns(limit = 20): Promise<TaskRun[]> {
    return InspectionService.ListRecentInspectionRuns(limit);
  },

  /**
   * 导出巡检报告为 CSV (带 UTF-8 BOM)
   */
  async exportCSV(runId = ""): Promise<string> {
    return InspectionService.ExportInspectionCSV(runId);
  },

  /**
   * 导出巡检报告为结构化 JSON
   */
  async exportJSON(runId = ""): Promise<string> {
    return InspectionService.ExportInspectionJSON(runId);
  },
};
