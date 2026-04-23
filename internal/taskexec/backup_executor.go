package taskexec

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/executor"
	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/repository"
	"github.com/pkg/sftp"
	"gorm.io/gorm"
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
		concurrency = 5
	}

	handler := NewErrorHandler(ctx.RunID())
	semaphore := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var completedCount, failedCount, cancelledCount int
	var firstErr error
	var mu sync.Mutex

	for _, unit := range stage.Units {
		if ctx.IsCancelled() {
			break
		}

		wg.Add(1)
		semaphore <- struct{}{}

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
			mu.Unlock()

			if IsContextCancelled(ctx, err) {
				emitProjectedUnitEvent(ctx, stage.ID, u.ID, EventTypeUnitFinished, EventLevelWarn, fmt.Sprintf("Backup cancelled for %s", u.Target.Key))
			} else if err != nil {
				emitProjectedUnitEvent(ctx, stage.ID, u.ID, EventTypeUnitFinished, EventLevelError, fmt.Sprintf("Backup failed for %s: %v", u.Target.Key, err))
			} else {
				emitProjectedUnitEvent(ctx, stage.ID, u.ID, EventTypeUnitFinished, EventLevelInfo, fmt.Sprintf("Backup completed for %s", u.Target.Key))
			}

			applyProjectedStageProgress(handler, ctx, stage.ID, len(stage.Units), localCompleted, localFailed, localCancelled, stageProgressFromCounts(len(stage.Units), localCompleted, localFailed, localCancelled), "更新采集阶段进度")
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
		return fmt.Errorf(errMsg)
	}
	
	emitProjectedUnitEvent(ctx, stageID, unit.ID, EventTypeStepFinished, EventLevelInfo, fmt.Sprintf("发现配置文件: %s", remotePath))

	// [step-1] SFTP下载配置文件
	emitProjectedUnitEvent(ctx, stageID, unit.ID, EventTypeStepStarted, EventLevelInfo, "正在通过 SFTP 下载配置...")

	localPath := e.pathManager.GetBackupConfigFilePath(
		saveRootPath, 
		dirNamePattern, 
		fileNamePattern, 
		device.IP, 
		time.Now(),
	)

	sftpClient, err := sftp.NewClient(exec.Client.Client)
	if err != nil {
		errMsg := fmt.Sprintf("建立SFTP会话失败: %v", err)
		failUnitExecution(handler, ctx, unit.ID, errMsg, "SFTP失败", nil)
		return fmt.Errorf("建立SFTP会话失败: %w", err)
	}
	defer sftpClient.Close()

	if err := e.downloadConfigFile(sftpClient, remotePath, localPath); err != nil {
		errMsg := fmt.Sprintf("SFTP下载文件失败: %v", err)
		failUnitExecution(handler, ctx, unit.ID, errMsg, "下载失败", nil)
		return fmt.Errorf("SFTP下载文件失败: %w", err)
	}

	e.createArtifact(ctx.RunID(), stageID, unit.ID, string(ArtifactTypeBackupConfig), fmt.Sprintf("%s:config", deviceIP), localPath)

	emitProjectedUnitEvent(ctx, stageID, unit.ID, EventTypeStepFinished, EventLevelInfo, "下载配置完成")

	if err := completeUnitExecution(handler, ctx, unit.ID, string(UnitStatusCompleted), 2, "写入采集完成状态"); err != nil {
		return err
	}
	projectTaskexecLifecycleRecord(ctx, runtimeLogger, scope, recordExecutionSucceeded, "备份完成", 2, 2)

	return nil
}

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

func (e *BackupExecutor) extractNextStartupConfigPath(output string) string {
	re := regexp.MustCompile(`(?i)Next startup saved-configuration file:?\s+([^\r\n\s]+)`)
	matches := re.FindStringSubmatch(output)
	if len(matches) > 1 {
		path := strings.TrimSpace(matches[1])
		if path != "NULL" {
			return path
		}
	}
	
	reCisco := regexp.MustCompile(`(?i)boot system\s+(?:flash:)?([^\r\n\s]+)`)
	matchesCisco := reCisco.FindStringSubmatch(output)
	if len(matchesCisco) > 1 {
		return strings.TrimSpace(matchesCisco[1])
	}
	
	return ""
}

func (e *BackupExecutor) downloadConfigFile(sftpClient *sftp.Client, remotePath, localPath string) error {
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("创建本地目录失败: %w", err)
	}
	
	srcFile, err := sftpClient.Open(remotePath)
	if err != nil {
		return fmt.Errorf("无法打开远端文件 %s: %w", remotePath, err)
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(localPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("无法创建本地文件 %s: %w", localPath, err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("复制文件数据失败: %w", err)
	}
	
	return nil
}
