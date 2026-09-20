package taskexec

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/NetWeaverGo/core/internal/alarm"
	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/executor"
	"github.com/NetWeaverGo/core/internal/inspection"
	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/metrics"
	"github.com/NetWeaverGo/core/internal/models"
	"github.com/NetWeaverGo/core/internal/parser"
	"github.com/NetWeaverGo/core/internal/report"
	"github.com/NetWeaverGo/core/internal/repository"
	"gorm.io/gorm"
)

// InspectionCheckExecutor 巡检规则判定执行器
type InspectionCheckExecutor struct {
	repo           repository.DeviceRepository
	pathManager    *config.PathManager
	db             *gorm.DB
	settings       *models.GlobalSettings
	parserProvider parser.ParserProvider
}

// NewInspectionCheckExecutor 创建巡检执行器
func NewInspectionCheckExecutor(repo repository.DeviceRepository, db *gorm.DB, parserProvider parser.ParserProvider) *InspectionCheckExecutor {
	settings, _, _ := config.LoadSettings()
	return &InspectionCheckExecutor{
		repo:           repo,
		pathManager:    config.GetPathManager(),
		db:             db,
		settings:       settings,
		parserProvider: parserProvider,
	}
}

// Kind 返回支持的阶段类型
func (e *InspectionCheckExecutor) Kind() string {
	return string(StageKindInspectionCheck)
}

// Run 执行巡检判定阶段
func (e *InspectionCheckExecutor) Run(ctx RuntimeContext, stage *StagePlan) error {
	logger.Info("InspectionCheckExecutor", ctx.RunID(), "开始执行设备巡检阶段: stage=%s, units=%d, concurrency=%d", stage.Name, len(stage.Units), stage.Concurrency)

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
				handler.MarkUnitCancelled(ctx, u.ID, u.Target.Key, "run cancelled before inspection unit start", intPtrLocal(0))
				mu.Lock()
				cancelledCount++
				mu.Unlock()
				applyProjectedStageProgress(handler, ctx, stage.ID, len(stage.Units), completedCount, failedCount, cancelledCount, stageProgressFromCounts(len(stage.Units), completedCount, failedCount, cancelledCount), "更新巡检阶段进度")
				return
			}

			err := e.executeInspectionUnit(ctx, stage.ID, &u)

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
				emitProjectedUnitEvent(ctx, stage.ID, u.ID, EventTypeUnitFinished, EventLevelWarn, fmt.Sprintf("Inspection cancelled for %s", u.Target.Key))
			} else if err != nil {
				emitProjectedUnitEvent(ctx, stage.ID, u.ID, EventTypeUnitFinished, EventLevelError, fmt.Sprintf("Inspection failed for %s: %v", u.Target.Key, err))
			} else {
				emitProjectedUnitEvent(ctx, stage.ID, u.ID, EventTypeUnitFinished, EventLevelInfo, fmt.Sprintf("Inspection completed for %s", u.Target.Key))
			}

			applyProjectedStageProgress(handler, ctx, stage.ID, len(stage.Units), localCompleted, localFailed, localCancelled, stageProgressFromCounts(len(stage.Units), localCompleted, localFailed, localCancelled), "更新巡检阶段进度")
			mu.Unlock()
		}(unit)
	}

	wg.Wait()
	logger.Info("InspectionCheckExecutor", ctx.RunID(), "Inspection stage finished: completed=%d, failed=%d, cancelled=%d", completedCount, failedCount, cancelledCount)
	if ctx.IsCancelled() {
		return ctx.Context().Err()
	}
	return firstErr
}

// executeCheckOnly 三阶段编排下的纯判定路径：消费采集/解析阶段产出的内存快照，不连接设备。
// 该路径保证"采集完成后连接即释放"，且单台设备采集失败不会阻断其他设备判定。
func (e *InspectionCheckExecutor) executeCheckOnly(
	ctx RuntimeContext,
	stageID string,
	unit *UnitPlan,
	deviceIP string,
	holder RunDataHolder,
	handler *ErrorHandler,
) error {
	taskID := ctx.RunID()

	templateID := ""
	for _, st := range unit.Steps {
		if st.Params != nil && st.Params["templateId"] != "" {
			templateID = st.Params["templateId"]
			break
		}
	}
	if templateID == "" {
		templateID = "tpl-huawei-general"
	}

	items := e.loadInspectionItems(templateID)
	if len(items) == 0 {
		errMsg := fmt.Sprintf("模板 [%s] 下无启用的检查项", templateID)
		failUnitExecution(handler, ctx, unit.ID, deviceIP, errMsg, "写入无检查项状态", nil)
		return fmt.Errorf("%s", errMsg)
	}

	if !holder.HasDeviceEchoes(deviceIP) {
		errMsg := "前置采集无数据（该设备在采集阶段失败）"
		failUnitExecution(handler, ctx, unit.ID, deviceIP, errMsg, "写入巡检Unit失败状态", nil)
		return fmt.Errorf("%s", errMsg)
	}

	// P0-6：三阶段纯判定路径同样基于内存回显快照执行告警匹配，保证两种编排模式的告警数据一致
	family := "COMMON"
	if e.repo != nil {
		if dev, devErr := e.repo.FindByIP(deviceIP); devErr == nil && dev != nil {
			family = resolveAlarmFamily(dev.Model)
		}
	}
	var matchedAlarms []models.AlarmRecord
	if alarmReg := alarm.GetDefaultRegistry(); alarmReg != nil {
		seenCmd := make(map[string]struct{}, len(items))
		for _, it := range items {
			cmd := strings.TrimSpace(it.CommandKey)
			if cmd == "" {
				continue
			}
			if _, ok := seenCmd[cmd]; ok {
				continue
			}
			seenCmd[cmd] = struct{}{}

			echo, ok := holder.GetCommandEcho(deviceIP, cmd)
			if !ok || echo == "" {
				continue
			}
			for _, line := range strings.Split(echo, "\n") {
				trimmedLine := strings.TrimSpace(line)
				if trimmedLine == "" {
					continue
				}
				for _, hit := range alarmReg.MatchLine(family, trimmedLine) {
					matchedAlarms = append(matchedAlarms, models.AlarmRecord{
						RunID:      taskID,
						DeviceIP:   deviceIP,
						Family:     family,
						AlarmName:  hit.AlarmName,
						Severity:   hit.Severity,
						Category:   hit.Category,
						Summary:    hit.Description,
						RawEcho:    trimmedLine,
						Status:     "active",
						OccurredAt: time.Now(),
						CreatedAt:  time.Now(),
					})
				}
			}
		}
	}

	results := make([]models.InspectionResult, 0, len(items))
	for _, it := range items {
		cmd := strings.TrimSpace(it.CommandKey)
		itemCopy := it

		rows, _ := holder.GetParsedRows(deviceIP, cmd)
		parsedRows := make([]map[string]interface{}, 0, len(rows))
		for _, r := range rows {
			m := make(map[string]interface{}, len(r))
			for k, v := range r {
				m[k] = v
			}
			parsedRows = append(parsedRows, m)
		}
		echo, _ := holder.GetCommandEcho(deviceIP, cmd)

		evalRes := inspection.EvaluateItem(&inspection.EvaluateInput{
			RunID:      taskID,
			DeviceIP:   deviceIP,
			Item:       &itemCopy,
			RawEcho:    echo,
			ParsedRows: parsedRows,
		})
		results = append(results, evalRes)
		metrics.Default.LabelInc(taskID, "inspection.result_code", string(evalRes.Status))
	}

	// 结果持久化（与单阶段路径一致的幂等清理 + 批量插入）
	if e.db != nil {
		errTx := e.db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Where("run_id = ? AND device_ip = ?", taskID, deviceIP).Delete(&models.InspectionResult{}).Error; err != nil {
				return err
			}
			if len(results) > 0 {
				if err := tx.CreateInBatches(results, 100).Error; err != nil {
					return err
				}
			}
			if len(matchedAlarms) > 0 {
				if err := tx.CreateInBatches(matchedAlarms, 100).Error; err != nil {
					return err
				}
			}
			return nil
		})
		if errTx != nil {
			logger.Error("InspectionCheckExecutor", taskID, "写入 InspectionResult 与 AlarmRecord 数据库事务失败: %v", errTx)
		} else if len(matchedAlarms) > 0 {
			if _, mergeErr := alarm.PersistMergedPhenomena(e.db, nil, matchedAlarms); mergeErr != nil {
				logger.Warn("InspectionCheckExecutor", taskID, "告警归并持久化失败: %v", mergeErr)
			}
		}
	}

	// 产物登记：与单阶段路径 executeInspectionUnit 对齐。
	// 三阶段纯判定路径此前只写库、不登记产物，导致任务产物列表缺少原始回显与报表导出文件。
	e.registerCheckOnlyArtifacts(taskID, stageID, unit.ID, deviceIP, items, holder, results)

	return completeUnitExecution(handler, ctx, unit.ID, string(UnitStatusCompleted), len(results), "巡检判定完成", deviceIP)
}

// resolveAlarmFamily 将设备型号映射为内置告警规则的产品族（P0-6）。
// 注意：内置规则族名为 "Router"（并非 AR/NE），AR/NE 型号必须统一映射到 Router，
// 否则 4 条 Router 专属规则永远无法命中。
func resolveAlarmFamily(model string) string {
	mUpper := strings.ToUpper(strings.TrimSpace(model))
	switch {
	case mUpper == "":
		return "COMMON"
	case strings.HasPrefix(mUpper, "CE"):
		return "CE"
	case strings.HasPrefix(mUpper, "AR"), strings.HasPrefix(mUpper, "NE"):
		return "Router"
	case strings.HasPrefix(mUpper, "USG"):
		return "USG"
	case strings.HasPrefix(mUpper, "S"):
		return "S"
	}
	return "COMMON"
}

// executeInspectionUnit 执行单台设备的指标采集与规则判定
func (e *InspectionCheckExecutor) executeInspectionUnit(ctx RuntimeContext, stageID string, unit *UnitPlan) error {
	handler := NewErrorHandler(ctx.RunID())
	deviceIP := unit.Target.Key
	if unit.InitialStatus == string(UnitStatusUnsupported) {
		logger.Info("TaskExec", ctx.RunID(), "设备 %s 不支持巡检能力，跳过执行: %s", deviceIP, unit.ErrorMessage)
		emitProjectedUnitEvent(ctx, stageID, unit.ID, EventTypeUnitFinished, EventLevelInfo, fmt.Sprintf("设备不受支持已跳过: %s", unit.ErrorMessage))
		return nil
	}
	if ctx.IsCancelled() {
		return cancelUnitExecution(ctx, handler, unit.ID, deviceIP, "run cancelled before inspection unit start", intPtrLocal(0))
	}

	if err := markUnitRunning(handler, ctx, unit.ID, "设置巡检Unit为running"); err != nil {
		return err
	}
	if unit.Target.Type != "device_ip" {
		errMsg := fmt.Sprintf("unsupported target type: %s", unit.Target.Type)
		failUnitExecution(handler, ctx, unit.ID, deviceIP, errMsg, "写入巡检Unit失败状态", nil)
		return fmt.Errorf("%s", errMsg)
	}
	scope := LogScope{RunID: ctx.RunID(), StageID: stageID, UnitID: unit.ID, UnitKey: deviceIP}
	runtimeLogger := ctx.Logger(scope)
	logSession := runtimeLogger.Session(scope)

	device, err := e.repo.FindByIP(deviceIP)
	if err != nil {
		errMsg := fmt.Sprintf("device not found: %v", err)
		failUnitExecution(handler, ctx, unit.ID, deviceIP, errMsg, "写入巡检设备不存在状态", nil)
		projectTaskexecLifecycleRecord(ctx, runtimeLogger, scope, recordDeviceMissing, fmt.Sprintf("巡检设备不存在: %v", err), 0, 0)
		return fmt.Errorf("device not found: %w", err)
	}

	// 三阶段编排：若采集/解析阶段已产出内存快照，则本阶段只做判定，不再建立设备连接
	checkHolder := GetRunData(ctx.RunID())
	if checkHolder.HasAnyData() {
		return e.executeCheckOnly(ctx, stageID, unit, deviceIP, checkHolder, handler)
	}

	// 1. 建立设备连接
	opts := executor.ExecutorOptions{
		Vendor:     device.Vendor,
		Protocol:   device.Protocol,
		LogSession: logSession,
		RunID:      ctx.RunID(),
		Charset:    device.Charset,
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
	projectTaskexecLifecycleRecord(ctx, runtimeLogger, scope, recordSessionConnecting, "开始建立巡检采集连接", 0, 0)

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
	projectTaskexecLifecycleRecord(ctx, runtimeLogger, scope, recordSessionConnected, "巡检采集连接成功", 0, 0)

	if ctx.IsCancelled() {
		return cancelUnitExecution(ctx, handler, unit.ID, deviceIP, "run cancelled before collect commands", intPtrLocal(0))
	}

	// 2. 加载待执行检查项
	templateID := ""
	for _, st := range unit.Steps {
		if st.Params != nil && st.Params["templateId"] != "" {
			templateID = st.Params["templateId"]
			break
		}
	}
	if templateID == "" {
		templateID = "tpl-huawei-general"
	}

	items := e.loadInspectionItems(templateID)
	if len(items) == 0 {
		errMsg := fmt.Sprintf("模板 [%s] 下无启用的检查项", templateID)
		failUnitExecution(handler, ctx, unit.ID, deviceIP, errMsg, "写入无检查项状态", nil)
		return fmt.Errorf("%s", errMsg)
	}

	// 3. 命令归类与前置执行（去重，同一个命令只执行一次）
	commandEchos := make(map[string]string)
	parsedData := make(map[string][]map[string]interface{})
	contextVars := make(map[string]string)
	var matchedAlarms []models.AlarmRecord
	alarmReg := alarm.GetDefaultRegistry()

	family := resolveAlarmFamily(device.Model)

	// 先按 IsPreCollect 排序保证前置采集项优先执行
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].IsPreCollect != items[j].IsPreCollect {
			return items[i].IsPreCollect
		}
		return items[i].Order < items[j].Order
	})

	taskID := ctx.RunID()
	for _, it := range items {
		cmd := strings.TrimSpace(it.CommandKey)
		if cmd == "" {
			continue
		}
		if _, cached := commandEchos[cmd]; cached {
			continue // 已执行并缓存，复用回显
		}

		if ctx.IsCancelled() {
			return cancelUnitExecution(ctx, handler, unit.ID, deviceIP, "run cancelled during command execution", intPtrLocal(0))
		}

		emitProjectedUnitEvent(ctx, stageID, unit.ID, EventTypeStepStarted, EventLevelInfo, fmt.Sprintf("执行巡检命令: %s", cmd))
		echo, cmdErr := exec.ExecuteCommandSync(ctx.Context(), cmd, cmdTimeout)
		if cmdErr != nil {
			logger.Warn("InspectionCheckExecutor", taskID, "设备 %s 执行命令 [%s] 发生异常: %v", deviceIP, cmd, cmdErr)
		}
		commandEchos[cmd] = echo

		// 前置采集规约变量提取
		if it.IsPreCollect {
			contextVars[it.Code] = echo
			dslInterp := inspection.GetGlobalDSLInterpreter()
			if dslInterp != nil {
				if r := dslInterp.FindRule(it.Code); r != nil {
					for _, pc := range r.PreCollects {
						if k, v := dslInterp.ExecutePreCollect(&pc, echo); k != "" {
							contextVars[k] = v
						}
					}
				}
			}
		}

		// 告警规则匹配收集
		if echo != "" && alarmReg != nil {
			lines := strings.Split(echo, "\n")
			for _, line := range lines {
				trimmedLine := strings.TrimSpace(line)
				if trimmedLine == "" {
					continue
				}
				hits := alarmReg.MatchLine(family, trimmedLine)
				for _, hit := range hits {
					matchedAlarms = append(matchedAlarms, models.AlarmRecord{
						RunID:      taskID,
						DeviceIP:   deviceIP,
						Family:     family,
						AlarmName:  hit.AlarmName,
						Severity:   hit.Severity,
						Category:   hit.Category,
						Summary:    hit.Description,
						RawEcho:    trimmedLine,
						Status:     "active",
						OccurredAt: time.Now(),
						CreatedAt:  time.Now(),
					})
				}
			}
		}

		// 保存原始回显文件产物（P0-1：落盘前先执行分厂商脱敏，避免明文口令写入磁盘）
		sanitizedEcho := report.SanitizeContent(device.Vendor, device.Model, cmd, echo)
		rawPath := e.pathManager.GetInspectionRawFilePath(taskID, deviceIP, strings.ReplaceAll(cmd, " ", "_")+".txt")
		if err := os.MkdirAll(filepath.Dir(rawPath), 0755); err == nil {
			_ = os.WriteFile(rawPath, []byte(sanitizedEcho), 0644)
		}
		_ = e.createArtifactWithResult(taskID, stageID, unit.ID, string(ArtifactTypeRawOutput), fmt.Sprintf("%s:%s", deviceIP, cmd), rawPath)

		// 尝试进行结构化解析
		if e.parserProvider != nil && echo != "" {
			vendor := device.Vendor
			if vendor == "" {
				vendor = "huawei"
			}
			cliParser, pErr := e.parserProvider.GetParserForDevice(vendor, device.Model, device.Version)
			if pErr == nil && cliParser != nil {
				if rows, parseErr := parseWithMetrics(taskID, cliParser, cmd, echo); parseErr == nil && len(rows) > 0 {
					parsedRows := make([]map[string]interface{}, len(rows))
					for idx, r := range rows {
						m := make(map[string]interface{}, len(r))
						for k, v := range r {
							m[k] = v
						}
						parsedRows[idx] = m
					}
					parsedData[cmd] = parsedRows
				}
			}
		}
	}

	// 4. 调用判定引擎执行规则评估
	results := make([]models.InspectionResult, 0, len(items))
	passCount, failCount, warnCount := 0, 0, 0

	for _, it := range items {
		cmd := strings.TrimSpace(it.CommandKey)
		itemCopy := it
		input := &inspection.EvaluateInput{
			RunID:       taskID,
			DeviceIP:    deviceIP,
			Item:        &itemCopy,
			RawEcho:     commandEchos[cmd],
			ParsedRows:  parsedData[cmd],
			ContextVars: contextVars,
		}
		evalRes := inspection.EvaluateItem(input)
		results = append(results, evalRes)
		// 巡检结果码分布（方案 §10.2）
		metrics.Default.LabelInc(taskID, "inspection.result_code", string(evalRes.Status))

		switch evalRes.Status {
		case string(inspection.ResultPass):
			passCount++
		case string(inspection.ResultFail), string(inspection.ResultUnaccord):
			failCount++
		case string(inspection.ResultWarning):
			warnCount++
		}
	}

	// 5. 结果持久化与产物登记（事务保证，清理历史并批量插入）
	if e.db != nil {
		errTx := e.db.Transaction(func(tx *gorm.DB) error {
			// 按当前运行 ID 与设备 IP 幂等清理本轮历史结果，保留历史 run 的报告记录
			if err := tx.Where("run_id = ? AND device_ip = ?", taskID, deviceIP).Delete(&models.InspectionResult{}).Error; err != nil {
				return err
			}
			if len(results) > 0 {
				if err := tx.CreateInBatches(results, 100).Error; err != nil {
					return err
				}
			}
			if len(matchedAlarms) > 0 {
				if err := tx.CreateInBatches(matchedAlarms, 100).Error; err != nil {
					return err
				}
			}
			return nil
		})
		if errTx != nil {
			logger.Error("InspectionCheckExecutor", taskID, "写入 InspectionResult 与 AlarmRecord 数据库事务失败: %v", errTx)
		} else if len(matchedAlarms) > 0 && e.db != nil {
			// P0-6：将原始告警归并为故障现象并持久化（幂等；失败不阻断巡检主流程）
			if _, mergeErr := alarm.PersistMergedPhenomena(e.db, nil, matchedAlarms); mergeErr != nil {
				logger.Warn("InspectionCheckExecutor", taskID, "告警归并持久化失败: %v", mergeErr)
			}
		}
	}

	// 6. 生成巡检报表产物（CSV 与 JSON，P0-1：落盘前统一脱敏）
	csvData, errCSV := inspection.ExportInspectionResultsCSV(results)
	if errCSV == nil {
		csvData = report.SanitizeContent(device.Vendor, device.Model, "", csvData)
		csvPath := e.pathManager.GetInspectionRawFilePath(taskID, deviceIP, "inspection_report.csv")
		if err := os.MkdirAll(filepath.Dir(csvPath), 0755); err == nil {
			_ = os.WriteFile(csvPath, []byte(csvData), 0644)
		}
		_ = e.createArtifactWithResult(taskID, stageID, unit.ID, string(ArtifactTypeInspectionReport), fmt.Sprintf("%s:inspection_report.csv", deviceIP), csvPath)
	}

	jsonData, errJSON := inspection.ExportInspectionResultsJSON(results)
	if errJSON == nil {
		jsonData = report.SanitizeContent(device.Vendor, device.Model, "", jsonData)
		jsonPath := e.pathManager.GetInspectionRawFilePath(taskID, deviceIP, "inspection_report.json")
		if err := os.MkdirAll(filepath.Dir(jsonPath), 0755); err == nil {
			_ = os.WriteFile(jsonPath, []byte(jsonData), 0644)
		}
		_ = e.createArtifactWithResult(taskID, stageID, unit.ID, string(ArtifactTypeInspectionReport), fmt.Sprintf("%s:inspection_report.json", deviceIP), jsonPath)
	}

	unitStatus := string(UnitStatusCompleted)
	if failCount > 0 && passCount == 0 {
		unitStatus = string(UnitStatusFailed)
	} else if failCount > 0 || warnCount > 0 {
		unitStatus = string(UnitStatusPartial)
	}

	if err := completeUnitExecution(handler, ctx, unit.ID, unitStatus, len(items), "巡检规则判定完成", deviceIP); err != nil {
		return err
	}

	emitProjectedUnitEvent(ctx, stageID, unit.ID, EventTypeUnitFinished, EventLevelInfo,
		fmt.Sprintf("设备 %s 巡检完成: 通过 %d 项, 不合格 %d 项, 警告 %d 项", deviceIP, passCount, failCount, warnCount))
	return nil
}

func (e *InspectionCheckExecutor) loadInspectionItems(templateID string) []models.InspectionItem {
	return ResolveInspectionItems(e.db, templateID)
}

func (e *InspectionCheckExecutor) createArtifactWithResult(taskRunID, stageID, unitID, artifactType, artifactKey, filePath string) error {
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
		logger.Warn("InspectionCheckExecutor", taskRunID, "保存产物记录失败: err=%v, artifact=%+v", err, artifact)
		return err
	}
	return nil
}

// registerCheckOnlyArtifacts 在三阶段纯判定路径下补齐产物登记，使其与单阶段路径
// executeInspectionUnit 的产物可见性一致：
//  1. raw_output：各检查项命令的原始回显（采集阶段虽已落盘，但仍需登记产物记录）；
//  2. inspection_report.csv / inspection_report.json：巡检报表导出产物。
//
// 任何写盘或登记失败仅告警，不影响判定结果（与单阶段路径保持一致）。
func (e *InspectionCheckExecutor) registerCheckOnlyArtifacts(
	taskID, stageID, unitID, deviceIP string,
	items []models.InspectionItem,
	holder RunDataHolder,
	results []models.InspectionResult,
) {
	// 1. 原始回显产物（按命令去重）
	seen := make(map[string]struct{}, len(items))
	for _, it := range items {
		cmd := strings.TrimSpace(it.CommandKey)
		if cmd == "" {
			continue
		}
		if _, ok := seen[cmd]; ok {
			continue
		}
		seen[cmd] = struct{}{}

		echo, ok := holder.GetCommandEcho(deviceIP, cmd)
		if !ok {
			continue
		}
		// P0-1：三阶段纯判定路径无设备画像上下文，按通用规则脱敏后落盘
		sanitizedEcho := report.SanitizeContent("", "", cmd, echo)
		rawPath := e.pathManager.GetInspectionRawFilePath(taskID, deviceIP, strings.ReplaceAll(cmd, " ", "_")+".txt")
		if err := os.MkdirAll(filepath.Dir(rawPath), 0755); err == nil {
			_ = os.WriteFile(rawPath, []byte(sanitizedEcho), 0644)
		}
		_ = e.createArtifactWithResult(taskID, stageID, unitID, string(ArtifactTypeRawOutput), fmt.Sprintf("%s:%s", deviceIP, cmd), rawPath)
	}

	// 2. 巡检报表产物（CSV 与 JSON，P0-1：落盘前统一脱敏）
	if csvData, err := inspection.ExportInspectionResultsCSV(results); err == nil {
		csvData = report.SanitizeContent("", "", "", csvData)
		csvPath := e.pathManager.GetInspectionRawFilePath(taskID, deviceIP, "inspection_report.csv")
		if err := os.MkdirAll(filepath.Dir(csvPath), 0755); err == nil {
			_ = os.WriteFile(csvPath, []byte(csvData), 0644)
		}
		_ = e.createArtifactWithResult(taskID, stageID, unitID, string(ArtifactTypeInspectionReport), fmt.Sprintf("%s:inspection_report.csv", deviceIP), csvPath)
	}
	if jsonData, err := inspection.ExportInspectionResultsJSON(results); err == nil {
		jsonData = report.SanitizeContent("", "", "", jsonData)
		jsonPath := e.pathManager.GetInspectionRawFilePath(taskID, deviceIP, "inspection_report.json")
		if err := os.MkdirAll(filepath.Dir(jsonPath), 0755); err == nil {
			_ = os.WriteFile(jsonPath, []byte(jsonData), 0644)
		}
		_ = e.createArtifactWithResult(taskID, stageID, unitID, string(ArtifactTypeInspectionReport), fmt.Sprintf("%s:inspection_report.json", deviceIP), jsonPath)
	}
}
