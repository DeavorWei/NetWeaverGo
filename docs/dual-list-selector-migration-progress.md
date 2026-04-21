# DualListSelector 迁移实施进度报告

## 概述

本文档记录 DualListSelector 组件迁移方案的实施进度、当前问题及后续任务清单。

---

## 当前进度总览

| 阶段 | 状态 | 说明 |
|------|------|------|
| 前置条件：扩展 protocol 筛选 | ✅ 已完成 | DualListSelector 已支持 protocol 类型筛选 |
| 第一阶段：BatchPing.vue | ✅ 已完成 | 设备选择弹窗已替换为 DualListSelector |
| 第二阶段：TaskEditModal.vue | ⚠️ 有编译错误 | 代码已写入，但 vue-tsc 报 TS6133 错误 |
| 第三阶段：DeviceSelector.vue 重构 | ✅ 已完成 | 新建 DeviceSelectorModal.vue，已删除旧组件 |
| 编译验证 | ❌ 未通过 | 存在 TS6133 声明未使用错误 |
| 样式检查 | ⏳ 待执行 | - |
| 遗漏检查 | ⏳ 待执行 | - |

---

## 当前编译错误详情

### 错误信息

```
src/components/task/TaskEditModal.vue(779,1): error TS6133: 'DeviceSelectorModal' is declared but its value is never read.
src/components/task/TaskEditModal.vue(847,7): error TS6133: 'handleDeviceConfirm' is declared but its value is never read.
src/components/task/TaskEditModal.vue(1177,10): error TS6133: 'submit' is declared but its value is never read.
```

### 根本原因分析

**vue-tsc 对 fragment（多根节点）模板的识别问题**

TaskEditModal.vue 使用了 Vue 3 的 fragment 模板结构：

```vue
<template>
  <Transition name="modal">
    <!-- 主弹窗内容 -->
  </Transition>

  <!-- 设备选择弹窗 - 在 Transition 外部 -->
  <DeviceSelectorModal ... />
</template>
```

vue-tsc 在处理多根节点模板时，无法正确识别：
1. 组件的使用（`DeviceSelectorModal`）
2. 事件处理函数的引用（`handleDeviceConfirm`）
3. 其他在模板中使用但在 script 中声明的变量

### 解决方案选项

| 方案 | 描述 | 优点 | 缺点 |
|------|------|------|------|
| **方案 A** | 将 DeviceSelectorModal 移入 Transition 内部 | 结构清晰 | 需要调整 Transition 逻辑 |
| **方案 B** | 使用单一根节点包裹整个模板 | 兼容性好 | 增加一层 DOM |
| **方案 C** | 在 tsconfig 中关闭 noUnusedLocals | 快速解决 | 隐藏其他潜在问题 |
| **方案 D** | 使用 `// @ts-expect-error` 注释 | 针对性强 | 需要多处添加 |

**推荐方案**：方案 B - 使用单一根节点包裹，这是最稳妥的解决方案。

---

## 已完成的修改清单

### 1. DualListSelector 组件扩展

**文件**: [`frontend/src/components/common/DualListSelector/types/dualListSelector.ts`](frontend/src/components/common/DualListSelector/types/dualListSelector.ts)

- 扩展 `FilterConfig.type` 类型：`"group" | "tag" | "protocol" | "all"`

**文件**: [`frontend/src/components/common/DualListSelector/DualListSelector.vue`](frontend/src/components/common/DualListSelector/DualListSelector.vue)

- 新增 `protocolData` prop
- 更新 `filterOptions` 计算属性支持"按协议"选项
- 更新 `showSubFilter`、`subFilterLabel`、`subFilterOptions` 支持协议筛选

### 2. BatchPing.vue 替换（第一阶段）

**文件**: [`frontend/src/views/Tools/BatchPing.vue`](frontend/src/views/Tools/BatchPing.vue)

- 移除旧设备选择相关状态（showDeviceModal, selectedDeviceIds 等）
- 新增 DualListSelector 相关状态和配置
- 替换 Teleport 弹窗为 DualListSelector 组件
- 数据映射：`key: d.id`（数字 ID）

### 3. TaskEditModal.vue 替换（第二阶段）

**文件**: [`frontend/src/components/task/TaskEditModal.vue`](frontend/src/components/task/TaskEditModal.vue)

- 导入 DeviceSelectorModal 组件
- 新增 `showDeviceSelector`、`selectedDevicesPreview` 等状态
- 新增 `openDeviceSelector`、`handleDeviceConfirm` 函数
- 替换设备选择 checkbox 网格为按钮 + 弹窗模式
- 模板末尾添加 DeviceSelectorModal 组件

### 4. DeviceSelector.vue 重构（第三阶段）

**新建文件**: [`frontend/src/components/task/DeviceSelectorModal.vue`](frontend/src/components/task/DeviceSelectorModal.vue)

- 封装 DualListSelector 的设备选择弹窗
- Props: `visible`, `devices`, `selectedIPs`, `title`
- Emits: `update:visible`, `confirm(DeviceAsset[])`, `change`, `cancel`
- 数据映射：`key: d.ip`（IP 字符串）

**修改文件**: [`frontend/src/views/Tasks.vue`](frontend/src/views/Tasks.vue)

- 替换 DeviceSelector 导入为 DeviceSelectorModal
- 新增 `showDeviceSelector` 状态
- 替换内联设备选择为按钮 + 弹窗模式

**删除文件**: `frontend/src/components/task/DeviceSelector.vue`

---

## 后续任务清单

### 任务 1：修复 TaskEditModal.vue 编译错误

**优先级**: 高  
**状态**: 待执行

**具体步骤**:
1. 将 TaskEditModal.vue 模板改为单一根节点结构
2. 或将 DeviceSelectorModal 移入 Transition 内部
3. 重新编译验证

**预期结果**: 编译通过，无 TS6133 错误

---

### 任务 2：编译验证

**优先级**: 高  
**状态**: 待执行（依赖任务 1）

**具体步骤**:
1. 执行 `build.bat` 进行完整编译
2. 确认无 TypeScript 错误
3. 确认无 Vue 模板错误

---

### 任务 3：对照迁移方案检查遗漏

**优先级**: 中  
**状态**: 待执行（依赖任务 2）

**检查项**:
- [ ] BatchPing.vue 设备选择功能是否正常
- [ ] TaskEditModal.vue 设备选择功能是否正常
- [ ] Tasks.vue 设备选择功能是否正常
- [ ] 协议筛选功能是否正常工作
- [ ] 分组筛选功能是否正常工作
- [ ] 标签筛选功能是否正常工作
- [ ] 搜索功能是否正常工作
- [ ] 确认/取消回调是否正确触发

---

### 任务 4：样式检查

**优先级**: 中  
**状态**: 待执行（依赖任务 2）

**检查项**:
- [ ] DualListSelector 弹窗样式是否一致
- [ ] 设备选择按钮样式是否与设计一致
- [ ] 已选设备预览文本样式是否正确
- [ ] 暗色主题适配是否正常
- [ ] 响应式布局是否正常

---

### 任务 5：功能测试

**优先级**: 高  
**状态**: 待执行（依赖任务 2）

**测试场景**:

| 场景 | 文件 | 操作 |
|------|------|------|
| 批量 Ping 设备选择 | BatchPing.vue | 打开设备选择弹窗，选择设备，确认 |
| 任务编辑设备选择（普通任务） | TaskEditModal.vue | 编辑普通任务，选择设备，保存 |
| 任务编辑设备选择（拓扑任务） | TaskEditModal.vue | 编辑拓扑任务，选择设备，保存 |
| 新建任务设备选择 | Tasks.vue | 新建任务，选择设备，创建 |
| 协议筛选 | 所有使用处 | 切换协议筛选，验证结果 |
| 分组筛选 | 所有使用处 | 切换分组筛选，验证结果 |
| 标签筛选 | 所有使用处 | 切换标签筛选，验证结果 |

---

## 数据映射策略汇总

| 组件 | key 字段 | 说明 |
|------|----------|------|
| BatchPing.vue | `d.id` (number) | 使用设备数字 ID |
| TaskEditModal.vue | `d.ip` (string) | 通过 DeviceSelectorModal 使用 IP |
| Tasks.vue | `d.ip` (string) | 通过 DeviceSelectorModal 使用 IP |

---

## 风险与注意事项

1. **vue-tsc fragment 问题**: 多根节点模板可能导致类型检查误报，建议统一使用单一根节点
2. **数据映射一致性**: 不同组件使用不同的 key 字段，需确保数据一致性
3. **向后兼容**: DeviceSelector.vue 已删除，需确认无其他组件依赖

---

## 文件变更汇总

| 操作 | 文件路径 |
|------|----------|
| 修改 | `frontend/src/components/common/DualListSelector/types/dualListSelector.ts` |
| 修改 | `frontend/src/components/common/DualListSelector/DualListSelector.vue` |
| 修改 | `frontend/src/views/Tools/BatchPing.vue` |
| 修改 | `frontend/src/components/task/TaskEditModal.vue` |
| 修改 | `frontend/src/views/Tasks.vue` |
| 新建 | `frontend/src/components/task/DeviceSelectorModal.vue` |
| 删除 | `frontend/src/components/task/DeviceSelector.vue` |

---

*文档生成时间: 2026-04-21*
