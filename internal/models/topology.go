// Package models 包含所有数据库模型定义
package models

import "time"

// ============================================================================
// 拓扑发现相关模型
// ============================================================================

// TopologyEdge 拓扑边（链路）
type TopologyEdge struct {
	ID               string         `json:"id" gorm:"primaryKey"`
	TaskID           string         `json:"taskId" gorm:"index;not null"`
	ADeviceID        string         `json:"aDeviceId" gorm:"index"`
	AIf              string         `json:"aIf"`
	BDeviceID        string         `json:"bDeviceId" gorm:"index"`
	BIf              string         `json:"bIf"`
	LogicalAIf       string         `json:"logicalAIf"`
	LogicalBIf       string         `json:"logicalBIf"`
	EdgeType         string         `json:"edgeType"` // physical / logical_aggregate
	Status           string         `json:"status"`   // confirmed / semi_confirmed / inferred / conflict
	Confidence       float64        `json:"confidence"`
	DiscoveryMethods string         `json:"discoveryMethods" gorm:"serializer:json"`
	Evidence         []EdgeEvidence `json:"evidence" gorm:"serializer:json"`
	CreatedAt        time.Time      `json:"createdAt"`
	UpdatedAt        time.Time      `json:"updatedAt"`
}

// TableName 指定表名
func (TopologyEdge) TableName() string {
	return "topology_edges"
}

// EdgeEvidence 边证据
type EdgeEvidence struct {
	Source     string `json:"source"`     // 来源：lldp / fdb / arp
	LocalIf    string `json:"localIf"`    // 本地接口
	RemoteName string `json:"remoteName"` // 远端名称
	RemoteIf   string `json:"remoteIf"`   // 远端接口
	RemoteMAC  string `json:"remoteMac"`  // 远端MAC
	RemoteIP   string `json:"remoteIp"`   // 远端IP
	Timestamp  string `json:"timestamp"`  // 时间戳
}

// TopologyInterface 拓扑接口信息
type TopologyInterface struct {
	ID            uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	TaskID        string    `json:"taskId" gorm:"index;not null"`
	DeviceIP      string    `json:"deviceIp" gorm:"index;not null"`
	InterfaceName string    `json:"interfaceName" gorm:"index"`
	Status        string    `json:"status"` // up / down
	Speed         string    `json:"speed"`
	Duplex        string    `json:"duplex"`
	Description   string    `json:"description"`
	MACAddress    string    `json:"macAddress"`
	IPAddress     string    `json:"ipAddress"`
	IsAggregate   bool      `json:"isAggregate"`
	AggregateID   string    `json:"aggregateId"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// TableName 指定表名
func (TopologyInterface) TableName() string {
	return "topology_interfaces"
}

// TopologyLLDPNeighbor LLDP邻居信息
type TopologyLLDPNeighbor struct {
	ID              uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	TaskID          string    `json:"taskId" gorm:"index;not null"`
	DeviceIP        string    `json:"deviceIp" gorm:"index;not null"`
	LocalInterface  string    `json:"localInterface" gorm:"index"`
	NeighborName    string    `json:"neighborName" gorm:"index"`
	NeighborChassis string    `json:"neighborChassis"`
	NeighborPort    string    `json:"neighborPort"`
	NeighborIP      string    `json:"neighborIp"`
	NeighborDesc    string    `json:"neighborDesc"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

// TableName 指定表名
func (TopologyLLDPNeighbor) TableName() string {
	return "topology_lldp_neighbors"
}

// TopologyFDBEntry MAC地址表项
type TopologyFDBEntry struct {
	ID         uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	TaskID     string    `json:"taskId" gorm:"index;not null"`
	DeviceIP   string    `json:"deviceIp" gorm:"index;not null"`
	MACAddress string    `json:"macAddress" gorm:"index"`
	VLAN       int       `json:"vlan"`
	Interface  string    `json:"interface" gorm:"index"`
	Type       string    `json:"type"` // dynamic / static
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// TableName 指定表名
func (TopologyFDBEntry) TableName() string {
	return "topology_fdb_entries"
}

// TopologyARPEntry ARP表项
type TopologyARPEntry struct {
	ID         uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	TaskID     string    `json:"taskId" gorm:"index;not null"`
	DeviceIP   string    `json:"deviceIp" gorm:"index;not null"`
	IPAddress  string    `json:"ipAddress" gorm:"index"`
	MACAddress string    `json:"macAddress" gorm:"index"`
	Interface  string    `json:"interface" gorm:"index"`
	Type       string    `json:"type"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// TableName 指定表名
func (TopologyARPEntry) TableName() string {
	return "topology_arp_entries"
}

// TopologyAggregateGroup 聚合组
type TopologyAggregateGroup struct {
	ID            uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	TaskID        string    `json:"taskId" gorm:"index;not null"`
	DeviceIP      string    `json:"deviceIp" gorm:"index;not null"`
	AggregateName string    `json:"aggregateName" gorm:"index"`
	Mode          string    `json:"mode"` // lacp / static
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// TableName 指定表名
func (TopologyAggregateGroup) TableName() string {
	return "topology_aggregate_groups"
}

// TopologyAggregateMember 聚合成员
type TopologyAggregateMember struct {
	ID            uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	TaskID        string    `json:"taskId" gorm:"index;not null"`
	DeviceIP      string    `json:"deviceIp" gorm:"index;not null"`
	AggregateName string    `json:"aggregateName" gorm:"index"`
	MemberPort    string    `json:"memberPort" gorm:"index"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// TableName 指定表名
func (TopologyAggregateMember) TableName() string {
	return "topology_aggregate_members"
}

// ============================================================================
// 拓扑视图模型
// ============================================================================

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
