package taskexec

import (
	"fmt"
	"strings"
	"time"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/device"
	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
	"github.com/NetWeaverGo/core/internal/normalize"
	"github.com/NetWeaverGo/core/internal/parser"
	"gorm.io/gorm"
)

// =============================================================================
// 拓扑事实持久化器
// 在线执行器和重放执行器共享的解析结果持久化逻辑
// =============================================================================

// TopologyFactsPersister 拓扑事实持久化器
type TopologyFactsPersister struct {
	db *gorm.DB
}

// NewTopologyFactsPersister 创建拓扑事实持久化器
func NewTopologyFactsPersister(db *gorm.DB) *TopologyFactsPersister {
	return &TopologyFactsPersister{db: db}
}

// SaveDeviceIdentity 保存设备身份信息
func (p *TopologyFactsPersister) SaveDeviceIdentity(runID, deviceIP string, identity *parser.DeviceIdentity) error {
	updates := map[string]interface{}{
		"vendor":          identity.Vendor,
		"model":           identity.Model,
		"version":         identity.Version,
		"hostname":        identity.Hostname,
		"normalized_name": identity.Hostname,
		"mgmt_ip":         identity.MgmtIP,
		"chassis_id":      identity.ChassisID,
		"status":          "completed",
	}

	if identity.DeviceRef != nil {
		if identity.DeviceRef.Series != "" {
			updates["model_series"] = identity.DeviceRef.Series
		}
		if identity.DeviceRef.Patch != "" {
			updates["patch_version"] = identity.DeviceRef.Patch
		}
		if len(identity.DeviceRef.Evidence) > 0 {
			updates["identity_evidence"] = strings.Join(identity.DeviceRef.Evidence, "; ")
		}
	}
	if identity.ProfileMatchPath != "" {
		updates["profile_match_path"] = identity.ProfileMatchPath
	}
	if identity.ModelSeries != "" && updates["model_series"] == nil {
		updates["model_series"] = identity.ModelSeries
	}
	if identity.PatchVersion != "" && updates["patch_version"] == nil {
		updates["patch_version"] = identity.PatchVersion
	}
	if identity.IdentityEvidence != "" && updates["identity_evidence"] == nil {
		updates["identity_evidence"] = identity.IdentityEvidence
	}

	// 同步回写资产表的款型、版本、系列与补丁字段（彻底解决 P2-4 半成品列问题）
	assetUpdates := map[string]interface{}{}
	if identity.Model != "" {
		assetUpdates["model"] = identity.Model
	}
	if identity.Version != "" {
		assetUpdates["version"] = identity.Version
	}
	if s := identity.ModelSeries; s != "" {
		assetUpdates["model_series"] = s
	} else if identity.DeviceRef != nil && identity.DeviceRef.Series != "" {
		assetUpdates["model_series"] = identity.DeviceRef.Series
	}
	if p := identity.PatchVersion; p != "" {
		assetUpdates["patch_version"] = p
	} else if identity.DeviceRef != nil && identity.DeviceRef.Patch != "" {
		assetUpdates["patch_version"] = identity.DeviceRef.Patch
	}
	if len(assetUpdates) > 0 {
		_ = p.db.Model(&models.DeviceAsset{}).Where("ip = ?", deviceIP).Updates(assetUpdates).Error
	}

	return p.db.Model(&TaskRunDevice{}).
		Where("task_run_id = ? AND device_ip = ?", runID, deviceIP).
		Updates(updates).Error
}

// SaveParsedFacts 保存解析后的事实数据（接口/LLDP/FDB/ARP/聚合）
func (p *TopologyFactsPersister) SaveParsedFacts(runID, deviceIP string,
	interfaces []parser.InterfaceFact, lldps []parser.LLDPFact,
	fdbs []parser.FDBFact, arps []parser.ARPFact, aggs []parser.AggregateFact) error {

	return p.db.Transaction(func(tx *gorm.DB) error {
		// 清理旧数据
		if err := clearDeviceFactsInTx(tx, runID, deviceIP); err != nil {
			return err
		}

		now := time.Now()
		const batchSize = 100

		// 批量保存接口
		if len(interfaces) > 0 {
			records := make([]TaskParsedInterface, 0, len(interfaces))
			for _, iface := range interfaces {
				records = append(records, TaskParsedInterface{
					TaskRunID:     runID,
					DeviceIP:      deviceIP,
					InterfaceName: iface.Name,
					Status:        iface.Status,
					IsAggregate:   iface.IsAggregate,
					AggregateID:   iface.AggregateID,
					CreatedAt:     now,
					UpdatedAt:     now,
				})
			}
			if err := tx.CreateInBatches(&records, batchSize).Error; err != nil {
				return err
			}
		}

		// 批量保存LLDP
		if len(lldps) > 0 {
			records := make([]TaskParsedLLDPNeighbor, 0, len(lldps))
			for _, n := range lldps {
				records = append(records, TaskParsedLLDPNeighbor{
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
					CreatedAt:       now,
					UpdatedAt:       now,
				})
			}
			if err := tx.CreateInBatches(&records, batchSize).Error; err != nil {
				return err
			}
		}

		// 批量保存FDB
		if len(fdbs) > 0 {
			records := make([]TaskParsedFDBEntry, 0, len(fdbs))
			for _, f := range fdbs {
				records = append(records, TaskParsedFDBEntry{
					TaskRunID:  runID,
					DeviceIP:   deviceIP,
					MACAddress: f.MACAddress,
					VLAN:       f.VLAN,
					Interface:  f.Interface,
					Type:       f.Type,
					CommandKey: f.CommandKey,
					RawRefID:   f.RawRefID,
					CreatedAt:  now,
					UpdatedAt:  now,
				})
			}
			if err := tx.CreateInBatches(&records, batchSize).Error; err != nil {
				return err
			}
		}

		// 批量保存ARP
		if len(arps) > 0 {
			records := make([]TaskParsedARPEntry, 0, len(arps))
			for _, a := range arps {
				records = append(records, TaskParsedARPEntry{
					TaskRunID:  runID,
					DeviceIP:   deviceIP,
					IPAddress:  a.IPAddress,
					MACAddress: a.MACAddress,
					Interface:  a.Interface,
					Type:       a.Type,
					CommandKey: a.CommandKey,
					RawRefID:   a.RawRefID,
					CreatedAt:  now,
					UpdatedAt:  now,
				})
			}
			if err := tx.CreateInBatches(&records, batchSize).Error; err != nil {
				return err
			}
		}

		// 批量保存聚合
		for _, g := range aggs {
			if err := tx.Create(&TaskParsedAggregateGroup{
				TaskRunID:     runID,
				DeviceIP:      deviceIP,
				AggregateName: g.AggregateName,
				Mode:          g.Mode,
				CommandKey:    g.CommandKey,
				RawRefID:      g.RawRefID,
				CreatedAt:     now,
				UpdatedAt:     now,
			}).Error; err != nil {
				return err
			}
			if len(g.MemberPorts) > 0 {
				members := make([]TaskParsedAggregateMember, 0, len(g.MemberPorts))
				for _, member := range g.MemberPorts {
					members = append(members, TaskParsedAggregateMember{
						TaskRunID:     runID,
						DeviceIP:      deviceIP,
						AggregateName: g.AggregateName,
						MemberPort:    member,
						CommandKey:    g.CommandKey,
						RawRefID:      g.RawRefID,
						CreatedAt:     now,
						UpdatedAt:     now,
					})
				}
				if err := tx.CreateInBatches(&members, batchSize).Error; err != nil {
					return err
				}
			}
		}

		return nil
	})
}

// ClearDeviceFacts 清除指定设备的事实数据
func (p *TopologyFactsPersister) ClearDeviceFacts(runID, deviceIP string) error {
	return p.db.Transaction(func(tx *gorm.DB) error {
		return clearDeviceFactsInTx(tx, runID, deviceIP)
	})
}

// ClearAllFacts 清除指定运行的所有事实和拓扑数据
func (p *TopologyFactsPersister) ClearAllFacts(runID string) error {
	logger.Info("TopologyFacts", runID, "清除所有事实数据")

	return p.db.Transaction(func(tx *gorm.DB) error {
		models := []interface{}{
			&TaskParsedLLDPNeighbor{},
			&TaskParsedFDBEntry{},
			&TaskParsedARPEntry{},
			&TaskParsedInterface{},
			&TaskParsedAggregateMember{},
			&TaskParsedAggregateGroup{},
			&TaskTopologyEdge{},
			&TopologyEdgeCandidate{},
			&TopologyDecisionTrace{},
			&TaskRawOutput{},
		}
		for _, m := range models {
			if err := tx.Where("task_run_id = ?", runID).Delete(m).Error; err != nil {
				return fmt.Errorf("清除 %T 失败: %w", m, err)
			}
		}
		return nil
	})
}

// clearDeviceFactsInTx 在事务中清除指定设备的事实数据
func clearDeviceFactsInTx(tx *gorm.DB, runID, deviceIP string) error {
	models := []interface{}{
		&TaskParsedInterface{},
		&TaskParsedLLDPNeighbor{},
		&TaskParsedFDBEntry{},
		&TaskParsedARPEntry{},
		&TaskParsedAggregateMember{},
		&TaskParsedAggregateGroup{},
	}
	for _, m := range models {
		if err := tx.Where("task_run_id = ? AND device_ip = ?", runID, deviceIP).Delete(m).Error; err != nil {
			return err
		}
	}
	return nil
}

// =============================================================================
// 命令输出映射
// 在线执行器和重放执行器共享的命令解析结果映射逻辑
// =============================================================================

// ParsedFactBatch 解析后的事实批次
type ParsedFactBatch struct {
	Identity   *parser.DeviceIdentity
	Interfaces []parser.InterfaceFact
	LLDPs      []parser.LLDPFact
	FDBs       []parser.FDBFact
	ARPs       []parser.ARPFact
	Aggregates []parser.AggregateFact
}

// MapCommandOutput 将解析器输出映射为事实数据
// commandKey: version/sysname/interface_brief/lldp_neighbor/arp_all/eth_trunk 等
// rawRefID: 原始数据引用ID（用于追溯）
// rawOutputs: 可选的命令原始文本（主要用于 version 分支调用 device.Identify 提取形态认知模型）
func MapCommandOutput(mapper parser.ResultMapper, commandKey string, rows []map[string]string,
	identity *parser.DeviceIdentity, rawRefID string, rawOutputs ...string) (*ParsedFactBatch, error) {

	batch := &ParsedFactBatch{
		Identity: identity,
	}

	switch commandKey {
	case "version":
		id, err := mapper.ToDeviceInfo(rows)
		if err != nil {
			return nil, fmt.Errorf("映射设备信息失败: %w", err)
		}
		mergeIdentityResult(identity, id, identity.Vendor)

		// 桥接设备形态认知层 (device.Identify)
		var rawVer string
		if len(rawOutputs) > 0 {
			rawVer = rawOutputs[0]
		}
		if rawVer != "" {
			raws := map[string]string{"version": rawVer}
			if identity.DeviceRef != nil && identity.DeviceRef.Raws != nil {
				for k, v := range identity.DeviceRef.Raws {
					raws[k] = v
				}
			}
			raws["version"] = rawVer
			if devId, err := device.Identify(identity.Vendor, raws); err == nil {
				identity.DeviceRef = devId
				if devId.Model != "" {
					identity.Model = devId.Model
				}
				if devId.Version != "" {
					identity.Version = devId.Version
				}
				if devId.Series != "" {
					identity.ModelSeries = devId.Series
				}
				if devId.Patch != "" {
					identity.PatchVersion = devId.Patch
				}
				if len(devId.Evidence) > 0 {
					identity.IdentityEvidence = strings.Join(devId.Evidence, "; ")
				}
				if identity.Hostname == "" && devId.SysName != "" {
					identity.Hostname = devId.SysName
				}
				if (identity.Vendor == "" || identity.Vendor == "unknown") && devId.Vendor != "" {
					identity.Vendor = devId.Vendor
				}
				modelKey := identity.Model
				if modelKey == "" {
					modelKey = identity.ModelSeries
				}
				prof, matchPath := config.ResolveProfile(identity.Vendor, modelKey, identity.Version)
				if prof != nil {
					identity.ProfileMatchPath = matchPath
				}
			}
		}

	case "patch_info":
		var rawPatch string
		if len(rawOutputs) > 0 {
			rawPatch = rawOutputs[0]
		}
		if rawPatch != "" {
			raws := map[string]string{"patch_info": rawPatch}
			if identity.DeviceRef != nil && identity.DeviceRef.Raws != nil {
				for k, v := range identity.DeviceRef.Raws {
					raws[k] = v
				}
			}
			raws["patch_info"] = rawPatch
			if devId, err := device.Identify(identity.Vendor, raws); err == nil {
				identity.DeviceRef = devId
				if devId.Patch != "" {
					identity.PatchVersion = devId.Patch
				}
				if devId.Series != "" && identity.ModelSeries == "" {
					identity.ModelSeries = devId.Series
				}
				if len(devId.Evidence) > 0 {
					identity.IdentityEvidence = strings.Join(devId.Evidence, "; ")
				}
				modelKey := identity.Model
				if modelKey == "" {
					modelKey = identity.ModelSeries
				}
				prof, matchPath := config.ResolveProfile(identity.Vendor, modelKey, identity.Version)
				if prof != nil {
					identity.ProfileMatchPath = matchPath
				}
			}
		}

	case "sysname":
		mergeIdentityFields(identity, flattenParseRows(rows), identity.Vendor)

	case "interface_brief":
		items, err := mapper.ToInterfaces(rows)
		if err != nil {
			return nil, fmt.Errorf("映射接口失败: %w", err)
		}
		batch.Interfaces = items

	case "lldp_neighbor", "lldp_neighbor_verbose":
		items, err := mapper.ToLLDP(rows)
		if err != nil {
			return nil, fmt.Errorf("映射LLDP失败: %w", err)
		}
		for i := range items {
			items[i].CommandKey = commandKey
			items[i].RawRefID = rawRefID
		}
		batch.LLDPs = items

	case "arp_all", "arp":
		items, err := mapper.ToARP(rows)
		if err != nil {
			return nil, fmt.Errorf("映射ARP失败: %w", err)
		}
		for i := range items {
			items[i].CommandKey = commandKey
			items[i].RawRefID = rawRefID
		}
		batch.ARPs = items

	case "mac_address":
		items, err := mapper.ToFDB(rows)
		if err != nil {
			return nil, fmt.Errorf("映射FDB失败: %w", err)
		}
		for i := range items {
			items[i].CommandKey = commandKey
			items[i].RawRefID = rawRefID
		}
		batch.FDBs = items


	case "eth_trunk", "eth_trunk_verbose":
		items, err := mapper.ToAggregate(rows)
		if err != nil {
			return nil, fmt.Errorf("映射聚合失败: %w", err)
		}
		for i := range items {
			items[i].CommandKey = commandKey
			items[i].RawRefID = rawRefID
		}
		batch.Aggregates = items

	default:
		logger.Verbose("TopologyFacts", "", "未知命令键: %s, 跳过映射", commandKey)
	}

	return batch, nil
}

// NormalizeIdentity 标准化设备身份信息
func NormalizeIdentity(identity *parser.DeviceIdentity) {
	identity.Vendor = strings.ToLower(strings.TrimSpace(identity.Vendor))
	identity.Hostname = normalize.NormalizeDeviceName(identity.Hostname)
}

