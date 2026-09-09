// Package snmp 提供 SNMP 即时查询能力。
//
// 定位：交付现场的临时采集工具，用于设备上线确认、资产信息（型号/版本/序列号）核对。
// 刻意不做：周期性轮询调度、Trap 值守监听、MIB 库管理 —— 这些属于长期值守型网管系统的职责。
package snmp

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/gosnmp/gosnmp"

	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
)

// ============================================================================
// 常量与默认 OID
// ============================================================================

const (
	// DefaultPort SNMP 默认端口
	DefaultPort = 161
	// DefaultTimeout 默认单次查询超时
	DefaultTimeout = 5 * time.Second
	// DefaultConcurrency 批量查询默认并发数
	DefaultConcurrency = 10
)

// OIDEntry OID 条目
type OIDEntry struct {
	OID  string `json:"oid"`
	Name string `json:"name"`
}

// DeviceInfoOIDs 设备基本信息 OID 集合
// 用于设备上线确认与基础资产信息采集
var DeviceInfoOIDs = []OIDEntry{
	{OID: "1.3.6.1.2.1.1.1.0", Name: "sysDescr"},
	{OID: "1.3.6.1.2.1.1.3.0", Name: "sysUpTime"},
	{OID: "1.3.6.1.2.1.1.5.0", Name: "sysName"},
}

// ============================================================================
// 查询器
// ============================================================================

// QuerierConfig 查询器配置
type QuerierConfig struct {
	Timeout     time.Duration // 单次 SNMP 请求超时（默认 5s）
	Concurrency int           // 批量查询并发数（默认 10）
}

// DefaultQuerierConfig 默认查询器配置
var DefaultQuerierConfig = QuerierConfig{
	Timeout:     DefaultTimeout,
	Concurrency: DefaultConcurrency,
}

// Querier SNMP 查询器
// 提供即时的 GET / WALK 能力，无状态、无后台任务
type Querier struct {
	crypto *CredentialCrypto
	config QuerierConfig
}

// NewQuerier 创建 SNMP 查询器实例
func NewQuerier(crypto *CredentialCrypto, config ...QuerierConfig) *Querier {
	cfg := DefaultQuerierConfig
	if len(config) > 0 {
		cfg = config[0]
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = DefaultQuerierConfig.Timeout
	}
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = DefaultQuerierConfig.Concurrency
	}

	logger.Info("SNMP", "-", "SNMP 查询器已初始化 (超时: %v, 并发: %d)", cfg.Timeout, cfg.Concurrency)
	return &Querier{crypto: crypto, config: cfg}
}

// ============================================================================
// 对外查询方法
// ============================================================================

// Get 对指定地址执行 SNMP GET
// addr 形如 192.168.1.1 或 192.168.1.1:161，缺省端口为 161
// cred 为 nil 时使用默认 community "public"
func (q *Querier) Get(ctx context.Context, addr string, oids []string, cred *models.SNMPCredential) ([]SNMPResult, error) {
	if len(oids) == 0 {
		return nil, fmt.Errorf("OID 列表不能为空")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	startTime := time.Now()
	client, err := q.connect(addr, cred)
	if err != nil {
		return nil, err
	}
	defer client.Conn.Close()

	res, err := client.Get(oids)
	if err != nil {
		logger.Warn("SNMP", client.Target, "GET 失败: 地址=%s, 错误=%v", addr, err)
		return nil, fmt.Errorf("SNMP GET 失败: %w", err)
	}

	results := make([]SNMPResult, 0, len(res.Variables))
	for _, pdu := range res.Variables {
		results = append(results, q.pduToResult(pdu, ""))
	}

	logger.Debug("SNMP", client.Target, "GET 成功: 地址=%s, PDU数=%d, 耗时=%v",
		addr, len(results), time.Since(startTime))
	return results, nil
}

// Walk 对指定地址执行 SNMP WALK
func (q *Querier) Walk(ctx context.Context, addr string, rootOID string, cred *models.SNMPCredential) ([]SNMPResult, error) {
	if rootOID == "" {
		return nil, fmt.Errorf("根 OID 不能为空")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	startTime := time.Now()
	client, err := q.connect(addr, cred)
	if err != nil {
		return nil, err
	}
	defer client.Conn.Close()

	var results []SNMPResult
	err = client.Walk(rootOID, func(pdu gosnmp.SnmpPDU) error {
		results = append(results, q.pduToResult(pdu, ""))
		return nil
	})
	if err != nil {
		logger.Warn("SNMP", client.Target, "WALK 失败: 地址=%s, 根OID=%s, 错误=%v", addr, rootOID, err)
		return nil, fmt.Errorf("SNMP WALK 失败: %w", err)
	}

	logger.Debug("SNMP", client.Target, "WALK 成功: 地址=%s, 根OID=%s, PDU数=%d, 耗时=%v",
		addr, rootOID, len(results), time.Since(startTime))
	return results, nil
}

// GetDeviceInfo 采集设备基本信息（sysDescr / sysUpTime / sysName）
// 用于设备上线确认，单个 OID 失败不影响其余结果
func (q *Querier) GetDeviceInfo(ctx context.Context, addr string, cred *models.SNMPCredential) ([]SNMPResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	client, err := q.connect(addr, cred)
	if err != nil {
		return nil, err
	}
	defer client.Conn.Close()

	results := make([]SNMPResult, 0, len(DeviceInfoOIDs))
	var failed int
	for _, entry := range DeviceInfoOIDs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		res, err := client.Get([]string{entry.OID})
		if err != nil || len(res.Variables) == 0 {
			logger.Verbose("SNMP", client.Target, "设备信息 OID 采集失败: %s(%s), 错误=%v", entry.Name, entry.OID, err)
			results = append(results, SNMPResult{
				OID:     entry.OID,
				OIDName: entry.Name,
				Value:   "",
				Error:   "noSuchObject 或请求失败",
			})
			failed++
			continue
		}
		results = append(results, q.pduToResult(res.Variables[0], entry.Name))
	}

	// 全部失败才判定为设备不可达
	if failed == len(DeviceInfoOIDs) {
		return results, fmt.Errorf("设备 %s 无响应，SNMP 查询全部失败", addr)
	}

	logger.Info("SNMP", client.Target, "设备信息采集完成: 地址=%s, 成功=%d/%d",
		addr, len(DeviceInfoOIDs)-failed, len(DeviceInfoOIDs))
	return results, nil
}

// BatchGetDeviceInfo 批量采集设备基本信息
// 用于批量上线确认，单台设备失败不影响其余设备
func (q *Querier) BatchGetDeviceInfo(ctx context.Context, addrs []string, cred *models.SNMPCredential) []BatchQueryResult {
	if len(addrs) == 0 {
		return nil
	}

	results := make([]BatchQueryResult, len(addrs))
	sem := make(chan struct{}, q.config.Concurrency)
	var wg sync.WaitGroup

	for i, addr := range addrs {
		wg.Add(1)
		go func(idx int, address string) {
			defer wg.Done()

			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				results[idx] = BatchQueryResult{Address: address, Reachable: false, Error: "任务已取消"}
				return
			}

			start := time.Now()
			info, err := q.GetDeviceInfo(ctx, address, cred)
			results[idx] = BatchQueryResult{
				Address:   address,
				Reachable: err == nil,
				Results:   info,
				Error:     errText(err),
				Latency:   time.Since(start).Milliseconds(),
			}
		}(i, addr)
	}

	wg.Wait()
	logger.Info("SNMP", "-", "批量设备信息采集完成: 总数=%d", len(addrs))
	return results
}

// ============================================================================
// 连接与客户端构造
// ============================================================================

// connect 建立 SNMP 连接
func (q *Querier) connect(addr string, cred *models.SNMPCredential) (*gosnmp.GoSNMP, error) {
	community, err := q.getCommunity(cred)
	if err != nil {
		return nil, fmt.Errorf("获取凭据失败: %w", err)
	}

	host, port := splitAddr(addr)
	if cred != nil && strings.ToLower(cred.Version) == "v3" {
		return q.connectV3(host, port, cred)
	}
	return q.connectV1V2(host, port, community, cred)
}

// splitAddr 拆分地址与端口，缺省端口为 161
func splitAddr(addr string) (host string, port uint16) {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return addr, DefaultPort
	}
	port = DefaultPort
	var portInt int
	if _, err := fmt.Sscanf(portStr, "%d", &portInt); err == nil && portInt > 0 {
		port = uint16(portInt)
	}
	return host, port
}

// connectV1V2 建立 SNMP v1/v2c 连接
func (q *Querier) connectV1V2(host string, port uint16, community string, cred *models.SNMPCredential) (*gosnmp.GoSNMP, error) {
	version := gosnmp.Version2c
	versionStr := "v2c"
	if cred != nil && strings.ToLower(cred.Version) == "v1" {
		version = gosnmp.Version1
		versionStr = "v1"
	}

	client := &gosnmp.GoSNMP{
		Target:         host,
		Port:           port,
		Transport:      "udp",
		Community:      community,
		Version:        version,
		Timeout:        q.config.Timeout,
		Retries:        0,
		MaxOids:        60,
		MaxRepetitions: 50,
	}

	if err := client.Connect(); err != nil {
		logger.Warn("SNMP", host, "SNMP%s 连接失败: %s:%d, 错误=%v", versionStr, host, port, err)
		return nil, fmt.Errorf("连接 SNMP 目标失败 (%s:%d): %w", host, port, err)
	}
	return client, nil
}

// connectV3 建立 SNMP v3 连接
func (q *Querier) connectV3(host string, port uint16, cred *models.SNMPCredential) (*gosnmp.GoSNMP, error) {
	authPassword, err := q.decrypt(cred.AuthPassword)
	if err != nil {
		return nil, fmt.Errorf("解密认证密钥失败: %w", err)
	}
	privPassword, err := q.decrypt(cred.PrivPassword)
	if err != nil {
		return nil, fmt.Errorf("解密加密密钥失败: %w", err)
	}

	client := &gosnmp.GoSNMP{
		Target:         host,
		Port:           port,
		Transport:      "udp",
		Version:        gosnmp.Version3,
		Timeout:        q.config.Timeout,
		Retries:        0,
		MaxOids:        60,
		MaxRepetitions: 50,
		SecurityModel:  gosnmp.UserSecurityModel,
		MsgFlags:       mapSecurityLevel(cred.SecurityLevel),
		SecurityParameters: &gosnmp.UsmSecurityParameters{
			UserName:                 cred.Username,
			AuthenticationProtocol:   mapAuthProtocol(cred.AuthProtocol),
			AuthenticationPassphrase: authPassword,
			PrivacyProtocol:          mapPrivProtocol(cred.PrivProtocol),
			PrivacyPassphrase:        privPassword,
		},
		ContextName:     cred.ContextName,
		ContextEngineID: cred.ContextEngineID,
	}

	if err := client.Connect(); err != nil {
		logger.Warn("SNMP", host, "SNMPv3 连接失败: %s:%d, 用户=%s, 错误=%v", host, port, cred.Username, err)
		return nil, fmt.Errorf("连接 SNMPv3 目标失败 (%s:%d): %w", host, port, err)
	}
	return client, nil
}

// ============================================================================
// 凭据处理
// ============================================================================

// getCommunity 获取解密后的 community string
func (q *Querier) getCommunity(cred *models.SNMPCredential) (string, error) {
	if cred == nil || cred.Community == "" {
		return "public", nil
	}
	if q.crypto != nil && IsEncrypted(cred.Community) {
		decrypted, err := q.crypto.DecryptCredential(cred.Community)
		if err != nil {
			return "", fmt.Errorf("解密 community 失败: %w", err)
		}
		return decrypted, nil
	}
	return cred.Community, nil
}

// decrypt 解密凭据字段，空值直接返回空
func (q *Querier) decrypt(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	if q.crypto == nil || !IsEncrypted(value) {
		return value, nil
	}
	return q.crypto.DecryptCredential(value)
}

// ============================================================================
// 结果转换
// ============================================================================

// pduToResult 将 SNMP PDU 转换为查询结果
// oidName 为空时保留 OID 本身作为名称
func (q *Querier) pduToResult(pdu gosnmp.SnmpPDU, oidName string) SNMPResult {
	if oidName == "" {
		oidName = normalizeOID(pdu.Name)
	}
	return SNMPResult{
		OID:       normalizeOID(pdu.Name),
		OIDName:   oidName,
		Value:     formatPDUValue(pdu),
		ValueType: getPDUTypeString(pdu),
	}
}

// formatPDUValue 格式化 PDU 值为字符串
func formatPDUValue(pdu gosnmp.SnmpPDU) string {
	switch pdu.Type {
	case gosnmp.OctetString:
		if b, ok := pdu.Value.([]byte); ok {
			if isPrintableString(b) {
				return string(b)
			}
			return fmt.Sprintf("%x", b)
		}
		return fmt.Sprintf("%v", pdu.Value)

	case gosnmp.ObjectIdentifier, gosnmp.IPAddress:
		return fmt.Sprintf("%v", pdu.Value)

	case gosnmp.TimeTicks:
		// TimeTicks 单位为百分之一秒
		ticks := gosnmp.ToBigInt(pdu.Value).Int64()
		duration := time.Duration(ticks) * time.Millisecond * 10
		return fmt.Sprintf("%v (%s)", ticks, duration.String())

	// SNMP 异常类型，无实际值
	case gosnmp.NoSuchObject, gosnmp.NoSuchInstance, gosnmp.EndOfMibView:
		return ""

	default:
		return fmt.Sprintf("%v", gosnmp.ToBigInt(pdu.Value))
	}
}

// getPDUTypeString 获取 PDU 类型字符串
func getPDUTypeString(pdu gosnmp.SnmpPDU) string {
	switch pdu.Type {
	case gosnmp.Integer:
		return "integer"
	case gosnmp.OctetString:
		return "string"
	case gosnmp.ObjectIdentifier:
		return "oid"
	case gosnmp.IPAddress:
		return "ipaddress"
	case gosnmp.Counter32:
		return "counter32"
	case gosnmp.Gauge32:
		return "gauge32"
	case gosnmp.TimeTicks:
		return "timeticks"
	case gosnmp.Counter64:
		return "counter64"
	case gosnmp.Uinteger32:
		return "uinteger32"
	case gosnmp.OpaqueFloat:
		return "float"
	case gosnmp.OpaqueDouble:
		return "double"
	case gosnmp.Null:
		return "null"
	case gosnmp.NoSuchObject:
		return "noSuchObject"
	case gosnmp.NoSuchInstance:
		return "noSuchInstance"
	case gosnmp.EndOfMibView:
		return "endOfMibView"
	default:
		return fmt.Sprintf("unknown(%d)", pdu.Type)
	}
}

// ============================================================================
// 协议参数映射
// ============================================================================

func mapSecurityLevel(level string) gosnmp.SnmpV3MsgFlags {
	switch strings.ToLower(level) {
	case "authnopriv":
		return gosnmp.AuthNoPriv
	case "authpriv":
		return gosnmp.AuthPriv
	default:
		return gosnmp.NoAuthNoPriv
	}
}

func mapAuthProtocol(protocol string) gosnmp.SnmpV3AuthProtocol {
	switch strings.ToLower(protocol) {
	case "md5":
		return gosnmp.MD5
	case "sha":
		return gosnmp.SHA
	case "sha224":
		return gosnmp.SHA224
	case "sha256":
		return gosnmp.SHA256
	case "sha384":
		return gosnmp.SHA384
	case "sha512":
		return gosnmp.SHA512
	default:
		return gosnmp.MD5
	}
}

func mapPrivProtocol(protocol string) gosnmp.SnmpV3PrivProtocol {
	switch strings.ToLower(protocol) {
	case "des":
		return gosnmp.DES
	case "aes":
		return gosnmp.AES
	case "aes192":
		return gosnmp.AES192
	case "aes256":
		return gosnmp.AES256
	case "aes192c":
		return gosnmp.AES192C
	case "aes256c":
		return gosnmp.AES256C
	default:
		return gosnmp.AES
	}
}

// ============================================================================
// 辅助函数
// ============================================================================

// normalizeOID 标准化 OID 格式（移除前后空白与前导点）
func normalizeOID(oid string) string {
	oid = strings.TrimSpace(oid)
	return strings.TrimPrefix(oid, ".")
}

// isPrintableString 判断字节切片是否为可打印字符串
func isPrintableString(b []byte) bool {
	for _, c := range b {
		if c < 32 && c != '\t' && c != '\n' && c != '\r' {
			return false
		}
		if c > 126 {
			return false
		}
	}
	return true
}

// errText 提取错误文本，nil 返回空串
func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
