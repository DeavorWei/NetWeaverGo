package taskexec

import "testing"

// P0-9：发现源可选配置应正确过滤采集字段，并在缺失 LLDP/CDP 时产出 ARP 风险提示
func TestBuildCollectSteps_DiscoverySourcesFilterAndRiskNotice(t *testing.T) {
	c := NewTopologyTaskCompiler(nil)
	config := &TopologyTaskConfig{
		DiscoverySources: []string{"arp", "mac"},
		ResolvedCommands: []ResolvedTopologyCommand{
			{FieldKey: "lldp_neighbor", DisplayName: "LLDP 邻居", Command: "show lldp neighbors detail", Enabled: true},
			{FieldKey: "cdp_neighbor", DisplayName: "CDP 邻居", Command: "show cdp neighbors detail", Enabled: true},
			{FieldKey: "arp_all", DisplayName: "ARP 表", Command: "show ip arp", Enabled: true},
			{FieldKey: "version", DisplayName: "版本", Command: "show version", Enabled: true},
		},
	}

	steps := c.buildCollectSteps(config)
	keys := make(map[string]bool, len(steps))
	for _, s := range steps {
		keys[s.CommandKey] = true
	}

	if keys["lldp_neighbor"] || keys["cdp_neighbor"] {
		t.Fatal("发现源白名单（arp/mac）应过滤 LLDP/CDP 采集步骤")
	}
	if !keys["arp_all"] {
		t.Fatal("ARP 字段应在白名单内保留")
	}
	if !keys["version"] {
		t.Fatal("非发现类字段不应被发现源限制过滤")
	}
	if len(steps) == 0 {
		t.Fatal("应至少保留一个采集步骤")
	}
	if steps[0].Params["discoveryRiskNotice"] == "" {
		t.Fatal("缺少 LLDP/CDP 时应输出 ARP 推断风险提示")
	}

	// 含 LLDP 时不产生风险提示
	config.DiscoverySources = []string{"lldp", "arp"}
	steps = c.buildCollectSteps(config)
	keys = make(map[string]bool, len(steps))
	for _, s := range steps {
		keys[s.CommandKey] = true
	}
	if !keys["lldp_neighbor"] || keys["cdp_neighbor"] {
		t.Fatal("发现源为 lldp/arp 时应保留 LLDP、过滤 CDP")
	}
	if steps[0].Params["discoveryRiskNotice"] != "" {
		t.Fatal("包含 LLDP 时不应输出 ARP 风险提示")
	}
}

func TestNormalizeDiscoverySources(t *testing.T) {
	if normalizeDiscoverySources(nil) != nil {
		t.Fatal("空配置应表示不限制（返回 nil）")
	}
	if normalizeDiscoverySources([]string{"unknown"}) != nil {
		t.Fatal("非法值应被忽略，等效于不限制")
	}
	got := normalizeDiscoverySources([]string{"LLDP", " cdp ", "lldp"})
	if !got["lldp"] || !got["cdp"] || len(got) != 2 {
		t.Fatalf("发现源归一化结果不正确: %+v", got)
	}
}
