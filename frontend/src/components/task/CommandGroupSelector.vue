<template>
  <div class="command-group-selector flex flex-col h-full">
    <!-- 命令组选择 -->
    <div class="flex items-center gap-3 flex-shrink-0">
      <span class="text-sm text-text-muted">选择命令组:</span>
      <select
        v-model="selectedGroupId"
        :disabled="loading"
        class="flex-1 px-3 py-2 rounded-lg bg-bg-card border border-border text-sm text-text-primary focus:outline-none focus:border-accent/50 transition-all disabled:opacity-50"
      >
        <option value="">请选择命令组</option>
        <option v-for="group in groups" :key="group.id" :value="group.id">
          {{ group.name }} ({{ group.commands.length }} 条命令)
        </option>
      </select>
    </div>

    <!-- 加载状态 -->
    <div v-if="loading" class="flex items-center justify-center py-4 flex-1">
      <div class="w-5 h-5 border-2 border-accent border-t-transparent rounded-full animate-spin"></div>
    </div>

    <!-- 无命令组提示 -->
    <div v-else-if="groups.length === 0" class="flex flex-col items-center justify-center py-4 text-text-muted gap-2 flex-1">
      <svg xmlns="http://www.w3.org/2000/svg" class="w-8 h-8 text-text-muted/30" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
        <polyline points="4 17 10 11 4 5"/><line x1="12" y1="19" x2="20" y2="19"/>
      </svg>
      <p class="text-sm">暂无命令组，请先创建命令组</p>
    </div>

    <!-- 已选命令组展示 -->
    <Transition name="slide">
      <div v-if="selectedGroup && !loading" class="flex flex-col flex-1 min-h-0 mt-3 overflow-hidden">
        <!-- 命令组信息卡片 -->
        <div class="border border-border rounded-lg overflow-hidden bg-bg-secondary/30 flex flex-col flex-1 min-h-0">
          <!-- 描述和标签 -->
          <div v-if="selectedGroup.description || (selectedGroup.tags && selectedGroup.tags.length > 0)" class="px-3 py-2 border-b border-border/50 flex items-center gap-3 flex-wrap flex-shrink-0">
            <p v-if="selectedGroup.description" class="text-xs text-text-muted">{{ selectedGroup.description }}</p>
            <div v-if="selectedGroup.tags && selectedGroup.tags.length > 0" class="flex flex-wrap gap-1">
              <span
                v-for="tag in selectedGroup.tags"
                :key="tag"
                class="px-1.5 py-0.5 text-xs rounded bg-accent/10 text-accent"
              >
                {{ tag }}
              </span>
            </div>
          </div>

          <!-- 命令预览列表 -->
          <div class="font-mono text-xs overflow-y-auto scrollbar-custom bg-terminal-bg flex-1 min-h-0 py-1">
            <div
              v-for="(cmd, idx) in selectedGroup.commands"
              :key="idx"
              class="px-3 py-1 border-b border-border/20 last:border-0 hover:bg-accent/5 transition-colors"
            >
              <span class="text-text-muted/60 mr-2 select-none">{{ idx + 1 }}.</span>
              <span class="text-text-secondary">{{ cmd }}</span>
            </div>
          </div>
        </div>
      </div>
    </Transition>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import type { CommandGroup } from '../../types/command'
import { ListCommandGroups } from '../../services/api'

const props = defineProps<{
  modelValue?: string // 选中的命令组ID
}>()

const emit = defineEmits<{
  'update:modelValue': [groupId: string]
  'selectionChange': [group: CommandGroup | null]
}>()

const loading = ref(true)
const groups = ref<CommandGroup[]>([])
const selectedGroupId = ref('')

// 计算选中的命令组
const selectedGroup = computed(() => {
  if (!selectedGroupId.value) return null
  return groups.value.find(g => g.id === selectedGroupId.value) || null
})

// 加载命令组列表
async function loadGroups() {
  loading.value = true
  try {
    const result = await ListCommandGroups()
    groups.value = result || []
    
    // 如果有传入的modelValue，设置为选中项
    if (props.modelValue && groups.value.some(g => g.id === props.modelValue)) {
      selectedGroupId.value = props.modelValue
    } else if (groups.value.length > 0 && !selectedGroupId.value) {
      // 默认选中第一个
      const firstGroup = groups.value[0]
      if (firstGroup) {
        selectedGroupId.value = firstGroup.id
      }
    }
  } catch (err) {
    console.error('加载命令组失败:', err)
    groups.value = []
  } finally {
    loading.value = false
  }
}

// 监听选中变化
watch(selectedGroupId, (newVal) => {
  emit('update:modelValue', newVal)
  emit('selectionChange', selectedGroup.value)
})

// 监听props变化
watch(() => props.modelValue, (newVal) => {
  if (newVal && newVal !== selectedGroupId.value) {
    selectedGroupId.value = newVal
  }
})

onMounted(() => {
  loadGroups()
})
</script>

<style scoped>
.slide-enter-active,
.slide-leave-active {
  transition: all 0.3s ease;
}

.slide-enter-from,
.slide-leave-to {
  opacity: 0;
  transform: translateY(-10px);
}
</style>
