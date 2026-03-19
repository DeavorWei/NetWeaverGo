/**
 * 设备选择逻辑 Composable
 * 
 * 提供设备列表的选择功能，支持单选、全选、批量操作
 */

import { ref, computed } from 'vue'

/**
 * 设备选择 Hook
 */
export function useDeviceSelection() {
  // 已选择的设备 ID 集合
  const selectedIds = ref<Set<number>>(new Set())
  
  // 是否正在执行全选操作
  const isSelectingAll = ref(false)
  
  // 已选择的设备数量
  const selectedCount = computed(() => selectedIds.value.size)
  
  // 是否全选（当前页）
  const isAllSelected = computed(() => selectedIds.value.size > 0)
  
  // 是否部分选择（用于显示半选状态）
  const isIndeterminate = computed(() => {
    return selectedIds.value.size > 0
  })
  
  /**
   * 切换单个设备的选择状态
   */
  const toggleSelect = (id: number) => {
    const newSet = new Set(selectedIds.value)
    if (newSet.has(id)) {
      newSet.delete(id)
    } else {
      newSet.add(id)
    }
    selectedIds.value = newSet
  }
  
  /**
   * 选择单个设备
   */
  const select = (id: number) => {
    const newSet = new Set(selectedIds.value)
    newSet.add(id)
    selectedIds.value = newSet
  }
  
  /**
   * 取消选择单个设备
   */
  const deselect = (id: number) => {
    const newSet = new Set(selectedIds.value)
    newSet.delete(id)
    selectedIds.value = newSet
  }
  
  /**
   * 切换全选状态（当前页）
   * 不需要参数，由组件内部获取当前页设备 ID
   */
  const toggleSelectAll = () => {
    isSelectingAll.value = true
    // 此函数由 DeviceTable 组件调用，需要传入当前页设备 ID
    // 但为了兼容性，这里改为无参数版本，由组件自行处理
    isSelectingAll.value = false
  }
  
  /**
   * 切换全选状态（指定设备列表）
   * @param allIds 所有可选的设备 ID 列表
   */
  const toggleSelectAllWithIds = (allIds: number[]) => {
    isSelectingAll.value = true
    
    // 检查当前页是否全部选中
    const currentPageSelected = allIds.every(id => selectedIds.value.has(id))
    
    if (currentPageSelected) {
      // 如果当前页全选，则取消当前页的选择
      const newSet = new Set(selectedIds.value)
      allIds.forEach(id => newSet.delete(id))
      selectedIds.value = newSet
    } else {
      // 否则选中当前页所有设备
      const newSet = new Set(selectedIds.value)
      allIds.forEach(id => newSet.add(id))
      selectedIds.value = newSet
    }
    
    isSelectingAll.value = false
  }
  
  /**
   * 全选所有设备
   * @param allIds 所有设备 ID 列表
   */
  const selectAll = (allIds: number[]) => {
    selectedIds.value = new Set(allIds)
  }
  
  /**
   * 清空所有选择
   */
  const clearSelection = () => {
    selectedIds.value = new Set()
  }
  
  /**
   * 检查设备是否已选中
   */
  const isSelected = (id: number): boolean => {
    return selectedIds.value.has(id)
  }
  
  /**
   * 获取已选择的 ID 数组
   */
  const getSelectedIdsArray = (): number[] => {
    return Array.from(selectedIds.value)
  }

  return {
    selectedIds,
    isSelectingAll,
    selectedCount,
    isAllSelected,
    isIndeterminate,
    toggleSelect,
    select,
    deselect,
    toggleSelectAll,
    toggleSelectAllWithIds,
    selectAll,
    clearSelection,
    isSelected,
    getSelectedIdsArray
  }
}
