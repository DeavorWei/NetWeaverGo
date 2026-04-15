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
			node.NodeType = models.NodeTypeManaged
		} else if strings.HasPrefix(id, "server:") {
			ip := strings.TrimPrefix(id, "server:")
			node.Label = ip
			node.IP = ip
			node.Role = "server-inferred"
			node.Vendor = "endpoint"
			node.NodeType = models.NodeTypeInferred
			// 从边信息中提取MAC地址（优先从BDeviceMAC字段）
			mac, macs := extractMACFromEdges(edges, id)
			node.MACAddress = mac
			node.MACAddresses = macs
		} else if strings.HasPrefix(id, "terminal:") {
			ipOrMAC := strings.TrimPrefix(id, "terminal:")
			// 判断是IP还是MAC（检查是否为有效IP）
			if isValidIP(ipOrMAC) {
				node.Label = ipOrMAC
				node.IP = ipOrMAC
			} else {
				node.Label = ipOrMAC
				node.MACAddress = ipOrMAC
			}
			node.Role = "terminal-inferred"
			node.Vendor = "endpoint"
			node.NodeType = models.NodeTypeInferred
			// 从边信息中提取MAC地址
			mac, macs := extractMACFromEdges(edges, id)
			if node.MACAddress == "" {
				node.MACAddress = mac
			}
			node.MACAddresses = macs
		} else if strings.HasPrefix(id, "unknown:") {
			// 处理未知MAC节点
			mac := strings.TrimPrefix(id, "unknown:")
			node.Label = mac
			node.MACAddress = mac
			node.Role = "unknown-inferred"
			node.Vendor = "unknown"
			node.NodeType = models.NodeTypeUnknown
		} else if strings.HasPrefix(id, "unmanaged:") {
			unmanagedID := strings.TrimPrefix(id, "unmanaged:")
			node.Label = unmanagedID
			node.IP = unmanagedID
			node.Role = "unmanaged"
			node.Vendor = "unknown"
			node.NodeType = models.NodeTypeUnmanaged
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
		ID:                  edge.ID,
		ADevice:             s.getGraphNode(runID, edge.ADeviceID),
		AIf:                 edge.AIf,
		LogicalAIf:          edge.LogicalAIf,
		BDevice:             s.getGraphNode(runID, edge.BDeviceID, edge.BDeviceMAC),
		BIf:                 edge.BIf,
		LogicalBIf:          edge.LogicalBIf,
		EdgeType:            edge.EdgeType,
		Status:              edge.Status,
		Confidence:          edge.Confidence,
		DiscoveryMethods:    append([]string(nil), edge.DiscoveryMethods...),
		Evidence:            convertToModelEvidence(edge.Evidence),
		ConfidenceBreakdown: edge.ConfidenceBreakdown,
		DecisionReason:      edge.DecisionReason,
		CandidateID:         edge.CandidateID,
		TraceID:             edge.TraceID,
	}, nil
}

// GetTopologyEdgeExplain 获取边的完整解释视图（包含候选和决策轨迹）
func (s *TaskExecutionService) GetTopologyEdgeExplain(runID, edgeID string) (*models.TopologyEdgeExplainView, error) {
	// 获取边详情
	edgeDetail, err := s.GetTopologyEdgeDetail(runID, edgeID)
	if err != nil {
		return nil, err
	}

	view := &models.TopologyEdgeExplainView{
		Edge: *edgeDetail,
	}

	// 获取关联的候选列表
	candidates := make([]models.TopologyCandidateView, 0)
	if edgeDetail.CandidateID != "" {
		// 查询同一端点组的所有候选
		var allCandidates []TopologyEdgeCandidate
		if err := s.db.Where("task_run_id = ?", runID).Find(&allCandidates).Error; err == nil {
			for _, c := range allCandidates {
				// 筛选与当前边相关的候选（同一端点组）
				if s.isRelatedCandidate(c, edgeDetail) {
					candidates = append(candidates, models.TopologyCandidateView{
						CandidateID:    c.CandidateID,
						ADeviceID:      c.ADeviceID,
						AIf:            c.AIf,
						LogicalAIf:     c.LogicalAIf,
						BDeviceID:      c.BDeviceID,
						BIf:            c.BIf,
						LogicalBIf:     c.LogicalBIf,
						Source:         c.Source,
						Status:         c.Status,
						TotalScore:     c.TotalScore,
						ScoreBreakdown: c.ScoreBreakdown,
						Features:       c.Features,
						DecisionReason: c.DecisionReason,
					})
				}
			}
		}
	}
	view.Candidates = candidates

	// 获取决策轨迹
	if edgeDetail.TraceID != "" {
		var trace TopologyDecisionTrace
		if err := s.db.Where("task_run_id = ? AND trace_id = ?", runID, edgeDetail.TraceID).First(&trace).Error; err == nil {
			view.DecisionTrace = &models.TopologyDecisionTraceView{
				TraceID:              trace.TraceID,
				DecisionType:         trace.DecisionType,
				DecisionGroup:        trace.DecisionGroup,
				DecisionResult:       trace.DecisionResult,
				DecisionReason:       trace.DecisionReason,
				DecisionBasis:        trace.DecisionBasis,
				RetainedCandidateIDs: trace.RetainedCandidateIDs,
				RejectedCandidateIDs: trace.RejectedCandidateIDs,
				Candidates:           trace.Candidates,
			}
		}
	}

	return view, nil
}

// isRelatedCandidate 判断候选是否与指定边相关
func (s *TaskExecutionService) isRelatedCandidate(c TopologyEdgeCandidate, edge *models.TopologyEdgeDetailView) bool {
	// 检查候选是否与边共享同一端点
	aMatch := c.ADeviceID == edge.ADevice.ID &&
		(c.AIf == edge.AIf || c.LogicalAIf == edge.LogicalAIf)
	bMatch := c.BDeviceID == edge.BDevice.ID &&
		(c.BIf == edge.BIf || c.LogicalBIf == edge.LogicalBIf)
	return aMatch || bMatch
}

// GetTopologyCandidatesByRun 获取运行的所有候选边
func (s *TaskExecutionService) GetTopologyCandidatesByRun(runID string) ([]models.TopologyCandidateView, error) {
	var candidates []TopologyEdgeCandidate
	if err := s.db.Where("task_run_id = ?", runID).Find(&candidates).Error; err != nil {
		return nil, err
	}

	result := make([]models.TopologyCandidateView, 0, len(candidates))
	for _, c := range candidates {
		result = append(result, models.TopologyCandidateView{
			CandidateID:    c.CandidateID,
			ADeviceID:      c.ADeviceID,
			AIf:            c.AIf,
			LogicalAIf:     c.LogicalAIf,
			BDeviceID:      c.BDeviceID,
			BIf:            c.BIf,
			LogicalBIf:     c.LogicalBIf,
			Source:         c.Source,
			Status:         c.Status,
			TotalScore:     c.TotalScore,
			ScoreBreakdown: c.ScoreBreakdown,
			Features:       c.Features,
			DecisionReason: c.DecisionReason,
		})
	}
	return result, nil
}

// GetTopologyDecisionTracesByRun 获取运行的所有决策轨迹
func (s *TaskExecutionService) GetTopologyDecisionTracesByRun(runID string) ([]models.TopologyDecisionTraceView, error) {
	var traces []TopologyDecisionTrace
	if err := s.db.Where("task_run_id = ?", runID).Find(&traces).Error; err != nil {
		return nil, err
	}

	result := make([]models.TopologyDecisionTraceView, 0, len(traces))
	for _, t := range traces {
		result = append(result, models.TopologyDecisionTraceView{
			TraceID:              t.TraceID,
			DecisionType:         t.DecisionType,
			DecisionGroup:        t.DecisionGroup,
			DecisionResult:       t.DecisionResult,
			DecisionReason:       t.DecisionReason,
			DecisionBasis:        t.DecisionBasis,
			RetainedCandidateIDs: t.RetainedCandidateIDs,
			RejectedCandidateIDs: t.RejectedCandidateIDs,
			Candidates:           t.Candidates,
		})
	}
	return result, nil
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

func (s *TaskExecutionService) getGraphNode(runID, deviceID string, macAddress ...string) models.GraphNode {
	if strings.TrimSpace(deviceID) == "" {
		return models.GraphNode{ID: "unknown", Label: "unknown"}
	}
	if strings.HasPrefix(deviceID, "server:") {
		ip := strings.TrimPrefix(deviceID, "server:")
		node := models.GraphNode{
			ID:        deviceID,
			Label:     ip,
			IP:        ip,
			Role:      "server-inferred",
			Vendor:    "endpoint",
			NodeType:  models.NodeTypeInferred,
		}
		if len(macAddress) > 0 && macAddress[0] != "" {
			node.MACAddress = macAddress[0]
		}
		return node
	}
	if strings.HasPrefix(deviceID, "terminal:") {
		ip := strings.TrimPrefix(deviceID, "terminal:")
		node := models.GraphNode{
			ID:        deviceID,
			Label:     ip,
			IP:        ip,
			Role:      "terminal-inferred",
			Vendor:    "endpoint",
			NodeType:  models.NodeTypeInferred,
		}
		if len(macAddress) > 0 && macAddress[0] != "" {
			node.MACAddress = macAddress[0]
		}
		return node
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
		NodeType:     models.NodeTypeManaged,
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

// extractMACFromEdges 从边信息中提取推断节点的MAC地址
// 用于server:和terminal:类型的推断节点
// 返回: (主MAC, 所有MAC列表)
func extractMACFromEdges(edges []TaskTopologyEdge, deviceID string) (string, []string) {
	for _, e := range edges {
		if e.BDeviceID == deviceID {
			// 优先从BDeviceMAC字段读取
			if e.BDeviceMAC != "" {
				var macs []string
				if e.BDeviceMACs != "" {
					json.Unmarshal([]byte(e.BDeviceMACs), &macs)
				}
				if len(macs) == 0 && e.BDeviceMAC != "" {
					macs = []string{e.BDeviceMAC}
				}
				return e.BDeviceMAC, macs
			}
			// 降级：从Evidence中提取
			for _, ev := range e.Evidence {
				if ev.RemoteMAC != "" {
					return ev.RemoteMAC, nil
				}
			}
		}
	}
	return "", nil
}
