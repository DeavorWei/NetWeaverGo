<template>
  <Transition name="toast">
    <div 
      v-if="toast.state.value.visible" 
      class="fixed bottom-6 left-1/2 -translate-x-1/2 z-50"
    >
      <div 
        class="flex items-center gap-2 px-5 py-3 rounded-xl shadow-2xl border transition-all"
        :class="toastClasses"
      >
        <!-- 成功图标 -->
        <svg v-if="toast.state.value.type === 'success'" class="w-5 h-5 flex-shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <polyline points="20 6 9 17 4 12"/>
        </svg>
        <!-- 错误图标 -->
        <svg v-else-if="toast.state.value.type === 'error'" class="w-5 h-5 flex-shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <circle cx="12" cy="12" r="10"/>
          <line x1="15" y1="9" x2="9" y2="15"/>
          <line x1="9" y1="9" x2="15" y2="15"/>
        </svg>
        <!-- 警告图标 -->
        <svg v-else-if="toast.state.value.type === 'warning'" class="w-5 h-5 flex-shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/>
          <line x1="12" y1="9" x2="12" y2="13"/>
          <line x1="12" y1="17" x2="12.01" y2="17"/>
        </svg>
        <!-- 信息图标 -->
        <svg v-else class="w-5 h-5 flex-shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <circle cx="12" cy="12" r="10"/>
          <line x1="12" y1="16" x2="12" y2="12"/>
          <line x1="12" y1="8" x2="12.01" y2="8"/>
        </svg>
        <span class="text-sm font-medium">{{ toast.state.value.message }}</span>
      </div>
    </div>
  </Transition>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useToast } from '../../utils/useToast'

const toast = useToast()

const toastClasses = computed(() => {
  const type = toast.state.value.type
  const classes: Record<string, string> = {
    success: 'bg-success/10 border-success/30 text-success',
    error: 'bg-error/10 border-error/30 text-error',
    warning: 'bg-warning/10 border-warning/30 text-warning',
    info: 'bg-accent/10 border-accent/30 text-accent',
  }
  return classes[type] || classes.info
})
</script>

<style scoped>
.toast-enter-active { 
  transition: all 0.3s ease-out; 
}
.toast-leave-active { 
  transition: all 0.2s ease-in; 
}
.toast-enter-from { 
  opacity: 0; 
  transform: translateX(-50%) translateY(20px); 
}
.toast-leave-to { 
  opacity: 0; 
  transform: translateX(-50%) translateY(10px); 
}
</style>
