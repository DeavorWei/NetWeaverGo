package parser

import "time"

// DeviceIdentity 设备身份信息
type DeviceIdentity struct {
	Vendor       string `json:"vendor"`       // 厂商：huawei / h3c / cisco
	Model        string `json:"model"`        // 型号
	SerialNumber string `json:"serialNumber"` // 序列号
	Version      string `json:"version"`      // 版本
	Hostname     string `json:"hostname"`     // 主机名
	MgmtIP       string `json:"mgmtIp"`       // 管理IP
	ChassisID    string `json:"chassisId"`    // 机箱ID（LLDP）
}

// InterfaceFact 接口信息
type InterfaceFact struct {
	Name        string `json:"name"`        // 接口名
	Status      string `json:"status"`      // 状态：up / down
	Protocol    string `json:"protocol"`    // 协议状态
	Speed       string `json:"speed"`       // 速率
	Duplex      string `json:"duplex"`      // 双工模式
	Description string `json:"description"` // 描述
	MACAddress  string `json:"macAddress"`  // MAC地址
	IPAddress   string `json:"ipAddress"`   // IP地址
	IsAggregate bool   `json:"isAggregate"` // 是否聚合口
	AggregateID string `json:"aggregateId"` // 所属聚合组ID
}

// LLDPFact LLDP邻居信息
type LLDPFact struct {
	LocalInterface  string `json:"localInterface"`  // 本地接口
	NeighborName    string `json:"neighborName"`    // 邻居设备名
	NeighborChassis string `json:"neighborChassis"` // 邻居机箱ID
	NeighborPort    string `json:"neighborPort"`    // 邻居端口
	NeighborIP      string `json:"neighborIp"`      // 邻居管理IP
	NeighborDesc    string `json:"neighborDesc"`    // 邻居描述
}

// FDBFact MAC地址表项
type FDBFact struct {
	MACAddress string `json:"macAddress"` // MAC地址
	VLAN       int    `json:"vlan"`       // VLAN ID
	Interface  string `json:"interface"`  // 接口
	Type       string `json:"type"`       // 类型：dynamic / static
}

// ARPFact ARP表项
type ARPFact struct {
	IPAddress  string `json:"ipAddress"`  // IP地址
	MACAddress string `json:"macAddress"` // MAC地址
	Interface  string `json:"interface"`  // 接口
	Type       string `json:"type"`       // 类型
}

// AggregateFact 聚合组信息
type AggregateFact struct {
	AggregateName string   `json:"aggregateName"` // 聚合口名称
	MemberPorts   []string `json:"memberPorts"`   // 成员端口列表
	Mode          string   `json:"mode"`          // 模式：lacp / static
}

// ParsedResult 解析结果
type ParsedResult struct {
	TaskID        string          `json:"taskId"`
	DeviceIP      string          `json:"deviceIp"`
	ParsedAt      time.Time       `json:"parsedAt"`
	Identity      *DeviceIdentity `json:"identity,omitempty"`
	Interfaces    []InterfaceFact `json:"interfaces,omitempty"`
	LLDPNeighbors []LLDPFact      `json:"lldpNeighbors,omitempty"`
	FDBEntries    []FDBFact       `json:"fdbEntries,omitempty"`
	ARPEntries    []ARPFact       `json:"arpEntries,omitempty"`
	Aggregates    []AggregateFact `json:"aggregates,omitempty"`
	ParseErrors   []string        `json:"parseErrors,omitempty"`
}

// CliParser CLI解析器接口
type CliParser interface {
	// Parse 解析原始CLI输出
	Parse(commandKey string, rawText string) ([]map[string]string, error)
}

// ResultMapper 结果映射器接口
type ResultMapper interface {
	// ToDeviceInfo 将version解析结果映射为设备身份信息
	ToDeviceInfo(rows []map[string]string) (*DeviceIdentity, error)
	// ToInterfaces 将接口解析结果映射为接口信息
	ToInterfaces(rows []map[string]string) ([]InterfaceFact, error)
	// ToLLDP 将LLDP解析结果映射为邻居信息
	ToLLDP(rows []map[string]string) ([]LLDPFact, error)
	// ToFDB 将MAC表解析结果映射为FDB信息
	ToFDB(rows []map[string]string) ([]FDBFact, error)
	// ToARP 将ARP解析结果映射为ARP信息
	ToARP(rows []map[string]string) ([]ARPFact, error)
	// ToAggregate 将聚合口解析结果映射为聚合信息
	ToAggregate(rows []map[string]string) ([]AggregateFact, error)
}
