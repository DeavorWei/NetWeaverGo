# LLDP 拓扑功能分阶段实施计划

基于对 `docs/lldp-topology-implementation.md` 文档和项目现有架构的详细分析，我将实施计划拆分为 **5 个阶段**，每个阶段都有明确的交付目标、技术实现细节和验收标准。

---

## 📊 项目架构分析总结

### 现有可复用能力

| 模块         | 位置                           | 复用价值               |
| ------------ | ------------------------------ | ---------------------- |
| SSH 连接层   | `internal/sshutil`             | 直接作为采集传输层     |
| 单设备执行器 | `internal/executor`            | 可抽象为"设备采集会话" |
| 并发调度引擎 | `internal/engine`              | 驱动发现任务并发采集   |
| 事件总线     | `internal/report`              | 实时进度展示、快照推送 |
| 数据库层     | `internal/config/db.go`        | 扩展拓扑领域表结构     |
| Wails 服务层 | `internal/ui`                  | 新增拓扑相关服务       |
| 前端 API 层  | `frontend/src/services/api.ts` | 扩展拓扑 API           |

### 当前缺失能力

- 设备厂商识别字段
- 专用发现命令模板
- CLI 结构化解析层（TextFSM）
- 标准化层（接口名、设备名归一化）
- 拓扑推理层（邻居还原、聚合归并）
- 规划 Excel 导入与比对
- 拓扑可视化页面

---

## 🚀 分阶段实施计划

### Phase 0：基础设施准备（预估 2-3 天）

**目标**：为后续开发建立数据模型和目录结构基础

#### 后端任务

1. **扩展 DeviceAsset 模型**

   ```go
   // 在 internal/config/config.go 中扩展
   type DeviceAsset struct {
       // ... 现有字段 ...
       Vendor      string // huawei / h3c / cisco / server / unknown
       Role        string // core / aggregation / access / firewall / server
       Site        string // 站点/机房
       DisplayName string // 用户维护的显示名称
   }
   ```

2. **新建领域包目录结构**

   ```
   internal/
   ├── discovery/      # 发现任务
   ├── parser/         # CLI 解析
   ├── normalize/      # 标准化层
   ├── topology/       # 拓扑推理
   └── plancompare/    # 规划比对
   ```

3. **新增数据库表模型**
   - `DiscoveryTask` - 发现任务主表
   - `DiscoveryDevice` - 发现设备结果表
   - `RawCommandOutput` - 原始命令输出索引表
   - 在 `internal/config/db.go` 的 `AutoMigrate` 中注册

4. **扩展路径管理器**
   - 在 `internal/config/paths.go` 中添加：
     - `TopologyRawDir` - 原始 CLI 输出目录
     - `TopologyExportDir` - 导出图谱目录
     - `PlanImportDir` - 规划文件导入目录

#### 验收标准

- [x] 数据库迁移成功，新表结构创建完成
- [x] 设备资产支持厂商、角色、站点字段
- [x] 原始输出目录结构可正常创建

---

### Phase 1：采集与原始快照（预估 5-7 天）

**目标**：实现设备发现任务的全流程，从选设备到原始命令入库

#### 后端任务

1. **厂商命令集定义** (`internal/discovery/command_profile.go`)

   ```go
   type CommandSpec struct {
       Command    string
       CommandKey string // 唯一标识：version, lldp_neighbor, interface 等
       TimeoutSec int
   }

   type VendorCommandProfile struct {
       Vendor   string
       Commands []CommandSpec
   }
   ```

   - 首版实现华为命令集：
     - `display version` → `version`
     - `display lldp neighbor verbose` → `lldp_neighbor`
     - `display interface brief` → `interface_brief`
     - `display mac-address` → `mac_address`
     - `display eth-trunk` → `eth_trunk`
     - `display arp all` → `arp_all`

2. **发现任务运行器** (`internal/discovery/runner.go`)
   - 复用 `internal/engine` 的并发调度模式
   - 实现接口：
     ```go
     type Runner interface {
         Start(ctx context.Context, req StartDiscoveryRequest) (string, error)
         RetryFailed(ctx context.Context, taskID string) error
         Cancel(taskID string) error
     }
     ```

3. **原始输出存储** (`internal/discovery/snapshot.go`)
   - 双写策略：
     - 数据库存储索引（设备IP、命令Key、文件路径、状态）
     - 文件系统存储完整文本
   - 路径格式：`<storageRoot>/topology/raw/<taskID>/<deviceIP>/<commandKey>.txt`

4. **Wails 服务** (`internal/ui/discovery_service.go`)

   ```go
   type DiscoveryService struct {}

   func (s *DiscoveryService) StartDiscovery(req StartDiscoveryRequest) (TaskStartResponse, error)
   func (s *DiscoveryService) GetTaskStatus(taskID string) (*DiscoveryTaskView, error)
   func (s *DiscoveryService) ListTasks() ([]DiscoveryTaskView, error)
   func (s *DiscoveryService) CancelTask(taskID string) error
   ```

#### 前端任务

1. **新增发现任务页面** (`frontend/src/views/Discovery.vue`)
   - 设备选择器（复用现有 `DeviceSelector.vue` 组件模式）
   - 厂商过滤选项
   - 并发数和超时配置
   - 任务启动/取消按钮

2. **任务进度展示**
   - 复用 `TaskExecution.vue` 的设备状态卡片模式
   - 展示采集阶段进度
   - 失败设备列表和原因

3. **路由和导航**
   - 在 `router/index.ts` 添加 `/discovery` 路由
   - 在 `App.vue` 侧边栏添加入口

#### 验收标准

- [x] 可选择华为设备批量启动发现任务
- [x] 采集过程可实时查看设备状态
- [x] 每台设备的原始命令文本可回看
- [x] 失败设备可重试

---

### Phase 2：解析与标准化（预估 7-10 天）

**目标**：将原始 CLI 文本解析为结构化数据，并完成字段归一化

#### 后端任务

1. **引入 TextFSM 解析器** (`internal/parser/`)
   - 添加 `gotextfsm` 依赖
   - 实现解析接口：
     ```go
     type CliParser interface {
         Parse(commandKey string, rawText string) ([]map[string]string, error)
     }
     ```
   - 模板加载器 (`template_loader.go`)：使用 `embed` 嵌入模板文件

2. **华为模板文件** (`internal/parser/templates/huawei/`)
   - `version.textfsm` - 解析设备版本信息
   - `lldp_neighbor.textfsm` - 解析 LLDP 邻居
   - `interface.textfsm` - 解析接口信息
   - `mac_address.textfsm` - 解析 MAC 表
   - `eth_trunk.textfsm` - 解析聚合口
   - `arp.textfsm` - 解析 ARP 表

3. **结果映射器** (`internal/parser/mapper.go`)

   ```go
   type ResultMapper interface {
       ToDeviceInfo(rows []map[string]string) (*DeviceIdentity, error)
       ToInterfaces(rows []map[string]string) ([]InterfaceFact, error)
       ToLLDP(rows []map[string]string) ([]LLDPFact, error)
       ToFDB(rows []map[string]string) ([]FDBFact, error)
       ToARP(rows []map[string]string) ([]ARPFact, error)
       ToAggregate(rows []map[string]string) ([]AggregateFact, error)
   }
   ```

4. **标准化层** (`internal/normalize/`)

   ```go
   // 接口名归一化
   func NormalizeInterfaceName(name string) string
   // 示例：GigabitEthernet1/0/1 → GE1/0/1
   //       XGigabitEthernet1/0/1 → XGE1/0/1
   //       Eth-Trunk10 → Trunk10

   // 设备名归一化
   func NormalizeDeviceName(name string) string

   // MAC 地址归一化
   func NormalizeMAC(mac string) string
   ```

5. **新增数据库表**
   - `TopologyInterface` - 接口信息
   - `TopologyLLDPNeighbor` - LLDP 邻居
   - `TopologyFDBEntry` - MAC 表项
   - `TopologyARPEntry` - ARP 表项
   - `TopologyAggregateGroup` - 聚合组
   - `TopologyAggregateMember` - 聚合成员

6. **扩展 Wails 服务**
   - `TopologyService.GetDeviceDetail()` - 获取设备结构化详情
   - 支持从历史原始输出重新解析

#### 前端任务

1. **设备详情抽屉组件** (`frontend/src/components/topology/DeviceDetailDrawer.vue`)
   - 展示设备基础信息（厂商、型号、版本、序列号）
   - 展示接口列表表格
   - 展示聚合关系
   - 展示 LLDP 邻居表
   - 原始命令输出查看入口

2. **解析状态指示**
   - 展示解析成功/失败状态
   - 解析失败时显示原始输出

#### 验收标准

- [x] 可从原始 CLI 输出解析出结构化数据
- [x] 接口名、设备名、MAC 地址完成归一化
- [x] 支持重新解析（不重新采集）
- [x] 单台设备可展示完整结构化详情

---

### Phase 3：邻居还原与拓扑图（预估 10-14 天）

**目标**：实现邻居发现算法，生成拓扑图并可视化展示

#### 后端任务

1. **设备身份归并** (`internal/topology/identity.go`)
   - 优先级：SerialNumber > MgmtIP > ChassisID > NormalizedName
   - 冲突标记：`identity_conflict`

2. **邻居还原算法** (`internal/topology/lldp_matcher.go`)
   - **一级：双向 LLDP 确认链路**
     - 条件：A 的接口 X 发现远端 B:Y，B 的接口 Y 反向发现 A:X
     - 输出：`Status = confirmed, Confidence = 1.0`
   - **二级：单向 LLDP 半确认链路**
     - 条件：仅一侧能通过 LLDP 指向另一侧
     - 输出：`Status = semi_confirmed, Confidence = 0.75`
   - **三级：聚合逻辑链路归并**
     - 条件：成员口物理边双方都属于聚合组
     - 输出：`EdgeType = logical_aggregate`

   - **四级：FDB 弱推断**
     - 条件：接入口无 LLDP，FDB 学到少量 MAC，ARP 定位 IP
     - 输出：`Status = inferred, Confidence = 0.55-0.70`

3. **边去重与冲突处理** (`internal/topology/conflict.go`)
   - 无向边去重键：`min(A:if, B:if) + "|" + max(A:if, B:if)`
   - 冲突场景：
     - 一个接口命中多个远端
     - LLDP 与 FDB 指向不同对象
   - 输出：`Status = conflict`，保留所有证据

4. **证据链模型** (`internal/topology/view.go`)

   ```go
   type TopologyEdge struct {
       ID               string
       ADeviceID        string
       AIf              string
       BDeviceID        string
       BIf              string
       LogicalAIf       string  // 聚合口
       LogicalBIf       string
       EdgeType         string  // physical / logical_aggregate
       Status           string  // confirmed / semi_confirmed / inferred / conflict
       Confidence       float64
       DiscoveryMethods []string
       Evidence         []EdgeEvidence
   }
   ```

5. **拓扑图构建服务** (`internal/topology/builder.go`)

   ```go
   type Builder interface {
       Build(taskID string) (*TopologyBuildResult, error)
   }
   ```

6. **Wails 服务扩展** (`internal/ui/topology_service.go`)
   ```go
   func (s *TopologyService) BuildTopology(taskID string) (*TopologyBuildResult, error)
   func (s *TopologyService) GetTopologyGraph(taskID string) (*TopologyGraphView, error)
   func (s *TopologyService) GetEdgeDetail(taskID string, edgeID string) (*TopologyEdgeDetailView, error)
   ```

#### 前端任务

1. **拓扑图页面** (`frontend/src/views/Topology.vue`)
   - 引入 `cytoscape` + `cytoscape-dagre` 图形库
   - 节点搜索功能
   - 站点/角色过滤器
   - 链路状态过滤（confirmed / semi_confirmed / inferred）
   - 逻辑图/物理展开切换

2. **边详情抽屉** (`frontend/src/components/topology/EdgeDetailDrawer.vue`)
   - 两端设备和接口信息
   - 逻辑口与成员口映射
   - 状态与置信度
   - 发现方式
   - 证据链列表

3. **链路样式规范**
   ```
   confirmed:        绿色实线
   semi_confirmed:   黄色实线
   inferred:         橙色虚线
   logical_aggregate: 蓝色粗线
   conflict:         红色
   ```

#### 验收标准

- [x] 双向 LLDP 正确输出 confirmed 链路
- [x] 单向 LLDP 输出 semi_confirmed 链路
- [x] 聚合口成员正确折叠为逻辑链路
- [x] 无 LLDP 服务器接入口被推断
- [x] 冲突链路被正确标记
- [x] 拓扑图可交互展示

---

### Phase 4：规划表比对（预估 5-7 天）

**目标**：实现 Excel 规划表导入，与实际拓扑比对，输出差异报告

#### 后端任务

1. **Excel 导入器** (`internal/plancompare/importer.go`)
   - 使用 `excelize` 库解析 Excel
   - 支持标准规划表格式：
     - A 设备、A 接口、B 设备、B 接口、链路类型
   - 标准化规划链路字段

2. **规划链路模型**

   ```go
   type PlannedLink struct {
       ID          string
       PlanFileID  string
       ADeviceName string
       AIf         string
       BDeviceName string
       BIf         string
       LinkType    string
   }
   ```

3. **比对引擎** (`internal/plancompare/matcher.go`)
   - 无向边匹配算法
   - 输出差异类型：
     - **缺失链路**：规划有但实际没有
     - **意外链路**：实际有但规划没有
     - **聚合不一致**：规划聚合但实际不是
     - **接口不一致**：接口名称不匹配

4. **差异报告生成** (`internal/plancompare/diff.go`)

   ```go
   type DiffReport struct {
       ID              string
       TaskID          string
       PlanFileID      string
       TotalPlanned    int
       TotalActual     int
       Matched         int
       MissingLinks    []DiffItem
       UnexpectedLinks []DiffItem
       InconsistentItems []DiffItem
   }
   ```

5. **导出功能** (`internal/plancompare/exporter.go`)
   - 支持导出为 Excel / PDF

6. **Wails 服务** (`internal/ui/plan_compare_service.go`)
   ```go
   func (s *PlanCompareService) ImportPlanExcel(filePath string) (*PlanImportResult, error)
   func (s *PlanCompareService) Compare(taskID string, planID string) (*CompareResult, error)
   func (s *PlanCompareService) GetDiffReport(reportID string) (*DiffReportView, error)
   func (s *PlanCompareService) ExportDiffReport(reportID string, format string) (string, error)
   ```

#### 前端任务

1. **规划比对页面** (`frontend/src/views/PlanCompare.vue`)
   - Excel 文件上传组件
   - 规划链路预览表格
   - 比对触发按钮
   - 差异报告展示

2. **差异报告组件**
   - 缺失链路列表（红色高亮）
   - 意外链路列表（橙色高亮）
   - 不一致项列表（黄色高亮）
   - 导出按钮

#### 验收标准

- [x] 可导入 Excel 规划表
- [x] 正确识别缺失链路
- [x] 正确识别意外链路
- [x] 差异报告可导出

---

### Phase 5：测试与优化（预估 3-5 天）

**目标**：确保功能稳定性和可维护性

#### 测试任务

1. **单元测试**
   - `internal/parser/..._test.go` - 解析模板测试
   - `internal/normalize/..._test.go` - 标准化函数测试
   - `internal/topology/..._test.go` - 邻居还原算法测试
   - `internal/plancompare/..._test.go` - 比对算法测试

2. **Golden 测试数据**

   ```
   testdata/huawei/raw/      # 真实脱敏 CLI 输出
   testdata/huawei/parsed/   # 期望解析结果 JSON
   testdata/huawei/topology/ # 期望拓扑边 JSON
   ```

3. **集成测试场景**
   - 双交换机直连场景
   - 交换机双上联聚合场景
   - 接服务器无 LLDP 场景

#### 优化任务

1. 性能优化
   - 大规模设备采集并发控制
   - 拓扑图渲染优化（大量节点）

2. 用户体验优化
   - 加载状态指示
   - 错误提示优化
   - 操作引导

#### 验收标准

- [x] 核心函数单元测试覆盖率 > 80%
- [x] 集成测试场景全部通过
- [x] 无明显性能瓶颈

---

## 📈 实施时间线

| 阶段                      | 预估工期 | 累计  |
| ------------------------- | -------- | ----- |
| Phase 0: 基础设施准备     | 2-3 天   | 3 天  |
| Phase 1: 采集与原始快照   | 5-7 天   | 10 天 |
| Phase 2: 解析与标准化     | 7-10 天  | 20 天 |
| Phase 3: 邻居还原与拓扑图 | 10-14 天 | 34 天 |
| Phase 4: 规划表比对       | 5-7 天   | 41 天 |
| Phase 5: 测试与优化       | 3-5 天   | 46 天 |

**总预估工期：约 6-8 周**

---
