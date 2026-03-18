package topology

import "time"

// TopologyEdge 拓扑边（链路）
type TopologyEdge struct {
	ID               string         `json:"id" gorm:"primaryKey"`
	TaskID           string         `json:"taskId" gorm:"index;not null"`
	ADeviceID        string         `json:"aDeviceId" gorm:"index"`                  // A端设备ID
	AIf              string         `json:"aIf"`                                     // A端接口
	BDeviceID        string         `json:"bDeviceId" gorm:"index"`                  // B端设备ID
	BIf              string         `json:"bIf"`                                     // B端接口
	LogicalAIf       string         `json:"logicalAIf"`                              // A端聚合口
	LogicalBIf       string         `json:"logicalBIf"`                              // B端聚合口
	EdgeType         string         `json:"edgeType"`                                // physical / logical_aggregate
	Status           string         `json:"status"`                                  // confirmed / semi_confirmed / inferred / conflict
	Confidence       float64        `json:"confidence"`                              // 置信度 0-1
	DiscoveryMethods string         `json:"discoveryMethods" gorm:"serializer:json"` // 发现方式列表
	Evidence         []EdgeEvidence `json:"evidence" gorm:"serializer:json"`         // 证据链
	CreatedAt        time.Time      `json:"createdAt"`
	UpdatedAt        time.Time      `json:"updatedAt"`
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
	Status        string    `json:"status"`      // up / down
	Speed         string    `json:"speed"`       // 速率
	Duplex        string    `json:"duplex"`      // 双工
	Description   string    `json:"description"` // 描述
	MACAddress    string    `json:"macAddress"`  // MAC地址
	IPAddress     string    `json:"ipAddress"`   // IP地址
	IsAggregate   bool      `json:"isAggregate"` // 是否聚合口
	AggregateID   string    `json:"aggregateId"` // 所属聚合组
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// TopologyLLDPNeighbor LLDP邻居信息
type TopologyLLDPNeighbor struct {
	ID              uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	TaskID          string    `json:"taskId" gorm:"index;not null"`
	DeviceIP        string    `json:"deviceIp" gorm:"index;not null"`
	LocalInterface  string    `json:"localInterface" gorm:"index"`
	NeighborName    string    `json:"neighborName" gorm:"index"`
	NeighborChassis string    `json:"neighborChassis"` // 邻居机箱ID
	NeighborPort    string    `json:"neighborPort"`    // 邻居端口
	NeighborIP      string    `json:"neighborIp"`      // 邻居管理IP
	NeighborDesc    string    `json:"neighborDesc"`    // 邻居描述
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
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
	Source          string  `json:"source"`          // A端设备ID
	Target          string  `json:"target"`          // B端设备ID
	SourceIf        string  `json:"sourceIf"`        // A端接口
	TargetIf        string  `json:"targetIf"`        // B端接口
	LogicalSourceIf string  `json:"logicalSourceIf"` // A端聚合口
	LogicalTargetIf string  `json:"logicalTargetIf"` // B端聚合口
	EdgeType        string  `json:"edgeType"`        // physical / logical_aggregate
	Status          string  `json:"status"`          // confirmed / semi_confirmed / inferred / conflict
	Confidence      float64 `json:"confidence"`      // 置信度
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
