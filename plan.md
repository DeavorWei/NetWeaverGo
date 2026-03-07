# NetWeaverGo 项目架构功能演进说明书

## 一、项目现状总结

| 维度         | 现状                                       |
| ------------ | ------------------------------------------ |
| **核心定位** | 网络设备批量自动化运维工具                 |
| **技术架构** | Go (Wails v3) + Vue 3 + Tailwind CSS       |
| **运行模式** | GUI 桌面端 + CLI 命令行                    |
| **核心能力** | SSH 批量执行、配置备份、设备管理、异常干预 |
| **并发能力** | Worker Pool 模式，最大 32 并发             |
| **设备支持** | Huawei/H3C/Cisco 等主流厂商                |

---

## 二、演进路线图

```
┌─────────────────────────────────────────────────────────────────────────────────────┐
│                              NetWeaverGo 演进路线图                                  │
├─────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                     │
│   阶段一（基础增强）    阶段二（能力扩展）    阶段三（企业级）    阶段四（生态构建）    │
│   ════════════════    ═════════════════    ═══════════════    ══════════════════    │
│   [1-2 个月]          [3-4 个月]           [5-6 个月]         [持续迭代]             │
│                                                                                     │
│   ┌──────────────┐   ┌──────────────┐   ┌──────────────┐   ┌──────────────┐        │
│   │ ✅ 任务调度   │   │ 📊 拓扑发现  │   │ 🔐 RBAC权限  │   │ 🔌 插件系统  │        │
│   │ ✅ 历史记录   │   │ 📡 SNMP采集  │   │ 🗄️ 数据持久化│   │ 🌐 REST API  │        │
│   │ ✅ 导出报告   │   │ 🔔 告警通知  │   │ 📈 性能监控  │   │ 🤖 自动化流水│        │
│   │ ✅ 设备分组   │   │ 📋 Playbook  │   │ 🏗️ 分布式   │   │ 📦 CI/CD集成 │        │
│   └──────────────┘   └──────────────┘   └──────────────┘   └──────────────┘        │
│                                                                                     │
└─────────────────────────────────────────────────────────────────────────────────────┘
```

---

## 三、阶段一：基础功能增强（1-2 个月）

### 3.1 任务历史与审计

**需求背景**：当前任务执行后无历史记录，无法追溯过往操作

**功能设计**：

```
┌─────────────────────────────────────────────────────────┐
│                    任务历史模块                          │
├─────────────────────────────────────────────────────────┤
│  数据结构：                                             │
│  ┌────────────────────────────────────────────────────┐ │
│  │ TaskRecord {                                       │ │
│  │   ID          string    // 任务ID                  │ │
│  │   Type        string    // 任务类型                │ │
│  │   StartTime   time.Time // 开始时间                │ │
│  │   EndTime     time.Time // 结束时间                │ │
│  │   Status      string    // success/failed/partial  │ │
│  │   DeviceCount int       // 设备数量                │ │
│  │   SuccessRate float64   // 成功率                  │ │
│  │   Config      string    // 任务配置快照            │ │
│  │   LogPath     string    // 日志文件路径            │ │
│  │ }                                                  │ │
│  └────────────────────────────────────────────────────┘ │
│                                                         │
│  新增文件：                                             │
│  ├── internal/history/                                 │
│  │   ├── store.go         // 历史记录存储              │
│  │   └── record.go        // 数据结构定义              │
│  └── frontend/src/views/History.vue                    │
└─────────────────────────────────────────────────────────┘
```

**前端界面**：

- 新增 `/history` 路由
- 任务列表展示：时间、类型、状态、设备数、成功率
- 点击任务可查看详细执行日志
- 支持按时间范围、任务类型筛选

### 3.2 设备分组管理

**需求背景**：设备数量增多时，扁平列表管理效率低

**功能设计**：

```go
// 分组数据结构
type DeviceGroup struct {
    ID          string   `json:"id"`
    Name        string   `json:"name"`
    Description string   `json:"description"`
    DeviceIPs   []string `json:"device_ips"`
    Tags        []string `json:"tags"`
}
```

**界面改进**：

- `Devices.vue` 左侧增加分组树形导航
- 支持拖拽设备到分组
- 任务执行时可按分组选择设备

### 3.3 报告导出增强

**当前能力**：CSV 格式进度报告

**增强方向**：
| 报告类型 | 格式 | 内容 |
|----------|------|------|
| 执行报告 | PDF/HTML | 包含统计图表、设备状态汇总 |
| 配置变更报告 | Diff | 对比备份配置变化 |
| 合规检查报告 | PDF | 配置基线检查结果 |

**新增文件**：

```
internal/report/
├── pdf.go         // PDF 报告生成
├── diff.go        // 配置差异对比
└── compliance.go  // 合规检查报告
```

### 3.4 命令模板库

**需求背景**：常用命令组合需要反复输入

**功能设计**：

```
┌─────────────────────────────────────────────────────────┐
│                   命令模板库                            │
├─────────────────────────────────────────────────────────┤
│  模板结构：                                             │
│  ┌────────────────────────────────────────────────────┐ │
│  │ CommandTemplate {                                  │ │
│  │   Name        string     // 模板名称               │ │
│  │   Category    string     // 分类：巡检/配置/排查   │ │
│  │   Vendor      string     // 厂商：huawei/cisco     │ │
│  │   Commands    []string   // 命令列表               │ │
│  │   Variables   []Variable // 可替换变量             │ │
│  │ }                                                  │ │
│  └────────────────────────────────────────────────────┘ │
│                                                         │
│  预置模板：                                             │
│  ├── 华为交换机日常巡检                                 │
│  ├── Cisco 接口状态检查                                 │
│  ├── H3C VLAN 配置备份                                  │
│  └── 批量修改密码                                       │
└─────────────────────────────────────────────────────────┘
```

---

## 四、阶段二：能力扩展（3-4 个月）

### 4.1 网络拓扑发现

**功能目标**：自动发现网络拓扑，可视化展示

**技术方案**：

```
┌─────────────────────────────────────────────────────────┐
│                   拓扑发现模块                          │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  发现协议：                                             │
│  ├── LLDP (Link Layer Discovery Protocol)              │
│  ├── CDP (Cisco Discovery Protocol)                    │
│  └── NDP (Huawei Discovery Protocol)                   │
│                                                         │
│  实现流程：                                             │
│  ┌─────────┐    ┌─────────┐    ┌─────────┐            │
│  │ SSH连接 │ -> │ 执行命令│ -> │ 解析结果│            │
│  │ 设备    │    │LLDP/CDP │    │ 提取邻居│            │
│  └─────────┘    └─────────┘    └─────────┘            │
│                      ↓                                  │
│               ┌─────────────┐                          │
│               │ 构建拓扑图  │                          │
│               └─────────────┘                          │
│                      ↓                                  │
│               ┌─────────────┐                          │
│               │ 前端可视化  │                          │
│               │ (D3.js/vis) │                          │
│               └─────────────┘                          │
│                                                         │
│  新增文件：                                             │
│  ├── internal/topology/                                │
│  │   ├── discovery.go    // 发现逻辑                   │
│  │   ├── parser.go       // 协议解析                   │
│  │   └── graph.go        // 图结构                     │
│  └── frontend/src/views/Topology.vue                   │
└─────────────────────────────────────────────────────────┘
```

**关键命令**：

```bash
# Huawei/H3C
display lldp neighbor
display lldp neighbor brief

# Cisco
show lldp neighbors
show cdp neighbors detail
```

### 4.2 SNMP 监控采集

**功能目标**：支持 SNMP 协议获取设备状态

**技术方案**：

```go
// SNMP 客户端封装
type SNMPClient struct {
    Target    string
    Community string
    Version   SNMPVersion  // v1/v2c/v3
    Timeout   time.Duration
}

// 常用 OID 采集
var StandardOIDs = map[string]string{
    "sysDescr":         "1.3.6.1.2.1.1.1.0",
    "sysUpTime":        "1.3.6.1.2.1.1.3.0",
    "ifNumber":         "1.3.6.1.2.1.2.1.0",
    "ifOperStatus":     "1.3.6.1.2.1.2.2.1.8",
    "cpuUsage":         "1.3.6.1.4.1.2011.6.3.4.1.2",  // Huawei
    "memoryUsage":      "1.3.6.1.4.1.2011.6.1.2.1.1.2", // Huawei
}
```

**新增依赖**：

```
go get github.com/gosnmp/gosnmp
```

### 4.3 Playbook 系统

**功能目标**：支持 YAML 格式的自动化剧本

**剧本格式设计**：

```yaml
# playbook.yaml
name: "交换机日常巡检"
description: "收集设备状态信息"
vendor: huawei
devices:
  groups: ["core-switch", "access-switch"]
  # 或指定 IP 列表
  # ips: ["192.168.1.1", "192.168.1.2"]

tasks:
  - name: "收集系统信息"
    commands:
      - "display version"
      - "display cpu"
      - "display memory"
    save_output: true

  - name: "检查接口状态"
    commands:
      - "display interface brief"
    conditions:
      - match: "down"
        severity: warning

  - name: "收集日志"
    commands:
      - "display logbuffer"
    filters:
      - level: warning
        keywords: ["error", "fail", "down"]

on_error: pause # pause/skip/abort
schedule: "0 9 * * 1-5" # 工作日早9点执行
```

**新增模块**：

```
internal/playbook/
├── parser.go      // YAML 解析
├── runner.go      // 剧本执行器
├── scheduler.go   // 定时调度
└── validator.go   // 剧本校验
```

### 4.4 告警通知系统

**功能目标**：任务异常时自动发送通知

**支持渠道**：
| 渠道 | 场景 | 实现方式 |
|------|------|----------|
| 邮件 | 任务报告 | SMTP |
| 企业微信 | 实时告警 | Webhook |
| 钉钉 | 实时告警 | Webhook |
| Telegram | 国际用户 | Bot API |
| Webhook | 自定义集成 | HTTP POST |

**告警规则配置**：

```yaml
# settings.yaml 新增
notifications:
  enabled: true
  channels:
    - type: wechat
      webhook: "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxx"
      events: ["error", "suspend"]
    - type: email
      smtp: "smtp.example.com:25"
      sender: "netweaver@example.com"
      recipients: ["admin@example.com"]
      events: ["finished"]
```

---

## 五、阶段三：企业级能力（5-6 个月）

### 5.1 数据持久化与数据库

**需求背景**：从文件存储转向数据库，支持更复杂的查询和关联

**数据库选型**：
| 方案 | 优势 | 劣势 |
|------|------|------|
| SQLite | 零配置、单文件 | 并发写入限制 |
| PostgreSQL | 功能强大、可扩展 | 需要独立部署 |
| **推荐：SQLite + 可选 PostgreSQL** | 兼顾易用性和扩展性 |

**数据模型设计**：

```sql
-- 设备表
CREATE TABLE devices (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    ip VARCHAR(45) NOT NULL UNIQUE,
    hostname VARCHAR(255),
    port INTEGER DEFAULT 22,
    vendor VARCHAR(50),
    model VARCHAR(100),
    username VARCHAR(100),
    password TEXT,  -- 加密存储
    group_id INTEGER,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    last_seen TIMESTAMP,
    FOREIGN KEY (group_id) REFERENCES device_groups(id)
);

-- 设备分组表
CREATE TABLE device_groups (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name VARCHAR(100) NOT NULL,
    parent_id INTEGER,
    description TEXT,
    FOREIGN KEY (parent_id) REFERENCES device_groups(id)
);

-- 任务执行记录表
CREATE TABLE task_records (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id VARCHAR(36) NOT NULL,
    type VARCHAR(50),
    status VARCHAR(20),
    started_at TIMESTAMP,
    finished_at TIMESTAMP,
    total_devices INTEGER,
    success_count INTEGER,
    config_snapshot TEXT
);

-- 设备执行明细表
CREATE TABLE device_executions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_record_id INTEGER,
    device_id INTEGER,
    status VARCHAR(20),
    error_message TEXT,
    duration_ms INTEGER,
    output TEXT,
    FOREIGN KEY (task_record_id) REFERENCES task_records(id),
    FOREIGN KEY (device_id) REFERENCES devices(id)
);

-- 配置备份记录表
CREATE TABLE config_backups (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    device_id INTEGER,
    file_path TEXT,
    file_size INTEGER,
    checksum VARCHAR(64),
    backed_at TIMESTAMP,
    FOREIGN KEY (device_id) REFERENCES devices(id)
);
```

**新增模块**：

```
internal/database/
├── database.go    // 数据库连接管理
├── migrations/    // 数据库迁移
├── models/        // 数据模型
└── repository/    // 数据访问层
```

### 5.2 用户权限管理 (RBAC)

**功能目标**：多用户、角色权限控制

**权限模型**：

```
┌─────────────────────────────────────────────────────────┐
│                      RBAC 模型                          │
├─────────────────────────────────────────────────────────┤
│                                                         │
│   ┌─────────┐     ┌─────────┐     ┌─────────────┐     │
│   │  User   │ --> │  Role   │ --> │ Permission  │     │
│   └─────────┘     └─────────┘     └─────────────┘     │
│                                                         │
│   角色：                                                │
│   ├── admin      - 完全权限                            │
│   ├── operator   - 执行任务、查看结果                   │
│   ├── viewer     - 只读访问                            │
│   └── auditor    - 查看审计日志                        │
│                                                         │
│   权限点：                                              │
│   ├── device:read       设备查看                       │
│   ├── device:write      设备编辑                       │
│   ├── device:delete     设备删除                       │
│   ├── task:execute      任务执行                       │
│   ├── task:stop         任务停止                       │
│   ├── config:backup     配置备份                       │
│   ├── report:export     报告导出                       │
│   └── settings:manage   系统配置                       │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

**用户表设计**：

```sql
CREATE TABLE users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username VARCHAR(100) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    email VARCHAR(255),
    role_id INTEGER,
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMP,
    last_login TIMESTAMP,
    FOREIGN KEY (role_id) REFERENCES roles(id)
);
```

### 5.3 配置变更追踪

**功能目标**：自动检测配置变化，生成变更报告

**实现方案**：

```
┌─────────────────────────────────────────────────────────┐
│                  配置变更追踪流程                        │
├─────────────────────────────────────────────────────────┤
│                                                         │
│   ┌─────────────┐                                      │
│   │ 定时备份任务│                                      │
│   └──────┬──────┘                                      │
│          ↓                                             │
│   ┌─────────────┐    ┌─────────────┐                  │
│   │ 下载配置文件│ -> │ 计算MD5校验 │                  │
│   └─────────────┘    └──────┬──────┘                  │
│                             ↓                          │
│                      ┌─────────────┐                   │
│                      │ 对比历史版本│                   │
│                      └──────┬──────┘                   │
│                             ↓                          │
│              ┌──────────────┼──────────────┐          │
│              ↓              ↓              ↓          │
│        ┌─────────┐   ┌─────────┐   ┌─────────┐       │
│        │ 无变化  │   │ 有变化  │   │ 首次备份│       │
│        └─────────┘   └────┬────┘   └─────────┘       │
│                           ↓                           │
│                    ┌─────────────┐                    │
│                    │ 生成Diff报告│                    │
│                    └──────┬──────┘                    │
│                           ↓                           │
│                    ┌─────────────┐                    │
│                    │ 发送通知    │                    │
│                    └─────────────┘                    │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

**Diff 报告示例**：

```diff
设备: 192.168.1.1 (Core-SW-01)
时间: 2026-03-07 10:30:00
变更行数: +5, -2

--- backup_20260306.cfg
+++ backup_20260307.cfg
@@ -45,7 +45,10 @@
 interface Vlanif100
  ip address 192.168.100.1 255.255.255.0
- description Management
+ description Management-Network
+ dhcp select interface
+ dhcp server excluded-ip-address 192.168.100.1 192.168.100.10
+ dhcp server lease day 7
```

### 5.4 性能监控仪表盘

**功能目标**：实时监控系统运行状态

**监控指标**：

```
┌─────────────────────────────────────────────────────────┐
│                    监控指标面板                         │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  系统指标：                                             │
│  ├── CPU 使用率                                        │
│  ├── 内存使用量                                        │
│  ├── Goroutine 数量                                    │
│  └── GC 停顿时间                                       │
│                                                         │
│  业务指标：                                             │
│  ├── 活跃连接数                                        │
│  ├── 任务队列深度                                      │
│  ├── 任务成功率 (滑动窗口)                             │
│  ├── 平均执行时间                                      │
│  └── 错误类型分布                                      │
│                                                         │
│  设备指标：                                             │
│  ├── 在线设备数量                                      │
│  ├── 离线设备 TOP10                                    │
│  ├── 执行失败 TOP10                                    │
│  └── 设备响应延迟分布                                  │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

**前端组件**：

- 使用 ECharts 或 ApexCharts 渲染图表
- 实时数据通过 WebSocket 推送
- 支持自定义仪表盘布局

---

## 六、阶段四：生态构建（持续迭代）

### 6.1 插件系统

**功能目标**：支持第三方扩展

**插件架构**：

```
┌─────────────────────────────────────────────────────────┐
│                     插件系统架构                        │
├─────────────────────────────────────────────────────────┤
│                                                         │
│   ┌─────────────────────────────────────────────────┐  │
│   │                   NetWeaverGo Core               │  │
│   │  ┌─────────────────────────────────────────────┐│  │
│   │  │              Plugin Manager                  ││  │
│   │  │  ├── Load()   加载插件                       ││  │
│   │  │  ├── Unload() 卸载插件                       ││  │
│   │  │  └── List()   插件列表                       ││  │
│   │  └─────────────────────────────────────────────┘│  │
│   └─────────────────────────────────────────────────┘  │
│                           ↑                             │
│                           │ Plugin Interface            │
│          ┌────────────────┼────────────────┐           │
│          ↓                ↓                ↓           │
│   ┌────────────┐  ┌────────────┐  ┌────────────┐      │
│   │ 网络设备   │  │ 告警通知   │  │ 报告生成   │      │
│   │ 厂商插件   │  │ 渠道插件   │  │ 格式插件   │      │
│   │            │  │            │  │            │      │
│   │ Juniper    │  │ Slack      │  │ Word       │      │
│   │ Arista     │  │ PagerDuty  │  │ Excel      │      │
│   │ Fortinet   │  │ SMS        │  │ JSON       │      │
│   └────────────┘  └────────────┘  └────────────┘      │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

**插件接口定义**：

```go
// 插件接口
type Plugin interface {
    // 插件元信息
    Info() PluginInfo

    // 初始化
    Init(config map[string]interface{}) error

    // 启动
    Start() error

    // 停止
    Stop() error
}

// 设备驱动插件接口
type DeviceDriverPlugin interface {
    Plugin

    // 支持的厂商列表
    SupportedVendors() []string

    // 自定义命令解析
    ParseOutput(command, output string) (map[string]interface{}, error)

    // 自定义提示符检测
    DetectPrompt(output string) (bool, string)
}

// 告警渠道插件接口
type NotificationPlugin interface {
    Plugin

    // 发送通知
    Send(ctx context.Context, notification *Notification) error

    // 支持的通知类型
    SupportedTypes() []string
}
```

### 6.2 REST API 服务

**功能目标**：提供完整的 REST API，支持外部系统集成

**API 设计**：

```
┌─────────────────────────────────────────────────────────┐
│                      REST API                          │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  认证：                                                 │
│  POST   /api/v1/auth/login          登录获取Token      │
│  POST   /api/v1/auth/logout         注销登录           │
│  POST   /api/v1/auth/refresh        刷新Token          │
│                                                         │
│  设备管理：                                             │
│  GET    /api/v1/devices             设备列表           │
│  POST   /api/v1/devices             新增设备           │
│  GET    /api/v1/devices/{id}        设备详情           │
│  PUT    /api/v1/devices/{id}        更新设备           │
│  DELETE /api/v1/devices/{id}        删除设备           │
│  POST   /api/v1/devices/import      批量导入           │
│  GET    /api/v1/devices/export      导出设备           │
│                                                         │
│  分组管理：                                             │
│  GET    /api/v1/groups              分组列表           │
│  POST   /api/v1/groups              新增分组           │
│  PUT    /api/v1/groups/{id}         更新分组           │
│  DELETE /api/v1/groups/{id}         删除分组           │
│                                                         │
│  任务管理：                                             │
│  POST   /api/v1/tasks               创建任务           │
│  GET    /api/v1/tasks               任务列表           │
│  GET    /api/v1/tasks/{id}          任务详情           │
│  POST   /api/v1/tasks/{id}/stop     停止任务           │
│  GET    /api/v1/tasks/{id}/logs     任务日志           │
│                                                         │
│  Playbook：                                             │
│  GET    /api/v1/playbooks           Playbook列表       │
│  POST   /api/v1/playbooks           创建Playbook       │
│  PUT    /api/v1/playbooks/{id}      更新Playbook       │
│  DELETE /api/v1/playbooks/{id}      删除Playbook       │
│  POST   /api/v1/playbooks/{id}/run  执行Playbook       │
│                                                         │
│  备份管理：                                             │
│  GET    /api/v1/backups             备份列表           │
│  POST   /api/v1/backups             触发备份           │
│  GET    /api/v1/backups/{id}        备份详情           │
│  GET    /api/v1/backups/{id}/download 下载配置         │
│  GET    /api/v1/backups/{id}/diff   配置对比           │
│                                                         │
│  WebSocket：                                            │
│  WS     /api/v1/ws/events           事件订阅           │
│  WS     /api/v1/ws/terminal/{ip}    实时终端           │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

**API 服务模式**：

```go
// main.go 新增 API 服务模式
func main() {
    if *apiMode {
        // 纯 API 服务模式（无 GUI）
        server := api.NewAPIServer(cfg)
        server.Start()
    } else if *cliMode {
        // CLI 模式
        cli.Run()
    } else {
        // GUI 模式
        app := wails.NewApp()
        app.Run()
    }
}
```

### 6.3 CI/CD 集成

**功能目标**：与主流 CI/CD 平台集成

**集成场景**：

```
┌─────────────────────────────────────────────────────────┐
│                    CI/CD 集成场景                       │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  场景1：网络配置即代码                     │
│  ┌─────────────────────────────────────────────────┐  │
│  │ Git Push → CI Trigger → NetWeaverGo API         │  │
│  │         → 执行配置变更 → 结果反馈                │  │
│  └─────────────────────────────────────────────────┘  │
│                                                         │
│  场景2：自动化测试                                      │
│  ┌─────────────────────────────────────────────────┐  │
│  │ 定时任务 → NetWeaverGo 执行连通性测试            │  │
│  │          → 结果写入 InfluxDB                    │  │
│  │          → Grafana 展示可用性趋势               │  │
│  └─────────────────────────────────────────────────┘  │
│                                                         │
│  场景3：配置审计                                        │
│  ┌─────────────────────────────────────────────────┐  │
│  │ 定时备份 → 配置 Diff → 违规检测                  │  │
│  │          → 创建 Jira 工单                       │  │
│  └─────────────────────────────────────────────────┘  │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

### 6.4 分布式部署支持

**功能目标**：支持大规模设备管理

**架构设计**：

```
┌─────────────────────────────────────────────────────────┐
│                    分布式架构                           │
├─────────────────────────────────────────────────────────┤
│                                                         │
│                    ┌─────────────┐                     │
│                    │   Master    │                     │
│                    │  (调度中心) │                     │
│                    └──────┬──────┘                     │
│                           │                             │
│         ┌─────────────────┼─────────────────┐         │
│         ↓                 ↓                 ↓         │
│   ┌───────────┐    ┌───────────┐    ┌───────────┐    │
│   │  Agent-1  │    │  Agent-2  │    │  Agent-3  │    │
│   │ (区域A)   │    │ (区域B)   │    │ (区域C)   │    │
│   │           │    │           │    │           │    │
│   │ 设备1-100 │    │ 设备101-200│   │ 设备201-300│   │
│   └───────────┘    └───────────┘    └───────────┘    │
│                                                         │
│   组件职责：                                           │
│   ├── Master: 任务调度、状态聚合、配置管理            │
│   └── Agent: 本地执行、心跳上报、日志收集            │
│                                                         │
│   通信方式：                                           │
│   ├── gRPC: 控制指令                                  │
│   └── NATS/Redis PubSub: 事件广播                    │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

---

## 七、技术债务与架构优化

### 7.1 代码重构计划

| 重构项   | 现状               | 目标                      | 优先级 |
| -------- | ------------------ | ------------------------- | ------ |
| 接口抽象 | 部分模块耦合紧     | 定义接口，支持 Mock 测试  | 高     |
| 错误处理 | 错误信息不够结构化 | 统一错误码和错误类型      | 高     |
| 日志规范 | DEBUG 级别粒度不够 | 结构化日志，支持 Trace ID | 中     |
| 配置管理 | 多文件分散         | 统一配置中心，支持热更新  | 中     |

### 7.2 测试覆盖计划

```
┌─────────────────────────────────────────────────────────┐
│                     测试策略                            │
├─────────────────────────────────────────────────────────┤
│                                                         │
│   单元测试：                                           │
│   ├── Matcher 正则匹配逻辑                            │
│   ├── Config 解析器                                   │
│   ├── Playbook 解析器                                 │
│   └── 目标覆盖率: 70%                                 │
│                                                         │
│   集成测试：                                           │
│   ├── SSH 连接流程 (Mock Server)                      │
│   ├── Engine 并发调度                                 │
│   └── API 端点测试                                    │
│                                                         │
│   E2E 测试：                                           │
│   ├── GUI 完整工作流                                  │
│   ├── CLI 命令执行                                    │
│   └── 使用真实设备或容器化模拟器                      │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

---

## 八、优先级矩阵

```
                    重要性
           高                              低
    ┌──────────────────┬──────────────────┐
    │                  │                  │
 高 │ 阶段一基础增强   │ 阶段三企业级     │
    │ - 任务历史       │ - 数据持久化     │
    │ - 设备分组       │ - RBAC权限       │
    │ - 报告导出       │                  │
 紧 │                  │                  │
 迫 ┼──────────────────┼──────────────────┤
 性 │                  │                  │
    │ 阶段二能力扩展   │ 阶段四生态构建   │
    │ - SNMP采集       │ - 插件系统       │
    │ - 告警通知       │ - 分布式部署     │
    │ - Playbook       │                  │
 低 │                  │                  │
    └──────────────────┴──────────────────┘
```

---

## 九、资源估算

| 阶段   | 预计工时     | 核心人力 | 技术风险 |
| ------ | ------------ | -------- | -------- |
| 阶段一 | 160-200 人时 | 1 全栈   | 低       |
| 阶段二 | 240-320 人时 | 1-2 人   | 中       |
| 阶段三 | 320-400 人时 | 2-3 人   | 中高     |
| 阶段四 | 持续迭代     | 2-3 人   | 高       |

---

## 十、总结

NetWeaverGo 作为网络自动化运维工具，具备良好的架构基础。通过四个阶段的演进：

1. **夯实基础**：完善历史记录、分组管理、报告导出等基础能力
2. **扩展能力**：引入 SNMP、拓扑发现、Playbook 等高级功能
3. **企业升级**：实现数据持久化、权限管理、分布式支持
4. **生态构建**：打造插件生态、API 平台、CI/CD 集成

最终目标是成为**企业级网络自动化运维平台**，支持大规模设备管理、灵活的自动化编排、完善的审计追踪能力。

---

如需进一步细化某个阶段的具体实现方案，请告知。
