<template>
  <div 
    ref="terminalRef"
    class="h-52 overflow-y-auto scrollbar-custom bg-terminal-bg p-3 font-mono text-xs leading-relaxed"
    @scroll="handleScroll"
  >
    <!-- 截断提示 -->
    <div v-if="truncated && logs.length > 0" class="text-warning text-xs mb-1 sticky top-0 bg-terminal-bg/90 py-1">
      [日志已截断，显示最近 {{ logs.length }} 条 / 共 {{ totalCount }} 条]
    </div>
    
    <!-- 日志列表 - 使用简单的窗口渲染优化 -->
    <div ref="logContainer" class="space-y-0.5">
      <div 
        v-for="(log, idx) in visibleLogs" 
        :key="`${deviceIp}-${idx}`"
        class="whitespace-pre-wrap break-all"
        :class="getLogColor(log)"
      >
        {{ log }}
      </div>
    </div>
    
    <!-- 空状态 -->
    <div v-if="logs.length === 0" class="text-text-muted/50 italic">
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
const isUserScrolling = ref(false)
const scrollTimeout = ref<ReturnType<typeof setTimeout> | null>(null)

// 组件卸载时清理定时器，防止内存泄漏
onUnmounted(() => {
  if (scrollTimeout.value) {
    clearTimeout(scrollTimeout.value)
    scrollTimeout.value = null
  }
})

// 可见日志数量限制（防止DOM节点过多）
const MAX_VISIBLE_LOGS = 100

// 计算可见日志（始终显示最新的）
const visibleLogs = computed(() => {
  if (props.logs.length <= MAX_VISIBLE_LOGS) {
    return props.logs
  }
  // 只显示最新的日志
  return props.logs.slice(-MAX_VISIBLE_LOGS)
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

// 处理滚动事件
function handleScroll() {
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
    }
  })
}

// 监听日志变化，自动滚动
watch(() => props.logs.length, scrollToBottom, { immediate: true })

// 组件卸载时清理
defineExpose({
  scrollToBottom
})
</script>
