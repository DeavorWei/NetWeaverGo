/**
 * 面板拖拽调整大小 composable
 *
 * 支持：
 * - 水平方向面板宽度拖拽（左/中/右面板间）
 * - 垂直方向面板高度拖拽（上/下面板间）
 * - 面板折叠/展开
 * - 最小尺寸约束
 * - 拖拽手柄样式
 */

import { ref, computed, onUnmounted, type Ref } from 'vue'

/** 面板方向 */
export type PanelDirection = 'horizontal' | 'vertical'

/** 面板配置 */
export interface PanelConfig {
  /** 初始大小（px） */
  initialSize: number
  /** 最小大小（px），默认 120 */
  minSize?: number
  /** 最大大小（px），默认无限 */
  maxSize?: number
}

/** 拖拽手柄位置 */
export type HandlePosition = 'before' | 'after'

/**
 * 面板拖拽调整大小
 *
 * @param containerRef 容器元素 ref
 * @param direction 方向：horizontal（宽度）或 vertical（高度）
 * @param panels 面板配置数组
 */
export function usePanelResize(
  containerRef: Ref<HTMLElement | null>,
  direction: PanelDirection,
  panels: PanelConfig[],
) {
  // ==================== 状态 ====================

  /** 各面板的大小 */
  const sizes = ref(panels.map(p => p.initialSize)) as Ref<number[]>

  /** 各面板的折叠状态 */
  const collapsed = ref(panels.map(() => false)) as Ref<boolean[]>

  /** 各面板折叠前的大小（用于展开恢复） */
  const preCollapseSizes = ref(panels.map(p => p.initialSize)) as Ref<number[]>

  /** 拖拽状态 */
  const isResizing = ref(false)
  const activeHandleIndex = ref<number | null>(null)

  /** 当前活跃的事件处理器引用（用于 onUnmounted 清理） */
  let activeMouseMoveHandler: ((ev: MouseEvent) => void) | null = null
  let activeMouseUpHandler: (() => void) | null = null

  // ==================== 内部方法 ====================

  /** 获取面板最小尺寸 */
  function getMinSize(index: number): number {
    return panels[index]?.minSize ?? 120
  }

  /** 获取面板最大尺寸 */
  function getMaxSize(index: number): number {
    return panels[index]?.maxSize ?? Infinity
  }

  /** 获取鼠标在容器中的位置（像素） */
  function getMousePosition(e: MouseEvent): number {
    if (!containerRef.value) return 0
    const rect = containerRef.value.getBoundingClientRect()
    if (direction === 'horizontal') {
      return e.clientX - rect.left
    }
    return e.clientY - rect.top
  }

  /**
   * 开始拖拽
   * @param handleIndex 手柄索引（表示调整 handleIndex 和 handleIndex+1 之间的面板）
   */
  function startResize(handleIndex: number, e: MouseEvent) {
    // 如果相邻面板有折叠的，不允许拖拽
    if (collapsed.value[handleIndex] || collapsed.value[handleIndex + 1]) return

    isResizing.value = true
    activeHandleIndex.value = handleIndex

    const startPos = getMousePosition(e)
    const startSizes = [...sizes.value]

    const onMouseMove = (ev: MouseEvent) => {
      const currentPos = getMousePosition(ev)
      const delta = currentPos - startPos

      const leftIndex = handleIndex
      const rightIndex = handleIndex + 1

      let newLeftSize = startSizes[leftIndex]! + delta
      let newRightSize = startSizes[rightIndex]! - delta

      // 约束最小/最大尺寸
      const leftMin = getMinSize(leftIndex)
      const leftMax = getMaxSize(leftIndex)
      const rightMin = getMinSize(rightIndex)
      const rightMax = getMaxSize(rightIndex)

      // 两面板总大小（拖拽过程中守恒）
      const totalSize = startSizes[leftIndex]! + startSizes[rightIndex]!

      // 左侧面板约束
      if (newLeftSize < leftMin) {
        newLeftSize = leftMin
      } else if (newLeftSize > leftMax) {
        newLeftSize = leftMax
      }

      // 右侧面板约束
      newRightSize = totalSize - newLeftSize
      if (newRightSize < rightMin) {
        newRightSize = rightMin
        newLeftSize = totalSize - rightMin
      } else if (newRightSize > rightMax) {
        newRightSize = rightMax
        newLeftSize = totalSize - rightMax
      }

      // 交叉验证：当容器过小（totalSize < leftMin + rightMin）时，
      // 保证两面板都不低于各自 minSize 是不可能的，此时按比例分配
      if (totalSize < leftMin + rightMin) {
        const ratio = totalSize > 0 ? leftMin / (leftMin + rightMin) : 0.5
        newLeftSize = Math.round(totalSize * ratio)
        newRightSize = totalSize - newLeftSize
      }

      sizes.value[leftIndex] = newLeftSize
      sizes.value[rightIndex] = newRightSize
    }

    const onMouseUp = () => {
      isResizing.value = false
      activeHandleIndex.value = null
      activeMouseMoveHandler = null
      activeMouseUpHandler = null
      document.body.style.cursor = ''
      document.body.style.userSelect = ''
      window.removeEventListener('mousemove', onMouseMove)
      window.removeEventListener('mouseup', onMouseUp)
    }

    // 保存处理器引用，用于 onUnmounted 清理
    activeMouseMoveHandler = onMouseMove
    activeMouseUpHandler = onMouseUp

    document.body.style.cursor = direction === 'horizontal' ? 'col-resize' : 'row-resize'
    document.body.style.userSelect = 'none'
    window.addEventListener('mousemove', onMouseMove)
    window.addEventListener('mouseup', onMouseUp)
  }

  // ==================== 折叠/展开 ====================

  /**
   * 切换面板折叠状态
   * @param index 面板索引
   */
  function toggleCollapse(index: number) {
    if (collapsed.value[index]) {
      // 展开：恢复之前的大小
      sizes.value[index] = preCollapseSizes.value[index]!
      collapsed.value[index] = false
    } else {
      // 折叠：保存当前大小
      preCollapseSizes.value[index] = sizes.value[index]!
      sizes.value[index] = 0
      collapsed.value[index] = true
    }
  }

  /**
   * 折叠面板
   */
  function collapsePanel(index: number) {
    if (collapsed.value[index]) return
    preCollapseSizes.value[index] = sizes.value[index]!
    sizes.value[index] = 0
    collapsed.value[index] = true
  }

  /**
   * 展开面板
   */
  function expandPanel(index: number) {
    if (!collapsed.value[index]) return
    sizes.value[index] = preCollapseSizes.value[index]!
    collapsed.value[index] = false
  }

  // ==================== 计算属性 ====================

  /** 面板是否被折叠 */
  function isCollapsed(index: number): boolean {
    return collapsed.value[index] ?? false
  }

  /** 获取面板的样式 */
  function getPanelStyle(index: number): Record<string, string> {
    const prop = direction === 'horizontal' ? 'width' : 'height'
    return {
      [prop]: `${sizes.value[index]}px`,
      flexShrink: '0',
      overflow: collapsed.value[index] ? 'hidden' : 'auto',
    }
  }

  /** 拖拽手柄的 CSS 光标样式 */
  const resizeCursor = computed(() =>
    direction === 'horizontal' ? 'col-resize' : 'row-resize'
  )

  // ==================== 清理 ====================

  onUnmounted(() => {
    // 确保清理拖拽事件监听器，防止内存泄漏
    if (activeMouseMoveHandler) {
      window.removeEventListener('mousemove', activeMouseMoveHandler)
      activeMouseMoveHandler = null
    }
    if (activeMouseUpHandler) {
      window.removeEventListener('mouseup', activeMouseUpHandler)
      activeMouseUpHandler = null
    }
    isResizing.value = false
    activeHandleIndex.value = null
    document.body.style.cursor = ''
    document.body.style.userSelect = ''
  })

  return {
    /** 各面板的大小 */
    sizes,
    /** 各面板的折叠状态 */
    collapsed,
    /** 是否正在拖拽 */
    isResizing,
    /** 当前活动的手柄索引 */
    activeHandleIndex,
    /** 拖拽光标样式 */
    resizeCursor,
    /** 开始拖拽 */
    startResize,
    /** 切换折叠 */
    toggleCollapse,
    /** 折叠面板 */
    collapsePanel,
    /** 展开面板 */
    expandPanel,
    /** 查询是否折叠 */
    isCollapsed,
    /** 获取面板样式 */
    getPanelStyle,
  }
}
