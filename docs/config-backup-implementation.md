# SFTP 配置备份功能 - 分阶段实施方案

> 基于 `config-backup-design.md` 设计文档，本方案将实施分为4个阶段，每阶段可独立验证。
> 已通过审计修复，补充了遗漏的修改点和验证项。

---

## 阶段1：后端模型与枚举扩展

**目标**：扩展数据模型和枚举，为后续编译器/执行器提供数据基础。

### 1.1 修改 `internal/taskexec/status.go`

- 新增 `RunKindBackup RunKind = "backup"`
- 新增 `StageKindBackupCollect StageKind = "backup_collect"`
- 新增 `ArtifactTypeBackupConfig ArtifactType = "backup_config"`

### 1.2 修改 `internal/taskexec/config_models.go`

- 新增 `BackupTaskConfig` 结构体（含 DeviceIDs/DeviceIPs/Concurrency/TimeoutSec/StartupCommand/SaveRootPath/DirNamePattern/FileNamePattern/EnableRawLog）

### 1.3 修改 `internal/models/models.go`

- `TaskGroup` 新增4个字段：`BackupSaveRootPath`/`BackupDirNamePattern`/`BackupFileNamePattern`/`BackupStartupCommand`
- 添加 gorm 默认值标签（`default:''`/`default:'%Y%M%D'`/`default:'%H.cfg'`/`default:'display startup'`）
- `TaskType` 注释更新为 `"normal" | "topology" | "backup"`

### 1.4 修改 `internal/config/paths.go`

- 新增 `resolvePathPattern(pattern, deviceIP string, t time.Time) string` 函数（模板变量解析 + IPv6 冒号清洗 + `filepath.Clean`）
- 新增 `buildBackupLocalPath(saveRoot, dirPattern, filePattern, deviceIP string, timestamp time.Time) string` 函数（路径拼接 + 路径逃逸防护）
- 新增 `GetBackupConfigFilePath(saveRoot, dirPattern, filePattern, deviceIP string, timestamp time.Time) string` 方法（对外入口，saveRoot 为空时使用默认 BackupConfigDir）
- 与现有 `GetBackupFilePath` 的关系：`GetBackupFilePath` 是简单路径拼接，`GetBackupConfigFilePath` 增加模板变量解析，两者并存
- **架构约束**：`resolvePathPattern`、`buildBackupLocalPath`、`GetBackupConfigFilePath` 统一收口在 `paths.go` 中，执行器不自行实现路径解析逻辑

### 1.5 ~~修改 `internal/ui/view_models.go`~~（已移除）

> **移除原因**：`TaskGroupListView` 专门用于前端任务列表页展示，列表页不需要展示保存路径、命名模板等深度配置细节。将这些细分类型的配置加入全局列表视图，会造成不必要的网络 Payload 负担和模型污染。前端在"编辑任务"时使用详情 API（其返回的 `TaskGroupDetailViewModel` 包含了完整的 `models.TaskGroup` 实体，已自带这些备份字段）。

### 1.6 ~~修改 `internal/ui/task_group_service.go`~~（已移除）

> **移除原因**：同 1.5，`ListTaskGroups()` 和 `normalizeTaskGroupListView()` 不需要处理备份专用字段。

### 1.7 验证

- 编译通过：`go build ./...`
- GORM AutoMigrate 正确创建新列
- 现有 normal/topology 类型 TaskGroup 的备份字段默认值为空字符串
- `resolvePathPattern` 单元测试：各变量替换正确、IPv6 冒号清洗、`filepath.Clean` 路径遍历防护
- `buildBackupLocalPath` 单元测试：路径逃逸防护、默认路径生成

---

## 阶段2：备份编译器与执行器

**目标**：实现备份任务的核心执行逻辑。

### 2.1 新增 `internal/taskexec/backup_compiler.go`

- 实现 `BackupTaskCompiler` 结构体
- 实现 `Supports()` 方法（返回 `kind == "backup"`）
- 实现 `Compile()` 方法：
  - 解析 `BackupTaskConfig`
  - 校验至少一台设备
  - 为每台设备创建 Unit（含2个Step：backup_query_startup + backup_sftp_download）
  - 创建 Stage（Kind=backup_collect，默认并发5）
  - 返回 ExecutionPlan

### 2.2 新增 `internal/taskexec/backup_executor.go`

- 实现 `BackupExecutor` 结构体（含 repo/settings/pathManager）
- 实现 `Kind()` 方法（返回 `string(StageKindBackupCollect)`）
- 实现 `Run()` 方法（并发控制，与 DeviceCommandExecutor 模式一致）
- 实现 `executeBackupUnit()` 方法（与编译器生成的 2 个 Step 对齐，基于 StepIndex 发射事件）：
  1. 标记 Unit running
  2. 查找设备信息
  3. **[step-0] 发射 `EventStepStarted(stepIndex=0)`**
  4. SSH 连接 + 执行 display startup
  5. 正则提取配置文件路径
  6. 提取失败则发射 `EventStepFinished(stepIndex=0, error)`，标记 Unit 失败并返回
  7. **发射 `EventStepFinished(stepIndex=0)`**
  8. **[step-1] 发射 `EventStepStarted(stepIndex=1)`**
  9. **调用 `pathManager.GetBackupConfigFilePath()` 获取本地保存路径**（路径解析逻辑统一由 PathManager 处理）
  10. SFTP 连接 + 下载配置文件（采用 Truncate 覆盖策略）
  11. **发射 `EventStepFinished(stepIndex=1)`**
  12. 更新状态 + 创建产物记录
- 实现 `extractNextStartupConfigPath()` 函数
- 实现 `downloadConfigFile()` 方法
- **不实现** `resolvePathPattern()` 和 `buildBackupLocalPath()`（已统一收口在 `internal/config/paths.go` 中）

### 2.3 修改 `internal/taskexec/service.go`

- 注册备份编译器：`compilerReg.Register(string(RunKindBackup), NewBackupTaskCompiler(nil))`
  - 注意：与现有 `NewNormalTaskCompiler(nil)` 一致，传入 nil 使用默认 CompileOptions
- 注册备份执行器：`runtime.RegisterExecutor(NewBackupExecutor(repository.NewDeviceRepository()))`
  - 注意：使用 `RegisterExecutor()` 方法（与现有执行器注册方式一致），通过 `Kind()` 自动映射

### 2.4 验证

- 编译通过
- 单元测试：正则提取（正常/NULL/无匹配）
- 单元测试：编译器编译（正常/无设备）
- 单元测试：执行器 Step 事件上报（step-0/step-1 的 EventStepStarted/EventStepFinished 按正确顺序和 StepIndex 发射）

---

## 阶段3：启动服务与配置层扩展

**目标**：打通从 TaskGroup 到 TaskDefinition 的完整数据链路。

### 3.1 修改 `internal/taskexec/launch_service.go`

- `CanonicalLaunchSpec` 新增 `Backup *CanonicalBackup` 字段
- 新增 `CanonicalBackup` 结构体
- **`normalizeRunKind()`** 修改为 switch 结构，新增 `case string(RunKindBackup):` 分支
  - **关键**：未修改此函数会导致 backup 被错误归为 normal，整个链路断裂
- `NormalizeTaskGroup()` switch 中新增 `case string(RunKindBackup):` 分支 -> `normalizeBackup()`
  - 注意：在 `case string(RunKindTopology):` 之后、`default:` 之前插入
- `ValidateLaunchSpec()` switch 中新增 `case string(RunKindBackup):` 分支
  - 校验：至少一台设备（不需要命令）
- `CreateTaskDefinitionFromLaunchSpec()` switch 中新增 `case string(RunKindBackup):` 分支
  - 构建 `BackupTaskConfig` 并序列化
- `specTargetIPs()` 新增 `if spec.Backup != nil` 分支
  - **关键**：未修改会导致活跃运行冲突检测遗漏备份任务设备

### 3.2 修改 `internal/taskexec/runtime.go`

- `extractEnableRawLog()` switch 中新增 `case string(RunKindBackup):` 分支
  - 在 `case string(RunKindTopology):` 之后、`default:` 之前插入
  - 反序列化 `BackupTaskConfig`，返回 `cfg.EnableRawLog`

### 3.3 修改 `internal/config/task_group.go`

- `validateTaskGroup()` 新增 `backup` 类型校验逻辑
  - 在 `if strings.TrimSpace(group.TaskType) != "topology"` 判断之前插入 backup 分支
  - 校验：路径模板非空、启动命令非空
- `normalizeTaskGroup()` 新增备份字段默认值处理
  - 当 `TaskType == "backup"` 时，为空的备份模板字段填充默认值

### 3.4 验证

- 编译通过
- 单元测试：`normalizeRunKind("backup") == "backup"`
- 单元测试：`ValidateLaunchSpec` 对 backup 类型 spec 的校验
- 单元测试：`specTargetIPs` 对含 Backup 的 spec 返回正确设备IP
- 单元测试：`CreateTaskDefinitionFromLaunchSpec` 对 backup spec 生成正确 TaskDefinition
- 创建备份任务组 -> 启动 -> 编译 -> 执行 链路通畅

---

## 阶段4：前端页面改造

**目标**：实现备份任务的创建、编辑、执行前端交互。

### 4.1 修改 `frontend/src/views/Tasks.vue`

- 任务类型按钮组新增"配置备份"选项（与"普通任务"/"拓扑采集任务"并列）
- `selectedTaskType` 类型扩展为 `"normal" | "topology" | "backup"`
- 选择 backup 时显示备份专用配置区域（设备选择器、路径模板、启动命令等）
- 备份任务不需要命令组选择
- `canCreate` 计算属性新增 backup 分支（只需设备，不需要命令组）
- 提交逻辑新增 backup 分支

### 4.2 修改 `frontend/src/views/TaskExecution.vue`

- 备份任务卡片显示"配置备份"标签和特殊图标
- "开始备份"按钮（替代"开始执行"）

### 4.3 修改 `frontend/src/components/task/TaskEditModal.vue`

- 新增 `isBackupTaskValue` 计算属性
- 备份任务编辑区域（使用 `v-else-if="isBackupTaskValue"`）
- `submit()` 新增 backup 分支
- `hydrateForm()` 新增 backup 分支

### 4.4 新增 `frontend/src/components/task/PathTemplateHelpPopover.vue`

- 变量说明弹窗组件（问号按钮触发）
- 展示 %H/%Y/%M/%D/%h/%m/%s 变量说明
- 特别标注 %M(月) vs %m(分) 的区别

### 4.5 前端类型定义更新

- 运行 `wails3 generate bindings` 更新前端 TypeScript 类型绑定
- `TaskGroupListView` 接口**不包含**备份专用字段（列表页不需要这些深度配置）
- 前端"编辑任务"时通过详情 API 获取完整的 `TaskGroup` 实体（包含备份字段）

### 4.6 验证

- 前端编译通过：`npm run build`
- Wails 绑定生成成功
- 创建备份任务 -> 选择设备 -> 开始备份 -> 查看进度 完整流程

---

## 实施依赖关系

```
阶段1 (模型/枚举/路径解析逻辑)
  -> 阶段2 (编译器/执行器)  [依赖阶段1的枚举、模型和PathManager路径解析方法]
    -> 阶段3 (启动服务)     [依赖阶段2的编译器/执行器]
      -> 阶段4 (前端)       [依赖阶段3的完整后端链路]
```

## 每阶段验证检查清单

| 阶段 | 验证项 |
|------|--------|
| 1 | `go build ./...` 编译通过 |
| 1 | 数据库迁移成功，task_groups 表新增4列 |
| 1 | 现有 TaskGroup 的备份字段默认值为空字符串 |
| 1 | `resolvePathPattern` 单元测试通过（变量替换/IPv6冒号清洗/路径遍历防护） |
| 1 | `buildBackupLocalPath` 单元测试通过（路径逃逸防护/默认路径生成） |
| 2 | `go build ./...` 编译通过 |
| 2 | 正则提取单元测试通过（正常/NULL/无匹配） |
| 2 | 编译器编译单元测试通过 |
| 2 | 执行器 Step 事件上报单元测试通过（step-0/step-1 事件顺序和 StepIndex 正确） |
| 3 | `go build ./...` 编译通过 |
| 3 | `normalizeRunKind("backup") == "backup"` |
| 3 | `ValidateLaunchSpec` 对 backup spec 校验正确 |
| 3 | `specTargetIPs` 对 backup spec 返回正确设备IP |
| 3 | `CreateTaskDefinitionFromLaunchSpec` 对 backup 生成正确 TaskDefinition |
| 3 | 创建 backup TaskGroup 并启动，编译器/执行器正确匹配 |
| 4 | 前端 `npm run build` 编译通过 |
| 4 | Wails 绑定生成成功 |
| 4 | 完整端到端流程验证 |
