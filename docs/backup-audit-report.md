# 批量备份功能审计报告

> 审计日期: 2026-04-23
> 审计范围: NetWeaverGo 批量配置备份功能全链路
> 审计版本: 基于 main 分支当前代码

---

## 一、审计概述

本报告对 NetWeaverGo 项目的批量配置备份功能进行了全面代码审计，覆盖从用户操作入口到最终文件落盘的完整调用链，重点审查**正确性、安全性、健壮性、并发安全、资源管理**五个维度。

### 审计范围

| 层级 | 文件 | 职责 |
|------|------|------|
| 编译层 | `internal/taskexec/backup_compiler.go` | 备份任务配置编译为执行计划 |
| 执行层 | `internal/taskexec/backup_executor.go` | 并发调度 + 单设备备份执行 |
| 启动层 | `internal/taskexec/launch_service.go` | 备份参数归一化 + 校验 + 任务定义创建 |
| 运行时 | `internal/taskexec/runtime.go` | 执行计划调度 + 状态管理 + 快照 |
| 路径层 | `internal/config/paths.go` | 备份文件路径生成 + 路径逃逸防护 |
| 配置层 | `internal/config/task_group.go` | 备份默认值归一化 + 校验 |
| SFTP层 | `internal/sftputil/client.go` | SFTP连接 + 文件下载 |
| SSH层 | `internal/sshutil/client.go` | SSH连接建立 + 命令执行 |
| 数据层 | `internal/models/models.go` | 数据模型定义 |
| 仓库层 | `internal/repository/device_repository.go` | 设备查询 |

---

## 二、发现汇总

| 严重级别 | 数量 | 说明 |
|---------|------|------|
| **严重 (Critical)** | 2 | 可能导致数据丢失或安全漏洞 |
| **高 (High)** | 4 | 可能导致功能异常或资源泄漏 |
| **中 (Medium)** | 5 | 可能影响可靠性或可维护性 |
| **低 (Low)** | 3 | 代码质量或最佳实践问题 |

---

## 三、严重问题 (Critical)

### C-01: SFTP下载失败时产生不完整文件，无原子写入保障

**位置**: `internal/sftputil/client.go:53-79` (`DownloadFile` 方法)

**问题描述**:
`DownloadFile` 方法先 `os.Create(localPath)` 创建本地文件，再 `io.Copy` 写入内容。如果下载过程中发生错误（网络中断、SFTP连接断开、磁盘满等），已创建的文件将包含不完整内容，但不会被清理。后续逻辑可能误认为该文件是完整的备份配置。

```go
localFile, err := os.Create(localPath)  // 文件已创建
defer localFile.Close()

bytesCopied, err := io.Copy(localFile, remoteFile)  // 如果这里失败...
if err != nil {
    return fmt.Errorf("下载文件失败 (已复制 %d 字节): %w", bytesCopied, err)
    // 不完整文件留在磁盘上！
}
```

**影响**:
- 网络抖动导致下载中断时，磁盘上留下截断的配置文件
- 下次备份可能因文件已存在而覆盖，但用户无法区分完整和截断的备份
- 恢复时使用截断配置可能导致设备配置异常

**建议修复**:
采用"写临时文件 + 原子重命名"模式：
1. 先写入 `.tmp` 后缀的临时文件
2. 下载成功后 `os.Rename` 原子替换为目标文件
3. 下载失败时删除临时文件
4. 可选：下载完成后校验文件大小非零

---

### C-02: 路径逃逸防护存在绕过风险

**位置**: `internal/config/paths.go:281-296` (`buildBackupLocalPath` 函数)

**问题描述**:
路径逃逸检测逻辑使用 `strings.HasPrefix` 判断最终路径是否在 `saveRoot` 下：

```go
prefix := cleanRoot + string(filepath.Separator)
if !strings.HasPrefix(cleanFullPath, prefix) && cleanFullPath != cleanRoot {
    return filepath.Join(cleanRoot, "escape_prevented", fileName)
}
```

存在以下绕过场景：

1. **Windows 平台大小写问题**: Windows 文件系统不区分大小写，但 `strings.HasPrefix` 区分大小写。如果 `saveRoot` 为 `C:\Data`，而解析后路径为 `c:\data\backup\...`（大小写不同），`HasPrefix` 判断失败，触发误报的逃逸防护，文件被重定向到 `escape_prevented` 目录。

2. **符号链接绕过**: 如果 `saveRoot` 下存在指向外部的符号链接，`filepath.Clean` 不会解析符号链接，路径检查通过但实际文件写入到了 `saveRoot` 之外。

3. **`saveRoot` 本身为空时的行为**: 当 `saveRoot` 为空字符串时，`filepath.Clean("")` 返回 `"."`，`prefix` 变为 `".\" + separator`，此时所有相对路径都可能通过检查，导致文件写入到不可预期的位置。

**影响**:
- Windows 平台可能因大小写导致备份文件被错误重定向
- 符号链接场景下可能写入到预期目录之外
- 空 `saveRoot` 时路径不可控

**建议修复**:
1. 使用 `filepath.EvalSymlinks` 解析真实路径后再做前缀比较
2. Windows 平台使用 `strings.EqualFold` 做大小写不敏感比较
3. 对 `saveRoot` 为空的情况做显式校验，拒绝空值或强制回退到默认路径

---

## 四、高优先级问题 (High)

### H-01: 正则表达式每次调用都重新编译，性能浪费

**位置**: `internal/taskexec/backup_executor.go:256-279` (`extractNextStartupConfigPath` 方法)

**问题描述**:
`extractNextStartupConfigPath` 在每次调用时都通过 `regexp.MustCompile` 编译正则表达式。在批量备份场景下，如果有 100 台设备，正则会被编译 100 次。

```go
func (e *BackupExecutor) extractNextStartupConfigPath(output string) string {
    re := regexp.MustCompile(`(?i)Next startup saved-configuration file:?\s+...`)  // 每次调用都编译
    // ...
    reCisco := regexp.MustCompile(`(?i)boot system\s+...`)  // 每次调用都编译
}
```

**影响**:
- 正则编译是 CPU 密集操作，批量备份时造成不必要的性能开销
- 虽然不会导致功能错误，但在大规模备份（数百台设备）时可能影响执行效率

**建议修复**:
将正则提升为包级变量，使用 `var` 在 init 时编译一次：
```go
var (
    reHuawei  = regexp.MustCompile(`(?i)Next startup saved-configuration file:?\s+(?:flash:|cf:|sdcard:)?/?([^\r\n\s]+)`)
    reCisco   = regexp.MustCompile(`(?i)boot system\s+(?:flash:|bootflash:|slot0:|usb0:)?/?([^\r\n\s]+)`)
)
```

---

### H-02: 并发调度中信号量获取是阻塞的，无法响应取消

**位置**: `internal/taskexec/backup_executor.go:56-62` (`Run` 方法)

**问题描述**:
在 `Run` 方法的 for 循环中，`semaphore <- struct{}{}` 是阻塞操作。当并发数已满时，主 goroutine 会阻塞在信号量获取上，此时即使 `ctx` 已取消，也无法立即退出循环。

```go
for _, unit := range stage.Units {
    if ctx.IsCancelled() {
        break
    }
    wg.Add(1)
    semaphore <- struct{}{}  // 阻塞！如果并发已满且ctx已取消，这里会卡住
    go func(u UnitPlan) { ... }(unit)
}
```

**影响**:
- 取消操作可能延迟响应，最多等待一个设备备份完成（可能长达 300 秒超时）
- 用户点击取消后，UI 可能长时间无响应

**建议修复**:
使用 `select` 同时监听信号量和取消通道：
```go
select {
case semaphore <- struct{}{}:
    // 获取到信号量，继续
case <-ctx.Context().Done():
    // 取消，退出循环
    break
}
```

---

### H-03: 备份校验不充分 — 缺少 StartupCommand 和路径模式校验

**位置**: `internal/taskexec/launch_service.go:329-332` (`ValidateLaunchSpec` 方法)

**问题描述**:
备份任务的校验仅检查 `spec.Backup != nil && len(spec.Backup.DeviceIPs) > 0`，缺少以下关键校验：

1. **StartupCommand 为空**: 如果用户未设置启动配置查询命令，编译后的 StepPlan 中 `startupCommand` 为空字符串，SSH 执行空命令可能返回意外输出或设备提示符卡住
2. **DirNamePattern / FileNamePattern 为空**: 路径模式为空时，`resolvePathPattern` 返回空字符串，最终文件路径可能不合法
3. **SaveRootPath 包含非法字符**: 未校验路径中是否包含 `..`、绝对路径跨盘符等

**影响**:
- 空命令可能导致 SSH 会话挂起或超时
- 空路径模式导致文件保存到不可预期位置
- 非法路径可能导致 `os.MkdirAll` 失败

**建议修复**:
在 `ValidateLaunchSpec` 中增加备份专用校验：
```go
case string(RunKindBackup):
    if spec.Backup == nil || len(spec.Backup.DeviceIPs) == 0 {
        return fmt.Errorf("备份任务至少需要一台设备")
    }
    if strings.TrimSpace(spec.Backup.StartupCommand) == "" {
        return fmt.Errorf("备份任务缺少启动配置查询命令")
    }
    if strings.TrimSpace(spec.Backup.FileNamePattern) == "" {
        return fmt.Errorf("备份任务缺少文件名模式")
    }
```

---

### H-04: 设备查找失败时静默跳过，可能导致备份任务"空跑"

**位置**: `internal/taskexec/launch_service.go:298-314` (`lookupDeviceIPs` 方法)

**问题描述**:
`lookupDeviceIPs` 在设备查找失败或设备缺少 IP 时仅打印 Warn 日志并跳过，不返回错误。如果所有设备的 ID 都无效（例如设备已被删除），`normalizeBackup` 返回的 `DeviceIPs` 为空列表，但不会报错。

后续 `ValidateLaunchSpec` 会检查 `len(spec.Backup.DeviceIPs) == 0` 并返回错误，但错误信息是"备份任务至少需要一台设备"，无法让用户理解真正原因是设备ID无效。

**影响**:
- 用户看到"至少需要一台设备"的错误，但实际原因是设备已被删除
- 调试困难，需要查看日志才能定位问题

**建议修复**:
1. 在 `lookupDeviceIPs` 中收集查找失败的设备ID，返回更详细的错误信息
2. 或在 `normalizeBackup` 中检查：如果 `deviceIDs` 非空但 `deviceIPs` 为空，返回明确错误

---

## 五、中优先级问题 (Medium)

### M-01: 备份文件可能被静默覆盖，无版本保护

**位置**: `internal/sftputil/client.go:66` + `internal/config/paths.go:265-278`

**问题描述**:
默认文件名模式 `%H_startup.cfg` 不包含时间戳（仅目录名包含日期 `%Y-%M-%D`）。同一天内对同一设备执行多次备份时，后一次会静默覆盖前一次的文件。

**影响**:
- 用户可能无意中丢失历史备份
- 无法对比同一天内不同时间点的配置变化

**建议修复**:
1. 在默认文件名模式中加入时间：`%H_startup_%h%m%s.cfg`
2. 或在下载前检查文件是否已存在，存在时自动添加序号后缀
3. 或在文档中明确说明覆盖行为，让用户自行决定是否在模式中加入时间

---

### M-02: 配置路径提取正则不支持部分厂商格式

**位置**: `internal/taskexec/backup_executor.go:256-279`

**问题描述**:
当前正则仅支持两种格式：
- 华为/华三: `Next startup saved-configuration file: flash:/1.cfg`
- Cisco: `boot system flash:1.cfg`

缺少以下常见格式支持：
- **锐捷 (Ruijie)**: `Next startup configuration file: flash:config.text`（注意关键字是 `configuration` 而非 `saved-configuration`）
- **华三 Comware V7**: `Main startup configuration file: flash:/startup.cfg`（关键字为 `Main startup`）
- **Juniper**: `set system config file /config/juniper.conf.gz`（完全不同格式）
- **Arista**: `boot system flash:/zeos.cfg`（与 Cisco 类似但可能有变体）

**影响**:
- 不支持的厂商设备备份必然失败
- 错误信息"未能从输出中提取配置文件路径"不够明确，用户难以定位

**建议修复**:
1. 增加锐捷和华三 V7 格式的正则匹配
2. 考虑将正则配置化，允许用户自定义匹配规则
3. 提取失败时在错误信息中包含设备厂商信息，便于排查

---

### M-03: SSH连接和SFTP连接使用相同超时，但SFTP下载大文件可能需要更长时间

**位置**: `internal/taskexec/backup_executor.go:208` + `internal/taskexec/backup_compiler.go:72`

**问题描述**:
`UnitPlan.Timeout` 被同时用于 SSH 命令执行超时和 SFTP 连接超时。但 SFTP 下载大配置文件（如包含大量 ACL 的配置）可能需要远超 SSH 命令执行的时间。

```go
sftpSSHConfig := sshutil.Config{
    // ...
    Timeout: unit.Timeout,  // 与SSH命令共享同一超时
}
```

**影响**:
- 大配置文件下载可能因超时被截断
- 默认 300 秒对大多数场景足够，但极端情况下可能不够

**建议修复**:
1. 为 SFTP 下载设置独立的超时参数（如 `SFTPTimeoutSec`）
2. 或在 SFTP 下载时使用更宽松的超时（如命令超时的 2 倍）

---

### M-04: `createArtifact` 使用 BestEffort 策略，产物记录丢失无告警

**位置**: `internal/taskexec/backup_executor.go:241-254`

**问题描述**:
`createArtifact` 在数据库写入失败时仅打印 Warn 日志，不返回错误。这意味着备份文件可能已成功下载到磁盘，但产物索引记录丢失。

```go
if err := e.db.Create(&artifact).Error; err != nil {
    logger.Warn("TaskExec", taskRunID, "保存产物记录失败: err=%v, artifact=%+v", err, artifact)
}
```

**影响**:
- 用户在 UI 上看不到备份产物记录，以为备份失败
- 实际文件已在磁盘上，造成"幽灵备份"
- 后续清理操作可能遗漏这些无索引的备份文件

**建议修复**:
1. 将 `createArtifact` 改为返回错误，由调用方决定是否标记 Unit 失败
2. 或在产物记录失败时发射一个 Warn 级别事件到前端，提示用户检查

---

### M-05: 并发计数器更新与事件发射之间存在竞态窗口

**位置**: `internal/taskexec/backup_executor.go:79-104`

**问题描述**:
在 `Run` 方法的 goroutine 中，计数器更新和事件发射的顺序如下：

```go
mu.Lock()
// 更新计数器
completedCount++  // 或 failedCount++ / cancelledCount++
localCompleted := completedCount
localFailed := failedCount
localCancelled := cancelledCount
mu.Unlock()

// 发射事件（在锁外）
emitProjectedUnitEvent(...)
applyProjectedStageProgress(..., localCompleted, localFailed, localCancelled, ...)
```

虽然计数器读取在锁内，但事件发射在锁外。两个 goroutine 可能按以下交错顺序执行：

1. goroutine-A: 更新计数器 (completed=1), 解锁
2. goroutine-B: 更新计数器 (completed=2), 解锁, 发射事件 (completed=2)
3. goroutine-A: 发射事件 (completed=1) ← 事件中携带的进度是过时的

**影响**:
- 前端可能收到进度"回退"的事件（先收到 completed=2，再收到 completed=1）
- 不影响最终一致性，但可能导致 UI 闪烁

**建议修复**:
将事件发射移入锁内，或在事件中不携带绝对计数而仅携带增量。

---

## 六、低优先级问题 (Low)

### L-01: `normalizeDeviceIPs` 仅过滤空字符串，不校验 IP 格式

**位置**: `internal/taskexec/backup_compiler.go:98-106`

**问题描述**:
`normalizeDeviceIPs` 仅过滤空字符串，不校验 IP 地址格式。非法 IP（如 `999.999.999.999`、`not-an-ip`）会进入执行计划，最终在 SSH 连接时失败。

**影响**:
- 非法 IP 导致连接失败，错误信息不明确
- 浪费并发槽位和超时等待时间

**建议修复**:
在编译阶段校验 IP 格式，非法 IP 直接拒绝编译。

---

### L-02: 备份执行器默认并发数不一致

**位置**: `internal/taskexec/backup_executor.go:46` vs `internal/taskexec/config_models.go:76`

**问题描述**:
- `BackupExecutor.Run()` 中并发数 <= 0 时默认为 **5**
- `DefaultCompileOptions.DefaultConcurrency` 默认为 **10**

两处默认值不一致。虽然编译器会设置并发数为 10，执行器仅在并发数为 0 或负数时回退到 5，但语义上存在混淆。

**建议修复**:
统一默认并发数，或让执行器从 `CompileOptions` 获取默认值。

---

### L-03: `fmt.Errorf(errMsg)` 使用变量作为格式字符串

**位置**: `internal/taskexec/backup_executor.go:183`

**问题描述**:
```go
errMsg := fmt.Sprintf("未能从输出中提取配置文件路径:\n%s", cmdOutput)
// ...
return fmt.Errorf(errMsg)  // errMsg 作为格式字符串，如果包含 % 会产生意外行为
```

`fmt.Errorf` 的第一个参数是格式字符串。如果 `cmdOutput` 中包含 `%` 字符（如配置内容中有百分号），`fmt.Errorf(errMsg)` 会尝试解析格式化占位符，可能产生 `%!(EXTRA ...)` 错误或丢失内容。

**建议修复**:
```go
return fmt.Errorf("%s", errMsg)
// 或直接
return fmt.Errorf("未能从输出中提取配置文件路径:\n%s", cmdOutput)
```

---

## 七、架构与设计观察

### 正面设计

| 设计点 | 评价 |
|--------|------|
| **编译-执行分离** | BackupCompiler 负责编译，BackupExecutor 负责执行，职责清晰，易于测试 |
| **独立SFTP连接** | 为 SFTP 建立全新 SSH 连接，避免华为/华三设备拒绝在已有 shell/pty 通道上开 sftp 子系统，这是正确的做法 |
| **路径逃逸防护** | `buildBackupLocalPath` 检测路径逃逸，虽然存在绕过风险（C-02），但设计意图正确 |
| **并发信号量模式** | 使用 channel 作为信号量控制并发数，是 Go 惯用模式 |
| **取消传播** | 通过 `context.Context` 和 `IsCancelled()` 双重检查传播取消信号 |
| **产物追踪** | 下载完成后创建 TaskArtifact 记录，便于后续查询和审计 |
| **错误分类策略** | executor 包的错误分类和致命性判断策略矩阵设计合理 |

### 架构风险

| 风险点 | 评价 |
|--------|------|
| **单阶段设计** | 备份仅有一个 `backup_collect` 阶段，无法支持"先备份再校验"等多阶段工作流 |
| **无重试机制** | 单设备备份失败后无重试，网络抖动导致的临时失败需要用户手动重新执行整个任务 |
| **无增量备份** | 每次都是全量下载，无法跳过未变更的配置 |
| **无备份保留策略** | 无自动清理旧备份的机制，长期运行可能占满磁盘 |
| **无下载完整性校验** | 下载后不校验文件哈希，无法确认传输完整性 |

---

## 八、完整调用链与问题定位

```
[前端] Tasks.vue / TaskEditModal.vue
  │
  ▼
[UI服务] TaskGroupService.StartTaskGroup(id)
  │
  ▼
[启动服务] TaskLaunchService.StartTaskGroup()
  │  ├─ config.GetTaskGroup(id)                    ← 加载任务组
  │  ├─ normalizer.NormalizeTaskGroup()            ← 归一化
  │  │   └─ normalizeBackup()                      ← [H-04] 设备查找失败静默跳过
  │  │       └─ lookupDeviceIPs()                  ← [H-04] Warn日志，不返回错误
  │  ├─ validator.ValidateLaunchSpec()             ← [H-03] 缺少命令/路径校验
  │  └─ CreateTaskDefinitionFromLaunchSpec()       ← 创建任务定义
  │
  ▼
[编译器] BackupTaskCompiler.Compile()
  │  ├─ json.Unmarshal(def.Config → BackupTaskConfig)
  │  ├─ normalizeDeviceIPs()                       ← [L-01] 不校验IP格式
  │  └─ 为每台设备生成 UnitPlan(2步)
  │
  ▼
[运行时] RuntimeManager.Execute() → go executePlan()
  │
  ▼
[备份执行器] BackupExecutor.Run()
  │  ├─ semaphore := make(chan struct{}, concurrency) ← [L-02] 默认5 vs 编译器默认10
  │  ├─ for unit := range stage.Units
  │  │   ├─ semaphore <- struct{}{}                ← [H-02] 阻塞，无法响应取消
  │  │   └─ go executeBackupUnit()
  │
  ▼
[备份执行器] BackupExecutor.executeBackupUnit()
  │  ├─ repo.FindByIP(deviceIP)                    ← 查找设备凭据
  │  ├─ exec.Connect()                             ← SSH连接
  │  ├─ exec.ExecuteCommandSync(startupCommand)    ← 执行查询命令
  │  ├─ extractNextStartupConfigPath()             ← [H-01] 每次编译正则
  │  │                                             ← [M-02] 不支持部分厂商
  │  ├─ pathManager.GetBackupConfigFilePath()      ← [C-02] 路径逃逸防护
  │  ├─ sftputil.NewSFTPClient()                   ← [M-03] 共享超时
  │  ├─ sftpClient.DownloadFile()                  ← [C-01] 非原子写入
  │  │                                             ← [M-01] 静默覆盖
  │  ├─ createArtifact()                           ← [M-04] BestEffort，可能丢失
  │  └─ completeUnitExecution()
```

---

## 九、修复优先级建议

| 优先级 | 问题编号 | 修复建议 |
|--------|---------|---------|
| P0 | C-01 | 实现原子写入（临时文件 + 重命名），失败时清理不完整文件 |
| P0 | C-02 | 修复路径逃逸检测：EvalSymlinks + 大小写不敏感比较 + 空值校验 |
| P1 | H-02 | 信号量获取使用 select 监听取消通道 |
| P1 | H-03 | 增加备份参数校验（StartupCommand、路径模式） |
| P1 | H-04 | 设备查找失败时返回明确错误信息 |
| P2 | H-01 | 正则提升为包级变量 |
| P2 | M-01 | 默认文件名加入时间戳或检测文件已存在 |
| P2 | M-02 | 增加锐捷/华三V7格式支持 |
| P2 | M-04 | 产物记录失败时发射告警事件 |
| P3 | M-03 | SFTP下载使用独立超时 |
| P3 | M-05 | 事件发射移入锁内或使用增量模式 |
| P3 | L-01 | 编译阶段校验IP格式 |
| P3 | L-02 | 统一默认并发数 |
| P3 | L-03 | 修复 fmt.Errorf 格式字符串问题 |

---

## 十、测试建议

基于审计发现，建议补充以下测试用例：

| 测试场景 | 覆盖问题 | 测试类型 |
|---------|---------|---------|
| SFTP下载中断后验证无残留不完整文件 | C-01 | 集成测试 |
| 路径模式包含 `..` 时验证逃逸防护 | C-02 | 单元测试 |
| Windows路径大小写不一致时验证行为 | C-02 | 单元测试 |
| 取消操作时验证信号量不阻塞 | H-02 | 并发测试 |
| StartupCommand为空时验证编译拒绝 | H-03 | 单元测试 |
| 设备ID无效时验证明确错误信息 | H-04 | 单元测试 |
| 同一设备同一天多次备份验证文件处理 | M-01 | 集成测试 |
| 锐捷设备输出验证路径提取 | M-02 | 单元测试 |
| 配置输出包含 `%` 字符时验证错误信息 | L-03 | 单元测试 |
| 大规模并发（100+设备）验证性能 | H-01 | 压力测试 |

---

*审计完成。本报告基于静态代码分析，建议结合动态测试验证上述发现。*
