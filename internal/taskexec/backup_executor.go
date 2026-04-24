package taskexec

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/executor"
	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/repository"
	"github.com/NetWeaverGo/core/internal/sftputil"
	"github.com/NetWeaverGo/core/internal/sshutil"
	"gorm.io/gorm"
)

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

// BackupExecutor 备份采集执行器
type BackupExecutor struct {
	repo        repository.DeviceRepository
	pathManager *config.PathManager
	db          *gorm.DB
}

// NewBackupExecutor 创建备份执行器
func NewBackupExecutor(repo repository.DeviceRepository, db *gorm.DB) *BackupExecutor {
	return &BackupExecutor{
		repo:        repo,
		pathManager: config.GetPathManager(),
		db:          db,
	}
}

// Kind 返回支持的阶段类型
func (e *BackupExecutor) Kind() string {
	return string(StageKindBackupCollect)
}

// Run 执行备份阶段
func (e *BackupExecutor) Run(ctx RuntimeContext, stage *StagePlan) error {
	logger.Info("BackupExecutor", ctx.RunID(), "开始执行备份阶段: stage=%s, units=%d, concurrency=%d", stage.Name, len(stage.Units), stage.Concurrency)

	concurrency := stage.Concurrency
	if concurrency <= 0 {
		concurrency = 10 // 与 DefaultCompileOptions.DefaultConcurrency 保持一致
	}

	handler := NewErrorHandler(ctx.RunID())
	semaphore := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var completedCount, failedCount, cancelledCount int
	var firstErr error
	var mu sync.Mutex

loop:
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
			break loop
		}

		go func(u UnitPlan) {
			defer wg.Done()
			defer func() { <-semaphore }()

			if ctx.IsCancelled() {
				handler.MarkUnitCancelled(ctx, u.ID, "run cancelled before collect unit start", intPtrLocal(0))
				mu.Lock()
				cancelledCount++
				mu.Unlock()
				applyProjectedStageProgress(handler, ctx, stage.ID, len(stage.Units), completedCount, failedCount, cancelledCount, stageProgressFromCounts(len(stage.Units), completedCount, failedCount, cancelledCount), "更新采集阶段进度")
				return
			}

			err := e.executeBackupUnit(ctx, stage.ID, &u)

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
		}(unit)
	}

	wg.Wait()
	logger.Info("TaskExec", ctx.RunID(), "Backup stage completed: success=%d, failed=%d, cancelled=%d", completedCount, failedCount, cancelledCount)
	if ctx.IsCancelled() {
		return ctx.Context().Err()
	}
	return firstErr
}

// executeBackupUnit 执行单个设备的备份单元
func (e *BackupExecutor) executeBackupUnit(ctx RuntimeContext, stageID string, unit *UnitPlan) error {
	handler := NewErrorHandler(ctx.RunID())
	if ctx.IsCancelled() {
		return cancelUnitExecution(ctx, handler, unit.ID, "run cancelled before collect unit start", intPtrLocal(0))
	}

	if err := markUnitRunning(handler, ctx, unit.ID, "设置采集 Unit 为 running"); err != nil {
		return err
	}

	if unit.Target.Type != "device_ip" {
		errMsg := fmt.Sprintf("unsupported target type: %s", unit.Target.Type)
		failUnitExecution(handler, ctx, unit.ID, errMsg, "写入采集 Unit 失败状态", nil)
		return fmt.Errorf("%s", errMsg)
	}

	deviceIP := unit.Target.Key
	scope := LogScope{RunID: ctx.RunID(), StageID: stageID, UnitID: unit.ID, UnitKey: deviceIP}
	runtimeLogger := ctx.Logger(scope)

	device, err := e.repo.FindByIP(deviceIP)
	if err != nil {
		errMsg := fmt.Sprintf("device not found: %v", err)
		failUnitExecution(handler, ctx, unit.ID, errMsg, "写入采集设备不存在状态", nil)
		projectTaskexecLifecycleRecord(ctx, runtimeLogger, scope, recordDeviceMissing, fmt.Sprintf("采集设备不存在: %v", err), 0, 0)
		return fmt.Errorf("device not found: %w", err)
	}

	startupCommand := unit.Steps[0].Params["startupCommand"]
	saveRootPath := unit.Steps[1].Params["saveRootPath"]
	dirNamePattern := unit.Steps[1].Params["dirNamePattern"]
	fileNamePattern := unit.Steps[1].Params["fileNamePattern"]

	// [step-0] 获取配置文件路径
	emitProjectedUnitEvent(ctx, stageID, unit.ID, EventTypeStepStarted, EventLevelInfo, "获取配置路径...")
	
	opts := executor.ExecutorOptions{
		Vendor: device.Vendor,
	}
	exec := executor.NewDeviceExecutor(
		device.IP,
		device.Port,
		device.Username,
		device.Password,
		opts,
	)
	defer exec.Close()

	if err := exec.Connect(ctx.Context(), unit.Timeout); err != nil {
		errMsg := fmt.Sprintf("SSH连接失败: %v", err)
		failUnitExecution(handler, ctx, unit.ID, errMsg, "写入连接失败状态", nil)
		projectTaskexecLifecycleRecord(ctx, runtimeLogger, scope, recordSessionConnectFailed, errMsg, 0, 0)
		return fmt.Errorf("SSH连接失败: %w", err)
	}

	cmdOutput, err := exec.ExecuteCommandSync(ctx.Context(), startupCommand, unit.Timeout)
	if err != nil {
		errMsg := fmt.Sprintf("执行启动配置命令失败: %v", err)
		failUnitExecution(handler, ctx, unit.ID, errMsg, "写入命令失败状态", nil)
		return fmt.Errorf("执行启动配置命令失败: %w", err)
	}
	
	remotePath := e.extractNextStartupConfigPath(cmdOutput)
	if remotePath == "" {
		errMsg := fmt.Sprintf("未能从输出中提取配置文件路径:\n%s", cmdOutput)
		failUnitExecution(handler, ctx, unit.ID, errMsg, "提取失败", nil)
		return fmt.Errorf("未能从输出中提取配置文件路径: %s", truncateForError(cmdOutput, 200))
	}
	
	emitProjectedUnitEvent(ctx, stageID, unit.ID, EventTypeStepFinished, EventLevelInfo, fmt.Sprintf("发现配置文件: %s", remotePath))

	// [step-1] SFTP下载配置文件
	// 注意：必须使用独立的纯净SSH连接建立SFTP会话，
	// 因为华为/华三等设备拒绝在已有shell/pty通道的连接上打开sftp子系统。
	emitProjectedUnitEvent(ctx, stageID, unit.ID, EventTypeStepStarted, EventLevelInfo, "正在通过 SFTP 下载配置...")

	localPath := e.pathManager.GetBackupConfigFilePath(
		saveRootPath,
		dirNamePattern,
		fileNamePattern,
		device.IP,
		time.Now(),
	)

	// 构建SFTP专用的SSH配置（与DeviceExecutor.Connect()保持一致的策略配置）
	// 计算SFTP超时：优先使用专用超时，否则使用命令超时的2倍
	sftpTimeoutSec := 0
	if v, ok := unit.Steps[1].Params["sftpTimeoutSec"]; ok {
		fmt.Sscanf(v, "%d", &sftpTimeoutSec)
	}
	sftpTimeout := unit.Timeout * 2
	if sftpTimeoutSec > 0 {
		sftpTimeout = time.Duration(sftpTimeoutSec) * time.Second
	}

	hostKeyPolicy, knownHostsPath := config.ResolveSSHHostKeyPolicy()
	sftpSSHConfig := sshutil.Config{
		IP:             device.IP,
		Port:           device.Port,
		Username:       device.Username,
		Password:       device.Password,
		Timeout:        sftpTimeout,
		HostKeyPolicy:  hostKeyPolicy,
		KnownHostsPath: knownHostsPath,
		// 注意：PTY 不设置 — SFTP连接不需要伪终端
		// 注意：Algorithms 不设置 — 为nil时NewRawSSHClient内部使用内置默认兼容配置
	}

	sftpClient, err := sftputil.NewSFTPClient(ctx.Context(), sftpSSHConfig)
	if err != nil {
		errMsg := fmt.Sprintf("建立SFTP会话失败: %v", err)
		failUnitExecution(handler, ctx, unit.ID, errMsg, "SFTP失败", nil)
		return fmt.Errorf("建立SFTP会话失败: %w", err)
	}
	defer sftpClient.Close()

	if err := sftpClient.DownloadFile(remotePath, localPath); err != nil {
		errMsg := fmt.Sprintf("SFTP下载文件失败: %v", err)
		failUnitExecution(handler, ctx, unit.ID, errMsg, "下载失败", nil)
		return fmt.Errorf("SFTP下载文件失败: %w", err)
	}

	if artifactErr := e.createArtifactWithResult(ctx.RunID(), stageID, unit.ID, string(ArtifactTypeBackupConfig), fmt.Sprintf("%s:config", deviceIP), localPath); artifactErr != nil {
		// 产物记录写入失败，文件已下载但索引丢失，发射告警事件
		emitProjectedUnitEvent(ctx, stageID, unit.ID, EventTypeUnitProgress, EventLevelWarn,
			fmt.Sprintf("备份文件已下载但产物记录保存失败，请检查数据库: %v", artifactErr))
	}

	emitProjectedUnitEvent(ctx, stageID, unit.ID, EventTypeStepFinished, EventLevelInfo, "下载配置完成")

	if err := completeUnitExecution(handler, ctx, unit.ID, string(UnitStatusCompleted), 2, "写入采集完成状态"); err != nil {
		return err
	}
	projectTaskexecLifecycleRecord(ctx, runtimeLogger, scope, recordExecutionSucceeded, "备份完成", 2, 2)

	return nil
}

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

// truncateForError 截断字符串用于错误信息，避免超长输出污染日志
func truncateForError(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "...(truncated)"
}
