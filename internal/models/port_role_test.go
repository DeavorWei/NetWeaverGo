package models_test

import (
	"testing"

	"github.com/NetWeaverGo/core/internal/models"
)

func TestInferPortRole(t *testing.T) {
	tests := []struct {
		name       string
		localRole  string
		remoteRole string
		localIf    string
		remoteIf   string
		expected   models.PortRole
	}{
		{
			name:     "堆叠端口判定",
			localIf:  "Stack-Port1/1",
			remoteIf: "Stack-Port2/1",
			expected: models.PortRoleStacking,
		},
		{
			name:     "IRF 端口判定",
			localIf:  "IRF-Port1/1",
			remoteIf: "Ten-GigabitEthernet1/0/49",
			expected: models.PortRoleStacking,
		},
		{
			name:     "带外管理口判定",
			localIf:  "MEth0/0/1",
			remoteIf: "GigabitEthernet0/0/1",
			expected: models.PortRoleOOB,
		},
		{
			name:       "接入交换机上联到核心交换机",
			localRole:  "Access",
			remoteRole: "Core",
			localIf:    "10GE1/0/1",
			remoteIf:   "10GE1/0/24",
			expected:   models.PortRoleUplink,
		},
		{
			name:       "核心交换机下联到接入交换机",
			localRole:  "Core",
			remoteRole: "Access",
			localIf:    "10GE1/0/24",
			remoteIf:   "10GE1/0/1",
			expected:   models.PortRoleDownlink,
		},
		{
			name:       "Spine 与 Spine 横向互联",
			localRole:  "Spine",
			remoteRole: "Spine",
			localIf:    "100GE1/0/1",
			remoteIf:   "100GE1/0/1",
			expected:   models.PortRoleInterconnect,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := models.InferPortRole(tt.localRole, tt.remoteRole, tt.localIf, tt.remoteIf)
			if got != tt.expected {
				t.Errorf("InferPortRole() = %v, 预期 %v", got, tt.expected)
			}
		})
	}
}
