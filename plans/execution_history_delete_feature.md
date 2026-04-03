# 任务执行历史删除功能实现方案

## 1. 需求概述

在任务执行大屏的任务列表中，对于查看执行历史功能：

1. 在每条历史记录上新增一个**删除按钮**
2. 在列表最上面新增一个**删除全部按钮**
3. 删除拓扑采集任务时，同步删除拓扑视图中"选择拓扑运行"选项中的对应记录

## 2. 数据来源分析

### 2.1 核心发现：数据源统一

经过代码分析，确认**任务执行历史和拓扑视图运行选项使用同一个数据源**：

```
┌─────────────────────────────────────────────────────────────────┐
│                        数据流向图                                │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────────┐                                            │
│  │   task_runs     │  (数据库表)                                 │
│  │   表            │                                            │
│  └────────┬────────┘                                            │
│           │                                                     │
│           ▼                                                     │
│  ┌─────────────────┐                                            │
│  │ TaskExecution   │  internal/taskexec/service.go              │
│  │ Service.ListRuns│                                            │
│  └────────┬────────┘                                            │
│           │                                                     │
│           ▼                                                     │
│  ┌─────────────────┐                                            │
│  │ ExecutionHistory│  internal/ui/execution_history_service.go  │
│  │ Service         │                                            │
│  └────────┬────────┘                                            │
│           │                                                     │
│           ▼                                                     │
│  ┌─────────────────┐                                            │
│  │ taskexecStore   │  frontend/src/stores/taskexecStore.ts      │
│  │ .runHistory     │                                            │
│  └────────┬────────┘                                            │
│           │                                                     │
│     ┌─────┴─────┐                                               │
│     │           │                                               │
│     ▼           ▼                                               │
│ ┌───────────┐ ┌───────────────────────────────────┐             │
│ │ 任务执行  │ │ 拓扑视图                          │             │
│ │ 历史抽屉  │ │ topologyRuns = runHistory.filter  │             │
│ │           │ │   (r.runKind === "topology")     │             │
│ └───────────┘ └───────────────────────────────────┘             │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### 2.2 关键代码位置

| 组件         | 文件路径                                                                                                               | 说明                      |
| ------------ | ---------------------------------------------------------------------------------------------------------------------- | ------------------------- |
| 数据模型     | [`internal/taskexec/models.go`](internal/taskexec/models.go:76)                                                        | `TaskRun` 结构体定义      |
| 数据仓库接口 | [`internal/taskexec/persistence.go`](internal/taskexec/persistence.go:11)                                              | `Repository` 接口         |
| 执行服务     | [`internal/taskexec/service.go`](internal/taskexec/service.go:157)                                                     | `TaskExecutionService`    |
| 历史服务     | [`internal/ui/execution_history_service.go`](internal/ui/execution_history_service.go:14)                              | `ExecutionHistoryService` |
| 前端Store    | [`frontend/src/stores/taskexecStore.ts`](frontend/src/stores/taskexecStore.ts:56)                                      | `runHistory` 状态         |
| 历史抽屉     | [`frontend/src/components/task/ExecutionHistoryDrawer.vue`](frontend/src/components/task/ExecutionHistoryDrawer.vue:1) | 执行历史UI组件            |
| 拓扑视图     | [`frontend/src/views/Topology.vue`](frontend/src/views/Topology.vue:389)                                               | `topologyRuns` 计算属性   |

### 2.3 关联数据表

删除一条运行记录需要清理以下数据：

| 表名              | 模型           | 关联字段      | 说明       |
| ----------------- | -------------- | ------------- | ---------- |
| `task_runs`       | `TaskRun`      | `id` (主键)   | 运行主记录 |
| `task_run_stages` | `TaskRunStage` | `task_run_id` | 阶段记录   |
| `task_run_units`  | `TaskRunUnit`  | `task_run_id` | 单元记录   |
| `task_run_events` | `TaskRunEvent` | `task_run_id` | 事件记录   |
| `task_artifacts`  | `TaskArtifact` | `task_run_id` | 产物记录   |

### 2.4 关联文件目录

拓扑采集任务会产生以下文件目录：

```
Dist/netWeaverGoData/
├── topology/
│   ├── raw/
│   │   └── run_{runId}/           # 原始采集数据
│   │       └── {deviceIp}/
│   │           └── *.txt
│   └── normalized/
│       └── run_{runId}/           # 标准化数据
│           └── {deviceIp}/
│               └── *.txt
└── execution/
    └── live-logs/
        └── {timestamp}_task_{ip}_*.log  # 执行日志
```

## 3. 实现方案

### 3.1 后端实现

#### 3.1.1 Repository 接口扩展

在 [`internal/taskexec/persistence.go`](internal/taskexec/persistence.go:11) 的 `Repository` 接口添加：

```go
// DeleteRun 删除运行记录（含级联删除）
DeleteRun(ctx context.Context, runID string) error

// DeleteAllRuns 删除所有运行记录
DeleteAllRuns(ctx context.Context) error

// DeleteRunsByKind 按类型删除运行记录
DeleteRunsByKind(ctx context.Context, runKind string) error
```

#### 3.1.2 GormRepository 实现

在 [`internal/taskexec/persistence.go`](internal/taskexec/persistence.go:43) 的 `GormRepository` 添加实现：

```go
// DeleteRun 删除运行记录（含级联删除关联数据）
func (r *GormRepository) DeleteRun(ctx context.Context, runID string) error {
    // 使用事务确保原子性
    return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
        // 1. 删除关联的 artifacts
        if err := tx.Where("task_run_id = ?", runID).Delete(&TaskArtifact{}).Error; err != nil {
            return err
        }
        // 2. 删除关联的 events
        if err := tx.Where("task_run_id = ?", runID).Delete(&TaskRunEvent{}).Error; err != nil {
            return err
        }
        // 3. 删除关联的 units
        if err := tx.Where("task_run_id = ?", runID).Delete(&TaskRunUnit{}).Error; err != nil {
            return err
        }
        // 4. 删除关联的 stages
        if err := tx.Where("task_run_id = ?", runID).Delete(&TaskRunStage{}).Error; err != nil {
            return err
        }
        // 5. 删除主记录
        if err := tx.Where("id = ?", runID).Delete(&TaskRun{}).Error; err != nil {
            return err
        }
        return nil
    })
}

// DeleteAllRuns 删除所有运行记录
func (r *GormRepository) DeleteAllRuns(ctx context.Context) error {
    // 获取所有 runID
    var runIDs []string
    if err := r.db.WithContext(ctx).Model(&TaskRun{}).Pluck("id", &runIDs).Error; err != nil {
        return err
    }

    // 逐个删除（复用 DeleteRun 确保级联删除）
    for _, runID := range runIDs {
        if err := r.DeleteRun(ctx, runID); err != nil {
            return err
        }
    }
    return nil
}

// DeleteRunsByKind 按类型删除运行记录
func (r *GormRepository) DeleteRunsByKind(ctx context.Context, runKind string) error {
    var runIDs []string
    if err := r.db.WithContext(ctx).Model(&TaskRun{}).Where("run_kind = ?", runKind).Pluck("id", &runIDs).Error; err != nil {
        return err
    }

    for _, runID := range runIDs {
        if err := r.DeleteRun(ctx, runID); err != nil {
            return err
        }
    }
    return nil
}
```

#### 3.1.3 TaskExecutionService 扩展

在 [`internal/taskexec/service.go`](internal/taskexec/service.go:1) 添加：

```go
// DeleteRun 删除运行记录
func (s *TaskExecutionService) DeleteRun(runID string) error {
    // 1. 检查是否正在运行
    run, err := s.repo.GetRun(context.Background(), runID)
    if err != nil {
        return err
    }

    activeStatuses := ActiveRunStatuses()
    for _, status := range activeStatuses {
        if run.Status == string(status) {
            return fmt.Errorf("无法删除正在运行的任务")
        }
    }

    // 2. 获取产物路径用于删除文件
    artifacts, _ := s.repo.GetArtifactsByRun(context.Background(), runID)

    // 3. 删除数据库记录
    if err := s.repo.DeleteRun(context.Background(), runID); err != nil {
        return err
    }

    // 4. 删除关联文件（异步执行，失败不影响主流程）
    go s.deleteRunFiles(runID, run.RunKind, artifacts)

    return nil
}

// DeleteAllRuns 删除所有运行记录
func (s *TaskExecutionService) DeleteAllRuns() error {
    // 检查是否有正在运行的任务
    running, err := s.repo.ListRunningRuns(context.Background())
    if err != nil {
        return err
    }
    if len(running) > 0 {
        return fmt.Errorf("存在 %d 个正在运行的任务，无法删除全部", len(running))
    }

    return s.repo.DeleteAllRuns(context.Background())
}

// deleteRunFiles 删除运行关联的文件
func (s *TaskExecutionService) deleteRunFiles(runID, runKind string, artifacts []TaskArtifact) {
    // 删除拓扑采集文件目录
    if runKind == "topology" {
        rawDir := filepath.Join("Dist", "netWeaverGoData", "topology", "raw", "run_"+runID)
        normalizedDir := filepath.Join("Dist", "netWeaverGoData", "topology", "normalized", "run_"+runID)
        os.RemoveAll(rawDir)
        os.RemoveAll(normalizedDir)
    }

    // 删除产物文件
    for _, artifact := range artifacts {
        if artifact.FilePath != "" {
            os.Remove(artifact.FilePath)
        }
    }
}
```

#### 3.1.4 ExecutionHistoryService API

在 [`internal/ui/execution_history_service.go`](internal/ui/execution_history_service.go:1) 添加：

```go
// DeleteRunRecordRequest 删除运行记录请求
type DeleteRunRecordRequest struct {
    RunID string `json:"runId"`
}

// DeleteRunRecordResponse 删除运行记录响应
type DeleteRunRecordResponse struct {
    Success bool   `json:"success"`
    Message string `json:"message"`
}

// DeleteRunRecord 删除单条运行记录
func (s *ExecutionHistoryService) DeleteRunRecord(runID string) (*DeleteRunRecordResponse, error) {
    if s.taskExecutionService == nil {
        return nil, fmt.Errorf("任务执行服务未初始化")
    }

    if strings.TrimSpace(runID) == "" {
        return &DeleteRunRecordResponse{Success: false, Message: "runID 不能为空"}, nil
    }

    if err := s.taskExecutionService.DeleteRun(runID); err != nil {
        return &DeleteRunRecordResponse{Success: false, Message: err.Error()}, nil
    }

    logger.Info("ExecutionHistoryService", "-", "已删除运行记录: %s", runID)
    return &DeleteRunRecordResponse{Success: true, Message: "删除成功"}, nil
}

// DeleteAllRunRecords 删除所有运行记录
func (s *ExecutionHistoryService) DeleteAllRunRecords() (*DeleteRunRecordResponse, error) {
    if s.taskExecutionService == nil {
        return nil, fmt.Errorf("任务执行服务未初始化")
    }

    if err := s.taskExecutionService.DeleteAllRuns(); err != nil {
        return &DeleteRunRecordResponse{Success: false, Message: err.Error()}, nil
    }

    logger.Info("ExecutionHistoryService", "-", "已删除所有运行记录")
    return &DeleteRunRecordResponse{Success: true, Message: "删除成功"}, nil
}
```

### 3.2 前端实现

#### 3.2.1 API 绑定

在 [`frontend/src/services/api.ts`](frontend/src/services/api.ts:280) 扩展：

```typescript
export const ExecutionHistoryAPI = {
  /** 从统一运行时查询历史记录 */
  listTaskRunRecords: ExecutionHistoryServiceBinding.ListTaskRunRecords,
  /** 使用系统默认应用打开文件 */
  openFileWithDefaultApp: ExecutionHistoryServiceBinding.OpenFileWithDefaultApp,
  /** 删除单条运行记录 */
  deleteRunRecord: ExecutionHistoryServiceBinding.DeleteRunRecord,
  /** 删除所有运行记录 */
  deleteAllRunRecords: ExecutionHistoryServiceBinding.DeleteAllRunRecords,
} as const;
```

#### 3.2.2 Store 状态更新

在 [`frontend/src/stores/taskexecStore.ts`](frontend/src/stores/taskexecStore.ts:1) 添加：

```typescript
// 删除运行记录后的状态更新
function removeRunFromHistory(runId: string) {
  // 从 runHistory 中移除
  runHistory.value = runHistory.value.filter((r) => r.runId !== runId);

  // 清理对应的 snapshot
  delete snapshots.value[runId];

  // 如果删除的是当前运行的记录，重置状态
  if (currentRunId.value === runId) {
    currentRunId.value = null;
  }
}

// 清空所有历史
function clearAllHistory() {
  runHistory.value = [];
  snapshots.value = {};
  currentRunId.value = null;
}

// 暴露方法
return {
  // ... 现有导出
  removeRunFromHistory,
  clearAllHistory,
};
```

#### 3.2.3 ExecutionHistoryDrawer 组件修改

在 [`frontend/src/components/task/ExecutionHistoryDrawer.vue`](frontend/src/components/task/ExecutionHistoryDrawer.vue:1) 中：

**模板部分修改：**

```vue
<template>
  <Teleport to="body">
    <Transition name="drawer">
      <div v-if="modelValue" class="execution-history-drawer">
        <!-- 遮罩 -->
        <div class="drawer-overlay" @click="handleOverlayClick" />

        <!-- 抽屉内容 -->
        <div class="drawer-content">
          <!-- 头部 -->
          <div class="drawer-header">
            <h3 class="drawer-title">
              <i class="icon-history"></i>
              执行历史记录
              <span v-if="taskGroupName" class="task-name">{{
                taskGroupName
              }}</span>
            </h3>
            <div class="header-actions">
              <!-- 新增：删除全部按钮 -->
              <button
                v-if="records.length > 0"
                class="btn-delete-all"
                @click="showDeleteAllConfirm"
                :disabled="deleting"
              >
                <svg
                  class="icon"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="2"
                >
                  <polyline points="3 6 5 6 21 6" />
                  <path
                    d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"
                  />
                </svg>
                删除全部
              </button>
              <button class="btn-close" @click="closeDrawer">
                <span>×</span>
              </button>
            </div>
          </div>

          <!-- 筛选栏 -->
          <div class="drawer-filter">
            <!-- 保持原有筛选 -->
          </div>

          <!-- 列表内容 -->
          <div class="drawer-body">
            <!-- 加载状态 -->
            <div v-if="loading" class="loading-state">...</div>

            <!-- 空状态 -->
            <div v-else-if="records.length === 0" class="empty-state">...</div>

            <!-- 记录列表 -->
            <div v-else class="record-list">
              <div
                v-for="record in records"
                :key="record.id"
                class="record-item"
                :class="`status-${record.status}`"
              >
                <div class="record-header">
                  <span
                    class="record-status"
                    :class="`status-${record.status}`"
                  >
                    {{ getStatusText(record.status) }}
                  </span>
                  <span class="record-time">{{
                    formatTime(record.startedAt)
                  }}</span>
                </div>

                <div class="record-info" @click="showDetail(record)">
                  <div class="record-task">{{ record.taskName }}</div>
                  <div class="record-stats">
                    <span>设备: {{ record.totalDevices }}</span>
                    <span class="success">成功: {{ record.successCount }}</span>
                    <span class="error" v-if="record.errorCount > 0"
                      >失败: {{ record.errorCount }}</span
                    >
                    <span class="duration">{{
                      formatDuration(record.durationMs)
                    }}</span>
                  </div>
                </div>

                <!-- 新增：删除按钮 -->
                <div class="record-actions">
                  <button
                    class="btn-delete"
                    @click.stop="showDeleteConfirm(record)"
                    :disabled="deleting"
                    title="删除此记录"
                  >
                    <svg
                      class="icon"
                      viewBox="0 0 24 24"
                      fill="none"
                      stroke="currentColor"
                      stroke-width="2"
                    >
                      <polyline points="3 6 5 6 21 6" />
                      <path
                        d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"
                      />
                    </svg>
                  </button>
                </div>
              </div>
            </div>

            <!-- 分页 -->
            <div v-if="totalPages > 1" class="pagination">...</div>
          </div>
        </div>

        <!-- 详情弹窗 -->
        <ExecutionRecordDetail ... />

        <!-- 新增：删除确认弹窗 -->
        <Transition name="modal">
          <div
            v-if="deleteConfirm.show"
            class="confirm-modal-overlay"
            @click="deleteConfirm.show = false"
          >
            <div class="confirm-modal" @click.stop>
              <div class="confirm-icon">
                <svg
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="2"
                >
                  <circle cx="12" cy="12" r="10" />
                  <line x1="12" y1="8" x2="12" y2="12" />
                  <line x1="12" y1="16" x2="12.01" y2="16" />
                </svg>
              </div>
              <h3 class="confirm-title">确认删除</h3>
              <p class="confirm-message">
                {{
                  deleteConfirm.isAll
                    ? "确定要删除所有执行历史记录吗？此操作不可撤销。"
                    : `确定要删除记录「${deleteConfirm.taskName}」吗？此操作不可撤销。`
                }}
              </p>
              <div class="confirm-actions">
                <button class="btn-cancel" @click="deleteConfirm.show = false">
                  取消
                </button>
                <button
                  class="btn-confirm-delete"
                  @click="executeDelete"
                  :disabled="deleting"
                >
                  {{ deleting ? "删除中..." : "确认删除" }}
                </button>
              </div>
            </div>
          </div>
        </Transition>
      </div>
    </Transition>
  </Teleport>
</template>
```

**脚本部分修改：**

```typescript
<script setup lang="ts">
import { ref, watch } from 'vue'
import { ExecutionHistoryAPI } from '../../services/api'
import type { ExecutionHistoryRecord } from '../../types/executionHistory'
import ExecutionRecordDetail from './ExecutionRecordDetail.vue'
import { useTaskexecStore } from '../../stores/taskexec'

const props = defineProps<{
  modelValue: boolean
  taskGroupId?: string
  taskGroupName?: string
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void
}>()

const taskexecStore = useTaskexecStore()

// 状态
const loading = ref(false)
const records = ref<ExecutionHistoryRecord[]>([])
const filterStatus = ref('')
const currentPage = ref(1)
const pageSize = 10
const total = ref(0)
const totalPages = ref(0)

// 详情弹窗
const showDetailModal = ref(false)
const selectedRecord = ref<ExecutionHistoryRecord | null>(null)

// 新增：删除相关状态
const deleting = ref(false)
const deleteConfirm = ref({
  show: false,
  runId: '',
  taskName: '',
  isAll: false
})

// ... 保持原有的 loadRecords, changePage, closeDrawer, showDetail 等方法

// 新增：显示删除确认弹窗
const showDeleteConfirm = (record: ExecutionHistoryRecord) => {
  deleteConfirm.value = {
    show: true,
    runId: record.id,
    taskName: record.taskName,
    isAll: false
  }
}

// 新增：显示删除全部确认弹窗
const showDeleteAllConfirm = () => {
  deleteConfirm.value = {
    show: true,
    runId: '',
    taskName: '',
    isAll: true
  }
}

// 新增：执行删除
const executeDelete = async () => {
  deleting.value = true
  try {
    if (deleteConfirm.value.isAll) {
      const result = await ExecutionHistoryAPI.deleteAllRunRecords()
      if (result?.success) {
        // 清空 Store 中的历史
        taskexecStore.clearAllHistory()
        // 重新加载列表
        await loadRecords()
      } else {
        console.error('删除失败:', result?.message)
      }
    } else {
      const result = await ExecutionHistoryAPI.deleteRunRecord(deleteConfirm.value.runId)
      if (result?.success) {
        // 从 Store 中移除
        taskexecStore.removeRunFromHistory(deleteConfirm.value.runId)
        // 重新加载列表
        await loadRecords()
      } else {
        console.error('删除失败:', result?.message)
      }
    }
  } catch (error) {
    console.error('删除失败:', error)
  } finally {
    deleting.value = false
    deleteConfirm.value.show = false
  }
}

// ... 其他方法保持不变
</script>
```

**样式部分添加：**

```css
/* 删除按钮样式 */
.record-actions {
  display: flex;
  align-items: center;
  padding-left: 8px;
}

.btn-delete {
  padding: 6px;
  border: none;
  background: transparent;
  color: var(--text-muted);
  cursor: pointer;
  border-radius: 4px;
  transition: all 0.2s;
}

.btn-delete:hover:not(:disabled) {
  color: var(--error);
  background: var(--error-bg);
}

.btn-delete:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.btn-delete .icon {
  width: 16px;
  height: 16px;
}

/* 头部操作区域 */
.header-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.btn-delete-all {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 6px 12px;
  border: 1px solid var(--error-border);
  background: var(--error-bg);
  color: var(--error);
  font-size: 12px;
  font-weight: 500;
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.2s;
}

.btn-delete-all:hover:not(:disabled) {
  background: var(--error);
  color: white;
}

.btn-delete-all:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.btn-delete-all .icon {
  width: 14px;
  height: 14px;
}

/* 确认弹窗样式 */
.confirm-modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.6);
  backdrop-filter: blur(4px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.confirm-modal {
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 24px;
  max-width: 400px;
  width: 90%;
  text-align: center;
}

.confirm-icon {
  width: 48px;
  height: 48px;
  margin: 0 auto 16px;
  color: var(--warning);
}

.confirm-icon svg {
  width: 100%;
  height: 100%;
}

.confirm-title {
  font-size: 16px;
  font-weight: 600;
  color: var(--text-primary);
  margin-bottom: 8px;
}

.confirm-message {
  font-size: 13px;
  color: var(--text-muted);
  margin-bottom: 20px;
  line-height: 1.5;
}

.confirm-actions {
  display: flex;
  gap: 12px;
  justify-content: center;
}

.btn-cancel {
  padding: 8px 20px;
  border: 1px solid var(--border);
  background: var(--bg-panel);
  color: var(--text-secondary);
  font-size: 13px;
  font-weight: 500;
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.2s;
}

.btn-cancel:hover {
  border-color: var(--accent);
  color: var(--text-primary);
}

.btn-confirm-delete {
  padding: 8px 20px;
  border: none;
  background: var(--error);
  color: white;
  font-size: 13px;
  font-weight: 500;
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.2s;
}

.btn-confirm-delete:hover:not(:disabled) {
  background: var(--error-dark);
}

.btn-confirm-delete:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

/* 过渡动画 */
.modal-enter-active,
.modal-leave-active {
  transition: opacity 0.2s ease;
}

.modal-enter-from,
.modal-leave-to {
  opacity: 0;
}
```

## 4. 数据流验证

### 4.1 删除单条记录流程

```
用户点击删除按钮
    ↓
显示确认弹窗
    ↓
用户确认删除
    ↓
调用 ExecutionHistoryAPI.deleteRunRecord(runId)
    ↓
后端 ExecutionHistoryService.DeleteRunRecord(runId)
    ↓
TaskExecutionService.DeleteRun(runId)
    ↓
Repository.DeleteRun(ctx, runId) [事务删除]
    ├── 删除 task_artifacts
    ├── 删除 task_run_events
    ├── 删除 task_run_units
    ├── 删除 task_run_stages
    └── 删除 task_runs
    ↓
异步删除文件目录
    ↓
返回成功
    ↓
前端 taskexecStore.removeRunFromHistory(runId)
    ↓
重新加载列表
    ↓
拓扑视图 topologyRuns 自动更新（因为是从 runHistory 筛选的）
```

### 4.2 删除全部记录流程

```
用户点击删除全部按钮
    ↓
显示确认弹窗
    ↓
用户确认删除
    ↓
调用 ExecutionHistoryAPI.deleteAllRunRecords()
    ↓
后端检查是否有运行中的任务
    ↓
遍历删除所有记录（复用单条删除逻辑）
    ↓
返回成功
    ↓
前端 taskexecStore.clearAllHistory()
    ↓
重新加载列表（应为空）
    ↓
拓扑视图 topologyRuns 自动清空
```

## 5. 注意事项

### 5.1 安全检查

1. **禁止删除运行中的任务**：在删除前检查任务状态，如果状态为 `pending` 或 `running`，拒绝删除
2. **事务保证**：数据库删除操作使用事务，确保关联数据一致性
3. **文件删除异步化**：文件系统操作放在 goroutine 中异步执行，避免阻塞主流程

### 5.2 用户体验

1. **二次确认**：删除操作需要用户确认，防止误操作
2. **删除中状态**：删除过程中禁用按钮，显示加载状态
3. **错误提示**：删除失败时显示具体错误信息

### 5.3 数据一致性

1. **Store 同步更新**：删除成功后立即更新前端 Store，无需等待重新请求
2. **拓扑视图联动**：由于拓扑视图的运行选项是从 `runHistory` 筛选的，删除后自动同步

## 6. 测试要点

1. **单条删除测试**
   - 删除普通任务历史记录
   - 删除拓扑采集任务历史记录
   - 验证拓扑视图选项同步消失
   - 验证文件目录被删除

2. **删除全部测试**
   - 删除所有历史记录
   - 验证列表清空
   - 验证拓扑视图选项清空

3. **边界情况测试**
   - 尝试删除运行中的任务（应失败）
   - 删除不存在的记录
   - 空列表时删除全部按钮状态

4. **并发测试**
   - 删除过程中刷新页面
   - 多次快速点击删除按钮
