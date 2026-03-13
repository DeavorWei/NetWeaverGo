package ui

import (
	"encoding/csv"
	"errors"
	"fmt"
	"math/big"
	"net/netip"
	"strconv"
	"strings"
)

const (
	maxIPv4SubnetsToGenerate = 256
	absoluteIPv4SubnetsLimit = 65535
	maxIPv6SubnetsToGenerate = 256
	maxIPv6SubnetDiff        = 16
)

// NetworkRecord 通用标签值记录，供前端直接渲染。
type NetworkRecord struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// IPv4CalcRequest IPv4 计算请求。
type IPv4CalcRequest struct {
	IP                     string `json:"ip"`
	Mask                   string `json:"mask"`
	HostCount              string `json:"hostCount"`
	SubnetCount            string `json:"subnetCount"`
	ForceDisplayAllSubnets bool   `json:"forceDisplayAllSubnets"`
}

// IPv4SubnetItem IPv4 子网条目。
type IPv4SubnetItem struct {
	Index       int    `json:"index"`
	Network     string `json:"network"`
	Cidr        int    `json:"cidr"`
	FirstUsable string `json:"firstUsable"`
	LastUsable  string `json:"lastUsable"`
	Broadcast   string `json:"broadcast"`
	Mask        string `json:"mask"`
}

// IPv4CalcResult IPv4 计算结果。
type IPv4CalcResult struct {
	BaseError       string           `json:"baseError"`
	BaseRecords     []NetworkRecord  `json:"baseRecords"`
	SubnetError     string           `json:"subnetError"`
	SubnetWarning   string           `json:"subnetWarning"`
	ShowForceButton bool             `json:"showForceButton"`
	Subnets         []IPv4SubnetItem `json:"subnets"`
	TotalSubnets    int              `json:"totalSubnets"`
}

// IPv4SubnetCSVResult IPv4 子网 CSV 导出结果。
type IPv4SubnetCSVResult struct {
	FileName string `json:"fileName"`
	Content  string `json:"content"`
}

// IPv6CalcRequest IPv6 计算请求。
type IPv6CalcRequest struct {
	IP        string `json:"ip"`
	Prefix    string `json:"prefix"`
	CheckIP   string `json:"checkIp"`
	NewPrefix string `json:"newPrefix"`
}

// IPv6InclusionResult IPv6 包含关系检查结果。
type IPv6InclusionResult struct {
	IsIncluded bool   `json:"isIncluded"`
	Message    string `json:"message"`
}

// IPv6SubnetItem IPv6 子网条目。
type IPv6SubnetItem struct {
	Index      int    `json:"index"`
	Network    string `json:"network"`
	Cidr       int    `json:"cidr"`
	IsIncluded bool   `json:"isIncluded"`
}

// IPv6CalcResult IPv6 计算结果。
type IPv6CalcResult struct {
	BaseError      string               `json:"baseError"`
	BaseRecords    []NetworkRecord      `json:"baseRecords"`
	InclusionCheck *IPv6InclusionResult `json:"inclusionCheck,omitempty"`
	SubnetError    string               `json:"subnetError"`
	SubnetWarning  string               `json:"subnetWarning"`
	Subnets        []IPv6SubnetItem     `json:"subnets"`
	TotalSubnets   int                  `json:"totalSubnets"`
}

// CalculateIPv4 IPv4 网络信息与子网划分计算。
func (s *ForgeService) CalculateIPv4(req IPv4CalcRequest) *IPv4CalcResult {
	result := &IPv4CalcResult{
		BaseRecords: make([]NetworkRecord, 0),
		Subnets:     make([]IPv4SubnetItem, 0),
	}

	ip := strings.TrimSpace(req.IP)
	mask := strings.TrimSpace(req.Mask)
	hostCount := strings.TrimSpace(req.HostCount)
	subnetCount := strings.TrimSpace(req.SubnetCount)

	if mask == "" {
		return result
	}

	cidr, maskValid := parseMaskOrWildcard(mask)
	var ipAddr netip.Addr
	ipValid := false
	if ip != "" {
		ipAddr, ipValid = parseIPv4Addr(ip)
	}

	if !maskValid {
		result.BaseError = "无效的掩码格式，请输入CIDR、子网掩码或反掩码"
	} else if ip == "" {
		result.BaseRecords = buildIPv4MaskOnlyRecords(cidr)
	} else if !ipValid {
		result.BaseError = "无效的 IP 地址格式"
	} else {
		result.BaseRecords = buildIPv4Details(ipAddr, cidr)
	}

	if ip == "" || mask == "" || (hostCount == "" && subnetCount == "") {
		return result
	}

	if !maskValid || !ipValid {
		result.SubnetError = "请先完成上方基础网络信息的有效填写"
		return result
	}

	targetCidr := cidr
	if hostCount != "" {
		hosts, ok := parsePositiveUint(hostCount)
		if !ok {
			result.SubnetError = "请输入有效的主机数"
			return result
		}
		hostBits := requiredHostBits(hosts)
		targetCidr = 32 - hostBits
		if targetCidr < cidr {
			result.SubnetError = fmt.Sprintf("当前掩码 /%d 的网络无法提供 %d 个连续主机的空间", cidr, hosts)
			return result
		}
	} else if subnetCount != "" {
		subnets, ok := parsePositiveUint(subnetCount)
		if !ok {
			result.SubnetError = "请输入有效的子网数"
			return result
		}
		subnetBits := requiredSubnetBits(subnets)
		targetCidr = cidr + subnetBits
		if targetCidr > 32 {
			result.SubnetError = fmt.Sprintf("当前掩码 /%d 无法划分出 %d 个子网，位空间不足", cidr, subnets)
			return result
		}
	}

	baseIPLong := ipv4ToUint32(ipAddr)
	baseMaskLong := cidrToMaskUint32(cidr)
	baseNetworkLong := baseIPLong & baseMaskLong
	newMaskLong := cidrToMaskUint32(targetCidr)

	totalSubnets := uint64(1) << uint(targetCidr-cidr)
	result.TotalSubnets = int(totalSubnets)

	limit := totalSubnets
	if req.ForceDisplayAllSubnets {
		if limit > absoluteIPv4SubnetsLimit {
			limit = absoluteIPv4SubnetsLimit
		}
	} else if limit > maxIPv4SubnetsToGenerate {
		limit = maxIPv4SubnetsToGenerate
	}

	step := uint64(1) << uint(32-targetCidr)
	for i := uint64(0); i < limit; i++ {
		networkLong := uint64(baseNetworkLong) + i*step
		broadcastLong := networkLong | uint64(^newMaskLong)

		var usableHosts uint64
		var firstUsable string
		var lastUsable string
		if targetCidr < 31 {
			usableHosts = (uint64(1) << uint(32-targetCidr)) - 2
			firstUsable = uint32ToIPv4(uint32(networkLong + 1))
			lastUsable = uint32ToIPv4(uint32(broadcastLong - 1))
		} else if targetCidr == 31 {
			usableHosts = 2
			firstUsable = uint32ToIPv4(uint32(networkLong))
			lastUsable = uint32ToIPv4(uint32(broadcastLong))
		} else {
			usableHosts = 1
			firstUsable = uint32ToIPv4(uint32(networkLong))
			lastUsable = uint32ToIPv4(uint32(broadcastLong))
		}

		_ = usableHosts
		result.Subnets = append(result.Subnets, IPv4SubnetItem{
			Index:       int(i) + 1,
			Network:     uint32ToIPv4(uint32(networkLong)),
			Cidr:        targetCidr,
			FirstUsable: firstUsable,
			LastUsable:  lastUsable,
			Broadcast:   uint32ToIPv4(uint32(broadcastLong)),
			Mask:        uint32ToIPv4(newMaskLong),
		})
	}

	if !req.ForceDisplayAllSubnets && totalSubnets > maxIPv4SubnetsToGenerate {
		result.SubnetWarning = fmt.Sprintf("由于数据量过大，总计划分出 %s 个子网，此处仅展示前 %d 个保护浏览器性能。", formatUintWithCommas(totalSubnets), maxIPv4SubnetsToGenerate)
		result.ShowForceButton = true
	} else if req.ForceDisplayAllSubnets && totalSubnets > absoluteIPv4SubnetsLimit {
		result.SubnetWarning = fmt.Sprintf("数据量极其庞大！已强制展示前 %s 个子网。为防止浏览器崩溃，剩余数据不再渲染。", formatUintWithCommas(absoluteIPv4SubnetsLimit))
	} else if req.ForceDisplayAllSubnets && totalSubnets > maxIPv4SubnetsToGenerate {
		result.SubnetWarning = fmt.Sprintf("已强制展示全部 %s 个子网，由于节点众多，页面若有轻微卡顿属于正常现象。", formatUintWithCommas(totalSubnets))
	}

	return result
}

// ExportIPv4SubnetsCSV 导出 IPv4 子网划分 CSV（后端生成内容，前端仅触发下载）。
func (s *ForgeService) ExportIPv4SubnetsCSV(req IPv4CalcRequest) (*IPv4SubnetCSVResult, error) {
	calc := s.CalculateIPv4(req)
	if calc.SubnetError != "" {
		return nil, errors.New(calc.SubnetError)
	}
	if len(calc.Subnets) == 0 {
		return nil, errors.New("暂无可导出的子网数据")
	}

	var sb strings.Builder
	sb.WriteString("\ufeff")
	cw := csv.NewWriter(&sb)

	headers := []string{
		"序号",
		"网络号",
		"CIDR",
		"首个可用 IP",
		"最后可用 IP",
		"广播地址",
		"子网掩码",
	}
	if err := cw.Write(headers); err != nil {
		return nil, fmt.Errorf("写入 CSV 表头失败: %w", err)
	}

	for _, item := range calc.Subnets {
		row := []string{
			strconv.Itoa(item.Index),
			item.Network,
			strconv.Itoa(item.Cidr),
			item.FirstUsable,
			item.LastUsable,
			item.Broadcast,
			item.Mask,
		}
		if err := cw.Write(row); err != nil {
			return nil, fmt.Errorf("写入 CSV 行失败: %w", err)
		}
	}

	cw.Flush()
	if err := cw.Error(); err != nil {
		return nil, fmt.Errorf("生成 CSV 失败: %w", err)
	}

	baseNet := calc.Subnets[0].Network
	if baseNet == "" {
		baseNet = "未知网络"
	}
	fileName := strings.ReplaceAll(baseNet, ".", "_") + "_子网划分明细.csv"

	return &IPv4SubnetCSVResult{
		FileName: fileName,
		Content:  sb.String(),
	}, nil
}

// CalculateIPv6 IPv6 网络信息、包含关系与子网划分计算。
func (s *ForgeService) CalculateIPv6(req IPv6CalcRequest) *IPv6CalcResult {
	result := &IPv6CalcResult{
		BaseRecords: make([]NetworkRecord, 0),
		Subnets:     make([]IPv6SubnetItem, 0),
	}

	ip := strings.TrimSpace(req.IP)
	prefixRaw := strings.TrimSpace(req.Prefix)
	checkIP := strings.TrimSpace(req.CheckIP)
	newPrefixRaw := strings.TrimSpace(req.NewPrefix)

	if ip == "" {
		return result
	}

	ipAddr, ipValid := parseIPv6Addr(ip)
	if !ipValid {
		result.BaseError = "无效的 IPv6 地址格式"
	} else if prefixRaw == "" {
		expanded := expandIPv6(ipAddr)
		result.BaseRecords = []NetworkRecord{
			{Label: "完整地址展示", Value: expanded},
			{Label: "压缩地址展示", Value: compressIPv6(ipAddr)},
			{Label: "类型", Value: getIPv6Type(expanded)},
		}
	} else if prefixNum, ok := parsePrefix(prefixRaw, 128); !ok {
		result.BaseError = "前缀长度必须在 0 到 128 之间"
	} else {
		result.BaseRecords = buildIPv6Details(ipAddr, prefixNum)
	}

	if ipValid && prefixRaw != "" && checkIP != "" {
		if prefixNum, ok := parsePrefix(prefixRaw, 128); ok {
			if checkAddr, checkOK := parseIPv6Addr(checkIP); checkOK {
				maskBig := ipv6Mask(prefixNum)
				baseNetwork := new(big.Int).And(addrToBigInt(ipAddr), maskBig)
				checkNetwork := new(big.Int).And(addrToBigInt(checkAddr), maskBig)
				isIncluded := baseNetwork.Cmp(checkNetwork) == 0
				baseNetAddr := bigIntToIPv6Addr(baseNetwork)

				msg := ""
				if isIncluded {
					msg = fmt.Sprintf("地址 %s 包含在当前 %s/%d 网段内", checkIP, compressIPv6(baseNetAddr), prefixNum)
				} else {
					msg = fmt.Sprintf("地址 %s 不在此网段内", checkIP)
				}

				result.InclusionCheck = &IPv6InclusionResult{
					IsIncluded: isIncluded,
					Message:    msg,
				}
			}
		}
	}

	if ip == "" || prefixRaw == "" || newPrefixRaw == "" {
		return result
	}

	if !ipValid {
		result.SubnetError = "请先完成上方有效 IPv6 填写"
		return result
	}

	prefixNum, ok := parsePrefix(prefixRaw, 128)
	if !ok {
		result.SubnetError = "请填写正确的原前缀长度"
		return result
	}

	newPrefixNum, ok := parsePrefix(newPrefixRaw, 128)
	if !ok || newPrefixNum <= prefixNum || newPrefixNum > 128 {
		result.SubnetError = fmt.Sprintf("新前缀必须大于原前缀 (%d) 且不超过 128", prefixNum)
		return result
	}

	diff := newPrefixNum - prefixNum
	if diff > maxIPv6SubnetDiff {
		result.SubnetError = fmt.Sprintf("单次最多仅支持下拨 %d 位 (即 65536 个子网)，当前尝试下拨 %d 位，数据过大导致浏览器越界。", maxIPv6SubnetDiff, diff)
		return result
	}

	totalSubnets := 1 << diff
	result.TotalSubnets = totalSubnets

	baseMask := ipv6Mask(prefixNum)
	networkBig := new(big.Int).And(addrToBigInt(ipAddr), baseMask)
	step := new(big.Int).Lsh(big.NewInt(1), uint(128-newPrefixNum))

	limit := totalSubnets
	if limit > maxIPv6SubnetsToGenerate {
		limit = maxIPv6SubnetsToGenerate
	}

	checkValid := false
	checkIncludedInBase := false
	targetSubnetMask := ipv6Mask(newPrefixNum)
	checkNetwork := big.NewInt(0)
	if checkAddr, ok := parseIPv6Addr(checkIP); ok {
		checkValid = true
		checkNetwork = new(big.Int).And(addrToBigInt(checkAddr), targetSubnetMask)
		if result.InclusionCheck != nil && result.InclusionCheck.IsIncluded {
			checkIncludedInBase = true
		}
	}

	current := new(big.Int).Set(networkBig)
	for i := 0; i < limit; i++ {
		subnetBig := new(big.Int).Set(current)
		isIncluded := false
		if checkValid {
			currentSubnet := new(big.Int).And(new(big.Int).Set(subnetBig), targetSubnetMask)
			isIncluded = currentSubnet.Cmp(checkNetwork) == 0 && checkIncludedInBase
		}

		result.Subnets = append(result.Subnets, IPv6SubnetItem{
			Index:      i + 1,
			Network:    compressIPv6(bigIntToIPv6Addr(subnetBig)),
			Cidr:       newPrefixNum,
			IsIncluded: isIncluded,
		})

		current.Add(current, step)
	}

	if totalSubnets > maxIPv6SubnetsToGenerate {
		result.SubnetWarning = fmt.Sprintf("总计划分出 %s 个子网，为保护页面性能，此处仅展示前 %d 个。", formatUintWithCommas(uint64(totalSubnets)), maxIPv6SubnetsToGenerate)
	}

	return result
}

func parseMaskOrWildcard(maskInput string) (int, bool) {
	maskInput = strings.TrimSpace(maskInput)
	if maskInput == "" {
		return 0, false
	}

	cidrRaw := maskInput
	if strings.HasPrefix(cidrRaw, "/") {
		cidrRaw = strings.TrimSpace(strings.TrimPrefix(cidrRaw, "/"))
	}
	if cidr, ok := parseStrictInt(cidrRaw, 0, 32); ok {
		return cidr, true
	}

	maskAddr, ok := parseIPv4Addr(maskInput)
	if !ok {
		return 0, false
	}
	maskLong := ipv4ToUint32(maskAddr)
	if ok, ones := isOnesThenZeros(maskLong); ok {
		return ones, true
	}
	if ok, zeros := isZerosThenOnes(maskLong); ok {
		return zeros, true
	}
	return 0, false
}

func buildIPv4Details(ip netip.Addr, cidr int) []NetworkRecord {
	ipLong := ipv4ToUint32(ip)
	maskLong := cidrToMaskUint32(cidr)
	networkLong := ipLong & maskLong
	broadcastLong := networkLong | ^maskLong

	hostBits := 32 - cidr
	totalHosts := uint64(1) << uint(hostBits)
	usableHosts := uint64(0)
	if cidr < 31 {
		usableHosts = totalHosts - 2
	} else if cidr == 31 {
		usableHosts = 2
	} else {
		usableHosts = 1
	}

	firstUsable := networkLong
	lastUsable := broadcastLong
	if cidr < 31 {
		firstUsable = networkLong + 1
		lastUsable = broadcastLong - 1
	}

	return []NetworkRecord{
		{Label: "网络地址", Value: uint32ToIPv4(networkLong)},
		{Label: "广播地址", Value: uint32ToIPv4(broadcastLong)},
		{Label: "子网掩码", Value: uint32ToIPv4(maskLong)},
		{Label: "反掩码", Value: uint32ToIPv4(^maskLong)},
		{Label: "CIDR", Value: fmt.Sprintf("/%d", cidr)},
		{Label: "子网范围", Value: fmt.Sprintf("%s - %s", uint32ToIPv4(networkLong), uint32ToIPv4(broadcastLong))},
		{Label: "可用主机数", Value: formatUintWithCommas(usableHosts)},
		{Label: "首个可用地址", Value: uint32ToIPv4(firstUsable)},
		{Label: "最后一个可用地址", Value: uint32ToIPv4(lastUsable)},
	}
}

func buildIPv4MaskOnlyRecords(cidr int) []NetworkRecord {
	maskLong := cidrToMaskUint32(cidr)
	return []NetworkRecord{
		{Label: "子网掩码", Value: uint32ToIPv4(maskLong)},
		{Label: "反掩码", Value: uint32ToIPv4(^maskLong)},
		{Label: "CIDR", Value: fmt.Sprintf("/%d", cidr)},
	}
}

func buildIPv6Details(ip netip.Addr, prefix int) []NetworkRecord {
	maskBig := ipv6Mask(prefix)
	ipBig := addrToBigInt(ip)
	networkBig := new(big.Int).And(ipBig, maskBig)
	hostMask := new(big.Int).Xor(new(big.Int).Set(ipv6AllOnes()), maskBig)
	endBig := new(big.Int).Or(new(big.Int).Set(networkBig), hostMask)

	networkAddr := bigIntToIPv6Addr(networkBig)
	endAddr := bigIntToIPv6Addr(endBig)
	expanded := expandIPv6(ip)

	hostBits := 128 - prefix
	hosts := ""
	if hostBits == 0 {
		hosts = "1 (表示单一地址)"
	} else if hostBits < 53 {
		hosts = formatUintWithCommas(uint64(1) << uint(hostBits))
	} else {
		hosts = fmt.Sprintf("2^%d (极其庞大)", hostBits)
	}

	ipRange := compressIPv6(networkAddr)
	if hostBits != 0 {
		ipRange = fmt.Sprintf("%s - %s", compressIPv6(networkAddr), compressIPv6(endAddr))
	}

	return []NetworkRecord{
		{Label: "完整地址展示", Value: expanded},
		{Label: "压缩地址展示", Value: compressIPv6(ip)},
		{Label: "网络前缀 / CIDR", Value: fmt.Sprintf("/%d", prefix)},
		{Label: "网络地址", Value: compressIPv6(networkAddr)},
		{Label: "类型", Value: getIPv6Type(expanded)},
		{Label: "可用 IP 范围", Value: ipRange},
		{Label: "地址总数", Value: hosts},
	}
}

func parseIPv4Addr(ip string) (netip.Addr, bool) {
	addr, err := netip.ParseAddr(strings.TrimSpace(ip))
	if err != nil || !addr.Is4() {
		return netip.Addr{}, false
	}
	return addr, true
}

func parseIPv6Addr(ip string) (netip.Addr, bool) {
	ip = strings.TrimSpace(ip)
	if idx := strings.Index(ip, "%"); idx >= 0 {
		ip = ip[:idx]
	}
	addr, err := netip.ParseAddr(ip)
	if err != nil || !addr.Is6() {
		return netip.Addr{}, false
	}
	return addr, true
}

func parsePrefix(raw string, max int) (int, bool) {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "/") {
		raw = strings.TrimSpace(strings.TrimPrefix(raw, "/"))
	}
	return parseStrictInt(raw, 0, max)
}

func parseStrictInt(raw string, min int, max int) (int, bool) {
	if raw == "" {
		return 0, false
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false
	}
	if strconv.Itoa(n) != raw {
		return 0, false
	}
	if n < min || n > max {
		return 0, false
	}
	return n, true
}

func parsePositiveUint(raw string) (uint64, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false
	}
	v, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || v == 0 {
		return 0, false
	}
	return v, true
}

func requiredHostBits(hosts uint64) int {
	needed := hosts + 2
	if needed < hosts {
		return 32
	}
	bits := 0
	for bits < 32 && (uint64(1)<<uint(bits)) < needed {
		bits++
	}
	return bits
}

func requiredSubnetBits(subnets uint64) int {
	bits := 0
	for bits < 32 && (uint64(1)<<uint(bits)) < subnets {
		bits++
	}
	return bits
}

func ipv4ToUint32(ip netip.Addr) uint32 {
	octets := ip.As4()
	return uint32(octets[0])<<24 | uint32(octets[1])<<16 | uint32(octets[2])<<8 | uint32(octets[3])
}

func uint32ToIPv4(v uint32) string {
	return fmt.Sprintf("%d.%d.%d.%d", byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
}

func cidrToMaskUint32(cidr int) uint32 {
	if cidr == 0 {
		return 0
	}
	return ^uint32(0) << uint(32-cidr)
}

func isOnesThenZeros(v uint32) (bool, int) {
	ones := 0
	seenZero := false
	for i := 31; i >= 0; i-- {
		bit := (v >> uint(i)) & 1
		if bit == 1 {
			if seenZero {
				return false, 0
			}
			ones++
		} else {
			seenZero = true
		}
	}
	return true, ones
}

func isZerosThenOnes(v uint32) (bool, int) {
	zeros := 0
	seenOne := false
	for i := 31; i >= 0; i-- {
		bit := (v >> uint(i)) & 1
		if bit == 0 {
			if seenOne {
				return false, 0
			}
			zeros++
		} else {
			seenOne = true
		}
	}
	return true, zeros
}

func addrToBigInt(addr netip.Addr) *big.Int {
	b := addr.As16()
	return new(big.Int).SetBytes(b[:])
}

func bigIntToIPv6Addr(n *big.Int) netip.Addr {
	var bytes [16]byte
	nb := n.FillBytes(make([]byte, 16))
	copy(bytes[:], nb)
	return netip.AddrFrom16(bytes)
}

func ipv6Mask(prefix int) *big.Int {
	if prefix <= 0 {
		return big.NewInt(0)
	}
	if prefix >= 128 {
		return new(big.Int).Set(ipv6AllOnes())
	}
	mask := new(big.Int).Lsh(big.NewInt(1), uint(prefix))
	mask.Sub(mask, big.NewInt(1))
	mask.Lsh(mask, uint(128-prefix))
	return mask
}

func ipv6AllOnes() *big.Int {
	v := new(big.Int).Lsh(big.NewInt(1), 128)
	v.Sub(v, big.NewInt(1))
	return v
}

func expandIPv6(addr netip.Addr) string {
	b := addr.As16()
	parts := make([]string, 8)
	for i := 0; i < 8; i++ {
		parts[i] = fmt.Sprintf("%02x%02x", b[i*2], b[i*2+1])
	}
	return strings.Join(parts, ":")
}

func compressIPv6(addr netip.Addr) string {
	return addr.String()
}

func getIPv6Type(expanded string) string {
	if expanded == "" {
		return "未知"
	}
	if expanded == "0000:0000:0000:0000:0000:0000:0000:0000" {
		return "未指定地址 (Unspecified)"
	}
	if expanded == "0000:0000:0000:0000:0000:0000:0000:0001" {
		return "环回地址 (Loopback)"
	}

	parts := strings.Split(expanded, ":")
	if len(parts) != 8 {
		return "未知"
	}
	firstBlock, err := strconv.ParseInt(parts[0], 16, 32)
	if err != nil {
		return "未知"
	}

	v := int(firstBlock)
	if (v & 0xff00) == 0xff00 {
		return "组播地址 (Multicast)"
	}
	if (v & 0xfe80) == 0xfe80 {
		return "链路本地单播 (Link-Local)"
	}
	if (v & 0xfec0) == 0xfec0 {
		return "站点本地单播 (Site-Local) - 已废弃"
	}
	if (v&0xfc00) == 0xfc00 || (v&0xfd00) == 0xfd00 {
		return "唯一本地地址 (ULA)"
	}
	if (v & 0xe000) == 0x2000 {
		return "全球单播地址 (Global Unicast)"
	}
	return "未知/保留地址"
}

func formatUintWithCommas(v uint64) string {
	s := strconv.FormatUint(v, 10)
	if len(s) <= 3 {
		return s
	}

	var b strings.Builder
	head := len(s) % 3
	if head != 0 {
		b.WriteString(s[:head])
		if len(s) > head {
			b.WriteByte(',')
		}
	}

	for i := head; i < len(s); i += 3 {
		b.WriteString(s[i : i+3])
		if i+3 < len(s) {
			b.WriteByte(',')
		}
	}
	return b.String()
}
