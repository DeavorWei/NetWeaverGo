约束收敛：

- 技术栈：**Wails3 + Vue + Golang**
- 采集协议：**当前仅 SSH CLI**
- 厂商优先级：**华为**
- 网络规模：**100 台左右**
- 设备类型：**交换机、路由器、防火墙、服务器**
- 拓扑粒度：**设备 + 逻辑聚合口 + 成员口可展开**
- 规划表来源：**固定格式 Excel**
- CLI 解析方案：**使用 `gotextfsm`**
- 后续扩展：为 **SNMP / NETCONF / API** 预留接口

`gotextfsm` 是 Google `TextFSM` 的 Go 实现，定位就是“基于模板状态机解析半结构化文本”，原本就是为网络设备 CLI 输出这类场景设计；其文档页显示模块路径为 `github.com/sirikothe/gotextfsm`，可见版本为 `v1.1.0`。`TextFSM` 官方 Wiki 也明确说明：它通过“模板 + 输入文本”返回记录列表，适合路由器、交换机等 CLI 输出解析。 ([Go Packages][1])

---

# 1. 项目目标

系统分两阶段建设：

## 阶段一：网络拓扑自动还原

通过 SSH 登录设备，采集 CLI 输出，经 `gotextfsm` 解析后，还原：

- 设备列表
- 接口列表
- LLDP 邻接关系
- 聚合链路关系
- 基于 LLDP + MAC 地址表的实际连接关系
- 前端拓扑图展示

## 阶段二：与网络连接规划表进行一致性比对

导入固定格式 Excel，自动分析：

- 实际连接是否与规划一致
- 是否存在缺失链路
- 是否存在错连、串接、临时链路
- 是否存在聚合口不一致
- 是否存在单边发现、弱证据链路
- 生成差异报告并支持导出

---

# 2. 总体架构

系统建议分为 8 层。

## 2.1 传输层 `transport`

负责“如何连接设备”。

职责：

- SSH 建连
- 登录认证
- 命令执行
- 分页处理
- 超时处理
- 并发控制
- 原始输出回收

只负责连接与执行，不负责业务解析。

---

## 2.2 采集层 `collector`

负责“采哪些命令”。

职责：

- 根据厂商和设备角色选择命令集
- 驱动 transport 执行命令
- 形成原始快照 `RawDeviceSnapshot`

---

## 2.3 解析层 `parser`

负责“把 CLI 文本变成结构化记录”。

这里统一采用：

- **`gotextfsm` 解析模板**
- 少量 Go 后处理逻辑
- 不使用 Python 子进程
- 不依赖外部 Python 环境

---

## 2.4 标准化层 `normalize`

负责“统一格式”。

职责：

- 设备标识归一化
- 接口名归一化
- MAC 地址归一化
- LLDP 远端字段归一化
- 聚合关系归一化

---

## 2.5 拓扑推理层 `topology`

负责“谁和谁相连”。

职责：

- 基于 LLDP 构建确定链路
- 基于 MAC 地址表辅助推断
- 处理逻辑聚合口与成员口映射
- 生成逻辑图与物理展开图
- 为每条边给出置信度与证据链

---

## 2.6 比对层 `compare`

负责“规划表与实际拓扑比对”。

职责：

- Excel 导入
- 字段校验
- 设备/接口映射
- 链路对比
- 差异分类
- 生成报告

---

## 2.7 存储层 `storage`

建议使用 SQLite。

职责：

- 任务持久化
- 原始命令输出保存
- 解析结果保存
- 拓扑结果保存
- 规划表保存
- 差异报告保存

---

## 2.8 展示层 `presentation`

- Go：暴露 Wails 服务接口
- Vue：展示任务、拓扑、详情、差异报告

---

# 3. 推荐目录结构

```bash
/internal
  /app
    /service
      discovery_service.go
      topology_service.go
      plan_compare_service.go
      report_service.go

  /transport
    transport.go
    /ssh
      client.go
      session.go
      pager.go

  /collector
    collector.go
    huawei_collector.go
    command_profile.go

  /parser
    parser.go
    registry.go
    gotextfsm_engine.go
    mapper.go
    template_loader.go
    /templates
      /huawei
        display_version.textfsm
        display_interface_brief.textfsm
        display_interface_detail.textfsm
        display_lldp_neighbor_verbose.textfsm
        display_mac_address.textfsm
        display_eth_trunk.textfsm
        display_arp_all.textfsm

  /normalize
    device_identity.go
    interface_name.go
    mac.go
    lldp.go
    agg.go

  /topology
    builder.go
    lldp_matcher.go
    fdb_infer.go
    agg_resolver.go
    confidence.go
    validator.go

  /compare
    excel_importer.go
    matcher.go
    diff_engine.go

  /report
    excel_exporter.go
    html_exporter.go
    json_exporter.go

  /storage
    /sqlite
      db.go
      migrations.go
      task_repo.go
      device_repo.go
      topology_repo.go

  /domain
    device.go
    interface.go
    lldp.go
    fdb.go
    arp.go
    agg.go
    topology.go
    planned_link.go
    diff.go
```

---

# 4. 解析方案：统一使用 gotextfsm

## 4.1 设计原则

`gotextfsm` 只负责把 CLI 文本解析成“表格记录”，业务代码不直接依赖模板细节。

也就是说，解析层输出先是：

```go
[]map[string]string
```

然后再由 mapper 转成强类型结构体。

---

## 4.2 解析接口定义

```go
type CliParser interface {
    Parse(commandKey string, rawText string) ([]map[string]string, error)
}
```

`commandKey` 不直接使用原始命令，而使用内部标准键，例如：

- `huawei.display.version`
- `huawei.display.interface.brief`
- `huawei.display.interface.detail`
- `huawei.display.lldp.neighbor.verbose`
- `huawei.display.mac-address`
- `huawei.display.eth-trunk`
- `huawei.display.arp.all`

---

## 4.3 gotextfsm 引擎包装

```go
type GoTextFSMParser struct {
    loader   TemplateLoader
    registry TemplateRegistry
}

func (p *GoTextFSMParser) Parse(commandKey string, rawText string) ([]map[string]string, error) {
    tmpl, err := p.loader.Load(commandKey)
    if err != nil {
        return nil, err
    }

    rows, err := ParseWithGoTextFSM(tmpl, rawText)
    if err != nil {
        return nil, err
    }

    return rows, nil
}
```

---

## 4.4 模板管理

建议使用 `embed` 内嵌模板文件：

```go
//go:embed templates/**/*.textfsm
var templateFS embed.FS
```

优点：

- 打包简单
- 版本一致
- 不依赖外部目录
- 适合桌面程序

---

## 4.5 模板注册表

建议维护命令键与模板路径映射：

```go
var HuaweiTemplateRegistry = map[string]string{
    "huawei.display.version":               "templates/huawei/display_version.textfsm",
    "huawei.display.interface.brief":       "templates/huawei/display_interface_brief.textfsm",
    "huawei.display.interface.detail":      "templates/huawei/display_interface_detail.textfsm",
    "huawei.display.lldp.neighbor.verbose": "templates/huawei/display_lldp_neighbor_verbose.textfsm",
    "huawei.display.mac-address":           "templates/huawei/display_mac_address.textfsm",
    "huawei.display.eth-trunk":             "templates/huawei/display_eth_trunk.textfsm",
    "huawei.display.arp.all":               "templates/huawei/display_arp_all.textfsm",
}
```

---

## 4.6 mapper 层

建议增加一层 `mapper`，专门把模板结果转成领域模型。

```go
type ResultMapper interface {
    ToDeviceInfo(rows []map[string]string) (*Device, error)
    ToInterfaces(rows []map[string]string) ([]*Interface, error)
    ToLLDPNeighbors(rows []map[string]string) ([]*LLDPNeighbor, error)
    ToFDBEntries(rows []map[string]string) ([]*FDBEntry, error)
    ToAggregateGroups(rows []map[string]string) ([]*AggregateGroup, error)
    ToARPEntries(rows []map[string]string) ([]*ARPEntry, error)
}
```

---

# 5. 华为首版采集命令集

为 100 台规模、华为优先的场景，建议首版采集如下命令。

## 5.1 设备基础信息

```text
display version
display current-configuration | include sysname
display esn
display device
```

目标字段：

- hostname / sysname
- model
- version
- serial number / esn
- 设备类型辅助特征

---

## 5.2 接口基础信息

```text
display interface brief
display interface
```

目标字段：

- 接口名
- up/down
- protocol 状态
- 描述
- IP
- MAC
- L2/L3 属性
- 虚拟/管理接口识别

---

## 5.3 LLDP 邻居信息

```text
display lldp neighbor verbose
```

目标字段：

- 本地接口
- 远端系统名
- 远端管理地址
- 远端 chassis id
- 远端 port id
- 远端 capability

这是拓扑恢复的第一核心数据源。

---

## 5.4 MAC 地址表

```text
display mac-address
```

目标字段：

- MAC
- VLAN
- 出接口
- 动态/静态类型

这是 LLDP 缺失时的重要辅助依据。

---

## 5.5 聚合信息

```text
display eth-trunk
display eth-trunk verbose
```

目标字段：

- 逻辑聚合接口
- 成员接口
- 状态
- LACP 模式

这是“逻辑聚合口 + 成员口展开”的关键数据源。

---

## 5.6 ARP 表

```text
display arp all
```

目标字段：

- IP
- MAC
- 接口

用于：

- 辅助识别服务器
- 辅助识别三层邻居
- 辅助推断被动节点

---

# 6. 数据模型设计

---

## 6.1 设备模型

```go
type Device struct {
    ID             string
    Hostname       string
    NormalizedName string
    MgmtIP         string
    Vendor         string
    Model          string
    SerialNumber   string
    ChassisID      string
    DeviceType     string // switch/router/firewall/server/server-inferred
    Attributes     map[string]string
}
```

---

## 6.2 接口模型

```go
type Interface struct {
    ID               string
    DeviceID         string
    Name             string
    NormalizedName   string
    Description      string
    Type             string // physical, aggregate, member, loopback, vlanif, mgmt
    AdminStatus      string
    OperStatus       string
    MAC              string
    IPv4             []string
    ParentAggregate  string
    MemberInterfaces []string
    IsL3             bool
    IsMgmt           bool
    IsVirtual        bool
}
```

---

## 6.3 LLDP 模型

```go
type LLDPNeighbor struct {
    LocalDeviceID      string
    LocalInterface     string
    RemoteSystemName   string
    RemoteMgmtIP       string
    RemoteChassisID    string
    RemotePortID       string
    RemotePortDesc     string
    RemoteCapabilities []string
}
```

---

## 6.4 MAC 地址表模型

```go
type FDBEntry struct {
    DeviceID     string
    VLAN         int
    MAC          string
    OutInterface string
    EntryType    string
}
```

---

## 6.5 聚合模型

```go
type AggregateGroup struct {
    DeviceID         string
    LogicalIf        string
    MemberInterfaces []string
    Mode             string
    Status           string
}
```

---

## 6.6 ARP 模型

```go
type ARPEntry struct {
    DeviceID   string
    IP         string
    MAC        string
    Interface  string
}
```

---

## 6.7 拓扑边模型

```go
type TopologyEdge struct {
    ID               string
    ADeviceID        string
    AIf              string
    BDeviceID        string
    BIf              string

    LogicalAIf       string
    LogicalBIf       string

    EdgeType         string   // physical, logical-aggregate, inferred, server-access
    Status           string   // confirmed, semi-confirmed, inferred, conflict
    Confidence       float64
    DiscoveryMethods []string
    Evidence         []EdgeEvidence
}
```

---

## 6.8 证据模型

```go
type EdgeEvidence struct {
    Type      string
    DeviceID  string
    Interface string
    Command   string
    Summary   string
    RawRefID  string
}
```

---

# 7. 标准化设计

---

## 7.1 设备标识归一化

同一设备可能被多种字段描述：

- sysname
- 管理 IP
- chassis id
- serial number

建议设备合并优先级：

1. SerialNumber
2. MgmtIP
3. ChassisID
4. Normalized Hostname

---

## 7.2 接口名归一化

必须统一接口名，否则 LLDP 和规划表匹配会大量失败。

示例归一化：

- `GigabitEthernet1/0/1` → `GE1/0/1`
- `XGigabitEthernet1/0/1` → `XGE1/0/1`
- `Eth-Trunk10` → `Trunk10`

建议封装：

```go
func NormalizeInterfaceName(raw string, vendor string) string
```

---

## 7.3 MAC 地址归一化

统一成一个格式，例如：

```text
00e0-fc12-3456
```

---

## 7.4 聚合关系归一化

统一把物理成员口挂接到逻辑聚合口下：

- `GE1/0/1`, `GE1/0/2` → `Eth-Trunk10`

---

# 8. 拓扑恢复算法设计

核心原则：**多证据、分级置信度、逻辑链路优先展示**。

---

## 8.1 一级：双向 LLDP 确认链路

若满足：

- A 接口 X 的 LLDP 指向 B 接口 Y
- B 接口 Y 的 LLDP 指向 A 接口 X

则生成：

- `status = confirmed`
- `confidence = 1.0`

这是最可靠链路。

---

## 8.2 二级：单向 LLDP 半确认链路

若满足：

- A:X 指向 B:Y
- 但 B 没有反向记录

则生成：

- `status = semi-confirmed`
- `confidence = 0.75`

备注说明：

- 单向发现
- 可能是对端未开启 LLDP 或采集不完整

---

## 8.3 三级：聚合逻辑链路归并

如果物理成员口属于同一聚合组，则将物理链路归并为逻辑链路。

例如：

- 物理：`GE1/0/1`, `GE1/0/2`
- 逻辑：`Eth-Trunk10`

前端默认显示：

- `Trunk10 ↔ Trunk1`

点击展开后再显示成员链路。

---

## 8.4 四级：MAC 地址表辅助推断

当 LLDP 缺失时，使用 FDB 作为辅助证据，不直接作为绝对真相。

### 场景 A：服务器接入口推断

若某交换机接口满足：

- access 口
- 仅学习到 1~2 个 MAC
- 无 LLDP 邻居
- ARP 能映射出单个业务 IP
- 描述字段像服务器端口

则生成一个被动节点：

- `DeviceType = server-inferred`
- `EdgeType = server-access`
- `Status = inferred`

### 场景 B：交换设备间弱推断

若 trunk 口上学习到大量与另一台设备强相关的 MAC，并且没有更合理的候选链路，则可生成：

- `status = inferred`
- `confidence = 0.4 ~ 0.6`

但 UI 必须和 confirmed 明确区分。

---

## 8.5 链路去重

`A:X ↔ B:Y` 与 `B:Y ↔ A:X` 视为同一条边。

建议生成无向标准键：

```text
min(A:X, B:Y) + "|" + max(A:X, B:Y)
```

---

## 8.6 冲突处理

若同一接口出现多个候选对端：

- 标记 `conflict`
- 不自动升为 confirmed
- 在 Evidence 中记录冲突来源

---

# 9. 规划表设计

---

## 9.1 Excel 固定模板

建议固定如下列：

| 本端设备名 | 本端管理IP | 本端接口 | 对端设备名 | 对端管理IP | 对端接口 | 链路类型 | 备注 |
| ---------- | ---------- | -------- | ---------- | ---------- | -------- | -------- | ---- |

说明：

- 管理 IP 用于精确匹配
- 接口名用于标准化匹配
- 链路类型用于判断 aggregate/trunk/access
- 备注用于报告呈现

---

## 9.2 规划表内部模型

```go
type PlannedLink struct {
    ID               string
    LocalDeviceName  string
    LocalMgmtIP      string
    LocalInterface   string
    PeerDeviceName   string
    PeerMgmtIP       string
    PeerInterface    string
    LinkType         string
    Remark           string
}
```

---

## 9.3 Excel 导入规则

导入时做以下校验：

- 必填列不能为空
- 管理 IP 格式校验
- 接口名标准化
- A-B 与 B-A 去重
- 空白行跳过
- 同名设备告警

---

# 10. 规划表与实际拓扑比对设计

---

## 10.1 匹配顺序

### 设备匹配

优先级：

1. 管理 IP
2. 标准化设备名
3. 用户维护的别名表

### 接口匹配

优先级：

1. 标准化接口名
2. 聚合逻辑口
3. 成员口归属到聚合口后的映射

---

## 10.2 链路匹配

规划链路与实际链路都转成无向边，再比较。

例如：

- `Core1:Trunk10 ↔ Agg1:Trunk1`

---

## 10.3 差异类型

建议固定这些枚举：

```go
type DiffType string

const (
    DiffMatched             DiffType = "matched"
    DiffMissingLink         DiffType = "missing_link"
    DiffUnexpectedLink      DiffType = "unexpected_link"
    DiffDeviceMismatch      DiffType = "device_mismatch"
    DiffInterfaceMismatch   DiffType = "interface_mismatch"
    DiffAggregationMismatch DiffType = "aggregation_mismatch"
    DiffOneSideOnly         DiffType = "one_side_only"
    DiffAmbiguous           DiffType = "ambiguous"
)
```

---

## 10.4 差异项模型

```go
type DiffItem struct {
    ID              string
    Type            DiffType
    Severity        string
    PlannedLinkID   string
    ActualEdgeID    string
    ExpectedText    string
    ActualText      string
    Evidence        []string
    Suggestion      string
}
```

---

## 10.5 报告内容

报告分两部分：

### 汇总

- 规划链路总数
- 实际链路总数
- 完全匹配数
- 缺失链路数
- 额外链路数
- 接口不一致数
- 聚合不一致数
- 匹配率

### 明细

逐条差异项，包含：

- 类型
- 严重级别
- 规划链路
- 实际链路
- 说明
- 证据
- 修复建议

---

# 11. 存储设计（SQLite）

建议至少这些表。

## `tasks`

```sql
id
name
status
started_at
finished_at
target_count
success_count
failed_count
```

## `devices`

```sql
id
task_id
hostname
normalized_name
mgmt_ip
vendor
model
serial_number
chassis_id
device_type
```

## `interfaces`

```sql
id
device_id
name
normalized_name
description
type
admin_status
oper_status
mac
parent_aggregate
is_l3
is_mgmt
is_virtual
```

## `lldp_neighbors`

```sql
id
task_id
local_device_id
local_interface
remote_system_name
remote_mgmt_ip
remote_chassis_id
remote_port_id
remote_port_desc
raw_ref_id
```

## `fdb_entries`

```sql
id
task_id
device_id
vlan
mac
out_interface
entry_type
```

## `aggregate_groups`

```sql
id
task_id
device_id
logical_if
mode
status
```

## `aggregate_members`

```sql
id
aggregate_id
member_interface
```

## `arp_entries`

```sql
id
task_id
device_id
ip
mac
interface
```

## `topology_edges`

```sql
id
task_id
a_device_id
a_if
b_device_id
b_if
logical_a_if
logical_b_if
edge_type
status
confidence
methods_json
evidence_json
```

## `raw_command_outputs`

```sql
id
task_id
device_id
command
raw_text
parser_name
parse_status
created_at
```

## `plan_files`

```sql
id
name
version
uploaded_at
template_version
```

## `planned_links`

```sql
id
plan_file_id
local_device_name
local_mgmt_ip
local_interface
peer_device_name
peer_mgmt_ip
peer_interface
link_type
remark
```

## `diff_reports`

```sql
id
task_id
plan_file_id
summary_json
created_at
```

## `diff_items`

```sql
id
report_id
diff_type
severity
planned_link_id
actual_edge_id
expected_text
actual_text
evidence_json
suggestion
```

---

# 12. Wails 后端 API 设计

---

## 12.1 采集任务接口

```go
type DiscoveryService struct {}

func (s *DiscoveryService) StartDiscovery(req StartDiscoveryRequest) (TaskStartResponse, error)
func (s *DiscoveryService) GetTaskStatus(taskID string) (TaskStatusResponse, error)
func (s *DiscoveryService) GetTaskLogs(taskID string) ([]TaskLogItem, error)
func (s *DiscoveryService) CancelTask(taskID string) error
```

---

## 12.2 拓扑接口

```go
type TopologyService struct {}

func (s *TopologyService) BuildTopology(taskID string) (TopologyBuildResult, error)
func (s *TopologyService) GetTopologyGraph(taskID string) (TopologyViewData, error)
func (s *TopologyService) GetDeviceDetail(taskID string, deviceID string) (DeviceDetailView, error)
func (s *TopologyService) GetEdgeDetail(taskID string, edgeID string) (EdgeDetailView, error)
```

---

## 12.3 规划比对接口

```go
type PlanCompareService struct {}

func (s *PlanCompareService) ImportPlanExcel(filePath string) (PlanImportResult, error)
func (s *PlanCompareService) Compare(taskID string, planID string) (CompareResult, error)
func (s *PlanCompareService) GetDiffReport(reportID string) (DiffReportView, error)
func (s *PlanCompareService) ExportDiffReport(reportID string, format string) (string, error)
```

---

# 13. 前端 Vue 页面设计

---

## 13.1 页面一：采集任务页

功能：

- 导入设备列表
- SSH 凭据配置
- 并发数配置
- 启动采集
- 任务日志展示
- 失败设备重试

---

## 13.2 页面二：拓扑页

功能：

- 显示全网逻辑拓扑
- 搜索设备
- 过滤链路状态
- 点击链路查看证据
- 展开聚合成员口

### 推荐图形库

建议使用 `Cytoscape.js`。

---

## 13.3 页面三：设备详情抽屉

展示：

- 基础信息
- 接口列表
- 聚合组
- LLDP 邻居
- 原始命令输出入口

---

## 13.4 页面四：链路详情抽屉

展示：

- A 端设备 / 接口
- B 端设备 / 接口
- 逻辑口 / 成员口
- 发现方式
- 置信度
- 证据链
- 与规划是否一致

---

## 13.5 页面五：规划表比对页

功能：

- 上传 Excel
- 预览导入结果
- 执行比对
- 查看差异
- 导出 Excel / HTML 报告

---

# 14. 前端展示规则

---

## 14.1 节点样式

- 交换机：蓝色
- 路由器：绿色
- 防火墙：红色
- 服务器：灰色
- 推断服务器：灰色虚边或特殊图标

---

## 14.2 链路样式

- 绿色实线：双向 LLDP confirmed
- 黄色实线：单向 LLDP semi-confirmed
- 橙色虚线：FDB inferred
- 蓝色粗线：逻辑聚合链路
- 灰色细线：成员口展开链路
- 红色：冲突或与规划不一致

---

# 15. 并发与性能建议

针对 100 台规模：

## 并发数

- 默认：10~20
- 最大：30

## 超时建议

- SSH 建连：5~8 秒
- 单命令执行：8~15 秒
- 单设备总超时：60~90 秒

## 调度建议

- 设备级并发
- 单设备命令串行
- 原始输出实时入库
- 解析可在采集后立即执行，也可异步批处理

---

# 16. 异常处理设计

---

## 16.1 SSH 异常

- 建连失败
- 认证失败
- 命令超时
- 分页未正确关闭

这些都要在任务日志中可见。

---

## 16.2 解析异常

- 模板不存在
- 模板解析失败
- 字段缺失
- 输出格式偏差

解析失败时：

- 保留原始输出
- 标注命令为 `parse_failed`
- 允许后续重新套模板重跑

---

## 16.3 拓扑冲突

- 同一接口多个对端
- LLDP 与 FDB 推断不一致
- 聚合成员冲突

冲突边不自动覆盖 confirmed，需保留证据。

---

# 17. 开发阶段建议

---

## Phase 1：采集与解析打通

完成：

- SSH 采集框架
- 华为命令集
- gotextfsm 模板加载
- 原始输出保存
- mapper 输出结构体

---

## Phase 2：标准化与拓扑基础版

完成：

- 设备归一化
- 接口归一化
- LLDP 双向链路
- 单向 LLDP 链路
- 聚合口归并
- 基础拓扑图

---

## Phase 3：弱推断与服务器识别

完成：

- FDB 辅助推断
- ARP 辅助识别
- 被动服务器节点
- 证据链展示

---

## Phase 4：规划表比对

完成：

- Excel 固定模板
- 设备/接口映射
- 差异分析
- 报告导出

---

# 18. 最终设计结论

这个方案的核心价值在于：

## 1. 解析方案完全 Go 原生

使用 `gotextfsm`，不引入 Python 依赖。`gotextfsm` 的定位就是兼容 TextFSM 规范的 Go 解析实现，适合网络设备 CLI 半结构化输出解析。 ([Go Packages][1])

## 2. 架构上把“解析”和“拓扑推理”彻底分开

这样以后换 SNMP / NETCONF / API 时，不需要重写拓扑逻辑。

## 3. 从一开始支持“逻辑聚合口 + 成员口展开”

这正匹配你希望的 UI 粒度。

## 4. 通过“LLDP + MAC 地址表 + ARP + 聚合信息”构建多证据拓扑

不是单纯依赖 LLDP。

## 5. 阶段二直接面向固定 Excel 模板做强匹配

更适合企业内工具落地。
