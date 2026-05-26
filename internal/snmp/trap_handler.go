// Package snmp 提供 SNMP 核心业务功能
// trap_handler.go 实现 Trap 处理器，负责解析和存储 Trap 数据
package snmp

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/gosnmp/gosnmp"

	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
	"github.com/NetWeaverGo/core/internal/repository"
)

// ============================================================================
// 处理统计
// ============================================================================

// HandlerStats 处理统计
type HandlerStats struct {
	TotalReceived int64 `json:"totalReceived"` // 总接收数
	TotalStored   int64 `json:"totalStored"`   // 总存储数
	TotalFiltered int64 `json:"totalFiltered"` // 总过滤数
	TotalErrors   int64 `json:"totalErrors"`   // 总错误数
}

// ============================================================================
// Trap 处理器
// ============================================================================

// DefaultTrapTimeout Trap 处理默认超时时间
const DefaultTrapTimeout = 30 * time.Second

// TrapHandler Trap 处理器
// 负责解析 gosnmp.SnmpPacket 为 TrapRecord 模型，
// 通过过滤引擎过滤，使用 OID 解析器解析名称，存储到数据库，并通过事件通知器推送
type TrapHandler struct {
	repo     repository.TrapRepository
	filter   *TrapFilterEngine
	resolver *OIDResolver
	notifier EventNotifier

	// 可配置的超时时间
	timeout time.Duration

	stats HandlerStats
	mu    sync.Mutex
}

// NewTrapHandler 创建处理器
func NewTrapHandler(
	repo repository.TrapRepository,
	filter *TrapFilterEngine,
	resolver *OIDResolver,
	notifier EventNotifier,
) *TrapHandler {
	handler := &TrapHandler{
		repo:     repo,
		filter:   filter,
		resolver: resolver,
		notifier: notifier,
		timeout:  DefaultTrapTimeout,
	}

	logger.Info("SNMP-Handler", "-", "Trap 处理器已初始化")
	return handler
}

// SetTimeout 设置处理超时时间
func (h *TrapHandler) SetTimeout(timeout time.Duration) {
	h.mu.Lock()
	h.timeout = timeout
	h.mu.Unlock()
}

// ============================================================================
// 核心处理方法
// ============================================================================

// HandleTrap 处理接收到的 Trap
// 实现 gosnmp.TrapListenerFunc 接口
func (h *TrapHandler) HandleTrap(packet *gosnmp.SnmpPacket, addr *net.UDPAddr) error {
	handleStartTime := time.Now()
	h.mu.Lock()
	h.stats.TotalReceived++
	h.mu.Unlock()

	if packet == nil || addr == nil {
		h.mu.Lock()
		h.stats.TotalErrors++
		h.mu.Unlock()
		logger.Error("SNMP-Handler", "-", "无效的 Trap 数据包或地址")
		return fmt.Errorf("无效的 Trap 数据包或地址")
	}

	logger.Debug("SNMP-Handler", addr.IP.String(), "收到 Trap: 版本=%s, Community=%s, PDU类型=%s",
		packet.Version.String(), string(packet.Community), packet.PDUType.String())

	// 1. 解析 Trap 数据包
	parseStartTime := time.Now()
	trap, err := h.parseTrap(packet, addr)
	if err != nil {
		h.mu.Lock()
		h.stats.TotalErrors++
		h.mu.Unlock()
		logger.Error("SNMP-Handler", addr.IP.String(), "解析 Trap 失败: 耗时=%v, 错误=%v",
			time.Since(parseStartTime), err)
		return err
	}
	parseLatency := time.Since(parseStartTime)

	// 2. 应用过滤规则
	filterLatency := time.Duration(0)
	if h.filter != nil {
		filterStartTime := time.Now()
		filterResult := h.filter.ApplyFilter(trap)
		filterLatency = time.Since(filterStartTime)

		if filterResult.Action == "drop" {
			h.mu.Lock()
			h.stats.TotalFiltered++
			h.mu.Unlock()
			logger.Debug("SNMP-Handler", addr.IP.String(), "Trap 已过滤: OID=%s, 规则ID=%d, 规则名=%s, 过滤耗时=%v",
				trap.TrapOID, filterResult.RuleID, filterResult.RuleName, filterLatency)
			return nil
		}

		// 应用严重级别覆盖
		if filterResult.Action == "severity_override" && filterResult.OverrideSeverity != "" {
			logger.Debug("SNMP-Handler", addr.IP.String(), "Trap 严重级别覆盖: OID=%s, %s -> %s",
				trap.TrapOID, trap.Severity, filterResult.OverrideSeverity)
			trap.Severity = filterResult.OverrideSeverity
		}
	}

	// 3. 使用 OID 解析器解析 Trap OID
	resolveLatency := time.Duration(0)
	if h.resolver != nil {
		resolveStartTime := time.Now()
		resolved, err := h.resolver.ResolveOID(trap.TrapOID)
		if err == nil && resolved.Found {
			trap.TrapName = resolved.Name
			logger.Verbose("SNMP-Handler", addr.IP.String(), "OID 解析成功: %s -> %s (模块: %s)",
				trap.TrapOID, resolved.Name, resolved.ModuleName)
		} else if err != nil {
			logger.Verbose("SNMP-Handler", addr.IP.String(), "OID 解析失败: %s, %v", trap.TrapOID, err)
		}

		// 解析 VarBinds 中的 OID
		h.resolveVarBinds(trap)
		resolveLatency = time.Since(resolveStartTime)
	}

	// 4. 存储到数据库（使用带超时的上下文，避免阻塞）
	h.mu.Lock()
	timeout := h.timeout
	h.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	storeStartTime := time.Now()
	if err := h.repo.CreateTrap(ctx, trap); err != nil {
		h.mu.Lock()
		h.stats.TotalErrors++
		h.mu.Unlock()
		logger.Error("SNMP-Handler", addr.IP.String(), "存储 Trap 失败: OID=%s, 耗时=%v, 错误=%v",
			trap.TrapOID, time.Since(storeStartTime), err)
		return err
	}
	storeLatency := time.Since(storeStartTime)

	h.mu.Lock()
	h.stats.TotalStored++
	h.mu.Unlock()

	// 5. 通过事件通知器推送实时通知
	// 注意：仅调用 NotifyTrapReceived，避免重复发射 snmp:trap_received 事件
	// NotifyNewTrap 内部也会发射同一事件名，会导致前端收到重复通知
	if h.notifier != nil {
		h.notifier.NotifyTrapReceived(TrapEvent{
			SourceIP:   trap.SourceIP,
			SourcePort: trap.SourcePort,
			TrapOID:    trap.TrapOID,
			TrapName:   trap.TrapName,
			Severity:   trap.Severity,
			Community:  trap.Community,
			Version:    trap.Version,
			ReceivedAt: trap.ReceivedAt.UnixMilli(),
		})
	}

	totalLatency := time.Since(handleStartTime)
	logger.Info("SNMP-Handler", addr.IP.String(), "Trap 处理完成: OID=%s, 名称=%s, 严重级别=%s, 总耗时=%v (解析=%v, 过滤=%v, 解析OID=%v, 存储=%v)",
		trap.TrapOID, trap.TrapName, trap.Severity, totalLatency, parseLatency, filterLatency, resolveLatency, storeLatency)

	return nil
}

// ============================================================================
// 解析方法
// ============================================================================

// parseTrap 解析 Trap 数据包为 TrapRecord
func (h *TrapHandler) parseTrap(packet *gosnmp.SnmpPacket, addr *net.UDPAddr) (*models.SNMPTrapRecord, error) {
	trap := &models.SNMPTrapRecord{
		SourceIP:   addr.IP.String(),
		SourcePort: addr.Port,
		Community:  string(packet.Community),
		ReceivedAt: time.Now(),
	}

	// 根据 SNMP 版本解析
	switch packet.Version {
	case gosnmp.Version1:
		trap.Version = "v1"
		h.parseV1Trap(packet, trap)
	case gosnmp.Version2c:
		trap.Version = "v2c"
		h.parseV2cTrap(packet, trap)
	case gosnmp.Version3:
		trap.Version = "v3"
		h.parseV3Trap(packet, trap)
	default:
		trap.Version = fmt.Sprintf("unknown(%d)", packet.Version)
	}

	// 解析 VarBinds
	vars, rawHex := h.parseVarBinds(packet.Variables)
	trap.Variables = vars
	trap.RawHex = rawHex

	// 推断严重级别（如果未设置）
	if trap.Severity == "" {
		trap.Severity = h.inferSeverity(trap)
	}

	return trap, nil
}

// parseV1Trap 解析 SNMPv1 Trap
func (h *TrapHandler) parseV1Trap(packet *gosnmp.SnmpPacket, trap *models.SNMPTrapRecord) {
	// SNMPv1 Trap PDU 特有字段
	if packet.Enterprise != "" {
		trap.Enterprise = packet.Enterprise
	}
	trap.GenericTrap = int(packet.GenericTrap)
	trap.SpecificTrap = int(packet.SpecificTrap)

	// 从 Enterprise + GenericTrap/SpecificTrap 推导 Trap OID
	trap.TrapOID = h.deriveV1TrapOID(packet)
}

// parseV2cTrap 解析 SNMPv2c Trap
func (h *TrapHandler) parseV2cTrap(packet *gosnmp.SnmpPacket, trap *models.SNMPTrapRecord) {
	// SNMPv2c Trap 中 Trap OID 在 VarBinds 的第一个变量中
	// snmpTrapOID (1.3.6.1.6.3.1.1.4.1.0)
	for _, pdu := range packet.Variables {
		if pdu.Name == "1.3.6.1.6.3.1.1.4.1.0" {
			switch pdu.Type {
			case gosnmp.ObjectIdentifier:
				if oid, ok := pdu.Value.(string); ok {
					trap.TrapOID = normalizeOID(oid)
				}
			}
			break
		}
	}

	// 如果没有找到 Trap OID，使用 PDU 的 Name
	if trap.TrapOID == "" && len(packet.Variables) > 0 {
		trap.TrapOID = normalizeOID(packet.Variables[0].Name)
	}
}

// parseV3Trap 解析 SNMPv3 Trap
// SNMPv3 Trap 格式与 v2c 类似，Trap OID 在 VarBinds 的 snmpTrapOID.0 中
func (h *TrapHandler) parseV3Trap(packet *gosnmp.SnmpPacket, trap *models.SNMPTrapRecord) {
	// SNMPv3 Trap 中 Trap OID 在 VarBinds 的第一个变量中
	// snmpTrapOID (1.3.6.1.6.3.1.1.4.1.0)
	for _, pdu := range packet.Variables {
		if pdu.Name == "1.3.6.1.6.3.1.1.4.1.0" {
			switch pdu.Type {
			case gosnmp.ObjectIdentifier:
				if oid, ok := pdu.Value.(string); ok {
					trap.TrapOID = oid
				}
			}
			break
		}
	}

	// 如果没有找到 Trap OID，使用 PDU 的 Name
	if trap.TrapOID == "" && len(packet.Variables) > 0 {
		trap.TrapOID = packet.Variables[0].Name
	}
}

// deriveV1TrapOID 从 SNMPv1 Trap PDU 推导 Trap OID
func (h *TrapHandler) deriveV1TrapOID(packet *gosnmp.SnmpPacket) string {
	// Generic Trap 类型 (0-6)
	// 0: coldStart             1.3.6.1.6.3.1.1.5.1
	// 1: warmStart             1.3.6.1.6.3.1.1.5.2
	// 2: linkDown              1.3.6.1.6.3.1.1.5.3
	// 3: linkUp                1.3.6.1.6.3.1.1.5.4
	// 4: authenticationFailure 1.3.6.1.6.3.1.1.5.5
	// 5: egpNeighborLoss       1.3.6.1.6.3.1.1.5.6
	// 6: enterpriseSpecific    enterprise.0.specificTrap

	genericTrapOIDs := map[int]string{
		0: "1.3.6.1.6.3.1.1.5.1", // coldStart
		1: "1.3.6.1.6.3.1.1.5.2", // warmStart
		2: "1.3.6.1.6.3.1.1.5.3", // linkDown
		3: "1.3.6.1.6.3.1.1.5.4", // linkUp
		4: "1.3.6.1.6.3.1.1.5.5", // authenticationFailure
		5: "1.3.6.1.6.3.1.1.5.6", // egpNeighborLoss
	}

	if oid, ok := genericTrapOIDs[int(packet.GenericTrap)]; ok && packet.GenericTrap < 6 {
		return oid
	}

	// Enterprise Specific Trap
	if packet.Enterprise != "" {
		return fmt.Sprintf("%s.0.%d", packet.Enterprise, packet.SpecificTrap)
	}

	return ""
}

// parseVarBinds 解析 VarBinds 为 JSON 和原始十六进制
func (h *TrapHandler) parseVarBinds(variables []gosnmp.SnmpPDU) (string, string) {
	if len(variables) == 0 {
		return "[]", ""
	}

	type VarBind struct {
		OID       string      `json:"oid"`
		OIDName   string      `json:"oidName,omitempty"`
		Type      string      `json:"type"`
		Value     interface{} `json:"value"`
	}

	var binds []VarBind
	var hexParts []string

	for _, pdu := range variables {
		vb := VarBind{
			OID:   normalizeOID(pdu.Name),
			Type:  pduTypeToString(pdu.Type),
			Value: pduValueToString(pdu),
		}

		binds = append(binds, vb)

		// 收集原始十六进制
		if pdu.Value != nil {
			hexParts = append(hexParts, fmt.Sprintf("%s=%v", pdu.Name, pdu.Value))
		}
	}

	jsonData, err := json.Marshal(binds)
	if err != nil {
		logger.Error("SNMP-Handler", "-", "序列化 VarBinds 失败: %v", err)
		jsonData = []byte("[]")
	}

	rawHex := strings.Join(hexParts, ";")
	if len(rawHex) > 10000 {
		rawHex = rawHex[:10000] + "...(truncated)"
	}

	return string(jsonData), rawHex
}

// resolveVarBinds 解析 Trap 中 VarBinds 的 OID
func (h *TrapHandler) resolveVarBinds(trap *models.SNMPTrapRecord) {
	if trap.Variables == "" || trap.Variables == "[]" || h.resolver == nil {
		return
	}

	type VarBind struct {
		OID       string      `json:"oid"`
		OIDName   string      `json:"oidName,omitempty"`
		Type      string      `json:"type"`
		Value     interface{} `json:"value"`
	}

	var binds []VarBind
	if err := json.Unmarshal([]byte(trap.Variables), &binds); err != nil {
		return
	}

	changed := false
	for i, vb := range binds {
		if vb.OIDName == "" && vb.OID != "" {
			resolved, err := h.resolver.ResolveOID(vb.OID)
			if err == nil && resolved.Found {
				binds[i].OIDName = resolved.Name
				changed = true
			}
		}
	}

	if changed {
		if jsonData, err := json.Marshal(binds); err == nil {
			trap.Variables = string(jsonData)
		}
	}
}

// ============================================================================
// 辅助方法
// ============================================================================

// inferSeverity 推断 Trap 严重级别
func (h *TrapHandler) inferSeverity(trap *models.SNMPTrapRecord) string {
	// 基于 Trap OID 推断严重级别
	trapOID := normalizeOID(trap.TrapOID)

	// 已知的关键 Trap OID 模式
	criticalPatterns := []string{
		"1.3.6.1.6.3.1.1.5.1", // coldStart
	}
	majorPatterns := []string{
		"1.3.6.1.6.3.1.1.5.2", // warmStart
		"1.3.6.1.6.3.1.1.5.5", // authenticationFailure
	}
	minorPatterns := []string{
		"1.3.6.1.6.3.1.1.5.3", // linkDown
	}
	infoPatterns := []string{
		"1.3.6.1.6.3.1.1.5.4", // linkUp
		"1.3.6.1.6.3.1.1.5.6", // egpNeighborLoss
	}

	for _, p := range criticalPatterns {
		if strings.HasPrefix(trapOID, p) {
			return "critical"
		}
	}
	for _, p := range majorPatterns {
		if strings.HasPrefix(trapOID, p) {
			return "major"
		}
	}
	for _, p := range minorPatterns {
		if strings.HasPrefix(trapOID, p) {
			return "minor"
		}
	}
	for _, p := range infoPatterns {
		if strings.HasPrefix(trapOID, p) {
			return "info"
		}
	}

	return "unknown"
}

// pduTypeToString 将 gosnmp PDU 类型转换为字符串
func pduTypeToString(t gosnmp.Asn1BER) string {
	switch t {
	case gosnmp.Boolean:
		return "boolean"
	case gosnmp.Integer:
		return "integer"
	case gosnmp.OctetString:
		return "octetString"
	case gosnmp.Null:
		return "null"
	case gosnmp.ObjectIdentifier:
		return "objectIdentifier"
	case gosnmp.IPAddress:
		return "ipAddress"
	case gosnmp.Counter32:
		return "counter32"
	case gosnmp.Gauge32:
		return "gauge32"
	case gosnmp.TimeTicks:
		return "timeTicks"
	case gosnmp.Opaque:
		return "opaque"
	case gosnmp.Counter64:
		return "counter64"
	case gosnmp.OpaqueFloat:
		return "opaqueFloat"
	case gosnmp.OpaqueDouble:
		return "opaqueDouble"
	case gosnmp.NoSuchInstance:
		return "noSuchInstance"
	case gosnmp.NoSuchObject:
		return "noSuchObject"
	case gosnmp.EndOfMibView:
		return "endOfMibView"
	default:
		return fmt.Sprintf("unknown(%d)", t)
	}
}

// pduValueToString 将 gosnmp PDU 值转换为字符串
func pduValueToString(pdu gosnmp.SnmpPDU) string {
	if pdu.Value == nil {
		return ""
	}

	switch pdu.Type {
	case gosnmp.OctetString:
		// 尝试作为字符串，否则转为十六进制
		if bytes, ok := pdu.Value.([]byte); ok {
			// 检查是否是可打印的 ASCII
			printable := true
			for _, b := range bytes {
				if b < 32 || b > 126 {
					printable = false
					break
				}
			}
			if printable {
				return string(bytes)
			}
			return hex.EncodeToString(bytes)
		}
		return fmt.Sprintf("%v", pdu.Value)

	case gosnmp.ObjectIdentifier:
		return fmt.Sprintf("%v", pdu.Value)

	case gosnmp.IPAddress:
		return fmt.Sprintf("%v", pdu.Value)

	case gosnmp.TimeTicks:
		if ticks, ok := pdu.Value.(uint); ok {
			duration := time.Duration(ticks) * time.Millisecond / 100
			return fmt.Sprintf("%d days, %02d:%02d:%02d",
				int(duration.Hours()/24),
				int(duration.Hours())%24,
				int(duration.Minutes())%60,
				int(duration.Seconds())%60)
		}
		return fmt.Sprintf("%v", pdu.Value)

	case gosnmp.Counter64:
		return fmt.Sprintf("%v", pdu.Value)

	default:
		return fmt.Sprintf("%v", pdu.Value)
	}
}

// ============================================================================
// 统计方法
// ============================================================================

// GetStats 获取处理统计
func (h *TrapHandler) GetStats() HandlerStats {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.stats
}

// ResetStats 重置统计
func (h *TrapHandler) ResetStats() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.stats = HandlerStats{}
	logger.Info("SNMP-Handler", "-", "处理统计已重置")
}
