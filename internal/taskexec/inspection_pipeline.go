package taskexec

import (
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

// ============================================================================
// 巡检三阶段编排（方案 §5.3）
//
// 目标：采集完成后立即释放设备连接；解析与判定离线进行；
// 中间产物经内存快照传递（零中间表，避免 SQLite 写入放大）。
// 设备级容错：仅当"全量设备失败"时 Stage 才返回错误，避免单台故障拖垮整批。
// ============================================================================

// InspectionCollectExecutor 巡检采集阶段执行器（连接在此阶段结束即释放）
type InspectionCollectExecutor struct {
	repo        repository.DeviceRepository
	pathManager *config.PathManager
	settings    *models.GlobalSettings
}

// NewInspectionCollectExecutor 创建巡检采集执行器
func NewInspectionCollectExecutor(repo repository.DeviceRepository) *InspectionCollectExecutor {
	settings, _, _ := config.LoadSettings()
	return &InspectionCollectExecutor{
		repo:        repo,
		pathManager: config.GetPathManager(),
		settings:    settings,
	}
}

// Kind 返回支持的阶段类型
func (e *InspectionCollectExecutor) Kind() string { return string(StageKindInspectionCollect) }

// Run 执行采集阶段：仅全量设备失败或取消时返回错误
func (e *InspectionCollectExecutor) Run(ctx RuntimeContext, stage *StagePlan) error {
	logger.Info("InspectionCollectExecutor", ctx.RunID(),
		"开始巡检采集阶段: stage=%s, units=%d, concurrency=%d", stage.Name, len(stage.Units), stage.Concurrency)

	concurrency := stage.Concurrency
	if concurrency <= 0 {
		concurrency = 10
	}

	handler := NewErrorHandler(ctx.RunID())
	semaphore := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var completedCount, failedCount, cancelledCount int
	var firstErr error

loop:
	for _, unit := range stage.Units {
		if ctx.IsCancelled() {
			break
		}
		wg.Add(1)
		select {
		case semaphore <- struct{}{}:
		case <-ctx.Context().Done():
			wg.Done()
			break loop
		}

		go func(u UnitPlan) {
			defer wg.Done()
			defer func() { <-semaphore }()

			if ctx.IsCancelled() {
				handler.MarkUnitCancelled(ctx, u.ID, u.Target.Key, "run cancelled before collect unit start", intPtrLocal(0))
				mu.Lock()
				cancelledCount++
				mu.Unlock()
				return
			}

			err := e.executeCollectUnit(ctx, stage.ID, &u)

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
			localCompleted, localFailed, localCancelled := completedCount, failedCount, cancelledCount
			mu.Unlock()

			level := EventLevelInfo
			msg := fmt.Sprintf("巡检采集完成: %s", u.Target.Key)
			if err != nil {
				level = EventLevelError
				msg = fmt.Sprintf("巡检采集失败: %s, err=%v", u.Target.Key, err)
			}
			emitProjectedUnitEvent(ctx, stage.ID, u.ID, EventTypeUnitFinished, level, msg)
			applyProjectedStageProgress(handler, ctx, stage.ID, len(stage.Units),
				localCompleted, localFailed, localCancelled,
				stageProgressFromCounts(len(stage.Units), localCompleted, localFailed, localCancelled),
				"更新巡检采集阶段进度")
		}(unit)
	}

	wg.Wait()

	logger.Info("InspectionCollectExecutor", ctx.RunID(),
		"巡检采集阶段结束: success=%d, failed=%d, cancelled=%d", completedCount, failedCount, cancelledCount)

	if ctx.IsCancelled() {
		return ctx.Context().Err()
	}
	// 设备级容错：只要有一台成功，阶段即视为可继续（状态由 Unit 投影推导为 partial）
	if completedCount == 0 && (failedCount > 0 || cancelledCount > 0) {
		if firstErr == nil {
			firstErr = fmt.Errorf("巡检采集阶段全量设备失败")
		}
		return firstErr
	}
	return nil
}

func (e *InspectionCollectExecutor) executeCollectUnit(ctx RuntimeContext, stageID string, unit *UnitPlan) error {
	handler := NewErrorHandler(ctx.RunID())
	deviceIP := unit.Target.Key
	if unit.Target.Type != "device_ip" {
		return failCollectUnit(handler, ctx, unit, deviceIP, "unsupported target type")
	}
	if err := markUnitRunning(handler, ctx, unit.ID, "设置巡检采集Unit为running"); err != nil {
		return err
	}

	device, err := e.repo.FindByIP(deviceIP)
	if err != nil {
		return failCollectUnit(handler, ctx, unit, deviceIP, fmt.Sprintf("device not found: %v", err))
	}

	taskID := ctx.RunID()
	scope := LogScope{RunID: taskID, StageID: stageID, UnitID: unit.ID, UnitKey: deviceIP}
	logSession := ctx.Logger(scope).Session(scope)

	exec := executor.NewDeviceExecutor(device.IP, device.Port, device.Username, device.Password,
		executor.ExecutorOptions{Vendor: device.Vendor, Protocol: device.Protocol, LogSession: logSession, RunID: taskID})
	defer exec.Close()

	connTimeout, cmdTimeout := 30*time.Second, unit.Timeout
	if cmdTimeout <= 0 {
		cmdTimeout = 60 * time.Second
	}
	if e.settings != nil && e.settings.ConnectTimeout != "" {
		if d, err := time.ParseDuration(e.settings.ConnectTimeout); err == nil {
			connTimeout = d
		}
	}

	if err := exec.Connect(ctx.Context(), connTimeout); err != nil {
		if IsContextCancelled(ctx, err) {
			return cancelUnitExecution(ctx, handler, unit.ID, deviceIP, "run cancelled during connect", intPtrLocal(0))
		}
		return failCollectUnit(handler, ctx, unit, deviceIP, fmt.Sprintf("连接失败: %v", err))
	}

	holder := GetRunData(taskID)
	successSteps := 0
	for _, step := range unit.Steps {
		if ctx.IsCancelled() {
			return cancelUnitExecution(ctx, handler, unit.ID, deviceIP, "run cancelled during collect", intPtrLocal(0))
		}
		cmd := strings.TrimSpace(step.Command)
		if cmd == "" {
			continue
		}
		echo, cmdErr := exec.ExecuteCommandSync(ctx.Context(), cmd, cmdTimeout)
		if cmdErr != nil {
			logger.Warn("InspectionCollectExecutor", taskID, "设备 %s 执行命令 [%s] 异常: %v", deviceIP, cmd, cmdErr)
		} else {
			// 仅统计实际成功执行的命令数，避免个别命令异常时 doneSteps 被计满
			successSteps++
		}
		holder.SetCommandEcho(deviceIP, cmd, echo)

		rawPath := e.pathManager.GetInspectionRawFilePath(taskID, deviceIP, strings.ReplaceAll(cmd, " ", "_")+".txt")
		if err := os.MkdirAll(filepath.Dir(rawPath), 0755); err == nil {
			_ = os.WriteFile(rawPath, []byte(echo), 0644)
		}
	}

	// 进度精度：doneSteps 反映实际成功命令数（设备级容错策略不变，Unit 终态仍为 Completed）
	return completeUnitExecution(handler, ctx, unit.ID, string(UnitStatusCompleted), successSteps, "巡检采集完成", deviceIP)
}

func failCollectUnit(handler *ErrorHandler, ctx RuntimeContext, unit *UnitPlan, deviceIP, errMsg string) error {
	failUnitExecution(handler, ctx, unit.ID, deviceIP, errMsg, "写入巡检采集Unit失败状态", nil)
	return fmt.Errorf("%s", errMsg)
}

// InspectionParseExecutor 巡检解析阶段执行器（离线解析，不占用设备连接）
type InspectionParseExecutor struct {
	db             *gorm.DB
	repo           repository.DeviceRepository
	pathManager    *config.PathManager
	parserProvider parser.ParserProvider
}

// NewInspectionParseExecutor 创建巡检解析执行器
func NewInspectionParseExecutor(db *gorm.DB, repo repository.DeviceRepository, provider parser.ParserProvider) *InspectionParseExecutor {
	return &InspectionParseExecutor{
		db:             db,
		repo:           repo,
		pathManager:    config.GetPathManager(),
		parserProvider: provider,
	}
}

// Kind 返回支持的阶段类型
func (e *InspectionParseExecutor) Kind() string { return string(StageKindInspectionParse) }

// Run 执行解析阶段：逐设备独立，单设备无数据不影响其他设备
func (e *InspectionParseExecutor) Run(ctx RuntimeContext, stage *StagePlan) error {
	logger.Info("InspectionParseExecutor", ctx.RunID(),
		"开始巡检解析阶段: stage=%s, units=%d", stage.Name, len(stage.Units))

	handler := NewErrorHandler(ctx.RunID())
	var wg sync.WaitGroup
	var mu sync.Mutex
	var completedCount, failedCount int
	var firstErr error

	for _, unit := range stage.Units {
		if ctx.IsCancelled() {
			break
		}
		wg.Add(1)
		go func(u UnitPlan) {
			defer wg.Done()

			err := e.executeParseUnit(ctx, stage.ID, &u)

			mu.Lock()
			if err != nil {
				failedCount++
				if firstErr == nil {
					firstErr = err
				}
			} else {
				completedCount++
			}
			localCompleted, localFailed := completedCount, failedCount
			mu.Unlock()

			level := EventLevelInfo
			msg := fmt.Sprintf("巡检解析完成: %s", u.Target.Key)
			if err != nil {
				level = EventLevelWarn
				msg = fmt.Sprintf("巡检解析跳过: %s, reason=%v", u.Target.Key, err)
			}
			emitProjectedUnitEvent(ctx, stage.ID, u.ID, EventTypeUnitFinished, level, msg)
			applyProjectedStageProgress(handler, ctx, stage.ID, len(stage.Units),
				localCompleted, localFailed, 0,
				stageProgressFromCounts(len(stage.Units), localCompleted, localFailed, 0),
				"更新巡检解析阶段进度")
		}(unit)
	}
	wg.Wait()

	logger.Info("InspectionParseExecutor", ctx.RunID(),
		"巡检解析阶段结束: success=%d, skipped=%d", completedCount, failedCount)

	if ctx.IsCancelled() {
		return ctx.Context().Err()
	}
	// 解析阶段同样是设备级容错，仅全量失败才返回错误
	if completedCount == 0 && failedCount > 0 {
		if firstErr == nil {
			firstErr = fmt.Errorf("巡检解析阶段全量设备无数据")
		}
		return firstErr
	}
	return nil
}

func (e *InspectionParseExecutor) executeParseUnit(ctx RuntimeContext, stageID string, unit *UnitPlan) error {
	handler := NewErrorHandler(ctx.RunID())
	deviceIP := unit.Target.Key
	if err := markUnitRunning(handler, ctx, unit.ID, "设置巡检解析Unit为running"); err != nil {
		return err
	}
	if unit.Target.Type != "device_ip" {
		return failParseUnit(handler, ctx, unit, deviceIP, "unsupported target type")
	}

	taskID := ctx.RunID()
	holder := GetRunData(taskID)
	if !holder.HasDeviceEchoes(deviceIP) {
		return failParseUnit(handler, ctx, unit, deviceIP, "前置采集无数据（该设备采集失败）")
	}

	device, err := e.repo.FindByIP(deviceIP)
	vendor := ""
	if err == nil && device != nil {
		vendor = device.Vendor
	}
	if vendor == "" {
		vendor = "huawei"
	}
	model, version := "", ""
	if err == nil && device != nil {
		model, version = device.Model, device.Version
	}

	parsedCommands := 0
	for _, step := range unit.Steps {
		cmd := strings.TrimSpace(step.Command)
		if cmd == "" {
			continue
		}
		echo, ok := holder.GetCommandEcho(deviceIP, cmd)
		if !ok || strings.TrimSpace(echo) == "" {
			// 内存超限或缺失时回退读取落盘原始回显
			rawPath := e.pathManager.GetInspectionRawFilePath(taskID, deviceIP, strings.ReplaceAll(cmd, " ", "_")+".txt")
			if data, readErr := os.ReadFile(rawPath); readErr == nil {
				echo, ok = string(data), true
			}
		}
		if !ok || echo == "" || e.parserProvider == nil {
			continue
		}
		cliParser, pErr := e.parserProvider.GetParserForDevice(vendor, model, version)
		if pErr != nil || cliParser == nil {
			continue
		}
		rows, _ := parseWithMetrics(taskID, cliParser, cmd, echo)
		if len(rows) > 0 {
			holder.SetParsedRows(deviceIP, cmd, rows)
			parsedCommands++
		}
	}

	logger.Debug("InspectionParseExecutor", taskID, "设备 %s 解析完成命令数=%d", deviceIP, parsedCommands)
	return completeUnitExecution(handler, ctx, unit.ID, string(UnitStatusCompleted), parsedCommands, "巡检解析完成", deviceIP)
}

func failParseUnit(handler *ErrorHandler, ctx RuntimeContext, unit *UnitPlan, deviceIP, errMsg string) error {
	failUnitExecution(handler, ctx, unit.ID, deviceIP, errMsg, "写入巡检解析Unit失败状态", nil)
	return fmt.Errorf("%s", errMsg)
}
