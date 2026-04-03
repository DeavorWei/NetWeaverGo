# 任务执行历史删除功能 - 问题清单汇总

**文档生成时间**: 2026-04-03  
**数据来源**:

- `docs/execution_history_delete_feature_analysis.md` (分析报告)
- `docs/execution_history_delete_feature_review.md` (评审报告)

**验证状态**: 所有问题已通过代码核实确认

---

## 一、严重问题 (🔴 高优先级)

### 1.1 数据表遗漏

**来源**: 分析报告 2.1节

**问题描述**:  
方案中列出的关联数据表**不完整**，会导致删除后数据库中残留孤立数据。

**方案中列出的表** (5个):
| 表名 | 模型 |
|------|------|
| `task_runs` | `TaskRun` |
| `task_run_stages` | `TaskRunStage` |
| `task_run_units` | `TaskRunUnit` |
| `task_run_events` | `TaskRunEvent` |
| `task_artifacts` | `TaskArtifact` |

**实际需要删除的表** (14个，根据 [`persistence.go:280-298`](internal/taskexec/persistence.go:280)):
| 表名 | 模型 | 状态 |
|------|------|------|
| `task_runs` | `TaskRun` | ✅ 已包含 |
| `task_run_stages` | `TaskRunStage` | ✅ 已包含 |
| `task_run_units` | `TaskRunUnit` | ✅ 已包含 |
| `task_run_events` | `TaskRunEvent` | ✅ 已包含 |
| `task_artifacts` | `TaskArtifact` | ✅ 已包含 |
| `task_run_devices` | `TaskRunDevice` | ❌ **遗漏** |
| `task_raw_outputs` | `TaskRawOutput` | ❌ **遗漏** |
| `task_parsed_interfaces` | `TaskParsedInterface` | ❌ **遗漏** |
| `task_parsed_lldp_neighbors` | `TaskParsedLLDPNeighbor` | ❌ **遗漏** |
| `task_parsed_fdb_entries` | `TaskParsedFDBEntry` | ❌ **遗漏** |
| `task_parsed_arp_entries` | `TaskParsedARPEntry` | ❌ **遗漏** |
| `task_parsed_aggregate_groups` | `TaskParsedAggregateGroup` | ❌ **遗漏** |
| `task_parsed_aggregate_members` | `TaskParsedAggregateMember` | ❌ **遗漏** |
| `task_topology_edges` | `TaskTopologyEdge` | ❌ **遗漏** |

**影响**: 删除拓扑采集任务后，会残留大量解析数据和拓扑边数据，造成数据库膨胀和数据不一致。

**涉及文件**: `internal/taskexec/persistence.go`

---

### 1.2 文件路径硬编码

**来源**: 分析报告 2.2节、评审报告 问题2

**问题描述**:  
方案中硬编码了文件路径，未使用项目统一的路径管理器。

**方案中的代码** (第238-239行):

```go
rawDir := filepath.Join("Dist", "netWeaverGoData", "topology", "raw", "run_"+runID)
normalizedDir := filepath.Join("Dist", "netWeaverGoData", "topology", "normalized", "run_"+runID)
```

**问题分析**:

1. 项目有统一的路径管理器 [`PathManager`](internal/config/paths.go:24)，应使用 `config.GetPathManager()` 获取路径
2. 用户可能配置了自定义的 `StorageRoot`，硬编码路径会导致文件无法删除
3. `execution/live-logs` 目录的日志文件未被清理

**正确做法**:

```go
pm := config.GetPathManager()
rawDir := filepath.Join(pm.TopologyRawDir, "run_"+runID)
normalizedDir := filepath.Join(pm.StorageRoot, "topology", "normalized", "run_"+runID)
```

**涉及文件**: `internal/taskexec/service.go` 或 `internal/ui/execution_history_service.go`

---

### 1.3 架构职责混乱

**来源**: 评审报告 问题1

**问题描述**:  
方案将删除逻辑放在 `TaskExecutionService`，但 `TaskExecutionService` 的职责是**执行任务**，而非**管理历史记录**。

**当前方案设计**:

```
ExecutionHistoryService → TaskExecutionService → Repository
```

**问题分析**:

- `TaskExecutionService` 应专注于任务执行的编排和协调
- 历史记录管理属于数据管理范畴，不应耦合在执行服务中
- 这会导致 `TaskExecutionService` 变得臃肿，违反单一职责原则

**建议修改**:
删除功能应该直接由 `ExecutionHistoryService` 通过 `Repository` 接口实现：

```
ExecutionHistoryService → Repository
```

**涉及文件**:

- `internal/taskexec/service.go` - 不应在此添加删除方法
- `internal/ui/execution_history_service.go` - 应在此实现删除逻辑

---

## 二、中等问题 (🟡 中优先级)

### 2.1 DeleteAllRuns 文件清理缺失

**来源**: 分析报告 2.3节

**问题描述**:  
方案中 `DeleteAllRuns` 方法只删除数据库记录，未删除文件。

**方案中的代码** (第220-232行):

```go
func (s *TaskExecutionService) DeleteAllRuns() error {
    // ... 检查运行中任务
    return s.repo.DeleteAllRuns(context.Background())  // 只删除数据库
}
```

**影响**: 删除全部记录后，文件系统会残留大量孤立文件目录。

**涉及文件**: `internal/taskexec/service.go` 或 `internal/ui/execution_history_service.go`

---

### 2.2 遗漏执行日志文件删除

**来源**: 评审报告 问题3

**问题描述**:  
方案只删除了拓扑采集文件，遗漏了执行日志文件。

**拓扑文件** (已处理):

```
Dist/netWeaverGoData/
├── topology/
│   ├── raw/run_{runId}/
│   └── normalized/run_{runId}/
```

**执行日志文件** (遗漏):

```
Dist/netWeaverGoData/
└── execution/
    └── live-logs/
        └── {timestamp}_task_{ip}_*.log  # 这些文件未被删除
```

**建议修改**:

1. 在删除运行时，查询 `task_run_units` 表获取每个单元的日志路径
2. 遍历删除 `summary_log_path`、`detail_log_path`、`raw_log_path`、`journal_log_path` 字段指向的文件

**涉及文件**: `internal/ui/execution_history_service.go`

---

### 2.3 异步删除缺少 Panic 保护

**来源**: 评审报告 问题4

**问题描述**:  
异步删除文件时缺少 panic 恢复机制。

**方案代码** (第215行):

```go
go s.deleteRunFiles(runID, run.RunKind, artifacts)
```

**问题分析**:

- 如果 `deleteRunFiles` 中出现 panic，会导致整个程序崩溃
- 文件系统操作（如权限问题、磁盘错误）可能引发不可预期的错误

**建议修改**:

```go
go func() {
    defer func() {
        if r := recover(); r != nil {
            logger.Error("TaskExec", runID, "删除文件时发生panic: %v", r)
        }
    }()
    s.deleteRunFiles(runID, run.RunKind, artifacts)
}()
```

**涉及文件**: `internal/ui/execution_history_service.go`

---

### 2.4 前端 Store 方法缺失

**来源**: 分析报告 2.4节

**问题描述**:  
方案假设 `taskexecStore` 有以下方法，但当前代码中**不存在**。

**方案假设存在的方法**:

```typescript
removeRunFromHistory(runId: string)
clearAllHistory()
```

**当前 [`taskexecStore.ts`](frontend/src/stores/taskexecStore.ts:283) 只有**:

- `setRunHistory(runs: RunSummary[])` - 设置历史列表
- `runHistory` - 响应式引用

**需要新增这些方法**。

**涉及文件**: `frontend/src/stores/taskexecStore.ts`

---

### 2.5 API 绑定缺失

**来源**: 分析报告 2.5节、评审报告 问题5

**问题描述**:  
当前 [`api.ts:280-285`](frontend/src/services/api.ts:280) 中 `ExecutionHistoryAPI` 只有查询和打开文件方法，缺少删除方法。

**当前代码**:

```typescript
export const ExecutionHistoryAPI = {
  listTaskRunRecords: ExecutionHistoryServiceBinding.ListTaskRunRecords,
  openFileWithDefaultApp: ExecutionHistoryServiceBinding.OpenFileWithDefaultApp,
} as const;
```

**缺少方案中使用的**:

- `deleteRunRecord`
- `deleteAllRunRecords`

**涉及文件**:

- `internal/ui/execution_history_service.go` - 实现新方法
- `frontend/src/services/api.ts` - 添加 API 导出

---

## 三、轻微问题 (🟢 低优先级)

### 3.1 错误处理不完善

**来源**: 分析报告 2.6节

**问题描述**:  
删除文件使用异步执行，但未记录失败日志。

**方案中的代码**:

```go
go s.deleteRunFiles(runID, run.RunKind, artifacts)  // 失败无感知
```

**建议**: 至少记录错误日志，便于问题排查。

**涉及文件**: `internal/ui/execution_history_service.go`

---

### 3.2 事务隔离问题

**来源**: 分析报告 2.7节

**问题描述**:  
`DeleteAllRuns` 逐个调用 `DeleteRun`，每次是独立事务。

**方案中的代码**:

```go
for _, runID := range runIDs {
    if err := r.DeleteRun(ctx, runID); err != nil {
        return err  // 中途失败，前面已删除的无法回滚
    }
}
```

**影响**: 如果删除过程中出错，会导致部分删除的不一致状态。

**涉及文件**: `internal/taskexec/persistence.go`

---

### 3.3 Repository 接口扩展不完整

**来源**: 评审报告 问题6

**问题描述**:  
`Repository` 接口缺少删除相关方法，需要扩展。

**需要添加的接口**:

```go
// internal/taskexec/persistence.go
type Repository interface {
    // ... 现有方法

    // 删除方法
    DeleteRun(ctx context.Context, runID string) error
    DeleteAllRuns(ctx context.Context) error
    DeleteRunsByKind(ctx context.Context, runKind string) error
}
```

**涉及文件**: `internal/taskexec/persistence.go`

---

## 四、问题统计

| 问题等级 | 数量 | 说明                                                              |
| -------- | ---- | ----------------------------------------------------------------- |
| 🔴 严重  | 3    | 数据表遗漏、文件路径硬编码、架构职责混乱                          |
| 🟡 中等  | 5    | 文件清理缺失、执行日志遗漏、panic保护、Store方法缺失、API绑定缺失 |
| 🟢 轻微  | 3    | 错误处理、事务隔离、接口扩展                                      |

---

## 五、修复优先级建议

### 第一阶段 (必须修复)

1. **完善数据表删除** - 添加遗漏的9个数据表删除逻辑
2. **使用 PathManager 获取路径** - 替换硬编码路径
3. **调整架构职责** - 将删除逻辑移至 `ExecutionHistoryService`

### 第二阶段 (建议修复)

4. **DeleteAllRuns 添加文件清理** - 删除全部时同步清理文件
5. **补充执行日志删除** - 删除 `live-logs` 目录文件
6. **添加 Panic 保护** - 异步删除时增加 recover
7. **前端 Store 添加方法** - 实现 `removeRunFromHistory` 和 `clearAllHistory`
8. **API 绑定补充** - 添加删除相关 API

### 第三阶段 (优化项)

9. **完善错误日志** - 记录文件删除失败信息
10. **优化事务处理** - 考虑批量删除的事务一致性
11. **扩展 Repository 接口** - 添加删除方法声明

---

## 六、涉及文件清单

| 文件路径                                   | 修改类型      | 涉及问题                          |
| ------------------------------------------ | ------------- | --------------------------------- |
| `internal/taskexec/persistence.go`         | 接口扩展+实现 | 1.1, 3.2, 3.3                     |
| `internal/ui/execution_history_service.go` | 新增方法      | 1.2, 1.3, 2.1, 2.2, 2.3, 2.5, 3.1 |
| `internal/taskexec/service.go`             | 不修改/移除   | 1.3                               |
| `frontend/src/stores/taskexecStore.ts`     | 新增方法      | 2.4                               |
| `frontend/src/services/api.ts`             | 新增导出      | 2.5                               |
