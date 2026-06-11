<template>
  <div class="animate-slide-in space-y-5 h-full flex flex-col">
    <!-- 标题栏 + 操作按钮 -->
    <div class="flex items-center justify-between flex-shrink-0">
      <p class="text-sm text-text-muted">管理命令组，支持创建、编辑、删除和导入导出</p>
      <div class="flex gap-3">
        <el-button type="primary" :icon="Plus" @click="openCreateModal">
          新建命令组
        </el-button>
      </div>
    </div>

    <!-- 搜索和筛选栏 -->
    <div class="flex items-center gap-4 flex-shrink-0">
      <el-input
        v-model="searchKeyword"
        placeholder="搜索命令组名称或描述..."
        :prefix-icon="Search"
        clearable
        class="max-w-md"
      />
      <div class="flex items-center gap-2">
        <span class="text-xs text-text-muted whitespace-nowrap">标签筛选:</span>
        <el-select v-model="selectedTag" clearable placeholder="全部" class="w-32">
          <el-option v-for="tag in allTags" :key="tag" :label="tag" :value="tag" />
        </el-select>
      </div>
    </div>

    <!-- 命令组列表 -->
    <div class="flex-1 overflow-auto scrollbar-custom min-h-0" v-loading="loading">
      <el-empty v-if="!loading && filteredGroups.length === 0" description="暂无命令组，点击「新建命令组」开始创建" />
      <div v-else class="grid grid-cols-2 gap-4">
        <el-card
          v-for="group in filteredGroups"
          :key="group.id"
          class="group/card"
          :body-style="{ padding: '0px' }"
          shadow="hover"
        >
          <!-- 卡片头部 -->
          <div class="flex items-start justify-between px-4 py-3 border-b border-border bg-bg-panel">
            <div class="flex-1 min-w-0">
              <div class="flex items-center justify-between gap-2">
                <div class="flex items-center gap-2 min-w-0">
                  <el-icon class="text-accent"><Document /></el-icon>
                  <h3 class="text-sm font-semibold text-text-primary truncate">{{ group.name }}</h3>
                </div>
                <!-- 标签贴近右侧 -->
                <div class="flex flex-wrap gap-1 justify-end flex-shrink-0">
                  <el-tag v-for="tag in group.tags?.slice(0, 2)" :key="tag" size="small" type="info">{{ tag }}</el-tag>
                  <el-tag v-if="group.tags && group.tags.length > 2" size="small" type="info">+{{ group.tags.length - 2 }}</el-tag>
                </div>
              </div>
              <p class="text-xs text-text-muted line-clamp-1 mt-1">{{ group.description || '暂无描述' }}</p>
            </div>
            <div class="flex items-center gap-1 opacity-0 group-hover/card:opacity-100 transition-opacity ml-2">
              <el-button link type="primary" :icon="Edit" @click="openEditModal(group)" title="编辑" />
              <el-button link type="success" :icon="DocumentCopy" @click="duplicateGroup(group.id)" title="复制" />
              <el-button link type="danger" :icon="Delete" @click="confirmDelete(group)" title="删除" />
            </div>
          </div>
          
          <!-- 命令预览 -->
          <div class="px-4 py-3 border-t border-border">
            <div class="flex items-center justify-between text-xs text-text-muted mb-2">
              <span>命令列表 ({{ group.commands.length }} 条)</span>
              <el-button link type="primary" size="small" :icon="View" @click="openPreviewModal(group)">预览</el-button>
            </div>
            <div class="text-xs text-text-muted line-clamp-2 font-mono">
              {{ group.commands.slice(0, 2).join('; ') }}{{ group.commands.length > 2 ? '...' : '' }}
            </div>
          </div>
          
          <!-- 卡片底部 -->
          <div class="px-4 py-2 border-t border-border bg-bg-secondary/30 text-xs text-text-muted flex items-center justify-between">
            <span>更新于: {{ formatDate(group.updatedAt) }}</span>
          </div>
        </el-card>
      </div>
    </div>

    <!-- 编辑/创建弹窗 -->
    <el-dialog
      v-model="editModal.show"
      :title="editModal.isCreate ? '新建命令组' : '编辑命令组'"
      width="600px"
      destroy-on-close
      :close-on-click-modal="false"
    >
      <el-form :model="editModal.form" label-width="80px" label-position="top">
        <el-form-item label="名称" required>
          <el-input v-model="editModal.form.name" placeholder="输入命令组名称" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="editModal.form.description" placeholder="输入命令组描述（可选）" />
        </el-form-item>
        <el-form-item label="标签">
          <div class="flex flex-col gap-2 w-full">
            <div class="flex flex-wrap gap-2">
              <el-tag
                v-for="(tag, idx) in editModal.form.tags"
                :key="idx"
                closable
                @close="removeTag(idx)"
              >
                {{ tag }}
              </el-tag>
            </div>
            <div class="flex gap-2">
              <el-input
                v-model="newTag"
                placeholder="添加标签"
                @keyup.enter="addTag"
                class="flex-1"
              />
              <el-button @click="addTag">添加</el-button>
            </div>
          </div>
        </el-form-item>
        <el-form-item label="命令列表" required>
          <el-input
            v-model="editModal.form.commandsText"
            type="textarea"
            :rows="10"
            placeholder="每行输入一条命令&#10;以 # 开头的行为注释&#10;空行将被忽略"
            class="font-mono text-sm"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <div class="flex justify-end gap-2">
          <el-button @click="closeEditModal">取消</el-button>
          <el-button type="primary" :loading="editModal.saving" @click="saveGroup">
            保存
          </el-button>
        </div>
      </template>
    </el-dialog>

    <!-- 命令预览弹窗 -->
    <el-dialog
      v-model="previewModal.show"
      title="命令组预览"
      width="700px"
      draggable
      destroy-on-close
    >
      <template #header>
        <div class="flex items-center gap-3">
          <div class="w-8 h-8 rounded-lg bg-accent/15 flex items-center justify-center">
            <el-icon class="text-accent text-lg"><View /></el-icon>
          </div>
          <div>
            <h3 class="text-sm font-semibold text-text-primary">命令组预览</h3>
            <p class="text-xs text-text-muted mt-0.5">{{ previewModal.group?.name }}</p>
          </div>
        </div>
      </template>

      <div class="flex gap-4 mb-4">
        <div class="flex-1 space-y-1">
          <label class="text-xs font-medium text-text-muted uppercase">描述</label>
          <p class="text-sm text-text-secondary">{{ previewModal.group?.description || '暂无描述' }}</p>
        </div>
        <div class="flex-1 space-y-1">
          <label class="text-xs font-medium text-text-muted uppercase">标签</label>
          <div class="flex flex-wrap gap-1">
            <el-tag v-for="tag in previewModal.group?.tags" :key="tag" size="small" type="info">{{ tag }}</el-tag>
            <span v-if="!previewModal.group?.tags || previewModal.group.tags.length === 0" class="text-xs text-text-muted">无标签</span>
          </div>
        </div>
      </div>

      <div class="space-y-1 flex-1 flex flex-col h-64">
        <label class="text-xs font-medium text-text-muted uppercase">命令列表 ({{ previewModal.group?.commands.length }} 条)</label>
        <div class="border border-border rounded-lg overflow-auto bg-terminal-bg h-full p-2 font-mono text-sm">
          <div
            v-for="(cmd, idx) in previewModal.group?.commands"
            :key="idx"
            class="group/cmd flex items-start gap-2 px-2 py-1.5 hover:bg-text-inverse/5 transition-colors"
          >
            <span class="text-text-muted/50 w-6 text-right select-none mt-0.5">{{ idx + 1 }}</span>
            <span class="flex-1 text-terminal-text break-all mt-0.5">{{ cmd }}</span>
            <el-button link type="primary" :icon="DocumentCopy" @click="copyCommand(cmd)" class="opacity-0 group-hover/cmd:opacity-100" />
          </div>
        </div>
      </div>

      <template #footer>
        <div class="flex justify-between items-center w-full">
          <span class="text-xs text-text-muted">更新于 {{ formatDate(previewModal.group?.updatedAt || '') }}</span>
          <div class="flex gap-2">
            <el-button @click="closePreviewModal">关闭</el-button>
            <el-button type="primary" :icon="DocumentCopy" @click="copyAllCommands">复制全部</el-button>
          </div>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Search, Document, Edit, DocumentCopy, Delete, View } from '@element-plus/icons-vue'
import type { CommandGroup } from '../services/api'
import { CommandGroupAPI } from '../services/api'
import { getLogger } from '@/utils/logger'

const logger = getLogger()

// 提取错误消息的辅助函数（修复问题08：前端错误处理不完整）
function extractErrorMessage(err: any): string {
  if (!err) return '未知错误'
  
  // 标准 Error 对象
  if (err instanceof Error) return err.message
  
  // 带 message 属性的对象
  if (typeof err === 'object' && err.message) return String(err.message)
  
  // 字符串
  if (typeof err === 'string') return err
  
  // 其他情况：尝试 JSON 序列化
  try {
    return JSON.stringify(err)
  } catch {
    return '未知错误'
  }
}

const loading = ref(true)
const groups = ref<CommandGroup[]>([])
const searchKeyword = ref('')
const selectedTag = ref('')

// 编辑弹窗状态
const editModal = ref({
  show: false,
  isCreate: true,
  editingId: 0 as number,
  form: {
    name: '',
    description: '',
    tags: [] as string[],
    commandsText: ''
  },
  saving: false
})

const newTag = ref('')

// 预览弹窗状态
const previewModal = ref({
  show: false,
  group: null as CommandGroup | null
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

// 加载命令组列表
async function loadGroups() {
  loading.value = true
  try {
    const result = await CommandGroupAPI.listCommandGroups()
    groups.value = result || []
  } catch (err) {
    logger.error('加载命令组失败', 'Commands', err)
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
    editingId: 0,
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

// 添加标签（修复问题12：前端标签验证不完整）
function addTag() {
  const tag = newTag.value.trim()
  
  // 验证标签
  if (!tag) return
  
  // 长度验证
  if (tag.length > 20) {
    ElMessage.warning('标签长度不能超过20个字符')
    return
  }
  
  // 字符集验证（中文、英文、数字、下划线、短横线）
  const validPattern = /^[\u4e00-\u9fa5a-zA-Z0-9_-]+$/
  if (!validPattern.test(tag)) {
    ElMessage.warning('标签只能包含中文、英文、数字、下划线和短横线')
    return
  }
  
  // 去重验证
  if (editModal.value.form.tags.includes(tag)) {
    ElMessage.warning('标签已存在')
    return
  }
  
  editModal.value.form.tags.push(tag)
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
    ElMessage.warning('请输入命令组名称')
    return
  }
  
  // 解析命令
  const commands = commandsText.split('\n')
    .map(line => line.trim())
    .filter(line => line !== '' && !line.startsWith('#'))
  
  if (commands.length === 0) {
    ElMessage.warning('命令列表不能为空')
    return
  }
  
  editModal.value.saving = true
  
  try {
    const groupData = {
      name: name.trim(),
      description: description.trim(),
      tags: tags,
      commands: commands
    } as Partial<CommandGroup>
    
    if (editModal.value.isCreate) {
      await CommandGroupAPI.createCommandGroup(groupData as CommandGroup)
      ElMessage.success('命令组创建成功')
    } else {
      await CommandGroupAPI.updateCommandGroup(editModal.value.editingId, groupData as CommandGroup)
      ElMessage.success('命令组更新成功')
    }
    
    closeEditModal()
    await loadGroups()
  } catch (err: any) {
    logger.error('保存命令组失败', 'Commands', err)
    ElMessage.error('保存失败: ' + extractErrorMessage(err))
  } finally {
    editModal.value.saving = false
  }
}

// 复制命令组
async function duplicateGroup(id: number) {
  try {
    await CommandGroupAPI.duplicateCommandGroup(id)
    ElMessage.success('命令组复制成功')
    await loadGroups()
  } catch (err: any) {
    logger.error('复制命令组失败', 'Commands', err)
    ElMessage.error('复制失败: ' + extractErrorMessage(err))
  }
}

// 确认删除
function confirmDelete(group: CommandGroup) {
  ElMessageBox.confirm(
    `确定要删除命令组「${group.name}」吗？此操作无法撤销。`,
    '确认删除',
    {
      confirmButtonText: '删除',
      cancelButtonText: '取消',
      type: 'warning',
      confirmButtonClass: 'el-button--danger'
    }
  ).then(async () => {
    try {
      await CommandGroupAPI.deleteCommandGroup(group.id)
      ElMessage.success('命令组删除成功')
      await loadGroups()
    } catch (err: any) {
      logger.error('删除命令组失败', 'Commands', err)
      ElMessage.error('删除失败: ' + extractErrorMessage(err))
    }
  }).catch(() => {})
}

// 打开预览弹窗
function openPreviewModal(group: CommandGroup) {
  previewModal.value = {
    show: true,
    group: group
  }
}

// 关闭预览弹窗
function closePreviewModal() {
  previewModal.value.show = false
}

// 复制单条命令
async function copyCommand(command: string) {
  try {
    await navigator.clipboard.writeText(command)
    ElMessage.success('命令已复制到剪贴板')
  } catch (err) {
    logger.error('复制失败', 'Commands', err)
    ElMessage.error('复制失败')
  }
}

// 复制全部命令
async function copyAllCommands() {
  if (!previewModal.value.group) return
  try {
    const commands = previewModal.value.group.commands.join('\n')
    await navigator.clipboard.writeText(commands)
    ElMessage.success('已复制全部命令')
  } catch (err) {
    logger.error('复制失败', 'Commands', err)
    ElMessage.error('复制失败')
  }
}

onMounted(() => {
  loadGroups()
})
</script>

<style scoped>
</style>
