package taskexec

import "time"

// NodeType 节点类型定义
type NodeType string

const (
	NodeTypeManaged   NodeType = "managed"   // 已管理设备（在采集列表中且已成功采集）
	NodeTypeUnmanaged NodeType = "unmanaged" // 未管理设备（LLDP发现但不在采集列表中）
	NodeTypeInferred  NodeType = "inferred"  // 推断设备（通过FDB/ARP推断的终端设备）
	NodeTypeUnknown   NodeType = "unknown"   // 未知类型
)

// TaskRunDevice 运行期设备信息
// 阶段3架构演进：引入NodeUUID作为主键，支持多IP设备
type TaskRunDevice struct {
	ID             uint       `gorm:"primaryKey;autoIncrement" json:"id"`
	TaskRunID      string     `gorm:"index;not null" json:"taskRunId"`
	NodeUUID       string     `gorm:"index" json:"nodeUuid"` // 阶段3新增：全局唯一节点标识
	DeviceID       uint       `json:"deviceId"`
	DeviceIP       string     `gorm:"index;not null" json:"deviceIp"`
	AllIPs         string     `gorm:"type:text" json:"allIps"` // 阶段3新增：所有IP地址JSON数组
	Status         string     `json:"status"`
	ErrorMessage   string     `json:"errorMessage"`
	Vendor         string     `json:"vendor"`
	VendorSource   string     `json:"vendorSource"`
	DisplayName    string     `json:"displayName"`
	Role           string     `json:"role"`
	Site           string     `json:"site"`
	Hostname       string     `json:"hostname"`
	Model          string     `json:"model"`
	SerialNumber   string     `json:"serialNumber"`
	Version        string     `json:"version"`
	MgmtIP         string     `json:"mgmtIp"`
	NormalizedName string     `json:"normalizedName"`
	ChassisID      string     `json:"chassisId"`
	NodeType       NodeType   `json:"nodeType"` // 阶段3新增：节点类型
	StartedAt      *time.Time `json:"startedAt"`
	FinishedAt     *time.Time `json:"finishedAt"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

func (TaskRunDevice) TableName() string {
	return "task_run_devices"
}

// TaskRawOutput 运行期命令输出索引
type TaskRawOutput struct {
	ID             uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	TaskRunID      string    `gorm:"index;not null" json:"taskRunId"`
	DeviceIP       string    `gorm:"index;not null" json:"deviceIp"`
	CommandKey     string    `gorm:"index" json:"commandKey"`
	Command        string    `json:"command"`
	FieldEnabled   bool      `json:"fieldEnabled"`
	CommandSource  string    `json:"commandSource"`
	ResolvedVendor string    `json:"resolvedVendor"`
	VendorSource   string    `json:"vendorSource"`
	RawFilePath    string    `json:"rawFilePath"`
	ParseFilePath  string    `json:"parseFilePath"`
	Status         string    `json:"status"`
	ParseStatus    string    `gorm:"index" json:"parseStatus"`
	ParseError     string    `json:"parseError"`
	RawSize        int64     `json:"rawSize"`
	NormalizedSize int64     `json:"normalizedSize"`
	LineCount      int       `json:"lineCount"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// TopologyCollectionPlanCommand 拓扑采集计划中的字段命令项。
type TopologyCollectionPlanCommand struct {
	FieldKey       string `json:"fieldKey"`
	DisplayName    string `json:"displayName"`
	Enabled        bool   `json:"enabled"`
	Command        string `json:"command"`
	TimeoutSec     int    `json:"timeoutSec"`
	CommandSource  string `json:"commandSource"`
	ResolvedVendor string `json:"resolvedVendor"`
	VendorSource   string `json:"vendorSource"`
	ParserBinding  string `json:"parserBinding"`
	Required       bool   `json:"required"`
	Description    string `json:"description"`
}

// TopologyCollectionPlanArtifact 拓扑采集计划快照。
type TopologyCollectionPlanArtifact struct {
	RunID           string                          `json:"runId"`
	StageID         string                          `json:"stageId"`
	UnitID          string                          `json:"unitId"`
	DeviceIP        string                          `json:"deviceIp"`
	ResolvedVendor  string                          `json:"resolvedVendor"`
	VendorSource    string                          `json:"vendorSource"`
	CollectedFields []string                        `json:"collectedFields"`
	Commands        []TopologyCollectionPlanCommand `json:"commands"`
	GeneratedAt     time.Time                       `json:"generatedAt"`
	ArtifactKey     string                          `json:"artifactKey,omitempty"`
	FilePath        string                          `json:"filePath,omitempty"`
}

func (TaskRawOutput) TableName() string {
	return "task_raw_outputs"
}

// TaskParsedInterface 解析后的接口事实
type TaskParsedInterface struct {
	ID            uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	TaskRunID     string    `gorm:"index;not null" json:"taskRunId"`
	DeviceIP      string    `gorm:"index;not null" json:"deviceIp"`
	InterfaceName string    `gorm:"index" json:"interfaceName"`
	Status        string    `json:"status"`
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

func (TaskParsedInterface) TableName() string {
	return "task_parsed_interfaces"
}

// TaskParsedLLDPNeighbor 解析后的 LLDP 事实
type TaskParsedLLDPNeighbor struct {
	ID              uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	TaskRunID       string    `gorm:"index;not null" json:"taskRunId"`
	DeviceIP        string    `gorm:"index;not null" json:"deviceIp"`
	LocalInterface  string    `gorm:"index" json:"localInterface"`
	NeighborName    string    `json:"neighborName"`
	NeighborChassis string    `json:"neighborChassis"`
	NeighborPort    string    `json:"neighborPort"`
	NeighborIP      string    `json:"neighborIp"`
	NeighborDesc    string    `json:"neighborDesc"`
	CommandKey      string    `json:"commandKey"`
	RawRefID        string    `json:"rawRefId"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

func (TaskParsedLLDPNeighbor) TableName() string {
	return "task_parsed_lldp_neighbors"
}

// TaskParsedFDBEntry 解析后的 FDB 事实
type TaskParsedFDBEntry struct {
	ID         uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	TaskRunID  string    `gorm:"index;not null" json:"taskRunId"`
	DeviceIP   string    `gorm:"index;not null" json:"deviceIp"`
	MACAddress string    `gorm:"index" json:"macAddress"`
	VLAN       int       `json:"vlan"`
	Interface  string    `gorm:"index" json:"interface"`
	Type       string    `json:"type"`
	CommandKey string    `json:"commandKey"`
	RawRefID   string    `json:"rawRefId"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

func (TaskParsedFDBEntry) TableName() string {
	return "task_parsed_fdb_entries"
}

// TaskParsedARPEntry 解析后的 ARP 事实
type TaskParsedARPEntry struct {
	ID         uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	TaskRunID  string    `gorm:"index;not null" json:"taskRunId"`
	DeviceIP   string    `gorm:"index;not null" json:"deviceIp"`
	IPAddress  string    `gorm:"index" json:"ipAddress"`
	MACAddress string    `gorm:"index" json:"macAddress"`
	Interface  string    `gorm:"index" json:"interface"`
	Type       string    `json:"type"`
	CommandKey string    `json:"commandKey"`
	RawRefID   string    `json:"rawRefId"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

func (TaskParsedARPEntry) TableName() string {
	return "task_parsed_arp_entries"
}

// TaskParsedAggregateGroup 解析后的聚合组事实
type TaskParsedAggregateGroup struct {
	ID            uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	TaskRunID     string    `gorm:"index;not null" json:"taskRunId"`
	DeviceIP      string    `gorm:"index;not null" json:"deviceIp"`
	AggregateName string    `gorm:"index" json:"aggregateName"`
	Mode          string    `json:"mode"`
	CommandKey    string    `json:"commandKey"`
	RawRefID      string    `json:"rawRefId"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

func (TaskParsedAggregateGroup) TableName() string {
	return "task_parsed_aggregate_groups"
}

// TaskParsedAggregateMember 解析后的聚合成员事实
type TaskParsedAggregateMember struct {
	ID            uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	TaskRunID     string    `gorm:"index;not null" json:"taskRunId"`
	DeviceIP      string    `gorm:"index;not null" json:"deviceIp"`
	AggregateName string    `gorm:"index" json:"aggregateName"`
	MemberPort    string    `gorm:"index" json:"memberPort"`
	CommandKey    string    `json:"commandKey"`
	RawRefID      string    `json:"rawRefId"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

func (TaskParsedAggregateMember) TableName() string {
	return "task_parsed_aggregate_members"
}

// TaskTopologyEdge 运行期拓扑边
type TaskTopologyEdge struct {
	ID               string         `gorm:"primaryKey" json:"id"`
	TaskRunID        string         `gorm:"index;not null" json:"taskRunId"`
	ADeviceID        string         `gorm:"index" json:"aDeviceId"`
	AIf              string         `json:"aIf"`
	BDeviceID        string         `gorm:"index" json:"bDeviceId"`
	BIf              string         `json:"bIf"`
	LogicalAIf       string         `json:"logicalAIf"`
	LogicalBIf       string         `json:"logicalBIf"`
	EdgeType         string         `json:"edgeType"`
	Status           string         `json:"status"`
	Confidence       float64        `json:"confidence"`
	DiscoveryMethods []string       `gorm:"serializer:json" json:"discoveryMethods"`
	Evidence         []EdgeEvidence `gorm:"serializer:json" json:"evidence"`
	// Phase A 扩展字段：置信度拆解与决策解释
	ConfidenceBreakdown string    `gorm:"type:text" json:"confidenceBreakdown"` // JSON 序列化的评分明细
	DecisionReason      string    `json:"decisionReason"`                       // 决策原因
	EvidenceRefs        []string  `gorm:"serializer:json" json:"evidenceRefs"`  // 证据引用 ID 列表
	CandidateID         string    `json:"candidateId"`                          // 关联的候选 ID
	TraceID             string    `json:"traceId"`                              // 关联的决策轨迹 ID
	CreatedAt           time.Time `json:"createdAt"`
	UpdatedAt           time.Time `json:"updatedAt"`
}

func (TaskTopologyEdge) TableName() string {
	return "task_topology_edges"
}

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
