# API 迁移指南

## 概述

v2.0 版本将移除所有向后兼容的 API 导出，请尽快迁移到新的命名空间 API。

## API 映射表

### 设备管理 API

| 旧 API (v1.x)                 | 新 API (v2.0)                           |
| ----------------------------- | --------------------------------------- |
| `ListDevices()`               | `DeviceAPI.listDevices()`               |
| `AddDevice(device)`           | `DeviceAPI.addDevice(device)`           |
| `UpdateDevice(index, device)` | `DeviceAPI.updateDevice(index, device)` |
| `DeleteDevice(index)`         | `DeviceAPI.deleteDevice(index)`         |
| `SaveDevices(devices)`        | `DeviceAPI.saveDevices(devices)`        |
| `GetProtocolDefaultPorts()`   | `DeviceAPI.getProtocolDefaultPorts()`   |
| `GetValidProtocols()`         | `DeviceAPI.getValidProtocols()`         |

### 命令组管理 API

| 旧 API (v1.x)                      | 新 API (v2.0)                                      |
| ---------------------------------- | -------------------------------------------------- |
| `ListCommandGroups()`              | `CommandGroupAPI.listCommandGroups()`              |
| `GetCommandGroup(id)`              | `CommandGroupAPI.getCommandGroup(id)`              |
| `CreateCommandGroup(group)`        | `CommandGroupAPI.createCommandGroup(group)`        |
| `UpdateCommandGroup(id, group)`    | `CommandGroupAPI.updateCommandGroup(id, group)`    |
| `DeleteCommandGroup(id)`           | `CommandGroupAPI.deleteCommandGroup(id)`           |
| `DuplicateCommandGroup(id)`        | `CommandGroupAPI.duplicateCommandGroup(id)`        |
| `ImportCommandGroup(filePath)`     | `CommandGroupAPI.importCommandGroup(filePath)`     |
| `ExportCommandGroup(id, filePath)` | `CommandGroupAPI.exportCommandGroup(id, filePath)` |
| `GetCommands()`                    | `CommandGroupAPI.getCommands()`                    |
| `SaveCommands(commands)`           | `CommandGroupAPI.saveCommands(commands)`           |

### 设置管理 API

| 旧 API (v1.x)                     | 新 API (v2.0)                                 |
| --------------------------------- | --------------------------------------------- |
| `LoadSettings()`                  | `SettingsAPI.loadSettings()`                  |
| `SaveSettings(settings)`          | `SettingsAPI.saveSettings(settings)`          |
| `EnsureConfig()`                  | `SettingsAPI.ensureConfig()`                  |
| `GetAppInfo()`                    | `SettingsAPI.getAppInfo()`                    |
| `LogInfo(category, ip, message)`  | `SettingsAPI.logInfo(category, ip, message)`  |
| `LogWarn(category, ip, message)`  | `SettingsAPI.logWarn(category, ip, message)`  |
| `LogError(category, ip, message)` | `SettingsAPI.logError(category, ip, message)` |

### 引擎控制 API

| 旧 API (v1.x)                                 | 新 API (v2.0)                                           |
| --------------------------------------------- | ------------------------------------------------------- |
| `StartEngine()`                               | `EngineAPI.startEngine()`                               |
| `StartEngineWithSelection(devices, cmdGroup)` | `EngineAPI.startEngineWithSelection(devices, cmdGroup)` |
| `StartBackup()`                               | `EngineAPI.startBackup()`                               |
| `ResolveSuspend(ip, action)`                  | `EngineAPI.resolveSuspend(ip, action)`                  |
| `IsRunning()`                                 | `EngineAPI.isRunning()`                                 |

### 任务组管理 API

| 旧 API (v1.x)                | 新 API (v2.0)                             |
| ---------------------------- | ----------------------------------------- |
| `ListTaskGroups()`           | `TaskGroupAPI.listTaskGroups()`           |
| `GetTaskGroup(id)`           | `TaskGroupAPI.getTaskGroup(id)`           |
| `CreateTaskGroup(group)`     | `TaskGroupAPI.createTaskGroup(group)`     |
| `UpdateTaskGroup(id, group)` | `TaskGroupAPI.updateTaskGroup(id, group)` |
| `DeleteTaskGroup(id)`        | `TaskGroupAPI.deleteTaskGroup(id)`        |
| `StartTaskGroup(id)`         | `TaskGroupAPI.startTaskGroup(id)`         |

## 迁移示例

### 示例 1：单个 API 调用

```typescript
// 迁移前
import { ListDevices } from "@/services/api";
const devices = await ListDevices();

// 迁移后
import { DeviceAPI } from "@/services/api";
const devices = await DeviceAPI.listDevices();
```

### 示例 2：多个 API 调用

```typescript
// 迁移前
import { ListDevices, AddDevice, DeleteDevice } from "@/services/api";

// 迁移后 - 使用命名空间
import { DeviceAPI } from "@/services/api";
```

### 示例 3：Settings.vue 特殊处理

```typescript
// 迁移前 - 直接绑定导入
import {
  LoadSettings,
  SaveSettings,
} from "../bindings/github.com/NetWeaverGo/core/internal/ui/settingsservice";

// 迁移后 - 统一使用 api.ts
import { SettingsAPI } from "@/services/api";
```

## 迁移工具

运行以下命令自动转换所有 API 调用：

```bash
# 迁移整个 src 目录
npm run migrate:api

# 迁移单个文件
npm run migrate:api:file src/views/Settings.vue
```

## 时间表

| 版本     | 时间     | 操作                       |
| -------- | -------- | -------------------------- |
| v1.5     | 当前     | 弃用警告 + 迁移文档        |
| v1.6-1.9 | 1-4 周后 | ESLint 强制 + Codemod 工具 |
| v2.0     | 6 周后   | 移除所有向后兼容代码       |

## 常见问题

### Q: 迁移后代码有什么优势？

1. **更好的 IDE 支持**：命名空间提供清晰的代码补全
2. **更小的包体积**：移除 150 行兼容代码，减少 2-3KB
3. **一致的导入方式**：所有 API 通过单一入口 `@/services/api`
4. **类型安全**：TypeScript 能更好地推断类型

### Q: 如果不迁移会怎样？

v2.0 发布后，旧 API 将完全不可用，代码会编译失败。请务必在 v2.0 前完成迁移。

### Q: 迁移工具能处理所有情况吗？

迁移工具能处理 99% 的情况，但以下场景需要手动检查：

- 动态 API 调用（如 `const fn = ListDevices; fn()`）
- 通过变量别名使用（如 `const fn = ListDevices`）
- 条件导入（`await import()`）

运行迁移工具后，请检查控制台输出和代码变更。
