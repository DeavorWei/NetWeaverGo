package taskexec

import (
	"encoding/json"
	"strings"
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

	// 验证用户可调参数
	assert.Equal(t, 5, cfg.MaxInferenceCandidates)
	assert.Equal(t, 10.0, cfg.ConflictWindow)
	assert.True(t, cfg.SaveCandidates)
	assert.True(t, cfg.SaveDecisionTraces)
}

// TestScoringWeightConstants 测试评分权重常量值
func TestScoringWeightConstants(t *testing.T) {
	// LLDP 权重
	assert.Equal(t, 75.0, wLLDPBaseSingleSide)
	assert.Equal(t, 25.0, wLLDPBidirectionalBonus)
	assert.Equal(t, 5.0, wLLDPChassisMatch)
	assert.Equal(t, 3.0, wLLDPNameMatch)
	assert.Equal(t, 5.0, wLLDPIPMatch)
	assert.Equal(t, 2.0, wLLDPRemoteIfPresent)

	// FDB/ARP 权重
	assert.Equal(t, 20.0, wFDBBaseScore)
	assert.Equal(t, 2.0, wFDBMACCountFactor)
	assert.Equal(t, 30.0, wFDBDeviceBonus)
	assert.Equal(t, 10.0, wFDBEndpointBonus)
	assert.Equal(t, 3.0, wFDBUnknownBonus)

	// 置信度阈值
	assert.Equal(t, 0.95, confidenceConfirmed)
	assert.Equal(t, 0.75, confidenceSemiConfirmed)
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
			name:             "端点类型中等分",
			remoteKind:       "endpoint",
			macCount:         1,
			hasLogicalIf:     false,
			hasRemoteIP:      true,
			vlans:            map[int]bool{1: true},
			expectedMinScore: 30.0,
		},
		{
			name:             "未知类型低分",
			remoteKind:       "unknown",
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
	}

	candidates := []*TopologyEdgeCandidate{
		{
			CandidateID:    "c1",
			ADeviceID:      "192.168.1.1",
			AIf:            "Gi0/0/1",
			TotalScore:     75.0,
			ScoreBreakdown: `{"version":"1.0","totalScore":75.0}`,
			Status:         "pending",
			score:          ScoreBreakdown{Version: "1.0", TotalScore: 75.0},
		},
	}

	builder.enrichCandidatesWithInterfaceFacts(candidates, n)

	// 验证接口状态被识别
	assert.True(t, candidates[0].score.InterfaceScore.LocalIfUp, "接口 up 状态应被识别")
	// 验证总分增加了接口加分（wIfUpBonus = 3.0）
	// 初始 75.0 + IfUpBonus 3.0 = 78.0
	assert.Equal(t, 78.0, candidates[0].TotalScore, "总分应增加 IfUpBonus")
}

// =============================================================================
// Phase 2.2: 拓扑重复问题修复 - 单元测试覆盖
// =============================================================================

// TestResolveLLDPPeer 测试 LLDP 对端设备解析的多维穿透匹配
func TestResolveLLDPPeer(t *testing.T) {
	cfg := DefaultTopologyBuildConfig()
	builder := &TopologyBuilder{config: cfg}

	// 创建标准化事实数据
	n := &NormalizedFacts{
		Devices:           make(map[string]*DeviceInfo),
		DeviceByName:      make(map[string]string),
		DeviceByMgmtIP:    make(map[string]string),
		DeviceByChassisID: make(map[string]string),
	}

	// 设置设备信息（模拟 MgmtIP 为空的情况）
	n.Devices["192.168.1.1"] = &DeviceInfo{
		DeviceIP:       "192.168.1.1",
		NormalizedName: "switch-a",
		ChassisID:      "00:11:22:33:44:55",
		MgmtIP:         "", // MgmtIP 为空
	}
	n.Devices["192.168.1.2"] = &DeviceInfo{
		DeviceIP:       "192.168.1.2",
		NormalizedName: "switch-b",
		ChassisID:      "00:11:22:33:44:66",
		MgmtIP:         "10.0.0.2", // MgmtIP 与 DeviceIP 不同
	}

	// 建立索引（与 normalizeFacts 逻辑一致）
	// DeviceIP 应入库 DeviceByMgmtIP（阶段 1.1 修复）
	n.DeviceByMgmtIP["192.168.1.1"] = "192.168.1.1"
	n.DeviceByMgmtIP["192.168.1.2"] = "192.168.1.2"
	n.DeviceByMgmtIP["10.0.0.2"] = "192.168.1.2"

	n.DeviceByName["switch-a"] = "192.168.1.1"
	n.DeviceByName["switch-b"] = "192.168.1.2"

	n.DeviceByChassisID["00:11:22:33:44:55"] = "192.168.1.1"
	n.DeviceByChassisID["00:11:22:33:44:66"] = "192.168.1.2"

	// TC-01: MgmtIP 为空，NeighborIP == DeviceIP - 应通过 DeviceIP 索引匹配
	t.Run("TC-01: MgmtIP为空_NeighborIP匹配DeviceIP", func(t *testing.T) {
		lldp := NormalizedLLDPNeighbor{
			DeviceIP:     "192.168.1.2",
			LocalIf:      "Gi0/0/1",
			NeighborIP:   "192.168.1.1", // 指向 DeviceIP（因为 MgmtIP 为空）
			NeighborName: "Switch-A",
		}
		deviceIP, source := builder.resolveLLDPPeer(lldp, n)
		assert.Equal(t, "192.168.1.1", deviceIP, "应通过 DeviceIP 索引匹配到设备")
		assert.Equal(t, "neighbor_ip", source)
	})

	// TC-02: MgmtIP != DeviceIP，NeighborIP == MgmtIP - 应通过 MgmtIP 索引匹配
	t.Run("TC-02: NeighborIP匹配MgmtIP", func(t *testing.T) {
		lldp := NormalizedLLDPNeighbor{
			DeviceIP:     "192.168.1.1",
			LocalIf:      "Gi0/0/1",
			NeighborIP:   "10.0.0.2", // 指向 MgmtIP
			NeighborName: "Switch-B",
		}
		deviceIP, source := builder.resolveLLDPPeer(lldp, n)
		assert.Equal(t, "192.168.1.2", deviceIP, "应通过 MgmtIP 索引匹配到设备")
		assert.Equal(t, "neighbor_ip", source)
	})

	// TC-03: MgmtIP != DeviceIP，NeighborIP == DeviceIP - 应通过 DeviceIP 索引匹配
	t.Run("TC-03: NeighborIP匹配DeviceIP而非MgmtIP", func(t *testing.T) {
		lldp := NormalizedLLDPNeighbor{
			DeviceIP:     "192.168.1.1",
			LocalIf:      "Gi0/0/1",
			NeighborIP:   "192.168.1.2", // 指向 DeviceIP
			NeighborName: "Switch-B",
		}
		deviceIP, source := builder.resolveLLDPPeer(lldp, n)
		assert.Equal(t, "192.168.1.2", deviceIP, "应通过 DeviceIP 索引匹配到设备")
		assert.Equal(t, "neighbor_ip", source)
	})

	// TC-04: NeighborIP 匹配失败，ChassisID 匹配成功 - 穿透匹配
	t.Run("TC-04: ChassisID穿透匹配", func(t *testing.T) {
		lldp := NormalizedLLDPNeighbor{
			DeviceIP:        "192.168.1.2",
			LocalIf:         "Gi0/0/1",
			NeighborIP:      "192.168.99.99",     // 不存在的 IP
			NeighborChassis: "00:11:22:33:44:55", // 但 ChassisID 匹配
			NeighborName:    "Switch-A",
		}
		deviceIP, source := builder.resolveLLDPPeer(lldp, n)
		assert.Equal(t, "192.168.1.1", deviceIP, "应通过 ChassisID 穿透匹配到设备")
		assert.Equal(t, "chassis_id", source)
	})

	// TC-05: NeighborIP 匹配失败，NeighborName 匹配成功 - 穿透匹配
	t.Run("TC-05: NeighborName穿透匹配", func(t *testing.T) {
		lldp := NormalizedLLDPNeighbor{
			DeviceIP:     "192.168.1.2",
			LocalIf:      "Gi0/0/1",
			NeighborIP:   "192.168.99.99", // 不存在的 IP
			NeighborName: "Switch-A",      // 但名称匹配
		}
		deviceIP, source := builder.resolveLLDPPeer(lldp, n)
		assert.Equal(t, "192.168.1.1", deviceIP, "应通过 NeighborName 穿透匹配到设备")
		assert.Equal(t, "neighbor_name", source)
	})

	// TC-06: 所有维度均失败 - 应返回 unmanaged 前缀
	t.Run("TC-06: 所有维度失败返回unmanaged", func(t *testing.T) {
		lldp := NormalizedLLDPNeighbor{
			DeviceIP:     "192.168.1.1",
			LocalIf:      "Gi0/0/1",
			NeighborIP:   "192.168.99.99",  // 不存在的 IP
			NeighborName: "Unknown-Switch", // 不存在的名称
		}
		deviceIP, source := builder.resolveLLDPPeer(lldp, n)
		assert.True(t, strings.HasPrefix(deviceIP, "unmanaged:"), "应返回 unmanaged: 前缀")
		assert.Equal(t, "unknown_peer", source)
	})

	// TC-07: ChassisID 优先级高于 Name（MAC 比名称更可靠）
	t.Run("TC-07: ChassisID优先级高于Name", func(t *testing.T) {
		lldp := NormalizedLLDPNeighbor{
			DeviceIP:        "192.168.1.2",
			LocalIf:         "Gi0/0/1",
			NeighborIP:      "", // 无 IP
			NeighborChassis: "00:11:22:33:44:55",
			NeighborName:    "Wrong-Name", // 错误的名称（但 ChassisID 正确）
		}
		deviceIP, source := builder.resolveLLDPPeer(lldp, n)
		assert.Equal(t, "192.168.1.1", deviceIP)
		assert.Equal(t, "chassis_id", source, "ChassisID 应优先于 Name 匹配")
	})
}

// TestNormalizeFacts_DeviceIPIndexing 测试 DeviceIP 入库 DeviceByMgmtIP 索引
func TestNormalizeFacts_DeviceIPIndexing(t *testing.T) {
	builder := &TopologyBuilder{config: DefaultTopologyBuildConfig()}

	input := &TopologyBuildInput{
		Devices: []TaskRunDevice{
			{
				DeviceIP:       "192.168.1.1",
				NormalizedName: "switch-a",
				MgmtIP:         "", // MgmtIP 为空
			},
			{
				DeviceIP:       "192.168.1.2",
				NormalizedName: "switch-b",
				MgmtIP:         "10.0.0.2", // MgmtIP 与 DeviceIP 不同
			},
		},
	}

	n := builder.normalizeFacts(input)

	// 验证 DeviceIP 已入库 DeviceByMgmtIP（阶段 1.1 修复）
	assert.Equal(t, "192.168.1.1", n.DeviceByMgmtIP["192.168.1.1"], "DeviceIP 应入库 DeviceByMgmtIP")
	assert.Equal(t, "192.168.1.2", n.DeviceByMgmtIP["192.168.1.2"], "DeviceIP 应入库 DeviceByMgmtIP")

	// 验证 MgmtIP 也已入库
	assert.Equal(t, "192.168.1.2", n.DeviceByMgmtIP["10.0.0.2"], "MgmtIP 应入库 DeviceByMgmtIP")

	// 验证 Devices 映射正确
	assert.Equal(t, "192.168.1.1", n.Devices["192.168.1.1"].DeviceIP)
	assert.Equal(t, "192.168.1.2", n.Devices["192.168.1.2"].DeviceIP)
}

// TestTopologyDuplicationFix_Integration 拓扑重复问题修复集成测试
// 模拟 5 台设备（MgmtIP 全部置空）的场景，验证不会产生重复节点
func TestTopologyDuplicationFix_Integration(t *testing.T) {
	builder := &TopologyBuilder{config: DefaultTopologyBuildConfig()}

	// 模拟 5 台设备，所有 MgmtIP 为空
	input := &TopologyBuildInput{
		Devices: []TaskRunDevice{
			{DeviceIP: "192.168.1.1", NormalizedName: "switch-1", ChassisID: "00:11:22:33:44:01"},
			{DeviceIP: "192.168.1.2", NormalizedName: "switch-2", ChassisID: "00:11:22:33:44:02"},
			{DeviceIP: "192.168.1.3", NormalizedName: "switch-3", ChassisID: "00:11:22:33:44:03"},
			{DeviceIP: "192.168.1.4", NormalizedName: "switch-4", ChassisID: "00:11:22:33:44:04"},
			{DeviceIP: "192.168.1.5", NormalizedName: "switch-5", ChassisID: "00:11:22:33:44:05"},
		},
		LLDPNeighbors: []TaskParsedLLDPNeighbor{
			// switch-1 看到 switch-2
			{DeviceIP: "192.168.1.1", LocalInterface: "Gi0/0/1", NeighborName: "switch-2", NeighborChassis: "00:11:22:33:44:02", NeighborPort: "Gi0/0/1"},
			// switch-2 看到 switch-1 和 switch-3
			{DeviceIP: "192.168.1.2", LocalInterface: "Gi0/0/1", NeighborName: "switch-1", NeighborChassis: "00:11:22:33:44:01", NeighborPort: "Gi0/0/1"},
			{DeviceIP: "192.168.1.2", LocalInterface: "Gi0/0/2", NeighborName: "switch-3", NeighborChassis: "00:11:22:33:44:03", NeighborPort: "Gi0/0/1"},
			// switch-3 看到 switch-2 和 switch-4
			{DeviceIP: "192.168.1.3", LocalInterface: "Gi0/0/1", NeighborName: "switch-2", NeighborChassis: "00:11:22:33:44:02", NeighborPort: "Gi0/0/2"},
			{DeviceIP: "192.168.1.3", LocalInterface: "Gi0/0/2", NeighborName: "switch-4", NeighborChassis: "00:11:22:33:44:04", NeighborPort: "Gi0/0/1"},
			// switch-4 看到 switch-3 和 switch-5
			{DeviceIP: "192.168.1.4", LocalInterface: "Gi0/0/1", NeighborName: "switch-3", NeighborChassis: "00:11:22:33:44:03", NeighborPort: "Gi0/0/2"},
			{DeviceIP: "192.168.1.4", LocalInterface: "Gi0/0/2", NeighborName: "switch-5", NeighborChassis: "00:11:22:33:44:05", NeighborPort: "Gi0/0/1"},
			// switch-5 看到 switch-4
			{DeviceIP: "192.168.1.5", LocalInterface: "Gi0/0/1", NeighborName: "switch-4", NeighborChassis: "00:11:22:33:44:04", NeighborPort: "Gi0/0/2"},
		},
	}

	n := builder.normalizeFacts(input)

	// 验证所有 DeviceIP 都已入库 DeviceByMgmtIP
	for _, d := range input.Devices {
		assert.Equal(t, d.DeviceIP, n.DeviceByMgmtIP[d.DeviceIP],
			"设备 %s 的 DeviceIP 应入库 DeviceByMgmtIP", d.DeviceIP)
	}

	// 验证 LLDP 对端解析不会产生重复
	resolvedDevices := make(map[string]bool)
	for _, lldp := range n.LLDPNeighbors {
		deviceIP, source := builder.resolveLLDPPeer(lldp, n)
		resolvedDevices[deviceIP] = true

		// 验证解析结果不是裸 IP（应该是设备 IP 或 unmanaged: 前缀）
		assert.True(t,
			strings.HasPrefix(deviceIP, "unmanaged:") || n.Devices[deviceIP] != nil,
			"LLDP 对端应解析为已知设备或标记为 unmanaged，而不是裸 IP")

		// 验证解析来源正确
		assert.Contains(t, []string{"neighbor_ip", "chassis_id", "neighbor_name", "unknown_peer"}, source)
	}

	// 验证所有设备都被正确解析（没有产生额外节点）
	// 5 台设备应该只产生 5 个节点标识
	expectedDevices := map[string]bool{
		"192.168.1.1": true,
		"192.168.1.2": true,
		"192.168.1.3": true,
		"192.168.1.4": true,
		"192.168.1.5": true,
	}

	for deviceIP := range resolvedDevices {
		if !strings.HasPrefix(deviceIP, "unmanaged:") {
			assert.True(t, expectedDevices[deviceIP],
				"解析结果 %s 应该是 5 台设备之一，不应产生重复节点", deviceIP)
		}
	}
}
