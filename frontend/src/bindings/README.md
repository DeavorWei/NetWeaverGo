# Wails3 绑定说明

## 概述

本目录包含 Wails3 自动生成的 TypeScript 绑定文件，用于前端与后端 Go 服务的通信。

> ⚠️ **重要提示**: 所有绑定文件由 `wails3 generate bindings` 自动生成，**请勿手动编辑**！

## 目录结构

```
frontend/src/bindings/
├── README.md                           # 本说明文件
└── github.com/
    ├── NetWeaverGo/
    │   └── core/
    │       └── internal/
    │           ├── config/              # 配置相关类型定义
    │           │   ├── index.ts         # 模块导出聚合
    │           │   └── models.ts        # 数据模型类（结构体绑定）
    │           ├── executor/            # 执行器相关类型
    │           │   ├── index.ts
    │           │   └── models.ts
    │           └── ui/                  # UI 服务绑定
    │               ├── index.ts                    # 服务模块聚合导出
    │               ├── deviceservice.ts            # 设备管理服务
    │               ├── commandgroupservice.ts      # 命令组管理服务
    │               ├── settingsservice.ts          # 设置管理服务
    │               ├── engineservice.ts            # 引擎控制服务
    │               └── taskgroupservice.ts         # 任务组管理服务
    └── wailsapp/
        └── wails/
            └── v3/
                └── pkg/
                    └── application/    # Wails 应用核心类型
                        ├── index.ts
                        └── models.ts
```

## 文件类型说明

| 文件类型      | 说明                                             | 示例               |
| ------------- | ------------------------------------------------ | ------------------ |
| `*service.ts` | 服务绑定文件，包含对应 Go 服务的所有公开方法     | `deviceservice.ts` |
| `models.ts`   | 数据模型文件，包含对应 Go 结构体的 TypeScript 类 | `config/models.ts` |
| `index.ts`    | 模块索引文件，聚合导出目录下的所有绑定           | `ui/index.ts`      |

## 绑定调用方式

Wails3 使用数字 ID 进行方法调用，例如：

```typescript
// 自动生成的绑定代码
export function IsRunning(): Promise<boolean> & { cancel(): void } {
  let $resultPromise = $Call.ByID(271359178) as any;
  return $resultPromise;
}
```

### ID 映射参考表

| 服务                | 方法                     | 功能描述       |
| ------------------- | ------------------------ | -------------- |
| DeviceService       | ListDevices              | 获取设备列表   |
| DeviceService       | AddDevice                | 新增设备       |
| DeviceService       | UpdateDevice             | 更新设备       |
| DeviceService       | DeleteDevice             | 删除设备       |
| CommandGroupService | ListCommandGroups        | 获取命令组列表 |
| CommandGroupService | GetCommandGroup          | 获取单个命令组 |
| EngineService       | StartEngine              | 启动引擎       |
| EngineService       | StartEngineWithSelection | 按选择启动引擎 |
| EngineService       | ResolveSuspend           | 解除挂起状态   |
| EngineService       | IsRunning                | 检查运行状态   |
| TaskGroupService    | ListTaskGroups           | 获取任务组列表 |
| TaskGroupService    | StartTaskGroup           | 启动任务组     |

## 使用方式

### 推荐方式：使用命名空间 API

```typescript
import { DeviceAPI, EngineAPI } from "../services/api";

// 获取设备列表
const devices = await DeviceAPI.listDevices();

// 启动引擎
await EngineAPI.startEngine();
```

### 直接使用绑定（不推荐）

```typescript
import { ListDevices } from "../bindings/github.com/NetWeaverGo/core/internal/ui/deviceservice.js";

const devices = await ListDevices();
```

---

## 如何新增绑定

### 场景 A：在现有服务中添加新方法

**步骤 1：编辑 Go 服务文件**

例如在 `internal/ui/device_service.go` 中添加新方法：

```go
// GetDeviceByIP 根据 IP 获取设备信息
// 注意：方法首字母必须大写，否则不会被绑定
func (s *DeviceService) GetDeviceByIP(ip string) (*config.DeviceAsset, error) {
    assets, _, _, _, err := config.ParseOrGenerate(false)
    if err != nil {
        return nil, err
    }
    for _, asset := range assets {
        if asset.IP == ip {
            return &asset, nil
        }
    }
    return nil, fmt.Errorf("设备不存在: %s", ip)
}
```

**步骤 2：运行绑定生成**

```bash
wails3 generate bindings
# 或运行 build.bat
```

**步骤 3：在前端使用**

生成的 TypeScript 绑定会自动添加到 `deviceservice.ts`：

```typescript
import { GetDeviceByIP } from "../bindings/github.com/NetWeaverGo/core/internal/ui/deviceservice.js";

const device = await GetDeviceByIP("192.168.1.1");
```

---

### 场景 B：创建全新的服务

**步骤 1：创建 Go 服务文件**

在 `internal/ui/` 目录下创建新文件，如 `report_service.go`：

```go
package ui

import (
    "context"
    "github.com/wailsapp/wails/v3/pkg/application"
)

// ReportService 报表服务
type ReportService struct {
    wailsApp *application.App
}

// NewReportService 创建服务实例
func NewReportService() *ReportService {
    return &ReportService{}
}

// ServiceStartup Wails 服务启动生命周期钩子（必需）
func (s *ReportService) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
    s.wailsApp = application.Get()
    return nil
}

// ExportReport 导出报表（公开方法，首字母大写）
func (s *ReportService) ExportReport(format string, deviceIPs []string) (string, error) {
    // 业务逻辑...
    return "/path/to/report.pdf", nil
}
```

**步骤 2：注册服务**

编辑 `cmd/netweaver/main.go`：

```go
func runGUI() {
    // ... 现有服务 ...
    reportService := ui.NewReportService()  // 新增

    app := application.New(application.Options{
        Services: []application.Service{
            application.NewService(deviceService),
            application.NewService(commandGroupService),
            application.NewService(settingsService),
            application.NewService(engineService),
            application.NewService(taskGroupService),
            application.NewService(reportService),  // 新增注册
        },
        // ...
    })
}
```

**步骤 3：更新绑定配置**

编辑 `wails.json`：

```json
{
  "bindings": {
    "services": [
      {
        "package": "github.com/NetWeaverGo/core/internal/ui",
        "services": [
          "DeviceService",
          "CommandGroupService",
          "SettingsService",
          "EngineService",
          "TaskGroupService",
          "ReportService" // 新增
        ]
      }
    ]
  }
}
```

**步骤 4：生成绑定**

```bash
wails3 generate bindings
```

**生成的文件结构：**

```
frontend/src/bindings/github.com/NetWeaverGo/core/internal/ui/
├── reportservice.ts  ← 自动生成
└── index.ts          ← 自动更新导出
```

---

### 数据模型绑定

Go 结构体会自动生成对应的 TypeScript 类：

**Go 结构体** (`internal/config/report.go`)：

```go
type ReportConfig struct {
    Title    string
    Format   string
    Devices  []string
}
```

**自动生成** (`bindings/.../config/models.ts`)：

```typescript
export class ReportConfig {
    "Title": string;
    "Format": string;
    "Devices": string[];

    constructor($$source: Partial<ReportConfig> = {}) { ... }
    static createFrom($$source: any = {}): ReportConfig { ... }
}
```

---

## 关键规则总结

| 规则         | 说明                                                |
| ------------ | --------------------------------------------------- |
| **方法公开** | Go 方法首字母大写才会被绑定到前端                   |
| **生命周期** | 实现 `ServiceStartup` 方法获取 App 实例             |
| **服务注册** | 在 `main.go` 中使用 `application.NewService()` 注册 |
| **配置更新** | 新服务需添加到 `wails.json` 的 `services` 列表      |
| **重新生成** | 每次修改后端服务都需运行 `wails3 generate bindings` |

---

## 更新绑定

修改后端 Go 服务方法后，需重新生成绑定：

```bash
# 在项目根目录执行
wails3 generate bindings
```

或者在 Windows 环境下：

```powershell
./build.bat
```

> 💡 `build.bat` 已包含绑定生成步骤，会自动执行 `wails3 generate bindings`

---

## 可取消的请求

所有绑定方法返回的 Promise 都带有 `cancel()` 方法：

```typescript
import { useCancellable } from "../composables/useCancellable";
import { DeviceAPI } from "../services/api";

// 使用 Composable 自动管理取消
const { execute, loading, cancel } = useCancellable(DeviceAPI.listDevices);

// 执行请求
execute();

// 手动取消
cancel();
```

---

## 事件系统

后端通过事件向前端发送消息：

```typescript
import { useEngineEvents } from "../composables/useEngineEvents";
import type { DeviceEvent, SuspendRequiredEvent } from "../types/events";

useEngineEvents({
  onFinished: () => console.log("引擎执行完成"),
  onDeviceEvent: (data: DeviceEvent) => console.log(data.IP, data.Message),
  onSuspend: (data: SuspendRequiredEvent) => console.log("挂起请求", data.ip),
});
```

### 事件列表

| 事件名称                  | 数据类型             | 描述         |
| ------------------------- | -------------------- | ------------ |
| `engine:finished`         | void                 | 引擎执行完成 |
| `device:event`            | DeviceEvent          | 设备执行事件 |
| `engine:suspend_required` | SuspendRequiredEvent | 挂起请求     |

---

## 注意事项

1. **类型安全**: 推荐使用 `frontend/src/services/api.ts` 中导出的命名空间 API，提供更好的类型提示和代码组织
2. **向后兼容**: 旧的函数式导出已标记为 `@deprecated`，将在 v2.0 版本移除
3. **自动生成**: 不要直接修改 `bindings` 目录下的任何文件，所有修改都会在重新生成时被覆盖
4. **命令名称**: 正确命令是 `wails3 generate bindings`（注意是 `wails3` 而非 `wails`）
