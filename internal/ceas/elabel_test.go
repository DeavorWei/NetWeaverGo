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
	})

	// 5. FW 防火墙
	t.Run("FW_Firewall", func(t *testing.T) {
		raw := readSample(t, "fw_elabel.txt")
		tree := ParseELabel(raw)
		require.NotNil(t, tree)
		assert.Equal(t, "2102354FW10N2000001", tree.ChassisESN)
	})

	// 6. Route 核心路由器
	t.Run("Route_CoreNE", func(t *testing.T) {
		raw := readSample(t, "route_elabel.txt")
		tree := ParseELabel(raw)
		require.NotNil(t, tree)
		assert.Equal(t, "2103031AAA10P7000001", tree.ChassisESN)

		hasOfc := false
		hasCard := false
		for _, n := range tree.AllNodes {
			if n.Type == "ofccard" {
				hasOfc = true
			}
			if n.Type == "card" {
				hasCard = true
			}
		}
		assert.True(t, hasOfc)
		assert.True(t, hasCard)
	})
}
