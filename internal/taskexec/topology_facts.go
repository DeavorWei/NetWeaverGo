package taskexec

import (
	"fmt"
	"time"

	"github.com/NetWeaverGo/core/internal/logger"
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
	return p.db.Model(&TaskRunDevice{}).
		Where("task_run_id = ? AND device_ip = ?", runID, deviceIP).
		Updates(map[string]interface{}{
			"vendor":          identity.Vendor,
			"model":           identity.Model,
			"hostname":        identity.Hostname,
			"normalized_name": identity.Hostname,
			"mgmt_ip":         identity.MgmtIP,
			"chassis_id":      identity.ChassisID,
			"status":          "completed",
		}).Error
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

		// 保存聚合
		for _, g := range aggs {
			if err := tx.Create(&TaskParsedAggregateGroup{
				TaskRunID:     runID,
				DeviceIP:      deviceIP,
				AggregateName: g.AggregateName,
				Mode:          g.Mode,
				CommandKey:    g.CommandKey,
				RawRefID:      g.RawRefID,
				CreatedAt:     time.Now(),
				UpdatedAt:     time.Now(),
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
					CreatedAt:     time.Now(),
					UpdatedAt:     time.Now(),
				}).Error; err != nil {
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
func MapCommandOutput(mapper parser.ResultMapper, commandKey string, rows []map[string]string,
	identity *parser.DeviceIdentity, rawRefID string) (*ParsedFactBatch, error) {

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
	identity.Vendor = normalize.NormalizeVendor(identity.Vendor)
	identity.Hostname = normalize.NormalizeDeviceName(identity.Hostname)
}

