package taskexec

import "time"

// TaskRunDevice 运行期设备信息
type TaskRunDevice struct {
	ID             uint       `gorm:"primaryKey;autoIncrement" json:"id"`
	TaskRunID      string     `gorm:"index;not null" json:"taskRunId"`
	DeviceID       uint       `json:"deviceId"`
	DeviceIP       string     `gorm:"index;not null" json:"deviceIp"`
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
	CreatedAt        time.Time      `json:"createdAt"`
	UpdatedAt        time.Time      `json:"updatedAt"`
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
