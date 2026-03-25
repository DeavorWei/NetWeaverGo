// Package taskexec provides a unified task execution runtime for NetWeaverGo.
//
// Architecture Overview:
//
// 1. Task Definition Layer
//   - TaskDefinition: Unified task definition with kind (normal/topology)
//   - NormalTaskConfig: Configuration for normal tasks (Mode A/B)
//   - TopologyTaskConfig: Configuration for topology tasks
//
// 2. Plan Compilation Layer
//   - PlanCompiler: Interface for compiling task definitions to execution plans
//   - NormalTaskCompiler: Compiles normal tasks to device_command stage
//   - TopologyTaskCompiler: Compiles topology tasks to collect->parse->build stages
//
// 3. Unified Runtime Layer
//   - RuntimeManager: Manages task lifecycle, stage execution, concurrency
//   - RuntimeContext: Provides state updates, event emission, cancellation
//   - EventBus: Unified event bus for task events
//   - SnapshotHub: Manages execution snapshots for frontend
//
// 4. Stage Executor Layer
//   - StageExecutor: Interface for stage execution
//   - DeviceCommandExecutor: Executes device commands (normal tasks)
//   - DeviceCollectExecutor: Collects device info (topology tasks)
//   - ParseExecutor: Parses collected data
//   - TopologyBuildExecutor: Builds topology graph
//
// 5. Data Layer
//   - TaskRun/Stage/Unit: Runtime state models
//   - ExecutionSnapshot: Unified snapshot for frontend
//   - Repository: Persistence interface
//
// Usage:
//
//	service := taskexec.NewTaskExecutionService(db)
//	service.Start()
//	defer service.Stop()
//
//	// Create and start a task
//	def, _ := service.CreateNormalTask("My Task", config)
//	runID, _ := service.StartTask(ctx, def)
//
//	// Monitor execution
//	snapshot, _ := service.GetSnapshot(runID)
//
// Migration Guide:
//
// See docs/REFACTOR_ROADMAP.md for detailed migration steps from old architecture.
package taskexec
