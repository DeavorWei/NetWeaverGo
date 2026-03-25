package taskexec

import (
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
	"github.com/google/uuid"
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

	semaphore := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var completedCount, failedCount int
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

			// Execute unit
			if err := e.executeUnit(ctx, stage.ID, &u); err != nil {
				mu.Lock()
				failedCount++
				mu.Unlock()
			} else {
				mu.Lock()
				completedCount++
				mu.Unlock()
			}

			mu.Lock()
			completed := completedCount + failedCount
			progress := 100
			if len(stage.Units) > 0 {
				progress = completed * 100 / len(stage.Units)
			}
			mu.Unlock()

			ctx.UpdateStage(stage.ID, &StagePatch{
				CompletedUnits: &completed,
				SuccessUnits:   &completedCount,
				FailedUnits:    &failedCount,
				Progress:       &progress,
			})
		}(unit)
	}

	wg.Wait()
	logger.Info("TaskExec", ctx.RunID(), "Stage completed: success=%d, failed=%d", completedCount, failedCount)
	return nil
}

func (e *DeviceCommandExecutor) executeUnit(ctx RuntimeContext, stageID string, unit *UnitPlan) error {
	startedAt := time.Now()
	unitRunning := string(UnitStatusRunning)
	_ = ctx.UpdateUnit(unit.ID, &UnitPatch{
		Status:    &unitRunning,
		StartedAt: &startedAt,
	})

	// Get device info from unit target
	if unit.Target.Type != "device_ip" {
		errMsg := fmt.Sprintf("unsupported target type: %s", unit.Target.Type)
		unitFailed := string(UnitStatusFailed)
		_ = ctx.UpdateUnit(unit.ID, &UnitPatch{
			Status:       &unitFailed,
			ErrorMessage: &errMsg,
		})
		return fmt.Errorf("%s", errMsg)
	}

	deviceIP := unit.Target.Key
	logger.Debug("TaskExec", ctx.RunID(), "Execute unit for device: %s", deviceIP)

	// Get device from repository
	device, err := e.repo.FindByIP(deviceIP)
	if err != nil {
		logger.Error("TaskExec", ctx.RunID(), "Failed to find device %s: %v", deviceIP, err)
		errMsg := fmt.Sprintf("Device not found: %s", deviceIP)
		unitFailed := string(UnitStatusFailed)
		finishedAt := time.Now()
		_ = ctx.UpdateUnit(unit.ID, &UnitPatch{
			Status:       &unitFailed,
			ErrorMessage: &errMsg,
			FinishedAt:   &finishedAt,
		})
		ctx.Emit(&TaskEvent{
			RunID:   ctx.RunID(),
			StageID: stageID,
			UnitID:  unit.ID,
			Type:    EventTypeUnitFinished,
			Level:   EventLevelError,
			Message: fmt.Sprintf("Device not found: %s", deviceIP),
		})
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
		_ = ctx.UpdateUnit(unit.ID, &UnitPatch{
			Status:     &unitCompleted,
			DoneSteps:  &doneSteps,
			FinishedAt: &finishedAt,
		})
		return nil
	}

	// Create device executor options
	execCtx := ctx.Context()
	opts := executor.ExecutorOptions{
		Vendor: device.Vendor,
	}

	// Create device executor
	exec := executor.NewDeviceExecutor(
		device.IP,
		device.Port,
		device.Username,
		device.Password,
		opts,
	)

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

	// Connect to device
	ctx.Emit(&TaskEvent{
		RunID:   ctx.RunID(),
		StageID: stageID,
		UnitID:  unit.ID,
		Type:    EventTypeUnitStarted,
		Level:   EventLevelInfo,
		Message: fmt.Sprintf("Connecting to %s...", deviceIP),
	})

	if err := exec.Connect(execCtx, connTimeout); err != nil {
		logger.Error("TaskExec", ctx.RunID(), "Failed to connect to %s: %v", deviceIP, err)
		errMsg := fmt.Sprintf("connection failed: %v", err)
		unitFailed := string(UnitStatusFailed)
		finishedAt := time.Now()
		_ = ctx.UpdateUnit(unit.ID, &UnitPatch{
			Status:       &unitFailed,
			ErrorMessage: &errMsg,
			FinishedAt:   &finishedAt,
		})
		ctx.Emit(&TaskEvent{
			RunID:   ctx.RunID(),
			StageID: stageID,
			UnitID:  unit.ID,
			Type:    EventTypeUnitFinished,
			Level:   EventLevelError,
			Message: fmt.Sprintf("Connection failed: %v", err),
		})
		return err
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

	if err := exec.ExecutePlaybook(execCtx, commands, cmdTimeout); err != nil {
		logger.Error("TaskExec", ctx.RunID(), "Failed to execute commands on %s: %v", deviceIP, err)
		errMsg := fmt.Sprintf("command execution failed: %v", err)
		unitFailed := string(UnitStatusFailed)
		doneSteps := len(commands)
		finishedAt := time.Now()
		_ = ctx.UpdateUnit(unit.ID, &UnitPatch{
			Status:       &unitFailed,
			DoneSteps:    &doneSteps,
			ErrorMessage: &errMsg,
			FinishedAt:   &finishedAt,
		})
		ctx.Emit(&TaskEvent{
			RunID:   ctx.RunID(),
			StageID: stageID,
			UnitID:  unit.ID,
			Type:    EventTypeUnitFinished,
			Level:   EventLevelError,
			Message: fmt.Sprintf("Command execution failed: %v", err),
		})
		return err
	}

	doneSteps := len(commands)
	unitCompleted := string(UnitStatusCompleted)
	finishedAt := time.Now()
	_ = ctx.UpdateUnit(unit.ID, &UnitPatch{
		Status:     &unitCompleted,
		DoneSteps:  &doneSteps,
		FinishedAt: &finishedAt,
	})

	// Success
	ctx.Emit(&TaskEvent{
		RunID:   ctx.RunID(),
		StageID: stageID,
		UnitID:  unit.ID,
		Type:    EventTypeUnitFinished,
		Level:   EventLevelInfo,
		Message: fmt.Sprintf("Successfully executed %d commands on %s", len(commands), deviceIP),
	})

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

	semaphore := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var completedCount, failedCount int
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

			// Execute collection
			if err := e.executeCollect(ctx, stage.ID, &u, outputDir); err != nil {
				mu.Lock()
				failedCount++
				mu.Unlock()

				ctx.Emit(&TaskEvent{
					RunID:   ctx.RunID(),
					StageID: stage.ID,
					UnitID:  u.ID,
					Type:    EventTypeUnitFinished,
					Level:   EventLevelError,
					Message: fmt.Sprintf("Collection failed for %s: %v", u.Target.Key, err),
				})
			} else {
				mu.Lock()
				completedCount++
				mu.Unlock()

				ctx.Emit(&TaskEvent{
					RunID:   ctx.RunID(),
					StageID: stage.ID,
					UnitID:  u.ID,
					Type:    EventTypeUnitFinished,
					Level:   EventLevelInfo,
					Message: fmt.Sprintf("Collection completed for %s", u.Target.Key),
				})
			}

			mu.Lock()
			completed := completedCount + failedCount
			progress := 100
			if len(stage.Units) > 0 {
				progress = completed * 100 / len(stage.Units)
			}
			mu.Unlock()

			ctx.UpdateStage(stage.ID, &StagePatch{
				CompletedUnits: &completed,
				Progress:       &progress,
			})
		}(unit)
	}

	wg.Wait()
	logger.Info("TaskExec", ctx.RunID(), "Collection stage completed: success=%d, failed=%d", completedCount, failedCount)
	return nil
}

func (e *DeviceCollectExecutor) executeCollect(ctx RuntimeContext, stageID string, unit *UnitPlan, outputDir string) error {
	startedAt := time.Now()
	unitRunning := string(UnitStatusRunning)
	_ = ctx.UpdateUnit(unit.ID, &UnitPatch{
		Status:    &unitRunning,
		StartedAt: &startedAt,
	})

	// Get device info from unit target
	if unit.Target.Type != "device_ip" {
		errMsg := fmt.Sprintf("unsupported target type: %s", unit.Target.Type)
		unitFailed := string(UnitStatusFailed)
		_ = ctx.UpdateUnit(unit.ID, &UnitPatch{
			Status:       &unitFailed,
			ErrorMessage: &errMsg,
		})
		return fmt.Errorf("%s", errMsg)
	}

	deviceIP := unit.Target.Key
	logger.Debug("TaskExec", ctx.RunID(), "Collecting from device: %s", deviceIP)

	// Get device from repository
	device, err := e.repo.FindByIP(deviceIP)
	if err != nil {
		errMsg := fmt.Sprintf("device not found: %v", err)
		unitFailed := string(UnitStatusFailed)
		finishedAt := time.Now()
		_ = ctx.UpdateUnit(unit.ID, &UnitPatch{
			Status:       &unitFailed,
			ErrorMessage: &errMsg,
			FinishedAt:   &finishedAt,
		})
		return fmt.Errorf("device not found: %w", err)
	}
	e.ensureRunDevice(ctx.RunID(), device)

	// Get vendor profile
	vendor := device.Vendor
	if vendor == "" {
		vendor = "generic"
	}

	// Create device executor options
	execCtx := ctx.Context()
	opts := executor.ExecutorOptions{
		Vendor: vendor,
	}

	// Create device executor
	exec := executor.NewDeviceExecutor(
		device.IP,
		device.Port,
		device.Username,
		device.Password,
		opts,
	)

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

	// Connect to device
	if err := exec.Connect(execCtx, connTimeout); err != nil {
		errMsg := fmt.Sprintf("connection failed: %v", err)
		unitFailed := string(UnitStatusFailed)
		finishedAt := time.Now()
		_ = ctx.UpdateUnit(unit.ID, &UnitPatch{
			Status:       &unitFailed,
			ErrorMessage: &errMsg,
			FinishedAt:   &finishedAt,
		})
		e.updateRunDeviceStatus(ctx.RunID(), deviceIP, "failed", errMsg)
		return fmt.Errorf("connection failed: %w", err)
	}

	// Get commands from device profile
	profile := config.GetDeviceProfile(vendor)
	if profile == nil {
		errMsg := fmt.Sprintf("no profile found for vendor: %s", vendor)
		unitFailed := string(UnitStatusFailed)
		_ = ctx.UpdateUnit(unit.ID, &UnitPatch{
			Status:       &unitFailed,
			ErrorMessage: &errMsg,
		})
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
		unitFailed := string(UnitStatusFailed)
		_ = ctx.UpdateUnit(unit.ID, &UnitPatch{
			Status:       &unitFailed,
			ErrorMessage: &errMsg,
		})
		return fmt.Errorf("no commands defined for vendor: %s", vendor)
	}

	// Execute plan
	plan := executor.ExecutionPlan{
		Name:               fmt.Sprintf("topology_collect_%s", deviceIP),
		Commands:           commands,
		ContinueOnCmdError: true,
		Mode:               executor.PlanModeDiscovery,
	}

	report, err := exec.ExecutePlan(execCtx, plan)
	if err != nil {
		errMsg := fmt.Sprintf("execution failed: %v", err)
		unitFailed := string(UnitStatusFailed)
		finishedAt := time.Now()
		_ = ctx.UpdateUnit(unit.ID, &UnitPatch{
			Status:       &unitFailed,
			ErrorMessage: &errMsg,
			FinishedAt:   &finishedAt,
		})
		e.updateRunDeviceStatus(ctx.RunID(), deviceIP, "failed", errMsg)
		return fmt.Errorf("execution failed: %w", err)
	}

	// Save outputs
	taskID := ctx.RunID()
	successCount := 0
	for _, result := range report.Results {
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

			e.createTaskRawOutput(ctx.RunID(), deviceIP, result, rawPath, normalizedPath)
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
	_ = ctx.UpdateUnit(unit.ID, &UnitPatch{
		Status:     &status,
		DoneSteps:  &doneSteps,
		FinishedAt: &finishedAt,
	})
	e.updateRunDeviceStatus(ctx.RunID(), deviceIP, deviceStatus, "")

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

	var wg sync.WaitGroup
	var completedCount, failedCount int
	var mu sync.Mutex

	for _, unit := range stage.Units {
		if ctx.IsCancelled() {
			break
		}

		wg.Add(1)
		go func(u UnitPlan) {
			defer wg.Done()

			if err := e.executeParse(ctx, stage.ID, &u); err != nil {
				mu.Lock()
				failedCount++
				mu.Unlock()

				ctx.Emit(&TaskEvent{
					RunID:   ctx.RunID(),
					StageID: stage.ID,
					UnitID:  u.ID,
					Type:    EventTypeUnitFinished,
					Level:   EventLevelError,
					Message: fmt.Sprintf("Parse failed for %s: %v", u.Target.Key, err),
				})
			} else {
				mu.Lock()
				completedCount++
				mu.Unlock()

				ctx.Emit(&TaskEvent{
					RunID:   ctx.RunID(),
					StageID: stage.ID,
					UnitID:  u.ID,
					Type:    EventTypeUnitFinished,
					Level:   EventLevelInfo,
					Message: fmt.Sprintf("Parse completed for %s", u.Target.Key),
				})
			}

			mu.Lock()
			completed := completedCount + failedCount
			progress := 100
			if len(stage.Units) > 0 {
				progress = completed * 100 / len(stage.Units)
			}
			mu.Unlock()

			ctx.UpdateStage(stage.ID, &StagePatch{
				CompletedUnits: &completed,
				Progress:       &progress,
			})
		}(unit)
	}

	wg.Wait()
	logger.Info("TaskExec", ctx.RunID(), "Parse stage completed: success=%d, failed=%d", completedCount, failedCount)
	return nil
}

func (e *ParseExecutor) executeParse(ctx RuntimeContext, stageID string, unit *UnitPlan) error {
	startedAt := time.Now()
	unitRunning := string(UnitStatusRunning)
	_ = ctx.UpdateUnit(unit.ID, &UnitPatch{
		Status:    &unitRunning,
		StartedAt: &startedAt,
	})

	// For parse stage, target is device_ip
	if unit.Target.Type != "device_ip" {
		errMsg := fmt.Sprintf("unsupported target type: %s", unit.Target.Type)
		unitFailed := string(UnitStatusFailed)
		_ = ctx.UpdateUnit(unit.ID, &UnitPatch{
			Status:       &unitFailed,
			ErrorMessage: &errMsg,
		})
		return fmt.Errorf("%s", errMsg)
	}

	deviceIP := unit.Target.Key
	logger.Debug("TaskExec", ctx.RunID(), "Parsing device: %s", deviceIP)

	vendor := ""
	if len(unit.Steps) > 0 {
		vendor = unit.Steps[0].Params["vendor"]
	}
	if err := e.parseAndSaveRunDevice(ctx.RunID(), deviceIP, vendor); err != nil {
		errMsg := fmt.Sprintf("parse failed: %v", err)
		unitFailed := string(UnitStatusFailed)
		doneSteps := 0
		finishedAt := time.Now()
		_ = ctx.UpdateUnit(unit.ID, &UnitPatch{
			Status:       &unitFailed,
			DoneSteps:    &doneSteps,
			ErrorMessage: &errMsg,
			FinishedAt:   &finishedAt,
		})
		return fmt.Errorf("parse failed: %w", err)
	}

	doneSteps := 1
	unitCompleted := string(UnitStatusCompleted)
	finishedAt := time.Now()
	_ = ctx.UpdateUnit(unit.ID, &UnitPatch{
		Status:     &unitCompleted,
		DoneSteps:  &doneSteps,
		FinishedAt: &finishedAt,
	})
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

	if len(stage.Units) == 0 {
		return fmt.Errorf("no units in build stage")
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

	ctx.UpdateStage(stage.ID, &StagePatch{
		Status:         &completedStatus,
		Progress:       &progress,
		CompletedUnits: &doneSteps,
		SuccessUnits:   &doneSteps,
		FinishedAt:     &now,
	})
	e.createArtifact(ctx.RunID(), stage.ID, stage.Units[0].ID, string(ArtifactTypeTopologyGraph), "topology_graph", "")

	logger.Info("TaskExec", ctx.RunID(), "Topology build completed with %d edges", edgeCount)
	return nil
}

func (e *DeviceCollectExecutor) ensureRunDevice(taskID string, device *models.DeviceAsset) {
	now := time.Now()
	var record TaskRunDevice
	err := e.db.Where("task_run_id = ? AND device_ip = ?", taskID, device.IP).First(&record).Error
	if err == nil {
		return
	}
	_ = e.db.Create(&TaskRunDevice{
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
	updates := map[string]interface{}{
		"status":        status,
		"error_message": errMsg,
		"updated_at":    time.Now(),
	}
	if status == "success" || status == "partial" || status == "failed" {
		now := time.Now()
		updates["finished_at"] = now
	}
	_ = e.db.Model(&TaskRunDevice{}).
		Where("task_run_id = ? AND device_ip = ?", taskID, deviceIP).
		Updates(updates).Error
}

func (e *DeviceCollectExecutor) createTaskRawOutput(taskID, deviceIP string, result *executor.CommandResult, rawPath, normalizedPath string) {
	if result == nil {
		return
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
	_ = e.db.Create(&TaskRawOutput{
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

func (e *ParseExecutor) parseAndSaveRunDevice(runID, deviceIP, vendor string) error {
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
		parsePath := strings.TrimSpace(output.ParseFilePath)
		if parsePath == "" {
			_ = e.db.Model(&TaskRawOutput{}).Where("id = ?", output.ID).
				Updates(map[string]interface{}{"parse_status": "skipped", "parse_error": "parse file path empty"}).Error
			continue
		}
		rawText, err := os.ReadFile(parsePath)
		if err != nil {
			_ = e.db.Model(&TaskRawOutput{}).Where("id = ?", output.ID).
				Updates(map[string]interface{}{"parse_status": "parse_failed", "parse_error": err.Error()}).Error
			continue
		}
		rows, err := e.parserEngine.Parse(output.CommandKey, string(rawText))
		if err != nil {
			_ = e.db.Model(&TaskRawOutput{}).Where("id = ?", output.ID).
				Updates(map[string]interface{}{"parse_status": "parse_failed", "parse_error": err.Error()}).Error
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
		_ = e.db.Model(&TaskRawOutput{}).Where("id = ?", output.ID).
			Updates(map[string]interface{}{"parse_status": parseStatus, "parse_error": parseError}).Error
	}

	identity.Vendor = normalize.NormalizeVendor(identity.Vendor)
	identity.Hostname = normalize.NormalizeDeviceName(identity.Hostname)

	return e.db.Transaction(func(tx *gorm.DB) error {
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

func (e *DeviceCollectExecutor) createArtifact(taskRunID, stageID, unitID, artifactType, artifactKey, filePath string) {
	_ = e.db.Create(&TaskArtifact{
		ID:           uuid.New().String()[:8],
		TaskRunID:    taskRunID,
		StageID:      stageID,
		UnitID:       unitID,
		ArtifactType: artifactType,
		ArtifactKey:  artifactKey,
		FilePath:     filePath,
		CreatedAt:    time.Now(),
	}).Error
}

func (e *ParseExecutor) createArtifact(taskRunID, stageID, unitID, artifactType, artifactKey, filePath string) {
	_ = e.db.Create(&TaskArtifact{
		ID:           uuid.New().String()[:8],
		TaskRunID:    taskRunID,
		StageID:      stageID,
		UnitID:       unitID,
		ArtifactType: artifactType,
		ArtifactKey:  artifactKey,
		FilePath:     filePath,
		CreatedAt:    time.Now(),
	}).Error
}

func (e *TopologyBuildExecutor) createArtifact(taskRunID, stageID, unitID, artifactType, artifactKey, filePath string) {
	_ = e.db.Create(&TaskArtifact{
		ID:           uuid.New().String()[:8],
		TaskRunID:    taskRunID,
		StageID:      stageID,
		UnitID:       unitID,
		ArtifactType: artifactType,
		ArtifactKey:  artifactKey,
		FilePath:     filePath,
		CreatedAt:    time.Now(),
	}).Error
}
