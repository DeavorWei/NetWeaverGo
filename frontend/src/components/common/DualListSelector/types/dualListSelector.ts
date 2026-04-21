/** 列表项唯一标识类型 */
export type ItemKey = string | number;

/** 列表项基础接口 */
export interface ListItem {
  key: ItemKey;
  label: string;
  disabled?: boolean;
  description?: string;
  tags?: string[];
  [key: string]: any;
}

/** 分组数据接口 */
export interface GroupData {
  key: string;
  label: string;
  items: ListItem[];
}

/** 筛选配置 */
export interface FilterConfig {
  type: "group" | "tag" | "all";
  label: string;
  options?: { label: string; value: string; count?: number }[];
}

/** 选择器配置 */
export interface SelectorConfig {
  /** 是否支持分组显示 */
  enableGrouping: boolean;
  /** 是否支持标签筛选 */
  enableTagFilter: boolean;
  /** 是否启用搜索 */
  enableSearch: boolean;
  /** 搜索字段 */
  searchFields: string[];
  /** 左栏标题 */
  sourceTitle: string;
  /** 右栏标题 */
  targetTitle: string;
  /** 是否显示计数 */
  showCount: boolean;
  /** 最大选择数量 */
  maxSelection?: number;
  /** 弹窗标题 */
  modalTitle: string;
  /** 确认按钮文本 */
  confirmText: string;
  /** 取消按钮文本 */
  cancelText: string;
}

/** 选择器状态 */
export interface SelectorState {
  /** 当前筛选类型 */
  currentFilter: string;
  /** 当前筛选值 */
  filterValue: string;
  /** 搜索关键词 */
  searchKeyword: string;
  /** 左栏选中项 */
  sourceSelected: Set<ItemKey>;
  /** 右栏选中项 */
  targetSelected: Set<ItemKey>;
  /** 已选项目列表 */
  selectedItems: ListItem[];
}

/** 默认配置 */
export const DEFAULT_CONFIG: SelectorConfig = {
  enableGrouping: true,
  enableTagFilter: true,
  enableSearch: true,
  searchFields: ["label", "key"],
  sourceTitle: "可选项",
  targetTitle: "已选项",
  showCount: true,
  modalTitle: "选择项目",
  confirmText: "确认",
  cancelText: "取消",
};
