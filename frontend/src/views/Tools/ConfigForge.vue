<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import { CreateCommandGroup } from '../../bindings/github.com/NetWeaverGo/core/internal/ui/appservice.js'

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
  
  // 获取容器（main 元素）及其地理位置
  const rect = workspaceRef.value.getBoundingClientRect()
  const containerWidth = rect.width
  const mouseX = e.clientX - rect.left // 鼠标相对于容器左侧的位置
  
  // 计算百分比
  const mousePct = (mouseX / containerWidth) * 100

  // 设定最小容忍宽度百分比
  const minW = 10 // 稍微缩小最小限制，增加灵活性
  
  if (resizeType.value === 'left') {
    // 调整 leftBar: 改变 leftCol 和 midCol
    // 注意：不能让 leftCol 太小，也不能让 midCol 压缩到限制以下
    if (mousePct > minW && mousePct < (leftColWidth.value + midColWidth.value - minW)) {
      const diff = mousePct - leftColWidth.value
      leftColWidth.value += diff
      midColWidth.value -= diff
    }
  } else if (resizeType.value === 'right') {
    // 调整 rightBar: 改变 midCol 和 rightCol
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

// 状态定义
const templateText = ref('')
const variables = ref([
  { id: 1, name: '[A]', valueString: '' },
  { id: 2, name: '[B]', valueString: '' },
  { id: 3, name: '[C]', valueString: '' },
  { id: 4, name: '[D]', valueString: '' },
])
const outputBlocks = ref<string[]>([])
const isCopied = ref(false)

// Toast 通知状态
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

// ========== 发送到命令管理功能状态 ==========

// 弹窗显示状态
const sendModal = ref({
  show: false,           // 弹窗是否显示
  mode: 'merge' as 'merge' | 'split',  // 创建模式: 'merge' | 'split'
  saving: false,         // 保存中状态
  form: {
    name: '',            // 命令组名称（合并模式）或名称前缀（分开模式）
    description: '',     // 描述
    tags: [] as string[] // 标签
  }
})

// 标签输入临时状态
const newSendTag = ref('')

// 创建结果状态（用于成功后显示）
const sendResult = ref({
  show: false,           // 是否显示结果提示
  success: true,         // 是否成功
  message: '',           // 提示消息
  createdCount: 0,       // 创建的命令组数量
  groupIds: [] as string[] // 创建的命令组ID列表
})

// 生成默认名称（带时间戳）
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

// 预览信息
const sendPreview = computed(() => {
  const count = outputBlocks.value.length
  if (sendModal.value.mode === 'merge') {
    return {
      type: 'merge',
      commandCount: count,
      message: `共 ${count} 条命令将被添加`
    }
  } else {
    // 生成示例名称
    const examples: string[] = []
    const prefix = sendModal.value.form.name || 'ConfigForge_'
    for (let i = 0; i < Math.min(count, 3); i++) {
      examples.push(`${prefix}${String(i + 1).padStart(2, '0')}`)
    }
    return {
      type: 'split',
      groupCount: count,
      message: `将创建 ${count} 个命令组`,
      examples: examples
    }
  }
})

/**
 * 打开发送到命令管理弹窗
 * 初始化表单数据
 */
function openSendModal() {
  // 重置表单状态
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

/**
 * 关闭发送弹窗
 */
function closeSendModal() {
  sendModal.value.show = false
}

/**
 * 添加标签
 */
function addSendTag() {
  const tag = newSendTag.value.trim()
  if (tag && !sendModal.value.form.tags.includes(tag)) {
    sendModal.value.form.tags.push(tag)
  }
  newSendTag.value = ''
}

/**
 * 移除标签
 */
function removeSendTag(index: number) {
  sendModal.value.form.tags.splice(index, 1)
}

/**
 * 执行创建命令组
 */
async function executeSend() {
  if (sendModal.value.saving) return
  
  // 表单验证
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
      // ===== 合并模式：创建单个命令组 =====
      // 将所有 block 展开并拆分为独立行，确保符合后端命令管理系统的行处理逻辑
      const allLines: string[] = []
      outputBlocks.value.forEach(block => {
        if (block) {
          const lines = block.split('\n').map(l => l.trim()).filter(l => l !== '')
          allLines.push(...lines)
        }
      })

      const groupData = {
        name: name.trim(),
        description: description.trim(),
        tags: tags,
        commands: allLines
      }
      
      const result = await CreateCommandGroup(groupData)
      
      // 显示成功提示
      showSendResult(true, `命令组「${name.trim()}」创建成功`, 1, [result.id || ''])
      
    } else {
      // ===== 分开模式：批量创建多个命令组 =====
      const createdIds: string[] = []
      const prefix = name.trim()
      
      for (let i = 0; i < outputBlocks.value.length; i++) {
        const block = outputBlocks.value[i]
        if (!block) continue
        
        // 将当前 block 拆分为独立行
        const blockLines = block.split('\n').map(l => l.trim()).filter(l => l !== '')
        if (blockLines.length === 0) continue

        // 生成序号：01, 02, ..., 10, 11, ...
        const seq = String(i + 1).padStart(2, '0')
        
        const groupData = {
          name: `${prefix}${seq}`,
          description: description.trim(),
          tags: tags,
          commands: blockLines
        }
        
        const result = await CreateCommandGroup(groupData)
        createdIds.push(result.id || '')
      }
      
      // 显示成功提示
      showSendResult(
        true, 
        `成功创建 ${createdIds.length} 个命令组`, 
        createdIds.length, 
        createdIds
      )
    }
    
    closeSendModal()
    
  } catch (err: any) {
    console.error('创建命令组失败:', err)
    triggerToast('创建失败: ' + (err.message || err))
  } finally {
    sendModal.value.saving = false
  }
}

/**
 * 显示创建结果
 */
function showSendResult(success: boolean, message: string, count: number, ids: string[]) {
  sendResult.value = {
    show: true,
    success,
    message,
    createdCount: count,
    groupIds: ids
  }
  
  // 5秒后自动隐藏
  setTimeout(() => {
    sendResult.value.show = false
  }, 5000)
}

/**
 * 跳转到命令管理页面
 */
function goToCommands() {
  router.push('/commands')
}

// 动态变量管理
const nextVarIndex = ref(4) // 记录生成的序号（从 4 开始即 E）
const nextVarId = ref(5)    // 用于 Vue v-for 的唯一标识 key

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
  variables.value.push({ id: nextVarId.value++, name: `[${newName}]`, valueString: '' })
}
const removeVariable = (idx: number) => {
  variables.value.splice(idx, 1)
}

// 语法糖智能展开 (例如: 1-10 -> 1, 2, ..., 10)
const expandSyntaxSugar = (v: { id?: number; name: string; valueString: string }) => {
  if (!v.valueString) return
  
  let hasExpanded = false
  const lines = v.valueString.split('\n')
  
  const newLines = lines.map(line => {
    let lineExpanded = false
    const parts = line.split(',')
    const expandedParts = parts.flatMap(part => {
      const trimmed = part.trim()
      if (!trimmed) return [] // 展开时自动清理空项
      
      const match = trimmed.match(/^(.*?)(\d+)([-~])(\d+)(.*?)$/)
      if (match) {
        const prefix = match[1] || ''
        const startStr = match[2] || '0'
        const endStr = match[4] || '0'
        const suffix = match[5] || ''
        
        const start = parseInt(startStr, 10)
        const end = parseInt(endStr, 10)
        
        // 防呆：范围太大会卡死，最大支持 1000 级展开
        if (Math.abs(start - end) > 1000) return [trimmed]
        
        const padLen = Math.max(startStr.length, endStr.length)
        const hasLeadingZero = startStr.startsWith('0') || endStr.startsWith('0')
        
        const step = start <= end ? 1 : -1
        const result = []
        
        for (let i = start; step === 1 ? i <= end : i >= end; i += step) {
          let s = i.toString()
          if (hasLeadingZero) s = s.padStart(padLen, '0')
          result.push(`${prefix}${s}${suffix}`)
        }
        lineExpanded = true
        hasExpanded = true
        return result
      }
      return [trimmed]
    })
    
    if (lineExpanded) {
      return expandedParts.join(', ')
    }
    return line
  })
  
  if (hasExpanded) {
    v.valueString = newLines.join('\n')
  }

  // 二次推断：基于等差数列进行补全
  // 1. 获取当前所有变量拆分后的最大长度，作为标杆
  const allParsed = variables.value.map(varItem => {
    return varItem.valueString
      .split(/,|\n/)
      .map(s => s.trim())
      .filter(s => s !== '')
  })
  // 限制最大推断长度为 1000，防止性能问题卡死浏览器
  const maxLen = Math.min(Math.max(...allParsed.map(arr => arr.length)), 1000)

  // 2. 当前输入框拆分出的值
  const currentVals = v.valueString
      .split(/,|\n/)
      .map(s => s.trim())
      .filter(s => s !== '')
  
  const rawLen = currentVals.length
  
  // 3. 如果当前变量长度不足标杆，并且有至少2个值可以用来推断步长
  if (rawLen > 0 && rawLen < maxLen && rawLen >= 2) {
    let isArithmetic = true
    let commonPrefix = ''
    let commonSuffix = ''
    let nums: number[] = []
    let padLen = 0
    let hasLeadingZero = false

    for (let i = 0; i < rawLen; i++) {
        const val = currentVals[i]
        if (!val) {
          isArithmetic = false
          break
        }
        const match = val.match(/^(.*?)(\d+)(.*?)$/)
        if (!match) {
          isArithmetic = false
          break
        }
        const p = match[1] || ''
        const nStr = match[2] || '0'
        const s = match[3] || ''

        if (i === 0) {
          commonPrefix = p
          commonSuffix = s
          padLen = nStr.length
          hasLeadingZero = nStr.startsWith('0')
        } else {
          // 前后缀必须完全一致才能被认为是同一数列
          if (p !== commonPrefix || s !== commonSuffix) {
            isArithmetic = false
            break
          }
        }
        nums.push(parseInt(nStr, 10))
    }

    // 验证数字是否构成等差数列
    if (isArithmetic && nums.length >= 2) {
      const step = (nums[1] as number) - (nums[0] as number)
      let isConstantStep = true
      for (let i = 2; i < nums.length; i++) {
        if ((nums[i] as number) - (nums[i - 1] as number) !== step) {
          isConstantStep = false
          break
        }
      }

      if (isConstantStep) {
        // 是等差数列！将此变量自动延长到 maxLen 并回写到文本区
        const newValues = [...currentVals]
        let lastNum = nums[nums.length - 1]
        if (lastNum === undefined) lastNum = 0
        
        for (let i = rawLen; i < maxLen; i++) {
          lastNum += step
          let strNum = lastNum.toString()
          if (hasLeadingZero) strNum = strNum.padStart(padLen, '0')
          newValues.push(`${commonPrefix}${strNum}${commonSuffix}`)
        }
        
        // 使用逗号和空格拼接，保持与普通输入习惯一致
        v.valueString = newValues.join(', ')
      }
    }
  }
}

// 防抖定时器
let debounceTimer: ReturnType<typeof setTimeout> | null = null

// 核心执行逻辑：精确替换算法
const generateBlocks = () => {
  // 1. 数据清洗与解析
  const parsedVars = variables.value.map(v => {
    // 按逗号或换行拆分
    const vals = v.valueString
      .split(/,|\n/)
      .map(s => s.trim())
      .filter(s => s !== '')
    return { name: v.name, values: vals }
  })

  // 过滤掉完全没有输入值的变量
  const activeVars = parsedVars.filter(v => v.values.length > 0)

  if (activeVars.length === 0 || !templateText.value.trim()) {
    outputBlocks.value = []
    return
  }

  // 2. 确定生成批次 (N)
  const maxLen = Math.max(...activeVars.map(v => v.values.length))
  const newBlocks: string[] = []

  // 3. 将变量按名称长度降序排列，防止短变量优先匹配导致覆盖（如 [A] 误替换 [AA] 内部的 [A]）
  const sortedVars = [...activeVars].sort((a, b) => b.name.length - a.name.length)

  // 4. 精确替换循环
  for (let i = 0; i < maxLen; i++) {
    let currentBlock = templateText.value
    
    sortedVars.forEach(v => {
      // 如果当前变量的值数量不足，取最后一个值循环补齐
      const valIndex = i < v.values.length ? i : v.values.length - 1
      const val = v.values[valIndex]
      // 精确替换，直接匹配带括号的变量名进行替换
      currentBlock = currentBlock.split(v.name).join(val)
    })
    
    // 清除每个 block 的头部及尾部可能导致多余空行的空白字符
    newBlocks.push(currentBlock.trim())
  }

  // 4. 状态更新与渲染
  outputBlocks.value = newBlocks
  isCopied.value = false // 重置复制状态
}

// 自动生成模式监听
watch([templateText, variables], () => {
  if (debounceTimer) clearTimeout(debounceTimer)
  debounceTimer = setTimeout(() => {
    generateBlocks()
  }, 300)
}, { deep: true })

// 下载全部配置为一个文件 (使用 File System Access API)
const downloadAll = async () => {
  if (outputBlocks.value.length === 0) return
  const text = outputBlocks.value.join('\n\n')
  
  try {
    if ('showSaveFilePicker' in window) {
      // Modern way: showSaveFilePicker
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
      // Fallback for older browsers
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

// 分块保存为多个文件 (使用 File System Access API 选择文件夹)
const downloadSplit = async () => {
  if (outputBlocks.value.length === 0) return
  
  try {
    if ('showDirectoryPicker' in window) {
      // Modern way: showDirectoryPicker
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
      // Fallback for older browsers
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

// 一键复制功能
const copyAll = async () => {
  if (outputBlocks.value.length === 0) return
  // join 时使用双换行符隔开多个区块，但不会在最后一个区块后面增加换行
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

    <!-- Main Workspace with Margins and Gaps -->
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

      <!-- Left Resizer (hidden on mobile) -->
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
              <button @click="showSyntaxHelp = true" class="ml-2.5 text-text-muted hover:text-accent transition-colors cursor-help focus:outline-none" title="语法说明" aria-label="语法说明">
                <svg xmlns="http://www.w3.org/2000/svg" class="h-[1.125rem] w-[1.125rem]" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8.228 9c.549-1.165 2.03-2 3.772-2 2.21 0 4 1.343 4 3 0 1.4-1.278 2.575-3.006 2.907-.542.104-.994.54-.994 1.093m0 3h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
              </button>
            </h2>
            <p class="text-xs text-text-muted mt-0.5 ml-6">使用"英文逗号"分隔数值</p>
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
          <div v-for="(v, index) in variables" :key="v.id || index" class="bg-bg-tertiary/40 border border-border backdrop-blur-sm flex flex-col rounded-xl shadow-sm hover:shadow-md transition-shadow group">
            <div class="flex items-center justify-between px-3 py-2 border-b border-border bg-bg-tertiary/40 rounded-t-xl">
              <div class="relative">
                <input 
                  v-model="v.name"
                  class="input input-sm input-mono w-20 text-center tracking-wider"
                />
              </div>
              <button 
                @click="removeVariable(index)"
                class="text-text-muted hover:text-error hover:bg-error-bg p-1.5 rounded-md transition-all"
                title="删除变量"
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
              :placeholder="index === 0 ? '1, 2, 3...' : index === 1 ? '1-3' : index === 2 ? 'vlan10-13' : index === 3 ? '192.168.1.1-3' : '...'"
              spellcheck="false"
            ></textarea>
          </div>
        </div>
      </div>

      <!-- Right Resizer (hidden on mobile) -->
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
          </h2>

          <!-- 功能按钮区 -->
          <div class="flex space-x-2 items-center" v-if="outputBlocks.length > 0">
            <!-- 新增：发送到命令管理按钮 -->
            <button 
              @click="openSendModal"
              class="btn btn-sm btn-secondary group relative"
              title="发送到命令管理"
            >
              <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 text-success" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" />
              </svg>
              <span class="absolute top-full mt-2 right-0 px-3 py-1.5 bg-bg-primary text-text-primary text-xs font-medium rounded-lg opacity-0 group-hover:opacity-100 transition-opacity whitespace-nowrap shadow-xl pointer-events-none z-50">
                发送到命令管理
                <span class="absolute -top-1 right-2 w-2 h-2 bg-bg-primary rotate-45"></span>
              </span>
            </button>
            
            <button 
              @click="downloadSplit"
              class="btn btn-sm btn-secondary group relative"
              title="分块下载 (保存为多个文件)"
            >
              <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 text-accent" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 7v8a2 2 0 002 2h6M8 7V5a2 2 0 012-2h4.586a1 1 0 01.707.293l4.414 4.414a1 1 0 01.293.707V15a2 2 0 01-2 2h-2M8 7H6a2 2 0 00-2 2v10a2 2 0 002 2h8a2 2 0 002-2v-2" />
              </svg>
              <!-- tooltip -->
              <span class="absolute top-full mt-2 right-0 px-3 py-1.5 bg-bg-primary text-text-primary text-xs font-medium rounded-lg opacity-0 group-hover:opacity-100 transition-opacity whitespace-nowrap shadow-xl pointer-events-none z-50">
                分块下载
                <span class="absolute -top-1 right-2 w-2 h-2 bg-bg-primary rotate-45"></span>
              </span>
            </button>
            <button 
              @click="downloadAll"
              class="btn btn-sm btn-secondary group relative"
              title="合并下载 (保存为一个文件)"
            >
              <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 text-info" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
              </svg>
              <span class="absolute top-full mt-2 right-0 px-3 py-1.5 bg-bg-primary text-text-primary text-xs font-medium rounded-lg opacity-0 group-hover:opacity-100 transition-opacity whitespace-nowrap shadow-xl pointer-events-none z-50">
                合并下载
                <span class="absolute -top-1 right-2 w-2 h-2 bg-bg-primary rotate-45"></span>
              </span>
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
              <span class="absolute top-full mt-2 right-0 px-3 py-1.5 bg-bg-primary text-text-primary text-xs font-medium rounded-lg opacity-0 group-hover:opacity-100 transition-opacity whitespace-nowrap shadow-xl pointer-events-none z-50">
                {{ isCopied ? '已复制到剪贴板!' : '复制全部' }}
                <span class="absolute -top-1 right-2 w-2 h-2 bg-bg-primary rotate-45"></span>
              </span>
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
      aria-label="使用简介"
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
          <!-- 头部 -->
          <div class="modal-header">
            <h3 class="modal-header-title">
              <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 text-success" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" />
              </svg>
              发送到命令管理
              <span class="text-xs text-text-muted font-normal ml-2">将生成的配置创建为命令组</span>
            </h3>
            <button @click="closeSendModal" class="modal-close">
              <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>
          
          <!-- 表单内容 -->
          <div class="modal-body space-y-5">
            <!-- 模式选择 -->
            <div class="space-y-3">
              <label class="text-sm font-medium text-text-primary">创建模式</label>
              <div class="grid grid-cols-2 gap-3">
                <!-- 合并模式 -->
                <div 
                  class="mode-card"
                  :class="{ active: sendModal.mode === 'merge' }"
                  @click="sendModal.mode = 'merge'"
                >
                  <div class="mode-icon">📦</div>
                  <div class="mode-title">合并为一个命令组</div>
                  <div class="mode-desc">所有配置块合并，每个块作为一条命令</div>
                </div>
                <!-- 分开模式 -->
                <div 
                  class="mode-card"
                  :class="{ active: sendModal.mode === 'split' }"
                  @click="sendModal.mode = 'split'"
                >
                  <div class="mode-icon">📂</div>
                  <div class="mode-title">分开创建多个命令组</div>
                  <div class="mode-desc">每个配置块创建独立命令组</div>
                </div>
              </div>
            </div>
            
            <!-- 基本信息 -->
            <div class="space-y-4">
              <!-- 名称 -->
              <div class="space-y-1.5">
                <label class="text-sm font-medium text-text-primary">
                  {{ sendModal.mode === 'merge' ? '命令组名称' : '名称前缀' }}
                  <span class="text-error">*</span>
                </label>
                <input 
                  v-model="sendModal.form.name" 
                  type="text" 
                  class="w-full px-4 py-2.5 rounded-lg bg-bg-secondary border border-border text-sm text-text-primary placeholder:text-text-muted focus:outline-none focus:border-accent/50 focus:ring-1 focus:ring-accent/20 transition-all"
                  :placeholder="sendModal.mode === 'merge' ? '输入命令组名称' : '输入名称前缀'"
                />
              </div>
              
              <!-- 描述 -->
              <div class="space-y-1.5">
                <label class="text-sm font-medium text-text-primary">描述</label>
                <input 
                  v-model="sendModal.form.description" 
                  type="text" 
                  class="w-full px-4 py-2.5 rounded-lg bg-bg-secondary border border-border text-sm text-text-primary placeholder:text-text-muted focus:outline-none focus:border-accent/50 focus:ring-1 focus:ring-accent/20 transition-all"
                  placeholder="输入描述（可选）" 
                />
              </div>
              
              <!-- 标签 -->
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
                    class="flex-1 px-3 py-2 rounded-lg bg-bg-secondary border border-border text-sm text-text-primary placeholder:text-text-muted focus:outline-none focus:border-accent/50 transition-all"
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
              
              <!-- 预览信息 -->
              <div class="preview-box">
                <div class="preview-icon">📊</div>
                <div class="preview-content">
                  <template v-if="sendModal.mode === 'merge'">
                    <span class="preview-text">共 <strong class="text-accent">{{ outputBlocks.length }}</strong> 条命令将被添加</span>
                  </template>
                  <template v-else>
                    <span class="preview-text">将创建 <strong class="text-accent">{{ outputBlocks.length }}</strong> 个命令组</span>
                    <div class="preview-examples">
                      <span class="text-xs text-text-muted">命名规则：</span>
                      <code v-for="(ex, i) in sendPreview.examples" :key="i" class="px-2 py-0.5 rounded bg-bg-tertiary font-mono text-xs">{{ ex }}</code>
                      <span v-if="outputBlocks.length > 3" class="text-xs text-text-muted">...</span>
                    </div>
                  </template>
                </div>
              </div>
            </div>
          </div>
          
          <!-- 底部按钮 -->
          <div class="modal-footer">
            <button @click="closeSendModal" class="btn btn-secondary">取消</button>
            <button 
              @click="executeSend" 
              :disabled="sendModal.saving" 
              class="btn btn-primary"
            >
              <svg v-if="sendModal.saving" class="w-4 h-4 animate-spin" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <circle cx="12" cy="12" r="10" stroke-opacity="0.25"/>
                <path d="M12 2a10 10 0 0 1 10 10" stroke-opacity="1"/>
              </svg>
              {{ sendModal.saving ? '创建中...' : '确认创建' }}
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
          <button @click="goToCommands" class="toast-link">
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
            <strong class="text-text-primary">ConfigForge</strong> 是一款强大的自动化配置生成工具，主要用于快速批量生成具有规律性的多组配置文本。
          </p>

          <div class="bg-accent-bg/50 rounded-xl p-5 border border-accent/20 shadow-inner">
            <h4 class="font-semibold text-accent mb-4 flex items-center">
              <span class="text-base mr-1.5">🚀</span> 核心工作流：
            </h4>
            <ul class="space-y-4">
              <li class="flex flex-col">
                <span class="text-sm font-bold text-text-primary mb-1.5">1. Template Input (模板输入)</span>
                <span class="text-text-muted leading-relaxed">在左侧输入需要生成的框架内容（如交换机配置、批量命令等），并将需要动态变化的变量位用 <code class="px-1.5 py-0.5 bg-bg-tertiary rounded-md border border-border text-accent font-mono shadow-sm">[A]</code>, <code class="px-1.5 py-0.5 bg-bg-tertiary rounded-md border border-border text-accent font-mono shadow-sm">[B]</code> 等格式替代占位。</span>
              </li>
              <li class="flex flex-col">
                <span class="text-sm font-bold text-text-primary mb-1.5">2. Variables Mapping (变量映射)</span>
                <span class="text-text-muted leading-relaxed">在中列输入各个变量对应的具体数值。支持多行或英文逗号分隔，并拥有<strong class="text-accent">语法糖展开</strong>及<strong class="text-accent">等差数列补全</strong>等智能功能（详见该列右侧的问号提示）。</span>
              </li>
              <li class="flex flex-col">
                <span class="text-sm font-bold text-text-primary mb-1.5">3. Output Preview (输出预览)</span>
                <span class="text-text-muted leading-relaxed">右侧将自动并列渲染出所有的配置块。系统会以各变量中数量最多的为基准生成批次，数量不足的变量会循环使用最后一个值进行补充。生成后可通过底部悬浮按钮一键复制全部内容。</span>
              </li>
            </ul>
          </div>
          
          <div class="flex items-start bg-info-bg rounded-xl p-4 border border-info/30 shadow-sm">
             <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 text-info mt-0.5 shrink-0 mr-2.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
               <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
             </svg>
             <p class="text-sm text-info leading-relaxed">
               <strong>自动化生成提示：</strong>系统默认开启 <span class="font-semibold">"自动实时生成"</span>，随着您输入变量或模板内容，结果将无缝实时渲染，大大提高工作效率。
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
            您可以快速生成一段连续的变量序列，只需输入起始值与结束值并用连字符（<code class="px-1.5 py-0.5 bg-bg-tertiary rounded-md text-accent font-mono shadow-sm border border-border">-</code> 或 <code class="px-1.5 py-0.5 bg-bg-tertiary rounded-md text-accent font-mono shadow-sm border border-border">~</code>）连接。当输入框失去焦点时，将<strong>自动补全</strong>中间的数值。系统同时还支持多个变量数组的<strong>等差数列智能对齐</strong>！
          </p>
          
          <div class="bg-accent-bg/50 rounded-xl p-5 border border-accent/20 shadow-inner">
            <h4 class="font-semibold text-accent mb-4 flex items-center">
              <span class="text-base mr-1.5">📌</span> 常规连字符生成（主动式）：
            </h4>
            <ul class="space-y-4">
              <li class="flex flex-col">
                <span class="text-xs font-medium text-text-muted mb-1.5">▶ 基础数字序列</span>
                <div class="flex items-center space-x-2.5">
                  <code class="bg-bg-secondary px-2.5 py-1 rounded-lg border border-border shadow-sm text-text-primary font-mono font-medium tracking-tight">1-5</code>
                  <span class="text-text-muted text-xs">➜</span>
                  <code class="bg-bg-tertiary/80 px-2.5 py-1 rounded-lg border border-border shadow-sm text-text-secondary font-mono tracking-tight leading-relaxed">1, 2, 3, 4, 5</code>
                </div>
              </li>
              <li class="flex flex-col">
                <span class="text-xs font-medium text-text-muted mb-1.5">▶ 带前导零对齐</span>
                <div class="flex items-center space-x-2.5">
                  <code class="bg-bg-secondary px-2.5 py-1 rounded-lg border border-border shadow-sm text-text-primary font-mono font-medium tracking-tight">01~03</code>
                  <span class="text-text-muted text-xs">➜</span>
                  <code class="bg-bg-tertiary/80 px-2.5 py-1 rounded-lg border border-border shadow-sm text-text-secondary font-mono tracking-tight leading-relaxed">01, 02, 03</code>
                </div>
              </li>
              <li class="flex flex-col">
                <span class="text-xs font-medium text-text-muted mb-1.5">▶ 携带前后缀（例如IP、端口、VLAN）</span>
                <div class="flex items-center space-x-2.5">
                  <code class="bg-bg-secondary px-2.5 py-1 rounded-lg border border-border shadow-sm text-text-primary font-mono font-medium tracking-tight">vlan10-12</code>
                  <span class="text-text-muted text-xs">➜</span>
                  <code class="bg-bg-tertiary/80 px-2.5 py-1 rounded-lg border border-border shadow-sm text-text-secondary font-mono tracking-tight leading-relaxed">vlan10, vlan11, vlan12</code>
                </div>
              </li>
              <li class="flex flex-col">
                <div class="flex items-center space-x-2.5 pt-1">
                  <code class="bg-bg-secondary px-2.5 py-1 rounded-lg border border-border shadow-sm text-text-primary font-mono font-medium tracking-tight">192.168.1.1~3</code>
                  <span class="text-text-muted text-xs">➜</span>
                  <code class="bg-bg-tertiary/80 px-2.5 py-1 rounded-lg border border-border shadow-sm text-text-secondary font-mono tracking-tight leading-relaxed">192.168.1.1, 192.168.1.2, 192.168.1.3</code>
                </div>
              </li>
            </ul>
          </div>

          <div class="bg-success-bg/50 rounded-xl p-5 border border-success/20 shadow-inner">
            <h4 class="font-semibold text-success mb-4 flex items-center">
              <span class="text-base mr-1.5">🪄</span> 多变量等差对齐（智能式）：
            </h4>
            <p class="text-sm text-success leading-relaxed mb-3">
              当其他变量已经有了确定的总长度（比如变量 [A] 有 10 个值），您只需在新的变量框中输入 <strong>2个及以上</strong> 构成规律的数列，失去焦点时将**自动对齐补全到**最长的数量：
            </p>
            <ul class="space-y-4">
              <li class="flex flex-col">
                <span class="text-xs font-medium text-text-muted mb-1.5">▶ 极简智能外推补齐</span>
                <div class="flex items-center space-x-2.5">
                  <span class="text-text-muted text-xs">变量[A] 已经有 10 个值，变量[B] 填入：</span>
                  <code class="bg-bg-secondary px-2.5 py-1 rounded-lg border border-border shadow-sm text-text-primary font-mono font-medium tracking-tight">10, 20, 30</code>
                </div>
                <div class="flex items-center space-x-2.5 mt-2 ml-4">
                  <span class="text-text-muted text-xs">➜ 自动补全为 10 个：</span>
                  <code class="bg-bg-tertiary/80 px-2.5 py-1 flex-1 rounded-lg border border-border shadow-sm text-text-secondary font-mono tracking-tight leading-relaxed line-clamp-1" title="10, 20, 30, 40, 50, 60, 70, 80, 90, 100">10, 20, 30, 40, 50, 60, 70, 80, 90, 100</code>
                </div>
              </li>
            </ul>
          </div>
          
          <div class="flex items-start bg-warning-bg rounded-xl p-4 border border-warning/30 shadow-sm">
            <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 text-warning mt-0.5 shrink-0 mr-2.5" viewBox="0 0 20 20" fill="currentColor">
               <path fill-rule="evenodd" d="M8.257 3.099c.765-1.36 2.722-1.36 3.486 0l5.58 9.92c.75 1.334-.213 2.98-1.742 2.98H4.42c-1.53 0-2.493-1.646-1.743-2.98l5.58-9.92zM11 13a1 1 0 11-2 0 1 1 0 012 0zm-1-8a1 1 0 00-1 1v3a1 1 0 002 0V6a1 1 0 00-1-1z" clip-rule="evenodd" />
            </svg>
            <p class="text-sm text-warning leading-relaxed">
               为了保证性能，防止页面卡顿崩溃，单次补全的最大数量上限为 <strong>1000</strong> 项。而且您可以与普通逗号分隔的内容混合使用，如：
               <code class="bg-bg-tertiary/60 px-1.5 py-0.5 ml-0.5 rounded border border-warning/30 font-mono font-medium tracking-tight">vlan1, vlan5-8, vlan10</code>
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
.preview-examples {
  @apply flex items-center gap-2 mt-2 text-xs text-text-muted flex-wrap;
}
.preview-examples code {
  @apply px-2 py-0.5 rounded bg-bg-tertiary font-mono;
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

