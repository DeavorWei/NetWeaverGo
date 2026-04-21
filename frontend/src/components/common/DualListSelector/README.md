# DualListSelector 双列表选择器

双列表选择器组件，支持分组筛选、标签筛选、模糊搜索等功能。

## 基础用法

```vue
<template>
  <div>
    <button @click="showSelector = true">打开选择器</button>

    <DualListSelector
      v-model:visible="showSelector"
      :source-data="deviceList"
      :target-data="selectedDevices"
      :config="selectorConfig"
      @confirm="handleConfirm"
    />
  </div>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { DualListSelector } from "@/components/common/DualListSelector";
import type {
  ListItem,
  SelectorConfig,
} from "@/components/common/DualListSelector";

const showSelector = ref(false);

const deviceList: ListItem[] = [
  { key: "1", label: "Router-01", group: "core", tags: ["华为", "核心"] },
  { key: "2", label: "Switch-01", group: "access", tags: ["H3C", "接入"] },
  { key: "3", label: "Firewall-01", group: "security", tags: ["思科", "安全"] },
];

const selectedDevices = ref<ListItem[]>([]);

const selectorConfig: Partial<SelectorConfig> = {
  modalTitle: "选择设备",
  sourceTitle: "可用设备",
  targetTitle: "已选设备",
  enableSearch: true,
  enableGrouping: true,
  enableTagFilter: true,
  searchFields: ["label", "key"],
  maxSelection: 50,
};

const handleConfirm = (items: ListItem[]) => {
  selectedDevices.value = items;
};
</script>
```

## 带分组数据用法

```vue
<template>
  <DualListSelector
    v-model:visible="showSelector"
    :source-data="allCommands"
    :target-data="selectedCommands"
    :group-data="commandGroups"
    :config="config"
    @confirm="handleConfirm"
  />
</template>

<script setup lang="ts">
const commandGroups = [
  {
    key: "system",
    label: "系统命令",
    items: [
      { key: "sys-1", label: "display version" },
      { key: "sys-2", label: "display clock" },
    ],
  },
  {
    key: "interface",
    label: "接口命令",
    items: [
      { key: "if-1", label: "display interface" },
      { key: "if-2", label: "display ip interface" },
    ],
  },
];
</script>
```

## Props

| 属性 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| visible | `boolean` | - | 是否显示弹窗，支持 `v-model:visible` |
| sourceData | `ListItem[]` | - | 源数据列表 |
| targetData | `ListItem[]` | `[]` | 初始已选数据 |
| groupData | `GroupData[]` | `[]` | 分组数据 |
| tagData | `{ key: string; label: string }[]` | `[]` | 标签数据 |
| config | `Partial<SelectorConfig>` | `{}` | 组件配置 |
| loading | `boolean` | `false` | 加载状态 |

## Events

| 事件 | 参数 | 说明 |
| --- | --- | --- |
| update:visible | `boolean` | 更新显示状态 |
| confirm | `ListItem[]` | 确认选择 |
| cancel | - | 取消选择 |
| change | `ListItem[]` | 选择变化 |

## SelectorConfig 配置项

| 字段 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| enableGrouping | `boolean` | `true` | 是否支持分组显示 |
| enableTagFilter | `boolean` | `true` | 是否支持标签筛选 |
| enableSearch | `boolean` | `true` | 是否启用搜索 |
| searchFields | `string[]` | `["label", "key"]` | 搜索字段 |
| sourceTitle | `string` | `"可选项"` | 左栏标题 |
| targetTitle | `string` | `"已选项"` | 右栏标题 |
| showCount | `boolean` | `true` | 是否显示计数 |
| maxSelection | `number` | - | 最大选择数量 |
| modalTitle | `string` | `"选择项目"` | 弹窗标题 |
| confirmText | `string` | `"确认"` | 确认按钮文本 |
| cancelText | `string` | `"取消"` | 取消按钮文本 |
