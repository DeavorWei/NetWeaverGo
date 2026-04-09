package taskexec

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestNodeIdentityResolver 测试节点身份解析器
func TestNodeIdentityResolver(t *testing.T) {
	resolver := NewNodeIdentityResolver()

	t.Run("RegisterManagedDevice", func(t *testing.T) {
		uuid := resolver.RegisterManagedDevice("192.168.1.1", "")
		assert.NotEmpty(t, uuid)
		assert.True(t, len(uuid) > 10)

		// 验证可以获取UUID
		retrievedUUID, ok := resolver.GetNodeUUID("192.168.1.1")
		assert.True(t, ok)
		assert.Equal(t, uuid, retrievedUUID)

		// 验证节点类型
		assert.Equal(t, NodeTypeManaged, resolver.GetNodeType("192.168.1.1"))
	})

	t.Run("ReuseExistingUUID", func(t *testing.T) {
		existingUUID := "node_test_existing_uuid"
		uuid := resolver.RegisterManagedDevice("192.168.1.2", existingUUID)
		assert.Equal(t, existingUUID, uuid)
	})

	t.Run("AddDeviceIP", func(t *testing.T) {
		resolver.RegisterManagedDevice("192.168.1.3", "")
		resolver.AddDeviceIP("192.168.1.3", "10.0.0.3")
		resolver.AddDeviceIP("192.168.1.3", "172.16.0.3")

		allIPs := resolver.GetAllDeviceIPs("192.168.1.3")
		assert.Contains(t, allIPs, "192.168.1.3")
		assert.Contains(t, allIPs, "10.0.0.3")
		assert.Contains(t, allIPs, "172.16.0.3")
	})

	t.Run("ResolveDeviceByIP_DeviceIP", func(t *testing.T) {
		resolver.RegisterManagedDevice("192.168.1.4", "")
		deviceIP, found := resolver.ResolveDeviceByIP("192.168.1.4")
		assert.True(t, found)
		assert.Equal(t, "192.168.1.4", deviceIP)
	})

	t.Run("ResolveDeviceByIP_AdditionalIP", func(t *testing.T) {
		resolver.RegisterManagedDevice("192.168.1.5", "")
		resolver.AddDeviceIP("192.168.1.5", "10.0.0.5")

		deviceIP, found := resolver.ResolveDeviceByIP("10.0.0.5")
		assert.True(t, found)
		assert.Equal(t, "192.168.1.5", deviceIP)
	})

	t.Run("ResolveDeviceByIP_NotFound", func(t *testing.T) {
		_, found := resolver.ResolveDeviceByIP("192.168.99.99")
		assert.False(t, found)
	})

	t.Run("GetAllIPsMap", func(t *testing.T) {
		resolver.RegisterManagedDevice("192.168.1.6", "")
		resolver.AddDeviceIP("192.168.1.6", "10.0.0.6")

		allIPsMap := resolver.GetAllIPsMap()
		assert.Equal(t, "192.168.1.6", allIPsMap["192.168.1.6"])
		assert.Equal(t, "192.168.1.6", allIPsMap["10.0.0.6"])
	})
}

// TestNodeTypeMethods 测试节点类型方法
func TestNodeTypeMethods(t *testing.T) {
	t.Run("NodeTypeManaged", func(t *testing.T) {
		nt := NodeTypeManaged
		assert.True(t, nt.IsManaged())
		assert.False(t, nt.IsUnmanaged())
		assert.False(t, nt.IsInferred())
	})

	t.Run("NodeTypeUnmanaged", func(t *testing.T) {
		nt := NodeTypeUnmanaged
		assert.False(t, nt.IsManaged())
		assert.True(t, nt.IsUnmanaged())
		assert.False(t, nt.IsInferred())
	})

	t.Run("NodeTypeInferred", func(t *testing.T) {
		nt := NodeTypeInferred
		assert.False(t, nt.IsManaged())
		assert.False(t, nt.IsUnmanaged())
		assert.True(t, nt.IsInferred())
	})
}

// TestAllDeviceIPsJSON 测试IP序列化
func TestAllDeviceIPsJSON(t *testing.T) {
	t.Run("ToJSON", func(t *testing.T) {
		allIPs := map[string][]string{
			"192.168.1.1": {"192.168.1.1", "10.0.0.1"},
		}
		json := AllDeviceIPsToJSON(allIPs)
		assert.NotEmpty(t, json)
		assert.Contains(t, json, "192.168.1.1")
	})

	t.Run("FromJSON", func(t *testing.T) {
		jsonStr := `{"192.168.1.1":["192.168.1.1","10.0.0.1"]}`
		result := ParseAllDeviceIPsFromJSON(jsonStr)
		assert.Contains(t, result["192.168.1.1"], "192.168.1.1")
		assert.Contains(t, result["192.168.1.1"], "10.0.0.1")
	})

	t.Run("EmptyJSON", func(t *testing.T) {
		result := ParseAllDeviceIPsFromJSON("")
		assert.NotNil(t, result)
		assert.Empty(t, result)
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		result := ParseAllDeviceIPsFromJSON("invalid json")
		assert.NotNil(t, result)
		assert.Empty(t, result)
	})
}

// TestIsUnmanagedNodeID 测试节点ID类型检测
func TestIsUnmanagedNodeID(t *testing.T) {
	assert.True(t, IsUnmanagedNodeID("unmanaged:192.168.1.1"))
	assert.True(t, IsUnmanagedNodeID("unmanaged:some-device"))
	assert.False(t, IsUnmanagedNodeID("192.168.1.1"))
	assert.False(t, IsUnmanagedNodeID("node_abc123"))
	assert.False(t, IsUnmanagedNodeID("server:10.0.0.1"))
}

// TestIsInferredNodeID 测试推断节点检测
func TestIsInferredNodeID(t *testing.T) {
	assert.True(t, IsInferredNodeID("server:10.0.0.1"))
	assert.True(t, IsInferredNodeID("terminal:10.0.0.2"))
	assert.True(t, IsInferredNodeID("inferred:some-device"))
	assert.False(t, IsInferredNodeID("192.168.1.1"))
	assert.False(t, IsInferredNodeID("unmanaged:device"))
}

// TestExtractNodeTypeFromID 测试从ID提取节点类型
func TestExtractNodeTypeFromID(t *testing.T) {
	t.Run("Managed_IP", func(t *testing.T) {
		nt := ExtractNodeTypeFromID("192.168.1.1")
		assert.Equal(t, NodeTypeManaged, nt)
	})

	t.Run("Managed_UUID", func(t *testing.T) {
		nt := ExtractNodeTypeFromID("node_abc123")
		assert.Equal(t, NodeTypeManaged, nt)
	})

	t.Run("Unmanaged", func(t *testing.T) {
		nt := ExtractNodeTypeFromID("unmanaged:192.168.1.1")
		assert.Equal(t, NodeTypeUnmanaged, nt)
	})

	t.Run("Inferred_Server", func(t *testing.T) {
		nt := ExtractNodeTypeFromID("server:10.0.0.1")
		assert.Equal(t, NodeTypeInferred, nt)
	})

	t.Run("Inferred_Terminal", func(t *testing.T) {
		nt := ExtractNodeTypeFromID("terminal:10.0.0.2")
		assert.Equal(t, NodeTypeInferred, nt)
	})

	t.Run("Unknown", func(t *testing.T) {
		nt := ExtractNodeTypeFromID("some-random-id")
		assert.Equal(t, NodeTypeUnknown, nt)
	})
}

// TestPhase3BuildContext 测试Phase3构建上下文
func TestPhase3BuildContext(t *testing.T) {
	ctx := NewPhase3BuildContext()
	assert.NotNil(t, ctx)
	assert.NotNil(t, ctx.NodeIdentityResolver)

	// 测试注册设备
	uuid := ctx.RegisterManagedDevice("192.168.1.1", "")
	assert.NotEmpty(t, uuid)

	// 测试添加IP
	ctx.AddDeviceIP("192.168.1.1", "10.0.0.1")
	deviceIP, found := ctx.ResolveDeviceByIP("10.0.0.1")
	assert.True(t, found)
	assert.Equal(t, "192.168.1.1", deviceIP)
}

// TestCreateUnmanagedNode 测试创建未管理节点
func TestCreateUnmanagedNode(t *testing.T) {
	resolver := NewNodeIdentityResolver()

	id := resolver.CreateUnmanagedNode("192.168.99.99", NodeTypeUnmanaged)
	assert.Equal(t, "unmanaged:192.168.99.99", id)

	id2 := resolver.CreateUnmanagedNode("some-device", NodeTypeInferred)
	assert.Equal(t, "inferred:some-device", id2)
}

// TestNormalizedFactsWithAllDeviceIPs 测试标准化事实中的全量IP映射
func TestNormalizedFactsWithAllDeviceIPs(t *testing.T) {
	input := &TopologyBuildInput{
		Devices: []TaskRunDevice{
			{
				DeviceIP:       "192.168.1.1",
				NormalizedName: "switch-1",
				ChassisID:      "00:11:22:33:44:55",
				MgmtIP:         "10.0.0.1",
			},
		},
		Interfaces: []TaskParsedInterface{
			{
				DeviceIP:      "192.168.1.1",
				InterfaceName: "Loopback0",
				IPAddress:     "1.1.1.1",
			},
			{
				DeviceIP:      "192.168.1.1",
				InterfaceName: "Vlan10",
				IPAddress:     "10.10.10.1",
			},
		},
	}

	builder := &TopologyBuilder{config: DefaultTopologyBuildConfig()}
	n := builder.normalizeFacts(input)

	// 验证 AllDeviceIPs 映射
	assert.NotNil(t, n.AllDeviceIPs)

	// DeviceIP 应该在映射中
	assert.Equal(t, "192.168.1.1", n.AllDeviceIPs["192.168.1.1"])

	// MgmtIP 应该在映射中
	assert.Equal(t, "192.168.1.1", n.AllDeviceIPs["10.0.0.1"])

	// 接口IP应该在映射中
	assert.Equal(t, "192.168.1.1", n.AllDeviceIPs["1.1.1.1"])
	assert.Equal(t, "192.168.1.1", n.AllDeviceIPs["10.10.10.1"])

	// 验证设备信息
	dev := n.Devices["192.168.1.1"]
	assert.NotNil(t, dev)
	assert.NotEmpty(t, dev.NodeUUID) // 应该生成了UUID
	assert.Equal(t, NodeTypeManaged, dev.NodeType)

	// 验证AllIPs包含接口IP
	assert.Contains(t, dev.AllIPs, "1.1.1.1")
	assert.Contains(t, dev.AllIPs, "10.10.10.1")
}

// TestResolveLLDPPeerWithAllDeviceIPs 测试使用全量IP映射解析LLDP对端
func TestResolveLLDPPeerWithAllDeviceIPs(t *testing.T) {
	builder := &TopologyBuilder{config: DefaultTopologyBuildConfig()}

	n := &NormalizedFacts{
		Devices: map[string]*DeviceInfo{
			"192.168.1.1": {
				NodeUUID: "node_test_001",
				DeviceIP: "192.168.1.1",
				AllIPs:   []string{"192.168.1.1", "10.0.0.1", "1.1.1.1"},
			},
		},
		DeviceByMgmtIP:    map[string]string{"192.168.1.1": "192.168.1.1"},
		DeviceByChassisID: map[string]string{},
		DeviceByName:      map[string]string{},
		AllDeviceIPs: map[string]string{
			"192.168.1.1": "192.168.1.1",
			"10.0.0.1":    "192.168.1.1",
			"1.1.1.1":     "192.168.1.1",
		},
	}

	t.Run("MatchViaAllDeviceIPs", func(t *testing.T) {
		lldp := NormalizedLLDPNeighbor{
			DeviceIP:     "192.168.1.2",
			LocalIf:      "Gi0/0/1",
			NeighborIP:   "1.1.1.1", // 不在DeviceByMgmtIP中，但在AllDeviceIPs中
			NeighborName: "Switch-1",
		}

		deviceIP, source := builder.resolveLLDPPeer(lldp, n)
		assert.Equal(t, "192.168.1.1", deviceIP)
		assert.Equal(t, "neighbor_ip_extended", source)
	})

	t.Run("MatchViaDeviceByMgmtIP", func(t *testing.T) {
		lldp := NormalizedLLDPNeighbor{
			DeviceIP:     "192.168.1.2",
			LocalIf:      "Gi0/0/1",
			NeighborIP:   "192.168.1.1", // 在DeviceByMgmtIP中
			NeighborName: "Switch-1",
		}

		deviceIP, source := builder.resolveLLDPPeer(lldp, n)
		assert.Equal(t, "192.168.1.1", deviceIP)
		assert.Equal(t, "neighbor_ip", source)
	})
}
