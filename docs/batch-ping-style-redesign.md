# 批量 Ping 页面样式调整方案

## 1. 调整目标

将左侧固定面板（目标输入框 + 配置参数框）提取为独立交互组件，以弹窗形式通过 Header 栏的"设置"按钮触发，释放主内容区空间，使结果表格占满全宽。

### 核心需求

1. **提取交互组件**：将左侧"目标输入"和"配置参数"合并为一个 `PingSettingsModal` 弹窗组件
2. **设置按钮位置**：放置在 Header 右侧按钮组的最左边（导入设备按钮之前）
3. **首次自动弹窗**：首次打开页面且参数为空时，自动弹出设置弹窗；后续不再自动弹出

---

## 2. 当前布局分析

### 2.1 当前结构（BatchPing.vue）

```
┌─────────────────────────────────────────────────────────┐
│ Header: [标题 🏓 批量 Ping 检测]    [导入设备] [开始]   │
├──────────────┬──────────────────────────────────────────┤
│ Left Panel   │ Right Panel                              │
│ w-80 (320px) │ flex-1                                   │
│              │                                          │
│ ┌──────────┐ │ ┌──────────────────────────────────────┐ │
│ │ 目标输入  │ │ │ 进度条                               │ │
│ │ textarea  │ │ └──────────────────────────────────────┘ │
│ └──────────┘ │ ┌──────────────────────────────────────┐ │
│ ┌──────────┐ │ │ 实时统计                              │ │
│ │ 配置参数  │ │ └──────────────────────────────────────┘ │
│ │ 超时/重试 │ │ ┌──────────────────────────────────────┐ │
│ │ 并发/包大 │ │ │ 结果表格 (全宽)                      │ │
│ │ 间隔/开关 │ │ │                                      │ │
│ └──────────┘ │ └──────────────────────────────────────┘ │
└──────────────┴──────────────────────────────────────────┘
```

### 2.2 当前代码位置

| 区域 | 文件 | 行号 | 说明 |
|------|------|------|------|
| Header | BatchPing.vue | 623-663 | 标题 + 导入设备 + 开始/停止按钮 |
| 左侧面板容器 | BatchPing.vue | 667-668 | `w-80 flex flex-col gap-4` |
| 目标输入 | BatchPing.vue | 669-683 | textarea + v-model="targetInput" |
| 配置参数 | BatchPing.vue | 686-812 | 超时/重试/并发/包大小/间隔/开关 |
| 右侧面板 | BatchPing.vue | 816-1003 | 进度条 + 统计 + 结果表格 |
| 主内容区 | BatchPing.vue | 666 | `flex-1 flex gap-4 overflow-hidden` |

---

## 3. 调整后布局

### 3.1 新结构

```
┌─────────────────────────────────────────────────────────┐
│ Header: [标题]          [⚙ 设置] [导入设备] [开始]     │
├─────────────────────────────────────────────────────────┤
│ Main Content (全宽)                                     │
│                                                         │
│ ┌─────────────────────────────────────────────────────┐ │
│ │ 进度条                                              │ │
│ └─────────────────────────────────────────────────────┘ │
│ ┌─────────────────────────────────────────────────────┐ │
│ │ 实时统计                                            │ │
│ └─────────────────────────────────────────────────────┘ │
│ ┌─────────────────────────────────────────────────────┐ │
│ │ 结果表格 (全宽)                                     │ │
│ │                                                     │ │
│ └─────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────┘

弹窗（点击 ⚙ 设置 按钮触发）:
┌─────────────────────────────────────┐
│ Ping 检测设置              [✕]     │
├─────────────────────────────────────┤
│                                     │
│ ┌─ 目标输入 ──────────────────────┐ │
│ │ textarea                        │ │
│ └─────────────────────────────────┘ │
│                                     │
│ ┌─ 配置参数 ──────────────────────┐ │
│ │ 超时 (ms)          [1000    ]   │ │
│ │ 重试次数           [1       ]   │ │
│ │ 并发数             [64      ]   │ │
│ │ 包大小 (bytes)     [32      ]   │ │
│ │ 间隔 (ms)          [0       ]   │ │
│ │ ─────────────────────────────   │ │
│ │ 解析主机名         [开关    ]   │ │
│ │ 启用实时进度       [开关    ]   │ │
│ │ 更新间隔(ms)       [100     ]   │ │
│ └─────────────────────────────────┘ │
│                                     │
├─────────────────────────────────────┤
│                    [取消] [确定]    │
└─────────────────────────────────────┘
```

---

## 4. 详细实现方案

### 4.1 新增组件：PingSettingsModal

**文件路径**: `frontend/src/components/tools/PingSettingsModal.vue`

**职责**: 封装目标输入和配置参数，以弹窗形式呈现

**Props 设计**:

```typescript
interface Props {
  show: boolean                    // 控制弹窗显隐
  targetInput: string              // 目标输入（v-model）
  config: PingConfig               // 配置参数（v-model）
  resolveHostName: boolean         // 解析主机名开关（v-model）
  enableRealtime: boolean          // 实时进度开关（v-model）
  realtimeThrottle: number         // 更新间隔（v-model）
  disabled: boolean                // 运行中禁用编辑
}
```

**Emits 设计**:

```typescript
interface Emits {
  'update:show': (value: boolean) => void
  'update:targetInput': (value: string) => void
  'update:config': (value: PingConfig) => void
  'update:resolveHostName': (value: boolean) => void
  'update:enableRealtime': (value: boolean) => void
  'update:realtimeThrottle': (value: number) => void
  confirm: () => void              // 点击确定按钮
}
```

**弹窗样式规范**（遵循项目样式库）:

| 元素 | 使用的样式类 | 来源 |
|------|-------------|------|
| 遮罩层 | `modal-overlay` | `styles/components/modal.css` |
| 弹窗容器 | `modal-container` | `styles/components/modal.css` |
| 弹窗主体 | `modal modal-lg modal-glass` | `styles/components/modal.css` |
| 弹窗头部 | `modal-header` | `styles/components/modal.css` |
| 头部标题 | `modal-header-title` | `styles/components/modal.css` |
| 关闭按钮 | `modal-close` | `styles/components/modal.css` |
| 弹窗内容 | `modal-body` | `styles/components/modal.css` |
| 弹窗底部 | `modal-footer` | `styles/components/modal.css` |
| 确定按钮 | `btn btn-primary` | `styles/components/button.css` |
| 取消按钮 | `btn btn-secondary` | `styles/components/button.css` |
| 输入框 | `input input-sm` | `styles/components/input.css` |
| 文本域 | `input textarea textarea-no-resize` | `styles/components/input.css` |
| 分组卡片 | `glass-card` | `styles/utilities/_glass.css` |

**弹窗尺寸**: 使用 `modal-lg`（max-width: 48rem），因为需要容纳目标输入 textarea 和多行配置参数，比标准弹窗更宽。

**弹窗内部布局**:

```html
<template>
  <Teleport to="body">
    <!-- 遮罩层 -->
    <div v-if="show" class="modal-overlay" @click.self="emit('update:show', false)">
      <!-- 容器 -->
      <div class="modal-container">
        <!-- 弹窗主体 -->
        <div class="modal modal-lg modal-glass">
          <!-- 头部 -->
          <div class="modal-header">
            <div class="modal-header-title">
              <svg><!-- 齿轮图标 --></svg>
              Ping 检测设置
            </div>
            <button class="modal-close" @click="emit('update:show', false)">
              <svg><!-- 关闭图标 --></svg>
            </button>
          </div>

          <!-- 内容区 -->
          <div class="modal-body space-y-4">
            <!-- 目标输入分组 -->
            <div class="glass-card p-4">
              <h3 class="text-sm font-semibold text-text-primary mb-3 flex items-center">
                <svg class="w-5 h-5 mr-2 text-accent"><!-- 剪贴板图标 --></svg>
                目标输入
              </h3>
              <textarea
                :value="targetInput"
                @input="emit('update:targetInput', $event.target.value)"
                :disabled="disabled"
                placeholder="输入 IP 地址&#10;支持格式：&#10;• 单个 IP: 192.168.1.1&#10;• CIDR: 192.168.1.0/24&#10;• 范围: 192.168.1.1-100&#10;• 多个 IP: 192.168.1.1, 192.168.1.2&#10;• 混合: 192.168.1.1, 192.168.1.0/30"
                class="input textarea textarea-no-resize h-40 font-mono"
              ></textarea>
            </div>

            <!-- 配置参数分组 -->
            <div class="glass-card p-4">
              <h3 class="text-sm font-semibold text-text-primary mb-3 flex items-center">
                <svg class="w-5 h-5 mr-2 text-accent"><!-- 齿轮图标 --></svg>
                配置参数
              </h3>
              <div class="space-y-3">
                <!-- 各配置项：与当前左侧面板内容一致 -->
                <!-- 超时/重试/并发/包大小/间隔/开关 -->
              </div>
            </div>
          </div>

          <!-- 底部 -->
          <div class="modal-footer">
            <button class="btn btn-secondary" @click="emit('update:show', false)">取消</button>
            <button class="btn btn-primary" @click="handleConfirm">确定</button>
          </div>
        </div>
      </div>
    </div>
  </Teleport>
</template>
```

### 4.2 Header 栏调整

**调整前** (BatchPing.vue 行 628):

```html
<div class="flex gap-2">
  <!-- 导入设备按钮 -->
  <button v-if="!isRunning" @click="openDeviceModal" ...>导入设备</button>
  <!-- 开始按钮 -->
  <button v-if="!isRunning" @click="startPing" ...>开始</button>
  <!-- 停止按钮 -->
  <button v-else @click="stopPing" ...>停止</button>
</div>
```

**调整后**:

```html
<div class="flex gap-2">
  <!-- 设置按钮（始终显示，运行中禁用） -->
  <button
    @click="showSettingsModal = true"
    :disabled="isRunning"
    class="btn btn-secondary"
    :class="{ 'opacity-50 cursor-not-allowed': isRunning }"
  >
    <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <!-- 齿轮图标 (与配置参数标题同图标) -->
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
        d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
        d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
    </svg>
    设置
  </button>
  <!-- 导入设备按钮 -->
  <button v-if="!isRunning" @click="openDeviceModal" ...>导入设备</button>
  <!-- 开始按钮 -->
  <button v-if="!isRunning" @click="startPing" ...>开始</button>
  <!-- 停止按钮 -->
  <button v-else @click="stopPing" ...>停止</button>
</div>
```

**设置按钮样式说明**:
- 使用项目样式库的 `btn btn-secondary` 类（透明背景 + 边框 + hover 变色）
- 齿轮图标与当前配置参数标题的图标一致，保持视觉统一
- 运行中禁用（`disabled`），与导入设备按钮行为一致
- 始终可见（不受 `isRunning` 的 `v-if` 控制），运行中仅灰显

### 4.3 主内容区调整

**调整前** (BatchPing.vue 行 666):

```html
<div class="flex-1 flex gap-4 overflow-hidden">
  <!-- Left Panel: Input (w-80) -->
  <div class="w-80 flex flex-col gap-4">
    <!-- 目标输入 + 配置参数 -->
  </div>
  <!-- Right Panel: Results -->
  <div class="flex-1 flex flex-col gap-4 overflow-hidden">
    <!-- 进度条 + 统计 + 结果表格 -->
  </div>
</div>
```

**调整后**:

```html
<div class="flex-1 flex flex-col gap-4 overflow-hidden">
  <!-- 进度条（原 Right Panel 内容，直接展开） -->
  <!-- 实时统计 -->
  <!-- 结果表格（全宽） -->
</div>
```

**变更要点**:
- 移除左侧 `w-80` 面板容器
- 移除 `flex gap-4` 横向布局（不再需要左右分栏）
- 右侧面板内容直接作为主内容区的子元素
- 结果表格自动占满全宽

### 4.4 首次自动弹窗逻辑

**实现方式**: 在 `onMounted` 中判断是否为首次打开且参数为空

```typescript
// 新增状态
const showSettingsModal = ref(false)
const hasAutoShownSettings = ref(false)  // 标记是否已自动弹窗过

// onMounted 中追加逻辑
onMounted(async () => {
  loadColumnConfig()

  // 获取默认配置
  try {
    const defaultConfig = await PingService.GetPingDefaultConfig()
    if (defaultConfig) {
      config.value = defaultConfig
    }
  } catch (err) {
    console.error('Failed to get default config:', err)
  }

  // 首次打开且目标输入为空时，自动弹出设置弹窗
  if (!hasAutoShownSettings.value && !targetInput.value.trim()) {
    showSettingsModal.value = true
    hasAutoShownSettings.value = true
  }

  // 订阅事件...
})
```

**判断条件**: `targetInput` 为空（trim 后为空字符串），说明用户尚未输入任何目标地址，此时自动弹窗引导用户配置。

**持久化考虑**: `hasAutoShownSettings` 不需要持久化到 localStorage。每次页面加载（组件挂载）视为一次"首次打开"，只要目标输入为空就自动弹窗。一旦用户输入了目标地址，后续进入页面不再自动弹窗。

### 4.5 PingSettingsModal 组件引用

在 BatchPing.vue 中引入并使用：

```html
<!-- 设置弹窗 -->
<PingSettingsModal
  v-model:show="showSettingsModal"
  v-model:targetInput="targetInput"
  v-model:config="config"
  v-model:resolveHostName="resolveHostName"
  v-model:enableRealtime="enableRealtime"
  v-model:realtimeThrottle="realtimeThrottle"
  :disabled="isRunning"
  @confirm="showSettingsModal = false"
/>
```

---

## 5. 样式映射表

确保所有新增/调整的样式均来自项目样式库，不引入外部依赖：

| 场景 | 样式类 | 来源文件 |
|------|--------|---------|
| 弹窗遮罩 | `modal-overlay` | `styles/components/modal.css:7` |
| 弹窗容器 | `modal-container` | `styles/components/modal.css:18` |
| 弹窗主体 | `modal modal-lg modal-glass` | `styles/components/modal.css:30,57,126` |
| 弹窗头部 | `modal-header` | `styles/components/modal.css:71` |
| 头部标题 | `modal-header-title` | `styles/components/modal.css:79` |
| 关闭按钮 | `modal-close` | `styles/components/modal.css:88` |
| 弹窗内容 | `modal-body` | `styles/components/modal.css:108` |
| 弹窗底部 | `modal-footer` | `styles/components/modal.css:115` |
| 确定按钮 | `btn btn-primary` | `styles/components/button.css:42` |
| 取消按钮 | `btn btn-secondary` | `styles/components/button.css:55` |
| 设置按钮 | `btn btn-secondary` | `styles/components/button.css:55` |
| 输入框 | `input input-sm` | `styles/components/input.css:7,42` |
| 文本域 | `input textarea textarea-no-resize` | `styles/components/input.css:72,78` |
| 分组卡片 | `glass-card` | `styles/utilities/_glass.css:142` |
| 开关按钮 | 保持现有 toggle switch 内联样式 | BatchPing.vue 现有实现 |
| 数据包警告 | 保持现有警告样式 | BatchPing.vue 现有实现 |

---

## 6. 变更清单

### 6.1 新增文件

| 文件 | 说明 |
|------|------|
| `frontend/src/components/tools/PingSettingsModal.vue` | Ping 设置弹窗组件 |

### 6.2 修改文件

| 文件 | 变更内容 |
|------|---------|
| `frontend/src/views/Tools/BatchPing.vue` | 见下方详细变更 |

### 6.3 BatchPing.vue 详细变更

#### Script 部分

| 变更 | 说明 |
|------|------|
| 新增 import | `import PingSettingsModal from '@/components/tools/PingSettingsModal.vue'` |
| 新增 `showSettingsModal` ref | `const showSettingsModal = ref(false)` |
| 新增 `hasAutoShownSettings` ref | `const hasAutoShownSettings = ref(false)` |
| 修改 `onMounted` | 追加首次自动弹窗逻辑 |
| 移除 `dataSizeWarning` | 移至 PingSettingsModal 内部（或保留在父组件通过 prop 传递） |

> **关于 dataSizeWarning**: 建议保留在 BatchPing.vue 中，通过 prop 传入 PingSettingsModal。因为该计算属性依赖 `config.value.DataSize`，而 config 通过 v-model 双向绑定，父组件的计算属性可以实时响应变化。

#### Template 部分

| 变更 | 说明 |
|------|------|
| Header 按钮组 | 在最左侧新增"设置"按钮 |
| 移除左侧面板 | 删除行 667-813 的 `w-80` 容器及其内容 |
| 主内容区 | 移除 `flex gap-4` 横向布局，改为 `flex flex-col gap-4` 纵向布局 |
| 右侧面板 | 移除外层 `flex-1` 容器，内容直接作为主内容区子元素 |
| 新增弹窗组件 | `<PingSettingsModal ... />` |

#### Style 部分

| 变更 | 说明 |
|------|------|
| 无变更 | scoped 样式中的 `.glass`、`.shadow-card`、`.shadow-glow`、`.ping-animation` 保留，因为右侧面板仍在使用 |

---

## 7. 交互细节

### 7.1 设置按钮

- **位置**: Header 右侧按钮组最左
- **图标**: 齿轮（与配置参数标题图标一致）
- **文字**: "设置"
- **样式**: `btn btn-secondary`（次要按钮，透明背景 + 边框）
- **行为**: 点击打开设置弹窗；运行中 disabled 灰显

### 7.2 设置弹窗

- **打开方式**: 点击设置按钮 / 首次自动弹出
- **关闭方式**: 点击遮罩层 / 点击关闭按钮 / 点击取消按钮 / 点击确定按钮
- **确定按钮**: 关闭弹窗（参数已通过 v-model 实时同步，无需额外保存动作）
- **取消按钮**: 关闭弹窗（v-model 已实时同步，取消不回滚。如需回滚，需在打开时缓存快照，取消时恢复——此为可选增强）
- **运行中**: 所有输入项 disabled，与当前行为一致

### 7.3 首次自动弹窗

- **触发条件**: 组件挂载时 `targetInput` 为空
- **触发次数**: 每次页面加载最多一次
- **后续行为**: 用户输入目标地址后，再次进入页面不再自动弹窗

---

## 8. 可选增强（不在本次需求范围内，供参考）

1. **取消回滚**: 打开弹窗时缓存当前参数快照，点击取消时恢复到快照状态
2. **设置按钮徽标**: 当目标输入为空时，在设置按钮上显示红点徽标提示用户需要配置
3. **快捷键**: 支持 `Ctrl+S` 快速打开设置弹窗
4. **弹窗动画**: 使用 `modal-slide-up` 动画变体，从底部滑入
5. **参数摘要**: 设置按钮旁显示简要参数摘要（如 "3 目标 · 64 并发"），让用户无需打开弹窗即可了解当前配置
