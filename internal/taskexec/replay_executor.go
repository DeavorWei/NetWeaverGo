package taskexec

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/normalize"
	"github.com/NetWeaverGo/core/internal/parser"
	"gorm.io/gorm"
)

// =============================================================================
// 离线重放执行器
// 支持从历史Raw文件重新解析构建拓扑，无需连接设备
// =============================================================================

// ReplayExecutor 离线重放执行器
type ReplayExecutor struct {
	db             *gorm.DB
	parserProvider parser.ParserProvider
	pathManager    *config.PathManager
}

// NewReplayExecutor 创建重放执行器
func NewReplayExecutor(db *gorm.DB, parserProvider parser.ParserProvider) *ReplayExecutor {
	return &ReplayExecutor{
		db:             db,
		parserProvider: parserProvider,
		pathManager:    config.GetPathManager(),
	}
}

// Execute 执行重放流程
func (e *ReplayExecutor) Execute(ctx context.Context, originalRunID string, opts ReplayOptions) (*ReplayResult, error) {
	startedAt := time.Now()
	result := &ReplayResult{
		ReplayRunID: generateReplayRunID(originalRunID),
		Status:      string(ReplayStatusRunning),
		StartedAt:   startedAt,
		Errors:      []string{},
	}

	// 创建重放记录
	record := &TopologyReplayRecord{
		OriginalRunID: originalRunID,
		ReplayRunID:   result.ReplayRunID,
		Status:        string(ReplayStatusRunning),
		TriggerSource: "manual",
		StartedAt:     &startedAt,
	}
	if err := e.db.Create(record).Error; err != nil {
		return nil, fmt.Errorf("创建重放记录失败: %w", err)
	}

	// 更新函数
	updateRecord := func(status string, errMsg string) {
		now := time.Now()
		updates := map[string]interface{}{
			"status":      status,
			"finished_at": now,
		}
		if errMsg != "" {
			updates["error_message"] = errMsg
		}
		e.db.Model(&TopologyReplayRecord{}).Where("id = ?", record.ID).Updates(updates)
	}

	// 1. 如果指定了清除现有结果，先清除
	if opts.ClearExisting {
		logger.Info("Replay", result.ReplayRunID, "清除原有解析结果: originalRunID=%s", originalRunID)
		if err := e.clearExistingResults(originalRunID); err != nil {
			updateRecord(string(ReplayStatusFailed), err.Error())
			result.Status = string(ReplayStatusFailed)
			result.Errors = append(result.Errors, err.Error())
			return result, err
		}
	}

	// 2. 扫描Raw文件
	scanStart := time.Now()
	rawFiles, err := e.scanRawFiles(originalRunID)
	if err != nil {
		updateRecord(string(ReplayStatusFailed), err.Error())
		result.Status = string(ReplayStatusFailed)
		result.Errors = append(result.Errors, err.Error())
		return result, err
	}
	result.Statistics.ScanDurationMs = time.Since(scanStart).Milliseconds()
	result.Statistics.TotalRawFiles = len(rawFiles)

	// 统计设备数和命令类型数
	deviceSet := make(map[string]struct{})
	commandKeySet := make(map[string]struct{})
	for _, f := range rawFiles {
		deviceSet[f.DeviceIP] = struct{}{}
		commandKeySet[f.CommandKey] = struct{}{}
	}
	result.Statistics.TotalDevices = len(deviceSet)
	result.Statistics.TotalCommandKeys = len(commandKeySet)

	logger.Info("Replay", result.ReplayRunID, "扫描完成: rawFiles=%d, devices=%d, commandKeys=%d",
		len(rawFiles), result.Statistics.TotalDevices, result.Statistics.TotalCommandKeys)

	// 如果指定了设备列表，过滤
	if len(opts.DeviceIPs) > 0 {
		filterSet := make(map[string]struct{})
		for _, ip := range opts.DeviceIPs {
			filterSet[ip] = struct{}{}
		}
		filtered := make([]RawFileInfo, 0)
		for _, f := range rawFiles {
			if _, ok := filterSet[f.DeviceIP]; ok {
				filtered = append(filtered, f)
			}
		}
		rawFiles = filtered
		logger.Info("Replay", result.ReplayRunID, "按设备过滤后: rawFiles=%d", len(rawFiles))
	}

	if len(rawFiles) == 0 {
		err := fmt.Errorf("未找到可重放的Raw文件")
		updateRecord(string(ReplayStatusFailed), err.Error())
		result.Status = string(ReplayStatusFailed)
		result.Errors = append(result.Errors, err.Error())
		return result, err
	}

	// 3. 创建虚拟运行记录（如果不存在）
	if err := e.ensureVirtualRun(result.ReplayRunID, originalRunID); err != nil {
		updateRecord(string(ReplayStatusFailed), err.Error())
		result.Status = string(ReplayStatusFailed)
		result.Errors = append(result.Errors, err.Error())
		return result, err
	}

	// 4. 执行解析阶段
	parseStart := time.Now()
	parseErrors := e.executeParse(ctx, result.ReplayRunID, rawFiles, &result.Statistics)
	result.Statistics.ParseDurationMs = time.Since(parseStart).Milliseconds()
	if len(parseErrors) > 0 {
		result.Errors = append(result.Errors, parseErrors...)
	}

	// 检查是否取消
	select {
	case <-ctx.Done():
		updateRecord(string(ReplayStatusCancelled), "用户取消")
		result.Status = string(ReplayStatusCancelled)
		return result, ctx.Err()
	default:
	}

	// 5. 执行构建阶段（除非指定跳过）
	if !opts.SkipBuild {
		buildStart := time.Now()
		buildErr := e.executeBuild(ctx, result.ReplayRunID, &result.Statistics)
		result.Statistics.BuildDurationMs = time.Since(buildStart).Milliseconds()
		if buildErr != nil {
			result.Errors = append(result.Errors, buildErr.Error())
		}
	}

	// 6. 完成
	finishedAt := time.Now()
	result.Statistics.TotalDurationMs = time.Since(startedAt).Milliseconds()
	result.FinishedAt = finishedAt
	result.Status = string(ReplayStatusCompleted)

	// 更新重放记录
	statsJSON, _ := json.Marshal(result.Statistics)
	e.db.Model(&TopologyReplayRecord{}).Where("id = ?", record.ID).Updates(map[string]interface{}{
		"status":          string(ReplayStatusCompleted),
		"finished_at":     finishedAt,
		"statistics":      string(statsJSON),
		"parser_version":  opts.ParserVersion,
		"builder_version": "1.0",
	})

	logger.Info("Replay", result.ReplayRunID, "重放完成: status=%s, totalMs=%d, lldp=%d, fdb=%d, arp=%d, edges=%d",
		result.Status, result.Statistics.TotalDurationMs,
		result.Statistics.LLDPCount, result.Statistics.FDBCount, result.Statistics.ARPCount,
		result.Statistics.RetainedEdges)

	return result, nil
}

// scanRawFiles 扫描指定运行ID的所有Raw文件
func (e *ReplayExecutor) scanRawFiles(runID string) ([]RawFileInfo, error) {
	rawDir := filepath.Join(e.pathManager.TopologyRawDir, runID)

	// 检查目录是否存在
	if _, err := os.Stat(rawDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("Raw文件目录不存在: %s", rawDir)
	}

	var files []RawFileInfo
	err := filepath.Walk(rawDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		// 匹配 *_raw.txt 文件
		if strings.HasSuffix(info.Name(), "_raw.txt") {
			relPath, err := filepath.Rel(rawDir, path)
			if err != nil {
				return nil
			}
			parts := strings.Split(relPath, string(filepath.Separator))
			if len(parts) >= 2 {
				commandKey := strings.TrimSuffix(parts[1], "_raw.txt")
				files = append(files, RawFileInfo{
					DeviceIP:   parts[0],
					CommandKey: commandKey,
					FilePath:   path,
					FileSize:   info.Size(),
				})
			}
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("扫描Raw文件失败: %w", err)
	}

	return files, nil
}

// ensureVirtualRun 确保虚拟运行记录存在
func (e *ReplayExecutor) ensureVirtualRun(replayRunID, originalRunID string) error {
	// 检查是否已存在
	var count int64
	e.db.Model(&TaskRun{}).Where("id = ?", replayRunID).Count(&count)
	if count > 0 {
		return nil
	}

	// 获取原始运行记录
	var originalRun TaskRun
	if err := e.db.Where("id = ?", originalRunID).First(&originalRun).Error; err != nil {
		return fmt.Errorf("获取原始运行记录失败: %w", err)
	}

	// 创建虚拟运行记录
	now := time.Now()
	virtualRun := &TaskRun{
		ID:        replayRunID,
		Name:      fmt.Sprintf("[重放] %s", originalRun.Name),
		RunKind:   string(RunKindTopology),
		Status:    string(RunStatusRunning),
		Progress:  0,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := e.db.Create(virtualRun).Error; err != nil {
		return fmt.Errorf("创建虚拟运行记录失败: %w", err)
	}

	// 复制原始设备记录
	var originalDevices []TaskRunDevice
	if err := e.db.Where("task_run_id = ?", originalRunID).Find(&originalDevices).Error; err != nil {
		return fmt.Errorf("获取原始设备记录失败: %w", err)
	}

	for _, dev := range originalDevices {
		newDev := TaskRunDevice{
			TaskRunID:      replayRunID,
			DeviceID:       dev.DeviceID,
			DeviceIP:       dev.DeviceIP,
			Status:         "pending",
			Vendor:         dev.Vendor,
			VendorSource:   dev.VendorSource,
			DisplayName:    dev.DisplayName,
			Role:           dev.Role,
			Site:           dev.Site,
			Hostname:       dev.Hostname,
			Model:          dev.Model,
			SerialNumber:   dev.SerialNumber,
			Version:        dev.Version,
			MgmtIP:         dev.MgmtIP,
			NormalizedName: dev.NormalizedName,
			ChassisID:      dev.ChassisID,
			NodeType:       dev.NodeType,
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		if err := e.db.Create(&newDev).Error; err != nil {
			logger.Warn("Replay", replayRunID, "复制设备记录失败: device=%s, err=%v", dev.DeviceIP, err)
		}
	}

	return nil
}

// executeParse 执行解析阶段
func (e *ReplayExecutor) executeParse(ctx context.Context, replayRunID string, rawFiles []RawFileInfo, stats *ReplayStatistics) []string {
	var errors []string

	// 按设备分组
	deviceFiles := make(map[string][]RawFileInfo)
	for _, f := range rawFiles {
		deviceFiles[f.DeviceIP] = append(deviceFiles[f.DeviceIP], f)
	}

	stats.TotalDevices = len(deviceFiles)

	for deviceIP, files := range deviceFiles {
		select {
		case <-ctx.Done():
			errors = append(errors, "用户取消")
			return errors
		default:
		}

		logger.Verbose("Replay", replayRunID, "解析设备: device=%s, files=%d", deviceIP, len(files))

		// 获取设备厂商
		vendor := e.getDeviceVendor(replayRunID, deviceIP)
		if vendor == "" {
			vendor = "huawei" // 默认华为
		}

		// 获取解析器
		parserEngine, err := e.parserProvider.GetParser(vendor)
		if err != nil {
			errors = append(errors, fmt.Sprintf("设备 %s 获取解析器失败: %v", deviceIP, err))
			continue
		}

		mapper := parser.GetMapper(vendor)
		identity := &parser.DeviceIdentity{Vendor: vendor, MgmtIP: deviceIP}
		var interfaces []parser.InterfaceFact
		var lldps []parser.LLDPFact
		var fdbs []parser.FDBFact
		var arps []parser.ARPFact
		var aggs []parser.AggregateFact

		// 解析每个文件
		for _, file := range files {
			select {
			case <-ctx.Done():
				errors = append(errors, "用户取消")
				return errors
			default:
			}

			rawText, err := os.ReadFile(file.FilePath)
			if err != nil {
				errors = append(errors, fmt.Sprintf("读取文件失败: %s, %v", file.FilePath, err))
				stats.FailedCommands++
				continue
			}

			// 创建 TaskRawOutput 记录
			rawOutput := &TaskRawOutput{
				TaskRunID:      replayRunID,
				DeviceIP:       deviceIP,
				CommandKey:     file.CommandKey,
				RawFilePath:    file.FilePath,
				Status:         "success",
				ResolvedVendor: vendor,
				VendorSource:   "replay",
				RawSize:        file.FileSize,
				CreatedAt:      time.Now(),
				UpdatedAt:      time.Now(),
			}
			if err := e.db.Create(rawOutput).Error; err != nil {
				logger.Warn("Replay", replayRunID, "创建RawOutput记录失败: %v", err)
			}

			// 解析
			rows, err := parserEngine.Parse(file.CommandKey, string(rawText))
			if err != nil {
				errors = append(errors, fmt.Sprintf("解析失败: device=%s, cmd=%s, %v", deviceIP, file.CommandKey, err))
				stats.FailedCommands++
				rawOutput.ParseStatus = "parse_failed"
				rawOutput.ParseError = err.Error()
				e.db.Save(rawOutput)
				continue
			}

			stats.ParsedCommands++

			// 映射解析结果
			switch file.CommandKey {
			case "version", "device_info":
				id, mapErr := mapper.ToDeviceInfo(rows)
				if mapErr != nil {
					logger.Warn("Replay", replayRunID, "映射设备信息失败: %v", mapErr)
				} else {
					if id.Vendor != "" {
						identity.Vendor = id.Vendor
					}
					if id.Hostname != "" {
						identity.Hostname = id.Hostname
					}
					if id.Model != "" {
						identity.Model = id.Model
					}
					if id.SerialNumber != "" {
						identity.SerialNumber = id.SerialNumber
					}
					if id.Version != "" {
						identity.Version = id.Version
					}
					if id.ChassisID != "" {
						identity.ChassisID = id.ChassisID
					}
					if id.MgmtIP != "" {
						identity.MgmtIP = id.MgmtIP
					}
				}

			case "interface_brief", "interface_detail":
				items, mapErr := mapper.ToInterfaces(rows)
				if mapErr != nil {
					logger.Warn("Replay", replayRunID, "映射接口失败: %v", mapErr)
				} else {
					interfaces = append(interfaces, items...)
				}

			case "lldp_neighbor", "lldp_neighbor_verbose":
				items, mapErr := mapper.ToLLDP(rows)
				if mapErr != nil {
					logger.Warn("Replay", replayRunID, "映射LLDP失败: %v", mapErr)
				} else {
					lldps = append(lldps, items...)
				}

			case "mac_address", "fdb":
				items, mapErr := mapper.ToFDB(rows)
				if mapErr != nil {
					logger.Warn("Replay", replayRunID, "映射FDB失败: %v", mapErr)
				} else {
					fdbs = append(fdbs, items...)
				}

			case "arp_all", "arp":
				items, mapErr := mapper.ToARP(rows)
				if mapErr != nil {
					logger.Warn("Replay", replayRunID, "映射ARP失败: %v", mapErr)
				} else {
					arps = append(arps, items...)
				}

			case "eth_trunk", "eth_trunk_verbose":
				items, mapErr := mapper.ToAggregate(rows)
				if mapErr != nil {
					logger.Warn("Replay", replayRunID, "映射聚合失败: %v", mapErr)
				} else {
					aggs = append(aggs, items...)
				}
			}

			rawOutput.ParseStatus = "success"
			e.db.Save(rawOutput)
		}

		// 更新设备信息
		identity.Vendor = normalize.NormalizeVendor(identity.Vendor)
		identity.Hostname = normalize.NormalizeDeviceName(identity.Hostname)

		// 保存解析结果到数据库
		if err := e.saveParsedResults(replayRunID, deviceIP, identity, interfaces, lldps, fdbs, arps, aggs); err != nil {
			errors = append(errors, fmt.Sprintf("保存解析结果失败: device=%s, %v", deviceIP, err))
			continue
		}

		stats.ParsedDevices++
		stats.LLDPCount += len(lldps)
		stats.FDBCount += len(fdbs)
		stats.ARPCount += len(arps)
		stats.InterfaceCount += len(interfaces)

		logger.Verbose("Replay", replayRunID, "设备解析完成: device=%s, vendor=%s, lldp=%d, fdb=%d, arp=%d, if=%d",
			deviceIP, identity.Vendor, len(lldps), len(fdbs), len(arps), len(interfaces))
	}

	return errors
}

// getDeviceVendor 获取设备厂商
func (e *ReplayExecutor) getDeviceVendor(runID, deviceIP string) string {
	var dev TaskRunDevice
	if err := e.db.Where("task_run_id = ? AND device_ip = ?", runID, deviceIP).First(&dev).Error; err == nil {
		return strings.ToLower(strings.TrimSpace(dev.Vendor))
	}
	return ""
}

// saveParsedResults 保存解析结果到数据库
func (e *ReplayExecutor) saveParsedResults(runID, deviceIP string, identity *parser.DeviceIdentity,
	interfaces []parser.InterfaceFact, lldps []parser.LLDPFact, fdbs []parser.FDBFact,
	arps []parser.ARPFact, aggs []parser.AggregateFact) error {
	return e.db.Transaction(func(tx *gorm.DB) error {
		// 更新设备信息
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
				"status":          "completed",
			}).Error; err != nil {
			return err
		}

		// 清理旧数据
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

		// 保存接口
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
				CreatedAt:     time.Now(),
				UpdatedAt:     time.Now(),
			}).Error; err != nil {
				return err
			}
		}

		// 保存LLDP
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
				CreatedAt:       time.Now(),
				UpdatedAt:       time.Now(),
			}).Error; err != nil {
				return err
			}
		}

		// 保存FDB
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
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
			}).Error; err != nil {
				return err
			}
		}

		// 保存ARP
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
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
			}).Error; err != nil {
				return err
			}
		}

		// 保存聚合成员（AggregateFact.MemberPorts是数组）
		for _, agg := range aggs {
			for _, memberPort := range agg.MemberPorts {
				if err := tx.Create(&TaskParsedAggregateMember{
					TaskRunID:     runID,
					DeviceIP:      deviceIP,
					AggregateName: agg.AggregateName,
					MemberPort:    memberPort,
					CommandKey:    agg.CommandKey,
					CreatedAt:     time.Now(),
					UpdatedAt:     time.Now(),
				}).Error; err != nil {
					return err
				}
			}
			// 保存聚合组信息
			if err := tx.Create(&TaskParsedAggregateGroup{
				TaskRunID:     runID,
				DeviceIP:      deviceIP,
				AggregateName: agg.AggregateName,
				Mode:          agg.Mode,
				CreatedAt:     time.Now(),
				UpdatedAt:     time.Now(),
			}).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

// clearExistingResults 清除原有解析结果
func (e *ReplayExecutor) clearExistingResults(runID string) error {
	logger.Info("Replay", runID, "清除原有解析结果")

	return e.db.Transaction(func(tx *gorm.DB) error {
		// 删除解析结果
		if err := tx.Where("task_run_id = ?", runID).Delete(&TaskParsedLLDPNeighbor{}).Error; err != nil {
			return fmt.Errorf("删除LLDP解析结果失败: %w", err)
		}
		if err := tx.Where("task_run_id = ?", runID).Delete(&TaskParsedFDBEntry{}).Error; err != nil {
			return fmt.Errorf("删除FDB解析结果失败: %w", err)
		}
		if err := tx.Where("task_run_id = ?", runID).Delete(&TaskParsedARPEntry{}).Error; err != nil {
			return fmt.Errorf("删除ARP解析结果失败: %w", err)
		}
		if err := tx.Where("task_run_id = ?", runID).Delete(&TaskParsedInterface{}).Error; err != nil {
			return fmt.Errorf("删除接口解析结果失败: %w", err)
		}
		if err := tx.Where("task_run_id = ?", runID).Delete(&TaskParsedAggregateMember{}).Error; err != nil {
			return fmt.Errorf("删除聚合成员解析结果失败: %w", err)
		}
		if err := tx.Where("task_run_id = ?", runID).Delete(&TaskParsedAggregateGroup{}).Error; err != nil {
			return fmt.Errorf("删除聚合组解析结果失败: %w", err)
		}

		// 删除拓扑结果
		if err := tx.Where("task_run_id = ?", runID).Delete(&TaskTopologyEdge{}).Error; err != nil {
			return fmt.Errorf("删除拓扑边失败: %w", err)
		}
		if err := tx.Where("task_run_id = ?", runID).Delete(&TopologyEdgeCandidate{}).Error; err != nil {
			return fmt.Errorf("删除拓扑候选失败: %w", err)
		}
		if err := tx.Where("task_run_id = ?", runID).Delete(&TopologyDecisionTrace{}).Error; err != nil {
			return fmt.Errorf("删除决策轨迹失败: %w", err)
		}

		// 删除Raw输出记录
		if err := tx.Where("task_run_id = ?", runID).Delete(&TaskRawOutput{}).Error; err != nil {
			return fmt.Errorf("删除Raw输出记录失败: %w", err)
		}

		logger.Info("Replay", runID, "原有解析结果清除完成")
		return nil
	})
}

// executeBuild 执行构建阶段
func (e *ReplayExecutor) executeBuild(ctx context.Context, replayRunID string, stats *ReplayStatistics) error {
	logger.Info("Replay", replayRunID, "开始拓扑构建阶段")

	// 使用现有的 TopologyBuilder
	builder := NewTopologyBuilder(e.db, DefaultTopologyBuildConfig())
	output, err := builder.Build(ctx, replayRunID)
	if err != nil {
		return fmt.Errorf("拓扑构建失败: %w", err)
	}

	stats.TotalCandidates = len(output.Candidates)
	stats.RetainedEdges = len(output.Edges)
	stats.RejectedEdges = stats.TotalCandidates - stats.RetainedEdges

	// 统计冲突边
	for _, c := range output.Candidates {
		if c.Status == "conflict" {
			stats.ConflictEdges++
		}
	}

	// 更新运行状态
	e.db.Model(&TaskRun{}).Where("id = ?", replayRunID).Updates(map[string]interface{}{
		"status":     string(RunStatusCompleted),
		"progress":   100,
		"updated_at": time.Now(),
	})

	logger.Info("Replay", replayRunID, "拓扑构建完成: candidates=%d, edges=%d, conflicts=%d",
		stats.TotalCandidates, stats.RetainedEdges, stats.ConflictEdges)

	return nil
}

// generateReplayRunID 生成重放运行ID
func generateReplayRunID(originalRunID string) string {
	return fmt.Sprintf("replay_%s_%d", originalRunID, time.Now().UnixMilli())
}

// ListReplayableRuns 列出可重放的运行记录
func (e *ReplayExecutor) ListReplayableRuns(limit int) ([]ReplayableRunInfo, error) {
	var runs []TaskRun
	if err := e.db.Where("run_kind = ?", string(RunKindTopology)).
		Order("created_at DESC").
		Limit(limit).
		Find(&runs).Error; err != nil {
		return nil, err
	}

	var result []ReplayableRunInfo
	for _, run := range runs {
		info := ReplayableRunInfo{
			RunID:       run.ID,
			TaskName:    run.Name,
			Status:      run.Status,
			RunKind:     run.RunKind,
			CreatedAt:   run.CreatedAt,
			HasRawFiles: e.hasRawFiles(run.ID),
		}

		// 统计设备数
		var deviceCount int64
		e.db.Model(&TaskRunDevice{}).Where("task_run_id = ?", run.ID).Count(&deviceCount)
		info.DeviceCount = int(deviceCount)

		// 统计Raw文件数
		if info.HasRawFiles {
			info.RawFileCount = e.countRawFiles(run.ID)
		}

		result = append(result, info)
	}

	return result, nil
}

// hasRawFiles 检查是否有Raw文件
func (e *ReplayExecutor) hasRawFiles(runID string) bool {
	rawDir := filepath.Join(e.pathManager.TopologyRawDir, runID)
	if _, err := os.Stat(rawDir); os.IsNotExist(err) {
		return false
	}
	return true
}

// countRawFiles 统计Raw文件数量
func (e *ReplayExecutor) countRawFiles(runID string) int {
	rawDir := filepath.Join(e.pathManager.TopologyRawDir, runID)
	count := 0
	filepath.Walk(rawDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), "_raw.txt") {
			count++
		}
		return nil
	})
	return count
}

// GetReplayHistory 获取重放历史
func (e *ReplayExecutor) GetReplayHistory(originalRunID string) ([]TopologyReplayRecord, error) {
	var records []TopologyReplayRecord
	if err := e.db.Where("original_run_id = ?", originalRunID).
		Order("created_at DESC").
		Find(&records).Error; err != nil {
		return nil, err
	}
	return records, nil
}
