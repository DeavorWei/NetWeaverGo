package taskexec

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/NetWeaverGo/core/internal/ceas"
	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/executor"
	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
	"github.com/NetWeaverGo/core/internal/repository"
	"gorm.io/gorm"
)

// CEASExecutor CEAS硬件清单与批次预警执行器
type CEASExecutor struct {
	repo        repository.DeviceRepository
	pathManager *config.PathManager
	db          *gorm.DB
	settings    *models.GlobalSettings
}

// NewCEASExecutor 创建CEAS执行器
func NewCEASExecutor(repo repository.DeviceRepository, db *gorm.DB) *CEASExecutor {
	settings, _, _ := config.LoadSettings()
	return &CEASExecutor{
		repo:        repo,
		pathManager: config.GetPathManager(),
		db:          db,
		settings:    settings,
	}
}

// Kind 返回支持的阶段类型
func (e *CEASExecutor) Kind() string {
	return string(StageKindCEASCollect)
}

// Run 执行CEAS硬件清单阶段
func (e *CEASExecutor) Run(ctx RuntimeContext, stage *StagePlan) error {
	logger.Info("CEASExecutor", ctx.RunID(), "开始执行CEAS硬件采集阶段: stage=%s, units=%d, concurrency=%d", stage.Name, len(stage.Units), stage.Concurrency)

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
				handler.MarkUnitCancelled(ctx, u.ID, u.Target.Key, "run cancelled before ceas unit start", intPtrLocal(0))
				mu.Lock()
				cancelledCount++
				mu.Unlock()
				applyProjectedStageProgress(handler, ctx, stage.ID, len(stage.Units), completedCount, failedCount, cancelledCount, stageProgressFromCounts(len(stage.Units), completedCount, failedCount, cancelledCount), "更新CEAS阶段进度")
				return
			}

			err := e.executeCEASUnit(ctx, stage.ID, &u)

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

			if IsContextCancelled(ctx, err) {
				emitProjectedUnitEvent(ctx, stage.ID, u.ID, EventTypeUnitFinished, EventLevelWarn, fmt.Sprintf("CEAS cancelled for %s", u.Target.Key))
			} else if err != nil {
				emitProjectedUnitEvent(ctx, stage.ID, u.ID, EventTypeUnitFinished, EventLevelError, fmt.Sprintf("CEAS failed for %s: %v", u.Target.Key, err))
			} else {
				emitProjectedUnitEvent(ctx, stage.ID, u.ID, EventTypeUnitFinished, EventLevelInfo, fmt.Sprintf("CEAS completed for %s", u.Target.Key))
			}

			applyProjectedStageProgress(handler, ctx, stage.ID, len(stage.Units), localCompleted, localFailed, localCancelled, stageProgressFromCounts(len(stage.Units), localCompleted, localFailed, localCancelled), "更新CEAS阶段进度")
			mu.Unlock()
		}(unit)
	}

	wg.Wait()
	logger.Info("CEASExecutor", ctx.RunID(), "CEAS stage completed: success=%d, failed=%d, cancelled=%d", completedCount, failedCount, cancelledCount)
	if ctx.IsCancelled() {
		return ctx.Context().Err()
	}
	return firstErr
}

// executeCEASUnit 执行单台设备的CEAS采集
func (e *CEASExecutor) executeCEASUnit(ctx RuntimeContext, stageID string, unit *UnitPlan) error {
	handler := NewErrorHandler(ctx.RunID())
	deviceIP := unit.Target.Key
	if ctx.IsCancelled() {
		return cancelUnitExecution(ctx, handler, unit.ID, deviceIP, "run cancelled before ceas unit start", intPtrLocal(0))
	}

	if err := markUnitRunning(handler, ctx, unit.ID, "设置CEAS采集Unit为running"); err != nil {
		return err
	}
	if unit.Target.Type != "device_ip" {
		errMsg := fmt.Sprintf("unsupported target type: %s", unit.Target.Type)
		failUnitExecution(handler, ctx, unit.ID, deviceIP, errMsg, "写入CEAS采集Unit失败状态", nil)
		return fmt.Errorf("%s", errMsg)
	}
	scope := LogScope{RunID: ctx.RunID(), StageID: stageID, UnitID: unit.ID, UnitKey: deviceIP}
	runtimeLogger := ctx.Logger(scope)
	logSession := runtimeLogger.Session(scope)

	device, err := e.repo.FindByIP(deviceIP)
	if err != nil {
		errMsg := fmt.Sprintf("device not found: %v", err)
		failUnitExecution(handler, ctx, unit.ID, deviceIP, errMsg, "写入CEAS设备不存在状态", nil)
		projectTaskexecLifecycleRecord(ctx, runtimeLogger, scope, recordDeviceMissing, fmt.Sprintf("CEAS采集设备不存在: %v", err), 0, 0)
		return fmt.Errorf("device not found: %w", err)
	}

	opts := executor.ExecutorOptions{
		Vendor:     device.Vendor,
		Protocol:   device.Protocol,
		LogSession: logSession,
	}
	exec := executor.NewDeviceExecutor(
		device.IP,
		device.Port,
		device.Username,
		device.Password,
		opts,
	)
	defer exec.Close()

	connTimeout := 30 * time.Second
	cmdTimeout := unit.Timeout
	if cmdTimeout <= 0 {
		cmdTimeout = 60 * time.Second
	}
	if e.settings != nil && e.settings.ConnectTimeout != "" {
		if d, err := time.ParseDuration(e.settings.ConnectTimeout); err == nil {
			connTimeout = d
		}
	}

	emitProjectedUnitEvent(ctx, stageID, unit.ID, EventTypeUnitStarted, EventLevelInfo, fmt.Sprintf("正在连接设备 %s...", deviceIP))
	projectTaskexecLifecycleRecord(ctx, runtimeLogger, scope, recordSessionConnecting, "开始建立CEAS采集连接", 0, 0)

	if err := exec.Connect(ctx.Context(), connTimeout); err != nil {
		if IsContextCancelled(ctx, err) {
			return cancelUnitExecution(ctx, handler, unit.ID, deviceIP, "run cancelled during connect", intPtrLocal(0))
		}
		errMsg := fmt.Sprintf("连接失败: %v", err)
		failUnitExecution(handler, ctx, unit.ID, deviceIP, errMsg, "写入连接失败状态", nil)
		projectTaskexecLifecycleRecord(ctx, runtimeLogger, scope, recordSessionConnectFailed, errMsg, 0, 0)
		emitProjectedUnitEvent(ctx, stageID, unit.ID, EventTypeUnitFinished, EventLevelError, errMsg)
		return fmt.Errorf("连接失败: %w", err)
	}
	projectTaskexecLifecycleRecord(ctx, runtimeLogger, scope, recordSessionConnected, "CEAS采集连接成功", 0, 0)

	if ctx.IsCancelled() {
		return cancelUnitExecution(ctx, handler, unit.ID, deviceIP, "run cancelled before collect elabel", intPtrLocal(0))
	}

	// 1. 采集电子标签
	emitProjectedUnitEvent(ctx, stageID, unit.ID, EventTypeStepStarted, EventLevelInfo, "正在采集电子标签(display elabel)...")
	elabelCmd := "display elabel"
	elabelOutput, err := exec.ExecuteCommandSync(ctx.Context(), elabelCmd, cmdTimeout)
	if err != nil || strings.Contains(elabelOutput, "Unrecognized command") || strings.Contains(elabelOutput, "Wrong parameter") {
		// 尝试 fallback 1: display device elabel
		if fallbackOutput, fallbackErr := exec.ExecuteCommandSync(ctx.Context(), "display device elabel", cmdTimeout); fallbackErr == nil && !strings.Contains(fallbackOutput, "Unrecognized command") {
			elabelOutput = fallbackOutput
			elabelCmd = "display device elabel"
			err = nil
		} else {
			// 尝试 fallback 2: 提权至 diagnose 视图执行 (对应 eDeskPro viewname=diagnose)
			_, _ = exec.ExecuteCommandSync(ctx.Context(), "system-view", 10*time.Second)
			_, _ = exec.ExecuteCommandSync(ctx.Context(), "diagnose", 10*time.Second)
			if diagOutput, diagErr := exec.ExecuteCommandSync(ctx.Context(), "display elabel", cmdTimeout); diagErr == nil && !strings.Contains(diagOutput, "Unrecognized command") && len(diagOutput) > 30 {
				elabelOutput = diagOutput
				elabelCmd = "diagnose:display elabel"
				err = nil
			}
			_, _ = exec.ExecuteCommandSync(ctx.Context(), "quit", 5*time.Second)
			_, _ = exec.ExecuteCommandSync(ctx.Context(), "return", 5*time.Second)
		}
	}
	if err != nil {
		errMsg := fmt.Sprintf("执行电子标签命令失败: %v", err)
		failUnitExecution(handler, ctx, unit.ID, deviceIP, errMsg, "写入命令失败状态", nil)
		return fmt.Errorf("执行电子标签命令失败: %w", err)
	}

	// 2. 采集 ESN
	emitProjectedUnitEvent(ctx, stageID, unit.ID, EventTypeStepStarted, EventLevelInfo, "正在采集设备序列号(display esn)...")
	esnOutput, _ := exec.ExecuteCommandSync(ctx.Context(), "display esn", 30*time.Second)

	// 3. 产物持久化（原始回显文本保存）
	taskID := ctx.RunID()
	elabelPath := e.pathManager.GetCEASRawFilePath(taskID, deviceIP, "display_elabel")
	if err := os.MkdirAll(filepath.Dir(elabelPath), 0755); err == nil {
		_ = os.WriteFile(elabelPath, []byte(elabelOutput), 0644)
	}
	_ = e.createArtifactWithResult(taskID, stageID, unit.ID, string(ArtifactTypeCEASCmdEcho), fmt.Sprintf("%s:display_elabel", deviceIP), elabelPath)

	if strings.TrimSpace(esnOutput) != "" {
		esnPath := e.pathManager.GetCEASRawFilePath(taskID, deviceIP, "display_esn")
		if err := os.MkdirAll(filepath.Dir(esnPath), 0755); err == nil {
			_ = os.WriteFile(esnPath, []byte(esnOutput), 0644)
		}
		_ = e.createArtifactWithResult(taskID, stageID, unit.ID, string(ArtifactTypeCEASCmdEcho), fmt.Sprintf("%s:display_esn", deviceIP), esnPath)
	}

	// 4. 解析 elabel 与提取 ESN
	tree := ceas.ParseELabel(elabelOutput)
	tree.DeviceIP = deviceIP
	modelToUse := device.ModelSeries
	if modelToUse == "" {
		modelToUse = device.Model
	}
	esn := ceas.ExtractESN(device.Vendor, modelToUse, elabelOutput, esnOutput)
	if esn != "" {
		tree.ChassisESN = esn
	} else {
		esn = tree.ChassisESN
	}

	// 4.1 产物持久化：ceas_data (硬件树 JSON) 与 ceas_baseinfo (设备基础信息)
	treeJSON, errTree := json.Marshal(tree)
	if errTree == nil {
		dataPath := e.pathManager.GetCEASRawFilePath(taskID, deviceIP, "ceas_data")
		if err := os.MkdirAll(filepath.Dir(dataPath), 0755); err == nil {
			_ = os.WriteFile(dataPath, treeJSON, 0644)
		}
		_ = e.createArtifactWithResult(taskID, stageID, unit.ID, string(ArtifactTypeCEASData), fmt.Sprintf("%s:ceas_data", deviceIP), dataPath)
	}

	baseInfo := map[string]interface{}{
		"device_ip":    deviceIP,
		"vendor":       device.Vendor,
		"model":        device.Model,
		"model_series": device.ModelSeries,
		"esn":          esn,
		"chassis_esn":  tree.ChassisESN,
		"node_count":   len(tree.AllNodes),
		"collected_at": time.Now().Format(time.RFC3339),
	}
	baseInfoJSON, errBase := json.Marshal(baseInfo)
	if errBase == nil {
		baseInfoPath := e.pathManager.GetCEASRawFilePath(taskID, deviceIP, "ceas_baseinfo")
		if err := os.MkdirAll(filepath.Dir(baseInfoPath), 0755); err == nil {
			_ = os.WriteFile(baseInfoPath, baseInfoJSON, 0644)
		}
		_ = e.createArtifactWithResult(taskID, stageID, unit.ID, string(ArtifactTypeCEASBaseInfo), fmt.Sprintf("%s:ceas_baseinfo", deviceIP), baseInfoPath)
	}

	// 5. 写入 task_ceas_nodes 数据库表（事务隔离，按设备IP清理历史，避免多轮采集节点膨胀）
	if e.db != nil {
		dbNodes := make([]models.TaskCEASNode, 0, len(tree.AllNodes))
		for _, n := range tree.AllNodes {
			attrsJSON, _ := json.Marshal(n.Attrs)
			dbNodes = append(dbNodes, models.TaskCEASNode{
				TaskRunID:    taskID,
				DeviceIP:     deviceIP,
				NodeID:       n.ID,
				ParentID:     n.ParentID,
				Level:        n.Level,
				Type:         n.Type,
				Name:         n.Name,
				Path:         n.Path,
				Slot:         n.Slot,
				Item:         n.Item,
				BarCode:      n.BarCode,
				Description:  n.Description,
				Manufactured: n.Manufactured,
				VendorName:   n.VendorName,
				BoardType:    n.BoardType,
				AttrsJSON:    string(attrsJSON),
			})
		}

		errTx := e.db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Where("device_ip = ?", deviceIP).Delete(&models.TaskCEASNode{}).Error; err != nil {
				return err
			}
			if len(dbNodes) > 0 {
				if err := tx.CreateInBatches(dbNodes, 100).Error; err != nil {
					return err
				}
			}
			return nil
		})
		if errTx != nil {
			logger.Error("CEASExecutor", taskID, "写入 TaskCEASNode 数据库事务失败: %v", errTx)
		}

		// 回写资产与运行设备 ESN
		if esn != "" {
			if err := e.db.Model(&models.DeviceAsset{}).Where("ip = ?", deviceIP).Update("esn", esn).Error; err != nil {
				logger.Warn("CEASExecutor", taskID, "更新设备资产 ESN 失败: %v", err)
			}
			_ = e.db.Model(&TaskRunDevice{}).Where("task_run_id = ? AND device_ip = ?", taskID, deviceIP).Update("esn", esn).Error
		}

		// 6. BOM 白名单比对与预警产物
		var watchlist []models.BOMWatchlistItem
		if err := e.db.Where("enabled = ?", true).Find(&watchlist).Error; err == nil && len(watchlist) > 0 {
			alerts := ceas.GetInvolvedSlots(tree, watchlist)
			if len(alerts) > 0 {
				logger.Warn("CEASExecutor", taskID, "设备 %s 命中 %d 条 BOM 批次预警", deviceIP, len(alerts))
				alertsJSON, _ := json.Marshal(alerts)
				alertsPath := e.pathManager.GetCEASRawFilePath(taskID, deviceIP, "bom_alerts")
				if err := os.MkdirAll(filepath.Dir(alertsPath), 0755); err == nil {
					_ = os.WriteFile(alertsPath, alertsJSON, 0644)
				}
				_ = e.createArtifactWithResult(taskID, stageID, unit.ID, string(ArtifactTypeCEASAlerts), fmt.Sprintf("%s:bom_alerts", deviceIP), alertsPath)
			}
		}
	}

	if err := completeUnitExecution(handler, ctx, unit.ID, string(UnitStatusCompleted), 1, "CEAS采集完成", deviceIP); err != nil {
		return err
	}

	emitProjectedUnitEvent(ctx, stageID, unit.ID, EventTypeUnitFinished, EventLevelInfo,
		fmt.Sprintf("设备 %s CEAS 采集完成: 发现节点 %d 个, ESN: %s", deviceIP, len(tree.AllNodes), esn))
	return nil
}

func (e *CEASExecutor) createArtifactWithResult(taskRunID, stageID, unitID, artifactType, artifactKey, filePath string) error {
	if e.db == nil {
		return nil
	}
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
		logger.Warn("CEASExecutor", taskRunID, "保存产物记录失败: err=%v, artifact=%+v", err, artifact)
		return err
	}
	return nil
}
