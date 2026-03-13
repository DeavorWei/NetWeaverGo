# 历史执行记录功能规划设计实施文档

## 1. 文档目标

本文档基于 NetWeaverGo 现有项目架构，对“历史执行记录”功能进行重新核对、优化与落地规划，目标是形成一套可直接指导后续开发实施的设计方案。

本文档只做规划设计，不包含实际代码改造。

---

## 2. 现有项目架构核对结论

## 2.1 技术栈与分层

当前项目采用如下结构：

- 后端：Go + Wails v3
- 前端：Vue 3 + TypeScript + Pinia + Vite
- 数据存储：SQLite + GORM
- 实时执行状态：Engine + ProgressTracker + Wails Event

核心目录职责如下：

- `internal/config`：数据库模型、配置管理、数据访问
- `internal/engine`：并发执行引擎、生命周期状态机
- `internal/executor`：单设备 SSH 执行器
- `internal/report`：执行事件采集、快照汇总、CSV 报告、设备日志
- `internal/ui`：Wails 服务层与执行编排层
- `frontend/src/views/TaskExecution.vue`：任务执行页面入口
- `frontend/src/stores/engineStore.ts`：前端执行状态管理

## 2.2 当前执行链路

项目当前并不是单一路径执行，至少存在 4 条执行链路：

1. 普通批量执行：`EngineService.StartEngine`
2. 选择设备/命令组执行：`EngineService.StartEngineWithSelection`
3. 任务组执行：`TaskGroupService.StartTaskGroup`
4. 配置备份执行：`EngineService.StartBackup`

其中任务组执行又分两种模式：

- 模式 A：单引擎执行一组命令到多台设备
- 模式 B：多个子引擎并发执行，由聚合 Tracker 汇总

这意味着历史记录不能只挂在某个单独的 `engine.Run()` 结束点，否则会出现以下问题：

- 模式 B 可能重复保存多条子记录
- 普通执行和任务组执行的记录结构不统一
- 备份执行链路无法复用

## 2.3 当前已有能力

当前项目实际上已经具备历史记录的一部分基础能力：

- `ProgressTracker` 已维护设备最终状态
- `ExecutionSnapshot` 已可生成前端可用快照
- `LogStorage` 已将设备执行日志写入磁盘
- `ExportCSV()` 已生成 CSV 报告

当前缺失的是“执行结果持久化”，而不是“执行结果采集”。

---

## 3. 原始方案评估

用户原始方案总体方向正确，但需要调整以下关键点。

## 3.1 可以保留的部分

- 新增 `ExecutionRecord` 表的思路正确
- 需要支持列表查询、详情查询、统计查询
- 前端在任务执行页增加“历史记录”入口合理
- 保存 CSV 报告路径是必要字段

## 3.2 需要调整的部分

### 1. 保存位置不应直接落在 `internal/engine/engine.go`

原因：

- `engine.go` 是底层执行引擎，应保持“执行”职责单一
- 历史记录属于“编排结果持久化”，更适合放在 UI 编排层统一处理
- 模式 B 是复合执行，不适合在单个子引擎结束时直接落库

### 2. `internal/ui/execution_service.go` 不是当前项目最自然的扩展点

现有项目服务分工已经比较明确：

- 执行编排：`execution_manager.go`
- Wails 暴露服务：`engine_service.go`、`task_group_service.go`

因此更合理的结构是：

- 在 `execution_manager.go` 统一保存历史
- 新增 `execution_history_service.go` 专门暴露查询接口

### 3. `Devices.Logs` 不宜完整入库

当前日志系统已经落到磁盘文件，若把全部设备日志再次以 JSON 形式存入 SQLite，会带来：

- 数据膨胀
- 查询性能下降
- 历史记录列表读取压力上升

更合理的方式是：

- 数据库存摘要
- 文件存完整日志
- 数据库只保留日志尾部、日志数、日志文件路径

### 4. 历史记录入口不应只在 `completed/failed` 状态显示

任务组当前的 `Status` 表示“当前任务状态”，不表示“是否有历史”。

例如：

- `pending` 任务也可能已经执行过
- `running` 期间也可能需要查看历史执行

因此历史入口应与任务当前状态解耦。

---

## 4. 功能目标定义

## 4.1 V1 范围

V1 聚焦“任务组执行历史”，同时在数据模型上兼容后续扩展到普通执行和备份执行。

V1 包含：

- 任务组历史执行记录列表
- 单次执行详情查看
- 设备级结果查看
- 日志尾部查看
- CSV 报告路径查看
- 基础分页、筛选、排序
- 可配置保留策略

V1 不包含：

- 全量日志入库
- 历史记录全文检索
- 多维统计仪表盘
- 历史记录导出为 PDF/HTML
- 差异比对

## 4.2 功能目标

实现后，用户应能够：

1. 在任务执行页查看某个任务组的历史运行记录
2. 看到每次执行的开始时间、结束时间、时长、结果统计
3. 查看单次执行的设备执行明细
4. 查看每台设备的最终状态、执行进度和日志尾部
5. 打开对应的报告路径或日志路径
6. 在不影响现有执行链路的前提下平滑接入

---

## 5. 总体设计

## 5.1 架构原则

本功能设计遵循以下原则：

- 底层引擎不关心历史持久化
- 执行结果在编排层统一收口
- 数据库存摘要，文件存完整日志
- 不破坏现有快照与事件驱动模式
- 兼容模式 A、模式 B、普通执行、备份执行

## 5.2 分层设计

建议新增或改造如下模块：

- 数据模型层：`internal/config/execution_record.go`
- 汇总能力层：扩展 `internal/report/collector.go`
- 执行持久化层：扩展 `internal/ui/execution_manager.go`
- 查询服务层：新增 `internal/ui/execution_history_service.go`
- 前端 API 层：扩展 `frontend/src/services/api.ts`
- 前端展示层：改造 `TaskExecution.vue`，新增历史列表与详情组件

## 5.3 推荐职责边界

### `internal/engine`

职责：

- 只负责执行
- 只发事件
- 不直接处理历史持久化

### `internal/report`

职责：

- 汇总运行结果
- 生成执行摘要
- 返回 CSV 报告路径

### `internal/ui/execution_manager.go`

职责：

- 管理执行生命周期
- 统一持久化历史记录
- 为不同执行来源提供一致的执行元信息

### `internal/ui/execution_history_service.go`

职责：

- 为前端提供历史记录列表、详情、统计查询接口

---

## 6. 数据模型设计

## 6.1 表结构建议

建议新增 `ExecutionRecord` 表。

```go
type ExecutionRecord struct {
    ID            string                  `json:"id" gorm:"primaryKey"`
    RunnerSource  string                  `json:"runnerSource"`  // task_group / engine_service / backup_service
    RunnerID      string                  `json:"runnerId"`      // 运行实例ID，可为空
    TaskGroupID   string                  `json:"taskGroupId"`   // 任务组ID，非任务组执行时为空
    TaskGroupName string                  `json:"taskGroupName"` // 任务组名称快照
    TaskName      string                  `json:"taskName"`      // 执行任务名称快照
    Mode          string                  `json:"mode"`          // group / binding / manual / backup
    Status        string                  `json:"status"`        // completed / partial / failed / cancelled
    TotalDevices  int                     `json:"totalDevices"`
    FinishedCount int                     `json:"finishedCount"`
    SuccessCount  int                     `json:"successCount"`
    ErrorCount    int                     `json:"errorCount"`
    AbortedCount  int                     `json:"abortedCount"`
    WarningCount  int                     `json:"warningCount"`
    StartedAt     string                  `json:"startedAt"`
    FinishedAt    string                  `json:"finishedAt"`
    DurationMs    int64                   `json:"durationMs"`
    ReportPath    string                  `json:"reportPath"`
    Devices       []ExecutionDeviceRecord `json:"devices" gorm:"serializer:json"`
    CreatedAt     string                  `json:"createdAt"`
}

type ExecutionDeviceRecord struct {
    IP          string   `json:"ip"`
    Status      string   `json:"status"`
    TotalCmd    int      `json:"totalCmd"`
    ExecCmd     int      `json:"execCmd"`
    ErrorMsg    string   `json:"errorMsg"`
    LogCount    int      `json:"logCount"`
    LogTail     []string `json:"logTail"`
    LogFilePath string   `json:"logFilePath"`
}
```

## 6.2 字段说明

### 执行来源字段

- `RunnerSource` 用于标记记录来自哪条执行链路
- `RunnerID` 用于扩展未来普通执行的唯一实例标识

这样设计的好处是后续可以统一历史中心，而不仅限于任务组。

### 任务字段

- `TaskGroupID` 与 `TaskGroupName` 是任务组视角查询的关键
- `TaskName` 用于前端显示，避免依赖实时查任务名

### 状态字段

建议历史记录状态与 `TaskGroup.Status` 区分：

- `TaskGroup.Status`：当前配置对象状态
- `ExecutionRecord.Status`：某次执行结果

推荐取值：

- `completed`
- `partial`
- `failed`
- `cancelled`

### 日志字段

日志建议采用“摘要 + 文件引用”模型：

- `LogTail`：保留最后 20 到 50 条日志
- `LogCount`：原始日志数量
- `LogFilePath`：完整日志路径

不建议将完整 `Logs []string` 长期入库。

## 6.3 索引建议

在数据库迁移时新增索引：

- `idx_execution_records_task_group_id`
- `idx_execution_records_runner_source`
- `idx_execution_records_status`
- `idx_execution_records_created_at`
- 如需按时间倒序分页：`idx_execution_records_started_at`

---

## 7. 执行结果采集与持久化设计

## 7.1 现有基础能力复用

当前 `ProgressTracker` 已具备以下能力：

- 统计设备状态
- 获取执行开始时间
- 获取设备日志尾部
- 生成前端快照
- 生成 CSV 报告

因此 V1 不需要重新设计设备结果采集器，只需扩展 `ProgressTracker` 输出能力。

## 7.2 需要扩展的能力

建议在 `internal/report/collector.go` 增加：

### 1. 返回报告路径

当前 `ExportCSV(outputDir string)` 只生成文件，不返回路径。

建议改为：

```go
func (p *ProgressTracker) ExportCSV(outputDir string) (string, error)
```

返回值供历史记录保存使用。

### 2. 输出结构化执行摘要

建议新增方法：

```go
func (p *ProgressTracker) BuildExecutionSummary() *ExecutionSummaryData
func (p *ProgressTracker) BuildExecutionDevices(maxLogTail int) []ExecutionDeviceData
```

其中应包含：

- 设备总数
- 已完成数
- 成功数
- 失败数
- 中止数
- 告警数
- 每设备最终状态
- 每设备日志尾部

### 3. 输出开始与结束时间

当前已有开始时间，应补充结束时间或由执行管理层记录：

- `startTime`：由 tracker 保留
- `finishTime`：由 execution manager 在执行收尾时记录

## 7.3 历史保存的统一收口点

建议在 `internal/ui/execution_manager.go` 保存历史，而不是 `engine.go`。

原因如下：

### 原因 1：统一覆盖所有执行链路

`execution_manager` 当前已经负责：

- 执行会话创建
- tracker 注入
- Wails 事件转发
- 执行结束收尾

它天然就是最合适的“执行完成统一收口层”。

### 原因 2：避免模式 B 重复保存

模式 B 会启动多个子引擎，如果在子引擎层面保存，会导致：

- 一次任务组执行产生多条历史记录
- 历史结果与用户认知不一致

### 原因 3：降低 engine 层耦合

保持 `engine` 只处理执行，不让它依赖数据库模型和 UI 语义。

## 7.4 建议引入执行元数据

建议在 `execution_manager.go` 中定义统一执行元数据：

```go
type ExecutionMeta struct {
    RunnerSource  string
    RunnerID      string
    TaskGroupID   string
    TaskGroupName string
    TaskName      string
    Mode          string
}
```

各条执行链路在启动执行时传入元数据：

- 普通执行：`manual`
- 任务组模式 A：`group`
- 任务组模式 B：`binding`
- 备份：`backup`

## 7.5 保存时机

历史记录保存应发生在：

- 执行完成
- tracker 汇总完成
- CSV 路径已确定
- 所有前端事件转发结束之后

推荐顺序：

1. 执行逻辑结束
2. Tracker 收尾
3. 生成 CSV
4. 构建历史记录对象
5. 落库
6. 执行保留策略清理
7. 触发最终完成事件

---

## 8. 状态判定设计

## 8.1 历史记录状态与任务状态分离

当前 `TaskGroup.Status` 取值是：

- `pending`
- `running`
- `completed`
- `failed`

这套状态适合表示当前任务配置的执行态，不适合表示“某一次历史执行结果”。

历史记录建议使用独立规则。

## 8.2 推荐判定规则

### `completed`

条件：

- 所有设备最终为成功或告警放行完成
- 无失败和中止设备

### `partial`

条件：

- 存在成功设备
- 同时存在失败或中止设备

### `failed`

条件：

- 所有设备均失败或中止
- 或任务压根没有可成功执行的设备

### `cancelled`

条件：

- 用户主动停止
- Context 被取消，且执行不是正常成功收尾

## 8.3 与 V1 前端展示映射

建议前端标签：

- `completed` -> 成功
- `partial` -> 部分成功
- `failed` -> 失败
- `cancelled` -> 已取消

---

## 9. 日志与报告存储策略

## 9.1 当前情况

当前项目已有两类输出：

- `data/logs`：设备日志文件
- `output`：CSV 报告及其他导出文件

## 9.2 V1 建议策略

### 数据库存摘要

数据库仅保存：

- 执行统计
- 每设备摘要
- 日志尾部
- 文件路径

### 文件存完整日志

完整日志继续使用文件系统保存。

建议新增归档目录：

- `data/history/<recordID>/`

可选归档内容：

- 每设备日志快照
- 执行级 JSON 摘要

如果 V1 不做归档复制，也至少要保存原始日志路径。

## 9.3 保留策略建议

建议增加运行时可配置保留项：

- `maxExecutionHistoryRecords`：默认 100
- `maxHistoryLogTailLines`：默认 30

V1 也可先简化成固定策略：

- 每任务组保留最近 100 条
- 或全局保留最近 1000 条

清理策略在每次新增记录后执行。

---

## 10. 后端详细实施方案

## 10.1 新增模型文件

新增：

- `internal/config/execution_record.go`

职责：

- 定义 `ExecutionRecord`
- 定义 `ExecutionDeviceRecord`
- 提供 CRUD 方法

建议方法：

- `CreateExecutionRecord(record ExecutionRecord) (*ExecutionRecord, error)`
- `GetExecutionRecord(id string) (*ExecutionRecord, error)`
- `ListExecutionRecords(taskGroupID string, opts QueryOptions) (*QueryResult, error)`
- `DeleteOldExecutionRecords(limit int) error`

## 10.2 数据库注册

修改：

- `internal/config/db.go`

新增：

- `AutoMigrate(&ExecutionRecord{})`
- 历史记录索引创建 SQL

## 10.3 扩展 ProgressTracker

修改：

- `internal/report/collector.go`

需要补充的能力：

- 返回 CSV 报告路径
- 生成结构化执行结果摘要
- 导出设备级结果列表

## 10.4 扩展 execution_manager

修改：

- `internal/ui/execution_manager.go`

新增职责：

- 保存执行元数据
- 在 `session.Finish()` 前后统一触发历史持久化

建议新增逻辑：

- `beginExecution(..., meta ExecutionMeta)`
- `persistExecutionRecord(meta, tracker, finishTime, reportPath, runErr)`

## 10.5 调整各执行入口

修改：

- `internal/ui/task_group_service.go`
- `internal/ui/engine_service.go`

目的：

- 在启动执行时传入 `ExecutionMeta`

### 任务组模式 A

记录：

- `RunnerSource = "task_group"`
- `TaskGroupID = taskGroup.ID`
- `TaskGroupName = taskGroup.Name`
- `TaskName = taskGroup.Name`
- `Mode = "group"`

### 任务组模式 B

同样只在聚合执行层保存一次：

- `Mode = "binding"`

### 普通执行

- `RunnerSource = "engine_service"`
- `Mode = "manual"`

### 备份执行

- `RunnerSource = "backup_service"`
- `Mode = "backup"`

## 10.6 新增查询服务

新增：

- `internal/ui/execution_history_service.go`

建议暴露接口：

- `ListExecutionRecords(taskGroupID string, opts QueryOptions) *QueryResult`
- `GetExecutionRecord(id string) (*config.ExecutionRecord, error)`
- 可选：`GetExecutionRecordStats(taskGroupID string) (*ExecutionHistoryStats, error)`

说明：

- V1 不一定需要单独统计接口，也可以由前端根据列表聚合
- 若后续要做卡片统计，单独接口更合理

## 10.7 Wails 服务注册

修改：

- `cmd/netweaver/main.go`

新增 service：

- `executionHistoryService := ui.NewExecutionHistoryService()`
- 注册到 `application.NewService(...)`

---

## 11. 前端详细实施方案

## 11.1 API 层扩展

修改：

- `frontend/src/services/api.ts`

新增：

```ts
export const ExecutionHistoryAPI = {
  listExecutionRecords: ExecutionHistoryServiceBinding.ListExecutionRecords,
  getExecutionRecord: ExecutionHistoryServiceBinding.GetExecutionRecord,
} as const
```

## 11.2 页面入口改造

修改：

- `frontend/src/views/TaskExecution.vue`

建议改造点：

### 1. 卡片操作区增加“历史记录”按钮

位置：

- 与“执行”“删除”并列

注意：

- 不要限制为 `completed/failed` 才显示
- 所有任务卡片都可打开历史

### 2. 历史记录抽屉或弹窗

建议优先使用抽屉：

- 任务上下文更连贯
- 与当前页面布局兼容更好
- 适合列表 + 详情切换

## 11.3 新增组件建议

### `frontend/src/components/task/ExecutionHistoryDrawer.vue`

职责：

- 按任务组展示历史记录列表
- 支持分页、状态筛选、时间倒序

列表建议字段：

- 开始时间
- 结束时间
- 时长
- 状态
- 设备总数
- 成功/失败/中止数

### `frontend/src/components/task/ExecutionRecordDetail.vue`

职责：

- 展示单次执行详情
- 展示设备级结果
- 展示日志尾部

建议详情字段：

- 任务名称
- 执行模式
- 状态
- 开始时间
- 结束时间
- 时长
- 报告路径
- 设备结果表格

设备表格字段：

- IP
- 状态
- 执行命令数
- 总命令数
- 错误信息
- 日志尾部展开

## 11.4 交互建议

### 历史列表交互

- 默认按开始时间倒序
- 支持状态筛选
- 支持分页

### 详情交互

- 点击列表项打开详情面板
- 支持复制报告路径
- 支持复制日志路径

### 空状态

- “暂无历史执行记录”

### 错误状态

- “加载历史记录失败，请稍后重试”

---

## 12. 推荐实施顺序

建议按以下顺序实施。

### 第一阶段：后端数据能力打底

1. 新增 `ExecutionRecord` 模型
2. 注册数据库迁移与索引
3. 扩展 `ProgressTracker` 摘要输出能力
4. 改造 CSV 导出返回路径

### 第二阶段：执行链路接入

5. 在 `execution_manager.go` 增加统一持久化逻辑
6. 在 `task_group_service.go` 传入任务组执行元数据
7. 在 `engine_service.go` 传入普通执行和备份元数据
8. 完成模式 A、模式 B、备份链路的统一保存

### 第三阶段：前端查询与展示

9. 新增 `ExecutionHistoryService`
10. 生成 Wails bindings
11. 扩展 `api.ts`
12. 开发历史记录抽屉组件
13. 开发执行详情组件
14. 在 `TaskExecution.vue` 接入入口

### 第四阶段：收尾与验证

15. 增加保留策略
16. 验证历史记录分页和详情加载
17. 验证模式 A、模式 B、普通执行、备份执行都能正确落库
18. 验证报告路径和日志路径有效

---

## 13. 风险点与规避方案

## 13.1 模式 B 重复落库风险

风险：

- 模式 B 有多个子引擎，若在引擎层保存，会一条任务产生多条历史

规避：

- 只允许聚合编排层保存一次

## 13.2 日志体积过大风险

风险：

- 将完整日志入库会导致 SQLite 体积快速膨胀

规避：

- 只存日志尾部
- 完整日志走文件路径引用

## 13.3 当前视图模型重复定义风险

项目里存在两套执行视图结构：

- `internal/report` 中真实在用的快照模型
- `internal/ui/view_models.go` 中一套未完全接入的视图模型

风险：

- 若新功能继续基于未实际接入的模型扩展，会造成模型分叉

规避：

- 历史执行记录功能统一基于 `internal/report` 的真实数据来源构建

## 13.4 用户停止执行的状态归类风险

风险：

- 当前停止执行只是 `cancelFunc()`，历史记录若简单按错误统计可能误判为 `failed`

规避：

- 在 execution manager 中识别用户取消路径
- 单独映射为 `cancelled`

## 13.5 任务状态与历史状态混淆风险

风险：

- 将 `TaskGroup.Status` 当成最后一次历史结果使用

规避：

- 明确两者语义不同
- 前端历史记录完全基于 `ExecutionRecord`

---

## 14. 验收标准

功能完成后，至少应满足以下验收标准。

## 14.1 数据层

- 可成功创建 `execution_records` 表
- 执行结束后可自动写入历史记录
- 可按任务组查询历史记录
- 可按记录 ID 查询执行详情

## 14.2 执行链路

- 模式 A 执行后生成 1 条历史记录
- 模式 B 执行后生成 1 条历史记录，不重复
- 普通批量执行可生成历史记录
- 备份执行可生成历史记录

## 14.3 内容正确性

- 开始时间、结束时间、时长正确
- 成功数、失败数、中止数与真实执行结果一致
- CSV 报告路径有效
- 每设备摘要信息正确
- 日志尾部可正常查看

## 14.4 前端体验

- 任务列表可打开历史记录
- 历史记录支持分页
- 详情展示完整
- 空状态与错误状态明确

---

## 15. 最终建议结论

基于现有架构，历史执行记录功能的正确实现方式不是“在 `engine.go` 中直接保存执行结果”，而是：

1. 由 `engine` 继续负责执行
2. 由 `report` 负责汇总执行结果
3. 由 `execution_manager` 在统一收尾点保存历史记录
4. 由新增的 `execution_history_service` 提供查询接口
5. 由 `TaskExecution.vue` 提供任务视角的历史入口

最终建议如下：

- 保留“新增 `ExecutionRecord` 表”的总体方向
- 将服务层从 `execution_service.go` 调整为 `execution_history_service.go`
- 将保存时机从 `engine.go` 调整到 `execution_manager.go`
- 将设备日志存储从“完整入库”调整为“摘要入库 + 文件引用”
- 将历史记录能力设计为可兼容普通执行、任务组执行和备份执行的统一模型

这套方案与项目当前实际架构匹配度更高，后续扩展成本也更低。
