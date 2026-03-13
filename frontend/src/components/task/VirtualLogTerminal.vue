<template>
  <div 
    ref="terminalRef"
    class="h-52 overflow-y-auto scrollbar-custom bg-terminal-bg p-3 font-mono text-xs leading-relaxed"
    @scroll="handleScroll"
  >
    <!-- 顶部占位 -->
    <div :style="{ height: `${topPlaceholderHeight}px` }"></div>
    
    <!-- 可见日志 -->
    <div 
      v-for="(log, idx) in visibleLogs" 
      :key="startIndex + idx"
      class="whitespace-pre-wrap break-all h-[20px]"
      :class="getLogColor(log)"
    >
      {{ log }}
    </div>
    
    <!-- 底部占位 -->
    <div :style="{ height: `${bottomPlaceholderHeight}px` }"></div>
    
    <!-- 截断提示 -->
    <div v-if="truncated && logs.length > 0" class="text-warning text-xs mt-2 sticky bottom-0 bg-terminal-bg/90 py-1">
      [共 {{ totalCount }} 条日志，显示 {{ logs.length }} 条]
    </div>
    
    <!-- 空状态 -->
    <div v-if="logs.length === 0" class="text-text-muted/50 italic h-full flex items-center justify-center">
      等待日志输出...
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick, onUnmounted } from 'vue'

const props = defineProps<{
  logs: string[]
  totalCount: number
  truncated: boolean
  deviceIp: string
}>()

const terminalRef = ref<HTMLElement>()
const scrollTop = ref(0)
const isUserScrolling = ref(false)
const scrollTimeout = ref<ReturnType<typeof setTimeout> | null>(null)

// 虚拟滚动配置
const ITEM_HEIGHT = 20 // 每行高度（px）
const CONTAINER_HEIGHT = 208 // 52 * 4 = 208px (h-52)
const BUFFER_SIZE = 5 // 上下缓冲行数

// 计算可见范围
const startIndex = computed(() => {
  const start = Math.floor(scrollTop.value / ITEM_HEIGHT) - BUFFER_SIZE
  return Math.max(0, start)
})

const endIndex = computed(() => {
  const visibleCount = Math.ceil(CONTAINER_HEIGHT / ITEM_HEIGHT)
  const end = startIndex.value + visibleCount + BUFFER_SIZE * 2
  return Math.min(props.logs.length, end)
})

// 可见日志
const visibleLogs = computed(() => {
  return props.logs.slice(startIndex.value, endIndex.value)
})

// 占位高度
const topPlaceholderHeight = computed(() => {
  return startIndex.value * ITEM_HEIGHT
})

const bottomPlaceholderHeight = computed(() => {
  return (props.logs.length - endIndex.value) * ITEM_HEIGHT
})

// 处理滚动事件
function handleScroll() {
  if (!terminalRef.value) return
  
  scrollTop.value = terminalRef.value.scrollTop
  isUserScrolling.value = true
  
  if (scrollTimeout.value) {
    clearTimeout(scrollTimeout.value)
  }
  
  scrollTimeout.value = setTimeout(() => {
    isUserScrolling.value = false
  }, 1000)
}

// 自动滚动到底部
function scrollToBottom() {
  if (isUserScrolling.value || !terminalRef.value) return
  
  nextTick(() => {
    const el = terminalRef.value
    if (el) {
      el.scrollTop = el.scrollHeight
      scrollTop.value = el.scrollTop
    }
  })
}

// 监听日志变化
watch(() => props.logs.length, scrollToBottom)

// 清理
onUnmounted(() => {
  if (scrollTimeout.value) {
    clearTimeout(scrollTimeout.value)
  }
})

// 获取日志颜色
function getLogColor(log: string): string {
  const l = log.toLowerCase()
  if (l.includes('error') || l.includes('失败') || l.includes('错误')) return 'text-error'
  if (l.includes('warn') || l.includes('警告')) return 'text-warning'
  if (l.includes('success') || l.includes('成功') || l.includes('完成')) return 'text-success'
  if (log.startsWith('[')) return 'text-info'
  return 'text-text-muted'
}

defineExpose({
  scrollToBottom
})
</script>
