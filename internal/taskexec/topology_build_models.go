package taskexec

import "time"

// =============================================================================
// Phase A: 数据模型升级
// 本文件包含拓扑构建所需的新增数据模型，用于支持可解释、可追溯的拓扑还原
// =============================================================================

// TopologyFactSnapshot 拓扑事实快照
// 固化每次构图输入摘要，支持复现和回溯
type TopologyFactSnapshot struct {
	ID          uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	TaskRunID   string    `gorm:"uniqueIndex;not null" json:"taskRunId"`
	SnapshotAt  time.Time `json:"snapshotAt"`
	DeviceCount int       `json:"deviceCount"` // 设备数量
	LLDPCount   int       `json:"lldpCount"`   // LLDP 事实数量
	FDBCount    int       `json:"fdbCount"`    // FDB 事实数量
	ARPCount    int       `json:"arpCount"`    // ARP 事实数量
	AggCount    int       `json:"aggCount"`    // 聚合组成员数量
	IfCount     int       `json:"ifCount"`     // 接口事实数量
	// 事实摘要哈希，用于快速比对两次构建输入是否相同
	FactHash string `json:"factHash"`
	// 采集计划元数据快照（JSON 序列化）
	CollectionPlanSummary string `json:"collectionPlanSummary"`
	// 构建配置快照（JSON 序列化）
	BuildConfigSnapshot string    `json:"buildConfigSnapshot"`
	CreatedAt           time.Time `json:"createdAt"`
	UpdatedAt           time.Time `json:"updatedAt"`
}

func (TopologyFactSnapshot) TableName() string {
	return "topology_fact_snapshots"
}

// TopologyEdgeCandidate 拓扑边候选
// 保存候选边、来源、特征、评分明细
type TopologyEdgeCandidate struct {
	ID          uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	TaskRunID   string `gorm:"index;not null" json:"taskRunId"`
	CandidateID string `gorm:"index;not null" json:"candidateId"` // 候选唯一标识
	ADeviceID   string `gorm:"index" json:"aDeviceId"`
	AIf         string `json:"aIf"`
	LogicalAIf  string `json:"logicalAIf"`
	BDeviceID   string `gorm:"index" json:"bDeviceId"`
	BIf         string `json:"bIf"`
	LogicalBIf  string `json:"logicalBIf"`
	EdgeType    string `json:"edgeType"` // physical, logical
	Source      string `json:"source"`   // lldp, fdb_arp, manual
	Status      string `json:"status"`   // pending, retained, rejected, merged
	// 评分明细（JSON 序列化的 ScoreBreakdown）
	ScoreBreakdown string  `json:"scoreBreakdown"`
	TotalScore     float64 `json:"totalScore"`
	// 特征标签
	Features []string `gorm:"serializer:json" json:"features"`
	// 证据引用列表（指向原始事实的引用）
	EvidenceRefs []EdgeEvidence `gorm:"serializer:json" json:"evidenceRefs"`
	// 决策原因（为何被保留或淘汰）
	DecisionReason string `json:"decisionReason"`
	// 最终边 ID（如果被保留，指向最终的 TaskTopologyEdge）
	FinalEdgeID string `json:"finalEdgeId"`
	// B端设备的MAC地址列表（用于IP标识时保留MAC信息）
	BDeviceMACs []string `gorm:"serializer:json" json:"bDeviceMacs"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func (TopologyEdgeCandidate) TableName() string {
	return "topology_edge_candidates"
}

// ScoreBreakdown 评分明细结构
// 用于记录候选边的各项评分细节
type ScoreBreakdown struct {
	// LLDP 相关评分
	LLDPScore LLDPScoreDetail `json:"lldpScore"`

	// FDB/ARP 推断评分
	FDBARPScore FDBARPScoreDetail `json:"fdbArpScore"`

	// 接口事实加分
	InterfaceScore InterfaceScoreDetail `json:"interfaceScore"`

	// 聚合相关评分
	AggregateScore AggregateScoreDetail `json:"aggregateScore"`

	// 总分
	TotalScore float64 `json:"totalScore"`
	// 置信度（0-1）
	Confidence float64 `json:"confidence"`
	// 评分版本（用于后续调参对比）
	Version string `json:"version"`
}

// LLDPScoreDetail LLDP 评分明细
type LLDPScoreDetail struct {
	BaseScore        float64 `json:"baseScore"`        // 基础分（单边 0.75，双向 1.0）
	ChassisMatch     float64 `json:"chassisMatch"`     // chassis_id 一致性加分
	NameMatch        float64 `json:"nameMatch"`        // 名称匹配加分
	IPMatch          float64 `json:"ipMatch"`          // IP 匹配加分
	RemoteIfPresent  float64 `json:"remoteIfPresent"`  // 远端接口存在加分
	Bidirectional    bool    `json:"bidirectional"`    // 是否双向确认
	ResolutionSource string  `json:"resolutionSource"` // 解析来源：neighbor_ip, neighbor_name, unknown
}

// FDBARPScoreDetail FDB/ARP 评分明细
type FDBARPScoreDetail struct {
	BaseScore       float64 `json:"baseScore"`       // 基础分
	MACCount        int     `json:"macCount"`        // MAC 数量
	MACCountScore   float64 `json:"macCountScore"`   // MAC 数量评分
	EndpointKind    string  `json:"endpointKind"`    // device, server, terminal
	EndpointScore   float64 `json:"endpointScore"`   // 端点类型评分
	VLANConsistent  bool    `json:"vlanConsistent"`  // VLAN 一致性
	VLANScore       float64 `json:"vlanScore"`       // VLAN 评分
	HasLogicalIf    bool    `json:"hasLogicalIf"`    // 是否有聚合接口
	LogicalIfScore  float64 `json:"logicalIfScore"`  // 聚合接口加分
	HasRemoteIP     bool    `json:"hasRemoteIP"`     // 是否有远端 IP
	RemoteIPScore   float64 `json:"remoteIPScore"`   // 远端 IP 加分
	InterfaceStatus string  `json:"interfaceStatus"` // 接口状态
	IfStatusScore   float64 `json:"ifStatusScore"`   // 接口状态评分
}

// InterfaceScoreDetail 接口评分明细
type InterfaceScoreDetail struct {
	LocalIfUp       bool    `json:"localIfUp"`       // 本端接口是否 up
	LocalIfUpScore  float64 `json:"localIfUpScore"`  // 本端接口 up 加分
	RemoteIfUp      bool    `json:"remoteIfUp"`      // 远端接口是否 up
	RemoteIfUpScore float64 `json:"remoteIfUpScore"` // 远端接口 up 加分
	SpeedMatch      bool    `json:"speedMatch"`      // 速率是否匹配
	SpeedMatchScore float64 `json:"speedMatchScore"` // 速率匹配加分
	DuplexMatch     bool    `json:"duplexMatch"`     // 双工是否匹配
	DuplexScore     float64 `json:"duplexScore"`     // 双工匹配加分
}

// AggregateScoreDetail 聚合评分明细
type AggregateScoreDetail struct {
	IsAggregateLink    bool    `json:"isAggregateLink"`    // 是否聚合链路
	AggregateMode      string  `json:"aggregateMode"`      // 聚合模式：lacp, static
	AggregateModeScore float64 `json:"aggregateModeScore"` // 聚合模式加分
	MemberCount        int     `json:"memberCount"`        // 成员数量
	MemberCountScore   float64 `json:"memberCountScore"`   // 成员数量加分
}

// TopologyDecisionTrace 拓扑决策轨迹
// 保存冲突分组、淘汰原因、最终决策
type TopologyDecisionTrace struct {
	ID             uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	TaskRunID      string `gorm:"index;not null" json:"taskRunId"`
	TraceID        string `gorm:"index;not null" json:"traceId"` // 决策轨迹唯一标识
	DecisionType   string `json:"decisionType"`                  // conflict_resolution, candidate_selection, edge_merge
	DecisionGroup  string `json:"decisionGroup"`                 // 决策分组标识（如同一端点的多个候选）
	DecisionResult string `json:"decisionResult"`                // retained, rejected, merged, conflict
	// 候选列表（JSON 序列化的 DecisionCandidate）
	Candidates string `json:"candidates"`
	// 被保留的候选 ID
	RetainedCandidateIDs []string `gorm:"serializer:json" json:"retainedCandidateIds"`
	// 被淘汰的候选 ID
	RejectedCandidateIDs []string `gorm:"serializer:json" json:"rejectedCandidateIds"`
	// 决策原因明细
	DecisionReason string `json:"decisionReason"`
	// 决策依据（评分差异、规则触发等）
	DecisionBasis string `json:"decisionBasis"`
	// 最终边 ID
	FinalEdgeID string `json:"finalEdgeId"`
	// 决策耗时（毫秒）
	DecisionMs int       `json:"decisionMs"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

func (TopologyDecisionTrace) TableName() string {
	return "topology_decision_traces"
}

// DecisionCandidate 决策候选结构
// 用于记录参与决策的候选信息
type DecisionCandidate struct {
	CandidateID string   `json:"candidateId"`
	ADeviceID   string   `json:"aDeviceId"`
	AIf         string   `json:"aIf"`
	BDeviceID   string   `json:"bDeviceId"`
	BIf         string   `json:"bIf"`
	TotalScore  float64  `json:"totalScore"`
	Confidence  float64  `json:"confidence"`
	Source      string   `json:"source"`
	Features    []string `json:"features"`
}

// TopologyBuildConfig 拓扑构建配置
// 运行时可调整的参数
type TopologyBuildConfig struct {
	// 最大推断候选数
	MaxInferenceCandidates int `json:"maxInferenceCandidates"`
	// 冲突窗口（分差小于此值视为竞争）
	ConflictWindow float64 `json:"conflictWindow"`
	// LLDP 评分权重
	LLDPWeights struct {
		BaseSingleSide    float64 `json:"baseSingleSide"`    // 单边基础分
		BaseBidirectional float64 `json:"baseBidirectional"` // 双向基础分
		ChassisMatch      float64 `json:"chassisMatch"`      // chassis 匹配加分
		NameMatch         float64 `json:"nameMatch"`         // 名称匹配加分
		IPMatch           float64 `json:"ipMatch"`           // IP 匹配加分
		RemoteIfPresent   float64 `json:"remoteIfPresent"`   // 远端接口存在加分
	} `json:"lldpWeights"`
	// FDB/ARP 评分权重
	FDBARPWeights struct {
		BaseScore      float64 `json:"baseScore"`      // 基础分
		MACCountFactor float64 `json:"macCountFactor"` // MAC 数量因子
		DeviceBonus    float64 `json:"deviceBonus"`    // 设备类型加分
		ServerBonus    float64 `json:"serverBonus"`    // 服务器类型加分
		TerminalBonus  float64 `json:"terminalBonus"`  // 终端类型加分
		VLANBonus      float64 `json:"vlanBonus"`      // VLAN 一致性加分
		LogicalIfBonus float64 `json:"logicalIfBonus"` // 聚合接口加分
		RemoteIPBonus  float64 `json:"remoteIPBonus"`  // 远端 IP 加分
	} `json:"fdbArpWeights"`
	// 接口评分权重
	InterfaceWeights struct {
		IfUpBonus   float64 `json:"ifUpBonus"`   // 接口 up 加分
		SpeedMatch  float64 `json:"speedMatch"`  // 速率匹配加分
		DuplexMatch float64 `json:"duplexMatch"` // 双工匹配加分
	} `json:"interfaceWeights"`
	// 聚合评分权重
	AggregateWeights struct {
		LACPModeBonus   float64 `json:"lacpModeBonus"`   // LACP 模式加分
		StaticModeBonus float64 `json:"staticModeBonus"` // 静态模式加分
		MemberBonus     float64 `json:"memberBonus"`     // 成员加分
	} `json:"aggregateWeights"`
	// 置信度阈值
	ConfidenceThresholds struct {
		Confirmed     float64 `json:"confirmed"`     // 确认阈值
		SemiConfirmed float64 `json:"semiConfirmed"` // 半确认阈值
		Inferred      float64 `json:"inferred"`      // 推断阈值
	} `json:"confidenceThresholds"`
	// 是否启用新构建逻辑
	UseNewBuilder bool `json:"useNewBuilder"`
	// 是否保存候选轨迹
	SaveCandidates bool `json:"saveCandidates"`
	// 是否保存决策轨迹
	SaveDecisionTraces bool `json:"saveDecisionTraces"`
}

// DefaultTopologyBuildConfig 返回默认拓扑构建配置
func DefaultTopologyBuildConfig() TopologyBuildConfig {
	cfg := TopologyBuildConfig{
		MaxInferenceCandidates: 5,
		ConflictWindow:         10.0,
		UseNewBuilder:          false, // 默认使用旧逻辑，通过配置切换
		SaveCandidates:         true,
		SaveDecisionTraces:     true,
	}

	// LLDP 权重
	cfg.LLDPWeights.BaseSingleSide = 75.0
	cfg.LLDPWeights.BaseBidirectional = 100.0
	cfg.LLDPWeights.ChassisMatch = 5.0
	cfg.LLDPWeights.NameMatch = 3.0
	cfg.LLDPWeights.IPMatch = 5.0
	cfg.LLDPWeights.RemoteIfPresent = 2.0

	// FDB/ARP 权重
	cfg.FDBARPWeights.BaseScore = 20.0
	cfg.FDBARPWeights.MACCountFactor = 2.0
	cfg.FDBARPWeights.DeviceBonus = 30.0
	cfg.FDBARPWeights.ServerBonus = 15.0
	cfg.FDBARPWeights.TerminalBonus = 5.0
	cfg.FDBARPWeights.VLANBonus = 3.0
	cfg.FDBARPWeights.LogicalIfBonus = 5.0
	cfg.FDBARPWeights.RemoteIPBonus = 5.0

	// 接口权重
	cfg.InterfaceWeights.IfUpBonus = 3.0
	cfg.InterfaceWeights.SpeedMatch = 2.0
	cfg.InterfaceWeights.DuplexMatch = 1.0

	// 聚合权重
	cfg.AggregateWeights.LACPModeBonus = 5.0
	cfg.AggregateWeights.StaticModeBonus = 3.0
	cfg.AggregateWeights.MemberBonus = 1.0

	// 置信度阈值
	cfg.ConfidenceThresholds.Confirmed = 0.95
	cfg.ConfidenceThresholds.SemiConfirmed = 0.75
	cfg.ConfidenceThresholds.Inferred = 0.35

	return cfg
}

// TopologyBuildInput 拓扑构建输入
// 封装所有构建所需的输入数据
type TopologyBuildInput struct {
	RunID         string
	Devices       []TaskRunDevice
	LLDPNeighbors []TaskParsedLLDPNeighbor
	FDBEntries    []TaskParsedFDBEntry
	ARPEntries    []TaskParsedARPEntry
	AggMembers    []TaskParsedAggregateMember
	AggGroups     []TaskParsedAggregateGroup
	Interfaces    []TaskParsedInterface
	BuildConfig   TopologyBuildConfig
}

// TopologyBuildOutput 拓扑构建输出
// 封装构建结果和轨迹
type TopologyBuildOutput struct {
	Edges          []TaskTopologyEdge
	Candidates     []*TopologyEdgeCandidate
	DecisionTraces []TopologyDecisionTrace
	FactSnapshot   TopologyFactSnapshot
	Statistics     TopologyBuildStatistics
	Errors         []string
}

// TopologyBuildStatistics 拓扑构建统计
type TopologyBuildStatistics struct {
	TotalEdges          int           `json:"totalEdges"`
	ConfirmedEdges      int           `json:"confirmedEdges"`
	SemiConfirmedEdges  int           `json:"semiConfirmedEdges"`
	InferredEdges       int           `json:"inferredEdges"`
	ConflictEdges       int           `json:"conflictEdges"`
	TotalCandidates     int           `json:"totalCandidates"`
	RetainedCandidates  int           `json:"retainedCandidates"`
	RejectedCandidates  int           `json:"rejectedCandidates"`
	DecisionTraceCount  int           `json:"decisionTraceCount"`
	BuildDuration       time.Duration `json:"buildDuration"`
	LLDPResolvedByIP    int           `json:"lldpResolvedByIP"`
	LLDPResolvedByName  int           `json:"lldpResolvedByName"`
	LLDPUnresolvedPeer  int           `json:"lldpUnresolvedPeer"`
	LLDPMissingRemoteIf int           `json:"lldpMissingRemoteIf"`
	FDBInferred         int           `json:"fdbInferred"`
	FDBEnriched         int           `json:"fdbEnriched"`
	FDBSelfHit          int           `json:"fdbSelfHit"`
	FDBAmbiguous        int           `json:"fdbAmbiguous"`
	ClassifiedServer    int           `json:"classifiedServer"`
	ClassifiedTerminal  int           `json:"classifiedTerminal"`
}
