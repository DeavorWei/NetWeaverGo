<template>
  <DualListSelector
    :visible="visible"
    @update:visible="val => $emit('update:visible', val)"
    :source-data="sourceData"
    :target-data="targetData"
    :group-data="groupData"
    :tag-data="tagData"
    :protocol-data="protocolData"
    :config="selectorConfig"
    @confirm="handleConfirm"
    @cancel="handleCancel"
    @change="handleChange"
  />
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { DualListSelector } from '@/components/common/DualListSelector'
import type { ListItem, GroupData, SelectorConfig } from '@/components/common/DualListSelector'
import type { DeviceAsset } from '../../services/api'

interface Props {
  /** 是否显示弹窗 */
  visible: boolean
  /** 设备列表 */
  devices: DeviceAsset[]
  /** 初始已选设备 IP 列表 */
  selectedIPs?: string[]
  /** 弹窗标题 */
  title?: string
}

const props = withDefaults(defineProps<Props>(), {
  selectedIPs: () => [],
  title: '选择设备'
})

const emit = defineEmits<{
  /** 更新显示状态 */
  (e: 'update:visible', value: boolean): void
  /** 确认选择 - 传出完整 DeviceAsset[] */
  (e: 'confirm', devices: DeviceAsset[]): void
  /** 选择变化 - 传出完整 DeviceAsset[] */
  (e: 'change', devices: DeviceAsset[]): void
  /** 取消 */
  (e: 'cancel'): void
}>()

// 源数据映射
const sourceData = computed<ListItem[]>(() =>
  props.devices.map(d => ({
    key: d.ip,
    label: d.ip,
    description: d.protocol,
    group: d.group || '默认分组',
    protocol: d.protocol,
    tags: d.tags
  }))
)

// 目标数据（已选）
const targetData = ref<ListItem[]>([])

// 监听 props.selectedIPs 变化，同步到 targetData
watch(
  () => props.selectedIPs,
  (newIPs) => {
    targetData.value = sourceData.value.filter(item => newIPs.includes(item.key as string))
  },
  { immediate: true }
)

// 监听 props.devices 变化，更新 targetData
watch(
  () => props.devices,
  () => {
    const selectedIPs = targetData.value.map(item => item.key as string)
    targetData.value = sourceData.value.filter(item => selectedIPs.includes(item.key as string))
  }
)

// 分组数据
const groupData = computed<GroupData[]>(() => {
  const groups = new Map<string, DeviceAsset[]>()
  props.devices.forEach(d => {
    const g = d.group || '默认分组'
    if (!groups.has(g)) groups.set(g, [])
    groups.get(g)!.push(d)
  })
  return Array.from(groups.entries()).map(([key, items]) => ({
    key,
    label: key,
    items: items.map(d => ({
      key: d.ip,
      label: d.ip,
      description: d.protocol
    }))
  }))
})

// 标签数据
const tagData = computed(() => {
  const tagMap = new Map<string, number>()
  props.devices.forEach(d => {
    d.tags?.forEach(tag => {
      tagMap.set(tag, (tagMap.get(tag) || 0) + 1)
    })
  })
  return Array.from(tagMap.entries()).map(([key, count]) => ({
    key,
    label: key,
    count
  }))
})

// 协议数据
const protocolData = computed(() => {
  const protocolMap = new Map<string, number>()
  props.devices.forEach(d => {
    protocolMap.set(d.protocol, (protocolMap.get(d.protocol) || 0) + 1)
  })
  return Array.from(protocolMap.entries()).map(([key, count]) => ({
    key,
    label: key,
    count
  }))
})

// 选择器配置
const selectorConfig = computed<Partial<SelectorConfig>>(() => ({
  modalTitle: props.title,
  sourceTitle: '可选项',
  targetTitle: '已选项',
  enableSearch: true,
  enableGrouping: true,
  enableTagFilter: true,
  searchFields: ['label', 'description', 'ip'],
  confirmText: '确认',
  cancelText: '取消'
}))

// 通过 key(ip) 查找原始设备对象
const findDevicesByItems = (items: ListItem[]): DeviceAsset[] => {
  return items
    .map(item => props.devices.find(d => d.ip === item.key))
    .filter((d): d is DeviceAsset => !!d)
}

// 确认回调
const handleConfirm = (items: ListItem[]) => {
  const selectedDevices = findDevicesByItems(items)
  emit('confirm', selectedDevices)
  emit('update:visible', false)
}

// 变化回调
const handleChange = (items: ListItem[]) => {
  const selectedDevices = findDevicesByItems(items)
  emit('change', selectedDevices)
}

// 取消回调
const handleCancel = () => {
  emit('cancel')
  emit('update:visible', false)
}
</script>
