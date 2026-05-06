package taskexec

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/NetWeaverGo/core/internal/config"
	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/normalize"
	"gorm.io/gorm"
)

// =============================================================================
// Phase A: 拓扑构建执行器重构
// 本文件实现分层架构的拓扑构建器，支持可解释、可追溯的拓扑还原
// =============================================================================

// TopologyBuilder 拓扑构建器
// 实现分层构建流程：事实收集 -> 标准化 -> 候选生成 -> 评分 -> 冲突消解 -> 边落库
type TopologyBuilder struct {
	db     *gorm.DB
	config TopologyBuildConfig
}

// NewTopologyBuilder 创建拓扑构建器
func NewTopologyBuilder(db *gorm.DB, cfg TopologyBuildConfig) *TopologyBuilder {
	return &TopologyBuilder{
		db:     db,
		config: cfg,
	}
}

// Build 执行拓扑构建主流程
func (b *TopologyBuilder) Build(ctx context.Context, runID string) (*TopologyBuildOutput, error) {
	startedAt := time.Now()
	output := &TopologyBuildOutput{
		Errors: []string{},
	}

	// Step 1: 收集构建输入
	input, err := b.collectBuildInputs(runID)
	if err != nil {
		return nil, fmt.Errorf("收集构建输入失败: %w", err)
	}
	input.BuildConfig = b.config

	// Step 2: 创建事实快照
	snapshot := b.createFactSnapshot(input)
	output.FactSnapshot = snapshot

	// Step 3: 标准化事实
	normalized := b.normalizeFacts(input)

	// Step 4: 生成 LLDP 候选
	lldpCandidates := b.buildLLDPCandidates(normalized)

	// Step 5: 生成 FDB/ARP 推断候选
	fdbCandidates := b.buildFDBARPCandidates(normalized)

	// Step 6: 合并候选
	allCandidates := make([]*TopologyEdgeCandidate, 0, len(lldpCandidates)+len(fdbCandidates))
	allCandidates = append(allCandidates, lldpCandidates...)
	allCandidates = append(allCandidates, fdbCandidates...)

	// Step 7: 用接口事实丰富候选
	b.enrichCandidatesWithInterfaceFacts(allCandidates, normalized)

	// Step 8: 全局冲突消解
	resolvedCandidates, decisionTraces := b.resolveCandidatesGlobal(allCandidates)

	// Step 9: 生成最终边
	edges := b.materializeEdges(resolvedCandidates, runID)

	// Step 10: 持久化结果
	if err := b.persistResults(runID, edges, allCandidates, decisionTraces, snapshot); err != nil {
		return nil, fmt.Errorf("持久化结果失败: %w", err)
	}

	// 统计
	output.Edges = edges
	output.Candidates = allCandidates
	output.DecisionTraces = decisionTraces
	output.Statistics = b.computeStatistics(edges, allCandidates, decisionTraces, startedAt)

	// 生成错误信息
	output.Errors = b.generateErrors(normalized, output.Statistics)

	return output, nil
}

// NormalizedFacts 标准化的事实数据
type NormalizedFacts struct {
	Devices           map[string]*DeviceInfo // key: DeviceIP
	LLDPNeighbors     []NormalizedLLDPNeighbor
	FDBEntries        []NormalizedFDBEntry
	ARPEntries        []NormalizedARPEntry
	AggregateGroups   map[string]*AggregateGroupInfo // key: DeviceIP|AggregateName
	AggregateMembers  map[string]string              // key: DeviceIP|MemberPort -> AggregateName
	Interfaces        map[string]*InterfaceInfo      // key: DeviceIP|InterfaceName
	DeviceByName      map[string]string              // key: NormalizedName -> DeviceIP
	DeviceByMgmtIP    map[string]string              // key: MgmtIP -> DeviceIP
	DeviceByChassisID map[string]string              // key: ChassisID -> DeviceIP
	ARPMACToDevice    map[string]string              // key: MAC -> DeviceIP
	ARPMACToIP        map[string]string              // key: MAC -> IP
}

// DeviceInfo 设备信息
type DeviceInfo struct {
	DeviceIP       string
	NormalizedName string
	ChassisID      string
	MgmtIP         string
	Hostname       string
	Vendor         string
	Model          string
}

// NormalizedLLDPNeighbor 标准化的 LLDP 邻居
type NormalizedLLDPNeighbor struct {
	DeviceIP        string
	LocalIf         string
	LocalLogicalIf  string
	NeighborName    string
	NeighborChassis string
	NeighborPort    string
	NeighborIP      string
	NeighborDesc    string
	CommandKey      string
	RawRefID        string
}

// NormalizedFDBEntry 标准化的 FDB 条目
type NormalizedFDBEntry struct {
	DeviceIP   string
	MACAddress string
	VLAN       int
	Interface  string
	Type       string
	CommandKey string
	RawRefID   string
}

// NormalizedARPEntry 标准化的 ARP 条目
type NormalizedARPEntry struct {
	DeviceIP   string
	IPAddress  string
	MACAddress string
	Interface  string
	Type       string
	CommandKey string
	RawRefID   string
}

// AggregateGroupInfo 聚合组信息
type AggregateGroupInfo struct {
	DeviceIP      string
	AggregateName string
	Mode          string
}

// InterfaceInfo 接口信息
type InterfaceInfo struct {
	DeviceIP      string
	InterfaceName string
	Status        string
	IsAggregate   bool
	AggregateID   string
}

// collectBuildInputs 收集构建输入
func (b *TopologyBuilder) collectBuildInputs(runID string) (*TopologyBuildInput, error) {
	input := &TopologyBuildInput{
		RunID: runID,
	}

	// 收集设备
	if err := b.db.Where("task_run_id = ?", runID).Find(&input.Devices).Error; err != nil {
		return nil, fmt.Errorf("查询设备失败: %w", err)
	}

	// 收集 LLDP 邻居
	if err := b.db.Where("task_run_id = ?", runID).Find(&input.LLDPNeighbors).Error; err != nil {
		return nil, fmt.Errorf("查询 LLDP 邻居失败: %w", err)
	}

	// 收集 FDB 条目
	if err := b.db.Where("task_run_id = ?", runID).Find(&input.FDBEntries).Error; err != nil {
		return nil, fmt.Errorf("查询 FDB 条目失败: %w", err)
	}

	// 收集 ARP 条目
	if err := b.db.Where("task_run_id = ?", runID).Find(&input.ARPEntries).Error; err != nil {
		return nil, fmt.Errorf("查询 ARP 条目失败: %w", err)
	}

	// 收集聚合成员
	if err := b.db.Where("task_run_id = ?", runID).Find(&input.AggMembers).Error; err != nil {
		return nil, fmt.Errorf("查询聚合成员失败: %w", err)
	}

	// 收集聚合组
	if err := b.db.Where("task_run_id = ?", runID).Find(&input.AggGroups).Error; err != nil {
		return nil, fmt.Errorf("查询聚合组失败: %w", err)
	}

	// 收集接口
	if err := b.db.Where("task_run_id = ?", runID).Find(&input.Interfaces).Error; err != nil {
		return nil, fmt.Errorf("查询接口失败: %w", err)
	}

	return input, nil
}

// createFactSnapshot 创建事实快照
func (b *TopologyBuilder) createFactSnapshot(input *TopologyBuildInput) TopologyFactSnapshot {
	snapshot := TopologyFactSnapshot{
		TaskRunID:   input.RunID,
		SnapshotAt:  time.Now(),
		DeviceCount: len(input.Devices),
		LLDPCount:   len(input.LLDPNeighbors),
		FDBCount:    len(input.FDBEntries),
		ARPCount:    len(input.ARPEntries),
		AggCount:    len(input.AggMembers),
		IfCount:     len(input.Interfaces),
	}

	// 计算事实哈希
	hash := sha256.New()
	for _, d := range input.Devices {
		hash.Write([]byte(d.DeviceIP))
	}
	for _, l := range input.LLDPNeighbors {
		hash.Write([]byte(l.DeviceIP + l.LocalInterface + l.NeighborName))
	}
	snapshot.FactHash = hex.EncodeToString(hash.Sum(nil))[:16]

	// 序列化构建配置
	if cfgBytes, err := json.Marshal(input.BuildConfig); err == nil {
		snapshot.BuildConfigSnapshot = string(cfgBytes)
	}

	return snapshot
}

// normalizeFacts 标准化事实
func (b *TopologyBuilder) normalizeFacts(input *TopologyBuildInput) *NormalizedFacts {
	n := &NormalizedFacts{
		Devices:           make(map[string]*DeviceInfo),
		LLDPNeighbors:     make([]NormalizedLLDPNeighbor, 0, len(input.LLDPNeighbors)),
		FDBEntries:        make([]NormalizedFDBEntry, 0, len(input.FDBEntries)),
		ARPEntries:        make([]NormalizedARPEntry, 0, len(input.ARPEntries)),
		AggregateGroups:   make(map[string]*AggregateGroupInfo),
		AggregateMembers:  make(map[string]string),
		Interfaces:        make(map[string]*InterfaceInfo),
		DeviceByName:      make(map[string]string),
		DeviceByMgmtIP:    make(map[string]string),
		DeviceByChassisID: make(map[string]string),
		ARPMACToDevice:    make(map[string]string),
		ARPMACToIP:        make(map[string]string),
	}

	// 标准化设备
	for _, d := range input.Devices {
		info := &DeviceInfo{
			DeviceIP:       d.DeviceIP,
			NormalizedName: strings.ToLower(strings.TrimSpace(d.NormalizedName)),
			ChassisID:      strings.TrimSpace(d.ChassisID),
			MgmtIP:         strings.TrimSpace(d.MgmtIP),
			Hostname:       d.Hostname,
			Vendor:         d.Vendor,
			Model:          d.Model,
		}
		n.Devices[d.DeviceIP] = info

		// 建立名称索引
		if info.NormalizedName != "" {
			n.DeviceByName[info.NormalizedName] = d.DeviceIP
		}
		// 建立 IP 映射（DeviceIP + MgmtIP）
		if d.DeviceIP != "" {
			n.DeviceByMgmtIP[d.DeviceIP] = d.DeviceIP
		}
		if info.MgmtIP != "" && info.MgmtIP != d.DeviceIP {
			n.DeviceByMgmtIP[info.MgmtIP] = d.DeviceIP
		}
		// 建立 ChassisID 索引
		if info.ChassisID != "" {
			n.DeviceByChassisID[info.ChassisID] = d.DeviceIP
		}
	}

	// 标准化 LLDP 邻居
	for _, l := range input.LLDPNeighbors {
		localIf := normalize.NormalizeInterfaceName(l.LocalInterface)
		if localIf == "" {
			continue
		}
		n.LLDPNeighbors = append(n.LLDPNeighbors, NormalizedLLDPNeighbor{
			DeviceIP:        l.DeviceIP,
			LocalIf:         localIf,
			NeighborName:    strings.TrimSpace(l.NeighborName),
			NeighborChassis: strings.TrimSpace(l.NeighborChassis),
			NeighborPort:    normalize.NormalizeInterfaceName(l.NeighborPort),
			NeighborIP:      strings.TrimSpace(l.NeighborIP),
			NeighborDesc:    strings.TrimSpace(l.NeighborDesc),
			CommandKey:      l.CommandKey,
			RawRefID:        l.RawRefID,
		})
	}

	// 标准化 FDB 条目
	for _, f := range input.FDBEntries {
		mac := normalizeMACAddress(f.MACAddress)
		if mac == "" {
			continue
		}
		ifName := normalize.NormalizeInterfaceName(f.Interface)
		if ifName == "" {
			continue
		}
		n.FDBEntries = append(n.FDBEntries, NormalizedFDBEntry{
			DeviceIP:   f.DeviceIP,
			MACAddress: mac,
			VLAN:       f.VLAN,
			Interface:  ifName,
			Type:       f.Type,
			CommandKey: f.CommandKey,
			RawRefID:   f.RawRefID,
		})
	}

	// 标准化 ARP 条目
	for _, a := range input.ARPEntries {
		mac := normalizeMACAddress(a.MACAddress)
		if mac == "" {
			continue
		}
		n.ARPEntries = append(n.ARPEntries, NormalizedARPEntry{
			DeviceIP:   a.DeviceIP,
			IPAddress:  strings.TrimSpace(a.IPAddress),
			MACAddress: mac,
			Interface:  normalize.NormalizeInterfaceName(a.Interface),
			Type:       a.Type,
			CommandKey: a.CommandKey,
			RawRefID:   a.RawRefID,
		})
	}

	// 建立聚合组索引
	for _, g := range input.AggGroups {
		key := g.DeviceIP + "|" + g.AggregateName
		n.AggregateGroups[key] = &AggregateGroupInfo{
			DeviceIP:      g.DeviceIP,
			AggregateName: g.AggregateName,
			Mode:          g.Mode,
		}
	}

	// 建立聚合成员索引
	for _, m := range input.AggMembers {
		key := m.DeviceIP + "|" + normalize.NormalizeInterfaceName(m.MemberPort)
		n.AggregateMembers[key] = m.AggregateName
	}

	// 建立接口索引
	for _, i := range input.Interfaces {
		ifName := normalize.NormalizeInterfaceName(i.InterfaceName)
		if ifName == "" {
			continue
		}
		key := i.DeviceIP + "|" + ifName
		n.Interfaces[key] = &InterfaceInfo{
			DeviceIP:      i.DeviceIP,
			InterfaceName: ifName,
			Status:        i.Status,
			IsAggregate:   i.IsAggregate,
			AggregateID:   i.AggregateID,
		}
	}

	// 建立 ARP MAC -> Device 映射
	for _, a := range n.ARPEntries {
		// 检查 MAC 是否属于已知设备
		for deviceIP, dev := range n.Devices {
			if normalizeMACAddress(dev.ChassisID) == a.MACAddress {
				n.ARPMACToDevice[a.MACAddress] = deviceIP
				break
			}
		}
		n.ARPMACToIP[a.MACAddress] = a.IPAddress
	}

	return n
}

// buildLLDPCandidates 构建 LLDP 候选边
func (b *TopologyBuilder) buildLLDPCandidates(n *NormalizedFacts) []*TopologyEdgeCandidate {
	candidates := make([]*TopologyEdgeCandidate, 0)
	candidateMap := make(map[string]*TopologyEdgeCandidate) // 用于合并双向 LLDP

	for _, lldp := range n.LLDPNeighbors {
		// 解析远端设备
		remoteDevice, resolutionSource := b.resolveLLDPPeer(lldp, n)

		// 解析逻辑接口
		localLogicalIf := b.resolveAggregateInterface(n, lldp.DeviceIP, lldp.LocalIf)
		remoteIf := lldp.NeighborPort
		if remoteIf == "" {
			remoteIf = "unknown"
		}
		remoteLogicalIf := b.resolveAggregateInterface(n, remoteDevice, remoteIf)

		// 构建候选键
		candidateKey := b.buildCandidateKey(lldp.DeviceIP, localLogicalIf, lldp.LocalIf, remoteDevice, remoteLogicalIf, remoteIf)

		// 计算评分
		score := b.scoreLLDPCandidate(lldp, remoteDevice, resolutionSource, localLogicalIf, remoteLogicalIf, n)

		// 构建证据
		evidence := EdgeEvidence{
			Type:       "lldp",
			Source:     "lldp",
			DeviceID:   lldp.DeviceIP,
			Command:    chooseValue(lldp.CommandKey, "lldp_neighbor"),
			RawRefID:   lldp.RawRefID,
			LocalIf:    lldp.LocalIf,
			RemoteName: lldp.NeighborName,
			RemoteIf:   remoteIf,
			RemoteMAC:  lldp.NeighborChassis,
			RemoteIP:   lldp.NeighborIP,
			Summary:    fmt.Sprintf("LLDP %s -> %s(%s)", chooseValue(localLogicalIf, lldp.LocalIf), lldp.NeighborName, chooseValue(remoteLogicalIf, remoteIf)),
		}

		// 检查是否已有相同候选（双向 LLDP）
		if existing, ok := candidateMap[candidateKey]; ok {
			// 合并证据
			existing.EvidenceRefs = append(existing.EvidenceRefs, evidence)
			// 更新特征
			existing.Features = appendUniqueStrings(existing.Features, "lldp_bidirectional")
			// 更新评分 - 标记为双向
			existing.score.LLDPScore.Bidirectional = true
			existing.score.TotalScore = b.recalculateLLDPScore(existing.score)
			existing.TotalScore = existing.score.TotalScore
		} else {
			// 创建新候选
			features := []string{"lldp_single_side"}
			if localLogicalIf != "" || remoteLogicalIf != "" {
				features = appendUniqueStrings(features, "aggregate_mapped")
			}

			candidate := &TopologyEdgeCandidate{
				TaskRunID:    n.Devices[lldp.DeviceIP].DeviceIP + "_" + lldp.LocalIf + "_" + remoteDevice, // 临时 ID
				CandidateID:  makeTaskEdgeID(),
				ADeviceID:    lldp.DeviceIP,
				AIf:          lldp.LocalIf,
				LogicalAIf:   localLogicalIf,
				BDeviceID:    remoteDevice,
				BIf:          remoteIf,
				LogicalBIf:   remoteLogicalIf,
				EdgeType:     "physical",
				Source:       "lldp",
				Status:       "pending",
				TotalScore:   score.TotalScore,
				Features:     features,
				EvidenceRefs: []EdgeEvidence{evidence},
				score:        score,
			}
			candidates = append(candidates, candidate)
			candidateMap[candidateKey] = candidate
		}
	}

	return candidates
}

// recalculateLLDPScore 重新计算 LLDP 评分
func (b *TopologyBuilder) recalculateLLDPScore(score ScoreBreakdown) float64 {
	total := score.LLDPScore.BaseScore +
		score.LLDPScore.IPMatch +
		score.LLDPScore.NameMatch +
		score.LLDPScore.ChassisMatch +
		score.LLDPScore.RemoteIfPresent +
		score.AggregateScore.AggregateModeScore

	// 双向加分
	if score.LLDPScore.Bidirectional {
		total += 25.0 // 双向确认额外加分
	}

	return total
}

// resolveLLDPPeer 解析 LLDP 对端设备
// 穿透式匹配：IP → ChassisID → Name，任一维度匹配失败不中断
func (b *TopologyBuilder) resolveLLDPPeer(lldp NormalizedLLDPNeighbor, n *NormalizedFacts) (string, string) {
	// 1. 优先尝试 NeighborIP 匹配（依赖 DeviceIP 已入库 DeviceByMgmtIP）
	if lldp.NeighborIP != "" {
		if deviceIP, ok := n.DeviceByMgmtIP[lldp.NeighborIP]; ok {
			logger.Verbose("TopologyBuilder", lldp.DeviceIP,
				"resolveLLDPPeer: localIf=%s, neighborIP=%s → matched via neighbor_ip → deviceIP=%s",
				lldp.LocalIf, lldp.NeighborIP, deviceIP)
			return deviceIP, "neighbor_ip"
		}
		// ⚠ 不要在这里 return！继续尝试其他维度
	}

	// 2. 其次尝试 ChassisID 匹配（硬件 MAC 比设备名更可靠）
	if lldp.NeighborChassis != "" {
		if deviceIP, ok := n.DeviceByChassisID[lldp.NeighborChassis]; ok {
			logger.Verbose("TopologyBuilder", lldp.DeviceIP,
				"resolveLLDPPeer: localIf=%s, neighborChassis=%s → matched via chassis_id → deviceIP=%s",
				lldp.LocalIf, lldp.NeighborChassis, deviceIP)
			return deviceIP, "chassis_id"
		}
	}

	// 3. 最后尝试 NeighborName 匹配
	if lldp.NeighborName != "" {
		normalizedName := strings.ToLower(strings.TrimSpace(lldp.NeighborName))
		if deviceIP, ok := n.DeviceByName[normalizedName]; ok {
			logger.Verbose("TopologyBuilder", lldp.DeviceIP,
				"resolveLLDPPeer: localIf=%s, neighborName=%s → matched via neighbor_name → deviceIP=%s",
				lldp.LocalIf, lldp.NeighborName, deviceIP)
			return deviceIP, "neighbor_name"
		}
	}

	// 4. 全部匹配失败 → 标记为未管设备 (Unmanaged Node)
	//    优先使用 IP 作为占位标识，其次 ChassisID，最后拼接 DeviceIP+LocalIf
	fallbackID := lldp.NeighborIP
	if fallbackID == "" {
		fallbackID = lldp.NeighborChassis
	}
	if fallbackID == "" {
		fallbackID = lldp.DeviceIP + ":" + lldp.LocalIf
	}
	logger.Verbose("TopologyBuilder", lldp.DeviceIP,
		"resolveLLDPPeer: localIf=%s, neighborIP=%s, neighborChassis=%s, neighborName=%s → no match → unmanaged:%s",
		lldp.LocalIf, lldp.NeighborIP, lldp.NeighborChassis, lldp.NeighborName, fallbackID)
	return "unmanaged:" + fallbackID, "unknown_peer"
}

// scoreLLDPCandidate 计算 LLDP 候选评分
func (b *TopologyBuilder) scoreLLDPCandidate(lldp NormalizedLLDPNeighbor, remoteDevice, resolutionSource, localLogicalIf, remoteLogicalIf string, n *NormalizedFacts) ScoreBreakdown {
	score := ScoreBreakdown{Version: "1.0"}

	// 基础分
	if resolutionSource == "neighbor_ip" || resolutionSource == "neighbor_name" || resolutionSource == "chassis_id" {
		score.LLDPScore.BaseScore = wLLDPBaseSingleSide
	} else {
		score.LLDPScore.BaseScore = wLLDPBaseSingleSide * 0.8 // 未知对端降权
	}

	score.LLDPScore.ResolutionSource = resolutionSource

	// IP 匹配加分
	if lldp.NeighborIP != "" && resolutionSource == "neighbor_ip" {
		score.LLDPScore.IPMatch = wLLDPIPMatch
	}

	// 名称匹配加分
	if lldp.NeighborName != "" && resolutionSource == "neighbor_name" {
		score.LLDPScore.NameMatch = wLLDPNameMatch
	}

	// Chassis 匹配加分
	if lldp.NeighborChassis != "" {
		if _, ok := n.DeviceByChassisID[lldp.NeighborChassis]; ok {
			score.LLDPScore.ChassisMatch = wLLDPChassisMatch
		}
	}

	// 远端接口存在加分
	if lldp.NeighborPort != "" {
		score.LLDPScore.RemoteIfPresent = wLLDPRemoteIfPresent
	}

	// 聚合接口加分
	if localLogicalIf != "" || remoteLogicalIf != "" {
		score.AggregateScore.IsAggregateLink = true
		score.AggregateScore.AggregateModeScore = wAggLACPModeBonus
	}

	// 计算总分
	score.TotalScore = score.LLDPScore.BaseScore +
		score.LLDPScore.IPMatch +
		score.LLDPScore.NameMatch +
		score.LLDPScore.ChassisMatch +
		score.LLDPScore.RemoteIfPresent +
		score.AggregateScore.AggregateModeScore

	// 计算置信度
	score.Confidence = score.TotalScore / 100.0
	if score.Confidence > 1.0 {
		score.Confidence = 1.0
	}

	return score
}

// buildFDBARPCandidates 构建 FDB/ARP 推断候选
func (b *TopologyBuilder) buildFDBARPCandidates(n *NormalizedFacts) []*TopologyEdgeCandidate {
	candidates := make([]*TopologyEdgeCandidate, 0)
	candidateMap := make(map[string]*TopologyEdgeCandidate)

	// 按 (DeviceIP, Interface) 分组 FDB 条目
	type fdbGroup struct {
		macs       []string
		vlans      map[int]bool
		types      map[string]bool
		rawRefIDs  []string
		commandKey string
	}
	fdbGroups := make(map[string]*fdbGroup)

	for _, fdb := range n.FDBEntries {
		key := fdb.DeviceIP + "|" + fdb.Interface
		if _, ok := fdbGroups[key]; !ok {
			fdbGroups[key] = &fdbGroup{
				vlans: make(map[int]bool),
				types: make(map[string]bool),
			}
		}
		g := fdbGroups[key]
		g.macs = appendUniqueStrings(g.macs, fdb.MACAddress)
		g.vlans[fdb.VLAN] = true
		g.types[fdb.Type] = true
		g.rawRefIDs = append(g.rawRefIDs, fdb.RawRefID)
		g.commandKey = fdb.CommandKey
	}

	// 为每个 FDB 组生成候选
	for endpointKey, g := range fdbGroups {
		parts := strings.SplitN(endpointKey, "|", 2)
		if len(parts) != 2 {
			continue
		}
		deviceIP, localIf := parts[0], parts[1]

		// 解析逻辑接口
		localLogicalIf := b.resolveAggregateInterface(n, deviceIP, localIf)

		// 统计候选对端（同时收集MAC信息）
		candidatePeers := make(map[string]int)           // remoteDevice -> count
		candidatePeerMACs := make(map[string][]string)   // remoteDevice -> []mac
		for _, mac := range g.macs {
			remoteDevice, _, _, resolvedMAC := b.resolveFDBRemoteEndpoint(deviceIP, mac, n)
			if remoteDevice != "" && remoteDevice != deviceIP {
				candidatePeers[remoteDevice]++
				if resolvedMAC != "" {
					candidatePeerMACs[remoteDevice] = append(candidatePeerMACs[remoteDevice], resolvedMAC)
				}
			}
		}

		// 如果候选对端过多，跳过
		if len(candidatePeers) > b.config.MaxInferenceCandidates {
			continue
		}

		// 为每个候选对端生成候选边
		for remoteDevice, macCount := range candidatePeers {
			remoteKind := b.classifyEndpoint(remoteDevice, n)
			remoteIP := ""
			for _, mac := range g.macs {
				if ip, ok := n.ARPMACToIP[mac]; ok {
					remoteIP = ip
					break
				}
			}

			remoteIf := "unknown"
			if strings.HasPrefix(remoteDevice, "server:") || strings.HasPrefix(remoteDevice, "terminal:") {
				remoteIf = "access"
			}

			// 计算评分
			score := b.scoreFDBARPCandidate(remoteKind, macCount, localLogicalIf != "", remoteIP != "", g.vlans, g.types)

			// 构建候选键
			candidateKey := b.buildCandidateKey(deviceIP, localLogicalIf, localIf, remoteDevice, "", remoteIf)

			// 构建证据（保留MAC信息）
			macList := candidatePeerMACs[remoteDevice]
			evidence := EdgeEvidence{
				Type:       "fdb_arp",
				Source:     "fdb",
				DeviceID:   deviceIP,
				Command:    chooseValue(g.commandKey, "arp_all"),
				RawRefID:   strings.Join(g.rawRefIDs, ","),
				LocalIf:    localIf,
				RemoteName: remoteDevice,
				RemoteIf:   remoteIf,
				RemoteMAC:  strings.Join(macList, ","), // 保留所有MAC
				RemoteIP:   remoteIP,
				Summary:    fmt.Sprintf("FDB/ARP 推断 %s -> %s via %s, macs=%d, kind=%s", deviceIP, remoteDevice, chooseValue(localLogicalIf, localIf), macCount, remoteKind),
			}

			// 检查是否已有相同候选
			if existing, ok := candidateMap[candidateKey]; ok {
				existing.EvidenceRefs = append(existing.EvidenceRefs, evidence)
				// 更新评分
				existing.score.FDBARPScore.MACCount += macCount
				existing.score.FDBARPScore.MACCountScore = float64(existing.score.FDBARPScore.MACCount) * wFDBMACCountFactor
				existing.score.TotalScore = b.recalculateFDBARPScore(existing.score)
				existing.TotalScore = existing.score.TotalScore
			} else {
				features := []string{"fdb_arp_inference"}
				if localLogicalIf != "" {
					features = appendUniqueStrings(features, "aggregate_mapped")
				}

				candidate := &TopologyEdgeCandidate{
					CandidateID:  makeTaskEdgeID(),
					ADeviceID:    deviceIP,
					AIf:          localIf,
					LogicalAIf:   localLogicalIf,
					BDeviceID:    remoteDevice,
					BIf:          remoteIf,
					LogicalBIf:   "",
					EdgeType:     "physical",
					Source:       "fdb_arp",
					Status:       "pending",
					TotalScore:   score.TotalScore,
					Features:     features,
					EvidenceRefs: []EdgeEvidence{evidence},
					BDeviceMACs:  macList,
					score:        score,
				}
				candidates = append(candidates, candidate)
				candidateMap[candidateKey] = candidate
			}
		}
	}

	return candidates
}

// recalculateFDBARPScore 重新计算 FDB/ARP 评分
func (b *TopologyBuilder) recalculateFDBARPScore(score ScoreBreakdown) float64 {
	return score.FDBARPScore.BaseScore +
		score.FDBARPScore.MACCountScore +
		score.FDBARPScore.EndpointScore +
		score.FDBARPScore.VLANScore +
		score.FDBARPScore.LogicalIfScore +
		score.FDBARPScore.RemoteIPScore
}

// resolveFDBRemoteEndpoint 解析 FDB 远端端点
// 返回值: (节点标识, 节点类型, IP地址, MAC地址)
// 混合标识方案：使用IP作为节点标识，同时返回MAC用于追溯
func (b *TopologyBuilder) resolveFDBRemoteEndpoint(deviceIP, mac string, n *NormalizedFacts) (nodeID, kind, ip, resolvedMAC string) {
	// 检查 MAC 是否属于已知设备
	if deviceIP, ok := n.ARPMACToDevice[mac]; ok {
		return deviceIP, "device", "", mac
	}

	// 检查 MAC 是否有 ARP 记录
	if ip, ok := n.ARPMACToIP[mac]; ok {
		// 根据特征判断类型
		kind := "terminal"
		if strings.HasPrefix(ip, "192.168.") || strings.HasPrefix(ip, "10.") {
			kind = "server"
		}
		// 使用IP作为标识，同时返回MAC用于追溯
		return kind + ":" + ip, kind, ip, mac
	}

	// 未知 MAC - 仍使用MAC作为标识（因为没有IP）
	return "unknown:" + mac, "unknown", "", mac
}

// classifyEndpoint 分类端点类型
func (b *TopologyBuilder) classifyEndpoint(remoteDevice string, n *NormalizedFacts) string {
	if _, ok := n.Devices[remoteDevice]; ok {
		return "device"
	}
	if strings.HasPrefix(remoteDevice, "server:") {
		return "server"
	}
	if strings.HasPrefix(remoteDevice, "terminal:") {
		return "terminal"
	}
	return "unknown"
}

// scoreFDBARPCandidate 计算 FDB/ARP 候选评分
func (b *TopologyBuilder) scoreFDBARPCandidate(remoteKind string, macCount int, hasLogicalIf, hasRemoteIP bool, vlans map[int]bool, types map[string]bool) ScoreBreakdown {
	score := ScoreBreakdown{Version: "1.0"}

	// 基础分
	score.FDBARPScore.BaseScore = wFDBBaseScore

	// MAC 数量评分
	score.FDBARPScore.MACCount = macCount
	score.FDBARPScore.MACCountScore = float64(macCount) * wFDBMACCountFactor

	// 端点类型评分
	score.FDBARPScore.EndpointKind = remoteKind
	switch remoteKind {
	case "device":
		score.FDBARPScore.EndpointScore = wFDBDeviceBonus
	case "server":
		score.FDBARPScore.EndpointScore = wFDBServerBonus
	case "terminal":
		score.FDBARPScore.EndpointScore = wFDBTerminalBonus
	}

	// VLAN 一致性
	if len(vlans) == 1 {
		score.FDBARPScore.VLANConsistent = true
		score.FDBARPScore.VLANScore = wFDBVLANBonus
	}

	// 聚合接口加分
	if hasLogicalIf {
		score.FDBARPScore.HasLogicalIf = true
		score.FDBARPScore.LogicalIfScore = wFDBLogicalIfBonus
	}

	// 远端 IP 加分
	if hasRemoteIP {
		score.FDBARPScore.HasRemoteIP = true
		score.FDBARPScore.RemoteIPScore = wFDBRemoteIPBonus
	}

	// 计算总分
	score.TotalScore = score.FDBARPScore.BaseScore +
		score.FDBARPScore.MACCountScore +
		score.FDBARPScore.EndpointScore +
		score.FDBARPScore.VLANScore +
		score.FDBARPScore.LogicalIfScore +
		score.FDBARPScore.RemoteIPScore

	// 计算置信度
	score.Confidence = score.TotalScore / 100.0
	if score.Confidence > 0.95 {
		score.Confidence = 0.95
	}
	if score.Confidence < 0.35 {
		score.Confidence = 0.35
	}

	return score
}

// enrichCandidatesWithInterfaceFacts 用接口事实丰富候选
func (b *TopologyBuilder) enrichCandidatesWithInterfaceFacts(candidates []*TopologyEdgeCandidate, n *NormalizedFacts) {
	for _, c := range candidates {
		// 检查本端接口状态
		localIfKey := c.ADeviceID + "|" + c.AIf
		if localIf, ok := n.Interfaces[localIfKey]; ok {
			if localIf.Status == "up" {
				c.score.InterfaceScore.LocalIfUp = true
				c.score.InterfaceScore.LocalIfUpScore = wIfUpBonus
			}
		}

		// 检查远端接口状态（如果有）
		if c.BDeviceID != "" && c.BIf != "" && c.BIf != "unknown" && c.BIf != "access" {
			remoteIfKey := c.BDeviceID + "|" + c.BIf
			if remoteIf, ok := n.Interfaces[remoteIfKey]; ok {
				if remoteIf.Status == "up" {
					c.score.InterfaceScore.RemoteIfUp = true
					c.score.InterfaceScore.RemoteIfUpScore = wIfUpBonus
				}
			}
		}

		// 更新总分
		c.score.TotalScore += c.score.InterfaceScore.LocalIfUpScore + c.score.InterfaceScore.RemoteIfUpScore
		c.TotalScore = c.score.TotalScore
	}
}

// resolveCandidatesGlobal 全局冲突消解
func (b *TopologyBuilder) resolveCandidatesGlobal(candidates []*TopologyEdgeCandidate) ([]*TopologyEdgeCandidate, []TopologyDecisionTrace) {
	traces := make([]TopologyDecisionTrace, 0)

	// 按端点分组
	type endpointGroup struct {
		candidates []*TopologyEdgeCandidate
	}
	groups := make(map[string]*endpointGroup)

	for _, c := range candidates {
		// 本端端点
		localA := c.ADeviceID + "|" + chooseValue(c.LogicalAIf, c.AIf, "unknown")
		if _, ok := groups[localA]; !ok {
			groups[localA] = &endpointGroup{candidates: make([]*TopologyEdgeCandidate, 0)}
		}
		groups[localA].candidates = append(groups[localA].candidates, c)

		// 远端端点
		if c.BDeviceID != "" && !strings.HasPrefix(c.BDeviceID, "unknown:") && !strings.HasPrefix(c.BDeviceID, "server:") && !strings.HasPrefix(c.BDeviceID, "terminal:") {
			localB := c.BDeviceID + "|" + chooseValue(c.LogicalBIf, c.BIf, "unknown")
			if _, ok := groups[localB]; !ok {
				groups[localB] = &endpointGroup{candidates: make([]*TopologyEdgeCandidate, 0)}
			}
			groups[localB].candidates = append(groups[localB].candidates, c)
		}
	}

	// 对每个端点组进行冲突消解
	resolved := make(map[string]bool)
	for endpoint, g := range groups {
		if len(g.candidates) <= 1 {
			continue
		}

		// 按评分排序
		sort.Slice(g.candidates, func(i, j int) bool {
			return g.candidates[i].TotalScore > g.candidates[j].TotalScore
		})

		// 检查是否有竞争
		topScore := g.candidates[0].TotalScore
		hasCloseCompetitor := false
		for i := 1; i < len(g.candidates); i++ {
			if g.candidates[i].TotalScore >= topScore-b.config.ConflictWindow {
				hasCloseCompetitor = true
				break
			}
		}

		// 创建决策轨迹
		trace := TopologyDecisionTrace{
			TraceID:       makeTaskEdgeID(),
			DecisionType:  "conflict_resolution",
			DecisionGroup: endpoint,
		}

		decisionCandidates := make([]DecisionCandidate, 0, len(g.candidates))
		for _, c := range g.candidates {
			decisionCandidates = append(decisionCandidates, DecisionCandidate{
				CandidateID: c.CandidateID,
				ADeviceID:   c.ADeviceID,
				AIf:         c.AIf,
				BDeviceID:   c.BDeviceID,
				BIf:         c.BIf,
				TotalScore:  c.TotalScore,
				Confidence:  c.TotalScore / 100.0,
				Source:      c.Source,
				Features:    c.Features,
			})
		}
		if bytes, err := json.Marshal(decisionCandidates); err == nil {
			trace.Candidates = string(bytes)
		}

		if hasCloseCompetitor {
			// 标记所有竞争者为冲突
			trace.DecisionResult = "conflict"
			trace.DecisionReason = "multiple close candidates within conflict window"
			trace.DecisionBasis = fmt.Sprintf("top_score=%.2f, window=%.2f, candidate_count=%d", topScore, b.config.ConflictWindow, len(g.candidates))

			for _, c := range g.candidates {
				if !resolved[c.CandidateID] {
					c.Status = "conflict"
					c.DecisionReason = "conflict: multiple close candidates"
					resolved[c.CandidateID] = true
				}
				trace.RejectedCandidateIDs = append(trace.RejectedCandidateIDs, c.CandidateID)
			}
		} else {
			// 保留最高分，淘汰其他
			trace.DecisionResult = "retained"
			trace.DecisionReason = "top candidate retained"
			trace.DecisionBasis = fmt.Sprintf("top_score=%.2f, second_score=%.2f, gap=%.2f", topScore, g.candidates[1].TotalScore, topScore-g.candidates[1].TotalScore)

			// 保留第一个
			if !resolved[g.candidates[0].CandidateID] {
				g.candidates[0].Status = "retained"
				g.candidates[0].DecisionReason = "highest score candidate"
				trace.RetainedCandidateIDs = append(trace.RetainedCandidateIDs, g.candidates[0].CandidateID)
				resolved[g.candidates[0].CandidateID] = true
			}

			// 淘汰其他
			for i := 1; i < len(g.candidates); i++ {
				if !resolved[g.candidates[i].CandidateID] {
					g.candidates[i].Status = "rejected"
					g.candidates[i].DecisionReason = fmt.Sprintf("lower score: %.2f vs %.2f", g.candidates[i].TotalScore, topScore)
					trace.RejectedCandidateIDs = append(trace.RejectedCandidateIDs, g.candidates[i].CandidateID)
					resolved[g.candidates[i].CandidateID] = true
				}
			}
		}

		traces = append(traces, trace)
	}

	// 标记未处理的候选为保留
	retained := make([]*TopologyEdgeCandidate, 0)
	for _, c := range candidates {
		if c.Status == "pending" {
			c.Status = "retained"
			c.DecisionReason = "no conflict"
		}
		if c.Status == "retained" || c.Status == "conflict" {
			retained = append(retained, c)
		}
	}

	return retained, traces
}

// materializeEdges 生成最终边
func (b *TopologyBuilder) materializeEdges(candidates []*TopologyEdgeCandidate, runID string) []TaskTopologyEdge {
	edges := make([]TaskTopologyEdge, 0, len(candidates))

	for _, c := range candidates {
		if c.Status == "rejected" {
			continue
		}

		// 确定边状态
		status := "inferred"
		confidence := c.TotalScore / 100.0
		if confidence > confidenceConfirmed {
			status = "confirmed"
		} else if confidence > confidenceSemiConfirmed {
			status = "semi_confirmed"
		}

		if c.Status == "conflict" {
			status = "conflict"
		}

		// 提取B端设备MAC信息
		bDeviceMAC := ""
		bDeviceMACsJSON := ""
		if len(c.BDeviceMACs) > 0 {
			bDeviceMAC = c.BDeviceMACs[0] // 主MAC
			if len(c.BDeviceMACs) > 1 {
				// 多MAC序列化为JSON数组
				macsJSON, _ := json.Marshal(c.BDeviceMACs)
				bDeviceMACsJSON = string(macsJSON)
			}
		} else {
			// 从Evidence中提取MAC（兼容旧逻辑）
			for _, ev := range c.EvidenceRefs {
				if ev.RemoteMAC != "" {
					bDeviceMAC = ev.RemoteMAC
					break
				}
			}
		}

		edge := TaskTopologyEdge{
			ID:                  makeTaskEdgeID(),
			TaskRunID:           runID,
			ADeviceID:           c.ADeviceID,
			AIf:                 c.AIf,
			BDeviceID:           c.BDeviceID,
			BIf:                 c.BIf,
			BDeviceMAC:          bDeviceMAC,      // 填充B端MAC
			BDeviceMACs:         bDeviceMACsJSON, // 填充多MAC
			LogicalAIf:          c.LogicalAIf,
			LogicalBIf:          c.LogicalBIf,
			EdgeType:            c.EdgeType,
			Status:              status,
			Confidence:          confidence,
			DiscoveryMethods:    c.Features,
			Evidence:            c.EvidenceRefs,
			ConfidenceBreakdown: b.serializeScoreBreakdown(c.score),
			DecisionReason:      c.DecisionReason,
			CandidateID:         c.CandidateID,
		}

		edges = append(edges, edge)
	}

	return edges
}

// persistResults 持久化结果
func (b *TopologyBuilder) persistResults(runID string, edges []TaskTopologyEdge, candidates []*TopologyEdgeCandidate, traces []TopologyDecisionTrace, snapshot TopologyFactSnapshot) error {
	// 清理旧数据
	if err := b.db.Where("task_run_id = ?", runID).Delete(&TaskTopologyEdge{}).Error; err != nil {
		return err
	}

	// 保存边
	if len(edges) > 0 {
		if err := b.db.Create(&edges).Error; err != nil {
			return err
		}
	}

	// 保存候选（如果配置启用）
	if b.config.SaveCandidates {
		if err := b.db.Where("task_run_id = ?", runID).Delete(&TopologyEdgeCandidate{}).Error; err != nil {
			return err
		}
		for i := range candidates {
			candidates[i].TaskRunID = runID
			// 将运行时 score 字段序列化为持久化字段
			candidates[i].ScoreBreakdown = b.serializeScoreBreakdown(candidates[i].score)
		}
		if len(candidates) > 0 {
			if err := b.db.Create(&candidates).Error; err != nil {
				return err
			}
		}
	}

	// 保存决策轨迹（如果配置启用）
	if b.config.SaveDecisionTraces {
		if err := b.db.Where("task_run_id = ?", runID).Delete(&TopologyDecisionTrace{}).Error; err != nil {
			return err
		}
		for i := range traces {
			traces[i].TaskRunID = runID
		}
		if len(traces) > 0 {
			if err := b.db.Create(&traces).Error; err != nil {
				return err
			}
		}
	}

	// 保存事实快照
	if err := b.db.Where("task_run_id = ?", runID).Delete(&TopologyFactSnapshot{}).Error; err != nil {
		return err
	}
	snapshot.TaskRunID = runID
	if err := b.db.Create(&snapshot).Error; err != nil {
		return err
	}

	return nil
}

// computeStatistics 计算统计信息
func (b *TopologyBuilder) computeStatistics(edges []TaskTopologyEdge, candidates []*TopologyEdgeCandidate, traces []TopologyDecisionTrace, startedAt time.Time) TopologyBuildStatistics {
	stats := TopologyBuildStatistics{
		TotalEdges:         len(edges),
		TotalCandidates:    len(candidates),
		DecisionTraceCount: len(traces),
		BuildDuration:      time.Since(startedAt),
	}

	for _, e := range edges {
		switch e.Status {
		case "confirmed":
			stats.ConfirmedEdges++
		case "semi_confirmed":
			stats.SemiConfirmedEdges++
		case "inferred":
			stats.InferredEdges++
		case "conflict":
			stats.ConflictEdges++
		}
	}

	for _, c := range candidates {
		switch c.Status {
		case "retained":
			stats.RetainedCandidates++
		case "rejected":
			stats.RejectedCandidates++
		}
	}

	return stats
}

// generateErrors 生成错误信息
func (b *TopologyBuilder) generateErrors(n *NormalizedFacts, stats TopologyBuildStatistics) []string {
	errors := make([]string, 0)

	if len(n.LLDPNeighbors) == 0 {
		if stats.InferredEdges > 0 {
			errors = append(errors, "未解析到任何 LLDP 邻居事实，当前拓扑边来自 FDB/ARP 推断，请重点关注置信度与冲突验证")
		} else {
			errors = append(errors, "未解析到任何 LLDP 邻居事实，请重点检查 LLDP 采集命令输出与解析模板是否匹配")
		}
	}

	if len(n.LLDPNeighbors) > 0 && stats.TotalEdges == 0 {
		errors = append(errors, "存在 LLDP 邻居事实，但未成功构建出任何拓扑边，请检查接口名规范化和邻居设备映射逻辑")
	}

	if stats.ConflictEdges > 0 {
		errors = append(errors, fmt.Sprintf("有 %d 条拓扑边因同端口多候选冲突被标记为 conflict，请结合证据评分进一步确认", stats.ConflictEdges))
	}

	return errors
}

// 辅助方法

func (b *TopologyBuilder) resolveAggregateInterface(n *NormalizedFacts, deviceIP, ifName string) string {
	key := deviceIP + "|" + ifName
	if aggName, ok := n.AggregateMembers[key]; ok {
		return aggName
	}
	return ""
}

func (b *TopologyBuilder) buildCandidateKey(aDevice, aLogicalIf, aIf, bDevice, bLogicalIf, bIf string) string {
	left := aDevice + ":" + chooseValue(aLogicalIf, aIf, "unknown")
	right := bDevice + ":" + chooseValue(bLogicalIf, bIf, "unknown")
	pair := []string{left, right}
	sort.Strings(pair)
	return pair[0] + "<->" + pair[1]
}

func (b *TopologyBuilder) serializeScoreBreakdown(score ScoreBreakdown) string {
	if bytes, err := json.Marshal(score); err == nil {
		return string(bytes)
	}
	return "{}"
}

// EnsureDBTables 确保数据库表存在
func EnsureDBTables(db *gorm.DB) error {
	return db.AutoMigrate(
		&TopologyFactSnapshot{},
		&TopologyEdgeCandidate{},
		&TopologyDecisionTrace{},
		&TaskTopologyEdge{},
	)
}

// BuildTopologyWithNewLogic 使用新逻辑构建拓扑（入口函数）
func BuildTopologyWithNewLogic(db *gorm.DB, runID string) (*TopologyBuildOutput, error) {
	cfg := DefaultTopologyBuildConfig()

	// 从运行时配置加载参数
	maxCandidates, conflictWindow, _, saveCandidates, saveTraces := config.ResolveTopologyConfig()
	if maxCandidates > 0 {
		cfg.MaxInferenceCandidates = maxCandidates
	}
	if conflictWindow > 0 {
		cfg.ConflictWindow = conflictWindow
	}
	cfg.SaveCandidates = saveCandidates
	cfg.SaveDecisionTraces = saveTraces

	builder := NewTopologyBuilder(db, cfg)
	return builder.Build(context.Background(), runID)
}
