package taskexec

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Phase D: 测试与质量门禁
// =============================================================================

// TestScoreBreakdownSerialization 测试评分明细序列化
func TestScoreBreakdownSerialization(t *testing.T) {
	score := ScoreBreakdown{
		Version: "1.0",
		LLDPScore: LLDPScoreDetail{
			BaseScore:        75.0,
			ChassisMatch:     5.0,
			NameMatch:        3.0,
			IPMatch:          5.0,
			RemoteIfPresent:  2.0,
			Bidirectional:    false,
			ResolutionSource: "neighbor_ip",
		},
		TotalScore: 90.0,
		Confidence: 0.9,
	}

	// 序列化
	bytes, err := json.Marshal(score)
	require.NoError(t, err)

	// 反序列化
	var decoded ScoreBreakdown
	err = json.Unmarshal(bytes, &decoded)
	require.NoError(t, err)

	assert.Equal(t, score.Version, decoded.Version)
	assert.Equal(t, score.LLDPScore.BaseScore, decoded.LLDPScore.BaseScore)
	assert.Equal(t, score.LLDPScore.ChassisMatch, decoded.LLDPScore.ChassisMatch)
	assert.Equal(t, score.TotalScore, decoded.TotalScore)
	assert.Equal(t, score.Confidence, decoded.Confidence)
}

// TestDefaultTopologyBuildConfig 测试默认配置
func TestDefaultTopologyBuildConfig(t *testing.T) {
	cfg := DefaultTopologyBuildConfig()

	assert.Equal(t, 5, cfg.MaxInferenceCandidates)
	assert.Equal(t, 10.0, cfg.ConflictWindow)
	assert.False(t, cfg.UseNewBuilder)
	assert.True(t, cfg.SaveCandidates)
	assert.True(t, cfg.SaveDecisionTraces)

	// LLDP 权重
	assert.Equal(t, 75.0, cfg.LLDPWeights.BaseSingleSide)
	assert.Equal(t, 100.0, cfg.LLDPWeights.BaseBidirectional)
	assert.Equal(t, 5.0, cfg.LLDPWeights.ChassisMatch)
	assert.Equal(t, 3.0, cfg.LLDPWeights.NameMatch)
	assert.Equal(t, 5.0, cfg.LLDPWeights.IPMatch)
	assert.Equal(t, 2.0, cfg.LLDPWeights.RemoteIfPresent)

	// FDB/ARP 权重
	assert.Equal(t, 20.0, cfg.FDBARPWeights.BaseScore)
	assert.Equal(t, 2.0, cfg.FDBARPWeights.MACCountFactor)
	assert.Equal(t, 30.0, cfg.FDBARPWeights.DeviceBonus)
	assert.Equal(t, 15.0, cfg.FDBARPWeights.ServerBonus)
	assert.Equal(t, 5.0, cfg.FDBARPWeights.TerminalBonus)

	// 置信度阈值
	assert.Equal(t, 0.95, cfg.ConfidenceThresholds.Confirmed)
	assert.Equal(t, 0.75, cfg.ConfidenceThresholds.SemiConfirmed)
	assert.Equal(t, 0.35, cfg.ConfidenceThresholds.Inferred)
}

// TestBuildCandidateKey 测试候选键生成
func TestBuildCandidateKey(t *testing.T) {
	builder := &TopologyBuilder{config: DefaultTopologyBuildConfig()}

	// 测试相同端点生成相同键（无论顺序）
	key1 := builder.buildCandidateKey("192.168.1.1", "Eth-Trunk1", "GigabitEthernet0/0/1", "192.168.1.2", "Eth-Trunk2", "GigabitEthernet0/0/2")
	key2 := builder.buildCandidateKey("192.168.1.2", "Eth-Trunk2", "GigabitEthernet0/0/2", "192.168.1.1", "Eth-Trunk1", "GigabitEthernet0/0/1")
	assert.Equal(t, key1, key2, "候选键应该与端点顺序无关")
}

// TestLLDPScoreCalculation 测试 LLDP 评分计算
func TestLLDPScoreCalculation(t *testing.T) {
	cfg := DefaultTopologyBuildConfig()
	builder := &TopologyBuilder{config: cfg}

	n := &NormalizedFacts{
		Devices:           make(map[string]*DeviceInfo),
		DeviceByChassisID: make(map[string]string),
	}

	// 添加设备信息
	n.Devices["192.168.1.1"] = &DeviceInfo{DeviceIP: "192.168.1.1"}
	n.DeviceByChassisID["aa:bb:cc:dd:ee:ff"] = "192.168.1.2"

	tests := []struct {
		name             string
		lldp             NormalizedLLDPNeighbor
		remoteDevice     string
		resolutionSource string
		expectedMinScore float64
	}{
		{
			name: "IP 解析高分",
			lldp: NormalizedLLDPNeighbor{
				DeviceIP:        "192.168.1.1",
				LocalIf:         "GigabitEthernet0/0/1",
				NeighborIP:      "192.168.1.2",
				NeighborName:    "Switch2",
				NeighborPort:    "GigabitEthernet0/0/1",
				NeighborChassis: "aa:bb:cc:dd:ee:ff",
			},
			remoteDevice:     "192.168.1.2",
			resolutionSource: "neighbor_ip",
			expectedMinScore: 75.0, // 基础分
		},
		{
			name: "名称解析中等分",
			lldp: NormalizedLLDPNeighbor{
				DeviceIP:     "192.168.1.1",
				LocalIf:      "GigabitEthernet0/0/2",
				NeighborName: "Switch2",
				NeighborPort: "GigabitEthernet0/0/2",
			},
			remoteDevice:     "192.168.1.2",
			resolutionSource: "neighbor_name",
			expectedMinScore: 75.0,
		},
		{
			name: "未知对端降权",
			lldp: NormalizedLLDPNeighbor{
				DeviceIP: "192.168.1.1",
				LocalIf:  "GigabitEthernet0/0/3",
			},
			remoteDevice:     "unknown:192.168.1.1:GigabitEthernet0/0/3",
			resolutionSource: "unknown_peer",
			expectedMinScore: 50.0, // 降权后的基础分
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := builder.scoreLLDPCandidate(tt.lldp, tt.remoteDevice, tt.resolutionSource, "", "", n)
			assert.GreaterOrEqual(t, score.TotalScore, tt.expectedMinScore, "评分应大于等于预期最小值")
			assert.GreaterOrEqual(t, score.Confidence, 0.0)
			assert.LessOrEqual(t, score.Confidence, 1.0)
		})
	}
}

// TestFDBARPScoreCalculation 测试 FDB/ARP 评分计算
func TestFDBARPScoreCalculation(t *testing.T) {
	cfg := DefaultTopologyBuildConfig()
	builder := &TopologyBuilder{config: cfg}

	tests := []struct {
		name             string
		remoteKind       string
		macCount         int
		hasLogicalIf     bool
		hasRemoteIP      bool
		vlans            map[int]bool
		expectedMinScore float64
	}{
		{
			name:             "设备类型高分",
			remoteKind:       "device",
			macCount:         3,
			hasLogicalIf:     true,
			hasRemoteIP:      true,
			vlans:            map[int]bool{1: true},
			expectedMinScore: 50.0,
		},
		{
			name:             "服务器类型中等分",
			remoteKind:       "server",
			macCount:         1,
			hasLogicalIf:     false,
			hasRemoteIP:      true,
			vlans:            map[int]bool{1: true},
			expectedMinScore: 35.0,
		},
		{
			name:             "终端类型低分",
			remoteKind:       "terminal",
			macCount:         1,
			hasLogicalIf:     false,
			hasRemoteIP:      false,
			vlans:            map[int]bool{1: true, 2: true}, // 多 VLAN
			expectedMinScore: 20.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := builder.scoreFDBARPCandidate(tt.remoteKind, tt.macCount, tt.hasLogicalIf, tt.hasRemoteIP, tt.vlans, nil)
			assert.GreaterOrEqual(t, score.TotalScore, tt.expectedMinScore, "评分应大于等于预期最小值")
			assert.GreaterOrEqual(t, score.Confidence, 0.35)
			assert.LessOrEqual(t, score.Confidence, 0.95)
		})
	}
}

// TestResolveCandidatesGlobal 测试全局冲突消解
func TestResolveCandidatesGlobal(t *testing.T) {
	cfg := DefaultTopologyBuildConfig()
	cfg.ConflictWindow = 10.0
	builder := &TopologyBuilder{config: cfg}

	// 创建测试候选 - 注意：分差小于冲突窗口会触发冲突
	candidates := []*TopologyEdgeCandidate{
		{
			CandidateID: "c1",
			ADeviceID:   "192.168.1.1",
			AIf:         "Gi0/0/1",
			BDeviceID:   "192.168.1.2",
			BIf:         "Gi0/0/1",
			TotalScore:  90.0,
			Status:      "pending",
		},
		{
			CandidateID: "c2",
			ADeviceID:   "192.168.1.1",
			AIf:         "Gi0/0/1",
			BDeviceID:   "192.168.1.3", // 同一端口的另一个候选
			BIf:         "Gi0/0/1",
			TotalScore:  85.0, // 分差 5，小于冲突窗口 10，会触发冲突
			Status:      "pending",
		},
		{
			CandidateID: "c3",
			ADeviceID:   "192.168.1.2",
			AIf:         "Gi0/0/2",
			BDeviceID:   "192.168.1.3",
			BIf:         "Gi0/0/2",
			TotalScore:  80.0,
			Status:      "pending",
		},
	}

	resolved, traces := builder.resolveCandidatesGlobal(candidates)

	// 验证结果
	assert.NotEmpty(t, resolved, "应该有保留的候选")
	assert.NotEmpty(t, traces, "应该有决策轨迹")

	// 由于 c1 和 c2 分差小于冲突窗口，会标记为 conflict
	// c3 没有竞争，应该被保留
	foundC3 := false
	for _, c := range resolved {
		if c.CandidateID == "c3" {
			foundC3 = true
			assert.Equal(t, "retained", c.Status, "无竞争候选应被保留")
		}
	}
	assert.True(t, foundC3, "无竞争候选 c3 应该被保留")
}

// TestDecisionTrace 测试决策轨迹生成
func TestDecisionTrace(t *testing.T) {
	trace := TopologyDecisionTrace{
		TaskRunID:            "test-run-001",
		TraceID:              "trace-001",
		DecisionType:         "conflict_resolution",
		DecisionGroup:        "192.168.1.1|Gi0/0/1",
		DecisionResult:       "retained",
		DecisionReason:       "top candidate retained",
		DecisionBasis:        "top_score=90.00, second_score=85.00, gap=5.00",
		RetainedCandidateIDs: []string{"c1"},
		RejectedCandidateIDs: []string{"c2"},
	}

	// 序列化候选信息
	candidates := []DecisionCandidate{
		{CandidateID: "c1", TotalScore: 90.0, Source: "lldp"},
		{CandidateID: "c2", TotalScore: 85.0, Source: "lldp"},
	}
	bytes, err := json.Marshal(candidates)
	require.NoError(t, err)
	trace.Candidates = string(bytes)

	// 验证序列化
	assert.NotEmpty(t, trace.Candidates)
	assert.Contains(t, trace.Candidates, "c1")
	assert.Contains(t, trace.Candidates, "c2")
}

// TestTopologyEdgeCandidate 测试候选边模型
func TestTopologyEdgeCandidate(t *testing.T) {
	candidate := TopologyEdgeCandidate{
		TaskRunID:   "test-run-001",
		CandidateID: "candidate-001",
		ADeviceID:   "192.168.1.1",
		AIf:         "GigabitEthernet0/0/1",
		LogicalAIf:  "Eth-Trunk1",
		BDeviceID:   "192.168.1.2",
		BIf:         "GigabitEthernet0/0/1",
		LogicalBIf:  "Eth-Trunk1",
		EdgeType:    "physical",
		Source:      "lldp",
		Status:      "retained",
		TotalScore:  95.0,
		Features:    []string{"lldp_single_side", "lldp_bidirectional", "aggregate_mapped"},
		EvidenceRefs: []EdgeEvidence{
			{
				Type:     "lldp",
				Source:   "lldp",
				DeviceID: "192.168.1.1",
				LocalIf:  "GigabitEthernet0/0/1",
			},
		},
	}

	// 验证字段
	assert.Equal(t, "test-run-001", candidate.TaskRunID)
	assert.Equal(t, "candidate-001", candidate.CandidateID)
	assert.Equal(t, "lldp", candidate.Source)
	assert.Equal(t, "retained", candidate.Status)
	assert.Len(t, candidate.Features, 3)
	assert.Len(t, candidate.EvidenceRefs, 1)
}

// TestTopologyFactSnapshot 测试事实快照模型
func TestTopologyFactSnapshot(t *testing.T) {
	snapshot := TopologyFactSnapshot{
		TaskRunID:   "test-run-001",
		DeviceCount: 10,
		LLDPCount:   25,
		FDBCount:    150,
		ARPCount:    80,
		AggCount:    5,
		IfCount:     100,
		FactHash:    "abc123def456",
	}

	assert.Equal(t, "test-run-001", snapshot.TaskRunID)
	assert.Equal(t, 10, snapshot.DeviceCount)
	assert.Equal(t, 25, snapshot.LLDPCount)
	assert.NotEmpty(t, snapshot.FactHash)
}

// TestMaterializeEdges 测试边生成
func TestMaterializeEdges(t *testing.T) {
	cfg := DefaultTopologyBuildConfig()
	builder := &TopologyBuilder{config: cfg}

	candidates := []*TopologyEdgeCandidate{
		{
			CandidateID:    "c1",
			ADeviceID:      "192.168.1.1",
			AIf:            "Gi0/0/1",
			BDeviceID:      "192.168.1.2",
			BIf:            "Gi0/0/1",
			TotalScore:     95.0,
			Status:         "retained",
			DecisionReason: "highest score",
			Features:       []string{"lldp_bidirectional"},
			EvidenceRefs:   []EdgeEvidence{{Type: "lldp"}},
		},
		{
			CandidateID: "c2",
			ADeviceID:   "192.168.1.1",
			AIf:         "Gi0/0/2",
			BDeviceID:   "server:aa:bb:cc:dd:ee:ff",
			BIf:         "access",
			TotalScore:  50.0,
			Status:      "rejected", // 被淘汰的候选不应生成边
			Features:    []string{"fdb_arp_inference"},
		},
		{
			CandidateID:    "c3",
			ADeviceID:      "192.168.1.3",
			AIf:            "Gi0/0/1",
			BDeviceID:      "192.168.1.4",
			BIf:            "Gi0/0/1",
			TotalScore:     60.0,
			Status:         "conflict",
			DecisionReason: "conflict: multiple close candidates",
			Features:       []string{"lldp_single_side"},
			EvidenceRefs:   []EdgeEvidence{{Type: "lldp"}},
		},
	}

	edges := builder.materializeEdges(candidates, "test-run-001")

	// 只有 retained 和 conflict 状态的候选才生成边
	assert.Len(t, edges, 2, "应该生成 2 条边（retained 和 conflict）")

	// 验证边状态
	for _, edge := range edges {
		if edge.CandidateID == "c1" {
			// 95.0 分对应置信度 0.95，等于 Confirmed 阈值，所以是 semi_confirmed
			assert.Equal(t, "semi_confirmed", edge.Status, "高分候选应确认为 semi_confirmed")
		}
		if edge.CandidateID == "c3" {
			assert.Equal(t, "conflict", edge.Status, "冲突候选状态应为 conflict")
		}
	}
}

// TestEnrichCandidatesWithInterfaceFacts 测试接口事实丰富
func TestEnrichCandidatesWithInterfaceFacts(t *testing.T) {
	cfg := DefaultTopologyBuildConfig()
	builder := &TopologyBuilder{config: cfg}

	n := &NormalizedFacts{
		Interfaces: make(map[string]*InterfaceInfo),
	}

	// 添加接口信息
	n.Interfaces["192.168.1.1|Gi0/0/1"] = &InterfaceInfo{
		DeviceIP:      "192.168.1.1",
		InterfaceName: "Gi0/0/1",
		Status:        "up",
		Speed:         "1000",
		Duplex:        "full",
	}

	candidates := []*TopologyEdgeCandidate{
		{
			CandidateID:    "c1",
			ADeviceID:      "192.168.1.1",
			AIf:            "Gi0/0/1",
			TotalScore:     75.0,
			ScoreBreakdown: `{"version":"1.0","totalScore":75.0}`,
			Status:         "pending",
		},
	}

	builder.enrichCandidatesWithInterfaceFacts(candidates, n)

	// 验证评分增加
	var score ScoreBreakdown
	err := json.Unmarshal([]byte(candidates[0].ScoreBreakdown), &score)
	require.NoError(t, err)

	assert.True(t, score.InterfaceScore.LocalIfUp, "接口 up 状态应被识别")
	// 验证总分增加了接口加分（IfUpBonus = 3.0）
	// 初始 75.0 + IfUpBonus 3.0 = 78.0
	assert.Equal(t, 78.0, candidates[0].TotalScore, "总分应增加 IfUpBonus")
}
