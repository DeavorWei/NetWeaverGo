// Package snmp 提供 SNMP 核心业务功能
// poller.go 实现 SNMP 轮询器，支持 v1/v2c 协议的 GET/WALK 操作
package snmp

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gosnmp/gosnmp"
	"github.com/google/uuid"

	"github.com/NetWeaverGo/core/internal/logger"
	"github.com/NetWeaverGo/core/internal/models"
)

// ============================================================================
// 轮询器配置
// ============================================================================

// PollerConfig 轮询器配置
type PollerConfig struct {
	Timeout     time.Duration // SNMP 请求超时时间（默认 5s）
	Retries     int           // 协议层重试次数（默认 3，gosnmp 内置）
	MaxWorkers  int           // 最大并发轮询数（默认 10）
	MaxAppRetries int         // 应用层重试次数（默认 3，用于网络不稳定场景）
	BaseRetryDelay time.Duration // 应用层重试基础延迟（默认 1s）
}

// DefaultPollerConfig 默认轮询器配置
var DefaultPollerConfig = PollerConfig{
	Timeout:        5 * time.Second,
	Retries:        3,
	MaxWorkers:     10,
	MaxAppRetries:  3,
	BaseRetryDelay: 1 * time.Second,
}

// ============================================================================
// 轮询目标封装
// ============================================================================

// PollTarget 轮询目标封装（包含目标、模板和凭据）
type PollTarget struct {
	Target   *models.SNMPPollingTarget
	Template *models.SNMPPollingTemplate
	Cred     *models.SNMPCredential
}

// ============================================================================
// SNMP 轮询器
// ============================================================================

// Poller SNMP 轮询器
// 支持 v1/v2c 协议的 GET/WALK 操作，提供并发轮询能力
type Poller struct {
	resolver *OIDResolver
	crypto   *CredentialCrypto
	notifier EventNotifier
	config   PollerConfig

	// 统计信息
	totalPolls   int64
	successCount int64
	failCount    int64

	// 并发控制
	workerSem chan struct{}
	mu        sync.RWMutex
}

// NewPoller 创建 SNMP 轮询器实例
func NewPoller(resolver *OIDResolver, crypto *CredentialCrypto, notifier EventNotifier, config ...PollerConfig) *Poller {
	cfg := DefaultPollerConfig
	if len(config) > 0 {
		cfg = config[0]
	}

	// 确保配置有效
	if cfg.Timeout <= 0 {
		cfg.Timeout = DefaultPollerConfig.Timeout
	}
	if cfg.Retries < 0 {
		cfg.Retries = DefaultPollerConfig.Retries
	}
	if cfg.MaxWorkers <= 0 {
		cfg.MaxWorkers = DefaultPollerConfig.MaxWorkers
	}
	if cfg.MaxAppRetries < 0 {
		cfg.MaxAppRetries = DefaultPollerConfig.MaxAppRetries
	}
	if cfg.BaseRetryDelay <= 0 {
		cfg.BaseRetryDelay = DefaultPollerConfig.BaseRetryDelay
	}

	poller := &Poller{
		resolver:  resolver,
		crypto:    crypto,
		notifier:  notifier,
		config:    cfg,
		workerSem: make(chan struct{}, cfg.MaxWorkers),
	}

	logger.Info("SNMP-Poller", "-", "SNMP 轮询器已初始化 (超时: %v, 协议重试: %d, 应用重试: %d, 并发: %d)",
		cfg.Timeout, cfg.Retries, cfg.MaxAppRetries, cfg.MaxWorkers)

	return poller
}

// ============================================================================
// 核心轮询方法
// ============================================================================

// ============================================================================
// 应用层重试方法
// ============================================================================

// pollWithRetry 带应用层重试的轮询方法
// 用于网络不稳定场景，实现指数退避+抖动
// 只对特定错误类型重试（超时、网络错误），不重试认证错误
func (p *Poller) pollWithRetry(ctx context.Context, target *PollTarget) ([]*models.SNMPPollingResult, error) {
	var lastErr error

	for attempt := 0; attempt <= p.config.MaxAppRetries; attempt++ {
		// 检查上下文是否已取消
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		// 执行轮询
		results, err := p.PollSingle(ctx, target)
		if err == nil {
			// 成功，返回结果
			if attempt > 0 {
				logger.Info("SNMP-Poller", "-", "应用层重试成功: 目标=%s, 重试次数=%d",
					target.Target.TargetIP, attempt)
			}
			return results, nil
		}

		// 检查是否为可重试错误
		if !isRetriableError(err) {
			logger.Warn("SNMP-Poller", "-", "不可重试错误: 目标=%s, 错误=%v",
				target.Target.TargetIP, err)
			return nil, err
		}

		lastErr = err

		// 非最后一次尝试，计算延迟并等待
		if attempt < p.config.MaxAppRetries {
			delay := p.calculateRetryDelay(attempt)
			logger.Warn("SNMP-Poller", "-", "轮询失败，准备重试: 目标=%s, 尝试=%d/%d, 延迟=%v, 错误=%v",
				target.Target.TargetIP, attempt+1, p.config.MaxAppRetries, delay, err)

			// 等待延迟或上下文取消
			select {
			case <-time.After(delay):
				// 继续重试
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}

	// 所有重试失败
	logger.Error("SNMP-Poller", "-", "应用层重试耗尽: 目标=%s, 重试次数=%d, 最终错误=%v",
		target.Target.TargetIP, p.config.MaxAppRetries, lastErr)
	return nil, fmt.Errorf("应用层重试耗尽: %w", lastErr)
}

// calculateRetryDelay 计算重试延迟（指数退避+抖动）
// 公式: delay = baseDelay * 2^attempt + jitter(0-500ms)
func (p *Poller) calculateRetryDelay(attempt int) time.Duration {
	// 指数退避基础
	baseDelay := p.config.BaseRetryDelay * time.Duration(1<<uint(attempt))

	// 抖动：随机 0-500ms，避免惊群效应
	jitter := time.Duration(rand.Intn(500)) * time.Millisecond

	return baseDelay + jitter
}

// isRetriableError 判断错误是否可重试
// 只重试超时和网络错误，不重试认证错误
func isRetriableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// 可重试的错误类型
	retriablePatterns := []string{
		"timeout",
		"timed out",
		"deadline exceeded",
		"context deadline exceeded",
		"connection refused",
		"connection reset",
		"network is unreachable",
		"no route to host",
		"temporary failure",
		"resource temporarily unavailable",
		"io: read/write on closed pipe",
		"broken pipe",
	}

	for _, pattern := range retriablePatterns {
		if strings.Contains(strings.ToLower(errStr), pattern) {
			return true
		}
	}

	// 不可重试的错误类型（认证、配置错误）
	nonRetriablePatterns := []string{
		"authentication",
		"auth",
		"credential",
		"community",
		"permission denied",
		"access denied",
		"invalid",
		"unsupported",
		"version",
	}

	for _, pattern := range nonRetriablePatterns {
		if strings.Contains(strings.ToLower(errStr), pattern) {
			return false
		}
	}

	// 默认不重试未知错误
	return false
}

// ============================================================================
// 核心轮询方法
// ============================================================================

// PollSingle 执行单次轮询
// 返回轮询结果列表和错误信息
func (p *Poller) PollSingle(ctx context.Context, target *PollTarget) ([]*models.SNMPPollingResult, error) {
	if target == nil || target.Target == nil {
		return nil, fmt.Errorf("轮询目标不能为空")
	}

	startTime := time.Now()
	targetID := target.Target.ID
	targetIP := target.Target.TargetIP

	logger.Debug("SNMP-Poller", "-", "开始轮询目标: ID=%d, IP=%s", targetID, targetIP)

	// 获取凭据（解密 community string）
	community, err := p.getCommunity(target.Cred)
	if err != nil {
		p.notifyError(targetID, targetIP, err)
		return nil, fmt.Errorf("获取凭据失败: %w", err)
	}

	// 创建 SNMP 客户端
	addr := fmt.Sprintf("%s:%d", targetIP, target.Target.TargetPort)
	if target.Target.TargetPort == 0 {
		addr = fmt.Sprintf("%s:161", targetIP)
	}

	client, err := p.createSNMPClient(addr, community, target.Cred)
	if err != nil {
		p.notifyError(targetID, targetIP, err)
		return nil, fmt.Errorf("创建 SNMP 客户端失败: %w", err)
	}
	defer client.Conn.Close()

	// 执行轮询
	results := make([]*models.SNMPPollingResult, 0)
	batchID := uuid.New().String()
	pollTime := time.Now()

	if target.Template != nil && len(target.Template.OIDItems) > 0 {
		// 按模板 OID 列表轮询
		for _, oidItem := range target.Template.OIDItems {
			oidResults, err := p.pollOID(ctx, client, target.Target, oidItem, batchID, pollTime)
			if err != nil {
				logger.Warn("SNMP-Poller", "-", "轮询 OID 失败: %s, %v", oidItem.OID, err)
				continue
			}
			results = append(results, oidResults...)
		}
	} else {
		// 无模板时，使用默认系统信息 OID
		defaultOIDs := []models.SNMPOIDItem{
			{OID: "1.3.6.1.2.1.1.1.0", Name: "sysDescr", Operation: "get"},
			{OID: "1.3.6.1.2.1.1.3.0", Name: "sysUpTime", Operation: "get"},
			{OID: "1.3.6.1.2.1.1.5.0", Name: "sysName", Operation: "get"},
		}
		for _, oidItem := range defaultOIDs {
			oidResults, err := p.pollOID(ctx, client, target.Target, oidItem, batchID, pollTime)
			if err != nil {
				logger.Warn("SNMP-Poller", "-", "轮询默认 OID 失败: %s, %v", oidItem.OID, err)
				continue
			}
			results = append(results, oidResults...)
		}
	}

	// 更新统计
	atomic.AddInt64(&p.totalPolls, 1)
	atomic.AddInt64(&p.successCount, 1)

	// 发送成功通知
	latency := time.Since(startTime)
	p.notifySuccess(targetID, targetIP, len(results), batchID, pollTime, latency)

	logger.Info("SNMP-Poller", "-", "轮询完成: ID=%d, IP=%s, 结果数=%d, 耗时=%v",
		targetID, targetIP, len(results), latency)

	return results, nil
}

// PollBatch 执行批量轮询
// 使用并发控制，限制同时轮询的目标数量
// 使用应用层重试机制提高成功率
func (p *Poller) PollBatch(ctx context.Context, targets []*PollTarget) [][]*models.SNMPPollingResult {
	if len(targets) == 0 {
		return nil
	}

	batchStartTime := time.Now()
	logger.Info("SNMP-Poller", "-", "开始批量轮询: 目标数=%d, 并发数=%d, 应用层重试启用",
		len(targets), p.config.MaxWorkers)

	results := make([][]*models.SNMPPollingResult, len(targets))
	var wg sync.WaitGroup

	// 并发控制统计
	activeCount := int64(0)

	for i, target := range targets {
		// 获取信号量，限制并发数
		p.workerSem <- struct{}{}
		active := atomic.AddInt64(&activeCount, 1)
		logger.Debug("SNMP-Poller", "-", "并发状态: 活跃=%d, 等待=%d", active, len(targets)-i-1)

		wg.Add(1)
		go func(idx int, t *PollTarget) {
			defer wg.Done()
			defer func() {
				<-p.workerSem
				atomic.AddInt64(&activeCount, -1)
			}()

			// 使用带重试的轮询方法
			result, err := p.pollWithRetry(ctx, t)
			if err != nil {
				logger.Error("SNMP-Poller", "-", "批量轮询单目标失败: ID=%d, IP=%s, %v",
					t.Target.ID, t.Target.TargetIP, err)
				results[idx] = nil
				return
			}
			results[idx] = result
		}(i, target)
	}

	wg.Wait()

	successCount := 0
	totalOidCount := 0
	for _, r := range results {
		if r != nil {
			successCount++
			totalOidCount += len(r)
		}
	}

	batchLatency := time.Since(batchStartTime)
	logger.Info("SNMP-Poller", "-", "批量轮询完成: 成功=%d/%d, 总OID数=%d, 总耗时=%v, 平均耗时=%v/目标",
		successCount, len(targets), totalOidCount, batchLatency, batchLatency/time.Duration(len(targets)))

	return results
}

// PollBatchNoRetry 执行批量轮询（不带应用层重试）
// 用于需要快速失败的场景
func (p *Poller) PollBatchNoRetry(ctx context.Context, targets []*PollTarget) [][]*models.SNMPPollingResult {
	if len(targets) == 0 {
		return nil
	}

	batchStartTime := time.Now()
	logger.Info("SNMP-Poller", "-", "开始批量轮询（无应用重试）: 目标数=%d, 并发数=%d",
		len(targets), p.config.MaxWorkers)

	results := make([][]*models.SNMPPollingResult, len(targets))
	var wg sync.WaitGroup

	for i, target := range targets {
		// 获取信号量，限制并发数
		p.workerSem <- struct{}{}

		wg.Add(1)
		go func(idx int, t *PollTarget) {
			defer wg.Done()
			defer func() { <-p.workerSem }()

			result, err := p.PollSingle(ctx, t)
			if err != nil {
				logger.Error("SNMP-Poller", "-", "批量轮询单目标失败: ID=%d, IP=%s, %v",
					t.Target.ID, t.Target.TargetIP, err)
				results[idx] = nil
				return
			}
			results[idx] = result
		}(i, target)
	}

	wg.Wait()

	successCount := 0
	for _, r := range results {
		if r != nil {
			successCount++
		}
	}

	batchLatency := time.Since(batchStartTime)
	logger.Info("SNMP-Poller", "-", "批量轮询（无重试）完成: 成功=%d/%d, 总耗时=%v",
		successCount, len(targets), batchLatency)

	return results
}

// ============================================================================
// SNMP 操作方法
// ============================================================================

// Get 执行 SNMP GET 操作
func (p *Poller) Get(ctx context.Context, addr string, oids []string, cred *models.SNMPCredential) ([]gosnmp.SnmpPDU, error) {
	startTime := time.Now()
	logger.Debug("SNMP-Poller", "-", "SNMP GET 请求: 地址=%s, OID数=%d", addr, len(oids))

	community, err := p.getCommunity(cred)
	if err != nil {
		return nil, err
	}

	client, err := p.createSNMPClient(addr, community, cred)
	if err != nil {
		return nil, err
	}
	defer client.Conn.Close()

	result, err := client.Get(oids)
	if err != nil {
		logger.Warn("SNMP-Poller", "-", "SNMP GET 失败: 地址=%s, 错误=%v", addr, err)
		return nil, fmt.Errorf("SNMP GET 失败: %w", err)
	}

	latency := time.Since(startTime)
	logger.Debug("SNMP-Poller", "-", "SNMP GET 成功: 地址=%s, 返回PDU数=%d, 耗时=%v", addr, len(result.Variables), latency)
	return result.Variables, nil
}

// Walk 执行 SNMP WALK 操作
func (p *Poller) Walk(ctx context.Context, addr string, rootOID string, cred *models.SNMPCredential) ([]gosnmp.SnmpPDU, error) {
	startTime := time.Now()
	logger.Debug("SNMP-Poller", "-", "SNMP WALK 请求: 地址=%s, 根OID=%s", addr, rootOID)

	community, err := p.getCommunity(cred)
	if err != nil {
		return nil, err
	}

	client, err := p.createSNMPClient(addr, community, cred)
	if err != nil {
		return nil, err
	}
	defer client.Conn.Close()

	var results []gosnmp.SnmpPDU
	err = client.Walk(rootOID, func(pdu gosnmp.SnmpPDU) error {
		results = append(results, pdu)
		return nil
	})
	if err != nil {
		logger.Warn("SNMP-Poller", "-", "SNMP WALK 失败: 地址=%s, 根OID=%s, 错误=%v", addr, rootOID, err)
		return nil, fmt.Errorf("SNMP WALK 失败: %w", err)
	}

	latency := time.Since(startTime)
	logger.Debug("SNMP-Poller", "-", "SNMP WALK 成功: 地址=%s, 根OID=%s, 返回PDU数=%d, 耗时=%v", addr, rootOID, len(results), latency)
	return results, nil
}

// ============================================================================
// 辅助方法
// ============================================================================

// pollOID 轮询单个 OID
func (p *Poller) pollOID(ctx context.Context, client *gosnmp.GoSNMP, target *models.SNMPPollingTarget, oidItem models.SNMPOIDItem, batchID string, pollTime time.Time) ([]*models.SNMPPollingResult, error) {
	var results []*models.SNMPPollingResult

	switch oidItem.Operation {
	case "walk":
		// WALK 操作
		pdus, err := p.walkOID(client, oidItem.OID)
		if err != nil {
			return nil, err
		}
		for _, pdu := range pdus {
			result := p.pduToResult(pdu, target, oidItem, batchID, pollTime)
			results = append(results, result)
		}

	case "bulk":
		// BULK 操作（v2c 使用 GetBulk）
		pdus, err := p.bulkWalk(client, oidItem.OID)
		if err != nil {
			return nil, err
		}
		for _, pdu := range pdus {
			result := p.pduToResult(pdu, target, oidItem, batchID, pollTime)
			results = append(results, result)
		}

	default: // "get"
		// GET 操作
		pdu, err := p.getOID(client, oidItem.OID)
		if err != nil {
			return nil, err
		}
		result := p.pduToResult(pdu, target, oidItem, batchID, pollTime)
		results = append(results, result)
	}

	return results, nil
}

// getOID 执行单个 OID 的 GET 操作
func (p *Poller) getOID(client *gosnmp.GoSNMP, oid string) (gosnmp.SnmpPDU, error) {
	startTime := time.Now()
	logger.Verbose("SNMP-Poller", client.Target, "发送 GET 请求: OID=%s", oid)

	result, err := client.Get([]string{oid})
	if err != nil {
		logger.Verbose("SNMP-Poller", client.Target, "GET 请求失败: OID=%s, 错误=%v", oid, err)
		return gosnmp.SnmpPDU{}, fmt.Errorf("GET %s 失败: %w", oid, err)
	}

	if len(result.Variables) == 0 {
		logger.Verbose("SNMP-Poller", client.Target, "GET 请求无返回: OID=%s", oid)
		return gosnmp.SnmpPDU{}, fmt.Errorf("GET %s 无返回结果", oid)
	}

	latency := time.Since(startTime)
	logger.Verbose("SNMP-Poller", client.Target, "GET 请求成功: OID=%s, 类型=%s, 耗时=%v",
		oid, p.getPDUTypeString(result.Variables[0]), latency)
	return result.Variables[0], nil
}

// walkOID 执行 WALK 操作
func (p *Poller) walkOID(client *gosnmp.GoSNMP, rootOID string) ([]gosnmp.SnmpPDU, error) {
	startTime := time.Now()
	logger.Verbose("SNMP-Poller", client.Target, "发送 WALK 请求: 根OID=%s", rootOID)

	var results []gosnmp.SnmpPDU

	err := client.Walk(rootOID, func(pdu gosnmp.SnmpPDU) error {
		results = append(results, pdu)
		return nil
	})

	if err != nil {
		logger.Verbose("SNMP-Poller", client.Target, "WALK 请求失败: 根OID=%s, 错误=%v", rootOID, err)
		return nil, fmt.Errorf("WALK %s 失败: %w", rootOID, err)
	}

	latency := time.Since(startTime)
	logger.Verbose("SNMP-Poller", client.Target, "WALK 请求成功: 根OID=%s, 返回PDU数=%d, 耗时=%v",
		rootOID, len(results), latency)
	return results, nil
}

// bulkWalk 执行 GetBulk 操作（v2c 优化）
func (p *Poller) bulkWalk(client *gosnmp.GoSNMP, rootOID string) ([]gosnmp.SnmpPDU, error) {
	startTime := time.Now()
	logger.Verbose("SNMP-Poller", client.Target, "发送 BULK WALK 请求: 根OID=%s", rootOID)

	var results []gosnmp.SnmpPDU

	// 使用 BulkWalk 替代普通 Walk（更高效）
	err := client.BulkWalk(rootOID, func(pdu gosnmp.SnmpPDU) error {
		results = append(results, pdu)
		return nil
	})

	if err != nil {
		logger.Verbose("SNMP-Poller", client.Target, "BULK WALK 请求失败: 根OID=%s, 错误=%v", rootOID, err)
		return nil, fmt.Errorf("BULK WALK %s 失败: %w", rootOID, err)
	}

	latency := time.Since(startTime)
	logger.Verbose("SNMP-Poller", client.Target, "BULK WALK 请求成功: 根OID=%s, 返回PDU数=%d, 耗时=%v",
		rootOID, len(results), latency)
	return results, nil
}

// pduToResult 将 SNMP PDU 转换为轮询结果模型
func (p *Poller) pduToResult(pdu gosnmp.SnmpPDU, target *models.SNMPPollingTarget, oidItem models.SNMPOIDItem, batchID string, pollTime time.Time) *models.SNMPPollingResult {
	// 解析 OID 名称
	oidName := oidItem.Name
	if p.resolver != nil {
		resolved, err := p.resolver.ResolveOID(pdu.Name)
		if err == nil && resolved.Found {
			oidName = resolved.Name
		}
	}

	// 转换值
	value := p.formatPDUValue(pdu)
	valueType := p.getPDUTypeString(pdu)

	return &models.SNMPPollingResult{
		TargetID:  target.ID,
		TargetIP:  target.TargetIP,
		BatchID:   batchID,
		OID:       pdu.Name,
		OIDName:   oidName,
		Value:     value,
		ValueType: valueType,
		PollTime:  pollTime,
		CreatedAt: time.Now(),
	}
}

// formatPDUValue 格式化 PDU 值为字符串
func (p *Poller) formatPDUValue(pdu gosnmp.SnmpPDU) string {
	switch pdu.Type {
	case gosnmp.OctetString:
		if bytes, ok := pdu.Value.([]byte); ok {
			// 尝试作为 UTF-8 字符串
			if isPrintableString(bytes) {
				return string(bytes)
			}
			// 否则返回十六进制
			return fmt.Sprintf("%x", bytes)
		}
		return fmt.Sprintf("%v", pdu.Value)

	case gosnmp.ObjectIdentifier:
		return fmt.Sprintf("%v", pdu.Value)

	case gosnmp.IPAddress:
		return fmt.Sprintf("%v", pdu.Value)

	case gosnmp.TimeTicks:
		// TimeTicks 是百分之一秒
		ticks := gosnmp.ToBigInt(pdu.Value).Int64()
		duration := time.Duration(ticks) * time.Millisecond * 10
		return fmt.Sprintf("%v (%s)", ticks, duration.String())

	default:
		// 数值类型使用 BigInt 处理
		return fmt.Sprintf("%v", gosnmp.ToBigInt(pdu.Value))
	}
}

// getPDUTypeString 获取 PDU 类型字符串
func (p *Poller) getPDUTypeString(pdu gosnmp.SnmpPDU) string {
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
	default:
		return fmt.Sprintf("unknown(%d)", pdu.Type)
	}
}

// createSNMPClient 创建 SNMP 客户端连接
// 根据凭据版本创建 v1/v2c 或 v3 客户端
func (p *Poller) createSNMPClient(addr string, community string, cred *models.SNMPCredential) (*gosnmp.GoSNMP, error) {
	// 解析地址和端口（使用 net.SplitHostPort 正确处理 IPv6）
	target := addr
	port := uint16(161)
	host, portStr, err := net.SplitHostPort(addr)
	if err == nil {
		target = host
		var portInt int
		fmt.Sscanf(portStr, "%d", &portInt)
		if portInt > 0 {
			port = uint16(portInt)
		}
	}

	// 根据版本创建不同客户端
	if cred != nil && strings.ToLower(cred.Version) == "v3" {
		return p.createV3Client(target, port, cred)
	}

	// v1/v2c 客户端
	return p.createV1V2Client(target, port, community, cred)
}

// createV1V2Client 创建 SNMP v1/v2c 客户端
func (p *Poller) createV1V2Client(target string, port uint16, community string, cred *models.SNMPCredential) (*gosnmp.GoSNMP, error) {
	// 确定协议版本
	version := gosnmp.Version2c
	versionStr := "v2c"
	if cred != nil && strings.ToLower(cred.Version) == "v1" {
		version = gosnmp.Version1
		versionStr = "v1"
	}

	logger.Debug("SNMP-Poller", target, "创建 SNMP%s 客户端: 地址=%s:%d, 超时=%v, 重试=%d",
		versionStr, target, port, p.config.Timeout, p.config.Retries)

	client := &gosnmp.GoSNMP{
		Target:             target,
		Port:               port,
		Transport:          "udp",
		Community:          community,
		Version:            version,
		Timeout:            p.config.Timeout,
		Retries:            p.config.Retries,
		ExponentialTimeout: true,
		MaxOids:            60,
		MaxRepetitions:     50,
	}

	err := client.Connect()
	if err != nil {
		logger.Warn("SNMP-Poller", target, "SNMP%s 连接失败: 地址=%s:%d, 错误=%v",
			versionStr, target, port, err)
		return nil, fmt.Errorf("连接 SNMP 目标失败 (%s:%d): %w", target, port, err)
	}

	logger.Debug("SNMP-Poller", target, "SNMP%s 连接建立成功: 地址=%s:%d", versionStr, target, port)
	return client, nil
}

// createV3Client 创建 SNMP v3 客户端
func (p *Poller) createV3Client(target string, port uint16, cred *models.SNMPCredential) (*gosnmp.GoSNMP, error) {
	logger.Debug("SNMP-Poller", target, "创建 SNMPv3 客户端: 地址=%s:%d, 用户=%s, 安全级别=%s, 认证协议=%s, 加密协议=%s",
		target, port, cred.Username, cred.SecurityLevel, cred.AuthProtocol, cred.PrivProtocol)

	// 解密认证和加密密钥
	authPassword := ""
	if cred.AuthPassword != "" {
		decrypted, err := p.crypto.DecryptCredential(cred.AuthPassword)
		if err != nil {
			logger.Warn("SNMP-Poller", target, "解密认证密钥失败: %v", err)
			return nil, fmt.Errorf("解密认证密钥失败: %w", err)
		}
		authPassword = decrypted
	}

	privPassword := ""
	if cred.PrivPassword != "" {
		decrypted, err := p.crypto.DecryptCredential(cred.PrivPassword)
		if err != nil {
			logger.Warn("SNMP-Poller", target, "解密加密密钥失败: %v", err)
			return nil, fmt.Errorf("解密加密密钥失败: %w", err)
		}
		privPassword = decrypted
	}

	// 映射安全级别
	securityLevel := mapSecurityLevel(cred.SecurityLevel)

	// 映射认证协议
	authProtocol := mapAuthProtocol(cred.AuthProtocol)

	// 映射加密协议
	privProtocol := mapPrivProtocol(cred.PrivProtocol)

	client := &gosnmp.GoSNMP{
		Target:             target,
		Port:               port,
		Transport:          "udp",
		Version:            gosnmp.Version3,
		Timeout:            p.config.Timeout,
		Retries:            p.config.Retries,
		ExponentialTimeout: true,
		MaxOids:            60,
		MaxRepetitions:     50,
		SecurityModel:      gosnmp.UserSecurityModel,
		MsgFlags:          securityLevel,
		SecurityParameters: &gosnmp.UsmSecurityParameters{
			UserName:                 cred.Username,
			AuthenticationProtocol:   authProtocol,
			AuthenticationPassphrase: authPassword,
			PrivacyProtocol:          privProtocol,
			PrivacyPassphrase:        privPassword,
		},
		ContextName:     cred.ContextName,
		ContextEngineID: cred.ContextEngineID,
	}

	err := client.Connect()
	if err != nil {
		logger.Warn("SNMP-Poller", target, "SNMPv3 连接失败: 地址=%s:%d, 用户=%s, 错误=%v",
			target, port, cred.Username, err)
		return nil, fmt.Errorf("连接 SNMPv3 目标失败 (%s:%d): %w", target, port, err)
	}

	logger.Debug("SNMP-Poller", target, "SNMPv3 连接建立成功: 地址=%s:%d, 用户=%s", target, port, cred.Username)
	return client, nil
}

// mapSecurityLevel 映射安全级别字符串到 gosnmp 标志
func mapSecurityLevel(level string) gosnmp.SnmpV3MsgFlags {
	switch strings.ToLower(level) {
	case "authnopriv":
		return gosnmp.AuthNoPriv
	case "authpriv":
		return gosnmp.AuthPriv
	case "noauthnopriv":
		return gosnmp.NoAuthNoPriv
	default:
		return gosnmp.NoAuthNoPriv
	}
}

// mapAuthProtocol 映射认证协议字符串到 gosnmp 常量
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

// mapPrivProtocol 映射加密协议字符串到 gosnmp 常量
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

// getCommunity 获取解密后的 community string
func (p *Poller) getCommunity(cred *models.SNMPCredential) (string, error) {
	if cred == nil {
		return "public", nil // 默认 community
	}

	community := cred.Community
	if community == "" {
		return "public", nil
	}

	// 如果已加密，解密
	if p.crypto != nil && IsEncrypted(community) {
		decrypted, err := p.crypto.DecryptCredential(community)
		if err != nil {
			return "", fmt.Errorf("解密 community 失败: %w", err)
		}
		return decrypted, nil
	}

	return community, nil
}

// ============================================================================
// 事件通知
// ============================================================================

// notifySuccess 发送成功通知
func (p *Poller) notifySuccess(targetID uint, targetIP string, oidCount int, batchID string, pollTime time.Time, latency time.Duration) {
	if p.notifier == nil {
		return
	}

	p.notifier.NotifyPollingResult(PollingResultEvent{
		TargetID:  targetID,
		TargetIP:  targetIP,
		Status:    "success",
		Error:     "",
		PollTime:  pollTime.UnixMilli(),
		OIDCount:  oidCount,
		BatchID:   batchID,
	})
}

// notifyError 发送错误通知
func (p *Poller) notifyError(targetID uint, targetIP string, err error) {
	if p.notifier == nil {
		return
	}

	atomic.AddInt64(&p.totalPolls, 1)
	atomic.AddInt64(&p.failCount, 1)

	p.notifier.NotifyPollingResult(PollingResultEvent{
		TargetID:  targetID,
		TargetIP:  targetIP,
		Status:    "error",
		Error:     err.Error(),
		PollTime:  time.Now().UnixMilli(),
		OIDCount:  0,
		BatchID:   "",
	})

	p.notifier.NotifyPollError(targetID, err)
}

// ============================================================================
// 统计方法
// ============================================================================

// GetStats 获取轮询统计信息
func (p *Poller) GetStats() (total, success, fail int64) {
	return atomic.LoadInt64(&p.totalPolls),
		atomic.LoadInt64(&p.successCount),
		atomic.LoadInt64(&p.failCount)
}

// ResetStats 重置统计信息
func (p *Poller) ResetStats() {
	atomic.StoreInt64(&p.totalPolls, 0)
	atomic.StoreInt64(&p.successCount, 0)
	atomic.StoreInt64(&p.failCount, 0)
}

// ============================================================================
// 辅助函数
// ============================================================================

// isPrintableString 判断字节切片是否为可打印字符串
func isPrintableString(b []byte) bool {
	for _, c := range b {
		// 允许 ASCII 可打印字符和常见空白字符
		if c < 32 && c != '\t' && c != '\n' && c != '\r' {
			return false
		}
		if c > 126 {
			return false
		}
	}
	return true
}
