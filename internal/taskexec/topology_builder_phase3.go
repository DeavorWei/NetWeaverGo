package taskexec

// =============================================================================
// Phase 3: 拓扑架构演进 - 长期改进方案实现
// =============================================================================
// 本文件包含阶段3架构演进的核心功能：
// 1. NodeUUID 支持 - 全局唯一节点标识
// 2. 节点类型区分 - Managed / Unmanaged / Inferred
// 3. 全量 IP/MAC 映射表 - 支持多IP设备匹配
//
// 注意：NodeType 定义在 topology_models.go 中
// =============================================================================

import (
	"encoding/json"
	"strings"

	"github.com/NetWeaverGo/core/internal/logger"
)

// IsManaged 检查节点是否为已管理设备
func (nt NodeType) IsManaged() bool {
	return nt == NodeTypeManaged
}

// IsUnmanaged 检查节点是否为未管理设备
func (nt NodeType) IsUnmanaged() bool {
	return nt == NodeTypeUnmanaged
}

// IsInferred 检查节点是否为推断设备
func (nt NodeType) IsInferred() bool {
	return nt == NodeTypeInferred
}

// NodeIdentityResolver 节点身份解析器
// 阶段3架构演进核心组件，负责维护和管理节点身份信息
type NodeIdentityResolver struct {
	// 核心映射表
	nodeUUIDByDeviceIP map[string]string   // DeviceIP -> NodeUUID
	deviceIPByNodeUUID map[string]string   // NodeUUID -> DeviceIP
	allIPsByDeviceIP   map[string][]string // DeviceIP -> 所有IP列表（包括接口IP）
	nodeTypeByDeviceIP map[string]NodeType // DeviceIP -> NodeType

	// 快速查找索引
	deviceByAllIPs map[string]string // 任意IP -> DeviceIP（支持多IP设备匹配）
}

// NewNodeIdentityResolver 创建节点身份解析器
func NewNodeIdentityResolver() *NodeIdentityResolver {
	return &NodeIdentityResolver{
		nodeUUIDByDeviceIP: make(map[string]string),
		deviceIPByNodeUUID: make(map[string]string),
		allIPsByDeviceIP:   make(map[string][]string),
		nodeTypeByDeviceIP: make(map[string]NodeType),
		deviceByAllIPs:     make(map[string]string),
	}
}

// RegisterManagedDevice 注册已管理设备
// 阶段3：为采集列表中的设备分配NodeUUID并建立映射
func (r *NodeIdentityResolver) RegisterManagedDevice(deviceIP string, existingUUID string) string {
	// 如果设备已有UUID则复用，否则生成新的
	nodeUUID := existingUUID
	if nodeUUID == "" {
		nodeUUID = newNodeUUID()
	}

	r.nodeUUIDByDeviceIP[deviceIP] = nodeUUID
	r.deviceIPByNodeUUID[nodeUUID] = deviceIP
	r.nodeTypeByDeviceIP[deviceIP] = NodeTypeManaged
	r.allIPsByDeviceIP[deviceIP] = []string{deviceIP} // 初始包含DeviceIP

	// 将DeviceIP加入全量映射
	r.deviceByAllIPs[deviceIP] = deviceIP

	logger.Verbose("TopologyBuilder", deviceIP,
		"NodeIdentityResolver: 注册已管理设备, nodeUUID=%s", nodeUUID)

	return nodeUUID
}

// AddDeviceIP 为设备添加额外的IP地址
// 阶段3：支持多IP设备（Loopback、SVI等）
func (r *NodeIdentityResolver) AddDeviceIP(deviceIP string, additionalIP string) {
	if additionalIP == "" || additionalIP == deviceIP {
		return
	}

	// 添加到设备的IP列表
	r.allIPsByDeviceIP[deviceIP] = append(r.allIPsByDeviceIP[deviceIP], additionalIP)

	// 建立全局IP映射
	r.deviceByAllIPs[additionalIP] = deviceIP

	logger.Verbose("TopologyBuilder", deviceIP,
		"NodeIdentityResolver: 添加设备IP, additionalIP=%s", additionalIP)
}

// ResolveDeviceByIP 通过任意IP解析设备
// 阶段3：支持多IP设备匹配，查找所有已知IP映射
func (r *NodeIdentityResolver) ResolveDeviceByIP(ip string) (deviceIP string, found bool) {
	if ip == "" {
		return "", false
	}

	// 1. 首先尝试直接匹配DeviceIP
	if _, ok := r.nodeUUIDByDeviceIP[ip]; ok {
		return ip, true
	}

	// 2. 尝试全量IP映射（包括MgmtIP、接口IP等）
	if deviceIP, ok := r.deviceByAllIPs[ip]; ok {
		return deviceIP, true
	}

	return "", false
}

// GetNodeUUID 获取设备的NodeUUID
func (r *NodeIdentityResolver) GetNodeUUID(deviceIP string) (string, bool) {
	uuid, ok := r.nodeUUIDByDeviceIP[deviceIP]
	return uuid, ok
}

// GetNodeType 获取设备的节点类型
func (r *NodeIdentityResolver) GetNodeType(deviceIP string) NodeType {
	if nt, ok := r.nodeTypeByDeviceIP[deviceIP]; ok {
		return nt
	}
	return NodeTypeUnknown
}

// GetAllDeviceIPs 获取设备的所有IP地址
func (r *NodeIdentityResolver) GetAllDeviceIPs(deviceIP string) []string {
	if ips, ok := r.allIPsByDeviceIP[deviceIP]; ok {
		// 返回副本防止外部修改
		result := make([]string, len(ips))
		copy(result, ips)
		return result
	}
	return nil
}

// GetAllIPsMap 获取全量IP映射表的副本（用于LLDP解析）
func (r *NodeIdentityResolver) GetAllIPsMap() map[string]string {
	result := make(map[string]string, len(r.deviceByAllIPs))
	for k, v := range r.deviceByAllIPs {
		result[k] = v
	}
	return result
}

// CreateUnmanagedNode 创建未管理节点标识
// 阶段3：为LLDP发现但不在采集列表中的设备创建标识
func (r *NodeIdentityResolver) CreateUnmanagedNode(identifier string, nodeType NodeType) string {
	// 未管理节点使用前缀标识，不包含在标准UUID映射中
	unmanagedID := string(nodeType) + ":" + identifier

	logger.Verbose("TopologyBuilder", identifier,
		"NodeIdentityResolver: 创建未管理节点, nodeType=%s, id=%s", nodeType, unmanagedID)

	return unmanagedID
}

// Phase3BuildContext Phase 3拓扑构建上下文
// 封装阶段3所有增强功能的运行时上下文
type Phase3BuildContext struct {
	*NodeIdentityResolver
}

// NewPhase3BuildContext 创建Phase 3构建上下文
func NewPhase3BuildContext() *Phase3BuildContext {
	return &Phase3BuildContext{
		NodeIdentityResolver: NewNodeIdentityResolver(),
	}
}

// AllDeviceIPsToJSON 将所有设备IP序列化为JSON
// 用于存储到TaskRunDevice.AllIPs字段
func AllDeviceIPsToJSON(allIPs map[string][]string) string {
	if len(allIPs) == 0 {
		return ""
	}

	data, err := json.Marshal(allIPs)
	if err != nil {
		return ""
	}
	return string(data)
}

// ParseAllDeviceIPsFromJSON 从JSON解析所有设备IP
// 从TaskRunDevice.AllIPs字段恢复
func ParseAllDeviceIPsFromJSON(jsonStr string) map[string][]string {
	if jsonStr == "" {
		return make(map[string][]string)
	}

	var result map[string][]string
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return make(map[string][]string)
	}
	return result
}

// IsUnmanagedNodeID 检查是否为未管理节点ID
func IsUnmanagedNodeID(nodeID string) bool {
	return strings.HasPrefix(nodeID, string(NodeTypeUnmanaged)+":")
}

// IsInferredNodeID 检查是否为推断节点ID
func IsInferredNodeID(nodeID string) bool {
	return strings.HasPrefix(nodeID, string(NodeTypeInferred)+":") ||
		strings.HasPrefix(nodeID, "server:") ||
		strings.HasPrefix(nodeID, "terminal:")
}

// ExtractNodeTypeFromID 从节点ID提取节点类型
func ExtractNodeTypeFromID(nodeID string) NodeType {
	if IsUnmanagedNodeID(nodeID) {
		return NodeTypeUnmanaged
	}
	if IsInferredNodeID(nodeID) {
		return NodeTypeInferred
	}
	// 如果是UUID格式（node_xxx）或IP格式，认为是Managed
	if strings.HasPrefix(nodeID, "node_") || isValidIP(nodeID) {
		return NodeTypeManaged
	}
	return NodeTypeUnknown
}

// isValidIP 简单IP地址验证
func isValidIP(ip string) bool {
	// 简化验证：检查是否包含数字和点
	return strings.Contains(ip, ".") && strings.ContainsAny(ip, "0123456789")
}
