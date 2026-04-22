# SFTP 配置备份功能 - 详细设计文档

## 1. 需求概述

### 1.1 功能目标

在任务执行页面增加"配置备份"任务类型，通过 SSH 连接网络设备，执行 `display startup` 命令获取下次启动配置文件路径，再通过 SFTP 下载该配置文件到本地，实现网络设备配置的批量备份。

### 1.2 核心需求

| 编号 | 需求 | 优先级 |
|------|------|--------|
| R1 | 任务执行页面增加备份任务入口，默认不选设备、命令留空 | P0 |
| R2 | 备份任务使用特殊任务类型标识 `taskType="backup"` | P0 |
| R3 | 用户可选择设备加入备份任务 | P0 |
| R4 | 点击"开始备份"后，SSH连接设备执行 `display startup` 获取配置文件路径 | P0 |
| R5 | 正则提取 `Next startup saved-configuration file` 对应的文件路径 | P0 |
| R6 | SFTP连接设备下载配置文件 | P0 |
| R7 | 默认保存到程序运行目录下 `yyyymmdd` 目录，文件名 `设备IP.cfg` | P0 |
| R8 | 编辑任务页面允许设置保存根路径、路径命名模板和文件命名模板 | P1 |
| R9 | 路径/文件命名支持变量：`%H`(主机IP)、`%Y`(年)、`%M`(月)、`%D`(日)、`%h`(时)、`%m`(分)、`%s`(秒) | P1 |
| R10 | 变量说明通过问号按钮展示 | P1 |

### 1.3 `display startup` 回显示例

```
<CE1>display startup
MainBoard:
  Configured startup system software:        flash:/ensp-ce.cc
  Startup system software:                   flash:/ensp-ce.cc
  Next startup system software:              flash:/ensp-ce.cc
  Startup saved-configuration file:          NULL
  Next startup saved-configuration file:     flash:/1.cfg
  Startup paf file:                          default
  Next startup paf file:                     default
  Startup patch package:                     NULL
  Next startup patch package:                NULL
  Startup feature software:                  NULL
  Next startup feature software:             NULL
```

**提取目标**：`Next startup saved-configuration file:     flash:/1.cfg` 中的 `flash:/1.cfg`

---

## 2. 架构设计

### 2.1 整体架构

配置备份功能作为**新的任务类型** (`backup`) 集成到现有 `taskexec` 统一任务执行运行时中，复用以下现有模块：

- **SSH 连接**：`internal/sshutil` - SSHClient 建连与命令执行
- **SFTP 传输**：`internal/sftputil` - SFTPClient 文件下载
- **任务执行运行时**：`internal/taskexec` - 编译器、执行器、状态管理、事件系统
- **设备仓库**：`internal/repository` - 设备信息查询
- **路径管理**：`internal/config/paths.go` - PathManager 统一路径管理
- **前端 Store**：`taskexecStore` - 执行状态实时同步

### 2.2 新增/修改模块一览

| 模块 | 文件 | 变更类型 | 说明 |
|------|------|----------|------|
| 状态枚举 | `taskexec/status.go` | 修改 | 新增 `RunKindBackup`、`StageKindBackupCollect` |
| 配置模型 | `taskexec/config_models.go` | 修改 | 新增 `BackupTaskConfig` |
| 编译器 | `taskexec/backup_compiler.go` | **新增** | 备份任务编译器 |
| 执行器 | `taskexec/backup_executor.go` | **新增** | 备份任务执行器（SSH+SFTP） |
| 编译器注册 | `taskexec/service.go` | 修改 | 注册备份编译器 |
| 执行器注册 | `taskexec/runtime.go` | 修改 | 注册备份执行器 |
| 启动服务 | `taskexec/launch_service.go` | 修改 | 支持备份任务归一化与校验 |
| 任务组模型 | `models/models.go` | 修改 | TaskGroup 新增备份相关字段 |
| 路径管理 | `config/paths.go` | 修改 | 新增备份文件路径生成方法 |
| UI 服务 | `ui/task_group_service.go` | 修改 | 支持备份任务组 CRUD |
| 前端-任务执行 | `views/TaskExecution.vue` | 修改 | 新增备份任务卡片与交互 |
| 前端-任务编辑 | `components/task/TaskEditModal.vue` | 修改 | 备份任务编辑（路径模板等） |
| 前端-任务创建 | `views/Tasks.vue` | 修改 | 新增备份任务创建入口 |

### 2.3 执行流程

```
用户点击"开始备份"
  -> TaskLaunchService.StartTaskGroup()
    -> BackupTaskCompiler.Compile()          // 编译为执行计划
    -> RuntimeManager.Execute()              // 创建 TaskRun，异步执行
      -> executePlan()
        -> BackupExecutor.Run()              // 备份执行器
          -> 并发控制（信号量，默认5）
          -> 每设备 goroutine:
            1. SSH 连接设备
            2. 执行 display startup
            3. 正则提取配置文件路径
            4. SFTP 连接设备（独立连接）
            5. 下载配置文件到本地
            6. 关闭连接，更新状态
```

---

## 3. 详细设计

### 3.1 状态枚举扩展

**文件**：`internal/taskexec/status.go`

新增常量：

```go
// RunKind 新增
RunKindBackup RunKind = "backup"

// StageKind 新增
StageKindBackupCollect StageKind = "backup_collect"

// ArtifactType 新增
ArtifactTypeBackupConfig ArtifactType = "backup_config"
```

### 3.1.1 特殊任务类型标识（需求R2）

备份任务通过 `TaskGroup.TaskType = "backup"` 进行类型标识，而非使用特殊ID值。原因：
- `TaskGroup.ID` 为 uint 自增主键，不适合承载类型语义
- `TaskType` 字段天然用于区分任务类型，前端/后端均可通过此字段判断
- 与现有 `normal`/`topology` 类型体系一致

前端通过 `task.taskType === "backup"` 判断是否为备份任务，展示专用UI。

### 3.2 备份任务配置模型

**文件**：`internal/taskexec/config_models.go`

```go
// BackupTaskConfig 备份任务配置
type BackupTaskConfig struct {
    // 设备选择
    DeviceIDs []uint   `json:"deviceIDs"`
    DeviceIPs []string `json:"deviceIPs"`

    // 执行选项
    Concurrency int `json:"concurrency"`  // 并发数（默认5）
    TimeoutSec  int `json:"timeoutSec"`   // 超时(秒)（默认300）

    // 备份命令配置
    StartupCommand string `json:"startupCommand"` // 获取启动配置的命令，默认 "display startup"

    // 保存路径配置
    SaveRootPath    string `json:"saveRootPath"`    // 保存根路径（空=使用默认 BackupConfigDir）
    DirNamePattern  string `json:"dirNamePattern"`  // 目录命名模板，默认 "%Y%M%D"
    FileNamePattern string `json:"fileNamePattern"` // 文件命名模板，默认 "%H.cfg"

    // 日志选项
    EnableRawLog bool `json:"enableRawLog"`
}
```

### 3.3 路径模板变量系统

#### 3.3.1 支持的变量

| 变量 | 含义 | 示例值 |
|------|------|--------|
| `%H` | 设备主机IP（IPv6冒号自动替换为短横线） | `192.168.1.1` / `fe80--1` |
| `%Y` | 四位年份 | `2026` |
| `%M` | 两位月份 | `04` |
| `%D` | 两位日期 | `22` |
| `%h` | 两位小时(24h) | `14` |
| `%m` | 两位分钟 | `30` |
| `%s` | 两位秒 | `05` |

> **IPv6 说明**：IPv6 地址中的冒号 `:` 在文件路径中为非法字符（尤其在 Windows 上），`resolvePathPattern` 在替换 `%H` 时会自动将冒号替换为短横线 `-`（如 `fe80::1` → `fe80--1`）。

#### 3.3.2 模板解析与路径生成

> **架构约束**：路径模板解析（`resolvePathPattern`）、路径拼接（`buildBackupLocalPath`）及路径安全防御逻辑**统一收口在 `internal/config/paths.go` 的 `PathManager.GetBackupConfigFilePath` 方法中**（见 3.8）。执行器 `BackupExecutor` 仅负责调用该方法获取最终本地路径，不自行实现路径解析逻辑。

模板解析函数 `resolvePathPattern` 和完整路径生成函数 `buildBackupLocalPath` 的详细实现见 **3.8 PathManager 扩展**。

**默认行为**：
- `saveRoot` = PathManager.BackupConfigDir（即 `<StorageRoot>/backup/config`）
- `dirPattern` = `%Y%M%D` -> 生成如 `20260422`
- `filePattern` = `%H.cfg` -> 生成如 `192.168.1.1.cfg`
- 最终路径：`<StorageRoot>/backup/config/20260422/192.168.1.1.cfg`

**安全防护**：
- `resolvePathPattern` 结果经过 `filepath.Clean` 清洗，防止 `../` 路径遍历
- `buildBackupLocalPath` 校验最终路径必须在 `saveRoot` 之下，防止路径逃逸
- 设备IP中的 `.` 在路径中合法，但异常IP（含 `..`）会被 `filepath.Clean` 清洗
- IPv6 地址中的冒号 `:` 在 Windows 文件路径中为非法字符，`resolvePathPattern` 在替换 `%H` 时会对 IP 值进行清洗（将冒号替换为 `-`），详见 3.8

### 3.4 正则提取配置文件路径

#### 3.4.1 正则表达式

```go
var nextStartupConfigRegex = regexp.MustCompile(
    `Next\s+startup\s+saved-configuration\s+file:\s+(\S+)`,
)
```

#### 3.4.2 提取函数

```go
// extractNextStartupConfigPath 从 display startup 输出中提取下次启动配置文件路径
func extractNextStartupConfigPath(output string) string {
    matches := nextStartupConfigRegex.FindStringSubmatch(output)
    if len(matches) < 2 {
        return ""
    }
    path := strings.TrimSpace(matches[1])
    if strings.EqualFold(path, "NULL") || path == "" {
        return ""
    }
    return path
}
```

**处理边界情况**：
- 输出中无匹配行 -> 返回空，标记为"未找到配置文件路径"
- 匹配值为 `NULL` -> 返回空，标记为"设备无下次启动配置文件"
- 匹配值为空 -> 返回空，标记为"配置文件路径为空"

### 3.5 备份任务编译器

**文件**：`internal/taskexec/backup_compiler.go`（新增）

编译器将 `BackupTaskConfig` 编译为 `ExecutionPlan`，每个设备生成一个 Unit，每个 Unit 包含两个 Step：
- `step-0`: `backup_query_startup` - SSH 执行 display startup
- `step-1`: `backup_sftp_download` - SFTP 下载配置文件

```go
type BackupTaskCompiler struct {
    options *CompileOptions
}

func (c *BackupTaskCompiler) Supports(kind string) bool {
    return kind == string(RunKindBackup)
}

func (c *BackupTaskCompiler) Compile(ctx context.Context, def *TaskDefinition) (*ExecutionPlan, error) {
    // 解析 BackupTaskConfig
    // 校验至少一台设备
    // 为每台设备创建 Unit（含2个Step）
    // 创建 Stage（Kind=backup_collect，默认并发5）
    // 返回 ExecutionPlan
}
```

### 3.6 备份任务执行器

**文件**：`internal/taskexec/backup_executor.go`（新增）

#### 3.6.1 结构体

```go
type BackupExecutor struct {
    repo     repository.DeviceRepository
    settings *models.GlobalSettings
}
```

#### 3.6.2 执行流程

`Run()` 方法与 `DeviceCommandExecutor.Run()` 采用相同的并发控制模式（信号量 + WaitGroup），每个 Unit 调用 `executeBackupUnit()`。

`executeBackupUnit()` 流程（与编译器生成的 2 个 Step 对齐）：

1. 标记 Unit 为 running
2. 从 `unit.Target` 获取设备IP
3. 从 `repo` 查找设备信息
4. **[step-0] 发射 `EventStepStarted(stepIndex=0)`**，日志："Querying startup config..."
5. **SSH 连接设备**（复用 `executor.NewDeviceExecutor`）
6. **执行 display startup**（通过 executor 执行命令）
7. **正则提取配置文件路径**（`extractNextStartupConfigPath`）
8. 路径为空则标记 step-0 失败，**发射 `EventStepFinished(stepIndex=0, error)`**，标记 Unit 失败并返回
9. **发射 `EventStepFinished(stepIndex=0)`**，日志："Startup config path: {remotePath}"
10. **[step-1] 发射 `EventStepStarted(stepIndex=1)`**，日志："Downloading config file..."
11. **调用 `PathManager.GetBackupConfigFilePath()` 获取本地保存路径**（路径解析逻辑统一由 PathManager 处理，执行器不自行实现）
12. **SFTP 连接设备**（`sftputil.NewSFTPClient`，独立SSH连接）
13. **下载配置文件**（`sftpClient.DownloadFile`，采用 Truncate 覆盖策略）
14. 关闭 SFTP 和 SSH 连接
15. **发射 `EventStepFinished(stepIndex=1)`**，日志："Config file saved: {localPath}"
16. 标记 Unit 完成，创建产物记录

> **关键约束**：执行器必须基于 StepIndex 向外部发射 `EventStepStarted`、`EventStepFinished` 及 `EventLog` 事件，且日志输出携带正确的 StepIndex。这是前端任务执行详情视图正确展示步骤进度和对应日志的前提。

#### 3.6.3 SSH 执行 display startup

复用 `executor.NewDeviceExecutor` 建连并执行命令，获取 `display startup` 的原始输出文本。

#### 3.6.4 SFTP 下载配置文件

```go
func (e *BackupExecutor) downloadConfigFile(
    ctx context.Context,
    device *models.DeviceAsset,
    remotePath, localPath string,
    connTimeout time.Duration,
) error {
    // 构建 sshutil.Config（复用全局设置中的算法和密钥策略）
    // 调用 sftputil.NewSFTPClient 创建 SFTP 客户端
    // 调用 sftpClient.DownloadFile 下载文件
    // 关闭 SFTP 客户端
}
```

### 3.7 TaskGroup 模型扩展

**文件**：`internal/models/models.go`

在 `TaskGroup` 结构体中新增字段：

```go
// 备份任务专用配置
BackupSaveRootPath    string `json:"backupSaveRootPath" gorm:"default:''"`    // 备份保存根路径
BackupDirNamePattern  string `json:"backupDirNamePattern" gorm:"default:'%Y%M%D'"`  // 目录命名模板
BackupFileNamePattern string `json:"backupFileNamePattern" gorm:"default:'%H.cfg'"` // 文件命名模板
BackupStartupCommand  string `json:"backupStartupCommand" gorm:"default:'display startup'"`  // 获取启动配置命令
```

**默认值**：
- `BackupSaveRootPath` = ""（空表示使用 PathManager.BackupConfigDir）
- `BackupDirNamePattern` = "%Y%M%D"
- `BackupFileNamePattern` = "%H.cfg"
- `BackupStartupCommand` = "display startup"

### 3.8 PathManager 扩展

**文件**：`internal/config/paths.go`

PathManager 统一收口所有路径模板解析、路径拼接及路径安全防御逻辑。执行器不自行实现路径解析。

#### 3.8.1 模板解析函数

```go
// resolvePathPattern 解析路径模板变量
func resolvePathPattern(pattern, deviceIP string, t time.Time) string {
    // 清洗 IP 值：IPv6 地址中的冒号在文件路径中非法，替换为短横线
    safeIP := strings.ReplaceAll(deviceIP, ":", "-")
    replacements := []struct{ placeholder, value string }{
        {"%H", safeIP},
        {"%Y", fmt.Sprintf("%04d", t.Year())},
        {"%M", fmt.Sprintf("%02d", t.Month())},
        {"%D", fmt.Sprintf("%02d", t.Day())},
        {"%h", fmt.Sprintf("%02d", t.Hour())},
        {"%m", fmt.Sprintf("%02d", t.Minute())},
        {"%s", fmt.Sprintf("%02d", t.Second())},
    }
    result := pattern
    for _, r := range replacements {
        result = strings.ReplaceAll(result, r.placeholder, r.value)
    }
    return filepath.Clean(result)
}
```

#### 3.8.2 完整路径生成函数

```go
// buildBackupLocalPath 构建备份文件的完整本地路径
func buildBackupLocalPath(saveRoot, dirPattern, filePattern, deviceIP string, timestamp time.Time) string {
    dirName := resolvePathPattern(dirPattern, deviceIP, timestamp)
    fileName := resolvePathPattern(filePattern, deviceIP, timestamp)
    result := filepath.Join(saveRoot, dirName, fileName)
    // 安全防护：确保结果路径在 saveRoot 之下，防止路径逃逸
    absResult, _ := filepath.Abs(result)
    absRoot, _ := filepath.Abs(saveRoot)
    if !strings.HasPrefix(absResult, absRoot) {
        return filepath.Join(saveRoot, "invalid", fileName)
    }
    return result
}
```

#### 3.8.3 对外方法

```go
// GetBackupConfigFilePath 获取备份配置文件的完整路径
func (pm *PathManager) GetBackupConfigFilePath(
    saveRoot, dirPattern, filePattern, deviceIP string, timestamp time.Time,
) string {
    // saveRoot 为空则使用默认 BackupConfigDir
    if saveRoot == "" {
        saveRoot = pm.BackupConfigDir()
    }
    return buildBackupLocalPath(saveRoot, dirPattern, filePattern, deviceIP, timestamp)
}
```

**文件覆盖策略**：当本地已存在同名文件时，采用**直接覆盖（Truncate）策略**，即新下载的配置文件直接覆盖已存在的同名文件。这符合备份场景"保留最新配置"的语义。SFTP 下载时使用 `os.Create`（而非 `os.OpenFile` 追加模式）即可实现此行为。

### 3.9 启动服务扩展

**文件**：`internal/taskexec/launch_service.go`

#### 3.9.1 CanonicalLaunchSpec 扩展

```go
type CanonicalLaunchSpec struct {
    // ... 现有字段 ...
    Backup *CanonicalBackup `json:"backup,omitempty"`
}

type CanonicalBackup struct {
    DeviceIDs       []uint   `json:"deviceIDs"`
    DeviceIPs       []string `json:"deviceIPs"`
    StartupCommand  string   `json:"startupCommand,omitempty"`
    SaveRootPath    string   `json:"saveRootPath,omitempty"`
    DirNamePattern  string   `json:"dirNamePattern,omitempty"`
    FileNamePattern string   `json:"fileNamePattern,omitempty"`
}
```

#### 3.9.2 归一化逻辑

在 `NormalizeTaskGroup` 中新增 `backup` 分支，从 `TaskGroup` 提取设备ID、备份配置等。

#### 3.9.3 校验逻辑

在 `ValidateLaunchSpec` 中新增 `backup` 分支，校验至少一台设备。

#### 3.9.4 任务定义创建

在 `CreateTaskDefinitionFromLaunchSpec` 中新增 `backup` 分支，构建 `BackupTaskConfig` 并序列化。

### 3.10 编译器与执行器注册

**文件**：`internal/taskexec/service.go`

在 `NewTaskExecutionService` 中注册：

```go
compilerReg.Register(string(RunKindBackup), NewBackupTaskCompiler(compileOpts))
```

**文件**：`internal/taskexec/service.go`（执行器注册与现有模式一致）

```go
// 注册 BackupExecutor（使用 RegisterExecutor，与现有执行器注册方式一致）
runtime.RegisterExecutor(NewBackupExecutor(repository.NewDeviceRepository()))
```

`BackupExecutor` 必须实现 `StageExecutor` 接口：

```go
func (e *BackupExecutor) Kind() string {
    return string(StageKindBackupCollect)
}

func (e *BackupExecutor) Run(ctx RuntimeContext, stage *StagePlan) error {
    // ... 并发执行逻辑
}
```

### 3.11 关键遗漏修改点

#### 3.11.1 `normalizeRunKind()` 修改

**文件**：`internal/taskexec/launch_service.go`

现有 `normalizeRunKind()` 只识别 `topology`，其他一律归为 `normal`。**必须新增 `backup` 识别**：

```go
func normalizeRunKind(taskType string) string {
    value := strings.ToLower(strings.TrimSpace(taskType))
    switch value {
    case string(RunKindTopology):
        return string(RunKindTopology)
    case string(RunKindBackup):
        return string(RunKindBackup)
    default:
        return string(RunKindNormal)
    }
}
```

#### 3.11.2 `extractEnableRawLog()` 修改

**文件**：`internal/taskexec/runtime.go`

当前 `extractEnableRawLog()` 只处理 `topology` 和 `default`(normal)，需新增 `backup` 分支：

```go
case string(RunKindBackup):
    var cfg BackupTaskConfig
    if err := json.Unmarshal(def.Config, &cfg); err == nil {
        return cfg.EnableRawLog
    }
```

#### 3.11.3 `specTargetIPs()` 扩展

**文件**：`internal/taskexec/launch_service.go`

当前只处理 `Normal` 和 `Topology`，需新增 `Backup` 分支：

```go
if spec.Backup != nil {
    result = append(result, spec.Backup.DeviceIPs...)
}
```

#### 3.11.4 `validateTaskGroup()` 扩展

**文件**：`internal/config/task_group.go`

新增 `backup` 类型的校验逻辑：
- 路径模板非空校验
- 启动配置命令非空校验

#### 3.11.5 备份任务阶段失败策略

备份任务只有一个 `backup_collect` 阶段，失败策略与普通任务一致：
- 部分设备失败 -> Run 状态为 `partial`
- 全部失败 -> Run 状态为 `failed`
- 不需要拓扑任务的关键阶段中止逻辑

### 3.12 前端设计

#### 3.12.1 任务执行页面 (TaskExecution.vue)

- 任务列表中，`taskType === "backup"` 的任务卡片显示特殊图标和"配置备份"标签
- 备份任务卡片上显示"开始备份"按钮（替代"开始执行"）
- 备份任务执行视图与普通任务共享同一套进度展示组件

#### 3.12.2 任务创建页面 (Tasks.vue)

- 在任务类型按钮组中新增"配置备份"选项（与"普通任务"/"拓扑采集任务"并列）
- 选择"配置备份"后，步骤2区域显示备份专用配置：
  - 任务名称（必填）
  - 任务描述
  - 设备选择器（默认不选任何设备）
  - 保存根路径（可选，默认使用系统路径）
  - 目录命名模板（默认 `%Y%M%D`）+ 问号按钮
  - 文件命名模板（默认 `%H.cfg`）+ 问号按钮
  - 启动配置命令（默认 `display startup`）
  - 并发数（默认5）
  - 超时时间（默认300秒）
- 备份任务不需要命令组选择
- `selectedTaskType` 类型扩展为 `"normal" | "topology" | "backup"`
- `canCreate` 计算属性新增 `backup` 分支（备份任务不需要命令组，只需设备）

#### 3.12.3 任务编辑弹窗 (TaskEditModal.vue)

- 新增 `isBackupTaskValue` 计算属性
- 当 `taskType === "backup"` 时，使用 `v-else-if="isBackupTaskValue"` 显示备份专用编辑区域：
  - 保存根路径输入框
  - 目录命名模板输入框 + 问号按钮
  - 文件命名模板输入框 + 问号按钮
  - 启动配置命令输入框
- `submit()` 方法新增 `backup` 类型的提交逻辑
- `hydrateForm()` 方法新增 `backup` 类型的表单初始化

#### 3.12.4 变量说明弹窗组件

抽取为独立组件 `PathTemplateHelpPopover.vue`，在创建和编辑页面中复用。

问号按钮点击后展示的变量说明内容：

| 变量 | 含义 | 示例 |
|------|------|------|
| %H | 设备主机IP（IPv6冒号自动替换为短横线） | 192.168.1.1 / fe80--1 |
| %Y | 四位年份 | 2026 |
| **%M** | **两位月份（注意大写M=月）** | 04 |
| %D | 两位日期 | 22 |
| %h | 两位小时(24h) | 14 |
| **%m** | **两位分钟（注意小写m=分）** | 30 |
| %s | 两位秒 | 05 |

> **注意**：
> - `%M`(大写)表示月份，`%m`(小写)表示分钟，请注意大小写区分。
> - `%H` 使用设备IP地址。IPv6 地址中的冒号 `:` 在文件路径中为非法字符（尤其在 Windows 上），系统会自动将冒号替换为短横线 `-`（如 `fe80::1` → `fe80--1`）。

#### 3.12.5 前端类型定义扩展

`TaskGroup` TypeScript 类型需新增备份相关字段：

```typescript
interface TaskGroup {
  // ... 现有字段 ...
  backupSaveRootPath?: string
  backupDirNamePattern?: string
  backupFileNamePattern?: string
  backupStartupCommand?: string
}
```

---

## 4. 数据流

### 4.1 备份执行数据流

```
TaskGroup (taskType="backup")
  -> CanonicalLaunchSpec (Backup字段)
    -> TaskDefinition (Kind="backup", Config=BackupTaskConfig JSON)
      -> ExecutionPlan (RunKind="backup", Stage[Kind="backup_collect"])
        -> BackupExecutor.Run()
          -> 每设备:
            SSH -> display startup -> 正则提取路径
            SFTP -> download -> 本地文件
```

### 4.2 产物记录

备份完成后，为每个设备创建 `TaskArtifact` 记录：

```go
TaskArtifact{
    TaskRunID: runID,
    StageID:  stageID,
    UnitID:   unitID,
    Type:     "backup_config",  // 新增产物类型
    Key:      fmt.Sprintf("%s:config_backup", deviceIP),
    Path:     localFilePath,    // 本地保存路径
}
```

### 4.3 事件流

备份执行过程中发出的事件（必须携带正确的 StepIndex，与编译器生成的 Step 对齐）：

| 事件 | StepIndex | 级别 | 消息 |
|------|-----------|------|------|
| UnitStarted | - | info | "Connecting to {IP}..." |
| StepStarted | 0 | info | "Querying startup config..." |
| StepFinished | 0 | info | "Startup config path: {path}" |
| StepFinished | 0 | error | "Failed to query startup config: {error}" |
| StepStarted | 1 | info | "Downloading config file..." |
| StepFinished | 1 | info | "Config file saved: {localPath}" |
| StepFinished | 1 | error | "Failed to download config file: {error}" |
| UnitFinished | - | info | "Backup completed for {IP}" |
| UnitFinished | - | error | "Backup failed for {IP}: {error}" |

---

## 5. 错误处理

### 5.1 错误场景

| 场景 | 处理方式 |
|------|----------|
| SSH连接失败 | 标记Unit失败，记录错误信息，继续下一设备 |
| display startup 执行失败 | 标记Unit失败，记录错误信息 |
| 正则未匹配到配置文件路径 | 标记Unit失败，提示"未找到下次启动配置文件" |
| 配置文件路径为NULL | 标记Unit失败，提示"设备无下次启动配置文件" |
| SFTP连接失败 | 标记Unit失败，记录错误信息 |
| SFTP下载失败 | 标记Unit失败，记录错误信息（文件不存在/权限不足等） |
| 本地目录创建失败 | 标记Unit失败，记录错误信息 |
| 上下文取消 | 标记Unit取消，优雅关闭连接 |

### 5.2 部分成功处理

与现有任务执行一致：部分设备成功、部分失败时，Run 状态为 `partial`。

---

## 6. 安全考虑

- SSH/SFTP 连接复用全局设置中的算法配置和主机密钥校验策略
- 设备密码仅在运行时使用，不记录到日志或产物中
- 备份文件保存到本地，不经过网络传输（除SFTP下载过程）
- 路径模板注入防护：`resolvePathPattern` 结果经过 `filepath.Clean` 清洗，防止路径遍历
- IPv6 地址防护：`resolvePathPattern` 在替换 `%H` 时将 IP 中的冒号 `:` 替换为短横线 `-`，防止 IPv6 地址在 Windows 文件路径中非法
- 文件覆盖策略：采用直接覆盖（Truncate），SFTP 下载使用 `os.Create` 打开目标文件，同名文件将被截断覆盖

---

## 7. 测试要点

| 测试项 | 方法 |
|--------|------|
| 正则提取 - 正常输出 | 单元测试，验证各种格式的 display startup 输出 |
| 正则提取 - NULL | 单元测试，验证 NULL 返回空 |
| 正则提取 - 无匹配 | 单元测试，验证无匹配返回空 |
| 路径模板解析 | 单元测试，验证各变量替换正确 |
| 路径模板 - 空模板 | 单元测试，验证空模板不崩溃 |
| 路径模板 - 路径遍历防护 | 单元测试，验证 `../` 被清洗 |
| 路径模板 - IPv6 冒号清洗 | 单元测试，验证 IPv6 地址中冒号替换为短横线 |
| 路径生成 - 路径逃逸防护 | 单元测试，验证最终路径在 saveRoot 之下 |
| 编译器 - 正常编译 | 单元测试，验证生成正确的 ExecutionPlan |
| 编译器 - 无设备 | 单元测试，验证返回错误 |
| 执行器 - Step 事件上报 | 单元测试，验证 step-0/step-1 的 EventStepStarted/EventStepFinished 按正确顺序和 StepIndex 发射 |
| 执行器 - SSH失败 | 集成测试（mock） |
| 执行器 - SFTP失败 | 集成测试（mock） |
| 执行器 - 正常备份 | 集成测试（mock） |
| 执行器 - 文件覆盖 | 集成测试（mock），验证同名文件被 Truncate 覆盖 |
| 前端 - 创建备份任务 | 手动测试 |
| 前端 - 执行备份任务 | 手动测试 |
| 前端 - 编辑备份配置 | 手动测试 |
