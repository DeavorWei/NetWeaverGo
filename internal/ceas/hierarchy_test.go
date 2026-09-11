package ceas

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateNodeID(t *testing.T) {
	assert.Equal(t, "0", GenerateNodeID("", 0))
	assert.Equal(t, "1", GenerateNodeID("", 1))
	assert.Equal(t, "0_1", GenerateNodeID("0", 1))
	assert.Equal(t, "0_1_2", GenerateNodeID("0_1", 2))
}

func TestNormalizeSlot(t *testing.T) {
	assert.Equal(t, "1", NormalizeSlot("Slot_1"))
	assert.Equal(t, "2", NormalizeSlot("Slot2"))
	assert.Equal(t, "3", NormalizeSlot("Unit_3"))
	assert.Equal(t, "1/3", NormalizeSlot("Slot_1/3"))
	assert.Equal(t, "0", NormalizeSlot("0"))
}

func TestFormat3SegmentPath(t *testing.T) {
	tests := []struct {
		name       string
		nodeType   string
		nodeName   string
		slot       string
		parentPath string
		expected   string
	}{
		{
			name:       "机框",
			nodeType:   "frame",
			nodeName:   "BackPlane_1",
			slot:       "",
			parentPath: "",
			expected:   "1",
		},
		{
			name:       "槽位_1",
			nodeType:   "slot",
			nodeName:   "Slot_1",
			slot:       "1",
			parentPath: "1",
			expected:   "1/0",
		},
		{
			name:       "子卡_1",
			nodeType:   "daughterboard",
			nodeName:   "Daughter_Board_1/2",
			slot:       "1",
			parentPath: "1/0",
			expected:   "1/2",
		},
		{
			name:       "端口_1",
			nodeType:   "port",
			nodeName:   "Port_3",
			slot:       "1",
			parentPath: "1/2",
			expected:   "1/2/3",
		},
		{
			name:       "主板挂载端口",
			nodeType:   "port",
			nodeName:   "Port_4",
			slot:       "1",
			parentPath: "1/0",
			expected:   "1/0/4",
		},
		{
			name:       "电源框",
			nodeType:   "power",
			nodeName:   "PowerFrame_1",
			slot:       "",
			parentPath: "1",
			expected:   "power/1",
		},
		{
			name:       "风扇框",
			nodeType:   "fanframe",
			nodeName:   "FanSlot_2",
			slot:       "",
			parentPath: "1",
			expected:   "fanframe/2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := Format3SegmentPath(tt.nodeType, tt.nodeName, tt.slot, tt.parentPath)
			assert.Equal(t, tt.expected, path)
		})
	}
}

func TestBuildHardwareTree_And_ConvertVO(t *testing.T) {
	nodes := []*Node{
		{
			ID:       "0",
			ParentID: "",
			Level:    1,
			Type:     "frame",
			Name:     "BackPlane_1",
			Path:     "1",
			BarCode:  "2102352BBR10J8000001",
		},
		{
			ID:       "0_1",
			ParentID: "0",
			Level:    2,
			Type:     "slot",
			Name:     "Slot_1",
			Path:     "1/0",
			Slot:     "1",
		},
		{
			ID:       "0_1_1",
			ParentID: "0_1",
			Level:    3,
			Type:     "daughterboard",
			Name:     "Daughter_Board_1/1",
			Path:     "1/1",
			Slot:     "1",
			Item:     "02311UHE",
		},
	}

	tree := BuildHardwareTree("10.0.0.1", "2102352BBR10J8000001", nodes)
	assert.NotNil(t, tree)
	assert.Equal(t, "10.0.0.1", tree.DeviceIP)
	assert.Equal(t, "2102352BBR10J8000001", tree.ChassisESN)
	assert.Equal(t, 1, len(tree.Roots))
	assert.Equal(t, 1, len(tree.Roots[0].Children))
	assert.Equal(t, 1, len(tree.Roots[0].Children[0].Children))

	vo := ConvertTreeToVO(tree)
	assert.NotNil(t, vo)
	assert.Equal(t, 3, vo.TotalNodes)
	assert.Equal(t, 1, len(vo.Roots))
	assert.Equal(t, 1, len(vo.Roots[0].Children))
}

func TestFilterTreeByWhitelist(t *testing.T) {
	nodes := []*Node{
		{ID: "0", Type: "frame", Name: "Frame"},
		{ID: "0_1", ParentID: "0", Type: "slot", Name: "Slot_1", Slot: "1"},
		{ID: "0_1_1", ParentID: "0_1", Type: "card", Name: "Card_1", Item: "03030303"},
		{ID: "0_2", ParentID: "0", Type: "slot", Name: "Slot_2", Slot: "2"},
		{ID: "0_2_1", ParentID: "0_2", Type: "card", Name: "Card_2", Item: "99999999"},
	}
	tree := BuildHardwareTree("10.0.0.1", "ESN123", nodes)

	// 仅保留 Item 为 03030303 的卡及其祖先路径
	filtered := FilterTreeByWhitelist(tree, []string{"03030303"}, nil)
	assert.NotNil(t, filtered)
	assert.Equal(t, 1, len(filtered.Roots))
	assert.Equal(t, "Frame", filtered.Roots[0].Name)
	assert.Equal(t, 1, len(filtered.Roots[0].Children))
	assert.Equal(t, "Slot_1", filtered.Roots[0].Children[0].Name)
	assert.Equal(t, 1, len(filtered.Roots[0].Children[0].Children))
	assert.Equal(t, "Card_1", filtered.Roots[0].Children[0].Children[0].Name)
	assert.Equal(t, 3, len(filtered.AllNodes))
}
