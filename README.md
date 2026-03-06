# NetWeaverGo Wails v3 前端集成设计方案

## 1. 架构概览与目标

当前 NetWeaverGo 是一套拥有强大并发调度能力和异常干预能力的 CLI 应用。
引入 Wails v3 后，目标是将核心调度引擎 (`engine`) 与原本的控制台输入输出（`fmt/logger/bufio`）解耦，利用 Wails 的 Events 机制作为全双工的事件总线，利用 Bindings 方法作为前端对引擎的控制接口。

**技术栈建议：**

- **前端:** Vue 3 + TypeScript + Vite (TailwindCSS 或原生 CSS)
- **后端:** Wails v3 (Go)
- **前后端通信:** Wails Bindings 提供 RPC 调用；Wails Events 提供状态广播。

---

## 2. 核心挑战与解决方案

### 2.1 阻塞式的异常干预 ([handleSuspend](file:///d:/Document/GO/NetWeaverGo/internal/engine/engine.go#288-357) 改造)

**现状:** [executor.go](file:///d:/Document/GO/NetWeaverGo/internal/executor/executor.go) 发生严重报错时，调用回调函数 [handleSuspend(ip, logLine, cmd) ErrorAction](file:///d:/Document/GO/NetWeaverGo/internal/engine/engine.go#288-357)，该函数在 [engine.go](file:///d:/Document/GO/NetWeaverGo/internal/engine/engine.go) 中通过获得全局 `promptMu` 互斥锁，阻塞地从标准输入读取用户的动作 (`C/S/A`)。
**Wails 方案:** 必须将标准输入的阻塞改造为“事件阻塞”。

1. 后端维护一个全局（或由引擎持有）的交互状态管理器，如 `suspendChannels := map[string]chan ErrorAction{}`。
2. 当发生异常时，[handleSuspend](file:///d:/Document/GO/NetWeaverGo/internal/engine/engine.go#288-357) 首先向前端发送广播事件 `Emit("engine:suspend_required", SuspendPayload{IP:..., Error:...})`。
3. [handleSuspend](file:///d:/Document/GO/NetWeaverGo/internal/engine/engine.go#288-357) 通过 `<-suspendChannels[ip]` 陷入阻塞等待。
4. 前端展示 Modal 对话框让用户选择，并调用暴露的 Wails 绑定函数 `ResolveSuspend(ip, action)`。
5. 该函数内部寻址到对应的 channel，将 action 送入，解锁挂起的设备 Goroutine。

### 2.2 日志与进度回传 (`ProgressTracker` 与 `EventBus` 改造)

**现状:** [engine.go](file:///d:/Document/GO/NetWeaverGo/internal/engine/engine.go) 构建了 `EventBus chan report.ExecutorEvent`，交由 `report.ProgressTracker` 在终端利用 ANSI 控制符进行刷新渲染。
**Wails 方案:** 

1. 完全废弃基于终端控制符的 `ProgressTracker` （或保留并仅在使用纯 CLI 标志启动时激活）。
2. 在 Wails 环境下，初始化一个桥接 Goroutine 监听 `EventBus`。
3. 将接收到的 `report.ExecutorEvent` 转化为 Wails 事件并发送给前端：`Emit("device:event", event)`。
4. 前端 Vue 接管所有渲染逻辑（进度条更新、终端回显输出），实现每台设备的独立日志面板。

### 2.3 生命周期与入口 ([main.go](file:///d:/Document/GO/NetWeaverGo/cmd/netweaver/main.go) 改造)

**现状:** 解析命令行参数 -> 读取配置文件 -> 启动 Engine -> Wait。
**Wails 方案:**

1. 将核心 CLI 逻辑抽取。
2. [main.go](file:///d:/Document/GO/NetWeaverGo/cmd/netweaver/main.go) 启动时可识别是否带有 `--cli` 等参数，如果没有，则初始化 `wails.Application`。
3. 在 `OnStartup` 钩子中，准备好系统配置 `config.LoadSettings()` 等。
4. 暴露一组 Wails 结构体方法 (如 `AppService`) 供前端调用：
   - `GetAssets()`: 前端拉取资产列表展示。
   - `StartEngine(assets, commands)`: 前端点击“开始”后，触发后端的并行下发。
   - `StartBackup(assets)`: 启动备份模式。
   - `SaveSettings(settings)`: 将页面上的设置保存至 `settings.yaml`。

---

## 3. 具体实施计划

### 第一阶段：准备工作与 Wails v3 脚手架初始化

1. 在项目根目录外使用 Wails v3 CLI 生成 Vue3 模板项目。
2. 将现有的 NetWeaverGo `internal` 目录迁移到新的 Wails 目录架构内。
3. 保留原有的核心逻辑 `executor` 与 `engine` 保持纯 Go 环境不污染前端代码。

### 第二阶段：重构 Engine 交互层

1. **新增 `WailsAdapter`:** 为 [Engine](file:///d:/Document/GO/NetWeaverGo/internal/engine/engine.go#24-40) 编写一层封装。在 Wails 模式下运行 [Engine](file:///d:/Document/GO/NetWeaverGo/internal/engine/engine.go#24-40) 时，不调用原始带 bufio 的 [handleSuspend](file:///d:/Document/GO/NetWeaverGo/internal/engine/engine.go#288-357)。
2. **重写 [handleSuspend](file:///d:/Document/GO/NetWeaverGo/internal/engine/engine.go#288-357) 回调:** 通过注入闭包的方式，由 `WailsAdapter` 提供基于 channel 阻塞的 [handleSuspend](file:///d:/Document/GO/NetWeaverGo/internal/engine/engine.go#288-357)。
3. **暴露 `AppService` Binding:** 实现获取配置、保存配置等方法。前端可以管理 `settings.yaml` 和 `config.txt` 内容，而不再强依赖配置文件读写。

### 第三阶段：前端 UI 开发

1. **仪表盘 (Dashboard):** 展示设备清单 (IP/状态)、全局并发数、执行总览进度条。
2. **任务配置页:** 可以在 UI 上直接编写/粘贴下发的指令流。
3. **执行与实时监控面板:** 发起 `StartEngine`，挂载 `OnEvent("device:event")`，根据发回的数据渲染各设备的实时终端回放，状态显示排队/拉取/成功/异常。
4. **干预弹窗 (Suspend Dialog):** 监听 `engine:suspend_required`，弹窗强制要求用户决策处理方式。

### 第四阶段：整合测试与编译

1. 测试 Wails 环境下的日志文件输出 (`logs/` 目录挂载是否正常)。
2. 测试并行 100+ 设备前端 UI 的性能压力（限制终端回放面板数量以节省内存）。
3. 使用 `wails build` 输出多平台的可执行文件，交付最终产物。