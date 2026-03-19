package parser

import (
	"fmt"
	"strings"

	"github.com/NetWeaverGo/core/internal/normalize"
)

// HuaweiMapper 华为设备结果映射器
type HuaweiMapper struct{}

// NewHuaweiMapper 创建华为映射器
func NewHuaweiMapper() *HuaweiMapper {
	return &HuaweiMapper{}
}

// ToDeviceInfo 将version解析结果映射为设备身份信息
func (m *HuaweiMapper) ToDeviceInfo(rows []map[string]string) (*DeviceIdentity, error) {
	if len(rows) == 0 {
		return nil, fmt.Errorf("no version data found")
	}

	// 合并所有行数据
	data := make(map[string]string)
	for _, row := range rows {
		for k, v := range row {
			if v != "" {
				data[k] = v
			}
		}
	}

	identity := &DeviceIdentity{
		Vendor:       "huawei",
		Model:        data["model"],
		SerialNumber: data["serial_number"],
		Version:      data["version"],
		Hostname:     data["hostname"],
	}

	return identity, nil
}

// ToInterfaces 将接口解析结果映射为接口信息
func (m *HuaweiMapper) ToInterfaces(rows []map[string]string) ([]InterfaceFact, error) {
	var interfaces []InterfaceFact

	for _, row := range rows {
		ifName := normalize.NormalizeInterfaceName(row["interface"])
		if ifName == "" {
			continue
		}

		iface := InterfaceFact{
			Name:        ifName,
			Status:      normalizeStatus(row["phy"]),
			Protocol:    normalizeProtocol(row["protocol"]),
			Description: row["description"],
		}

		// 解析速率和双工
		if row["speed"] != "" {
			iface.Speed = row["speed"]
		}
		if row["duplex"] != "" {
			iface.Duplex = row["duplex"]
		}

		// 解析MAC地址和IP地址（来自interface_detail模板）
		if mac := normalize.NormalizeMAC(row["mac"]); mac != "" {
			iface.MACAddress = mac
		}
		if ip := row["ip"]; ip != "" {
			iface.IPAddress = ip
		}

		interfaces = append(interfaces, iface)
	}

	return interfaces, nil
}

// ToLLDP 将LLDP解析结果映射为邻居信息
func (m *HuaweiMapper) ToLLDP(rows []map[string]string) ([]LLDPFact, error) {
	var neighbors []LLDPFact

	// 临时存储，用于聚合同一接口的邻居信息
	type lldpInfo struct {
		LocalInterface  string
		NeighborName    string
		NeighborChassis string
		NeighborPort    string
		NeighborIP      string
		NeighborDesc    string
	}

	infoMap := make(map[string]*lldpInfo)
	var order []string
	currentIf := ""

	for _, row := range rows {
		if localIf := row["local_if"]; localIf != "" {
			normalizedIf := normalize.NormalizeInterfaceName(localIf)
			if _, ok := infoMap[normalizedIf]; !ok {
				infoMap[normalizedIf] = &lldpInfo{LocalInterface: normalizedIf}
				order = append(order, normalizedIf)
			}
			currentIf = normalizedIf
		}

		if currentIf == "" {
			continue
		}

		info := infoMap[currentIf]

		if v := row["neighbor_name"]; v != "" {
			info.NeighborName = normalize.NormalizeDeviceName(v)
		}
		if v := row["neighbor_port"]; v != "" {
			info.NeighborPort = normalize.NormalizeLLDPRemotePort(v)
		}
		if v := row["chassis_id"]; v != "" {
			info.NeighborChassis = v
		}
		if v := row["mgmt_ip"]; v != "" {
			info.NeighborIP = v
		}
	}

	// 转换为结果
	for _, ifName := range order {
		info := infoMap[ifName]
		if info.NeighborName == "" && info.NeighborChassis == "" {
			continue
		}
		neighbors = append(neighbors, LLDPFact{
			LocalInterface:  info.LocalInterface,
			NeighborName:    info.NeighborName,
			NeighborChassis: info.NeighborChassis,
			NeighborPort:    info.NeighborPort,
			NeighborIP:      info.NeighborIP,
			NeighborDesc:    info.NeighborDesc,
		})
	}

	return neighbors, nil
}

// ToFDB 将MAC表解析结果映射为FDB信息
func (m *HuaweiMapper) ToFDB(rows []map[string]string) ([]FDBFact, error) {
	var entries []FDBFact

	for _, row := range rows {
		mac := normalize.NormalizeMAC(row["mac"])
		if mac == "" {
			continue
		}

		vlan := 0
		if v := row["vlan"]; v != "" {
			fmt.Sscanf(v, "%d", &vlan)
		}

		ifName := normalize.NormalizeInterfaceName(row["interface"])
		if ifName == "" {
			continue
		}

		entry := FDBFact{
			MACAddress: mac,
			VLAN:       vlan,
			Interface:  ifName,
			Type:       normalizeFDBType(row["type"]),
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

// ToARP 将ARP解析结果映射为ARP信息
func (m *HuaweiMapper) ToARP(rows []map[string]string) ([]ARPFact, error) {
	var entries []ARPFact

	for _, row := range rows {
		ip := row["ip"]
		if ip == "" {
			continue
		}

		mac := normalize.NormalizeMAC(row["mac"])
		ifName := normalize.NormalizeInterfaceName(row["interface"])

		entry := ARPFact{
			IPAddress:  ip,
			MACAddress: mac,
			Interface:  ifName,
			Type:       row["type"],
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

// ToAggregate 将聚合口解析结果映射为聚合信息
func (m *HuaweiMapper) ToAggregate(rows []map[string]string) ([]AggregateFact, error) {
	var aggregates []AggregateFact

	// 临时存储聚合组
	aggMap := make(map[string]*AggregateFact)

	var currentAgg string

	for _, row := range rows {
		// 检查是否是聚合组定义行
		if trunkID := row["trunk_id"]; trunkID != "" {
			aggName := normalize.NormalizeAggregateName("Eth-Trunk" + trunkID)
			if _, ok := aggMap[aggName]; !ok {
				aggMap[aggName] = &AggregateFact{
					AggregateName: aggName,
					Mode:          "static", // 华为默认static
					MemberPorts:   []string{},
				}
			}
			currentAgg = aggName
			continue
		}

		// 成员端口行
		if ifType := row["if_type"]; ifType != "" && currentAgg != "" {
			ifNum := row["if_num"]
			memberPort := normalize.NormalizeInterfaceName(ifType + ifNum)
			aggMap[currentAgg].MemberPorts = append(aggMap[currentAgg].MemberPorts, memberPort)
		}
	}

	// 转换为结果
	for _, agg := range aggMap {
		aggregates = append(aggregates, *agg)
	}

	return aggregates, nil
}

// H3CMapper H3C设备结果映射器
type H3CMapper struct {
	HuaweiMapper // 继承华为实现，大部分逻辑相同
}

// NewH3CMapper 创建H3C映射器
func NewH3CMapper() *H3CMapper {
	return &H3CMapper{
		HuaweiMapper: *NewHuaweiMapper(),
	}
}

// ToDeviceInfo 重写设备信息映射
func (m *H3CMapper) ToDeviceInfo(rows []map[string]string) (*DeviceIdentity, error) {
	identity, err := m.HuaweiMapper.ToDeviceInfo(rows)
	if err != nil {
		return nil, err
	}
	identity.Vendor = "h3c"
	return identity, nil
}

// CiscoMapper Cisco设备结果映射器
type CiscoMapper struct{}

// NewCiscoMapper 创建Cisco映射器
func NewCiscoMapper() *CiscoMapper {
	return &CiscoMapper{}
}

// ToDeviceInfo 将version解析结果映射为设备身份信息
func (m *CiscoMapper) ToDeviceInfo(rows []map[string]string) (*DeviceIdentity, error) {
	if len(rows) == 0 {
		return nil, fmt.Errorf("no version data found")
	}

	data := make(map[string]string)
	for _, row := range rows {
		for k, v := range row {
			if v != "" {
				data[k] = v
			}
		}
	}

	return &DeviceIdentity{
		Vendor:       "cisco",
		Model:        data["model"],
		SerialNumber: data["serial_number"],
		Version:      data["version"],
	}, nil
}

// ToInterfaces 将接口解析结果映射为接口信息
func (m *CiscoMapper) ToInterfaces(rows []map[string]string) ([]InterfaceFact, error) {
	var interfaces []InterfaceFact

	for _, row := range rows {
		ifName := normalize.NormalizeInterfaceName(row["interface"])
		if ifName == "" {
			continue
		}

		iface := InterfaceFact{
			Name:    ifName,
			Status:  normalizeCiscoStatus(row["status"]),
			Duplex:  row["duplex"],
			VLAN:    row["vlan"],
			IsTrunk: strings.Contains(row["vlan"], "trunk"),
		}

		// 解析MAC地址和IP地址（来自interface_detail模板）
		if mac := normalize.NormalizeMAC(row["mac"]); mac != "" {
			iface.MACAddress = mac
		}
		if ip := row["ip"]; ip != "" {
			iface.IPAddress = ip
		}

		interfaces = append(interfaces, iface)
	}

	return interfaces, nil
}

// ToLLDP 将LLDP解析结果映射为邻居信息
func (m *CiscoMapper) ToLLDP(rows []map[string]string) ([]LLDPFact, error) {
	var neighbors []LLDPFact

	type lldpInfo struct {
		LocalInterface  string
		NeighborName    string
		NeighborChassis string
		NeighborPort    string
	}

	infoMap := make(map[string]*lldpInfo)
	var order []string
	currentIf := ""

	for _, row := range rows {
		if localIf := row["local_if"]; localIf != "" {
			normalizedIf := normalize.NormalizeInterfaceName(localIf)
			if _, ok := infoMap[normalizedIf]; !ok {
				infoMap[normalizedIf] = &lldpInfo{LocalInterface: normalizedIf}
				order = append(order, normalizedIf)
			}
			currentIf = normalizedIf
		}

		if currentIf == "" {
			continue
		}

		info := infoMap[currentIf]

		if v := row["neighbor_name"]; v != "" {
			info.NeighborName = normalize.NormalizeDeviceName(v)
		}
		if v := row["neighbor_port"]; v != "" {
			info.NeighborPort = normalize.NormalizeLLDPRemotePort(v)
		}
		if v := row["chassis_id"]; v != "" {
			info.NeighborChassis = v
		}
	}

	for _, ifName := range order {
		info := infoMap[ifName]
		if info.NeighborName == "" && info.NeighborChassis == "" {
			continue
		}
		neighbors = append(neighbors, LLDPFact{
			LocalInterface:  info.LocalInterface,
			NeighborName:    info.NeighborName,
			NeighborChassis: info.NeighborChassis,
			NeighborPort:    info.NeighborPort,
		})
	}

	return neighbors, nil
}

// ToFDB 将MAC表解析结果映射为FDB信息
func (m *CiscoMapper) ToFDB(rows []map[string]string) ([]FDBFact, error) {
	var entries []FDBFact

	for _, row := range rows {
		mac := normalize.NormalizeMAC(row["mac"])
		if mac == "" {
			continue
		}

		vlan := 0
		if v := row["vlan"]; v != "" && v != "All" {
			fmt.Sscanf(v, "%d", &vlan)
		}

		ifName := normalize.NormalizeInterfaceName(row["interface"])

		entry := FDBFact{
			MACAddress: mac,
			VLAN:       vlan,
			Interface:  ifName,
			Type:       normalizeFDBType(row["type"]),
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

// ToARP 将ARP解析结果映射为ARP信息
func (m *CiscoMapper) ToARP(rows []map[string]string) ([]ARPFact, error) {
	// Cisco ARP格式与华为类似
	var entries []ARPFact

	for _, row := range rows {
		ip := row["ip"]
		if ip == "" {
			continue
		}

		mac := normalize.NormalizeMAC(row["mac"])
		ifName := normalize.NormalizeInterfaceName(row["interface"])

		entry := ARPFact{
			IPAddress:  ip,
			MACAddress: mac,
			Interface:  ifName,
			Type:       row["type"],
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

// ToAggregate 将聚合口解析结果映射为聚合信息
func (m *CiscoMapper) ToAggregate(rows []map[string]string) ([]AggregateFact, error) {
	var aggregates []AggregateFact

	// Cisco EtherChannel格式与华为有所不同
	aggMap := make(map[string]*AggregateFact)

	for _, row := range rows {
		if po := row["port_channel"]; po != "" {
			aggName := "Po" + po
			if _, ok := aggMap[aggName]; !ok {
				aggMap[aggName] = &AggregateFact{
					AggregateName: aggName,
					Mode:          row["protocol"], // PAgP, LACP, 或 ON
					MemberPorts:   []string{},
				}
			}

			if member := row["member"]; member != "" {
				memberPort := normalize.NormalizeInterfaceName(member)
				aggMap[aggName].MemberPorts = append(aggMap[aggName].MemberPorts, memberPort)
			}
		}
	}

	for _, agg := range aggMap {
		aggregates = append(aggregates, *agg)
	}

	return aggregates, nil
}

// 辅助函数

func normalizeStatus(status string) string {
	status = strings.ToLower(strings.TrimSpace(status))
	switch status {
	case "up", "up(up)":
		return "up"
	case "down", "down(down)", "down(*)":
		return "down"
	case "administratively down", "*down":
		return "admin_down"
	default:
		return status
	}
}

func normalizeProtocol(protocol string) string {
	protocol = strings.ToLower(strings.TrimSpace(protocol))
	switch protocol {
	case "up", "up(up)":
		return "up"
	case "down", "down(down)":
		return "down"
	default:
		return protocol
	}
}

func normalizeFDBType(t string) string {
	t = strings.ToLower(strings.TrimSpace(t))
	switch t {
	case "dynamic", "learned", "d":
		return "dynamic"
	case "static", "s":
		return "static"
	default:
		return t
	}
}

func normalizeCiscoStatus(status string) string {
	status = strings.ToLower(strings.TrimSpace(status))
	switch status {
	case "connected", "up":
		return "up"
	case "notconnect", "down":
		return "down"
	case "disabled", "err-disabled":
		return "admin_down"
	default:
		return status
	}
}

// GetMapper 根据厂商获取对应的映射器
func GetMapper(vendor string) ResultMapper {
	switch vendor {
	case "h3c":
		return NewH3CMapper()
	case "cisco":
		return NewCiscoMapper()
	case "huawei":
		return NewHuaweiMapper()
	default:
		return NewHuaweiMapper()
	}
}
