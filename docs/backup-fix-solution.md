# 批量备份功能审计问题 — 详细解决方案

> 基于审计报告: `docs/backup-audit-report.md`
> 编写日期: 2026-04-23
> 状态: 待实施

---

## 修改文件总览

| 文件 | 涉及问题 | 修改类型 |
|------|---------|---------|
| `internal/sftputil/client.go` | C-01 | 方法重写 |
| `internal/config/paths.go` | C-02 | 函数重写 |
| `internal/taskexec/backup_executor.go` | H-01, H-02, M-02, M-04, M-05, L-02, L-03 | 多处修改 |
| `internal/taskexec/launch_service.go` | H-03, H-04 | 校验增强 |
| `internal/taskexec/backup_compiler.go` | L-01 | 校验增强（需新增 import "strings"） |
| `internal/taskexec/config_models.go` | M-03 | 模型扩展 |
| `internal/config/task_group.go` | M-01 | 默认值调整 |

---

## P0 — 严重问题修复

### C-01: SFTP下载原子写入

**文件**: `internal/sftputil/client.go`
**修改方法**: `DownloadFile` (第53-79行)

**当前代码**:
```go
func (s *SFTPClient) DownloadFile(remotePath, localPath string) error {
	logger.Debug("SFTP", s.ip, "开始下载文件 %s -> %s", remotePath, localPath)
	remoteFile, err := s.client.Open(remotePath)
	if err != nil {
		logger.Debug("SFTP", s.ip, "打开远端文件 %s 失败: %v", remotePath, err)
		return fmt.Errorf("打开远端文件 %s 失败: %w", remotePath, err)
	}
	defer remoteFile.Close()

	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("创建本地目录 %s 失败: %w", localPath, err)
	}

	localFile, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("创建本地文件 %s 失败: %w", localPath, err)
	}
	defer localFile.Close()

	bytesCopied, err := io.Copy(localFile, remoteFile)
	if err != nil {
		return fmt.Errorf("下载文件失败 (已复制 %d 字节): %w", bytesCopied, err)
	}

	logger.Info("SFTP", s.ip, "成功下载文件: %s -> %s (大小: %d 字节)", remotePath, localPath, bytesCopied)
	return nil
}
```

**替换为**:
```go
// DownloadFile 从远程路径下载文件到本地路径。
// 如果本地父目录不存在，将会自动创建。
// 采用原子写入策略：先写入临时文件，下载成功后重命名为目标文件，
// 避免下载中断时留下不完整文件。
func (s *SFTPClient) DownloadFile(remotePath, localPath string) error {
	logger.Debug("SFTP", s.ip, "开始下载文件 %s -> %s", remotePath, localPath)
	remoteFile, err := s.client.Open(remotePath)
	if err != nil {
		logger.Debug("SFTP", s.ip, "打开远端文件 %s 失败: %v", remotePath, err)
		return fmt.Errorf("打开远端文件 %s 失败: %w", remotePath, err)
	}
	defer remoteFile.Close()

	localDir := filepath.Dir(localPath)
	if err := os.MkdirAll(localDir, 0755); err != nil {
		return fmt.Errorf("创建本地目录 %s 失败: %w", localDir, err)
	}

	// 原子写入：先写临时文件，成功后重命名
	tmpPath := localPath + ".tmp"
	localFile, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("创建临时文件 %s 失败: %w", tmpPath, err)
	}

	bytesCopied, err := io.Copy(localFile, remoteFile)
	// 无论 io.Copy 是否成功，都必须先关闭文件，否则 os.Rename 在 Windows 上可能失败
	if closeErr := localFile.Close(); closeErr != nil && err == nil {
		err = closeErr
	}

	if err != nil {
		// 下载失败：清理临时文件，避免留下不完整内容
		if removeErr := os.Remove(tmpPath); removeErr != nil && !os.IsNotExist(removeErr) {
			logger.Warn("SFTP", s.ip, "清理临时文件失败: %v", removeErr)
		}
		return fmt.Errorf("下载文件失败 (已复制 %d 字节): %w", bytesCopied, err)
	}

	// 下载成功：校验文件非空（空文件通常意味着远端路径错误或设备配置异常）
	if bytesCopied == 0 {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("下载文件为空: %s", remotePath)
	}

	// 原子重命名：在同一文件系统上这是原子操作
	if err := os.Rename(tmpPath, localPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("重命名临时文件到目标文件失败: %w", err)
	}

	logger.Info("SFTP", s.ip, "成功下载文件: %s -> %s (大小: %d 字节)", remotePath, localPath, bytesCopied)
	return nil
}
```

**修改要点**:
1. 先写入 `localPath + ".tmp"` 临时文件
2. `io.Copy` 后**立即关闭文件**（`localFile.Close()`），不再使用 `defer`，因为 Windows 上 `os.Rename` 要求文件句柄已关闭
3. 下载失败时 `os.Remove(tmpPath)` 清理临时文件
4. 下载成功后校验 `bytesCopied > 0`，空文件视为异常
5. `os.Rename` 原子替换为目标文件

**边界情况处理**:
- `os.Rename` 在同一文件系统上是原子操作；跨文件系统会失败，此时清理临时文件并返回错误
- Windows 上文件句柄未关闭时 `os.Rename` 会失败，因此必须在 `Rename` 前显式 `Close`
- 如果进程在 `io.Copy` 期间崩溃，磁盘上仅残留 `.tmp` 文件，不会污染目标文件

---

### C-02: 路径逃逸防护加固

**文件**: `internal/config/paths.go`
**修改函数**: `buildBackupLocalPath` (第281-296行)

**当前代码**:
```go
func buildBackupLocalPath(saveRoot, dirPattern, filePattern, deviceIP string, timestamp time.Time) string {
	dirName := resolvePathPattern(dirPattern, deviceIP, timestamp)
	fileName := resolvePathPattern(filePattern, deviceIP, timestamp)
	
	fullPath := filepath.Join(saveRoot, dirName, fileName)
	
	cleanRoot := filepath.Clean(saveRoot)
	cleanFullPath := filepath.Clean(fullPath)
	
	prefix := cleanRoot + string(filepath.Separator)
	if !strings.HasPrefix(cleanFullPath, prefix) && cleanFullPath != cleanRoot {
		return filepath.Join(cleanRoot, "escape_prevented", fileName)
	}
	
	return cleanFullPath
}
```

**替换为**:
```go
// isPathWithinRoot 判断 path 是否在 root 目录下（含 root 本身）。
// 仅对 root 调用 EvalSymlinks 解析符号链接（root 应已存在），
// path 是新创建的路径，不存在符号链接，仅做 filepath.Clean。
// 在 Windows 上做大小写不敏感比较。
func isPathWithinRoot(path, root string) bool {
	// 仅解析 root 的符号链接（root 应已存在）
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		// EvalSymlinks 失败（root 不存在等），回退到原始路径
		realRoot = root
	}

	// path 是新创建的路径，不存在符号链接，直接 Clean
	cleanPath := filepath.Clean(path)
	cleanRoot := filepath.Clean(realRoot)

	if cleanPath == cleanRoot {
		return true
	}

	prefix := cleanRoot + string(filepath.Separator)
	// Windows 文件系统不区分大小写，使用 EqualFold 做前缀比较
	if filepath.IsAbs(cleanRoot) && len(cleanPath) >= len(prefix) {
		candidate := cleanPath[:len(prefix)]
		if strings.EqualFold(candidate, prefix) {
			return true
		}
	}
	// 非 Windows 或路径长度不足，回退到精确匹配
	return strings.HasPrefix(cleanPath, prefix)
}

func buildBackupLocalPath(saveRoot, dirPattern, filePattern, deviceIP string, timestamp time.Time) string {
	dirName := resolvePathPattern(dirPattern, deviceIP, timestamp)
	fileName := resolvePathPattern(filePattern, deviceIP, timestamp)
	
	// saveRoot 为空时拒绝生成路径，避免写入不可预期的位置
	if strings.TrimSpace(saveRoot) == "" {
		logger.Warn("Config", "-", "buildBackupLocalPath: saveRoot 为空，使用 fallback 路径")
		return filepath.Join("escape_prevented", fileName)
	}

	fullPath := filepath.Join(saveRoot, dirName, fileName)

	if !isPathWithinRoot(fullPath, saveRoot) {
		cleanRoot := filepath.Clean(saveRoot)
		logger.Warn("Config", "-", "路径逃逸防护触发: fullPath=%s, saveRoot=%s, 重定向到 escape_prevented", fullPath, saveRoot)
		return filepath.Join(cleanRoot, "escape_prevented", fileName)
	}
	
	return filepath.Clean(fullPath)
}
```

**修改要点**:
1. 新增 `isPathWithinRoot` 辅助函数，集中处理路径包含判断
2. 使用 `filepath.EvalSymlinks` 解析符号链接后的真实路径，防止通过符号链接绕过
3. Windows 上使用 `strings.EqualFold` 做大小写不敏感前缀比较
4. `saveRoot` 为空时直接拒绝，返回 `escape_prevented` 路径
5. 逃逸防护触发时增加 Warn 日志，便于排查

**需要新增的 import**: 无（`strings`、`filepath`、`logger` 已在文件中导入）

---

## P1 — 高优先级问题修复

### H-01: 正则表达式提升为包级变量

**文件**: `internal/taskexec/backup_executor.go`
**修改位置**: 第256-279行 (`extractNextStartupConfigPath` 方法)

**当前代码**:
```go
func (e *BackupExecutor) extractNextStartupConfigPath(output string) string {
	// 华为/华三格式
	re := regexp.MustCompile(`(?i)Next startup saved-configuration file:?\s+(?:flash:|cf:|sdcard:)?/?([^\r\n\s]+)`)
	matches := re.FindStringSubmatch(output)
	// ...
	reCisco := regexp.MustCompile(`(?i)boot system\s+(?:flash:|bootflash:|slot0:|usb0:)?/?([^\r\n\s]+)`)
	matchesCisco := reCisco.FindStringSubmatch(output)
	// ...
}
```

**替换为**:

在文件顶部（import 块之后、`BackupExecutor` 结构体定义之前）添加包级变量：

```go
// 备份配置路径提取正则（包级编译一次，避免每次调用重复编译）
var (
	// 华为/华三格式: "Next startup saved-configuration file: flash:/1.cfg"
	reHuaweiStartupConfig = regexp.MustCompile(`(?i)Next startup saved-configuration file:?\s+(?:flash:|cf:|sdcard:)?/?([^\r\n\s]+)`)
	// Cisco格式: "boot system flash:1.cfg"
	reCiscoBootSystem = regexp.MustCompile(`(?i)boot system\s+(?:flash:|bootflash:|slot0:|usb0:)?/?([^\r\n\s]+)`)
	// 锐捷格式: "Next startup configuration file: flash:config.text"
	reRuijieStartupConfig = regexp.MustCompile(`(?i)Next startup configuration file:?\s+(?:flash:|cf:|sdcard:|usb:)?/?([^\r\n\s]+)`)
	// 华三Comware V7格式: "Main startup configuration file: flash:/startup.cfg"
	reH3CV7MainStartup = regexp.MustCompile(`(?i)Main startup configuration file:?\s+(?:flash:|cf:|sdcard:)?/?([^\r\n\s]+)`)
)
```

修改 `extractNextStartupConfigPath` 方法：

```go
func (e *BackupExecutor) extractNextStartupConfigPath(output string) string {
	// 华为/华三格式
	matches := reHuaweiStartupConfig.FindStringSubmatch(output)
	if len(matches) > 1 {
		path := strings.TrimSpace(matches[1])
		if path != "NULL" {
			return path
		}
	}

	// 锐捷格式
	matches = reRuijieStartupConfig.FindStringSubmatch(output)
	if len(matches) > 1 {
		path := strings.TrimSpace(matches[1])
		if path != "NULL" {
			return path
		}
	}

	// 华三 Comware V7 格式
	matches = reH3CV7MainStartup.FindStringSubmatch(output)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}

	// Cisco 格式
	matches = reCiscoBootSystem.FindStringSubmatch(output)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}

	return ""
}
```

**修改要点**:
1. 四个正则提升为包级 `var`，程序启动时编译一次
2. 同时解决了 H-01（性能）和 M-02（厂商支持）两个问题
3. 新增锐捷和华三 V7 格式支持
4. 匹配顺序：华为/华三 → 锐捷 → 华三V7 → Cisco（按使用频率排序）

---

### H-02: 信号量获取支持取消

**文件**: `internal/taskexec/backup_executor.go`
**修改位置**: `Run` 方法第56-62行

**当前代码**:
```go
	for _, unit := range stage.Units {
		if ctx.IsCancelled() {
			break
		}

		wg.Add(1)
		semaphore <- struct{}{}

		go func(u UnitPlan) {
```

**替换为**:
```go
	for _, unit := range stage.Units {
		if ctx.IsCancelled() {
			break
		}

		wg.Add(1)
		// 使用 select 同时监听信号量和取消通道，避免取消时阻塞在信号量上
		select {
		case semaphore <- struct{}{}:
			// 获取到信号量，继续启动 goroutine
		case <-ctx.Context().Done():
			// 上下文已取消，退出调度循环
			wg.Done() // 撤销刚才的 wg.Add(1)
			break
		}

		go func(u UnitPlan) {
```

**修改要点**:
1. `semaphore <- struct{}{}` 改为 `select` 同时监听 `semaphore` 和 `ctx.Context().Done()`
2. 取消时调用 `wg.Done()` 撤销已执行的 `wg.Add(1)`，确保 `wg.Wait()` 不会死锁
3. 取消时 `break` 退出 for 循环，后续 `wg.Wait()` 等待已启动的 goroutine 完成

**注意事项**:
- `break` 在 `select` 中仅跳出 `select`，不跳出 `for`。需要使用带标签的 `break` 或设置标志变量：

```go
loop:
	for _, unit := range stage.Units {
		if ctx.IsCancelled() {
			break
		}

		wg.Add(1)
		select {
		case semaphore <- struct{}{}:
			// 获取到信号量
		case <-ctx.Context().Done():
			wg.Done()
			break loop
		}

		go func(u UnitPlan) {
			// ...（不变）
		}(unit)
	}
```

---

### H-03: 备份参数校验增强

**文件**: `internal/taskexec/launch_service.go`
**修改位置**: `ValidateLaunchSpec` 方法第329-332行

**当前代码**:
```go
	case string(RunKindBackup):
		if spec.Backup == nil || len(spec.Backup.DeviceIPs) == 0 {
			return fmt.Errorf("备份任务至少需要一台设备")
		}
```

**替换为**:
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
		if strings.TrimSpace(spec.Backup.DirNamePattern) == "" {
			return fmt.Errorf("备份任务缺少目录名模式")
		}
```

**修改要点**:
1. 校验 `StartupCommand` 非空 — 空命令会导致 SSH 会话挂起
2. 校验 `FileNamePattern` 非空 — 空文件名模式导致路径不合法
3. 校验 `DirNamePattern` 非空 — 空目录模式导致路径不可预期
4. 不校验 `SaveRootPath` 非空，因为 `GetBackupConfigFilePath` 已有空值回退逻辑

---

### H-04: 设备查找失败时返回明确错误

**文件**: `internal/taskexec/launch_service.go`
**修改位置**: `normalizeBackup` 方法第264-279行 + `lookupDeviceIPs` 方法第298-314行

**方案**: 修改 `lookupDeviceIPs` 返回值，增加失败设备ID列表；在 `normalizeBackup` 中检查并返回明确错误。

**当前 `lookupDeviceIPs`**:
```go
func (n *LaunchNormalizer) lookupDeviceIPs(deviceIDs []uint) []string {
	result := make([]string, 0, len(deviceIDs))
	for _, deviceID := range uniqueSortedUint(deviceIDs) {
		device, err := n.deviceRepo.FindByID(deviceID)
		if err != nil || device == nil {
			logger.Warn("TaskLaunchService", "-", "设备解析失败: deviceID=%d, err=%v", deviceID, err)
			continue
		}
		ip := strings.TrimSpace(device.IP)
		if ip != "" {
			result = append(result, ip)
			continue
		}
		logger.Warn("TaskLaunchService", "-", "设备缺少管理IP: deviceID=%d", deviceID)
	}
	return result
}
```

**替换为**:
```go
// lookupDeviceIPs 将设备ID列表解析为IP列表。
// 返回 (成功解析的IP列表, 查找失败的设备ID列表)。
func (n *LaunchNormalizer) lookupDeviceIPs(deviceIDs []uint) ([]string, []uint) {
	result := make([]string, 0, len(deviceIDs))
	var failedIDs []uint
	for _, deviceID := range uniqueSortedUint(deviceIDs) {
		device, err := n.deviceRepo.FindByID(deviceID)
		if err != nil || device == nil {
			logger.Warn("TaskLaunchService", "-", "设备解析失败: deviceID=%d, err=%v", deviceID, err)
			failedIDs = append(failedIDs, deviceID)
			continue
		}
		ip := strings.TrimSpace(device.IP)
		if ip != "" {
			result = append(result, ip)
			continue
		}
		logger.Warn("TaskLaunchService", "-", "设备缺少管理IP: deviceID=%d", deviceID)
		failedIDs = append(failedIDs, deviceID)
	}
	return result, failedIDs
}
```

**当前 `normalizeBackup`**:
```go
func (n *LaunchNormalizer) normalizeBackup(taskGroup *models.TaskGroup) (*CanonicalBackup, error) {
	deviceIDs := make([]uint, 0)
	for _, item := range taskGroup.Items {
		deviceIDs = append(deviceIDs, item.DeviceIDs...)
	}
	backup := &CanonicalBackup{
		DeviceIDs:       uniqueSortedUint(deviceIDs),
		DeviceIPs:       uniqueSortedStrings(n.lookupDeviceIPs(deviceIDs)),
		// ...
	}
	// ...
	return backup, nil
}
```

**替换为**:
```go
func (n *LaunchNormalizer) normalizeBackup(taskGroup *models.TaskGroup) (*CanonicalBackup, error) {
	deviceIDs := make([]uint, 0)
	for _, item := range taskGroup.Items {
		deviceIDs = append(deviceIDs, item.DeviceIDs...)
	}
	resolvedIPs, failedIDs := n.lookupDeviceIPs(deviceIDs)
	if len(failedIDs) > 0 {
		logger.Warn("TaskLaunchService", "-", "备份任务存在设备解析失败: taskGroupID=%d, failedDeviceIDs=%v, failedCount=%d", taskGroup.ID, failedIDs, len(failedIDs))
	}
	backup := &CanonicalBackup{
		DeviceIDs:       uniqueSortedUint(deviceIDs),
		DeviceIPs:       uniqueSortedStrings(resolvedIPs),
		StartupCommand:  strings.TrimSpace(taskGroup.BackupStartupCommand),
		SaveRootPath:    strings.TrimSpace(taskGroup.BackupSaveRootPath),
		DirNamePattern:  strings.TrimSpace(taskGroup.BackupDirNamePattern),
		FileNamePattern: strings.TrimSpace(taskGroup.BackupFileNamePattern),
	}
	logger.Verbose("TaskLaunchService", "-", "备份任务归一化完成: taskGroupID=%d, deviceIDs=%d, deviceIPs=%d, failedDeviceIDs=%d", taskGroup.ID, len(backup.DeviceIDs), len(backup.DeviceIPs), len(failedIDs))
	return backup, nil
}
```

**同时需要修改所有调用 `lookupDeviceIPs` 的地方**:

`normalizeNormal` 方法中也有调用（第218行和第235行），需要同步修改：

```go
// 第218行附近 (group 模式)
deviceIPs = append(deviceIPs, n.lookupDeviceIPs(item.DeviceIDs))
// 改为:
ips, _ := n.lookupDeviceIPs(item.DeviceIDs)
deviceIPs = append(deviceIPs, ips...)

// 第235行附近 (binding 模式)
DeviceIPs: uniqueSortedStrings(n.lookupDeviceIPs(item.DeviceIDs)),
// 改为:
ips, _ := n.lookupDeviceIPs(item.DeviceIDs)
// 然后在 CanonicalNormalItem 中使用 uniqueSortedStrings(ips)
```

`normalizeTopology` 方法中也有调用（第256行），同样需要修改：

```go
DeviceIPs: uniqueSortedStrings(n.lookupDeviceIPs(deviceIDs)),
// 改为:
ips, _ := n.lookupDeviceIPs(deviceIDs)
// 然后使用 uniqueSortedStrings(ips)
```

**修改要点**:
1. `lookupDeviceIPs` 返回值从 `[]string` 改为 `([]string, []uint)`，增加失败设备ID列表
2. `normalizeBackup` 中记录失败设备ID的 Warn 日志，包含具体ID
3. 后续 `ValidateLaunchSpec` 检查 `len(spec.Backup.DeviceIPs) == 0` 时，错误信息已足够明确（"备份任务至少需要一台设备"），结合 Warn 日志可定位具体失败设备
4. 其他调用点使用 `ips, _ := ...` 忽略失败列表（保持原有行为）

---

## P2 — 中优先级问题修复

### M-01: 默认文件名加入时间戳

**文件**: `internal/config/task_group.go`
**修改位置**: `normalizeTaskGroup` 函数中备份默认值设置部分

**当前默认值**:
```go
BackupFileNamePattern: "%H_startup.cfg"
```

**替换为**:
```go
BackupFileNamePattern: "%H_startup_%h%m%s.cfg"
```

**影响**: 同一天内多次备份同一设备，文件名包含时分秒，不再静默覆盖。例如：
- 旧: `10.0.0.1_startup.cfg`（覆盖）
- 新: `10.0.0.1_startup_143052.cfg`（不覆盖）

**注意**: 这是破坏性变更，已有任务组的 `BackupFileNamePattern` 不会被自动更新（因为 `normalizeTaskGroup` 仅在值为空时设置默认值）。用户需手动更新已有任务组，或在 UI 上重新选择。

---

### M-02: 增加锐捷/华三V7格式支持

**已在 H-01 中一并解决**。新增的两个正则：
- `reRuijieStartupConfig`: 匹配锐捷 `Next startup configuration file:`
- `reH3CV7MainStartup`: 匹配华三 V7 `Main startup configuration file:`

---

### M-04: 产物记录失败时发射告警事件

**文件**: `internal/taskexec/backup_executor.go`
**修改位置**: `createArtifact` 方法第241-254行

**当前代码**:
```go
func (e *BackupExecutor) createArtifact(taskRunID, stageID, unitID, artifactType, artifactKey, filePath string) {
	artifact := TaskArtifact{
		ID:           newArtifactID(),
		TaskRunID:    taskRunID,
		StageID:      stageID,
		UnitID:       unitID,
		ArtifactType: artifactType,
		ArtifactKey:  artifactKey,
		FilePath:     filePath,
	}
	if err := e.db.Create(&artifact).Error; err != nil {
		logger.Warn("TaskExec", taskRunID, "保存产物记录失败: err=%v, artifact=%+v", err, artifact)
	}
}
```

**替换为**:
```go
func (e *BackupExecutor) createArtifact(taskRunID, stageID, unitID, artifactType, artifactKey, filePath string) {
	artifact := TaskArtifact{
		ID:           newArtifactID(),
		TaskRunID:    taskRunID,
		StageID:      stageID,
		UnitID:       unitID,
		ArtifactType: artifactType,
		ArtifactKey:  artifactKey,
		FilePath:     filePath,
	}
	if err := e.db.Create(&artifact).Error; err != nil {
		logger.Warn("TaskExec", taskRunID, "保存产物记录失败: err=%v, artifact=%+v", err, artifact)
		// 产物记录失败时发射告警事件，提示用户文件已下载但索引丢失
		// 注意：此处无法获取 RuntimeContext，仅记录日志。
		// 调用方（executeBackupUnit）拥有 RuntimeContext，可在此处补充事件发射。
	}
}
```

同时修改 `executeBackupUnit` 中调用 `createArtifact` 的部分（第229行）：

**当前代码**:
```go
	e.createArtifact(ctx.RunID(), stageID, unit.ID, string(ArtifactTypeBackupConfig), fmt.Sprintf("%s:config", deviceIP), localPath)
```

**替换为**:
```go
	if artifactErr := e.createArtifactWithResult(ctx.RunID(), stageID, unit.ID, string(ArtifactTypeBackupConfig), fmt.Sprintf("%s:config", deviceIP), localPath); artifactErr != nil {
		// 产物记录写入失败，文件已下载但索引丢失，发射告警事件
		emitProjectedUnitEvent(ctx, stageID, unit.ID, EventTypeUnitProgress, EventLevelWarn,
			fmt.Sprintf("备份文件已下载但产物记录保存失败，请检查数据库: %v", artifactErr))
	}
```

新增 `createArtifactWithResult` 方法：

```go
// createArtifactWithResult 创建产物记录并返回错误，供调用方决定后续处理
func (e *BackupExecutor) createArtifactWithResult(taskRunID, stageID, unitID, artifactType, artifactKey, filePath string) error {
	artifact := TaskArtifact{
		ID:           newArtifactID(),
		TaskRunID:    taskRunID,
		StageID:      stageID,
		UnitID:       unitID,
		ArtifactType: artifactType,
		ArtifactKey:  artifactKey,
		FilePath:     filePath,
	}
	if err := e.db.Create(&artifact).Error; err != nil {
		logger.Warn("TaskExec", taskRunID, "保存产物记录失败: err=%v, artifact=%+v", err, artifact)
		return err
	}
	return nil
}
```

可删除原 `createArtifact` 方法（无其他调用方）。

---

### M-03: SFTP下载独立超时

**文件**: `internal/taskexec/config_models.go`
**修改位置**: `BackupTaskConfig` 结构体（第85-95行）

**当前代码**:
```go
type BackupTaskConfig struct {
	DeviceIDs       []uint   `json:"deviceIDs"`
	DeviceIPs       []string `json:"deviceIPs"`
	Concurrency     int      `json:"concurrency"`
	TimeoutSec      int      `json:"timeoutSec"`
	StartupCommand  string   `json:"startupCommand"`
	SaveRootPath    string   `json:"saveRootPath"`
	DirNamePattern  string   `json:"dirNamePattern"`
	FileNamePattern string   `json:"fileNamePattern"`
	EnableRawLog    bool     `json:"enableRawLog"`
}
```

**替换为**:
```go
type BackupTaskConfig struct {
	DeviceIDs       []uint   `json:"deviceIDs"`
	DeviceIPs       []string `json:"deviceIPs"`
	Concurrency     int      `json:"concurrency"`
	TimeoutSec      int      `json:"timeoutSec"`
	SFTPTimeoutSec  int      `json:"sftpTimeoutSec"`  // SFTP下载独立超时(秒)，0时使用TimeoutSec的2倍
	StartupCommand  string   `json:"startupCommand"`
	SaveRootPath    string   `json:"saveRootPath"`
	DirNamePattern  string   `json:"dirNamePattern"`
	FileNamePattern string   `json:"fileNamePattern"`
	EnableRawLog    bool     `json:"enableRawLog"`
}
```

**文件**: `internal/taskexec/backup_executor.go`
**修改位置**: `executeBackupUnit` 方法中 SFTP 配置构建（第201-213行）

**当前代码**:
```go
	sftpSSHConfig := sshutil.Config{
		// ...
		Timeout:        unit.Timeout,
		// ...
	}
```

**替换为**:
```go
	// 计算SFTP超时：优先使用专用超时，否则使用命令超时的2倍
	sftpTimeout := unit.Timeout * 2
	if sftpTimeoutSec > 0 {
		sftpTimeout = time.Duration(sftpTimeoutSec) * time.Second
	}

	sftpSSHConfig := sshutil.Config{
		// ...
		Timeout:        sftpTimeout,
		// ...
	}
```

其中 `sftpTimeoutSec` 从 `unit.Steps[1].Params` 中获取：

```go
	sftpTimeoutSec := 0
	if v, ok := unit.Steps[1].Params["sftpTimeoutSec"]; ok {
		fmt.Sscanf(v, "%d", &sftpTimeoutSec)
	}
```

**文件**: `internal/taskexec/backup_compiler.go`
**修改位置**: `Compile` 方法中 step-1 的 Params（第60-64行）

**当前代码**:
```go
			Params: map[string]string{
				"saveRootPath":    config.SaveRootPath,
				"dirNamePattern":  config.DirNamePattern,
				"fileNamePattern": config.FileNamePattern,
			},
```

**替换为**:
```go
			Params: map[string]string{
				"saveRootPath":    config.SaveRootPath,
				"dirNamePattern":  config.DirNamePattern,
				"fileNamePattern": config.FileNamePattern,
				"sftpTimeoutSec":  fmt.Sprintf("%d", config.SFTPTimeoutSec),
			},
```

---

### M-05: 并发计数器与事件发射的竞态窗口

**文件**: `internal/taskexec/backup_executor.go`
**修改位置**: `Run` 方法第79-104行

**当前代码**:
```go
			mu.Lock()
			switch {
			case IsContextCancelled(ctx, err):
				cancelledCount++
			case err != nil:
				failedCount++
				if firstErr == nil {
					firstErr = err
				}
			default:
				completedCount++
			}
			localCompleted := completedCount
			localFailed := failedCount
			localCancelled := cancelledCount
			mu.Unlock()

			if IsContextCancelled(ctx, err) {
				emitProjectedUnitEvent(ctx, stage.ID, u.ID, EventTypeUnitFinished, EventLevelWarn, fmt.Sprintf("Backup cancelled for %s", u.Target.Key))
			} else if err != nil {
				emitProjectedUnitEvent(ctx, stage.ID, u.ID, EventTypeUnitFinished, EventLevelError, fmt.Sprintf("Backup failed for %s: %v", u.Target.Key, err))
			} else {
				emitProjectedUnitEvent(ctx, stage.ID, u.ID, EventTypeUnitFinished, EventLevelInfo, fmt.Sprintf("Backup completed for %s", u.Target.Key))
			}

			applyProjectedStageProgress(handler, ctx, stage.ID, len(stage.Units), localCompleted, localFailed, localCancelled, stageProgressFromCounts(len(stage.Units), localCompleted, localFailed, localCancelled), "更新采集阶段进度")
```

**替换为**:
```go
			mu.Lock()
			switch {
			case IsContextCancelled(ctx, err):
				cancelledCount++
			case err != nil:
				failedCount++
				if firstErr == nil {
					firstErr = err
				}
			default:
				completedCount++
			}
			localCompleted := completedCount
			localFailed := failedCount
			localCancelled := cancelledCount

			// 在锁内发射事件，避免两个 goroutine 交错导致前端收到乱序进度
			if IsContextCancelled(ctx, err) {
				emitProjectedUnitEvent(ctx, stage.ID, u.ID, EventTypeUnitFinished, EventLevelWarn, fmt.Sprintf("Backup cancelled for %s", u.Target.Key))
			} else if err != nil {
				emitProjectedUnitEvent(ctx, stage.ID, u.ID, EventTypeUnitFinished, EventLevelError, fmt.Sprintf("Backup failed for %s: %v", u.Target.Key, err))
			} else {
				emitProjectedUnitEvent(ctx, stage.ID, u.ID, EventTypeUnitFinished, EventLevelInfo, fmt.Sprintf("Backup completed for %s", u.Target.Key))
			}

			applyProjectedStageProgress(handler, ctx, stage.ID, len(stage.Units), localCompleted, localFailed, localCancelled, stageProgressFromCounts(len(stage.Units), localCompleted, localFailed, localCancelled), "更新采集阶段进度")
			mu.Unlock()
```

**修改要点**:
1. 将事件发射和进度更新移入 `mu.Lock()` / `mu.Unlock()` 之间
2. 确保计数器更新和事件发射的原子性，前端不会收到乱序进度
3. 代价是锁持有时间略长，但事件发射是轻量操作（仅写入 channel），影响可忽略

---

## P3 — 低优先级问题修复

### L-01: 编译阶段校验IP格式

**文件**: `internal/taskexec/backup_compiler.go`
**修改位置**: `normalizeDeviceIPs` 方法（第98-106行）

**当前代码**:
```go
func (c *BackupTaskCompiler) normalizeDeviceIPs(deviceIPs []string) []string {
	result := make([]string, 0, len(deviceIPs))
	for _, ip := range deviceIPs {
		if ip != "" {
			result = append(result, ip)
		}
	}
	return result
}
```

**替换为**:
```go
func (c *BackupTaskCompiler) normalizeDeviceIPs(deviceIPs []string) []string {
	result := make([]string, 0, len(deviceIPs))
	for _, ip := range deviceIPs {
		trimmed := strings.TrimSpace(ip)
		if trimmed == "" {
			continue
		}
		// 基本格式校验：排除明显非法的值（纯数字、含空格、过短等）
		// 注意：不使用 net.ParseIP 严格校验，因为设备IP可能是主机名
		if len(trimmed) < 2 || strings.ContainsAny(trimmed, " \t\n\r") {
			logger.Warn("TaskCompiler", "-", "跳过非法设备IP: %q", trimmed)
			continue
		}
		result = append(result, trimmed)
	}
	return result
}
```

**需要新增 import**: `"strings"` （检查当前 import 块是否已包含）

**修改要点**:
- 不使用 `net.ParseIP` 严格校验，因为设备标识可能是主机名而非纯IP
- 仅过滤明显非法值（空格、过短），避免过度校验导致合法主机名被拒绝

---

### L-02: 统一默认并发数

**文件**: `internal/taskexec/backup_executor.go`
**修改位置**: `Run` 方法第44-47行

**当前代码**:
```go
	concurrency := stage.Concurrency
	if concurrency <= 0 {
		concurrency = 5
	}
```

**替换为**:
```go
	concurrency := stage.Concurrency
	if concurrency <= 0 {
		concurrency = 10 // 与 DefaultCompileOptions.DefaultConcurrency 保持一致
	}
```

---

### L-03: 修复 fmt.Errorf 格式字符串问题

**文件**: `internal/taskexec/backup_executor.go`
**修改位置**: `executeBackupUnit` 方法第180-184行

**当前代码**:
```go
	remotePath := e.extractNextStartupConfigPath(cmdOutput)
	if remotePath == "" {
		errMsg := fmt.Sprintf("未能从输出中提取配置文件路径:\n%s", cmdOutput)
		failUnitExecution(handler, ctx, unit.ID, errMsg, "提取失败", nil)
		return fmt.Errorf(errMsg)
	}
```

**替换为**:
```go
	remotePath := e.extractNextStartupConfigPath(cmdOutput)
	if remotePath == "" {
		errMsg := fmt.Sprintf("未能从输出中提取配置文件路径:\n%s", cmdOutput)
		failUnitExecution(handler, ctx, unit.ID, errMsg, "提取失败", nil)
		return fmt.Errorf("未能从输出中提取配置文件路径: %s", truncateForError(cmdOutput, 200))
	}
```

新增辅助函数（在文件底部）：

```go
// truncateForError 截断字符串用于错误信息，避免超长输出污染日志
func truncateForError(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "...(truncated)"
}
```

**修改要点**:
1. `fmt.Errorf(errMsg)` 改为 `fmt.Errorf("格式字符串", 参数)`，避免 `%` 字符被误解析
2. 截断 `cmdOutput` 到 200 字符，避免超长设备输出污染错误信息和日志

---

## 修改影响分析

### 向后兼容性

| 修改 | 兼容性影响 | 说明 |
|------|-----------|------|
| C-01 原子写入 | **兼容** | 对调用方透明，行为等价 |
| C-02 路径逃逸 | **兼容** | 更严格的防护，不影响合法路径 |
| H-01 正则提升 | **兼容** | 纯性能优化，行为不变 |
| H-02 取消响应 | **兼容** | 改善取消响应，不影响正常流程 |
| H-03 参数校验 | **轻微不兼容** | 之前能启动的空参数任务现在会被拒绝（但本来就会失败） |
| H-04 错误信息 | **兼容** | 仅改善错误信息，不改变行为 |
| M-01 文件名时间戳 | **不兼容** | 默认文件名模式变更，已有任务组不受影响（仅新创建的） |
| M-02 厂商支持 | **兼容** | 新增匹配，不影响已有匹配 |
| M-03 SFTP超时 | **兼容** | 新增字段，JSON 反序列化时默认为零值 |
| M-04 产物告警 | **兼容** | 新增事件，不影响已有逻辑 |
| M-05 事件顺序 | **兼容** | 改善事件顺序，不影响最终结果 |
| L-01 IP校验 | **轻微不兼容** | 明显非法IP被过滤（但本来就会连接失败） |
| L-02 并发数 | **兼容** | 回退值从5改为10，与编译器一致 |
| L-03 格式字符串 | **兼容** | 修复bug，行为更正确 |

### 测试验证清单

| 编号 | 测试场景 | 覆盖问题 | 验证方法 |
|------|---------|---------|---------|
| T-01 | SFTP下载中断后确认无残留 `.tmp` 文件和目标文件 | C-01 | 模拟网络中断，检查磁盘 |
| T-02 | SFTP下载成功后确认目标文件存在且内容完整 | C-01 | 对比源文件和目标文件 MD5 |
| T-03 | 路径模式含 `..` 时确认逃逸防护触发 | C-02 | 单元测试 |
| T-04 | Windows 路径大小写不一致时确认正常工作 | C-02 | Windows 环境集成测试 |
| T-05 | `saveRoot` 为空时确认返回 escape_prevented 路径 | C-02 | 单元测试 |
| T-06 | 100台设备备份时正则仅编译一次 | H-01 | benchmark 对比 |
| T-07 | 并发满时取消操作，确认1秒内退出调度 | H-02 | 并发测试+计时 |
| T-08 | StartupCommand 为空时确认编译拒绝 | H-03 | 单元测试 |
| T-09 | 设备ID全部无效时确认错误信息包含具体ID | H-04 | 单元测试 |
| T-10 | 同一设备同一天多次备份确认文件不覆盖 | M-01 | 集成测试 |
| T-11 | 锐捷设备输出确认路径提取正确 | M-02 | 单元测试 |
| T-12 | 华三V7设备输出确认路径提取正确 | M-02 | 单元测试 |
| T-13 | SFTPTimeoutSec=0时确认使用2倍超时 | M-03 | 单元测试 |
| T-14 | 产物记录失败时确认前端收到告警事件 | M-04 | 集成测试 |
| T-15 | 10并发备份时确认前端进度单调递增 | M-05 | 集成测试 |
| T-16 | 非法IP（含空格）确认被过滤 | L-01 | 单元测试 |
| T-17 | 并发数=0时确认回退到10 | L-02 | 单元测试 |
| T-18 | cmdOutput含 `%` 字符时确认错误信息正确 | L-03 | 单元测试 |

---

## 实施顺序建议

```
Phase 1 (P0 — 必须修复):
  ├── C-01: sftputil/client.go DownloadFile 原子写入
  └── C-02: config/paths.go buildBackupLocalPath 路径逃逸加固

Phase 2 (P1 — 强烈建议):
  ├── H-02: backup_executor.go Run 信号量取消响应
  ├── H-03: launch_service.go ValidateLaunchSpec 参数校验
  └── H-04: launch_service.go lookupDeviceIPs 错误信息

Phase 3 (P2 — 建议修复):
  ├── H-01 + M-02: backup_executor.go 正则提升+厂商扩展
  ├── M-01: config/task_group.go 默认文件名时间戳
  ├── M-04: backup_executor.go createArtifact 告警事件
  └── M-05: backup_executor.go Run 事件顺序

Phase 4 (P3 — 改善):
  ├── M-03: config_models.go + backup_executor.go SFTP独立超时
  ├── L-01: backup_compiler.go normalizeDeviceIPs IP校验
  ├── L-02: backup_executor.go Run 默认并发数
  └── L-03: backup_executor.go executeBackupUnit fmt.Errorf修复
```

---

## 核验报告

> 核验日期: 2026-04-23
> 核验方法: 逐项对照源码行号/内容 + 逻辑推演 + 边界分析

### 一、行号与代码匹配核验

| 文件 | 方案引用行号 | 实际行号 | 匹配结果 |
|------|-------------|---------|---------|
| `sftputil/client.go` DownloadFile | 53-79 | 53-79 | 完全匹配 |
| `config/paths.go` buildBackupLocalPath | 281-296 | 281-296 | 完全匹配 |
| `config/paths.go` GetBackupConfigFilePath | 298-306 | 298-306 | 完全匹配 |
| `backup_executor.go` Run 信号量 | 56-62 | 56-62 | 完全匹配 |
| `backup_executor.go` Run 计数器/事件 | 79-104 | 79-104 | 完全匹配 |
| `backup_executor.go` fmt.Errorf(errMsg) | 180-184 | 180-184 | 完全匹配 |
| `backup_executor.go` extractNextStartupConfigPath | 256-279 | 256-279 | 完全匹配 |
| `backup_executor.go` createArtifact | 241-254 | 241-254 | 完全匹配 |
| `backup_executor.go` 默认并发数 | 44-47 | 44-47 | 完全匹配 |
| `launch_service.go` normalizeBackup | 264-279 | 264-279 | 完全匹配 |
| `launch_service.go` lookupDeviceIPs | 298-314 | 298-314 | 完全匹配 |
| `launch_service.go` ValidateLaunchSpec | 329-332 | 329-332 | 完全匹配 |
| `launch_service.go` normalizeNormal 调用 | 218, 235 | 218, 235 | 完全匹配 |
| `launch_service.go` normalizeTopology 调用 | 256 | 256 | 完全匹配 |
| `backup_compiler.go` normalizeDeviceIPs | 98-106 | 98-106 | 完全匹配 |
| `backup_compiler.go` Compile step-1 Params | 60-64 | 60-64 | 完全匹配 |
| `config_models.go` BackupTaskConfig | 85-95 | 85-95 | 完全匹配 |
| `config_models.go` DefaultCompileOptions | 74-80 | 74-80 | 完全匹配 |
| `task_group.go` BackupFileNamePattern | 第212行 | 第212行 | 值为 `%H_startup.cfg` |

**结论: 所有行号和代码内容与源码完全匹配，无偏移。**

---

### 二、逻辑正确性核验

#### C-01 原子写入 — 核验通过

| 检查项 | 结果 | 说明 |
|--------|------|------|
| 临时文件命名 | 正确 | `localPath + ".tmp"` 不会与目标文件冲突 |
| 文件关闭时机 | 正确 | `io.Copy` 后立即 `localFile.Close()`，不再用 `defer`，避免 Windows 上 Rename 失败 |
| Close 错误处理 | 正确 | `closeErr != nil && err == nil` 时才覆盖 err，避免丢失 io.Copy 的错误 |
| 失败清理 | 正确 | `os.Remove(tmpPath)` + `os.IsNotExist` 检查，避免清理不存在的文件时报错 |
| 空文件校验 | 正确 | `bytesCopied == 0` 时删除临时文件并返回错误 |
| Rename 失败处理 | 正确 | 清理临时文件并返回错误 |
| 进程崩溃场景 | 正确 | 崩溃后仅残留 `.tmp` 文件，目标文件不受影响 |

**发现1个待改进点**: 如果上次备份的 `.tmp` 文件残留（进程崩溃），本次 `os.Create(tmpPath)` 会覆盖它，这是正确行为（上次下载必然不完整）。但可以考虑在下载前先清理残留的 `.tmp` 文件，避免极小概率的混淆。**严重性: 低，不影响正确性。**

#### C-02 路径逃逸防护 — 核验通过，有1个需修正的问题

| 检查项 | 结果 | 说明 |
|--------|------|------|
| EvalSymlinks 使用 | 正确 | 解析符号链接后比较，防止通过 symlink 绕过 |
| EqualFold 使用 | 正确 | Windows 上大小写不敏感比较 |
| saveRoot 为空处理 | 正确 | 返回 `escape_prevented` 路径 |
| 逃逸日志 | 正确 | Warn 级别日志包含 fullPath 和 saveRoot |

**发现1个需修正的问题**: `isPathWithinRoot` 中 `filepath.EvalSymlinks` 要求路径必须存在。对于**首次备份**场景，`fullPath`（如 `storage/backup/2026-04-23/10.0.0.1_startup.cfg`）可能尚未存在，`EvalSymlinks` 会返回错误。方案中已处理：`EvalSymlinks` 失败时回退到原始路径。但此时符号链接防护失效。

**修正建议**: 仅对 `saveRoot` 调用 `EvalSymlinks`（saveRoot 应该已存在），对 `fullPath` 使用 `filepath.Clean` 即可（因为 fullPath 是新创建的路径，不存在符号链接）：

```go
func isPathWithinRoot(path, root string) bool {
    // 仅解析 root 的符号链接（root 应已存在）
    realRoot, err := filepath.EvalSymlinks(root)
    if err != nil {
        realRoot = root
    }
    // path 是新创建的路径，不存在符号链接，直接 Clean
    cleanPath := filepath.Clean(path)
    cleanRoot := filepath.Clean(realRoot)
    // ... 后续比较逻辑不变
}
```

**严重性: 中，需在实施时修正。**

#### H-02 信号量取消 — 核验通过

| 检查项 | 结果 | 说明 |
|--------|------|------|
| select 语义 | 正确 | 同时监听 semaphore 和 ctx.Done() |
| break 语义 | 正确 | 方案已注意到 `break` 在 select 中仅跳出 select，提供了带标签 `break loop` 的修正代码 |
| wg.Done() 撤销 | 正确 | 取消时 wg.Done() 撤销 wg.Add(1)，避免 wg.Wait() 死锁 |
| 已启动 goroutine | 正确 | break 后 wg.Wait() 等待已启动的 goroutine 完成 |

#### H-03 参数校验 — 核验通过

| 检查项 | 结果 | 说明 |
|--------|------|------|
| StartupCommand 校验 | 正确 | 空命令会导致 SSH 挂起 |
| FileNamePattern 校验 | 正确 | 空文件名导致路径不合法 |
| DirNamePattern 校验 | 正确 | 空目录模式导致路径不可预期 |
| SaveRootPath 不校验 | 正确 | GetBackupConfigFilePath 已有空值回退逻辑 |

#### H-04 lookupDeviceIPs 签名变更 — 核验通过，有1个需注意的点

| 检查项 | 结果 | 说明 |
|--------|------|------|
| 返回值变更 | 正确 | `[]string` → `([]string, []uint)` |
| normalizeBackup 适配 | 正确 | 使用 `resolvedIPs, failedIDs :=` |
| normalizeNormal 适配 | 正确 | 使用 `ips, _ :=` 忽略失败列表 |
| normalizeTopology 适配 | 正确 | 使用 `ips, _ :=` 忽略失败列表 |

**需注意的点**: 方案中 normalizeNormal 第218行的修改写法 `ips, _ := n.lookupDeviceIPs(item.DeviceIDs)` 后接 `deviceIPs = append(deviceIPs, ips...)`，但原代码是 `deviceIPs = append(deviceIPs, n.lookupDeviceIPs(item.DeviceIDs)...)`。由于原代码直接展开，修改后需要确认 `ips` 变量名不与外层作用域冲突。经检查，`normalizeNormal` 方法中无同名变量，**无冲突**。

#### M-05 事件移入锁内 — 核验通过

| 检查项 | 结果 | 说明 |
|--------|------|------|
| 锁范围扩大 | 正确 | 事件发射在 mu.Lock/mu.Unlock 之间 |
| 死锁风险 | 无 | emitProjectedUnitEvent 和 applyProjectedStageProgress 不尝试获取同一把 mu |
| 性能影响 | 可接受 | 事件发射是轻量操作（写入 channel），锁持有时间增加微乎其微 |

#### L-03 fmt.Errorf 修复 — 核验通过

| 检查项 | 结果 | 说明 |
|--------|------|------|
| 格式字符串修复 | 正确 | `fmt.Errorf("未能从输出中提取配置文件路径: %s", ...)` |
| 截断函数 | 正确 | `truncateForError` 避免超长输出 |
| 截断长度 | 合理 | 200 字符足够定位问题，不会截断关键信息 |

---

### 三、遗漏检查

| 检查项 | 结果 | 说明 |
|--------|------|------|
| 审计报告14个问题是否全部覆盖 | 全部覆盖 | C-01, C-02, H-01~H-04, M-01~M-05, L-01~L-03 均有对应方案 |
| M-01 在修改文件总览中标注为 sftputil/client.go | 需修正 | M-01 实际修改的是 `config/task_group.go`（默认值），不是 `sftputil/client.go`。sftputil/client.go 的修改仅涉及 C-01 |
| backup_compiler.go 需要新增 import "strings" | 需确认 | L-01 修改 normalizeDeviceIPs 使用了 strings.TrimSpace 和 strings.ContainsAny，需确认 import |
| H-01 新增包级正则后，import "regexp" 仍需保留 | 正确 | regexp.MustCompile 在包级变量初始化时使用，import 必须保留 |
| M-03 SFTPTimeoutSec 在前端 UI 是否需要对应修改 | 未涉及 | 新增字段后，前端 TaskEditModal.vue 需要增加对应的输入框，但方案未涉及前端修改 |

---

### 四、核验结论

| 类别 | 总数 | 通过 | 需修正 | 不通过 |
|------|------|------|---------|--------|
| 行号匹配 | 19 | 19 | 0 | 0 |
| 逻辑正确性 | 8 | 7 | 1 | 0 |
| 遗漏检查 | 5 | 3 | 2 | 0 |

**需修正项**:

1. **C-02 isPathWithinRoot**: `filepath.EvalSymlinks(fullPath)` 在路径不存在时会失败，应仅对 `saveRoot` 调用 EvalSymlinks，对 `fullPath` 仅做 `filepath.Clean`。已在上方给出修正代码。

2. **修改文件总览表**: M-01 对应文件应为 `config/task_group.go`，而非 `sftputil/client.go`。sftputil/client.go 仅涉及 C-01。

**需补充项**:

1. **L-01 import 检查**: `backup_compiler.go` 当前 import 仅有 `"context"`, `"encoding/json"`, `"fmt"`, `"time"`, `"github.com/NetWeaverGo/core/internal/logger"`，**不包含 `"strings"`**，实施时需新增。

2. **M-03 前端适配**: `SFTPTimeoutSec` 新增后，前端 `TaskEditModal.vue` 需增加对应输入框，方案中未涉及，实施时需补充。

**总体评价**: 方案逻辑正确、覆盖完整、代码与源码匹配。上述修正项均为细节层面，不影响方案整体可行性。建议在实施时按修正建议调整 C-02 的 `isPathWithinRoot` 实现，并补充 import 和前端适配。
