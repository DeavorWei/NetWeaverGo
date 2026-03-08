<template>
  <div class="device-selector space-y-3">
    <!-- 筛选方式与选项合并行 -->
    <div class="flex items-center gap-4 flex-wrap bg-bg-panel/50 p-2 rounded-lg border border-border/50">
      <div class="flex items-center gap-3">
        <span class="text-xs font-medium text-text-muted uppercase tracking-wider">筛选方式:</span>
        <div class="flex gap-1.5 flex-wrap">
          <button
            v-for="filter in filterOptions"
            :key="filter.value"
            @click="applyFilter(filter.value)"
            :class="[
              'px-2.5 py-1.5 text-xs font-medium rounded-md border transition-all duration-200',
              currentFilter === filter.value
                ? 'bg-accent border-accent text-white shadow-sm'
                : 'bg-bg-card border-border text-text-muted hover:text-text-primary hover:border-accent/30'
            ]"
          >
            {{ filter.label }}
          </button>
        </div>
      </div>

      <!-- 分组/标签/协议下拉选择 (移动到右侧) -->
      <div v-if="currentFilter !== 'all' && currentFilter !== 'manual'" class="flex items-center gap-3 pl-4 border-l border-border/50">
        <span class="text-xs font-medium text-text-muted uppercase tracking-wider">
          {{ filterLabel }}:
        </span>
        <select
          v-model="selectedOption"
          class="px-3 py-1.5 rounded-md bg-bg-card border border-border text-xs text-text-primary focus:outline-none focus:border-accent/50 transition-all min-w-[140px]"
        >
          <option value="">全部{{ filterLabel }}</option>
          <option v-for="opt in selectOptions" :key="opt.value" :value="opt.value">
            {{ opt.label }} ({{ opt.count }} 台)
          </option>
        </select>
      </div>
    </div>

    <!-- 已选设备展示 -->
    <div class="space-y-1.5">
      <div class="flex items-center justify-between">
        <span class="text-sm text-text-muted">
          已选择: <span class="text-accent font-medium">{{ selectedDevices.length }}</span> 台设备
          <span v-if="currentFilter === 'all'" class="text-xs text-text-muted/60 ml-1">(全选模式)</span>
        </span>
        <button
          v-if="selectedDevices.length > 0"
          @click="clearSelection"
          class="text-xs text-text-muted hover:text-error transition-colors"
        >
          清空选择
        </button>
      </div>
      
      <!-- 全选模式: 只显示统计信息 -->
      <div v-if="selectedDevices.length > 0 && currentFilter === 'all'" class="p-3 bg-bg-secondary/30 rounded-lg">
        <div class="flex items-center gap-2 text-sm text-text-secondary">
          <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4 text-accent" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <polyline points="20 6 9 17 4 12"/>
          </svg>
          <span>已选择全部 <span class="font-semibold text-accent">{{ selectedDevices.length }}</span> 台设备</span>
        </div>
        <div class="mt-2 flex gap-4 text-xs text-text-muted">
          <span>协议: {{ protocolStats }}</span>
          <span>分组: {{ groupStats }}</span>
        </div>
      </div>
      
      <!-- 非全选模式: 显示设备列表 -->
      <div v-if="selectedDevices.length > 0 && currentFilter !== 'all'" class="max-h-32 overflow-y-auto scrollbar-custom border border-border rounded-lg">
        <div
          v-for="device in selectedDevices"
          :key="device.IP"
          class="flex items-center justify-between px-3 py-2 border-b border-border/50 last:border-0 hover:bg-bg-secondary/30 transition-colors"
        >
          <div class="flex items-center gap-3">
            <span class="font-mono text-sm text-text-primary">{{ device.IP }}</span>
            <span class="text-xs text-text-muted">{{ device.Protocol }}</span>
            <span v-if="device.Group" class="text-xs px-1.5 py-0.5 rounded bg-bg-panel text-text-muted">{{ device.Group }}</span>
            <span v-if="device.Tag" class="text-xs px-1.5 py-0.5 rounded bg-accent/10 text-accent">{{ device.Tag }}</span>
          </div>
          <button
            @click="removeDevice(device.IP)"
            class="p-1 text-text-muted hover:text-error transition-colors rounded hover:bg-error/10"
            title="移除此设备"
          >
            <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/>
            </svg>
          </button>
        </div>
      </div>
    </div>

    <!-- 手动选择模式: 设备列表 -->
    <Transition name="slide">
      <div v-if="currentFilter === 'manual'" class="border border-border rounded-lg overflow-hidden">
        <div class="bg-bg-panel px-4 py-2 border-b border-border flex items-center justify-between">
          <span class="text-sm text-text-primary font-medium">设备列表</span>
          <div class="flex items-center gap-2">
            <input
              v-model="searchKeyword"
              type="text"
              placeholder="搜索 IP..."
              class="px-3 py-1.5 text-xs rounded-md bg-bg-secondary border border-border text-text-primary placeholder:text-text-muted focus:outline-none focus:border-accent/50 w-40"
            />
            <button
              @click="selectAllVisible"
              class="px-2 py-1 text-xs text-accent hover:text-accent-glow transition-colors"
            >
              全选
            </button>
            <button
              @click="deselectAllVisible"
              class="px-2 py-1 text-xs text-text-muted hover:text-text-primary transition-colors"
            >
              取消
            </button>
          </div>
        </div>
        <div class="max-h-32 overflow-y-auto scrollbar-custom">
          <div
            v-for="device in filteredDeviceList"
            :key="device.IP"
            @click="toggleDevice(device)"
            :class="[
              'flex items-center justify-between px-4 py-2 border-b border-border/50 cursor-pointer transition-colors last:border-0',
              isDeviceSelected(device.IP)
                ? 'bg-accent/5 hover:bg-accent/10'
                : 'hover:bg-bg-secondary/50'
            ]"
          >
            <div class="flex items-center gap-3">
              <div
                :class="[
                  'w-4 h-4 rounded border flex items-center justify-center transition-colors',
                  isDeviceSelected(device.IP)
                    ? 'bg-accent border-accent'
                    : 'border-border'
                ]"
              >
                <svg v-if="isDeviceSelected(device.IP)" xmlns="http://www.w3.org/2000/svg" class="w-3 h-3 text-white" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3">
                  <polyline points="20 6 9 17 4 12"/>
                </svg>
              </div>
              <span class="font-mono text-sm text-text-primary">{{ device.IP }}</span>
              <span class="text-xs text-text-muted">{{ device.Protocol }}</span>
            </div>
            <span class="text-xs text-text-muted">{{ device.Group || '默认分组' }}</span>
          </div>
        </div>
      </div>
    </Transition>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import type { DeviceAsset } from '../../bindings/github.com/NetWeaverGo/core/internal/config/models'

const props = defineProps<{
  devices: DeviceAsset[]
}>()

const emit = defineEmits<{
  selectionChange: [devices: DeviceAsset[]]
}>()

const currentFilter = ref<'all' | 'group' | 'tag' | 'protocol' | 'manual'>('all')
const selectedOption = ref('')
const searchKeyword = ref('')
const selectedIPs = ref<Set<string>>(new Set())

const filterOptions = [
  { label: '全选', value: 'all' },
  { label: '按分组', value: 'group' },
  { label: '按标签', value: 'tag' },
  { label: '按协议', value: 'protocol' },
  { label: '手动选择', value: 'manual' }
]

// 计算筛选标签
const filterLabel = computed(() => {
  switch (currentFilter.value) {
    case 'group': return '分组'
    case 'tag': return '标签'
    case 'protocol': return '协议'
    default: return ''
  }
})

// 计算下拉选项
const selectOptions = computed(() => {
  const options: { label: string; value: string; count: number }[] = []
  
  if (currentFilter.value === 'group') {
    const groups = new Map<string, number>()
    props.devices.forEach(d => {
      const group = d.Group || '默认分组'
      groups.set(group, (groups.get(group) || 0) + 1)
    })
    groups.forEach((count, group) => {
      options.push({ label: group, value: group, count })
    })
  } else if (currentFilter.value === 'tag') {
    const tags = new Map<string, number>()
    props.devices.forEach(d => {
      if (d.Tag) {
        tags.set(d.Tag, (tags.get(d.Tag) || 0) + 1)
      }
    })
    tags.forEach((count, tag) => {
      options.push({ label: tag, value: tag, count })
    })
  } else if (currentFilter.value === 'protocol') {
    const protocols = new Map<string, number>()
    props.devices.forEach(d => {
      protocols.set(d.Protocol, (protocols.get(d.Protocol) || 0) + 1)
    })
    protocols.forEach((count, protocol) => {
      options.push({ label: protocol, value: protocol, count })
    })
  }
  
  return options
})

// 过滤后的设备列表（手动选择模式）
const filteredDeviceList = computed(() => {
  // 数据验证：确保 devices 数组有效
  if (!props.devices || props.devices.length === 0) return []
  
  // 如果没有搜索关键字，返回全部设备
  if (!searchKeyword.value) return props.devices
  
  // 根据关键字过滤
  const keyword = searchKeyword.value.toLowerCase()
  return props.devices.filter(d => {
    // 容错处理：确保 IP 字段存在
    const ip = d.IP || ''
    return ip.toLowerCase().includes(keyword)
  })
})

// 选中的设备列表
const selectedDevices = computed(() => {
  return props.devices.filter(d => selectedIPs.value.has(d.IP))
})

// 协议统计信息
const protocolStats = computed(() => {
  const stats = new Map<string, number>()
  selectedDevices.value.forEach(d => {
    const protocol = d.Protocol || '未知'
    stats.set(protocol, (stats.get(protocol) || 0) + 1)
  })
  return Array.from(stats.entries()).map(([k, v]) => `${k}(${v})`).join(', ')
})

// 分组统计信息
const groupStats = computed(() => {
  const stats = new Map<string, number>()
  selectedDevices.value.forEach(d => {
    const group = d.Group || '默认分组'
    stats.set(group, (stats.get(group) || 0) + 1)
  })
  return Array.from(stats.entries()).map(([k, v]) => `${k}(${v})`).join(', ')
})

// 应用筛选
function applyFilter(filter: string) {
  currentFilter.value = filter as any
  selectedOption.value = ''
  selectedIPs.value.clear()
  
  if (filter === 'all') {
    // 全选
    props.devices.forEach(d => selectedIPs.value.add(d.IP))
  }
  
  emitSelection()
}

// 监听筛选选项变化
watch(selectedOption, (option) => {
  selectedIPs.value.clear()
  
  if (currentFilter.value === 'group') {
    props.devices.forEach(d => {
      if ((d.Group || '默认分组') === option) {
        selectedIPs.value.add(d.IP)
      }
    })
  } else if (currentFilter.value === 'tag') {
    props.devices.forEach(d => {
      if (d.Tag === option) {
        selectedIPs.value.add(d.IP)
      }
    })
  } else if (currentFilter.value === 'protocol') {
    props.devices.forEach(d => {
      if (d.Protocol === option) {
        selectedIPs.value.add(d.IP)
      }
    })
  }
  
  emitSelection()
})

// 切换设备选择
function toggleDevice(device: DeviceAsset) {
  if (selectedIPs.value.has(device.IP)) {
    selectedIPs.value.delete(device.IP)
  } else {
    selectedIPs.value.add(device.IP)
  }
  emitSelection()
}

// 检查设备是否已选
function isDeviceSelected(ip: string) {
  return selectedIPs.value.has(ip)
}

// 移除设备
function removeDevice(ip: string) {
  selectedIPs.value.delete(ip)
  emitSelection()
}

// 清空选择
function clearSelection() {
  selectedIPs.value.clear()
  currentFilter.value = 'all'
  selectedOption.value = ''
  emitSelection()
}

// 全选可见设备
function selectAllVisible() {
  filteredDeviceList.value.forEach(d => selectedIPs.value.add(d.IP))
  emitSelection()
}

// 取消选择可见设备
function deselectAllVisible() {
  filteredDeviceList.value.forEach(d => selectedIPs.value.delete(d.IP))
  emitSelection()
}

// 发送选择变化事件
function emitSelection() {
  emit('selectionChange', selectedDevices.value)
}

// 初始化时全选
onMounted(() => {
  props.devices.forEach(d => selectedIPs.value.add(d.IP))
  emitSelection()
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
