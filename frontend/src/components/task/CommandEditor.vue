<template>
  <div class="command-editor">
    <!-- 触发按钮 -->
    <button
      @click="openPanel"
      class="flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-medium transition-all duration-200 bg-bg-card border border-border text-text-secondary hover:text-text-primary hover:border-accent/50"
    >
      <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4 text-accent" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <polyline points="4 17 10 11 4 5"/>
        <line x1="12" y1="19" x2="20" y2="19"/>
      </svg>
      <span>命令编辑器</span>
      <span class="px-1.5 py-0.5 text-xs rounded bg-accent/15 text-accent font-mono">{{ commandCount }}</span>
    </button>

    <!-- 侧滑面板 -->
    <Transition name="slide">
      <div v-if="isOpen" class="fixed inset-0 z-50 flex">
        <!-- 遮罩层 -->
        <div class="absolute inset-0 bg-black/50 backdrop-blur-sm" @click="closePanel"></div>
        
        <!-- 面板主体 -->
        <div class="relative ml-auto w-[480px] h-full bg-bg-card border-l border-border shadow-2xl flex flex-col">
          <!-- 头部 -->
          <div class="flex items-center justify-between px-5 py-4 border-b border-border bg-bg-panel">
            <div class="flex items-center gap-3">
              <div class="w-8 h-8 rounded-lg bg-accent/15 flex items-center justify-center">
                <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4 text-accent" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <polyline points="4 17 10 11 4 5"/>
                  <line x1="12" y1="19" x2="20" y2="19"/>
                </svg>
              </div>
              <div>
                <h3 class="text-sm font-semibold text-text-primary">命令编辑器</h3>
                <p class="text-xs text-text-muted mt-0.5">预览和编辑待执行的命令列表</p>
              </div>
            </div>
            <button
              @click="closePanel"
              class="p-2 rounded-lg text-text-muted hover:text-text-primary hover:bg-bg-secondary transition-colors"
            >
              <svg xmlns="http://www.w3.org/2000/svg" class="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <line x1="18" y1="6" x2="6" y2="18"/>
                <line x1="6" y1="6" x2="18" y2="18"/>
              </svg>
            </button>
          </div>

          <!-- 操作栏 -->
          <div class="flex items-center justify-between px-5 py-3 border-b border-border bg-bg-secondary/50">
            <div class="flex items-center gap-2 text-xs text-text-muted">
              <span>共</span>
              <span class="font-mono text-accent">{{ localCommands.length }}</span>
              <span>条命令</span>
            </div>
            <div class="flex items-center gap-2">
              <button
                v-if="!isEditing"
                @click="startEdit"
                class="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-md bg-accent/10 border border-accent/30 text-accent hover:bg-accent/20 transition-colors"
              >
                <svg xmlns="http://www.w3.org/2000/svg" class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/>
                  <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/>
                </svg>
                编辑
              </button>
              <template v-else>
                <button
                  @click="cancelEdit"
                  class="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-md bg-bg-card border border-border text-text-muted hover:text-text-primary transition-colors"
                >
                  取消
                </button>
                <button
                  @click="saveCommands"
                  :disabled="isSaving"
                  class="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-md bg-accent border border-accent/50 text-white hover:bg-accent-glow transition-colors disabled:opacity-50"
                >
                  <svg v-if="isSaving" class="w-3.5 h-3.5 animate-spin" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <circle cx="12" cy="12" r="10" stroke-opacity="0.25"/>
                    <path d="M12 2a10 10 0 0 1 10 10" stroke-opacity="1"/>
                  </svg>
                  <svg v-else xmlns="http://www.w3.org/2000/svg" class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <polyline points="20 6 9 17 4 12"/>
                  </svg>
                  保存
                </button>
              </template>
            </div>
          </div>

          <!-- 内容区 -->
          <div class="flex-1 overflow-hidden">
            <!-- 预览模式 -->
            <div v-if="!isEditing" class="h-full overflow-y-auto scrollbar-custom p-4">
              <div v-if="localCommands.length === 0" class="flex flex-col items-center justify-center h-32 text-text-muted gap-2">
                <svg xmlns="http://www.w3.org/2000/svg" class="w-8 h-8 text-text-muted/30" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
                  <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/>
                  <polyline points="14 2 14 8 20 8"/>
                  <line x1="12" y1="18" x2="12" y2="12"/>
                  <line x1="9" y1="15" x2="15" y2="15"/>
                </svg>
                <p class="text-sm">暂无命令</p>
              </div>
              <div v-else class="space-y-1.5">
                <div
                  v-for="(cmd, index) in localCommands"
                  :key="index"
                  class="flex items-start gap-3 px-3 py-2.5 rounded-lg bg-bg-secondary/50 border border-border/50 group hover:border-border transition-colors"
                >
                  <span class="flex-shrink-0 w-6 h-6 flex items-center justify-center text-xs font-mono text-text-muted bg-bg-panel rounded">
                    {{ index + 1 }}
                  </span>
                  <code class="flex-1 text-sm font-mono text-text-primary break-all leading-relaxed">{{ cmd }}</code>
                </div>
              </div>
            </div>

            <!-- 编辑模式 -->
            <div v-else class="h-full flex flex-col">
              <div class="flex-1 relative">
                <textarea
                  v-model="editContent"
                  class="w-full h-full p-4 bg-terminal-bg text-terminal-text font-mono text-sm leading-relaxed resize-none border-none outline-none scrollbar-custom"
                  placeholder="每行输入一条命令&#10;以 # 开头的行为注释&#10;空行将被忽略"
                  spellcheck="false"
                ></textarea>
              </div>
              <div class="px-4 py-2 border-t border-border bg-bg-secondary/30 text-xs text-text-muted">
                提示：每行一条命令，以 # 开头的行为注释，空行将被自动忽略
              </div>
            </div>
          </div>

          <!-- 底部状态栏 -->
          <div class="flex items-center justify-between px-5 py-3 border-t border-border bg-bg-panel">
            <div class="flex items-center gap-2 text-xs text-text-muted">
              <svg xmlns="http://www.w3.org/2000/svg" class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <circle cx="12" cy="12" r="10"/>
                <line x1="12" y1="16" x2="12" y2="12"/>
                <line x1="12" y1="8" x2="12.01" y2="8"/>
              </svg>
              <span>{{ isEditing ? '编辑模式' : '预览模式' }}</span>
            </div>
            <div v-if="lastSaved" class="text-xs text-text-muted">
              上次保存: {{ lastSaved }}
            </div>
          </div>
        </div>
      </div>
    </Transition>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { CommandGroupAPI } from '../../services/api'
import type { CommandGroup } from '../../services/api'
import { getLogger } from '@/utils/logger'

const logger = getLogger()

const isOpen = ref(false)
const isEditing = ref(false)
const isSaving = ref(false)
const localCommands = ref<string[]>([])
const editContent = ref('')
const lastSaved = ref('')

const commandCount = computed(() => localCommands.value.length)

async function loadCommands() {
  try {
    const groups = await CommandGroupAPI.listCommandGroups()
    const defaultGroup = (groups || []).find((group: CommandGroup) => group.name === '默认命令组')
    localCommands.value = defaultGroup?.commands || []
  } catch (err) {
    logger.error('加载命令失败', 'CommandEditor', err)
    localCommands.value = []
  }
}

function openPanel() {
  isOpen.value = true
  loadCommands()
}

function closePanel() {
  isOpen.value = false
  isEditing.value = false
}

function startEdit() {
  editContent.value = localCommands.value.join('\n')
  isEditing.value = true
}

function cancelEdit() {
  isEditing.value = false
  editContent.value = ''
}

async function saveCommands() {
  if (isSaving.value) return
  
  isSaving.value = true
  try {
    const lines = editContent.value.split('\n').filter(line => {
      const trimmed = line.trim()
      return trimmed !== '' && !trimmed.startsWith('#')
    })
    
    if (lines.length === 0) {
      alert('命令列表不能为空')
      isSaving.value = false
      return
    }

    const groups = await CommandGroupAPI.listCommandGroups()
    const defaultGroup = (groups || []).find((group: CommandGroup) => group.name === '默认命令组')

    if (defaultGroup) {
      await CommandGroupAPI.updateCommandGroup(defaultGroup.id, {
        ...defaultGroup,
        commands: lines,
      })
    } else {
      await CommandGroupAPI.createCommandGroup({
        id: 0,
        name: '默认命令组',
        description: '自动生成的默认命令组',
        commands: lines,
        tags: [],
        createdAt: '',
        updatedAt: '',
      } as any)
    }

    localCommands.value = lines
    isEditing.value = false
    editContent.value = ''
    lastSaved.value = new Date().toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })

    // 触发事件通知父组件
    emit('saved', lines)
  } catch (err: any) {
    logger.error('保存命令失败', 'CommandEditor', err)
    alert('保存失败: ' + (err.message || err))
  } finally {
    isSaving.value = false
  }
}

const emit = defineEmits<{
  saved: [commands: string[]]
}>()

onMounted(() => {
  loadCommands()
})
</script>

<style scoped lang="postcss">
@reference "../../styles/index.css";

.command-editor {
  @apply inline-flex;
}

/* 面板滑入滑出动画 - 特殊过渡，保留组件内定义 */
.slide-enter-active,
.slide-leave-active {
  transition: opacity 0.3s ease;
}

.slide-enter-active > div:last-child,
.slide-leave-active > div:last-child {
  transition: transform 0.3s ease;
}

.slide-enter-from,
.slide-leave-to {
  opacity: 0;
}

.slide-enter-from > div:last-child,
.slide-leave-to > div:last-child {
  transform: translateX(100%);
}

/* 终端颜色类已移至全局 index.css */
</style>
