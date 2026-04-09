package taskexec

// =============================================================================
// Phase 3: 拓扑查询增强 - 支持 NodeType 和多IP展示
// =============================================================================

import (
	"encoding/json"
	"strings"

	"github.com/NetWeaverGo/core/internal/models"
)

// parseAllIPs 解析存储在 AllIPs 字段中的 JSON 数组
func parseAllIPs(allIPsJSON string) []string {
	if allIPsJSON == "" {
		return nil
	}
	var result []string
	if err := json.Unmarshal([]byte(allIPsJSON), &result); err != nil {
		return nil
	}
	return result
}

// BuildGraphNodeWithPhase3 构建支持阶段3特性的图节点
// 用于 GetTopologyGraph 函数中创建节点
func BuildGraphNodeWithPhase3(id string, device TaskRunDevice, isManaged bool) models.GraphNode {
	node := models.GraphNode{
		ID:           id,
		Label:        chooseValue(device.DisplayName, device.Hostname, device.Model, device.DeviceIP),
		IP:           device.DeviceIP,
		Vendor:       device.Vendor,
		Model:        device.Model,
		Role:         device.Role,
		Site:         device.Site,
		SerialNumber: device.SerialNumber,
		ChassisID:    device.ChassisID,
	}

	// 阶段3特性
	if isManaged {
		node.NodeUUID = device.NodeUUID
		node.NodeType = models.NodeTypeManaged
		if device.AllIPs != "" {
			node.AllIPs = parseAllIPs(device.AllIPs)
		}
	}

	return node
}

// BuildInferredNode 构建推断节点（server/terminal）
func BuildInferredNode(id string, prefix string) models.GraphNode {
	cleanID := strings.TrimPrefix(id, prefix+":")
	var role string
	if prefix == "server" {
		role = "server-inferred"
	} else {
		role = "terminal-inferred"
	}

	return models.GraphNode{
		ID:       id,
		Label:    cleanID,
		IP:       cleanID,
		Role:     role,
		Vendor:   "endpoint",
		NodeType: models.NodeTypeInferred,
	}
}

// BuildUnmanagedNode 构建未管理节点
func BuildUnmanagedNode(id string) models.GraphNode {
	// 支持两种前缀格式：unmanaged: 和 unmanaged:
	cleanID := id
	if strings.HasPrefix(id, string(NodeTypeUnmanaged)+":") {
		cleanID = strings.TrimPrefix(id, string(NodeTypeUnmanaged)+":")
	} else if strings.HasPrefix(id, "unmanaged:") {
		cleanID = strings.TrimPrefix(id, "unmanaged:")
	}

	return models.GraphNode{
		ID:       id,
		Label:    cleanID,
		IP:       cleanID,
		Role:     "unmanaged",
		Vendor:   "unknown",
		NodeType: models.NodeTypeUnmanaged,
	}
}

// DetectNodeType 从节点ID检测节点类型
// 用于向后兼容旧数据
func DetectNodeType(nodeID string) models.NodeType {
	if strings.HasPrefix(nodeID, string(NodeTypeUnmanaged)+":") ||
		strings.HasPrefix(nodeID, "unmanaged:") {
		return models.NodeTypeUnmanaged
	}
	if strings.HasPrefix(nodeID, "server:") ||
		strings.HasPrefix(nodeID, "terminal:") ||
		strings.HasPrefix(nodeID, string(NodeTypeInferred)+":") {
		return models.NodeTypeInferred
	}
	// UUID格式或IP格式认为是Managed
	if strings.HasPrefix(nodeID, "node_") || isIPAddress(nodeID) {
		return models.NodeTypeManaged
	}
	return models.NodeTypeUnknown
}

// isIPAddress 简单判断是否为IP地址
func isIPAddress(s string) bool {
	return strings.Count(s, ".") == 3 && strings.ContainsAny(s, "0123456789")
}
