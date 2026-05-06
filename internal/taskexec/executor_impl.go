package taskexec

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/executor"
	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
	"github.com/NetWeaverGo/core/internal/parser"
	"github.com/NetWeaverGo/core/internal/repository"
	"gorm.io/gorm"
)

// DeviceCommandExecutor for normal tasks
type DeviceCommandExecutor struct {
	repo     repository.DeviceRepository
	settings *models.GlobalSettings
}

// NewDeviceCommandExecutor creates executor
func NewDeviceCommandExecutor(repo repository.DeviceRepository) *DeviceCommandExecutor {
	settings, _, _ := config.LoadSettings()
	return &DeviceCommandExecutor{repo: repo, settings: settings}
}

// Kind returns executor type
func (e *DeviceCommandExecutor) Kind() string {
	return string(StageKindDeviceCommand)
}

// Run executes the stage
func (e *DeviceCommandExecutor) Run(ctx RuntimeContext, stage *StagePlan) error {
	logger.Info("TaskExec", ctx.RunID(), "Start device command stage: %s, units: %d", stage.Name, len(stage.Units))

	concurrency := stage.Concurrency
	if concurrency <= 0 {
		concurrency = 10
	}

	handler := NewErrorHandler(ctx.RunID())
	semaphore := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var completedCount, failedCount, cancelledCount int
	var firstErr error
	var mu sync.Mutex
	unitProgress := make(map[string]int, len(stage.Units))
	for _, unit := range stage.Units {
		unitProgress[unit.ID] = 0
	}

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

			reportUnitProgress := func(doneSteps, totalSteps int) {
				progress := unitProgressPercent(doneSteps, totalSteps)
				mu.Lock()
				if progress > unitProgress[u.ID] {
					unitProgress[u.ID] = progress
				}
				localCompleted := completedCount
				localFailed := failedCount
				localCancelled := cancelledCount
				stageProgress := aggregateUnitProgress(unitProgress, len(stage.Units))
				mu.Unlock()

				applyProjectedStageProgress(handler, ctx, stage.ID, len(stage.Units), localCompleted, localFailed, localCancelled, stageProgress, "更新命令阶段细粒度进度")
			}

			if ctx.IsCancelled() {
				handler.MarkUnitCancelled(ctx, u.ID, "run cancelled before unit start", intPtrLocal(0))
				mu.Lock()
				cancelledCount++
				unitProgress[u.ID] = 0
				stageProgress := aggregateUnitProgress(unitProgress, len(stage.Units))
				mu.Unlock()
				applyProjectedStageProgress(handler, ctx, stage.ID, len(stage.Units), completedCount, failedCount, cancelledCount, stageProgress, "更新命令阶段细粒度进度")
				return
			}

			err := e.executeUnit(ctx, stage.ID, &u, reportUnitProgress)

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
			stageProgress := aggregateUnitProgress(unitProgress, len(stage.Units))
			mu.Unlock()

			applyProjectedStageProgress(handler, ctx, stage.ID, len(stage.Units), localCompleted, localFailed, localCancelled, stageProgress, "更新命令阶段细粒度进度")
		}(unit)
	}

	wg.Wait()
	logger.Info("TaskExec", ctx.RunID(), "Stage completed: success=%d, failed=%d, cancelled=%d", completedCount, failedCount, cancelledCount)
	if ctx.IsCancelled() {
		return ctx.Context().Err()
	}
	return firstErr
}

func (e *DeviceCommandExecutor) executeUnit(ctx RuntimeContext, stageID string, unit *UnitPlan, reportProgress func(doneSteps, totalSteps int)) error {
	handler := NewErrorHandler(ctx.RunID())
	if ctx.IsCancelled() {
		return cancelUnitExecution(ctx, handler, unit.ID, "run cancelled before unit start", intPtrLocal(0))
	}

	if err := markUnitRunning(handler, ctx, unit.ID, "设置命令执行 Unit 为 running"); err != nil {
		return err
	}

	if ctx.IsCancelled() {
		return cancelUnitExecution(ctx, handler, unit.ID, "run cancelled after unit start", intPtrLocal(0))
	}

	// Get device info from unit target
	if unit.Target.Type != "device_ip" {
		errMsg := fmt.Sprintf("unsupported target type: %s", unit.Target.Type)
		failUnitExecution(handler, ctx, unit.ID, errMsg, "写入命令执行 Unit 失败状态", nil)
		return fmt.Errorf("%s", errMsg)
	}

	deviceIP := unit.Target.Key
	scope := LogScope{RunID: ctx.RunID(), StageID: stageID, UnitID: unit.ID, UnitKey: deviceIP}
	runtimeLogger := ctx.Logger(scope)
	logSession := runtimeLogger.Session(scope)
	logger.Debug("TaskExec", ctx.RunID(), "Execute unit for device: %s", deviceIP)

	// Get device from repository
	device, err := e.repo.FindByIP(deviceIP)
	if err != nil {
		logger.Error("TaskExec", ctx.RunID(), "Failed to find device %s: %v", deviceIP, err)
		errMsg := fmt.Sprintf("Device not found: %s", deviceIP)
		failUnitExecution(handler, ctx, unit.ID, errMsg, "写入设备不存在失败状态", nil)
		projectTaskexecLifecycleRecord(ctx, runtimeLogger, scope, recordDeviceMissing, fmt.Sprintf("设备不存在: %s", deviceIP), 0, 0)
		emitProjectedUnitEvent(ctx, stageID, unit.ID, EventTypeUnitFinished, EventLevelError, fmt.Sprintf("Device not found: %s", deviceIP))
		return fmt.Errorf("%s", errMsg)
	}

	// Build commands from steps
	commands := make([]string, 0, len(unit.Steps))
	for _, step := range unit.Steps {
		if step.Kind == "command" && step.Command != "" {
			commands = append(commands, step.Command)
		}
	}

	if len(commands) == 0 {
		logger.Warn("TaskExec", ctx.RunID(), "No commands to execute for device: %s", deviceIP)
		doneSteps := 0
		if err := completeUnitExecution(handler, ctx, unit.ID, string(UnitStatusCompleted), doneSteps, "写入空命令 Unit 完成状态"); err != nil {
			return err
		}
		projectTaskexecLifecycleRecord(ctx, runtimeLogger, scope, recordNoCommands, "未配置可执行命令，直接完成", 0, 0)
		return nil
	}

	// Create device executor options
	execCtx := ctx.Context()
	opts := executor.ExecutorOptions{
		Vendor:     device.Vendor,
		LogSession: logSession,
		Protocol:   device.Protocol,
	}

	// Create device executor
	exec := executor.NewDeviceExecutor(
		device.IP,
		device.Port,
		device.Username,
		device.Password,
		opts,
	)
	// 确保连接关闭
	defer func() {
		if exec != nil {
			exec.Close()
			logger.Debug("TaskExec", ctx.RunID(), "关闭设备 %s 的连接", deviceIP)
		}
	}()

	// Get connection timeout from settings
	connTimeout := 30 * time.Second
	cmdTimeout := 60 * time.Second
	if e.settings != nil {
		if e.settings.ConnectTimeout != "" {
			if d, err := time.ParseDuration(e.settings.ConnectTimeout); err == nil {
				connTimeout = d
			}
		}
		if e.settings.CommandTimeout != "" {
			if d, err := time.ParseDuration(e.settings.CommandTimeout); err == nil {
				cmdTimeout = d
			}
		}
	}

	if ctx.IsCancelled() {
		return cancelUnitExecution(ctx, handler, unit.ID, "run cancelled before connect", intPtrLocal(0))
	}

	// Connect to device
	emitProjectedUnitEvent(ctx, stageID, unit.ID, EventTypeUnitStarted, EventLevelInfo, fmt.Sprintf("Connecting to %s...", deviceIP))
	projectTaskexecLifecycleRecord(ctx, runtimeLogger, scope, recordSessionConnecting, fmt.Sprintf("开始连接设备，共 %d 条命令", len(commands)), len(commands), 0)

	if err := exec.Connect(execCtx, connTimeout); err != nil {
		if IsContextCancelled(ctx, err) {
			return cancelUnitExecution(ctx, handler, unit.ID, "run cancelled during connect", intPtrLocal(0))
		}
		logger.Error("TaskExec", ctx.RunID(), "Failed to connect to %s: %v", deviceIP, err)
		errMsg := fmt.Sprintf("connection failed: %v", err)
		failUnitExecution(handler, ctx, unit.ID, errMsg, "写入连接失败状态", nil)
		projectTaskexecLifecycleRecord(ctx, runtimeLogger, scope, recordSessionConnectFailed, fmt.Sprintf("连接失败: %v", err), len(commands), 0)
		emitProjectedUnitEvent(ctx, stageID, unit.ID, EventTypeUnitFinished, EventLevelError, fmt.Sprintf("Connection failed: %v", err))
		return err
	}
	protocol := strings.ToUpper(device.Protocol)
	if protocol == "" {
		protocol = "SSH"
	}
	connMsg := fmt.Sprintf("%s 连接成功", protocol)
	projectTaskexecLifecycleRecord(ctx, runtimeLogger, scope, recordSessionConnected, connMsg, len(commands), 0)

	if ctx.IsCancelled() {
		projectTaskexecLifecycleRecord(ctx, runtimeLogger, scope, recordExecutionCancelled, "执行前收到取消信号", len(commands), 0)
		return cancelUnitExecution(ctx, handler, unit.ID, "run cancelled before execute commands", intPtrLocal(0))
	}

	// Execute commands
	emitProjectedUnitEvent(ctx, stageID, unit.ID, EventTypeStepStarted, EventLevelInfo, fmt.Sprintf("Executing %d commands...", len(commands)))
	projectTaskexecLifecycleRecord(ctx, runtimeLogger, scope, recordExecutionStarting, fmt.Sprintf("开始执行命令，命令总数: %d", len(commands)), len(commands), 0)

	commandEventHandler := func(event executor.ExecutionEvent) {
		projectExecutorRecord(ctx, handler, runtimeLogger, scope, stageID, unit.ID, len(commands), event, reportProgress, executorRecordProjectionOptions{
			CommandNoun: "命令",
		})
	}

	report, err := exec.ExecutePlaybookWithReport(execCtx, commands, cmdTimeout, commandEventHandler)

	if err != nil {
		if IsContextCancelled(ctx, err) {
			projectTaskexecLifecycleRecord(ctx, runtimeLogger, scope, recordExecutionCancelled, "命令执行过程中收到取消信号", len(commands), 0)
			return cancelUnitExecution(ctx, handler, unit.ID, "run cancelled during command execution", intPtrLocal(len(commands)))
		}
		// 连接级错误，整个 Unit 失败
		logger.Error("TaskExec", ctx.RunID(), "Failed to execute commands on %s: %v", deviceIP, err)
		errMsg := fmt.Sprintf("command execution failed: %v", err)
		failUnitExecution(handler, ctx, unit.ID, errMsg, "写入命令执行失败状态", intPtrLocal(len(commands)))
		projectTaskexecLifecycleRecord(ctx, runtimeLogger, scope, recordExecutionFailed, fmt.Sprintf("命令执行失败: %v", err), len(commands), 0)
		emitProjectedUnitEvent(ctx, stageID, unit.ID, EventTypeUnitFinished, EventLevelError, fmt.Sprintf("Command execution failed: %v", err))
		return err
	}

	// 根据 report 计算 Unit 状态
	var unitStatus string
	var doneSteps int
	var successCount, failureCount int

	if report != nil {
		doneSteps = len(report.Results)
		successCount = report.SuccessCount()
		failureCount = report.FailureCount()

		if failureCount == 0 {
			unitStatus = string(UnitStatusCompleted)
		} else if successCount == 0 {
			unitStatus = string(UnitStatusFailed)
		} else {
			unitStatus = string(UnitStatusPartial)
		}
	} else {
		unitStatus = string(UnitStatusFailed)
		doneSteps = len(commands)
	}

	// 根据状态选择日志级别
	eventLevel := EventLevelInfo
	lifecycleRecord := recordExecutionSucceeded
	if unitStatus == string(UnitStatusFailed) {
		eventLevel = EventLevelError
		lifecycleRecord = recordExecutionFailed
	} else if unitStatus == string(UnitStatusPartial) {
		eventLevel = EventLevelWarn
	}

	if err := completeUnitExecution(handler, ctx, unit.ID, unitStatus, doneSteps, "写入命令执行状态"); err != nil {
		return err
	}

	if reportProgress != nil {
		reportProgress(doneSteps, len(commands))
	}

	projectTaskexecLifecycleRecord(ctx, runtimeLogger, scope, lifecycleRecord,
		fmt.Sprintf("设备执行完成: status=%s, success=%d, failed=%d", unitStatus, successCount, failureCount), len(commands), successCount)
	emitProjectedUnitEvent(ctx, stageID, unit.ID, EventTypeUnitFinished, eventLevel,
		fmt.Sprintf("Executed %d commands on %s: %d success, %d failed", len(commands), deviceIP, successCount, failureCount))

	logger.Debug("TaskExec", ctx.RunID(), "Unit completed for device: %s, status=%s", deviceIP, unitStatus)
	return nil
}

// DeviceCollectExecutor for topology collection
type DeviceCollectExecutor struct {
	repo     repository.DeviceRepository
	settings *models.GlobalSettings
	db       *gorm.DB
}

// NewDeviceCollectExecutor creates executor
func NewDeviceCollectExecutor(repo repository.DeviceRepository) *DeviceCollectExecutor {
	settings, _, _ := config.LoadSettings()
	return &DeviceCollectExecutor{repo: repo, settings: settings, db: config.DB}
}

type topologyCollectOptions struct {
	TaskVendor     string
	FieldOverrides []models.TopologyTaskFieldOverride
}

func parseTopologyCollectOptions(unit *UnitPlan) (*topologyCollectOptions, error) {
	result := &topologyCollectOptions{}
	if unit == nil || len(unit.Steps) == 0 {
		return result, nil
	}
	params := unit.Steps[0].Params
	if len(params) == 0 {
		return result, nil
	}
	result.TaskVendor = strings.TrimSpace(params["taskVendor"])
	overridesJSON := strings.TrimSpace(params["fieldOverrides"])
	if overridesJSON == "" {
		return result, nil
	}
	if err := json.Unmarshal([]byte(overridesJSON), &result.FieldOverrides); err != nil {
		return nil, err
	}
	return result, nil
}

// Kind returns executor type
func (e *DeviceCollectExecutor) Kind() string {
	return string(StageKindDeviceCollect)
}

// Run executes the stage
func (e *DeviceCollectExecutor) Run(ctx RuntimeContext, stage *StagePlan) error {
	logger.Info("TaskExec", ctx.RunID(), "Start device collect stage: %s, units: %d", stage.Name, len(stage.Units))

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

	// 输出路径在 executeCollect 内按 PathManager 统一解析

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
				if u.Target.Key != "" {
					e.updateRunDeviceStatus(ctx.RunID(), u.Target.Key, "cancelled", "run cancelled before collect unit start")
				}
				mu.Lock()
				cancelledCount++
				mu.Unlock()
				applyProjectedStageProgress(handler, ctx, stage.ID, len(stage.Units), completedCount, failedCount, cancelledCount, stageProgressFromCounts(len(stage.Units), completedCount, failedCount, cancelledCount), "更新采集阶段进度")
				return
			}

			err := e.executeCollect(ctx, stage.ID, &u)

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
				emitProjectedUnitEvent(ctx, stage.ID, u.ID, EventTypeUnitFinished, EventLevelWarn, fmt.Sprintf("Collection cancelled for %s", u.Target.Key))
			} else if err != nil {
				emitProjectedUnitEvent(ctx, stage.ID, u.ID, EventTypeUnitFinished, EventLevelError, fmt.Sprintf("Collection failed for %s: %v", u.Target.Key, err))
			} else {
				emitProjectedUnitEvent(ctx, stage.ID, u.ID, EventTypeUnitFinished, EventLevelInfo, fmt.Sprintf("Collection completed for %s", u.Target.Key))
			}

			applyProjectedStageProgress(handler, ctx, stage.ID, len(stage.Units), localCompleted, localFailed, localCancelled, stageProgressFromCounts(len(stage.Units), localCompleted, localFailed, localCancelled), "更新采集阶段进度")
		}(unit)
	}

	wg.Wait()
	logger.Info("TaskExec", ctx.RunID(), "Collection stage completed: success=%d, failed=%d, cancelled=%d", completedCount, failedCount, cancelledCount)
	if ctx.IsCancelled() {
		return ctx.Context().Err()
	}
	return firstErr
}

func (e *DeviceCollectExecutor) executeCollect(ctx RuntimeContext, stageID string, unit *UnitPlan) error {
	handler := NewErrorHandler(ctx.RunID())
	pm := config.GetPathManager()
	normalizedRoot := filepath.Join(pm.GetStorageRoot(), "topology", "normalized")
	if ctx.IsCancelled() {
		if unit.Target.Key != "" {
			e.updateRunDeviceStatus(ctx.RunID(), unit.Target.Key, "cancelled", "run cancelled before collect unit start")
		}
		return cancelUnitExecution(ctx, handler, unit.ID, "run cancelled before collect unit start", intPtrLocal(0))
	}

	if err := markUnitRunning(handler, ctx, unit.ID, "设置采集 Unit 为 running"); err != nil {
		return err
	}

	// Get device info from unit target
	if unit.Target.Type != "device_ip" {
		errMsg := fmt.Sprintf("unsupported target type: %s", unit.Target.Type)
		failUnitExecution(handler, ctx, unit.ID, errMsg, "写入采集 Unit 失败状态", nil)
		return fmt.Errorf("%s", errMsg)
	}

	deviceIP := unit.Target.Key
	scope := LogScope{RunID: ctx.RunID(), StageID: stageID, UnitID: unit.ID, UnitKey: deviceIP}
	runtimeLogger := ctx.Logger(scope)
	logSession := runtimeLogger.Session(scope)
	logger.Debug("TaskExec", ctx.RunID(), "Collecting from device: %s", deviceIP)

	if ctx.IsCancelled() {
		e.updateRunDeviceStatus(ctx.RunID(), deviceIP, "cancelled", "run cancelled before loading device")
		return cancelUnitExecution(ctx, handler, unit.ID, "run cancelled before loading device", intPtrLocal(0))
	}

	// Get device from repository
	device, err := e.repo.FindByIP(deviceIP)
	if err != nil {
		errMsg := fmt.Sprintf("device not found: %v", err)
		failUnitExecution(handler, ctx, unit.ID, errMsg, "写入采集设备不存在状态", nil)
		projectTaskexecLifecycleRecord(ctx, runtimeLogger, scope, recordDeviceMissing, fmt.Sprintf("采集设备不存在: %v", err), 0, 0)
		return fmt.Errorf("device not found: %w", err)
	}
	if err := e.ensureRunDevice(ctx.RunID(), device); err != nil {
		errMsg := fmt.Sprintf("ensure run device failed: %v", err)
		failUnitExecution(handler, ctx, unit.ID, errMsg, "创建运行设备记录失败", nil)
		projectTaskexecLifecycleRecord(ctx, runtimeLogger, scope, recordExecutionFailed, errMsg, 0, 0)
		return err
	}

	collectOptions, err := parseTopologyCollectOptions(unit)
	if err != nil {
		errMsg := fmt.Sprintf("parse topology collect options failed: %v", err)
		failUnitExecution(handler, ctx, unit.ID, errMsg, "解析拓扑采集配置失败", nil)
		e.updateRunDeviceStatus(ctx.RunID(), deviceIP, "failed", errMsg)
		projectTaskexecLifecycleRecord(ctx, runtimeLogger, scope, recordExecutionFailed, errMsg, 0, 0)
		return fmt.Errorf("parse topology collect options failed: %w", err)
	}
	resolver := NewTopologyCommandResolver()
	resolution, err := resolver.Resolve(collectOptions.TaskVendor, device, collectOptions.FieldOverrides)
	if err != nil {
		errMsg := fmt.Sprintf("resolve topology commands failed: %v", err)
		failUnitExecution(handler, ctx, unit.ID, errMsg, "解析拓扑命令计划失败", nil)
		e.updateRunDeviceStatus(ctx.RunID(), deviceIP, "failed", errMsg)
		projectTaskexecLifecycleRecord(ctx, runtimeLogger, scope, recordExecutionFailed, errMsg, 0, 0)
		return fmt.Errorf("resolve topology commands failed: %w", err)
	}
	profile, ok := config.GetDeviceProfileByVendor(resolution.ResolvedVendor)
	if !ok || profile == nil {
		errMsg := fmt.Sprintf("no profile found for vendor: %s", resolution.ResolvedVendor)
		failUnitExecution(handler, ctx, unit.ID, errMsg, "写入采集 Profile 缺失状态", nil)
		e.updateRunDeviceStatus(ctx.RunID(), deviceIP, "failed", errMsg)
		projectTaskexecLifecycleRecord(ctx, runtimeLogger, scope, recordExecutionFailed, errMsg, 0, 0)
		return fmt.Errorf("no profile found for vendor: %s", resolution.ResolvedVendor)
	}
	e.updateRunDeviceCollectionContext(ctx.RunID(), deviceIP, resolution.ResolvedVendor, resolution.VendorSource)
	logger.Verbose("TaskExec", ctx.RunID(), "拓扑采集设备画像解析: device=%s, inventoryVendor=%s, taskVendor=%s, vendorSource=%s, resolvedVendor=%s, overrides=%d", deviceIP, strings.TrimSpace(device.Vendor), collectOptions.TaskVendor, resolution.VendorSource, resolution.ResolvedVendor, len(collectOptions.FieldOverrides))

	// Create device executor options
	execCtx := ctx.Context()
	opts := executor.ExecutorOptions{
		Vendor:        profile.Vendor,
		DeviceProfile: profile,
		LogSession:    logSession,
		Protocol:      device.Protocol,
	}

	// Create device executor
	exec := executor.NewDeviceExecutor(
		device.IP,
		device.Port,
		device.Username,
		device.Password,
		opts,
	)
	// 确保连接关闭
	defer func() {
		if exec != nil {
			exec.Close()
			logger.Debug("TaskExec", ctx.RunID(), "关闭设备 %s 的连接", deviceIP)
		}
	}()

	// Get timeout settings
	connTimeout := 30 * time.Second
	cmdTimeout := 60 * time.Second
	if e.settings != nil {
		if e.settings.ConnectTimeout != "" {
			if d, err := time.ParseDuration(e.settings.ConnectTimeout); err == nil {
				connTimeout = d
			}
		}
		if e.settings.CommandTimeout != "" {
			if d, err := time.ParseDuration(e.settings.CommandTimeout); err == nil {
				cmdTimeout = d
			}
		}
	}

	if ctx.IsCancelled() {
		e.updateRunDeviceStatus(ctx.RunID(), deviceIP, "cancelled", "run cancelled before connect")
		projectTaskexecLifecycleRecord(ctx, runtimeLogger, scope, recordExecutionCancelled, "采集前收到取消信号", 0, 0)
		return cancelUnitExecution(ctx, handler, unit.ID, "run cancelled before connect", intPtrLocal(0))
	}

	// Connect to device
	projectTaskexecLifecycleRecord(ctx, runtimeLogger, scope, recordSessionConnecting, "开始建立采集连接", 0, 0)
	if err := exec.Connect(execCtx, connTimeout); err != nil {
		if IsContextCancelled(ctx, err) {
			e.updateRunDeviceStatus(ctx.RunID(), deviceIP, "cancelled", "run cancelled during connect")
			return cancelUnitExecution(ctx, handler, unit.ID, "run cancelled during connect", intPtrLocal(0))
		}
		errMsg := fmt.Sprintf("connection failed: %v", err)
		failUnitExecution(handler, ctx, unit.ID, errMsg, "写入采集连接失败状态", nil)
		e.updateRunDeviceStatus(ctx.RunID(), deviceIP, "failed", errMsg)
		projectTaskexecLifecycleRecord(ctx, runtimeLogger, scope, recordSessionConnectFailed, fmt.Sprintf("采集连接失败: %v", err), 0, 0)
		return fmt.Errorf("connection failed: %w", err)
	}
	projectTaskexecLifecycleRecord(ctx, runtimeLogger, scope, recordSessionConnected, "采集连接成功", 0, 0)

	// Build command plan
	commands := make([]executor.PlannedCommand, 0, len(resolution.Commands))
	commandKeys := make([]string, 0, len(resolution.Commands))
	resolvedCommandMap := make(map[string]ResolvedTopologyCommand, len(resolution.Commands))
	for _, cmd := range resolution.Commands {
		if !cmd.Enabled || strings.TrimSpace(cmd.Command) == "" {
			continue
		}
		timeout := time.Duration(cmd.TimeoutSec) * time.Second
		if timeout == 0 {
			timeout = cmdTimeout
		}
		commandKeys = append(commandKeys, cmd.FieldKey)
		resolvedCommandMap[cmd.FieldKey] = cmd
		commands = append(commands, executor.PlannedCommand{
			Key:             cmd.FieldKey,
			Command:         cmd.Command,
			Timeout:         timeout,
			ContinueOnError: true,
		})
	}
	logger.Verbose("TaskExec", ctx.RunID(), "拓扑采集命令计划: device=%s, vendor=%s, commandCount=%d, commandKeys=%v", deviceIP, resolution.ResolvedVendor, len(commands), commandKeys)

	if len(commands) == 0 {
		errMsg := fmt.Sprintf("no commands defined for vendor: %s", resolution.ResolvedVendor)
		failUnitExecution(handler, ctx, unit.ID, errMsg, "写入采集命令为空状态", nil)
		e.updateRunDeviceStatus(ctx.RunID(), deviceIP, "failed", errMsg)
		projectTaskexecLifecycleRecord(ctx, runtimeLogger, scope, recordNoCommands, errMsg, 0, 0)
		return fmt.Errorf("no commands defined for vendor: %s", resolution.ResolvedVendor)
	}

	if ctx.IsCancelled() {
		e.updateRunDeviceStatus(ctx.RunID(), deviceIP, "cancelled", "run cancelled before execute plan")
		projectTaskexecLifecycleRecord(ctx, runtimeLogger, scope, recordExecutionCancelled, "采集执行前收到取消信号", len(commands), 0)
		return cancelUnitExecution(ctx, handler, unit.ID, "run cancelled before execute plan", intPtrLocal(0))
	}

	// Execute plan
	plan := executor.ExecutionPlan{
		Name:               fmt.Sprintf("topology_collect_%s", deviceIP),
		Commands:           commands,
		ContinueOnCmdError: true,
		Mode:               executor.PlanModeDiscovery,
	}

	projectTaskexecLifecycleRecord(ctx, runtimeLogger, scope, recordExecutionStarting, fmt.Sprintf("开始采集命令，总数: %d", len(commands)), len(commands), 0)
	commandEventHandler := func(event executor.ExecutionEvent) {
		projectExecutorRecord(ctx, handler, runtimeLogger, scope, stageID, unit.ID, len(commands), event, nil, executorRecordProjectionOptions{
			CommandNoun: "采集命令",
		})
	}

	report, err := exec.ExecutePlanWithEvents(execCtx, plan, commandEventHandler)
	if err != nil {
		if IsContextCancelled(ctx, err) {
			e.updateRunDeviceStatus(ctx.RunID(), deviceIP, "cancelled", "run cancelled during execute plan")
			projectTaskexecLifecycleRecord(ctx, runtimeLogger, scope, recordExecutionCancelled, "采集执行过程中收到取消信号", len(commands), 0)
			return cancelUnitExecution(ctx, handler, unit.ID, "run cancelled during execute plan", intPtrLocal(0))
		}
		errMsg := fmt.Sprintf("execution failed: %v", err)
		failUnitExecution(handler, ctx, unit.ID, errMsg, "写入采集执行失败状态", nil)
		e.updateRunDeviceStatus(ctx.RunID(), deviceIP, "failed", errMsg)
		projectTaskexecLifecycleRecord(ctx, runtimeLogger, scope, recordExecutionFailed, fmt.Sprintf("采集执行失败: %v", err), len(commands), 0)
		return fmt.Errorf("execution failed: %w", err)
	}

	if ctx.IsCancelled() {
		e.updateRunDeviceStatus(ctx.RunID(), deviceIP, "cancelled", "run cancelled before persisting outputs")
		return cancelUnitExecution(ctx, handler, unit.ID, "run cancelled before persisting outputs", intPtrLocal(0))
	}

	// Save outputs
	taskID := ctx.RunID()
	successCount := 0
	for _, result := range report.Results {
		if ctx.IsCancelled() {
			e.updateRunDeviceStatus(ctx.RunID(), deviceIP, "cancelled", "run cancelled during output persistence")
			return cancelUnitExecution(ctx, handler, unit.ID, "run cancelled during output persistence", &successCount)
		}
		if result != nil && result.Success {
			successCount++
			// Save raw output
			rawPath := pm.GetDiscoveryRawFilePath(taskID, deviceIP, result.CommandKey+"_raw")
			if err := os.MkdirAll(filepath.Dir(rawPath), 0755); err == nil {
				if err := os.WriteFile(rawPath, []byte(result.RawText), 0644); err != nil {
					logger.Warn("TaskExec", ctx.RunID(), "Failed to save raw output for %s/%s: %v",
						deviceIP, result.CommandKey, err)
				}
			}

			// Save normalized output
			normalizedPath := ""
			if len(result.NormalizedLines) > 0 {
				normalizedText := ""
				for _, line := range result.NormalizedLines {
					normalizedText += line + "\n"
				}
				normalizedPath = pm.GetDiscoveryNormalizedFilePath(taskID, deviceIP, result.CommandKey)
				if err := os.MkdirAll(filepath.Dir(normalizedPath), 0755); err == nil {
					if err := os.WriteFile(normalizedPath, []byte(normalizedText), 0644); err != nil {
						logger.Warn("TaskExec", ctx.RunID(), "Failed to save normalized output for %s/%s: %v",
							deviceIP, result.CommandKey, err)
					}
				}
			}

			resolvedCommand := resolvedCommandMap[result.CommandKey]
			if err := e.createTaskRawOutput(ctx.RunID(), deviceIP, result, rawPath, normalizedPath, &resolvedCommand); err != nil {
				errMsg := fmt.Sprintf("create task raw output failed: %v", err)
				failUnitExecution(handler, ctx, unit.ID, errMsg, "写入采集原始输出失败状态", &successCount)
				e.updateRunDeviceStatus(ctx.RunID(), deviceIP, "failed", errMsg)
				return err
			}
			e.createArtifact(ctx.RunID(), stageID, unit.ID, string(ArtifactTypeRawOutput), fmt.Sprintf("%s:%s:raw", deviceIP, result.CommandKey), rawPath)
			if normalizedPath != "" {
				e.createArtifact(ctx.RunID(), stageID, unit.ID, string(ArtifactTypeNormalizedOutput), fmt.Sprintf("%s:%s:normalized", deviceIP, result.CommandKey), normalizedPath)
			}
		}
	}

	status := string(UnitStatusCompleted)
	deviceStatus := "success"
	if successCount == 0 {
		status = string(UnitStatusFailed)
		deviceStatus = "failed"
	} else if successCount < len(report.Results) {
		status = string(UnitStatusPartial)
		deviceStatus = "partial"
	}
	doneSteps := successCount
	if err := completeUnitExecution(handler, ctx, unit.ID, status, doneSteps, "写入采集完成状态"); err != nil {
		return err
	}
	e.updateRunDeviceStatus(ctx.RunID(), deviceIP, deviceStatus, "")
	projectTaskexecLifecycleRecord(ctx, runtimeLogger, scope, recordExecutionSucceeded, fmt.Sprintf("采集完成: status=%s, success=%d/%d", status, successCount, len(report.Results)), len(report.Results), successCount)

	planPath := filepath.Join(normalizedRoot, taskID, deviceIP, "topology_collection_plan.json")
	if err := e.persistCollectionPlanArtifact(taskID, stageID, unit.ID, deviceIP, resolution, commandKeys, planPath); err != nil {
		logger.Warn("TaskExec", ctx.RunID(), "persist collection plan artifact failed: device=%s err=%v", deviceIP, err)
	}

	logger.Debug("TaskExec", ctx.RunID(), "Collection completed for device: %s", deviceIP)
	return nil
}

// ParseExecutor for parsing collected data
type ParseExecutor struct {
	db             *gorm.DB
	parserProvider parser.ParserProvider
}

// NewParseExecutor creates executor
func NewParseExecutor(db *gorm.DB, provider parser.ParserProvider) *ParseExecutor {
	return &ParseExecutor{db: db, parserProvider: provider}
}

// Kind returns executor type
func (e *ParseExecutor) Kind() string {
	return string(StageKindParse)
}

// Run executes the stage
func (e *ParseExecutor) Run(ctx RuntimeContext, stage *StagePlan) error {
	logger.Info("TaskExec", ctx.RunID(), "Start parse stage: %s, units: %d", stage.Name, len(stage.Units))

	handler := NewErrorHandler(ctx.RunID())
	var wg sync.WaitGroup
	var completedCount, failedCount, cancelledCount int
	var firstErr error
	var mu sync.Mutex

	for _, unit := range stage.Units {
		if ctx.IsCancelled() {
			break
		}

		wg.Add(1)
		go func(u UnitPlan) {
			defer wg.Done()

			if ctx.IsCancelled() {
				handler.MarkUnitCancelled(ctx, u.ID, "run cancelled before parse unit start", intPtrLocal(0))
				mu.Lock()
				cancelledCount++
				mu.Unlock()
				applyProjectedStageProgress(handler, ctx, stage.ID, len(stage.Units), completedCount, failedCount, cancelledCount, stageProgressFromCounts(len(stage.Units), completedCount, failedCount, cancelledCount), "更新解析阶段进度")
				return
			}

			err := e.executeParse(ctx, stage.ID, &u)

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
				emitProjectedUnitEvent(ctx, stage.ID, u.ID, EventTypeUnitFinished, EventLevelWarn, fmt.Sprintf("Parse cancelled for %s", u.Target.Key))
			} else if err != nil {
				emitProjectedUnitEvent(ctx, stage.ID, u.ID, EventTypeUnitFinished, EventLevelError, fmt.Sprintf("Parse failed for %s: %v", u.Target.Key, err))
			} else {
				emitProjectedUnitEvent(ctx, stage.ID, u.ID, EventTypeUnitFinished, EventLevelInfo, fmt.Sprintf("Parse completed for %s", u.Target.Key))
			}

			applyProjectedStageProgress(handler, ctx, stage.ID, len(stage.Units), localCompleted, localFailed, localCancelled, stageProgressFromCounts(len(stage.Units), localCompleted, localFailed, localCancelled), "更新解析阶段进度")
		}(unit)
	}

	wg.Wait()
	logger.Info("TaskExec", ctx.RunID(), "Parse stage completed: success=%d, failed=%d, cancelled=%d", completedCount, failedCount, cancelledCount)
	if ctx.IsCancelled() {
		return ctx.Context().Err()
	}
	return firstErr
}

func (e *ParseExecutor) executeParse(ctx RuntimeContext, stageID string, unit *UnitPlan) error {
	handler := NewErrorHandler(ctx.RunID())
	if ctx.IsCancelled() {
		return cancelUnitExecution(ctx, handler, unit.ID, "run cancelled before parse unit start", intPtrLocal(0))
	}

	if err := markUnitRunning(handler, ctx, unit.ID, "设置解析 Unit 为 running"); err != nil {
		return err
	}

	// For parse stage, target is device_ip
	if unit.Target.Type != "device_ip" {
		errMsg := fmt.Sprintf("unsupported target type: %s", unit.Target.Type)
		failUnitExecution(handler, ctx, unit.ID, errMsg, "写入解析 Unit 失败状态", nil)
		return fmt.Errorf("%s", errMsg)
	}

	deviceIP := unit.Target.Key
	logger.Debug("TaskExec", ctx.RunID(), "Parsing device: %s", deviceIP)

	if ctx.IsCancelled() {
		return cancelUnitExecution(ctx, handler, unit.ID, "run cancelled before parse execution", intPtrLocal(0))
	}

	vendor := ""
	if len(unit.Steps) > 0 {
		vendor = strings.TrimSpace(unit.Steps[0].Params["resolvedVendor"])
		if vendor == "" {
			vendor = strings.TrimSpace(unit.Steps[0].Params["vendor"])
		}
	}
	if err := e.parseAndSaveRunDevice(ctx, deviceIP, vendor); err != nil {
		if IsContextCancelled(ctx, err) {
			return cancelUnitExecution(ctx, handler, unit.ID, "run cancelled during parse execution", intPtrLocal(0))
		}
		errMsg := fmt.Sprintf("parse failed: %v", err)
		doneSteps := 0
		failUnitExecution(handler, ctx, unit.ID, errMsg, "写入解析失败状态", &doneSteps)
		return fmt.Errorf("parse failed: %w", err)
	}

	doneSteps := 1
	if err := completeUnitExecution(handler, ctx, unit.ID, string(UnitStatusCompleted), doneSteps, "写入解析完成状态"); err != nil {
		return err
	}
	e.createArtifact(ctx.RunID(), stageID, unit.ID, string(ArtifactTypeParseResult), fmt.Sprintf("%s:parse_result", deviceIP), "")

	logger.Debug("TaskExec", ctx.RunID(), "Parse completed for device: %s", deviceIP)
	return nil
}

// TopologyBuildExecutor for building topology graph
type TopologyBuildExecutor struct {
	db *gorm.DB
}

// NewTopologyBuildExecutor creates executor
func NewTopologyBuildExecutor(db *gorm.DB) *TopologyBuildExecutor {
	return &TopologyBuildExecutor{db: db}
}

// Kind returns executor type
func (e *TopologyBuildExecutor) Kind() string {
	return string(StageKindTopologyBuild)
}

// Run executes the stage
func (e *TopologyBuildExecutor) Run(ctx RuntimeContext, stage *StagePlan) error {
	logger.Info("TaskExec", ctx.RunID(), "Start topology build stage")
	handler := NewErrorHandler(ctx.RunID())

	if len(stage.Units) == 0 {
		return fmt.Errorf("no units in build stage")
	}
	if ctx.IsCancelled() {
		return ctx.Context().Err()
	}

	// Emit build started event
	emitProjectedStageEvent(ctx, stage.ID, EventTypeStageStarted, EventLevelInfo, "Starting topology build...")

	var result *models.TopologyBuildResult
	var err error

	logger.Info("TaskExec", ctx.RunID(), "Using topology builder")
	output, buildErr := BuildTopologyWithNewLogic(e.db, ctx.RunID())
	if buildErr != nil {
		err = buildErr
	} else if output != nil {
		result = &models.TopologyBuildResult{
			TaskID:             ctx.RunID(),
			TotalEdges:         output.Statistics.TotalEdges,
			ConfirmedEdges:     output.Statistics.ConfirmedEdges,
			SemiConfirmedEdges: output.Statistics.SemiConfirmedEdges,
			InferredEdges:      output.Statistics.InferredEdges,
			ConflictEdges:      output.Statistics.ConflictEdges,
			BuildTime:          output.Statistics.BuildDuration,
			Errors:             output.Errors,
		}
	}
	if err != nil {
		if IsContextCancelled(ctx, err) {
			return err
		}
		logger.Error("TaskExec", ctx.RunID(), "Topology build failed: %v", err)

		emitProjectedStageEvent(ctx, stage.ID, EventTypeStageFinished, EventLevelError, fmt.Sprintf("Topology build failed: %v", err))

		return fmt.Errorf("topology build failed: %w", err)
	}

	// Get build statistics
	edgeCount := 0
	if result != nil {
		edgeCount = result.TotalEdges
	}

	// Emit completion event
	emitProjectedStageEvent(ctx, stage.ID, EventTypeStageFinished, EventLevelInfo, fmt.Sprintf("Topology build completed with %d edges", edgeCount))

	// Update stage status
	completedStatus := string(StageStatusCompleted)
	doneSteps := 1
	progress := 100
	now := time.Now()

	handler.UpdateStageBestEffort(ctx, stage.ID, &StagePatch{
		Status:         &completedStatus,
		Progress:       &progress,
		CompletedUnits: &doneSteps,
		SuccessUnits:   &doneSteps,
		FinishedAt:     &now,
	}, "写入拓扑构建完成状态")
	e.createArtifact(ctx.RunID(), stage.ID, stage.Units[0].ID, string(ArtifactTypeTopologyGraph), "topology_graph", "")

	logger.Info("TaskExec", ctx.RunID(), "Topology build completed with %d edges", edgeCount)
	return nil
}

func (e *DeviceCollectExecutor) ensureRunDevice(taskID string, device *models.DeviceAsset) error {
	now := time.Now()
	var record TaskRunDevice
	err := e.db.Where("task_run_id = ? AND device_ip = ?", taskID, device.IP).First(&record).Error
	if err == nil {
		return nil
	}
	if !IsNotFoundError(err) {
		return err
	}
	return e.db.Create(&TaskRunDevice{
		TaskRunID:   taskID,
		DeviceIP:    device.IP,
		DeviceID:    device.ID,
		Status:      "running",
		StartedAt:   &now,
		Vendor:      device.Vendor,
		DisplayName: device.DisplayName,
		Role:        device.Role,
		Site:        device.Site,
		CreatedAt:   now,
		UpdatedAt:   now,
	}).Error
}

func (e *DeviceCollectExecutor) updateRunDeviceStatus(taskID, deviceIP, status, errMsg string) {
	handler := NewErrorHandler(taskID)
	updates := map[string]interface{}{
		"status":        status,
		"error_message": errMsg,
		"updated_at":    time.Now(),
	}
	if status == "success" || status == "partial" || status == "failed" || status == "cancelled" {
		now := time.Now()
		updates["finished_at"] = now
	}
	handler.DBBestEffort(fmt.Sprintf("更新运行设备状态[%s][%s]", deviceIP, status), func() error {
		return e.db.Model(&TaskRunDevice{}).
			Where("task_run_id = ? AND device_ip = ?", taskID, deviceIP).
			Updates(updates).Error
	})
}

func (e *DeviceCollectExecutor) updateRunDeviceCollectionContext(taskID, deviceIP, vendor, vendorSource string) {
	handler := NewErrorHandler(taskID)
	handler.DBBestEffort(fmt.Sprintf("更新运行设备厂商上下文[%s][%s]", deviceIP, vendor), func() error {
		return e.db.Model(&TaskRunDevice{}).
			Where("task_run_id = ? AND device_ip = ?", taskID, deviceIP).
			Updates(map[string]interface{}{
				"vendor":        strings.TrimSpace(vendor),
				"vendor_source": strings.TrimSpace(vendorSource),
				"updated_at":    time.Now(),
			}).Error
	})
}

func (e *DeviceCollectExecutor) createTaskRawOutput(taskID, deviceIP string, result *executor.CommandResult, rawPath, normalizedPath string, resolved *ResolvedTopologyCommand) error {
	if result == nil {
		return nil
	}
	rawSize := int64(len(result.RawText))
	normalizedSize := int64(0)
	lineCount := 0
	if len(result.NormalizedLines) > 0 {
		for _, line := range result.NormalizedLines {
			normalizedSize += int64(len(line))
		}
		lineCount = len(result.NormalizedLines)
	}
	output := &TaskRawOutput{
		TaskRunID:      taskID,
		DeviceIP:       deviceIP,
		CommandKey:     result.CommandKey,
		Command:        result.Command,
		ParseFilePath:  normalizedPath,
		RawFilePath:    rawPath,
		Status:         "success",
		ParseStatus:    "pending",
		RawSize:        rawSize,
		NormalizedSize: normalizedSize,
		LineCount:      lineCount,
	}
	if resolved != nil {
		output.FieldEnabled = resolved.Enabled
		output.CommandSource = resolved.CommandSource
		output.ResolvedVendor = resolved.ResolvedVendor
		output.VendorSource = resolved.VendorSource
	}
	return e.db.Create(output).Error
}

func (e *DeviceCollectExecutor) persistCollectionPlanArtifact(taskRunID, stageID, unitID, deviceIP string, resolution *TopologyCommandResolution, commandKeys []string, planPath string) error {
	if strings.TrimSpace(planPath) == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(planPath), 0755); err != nil {
		return err
	}

	type collectionPlanItem struct {
		FieldKey       string `json:"fieldKey"`
		DisplayName    string `json:"displayName"`
		Enabled        bool   `json:"enabled"`
		Command        string `json:"command"`
		TimeoutSec     int    `json:"timeoutSec"`
		CommandSource  string `json:"commandSource"`
		ResolvedVendor string `json:"resolvedVendor"`
		VendorSource   string `json:"vendorSource"`
		ParserBinding  string `json:"parserBinding"`
		Required       bool   `json:"required"`
		Description    string `json:"description"`
	}
	type collectionPlanDoc struct {
		RunID           string               `json:"runId"`
		StageID         string               `json:"stageId"`
		UnitID          string               `json:"unitId"`
		DeviceIP        string               `json:"deviceIp"`
		ResolvedVendor  string               `json:"resolvedVendor"`
		VendorSource    string               `json:"vendorSource"`
		CollectedFields []string             `json:"collectedFields"`
		Commands        []collectionPlanItem `json:"commands"`
		GeneratedAt     time.Time            `json:"generatedAt"`
	}

	doc := collectionPlanDoc{
		RunID:           taskRunID,
		StageID:         stageID,
		UnitID:          unitID,
		DeviceIP:        strings.TrimSpace(deviceIP),
		CollectedFields: append([]string(nil), commandKeys...),
		GeneratedAt:     time.Now(),
		Commands:        make([]collectionPlanItem, 0),
	}
	if resolution != nil {
		doc.ResolvedVendor = strings.TrimSpace(resolution.ResolvedVendor)
		doc.VendorSource = strings.TrimSpace(resolution.VendorSource)
		for _, cmd := range resolution.Commands {
			doc.Commands = append(doc.Commands, collectionPlanItem{
				FieldKey:       strings.TrimSpace(cmd.FieldKey),
				DisplayName:    strings.TrimSpace(cmd.DisplayName),
				Enabled:        cmd.Enabled,
				Command:        strings.TrimSpace(cmd.Command),
				TimeoutSec:     cmd.TimeoutSec,
				CommandSource:  strings.TrimSpace(cmd.CommandSource),
				ResolvedVendor: strings.TrimSpace(cmd.ResolvedVendor),
				VendorSource:   strings.TrimSpace(cmd.VendorSource),
				ParserBinding:  strings.TrimSpace(cmd.ParserBinding),
				Required:       cmd.Required,
				Description:    strings.TrimSpace(cmd.Description),
			})
		}
	}

	payload, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(planPath, payload, 0644); err != nil {
		return err
	}
	e.createArtifact(taskRunID, stageID, unitID, string(ArtifactTypeTopologyCollectionPlan), fmt.Sprintf("%s:topology_collection_plan", strings.TrimSpace(deviceIP)), planPath)
	return nil
}

func (e *ParseExecutor) parseAndSaveRunDevice(ctx RuntimeContext, deviceIP, vendor string) error {
	handler := NewErrorHandler(ctx.RunID())
	runID := ctx.RunID()
	vendorSource := "unit_plan"
	if vendor == "" {
		var dev TaskRunDevice
		if err := e.db.Where("task_run_id = ? AND device_ip = ?", runID, deviceIP).First(&dev).Error; err == nil {
			vendor = strings.ToLower(strings.TrimSpace(dev.Vendor))
			vendorSource = "run_device"
		}
	}
	if vendor == "" {
		vendor = "huawei"
		vendorSource = "default_huawei"
	}
	logger.Verbose("TaskExec", runID, "开始解析运行设备: device=%s, vendor=%s, vendorSource=%s", deviceIP, vendor, vendorSource)

	// 获取厂商解析器
	parserEngine, err := e.parserProvider.GetParser(vendor)
	if err != nil {
		return fmt.Errorf("get parser for vendor %s failed: %w", vendor, err)
	}

	var outputs []TaskRawOutput
	if err := e.db.Where("task_run_id = ? AND device_ip = ? AND status = ?", runID, deviceIP, "success").
		Order("created_at ASC").Find(&outputs).Error; err != nil {
		return fmt.Errorf("load raw outputs failed: %w", err)
	}
	logger.Verbose("TaskExec", runID, "解析输入已加载: device=%s, vendor=%s, outputs=%d", deviceIP, vendor, len(outputs))
	if len(outputs) == 0 {
		logger.Warn("TaskExec", runID, "设备没有可解析的采集输出: device=%s, vendor=%s", deviceIP, vendor)
	}

	mapper := parser.GetMapper(vendor)
	identity := &parser.DeviceIdentity{Vendor: vendor, MgmtIP: deviceIP}
	var interfaces []parser.InterfaceFact
	var lldps []parser.LLDPFact
	var fdbs []parser.FDBFact
	var arps []parser.ARPFact
	var aggs []parser.AggregateFact

	for _, output := range outputs {
		if ctx.IsCancelled() {
			return ctx.Context().Err()
		}
		parsePath := strings.TrimSpace(output.ParseFilePath)
		if parsePath == "" {
			handler.LogDBErrorWithContext("更新 parse_status 为 skipped", e.db.Model(&TaskRawOutput{}).Where("id = ?", output.ID).
				Updates(map[string]interface{}{"parse_status": "skipped", "parse_error": "parse file path empty"}).Error,
				map[string]interface{}{"runID": runID, "deviceIP": deviceIP, "outputID": output.ID, "commandKey": output.CommandKey})
			continue
		}
		rawText, err := os.ReadFile(parsePath)
		if err != nil {
			handler.LogDBErrorWithContext("更新 parse_status 为 parse_failed", e.db.Model(&TaskRawOutput{}).Where("id = ?", output.ID).
				Updates(map[string]interface{}{"parse_status": "parse_failed", "parse_error": err.Error()}).Error,
				map[string]interface{}{"runID": runID, "deviceIP": deviceIP, "outputID": output.ID, "commandKey": output.CommandKey})
			continue
		}
		if ctx.IsCancelled() {
			return ctx.Context().Err()
		}
		rows, err := parserEngine.Parse(output.CommandKey, string(rawText))
		if err != nil {
			handler.LogDBErrorWithContext("更新 parse_status 为 parse_failed", e.db.Model(&TaskRawOutput{}).Where("id = ?", output.ID).
				Updates(map[string]interface{}{"parse_status": "parse_failed", "parse_error": err.Error()}).Error,
				map[string]interface{}{"runID": runID, "deviceIP": deviceIP, "outputID": output.ID, "commandKey": output.CommandKey})
			continue
		}

		parseStatus := "success"
		parseError := ""
		rawRefID := fmt.Sprintf("%d", output.ID)
		batch, mapErr := MapCommandOutput(mapper, output.CommandKey, rows, identity, rawRefID)
		if mapErr != nil {
			parseStatus = "parse_failed"
			parseError = mapErr.Error()
		} else if batch != nil {
			interfaces = append(interfaces, batch.Interfaces...)
			lldps = append(lldps, batch.LLDPs...)
			fdbs = append(fdbs, batch.FDBs...)
			arps = append(arps, batch.ARPs...)
			aggs = append(aggs, batch.Aggregates...)
		}
		handler.LogDBErrorWithContext("更新 parse_status", e.db.Model(&TaskRawOutput{}).Where("id = ?", output.ID).
			Updates(map[string]interface{}{"parse_status": parseStatus, "parse_error": parseError}).Error,
			map[string]interface{}{"runID": runID, "deviceIP": deviceIP, "outputID": output.ID, "commandKey": output.CommandKey, "parseStatus": parseStatus})
		logger.Verbose("TaskExec", runID, "解析命令统计: device=%s, cmd=%s, rows=%d, status=%s, interfaces=%d, lldps=%d, fdbs=%d, arps=%d, aggs=%d", deviceIP, output.CommandKey, len(rows), parseStatus, len(interfaces), len(lldps), len(fdbs), len(arps), len(aggs))
		if parseStatus != "success" {
			logger.Warn("TaskExec", runID, "解析命令失败: device=%s, cmd=%s, rows=%d, error=%s", deviceIP, output.CommandKey, len(rows), parseError)
		}
	}

	if ctx.IsCancelled() {
		return ctx.Context().Err()
	}

	NormalizeIdentity(identity)

	logger.Verbose("TaskExec", runID, "解析汇总: device=%s, vendor=%s, hostname=%s, mgmtIP=%s, interfaces=%d, lldps=%d, fdbs=%d, arps=%d, aggs=%d", deviceIP, identity.Vendor, identity.Hostname, identity.MgmtIP, len(interfaces), len(lldps), len(fdbs), len(arps), len(aggs))
	if len(lldps) == 0 {
		logger.Warn("TaskExec", runID, "设备未解析出任何 LLDP 邻居: device=%s, vendor=%s, outputs=%d", deviceIP, identity.Vendor, len(outputs))
	}

	if ctx.IsCancelled() {
		return ctx.Context().Err()
	}

	persister := NewTopologyFactsPersister(e.db)
	if err := persister.SaveDeviceIdentity(runID, deviceIP, identity); err != nil {
		logger.Warn("TaskExec", runID, "保存设备身份失败: device=%s, err=%v", deviceIP, err)
		return err
	}
	if err := persister.SaveParsedFacts(runID, deviceIP, interfaces, lldps, fdbs, arps, aggs); err != nil {
		logger.Warn("TaskExec", runID, "持久化解析结果失败: device=%s, err=%v", deviceIP, err)
		return err
	}
	logger.Verbose("TaskExec", runID, "解析结果已持久化: device=%s, interfaces=%d, lldps=%d, fdbs=%d, arps=%d, aggs=%d", deviceIP, len(interfaces), len(lldps), len(fdbs), len(arps), len(aggs))
	return nil
}

func normalizeMACAddress(mac string) string {
	mac = strings.ToLower(strings.TrimSpace(mac))
	if mac == "" {
		return ""
	}
	replacer := strings.NewReplacer("-", "", ":", "", ".", "")
	return replacer.Replace(mac)
}

func mergeIdentityResult(identity *parser.DeviceIdentity, incoming *parser.DeviceIdentity, fallbackVendor string) {
	if incoming == nil {
		return
	}
	mergeIdentityFields(identity, map[string]string{
		"vendor":        incoming.Vendor,
		"model":         incoming.Model,
		"hostname":      incoming.Hostname,
		"mgmt_ip":       incoming.MgmtIP,
		"chassis_id":    incoming.ChassisID,
	}, fallbackVendor)
}

func flattenParseRows(rows []map[string]string) map[string]string {
	data := make(map[string]string)
	for _, row := range rows {
		for key, value := range row {
			trimmed := strings.TrimSpace(value)
			if trimmed == "" {
				continue
			}
			data[key] = trimmed
		}
	}
	return data
}

func mergeIdentityFields(identity *parser.DeviceIdentity, fields map[string]string, fallbackVendor string) {
	if identity == nil {
		return
	}
	if v := strings.TrimSpace(fields["vendor"]); v != "" {
		identity.Vendor = v
	} else if strings.TrimSpace(identity.Vendor) == "" && strings.TrimSpace(fallbackVendor) != "" {
		identity.Vendor = fallbackVendor
	}
	if v := strings.TrimSpace(fields["model"]); v != "" {
		identity.Model = v
	}
	if v := chooseValue(fields["hostname"], fields["sysname"]); strings.TrimSpace(v) != "" {
		identity.Hostname = strings.TrimSpace(v)
	}
	if v := chooseValue(fields["mgmt_ip"], fields["management_ip"], fields["ip"]); strings.TrimSpace(v) != "" {
		identity.MgmtIP = strings.TrimSpace(v)
	}
	if v := strings.TrimSpace(fields["chassis_id"]); v != "" {
		identity.ChassisID = v
	}
}

func appendUniqueStrings(base []string, values ...string) []string {
	seen := make(map[string]struct{}, len(base))
	for _, v := range base {
		seen[v] = struct{}{}
	}
	for _, v := range values {
		if strings.TrimSpace(v) == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		base = append(base, v)
	}
	return base
}

func stageProgressFromCounts(totalUnits, completedCount, failedCount, cancelledCount int) int {
	finished := completedCount + failedCount + cancelledCount
	progress := 100
	if totalUnits > 0 {
		progress = finished * 100 / totalUnits
	}
	return progress
}

func aggregateUnitProgress(unitProgress map[string]int, totalUnits int) int {
	if totalUnits <= 0 || len(unitProgress) == 0 {
		return 0
	}

	total := 0
	for _, progress := range unitProgress {
		total += progress
	}
	return total / totalUnits
}

func unitProgressPercent(doneSteps, totalSteps int) int {
	if totalSteps <= 0 {
		if doneSteps > 0 {
			return 100
		}
		return 0
	}
	if doneSteps <= 0 {
		return 0
	}
	if doneSteps >= totalSteps {
		return 100
	}
	return doneSteps * 100 / totalSteps
}

func (e *DeviceCollectExecutor) createArtifact(taskRunID, stageID, unitID, artifactType, artifactKey, filePath string) {
	handler := NewErrorHandler(taskRunID)
	handler.ArtifactBestEffort(NewGormRepository(e.db), context.Background(), &TaskArtifact{
		ID:           newArtifactID(),
		TaskRunID:    taskRunID,
		StageID:      stageID,
		UnitID:       unitID,
		ArtifactType: artifactType,
		ArtifactKey:  artifactKey,
		FilePath:     filePath,
		CreatedAt:    time.Now(),
	})
}

func (e *ParseExecutor) createArtifact(taskRunID, stageID, unitID, artifactType, artifactKey, filePath string) {
	handler := NewErrorHandler(taskRunID)
	handler.ArtifactBestEffort(NewGormRepository(e.db), context.Background(), &TaskArtifact{
		ID:           newArtifactID(),
		TaskRunID:    taskRunID,
		StageID:      stageID,
		UnitID:       unitID,
		ArtifactType: artifactType,
		ArtifactKey:  artifactKey,
		FilePath:     filePath,
		CreatedAt:    time.Now(),
	})
}

func (e *TopologyBuildExecutor) createArtifact(taskRunID, stageID, unitID, artifactType, artifactKey, filePath string) {
	handler := NewErrorHandler(taskRunID)
	handler.ArtifactBestEffort(NewGormRepository(e.db), context.Background(), &TaskArtifact{
		ID:           newArtifactID(),
		TaskRunID:    taskRunID,
		StageID:      stageID,
		UnitID:       unitID,
		ArtifactType: artifactType,
		ArtifactKey:  artifactKey,
		FilePath:     filePath,
		CreatedAt:    time.Now(),
	})
}

func cancelUnitExecution(ctx RuntimeContext, handler *ErrorHandler, unitID, reason string, doneSteps *int) error {
	handler.MarkUnitCancelled(ctx, unitID, reason, doneSteps)
	return ctx.Context().Err()
}

func markUnitRunning(handler *ErrorHandler, ctx RuntimeContext, unitID, operation string) error {
	startedAt := time.Now()
	unitRunning := string(UnitStatusRunning)
	return handler.UpdateUnitRequired(ctx, unitID, &UnitPatch{
		Status:    &unitRunning,
		StartedAt: &startedAt,
	}, operation)
}

func failUnitExecution(handler *ErrorHandler, ctx RuntimeContext, unitID, errMsg, operation string, doneSteps *int) {
	finishedAt := time.Now()
	patch := &UnitPatch{
		Status:       strPtr(string(UnitStatusFailed)),
		ErrorMessage: &errMsg,
		FinishedAt:   &finishedAt,
	}
	if doneSteps != nil {
		patch.DoneSteps = doneSteps
	}
	_ = handler.UpdateUnitRequired(ctx, unitID, patch, operation)
}

func completeUnitExecution(handler *ErrorHandler, ctx RuntimeContext, unitID, status string, doneSteps int, operation string) error {
	finishedAt := time.Now()
	return handler.UpdateUnitRequired(ctx, unitID, &UnitPatch{
		Status:     &status,
		DoneSteps:  &doneSteps,
		FinishedAt: &finishedAt,
	}, operation)
}

func intPtrLocal(v int) *int {
	return &v
}
