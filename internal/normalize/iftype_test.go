package normalize

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNameToIFType(t *testing.T) {
	testCases := []struct {
		input    string
		expected int
		found    bool
	}{
		{"GigabitEthernet1/0/1", IFTypeEthernet, true},
		{"GE0/0/1", IFTypeEthernet, true},
		{"XGE1/0/1", IFTypeEthernet, true},
		{"100GE1/0/1", IFTypeEthernet, true},
		{"Vlanif100", IFTypeVlanif, true},
		{"Vlan-interface100", IFTypeVlanif, true},
		{"Eth-Trunk10", IFTypeTrunk, true},
		{"Trunk5", IFTypeTrunk, true},
		{"Pos1/0/0", IFTypePos, true},
		{"Tunnel0", IFTypeTunnel, true},
		{"LoopBack0", IFTypeLoopback, true},
		{"UnknownInterface999", 0, false},
	}

	for _, tc := range testCases {
		code, ok := NameToIFType(tc.input)
		assert.Equal(t, tc.found, ok, "接口 %s 查找状态不匹配", tc.input)
		if tc.found {
			assert.Equal(t, tc.expected, code, "接口 %s 编码不匹配", tc.input)
		}
	}
}

func TestIFTypeToName(t *testing.T) {
	name, ok := IFTypeToName(IFTypeEthernet)
	assert.True(t, ok)
	assert.Equal(t, "Ethernet", name)

	nameVlan, okVlan := IFTypeToName(IFTypeVlanif)
	assert.True(t, okVlan)
	assert.Equal(t, "Vlanif", nameVlan)

	nameTrunk, okTrunk := IFTypeToName(IFTypeTrunk)
	assert.True(t, okTrunk)
	assert.Equal(t, "Trunk", nameTrunk)
}

func TestNormalizeWithAlias(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"Twenty-FiveGigE1/0/1", "25GE1/0/1"},
		{"TwentyFiveGigE1/0/2", "25GE1/0/2"},
		{"FortyGigE1/0/1", "40GE1/0/1"},
		{"HundredGigE1/0/1", "100GE1/0/1"},
		{"Port-channel20", "Po20"},
		{"GigabitEthernet0/0/1", "GE0/0/1"},
		{"Vlan-interface50", "Vlanif50"},
		{"Bridge-Aggregation1", "BAgg1"},
	}

	for _, tc := range testCases {
		res := NormalizeWithAlias(tc.input)
		assert.Equal(t, tc.expected, res, "接口名别名归一化不符合预期: %s", tc.input)
	}
}
