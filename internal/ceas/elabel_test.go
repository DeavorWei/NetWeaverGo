package ceas

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func readSample(t *testing.T, filename string) string {
	path := filepath.Join("..", "..", "testdata", "ceas", filename)
	content, err := os.ReadFile(path)
	require.NoError(t, err, "读取样本文件失败: %s", path)
	return string(content)
}

func TestParseELabel_6ProductFamilies(t *testing.T) {
	// 1. CE 数据中心交换机
	t.Run("CE_DataCenter", func(t *testing.T) {
		raw := readSample(t, "ce_elabel.txt")
		tree := ParseELabel(raw)
		require.NotNil(t, tree)
		assert.Equal(t, "2102352BBR10J8000001", tree.ChassisESN)
		assert.NotEmpty(t, tree.AllNodes)

		// 检查各类节点
		hasFrame := false
		hasSlot := false
		hasDaughter := false
		hasPort := false
		hasPower := false
		hasFan := false

		for _, n := range tree.AllNodes {
			switch n.Type {
			case "frame":
				hasFrame = true
			case "slot":
				hasSlot = true
			case "daughterboard":
				hasDaughter = true
			case "port":
				hasPort = true
			case "power":
				hasPower = true
			case "fanframe":
				hasFan = true
			}
		}
		assert.True(t, hasFrame)
		assert.True(t, hasSlot)
		assert.True(t, hasDaughter)
		assert.True(t, hasPort)
		assert.True(t, hasPower)
		assert.True(t, hasFan)
	})

	// 2. S 园区交换机（含 handle_extra_properties 多段属性拆分）
	t.Run("S_Campus_ExtraProperties", func(t *testing.T) {
		raw := readSample(t, "sw_elabel.txt")
		tree := ParseELabel(raw)
		require.NotNil(t, tree)

		// 验证 Sub_Board_1 成功被拆分生成独立的子板节点
		hasSubBoard := false
		for _, n := range tree.AllNodes {
			if n.Type == "daughterboard" && n.Item == "02353HJJ" {
				hasSubBoard = true
				assert.Equal(t, "ES5D21X08S00", n.BoardType)
				assert.Equal(t, "2102353HJJ10K5000002", n.BarCode)
				assert.Equal(t, "0", n.Slot) // 归属于 Slot 0
			}
		}
		assert.True(t, hasSubBoard, "必须成功将块内多段属性拆分为独立子板节点")
	})

	// 3. AR 系列路由器
	t.Run("AR_Router", func(t *testing.T) {
		raw := readSample(t, "ar_elabel.txt")
		tree := ParseELabel(raw)
		require.NotNil(t, tree)
		assert.Equal(t, "2102311KLT10L3000001", tree.ChassisESN)

		hasMotherboard := false
		hasDaughterboard := false
		for _, n := range tree.AllNodes {
			if n.Type == "motherboard" {
				hasMotherboard = true
			}
			if n.Type == "daughterboard" && n.Item == "02312HQQ" {
				hasDaughterboard = true
			}
		}
		assert.True(t, hasMotherboard)
		assert.True(t, hasDaughterboard)
	})

	// 4. WLAN 控制器
	t.Run("WLAN_AC", func(t *testing.T) {
		raw := readSample(t, "wlan_elabel.txt")
		tree := ParseELabel(raw)
		require.NotNil(t, tree)
		assert.NotEmpty(t, tree.AllNodes)
		// 黄金断言：Unit_1 (slot) 和 Main_Board (mainboard)
		hasUnit := false
		hasMain := false
		hasPort := false
		for _, n := range tree.AllNodes {
			if n.Type == "slot" && n.Slot == "1" {
				hasUnit = true
				assert.Equal(t, "AirEngine9700-M", n.BoardType)
				assert.Equal(t, "02353WLAN", n.Item)
			}
			if n.Type == "mainboard" {
				hasMain = true
				assert.Equal(t, "AC-MAIN-BD", n.BoardType)
				assert.Equal(t, "02353WMB", n.Item)
			}
			if n.Type == "port" {
				hasPort = true
				assert.Equal(t, "02353WPT", n.Item)
			}
		}
		assert.True(t, hasUnit)
		assert.True(t, hasMain)
		assert.True(t, hasPort)
	})

	// 5. FW 防火墙
	t.Run("FW_Firewall", func(t *testing.T) {
		raw := readSample(t, "fw_elabel.txt")
		tree := ParseELabel(raw)
		require.NotNil(t, tree)
		assert.Equal(t, "2102354FW10N2000001", tree.ChassisESN)

		hasSlot1 := false
		hasPower := false
		hasFan := false
		for _, n := range tree.AllNodes {
			if n.Type == "slot" && n.Slot == "1" {
				hasSlot1 = true
				assert.Equal(t, "SPU-100", n.BoardType)
				assert.Equal(t, "02354FW2", n.Item)
				assert.Equal(t, "1/0", n.Path)
			}
			if n.Type == "power" {
				hasPower = true
				assert.Equal(t, "PWR-600W-DC", n.BoardType)
			}
			if n.Type == "fanframe" {
				hasFan = true
				assert.Equal(t, "FAN-USG-60", n.BoardType)
			}
		}
		assert.True(t, hasSlot1)
		assert.True(t, hasPower)
		assert.True(t, hasFan)
	})

	// 6. Route 核心路由器
	t.Run("Route_CoreNE", func(t *testing.T) {
		raw := readSample(t, "route_elabel.txt")
		tree := ParseELabel(raw)
		require.NotNil(t, tree)
		assert.Equal(t, "2103031AAA10P7000001", tree.ChassisESN)

		hasOfc := false
		hasCard := false
		var cardNode *Node
		for _, n := range tree.AllNodes {
			if n.Type == "ofccard" {
				hasOfc = true
			}
			if n.Type == "card" {
				hasCard = true
				cardNode = n
			}
		}
		assert.True(t, hasOfc)
		assert.True(t, hasCard)
		require.NotNil(t, cardNode)
		// 黄金断言：Slot_2 Card_1 槽号提取为 2，三段式路径为 2/1
		assert.Equal(t, "2", cardNode.Slot)
		assert.Equal(t, "2/1", cardNode.Path)
		assert.Equal(t, "CR52-P10-FP", cardNode.BoardType)
		assert.Equal(t, "03032CR5", cardNode.Item)
	})
}

// TestParseELabel_SlotCardHierarchyAndBoundaryReset 严格测试 Major 3 与 Minor 8：
// 1. [Slot_2 Card_1] 提取 slot=2，path=2/1
// 2. 后续节点不被卡号污染
// 3. 跨机框边界 [BackPlane_2] 重置 lastSlot 上下文
func TestParseELabel_SlotCardHierarchyAndBoundaryReset(t *testing.T) {
	raw := `
[Slot_2 Card_1]
/$[ArchivesInfo Version]
/$ArchivesInfoVersion=3.0
[Board Properties]
BoardType=CR52-P10-FP
BarCode=2103032CR510P7000004
Item=03032CR5
Description=Flexible PIC 10x10GE

[Slot_3]
/$[ArchivesInfo Version]
/$ArchivesInfoVersion=3.0
[Board Properties]
BoardType=LPUF-100
BarCode=2103032BBB10P7000008
Item=03032BBB
Description=Line Processing Unit 100G

[BackPlane_2]
/$[ArchivesInfo Version]
/$ArchivesInfoVersion=3.0
[Board Properties]
BoardType=NE40E-Chassis-2
BarCode=2103031AAA10P7000009
Item=03031AAA
Description=Chassis 2

[Port_1]
/$[ArchivesInfo Version]
/$ArchivesInfoVersion=3.0
[Port Properties]
BarCode=2103031AAA10P7000010
Item=03031PRT
Description=GigabitEthernet Port 1
`
	tree := ParseELabel(raw)
	require.NotNil(t, tree)

	nodeMap := make(map[string]*Node)
	for _, n := range tree.AllNodes {
		nodeMap[n.Name] = n
	}

	// 1. 验证 [Slot_2 Card_1]
	cardNode, ok := nodeMap["Slot_2 Card_1"]
	require.True(t, ok, "必须解析出 Slot_2 Card_1 节点")
	assert.Equal(t, "card", cardNode.Type)
	assert.Equal(t, "2", cardNode.Slot, "Slot 必须准确提取为 2，而非卡号 1")
	assert.Equal(t, "2/1", cardNode.Path, "三段式路径必须为 2/1")

	// 2. 验证后续 [Slot_3] 槽位不受前面 card 的卡号 1 污染
	slot3Node, ok := nodeMap["Slot_3"]
	require.True(t, ok, "必须解析出 Slot_3 节点")
	assert.Equal(t, "slot", slot3Node.Type)
	assert.Equal(t, "3", slot3Node.Slot)
	assert.Equal(t, "3/0", slot3Node.Path)

	// 3. 验证机框边界重置：遇到 [BackPlane_2] 重置 lastSlot
	frame2Node, ok := nodeMap["BackPlane_2"]
	require.True(t, ok, "必须解析出 BackPlane_2 机框")
	assert.Equal(t, "frame", frame2Node.Type)

	// 验证紧随 BackPlane_2 后面的 Port_1 不会错误继承 Slot 3 或 Slot 2
	portNode, ok := nodeMap["Port_1"]
	require.True(t, ok, "必须解析出 Port_1 节点")
	assert.Equal(t, "port", portNode.Type)
	assert.Equal(t, "", portNode.Slot, "机框重置后无 slot 继承，slot 应为空")
	assert.Equal(t, "0/0/1", portNode.Path, "无 slot 继承时端口路径应为 0/0/1")
}
