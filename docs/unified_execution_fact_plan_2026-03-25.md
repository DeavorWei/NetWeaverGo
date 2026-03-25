# 统一执行框架无兼容硬切换实施方案

制定时间：2026-03-25

基于以下审计结果：

- [unified_execution_final_audit_2026-03-25.md](./unified_execution_final_audit_2026-03-25.md)

---

## 1. 实施前提

本方案采用硬切换原则，不保留任何历史兼容设计。

已确认约束：

1. 项目处于新建期
2. 可以删除所有老旧代码
3. 可以删除所有兼容逻辑
4. 可以直接调整或重建旧数据库结构
5. 不需要对旧 `task_id`、旧发现任务、旧历史记录做兼容迁移

因此，本方案不再采用：

- 兼容层保留
- 双状态并存
- 双主键过渡
- 旧表字段继续复用
- 旧服务降级保留

本方案直接以“统一执行框架成为唯一主框架”为目标进行重构。

---

## 2. 目标状态

实施完成后，项目应满足以下唯一性约束：

### 2.1 唯一执行内核

只保留：

- `taskexec`

彻底删除：

- `EngineService` 执行链
- `executionManager`
- `discovery.Runner`
- 旧任务组执行逻辑

### 2.2 唯一运行主键

只保留：

- `runID`

彻底删除前端和服务层主语义中的：

- `taskID`
- `DiscoveryTaskID`
- `RunnerID`

说明：

- 数据库层如果仍有旧字段，也必须同步重构，不允许继续以旧字段作为主语义承载执行结果。

### 2.3 唯一前端执行状态中心

只保留：

- `frontend/src/stores/taskexecStore.ts`

彻底删除：

- `engineStore`
- 旧 `ExecutionSnapshot` 兼容态
- 页面内并行执行状态源

### 2.4 唯一结果消费模型

以下模块全部改为只围绕 `runID` 工作：

- 任务执行页
- 拓扑图谱页
- 规划比对页
- 历史记录页

---

## 3. 直接删除清单

以下内容不做保留，不做兼容，不做过渡。

## 3.1 后端删除清单

直接删除：

- `internal/ui/engine_service.go`
- `internal/ui/execution_manager.go`
- `internal/discovery/runner.go`
- `internal/ui/discovery_service.go`
- `internal/ui/topology_service.go`
- `internal/ui/task_group_service.go` 中所有旧执行逻辑

清理主程序注册：

- `main.go` 中不再注册旧发现/拓扑服务

### 3.2 前端删除清单

直接删除：

- `frontend/src/stores/engineStore.ts`
- `frontend/src/composables/useTaskExecution.ts` 中旧事件兼容逻辑
- 所有 `selectedTaskID` 状态
- 所有旧 `execution:snapshot` / `engine:finished` 监听
- 所有围绕旧发现任务的选择器和列表

### 3.3 数据库删除清单

允许直接删除或重建：

- 旧发现任务主表
- 旧发现设备表
- 旧原始输出表
- 旧拓扑边/节点/解析相关表
- 旧执行历史表

说明：

如果这些表中有部分字段仍可复用，也应迁移进新的 `taskexec` 领域表，而不是继续沿用旧表语义。

---

## 4. 新架构落地方案

## 阶段 A：统一运行时成为唯一后端执行服务

### A.1 目标

让 `TaskExecutionService` 成为系统唯一执行入口。

### A.2 实施内容

修改文件：

- `cmd/netweaver/main.go`
- `internal/ui/task_group_service_v2.go`
- `internal/ui/taskexec_ui_service.go`

实施动作：

1. 保留应用级共享 `TaskExecutionService`
2. `TaskGroupServiceV2` 继续作为任务组到统一运行时的编排入口
3. 删除旧 `TaskGroupService` 中残留的旧拓扑执行逻辑
4. 删除旧 discovery/topology 服务注册
5. 所有拓扑相关执行都必须通过 `taskexec` 发起

### A.3 验收标准

- 后端不存在第二套执行引擎
- 主程序只注册统一执行服务相关服务
- 所有任务执行路径都进入 `taskexec`

---

## 阶段 B：重建统一运行时数据模型

### B.1 目标

不再复用旧发现/拓扑/历史表，直接建立统一运行时自己的持久化模型。

### B.2 实施内容

修改范围：

- `internal/taskexec/models.go`
- `internal/taskexec/persistence.go`
- 相关迁移逻辑

新增或重构数据模型建议：

1. `task_runs`
2. `task_run_stages`
3. `task_run_units`
4. `task_events`
5. `task_artifacts`
6. `task_topology_nodes`
7. `task_topology_edges`
8. `task_topology_device_facts`
9. `task_run_history_views` 或等价摘要模型

### B.3 实施动作

1. 拓扑采集的解析事实直接落到 `taskexec` 新表
2. 拓扑构图结果直接落到 `taskexec` 新拓扑表
3. 历史摘要直接由 `taskexec` 生成，不再依赖旧 `ExecutionRecord`
4. 删除旧 discovery/topology/history 表迁移逻辑

### B.4 验收标准

- 新 run 的全部事实、拓扑、历史都只写入新表
- 数据库中不存在“新执行链写旧发现表”的情况

---

## 阶段 C：打通 `runID` 唯一主键

### C.1 目标

前后端所有执行消费链全部只认 `runID`。

### C.2 实施内容

修改文件：

- `internal/ui/task_group_service.go`
- `internal/ui/task_group_service_v2.go`
- Wails 绑定
- `frontend/src/services/api.ts`
- `frontend/src/views/TaskExecution.vue`

实施动作：

1. `StartTaskGroup()` 直接返回 `runID`
2. 前端启动任务后立即保存当前 `runID`
3. 当前运行快照、取消、历史、跳转都基于该 `runID`
4. 所有旧 `taskID` 相关接口删除

### C.3 验收标准

- 页面内没有 `selectedTaskID`
- 启动任务必返回 `runID`
- 停止任务必携带 `runID`

---

## 阶段 D：重写事件桥和前端 API

### D.1 目标

建立统一运行时唯一事件协议和唯一前端 API。

### D.2 实施内容

修改文件：

- `internal/ui/taskexec_event_bridge.go`
- `internal/taskexec/eventbus.go`
- `frontend/src/services/api.ts`
- `frontend/src/stores/taskexecStore.ts`

实施动作：

1. 删除 JSON 字符串化事件发送方式
2. 统一直接发送结构化对象
3. 删除桥中的 `runIDs` 追踪 map，避免伪订阅和并发风险
4. EventBus 支持桥自身 handler 注销，而不是全量清空
5. `TaskExecutionAPI` 只接真实 Wails binding

标准事件只保留：

- `task:started`
- `task:snapshot`
- `task:event`
- `task:finished`

可选：

- `task:stage_updated`
- `task:unit_updated`

### D.3 验收标准

- 前端事件能正确解析
- 没有并发 map panic 风险
- 不再存在 stub API

---

## 阶段 E：前端状态管理硬切换

### E.1 目标

只保留 `taskexecStore` 作为执行状态中心。

### E.2 实施内容

修改文件：

- `frontend/src/stores/taskexecStore.ts`
- `frontend/src/App.vue`
- `frontend/src/views/TaskExecution.vue`

删除文件：

- `frontend/src/stores/engineStore.ts`

实施动作：

1. `App.vue` 只初始化 `taskexecStore`
2. `taskexecStore` 负责：
   - 当前 `runID`
   - 当前快照
   - run 历史
   - 事件日志
   - 取消任务
3. 页面只保留展示态，不再维护执行主状态
4. 删除旧执行快照兼容逻辑

### E.3 验收标准

- 项目中只有一套执行状态 store
- 任务执行页不再引用 `engineStore`

---

## 阶段 F：重写任务执行页

### F.1 目标

让任务执行页彻底围绕统一运行时工作，不保留任何旧页面逻辑。

### F.2 实施内容

修改文件：

- `frontend/src/views/TaskExecution.vue`

实施动作：

1. 启动任务后立即拿到 `runID`
2. 页面以 `runID` 绑定当前运行
3. 普通任务和拓扑任务统一展示 stage/unit 进度
4. 停止按钮只调用 `cancelTask(runID)`
5. 删除拓扑任务特殊轮询完成逻辑
6. 删除旧快照轮询兼容逻辑
7. 删除旧 suspend/engine 相关 UI

### F.3 验收标准

- 任务执行页是纯统一运行时页面
- 不再存在旧引擎痕迹

---

## 阶段 G：重写拓扑图谱页

### G.1 目标

拓扑图谱页只消费统一运行时拓扑结果。

### G.2 实施内容

修改文件：

- `frontend/src/views/Topology.vue`
- 后端统一拓扑查询接口

实施动作：

1. 删除 `selectedTaskID`
2. 页面只展示统一运行时的 topology runs
3. 新增统一接口：
   - `GetTopologyGraph(runID)`
   - `GetEdgeDetail(runID, edgeID)`
   - `GetDeviceTopologyDetail(runID, deviceID)`
4. 拓扑页不再调用 `DiscoveryAPI`

### G.3 验收标准

- 页面只围绕 `runID`
- 不再出现“发现任务”概念

---

## 阶段 H：重写规划比对页

### H.1 目标

规划比对只接受统一运行时拓扑 run。

### H.2 实施内容

修改文件：

- `frontend/src/views/PlanCompare.vue`
- `internal/ui/plan_compare_service.go`
- `internal/plancompare/service.go`

实施动作：

1. 删除 `selectedTaskID`
2. 比对入参统一为 `runID`
3. 页面只展示 topology run 列表
4. 后端服务命名和注释全部改成 run 语义

### H.3 验收标准

- 规划比对页中不存在旧发现任务语义
- 比对只使用 `runID`

---

## 阶段 I：重写执行历史

### I.1 目标

历史记录只基于统一运行时。

### I.2 实施内容

修改文件：

- `internal/ui/execution_history_service.go`
- `frontend/src/components/task/ExecutionHistoryDrawer.vue`
- `frontend/src/components/task/ExecutionRecordDetail.vue`

实施动作：

1. 删除旧 `ExecutionRecord` 主路径
2. 历史页默认只展示 `taskexec` runs
3. 历史详情 DTO 直接来自统一运行时
4. 删除 `RunnerSource` 旧分类模型

### I.3 验收标准

- 历史页面不再读取旧执行记录表
- 历史详情直接展示 run/stage/unit 数据

---

## 阶段 J：删除备份旧入口，独立决定是否重做

### J.1 目标

避免当前“入口存在但不可用”的状态。

### J.2 实施内容

短期动作：

1. 立即移除当前备份入口 UI
2. 删除与旧备份链路相关的前端兼容逻辑

中期决策：

- 如果需要保留备份功能，未来应基于 `taskexec` 重新设计为独立 run kind

### J.3 验收标准

- 用户界面中不存在失效入口

---

## 5. 执行顺序

按以下顺序硬切换：

1. 阶段 A：统一运行时成为唯一后端执行服务
2. 阶段 B：重建统一运行时数据模型
3. 阶段 C：打通 `runID` 唯一主键
4. 阶段 D：重写事件桥和前端 API
5. 阶段 E：前端状态管理硬切换
6. 阶段 F：重写任务执行页
7. 阶段 G：重写拓扑图谱页
8. 阶段 H：重写规划比对页
9. 阶段 I：重写执行历史
10. 阶段 J：删除备份旧入口

---

## 6. 交付批次建议

建议按 4 个批次提交：

### 批次 1

- A + B + C + D

目标：

- 完成后端唯一执行服务、唯一主键、唯一事件协议

### 批次 2

- E + F

目标：

- 完成前端执行态硬切换和任务执行页重写

### 批次 3

- G + H

目标：

- 完成拓扑图谱和规划比对硬切换

### 批次 4

- I + J

目标：

- 完成历史统一和失效功能清理

---

## 7. 测试方案

## 7.1 后端

- `go test ./...`
- 拓扑 run 端到端执行测试
- run 历史生成测试
- 事件桥事件发送测试

## 7.2 前端

- `npm run build`
- 启动任务并实时展示快照
- 取消任务
- 刷新页面恢复运行状态
- 拓扑 run 查看结果
- 规划比对执行
- 历史查看

## 7.3 数据层

- 新 schema 自动迁移验证
- 新 run 数据只写新表验证
- 删除旧表后系统仍完整可运行验证

---

## 8. 最终交付标准

完成后必须满足：

1. 项目中只存在 `taskexec` 一套执行框架
2. 项目中只存在 `runID` 一套执行主键
3. 前端只存在 `taskexecStore` 一套执行状态管理
4. 拓扑、规划比对、历史都只围绕 `runID`
5. 数据库中不再保留旧发现/旧历史主语义表
6. 用户界面中不存在失效功能入口

---

## 9. 结论

既然当前项目允许直接删除历史代码和旧数据库结构，就不应该再做兼容迁移。

本次实施应采用：

`唯一运行时 + 唯一主键 + 唯一状态源 + 唯一数据模型`

即：

`直接切干净，不保留旧执行链、不保留旧任务模型、不保留旧状态管理、不保留旧数据库语义。`
