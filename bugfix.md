# NetWeaverGo 项目架构分析报告

## 六、前端架构问题

### 6.1 类型定义重复

**位置：** `frontend/src/types/events.ts` 和 bindings

**问题：**

- 前端手动定义了与后端结构相似的类型
- 后端类型变化时前端需要手动同步

**优化建议：**

- 完全依赖 Wails 自动生成的 bindings 类型
- 或使用工具从 Go 结构自动生成 TypeScript 类型

### 6.2 状态管理分散

**位置：** 多个 Vue 组件

**问题：**

- 各组件管理自己的状态，没有全局状态管理
- `executionSnapshot` 在 `TaskExecution.vue` 中，其他组件无法访问

**优化建议：**

- 引入 Pinia 进行全局状态管理
- 或者使用 provide/inject 模式共享引擎状态

### 6.3 事件订阅清理

**位置：** `frontend/src/composables/useEngineEvents.ts`

**问题：**

- `Events.On` 返回的清理函数类型不确定，使用 `any` 处理
- 清理逻辑复杂

**优化建议：**

- 明确 Wails 事件 API 的返回类型
- 简化订阅/取消订阅逻辑

---
