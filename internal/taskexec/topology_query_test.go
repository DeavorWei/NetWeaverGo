package taskexec

import (
	"testing"

	"github.com/NetWeaverGo/core/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)



// TestExtractMACFromEdges 测试从边信息中提取推断节点的MAC地址
func TestExtractMACFromEdges(t *testing.T) {
	edges := []TaskTopologyEdge{
		{
			ID:         "edge-1",
			BDeviceID:  "server:192.168.1.100",
			BDeviceMAC: "00:50:56:c0:00:01",
		},
		{
			ID:         "edge-2",
			BDeviceID:  "terminal:192.168.1.101",
			BDeviceMAC: "00:50:56:c0:00:02",
		},
		{
			ID:         "edge-3",
			BDeviceID:  "192.168.1.1", // 管理设备，无MAC
			BDeviceMAC: "",
		},
	}

	tests := []struct {
		name     string
		deviceID string
		expected string
	}{
		{
			name:     "提取server节点的MAC",
			deviceID: "server:192.168.1.100",
			expected: "00:50:56:c0:00:01",
		},
		{
			name:     "提取terminal节点的MAC",
			deviceID: "terminal:192.168.1.101",
			expected: "00:50:56:c0:00:02",
		},
		{
			name:     "管理设备无MAC",
			deviceID: "192.168.1.1",
			expected: "",
		},
		{
			name:     "不存在的设备",
			deviceID: "server:192.168.1.200",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mac, _ := extractMACFromEdges(edges, tt.deviceID)
			assert.Equal(t, tt.expected, mac)
		})
	}
}

// TestGetGraphNodeWithMAC 测试getGraphNode函数支持MAC地址参数
func TestGetGraphNodeWithMAC(t *testing.T) {
	db := setupTestDB(t)
	service := NewTaskExecutionService(db, nil)
	runID := "run-mac-test"

	// 创建一个管理设备
	require.NoError(t, db.Create(&TaskRunDevice{
		TaskRunID: runID,
		DeviceIP:  "192.168.1.1",
		Hostname:  "Switch-1",
		Vendor:    "huawei",
		Model:     "S5735",
		Role:      "access",
	}).Error)

	tests := []struct {
		name          string
		deviceID      string
		macAddress    string
		expectedLabel string
		expectedIP    string
		expectedMAC   string
		expectedRole  string
		expectedType  models.NodeType
	}{
		{
			name:          "管理设备",
			deviceID:      "192.168.1.1",
			expectedLabel: "Switch-1",
			expectedIP:    "192.168.1.1",
			expectedMAC:   "",
			expectedRole:  "access",
			expectedType:  models.NodeTypeManaged,
		},
		{
			name:          "Server推断节点带MAC",
			deviceID:      "server:192.168.1.100",
			macAddress:    "00:50:56:c0:00:01",
			expectedLabel: "192.168.1.100",
			expectedIP:    "192.168.1.100",
			expectedMAC:   "00:50:56:c0:00:01",
			expectedRole:  "endpoint-inferred",
			expectedType:  models.NodeTypeInferred,
		},
		{
			name:          "Terminal推断节点带MAC",
			deviceID:      "terminal:192.168.1.101",
			macAddress:    "00:50:56:c0:00:02",
			expectedLabel: "192.168.1.101",
			expectedIP:    "192.168.1.101",
			expectedMAC:   "00:50:56:c0:00:02",
			expectedRole:  "endpoint-inferred",
			expectedType:  models.NodeTypeInferred,
		},
		{
			name:          "Server推断节点无MAC",
			deviceID:      "server:192.168.1.102",
			expectedLabel: "192.168.1.102",
			expectedIP:    "192.168.1.102",
			expectedMAC:   "",
			expectedRole:  "endpoint-inferred",
			expectedType:  models.NodeTypeInferred,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var node models.GraphNode
			if tt.macAddress != "" {
				node = service.getGraphNode(runID, tt.deviceID, tt.macAddress)
			} else {
				node = service.getGraphNode(runID, tt.deviceID)
			}

			assert.Equal(t, tt.deviceID, node.ID)
			assert.Equal(t, tt.expectedLabel, node.Label)
			assert.Equal(t, tt.expectedIP, node.IP)
			assert.Equal(t, tt.expectedMAC, node.MACAddress)
			assert.Equal(t, tt.expectedRole, node.Role)
			assert.Equal(t, tt.expectedType, node.NodeType)
		})
	}
}
