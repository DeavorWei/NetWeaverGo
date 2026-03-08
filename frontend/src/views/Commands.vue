<template>
  <div class="animate-slide-in space-y-5 h-full flex flex-col">
    <!-- 标题栏 + 操作按钮 -->
    <div class="flex items-center justify-between flex-shrink-0">
      <p class="text-sm text-text-muted">管理命令组，支持创建、编辑、删除和导入导出</p>
      <div class="flex gap-3">
        <button
          @click="openCreateModal"
          class="flex items-center gap-2 px-4 py-2.5 rounded-lg text-sm font-semibold transition-all duration-200 shadow-card bg-accent hover:bg-accent-glow text-white border border-accent/30 hover:shadow-glow"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
            <line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/>
          </svg>
          新建命令组
        </button>
      </div>
    </div>

    <!-- 搜索和筛选栏 -->
    <div class="flex items-center gap-4 flex-shrink-0">
      <div class="relative flex-1 max-w-md">
        <svg xmlns="http://www.w3.org/2000/svg" class="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-text-muted" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/>
        </svg>
        <input
          v-model="searchKeyword"
          type="text"
          placeholder="搜索命令组名称或描述..."
          class="w-full pl-10 pr-4 py-2.5 rounded-lg bg-bg-card border border-border text-sm text-text-primary placeholder:text-text-muted focus:outline-none focus:border-accent/50 focus:ring-1 focus:ring-accent/20 transition-all"
        />
      </div>
      <div class="flex items-center gap-2">
        <span class="text-xs text-text-muted">标签筛选:</span>
        <select
          v-model="selectedTag"
          class="px-3 py-2 rounded-lg bg-bg-card border border-border text-sm text-text-primary focus:outline-none focus:border-accent/50 transition-all"
        >
          <option value="">全部</option>
          <option v-for="tag in allTags" :key="tag" :value="tag">{{ tag }}</option>
        </select>
      </div>
    </div>

    <!-- 命令组列表 -->
    <div class="flex-1 overflow-auto scrollbar-custom min-h-0">
      <div v-if="loading" class="flex items-center justify-center h-48">
        <div class="w-8 h-8 border-2 border-accent border-t-transparent rounded-full animate-spin"></div>
      </div>
      <div v-else-if="filteredGroups.length === 0" class="flex flex-col items-center justify-center h-48 text-text-muted gap-3">
        <svg xmlns="http://www.w3.org/2000/svg" class="w-12 h-12 text-text-muted/30" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
          <polyline points="4 17 10 11 4 5"/><line x1="12" y1="19" x2="20" y2="19"/>
        </svg>
        <p class="text-sm">暂无命令组，点击「新建命令组」开始创建</p>
      </div>
      <div v-else class="grid grid-cols-2 gap-4">
        <div
          v-for="group in filteredGroups"
          :key="group.id"
          class="bg-bg-card border border-border rounded-xl overflow-hidden shadow-card hover:border-accent/30 transition-all duration-300 group/card"
        >
          <!-- 卡片头部 -->
          <div class="flex items-start justify-between px-4 py-3 border-b border-border bg-bg-panel">
            <div class="flex-1 min-w-0">
              <div class="flex items-center gap-2">
                <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4 text-accent flex-shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <polyline points="4 17 10 11 4 5"/><line x1="12" y1="19" x2="20" y2="19"/>
                </svg>
                <h3 class="text-sm font-semibold text-text-primary truncate">{{ group.name }}</h3>
              </div>
              <p class="text-xs text-text-muted mt-1 line-clamp-1">{{ group.description || '暂无描述' }}</p>
            </div>
            <div class="flex items-center gap-1 opacity-0 group-hover/card:opacity-100 transition-opacity">
              <button
                @click="openEditModal(group)"
                class="p-1.5 rounded-md text-text-muted hover:text-accent hover:bg-accent/10 transition-colors"
                title="编辑"
              >
                <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/>
                  <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/>
                </svg>
              </button>
              <button
                @click="duplicateGroup(group.id)"
                class="p-1.5 rounded-md text-text-muted hover:text-success hover:bg-success/10 transition-colors"
                title="复制"
              >
                <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <rect x="9" y="9" width="13" height="13" rx="2" ry="2"/>
                  <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/>
                </svg>
              </button>
              <button
                @click="confirmDelete(group)"
                class="p-1.5 rounded-md text-text-muted hover:text-error hover:bg-error/10 transition-colors"
                title="删除"
              >
                <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <polyline points="3 6 5 6 21 6"/>
                  <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/>
                </svg>
              </button>
            </div>
          </div>
          
          <!-- 标签 -->
          <div class="px-4 py-2 flex flex-wrap gap-1.5">
            <span
              v-for="tag in group.tags"
              :key="tag"
              class="px-2 py-0.5 text-xs rounded-full bg-accent/10 text-accent border border-accent/20"
            >
              {{ tag }}
            </span>
            <span v-if="!group.tags || group.tags.length === 0" class="text-xs text-text-muted">无标签</span>
          </div>
          
          <!-- 命令预览 -->
          <div class="px-4 py-3 border-t border-border">
            <div class="flex items-center justify-between text-xs text-text-muted mb-2">
              <span>命令列表 ({{ group.commands.length }} 条)</span>
              <button
                @click="togglePreview(group.id)"
                class="text-accent hover:text-accent-glow transition-colors"
              >
                {{ expandedPreviews.has(group.id) ? '收起' : '展开' }}
              </button>
            </div>
            <div
              v-if="expandedPreviews.has(group.id)"
              class="bg-bg-secondary/50 rounded-lg p-2 font-mono text-xs max-h-40 overflow-y-auto scrollbar-custom"
            >
              <div
                v-for="(cmd, idx) in group.commands"
                :key="idx"
                class="py-1 text-text-secondary border-b border-border/30 last:border-0"
              >
                <span class="text-text-muted mr-2">{{ idx + 1 }}.</span>{{ cmd }}
              </div>
            </div>
            <div v-else class="text-xs text-text-muted line-clamp-2 font-mono">
              {{ group.commands.slice(0, 2).join('; ') }}{{ group.commands.length > 2 ? '...' : '' }}
            </div>
          </div>
          
          <!-- 卡片底部 -->
          <div class="px-4 py-2 border-t border-border bg-bg-secondary/30 text-xs text-text-muted flex items-center justify-between">
            <span>更新于: {{ formatDate(group.updatedAt) }}</span>
          </div>
        </div>
      </div>
    </div>

    <!-- 编辑/创建弹窗 -->
    <Transition name="modal">
      <div v-if="editModal.show" class="fixed inset-0 z-50 flex items-center justify-center">
        <div class="absolute inset-0 bg-black/60 backdrop-blur-sm" @click="closeEditModal"></div>
        <div class="relative bg-bg-card border border-border rounded-xl shadow-2xl max-w-2xl w-full mx-4 max-h-[90vh] overflow-hidden animate-slide-in flex flex-col">
          <!-- 弹窗头部 -->
          <div class="flex items-center justify-between px-5 py-4 border-b border-border bg-bg-panel flex-shrink-0">
            <div class="flex items-center gap-3">
              <div class="w-9 h-9 rounded-lg bg-accent/15 flex items-center justify-center">
                <svg xmlns="http://www.w3.org/2000/svg" class="w-5 h-5 text-accent" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <polyline points="4 17 10 11 4 5"/><line x1="12" y1="19" x2="20" y2="19"/>
                </svg>
              </div>
              <div>
                <h3 class="text-sm font-semibold text-text-primary">{{ editModal.isCreate ? '新建命令组' : '编辑命令组' }}</h3>
                <p class="text-xs text-text-muted mt-0.5">{{ editModal.isCreate ? '创建新的命令组' : '修改命令组内容和设置' }}</p>
              </div>
            </div>
            <button
              @click="closeEditModal"
              class="p-2 rounded-lg text-text-muted hover:text-text-primary hover:bg-bg-secondary transition-colors"
            >
              <svg xmlns="http://www.w3.org/2000/svg" class="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/>
              </svg>
            </button>
          </div>

          <!-- 表单内容 -->
          <div class="flex-1 overflow-y-auto scrollbar-custom p-5 space-y-4">
            <!-- 名称 -->
            <div class="space-y-1.5">
              <label class="text-sm font-medium text-text-primary">名称 <span class="text-error">*</span></label>
              <input
                v-model="editModal.form.name"
                type="text"
                placeholder="输入命令组名称"
                class="w-full px-4 py-2.5 rounded-lg bg-bg-secondary border border-border text-sm text-text-primary placeholder:text-text-muted focus:outline-none focus:border-accent/50 focus:ring-1 focus:ring-accent/20 transition-all"
              />
            </div>

            <!-- 描述 -->
            <div class="space-y-1.5">
              <label class="text-sm font-medium text-text-primary">描述</label>
              <input
                v-model="editModal.form.description"
                type="text"
                placeholder="输入命令组描述（可选）"
                class="w-full px-4 py-2.5 rounded-lg bg-bg-secondary border border-border text-sm text-text-primary placeholder:text-text-muted focus:outline-none focus:border-accent/50 focus:ring-1 focus:ring-accent/20 transition-all"
              />
            </div>

            <!-- 标签 -->
            <div class="space-y-1.5">
              <label class="text-sm font-medium text-text-primary">标签</label>
              <div class="flex flex-wrap gap-2 mb-2">
                <span
                  v-for="(tag, idx) in editModal.form.tags"
                  :key="idx"
                  class="inline-flex items-center gap-1 px-2.5 py-1 text-xs rounded-full bg-accent/10 text-accent border border-accent/20"
                >
                  {{ tag }}
                  <button @click="removeTag(idx)" class="hover:text-error transition-colors">
                    <svg xmlns="http://www.w3.org/2000/svg" class="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3">
                      <line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/>
                    </svg>
                  </button>
                </span>
              </div>
              <div class="flex gap-2">
                <input
                  v-model="newTag"
                  type="text"
                  placeholder="添加标签"
                  class="flex-1 px-3 py-2 rounded-lg bg-bg-secondary border border-border text-sm text-text-primary placeholder:text-text-muted focus:outline-none focus:border-accent/50 transition-all"
                  @keyup.enter="addTag"
                />
                <button
                  @click="addTag"
                  class="px-3 py-2 rounded-lg bg-accent/10 border border-accent/30 text-accent text-sm font-medium hover:bg-accent/20 transition-colors"
                >
                  添加
                </button>
              </div>
            </div>

            <!-- 命令列表 -->
            <div class="space-y-1.5">
              <label class="text-sm font-medium text-text-primary">命令列表 <span class="text-error">*</span></label>
              <div class="border border-border rounded-lg overflow-hidden">
                <textarea
                  v-model="editModal.form.commandsText"
                  class="w-full h-64 p-4 bg-terminal-bg text-terminal-text font-mono text-sm leading-relaxed resize-none border-none outline-none scrollbar-custom"
                  placeholder="每行输入一条命令&#10;以 # 开头的行为注释&#10;空行将被忽略"
                  spellcheck="false"
                ></textarea>
              </div>
              <p class="text-xs text-text-muted">提示：每行一条命令，以 # 开头的行为注释，空行将被自动忽略</p>
            </div>
          </div>

          <!-- 操作按钮 -->
          <div class="flex justify-end gap-3 px-5 py-4 border-t border-border bg-bg-panel flex-shrink-0">
            <button
              @click="closeEditModal"
              class="px-4 py-2.5 rounded-lg text-sm font-medium bg-bg-secondary border border-border text-text-secondary hover:text-text-primary hover:border-accent/50 transition-all"
            >
              取消
            </button>
            <button
              @click="saveGroup"
              :disabled="editModal.saving"
              class="flex items-center gap-2 px-5 py-2.5 rounded-lg text-sm font-semibold bg-accent border border-accent/50 text-white hover:bg-accent-glow transition-all disabled:opacity-50"
            >
              <svg v-if="editModal.saving" class="w-4 h-4 animate-spin" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <circle cx="12" cy="12" r="10" stroke-opacity="0.25"/>
                <path d="M12 2a10 10 0 0 1 10 10" stroke-opacity="1"/>
              </svg>
              {{ editModal.saving ? '保存中...' : '保存' }}
            </button>
          </div>
        </div>
      </div>
    </Transition>

    <!-- 删除确认弹窗 -->
    <Transition name="modal">
      <div v-if="deleteModal.show" class="fixed inset-0 z-50 flex items-center justify-center">
        <div class="absolute inset-0 bg-black/60 backdrop-blur-sm" @click="closeDeleteModal"></div>
        <div class="relative bg-bg-card border border-error/50 rounded-xl shadow-2xl max-w-sm w-full mx-4 overflow-hidden animate-slide-in">
          <!-- 弹窗头部 -->
          <div class="flex items-center gap-3 px-5 py-4 border-b border-border bg-error/5">
            <div class="w-9 h-9 rounded-lg bg-error/15 flex items-center justify-center flex-shrink-0">
              <svg xmlns="http://www.w3.org/2000/svg" class="w-5 h-5 text-error" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/>
              </svg>
            </div>
            <div>
              <h3 class="text-sm font-semibold text-text-primary">确认删除</h3>
              <p class="text-xs text-text-muted mt-0.5">此操作无法撤销</p>
            </div>
          </div>
          <!-- 内容 -->
          <div class="px-5 py-4">
            <p class="text-sm text-text-secondary">
              确定要删除命令组 <span class="font-semibold text-text-primary">「{{ deleteModal.groupName }}」</span> 吗？
            </p>
          </div>
          <!-- 操作按钮 -->
          <div class="flex justify-end gap-3 px-5 py-4 border-t border-border">
            <button
              @click="closeDeleteModal"
              class="px-4 py-2.5 rounded-lg text-sm font-medium bg-bg-secondary border border-border text-text-secondary hover:text-text-primary transition-all"
            >
              取消
            </button>
            <button
              @click="deleteGroup"
              :disabled="deleteModal.deleting"
              class="px-4 py-2.5 rounded-lg text-sm font-semibold bg-error border border-error/50 text-white hover:bg-error/80 transition-all disabled:opacity-50"
            >
              {{ deleteModal.deleting ? '删除中...' : '确认删除' }}
            </button>
          </div>
        </div>
      </div>
    </Transition>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import type { CommandGroup } from '../types/command'
// @ts-ignore
import { 
  ListCommandGroups, 
  CreateCommandGroup, 
  UpdateCommandGroup, 
  DeleteCommandGroup as DeleteCommandGroupAPI,
  DuplicateCommandGroup as DuplicateCommandGroupAPI
} from '../bindings/github.com/NetWeaverGo/core/internal/ui/appservice.js'

const loading = ref(true)
const groups = ref<CommandGroup[]>([])
const searchKeyword = ref('')
const selectedTag = ref('')
const expandedPreviews = ref(new Set<string>())

// 编辑弹窗状态
const editModal = ref({
  show: false,
  isCreate: true,
  editingId: '',
  form: {
    name: '',
    description: '',
    tags: [] as string[],
    commandsText: ''
  },
  saving: false
})

const newTag = ref('')

// 删除弹窗状态
const deleteModal = ref({
  show: false,
  groupId: '',
  groupName: '',
  deleting: false
})

// 计算所有标签
const allTags = computed(() => {
  const tagSet = new Set<string>()
  groups.value.forEach(g => {
    g.tags?.forEach(t => tagSet.add(t))
  })
  return Array.from(tagSet)
})

// 过滤后的命令组列表
const filteredGroups = computed(() => {
  let result = groups.value
  
  if (searchKeyword.value) {
    const keyword = searchKeyword.value.toLowerCase()
    result = result.filter(g => 
      g.name.toLowerCase().includes(keyword) ||
      g.description?.toLowerCase().includes(keyword)
    )
  }
  
  if (selectedTag.value) {
    result = result.filter(g => g.tags?.includes(selectedTag.value))
  }
  
  return result
})

// 格式化日期
function formatDate(dateStr: string) {
  if (!dateStr) return '-'
  try {
    const date = new Date(dateStr)
    return date.toLocaleString('zh-CN', { 
      year: 'numeric', 
      month: '2-digit', 
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit'
    })
  } catch {
    return dateStr
  }
}

// 切换预览展开状态
function togglePreview(id: string) {
  if (expandedPreviews.value.has(id)) {
    expandedPreviews.value.delete(id)
  } else {
    expandedPreviews.value.add(id)
  }
}

// 加载命令组列表
async function loadGroups() {
  loading.value = true
  try {
    const result = await ListCommandGroups()
    groups.value = result || []
  } catch (err) {
    console.error('加载命令组失败:', err)
    groups.value = []
  } finally {
    loading.value = false
  }
}

// 打开创建弹窗
function openCreateModal() {
  editModal.value = {
    show: true,
    isCreate: true,
    editingId: '',
    form: {
      name: '',
      description: '',
      tags: [],
      commandsText: ''
    },
    saving: false
  }
  newTag.value = ''
}

// 打开编辑弹窗
function openEditModal(group: CommandGroup) {
  editModal.value = {
    show: true,
    isCreate: false,
    editingId: group.id,
    form: {
      name: group.name,
      description: group.description || '',
      tags: [...(group.tags || [])],
      commandsText: group.commands.join('\n')
    },
    saving: false
  }
  newTag.value = ''
}

// 关闭编辑弹窗
function closeEditModal() {
  editModal.value.show = false
}

// 添加标签
function addTag() {
  const tag = newTag.value.trim()
  if (tag && !editModal.value.form.tags.includes(tag)) {
    editModal.value.form.tags.push(tag)
  }
  newTag.value = ''
}

// 移除标签
function removeTag(index: number) {
  editModal.value.form.tags.splice(index, 1)
}

// 保存命令组
async function saveGroup() {
  if (editModal.value.saving) return
  
  const { name, description, tags, commandsText } = editModal.value.form
  
  if (!name.trim()) {
    alert('请输入命令组名称')
    return
  }
  
  // 解析命令
  const commands = commandsText.split('\n')
    .map(line => line.trim())
    .filter(line => line !== '' && !line.startsWith('#'))
  
  if (commands.length === 0) {
    alert('命令列表不能为空')
    return
  }
  
  editModal.value.saving = true
  
  try {
    const groupData = {
      name: name.trim(),
      description: description.trim(),
      tags: tags,
      commands: commands
    }
    
    if (editModal.value.isCreate) {
      await CreateCommandGroup(groupData)
    } else {
      await UpdateCommandGroup(editModal.value.editingId, groupData)
    }
    
    closeEditModal()
    await loadGroups()
  } catch (err: any) {
    console.error('保存命令组失败:', err)
    alert('保存失败: ' + (err.message || err))
  } finally {
    editModal.value.saving = false
  }
}

// 复制命令组
async function duplicateGroup(id: string) {
  try {
    await DuplicateCommandGroupAPI(id)
    await loadGroups()
  } catch (err: any) {
    console.error('复制命令组失败:', err)
    alert('复制失败: ' + (err.message || err))
  }
}

// 确认删除
function confirmDelete(group: CommandGroup) {
  deleteModal.value = {
    show: true,
    groupId: group.id,
    groupName: group.name,
    deleting: false
  }
}

// 关闭删除弹窗
function closeDeleteModal() {
  deleteModal.value.show = false
}

// 执行删除
async function deleteGroup() {
  if (deleteModal.value.deleting) return
  
  deleteModal.value.deleting = true
  
  try {
    await DeleteCommandGroupAPI(deleteModal.value.groupId)
    closeDeleteModal()
    await loadGroups()
  } catch (err: any) {
    console.error('删除命令组失败:', err)
    alert('删除失败: ' + (err.message || err))
  } finally {
    deleteModal.value.deleting = false
  }
}

onMounted(() => {
  loadGroups()
})
</script>

<style scoped>
.line-clamp-1 {
  display: -webkit-box;
  -webkit-line-clamp: 1;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.line-clamp-2 {
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.modal-enter-active,
.modal-leave-active {
  transition: opacity 0.2s ease;
}

.modal-enter-from,
.modal-leave-to {
  opacity: 0;
}

.bg-terminal-bg {
  background-color: var(--color-terminal-bg, #1a1a1a);
}

.text-terminal-text {
  color: var(--color-terminal-text, #e0e0e0);
}
</style>
