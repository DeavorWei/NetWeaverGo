// 组件导出
export { default as DualListSelector } from "./DualListSelector.vue";

// 类型导出
export type {
  ListItem,
  ItemKey,
  GroupData,
  FilterConfig,
  SelectorConfig,
  SelectorState,
} from "./types/dualListSelector";

// 常量导出
export { DEFAULT_CONFIG } from "./types/dualListSelector";

// Composable 导出
export { useDualListSelection } from "./composables/useDualListSelection";
