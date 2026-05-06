package taskexec

import (
	"time"

	"github.com/NetWeaverGo/core/internal/models"
)

// TaskRunDevice 运行期设备信息
type TaskRunDevice struct {
	ID             uint          `gorm:"primaryKey;autoIncrement" json:"id"`
	TaskRunID      string        `gorm:"index;not null" json:"taskRunId"`
	NodeUUID       string        `gorm:"index" json:"nodeUuid"`
	DeviceID       uint          `json:"deviceId"`
	DeviceIP       string        `gorm:"index;not null" json:"deviceIp"`
	AllIPs         string        `gorm:"type:text" json:"allIps"`
	Status         string        `json:"status"`
	ErrorMessage   string        `json:"errorMessage"`
	Vendor         string        `json:"vendor"`
	VendorSource   string        `json:"vendorSource"`
	DisplayName    string        `json:"displayName"`
	Role           string        `json:"role"`
	Site           string        `json:"site"`
	Hostname       string        `json:"hostname"`
	Model          string        `json:"model"`
	MgmtIP         string        `json:"mgmtIp"`
	NormalizedName string        `json:"normalizedName"`
	ChassisID      string        `json:"chassisId"`
	NodeType       models.NodeType `json:"nodeType"`
	StartedAt      *time.Time    `json:"startedAt"`
	FinishedAt     *time.Time    `json:"finishedAt"`
	CreatedAt      time.Time     `json:"createdAt"`
	UpdatedAt      time.Time     `json:"updatedAt"`
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
	// 推断节点MAC地址字段（用于IP标识时保留MAC信息）
	BDeviceMAC  string `json:"bDeviceMac"`  // B端设备的主MAC地址（推断节点）
	BDeviceMACs string `gorm:"type:text" json:"bDeviceMacs"` // B端设备的多MAC（JSON数组）
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func (TaskTopologyEdge) TableName() string {
	return "task_topology_edges"
}

// EdgeEvidence 链路证据 — 类型别名，实际定义在 models 包
type EdgeEvidence = models.EdgeEvidence
