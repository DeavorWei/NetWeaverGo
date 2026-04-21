import { ref, computed, type Ref } from "vue";
import type {
  ListItem,
  ItemKey,
  SelectorConfig,
} from "../types/dualListSelector";

export function useDualListSelection(
  sourceData: Ref<ListItem[]>,
  targetData: Ref<ListItem[]>,
  config: Ref<SelectorConfig>,
) {
  // ==================== State ====================

  /** 当前筛选类型 */
  const currentFilter = ref<string>("all");

  /** 当前筛选值 */
  const filterValue = ref<string>("");

  /** 搜索关键词 */
  const searchKeyword = ref<string>("");

  /** 左栏选中项 */
  const sourceSelected = ref<Set<ItemKey>>(new Set());

  /** 右栏选中项 */
  const targetSelected = ref<Set<ItemKey>>(new Set());

  // ==================== Computed ====================

  /** 过滤后的源数据 */
  const filteredSourceData = computed(() => {
    let result = sourceData.value;

    // 应用筛选
    if (currentFilter.value !== "all" && filterValue.value) {
      result = result.filter(
        (item) => item[currentFilter.value] === filterValue.value,
      );
    }

    // 应用搜索
    if (searchKeyword.value && config.value.enableSearch) {
      const keyword = searchKeyword.value.toLowerCase();
      result = result.filter((item) =>
        config.value.searchFields.some((field) =>
          String(item[field]).toLowerCase().includes(keyword),
        ),
      );
    }

    return result;
  });

  /** 已选项目（去重） */
  const selectedItems = computed(() => {
    const targetKeys = new Set(targetData.value.map((item) => item.key));
    return targetData.value.filter((item) => targetKeys.has(item.key));
  });

  /** 源数据统计 */
  const sourceStats = computed(() => ({
    total: sourceData.value.length,
    filtered: filteredSourceData.value.length,
    selected: sourceSelected.value.size,
  }));

  /** 目标数据统计 */
  const targetStats = computed(() => ({
    total: targetData.value.length,
    selected: targetSelected.value.size,
  }));

  /** 是否可右移 */
  const canMoveRight = computed(() => sourceSelected.value.size > 0);

  /** 是否可左移 */
  const canMoveLeft = computed(() => targetSelected.value.size > 0);

  /** 是否可全右移 */
  const canMoveAllRight = computed(() => filteredSourceData.value.length > 0);

  /** 是否可全左移 */
  const canMoveAllLeft = computed(() => targetData.value.length > 0);

  // ==================== Actions ====================

  /** 切换左栏选择 */
  const toggleSourceSelection = (key: ItemKey) => {
    if (sourceSelected.value.has(key)) {
      sourceSelected.value.delete(key);
    } else {
      sourceSelected.value.add(key);
    }
  };

  /** 切换右栏选择 */
  const toggleTargetSelection = (key: ItemKey) => {
    if (targetSelected.value.has(key)) {
      targetSelected.value.delete(key);
    } else {
      targetSelected.value.add(key);
    }
  };

  /** 右移选中项 */
  const moveRight = (): ListItem[] => {
    const itemsToMove = sourceData.value.filter((item) =>
      sourceSelected.value.has(item.key),
    );
    sourceSelected.value.clear();
    return itemsToMove;
  };

  /** 左移选中项 */
  const moveLeft = (): ItemKey[] => {
    const keysToRemove = Array.from(targetSelected.value);
    targetSelected.value.clear();
    return keysToRemove;
  };

  /** 全右移 */
  const moveAllRight = (): ListItem[] => {
    return filteredSourceData.value;
  };

  /** 全左移 */
  const moveAllLeft = (): ItemKey[] => {
    return targetData.value.map((item) => item.key);
  };

  /** 重置状态 */
  const resetState = () => {
    currentFilter.value = "all";
    filterValue.value = "";
    searchKeyword.value = "";
    sourceSelected.value.clear();
    targetSelected.value.clear();
  };

  /** 应用筛选 */
  const applyFilter = (type: string, value: string = "") => {
    currentFilter.value = type;
    filterValue.value = value;
    sourceSelected.value.clear();
  };

  return {
    // State
    currentFilter,
    filterValue,
    searchKeyword,
    sourceSelected,
    targetSelected,
    // Computed
    filteredSourceData,
    selectedItems,
    sourceStats,
    targetStats,
    canMoveRight,
    canMoveLeft,
    canMoveAllRight,
    canMoveAllLeft,
    // Actions
    toggleSourceSelection,
    toggleTargetSelection,
    moveRight,
    moveLeft,
    moveAllRight,
    moveAllLeft,
    resetState,
    applyFilter,
  };
}