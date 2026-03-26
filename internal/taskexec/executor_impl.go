package taskexec

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/executor"
	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
	"github.com/NetWeaverGo/core/internal/normalize"
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

	for _, unit := range stage.Units {
		if ctx.IsCancelled() {
			break
		}

		wg.Add(1)
		semaphore <- struct{}{}

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

				e.updateStageProgressDetailed(handler, ctx, stage.ID, len(stage.Units), localCompleted, localFailed, localCancelled, stageProgress)
			}

			if ctx.IsCancelled() {
				handler.MarkUnitCancelled(ctx, u.ID, "run cancelled before unit start", intPtrLocal(0))
				mu.Lock()
				cancelledCount++
				unitProgress[u.ID] = 0
				stageProgress := aggregateUnitProgress(unitProgress, len(stage.Units))
				mu.Unlock()
				e.updateStageProgressDetailed(handler, ctx, stage.ID, len(stage.Units), completedCount, failedCount, cancelledCount, stageProgress)
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

			e.updateStageProgressDetailed(handler, ctx, stage.ID, len(stage.Units), localCompleted, localFailed, localCancelled, stageProgress)
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
		handler.MarkUnitCancelled(ctx, unit.ID, "run cancelled before unit start", intPtrLocal(0))
		return ctx.Context().Err()
	}

	startedAt := time.Now()
	unitRunning := string(UnitStatusRunning)
	if err := handler.UpdateUnitRequired(ctx, unit.ID, &UnitPatch{
		Status:    &unitRunning,
		StartedAt: &startedAt,
	}, "设置命令执行 Unit 为 running"); err != nil {
		return err
	}

	if ctx.IsCancelled() {
		handler.MarkUnitCancelled(ctx, unit.ID, "run cancelled after unit start", intPtrLocal(0))
		return ctx.Context().Err()
	}

	// Get device info from unit target
	if unit.Target.Type != "device_ip" {
		errMsg := fmt.Sprintf("unsupported target type: %s", unit.Target.Type)
		finishedAt := time.Now()
		_ = handler.UpdateUnitRequired(ctx, unit.ID, &UnitPatch{
			Status:       strPtr(string(UnitStatusFailed)),
			ErrorMessage: &errMsg,
			FinishedAt:   &finishedAt,
		}, "写入命令执行 Unit 失败状态")
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
		finishedAt := time.Now()
		_ = handler.UpdateUnitRequired(ctx, unit.ID, &UnitPatch{
			Status:       strPtr(string(UnitStatusFailed)),
			ErrorMessage: &errMsg,
			FinishedAt:   &finishedAt,
		}, "写入设备不存在失败状态")
		ctx.Emit(&TaskEvent{
			RunID:   ctx.RunID(),
			StageID: stageID,
			UnitID:  unit.ID,
			Type:    EventTypeUnitFinished,
			Level:   EventLevelError,
			Message: fmt.Sprintf("Device not found: %s", deviceIP),
		})
		runtimeLogger.WriteSummary(scope, fmt.Sprintf("设备不存在: %s", deviceIP))
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
		unitCompleted := string(UnitStatusCompleted)
		finishedAt := time.Now()
		if err := handler.UpdateUnitRequired(ctx, unit.ID, &UnitPatch{
			Status:     &unitCompleted,
			DoneSteps:  &doneSteps,
			FinishedAt: &finishedAt,
		}, "写入空命令 Unit 完成状态"); err != nil {
			return err
		}
		runtimeLogger.WriteSummary(scope, "未配置可执行命令，直接完成")
		return nil
	}

	// Create device executor options
	execCtx := ctx.Context()
	opts := executor.ExecutorOptions{
		Vendor:     device.Vendor,
		LogSession: logSession,
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
		handler.MarkUnitCancelled(ctx, unit.ID, "run cancelled before connect", intPtrLocal(0))
		return ctx.Context().Err()
	}

	// Connect to device
	ctx.Emit(&TaskEvent{
		RunID:   ctx.RunID(),
		StageID: stageID,
		UnitID:  unit.ID,
		Type:    EventTypeUnitStarted,
		Level:   EventLevelInfo,
		Message: fmt.Sprintf("Connecting to %s...", deviceIP),
	})
	runtimeLogger.WriteSummary(scope, fmt.Sprintf("开始连接设备，共 %d 条命令", len(commands)))

	if err := exec.Connect(execCtx, connTimeout); err != nil {
		if IsContextCancelled(ctx, err) {
			handler.MarkUnitCancelled(ctx, unit.ID, "run cancelled during connect", intPtrLocal(0))
			return err
		}
		logger.Error("TaskExec", ctx.RunID(), "Failed to connect to %s: %v", deviceIP, err)
		errMsg := fmt.Sprintf("connection failed: %v", err)
		finishedAt := time.Now()
		_ = handler.UpdateUnitRequired(ctx, unit.ID, &UnitPatch{
			Status:       strPtr(string(UnitStatusFailed)),
			ErrorMessage: &errMsg,
			FinishedAt:   &finishedAt,
		}, "写入连接失败状态")
		ctx.Emit(&TaskEvent{
			RunID:   ctx.RunID(),
			StageID: stageID,
			UnitID:  unit.ID,
			Type:    EventTypeUnitFinished,
			Level:   EventLevelError,
			Message: fmt.Sprintf("Connection failed: %v", err),
		})
		runtimeLogger.WriteSummary(scope, fmt.Sprintf("连接失败: %v", err))
		return err
	}
	runtimeLogger.WriteSummary(scope, "SSH 连接成功")

	if ctx.IsCancelled() {
		handler.MarkUnitCancelled(ctx, unit.ID, "run cancelled before execute commands", intPtrLocal(0))
		runtimeLogger.WriteSummary(scope, "执行前收到取消信号")
		return ctx.Context().Err()
	}

	// Execute commands
	ctx.Emit(&TaskEvent{
		RunID:   ctx.RunID(),
		StageID: stageID,
		UnitID:  unit.ID,
		Type:    EventTypeStepStarted,
		Level:   EventLevelInfo,
		Message: fmt.Sprintf("Executing %d commands...", len(commands)),
	})
	runtimeLogger.WriteSummary(scope, fmt.Sprintf("开始执行命令，命令总数: %d", len(commands)))

	commandEventHandler := func(event executor.ExecutionEvent) {
		switch event.Type {
		case executor.EventCmdStart:
			runtimeLogger.WriteSummary(scope, fmt.Sprintf("命令[%d/%d]开始: %s", event.Index+1, len(commands), event.Command))
		case executor.EventCmdComplete:
			doneSteps := event.Index + 1
			logger.Verbose("TaskExec", ctx.RunID(), "设备 %s 命令进度更新: %d/%d, command=%s, err=%v", deviceIP, doneSteps, len(commands), event.Command, event.Error)
			handler.UpdateUnitBestEffort(ctx, unit.ID, &UnitPatch{
				DoneSteps: &doneSteps,
			}, "更新命令执行 Unit 进度")
			if reportProgress != nil {
				reportProgress(doneSteps, len(commands))
			}

			message := fmt.Sprintf("命令[%d/%d]完成: %s", doneSteps, len(commands), event.Command)
			level := EventLevelInfo
			if event.Error != nil {
				message = fmt.Sprintf("命令[%d/%d]失败: %s (%v)", doneSteps, len(commands), event.Command, event.Error)
				level = EventLevelWarn
			}
			runtimeLogger.WriteSummary(scope, message)
			ctx.Emit(NewTaskEvent(ctx.RunID(), EventTypeUnitProgress, message).
				WithStage(stageID).
				WithUnit(unit.ID).
				WithLevel(level).
				WithPayload("doneSteps", doneSteps).
				WithPayload("totalSteps", len(commands)).
				WithPayload("command", event.Command))
		}
	}

	if err := exec.ExecutePlaybookWithEvents(execCtx, commands, cmdTimeout, commandEventHandler); err != nil {
		if IsContextCancelled(ctx, err) {
			handler.MarkUnitCancelled(ctx, unit.ID, "run cancelled during command execution", intPtrLocal(len(commands)))
			runtimeLogger.WriteSummary(scope, "命令执行过程中收到取消信号")
			return err
		}
		logger.Error("TaskExec", ctx.RunID(), "Failed to execute commands on %s: %v", deviceIP, err)
		errMsg := fmt.Sprintf("command execution failed: %v", err)
		doneSteps := len(commands)
		finishedAt := time.Now()
		_ = handler.UpdateUnitRequired(ctx, unit.ID, &UnitPatch{
			Status:       strPtr(string(UnitStatusFailed)),
			DoneSteps:    &doneSteps,
			ErrorMessage: &errMsg,
			FinishedAt:   &finishedAt,
		}, "写入命令执行失败状态")
		ctx.Emit(&TaskEvent{
			RunID:   ctx.RunID(),
			StageID: stageID,
			UnitID:  unit.ID,
			Type:    EventTypeUnitFinished,
			Level:   EventLevelError,
			Message: fmt.Sprintf("Command execution failed: %v", err),
		})
		runtimeLogger.WriteSummary(scope, fmt.Sprintf("命令执行失败: %v", err))
		return err
	}

	doneSteps := len(commands)
	unitCompleted := string(UnitStatusCompleted)
	finishedAt := time.Now()
	if err := handler.UpdateUnitRequired(ctx, unit.ID, &UnitPatch{
		Status:     &unitCompleted,
		DoneSteps:  &doneSteps,
		FinishedAt: &finishedAt,
	}, "写入命令执行完成状态"); err != nil {
		return err
	}
	if reportProgress != nil {
		reportProgress(doneSteps, len(commands))
	}

	// Success
	ctx.Emit(&TaskEvent{
		RunID:   ctx.RunID(),
		StageID: stageID,
		UnitID:  unit.ID,
		Type:    EventTypeUnitFinished,
		Level:   EventLevelInfo,
		Message: fmt.Sprintf("Successfully executed %d commands on %s", len(commands), deviceIP),
	})
	runtimeLogger.WriteSummary(scope, fmt.Sprintf("设备执行完成，成功命令数: %d", len(commands)))

	logger.Debug("TaskExec", ctx.RunID(), "Unit completed for device: %s", deviceIP)
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

	// Get task context for output storage
	pm := config.GetPathManager()
	outputDir := pm.GetTopologyRawDir()

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
				e.updateStageProgress(handler, ctx, stage.ID, len(stage.Units), completedCount, failedCount, cancelledCount)
				return
			}

			err := e.executeCollect(ctx, stage.ID, &u, outputDir)

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
				ctx.Emit(&TaskEvent{
					RunID:   ctx.RunID(),
					StageID: stage.ID,
					UnitID:  u.ID,
					Type:    EventTypeUnitFinished,
					Level:   EventLevelWarn,
					Message: fmt.Sprintf("Collection cancelled for %s", u.Target.Key),
				})
			} else if err != nil {
				ctx.Emit(&TaskEvent{
					RunID:   ctx.RunID(),
					StageID: stage.ID,
					UnitID:  u.ID,
					Type:    EventTypeUnitFinished,
					Level:   EventLevelError,
					Message: fmt.Sprintf("Collection failed for %s: %v", u.Target.Key, err),
				})
			} else {
				ctx.Emit(&TaskEvent{
					RunID:   ctx.RunID(),
					StageID: stage.ID,
					UnitID:  u.ID,
					Type:    EventTypeUnitFinished,
					Level:   EventLevelInfo,
					Message: fmt.Sprintf("Collection completed for %s", u.Target.Key),
				})
			}

			e.updateStageProgress(handler, ctx, stage.ID, len(stage.Units), localCompleted, localFailed, localCancelled)
		}(unit)
	}

	wg.Wait()
	logger.Info("TaskExec", ctx.RunID(), "Collection stage completed: success=%d, failed=%d, cancelled=%d", completedCount, failedCount, cancelledCount)
	if ctx.IsCancelled() {
		return ctx.Context().Err()
	}
	return firstErr
}

func (e *DeviceCollectExecutor) executeCollect(ctx RuntimeContext, stageID string, unit *UnitPlan, outputDir string) error {
	handler := NewErrorHandler(ctx.RunID())
	if ctx.IsCancelled() {
		handler.MarkUnitCancelled(ctx, unit.ID, "run cancelled before collect unit start", intPtrLocal(0))
		if unit.Target.Key != "" {
			e.updateRunDeviceStatus(ctx.RunID(), unit.Target.Key, "cancelled", "run cancelled before collect unit start")
		}
		return ctx.Context().Err()
	}

	startedAt := time.Now()
	unitRunning := string(UnitStatusRunning)
	if err := handler.UpdateUnitRequired(ctx, unit.ID, &UnitPatch{
		Status:    &unitRunning,
		StartedAt: &startedAt,
	}, "设置采集 Unit 为 running"); err != nil {
		return err
	}

	// Get device info from unit target
	if unit.Target.Type != "device_ip" {
		errMsg := fmt.Sprintf("unsupported target type: %s", unit.Target.Type)
		finishedAt := time.Now()
		_ = handler.UpdateUnitRequired(ctx, unit.ID, &UnitPatch{
			Status:       strPtr(string(UnitStatusFailed)),
			ErrorMessage: &errMsg,
			FinishedAt:   &finishedAt,
		}, "写入采集 Unit 失败状态")
		return fmt.Errorf("%s", errMsg)
	}

	deviceIP := unit.Target.Key
	scope := LogScope{RunID: ctx.RunID(), StageID: stageID, UnitID: unit.ID, UnitKey: deviceIP}
	runtimeLogger := ctx.Logger(scope)
	logSession := runtimeLogger.Session(scope)
	logger.Debug("TaskExec", ctx.RunID(), "Collecting from device: %s", deviceIP)

	if ctx.IsCancelled() {
		handler.MarkUnitCancelled(ctx, unit.ID, "run cancelled before loading device", intPtrLocal(0))
		e.updateRunDeviceStatus(ctx.RunID(), deviceIP, "cancelled", "run cancelled before loading device")
		return ctx.Context().Err()
	}

	// Get device from repository
	device, err := e.repo.FindByIP(deviceIP)
	if err != nil {
		errMsg := fmt.Sprintf("device not found: %v", err)
		finishedAt := time.Now()
		_ = handler.UpdateUnitRequired(ctx, unit.ID, &UnitPatch{
			Status:       strPtr(string(UnitStatusFailed)),
			ErrorMessage: &errMsg,
			FinishedAt:   &finishedAt,
		}, "写入采集设备不存在状态")
		runtimeLogger.WriteSummary(scope, fmt.Sprintf("采集设备不存在: %v", err))
		return fmt.Errorf("device not found: %w", err)
	}
	if err := e.ensureRunDevice(ctx.RunID(), device); err != nil {
		errMsg := fmt.Sprintf("ensure run device failed: %v", err)
		finishedAt := time.Now()
		_ = handler.UpdateUnitRequired(ctx, unit.ID, &UnitPatch{
			Status:       strPtr(string(UnitStatusFailed)),
			ErrorMessage: &errMsg,
			FinishedAt:   &finishedAt,
		}, "创建运行设备记录失败")
		runtimeLogger.WriteSummary(scope, errMsg)
		return err
	}

	// Get vendor profile
	vendor := device.Vendor
	if vendor == "" {
		vendor = "generic"
	}

	// Create device executor options
	execCtx := ctx.Context()
	opts := executor.ExecutorOptions{
		Vendor:     vendor,
		LogSession: logSession,
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
		handler.MarkUnitCancelled(ctx, unit.ID, "run cancelled before connect", intPtrLocal(0))
		e.updateRunDeviceStatus(ctx.RunID(), deviceIP, "cancelled", "run cancelled before connect")
		runtimeLogger.WriteSummary(scope, "采集前收到取消信号")
		return ctx.Context().Err()
	}

	// Connect to device
	runtimeLogger.WriteSummary(scope, "开始建立采集连接")
	if err := exec.Connect(execCtx, connTimeout); err != nil {
		if IsContextCancelled(ctx, err) {
			handler.MarkUnitCancelled(ctx, unit.ID, "run cancelled during connect", intPtrLocal(0))
			e.updateRunDeviceStatus(ctx.RunID(), deviceIP, "cancelled", "run cancelled during connect")
			return err
		}
		errMsg := fmt.Sprintf("connection failed: %v", err)
		finishedAt := time.Now()
		_ = handler.UpdateUnitRequired(ctx, unit.ID, &UnitPatch{
			Status:       strPtr(string(UnitStatusFailed)),
			ErrorMessage: &errMsg,
			FinishedAt:   &finishedAt,
		}, "写入采集连接失败状态")
		e.updateRunDeviceStatus(ctx.RunID(), deviceIP, "failed", errMsg)
		runtimeLogger.WriteSummary(scope, fmt.Sprintf("采集连接失败: %v", err))
		return fmt.Errorf("connection failed: %w", err)
	}
	runtimeLogger.WriteSummary(scope, "采集连接成功")

	// Get commands from device profile
	profile := config.GetDeviceProfile(vendor)
	if profile == nil {
		errMsg := fmt.Sprintf("no profile found for vendor: %s", vendor)
		finishedAt := time.Now()
		_ = handler.UpdateUnitRequired(ctx, unit.ID, &UnitPatch{
			Status:       strPtr(string(UnitStatusFailed)),
			ErrorMessage: &errMsg,
			FinishedAt:   &finishedAt,
		}, "写入采集 Profile 缺失状态")
		e.updateRunDeviceStatus(ctx.RunID(), deviceIP, "failed", errMsg)
		runtimeLogger.WriteSummary(scope, errMsg)
		return fmt.Errorf("no profile found for vendor: %s", vendor)
	}

	// Build command plan
	commands := make([]executor.PlannedCommand, 0, len(profile.Commands))
	for _, cmd := range profile.Commands {
		timeout := time.Duration(cmd.TimeoutSec) * time.Second
		if timeout == 0 {
			timeout = cmdTimeout
		}
		commands = append(commands, executor.PlannedCommand{
			Key:             cmd.CommandKey,
			Command:         cmd.Command,
			Timeout:         timeout,
			ContinueOnError: true,
		})
	}

	if len(commands) == 0 {
		errMsg := fmt.Sprintf("no commands defined for vendor: %s", vendor)
		finishedAt := time.Now()
		_ = handler.UpdateUnitRequired(ctx, unit.ID, &UnitPatch{
			Status:       strPtr(string(UnitStatusFailed)),
			ErrorMessage: &errMsg,
			FinishedAt:   &finishedAt,
		}, "写入采集命令为空状态")
		e.updateRunDeviceStatus(ctx.RunID(), deviceIP, "failed", errMsg)
		runtimeLogger.WriteSummary(scope, errMsg)
		return fmt.Errorf("no commands defined for vendor: %s", vendor)
	}

	if ctx.IsCancelled() {
		handler.MarkUnitCancelled(ctx, unit.ID, "run cancelled before execute plan", intPtrLocal(0))
		e.updateRunDeviceStatus(ctx.RunID(), deviceIP, "cancelled", "run cancelled before execute plan")
		runtimeLogger.WriteSummary(scope, "采集执行前收到取消信号")
		return ctx.Context().Err()
	}

	// Execute plan
	plan := executor.ExecutionPlan{
		Name:               fmt.Sprintf("topology_collect_%s", deviceIP),
		Commands:           commands,
		ContinueOnCmdError: true,
		Mode:               executor.PlanModeDiscovery,
	}

	runtimeLogger.WriteSummary(scope, fmt.Sprintf("开始采集命令，总数: %d", len(commands)))
	commandEventHandler := func(event executor.ExecutionEvent) {
		switch event.Type {
		case executor.EventCmdStart:
			runtimeLogger.WriteSummary(scope, fmt.Sprintf("采集命令[%d/%d]开始: %s", event.Index+1, len(commands), event.Command))
		case executor.EventCmdComplete:
			doneSteps := event.Index + 1
			logger.Verbose("TaskExec", ctx.RunID(), "采集设备 %s 命令进度更新: %d/%d, command=%s, err=%v", deviceIP, doneSteps, len(commands), event.Command, event.Error)
			handler.UpdateUnitBestEffort(ctx, unit.ID, &UnitPatch{
				DoneSteps: &doneSteps,
			}, "更新采集 Unit 进度")

			message := fmt.Sprintf("采集命令[%d/%d]完成: %s", doneSteps, len(commands), event.Command)
			level := EventLevelInfo
			if event.Error != nil {
				message = fmt.Sprintf("采集命令[%d/%d]失败: %s (%v)", doneSteps, len(commands), event.Command, event.Error)
				level = EventLevelWarn
			}
			runtimeLogger.WriteSummary(scope, message)
			ctx.Emit(NewTaskEvent(ctx.RunID(), EventTypeUnitProgress, message).
				WithStage(stageID).
				WithUnit(unit.ID).
				WithLevel(level).
				WithPayload("doneSteps", doneSteps).
				WithPayload("totalSteps", len(commands)).
				WithPayload("command", event.Command))
		}
	}

	report, err := exec.ExecutePlanWithEvents(execCtx, plan, commandEventHandler)
	if err != nil {
		if IsContextCancelled(ctx, err) {
			handler.MarkUnitCancelled(ctx, unit.ID, "run cancelled during execute plan", intPtrLocal(0))
			e.updateRunDeviceStatus(ctx.RunID(), deviceIP, "cancelled", "run cancelled during execute plan")
			return err
		}
		errMsg := fmt.Sprintf("execution failed: %v", err)
		finishedAt := time.Now()
		_ = handler.UpdateUnitRequired(ctx, unit.ID, &UnitPatch{
			Status:       strPtr(string(UnitStatusFailed)),
			ErrorMessage: &errMsg,
			FinishedAt:   &finishedAt,
		}, "写入采集执行失败状态")
		e.updateRunDeviceStatus(ctx.RunID(), deviceIP, "failed", errMsg)
		runtimeLogger.WriteSummary(scope, fmt.Sprintf("采集执行失败: %v", err))
		return fmt.Errorf("execution failed: %w", err)
	}

	if ctx.IsCancelled() {
		handler.MarkUnitCancelled(ctx, unit.ID, "run cancelled before persisting outputs", intPtrLocal(0))
		e.updateRunDeviceStatus(ctx.RunID(), deviceIP, "cancelled", "run cancelled before persisting outputs")
		return ctx.Context().Err()
	}

	// Save outputs
	taskID := ctx.RunID()
	successCount := 0
	for _, result := range report.Results {
		if ctx.IsCancelled() {
			handler.MarkUnitCancelled(ctx, unit.ID, "run cancelled during output persistence", &successCount)
			e.updateRunDeviceStatus(ctx.RunID(), deviceIP, "cancelled", "run cancelled during output persistence")
			return ctx.Context().Err()
		}
		if result != nil && result.Success {
			successCount++
			// Save raw output
			rawPath := filepath.Join(outputDir, taskID, deviceIP, result.CommandKey+"_raw.txt")
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
				normalizedPath = filepath.Join(outputDir, "..", "normalized", taskID, deviceIP, result.CommandKey+".txt")
				if err := os.MkdirAll(filepath.Dir(normalizedPath), 0755); err == nil {
					if err := os.WriteFile(normalizedPath, []byte(normalizedText), 0644); err != nil {
						logger.Warn("TaskExec", ctx.RunID(), "Failed to save normalized output for %s/%s: %v",
							deviceIP, result.CommandKey, err)
					}
				}
			}

			if err := e.createTaskRawOutput(ctx.RunID(), deviceIP, result, rawPath, normalizedPath); err != nil {
				errMsg := fmt.Sprintf("create task raw output failed: %v", err)
				finishedAt := time.Now()
				_ = handler.UpdateUnitRequired(ctx, unit.ID, &UnitPatch{
					Status:       strPtr(string(UnitStatusFailed)),
					DoneSteps:    &successCount,
					ErrorMessage: &errMsg,
					FinishedAt:   &finishedAt,
				}, "写入采集原始输出失败状态")
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
	finishedAt := time.Now()
	if err := handler.UpdateUnitRequired(ctx, unit.ID, &UnitPatch{
		Status:     &status,
		DoneSteps:  &doneSteps,
		FinishedAt: &finishedAt,
	}, "写入采集完成状态"); err != nil {
		return err
	}
	e.updateRunDeviceStatus(ctx.RunID(), deviceIP, deviceStatus, "")
	runtimeLogger.WriteSummary(scope, fmt.Sprintf("采集完成: status=%s, success=%d/%d", status, successCount, len(report.Results)))

	logger.Debug("TaskExec", ctx.RunID(), "Collection completed for device: %s", deviceIP)
	return nil
}

// ParseExecutor for parsing collected data
type ParseExecutor struct {
	db           *gorm.DB
	parserEngine *parser.TextFSMParser
}

// NewParseExecutor creates executor
func NewParseExecutor(db *gorm.DB) *ParseExecutor {
	return &ParseExecutor{db: db, parserEngine: parser.NewTextFSMParser()}
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
				e.updateStageProgress(handler, ctx, stage.ID, len(stage.Units), completedCount, failedCount, cancelledCount)
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
				ctx.Emit(&TaskEvent{
					RunID:   ctx.RunID(),
					StageID: stage.ID,
					UnitID:  u.ID,
					Type:    EventTypeUnitFinished,
					Level:   EventLevelWarn,
					Message: fmt.Sprintf("Parse cancelled for %s", u.Target.Key),
				})
			} else if err != nil {
				ctx.Emit(&TaskEvent{
					RunID:   ctx.RunID(),
					StageID: stage.ID,
					UnitID:  u.ID,
					Type:    EventTypeUnitFinished,
					Level:   EventLevelError,
					Message: fmt.Sprintf("Parse failed for %s: %v", u.Target.Key, err),
				})
			} else {
				ctx.Emit(&TaskEvent{
					RunID:   ctx.RunID(),
					StageID: stage.ID,
					UnitID:  u.ID,
					Type:    EventTypeUnitFinished,
					Level:   EventLevelInfo,
					Message: fmt.Sprintf("Parse completed for %s", u.Target.Key),
				})
			}

			e.updateStageProgress(handler, ctx, stage.ID, len(stage.Units), localCompleted, localFailed, localCancelled)
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
		handler.MarkUnitCancelled(ctx, unit.ID, "run cancelled before parse unit start", intPtrLocal(0))
		return ctx.Context().Err()
	}

	startedAt := time.Now()
	unitRunning := string(UnitStatusRunning)
	if err := handler.UpdateUnitRequired(ctx, unit.ID, &UnitPatch{
		Status:    &unitRunning,
		StartedAt: &startedAt,
	}, "设置解析 Unit 为 running"); err != nil {
		return err
	}

	// For parse stage, target is device_ip
	if unit.Target.Type != "device_ip" {
		errMsg := fmt.Sprintf("unsupported target type: %s", unit.Target.Type)
		finishedAt := time.Now()
		_ = handler.UpdateUnitRequired(ctx, unit.ID, &UnitPatch{
			Status:       strPtr(string(UnitStatusFailed)),
			ErrorMessage: &errMsg,
			FinishedAt:   &finishedAt,
		}, "写入解析 Unit 失败状态")
		return fmt.Errorf("%s", errMsg)
	}

	deviceIP := unit.Target.Key
	logger.Debug("TaskExec", ctx.RunID(), "Parsing device: %s", deviceIP)

	if ctx.IsCancelled() {
		handler.MarkUnitCancelled(ctx, unit.ID, "run cancelled before parse execution", intPtrLocal(0))
		return ctx.Context().Err()
	}

	vendor := ""
	if len(unit.Steps) > 0 {
		vendor = unit.Steps[0].Params["vendor"]
	}
	if err := e.parseAndSaveRunDevice(ctx, deviceIP, vendor); err != nil {
		if IsContextCancelled(ctx, err) {
			handler.MarkUnitCancelled(ctx, unit.ID, "run cancelled during parse execution", intPtrLocal(0))
			return err
		}
		errMsg := fmt.Sprintf("parse failed: %v", err)
		doneSteps := 0
		finishedAt := time.Now()
		_ = handler.UpdateUnitRequired(ctx, unit.ID, &UnitPatch{
			Status:       strPtr(string(UnitStatusFailed)),
			DoneSteps:    &doneSteps,
			ErrorMessage: &errMsg,
			FinishedAt:   &finishedAt,
		}, "写入解析失败状态")
		return fmt.Errorf("parse failed: %w", err)
	}

	doneSteps := 1
	unitCompleted := string(UnitStatusCompleted)
	finishedAt := time.Now()
	if err := handler.UpdateUnitRequired(ctx, unit.ID, &UnitPatch{
		Status:     &unitCompleted,
		DoneSteps:  &doneSteps,
		FinishedAt: &finishedAt,
	}, "写入解析完成状态"); err != nil {
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
	ctx.Emit(&TaskEvent{
		RunID:   ctx.RunID(),
		StageID: stage.ID,
		Type:    EventTypeStageStarted,
		Level:   EventLevelInfo,
		Message: "Starting topology build...",
	})

	result, err := e.buildRunTopology(ctx.RunID())
	if err != nil {
		if IsContextCancelled(ctx, err) {
			return err
		}
		logger.Error("TaskExec", ctx.RunID(), "Topology build failed: %v", err)

		ctx.Emit(&TaskEvent{
			RunID:   ctx.RunID(),
			StageID: stage.ID,
			Type:    EventTypeStageFinished,
			Level:   EventLevelError,
			Message: fmt.Sprintf("Topology build failed: %v", err),
		})

		return fmt.Errorf("topology build failed: %w", err)
	}

	// Get build statistics
	edgeCount := 0
	if result != nil {
		edgeCount = result.TotalEdges
	}

	// Emit completion event
	ctx.Emit(&TaskEvent{
		RunID:   ctx.RunID(),
		StageID: stage.ID,
		Type:    EventTypeStageFinished,
		Level:   EventLevelInfo,
		Message: fmt.Sprintf("Topology build completed with %d edges", edgeCount),
	})

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

func (e *DeviceCollectExecutor) createTaskRawOutput(taskID, deviceIP string, result *executor.CommandResult, rawPath, normalizedPath string) error {
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
	return e.db.Create(&TaskRawOutput{
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
	}).Error
}

func (e *ParseExecutor) parseAndSaveRunDevice(ctx RuntimeContext, deviceIP, vendor string) error {
	handler := NewErrorHandler(ctx.RunID())
	runID := ctx.RunID()
	if vendor == "" {
		var dev TaskRunDevice
		if err := e.db.Where("task_run_id = ? AND device_ip = ?", runID, deviceIP).First(&dev).Error; err == nil {
			vendor = strings.ToLower(strings.TrimSpace(dev.Vendor))
		}
	}
	if vendor == "" {
		vendor = "huawei"
	}
	if err := e.parserEngine.LoadBuiltinTemplates(vendor); err != nil {
		return fmt.Errorf("load vendor templates failed: %w", err)
	}

	var outputs []TaskRawOutput
	if err := e.db.Where("task_run_id = ? AND device_ip = ? AND status = ?", runID, deviceIP, "success").
		Order("created_at ASC").Find(&outputs).Error; err != nil {
		return fmt.Errorf("load raw outputs failed: %w", err)
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
		rows, err := e.parserEngine.Parse(output.CommandKey, string(rawText))
		if err != nil {
			handler.LogDBErrorWithContext("更新 parse_status 为 parse_failed", e.db.Model(&TaskRawOutput{}).Where("id = ?", output.ID).
				Updates(map[string]interface{}{"parse_status": "parse_failed", "parse_error": err.Error()}).Error,
				map[string]interface{}{"runID": runID, "deviceIP": deviceIP, "outputID": output.ID, "commandKey": output.CommandKey})
			continue
		}

		parseStatus := "success"
		parseError := ""
		switch output.CommandKey {
		case "version":
			id, mapErr := mapper.ToDeviceInfo(rows)
			if mapErr != nil {
				parseStatus = "parse_failed"
				parseError = mapErr.Error()
				break
			}
			if id != nil {
				if strings.TrimSpace(id.Vendor) != "" {
					identity.Vendor = id.Vendor
				}
				if strings.TrimSpace(id.Model) != "" {
					identity.Model = id.Model
				}
				if strings.TrimSpace(id.SerialNumber) != "" {
					identity.SerialNumber = id.SerialNumber
				}
				if strings.TrimSpace(id.Version) != "" {
					identity.Version = id.Version
				}
				if strings.TrimSpace(id.Hostname) != "" {
					identity.Hostname = id.Hostname
				}
				if strings.TrimSpace(id.MgmtIP) != "" {
					identity.MgmtIP = id.MgmtIP
				}
				if strings.TrimSpace(id.ChassisID) != "" {
					identity.ChassisID = id.ChassisID
				}
			}
		case "interface_brief", "interface_detail":
			items, mapErr := mapper.ToInterfaces(rows)
			if mapErr != nil {
				parseStatus = "parse_failed"
				parseError = mapErr.Error()
				break
			}
			interfaces = append(interfaces, items...)
		case "lldp_neighbor":
			items, mapErr := mapper.ToLLDP(rows)
			if mapErr != nil {
				parseStatus = "parse_failed"
				parseError = mapErr.Error()
				break
			}
			rawRef := fmt.Sprintf("%d", output.ID)
			for i := range items {
				items[i].CommandKey = output.CommandKey
				items[i].RawRefID = rawRef
			}
			lldps = append(lldps, items...)
		case "mac_address":
			items, mapErr := mapper.ToFDB(rows)
			if mapErr != nil {
				parseStatus = "parse_failed"
				parseError = mapErr.Error()
				break
			}
			rawRef := fmt.Sprintf("%d", output.ID)
			for i := range items {
				items[i].CommandKey = output.CommandKey
				items[i].RawRefID = rawRef
			}
			fdbs = append(fdbs, items...)
		case "arp_all":
			items, mapErr := mapper.ToARP(rows)
			if mapErr != nil {
				parseStatus = "parse_failed"
				parseError = mapErr.Error()
				break
			}
			rawRef := fmt.Sprintf("%d", output.ID)
			for i := range items {
				items[i].CommandKey = output.CommandKey
				items[i].RawRefID = rawRef
			}
			arps = append(arps, items...)
		case "eth_trunk", "eth_trunk_verbose":
			items, mapErr := mapper.ToAggregate(rows)
			if mapErr != nil {
				parseStatus = "parse_failed"
				parseError = mapErr.Error()
				break
			}
			rawRef := fmt.Sprintf("%d", output.ID)
			for i := range items {
				items[i].CommandKey = output.CommandKey
				items[i].RawRefID = rawRef
			}
			aggs = append(aggs, items...)
		}
		handler.LogDBErrorWithContext("更新 parse_status", e.db.Model(&TaskRawOutput{}).Where("id = ?", output.ID).
			Updates(map[string]interface{}{"parse_status": parseStatus, "parse_error": parseError}).Error,
			map[string]interface{}{"runID": runID, "deviceIP": deviceIP, "outputID": output.ID, "commandKey": output.CommandKey, "parseStatus": parseStatus})
	}

	if ctx.IsCancelled() {
		return ctx.Context().Err()
	}

	identity.Vendor = normalize.NormalizeVendor(identity.Vendor)
	identity.Hostname = normalize.NormalizeDeviceName(identity.Hostname)

	return e.db.Transaction(func(tx *gorm.DB) error {
		if ctx.IsCancelled() {
			return ctx.Context().Err()
		}
		if err := tx.Model(&TaskRunDevice{}).
			Where("task_run_id = ? AND device_ip = ?", runID, deviceIP).
			Updates(map[string]interface{}{
				"vendor":          identity.Vendor,
				"model":           identity.Model,
				"serial_number":   identity.SerialNumber,
				"version":         identity.Version,
				"hostname":        identity.Hostname,
				"normalized_name": identity.Hostname,
				"mgmt_ip":         identity.MgmtIP,
				"chassis_id":      identity.ChassisID,
			}).Error; err != nil {
			return err
		}

		if err := tx.Where("task_run_id = ? AND device_ip = ?", runID, deviceIP).Delete(&TaskParsedInterface{}).Error; err != nil {
			return err
		}
		if err := tx.Where("task_run_id = ? AND device_ip = ?", runID, deviceIP).Delete(&TaskParsedLLDPNeighbor{}).Error; err != nil {
			return err
		}
		if err := tx.Where("task_run_id = ? AND device_ip = ?", runID, deviceIP).Delete(&TaskParsedFDBEntry{}).Error; err != nil {
			return err
		}
		if err := tx.Where("task_run_id = ? AND device_ip = ?", runID, deviceIP).Delete(&TaskParsedARPEntry{}).Error; err != nil {
			return err
		}
		if err := tx.Where("task_run_id = ? AND device_ip = ?", runID, deviceIP).Delete(&TaskParsedAggregateMember{}).Error; err != nil {
			return err
		}
		if err := tx.Where("task_run_id = ? AND device_ip = ?", runID, deviceIP).Delete(&TaskParsedAggregateGroup{}).Error; err != nil {
			return err
		}

		for _, iface := range interfaces {
			if ctx.IsCancelled() {
				return ctx.Context().Err()
			}
			if err := tx.Create(&TaskParsedInterface{
				TaskRunID:     runID,
				DeviceIP:      deviceIP,
				InterfaceName: iface.Name,
				Status:        iface.Status,
				Speed:         iface.Speed,
				Duplex:        iface.Duplex,
				Description:   iface.Description,
				MACAddress:    iface.MACAddress,
				IPAddress:     iface.IPAddress,
				IsAggregate:   iface.IsAggregate,
				AggregateID:   iface.AggregateID,
			}).Error; err != nil {
				return err
			}
		}
		for _, n := range lldps {
			if ctx.IsCancelled() {
				return ctx.Context().Err()
			}
			if err := tx.Create(&TaskParsedLLDPNeighbor{
				TaskRunID:       runID,
				DeviceIP:        deviceIP,
				LocalInterface:  n.LocalInterface,
				NeighborName:    n.NeighborName,
				NeighborChassis: n.NeighborChassis,
				NeighborPort:    n.NeighborPort,
				NeighborIP:      n.NeighborIP,
				NeighborDesc:    n.NeighborDesc,
				CommandKey:      n.CommandKey,
				RawRefID:        n.RawRefID,
			}).Error; err != nil {
				return err
			}
		}
		for _, f := range fdbs {
			if ctx.IsCancelled() {
				return ctx.Context().Err()
			}
			if err := tx.Create(&TaskParsedFDBEntry{
				TaskRunID:  runID,
				DeviceIP:   deviceIP,
				MACAddress: f.MACAddress,
				VLAN:       f.VLAN,
				Interface:  f.Interface,
				Type:       f.Type,
				CommandKey: f.CommandKey,
				RawRefID:   f.RawRefID,
			}).Error; err != nil {
				return err
			}
		}
		for _, a := range arps {
			if ctx.IsCancelled() {
				return ctx.Context().Err()
			}
			if err := tx.Create(&TaskParsedARPEntry{
				TaskRunID:  runID,
				DeviceIP:   deviceIP,
				IPAddress:  a.IPAddress,
				MACAddress: a.MACAddress,
				Interface:  a.Interface,
				Type:       a.Type,
				CommandKey: a.CommandKey,
				RawRefID:   a.RawRefID,
			}).Error; err != nil {
				return err
			}
		}
		for _, g := range aggs {
			if ctx.IsCancelled() {
				return ctx.Context().Err()
			}
			if err := tx.Create(&TaskParsedAggregateGroup{
				TaskRunID:     runID,
				DeviceIP:      deviceIP,
				AggregateName: g.AggregateName,
				Mode:          g.Mode,
				CommandKey:    g.CommandKey,
				RawRefID:      g.RawRefID,
			}).Error; err != nil {
				return err
			}
			for _, member := range g.MemberPorts {
				if ctx.IsCancelled() {
					return ctx.Context().Err()
				}
				if err := tx.Create(&TaskParsedAggregateMember{
					TaskRunID:     runID,
					DeviceIP:      deviceIP,
					AggregateName: g.AggregateName,
					MemberPort:    member,
					CommandKey:    g.CommandKey,
					RawRefID:      g.RawRefID,
				}).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (e *TopologyBuildExecutor) buildRunTopology(runID string) (*models.TopologyBuildResult, error) {
	if err := e.db.Where("task_run_id = ?", runID).Delete(&TaskTopologyEdge{}).Error; err != nil {
		return nil, err
	}

	var lldps []TaskParsedLLDPNeighbor
	if err := e.db.Where("task_run_id = ?", runID).Find(&lldps).Error; err != nil {
		return nil, err
	}

	type keyData struct {
		aDevice string
		aIf     string
		bDevice string
		bIf     string
	}
	edges := make(map[string]TaskTopologyEdge)

	buildKey := func(aDevice, aIf, bDevice, bIf string) string {
		left := aDevice + ":" + aIf
		right := bDevice + ":" + bIf
		pair := []string{left, right}
		sort.Strings(pair)
		return pair[0] + "<->" + pair[1]
	}

	var devices []TaskRunDevice
	_ = e.db.Where("task_run_id = ?", runID).Find(&devices).Error
	deviceByName := make(map[string]string, len(devices))
	for _, d := range devices {
		name := strings.TrimSpace(strings.ToLower(d.NormalizedName))
		if name != "" {
			deviceByName[name] = d.DeviceIP
		}
	}

	for _, n := range lldps {
		localIf := normalize.NormalizeInterfaceName(n.LocalInterface)
		if localIf == "" {
			continue
		}
		remoteIf := normalize.NormalizeInterfaceName(n.NeighborPort)
		if remoteIf == "" {
			remoteIf = "unknown"
		}
		remoteDevice := ""
		if strings.TrimSpace(n.NeighborIP) != "" {
			remoteDevice = strings.TrimSpace(n.NeighborIP)
		} else {
			remoteDevice = deviceByName[strings.ToLower(strings.TrimSpace(n.NeighborName))]
		}
		if remoteDevice == "" {
			remoteDevice = "unknown:" + n.DeviceIP + ":" + localIf
		}
		k := buildKey(n.DeviceIP, localIf, remoteDevice, remoteIf)
		evidence := EdgeEvidence{
			Type:       "lldp",
			Source:     "lldp",
			DeviceID:   n.DeviceIP,
			Command:    chooseValue(n.CommandKey, "lldp_neighbor"),
			RawRefID:   n.RawRefID,
			LocalIf:    localIf,
			RemoteName: n.NeighborName,
			RemoteIf:   remoteIf,
			RemoteMAC:  n.NeighborChassis,
			RemoteIP:   n.NeighborIP,
			Summary:    fmt.Sprintf("LLDP %s -> %s(%s)", localIf, n.NeighborName, remoteIf),
		}
		exist, ok := edges[k]
		if ok {
			exist.Evidence = append(exist.Evidence, evidence)
			exist.DiscoveryMethods = appendUniqueStrings(exist.DiscoveryMethods, "lldp_bidirectional")
			exist.Status = "confirmed"
			exist.Confidence = 1.0
			edges[k] = exist
			continue
		}
		edges[k] = TaskTopologyEdge{
			ID:               makeTaskEdgeID(),
			TaskRunID:        runID,
			ADeviceID:        n.DeviceIP,
			AIf:              localIf,
			BDeviceID:        remoteDevice,
			BIf:              remoteIf,
			EdgeType:         "physical",
			Status:           "semi_confirmed",
			Confidence:       0.75,
			DiscoveryMethods: []string{"lldp_single_side"},
			Evidence:         []EdgeEvidence{evidence},
		}
	}

	saved := make([]TaskTopologyEdge, 0, len(edges))
	for _, edge := range edges {
		saved = append(saved, edge)
	}
	if len(saved) > 0 {
		if err := e.db.Create(&saved).Error; err != nil {
			return nil, err
		}
	}

	result := &models.TopologyBuildResult{
		TaskID:             runID,
		TotalEdges:         len(saved),
		ConfirmedEdges:     0,
		SemiConfirmedEdges: 0,
		InferredEdges:      0,
		ConflictEdges:      0,
		BuildTime:          0,
		Errors:             []string{},
	}
	for _, edge := range saved {
		switch edge.Status {
		case "confirmed":
			result.ConfirmedEdges++
		case "semi_confirmed":
			result.SemiConfirmedEdges++
		case "inferred":
			result.InferredEdges++
		case "conflict":
			result.ConflictEdges++
		}
	}
	return result, nil
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

func (e *DeviceCommandExecutor) updateStageProgress(handler *ErrorHandler, ctx RuntimeContext, stageID string, totalUnits, completedCount, failedCount, cancelledCount int) {
	finished := completedCount + failedCount + cancelledCount
	progress := 100
	if totalUnits > 0 {
		progress = finished * 100 / totalUnits
	}
	handler.UpdateStageBestEffort(ctx, stageID, &StagePatch{
		CompletedUnits: &finished,
		SuccessUnits:   &completedCount,
		FailedUnits:    &failedCount,
		CancelledUnits: &cancelledCount,
		Progress:       &progress,
	}, "更新命令阶段进度")
}

func (e *DeviceCommandExecutor) updateStageProgressDetailed(handler *ErrorHandler, ctx RuntimeContext, stageID string, totalUnits, completedCount, failedCount, cancelledCount, progress int) {
	finished := completedCount + failedCount + cancelledCount
	handler.UpdateStageBestEffort(ctx, stageID, &StagePatch{
		CompletedUnits: &finished,
		SuccessUnits:   &completedCount,
		FailedUnits:    &failedCount,
		CancelledUnits: &cancelledCount,
		Progress:       &progress,
	}, "更新命令阶段细粒度进度")
}

func (e *DeviceCollectExecutor) updateStageProgress(handler *ErrorHandler, ctx RuntimeContext, stageID string, totalUnits, completedCount, failedCount, cancelledCount int) {
	finished := completedCount + failedCount + cancelledCount
	progress := 100
	if totalUnits > 0 {
		progress = finished * 100 / totalUnits
	}
	handler.UpdateStageBestEffort(ctx, stageID, &StagePatch{
		CompletedUnits: &finished,
		SuccessUnits:   &completedCount,
		FailedUnits:    &failedCount,
		CancelledUnits: &cancelledCount,
		Progress:       &progress,
	}, "更新采集阶段进度")
}

func (e *ParseExecutor) updateStageProgress(handler *ErrorHandler, ctx RuntimeContext, stageID string, totalUnits, completedCount, failedCount, cancelledCount int) {
	finished := completedCount + failedCount + cancelledCount
	progress := 100
	if totalUnits > 0 {
		progress = finished * 100 / totalUnits
	}
	handler.UpdateStageBestEffort(ctx, stageID, &StagePatch{
		CompletedUnits: &finished,
		SuccessUnits:   &completedCount,
		FailedUnits:    &failedCount,
		CancelledUnits: &cancelledCount,
		Progress:       &progress,
	}, "更新解析阶段进度")
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

func intPtrLocal(v int) *int {
	return &v
}
