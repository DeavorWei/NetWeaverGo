# NetWeaverGo 项目架构说明书

## 1. 项目概述

**NetWeaverGo** 是一款基于 Go 语言开发的并发网络自动化编排与配置集散工具。专为网络工程师设计，支持批量管理网络设备（交换机、路由器），提供大规模并发命令执行、配置备份以及智能异常干预功能。

### 1.1 核心特性

| 特性           | 描述                                                |
| -------------- | --------------------------------------------------- |
| **双模运行**   | GUI 模式（桌面端运维）与 CLI 模式（服务器后台调度） |
| **智能自动化** | 全自动终端交互、智能翻页、提示符识别                |
| **高并发控制** | 令牌桶限流、Worker Pool 模型、可配置并发数          |
| **异常干预**   | 单设备级挂起、用户决策（Continue/Skip/Abort）       |
| **配置备份**   | 自动解析 startup 配置、SFTP 安全下载                |

### 1.2 技术栈

- **后端**: Go 1.21+ / Wails v3
- **前端**: Vue 3 + TypeScript + Vite + Tailwind CSS
- **通信协议**: SSH/SFTP (golang.org/x/crypto/ssh)
- **构建工具**: Wails v3 (桌面应用打包)

---

## 2. 系统架构

### 2.1 整体架构图

```
┌─────────────────────────────────────────────────────────────────┐
│                        NetWeaverGo 应用                          │
├─────────────────────────────────────────────────────────────────┤
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                    前端层 (Frontend)                        │ │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌───────────┐   │ │
│  │  │Dashboard │  │ Devices  │  │  Tasks   │  │  Router   │   │ │
│  │  │   .vue   │  │   .vue   │  │   .vue   │  │  (Vue)    │   │ │
│  │  └──────────┘  └──────────┘  └──────────┘  └───────────┘   │ │
│  │                     ↓ Wails Bindings ↓                     │ │
│  └────────────────────────────────────────────────────────────┘ │
│                              ↕                                   │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                    桥接层 (UI Service)                      │ │
│  │  ┌─────────────────────────────────────────────────────┐   │ │
│  │  │ AppService: LoadSettings, StartEngine, ResolveSuspend│   │ │
│  │  └─────────────────────────────────────────────────────┘   │ │
│  └────────────────────────────────────────────────────────────┘ │
│                              ↕                                   │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                    核心引擎层 (Engine)                      │ │
│  │  ┌──────────────┐  ┌──────────────┐  ┌─────────────────┐   │ │
│  │  │   Engine     │  │EventBus事件总线│  │ProgressTracker │   │ │
│  │  │(并发调度中心) │  │              │  │  (进度追踪)     │   │ │
│  │  └──────────────┘  └──────────────┘  └─────────────────┘   │ │
│  └────────────────────────────────────────────────────────────┘ │
│                              ↕                                   │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                    执行层 (Executor)                        │ │
│  │  ┌──────────────────────────────────────────────────────┐  │ │
│  │  │ DeviceExecutor: Connect, ExecutePlaybook, Close       │  │ │
│  │  └──────────────────────────────────────────────────────┘  │ │
│  └────────────────────────────────────────────────────────────┘ │
│                              ↕                                   │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                    通信层 (SSH/SFTP)                        │ │
│  │  ┌──────────────┐  ┌──────────────┐  ┌─────────────────┐   │ │
│  │  │  SSHClient   │  │  SFTPClient  │  │ StreamMatcher   │   │ │
│  │  │ (终端交互)   │  │ (文件传输)   │  │  (智能匹配)     │   │ │
│  │  └──────────────┘  └──────────────┘  └─────────────────┘   │ │
│  └────────────────────────────────────────────────────────────┘ │
│                              ↕                                   │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                    基础设施层 (Infrastructure)              │ │
│  │  ┌──────────────┐  ┌──────────────┐  ┌─────────────────┐   │ │
│  │  │    Config    │  │    Logger    │  │     Report      │   │ │
│  │  │  (配置管理)  │  │  (日志系统)  │  │  (报告生成)     │   │ │
│  │  └──────────────┘  └──────────────┘  └─────────────────┘   │ │
│  └────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

### 2.2 数据流向

```
用户操作 → 前端Vue组件 → Wails Bindings → AppService → Engine → Executor → SSHClient → 网络设备
                                                    ↓
                                              EventBus
                                                    ↓
                                              前端实时更新
```

---

## 3. 模块详解

### 3.1 入口模块 (`cmd/netweaver/main.go`)

**职责**: 应用程序入口，负责初始化和模式选择

```
main()
├── logger.InitGlobalLogger()     // 初始化日志系统
├── 解析命令行参数
│   ├── -cli      : CLI模式开关
│   ├── -b        : 备份模式开关
│   ├── -non-interactive : 无人值守模式
│   ├── -debug    : DEBUG日志
│   └── -debugall : 全量DEBUG日志
├── runGUI()                      // Wails GUI模式
│   ├── 创建 AppService
│   ├── 配置 Wails Application
│   ├── 挂载前端资源 (嵌入FS)
│   └── 启动窗口 (1440x900)
└── runCLI()                      // CLI命令行模式
    ├── 加载配置 (settings.yaml)
    ├── 解析资产与命令文件
    ├── 创建 Engine
    └── 执行 Run() / RunBackup()
```

### 3.2 核心引擎模块 (`internal/engine/engine.go`)

**职责**: 全局并发调度中心，管理所有设备的任务分发

**核心结构**:

```go
type Engine struct {
    Devices        []config.DeviceAsset   // 设备资产列表
    Commands       []string               // 命令队列
    MaxWorkers     int                    // 最大并发数
    Settings       *config.GlobalSettings // 全局配置
    EventBus       chan report.ExecutorEvent // 事件总线
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
| `handleSuspend(ip, log, cmd)` | CLI模式异常交互处理 |

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

### 3.3 执行器模块 (`internal/executor/executor.go`)

**职责**: 单设备SSH会话生命周期管理，命令步进执行

**核心结构**:

```go
type DeviceExecutor struct {
    IP, Port, Username, Password  // 连接凭证
    Matcher    *matcher.StreamMatcher   // 流匹配器
    Client     *sshutil.SSHClient       // SSH客户端
    Log        *logger.DeviceOutput     // 设备日志
    EventBus   chan report.ExecutorEvent // 事件通道
    OnSuspend  SuspendHandler           // 异常回调
}
```

**执行流程**:

```
Connect(ctx, timeout)
    ↓
ExecutePlaybook(ctx, commands, timeout)
├── 等待首个提示符 (currentCmdIndex = -1)
├── 发送预检探针 (currentCmdIndex = -2)
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
    ActionSkip      // 跳过当前命令
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
| **Critical** | 命令无法识别、语法错误 | 触发 error_mode 策略 |

**厂商兼容性**:

- Generic (通用)
- Huawei/H3C (华为/华三)
- Cisco (思科)

### 3.5 SSH通信模块 (`internal/sshutil/client.go`)

**职责**: 底层SSH连接建立与维护

**两种连接模式**:
| 模式 | 方法 | 用途 |
|------|------|------|
| Shell模式 | `NewSSHClient()` | 命令执行，带PTY终端 |
| Raw模式 | `NewRawSSHClient()` | SFTP文件传输，无PTY |

**加密算法支持** (兼容老旧设备):

- **现代安全算法**: AES-GCM, ChaCha20-Poly1305, Curve25519
- **传统安全算法**: AES-CTR, DH14-SHA256
- **兼容模式**: AES-CBC, 3DES, DH1-SHA1 (仅用于老旧设备)

### 3.6 配置管理模块 (`internal/config/`)

**职责**: 配置文件解析与生成

**配置文件结构**:

```
├── inventory.csv    # 设备资产清单
│   └── IP, Port, Username, Password
├── config.txt       # 命令集
│   └── 每行一条命令，支持 # 注释
└── settings.yaml    # 全局参数
    ├── max_workers: 32
    ├── connect_timeout: "10s"
    ├── command_timeout: "30s"
    ├── output_dir: "output"
    ├── log_dir: "logs"
    └── error_mode: "pause" | "skip" | "abort"
```

**DeviceAsset 结构**:

```go
type DeviceAsset struct {
    IP       string
    Port     int
    Username string
    Password string
}
```

### 3.7 UI桥接层 (`internal/ui/app.go`)

**职责**: Wails服务绑定，前后端通信桥梁

**AppService 暴露方法**:
| 方法 | 功能 | 返回 |
|------|------|------|
| `LoadSettings()` | 加载全局配置 | `GlobalSettings` |
| `EnsureConfig()` | 检查配置文件 | `devices, commands, missingFiles` |
| `StartEngineWails()` | 启动任务执行 | `error` |
| `StartBackupWails()` | 启动备份任务 | `error` |
| `ResolveSuspend(ip, action)` | 解决挂起状态 | `void` |

**事件通信**:

```
后端 → 前端 (Emit)
├── "device:event"      → 设备执行事件
├── "engine:suspend_required" → 异常挂起请求
└── "engine:finished"   → 任务完成通知

前端 → 后端 (Call)
└── ResolveSuspend(ip, "C"|"S"|"A")
```

### 3.8 日志模块 (`internal/logger/logger.go`)

**职责**: 分级日志系统，支持设备独立日志

**日志级别**:

- `Info` - 常规信息
- `Warn` - 警告信息
- `Error` - 错误信息
- `Debug` - 调试信息 (需启用)
- `DebugAll` - 全量调试 (需启用)

**日志输出**:

- 控制台输出 (带颜色)
- 文件输出 (`logs/app.log`)
- 设备独立日志 (`output/{IP}.log`)

### 3.9 报告模块 (`internal/report/`)

**职责**: 进度追踪与报告生成

**事件类型**:

```go
const (
    EventDeviceStart   // 设备开始连接
    EventDeviceCmd     // 命令执行
    EventDeviceSuccess // 执行成功
    EventDeviceError   // 发生错误
    EventDeviceSkip    // 错误跳过
    EventDeviceAbort   // 执行中断
)
```

**ProgressTracker 功能**:

- 实时进度大盘渲染 (CLI模式)
- 设备状态追踪
- CSV报告导出 (UTF-8 BOM)

---

## 4. 前端架构

### 4.1 技术栈

| 技术       | 版本/配置             |
| ---------- | --------------------- |
| Vue        | 3.x (Composition API) |
| TypeScript | 5.x                   |
| 构建工具   | Vite                  |
| CSS框架    | Tailwind CSS          |
| 路由       | Vue Router (Hash模式) |

### 4.2 页面结构

```
frontend/
├── index.html           # 入口HTML
├── src/
│   ├── main.ts          # 应用入口
│   ├── App.vue          # 根组件 (侧边栏布局)
│   ├── style.css        # 全局样式
│   ├── router/
│   │   └── index.ts     # 路由配置
│   ├── views/
│   │   ├── Dashboard.vue # 概览仪表盘
│   │   ├── Devices.vue   # 设备资产页面
│   │   └── Tasks.vue     # 任务执行大屏
│   └── bindings/         # Wails绑定 (自动生成)
└── package.json
```

### 4.3 路由配置

| 路径       | 组件      | 功能         |
| ---------- | --------- | ------------ |
| `/`        | Dashboard | 概览仪表盘   |
| `/devices` | Devices   | 设备资产清单 |
| `/tasks`   | Tasks     | 任务执行大屏 |

### 4.4 主题系统

- 默认深色模式
- 支持浅色/深色切换
- Tailwind CSS 自定义主题色

---

## 5. 目录结构

```
NetWeaverGo/
├── cmd/
│   └── netweaver/
│       └── main.go              # 应用入口
├── internal/                    # 内部模块
│   ├── config/
│   │   ├── config.go            # 配置解析
│   │   └── settings.go          # 全局设置
│   ├── engine/
│   │   └── engine.go            # 并发调度引擎
│   ├── executor/
│   │   └── executor.go          # 设备执行器
│   ├── logger/
│   │   └── logger.go            # 日志系统
│   ├── matcher/
│   │   ├── matcher.go           # 流匹配器
│   │   └── rules.go             # 错误规则
│   ├── report/
│   │   ├── event.go             # 事件定义
│   │   └── collector.go         # 进度追踪
│   ├── sftputil/
│   │   └── client.go            # SFTP客户端
│   ├── sshutil/
│   │   └── client.go            # SSH客户端
│   └── ui/
│       └── app.go               # Wails服务绑定
├── frontend/                    # 前端源码
│   ├── src/
│   │   ├── App.vue
│   │   ├── main.ts
│   │   ├── router/
│   │   └── views/
│   └── package.json
├── ui.go                        # Wails资源嵌入
├── go.mod
├── go.sum
└── README.md
```

---

## 6. 运行时配置文件

程序运行时会在当前目录生成/读取以下配置文件:

| 文件            | 格式 | 用途         |
| --------------- | ---- | ------------ |
| `inventory.csv` | CSV  | 设备资产清单 |
| `config.txt`    | TXT  | 待下发命令集 |
| `settings.yaml` | YAML | 全局运行参数 |

输出文件:
| 目录/文件 | 用途 |
|-----------|------|
| `output/` | 设备回显日志 |
| `logs/app.log` | 系统运行日志 |
| `confBakup/` | 配置备份文件 |
| `report_*.csv` | 执行结果报告 |

---

## 7. 关键设计模式

### 7.1 Worker Pool 模式

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

### 7.2 事件总线模式

```go
EventBus chan ExecutorEvent
// 生产者: Executor 发送事件
// 消费者: ProgressTracker 监听并渲染
// 桥接: AppService 转发给前端
```

### 7.3 回调注入模式

```go
type SuspendHandler func(ip, log, cmd string) ErrorAction
// CLI模式: handleSuspend (控制台交互)
// GUI模式: WailsSuspendHandler (前端弹窗)
```

### 7.4 流式处理模式

```go
outReader := io.TeeReader(e.Client.Stdout, e.Log)
// SSH输出流 → 设备日志文件
//            → Matcher 分析
```

---

## 8. 命令行参数

| 参数               | 简写  | 说明                                   |
| ------------------ | ----- | -------------------------------------- |
| `-cli`             |       | 以纯命令行免UI模式运行                 |
| `-b`               |       | 启动备份模式，自动下载交换机配置       |
| `-non-interactive` | `-ni` | 无人值守模式，自动执行 error_mode 策略 |
| `-debug`           |       | 启用 DEBUG 级别日志                    |
| `-debugall`        |       | 启用全量详细 DEBUG 日志                |

---

## 9. 总结

NetWeaverGo 采用清晰的分层架构设计:

1. **表现层**: Vue 3 前端 + Wails GUI
2. **桥接层**: AppService 双向通信
3. **引擎层**: Engine 并发调度
4. **执行层**: Executor 设备交互
5. **通信层**: SSH/SFTP 协议实现
6. **基础层**: Config/Logger/Report 支撑服务

### 核心优势

- **高并发**: Worker Pool + 令牌桶限流
- **智能化**: 正则匹配器自动处理翻页/错误
- **可扩展**: 模块化设计，易于添加新功能
- **兼容性**: 支持多种加密算法，兼容老旧设备
- **双模式**: GUI/CLI 两种运行模式适应不同场景

### 适用场景

- 数据中心网络设备批量配置
- 企业网络设备日常巡检
- 网络配置备份与审计
- 网络故障批量排查
