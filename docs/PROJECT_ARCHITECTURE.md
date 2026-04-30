# NetWeaverGo 项目架构说明书

> **版本**: v1.0  
> **生成日期**: 2026-04-30  
> **项目定位**: 面向网络工程师的桌面级网络自动化编排与配置集散引擎

---

## 目录

- [1. 项目概览](#1-项目概览)
- [2. 技术栈](#2-技术栈)
- [3. 顶层目录结构](#3-顶层目录结构)
- [4. 后端架构（Go）](#4-后端架构go)
  - [4.1 入口层 cmd/](#41-入口层-cmd)
  - [4.2 核心业务层 internal/](#42-核心业务层-internal)
  - [4.3 数据模型层 internal/models/](#43-数据模型层-internalmodels)
  - [4.4 数据访问层 internal/repository/](#44-数据访问层-internalrepository)
  - [4.5 配置管理层 internal/config/](#45-配置管理层-internalconfig)
  - [4.6 统一任务执行运行时 internal/taskexec/](#46-统一任务执行运行时-internaltaskexec)
  - [4.7 命令执行引擎 internal/executor/](#47-命令执行引擎-internalexecutor)
  - [4.8 SSH 通信层 internal/sshutil/](#48-ssh-通信层-internalsshutil)
  - [4.9 终端仿真层 internal/terminal/](#49-终端仿真层-internalterminal)
  - [4.10 流匹配器 internal/matcher/](#410-流匹配器-internalmatcher)
  - [4.11 CLI 解析器 internal/parser/](#411-cli-解析器-internalparser)
  - [4.12 拓扑构建 internal/taskexec/topology_builder.go](#412-拓扑构建-internaltaskextopology_buildergo)
  - [4.13 规划比对 internal/plancompare/](#413-规划比对-internalplancompare)
  - [4.14 配置生成器 internal/forge/](#414-配置生成器-internalforge)
  - [4.15 ICMP 引擎 internal/icmp/](#415-icmp-引擎-internalicmp)
  - [4.16 文件服务器 internal/fileserver/](#416-文件服务器-internalfileserver)
  - [4.17 日志系统 internal/logger/](#417-日志系统-internallogger)
  - [4.18 报告系统 internal/report/](#418-报告系统-internalreport)
  - [4.19 UI 服务层 internal/ui/](#419-ui-服务层-internalui)
- [5. 前端架构（Vue 3 + TypeScript）](#5-前端架构vue-3--typescript)
  - [5.1 技术选型](#51-技术选型)
  - [5.2 目录结构](#52-目录结构)
  - [5.3 路由设计](#53-路由设计)
  - [5.4 状态管理](#54-状态管理)
  - [5.5 前后端通信机制](#55-前后端通信机制)
  - [5.6 组件体系](#56-组件体系)
  - [5.7 样式架构](#57-样式架构)
- [6. 数据库设计](#6-数据库设计)
  - [6.1 数据库选型](#61-数据库选型)
  - [6.2 核心表结构](#62-核心表结构)
- [7. 构建与部署](#7-构建与部署)
  - [7.1 构建流程](#71-构建流程)
  - [7.2 嵌入式前端资源](#72-嵌入式前端资源)
- [8. 架构设计原则](#8-架构设计原则)
- [9. 数据流全景图](#9-数据流全景图)
- [10. 模块依赖关系](#10-模块依赖关系)

---

## 1. 项目概览

**NetWeaverGo** 是一款基于 Go 语言开发的高性能网络自动化编排与配置集散工具。专为网络工程师设计，支持批量管理网络设备（交换机、路由器），提供大规模并发命令执行、配置备份、配置生成、拓扑发现以及智能异常干预功能。

### 核心能力矩阵

| 能力域 | 功能描述 |
|--------|----------|
| **设备管理** | 设备资产 CRUD、分组管理、设备画像（厂商定制）、批量导入导出 |
| **并发执行** | Worker Pool 并发模型、令牌桶限流、可配置并发数、连接抖动控制 |
| **命令编排** | 命令组定义与管理、标签分类、命令序列配置、任务组灵活绑定 |
| **智能终端** | 自动翻页检测、提示符智能识别、ANSI 转义序列处理、多厂商适配 |
| **拓扑发现** | 基于 LLDP 自动发现、接口信息解析、Cytoscape.js 可视化、离线重放 |
| **配置生成** | ConfigForge 模板引擎、变量展开与语法糖、范围展开、批量配置生成 |
| **规划比对** | 差异报告生成、HTML/Excel 导出、配置变更追踪、历史记录对比 |
| **文件服务器** | 内置 SFTP/FTP/TFTP/HTTP 服务器 |
| **网络工具** | 批量 Ping 探测、网络计算器、协议参考手册、执行历史追溯 |
| **异常处理** | 单设备级挂起机制、用户决策（Continue/Abort）、超时自动处理 |

---

## 2. 技术栈

### 后端

| 技术 | 版本 | 用途 |
|------|------|------|
| **Go** | >= 1.26 | 主语言 |
| **Wails v3** | v3.0.0-alpha.78 | 桌面应用框架（Go ↔ WebView 桥接） |
| **GORM** | v1.31.1 | ORM 框架 |
| **SQLite** (glebarez/sqlite) | v1.11.0 | 嵌入式数据库 |
| **golang.org/x/crypto** | v0.50.0 | SSH 客户端实现 |
| **github.com/pkg/sftp** | v1.13.10 | SFTP 文件传输 |
| **github.com/xuri/excelize** | v2.9.0 | Excel 文件读写 |
| **github.com/google/uuid** | v1.6.0 | UUID 生成 |

### 前端

| 技术 | 版本 | 用途 |
|------|------|------|
| **Vue.js** | 3.5.x | UI 框架 |
| **TypeScript** | 5.9.x | 类型安全 |
| **Vite** | 7.3.x | 构建工具 |
| **Pinia** | 3.0.x | 状态管理 |
| **Vue Router** | 4.6.x | 路由管理 |
| **Tailwind CSS** | 4.2.x | 原子化 CSS 框架 |
| **Cytoscape.js** | 3.33.x | 拓扑图可视化引擎 |
| **@wailsio/runtime** | 3.0.x | Wails 前端运行时 |

---

## 3. 顶层目录结构

```
NetWeaverGo/
├── cmd/                          # 应用入口
│   └── netweaver/
│       ├── main.go               # 程序主入口
│       └── rsrc.syso             # Windows 资源文件（图标）
├── internal/                     # 核心业务代码（Go 内部包，不可外部导入）
│   ├── config/                   # 配置管理（路径、数据库、设置、设备画像）
│   ├── executor/                 # 命令执行引擎（SSH 会话管理、命令步进）
│   ├── fileserver/               # 内置文件服务器（SFTP/FTP/TFTP/HTTP）
│   ├── forge/                    # ConfigForge 配置生成器
│   ├── icmp/                     # ICMP 批量 Ping 引擎
│   ├── logger/                   # 全局日志系统
│   ├── matcher/                  # 流匹配器（提示符/错误检测）
│   ├── models/                   # 数据库模型定义（GORM）
│   ├── normalize/                # 接口名规范化
│   ├── parser/                   # CLI 输出解析器（模板驱动）
│   ├── plancompare/              # 规划比对服务
│   ├── report/                   # 执行报告与日志采集
│   ├── repository/               # 数据访问层（Repository 模式）
│   ├── sftputil/                 # SFTP 工具客户端
│   ├── sshutil/                  # SSH 连接客户端
│   ├── taskexec/                 # 统一任务执行运行时（核心编排引擎）
│   ├── terminal/                 # 终端仿真器（ANSI 解析、行缓冲）
│   ├── ui/                       # UI 服务层（Wails 暴露给前端的服务）
│   └── utils/                    # 通用工具函数
├── frontend/                     # 前端源码（Vue 3 + TypeScript）
│   ├── src/
│   │   ├── bindings/             # Wails 自动生成的 Go→TS 类型绑定
│   │   ├── components/           # 可复用组件
│   │   ├── composables/          # Vue 组合式函数
│   │   ├── router/               # 路由配置
│   │   ├── services/             # API 服务层
│   │   ├── stores/               # Pinia 状态管理
│   │   ├── styles/               # 全局样式（Tailwind + 主题）
│   │   ├── types/                # TypeScript 类型定义
│   │   ├── utils/                # 前端工具函数
│   │   └── views/                # 页面视图组件
│   ├── package.json
│   ├── vite.config.ts
│   └── tsconfig.json
├── testdata/                     # 测试数据（Golden 测试、回归测试）
├── docs/                         # 项目文档
├── Dist/                         # 运行时数据目录（数据库、日志、报告）
├── build.bat                     # Windows 构建脚本
├── go.mod                        # Go 模块定义
├── go.sum                        # Go 依赖校验
└── README.md                     # 项目说明
```

---

## 4. 后端架构（Go）

### 4.1 入口层 cmd/

入口文件 [`cmd/netweaver/main.go`](cmd/netweaver/main.go) 负责应用启动的完整生命周期：

```
main()
  ├── config.GetPathManager()        # 初始化路径管理器（单例）
  ├── pm.EnsureDirectories()         # 确保数据目录存在
  ├── logger.InitGlobalLogger()      # 初始化全局日志
  ├── config.InitDB()                # 初始化 SQLite 数据库 + 自动迁移
  ├── taskexec.AutoMigrate()         # 统一运行时数据库迁移
  ├── config.InitRuntimeManager()    # 初始化运行时配置管理器
  └── runGUI()                       # 启动 Wails GUI
        ├── parser.NewParserManager().Bootstrap()   # 加载内置解析模板
        ├── taskexec.NewTaskExecutionService()      # 创建统一任务执行服务
        ├── ui.New*Service() × 12                   # 创建各 UI 服务实例
        ├── application.New()                       # 创建 Wails 应用
        │   └── Services: [12个 Wails Service]      # 注册服务到 Wails 桥接
        └── app.Run()                               # 启动主循环
```

### 4.2 核心业务层 internal/

后端采用**分层架构**，各层职责清晰：

```
┌─────────────────────────────────────────────────────┐
│                    UI 服务层 (internal/ui/)           │
│  DeviceService / TaskGroupService / SettingsService  │
│  TopologyCommandService / PlanCompareService / ...   │
├─────────────────────────────────────────────────────┤
│              统一任务执行运行时 (internal/taskexec/)    │
│  TaskExecutionService → RuntimeManager               │
│  CompilerRegistry → StageExecutor                    │
│  EventBus → SnapshotHub                              │
├─────────────────────────────────────────────────────┤
│              命令执行引擎 (internal/executor/)         │
│  DeviceExecutor → StreamEngine → SessionReducer      │
├─────────────────────────────────────────────────────┤
│              基础设施层                                │
│  sshutil / terminal / matcher / parser / report      │
├─────────────────────────────────────────────────────┤
│              数据层                                    │
│  repository (接口) → config (DB/GORM) → models       │
└─────────────────────────────────────────────────────┘
```

### 4.3 数据模型层 internal/models/

[`internal/models/models.go`](internal/models/models.go) 定义了所有数据库实体，采用 GORM 标签驱动映射：

| 模型 | 表名 | 说明 |
|------|------|------|
| [`DeviceAsset`](internal/models/models.go:12) | `device_assets` | 设备资产（IP、端口、凭证、厂商、分组、标签） |
| [`GlobalSettings`](internal/models/models.go:113) | `global_settings` | 全局运行参数（超时、SSH 算法、主题） |
| [`CommandGroup`](internal/models/models.go:145) | `command_groups` | 命令组（名称、命令列表、标签） |
| [`TaskGroup`](internal/models/models.go:165) | `task_groups` | 任务组（设备组×命令组绑定、执行模式、备份配置） |
| [`RuntimeSetting`](internal/models/models.go:201) | `runtime_settings` | 运行时 KV 配置 |
| [`FileServerConfig`](internal/models/models.go:232) | `file_server_configs` | 文件服务器配置 |
| [`TopologyVendorFieldCommand`](internal/models/topology_command.go) | `topology_vendor_field_commands` | 拓扑采集厂商字段命令映射 |
| [`PlanFile`](internal/models/plancompare.go:35) | `plan_files` | 规划文件元数据 |
| [`PlannedLink`](internal/models/plancompare.go:11) | `planned_links` | 规划链路 |
| [`DiffReport`](internal/models/plancompare.go:52) | `diff_reports` | 差异报告 |
| [`DiffItem`](internal/models/plancompare.go:72) | `diff_items` | 差异项 |

**拓扑相关视图模型**（[`internal/models/topology.go`](internal/models/topology.go)）：

| 类型 | 说明 |
|------|------|
| [`GraphNode`](internal/models/topology.go:54) | 拓扑图节点（支持 managed/unmanaged/inferred/unknown 类型） |
| [`GraphEdge`](internal/models/topology.go:73) | 拓扑图边（链路关系） |
| [`TopologyBuildResult`](internal/models/topology.go:24) | 拓扑构建结果统计 |
| [`TopologyGraphView`](internal/models/topology.go:36) | 拓扑图完整视图 |
| [`EdgeEvidence`](internal/models/topology.go:8) | 链路证据（LLDP/FDB/ARP 推断依据） |

### 4.4 数据访问层 internal/repository/

采用 **Repository 模式** 抽象数据访问，[`internal/repository/interfaces.go`](internal/repository/interfaces.go) 定义了核心接口：

- [`DeviceRepository`](internal/repository/interfaces.go:16) — 设备资产 CRUD + 分页查询 + 事务支持
- [`CommandGroupRepository`](internal/repository/interfaces.go:72) — 命令组 CRUD
- [`TaskGroupRepository`](internal/repository/interfaces.go:87) — 任务组 CRUD

实现类 [`device_repository.go`](internal/repository/device_repository.go) 基于 GORM 操作 SQLite。

### 4.5 配置管理层 internal/config/

| 文件 | 职责 |
|------|------|
| [`config.go`](internal/config/config.go) | 协议校验、默认端口映射、密码合并规则 |
| [`db.go`](internal/config/db.go) | SQLite 初始化、连接池配置、自动迁移、索引创建 |
| [`paths.go`](internal/config/paths.go) | [`PathManager`](internal/config/paths.go:26) 单例 — 统一管理所有运行时路径（DB、日志、报告、SSH、拓扑） |
| [`settings.go`](internal/config/settings.go) | 全局设置加载/保存、SSH 算法预设、调试模式应用 |
| [`runtime_config.go`](internal/config/runtime_config.go) | 运行时配置管理器（超时、限制、引擎、拓扑推理参数） |
| [`device_profile.go`](internal/config/device_profile.go) | 设备画像系统（PTY/提示符/分页/初始化/命令规格） |
| [`topology_command.go`](internal/config/topology_command.go) | 拓扑厂商字段命令映射 CRUD |
| [`constants.go`](internal/config/constants.go) | 全局常量定义 |
| [`command_group.go`](internal/config/command_group.go) | 命令组业务逻辑 |
| [`task_group.go`](internal/config/task_group.go) | 任务组业务逻辑 |

**PathManager 路径管理**：

```
StorageRoot (netWeaverGoData/)
├── db/netweaver.db              # SQLite 数据库
├── logs/app/app.log             # 应用日志
├── execution/reports/           # 执行报告 CSV
├── execution/live_logs/         # 实时日志
├── backup/configs/              # 配置备份
├── ssh/known_hosts              # SSH 主机密钥
├── topology/raw/                # 拓扑原始输出
├── topology/export/             # 拓扑导出
└── topology/plan_import/        # 规划文件导入
```

### 4.6 统一任务执行运行时 internal/taskexec/

这是项目的核心编排引擎，[`internal/taskexec/doc.go`](internal/taskexec/doc.go) 详细描述了其五层架构：

```
┌──────────────────────────────────────────────────────────┐
│  1. 任务定义层 (Task Definition Layer)                     │
│     TaskDefinition → NormalTaskConfig / TopologyTaskConfig│
├──────────────────────────────────────────────────────────┤
│  2. 计划编译层 (Plan Compilation Layer)                    │
│     CompilerRegistry                                     │
│     ├── NormalTaskCompiler   → device_command stages      │
│     ├── TopologyTaskCompiler → collect→parse→build stages │
│     └── BackupTaskCompiler   → backup stages              │
├──────────────────────────────────────────────────────────┤
│  3. 统一运行时层 (Unified Runtime Layer)                   │
│     RuntimeManager → RuntimeContext → EventBus            │
│     SnapshotHub (前端快照)                                 │
├──────────────────────────────────────────────────────────┤
│  4. 阶段执行器层 (Stage Executor Layer)                    │
│     DeviceCommandExecutor  (普通命令执行)                   │
│     DeviceCollectExecutor  (拓扑数据采集)                   │
│     ParseExecutor          (CLI 输出解析)                   │
│     TopologyBuildExecutor  (拓扑图构建)                     │
│     BackupExecutor         (配置备份)                       │
├──────────────────────────────────────────────────────────┤
│  5. 数据层 (Data Layer)                                    │
│     TaskRun / TaskRunStage / TaskRunUnit / TaskRunEvent   │
│     TaskArtifact / ExecutionSnapshot / Repository          │
└──────────────────────────────────────────────────────────┘
```

**核心数据模型**（[`internal/taskexec/models.go`](internal/taskexec/models.go)）：

| 模型 | 表名 | 说明 |
|------|------|------|
| [`TaskDefinition`](internal/taskexec/models.go:58) | `task_definitions` | 任务定义（kind: normal/topology） |
| [`TaskRun`](internal/taskexec/models.go:76) | `task_runs` | 任务运行实例（状态机：pending→running→completed/failed/cancelled） |
| [`TaskRunStage`](internal/taskexec/models.go:101) | `task_run_stages` | 阶段运行状态（device_command/device_collect/parse/topology_build） |
| [`TaskRunUnit`](internal/taskexec/models.go:127) | `task_run_units` | 调度单元（设备级粒度） |
| [`TaskRunEvent`](internal/taskexec/models.go:155) | `task_run_events` | 事件流水（全量审计） |
| [`TaskArtifact`](internal/taskexec/models.go:175) | `task_artifacts` | 产物索引（原始输出/解析结果/拓扑图/报告） |

**执行计划模型**：

```
ExecutionPlan
├── RunKind: "normal" | "topology"
├── Name: string
└── Stages: []StagePlan
    ├── Kind: "device_command" | "device_collect" | "parse" | "topology_build"
    ├── Concurrency: int
    └── Units: []UnitPlan
        ├── Target: { Type: "device_ip", Key: "192.168.1.1" }
        ├── Timeout: Duration
        └── Steps: []StepPlan
            ├── Kind: "command" | "parse" | "build"
            └── Command: "display version"
```

**关键文件**：

| 文件 | 职责 |
|------|------|
| [`service.go`](internal/taskexec/service.go) | [`TaskExecutionService`](internal/taskexec/service.go:17) — 服务入口，组装运行时、编译器、执行器 |
| [`runtime.go`](internal/taskexec/runtime.go) | [`RuntimeManager`](internal/taskexec/runtime.go) — 任务生命周期管理、阶段调度、并发控制 |
| [`compiler.go`](internal/taskexec/compiler.go) | [`CompilerRegistry`](internal/taskexec/compiler.go) — 编译器注册表 |
| [`normal_compiler.go`](internal/taskexec/normal_compiler.go) | 普通任务编译器（Mode A/B → device_command 阶段） |
| [`topology_compiler.go`](internal/taskexec/topology_compiler.go) | 拓扑任务编译器（collect→parse→build 三阶段） |
| [`backup_compiler.go`](internal/taskexec/backup_compiler.go) | 备份任务编译器 |
| [`executor_impl.go`](internal/taskexec/executor_impl.go) | 阶段执行器实现（DeviceCommand/Collect/Parse/Build/Backup） |
| [`eventbus.go`](internal/taskexec/eventbus.go) | 事件总线（带缓冲的 channel） |
| [`snapshot.go`](internal/taskexec/snapshot.go) | [`SnapshotHub`](internal/taskexec/snapshot.go) — 执行快照管理（前端实时状态） |
| [`persistence.go`](internal/taskexec/persistence.go) | GORM 持久化 Repository 实现 |
| [`topology_builder.go`](internal/taskexec/topology_builder.go) | 拓扑图构建器（LLDP/FDB/ARP 多源融合） |
| [`topology_builder_phase3.go`](internal/taskexec/topology_builder_phase3.go) | 拓扑构建阶段3（推断节点、冲突检测） |
| [`topology_query.go`](internal/taskexec/topology_query.go) | 拓扑查询服务 |
| [`replay_executor.go`](internal/taskexec/replay_executor.go) | 离线重放执行器 |
| [`orchestration_policy.go`](internal/taskexec/orchestration_policy.go) | 编排策略（错误处理、超时、重试） |
| [`launch_service.go`](internal/taskexec/launch_service.go) | 任务启动服务 |
| [`status.go`](internal/taskexec/status.go) | 状态机定义 |
| [`ids.go`](internal/taskexec/ids.go) | ID 生成策略 |

### 4.7 命令执行引擎 internal/executor/

[`internal/executor/executor.go`](internal/executor/executor.go) 实现了单设备级的 SSH 命令执行生命周期：

```
DeviceExecutor
├── Connect()          # 建立 SSH 长连接 + PTY 配置
├── Initialize()       # 设备初始化（禁用分页、等待提示符）
├── ExecutePlan()      # 按 ExecutionPlan 逐步执行命令
│   ├── StreamEngine   # 流式读取引擎
│   ├── SessionReducer # 会话状态归约器
│   └── ErrorHandler   # 错误处理器（Continue/Abort/Suspend）
└── Close()            # 关闭连接
```

**关键组件**：

| 文件 | 职责 |
|------|------|
| [`executor.go`](internal/executor/executor.go) | [`DeviceExecutor`](internal/executor/executor.go:43) — 设备执行器主类 |
| [`execution_plan.go`](internal/executor/execution_plan.go) | [`ExecutionPlan`](internal/executor/execution_plan.go:28) / [`ExecutionReport`](internal/executor/execution_plan.go:38) — 执行计划与报告 |
| [`stream_engine.go`](internal/executor/stream_engine.go) | 流式读取引擎（字节流→规范化文本） |
| [`session_adapter.go`](internal/executor/session_adapter.go) | 会话适配器 |
| [`session_detector.go`](internal/executor/session_detector.go) | 会话状态检测器 |
| [`session_reducer.go`](internal/executor/session_reducer.go) | 会话状态归约器 |
| [`session_types.go`](internal/executor/session_types.go) | 会话类型定义 |
| [`session_helpers.go`](internal/executor/session_helpers.go) | 会话辅助函数 |
| [`command_context.go`](internal/executor/command_context.go) | 命令上下文 |
| [`error_handler.go`](internal/executor/error_handler.go) | 错误处理策略 |
| [`errors.go`](internal/executor/errors.go) | 错误类型定义 |
| [`initializer.go`](internal/executor/initializer.go) | 设备初始化器 |

**错误处理策略**（[`ErrorAction`](internal/executor/executor.go:20)）：

| 策略 | 行为 |
|------|------|
| `ActionContinue` | 忽略错误，继续发送下一条命令 |
| `ActionAbort` | 立即停止该设备的后续命令 |
| `ActionAbortTimeout` | 挂起超时后自动停止 |

### 4.8 SSH 通信层 internal/sshutil/

[`internal/sshutil/client.go`](internal/sshutil/client.go) 封装了完整的 SSH 客户端：

- [`SSHClient`](internal/sshutil/client.go:26) — SSH 连接管理（Session、Stdin/Stdout/Stderr）
- PTY 配置（终端类型、窗口大小、回显模式）
- 主机密钥校验策略（strict/accept_new/insecure）
- 算法预设（secure/compatible/custom）
- [`presets.go`](internal/sshutil/presets.go) — SSH 算法预设配置

### 4.9 终端仿真层 internal/terminal/

[`internal/terminal/replayer.go`](internal/terminal/replayer.go) 实现了终端重放器，将 SSH 原始字节流转换为规范化逻辑文本：

```
SSH 字节流 → ANSIParser → Token 流 → Replayer → LineEvent 流
                                                    ├── EventLineCommitted
                                                    └── EventLineUpdated
```

| 文件 | 职责 |
|------|------|
| [`ansi.go`](internal/terminal/ansi.go) | ANSI 转义序列解析器 |
| [`line_buffer.go`](internal/terminal/line_buffer.go) | 行缓冲区（支持回车、退格、光标移动） |
| [`event.go`](internal/terminal/event.go) | 行事件定义 |
| [`replayer.go`](internal/terminal/replayer.go) | [`Replayer`](internal/terminal/replayer.go:5) — 终端重放器 |

### 4.10 流匹配器 internal/matcher/

[`internal/matcher/matcher.go`](internal/matcher/matcher.go) 负责实时检测 SSH 输出流中的关键模式：

- [`StreamMatcher`](internal/matcher/matcher.go:26) — 动态流匹配器
- 提示符检测（`>`, `#`, `]` 及正则模式）
- 分页提示符检测（`---- More ----`, `--More--` 等）
- 错误行检测（[`rules.go`](internal/matcher/rules.go) 定义的多厂商错误规则）
- ANSI 转义序列清理

### 4.11 CLI 解析器 internal/parser/

模板驱动的 CLI 输出解析系统：

```
ParserManager (模板管理器)
├── Bootstrap()           # 加载内置模板（huawei/h3c/cisco）
├── GetParser(vendor)     # 获取厂商解析器
└── ReloadVendor(vendor)  # 热重载模板

CompositeParser (组合解析器)
├── RegexParser           # 正则解析器
├── Mapper                # 字段映射器
└── AggregateEngine       # 聚合引擎
```

| 文件 | 职责 |
|------|------|
| [`manager.go`](internal/parser/manager.go) | [`ParserManager`](internal/parser/manager.go:16) — 模板管理器（嵌入式 FS 加载） |
| [`composite_parser.go`](internal/parser/composite_parser.go) | 组合解析器 |
| [`regex_parser.go`](internal/parser/regex_parser.go) | 正则解析器 |
| [`mapper.go`](internal/parser/mapper.go) | 字段映射器 |
| [`aggregate_engine.go`](internal/parser/aggregate_engine.go) | 聚合引擎 |
| [`models.go`](internal/parser/models.go) | 解析器模型定义 |
| [`templates/builtin/`](internal/parser/templates/builtin/) | 内置模板（huawei.json / h3c.json / cisco.json） |

### 4.12 拓扑构建 internal/taskexec/topology_builder.go

拓扑构建采用**多源融合**策略：

```
数据源:
├── LLDP 邻居信息（直接链路）
├── ARP 表（IP-MAC 映射）
├── FDB 表（MAC-端口映射）
└── 接口信息（端口状态）

融合策略:
├── 确认边 (Confirmed)    — 双向 LLDP 一致
├── 半确认边 (SemiConfirmed) — 单向 LLDP
├── 推断边 (Inferred)     — FDB/ARP 推断
└── 冲突边 (Conflict)     — 多源信息不一致
```

### 4.13 规划比对 internal/plancompare/

[`internal/plancompare/service.go`](internal/plancompare/service.go) 提供规划与实际拓扑的差异比对：

- 导入 Excel 规划文件
- 解析规划链路
- 与实际拓扑边进行匹配
- 生成差异报告（missing_link / unexpected_link / interface_mismatch）
- 导出 HTML/Excel 报告

### 4.14 配置生成器 internal/forge/

[`internal/forge/config_builder.go`](internal/forge/config_builder.go) 实现模板驱动的批量配置生成：

```
模板 + 变量 → ConfigBuilder.Build()
  ├── 解析变量值（逗号/换行分隔）
  ├── 展开语法糖（1-10 → 1,2,3,...,10）
  ├── 推断等差数列补全
  ├── 精确变量替换
  └── 返回 BuildResult { Blocks, Total, Warnings }
```

[`syntax_expander.go`](internal/forge/syntax_expander.go) — 范围语法展开器。

### 4.15 ICMP 引擎 internal/icmp/

[`internal/icmp/engine.go`](internal/icmp/engine.go) 实现 Windows 平台的批量 Ping 探测：

- [`BatchPingEngine`](internal/icmp/engine.go:17) — 并发 Ping 引擎
- 可配置并发数（最大 256）、超时、数据包大小、次数
- 进度回调机制
- [`icmp_windows.go`](internal/icmp/icmp_windows.go) — Windows 原生 ICMP 实现

### 4.16 文件服务器 internal/fileserver/

[`internal/fileserver/server.go`](internal/fileserver/server.go) 提供内置的多协议文件服务器：

| 文件 | 协议 | 说明 |
|------|------|------|
| [`sftp_server.go`](internal/fileserver/sftp_server.go) | SFTP | 基于 SSH 的文件传输 |
| [`ftp_server.go`](internal/fileserver/ftp_server.go) | FTP | 传统 FTP 服务器 |
| [`tftp_server.go`](internal/fileserver/tftp_server.go) | TFTP | 简单文件传输 |
| [`web_server.go`](internal/fileserver/web_server.go) | HTTP | Web 文件浏览 |

[`ServerManager`](internal/fileserver/server.go:76) 统一管理所有文件服务器的生命周期。

### 4.17 日志系统 internal/logger/

[`internal/logger/logger.go`](internal/logger/logger.go) 提供多级日志系统：

- 日志级别：Error / Warn / Info / Debug / Verbose
- 全局日志文件 + 控制台输出
- [`sanitizer.go`](internal/logger/sanitizer.go) — 日志脱敏器（自动清理密码等敏感信息）
- 支持运行时重配置（切换 storageRoot 时）

### 4.18 报告系统 internal/report/

| 文件 | 职责 |
|------|------|
| [`event.go`](internal/report/event.go) | [`ExecutorEvent`](internal/report/event.go:14) — 执行器事件定义（start/cmd/success/error/skip/abort） |
| [`collector.go`](internal/report/collector.go) | 事件收集器 |
| [`summary_logger.go`](internal/report/summary_logger.go) | 摘要日志 |
| [`detail_logger.go`](internal/report/detail_logger.go) | 详细日志 |
| [`raw_logger.go`](internal/report/raw_logger.go) | 原始输出日志 |
| [`journal_logger.go`](internal/report/journal_logger.go) | 流水日志 |
| [`log_storage.go`](internal/report/log_storage.go) | 日志存储管理 |

### 4.19 UI 服务层 internal/ui/

[`internal/ui/`](internal/ui/) 包含所有暴露给前端的 Wails 服务，每个服务对应一个业务域：

| 服务文件 | 说明 |
|----------|------|
| [`device_service.go`](internal/ui/device_service.go) | 设备管理服务（CRUD、批量操作） |
| [`command_group_service.go`](internal/ui/command_group_service.go) | 命令组管理服务 |
| [`task_group_service.go`](internal/ui/task_group_service.go) | 任务组管理服务 |
| [`settings_service.go`](internal/ui/settings_service.go) | 全局设置服务 |
| [`query_service.go`](internal/ui/query_service.go) | 通用查询服务 |
| [`forge_service.go`](internal/ui/forge_service.go) | ConfigForge 服务 |
| [`execution_history_service.go`](internal/ui/execution_history_service.go) | 执行历史服务 |
| [`topology_command_service.go`](internal/ui/topology_command_service.go) | 拓扑命令配置服务 |
| [`plan_compare_service.go`](internal/ui/plan_compare_service.go) | 规划比对服务 |
| [`ping_service.go`](internal/ui/ping_service.go) | 批量 Ping 服务 |
| [`fileserver_service.go`](internal/ui/fileserver_service.go) | 文件服务器服务 |
| [`taskexec_ui_service.go`](internal/ui/taskexec_ui_service.go) | 统一任务执行 UI 服务 |
| [`taskexec_event_bridge.go`](internal/ui/taskexec_event_bridge.go) | 任务执行事件桥接（Go→前端实时推送） |
| [`network_calc_service.go`](internal/ui/network_calc_service.go) | 网络计算器服务 |
| [`parse_template_service.go`](internal/ui/parse_template_service.go) | 解析模板管理服务 |
| [`view_models.go`](internal/ui/view_models.go) | 视图模型定义 |
| [`topology_command_view_models.go`](internal/ui/topology_command_view_models.go) | 拓扑命令视图模型 |

---

## 5. 前端架构（Vue 3 + TypeScript）

### 5.1 技术选型

- **框架**: Vue 3 Composition API + `<script setup>` 语法
- **语言**: TypeScript 5.9（严格模式）
- **构建**: Vite 7.3（HMR 热重载）
- **状态管理**: Pinia 3.0
- **路由**: Vue Router 4.6（Hash 模式）
- **样式**: Tailwind CSS 4.2（原子化 + CSS 变量主题系统）
- **可视化**: Cytoscape.js 3.33（拓扑图引擎）
- **桥接**: @wailsio/runtime（Go↔WebView 双向通信）

### 5.2 目录结构

```
frontend/src/
├── App.vue                    # 根组件（侧边栏 + 标题栏 + 路由视图）
├── main.ts                    # 应用入口（Vue/Pinia/Router 初始化）
├── env.d.ts                   # 环境类型声明
│
├── bindings/                  # Wails 自动生成的 Go→TS 类型绑定
│   └── github.com/NetWeaverGo/core/internal/
│       ├── forge/             # ConfigForge 绑定
│       ├── icmp/              # ICMP 绑定
│       ├── models/            # 数据模型绑定
│       ├── parser/            # 解析器绑定
│       ├── taskexec/          # 任务执行绑定
│       └── ui/                # UI 服务绑定（12个服务）
│
├── components/                # 可复用组件
│   ├── common/                # 通用组件
│   │   ├── TitleBar.vue       # 自定义标题栏（无边框窗口）
│   │   ├── ThemeSwitch.vue    # 主题切换器
│   │   ├── RouteLoading.vue   # 路由加载占位
│   │   ├── GlobalToast.vue    # 全局 Toast 通知
│   │   ├── ConfirmModal.vue   # 确认对话框
│   │   ├── HelpTip.vue        # 帮助提示
│   │   └── DualListSelector/  # 双列表选择器（设备选择）
│   ├── device/                # 设备相关组件
│   │   ├── DeviceTable.vue    # 设备表格
│   │   ├── DeviceEditModal.vue
│   │   ├── DeviceBatchEditModal.vue
│   │   ├── DeviceDeleteConfirm.vue
│   │   └── DeviceSearchBar.vue
│   ├── task/                  # 任务相关组件
│   │   ├── TaskEditModal.vue
│   │   ├── TaskDetailModal.vue
│   │   ├── CommandEditor.vue
│   │   ├── CommandGroupSelector.vue
│   │   ├── DeviceSelectorModal.vue
│   │   ├── DeviceExecutionProgressList.vue
│   │   ├── ExecutionHistoryDrawer.vue
│   │   ├── ExecutionRecordDetail.vue
│   │   ├── StageProgress.vue
│   │   ├── VirtualLogTerminal.vue
│   │   └── FileOperationButtons.vue
│   ├── topology/              # 拓扑相关组件
│   │   ├── TopologyGraph.vue  # Cytoscape.js 拓扑图
│   │   ├── TopologyDeviceDetailModal.vue
│   │   ├── TopologyEdgeDetailModal.vue
│   │   ├── DecisionTracePanel.vue
│   │   ├── RawFilePreview.vue
│   │   └── ReplayDialog.vue
│   ├── forge/                 # ConfigForge 组件
│   │   ├── TemplateEditor.vue
│   │   ├── VariablesPanel.vue
│   │   ├── OutputPreview.vue
│   │   ├── SendCommandModal.vue
│   │   ├── SendTaskModal.vue
│   │   ├── SyntaxHelpModal.vue
│   │   └── UsageHelpModal.vue
│   ├── network/               # 网络工具组件
│   │   ├── IPv4Calc.vue
│   │   └── IPv6Calc.vue
│   ├── settings/              # 设置组件
│   │   └── RuntimeConfigPanel.vue
│   └── tools/                 # 工具组件
│       └── PingSettingsModal.vue
│
├── composables/               # Vue 组合式函数（可复用逻辑）
│   ├── useTheme.ts            # 主题管理
│   ├── useDeviceForm.ts       # 设备表单逻辑
│   ├── useDeviceSearch.ts     # 设备搜索
│   ├── useDeviceSelection.ts  # 设备选择
│   ├── useIPBinding.ts        # IP 绑定
│   ├── useConfigBuilder.ts    # 配置构建器
│   ├── useColumnResize.ts     # 列宽调整
│   ├── useCancellable.ts      # 可取消操作
│   └── useTopologyReplay.ts   # 拓扑重放
│
├── router/
│   └── index.ts               # 路由配置（Hash 模式）
│
├── services/
│   └── api.ts                 # 统一 API 导出（命名空间模式）
│
├── stores/
│   └── taskexecStore.ts       # 任务执行状态管理
│
├── styles/                    # 全局样式
│   ├── index.css              # 样式入口
│   ├── foundation/
│   │   ├── _reset.css         # CSS 重置
│   │   └── _tokens.css        # 设计令牌（CSS 变量）
│   ├── themes/
│   │   └── _variables.css     # 主题变量（light/dark）
│   └── utilities/
│       ├── _animations.css    # 动画
│       ├── _glass.css         # 毛玻璃效果
│       ├── _scrollbar.css     # 自定义滚动条
│       └── _index.css         # 工具类入口
│
├── types/                     # TypeScript 类型定义
│   ├── command.ts             # 命令相关类型
│   ├── executionHistory.ts    # 执行历史类型
│   ├── taskexec.ts            # 任务执行类型
│   ├── theme.ts               # 主题类型
│   └── theme.d.ts             # 主题类型声明
│
├── utils/                     # 工具函数
│   ├── errorHandler.ts        # 错误处理器
│   └── useToast.ts            # Toast 通知
│
└── views/                     # 页面视图组件
    ├── Dashboard.vue          # 仪表盘（首页）
    ├── Devices.vue            # 设备管理
    ├── Commands.vue           # 命令组管理
    ├── Tasks.vue              # 任务组管理
    ├── TaskExecution.vue      # 任务执行监控
    ├── Topology.vue           # 拓扑发现与可视化
    ├── TopologyCommandConfig.vue # 拓扑命令配置
    ├── PlanCompare.vue        # 规划比对
    ├── Settings.vue           # 系统设置
    └── Tools/
        ├── BatchPing.vue      # 批量 Ping
        ├── ConfigForge.vue    # 配置生成器
        ├── FileServers.vue    # 文件服务器
        ├── NetworkCalc.vue    # 网络计算器
        └── ProtocolRef.vue    # 协议参考
```

### 5.3 路由设计

采用 **Hash 路由模式**（`createWebHashHistory`），路由配置见 [`frontend/src/router/index.ts`](frontend/src/router/index.ts)：

| 路径 | 组件 | 说明 |
|------|------|------|
| `/` | Dashboard | 仪表盘（静态导入，首屏快速加载） |
| `/devices` | Devices | 设备管理 |
| `/commands` | Commands | 命令组管理 |
| `/tasks` | Tasks | 任务组管理 |
| `/task-execution` | TaskExecution | 任务执行监控 |
| `/topology` | Topology | 拓扑发现 |
| `/topology-command-config` | TopologyCommandConfig | 拓扑命令配置 |
| `/plan-compare` | PlanCompare | 规划比对 |
| `/tools/calc` | NetworkCalc | 网络计算器 |
| `/tools/protocol` | ProtocolRef | 协议参考 |
| `/tools/config` | ConfigForge | 配置生成器 |
| `/tools/ping` | BatchPing | 批量 Ping |
| `/tools/fileservers` | FileServers | 文件服务器 |
| `/settings` | Settings | 系统设置 |

除首页 Dashboard 外，所有页面均采用**懒加载**（`() => import(...)`）以优化首屏体积。

### 5.4 状态管理

使用 **Pinia** 进行全局状态管理：

- [`taskexecStore.ts`](frontend/src/stores/taskexecStore.ts) — 任务执行状态（运行中任务、进度、事件流）

### 5.5 前后端通信机制

NetWeaverGo 采用 **Wails v3 桥接机制** 实现 Go↔前端通信：

```
┌─────────────────┐     Wails Bridge      ┌─────────────────┐
│   Vue 前端       │ ◄──────────────────► │   Go 后端        │
│                  │   自动生成 TS 绑定     │                  │
│  api.ts          │ ──────────────────► │  internal/ui/*   │
│  (命名空间导出)   │   函数调用            │  (Wails Service) │
│                  │                      │                  │
│  bindings/       │ ◄────────────────── │  models/         │
│  (类型定义)       │   类型同步            │  (数据结构)       │
│                  │                      │                  │
│  taskexecStore   │ ◄── Event Bridge ── │  EventBus        │
│  (实时状态)       │   事件推送            │  (事件总线)       │
└─────────────────┘                      └─────────────────┘
```

**API 层设计**（[`frontend/src/services/api.ts`](frontend/src/services/api.ts)）：

采用**命名空间模式**组织，每个命名空间对应后端一个服务：

```typescript
export const DeviceAPI = { ... }           // 设备管理
export const CommandGroupAPI = { ... }     // 命令组管理
export const TaskGroupAPI = { ... }        // 任务组管理
export const SettingsAPI = { ... }         // 系统设置
export const ForgeAPI = { ... }            // ConfigForge
export const QueryAPI = { ... }            // 通用查询
export const ExecutionHistoryAPI = { ... } // 执行历史
export const PlanCompareAPI = { ... }      // 规划比对
export const TaskExecutionAPI = { ... }    // 任务执行
export const TopologyCommandAPI = { ... }  // 拓扑命令配置
```

**类型来源规则**：
1. 后端 DTO 类型：全部从 `bindings/` 导入
2. 前端视图态/表单态类型：定义在 `types/` 目录
3. `api.ts` 仅负责聚合导出，保持类型来源唯一

### 5.6 组件体系

组件按**业务域**组织，遵循以下分层：

```
views/          → 页面级组件（路由对应）
components/     → 业务组件（按域分组）
composables/    → 可复用逻辑（组合式函数）
common/         → 通用 UI 组件
```

**核心页面组件**：

| 页面 | 核心组件 | 说明 |
|------|----------|------|
| Dashboard | 统计卡片、快速操作 | 系统概览 |
| Devices | DeviceTable, DeviceEditModal | 设备 CRUD + 批量操作 |
| Commands | CommandEditor | 命令组编辑 |
| Tasks | TaskEditModal, TaskDetailModal | 任务组配置 |
| TaskExecution | StageProgress, VirtualLogTerminal | 实时执行监控 |
| Topology | TopologyGraph (Cytoscape.js) | 拓扑图可视化 |
| PlanCompare | 差异表格、导出 | 规划比对 |
| ConfigForge | TemplateEditor, VariablesPanel, OutputPreview | 配置生成 |
| BatchPing | 结果表格、进度条 | 批量探测 |
| FileServers | 服务器状态、日志 | 文件服务管理 |
| Settings | RuntimeConfigPanel | 系统配置 |

### 5.7 样式架构

采用 **Tailwind CSS 4.2 + CSS 变量主题系统**：

```
styles/
├── index.css              # 入口（导入所有子模块）
├── foundation/
│   ├── _reset.css         # CSS 重置
│   └── _tokens.css        # 设计令牌（间距、字号、圆角等）
├── themes/
│   └── _variables.css     # 主题变量（light/dark 双主题）
└── utilities/
    ├── _animations.css    # 动画（fade-in、slide 等）
    ├── _glass.css         # 毛玻璃效果
    └── _scrollbar.css     # 自定义滚动条样式
```

主题切换通过 [`useTheme.ts`](frontend/src/composables/useTheme.ts) 组合式函数管理，支持 light/dark/system 三种模式。

---

## 6. 数据库设计

### 6.1 数据库选型

- **数据库**: SQLite（嵌入式，零配置）
- **ORM**: GORM（Go 最流行的 ORM）
- **驱动**: glebarez/sqlite（纯 Go 实现，无 CGO 依赖）
- **优化**: WAL 模式、连接池（10 max open / 5 max idle）、预编译语句缓存

### 6.2 核心表结构

```
┌─────────────────────────────────────────────────────────────┐
│                      基础配置表                               │
├─────────────────────┬───────────────────────────────────────┤
│ device_assets       │ 设备资产（IP/端口/凭证/厂商/分组/标签）  │
│ global_settings     │ 全局设置（超时/SSH算法/主题）            │
│ command_groups      │ 命令组（名称/命令列表/标签）              │
│ task_groups         │ 任务组（设备组×命令组/执行模式/备份配置） │
│ runtime_settings    │ 运行时 KV 配置                         │
│ file_server_configs │ 文件服务器配置                          │
│ topology_vendor_    │ 拓扑厂商字段命令映射                    │
│   field_commands    │                                        │
├─────────────────────┴───────────────────────────────────────┤
│                      任务执行表                               │
├─────────────────────┬───────────────────────────────────────┤
│ task_definitions    │ 任务定义（kind/配置JSON）               │
│ task_runs           │ 任务运行实例（状态/进度/时间）           │
│ task_run_stages     │ 阶段运行状态（kind/进度/统计）          │
│ task_run_units      │ 调度单元（设备级/状态/日志路径）         │
│ task_run_events     │ 事件流水（全量审计）                    │
│ task_artifacts      │ 产物索引（原始输出/解析结果/拓扑图）     │
├─────────────────────┴───────────────────────────────────────┤
│                      规划比对表                               │
├─────────────────────┬───────────────────────────────────────┤
│ plan_files          │ 规划文件元数据                          │
│ planned_links       │ 规划链路                               │
│ diff_reports        │ 差异报告                               │
│ diff_items          │ 差异项                                 │
└─────────────────────┴───────────────────────────────────────┘
```

---

## 7. 构建与部署

### 7.1 构建流程

构建脚本 [`build.bat`](build.bat) 执行以下步骤：

```
Step 0: 生成 Windows 资源文件（图标）
  └── rsrc.exe → cmd/netweaver/rsrc.syso

Step 1: 构建前端
  └── cd frontend && npm run build
      └── vue-tsc -b && vite build → frontend/dist/

Step 2: 构建 Go 应用
  └── wails build
      └── 嵌入 frontend/dist/ → 单一可执行文件

Step 3: 输出
  └── build/bin/netWeaverGo.exe
```

### 7.2 嵌入式前端资源

前端构建产物通过 Go 的 `embed` 机制嵌入到可执行文件中：

```go
//go:embed frontend/dist
var FrontendAssets embed.FS
```

运行时通过 [`fs.Sub`](cmd/netweaver/main.go:83) 提取子文件系统，由 Wails 的 `AssetFileServerFS` 提供静态文件服务。

---

## 8. 架构设计原则

### 8.1 分层解耦

- **UI 服务层**（`internal/ui/`）仅负责 Wails 桥接和数据转换，不包含业务逻辑
- **业务逻辑层**（`internal/taskexec/`、`internal/executor/`）独立于 UI 框架
- **数据访问层**（`internal/repository/`）通过接口抽象，支持测试替身

### 8.2 接口驱动

关键组件均通过接口定义契约：

- [`DeviceRepository`](internal/repository/interfaces.go:16) — 数据访问
- [`ParserProvider`](internal/parser/manager.go) / [`ParserReloader`](internal/parser/manager.go) — 解析器
- [`FileServer`](internal/fileserver/server.go:51) — 文件服务器
- [`ProfileProvider`](internal/config/device_profile.go:94) — 设备画像
- [`PlanCompiler`](internal/taskexec/compiler.go) — 计划编译器
- [`StageExecutor`](internal/taskexec/runtime.go) — 阶段执行器

### 8.3 事件驱动

- [`EventBus`](internal/taskexec/eventbus.go) — 带缓冲的事件总线，解耦生产者和消费者
- [`SnapshotHub`](internal/taskexec/snapshot.go) — 执行快照管理，支持前端实时状态订阅
- [`taskexec_event_bridge.go`](internal/ui/taskexec_event_bridge.go) — Go→前端事件桥接

### 8.4 并发安全

- 关键数据结构使用 `sync.RWMutex` 保护
- SSH 连接使用 `atomic.Bool` 标记关闭状态
- 数据库操作使用 GORM 事务保证一致性
- Worker Pool 模型控制并发设备数

### 8.5 可测试性

- 丰富的单元测试（`*_test.go` 文件遍布各包）
- Golden 测试（`testdata/` 目录下的期望输出）
- Mock 实现（[`mock_device_repository.go`](internal/repository/mock_device_repository.go)）
- 回归测试（`testdata/regression/`）

---

## 9. 数据流全景图

### 普通任务执行流

```
用户配置任务组
    │
    ▼
TaskGroupService.CreateTaskGroup()
    │
    ▼
TaskExecutionService.CreateNormalTask()
    │  (TaskDefinition)
    ▼
TaskExecutionService.StartTask()
    │
    ▼
NormalTaskCompiler.Compile()
    │  (ExecutionPlan → StagePlan[] → UnitPlan[])
    ▼
RuntimeManager.Execute()
    │
    ├── Stage 1: device_command
    │   ├── DeviceCommandExecutor.Execute()
    │   │   ├── DeviceExecutor.Connect()     → SSH 连接
    │   │   ├── DeviceExecutor.Initialize()  → 禁用分页
    │   │   └── DeviceExecutor.ExecutePlan() → 逐步执行命令
    │   │       ├── StreamEngine.Read()      → 流式读取
    │   │       ├── SessionReducer.Reduce()  → 状态归约
    │   │       └── Matcher.Match()          → 提示符/错误检测
    │   └── EventBus.Emit() → SnapshotHub.Update()
    │
    └── 完成 → TaskRun.Status = "completed"
```

### 拓扑任务执行流

```
用户配置拓扑任务
    │
    ▼
TopologyTaskCompiler.Compile()
    │
    ▼
RuntimeManager.Execute()
    │
    ├── Stage 1: device_collect (并发采集)
    │   └── DeviceCollectExecutor → SSH 执行 LLDP/ARP/FDB 命令
    │
    ├── Stage 2: parse (解析)
    │   └── ParseExecutor → ParserManager.GetParser(vendor)
    │       └── CompositeParser.Parse() → 结构化数据
    │
    └── Stage 3: topology_build (构建)
        └── TopologyBuildExecutor → TopologyBuilder
            ├── LLDP 邻居解析 → 直接链路
            ├── ARP/FDB 融合  → 推断链路
            └── 冲突检测      → 冲突标记
```

---

## 10. 模块依赖关系

```
cmd/netweaver
    ├── config
    ├── logger
    ├── parser
    ├── taskexec
    └── ui

taskexec
    ├── config
    ├── executor
    ├── logger
    ├── matcher
    ├── models
    ├── normalize
    ├── parser
    ├── repository
    ├── report
    ├── sshutil
    └── terminal

executor
    ├── config
    ├── logger
    ├── matcher
    ├── models
    ├── report
    ├── sshutil
    └── terminal

ui
    ├── config
    ├── fileserver
    ├── forge
    ├── icmp
    ├── logger
    ├── models
    ├── normalize
    ├── plancompare
    ├── repository
    ├── taskexec
    └── utils

parser
    └── (独立，仅依赖标准库)

plancompare
    ├── config
    ├── models
    ├── normalize
    └── taskexec

fileserver
    ├── logger
    └── models

forge
    └── (独立，仅依赖标准库)

icmp
    └── logger

matcher
    └── logger

sshutil
    ├── config
    ├── logger
    ├── models
    └── report

terminal
    └── (独立，仅依赖标准库)

report
    └── (独立，仅依赖标准库)

logger
    └── (独立，仅依赖标准库)

repository
    ├── config
    └── models

config
    ├── logger
    └── models

models
    └── (独立，仅依赖标准库)
```

---

> **文档维护说明**: 本文档基于项目源码自动生成，反映当前代码库的实际架构状态。当项目架构发生重大变更时，应及时更新本文档。
