package bizcompare

// RegisterBuiltinScenes 注册首期支持的 3 域场景种子数据
func (m *SceneManager) RegisterBuiltinScenes() {
	// 1. S 系列园区交换机域 (裁剪重命令，突出二层与基础三层)
	m.Register(&SceneDefinition{
		ID:          "s_campus_switch",
		Domain:      "S",
		Name:        "S系列园区交换机标准比对",
		Description: "覆盖接口物理与协议状态、MAC地址表、ARP表、VLAN及STP生成树，屏蔽海量BGP路由",
		Commands: []SceneCommand{
			{CommandKey: "interface_brief", Command: "display interface brief", Category: "interface"},
			{CommandKey: "ip_interface_brief", Command: "display ip interface brief", Category: "interface"},
			{CommandKey: "mac_address", Command: "display mac-address", Category: "mac"},
			{CommandKey: "arp_table", Command: "display arp all", Category: "arp"},
			{CommandKey: "vlan_summary", Command: "display vlan summary", Category: "vlan"},
			{CommandKey: "stp_brief", Command: "display stp brief", Category: "stp"},
		},
	})

	// 2. NE-SR 系列路由器域 (突出路由对等体与关键三层协议)
	m.Register(&SceneDefinition{
		ID:          "ne_core_router",
		Domain:      "NE-SR",
		Name:        "NE/SR核心路由器业务比对",
		Description: "覆盖接口状态、BGP对等体状态、OSPF/ISIS邻居状态及路由统计",
		Commands: []SceneCommand{
			{CommandKey: "interface_brief", Command: "display interface brief", Category: "interface"},
			{CommandKey: "ip_interface_brief", Command: "display ip interface brief", Category: "interface"},
			{CommandKey: "bgp_peer", Command: "display bgp peer", Category: "route"},
			{CommandKey: "ospf_peer", Command: "display ospf peer brief", Category: "route"},
			{CommandKey: "isis_peer", Command: "display isis peer", Category: "route"},
			{CommandKey: "route_stat", Command: "display ip routing-table statistics", Category: "route"},
			{CommandKey: "arp_table", Command: "display arp all", Category: "arp"},
		},
	})

	// 3. CE 系列数据中心交换机域 (突出 EVPN/VXLAN 与 Fabric 拓扑)
	m.Register(&SceneDefinition{
		ID:          "ce_dc_switch",
		Domain:      "CE",
		Name:        "CE数据中心交换机业务比对",
		Description: "覆盖接口状态、EVPN对等体、VXLAN VNI状态、LLDP拓扑邻居及MAC/ARP表",
		Commands: []SceneCommand{
			{CommandKey: "interface_brief", Command: "display interface brief", Category: "interface"},
			{CommandKey: "ip_interface_brief", Command: "display ip interface brief", Category: "interface"},
			{CommandKey: "bgp_evpn", Command: "display bgp evpn peer", Category: "route"},
			{CommandKey: "vxlan_vni", Command: "display vxlan vni", Category: "vxlan"},
			{CommandKey: "lldp_neighbor", Command: "display lldp neighbor brief", Category: "lldp"},
			{CommandKey: "mac_address", Command: "display mac-address", Category: "mac"},
			{CommandKey: "arp_table", Command: "display arp all", Category: "arp"},
		},
	})
}
