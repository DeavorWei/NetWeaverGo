import * as ParseTemplateService from "@/bindings/github.com/NetWeaverGo/core/internal/ui/parsetemplateservice";
import * as Models from "@/bindings/github.com/NetWeaverGo/core/internal/models/models";

export interface TreeRule {
  parseItem: string;
  parentItem?: string;
  isList?: boolean;
  itemType?: "string" | "number";
  parseRegex?: string;
  parseFlags?: string;
  groupIndex?: number;
  splitRegex?: string;
  splitFlags?: string;
  isOutput?: boolean;
  defaultValue?: string;
  order?: number;
}

export type UserParseTemplateVO = Models.UserParseTemplateVO & {
  parseRules?: {
    rules: TreeRule[];
    maxOutputLevel?: number;
    schemaVersion?: string;
  };
};

export type SaveParseTemplateRequest = Models.SaveParseTemplateRequest;
export type TestParseTemplateRequest = Models.TestParseTemplateRequest;
export type TestParseTemplateResult = Models.TestParseTemplateResult;

export const ParseTemplateAPI = {
  async listTemplates(vendor: string): Promise<UserParseTemplateVO[]> {
    const list = await ParseTemplateService.ListTemplates(vendor);
    return list as UserParseTemplateVO[];
  },

  async getTemplate(id: number): Promise<UserParseTemplateVO | null> {
    const tpl = await ParseTemplateService.GetTemplate(id);
    return tpl as UserParseTemplateVO | null;
  },

  async createTemplate(req: SaveParseTemplateRequest): Promise<void> {
    return ParseTemplateService.CreateTemplate(req);
  },

  async updateTemplate(id: number, req: SaveParseTemplateRequest): Promise<void> {
    return ParseTemplateService.UpdateTemplate(id, req);
  },

  async deleteTemplate(id: number): Promise<void> {
    return ParseTemplateService.DeleteTemplate(id);
  },

  async testTemplate(req: TestParseTemplateRequest): Promise<TestParseTemplateResult> {
    const res = await ParseTemplateService.TestTemplate(req);
    if (!res) {
      return {
        success: false,
        results: [],
        count: 0,
        error: "未返回测试结果",
      } as TestParseTemplateResult;
    }
    return res;
  },
};
