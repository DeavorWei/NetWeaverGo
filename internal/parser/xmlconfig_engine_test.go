package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestXmlConfigEngine_Load(t *testing.T) {
	engine := GetDefaultXmlConfigEngine()
	require.NotNil(t, engine)

	engine.mu.RLock()
	configCount := len(engine.configs)
	itemCount := len(engine.parseItems)
	engine.mu.RUnlock()

	t.Logf("Loaded %d XML configs, %d parseitem vendors", configCount, itemCount)
	assert.GreaterOrEqual(t, configCount, 30, "至少加载 30 份 XML 配置")
	assert.GreaterOrEqual(t, itemCount, 2, "至少加载 2 个厂商的 parseitem 映射")
}

// P0-7：不得跨厂商兜底命中（A 厂规则不得套用到 B 厂设备）
func TestResolveConfig_NoCrossVendorFallback(t *testing.T) {
	engine := GetDefaultXmlConfigEngine()

	// huawei 存在 "display arp" 规则；cisco 不存在对应 vendor 目录，必须未命中
	if _, found := engine.ResolveConfig("cisco", "display arp"); found {
		t.Fatal("cisco 设备不得命中 huawei 的 XML 解析规则（禁止跨厂商兜底）")
	}
	// 同厂商命令必须命中，证明拒绝兜底不影响正常解析
	if _, found := engine.ResolveConfig("huawei", "display arp"); !found {
		t.Fatal("huawei/display arp 应正常命中")
	}
}

// P0-7：最长命令键优先，且多次匹配结果确定（消除 map 迭代随机性）
func TestResolveConfig_LongestMatchDeterministic(t *testing.T) {
	engine := GetDefaultXmlConfigEngine()

	// huawei 同时存在 "display interface" 与 "display interface brief"，
	// 传入带后缀的命令时应稳定命中最长键（display interface brief）
	first, ok := engine.ResolveConfig("huawei", "display interface brief 10GE1/0/1")
	if !ok {
		t.Fatal("应命中最长前缀规则")
	}
	for i := 0; i < 100; i++ {
		cfg, ok := engine.ResolveConfig("huawei", "display interface brief 10GE1/0/1")
		if !ok {
			t.Fatalf("第 %d 次匹配失败", i)
		}
		if cfg != first {
			t.Fatalf("第 %d 次匹配结果与首次不一致（存在非确定性）", i)
		}
	}
}

// 6 模型 Golden 测试
func TestXmlConfigEngine_Golden_6Models(t *testing.T) {
	engine := GetDefaultXmlConfigEngine()
	require.NotNil(t, engine)

	// 1. MAC 模型 Golden
	t.Run("Golden_MAC", func(t *testing.T) {
		echo := `
MAC Address    VLAN/VSI/BD   Learned-From        Type                Age
00e0-fc00-0001 100/-/-       10GE1/0/1           dynamic      4294367295
00e0-fc00-0002 200/-/-       10GE1/0/2           static                -
`
		rows, err := engine.ParseWithVendor("huawei", "display mac-address", echo)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(rows), 2)
		assert.Equal(t, "00e0-fc00-0001", rows[0]["mac"])
		assert.Equal(t, "100", rows[0]["vlanId"])
		assert.Equal(t, "10GE1/0/1", rows[0]["ifName"])
		assert.Equal(t, "dynamic", rows[0]["type"])
	})

	// 2. ARP 模型 Golden
	t.Run("Golden_ARP", func(t *testing.T) {
		echo := `
IP ADDRESS      MAC ADDRESS    EXP(M) TYPE/VLAN       INTERFACE        VPN-INSTANCE
192.168.1.1     00e0-fc11-2233        I               MEth0/0/0        MGMT
192.168.1.2     00e0-fc44-5566   19   D               MEth0/0/0        MGMT
`
		rows, err := engine.ParseWithVendor("huawei", "display arp", echo)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(rows), 2)
		assert.Equal(t, "192.168.1.1", rows[0]["entryIpAddr"])
		assert.Equal(t, "00e0-fc11-2233", rows[0]["entryMac"])
		assert.Equal(t, "I", rows[0]["entryType"])
	})

	// 3. ESN / 硬件资产 Golden
	t.Run("Golden_Hardware_ESN", func(t *testing.T) {
		echo := `
<YWM-ASW-CE6857-12>display esn
 ESN of slot 1: 6T2380029201
`
		rows, err := engine.ParseWithVendor("huawei", "display esn", echo)
		require.NoError(t, err)
		require.NotEmpty(t, rows)
		assert.Equal(t, "6T2380029201", rows[0]["esn"])
	})

	// 4. LLDP 邻居 Golden
	t.Run("Golden_LLDP", func(t *testing.T) {
		echo := `
Local Interface         Exptime(s) Neighbor Interface            Neighbor Device
400GE1/1/5                    113  400GE6/0/19                   CSM-DSW-CE16808-01
400GE1/1/6                    103  400GE6/0/19                   CSM-DSW-CE16808-02
`
		rows, err := engine.ParseWithVendor("huawei", "display lldp neighbor brief", echo)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(rows), 2)
		assert.Equal(t, "400GE1/1/5", rows[0]["localPortName"])
		assert.Equal(t, "400GE6/0/19", rows[0]["remotePortId"])
		assert.Equal(t, "CSM-DSW-CE16808-01", rows[0]["remoteSystemName"])
	})

	// 5. Interface Brief Golden
	t.Run("Golden_Interface_Brief", func(t *testing.T) {
		echo := `
Interface                  PHY      Protocol  InUti OutUti   inErrors  outErrors
400GE1/1/1:1(200GE)        up       up        0.01%  0.01%          0          0
400GE1/1/1:2(200GE)        up       up        0.01%  0.01%          0          0
`
		rows, err := engine.ParseWithVendor("huawei", "display interface brief", echo)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(rows), 2)
		assert.Equal(t, "400GE1/1/1:1", rows[0]["interfaceName"])
		assert.Equal(t, "UP", rows[0]["ifPhyStatus"])
	})

	// 6. Optical Module / Transceiver Golden
	t.Run("Golden_Transceiver", func(t *testing.T) {
		cfg, found := engine.ResolveConfig("huawei", "display optical-module interface")
		assert.True(t, found, "应当能解析 optical-module 规则")
		assert.NotNil(t, cfg)
	})
}
