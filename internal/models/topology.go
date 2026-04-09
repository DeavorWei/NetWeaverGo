// Package models 包含所有数据库模型定义
package models

import "time"

// EdgeEvidence 链路证据
// 仅作为统一运行时拓扑详情视图的嵌套 DTO。
type EdgeEvidence struct {
	Type       string `json:"type"`
	DeviceID   string `json:"deviceId"`
	Command    string `json:"command"`
	RawRefID   string `json:"rawRefId"`
	Summary    string `json:"summary"`
	Source     string `json:"source"`
	LocalIf    string `json:"localIf"`
	RemoteName string `json:"remoteName"`
	RemoteIf   string `json:"remoteIf"`
	RemoteMAC  string `json:"remoteMac"`
	RemoteIP   string `json:"remoteIp"`
	Timestamp  string `json:"timestamp,omitempty"`
}

// TopologyBuildResult 拓扑构建结果
type TopologyBuildResult struct {
	TaskID             string        `json:"taskId"`
	TotalEdges         int           `json:"totalEdges"`
	ConfirmedEdges     int           `json:"confirmedEdges"`
	SemiConfirmedEdges int           `json:"semiConfirmedEdges"`
	InferredEdges      int           `json:"inferredEdges"`
	ConflictEdges      int           `json:"conflictEdges"`
	BuildTime          time.Duration `json:"buildTime"`
	Errors             []string      `json:"errors,omitempty"`
}

// TopologyGraphView 拓扑图视图
type TopologyGraphView struct {
	TaskID string      `json:"taskId"`
	Nodes  []GraphNode `json:"nodes"`
	Edges  []GraphEdge `json:"edges"`
}

// NodeType 节点类型（与 internal/taskexec/topology_models.go 保持一致）
type NodeType string

const (
	NodeTypeManaged   NodeType = "managed"   // 已管理设备
	NodeTypeUnmanaged NodeType = "unmanaged" // 未管理设备（LLDP发现但不在采集列表）
	NodeTypeInferred  NodeType = "inferred"  // 推断设备（通过FDB/ARP推断）
	NodeTypeUnknown   NodeType = "unknown"   // 未知类型
)

// GraphNode 图节点
// 阶段3架构演进：支持NodeUUID和NodeType，区分设备类型
type GraphNode struct {
	ID           string   `json:"id"`       // 兼容旧版：通常为DeviceIP
	NodeUUID     string   `json:"nodeUuid"` // 阶段3新增：全局唯一节点标识
	Label        string   `json:"label"`
	IP           string   `json:"ip"`
	AllIPs       []string `json:"allIps"` // 阶段3新增：设备所有IP地址
	Vendor       string   `json:"vendor"`
	Model        string   `json:"model"`
	Role         string   `json:"role"`
	Site         string   `json:"site"`
	SerialNumber string   `json:"serialNumber"`
	NodeType     NodeType `json:"nodeType"`  // 阶段3新增：节点类型
	ChassisID    string   `json:"chassisId"` // 阶段3新增：硬件标识
}

// GraphEdge 图边
type GraphEdge struct {
	ID              string  `json:"id"`
	Source          string  `json:"source"`
	Target          string  `json:"target"`
	SourceIf        string  `json:"sourceIf"`
	TargetIf        string  `json:"targetIf"`
	LogicalSourceIf string  `json:"logicalSourceIf"`
	LogicalTargetIf string  `json:"logicalTargetIf"`
	EdgeType        string  `json:"edgeType"`
	Status          string  `json:"status"`
	Confidence      float64 `json:"confidence"`
}

// TopologyEdgeDetailView 边详情视图
type TopologyEdgeDetailView struct {
	ID               string         `json:"id"`
	ADevice          GraphNode      `json:"aDevice"`
	AIf              string         `json:"aIf"`
	LogicalAIf       string         `json:"logicalAIf"`
	BDevice          GraphNode      `json:"bDevice"`
	BIf              string         `json:"bIf"`
	LogicalBIf       string         `json:"logicalBIf"`
	EdgeType         string         `json:"edgeType"`
	Status           string         `json:"status"`
	Confidence       float64        `json:"confidence"`
	DiscoveryMethods []string       `json:"discoveryMethods"`
	Evidence         []EdgeEvidence `json:"evidence"`
	// Phase A 扩展：决策解释字段
	ConfidenceBreakdown string `json:"confidenceBreakdown"` // JSON 序列化的评分明细
	DecisionReason      string `json:"decisionReason"`      // 决策原因
	CandidateID         string `json:"candidateId"`         // 关联的候选 ID
	TraceID             string `json:"traceId"`             // 关联的决策轨迹 ID
}

// TopologyEdgeExplainView 边解释视图（包含候选和决策轨迹）
type TopologyEdgeExplainView struct {
	Edge          TopologyEdgeDetailView     `json:"edge"`
	Candidates    []TopologyCandidateView    `json:"candidates"`    // 所有候选（包括被淘汰的）
	DecisionTrace *TopologyDecisionTraceView `json:"decisionTrace"` // 决策轨迹
}

// TopologyCandidateView 候选边视图
type TopologyCandidateView struct {
	CandidateID    string   `json:"candidateId"`
	ADeviceID      string   `json:"aDeviceId"`
	AIf            string   `json:"aIf"`
	LogicalAIf     string   `json:"logicalAIf"`
	BDeviceID      string   `json:"bDeviceId"`
	BIf            string   `json:"bIf"`
	LogicalBIf     string   `json:"logicalBIf"`
	Source         string   `json:"source"` // lldp, fdb_arp, manual
	Status         string   `json:"status"` // pending, retained, rejected, merged, conflict
	TotalScore     float64  `json:"totalScore"`
	ScoreBreakdown string   `json:"scoreBreakdown"` // JSON 序列化的评分明细
	Features       []string `json:"features"`
	DecisionReason string   `json:"decisionReason"` // 为何被保留或淘汰
}

// TopologyDecisionTraceView 决策轨迹视图
type TopologyDecisionTraceView struct {
	TraceID              string   `json:"traceId"`
	DecisionType         string   `json:"decisionType"`   // conflict_resolution, candidate_selection, edge_merge
	DecisionGroup        string   `json:"decisionGroup"`  // 决策分组标识
	DecisionResult       string   `json:"decisionResult"` // retained, rejected, merged, conflict
	DecisionReason       string   `json:"decisionReason"` // 决策原因描述
	DecisionBasis        string   `json:"decisionBasis"`  // 决策依据（量化数据）
	RetainedCandidateIDs []string `json:"retainedCandidateIds"`
	RejectedCandidateIDs []string `json:"rejectedCandidateIds"`
	Candidates           string   `json:"candidates"` // JSON 序列化的候选列表快照
}
