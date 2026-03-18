import { ref, onMounted, onUnmounted } from 'vue'

/**
 * 列宽拖拽调整逻辑
 * 用于三列布局的宽度拖拽调整
 */
export function useColumnResize() {
  const windowWidth = ref(window.innerWidth)

  // 三列的宽度状态 (百分比)
  const leftColWidth = ref(35)
  const midColWidth = ref(30)
  const rightColWidth = ref(35)

  // 拖拽相关状态
  const isResizing = ref(false)
  const resizeType = ref<'left' | 'right' | null>(null)
  const workspaceRef = ref<HTMLElement | null>(null)

  const handleResize = () => {
    windowWidth.value = window.innerWidth
  }

  const startResize = (type: 'left' | 'right') => {
    isResizing.value = true
    resizeType.value = type
    document.body.style.cursor = 'col-resize'
    window.addEventListener('mousemove', doResize)
    window.addEventListener('mouseup', stopResize)
  }

  const doResize = (e: MouseEvent) => {
    if (!isResizing.value || !resizeType.value || !workspaceRef.value) return

    const rect = workspaceRef.value.getBoundingClientRect()
    const containerWidth = rect.width
    const mouseX = e.clientX - rect.left

    const mousePct = (mouseX / containerWidth) * 100
    const minW = 10

    if (resizeType.value === 'left') {
      if (
        mousePct > minW &&
        mousePct < leftColWidth.value + midColWidth.value - minW
      ) {
        const diff = mousePct - leftColWidth.value
        leftColWidth.value += diff
        midColWidth.value -= diff
      }
    } else if (resizeType.value === 'right') {
      const leftBound = leftColWidth.value
      if (mousePct > leftBound + minW && mousePct < 100 - minW) {
        const diff = mousePct - (leftBound + midColWidth.value)
        midColWidth.value += diff
        rightColWidth.value -= diff
      }
    }
  }

  const stopResize = () => {
    if (!isResizing.value) return
    isResizing.value = false
    resizeType.value = null
    document.body.style.cursor = 'default'
    window.removeEventListener('mousemove', doResize)
    window.removeEventListener('mouseup', stopResize)
  }

  onMounted(() => {
    window.addEventListener('resize', handleResize)
  })

  onUnmounted(() => {
    window.removeEventListener('resize', handleResize)
    // 确保清理拖拽事件
    window.removeEventListener('mousemove', doResize)
    window.removeEventListener('mouseup', stopResize)
  })

  return {
    windowWidth,
    leftColWidth,
    midColWidth,
    rightColWidth,
    isResizing,
    resizeType,
    workspaceRef,
    startResize,
  }
}
