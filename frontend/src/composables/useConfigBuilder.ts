import { ref, watch, computed } from 'vue'
import { ForgeAPI } from '../services/api'
import type { VarInput, BuildResult } from '../services/api'

/**
 * 配置构建核心逻辑
 * 处理模板、变量、构建配置等核心功能
 */
export function useConfigBuilder() {
  // 核心状态
  const templateText = ref('')
  const variables = ref<VarInput[]>([
    { name: '[A]', valueString: '' },
    { name: '[B]', valueString: '' },
    { name: '[C]', valueString: '' },
    { name: '[D]', valueString: '' },
  ])
  const buildResult = ref<BuildResult | null>(null)
  const isBuilding = ref(false)
  const isCopied = ref(false)

  // 动态变量索引
  const nextVarIndex = ref(4)

  // 防抖定时器
  let debounceTimer: ReturnType<typeof setTimeout> | null = null

  // 调用后端构建配置
  const buildConfig = async () => {
    if (!templateText.value.trim()) {
      buildResult.value = null
      return
    }

    isBuilding.value = true
    try {
      const result = await ForgeAPI.buildConfig({
        template: templateText.value,
        variables: variables.value.filter(
          (v: VarInput) => v.valueString.trim() !== '' || v.name === '[A]',
        ),
      })
      buildResult.value = result
      isCopied.value = false
    } catch (err) {
      console.error('构建配置失败:', err)
      buildResult.value = null
    } finally {
      isBuilding.value = false
    }
  }

  // 自动生成模式监听
  watch(
    [templateText, variables],
    () => {
      if (debounceTimer) clearTimeout(debounceTimer)
      debounceTimer = setTimeout(() => {
        buildConfig()
      }, 300)
    },
    { deep: true },
  )

  // 输出块（直接使用后端返回的结果）
  const outputBlocks = computed(() => buildResult.value?.blocks ?? [])

  // 获取列名（A, B, C...）
  const getColumnName = (index: number) => {
    let name = ''
    let num = index
    while (num >= 0) {
      name = String.fromCharCode((num % 26) + 65) + name
      num = Math.floor(num / 26) - 1
    }
    return name
  }

  // 添加变量
  const addVariable = () => {
    const newName = getColumnName(nextVarIndex.value++)
    variables.value.push({ name: `[${newName}]`, valueString: '' })
  }

  // 删除变量
  const removeVariable = (idx: number) => {
    variables.value.splice(idx, 1)
  }

  // 展开语法糖
  const expandSyntaxSugar = async (v: VarInput) => {
    if (!v.valueString) return

    // 获取其他变量的最大长度作为目标长度
    const otherVars = variables.value.filter(
      (item: VarInput) => item.name !== v.name,
    )
    let maxLen = 0
    for (const otherVar of otherVars) {
      const vals = otherVar.valueString
        .split(/,|\n/)
        .filter((s: string) => s.trim() !== '')
      if (vals.length > maxLen) maxLen = vals.length
    }

    try {
      const result = await ForgeAPI.expandValues({
        valueString: v.valueString,
        maxLen: maxLen,
      })
      if (result && (result.hasExpanded || result.hasInferred)) {
        v.valueString = result.values.join(', ')
      }
    } catch (err) {
      console.error('展开语法糖失败:', err)
    }
  }

  // 复制全部
  const copyAll = async () => {
    if (outputBlocks.value.length === 0) return
    const textToCopy = outputBlocks.value.join('\n')
    try {
      await navigator.clipboard.writeText(textToCopy)
      isCopied.value = true
      setTimeout(() => {
        isCopied.value = false
      }, 2000)
    } catch (err) {
      console.error('Failed to copy text: ', err)
    }
  }

  return {
    // 状态
    templateText,
    variables,
    buildResult,
    isBuilding,
    isCopied,
    outputBlocks,
    nextVarIndex,
    // 方法
    buildConfig,
    getColumnName,
    addVariable,
    removeVariable,
    expandSyntaxSugar,
    copyAll,
  }
}
