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
			case "version":
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
					if id.ChassisID != "" {
						identity.ChassisID = id.ChassisID
					}
					if id.MgmtIP != "" {
						identity.MgmtIP = id.MgmtIP
					}
				}
	
				case "sysname":
					// 合并身份字段（hostname、mgmt_ip、chassis_id等）
					for _, row := range rows {
						if v, ok := row["hostname"]; ok {
							if s := strings.TrimSpace(v); s != "" {
								identity.Hostname = s
							}
						}
						if v, ok := row["sysname"]; ok {
							if s := strings.TrimSpace(v); s != "" {
								identity.Hostname = s
							}
						}
						if v, ok := row["mgmt_ip"]; ok {
							if s := strings.TrimSpace(v); s != "" {
								identity.MgmtIP = s
							}
						}
						if v, ok := row["management_ip"]; ok {
							if s := strings.TrimSpace(v); s != "" {
								identity.MgmtIP = s
							}
						}
						if v, ok := row["chassis_id"]; ok {
							if s := strings.TrimSpace(v); s != "" {
								identity.ChassisID = s
							}
						}
					}
	
				case "interface_brief":
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

// =============================================================================
// 对比功能
// =============================================================================

// CompareReplayResults 对比两次运行的解析结果
func (e *ReplayExecutor) CompareReplayResults(runID1, runID2 string) (*ParseResultDiff, error) {
	diff := &ParseResultDiff{
		RunID1: runID1,
		RunID2: runID2,
	}

	// 对比 LLDP
	lldp1, err := e.getLLDPEntries(runID1)
	if err != nil {
		return nil, fmt.Errorf("获取LLDP数据失败(run1): %w", err)
	}
	lldp2, err := e.getLLDPEntries(runID2)
	if err != nil {
		return nil, fmt.Errorf("获取LLDP数据失败(run2): %w", err)
	}
	diff.LLDPDiff = e.compareLLDP(lldp1, lldp2)

	// 对比 FDB
	fdb1, err := e.getFDBEntries(runID1)
	if err != nil {
		return nil, fmt.Errorf("获取FDB数据失败(run1): %w", err)
	}
	fdb2, err := e.getFDBEntries(runID2)
	if err != nil {
		return nil, fmt.Errorf("获取FDB数据失败(run2): %w", err)
	}
	diff.FDBDiff = e.compareFDB(fdb1, fdb2)

	// 对比 ARP
	arp1, err := e.getARPEntries(runID1)
	if err != nil {
		return nil, fmt.Errorf("获取ARP数据失败(run1): %w", err)
	}
	arp2, err := e.getARPEntries(runID2)
	if err != nil {
		return nil, fmt.Errorf("获取ARP数据失败(run2): %w", err)
	}
	diff.ARPDiff = e.compareARP(arp1, arp2)

	// 对比接口
	if1, err := e.getInterfaceEntries(runID1)
	if err != nil {
		return nil, fmt.Errorf("获取接口数据失败(run1): %w", err)
	}
	if2, err := e.getInterfaceEntries(runID2)
	if err != nil {
		return nil, fmt.Errorf("获取接口数据失败(run2): %w", err)
	}
	diff.InterfaceDiff = e.compareInterface(if1, if2)

	return diff, nil
}

// CompareTopologyEdges 对比两次运行的拓扑边
func (e *ReplayExecutor) CompareTopologyEdges(runID1, runID2 string) (*TopologyEdgeDiff, error) {
	diff := &TopologyEdgeDiff{
		RunID1: runID1,
		RunID2: runID2,
	}

	// 获取边数据
	edges1, err := e.getTopologyEdges(runID1)
	if err != nil {
		return nil, fmt.Errorf("获取拓扑边失败(run1): %w", err)
	}
	edges2, err := e.getTopologyEdges(runID2)
	if err != nil {
		return nil, fmt.Errorf("获取拓扑边失败(run2): %w", err)
	}

	// 构建边索引
	edgeMap1 := make(map[string]*TaskTopologyEdge)
	for _, edge := range edges1 {
		key := e.edgeKey(edge.ADeviceID, edge.AIf, edge.BDeviceID, edge.BIf)
		edgeMap1[key] = &edge
	}

	edgeMap2 := make(map[string]*TaskTopologyEdge)
	for _, edge := range edges2 {
		key := e.edgeKey(edge.ADeviceID, edge.AIf, edge.BDeviceID, edge.BIf)
		edgeMap2[key] = &edge
	}

	// 查找差异
	for key, edge1 := range edgeMap1 {
		if edge2, exists := edgeMap2[key]; exists {
			// 两边都存在，检查是否修改
			if edge1.Status != edge2.Status || edge1.Confidence != edge2.Confidence {
				diff.ModifiedEdges = append(diff.ModifiedEdges, EdgeDiffItem{
					ADeviceID:  edge1.ADeviceID,
					AIf:        edge1.AIf,
					BDeviceID:  edge1.BDeviceID,
					BIf:        edge1.BIf,
					Status:     edge2.Status,
					Confidence: edge2.Confidence,
					EdgeType:   edge2.EdgeType,
				})
			} else {
				diff.UnchangedCount++
			}
		} else {
			// 只在run1中存在，已删除
			diff.RemovedEdges = append(diff.RemovedEdges, EdgeDiffItem{
				ADeviceID:  edge1.ADeviceID,
				AIf:        edge1.AIf,
				BDeviceID:  edge1.BDeviceID,
				BIf:        edge1.BIf,
				Status:     edge1.Status,
				Confidence: edge1.Confidence,
				EdgeType:   edge1.EdgeType,
			})
		}
	}

	for key, edge2 := range edgeMap2 {
		if _, exists := edgeMap1[key]; !exists {
			// 只在run2中存在，新增
			diff.AddedEdges = append(diff.AddedEdges, EdgeDiffItem{
				ADeviceID:  edge2.ADeviceID,
				AIf:        edge2.AIf,
				BDeviceID:  edge2.BDeviceID,
				BIf:        edge2.BIf,
				Status:     edge2.Status,
				Confidence: edge2.Confidence,
				EdgeType:   edge2.EdgeType,
			})
		}
	}

	return diff, nil
}

// =============================================================================
// 对比辅助方法
// =============================================================================

func (e *ReplayExecutor) getLLDPEntries(runID string) ([]TaskParsedLLDPNeighbor, error) {
	var entries []TaskParsedLLDPNeighbor
	if err := e.db.Where("task_run_id = ?", runID).Find(&entries).Error; err != nil {
		return nil, err
	}
	return entries, nil
}

func (e *ReplayExecutor) getFDBEntries(runID string) ([]TaskParsedFDBEntry, error) {
	var entries []TaskParsedFDBEntry
	if err := e.db.Where("task_run_id = ?", runID).Find(&entries).Error; err != nil {
		return nil, err
	}
	return entries, nil
}

func (e *ReplayExecutor) getARPEntries(runID string) ([]TaskParsedARPEntry, error) {
	var entries []TaskParsedARPEntry
	if err := e.db.Where("task_run_id = ?", runID).Find(&entries).Error; err != nil {
		return nil, err
	}
	return entries, nil
}

func (e *ReplayExecutor) getInterfaceEntries(runID string) ([]TaskParsedInterface, error) {
	var entries []TaskParsedInterface
	if err := e.db.Where("task_run_id = ?", runID).Find(&entries).Error; err != nil {
		return nil, err
	}
	return entries, nil
}

func (e *ReplayExecutor) getTopologyEdges(runID string) ([]TaskTopologyEdge, error) {
	var edges []TaskTopologyEdge
	if err := e.db.Where("task_run_id = ?", runID).Find(&edges).Error; err != nil {
		return nil, err
	}
	return edges, nil
}

func (e *ReplayExecutor) compareLLDP(entries1, entries2 []TaskParsedLLDPNeighbor) LLDPDiffSummary {
	diff := LLDPDiffSummary{}

	// 构建索引：使用 DeviceIP + LocalInterface + NeighborChassis 作为唯一标识
	map1 := make(map[string]*TaskParsedLLDPNeighbor)
	for i, e := range entries1 {
		key := fmt.Sprintf("%s|%s|%s", e.DeviceIP, e.LocalInterface, e.NeighborChassis)
		map1[key] = &entries1[i]
	}

	map2 := make(map[string]*TaskParsedLLDPNeighbor)
	for i, e := range entries2 {
		key := fmt.Sprintf("%s|%s|%s", e.DeviceIP, e.LocalInterface, e.NeighborChassis)
		map2[key] = &entries2[i]
	}

	for key, e1 := range map1 {
		if e2, exists := map2[key]; exists {
			// 检查是否修改（比较NeighborPort等字段）
			if e1.NeighborPort != e2.NeighborPort || e1.NeighborName != e2.NeighborName || e1.NeighborIP != e2.NeighborIP {
				diff.Modified++
			} else {
				diff.Unchanged++
			}
		} else {
			diff.OnlyIn1++
		}
	}

	for key := range map2 {
		if _, exists := map1[key]; !exists {
			diff.OnlyIn2++
		}
	}

	return diff
}

func (e *ReplayExecutor) compareFDB(entries1, entries2 []TaskParsedFDBEntry) FDBDiffSummary {
	diff := FDBDiffSummary{}

	// 使用 DeviceIP + MACAddress + VLAN 作为唯一标识
	map1 := make(map[string]*TaskParsedFDBEntry)
	for i, e := range entries1 {
		key := fmt.Sprintf("%s|%s|%d", e.DeviceIP, e.MACAddress, e.VLAN)
		map1[key] = &entries1[i]
	}

	map2 := make(map[string]*TaskParsedFDBEntry)
	for i, e := range entries2 {
		key := fmt.Sprintf("%s|%s|%d", e.DeviceIP, e.MACAddress, e.VLAN)
		map2[key] = &entries2[i]
	}

	for key, e1 := range map1 {
		if e2, exists := map2[key]; exists {
			// 检查是否修改（比较Interface, Type）
			if e1.Interface != e2.Interface || e1.Type != e2.Type {
				diff.Modified++
			} else {
				diff.Unchanged++
			}
		} else {
			diff.OnlyIn1++
		}
	}

	for key := range map2 {
		if _, exists := map1[key]; !exists {
			diff.OnlyIn2++
		}
	}

	return diff
}

func (e *ReplayExecutor) compareARP(entries1, entries2 []TaskParsedARPEntry) ARPDiffSummary {
	diff := ARPDiffSummary{}

	// 使用 DeviceIP + IPAddress + MACAddress 作为唯一标识
	map1 := make(map[string]*TaskParsedARPEntry)
	for i, e := range entries1 {
		key := fmt.Sprintf("%s|%s|%s", e.DeviceIP, e.IPAddress, e.MACAddress)
		map1[key] = &entries1[i]
	}

	map2 := make(map[string]*TaskParsedARPEntry)
	for i, e := range entries2 {
		key := fmt.Sprintf("%s|%s|%s", e.DeviceIP, e.IPAddress, e.MACAddress)
		map2[key] = &entries2[i]
	}

	for key, e1 := range map1 {
		if e2, exists := map2[key]; exists {
			// 检查是否修改（比较Interface, Type）
			if e1.Interface != e2.Interface || e1.Type != e2.Type {
				diff.Modified++
			} else {
				diff.Unchanged++
			}
		} else {
			diff.OnlyIn1++
		}
	}

	for key := range map2 {
		if _, exists := map1[key]; !exists {
			diff.OnlyIn2++
		}
	}

	return diff
}

func (e *ReplayExecutor) compareInterface(entries1, entries2 []TaskParsedInterface) InterfaceDiffSummary {
	diff := InterfaceDiffSummary{}

	// 使用 DeviceIP + InterfaceName 作为唯一标识
	map1 := make(map[string]*TaskParsedInterface)
	for i, e := range entries1 {
		key := fmt.Sprintf("%s|%s", e.DeviceIP, e.InterfaceName)
		map1[key] = &entries1[i]
	}

	map2 := make(map[string]*TaskParsedInterface)
	for i, e := range entries2 {
		key := fmt.Sprintf("%s|%s", e.DeviceIP, e.InterfaceName)
		map2[key] = &entries2[i]
	}

	for key, e1 := range map1 {
		if e2, exists := map2[key]; exists {
			// 检查是否修改（比较Status等）
			if e1.Status != e2.Status || e1.IsAggregate != e2.IsAggregate {
				diff.Modified++
			} else {
				diff.Unchanged++
			}
		} else {
			diff.OnlyIn1++
		}
	}

	for key := range map2 {
		if _, exists := map1[key]; !exists {
			diff.OnlyIn2++
		}
	}

	return diff
}

func (e *ReplayExecutor) edgeKey(aDevice, aIf, bDevice, bIf string) string {
	// 保证边的方向一致性（较小的设备ID在前）
	if aDevice < bDevice {
		return fmt.Sprintf("%s|%s|%s|%s", aDevice, aIf, bDevice, bIf)
	}
	return fmt.Sprintf("%s|%s|%s|%s", bDevice, bIf, aDevice, aIf)
}

// =============================================================================
// Raw文件预览功能
// =============================================================================

// RawFilePreview Raw文件预览结果
type RawFilePreview struct {
	DeviceIP   string `json:"deviceIp"`
	CommandKey string `json:"commandKey"`
	FilePath   string `json:"filePath"`
	Content    string `json:"content"`
	Size       int64  `json:"size"`
	Exists     bool   `json:"exists"`
}

// GetRawFilePreview 获取Raw文件预览
func (e *ReplayExecutor) GetRawFilePreview(runID, deviceIP, commandKey string) (*RawFilePreview, error) {
	preview := &RawFilePreview{
		DeviceIP:   deviceIP,
		CommandKey: commandKey,
		Exists:     false,
	}

	// 构建文件路径
	rawDir := filepath.Join(e.pathManager.TopologyRawDir, runID, deviceIP)
	filePath := filepath.Join(rawDir, commandKey+"_raw.txt")

	// 检查文件是否存在
	info, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return preview, nil
		}
		return nil, fmt.Errorf("获取文件信息失败: %w", err)
	}

	preview.FilePath = filePath
	preview.Size = info.Size()
	preview.Exists = true

	// 读取文件内容（限制大小，避免内存溢出）
	const maxSize = 10 * 1024 * 1024 // 10MB
	if info.Size() > maxSize {
		// 只读取前10MB
		content, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("读取文件失败: %w", err)
		}
		preview.Content = string(content[:maxSize]) + "\n... (文件过大，已截断)"
	} else {
		content, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("读取文件失败: %w", err)
		}
		preview.Content = string(content)
	}

	return preview, nil
}

// ListRawFiles 列出指定设备和运行ID的所有Raw文件
func (e *ReplayExecutor) ListRawFiles(runID, deviceIP string) ([]RawFileInfo, error) {
	rawDir := filepath.Join(e.pathManager.TopologyRawDir, runID, deviceIP)

	if _, err := os.Stat(rawDir); os.IsNotExist(err) {
		return []RawFileInfo{}, nil
	}

	var files []RawFileInfo
	err := filepath.Walk(rawDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		if strings.HasSuffix(info.Name(), "_raw.txt") {
			commandKey := strings.TrimSuffix(info.Name(), "_raw.txt")
			files = append(files, RawFileInfo{
				DeviceIP:   deviceIP,
				CommandKey: commandKey,
				FilePath:   path,
				FileSize:   info.Size(),
			})
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("扫描Raw文件失败: %w", err)
	}

	return files, nil
}
