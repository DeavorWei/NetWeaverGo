# NetWeaverGo 项目架构说明书

## 1. 项目概述

**NetWeaverGo** 是一款基于 Go 语言开发的并发网络自动化编排与配置集散工具。专为网络工程师设计，支持批量管理网络设备（交换机、路由器），提供大规模并发命令执行、配置备份、配置生成以及智能异常干预功能。

### 1.1 核心特性

| 特性           | 描述                                                     |
| -------------- | -------------------------------------------------------- |
| **GUI 模式**   | 基于 Wails v3 的现代化桌面应用，支持 Windows/macOS/Linux |
| **智能自动化** | 全自动终端交互、智能翻页检测、提示符识别                 |
| **高并发控制** | Worker Pool 模型 + 令牌桶限流，可配置并发数（默认32）    |
| **异常干预**   | 单设备级挂起、用户决策（Continue/Abort）                 |
| **配置备份**   | 自动解析 startup 配置、SFTP 安全下载                     |
| **配置生成**   | ConfigForge 配置模板引擎，支持变量展开与语法糖           |
| **任务管理**   | 支持命令组与设备组的灵活绑定，两种任务模式               |
| **执行历史**   | 完整的执行记录与日志追溯                                 |

### 1.2 技术栈

| 层级         | 技术                                     |
| ------------ | ---------------------------------------- |
| **后端**     | Go 1.21+ / Wails v3                      |
| **前端**     | Vue 3 + TypeScript + Vite + Tailwind CSS |
| **状态管理** | Pinia                                    |
| **数据库**   | SQLite (GORM)                            |
| **通信协议** | SSH/SFTP (golang.org/x/crypto/ssh)       |
| **构建工具** | Wails v3 (桌面应用打包)                  |

---

## 2. 系统架构

### 2.1 整体架构图

```
┌─────────────────────────────────────────────────────────────────────────┐
│                          NetWeaverGo 应用                                │
├─────────────────────────────────────────────────────────────────────────┤
│  ┌────────────────────────────────────────────────────────────────────┐ │
│  │                      前端层 (Frontend - Vue 3)                      │ │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐  │ │
│  │  │Dashboard │ │ Devices  │ │ Commands │ │  Tasks   │ │Execution │  │ │
│  │  │   .vue   │ │   .vue   │ │   .vue   │ │   .vue   │ │   .vue   │  │ │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘ └──────────┘  │ │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐               │ │
│  │  │Settings  │ │NetCalc   │ │Protocol  │ │ConfigForge│              │ │
│  │  │   .vue   │ │   .vue   │ │  Ref.vue │ │   .vue   │               │ │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘               │ │
│  │                     ↓ Wails Bindings (Events/Call) ↓               │ │
│  └────────────────────────────────────────────────────────────────────┘ │
│                                    ↕                                     │
│  ┌────────────────────────────────────────────────────────────────────┐ │
│  │                    服务层 (UI Services - Wails)                     │ │
│  │  ┌──────────────────────────────────────────────────────────────┐  │ │
│  │  │ EngineService │ DeviceService │ CommandGroupService │ ...    │  │ │
│  │  │ (引擎控制)     │ (设备管理)    │ (命令组管理)          │        │  │ │
│  │  └──────────────────────────────────────────────────────────────┘  │ │
│  └────────────────────────────────────────────────────────────────────┘ │
│                                    ↕                                     │
│  ┌────────────────────────────────────────────────────────────────────┐ │
│  │                    核心引擎层 (Engine)                              │ │
│  │  ┌──────────────┐  ┌──────────────┐  ┌─────────────────────────┐   │ │
│  │  │   Engine     │  │ EventBus     │  │ EngineStateManager      │   │ │
│  │  │(并发调度中心) │  │ (双通道事件)  │  │ (状态机: Idle→Running)   │   │ │
│  │  └──────────────┘  └──────────────┘  └─────────────────────────┘   │ │
│  └────────────────────────────────────────────────────────────────────┘ │
│                                    ↕                                     │
│  ┌────────────────────────────────────────────────────────────────────┐ │
│  │                    执行层 (Executor)                                │ │
│  │  ┌──────────────────────────────────────────────────────────────┐  │ │
│  │  │ DeviceExecutor: Connect → ExecutePlaybook → Close             │  │ │
│  │  │ (单设备SSH会话生命周期管理)                                    │  │ │
│  │  └──────────────────────────────────────────────────────────────┘  │ │
│  └────────────────────────────────────────────────────────────────────┘ │
│                                    ↕                                     │
│  ┌────────────────────────────────────────────────────────────────────┐ │
│  │                    通信层 (SSH/SFTP)                                │ │
│  │  ┌──────────────┐  ┌──────────────┐  ┌─────────────────────────┐   │ │
│  │  │  SSHClient   │  │  SFTPClient  │  │ StreamMatcher           │   │ │
│  │  │ (终端交互)   │  │ (文件传输)   │  │ (智能提示符/分页/错误)   │   │ │
│  │  └──────────────┘  └──────────────┘  └─────────────────────────┘   │ │
│  └────────────────────────────────────────────────────────────────────┘ │
│                                    ↕                                     │
│  ┌────────────────────────────────────────────────────────────────────┐ │
│  │                    基础设施层 (Infrastructure)                      │ │
│  │  ┌──────────────┐  ┌──────────────┐  ┌─────────────────────────┐   │ │
│  │  │    Config    │  │    Logger    │  │     Report              │   │ │
│  │  │  (SQLite数据库)│  │  (日志系统)  │  │  (进度追踪/报告生成)    │   │ │
│  │  └──────────────┘  └──────────────┘  └─────────────────────────┘   │ │
│  └────────────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────────────┘
```

### 2.2 数据流向

```
用户操作 → Vue组件 → Wails Bindings → Service层 → Engine → Executor → SSHClient → 网络设备
                                                              ↓
                                                         EventBus
                                                              ↓
                                                    ProgressTracker
                                                              ↓
                                              前端实时更新 (execution:snapshot)
```

---

## 3. 模块详解

### 3.1 入口模块 (`cmd/netweaver/main.go`)

**职责**: 应用程序入口，负责初始化所有子系统并启动GUI

**初始化流程**:

```
main()
├── config.GetPathManager()        // 获取路径管理器
├── pm.EnsureDirectories()         // 确保存储目录存在
├── logger.InitGlobalLogger()      // 初始化日志系统
├── config.InitDB()                // 初始化SQLite数据库
├── config.InitRuntimeManager()    // 初始化运行时配置
└── runGUI()                       // 启动Wails GUI
    ├── 创建服务实例 (8个Service)
    ├── 配置 Wails Application
    ├── 挂载前端资源 (嵌入FS)
    └── 启动窗口 (1440x900, 无边框)
```

**注册的服务**:
| 服务 | 功能 |
|------|------|
| `DeviceService` | 设备资产管理 |
| `CommandGroupService` | 命令组管理 |
| `SettingsService` | 全局设置管理 |
| `EngineService` | 引擎控制（启动/停止/状态） |
| `TaskGroupService` | 任务组管理 |
| `QueryService` | 查询服务 |
| `ForgeService` | 配置模板构建 |
| `ExecutionHistoryService` | 执行历史记录 |

### 3.2 核心引擎模块 (`internal/engine/engine.go`)

**职责**: 全局并发调度中心，管理所有设备的任务分发

**核心结构**:

```go
type Engine struct {
    Devices        []config.DeviceAsset      // 设备资产列表
    Commands       []string                  // 命令队列
    MaxWorkers     int                       // 最大并发数（默认32）
    Settings       *config.GlobalSettings    // 全局配置
    EventBus       chan report.ExecutorEvent // 内部事件总线
    FrontendBus    chan report.ExecutorEvent // 前端事件通道
    stateManager   *EngineStateManager       // 状态机管理器
    ctx            context.Context           // 生命周期控制
    cancel         context.CancelFunc        // 取消函数
    fallback       *RingBuffer               // 后备事件存储（环形缓冲）
    CustomSuspendHandler executor.SuspendHandler // 异常挂起回调
}
```

**核心方法**:
| 方法 | 功能 |
|------|------|
| `NewEngine()` | 初始化引擎，设置并发限制 |
| `Run(ctx)` | 启动 Worker Pool 分发任务 |
| `RunBackup(ctx)` | 启动配置备份专用流程 |
| `worker(ctx, device, wg)` | 单设备任务执行单元 |
| `backupWorker(ctx, device, wg)` | 单设备备份执行单元 |
| `emitEvent(ev)` | 广播事件到双通道 |
| `handleSuspend(ip, log, cmd)` | 异常交互处理 |

**并发控制机制**:

```
令牌桶 (sem chan struct{})
    ↓
┌───────────────────────────────────────┐
│  Worker Pool (MaxWorkers = 32 默认)   │
├───────────────────────────────────────┤
│  Worker1 → Device1 → Executor1 → SSH  │
│  Worker2 → Device2 → Executor2 → SSH  │
│  ...                                  │
│  WorkerN → DeviceN → ExecutorN → SSH  │
└───────────────────────────────────────┘
    ↓ (Jitter 0-500ms 平滑连接压力)
```

**状态机**:

```
Idle → Starting → Running → Closing → Closed
                    ↓
                 Paused (异常挂起)
```

### 3.3 执行器模块 (`internal/executor/executor.go`)

**职责**: 单设备SSH会话生命周期管理，命令步进执行

**核心结构**:

```go
type DeviceExecutor struct {
    IP, Port, Username, Password     // 连接凭证
    Matcher    *matcher.StreamMatcher // 流匹配器
    Client     *sshutil.SSHClient     // SSH客户端
    EventBus   chan report.ExecutorEvent // 事件通道
    OnSuspend  SuspendHandler         // 异常回调
    LogSession *report.DeviceLogSession // 日志会话
}
```

**执行流程**:

```
Connect(ctx, timeout)
    ↓
ExecutePlaybook(ctx, commands, timeout)
├── 等待首个提示符 (currentCmdIndex = -1)
├── 发送预热空行 (currentCmdIndex = -2)
├── 进入命令循环 (currentCmdIndex >= 0)
│   ├── 等待提示符
│   ├── 检测分页符 → 自动发送空格
│   ├── 检测错误 → 触发 SuspendHandler
│   └── 发送下一条命令
└── 全部完成
    ↓
Close()  // 释放资源
```

**错误处理策略**:

```go
type ErrorAction int
const (
    ActionContinue  // 忽略错误，继续下一条
    ActionAbort     // 终止该设备执行
)
```

### 3.4 智能匹配器模块 (`internal/matcher/`)

**职责**: SSH流数据智能分析，支持提示符识别、分页检测、错误匹配

**StreamMatcher 核心功能**:

```go
type StreamMatcher struct {
    Rules             []ErrorRule   // 错误规则集
    Prompts           []string      // 提示符 (>, #, ])
    PaginationPrompts []string      // 分页符 (---- More ----)
}
```

**支持的提示符**:

- `>` - 用户模式
- `#` - 特权模式
- `]` - 部分厂商设备

**错误规则分级**:
| 级别 | 示例规则 | 处理方式 |
|------|----------|----------|
| **Warning** | 配置未改变、Info告警 | 自动跳过，黄色提示 |
| **Critical** | 命令无法识别、语法错误 | 触发 SuspendHandler |

### 3.5 SSH通信模块 (`internal/sshutil/client.go`)

**职责**: 底层SSH连接建立与维护

**两种连接模式**:
| 模式 | 方法 | 用途 |
|------|------|------|
| Shell模式 | `NewSSHClient()` | 命令执行，带PTY终端 |
| Raw模式 | `NewRawSSHClient()` | SFTP文件传输，无PTY |

**加密算法支持** (兼容老旧设备):

| 类型     | 现代安全算法                  | 传统兼容算法           |
| -------- | ----------------------------- | ---------------------- |
| 加密     | AES-GCM, ChaCha20-Poly1305    | AES-CTR, AES-CBC, 3DES |
| 密钥交换 | Curve25519, ECDH-P256/384/521 | DH14-SHA256, DH1-SHA1  |
| 主机密钥 | ED25519, ECDSA, RSA-SHA2      | RSA-SHA1, DSA          |

### 3.6 配置管理模块 (`internal/config/`)

**职责**: 数据持久化与配置管理

**数据模型**:

```go
// 设备资产
type DeviceAsset struct {
    ID       uint     `json:"id" gorm:"primaryKey"`
    IP       string   `json:"ip" gorm:"uniqueIndex"`
    Port     int      `json:"port"`
    Protocol string   `json:"protocol"` // SSH/SNMP/TELNET
    Username string   `json:"username"`
    Password string   `json:"password"`
    Group    string   `json:"group_name"`
    Tags     []string `json:"tags" gorm:"serializer:json"`
}

// 命令组
type CommandGroup struct {
    ID          string   `json:"id"`
    Name        string   `json:"name"`
    Description string   `json:"description"`
    Commands    []string `json:"commands" gorm:"serializer:json"`
    Tags        []string `json:"tags" gorm:"serializer:json"`
}

// 任务组
type TaskGroup struct {
    ID          string `json:"id"`
    Name        string `json:"name"`
    Description string `json:"description"`
    Mode        string `json:"mode"` // group/binding
    Items       []TaskGroupItem `json:"items" gorm:"serializer:json"`
}

// 执行记录
type ExecutionRecord struct {
    ID            string    `json:"id"`
    TaskGroupID   string    `json:"taskGroupId"`
    RunnerSource  string    `json:"runnerSource"`
    Status        string    `json:"status"`
    StartedAt     time.Time `json:"startedAt"`
    FinishedAt    time.Time `json:"finishedAt"`
    Summary       string    `json:"summary"`
}
```

**数据库特性**:

- SQLite + WAL模式（高性能并发）
- 自动迁移表结构
- 索引优化查询
- 旧文件数据自动迁移

### 3.7 UI服务层 (`internal/ui/`)

**职责**: Wails服务绑定，前后端通信桥梁

**EngineService 暴露方法**:
| 方法 | 功能 | 返回 |
|------|------|------|
| `IsRunning()` | 检查引擎状态 | `bool` |
| `StartEngine()` | 启动默认任务 | `error` |
| `StartEngineWithSelection()` | 使用选定设备和命令组启动 | `error` |
| `StartBackup()` | 启动配置备份 | `error` |
| `StopEngine()` | 停止任务 | `error` |
| `GetEngineState()` | 获取引擎状态 | `map[string]interface{}` |
| `GetExecutionSnapshot()` | 获取执行快照 | `*ExecutionSnapshot` |
| `ResolveSuspend(ip, action)` | 解决挂起状态 | `void` |

**事件通信**:

```
后端 → 前端 (Events.Emit)
├── "execution:snapshot"      → 执行状态快照（200ms周期）
├── "engine:suspend_required" → 异常挂起请求
├── "engine:suspend_timeout"  → 挂起超时
└── "engine:finished"         → 任务完成通知

前端 → 后端 (Service.Call)
└── ResolveSuspend(ip, "C"|"A")
```

### 3.8 日志模块 (`internal/logger/logger.go`)

**职责**: 分级日志系统，支持设备独立日志

**日志级别**:
| 级别 | 用途 |
|------|------|
| `Info` | 常规信息 |
| `Warn` | 警告信息 |
| `Error` | 错误信息 |
| `Debug` | 调试信息 |
| `Verbose` | 详细调试信息 |

**日志输出**:

- 控制台输出（带颜色）
- 文件输出 (`logs/app.log`)
- 设备独立日志 (`execution/{taskName}/{IP}/`)

### 3.9 报告模块 (`internal/report/`)

**职责**: 进度追踪与报告生成

**事件类型**:

```go
const (
    EventDeviceStart   EventType = "start"   // 设备开始连接
    EventDeviceCmd     EventType = "cmd"     // 命令执行
    EventDeviceSuccess EventType = "success" // 执行成功
    EventDeviceError   EventType = "error"   // 发生错误
    EventDeviceSkip    EventType = "skip"    // 错误跳过
    EventDeviceAbort   EventType = "abort"   // 执行中断
)
```

**ExecutionSnapshot 结构**（前端直接绑定）:

```go
type ExecutionSnapshot struct {
    TaskName      string            `json:"taskName"`
    TotalDevices  int               `json:"totalDevices"`
    FinishedCount int               `json:"finishedCount"`
    Progress      int               `json:"progress"` // 0-100
    IsRunning     bool              `json:"isRunning"`
    StartTime     string            `json:"startTime"`
    Devices       []DeviceViewState `json:"devices"`
}
```

### 3.10 配置生成器模块 (`internal/forge/`)

**职责**: 配置模板构建，支持变量展开与语法糖

**核心功能**:

- 模板变量替换 `[A]`, `[B]`, ...
- 语法糖展开 `1-10` → `1,2,3,...,10`
- 等差数列推断补全
- 批量配置块生成

**使用示例**:

```
模板: interface GigabitEthernet0/0/[A]
       ip address 192.168.[B].[C] 255.255.255.0

变量: [A] = 1-3
      [B] = 1
      [C] = 1,2,3

生成: interface GigabitEthernet0/0/1
       ip address 192.168.1.1 255.255.255.0
       ---
       interface GigabitEthernet0/0/2
       ip address 192.168.1.2 255.255.255.0
       ---
       interface GigabitEthernet0/0/3
       ip address 192.168.1.3 255.255.255.0
```

---

## 4. 前端架构

### 4.1 技术栈

| 技术       | 版本/配置             |
| ---------- | --------------------- |
| Vue        | 3.x (Composition API) |
| TypeScript | 5.x                   |
| 构建工具   | Vite                  |
| CSS框架    | Tailwind CSS          |
| 状态管理   | Pinia                 |
| 路由       | Vue Router (Hash模式) |

### 4.2 页面结构

| 路由              | 组件          | 功能          |
| ----------------- | ------------- | ------------- |
| `/`               | Dashboard     | 概览仪表盘    |
| `/devices`        | Devices       | 设备资产清单  |
| `/commands`       | Commands      | 命令组管理    |
| `/tasks`          | Tasks         | 任务创建      |
| `/task-execution` | TaskExecution | 任务执行大屏  |
| `/tools/calc`     | NetworkCalc   | IP/子网计算器 |
| `/tools/protocol` | ProtocolRef   | 协议端口速查  |
| `/tools/config`   | ConfigForge   | 配置生成器    |
| `/settings`       | Settings      | 系统设置      |

### 4.3 状态管理 (Pinia Stores)

**engineStore** - 引擎状态管理:

```typescript
// 状态
executionSnapshot: ExecutionSnapshot | null; // 执行快照
suspendSessions: Record<string, SuspendSessionState>; // 挂起会话
backupState: BackupState; // 备份状态

// 方法
initListeners(); // 初始化事件监听
syncExecutionState(); // 同步执行状态
stopEngine(); // 停止引擎
resolveSuspend(ip, action); // 解决挂起
```

### 4.4 组件库

**通用组件** (`components/common/`):

- `TitleBar.vue` - 自定义标题栏
- `ThemeSwitch.vue` - 主题切换
- `GlobalToast.vue` - 全局通知
- `HelpTip.vue` - 帮助提示

**任务组件** (`components/task/`):

- `DeviceSelector.vue` - 设备选择器
- `CommandEditor.vue` - 命令编辑器
- `VirtualLogTerminal.vue` - 虚拟日志终端
- `TaskDetailModal.vue` - 任务详情弹窗
- `TaskEditModal.vue` - 任务编辑弹窗
- `ExecutionHistoryDrawer.vue` - 执行历史抽屉

**配置生成组件** (`components/forge/`):

- `TemplateEditor.vue` - 模板编辑器
- `VariablesPanel.vue` - 变量面板
- `OutputPreview.vue` - 输出预览
- `SendCommandModal.vue` - 发送命令弹窗
- `SendTaskModal.vue` - 发送任务弹窗

### 4.5 主题系统

- 默认深色模式
- 支持浅色/深色切换
- Tailwind CSS 自定义主题色
- CSS变量驱动

---

## 5. 目录结构

```
NetWeaverGo/
├── cmd/
│   └── netweaver/
│       └── main.go              # 应用入口
├── internal/                    # 内部模块
│   ├── config/
│   │   ├── config.go            # 设备资产管理
│   │   ├── db.go                # 数据库初始化
│   │   ├── settings.go          # 全局设置
│   │   ├── runtime_config.go    # 运行时配置
│   │   ├── command_group.go     # 命令组管理
│   │   ├── task_group.go        # 任务组管理
│   │   ├── execution_record.go  # 执行记录
│   │   └── paths.go             # 路径管理
│   ├── engine/
│   │   ├── engine.go            # 并发调度引擎
│   │   ├── engine_state.go      # 状态机
│   │   ├── ring_buffer.go       # 环形缓冲区
│   │   └── global_state.go      # 全局状态
│   ├── executor/
│   │   ├── executor.go          # 设备执行器
│   │   ├── errors.go            # 错误定义
│   │   └── error_handler.go     # 错误处理
│   ├── matcher/
│   │   ├── matcher.go           # 流匹配器
│   │   └── rules.go             # 错误规则
│   ├── sshutil/
│   │   ├── client.go            # SSH客户端
│   │   └── presets.go           # 算法预设
│   ├── sftputil/
│   │   └── client.go            # SFTP客户端
│   ├── report/
│   │   ├── event.go             # 事件定义
│   │   ├── collector.go         # 进度追踪器
│   │   └── log_storage.go       # 日志存储
│   ├── forge/
│   │   ├── config_builder.go    # 配置构建器
│   │   └── syntax_expander.go   # 语法展开器
│   ├── logger/
│   │   └── logger.go            # 日志系统
│   └── ui/
│       ├── engine_service.go    # 引擎服务
│       ├── device_service.go    # 设备服务
│       ├── forge_service.go     # 配置构建服务
│       ├── suspend_manager.go   # 挂起管理器
│       └── execution_manager.go # 执行管理器
├── frontend/                    # 前端源码
│   ├── src/
│   │   ├── App.vue              # 根组件
│   │   ├── main.ts              # 入口文件
│   │   ├── router/index.ts      # 路由配置
│   │   ├── stores/              # Pinia Stores
│   │   ├── views/               # 页面视图
│   │   ├── components/          # 组件库
│   │   ├── composables/         # 组合式函数
│   │   ├── services/api.ts      # API封装
│   │   ├── types/               # 类型定义
│   │   └── styles/              # 样式文件
│   └── package.json
├── docs/                        # 文档目录
├── ui.go                        # Wails资源嵌入
├── wails.json                   # Wails配置
├── go.mod
├── go.sum
└── README.md
```

---

## 6. 数据存储

### 6.1 存储位置

程序运行时数据存储在当前目录的 `netWeaverGoData/` 文件夹下：

```
netWeaverGoData/
├── netweaver.db                 # SQLite 数据库
├── logs/
│   └── app.log                  # 系统日志
├── execution/                   # 执行日志
│   └── {taskName}/
│       └── {IP}/
│           ├── summary.log      # 简略日志
│           ├── detail.log       # 详细日志
│           └── raw.log          # 原始SSH流
├── backup/                      # 配置备份
│   └── config/
│       └── {日期}/
│           └── {IP}_{时间}.cfg
└── report/                      # 执行报告
    └── report_{时间}.csv
```

### 6.2 数据库表结构

| 表名                | 用途         |
| ------------------- | ------------ |
| `device_assets`     | 设备资产     |
| `global_settings`   | 全局设置     |
| `command_groups`    | 命令组       |
| `task_groups`       | 任务组       |
| `runtime_settings`  | 运行时配置   |
| `execution_records` | 执行历史记录 |

---

## 7. 任务模式

### 7.1 模式A：命令组 → 设备组

适用于批量执行相同命令集的场景。

```
命令组（如"基础巡检"）→ 设备组（如"核心交换机"）
                    ↓
              所有设备执行相同命令
```

### 7.2 模式B：IP绑定 → 独立命令

适用于每台设备执行不同命令的场景。

```
设备A → 命令集A
设备B → 命令集B
设备C → 命令集C
```

---

## 8. 配置备份流程

```
1. SFTP 连接设备
     ↓
2. 执行 display startup 获取配置文件路径
     ↓
3. 解析 "Next startup saved-configuration file"
     ↓
4. SFTP 下载配置文件
     ↓
5. 保存到 backup/config/{日期}/{IP}_{时间}.cfg
```

**前置条件**:

- 设备需开启 SFTP 服务 (`sftp server enable`)
- 设备需配置下次启动文件

**支持的设备**:

- 华为、华三（H3C）等支持 SFTP 子系统的网络设备

---

## 9. 关键设计模式

### 9.1 Worker Pool 模式

```go
sem := make(chan struct{}, e.MaxWorkers)
for _, device := range devices {
    sem <- struct{}{}  // 获取令牌
    go func(d Device) {
        defer func() { <-sem }()  // 归还令牌
        // 执行任务
    }(device)
}
```

### 9.2 事件总线模式（双通道）

```go
EventBus chan ExecutorEvent     // 内部：ProgressTracker消费
FrontendBus chan ExecutorEvent   // 外部：Wails前端消费

// 广播到双通道
func (e *Engine) emitEvent(ev ExecutorEvent) {
    e.FrontendBus <- ev  // 前端实时更新
    e.EventBus <- ev     // 后端进度追踪
}
```

### 9.3 回调注入模式

```go
type SuspendHandler func(ctx context.Context, ip, log, cmd string) ErrorAction

// GUI模式：前端弹窗决策
ng.CustomSuspendHandler = GetSuspendManager().CreateHandler()
```

### 9.4 状态机模式

```go
type EngineState int
const (
    StateIdle EngineState = iota
    StateStarting
    StateRunning
    StatePaused
    StateClosing
    StateClosed
)

func (m *EngineStateManager) TransitionTo(newState EngineState) error
```

### 9.5 环形缓冲区模式

```go
// O(1) 复杂度的后备事件存储
type RingBuffer struct {
    data  []ExecutorEvent
    head  int
    tail  int
    count int
}
```

---

## 10. 构建与运行

### 10.1 开发环境

```bash
# 安装依赖
cd frontend && npm install

# 开发模式（热重载）
wails3 dev
```

### 10.2 生产构建

```bash
# 构建生产版本
wails3 build

# 或使用项目构建脚本
.\build.bat
```

### 10.3 构建产物

构建产物位于 `dist/` 目录：

- Windows: `NetWeaverGo.exe`

---

## 11. 敏感信息脱敏

引擎内置敏感信息脱敏功能，自动处理以下模式：

- `password xxx cipher ****` → 密码脱敏
- `password xxx plain ****` → 密码脱敏
- `cipher ****` → 密钥脱敏
- `key xxx ****` → 密钥脱敏
- `secret ****` → 密钥脱敏

---

## 12. 总结

NetWeaverGo 采用清晰的分层架构设计:

1. **表现层**: Vue 3 前端 + Wails GUI
2. **服务层**: Wails Service 双向通信
3. **引擎层**: Engine 并发调度 + 状态机
4. **执行层**: Executor 设备交互
5. **通信层**: SSH/SFTP 协议实现
6. **基础层**: Config/Logger/Report 支撑服务

### 核心优势

- **高并发**: Worker Pool + 令牌桶限流
- **智能化**: 正则匹配器自动处理翻页/错误
- **可扩展**: 模块化设计，易于添加新功能
- **兼容性**: 支持多种加密算法，兼容老旧设备
- **现代化**: Vue 3 + TypeScript + Tailwind CSS 前端技术栈
- **数据持久化**: SQLite 数据库，支持自动迁移

### 适用场景

- 数据中心网络设备批量配置
- 企业网络设备日常巡检
- 网络配置备份与审计
- 网络故障批量排查
- 配置模板批量生成

---

## 13. 版本信息

- **版本**: v1.0
- **许可证**: MIT
- **作者**: DeavorWei
- **仓库**: https://github.com/DeavorWei/NetWeaverGo
