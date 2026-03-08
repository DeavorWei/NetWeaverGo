<template>
  <div class="command-group-selector space-y-4">
    <!-- 命令组选择 -->
    <div class="flex items-center gap-3">
      <span class="text-sm text-text-muted">选择命令组:</span>
      <select
        v-model="selectedGroupId"
        :disabled="loading"
        class="flex-1 max-w-xs px-3 py-2 rounded-lg bg-bg-card border border-border text-sm text-text-primary focus:outline-none focus:border-accent/50 transition-all disabled:opacity-50"
      >
        <option value="">请选择命令组</option>
        <option v-for="group in groups" :key="group.id" :value="group.id">
          {{ group.name }} ({{ group.commands.length }} 条命令)
        </option>
      </select>
      <button
        @click="openCommandsPage"
        class="px-3 py-2 text-xs text-accent hover:text-accent-glow transition-colors"
      >
        管理命令组
      </button>
    </div>

    <!-- 加载状态 -->
    <div v-if="loading" class="flex items-center justify-center py-6">
      <div class="w-6 h-6 border-2 border-accent border-t-transparent rounded-full animate-spin"></div>
    </div>

    <!-- 无命令组提示 -->
    <div v-else-if="groups.length === 0" class="flex flex-col items-center justify-center py-6 text-text-muted gap-2">
      <svg xmlns="http://www.w3.org/2000/svg" class="w-10 h-10 text-text-muted/30" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
        <polyline points="4 17 10 11 4 5"/><line x1="12" y1="19" x2="20" y2="19"/>
      </svg>
      <p class="text-sm">暂无命令组</p>
      <button
        @click="openCommandsPage"
        class="text-accent hover:text-accent-glow transition-colors text-sm"
      >
        去创建命令组
      </button>
    </div>

    <!-- 命令预览 -->
    <Transition name="slide">
      <div v-if="selectedGroup && !loading" class="border border-border rounded-lg overflow-hidden">
        <div class="bg-bg-panel px-4 py-2 border-b border-border flex items-center justify-between">
          <div class="flex items-center gap-2">
            <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4 text-accent" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <polyline points="4 17 10 11 4 5"/><line x1="12" y1="19" x2="20" y2="19"/>
            </svg>
            <span class="text-sm text-text-primary font-medium">{{ selectedGroup.name }}</span>
            <span class="text-xs text-text-muted">({{ selectedGroup.commands.length }} 条命令)</span>
          </div>
          <button
            @click="togglePreview"
            class="text-xs text-accent hover:text-accent-glow transition-colors"
          >
            {{ showFullPreview ? '收起' : '展开' }}
          </button>
        </div>
        
        <!-- 描述和标签 -->
        <div class="px-4 py-2 border-b border-border bg-bg-secondary/30">
          <p v-if="selectedGroup.description" class="text-xs text-text-muted mb-2">{{ selectedGroup.description }}</p>
          <div class="flex flex-wrap gap-1.5">
            <span
              v-for="tag in selectedGroup.tags"
              :key="tag"
              class="px-2 py-0.5 text-xs rounded-full bg-accent/10 text-accent border border-accent/20"
            >
              {{ tag }}
            </span>
            <span v-if="!selectedGroup.tags || selectedGroup.tags.length === 0" class="text-xs text-text-muted">无标签</span>
          </div>
        </div>

        <!-- 命令列表 -->
        <div
          :class="[
            'bg-bg-secondary/50 font-mono text-xs overflow-y-auto scrollbar-custom transition-all duration-300',
            showFullPreview ? 'max-h-64' : 'max-h-32'
          ]"
        >
          <div
            v-for="(cmd, idx) in selectedGroup.commands"
            :key="idx"
            class="px-4 py-1.5 border-b border-border/30 last:border-0 hover:bg-accent/5 transition-colors"
          >
            <span class="text-text-muted mr-2 select-none">{{ idx + 1 }}.</span>
            <span class="text-text-secondary">{{ cmd }}</span>
          </div>
        </div>
      </div>
    </Transition>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import type { CommandGroup } from '../../types/command'
// @ts-ignore
import { ListCommandGroups } from '../../bindings/github.com/NetWeaverGo/core/internal/ui/appservice.js'
import { useRouter } from 'vue-router'

const props = defineProps<{
  modelValue?: string // 选中的命令组ID
}>()

const emit = defineEmits<{
  'update:modelValue': [groupId: string]
  'selectionChange': [group: CommandGroup | null]
}>()

const router = useRouter()
const loading = ref(true)
const groups = ref<CommandGroup[]>([])
const selectedGroupId = ref('')
const showFullPreview = ref(false)

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

// 切换预览展开状态
function togglePreview() {
  showFullPreview.value = !showFullPreview.value
}

// 跳转到命令管理页面
function openCommandsPage() {
  router.push('/commands')
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
