<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted } from 'vue'

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

const doResize = (e: MouseEvent) => {
  if (!isResizing.value || !resizeType.value) return
  
  // 获取窗口总宽度
  const windowWidth = document.documentElement.clientWidth
  // 鼠标位置对应的百分比
  const mousePct = (e.clientX / windowWidth) * 100

  // 设定最小容忍宽度百分比
  const minW = 15
  
  if (resizeType.value === 'left') {
    // 调整 leftBar: 改变 leftCol 和 midCol
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
      class="fixed top-6 left-1/2 transform -translate-x-1/2 z-[100] transition-all duration-300 pointer-events-none"
      :class="showToast ? 'translate-y-0 opacity-100 visible scale-100' : '-translate-y-4 opacity-0 invisible scale-95'"
    >
      <div class="px-5 py-3 bg-slate-800/90 backdrop-blur-xl border border-white/10 rounded-2xl shadow-2xl flex items-center space-x-3">
        <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 text-emerald-400" viewBox="0 0 20 20" fill="currentColor">
          <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd" />
        </svg>
        <span class="text-white text-[14px] font-medium tracking-wide drop-shadow-sm">{{ toastMessage }}</span>
      </div>
    </div>

    <!-- Main Workspace with Margins and Gaps -->
    <main class="flex-1 flex overflow-y-auto md:overflow-hidden flex-col md:flex-row p-2 sm:p-4 gap-2.5 z-10">
      
      <!-- Column 1: 配置模版 -->
      <div 
        class="min-h-[300px] md:h-full md:min-h-[700px] glass-panel flex flex-col rounded-2xl overflow-hidden shadow-xl"
        :class="'w-full md:w-auto'"
        :style="{ width: windowWidth < 768 ? '100%' : leftColWidth + '%' }"
      >
        <div class="px-5 py-4 border-b border-slate-200 dark:border-slate-700/50 flex justify-between items-center shrink-0 bg-white dark:bg-slate-800/30">
          <h2 class="text-sm font-semibold text-slate-800 dark:text-slate-200 flex items-center">
            <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 mr-2 text-indigo-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
            </svg>
            配置模版
          </h2>
        </div>
        <div class="flex-1 p-4 flex flex-col">
          <textarea 
            v-model="templateText"
            class="flex-1 w-full resize-none outline-none premium-textarea p-4 rounded-xl font-mono text-sm leading-relaxed text-slate-700 dark:text-slate-300" 
            spellcheck="false" 
            placeholder="在此处输入配置模板，例如：&#10;interface GigabitEthernet0/0/[A]&#10; description [B]&#10;..."></textarea>
        </div>
      </div>

      <!-- Left Resizer (hidden on mobile) -->
      <div 
        class="hidden md:flex w-1.5 cursor-col-resize hover:bg-indigo-400/50 rounded-full active:bg-indigo-500/80 transition-all z-20 flex-shrink-0" 
        @mousedown="startResize('left')"
      ></div>

      <!-- Column 2: Variables Mapping -->
      <div 
        class="min-h-[400px] md:h-full md:min-h-[700px] glass-panel flex flex-col rounded-2xl overflow-hidden shadow-xl"
        :class="'w-full md:w-auto'"
        :style="{ width: windowWidth < 768 ? '100%' : midColWidth + '%' }"
      >
        <div class="px-5 py-4 border-b border-slate-200 dark:border-slate-700/50 flex justify-between items-center shrink-0 bg-white dark:bg-slate-800/30">
          <div>
            <h2 class="text-sm font-semibold text-slate-800 dark:text-slate-200 flex items-center">
              <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 mr-2 text-sky-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" />
              </svg>
              变量映射
              <button @click="showSyntaxHelp = true" class="ml-2.5 text-slate-400 dark:text-slate-500 hover:text-indigo-500 transition-colors cursor-help focus:outline-none" title="语法说明" aria-label="语法说明">
                <svg xmlns="http://www.w3.org/2000/svg" class="h-[1.125rem] w-[1.125rem]" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8.228 9c.549-1.165 2.03-2 3.772-2 2.21 0 4 1.343 4 3 0 1.4-1.278 2.575-3.006 2.907-.542.104-.994.54-.994 1.093m0 3h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
              </button>
            </h2>
            <p class="text-[10px] text-slate-500 dark:text-slate-400 mt-0.5 ml-6">使用"英文逗号"分隔数值</p>
          </div>
          <button 
            @click="addVariable"
            class="p-1 px-3 text-xs bg-white dark:bg-slate-800/60 hover:bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-700 text-slate-700 dark:text-slate-300 rounded-lg flex items-center shadow-sm hover:shadow transition-all duration-200"
          >
            <svg xmlns="http://www.w3.org/2000/svg" class="h-3.5 w-3.5 mr-1 text-indigo-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
            </svg>
            添加变量
          </button>
        </div>
        <div class="flex-1 overflow-y-auto p-4 space-y-4">
          <div v-for="(v, index) in variables" :key="v.id || index" class="bg-white dark:bg-slate-800/40 border border-slate-200 dark:border-slate-700/60 backdrop-blur-sm flex flex-col rounded-xl shadow-sm hover:shadow-md transition-shadow group">
            <div class="flex items-center justify-between px-3 py-2 border-b border-white/50 bg-white dark:bg-slate-800/40 rounded-t-xl">
              <div class="relative">
                <input 
                  v-model="v.name"
                  class="px-3 py-1 bg-slate-800 dark:bg-slate-700 text-white text-xs font-mono rounded-lg w-20 outline-none focus:ring-2 focus:ring-indigo-400 shadow-inner text-center tracking-wider"
                />
              </div>
              <button 
                @click="removeVariable(index)"
                class="text-slate-400 dark:text-slate-500 hover:text-rose-500 hover:bg-rose-50 p-1.5 rounded-md transition-all"
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
              class="w-full h-[64px] resize-y outline-none px-3 py-2.5 text-sm font-mono bg-transparent focus:bg-white dark:bg-slate-800/50 dark:focus:bg-slate-800/50 transition-colors rounded-b-xl leading-relaxed text-slate-700 dark:text-slate-300 placeholder-slate-400"
              :placeholder="index === 0 ? '1, 2, 3...' : index === 1 ? '1-3' : index === 2 ? 'vlan10-13' : index === 3 ? '192.168.1.1-3' : '...'"
              spellcheck="false"
            ></textarea>
          </div>
        </div>
      </div>

      <!-- Right Resizer (hidden on mobile) -->
      <div 
        class="hidden md:flex w-1.5 cursor-col-resize hover:bg-sky-400/50 rounded-full active:bg-sky-500/80 transition-all z-20 flex-shrink-0" 
        @mousedown="startResize('right')"
      ></div>

      <!-- Column 3: Output Preview -->
      <div 
        class="min-h-[400px] md:h-full md:min-h-[700px] glass-panel relative flex flex-col rounded-2xl overflow-hidden shadow-xl"
        :class="'w-full md:w-auto'"
         :style="{ width: windowWidth < 768 ? '100%' : rightColWidth + '%' }"
      >
        <div class="px-5 py-4 border-b border-slate-200 dark:border-slate-700/50 flex justify-between items-center shrink-0 bg-white dark:bg-slate-800/30">
          <h2 class="text-sm font-semibold text-slate-800 dark:text-slate-200 flex items-center shrink-0">
            <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 mr-2 text-teal-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 20l4-16m4 4l4 4-4 4M6 16l-4-4 4-4" />
            </svg>
            生成预览
            <span class="ml-2 px-2 py-0.5 bg-indigo-100 text-indigo-700 text-xs rounded-full font-mono">{{ outputBlocks.length }} blocks</span>
          </h2>

          <!-- 功能按钮区 -->
          <div class="flex space-x-2 items-center" v-if="outputBlocks.length > 0">
            <button 
              @click="downloadSplit"
              class="p-1.5 bg-white dark:bg-slate-800/60 hover:bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-700 text-indigo-500 rounded-lg flex items-center shadow-sm hover:shadow transition-all duration-200 group relative focus:outline-none"
              title="分块下载 (保存为多个文件)"
            >
              <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 7v8a2 2 0 002 2h6M8 7V5a2 2 0 012-2h4.586a1 1 0 01.707.293l4.414 4.414a1 1 0 01.293.707V15a2 2 0 01-2 2h-2M8 7H6a2 2 0 00-2 2v10a2 2 0 002 2h8a2 2 0 002-2v-2" />
              </svg>
              <!-- tooltip -->
              <span class="absolute top-full mt-2 right-0 px-3 py-1.5 bg-slate-800 text-white text-xs font-medium rounded-lg opacity-0 group-hover:opacity-100 transition-opacity whitespace-nowrap shadow-xl pointer-events-none z-50">
                分块下载
                <span class="absolute -top-1 right-2 w-2 h-2 bg-slate-800 rotate-45"></span>
              </span>
            </button>
            <button 
              @click="downloadAll"
              class="p-1.5 bg-white dark:bg-slate-800/60 hover:bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-700 text-sky-500 rounded-lg flex items-center shadow-sm hover:shadow transition-all duration-200 group relative focus:outline-none"
              title="合并下载 (保存为一个文件)"
            >
              <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
              </svg>
              <span class="absolute top-full mt-2 right-0 px-3 py-1.5 bg-slate-800 text-white text-xs font-medium rounded-lg opacity-0 group-hover:opacity-100 transition-opacity whitespace-nowrap shadow-xl pointer-events-none z-50">
                合并下载
                <span class="absolute -top-1 right-2 w-2 h-2 bg-slate-800 rotate-45"></span>
              </span>
            </button>
            <button 
              @click="copyAll"
              class="p-1.5 bg-indigo-50 border border-indigo-100 text-indigo-600 hover:bg-indigo-100 rounded-lg flex items-center shadow-sm transition-all duration-200 group relative focus:outline-none"
              title="复制全部"
            >
              <svg v-if="!isCopied" xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
              </svg>
              <svg v-else xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
              </svg>
              <span class="absolute top-full mt-2 right-0 px-3 py-1.5 bg-slate-800 text-white text-xs font-medium rounded-lg opacity-0 group-hover:opacity-100 transition-opacity whitespace-nowrap shadow-xl pointer-events-none z-50">
                {{ isCopied ? '已复制到剪贴板!' : '复制全部' }}
                <span class="absolute -top-1 right-2 w-2 h-2 bg-slate-800 rotate-45"></span>
              </span>
            </button>
          </div>
        </div>
        
        <div class="flex-1 overflow-y-auto p-5 pb-5">
          <template v-if="outputBlocks.length > 0">
            <div 
              v-for="(block, idx) in outputBlocks" 
              :key="idx" 
              class="p-4 mb-4 bg-white dark:bg-slate-800/60 backdrop-blur-sm border border-slate-200 dark:border-slate-700/60 rounded-xl font-mono text-sm whitespace-pre-wrap text-slate-700 dark:text-slate-300 shadow-sm transition-all hover:shadow-md"
            >{{ block }}</div>
          </template>
          <div v-else class="h-full flex flex-col items-center justify-center text-slate-400 dark:text-slate-500 space-y-3">
            <svg xmlns="http://www.w3.org/2000/svg" class="h-12 w-12 text-slate-300" fill="none" viewBox="0 0 24 24" stroke="currentColor">
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
      class="fixed bottom-6 right-6 z-40 w-12 h-12 flex items-center justify-center rounded-full bg-gradient-to-r from-indigo-500 to-sky-500 hover:from-indigo-600 hover:to-sky-600 text-white shadow-lg shadow-indigo-500/30 hover:shadow-indigo-500/50 hover:scale-110 transition-all duration-300 cursor-help focus:outline-none focus:ring-2 focus:ring-indigo-400 focus:ring-offset-2"
      title="使用简介"
      aria-label="使用简介"
    >
      <svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8.228 9c.549-1.165 2.03-2 3.772-2 2.21 0 4 1.343 4 3 0 1.4-1.278 2.575-3.006 2.907-.542.104-.994.54-.994 1.093m0 3h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
      </svg>
    </button>

    <!-- 使用简介弹窗 -->
    <div v-if="showUsageHelp" class="fixed inset-0 z-50 flex items-center justify-center p-4">
      <div class="absolute inset-0 bg-slate-900/40 backdrop-blur-sm transition-opacity" @click="showUsageHelp = false"></div>
      
      <div class="relative bg-white dark:bg-slate-800/95 backdrop-blur-xl border border-white/50 rounded-2xl shadow-2xl w-full max-w-2xl max-h-[90vh] overflow-hidden flex flex-col transform transition-all">
        <div class="px-6 py-4 border-b border-slate-200 dark:border-slate-700/60 bg-white dark:bg-slate-800/50 flex justify-between items-center shrink-0">
          <h3 class="text-lg font-bold text-slate-800 dark:text-slate-200 flex items-center">
            <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 mr-2 text-indigo-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            ConfigForge 使用简介
          </h3>
          <button @click="showUsageHelp = false" class="text-slate-400 dark:text-slate-500 hover:text-slate-600 dark:text-slate-400 transition-colors p-1.5 rounded-lg hover:bg-slate-100 dark:bg-slate-800">
            <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>
        
        <div class="p-6 overflow-y-auto text-sm text-slate-700 dark:text-slate-300 space-y-5">
          <p class="leading-relaxed text-[15px]">
            <strong>ConfigForge</strong> 是一款强大的自动化配置生成工具，主要用于快速批量生成具有规律性的多组配置文本。
          </p>

          <div class="bg-indigo-50/50 rounded-xl p-5 border border-indigo-100/50 shadow-inner">
            <h4 class="font-semibold text-indigo-800 mb-4 flex items-center">
              <span class="text-base mr-1.5">🚀</span> 核心工作流：
            </h4>
            <ul class="space-y-4">
              <li class="flex flex-col">
                <span class="text-[14px] font-bold text-indigo-900 mb-1.5">1. Template Input (模板输入)</span>
                <span class="text-slate-600 dark:text-slate-400 leading-relaxed">在左侧输入需要生成的框架内容（如交换机配置、批量命令等），并将需要动态变化的变量位用 <code class="px-1.5 py-0.5 bg-white dark:bg-slate-800 rounded-md border text-indigo-600 font-mono shadow-sm">[A]</code>, <code class="px-1.5 py-0.5 bg-white dark:bg-slate-800 rounded-md border text-indigo-600 font-mono shadow-sm">[B]</code> 等格式替代占位。</span>
              </li>
              <li class="flex flex-col">
                <span class="text-[14px] font-bold text-indigo-900 mb-1.5">2. Variables Mapping (变量映射)</span>
                <span class="text-slate-600 dark:text-slate-400 leading-relaxed">在中列输入各个变量对应的具体数值。支持多行或英文逗号分隔，并拥有<strong class="text-indigo-600">语法糖展开</strong>及<strong class="text-indigo-600">等差数列补全</strong>等智能功能（详见该列右侧的问号提示）。</span>
              </li>
              <li class="flex flex-col">
                <span class="text-[14px] font-bold text-indigo-900 mb-1.5">3. Output Preview (输出预览)</span>
                <span class="text-slate-600 dark:text-slate-400 leading-relaxed">右侧将自动并列渲染出所有的配置块。系统会以各变量中数量最多的为基准生成批次，数量不足的变量会循环使用最后一个值进行补充。生成后可通过底部悬浮按钮一键复制全部内容。</span>
              </li>
            </ul>
          </div>
          
          <div class="flex items-start bg-sky-50 rounded-xl p-4 border border-sky-200/80 shadow-sm">
             <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 text-sky-500 mt-0.5 shrink-0 mr-2.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
               <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
             </svg>
             <p class="text-[13px] text-sky-700 leading-relaxed">
               <strong>自动化生成提示：</strong>系统默认开启 <span class="font-semibold">“自动实时生成”</span>，随着您输入变量或模板内容，结果将无缝实时渲染，大大提高工作效率。
             </p>
          </div>
        </div>
        
        <div class="px-6 py-4 bg-slate-50 dark:bg-slate-800/50 border-t border-slate-200 dark:border-slate-700/60 flex justify-end rounded-b-2xl shrink-0">
          <button @click="showUsageHelp = false" class="px-6 py-2 bg-gradient-to-r from-indigo-500 to-sky-500 hover:from-indigo-600 hover:to-sky-600 text-white font-semibold rounded-xl shadow-lg shadow-indigo-500/20 hover:shadow-indigo-500/40 hover:-translate-y-0.5 transition-all duration-300">
            我知道了
          </button>
        </div>
      </div>
    </div>

    <!-- 语法说明弹窗 -->
    <div v-if="showSyntaxHelp" class="fixed inset-0 z-50 flex items-center justify-center p-4">
      <div class="absolute inset-0 bg-slate-900/40 backdrop-blur-sm transition-opacity" @click="showSyntaxHelp = false"></div>
      
      <div class="relative bg-white dark:bg-slate-800/95 backdrop-blur-xl border border-white/50 rounded-2xl shadow-2xl w-full max-w-2xl max-h-[90vh] overflow-hidden flex flex-col transform transition-all">
        <div class="px-6 py-4 border-b border-slate-200 dark:border-slate-700/60 bg-white dark:bg-slate-800/50 flex justify-between items-center shrink-0">
          <h3 class="text-lg font-bold text-slate-800 dark:text-slate-200 flex items-center">
            <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 mr-2 text-indigo-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            语法糖智能展开说明
          </h3>
          <button @click="showSyntaxHelp = false" class="text-slate-400 dark:text-slate-500 hover:text-slate-600 dark:text-slate-400 transition-colors p-1.5 rounded-lg hover:bg-slate-100 dark:bg-slate-800">
            <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>
        
        <div class="p-6 overflow-y-auto text-sm text-slate-700 dark:text-slate-300 space-y-5">
          <p class="leading-relaxed">
            您可以快速生成一段连续的变量序列，只需输入起始值与结束值并用连字符（<code class="px-1.5 py-0.5 bg-slate-100 dark:bg-slate-800/80 rounded-md text-indigo-600 font-mono shadow-sm border border-slate-200 dark:border-slate-700/50">-</code> 或 <code class="px-1.5 py-0.5 bg-slate-100 dark:bg-slate-800/80 rounded-md text-indigo-600 font-mono shadow-sm border border-slate-200 dark:border-slate-700/50">~</code>）连接。当输入框失去焦点时，将<strong>自动补全</strong>中间的数值。系统同时还支持多个变量数组的<strong>等差数列智能对齐</strong>！
          </p>
          
          <div class="bg-indigo-50/50 rounded-xl p-5 border border-indigo-100/50 shadow-inner">
            <h4 class="font-semibold text-indigo-800 mb-4 flex items-center">
              <span class="text-base mr-1.5">📌</span> 常规连字符生成（主动式）：
            </h4>
            <ul class="space-y-4">
              <li class="flex flex-col">
                <span class="text-xs font-medium text-slate-500 dark:text-slate-400 mb-1.5">▶ 基础数字序列</span>
                <div class="flex items-center space-x-2.5">
                  <code class="bg-white dark:bg-slate-800 px-2.5 py-1 rounded-lg border border-slate-200 dark:border-slate-700 shadow-sm text-slate-800 dark:text-slate-200 font-mono font-medium tracking-tight">1-5</code>
                  <span class="text-slate-400 dark:text-slate-500 text-xs">➜</span>
                  <code class="bg-white dark:bg-slate-800/80 px-2.5 py-1 rounded-lg border border-slate-200 dark:border-slate-700/60 shadow-sm text-slate-600 dark:text-slate-400 font-mono tracking-tight leading-relaxed">1, 2, 3, 4, 5</code>
                </div>
              </li>
              <li class="flex flex-col">
                <span class="text-xs font-medium text-slate-500 dark:text-slate-400 mb-1.5">▶ 带前导零对齐</span>
                <div class="flex items-center space-x-2.5">
                  <code class="bg-white dark:bg-slate-800 px-2.5 py-1 rounded-lg border border-slate-200 dark:border-slate-700 shadow-sm text-slate-800 dark:text-slate-200 font-mono font-medium tracking-tight">01~03</code>
                  <span class="text-slate-400 dark:text-slate-500 text-xs">➜</span>
                  <code class="bg-white dark:bg-slate-800/80 px-2.5 py-1 rounded-lg border border-slate-200 dark:border-slate-700/60 shadow-sm text-slate-600 dark:text-slate-400 font-mono tracking-tight leading-relaxed">01, 02, 03</code>
                </div>
              </li>
              <li class="flex flex-col">
                <span class="text-xs font-medium text-slate-500 dark:text-slate-400 mb-1.5">▶ 携带前后缀（例如IP、端口、VLAN）</span>
                <div class="flex items-center space-x-2.5">
                  <code class="bg-white dark:bg-slate-800 px-2.5 py-1 rounded-lg border border-slate-200 dark:border-slate-700 shadow-sm text-slate-800 dark:text-slate-200 font-mono font-medium tracking-tight">vlan10-12</code>
                  <span class="text-slate-400 dark:text-slate-500 text-xs">➜</span>
                  <code class="bg-white dark:bg-slate-800/80 px-2.5 py-1 rounded-lg border border-slate-200 dark:border-slate-700/60 shadow-sm text-slate-600 dark:text-slate-400 font-mono tracking-tight leading-relaxed">vlan10, vlan11, vlan12</code>
                </div>
              </li>
              <li class="flex flex-col">
                <div class="flex items-center space-x-2.5 pt-1">
                  <code class="bg-white dark:bg-slate-800 px-2.5 py-1 rounded-lg border border-slate-200 dark:border-slate-700 shadow-sm text-slate-800 dark:text-slate-200 font-mono font-medium tracking-tight">192.168.1.1~3</code>
                  <span class="text-slate-400 dark:text-slate-500 text-xs">➜</span>
                  <code class="bg-white dark:bg-slate-800/80 px-2.5 py-1 rounded-lg border border-slate-200 dark:border-slate-700/60 shadow-sm text-slate-600 dark:text-slate-400 font-mono tracking-tight leading-relaxed">192.168.1.1, 192.168.1.2, 192.168.1.3</code>
                </div>
              </li>
            </ul>
          </div>

          <div class="bg-emerald-50/50 rounded-xl p-5 border border-emerald-100/50 shadow-inner">
            <h4 class="font-semibold text-emerald-800 mb-4 flex items-center">
              <span class="text-base mr-1.5">🪄</span> 多变量等差对齐（智能式）：
            </h4>
            <p class="text-[13px] text-emerald-700 leading-relaxed mb-3">
              当其他变量已经有了确定的总长度（比如变量 [A] 有 10 个值），您只需在新的变量框中输入 <strong>2个及以上</strong> 构成规律的数列，失去焦点时将**自动对齐补全到**最长的数量：
            </p>
            <ul class="space-y-4">
              <li class="flex flex-col">
                <span class="text-xs font-medium text-slate-500 dark:text-slate-400 mb-1.5">▶ 极简智能外推补齐</span>
                <div class="flex items-center space-x-2.5">
                  <span class="text-slate-500 dark:text-slate-400 text-xs">变量[A] 已经有 10 个值，变量[B] 填入：</span>
                  <code class="bg-white dark:bg-slate-800 px-2.5 py-1 rounded-lg border border-slate-200 dark:border-slate-700 shadow-sm text-slate-800 dark:text-slate-200 font-mono font-medium tracking-tight">10, 20, 30</code>
                </div>
                <div class="flex items-center space-x-2.5 mt-2 ml-4">
                  <span class="text-slate-400 dark:text-slate-500 text-xs">➜ 自动补全为 10 个：</span>
                  <code class="bg-white dark:bg-slate-800/80 px-2.5 py-1 flex-1 rounded-lg border border-slate-200 dark:border-slate-700/60 shadow-sm text-slate-600 dark:text-slate-400 font-mono tracking-tight leading-relaxed line-clamp-1" title="10, 20, 30, 40, 50, 60, 70, 80, 90, 100">10, 20, 30, 40, 50, 60, 70, 80, 90, 100</code>
                </div>
              </li>
            </ul>
          </div>
          
          <div class="flex items-start bg-amber-50 rounded-xl p-4 border border-amber-200/80 shadow-sm">
            <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 text-amber-500 mt-0.5 shrink-0 mr-2.5" viewBox="0 0 20 20" fill="currentColor">
               <path fill-rule="evenodd" d="M8.257 3.099c.765-1.36 2.722-1.36 3.486 0l5.58 9.92c.75 1.334-.213 2.98-1.742 2.98H4.42c-1.53 0-2.493-1.646-1.743-2.98l5.58-9.92zM11 13a1 1 0 11-2 0 1 1 0 012 0zm-1-8a1 1 0 00-1 1v3a1 1 0 002 0V6a1 1 0 00-1-1z" clip-rule="evenodd" />
            </svg>
            <p class="text-[13px] text-amber-700 leading-relaxed">
               为了保证性能，防止页面卡顿崩溃，单次补全的最大数量上限为 <strong>1000</strong> 项。而且您可以与普通逗号分隔的内容混合使用，如：
               <code class="bg-white dark:bg-slate-800/60 px-1.5 py-0.5 ml-0.5 rounded border border-amber-200 font-mono font-medium tracking-tight">vlan1, vlan5-8, vlan10</code>
            </p>
          </div>
        </div>
        
        <div class="px-6 py-4 bg-slate-50 dark:bg-slate-800/50 border-t border-slate-200 dark:border-slate-700/60 flex justify-end rounded-b-2xl shrink-0">
          <button @click="showSyntaxHelp = false" class="px-6 py-2 bg-gradient-to-r from-indigo-500 to-sky-500 hover:from-indigo-600 hover:to-sky-600 text-white font-semibold rounded-xl shadow-lg shadow-indigo-500/20 hover:shadow-indigo-500/40 hover:-translate-y-0.5 transition-all duration-300">
            我知道了
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
