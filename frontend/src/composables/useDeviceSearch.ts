/**
 * 设备搜索逻辑 Composable
 * 
 * 提供设备列表页面的搜索功能，支持按分组、标签、IP 搜索
 */

import { ref, computed } from 'vue'

export type SearchType = 'group' | 'tag' | 'ip' | 'protocol'

export interface SearchOption {
  label: string
  value: SearchType
}

/**
 * 设备搜索 Hook
 */
export function useDeviceSearch() {
  // 搜索关键词
  const searchQuery = ref('')
  
  // 搜索类型
  const searchType = ref<SearchType>('group')
  
  // 搜索选项
  const searchOptions: SearchOption[] = [
    { label: '分组', value: 'group' },
    { label: '标签', value: 'tag' },
    { label: 'IP', value: 'ip' },
    { label: '协议', value: 'protocol' }
  ]
  
  // 当前搜索类型的标签
  const currentSearchLabel = computed(() => 
    searchOptions.find(o => o.value === searchType.value)?.label || ''
  )
  
  /**
   * 重置搜索
   */
  const resetSearch = () => {
    searchQuery.value = ''
  }
  
  /**
   * 设置搜索类型
   */
  const setSearchType = (type: SearchType) => {
    searchType.value = type
    searchQuery.value = '' // 切换类型时清空搜索词
  }

  return {
    searchQuery,
    searchType,
    searchOptions,
    currentSearchLabel,
    resetSearch,
    setSearchType
  }
}
