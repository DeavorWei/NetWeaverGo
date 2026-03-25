package taskexec

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/executor"
	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
	"github.com/NetWeaverGo/core/internal/parser"
	"github.com/NetWeaverGo/core/internal/repository"
	"github.com/NetWeaverGo/core/internal/topology"
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
	e.ensureDiscoveryDevice(ctx.RunID(), device)

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
		e.updateDiscoveryDeviceStatus(ctx.RunID(), deviceIP, "failed", errMsg)
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
		e.updateDiscoveryDeviceStatus(ctx.RunID(), deviceIP, "failed", errMsg)
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

			e.createRawCommandOutput(ctx.RunID(), deviceIP, result, rawPath, normalizedPath)
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
	e.updateDiscoveryDeviceStatus(ctx.RunID(), deviceIP, deviceStatus, "")

	logger.Debug("TaskExec", ctx.RunID(), "Collection completed for device: %s", deviceIP)
	return nil
}

// ParseExecutor for parsing collected data
type ParseExecutor struct {
	db     *gorm.DB
	parser *parser.Service
}

// NewParseExecutor creates executor
func NewParseExecutor(db *gorm.DB) *ParseExecutor {
	return &ParseExecutor{db: db, parser: parser.NewService(db)}
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

	// Get vendor from unit params or discover it
	vendor := ""
	if unit.Steps != nil && len(unit.Steps) > 0 {
		vendor = unit.Steps[0].Params["vendor"]
	}

	// Call parser service
	err := e.parser.ParseAndSaveTaskDevice(ctx.RunID(), deviceIP, vendor)
	if err != nil {
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
	db      *gorm.DB
	builder *topology.Builder
}

// NewTopologyBuildExecutor creates executor
func NewTopologyBuildExecutor(db *gorm.DB) *TopologyBuildExecutor {
	return &TopologyBuildExecutor{db: db, builder: topology.NewBuilder(db)}
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

	// Execute topology build
	result, err := e.builder.Build(ctx.RunID())
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

func (e *DeviceCollectExecutor) ensureDiscoveryDevice(taskID string, device *models.DeviceAsset) {
	now := time.Now()
	var record models.DiscoveryDevice
	err := e.db.Where("task_id = ? AND device_ip = ?", taskID, device.IP).First(&record).Error
	if err == nil {
		return
	}
	_ = e.db.Create(&models.DiscoveryDevice{
		TaskID:       taskID,
		DeviceIP:     device.IP,
		DeviceID:     device.ID,
		Status:       "running",
		StartedAt:    &now,
		Vendor:       device.Vendor,
		DisplayName:  device.DisplayName,
		Role:         device.Role,
		Site:         device.Site,
		CreatedAt:    now,
		UpdatedAt:    now,
	}).Error
}

func (e *DeviceCollectExecutor) updateDiscoveryDeviceStatus(taskID, deviceIP, status, errMsg string) {
	updates := map[string]interface{}{
		"status":        status,
		"error_message": errMsg,
		"updated_at":    time.Now(),
	}
	if status == "success" || status == "partial" || status == "failed" {
		now := time.Now()
		updates["finished_at"] = now
	}
	_ = e.db.Model(&models.DiscoveryDevice{}).
		Where("task_id = ? AND device_ip = ?", taskID, deviceIP).
		Updates(updates).Error
}

func (e *DeviceCollectExecutor) createRawCommandOutput(taskID, deviceIP string, result *executor.CommandResult, rawPath, normalizedPath string) {
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
	_ = e.db.Create(&models.RawCommandOutput{
		TaskID:          taskID,
		DeviceIP:        deviceIP,
		CommandKey:      result.CommandKey,
		Command:         result.Command,
		FilePath:        normalizedPath,
		RawFilePath:     rawPath,
		Status:          "success",
		ParseStatus:     "pending",
		RawSize:         rawSize,
		NormalizedSize:  normalizedSize,
		LineCount:       lineCount,
		EchoConsumed:    result.EchoConsumed,
		PromptMatched:   result.PromptMatched,
	}).Error
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
