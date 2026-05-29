# NetWeaverGo 未来功能扩展路线图

> **版本**: v1.0  
> **制定日期**: 2026-04-30  
> **基于版本**: 当前代码库架构分析  
> **文档性质**: 战略规划文档，供团队讨论和迭代

---

## 目录

- [1. 扩展战略总览](#1-扩展战略总览)
- [2. 当前能力基线评估](#2-当前能力基线评估)
- [3. 阶段一：基础能力夯实](#3-阶段一基础能力夯实)
- [4. 阶段二：协议与厂商扩展](#4-阶段二协议与厂商扩展)
- [5. 阶段三：智能运维增强](#5-阶段三智能运维增强)
- [6. 阶段四：企业级特性](#6-阶段四企业级特性)
- [7. 阶段五：生态与开放平台](#7-阶段五生态与开放平台)
- [8. 架构演进策略](#8-架构演进策略)
- [9. 风险与依赖矩阵](#9-风险与依赖矩阵)

---

## 1. 扩展战略总览

### 1.1 愿景

将 NetWeaverGo 从**网络自动化编排工具**演进为**企业级网络运维平台**，覆盖网络设备全生命周期管理。

### 1.2 扩展原则

| 原则 | 说明 |
|------|------|
| **架构先行** | 每个新功能必须先评估架构影响，确保与分层架构一致 |
| **接口驱动** | 新能力通过接口定义契约，保持可测试性和可替换性 |
| **渐进增强** | 优先增强现有模块，避免大规模重构 |
| **向后兼容** | 数据模型变更通过迁移脚本保证兼容 |
| **事件驱动** | 新功能优先使用 EventBus 机制，保持模块解耦 |

### 1.3 阶段总览

```
阶段一：基础能力夯实 ──→ 阶段二：协议与厂商扩展 ──→ 阶段三：智能运维增强
      │                        │                        │
      ▼                        ▼                        ▼
  核心稳定性               连接能力扩展             智能化能力
  用户体验优化             厂商覆盖扩展             自动化决策
  
阶段四：企业级特性 ──→ 阶段五：生态与开放平台
      │                        │
      ▼                        ▼
  安全与权限               API 开放
  多租户支持               插件生态
```

---

## 2. 当前能力基线评估

### 2.1 已实现能力矩阵

| 能力域 | 成熟度 | 覆盖范围 | 扩展空间 |
|--------|--------|----------|----------|
| **设备管理** | ★★★★☆ | CRUD、分组、画像（3厂商） | 厂商扩展、设备发现 |
| **命令执行** | ★★★★★ | SSH并发、流式读取、错误处理 | Telnet、NETCONF |
| **拓扑发现** | ★★★★☆ | LLDP/FDB/ARP融合、可视化 | 历史对比、告警 |
| **配置生成** | ★★★☆☆ | 模板引擎、变量展开 | 合规检查、回滚 |
| **规划比对** | ★★★★☆ | 差异报告、导出 | 实时比对 |
| **文件服务** | ★★★★☆ | SFTP/FTP/TFTP/HTTP | 传输队列 |
| **网络工具** | ★★★☆☆ | Ping、计算器、协议参考 | Traceroute、SNMP |
| **任务编排** | ★★★★☆ | 并发执行、阶段编排 | 定时任务、依赖链 |
| **报告系统** | ★★★☆☆ | CSV/日志 | HTML报表、趋势分析 |
| **安全机制** | ★★☆☆☆ | 密码脱敏、日志脱敏 | RBAC、审计 |

### 2.2 架构优势

- **分层清晰**：UI服务层 → 业务逻辑层 → 数据访问层，职责明确
- **事件驱动**：EventBus + SnapshotHub 支持实时状态推送
- **接口驱动**：Repository、ProfileProvider、PlanCompiler 等接口抽象
- **并发安全**：sync.RWMutex、atomic.Bool、GORM 事务
- **可测试**：丰富的单元测试、Golden 测试、Mock 实现

### 2.3 架构瓶颈

| 瓶颈 | 影响 | 解决方案 |
|------|------|----------|
| 仅支持 SSH 协议 | 无法管理仅支持 Telnet/SNMP 的设备 | 阶段二扩展协议层 |
| 厂商画像硬编码 | 新增厂商需修改代码 | 阶段二引入画像注册机制 |
| 无定时任务能力 | 无法自动化周期性运维 | 阶段三引入调度器 |
| 无权限控制 | 不适合多用户场景 | 阶段四引入 RBAC |
| SQLite 单机限制 | 不支持分布式部署 | 阶段四评估数据库迁移 |

---

## 3. 阶段一：基础能力夯实

> **目标**：提升核心稳定性、用户体验和开发效率

### 3.1 Dashboard 增强

**当前状态**：静态统计卡片，信息有限

**扩展内容**：

| 功能 | 说明 | 涉及模块 |
|------|------|----------|
| 实时任务状态面板 | 显示运行中任务的进度、设备状态 | `taskexecStore`、`Dashboard.vue` |
| 最近执行历史 | 快速查看最近 N 次任务执行结果 | `ExecutionHistoryService` |
| 设备健康概览 | 设备在线/离线状态统计 | `DeviceService`、`BatchPingEngine` |
| 快速操作入口 | 一键启动常用任务 | `Dashboard.vue` |
| 系统资源监控 | 内存、协程数、数据库大小 | 新增 `SystemMonitor` |

**架构影响**：
- 新增 `SystemMonitor` 服务暴露系统指标
- `Dashboard.vue` 重构为组件化布局
- `taskexecStore` 增加历史任务订阅

### 3.2 执行历史增强

**当前状态**：基础的执行记录查询

**扩展内容**：

| 功能 | 说明 | 涉及模块 |
|------|------|----------|
| 执行记录搜索 | 按设备、命令、时间范围搜索 | `ExecutionHistoryService` |
| 执行趋势图表 | 成功率、耗时趋势可视化 | 新增 `TrendChart.vue` |
| 批量导出 | 选择多条记录批量导出 CSV/Excel | `report/` |
| 执行对比 | 对比两次执行的输出差异 | 新增 `ExecutionDiff` |
| 标签筛选 | 按任务标签筛选历史记录 | `TaskGroup.Tags` |

**架构影响**：
- `TaskRunEvent` 增加全文索引字段
- 新增 `ExecutionDiffService` 对比服务
- 前端新增 `TrendChart` 组件（基于 Chart.js 或 ECharts）

### 3.3 设备管理增强

**当前状态**：基础 CRUD、分组、标签

**扩展内容**：

| 功能 | 说明 | 涉及模块 |
|------|------|----------|
| 设备导入模板 | 提供 Excel/CSV 导入模板下载 | `DeviceService` |
| 设备连接测试 | 单设备 SSH 连接测试 + 延迟检测 | `SSHClient`、`BatchPingEngine` |
| 设备画像自定义 | 用户可自定义厂商画像参数 | `DeviceProfile` 注册表 |
| 批量凭证更新 | 批量修改设备密码/用户名 | `DeviceRepository` |
| 设备标签管理 | 标签的 CRUD 和批量打标 | `DeviceAsset.Tags` |

**架构影响**：
- `DeviceProfile` 从硬编码改为数据库 + 内置双源
- 新增 `ProfileRepository` 持久化自定义画像
- `DeviceService` 增加连接测试方法

### 3.4 命令组增强

**当前状态**：命令组 CRUD、标签分类

**扩展内容**：

| 功能 | 说明 | 涉及模块 |
|------|------|----------|
| 命令模板库 | 预置常用命令模板（巡检、备份等） | 新增 `CommandTemplate` |
| 命令变量支持 | 命令中支持变量占位符 | `CommandGroup` |
| 命令执行预览 | 执行前预览将发送的命令序列 | `CommandEditor.vue` |
| 命令组导入导出 | JSON 格式导入导出 | `CommandGroupService` |

### 3.5 UI/UX 优化

| 功能 | 说明 | 涉及模块 |
|------|------|----------|
| 键盘快捷键 | 全局快捷键（Ctrl+N 新建、Ctrl+F 搜索等） | `App.vue` |
| 表格列配置 | 用户自定义表格列显示/隐藏/排序 | `DeviceTable.vue` 等 |
| 操作确认优化 | 危险操作二次确认 + 倒计时 | `ConfirmModal.vue` |
| 加载状态优化 | 骨架屏、进度条、加载动画 | 全局组件 |
| 响应式布局 | 适配不同窗口尺寸 | Tailwind CSS 断点 |

### 3.6 国际化（i18n）支持

**当前状态**：界面硬编码中文，无多语言支持

**扩展内容**：

| 功能 | 说明 | 涉及模块 |
|------|------|----------|
| i18n 框架集成 | 集成 vue-i18n 国际化框架 | `main.ts`、`i18n/` |
| 中文语言包 | 提取现有中文文案为语言包 | `locales/zh-CN.json` |
| 英文语言包 | 提供英文翻译 | `locales/en-US.json` |
| 语言切换 | 设置页面支持语言切换 | `Settings.vue` |
| 日志国际化 | 后端日志消息支持中英文 | `logger/` |

**架构影响**：
- 新增 `frontend/src/i18n/` 目录
- 所有 Vue 组件使用 `$t()` 替换硬编码文案
- 后端日志消息使用消息键而非硬编码字符串

### 3.7 拓扑图增强

**当前状态**：基础 Cytoscape.js 拓扑图，布局算法有限

**扩展内容**：

| 功能 | 说明 | 涉及模块 |
|------|------|----------|
| 多种布局算法 | 支持层次、力导向、圆形等多种布局 | `TopologyGraph.vue` |
| 拓扑图导出 | 导出为 PNG/SVG/PDF 格式 | `TopologyGraph.vue` |
| 节点分组 | 按站点/角色/厂商分组显示 | `TopologyGraph.vue` |
| 链路带宽显示 | 根据接口速率显示链路粗细 | `TopologyGraph.vue` |
| 拓扑图标注 | 支持手动添加标注和备注 | `TopologyAnnotation.vue` |
| 拓扑图搜索 | 搜索并高亮特定设备/链路 | `TopologySearch.vue` |
| 小地图导航 | 大型拓扑图的小地图导航 | `TopologyMinimap.vue` |

---

## 4. 阶段二：协议与厂商扩展

> **目标**：扩展连接协议覆盖范围，支持更多网络设备厂商

### 4.1 协议层抽象

**当前状态**：仅实现 SSH 协议连接。数据模型层已预留 TELNET/SNMP 协议定义（[`config.go`](internal/config/config.go:20) 中 `ValidProtocols` 包含 `TELNET` 和 `SNMP`，设备资产可选择协议类型），但执行层（`internal/executor/`）仅通过 `internal/sshutil/` 实现 SSH 连接，Telnet/SNMP 的实际连接和命令执行尚未实现。

**架构重构**：

```
当前架构：
  executor → sshutil/client.go → SSH 连接
  config   → ValidProtocols: [SSH, SNMP, TELNET]  (已预留，但无实现)

目标架构：
  executor → ConnectionProvider (接口)
                ├── SSHProvider    → sshutil/ (已有)
                ├── TelnetProvider → telnetutil/ (新增实现)
                ├── SNMPProvider   → snmputil/ (新增实现)
                └── NETCONFProvider → netconfutil/ (新增实现)
```

**接口设计**：

```go
// ConnectionProvider 连接提供者接口
type ConnectionProvider interface {
    // Connect 建立连接
    Connect(ctx context.Context, target DeviceTarget) (Connection, error)
    // Protocol 返回协议标识
    Protocol() string
    // Supports 检查是否支持目标设备
    Supports(target DeviceTarget) bool
}

// Connection 统一连接接口
type Connection interface {
    // Execute 执行命令并返回输出
    Execute(ctx context.Context, cmd string) (string, error)
    // ExecuteInteractive 交互式执行（支持分页）
    ExecuteInteractive(ctx context.Context, cmd string, opts ExecOptions) (<-chan OutputChunk, error)
    // Close 关闭连接
    Close() error
    // IsAlive 检查连接是否存活
    IsAlive() bool
}
```

**涉及模块**：
- 新增 `internal/telnetutil/` — Telnet 客户端
- 新增 `internal/snmputil/` — SNMP 客户端
- 新增 `internal/netconfutil/` — NETCONF 客户端
- 重构 `internal/executor/executor.go` — 使用 `ConnectionProvider` 接口
- 重构 `internal/sshutil/client.go` — 实现 `ConnectionProvider` 接口

### 4.2 Telnet 协议支持

| 功能 | 说明 | 涉及模块 |
|------|------|----------|
| Telnet 客户端 | 基础 Telnet 连接和命令执行 | `internal/telnetutil/` |
| Telnet 会话管理 | 登录认证、提示符检测 | `telnetutil/session.go` |
| Telnet 分页处理 | 复用现有分页检测逻辑 | `matcher/` |
| 设备协议配置 | 设备资产支持 Telnet 协议选择 | `DeviceAsset.Protocol` |

**技术选型**：
- 使用 `github.com/reiver/go-telnet` 或自行实现轻量级 Telnet 客户端
- 复用 `terminal/` 的 ANSI 解析和行缓冲
- 复用 `matcher/` 的提示符和分页检测

### 4.3 SNMP 协议支持

| 功能 | 说明 | 涉及模块 |
|------|------|----------|
| SNMP GET/WALK | 基础 SNMP 查询操作 | `internal/snmputil/` |
| SNMP Trap 接收 | Trap 消息监听和处理 | `snmputil/trap.go` |
| MIB 浏览器 | MIB 文件解析和 OID 浏览 | 新增 `MibBrowser.vue` |
| SNMP 设备发现 | 通过 SNMP 发现网络设备 | `snmputil/discovery.go` |

**技术选型**：
- 使用 `github.com/gosnmp/gosnmp` — Go 最流行的 SNMP 库
- 支持 SNMPv1/v2c/v3

### 4.4 NETCONF 协议支持

| 功能 | 说明 | 涉及模块 |
|------|------|----------|
| NETCONF 客户端 | 基于 SSH 的 NETCONF 连接 | `internal/netconfutil/` |
| YANG 模型浏览 | YANG 文件解析和数据模型浏览 | `netconfutil/yang.go` |
| 配置推送 | 通过 NETCONF 推送配置 | `netconfutil/config.go` |
| 状态查询 | 通过 NETCONF 查询设备状态 | `netconfutil/query.go` |

**技术选型**：
- 使用 `github.com/nemith/netconf` 或 `github.com/scrapli/scrapligo`
- 复用现有 SSH 连接基础设施

### 4.5 厂商画像扩展

**当前状态**：硬编码 3 个厂商（华为、华三、思科）

**扩展目标**：

| 厂商 | 优先级 | 说明 |
|------|--------|------|
| **锐捷 (Ruijie)** | 高 | 国内市场份额大 |
| **中兴 (ZTE)** | 高 | 运营商市场 |
| **迈普 (Maipu)** | 中 | 行业市场 |
| **Aruba** | 中 | 无线网络 |
| **Juniper** | 中 | 高端市场 |
| **Arista** | 低 | 数据中心 |
| **Palo Alto** | 低 | 防火墙 |

**架构改进**：

```go
// 画像注册机制改进
type ProfileRegistry interface {
    // Register 注册厂商画像
    Register(profile *DeviceProfile)
    // GetByVendor 获取厂商画像
    GetByVendor(vendor string) (*DeviceProfile, bool)
    // ListVendors 列出所有已注册厂商
    ListVendors() []string
    // LoadFromDB 从数据库加载用户自定义画像
    LoadFromDB() error
    // LoadFromFS 从文件系统加载画像文件
    LoadFromFS(dir string) error
}
```

**画像文件格式**（JSON/YAML）：

```json
{
  "vendor": "ruijie",
  "name": "锐捷",
  "pty": { "termType": "vt100", "width": 256, "height": 200 },
  "prompt": { "suffixes": [">", "#"] },
  "pager": { "patterns": ["---- More ----"] },
  "init": { "disablePagerCommands": ["terminal length 0"] },
  "commands": [
    { "command": "show version", "commandKey": "version", "timeoutSec": 30 }
  ]
}
```

### 4.6 CLI 解析模板扩展

**当前状态**：内置 3 个厂商模板（huawei.json、h3c.json、cisco.json）

**扩展内容**：

| 厂商模板 | 优先级 | 解析命令 |
|----------|--------|----------|
| ruijie.json | 高 | version、lldp、interface、mac、arp |
| zte.json | 高 | version、lldp、interface、mac、arp |
| maipu.json | 中 | version、lldp、interface、mac、arp |
| aruba.json | 中 | version、lldp、interface |
| juniper.json | 中 | version、lldp、interface |

**架构改进**：
- `ParserManager` 支持从文件系统加载自定义模板
- 模板版本管理（`templateVersion` 字段）
- 模板验证工具（检查正则语法、字段映射完整性）

---

## 5. 阶段三：智能运维增强

> **目标**：引入定时任务、告警机制、配置合规等智能化能力

### 5.1 定时任务调度器

**当前状态**：所有任务需手动触发

**架构设计**：

```
新增 internal/scheduler/
├── scheduler.go        # 调度器核心
├── cron_parser.go      # Cron 表达式解析
├── trigger.go          # 触发器定义
└── models.go           # 调度任务模型
```

**数据模型**：

```go
// ScheduledTask 定时任务定义
type ScheduledTask struct {
    ID             uint      `gorm:"primaryKey"`
    TaskGroupID    uint      `gorm:"index"`      // 关联任务组
    Name           string    `json:"name"`
    CronExpr       string    `json:"cronExpr"`    // Cron 表达式
    Enabled        bool      `json:"enabled"`
    NextRunAt      time.Time `json:"nextRunAt"`
    LastRunAt      *time.Time `json:"lastRunAt"`
    LastRunStatus  string    `json:"lastRunStatus"`
    MaxRetries     int       `json:"maxRetries"`
    RetryInterval  int       `json:"retryInterval"` // 秒
    CreatedAt      time.Time
    UpdatedAt      time.Time
}
```

**功能清单**：

| 功能 | 说明 | 涉及模块 |
|------|------|----------|
| Cron 表达式支持 | 标准 Cron 语法（5字段） | `scheduler/cron_parser.go` |
| 任务调度管理 | 创建、编辑、启用/禁用定时任务 | `scheduler/scheduler.go` |
| 调度日志 | 记录每次调度的触发时间和结果 | `ScheduledTaskLog` |
| 错误重试 | 失败后自动重试（可配置次数和间隔） | `scheduler/retry.go` |
| 调度日历 | 可视化展示任务调度时间线 | 新增 `ScheduleCalendar.vue` |

**前端页面**：
- 新增 `/schedule` 路由
- `ScheduleCalendar.vue` — 调度日历视图
- `ScheduleEditModal.vue` — 定时任务编辑弹窗

### 5.2 任务依赖链

**当前状态**：任务独立执行，无依赖关系

**架构设计**：

```go
// TaskChain 任务链定义
type TaskChain struct {
    ID          uint        `gorm:"primaryKey"`
    Name        string      `json:"name"`
    Description string      `json:"description"`
    Steps       []ChainStep `gorm:"serializer:json"`
    Enabled     bool        `json:"enabled"`
}

// ChainStep 链步骤
type ChainStep struct {
    StepID       string `json:"stepId"`
    TaskGroupID  uint   `json:"taskGroupId"`
    Order        int    `json:"order"`
    Condition    string `json:"condition"`    // always / on_success / on_failure
    WaitTimeout  int    `json:"waitTimeout"`  // 等待超时（秒）
}
```

**功能清单**：

| 功能 | 说明 |
|------|------|
| 任务链定义 | 可视化定义任务执行顺序和条件 |
| 条件分支 | 根据前序任务结果决定后续执行路径 |
| 并行步骤 | 支持同一步骤内多个任务并行执行 |
| 链状态追踪 | 实时追踪整条任务链的执行进度 |
| 链模板 | 预置常用任务链模板（巡检→备份→比对） |

### 5.3 配置合规检查

**当前状态**：ConfigForge 仅支持配置生成，无合规检查

**架构设计**：

```
新增 internal/compliance/
├── checker.go          # 合规检查器
├── rules.go            # 规则定义
├── rule_engine.go      # 规则引擎
└── models.go           # 合规模型
```

**功能清单**：

| 功能 | 说明 | 涉及模块 |
|------|------|----------|
| 合规规则定义 | 定义配置检查规则（正则、关键字、阈值） | `compliance/rules.go` |
| 规则引擎 | 执行规则检查并生成报告 | `compliance/rule_engine.go` |
| 合规报告 | 生成合规检查报告（HTML/Excel） | `compliance/report.go` |
| 自动修复建议 | 对不合规项提供修复建议 | `compliance/suggestion.go` |
| 规则模板库 | 预置常见合规规则（安全基线等） | `compliance/templates/` |

**规则示例**：

```json
{
  "ruleId": "SEC-001",
  "name": "SNMP Community 检查",
  "description": "检查是否使用默认 SNMP Community",
  "severity": "high",
  "vendor": ["huawei", "h3c", "cisco"],
  "command": "display current-configuration",
  "pattern": "snmp-agent community (public|private)",
  "expected": "none",
  "suggestion": "修改 SNMP Community 为非默认值"
}
```

### 5.4 拓扑历史对比

**当前状态**：支持离线重放，但无历史对比功能

**功能清单**：

| 功能 | 说明 | 涉及模块 |
|------|------|----------|
| 拓扑快照存储 | 每次拓扑构建保存快照 | `TopologySnapshot` |
| 快照对比 | 对比两个时间点的拓扑差异 | `topology_compare.go` |
| 变更追踪 | 追踪链路新增/删除/变更 | `TopologyEdgeDiff` |
| 变更时间线 | 可视化展示拓扑变更历史 | 新增 `TopologyTimeline.vue` |
| 自动告警 | 拓扑变更时触发告警 | `AlertService` |

**数据模型**：

```go
// TopologySnapshot 拓扑快照
type TopologySnapshot struct {
    ID          uint      `gorm:"primaryKey"`
    TaskRunID   string    `gorm:"index"`
    SnapshotAt  time.Time `gorm:"index"`
    NodeCount   int
    EdgeCount   int
    GraphJSON   string    `gorm:"type:text"` // 完整拓扑图 JSON
    Fingerprint string    `gorm:"index"`      // 拓扑指纹（哈希）
}
```

### 5.5 告警通知系统

**架构设计**：

```
新增 internal/alerting/
├── alert_manager.go    # 告警管理器
├── rules.go            # 告警规则
├── channels.go         # 通知渠道
└── models.go           # 告警模型
```

**功能清单**：

| 功能 | 说明 |
|------|------|
| 告警规则定义 | 定义触发条件（设备离线、任务失败、拓扑变更等） |
| 通知渠道 | 支持多种通知方式 |
| 告警历史 | 记录所有告警事件 |
| 告警抑制 | 避免重复告警（静默期、去重） |
| 告警升级 | 超时未处理自动升级 |

**通知渠道**：

| 渠道 | 优先级 | 说明 |
|------|--------|------|
| 应用内通知 | 高 | Toast + 通知中心 |
| 邮件通知 | 高 | SMTP 邮件发送 |
| Webhook | 中 | HTTP 回调 |
| 企业微信 | 中 | 企业微信机器人 |
| 钉钉 | 低 | 钉钉机器人 |

### 5.6 配置差异分析

**当前状态**：规划比对仅支持拓扑链路比对

**扩展内容**：

| 功能 | 说明 | 涉及模块 |
|------|------|----------|
| 配置快照 | 保存设备配置的历史快照 | `ConfigSnapshot` |
| 配置差异对比 | 对比两个时间点的配置差异 | `ConfigDiffService` |
| 配置变更追踪 | 追踪配置变更历史 | `ConfigChangeLog` |
| 配置回滚建议 | 基于差异生成回滚命令 | `ConfigRollback` |

---

## 6. 阶段四：企业级特性

> **目标**：满足企业级部署需求，支持多用户、高可用

### 6.1 用户认证与权限

**架构设计**：

```
新增 internal/auth/
├── auth_service.go     # 认证服务
├── rbac.go             # RBAC 权限引擎
├── session.go          # 会话管理
└── models.go           # 用户/角色模型
```

**数据模型**：

```go
// User 用户
type User struct {
    ID           uint      `gorm:"primaryKey"`
    Username     string    `gorm:"uniqueIndex"`
    PasswordHash string    // bcrypt 哈希
    DisplayName  string
    Email        string
    RoleID       uint      `gorm:"index"`
    Enabled      bool
    LastLoginAt  *time.Time
    CreatedAt    time.Time
}

// Role 角色
type Role struct {
    ID          uint   `gorm:"primaryKey"`
    Name        string `gorm:"uniqueIndex"`
    Permissions []Permission `gorm:"serializer:json"`
}

// Permission 权限
type Permission struct {
    Resource string `json:"resource"` // device, task, topology, settings
    Actions  []string `json:"actions"` // read, write, execute, delete
}
```

**预置角色**：

| 角色 | 权限 |
|------|------|
| **管理员** | 全部权限 |
| **运维工程师** | 设备读写、任务执行、拓扑查看 |
| **只读用户** | 设备查看、拓扑查看、报告查看 |
| **审计员** | 日志查看、审计报告 |

### 6.2 审计日志系统

**功能清单**：

| 功能 | 说明 |
|------|------|
| 操作审计 | 记录所有用户操作（登录、设备修改、任务执行等） |
| 审计查询 | 按时间、用户、操作类型查询审计日志 |
| 审计报告 | 生成审计报告（日/周/月） |
| 日志导出 | 导出审计日志为 CSV/Excel |
| 日志保留策略 | 配置日志保留天数，自动清理过期日志 |

**数据模型**：

```go
// AuditLog 审计日志
type AuditLog struct {
    ID        uint      `gorm:"primaryKey"`
    UserID    uint      `gorm:"index"`
    Username  string    `gorm:"index"`
    Action    string    `gorm:"index"` // login, create, update, delete, execute
    Resource  string    `gorm:"index"` // device, task, topology, settings
    ResourceID string
    Details   string    `gorm:"type:text"` // JSON 格式的操作详情
    IPAddress string
    UserAgent string
    CreatedAt time.Time `gorm:"index"`
}
```

### 6.3 凭证安全管理

**当前状态**：密码明文存储在 SQLite

**改进方案**：

| 功能 | 说明 | 优先级 |
|------|------|--------|
| 密码加密存储 | 使用 AES-256 加密存储设备密码 | 高 |
| 主密钥管理 | 支持环境变量或文件提供主密钥 | 高 |
| 凭证库 | 集中管理所有设备凭证 | 中 |
| 密码轮换 | 定期提醒更新设备密码 | 低 |
| 外部密钥集成 | 支持 HashiCorp Vault 等外部密钥管理 | 低 |

### 6.4 数据库优化

**当前状态**：SQLite 嵌入式数据库

**优化方案**：

| 优化项 | 说明 | 优先级 |
|--------|------|--------|
| 索引优化 | 为高频查询字段添加复合索引 | 高 |
| 数据归档 | 历史执行记录自动归档 | 高 |
| 数据库迁移工具 | 版本化数据库迁移脚本 | 高 |
| 可选 PostgreSQL | 支持切换到 PostgreSQL | 中 |
| 读写分离 | 大规模场景下的读写分离 | 低 |

### 6.5 报表系统增强

**当前状态**：CSV 导出、基础日志

**扩展内容**：

| 功能 | 说明 |
|------|------|
| HTML 报表 | 生成美观的 HTML 执行报告 |
| Excel 报表 | 多 Sheet 的 Excel 报表（摘要+详情+图表） |
| 定期报告 | 自动生成并发送定期运维报告 |
| 自定义报表 | 用户自定义报表模板和字段 |
| 报表订阅 | 订阅报表，自动推送到邮箱 |

### 6.6 配置备份增强

**当前状态**：基础的 SFTP 配置备份

**扩展内容**：

| 功能 | 说明 |
|------|------|
| 备份版本管理 | 保留多个历史版本，支持回滚 |
| 备份差异高亮 | 对比相邻版本的配置差异 |
| 备份策略 | 支持全量/增量备份策略 |
| 备份验证 | 备份完成后自动验证文件完整性 |
| 备份存储扩展 | 支持远程存储（S3、FTP、NAS） |

---

## 7. 阶段五：生态与开放平台

> **目标**：构建开放生态，支持第三方集成和扩展

### 7.1 REST API 开放

**架构设计**：

```
新增 internal/api/
├── router.go           # HTTP 路由
├── middleware.go        # 中间件（认证、限流、日志）
├── handlers/           # 处理器
│   ├── devices.go
│   ├── tasks.go
│   ├── topology.go
│   └── ...
└── models.go           # API 模型
```

**API 设计原则**：
- RESTful 风格
- JWT 认证
- 版本化（`/api/v1/`）
- OpenAPI 3.0 文档
- 限流保护

**核心 API 端点**：

| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/v1/devices` | GET/POST | 设备列表/创建 |
| `/api/v1/devices/:id` | GET/PUT/DELETE | 设备详情/更新/删除 |
| `/api/v1/tasks` | GET/POST | 任务列表/创建 |
| `/api/v1/tasks/:id/execute` | POST | 执行任务 |
| `/api/v1/topology` | GET | 获取拓扑图 |
| `/api/v1/topology/snapshots` | GET | 拓扑快照列表 |
| `/api/v1/commands` | GET/POST | 命令组管理 |
| `/api/v1/reports` | GET | 报告列表 |
| `/api/v1/webhooks` | GET/POST | Webhook 管理 |

### 7.2 Webhook 机制

**功能清单**：

| 功能 | 说明 |
|------|------|
| Webhook 注册 | 注册回调 URL 和触发事件 |
| 事件触发 | 任务完成、拓扑变更、告警触发时发送 Webhook |
| 重试机制 | 发送失败自动重试（指数退避） |
| 签名验证 | HMAC 签名保证消息完整性 |
| Webhook 日志 | 记录所有 Webhook 发送历史 |

**支持的事件类型**：

```go
const (
    EventTaskCompleted   = "task.completed"
    EventTaskFailed      = "task.failed"
    EventTopologyChanged = "topology.changed"
    AlertTriggered       = "alert.triggered"
    DeviceStatusChanged  = "device.status_changed"
)
```

### 7.3 插件系统

**架构设计**：

```
新增 internal/plugin/
├── manager.go          # 插件管理器
├── loader.go           # 插件加载器
├── registry.go         # 插件注册表
└── interface.go        # 插件接口定义
```

**插件接口**：

```go
// Plugin 插件接口
type Plugin interface {
    // Name 返回插件名称
    Name() string
    // Version 返回插件版本
    Version() string
    // Init 初始化插件
    Init(ctx PluginContext) error
    // Start 启动插件
    Start() error
    // Stop 停止插件
    Stop() error
}

// CommandPlugin 命令插件
type CommandPlugin interface {
    Plugin
    // Commands 返回插件提供的命令
    Commands() []CommandDefinition
    // Execute 执行命令
    Execute(cmd string, args map[string]string) (string, error)
}

// ParserPlugin 解析器插件
type ParserPlugin interface {
    Plugin
    // Vendor 返回支持的厂商
    Vendor() string
    // Parse 解析命令输出
    Parse(commandKey string, output string) (interface{}, error)
}
```

**插件类型**：

| 类型 | 说明 | 示例 |
|------|------|------|
| 命令插件 | 扩展命令执行能力 | Traceroute、DNS 查询 |
| 解析器插件 | 扩展 CLI 输出解析 | 新厂商解析器 |
| 通知插件 | 扩展通知渠道 | Slack、Telegram |
| 存储插件 | 扩展数据存储 | S3、MinIO |
| 集成插件 | 第三方系统集成 | CMDB、ITSM |

### 7.4 数据导出与集成

| 功能 | 说明 |
|------|------|
| CMDB 集成 | 设备信息同步到 CMDB 系统 |
| ITSM 集成 | 工单系统集成（创建/关闭工单） |
| 监控系统集成 | 对接 Zabbix、Prometheus 等监控系统 |
| 数据导出 API | 提供标准化数据导出接口 |
| 数据导入工具 | 支持从其他工具导入设备和配置 |

---

## 8. 架构演进策略

### 8.1 模块演进路线

```
                    当前架构                           目标架构
                    
┌─────────────────────────────┐    ┌─────────────────────────────────────┐
│      UI 服务层 (Wails)       │    │         API Gateway                  │
│  internal/ui/*_service.go   │    │  ├── Wails Bridge (桌面端)            │
│                             │    │  ├── REST API (Web/集成)              │
│                             │    │  └── WebSocket (实时推送)             │
├─────────────────────────────┤    ├─────────────────────────────────────┤
│      业务逻辑层              │    │         业务逻辑层                    │
│  taskexec / executor        │    │  ├── taskexec (任务执行)              │
│  topology / plancompare     │    │  ├── scheduler (定时调度) [新增]      │
│  forge / icmp               │    │  ├── compliance (合规检查) [新增]     │
│                             │    │  ├── alerting (告警通知) [新增]       │
│                             │    │  ├── auth (认证授权) [新增]           │
│                             │    │  └── plugin (插件系统) [新增]         │
├─────────────────────────────┤    ├─────────────────────────────────────┤
│      连接层                  │    │         连接层 (抽象化)               │
│  sshutil                    │    │  ├── ConnectionProvider (接口)        │
│                             │    │  ├── SSHProvider                      │
│                             │    │  ├── TelnetProvider [新增]            │
│                             │    │  ├── SNMPProvider [新增]              │
│                             │    │  └── NETCONFProvider [新增]           │
├─────────────────────────────┤    ├─────────────────────────────────────┤
│      数据层                  │    │         数据层                        │
│  repository (SQLite/GORM)   │    │  ├── repository (接口)                │
│                             │    │  ├── SQLite (默认)                    │
│                             │    │  └── PostgreSQL (可选) [新增]         │
└─────────────────────────────┘    └─────────────────────────────────────┘
```

### 8.2 关键架构决策

| 决策点 | 选项 | 推荐 | 理由 |
|--------|------|------|------|
| API 框架 | Gin / Echo / Fiber | **Gin** | 社区最大、文档完善、性能优秀 |
| 定时调度 | 自研 / robfig/cron | **robfig/cron** | 成熟稳定、Cron 语法完整 |
| 认证方案 | JWT / Session | **JWT** | 无状态、适合桌面应用 |
| ORM 扩展 | 继续 GORM / 切换 | **继续 GORM** | 已有大量代码，迁移成本高 |
| 前端图表 | Chart.js / ECharts | **ECharts** | 功能更丰富、中文文档好 |
| 插件系统 | Go Plugin / gRPC | **gRPC** | 跨语言、进程隔离 |

### 8.3 数据库迁移策略

```go
// 版本化迁移
type Migration struct {
    Version     string
    Description string
    Up          func(tx *gorm.DB) error
    Down        func(tx *gorm.DB) error
}

// 迁移注册
var migrations = []Migration{
    {Version: "001", Description: "添加定时任务表", Up: migrate001Up, Down: migrate001Down},
    {Version: "002", Description: "添加用户认证表", Up: migrate002Up, Down: migrate002Down},
    // ...
}
```

---

## 9. 风险与依赖矩阵

### 9.1 技术风险

| 风险 | 影响 | 概率 | 缓解措施 |
|------|------|------|----------|
| Telnet 协议兼容性 | 不同厂商 Telnet 实现差异大 | 高 | 充分测试、厂商画像适配 |
| NETCONF 复杂度 | YANG 模型复杂、厂商扩展不一致 | 高 | 优先支持核心操作、渐进扩展 |
| SQLite 性能瓶颈 | 大量历史数据查询变慢 | 中 | 数据归档、索引优化、可选 PostgreSQL |
| 插件系统安全 | 第三方插件可能影响主程序稳定性 | 中 | 进程隔离、权限控制、沙箱机制 |
| 前端包体积增长 | 功能增加导致首屏加载变慢 | 中 | 懒加载、代码分割、Tree Shaking |

### 9.2 依赖关系

```
阶段一（基础夯实）──→ 阶段二（协议扩展）
        │                    │
        │                    ▼
        │            阶段三（智能运维）
        │                    │
        ▼                    ▼
阶段四（企业级）◄─── 阶段三完成后的反馈
        │
        ▼
阶段五（开放平台）
```

**关键依赖**：
- 阶段二的协议层抽象是后续所有协议扩展的基础
- 阶段三的定时调度器依赖阶段一的任务执行稳定性
- 阶段四的 RBAC 依赖阶段一的用户管理基础
- 阶段五的 REST API 依赖阶段四的认证系统

### 9.3 优先级建议

| 阶段 | 优先级 | 预期价值 |
|------|--------|----------|
| 阶段一 | **P0** | 提升核心体验，为后续扩展打基础 |
| 阶段二 | **P1** | 扩大设备覆盖范围，提升市场竞争力 |
| 阶段三 | **P1** | 提升运维效率，减少人工干预 |
| 阶段四 | **P2** | 满足企业级需求，支持多用户场景 |
| 阶段五 | **P3** | 构建生态，长期价值 |

### 9.4 跨阶段关注项

以下关注项贯穿多个阶段，需在各阶段中持续投入：

| 关注项 | 说明 | 涉及阶段 |
|--------|------|----------|
| **数据导入导出标准化** | 统一所有模块的导入导出格式（JSON Schema 校验、Excel 模板标准化、CSV 编码规范） | 阶段一~三 |
| **远程访问能力** | 当前为桌面应用（Wails），未来需支持 Web 远程访问（REST API + WebSocket 实时推送） | 阶段四~五 |
| **自动化测试覆盖** | 每个新功能必须配套单元测试和集成测试，保持测试覆盖率 | 全阶段 |
| **文档同步更新** | 架构变更后及时更新 PROJECT_ARCHITECTURE.md 和本路线图 | 全阶段 |
| **性能基准测试** | 建立关键路径的性能基准，防止功能扩展导致性能退化 | 阶段二起 |

---

## 附录：术语表

| 术语 | 说明 |
|------|------|
| **设备画像** | 针对特定厂商的设备配置模板（PTY、提示符、分页、命令等） |
| **任务链** | 多个任务按顺序或条件编排执行的工作流 |
| **合规检查** | 检查设备配置是否符合安全基线和最佳实践 |
| **拓扑快照** | 某一时间点的完整拓扑图状态 |
| **RBAC** | 基于角色的访问控制（Role-Based Access Control） |
| **Webhook** | 事件发生时向注册的 URL 发送 HTTP 回调 |
| **Cron 表达式** | 定时任务的时间调度表达式 |

---

> **文档维护说明**：本路线图应随项目发展定期更新。每个阶段完成后，应回顾并调整后续阶段的计划。
