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

// GraphNode 图节点
type GraphNode struct {
	ID           string `json:"id"`
	Label        string `json:"label"`
	IP           string `json:"ip"`
	Vendor       string `json:"vendor"`
	Model        string `json:"model"`
	Role         string `json:"role"`
	Site         string `json:"site"`
	SerialNumber string `json:"serialNumber"`
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
}
