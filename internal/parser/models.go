package parser

import (
	"errors"
	"regexp"
	"time"
)

// DeviceIdentity 设备身份信息
type DeviceIdentity struct {
	Vendor       string `json:"vendor"`       // 厂商：huawei / h3c / cisco
	Model        string `json:"model"`        // 型号
	SerialNumber string `json:"serialNumber"` // 序列号
	Version      string `json:"version"`      // 版本
	Hostname     string `json:"hostname"`     // 主机名
	MgmtIP       string `json:"mgmtIp"`       // 管理IP
	ChassisID    string `json:"chassisId"`    // 机箱ID（LLDP）
	RawRefID     string `json:"rawRefId"`     // 原始输出引用ID
}

// InterfaceFact 接口信息
type InterfaceFact struct {
	Name        string `json:"name"`              // 接口名
	Status      string `json:"status"`            // 状态：up / down
	Protocol    string `json:"protocol"`          // 协议状态
	Speed       string `json:"speed"`             // 速率
	Duplex      string `json:"duplex"`            // 双工模式
	Description string `json:"description"`       // 描述
	MACAddress  string `json:"macAddress"`        // MAC地址
	IPAddress   string `json:"ipAddress"`         // IP地址
	IsAggregate bool   `json:"isAggregate"`       // 是否聚合口
	AggregateID string `json:"aggregateId"`       // 所属聚合组ID
	VLAN        string `json:"vlan,omitempty"`    // Cisco特有：VLAN或trunk
	IsTrunk     bool   `json:"isTrunk,omitempty"` // Cisco特有：是否为trunk
}

// LLDPFact LLDP邻居信息
type LLDPFact struct {
	LocalInterface  string `json:"localInterface"`  // 本地接口
	NeighborName    string `json:"neighborName"`    // 邻居设备名
	NeighborChassis string `json:"neighborChassis"` // 邻居机箱ID
	NeighborPort    string `json:"neighborPort"`    // 邻居端口
	NeighborIP      string `json:"neighborIp"`      // 邻居管理IP
	NeighborDesc    string `json:"neighborDesc"`    // 邻居描述
	CommandKey      string `json:"commandKey"`      // 来源命令
	RawRefID        string `json:"rawRefId"`        // 原始输出引用ID
}

// FDBFact MAC地址表项
type FDBFact struct {
	MACAddress string `json:"macAddress"` // MAC地址
	VLAN       int    `json:"vlan"`       // VLAN ID
	Interface  string `json:"interface"`  // 接口
	Type       string `json:"type"`       // 类型：dynamic / static
	CommandKey string `json:"commandKey"` // 来源命令
	RawRefID   string `json:"rawRefId"`   // 原始输出引用ID
}

// ARPFact ARP表项
type ARPFact struct {
	IPAddress  string `json:"ipAddress"`  // IP地址
	MACAddress string `json:"macAddress"` // MAC地址
	Interface  string `json:"interface"`  // 接口
	Type       string `json:"type"`       // 类型
	CommandKey string `json:"commandKey"` // 来源命令
	RawRefID   string `json:"rawRefId"`   // 原始输出引用ID
}

// AggregateFact 聚合组信息
type AggregateFact struct {
	AggregateName string   `json:"aggregateName"` // 聚合口名称
	MemberPorts   []string `json:"memberPorts"`   // 成员端口列表
	Mode          string   `json:"mode"`          // 模式：lacp / static
	CommandKey    string   `json:"commandKey"`    // 来源命令
	RawRefID      string   `json:"rawRefId"`      // 原始输出引用ID
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

// ============================================================================
// 解析器管理接口（任务 1.1）
// ============================================================================

// ParserProvider 解析器提供者接口
// 用于从管理器获取指定厂商的只读解析器快照
type ParserProvider interface {
	// GetParser 获取指定厂商的解析器
	GetParser(vendor string) (CliParser, error)
}

// ParserReloader 解析器重载接口
// 用于在模板变更后刷新解析器快照
type ParserReloader interface {
	// ReloadVendor 重载指定厂商的解析器快照
	ReloadVendor(vendor string) error
}

// ============================================================================
// 统一模板 DSL（任务 1.2）
// ============================================================================

// TemplateEngine 模板引擎类型
type TemplateEngine string

const (
	// EngineRegex 纯正则引擎
	EngineRegex TemplateEngine = "regex"
	// EngineAggregate 多行聚合引擎
	EngineAggregate TemplateEngine = "aggregate"
)

// RegexTemplate 统一模板定义
type RegexTemplate struct {
	Vendor       string             `json:"vendor,omitempty"`
	CommandKey   string             `json:"commandKey"`
	Engine       TemplateEngine     `json:"engine"`
	Pattern      string             `json:"pattern,omitempty"`
	Multiline    bool               `json:"multiline,omitempty"`
	Aggregation  *AggregationConfig `json:"aggregation,omitempty"`
	FieldMapping map[string]string  `json:"fieldMapping,omitempty"`
	Description  string             `json:"description,omitempty"`
}

// AggregationConfig 多行聚合配置
type AggregationConfig struct {
	// RecordStart 记录起始模式列表
	RecordStart []string `json:"recordStart,omitempty"`
	// CaptureRules 字段捕获规则
	CaptureRules []CaptureRule `json:"captureRules,omitempty"`
	// Filldown 需要向下填充的字段列表
	Filldown []string `json:"filldown,omitempty"`
	// EmitWhen 记录输出条件字段列表
	EmitWhen []string `json:"emitWhen,omitempty"`
}

// CaptureRule 字段捕获规则
type CaptureRule struct {
	// Pattern 正则模式
	Pattern string `json:"pattern"`
	// Mode 捕获模式：set / append
	Mode string `json:"mode"`
}

// CompiledTemplate 已编译的模板
type CompiledTemplate struct {
	RegexTemplate
	// CompiledPattern 已编译的主正则
	CompiledPattern *regexp.Regexp
	// CompiledRecordStart 已编译的记录起始模式
	CompiledRecordStart []*regexp.Regexp
	// CompiledCaptureRules 已编译的捕获规则
	CompiledCaptureRules []CompiledCaptureRule
}

// CompiledCaptureRule 已编译的捕获规则
type CompiledCaptureRule struct {
	Pattern         *regexp.Regexp
	Mode            string
	OriginalPattern string
}

// VendorTemplates 厂商模板集合
type VendorTemplates struct {
	Vendor    string                   `json:"vendor"`
	Templates map[string]RegexTemplate `json:"templates"`
}

// ============================================================================
// 解析器错误定义
// ============================================================================

var (
	// ErrNilTemplate 模板为空
	ErrNilTemplate = errors.New("模板为空")
	// ErrPatternNotCompiled 正则模式未编译
	ErrPatternNotCompiled = errors.New("正则模式未编译")
	// ErrTemplateNotFound 模板未找到
	ErrTemplateNotFound = errors.New("模板未找到")
	// ErrVendorNotLoaded 厂商解析器未加载
	ErrVendorNotLoaded = errors.New("厂商解析器未加载")
	// ErrUnsupportedEngine 不支持的模板引擎
	ErrUnsupportedEngine = errors.New("不支持的模板引擎")
	// ErrInvalidAggregationConfig 无效的聚合配置
	ErrInvalidAggregationConfig = errors.New("无效的聚合配置")
)
