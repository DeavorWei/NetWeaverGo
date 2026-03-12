<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import { CreateCommandGroup, CreateTaskGroup, ForgeAPI } from '../../services/api'
import type { CommandGroup } from '../../bindings/github.com/NetWeaverGo/core/internal/config/models'
import type { VarInput, BuildResult, ForgeIPValidationResult } from '../../bindings/github.com/NetWeaverGo/core/internal/ui/forgeservice'

const router = useRouter()

const showSyntaxHelp = ref(false)
const showUsageHelp = ref(false)

const windowWidth = ref(window.innerWidth)

const handleResize = () => {
  windowWidth.value = window.innerWidth
}

onMounted(() => {
  window.addEventListener('resize', handleResize)
})

onUnmounted(() => {
  window.removeEventListener('resize', handleResize)
})

// 三列的宽度状态 (百分比)
const leftColWidth = ref(35)
const midColWidth = ref(30)
const rightColWidth = ref(35)

// 拖拽相关状态
const isResizing = ref(false)
const resizeType = ref<'left' | 'right' | null>(null)

const startResize = (type: 'left' | 'right') => {
  isResizing.value = true
  resizeType.value = type
  document.body.style.cursor = 'col-resize'
  window.addEventListener('mousemove', doResize)
  window.addEventListener('mouseup', stopResize)
}

const workspaceRef = ref<HTMLElement | null>(null)

const doResize = (e: MouseEvent) => {
  if (!isResizing.value || !resizeType.value || !workspaceRef.value) return
  
  const rect = workspaceRef.value.getBoundingClientRect()
  const containerWidth = rect.width
  const mouseX = e.clientX - rect.left
  
  const mousePct = (mouseX / containerWidth) * 100
  const minW = 10
  
  if (resizeType.value === 'left') {
    if (mousePct > minW && mousePct < (leftColWidth.value + midColWidth.value - minW)) {
      const diff = mousePct - leftColWidth.value
      leftColWidth.value += diff
      midColWidth.value -= diff
    }
  } else if (resizeType.value === 'right') {
    const leftBound = leftColWidth.value
    if (mousePct > (leftBound + minW) && mousePct < (100 - minW)) {
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

// ==================== 核心状态 ====================
// 前端只维护表单状态，所有计算逻辑由后端处理

const templateText = ref('')
const variables = ref<VarInput[]>([
  { name: '[A]', valueString: '' },
  { name: '[B]', valueString: '' },
  { name: '[C]', valueString: '' },
  { name: '[D]', valueString: '' },
])
const buildResult = ref<BuildResult | null>(null)
const isBuilding = ref(false)
const isCopied = ref(false)

// ==================== 后端调用 ====================

// 防抖定时器
let debounceTimer: ReturnType<typeof setTimeout> | null = null

// 调用后端构建配置
const buildConfig = async () => {
  if (!templateText.value.trim()) {
    buildResult.value = null
    return
  }

  isBuilding.value = true
  try {
    const result = await ForgeAPI.buildConfig({
      template: templateText.value,
      variables: variables.value.filter(v => v.valueString.trim() !== '' || v.name === '[A]'), // 保留有值的变量
    })
    buildResult.value = result
    isCopied.value = false
  } catch (err) {
    console.error('构建配置失败:', err)
    buildResult.value = null
  } finally {
    isBuilding.value = false
  }
}

// 自动生成模式监听
watch([templateText, variables], () => {
  if (debounceTimer) clearTimeout(debounceTimer)
  debounceTimer = setTimeout(() => {
    buildConfig()
  }, 300)
}, { deep: true })

// 输出块（直接使用后端返回的结果）
const outputBlocks = computed(() => buildResult.value?.blocks ?? [])

// ==================== 语法糖展开（后端调用） ====================

const expandSyntaxSugar = async (v: VarInput) => {
  if (!v.valueString) return
  
  // 获取其他变量的最大长度作为目标长度
  const otherVars = variables.value.filter(item => item.name !== v.name)
  let maxLen = 0
  for (const otherVar of otherVars) {
    const vals = otherVar.valueString.split(/,|\n/).filter(s => s.trim() !== '')
    if (vals.length > maxLen) maxLen = vals.length
  }
  
  try {
    const result = await ForgeAPI.expandValues({
      valueString: v.valueString,
      maxLen: maxLen,
    })
    if (result.hasExpanded || result.hasInferred) {
      v.valueString = result.values.join(', ')
    }
  } catch (err) {
    console.error('展开语法糖失败:', err)
  }
}

// ==================== 动态变量管理 ====================

const nextVarIndex = ref(4)

const getColumnName = (index: number) => {
  let name = ''
  let num = index
  while (num >= 0) {
    name = String.fromCharCode((num % 26) + 65) + name
    num = Math.floor(num / 26) - 1
  }
  return name
}

const addVariable = () => {
  const newName = getColumnName(nextVarIndex.value++)
  variables.value.push({ name: `[${newName}]`, valueString: '' })
}

const removeVariable = (idx: number) => {
  variables.value.splice(idx, 1)
}

// ==================== BindingDeviceIP 模式 ====================

// 检测是否为绑定模式（前端简化检测，后端也会验证）
const isBindingMode = computed(() => {
  const firstLine = templateText.value.split('\n')[0]?.trim() || ''
  return firstLine.includes('[BindingDeviceIP]')
})

// 当检测到 BindingDeviceIP 时，自动将第一个变量名设为 [BindingDeviceIP]
watch(isBindingMode, (isBinding) => {
  if (isBinding && variables.value.length > 0) {
    const firstVar = variables.value[0]
    if (firstVar) firstVar.name = '[BindingDeviceIP]'
  }
})

// IP 列表验证结果缓存
const ipValidationCache = ref<Map<string, ForgeIPValidationResult>>(new Map())

// 获取无效的 IP 列表
const invalidIPs = computed(() => {
  if (!isBindingMode.value || variables.value.length === 0) return []
  const firstVar = variables.value[0]
  if (!firstVar) return []
  
  const ipValues = firstVar.valueString
    .split(/,|\n/)
    .map(s => s.trim())
    .filter(s => s !== '')
  
  return ipValues.filter(ip => {
    const cached = ipValidationCache.value.get(ip)
    return cached && !cached.isValid
  })
})

const hasInvalidIP = computed(() => invalidIPs.value.length > 0)

// 更新 IP 验证缓存
watch(() => variables.value[0]?.valueString, async (newValue) => {
  if (!isBindingMode.value || !newValue) return
  
  const ipValues = newValue
    .split(/,|\n/)
    .map(s => s.trim())
    .filter(s => s !== '')
  
  for (const ip of ipValues) {
    if (!ipValidationCache.value.has(ip)) {
      try {
        const result = await ForgeAPI.validateIP(ip)
        ipValidationCache.value.set(ip, result)
      } catch {
        // ignore
      }
    }
  }
}, { immediate: true })

// 绑定预览（使用后端服务）
const bindingPreview = ref<{ip: string, commands: string}[]>([])

watch([isBindingMode, () => variables.value[0]?.valueString, templateText], async () => {
  if (!isBindingMode.value || variables.value.length === 0) {
    bindingPreview.value = []
    return
  }
  
  try {
    const result = await ForgeAPI.generateBindingPreview(templateText.value, variables.value)
    bindingPreview.value = result
  } catch (err) {
    console.error('生成绑定预览失败:', err)
    bindingPreview.value = []
  }
}, { deep: true })

// ==================== Toast 通知 ====================

const toastMessage = ref('')
const showToast = ref(false)
let toastTimer: ReturnType<typeof setTimeout> | null = null

const triggerToast = (msg: string) => {
  toastMessage.value = msg
  showToast.value = true
  if (toastTimer) clearTimeout(toastTimer)
  toastTimer = setTimeout(() => {
    showToast.value = false
  }, 3000)
}

// ==================== 发送到命令管理 ====================

const sendModal = ref({
  show: false,
  mode: 'merge' as 'merge' | 'split',
  saving: false,
  form: {
    name: '',
    description: '',
    tags: [] as string[]
  }
})

const newSendTag = ref('')

const defaultGroupName = computed(() => {
  const now = new Date()
  const year = now.getFullYear()
  const month = String(now.getMonth() + 1).padStart(2, '0')
  const day = String(now.getDate()).padStart(2, '0')
  const hour = String(now.getHours()).padStart(2, '0')
  const minute = String(now.getMinutes()).padStart(2, '0')
  const second = String(now.getSeconds()).padStart(2, '0')
  return `ConfigForge_${year}${month}${day}_${hour}${minute}${second}`
})

const sendPreview = computed(() => {
  const count = outputBlocks.value.length
  if (sendModal.value.mode === 'merge') {
    return {
      type: 'merge',
      commandCount: count,
      message: `共 ${count} 条命令将被添加`,
      examples: []
    }
  } else {
    const examples: string[] = []
    const prefix = sendModal.value.form.name || 'ConfigForge_'
    for (let i = 0; i < Math.min(count, 3); i++) {
      examples.push(`${prefix}${String(i + 1).padStart(2, '0')}`)
    }
    return {
      type: 'split',
      groupCount: count,
      message: `将创建 ${count} 个命令组`,
      examples
    }
  }
})

function openSendModal() {
  sendModal.value = {
    show: true,
    mode: 'merge',
    saving: false,
    form: {
      name: defaultGroupName.value,
      description: '从 ConfigForge 生成的配置',
      tags: ['ConfigForge']
    }
  }
  newSendTag.value = ''
}

function closeSendModal() {
  sendModal.value.show = false
}

function addSendTag() {
  const tag = newSendTag.value.trim()
  if (tag && !sendModal.value.form.tags.includes(tag)) {
    sendModal.value.form.tags.push(tag)
  }
  newSendTag.value = ''
}

function removeSendTag(index: number) {
  sendModal.value.form.tags.splice(index, 1)
}

async function executeSend() {
  if (sendModal.value.saving) return
  
  const { name, description, tags } = sendModal.value.form
  if (!name.trim()) {
    triggerToast('请输入命令组名称')
    return
  }
  
  if (outputBlocks.value.length === 0) {
    triggerToast('没有可发送的配置')
    return
  }
  
  sendModal.value.saving = true
  
  try {
    if (sendModal.value.mode === 'merge') {
      const allLines: string[] = []
      outputBlocks.value.forEach(block => {
        if (block) {
          const lines = block.split('\n').map(l => l.trim()).filter(l => l !== '')
          allLines.push(...lines)
        }
      })

      const groupData: Partial<CommandGroup> = {
        name: name.trim(),
        description: description.trim(),
        tags: tags,
        commands: allLines
      }
      
      await CreateCommandGroup(groupData as CommandGroup)
      showSendResult(true, `命令组「${name.trim()}」创建成功`, 1, [])
      
    } else {
      const createdIds: string[] = []
      const prefix = name.trim()
      
      for (let i = 0; i < outputBlocks.value.length; i++) {
        const block = outputBlocks.value[i]
        if (!block) continue
        
        const blockLines = block.split('\n').map(l => l.trim()).filter(l => l !== '')
        if (blockLines.length === 0) continue

        const seq = String(i + 1).padStart(2, '0')
        
        const groupData: Partial<CommandGroup> = {
          name: `${prefix}${seq}`,
          description: description.trim(),
          tags: tags,
          commands: blockLines
        }
        
        const result = await CreateCommandGroup(groupData as CommandGroup)
        createdIds.push(result?.id || '')
      }
      
      showSendResult(true, `成功创建 ${createdIds.length} 个命令组`, createdIds.length, createdIds)
    }
    
    closeSendModal()
    
  } catch (err: any) {
    console.error('创建命令组失败:', err)
    triggerToast('创建失败: ' + (err.message || err))
  } finally {
    sendModal.value.saving = false
  }
}

// ==================== 发送到任务执行 ====================

const taskModal = ref({
  show: false,
  saving: false,
  name: '',
  description: '',
  tags: [] as string[],
  newTag: ''
})

function openTaskModal() {
  const now = new Date()
  const y = now.getFullYear()
  const m = String(now.getMonth() + 1).padStart(2, '0')
  const d = String(now.getDate()).padStart(2, '0')
  const h = String(now.getHours()).padStart(2, '0')
  const mi = String(now.getMinutes()).padStart(2, '0')
  const s = String(now.getSeconds()).padStart(2, '0')
  taskModal.value = {
    show: true,
    saving: false,
    name: `ConfigForge_${y}${m}${d}_${h}${mi}${s}`,
    description: '从 ConfigForge (BindingDeviceIP) 生成的任务',
    tags: ['ConfigForge', 'BindingDeviceIP'],
    newTag: ''
  }
}

function closeTaskModal() {
  taskModal.value.show = false
}

function addTaskTag() {
  const tag = taskModal.value.newTag.trim()
  if (tag && !taskModal.value.tags.includes(tag)) {
    taskModal.value.tags.push(tag)
  }
  taskModal.value.newTag = ''
}

function removeTaskTag(index: number) {
  taskModal.value.tags.splice(index, 1)
}

async function executeTaskSend() {
  if (taskModal.value.saving) return
  if (!taskModal.value.name.trim()) {
    triggerToast('请输入任务名称')
    return
  }
  if (bindingPreview.value.length === 0) {
    triggerToast('没有可发送的绑定配置')
    return
  }

  taskModal.value.saving = true
  try {
    const items = bindingPreview.value.map(b => ({
      commandGroupId: '',
      commands: b.commands.split('\n').map(l => l.trim()).filter(l => l !== ''),
      deviceIPs: [b.ip]
    }))

    const taskGroup = {
      id: '',
      name: taskModal.value.name.trim(),
      description: taskModal.value.description.trim(),
      mode: 'binding' as const,
      items,
      tags: taskModal.value.tags,
      status: 'pending' as const,
      createdAt: '',
      updatedAt: ''
    }

    await CreateTaskGroup(taskGroup)
    closeTaskModal()
    showSendResult(true, `任务「${taskModal.value.name.trim()}」已发送到任务执行`, bindingPreview.value.length, [])
  } catch (err: any) {
    console.error('发送到任务执行失败:', err)
    triggerToast('发送失败: ' + (err.message || err))
  } finally {
    taskModal.value.saving = false
  }
}

function goToTaskExecution() {
  router.push('/task-execution')
}

// ==================== 结果提示 ====================

const sendResult = ref({
  show: false,
  success: true,
  message: '',
  createdCount: 0,
  groupIds: [] as string[]
})

function showSendResult(success: boolean, message: string, count: number, ids: string[]) {
  sendResult.value = {
    show: true,
    success,
    message,
    createdCount: count,
    groupIds: ids
  }
  
  setTimeout(() => {
    sendResult.value.show = false
  }, 5000)
}

function goToCommands() {
  router.push('/commands')
}

// ==================== 下载功能 ====================

const downloadAll = async () => {
  if (outputBlocks.value.length === 0) return
  const text = outputBlocks.value.join('\n\n')
  
  try {
    if ('showSaveFilePicker' in window) {
      const handle = await (window as any).showSaveFilePicker({
        suggestedName: 'ConfigForge_All.txt',
        types: [{
          description: 'Text Files',
          accept: { 'text/plain': ['.txt'] },
        }],
      });
      const writable = await handle.createWritable();
      await writable.write(text);
      await writable.close();
      triggerToast('配置文件已成功保存到本地！');
    } else {
      const blob = new Blob([text], { type: 'text/plain;charset=utf-8' })
      const url = URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url
      link.download = 'ConfigForge_All.txt'
      document.body.appendChild(link)
      link.click()
      document.body.removeChild(link)
      URL.revokeObjectURL(url)
      triggerToast('配置文件已开始下载！');
    }
  } catch (err: any) {
    if (err.name !== 'AbortError') {
      console.error('Failed to save file:', err);
      triggerToast('保存文件失败，请检查浏览器授权。');
    }
  }
}

const downloadSplit = async () => {
  if (outputBlocks.value.length === 0) return
  
  try {
    if ('showDirectoryPicker' in window) {
      const dirHandle = await (window as any).showDirectoryPicker({
        mode: 'readwrite'
      });
      
      for (let i = 0; i < outputBlocks.value.length; i++) {
        const block = outputBlocks.value[i];
        const fileHandle = await dirHandle.getFileHandle(`ConfigForge_Block_${i + 1}.txt`, { create: true });
        const writable = await fileHandle.createWritable();
        await writable.write(block);
        await writable.close();
      }
      triggerToast(`成功在规定目录下生成并保存 ${outputBlocks.value.length} 个配置文件！`);
    } else {
      outputBlocks.value.forEach((block, idx) => {
        setTimeout(() => {
          const blob = new Blob([block], { type: 'text/plain;charset=utf-8' })
          const url = URL.createObjectURL(blob)
          const link = document.createElement('a')
          link.href = url
          link.download = `ConfigForge_Block_${idx + 1}.txt`
          document.body.appendChild(link)
          link.click()
          document.body.removeChild(link)
          URL.revokeObjectURL(url)
        }, idx * 200)
      })
      triggerToast('分块配置文件已开始下载！');
    }
  } catch (err: any) {
    if (err.name !== 'AbortError') {
      console.error('Failed to save files:', err);
      triggerToast('批量保存文件失败，请检查浏览器授权。');
    }
  }
}

// ==================== 复制功能 ====================

const copyAll = async () => {
  if (outputBlocks.value.length === 0) return
  const textToCopy = outputBlocks.value.join('\n')
  try {
    await navigator.clipboard.writeText(textToCopy)
    isCopied.value = true
    setTimeout(() => {
      isCopied.value = false
    }, 2000)
  } catch (err) {
    console.error('Failed to copy text: ', err)
  }
}
</script>

<template>
  <div class="h-full w-full flex flex-col relative bg-transparent">
    
    <!-- 全局 Toast 提示 -->
    <div 
      class="toast-container toast-container-top-center"
      :class="showToast ? 'visible' : 'invisible'"
    >
      <div 
        class="toast toast-success"
        :class="showToast ? 'toast-visible' : ''"
      >
        <svg xmlns="http://www.w3.org/2000/svg" class="toast-icon" viewBox="0 0 20 20" fill="currentColor">
          <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd" />
        </svg>
        <span class="toast-message font-medium">{{ toastMessage }}</span>
      </div>
    </div>

    <!-- Main Workspace -->
    <main 
      ref="workspaceRef"
      class="flex-1 flex overflow-y-auto md:overflow-hidden flex-col md:flex-row gap-3 z-10"
    >
      
      <!-- Column 1: 配置模版 -->
      <div 
        class="min-h-[300px] md:h-full md:min-h-[700px] glass-panel border border-border flex flex-col overflow-hidden"
        :style="{ 
          width: windowWidth < 768 ? '100%' : 'auto',
          flex: windowWidth < 768 ? 'none' : `${leftColWidth} 1 0%`
        }"
      >
        <div class="card-header">
          <h2 class="card-header-title">
            <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 text-accent" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
            </svg>
            配置模版
          </h2>
        </div>
        <div class="flex-1 p-4 flex flex-col">
          <textarea 
            v-model="templateText"
            class="flex-1 w-full resize-none outline-none premium-textarea p-4 font-mono text-sm leading-relaxed" 
            spellcheck="false" 
            placeholder="在此处输入配置模板，例如：&#10;interface GigabitEthernet0/0/[A]&#10; description [B]&#10;..."></textarea>
        </div>
      </div>

      <!-- Left Resizer -->
      <div 
        class="hidden md:flex w-1.5 cursor-col-resize hover:bg-accent/50 rounded-full active:bg-accent/80 transition-all z-20 flex-shrink-0" 
        @mousedown="startResize('left')"
      ></div>

      <!-- Column 2: Variables Mapping -->
      <div 
        class="min-h-[400px] md:h-full md:min-h-[700px] glass-panel border border-border flex flex-col overflow-hidden"
        :style="{ 
          width: windowWidth < 768 ? '100%' : 'auto',
          flex: windowWidth < 768 ? 'none' : `${midColWidth} 1 0%`
        }"
      >
        <div class="card-header">
          <div>
            <h2 class="card-header-title">
              <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 text-info" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" />
              </svg>
              变量映射
              <button @click="showSyntaxHelp = true" class="ml-2.5 text-text-muted hover:text-accent transition-colors cursor-help focus:outline-none" title="语法说明">
                <svg xmlns="http://www.w3.org/2000/svg" class="h-[1.125rem] w-[1.125rem]" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8.228 9c.549-1.165 2.03-2 3.772-2 2.21 0 4 1.343 4 3 0 1.4-1.278 2.575-3.006 2.907-.542.104-.994.54-.994 1.093m0 3h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
              </button>
            </h2>
            <p class="text-xs text-text-muted mt-0.5 ml-6">使用"英文逗号"分隔数值（后端处理语法糖展开）</p>
          </div>
          <button 
            @click="addVariable"
            class="btn btn-sm btn-secondary"
          >
            <svg xmlns="http://www.w3.org/2000/svg" class="h-3.5 w-3.5 text-accent" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
            </svg>
            添加变量
          </button>
        </div>
        <div class="flex-1 overflow-y-auto p-4 space-y-4">
          <div v-for="(v, index) in variables" :key="index" class="bg-bg-tertiary/40 border border-border backdrop-blur-sm flex flex-col rounded-xl shadow-sm hover:shadow-md transition-shadow group" :class="isBindingMode && index === 0 ? 'border-warning/40 ring-1 ring-warning/20' : ''">
            <div class="flex items-center justify-between px-3 py-2 border-b border-border bg-bg-tertiary/40 rounded-t-xl">
              <div class="relative flex items-center gap-2">
                <input 
                  v-model="v.name"
                  class="input input-sm input-mono w-20 text-center tracking-wider"
                  :class="isBindingMode && index === 0 ? 'text-warning font-bold' : ''"
                  :readonly="isBindingMode && index === 0"
                />
                <span v-if="isBindingMode && index === 0" class="text-xs px-1.5 py-0.5 rounded bg-warning/10 border border-warning/30 text-warning font-medium">IP绑定</span>
              </div>
              <button 
                @click="removeVariable(index)"
                class="text-text-muted hover:text-error hover:bg-error-bg p-1.5 rounded-md transition-all"
                title="删除变量"
                :disabled="isBindingMode && index === 0"
                :class="isBindingMode && index === 0 ? 'opacity-30 cursor-not-allowed' : ''"
              >
                <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                </svg>
              </button>
            </div>
            <textarea 
              v-model="v.valueString"
              @blur="expandSyntaxSugar(v)"
              class="input textarea h-16 input-mono rounded-b-xl rounded-t-none border-0 border-t border-border bg-bg-tertiary/30"
              :placeholder="isBindingMode && index === 0 ? '192.168.1.1, 192.168.1.2, ...' : index === 0 ? '1, 2, 3...' : index === 1 ? '1-3' : index === 2 ? 'vlan10-13' : index === 3 ? '192.168.1.1-3' : '...'"
              spellcheck="false"
            ></textarea>
          </div>
        </div>
      </div>

      <!-- Right Resizer -->
      <div 
        class="hidden md:flex w-1.5 cursor-col-resize hover:bg-info/50 rounded-full active:bg-info/80 transition-all z-20 flex-shrink-0" 
        @mousedown="startResize('right')"
      ></div>

      <!-- Column 3: Output Preview -->
      <div 
        class="min-h-[400px] md:h-full md:min-h-[700px] glass-panel border border-border relative flex flex-col overflow-hidden"
        :style="{ 
          width: windowWidth < 768 ? '100%' : 'auto',
          flex: windowWidth < 768 ? 'none' : `${rightColWidth} 1 0%`
        }"
      >
        <div class="card-header">
          <h2 class="card-header-title shrink-0">
            <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 text-success" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 20l4-16m4 4l4 4-4 4M6 16l-4-4 4-4" />
            </svg>
            生成预览
            <span class="ml-2 px-2 py-0.5 bg-accent-bg text-accent text-xs rounded-full font-mono">{{ outputBlocks.length }} blocks</span>
            <span v-if="isBuilding" class="ml-2 text-xs text-text-muted animate-pulse">构建中...</span>
          </h2>

          <!-- 功能按钮区 -->
          <div class="flex space-x-2 items-center" v-if="outputBlocks.length > 0">
            <!-- BindingDeviceIP 模式 -->
            <button 
              v-if="isBindingMode"
              @click="openTaskModal"
              class="btn btn-sm btn-secondary group relative"
              title="发送到任务执行"
            >
              <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 text-warning" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <polygon points="5 3 19 12 5 21 5 3" fill="none" stroke="currentColor" stroke-width="2"/>
              </svg>
            </button>
            <!-- 发送到命令管理 -->
            <button 
              @click="openSendModal"
              class="btn btn-sm btn-secondary group relative"
              title="发送到命令管理"
            >
              <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 text-success" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" />
              </svg>
            </button>
            
            <button 
              @click="downloadSplit"
              class="btn btn-sm btn-secondary group relative"
              title="分块下载"
            >
              <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 text-accent" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 7v8a2 2 0 002 2h6M8 7V5a2 2 0 012-2h4.586a1 1 0 01.707.293l4.414 4.414a1 1 0 01.293.707V15a2 2 0 01-2 2h-2M8 7H6a2 2 0 00-2 2v10a2 2 0 002 2h8a2 2 0 002-2v-2" />
              </svg>
            </button>
            <button 
              @click="downloadAll"
              class="btn btn-sm btn-secondary group relative"
              title="合并下载"
            >
              <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 text-info" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
              </svg>
            </button>
            <button 
              @click="copyAll"
              class="btn btn-sm group relative"
              :class="isCopied ? 'btn-success' : 'btn-primary'"
              title="复制全部"
            >
              <svg v-if="!isCopied" xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
              </svg>
              <svg v-else xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
              </svg>
            </button>
          </div>
        </div>
        
        <div class="flex-1 overflow-y-auto p-5 scrollbar-custom">
          <template v-if="outputBlocks.length > 0">
            <div 
              v-for="(block, idx) in outputBlocks" 
              :key="idx" 
              class="p-4 mb-4 bg-bg-tertiary/60 backdrop-blur-sm border border-border rounded-xl font-mono text-sm whitespace-pre-wrap text-text-primary shadow-sm transition-all hover:shadow-md"
            >{{ block }}</div>
          </template>
          <div v-else class="h-full flex flex-col items-center justify-center text-text-muted space-y-3">
            <svg xmlns="http://www.w3.org/2000/svg" class="h-12 w-12 text-text-muted/50" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M19.428 15.428a2 2 0 00-1.022-.547l-2.387-.477a6 6 0 00-3.86.517l-.318.158a6 6 0 01-3.86.517L6.05 15.21a2 2 0 00-1.806.547M8 4h8l-1 1v5.172a2 2 0 00.586 1.414l5 5c1.26 1.26.367 3.414-1.415 3.414H4.828c-1.782 0-2.674-2.154-1.414-3.414l5-5A2 2 0 009 10.172V5L8 4z" />
            </svg>
            <span class="text-sm font-medium">配置等待生成...</span>
          </div>
        </div>
      </div>
    </main>

    <!-- 右下角悬浮帮助按钮 -->
    <button 
      @click="showUsageHelp = true" 
      class="fixed bottom-6 right-6 z-40 w-12 h-12 flex items-center justify-center rounded-full bg-gradient-to-r from-accent to-info text-white shadow-lg shadow-accent/30 hover:shadow-accent/50 hover:scale-110 transition-all duration-300 cursor-help focus:outline-none focus:ring-2 focus:ring-accent focus:ring-offset-2"
      title="使用简介"
    >
      <svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8.228 9c.549-1.165 2.03-2 3.772-2 2.21 0 4 1.343 4 3 0 1.4-1.278 2.575-3.006 2.907-.542.104-.994.54-.994 1.093m0 3h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
      </svg>
    </button>

    <!-- 发送到命令管理弹窗 -->
    <Transition name="modal">
      <div v-if="sendModal.show" class="modal-container modal-active">
        <div class="modal-overlay" @click="closeSendModal"></div>
        
        <div class="modal modal-lg modal-glass">
          <div class="modal-header">
            <h3 class="modal-header-title">
              <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 text-success" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" />
              </svg>
              发送到命令管理
            </h3>
            <button @click="closeSendModal" class="modal-close">
              <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>
          
          <div class="modal-body space-y-5">
            <!-- 模式选择 -->
            <div class="space-y-3">
              <label class="text-sm font-medium text-text-primary">创建模式</label>
              <div class="grid grid-cols-2 gap-3">
                <div 
                  class="mode-card"
                  :class="{ active: sendModal.mode === 'merge' }"
                  @click="sendModal.mode = 'merge'"
                >
                  <div class="mode-icon">📦</div>
                  <div class="mode-title">合并为一个命令组</div>
                  <div class="mode-desc">所有配置块合并</div>
                </div>
                <div 
                  class="mode-card"
                  :class="{ active: sendModal.mode === 'split' }"
                  @click="sendModal.mode = 'split'"
                >
                  <div class="mode-icon">📂</div>
                  <div class="mode-title">分开创建多个命令组</div>
                  <div class="mode-desc">每个配置块独立</div>
                </div>
              </div>
            </div>
            
            <!-- 基本信息 -->
            <div class="space-y-4">
              <div class="space-y-1.5">
                <label class="text-sm font-medium text-text-primary">
                  {{ sendModal.mode === 'merge' ? '命令组名称' : '名称前缀' }}
                  <span class="text-error">*</span>
                </label>
                <input 
                  v-model="sendModal.form.name" 
                  type="text" 
                  class="w-full px-4 py-2.5 rounded-lg bg-bg-secondary border border-border text-sm text-text-primary placeholder:text-text-muted focus:outline-none focus:border-accent/50"
                />
              </div>
              
              <div class="space-y-1.5">
                <label class="text-sm font-medium text-text-primary">描述</label>
                <input 
                  v-model="sendModal.form.description" 
                  type="text" 
                  class="w-full px-4 py-2.5 rounded-lg bg-bg-secondary border border-border text-sm text-text-primary placeholder:text-text-muted focus:outline-none focus:border-accent/50"
                />
              </div>
              
              <div class="space-y-1.5">
                <label class="text-sm font-medium text-text-primary">标签</label>
                <div class="flex flex-wrap gap-2 mb-2">
                  <span 
                    v-for="(tag, idx) in sendModal.form.tags" 
                    :key="idx" 
                    class="inline-flex items-center gap-1 px-2.5 py-1 text-xs rounded-full bg-accent/10 text-accent border border-accent/20"
                  >
                    {{ tag }}
                    <button @click="removeSendTag(idx)" class="hover:text-error transition-colors">
                      <svg xmlns="http://www.w3.org/2000/svg" class="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3">
                        <line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/>
                      </svg>
                    </button>
                  </span>
                </div>
                <div class="flex gap-2">
                  <input 
                    v-model="newSendTag" 
                    type="text" 
                    class="flex-1 px-3 py-2 rounded-lg bg-bg-secondary border border-border text-sm text-text-primary placeholder:text-text-muted focus:outline-none focus:border-accent/50"
                    placeholder="添加标签" 
                    @keyup.enter="addSendTag"
                  />
                  <button 
                    @click="addSendTag" 
                    class="px-3 py-2 rounded-lg bg-accent/10 border border-accent/30 text-accent text-sm font-medium hover:bg-accent/20 transition-colors"
                  >
                    添加
                  </button>
                </div>
              </div>
              
              <div class="preview-box">
                <div class="preview-icon">📊</div>
                <div class="preview-content">
                  <span class="preview-text">{{ sendPreview.message }}</span>
                </div>
              </div>
            </div>
          </div>
          
          <div class="modal-footer">
            <button @click="closeSendModal" class="btn btn-secondary">取消</button>
            <button 
              @click="executeSend" 
              :disabled="sendModal.saving" 
              class="btn btn-primary"
            >
              {{ sendModal.saving ? '创建中...' : '确认创建' }}
            </button>
          </div>
        </div>
      </div>
    </Transition>

    <!-- 发送到任务执行弹窗 -->
    <Transition name="modal">
      <div v-if="taskModal.show" class="modal-container modal-active">
        <div class="modal-overlay" @click="closeTaskModal"></div>
        <div class="modal modal-lg modal-glass">
          <div class="modal-header">
            <h3 class="modal-header-title">
              <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 text-warning" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <polygon points="5 3 19 12 5 21 5 3" fill="none" stroke="currentColor" stroke-width="2"/>
              </svg>
              发送到任务执行
            </h3>
            <button @click="closeTaskModal" class="modal-close">
              <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>
          <div class="modal-body space-y-5">
            <!-- 无效 IP 警告 -->
            <div v-if="hasInvalidIP" class="flex items-start gap-3 p-4 rounded-xl bg-error/10 border border-error/30">
              <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 text-error mt-0.5 shrink-0" viewBox="0 0 20 20" fill="currentColor">
                <path fill-rule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7 4a1 1 0 11-2 0 1 1 0 012 0zm-1-9a1 1 0 00-1 1v4a1 1 0 102 0V6a1 1 0 00-1-1z" clip-rule="evenodd" />
              </svg>
              <div class="flex-1">
                <p class="text-sm font-medium text-error">检测到无效 IP 地址</p>
                <p class="text-xs text-text-muted mt-1">
                  以下 IP 格式无效，将被过滤：{{ invalidIPs.slice(0, 3).join(', ') }}{{ invalidIPs.length > 3 ? ` 等 ${invalidIPs.length} 个` : '' }}
                </p>
              </div>
            </div>
            <!-- 名称 -->
            <div class="space-y-1.5">
              <label class="text-sm font-medium text-text-primary">任务名称 <span class="text-error">*</span></label>
              <input v-model="taskModal.name" type="text" class="w-full px-4 py-2.5 rounded-lg bg-bg-secondary border border-border text-sm text-text-primary placeholder:text-text-muted focus:outline-none focus:border-accent/50" />
            </div>
            <!-- 描述 -->
            <div class="space-y-1.5">
              <label class="text-sm font-medium text-text-primary">描述</label>
              <input v-model="taskModal.description" type="text" class="w-full px-4 py-2.5 rounded-lg bg-bg-secondary border border-border text-sm text-text-primary placeholder:text-text-muted focus:outline-none focus:border-accent/50" />
            </div>
            <!-- 标签 -->
            <div class="space-y-1.5">
              <label class="text-sm font-medium text-text-primary">标签</label>
              <div class="flex flex-wrap gap-2 mb-2">
                <span v-for="(tag, idx) in taskModal.tags" :key="idx" class="inline-flex items-center gap-1 px-2.5 py-1 text-xs rounded-full bg-accent/10 text-accent border border-accent/20">
                  {{ tag }}
                  <button @click="removeTaskTag(idx)" class="hover:text-error transition-colors">
                    <svg xmlns="http://www.w3.org/2000/svg" class="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
                  </button>
                </span>
              </div>
              <div class="flex gap-2">
                <input v-model="taskModal.newTag" type="text" class="flex-1 px-3 py-2 rounded-lg bg-bg-secondary border border-border text-sm text-text-primary placeholder:text-text-muted focus:outline-none focus:border-accent/50" placeholder="添加标签" @keyup.enter="addTaskTag" />
                <button @click="addTaskTag" class="px-3 py-2 rounded-lg bg-accent/10 border border-accent/30 text-accent text-sm font-medium hover:bg-accent/20 transition-colors">添加</button>
              </div>
            </div>
            <!-- 绑定预览 -->
            <div class="preview-box">
              <div class="preview-icon">🔗</div>
              <div class="preview-content">
                <span class="preview-text">共 <strong class="text-warning">{{ bindingPreview.length }}</strong> 台设备的 IP 绑定任务</span>
                <div class="mt-2 space-y-1 max-h-32 overflow-auto scrollbar-custom">
                  <div v-for="(b, i) in bindingPreview.slice(0, 5)" :key="i" class="flex items-center gap-2 text-xs">
                    <span class="font-mono text-warning">{{ b.ip }}</span>
                    <span class="text-text-muted">→</span>
                    <span class="text-text-secondary truncate">{{ b.commands.split('\n').length }} 行命令</span>
                  </div>
                  <div v-if="bindingPreview.length > 5" class="text-xs text-text-muted">+{{ bindingPreview.length - 5 }} 台设备...</div>
                </div>
              </div>
            </div>
          </div>
          <div class="modal-footer">
            <button @click="closeTaskModal" class="btn btn-secondary">取消</button>
            <button @click="executeTaskSend" :disabled="taskModal.saving" class="btn btn-primary">
              {{ taskModal.saving ? '发送中...' : '确认发送' }}
            </button>
          </div>
        </div>
      </div>
    </Transition>

    <!-- 创建成功提示 -->
    <Transition name="toast">
      <div v-if="sendResult.show" class="success-toast">
        <div class="toast-icon">
          <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" viewBox="0 0 20 20" fill="currentColor">
            <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd" />
          </svg>
        </div>
        <div class="toast-content">
          <div class="toast-title">{{ sendResult.message }}</div>
          <button v-if="sendResult.message.includes('任务执行')" @click="goToTaskExecution" class="toast-link">
            查看任务执行 →
          </button>
          <button v-else @click="goToCommands" class="toast-link">
            查看命令管理 →
          </button>
        </div>
      </div>
    </Transition>

    <!-- 使用简介弹窗 -->
    <Transition name="modal">
      <div v-if="showUsageHelp" class="modal-container modal-active">
        <div class="modal-overlay" @click="showUsageHelp = false"></div>
        
        <div class="modal modal-lg modal-glass">
          <div class="modal-header">
            <h3 class="modal-header-title">
              <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 text-accent" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
              ConfigForge 使用简介
            </h3>
            <button @click="showUsageHelp = false" class="modal-close">
              <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>
        
          <div class="modal-body text-sm text-text-secondary space-y-5">
            <p class="leading-relaxed text-base">
              <strong class="text-text-primary">ConfigForge</strong> 是一款强大的自动化配置生成工具，所有核心计算逻辑均由后端处理，前端仅负责表单提交和结果渲染。
            </p>

            <div class="bg-accent-bg/50 rounded-xl p-5 border border-accent/20 shadow-inner">
              <h4 class="font-semibold text-accent mb-4 flex items-center">
                <span class="text-base mr-1.5">🚀</span> 核心工作流：
              </h4>
              <ul class="space-y-4">
                <li class="flex flex-col">
                  <span class="text-sm font-bold text-text-primary mb-1.5">1. Template Input (模板输入)</span>
                  <span class="text-text-muted leading-relaxed">在左侧输入配置框架，使用 <code class="px-1.5 py-0.5 bg-bg-tertiary rounded-md border border-border text-accent font-mono">[A]</code>, <code class="px-1.5 py-0.5 bg-bg-tertiary rounded-md border border-border text-accent font-mono">[B]</code> 等变量占位。</span>
                </li>
                <li class="flex flex-col">
                  <span class="text-sm font-bold text-text-primary mb-1.5">2. Variables Mapping (变量映射)</span>
                  <span class="text-text-muted leading-relaxed">在中列输入变量值。后端自动处理<strong class="text-accent">语法糖展开</strong>及<strong class="text-accent">等差数列补全</strong>。</span>
                </li>
                <li class="flex flex-col">
                  <span class="text-sm font-bold text-text-primary mb-1.5">3. Output Preview (输出预览)</span>
                  <span class="text-text-muted leading-relaxed">右侧实时显示后端生成的配置块，前端无需任何计算。</span>
                </li>
              </ul>
            </div>
            
            <div class="flex items-start bg-info-bg rounded-xl p-4 border border-info/30 shadow-sm">
              <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 text-info mt-0.5 shrink-0 mr-2.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
              </svg>
              <p class="text-sm text-info leading-relaxed">
                <strong>后端计算：</strong>变量替换、语法糖展开、等差数列推断、IP验证等核心逻辑均在后端执行，前端仅负责表单提交和结果渲染。
              </p>
            </div>
          </div>
          
          <div class="modal-footer">
            <button @click="showUsageHelp = false" class="btn btn-primary">
              我知道了
            </button>
          </div>
        </div>
      </div>
    </Transition>

    <!-- 语法说明弹窗 -->
    <Transition name="modal">
      <div v-if="showSyntaxHelp" class="modal-container modal-active">
        <div class="modal-overlay" @click="showSyntaxHelp = false"></div>
      
        <div class="modal modal-lg modal-glass">
          <div class="modal-header">
            <h3 class="modal-header-title">
              <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 text-accent" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
              语法糖智能展开说明
            </h3>
            <button @click="showSyntaxHelp = false" class="modal-close">
              <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>
        
          <div class="modal-body text-sm text-text-secondary space-y-5">
            <p class="leading-relaxed">
              语法糖展开和等差数列推断由<strong class="text-accent">后端服务</strong>处理。输入格式后失去焦点时，后端会自动展开。
            </p>
            
            <div class="bg-accent-bg/50 rounded-xl p-5 border border-accent/20 shadow-inner">
              <h4 class="font-semibold text-accent mb-4 flex items-center">
                <span class="text-base mr-1.5">📌</span> 常规连字符生成：
              </h4>
              <ul class="space-y-4">
                <li class="flex flex-col">
                  <span class="text-xs font-medium text-text-muted mb-1.5">▶ 基础数字序列</span>
                  <div class="flex items-center space-x-2.5">
                    <code class="bg-bg-secondary px-2.5 py-1 rounded-lg border border-border text-text-primary font-mono">1-5</code>
                    <span class="text-text-muted text-xs">➜</span>
                    <code class="bg-bg-tertiary/80 px-2.5 py-1 rounded-lg border border-border text-text-secondary font-mono">1, 2, 3, 4, 5</code>
                  </div>
                </li>
                <li class="flex flex-col">
                  <span class="text-xs font-medium text-text-muted mb-1.5">▶ 携带前后缀</span>
                  <div class="flex items-center space-x-2.5">
                    <code class="bg-bg-secondary px-2.5 py-1 rounded-lg border border-border text-text-primary font-mono">vlan10-12</code>
                    <span class="text-text-muted text-xs">➜</span>
                    <code class="bg-bg-tertiary/80 px-2.5 py-1 rounded-lg border border-border text-text-secondary font-mono">vlan10, vlan11, vlan12</code>
                  </div>
                </li>
              </ul>
            </div>

            <div class="bg-success-bg/50 rounded-xl p-5 border border-success/20 shadow-inner">
              <h4 class="font-semibold text-success mb-4 flex items-center">
                <span class="text-base mr-1.5">🪄</span> 等差数列智能补全：
              </h4>
              <p class="text-sm text-success leading-relaxed mb-3">
                当其他变量已有确定长度时，输入 <strong>2个及以上</strong> 构成规律的数列，后端将自动补全对齐。
              </p>
            </div>
          </div>
        
          <div class="modal-footer">
            <button @click="showSyntaxHelp = false" class="btn btn-primary">
              我知道了
            </button>
          </div>
        </div>
      </div>
    </Transition>
  </div>
</template>

<style scoped lang="postcss">
@reference "../../styles/index.css";

/* 模式选择卡片 */
.mode-card {
  @apply relative p-4 rounded-xl border border-border bg-bg-secondary/50 
         cursor-pointer transition-all duration-200;
}
.mode-card:hover {
  @apply border-accent/30 bg-bg-secondary/80;
}
.mode-card.active {
  @apply border-accent bg-accent/10 ring-1 ring-accent/20;
}
.mode-icon {
  @apply text-2xl mb-2;
}
.mode-title {
  @apply text-sm font-semibold text-text-primary;
}
.mode-desc {
  @apply text-xs text-text-muted mt-1;
}

/* 预览信息框 */
.preview-box {
  @apply flex items-start gap-3 p-4 rounded-xl bg-info/10 border border-info/20;
}
.preview-icon {
  @apply text-lg;
}
.preview-content {
  @apply flex-1;
}
.preview-text {
  @apply text-sm text-text-secondary;
}

/* 成功提示 */
.success-toast {
  @apply fixed bottom-6 left-1/2 -translate-x-1/2 z-[100] 
         flex items-center gap-3 px-5 py-4 rounded-xl 
         bg-success/95 text-white shadow-lg shadow-success/20;
}
.success-toast .toast-icon {
  @apply flex items-center justify-center w-8 h-8 rounded-full bg-white/20;
}
.success-toast .toast-content {
  @apply flex flex-col;
}
.success-toast .toast-title {
  @apply text-sm font-medium;
}
.success-toast .toast-link {
  @apply text-xs text-white/80 hover:text-white underline underline-offset-2 mt-1 text-left;
}

/* Toast 动画 */
.toast-enter-active,
.toast-leave-active {
  transition: all 0.3s ease;
}

.toast-enter-from {
  opacity: 0;
  transform: translate(-50%, 20px);
}

.toast-leave-to {
  opacity: 0;
  transform: translate(-50%, -10px);
}
</style>
