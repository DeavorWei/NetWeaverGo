package taskexec

import (
	"encoding/json"
	"os"
	"strings"
	"time"

	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
	"github.com/NetWeaverGo/core/internal/parser"
)

func (s *TaskExecutionService) GetSupportedTopologyVendors() []string {
	resolver := NewTopologyCommandResolver()
	vendors := resolver.SupportedVendors()
	if len(vendors) == 0 {
		return []string{"huawei", "h3c", "cisco"}
	}
	return vendors
}

func (s *TaskExecutionService) GetTopologyGraph(runID string) (*models.TopologyGraphView, error) {
	var edges []TaskTopologyEdge
	if err := s.db.Where("task_run_id = ?", runID).Find(&edges).Error; err != nil {
		return nil, err
	}

	var devices []TaskRunDevice
	if err := s.db.Where("task_run_id = ?", runID).Find(&devices).Error; err != nil {
		return nil, err
	}
	deviceMap := make(map[string]TaskRunDevice, len(devices))
	deviceStatusStats := make(map[string]int)
	for _, d := range devices {
		deviceMap[d.DeviceIP] = d
		deviceStatusStats[strings.TrimSpace(d.Status)]++
	}

	nodeSet := make(map[string]struct{}, len(devices)+len(edges)*2)
	for _, d := range devices {
		if strings.TrimSpace(d.DeviceIP) != "" {
			nodeSet[d.DeviceIP] = struct{}{}
		}
	}
	for _, e := range edges {
		if strings.TrimSpace(e.ADeviceID) != "" {
			nodeSet[e.ADeviceID] = struct{}{}
		}
		if strings.TrimSpace(e.BDeviceID) != "" {
			nodeSet[e.BDeviceID] = struct{}{}
		}
	}

	nodes := make([]models.GraphNode, 0, len(nodeSet))
	for id := range nodeSet {
		node := models.GraphNode{ID: id, Label: id}
		if d, ok := deviceMap[id]; ok {
			node.Label = chooseValue(d.DisplayName, d.Hostname, d.Model, d.DeviceIP)
			node.IP = d.DeviceIP
			node.Vendor = d.Vendor
			node.Model = d.Model
			node.Role = d.Role
			node.Site = d.Site
			node.SerialNumber = d.SerialNumber
		} else if strings.HasPrefix(id, "server:") {
			node.Label = strings.TrimPrefix(id, "server:")
			node.IP = strings.TrimPrefix(id, "server:")
			node.Role = "server-inferred"
			node.Vendor = "endpoint"
		} else if strings.HasPrefix(id, "terminal:") {
			node.Label = strings.TrimPrefix(id, "terminal:")
			node.Role = "terminal-inferred"
			node.Vendor = "endpoint"
		}
		nodes = append(nodes, node)
	}

	graphEdges := make([]models.GraphEdge, 0, len(edges))
	for _, e := range edges {
		graphEdges = append(graphEdges, models.GraphEdge{
			ID:              e.ID,
			Source:          e.ADeviceID,
			Target:          e.BDeviceID,
			SourceIf:        e.AIf,
			TargetIf:        e.BIf,
			LogicalSourceIf: e.LogicalAIf,
			LogicalTargetIf: e.LogicalBIf,
			EdgeType:        e.EdgeType,
			Status:          e.Status,
			Confidence:      e.Confidence,
		})
	}

	logger.Verbose("TaskExec", runID, "查询拓扑图: devices=%d, nodes=%d, edges=%d, deviceStatus=%v", len(devices), len(nodes), len(graphEdges), deviceStatusStats)
	if len(graphEdges) == 0 {
		var lldpCount int64
		var rawOutputCount int64
		var parseFailedCount int64
		var parseSuccessCount int64
		_ = s.db.Model(&TaskParsedLLDPNeighbor{}).Where("task_run_id = ?", runID).Count(&lldpCount).Error
		_ = s.db.Model(&TaskRawOutput{}).Where("task_run_id = ?", runID).Count(&rawOutputCount).Error
		_ = s.db.Model(&TaskRawOutput{}).Where("task_run_id = ? AND parse_status = ?", runID, "parse_failed").Count(&parseFailedCount).Error
		_ = s.db.Model(&TaskRawOutput{}).Where("task_run_id = ? AND parse_status = ?", runID, "success").Count(&parseSuccessCount).Error
		logger.Warn("TaskExec", runID, "查询拓扑图无边结果: devices=%d, nodes=%d, rawOutputs=%d, parseSuccess=%d, parseFailed=%d, lldpFacts=%d, deviceStatus=%v", len(devices), len(nodes), rawOutputCount, parseSuccessCount, parseFailedCount, lldpCount, deviceStatusStats)
	}

	return &models.TopologyGraphView{
		TaskID: runID,
		Nodes:  nodes,
		Edges:  graphEdges,
	}, nil
}

func (s *TaskExecutionService) GetTopologyEdgeDetail(runID, edgeID string) (*models.TopologyEdgeDetailView, error) {
	var edge TaskTopologyEdge
	if err := s.db.Where("task_run_id = ? AND id = ?", runID, edgeID).First(&edge).Error; err != nil {
		return nil, err
	}

	return &models.TopologyEdgeDetailView{
		ID:               edge.ID,
		ADevice:          s.getGraphNode(runID, edge.ADeviceID),
		AIf:              edge.AIf,
		LogicalAIf:       edge.LogicalAIf,
		BDevice:          s.getGraphNode(runID, edge.BDeviceID),
		BIf:              edge.BIf,
		LogicalBIf:       edge.LogicalBIf,
		EdgeType:         edge.EdgeType,
		Status:           edge.Status,
		Confidence:       edge.Confidence,
		DiscoveryMethods: append([]string(nil), edge.DiscoveryMethods...),
		Evidence:         convertToModelEvidence(edge.Evidence),
	}, nil
}

func (s *TaskExecutionService) GetTopologyDeviceDetail(runID, deviceIP string) (*parser.ParsedResult, error) {
	result := &parser.ParsedResult{
		TaskID:   runID,
		DeviceIP: deviceIP,
		ParsedAt: time.Now(),
	}

	var dev TaskRunDevice
	if err := s.db.Where("task_run_id = ? AND device_ip = ?", runID, deviceIP).First(&dev).Error; err == nil {
		result.Identity = &parser.DeviceIdentity{
			Vendor:       dev.Vendor,
			Model:        dev.Model,
			SerialNumber: dev.SerialNumber,
			Version:      dev.Version,
			Hostname:     dev.Hostname,
			MgmtIP:       dev.MgmtIP,
			ChassisID:    dev.ChassisID,
		}
	}

	var ifaces []TaskParsedInterface
	_ = s.db.Where("task_run_id = ? AND device_ip = ?", runID, deviceIP).Find(&ifaces).Error
	for _, iface := range ifaces {
		result.Interfaces = append(result.Interfaces, parser.InterfaceFact{
			Name:        iface.InterfaceName,
			Status:      iface.Status,
			Speed:       iface.Speed,
			Duplex:      iface.Duplex,
			Description: iface.Description,
			MACAddress:  iface.MACAddress,
			IPAddress:   iface.IPAddress,
			IsAggregate: iface.IsAggregate,
			AggregateID: iface.AggregateID,
		})
	}

	var lldps []TaskParsedLLDPNeighbor
	_ = s.db.Where("task_run_id = ? AND device_ip = ?", runID, deviceIP).Find(&lldps).Error
	for _, n := range lldps {
		result.LLDPNeighbors = append(result.LLDPNeighbors, parser.LLDPFact{
			LocalInterface:  n.LocalInterface,
			NeighborName:    n.NeighborName,
			NeighborChassis: n.NeighborChassis,
			NeighborPort:    n.NeighborPort,
			NeighborIP:      n.NeighborIP,
			NeighborDesc:    n.NeighborDesc,
			CommandKey:      n.CommandKey,
			RawRefID:        n.RawRefID,
		})
	}

	var fdbs []TaskParsedFDBEntry
	_ = s.db.Where("task_run_id = ? AND device_ip = ?", runID, deviceIP).Find(&fdbs).Error
	for _, f := range fdbs {
		result.FDBEntries = append(result.FDBEntries, parser.FDBFact{
			MACAddress: f.MACAddress,
			VLAN:       f.VLAN,
			Interface:  f.Interface,
			Type:       f.Type,
			CommandKey: f.CommandKey,
			RawRefID:   f.RawRefID,
		})
	}

	var arps []TaskParsedARPEntry
	_ = s.db.Where("task_run_id = ? AND device_ip = ?", runID, deviceIP).Find(&arps).Error
	for _, a := range arps {
		result.ARPEntries = append(result.ARPEntries, parser.ARPFact{
			IPAddress:  a.IPAddress,
			MACAddress: a.MACAddress,
			Interface:  a.Interface,
			Type:       a.Type,
			CommandKey: a.CommandKey,
			RawRefID:   a.RawRefID,
		})
	}

	var groups []TaskParsedAggregateGroup
	_ = s.db.Where("task_run_id = ? AND device_ip = ?", runID, deviceIP).Find(&groups).Error
	for _, g := range groups {
		var members []TaskParsedAggregateMember
		_ = s.db.Where("task_run_id = ? AND device_ip = ? AND aggregate_name = ?", runID, deviceIP, g.AggregateName).Find(&members).Error
		ports := make([]string, 0, len(members))
		for _, m := range members {
			ports = append(ports, m.MemberPort)
		}
		result.Aggregates = append(result.Aggregates, parser.AggregateFact{
			AggregateName: g.AggregateName,
			Mode:          g.Mode,
			MemberPorts:   ports,
			CommandKey:    g.CommandKey,
			RawRefID:      g.RawRefID,
		})
	}

	return result, nil
}

// ListTopologyCollectionPlans 查询指定运行的拓扑采集计划快照。
func (s *TaskExecutionService) ListTopologyCollectionPlans(runID string) ([]TopologyCollectionPlanArtifact, error) {
	if strings.TrimSpace(runID) == "" {
		return []TopologyCollectionPlanArtifact{}, nil
	}

	artifacts, err := s.GetRunArtifacts(runID)
	if err != nil {
		return nil, err
	}
	plans := make([]TopologyCollectionPlanArtifact, 0)
	for _, artifact := range artifacts {
		if strings.TrimSpace(artifact.ArtifactType) != string(ArtifactTypeTopologyCollectionPlan) {
			continue
		}
		filePath := strings.TrimSpace(artifact.FilePath)
		if filePath == "" {
			continue
		}
		payload, readErr := os.ReadFile(filePath)
		if readErr != nil {
			logger.Warn("TaskExec", runID, "读取拓扑采集计划失败: key=%s, path=%s, err=%v", strings.TrimSpace(artifact.ArtifactKey), filePath, readErr)
			continue
		}
		var plan TopologyCollectionPlanArtifact
		if unmarshalErr := json.Unmarshal(payload, &plan); unmarshalErr != nil {
			logger.Warn("TaskExec", runID, "解析拓扑采集计划失败: key=%s, path=%s, err=%v", strings.TrimSpace(artifact.ArtifactKey), filePath, unmarshalErr)
			continue
		}
		plan.ArtifactKey = strings.TrimSpace(artifact.ArtifactKey)
		plan.FilePath = filePath
		plans = append(plans, plan)
	}
	return plans, nil
}

func (s *TaskExecutionService) getGraphNode(runID, deviceID string) models.GraphNode {
	if strings.TrimSpace(deviceID) == "" {
		return models.GraphNode{ID: "unknown", Label: "unknown"}
	}
	if strings.HasPrefix(deviceID, "server:") {
		return models.GraphNode{
			ID:     deviceID,
			Label:  strings.TrimPrefix(deviceID, "server:"),
			IP:     strings.TrimPrefix(deviceID, "server:"),
			Role:   "server-inferred",
			Vendor: "endpoint",
		}
	}
	if strings.HasPrefix(deviceID, "terminal:") {
		return models.GraphNode{
			ID:     deviceID,
			Label:  strings.TrimPrefix(deviceID, "terminal:"),
			Role:   "terminal-inferred",
			Vendor: "endpoint",
		}
	}
	var dev TaskRunDevice
	if err := s.db.Where("task_run_id = ? AND device_ip = ?", runID, deviceID).First(&dev).Error; err != nil {
		return models.GraphNode{ID: deviceID, Label: deviceID}
	}
	return models.GraphNode{
		ID:           deviceID,
		Label:        chooseValue(dev.DisplayName, dev.Hostname, dev.Model, dev.DeviceIP),
		IP:           dev.DeviceIP,
		Vendor:       dev.Vendor,
		Model:        dev.Model,
		Role:         dev.Role,
		Site:         dev.Site,
		SerialNumber: dev.SerialNumber,
	}
}

func chooseValue(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func convertToModelEvidence(items []EdgeEvidence) []models.EdgeEvidence {
	result := make([]models.EdgeEvidence, 0, len(items))
	for _, e := range items {
		result = append(result, models.EdgeEvidence{
			Type:       e.Type,
			DeviceID:   e.DeviceID,
			Command:    e.Command,
			RawRefID:   e.RawRefID,
			Summary:    e.Summary,
			Source:     e.Source,
			LocalIf:    e.LocalIf,
			RemoteName: e.RemoteName,
			RemoteIf:   e.RemoteIf,
			RemoteMAC:  e.RemoteMAC,
			RemoteIP:   e.RemoteIP,
			Timestamp:  e.Timestamp,
		})
	}
	return result
}

func makeTaskEdgeID() string {
	return newEdgeID()
}
